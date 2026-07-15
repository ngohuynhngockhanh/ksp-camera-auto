package dahua

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// GetPicture reads back a channel's full color + picture-tuning config:
// VideoColor[ch][0] (Brightness/Contrast/Hue/Saturation) and
// VideoInOptions[ch] (Flip/Mirror/Rotate90/WhiteBalance/exposure/gain/
// backlight/antiflicker, plus the nested NightOptions/NormalOptions
// day-night sub-profiles), confirmed against
// dahua_http_api_for_ipcsd-v1.40.pdf §4.2/§4.3.
//
// Both maps are returned exactly as the device sends them (decoded JSON:
// string/bool/float64/nested map/array), deliberately not flattened into a
// hand-typed Go struct — the real field set varies by model/firmware (see
// GetVideoInputCaps), and duplicating ~90 field names into Go risks silent
// drift from what a given device actually exposes. Callers (the picture API
// handler, the UI) work generically off these maps; SetPicture accepts the
// same shape back for the fields being changed.
func (c *Client) GetPicture(ch int) (color, options map[string]any, err error) {
	colorTable, err := c.getTable("VideoColor")
	if err != nil {
		return nil, nil, err
	}
	colorObj, err := videoColorObj(colorTable, ch)
	if err != nil {
		return nil, nil, err
	}
	optsTable, err := c.getTable("VideoInOptions")
	if err != nil {
		return nil, nil, err
	}
	optsObj, err := channelObj(optsTable, "VideoInOptions", ch)
	if err != nil {
		return nil, nil, err
	}
	return colorObj, optsObj, nil
}

// SetPicture merges colorChanges into VideoColor[ch][0] and optionsChanges
// into VideoInOptions[ch] (GET-modify-SET, so any field not present in the
// *Changes maps is left untouched), writes both tables, then reads them back.
// The returned maps are the post-write, live device state — the caller/UI
// compares them against what was requested to report which fields actually
// took (the device can silently clamp or ignore a value it doesn't support,
// the same tri-state situation GOP/bitrate handle in camera.go).
func (c *Client) SetPicture(ch int, colorChanges, optionsChanges map[string]any) (color, options map[string]any, err error) {
	if len(colorChanges) > 0 {
		table, err := c.getTable("VideoColor")
		if err != nil {
			return nil, nil, err
		}
		obj, err := videoColorObj(table, ch)
		if err != nil {
			return nil, nil, err
		}
		for k, v := range colorChanges {
			obj[k] = v
		}
		if err := c.setTable("VideoColor", table); err != nil {
			return nil, nil, err
		}
	}
	if len(optionsChanges) > 0 {
		table, err := c.getTable("VideoInOptions")
		if err != nil {
			return nil, nil, err
		}
		obj, err := channelObj(table, "VideoInOptions", ch)
		if err != nil {
			return nil, nil, err
		}
		mergeNested(obj, optionsChanges)
		if err := c.setTable("VideoInOptions", table); err != nil {
			return nil, nil, err
		}
	}
	return c.GetPicture(ch)
}

// mergeNested copies src into dst key by key, recursing one level when both
// sides hold a nested object at the same key (so a caller can send just
// {"NightOptions": {"GainRed": 60}} without clobbering the rest of
// NightOptions). Deeper structures (arrays, e.g. BacklightRegion) are
// replaced wholesale, matching setTable's existing whole-value-replace
// behavior elsewhere in this package.
func mergeNested(dst, src map[string]any) {
	for k, v := range src {
		if sv, ok := v.(map[string]any); ok {
			if dv, ok := dst[k].(map[string]any); ok {
				mergeNested(dv, sv)
				continue
			}
		}
		dst[k] = v
	}
}

// videoColorObj navigates table[ch][0] to the color-config object.
// VideoColor is doubly-indexed (table.VideoColor[ChannelNo][ColorConfigNo]
// per §4.2) — this project only ever reads/writes ColorConfigNo 0 ("Color
// Config 1"), the config active by default with no TimeSection restriction.
func videoColorObj(table []any, ch int) (map[string]any, error) {
	if ch < 0 || ch >= len(table) {
		return nil, fmt.Errorf("channel %d out of range for VideoColor (have %d)", ch, len(table))
	}
	arr, ok := table[ch].([]any)
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("VideoColor channel %d: no color config entries", ch)
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("VideoColor channel %d config 0: unexpected shape", ch)
	}
	return obj, nil
}

// GetVideoInputCaps reads a channel's video-input capability flags (GET
// /cgi-bin/devVideoInput.cgi?action=getCaps&channel=<n>), confirmed against
// dahua_http_api_for_ipcsd-v1.40.pdf §4.3.1: which of
// Flip/Mirror/Rotate90/WhiteBalance/NightOptions/SetColor... this specific
// channel/model actually supports, so the UI can disable controls the device
// will just ignore rather than let a user "successfully" set something with
// no effect. Like GetSnapshot/ScanWiFi, this is plain HTTP+Digest on port 80
// — there is no configManager/DVRIP equivalent for capability flags — so it
// opens a separate connection from the DVRIP session. host must be bare (no
// port).
func GetVideoInputCaps(ctx context.Context, host, user, pass string, channel int, timeout time.Duration) (map[string]any, error) {
	digest := isapi.NewDigestTransport(user, pass, nil)
	client := &http.Client{Transport: digest, Timeout: timeout}
	url := fmt.Sprintf("http://%s:80/cgi-bin/devVideoInput.cgi?action=getCaps&channel=%d", host, channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dahua: build getCaps request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dahua: getCaps %s: %w", host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("dahua: read getCaps %s: %w", host, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dahua: getCaps %s: HTTP %d: %s", host, resp.StatusCode, snapshotTruncate(body, 200))
	}
	return parseCapsLines(string(body)), nil
}

// parseCapsLines decodes the text/plain "caps.Field=Value" body §4.3.1
// documents, stripping the "caps." prefix so callers key off plain field
// names (e.g. "Flip", "WhiteBalance") matching GetPicture's VideoInOptions
// keys.
func parseCapsLines(body string) map[string]any {
	out := map[string]any{}
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "caps."))
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		out[line[:i]] = line[i+1:]
	}
	return out
}
