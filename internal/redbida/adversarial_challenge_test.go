package redbida

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type challengeMockBroker struct {
	mu         sync.Mutex
	reads      [][]string
	writes     []map[string]any
	readValues map[string]any
	writeAcks  map[string]WriteAck
	writeErr   error
	readErr    error
}

func (m *challengeMockBroker) Read(ctx context.Context, keys []string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads = append(m.reads, append([]string(nil), keys...))
	if m.readErr != nil {
		return nil, m.readErr
	}
	res := make(map[string]any)
	for _, k := range keys {
		if v, ok := m.readValues[k]; ok {
			res[k] = v
		}
	}
	return res, nil
}

func (m *challengeMockBroker) Write(ctx context.Context, values map[string]any) (map[string]WriteAck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make(map[string]any)
	for k, v := range values {
		copied[k] = v
		if m.readValues != nil {
			m.readValues[k] = v
		}
	}
	m.writes = append(m.writes, copied)
	if m.writeErr != nil {
		return nil, m.writeErr
	}
	acks := make(map[string]WriteAck)
	for k, v := range values {
		if ack, ok := m.writeAcks[k]; ok {
			acks[k] = ack
		} else {
			acks[k] = WriteAck{OldValue: "prev", NewValue: v}
		}
	}
	return acks, nil
}

// 1. Adversarial stress test for toolbar_show_count
func TestAdversarialToolbarShowCount(t *testing.T) {
	meta := metaForKey("toolbar_show_count", "", "")

	// A. Metadata assertions
	if !meta.Editable {
		t.Errorf("toolbar_show_count must be editable")
	}
	if meta.Risk != RiskEditable {
		t.Errorf("toolbar_show_count risk = %v, want RiskEditable", meta.Risk)
	}
	if meta.ValueType != TypeNumber {
		t.Errorf("toolbar_show_count valueType = %v, want TypeNumber", meta.ValueType)
	}
	if meta.Group != "Livestream" {
		t.Errorf("toolbar_show_count group = %q, want Livestream", meta.Group)
	}

	// B. Valid boundary values
	validCases := []struct {
		name  string
		input any
	}{
		{"min zero int", 0},
		{"min zero int64", int64(0)},
		{"min zero float64", float64(0)},
		{"max 4096 int", 4096},
		{"max 4096 int64", int64(4096)},
		{"max 4096 float64", float64(4096)},
		{"mid range 8", 8},
		{"mid range 16", 16},
		{"mid range 20", 20},
		{"float integer 100.0", float64(100.0)},
		{"uint32 8", uint32(8)},
	}

	for _, tc := range validCases {
		t.Run("valid_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.input); err != nil {
				t.Errorf("valid input %v (%T) rejected: %v", tc.input, tc.input, err)
			}
		})
	}

	// C. Adversarial invalid inputs: boundaries, types, NaNs, infinities
	invalidCases := []struct {
		name  string
		input any
	}{
		{"negative -1", -1},
		{"negative int64 -100", int64(-100)},
		{"negative float -0.001", -0.001},
		{"overflow 4097", 4097},
		{"huge overflow 1000000", 1000000},
		{"max float overflow math.MaxFloat64", math.MaxFloat64},
		{"non-integer float 0.5", 0.5},
		{"non-integer float 7.9999", 7.9999},
		{"non-integer float 4095.9", 4095.9},
		{"NaN", math.NaN()},
		{"Positive Inf", math.Inf(1)},
		{"Negative Inf", math.Inf(-1)},
		{"string number '8'", "8"},
		{"empty string ''", ""},
		{"boolean true", true},
		{"boolean false", false},
		{"nil value", nil},
		{"slice value", []int{8}},
		{"map value", map[string]int{"count": 8}},
	}

	for _, tc := range invalidCases {
		t.Run("invalid_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.input); err == nil {
				t.Errorf("invalid input %v (%T) was incorrectly accepted", tc.input, tc.input)
			}
		})
	}
}

// 2. Adversarial stress test for custom_hashtags
func TestAdversarialCustomHashtags(t *testing.T) {
	meta := metaForKey("custom_hashtags", "", "")

	if !meta.Editable || meta.Risk != RiskEditable || meta.ValueType != TypeString || meta.Group != "Branding / Logo" {
		t.Fatalf("unexpected custom_hashtags metadata: %+v", meta)
	}

	validStrings := []struct {
		name string
		val  string
	}{
		{"empty string", ""},
		{"single space", " "},
		{"standard tags", "#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports"},
		{"vietnamese tags with diacritics", "#BidaĐỉnhCao #QuánBidaXịn #SàiGònBida #ViệtNamVôĐịch"},
		{"emojis and unicode symbols", "#Billiards🎱 #Fire🔥 #Live🔴 #Trophy🏆 #100%_Accuracy🎯"},
		{"multiline tags", "#Tag1\n#Tag2\r\n#Tag3\t#Tag4"},
		{"long hashtag string 100KB", strings.Repeat("#HashtagLongName123 ", 5000)},
		{"max allowed size 2MB", strings.Repeat("A", 2*1024*1024)},
	}

	for _, tc := range validStrings {
		t.Run("valid_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.val); err != nil {
				t.Errorf("valid custom_hashtags %s rejected: %v", tc.name, err)
			}
		})
	}

	// Adversarial: oversized > 2MB and non-string types
	t.Run("oversized_string_rejected", func(t *testing.T) {
		oversized := strings.Repeat("B", 2*1024*1024+1)
		if err := validateValue(meta, oversized); err == nil {
			t.Errorf("oversized string >2MB should have been rejected")
		}
	})

	invalidTypes := []struct {
		name string
		val  any
	}{
		{"nil", nil},
		{"int 123", 123},
		{"bool true", true},
		{"slice of strings", []string{"#tag1", "#tag2"}},
		{"map of strings", map[string]string{"tag": "#tag1"}},
		{"json array any", []any{"#tag1"}},
	}

	for _, tc := range invalidTypes {
		t.Run("invalid_type_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.val); err == nil {
				t.Errorf("non-string type %v (%T) was incorrectly accepted", tc.val, tc.val)
			}
		})
	}
}

// 3. Adversarial stress test for ui_tabs_links
func TestAdversarialUiTabsLinks(t *testing.T) {
	meta := metaForKey("ui_tabs_links", "", "")

	if !meta.Editable || meta.Risk != RiskEditable || meta.ValueType != TypeString || meta.Group != "UI / Display" {
		t.Fatalf("unexpected ui_tabs_links metadata: %+v", meta)
	}

	// Standard 20-section INI generator
	var sb strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&sb, "[C%02d]\n", i)
		sb.WriteString("stream_label=Video Trực tiếp\n")
		sb.WriteString("vid_list_label=Danh sách highlight\n")
		sb.WriteString("vid_play_label=SD Billiards Club - CS2\n")
		sb.WriteString("list_refresh_label=Cập nhật highlight\n\n")
	}
	full20TabsINI := sb.String()

	validInputs := []struct {
		name string
		val  string
	}{
		{"full 20-tab INI", full20TabsINI},
		{"empty string", ""},
		{"single tab", "[C01]\nstream_label=Live\nvid_play_label=Test"},
		{"with CRLF windows endings", strings.ReplaceAll(full20TabsINI, "\n", "\r\n")},
		{"vietnamese utf8 special symbols", "[C01]\nstream_label=Bàn VIP 1 (Số 1) — Trực tiếp 🔴\nvid_play_label=Câu Lạc Bộ Bida Hoàng Gia ✨\n"},
		{"100KB large INI text", strings.Repeat("[C01]\nlabel=Test\n", 5000)},
	}

	for _, tc := range validInputs {
		t.Run("valid_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.val); err != nil {
				t.Errorf("valid ui_tabs_links %s rejected: %v", tc.name, err)
			}
		})
	}

	// Adversarial: oversized > 2MB and non-string types
	t.Run("oversized_ini_rejected", func(t *testing.T) {
		oversized := strings.Repeat("[C01]\nstream_label=Live\n", 100000)
		if len(oversized) > 2*1024*1024 {
			if err := validateValue(meta, oversized); err == nil {
				t.Errorf("oversized INI >2MB should have been rejected")
			}
		}
	})

	invalidTypes := []struct {
		name string
		val  any
	}{
		{"nil", nil},
		{"int", 42},
		{"bool", false},
		{"json map", map[string]any{"C01": map[string]string{"stream_label": "Live"}}},
		{"slice", []string{"[C01]"}},
	}

	for _, tc := range invalidTypes {
		t.Run("invalid_type_"+tc.name, func(t *testing.T) {
			if err := validateValue(meta, tc.val); err == nil {
				t.Errorf("non-string type %v (%T) was incorrectly accepted", tc.val, tc.val)
			}
		})
	}
}

// 4. Adversarial security tests for shinobi_group_key
func TestAdversarialShinobiGroupKeySecurity(t *testing.T) {
	tempDir := t.TempDir()
	catalog := NewCatalog(tempDir)

	// A. Fallback and direct discovery metadata check
	meta, ok := catalog.Meta("shinobi_group_key")
	if !ok {
		t.Fatalf("shinobi_group_key MUST be in catalog")
	}
	if meta.Editable {
		t.Errorf("SECURITY VIOLATION: shinobi_group_key must NEVER be editable")
	}
	if !meta.Secret {
		t.Errorf("SECURITY VIOLATION: shinobi_group_key must be marked Secret: true")
	}
	if meta.Risk != RiskProtected {
		t.Errorf("SECURITY VIOLATION: shinobi_group_key risk must be RiskProtected, got %v", meta.Risk)
	}
	if meta.Group != "Security / Credentials" {
		t.Errorf("shinobi_group_key group = %q, want Security / Credentials", meta.Group)
	}

	// B. Attempt mutation via Service.Apply — must be rejected without calling Broker.Write
	broker := &challengeMockBroker{
		readValues: map[string]any{"shinobi_group_key": "SECRET1234"},
	}
	svc := NewService(broker, catalog, 20)

	// Attack 1: Unconfirmed apply
	results, err := svc.Apply(context.Background(), map[string]any{
		"shinobi_group_key": "HACKED_KEY",
	}, false)
	if err != nil {
		t.Fatalf("unexpected error from Apply: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Applied || results[0].Verified || results[0].Error == "" {
		t.Fatalf("SECURITY VIOLATION: shinobi_group_key was applied or lacked error: %+v", results[0])
	}
	if len(broker.writes) != 0 {
		t.Fatalf("SECURITY VIOLATION: broker.Write was called with protected key: %+v", broker.writes)
	}

	// Attack 2: Confirmed apply
	results, err = svc.Apply(context.Background(), map[string]any{
		"shinobi_group_key": "HACKED_KEY",
	}, true)
	if err != nil {
		t.Fatalf("unexpected error from Apply: %v", err)
	}
	if results[0].Applied || results[0].Verified || results[0].Error != "key is read-only" {
		t.Fatalf("SECURITY VIOLATION: confirmed apply did not reject read-only key: %+v", results[0])
	}
	if len(broker.writes) != 0 {
		t.Fatalf("SECURITY VIOLATION: broker.Write was called during confirmed apply: %+v", broker.writes)
	}

	// C. Refresh redaction test: value should be redacted
	if err := os.WriteFile(filepath.Join(tempDir, "shinobi_group_key"), []byte("ACTUAL_SECRET_KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	broker.readValues["shinobi_group_key"] = "ACTUAL_SECRET_KEY"
	kvs, err := svc.Refresh(context.Background(), []string{"shinobi_group_key"})
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if len(kvs) != 1 {
		t.Fatalf("expected 1 kv, got %d", len(kvs))
	}
	// Redacted secret should NOT be the raw secret string
	if kvs[0].Value == "ACTUAL_SECRET_KEY" {
		t.Errorf("SECURITY VIOLATION: secret shinobi_group_key was returned unredacted: %v", kvs[0].Value)
	}
}

// 5. Adversarial domain grouping classifications stress test
func TestAdversarialDomainGroupingCompleteness(t *testing.T) {
	validGroups := map[string]bool{
		"Branding / Logo":        true,
		"Livestream":             true,
		"UI / Display":           true,
		"Schedule / Maintenance": true,
		"Security / Credentials": true,
		"Network / MQTT":         true,
		"Advanced / Unknown":     true,
	}

	catalog := NewCatalog(t.TempDir() + "/missing")
	list := catalog.List()

	groupCounts := make(map[string]int)

	for _, item := range list {
		groupCounts[item.Group]++
		if !validGroups[item.Group] {
			t.Errorf("key %s has invalid group %q", item.Key, item.Group)
		}
	}

	// Verify that standard business keys are classified into their proper domain groups (NOT Advanced / Unknown)
	mustBeDomainGrouped := []string{
		"toolbar_show_count", "custom_hashtags", "ui_tabs_links", "shinobi_group_key",
		"camera_count", "video_config", "hls_using_go2rtc", "button_generate_go2rtc_stream",
		"logo_header", "logo_header_text", "logo_livestream", "company_name",
		"ui_title", "ui_bg", "ui_scoreboard", "ui_css_custom",
		"stop_camera_00h05", "button_reboot", "button_restart_shinobi",
		"shinobi_camera_id", "shinobi_token", "shinobi_monitor_token", "ggcode", "frpc_config",
	}

	for _, k := range mustBeDomainGrouped {
		meta, ok := catalog.Meta(k)
		if !ok {
			t.Errorf("critical key %s missing from catalog", k)
			continue
		}
		if meta.Group == "Advanced / Unknown" {
			t.Errorf("critical key %s was not assigned a domain group (fell through to Advanced / Unknown)", k)
		}
	}

	// Ensure all 5 core domain groups have non-zero representation
	coreGroups := []string{
		"Branding / Logo",
		"Livestream",
		"UI / Display",
		"Schedule / Maintenance",
		"Security / Credentials",
	}

	for _, g := range coreGroups {
		count := groupCounts[g]
		if count == 0 {
			t.Errorf("Core domain group %q has 0 keys assigned", g)
		}
	}
}

// 6. Adversarial Batch Apply: Mixed Valid, Invalid, and Protected Keys
func TestAdversarialBatchApplyMixedTransaction(t *testing.T) {
	tempDir := t.TempDir()
	for _, k := range []string{"toolbar_show_count", "custom_hashtags", "ui_tabs_links", "shinobi_group_key", "button_reboot"} {
		if err := os.WriteFile(filepath.Join(tempDir, k), []byte("init"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	broker := &challengeMockBroker{
		readValues: map[string]any{
			"toolbar_show_count": float64(10),
			"custom_hashtags":    "#NewTags #Live",
			"ui_tabs_links":      "[C01]\nstream_label=Live",
		},
		writeAcks: map[string]WriteAck{
			"toolbar_show_count": {OldValue: float64(5), NewValue: float64(10)},
			"custom_hashtags":    {OldValue: "old", NewValue: "#NewTags #Live"},
			"ui_tabs_links":      {OldValue: "old", NewValue: "[C01]\nstream_label=Live"},
		},
	}

	catalog := NewCatalog(tempDir)
	svc := NewService(broker, catalog, 20)

	changes := map[string]any{
		"toolbar_show_count": float64(10),                // valid editable number
		"custom_hashtags":    "#NewTags #Live",           // valid editable string
		"ui_tabs_links":      "[C01]\nstream_label=Live", // valid editable string
		"shinobi_group_key":  "NEW_KEY",                  // protected -> must reject
		"button_reboot":      true,                       // confirmable but confirmed=false -> must reject
		"unknown_bad_key":    "val",                      // unknown key -> must reject
	}

	results, err := svc.Apply(context.Background(), changes, false)
	if err != nil {
		t.Fatalf("unexpected error from batch Apply: %v", err)
	}

	resMap := make(map[string]ChangeResult)
	for _, r := range results {
		resMap[r.Key] = r
	}

	// 1. toolbar_show_count -> Applied
	if r, ok := resMap["toolbar_show_count"]; !ok || !r.Applied || r.Error != "" {
		t.Errorf("toolbar_show_count failed to apply: %+v", r)
	}

	// 2. custom_hashtags -> Applied
	if r, ok := resMap["custom_hashtags"]; !ok || !r.Applied || r.Error != "" {
		t.Errorf("custom_hashtags failed to apply: %+v", r)
	}

	// 3. ui_tabs_links -> Applied
	if r, ok := resMap["ui_tabs_links"]; !ok || !r.Applied || r.Error != "" {
		t.Errorf("ui_tabs_links failed to apply: %+v", r)
	}

	// 4. shinobi_group_key -> Rejected (read-only)
	if r, ok := resMap["shinobi_group_key"]; !ok || r.Applied || r.Error != "key is read-only" {
		t.Errorf("shinobi_group_key should be rejected with 'key is read-only', got: %+v", r)
	}

	// 5. button_reboot -> Rejected (confirmation required)
	if r, ok := resMap["button_reboot"]; !ok || r.Applied || r.Error != "confirmation is required" {
		t.Errorf("button_reboot should require confirmation, got: %+v", r)
	}

	// 6. unknown_bad_key -> Rejected (read-only / unknown)
	if r, ok := resMap["unknown_bad_key"]; !ok || r.Applied || r.Error == "" {
		t.Errorf("unknown_bad_key should be rejected, got: %+v", r)
	}

	// Verify that ONLY the 3 valid keys were written to the broker
	if len(broker.writes) != 1 {
		t.Fatalf("expected exactly 1 write call to broker, got %d", len(broker.writes))
	}
	writtenKeys := broker.writes[0]
	if len(writtenKeys) != 3 {
		t.Errorf("expected 3 written keys, got %d: %+v", len(writtenKeys), writtenKeys)
	}
	if _, ok := writtenKeys["shinobi_group_key"]; ok {
		t.Errorf("SECURITY VIOLATION: shinobi_group_key was sent to broker.Write!")
	}
}

// 7. Concurrent read/write stress test
func TestAdversarialCatalogConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	catalog := NewCatalog(tempDir)

	var wg sync.WaitGroup
	workers := 10
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternating List, Meta, Observe
				_ = catalog.List()
				_, _ = catalog.Meta("toolbar_show_count")
				_, _ = catalog.Meta("custom_hashtags")
				_, _ = catalog.Meta("ui_tabs_links")
				_, _ = catalog.Meta("shinobi_group_key")
				catalog.Observe(fmt.Sprintf("dynamic_key_%d", workerID), j)
				_, _ = catalog.Status()
			}
		}(i)
	}

	wg.Wait()
}
