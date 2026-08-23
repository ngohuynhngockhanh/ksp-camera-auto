package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

func newTestSetup(t *testing.T) (*config.Config, *config.Inventory, *Server) {
	t.Helper()
	cfg := config.Default()
	cfg.MCP.APIKey = "test_mcp_secret_key"
	cfg.MCP.AllowUnauthenticatedLoopback = false

	invFile := t.TempDir() + "/cameras.yaml"
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatalf("load test inventory: %v", err)
	}

	sc := shinobi.NewClient("http://127.0.0.1:8080", "testkey", "testgroup")
	srv := NewServer(&cfg, inv, sc)
	return &cfg, inv, srv
}

func TestServer_InitializeAndPing(t *testing.T) {
	_, _, srv := newTestSetup(t)
	ctx := context.Background()

	// 1. Initialize
	initReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-client","version":"1.0"}}`),
	}
	resp, isNotif := srv.ProcessRequest(ctx, initReq)
	if isNotif {
		t.Errorf("expected response, got notification")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	initRes, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatalf("expected InitializeResult, got %T", resp.Result)
	}
	if initRes.ServerInfo.Name != "kspcam" {
		t.Errorf("expected server name kspcam, got %s", initRes.ServerInfo.Name)
	}

	// 2. notifications/initialized
	notifReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	_, isNotif = srv.ProcessRequest(ctx, notifReq)
	if !isNotif {
		t.Errorf("expected isNotification=true for notifications/initialized")
	}

	// 3. Ping
	pingReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "ping",
	}
	resp, isNotif = srv.ProcessRequest(ctx, pingReq)
	if isNotif || resp.Error != nil {
		t.Fatalf("ping failed: %v", resp.Error)
	}
}

func TestServer_ToolsList(t *testing.T) {
	_, _, srv := newTestSetup(t)
	ctx := context.Background()

	listReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-list",
		Method:  "tools/list",
	}
	resp, _ := srv.ProcessRequest(ctx, listReq)
	if resp.Error != nil {
		t.Fatalf("tools/list error: %v", resp.Error)
	}

	toolsList, ok := resp.Result.(ToolsListResult)
	if !ok {
		t.Fatalf("expected ToolsListResult, got %T", resp.Result)
	}

	// Ensure all key tool names are registered
	toolMap := make(map[string]Tool)
	for _, tool := range toolsList.Tools {
		toolMap[tool.Name] = tool
	}

	expectedTools := []string{
		"kspcam_list_cameras",
		"kspcam_upsert_camera",
		"kspcam_delete_camera",
		"kspcam_probe_camera",
		"kspcam_apply_profile",
		"kspcam_set_channel_name",
		"kspcam_set_osd",
		"kspcam_reboot_camera",
		"kspcam_change_password",
		"kspcam_scan_lan",
		"kspcam_try_password",
		"kspcam_wifi_scan",
		"kspcam_get_network",
		"kspcam_get_nvr_health",
		"kspcam_get_recordings",
		"kspcam_get_snapshot",
		"shinobi_list_monitors",
		"shinobi_add_monitor",
		"shinobi_edit_monitor",
		"shinobi_delete_monitor",
		"shinobi_sync_to_shinobi",
		"shinobi_sync_from_shinobi",
		"shinobi_change_monitor_state",
		"shinobi_get_videos",
	}

	for _, name := range expectedTools {
		if _, found := toolMap[name]; !found {
			t.Errorf("missing expected tool in registry: %s", name)
		}
	}
}

func TestServer_StdioTransport(t *testing.T) {
	_, _, srv := newTestSetup(t)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	}, "\n") + "\n"

	in := strings.NewReader(input)
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := RunStdioWithStreams(ctx, srv, in, out); err != nil {
		t.Fatalf("RunStdioWithStreams failed: %v", err)
	}

	scanner := bufio.NewScanner(out)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 2 { // initialize and ping responses (notification has no response)
		t.Fatalf("expected 2 responses on stdout, got %d:\n%s", len(lines), out.String())
	}

	var resp1 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if resp1.ID != float64(1) && resp1.ID != 1 {
		t.Errorf("expected ID 1, got %v", resp1.ID)
	}

	var resp2 JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if resp2.ID != float64(2) && resp2.ID != 2 {
		t.Errorf("expected ID 2, got %v", resp2.ID)
	}
}

func TestServer_HTTPDirectAndAuth(t *testing.T) {
	_, _, srv := newTestSetup(t)
	handler := srv.HTTPHandler()

	// 1. Direct POST without auth should return 401 Unauthorized
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.RemoteAddr = "192.168.1.50:54321" // Non-loopback IP
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. Direct POST with X-MCP-Key header should succeed
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.RemoteAddr = "192.168.1.50:54321"
	req.Header.Set("X-MCP-Key", "test_mcp_secret_key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error in ping: %v", resp.Error)
	}

	// 3. Direct POST with query param ?key=
	req = httptest.NewRequest(http.MethodPost, "/mcp?key=test_mcp_secret_key", strings.NewReader(reqBody))
	req.RemoteAddr = "192.168.1.50:54321"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK with query param, got %d", rec.Code)
	}

	// 4. Direct POST with Bearer token
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.RemoteAddr = "192.168.1.50:54321"
	req.Header.Set("Authorization", "Bearer test_mcp_secret_key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK with bearer token, got %d", rec.Code)
	}
}

func TestServer_SSETransport(t *testing.T) {
	_, _, srv := newTestSetup(t)
	handler := srv.HTTPHandler()

	// 1. Establish SSE connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sseReq := httptest.NewRequest(http.MethodGet, "/mcp?key=test_mcp_secret_key", nil).WithContext(ctx)
	sseReq.RemoteAddr = "192.168.1.50:54321"
	sseRec := httptest.NewRecorder()

	go func() {
		handler.ServeHTTP(sseRec, sseReq)
	}()

	// Wait for SSE handshake to write headers
	time.Sleep(50 * time.Millisecond)

	// Extract endpoint event and session ID from SSE output
	output := sseRec.Body.String()
	if !strings.Contains(output, "event: endpoint") {
		t.Fatalf("expected 'event: endpoint' in SSE output, got: %s", output)
	}

	var sessionID string
	lines := strings.Split(output, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "data: /mcp/messages?sessionId=") {
			sessionID = strings.TrimPrefix(l, "data: /mcp/messages?sessionId=")
			break
		}
	}
	if sessionID == "" {
		t.Fatalf("failed to extract sessionId from SSE output: %s", output)
	}

	// 2. Post a message to /mcp/messages?sessionId=<sessionID>
	postBody := `{"jsonrpc":"2.0","id":100,"method":"ping"}`
	postReq := httptest.NewRequest(http.MethodPost, "/mcp/messages?sessionId="+sessionID+"&key=test_mcp_secret_key", strings.NewReader(postBody))
	postReq.RemoteAddr = "192.168.1.50:54321"
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted for /mcp/messages, got %d", postRec.Code)
	}

	// Give SSE goroutine a moment to stream the message
	time.Sleep(50 * time.Millisecond)

	sseFinalOutput := sseRec.Body.String()
	if !strings.Contains(sseFinalOutput, "event: message") || !strings.Contains(sseFinalOutput, `"id":100`) {
		t.Errorf("expected event: message containing id:100 in SSE output, got:\n%s", sseFinalOutput)
	}
}
