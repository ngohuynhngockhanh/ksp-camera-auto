package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

type mockBroker struct {
	mu    sync.Mutex
	store map[string]any
}

func newMockBroker() *mockBroker {
	return &mockBroker{
		store: make(map[string]any),
	}
}

func (m *mockBroker) Read(ctx context.Context, keys []string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]any)
	for _, k := range keys {
		if v, ok := m.store[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (m *mockBroker) Write(ctx context.Context, changes map[string]any) (map[string]redbida.WriteAck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	acks := make(map[string]redbida.WriteAck)
	for k, v := range changes {
		old := m.store[k]
		m.store[k] = v
		acks[k] = redbida.WriteAck{OldValue: old, NewValue: v}
	}
	return acks, nil
}

// setupTestServer creates an MCP Server with mock redbida service for adversarial testing.
func setupTestServer(t *testing.T) (*Server, *mockBroker) {
	cfg := config.Default()
	cfg.MCP.Enabled = true
	testAuthKey := "test_" + "mcp_key"
	cfg.MCP.APIKey = testAuthKey
	cfg.Redbida.Enabled = true
	cfg.Redbida.TimeoutSeconds = 2
	cfg.Redbida.MaxBatchKeys = 200

	tempKeyDir := t.TempDir()
	cfg.Redbida.KeyDir = tempKeyDir

	for _, key := range []string{
		"ui_title", "company_name", "ui_bg", "custom_hashtags", "ui_tabs_links",
		"camera_count", "toolbar_show_count", "video_config", "hls_using_go2rtc",
		"hls_using_go2rtc_livestream", "hls_using_go2rtc_tiktok", "ui_scoreboard",
		"logo_header", "logo_header_text", "button_generate_go2rtc_stream",
		"shinobi_camera_id", "shinobi_group_key", "shinobi_token", "shinobi_monitor_token", "ggcode",
	} {
		_ = os.WriteFile(tempKeyDir+"/"+key, []byte("init"), 0o600)
	}

	invFile := t.TempDir() + "/cameras.yaml"
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}

	broker := newMockBroker()
	catalog := redbida.NewCatalog(tempKeyDir)
	rSvc := redbida.NewService(broker, catalog, 200)

	server := NewServer(&cfg, inv, nil, rSvc)
	return server, broker
}

func TestAdversarialChallenge_RedbidaJSONRPC_AllTools(t *testing.T) {
	server, broker := setupTestServer(t)
	ctx := context.Background()

	// Populate mock broker with initial keys
	broker.store["ui_title"] = "Test Pool Arena"
	broker.store["camera_count"] = 8
	broker.store["shinobi_token"] = "real-secret-token"

	t.Run("1. initialize handshake", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-challenger","version":"1.0"}}`),
		}
		resp, isNotif := server.ProcessRequest(ctx, req)
		if isNotif {
			t.Fatalf("expected response, got notification")
		}
		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}
		initRes, ok := resp.Result.(InitializeResult)
		if !ok || initRes.ServerInfo.Name != "kspcam" {
			t.Fatalf("unexpected init result: %+v", resp.Result)
		}
	})

	t.Run("2. tools/list contains all 6 redbida tools and total 31 tools", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      2,
			Method:  "tools/list",
		}
		resp, _ := server.ProcessRequest(ctx, req)
		if resp.Error != nil {
			t.Fatalf("unexpected error: %v", resp.Error)
		}
		listRes, ok := resp.Result.(ToolsListResult)
		if !ok {
			t.Fatalf("invalid result type: %T", resp.Result)
		}
		if len(listRes.Tools) != 31 {
			t.Errorf("expected 31 tools, got %d", len(listRes.Tools))
		}

		expectedRedbida := []string{
			"redbida_list_catalog",
			"redbida_get_keys",
			"redbida_set_keys",
			"redbida_apply_onboarding_preset",
			"redbida_trigger_go2rtc",
			"redbida_get_time_status",
		}
		toolMap := make(map[string]bool)
		for _, tool := range listRes.Tools {
			toolMap[tool.Name] = true
		}
		for _, name := range expectedRedbida {
			if !toolMap[name] {
				t.Errorf("missing redbida tool: %s", name)
			}
		}
	})

	t.Run("3. redbida_list_catalog filters", func(t *testing.T) {
		// Group filter
		args, _ := json.Marshal(map[string]any{"group": "UI / Display"})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_list_catalog", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: p})
		if resp.Error != nil {
			t.Fatalf("error: %v", resp.Error)
		}
		res, ok := resp.Result.(ToolResult)
		if !ok || res.IsError || len(res.Content) == 0 {
			t.Fatalf("unexpected tool result: %+v", resp.Result)
		}
		var payload struct {
			Keys  []redbida.KeyMeta `json:"keys"`
			Count int               `json:"count"`
		}
		if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
			t.Fatalf("json parse error: %v", err)
		}
		if payload.Count == 0 {
			t.Errorf("expected UI / Display keys, got 0")
		}
		for _, k := range payload.Keys {
			if !strings.EqualFold(k.Group, "UI / Display") {
				t.Errorf("key %s has group %s, expected 'UI / Display'", k.Key, k.Group)
			}
		}

		// Editable only filter
		argsEditable, _ := json.Marshal(map[string]any{"editableOnly": true})
		pEdit, _ := json.Marshal(ToolCallParams{Name: "redbida_list_catalog", Arguments: argsEditable})
		respEdit, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 4, Method: "tools/call", Params: pEdit})
		resEdit := respEdit.Result.(ToolResult)
		var editPayload struct {
			Keys []redbida.KeyMeta `json:"keys"`
		}
		_ = json.Unmarshal([]byte(resEdit.Content[0].Text), &editPayload)
		for _, k := range editPayload.Keys {
			if !k.Editable {
				t.Errorf("key %s is not editable but returned under editableOnly", k.Key)
			}
		}
	})

	t.Run("4. redbida_get_keys with masking", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{"keys": []string{"ui_title", "shinobi_token"}})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_get_keys", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 5, Method: "tools/call", Params: p})
		if resp.Error != nil {
			t.Fatalf("error: %v", resp.Error)
		}
		res := resp.Result.(ToolResult)
		if res.IsError {
			t.Fatalf("expected success, got error: %s", res.Content[0].Text)
		}
		var payload struct {
			Values []redbida.KeyValue `json:"values"`
		}
		_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
		foundToken := false
		for _, kv := range payload.Values {
			if kv.Key == "shinobi_token" {
				foundToken = true
				if kv.Value != "********" {
					t.Errorf("secret shinobi_token was NOT masked! got: %v", kv.Value)
				}
			}
			if kv.Key == "ui_title" && kv.Value != "Test Pool Arena" {
				t.Errorf("expected 'Test Pool Arena', got %v", kv.Value)
			}
		}
		if !foundToken {
			t.Errorf("shinobi_token not found in result")
		}
	})

	t.Run("5. redbida_set_keys with read-back verification", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"changes": map[string]any{
				"ui_title": "Updated Champion Bida",
			},
		})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_set_keys", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 6, Method: "tools/call", Params: p})
		if resp.Error != nil {
			t.Fatalf("error: %v", resp.Error)
		}
		res := resp.Result.(ToolResult)
		if res.IsError {
			t.Fatalf("expected success, got error: %s", res.Content[0].Text)
		}
		var payload struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
		if len(payload.Results) != 1 || !payload.Results[0].Applied || !payload.Results[0].Verified {
			t.Fatalf("expected applied and verified, got: %+v", payload.Results)
		}
		if broker.store["ui_title"] != "Updated Champion Bida" {
			t.Errorf("store not updated, got %v", broker.store["ui_title"])
		}
	})

	t.Run("6. redbida_apply_onboarding_preset dryRun and live", func(t *testing.T) {
		// DryRun
		dryArgs, _ := json.Marshal(map[string]any{
			"title":       "CLB Bida Hoàng Gia Sài Gòn",
			"cameraCount": 12,
			"bg":          "radial-gradient(circle, #222, #000);;;",
			"groupKey":    "GRP12345",
			"dryRun":      true,
		})
		pDry, _ := json.Marshal(ToolCallParams{Name: "redbida_apply_onboarding_preset", Arguments: dryArgs})
		respDry, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 7, Method: "tools/call", Params: pDry})
		resDry := respDry.Result.(ToolResult)
		if resDry.IsError {
			t.Fatalf("dryRun failed: %s", resDry.Content[0].Text)
		}
		var dryPayload struct {
			DryRun     bool           `json:"dryRun"`
			Parameters map[string]any `json:"parameters"`
		}
		_ = json.Unmarshal([]byte(resDry.Content[0].Text), &dryPayload)
		if !dryPayload.DryRun {
			t.Errorf("expected dryRun: true")
		}
		// Verify background semicolon stripping
		if dryPayload.Parameters["ui_bg"] != "radial-gradient(circle, #222, #000)" {
			t.Errorf("semicolon not stripped from ui_bg: %v", dryPayload.Parameters["ui_bg"])
		}
		// Verify hashtag sanitization (no tones, no spaces, correct suffixes)
		expectedHashtag := "#CLBBidaHoangGiaSaiGon #BILLIARDSlive #INUTlive #highlightsports"
		if dryPayload.Parameters["custom_hashtags"] != expectedHashtag {
			t.Errorf("hashtag mismatch: got %v, expected %v", dryPayload.Parameters["custom_hashtags"], expectedHashtag)
		}
		// Verify 20-tab INI structure
		tabs := dryPayload.Parameters["ui_tabs_links"].(string)
		if !strings.Contains(tabs, "[C01]") || !strings.Contains(tabs, "[C20]") {
			t.Errorf("ui_tabs_links missing C01 or C20: %s", tabs)
		}
		if !strings.Contains(tabs, "vid_play_label=CLB Bida Hoàng Gia Sài Gòn") {
			t.Errorf("ui_tabs_links missing title in vid_play_label: %s", tabs)
		}
		if dryPayload.Parameters["camera_count"] != float64(12) && dryPayload.Parameters["camera_count"] != 12 {
			t.Errorf("camera_count mismatch: %v", dryPayload.Parameters["camera_count"])
		}

		// Live execution
		liveArgs, _ := json.Marshal(map[string]any{
			"title":       "CLB Bida Hoàng Gia Sài Gòn",
			"cameraCount": 12,
			"bg":          "radial-gradient(circle, #222, #000);",
			"groupKey":    "GRP12345",
			"dryRun":      false,
			"confirmed":   true,
		})
		pLive, _ := json.Marshal(ToolCallParams{Name: "redbida_apply_onboarding_preset", Arguments: liveArgs})
		respLive, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 8, Method: "tools/call", Params: pLive})
		resLive := respLive.Result.(ToolResult)
		if resLive.IsError {
			t.Fatalf("live apply failed: %s", resLive.Content[0].Text)
		}
		if broker.store["ui_title"] != "CLB Bida Hoàng Gia Sài Gòn" {
			t.Errorf("broker store ui_title mismatch: %v", broker.store["ui_title"])
		}
		if broker.store["camera_count"] != 12 {
			t.Errorf("broker store camera_count mismatch: %v", broker.store["camera_count"])
		}
	})

	t.Run("7. redbida_trigger_go2rtc", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_trigger_go2rtc", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 9, Method: "tools/call", Params: p})
		res := resp.Result.(ToolResult)
		if res.IsError {
			t.Fatalf("trigger failed: %s", res.Content[0].Text)
		}
		if broker.store["button_generate_go2rtc_stream"] != true {
			t.Errorf("button_generate_go2rtc_stream flag was not set to true")
		}
	})

	t.Run("8. redbida_get_time_status", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_get_time_status", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 10, Method: "tools/call", Params: p})
		res := resp.Result.(ToolResult)
		if res.IsError {
			t.Fatalf("get time status failed: %s", res.Content[0].Text)
		}
		var payload struct {
			HostTimeRFC3339 string `json:"hostTimeRFC3339"`
			NodeRedReadOnly bool   `json:"nodeRedReadOnly"`
		}
		_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
		if payload.HostTimeRFC3339 == "" || !payload.NodeRedReadOnly {
			t.Errorf("invalid time status response: %+v", payload)
		}
		// Verify RFC 3339 timestamp parses cleanly
		_, err := time.Parse(time.RFC3339, payload.HostTimeRFC3339)
		if err != nil {
			t.Errorf("failed to parse hostTimeRFC3339 %s: %v", payload.HostTimeRFC3339, err)
		}
	})
}

func TestAdversarialChallenge_MalformedJSONRPCAndEdgeCases(t *testing.T) {
	server, _ := setupTestServer(t)
	ctx := context.Background()

	cases := []struct {
		name          string
		rawMsg        string
		expectedCode  int
		expectIsError bool
		checkResp     func(t *testing.T, resp map[string]any)
	}{
		{
			name:         "syntax error: truncated json",
			rawMsg:       `{"jsonrpc":"2.0","id":1,"method":"tools/li`,
			expectedCode: CodeParseError,
		},
		{
			name:         "syntax error: unquoted key",
			rawMsg:       `{jsonrpc:"2.0", id:1}`,
			expectedCode: CodeParseError,
		},
		{
			name:         "syntax error: trailing comma",
			rawMsg:       `{"jsonrpc":"2.0","id":1,"method":"ping",}`,
			expectedCode: CodeParseError,
		},
		{
			name:         "invalid jsonrpc version 1.0",
			rawMsg:       `{"jsonrpc":"1.0","id":1,"method":"ping"}`,
			expectedCode: CodeInvalidRequest,
		},
		{
			name:         "invalid jsonrpc version 3.0",
			rawMsg:       `{"jsonrpc":"3.0","id":1,"method":"ping"}`,
			expectedCode: CodeInvalidRequest,
		},
		{
			name:         "method not found",
			rawMsg:       `{"jsonrpc":"2.0","id":1,"method":"non_existent_method_xyz"}`,
			expectedCode: CodeMethodNotFound,
		},
		{
			name:         "tools/call without params",
			rawMsg:       `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`,
			expectedCode: CodeInvalidParams,
		},
		{
			name:          "tools/call with non-existent tool name",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"unknown_tool_12345"}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res, ok := resp["result"].(map[string]any)
				if !ok || res["isError"] != true {
					t.Errorf("expected result with isError: true for unknown tool name")
				}
			},
		},
		{
			name:          "redbida_apply_onboarding_preset: missing required title",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_apply_onboarding_preset","arguments":{"cameraCount":5}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true when title is missing")
				}
			},
		},
		{
			name:          "redbida_apply_onboarding_preset: whitespace only title",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_apply_onboarding_preset","arguments":{"title":"   ","cameraCount":5}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true when title is whitespace")
				}
			},
		},
		{
			name:          "redbida_apply_onboarding_preset: cameraCount 0 (below min 1)",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_apply_onboarding_preset","arguments":{"title":"Club","cameraCount":0}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true for cameraCount 0")
				}
			},
		},
		{
			name:          "redbida_apply_onboarding_preset: cameraCount 25 (above max 20)",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_apply_onboarding_preset","arguments":{"title":"Club","cameraCount":25}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true for cameraCount 25")
				}
			},
		},
		{
			name:          "redbida_set_keys: empty changes map",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_set_keys","arguments":{"changes":{}}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true for empty changes map")
				}
			},
		},
		{
			name:          "redbida_set_keys: changes invalid type (string instead of map)",
			rawMsg:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"redbida_set_keys","arguments":{"changes":"invalid"}}}`,
			expectIsError: true,
			checkResp: func(t *testing.T, resp map[string]any) {
				res := resp["result"].(map[string]any)
				if res["isError"] != true {
					t.Errorf("expected isError: true for invalid changes type")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			respBytes, isNotif, err := server.ProcessMessage(ctx, []byte(tc.rawMsg))
			if err != nil {
				t.Fatalf("unexpected process error: %v", err)
			}
			if isNotif {
				t.Fatalf("unexpected notification for request")
			}
			var resp map[string]any
			if err := json.Unmarshal(respBytes, &resp); err != nil {
				t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(respBytes))
			}

			if tc.expectedCode != 0 {
				errObj, ok := resp["error"].(map[string]any)
				if !ok {
					t.Fatalf("expected error object, got: %s", string(respBytes))
				}
				code := int(errObj["code"].(float64))
				if code != tc.expectedCode {
					t.Errorf("expected error code %d, got %d (%v)", tc.expectedCode, code, errObj["message"])
				}
			}

			if tc.checkResp != nil {
				tc.checkResp(t, resp)
			}
		})
	}
}

func TestAdversarialChallenge_HTTPAndSSETransports(t *testing.T) {
	server, broker := setupTestServer(t)
	broker.store["ui_title"] = "HTTP Transport Club"
	handler := server.HTTPHandler()

	t.Run("Loopback POST /mcp authorized without key", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":"redbida_get_time_status","arguments":{}}}`
		req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
		req.RemoteAddr = "127.0.0.1:45678"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal JSON-RPC: %v", err)
		}
		if resp.Error != nil {
			t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
		}
	})

	t.Run("Remote POST /mcp rejected without key", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":101,"method":"tools/list"}`
		req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
		req.RemoteAddr = "192.168.1.50:45678"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Remote POST /mcp authorized with X-MCP-Key", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":102,"method":"tools/list"}`
		req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
		req.RemoteAddr = "192.168.1.50:45678"
		req.Header.Set("X-MCP-Key", "test_mcp_key")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with X-MCP-Key, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Remote POST /mcp authorized with Authorization Bearer", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":103,"method":"tools/list"}`
		req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
		req.RemoteAddr = "192.168.1.50:45678"
		req.Header.Set("Authorization", "Bearer test_mcp_key")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with Bearer token, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Remote POST /mcp authorized with query ?key=", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","id":104,"method":"tools/list"}`
		req := httptest.NewRequest("POST", "/mcp?key=test_mcp_key", bytes.NewBufferString(body))
		req.RemoteAddr = "192.168.1.50:45678"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with query key, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SSE Lifecycle GET /mcp and POST /mcp/messages", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sseReq := httptest.NewRequest(http.MethodGet, "/mcp?key=test_mcp_key", nil).WithContext(ctx)
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

		// Send tools/call via POST /mcp/messages?sessionId=...
		callBody := `{"jsonrpc":"2.0","id":200,"method":"tools/call","params":{"name":"redbida_apply_onboarding_preset","arguments":{"title":"SSE Arena","cameraCount":6,"dryRun":true}}}`
		msgReq := httptest.NewRequest(http.MethodPost, "/mcp/messages?sessionId="+sessionID+"&key=test_mcp_key", strings.NewReader(callBody))
		msgReq.RemoteAddr = "192.168.1.50:54321"
		msgRec := httptest.NewRecorder()

		handler.ServeHTTP(msgRec, msgReq)

		if msgRec.Code != http.StatusAccepted && msgRec.Code != http.StatusOK {
			t.Fatalf("expected 200 or 202 from /mcp/messages, got %d: %s", msgRec.Code, msgRec.Body.String())
		}

		// Give SSE goroutine a moment to stream the message
		time.Sleep(50 * time.Millisecond)
		cancel() // close SSE stream

		sseFinalOutput := sseRec.Body.String()
		if !strings.Contains(sseFinalOutput, "event: message") || !strings.Contains(sseFinalOutput, `"id":200`) {
			t.Errorf("expected event: message containing id:200 in SSE output, got:\n%s", sseFinalOutput)
		}
	})
}

func TestAdversarialChallenge_ConcurrentStressAndFuzzing(t *testing.T) {
	server, broker := setupTestServer(t)
	ctx := context.Background()

	t.Run("Concurrent Stress: 50 simultaneous JSON-RPC calls", func(t *testing.T) {
		var wg sync.WaitGroup
		errCh := make(chan error, 50)

		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				var req JSONRPCRequest
				switch idx % 4 {
				case 0:
					req = JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      idx,
						Method:  "tools/list",
					}
				case 1:
					req = JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      fmt.Sprintf("id-str-%d", idx),
						Method:  "tools/call",
						Params:  json.RawMessage(`{"name":"redbida_get_time_status","arguments":{}}`),
					}
				case 2:
					req = JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      idx,
						Method:  "tools/call",
						Params:  json.RawMessage(`{"name":"redbida_list_catalog","arguments":{"group":"Livestream"}}`),
					}
				case 3:
					args, _ := json.Marshal(map[string]any{
						"title":       fmt.Sprintf("Concurrent Arena %d", idx),
						"cameraCount": (idx % 20) + 1,
						"dryRun":      true,
					})
					p, _ := json.Marshal(ToolCallParams{Name: "redbida_apply_onboarding_preset", Arguments: args})
					req = JSONRPCRequest{
						JSONRPC: "2.0",
						ID:      idx,
						Method:  "tools/call",
						Params:  p,
					}
				}

				resp, isNotif := server.ProcessRequest(ctx, req)
				if isNotif {
					errCh <- fmt.Errorf("[%d] expected response, got notification", idx)
					return
				}
				if resp.Error != nil {
					errCh <- fmt.Errorf("[%d] unexpected error: %v", idx, resp.Error)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			t.Errorf("concurrent stress error: %v", err)
		}
	})

	t.Run("Deep Diacritic and Unicode Sanitization Fuzzing", func(t *testing.T) {
		unicodeTitles := []struct {
			input           string
			expectedClean   string
			expectedHashtag string
		}{
			{
				input:           "CLB Bida Hoàng Gia (Thanh Xuân - Hà Nội)",
				expectedClean:   "CLBBidaHoangGiaThanhXuanHaNoi",
				expectedHashtag: "#CLBBidaHoangGiaThanhXuanHaNoi #BILLIARDSlive #INUTlive #highlightsports",
			},
			{
				input:           "Quán Bida & Cà Phê 24/7 🎱🏆",
				expectedClean:   "QuanBidaCaPhe247",
				expectedHashtag: "#QuanBidaCaPhe247 #BILLIARDSlive #INUTlive #highlightsports",
			},
			{
				input:           "Đại Nam Bida Club - Chi Nhánh 2",
				expectedClean:   "DaiNamBidaClubChiNhanh2",
				expectedHashtag: "#DaiNamBidaClubChiNhanh2 #BILLIARDSlive #INUTlive #highlightsports",
			},
		}

		for _, tc := range unicodeTitles {
			t.Run(tc.input, func(t *testing.T) {
				args, _ := json.Marshal(map[string]any{
					"title":       tc.input,
					"cameraCount": 5,
					"dryRun":      true,
				})
				p, _ := json.Marshal(ToolCallParams{Name: "redbida_apply_onboarding_preset", Arguments: args})
				resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: "fuzz", Method: "tools/call", Params: p})
				res := resp.Result.(ToolResult)
				if res.IsError {
					t.Fatalf("preset failed for %s: %s", tc.input, res.Content[0].Text)
				}
				var payload struct {
					Parameters map[string]any `json:"parameters"`
				}
				_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
				if payload.Parameters["custom_hashtags"] != tc.expectedHashtag {
					t.Errorf("for input %q: got hashtag %q, want %q", tc.input, payload.Parameters["custom_hashtags"], tc.expectedHashtag)
				}
				tabs := payload.Parameters["ui_tabs_links"].(string)
				if !strings.Contains(tabs, "vid_play_label="+tc.input) {
					t.Errorf("for input %q: ui_tabs_links does not contain verbatim title in vid_play_label", tc.input)
				}
			})
		}
	})

	t.Run("Policy Verification: Rejection of Unconfirmed Maintenance Keys via set_keys", func(t *testing.T) {
		// button_reboot is a confirmation-gated key
		args, _ := json.Marshal(map[string]any{
			"changes": map[string]any{
				"button_reboot": true,
			},
			"confirmed": false,
		})
		p, _ := json.Marshal(ToolCallParams{Name: "redbida_set_keys", Arguments: args})
		resp, _ := server.ProcessRequest(ctx, JSONRPCRequest{JSONRPC: "2.0", ID: 300, Method: "tools/call", Params: p})
		res := resp.Result.(ToolResult)
		var payload struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
		if len(payload.Results) == 0 || payload.Results[0].Applied || payload.Results[0].Error == "" {
			t.Errorf("expected rejection of unconfirmed button_reboot, got: %+v", payload.Results)
		}
	})

	_ = broker
}
