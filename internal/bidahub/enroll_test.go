package bidahub

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A box presenting the fleet key is enrolled from its own RedBida identity, not seed placeholders.
func TestAutoEnrollFromRedBidaIdentity(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id": "inut_204_43",
		"lan_ip": "192.168.1.6",
		"arch":   "aarch64",
		"redbida_keys": map[string]string{
			"ui_title":          "77 Billiards",
			"shinobi_camera_id": "videoaivn03",
			"logo_header":       "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png",
			"custom_hashtags":   "#77Billiards #BILLIARDSlive #INUTlive #highlightsports",
			"camera_count":      "7",
			"frpc_config":       "[store-77]\ntype = http\nlocal_port = 2023\nsubdomain = store-777\n\n[c-store-77]\ntype = http\nlocal_port = 8080\nsubdomain = c-store-77\n",
		},
	}, map[string]string{"X-Box-Secret": testAPIKey, "NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fleet-key heartbeat to auto-enroll (200), got %d: %v", rec.Code, data)
	}

	_, detail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_43", nil, nil)
	box, _ := detail["box"].(map[string]any)
	if box == nil {
		t.Fatalf("auto-enrolled box missing from /fleet/api/boxes: %v", detail)
	}
	want := map[string]any{
		"name":              "77 Billiards",
		"club_name":         "77 Billiards",
		"lan_ip":            "192.168.1.6",
		"vpn_ip":            "77.88.204.43",
		"shinobi_group_key": "videoaivn03",
		"store_domain":      "store-777.video.io.vn",
		"cstore_domain":     "c-store-77.video.io.vn",
		"branding":          "billiards",
	}
	for k, v := range want {
		if box[k] != v {
			t.Errorf("box[%q] = %v, want %v", k, box[k], v)
		}
	}
	if box["cameras_count"] != float64(7) {
		t.Errorf("cameras_count = %v, want 7 (from RedBida camera_count)", box["cameras_count"])
	}
}

// Older agents (no RedBida identity) with the fleet key stay pending instead of creating placeholders.
func TestFleetKeyWithoutIdentityStaysPending(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id": "inut_209_55",
	}, map[string]string{"X-Box-Secret": testAPIKey, "NO_AUTH": "1"})
	if rec.Code != http.StatusForbidden || data["error"] != "ENROLLMENT_PENDING" {
		t.Fatalf("expected 403 ENROLLMENT_PENDING without ui_title, got %d: %v", rec.Code, data)
	}
}

// Unknown boxes without the fleet key still wait for manual approval.
func TestUnknownSecretStillRequiresApproval(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":       "inut_209_77",
		"redbida_keys": map[string]string{"ui_title": "Rogue Box"},
	}, map[string]string{"X-Box-Secret": "wrong-secret", "NO_AUTH": "1"})
	if rec.Code != http.StatusForbidden || data["error"] != "ENROLLMENT_PENDING" {
		t.Fatalf("expected 403 ENROLLMENT_PENDING for wrong secret, got %d: %v", rec.Code, data)
	}
	if rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_209_77", nil, nil); rec.Code == http.StatusOK {
		t.Fatalf("box with wrong secret must not be created")
	}
}

// Heartbeats overwrite seeded placeholder RedBida data on already-known boxes with box truth.
func TestHeartbeatReplacesSeededRedBidaData(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id": "inut_204_141",
		"redbida_keys": map[string]string{
			"ui_title":          "Real Venue Title",
			"shinobi_camera_id": "realgk",
			"camera_count":      "3",
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d", rec.Code)
	}
	_, rep := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141/redbida", nil, nil)
	keys, _ := rep["keys"].(map[string]any)
	if keys["ui_title"] != "Real Venue Title" || rep["shinobi_group_key"] != "realgk" {
		t.Fatalf("expected RedBida report to mirror box keys, got ui_title=%v gk=%v", keys["ui_title"], rep["shinobi_group_key"])
	}
	if _, seeded := keys["video_format"]; seeded {
		t.Errorf("seeded placeholder key video_format must not survive a real key report")
	}
}

// The agent reports allow-listed keys only and never ships frpc credentials.
func TestReadRedBidaKeysRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	write := func(k, v string) {
		if err := os.WriteFile(filepath.Join(dir, k), []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("ui_title", "77 Billiards\n")
	write("shinobi_token", "SECRET-API-TOKEN")
	write("frpc_config", "[common]\nserver_addr = video.io.vn\ntoken = SECRET-FRP\nmeta_token=SECRET-META\n[store-77]\nsubdomain = store-777\n")

	keys := readRedBidaKeys(dir)
	if keys["ui_title"] != "77 Billiards" {
		t.Errorf("ui_title = %q", keys["ui_title"])
	}
	if _, ok := keys["shinobi_token"]; ok {
		t.Errorf("shinobi_token must never be reported")
	}
	frpc := keys["frpc_config"]
	if strings.Contains(frpc, "SECRET") {
		t.Errorf("frpc_config leaked a credential: %q", frpc)
	}
	if !strings.Contains(frpc, "subdomain = store-777") {
		t.Errorf("frpc_config lost non-secret lines: %q", frpc)
	}
}
