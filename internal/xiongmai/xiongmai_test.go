package xiongmai

import (
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
