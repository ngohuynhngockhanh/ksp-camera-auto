package raysharp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockRaysharpDevice struct {
	mu       sync.Mutex
	server   *httptest.Server
	host     string
	port     int
	username string
	password string
	csrf     string
	cookie   string
	mainCh1  map[string]any
	subCh1   map[string]any
	osdCh1   map[string]any
	aiHvCh1  map[string]any
	lnkHvCh1 map[string]any
	seq      int
}

func startMockRaysharp(t *testing.T, user, pass string) *mockRaysharpDevice {
	t.Helper()
	m := &mockRaysharpDevice{
		username: user,
		password: pass,
		csrf:     "cf7d2face586815f56800f8636a5784e8652ffcd7d4f97d05bc15c7cf75b060b",
		cookie:   "session_80=d7243f14ca48333eaf24a27a30262c5cd5bb8b2020b02c4bbaf1b4bd643c629f",
		mainCh1: map[string]any{
			"resolution":         "2880 x 1620",
			"fps":                25,
			"bitrate":            3072,
			"bitrate_mode":       "Predefined",
			"custom_bitrate":     3072,
			"video_encode_type":  "H.265",
			"video_encode_level": "MainProfile",
			"bitrate_control":    "CBR",
			"audio":              true,
			"i_frame_interval":   50,
		},
		subCh1: map[string]any{
			"resolution":         "1280 x 720",
			"fps":                25,
			"bitrate":            1024,
			"bitrate_mode":       "Predefined",
			"custom_bitrate":     1024,
			"video_encode_type":  "H.265",
			"video_encode_level": "MainProfile",
			"bitrate_control":    "CBR",
			"audio":              true,
			"i_frame_interval":   60,
		},
		osdCh1: map[string]any{
			"status": "Online",
			"name": map[string]any{
				"show": true,
				"text": "Camera 01",
				"pos":  map[string]any{"x": 290, "y": 0},
			},
			"alarm": map[string]any{
				"show": false,
				"text": "",
				"pos":  map[string]any{"x": 0, "y": 50},
			},
		},
		aiHvCh1: map[string]any{
			"switch":         true,
			"sensitivity":    60,
			"detection_mode": "MotionMode",
			"detection_type": []string{"Pedestrian", "Motor Vehicle"},
		},
		lnkHvCh1: map[string]any{
			"record_enable":  true,
			"post_recording": "30",
			"http_listening": true,
			"deterrence":     false,
		},
		seq: 100,
	}

	m.server = httptest.NewServer(http.HandlerFunc(m.serveHTTP))
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(m.server.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	m.host = host
	m.port = port
	return m
}

func (m *mockRaysharpDevice) Close() {
	m.server.Close()
}

func (m *mockRaysharpDevice) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path

	if path == "/API/Login/Range" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"success","data":{"device_type":"N10256N","http_api_version":"V1.026","site_version":"N10256N","default_lang":"VIE"}}`))
		return
	}

	if path == "/API/Web/Login" {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="device",nonce="fda214a42b5b3f4bc63b544897117e8f",qop="auth",userhash="true"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.Contains(auth, `username="`+m.username+`"`) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var reqBody struct {
			Data struct {
				RemoteTerminalInfo string `json:"remote_terminal_info"`
			} `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody.Data.RemoteTerminalInfo == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"version":"1.0","error_code":"one_IE"}`))
			return
		}
		expectedAuth := buildDigestHeader(m.username, m.password, http.MethodPost, "/API/Web/Login", `Digest realm="device",nonce="fda214a42b5b3f4bc63b544897117e8f",qop="auth"`)
		_ = expectedAuth // verify response hash
		pMap := parseDigestChallenge(auth)
		ha1 := hashHex("", m.username+":device:"+m.password)
		ha2 := hashHex("", "POST:/API/Web/Login")
		wantResp := hashHex("", ha1+":fda214a42b5b3f4bc63b544897117e8f:"+pMap["nc"]+":"+pMap["cnonce"]+":auth:"+ha2)
		if pMap["response"] != wantResp {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Set-Cookie", m.cookie+";HttpOnly;path=/")
		w.Header().Set("X-csrftoken", m.csrf)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"success","device_type":"N10256-16HDD","data":{"strength_need_check":false}}`))
		return
	}

	// All other endpoints require Cookie + X-csrftoken
	if !strings.Contains(r.Header.Get("Cookie"), m.cookie) || r.Header.Get("X-csrftoken") != m.csrf {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"version":"1.0","error_code":"one_IE"}`))
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var env struct {
		Version string         `json:"version"`
		Data    map[string]any `json:"data"`
	}
	_ = json.NewDecoder(r.Body).Decode(&env)

	w.Header().Set("Content-Type", "application/json")
	switch path {
	case "/API/Web/Logout":
		_, _ = w.Write([]byte(`{"version":"1.0","result":"success"}`))
	case "/API/SystemInfo/Base/Get":
		_, _ = w.Write([]byte(`{"result":"success","data":{"device_id":"000000","device_name":"N10256N","device_type":"N10256N","hardware_version":"2M023","software_version":"VC1.3.0-260212","mac_address":"38-24-F1-15-D0-48","p2p_id":"9EHTWW5RGGNJL76Z111A"}}`))
	case "/API/DeviceConfig/Audio/Get":
		_, _ = w.Write([]byte(`{"result":"success","data":{"channel_info":{"CH1":{"audio_type":"G711A","in_volume":9,"out_volume":9}}}}`))
	case "/API/DeviceConfig/Audio/Range":
		_, _ = w.Write([]byte(`{"result":"success","data":{"channel_info":{"items":{"CH1":{"items":{"audio_type":{"items":["G711A","G711U"]}}}}}}}`))
	case "/API/StreamConfig/MainStream/Get":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"channel_info": map[string]any{"CH1": m.mainCh1}},
		})
	case "/API/StreamConfig/MainStream/Set":
		if ci, ok := env.Data["channel_info"].(map[string]any); ok {
			if ch1, ok := ci["CH1"].(map[string]any); ok {
				for k, v := range ch1 {
					m.mainCh1[k] = v
				}
			}
		}
		_, _ = w.Write([]byte(`{"result":"success","data":{}}`))
	case "/API/StreamConfig/MainStream/Range":
		_, _ = w.Write([]byte(`{"result":"success","data":{"channel_info":{"items":{"CH1":{"items":{"streaminfo":{"items":[{"resolution":{"items":["1920 x 1080","2880 x 1620"]},"fps":{"ranges":[[1,15,25,30],[1,15,25]]}}]}}}}}}}`))
	case "/API/StreamConfig/SubStream/Get":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"channel_info": map[string]any{"CH1": m.subCh1}},
		})
	case "/API/ChannelConfig/OSD/Get":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"channel_info": map[string]any{"CH1": m.osdCh1}},
		})
	case "/API/ChannelConfig/OSD/Set":
		if ci, ok := env.Data["channel_info"].(map[string]any); ok {
			if ch1, ok := ci["CH1"].(map[string]any); ok {
				m.osdCh1 = ch1
			}
		}
		_, _ = w.Write([]byte(`{"result":"success","data":{}}`))
	case "/API/Snapshot/Get":
		fakeJPG := base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xd9})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"img_format": "image/jpeg", "img_data": fakeJPG},
		})
	case "/API/ChannelConfig/IPChannel/Get":
		_, _ = w.Write([]byte(`{"result":"success","data":{"channel_info":{"CH1":{"state":"NotConfigured"},"CH2":{"state":"Online","ip_address":"192.168.002.100","port":80,"forward_port":65002,"channel_name":"Cam AI 3K"}}}}`))
	case "/API/NetworkConfig/NetBase/Get":
		_, _ = w.Write([]byte(`{"result":"success","data":{"lan1":{"dhcp":false,"ip_address":"192.168.001.234","subnet_mask":"255.255.255.000","gateway":"192.168.001.001","mac_address":"38-24-F1-15-D0-16"},"lan2":{"dhcp":true,"ip_address":"192.168.002.101","subnet_mask":"255.255.255.000","gateway":"000.000.000.000","mac_address":"38-24-F1-15-D0-48"},"dns1":"192.168.001.001","dns2":"008.008.008.008"}}`))
	case "/API/Event/ChnSmart/Get":
		_, _ = w.Write([]byte(`{"result":"success","data":{"channel_info":{"CH1":{"channel_name":"Camera 01","abilities":[{"title":"Target Detection","ability_info":[{"ability":"FaceDetection","state":"Off","mutual_ability":[{"channel":"CH1","ability":["HumanVehicle"]}]},{"ability":"HumanVehicle","state":"On","mutual_ability":[]}]}]}}}}`))
	case "/API/Event/SettingConfig/Get":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"channel_info": map[string]any{"CH1": m.aiHvCh1}},
		})
	case "/API/Event/SettingConfig/Range":
		_, _ = w.Write([]byte(`{"result":"success","data":{"default_timeout":20000}}`))
	case "/API/Event/SettingConfig/Set":
		if ci, ok := env.Data["channel_info"].(map[string]any); ok {
			if ch1, ok := ci["CH1"].(map[string]any); ok {
				for k, v := range ch1 {
					m.aiHvCh1[k] = v
				}
			}
		}
		_, _ = w.Write([]byte(`{"result":"success","data":{}}`))
	case "/API/Event/AlarmLinkage/Get":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data":   map[string]any{"channel_info": map[string]any{"CH1": m.lnkHvCh1}},
		})
	case "/API/Event/AlarmLinkage/Set":
		if ci, ok := env.Data["channel_info"].(map[string]any); ok {
			if ch1, ok := ci["CH1"].(map[string]any); ok {
				for k, v := range ch1 {
					m.lnkHvCh1[k] = v
				}
			}
		}
		_, _ = w.Write([]byte(`{"result":"success","data":{}}`))
	case "/API/Event/Check":
		m.seq++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": "success",
			"data": map[string]any{
				"reader_id":  3,
				"sequence":   m.seq,
				"lap_number": 0,
				"alarm_list": []map[string]any{
					{
						"time": "10/26/2026 16:00:00",
						"channel_alarm": []map[string]any{
							{
								"channel":               "CH2",
								"channel_name":          "Cam AI 3K",
								"motion_alarm":          false,
								"camera_connect_status": map[string]any{"connect_status": "Online"},
								"alarm_state":           []map[string]any{{"int_subtype": "pd_vd"}},
							},
						},
					},
				},
			},
		})
	default:
		_, _ = w.Write([]byte(`{"result":"success","data":{}}`))
	}
}

func TestRaysharpClientEndToEnd(t *testing.T) {
	mock := startMockRaysharp(t, "admin", "admin1234")
	defer mock.Close()

	ctx := context.Background()

	// 1. Pre-login Range probe
	rng, err := ProbeLoginRange(ctx, mock.server.URL, 3*time.Second)
	if err != nil {
		t.Fatalf("ProbeLoginRange failed: %v", err)
	}
	if rng.SiteVersion != "N10256N" {
		t.Fatalf("unexpected site_version: %q", rng.SiteVersion)
	}

	// 2. Client Login + Serial + ProbeAll
	cl := New(mock.host, mock.port, "admin", "admin1234", 5*time.Second)
	defer cl.Close()

	sn, err := cl.GetSerialNumber(ctx)
	if err != nil {
		t.Fatalf("GetSerialNumber failed: %v", err)
	}
	if sn != "9EHTWW5RGGNJL76Z111A" {
		t.Fatalf("unexpected serial: %q", sn)
	}

	streams, err := cl.ProbeAll(ctx)
	if err != nil {
		t.Fatalf("ProbeAll failed: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams (main+sub), got %d", len(streams))
	}
	if streams[0].Width != 2880 || streams[0].Height != 1620 || streams[0].BitRate != 3072 {
		t.Fatalf("unexpected main stream: %+v", streams[0])
	}

	// 3. Mutate Bitrate, FPS, SmartCodec and Read-Back
	if err := cl.SetBitrate(ctx, 0, StreamMain, 1536, "CBR"); err != nil {
		t.Fatalf("SetBitrate failed: %v", err)
	}
	if err := cl.SetSmartCodec(ctx, 0, StreamMain, true); err != nil {
		t.Fatalf("SetSmartCodec failed: %v", err)
	}
	after, err := cl.GetStreamInfo(ctx, 0, StreamMain)
	if err != nil {
		t.Fatalf("GetStreamInfo failed: %v", err)
	}
	if after.BitRate != 1536 || !after.SmartCodec || after.Compression != "H.265" {
		t.Fatalf("unexpected stream after update: %+v", after)
	}

	// 4. Snapshot Base64 decode
	jpg, err := cl.Snapshot(ctx, 0)
	if err != nil || len(jpg) != 4 || jpg[0] != 0xff || jpg[1] != 0xd8 {
		t.Fatalf("Snapshot failed: err=%v len=%d", err, len(jpg))
	}

	// 5. RemoteDevices (IPChannel/Get with zero-padded IP normalization)
	remDevs, err := cl.GetRemoteDevices(ctx)
	if err != nil || len(remDevs) != 1 {
		t.Fatalf("GetRemoteDevices failed: err=%v len=%d", err, len(remDevs))
	}
	if remDevs[0].Channel != 1 || remDevs[0].Address != "192.168.2.100" {
		t.Fatalf("unexpected remote device: %+v", remDevs[0])
	}

	// 6. NetworkConfig dual-NIC normalization
	netCfg, err := cl.GetNetworkConfig(ctx)
	if err != nil {
		t.Fatalf("GetNetworkConfig failed: %v", err)
	}
	if netCfg.Interfaces["lan1"]["IPAddress"] != "192.168.1.234" || netCfg.Interfaces["lan2"]["IPAddress"] != "192.168.2.101" {
		t.Fatalf("unexpected network interfaces: %+v", netCfg.Interfaces)
	}

	// 7. AI Capabilities, SettingConfig Get/Set, and Event/Check polling
	smart, err := cl.GetChnSmart(ctx)
	if err != nil || len(smart) != 1 {
		t.Fatalf("GetChnSmart failed: err=%v len=%d", err, len(smart))
	}
	updatedAI, err := cl.SetAISetting(ctx, 0, "HumanVehicle", map[string]any{"sensitivity": 80})
	if err != nil {
		t.Fatalf("SetAISetting failed: %v", err)
	}
	if sens, _ := updatedAI["sensitivity"].(float64); int(sens) != 80 {
		t.Fatalf("expected sensitivity 80, got %v", updatedAI["sensitivity"])
	}

	cur := &EventCheckCursor{}
	evs, err := cl.PollEventCheck(ctx, cur)
	if err != nil {
		t.Fatalf("PollEventCheck failed: %v", err)
	}
	if !cur.Initialized || cur.ReaderID != 3 || len(evs) != 1 || evs[0].EventType != "human_vehicle" {
		t.Fatalf("unexpected PollEventCheck result: cur=%+v evs=%+v", cur, evs)
	}
}
