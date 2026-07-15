package dahua

import (
	"encoding/json"
	"fmt"
)

// Stream selects which encoded stream to operate on.
type Stream int

const (
	StreamMain Stream = iota // MainFormat[0]
	StreamSub1               // ExtraFormat[0]
	StreamSub2               // ExtraFormat[1]
)

// StreamInfo is a read-back summary of one stream's encode settings.
type StreamInfo struct {
	Channel     int    `json:"channel"`
	Stream      Stream `json:"stream"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	Compression string `json:"compression"`
	Profile     string `json:"profile"`
	AudioCodec  string `json:"audioCodec"`
	AudioEnable bool   `json:"audioEnable"`
	SmartCodec  bool   `json:"smartCodec"`
}

// getTable fetches configManager.getConfig <name> and returns params.table.
func (c *Client) getTable(name string) ([]any, error) {
	resp, err := c.Call("configManager.getConfig", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	if !resp.ok() {
		return nil, fmt.Errorf("getConfig %s failed: %s", name, respErr(resp))
	}
	var p struct {
		Table []any `json:"table"`
	}
	if err := json.Unmarshal(resp.Params, &p); err != nil {
		return nil, fmt.Errorf("getConfig %s: decode table: %w", name, err)
	}
	return p.Table, nil
}

// setTable writes configManager.setConfig <name> with the full table.
func (c *Client) setTable(name string, table []any) error {
	resp, err := c.Call("configManager.setConfig", map[string]any{"name": name, "table": table})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("setConfig %s failed: %s", name, respErr(resp))
	}
	return nil
}

func respErr(r rpcResp) string {
	if msg := r.errMessage(); msg != "" {
		return msg
	}
	return "result=false"
}

// formatOf navigates table[ch] to the MainFormat/ExtraFormat object for stream.
func formatOf(table []any, ch int, s Stream) (map[string]any, error) {
	if ch < 0 || ch >= len(table) {
		return nil, fmt.Errorf("channel %d out of range (have %d)", ch, len(table))
	}
	chObj, ok := table[ch].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("channel %d: unexpected shape", ch)
	}
	var arrKey string
	var idx int
	switch s {
	case StreamMain:
		arrKey, idx = "MainFormat", 0
	case StreamSub1:
		arrKey, idx = "ExtraFormat", 0
	case StreamSub2:
		arrKey, idx = "ExtraFormat", 1
	default:
		return nil, fmt.Errorf("unknown stream %d", s)
	}
	arr, ok := chObj[arrKey].([]any)
	if !ok || idx >= len(arr) {
		return nil, fmt.Errorf("channel %d has no %s[%d]", ch, arrKey, idx)
	}
	fmtObj, ok := arr[idx].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("channel %d %s[%d]: unexpected shape", ch, arrKey, idx)
	}
	return fmtObj, nil
}

func subMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	nm := map[string]any{}
	m[key] = nm
	return nm
}

// SetResolution sets the pixel resolution (and keeps CustomResolutionName in sync)
// for one channel/stream, using GET-modify-SET on the Encode config.
func (c *Client) SetResolution(ch int, s Stream, w, h int) error {
	table, err := c.getTable("Encode")
	if err != nil {
		return err
	}
	fmtObj, err := formatOf(table, ch, s)
	if err != nil {
		return err
	}
	video := subMap(fmtObj, "Video")
	video["Width"] = w
	video["Height"] = h
	video["CustomResolutionName"] = fmt.Sprintf("%dx%d", w, h)
	return c.setTable("Encode", table)
}

// SetCodec sets the video codec/profile for a stream. compression is the Dahua
// Video.Compression value (e.g. "H.265", "H.264", "H.264H" = High profile,
// "H.264B" = Baseline, "MJPG"). If profile is non-empty it is also written to
// Video.Profile ("Main"/"High"/"Baseline"). The device rejects unsupported
// codecs with an explicit error, which callers surface in the progress log.
func (c *Client) SetCodec(ch int, s Stream, compression, profile string) error {
	table, err := c.getTable("Encode")
	if err != nil {
		return err
	}
	fmtObj, err := formatOf(table, ch, s)
	if err != nil {
		return err
	}
	video := subMap(fmtObj, "Video")
	if compression != "" {
		video["Compression"] = compression
	}
	if profile != "" {
		video["Profile"] = profile
	}
	return c.setTable("Encode", table)
}

// SetAudioAAC forces the stream's audio codec to AAC and enables audio.
func (c *Client) SetAudioAAC(ch int, s Stream) error {
	table, err := c.getTable("Encode")
	if err != nil {
		return err
	}
	fmtObj, err := formatOf(table, ch, s)
	if err != nil {
		return err
	}
	subMap(fmtObj, "Audio")["Compression"] = "AAC"
	fmtObj["AudioEnable"] = true
	return c.setTable("Encode", table)
}

// SetSmartCodec toggles Dahua "Smart Codec" (H.264+/H.265+) for a channel via
// the SmartEncode config. Smart codec is a per-channel switch (not per-stream).
func (c *Client) SetSmartCodec(ch int, on bool) error {
	table, err := c.getTable("SmartEncode")
	if err != nil {
		return err
	}
	if ch < 0 || ch >= len(table) {
		return fmt.Errorf("channel %d out of range for SmartEncode (have %d)", ch, len(table))
	}
	chObj, ok := table[ch].(map[string]any)
	if !ok {
		return fmt.Errorf("SmartEncode channel %d: unexpected shape", ch)
	}
	chObj["Enable"] = on
	return c.setTable("SmartEncode", table)
}

// ProbeAll reads every channel's main + sub streams in a single pass (fetches
// the Encode and SmartEncode configs once), so an NVR's whole camera list comes
// back in two requests. Channel in the result is 1-based (camera number).
func (c *Client) ProbeAll() ([]StreamInfo, error) {
	table, err := c.getTable("Encode")
	if err != nil {
		return nil, err
	}
	smart, _ := c.getTable("SmartEncode") // best-effort

	var out []StreamInfo
	for ci := 0; ci < len(table); ci++ {
		for _, s := range []Stream{StreamMain, StreamSub1, StreamSub2} {
			fmtObj, err := formatOf(table, ci, s)
			if err != nil {
				continue
			}
			info := StreamInfo{Channel: ci + 1, Stream: s}
			if v, ok := fmtObj["Video"].(map[string]any); ok {
				info.Width = toInt(v["Width"])
				info.Height = toInt(v["Height"])
				info.FPS = toInt(v["FPS"])
				info.Compression, _ = v["Compression"].(string)
				info.Profile, _ = v["Profile"].(string)
			}
			if a, ok := fmtObj["Audio"].(map[string]any); ok {
				info.AudioCodec, _ = a["Compression"].(string)
			}
			info.AudioEnable, _ = fmtObj["AudioEnable"].(bool)
			if ci < len(smart) {
				if so, ok := smart[ci].(map[string]any); ok {
					info.SmartCodec, _ = so["Enable"].(bool)
				}
			}
			out = append(out, info)
		}
	}
	return out, nil
}

// GetStreamInfo reads back the current encode settings for a channel/stream.
func (c *Client) GetStreamInfo(ch int, s Stream) (StreamInfo, error) {
	info := StreamInfo{Channel: ch, Stream: s}
	table, err := c.getTable("Encode")
	if err != nil {
		return info, err
	}
	fmtObj, err := formatOf(table, ch, s)
	if err != nil {
		return info, err
	}
	if v, ok := fmtObj["Video"].(map[string]any); ok {
		info.Width = toInt(v["Width"])
		info.Height = toInt(v["Height"])
		info.FPS = toInt(v["FPS"])
		info.Compression, _ = v["Compression"].(string)
		info.Profile, _ = v["Profile"].(string)
	}
	if a, ok := fmtObj["Audio"].(map[string]any); ok {
		info.AudioCodec, _ = a["Compression"].(string)
	}
	info.AudioEnable, _ = fmtObj["AudioEnable"].(bool)

	// Smart codec is a separate config.
	if st, err := c.getTable("SmartEncode"); err == nil && ch < len(st) {
		if chObj, ok := st[ch].(map[string]any); ok {
			info.SmartCodec, _ = chObj["Enable"].(bool)
		}
	}
	return info, nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}
