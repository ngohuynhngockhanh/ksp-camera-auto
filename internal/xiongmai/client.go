package xiongmai

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client is the high-level object-oriented facade over a Xiongmai IPC, DVR,
// HVR, or NVR. It owns a Transport strategy (auto-negotiated between Sofia TCP
// :34567 and HTTP CGI :80 unless pinned via NewWithTransport) and caches
// channel-topology metadata from SystemInfo so analog vs digital (bypass@)
// channels are routed transparently.
type Client struct {
	host     string
	port     int
	username string
	password string
	timeout  time.Duration

	mu        sync.Mutex
	transport Transport
	pinned    bool
	sysInfo   *SystemInfoData
}

// New creates a Client with automatic dual-transport negotiation:
//   - If port == 80 (or custom HTTP port != 34567), tries CGITransport first;
//     on dial/HTTP 404/timeout (typical on standalone Sofia IPCs), falls back
//     to SofiaTCPTransport on :34567.
//   - If port == 0 or 34567, tries SofiaTCPTransport on :34567 first; if the
//     device responds Ret=205 (ERR_RSA_REQUIRED, typical on newer RSA-hardened
//     HVRs/NVRs) or TCP 34567 is unreachable, falls back to CGITransport on :80.
func New(host string, port int, username, password string, timeout time.Duration) *Client {
	if port <= 0 {
		port = DefaultSofiaPort
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		host:     strings.TrimSpace(host),
		port:     port,
		username: username,
		password: password,
		timeout:  timeout,
	}
}

// NewWithTransport injects a custom Transport implementation (used in unit
// tests or when pinning either CGITransport or SofiaTCPTransport explicitly).
func NewWithTransport(host string, port int, username, password string, timeout time.Duration, tr Transport) *Client {
	c := New(host, port, username, password, timeout)
	c.transport = tr
	c.pinned = tr != nil
	return c
}

// Host returns the device IP/hostname.
func (c *Client) Host() string { return c.host }

// Port returns the configured control port.
func (c *Client) Port() int { return c.port }

// ActiveTransportName returns "sofia-tcp", "http-cgi", or "" before connect.
func (c *Client) ActiveTransportName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transport == nil {
		return ""
	}
	return c.transport.Name()
}

// Transport resolves (and authenticates on first use) the active Transport.
func (c *Client) Transport(ctx context.Context) (Transport, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transport != nil {
		if c.transport.LoginInfo() == nil {
			if _, err := c.transport.Login(ctx); err != nil {
				return nil, err
			}
		}
		return c.transport, nil
	}

	var primary, secondary Transport
	if c.port != DefaultSofiaPort && c.port > 0 {
		primary = NewCGITransport(c.host, c.port, c.username, c.password, c.timeout)
		secondary = NewSofiaTCPTransport(c.host, DefaultSofiaPort, c.username, c.password, c.timeout)
	} else {
		primary = NewSofiaTCPTransport(c.host, c.port, c.username, c.password, c.timeout)
		secondary = NewCGITransport(c.host, 80, c.username, c.password, c.timeout)
	}

	if _, err := primary.Login(ctx); err == nil {
		c.transport = primary
		return primary, nil
	} else {
		_ = primary.Close()
		// Never cross-probe secondary transport if credentials were explicitly
		// rejected (avoids burning 2 failed login attempts per call and risking Ret=206 lockout).
		if errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrAccountLocked) {
			return nil, err
		}
		if secondary != nil && (errors.Is(err, ErrRSAHandshakeRequired) || errors.Is(err, ErrDialUnreachable)) {
			if _, err2 := secondary.Login(ctx); err2 == nil {
				c.transport = secondary
				return secondary, nil
			} else {
				_ = secondary.Close()
				if errors.Is(err2, ErrAuthFailed) || errors.Is(err2, ErrAccountLocked) {
					return nil, err2
				}
			}
		}
		return nil, err
	}
}

// Close logs out and closes the active transport session.
func (c *Client) Close() error {
	c.mu.Lock()
	tr := c.transport
	if !c.pinned {
		c.transport = nil
	}
	c.mu.Unlock()
	if tr != nil {
		return tr.Close()
	}
	return nil
}

// BuildRTSPURL constructs the canonical Xiongmai/XMEye/Vitacam RTSP stream URL:
//
//	rtsp://<user>:<pass>@<host>:554/user=<user>_password=<pass>_channel=<1..N>_stream=<0|1>.sdp?real_stream
//
// channel1Based is 1-based (1..N); stream is 0 (main) or 1 (sub).
func BuildRTSPURL(host, username, password string, channel1Based, stream int) string {
	if channel1Based <= 0 {
		channel1Based = 1
	}
	if stream < 0 {
		stream = 0
	}
	path := BuildRTSPPath(username, password, channel1Based, stream)
	if username != "" && password != "" {
		return fmt.Sprintf("rtsp://%s:%s@%s:554%s",
			url.QueryEscape(username), url.QueryEscape(password), host, path)
	}
	if username != "" {
		return fmt.Sprintf("rtsp://%s@%s:554%s", url.QueryEscape(username), host, path)
	}
	return fmt.Sprintf("rtsp://%s:554%s", host, path)
}

// BuildRTSPPath returns the path+query portion of Xiongmai's RTSP URL.
func BuildRTSPPath(username, password string, channel1Based, stream int) string {
	if channel1Based <= 0 {
		channel1Based = 1
	}
	if stream < 0 {
		stream = 0
	}
	u := username
	if u == "" {
		u = "admin"
	}
	return fmt.Sprintf("/user=%s_password=%s_channel=%d_stream=%d.sdp?real_stream",
		url.QueryEscape(u), url.QueryEscape(password), channel1Based, stream)
}

// CallRaw executes a raw Xiongmai command for advanced diagnostics / MCP tools.
// Supported path forms:
//   - "info:<Name>"           -> GetInfo(Name)
//   - "ability:<Name>"        -> GetAbility(Name)
//   - "chability:<Name>"      -> GetChannelAbility(Name)
//   - "get:<ConfigName>"      -> GetConfig(ConfigName)
//   - "set:<ConfigName>"      -> SetConfig(ConfigName, reqData)
//   - "op:<OpName>"           -> OpDev(OpName, reqData)
//   - bare "<ConfigName>"     -> GetConfig(ConfigName) when reqData == nil, else SetConfig
func (c *Client) CallRaw(ctx context.Context, apiPath string, reqData any) (map[string]any, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	p := strings.TrimSpace(apiPath)
	switch {
	case strings.HasPrefix(p, "info:"):
		return tr.GetInfo(ctx, strings.TrimPrefix(p, "info:"))
	case strings.HasPrefix(p, "ability:"):
		return tr.GetAbility(ctx, strings.TrimPrefix(p, "ability:"))
	case strings.HasPrefix(p, "chability:"):
		return tr.GetChannelAbility(ctx, strings.TrimPrefix(p, "chability:"))
	case strings.HasPrefix(p, "get:"):
		return tr.GetConfig(ctx, strings.TrimPrefix(p, "get:"))
	case strings.HasPrefix(p, "set:"):
		return tr.SetConfig(ctx, strings.TrimPrefix(p, "set:"), reqData)
	case strings.HasPrefix(p, "op:"):
		return tr.OpDev(ctx, strings.TrimPrefix(p, "op:"), reqData)
	default:
		if reqData == nil {
			return tr.GetConfig(ctx, p)
		}
		return tr.SetConfig(ctx, p, reqData)
	}
}
