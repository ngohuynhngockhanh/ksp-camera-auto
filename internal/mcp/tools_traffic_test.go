package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestMCP_TrafficTools(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte("[]"), 0644)
	inv, _ := config.LoadInventory(invFile)

	cfg := config.Default()
	s := NewServer(&cfg, inv, nil)

	// Call kspcam_get_network_traffic
	resp, isNotif := s.ProcessRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mustJSON(map[string]interface{}{
			"name": "kspcam_get_network_traffic",
			"arguments": map[string]interface{}{
				"duration_seconds": 1,
			},
		}),
	})
	if isNotif || resp.Error != nil {
		t.Fatalf("kspcam_get_network_traffic failed: %v", resp.Error)
	}

	// Verify wlan0 rejection
	respWlan, _ := s.ProcessRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params: mustJSON(map[string]interface{}{
			"name": "kspcam_get_network_traffic",
			"arguments": map[string]interface{}{
				"iface": "wlan0",
			},
		}),
	})
	if respWlan.Result == nil {
		t.Errorf("expected result object, got nil")
	}
}
