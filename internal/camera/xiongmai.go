package camera

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/xiongmai"
)

// Compile-time OOP interface conformance assertions for xiongmaiCamera.
var (
	_ Camera             = (*xiongmaiCamera)(nil)
	_ FPSSettings        = (*xiongmaiCamera)(nil)
	_ DeviceIdentity     = (*xiongmaiCamera)(nil)
	_ NetworkSettings    = (*xiongmaiCamera)(nil)
	_ Rebooter           = (*xiongmaiCamera)(nil)
	_ StorageManager     = (*xiongmaiCamera)(nil)
	_ RemoteDeviceLister = (*xiongmaiCamera)(nil)
	_ AutoRebootConfig   = (*xiongmaiCamera)(nil)
	_ DeviceTimeConfig   = (*xiongmaiCamera)(nil)
	_ Recorder           = (*xiongmaiCamera)(nil)
	_ PTZControl         = (*xiongmaiCamera)(nil)
	_ RawRPC             = (*xiongmaiCamera)(nil)
)

// xiongmaiCamera adapts *xiongmai.Client (dual-transport Sofia TCP :34567 +
// HTTP CGI :80) to kspcam's vendor-agnostic Camera and capability interfaces.
type xiongmaiCamera struct {
	client  *xiongmai.Client
	device  config.Device
	timeout time.Duration
}

func toXiongmaiStreamInfo(info xiongmai.StreamInfo) StreamInfo {
	return StreamInfo{
		Channel:     info.Channel,
		Stream:      info.Stream,
		Width:       info.Width,
		Height:      info.Height,
		FPS:         info.FPS,
		Compression: info.Compression,
		Profile:     info.Profile,
		AudioCodec:  info.AudioCodec,
		AudioEnable: info.AudioEnable,
		SmartCodec:  info.SmartCodec,
		GOP:         info.GOP,
		BitrateKbps: info.BitRate,
		BitrateMode: info.BitRateControl,
		Name:        info.Name,
		OSDLines:    info.OSDLines,
	}
}

func (x *xiongmaiCamera) resolveChannel(channel int) int {
	if channel == 0 && x.device.NVRChannel > 0 {
		return x.device.NVRChannel - 1
	}
	return channel
}

func (x *xiongmaiCamera) Close() error {
	return x.client.Close()
}

func (x *xiongmaiCamera) GetSerialNumber(ctx context.Context) (string, error) {
	return x.client.GetSerialNumber(ctx)
}

func (x *xiongmaiCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	if x.device.NVRChannel > 0 && !x.device.IsNVR {
		ch := x.device.NVRChannel - 1
		var out []StreamInfo
		for _, s := range []int{StreamMain, StreamSub1} {
			info, err := x.client.GetStreamInfo(ctx, ch, s)
			if err == nil {
				si := toXiongmaiStreamInfo(info)
				si.Channel = ch + 1
				if s == StreamMain {
					if name, lines, _, _, oerr := x.client.GetChannelOSD(ctx, ch); oerr == nil {
						si.Name = name
						si.OSDLines = lines
					}
				}
				out = append(out, si)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	infos, err := x.client.ProbeAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(infos))
	for _, i := range infos {
		out = append(out, toXiongmaiStreamInfo(i))
	}
	return out, nil
}

func (x *xiongmaiCamera) GetFPSCapability(ctx context.Context, channel, stream, width, height int, codec string) (FPSCapability, error) {
	ch := x.resolveChannel(channel)
	cur, max, src, err := x.client.GetFPSCapability(ctx, ch, stream, width, height, codec)
	if err != nil {
		return FPSCapability{}, err
	}
	return FPSCapability{
		CurrentFPS: cur,
		MaxFPS:     max,
		Source:     src,
	}, nil
}

func (x *xiongmaiCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
	var steps []StepResult
	add := func(step StepResult) {
		steps = append(steps, step)
		if emit != nil {
			emit(step)
		}
	}

	for _, ch := range profile.channelsList() {
		for _, s := range profile.streams() {
			streamName := fmt.Sprintf("K%d %s", ch+1, streamLabel(s))
			if profile.SetCodec {
				add(x.applyCodec(ctx, ch, s, streamName, profile.Codec, profile.CodecProfile))
			}
			if profile.SetResolution {
				add(x.applyResolution(ctx, ch, s, streamName, profile.Width, profile.Height))
			}
			if profile.SetFPS {
				add(x.applyFPS(ctx, ch, s, streamName, profile.FPS))
			}
			if profile.SetGOP {
				add(x.applyGOP(ctx, ch, s, streamName, profile.GOP))
			}
			if profile.SetBitrate {
				add(x.applyBitrate(ctx, ch, s, streamName, profile.Bitrate, profile.BitrateMode))
			}
			if profile.SetAudioAAC {
				add(x.applyAudioAAC(ctx, ch, s, streamName))
			}
			if profile.SetSmartCodec {
				st := x.applySmartCodec(ctx, ch, s, profile.SmartCodec)
				st.Step = fmt.Sprintf("smart codec %s", streamName)
				add(st)
			}
		}
		if profile.SetOSD {
			add(x.applyOSD(ctx, ch, profile.OSDLines))
		}
	}
	return steps
}

func (x *xiongmaiCamera) applyCodec(ctx context.Context, ch, stream int, streamName, codec, prof string) StepResult {
	step := StepResult{Step: fmt.Sprintf("codec %s", streamName)}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	want := strings.ToUpper(strings.TrimSpace(codec))
	if want == "H.264H" || want == "H.264B" {
		want = "H.264"
	}
	if err == nil && strings.EqualFold(before.Compression, want) && (prof == "" || strings.EqualFold(before.Profile, prof)) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %s từ trước, bỏ qua", before.Compression)
		return step
	}
	if err := x.client.SetCodec(ctx, ch, stream, want, prof); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if !strings.EqualFold(after.Compression, want) {
		step.Err = fmt.Sprintf("camera không hỗ trợ %s (đọc lại vẫn là %s)", want, after.Compression)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%s → %s", before.Compression, after.Compression)
	return step
}

func (x *xiongmaiCamera) applyResolution(ctx context.Context, ch, stream int, streamName string, w, h int) StepResult {
	step := StepResult{Step: fmt.Sprintf("độ phân giải %s", streamName)}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.Width == w && before.Height == h {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %dx%d từ trước, bỏ qua", w, h)
		return step
	}
	if err := x.client.SetResolution(ctx, ch, stream, w, h); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.Width != w || after.Height != h {
		if after.Width != before.Width || after.Height != before.Height {
			step.OK = true
			step.Detail = fmt.Sprintf("camera tự điều chỉnh về %dx%d (%s)", after.Width, after.Height, after.ResolutionCode)
			return step
		}
		step.Err = fmt.Sprintf("camera không đổi sang %dx%d (vẫn ở %dx%d)", w, h, after.Width, after.Height)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%dx%d → %dx%d (%s)", before.Width, before.Height, after.Width, after.Height, after.ResolutionCode)
	return step
}

func (x *xiongmaiCamera) applyFPS(ctx context.Context, ch, stream int, streamName string, fps int) StepResult {
	step := StepResult{Step: fmt.Sprintf("FPS %s", streamName)}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.FPS == fps {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d fps từ trước, bỏ qua", fps)
		return step
	}
	if err := x.client.SetFPS(ctx, ch, stream, fps); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.FPS != fps {
		if after.FPS != before.FPS {
			step.OK = true
			step.Detail = fmt.Sprintf("camera tự ép dải về %d fps (yêu cầu %d)", after.FPS, fps)
			return step
		}
		step.Err = fmt.Sprintf("camera không hỗ trợ %d fps (vẫn ở %d fps)", fps, after.FPS)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%d → %d fps", before.FPS, after.FPS)
	return step
}

func (x *xiongmaiCamera) applyGOP(ctx context.Context, ch, stream int, streamName string, gop int) StepResult {
	step := StepResult{Step: fmt.Sprintf("GOP %s", streamName)}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	wantSec := xiongmai.GOPSecondsFromFrames(gop, before.FPS)
	if err == nil && (before.GOP == gop || before.GOPRaw == wantSec) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở GOP %ds (%d frames) từ trước, bỏ qua", before.GOPRaw, before.GOP)
		return step
	}
	if err := x.client.SetGOP(ctx, ch, stream, gop); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.GOPRaw != wantSec && after.GOP == before.GOP {
		step.Err = fmt.Sprintf("camera không đổi GOP (vẫn ở %ds)", after.GOPRaw)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("GOP %ds → %ds (~%d frames)", before.GOPRaw, after.GOPRaw, after.GOP)
	return step
}

func (x *xiongmaiCamera) applyBitrate(ctx context.Context, ch, stream int, streamName string, kbps int, mode string) StepResult {
	step := StepResult{Step: fmt.Sprintf("bitrate %s", streamName)}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.BitRate == kbps && (mode == "" || strings.EqualFold(before.BitRateControl, mode)) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d Kbps (%s) từ trước, bỏ qua", kbps, before.BitRateControl)
		return step
	}
	if err := x.client.SetBitrate(ctx, ch, stream, kbps, mode); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.BitRate != kbps && after.BitRate == before.BitRate {
		step.Err = fmt.Sprintf("camera không đổi bitrate (vẫn ở %d Kbps)", after.BitRate)
		return step
	}
	step.OK = true
	if after.BitRate != kbps {
		step.Detail = fmt.Sprintf("camera tự điều chỉnh về %d Kbps (%s)", after.BitRate, after.BitRateControl)
	} else {
		step.Detail = fmt.Sprintf("%d → %d Kbps (%s)", before.BitRate, after.BitRate, after.BitRateControl)
	}
	return step
}

func (x *xiongmaiCamera) applyAudioAAC(ctx context.Context, ch, stream int, streamName string) StepResult {
	step := StepResult{Step: fmt.Sprintf("audio %s", streamName)}
	before, _ := x.client.GetStreamInfo(ctx, ch, stream)
	if before.AudioEnable && strings.EqualFold(before.AudioCodec, "AAC") {
		step.OK = true
		step.Detail = "đã ở AAC từ trước, bỏ qua"
		return step
	}
	if err := x.client.SetAudioAAC(ctx, ch, stream); err != nil {
		// Most Xiongmai Sofia chips only implement hardware G711_ALAW.
		step.OK = true
		step.Detail = fmt.Sprintf("phần cứng chỉ hỗ trợ %s (giữ nguyên, Shinobi dùng acodec=no)", before.AudioCodec)
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if !strings.EqualFold(after.AudioCodec, "AAC") {
		step.OK = true
		step.Detail = fmt.Sprintf("phần cứng chỉ hỗ trợ %s (Shinobi dùng acodec=no)", after.AudioCodec)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%s → AAC", before.AudioCodec)
	return step
}

func (x *xiongmaiCamera) applySmartCodec(ctx context.Context, ch, stream int, enable bool) StepResult {
	step := StepResult{Step: "smart codec"}
	before, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.SmartCodec == enable {
		step.OK = true
		step.Detail = fmt.Sprintf("SmartCodec đã ở %v từ trước, bỏ qua", enable)
		return step
	}
	if err := x.client.SetSmartCodec(ctx, ch, enable); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := x.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("SmartCodec %v → %v", before.SmartCodec, after.SmartCodec)
	return step
}

func (x *xiongmaiCamera) applyOSD(ctx context.Context, ch int, lines []string) StepResult {
	step := StepResult{Step: fmt.Sprintf("OSD K%d", ch+1)}
	camName := x.device.ChannelName
	if camName == "" {
		camName = x.device.Name
	}
	expanded := make([]string, len(lines))
	en := make([]bool, len(lines))
	for i, l := range lines {
		expanded[i] = strings.ReplaceAll(l, "{name}", camName)
		en[i] = strings.TrimSpace(expanded[i]) != ""
	}
	applied, err := x.client.SetOSDLines(ctx, ch, expanded, en)
	if err != nil {
		// Fallback: write line 0 into ChannelTitle if fVideo.OSDInfo is absent on 8MB IPCs.
		if len(expanded) > 0 && strings.TrimSpace(expanded[0]) != "" {
			if nerr := x.client.SetChannelName(ctx, ch, expanded[0]); nerr == nil {
				step.OK = true
				step.Detail = "firmware không có OSD 4 dòng; đã ghi dòng 1 vào ChannelTitle"
				return step
			}
		}
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("đã ghi %d dòng OSD", applied)
	return step
}

func (x *xiongmaiCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return x.client.ChangePassword(ctx, newUser, newPass)
}

func (x *xiongmaiCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return x.client.Snapshot(ctx, x.resolveChannel(channel), stream)
}

func (x *xiongmaiCamera) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	return x.client.GetChannelOSD(ctx, x.resolveChannel(channel))
}

func (x *xiongmaiCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return x.client.SetChannelName(ctx, x.resolveChannel(channel), name)
}

func (x *xiongmaiCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	return x.client.SetOSDLines(ctx, x.resolveChannel(channel), lines, enabled)
}

func (x *xiongmaiCamera) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	return x.client.GetNetworkConfig(ctx)
}

func (x *xiongmaiCamera) SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	return x.client.SetStaticIP(ctx, iface, dhcpEnable, ip, mask, gateway, dns)
}

func (x *xiongmaiCamera) GetWiFiConfig(ctx context.Context) (map[string]map[string]any, error) {
	return map[string]map[string]any{}, nil
}

func (x *xiongmaiCamera) SetWiFiConfig(ctx context.Context, iface, ssid, password, encryption string) error {
	return fmt.Errorf("xiongmai: cấu hình Wi-Fi chưa hỗ trợ trên model này")
}

func (x *xiongmaiCamera) ScanWiFi(ctx context.Context) ([]dahua.WiFiAP, error) {
	return []dahua.WiFiAP{}, nil
}

func (x *xiongmaiCamera) Reboot(ctx context.Context) error {
	return x.client.Reboot(ctx)
}

func (x *xiongmaiCamera) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	return x.client.GetStorageInfo(ctx)
}

func (x *xiongmaiCamera) FormatStorage(ctx context.Context, name string) error {
	return x.client.FormatStorage(ctx, name)
}

func (x *xiongmaiCamera) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	return x.client.GetRemoteDevices(ctx)
}

func (x *xiongmaiCamera) GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error) {
	return x.client.GetAutoReboot(ctx)
}

func (x *xiongmaiCamera) SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error {
	return x.client.SetAutoReboot(ctx, ar)
}

func (x *xiongmaiCamera) GetTimeConfig(ctx context.Context) (dahua.TimeConfig, error) {
	return x.client.GetTimeConfig(ctx)
}

func (x *xiongmaiCamera) SetTimeConfig(ctx context.Context, cfg dahua.TimeConfig) error {
	return x.client.SetTimeConfig(ctx, cfg)
}

func (x *xiongmaiCamera) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	return x.client.FindRecordings(ctx, x.resolveChannel(channel), start, end)
}

func (x *xiongmaiCamera) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return x.client.StreamPlaybackMP4(ctx, w, x.resolveChannel(channel), start, end)
}

func (x *xiongmaiCamera) StreamPlaybackFast(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return x.StreamPlayback(ctx, w, channel, start, end)
}

func (x *xiongmaiCamera) StreamNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return x.client.DownloadNative(ctx, w, x.resolveChannel(channel), start, end)
}

func (x *xiongmaiCamera) PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error {
	return x.client.PTZMove(ctx, x.resolveChannel(channel), code, speed, start)
}

func (x *xiongmaiCamera) CallRaw(ctx context.Context, apiPath string, reqData any) (map[string]any, error) {
	return x.client.CallRaw(ctx, apiPath, reqData)
}
