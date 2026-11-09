package xiongmai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PreLoginInfo captures unauthenticated metadata returned by POST /cgi-bin/login.cgi {"Name":"GetPreLoginInfo"}.
type PreLoginInfo struct {
	Ret      int    `json:"Ret"`
	Language string `json:"Language"`
	TCPPort  int    `json:"TCPPort"`
}

// doXiongmaiHTTP executes a single atomic HTTP request against Xiongmai's
// embedded uc-httpd 1.0.0. For plain http:// URLs it uses doRawHTTP10 directly
// so uc-httpd's non-RFC leading space (" Content-type: text/plain\r\n") never
// causes net/http to fail response parsing and double-POST stateful CGI calls
// (such as GetSalt, Login, SetConfig, or websocket.cgi).
func doXiongmaiHTTP(ctx context.Context, hc *http.Client, method, rawURL, contentType string, body []byte, timeout time.Duration, maxBytes int64) (int, []byte, error) {
	if maxBytes <= 0 {
		maxBytes = 8 << 20
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "http://") {
		return doRawHTTP10(ctx, method, rawURL, contentType, body, timeout, maxBytes)
	}
	if hc == nil {
		hc = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	respBytes, rerr := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	return resp.StatusCode, respBytes, rerr
}

func doRawHTTP10(ctx context.Context, method, rawURL, contentType string, body []byte, timeout time.Duration, maxBytes int64) (int, []byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, nil, err
	}
	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		hostPort = net.JoinHostPort(hostPort, "80")
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		return 0, nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	reqURI := u.RequestURI()
	if reqURI == "" {
		reqURI = "/"
	}
	var hdr bytes.Buffer
	fmt.Fprintf(&hdr, "%s %s HTTP/1.0\r\nHost: %s\r\nConnection: close\r\n", method, reqURI, u.Host)
	if contentType != "" {
		fmt.Fprintf(&hdr, "Content-Type: %s\r\n", contentType)
	}
	if len(body) > 0 || method == http.MethodPost {
		fmt.Fprintf(&hdr, "Content-Length: %d\r\n", len(body))
	}
	hdr.WriteString("\r\n")
	if len(body) > 0 {
		hdr.Write(body)
	}
	if _, err := conn.Write(hdr.Bytes()); err != nil {
		return 0, nil, err
	}

	br := bufio.NewReaderSize(conn, 64<<10)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		return 0, nil, err
	}
	statusCode := http.StatusOK
	fields := strings.Fields(statusLine)
	if len(fields) >= 2 {
		_, _ = fmt.Sscanf(fields[1], "%d", &statusCode)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
	}
	respBytes, err := io.ReadAll(io.LimitReader(br, maxBytes))
	return statusCode, respBytes, err
}

// ProbePreLoginCGI sends an unauthenticated GetPreLoginInfo probe to HTTP port
// (default 80) to identify Xiongmai CGI-capable NVRs/HVRs/IPCs during LAN discovery.
func ProbePreLoginCGI(ctx context.Context, baseURL string, timeout time.Duration) (*PreLoginInfo, error) {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	u := strings.TrimRight(baseURL, "/") + "/cgi-bin/login.cgi"
	statusCode, body, err := doXiongmaiHTTP(ctx, nil, http.MethodPost, u, "application/json", []byte(`{"Name":"GetPreLoginInfo"}`), timeout, 64<<10)
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("xiongmai GetPreLoginInfo HTTP %d", statusCode)
	}
	var info PreLoginInfo
	if err := json.Unmarshal(bytes.TrimRight(body, "\x00\r\n "), &info); err != nil {
		return nil, err
	}
	if info.Ret != RetOK || info.TCPPort <= 0 {
		return nil, fmt.Errorf("not a Xiongmai GetPreLoginInfo payload")
	}
	return &info, nil
}

// CGITransport implements Transport over Xiongmai's HTTP /cgi-bin/*.cgi +
// RFC 6455 WebSocket /websocket-bin/* interface (RSA-1024 + AES-128-CBC ZeroPad).
type CGITransport struct {
	host     string
	port     int
	baseURL  string
	username string
	password string
	timeout  time.Duration
	hc       *http.Client

	mu        sync.Mutex
	salt      string
	sessionID string
	aesKey    string
	useAES    bool
	loginInfo *LoginInfo
}

// NewCGITransport creates a pure-Go HTTP CGI + WebSocket transport.
func NewCGITransport(host string, port int, username, password string, timeout time.Duration) *CGITransport {
	if port <= 0 || port == DefaultSofiaPort {
		port = 80
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &CGITransport{
		host:     host,
		port:     port,
		baseURL:  fmt.Sprintf("http://%s", net.JoinHostPort(host, fmt.Sprintf("%d", port))),
		username: username,
		password: password,
		timeout:  timeout,
		hc:       &http.Client{Timeout: timeout},
	}
}

func (c *CGITransport) Name() string { return "http-cgi" }

// EffectiveUsername returns the authenticated username (after any GetRandomUser resolution).
func (c *CGITransport) EffectiveUsername() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.username == "" {
		return "admin"
	}
	return c.username
}

func (c *CGITransport) LoginInfo() *LoginInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loginInfo
}

func (c *CGITransport) Login(ctx context.Context) (*LoginInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loginInfo != nil && c.sessionID != "" {
		return c.loginInfo, nil
	}
	return c.loginLocked(ctx)
}

func randQuerySuffix() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "0.123456"
	}
	return fmt.Sprintf("0.%06d", n.Int64())
}

func (c *CGITransport) postRawJSON(ctx context.Context, cgiPath string, reqObj any) ([]byte, error) {
	raw, err := json.Marshal(reqObj)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s%s?randnum=%s", c.baseURL, cgiPath, randQuerySuffix())
	statusCode, respBytes, err := doXiongmaiHTTP(ctx, c.hc, http.MethodPost, u, "application/json;charset=utf-8", raw, c.timeout, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDialUnreachable, err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: CGI %s HTTP %d", ErrDialUnreachable, cgiPath, statusCode)
	}
	return respBytes, nil
}

// resolveRandomUserLocked queries /cgi-bin/resetpwd.cgi GetDevInfo + GetRandomUser
// when the caller configured "admin" (or empty username) so NVRs where "admin"
// was renamed to e.g. "admin1" (due to IDS_ACC_InvalidUserName on newer firmware)
// authenticate on the first attempt without burning a failed login counter.
func (c *CGITransport) resolveRandomUserLocked(ctx context.Context) {
	if c.username != "" && !strings.EqualFold(c.username, "admin") {
		return
	}
	devBody, err := c.postRawJSON(ctx, "/cgi-bin/resetpwd.cgi", map[string]any{"Name": "GetDevInfo"})
	if err != nil {
		return
	}
	var devResp map[string]any
	if json.Unmarshal(bytes.TrimRight(devBody, "\x00\r\n "), &devResp) != nil || extractRet(devResp) != RetOK {
		return
	}
	devInfo, _ := devResp["GetDevInfo"].(map[string]any)
	sn := stringFieldAny(devInfo, "SerialNo")
	if len(sn) < 12 {
		return
	}
	randBody, err := c.postRawJSON(ctx, "/cgi-bin/resetpwd.cgi", map[string]any{"Name": "GetRandomUser"})
	if err != nil {
		return
	}
	var randResp map[string]any
	if json.Unmarshal(bytes.TrimRight(randBody, "\x00\r\n "), &randResp) != nil || extractRet(randResp) != RetOK {
		return
	}
	randInfo, _ := randResp["GetRandomUser"].(map[string]any)
	infoStr := stringFieldAny(randInfo, "InfoUser", "Info")
	if infoStr == "" {
		return
	}
	if user, pwd, err := DecryptRandomUserInfo(sn, infoStr); err == nil && user != "" {
		c.username = user
		if c.password == "" && pwd != "" && pwd != "101" && pwd != "102" {
			c.password = pwd
		}
	}
}

func (c *CGITransport) loginLocked(ctx context.Context) (*LoginInfo, error) {
	c.resolveRandomUserLocked(ctx)
	// Step 1: POST /cgi-bin/login.cgi {"Name":"GetSalt"}
	saltBody, err := c.postRawJSON(ctx, "/cgi-bin/login.cgi", map[string]any{"Name": "GetSalt"})
	if err != nil {
		return nil, err
	}
	var saltResp struct {
		Ret                 int    `json:"Ret"`
		Salt                string `json:"Salt"`
		PublicKey           string `json:"PublicKey"`
		LoginEncryptionType string `json:"LoginEncryptionType"`
		DataEncryptionType  string `json:"DataEncryptionType"`
		OEMHeader           string `json:"OEMHeader"`
	}
	if err := json.Unmarshal(bytes.TrimRight(saltBody, "\x00\r\n "), &saltResp); err != nil {
		return nil, fmt.Errorf("%w: invalid GetSalt JSON: %v", ErrDialUnreachable, err)
	}
	if saltResp.Ret != RetOK {
		return nil, &RetError{Op: "GetSalt", Ret: saltResp.Ret}
	}

	useAES := strings.Contains(strings.ToUpper(saltResp.DataEncryptionType), "AES")
	loginReq := map[string]any{
		"Name": "Login",
		"Salt": saltResp.Salt,
	}

	var aesKey string
	if strings.Contains(strings.ToUpper(saltResp.LoginEncryptionType), "RSA") && saltResp.PublicKey != "" {
		pub, err := ParseRSAPublicKey(saltResp.PublicKey)
		if err != nil {
			return nil, err
		}
		pwMD5 := HexMD5Lower(c.password)
		signPlain := saltResp.OEMHeader + saltResp.Salt + pwMD5
		signEnc, err := EncryptRSAPKCS1Hex(pub, []byte(signPlain))
		if err != nil {
			return nil, err
		}
		userEnc, err := EncryptRSAPKCS1Hex(pub, []byte(c.username))
		if err != nil {
			return nil, err
		}
		verkPlain, err := RandomHex96()
		if err != nil {
			return nil, err
		}
		verkEnc, err := EncryptRSAPKCS1Hex(pub, []byte(verkPlain))
		if err != nil {
			return nil, err
		}
		loginReq["LoginEncryptionType"] = "RSA"
		loginReq["User"] = userEnc
		loginReq["Sign"] = signEnc
		loginReq["VERK"] = verkEnc
		if useAES {
			aesKey, err = RandomAESKey16()
			if err != nil {
				return nil, err
			}
			dtakEnc, err := EncryptRSAPKCS1Hex(pub, []byte(aesKey))
			if err != nil {
				return nil, err
			}
			loginReq["DTAK"] = dtakEnc
		}
	} else {
		// Legacy MD5_8 CGI login fallback
		sign := MD5_8(saltResp.Salt + saltResp.OEMHeader + MD5_8(c.password))
		loginReq["LoginEncryptionType"] = "MD5_8"
		loginReq["User"] = c.username
		loginReq["Sign"] = sign
		useAES = false
	}

	// Step 2: POST /cgi-bin/login.cgi {"Name":"Login", ...} (plain JSON envelope)
	loginBody, err := c.postRawJSON(ctx, "/cgi-bin/login.cgi", loginReq)
	if err != nil {
		return nil, err
	}
	var loginResp map[string]any
	// Some firmwares encrypt the Login response with DTAK, most return plain JSON.
	plainLogin := bytes.TrimRight(loginBody, "\x00\r\n ")
	if useAES && len(plainLogin) > 0 && plainLogin[0] != '{' {
		if dec, derr := AESDecryptZeroPadB64(plainLogin, aesKey); derr == nil {
			plainLogin = dec
		}
	}
	if err := json.Unmarshal(plainLogin, &loginResp); err != nil {
		return nil, fmt.Errorf("xiongmai CGI Login decode: %w", err)
	}
	ret := extractRet(loginResp)
	if ret != RetOK {
		return nil, &RetError{Op: "CgiLogin", Ret: ret}
	}

	sessHex := stringFieldAny(loginResp, "SessionID")
	if sessHex == "" {
		sessHex = "0x00000001"
	}
	c.salt = saltResp.Salt
	c.sessionID = sessHex
	c.aesKey = aesKey
	c.useAES = useAES

	info := &LoginInfo{
		SessionIDHex:  sessHex,
		SessionIDNum:  parseSessionHex(sessHex),
		ChannelNum:    intFieldAny(loginResp, "ChannelNum"),
		ExtraChannel:  intFieldAny(loginResp, "ExtraChannel"),
		DeviceType:    strings.TrimSpace(stringFieldAny(loginResp, "DeviceType", "DeviceType ")),
		AliveInterval: intFieldAny(loginResp, "AliveInterval"),
		DataUseAES:    useAES,
		TransportName: c.Name(),
	}
	c.loginInfo = info
	return info, nil
}

func (c *CGITransport) doCGI(ctx context.Context, op, cgiPath string, req map[string]any) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.loginInfo == nil || c.sessionID == "" {
		if _, err := c.loginLocked(ctx); err != nil {
			return nil, err
		}
	}

	resp, err := c.sendAuthenticatedLocked(ctx, cgiPath, req)
	if err != nil {
		return nil, err
	}
	ret := extractRet(resp)
	if ret == RetNoLogin || ret == RetCGIFormatError || ret == RetEncryptError {
		c.loginInfo = nil
		c.sessionID = ""
		if _, err := c.loginLocked(ctx); err != nil {
			return nil, err
		}
		resp, err = c.sendAuthenticatedLocked(ctx, cgiPath, req)
		if err != nil {
			return nil, err
		}
		ret = extractRet(resp)
	}
	if ret != 0 && !IsRetSuccess(ret) {
		return resp, &RetError{Op: op, Ret: ret}
	}
	return resp, nil
}

func (c *CGITransport) sendAuthenticatedLocked(ctx context.Context, cgiPath string, req map[string]any) (map[string]any, error) {
	bodyMap := make(map[string]any, len(req)+2)
	for k, v := range req {
		bodyMap[k] = v
	}
	bodyMap["SessionID"] = c.sessionID
	bodyMap["Salt"] = c.salt

	rawJSON, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	var reqBytes []byte
	contentType := "application/json;charset=utf-8"
	if c.useAES && c.aesKey != "" {
		b64, err := AESEncryptZeroPadB64(rawJSON, c.aesKey)
		if err != nil {
			return nil, err
		}
		reqBytes = []byte(b64)
		contentType = "text/plain;charset=utf-8"
	} else {
		reqBytes = rawJSON
	}

	u := fmt.Sprintf("%s%s?randnum=%s", c.baseURL, cgiPath, randQuerySuffix())
	statusCode, respRaw, err := doXiongmaiHTTP(ctx, c.hc, http.MethodPost, u, contentType, reqBytes, c.timeout, 8<<20)
	if err != nil {
		return nil, fmt.Errorf("xiongmai CGI %s: %w", cgiPath, err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("xiongmai CGI %s HTTP %d", cgiPath, statusCode)
	}

	var plain []byte
	if c.useAES && c.aesKey != "" {
		plain, err = AESDecryptZeroPadB64(respRaw, c.aesKey)
		if err != nil {
			return nil, err
		}
	} else {
		plain = bytes.TrimRight(respRaw, "\x00\r\n ")
	}
	var out map[string]any
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("xiongmai CGI %s JSON decode: %w", cgiPath, err)
	}
	return out, nil
}

func (c *CGITransport) GetInfo(ctx context.Context, name string) (map[string]any, error) {
	return c.doCGI(ctx, "GetInfo("+name+")", "/cgi-bin/getinfo.cgi", map[string]any{"Name": name})
}

func (c *CGITransport) GetAbility(ctx context.Context, name string) (map[string]any, error) {
	return c.doCGI(ctx, "GetAbility("+name+")", "/cgi-bin/getability.cgi", map[string]any{"Name": name})
}

func (c *CGITransport) GetChannelAbility(ctx context.Context, name string) (map[string]any, error) {
	return c.doCGI(ctx, "GetChannelAbility("+name+")", "/cgi-bin/getchannelability.cgi", map[string]any{"Name": name})
}

func (c *CGITransport) GetConfig(ctx context.Context, name string) (map[string]any, error) {
	return c.doCGI(ctx, "GetConfig("+name+")", "/cgi-bin/getconfig.cgi", map[string]any{"Name": name})
}

func (c *CGITransport) SetConfig(ctx context.Context, name string, payload any) (map[string]any, error) {
	return c.doCGI(ctx, "SetConfig("+name+")", "/cgi-bin/setconfig.cgi", map[string]any{
		"Name": name,
		name:   payload,
	})
}

func (c *CGITransport) OpDev(ctx context.Context, name string, param any) (map[string]any, error) {
	req := map[string]any{"Name": name}
	if param != nil {
		req[name] = param
	}
	return c.doCGI(ctx, "OpDev("+name+")", "/cgi-bin/opdev.cgi", req)
}

func (c *CGITransport) PTZ(ctx context.Context, param any) (map[string]any, error) {
	return c.doCGI(ctx, "OPPTZControl", "/cgi-bin/opptz.cgi", map[string]any{
		"Name":         "OPPTZControl",
		"OPPTZControl": param,
	})
}

func (c *CGITransport) ModifyPassword(ctx context.Context, user, oldPass, newPass string) error {
	_, err := c.doCGI(ctx, "ModifyPassword", "/cgi-bin/usermanager.cgi", map[string]any{
		"Name":        "ModifyPassword",
		"EncryptType": "MD5",
		"UserName":    user,
		"PassWord":    MD5_8(oldPass),
		"NewPassWord": MD5_8(newPass),
	})
	return err
}

func (c *CGITransport) Snapshot(ctx context.Context, channel int) ([]byte, error) {
	if _, err := c.Login(ctx); err != nil {
		return nil, err
	}
	effUser := c.EffectiveUsername()
	// 1. Try Xiongmai HTTP snapshot endpoints (/webcapture.jpg and /cgi-bin/snapshot.cgi).
	endpoints := []string{
		fmt.Sprintf("%s/webcapture.jpg?command=snap&channel=%d&user=%s&password=%s",
			c.baseURL, channel+1, url.QueryEscape(effUser), url.QueryEscape(c.password)),
		fmt.Sprintf("%s/webcapture.jpg?command=snap&channel=%d", c.baseURL, channel),
		fmt.Sprintf("%s/cgi-bin/snapshot.cgi?chn=%d&u=%s&p=%s",
			c.baseURL, channel, url.QueryEscape(effUser), url.QueryEscape(c.password)),
	}
	for _, u := range endpoints {
		statusCode, data, err := doXiongmaiHTTP(ctx, c.hc, http.MethodGet, u, "", nil, c.timeout, 4<<20)
		if err != nil {
			continue
		}
		if statusCode == http.StatusOK && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
			return data, nil
		}
	}
	// 2. Fallback: pull 1 keyframe over WebSocket /websocket-bin/Type=HttpRealplay on HTTP port.
	if jpg, werr := c.snapshotViaWSRealplay(ctx, channel); werr == nil && len(jpg) > 0 {
		return jpg, nil
	}
	return nil, fmt.Errorf("xiongmai CGI snapshot endpoint unavailable")
}

func (c *CGITransport) snapshotViaWSRealplay(ctx context.Context, channel int) ([]byte, error) {
	ffPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	salt := c.salt
	c.mu.Unlock()
	if salt == "" {
		return nil, fmt.Errorf("missing CGI salt")
	}

	sctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	for _, streamName := range []string{"Sub", "Main"} {
		wsConn, br, err := c.dialWebSocketPath(sctx, fmt.Sprintf("/websocket-bin/Type=HttpRealplay?Salt=%s&Channel=%d&Stream=%s&XMHead=1",
			url.QueryEscape(salt), channel, streamName))
		if err != nil {
			continue
		}
		var esBuf bytes.Buffer
		ext := NewXMStreamExtractor(&esBuf, time.Time{}, time.Time{})
		deadline := time.Now().Add(4 * time.Second)
		for time.Now().Before(deadline) {
			if ext.VideoFrames() >= 2 || (ext.VideoFrames() >= 1 && ext.InFlightBytes() == 0 && esBuf.Len() > 256) {
				break
			}
			_ = wsConn.SetReadDeadline(deadline)
			op, payload, rerr := readWSFrame(br, wsConn)
			if rerr != nil || op == 8 {
				break
			}
			if op == 2 && len(payload) > 0 {
				_ = ext.Feed(payload)
			}
		}
		_ = wsConn.Close()
		_ = ext.Finish()
		if esBuf.Len() < 64 {
			continue
		}
		demuxFmt := DetectAnnexBCodec(esBuf.Bytes())
		// Append a trailing Annex-B AUD startcode so FFmpeg's raw parser flushes single-frame IDR buffers.
		_, _ = esBuf.Write([]byte{0x00, 0x00, 0x00, 0x01, 0x09, 0xF0})
		cmd := exec.CommandContext(sctx, ffPath,
			"-hide_banner", "-loglevel", "error",
			"-f", demuxFmt,
			"-i", "pipe:0",
			"-frames:v", "1",
			"-q:v", "4",
			"-f", "image2",
			"-vcodec", "mjpeg",
			"pipe:1",
		)
		cmd.Stdin = bytes.NewReader(esBuf.Bytes())
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err == nil && bytes.HasPrefix(stdout.Bytes(), []byte{0xFF, 0xD8, 0xFF}) {
			return stdout.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("ws HttpRealplay snapshot unavailable")
}

// DownloadRecordFile streams recorded H.264/H.265 media over Xiongmai's native
// RFC 6455 WebSocket /websocket-bin/Type=HttpPlayBack + /cgi-bin/websocket.cgi.
func (c *CGITransport) DownloadRecordFile(ctx context.Context, w io.Writer, channel int, file RecordFile, start, end time.Time) error {
	if _, err := c.Login(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	salt := c.salt
	c.mu.Unlock()

	wsConn, br, err := c.dialWebSocketPlayback(ctx, salt, channel)
	if err != nil {
		return err
	}
	defer wsConn.Close()

	// Expect initial text frame: {"Ret":100,"State":"Begin"}
	_ = wsConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	opCode, initMsg, err := readWSFrame(br, wsConn)
	if err != nil {
		return fmt.Errorf("xiongmai ws playback init read: %w", err)
	}
	if opCode == 1 {
		var st map[string]any
		if json.Unmarshal(bytes.TrimRight(initMsg, "\x00\r\n "), &st) == nil {
			if r := extractRet(st); r != 0 && !IsRetSuccess(r) {
				return &RetError{Op: "ws.HttpPlayBack.Begin", Ret: r}
			}
		}
	}

	localTime := file.RawBegin
	if !start.IsZero() && (file.BeginTime.IsZero() || start.After(file.BeginTime)) {
		localTime = start.Format("2006-01-02 15:04:05")
	}

	// Send StartDownLoad over /cgi-bin/websocket.cgi
	_, err = c.doCGI(ctx, "HttpPlayBack.StartDownLoad", "/cgi-bin/websocket.cgi", map[string]any{
		"Name": "HttpPlayBack",
		"HttpPlayBack": map[string]any{
			"Opr":       "StartDownLoad",
			"PlayMode":  "PlayByName",
			"Channel":   channel,
			"Stream":    "Main",
			"ExactSeek": 1,
			"PlayByName": map[string]any{
				"LocalTime": localTime,
				"FileName":  file.FileName,
			},
		},
	})
	if err != nil {
		return err
	}

	ext := NewXMStreamExtractor(w, start, end)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = wsConn.SetReadDeadline(time.Now().Add(15 * time.Second))
		op, payload, err := readWSFrame(br, wsConn)
		if err != nil {
			if ext.WroteBytes() > 0 {
				break
			}
			return err
		}
		if op == 8 { // Close frame
			break
		}
		if op == 1 { // Text frame (e.g. {"Ret":100,"State":"End"})
			if bytes.Contains(payload, []byte(`"End"`)) || bytes.Contains(payload, []byte(`"Stop"`)) {
				break
			}
			continue
		}
		if op == 2 && len(payload) > 0 { // Binary media frame
			if err := ext.Feed(payload); err != nil {
				if errors.Is(err, ErrXMStopTimeReached) {
					break
				}
				return err
			}
		}
	}
	if err := ext.Finish(); err != nil {
		return err
	}
	if st := xmStatsFromContext(ctx); st != nil {
		st.recordExtractor(ext)
	}
	if ext.WroteBytes() == 0 {
		return fmt.Errorf("xiongmai ws HttpPlayBack returned 0 media bytes")
	}
	return nil
}

func (c *CGITransport) dialWebSocketPlayback(ctx context.Context, salt string, channel int) (net.Conn, *bufio.Reader, error) {
	path := fmt.Sprintf("/websocket-bin/Type=HttpPlayBack?Salt=%s&Channel=%d&Stream=Main&XMHead=1",
		url.QueryEscape(salt), channel)
	return c.dialWebSocketPath(ctx, path)
}

func (c *CGITransport) dialWebSocketPath(ctx context.Context, path string) (net.Conn, *bufio.Reader, error) {
	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	var keyRaw [16]byte
	_, _ = rand.Read(keyRaw[:])
	wsKey := base64.StdEncoding.EncodeToString(keyRaw[:])
	handshake := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		path, addr, wsKey)
	_ = conn.SetDeadline(time.Now().Add(c.timeout))
	if _, err := conn.Write([]byte(handshake)); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	br := bufio.NewReaderSize(conn, 128<<10)
	statusLine, err := br.ReadString('\n')
	if err != nil || !strings.Contains(statusLine, "101") {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("xiongmai ws upgrade failed: %s (%v)", strings.TrimSpace(statusLine), err)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}
	return conn, br, nil
}

func readWSFrame(br *bufio.Reader, conn net.Conn) (byte, []byte, error) {
	var msg bytes.Buffer
	var firstOpcode byte
	for {
		var hdr [2]byte
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			return 0, nil, err
		}
		fin := (hdr[0] & 0x80) != 0
		opcode := hdr[0] & 0x0F
		masked := (hdr[1] & 0x80) != 0
		length := uint64(hdr[1] & 0x7F)
		switch length {
		case 126:
			var ext [2]byte
			if _, err := io.ReadFull(br, ext[:]); err != nil {
				return 0, nil, err
			}
			length = uint64(binary.BigEndian.Uint16(ext[:]))
		case 127:
			var ext [8]byte
			if _, err := io.ReadFull(br, ext[:]); err != nil {
				return 0, nil, err
			}
			length = binary.BigEndian.Uint64(ext[:])
		}
		if length > maxSofiaPayloadBytes {
			return 0, nil, fmt.Errorf("xiongmai ws frame too large (%d)", length)
		}
		var maskKey [4]byte
		if masked {
			if _, err := io.ReadFull(br, maskKey[:]); err != nil {
				return 0, nil, err
			}
		}
		payload := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(br, payload); err != nil {
				return 0, nil, err
			}
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
		if opcode == 9 { // Ping -> send masked Pong
			pong := []byte{0x8A, 0x80, 0, 0, 0, 0}
			_, _ = conn.Write(pong)
			continue
		}
		if firstOpcode == 0 && opcode != 0 {
			firstOpcode = opcode
		}
		msg.Write(payload)
		if fin {
			return firstOpcode, msg.Bytes(), nil
		}
	}
}

func (c *CGITransport) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loginInfo == nil || c.sessionID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, _ = c.sendAuthenticatedLocked(ctx, "/cgi-bin/login.cgi", map[string]any{"Name": "Logout"})
	c.loginInfo = nil
	c.sessionID = ""
	return nil
}
