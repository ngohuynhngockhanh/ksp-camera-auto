// Package camera provides a vendor-agnostic layer over the per-vendor camera
// clients (dahua, and later hikvision). The bulk orchestrator and web API talk
// only to this package, never to a vendor package directly.
package camera

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// kbvisionFallbackPort is the DVRIP port some KBVision (Dahua OEM) devices
// use instead of the 37777 default. See Open()'s VendorDahua case.
const kbvisionFallbackPort = 8888

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

	// Name is the device's own channel name (not our inventory label).
	// OSDLines is the on-screen custom-title text overlay, when the device
	// exposes it. Both are populated once per channel (not duplicated per
	// network call) and best-effort: left empty if the vendor/firmware
	// doesn't support them. For Hikvision, Probe leaves OSDLines empty even
	// on supported devices — reading it costs one extra request per channel,
	// which Probe skips to stay cheap on large NVRs; use ChannelInfo to fetch
	// it for one channel on demand (e.g. when a user opens its edit panel).
	Name     string   `json:"name,omitempty"`
	OSDLines []string `json:"osdLines,omitempty"`
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
	// Snapshot fetches a single JPEG frame for one channel/stream. Dahua has
	// no per-stream snapshot selector, so it ignores stream and always
	// returns whatever its snapshot pipeline serves.
	Snapshot(ctx context.Context, channel, stream int) ([]byte, error)
	// ChannelInfo reads back the device's own name and OSD text lines (plus
	// each line's on-screen enable state) for one channel. osdSupported is
	// false (with a nil error) when the vendor/firmware doesn't expose OSD
	// text at all, so callers can show "not supported" instead of an error.
	ChannelInfo(ctx context.Context, channel int) (name string, osdLines []string, osdEnabled []bool, osdSupported bool, err error)
	// SetChannelName writes the device's own channel name (distinct from our
	// inventory label, which /api/cameras already covers).
	SetChannelName(ctx context.Context, channel int, name string) error
	// SetOSDLines writes free-text OSD lines and each line's on-screen enable
	// state for a channel, applying as many as the device has slots for
	// (returned as applied). enabled[i] wins when present; a shorter/nil
	// enabled falls back to enabling exactly the lines getting non-empty
	// text. Returns an error wrapping ErrOSDUnsupported-equivalent state as
	// osdSupported=false would from ChannelInfo — callers should check
	// ChannelInfo first.
	SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (applied int, err error)
	Close() error
}

// PictureSettings is implemented by cameras that support Dahua-style
// color/picture tuning (VideoColor + VideoInOptions — brightness/contrast/
// hue/saturation, flip/mirror/rotate, white balance, and the day/night
// exposure sub-profiles). Only dahuaCamera implements it: Hikvision has no
// equivalent in this codebase, and this deliberately isn't folded into the
// common Camera interface (unlike ChannelInfo/OSD, which both vendors
// support in some form) — callers must type-assert:
//
//	ps, ok := cam.(camera.PictureSettings)
type PictureSettings interface {
	// GetPicture reads back channel's current color+options config exactly
	// as the device returns it (see dahua.GetPicture for why this is a raw
	// map rather than a hand-typed struct).
	GetPicture(ctx context.Context, channel int) (color, options map[string]any, err error)
	// SetPicture merges colorChanges/optionsChanges into channel's config
	// and returns the post-write, live device state, so callers can report
	// which fields actually took (the device may clamp or ignore some).
	SetPicture(ctx context.Context, channel int, colorChanges, optionsChanges map[string]any) (color, options map[string]any, err error)
	// GetPictureCaps reads channel's video-input capability flags, so the UI
	// can disable controls the device doesn't support.
	GetPictureCaps(ctx context.Context, channel int) (map[string]any, error)
}

// NetworkSettings is implemented by cameras that support reading/writing
// device-level network config (static IP, Wi-Fi). Only dahuaCamera
// implements it. This is a genuinely high-risk surface — a bad IP/mask or
// wrong Wi-Fi credential can make the device unreachable — so callers must
// type-assert (`ns, ok := cam.(camera.NetworkSettings)`) and should always
// read current config back after a write to confirm it applied as expected.
type NetworkSettings interface {
	GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error)
	// SetStaticIP writes one interface's IP config. See
	// dahua.Client.SetStaticIP for validation (ip/mask/gateway/dns must be
	// well-formed IPv4 or the call fails before touching the device).
	SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error
	GetWiFiConfig(ctx context.Context) (map[string]map[string]any, error)
	SetWiFiConfig(ctx context.Context, iface, ssid, password, encryption string) error
	// ScanWiFi triggers a live access-point scan. Requires the device's HTTP
	// CGI port (80) to be reachable, unlike the rest of this interface which
	// rides the existing DVRIP session.
	ScanWiFi(ctx context.Context) ([]dahua.WiFiAP, error)
}

// Rebooter is implemented by cameras that support a remote reboot. Both
// dahuaCamera (DVRIP magicBox.reboot) and hikCamera (ISAPI /System/reboot)
// implement it; callers type-assert.
type Rebooter interface {
	Reboot(ctx context.Context) error
}

// StorageManager is implemented by cameras that expose on-device storage
// (SD card / NVR HDD bay) info and formatting. Both dahuaCamera (DVRIP
// storage.getDeviceAllInfo) and hikCamera (ISAPI /ContentMgmt/Storage)
// implement it; callers type-assert. Formatting erases all recordings, so
// the UI must require explicit confirmation before calling Format — note
// hikCamera.FormatStorage always returns an error (unimplemented for Hik,
// see its doc comment), so only the read side works there today.
type StorageManager interface {
	GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error)
	FormatStorage(ctx context.Context, name string) error
}

// RemoteDeviceLister is implemented by an NVR that can report the camera
// connected to each of its channels (Dahua RemoteDevice config / Hik ISAPI
// InputProxy channels). Both dahuaCamera and hikCamera implement it; callers
// type-assert. Used to auto-map cameras to NVR channels.
type RemoteDeviceLister interface {
	GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error)
}

// AutoRebootConfig is implemented by cameras that expose a scheduled
// auto-reboot (Dahua's AutoMaintain table). Dahua-only.
type AutoRebootConfig interface {
	GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error)
	SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error
}

// Recorder is implemented by cameras that expose recorded footage: a timeline
// listing of stored segments, and a streamed remux of an arbitrary time range.
// Dahua-only (mediaFileFind + RTSP playback). callers type-assert.
type Recorder interface {
	// FindRecordings lists stored segments on a channel between start and end
	// (device-local times).
	FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error)
	// StreamPlayback writes the [start,end] recording for a channel to w as a
	// fragmented MP4, streamed with no on-box buffering (see
	// dahua.StreamPlayback).
	StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error
	// StreamDav writes the [start,end] recording for a channel to w as the
	// camera's native .dav (DHAV) — byte-exact, no remux (see dahua.StreamDav).
	// Requires the DVRIP config port (unlike StreamPlayback's RTSP).
	StreamDav(ctx context.Context, w io.Writer, channel int, start, end time.Time) error
}

// PTZControl is implemented by cameras that support live pan/tilt/zoom.
// Only dahuaCamera implements it (Dahua HTTP CGI); callers type-assert.
type PTZControl interface {
	// PTZMove issues one PTZ command. start=true begins motion for code,
	// start=false stops it — a UI pad calls start on press and stop on
	// release. speed is the pan/tilt speed (1-8; ignored for zoom/focus).
	PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error
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
		if err != nil && errors.Is(err, dahua.ErrDialUnreachable) && d.Port != kbvisionFallbackPort {
			// KBVision (a Dahua OEM) sometimes serves DVRIP on 8888 instead of
			// the 37777 default. Only retry when the configured port genuinely
			// couldn't be reached at the TCP level (ErrDialUnreachable) so a
			// real login/credential failure on 37777 is never masked by a
			// second, confusing attempt. This fallback is per-connection only:
			// it never rewrites d.Port in the saved inventory.
			fallbackAddr := fmt.Sprintf("%s:%d", d.Host, kbvisionFallbackPort)
			if cl2, err2 := dahua.Dial(fallbackAddr, d.Username, d.Password, timeout); err2 == nil {
				cl, err = cl2, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("dahua dial %s: %w", d.Addr(), err)
		}
		return &dahuaCamera{client: cl, device: d, timeout: timeout}, nil
	case config.VendorHikvision:
		cl, err := openHikClient(d, timeout)
		if err != nil {
			return nil, err
		}
		return &hikCamera{client: cl, device: d, timeout: timeout}, nil
	default:
		return nil, fmt.Errorf("unknown vendor %q", d.Vendor)
	}
}

// dahuaCamera adapts *dahua.Client to the Camera interface. device/timeout
// are kept alongside the DVRIP client so Snapshot can open a separate plain
// HTTP+Digest connection (Dahua's snapshot.cgi is not reachable over the
// DVRIP session) without re-deriving credentials from anywhere else.
type dahuaCamera struct {
	client  *dahua.Client
	device  config.Device
	timeout time.Duration
}

func (d *dahuaCamera) Close() error { return d.client.Close() }

func (d *dahuaCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return d.client.SetPassword(newPass)
}

// Snapshot fetches a single JPEG frame via Dahua's HTTP CGI (a separate
// connection from the DVRIP session, see dahua.GetSnapshot); stream is
// ignored (snapshot.cgi has no sub-stream selector).
func (d *dahuaCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return dahua.GetSnapshot(ctx, d.device.Host, d.device.Username, d.device.Password, channel, d.timeout)
}

// ChannelInfo reads back the channel's own name and OSD lines + enable state.
func (d *dahuaCamera) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	name, err := d.client.GetChannelTitle(channel)
	if err != nil {
		return "", nil, nil, false, err
	}
	lines, enabled, err := d.client.GetOSDLines(channel)
	if err != nil {
		if errors.Is(err, dahua.ErrOSDUnsupported) {
			return name, nil, nil, false, nil
		}
		return name, nil, nil, false, err
	}
	return name, lines, enabled, true, nil
}

// SetChannelName writes the device's own channel name.
func (d *dahuaCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return d.client.SetChannelTitle(channel, name)
}

// Reboot restarts the device via DVRIP magicBox.reboot.
func (d *dahuaCamera) Reboot(ctx context.Context) error { return d.client.Reboot() }

// FindRecordings lists stored recording segments on a channel over a range.
func (d *dahuaCamera) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	return d.client.FindRecordings(channel, start, end)
}

// StreamPlayback streams a channel's [start,end] recording to w as MP4.
func (d *dahuaCamera) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return dahua.StreamPlayback(ctx, w, d.device.Host, d.device.Username, d.device.Password, channel, start, end)
}

// StreamDav streams a channel's [start,end] recording to w as native .dav.
func (d *dahuaCamera) StreamDav(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	return dahua.StreamDav(ctx, w, d.device.Host, d.device.Username, d.device.Password, channel, start, end)
}

// GetStorageInfo reads the device's SD-card / storage status.
func (d *dahuaCamera) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	return d.client.GetStorageInfo()
}

// FormatStorage formats one storage device by name — ERASES ALL DATA.
func (d *dahuaCamera) FormatStorage(ctx context.Context, name string) error {
	return d.client.FormatStorage(name)
}

// GetRemoteDevices lists the camera connected to each NVR channel.
func (d *dahuaCamera) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	return d.client.GetRemoteDevices()
}

// GetAutoReboot reads the scheduled auto-reboot (AutoMaintain).
func (d *dahuaCamera) GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error) {
	return d.client.GetAutoReboot()
}

// SetAutoReboot writes the scheduled auto-reboot (AutoMaintain).
func (d *dahuaCamera) SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error {
	return d.client.SetAutoReboot(ar)
}

// SetOSDLines writes free-text OSD lines and enable state for a channel.
func (d *dahuaCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	return d.client.SetOSDLines(channel, lines, enabled)
}

// GetPicture reads back the channel's current color+options config.
func (d *dahuaCamera) GetPicture(ctx context.Context, channel int) (map[string]any, map[string]any, error) {
	return d.client.GetPicture(channel)
}

// SetPicture merges colorChanges/optionsChanges into the channel's config
// and returns the post-write, live device state.
func (d *dahuaCamera) SetPicture(ctx context.Context, channel int, colorChanges, optionsChanges map[string]any) (map[string]any, map[string]any, error) {
	return d.client.SetPicture(channel, colorChanges, optionsChanges)
}

// GetPictureCaps reads the channel's video-input capability flags via HTTP
// CGI (a separate connection from the DVRIP session, same as Snapshot).
func (d *dahuaCamera) GetPictureCaps(ctx context.Context, channel int) (map[string]any, error) {
	return dahua.GetVideoInputCaps(ctx, d.device.Host, d.device.Username, d.device.Password, channel, d.timeout)
}

// GetNetworkConfig reads the device's static IP / DHCP config for every
// interface.
func (d *dahuaCamera) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	return d.client.GetNetworkConfig()
}

// SetStaticIP writes one interface's IP configuration.
func (d *dahuaCamera) SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	return d.client.SetStaticIP(iface, dhcpEnable, ip, mask, gateway, dns)
}

// GetWiFiConfig reads the device's Wi-Fi (WLan) config for every interface.
func (d *dahuaCamera) GetWiFiConfig(ctx context.Context) (map[string]map[string]any, error) {
	return d.client.GetWiFiConfig()
}

// SetWiFiConfig writes one Wi-Fi interface's SSID/password.
func (d *dahuaCamera) SetWiFiConfig(ctx context.Context, iface, ssid, password, encryption string) error {
	return d.client.SetWiFiConfig(iface, ssid, password, encryption)
}

// ScanWiFi triggers a live access-point scan. Prefers the DVRIP-native
// netApp.scanWLanDevices (rides the existing session, works on NAT'd/DVRIP-only
// cameras where CGI port 80 is unreachable), falling back to the HTTP CGI scan
// for firmware that doesn't expose the RPC — mirroring the PTZ DVRIP-then-CGI
// strategy.
func (d *dahuaCamera) ScanWiFi(ctx context.Context) ([]dahua.WiFiAP, error) {
	// The DVRIP scan (netApp.scanWLanDevices) is the real path on these cams, but
	// a Wi-Fi radio scan is flaky: it can fail transiently when a scan is already
	// running, the radio is warming up, or several DVRIP sessions hit the camera
	// back-to-back (opening the network tab fires network+wifi reads first). One
	// retry after a short pause clears almost all of those.
	aps, rpcErr := d.client.ScanWiFiRPC("")
	if rpcErr != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
		aps, rpcErr = d.client.ScanWiFiRPC("")
	}
	if rpcErr == nil {
		return aps, nil
	}
	// CGI (port 80) is unreachable on many of these firmwares, so don't let its
	// bare "EOF" mask the real DVRIP error — surface both when everything fails.
	cgiAps, cgiErr := dahua.ScanWiFi(ctx, d.device.Host, d.device.Username, d.device.Password, d.timeout)
	if cgiErr == nil {
		return cgiAps, nil
	}
	return nil, fmt.Errorf("dvrip: %v; cgi: %v", rpcErr, cgiErr)
}

// PTZMove issues one PTZ command, preferring the DVRIP JSON-RPC session (the
// same one already open) since modern Dahua firmware tends to reject the HTTP
// CGI (ptz.cgi) with "Bad Request" — the same pattern seen with snapshot.cgi.
// Falls back to CGI when DVRIP can't express the move (focus/iris, which have
// no continuous-move RPC here) or errors.
func (d *dahuaCamera) PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error {
	err := d.client.PTZControl(channel, code, speed, start)
	if err == nil {
		return nil
	}
	// DVRIP errored — try CGI as a last resort (dead on cams with no :80).
	cgiErr := dahua.PTZMove(ctx, d.device.Host, d.device.Username, d.device.Password, channel, code, speed, start, d.timeout)
	if cgiErr == nil {
		return nil
	}
	return fmt.Errorf("dvrip: %v; cgi: %v", err, cgiErr)
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
		Name:        i.Name,
		OSDLines:    i.OSDLines,
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
	client  *hik.Client
	device  config.Device
	timeout time.Duration

	// loc caches the device's own UTC offset (from hik.Client.DeviceLocation)
	// across the several Recorder calls one request can make (FindRecordings,
	// then StreamPlayback/StreamDav), so it's resolved at most once per
	// hikCamera instance rather than once per call. hikCamera instances are
	// created fresh per request by Open and never shared across goroutines,
	// so no locking is needed here.
	loc *time.Location
}

func (h *hikCamera) Close() error { return h.client.Close() }

// location resolves and caches the device's own UTC offset, used to convert
// the review UI's device-local recording times for ISAPI's UTC-only content
// search/download (see hik.Client.DeviceLocation).
func (h *hikCamera) location(ctx context.Context) (*time.Location, error) {
	if h.loc != nil {
		return h.loc, nil
	}
	loc, err := h.client.DeviceLocation(ctx)
	if err != nil {
		return nil, err
	}
	h.loc = loc
	return loc, nil
}

func (h *hikCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return h.client.SetPassword(ctx, newUser, newPass)
}

// Snapshot fetches a single JPEG frame for one channel/stream via ISAPI.
func (h *hikCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return h.client.GetSnapshot(ctx, isapiChannel(channel), stream)
}

// ChannelInfo reads back the channel's own name and OSD overlay lines +
// enable state.
func (h *hikCamera) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	ch := isapiChannel(channel)
	name, err := h.client.GetChannelName(ctx, ch)
	if err != nil {
		return "", nil, nil, false, err
	}
	lines, enabled, err := h.client.GetOverlayText(ctx, ch)
	if err != nil {
		if errors.Is(err, hik.ErrOverlayUnsupported) {
			return name, nil, nil, false, nil
		}
		return name, nil, nil, false, err
	}
	return name, lines, enabled, true, nil
}

// SetChannelName writes the device's own channel name.
func (h *hikCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return h.client.SetChannelName(ctx, isapiChannel(channel), name)
}

// SetOSDLines writes free-text OSD overlay lines and enable state for a
// channel.
func (h *hikCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	return h.client.SetOverlayText(ctx, isapiChannel(channel), lines, enabled)
}

// FindRecordings lists stored recording segments on a channel over a range
// via ISAPI content search (see hik.Client.FindRecordings).
func (h *hikCamera) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	return h.client.FindRecordings(ctx, isapiChannel(channel), start, end)
}

// StreamPlayback streams a channel's [start,end] recording to w as a
// fragmented MP4, remuxed from Hikvision RTSP playback-by-time (see
// hik.StreamPlayback) — accurate to the requested range, browser-playable.
func (h *hikCamera) StreamPlayback(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	loc, err := h.location(ctx)
	if err != nil {
		return fmt.Errorf("hik playback %s: %w", h.device.Host, err)
	}
	return hik.StreamPlayback(ctx, w, h.device.Host, h.device.Port, h.device.Username, h.device.Password, isapiChannel(channel), start, end, loc)
}

// StreamDav streams a channel's [start,end] recording to w as Hikvision's
// native proprietary container (magic "IMKH" — the Hik analog of Dahua's
// .dav; see hik.StreamNative). Segment-coarse, not range-exact, but no
// ffmpeg/remux — the fast option when precise cut boundaries don't matter.
func (h *hikCamera) StreamDav(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	loc, err := h.location(ctx)
	if err != nil {
		return fmt.Errorf("hik native download %s: %w", h.device.Host, err)
	}
	return hik.StreamNative(ctx, w, h.device.Host, h.device.Port, h.device.Username, h.device.Password, isapiChannel(channel), start, end, loc)
}

// isapiChannel converts a vendor-neutral (0-based) Profile.Channel to
// Hikvision's native (1-based) channel number.
func isapiChannel(profileChannel int) int { return profileChannel + 1 }

// GetStorageInfo reads the NVR's HDD bays via ISAPI
// (/ISAPI/ContentMgmt/Storage), mapped onto the shared dahua.StorageDevice
// shape (see hik.Client.GetStorageInfo) — so the NVR scan flow's
// dahua.HasUsableStorage check and the web UI's storage view work unchanged
// for a Hikvision NVR.
func (h *hikCamera) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	return h.client.GetStorageInfo(ctx)
}

// errHikFormatUnsupported is returned by FormatStorage: Hikvision's ISAPI
// storage-format operation (its exact resource/body, and the destructive
// blast radius of getting it wrong) was not part of this milestone's live
// verification, so this deliberately refuses rather than guessing at a
// data-erasing call. hikCamera still satisfies camera.StorageManager (the
// read side, GetStorageInfo, is fully implemented); only the "format"
// button is unavailable for Hik devices in the UI.
var errHikFormatUnsupported = errors.New("hik: storage format not supported over ISAPI")

// FormatStorage is intentionally unimplemented for Hikvision — see
// errHikFormatUnsupported.
func (h *hikCamera) FormatStorage(ctx context.Context, name string) error {
	return errHikFormatUnsupported
}

// GetRemoteDevices lists the camera connected to each NVR channel via ISAPI
// (/ISAPI/ContentMgmt/InputProxy/channels), mapped onto the shared
// dahua.RemoteChannel shape (see hik.Client.GetRemoteDevices) — so the NVR
// scan flow (channel -> inventory camera matching) works unchanged for a
// Hikvision NVR.
func (h *hikCamera) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	return h.client.GetRemoteDevices(ctx)
}

// errHikWiFiUnsupported is returned by hikCamera's Wi-Fi methods: this
// milestone implements static-IP config for Hikvision (LAN and Wi-Fi
// interfaces alike) but not SSID/password provisioning or live AP scanning.
// GetWiFiConfig returning an error makes the web UI hide the Wi-Fi SSID
// section, exactly as it does for a Dahua device with no radio.
var errHikWiFiUnsupported = errors.New("cấu hình SSID/mật khẩu Wi-Fi chưa hỗ trợ cho Hikvision (chỉ đặt IP tĩnh)")

// GetNetworkConfig reads every interface's static IP / DHCP config over ISAPI
// and maps it into the same NetworkConfig shape the Dahua path produces, so
// the web API and UI render Hikvision network settings with no vendor-specific
// branch. Interfaces are keyed by the device's own id ("1" = LAN, "2" = Wi-Fi
// on wireless models); the first is reported as the default.
func (h *hikCamera) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	ifaces, err := h.client.GetNetworkInterfaces(ctx)
	if err != nil {
		return dahua.NetworkConfig{}, err
	}
	cfg := dahua.NetworkConfig{Interfaces: map[string]map[string]any{}}
	for i, ni := range ifaces {
		if i == 0 {
			cfg.DefaultInterface = ni.ID
		}
		dns := make([]any, len(ni.DNS))
		for j, d := range ni.DNS {
			dns[j] = d
		}
		cfg.Interfaces[ni.ID] = map[string]any{
			"DhcpEnable":      ni.DhcpEnable,
			"IPAddress":       ni.IPAddress,
			"SubnetMask":      ni.SubnetMask,
			"DefaultGateway":  ni.DefaultGateway,
			"DnsServers":      dns,
			"PhysicalAddress": ni.MAC,
			"MTU":             ni.MTU,
		}
	}
	return cfg, nil
}

// SetStaticIP writes one interface's IP configuration over ISAPI. iface is the
// device's own interface id ("1"/"2"), matching a key from GetNetworkConfig.
func (h *hikCamera) SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	return h.client.SetStaticIP(ctx, iface, dhcpEnable, ip, mask, gateway, dns)
}

// GetWiFiConfig / SetWiFiConfig / ScanWiFi satisfy NetworkSettings but are not
// implemented for Hikvision in this milestone (static IP only).
func (h *hikCamera) GetWiFiConfig(ctx context.Context) (map[string]map[string]any, error) {
	return nil, errHikWiFiUnsupported
}

func (h *hikCamera) SetWiFiConfig(ctx context.Context, iface, ssid, password, encryption string) error {
	return errHikWiFiUnsupported
}

func (h *hikCamera) ScanWiFi(ctx context.Context) ([]dahua.WiFiAP, error) {
	return nil, errHikWiFiUnsupported
}

// Reboot restarts the device via ISAPI /System/reboot.
func (h *hikCamera) Reboot(ctx context.Context) error { return h.client.Reboot(ctx) }

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
		Name:        i.Name,
	}
}
