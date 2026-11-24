package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

	cases := []struct {
		name       string
		remoteAddr string
		xff        string
		headers    map[string]string
		wantCode   int
	}{
		{
			name:       "loopback poller allowed",
			remoteAddr: "127.0.0.1:41234",
			wantCode:   http.StatusOK,
		},
		{
			name:       "LAN private IP allowed",
			remoteAddr: "192.168.1.50:50000",
			wantCode:   http.StatusOK,
		},
		{
			name:       "Tinc VPN 77.88.x.y allowed",
			remoteAddr: "77.88.204.36:51234",
			wantCode:   http.StatusOK,
		},
		{
			name:       "unauthenticated public WAN rejected",
			remoteAddr: "198.51.100.22:55555",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "unauthenticated WAN behind local FRPC rejected",
			remoteAddr: "127.0.0.1:41235",
			xff:        "198.51.100.22",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "WAN with valid X-Kspcam-Key allowed",
			remoteAddr: "198.51.100.22:55556",
			headers:    map[string]string{"X-Kspcam-Key": bridgeTok},
			wantCode:   http.StatusOK,
		},
		{
			name:       "Yamux staff proxy on loopback with WAN XFF allowed",
			remoteAddr: "127.0.0.1:41236",
			xff:        "198.51.100.22",
			headers:    map[string]string{"X-Inut-Staff-Sig": "verified"},
			wantCode:   http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/system/metrics", nil)
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
