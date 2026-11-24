package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func newSecurityTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	t.Setenv("KSPCAM_KEY", t.Name()+"-enc")
	bridgeTok := "test-hdr-" + t.Name()
	cfg := config.Default()
	cfg.Server.Username = "admin"
	cfg.Server.Password = "pw-" + t.Name()
	cfg.Bridge.APIKey = bridgeTok
	cfg.CamerasFile = filepath.Join(t.TempDir(), "cameras.yaml")

	inv, err := config.LoadInventory(cfg.CamerasFile)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}
	if err := inv.Upsert(config.Device{
		ID:     "cam-routed",
		Name:   "Routed Raysharp Cam",
		Vendor: "raysharp",
		Host:   "203.0.113.77",
		Port:   80,
	}); err != nil {
		t.Fatalf("Upsert device: %v", err)
	}
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv, bridgeTok
}

func TestSystemMetricsAccessControl(t *testing.T) {
	srv, bridgeTok := newSecurityTestServer(t)

	nowTs := strconv.FormatInt(time.Now().Unix(), 10)
	fakeHexSig := strings.Repeat("a1", 32)

	cases := []struct {
		name       string
		remoteAddr string
		host       string
		xff        string
		headers    map[string]string
		wantCode   int
	}{
		{
			name:       "loopback poller allowed",
			remoteAddr: "127.0.0.1:41234",
			host:       "127.0.0.1:2028",
			wantCode:   http.StatusOK,
		},
		{
			name:       "LAN private IP allowed",
			remoteAddr: "192.168.1.50:50000",
			host:       "192.168.1.10:2028",
			wantCode:   http.StatusOK,
		},
		{
			name:       "Tinc VPN 77.88.x.y allowed",
			remoteAddr: "77.88.204.36:51234",
			host:       "77.88.204.157:2028",
			wantCode:   http.StatusOK,
		},
		{
			name:       "unauthenticated public WAN rejected",
			remoteAddr: "198.51.100.22:55555",
			host:       "ksp-cam-inut-204-157.video.io.vn",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "unauthenticated WAN behind local FRPC rejected",
			remoteAddr: "127.0.0.1:41235",
			host:       "ksp-cam-inut-204-157.video.io.vn",
			xff:        "198.51.100.22",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "WAN with valid X-Kspcam-Key allowed",
			remoteAddr: "198.51.100.22:55556",
			host:       "ksp-cam-inut-204-157.video.io.vn",
			headers:    map[string]string{"X-Kspcam-Key": bridgeTok},
			wantCode:   http.StatusOK,
		},
		{
			name:       "Yamux staff proxy on loopback with loopback Host allowed",
			remoteAddr: "127.0.0.1:41236",
			host:       "127.0.0.1:2028",
			xff:        "198.51.100.22",
			headers: map[string]string{
				"X-Inut-Staff-Sig": fakeHexSig,
				"X-Inut-SSO-User":  "admin",
				"X-Inut-SSO-Role":  "admin",
				"X-Inut-SSO-Ts":    nowTs,
				"X-Inut-Box-ID":    "inut_204_157",
			},
			wantCode: http.StatusOK,
		},
		{
			name:       "forged staff header over FRPC public Host rejected",
			remoteAddr: "127.0.0.1:41237",
			host:       "ksp-cam-inut-204-157.video.io.vn",
			xff:        "198.51.100.22",
			headers: map[string]string{
				"X-Inut-Staff-Sig": fakeHexSig,
				"X-Inut-SSO-User":  "admin",
				"X-Inut-SSO-Role":  "admin",
				"X-Inut-SSO-Ts":    nowTs,
				"X-Inut-Box-ID":    "inut_204_157",
			},
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/system/metrics", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.host != "" {
				req.Host = tc.host
			}
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d (body=%s)", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFRPCBypassBlockedOnStaffSSOBridgeAndMCP(t *testing.T) {
	srv, _ := newSecurityTestServer(t)
	nowTs := strconv.FormatInt(time.Now().Unix(), 10)
	fakeHexSig := strings.Repeat("b2", 32)

	// 1. Forged X-Inut-Staff-Sig over FRPC public Host MUST get 401 on /api/cameras
	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.RemoteAddr = "127.0.0.1:41500"
	req.Host = "ksp-cam-inut-204-157.video.io.vn"
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-For", "198.51.100.22")
	req.Header.Set("X-Inut-Staff-Sig", fakeHexSig)
	req.Header.Set("X-Inut-SSO-User", "admin")
	req.Header.Set("X-Inut-SSO-Role", "admin")
	req.Header.Set("X-Inut-SSO-Ts", nowTs)
	req.Header.Set("X-Inut-Box-ID", "inut_204_157")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged FRPC staff header on /api/cameras expected 401, got %d", rec.Code)
	}

	// 2. Authentic local Yamux staff proxy (Host=127.0.0.1:2028) MUST succeed (200 OK)
	yamuxReq := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	yamuxReq.RemoteAddr = "127.0.0.1:41501"
	yamuxReq.Host = "127.0.0.1:2028"
	yamuxReq.Header.Set("X-Inut-Staff-Sig", fakeHexSig)
	yamuxReq.Header.Set("X-Inut-SSO-User", "admin")
	yamuxReq.Header.Set("X-Inut-SSO-Role", "admin")
	yamuxReq.Header.Set("X-Inut-SSO-Ts", nowTs)
	yamuxReq.Header.Set("X-Inut-Box-ID", "inut_204_157")
	yamuxRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(yamuxRec, yamuxReq)
	if yamuxRec.Code != http.StatusOK {
		t.Fatalf("authentic Yamux staff proxy on /api/cameras expected 200, got %d", yamuxRec.Code)
	}

	// 3. Unauthenticated MCP POST /mcp over FRPC (even when mcp.api_key == "" and allow_unauthenticated_loopback == true!) MUST get 401
	mcpFRPC := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	mcpFRPC.RemoteAddr = "127.0.0.1:41502"
	mcpFRPC.Host = "ksp-cam-inut-204-157.video.io.vn"
	mcpFRPC.Header.Set("X-Forwarded-For", "198.51.100.22")
	mcpRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(mcpRec, mcpFRPC)
	if mcpRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated MCP over FRPC expected 401, got %d", mcpRec.Code)
	}

	// 4. Direct local MCP POST /mcp (127.0.0.1 without XFF) MUST still succeed (200 OK)
	mcpLocal := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	mcpLocal.RemoteAddr = "127.0.0.1:41503"
	mcpLocal.Host = "127.0.0.1:2028"
	mcpLocalRec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(mcpLocalRec, mcpLocal)
	if mcpLocalRec.Code != http.StatusOK {
		t.Fatalf("direct local MCP expected 200, got %d", mcpLocalRec.Code)
	}

	// 5. Unauthenticated Bridge over FRPC when bridge.api_key == "" MUST get 401
	srvNoKey := *srv
	srvNoKey.cfg.Bridge.APIKey = ""
	srvNoKey.cfg.Shinobi.APIKey = ""
	srvNoKey.cfg.MCP.APIKey = ""
	brFRPC := httptest.NewRequest(http.MethodGet, "/api/v1/bridge/status", nil)
	brFRPC.RemoteAddr = "127.0.0.1:41504"
	brFRPC.Host = "ksp-cam-inut-204-157.video.io.vn"
	brFRPC.Header.Set("X-Forwarded-For", "198.51.100.22")
	if srvNoKey.authorizeBridge(brFRPC) {
		t.Fatalf("expected authorizeBridge to reject FRPC-proxied WAN request")
	}
}

func TestRaysharpWebhookAccessControl(t *testing.T) {
	srv, bridgeTok := newSecurityTestServer(t)

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		headers    map[string]string
		wantCode   int
	}{
		{
			name:       "LAN camera webhook allowed",
			remoteAddr: "192.168.1.108:33445",
			wantCode:   http.StatusOK,
		},
		{
			name:       "configured routed camera host allowed",
			remoteAddr: "203.0.113.77:33446",
			wantCode:   http.StatusOK,
		},
		{
			name:       "unknown public WAN spoof rejected",
			remoteAddr: "198.51.100.99:33447",
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "unauthenticated WAN behind local FRPC rejected",
			remoteAddr: "127.0.0.1:33448",
			xff:        "198.51.100.99",
			wantCode:   http.StatusForbidden,
		},
		{
			name:       "authenticated WAN caller with X-Kspcam-Key allowed",
			remoteAddr: "198.51.100.99:33449",
			headers:    map[string]string{"X-Kspcam-Key": bridgeTok},
			wantCode:   http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/events/webhook/raysharp", strings.NewReader(`{"alarm_list":[]}`))
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("expected status %d, got %d (body=%s)", tc.wantCode, rec.Code, rec.Body.String())
			}
		})
	}
}
