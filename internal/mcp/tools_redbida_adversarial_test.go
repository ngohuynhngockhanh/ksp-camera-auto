package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

// flexibleMockBroker allows simulating fine-grained broker behaviors such as
// timeouts, partial acknowledgements, corrupted read-backs, and network dropouts.
type flexibleMockBroker struct {
	mu          sync.Mutex
	store       map[string]any
	readBackVal map[string]any // if non-nil, overrides store during reads
	readErr     error
	writeErr    error
	partialAcks map[string]bool // if set, only acknowledged if true
	writeCalls  int32
	readCalls   int32
}

func newFlexibleBroker(initStore map[string]any) *flexibleMockBroker {
	st := make(map[string]any)
	for k, v := range initStore {
		st[k] = v
	}
	return &flexibleMockBroker{
		store: st,
	}
}

func (f *flexibleMockBroker) Read(_ context.Context, keys []string) (map[string]any, error) {
	atomic.AddInt32(&f.readCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.readErr != nil {
		return nil, f.readErr
	}

	res := make(map[string]any)
	for _, k := range keys {
		if f.readBackVal != nil {
			if v, ok := f.readBackVal[k]; ok {
				res[k] = v
			}
			continue
		}
		if v, ok := f.store[k]; ok {
			res[k] = v
		}
	}
	return res, nil
}

func (f *flexibleMockBroker) Write(_ context.Context, changes map[string]any) (map[string]redbida.WriteAck, error) {
	atomic.AddInt32(&f.writeCalls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writeErr != nil {
		return nil, f.writeErr
	}

	acks := make(map[string]redbida.WriteAck)
	for k, v := range changes {
		oldVal := f.store[k]
		f.store[k] = v

		if f.partialAcks != nil && !f.partialAcks[k] {
			// Skip returning ack for this key
			continue
		}

		acks[k] = redbida.WriteAck{
			OldValue: oldVal,
			NewValue: v,
		}
	}
	return acks, nil
}

func setupAdvTestEnv(t *testing.T, broker redbida.Broker) (*Registry, *redbida.Service) {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"ui_title", "company_name", "ui_bg", "custom_hashtags", "ui_tabs_links",
		"camera_count", "toolbar_show_count", "video_config", "hls_using_go2rtc",
		"hls_using_go2rtc_livestream", "hls_using_go2rtc_tiktok", "ui_scoreboard",
		"logo_header", "logo_header_text", "button_generate_go2rtc_stream",
		"mqtt_password", "shinobi_token", "shinobi_camera_id", "shinobi_group_key",
		"ggcode", "max_free_ram_restart_camera", "max_free_ram_force_reboot",
		"restart_camera_now", "reboot_now", "frpc_config", "db_check_range",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("init"), 0o600); err != nil {
			t.Fatalf("failed to setup test catalog: %v", err)
		}
	}

	cat := redbida.NewCatalog(dir)
	svc := redbida.NewService(broker, cat, 200)
	reg := NewRegistry()
	cfg := config.Default()
	registerRedbidaTools(reg, &cfg, svc)

	return reg, svc
}

// 1. Stress-test broker timeout and disconnect errors on Read & Write
func TestAdversarial_BrokerTimeout_ReadAndWrite(t *testing.T) {
	ctx := context.Background()

	// 1.1 Broker Read Timeout / Error
	broker := newFlexibleBroker(map[string]any{"ui_title": "Original"})
	broker.readErr = context.DeadlineExceeded

	reg, _ := setupAdvTestEnv(t, broker)

	getRes, err := reg.Call(ctx, "redbida_get_keys", []byte(`{"keys":["ui_title"]}`))
	if err == nil && !getRes.IsError {
		t.Fatalf("expected error on broker read timeout, got success: %+v", getRes)
	}
	if !getRes.IsError || !strings.Contains(getRes.Content[0].Text, "context deadline exceeded") {
		t.Errorf("expected deadline exceeded message in error, got: %+v", getRes)
	}

	// 1.2 Broker Write Generic Network Error
	broker = newFlexibleBroker(map[string]any{"ui_title": "Original"})
	broker.writeErr = errors.New("connection reset by peer")
	reg, _ = setupAdvTestEnv(t, broker)

	setRes, err := reg.Call(ctx, "redbida_set_keys", []byte(`{"changes":{"ui_title":"Updated"}}`))
	if err == nil && !setRes.IsError {
		t.Fatalf("expected error on broker write network error, got success: %+v", setRes)
	}
	if !setRes.IsError || !strings.Contains(setRes.Content[0].Text, "connection reset by peer") {
		t.Errorf("expected connection reset error, got: %+v", setRes)
	}
}

// 2. Stress-test broker ACK timeout: recovery via read-back vs fail-closed
func TestAdversarial_BrokerAckTimeout_RecoveryAndFailure(t *testing.T) {
	ctx := context.Background()

	// Scenario A: Write ACK times out, but device DID apply change. Read-back recovers and verifies!
	brokerA := newFlexibleBroker(map[string]any{"ui_title": "OldTitle"})
	brokerA.writeErr = &redbida.AckTimeoutError{Topic: "/private/i_sets/ack"}
	// Simulate that the store updated anyway
	brokerA.store["ui_title"] = "RecoveredTitle"

	regA, _ := setupAdvTestEnv(t, brokerA)

	setResA, err := regA.Call(ctx, "redbida_set_keys", []byte(`{"changes":{"ui_title":"RecoveredTitle"}}`))
	if err != nil || setResA.IsError {
		t.Fatalf("expected ACK timeout recovery via read-back, got error: %+v", setResA)
	}

	var payloadA struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(setResA.Content[0].Text), &payloadA)
	if len(payloadA.Results) == 0 || !payloadA.Results[0].Verified || !payloadA.Results[0].Applied {
		t.Errorf("expected Verified=true and Applied=true upon recovery, got: %+v", payloadA.Results)
	}

	// Scenario B: Write ACK times out and device did NOT apply change (read-back mismatch). Must fail closed!
	brokerB := newFlexibleBroker(map[string]any{"ui_title": "OldTitle"})
	brokerB.writeErr = &redbida.AckTimeoutError{Topic: "/private/i_sets/ack"}
	// Store remains OldTitle

	regB, _ := setupAdvTestEnv(t, brokerB)

	setResB, _ := regB.Call(ctx, "redbida_set_keys", []byte(`{"changes":{"ui_title":"NewTitle"}}`))
	var payloadB struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(setResB.Content[0].Text), &payloadB)
	if len(payloadB.Results) == 0 {
		t.Fatalf("expected results in payloadB")
	}
	resB := payloadB.Results[0]
	if resB.Applied || resB.Verified {
		t.Errorf("expected Applied=false and Verified=false on read-back mismatch, got: %+v", resB)
	}
	if !strings.Contains(resB.Error, "write acknowledgement timed out; read-back mismatch") {
		t.Errorf("expected read-back mismatch error, got %q", resB.Error)
	}

	// Scenario C: Write ACK times out and read-back also fails (e.g. read error). Must fail closed!
	brokerC := newFlexibleBroker(map[string]any{"ui_title": "OldTitle"})
	brokerC.writeErr = &redbida.AckTimeoutError{Topic: "/private/i_sets/ack"}
	brokerC.readErr = errors.New("mqtt client disconnected")

	regC, _ := setupAdvTestEnv(t, brokerC)

	setResC, _ := regC.Call(ctx, "redbida_set_keys", []byte(`{"changes":{"ui_title":"NewTitle"}}`))
	var payloadC struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(setResC.Content[0].Text), &payloadC)
	if len(payloadC.Results) == 0 {
		t.Fatalf("expected results in payloadC")
	}
	resC := payloadC.Results[0]
	if resC.Applied || resC.Verified {
		t.Errorf("expected Applied=false and Verified=false on read failure, got: %+v", resC)
	}
	if !strings.Contains(resC.Error, "read-back failed") {
		t.Errorf("expected read-back failed in error message, got: %q", resC.Error)
	}
}

// 3. Stress-test partial ACK from broker & corrupted/stale read-back
func TestAdversarial_PartialAcks_And_CorruptedReadBack(t *testing.T) {
	ctx := context.Background()

	// Broker acknowledges "ui_title" but NOT "company_name"
	broker := newFlexibleBroker(map[string]any{
		"ui_title":     "Old Title",
		"company_name": "Old Company",
		"video_config": "range=24",
	})
	broker.partialAcks = map[string]bool{
		"ui_title":     true,
		"company_name": false, // will not receive write ack
		"video_config": true,
	}
	// For video_config, simulate corrupted read-back returning stale "range=24"
	broker.readBackVal = map[string]any{
		"ui_title":     "New Title",
		"company_name": "New Company",
		"video_config": "range=24", // mismatch against desired "range=72"
	}

	reg, _ := setupAdvTestEnv(t, broker)

	setArgs := map[string]any{
		"changes": map[string]any{
			"ui_title":     "New Title",
			"company_name": "New Company",
			"video_config": "range=72",
		},
	}
	setJSON, _ := json.Marshal(setArgs)

	res, err := reg.Call(ctx, "redbida_set_keys", setJSON)
	if err != nil || res.IsError {
		t.Fatalf("expected call to execute, got error: %+v", res)
	}

	var payload struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)

	resMap := make(map[string]redbida.ChangeResult)
	for _, r := range payload.Results {
		resMap[r.Key] = r
	}

	// 1. ui_title: acked + readback matched -> Success
	if r, ok := resMap["ui_title"]; !ok || !r.Applied || !r.Verified || r.Error != "" {
		t.Errorf("ui_title should have succeeded: %+v", r)
	}

	// 2. company_name: missing ack -> Failure
	if r, ok := resMap["company_name"]; !ok || r.Applied || r.Verified || !strings.Contains(r.Error, "missing acknowledgement") {
		t.Errorf("company_name should have failed with missing acknowledgement: %+v", r)
	}

	// 3. video_config: acked, but read-back mismatch -> Failure
	if r, ok := resMap["video_config"]; !ok || r.Applied || r.Verified || !strings.Contains(r.Error, "read-back mismatch") {
		t.Errorf("video_config should have failed with read-back mismatch: %+v", r)
	}
}

// 4. Stress-test confirmation enforcement and protected read-only keys
func TestAdversarial_ConfirmationEnforcement_And_ProtectedKeys(t *testing.T) {
	ctx := context.Background()
	broker := newFlexibleBroker(map[string]any{
		"ui_title":                    "Normal Title",
		"max_free_ram_restart_camera": 500000000,
		"reboot_now":                  false,
		"frpc_config":                 "[common]\nserver_addr = 1.2.3.4",
	})

	reg, _ := setupAdvTestEnv(t, broker)

	// 4.1 Unconfirmed write to confirm-required maintenance key
	unconfArgs := map[string]any{
		"changes": map[string]any{
			"max_free_ram_restart_camera": 600000000,
		},
		"confirmed": false,
	}
	unconfJSON, _ := json.Marshal(unconfArgs)
	res, _ := reg.Call(ctx, "redbida_set_keys", unconfJSON)

	var unconfPayload struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(res.Content[0].Text), &unconfPayload)
	if len(unconfPayload.Results) == 0 {
		t.Fatalf("expected results for unconfirmed write")
	}
	r := unconfPayload.Results[0]
	if r.Applied || r.Verified || !strings.Contains(r.Error, "confirmation is required") {
		t.Errorf("expected confirmation is required error, got: %+v", r)
	}
	if atomic.LoadInt32(&broker.writeCalls) != 0 {
		t.Errorf("broker write should not have been called for unconfirmed write")
	}

	// 4.2 Confirmed write to confirm-required key
	confArgs := map[string]any{
		"changes": map[string]any{
			"max_free_ram_restart_camera": 600000000,
		},
		"confirmed": true,
	}
	confJSON, _ := json.Marshal(confArgs)
	resConf, err := reg.Call(ctx, "redbida_set_keys", confJSON)
	if err != nil || resConf.IsError {
		t.Fatalf("expected confirmed write to succeed, got: %+v", resConf)
	}
	var confPayload struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(resConf.Content[0].Text), &confPayload)
	if len(confPayload.Results) == 0 || !confPayload.Results[0].Applied || !confPayload.Results[0].Verified {
		t.Errorf("expected confirmed write to be applied and verified: %+v", confPayload.Results)
	}

	// 4.3 Attempt write to read-only / protected key (e.g. frpc_config)
	roArgs := map[string]any{
		"changes": map[string]any{
			"frpc_config": "[common]\nserver_addr = 9.9.9.9",
		},
		"confirmed": true,
	}
	roJSON, _ := json.Marshal(roArgs)
	resRO, _ := reg.Call(ctx, "redbida_set_keys", roJSON)
	var roPayload struct {
		Results []redbida.ChangeResult `json:"results"`
	}
	_ = json.Unmarshal([]byte(resRO.Content[0].Text), &roPayload)
	if len(roPayload.Results) == 0 {
		t.Fatalf("expected results for read-only write")
	}
	if roPayload.Results[0].Applied || !strings.Contains(roPayload.Results[0].Error, "key is read-only") {
		t.Errorf("expected key is read-only error for frpc_config, got: %+v", roPayload.Results[0])
	}
}

// 5. Stress-test 1-Click Onboarding preset with extreme/adversarial inputs
func TestAdversarial_OnboardingPreset_ExtremeInputs(t *testing.T) {
	ctx := context.Background()
	broker := newFlexibleBroker(map[string]any{})
	reg, _ := setupAdvTestEnv(t, broker)

	// 5.1 Complex Vietnamese title with mixed diacritics, symbols, punctuation
	complexTitle := "  CLB Bida Sài Gòn Đệ Nhất - CS3 & CS4 (Phú Nhuận) #2026 !  "
	presetArgs := map[string]any{
		"title":       complexTitle,
		"cameraCount": 16,
		"bg":          "linear-gradient(135deg, #02254e 0%, #040410 90%); ; ; ; \t\n",
		"groupKey":    "  AWU8wJMd2l  ",
		"dryRun":      true,
	}
	presetJSON, _ := json.Marshal(presetArgs)
	res, err := reg.Call(ctx, "redbida_apply_onboarding_preset", presetJSON)
	if err != nil || res.IsError {
		t.Fatalf("preset dryRun failed on complex Vietnamese title: %+v", res)
	}

	var payload struct {
		Parameters map[string]any `json:"parameters"`
	}
	_ = json.Unmarshal([]byte(res.Content[0].Text), &payload)
	params := payload.Parameters

	// Semicolons stripped
	if strings.Contains(params["ui_bg"].(string), ";") {
		t.Errorf("ui_bg must not contain semicolon: %q", params["ui_bg"])
	}

	// Hashtag normalized without diacritics
	expectedHashtagPrefix := "Tìm hiểu thêm tại BilliardLive.IO.VN\n#CLBBidaSaiGonDeNhatCS3CS4PhuNhuan2026 #BILLIARDSlive"
	if !strings.HasPrefix(params["custom_hashtags"].(string), expectedHashtagPrefix) {
		t.Errorf("expected hashtag to start with %q, got %q", expectedHashtagPrefix, params["custom_hashtags"])
	}

	// Title trimmed
	trimmedTitle := strings.TrimSpace(complexTitle)
	if params["ui_title"] != trimmedTitle || params["company_name"] != trimmedTitle {
		t.Errorf("expected trimmed title %q, got ui_title=%q, company_name=%q", trimmedTitle, params["ui_title"], params["company_name"])
	}

	// INI tabs contain 20 sections with trimmed title
	iniTabs := params["ui_tabs_links"].(string)
	if !strings.Contains(iniTabs, "[C01]") || !strings.Contains(iniTabs, "[C20]") {
		t.Errorf("ui_tabs_links missing [C01] or [C20]")
	}
	if !strings.Contains(iniTabs, "vid_play_label="+trimmedTitle) {
		t.Errorf("ui_tabs_links should have vid_play_label=%s", trimmedTitle)
	}

	// Group key mapped
	if params["shinobi_camera_id"] != "AWU8wJMd2l" || params["shinobi_group_key"] != "AWU8wJMd2l" {
		t.Errorf("groupKey mapping mismatch: camera_id=%v, group_key=%v", params["shinobi_camera_id"], params["shinobi_group_key"])
	}

	// 5.2 Pure Emoji/Symbol Title fallback
	symbolTitle := "✨⭐🎉🚀"
	resSym, _ := reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(fmt.Sprintf(`{"title":%q,"cameraCount":5,"dryRun":true}`, symbolTitle)))
	var payloadSym struct {
		Parameters map[string]any `json:"parameters"`
	}
	_ = json.Unmarshal([]byte(resSym.Content[0].Text), &payloadSym)
	if payloadSym.Parameters["custom_hashtags"] != "Tìm hiểu thêm tại BilliardLive.IO.VN\n#BILLIARDSlive #INUTlive #highlightsports" {
		t.Errorf("pure symbol title should fallback to standard hashtags, got: %v", payloadSym.Parameters["custom_hashtags"])
	}

	// 5.3 Extreme cameraCount boundaries
	invalidCounts := []int{-10, 0, 21, 100, 9999}
	for _, cc := range invalidCounts {
		invRes, _ := reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(fmt.Sprintf(`{"title":"Test","cameraCount":%d}`, cc)))
		if !invRes.IsError || !strings.Contains(invRes.Content[0].Text, "cameraCount must be between 1 and 20") {
			t.Errorf("expected cameraCount error for %d, got: %+v", cc, invRes)
		}
	}
}

// 6. Concurrency stress test: 50 concurrent goroutines executing mixed tool calls
func TestAdversarial_ConcurrencyStress(t *testing.T) {
	ctx := context.Background()
	broker := newFlexibleBroker(map[string]any{
		"ui_title":     "Initial Title",
		"camera_count": 8,
		"video_config": "range=72",
	})
	reg, _ := setupAdvTestEnv(t, broker)

	var wg sync.WaitGroup
	workers := 50
	iterations := 10

	errCh := make(chan error, workers*iterations)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				switch (workerID + j) % 6 {
				case 0:
					// list_catalog
					res, err := reg.Call(ctx, "redbida_list_catalog", []byte(`{"editableOnly":true}`))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d list_catalog failed: %v, %+v", workerID, err, res)
					}
				case 1:
					// get_keys
					res, err := reg.Call(ctx, "redbida_get_keys", []byte(`{"keys":["ui_title","camera_count"]}`))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d get_keys failed: %v, %+v", workerID, err, res)
					}
				case 2:
					// set_keys
					title := fmt.Sprintf("Title W%d-%d", workerID, j)
					setArgs := fmt.Sprintf(`{"changes":{"ui_title":%q}}`, title)
					res, err := reg.Call(ctx, "redbida_set_keys", []byte(setArgs))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d set_keys failed: %v, %+v", workerID, err, res)
					}
				case 3:
					// apply_onboarding_preset dryRun
					venue := fmt.Sprintf("Venue %d", workerID)
					presetArgs := fmt.Sprintf(`{"title":%q,"cameraCount":10,"dryRun":true}`, venue)
					res, err := reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(presetArgs))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d apply_preset dryRun failed: %v, %+v", workerID, err, res)
					}
				case 4:
					// trigger_go2rtc
					res, err := reg.Call(ctx, "redbida_trigger_go2rtc", []byte(`{}`))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d trigger_go2rtc failed: %v, %+v", workerID, err, res)
					}
				case 5:
					// get_time_status
					res, err := reg.Call(ctx, "redbida_get_time_status", []byte(`{}`))
					if err != nil || res.IsError {
						errCh <- fmt.Errorf("worker %d get_time_status failed: %v, %+v", workerID, err, res)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrency error: %v", err)
	}
}

// 7. Full JSON-RPC 2.0 ProcessRequest lifecycle validation
func TestAdversarial_JSONRPC20_Integration(t *testing.T) {
	broker := newFlexibleBroker(map[string]any{
		"ui_title":     "JSON-RPC Venue",
		"camera_count": 12,
	})
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "ui_title"), []byte("init"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "camera_count"), []byte("init"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "button_generate_go2rtc_stream"), []byte("init"), 0o600)

	cat := redbida.NewCatalog(dir)
	svc := redbida.NewService(broker, cat, 200)

	cfg := config.Default()
	invFile := filepath.Join(dir, "cameras.yaml")
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	srv := NewServer(&cfg, inv, nil)
	// Register redbida tools on server registry
	registerRedbidaTools(srv.Registry(), &cfg, svc)

	ctx := context.Background()

	// 7.1 tools/list contains all 6 redbida tools
	reqList := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/list",
	}
	respList, isNotif := srv.ProcessRequest(ctx, reqList)
	if isNotif || respList.Error != nil {
		t.Fatalf("tools/list failed: %+v", respList)
	}
	toolsListRes, ok := respList.Result.(ToolsListResult)
	if !ok {
		t.Fatalf("unexpected type for tools/list: %T", respList.Result)
	}
	toolNames := make(map[string]bool)
	for _, tool := range toolsListRes.Tools {
		toolNames[tool.Name] = true
	}
	expectedRedbidaTools := []string{
		"redbida_list_catalog",
		"redbida_get_keys",
		"redbida_set_keys",
		"redbida_apply_onboarding_preset",
		"redbida_trigger_go2rtc",
		"redbida_get_time_status",
	}
	for _, expected := range expectedRedbidaTools {
		if !toolNames[expected] {
			t.Errorf("tools/list missing expected redbida tool: %s", expected)
		}
	}

	// 7.2 tools/call: redbida_apply_onboarding_preset via JSON-RPC
	callParams, _ := json.Marshal(ToolCallParams{
		Name: "redbida_apply_onboarding_preset",
		Arguments: json.RawMessage(`{
			"title": "Bida VIP JSON-RPC",
			"cameraCount": 8,
			"dryRun": true
		}`),
	})
	reqCall := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      2,
		Method:  "tools/call",
		Params:  callParams,
	}
	respCall, _ := srv.ProcessRequest(ctx, reqCall)
	if respCall.Error != nil {
		t.Fatalf("tools/call failed: %+v", respCall.Error)
	}
	tr, ok := respCall.Result.(ToolResult)
	if !ok || tr.IsError {
		t.Fatalf("expected successful ToolResult, got: %+v", respCall.Result)
	}
	if !strings.Contains(tr.Content[0].Text, "Bida VIP JSON-RPC") {
		t.Errorf("expected response to contain title, got: %s", tr.Content[0].Text)
	}
}
