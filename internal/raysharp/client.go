// Package raysharp implements a pure-Go client for Raysharp Technology NVR and
// AI IP Camera devices (HTTP JSON API V1.0 / V1.026 on port 80/443).
//
// Authentication uses the 3-factor session mechanism verified on N10256N and
// VC31 hardware:
//  1. POST /API/Web/Login with {"data":{"remote_terminal_info":"WEB,chrome"}}
//     to obtain a 401 WWW-Authenticate Digest challenge.
//  2. POST /API/Web/Login with Digest Authorization (MD5 or SHA-256) to receive
//     both Set-Cookie (session_80 / session) and X-csrftoken response headers.
//  3. Subsequent POST /API/<Path> requests attach Cookie and X-csrftoken.
//  4. Close() sends POST /API/Web/Logout to free the device session slot.
package raysharp

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrAuthFailed is returned when Digest credentials are rejected.
var ErrAuthFailed = errors.New("raysharp: authentication failed")

// ErrSessionExpired is returned when the device reports "one_IE", "no_login", or "expired".
var ErrSessionExpired = errors.New("raysharp: session expired or invalid csrf token")

// Client manages an authenticated HTTP session with a Raysharp NVR or IP camera.
type Client struct {
	baseURL    string
	host       string
	port       int
	username   string
	password   string
	timeout    time.Duration
	httpClient *http.Client

	mu        sync.Mutex
	loggedIn  bool
	cookie    string
	csrfToken string
	devType   string
}

// New creates a new Raysharp client. Like hik.Dial, it does not dial the
// network immediately; authentication occurs lazily on first Call or explicitly
// via Login().
func New(host string, port int, username, password string, timeout time.Duration) *Client {
	if port <= 0 {
		port = 80
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	scheme := "http"
	base := fmt.Sprintf("%s://%s:%d", scheme, host, port)
	return &Client{
		baseURL:  base,
		host:     host,
		port:     port,
		username: username,
		password: password,
		timeout:  timeout,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewWithBaseURL creates a Client pointing at an explicit baseURL (used by unit
// tests with httptest.Server).
func NewWithBaseURL(baseURL, username, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{
		baseURL:    baseURL,
		host:       "127.0.0.1",
		port:       80,
		username:   username,
		password:   password,
		timeout:    timeout,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// APIEnvelope is the standard Raysharp JSON response wrapper.
type APIEnvelope struct {
	Version    string          `json:"version,omitempty"`
	Result     string          `json:"result,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	ErrorCode  string          `json:"error_code,omitempty"`
	DeviceType string          `json:"device_type,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// PreLoginRange holds the unauthenticated /API/Login/Range probe response.
type PreLoginRange struct {
	DeviceType     string `json:"device_type"`
	HTTPAPIVersion string `json:"http_api_version"`
	SiteVersion    string `json:"site_version"`
	DefaultLang    string `json:"default_lang"`
	CustomName     int    `json:"custom_name"`
}

// ProbeLoginRange queries POST /API/Login/Range without authentication to
// detect whether an HTTP server is a Raysharp NVR or IP camera.
func ProbeLoginRange(ctx context.Context, baseURL string, timeout time.Duration) (*PreLoginRange, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	cl := &http.Client{Timeout: timeout}
	reqBody := []byte(`{"version":"1.0","data":{}}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/API/Login/Range", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("raysharp Login/Range HTTP %d", resp.StatusCode)
	}
	var env APIEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if !strings.EqualFold(env.Result, "success") {
		return nil, fmt.Errorf("raysharp Login/Range result=%q", env.Result)
	}
	var info PreLoginRange
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &info)
	}
	if info.HTTPAPIVersion == "" && info.SiteVersion == "" && info.DeviceType == "" {
		return nil, fmt.Errorf("not a raysharp Login/Range payload")
	}
	return &info, nil
}

var digestKVRe = regexp.MustCompile(`([a-zA-Z0-9_-]+)=(?:"([^"]*)"|([^,\s]+))`)

func parseDigestChallenge(header string) map[string]string {
	out := make(map[string]string)
	for _, m := range digestKVRe.FindAllStringSubmatch(header, -1) {
		k := strings.ToLower(strings.TrimSpace(m[1]))
		v := m[2]
		if v == "" {
			v = m[3]
		}
		out[k] = v
	}
	return out
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "1234567890abcdef"
	}
	return hex.EncodeToString(b)
}

func hashHex(algo, s string) string {
	var h hash.Hash
	if strings.EqualFold(algo, "SHA-256") || strings.EqualFold(algo, "SHA256") {
		h = sha256.New()
	} else {
		h = md5.New()
	}
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func buildDigestHeader(user, pass, method, uri, wwwAuth string) string {
	params := parseDigestChallenge(wwwAuth)
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]
	if idx := strings.Index(qop, ","); idx >= 0 {
		qop = strings.TrimSpace(qop[:idx])
	}
	if qop == "" {
		qop = "auth"
	}
	opaque := params["opaque"]
	algo := params["algorithm"]

	nc := "00000001"
	cnonce := randomHex(8)
	ha1 := hashHex(algo, fmt.Sprintf("%s:%s:%s", user, realm, pass))
	ha2 := hashHex(algo, fmt.Sprintf("%s:%s", method, uri))
	response := hashHex(algo, fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))

	var b strings.Builder
	b.WriteString("Digest ")
	if algo != "" {
		fmt.Fprintf(&b, `algorithm="%s", `, algo)
	}
	fmt.Fprintf(&b, `username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		user, realm, nonce, uri, response)
	if opaque != "" {
		fmt.Fprintf(&b, `, opaque="%s"`, opaque)
	}
	fmt.Fprintf(&b, `, qop=%s, nc=%s, cnonce="%s"`, qop, nc, cnonce)
	return b.String()
}

// Login performs the 2-step Digest + Session Cookie + X-csrftoken handshake.
func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginLocked(ctx)
}

func (c *Client) loginLocked(ctx context.Context) error {
	uri := "/API/Web/Login"
	url := c.baseURL + uri
	loginPayload := []byte(`{"data":{"remote_terminal_info":"WEB,chrome"}}`)

	// Step 1: Unauthenticated POST to receive 401 WWW-Authenticate Digest challenge
	req1, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(loginPayload))
	if err != nil {
		return err
	}
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp1, err := c.httpClient.Do(req1)
	if err != nil {
		return fmt.Errorf("raysharp login dial %s: %w", c.baseURL, err)
	}
	body1, _ := io.ReadAll(io.LimitReader(resp1.Body, 1<<20))
	resp1.Body.Close()

	if resp1.StatusCode == http.StatusOK {
		return c.extractSessionLocked(resp1, body1)
	}
	if resp1.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("raysharp login step1 HTTP %d: %s", resp1.StatusCode, strings.TrimSpace(string(body1)))
	}

	wwwAuth := resp1.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return fmt.Errorf("raysharp login 401 missing WWW-Authenticate header")
	}

	// Step 2: Authenticated Digest POST
	authHdr := buildDigestHeader(c.username, c.password, http.MethodPost, uri, wwwAuth)
	req2, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(loginPayload))
	if err != nil {
		return err
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Requested-With", "XMLHttpRequest")
	req2.Header.Set("Authorization", authHdr)

	resp2, err := c.httpClient.Do(req2)
	if err != nil {
		return fmt.Errorf("raysharp login step2 %s: %w", c.baseURL, err)
	}
	body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 1<<20))
	resp2.Body.Close()

	if resp2.StatusCode == http.StatusUnauthorized || resp2.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: HTTP %d", ErrAuthFailed, resp2.StatusCode)
	}
	if resp2.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d: %s", ErrAuthFailed, resp2.StatusCode, strings.TrimSpace(string(body2)))
	}

	return c.extractSessionLocked(resp2, body2)
}

func (c *Client) extractSessionLocked(resp *http.Response, body []byte) error {
	var env APIEnvelope
	if len(body) > 0 {
		if err := json.Unmarshal(body, &env); err == nil {
			if strings.EqualFold(env.Result, "failed") {
				return fmt.Errorf("%w: %s (%s)", ErrAuthFailed, env.ErrorCode, env.Reason)
			}
			if env.DeviceType != "" {
				c.devType = env.DeviceType
			}
		}
	}

	for _, ck := range resp.Cookies() {
		if strings.HasPrefix(ck.Name, "session") && ck.Value != "" {
			c.cookie = ck.Name + "=" + ck.Value
			break
		}
	}
	if c.cookie == "" {
		if rawSetCookie := resp.Header.Get("Set-Cookie"); rawSetCookie != "" {
			c.cookie = strings.TrimSpace(strings.Split(rawSetCookie, ";")[0])
		}
	}

	csrf := resp.Header.Get("X-csrftoken")
	if csrf == "" {
		csrf = resp.Header.Get("X-Csrftoken")
	}
	if csrf != "" {
		c.csrfToken = strings.TrimSpace(strings.Split(csrf, ",")[0])
	}
	if c.csrfToken == "" && len(env.Data) > 0 {
		var ld struct {
			Token string `json:"token"`
		}
		if json.Unmarshal(env.Data, &ld) == nil && ld.Token != "" {
			c.csrfToken = ld.Token
		}
	}
	c.loggedIn = true
	return nil
}

// Call invokes POST /API/<apiPath> with {"version":"1.0","data":reqData},
// logging in automatically if needed and retrying once on session expiry.
func (c *Client) Call(ctx context.Context, apiPath string, reqData any) (*APIEnvelope, error) {
	apiPath = strings.TrimPrefix(strings.TrimSpace(apiPath), "/")
	apiPath = strings.TrimPrefix(apiPath, "API/")

	c.mu.Lock()
	if !c.loggedIn {
		if err := c.loginLocked(ctx); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}
	cookie := c.cookie
	csrf := c.csrfToken
	c.mu.Unlock()

	env, err := c.doCall(ctx, apiPath, reqData, cookie, csrf)
	if errors.Is(err, ErrSessionExpired) {
		c.mu.Lock()
		c.loggedIn = false
		loginErr := c.loginLocked(ctx)
		cookie = c.cookie
		csrf = c.csrfToken
		c.mu.Unlock()
		if loginErr != nil {
			return nil, loginErr
		}
		env, err = c.doCall(ctx, apiPath, reqData, cookie, csrf)
	}
	return env, err
}

func (c *Client) doCall(ctx context.Context, apiPath string, reqData any, cookie, csrf string) (*APIEnvelope, error) {
	if reqData == nil {
		reqData = map[string]any{}
	}
	payload := map[string]any{
		"version": "1.0",
		"data":    reqData,
	}
	rawReq, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("raysharp marshal %s: %w", apiPath, err)
	}

	url := fmt.Sprintf("%s/API/%s", c.baseURL, apiPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawReq))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	if csrf != "" {
		req.Header.Set("X-csrftoken", csrf)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("raysharp %s: %w", apiPath, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("raysharp read %s: %w", apiPath, err)
	}

	var env APIEnvelope
	if len(body) > 0 {
		_ = json.Unmarshal(body, &env)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrSessionExpired
	}
	if env.ErrorCode == "one_IE" || env.ErrorCode == "no_login" || env.ErrorCode == "expired" {
		return nil, ErrSessionExpired
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if env.ErrorCode != "" || env.Reason != "" {
			return &env, fmt.Errorf("raysharp %s HTTP %d: %s (%s)", apiPath, resp.StatusCode, env.ErrorCode, env.Reason)
		}
		return &env, fmt.Errorf("raysharp %s HTTP %d: %s", apiPath, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if strings.EqualFold(env.Result, "failed") {
		return &env, fmt.Errorf("raysharp %s failed: %s (%s)", apiPath, env.ErrorCode, env.Reason)
	}
	// Some /API/AI/* endpoints omit top-level "result":"success" and put
	// {"result":"failed","error_code":"..."} inside data.
	if len(env.Data) > 0 {
		var inner struct {
			Result    string `json:"result"`
			ErrorCode string `json:"error_code"`
			Reason    string `json:"reason"`
		}
		if json.Unmarshal(env.Data, &inner) == nil && strings.EqualFold(inner.Result, "failed") {
			return &env, fmt.Errorf("raysharp %s failed: %s (%s)", apiPath, inner.ErrorCode, inner.Reason)
		}
	}
	return &env, nil
}

// CallRaw executes an arbitrary POST /API/<apiPath> request and returns the raw
// decoded JSON map so callers can access all 200+ Raysharp API routes directly.
func (c *Client) CallRaw(ctx context.Context, apiPath string, reqData any) (map[string]any, error) {
	env, err := c.Call(ctx, apiPath, reqData)
	out := make(map[string]any)
	if env != nil {
		if env.Result != "" {
			out["result"] = env.Result
		}
		if env.ErrorCode != "" {
			out["error_code"] = env.ErrorCode
		}
		if env.Reason != "" {
			out["reason"] = env.Reason
		}
		if len(env.Data) > 0 {
			var d any
			if json.Unmarshal(env.Data, &d) == nil {
				out["data"] = d
			} else {
				out["data"] = string(env.Data)
			}
		}
	}
	return out, err
}

// Close logs out of the Raysharp session if one was opened.
func (c *Client) Close() error {
	c.mu.Lock()
	if !c.loggedIn {
		c.mu.Unlock()
		return nil
	}
	cookie := c.cookie
	csrf := c.csrfToken
	c.loggedIn = false
	c.cookie = ""
	c.csrfToken = ""
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = c.doCall(ctx, "Web/Logout", map[string]any{}, cookie, csrf)
	return nil
}

// NormalizeIPv4 strips leading zeros from each dotted-quad octet (e.g.
// "192.168.001.234" -> "192.168.1.234", "255.255.255.000" -> "255.255.255.0").
func NormalizeIPv4(s string) string {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return s
	}
	out := make([]string, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return s
		}
		out[i] = strconv.Itoa(n)
	}
	ip := strings.Join(out, ".")
	if net.ParseIP(ip) == nil {
		return s
	}
	return ip
}

// ChannelKey formats a 0-based channel index into Raysharp's "CH1".."CH256" key.
func ChannelKey(channel0 int) string {
	if channel0 < 0 {
		channel0 = 0
	}
	return fmt.Sprintf("CH%d", channel0+1)
}

// ParseChannelKey extracts the 0-based channel index from "CH1" -> 0, "CH2" -> 1.
func ParseChannelKey(key string) (int, bool) {
	key = strings.TrimSpace(strings.ToUpper(key))
	if !strings.HasPrefix(key, "CH") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(key, "CH"))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n - 1, true
}
