package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

type mockRedbidaBroker struct {
	mu         sync.Mutex
	readValues map[string]any
	writes     map[string]any
	writeErr   error
}

func (m *mockRedbidaBroker) Read(_ context.Context, keys []string) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make(map[string]any)
	for _, k := range keys {
		if v, ok := m.readValues[k]; ok {
			res[k] = v
		}
	}
	return res, nil
}

func (m *mockRedbidaBroker) Write(_ context.Context, changes map[string]any) (map[string]redbida.WriteAck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writes == nil {
		m.writes = make(map[string]any)
	}
	if m.readValues == nil {
		m.readValues = make(map[string]any)
	}
	for k, v := range changes {
		m.writes[k] = v
		// Update read-back store so read-back verification succeeds
		m.readValues[k] = v
	}
	if m.writeErr != nil {
		return nil, m.writeErr
	}
	ack := make(map[string]redbida.WriteAck)
	for k, v := range changes {
		ack[k] = redbida.WriteAck{
			OldValue: nil,
			NewValue: v,
		}
	}
	return ack, nil
}

func newTestRedbidaSetup(t *testing.T) (*Registry, *mockRedbidaBroker, *redbida.Service) {
	t.Helper()
	dir := t.TempDir()
	files := []string{
		"ui_title", "company_name", "ui_bg", "custom_hashtags", "ui_tabs_links",
		"camera_count", "toolbar_show_count", "video_config", "hls_using_go2rtc",
		"hls_using_go2rtc_livestream", "hls_using_go2rtc_tiktok", "ui_scoreboard",
		"logo_header", "logo_header_text", "button_generate_go2rtc_stream",
		"mqtt_password", "shinobi_token", "shinobi_camera_id", "shinobi_group_key",
		"ggcode",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("init"), 0o600); err != nil {
			t.Fatalf("failed to create test key file: %v", err)
		}
	}

	cat := redbida.NewCatalog(dir)
	broker := &mockRedbidaBroker{
		readValues: map[string]any{
			"ui_title":      "Old Title",
			"camera_count":  6,
			"mqtt_password": "super-secret-password",
		},
	}
	svc := redbida.NewService(broker, cat, 200)
	reg := NewRegistry()
	cfg := config.Default()
	registerRedbidaTools(reg, &cfg, svc)

	return reg, broker, svc
}

func TestRemoveVietnameseTones(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"CX King Luxury", "CX King Luxury"},
		{"Bida Lạc Long Quân", "Bida Lac Long Quan"},
		{"Câu lạc bộ Bida Hoàng Gia", "Cau lac bo Bida Hoang Gia"},
		{"ĐỒNG NAI - ĐÀ NẴNG - ĐỨC TRỌNG", "DONG NAI - DA NANG - DUC TRONG"},
		{"àáảãạăằắẳẵặâầấẩẫậ", "aaaaaaaaaaaaaaaaa"},
		{"ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬ", "AAAAAAAAAAAAAAAAA"},
		{"èéẻẽẹêềếểễệ", "eeeeeeeeeee"},
		{"ÈÉẺẼẸÊỀẾỂỄỆ", "EEEEEEEEEEE"},
		{"ìíỉĩị", "iiiii"},
		{"ÌÍỈĨỊ", "IIIII"},
		{"òóỏõọôồốổỗộơờớởỡợ", "ooooooooooooooooo"},
		{"ÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢ", "OOOOOOOOOOOOOOOOO"},
		{"ùúủũụưừứửữự", "uuuuuuuuuuu"},
		{"ÙÚỦŨỤƯỪỨỬỮỰ", "UUUUUUUUUUU"},
		{"ỳýỷỹỵ", "yyyyy"},
		{"ỲÝỶỸỴ", "YYYYY"},
		{"đ", "d"},
		{"Đ", "D"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := removeVietnameseTones(tc.input)
			if got != tc.expected {
				t.Errorf("removeVietnameseTones(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestSanitizeCleanTitle(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"CX King Luxury", "CXKingLuxury"},
		{"SD Billiards Club - CS2", "SDBilliardsClubCS2"},
		{"Bida Lạc Long Quân #1 @2026", "BidaLacLongQuan12026"},
		{"Quán Bida 99!", "QuanBida99"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeCleanTitle(tc.input)
			if got != tc.expected {
				t.Errorf("sanitizeCleanTitle(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestSanitizeCSSGradient(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			"",
			"radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )",
		},
		{
			"linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);",
			"linear-gradient(135deg, #1e3c72 0%, #2a5298 100%)",
		},
		{
			"linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);; ; ",
			"linear-gradient(135deg, #1e3c72 0%, #2a5298 100%)",
		},
		{
			"radial-gradient(circle, #000 0%, #fff 100%)",
			"radial-gradient(circle, #000 0%, #fff 100%)",
		},
	}

	for _, tc := range cases {
		got := sanitizeCSSGradient(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeCSSGradient(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestGenerate20TabINITabs(t *testing.T) {
	title := "CX King Luxury"
	ini := generate20TabINITabs(title)

	sections := strings.Split(ini, "\n\n")
	if len(sections) != 20 {
		t.Fatalf("expected 20 sections in INI, got %d", len(sections))
	}

	for i := 1; i <= 20; i++ {
		secHeader := fmt.Sprintf("[C%02d]", i)
		if !strings.Contains(sections[i-1], secHeader) {
			t.Errorf("section %d missing header %s: %s", i, secHeader, sections[i-1])
		}
		if !strings.Contains(sections[i-1], "vid_play_label="+title) {
			t.Errorf("section %d missing vid_play_label=%s: %s", i, title, sections[i-1])
		}
		if !strings.Contains(sections[i-1], "stream_label=Video Trực tiếp") {
			t.Errorf("section %d missing stream_label", i)
		}
		if !strings.Contains(sections[i-1], "vid_list_label=Danh sách highlight") {
			t.Errorf("section %d missing vid_list_label", i)
		}
		if !strings.Contains(sections[i-1], "list_refresh_label=Cập nhật highlight") {
			t.Errorf("section %d missing list_refresh_label", i)
		}
	}
}

func TestRedbidaTools_ListCatalog(t *testing.T) {
	reg, _, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	// 1. Full catalog listing
	res, err := reg.Call(ctx, "redbida_list_catalog", []byte(`{}`))
	if err != nil {
		t.Fatalf("redbida_list_catalog failed: %v", err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("unexpected error response: %+v", res)
	}

	var payload struct {
		Keys            []redbida.KeyMeta `json:"keys"`
		Count           int               `json:"count"`
		SourceAvailable bool              `json:"sourceAvailable"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("failed to unmarshal list_catalog response: %v", err)
	}
	if payload.Count == 0 || len(payload.Keys) == 0 {
		t.Errorf("expected non-empty keys, got count %d", payload.Count)
	}
	if !payload.SourceAvailable {
		t.Errorf("expected sourceAvailable to be true")
	}

	// 2. Filter by group
	filterArgs := []byte(`{"group":"UI / Display"}`)
	res, err = reg.Call(ctx, "redbida_list_catalog", filterArgs)
	if err != nil {
		t.Fatalf("redbida_list_catalog with group filter failed: %v", err)
	}
	var filteredPayload struct {
		Keys []redbida.KeyMeta `json:"keys"`
	}
	_ = json.Unmarshal([]byte(res.Content[0].Text), &filteredPayload)
	for _, k := range filteredPayload.Keys {
		if k.Group != "UI / Display" {
			t.Errorf("expected key %s to have group 'UI / Display', got %s", k.Key, k.Group)
		}
	}

	// 3. Filter by editableOnly
	editableArgs := []byte(`{"editableOnly":true}`)
	res, err = reg.Call(ctx, "redbida_list_catalog", editableArgs)
	if err != nil {
		t.Fatalf("redbida_list_catalog with editableOnly failed: %v", err)
	}
	var editablePayload struct {
		Keys []redbida.KeyMeta `json:"keys"`
	}
	_ = json.Unmarshal([]byte(res.Content[0].Text), &editablePayload)
	for _, k := range editablePayload.Keys {
		if !k.Editable {
			t.Errorf("expected key %s to be editable", k.Key)
		}
	}
}

func TestRedbidaTools_GetKeys(t *testing.T) {
	reg, _, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	// 1. Get specific keys including a secret key
	getArgs := []byte(`{"keys":["ui_title", "camera_count", "mqtt_password"]}`)
	res, err := reg.Call(ctx, "redbida_get_keys", getArgs)
	if err != nil {
		t.Fatalf("redbida_get_keys failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_keys returned error: %s", res.Content[0].Text)
	}

	var payload struct {
		Values []redbida.KeyValue `json:"values"`
		Count  int                `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal get_keys response: %v", err)
	}
	if payload.Count != 3 {
		t.Fatalf("expected 3 values, got %d", payload.Count)
	}

	for _, kv := range payload.Values {
		if kv.Key == "mqtt_password" {
			if kv.Value != "********" {
				t.Errorf("expected masked secret ********, got %v", kv.Value)
			}
		}
		if kv.Key == "ui_title" {
			if kv.Value != "Old Title" {
				t.Errorf("expected Old Title, got %v", kv.Value)
			}
		}
	}

	// 2. Get all keys
	resAll, err := reg.Call(ctx, "redbida_get_keys", []byte(`{"all":true}`))
	if err != nil {
		t.Fatalf("redbida_get_keys all failed: %v", err)
	}
	if resAll.IsError {
		t.Fatalf("get_keys all returned error: %s", resAll.Content[0].Text)
	}
}

func TestRedbidaTools_SetKeys(t *testing.T) {
	reg, broker, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	// 1. Set valid editable keys
	setArgs := map[string]any{
		"changes": map[string]any{
			"ui_title":     "New Venue Name",
			"camera_count": 8,
		},
	}
	setJSON, _ := json.Marshal(setArgs)
	res, err := reg.Call(ctx, "redbida_set_keys", setJSON)
	if err != nil {
		t.Fatalf("redbida_set_keys failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("set_keys returned error: %s", res.Content[0].Text)
	}

	var payload struct {
		Results []redbida.ChangeResult `json:"results"`
		Count   int                    `json:"count"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal set_keys response: %v", err)
	}

	for _, cr := range payload.Results {
		if !cr.Verified || !cr.Applied {
			t.Errorf("key %s was not verified/applied: %+v", cr.Key, cr)
		}
	}

	// Check broker writes
	if broker.writes["ui_title"] != "New Venue Name" {
		t.Errorf("expected broker write ui_title=New Venue Name, got %v", broker.writes["ui_title"])
	}

	// 2. Empty changes map error
	resEmpty, _ := reg.Call(ctx, "redbida_set_keys", []byte(`{"changes":{}}`))
	if !resEmpty.IsError {
		t.Errorf("expected error for empty changes map")
	}
}

func TestRedbidaTools_ApplyOnboardingPreset_DryRun(t *testing.T) {
	reg, broker, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	presetArgs := map[string]any{
		"title":          "CX King Luxury",
		"cameraCount":    8,
		"bg":             "radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% );",
		"groupKey":       "AWU8wJMd2l",
		"ggcode":         "G-SFSDZPR95Z",
		"customHashtags": "",
		"dryRun":         true,
	}
	presetJSON, _ := json.Marshal(presetArgs)
	res, err := reg.Call(ctx, "redbida_apply_onboarding_preset", presetJSON)
	if err != nil {
		t.Fatalf("apply_onboarding_preset dryRun failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("apply_onboarding_preset dryRun returned error: %s", res.Content[0].Text)
	}

	var payload struct {
		DryRun         bool           `json:"dryRun"`
		Title          string         `json:"title"`
		CameraCount    int            `json:"cameraCount"`
		ParameterCount int            `json:"parameterCount"`
		Parameters     map[string]any `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal preset dryRun response: %v", err)
	}

	if !payload.DryRun {
		t.Errorf("expected dryRun to be true")
	}
	if payload.Title != "CX King Luxury" {
		t.Errorf("expected title CX King Luxury, got %s", payload.Title)
	}
	if payload.CameraCount != 8 {
		t.Errorf("expected cameraCount 8, got %d", payload.CameraCount)
	}

	// Verify all synthesized parameters
	params := payload.Parameters
	if params["ui_title"] != "CX King Luxury" {
		t.Errorf("expected ui_title 'CX King Luxury', got %v", params["ui_title"])
	}
	if params["company_name"] != "CX King Luxury" {
		t.Errorf("expected company_name 'CX King Luxury', got %v", params["company_name"])
	}
	if strings.HasSuffix(params["ui_bg"].(string), ";") {
		t.Errorf("ui_bg must not contain trailing semicolon: %s", params["ui_bg"])
	}
	if params["custom_hashtags"] != "Tìm hiểu thêm tại BilliardLive.IO.VN\n#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports" {
		t.Errorf("expected custom_hashtags 'Tìm hiểu thêm tại BilliardLive.IO.VN\\n#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports', got %v", params["custom_hashtags"])
	}
	if !strings.Contains(params["ui_tabs_links"].(string), "[C01]") || !strings.Contains(params["ui_tabs_links"].(string), "[C20]") {
		t.Errorf("ui_tabs_links must contain [C01] and [C20]")
	}
	if params["camera_count"] != float64(8) && params["camera_count"] != 8 {
		t.Errorf("expected camera_count 8, got %v", params["camera_count"])
	}
	if params["toolbar_show_count"] != float64(8) && params["toolbar_show_count"] != 8 {
		t.Errorf("expected toolbar_show_count 8, got %v", params["toolbar_show_count"])
	}
	if params["video_config"] != "range=72" {
		t.Errorf("expected video_config 'range=72', got %v", params["video_config"])
	}
	if params["hls_using_go2rtc"] != true {
		t.Errorf("expected hls_using_go2rtc true, got %v", params["hls_using_go2rtc"])
	}
	if params["button_generate_go2rtc_stream"] != true {
		t.Errorf("expected button_generate_go2rtc_stream true, got %v", params["button_generate_go2rtc_stream"])
	}
	if params["logo_header"] != "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png" {
		t.Errorf("expected logo_header standard URL, got %v", params["logo_header"])
	}
	if params["logo_header_text"] != "Billiard Live - Tải clip bàn bida và livestream" {
		t.Errorf("expected logo_header_text standard slogan, got %v", params["logo_header_text"])
	}

	// Dry run should not write to broker
	if len(broker.writes) > 0 {
		t.Errorf("dryRun should not write to broker, found writes: %+v", broker.writes)
	}
}

func TestRedbidaTools_ApplyOnboardingPreset_Live(t *testing.T) {
	reg, broker, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	presetArgs := map[string]any{
		"title":       "Bida Lạc Long Quân",
		"cameraCount": 10,
		"dryRun":      false,
	}
	presetJSON, _ := json.Marshal(presetArgs)
	res, err := reg.Call(ctx, "redbida_apply_onboarding_preset", presetJSON)
	if err != nil {
		t.Fatalf("apply_onboarding_preset failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("apply_onboarding_preset returned error: %s", res.Content[0].Text)
	}

	var payload struct {
		OK      bool                   `json:"ok"`
		Title   string                 `json:"title"`
		Results []redbida.ChangeResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal live preset response: %v", err)
	}

	if !payload.OK {
		t.Errorf("expected OK true")
	}
	if payload.Title != "Bida Lạc Long Quân" {
		t.Errorf("expected title 'Bida Lạc Long Quân', got %s", payload.Title)
	}

	// Check broker writes
	if broker.writes["ui_title"] != "Bida Lạc Long Quân" {
		t.Errorf("expected broker write ui_title='Bida Lạc Long Quân', got %v", broker.writes["ui_title"])
	}
	if broker.writes["custom_hashtags"] != "Tìm hiểu thêm tại BilliardLive.IO.VN\n#BidaLacLongQuan #BILLIARDSlive #INUTlive #highlightsports" {
		t.Errorf("expected custom_hashtags 'Tìm hiểu thêm tại BilliardLive.IO.VN\\n#BidaLacLongQuan #BILLIARDSlive #INUTlive #highlightsports', got %v", broker.writes["custom_hashtags"])
	}
}

func TestRedbidaTools_ApplyOnboardingPreset_Validations(t *testing.T) {
	reg, _, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	// 1. Missing title
	res, _ := reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(`{"title":"","cameraCount":8}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "title is required") {
		t.Errorf("expected title is required error, got: %+v", res)
	}

	// 2. CameraCount < 1
	res, _ = reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(`{"title":"Test","cameraCount":0}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "cameraCount must be between 1 and 20") {
		t.Errorf("expected cameraCount range error, got: %+v", res)
	}

	// 3. CameraCount > 20
	res, _ = reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(`{"title":"Test","cameraCount":21}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "cameraCount must be between 1 and 20") {
		t.Errorf("expected cameraCount range error, got: %+v", res)
	}
}

func TestRedbidaTools_TriggerGo2RTC(t *testing.T) {
	reg, broker, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	res, err := reg.Call(ctx, "redbida_trigger_go2rtc", []byte(`{}`))
	if err != nil {
		t.Fatalf("redbida_trigger_go2rtc failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("trigger_go2rtc returned error: %s", res.Content[0].Text)
	}

	if broker.writes["button_generate_go2rtc_stream"] != true {
		t.Errorf("expected button_generate_go2rtc_stream true in broker writes, got: %v", broker.writes["button_generate_go2rtc_stream"])
	}
}

func TestRedbidaTools_GetTimeStatus(t *testing.T) {
	reg, _, _ := newTestRedbidaSetup(t)
	ctx := context.Background()

	res, err := reg.Call(ctx, "redbida_get_time_status", []byte(`{}`))
	if err != nil {
		t.Fatalf("redbida_get_time_status failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_time_status returned error: %s", res.Content[0].Text)
	}

	var payload struct {
		HostTime              string `json:"hostTime"`
		HostTimeRFC3339       string `json:"hostTimeRFC3339"`
		NTPSynchronized       bool   `json:"ntpSynchronized"`
		DriftThresholdSeconds int    `json:"driftThresholdSeconds"`
		NodeRedReadOnly       bool   `json:"nodeRedReadOnly"`
	}
	if err := json.Unmarshal([]byte(res.Content[0].Text), &payload); err != nil {
		t.Fatalf("unmarshal time_status response: %v", err)
	}

	if payload.HostTime == "" || payload.HostTimeRFC3339 == "" {
		t.Errorf("expected non-empty time strings, got hostTime=%s, rfc3339=%s", payload.HostTime, payload.HostTimeRFC3339)
	}
	if payload.DriftThresholdSeconds != 60 {
		t.Errorf("expected driftThresholdSeconds 60, got %d", payload.DriftThresholdSeconds)
	}
	if !payload.NodeRedReadOnly {
		t.Errorf("expected nodeRedReadOnly true")
	}
}

func TestRedbidaTools_DisabledServiceGracefulHandling(t *testing.T) {
	reg := NewRegistry()
	cfg := config.Default()
	// Register with nil redbida service
	registerRedbidaTools(reg, &cfg, nil)
	ctx := context.Background()

	// 1. list_catalog
	res, _ := reg.Call(ctx, "redbida_list_catalog", []byte(`{}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "disabled") {
		t.Errorf("expected disabled error for list_catalog, got: %+v", res)
	}

	// 2. get_keys
	res, _ = reg.Call(ctx, "redbida_get_keys", []byte(`{"keys":["ui_title"]}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "disabled") {
		t.Errorf("expected disabled error for get_keys, got: %+v", res)
	}

	// 3. set_keys
	res, _ = reg.Call(ctx, "redbida_set_keys", []byte(`{"changes":{"ui_title":"Test"}}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "disabled") {
		t.Errorf("expected disabled error for set_keys, got: %+v", res)
	}

	// 4. apply_onboarding_preset (live)
	res, _ = reg.Call(ctx, "redbida_apply_onboarding_preset", []byte(`{"title":"Test","cameraCount":8,"dryRun":false}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "disabled") {
		t.Errorf("expected disabled error for apply_onboarding_preset, got: %+v", res)
	}

	// 5. trigger_go2rtc
	res, _ = reg.Call(ctx, "redbida_trigger_go2rtc", []byte(`{}`))
	if !res.IsError || !strings.Contains(res.Content[0].Text, "disabled") {
		t.Errorf("expected disabled error for trigger_go2rtc, got: %+v", res)
	}

	// 6. get_time_status should still succeed even if redbida MQTT service is nil (queries host time)
	res, err := reg.Call(ctx, "redbida_get_time_status", []byte(`{}`))
	if err != nil || res.IsError {
		t.Errorf("get_time_status should succeed independently of redbida service: %+v", res)
	}
}
