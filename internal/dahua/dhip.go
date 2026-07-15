// Package dahua implements a pure-Go client for the Dahua/KBVision private
// protocol on TCP (default port 37777). Devices on this port speak "DVRIP": a
// binary login handshake (\xa0/\xb0 framed) followed by JSON-RPC carried in
// \xf6-framed packets. configManager.getConfig/setConfig read and write camera
// settings. No cgo, no vendor SDK.
//
// Reverse-engineered from mcw0's DahuaConsole (see docs-sdk/dahua, reference
// only). Frame headers are 32 bytes; length/id/session fields are little-endian.
package dahua

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const headerLen = 32

// Client is a connected, authenticated DVRIP session. Not safe for concurrent
// use: each Call is a single request followed by a single response.
type Client struct {
	conn      net.Conn
	sessionID uint32
	id        uint32
	timeout   time.Duration
	user      string // remembered for operations needing the current credential
	pass      string
}

// rpcResp is the generic JSON-RPC response envelope. Error is left raw because
// firmware returns it inconsistently: sometimes an object {code,message},
// sometimes a bare string.
type rpcResp struct {
	ID      int             `json:"id"`
	Session int64           `json:"session"`
	Result  json.RawMessage `json:"result"`
	Params  json.RawMessage `json:"params"`
	Error   json.RawMessage `json:"error"`
}

func (r rpcResp) ok() bool {
	switch strings.TrimSpace(string(r.Result)) {
	case "", "false", "null", "0":
		return false
	}
	return true
}

// errMessage decodes Error whether it is an object {code,message} or a string.
func (r rpcResp) errMessage() string {
	raw := strings.TrimSpace(string(r.Error))
	if raw == "" || raw == "null" {
		return ""
	}
	var obj struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(r.Error, &obj); err == nil && obj.Message != "" {
		if obj.Code != 0 {
			return fmt.Sprintf("%s (code %d)", obj.Message, obj.Code)
		}
		return obj.Message
	}
	var s string
	if err := json.Unmarshal(r.Error, &s); err == nil {
		return s
	}
	return raw
}

// Dial connects and logs in to a Dahua/KBVision device over DVRIP.
func Dial(addr, username, password string, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	c := &Client{conn: conn, timeout: timeout, user: username, pass: password}
	if err := c.login(username, password); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

// Close terminates the session.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) login(username, password string) error {
	// Step 1: realm/random request (\xa0\x01 control frame, no payload).
	realmReq := make([]byte, headerLen)
	binary.BigEndian.PutUint32(realmReq[0:4], 0xa0010000)
	binary.BigEndian.PutUint64(realmReq[24:32], 0x050201010000a1aa)
	if err := c.writeRaw(realmReq); err != nil {
		return fmt.Errorf("realm request: %w", err)
	}
	_, body, err := c.readFrame()
	if err != nil {
		return fmt.Errorf("realm response: %w", err)
	}
	realm, random, err := parseRealm(string(body))
	if err != nil {
		return err
	}

	// Step 2: send login hash (\xa0\x05 frame) and check the response ErrorCode.
	hash := dvripLoginHash(username, password, realm, random)
	loginReq := make([]byte, headerLen)
	binary.BigEndian.PutUint32(loginReq[0:4], 0xa0050000)
	binary.LittleEndian.PutUint32(loginReq[4:8], uint32(len(hash)))
	binary.BigEndian.PutUint64(loginReq[24:32], 0x050200080000a1aa)
	if err := c.writeRaw(append(loginReq, []byte(hash)...)); err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	hdr, _, err := c.readFrame()
	if err != nil {
		return fmt.Errorf("login response: %w", err)
	}
	// ErrorCode lives in header[8:12]; 0x0008 = success. SessionID in header[16:20].
	errCode := hdr[8:12]
	if !(errCode[0] == 0x00 && errCode[1] == 0x08) {
		return fmt.Errorf("login failed: %s", dvripErrString(errCode))
	}
	c.sessionID = binary.LittleEndian.Uint32(hdr[16:20])
	return nil
}

// parseRealm extracts the realm ("Login to XXXX") and random from the DVRIP
// realm response body, e.g. "Realm:Login to 1803..\r\nRandom:1660..\r\n\r\n".
func parseRealm(body string) (realm, random string, err error) {
	i := strings.Index(body, "Login to")
	if i < 0 {
		return "", "", fmt.Errorf("realm not found in response: %q", body)
	}
	j := strings.Index(body[i:], "\r\n")
	if j < 0 {
		return "", "", fmt.Errorf("malformed realm line: %q", body)
	}
	realm = body[i : i+j]

	r := strings.Index(body, "Random:")
	if r < 0 {
		return "", "", fmt.Errorf("random not found in response: %q", body)
	}
	rest := body[r+len("Random:"):]
	if k := strings.Index(rest, "\r\n"); k >= 0 {
		rest = rest[:k]
	}
	random = strings.TrimSpace(rest)
	if realm == "" || random == "" {
		return "", "", fmt.Errorf("empty realm/random: %q", body)
	}
	return realm, random, nil
}

func dvripErrString(code []byte) string {
	switch {
	case code[0] == 0x01 && code[1] == 0x00:
		return "authentication failed (wrong password)"
	case code[0] == 0x01 && code[1] == 0x01:
		return "username invalid"
	case code[0] == 0x01 && code[1] == 0x04:
		return "account locked"
	case code[0] == 0x01 && code[1] == 0x11:
		return "device not initialised"
	case code[0] == 0x03 && code[1] == 0x03:
		return "user already logged in"
	default:
		return fmt.Sprintf("error code % x", code)
	}
}

// Call issues a JSON-RPC method with params over the DVRIP JSON (\xf6) frame.
func (c *Client) Call(method string, params any) (rpcResp, error) {
	c.id++
	env := map[string]any{
		"method":  method,
		"id":      c.id,
		"session": c.sessionID,
	}
	if params != nil {
		env["params"] = params
	}
	payload, err := json.Marshal(env)
	if err != nil {
		return rpcResp{}, err
	}
	hdr := make([]byte, headerLen)
	binary.BigEndian.PutUint32(hdr[0:4], 0xf6000000)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(payload)))
	binary.LittleEndian.PutUint32(hdr[8:12], c.id)
	binary.LittleEndian.PutUint32(hdr[16:20], uint32(len(payload)))
	binary.LittleEndian.PutUint32(hdr[24:28], c.sessionID)
	if err := c.writeRaw(append(hdr, payload...)); err != nil {
		return rpcResp{}, err
	}
	_, body, err := c.readFrame()
	if err != nil {
		return rpcResp{}, err
	}
	var resp rpcResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return rpcResp{}, fmt.Errorf("decode response: %w (raw: %.200s)", err, body)
	}
	return resp, nil
}

func (c *Client) writeRaw(b []byte) error {
	_ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	if _, err := c.conn.Write(b); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

const maxFrame = 32 << 20 // 32 MiB sanity cap

// readFrame reads a DVRIP frame and returns the first header + full payload.
// Each frame is a 32-byte header + a chunk of length header[4:8]. A large JSON
// (\xf6) response — e.g. an NVR's whole Encode config — is FRAGMENTED across
// several such frames, with header[16:20] carrying the total JSON size; we
// accumulate frames until we have all of it. \xb0 login frames are single.
func (c *Client) readFrame() (header, payload []byte, err error) {
	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	header = make([]byte, headerLen)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	chunk := binary.LittleEndian.Uint32(header[4:8])
	if chunk == 0 {
		return header, []byte{}, nil
	}
	if chunk > maxFrame {
		return nil, nil, fmt.Errorf("frame too large: chunk=%d", chunk)
	}
	payload = make([]byte, chunk)
	if _, err := io.ReadFull(c.conn, payload); err != nil {
		return nil, nil, fmt.Errorf("read payload: %w", err)
	}

	// JSON (\xf6) responses fragment: keep reading (header + chunk) frames and
	// appending their payloads until we have the full `total` bytes. header[16:20]
	// is the total ONLY for JSON frames; on \xb0 login frames it is not a length,
	// so this block must not run for them.
	if header[0] == 0xf6 {
		total := binary.LittleEndian.Uint32(header[16:20])
		if total > maxFrame {
			return nil, nil, fmt.Errorf("json frame too large: total=%d", total)
		}
		for uint32(len(payload)) < total {
			_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
			h := make([]byte, headerLen)
			if _, err := io.ReadFull(c.conn, h); err != nil {
				return nil, nil, fmt.Errorf("read continuation header: %w", err)
			}
			cn := binary.LittleEndian.Uint32(h[4:8])
			if cn == 0 {
				break
			}
			if cn > maxFrame || uint32(len(payload))+cn > maxFrame {
				return nil, nil, fmt.Errorf("continuation frame too large: %d", cn)
			}
			p := make([]byte, cn)
			if _, err := io.ReadFull(c.conn, p); err != nil {
				return nil, nil, fmt.Errorf("read continuation payload: %w", err)
			}
			payload = append(payload, p...)
		}
	}
	return header, payload, nil
}
