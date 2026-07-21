package dahua

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
)

// StreamDav downloads a channel's [start,end] recording as the camera's native
// .dav (DHAV container) and writes it to w — the byte-exact stored file, no
// ffmpeg/remux. It speaks the Dahua "param protocol" (\xf4 frames) over DVRIP,
// reverse-engineered from a NetSDK CLIENT_DownloadByTimeEx capture:
//
//  1. Dial + DVRIP login, mediaFileFind enumerates the .dav segments in the range.
//  2. Split the range into byte-bounded CHUNKS and download each on its OWN login +
//     passive sub-channel (see davChunk), concatenating the DHAV bytes to w.
//
// Why chunk: a single whole-range PlayBack.download is capped by the device at
// ~350 MB / ~26 min — it streams that much then RESETS the sub-channel, so a long
// range comes back ~87% short. A whole-range request DOES span multiple segments
// (a request scoped to one segment's times yields only that segment), so we split
// the range into chunks that each finish under the cap and stitch them. Chunk cuts
// fall on segment boundaries — each .dav segment starts on an I-frame — so the
// seams are clean. Each chunk gets a fresh login because the device drops the whole
// control session (not just the sub-channel) when a download finishes.
//
// Nothing is written to the box's disk. Unlike StreamPlayback (RTSP, no DVRIP),
// this REQUIRES the DVRIP config port, so it can fail on a camera whose config
// port is saturated by another recorder — callers keep MP4 as the default.
func StreamDav(ctx context.Context, w io.Writer, host string, port int, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("dahua: dav %s: %w (waiting for a slot)", host, ctx.Err())
	}

	// Enumerate the .dav segments in the range on a short-lived login.
	c0, err := Dial(net.JoinHostPort(host, dvripPort(port)), user, pass, 15*time.Second)
	if err != nil {
		return fmt.Errorf("dahua: dav %s: login: %w", host, err)
	}
	recs, err := c0.FindRecordings(channel, start, end)
	c0.Close()
	if err != nil {
		return fmt.Errorf("dahua: dav %s: find: %w", host, err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("dahua: dav %s: no recording in range", host)
	}

	ch := channel + 1 // this protocol is 1-based

	// Walk the segments, grouping them into chunks whose combined size stays under
	// the device's per-download cap, and download each chunk on its own session.
	for i := 0; i < len(recs); {
		// Grow the chunk until adding the next segment would exceed the cap (always
		// take at least one segment, even if a single segment is itself over-cap).
		j, size := i, int64(0)
		for j < len(recs) && (j == i || size+recs[j].Length <= davChunkBytes) {
			size += recs[j].Length
			j++
		}

		// Clamp the first chunk's start and the last chunk's end to the requested
		// window; interior cuts use the exact segment boundary times so consecutive
		// chunks meet without overlap or gap.
		startStr := davTimeStr(recs[i].StartTime)
		if i == 0 || startStr == "" {
			startStr = davTime(start)
		}
		endStr := davTimeStr(recs[j-1].EndTime)
		if j == len(recs) || endStr == "" {
			endStr = davTime(end)
		}

		if err := davChunk(ctx, w, host, port, user, pass, ch, startStr, endStr, recs[i].FilePath); err != nil {
			return fmt.Errorf("dahua: dav %s: chunk segs %d-%d: %w", host, i, j-1, err)
		}
		i = j
	}
	return nil
}

// davChunkBytes caps how many bytes (by mediaFileFind's reported segment Length)
// one PlayBack.download chunk may cover. The device resets a single download after
// ~350 MB, so this stays well under that with margin for Length underreporting the
// on-wire DHAV size.
const davChunkBytes = 180 << 20

// davChunk downloads one [startStr,endStr] sub-range on its own DVRIP login +
// passive sub-channel and appends its DHAV bytes to w. fileDir is the on-device
// path of the chunk's first segment (the device wants it to locate the starting
// file). A fresh login per chunk is required — the device tears down the whole
// control session when a download finishes, so a prior chunk's session is unusable.
// It runs the full handshake: login -> AddObject Passive -> 2nd TCP + AckSubChannel
// -> PlayBack.download -> read 0xbb media frames (keepalive-poked) until the device
// finishes and closes, then PlayBack.Stop + DeleteObject to release it promptly.
func davChunk(ctx context.Context, w io.Writer, host string, port int, user, pass string, ch int, startStr, endStr, fileDir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c, err := Dial(net.JoinHostPort(host, dvripPort(port)), user, pass, 15*time.Second)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer c.Close()

	// Allocate a passive control connection (data sub-channel) for this chunk.
	addResp, err := c.callF4("TransactionID:1\r\nMethod:AddObject\r\n" +
		"ParameterName:Dahua.Device.Network.ControlConnection.Passive\r\nConnectProtocol:0\r\n\r\n")
	if err != nil {
		return fmt.Errorf("AddObject: %w", err)
	}
	connID := paramValue(addResp, "ConnectionID")
	if connID == "" {
		return fmt.Errorf("no ConnectionID (resp: %.200s)", addResp)
	}
	dataPort := paramValue(addResp, "Port")
	if dataPort == "" || dataPort == "0" {
		dataPort = dvripPort(port)
	}

	// Open the data sub-channel (a 2nd TCP) and register it for our session.
	dataConn, err := net.DialTimeout("tcp", net.JoinHostPort(host, dataPort), 10*time.Second)
	if err != nil {
		return fmt.Errorf("data dial: %w", err)
	}
	defer dataConn.Close()
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		select {
		case <-ctx.Done():
			dataConn.Close()
		case <-closed:
		}
	}()

	ack := fmt.Sprintf("TransactionID:0\r\nMethod:GetParameterNames\r\n"+
		"ParameterName:Dahua.Device.Network.ControlConnection.AckSubChannel\r\n"+
		"SessionID:%d\r\nConnectionID:%s\r\n\r\n", c.sessionID, connID)
	if err := writeF4(dataConn, []byte(ack), c.timeout); err != nil {
		return fmt.Errorf("AckSubChannel: %w", err)
	}

	stop := func() {
		_, _ = c.callF4(fmt.Sprintf("TransactionID:9\r\nMethod:GetParameterNames\r\n"+
			"ParameterName:Dahua.Device.Network.PlayBack.Stop\r\nchannel:%d\r\nConnectionID:%s\r\n\r\n", ch, connID))
	}
	// Always release the sub-channel so the camera frees the session promptly.
	defer func() {
		stop()
		_, _ = c.callF4(fmt.Sprintf("TransactionID:10\r\nMethod:DeleteObject\r\n"+
			"ParameterName:Dahua.Device.Network.ControlConnection.Passive\r\nConnectionID:%s\r\n\r\n", connID))
	}()

	// Start this chunk's download.
	stop()
	dl := fmt.Sprintf("TransactionID:3\r\nMethod:GetParameterNames\r\n"+
		"ParameterName:Dahua.Device.Network.PlayBack.download\r\n"+
		"channel:%d\r\nConnectionID:%s\r\n"+
		"StartTime:%s\r\nEndTime:%s\r\n"+
		"DriveNo:0\r\nClusterNo:0\r\nHint:0\r\nType:0\r\nIsTime:1\r\nOffLength:0\r\nUkey:\r\n"+
		"FileDir:%s\r\n\r\n", ch, connID, startStr, endStr, fileDir)
	if _, err := c.callF4(dl); err != nil {
		return fmt.Errorf("PlayBack.download: %w", err)
	}

	// Keep the sub-channel alive: the device buffers a chunk then PAUSES, waiting
	// for the client to poke it — the NetSDK sends periodic \xa1 keepalives, without
	// which the stream stalls mid-chunk. Send one every 2s.
	kaStop := make(chan struct{})
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		ka := make([]byte, headerLen)
		ka[0] = 0xa1
		for {
			select {
			case <-kaStop:
				return
			case <-t.C:
				_ = dataConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := dataConn.Write(ka); err != nil {
					return
				}
			}
		}
	}()
	defer close(kaStop)

	// Read the sub-channel until the chunk is delivered and the device closes/idles.
	return copyDavSegment(ctx, w, dataConn, 0)
}

// davTime formats a time as Dahua's "Y&M&D&H&M&S" with no zero-padding.
func davTime(t time.Time) string {
	return fmt.Sprintf("%d&%d&%d&%d&%d&%d", t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// davTimeStr reformats a device-local timestamp ("2006-01-02 15:04:05", as
// returned by mediaFileFind) into davTime's "Y&M&D&H&M&S". It parses in UTC so
// only the wall-clock fields carry over — no timezone shift — matching how the
// device names its own segment boundaries. Returns "" if the string is unparseable.
func davTimeStr(devTime string) string {
	t, err := time.Parse(deviceTimeLayout, devTime)
	if err != nil {
		return ""
	}
	return davTime(t)
}

// callF4 sends one \xf4 param-protocol frame on the control connection and
// returns the response payload. readFrame handles the reply framing (\xf4/\xf6).
func (c *Client) callF4(payload string) ([]byte, error) {
	if err := writeF4(c.conn, []byte(payload), c.timeout); err != nil {
		return nil, err
	}
	_, resp, err := c.readFrame()
	return resp, err
}

// writeF4 writes a \xf4 frame: 32-byte header (magic 0xf4, payload length at
// [4:8] little-endian, rest zero) followed by the payload.
func writeF4(conn net.Conn, payload []byte, timeout time.Duration) error {
	hdr := make([]byte, headerLen)
	binary.BigEndian.PutUint32(hdr[0:4], 0xf4000000)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(payload)))
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	if _, err := conn.Write(append(hdr, payload...)); err != nil {
		return fmt.Errorf("write f4: %w", err)
	}
	return nil
}

// paramValue extracts a "Key:Value" line's value from a \xf4 payload.
func paramValue(payload []byte, key string) string {
	for _, line := range strings.Split(string(payload), "\r\n") {
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// davIdle bounds how long a chunk's stream may go silent before it's treated as
// finished — a fallback for a device that goes quiet at end-of-range instead of
// closing the socket. The normal terminator is the device closing the sub-channel
// (EOF / connection reset), which ends the read immediately.
const davIdle = 20 * time.Second

// copyDavSegment reads a chunk's DHAV off the sub-channel and writes it to w until
// the device finishes: it closes the sub-channel (EOF or connection reset — its
// normal end-of-download signal) or goes idle past davIdle. Non-0xbb frames (the
// \xf4 AckSubChannel reply before the data, keepalive echoes) are consumed and
// skipped. The expected arg is unused (kept for the caller's clarity).
func copyDavSegment(ctx context.Context, w io.Writer, conn net.Conn, expected int64) error {
	hdr := make([]byte, headerLen)
	var got int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(davIdle))
		if _, err := io.ReadFull(conn, hdr); err != nil {
			if isIdleOrEOF(err) {
				return nil // chunk finished (device closed / reset / idle)
			}
			return fmt.Errorf("dahua: dav read header: %w (after %d bytes)", err, got)
		}
		n := binary.LittleEndian.Uint32(hdr[4:8])
		if n == 0 {
			continue
		}
		if n > maxFrame {
			return fmt.Errorf("dahua: dav frame too large: %d", n)
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(conn, payload); err != nil {
			if isIdleOrEOF(err) {
				return nil
			}
			return fmt.Errorf("dahua: dav read payload: %w", err)
		}
		if hdr[0] == 0xbb { // media frame -> DHAV bytes
			if _, err := w.Write(payload); err != nil {
				return err
			}
			got += int64(n)
		}
	}
}

// isIdleOrEOF reports whether err is a normal end-of-download signal: EOF, a read
// timeout (the idle terminator), or the device resetting the connection (its usual
// way of closing a finished download).
func isIdleOrEOF(err error) bool {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
