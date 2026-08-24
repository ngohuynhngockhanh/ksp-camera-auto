package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestServer_TrafficEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, _ := config.LoadInventory(invFile)

	cfg := config.Default()
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	defer srv.Close()

	token := srv.session.create("admin")

	// 1. GET /api/network/traffic/interfaces
	req := httptest.NewRequest("GET", "/api/network/traffic/interfaces", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/network/traffic/interfaces: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var ifaceRes struct {
		Interfaces []string `json:"interfaces"`
		Default    string   `json:"default"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ifaceRes); err != nil {
		t.Fatalf("unmarshal interfaces: %v", err)
	}
	for _, ifi := range ifaceRes.Interfaces {
		if ifi == "wlan0" || ifi == "lo" {
			t.Errorf("expected wlan0/lo excluded, got %s", ifi)
		}
	}

	// 2. GET /api/network/traffic/snapshot
	reqSnap := httptest.NewRequest("GET", "/api/network/traffic/snapshot?iface=eth0", nil)
	reqSnap.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	recSnap := httptest.NewRecorder()

	srv.Handler().ServeHTTP(recSnap, reqSnap)
	if recSnap.Code != http.StatusOK {
		t.Fatalf("GET /api/network/traffic/snapshot: expected 200, got %d: %s", recSnap.Code, recSnap.Body.String())
	}
}
