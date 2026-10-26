package camera

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/raysharp"
)

// raysharpCamera adapts *raysharp.Client to the full Camera and capability
// interfaces (FPSSettings, DeviceIdentity, PictureSettings, NetworkSettings,
// Rebooter, StorageManager, RemoteDeviceLister, AutoRebootConfig,
// DeviceTimeConfig, Recorder, PTZControl, AIConfig, RawRPC).
type raysharpCamera struct {
	client  *raysharp.Client
	device  config.Device
	timeout time.Duration
}

func toRaysharpStreamInfo(info raysharp.StreamInfo) StreamInfo {
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

func (r *raysharpCamera) Close() error {
	return r.client.Close()
}

func (r *raysharpCamera) GetSerialNumber(ctx context.Context) (string, error) {
	return r.client.GetSerialNumber(ctx)
}

func (r *raysharpCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	if r.device.NVRChannel > 0 {
		ch := r.device.NVRChannel - 1
		var out []StreamInfo
		for _, s := range []int{StreamMain, StreamSub1} {
			info, err := r.client.GetStreamInfo(ctx, ch, s)
			if err == nil {
				si := toRaysharpStreamInfo(info)
				si.Channel = ch + 1
				if s == StreamMain {
					if name, lines, _, _, err := r.client.GetChannelOSD(ctx, ch); err == nil {
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
	infos, err := r.client.ProbeAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(infos))
	for _, i := range infos {
		out = append(out, toRaysharpStreamInfo(i))
	}
	return out, nil
}

func (r *raysharpCamera) GetFPSCapability(ctx context.Context, channel, stream, width, height int, codec string) (FPSCapability, error) {
	cur, max, src, err := r.client.GetFPSCapability(ctx, channel, stream, width, height, codec)
	if err != nil {
		return FPSCapability{}, err
	}
	return FPSCapability{
		CurrentFPS: cur,
		MaxFPS:     max,
		Source:     src,
	}, nil
}

func (r *raysharpCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
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
				add(r.applyCodec(ctx, ch, s, streamName, profile.Codec, profile.CodecProfile))
			}
			if profile.SetResolution {
				add(r.applyResolution(ctx, ch, s, streamName, profile.Width, profile.Height))
			}
			if profile.SetFPS {
				add(r.applyFPS(ctx, ch, s, streamName, profile.FPS))
			}
			if profile.SetGOP {
				add(r.applyGOP(ctx, ch, s, streamName, profile.GOP))
			}
			if profile.SetBitrate {
				add(r.applyBitrate(ctx, ch, s, streamName, profile.Bitrate, profile.BitrateMode))
			}
			if profile.SetAudioAAC {
				add(r.applyAudioAAC(ctx, ch, s, streamName))
			}
			if profile.SetSmartCodec {
				st := r.applySmartCodec(ctx, ch, s, profile.SmartCodec)
				st.Step = fmt.Sprintf("smart codec %s", streamName)
				add(st)
			}
		}

		if profile.SetOSD {
			add(r.applyOSD(ctx, ch, profile.OSDLines))
		}
	}
	return steps
}

func (r *raysharpCamera) applyCodec(ctx context.Context, ch, stream int, streamName, codec, profile string) StepResult {
	step := StepResult{Step: fmt.Sprintf("codec %s", streamName)}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && strings.EqualFold(before.Compression, codec) && (profile == "" || strings.EqualFold(before.Profile, profile)) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %s từ trước, bỏ qua", before.Compression)
		return step
	}
	if err := r.client.SetCodec(ctx, ch, stream, codec, profile); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	wantBase, _ := raysharp.ParseEncodeType(codec)
	if !strings.EqualFold(after.Compression, wantBase) {
		step.Err = fmt.Sprintf("camera không đổi sang %s (vẫn là %s)", wantBase, after.Compression)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%s → %s (%s)", before.Compression, after.Compression, after.Profile)
	return step
}

func (r *raysharpCamera) applyResolution(ctx context.Context, ch, stream int, streamName string, w, h int) StepResult {
	step := StepResult{Step: fmt.Sprintf("độ phân giải %s", streamName)}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.Width == w && before.Height == h {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %dx%d từ trước, bỏ qua", w, h)
		return step
	}
	if err := r.client.SetResolution(ctx, ch, stream, w, h); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.Width != w || after.Height != h {
		step.Err = fmt.Sprintf("camera không hỗ trợ %dx%d (đang ở %dx%d)", w, h, after.Width, after.Height)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%dx%d → %dx%d", before.Width, before.Height, after.Width, after.Height)
	return step
}

func (r *raysharpCamera) applyFPS(ctx context.Context, ch, stream int, streamName string, fps int) StepResult {
	step := StepResult{Step: fmt.Sprintf("FPS %s", streamName)}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.FPS == fps {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d fps từ trước, bỏ qua", fps)
		return step
	}
	if err := r.client.SetFPS(ctx, ch, stream, fps); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.FPS != fps {
		if after.FPS > 0 && after.FPS != before.FPS {
			step.OK = true
			step.Detail = fmt.Sprintf("%d → %d fps (camera giới hạn từ %d fps)", before.FPS, after.FPS, fps)
			return step
		}
		step.Err = fmt.Sprintf("camera không đổi FPS sang %d (vẫn ở %d)", fps, after.FPS)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%d → %d fps", before.FPS, after.FPS)
	return step
}

func (r *raysharpCamera) applyGOP(ctx context.Context, ch, stream int, streamName string, gop int) StepResult {
	step := StepResult{Step: fmt.Sprintf("GOP %s", streamName)}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.GOP == gop {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở GOP %d từ trước, bỏ qua", gop)
		return step
	}
	if err := r.client.SetGOP(ctx, ch, stream, gop); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("GOP %d → %d", before.GOP, after.GOP)
	return step
}

func (r *raysharpCamera) applyBitrate(ctx context.Context, ch, stream int, streamName string, kbps int, mode string) StepResult {
	step := StepResult{Step: fmt.Sprintf("bitrate %s", streamName)}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.BitRate == kbps && (mode == "" || strings.EqualFold(before.BitRateControl, mode)) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d kbps (%s) từ trước, bỏ qua", kbps, before.BitRateControl)
		return step
	}
	if err := r.client.SetBitrate(ctx, ch, stream, kbps, mode); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%d kbps (%s) → %d kbps (%s)", before.BitRate, before.BitRateControl, after.BitRate, after.BitRateControl)
	return step
}

func (r *raysharpCamera) applyAudioAAC(ctx context.Context, ch, stream int, streamName string) StepResult {
	step := StepResult{Step: fmt.Sprintf("audio %s", streamName)}
	if err := r.client.SetAudioAAC(ctx, ch, stream); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	step.OK = true
	if strings.EqualFold(after.AudioCodec, "AAC") {
		step.Detail = "đã bật audio AAC"
	} else {
		step.Detail = fmt.Sprintf("đã bật audio (phần cứng Raysharp giữ codec %s, không hỗ trợ AAC)", after.AudioCodec)
	}
	return step
}

func (r *raysharpCamera) applySmartCodec(ctx context.Context, ch, stream int, enable bool) StepResult {
	step := StepResult{Step: "smart codec"}
	before, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.SmartCodec == enable {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở trạng thái %v từ trước, bỏ qua", enable)
		return step
	}
	if err := r.client.SetSmartCodec(ctx, ch, stream, enable); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := r.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = fmt.Sprintf("đã ghi nhưng đọc lại lỗi: %v", err)
		return step
	}
	if after.SmartCodec != enable {
		step.Err = fmt.Sprintf("camera không đổi SmartCodec sang %v", enable)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("SmartCodec %v → %v (%s)", before.SmartCodec, after.SmartCodec, raysharp.FormatEncodeType(after.Compression, after.SmartCodec))
	return step
}

func (r *raysharpCamera) applyOSD(ctx context.Context, ch int, lines []string) StepResult {
	step := StepResult{Step: fmt.Sprintf("OSD K%d", ch+1)}
	camName := r.device.ChannelName
	if camName == "" {
		camName = r.device.Name
	}
	expanded := make([]string, len(lines))
	en := make([]bool, len(lines))
	for i, l := range lines {
		expanded[i] = strings.ReplaceAll(l, "{name}", camName)
		en[i] = strings.TrimSpace(expanded[i]) != ""
	}
	applied, err := r.client.SetOSDLines(ctx, ch, expanded, en)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("đã ghi %d dòng OSD", applied)
	return step
}

func (r *raysharpCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	_, err := r.client.Call(ctx, "Login/Password/Set", map[string]any{
		"username": r.device.Username,
		"password": newPass,
	})
	return err
}

func (r *raysharpCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.Snapshot(ctx, channel)
}

func (r *raysharpCamera) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.GetChannelOSD(ctx, channel)
}

func (r *raysharpCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.SetChannelName(ctx, channel, name)
}

func (r *raysharpCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.SetOSDLines(ctx, channel, lines, enabled)
}

func (r *raysharpCamera) GetPicture(ctx context.Context, channel int) (map[string]any, map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.GetPicture(ctx, channel)
}

func (r *raysharpCamera) SetPicture(ctx context.Context, channel int, colorChanges, optionsChanges map[string]any) (map[string]any, map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.SetPicture(ctx, channel, colorChanges, optionsChanges)
}

func (r *raysharpCamera) GetPictureCaps(ctx context.Context, channel int) (map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.GetPictureCaps(ctx, channel)
}

func (r *raysharpCamera) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	return r.client.GetNetworkConfig(ctx)
}

func (r *raysharpCamera) SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	return r.client.SetStaticIP(ctx, iface, dhcpEnable, ip, mask, gateway, dns)
}

func (r *raysharpCamera) GetWiFiConfig(ctx context.Context) (map[string]map[string]any, error) {
	return map[string]map[string]any{}, nil
}

func (r *raysharpCamera) SetWiFiConfig(ctx context.Context, iface, ssid, password, encryption string) error {
	return fmt.Errorf("raysharp: Wi-Fi config not supported on this model")
}

func (r *raysharpCamera) ScanWiFi(ctx context.Context) ([]dahua.WiFiAP, error) {
	return []dahua.WiFiAP{}, nil
}

func (r *raysharpCamera) Reboot(ctx context.Context) error {
	return r.client.Reboot(ctx)
}

func (r *raysharpCamera) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	return r.client.GetStorageInfo(ctx)
}

func (r *raysharpCamera) FormatStorage(ctx context.Context, name string) error {
	return r.client.FormatStorage(ctx, name)
}

func (r *raysharpCamera) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	return r.client.GetRemoteDevices(ctx)
}

func (r *raysharpCamera) GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error) {
	return r.client.GetAutoReboot(ctx)
}

func (r *raysharpCamera) SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error {
	return r.client.SetAutoReboot(ctx, ar)
}

func (r *raysharpCamera) GetTimeConfig(ctx context.Context) (dahua.TimeConfig, error) {
	return r.client.GetTimeConfig(ctx)
}

func (r *raysharpCamera) SetTimeConfig(ctx context.Context, cfg dahua.TimeConfig) error {
	return r.client.SetTimeConfig(ctx, cfg)
}

func (r *raysharpCamera) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.FindRecordings(ctx, channel, start, end)
}

func (r *raysharpCamera) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return raysharp.StreamPlayback(ctx, w, r.device.Host, r.device.Username, r.device.Password, channel, start, end)
}

func (r *raysharpCamera) StreamPlaybackFast(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return r.StreamPlayback(ctx, w, channel, start, end)
}

func (r *raysharpCamera) StreamNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return r.StreamPlayback(ctx, w, channel, start, end)
}

func (r *raysharpCamera) PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.PTZMove(ctx, channel, code, speed, start)
}

// --- AIConfig & RawRPC implementation ---

func (r *raysharpCamera) GetChnSmart(ctx context.Context) ([]raysharp.ChannelSmartSummary, error) {
	return r.client.GetChnSmart(ctx)
}

func (r *raysharpCamera) GetAISetting(ctx context.Context, channel int, ability string) (map[string]any, map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.GetAISetting(ctx, channel, ability)
}

func (r *raysharpCamera) SetAISetting(ctx context.Context, channel int, ability string, changes map[string]any) (map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.SetAISetting(ctx, channel, ability, changes)
}

func (r *raysharpCamera) GetAIAlarmLinkage(ctx context.Context, channel int, ability string) (map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.GetAIAlarmLinkage(ctx, channel, ability)
}

func (r *raysharpCamera) SetAIAlarmLinkage(ctx context.Context, channel int, ability string, changes map[string]any) (map[string]any, error) {
	if channel == 0 && r.device.NVRChannel > 0 {
		channel = r.device.NVRChannel - 1
	}
	return r.client.SetAIAlarmLinkage(ctx, channel, ability, changes)
}

func (r *raysharpCamera) ConfigureEventPush(ctx context.Context, enable bool, targetHost string, targetPort int, targetPath string) error {
	return r.client.ConfigureEventPush(ctx, enable, targetHost, targetPort, targetPath)
}

func (r *raysharpCamera) CallRaw(ctx context.Context, apiPath string, reqData any) (map[string]any, error) {
	return r.client.CallRaw(ctx, apiPath, reqData)
}
