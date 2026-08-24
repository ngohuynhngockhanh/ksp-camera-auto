package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

type redbidaTestBroker struct {
	values map[string]any
	acks   map[string]redbida.WriteAck
}

func (b *redbidaTestBroker) Read(context.Context, []string) (map[string]any, error) {
	return b.values, nil
}
func (b *redbidaTestBroker) Write(context.Context, map[string]any) (map[string]redbida.WriteAck, error) {
	return b.acks, nil
}

func TestRedbidaHandlersRefreshAndApply(t *testing.T) {
	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logo_header"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.redbida = redbida.NewService(&redbidaTestBroker{
		values: map[string]any{"logo_header": "https://example.test/new.png"},
		acks:   map[string]redbida.WriteAck{"logo_header": {OldValue: "https://example.test/old.png", NewValue: "https://example.test/new.png"}},
	}, redbida.NewCatalog(dir), 20)

	refreshReq := httptest.NewRequest(http.MethodPost, "/api/redbida/refresh", bytes.NewBufferString(`{"keys":["logo_header"]}`))
	refreshRes := httptest.NewRecorder()
	s.handleRedbidaRefresh(refreshRes, refreshReq)
	if refreshRes.Code != http.StatusOK || !bytes.Contains(refreshRes.Body.Bytes(), []byte(`"logo_header"`)) {
		t.Fatalf("refresh response = %d %s", refreshRes.Code, refreshRes.Body.String())
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(`{"changes":{"logo_header":"https://example.test/new.png"}}`))
	applyRes := httptest.NewRecorder()
	s.handleRedbidaApply(applyRes, applyReq)
	if applyRes.Code != http.StatusOK || !bytes.Contains(applyRes.Body.Bytes(), []byte(`"applied":true`)) {
		t.Fatalf("apply response = %d %s", applyRes.Code, applyRes.Body.String())
	}
}

func TestRedbidaCatalogHandlerIncludesDiscoveredMetadata(t *testing.T) {
	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	s.redbida = redbida.NewService(&redbidaTestBroker{}, redbida.NewCatalog(t.TempDir()), 20)
	res := httptest.NewRecorder()
	s.handleRedbidaCatalog(res, httptest.NewRequest(http.MethodGet, "/api/redbida/catalog", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("catalog status = %d", res.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["keys"]; !ok {
		t.Fatalf("catalog missing keys: %+v", body)
	}
}

func TestRedbidaCatalogReportsUnavailableSourceOnFirstRequest(t *testing.T) {
	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	s.redbida = redbida.NewService(&redbidaTestBroker{}, redbida.NewCatalog(t.TempDir()+"/missing"), 20)
	res := httptest.NewRecorder()
	s.handleRedbidaCatalog(res, httptest.NewRequest(http.MethodGet, "/api/redbida/catalog", nil))
	var body struct {
		SourceAvailable bool `json:"sourceAvailable"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SourceAvailable {
		t.Fatalf("unavailable source reported available: %s", res.Body.String())
	}
}

func TestConfigReportsRedbidaEnabled(t *testing.T) {
	s := &Server{cfg: config.Default()}
	s.cfg.Redbida.Enabled = true
	res := httptest.NewRecorder()
	s.handleConfig(res, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var body struct {
		RedbidaEnabled bool `json:"redbidaEnabled"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.RedbidaEnabled {
		t.Fatalf("enabled Redbida missing from config payload: %s", res.Body.String())
	}
}

func TestRedbidaRoutesEnforceViewerAuthorization(t *testing.T) {
	dir := t.TempDir()
	inv, err := config.LoadInventory(filepath.Join(dir, "cameras.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Redbida.Enabled = true
	cfg.Redbida.KeyDir = dir
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	viewer := &http.Cookie{Name: sessionCookie, Value: srv.session.create("viewer")}

	getReq := httptest.NewRequest(http.MethodGet, "/api/redbida/catalog", nil)
	getReq.AddCookie(viewer)
	getRes := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("viewer catalog status = %d", getRes.Code)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(`{"changes":{"logo_header":"https://example.test/logo.png"}}`))
	postReq.AddCookie(viewer)
	postRes := httptest.NewRecorder()
	srv.Handler().ServeHTTP(postRes, postReq)
	if postRes.Code != http.StatusForbidden {
		t.Fatalf("viewer apply status = %d, want %d", postRes.Code, http.StatusForbidden)
	}
}

func TestRedbidaCatalogHandlerMetadataAndDomainGroups(t *testing.T) {
	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	s.redbida = redbida.NewService(&redbidaTestBroker{}, redbida.NewCatalog(t.TempDir()+"/missing"), 20)
	res := httptest.NewRecorder()
	s.handleRedbidaCatalog(res, httptest.NewRequest(http.MethodGet, "/api/redbida/catalog", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("catalog status = %d", res.Code)
	}
	var body struct {
		Keys []redbida.KeyMeta `json:"keys"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	byKey := make(map[string]redbida.KeyMeta)
	for _, k := range body.Keys {
		byKey[k.Key] = k
	}

	// toolbar_show_count: editable number, Livestream
	tb, ok := byKey["toolbar_show_count"]
	if !ok || !tb.Editable || tb.ValueType != redbida.TypeNumber || tb.Risk != redbida.RiskEditable || tb.Group != "Livestream" {
		t.Fatalf("unexpected toolbar_show_count: %+v", tb)
	}

	// custom_hashtags: editable string, Branding / Logo
	ch, ok := byKey["custom_hashtags"]
	if !ok || !ch.Editable || ch.ValueType != redbida.TypeString || ch.Risk != redbida.RiskEditable || ch.Group != "Branding / Logo" {
		t.Fatalf("unexpected custom_hashtags: %+v", ch)
	}

	// ui_tabs_links: editable string, UI / Display
	ut, ok := byKey["ui_tabs_links"]
	if !ok || !ut.Editable || ut.ValueType != redbida.TypeString || ut.Risk != redbida.RiskEditable || ut.Group != "UI / Display" {
		t.Fatalf("unexpected ui_tabs_links: %+v", ut)
	}

	// shinobi_group_key: secret read-only, Security / Credentials
	sg, ok := byKey["shinobi_group_key"]
	if !ok || sg.Editable || !sg.Secret || sg.Risk != redbida.RiskProtected || sg.Group != "Security / Credentials" {
		t.Fatalf("unexpected shinobi_group_key: %+v", sg)
	}

	// Check domain groupings for representative keys
	expectedGroups := map[string]string{
		"logo_header":       "Branding / Logo",
		"company_name":      "Branding / Logo",
		"camera_count":      "Livestream",
		"video_config":      "Livestream",
		"ui_title":          "UI / Display",
		"ui_bg":             "UI / Display",
		"stop_camera_00h05": "Schedule / Maintenance",
		"button_reboot":     "Schedule / Maintenance",
		"shinobi_camera_id": "Security / Credentials",
		"ggcode":            "Security / Credentials",
	}
	for key, wantGroup := range expectedGroups {
		meta, exists := byKey[key]
		if !exists {
			t.Errorf("expected key %s to exist in catalog", key)
			continue
		}
		if meta.Group != wantGroup {
			t.Errorf("key %s group = %q, want %q", key, meta.Group, wantGroup)
		}
	}
}

func TestRedbidaApplyBatchPresetChanges(t *testing.T) {
	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	dir := t.TempDir()
	for _, key := range []string{"toolbar_show_count", "custom_hashtags", "ui_tabs_links", "ui_title", "camera_count"} {
		if err := os.WriteFile(filepath.Join(dir, key), []byte("init"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	testValues := map[string]any{
		"toolbar_show_count": float64(8),
		"custom_hashtags":    "#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports",
		"ui_tabs_links":      "[C01]\nstream_label=Live\nvid_play_label=CX King",
		"ui_title":           "CX King Luxury",
		"camera_count":       float64(8),
	}
	testAcks := map[string]redbida.WriteAck{}
	for k, v := range testValues {
		testAcks[k] = redbida.WriteAck{OldValue: "old", NewValue: v}
	}

	s.redbida = redbida.NewService(&redbidaTestBroker{
		values: testValues,
		acks:   testAcks,
	}, redbida.NewCatalog(dir), 20)

	payload, err := json.Marshal(map[string]any{
		"changes": map[string]any{
			"toolbar_show_count": 8,
			"custom_hashtags":    "#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports",
			"ui_tabs_links":      "[C01]\nstream_label=Live\nvid_play_label=CX King",
			"ui_title":           "CX King Luxury",
			"camera_count":       8,
		},
		"confirmed": false,
	})
	if err != nil {
		t.Fatal(err)
	}

	applyReq := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewReader(payload))
	applyRes := httptest.NewRecorder()
	s.handleRedbidaApply(applyRes, applyReq)
	if applyRes.Code != http.StatusOK {
		t.Fatalf("apply status = %d body = %s", applyRes.Code, applyRes.Body.String())
	}

	var res struct {
		Results   []redbida.ChangeResult `json:"results"`
		AppliedAt string                 `json:"appliedAt"`
	}
	if err := json.Unmarshal(applyRes.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(res.Results))
	}
	for _, r := range res.Results {
		if !r.Applied {
			t.Errorf("key %s was not applied: error=%q", r.Key, r.Error)
		}
	}
}
