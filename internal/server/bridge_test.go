package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestBridgeAuthAndTargetResolution(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "bridge-test-crypto-key")
	tmp := t.TempDir()
	spoolDir := filepath.Join(tmp, "spool")
	invPath := filepath.Join(tmp, "cameras.yaml")

	inv, err := config.LoadInventory(invPath)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}
	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.243:37777",
		Name:     "Đầu ghi 243",
		Host:     "192.168.1.243",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "pw",
		IsNVR:    true,
	})
	_ = inv.Upsert(config.Device{
		ID:         "192.168.1.123:37777",
		Name:       "Camera03",
		Host:       "192.168.1.123",
		Port:       37777,
		Vendor:     config.VendorDahua,
		Username:   "admin",
		Password:   "pw",
		NVRID:      "192.168.1.243:37777",
		NVRChannel: 6,
		NoStorage:  true,
	})
	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.150:37777",
		Name:     "Camera04",
		Host:     "192.168.1.150",
		Port:     37777,
		Vendor:   config.VendorHikvision,
		Username: "admin",
		Password: "pw",
	})

	cfg := config.Default()
	cfg.Bridge.Enabled = true
	cfg.Bridge.APIKey = strings.Repeat("b1", 8)
	cfg.Bridge.SpoolDir = spoolDir
	cfg.Bridge.MaxSpoolMB = 30
	cfg.Events.Enabled = false

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	defer srv.Close()

	// 1. Missing X-Kspcam-Key must be rejected with 401.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bridge/status", nil)
	req.RemoteAddr = "10.0.0.5:45678"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without key, got %d", rec.Code)
	}

	// 2. Valid X-Kspcam-Key returns 200 with accurate mid -> NVR / Cam-SD mappings.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/bridge/status", nil)
	req2.RemoteAddr = "127.0.0.1:45678"
	req2.Header.Set("X-Kspcam-Key", strings.Repeat("b1", 8))
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid key, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var st struct {
		OK       bool `json:"ok"`
		Mappings []struct {
			Mid        string `json:"mid"`
			SourceHost string `json:"sourceHost"`
			Channel    int    `json:"channel"`
			IsCamSD    bool   `json:"isCamSd"`
		} `json:"mappings"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &st); err != nil || !st.OK {
		t.Fatalf("bad status JSON: %v (%s)", err, rec2.Body.String())
	}
	if len(st.Mappings) != 2 {
		t.Fatalf("expected 2 non-NVR camera mappings, got %d", len(st.Mappings))
	}

	// Verify camera03 resolves to NVR 192.168.1.243 channel 5 (0-based for NVRChannel=6) with IsCamSD=false.
	tgt3, ok := srv.resolveBridgeTarget("", "camera03", 0, false)
	if !ok || tgt3.Source.Host != "192.168.1.243" || tgt3.Channel != 5 || tgt3.IsCamSD {
		t.Fatalf("unexpected camera03 target: ok=%v %+v", ok, tgt3)
	}

	// Verify standalone camera04 resolves to itself with IsCamSD=true.
	tgt4, ok := srv.resolveBridgeTarget("", "camera04", 0, false)
	if !ok || tgt4.Source.Host != "192.168.1.150" || !tgt4.IsCamSD {
		t.Fatalf("unexpected camera04 target: ok=%v %+v", ok, tgt4)
	}
}

func TestBridgeVirtualFilenameAndSpoolRangeServing(t *testing.T) {
	st, en, ok := ParseVirtualNVRFilename("v_nvr_2026-11-08T17-35-00_300.mp4")
	if !ok {
		t.Fatal("expected valid virtual NVR filename")
	}
	if en.Sub(st) != 300*time.Second {
		t.Fatalf("expected 300s duration, got %s", en.Sub(st))
	}
	if _, _, ok := ParseVirtualNVRFilename("../etc/passwd"); ok {
		t.Fatal("expected path traversal filename to be rejected")
	}

	t.Setenv("KSPCAM_KEY", "bridge-test-crypto-key")
	tmp := t.TempDir()
	spoolDir := filepath.Join(tmp, "spool")
	inv, err := config.LoadInventory(filepath.Join(tmp, "cameras.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.119:37777",
		Name:     "Camera01",
		Host:     "192.168.1.119",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "secret",
	})

	cfg := config.Default()
	cfg.Bridge.SpoolDir = spoolDir
	cfg.Bridge.MaxSpoolMB = 25 // 25 MB cap (min floor 24 MB)
	cfg.Events.Enabled = false

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Pre-populate cached MP4 in spool for v_nvr_2026-11-08T17-35-00_300.mp4 and verify HTTP 206 Range!
	cacheName := clipSpoolKey("192.168.1.119:37777", 0, st, en)
	payload := make([]byte, 4096)
	copy(payload, []byte("0000ftypisom0000moov-faststart-header-at-0x1c"))
	if err := os.WriteFile(filepath.Join(spoolDir, cacheName), payload, 0o640); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bridge/clip.mp4?mid=camera01&file=v_nvr_2026-11-08T17-35-00_300.mp4", nil)
	req.RemoteAddr = "127.0.0.1:51234"
	req.Header.Set("Range", "bytes=0-31")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected HTTP 206 PartialContent, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 32 {
		t.Fatalf("expected 32 bytes in Range response, got %d", rec.Body.Len())
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("expected Accept-Ranges: bytes, got %q", got)
	}
}

func TestBridgeMultiChannelHikvisionNVRDetection(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "bridge-test-crypto-key")
	tmp := t.TempDir()
	inv, err := config.LoadInventory(filepath.Join(tmp, "cameras.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Simulate imported multi-channel Hikvision NVR entries like 77.88.205.123 (192.168.1.50:82-c31..c42)
	_ = inv.Upsert(config.Device{
		ID:         "192.168.1.50:82-c31",
		Name:       "Camera01",
		Host:       "192.168.1.50",
		Port:       82,
		Vendor:     config.VendorHikvision,
		Username:   "live",
		Password:   "secret",
		NVRChannel: 31,
	})
	_ = inv.Upsert(config.Device{
		ID:         "192.168.1.50:82-c32",
		Name:       "Camera02",
		Host:       "192.168.1.50",
		Port:       82,
		Vendor:     config.VendorHikvision,
		Username:   "live",
		Password:   "secret",
		NVRChannel: 32,
	})

	cfg := config.Default()
	cfg.Bridge.SpoolDir = filepath.Join(tmp, "spool")
	cfg.Events.Enabled = false
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	tgt, ok := srv.resolveBridgeTarget("", "camera01", 0, false)
	if !ok {
		t.Fatal("expected camera01 to resolve")
	}
	if tgt.IsCamSD {
		t.Fatalf("expected multi-channel NVR entry 192.168.1.50:82-c31 to have IsCamSD=false")
	}
	if tgt.Channel != 30 {
		t.Fatalf("expected 0-based channel=30 for nvrChannel=31, got %d", tgt.Channel)
	}
}
