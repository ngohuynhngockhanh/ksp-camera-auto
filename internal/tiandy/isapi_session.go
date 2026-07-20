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

// Tiandy serves the SAME XML documents hik expects, but under /CGI paths (its
// /ISAPI equivalents return statusCode 4 / notSupport). remapPath rewrites each
// so hik's get/set logic reuses unchanged. All verified live on a TC-R3440.
var (
	// encode: hik id = ch0*100 + stream0 + 1 (ch0 streams -> 1,2,3; ch1 -> 101..).
	streamChannelRe = regexp.MustCompile(`^/ISAPI/Streaming/channels/(\d+)$`)
	// OSD overlay doc (VideoOverlay/TextOverlayList) — Tiandy wants /type/1.
	overlaysRe = regexp.MustCompile(`^/ISAPI/System/Video/inputs/channels/(\d+)/overlays$`)
	// InputProxy per-channel (logical channel name). Collection (no id) stays /ISAPI.
	inputProxyChRe = regexp.MustCompile(`^/ISAPI/ContentMgmt/InputProxy/channels/(\d+)$`)
)

func remapPath(path string) string {
	if m := streamChannelRe.FindStringSubmatch(path); m != nil {
		if id, _ := strconv.Atoi(m[1]); id >= 1 {
			return fmt.Sprintf("/CGI/Streaming/channels/%d/type/%d", (id-1)/100+1, (id-1)%100+1)
		}
	}
	if m := overlaysRe.FindStringSubmatch(path); m != nil {
		return "/CGI/System/Video/inputs/channels/" + m[1] + "/overlays/type/1"
	}
	if m := inputProxyChRe.FindStringSubmatch(path); m != nil {
		return "/CGI/ContentMgmt/InputProxy/channels/" + m[1]
	}
	if path == "/ISAPI/ContentMgmt/Storage" {
		return "/CGI/ContentMgmt/Storage/hdd/" // trailing slash required by Tiandy
	}
	return path
}

// optMaxAttrRe strips the schema-hint attributes Tiandy emits on GET
// (opt="H.264,..." / max="16384"). It rejects its OWN document on PUT unless
// these are removed (statusCode 254 / "Parameter Error") — verified live.
var optMaxAttrRe = regexp.MustCompile(` (?:opt|max)="[^"]*"`)

func stripOptMax(b []byte) []byte { return optMaxAttrRe.ReplaceAll(b, nil) }

// renameElem rewrites <from>…</from> to <to>…</to> (open+close tags only), used
// to bridge hik's <name> vs Tiandy's <channelName> for the InputProxy channel.
func renameElem(b []byte, from, to string) []byte {
	s := string(b)
	s = strings.ReplaceAll(s, "<"+from+">", "<"+to+">")
	s = strings.ReplaceAll(s, "</"+from+">", "</"+to+">")
	return []byte(s)
}

// mergeStreamBody GETs the device's current full StreamingChannel doc and
// overlays the encode fields hik changed (hik PUTs only the fields it models;
// Tiandy needs the whole document), then strips opt/max attributes. Returns the
// original body on any read failure.
func (t *sessionTransport) mergeStreamBody(ctx context.Context, path string, changes []byte) []byte {
	full, status, err := t.doOnce(ctx, http.MethodGet, path, nil)
	if err != nil || status >= 300 || len(full) == 0 {
		return stripOptMax(changes)
	}
	s := string(full)
	for _, name := range encodeMergeFields {
		if v := xmlField(changes, name); v != "" {
			re := regexp.MustCompile(`(<` + name + `\b[^>]*>)[^<]*(</` + name + `>)`)
			s = re.ReplaceAllString(s, `${1}`+v+`${2}`)
		}
	}
	return stripOptMax([]byte(s))
}

// encodeMergeFields: unambiguous leaf elements an encode change touches, merged
// by value into the device's current full doc. "enabled"/audio excluded
// (ambiguous; encode edits don't change them).
var encodeMergeFields = []string{
	"videoCodecType", "videoResolutionWidth", "videoResolutionHeight",
	"maxFrameRate", "videoQualityControlType", "GovLength",
	"constantBitRate", "vbrUpperCap", "vbrAverageCap",
}

// Do implements isapi.Transport.
func (t *sessionTransport) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	// Tiandy doesn't serve the interface collection (returns notSupport); hik's
	// GetNetworkInterfaces reads it. Synthesize the list from per-interface docs.
	if method == http.MethodGet && path == netIfacesPath {
		return t.synthNetworkList(ctx)
	}
	path = remapPath(path)
	// Request-body transforms: Tiandy rejects the schema-hint attributes and
	// (for encode) a partial document, so fix up PUT bodies before sending.
	if method == http.MethodPut {
		switch {
		case strings.HasPrefix(path, "/CGI/Streaming/channels/"):
			body = t.mergeStreamBody(ctx, path, body)
		case strings.Contains(path, "/overlays/type/"):
			body = stripOptMax(body)
		case strings.HasPrefix(path, "/CGI/ContentMgmt/InputProxy/channels/"):
			// hik edits the channel name in a <name> element; Tiandy's InputProxy
			// doc calls it <channelName>. Rename back before PUT.
			body = renameElem(body, "name", "channelName")
		}
	}
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
	// Response transform: Tiandy's HDD list comes back as a bare <hddList>; hik's
	// GetStorage parses a <storage><hddList> document, so wrap it.
	if method == http.MethodGet && path == "/CGI/ContentMgmt/Storage/hdd/" {
		if s := string(data); strings.Contains(s, "<hddList>") && !strings.Contains(s, "<storage>") {
			i := strings.Index(s, "<hddList>")
			data = []byte(`<?xml version="1.0" encoding="UTF-8"?><storage>` + s[i:] + `</storage>`)
		}
	}
	// hik's GetChannelName reads a <name> element; Tiandy's InputProxy doc uses
	// <channelName>. Rename on read so the channel name is found.
	if method == http.MethodGet && strings.HasPrefix(path, "/CGI/ContentMgmt/InputProxy/channels/") {
		data = renameElem(data, "channelName", "name")
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
