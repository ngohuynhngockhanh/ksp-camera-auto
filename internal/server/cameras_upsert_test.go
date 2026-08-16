package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestHandleCamerasUpsertUpdatesExistingHostWhenPortChanges(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "camera-upsert-test-key")
	inv, err := config.LoadInventory(filepath.Join(t.TempDir(), "cameras.yaml"))
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	existing := config.Device{
		ID: "192.168.1.196:8888", Name: "旧名称", Host: "192.168.1.196", Port: 8888,
		Vendor: config.VendorDahua, Username: "old-user", Password: "old-pass",
		SerialNumber: "SN-OLD", NVRID: "nvr-1", NVRChannel: 4, NVRName: "Kênh 4", NoStorage: true,
	}
	if err := inv.Upsert(existing); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	cfg := config.Default()
	cfg.CamerasFile = filepath.Join(t.TempDir(), "server-cameras.yaml")
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Close()

	body := cameraUpsertReq{
		Name: "IPC-A32E-L_29598AFPBV104FD", Host: "192.168.1.196", Port: 37777,
		Vendor: config.VendorDahua, Username: "admin", Password: "inut1234",
	}
	res := postCameraUpsert(t, srv, body)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	devices := inv.List()
	if len(devices) != 1 {
		t.Fatalf("inventory length = %d, want 1: %+v", len(devices), devices)
	}
	got := devices[0]
	if got.ID != existing.ID {
		t.Errorf("ID = %q, want stable old ID %q", got.ID, existing.ID)
	}
	if got.Host != existing.Host || got.Port != 37777 || got.Vendor != config.VendorDahua || got.Name != body.Name {
		t.Errorf("network identity not updated: %+v", got)
	}
	if got.Username != "admin" || got.Password != "inut1234" {
		t.Errorf("credentials = %q/%q, want tested credentials", got.Username, got.Password)
	}
	if got.SerialNumber != existing.SerialNumber || got.NVRID != existing.NVRID || got.NVRChannel != existing.NVRChannel || !got.NoStorage {
		t.Errorf("non-scan metadata was not preserved: %+v", got)
	}
}

func TestHandleCamerasUpsertKeepsExistingCredentialsWhenBlank(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "camera-upsert-test-key")
	inv, err := config.LoadInventory(filepath.Join(t.TempDir(), "cameras.yaml"))
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	existing := config.Device{
		ID: "192.168.1.197:37777", Host: "192.168.1.197", Port: 37777,
		Vendor: config.VendorDahua, Username: "camera-user", Password: "camera-pass",
	}
	if err := inv.Upsert(existing); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}
	cfg := config.Default()
	cfg.CamerasFile = filepath.Join(t.TempDir(), "server-cameras.yaml")
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Close()

	res := postCameraUpsert(t, srv, cameraUpsertReq{
		Name: "camera-updated", Host: existing.Host, Port: existing.Port, Vendor: existing.Vendor,
	})
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	got, ok := inv.Get(existing.ID)
	if !ok {
		t.Fatal("existing camera disappeared")
	}
	if got.Username != existing.Username || got.Password != existing.Password {
		t.Errorf("blank credentials replaced existing values: %+v", got)
	}
}

func postCameraUpsert(t *testing.T, srv *Server, body cameraUpsertReq) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/cameras", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	srv.handleCamerasUpsert(res, req)
	return res
}
