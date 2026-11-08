package xiongmai

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// StreamInfo holds normalized encode parameters for one Xiongmai channel/stream.
type StreamInfo struct {
	Channel        int
	Stream         int // 0 = MainFormat, 1 = ExtraFormat
	Width          int
	Height         int
	ResolutionCode string
	FPS            int
	Compression    string
	Profile        string
	AudioCodec     string
	AudioEnable    bool
	SmartCodec     bool
	GOP            int // Normalized in frames (GOP_sec * FPS when firmware stores seconds <= 15)
	GOPRaw         int // Raw firmware GOP field
	BitRate        int // Kbps
	BitRateControl string
	Name           string
	OSDLines       []string
}

type resPair struct {
	Code string
	W    int
	H    int
}

// Canonical Xiongmai resolution table from Web/html/cfg/System_Encode.js:130-145.
var xmResTable = []resPair{
	{"12M", 4000, 3000},
	{"4K", 3840, 2160},
	{"8M", 3264, 2448},
	{"6M", 3072, 2048},
	{"5M", 2560, 1920},
	{"4M", 2560, 1440},
	{"3M", 2048, 1536},
	{"3M_N", 1920, 1536},
	{"WUXGA", 1920, 1200},
	{"1080P", 1920, 1080},
	{"2_5M", 1872, 1408},
	{"UXGA", 1600, 1200},
	{"1_3M", 1280, 960},
	{"720P", 1280, 720},
	{"1080N", 944, 1080},
	{"WSVGA", 1024, 576},
	{"650TVL", 928, 576},
	{"D1", 704, 576},
	{"HD1", 704, 288},
	{"VGA", 640, 480},
	{"NHD", 640, 360},
	{"BCIF", 352, 576},
	{"CIF", 352, 288},
	{"QVGA", 320, 240},
	{"QCIF", 176, 144},
}

// ParseResolutionCode maps a Xiongmai resolution token ("1080P", "4M", "D1", ...)
// to pixel dimensions (Width, Height).
func ParseResolutionCode(code string) (int, int) {
	upper := strings.ToUpper(strings.TrimSpace(code))
	for _, r := range xmResTable {
		if strings.EqualFold(r.Code, upper) {
			return r.W, r.H
		}
	}
	// Handle explicit "WxH" strings if returned by newer firmwares.
	var w, h int
	if _, err := fmt.Sscanf(upper, "%dX%d", &w, &h); err == nil && w > 0 && h > 0 {
		return w, h
	}
	return 0, 0
}

// FormatResolutionCode maps pixel dimensions (Width, Height) to Xiongmai's
// canonical Resolution string ("1080P", "4M", "720P", "D1", etc.).
func FormatResolutionCode(w, h int) (string, bool) {
	for _, r := range xmResTable {
		if r.W == w && r.H == h {
			return r.Code, true
		}
	}
	// Common NTSC variants
	if w == 704 && h == 480 {
		return "D1", true
	}
	if w == 352 && h == 240 {
		return "CIF", true
	}
	if w == 2880 && h == 1620 {
		return "5M", true
	}
	if w == 2304 && h == 1296 {
		return "3M", true
	}
	return "", false
}

// GOPSecondsFromFrames converts kspcam's frame-based GOP (e.g. 50 frames @ 25fps)
// into Xiongmai's seconds-based GOP (clamped to 1..12s).
func GOPSecondsFromFrames(gopFrames, fps int) int {
	if gopFrames <= 0 {
		return 2
	}
	if gopFrames <= 12 && fps > 12 {
		// Caller already passed seconds directly.
		return gopFrames
	}
	if fps <= 0 {
		fps = 25
	}
	sec := int(math.Round(float64(gopFrames) / float64(fps)))
	if sec < 1 {
		sec = 1
	}
	if sec > 12 {
		sec = 12
	}
	return sec
}

// GOPFramesFromRaw normalizes Xiongmai's raw Video.GOP (usually 1..12 seconds)
// into frame count so Probe/Tri-state verification matches Dahua/Hik/Raysharp.
func GOPFramesFromRaw(rawGOP, fps int) int {
	if rawGOP <= 0 {
		return 0
	}
	if fps <= 0 {
		fps = 25
	}
	if rawGOP <= 15 {
		return rawGOP * fps
	}
	return rawGOP
}

func normalizeAudioCodec(raw string) string {
	u := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case strings.Contains(u, "AAC"):
		return "AAC"
	case strings.Contains(u, "ALAW") || u == "G711A" || u == "G.711A":
		return "G.711A"
	case strings.Contains(u, "ULAW") || u == "G711U" || u == "G.711U":
		return "G.711U"
	case u == "":
		return "G.711A"
	default:
		return raw
	}
}

func normalizeVideoCodec(raw string) string {
	u := strings.ToUpper(strings.TrimSpace(raw))
	switch u {
	case "H.265", "H265", "HEVC", "H.265+", "H.265X":
		return "H.265"
	case "H.264", "H264", "H.264+", "H.264H", "H.264B":
		return "H.264"
	case "JPEG", "MJPEG", "MJPG":
		return "MJPG"
	default:
		return raw
	}
}

func profileNameFromInt(p int) string {
	switch p {
	case 1:
		return "Baseline"
	case 2:
		return "Main"
	case 3:
		return "High"
	default:
		return ""
	}
}

func profileIntFromName(name string) int {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "baseline":
		return 1
	case "main":
		return 2
	case "high":
		return 3
	default:
		return 0
	}
}

// encodeConfigTarget determines whether channel (0-based) lives in local
// "AVEnc.Encode" (IPC / Analog DVR channel) or "bypass@AVEnc.Encode.[nDig]"
// (digital IP channel on an HVR/NVR).
func (c *Client) encodeConfigTarget(ctx context.Context, channel int) (cfgName string, elemIdx int, isBypass bool) {
	sys, err := c.GetSystemInfo(ctx)
	if err == nil && sys.DigChannel > 0 && channel >= sys.VideoInChannel {
		nDig := channel - sys.VideoInChannel
		return fmt.Sprintf("bypass@AVEnc.Encode.[%d]", nDig), 0, true
	}
	return "AVEnc.Encode", channel, false
}

// fetchEncodeChannel reads the raw encode table and extracts the map for channel,
// returning (fullPayload, channelEntryMap, configName, err).
func (c *Client) fetchEncodeChannel(ctx context.Context, channel int) (any, map[string]any, string, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	cfgName, idx, isBypass := c.encodeConfigTarget(ctx, channel)
	resp, err := tr.GetConfig(ctx, cfgName)
	if err != nil && isBypass {
		// Fallback to top-level AVEnc.Encode if bypass@ is unsupported on this NVR.
		cfgName = "AVEnc.Encode"
		idx = channel
		resp, err = tr.GetConfig(ctx, cfgName)
	}
	if err != nil {
		return nil, nil, cfgName, err
	}
	rawTable := resp[cfgName]
	switch tbl := rawTable.(type) {
	case []any:
		if idx < 0 || idx >= len(tbl) {
			return tbl, nil, cfgName, fmt.Errorf("xiongmai %s: channel %d out of range (len=%d)", cfgName, channel, len(tbl))
		}
		m, ok := tbl[idx].(map[string]any)
		if !ok {
			return tbl, nil, cfgName, fmt.Errorf("xiongmai %s[%d]: unexpected type %T", cfgName, idx, tbl[idx])
		}
		return tbl, m, cfgName, nil
	case map[string]any:
		return tbl, tbl, cfgName, nil
	default:
		return nil, nil, cfgName, fmt.Errorf("xiongmai %s: missing table in response", cfgName)
	}
}

// extractFormatMap handles both Xiongmai representations of MainFormat / ExtraFormat:
//   - Sofia IPCs: `"MainFormat": { ... }`, `"ExtraFormat": [ { ... }, ... ]`
//   - Some firmwares: `"MainFormat": [ { ... } ]` or `"ExtraFormat": { ... }`
func extractFormatMap(chEntry map[string]any, stream int) (map[string]any, error) {
	key := "MainFormat"
	subIdx := 0
	if stream > 0 {
		key = "ExtraFormat"
		subIdx = stream - 1
	}
	raw, ok := chEntry[key]
	if !ok {
		return nil, fmt.Errorf("xiongmai encode: missing %s", key)
	}
	switch v := raw.(type) {
	case map[string]any:
		return v, nil
	case []any:
		if len(v) == 0 {
			return nil, fmt.Errorf("xiongmai encode: empty %s array", key)
		}
		if subIdx >= len(v) {
			subIdx = 0
		}
		m, ok := v[subIdx].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("xiongmai encode: invalid %s[%d]", key, subIdx)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("xiongmai encode: unsupported %s type %T", key, raw)
	}
}

func parseStreamFromEntry(chEntry map[string]any, channel1Based, stream int, smart bool) (StreamInfo, error) {
	fmtMap, err := extractFormatMap(chEntry, stream)
	if err != nil {
		return StreamInfo{}, err
	}
	vid, _ := fmtMap["Video"].(map[string]any)
	aud, _ := fmtMap["Audio"].(map[string]any)

	resCode := stringFieldAny(vid, "Resolution")
	w, h := ParseResolutionCode(resCode)
	if w == 0 {
		w = intFieldAny(vid, "Width")
		h = intFieldAny(vid, "Height")
	}
	fps := intFieldAny(vid, "FPS")
	rawGOP := intFieldAny(vid, "GOP")
	comp := normalizeVideoCodec(stringFieldAny(vid, "Compression"))

	prof := ""
	if sp, ok := chEntry["EncodeStaticParam"].([]any); ok && len(sp) > 0 {
		if sp0, ok := sp[0].(map[string]any); ok {
			prof = profileNameFromInt(intFieldAny(sp0, "Profile"))
		}
	}

	return StreamInfo{
		Channel:        channel1Based,
		Stream:         stream,
		Width:          w,
		Height:         h,
		ResolutionCode: resCode,
		FPS:            fps,
		Compression:    comp,
		Profile:        prof,
		AudioCodec:     normalizeAudioCodec(stringFieldAny(aud, "Compression")),
		AudioEnable:    boolFieldAny(fmtMap, "AudioEnable"),
		SmartCodec:     smart,
		GOP:            GOPFramesFromRaw(rawGOP, fps),
		GOPRaw:         rawGOP,
		BitRate:        intFieldAny(vid, "BitRate"),
		BitRateControl: stringFieldAny(vid, "BitRateControl"),
	}, nil
}

// GetSmartCodec reads AVEnc.SmartH264 / AVEnc.SmartH264V2 for channel (0-based).
func (c *Client) GetSmartCodec(ctx context.Context, channel int) (bool, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return false, err
	}
	if resp, err := tr.GetConfig(ctx, "AVEnc.SmartH264"); err == nil {
		if arr, ok := resp["AVEnc.SmartH264"].([]any); ok && channel >= 0 && channel < len(arr) {
			if m, ok := arr[channel].(map[string]any); ok {
				return boolFieldAny(m, "SmartH264"), nil
			}
		}
	}
	if resp, err := tr.GetConfig(ctx, "AVEnc.SmartH264V2"); err == nil {
		if arr, ok := resp["AVEnc.SmartH264V2"].([]any); ok && channel >= 0 && channel < len(arr) {
			if m, ok := arr[channel].(map[string]any); ok {
				return intFieldAny(m, "SmartH264Plus") > 0 || boolFieldAny(m, "SmartH264"), nil
			}
		}
	}
	return false, nil
}

// SetSmartCodec updates AVEnc.SmartH264 or AVEnc.SmartH264V2 for channel (0-based).
func (c *Client) SetSmartCodec(ctx context.Context, channel int, enable bool) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	if resp, err := tr.GetConfig(ctx, "AVEnc.SmartH264"); err == nil {
		if arr, ok := resp["AVEnc.SmartH264"].([]any); ok && channel >= 0 && channel < len(arr) {
			if m, ok := arr[channel].(map[string]any); ok {
				m["SmartH264"] = enable
				_, werr := tr.SetConfig(ctx, "AVEnc.SmartH264", arr)
				return werr
			}
		}
	}
	if resp, err := tr.GetConfig(ctx, "AVEnc.SmartH264V2"); err == nil {
		if arr, ok := resp["AVEnc.SmartH264V2"].([]any); ok && channel >= 0 && channel < len(arr) {
			if m, ok := arr[channel].(map[string]any); ok {
				if _, hasPlus := m["SmartH264Plus"]; hasPlus {
					if enable {
						m["SmartH264Plus"] = 1
					} else {
						m["SmartH264Plus"] = 0
					}
				} else {
					m["SmartH264"] = enable
				}
				_, werr := tr.SetConfig(ctx, "AVEnc.SmartH264V2", arr)
				return werr
			}
		}
	}
	if !enable {
		// Firmware has no SmartH264 table -> SmartCodec is already permanently off.
		return nil
	}
	return fmt.Errorf("%w: AVEnc.SmartH264 not supported", ErrUnsupported)
}

// GetStreamInfo reads normalized stream info for one 0-based channel and stream (0=main, 1=sub).
func (c *Client) GetStreamInfo(ctx context.Context, channel, stream int) (StreamInfo, error) {
	_, chEntry, _, err := c.fetchEncodeChannel(ctx, channel)
	if err != nil {
		return StreamInfo{}, err
	}
	smart, _ := c.GetSmartCodec(ctx, channel)
	si, err := parseStreamFromEntry(chEntry, channel+1, stream, smart)
	if err != nil {
		return StreamInfo{}, err
	}
	return si, nil
}

// ProbeAll reads all channels' Main and Sub1 streams plus channel titles and OSD lines.
func (c *Client) ProbeAll(ctx context.Context) ([]StreamInfo, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	titles, _ := c.GetChannelTitles(ctx)

	// Try reading global AVEnc.Encode first.
	var out []StreamInfo
	if resp, err := tr.GetConfig(ctx, "AVEnc.Encode"); err == nil {
		if arr, ok := resp["AVEnc.Encode"].([]any); ok && len(arr) > 0 {
			for chIdx, raw := range arr {
				chMap, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				smart, _ := c.GetSmartCodec(ctx, chIdx)
				for _, s := range []int{0, 1} {
					si, serr := parseStreamFromEntry(chMap, chIdx+1, s, smart)
					if serr != nil {
						continue
					}
					if s == 0 {
						if chIdx < len(titles) {
							si.Name = titles[chIdx]
						}
						if len(arr) == 1 {
							if _, lines, _, _, oerr := c.GetChannelOSD(ctx, chIdx); oerr == nil {
								si.OSDLines = lines
							}
						}
					}
					out = append(out, si)
				}
			}
		}
	}

	// On pure NVR/HVRs where VideoInChannel == 0 and DigChannel > 0, channels may
	// only be exposed via bypass@AVEnc.Encode.[nDig].
	if len(out) == 0 {
		sys, serr := c.GetSystemInfo(ctx)
		if serr != nil {
			return nil, serr
		}
		total := sys.TotalChannels()
		if total <= 0 {
			total = 1
		}
		for ch := 0; ch < total; ch++ {
			for _, s := range []int{0, 1} {
				if si, gerr := c.GetStreamInfo(ctx, ch, s); gerr == nil {
					if s == 0 && ch < len(titles) {
						si.Name = titles[ch]
					}
					out = append(out, si)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("xiongmai: ProbeAll found no readable encode streams")
	}
	return out, nil
}

func (c *Client) mutateEncodeStream(ctx context.Context, channel, stream int, fn func(fmtMap, vidMap map[string]any, chEntry map[string]any) error) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	fullTbl, chEntry, cfgName, err := c.fetchEncodeChannel(ctx, channel)
	if err != nil {
		return err
	}
	fmtMap, err := extractFormatMap(chEntry, stream)
	if err != nil {
		return err
	}
	vidMap, ok := fmtMap["Video"].(map[string]any)
	if !ok {
		return fmt.Errorf("xiongmai %s: missing Video object", cfgName)
	}
	if err := fn(fmtMap, vidMap, chEntry); err != nil {
		return err
	}
	_, err = tr.SetConfig(ctx, cfgName, fullTbl)
	return err
}

func (c *Client) SetCodec(ctx context.Context, channel, stream int, codec, profile string) error {
	xmCodec := normalizeVideoCodec(codec)
	if xmCodec == "MJPG" {
		xmCodec = "JPEG"
	}
	return c.mutateEncodeStream(ctx, channel, stream, func(_, vidMap map[string]any, chEntry map[string]any) error {
		vidMap["Compression"] = xmCodec
		if pInt := profileIntFromName(profile); pInt > 0 {
			if sp, ok := chEntry["EncodeStaticParam"].([]any); ok && len(sp) > 0 {
				if sp0, ok := sp[0].(map[string]any); ok {
					sp0["Profile"] = pInt
				}
			}
		}
		return nil
	})
}

func (c *Client) SetResolution(ctx context.Context, channel, stream, width, height int) error {
	code, ok := FormatResolutionCode(width, height)
	if !ok {
		return fmt.Errorf("xiongmai: unsupported resolution %dx%d", width, height)
	}
	return c.mutateEncodeStream(ctx, channel, stream, func(_, vidMap map[string]any, _ map[string]any) error {
		vidMap["Resolution"] = code
		return nil
	})
}

func (c *Client) SetFPS(ctx context.Context, channel, stream, fps int) error {
	if fps <= 0 {
		return fmt.Errorf("xiongmai: invalid FPS %d", fps)
	}
	return c.mutateEncodeStream(ctx, channel, stream, func(_, vidMap map[string]any, _ map[string]any) error {
		vidMap["FPS"] = fps
		return nil
	})
}

func (c *Client) SetGOP(ctx context.Context, channel, stream, gopFrames int) error {
	return c.mutateEncodeStream(ctx, channel, stream, func(_, vidMap map[string]any, _ map[string]any) error {
		fps := intFieldAny(vidMap, "FPS")
		vidMap["GOP"] = GOPSecondsFromFrames(gopFrames, fps)
		return nil
	})
}

func (c *Client) SetBitrate(ctx context.Context, channel, stream, kbps int, mode string) error {
	return c.mutateEncodeStream(ctx, channel, stream, func(_, vidMap map[string]any, _ map[string]any) error {
		if kbps > 0 {
			vidMap["BitRate"] = kbps
		}
		if mode != "" {
			vidMap["BitRateControl"] = strings.ToUpper(strings.TrimSpace(mode))
		}
		return nil
	})
}

func (c *Client) SetAudioAAC(ctx context.Context, channel, stream int) error {
	return c.mutateEncodeStream(ctx, channel, stream, func(fmtMap, _ map[string]any, _ map[string]any) error {
		fmtMap["AudioEnable"] = true
		aud, ok := fmtMap["Audio"].(map[string]any)
		if !ok {
			aud = map[string]any{}
			fmtMap["Audio"] = aud
		}
		aud["Compression"] = "AAC"
		return nil
	})
}

// GetFPSCapability queries EncodeCapability (or bypass@EncodeCapability.[nDig])
// with a safe 25fps fallback if the firmware omits capability details.
func (c *Client) GetFPSCapability(ctx context.Context, channel, stream, _, _ int, _ string) (curFPS, maxFPS int, source string, err error) {
	curFPS = 25
	if si, serr := c.GetStreamInfo(ctx, channel, stream); serr == nil && si.FPS > 0 {
		curFPS = si.FPS
	}
	tr, terr := c.Transport(ctx)
	if terr != nil {
		return curFPS, 25, "fallback", nil
	}
	if resp, aerr := tr.GetAbility(ctx, "EncodeCapability"); aerr == nil {
		if capMap, ok := resp["EncodeCapability"].(map[string]any); ok {
			if m := intFieldAny(capMap, "MaxFPS"); m > 0 {
				return curFPS, m, "capability", nil
			}
			if arr, ok := capMap["ImageSizePerChannel"].([]any); ok && len(arr) > 0 {
				return curFPS, 25, "capability", nil
			}
		}
	}
	if curFPS > 25 {
		return curFPS, curFPS, "fallback", nil
	}
	return curFPS, 25, "fallback", nil
}
