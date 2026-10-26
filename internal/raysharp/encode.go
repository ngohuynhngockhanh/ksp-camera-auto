package raysharp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Stream enum matches camera.Stream: 0=main, 1=sub1, 2=sub2 (mobilestream).
const (
	StreamMain = 0
	StreamSub1 = 1
	StreamSub2 = 2
)

// StreamInfo holds the normalized encode parameters for one channel/stream.
type StreamInfo struct {
	Channel        int      `json:"channel"` // 0-based
	Stream         int      `json:"stream"`  // 0=main, 1=sub1, 2=sub2
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	FPS            int      `json:"fps"`
	Compression    string   `json:"compression"` // "H.265", "H.264"
	Profile        string   `json:"profile"`
	AudioCodec     string   `json:"audioCodec"`
	AudioEnable    bool     `json:"audioEnable"`
	SmartCodec     bool     `json:"smartCodec"`
	GOP            int      `json:"gop"`
	BitRate        int      `json:"bitRate"`
	BitRateControl string   `json:"bitRateControl"`
	Name           string   `json:"name,omitempty"`
	OSDLines       []string `json:"osdLines,omitempty"`
}

func streamAPIPath(stream int) string {
	switch stream {
	case StreamSub1:
		return "StreamConfig/SubStream"
	case StreamSub2:
		return "StreamConfig/MobileStream"
	default:
		return "StreamConfig/MainStream"
	}
}

// ParseResolution splits Raysharp's "2880 x 1620" or "1920x1080" into width and height.
func ParseResolution(res string) (int, int) {
	res = strings.ToLower(strings.TrimSpace(res))
	parts := strings.Split(res, "x")
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return w, h
}

// FormatResolution formats width and height into Raysharp's canonical "W x H" string.
func FormatResolution(width, height int) string {
	return fmt.Sprintf("%d x %d", width, height)
}

// ParseEncodeType splits Raysharp's video_encode_type ("H.265+", "H.265", "H.264+", "H.264")
// into base codec and SmartCodec flag.
func ParseEncodeType(enc string) (codec string, smart bool) {
	enc = strings.TrimSpace(enc)
	if strings.HasSuffix(enc, "+") {
		return strings.TrimSuffix(enc, "+"), true
	}
	return enc, false
}

// FormatEncodeType combines base codec ("H.265", "H.264") and SmartCodec flag
// into Raysharp's video_encode_type string ("H.265+", "H.265", etc.).
func FormatEncodeType(codec string, smart bool) string {
	base, _ := ParseEncodeType(codec)
	if strings.HasPrefix(strings.ToUpper(base), "H.264") {
		base = "H.264"
	} else if strings.HasPrefix(strings.ToUpper(base), "H.265") || strings.EqualFold(base, "HEVC") {
		base = "H.265"
	}
	if smart && (base == "H.264" || base == "H.265") {
		return base + "+"
	}
	return base
}

var predefinedBitrates = map[int]bool{
	32: true, 48: true, 64: true, 80: true, 96: true, 128: true, 160: true,
	192: true, 224: true, 256: true, 320: true, 384: true, 448: true, 512: true,
	640: true, 768: true, 896: true, 1024: true, 1280: true, 1536: true,
	1792: true, 2048: true, 3072: true, 4096: true, 5120: true, 6144: true,
	8192: true, 10240: true, 12288: true, 16384: true,
}

type rawStreamChannel struct {
	Reason           string `json:"reason"`
	Resolution       string `json:"resolution"`
	FPS              int    `json:"fps"`
	Bitrate          int    `json:"bitrate"`
	BitrateMode      string `json:"bitrate_mode"`
	CustomBitrate    int    `json:"custom_bitrate"`
	VideoEncodeType  string `json:"video_encode_type"`
	VideoEncodeLevel string `json:"video_encode_level"`
	BitrateControl   string `json:"bitrate_control"`
	Audio            bool   `json:"audio"`
	IFrameInterval   int    `json:"i_frame_interval"`
}

// GetStreamInfo reads back one channel's encode settings for the given stream.
func (c *Client) GetStreamInfo(ctx context.Context, channel, stream int) (StreamInfo, error) {
	path := streamAPIPath(stream) + "/Get"
	env, err := c.Call(ctx, path, map[string]any{})
	if err != nil {
		return StreamInfo{}, err
	}
	var payload struct {
		ChannelInfo map[string]rawStreamChannel `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return StreamInfo{}, fmt.Errorf("raysharp %s decode: %w", path, err)
	}
	chKey := ChannelKey(channel)
	chData, ok := payload.ChannelInfo[chKey]
	if !ok {
		return StreamInfo{}, fmt.Errorf("raysharp %s: channel %s not found", path, chKey)
	}
	if chData.Reason != "" && chData.Resolution == "" {
		return StreamInfo{}, fmt.Errorf("raysharp %s %s: %s", path, chKey, chData.Reason)
	}
	audioCodec := "G.711A"
	if ac, err := c.GetAudioCodec(ctx, channel); err == nil && ac != "" {
		audioCodec = ac
	}
	return toStreamInfo(channel, stream, chData, audioCodec), nil
}

func toStreamInfo(channel, stream int, chData rawStreamChannel, audioCodec string) StreamInfo {
	w, h := ParseResolution(chData.Resolution)
	codec, smart := ParseEncodeType(chData.VideoEncodeType)
	br := chData.Bitrate
	if strings.EqualFold(chData.BitrateMode, "UserDefined") && chData.CustomBitrate > 0 {
		br = chData.CustomBitrate
	} else if br == 0 && chData.CustomBitrate > 0 {
		br = chData.CustomBitrate
	}
	prof := chData.VideoEncodeLevel
	prof = strings.TrimSuffix(prof, "Profile")
	if prof == "" {
		prof = "Main"
	}
	return StreamInfo{
		Channel:        channel,
		Stream:         stream,
		Width:          w,
		Height:         h,
		FPS:            chData.FPS,
		Compression:    codec,
		Profile:        prof,
		AudioCodec:     audioCodec,
		AudioEnable:    chData.Audio,
		SmartCodec:     smart,
		GOP:            chData.IFrameInterval,
		BitRate:        br,
		BitRateControl: chData.BitrateControl,
	}
}

// GetAudioCodec reads the configured audio codec for a channel from /API/DeviceConfig/Audio/Get.
func (c *Client) GetAudioCodec(ctx context.Context, channel int) (string, error) {
	env, err := c.Call(ctx, "DeviceConfig/Audio/Get", map[string]any{})
	if err != nil {
		return "", err
	}
	var p struct {
		ChannelInfo map[string]struct {
			AudioType string `json:"audio_type"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &p); err != nil {
		return "", err
	}
	ch, ok := p.ChannelInfo[ChannelKey(channel)]
	if !ok || ch.AudioType == "" {
		return "G.711A", nil
	}
	switch strings.ToUpper(ch.AudioType) {
	case "G711A", "G.711A":
		return "G.711A", nil
	case "G711U", "G.711U":
		return "G.711U", nil
	case "AAC":
		return "AAC", nil
	default:
		return ch.AudioType, nil
	}
}

// ProbeAll fetches main and sub1 StreamInfo for all online/configured channels,
// enriching each channel's main stream with its channel name and OSD lines.
func (c *Client) ProbeAll(ctx context.Context) ([]StreamInfo, error) {
	mainEnv, err := c.Call(ctx, "StreamConfig/MainStream/Get", map[string]any{})
	if err != nil {
		return nil, err
	}
	var mainPayload struct {
		ChannelInfo map[string]rawStreamChannel `json:"channel_info"`
	}
	if err := json.Unmarshal(mainEnv.Data, &mainPayload); err != nil {
		return nil, fmt.Errorf("raysharp MainStream/Get decode: %w", err)
	}

	subPayload := struct {
		ChannelInfo map[string]rawStreamChannel `json:"channel_info"`
	}{}
	if subEnv, err := c.Call(ctx, "StreamConfig/SubStream/Get", map[string]any{}); err == nil {
		_ = json.Unmarshal(subEnv.Data, &subPayload)
	}

	osdNames := map[int]string{}
	osdLines := map[int][]string{}
	if osdEnv, err := c.Call(ctx, "ChannelConfig/OSD/Get", map[string]any{}); err == nil {
		var osdP struct {
			ChannelInfo map[string]struct {
				Name struct {
					Show bool   `json:"show"`
					Text string `json:"text"`
				} `json:"name"`
				Alarm struct {
					Show bool   `json:"show"`
					Text string `json:"text"`
				} `json:"alarm"`
			} `json:"channel_info"`
		}
		if json.Unmarshal(osdEnv.Data, &osdP) == nil {
			for k, v := range osdP.ChannelInfo {
				if chIdx, ok := ParseChannelKey(k); ok {
					osdNames[chIdx] = v.Name.Text
					var lines []string
					if v.Name.Text != "" {
						lines = append(lines, v.Name.Text)
					}
					if v.Alarm.Text != "" {
						lines = append(lines, v.Alarm.Text)
					}
					osdLines[chIdx] = lines
				}
			}
		}
	}

	audioCodecs := map[int]string{}
	if audEnv, err := c.Call(ctx, "DeviceConfig/Audio/Get", map[string]any{}); err == nil {
		var audP struct {
			ChannelInfo map[string]struct {
				AudioType string `json:"audio_type"`
			} `json:"channel_info"`
		}
		if json.Unmarshal(audEnv.Data, &audP) == nil {
			for k, v := range audP.ChannelInfo {
				if chIdx, ok := ParseChannelKey(k); ok && v.AudioType != "" {
					switch strings.ToUpper(v.AudioType) {
					case "G711A":
						audioCodecs[chIdx] = "G.711A"
					case "G711U":
						audioCodecs[chIdx] = "G.711U"
					default:
						audioCodecs[chIdx] = v.AudioType
					}
				}
			}
		}
	}

	var channels []int
	for k, v := range mainPayload.ChannelInfo {
		if v.Reason != "" && v.Resolution == "" {
			continue
		}
		if chIdx, ok := ParseChannelKey(k); ok {
			channels = append(channels, chIdx)
		}
	}
	sort.Ints(channels)

	var out []StreamInfo
	for _, chIdx := range channels {
		chKey := ChannelKey(chIdx)
		ac := audioCodecs[chIdx]
		if ac == "" {
			ac = "G.711A"
		}
		if mData, ok := mainPayload.ChannelInfo[chKey]; ok {
			info := toStreamInfo(chIdx, StreamMain, mData, ac)
			info.Name = osdNames[chIdx]
			info.OSDLines = osdLines[chIdx]
			out = append(out, info)
		}
		if sData, ok := subPayload.ChannelInfo[chKey]; ok && (sData.Reason == "" || sData.Resolution != "") {
			info := toStreamInfo(chIdx, StreamSub1, sData, ac)
			out = append(out, info)
		}
	}
	return out, nil
}

// mutateStreamChannel performs a GET-Modify-SET cycle on MainStream or SubStream
// for one channel, preserving all other fields on that channel.
func (c *Client) mutateStreamChannel(ctx context.Context, channel, stream int, mutate func(chMap map[string]any) error) error {
	base := streamAPIPath(stream)
	env, err := c.Call(ctx, base+"/Get", map[string]any{})
	if err != nil {
		return err
	}
	var payload struct {
		ChannelInfo map[string]map[string]any `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return fmt.Errorf("raysharp %s/Get decode: %w", base, err)
	}
	chKey := ChannelKey(channel)
	chMap, ok := payload.ChannelInfo[chKey]
	if !ok {
		return fmt.Errorf("raysharp %s: channel %s not found", base, chKey)
	}
	if reason, _ := chMap["reason"].(string); reason != "" && chMap["resolution"] == nil {
		return fmt.Errorf("raysharp %s %s: %s", base, chKey, reason)
	}
	if err := mutate(chMap); err != nil {
		return err
	}
	// Send only the modified channel to avoid resetting other NVR channels.
	setReq := map[string]any{
		"channel_info": map[string]any{
			chKey: chMap,
		},
	}
	_, err = c.Call(ctx, base+"/Set", setReq)
	return err
}

// SetResolution sets a channel/stream's resolution ("W x H").
func (c *Client) SetResolution(ctx context.Context, channel, stream, width, height int) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		res := FormatResolution(width, height)
		chMap["resolution"] = res
		if _, hasEtr := chMap["etr_resolution"]; hasEtr {
			chMap["etr_resolution"] = res
		}
		return nil
	})
}

// SetCodec sets video_encode_type (preserving current SmartCodec suffix unless specified).
func (c *Client) SetCodec(ctx context.Context, channel, stream int, codec, profile string) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		cur, _ := chMap["video_encode_type"].(string)
		_, curSmart := ParseEncodeType(cur)
		targetCodec, explicitSmart := ParseEncodeType(codec)
		smart := curSmart || explicitSmart
		encType := FormatEncodeType(targetCodec, smart)
		chMap["video_encode_type"] = encType
		if _, hasEtr := chMap["etr_video_encode_type"]; hasEtr {
			chMap["etr_video_encode_type"] = encType
		}
		if profile != "" {
			lvl := profile
			if !strings.HasSuffix(lvl, "Profile") {
				lvl += "Profile"
			}
			if _, hasLvl := chMap["video_encode_level"]; hasLvl {
				chMap["video_encode_level"] = lvl
			}
		}
		return nil
	})
}

// SetSmartCodec toggles the "+" suffix on video_encode_type (e.g. "H.265" <-> "H.265+").
func (c *Client) SetSmartCodec(ctx context.Context, channel, stream int, enable bool) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		cur, _ := chMap["video_encode_type"].(string)
		base, _ := ParseEncodeType(cur)
		if base == "" {
			base = "H.265"
		}
		encType := FormatEncodeType(base, enable)
		chMap["video_encode_type"] = encType
		if _, hasEtr := chMap["etr_video_encode_type"]; hasEtr {
			chMap["etr_video_encode_type"] = encType
		}
		return nil
	})
}

// SetFPS sets the framerate for a channel/stream.
func (c *Client) SetFPS(ctx context.Context, channel, stream, fps int) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		chMap["fps"] = fps
		if _, hasEtr := chMap["etr_fps"]; hasEtr {
			chMap["etr_fps"] = fps
		}
		return nil
	})
}

// SetGOP sets the I-frame interval (i_frame_interval) in frames.
func (c *Client) SetGOP(ctx context.Context, channel, stream, gop int) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		chMap["i_frame_interval"] = gop
		if _, hasEtr := chMap["etr_i_frame_interval"]; hasEtr {
			chMap["etr_i_frame_interval"] = gop
		}
		return nil
	})
}

// SetBitrate sets bitrate (Kbps) and optional bitrate_control ("CBR" or "VBR").
func (c *Client) SetBitrate(ctx context.Context, channel, stream, kbps int, mode string) error {
	return c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		if mode != "" {
			m := strings.ToUpper(strings.TrimSpace(mode))
			chMap["bitrate_control"] = m
			if _, hasEtr := chMap["etr_bitrate_control"]; hasEtr {
				chMap["etr_bitrate_control"] = m
			}
		}
		brMode := "UserDefined"
		if predefinedBitrates[kbps] {
			brMode = "Predefined"
		}
		chMap["bitrate_mode"] = brMode
		chMap["bitrate"] = kbps
		chMap["custom_bitrate"] = kbps
		if _, hasEtr := chMap["etr_bitrate"]; hasEtr {
			chMap["etr_bitrate_mode"] = brMode
			chMap["etr_bitrate"] = kbps
			chMap["etr_custom_bitrate"] = kbps
		}
		return nil
	})
}

// SetAudioAAC enables audio on the stream and attempts to switch DeviceConfig/Audio
// to AAC if supported by the hardware's Range.
func (c *Client) SetAudioAAC(ctx context.Context, channel, stream int) error {
	if err := c.mutateStreamChannel(ctx, channel, stream, func(chMap map[string]any) error {
		chMap["audio"] = true
		if _, hasEtr := chMap["etr_audio"]; hasEtr {
			chMap["etr_audio"] = true
		}
		return nil
	}); err != nil {
		return err
	}

	// Check if DeviceConfig/Audio/Range supports AAC; if so, switch audio_type to AAC.
	rngEnv, err := c.Call(ctx, "DeviceConfig/Audio/Range", map[string]any{})
	if err == nil && strings.Contains(strings.ToUpper(string(rngEnv.Data)), `"AAC"`) {
		audEnv, err := c.Call(ctx, "DeviceConfig/Audio/Get", map[string]any{})
		if err == nil {
			var p struct {
				ChannelInfo map[string]map[string]any `json:"channel_info"`
			}
			if json.Unmarshal(audEnv.Data, &p) == nil {
				chKey := ChannelKey(channel)
				if chMap, ok := p.ChannelInfo[chKey]; ok {
					chMap["audio_type"] = "AAC"
					_, _ = c.Call(ctx, "DeviceConfig/Audio/Set", map[string]any{
						"channel_info": map[string]any{chKey: chMap},
					})
				}
			}
		}
	}
	return nil
}

// GetFPSCapability queries /API/StreamConfig/<Stream>/Range to determine the
// maximum supported FPS for the channel/resolution/codec.
func (c *Client) GetFPSCapability(ctx context.Context, channel, stream, width, height int, codec string) (currentFPS, maxFPS int, source string, err error) {
	curInfo, err := c.GetStreamInfo(ctx, channel, stream)
	if err == nil {
		currentFPS = curInfo.FPS
	}
	base := streamAPIPath(stream)
	env, err := c.Call(ctx, base+"/Range", map[string]any{})
	if err != nil {
		return currentFPS, 30, "fallback", nil
	}
	var rng struct {
		ChannelInfo struct {
			Items map[string]struct {
				Items struct {
					StreamInfo struct {
						Items []struct {
							Resolution struct {
								Items []string `json:"items"`
							} `json:"resolution"`
							FPS struct {
								Ranges [][]int `json:"ranges"`
								Max    int     `json:"max"`
							} `json:"fps"`
						} `json:"items"`
					} `json:"streaminfo"`
				} `json:"items"`
			} `json:"items"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &rng); err != nil {
		return currentFPS, 30, "fallback", nil
	}
	chItem, ok := rng.ChannelInfo.Items[ChannelKey(channel)]
	if !ok || len(chItem.Items.StreamInfo.Items) == 0 {
		return currentFPS, 30, "fallback", nil
	}
	wantRes := FormatResolution(width, height)
	for _, si := range chItem.Items.StreamInfo.Items {
		for idx, resStr := range si.Resolution.Items {
			if strings.EqualFold(strings.TrimSpace(resStr), wantRes) || width <= 0 {
				if idx < len(si.FPS.Ranges) && len(si.FPS.Ranges[idx]) > 0 {
					vals := si.FPS.Ranges[idx]
					m := vals[len(vals)-1]
					if m > 0 {
						return currentFPS, m, "capability", nil
					}
				}
				if si.FPS.Max > 0 {
					return currentFPS, si.FPS.Max, "capability", nil
				}
			}
		}
	}
	// If exact resolution wasn't matched, take the max from the first streaminfo entry.
	si0 := chItem.Items.StreamInfo.Items[0]
	if len(si0.FPS.Ranges) > 0 && len(si0.FPS.Ranges[0]) > 0 {
		vals := si0.FPS.Ranges[0]
		if m := vals[len(vals)-1]; m > 0 {
			return currentFPS, m, "capability", nil
		}
	}
	return currentFPS, 25, "fallback", nil
}
