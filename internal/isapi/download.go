package isapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DownloadStream POSTs a downloadRequest for playbackURI — which MUST be the
// EXACT URI a SearchTrack call returned, including its "name"/"size" query
// parameters (trimming/omitting them gets HTTP 400 badXmlContent, confirmed
// live) — and streams the device's response body straight to w. The response
// has no XML envelope (Content-Type: Opaque/data): it's Hikvision's
// proprietary native container (magic "IMKH"), byte-exact off the device,
// NOT a standard MP4 — see internal/hik.StreamNative, which is what plays it
// back (VLC / Hik player, not a browser <video> tag).
//
// This bypasses do's 1 MiB response cap AND its timeout entirely (a single
// segment can be gigabytes and take minutes) via streamClient — see
// httpTransport.streamClient. Returns the number of bytes copied to w.
func (c *Client) DownloadStream(ctx context.Context, w io.Writer, playbackURI string) (int64, error) {
	t, ok := c.rt.(*httpTransport)
	if !ok {
		return 0, fmt.Errorf("isapi: download: requires the HTTP transport (unsupported over this Client's Transport)")
	}
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><downloadRequest><playbackURI>%s</playbackURI></downloadRequest>`,
		xmlEscaper.Replace(playbackURI))
	resp, err := t.rawRequest(ctx, http.MethodPost, "/ISAPI/ContentMgmt/download", []byte(doc))
	if err != nil {
		return 0, fmt.Errorf("isapi: download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return 0, fmt.Errorf("isapi: download: unauthorized (digest auth failed): %s", truncate(data, 300))
	}
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if statusErr := checkResponseStatus(data); statusErr != nil {
			return 0, fmt.Errorf("isapi: download: HTTP %d: %w", resp.StatusCode, statusErr)
		}
		return 0, fmt.Errorf("isapi: download: HTTP %d: %s", resp.StatusCode, truncate(data, 300))
	}

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("isapi: download: copy body after %d bytes: %w", n, err)
	}
	return n, nil
}

// DownloadTimeRange exports ONLY the exact [start, end] sub-range (device-local
// wall-clock) on trackID (e.g. 3101 for channel 31 main stream) over HTTP ISAPI
// /ISAPI/ContentMgmt/download at full LAN speed (~10-20 MB/s — ~4.4s for 5 min 2K!)
// by passing a time-only playbackURI (omitting &name= and &size= so the NVR
// trims server-side instead of dumping the entire 1 GB 2-hour HDD file).
// If maxBytes > 0 and the device ignores endtime (legacy firmware) and streams
// past maxBytes, DownloadTimeRange aborts cleanly with an error so the caller
// can fall back to RTSP playback without exhausting RAM spool.
func (c *Client) DownloadTimeRange(ctx context.Context, w io.Writer, host string, port, trackID int, start, end time.Time, maxBytes int64) (int64, error) {
	t, ok := c.rt.(*httpTransport)
	if !ok {
		return 0, fmt.Errorf("isapi: download range: requires HTTP transport")
	}
	if port <= 0 {
		port = 80
	}
	const compactFmt = "20060102T150405Z"
	uri := fmt.Sprintf("rtsp://%s:%d/Streaming/tracks/%d/?starttime=%s&endtime=%s",
		host, port, trackID, start.Format(compactFmt), end.Format(compactFmt))
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?><downloadRequest version="1.0" xmlns="http://www.isapi.org/ver20/XMLSchema"><playbackURI>%s</playbackURI></downloadRequest>`,
		xmlEscaper.Replace(uri))
	resp, err := t.rawRequest(ctx, http.MethodPost, "/ISAPI/ContentMgmt/download", []byte(doc))
	if err != nil {
		return 0, fmt.Errorf("isapi: download range: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return 0, fmt.Errorf("isapi: download range: unauthorized: %s", truncate(data, 300))
	}
	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if statusErr := checkResponseStatus(data); statusErr != nil {
			return 0, fmt.Errorf("isapi: download range: HTTP %d: %w", resp.StatusCode, statusErr)
		}
		return 0, fmt.Errorf("isapi: download range: HTTP %d: %s", resp.StatusCode, truncate(data, 300))
	}

	var reader io.Reader = resp.Body
	if maxBytes > 0 {
		reader = io.LimitReader(resp.Body, maxBytes+1)
	}
	n, copyErr := io.Copy(w, reader)
	if maxBytes > 0 && n > maxBytes {
		return n, fmt.Errorf("isapi: download range exceeded %d bytes cap (firmware ignored endtime)", maxBytes)
	}
	if copyErr != nil {
		return n, fmt.Errorf("isapi: download range copy after %d bytes: %w", n, copyErr)
	}
	if n < 1024 {
		return n, fmt.Errorf("isapi: download range returned empty/short payload (%d bytes)", n)
	}
	return n, nil
}

type adminAccessProtocolList struct {
	Protocols []struct {
		Protocol string `xml:"protocol"`
		PortNo   int    `xml:"portNo"`
	} `xml:"AdminAccessProtocol"`
}

// GetRTSPPort queries GET /ISAPI/Security/adminAccesses to discover custom RTSP
// ports (e.g. 1554 instead of 554 on NAT/enterprise Hikvision NVRs).
func (c *Client) GetRTSPPort(ctx context.Context) (int, error) {
	raw, err := c.rt.Do(ctx, http.MethodGet, "/ISAPI/Security/adminAccesses", nil)
	if err != nil {
		return 0, err
	}
	var list adminAccessProtocolList
	if err := xml.Unmarshal(raw, &list); err != nil {
		return 0, err
	}
	for _, p := range list.Protocols {
		if strings.EqualFold(strings.TrimSpace(p.Protocol), "RTSP") && p.PortNo > 0 && p.PortNo <= 65535 {
			return p.PortNo, nil
		}
	}
	return 0, fmt.Errorf("isapi: RTSP port not found in adminAccesses")
}
