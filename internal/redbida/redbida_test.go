package redbida

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogToolbarShowCountMetadataAndValidation(t *testing.T) {
	meta := metaForKey("toolbar_show_count", "", "")
	if !meta.Editable {
		t.Fatalf("toolbar_show_count should be editable, got editable=%v", meta.Editable)
	}
	if meta.Risk != RiskEditable {
		t.Fatalf("toolbar_show_count risk = %v, want %v", meta.Risk, RiskEditable)
	}
	if meta.ValueType != TypeNumber {
		t.Fatalf("toolbar_show_count valueType = %v, want %v", meta.ValueType, TypeNumber)
	}
	if meta.Group != "Livestream" {
		t.Fatalf("toolbar_show_count group = %v, want %v", meta.Group, "Livestream")
	}

	// Valid numbers within [0, 4096] integer
	for _, val := range []any{0, 1, 8, 16, 20, 4096, float64(8)} {
		if err := validateValue(meta, val); err != nil {
			t.Errorf("validateValue(toolbar_show_count, %v) failed: %v", val, err)
		}
	}

	// Invalid: out of bounds or non-integer or string
	invalidTests := []any{-1, 4097, 8.5, "8", true, nil}
	for _, val := range invalidTests {
		if err := validateValue(meta, val); err == nil {
			t.Errorf("validateValue(toolbar_show_count, %v) should have failed but succeeded", val)
		}
	}
}

func TestCatalogStringKeysAcceptTextAndMultiline(t *testing.T) {
	// custom_hashtags
	hashMeta := metaForKey("custom_hashtags", "", "")
	if !hashMeta.Editable || hashMeta.Risk != RiskEditable {
		t.Fatalf("custom_hashtags should be editable: %+v", hashMeta)
	}
	if hashMeta.ValueType != TypeString {
		t.Fatalf("custom_hashtags valueType = %v, want %v", hashMeta.ValueType, TypeString)
	}
	if hashMeta.Group != "Branding / Logo" {
		t.Fatalf("custom_hashtags group = %v, want %v", hashMeta.Group, "Branding / Logo")
	}
	if err := validateValue(hashMeta, "#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports"); err != nil {
		t.Fatalf("valid custom_hashtags rejected: %v", err)
	}

	// ui_tabs_links
	tabsMeta := metaForKey("ui_tabs_links", "", "")
	if !tabsMeta.Editable || tabsMeta.Risk != RiskEditable {
		t.Fatalf("ui_tabs_links should be editable: %+v", tabsMeta)
	}
	if tabsMeta.ValueType != TypeString {
		t.Fatalf("ui_tabs_links valueType = %v, want %v", tabsMeta.ValueType, TypeString)
	}
	if tabsMeta.Group != "UI / Display" {
		t.Fatalf("ui_tabs_links group = %v, want %v", tabsMeta.Group, "UI / Display")
	}
	multilineINI := `[C01]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=CX King Luxury
list_refresh_label=Cập nhật highlight

[C02]
stream_label=Video Trực tiếp
vid_list_label=Danh sách highlight
vid_play_label=CX King Luxury
list_refresh_label=Cập nhật highlight`
	if err := validateValue(tabsMeta, multilineINI); err != nil {
		t.Fatalf("multiline INI ui_tabs_links rejected: %v", err)
	}
}

func TestCatalogShinobiGroupKeyFallbackAndClassification(t *testing.T) {
	catalog := NewCatalog(filepath.Join(t.TempDir(), "missing"))
	meta, ok := catalog.Meta("shinobi_group_key")
	if !ok {
		t.Fatal("shinobi_group_key missing from fallback catalog")
	}
	if meta.Editable {
		t.Fatalf("shinobi_group_key should NOT be editable: %+v", meta)
	}
	if !meta.Secret {
		t.Fatalf("shinobi_group_key should be secret: %+v", meta)
	}
	if meta.Risk != RiskProtected {
		t.Fatalf("shinobi_group_key risk = %v, want %v", meta.Risk, RiskProtected)
	}
	if meta.Group != "Security / Credentials" {
		t.Fatalf("shinobi_group_key group = %v, want %v", meta.Group, "Security / Credentials")
	}
}

func TestCatalogDomainGroupingClassifications(t *testing.T) {
	tests := []struct {
		key       string
		wantGroup string
	}{
		// Branding / Logo
		{"logo_header", "Branding / Logo"},
		{"logo_header_text", "Branding / Logo"},
		{"logo_livestream", "Branding / Logo"},
		{"logo_cat_cam", "Branding / Logo"},
		{"company_name", "Branding / Logo"},
		{"banner_top", "Branding / Logo"},
		{"custom_hashtags", "Branding / Logo"},
		{"app_background", "Branding / Logo"},
		{"app_hide_inut", "Branding / Logo"},
		{"app_hotline", "Branding / Logo"},
		{"app_website", "Branding / Logo"},

		// Livestream
		{"camera_count", "Livestream"},
		{"toolbar_show_count", "Livestream"},
		{"video_config", "Livestream"},
		{"hls_using_go2rtc", "Livestream"},
		{"hls_using_go2rtc_livestream", "Livestream"},
		{"hls_using_go2rtc_tiktok", "Livestream"},
		{"button_generate_go2rtc_stream", "Livestream"},
		{"default_delay_camera", "Livestream"},
		{"default_delay_go2rtc", "Livestream"},
		{"fps_default", "Livestream"},
		{"livestream_default_bitrate", "Livestream"},
		{"place_livestream", "Livestream"},

		// UI / Display
		{"ui_title", "UI / Display"},
		{"ui_bg", "UI / Display"},
		{"ui_scoreboard", "UI / Display"},
		{"ui_tabs_links", "UI / Display"},
		{"ui_css_custom", "UI / Display"},
		{"ui_title_color", "UI / Display"},
		{"ui_download_text", "UI / Display"},
		{"ui_fb", "UI / Display"},
		{"ui_zalo", "UI / Display"},
		{"ui_tiktok", "UI / Display"},
		{"ui_google", "UI / Display"},
		{"ui_phone", "UI / Display"},
		{"language", "UI / Display"},
		{"show_toolbar", "UI / Display"},
		{"large_monitor", "UI / Display"},
		{"help_link", "UI / Display"},
		{"url_live_help", "UI / Display"},
		{"default_tiso_1_color", "UI / Display"},
		{"default_tiso_type", "UI / Display"},
		{"shop_id", "UI / Display"},
		{"realtime_shop_id", "UI / Display"},

		// Schedule / Maintenance
		{"stop_camera_00h05", "Schedule / Maintenance"},
		{"button_reboot", "Schedule / Maintenance"},
		{"button_restart_shinobi", "Schedule / Maintenance"},
		{"max_free_ram_force_reboot", "Schedule / Maintenance"},
		{"max_shared_ram_camera", "Schedule / Maintenance"},
		{"db_check_range", "Schedule / Maintenance"},
		{"watch_uptime_process", "Schedule / Maintenance"},

		// Security / Credentials
		{"shinobi_group_key", "Security / Credentials"},
		{"shinobi_camera_id", "Security / Credentials"},
		{"shinobi_token", "Security / Credentials"},
		{"shinobi_monitor_token", "Security / Credentials"},
		{"ggcode", "Security / Credentials"},
		{"frpc_config", "Security / Credentials"},
		{"mqtt_password", "Security / Credentials"},
	}

	for _, tt := range tests {
		meta := metaForKey(tt.key, "", "")
		if meta.Group != tt.wantGroup {
			t.Errorf("metaForKey(%q).Group = %q, want %q", tt.key, meta.Group, tt.wantGroup)
		}
	}
}

func TestCatalogListOrderingAndFallbackCompleteness(t *testing.T) {
	catalog := NewCatalog(filepath.Join(t.TempDir(), "missing"))
	list := catalog.List()
	if len(list) < 50 {
		t.Fatalf("catalog list too short: %d", len(list))
	}

	// Verify items are grouped and sorted by key within group
	for i := 1; i < len(list); i++ {
		prev := list[i-1]
		curr := list[i]
		if prev.Group > curr.Group {
			t.Errorf("groups not sorted: %q after %q", curr.Group, prev.Group)
		} else if prev.Group == curr.Group && strings.Compare(prev.Key, curr.Key) > 0 {
			t.Errorf("keys within group %q not sorted: %q after %q", curr.Group, curr.Key, prev.Key)
		}
	}
}
