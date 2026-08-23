package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if cfg.Server.Addr != ":2028" {
		t.Errorf("expected default Server.Addr :2028, got %s", cfg.Server.Addr)
	}
	if cfg.Shinobi.APIURL != "http://127.0.0.1:8080" {
		t.Errorf("expected default Shinobi.APIURL http://127.0.0.1:8080, got %s", cfg.Shinobi.APIURL)
	}
	if cfg.Shinobi.APIKey != "" {
		t.Errorf("expected empty default Shinobi.APIKey, got %s", cfg.Shinobi.APIKey)
	}
	if cfg.Shinobi.GroupKey != "" {
		t.Errorf("expected empty default Shinobi.GroupKey, got %s", cfg.Shinobi.GroupKey)
	}
	if !cfg.MCP.Enabled {
		t.Errorf("expected default MCP.Enabled true, got false")
	}
	if !cfg.MCP.AllowUnauthenticatedLoopback {
		t.Errorf("expected default MCP.AllowUnauthenticatedLoopback true, got false")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("unexpected error loading missing file: %v", err)
	}
	if cfg.Server.Addr != ":2028" {
		t.Errorf("expected default Server.Addr :2028, got %s", cfg.Server.Addr)
	}
	if cfg.Shinobi.APIURL != "http://127.0.0.1:8080" {
		t.Errorf("expected default Shinobi.APIURL http://127.0.0.1:8080, got %s", cfg.Shinobi.APIURL)
	}
}

func TestLoadShinobiAndMCPConfig(t *testing.T) {
	yamlContent := `
server:
  addr: ":3000"
  username: "custom_admin"
shinobi:
  api_url: "http://192.168.1.100:8080"
  api_key: "test_api_key_12345678901234567890"
  group_key: "group_xyz"
mcp:
  enabled: true
  api_key: "mcp_secret_key"
  allow_unauthenticated_loopback: false
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Addr != ":3000" {
		t.Errorf("expected Server.Addr :3000, got %s", cfg.Server.Addr)
	}
	if cfg.Server.Username != "custom_admin" {
		t.Errorf("expected Server.Username custom_admin, got %s", cfg.Server.Username)
	}
	// Default password should be backfilled
	if cfg.Server.Password != "smarthome12345" {
		t.Errorf("expected backfilled default Server.Password, got %s", cfg.Server.Password)
	}

	// Shinobi fields
	if cfg.Shinobi.APIURL != "http://192.168.1.100:8080" {
		t.Errorf("expected Shinobi.APIURL http://192.168.1.100:8080, got %s", cfg.Shinobi.APIURL)
	}
	if cfg.Shinobi.APIKey != "test_api_key_12345678901234567890" {
		t.Errorf("expected Shinobi.APIKey test_api_key_12345678901234567890, got %s", cfg.Shinobi.APIKey)
	}
	if cfg.Shinobi.GroupKey != "group_xyz" {
		t.Errorf("expected Shinobi.GroupKey group_xyz, got %s", cfg.Shinobi.GroupKey)
	}

	// MCP fields
	if !cfg.MCP.Enabled {
		t.Errorf("expected MCP.Enabled true, got false")
	}
	if cfg.MCP.APIKey != "mcp_secret_key" {
		t.Errorf("expected MCP.APIKey mcp_secret_key, got %s", cfg.MCP.APIKey)
	}
	if cfg.MCP.AllowUnauthenticatedLoopback {
		t.Errorf("expected MCP.AllowUnauthenticatedLoopback false, got true")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(tmpFile, []byte("invalid: yaml: content: ["), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Errorf("expected error loading invalid yaml, got nil")
	}
}
