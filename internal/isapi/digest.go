// Package isapi implements the payload and HTTP transport layer for
// Hikvision's ISAPI protocol: an HTTP Digest authentication round-tripper
// plus a client for the Streaming/channels resources used to configure
// codec, resolution, Smart Codec and audio settings. It is pure Go stdlib —
// no third-party dependencies, CGO stays off.
package isapi

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Challenge holds the parsed fields of a "WWW-Authenticate: Digest ..."
// response header (RFC 2617).
type Challenge struct {
	Realm     string
	Nonce     string
	QOP       string // may list multiple values, e.g. "auth,auth-int"
	Opaque    string
	Algorithm string
}

// ParseChallenge parses the value of a WWW-Authenticate header. It expects
// the "Digest" scheme and requires at least realm and nonce to be present.
func ParseChallenge(header string) (Challenge, error) {
	var c Challenge
	header = strings.TrimSpace(header)
	const scheme = "Digest"
	if len(header) < len(scheme) || !strings.EqualFold(header[:len(scheme)], scheme) {
		return c, fmt.Errorf("isapi: unsupported WWW-Authenticate scheme: %q", header)
	}
	rest := strings.TrimSpace(header[len(scheme):])
	for _, part := range splitDigestParams(rest) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch strings.ToLower(key) {
		case "realm":
			c.Realm = val
		case "nonce":
			c.Nonce = val
		case "qop":
			c.QOP = val
		case "opaque":
			c.Opaque = val
		case "algorithm":
			c.Algorithm = val
		}
	}
	if c.Realm == "" || c.Nonce == "" {
		return c, fmt.Errorf("isapi: incomplete digest challenge: %q", header)
	}
	return c, nil
}

// splitDigestParams splits a comma-separated "k=v" list, treating commas
// inside double-quoted values as literal (so quoted realms/nonces containing
// commas are not split incorrectly).
func splitDigestParams(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuotes := false
	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes
			cur.WriteRune(r)
		case ',':
			if inQuotes {
				cur.WriteRune(r)
			} else {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// BuildAuthorization computes the RFC 2617 Digest Authorization header value
// for qop=auth using MD5 (the only algorithm Hikvision firmware negotiates).
// nc must already be formatted as the 8-hex-digit nonce count (e.g.
// "00000001") and cnonce a client-generated token.
func BuildAuthorization(c Challenge, method, uri, username, password, cnonce, nc string) string {
	ha1 := md5hex(fmt.Sprintf("%s:%s:%s", username, c.Realm, password))
	ha2 := md5hex(fmt.Sprintf("%s:%s", method, uri))
	const qop = "auth"
	response := md5hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, c.Nonce, nc, cnonce, qop, ha2))

	var b strings.Builder
	fmt.Fprintf(&b, `Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", qop=%s, nc=%s, cnonce="%s"`,
		username, c.Realm, c.Nonce, uri, response, qop, nc, cnonce)
	if c.Opaque != "" {
		fmt.Fprintf(&b, `, opaque="%s"`, c.Opaque)
	}
	return b.String()
}

// randomCnonce returns a fresh 16-hex-character client nonce.
func randomCnonce() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("isapi: generate cnonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// DigestTransport is an http.RoundTripper implementing RFC 2617 HTTP Digest
// authentication (qop=auth, MD5) — the scheme Hikvision devices negotiate by
// default. The standard library has no client-side Digest support, so this
// fills that gap: on a 401 challenge it parses WWW-Authenticate, computes the
// Authorization header, and retries once. It also caches the most recent
// challenge per host so subsequent requests can send a pre-emptive
// Authorization header (still with a freshly incremented nc/cnonce),
// avoiding a round trip through 401 for every call.
type DigestTransport struct {
	Username string
	Password string
	// Base is the underlying RoundTripper used to perform requests. If nil,
	// http.DefaultTransport is used.
	Base http.RoundTripper

	mu   sync.Mutex
	chal map[string]Challenge
	nc   map[string]int
}

// NewDigestTransport builds a DigestTransport for the given credentials. base
// may be nil to use http.DefaultTransport.
func NewDigestTransport(username, password string, base http.RoundTripper) *DigestTransport {
	return &DigestTransport{
		Username: username,
		Password: password,
		Base:     base,
		chal:     map[string]Challenge{},
		nc:       map[string]int{},
	}
}

// RoundTrip implements http.RoundTripper.
func (t *DigestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	host := req.URL.Host

	t.mu.Lock()
	chal, haveChal := t.chal[host]
	t.mu.Unlock()

	attempt, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	if haveChal {
		if err := t.authorize(attempt, chal, host); err != nil {
			return nil, err
		}
	}

	resp, err := base.RoundTrip(attempt)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Challenged (first contact, or our cached nonce expired) — parse the
	// fresh challenge and retry exactly once with it.
	wa := resp.Header.Get("WWW-Authenticate")
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if wa == "" {
		return resp, nil
	}
	newChal, perr := ParseChallenge(wa)
	if perr != nil {
		return resp, nil
	}
	t.mu.Lock()
	t.chal[host] = newChal
	t.nc[host] = 0
	t.mu.Unlock()

	retry, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	if err := t.authorize(retry, newChal, host); err != nil {
		return nil, err
	}
	return base.RoundTrip(retry)
}

// authorize increments the nonce counter for host and sets the Authorization
// header on req using challenge chal.
func (t *DigestTransport) authorize(req *http.Request, chal Challenge, host string) error {
	t.mu.Lock()
	t.nc[host]++
	nc := t.nc[host]
	t.mu.Unlock()

	cnonce, err := randomCnonce()
	if err != nil {
		return err
	}
	ncHex := fmt.Sprintf("%08x", nc)
	uri := req.URL.RequestURI()
	auth := BuildAuthorization(chal, req.Method, uri, t.Username, t.Password, cnonce, ncHex)
	req.Header.Set("Authorization", auth)
	return nil
}

// cloneRequest clones req (including its context) and rewinds the body via
// GetBody so it can be sent more than once, which the digest handshake
// requires (an initial unauthenticated attempt, then a retry).
func cloneRequest(req *http.Request) (*http.Request, error) {
	r2 := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("isapi: rewind request body: %w", err)
		}
		r2.Body = body
	}
	return r2, nil
}
