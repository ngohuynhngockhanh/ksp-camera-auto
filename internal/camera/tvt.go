package camera

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/tvt"
)

// Compile-time OOP interface conformance assertions for tvtCamera.
var (
	_ Camera             = (*tvtCamera)(nil)
	_ FPSSettings        = (*tvtCamera)(nil)
	_ DeviceIdentity     = (*tvtCamera)(nil)
	_ Rebooter           = (*tvtCamera)(nil)
	_ StorageManager     = (*tvtCamera)(nil)
	_ RemoteDeviceLister = (*tvtCamera)(nil)
	_ Recorder           = (*tvtCamera)(nil)
)

// tvtCamera adapts *tvt.Client (dual-plane NVMS-9000 XML + ONVIF Profile S/G +
// zero-CPU RTSP replay) to kspcam's vendor-agnostic Camera and capability interfaces.
type tvtCamera struct {
	client  *tvt.Client
	device  config.Device
	timeout time.Duration
}

func toTVTStreamInfo(info tvt.StreamInfo) StreamInfo {
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

func (t *tvtCamera) resolveChannel(channel int) int {
	if channel == 0 && t.device.NVRChannel > 0 {
		return t.device.NVRChannel - 1
	}
	return channel
}

func (t *tvtCamera) Close() error {
	return t.client.Close()
}

func (t *tvtCamera) GetSerialNumber(ctx context.Context) (string, error) {
	return t.client.GetSerialNumber(ctx)
}

func (t *tvtCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	if t.device.NVRChannel > 0 && !t.device.IsNVR {
		ch := t.device.NVRChannel - 1
		var out []StreamInfo
		for _, s := range []int{StreamMain, StreamSub1} {
			info, err := t.client.GetStreamInfo(ctx, ch, s)
			if err == nil {
				si := toTVTStreamInfo(info)
				si.Channel = ch + 1
				if s == StreamMain {
					if name, lines, _, ok, oerr := t.client.GetChannelOSD(ctx, ch); oerr == nil && ok {
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
	infos, err := t.client.ProbeAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(infos))
	for _, i := range infos {
		out = append(out, toTVTStreamInfo(i))
	}
	return out, nil
}

func (t *tvtCamera) GetFPSCapability(ctx context.Context, channel, stream, width, height int, codec string) (FPSCapability, error) {
	ch := t.resolveChannel(channel)
	cur, max, src, err := t.client.GetFPSCapability(ctx, ch, stream, width, height, codec)
	if err != nil {
		return FPSCapability{}, err
	}
	return FPSCapability{
		CurrentFPS: cur,
		MaxFPS:     max,
		Source:     src,
	}, nil
}

func (t *tvtCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
	var steps []StepResult
	add := func(step StepResult) {
		steps = append(steps, step)
		if emit != nil {
			emit(step)
		}
	}

	channels := profile.channelsList()
	if len(channels) == 1 && channels[0] == 0 && t.device.NVRChannel > 0 && !t.device.IsNVR {
		channels = []int{t.device.NVRChannel - 1}
	}

	for _, ch := range channels {
		for _, s := range profile.streams() {
			streamName := fmt.Sprintf("K%d %s", ch+1, streamLabel(s))
			if profile.SetCodec {
				add(t.applyCodec(ctx, ch, s, streamName, profile.Codec))
			}
			if profile.SetResolution {
				add(t.applyResolution(ctx, ch, s, streamName, profile.Width, profile.Height))
			}
			if profile.SetFPS {
				add(t.applyFPS(ctx, ch, s, streamName, profile.FPS))
			}
			if profile.SetGOP {
				add(t.applyGOP(ctx, ch, s, streamName, profile.GOP))
			}
			if profile.SetBitrate {
				add(t.applyBitrate(ctx, ch, s, streamName, profile.Bitrate, profile.BitrateMode))
			}
			if s == StreamMain && profile.SetSmartCodec {
				add(t.applySmartCodec(ctx, ch, s, profile.SmartCodec))
			}
			if s == StreamMain && profile.SetAudioAAC {
				add(StepResult{
					Step:   fmt.Sprintf("audio %s", streamName),
					OK:     true,
					Detail: "TVT NVR giữ nguyên cấu hình audio phần cứng (Shinobi acodec=no 0% CPU)",
				})
			}
		}
		if profile.SetOSD && len(profile.OSDLines) > 0 {
			add(t.applyOSD(ctx, ch, profile.OSDLines))
		}
	}
	return steps
}

func (t *tvtCamera) applyCodec(ctx context.Context, ch, stream int, streamName, codec string) StepResult {
	step := StepResult{Step: fmt.Sprintf("codec %s", streamName)}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && strings.EqualFold(before.Compression, codec) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %s từ trước, bỏ qua", before.Compression)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.Compression = codec
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	if !strings.EqualFold(after.Compression, codec) {
		step.Err = fmt.Sprintf("thiết bị vẫn giữ codec %s", after.Compression)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%s → %s", before.Compression, after.Compression)
	return step
}

func (t *tvtCamera) applyResolution(ctx context.Context, ch, stream int, streamName string, w, h int) StepResult {
	step := StepResult{Step: fmt.Sprintf("độ phân giải %s", streamName)}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.Width == w && before.Height == h {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %dx%d từ trước, bỏ qua", w, h)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.Width = w
		si.Height = h
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	if after.Width != w || after.Height != h {
		step.Err = fmt.Sprintf("thiết bị giữ %dx%d", after.Width, after.Height)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%dx%d → %dx%d", before.Width, before.Height, after.Width, after.Height)
	return step
}

func (t *tvtCamera) applyFPS(ctx context.Context, ch, stream int, streamName string, fps int) StepResult {
	step := StepResult{Step: fmt.Sprintf("FPS %s", streamName)}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.FPS == fps {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d fps từ trước, bỏ qua", fps)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.FPS = fps
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	if after.FPS == before.FPS && after.FPS != fps {
		step.Err = fmt.Sprintf("thiết bị không đổi FPS (vẫn %d)", after.FPS)
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%d → %d fps", before.FPS, after.FPS)
	return step
}

func (t *tvtCamera) applyGOP(ctx context.Context, ch, stream int, streamName string, gop int) StepResult {
	step := StepResult{Step: fmt.Sprintf("GOP %s", streamName)}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.GOP == gop {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở GOP %d từ trước, bỏ qua", gop)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.GOP = gop
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("GOP %d → %d", before.GOP, after.GOP)
	return step
}

func (t *tvtCamera) applyBitrate(ctx context.Context, ch, stream int, streamName string, kbps int, mode string) StepResult {
	step := StepResult{Step: fmt.Sprintf("bitrate %s", streamName)}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.BitRate == kbps && (mode == "" || strings.EqualFold(before.BitRateControl, mode)) {
		step.OK = true
		step.Detail = fmt.Sprintf("đã ở %d kbps (%s) từ trước, bỏ qua", kbps, before.BitRateControl)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.BitRate = kbps
		if mode != "" {
			si.BitRateControl = mode
		}
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("%d → %d kbps (%s)", before.BitRate, after.BitRate, after.BitRateControl)
	return step
}

func (t *tvtCamera) applySmartCodec(ctx context.Context, ch, stream int, enable bool) StepResult {
	step := StepResult{Step: "smart codec"}
	before, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err == nil && before.SmartCodec == enable {
		step.OK = true
		step.Detail = fmt.Sprintf("SmartCodec đã %v từ trước, bỏ qua", enable)
		return step
	}
	if err := t.client.SetStreamConfig(ctx, ch, stream, func(si *tvt.StreamInfo) {
		si.SmartCodec = enable
	}); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := t.client.GetStreamInfo(ctx, ch, stream)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("SmartCodec %v → %v", before.SmartCodec, after.SmartCodec)
	return step
}

func (t *tvtCamera) applyOSD(ctx context.Context, ch int, lines []string) StepResult {
	step := StepResult{Step: fmt.Sprintf("OSD K%d", ch+1)}
	n, err := t.client.SetOSD(ctx, ch, lines, []bool{true})
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = fmt.Sprintf("đã ghi %d dòng OSD", n)
	return step
}

func (t *tvtCamera) ChangePassword(_ context.Context, _, _ string) error {
	return fmt.Errorf("TVT NVR: vui lòng đổi mật khẩu trực tiếp trên giao diện quản trị NVR")
}

func (t *tvtCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return t.client.Snapshot(ctx, t.resolveChannel(channel), stream)
}

func (t *tvtCamera) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	return t.client.GetChannelOSD(ctx, t.resolveChannel(channel))
}

func (t *tvtCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return t.client.SetChannelName(ctx, t.resolveChannel(channel), name)
}

func (t *tvtCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	return t.client.SetOSD(ctx, t.resolveChannel(channel), lines, enabled)
}

func (t *tvtCamera) Reboot(ctx context.Context) error {
	return t.client.Reboot(ctx)
}

func (t *tvtCamera) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	return t.client.GetStorageInfo(ctx)
}

func (t *tvtCamera) FormatStorage(_ context.Context, _ string) error {
	return fmt.Errorf("TVT NVR: thao tác định dạng HDD qua kspcam bị khóa an toàn")
}

func (t *tvtCamera) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	return t.client.GetRemoteDevices(ctx)
}

func (t *tvtCamera) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	return t.client.FindRecordings(ctx, t.resolveChannel(channel), start, end)
}

func (t *tvtCamera) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return t.client.StreamPlaybackFast(ctx, w, t.resolveChannel(channel), start, end)
}

func (t *tvtCamera) StreamPlaybackFast(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return t.client.StreamPlaybackFast(ctx, w, t.resolveChannel(channel), start, end)
}

func (t *tvtCamera) StreamNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return t.client.StreamNative(ctx, w, t.resolveChannel(channel), start, end)
}
