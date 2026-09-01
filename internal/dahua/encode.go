package dahua

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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

	GOP            int    `json:"gop"`
	BitRate        int    `json:"bitRate"`
	BitRateControl string `json:"bitRateControl"`

	// Name is the device's own ChannelTitle[Channel].Name, and OSDLines is
	// VideoWidget[Channel].CustomTitle[].Text (see name.go). Populated once
	// per channel by ProbeAll (not GetStreamInfo, which stays fast for the
	// apply-verify before/after loop); best-effort, left empty on failure.
	Name     string   `json:"name,omitempty"`
	OSDLines []string `json:"osdLines,omitempty"`
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

// getChannelTable fetches configManager.getConfig for name[ch] (or full table fallback).
func (c *Client) getChannelTable(name string, ch int) (map[string]any, error) {
	resp, err := c.Call("configManager.getConfig", map[string]any{"name": fmt.Sprintf("%s[%d]", name, ch)})
	if err == nil && resp.ok() {
		var p struct {
			Table any `json:"table"`
		}
		if err := json.Unmarshal(resp.Params, &p); err == nil {
			if m, ok := p.Table.(map[string]any); ok {
				return m, nil
			}
			if arr, ok := p.Table.([]any); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]any); ok {
					return m, nil
				}
			}
		}
	}
	// Fallback to full table
	full, err := c.getTable(name)
	if err != nil {
		return nil, err
	}
	if ch < 0 || ch >= len(full) {
		return nil, fmt.Errorf("channel %d out of range (have %d)", ch, len(full))
	}
	if m, ok := full[ch].(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("channel %d: unexpected shape", ch)
}

// setChannelTable writes configManager.setConfig for name[ch] (or full table fallback).
func (c *Client) setChannelTable(name string, ch int, chObj map[string]any) error {
	resp, err := c.Call("configManager.setConfig", map[string]any{"name": fmt.Sprintf("%s[%d]", name, ch), "table": chObj})
	if err == nil && resp.ok() {
		return nil
	}
	// Fallback to setting via full table
	full, err := c.getTable(name)
	if err != nil {
		return err
	}
	if ch < 0 || ch >= len(full) {
		return fmt.Errorf("channel %d out of range (have %d)", ch, len(full))
	}
	full[ch] = chObj
	return c.setTable(name, full)
}

// formatOfChannel navigates a channel's object to the MainFormat/ExtraFormat object for stream.
func formatOfChannel(chObj map[string]any, s Stream) (map[string]any, error) {
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
		return nil, fmt.Errorf("has no %s[%d]", arrKey, idx)
	}
	fmtObj, ok := arr[idx].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s[%d]: unexpected shape", arrKey, idx)
	}
	return fmtObj, nil
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
	return formatOfChannel(chObj, s)
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
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return err
	}
	video := subMap(fmtObj, "Video")
	video["Width"] = w
	video["Height"] = h
	video["CustomResolutionName"] = fmt.Sprintf("%dx%d", w, h)
	return c.setChannelTable("Encode", ch, chObj)
}

// SetFPS sets the stream frame rate through Encode.Video.FPS.
func (c *Client) SetFPS(ch int, s Stream, fps int) error {
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return err
	}
	subMap(fmtObj, "Video")["FPS"] = fps
	return c.setChannelTable("Encode", ch, chObj)
}

// GetMaxFPS reads the vendor capability table. Dahua firmware varies widely
// in the exact nesting, so select the requested stream first and then accept
// any FPS/frame-rate field containing numeric values.
func (c *Client) GetMaxFPS(ch int, s Stream, width, height int, codec string) (int, error) {
	table, err := c.getTable("EncodeCapability")
	if err != nil {
		return 0, err
	}
	fmtObj, err := formatOf(table, ch, s)
	if err != nil {
		return 0, err
	}
	max := maxFPSValue(fmtObj, false)
	if max <= 0 {
		return 0, fmt.Errorf("EncodeCapability has no FPS value for channel %d stream %d", ch, s)
	}
	return max, nil
}

func maxFPSValue(v any, fpsField bool) int {
	max := 0
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			lk := strings.ToLower(k)
			m := maxFPSValue(child, fpsField || strings.Contains(lk, "fps") || strings.Contains(lk, "framerate"))
			if m > max {
				max = m
			}
		}
	case []any:
		for _, child := range x {
			if m := maxFPSValue(child, fpsField); m > max {
				max = m
			}
		}
	case float64:
		if fpsField {
			max = int(x)
		}
	case int:
		if fpsField {
			max = x
		}
	case json.Number:
		if fpsField {
			i, _ := x.Int64()
			max = int(i)
		}
	case string:
		if fpsField {
			for _, part := range strings.FieldsFunc(x, func(r rune) bool { return r < '0' || r > '9' }) {
				if n, err := strconv.Atoi(part); err == nil && n > max {
					max = n
				}
			}
		}
	}
	return max
}

// SetCodec sets the video codec/profile for a stream. compression is the Dahua
// Video.Compression value (e.g. "H.265", "H.264", "H.264H" = High profile,
// "H.264B" = Baseline, "MJPG"). If profile is non-empty it is also written to
// Video.Profile ("Main"/"High"/"Baseline"). The device rejects unsupported
// codecs with an explicit error, which callers surface in the progress log.
func (c *Client) SetCodec(ch int, s Stream, compression, profile string) error {
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
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
	return c.setChannelTable("Encode", ch, chObj)
}

// SetAudioAAC forces the stream's audio codec to AAC and enables audio.
func (c *Client) SetAudioAAC(ch int, s Stream) error {
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return err
	}
	subMap(fmtObj, "Audio")["Compression"] = "AAC"
	fmtObj["AudioEnable"] = true
	return c.setChannelTable("Encode", ch, chObj)
}

// SetGOP sets the I-frame interval (frames) for a stream, using GET-modify-SET
// on the Encode config.
func (c *Client) SetGOP(ch int, s Stream, gop int) error {
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return err
	}
	subMap(fmtObj, "Video")["GOP"] = gop
	return c.setChannelTable("Encode", ch, chObj)
}

// SetBitrate sets the video bitrate (Kbps) and, when mode is non-empty, the
// bitrate control mode ("VBR"/"CBR") for a stream, using GET-modify-SET on
// the Encode config.
func (c *Client) SetBitrate(ch int, s Stream, kbps int, mode string) error {
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return err
	}
	video := subMap(fmtObj, "Video")
	video["BitRate"] = kbps
	if mode != "" {
		video["BitRateControl"] = mode
	}
	return c.setChannelTable("Encode", ch, chObj)
}

// SetSmartCodec toggles Dahua "Smart Codec" (H.264+/H.265+) for a channel via
// the SmartEncode config. Smart codec is a per-channel switch (not per-stream).
func (c *Client) SetSmartCodec(ch int, on bool) error {
	chObj, err := c.getChannelTable("SmartEncode", ch)
	if err != nil {
		return err
	}
	chObj["Enable"] = on
	return c.setChannelTable("SmartEncode", ch, chObj)
}

// fillFromFormat copies the Video/Audio fields out of a MainFormat/ExtraFormat
// entry (fmtObj, as returned by formatOf) into info. Shared by ProbeAll and
// GetStreamInfo so the two parses can't drift.
func fillFromFormat(info *StreamInfo, fmtObj map[string]any) {
	if v, ok := fmtObj["Video"].(map[string]any); ok {
		info.Width = toInt(v["Width"])
		info.Height = toInt(v["Height"])
		info.FPS = toInt(v["FPS"])
		info.Compression, _ = v["Compression"].(string)
		info.Profile, _ = v["Profile"].(string)
		info.GOP = toInt(v["GOP"])
		info.BitRate = toInt(v["BitRate"])
		info.BitRateControl, _ = v["BitRateControl"].(string)
	}
	if a, ok := fmtObj["Audio"].(map[string]any); ok {
		info.AudioCodec, _ = a["Compression"].(string)
	}
	info.AudioEnable, _ = fmtObj["AudioEnable"].(bool)
}

// ProbeAll reads every channel's main + sub streams in a single pass (fetches
// the Encode and SmartEncode configs once), so an NVR's whole camera list comes
// back in two requests. Channel in the result is 1-based (camera number).
func (c *Client) ProbeAll() ([]StreamInfo, error) {
	table, err := c.getTable("Encode")
	if err != nil {
		return nil, err
	}
	smart, _ := c.getTable("SmartEncode")   // best-effort
	titles, _ := c.getTable("ChannelTitle") // best-effort
	widgets, _ := c.getTable("VideoWidget") // best-effort

	var out []StreamInfo
	for ci := 0; ci < len(table); ci++ {
		name := ""
		if ci < len(titles) {
			if chObj, ok := titles[ci].(map[string]any); ok {
				name, _ = chObj["Name"].(string)
			}
		}
		var osdLines []string
		if ci < len(widgets) {
			if slots, err := customTitleSlots(widgets, ci); err == nil {
				osdLines = make([]string, len(slots))
				for i, s := range slots {
					if obj, ok := s.(map[string]any); ok {
						osdLines[i], _ = obj["Text"].(string)
					}
				}
			}
		}
		for _, s := range []Stream{StreamMain, StreamSub1, StreamSub2} {
			fmtObj, err := formatOf(table, ci, s)
			if err != nil {
				continue
			}
			info := StreamInfo{Channel: ci + 1, Stream: s, Name: name, OSDLines: osdLines}
			fillFromFormat(&info, fmtObj)
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
	chObj, err := c.getChannelTable("Encode", ch)
	if err != nil {
		return info, err
	}
	fmtObj, err := formatOfChannel(chObj, s)
	if err != nil {
		return info, err
	}
	fillFromFormat(&info, fmtObj)

	// Smart codec is a separate config.
	if smartObj, err := c.getChannelTable("SmartEncode", ch); err == nil {
		info.SmartCodec, _ = smartObj["Enable"].(bool)
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
