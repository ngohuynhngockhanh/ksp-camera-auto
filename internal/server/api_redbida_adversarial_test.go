package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

type dynamicServerTestBroker struct {
	mu     sync.Mutex
	values map[string]any
	acks   map[string]redbida.WriteAck
}

func (b *dynamicServerTestBroker) Read(ctx context.Context, keys []string) (map[string]any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	res := make(map[string]any)
	for _, k := range keys {
		if v, ok := b.values[k]; ok {
			res[k] = v
		}
	}
	return res, nil
}

func (b *dynamicServerTestBroker) Write(ctx context.Context, changes map[string]any) (map[string]redbida.WriteAck, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	res := make(map[string]redbida.WriteAck)
	for k, v := range changes {
		old := b.values[k]
		b.values[k] = v
		res[k] = redbida.WriteAck{OldValue: old, NewValue: v}
	}
	return res, nil
}

// Adversarial HTTP testing for RedBida endpoints
func TestAdversarialHTTPApplyEndpoints(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"toolbar_show_count", "custom_hashtags", "ui_tabs_links", "shinobi_group_key", "ui_title"} {
		if err := os.WriteFile(filepath.Join(dir, key), []byte("init"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	testValues := map[string]any{
		"toolbar_show_count": float64(10),
		"custom_hashtags":    "#BidaLive #Champion",
		"ui_tabs_links":      "[C01]\nstream_label=Live\nvid_play_label=Test Club",
		"ui_title":           "Test Club",
	}

	s := &Server{cfg: config.Default(), mux: http.NewServeMux()}
	s.redbida = redbida.NewService(&dynamicServerTestBroker{
		values: testValues,
		acks:   make(map[string]redbida.WriteAck),
	}, redbida.NewCatalog(dir), 20)

	// 1. Attack: Malformed JSON body
	t.Run("malformed_json_body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(`{invalid_json`))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("malformed JSON returned status %d, want %d", res.Code, http.StatusBadRequest)
		}
	})

	// 2. Attack: Empty changes map (Service rejects with error -> 502 Bad Gateway)
	t.Run("empty_changes_map", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(`{"changes":{}}`))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		if res.Code != http.StatusBadGateway {
			t.Errorf("empty changes returned status %d, want %d", res.Code, http.StatusBadGateway)
		}
	})

	// 3. Attack: Negative and out-of-range toolbar_show_count via API
	t.Run("toolbar_show_count_out_of_bounds", func(t *testing.T) {
		payload := `{"changes":{"toolbar_show_count": -5}}`
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(payload))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with error in results, got %d", res.Code)
		}
		var resp struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || resp.Results[0].Applied || !strings.Contains(resp.Results[0].Error, "between 0 and 4096") {
			t.Errorf("negative toolbar_show_count not rejected properly: %+v", resp.Results)
		}
	})

	// 4. Attack: Float toolbar_show_count via API
	t.Run("toolbar_show_count_float", func(t *testing.T) {
		payload := `{"changes":{"toolbar_show_count": 8.75}}`
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewBufferString(payload))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		var resp struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || resp.Results[0].Applied || resp.Results[0].Error != "value must be an integer" {
			t.Errorf("float toolbar_show_count not rejected as non-integer: %+v", resp.Results)
		}
	})

	// 5. Valid: 20-section INI ui_tabs_links with UTF-8 Vietnamese diacritics
	t.Run("valid_20_tabs_ini_apply", func(t *testing.T) {
		var sb strings.Builder
		for i := 1; i <= 20; i++ {
			fmt.Fprintf(&sb, "[C%02d]\nstream_label=Video Trực tiếp\nvid_list_label=Highlight\nvid_play_label=CX King Luxury\n\n", i)
		}
		body, _ := json.Marshal(map[string]any{
			"changes": map[string]any{
				"ui_tabs_links": sb.String(),
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewReader(body))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("valid 20 tabs INI returned status %d", res.Code)
		}
		var resp struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || !resp.Results[0].Applied || resp.Results[0].Error != "" {
			t.Errorf("ui_tabs_links failed: %+v", resp.Results)
		}
	})

	// 6. Security Attack: Attempt to modify shinobi_group_key via API
	t.Run("shinobi_group_key_apply_rejected", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"changes": map[string]any{
				"shinobi_group_key": "COMPROMISED_KEY",
			},
			"confirmed": true,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/redbida/apply", bytes.NewReader(body))
		res := httptest.NewRecorder()
		s.handleRedbidaApply(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with result error, got %d", res.Code)
		}
		var resp struct {
			Results []redbida.ChangeResult `json:"results"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || resp.Results[0].Applied || resp.Results[0].Error != "key is read-only" {
			t.Errorf("SECURITY: shinobi_group_key was not rejected with 'key is read-only': %+v", resp.Results)
		}
	})
}
