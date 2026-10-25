package shinobi_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

func TestBuildMonitorConfigGoldenSOPDetails(t *testing.T) {
	client := shinobi.NewClient("http://127.0.0.1:8080", "test_api_key", "test_group_key")

	dev := config.Device{
		ID:       "192.168.1.101:37777",
		Name:     "Camera 01",
		Host:     "192.168.1.101",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "secret",
	}

	cfg := client.BuildMonitorConfig(dev)

	wantCustRecord := "-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"
	if cfg.Details.CustRecord != wantCustRecord {
		t.Fatalf("CustRecord = %q, want %q", cfg.Details.CustRecord, wantCustRecord)
	}
	if cfg.Details.WallClockTimestampIgnore != "0" {
		t.Fatalf("WallClockTimestampIgnore = %q, want %q", cfg.Details.WallClockTimestampIgnore, "0")
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal(cfg) failed: %v", err)
	}

	var parsed struct {
		Mid     string         `json:"mid"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if parsed.Mid != "camera01" {
		t.Errorf("mid = %q, want %q", parsed.Mid, "camera01")
	}

	if got, ok := parsed.Details["cust_record"].(string); !ok || got != wantCustRecord {
		t.Errorf("details.cust_record = %v, want %q", parsed.Details["cust_record"], wantCustRecord)
	}
	if got, ok := parsed.Details["wall_clock_timestamp_ignore"].(string); !ok || got != "0" {
		t.Errorf("details.wall_clock_timestamp_ignore = %v, want %q", parsed.Details["wall_clock_timestamp_ignore"], "0")
	}
	if got, ok := parsed.Details["snap"].(string); !ok || got != "0" {
		t.Errorf("details.snap = %v, want %q", parsed.Details["snap"], "0")
	}
	if got, exists := parsed.Details["input_maps"]; !exists {
		t.Errorf("details.input_maps missing from JSON")
	} else if gotStr, ok := got.(string); !ok || gotStr != "" {
		t.Errorf("details.input_maps = %#v, want empty string", got)
	}

	choicesRaw, ok := parsed.Details["input_map_choices"].(map[string]any)
	if !ok || choicesRaw == nil {
		t.Fatalf("details.input_map_choices missing or not an object: %#v", parsed.Details["input_map_choices"])
	}

	requiredKeys := []string{
		"stream",
		"snap",
		"record",
		"record_timelapse",
		"detector",
		"detector_object",
		"detector_sip_buffer",
	}
	for _, k := range requiredKeys {
		val, exists := choicesRaw[k]
		if !exists {
			t.Errorf("details.input_map_choices[%q] is missing", k)
			continue
		}
		arr, ok := val.([]any)
		if !ok {
			t.Errorf("details.input_map_choices[%q] is not a JSON array (got %T: %#v)", k, val, val)
			continue
		}
		if len(arr) != 0 {
			t.Errorf("details.input_map_choices[%q] len = %d, want 0", k, len(arr))
		}
	}
}

func TestSyncToShinobiUpdatesLegacyCustRecord(t *testing.T) {
	apiKey := "test_api_key_legacy"
	groupKey := "test_group_legacy"
	mock := NewMockShinobiServer(apiKey, groupKey)
	defer mock.Close()

	client := shinobi.NewClient(mock.Server.URL, apiKey, groupKey)
	ctx := context.Background()

	// Seed a monitor on Shinobi with legacy "-tag:v hvc1" CustRecord
	legacyMon := shinobi.MonitorConfig{
		Mid:      "camera01",
		Ke:       groupKey,
		Name:     "Camera 01",
		Type:     "h264",
		Mode:     "record",
		Host:     "192.168.1.101",
		Port:     "554",
		Protocol: "rtsp",
		Path:     "/cam/realmonitor?channel=1&subtype=0",
		Ext:      "mp4",
		Details: shinobi.MonitorDetails{
			AutoHost:                 "rtsp://admin:secret@192.168.1.101:554/cam/realmonitor?channel=1&subtype=0",
			Muser:                    "admin",
			Mpass:                    "secret",
			CustRecord:               "-tag:v hvc1",
			WallClockTimestampIgnore: "0",
		},
	}
	if err := client.AddMonitor(ctx, legacyMon); err != nil {
		t.Fatalf("AddMonitor legacyMon failed: %v", err)
	}

	inv, err := config.LoadInventory(t.TempDir() + "/cameras.yaml")
	if err != nil {
		t.Fatalf("LoadInventory failed: %v", err)
	}
	_ = inv.Upsert(config.Device{
		ID:       "192.168.1.101:37777",
		Name:     "Camera 01",
		Host:     "192.168.1.101",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "secret",
	})

	report, err := client.SyncToShinobi(ctx, inv)
	if err != nil {
		t.Fatalf("SyncToShinobi failed: %v", err)
	}
	if report.Updated != 1 || report.Unchanged != 0 {
		t.Fatalf("expected 1 updated monitor when upgrading legacy CustRecord, got %+v", report)
	}

	updated, err := client.GetMonitor(ctx, "camera01")
	if err != nil {
		t.Fatalf("GetMonitor failed: %v", err)
	}
	det := updated.ParseDetails()
	if det.CustRecord != shinobi.GoldenCustRecordFlags {
		t.Errorf("updated monitor CustRecord = %q, want %q", det.CustRecord, shinobi.GoldenCustRecordFlags)
	}
}
