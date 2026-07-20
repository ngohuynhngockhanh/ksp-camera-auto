package tiandy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// configPort is the Tiandy web/config HTTP port. Tiandy serves a
// Hikvision-compatible ISAPI surface here (/ISAPI/...) plus its own CGI login
// (/CGI/Security/...). Media/RTSP stays on 554 (see playback.go).
const configPort = 8081

// NewISAPIClient builds an isapi.Client whose transport speaks Tiandy's ISAPI
// over its CGI session auth. Because Tiandy's /ISAPI schemas match Hikvision's,
// wrapping this in hik.NewWithClient yields a hik.Client with the full config
// surface (network, encode, password, OSD, storage, remote devices, reboot) —
// the whole point of the seam: one auth shim, and internal/hik + internal/isapi
// are reused verbatim.
func NewISAPIClient(host, user, pass string, timeout time.Duration) *isapi.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return isapi.NewWithTransport(&sessionTransport{
		baseURL: fmt.Sprintf("http://%s:%d", host, configPort),
		user:    user,
		pass:    pass,
		http:    &http.Client{Timeout: timeout},
	})
}

// sessionTransport implements isapi.Transport for Tiandy. Tiandy authenticates
// with a CGI challenge/response (iterated SHA-256 → HttpSession header) rather
// than HTTP Digest, so this shim logs in, attaches HttpSession to every ISAPI
// call, and re-logins once on session expiry. It also papers over the one place
// Tiandy's ISAPI differs from Hik's: the network-interface *collection* GET is
// unsupported (only per-interface /N/ipAddress works), so that path is
// synthesized into the list shape hik expects.
type sessionTransport struct {
	baseURL    string
	user, pass string
	http       *http.Client

	mu      sync.Mutex
	session string
}

const netIfacesPath = "/ISAPI/System/Network/interfaces"

// streamChannelRe matches hik's encode path /ISAPI/Streaming/channels/{trackID}
// (trackID = ch*100 + streamType). Tiandy serves the identical StreamingChannel
// document at /CGI/Streaming/channels/{ch}/type/{streamType} instead.
var streamChannelRe = regexp.MustCompile(`^/ISAPI/Streaming/channels/(\d+)$`)

// remapPath rewrites the hik ISAPI paths whose Tiandy equivalent lives under
// /CGI with a different shape but the SAME XML body, so hik's get/set logic
// reuses unchanged. Currently: the per-stream encode document. hik builds the
// stream id as ch0*100 + stream0 + 1 (ch0/stream0 both 0-based, so channel 0's
// three streams are ids 1,2,3 and channel 1's are 101,102,103); Tiandy wants
// /CGI/Streaming/channels/{ch1}/type/{type1} with 1-based channel and type
// (type 1=main,2=sub,3=third) — verified live.
func remapPath(path string) string {
	if m := streamChannelRe.FindStringSubmatch(path); m != nil {
		id, _ := strconv.Atoi(m[1])
		if id >= 1 {
			ch1 := (id-1)/100 + 1
			typ1 := (id-1)%100 + 1
			return fmt.Sprintf("/CGI/Streaming/channels/%d/type/%d", ch1, typ1)
		}
	}
	return path
}

// Do implements isapi.Transport.
func (t *sessionTransport) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	// Tiandy doesn't serve the interface collection (returns notSupport); hik's
	// GetNetworkInterfaces reads it. Synthesize the list from per-interface docs.
	if method == http.MethodGet && path == netIfacesPath {
		return t.synthNetworkList(ctx)
	}
	path = remapPath(path)
	data, status, err := t.doOnce(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		t.mu.Lock()
		t.session = ""
		t.mu.Unlock()
		if data, status, err = t.doOnce(ctx, method, path, body); err != nil {
			return nil, err
		}
	}
	if status >= 300 {
		return data, fmt.Errorf("tiandy isapi: %s %s: HTTP %d: %s", method, path, status, truncate(data))
	}
	return data, nil
}

// doOnce ensures a session then issues one ISAPI request with the HttpSession
// header, returning body + HTTP status.
func (t *sessionTransport) doOnce(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	session, err := t.ensureSession(ctx)
	if err != nil {
		return nil, 0, err
	}
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, r)
	if err != nil {
		return nil, 0, fmt.Errorf("tiandy isapi: build %s %s: %w", method, path, err)
	}
	req.Header.Set("HttpSession", session)
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("tiandy isapi: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("tiandy isapi: read %s %s: %w", method, path, err)
	}
	return data, resp.StatusCode, nil
}

// ensureSession returns a cached session, logging in if none is held.
func (t *sessionTransport) ensureSession(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session != "" {
		return t.session, nil
	}
	s, err := t.login(ctx)
	if err != nil {
		return "", err
	}
	t.session = s
	return s, nil
}

// login runs the CGI challenge/response and returns the authenticated session.
func (t *sessionTransport) login(ctx context.Context) (string, error) {
	ch, err := t.cgi(ctx, http.MethodGet, "/CGI/Security/SessionCheck?timeStamp="+ts(), nil)
	if err != nil {
		return "", err
	}
	session, key, iters := xmlField(ch, "session"), xmlField(ch, "key"), xmlField(ch, "iterations")
	n, _ := strconv.Atoi(iters)
	if session == "" || key == "" || n <= 0 {
		return "", fmt.Errorf("tiandy: bad SessionCheck challenge: %s", truncate(ch))
	}
	digest := sessionDigest(t.user, t.pass, key, n)
	body := fmt.Sprintf("<User><username>%s</username><passwd>%s</passwd><sessionTmp>%s</sessionTmp></User>",
		xmlEscape(t.user), digest, session)
	resp, err := t.cgi(ctx, http.MethodPost, "/CGI/Security/Logon?timeStamp="+ts(), []byte(body))
	if err != nil {
		return "", err
	}
	if s := xmlField(resp, "statusString"); s != "" && !strings.EqualFold(s, "OK") {
		return "", fmt.Errorf("tiandy: logon rejected: %s", truncate(resp))
	}
	if ns := xmlField(resp, "session"); ns != "" {
		return ns, nil
	}
	return session, nil
}

// cgi issues a raw request WITHOUT the HttpSession header (used for the login
// handshake itself).
func (t *sessionTransport) cgi(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tiandy: cgi %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// synthNetworkList builds the <NetworkInterfaceList> hik expects out of the
// per-interface /ISAPI/System/Network/interfaces/N/ipAddress documents Tiandy
// does serve. It probes interfaces 1..2 and includes each that returns a
// NetworkInterface body.
func (t *sessionTransport) synthNetworkList(ctx context.Context) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><NetworkInterfaceList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">`)
	found := 0
	for id := 1; id <= 2; id++ {
		data, status, err := t.doOnce(ctx, http.MethodGet, fmt.Sprintf("%s/%d/ipAddress", netIfacesPath, id), nil)
		if err != nil || status >= 300 {
			continue
		}
		inner := extractElement(data, "NetworkInterface")
		if inner == "" {
			continue
		}
		b.WriteString(inner)
		found++
	}
	b.WriteString(`</NetworkInterfaceList>`)
	if found == 0 {
		return nil, fmt.Errorf("tiandy: no network interfaces readable")
	}
	return []byte(b.String()), nil
}

// sessionDigest = UPPER(sha256(user+pass)) then iterations× UPPER(sha256(prev+key)).
func sessionDigest(user, pass, key string, iterations int) string {
	up := func(sum [32]byte) string { return strings.ToUpper(hex.EncodeToString(sum[:])) }
	a := up(sha256.Sum256([]byte(user + pass)))
	for i := 0; i < iterations; i++ {
		a = up(sha256.Sum256([]byte(a + key)))
	}
	return a
}

func ts() string { return strconv.FormatInt(time.Now().UnixMilli(), 10) }

// xmlField returns the text of the first <tag>...</tag> in b (no namespace).
func xmlField(b []byte, tag string) string {
	open, close := "<"+tag+">", "</"+tag+">"
	s := string(b)
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	i += len(open)
	j := strings.Index(s[i:], close)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(s[i : i+j])
}

// extractElement returns the whole <tag ...>...</tag> substring (inclusive),
// used to lift a <NetworkInterface> out of one response into a synthesized list.
func extractElement(b []byte, tag string) string {
	s := string(b)
	start := strings.Index(s, "<"+tag)
	if start < 0 {
		return ""
	}
	end := strings.Index(s[start:], "</"+tag+">")
	if end < 0 {
		return ""
	}
	return s[start : start+end+len("</"+tag+">")]
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}
