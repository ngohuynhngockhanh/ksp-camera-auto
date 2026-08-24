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
