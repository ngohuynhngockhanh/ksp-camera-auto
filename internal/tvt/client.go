// Package tvt implements a pure-Go (CGO_ENABLED=0) dual-plane client for TVT
// Digital Technology NVMS-9000 / TD-33xx / TD-31xx NVRs and IP cameras,
// including Secuvision Australia (NVR-0804B1-4P, NVR-0808B2-8P, NVR-0816B2-16P)
// and Speco Technologies N9K OEM recorders.
//
// Architecture:
//  1. NVMS-9000 HTTP XML Plane (/reqLogin + /doLogin with MD5+SHA512 challenge,
//     Cookie: sessionId=<guid>, <token>...</token>): full web-admin control for
//     stream encoding, OSD, device list, storage, reboot, and recording logs.
//  2. ONVIF Profile S + Profile G SOAP Plane (/onvif/device_service,
//     /onvif/Media2, /onvif/Media1, /onvif/Recording, /onvif/SearchRecording,
//     /onvif/Replay) with WS-UsernameToken PasswordDigest: seamless fallback
//     when credentials belong to an ONVIF/RTSP media account (e.g. "rtsp").
//  3. Zero-CPU RTSP Replay Plane (:554 /chID=<ch>&date=<YYYY-MM-DD>&time=<HH:MM:SS>&timelen=<sec>&streamType=main&linkType=tcp):
//     parallel stream-copy MP4 export (-c:v copy -an -tag:v hvc1 -movflags +faststart).
package tvt

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/md5"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mediaexport"
)

const (
	DefaultHTTPPort = 80
	DefaultRTSPPort = 554
	SystemType      = "NVMS-9000"
	xmlHeader       = `<?xml version="1.0" encoding="utf-8" ?>`
)

var (
	ErrAuthFailed     = errors.New("tvt: authentication failed")
	ErrAccountLocked  = errors.New("tvt: user account temporarily locked")
	ErrSessionExpired = errors.New("tvt: session expired")
	ErrUnsupported    = errors.New("tvt: operation unsupported")
)

var chIDParamRe = regexp.MustCompile(`(?i)(?:^|[/?&])chid=(\d+)`)

// PreLoginInfo summarizes an unauthenticated NVMS-9000 /reqLogin probe.
type PreLoginInfo struct {
	SystemType  string
	ProtocolVer string
	Nonce       string
	SessionID   string
}

// DeviceInfo holds basic hardware identity reported by the NVR/camera.
type DeviceInfo struct {
	Name            string
	Manufacturer    string
	Model           string
	FirmwareVersion string
	SerialNumber    string
	HardwareID      string
}

// StreamInfo mirrors camera.StreamInfo in vendor-neutral fields.
type StreamInfo struct {
	Channel        int
	Stream         int // 0=main, 1=sub
	Width          int
	Height         int
	FPS            int
	Compression    string
	Profile        string
	AudioCodec     string
	AudioEnable    bool
	SmartCodec     bool
	GOP            int
	BitRate        int
	BitRateControl string
	Name           string
	OSDLines       []string
}

// Client manages an authenticated session (NVMS-9000 XML and/or ONVIF Profile S/G)
// against one TVT / Secuvision NVR or IP camera.
type Client struct {
	host       string
	port       int
	rtspPort   int
	username   string
	password   string
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration

	mu          sync.Mutex
	authed      bool
	xmlAuthed   bool
	onvifOnly   bool
	sessionID   string
	token       string
	sessionKey  string
	securityVer string
	nvrLoc      *time.Location
}

// New constructs a TVT Client. No network I/O is performed until the first method call.
func New(host string, port int, username, password string, timeout time.Duration) *Client {
	if port <= 0 || port == DefaultRTSPPort {
		port = DefaultHTTPPort
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		host:     strings.TrimSpace(host),
		port:     port,
		rtspPort: DefaultRTSPPort,
		username: strings.TrimSpace(username),
		password: password,
		baseURL:  "http://" + net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(port)),
		timeout:  timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// BuildRTSPLivePath builds the 1-based TVT RTSP live path (/chID=<N>&streamType=main|sub&linkType=tcp).
func BuildRTSPLivePath(channel1Based, stream int) string {
	if channel1Based <= 0 {
		channel1Based = 1
	}
	st := "main"
	if stream > 0 {
		st = "sub"
	}
	return fmt.Sprintf("/chID=%d&streamType=%s&linkType=tcp", channel1Based, st)
}

// BuildRTSPPlaybackURL builds the 1-based TVT RTSP playback URL for [start, end].
// TVT NVRs expect date=YYYY-MM-DD and time=HH:MM:SS in the NVR's local timezone
// and timelen slightly longer (+5s) than FFmpeg's -t duration so RTP packets
// keep flowing across the exact clip end boundary without stalling.
func BuildRTSPPlaybackURL(host string, rtspPort int, user, pass string, channel0Based int, start, end time.Time, nvrLoc *time.Location) string {
	ch1 := channel0Based + 1
	if ch1 <= 0 {
		ch1 = 1
	}
	if rtspPort <= 0 {
		rtspPort = DefaultRTSPPort
	}
	if nvrLoc == nil {
		nvrLoc = time.Local
	}
	stLocal := start.In(nvrLoc)
	durSec := int(end.Sub(start).Seconds() + 0.5)
	if durSec < 1 {
		durSec = 1
	}
	timeLen := durSec + 5

	u := &url.URL{
		Scheme: "rtsp",
		Host:   net.JoinHostPort(host, strconv.Itoa(rtspPort)),
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	path := fmt.Sprintf("/chID=%d&date=%s&time=%s&timelen=%d&streamType=main&linkType=tcp",
		ch1,
		stLocal.Format("2006-01-02"),
		stLocal.Format("15:04:05"),
		timeLen,
	)
	return u.String() + path
}

// ParseChannelFromRTSP extracts the 1-based chID=<N> parameter from a TVT RTSP URL or path.
func ParseChannelFromRTSP(rawURL, path string) (int, bool) {
	combined := rawURL + " " + path
	if m := chIDParamRe.FindStringSubmatch(combined); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

// IsTVTRTSPPath reports whether an RTSP URL/path uses TVT's /chID=<N>&streamType=... convention.
func IsTVTRTSPPath(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "chid=") && (strings.Contains(lower, "streamtype=") || strings.Contains(lower, "linktype="))
}

// ComputePasswordDigest computes NVMS-9000's login hash:
//
//	md5Hex = hex(MD5(password))
//	digest = hex(SHA512(md5Hex + "#" + nonce))
func ComputePasswordDigest(password, nonce string) (sha512Hex, md5Hex string) {
	m := md5.Sum([]byte(password))
	md5Hex = hex.EncodeToString(m[:])
	s := sha512.Sum512([]byte(md5Hex + "#" + nonce))
	sha512Hex = hex.EncodeToString(s[:])
	return sha512Hex, md5Hex
}

// DecryptECBZeroPad decrypts Base64-encoded AES-ECB ZeroPadding ciphertext (used for NVMS-9000 sessionKey).
func DecryptECBZeroPad(b64Cipher, keyStr string) string {
	if b64Cipher == "" || keyStr == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Cipher))
	if err != nil || len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		return ""
	}
	keyBytes := []byte(keyStr)
	if len(keyBytes) != 16 && len(keyBytes) != 24 && len(keyBytes) != 32 {
		return ""
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return ""
	}
	out := make([]byte, len(raw))
	for off := 0; off < len(raw); off += aes.BlockSize {
		block.Decrypt(out[off:off+aes.BlockSize], raw[off:off+aes.BlockSize])
	}
	return string(bytes.TrimRight(out, "\x00"))
}

// ProbePreLogin checks whether baseURL is a TVT NVMS-9000 HTTP endpoint via POST /reqLogin.
func ProbePreLogin(ctx context.Context, baseURL string, timeout time.Duration) (*PreLoginInfo, error) {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	cl := &http.Client{Timeout: timeout}
	reqBody := xmlHeader + `<request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>null</token></request>`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/reqLogin", strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tvt reqLogin HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		XMLName    xml.Name `xml:"response"`
		Version    string   `xml:"version,attr"`
		SystemType string   `xml:"systemType,attr"`
		Status     string   `xml:"status"`
		Content    struct {
			Nonce     string `xml:"nonce"`
			SessionID string `xml:"sessionId"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	if !strings.EqualFold(parsed.Status, "success") || (parsed.Content.Nonce == "" && parsed.Content.SessionID == "") {
		return nil, fmt.Errorf("not a TVT NVMS-9000 reqLogin response")
	}
	return &PreLoginInfo{
		SystemType:  parsed.SystemType,
		ProtocolVer: parsed.Version,
		Nonce:       parsed.Content.Nonce,
		SessionID:   strings.Trim(parsed.Content.SessionID, "{}"),
	}, nil
}

// ensureAuth authenticates via NVMS-9000 XML (/reqLogin + /doLogin) and transparently
// falls back to ONVIF Profile S/G WS-UsernameToken when the user is an ONVIF/RTSP media user.
func (c *Client) ensureAuth(ctx context.Context) error {
	c.mu.Lock()
	if c.authed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	err := c.loginXML(ctx)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrAccountLocked) {
		return err
	}

	// Probe ONVIF Profile S GetDeviceInformation with WS-UsernameToken PasswordDigest.
	if _, onvifErr := c.getDeviceInfoONVIF(ctx); onvifErr == nil {
		c.mu.Lock()
		c.authed = true
		c.xmlAuthed = false
		c.onvifOnly = true
		c.mu.Unlock()
		return nil
	}
	return err
}

func (c *Client) loginXML(ctx context.Context) error {
	req1Body := xmlHeader + `<request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>null</token></request>`
	raw1, err := c.rawXMLPost(ctx, "reqLogin", req1Body, "")
	if err != nil {
		return fmt.Errorf("tvt reqLogin %s: %w", c.baseURL, err)
	}
	var r1 struct {
		Status  string `xml:"status"`
		Content struct {
			Nonce     string `xml:"nonce"`
			SessionID string `xml:"sessionId"`
			Token     string `xml:"token"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(raw1, &r1); err != nil {
		return fmt.Errorf("tvt reqLogin parse: %w", err)
	}
	if !strings.EqualFold(r1.Status, "success") {
		return fmt.Errorf("tvt reqLogin status=%q", r1.Status)
	}
	sid := strings.Trim(strings.TrimSpace(r1.Content.SessionID), "{}")
	tok := strings.TrimSpace(r1.Content.Token)
	nonce := strings.TrimSpace(r1.Content.Nonce)

	shaPw, md5Pw := ComputePasswordDigest(c.password, nonce)
	req2Body := fmt.Sprintf(`%s<request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>%s</token><content><userName><![CDATA[%s]]></userName><password><![CDATA[%s]]></password></content></request>`,
		xmlHeader, xmlEscapeText(tok), c.username, shaPw)

	raw2, err := c.rawXMLPost(ctx, "doLogin", req2Body, sid)
	if err != nil {
		return fmt.Errorf("tvt doLogin %s: %w", c.baseURL, err)
	}
	var r2 struct {
		Status    string `xml:"status"`
		ErrorCode string `xml:"errorCode"`
		Content   struct {
			SessionKey  string `xml:"sessionKey"`
			SecurityVer string `xml:"securityVer"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(raw2, &r2); err != nil {
		return fmt.Errorf("tvt doLogin parse: %w", err)
	}
	if !strings.EqualFold(r2.Status, "success") {
		switch strings.TrimSpace(r2.ErrorCode) {
		case "536870951":
			return fmt.Errorf("%w (code %s)", ErrAccountLocked, r2.ErrorCode)
		case "536870947", "536870948":
			return fmt.Errorf("%w (code %s)", ErrAuthFailed, r2.ErrorCode)
		default:
			return fmt.Errorf("%w: status=%s errorCode=%s", ErrAuthFailed, r2.Status, r2.ErrorCode)
		}
	}

	c.mu.Lock()
	c.authed = true
	c.xmlAuthed = true
	c.onvifOnly = false
	c.sessionID = sid
	c.token = tok
	c.sessionKey = DecryptECBZeroPad(r2.Content.SessionKey, md5Pw)
	c.securityVer = strings.TrimSpace(r2.Content.SecurityVer)
	c.mu.Unlock()
	return nil
}

// Close releases any active NVMS-9000 web session on the NVR.
func (c *Client) Close() error {
	c.mu.Lock()
	xmlAuthed := c.xmlAuthed
	sid := c.sessionID
	tok := c.token
	c.authed = false
	c.xmlAuthed = false
	c.sessionID = ""
	c.token = ""
	c.mu.Unlock()

	if !xmlAuthed || sid == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	body := fmt.Sprintf(`%s<request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>%s</token></request>`, xmlHeader, xmlEscapeText(tok))
	_, _ = c.rawXMLPost(ctx, "doLogout", body, sid)
	return nil
}

func (c *Client) rawXMLPost(ctx context.Context, action, xmlBody, sessionID string) ([]byte, error) {
	u := fmt.Sprintf("%s/%s", c.baseURL, strings.TrimLeft(action, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(xmlBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	if sessionID != "" {
		req.Header.Set("Cookie", "sessionId="+sessionID)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// CallXML executes an authenticated NVMS-9000 XML action, re-authenticating once on 536871011 (session timeout).
func (c *Client) CallXML(ctx context.Context, action, innerXML string) ([]byte, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	if !c.xmlAuthed {
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: account %q has ONVIF/RTSP access only", ErrUnsupported, c.username)
	}
	sid := c.sessionID
	tok := c.token
	c.mu.Unlock()

	buildBody := func(t string) string {
		return fmt.Sprintf(`%s<request version="1.0" systemType="NVMS-9000" clientType="WEB"><token>%s</token>%s</request>`,
			xmlHeader, xmlEscapeText(t), innerXML)
	}
	respBytes, err := c.rawXMLPost(ctx, action, buildBody(tok), sid)
	if err != nil {
		return nil, err
	}
	var env struct {
		Status    string `xml:"status"`
		ErrorCode string `xml:"errorCode"`
	}
	if xml.Unmarshal(respBytes, &env) == nil && strings.TrimSpace(env.ErrorCode) == "536871011" {
		c.mu.Lock()
		c.authed = false
		c.xmlAuthed = false
		c.mu.Unlock()
		if err := c.loginXML(ctx); err != nil {
			return nil, err
		}
		c.mu.Lock()
		sid = c.sessionID
		tok = c.token
		c.mu.Unlock()
		return c.rawXMLPost(ctx, action, buildBody(tok), sid)
	}
	return respBytes, nil
}

// GetSerialNumber returns the NVR/camera hardware serial number.
func (c *Client) GetSerialNumber(ctx context.Context) (string, error) {
	info, err := c.GetDeviceInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.SerialNumber, nil
}

// GetDeviceInfo returns basic hardware identity via NVMS-9000 XML or ONVIF Profile S.
func (c *Client) GetDeviceInfo(ctx context.Context) (DeviceInfo, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return DeviceInfo{}, err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if info, err := c.getBasicCfgXML(ctx); err == nil && (info.SerialNumber != "" || info.Model != "") {
			return info, nil
		}
	}
	return c.getDeviceInfoONVIF(ctx)
}

// ProbeAll enumerates all active video streams on the device.
func (c *Client) ProbeAll(ctx context.Context) ([]StreamInfo, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if list, err := c.probeAllXML(ctx); err == nil && len(list) > 0 {
			return list, nil
		}
	}
	return c.probeStreamsONVIF(ctx)
}

// GetStreamInfo reads one (0-based channel, stream) configuration.
func (c *Client) GetStreamInfo(ctx context.Context, channel, stream int) (StreamInfo, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return StreamInfo{}, err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if info, err := c.getStreamInfoXML(ctx, channel, stream); err == nil {
			return info, nil
		}
	}
	return c.getStreamInfoONVIF(ctx, channel, stream)
}

// GetFPSCapability reports current and max framerate for a channel/stream.
func (c *Client) GetFPSCapability(ctx context.Context, channel, stream, _, _ int, _ string) (curFPS, maxFPS int, source string, err error) {
	info, err := c.GetStreamInfo(ctx, channel, stream)
	if err != nil {
		return 0, 30, "default", err
	}
	max := 30
	if info.FPS > max {
		max = info.FPS
	}
	return info.FPS, max, "tvt-profile", nil
}

// Snapshot captures a JPEG frame for (0-based channel, stream) via NVMS-9000 XML,
// ONVIF SnapshotUri, or 1-frame RTSP fallback.
func (c *Client) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	_ = c.ensureAuth(ctx)
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if jpg, err := c.snapshotXML(ctx, channel); err == nil && len(jpg) > 200 {
			return jpg, nil
		}
	}
	if jpg, err := c.snapshotONVIF(ctx, channel, stream); err == nil && len(jpg) > 200 {
		return jpg, nil
	}
	return c.snapshotRTSP(ctx, channel, stream)
}

func (c *Client) snapshotRTSP(ctx context.Context, channel, stream int) ([]byte, error) {
	ffmpegBin, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("tvt snapshot: ffmpeg not found: %w", err)
	}
	u := &url.URL{
		Scheme: "rtsp",
		Host:   net.JoinHostPort(c.host, strconv.Itoa(c.rtspPort)),
	}
	if c.username != "" {
		u.User = url.UserPassword(c.username, c.password)
	}
	rtspURL := u.String() + BuildRTSPLivePath(channel+1, stream)
	sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(sctx, ffmpegBin,
		"-nostdin", "-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-frames:v", "1",
		"-f", "image2",
		"-c:v", "mjpeg",
		"-q:v", "4",
		"pipe:1",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("tvt RTSP snapshot ch=%d: %w", channel+1, err)
	}
	if len(out) < 200 || out[0] != 0xff || out[1] != 0xd8 {
		return nil, fmt.Errorf("tvt RTSP snapshot ch=%d returned invalid JPEG", channel+1)
	}
	return out, nil
}

// GetRemoteDevices enumerates NVR channels via NVMS-9000 /queryDevList or ONVIF Profile G/S.
func (c *Client) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if list, err := c.getDevListXML(ctx); err == nil && len(list) > 0 {
			return list, nil
		}
	}
	return c.getRemoteDevicesONVIF(ctx)
}

// FindRecordings queries recorded intervals on 0-based channel over [start, end].
func (c *Client) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	if !end.After(start) {
		return nil, nil
	}
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}
	// ONVIF Profile G SearchRecording works identically for both admin and ONVIF/RTSP
	// accounts and provides UTC-exact boundaries; try it first, with XML fallback.
	if list, err := c.findRecordingsONVIF(ctx, channel, start, end); err == nil {
		return list, nil
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		return c.findRecordingsXML(ctx, channel, start, end)
	}
	return nil, fmt.Errorf("tvt FindRecordings ch=%d failed", channel+1)
}

// StreamPlaybackFast exports [start, end] on 0-based channel as a +faststart MP4
// using strictly zero-CPU stream copy (-c:v copy -an -tag:v hvc1 -movflags +faststart).
func (c *Client) StreamPlaybackFast(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	loc := c.GetNVRTimezoneLocation(ctx)
	durSec := end.Sub(start).Seconds()
	parallel := 2
	chunkSec := 60
	switch {
	case durSec <= 65:
		parallel = 1
		chunkSec = int(durSec) + 1
	case durSec <= 185:
		parallel = 2
		chunkSec = 60
	default:
		parallel = 3
		chunkSec = 75
	}
	return mediaexport.FastMP4Range(ctx, w, start, end, chunkSec, func(cs, ce time.Time) string {
		return BuildRTSPPlaybackURL(c.host, c.rtspPort, c.username, c.password, channel, cs, ce, loc)
	}, true, parallel)
}

// StreamNative exports [start, end] on 0-based channel as MKV with both video and
// G.711 audio stream-copied untouched (0% CPU).
func (c *Client) StreamNative(ctx context.Context, w io.Writer, channel int, start, end time.Time) error {
	loc := c.GetNVRTimezoneLocation(ctx)
	durSec := end.Sub(start).Seconds()
	parallel := 2
	chunkSec := 60
	if durSec <= 65 {
		parallel = 1
		chunkSec = int(durSec) + 1
	}
	return mediaexport.FastNativeRange(ctx, w, start, end, chunkSec, func(cs, ce time.Time) string {
		return BuildRTSPPlaybackURL(c.host, c.rtspPort, c.username, c.password, channel, cs, ce, loc)
	}, parallel)
}

// Reboot triggers a remote reboot via NVMS-9000 /reboot or ONVIF SystemReboot.
func (c *Client) Reboot(ctx context.Context) error {
	if err := c.ensureAuth(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if xmlOK {
		if err := c.rebootXML(ctx); err == nil {
			return nil
		}
	}
	return c.rebootONVIF(ctx)
}

func xmlEscapeText(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
