package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestMCP_AntiATools(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, _ := config.LoadInventory(invFile)

	cfg := config.Default()
	cfg.AntiA = config.AntiAConfig{
		Enabled:         false,
		IntervalMinutes: 30,
		Mode:            "random",
	}

	s := NewServer(&cfg, inv, nil)

	// 1. Test kspcam_get_anti_a_status
	resp, isNotif := s.ProcessRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(map[string]interface{}{
			"name":      "kspcam_get_anti_a_status",
			"arguments": map[string]interface{}{},
		}),
	})
	if isNotif || resp.Error != nil {
		t.Fatalf("kspcam_get_anti_a_status: %v", resp.Error)
	}

	// 2. Test kspcam_set_anti_a_config
	resp, _ = s.ProcessRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(map[string]interface{}{
			"name": "kspcam_set_anti_a_config",
			"arguments": map[string]interface{}{
				"enabled":          true,
				"interval_minutes": 20,
				"mode":             "full",
			},
		}),
	})
	if resp.Error != nil {
		t.Fatalf("kspcam_set_anti_a_config: %v", resp.Error)
	}
	if !cfg.AntiA.Enabled || cfg.AntiA.IntervalMinutes != 20 || cfg.AntiA.Mode != "full" {
		t.Errorf("expected config updated, got %+v", cfg.AntiA)
	}

	// 3. Test kspcam_trigger_anti_a
	resp, _ = s.ProcessRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: mustJSON(map[string]interface{}{
			"name": "kspcam_trigger_anti_a",
			"arguments": map[string]interface{}{
				"force_all": false,
			},
		}),
	})
	if resp.Error != nil {
		t.Fatalf("kspcam_trigger_anti_a: %v", resp.Error)
	}
}

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
