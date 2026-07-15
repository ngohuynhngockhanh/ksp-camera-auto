// Package camera provides a vendor-agnostic layer over the per-vendor camera
// clients (dahua, and later hikvision). The bulk orchestrator and web API talk
// only to this package, never to a vendor package directly.
package camera

import (
	"context"
	"fmt"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/hik"
)

// Stream selects an encoded stream. Values match dahua.Stream: 0=main,
// 1=sub1, 2=sub2.
type Stream = int

const (
	StreamMain Stream = iota
	StreamSub1
	StreamSub2
)

// Profile describes which encode settings to apply to a set of streams. A
// field is only applied when its "Set*" flag is true, so a Profile can carry
// a partial change (e.g. resolution only, leaving audio untouched).
type Profile struct {
	SetResolution bool `json:"setResolution"`
	Width         int  `json:"width"`
	Height        int  `json:"height"`

	SetCodec     bool   `json:"setCodec"`
	Codec        string `json:"codec"`        // Dahua compression value: H.265, H.264, H.264H, H.264B, MJPG
	CodecProfile string `json:"codecProfile"` // optional: Main/High/Baseline

	SetSmartCodec bool `json:"setSmartCodec"`
	SmartCodec    bool `json:"smartCodec"`

	SetAudioAAC bool `json:"setAudioAAC"`

	SetGOP bool `json:"setGop"`
	GOP    int  `json:"gop"` // I-frame interval, frames

	SetBitrate  bool   `json:"setBitrate"`
	Bitrate     int    `json:"bitrate"`     // Kbps
	BitrateMode string `json:"bitrateMode"` // "" = keep current, "CBR", "VBR"

	Streams  []int `json:"streams"`  // which streams to touch; defaults to [main]
	Channel  int   `json:"channel"`  // single channel (back-compat); 0-based
	Channels []int `json:"channels"` // multiple channels (NVR); 0-based; wins over Channel
}

// channelsList returns the 0-based channels to apply to: Channels if set,
// otherwise the single Channel. Used to drive NVR multi-channel bulk apply.
func (p Profile) channelsList() []int {
	if len(p.Channels) > 0 {
		return p.Channels
	}
	return []int{p.Channel}
}

// streams returns p.Streams, defaulting to [main] when empty.
func (p Profile) streams() []int {
	if len(p.Streams) == 0 {
		return []int{StreamMain}
	}
	return p.Streams
}

// StreamInfo mirrors dahua.StreamInfo but stays vendor-neutral so it can be
// reused once Hikvision support lands.
type StreamInfo struct {
	Channel     int    `json:"channel"`
	Stream      int    `json:"stream"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	Compression string `json:"compression"`
	Profile     string `json:"profile"`
	AudioCodec  string `json:"audioCodec"`
	AudioEnable bool   `json:"audioEnable"`
	SmartCodec  bool   `json:"smartCodec"`

	GOP         int    `json:"gop"`
	BitrateKbps int    `json:"bitrateKbps"`
	BitrateMode string `json:"bitrateMode"`
}

// StepResult records the outcome of one applied action (e.g. "set resolution
// on sub1"), so the UI can show a per-step audit trail rather than just a
// single pass/fail per device.
type StepResult struct {
	Step   string `json:"step"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
	Err    string `json:"err,omitempty"`
}

// Camera is the vendor-agnostic control surface the orchestrator drives.
type Camera interface {
	// Probe reads back current encode settings for the requested streams.
	Probe(ctx context.Context) ([]StreamInfo, error)
	// Apply pushes profile's settings to the device. emit is called with each
	// StepResult as it completes, so callers can stream progress live; the
	// full slice is also returned. emit may be nil.
	Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult
	// ChangePassword sets the device's account password (and username where the
	// vendor supports it). Dahua changes the logged-in account's password.
	ChangePassword(ctx context.Context, newUser, newPass string) error
	Close() error
}

// Open dials the device according to its configured vendor and returns a
// Camera implementation.
//
// Hikvision devices are controlled over ISAPI (HTTP), using the payload and
// transport layer in internal/isapi via the internal/hik adapter. Note that
// this is plain HTTP, not HTTPS: Device carries no per-device TLS flag yet,
// and ISAPI over HTTP is what Hikvision cameras serve by default. Unlike
// dahua.Dial, hik.Dial never touches the network -- it just builds the HTTP
// client -- so Open() always succeeds for a well-formed Device and any
// connectivity/credential problem surfaces from the first Probe/Apply call
// instead. The user's actual hardware also exposes a proprietary binary
// protocol on port 8000; that transport is a separate, later milestone (M6)
// and is not implemented here.
func Open(ctx context.Context, d config.Device, timeout time.Duration) (Camera, error) {
	switch d.Vendor {
	case config.VendorDahua:
		cl, err := dahua.Dial(d.Addr(), d.Username, d.Password, timeout)
		if err != nil {
			return nil, fmt.Errorf("dahua dial %s: %w", d.Addr(), err)
		}
		return &dahuaCamera{client: cl}, nil
	case config.VendorHikvision:
		cl, err := openHikClient(d, timeout)
		if err != nil {
			return nil, err
		}
		return &hikCamera{client: cl}, nil
	default:
		return nil, fmt.Errorf("unknown vendor %q", d.Vendor)
	}
}

// dahuaCamera adapts *dahua.Client to the Camera interface.
type dahuaCamera struct {
	client *dahua.Client
}

func (d *dahuaCamera) Close() error { return d.client.Close() }

func (d *dahuaCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return d.client.SetPassword(newPass)
}

// Probe reads back main + sub1 + sub2 stream info for channel 0.
func (d *dahuaCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	infos, err := d.client.ProbeAll()
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(infos))
	for _, i := range infos {
		out = append(out, toStreamInfo(i))
	}
	return out, nil
}

// Apply pushes profile's settings to the device, one StepResult per action,
// calling emit as each step completes so the caller can stream progress live
// (emit may be nil). It never returns early on a per-step failure: every
// requested action is attempted so the caller sees the full picture. For
// each requested stream the order is codec -> resolution -> GOP -> bitrate ->
// audio AAC, then smart codec is applied once per channel.
func (d *dahuaCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
	var steps []StepResult
	add := func(step StepResult) {
		steps = append(steps, step)
		if emit != nil {
			emit(step)
		}
	}
	for _, ch := range profile.channelsList() {
		for _, s := range profile.streams() {
			ds := dahua.Stream(s)
			streamName := fmt.Sprintf("K%d %s", ch+1, streamLabel(s))

			if profile.SetCodec {
				add(d.applyCodec(ch, ds, streamName, profile.Codec, profile.CodecProfile))
			}
			if profile.SetResolution {
				add(d.applyResolution(ch, ds, streamName, profile.Width, profile.Height))
			}
			if profile.SetGOP {
				add(d.applyGOP(ch, ds, streamName, profile.GOP))
			}
			if profile.SetBitrate {
				add(d.applyBitrate(ch, ds, streamName, profile.Bitrate, profile.BitrateMode))
			}
			if profile.SetAudioAAC {
				add(d.applyAudioAAC(ch, ds, streamName))
			}
		}

		if profile.SetSmartCodec {
			st := d.applySmartCodec(ch, profile.SmartCodec)
			st.Step = fmt.Sprintf("smart codec K%d", ch+1)
			add(st)
		}
	}

	return steps
}

func (d *dahuaCamera) applyCodec(ch int, s dahua.Stream, streamName string, compression, codecProfile string) StepResult {
	step := StepResult{Step: fmt.Sprintf("codec %s", streamName), Detail: compression}
	if err := d.client.SetCodec(ch, s, compression, codecProfile); err != nil {
		step.Err = err.Error()
		return step
	}
	// The device silently ignores unsupported codecs (returns ok but doesn't
	// change), so a read-back is mandatory here, not best-effort.
	info, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = info.Compression == compression
	if !step.OK {
		step.Detail = fmt.Sprintf("codec không đổi được (cam không hỗ trợ?) — hiện tại: %s", info.Compression)
		return step
	}
	if codecProfile != "" && info.Profile != codecProfile {
		step.OK = false
		step.Detail = fmt.Sprintf("%s OK nhưng profile không đổi được (hiện tại: %s)", compression, info.Profile)
		return step
	}
	step.Detail = fmt.Sprintf("codec %s OK", compression)
	return step
}

func (d *dahuaCamera) applyResolution(ch int, s dahua.Stream, streamName string, w, h int) StepResult {
	step := StepResult{Step: fmt.Sprintf("resolution %s", streamName), Detail: fmt.Sprintf("%dx%d", w, h)}
	if err := d.client.SetResolution(ch, s, w, h); err != nil {
		step.Err = err.Error()
		return step
	}
	info, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = info.Width == w && info.Height == h
	if !step.OK {
		step.Detail = fmt.Sprintf("độ phân giải không đổi được (đọc lại: %dx%d)", info.Width, info.Height)
		return step
	}
	step.Detail = fmt.Sprintf("độ phân giải %dx%d OK", w, h)
	return step
}

func (d *dahuaCamera) applyAudioAAC(ch int, s dahua.Stream, streamName string) StepResult {
	step := StepResult{Step: fmt.Sprintf("audio AAC %s", streamName)}
	if err := d.client.SetAudioAAC(ch, s); err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = "AAC bật"
	return step
}

func (d *dahuaCamera) applyGOP(ch int, s dahua.Stream, streamName string, gop int) StepResult {
	step := StepResult{Step: fmt.Sprintf("GOP %s", streamName), Detail: fmt.Sprintf("%d", gop)}
	before, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	if before.GOP == gop {
		step.OK = true
		step.Detail = fmt.Sprintf("GOP đã đúng %d", gop)
		return step
	}
	if err := d.client.SetGOP(ch, s, gop); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	switch {
	case after.GOP == gop:
		step.OK = true
		step.Detail = fmt.Sprintf("GOP %d OK", gop)
	case after.GOP != before.GOP:
		step.OK = true
		step.Detail = fmt.Sprintf("yêu cầu GOP %d, thiết bị kẹp còn %d", gop, after.GOP)
	default:
		step.Detail = fmt.Sprintf("GOP không đổi được (đọc lại: %d)", after.GOP)
	}
	return step
}

func (d *dahuaCamera) applyBitrate(ch int, s dahua.Stream, streamName string, kbps int, mode string) StepResult {
	label := fmt.Sprintf("%d Kbps", kbps)
	if mode != "" {
		label += " " + mode
	}
	step := StepResult{Step: fmt.Sprintf("bitrate %s", streamName), Detail: label}
	before, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	modeOK := mode == "" || before.BitRateControl == mode
	if before.BitRate == kbps && modeOK {
		step.OK = true
		step.Detail = fmt.Sprintf("bitrate đã đúng %s", label)
		return step
	}
	if err := d.client.SetBitrate(ch, s, kbps, mode); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := d.client.GetStreamInfo(ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	afterModeOK := mode == "" || after.BitRateControl == mode
	switch {
	case after.BitRate == kbps && afterModeOK:
		step.OK = true
		step.Detail = fmt.Sprintf("bitrate %s OK", label)
	case after.BitRate != before.BitRate || after.BitRateControl != before.BitRateControl:
		step.OK = true
		step.Detail = fmt.Sprintf("yêu cầu %s, thiết bị nhận %d Kbps %s", label, after.BitRate, after.BitRateControl)
	default:
		step.Detail = fmt.Sprintf("bitrate không đổi được (đọc lại: %d Kbps %s)", after.BitRate, after.BitRateControl)
	}
	return step
}

func (d *dahuaCamera) applySmartCodec(ch int, on bool) StepResult {
	step := StepResult{Step: "smart codec", Detail: onOff(on)}
	if err := d.client.SetSmartCodec(ch, on); err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	return step
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func onOff(b bool) string {
	if b {
		return "bật"
	}
	return "tắt"
}

func streamLabel(s int) string {
	switch s {
	case StreamMain:
		return "main"
	case StreamSub1:
		return "sub1"
	case StreamSub2:
		return "sub2"
	default:
		return fmt.Sprintf("stream%d", s)
	}
}

func toStreamInfo(i dahua.StreamInfo) StreamInfo {
	return StreamInfo{
		Channel:     i.Channel,
		Stream:      int(i.Stream),
		Width:       i.Width,
		Height:      i.Height,
		FPS:         i.FPS,
		Compression: i.Compression,
		Profile:     i.Profile,
		AudioCodec:  i.AudioCodec,
		AudioEnable: i.AudioEnable,
		SmartCodec:  i.SmartCodec,
		GOP:         i.GOP,
		BitrateKbps: i.BitRate,
		BitrateMode: i.BitRateControl,
	}
}

// hikCamera adapts *hik.Client to the Camera interface over ISAPI.
//
// Channel numbering: Profile.Channel is 0-based (matching the Dahua
// convention, so the zero value targets a single-channel device's only
// stream), while Hikvision's native ISAPI channel numbers are 1-based (101 =
// channel 1 main stream). isapiChannel converts between the two at the
// boundary; every hik.Client / isapi call below uses the converted value.
type hikCamera struct {
	client *hik.Client
}

func (h *hikCamera) Close() error { return h.client.Close() }

func (h *hikCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return h.client.SetPassword(ctx, newUser, newPass)
}

// isapiChannel converts a vendor-neutral (0-based) Profile.Channel to
// Hikvision's native (1-based) channel number.
func isapiChannel(profileChannel int) int { return profileChannel + 1 }

// Probe reads back main + sub1 + sub2 stream info for the default channel
// (Profile.Channel's zero value, i.e. ISAPI channel 1).
func (h *hikCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	infos, err := h.client.ProbeAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StreamInfo, 0, len(infos))
	for _, i := range infos {
		out = append(out, hikToStreamInfo(i))
	}
	return out, nil
}

// Apply pushes profile's settings to the device, one StepResult per action,
// calling emit as each step completes (emit may be nil). It never returns
// early on a per-step failure: every requested action is attempted so the
// caller sees the full picture. For each requested stream the order is
// codec -> resolution -> smart codec -> GOP -> bitrate -> audio AAC. Smart
// codec runs before GOP/bitrate so the bitrate step's smart-average branch
// observes the stream's final smart-codec state.
//
// Unlike Dahua's SmartEncode (a single per-physical-channel switch), ISAPI's
// smartCodec toggle is a resource on each compound streaming-channel id
// (e.g. /ISAPI/Streaming/channels/101/smartCodec), so it is applied once per
// requested stream here rather than once per profile.
func (h *hikCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
	var steps []StepResult
	add := func(step StepResult) {
		steps = append(steps, step)
		if emit != nil {
			emit(step)
		}
	}
	for _, pc := range profile.channelsList() {
		ch := isapiChannel(pc)
		for _, s := range profile.streams() {
			streamName := fmt.Sprintf("K%d %s", pc+1, streamLabel(s))

			if profile.SetCodec {
				add(h.applyCodec(ctx, ch, s, streamName, profile.Codec))
			}
			if profile.SetResolution {
				add(h.applyResolution(ctx, ch, s, streamName, profile.Width, profile.Height))
			}
			if profile.SetSmartCodec {
				add(h.applySmartCodec(ctx, ch, s, streamName, profile.SmartCodec))
			}
			if profile.SetGOP {
				add(h.applyGOP(ctx, ch, s, streamName, profile.GOP))
			}
			if profile.SetBitrate {
				add(h.applyBitrate(ctx, ch, s, streamName, profile.Bitrate, profile.BitrateMode))
			}
			if profile.SetAudioAAC {
				add(h.applyAudioAAC(ctx, ch, s, streamName))
			}
		}
	}

	return steps
}

// hikCodec maps the vendor-neutral (Dahua-flavored) Profile.Codec value to a
// Hikvision videoCodecType. Dahua encodes H.264 profile into the compression
// string itself (H.264H = High, H.264B = Baseline); ISAPI has no
// videoCodecType equivalent for that distinction (profile is a separate
// field this milestone doesn't touch), so both collapse to plain H.264.
func hikCodec(profileCodec string) string {
	switch profileCodec {
	case "H.265":
		return isapiCodecH265
	case "H.264", "H.264H", "H.264B":
		return isapiCodecH264
	case "MJPG":
		return isapiCodecMJPEG
	default:
		return profileCodec
	}
}

func (h *hikCamera) applyCodec(ctx context.Context, ch, s int, streamName, profileCodec string) StepResult {
	codec := hikCodec(profileCodec)
	step := StepResult{Step: fmt.Sprintf("codec %s", streamName), Detail: codec}
	if err := h.client.SetCodec(ctx, ch, s, codec); err != nil {
		step.Err = err.Error()
		return step
	}
	// Mirror Dahua's behavior: unsupported codecs can be silently ignored by
	// the device, so a read-back is mandatory here, not best-effort.
	info, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = info.Codec == codec
	if !step.OK {
		step.Detail = fmt.Sprintf("codec không đổi được (cam không hỗ trợ?) — hiện tại: %s", info.Codec)
		return step
	}
	step.Detail = fmt.Sprintf("codec %s OK", codec)
	return step
}

func (h *hikCamera) applyResolution(ctx context.Context, ch, s int, streamName string, w, h2 int) StepResult {
	step := StepResult{Step: fmt.Sprintf("resolution %s", streamName), Detail: fmt.Sprintf("%dx%d", w, h2)}
	if err := h.client.SetResolution(ctx, ch, s, w, h2, 0); err != nil {
		step.Err = err.Error()
		return step
	}
	info, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = info.Width == w && info.Height == h2
	if !step.OK {
		step.Detail = fmt.Sprintf("độ phân giải không đổi được (đọc lại: %dx%d)", info.Width, info.Height)
		return step
	}
	step.Detail = fmt.Sprintf("độ phân giải %dx%d OK", w, h2)
	return step
}

func (h *hikCamera) applyAudioAAC(ctx context.Context, ch, s int, streamName string) StepResult {
	step := StepResult{Step: fmt.Sprintf("audio AAC %s", streamName)}
	if err := h.client.SetAudioAAC(ctx, ch, s); err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	step.Detail = "AAC bật"
	return step
}

// applyGOP uses tolerance ±1 frame on the read-back equality check, to absorb
// any rounding the device applies internally.
func (h *hikCamera) applyGOP(ctx context.Context, ch, s int, streamName string, gop int) StepResult {
	step := StepResult{Step: fmt.Sprintf("GOP %s", streamName), Detail: fmt.Sprintf("%d", gop)}
	before, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	if abs(before.GOP-gop) <= 1 {
		step.OK = true
		step.Detail = fmt.Sprintf("GOP đã đúng %d", gop)
		return step
	}
	if err := h.client.SetGOP(ctx, ch, s, gop); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	switch {
	case abs(after.GOP-gop) <= 1:
		step.OK = true
		step.Detail = fmt.Sprintf("GOP %d OK", gop)
	case after.GOP != before.GOP:
		step.OK = true
		step.Detail = fmt.Sprintf("yêu cầu GOP %d, thiết bị kẹp còn %d", gop, after.GOP)
	default:
		step.Detail = fmt.Sprintf("GOP không đổi được (đọc lại: %d)", after.GOP)
	}
	return step
}

func (h *hikCamera) applyBitrate(ctx context.Context, ch, s int, streamName string, kbps int, mode string) StepResult {
	label := fmt.Sprintf("%d Kbps", kbps)
	if mode != "" {
		label += " " + mode
	}
	step := StepResult{Step: fmt.Sprintf("bitrate %s", streamName), Detail: label}
	before, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	modeOK := mode == "" || before.BitrateMode == mode
	if before.BitrateKbps == kbps && modeOK {
		step.OK = true
		step.Detail = fmt.Sprintf("bitrate đã đúng %s", label)
		return step
	}
	if err := h.client.SetBitrate(ctx, ch, s, kbps, mode); err != nil {
		step.Err = err.Error()
		return step
	}
	after, err := h.client.GetStreamInfo(ctx, ch, s)
	if err != nil {
		step.Err = err.Error()
		return step
	}
	afterModeOK := mode == "" || after.BitrateMode == mode
	switch {
	case after.BitrateKbps == kbps && afterModeOK:
		step.OK = true
		step.Detail = fmt.Sprintf("bitrate %s OK", label)
	case after.BitrateKbps != before.BitrateKbps || after.BitrateMode != before.BitrateMode:
		step.OK = true
		step.Detail = fmt.Sprintf("yêu cầu %s, thiết bị nhận %d Kbps %s", label, after.BitrateKbps, after.BitrateMode)
	default:
		step.Detail = fmt.Sprintf("bitrate không đổi được (đọc lại: %d Kbps %s)", after.BitrateKbps, after.BitrateMode)
	}
	return step
}

func (h *hikCamera) applySmartCodec(ctx context.Context, ch, s int, streamName string, on bool) StepResult {
	step := StepResult{Step: fmt.Sprintf("smart codec %s", streamName), Detail: onOff(on)}
	if err := h.client.SetSmartCodec(ctx, ch, s, on); err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	return step
}

// isapiCodecH264/H265/MJPEG mirror the isapi package's Codec* constants.
// They're redeclared here (rather than importing internal/isapi's constants
// directly into hikCodec's callers) purely so this file's vendor-facing
// vocabulary stays self-contained and greppable; the values must stay in
// sync with internal/isapi.CodecH264/CodecH265/CodecMJPEG.
const (
	isapiCodecH264  = "H.264"
	isapiCodecH265  = "H.265"
	isapiCodecMJPEG = "MJPEG"
)

func hikToStreamInfo(i hik.StreamInfo) StreamInfo {
	return StreamInfo{
		Channel:     i.Channel,
		Stream:      i.Stream,
		Width:       i.Width,
		Height:      i.Height,
		FPS:         i.FPS,
		Compression: i.Codec,
		AudioCodec:  i.AudioCodec,
		AudioEnable: i.AudioEnable,
		SmartCodec:  i.SmartCodec,
		GOP:         i.GOP,
		BitrateKbps: i.BitrateKbps,
		BitrateMode: i.BitrateMode,
	}
}
