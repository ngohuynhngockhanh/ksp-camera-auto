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

	Streams []int `json:"streams"` // which streams to touch; defaults to [main]
	Channel int   `json:"channel"`
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
	Close() error
}

// Open dials the device according to its configured vendor and returns a
// Camera implementation. Hikvision is not yet implemented (M6) and returns an
// error so callers can degrade gracefully.
func Open(ctx context.Context, d config.Device, timeout time.Duration) (Camera, error) {
	switch d.Vendor {
	case config.VendorDahua:
		cl, err := dahua.Dial(d.Addr(), d.Username, d.Password, timeout)
		if err != nil {
			return nil, fmt.Errorf("dahua dial %s: %w", d.Addr(), err)
		}
		return &dahuaCamera{client: cl}, nil
	case config.VendorHikvision:
		return nil, fmt.Errorf("hikvision not yet supported (M6)")
	default:
		return nil, fmt.Errorf("unknown vendor %q", d.Vendor)
	}
}

// dahuaCamera adapts *dahua.Client to the Camera interface.
type dahuaCamera struct {
	client *dahua.Client
}

func (d *dahuaCamera) Close() error { return d.client.Close() }

// Probe reads back main + sub1 + sub2 stream info for channel 0.
func (d *dahuaCamera) Probe(ctx context.Context) ([]StreamInfo, error) {
	var out []StreamInfo
	for _, s := range []dahua.Stream{dahua.StreamMain, dahua.StreamSub1, dahua.StreamSub2} {
		info, err := d.client.GetStreamInfo(0, s)
		if err != nil {
			return out, fmt.Errorf("probe stream %d: %w", s, err)
		}
		out = append(out, toStreamInfo(info))
	}
	return out, nil
}

// Apply pushes profile's settings to the device, one StepResult per action,
// calling emit as each step completes so the caller can stream progress live
// (emit may be nil). It never returns early on a per-step failure: every
// requested action is attempted so the caller sees the full picture. For
// each requested stream the order is codec -> resolution -> audio AAC, then
// smart codec is applied once per channel.
func (d *dahuaCamera) Apply(ctx context.Context, profile Profile, emit func(StepResult)) []StepResult {
	var steps []StepResult
	add := func(step StepResult) {
		steps = append(steps, step)
		if emit != nil {
			emit(step)
		}
	}
	ch := profile.Channel

	for _, s := range profile.streams() {
		ds := dahua.Stream(s)
		streamName := streamLabel(s)

		if profile.SetCodec {
			add(d.applyCodec(ch, ds, streamName, profile.Codec, profile.CodecProfile))
		}
		if profile.SetResolution {
			add(d.applyResolution(ch, ds, streamName, profile.Width, profile.Height))
		}
		if profile.SetAudioAAC {
			add(d.applyAudioAAC(ch, ds, streamName))
		}
	}

	if profile.SetSmartCodec {
		add(d.applySmartCodec(ch, profile.SmartCodec))
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

func (d *dahuaCamera) applySmartCodec(ch int, on bool) StepResult {
	step := StepResult{Step: "smart codec", Detail: onOff(on)}
	if err := d.client.SetSmartCodec(ch, on); err != nil {
		step.Err = err.Error()
		return step
	}
	step.OK = true
	return step
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
	}
}
