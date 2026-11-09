package xiongmai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMD5_8Vectors(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "tlJwpbo6"},
		{"admin", "6QNMIQGe"},
		{"123456", "nTBCS19C"},
	}
	for _, tc := range cases {
		if got := MD5_8(tc.in); got != tc.want {
			t.Fatalf("MD5_8(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHexIPRoundTrip(t *testing.T) {
	ip, err := HexIPToDecIP("0x8A4DA8C0")
	if err != nil {
		t.Fatalf("HexIPToDecIP: %v", err)
	}
	if ip != "192.168.77.138" {
		t.Fatalf("HexIPToDecIP = %q, want 192.168.77.138", ip)
	}
	hexStr, err := DecIPToHexIP("192.168.77.138")
	if err != nil {
		t.Fatalf("DecIPToHexIP: %v", err)
	}
	if hexStr != "0x8A4DA8C0" {
		t.Fatalf("DecIPToHexIP = %q, want 0x8A4DA8C0", hexStr)
	}
}

func TestAESZeroPadAndUnencryptedLargeJSONFallback(t *testing.T) {
	key := "0123456789ABCDEF"
	orig := `{"Name":"AVEnc.Encode","Ret":100,"SessionID":"0x12"}`
	enc, err := AESEncryptZeroPadB64([]byte(orig), key)
	if err != nil {
		t.Fatalf("AESEncryptZeroPadB64: %v", err)
	}
	dec, err := AESDecryptZeroPadB64([]byte(enc), key)
	if err != nil {
		t.Fatalf("AESDecryptZeroPadB64: %v", err)
	}
	if string(dec) != orig {
		t.Fatalf("decrypted = %q, want %q", string(dec), orig)
	}

	// Verify class.js:223 fallback when device returns plain JSON on huge tables.
	plainLarge := `{"Name":"Record","Ret":100,"Record":[{"Channel":0}]}`
	decPlain, err := AESDecryptZeroPadB64([]byte(plainLarge), key)
	if err != nil {
		t.Fatalf("AESDecryptZeroPadB64 plain fallback: %v", err)
	}
	if string(decPlain) != plainLarge {
		t.Fatalf("plain fallback = %q, want %q", string(decPlain), plainLarge)
	}
}

func TestStripXMMediaFramesAndCodecDetection(t *testing.T) {
	// Construct a synthetic XM 16-byte I-frame (00 00 01 FE) + H.265 VPS/SPS/IDR + 8-byte audio (00 00 01 FA) + 8-byte P-frame (00 00 01 FD).
	h265IDR := []byte{0x00, 0x00, 0x00, 0x01, 0x40, 0x01, 0xAA, 0xBB} // NAL type 32 = HEVC VPS
	h265P := []byte{0x00, 0x00, 0x00, 0x01, 0x02, 0x01, 0xCC}

	var pkt bytes.Buffer
	iHdr := make([]byte, 16)
	copy(iHdr[:4], []byte{0x00, 0x00, 0x01, 0xFE})
	binary.LittleEndian.PutUint32(iHdr[12:16], uint32(len(h265IDR)))
	pkt.Write(iHdr)
	pkt.Write(h265IDR)

	aHdr := make([]byte, 8)
	copy(aHdr[:4], []byte{0x00, 0x00, 0x01, 0xFA})
	binary.LittleEndian.PutUint16(aHdr[6:8], 4)
	pkt.Write(aHdr)
	pkt.Write([]byte{0x11, 0x22, 0x33, 0x44}) // audio bytes should be stripped

	pHdr := make([]byte, 8)
	copy(pHdr[:4], []byte{0x00, 0x00, 0x01, 0xFD})
	binary.LittleEndian.PutUint16(pHdr[6:8], uint16(len(h265P)))
	pkt.Write(pHdr)
	pkt.Write(h265P)

	got := StripXMMediaFrames(pkt.Bytes())
	want := append(append([]byte{}, h265IDR...), h265P...)
	if !bytes.Equal(got, want) {
		t.Fatalf("StripXMMediaFrames = %x, want %x", got, want)
	}
	if codec := DetectAnnexBCodec(got); codec != "hevc" {
		t.Fatalf("DetectAnnexBCodec = %q, want hevc", codec)
	}
}

// startMockSofiaIPC spins up an in-memory Sofia TCP 34567 mock IPC server
// (with array-typed ExtraFormat, matching real Vitacam/XMEye IPCs on 8.4).
func startMockSofiaIPC(t *testing.T, user, pass string, rejectRSA205 bool) (host string, port int, closeFn func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)

	var mu sync.Mutex
	encodeTable := []map[string]any{
		{
			"EncodeStaticParam": []map[string]any{{"Level": 41, "Profile": 2}},
			"MainFormat": map[string]any{
				"AudioEnable": true,
				"VideoEnable": true,
				"Audio":       map[string]any{"Compression": "G711_ALAW"},
				"Video": map[string]any{
					"BitRate":        2048,
					"BitRateControl": "CBR",
					"Compression":    "H.264",
					"FPS":            25,
					"GOP":            2,
					"Resolution":     "1080P",
				},
			},
			"ExtraFormat": []map[string]any{
				{
					"AudioEnable": false,
					"VideoEnable": true,
					"Video": map[string]any{
						"BitRate":        384,
						"BitRateControl": "VBR",
						"Compression":    "H.264",
						"FPS":            15,
						"GOP":            2,
						"Resolution":     "D1",
					},
				},
			},
		},
	}
	titles := []string{"Cam Yen 03"}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				for {
					hdr, err := readSofiaHeader(c)
					if err != nil {
						return
					}
					body := make([]byte, hdr.DataLength)
					if hdr.DataLength > 0 {
						if _, err := io.ReadFull(c, body); err != nil {
							return
						}
					}
					var req map[string]any
					_ = json.Unmarshal(bytes.TrimRight(body, "\x00\r\n "), &req)

					var resp map[string]any
					rspMsg := hdr.MessageID + 1

					switch hdr.MessageID {
					case MsgLoginReq:
						if rejectRSA205 {
							resp = map[string]any{"Ret": RetRSARequired, "SessionID": "0x0000000E"}
						} else if req["UserName"] == user && req["PassWord"] == MD5_8(pass) {
							resp = map[string]any{
								"Ret":         RetOK,
								"SessionID":   "0x0000002A",
								"ChannelNum":  1,
								"DeviceType ": "IPC",
							}
						} else {
							resp = map[string]any{"Ret": RetBadPassword}
						}
					case MsgSysInfoReq:
						name, _ := req["Name"].(string)
						if name == "StorageInfo" {
							resp = map[string]any{
								"Name": "StorageInfo",
								"Ret":  RetOK,
								"StorageInfo": []map[string]any{
									{
										"PlysicalNo": 0,
										"Partition": []map[string]any{
											{"TotalSpace": "0x0000EC00", "RemainSpace": "0x00004000", "Status": 0},
										},
									},
								},
							}
						} else {
							resp = map[string]any{
								"Name": "SystemInfo",
								"Ret":  RetOK,
								"SystemInfo": map[string]any{
									"SerialNo":       "XM-IPC-SN8899",
									"VideoInChannel": 1,
									"DigChannel":     0,
								},
							}
						}
					case MsgConfigGet, MsgChannelTitleGet:
						mu.Lock()
						name, _ := req["Name"].(string)
						switch name {
						case "AVEnc.Encode":
							raw, _ := json.Marshal(encodeTable)
							var cloned any
							_ = json.Unmarshal(raw, &cloned)
							resp = map[string]any{"Name": name, "Ret": RetOK, name: cloned}
						case "ChannelTitle":
							resp = map[string]any{"Name": name, "Ret": RetOK, name: append([]string{}, titles...)}
						case "NetWork.NetCommon":
							resp = map[string]any{
								"Name": name, "Ret": RetOK,
								name: map[string]any{
									"HostIP":  "0x224DA8C0", // 192.168.77.34
									"Submask": "0x00FFFFFF",
									"GateWay": "0x014DA8C0",
									"MAC":     "d4:b7:61:7b:e5:06",
								},
							}
						default:
							resp = map[string]any{"Name": name, "Ret": RetConfigNotExist}
						}
						mu.Unlock()
					case MsgConfigSet, MsgChannelTitleSet:
						mu.Lock()
						name, _ := req["Name"].(string)
						if name == "AVEnc.Encode" {
							raw, _ := json.Marshal(req[name])
							_ = json.Unmarshal(raw, &encodeTable)
						} else if name == "ChannelTitle" {
							raw, _ := json.Marshal(req[name])
							_ = json.Unmarshal(raw, &titles)
						}
						resp = map[string]any{"Name": name, "Ret": RetOK}
						mu.Unlock()
					case MsgFileQueryReq:
						resp = map[string]any{
							"Name": "OPFileQuery",
							"Ret":  RetOK,
							"OPFileQuery": []map[string]any{
								{
									"BeginTime":  "2026-11-08 18:00:00",
									"EndTime":    "2026-11-08 18:05:00",
									"FileName":   "/idea0/2026-11-08/001/18.00.00-18.05.00[R][@0][0].h264",
									"FileLength": "0x00008000",
								},
							},
						}
					case MsgSnapReq:
						// Multi-fragment JPEG response test
						fakeJPG := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0xFF, 0xD9}
						p1 := encodeSofiaPacket(0x2A, hdr.SequenceNumber, MsgSnapRsp, fakeJPG[:6])
						p1[12] = 2
						p1[13] = 0
						p2 := encodeSofiaPacket(0x2A, hdr.SequenceNumber, MsgSnapRsp, fakeJPG[6:])
						p2[12] = 2
						p2[13] = 1
						_, _ = c.Write(p1)
						_, _ = c.Write(p2)
						continue
					default:
						resp = map[string]any{"Ret": RetOK}
					}

					rawOut, _ := json.Marshal(resp)
					_, _ = c.Write(encodeSofiaPacket(0x2A, hdr.SequenceNumber, rspMsg, append(rawOut, '\n', 0x00)))
				}
			}(conn)
		}
	}()

	return tcpAddr.IP.String(), tcpAddr.Port, func() { _ = ln.Close() }
}

func TestSofiaTCPClientEndToEnd(t *testing.T) {
	host, port, closeFn := startMockSofiaIPC(t, "admin", "secret123", false)
	defer closeFn()

	ctx := context.Background()
	cl := NewWithTransport(host, port, "admin", "secret123", 5*time.Second,
		NewSofiaTCPTransport(host, port, "admin", "secret123", 5*time.Second))
	defer cl.Close()

	sn, err := cl.GetSerialNumber(ctx)
	if err != nil || sn != "XM-IPC-SN8899" {
		t.Fatalf("GetSerialNumber = %q, err=%v", sn, err)
	}

	streams, err := cl.ProbeAll(ctx)
	if err != nil || len(streams) != 2 {
		t.Fatalf("ProbeAll = %d streams, err=%v", len(streams), err)
	}
	if streams[0].Width != 1920 || streams[0].Height != 1080 || streams[0].GOP != 50 || streams[0].Name != "Cam Yen 03" {
		t.Fatalf("unexpected main stream: %+v", streams[0])
	}
	if streams[1].Width != 704 || streams[1].Height != 576 || streams[1].FPS != 15 {
		t.Fatalf("unexpected sub stream: %+v", streams[1])
	}

	// Mutate Codec -> H.265, Resolution -> 4M (2560x1440), FPS -> 20, GOP -> 40 frames (2s), Bitrate -> 1536 VBR
	if err := cl.SetCodec(ctx, 0, 0, "H.265", "Main"); err != nil {
		t.Fatalf("SetCodec: %v", err)
	}
	if err := cl.SetResolution(ctx, 0, 0, 2560, 1440); err != nil {
		t.Fatalf("SetResolution: %v", err)
	}
	if err := cl.SetFPS(ctx, 0, 0, 20); err != nil {
		t.Fatalf("SetFPS: %v", err)
	}
	if err := cl.SetGOP(ctx, 0, 0, 60); err != nil {
		t.Fatalf("SetGOP: %v", err)
	}
	if err := cl.SetBitrate(ctx, 0, 0, 1536, "VBR"); err != nil {
		t.Fatalf("SetBitrate: %v", err)
	}

	after, err := cl.GetStreamInfo(ctx, 0, 0)
	if err != nil {
		t.Fatalf("GetStreamInfo after mutate: %v", err)
	}
	if after.Compression != "H.265" || after.Width != 2560 || after.Height != 1440 || after.FPS != 20 || after.GOP != 60 || after.BitRate != 1536 || after.BitRateControl != "VBR" {
		t.Fatalf("read-back mismatch: %+v", after)
	}

	// Verify multi-packet OPSnap JPEG reassembly
	jpg, err := cl.Snapshot(ctx, 0, 0)
	if err != nil || !bytes.HasPrefix(jpg, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatalf("Snapshot failed: len=%d err=%v", len(jpg), err)
	}

	// Verify Storage, Network HexIP, and Recordings
	st, err := cl.GetStorageInfo(ctx)
	if err != nil || len(st) != 1 || st[0].Details[0].TotalBytes != 0xEC00*1024*1024 {
		t.Fatalf("GetStorageInfo = %+v, err=%v", st, err)
	}
	netCfg, err := cl.GetNetworkConfig(ctx)
	if err != nil || netCfg.Interfaces["eth0"]["IPAddress"] != "192.168.77.34" {
		t.Fatalf("GetNetworkConfig = %+v, err=%v", netCfg, err)
	}
	recs, err := cl.FindRecordings(ctx, 0, time.Date(2026, 11, 8, 0, 0, 0, 0, time.Local), time.Date(2026, 11, 8, 23, 59, 59, 0, time.Local))
	if err != nil || len(recs) != 1 || recs[0].Duration != 300 {
		t.Fatalf("FindRecordings = %+v, err=%v", recs, err)
	}
}

func TestCGITransportRSAAndAESWithBypassHVR(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	pubStr := fmt.Sprintf("%X,%06X", privKey.PublicKey.N, privKey.PublicKey.E)
	const wantUser = "admin"
	const wantPass = "xmHvrPass!"
	const salt = "Ho9mICnr"

	var sessionAESKey string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.URL.Path, "/cgi-bin/login.cgi") {
			var plain map[string]any
			if json.Unmarshal(body, &plain) == nil {
				switch plain["Name"] {
				case "GetPreLoginInfo":
					_ = json.NewEncoder(w).Encode(map[string]any{"Ret": 100, "Language": "English", "TCPPort": 34567})
					return
				case "GetSalt":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"Ret":                 100,
						"Salt":                salt,
						"PublicKey":           pubStr,
						"LoginEncryptionType": "RSA/MD5_8",
						"DataEncryptionType":  "AES",
					})
					return
				case "Login":
					dec := func(hexCipher string) string {
						ct, _ := hex.DecodeString(hexCipher)
						pt, err := rsa.DecryptPKCS1v15(rand.Reader, privKey, ct)
						if err != nil {
							t.Errorf("decrypt RSA: %v", err)
						}
						return string(pt)
					}
					gotUser := dec(plain["User"].(string))
					gotSign := dec(plain["Sign"].(string))
					gotDTAK := dec(plain["DTAK"].(string))
					if gotUser != wantUser || gotSign != salt+HexMD5Lower(wantPass) {
						_ = json.NewEncoder(w).Encode(map[string]any{"Ret": RetBadPassword})
						return
					}
					mu.Lock()
					sessionAESKey = gotDTAK
					mu.Unlock()
					_ = json.NewEncoder(w).Encode(map[string]any{
						"Ret":        100,
						"SessionID":  "0x00000012",
						"DeviceType": "HVR",
					})
					return
				}
			}
			// Encrypted Logout
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Ret":100}`))
			return
		}

		// Decrypt AES-128-CBC ZeroPad request
		mu.Lock()
		key := sessionAESKey
		mu.Unlock()
		pt, err := AESDecryptZeroPadB64(body, key)
		if err != nil {
			t.Fatalf("server decrypt CGI %s: %v", r.URL.Path, err)
		}
		var req map[string]any
		_ = json.Unmarshal(pt, &req)
		name, _ := req["Name"].(string)

		var resp map[string]any
		switch name {
		case "SystemInfo":
			resp = map[string]any{
				"Name": "SystemInfo", "Ret": 100,
				"SystemInfo": map[string]any{
					"SerialNo":       "XM-HVR-77138",
					"VideoInChannel": 0,
					"DigChannel":     2,
				},
			}
		case "NetWork.RemoteDeviceV3":
			resp = map[string]any{
				"Name": name, "Ret": 100,
				name: []map[string]any{
					{
						"Enable":       true,
						"SingleConnId": 1,
						"Decoder": []map[string]any{
							{"IPAddress": "192.168.77.34", "Port": 34567, "UserName": "admin"},
						},
					},
				},
			}
		case "ChannelTitle":
			resp = map[string]any{"Name": name, "Ret": 100, name: []string{"Vitacam 34", "Cam 2"}}
		case "bypass@AVEnc.Encode.[1]":
			resp = map[string]any{
				"Name": name, "Ret": 100,
				name: []map[string]any{
					{
						"MainFormat": map[string]any{
							"AudioEnable": true,
							"Audio":       map[string]any{"Compression": "AAC"},
							"Video":       map[string]any{"Compression": "H.265", "Resolution": "4M", "FPS": 25, "GOP": 2, "BitRate": 2048, "BitRateControl": "VBR"},
						},
					},
				},
			}
		default:
			resp = map[string]any{"Name": name, "Ret": RetConfigNotExist}
		}

		rawResp, _ := json.Marshal(resp)
		encB64, _ := AESEncryptZeroPadB64(rawResp, key)
		_, _ = w.Write([]byte(encB64))
	}))
	defer srv.Close()

	// Verify ProbePreLoginCGI discovery helper
	pre, err := ProbePreLoginCGI(context.Background(), srv.URL, 2*time.Second)
	if err != nil || pre.TCPPort != 34567 {
		t.Fatalf("ProbePreLoginCGI = %+v, err=%v", pre, err)
	}

	u := srv.Listener.Addr().(*net.TCPAddr)
	cl := NewWithTransport(u.IP.String(), u.Port, wantUser, wantPass, 5*time.Second,
		NewCGITransport(u.IP.String(), u.Port, wantUser, wantPass, 5*time.Second))
	defer cl.Close()

	ctx := context.Background()
	sn, err := cl.GetSerialNumber(ctx)
	if err != nil || sn != "XM-HVR-77138" {
		t.Fatalf("GetSerialNumber = %q, err=%v", sn, err)
	}

	// Channel 1 (0-based) on HVR with VideoInChannel=0, DigChannel=2 must use bypass@AVEnc.Encode.[1]
	si, err := cl.GetStreamInfo(ctx, 1, 0)
	if err != nil || si.Width != 2560 || si.Height != 1440 || si.AudioCodec != "AAC" {
		t.Fatalf("bypass HVR GetStreamInfo = %+v, err=%v", si, err)
	}

	rems, err := cl.GetRemoteDevices(ctx)
	if err != nil || len(rems) != 1 || rems[0].Address != "192.168.77.34" || rems[0].Channel != 0 {
		t.Fatalf("GetRemoteDevices = %+v, err=%v", rems, err)
	}

	// Verify wrong password yields sentinel ErrAuthFailed without panicking
	badCl := NewWithTransport(u.IP.String(), u.Port, wantUser, "wrongpass", 5*time.Second,
		NewCGITransport(u.IP.String(), u.Port, wantUser, "wrongpass", 5*time.Second))
	defer badCl.Close()
	if _, err := badCl.GetSerialNumber(ctx); !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("expected ErrAuthFailed on bad password, got %v", err)
	}
}

func TestDecryptRandomUserInfoAndExUserMapPassword(t *testing.T) {
	// Real vector captured from NBD80S16S-KL (192.168.77.138 on inut_208_4)
	sn := "c1beba4793bccbdapnx9"
	b64Info := "WkXIjh88NHCtm6VEwc90GNTswQ+HmDddpipZekdf/YQ="
	key, ok := DeriveRandomUserAESKey(sn)
	if !ok || key != "a4793b1beba493bc" {
		t.Fatalf("DeriveRandomUserAESKey = (%q, %v), want (a4793b1beba493bc, true)", key, ok)
	}
	user, pwd, err := DecryptRandomUserInfo(sn, b64Info)
	if err != nil {
		t.Fatalf("DecryptRandomUserInfo: %v", err)
	}
	if user != "admin1" || pwd != "101" {
		t.Fatalf("DecryptRandomUserInfo = (%q, %q), want (admin1, 101)", user, pwd)
	}

	if got := MD5_8("Inut12345"); got != "IgCDPz7Q" {
		t.Fatalf("MD5_8(Inut12345) = %q, want IgCDPz7Q", got)
	}
	if got := EncodeExUserMapPassword("Inut12345"); got != "00111W51dDEyMzQS" {
		t.Fatalf("EncodeExUserMapPassword(Inut12345) = %q, want 00111W51dDEyMzQS", got)
	}
}

func TestUcHttpdLeadingSpaceHeaderAndRandomUserResolution(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	pubKeyStr := fmt.Sprintf("%s,%06X", strings.ToUpper(privKey.PublicKey.N.Text(16)), privKey.PublicKey.E)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	const salt = "OuIEvLHp"
	const sn = "c1beba4793bccbdapnx9"
	const realUser = "admin1"
	const realPass = "Inut12345"

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(3 * time.Second))
				br := bufio.NewReader(c)
				var hdrLines []string
				contentLen := 0
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					hdrLines = append(hdrLines, line)
					if strings.HasPrefix(strings.ToLower(line), "content-length:") {
						_, _ = fmt.Sscanf(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), "%d", &contentLen)
					}
					if line == "\r\n" || line == "\n" {
						break
					}
				}
				rawReq := strings.Join(hdrLines, "")
				bodyBytes := make([]byte, contentLen)
				if contentLen > 0 {
					_, _ = io.ReadFull(br, bodyBytes)
				}
				body := strings.TrimSpace(string(bodyBytes))
				var respBody string
				switch {
				case strings.Contains(rawReq, "/cgi-bin/resetpwd.cgi") && strings.Contains(body, "GetDevInfo"):
					respBody = `{"Name":"GetDevInfo","Ret":100,"GetDevInfo":{"SerialNo":"` + sn + `"}}`
				case strings.Contains(rawReq, "/cgi-bin/resetpwd.cgi") && strings.Contains(body, "GetRandomUser"):
					respBody = `{"Name":"GetRandomUser","Ret":100,"GetRandomUser":{"Info":"WkXIjh88NHCtm6VEwc90GNTswQ+HmDddpipZekdf/YQ="}}`
				case strings.Contains(rawReq, "/cgi-bin/login.cgi") && strings.Contains(body, "GetSalt"):
					respBody = fmt.Sprintf(`{"Ret":100,"Salt":%q,"PublicKey":%q,"LoginEncryptionType":"RSA/MD5_8","DataEncryptionType":"AES"}`, salt, pubKeyStr)
				case strings.Contains(rawReq, "/cgi-bin/login.cgi") && strings.Contains(body, "Login"):
					var req map[string]any
					_ = json.Unmarshal([]byte(body), &req)
					uHex, _ := req["User"].(string)
					sHex, _ := req["Sign"].(string)
					uRaw, _ := hex.DecodeString(uHex)
					sRaw, _ := hex.DecodeString(sHex)
					uDec, _ := rsa.DecryptPKCS1v15(rand.Reader, privKey, uRaw)
					sDec, _ := rsa.DecryptPKCS1v15(rand.Reader, privKey, sRaw)
					if string(uDec) == realUser && string(sDec) == salt+HexMD5Lower(realPass) {
						respBody = `{"Ret":100,"SessionID":"0x00000024","ChannelNum":16,"DeviceType ":"HVR","AliveInterval":21}`
					} else {
						respBody = `{"Ret":106,"Tip":"User error"}`
					}
				default:
					respBody = `{"Ret":100}`
				}
				// Deliberately emit uc-httpd 1.0.0's malformed leading space on the first header line!
				wire := fmt.Sprintf("HTTP/1.0 200 OK\r\n Content-type: text/plain\r\nAccess-Control-Allow-Origin: *\r\nContent-Length: %d\r\n\r\n%s", len(respBody), respBody)
				_, _ = c.Write([]byte(wire))
			}(conn)
		}
	}()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	// Configure username="admin" — CGITransport must auto-resolve "admin" -> "admin1" via resetpwd.cgi
	// AND tolerate uc-httpd 1.0.0's leading space on " Content-type: text/plain"!
	tr := NewCGITransport(tcpAddr.IP.String(), tcpAddr.Port, "admin", realPass, 3*time.Second)
	info, err := tr.Login(context.Background())
	if err != nil {
		t.Fatalf("CGITransport.Login with uc-httpd header + admin->admin1 auto-resolve failed: %v", err)
	}
	if info.ChannelNum != 16 || info.SessionIDHex != "0x00000024" {
		t.Fatalf("unexpected LoginInfo: %+v", info)
	}
}

func TestXMStreamExtractorChunkSpanningAndTimeWindowTrim(t *testing.T) {
	loc := time.Local
	tPre := time.Date(2026, 11, 9, 11, 29, 58, 0, loc)
	tIn := time.Date(2026, 11, 9, 11, 30, 2, 0, loc)
	tPost := time.Date(2026, 11, 9, 11, 35, 3, 0, loc)

	// Build 3 GOPs where the in-window IDR payload deliberately embeds false 00 00 01 FA
	// CABAC bytes and is split across tiny 11-byte chunks.
	idrPre := []byte{0x00, 0x00, 0x00, 0x01, 0x67, 0x4D, 0x00, 0x2A}
	idrIn := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0x00, 0x00, 0x01, 0xFA, 0xBB, 0xCC, 0xDD}
	pIn := []byte{0x00, 0x00, 0x00, 0x01, 0x41, 0x9A, 0x77}
	idrPost := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xEE, 0xFF}

	makeIFrame := func(tm time.Time, fps byte, nal []byte) []byte {
		hdr := make([]byte, 16)
		copy(hdr[:4], []byte{0x00, 0x00, 0x01, 0xFC})
		hdr[4] = 0x02
		hdr[5] = fps
		binary.LittleEndian.PutUint32(hdr[8:12], EncodeXMTimestamp(tm))
		binary.LittleEndian.PutUint32(hdr[12:16], uint32(len(nal)))
		return append(hdr, nal...)
	}
	makePFrame := func(nal []byte) []byte {
		hdr := make([]byte, 8)
		copy(hdr[:4], []byte{0x00, 0x00, 0x01, 0xFD})
		binary.LittleEndian.PutUint16(hdr[6:8], uint16(len(nal)))
		return append(hdr, nal...)
	}
	makeAudio := func(aud []byte) []byte {
		hdr := make([]byte, 8)
		copy(hdr[:4], []byte{0x00, 0x00, 0x01, 0xFA})
		binary.LittleEndian.PutUint16(hdr[6:8], uint16(len(aud)))
		return append(hdr, aud...)
	}

	var wire bytes.Buffer
	wire.Write(makeIFrame(tPre, 20, idrPre))
	wire.Write(makeAudio([]byte{1, 2, 3, 4, 5}))
	wire.Write(makeIFrame(tIn, 20, idrIn))
	wire.Write(makePFrame(pIn))
	wire.Write(makeIFrame(tPost, 20, idrPost))

	reqStart := time.Date(2026, 11, 9, 11, 30, 0, 0, loc)
	reqEnd := time.Date(2026, 11, 9, 11, 35, 0, 0, loc)

	var out bytes.Buffer
	ext := NewXMStreamExtractor(&out, reqStart, reqEnd)
	raw := wire.Bytes()
	stoppedEarly := false
	for off := 0; off < len(raw); off += 11 {
		endOff := off + 11
		if endOff > len(raw) {
			endOff = len(raw)
		}
		if err := ext.Feed(raw[off:endOff]); err != nil {
			if errors.Is(err, ErrXMStopTimeReached) {
				stoppedEarly = true
				break
			}
			t.Fatalf("ext.Feed err: %v", err)
		}
	}
	_ = ext.Finish()
	if !stoppedEarly {
		t.Fatalf("expected ErrXMStopTimeReached on post-window I-frame")
	}
	if ext.ObservedFPS() != 20 {
		t.Fatalf("ObservedFPS = %d, want 20", ext.ObservedFPS())
	}
	// Must contain pre-start GOP (within 2s before reqStart) + in-window IDR (including embedded 00 00 01 FA!) + P-frame, and NOT idrPost!
	want := append(append(append([]byte{}, idrPre...), idrIn...), pIn...)
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatalf("extracted ES = %x, want %x", out.Bytes(), want)
	}
}

func TestHexSingleConnIdRemoteDevicesAndNVRHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s := string(body)
		switch {
		case strings.Contains(r.URL.Path, "resetpwd.cgi"):
			_, _ = w.Write([]byte(`{"Ret":102}`))
		case strings.Contains(r.URL.Path, "login.cgi") && strings.Contains(s, "GetSalt"):
			_, _ = w.Write([]byte(`{"Ret":100,"Salt":"12345678","LoginEncryptionType":"MD5_8","DataEncryptionType":"NONE"}`))
		case strings.Contains(r.URL.Path, "login.cgi") && strings.Contains(s, "Login"):
			_, _ = w.Write([]byte(`{"Ret":100,"SessionID":"0x00000010","ChannelNum":4,"DeviceType":"HVR"}`))
		case strings.Contains(r.URL.Path, "getinfo.cgi") && strings.Contains(s, "SystemInfo"):
			_, _ = w.Write([]byte(`{"Ret":100,"SystemInfo":{"SerialNo":"SN16","HardWare":"NBD80S16S-KL","VideoInChannel":0,"DigChannel":4,"DeviceRunTime":"0x00000078"}}`))
		case strings.Contains(r.URL.Path, "getconfig.cgi") && strings.Contains(s, "ChannelTitle"):
			_, _ = w.Write([]byte(`{"Ret":100,"ChannelTitle":["D01","D02","D03","D04"]}`))
		case strings.Contains(r.URL.Path, "getconfig.cgi") && strings.Contains(s, "NetWork.ChnStatus"):
			_, _ = w.Write([]byte(`{"Ret":100,"NetWork.ChnStatus":[{"ChnName":"D01","CurRes":"1080P/D1","Status":"Connected"},{"ChnName":"D02","CurRes":"4M/VGA","Status":"Connected"},{"ChnName":"D03","CurRes":"Unknown","Status":"NoConfig"},{"ChnName":"D04","CurRes":"Unknown","Status":"NoConfig"}]}`))
		case strings.Contains(r.URL.Path, "getconfig.cgi") && strings.Contains(s, "NetWork.RemoteDeviceV3"):
			_, _ = w.Write([]byte(`{"Ret":100,"NetWork.RemoteDeviceV3":[
				{"Enable":true,"SingleConnId":"0x00000001","Decoder":[{"IPAddress":"192.168.77.71","Port":554}]},
				{"Enable":true,"SingleConnId":"0x00000004","Decoder":[{"IPAddress":"192.168.77.71","Port":554},{"IPAddress":"192.168.77.218","Port":554},{"IPAddress":"192.168.77.194","Port":34567},{"IPAddress":"192.168.77.146","Port":554}]},
				{"Enable":true,"SingleConnId":"0x00000000","Decoder":[{"IPAddress":"192.168.77.71","Port":554}]},
				{"Enable":true,"SingleConnId":"0x00000000","Decoder":[{"IPAddress":"192.168.77.71","Port":554}]}
			]}`))
		case strings.Contains(r.URL.Path, "getconfig.cgi") && strings.Contains(s, "AVEnc.Encode"):
			_, _ = w.Write([]byte(`{"Ret":607}`))
		case strings.Contains(r.URL.Path, "getconfig.cgi") && strings.Contains(s, "Record"):
			mask7 := `[["0x00000007","0x0"],["0x00000007","0x0"],["0x00000007","0x0"],["0x00000007","0x0"],["0x00000007","0x0"],["0x00000007","0x0"],["0x00000007","0x0"]]`
			_, _ = w.Write([]byte(`{"Ret":100,"Record":[{"RecordMode":"ConfigRecord","Mask":` + mask7 + `},{"RecordMode":"ClosedRecord","Mask":` + mask7 + `}]}`))
		case strings.Contains(r.URL.Path, "setconfig.cgi"):
			_, _ = w.Write([]byte(`{"Ret":100}`))
		default:
			_, _ = w.Write([]byte(`{"Ret":100}`))
		}
	}))
	defer srv.Close()
	u := srv.Listener.Addr().(*net.TCPAddr)
	cl := NewWithTransport(u.IP.String(), u.Port, "admin", "pass", 5*time.Second,
		NewCGITransport(u.IP.String(), u.Port, "admin", "pass", 5*time.Second))
	defer cl.Close()

	ctx := context.Background()
	rems, err := cl.GetRemoteDevices(ctx)
	if err != nil || len(rems) != 2 {
		t.Fatalf("GetRemoteDevices = %+v, err=%v (want 2 configured channels)", rems, err)
	}
	if rems[0].Channel != 0 || rems[0].Address != "192.168.77.71" {
		t.Fatalf("rems[0] = %+v, want ch=0 192.168.77.71", rems[0])
	}
	if rems[1].Channel != 1 || rems[1].Address != "192.168.77.146" {
		t.Fatalf("rems[1] = %+v, want ch=1 192.168.77.146 (SingleConnId=0x4 -> Decoder[3])", rems[1])
	}

	// Verify ChnStatus Probe fallback when AVEnc.Encode returns Ret=607
	si, err := cl.GetStreamInfo(ctx, 1, 0)
	if err != nil || si.Width != 2560 || si.Height != 1440 {
		t.Fatalf("ChnStatus GetStreamInfo fallback = %+v, err=%v", si, err)
	}

	// Verify NVRHealthConfig (GetUptime + GetRecordState + EnableTimingRecord)
	up, err := cl.GetUptime(ctx)
	if err != nil || up != 120*time.Minute {
		t.Fatalf("GetUptime = %v, err=%v (want 2h0m)", up, err)
	}
	states, err := cl.GetRecordState(ctx, 2)
	if err != nil || len(states) != 2 || !states[0].Enable || !states[0].Timing24x7 || states[1].Enable {
		t.Fatalf("GetRecordState = %+v, err=%v", states, err)
	}
	if err := cl.EnableTimingRecord(ctx, []int{1}); err != nil {
		t.Fatalf("EnableTimingRecord err: %v", err)
	}
}
