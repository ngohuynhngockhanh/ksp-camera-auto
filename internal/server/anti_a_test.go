package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestAntiAGuardian_StatusAndConfig(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, _ := config.LoadInventory(invFile)

	cfg := config.AntiAConfig{
		Enabled:         false,
		IntervalMinutes: 15,
		Mode:            "random",
	}

	g := newAntiAGuardian(nil, inv, "", cfg)
	status := g.getStatus()
	if status.Enabled {
		t.Errorf("expected Enabled=false, got true")
	}
	if status.IntervalMinutes != 15 {
		t.Errorf("expected IntervalMinutes=15, got %d", status.IntervalMinutes)
	}
	if status.Mode != "random" {
		t.Errorf("expected Mode=random, got %s", status.Mode)
	}

	// Update config
	enabled := true
	mins := 60
	mode := "full"
	err := g.updateConfig(context.Background(), antiAConfigReq{
		Enabled:         &enabled,
		IntervalMinutes: &mins,
		Mode:            &mode,
	})
	if err != nil {
		t.Fatalf("updateConfig: %v", err)
	}

	status = g.getStatus()
	if !status.Enabled {
		t.Errorf("expected Enabled=true, got false")
	}
	if status.IntervalMinutes != 60 {
		t.Errorf("expected IntervalMinutes=60, got %d", status.IntervalMinutes)
	}
	if status.Mode != "full" {
		t.Errorf("expected Mode=full, got %s", status.Mode)
	}
}

func TestAntiAGuardian_IsH265AndSmartCodec(t *testing.T) {
	tests := []struct {
		stream camera.StreamInfo
		want   bool
	}{
		{stream: camera.StreamInfo{Compression: "H.265", SmartCodec: true}, want: true},
		{stream: camera.StreamInfo{Compression: "HEVC", SmartCodec: true}, want: true},
		{stream: camera.StreamInfo{Compression: "h265", SmartCodec: true}, want: true},
		{stream: camera.StreamInfo{Compression: "H.264", SmartCodec: true}, want: false},
		{stream: camera.StreamInfo{Compression: "H.265", SmartCodec: false}, want: false},
		{stream: camera.StreamInfo{Compression: "MJPEG", SmartCodec: true}, want: false},
	}

	for _, tc := range tests {
		got := isStreamH265AndSmartCodec(tc.stream)
		if got != tc.want {
			t.Errorf("isStreamH265AndSmartCodec(%+v) = %v, want %v", tc.stream, got, tc.want)
		}
	}
}

func TestAntiAGuardian_HTTPHandlers(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, _ := config.LoadInventory(invFile)

	cfg := config.Default()
	cfg.AntiA = config.AntiAConfig{
		Enabled:         true,
		IntervalMinutes: 30,
		Mode:            "random",
	}

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	defer srv.Close()

	// 1. GET /api/anti-a (authed as admin)
	req := httptest.NewRequest("GET", "/api/anti-a", nil)
	rec := httptest.NewRecorder()

	token := srv.session.create("admin")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})

	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/anti-a: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var status antiAStatusView
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if !status.Enabled {
		t.Errorf("expected status.Enabled=true, got false")
	}

	// 2. POST /api/anti-a (update config)
	enabled := false
	mins := 45
	postBody, _ := json.Marshal(antiAConfigReq{
		Enabled:         &enabled,
		IntervalMinutes: &mins,
	})

	postReq := httptest.NewRequest("POST", "/api/anti-a", bytes.NewReader(postBody))
	postReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	postRec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("POST /api/anti-a: expected 200, got %d: %s", postRec.Code, postRec.Body.String())
	}

	// 3. POST /api/anti-a/trigger
	trigReq := httptest.NewRequest("POST", "/api/anti-a/trigger", bytes.NewReader([]byte("{}")))
	trigReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	trigRec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(trigRec, trigReq)
	if trigRec.Code != http.StatusOK {
		t.Fatalf("POST /api/anti-a/trigger: expected 200, got %d: %s", trigRec.Code, trigRec.Body.String())
	}
}
