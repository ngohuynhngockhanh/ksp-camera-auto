package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mcp"
)

func TestServer_MCPRoutes(t *testing.T) {
	cfg := config.Default()
	cfg.MCP.APIKey = "ksp_mcp_test_secret"
	cfg.MCP.AllowUnauthenticatedLoopback = false

	invFile := t.TempDir() + "/cameras.yaml"
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}

	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	defer srv.Close()

	handler := srv.Handler()

	// 1. POST /mcp (Direct) - 401 without auth
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.RemoteAddr = "10.0.0.5:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
	}

	// 2. POST /mcp (Direct) - 200 with X-MCP-Key
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(reqBody))
	req.RemoteAddr = "10.0.0.5:12345"
	req.Header.Set("X-MCP-Key", "ksp_mcp_test_secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp mcp.JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal direct response: %v", err)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error in ping: %v", resp.Error)
	}

	// 3. GET /mcp (SSE Stream)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sseReq := httptest.NewRequest(http.MethodGet, "/mcp?key=ksp_mcp_test_secret", nil).WithContext(ctx)
	sseReq.RemoteAddr = "10.0.0.5:12345"
	sseRec := httptest.NewRecorder()

	go func() {
		handler.ServeHTTP(sseRec, sseReq)
	}()

	time.Sleep(50 * time.Millisecond)

	out := sseRec.Body.String()
	if !strings.Contains(out, "event: endpoint") {
		t.Fatalf("expected 'event: endpoint' in SSE output, got: %s", out)
	}

	var sessionID string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "data: /mcp/messages?sessionId=") {
			sessionID = strings.TrimPrefix(l, "data: /mcp/messages?sessionId=")
			break
		}
	}
	if sessionID == "" {
		t.Fatalf("failed to extract sessionId from SSE output: %s", out)
	}

	// 4. POST /mcp/messages
	msgBody := `{"jsonrpc":"2.0","id":42,"method":"tools/list"}`
	msgReq := httptest.NewRequest(http.MethodPost, "/mcp/messages?sessionId="+sessionID+"&key=ksp_mcp_test_secret", strings.NewReader(msgBody))
	msgReq.RemoteAddr = "10.0.0.5:12345"
	msgRec := httptest.NewRecorder()
	handler.ServeHTTP(msgRec, msgReq)

	if msgRec.Code != http.StatusAccepted {
		t.Errorf("expected 202 Accepted for POST /mcp/messages, got %d", msgRec.Code)
	}

	time.Sleep(50 * time.Millisecond)

	finalSSE := sseRec.Body.String()
	if !strings.Contains(finalSSE, "kspcam_list_cameras") || !strings.Contains(finalSSE, "shinobi_list_monitors") {
		t.Errorf("expected tools list streamed over SSE, got: %s", finalSSE)
	}
}
