package dahua

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// StreamDav downloads a channel's [start,end] recording as the camera's native
// .dav (DHAV container) and writes it to w — the byte-exact stored file, no
// ffmpeg/remux. It speaks the Dahua "param protocol" (\xf4 frames) over DVRIP,
// reverse-engineered from a NetSDK CLIENT_DownloadByTimeEx capture:
//
//  1. Dial + DVRIP login (a dedicated session — a download holds the socket).
//  2. AddObject Dahua.Device.Network.ControlConnection.Passive -> a ConnectionID
//     and the port for a data sub-channel.
//  3. mediaFileFind locates the .dav segment file for `start`.
//  4. Open a 2nd TCP to the advertised port and register it as the sub-channel
//     (AckSubChannel with our SessionID + ConnectionID).
//  5. PlayBack.download on the control conn -> the device streams the recording
//     on the data sub-channel as 0xbb media frames wrapping DHAV; we strip the
//     32-byte frame headers and write the DHAV bytes to w.
//  6. PlayBack.Stop + DeleteObject to release the session.
//
// Nothing is written to the box's disk. Unlike StreamPlayback (RTSP, no DVRIP),
// this REQUIRES the DVRIP config port, so it can fail on a camera whose config
// port is saturated by another recorder — callers keep MP4 as the default.
func StreamDav(ctx context.Context, w io.Writer, host, user, pass string, channel int, start, end time.Time) error {
	select {
	case playbackSem <- struct{}{}:
		defer func() { <-playbackSem }()
	case <-ctx.Done():
		return fmt.Errorf("dahua: dav %s: %w (waiting for a slot)", host, ctx.Err())
	}

	c, err := Dial(net.JoinHostPort(host, "37777"), user, pass, 15*time.Second)
	if err != nil {
		return fmt.Errorf("dahua: dav %s: login: %w", host, err)
	}
	defer c.Close()

	// 1. Allocate a passive control connection (data sub-channel).
	addResp, err := c.callF4("TransactionID:1\r\nMethod:AddObject\r\n" +
		"ParameterName:Dahua.Device.Network.ControlConnection.Passive\r\nConnectProtocol:0\r\n\r\n")
	if err != nil {
		return fmt.Errorf("dahua: dav %s: AddObject: %w", host, err)
	}
	connID := paramValue(addResp, "ConnectionID")
	if connID == "" {
		return fmt.Errorf("dahua: dav %s: no ConnectionID (resp: %.200s)", host, addResp)
	}
	dataPort := paramValue(addResp, "Port")
	if dataPort == "" || dataPort == "0" {
		dataPort = "37777"
	}

	// 2. Locate the .dav segment containing `start`. The device streams the whole
	// [start,end] range from this file handle when IsTime=1.
	recs, err := c.FindRecordings(channel, start, end)
	if err != nil {
		return fmt.Errorf("dahua: dav %s: find: %w", host, err)
	}
	if len(recs) == 0 {
		return fmt.Errorf("dahua: dav %s: no recording in range", host)
	}
	fileDir := recs[0].FilePath

	// 3. Open the data sub-channel (a 2nd TCP) and register it for our session.
	dataConn, err := net.DialTimeout("tcp", net.JoinHostPort(host, dataPort), 10*time.Second)
	if err != nil {
		return fmt.Errorf("dahua: dav %s: data dial: %w", host, err)
	}
	defer dataConn.Close()
	go func() { <-ctx.Done(); dataConn.Close() }()
	ack := fmt.Sprintf("TransactionID:0\r\nMethod:GetParameterNames\r\n"+
		"ParameterName:Dahua.Device.Network.ControlConnection.AckSubChannel\r\n"+
		"SessionID:%d\r\nConnectionID:%s\r\n\r\n", c.sessionID, connID)
	if err := writeF4(dataConn, []byte(ack), c.timeout); err != nil {
		return fmt.Errorf("dahua: dav %s: AckSubChannel: %w", host, err)
	}

	// 4. Reset then start the time-based download onto the sub-channel.
	ch := channel + 1 // this protocol is 1-based
	_, _ = c.callF4(fmt.Sprintf("TransactionID:2\r\nMethod:GetParameterNames\r\n"+
		"ParameterName:Dahua.Device.Network.PlayBack.Stop\r\nchannel:%d\r\nConnectionID:%s\r\n\r\n", ch, connID))
	dl := fmt.Sprintf("TransactionID:3\r\nMethod:GetParameterNames\r\n"+
		"ParameterName:Dahua.Device.Network.PlayBack.download\r\n"+
		"channel:%d\r\nConnectionID:%s\r\n"+
		"StartTime:%s\r\nEndTime:%s\r\n"+
		"DriveNo:0\r\nClusterNo:0\r\nHint:0\r\nType:0\r\nIsTime:1\r\nOffLength:0\r\nUkey:\r\n"+
		"FileDir:%s\r\n\r\n", ch, connID, davTime(start), davTime(end), fileDir)
	if _, err := c.callF4(dl); err != nil {
		return fmt.Errorf("dahua: dav %s: PlayBack.download: %w", host, err)
	}

	// Always try to stop cleanly so the camera frees the session promptly.
	defer func() {
		_, _ = c.callF4(fmt.Sprintf("TransactionID:9\r\nMethod:GetParameterNames\r\n"+
			"ParameterName:Dahua.Device.Network.PlayBack.Stop\r\nchannel:%d\r\nConnectionID:%s\r\n\r\n", ch, connID))
		_, _ = c.callF4(fmt.Sprintf("TransactionID:10\r\nMethod:DeleteObject\r\n"+
			"ParameterName:Dahua.Device.Network.ControlConnection.Passive\r\nConnectionID:%s\r\n\r\n", connID))
	}()

	// 5. Read the sub-channel: skip the \xf4 ack response, then copy every 0xbb
	// media frame's DHAV payload to w. The device delivers the whole range in a
	// fast burst then goes silent (it does not close), so an idle read-deadline
	// marks end-of-download.
	return copyDavStream(ctx, w, dataConn)
}

// davTime formats a time as Dahua's "Y&M&D&H&M&S" with no zero-padding.
func davTime(t time.Time) string {
	return fmt.Sprintf("%d&%d&%d&%d&%d&%d", t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
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

// davIdle is how long the data sub-channel may go silent before the download is
// treated as complete. At true end-of-range the device CLOSES the sub-channel
// (clean EOF ends the read instantly — no idle wait), so this is only a fallback
// for the rare "device stops sending but doesn't close" case. It must comfortably
// exceed any mid-stream stall (e.g. the device opening the next .dav segment on a
// multi-segment range), or a long download truncates at a segment boundary.
const davIdle = 15 * time.Second

// copyDavStream reads the data sub-channel's 32-byte-framed messages and writes
// the DHAV payload of every 0xbb media frame to w, until the stream goes idle
// (end of range) or the connection ends.
func copyDavStream(ctx context.Context, w io.Writer, conn net.Conn) error {
	hdr := make([]byte, headerLen)
	var wrote int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(davIdle))
		if _, err := io.ReadFull(conn, hdr); err != nil {
			if wrote > 0 && isIdleOrEOF(err) {
				return nil // burst finished
			}
			return fmt.Errorf("dahua: dav read header: %w (after %d bytes)", err, wrote)
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
			return fmt.Errorf("dahua: dav read payload: %w", err)
		}
		if hdr[0] == 0xbb { // media frame -> DHAV bytes
			if _, err := w.Write(payload); err != nil {
				return err
			}
			wrote += int64(n)
		}
		// other magics (0xf4 ack response, keepalive) are consumed and skipped
	}
}

func isIdleOrEOF(err error) bool {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
