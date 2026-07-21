package isapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
