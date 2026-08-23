package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

func TestTools_CameraInventory(t *testing.T) {
	_, inv, srv := newTestSetup(t)
	ctx := context.Background()

	// 1. Upsert a camera via kspcam_upsert_camera
	upsertArgs := map[string]any{
		"id":       "192.168.1.100:37777",
		"name":     "Camera Test 1",
		"host":     "192.168.1.100",
		"port":     37777,
		"vendor":   "dahua",
		"username": "admin",
		"password": "smarthome12345",
	}
	upsertJSON, _ := json.Marshal(upsertArgs)
	res, err := srv.Registry().Call(ctx, "kspcam_upsert_camera", upsertJSON)
	if err != nil {
		t.Fatalf("kspcam_upsert_camera failed: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("upsert returned error result: %+v", res)
	}

	// Verify device exists in inventory
	dev, ok := inv.Get("192.168.1.100:37777")
	if !ok {
		t.Fatalf("camera not found in inventory after upsert")
	}
	if dev.Name != "Camera Test 1" {
		t.Errorf("expected name 'Camera Test 1', got %s", dev.Name)
	}

	// 2. List cameras via kspcam_list_cameras
	listRes, err := srv.Registry().Call(ctx, "kspcam_list_cameras", []byte(`{"vendor":"dahua"}`))
	if err != nil {
		t.Fatalf("kspcam_list_cameras failed: %v", err)
	}
	if listRes.IsError || len(listRes.Content) == 0 {
		t.Fatalf("list returned error result: %+v", listRes)
	}
	if !strings.Contains(listRes.Content[0].Text, "192.168.1.100:37777") {
		t.Errorf("expected list output to contain 192.168.1.100:37777, got: %s", listRes.Content[0].Text)
	}

	// 3. Delete camera via kspcam_delete_camera
	delRes, err := srv.Registry().Call(ctx, "kspcam_delete_camera", []byte(`{"id":"192.168.1.100:37777"}`))
	if err != nil {
		t.Fatalf("kspcam_delete_camera failed: %v", err)
	}
	if delRes.IsError {
		t.Fatalf("delete returned error result: %+v", delRes)
	}
	if _, ok := inv.Get("192.168.1.100:37777"); ok {
		t.Errorf("expected device to be deleted from inventory")
	}
}

func TestTools_ShinobiManagement(t *testing.T) {
	// Mock Shinobi HTTP Server
	mockShinobi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		p := r.URL.Path

		switch {
		case strings.HasSuffix(p, "/monitor/testgroup"):
			// ListMonitors
			_, _ = w.Write([]byte(`[{"mid":"cam1","name":"Test Cam 1","type":"h264","mode":"record","host":"192.168.1.100","port":"554","details":{"auto_host":"rtsp://admin:pass@192.168.1.100:554/cam/realmonitor?channel=1&subtype=0"}}]`))
		case strings.HasSuffix(p, "/monitor/testgroup/cam1"):
			// GetMonitor
			_, _ = w.Write([]byte(`{"mid":"cam1","name":"Test Cam 1","type":"h264","mode":"record","host":"192.168.1.100","port":"554","details":{"auto_host":"rtsp://admin:pass@192.168.1.100:554/cam/realmonitor?channel=1&subtype=0"}}`))
		case strings.Contains(p, "/configureMonitor/"):
			// Add or Edit Monitor
			_, _ = w.Write([]byte(`{"ok":true,"msg":"saved"}`))
		case strings.HasSuffix(p, "/delete"):
			// Delete Monitor
			_, _ = w.Write([]byte(`{"ok":true,"msg":"deleted"}`))
		case strings.Contains(p, "/videos/"):
			// Get Videos
			_, _ = w.Write([]byte(`[{"mid":"cam1","filename":"2026-08-23T15-00-00.mp4","size":123456}]`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer mockShinobi.Close()

	cfg := config.Default()
	cfg.Shinobi.APIURL = mockShinobi.URL
	cfg.Shinobi.APIKey = "testkey"
	cfg.Shinobi.GroupKey = "testgroup"

	invFile := t.TempDir() + "/cameras.yaml"
	inv, _ := config.LoadInventory(invFile)
	sc := shinobi.NewClient(cfg.Shinobi.APIURL, cfg.Shinobi.APIKey, cfg.Shinobi.GroupKey)
	srv := NewServer(&cfg, inv, sc)
	ctx := context.Background()

	// 1. shinobi_list_monitors
	res, err := srv.Registry().Call(ctx, "shinobi_list_monitors", []byte(`{}`))
	if err != nil {
		t.Fatalf("shinobi_list_monitors failed: %v", err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, "cam1") {
		t.Fatalf("expected monitor cam1 in output: %s", res.Content[0].Text)
	}

	// 2. shinobi_add_monitor
	addArgs := map[string]any{
		"mid":     "cam2",
		"name":    "Camera 2",
		"rtspUrl": "rtsp://admin:pass@192.168.1.102:554/cam/realmonitor?channel=1&subtype=0",
	}
	addJSON, _ := json.Marshal(addArgs)
	res, err = srv.Registry().Call(ctx, "shinobi_add_monitor", addJSON)
	if err != nil {
		t.Fatalf("shinobi_add_monitor failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("shinobi_add_monitor returned error: %s", res.Content[0].Text)
	}

	// 3. shinobi_sync_from_shinobi (Pull)
	res, err = srv.Registry().Call(ctx, "shinobi_sync_from_shinobi", []byte(`{}`))
	if err != nil {
		t.Fatalf("shinobi_sync_from_shinobi failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("sync_from_shinobi returned error: %s", res.Content[0].Text)
	}

	// Verify inventory now has imported device
	if dev, ok := inv.FindByHost("192.168.1.100", 37777); !ok {
		t.Errorf("expected device 192.168.1.100 to be imported into inventory")
	} else if dev.Name != "Test Cam 1" {
		t.Errorf("expected imported name 'Test Cam 1', got: %s", dev.Name)
	}

	// 4. shinobi_sync_to_shinobi (Push)
	res, err = srv.Registry().Call(ctx, "shinobi_sync_to_shinobi", []byte(`{}`))
	if err != nil {
		t.Fatalf("shinobi_sync_to_shinobi failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("sync_to_shinobi returned error: %s", res.Content[0].Text)
	}

	// 5. shinobi_get_videos
	res, err = srv.Registry().Call(ctx, "shinobi_get_videos", []byte(`{"mid":"cam1"}`))
	if err != nil {
		t.Fatalf("shinobi_get_videos failed: %v", err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, "2026-08-23T15-00-00.mp4") {
		t.Fatalf("expected video filename in output: %s", res.Content[0].Text)
	}
}
