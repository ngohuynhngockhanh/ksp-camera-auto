package dahua

import (
	"context"
	"fmt"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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
		// Mirror top-level DayNightColor into NightOptions/NormalOptions so
		// active scheduled sub-profiles (e.g. Imou/Dahua VideoInMode) also
		// honor Full-Color (0), Auto (1), or B&W (2) immediately.
		if rawDNC, hasDNC := optionsChanges["DayNightColor"]; hasDNC {
			dncVal := obj["DayNightColor"]
			for _, subKey := range []string{"NightOptions", "NormalOptions"} {
				if sub, ok := obj[subKey].(map[string]any); ok {
					if _, exists := sub["DayNightColor"]; exists {
						sub["DayNightColor"] = dncVal
					}
				}
			}
			_ = rawDNC
		}
		if err := c.setTable("VideoInOptions", table); err != nil {
			return nil, nil, err
		}
		if _, hasDNC := optionsChanges["DayNightColor"]; hasDNC {
			c.syncDayNightAndSupplementLight(ch, toInt(obj["DayNightColor"]))
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
		// Coerce numeric strings (e.g. HTML <select> "0"/"1"/"2" for
		// DayNightColor, Rotate90, ExposureMode) when the device's existing
		// field is a JSON number so Sonia cJSON_IsNumber accepts the write.
		if s, isStr := v.(string); isStr {
			if _, dstIsNum := dst[k].(float64); dstIsNum {
				if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
					dst[k] = f
					continue
				}
			}
		}
		dst[k] = v
	}
}

// syncDayNightAndSupplementLight best-effort synchronizes auxiliary Dahua/Imou
// day-night and dual-light tables (VideoInDayNight, LightGlobal, Lighting_V2,
// LightingScheme) when DayNightColor changes:
//   - dnc == 0 (Full-Color / Always Color): sets VideoInDayNight[*].Mode = "Color",
//     enables LightGlobal, sets WhiteLight PercentOfMaxBrightness = 100 in
//     Lighting_V2 across Day/Night/Normal profiles, and attempts WhiteLightMode
//     on Dual-Light models supporting writable LightingScheme.
//   - dnc == 1 (Auto): sets VideoInDayNight[*].Mode = "Brightness".
//   - dnc == 2 (Always B&W): sets VideoInDayNight[*].Mode = "BlackWhite".
func (c *Client) syncDayNightAndSupplementLight(ch, dnc int) {
	dnMode := "Brightness"
	switch dnc {
	case 0:
		dnMode = "Color"
	case 2:
		dnMode = "BlackWhite"
	}
	if dnTable, err := c.getTable("VideoInDayNight"); err == nil && ch >= 0 && ch < len(dnTable) {
		if profiles, ok := dnTable[ch].([]any); ok {
			changed := false
			for _, p := range profiles {
				if pm, ok := p.(map[string]any); ok {
					pm["Mode"] = dnMode
					changed = true
				}
			}
			if changed {
				_ = c.setTable("VideoInDayNight", dnTable)
			}
		}
	}
	if dnc != 0 {
		return
	}
	if lgTable, err := c.getTable("LightGlobal"); err == nil && ch >= 0 && ch < len(lgTable) {
		if lgObj, ok := lgTable[ch].(map[string]any); ok {
			lgObj["Enable"] = true
			_ = c.setTable("LightGlobal", lgTable)
		}
	}
	if v2Table, err := c.getTable("Lighting_V2"); err == nil && ch >= 0 && ch < len(v2Table) {
		if profiles, ok := v2Table[ch].([]any); ok {
			changed := false
			for _, prof := range profiles {
				entries, ok := prof.([]any)
				if !ok {
					continue
				}
				for _, item := range entries {
					em, ok := item.(map[string]any)
					if !ok || em["LightType"] != "WhiteLight" {
						continue
					}
					if _, hasMode := em["Mode"]; hasMode {
						em["Mode"] = "Auto"
					}
					if toInt(em["PercentOfMaxBrightness"]) == 0 {
						em["PercentOfMaxBrightness"] = float64(100)
					}
					changed = true
				}
			}
			if changed {
				_ = c.setTable("Lighting_V2", v2Table)
			}
		}
	}
	// Best-effort switch Dual-Light cameras (Imou Cruiser/Bullet, Dahua Full-Color)
	// from InfraredMode to WhiteLightMode if their firmware permits writing LightingScheme.
	_, _ = c.Call("configManager.setConfig", map[string]any{
		"name": "LightingScheme",
		"table": []any{
			[]any{
				map[string]any{"LightingMode": "WhiteLightMode"},
				map[string]any{"LightingMode": "WhiteLightMode"},
				map[string]any{"LightingMode": "WhiteLightMode"},
			},
		},
	})
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
	// Cap best-effort HTTP :80 CGI probe at 2.5s so standalone Imou/Dahua IPCs
	// whose port 80 is a non-CGI stub (or behind DVRIP-only NAT) never stall
	// GET /api/picture for 30s.
	if timeout <= 0 || timeout > 2500*time.Millisecond {
		timeout = 2500 * time.Millisecond
	}
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
