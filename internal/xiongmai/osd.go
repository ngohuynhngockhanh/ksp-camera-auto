package xiongmai

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// GetChannelTitles reads the ChannelTitle array (1 entry per channel).
func (c *Client) GetChannelTitles(ctx context.Context) ([]string, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := tr.GetConfig(ctx, "ChannelTitle")
	if err != nil {
		return nil, err
	}
	raw, ok := resp["ChannelTitle"].([]any)
	if !ok {
		return nil, fmt.Errorf("xiongmai: missing ChannelTitle array")
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		if s, ok := v.(string); ok {
			out[i] = s
		}
	}
	return out, nil
}

// SetChannelName updates ChannelTitle[channel] (0-based channel, max 64 UTF-8 bytes).
func (c *Client) SetChannelName(ctx context.Context, channel int, name string) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	titles, err := c.GetChannelTitles(ctx)
	if err != nil {
		if channel == 0 {
			titles = []string{""}
		} else {
			return err
		}
	}
	for len(titles) <= channel {
		titles = append(titles, "")
	}
	if len(name) > 64 {
		name = name[:64]
	}
	titles[channel] = name
	_, err = tr.SetConfig(ctx, "ChannelTitle", titles)
	return err
}

// GetChannelOSD reads channel name and up to 4 custom OSD lines from fVideo.OSDInfo.
func (c *Client) GetChannelOSD(ctx context.Context, channel int) (name string, lines []string, enabled []bool, supported bool, err error) {
	titles, terr := c.GetChannelTitles(ctx)
	if terr == nil && channel >= 0 && channel < len(titles) {
		name = titles[channel]
	}
	tr, err := c.Transport(ctx)
	if err != nil {
		return name, nil, nil, false, err
	}
	resp, oerr := tr.GetConfig(ctx, "fVideo.OSDInfo")
	if oerr != nil {
		if isUnsupportedErr(oerr) {
			return name, nil, nil, false, nil
		}
		return name, nil, nil, false, nil
	}
	osdRoot := resp["fVideo.OSDInfo"]
	widgets := extractOSDWidgets(osdRoot, channel)
	if len(widgets) == 0 {
		return name, nil, nil, false, nil
	}
	maxSlots := len(widgets)
	if maxSlots > 4 {
		maxSlots = 4
	}
	lines = make([]string, maxSlots)
	enabled = make([]bool, maxSlots)
	for i := 0; i < maxSlots; i++ {
		w := widgets[i]
		if info, ok := w["OSDInfoWidget"].(map[string]any); ok {
			enabled[i] = boolFieldAny(info, "EncodeBlend")
		} else {
			enabled[i] = boolFieldAny(w, "EncodeBlend", "Enable")
		}
		if strs, ok := w["Info"].([]any); ok && len(strs) > 0 {
			if s0, ok := strs[0].(string); ok {
				lines[i] = s0
			}
		} else if s := stringFieldAny(w, "Name", "Text"); s != "" {
			lines[i] = s
		}
	}
	return name, lines, enabled, true, nil
}

// SetOSDLines writes up to 4 custom OSD lines into fVideo.OSDInfo.
func (c *Client) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return 0, err
	}
	resp, err := tr.GetConfig(ctx, "fVideo.OSDInfo")
	if err != nil {
		return 0, err
	}
	osdRoot := resp["fVideo.OSDInfo"]
	widgets := extractOSDWidgets(osdRoot, channel)
	if len(widgets) == 0 {
		return 0, fmt.Errorf("%w: fVideo.OSDInfo has no widgets for channel %d", ErrUnsupported, channel)
	}
	limit := len(widgets)
	if limit > 4 {
		limit = 4
	}
	applied := 0
	for i := 0; i < limit; i++ {
		txt := ""
		if i < len(lines) {
			txt = lines[i]
		}
		on := strings.TrimSpace(txt) != ""
		if i < len(enabled) {
			on = enabled[i]
		}
		w := widgets[i]
		if info, ok := w["OSDInfoWidget"].(map[string]any); ok {
			info["EncodeBlend"] = on
			info["PreviewBlend"] = on
		} else {
			w["EncodeBlend"] = on
		}
		if _, hasInfo := w["Info"]; hasInfo {
			w["Info"] = []string{txt}
		} else {
			w["Text"] = txt
		}
		applied++
	}
	if _, err := tr.SetConfig(ctx, "fVideo.OSDInfo", osdRoot); err != nil {
		return 0, err
	}
	return applied, nil
}

func extractOSDWidgets(osdRoot any, channel int) []map[string]any {
	switch root := osdRoot.(type) {
	case map[string]any:
		// Format A: {"OSDInfo": [ {"OSDInfoWidget": ...}, ... ]}
		if arr, ok := root["OSDInfo"].([]any); ok {
			// Either per-channel wrapper or direct widget list on 1-ch IPC
			if channel < len(arr) {
				if chObj, ok := arr[channel].(map[string]any); ok {
					if wArr, ok := chObj["OSDInfo"].([]any); ok {
						return toMapSlice(wArr)
					}
					if _, hasW := chObj["OSDInfoWidget"]; hasW && channel == 0 {
						return toMapSlice(arr)
					}
				}
			}
		}
	case []any:
		if channel >= 0 && channel < len(root) {
			if chObj, ok := root[channel].(map[string]any); ok {
				if wArr, ok := chObj["OSDInfo"].([]any); ok {
					return toMapSlice(wArr)
				}
			}
		}
	}
	return nil
}

func toMapSlice(in []any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// GetRemoteDevices reads NetWork.RemoteDeviceV3 on a Xiongmai HVR/NVR and maps
// configured IP channels to dahua.RemoteChannel (0-based Channel index).
// Handles hex SingleConnId ("0x00000004" -> Decoder[3]) and skips NoConfig slots
// where SingleConnId is explicitly 0 ("0x00000000").
func (c *Client) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	sys, _ := c.GetSystemInfo(ctx)
	titles, _ := c.GetChannelTitles(ctx)

	var chnStatuses []map[string]any
	if stResp, serr := tr.GetConfig(ctx, "NetWork.ChnStatus"); serr == nil {
		if stArr, ok := stResp["NetWork.ChnStatus"].([]any); ok {
			chnStatuses = toMapSlice(stArr)
		}
	}

	resp, err := tr.GetConfig(ctx, "NetWork.RemoteDeviceV3")
	if err != nil {
		// Fallback to older NetWork.RemoteDevice table if V3 is absent.
		resp, err = tr.GetConfig(ctx, "NetWork.RemoteDevice")
		if err != nil {
			return nil, err
		}
	}

	rawArr, isV3 := resp["NetWork.RemoteDeviceV3"].([]any)
	if !isV3 {
		rawArr, _ = resp["NetWork.RemoteDevice"].([]any)
	}
	var out []dahua.RemoteChannel
	for nDig, item := range rawArr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		enable := boolFieldAny(m, "Enable")
		nvrCh1 := sys.VideoInChannel + nDig + 1
		ch0 := nvrCh1 - 1

		if ch0 >= 0 && ch0 < len(chnStatuses) {
			st := strings.TrimSpace(stringFieldAny(chnStatuses[ch0], "Status"))
			if strings.EqualFold(st, "NoConfig") {
				continue
			}
		}

		var ip string
		var port int
		if isV3 {
			rawSingleID, hasSingleID := m["SingleConnId"]
			singleID := 1
			if hasSingleID {
				parsedID, valid := parseIntOrHex(rawSingleID)
				if valid && parsedID <= 0 {
					// SingleConnId = "0x00000000" indicates an unconfigured digital slot.
					continue
				}
				if valid {
					singleID = parsedID
				}
			}
			if decs, ok := m["Decoder"].([]any); ok && len(decs) > 0 {
				idx := singleID - 1
				if idx < 0 || idx >= len(decs) {
					idx = 0
				}
				if dm, ok := decs[idx].(map[string]any); ok {
					ip = strings.TrimSpace(stringFieldAny(dm, "IPAddress", "Address"))
					if strings.HasPrefix(ip, "0x") || strings.HasPrefix(ip, "0X") {
						if decIP, herr := HexIPToDecIP(ip); herr == nil {
							ip = decIP
						}
					}
					port = intFieldAny(dm, "Port")
				}
			}
		} else {
			if !enable {
				continue
			}
			ip = strings.TrimSpace(stringFieldAny(m, "IPAddress", "Address"))
			if strings.HasPrefix(ip, "0x") || strings.HasPrefix(ip, "0X") {
				if decIP, herr := HexIPToDecIP(ip); herr == nil {
					ip = decIP
				}
			}
			port = intFieldAny(m, "Port")
		}
		if ip == "" || ip == "0.0.0.0" {
			continue
		}
		chName := ""
		if ch0 >= 0 && ch0 < len(titles) {
			chName = titles[ch0]
		}
		if chName == "" && ch0 >= 0 && ch0 < len(chnStatuses) {
			chName = strings.TrimSpace(stringFieldAny(chnStatuses[ch0], "ChnName"))
		}
		out = append(out, dahua.RemoteChannel{
			Channel: ch0, // dahua.RemoteChannel.Channel is 0-based
			Enable:  enable,
			Address: ip,
			Port:    port,
			Name:    chName,
		})
	}
	return out, nil
}

// Snapshot captures one JPEG frame for channel (0-based), trying native
// Transport.Snapshot first and seamlessly falling back to a single-frame RTSP
// pull over FFmpeg (for Sofia IPCs whose OPSnap is disabled in 8MB flash).
func (c *Client) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	if tr, err := c.Transport(ctx); err == nil {
		if jpg, serr := tr.Snapshot(ctx, channel); serr == nil && len(jpg) > 0 {
			return jpg, nil
		}
	}
	return c.snapshotViaRTSP(ctx, channel+1, stream)
}

func (c *Client) snapshotViaRTSP(ctx context.Context, channel1Based, stream int) ([]byte, error) {
	ffPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("xiongmai snapshot: native OPSnap unsupported and ffmpeg not found")
	}
	sIdx := 1 // Prefer SubStream (faster keyframe, tiny bandwidth) unless caller requested Main
	if stream == 0 {
		sIdx = 0
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	effUser := c.EffectiveUsername(ctx)
	candidates := []string{
		BuildRTSPURL(c.host, effUser, c.password, channel1Based, sIdx),
		fmt.Sprintf("rtsp://%s:%s@%s:554/0", effUser, c.password, c.host),
	}
	var lastErr error
	for _, rtspURL := range candidates {
		cmd := exec.CommandContext(cctx, ffPath,
			"-hide_banner", "-loglevel", "error",
			"-rtsp_transport", "tcp",
			"-i", rtspURL,
			"-frames:v", "1",
			"-q:v", "4",
			"-f", "image2",
			"-vcodec", "mjpeg",
			"pipe:1",
		)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil && bytes.HasPrefix(stdout.Bytes(), []byte{0xFF, 0xD8, 0xFF}) {
			return stdout.Bytes(), nil
		} else if err != nil {
			lastErr = fmt.Errorf("%v (%s)", err, strings.TrimSpace(stderr.String()))
		}
	}
	return nil, fmt.Errorf("xiongmai RTSP snapshot failed: %v", lastErr)
}
