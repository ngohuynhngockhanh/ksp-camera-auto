package xiongmai

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// DefaultSofiaPort is the canonical Xiongmai DVRIP binary TCP port.
const DefaultSofiaPort = 34567

// Sofia 20-byte Little-Endian frame message IDs.
const (
	MsgLoginReq         uint16 = 1000
	MsgLoginRsp         uint16 = 1001
	MsgLogoutReq        uint16 = 1002
	MsgLogoutRsp        uint16 = 1003
	MsgKeepAliveReq     uint16 = 1006
	MsgKeepAliveRsp     uint16 = 1007
	MsgSysInfoReq       uint16 = 1020
	MsgSysInfoRsp       uint16 = 1021
	MsgConfigSet        uint16 = 1040
	MsgConfigSetRsp     uint16 = 1041
	MsgConfigGet        uint16 = 1042
	MsgConfigGetRsp     uint16 = 1043
	MsgChannelTitleSet  uint16 = 1046
	MsgChannelTitleSetR uint16 = 1047
	MsgChannelTitleGet  uint16 = 1048
	MsgChannelTitleGetR uint16 = 1049
	MsgAbilityGet       uint16 = 1360
	MsgAbilityGetRsp    uint16 = 1361
	MsgPTZReq           uint16 = 1400
	MsgPTZRsp           uint16 = 1401
	MsgPlayBackReq      uint16 = 1420
	MsgPlayBackRsp      uint16 = 1421
	MsgPlayBackClaim    uint16 = 1422
	MsgPlayBackClaimRsp uint16 = 1423
	MsgPlayBackData     uint16 = 1424
	MsgPlayBackEOF      uint16 = 1426
	MsgFileQueryReq     uint16 = 1440
	MsgFileQueryRsp     uint16 = 1441
	MsgSysManagerReq    uint16 = 1450
	MsgSysManagerRsp    uint16 = 1451
	MsgTimeQueryReq     uint16 = 1452
	MsgTimeQueryRsp     uint16 = 1453
	MsgModifyPswReq     uint16 = 1482
	MsgModifyPswRsp     uint16 = 1483
	MsgSnapReq          uint16 = 1560
	MsgSnapRsp          uint16 = 1561
)

const sofiaHeaderLen = 20
const maxSofiaPayloadBytes = 8 << 20 // 8 MiB safety cap

type sofiaHeader struct {
	HeadFlag       byte
	Version        byte
	Reserved1      byte
	Reserved2      byte
	SessionID      uint32
	SequenceNumber uint32
	TotalPacket    byte
	CurPacket      byte
	MessageID      uint16
	DataLength     uint32
}

func encodeSofiaPacket(sessID, seq uint32, msgID uint16, body []byte) []byte {
	pkt := make([]byte, sofiaHeaderLen+len(body))
	pkt[0] = 0xFF
	pkt[1] = 0x01
	binary.LittleEndian.PutUint32(pkt[4:8], sessID)
	binary.LittleEndian.PutUint32(pkt[8:12], seq)
	binary.LittleEndian.PutUint16(pkt[14:16], msgID)
	binary.LittleEndian.PutUint32(pkt[16:20], uint32(len(body)))
	copy(pkt[sofiaHeaderLen:], body)
	return pkt
}

func readSofiaHeader(r io.Reader) (sofiaHeader, error) {
	var raw [sofiaHeaderLen]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return sofiaHeader{}, err
	}
	if raw[0] != 0xFF {
		return sofiaHeader{}, fmt.Errorf("xiongmai sofia: invalid magic 0x%02X (expected 0xFF)", raw[0])
	}
	return sofiaHeader{
		HeadFlag:       raw[0],
		Version:        raw[1],
		Reserved1:      raw[2],
		Reserved2:      raw[3],
		SessionID:      binary.LittleEndian.Uint32(raw[4:8]),
		SequenceNumber: binary.LittleEndian.Uint32(raw[8:12]),
		TotalPacket:    raw[12],
		CurPacket:      raw[13],
		MessageID:      binary.LittleEndian.Uint16(raw[14:16]),
		DataLength:     binary.LittleEndian.Uint32(raw[16:20]),
	}, nil
}

// SofiaTCPTransport implements Transport over Xiongmai's binary TCP 34567 protocol.
type SofiaTCPTransport struct {
	host     string
	port     int
	username string
	password string
	timeout  time.Duration

	mu        sync.Mutex
	conn      net.Conn
	seq       uint32
	sessionID uint32
	sessHex   string
	loginInfo *LoginInfo
}

// NewSofiaTCPTransport constructs a Sofia binary DVRIP transport (default port 34567).
func NewSofiaTCPTransport(host string, port int, username, password string, timeout time.Duration) *SofiaTCPTransport {
	if port <= 0 {
		port = DefaultSofiaPort
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &SofiaTCPTransport{
		host:     host,
		port:     port,
		username: username,
		password: password,
		timeout:  timeout,
	}
}

func (t *SofiaTCPTransport) Name() string { return "sofia-tcp" }

func (t *SofiaTCPTransport) LoginInfo() *LoginInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.loginInfo
}

func (t *SofiaTCPTransport) addr() string {
	return net.JoinHostPort(t.host, fmt.Sprintf("%d", t.port))
}

func (t *SofiaTCPTransport) Login(ctx context.Context) (*LoginInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn != nil && t.loginInfo != nil {
		return t.loginInfo, nil
	}
	return t.loginLocked(ctx)
}

func (t *SofiaTCPTransport) loginLocked(ctx context.Context) (*LoginInfo, error) {
	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}
	dialer := net.Dialer{Timeout: t.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", t.addr())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDialUnreachable, err)
	}
	t.conn = conn
	t.seq = 0
	t.sessionID = 0
	t.sessHex = "0x0"

	req := map[string]any{
		"EncryptType": "MD5",
		"LoginType":   "DVRIP-Web",
		"PassWord":    MD5_8(t.password),
		"UserName":    t.username,
	}
	resp, hdr, err := t.rpcLocked(ctx, MsgLoginReq, req)
	if err != nil {
		_ = t.conn.Close()
		t.conn = nil
		return nil, err
	}
	ret := extractRet(resp)
	if ret != RetOK {
		_ = t.conn.Close()
		t.conn = nil
		return nil, &RetError{Op: "login", Ret: ret}
	}

	sessHex, _ := resp["SessionID"].(string)
	sessNum := parseSessionHex(sessHex)
	if sessNum == 0 && hdr.SessionID != 0 {
		sessNum = hdr.SessionID
	}
	if sessHex == "" {
		sessHex = fmt.Sprintf("0x%08X", sessNum)
	}
	t.sessionID = sessNum
	t.sessHex = sessHex

	devType := stringFieldAny(resp, "DeviceType", "DeviceType ")
	info := &LoginInfo{
		SessionIDHex:  sessHex,
		SessionIDNum:  sessNum,
		ChannelNum:    intFieldAny(resp, "ChannelNum"),
		ExtraChannel:  intFieldAny(resp, "ExtraChannel"),
		DeviceType:    strings.TrimSpace(devType),
		AliveInterval: intFieldAny(resp, "AliveInterval"),
		DataUseAES:    boolFieldAny(resp, "DataUseAES"),
		TransportName: t.Name(),
	}
	t.loginInfo = info
	return info, nil
}

func (t *SofiaTCPTransport) setDeadlineLocked(ctx context.Context) {
	if t.conn == nil {
		return
	}
	dl := time.Now().Add(t.timeout)
	if cdl, ok := ctx.Deadline(); ok && cdl.Before(dl) {
		dl = cdl
	}
	_ = t.conn.SetDeadline(dl)
}

func (t *SofiaTCPTransport) rpcLocked(ctx context.Context, msgID uint16, payload map[string]any) (map[string]any, sofiaHeader, error) {
	t.setDeadlineLocked(ctx)
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, sofiaHeader{}, err
	}
	body := append(rawJSON, '\n', 0x00)
	seq := t.seq
	t.seq++

	pkt := encodeSofiaPacket(t.sessionID, seq, msgID, body)
	if _, err := t.conn.Write(pkt); err != nil {
		return nil, sofiaHeader{}, fmt.Errorf("xiongmai sofia write msg=%d: %w", msgID, err)
	}

	var assembled bytes.Buffer
	var firstHdr sofiaHeader
	for {
		hdr, err := readSofiaHeader(t.conn)
		if err != nil {
			return nil, sofiaHeader{}, fmt.Errorf("xiongmai sofia read msg=%d: %w", msgID, err)
		}
		if hdr.DataLength > maxSofiaPayloadBytes {
			return nil, hdr, fmt.Errorf("xiongmai sofia payload too large (%d bytes)", hdr.DataLength)
		}
		chunk := make([]byte, hdr.DataLength)
		if hdr.DataLength > 0 {
			if _, err := io.ReadFull(t.conn, chunk); err != nil {
				return nil, hdr, fmt.Errorf("xiongmai sofia read body msg=%d: %w", msgID, err)
			}
		}
		// Skip spontaneous keepalive acks while waiting for our RPC response.
		if hdr.MessageID == MsgKeepAliveRsp && msgID != MsgKeepAliveReq {
			continue
		}
		if assembled.Len() == 0 {
			firstHdr = hdr
		}
		assembled.Write(chunk)
		if hdr.TotalPacket <= 1 || int(hdr.CurPacket)+1 >= int(hdr.TotalPacket) {
			break
		}
	}

	clean := bytes.TrimRight(assembled.Bytes(), "\x00\r\n ")
	if len(clean) == 0 {
		return map[string]any{"Ret": RetOK}, firstHdr, nil
	}
	var out map[string]any
	if err := json.Unmarshal(clean, &out); err != nil {
		return nil, firstHdr, fmt.Errorf("xiongmai sofia decode msg=%d: %w", msgID, err)
	}
	return out, firstHdr, nil
}

func (t *SofiaTCPTransport) doRPC(ctx context.Context, op string, msgID uint16, payload map[string]any) (map[string]any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil || t.loginInfo == nil {
		if _, err := t.loginLocked(ctx); err != nil {
			return nil, err
		}
	}
	if _, has := payload["SessionID"]; !has && t.sessHex != "" {
		payload["SessionID"] = t.sessHex
	}

	resp, _, err := t.rpcLocked(ctx, msgID, payload)
	if err != nil {
		// Reconnect once if TCP connection dropped cleanly between calls.
		if _, lerr := t.loginLocked(ctx); lerr != nil {
			return nil, err
		}
		payload["SessionID"] = t.sessHex
		resp, _, err = t.rpcLocked(ctx, msgID, payload)
		if err != nil {
			return nil, err
		}
	}
	ret := extractRet(resp)
	if ret == RetNoLogin {
		if _, lerr := t.loginLocked(ctx); lerr != nil {
			return nil, lerr
		}
		payload["SessionID"] = t.sessHex
		resp, _, err = t.rpcLocked(ctx, msgID, payload)
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

func (t *SofiaTCPTransport) GetInfo(ctx context.Context, name string) (map[string]any, error) {
	return t.doRPC(ctx, "GetInfo("+name+")", MsgSysInfoReq, map[string]any{"Name": name})
}

func (t *SofiaTCPTransport) GetAbility(ctx context.Context, name string) (map[string]any, error) {
	return t.doRPC(ctx, "GetAbility("+name+")", MsgAbilityGet, map[string]any{"Name": name})
}

func (t *SofiaTCPTransport) GetChannelAbility(ctx context.Context, name string) (map[string]any, error) {
	return t.doRPC(ctx, "GetChannelAbility("+name+")", MsgAbilityGet, map[string]any{"Name": name})
}

func (t *SofiaTCPTransport) GetConfig(ctx context.Context, name string) (map[string]any, error) {
	msgID := MsgConfigGet
	if name == "ChannelTitle" {
		msgID = MsgChannelTitleGet
	}
	resp, err := t.doRPC(ctx, "GetConfig("+name+")", msgID, map[string]any{"Name": name})
	if err != nil && msgID == MsgChannelTitleGet {
		// Some IPC firmwares serve ChannelTitle on standard MsgConfigGet (1042).
		return t.doRPC(ctx, "GetConfig("+name+")", MsgConfigGet, map[string]any{"Name": name})
	}
	return resp, err
}

func (t *SofiaTCPTransport) SetConfig(ctx context.Context, name string, payload any) (map[string]any, error) {
	msgID := MsgConfigSet
	if name == "ChannelTitle" {
		msgID = MsgChannelTitleSet
	}
	req := map[string]any{
		"Name": name,
		name:   payload,
	}
	resp, err := t.doRPC(ctx, "SetConfig("+name+")", msgID, req)
	if err != nil && msgID == MsgChannelTitleSet {
		return t.doRPC(ctx, "SetConfig("+name+")", MsgConfigSet, req)
	}
	return resp, err
}

func (t *SofiaTCPTransport) OpDev(ctx context.Context, name string, param any) (map[string]any, error) {
	msgID := MsgSysManagerReq
	switch name {
	case "OPFileQuery":
		msgID = MsgFileQueryReq
	case "OPTimeQuery", "OPTimeSetting", "OPUTCTimeSetting":
		msgID = MsgTimeQueryReq
	}
	req := map[string]any{"Name": name}
	if param != nil {
		req[name] = param
	}
	resp, err := t.doRPC(ctx, "OpDev("+name+")", msgID, req)
	if err != nil && (name == "OPTimeSetting" || name == "OPUTCTimeSetting") {
		// Fallback: several Sofia IPCs handle OPTimeSetting under 1450 instead of 1452.
		return t.doRPC(ctx, "OpDev("+name+")", MsgSysManagerReq, req)
	}
	return resp, err
}

func (t *SofiaTCPTransport) PTZ(ctx context.Context, param any) (map[string]any, error) {
	return t.doRPC(ctx, "OPPTZControl", MsgPTZReq, map[string]any{
		"Name":         "OPPTZControl",
		"OPPTZControl": param,
	})
}

func (t *SofiaTCPTransport) ModifyPassword(ctx context.Context, user, oldPass, newPass string) error {
	_, err := t.doRPC(ctx, "ModifyPassword", MsgModifyPswReq, map[string]any{
		"EncryptType": "MD5",
		"NewPassWord": MD5_8(newPass),
		"PassWord":    MD5_8(oldPass),
		"UserName":    user,
	})
	return err
}

// Snapshot requests a JPEG frame via Sofia OPSnap (1560).
func (t *SofiaTCPTransport) Snapshot(ctx context.Context, channel int) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil || t.loginInfo == nil {
		if _, err := t.loginLocked(ctx); err != nil {
			return nil, err
		}
	}
	t.setDeadlineLocked(ctx)
	req := map[string]any{
		"Name": "OPSnap",
		"OPSnap": map[string]any{
			"Channel": channel,
			"PicType": 0,
		},
		"SessionID": t.sessHex,
	}
	rawJSON, _ := json.Marshal(req)
	pkt := encodeSofiaPacket(t.sessionID, t.seq, MsgSnapReq, append(rawJSON, '\n', 0x00))
	t.seq++
	if _, err := t.conn.Write(pkt); err != nil {
		return nil, err
	}

	var img bytes.Buffer
	for {
		hdr, err := readSofiaHeader(t.conn)
		if err != nil {
			return nil, err
		}
		if hdr.DataLength > maxSofiaPayloadBytes {
			return nil, fmt.Errorf("xiongmai snap payload too large (%d)", hdr.DataLength)
		}
		chunk := make([]byte, hdr.DataLength)
		if hdr.DataLength > 0 {
			if _, err := io.ReadFull(t.conn, chunk); err != nil {
				return nil, err
			}
		}
		if hdr.MessageID == MsgKeepAliveRsp {
			continue
		}
		img.Write(chunk)
		if hdr.TotalPacket <= 1 || int(hdr.CurPacket)+1 >= int(hdr.TotalPacket) {
			break
		}
	}
	data := img.Bytes()
	// Locate JPEG SOI (0xFF 0xD8) inside returned binary/JSON response.
	if idx := bytes.Index(data, []byte{0xFF, 0xD8, 0xFF}); idx >= 0 {
		return data[idx:], nil
	}
	return nil, fmt.Errorf("xiongmai sofia OPSnap did not return JPEG")
}

// DownloadRecordFile opens a dedicated secondary Sofia TCP data connection
// (Claim -> DownloadStart) and streams elementary video frames to w.
func (t *SofiaTCPTransport) DownloadRecordFile(ctx context.Context, w io.Writer, channel int, file RecordFile, start, end time.Time) error {
	info, err := t.Login(ctx)
	if err != nil {
		return err
	}

	beginStr := file.RawBegin
	endStr := file.RawEnd
	if !start.IsZero() && (file.BeginTime.IsZero() || start.After(file.BeginTime)) {
		beginStr = start.Format("2006-01-02 15:04:05")
	}
	if !end.IsZero() && (file.EndTime.IsZero() || end.Before(file.EndTime)) {
		endStr = end.Format("2006-01-02 15:04:05")
	}

	// Open a dedicated data socket bound to the authenticated SessionID.
	dialer := net.Dialer{Timeout: t.timeout}
	dataConn, err := dialer.DialContext(ctx, "tcp", t.addr())
	if err != nil {
		return err
	}
	defer dataConn.Close()

	claimParam := map[string]any{
		"Action": "Claim",
		"Parameter": map[string]any{
			"Channel":         channel,
			"FileName":        file.FileName,
			"PlayMode":        "ByName",
			"TransMode":       "TCP",
			"Value":           0,
			"IntelligentPlay": 0,
		},
		"StartTime": beginStr,
		"EndTime":   endStr,
	}
	claimBody, _ := json.Marshal(map[string]any{
		"Name":       "OPPlayBack",
		"OPPlayBack": claimParam,
		"SessionID":  info.SessionIDHex,
	})
	dl := time.Now().Add(90 * time.Second)
	if cdl, ok := ctx.Deadline(); ok && cdl.Before(dl) {
		dl = cdl
	}
	_ = dataConn.SetDeadline(dl)

	if _, err := dataConn.Write(encodeSofiaPacket(info.SessionIDNum, 0, MsgPlayBackClaim, append(claimBody, '\n', 0x00))); err != nil {
		return err
	}
	hdr, err := readSofiaHeader(dataConn)
	if err != nil {
		return err
	}
	rspBuf := make([]byte, hdr.DataLength)
	if hdr.DataLength > 0 {
		if _, err := io.ReadFull(dataConn, rspBuf); err != nil {
			return err
		}
	}
	var claimRsp map[string]any
	if json.Unmarshal(bytes.TrimRight(rspBuf, "\x00\r\n "), &claimRsp) == nil {
		if r := extractRet(claimRsp); r != 0 && !IsRetSuccess(r) {
			return &RetError{Op: "OPPlayBack.Claim", Ret: r}
		}
	}

	// Trigger DownloadStart on the control session.
	startParam := map[string]any{
		"Action": "DownloadStart",
		"Parameter": map[string]any{
			"Channel":   channel,
			"FileName":  file.FileName,
			"PlayMode":  "ByName",
			"TransMode": "TCP",
			"Value":     0,
		},
		"StartTime": beginStr,
		"EndTime":   endStr,
	}
	if _, err := t.doRPC(ctx, "OPPlayBack.DownloadStart", MsgPlayBackReq, map[string]any{
		"Name":       "OPPlayBack",
		"OPPlayBack": startParam,
	}); err != nil {
		return err
	}

	var wrote int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = dataConn.SetReadDeadline(time.Now().Add(15 * time.Second))
		dh, err := readSofiaHeader(dataConn)
		if err != nil {
			if errors.Is(err, io.EOF) && wrote > 0 {
				break
			}
			return err
		}
		if dh.DataLength > maxSofiaPayloadBytes {
			return fmt.Errorf("xiongmai playback chunk too large (%d)", dh.DataLength)
		}
		payload := make([]byte, dh.DataLength)
		if dh.DataLength > 0 {
			if _, err := io.ReadFull(dataConn, payload); err != nil {
				return err
			}
		}
		if dh.MessageID == MsgPlayBackEOF {
			break
		}
		if dh.MessageID == MsgPlayBackData && len(payload) > 0 {
			es := StripXMMediaFrames(payload)
			if len(es) > 0 {
				n, werr := w.Write(es)
				wrote += int64(n)
				if werr != nil {
					return werr
				}
			}
		}
	}
	if wrote == 0 {
		return fmt.Errorf("xiongmai sofia OPPlayBack returned 0 media bytes")
	}
	return nil
}

func (t *SofiaTCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil {
		return nil
	}
	if t.loginInfo != nil {
		_ = t.conn.SetDeadline(time.Now().Add(1 * time.Second))
		body, _ := json.Marshal(map[string]any{"Name": "", "SessionID": t.sessHex})
		_, _ = t.conn.Write(encodeSofiaPacket(t.sessionID, t.seq, MsgLogoutReq, append(body, '\n', 0x00)))
	}
	err := t.conn.Close()
	t.conn = nil
	t.loginInfo = nil
	return err
}

func stringFieldAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func intFieldAny(m map[string]any, keys ...string) int {
	for _, k := range keys {
		switch v := m[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

func boolFieldAny(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k].(bool); ok {
			return v
		}
	}
	return false
}
