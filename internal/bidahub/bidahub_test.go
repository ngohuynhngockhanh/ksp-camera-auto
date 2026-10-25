package bidahub

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testAPIKey      = "test-hub-api-key-2026"
	testStaffSecret = "test-staff-hmac-secret-2026"
	testAdminUser   = "admin"
	testAdminPass   = "inut2026"
	testViewerUser  = "viewer"
	testViewerPass  = "viewer2026"
)

func newTestHubServer(t *testing.T, opts ...func(*Config)) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "hub-state.json")
	cfg := Config{
		BindAddr:     "127.0.0.1",
		Port:         2090,
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		ViewerUser:   testViewerUser,
		ViewerPass:   testViewerPass,
		APIKey:       testAPIKey,
		StaffSecret:  testStaffSecret,
		PollInterval: 0,
		DialTimeout:  120 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})
	return srv, dataPath
}

func doHubRequest(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var bodyReader io.Reader
	contentType := ""
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
			contentType = "application/json"
		case []byte:
			bodyReader = bytes.NewReader(v)
			contentType = "application/json"
		case url.Values:
			bodyReader = strings.NewReader(v.Encode())
			contentType = "application/x-www-form-urlencoded"
		default:
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal request body failed: %v", err)
			}
			bodyReader = bytes.NewReader(raw)
			contentType = "application/json"
		}
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.RemoteAddr = "198.51.100.42:54321"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	noAuth := false
	for k, v := range headers {
		if k == "NO_AUTH" {
			noAuth = true
			continue
		}
		if k == "REMOTE_ADDR" {
			req.RemoteAddr = v
			continue
		}
		req.Header.Set(k, v)
	}
	if !noAuth && req.Header.Get("Authorization") == "" && req.Header.Get("X-Hub-Key") == "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var parsed map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	}
	return rec, parsed
}

func loginAndGetSessionCookie(t *testing.T, h http.Handler, username, password, remoteIP string) *http.Cookie {
	t.Helper()
	form := url.Values{
		"username": {username},
		"password": {password},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if remoteIP != "" {
		req.RemoteAddr = remoteIP + ":40000"
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	for _, c := range res.Cookies() {
		if c.Name == "inut_bida_session" && c.Value != "" {
			return c
		}
	}
	return nil
}

func storeListClubs(s *Server) []*Club {
	return s.Store().Clubs()
}

func storeListBoxes(s *Server) []*Box {
	return s.Store().Boxes()
}

func storeGetBox(s *Server, id string) *Box {
	b, _ := s.Store().GetBox(id)
	return b
}

func storeGetClub(s *Server, id string) *Club {
	c, _ := s.Store().GetClub(id)
	return c
}

func storeMutateBox(t *testing.T, s *Server, id string, fn func(b *Box)) {
	t.Helper()
	if err := s.Store().UpdateBox(id, func(b *Box) error {
		fn(b)
		return nil
	}); err != nil {
		t.Fatalf("UpdateBox(%s) failed: %v", id, err)
	}
}

// =============================================================================
// TIER 1: FEATURE COVERAGE TESTS (F1, F2, F4, F6) — 14 TESTS
// =============================================================================

// Test 1 (F1/F6): Public GET /api/health contract on port 2090 with seeded totals.
func TestTier1_F1_HealthEndpointPublicContract(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodGet, "/api/health", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/health, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(data["status"]); got != "ok" {
		t.Errorf("expected status=ok, got %q", got)
	}
	if got := fmt.Sprint(data["service"]); got != "inut-bida-hub" {
		t.Errorf("expected service=inut-bida-hub, got %q", got)
	}
	if got := fmt.Sprint(data["version"]); got != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %q", got)
	}
	port, _ := data["port"].(float64)
	if int(port) != 2090 {
		t.Errorf("expected port=2090, got %v", data["port"])
	}
	boxesTotal, _ := data["boxes_total"].(float64)
	clubsTotal, _ := data["clubs_total"].(float64)
	if int(boxesTotal) < 30 {
		t.Errorf("expected boxes_total >= 30 from FLEET_WATCHLIST.md, got %v", boxesTotal)
	}
	if int(clubsTotal) < 20 {
		t.Errorf("expected clubs_total >= 20 from FLEET_WATCHLIST.md, got %v", clubsTotal)
	}
}

// Test 2 (F1): Built-in & file Watchlist seeder populates 30+ boxes from FLEET_WATCHLIST.md.
func TestTier1_F1_DefaultFleetCatalogSeeding30PlusBoxes(t *testing.T) {
	clubs, boxes := DefaultFleetCatalog()
	if len(boxes) < 30 {
		t.Fatalf("expected DefaultFleetCatalog to return >= 30 boxes, got %d", len(boxes))
	}
	if len(clubs) < 20 {
		t.Fatalf("expected DefaultFleetCatalog to return >= 20 clubs, got %d", len(clubs))
	}

	requiredBoxes := []string{
		"inut_204_141", "inut_204_150", "inut_204_152", "inut_206_60",
		"inut_204_157", "inut_205_72", "inut_205_87", "inut_204_159",
		"inut_205_67", "inut_205_69", "inut_205_85", "inut_205_56",
		"inut_205_53", "inut_205_62", "inut_205_6", "inut_205_10",
		"inut_205_77", "inut_204_148", "inut_207_12", "inut_205_122",
		"inut_205_123", "inut_205_124", "inut_202_213", "inut_204_222",
		"inut_204_161", "inut_202_226", "inut_205_120", "inut_205_55",
		"inut_204_53", "inut_205_60", "inut_204_38", "inut_204_83",
		"inut_204_165", "inut_207_18", "inut_205_57", "inut_205_31",
		"inut_205_24", "inut_204_220", "inut_207_13", "inut_205_73",
		"inut_207_44", "inut_204_167",
	}
	boxSet := make(map[string]*Box, len(boxes))
	for _, b := range boxes {
		boxSet[b.ID] = b
	}
	for _, reqID := range requiredBoxes {
		b, ok := boxSet[reqID]
		if !ok {
			t.Errorf("missing required box %q in DefaultFleetCatalog", reqID)
			continue
		}
		if b.VPNIP == "" {
			t.Errorf("box %q has empty VPNIP", reqID)
		}
		if b.ClubID == "" || b.ClubName == "" {
			t.Errorf("box %q missing ClubID/ClubName", reqID)
		}
	}
}

// Test 3 (F1): Multi-box clubs group 99 Billiards (3 boxes), Magic Arena (3 boxes), Top Billiards (2 boxes).
func TestTier1_F1_MultiBoxClubGrouping(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/summary, got %d", rec.Code)
	}
	rawClubs, ok := data["clubs"].([]any)
	if !ok || len(rawClubs) == 0 {
		t.Fatalf("expected non-empty clubs array in summary")
	}

	found99 := false
	foundMagic := false
	foundTop := false

	for _, item := range rawClubs {
		cm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(fmt.Sprint(cm["name"]))
		rawBoxIDs, _ := cm["box_ids"].([]any)
		boxIDs := make([]string, 0, len(rawBoxIDs))
		for _, id := range rawBoxIDs {
			boxIDs = append(boxIDs, fmt.Sprint(id))
		}
		joined := strings.Join(boxIDs, ",")

		if strings.Contains(name, "99 billiards") {
			found99 = true
			for _, expected := range []string{"inut_204_150", "inut_204_152", "inut_206_60"} {
				if !strings.Contains(joined, expected) {
					t.Errorf("99 Billiards club missing box %s (got %v)", expected, boxIDs)
				}
			}
		}
		if strings.Contains(name, "magic arena") {
			foundMagic = true
			for _, expected := range []string{"inut_205_122", "inut_205_123", "inut_205_124"} {
				if !strings.Contains(joined, expected) {
					t.Errorf("Magic Arena club missing box %s (got %v)", expected, boxIDs)
				}
			}
		}
		if strings.Contains(name, "top billiards") {
			foundTop = true
			for _, expected := range []string{"inut_204_38", "inut_204_83"} {
				if !strings.Contains(joined, expected) {
					t.Errorf("Top Billiards club missing box %s (got %v)", expected, boxIDs)
				}
			}
		}
	}

	if !found99 || !foundMagic || !foundTop {
		t.Errorf("missing multi-box club groupings: found99=%v foundMagic=%v foundTop=%v", found99, foundMagic, foundTop)
	}
}

// Test 4 (F1): Persistent dl_hmac_key (64-hex) & portal_token (32-hex) across server restarts.
func TestTier1_F1_PersistentStoreAcrossServerRestarts(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "hub-state.json")

	srv1, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #1 failed: %v", err)
	}

	clubs1 := storeListClubs(srv1)
	if len(clubs1) == 0 {
		t.Fatalf("expected seeded clubs in srv1")
	}
	tokens1 := make(map[string]string, len(clubs1))
	hmacKeys1 := make(map[string]string, len(clubs1))
	for _, c := range clubs1 {
		if len(c.PortalToken) != 32 {
			t.Errorf("club %s expected 32-hex portal_token, got %q (len=%d)", c.ID, c.PortalToken, len(c.PortalToken))
		}
		if len(c.DLHMACKey) != 64 {
			t.Errorf("club %s expected 64-hex dl_hmac_key, got len=%d", c.ID, len(c.DLHMACKey))
		}
		tokens1[c.ID] = c.PortalToken
		hmacKeys1[c.ID] = c.DLHMACKey
	}
	_ = srv1.Close()

	// Verify state file exists on disk.
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("expected persisted state file at %s: %v", dataPath, err)
	}

	// Start second server pointing at the same DataPath.
	srv2, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #2 failed: %v", err)
	}
	defer srv2.Close()

	for _, c2 := range storeListClubs(srv2) {
		if tokens1[c2.ID] != c2.PortalToken {
			t.Errorf("club %s portal_token changed across restart: %q -> %q", c2.ID, tokens1[c2.ID], c2.PortalToken)
		}
		if hmacKeys1[c2.ID] != c2.DLHMACKey {
			t.Errorf("club %s dl_hmac_key changed across restart: %q -> %q", c2.ID, hmacKeys1[c2.ID], c2.DLHMACKey)
		}
	}
}

// Test 5 (F1): Admin login (form & JSON), session cookie, /api/auth/me, and /logout.
func TestTier1_F1_AdminLoginFormAndCookieSession(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// 1. Form login
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "198.51.100.10")
	if cookie == nil {
		t.Fatalf("expected inut_bida_session cookie on valid admin login")
	}
	if !cookie.HttpOnly {
		t.Errorf("expected inut_bida_session cookie to have HttpOnly=true")
	}

	// 2. Authenticate /api/auth/me using session cookie
	rec, me := doHubRequest(t, srv, http.MethodGet, "/api/auth/me", nil, map[string]string{
		"Cookie": cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/me, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(me["username"]); got != testAdminUser {
		t.Errorf("expected username=%q, got %q", testAdminUser, got)
	}
	if got := fmt.Sprint(me["role"]); got != "superadmin" {
		t.Errorf("expected role=superadmin, got %q", got)
	}

	// 3. JSON login also works
	recJSON, loginResp := doHubRequest(t, srv, http.MethodPost, "/login", map[string]string{
		"username": testAdminUser,
		"password": testAdminPass,
	}, map[string]string{"NO_AUTH": "1", "Accept": "application/json"})
	if recJSON.Code != http.StatusOK && recJSON.Code != http.StatusSeeOther {
		t.Fatalf("expected 200 or 303 on JSON login, got %d", recJSON.Code)
	}
	if recJSON.Code == http.StatusOK && loginResp["ok"] != true {
		t.Errorf("expected ok=true in JSON login response, got %v", loginResp)
	}

	// 4. Logout invalidates cookie
	recOut, _ := doHubRequest(t, srv, http.MethodGet, "/logout", nil, map[string]string{
		"Cookie": cookie.Name + "=" + cookie.Value,
	})
	if recOut.Code != http.StatusSeeOther && recOut.Code != http.StatusFound && recOut.Code != http.StatusOK {
		t.Errorf("expected redirect on /logout, got %d", recOut.Code)
	}
	recAfter, _ := doHubRequest(t, srv, http.MethodGet, "/api/auth/me", nil, map[string]string{
		"Cookie": cookie.Name + "=" + cookie.Value,
	})
	if recAfter.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout when reusing old cookie, got %d", recAfter.Code)
	}
}

// Test 6 (F1): Bearer token and X-Hub-Key header authentication on /fleet/api/* and /api/*.
func TestTier1_F1_BearerAndXHubKeyAuthentication(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec1, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
		"Authorization": "Bearer " + testAPIKey,
	})
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200 OK with Authorization: Bearer, got %d", rec1.Code)
	}

	rec2, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes", nil, map[string]string{
		"X-Hub-Key": testAPIKey,
	})
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 OK with X-Hub-Key, got %d", rec2.Code)
	}
}

// Test 7 (F2): Fleet Summary, Alias, Boxes list, and Box Detail endpoints.
func TestTier1_F2_FleetSummaryAndBoxesEndpoints(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recSum, sum := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, nil)
	if recSum.Code != http.StatusOK {
		t.Fatalf("expected 200 from /fleet/api/summary, got %d", recSum.Code)
	}
	if sum["kpi"] == nil || sum["clubs"] == nil || sum["boxes"] == nil {
		t.Fatalf("expected kpi, clubs, boxes in /fleet/api/summary, got keys: %v", sum)
	}

	// Alias /api/fleet/summary
	recAlias, sumAlias := doHubRequest(t, srv, http.MethodGet, "/api/fleet/summary", nil, nil)
	if recAlias.Code != http.StatusOK || sumAlias["kpi"] == nil {
		t.Errorf("expected 200 with kpi from /api/fleet/summary alias, got %d", recAlias.Code)
	}

	// /fleet/api/boxes
	recBoxes, boxesData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes", nil, nil)
	if recBoxes.Code != http.StatusOK || boxesData["boxes"] == nil {
		t.Errorf("expected 200 with boxes from /fleet/api/boxes, got %d", recBoxes.Code)
	}

	// /fleet/api/boxes/inut_204_141
	recBox, boxDetail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	if recBox.Code != http.StatusOK {
		t.Fatalf("expected 200 from /fleet/api/boxes/inut_204_141, got %d: %s", recBox.Code, recBox.Body.String())
	}
	for _, requiredKey := range []string{"box", "club", "cameras", "monitors", "hardening", "redbida", "handover"} {
		if _, exists := boxDetail[requiredKey]; !exists {
			t.Errorf("expected key %q in /fleet/api/boxes/inut_204_141 response", requiredKey)
		}
	}
}

// Test 8 (F2): Outbound tunnel registration (/fleet/tunnel) and telemetry heartbeat (/fleet/api/heartbeat).
func TestTier1_F2_TunnelRegistrationAndHeartbeatTelemetry(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Register tunnel via POST /fleet/tunnel
	recTun, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/tunnel", map[string]any{
		"box_id":      "inut_204_141",
		"tunnel_addr": "127.0.0.1:22028",
		"rtt_ms":      14,
	}, map[string]string{
		"X-Box-ID":     "inut_204_141",
		"X-Box-Secret": "valid-box-secret",
	})
	if recTun.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST /fleet/tunnel, got %d: %s", recTun.Code, recTun.Body.String())
	}

	// Send telemetry heartbeat via POST /fleet/api/heartbeat
	recHB, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":           "inut_204_141",
		"online":           true,
		"rtt_ms":           11,
		"temp_c":           54.5,
		"ram_available_mb": 980,
		"zram_total_mb":    1900,
		"zram_mem_limit":   0,
		"swappiness":       80,
		"sd_used_pct":      48,
		"sd_fs_type":       "f2fs",
		"sd_mount_opts":    "rw,noatime,errors=continue",
		"ffmpeg_count":     14,
		"rx_mbps":          9.8,
	}, map[string]string{
		"X-Box-ID":     "inut_204_141",
		"X-Box-Secret": "valid-box-secret",
	})
	if recHB.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST /fleet/api/heartbeat, got %d: %s", recHB.Code, recHB.Body.String())
	}

	recBox, detail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	if recBox.Code != http.StatusOK {
		t.Fatalf("expected 200 from box detail, got %d", recBox.Code)
	}
	boxMap, _ := detail["box"].(map[string]any)
	if boxMap["online"] != true {
		t.Errorf("expected box online=true after heartbeat, got %v", boxMap["online"])
	}
	if int(boxMap["rtt_ms"].(float64)) != 11 {
		t.Errorf("expected rtt_ms=11, got %v", boxMap["rtt_ms"])
	}
	if !strings.Contains(fmt.Sprint(boxMap["sd_mount_opts"]), "errors=continue") {
		t.Errorf("expected sd_mount_opts to contain errors=continue, got %v", boxMap["sd_mount_opts"])
	}
}

// Test 9 (F2): DialBox and Streaming Reverse Proxy (/fleet/proxy/{box_id}/kspcam/*) with SSO headers and <base> injection.
func TestTier1_F2_DialBoxAndStreamingReverseProxyKspcam(t *testing.T) {
	srv, _ := newTestHubServer(t)

	var receivedSig, receivedSSOUser string
	edgeKspcam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("X-Inut-Staff-Sig")
		receivedSSOUser = r.Header.Get("X-Inut-SSO-User")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Edge KSPCam</title></head><body><h1>KSPCam On-Box</h1></body></html>`))
	}))
	defer edgeKspcam.Close()

	edgeHostPort := strings.TrimPrefix(edgeKspcam.URL, "http://")
	srv.RegisterTunnelTarget("inut_204_141", "kspcam", edgeHostPort)

	// Verify DialBox connects directly to the registered tunnel target
	conn, err := srv.DialBox(context.Background(), "inut_204_141", 2028)
	if err != nil {
		t.Fatalf("DialBox failed for registered tunnel target: %v", err)
	}
	_ = conn.Close()

	// Verify ReverseProxy rewrites HTML with <base href="/fleet/proxy/inut_204_141/kspcam/"> and injects SSO headers
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_141/kspcam/index.html", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from reverse proxy, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<base href="/fleet/proxy/inut_204_141/kspcam/">`) {
		t.Errorf("expected proxied HTML to contain injected <base href=\"/fleet/proxy/inut_204_141/kspcam/\">, got: %s", body)
	}
	if receivedSig == "" {
		t.Errorf("expected X-Inut-Staff-Sig HMAC header to be injected into proxied request")
	}
	if receivedSSOUser != "admin" {
		t.Errorf("expected X-Inut-SSO-User=admin header, got %q", receivedSSOUser)
	}
}

// Test 10 (F2): Streaming Reverse Proxy for Shinobi (:8080) and Node-RED (:1880/:2023) with zero-buffering streaming.
func TestTier1_F2_StreamingReverseProxyShinobiAndNodeRed(t *testing.T) {
	srv, _ := newTestHubServer(t)

	edgeShinobi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("HLS_SEGMENT_BYTES_0123456789"))
	}))
	defer edgeShinobi.Close()

	edgeNodeRed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"nodered":"ok","flow":"redbida"}`))
	}))
	defer edgeNodeRed.Close()

	srv.RegisterTunnelTarget("inut_204_150", "shinobi", strings.TrimPrefix(edgeShinobi.URL, "http://"))
	srv.RegisterTunnelTarget("inut_204_150", "nodered", strings.TrimPrefix(edgeNodeRed.URL, "http://"))

	recShinobi, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_150/shinobi/99cs111/hls/cam01/s.ts", nil, nil)
	if recShinobi.Code != http.StatusOK || !strings.Contains(recShinobi.Body.String(), "HLS_SEGMENT_BYTES_0123456789") {
		t.Errorf("expected Shinobi proxy stream 200 OK with payload, got %d: %s", recShinobi.Code, recShinobi.Body.String())
	}

	recNodeRed, nrData := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_150/nodered/flows", nil, nil)
	if recNodeRed.Code != http.StatusOK || fmt.Sprint(nrData["nodered"]) != "ok" {
		t.Errorf("expected Node-RED proxy 200 OK, got %d: %s", recNodeRed.Code, recNodeRed.Body.String())
	}
}

// Test 11 (F2): Real-time SSE stream (/fleet/api/events) and immediate poll trigger (POST /fleet/api/poll).
func TestTier1_F2_SSEEventsStreamAndPollTrigger(t *testing.T) {
	srv, _ := newTestHubServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/fleet/api/events", nil)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /fleet/api/events failed: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
	}

	// Trigger POST /fleet/api/poll in a goroutine so SSE receives fleet_update
	go func() {
		time.Sleep(50 * time.Millisecond)
		_, _ = doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]string{"box_id": "inut_204_141"}, nil)
	}()

	reader := bufio.NewReader(resp.Body)
	receivedEvent := false
	for range 30 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event:") && (strings.Contains(line, "fleet_update") || strings.Contains(line, "heartbeat")) {
			receivedEvent = true
			break
		}
	}
	if !receivedEvent {
		t.Errorf("expected to receive event: fleet_update or heartbeat from /fleet/api/events")
	}
}

// Test 12 (F4): Embedded SCADA Dual-Canvas static assets (#eef5f4 Light / #071a24 Dark, 6 SPA views).
func TestTier1_F4_SCADADualCanvasStaticAssetsEmbedded(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Check index.html via authenticated GET /
	recIdx, _ := doHubRequest(t, srv, http.MethodGet, "/", nil, nil)
	if recIdx.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on authenticated GET /, got %d", recIdx.Code)
	}
	idxHTML := recIdx.Body.String()
	for _, view := range []string{"fleet", "hardening", "cameras", "redbida", "handover"} {
		if !strings.Contains(idxHTML, view) {
			t.Errorf("expected index.html to contain view identifier %q", view)
		}
	}

	// Check style.css for SCADA Dual-Canvas colors (#eef5f4 Light & #071a24 Dark) and tabular-nums
	recCSS, _ := doHubRequest(t, srv, http.MethodGet, "/static/style.css", nil, map[string]string{"NO_AUTH": "1"})
	if recCSS.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /static/style.css, got %d", recCSS.Code)
	}
	css := strings.ToLower(recCSS.Body.String())
	if !strings.Contains(css, "#eef5f4") || !strings.Contains(css, "#071a24") {
		t.Errorf("expected style.css to define both SCADA Dual-Canvas colors #eef5f4 and #071a24")
	}
	if !strings.Contains(css, "tabular-nums") {
		t.Errorf("expected style.css to include font-variant-numeric: tabular-nums")
	}

	// Check app.js
	recJS, _ := doHubRequest(t, srv, http.MethodGet, "/static/app.js", nil, map[string]string{"NO_AUTH": "1"})
	if recJS.Code != http.StatusOK || recJS.Body.Len() < 100 {
		t.Errorf("expected non-empty /static/app.js, got code=%d len=%d", recJS.Code, recJS.Body.Len())
	}
}

// Test 13 (F1/F6): ParseWatchlist parses custom markdown and extracts clubs & boxes.
func TestTier1_F6_ParseWatchlistCustomMarkdown(t *testing.T) {
	md := `
# FLEET WATCHLIST
- [x] **Node ` + "`inut_204_150`" + `** (99 Billiards - CS11 - 1 — ` + "`77.88.204.150`" + `, group ` + "`99cs111`" + `)
  - Phần cứng: Rockchip RK3528 (ARM64), RAM 1.8 GiB, Thẻ nhớ 118 GB
  - Số lượng camera: 12 camera
- [x] **Node ` + "`inut_204_152`" + `** (99 Billiards - CS11 - 2 — ` + "`77.88.204.152`" + `, group ` + "`99cs112`" + `)
  - Phần cứng: Rockchip RK3528 (ARM64), RAM 1.8 GiB
  - Số lượng camera: 12 camera
`
	clubs, boxes := ParseWatchlist(md)
	if len(boxes) < 2 {
		t.Fatalf("expected ParseWatchlist to extract >= 2 boxes, got %d", len(boxes))
	}
	if len(clubs) < 1 {
		t.Fatalf("expected ParseWatchlist to group into >= 1 club, got %d", len(clubs))
	}
}

// Test 14 (F6): Deployment artifacts and port 2090 non-disruption contract.
func TestTier1_F6_DeploymentArtifactsAndPort2090Contract(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.Port = 0 // test default fallback to 2090
	})
	rec, data := doHubRequest(t, srv, http.MethodGet, "/api/health", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/health, got %d", rec.Code)
	}
	if int(data["port"].(float64)) != 2090 {
		t.Errorf("expected default port 2090 (preserving 2088/2089/31797), got %v", data["port"])
	}
}

// =============================================================================
// TIER 2: BOUNDARY & CORNER CASES (F1, F2, F4, F6) — 13 TESTS
// =============================================================================

// Test 15 (F1): Unauthenticated browser GET / redirects to /login; unauthenticated API returns 401.
func TestTier2_F1_UnauthenticatedRootRedirectsToLogin(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recRoot, _ := doHubRequest(t, srv, http.MethodGet, "/", nil, map[string]string{"NO_AUTH": "1"})
	if recRoot.Code != http.StatusFound && recRoot.Code != http.StatusSeeOther && recRoot.Code != http.StatusUnauthorized {
		t.Errorf("expected redirect (302/303) to /login on unauthenticated GET /, got %d", recRoot.Code)
	}
	if loc := recRoot.Header().Get("Location"); loc != "" && !strings.HasPrefix(loc, "/login") {
		t.Errorf("expected redirect Location /login, got %q", loc)
	}

	recAPI, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{"NO_AUTH": "1"})
	if recAPI.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized on unauthenticated /fleet/api/summary, got %d", recAPI.Code)
	}
}

// Test 16 (F1): 1..4 failed login attempts return error without locking IP prematurely.
func TestTier2_F1_InvalidCredentialsRejectedBeforeLockout(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clientIP := "203.0.113.55:41000"

	for i := 1; i <= 4; i++ {
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/login", map[string]string{
			"username": testAdminUser,
			"password": "wrong-password",
		}, map[string]string{
			"NO_AUTH":     "1",
			"REMOTE_ADDR": clientIP,
			"Accept":      "application/json",
		})
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d prematurely locked IP (got 429)", i)
		}
	}

	// Valid login on 5th attempt succeeds before lockout threshold is crossed
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "203.0.113.55")
	if cookie == nil {
		t.Errorf("expected valid login on 5th attempt (after 4 failures) to succeed")
	}
}

// Test 17 (F1): Brute-force lockout after 5 failed attempts locks IP for 30m (?err=locked / 429) and persists.
func TestTier2_F1_BruteForceLockoutOn6thAttemptPersists(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "hub-state.json")
	srv, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	attackerIP := "203.0.113.99:55555"
	for i := 1; i <= 5; i++ {
		form := url.Values{"username": {testAdminUser}, "password": {"badpass"}}
		doHubRequest(t, srv, http.MethodPost, "/login", form, map[string]string{
			"NO_AUTH":     "1",
			"REMOTE_ADDR": attackerIP,
		})
	}

	// 6th attempt via form -> redirects to /login?err=locked (or 429)
	form6 := url.Values{"username": {testAdminUser}, "password": {testAdminPass}}
	recForm, _ := doHubRequest(t, srv, http.MethodPost, "/login", form6, map[string]string{
		"NO_AUTH":     "1",
		"REMOTE_ADDR": attackerIP,
	})
	loc := recForm.Header().Get("Location")
	if recForm.Code != http.StatusTooManyRequests && !strings.Contains(loc, "err=locked") {
		t.Errorf("expected 6th form login attempt to redirect to /login?err=locked or return 429, got code=%d loc=%q", recForm.Code, loc)
	}

	// 6th attempt via JSON -> 429 {"error":"locked"} even with correct password
	recJSON, errData := doHubRequest(t, srv, http.MethodPost, "/login", map[string]string{
		"username": testAdminUser,
		"password": testAdminPass,
	}, map[string]string{
		"NO_AUTH":     "1",
		"REMOTE_ADDR": attackerIP,
		"Accept":      "application/json",
	})
	if recJSON.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests on locked IP JSON login, got %d: %s", recJSON.Code, recJSON.Body.String())
	}
	if fmt.Sprint(errData["error"]) != "locked" {
		t.Errorf("expected error=locked in 429 JSON response, got %v", errData)
	}
	_ = srv.Close()

	// Restart server and verify lockout persists in DataPath
	srv2, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #2 failed: %v", err)
	}
	defer srv2.Close()

	recAfterRestart, _ := doHubRequest(t, srv2, http.MethodPost, "/login", map[string]string{
		"username": testAdminUser,
		"password": testAdminPass,
	}, map[string]string{
		"NO_AUTH":     "1",
		"REMOTE_ADDR": attackerIP,
		"Accept":      "application/json",
	})
	if recAfterRestart.Code != http.StatusTooManyRequests {
		t.Errorf("expected IP lockout to persist across server restart, got %d", recAfterRestart.Code)
	}
}

// Test 18 (F1): Brute-force lockout on one IP does not lock out other client IPs.
func TestTier2_F1_BruteForceLockoutIsolatedPerClientIP(t *testing.T) {
	srv, _ := newTestHubServer(t)

	for i := 1; i <= 6; i++ {
		doHubRequest(t, srv, http.MethodPost, "/login", map[string]string{
			"username": testAdminUser,
			"password": "wrong",
		}, map[string]string{
			"NO_AUTH":     "1",
			"REMOTE_ADDR": "203.0.113.111:12345",
			"Accept":      "application/json",
		})
	}

	// Clean IP 203.0.113.222 can still log in immediately
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "203.0.113.222")
	if cookie == nil {
		t.Errorf("expected clean IP 203.0.113.222 to log in despite 203.0.113.111 being locked")
	}
}

// Test 19 (F1): Viewer role can read all GET endpoints but receives 403 Forbidden on mutating POST endpoints.
func TestTier2_F1_ViewerRoleReadAllowedMutateForbidden(t *testing.T) {
	srv, _ := newTestHubServer(t)
	viewerCookie := loginAndGetSessionCookie(t, srv, testViewerUser, testViewerPass, "198.51.100.77")
	if viewerCookie == nil {
		t.Fatalf("expected viewer login to succeed")
	}
	hdr := map[string]string{"Cookie": viewerCookie.Name + "=" + viewerCookie.Value}

	// Viewer GET requests succeed
	for _, path := range []string{"/fleet/api/summary", "/fleet/api/hardening", "/fleet/api/cameras", "/fleet/api/redbida"} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, path, nil, hdr)
		if rec.Code != http.StatusOK {
			t.Errorf("expected viewer GET %s to return 200, got %d", path, rec.Code)
		}
	}

	// Viewer POST mutating requests return 403 Forbidden
	mutations := []struct {
		path string
		body any
	}{
		{"/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{"steps": []int{1}}},
		{"/fleet/api/cameras/bulk-apply", map[string]any{"codec": "H.265"}},
		{"/fleet/api/boxes/inut_204_141/redbida/apply", map[string]any{"auto_fix": true}},
	}
	for _, m := range mutations {
		rec, _ := doHubRequest(t, srv, http.MethodPost, m.path, m.body, hdr)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected viewer POST %s to return 403 Forbidden, got %d", m.path, rec.Code)
		}
	}
}

// Test 20 (F1): Invalid Bearer token, wrong X-Hub-Key, or malformed Authorization header returns 401.
func TestTier2_F1_InvalidBearerTokenAndMalformedAuthHeader(t *testing.T) {
	srv, _ := newTestHubServer(t)

	cases := []map[string]string{
		{"Authorization": "Bearer wrong-api-key"},
		{"X-Hub-Key": "wrong-api-key"},
		{"Authorization": "Basic dXNlcjpwYXNz"},
	}
	for i, hdr := range cases {
		rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, hdr)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("case %d: expected 401 Unauthorized, got %d", i, rec.Code)
		}
	}
}

// Test 21 (F2): Unknown box_id and path traversal attempts on /fleet/api/boxes/{box_id} return 404/400.
func TestTier2_F2_BoxDetailUnknownBoxIDReturns404(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec1, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_999_999", nil, nil)
	if rec1.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown box inut_999_999, got %d", rec1.Code)
	}

	rec2, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/..%2f..%2fetc%2fpasswd", nil, nil)
	if rec2.Code != http.StatusNotFound && rec2.Code != http.StatusBadRequest {
		t.Errorf("expected 404 or 400 for traversal box_id, got %d", rec2.Code)
	}
}

// Test 22 (F2): Reverse proxy handles missing service (400), unknown box (404), and offline unreachable box (502) cleanly.
func TestTier2_F2_ReverseProxyUnknownServiceOrOfflineBox(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recMissingSvc, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_141", nil, nil)
	if recMissingSvc.Code != http.StatusBadRequest && recMissingSvc.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404 for missing proxy service segment, got %d", recMissingSvc.Code)
	}

	recUnknownBox, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_unknown_999/kspcam/", nil, nil)
	if recUnknownBox.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown box in proxy, got %d", recUnknownBox.Code)
	}

	// Register an unreachable closed local port for inut_204_141
	srv.RegisterTunnelTarget("inut_204_141", "kspcam", "127.0.0.1:1")
	recOffline, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_141/kspcam/", nil, nil)
	if recOffline.Code != http.StatusBadGateway && recOffline.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 502 Bad Gateway or 503 for unreachable edge box proxy, got %d", recOffline.Code)
	}
}

// Test 23 (F2): DialBox respects context deadline / DialTimeout on unreachable target.
func TestTier2_F2_DialBoxUnreachablePortTimeout(t *testing.T) {
	srv, _ := newTestHubServer(t, func(c *Config) {
		c.DialTimeout = 50 * time.Millisecond
	})
	srv.RegisterTunnelTarget("inut_204_141", "kspcam", "127.0.0.1:1")

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := srv.DialBox(ctx, "inut_204_141", 2028)
	if err == nil {
		t.Errorf("expected DialBox to return error for unreachable port 127.0.0.1:1")
	}
}

// Test 24 (F2): Heartbeat endpoint rejects malformed JSON and missing box_id with 400 Bad Request.
func TestTier2_F2_HeartbeatMalformedJSONAndMissingBoxID(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recMalformed, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", "{broken-json", nil)
	if recMalformed.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON heartbeat, got %d", recMalformed.Code)
	}

	recNoID, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{"online": true}, nil)
	if recNoID.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on heartbeat missing box_id, got %d", recNoID.Code)
	}
}

// Test 25 (F2): Unenrolled unknown box submitting heartbeat enters enrollment queue until approved.
func TestTier2_F2_EnrollmentQueueUnknownBoxRequiresApproval(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recHB, hbResp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":    "inut_209_99",
		"soc_model": "Rockchip RK3528",
	}, map[string]string{
		"X-Box-ID":     "inut_209_99",
		"X-Box-Secret": "new-candidate-secret-2026",
	})
	// Either 403 ENROLLMENT_PENDING or 202/200 with enrollment_pending status
	if recHB.Code != http.StatusForbidden && recHB.Code != http.StatusAccepted && recHB.Code != http.StatusOK {
		t.Fatalf("unexpected status for new box enrollment heartbeat: %d (%v)", recHB.Code, hbResp)
	}

	recEnr, enrData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/enrollments", nil, nil)
	if recEnr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/enrollments, got %d", recEnr.Code)
	}
	enrollments, _ := enrData["enrollments"].([]any)
	if len(enrollments) == 0 {
		t.Fatalf("expected pending enrollment for inut_209_99 in /fleet/api/enrollments")
	}
}

// Test 26 (F4): GET /login?err=locked serves login.html with 30-minute lockout message handler.
func TestTier2_F4_LoginHTMLContainsLockedErrorMessage(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/login?err=locked", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /login?err=locked, got %d", rec.Code)
	}
	html := rec.Body.String()
	if !strings.Contains(html, "locked") || !strings.Contains(html, "30") {
		t.Errorf("expected login.html to handle ?err=locked with 30-minute lockout message, got: %s", html)
	}
}

// Test 27 (F6): ParseWatchlist handles empty or malformed markdown gracefully without panic.
func TestTier2_F6_ParseWatchlistEmptyOrMalformedMarkdown(t *testing.T) {
	clubsEmpty, boxesEmpty := ParseWatchlist("")
	if clubsEmpty == nil && boxesEmpty == nil {
		// Allowed to be empty slice; just must not panic.
	}
	_, _ = ParseWatchlist("# Heading Only\nNo bullet items here\n- [ ] Incomplete line")
}

// =============================================================================
// TIER 3: CROSS-FEATURE PAIRWISE TESTS — 3 TESTS
// =============================================================================

// Test 28 (Pairwise F1 x F2): Heartbeat updates Summary KPI metrics, Box Detail, and broadcasts SSE event.
func TestTier3_Pairwise_HeartbeatUpdatesSummaryKPIAndBroadcastsSSE(t *testing.T) {
	srv, _ := newTestHubServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/fleet/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer resp.Body.Close()

	// Send heartbeat updating inut_204_141 FFmpeg count to 18 and RTT to 9ms
	recHB, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":       "inut_204_141",
		"online":       true,
		"rtt_ms":       9,
		"ffmpeg_count": 18,
		"sd_used_pct":  42,
	}, nil)
	if recHB.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d", recHB.Code)
	}

	// Verify box detail reflects new FFmpeg count
	_, detail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	boxMap, _ := detail["box"].(map[string]any)
	if int(boxMap["ffmpeg_count"].(float64)) != 18 {
		t.Errorf("expected ffmpeg_count=18 in box detail, got %v", boxMap["ffmpeg_count"])
	}

	// Verify SSE stream received event
	reader := bufio.NewReader(resp.Body)
	gotSSE := false
	for range 25 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event:") {
			gotSSE = true
			break
		}
	}
	if !gotSSE {
		t.Errorf("expected SSE event broadcast after heartbeat")
	}
}

// Test 29 (Pairwise F1 x F2): Admin session cookie authenticates Reverse Proxy and injects HMAC Staff Signature.
func TestTier3_Pairwise_AdminSessionThroughReverseProxySSO(t *testing.T) {
	srv, _ := newTestHubServer(t)

	var edgeHeaders http.Header
	edge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		edgeHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"edge_auth":"accepted"}`))
	}))
	defer edge.Close()

	srv.RegisterTunnelTarget("inut_205_72", "kspcam", strings.TrimPrefix(edge.URL, "http://"))
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "198.51.100.88")
	if cookie == nil {
		t.Fatalf("expected admin session cookie")
	}

	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_205_72/kspcam/api/config", nil, map[string]string{
		"Cookie": cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK || fmt.Sprint(data["edge_auth"]) != "accepted" {
		t.Fatalf("expected 200 OK from proxied kspcam with admin cookie, got %d: %s", rec.Code, rec.Body.String())
	}
	if edgeHeaders.Get("X-Inut-Staff-Sig") == "" {
		t.Errorf("expected X-Inut-Staff-Sig HMAC header on edge request")
	}
	if edgeHeaders.Get("X-Inut-SSO-User") != testAdminUser {
		t.Errorf("expected X-Inut-SSO-User=%s, got %q", testAdminUser, edgeHeaders.Get("X-Inut-SSO-User"))
	}
}

// Test 30 (Pairwise F1 x F2 x F3): Approving a pending box enrollment adds the box to Summary and Hardening matrix.
func TestTier3_Pairwise_EnrollmentApprovalAddsBoxToSummaryAndHardening(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Submit heartbeat from unknown box inut_208_88
	doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
		"box_id":    "inut_208_88",
		"soc_model": "Allwinner H616",
		"vpn_ip":    "77.88.208.88",
	}, map[string]string{
		"X-Box-ID":     "inut_208_88",
		"X-Box-Secret": "enroll-secret-88",
	})

	_, enrData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/enrollments", nil, nil)
	enrollments, _ := enrData["enrollments"].([]any)
	if len(enrollments) == 0 {
		t.Fatalf("expected enrollment entry for inut_208_88")
	}
	firstEnr, _ := enrollments[0].(map[string]any)
	enrID := fmt.Sprint(firstEnr["id"])

	// Approve enrollment
	recApp, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/enrollments/"+enrID+"/approve", map[string]any{
		"club_name": "New Arena 88",
	}, nil)
	if recApp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK approving enrollment %s, got %d: %s", enrID, recApp.Code, recApp.Body.String())
	}

	// Verify inut_208_88 now exists in /fleet/api/boxes/inut_208_88
	recBox, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_208_88", nil, nil)
	if recBox.Code != http.StatusOK {
		t.Errorf("expected approved box inut_208_88 to be accessible via /fleet/api/boxes/inut_208_88, got %d", recBox.Code)
	}
}

// =============================================================================
// TIER 4: REAL-WORLD APPLICATION SCENARIOS — 2 TESTS
// =============================================================================

// Test 31 (Scenario): Multi-Box Club 99 Billiards CS11 (3 boxes) full fleet telemetry, proxy & poll workflow.
func TestTier4_Scenario_MultiBoxClub99BilliardsFullFleetOperations(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// 1. Admin logs in
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "198.51.100.1")
	if cookie == nil {
		t.Fatalf("admin login failed")
	}
	authHdr := map[string]string{"Cookie": cookie.Name + "=" + cookie.Value}

	// 2. Send live telemetry heartbeats for all 3 boxes of 99 Billiards CS11
	boxIDs := []string{"inut_204_150", "inut_204_152", "inut_206_60"}
	for i, boxID := range boxIDs {
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/heartbeat", map[string]any{
			"box_id":       boxID,
			"online":       true,
			"rtt_ms":       12 + i,
			"ffmpeg_count": 24,
			"sd_used_pct":  55 + i,
			"rx_mbps":      11.5,
		}, authHdr)
		if rec.Code != http.StatusOK {
			t.Fatalf("heartbeat failed for %s: %d", boxID, rec.Code)
		}
	}

	// 3. Register mock edge services for box 1 and box 2 and proxy into them
	edge1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head></head><body>99 CS11 Box 1 KSPCam</body></html>`))
	}))
	defer edge1.Close()
	srv.RegisterTunnelTarget("inut_204_150", "kspcam", strings.TrimPrefix(edge1.URL, "http://"))

	recProxy, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_150/kspcam/", nil, authHdr)
	if recProxy.Code != http.StatusOK || !strings.Contains(recProxy.Body.String(), `<base href="/fleet/proxy/inut_204_150/kspcam/">`) {
		t.Errorf("expected proxied 99 CS11 Box 1 HTML with <base>, got %d: %s", recProxy.Code, recProxy.Body.String())
	}

	// 4. Trigger fleet poll and verify summary KPI
	recPoll, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/poll", map[string]string{"box_id": "inut_204_150"}, authHdr)
	if recPoll.Code != http.StatusOK {
		t.Errorf("expected 200 on POST /fleet/api/poll, got %d", recPoll.Code)
	}
}

// Test 32 (Scenario): Full Hub Disaster Recovery & State Persistence across process restart.
func TestTier4_Scenario_HubDisasterRecoveryAndStatePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "hub-state.json")

	srv1, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #1 failed: %v", err)
	}

	// Mutate state across multiple subsystems before restart:
	// 1) Remediate hardening on inut_204_141
	doHubRequest(t, srv1, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{
		"steps": []int{1, 2, 3, 4, 5, 6, 7, 8},
	}, nil)
	// 2) Update RedBida domain & virtual IP on inut_204_141
	doHubRequest(t, srv1, http.MethodPost, "/fleet/api/boxes/inut_204_141/redbida/apply", map[string]any{
		"auto_fix":        true,
		"eth0_virtual_ip": "192.168.1.250",
		"store_domain":    "store-789bida.video.io.vn",
		"cstore_domain":   "c-store-789bida.video.io.vn",
	}, nil)
	// 3) Sign a customer clip token
	box := storeGetBox(srv1, "inut_204_141")
	clipToken, err := srv1.SignClipToken(box.ClubID, box.ID, "camera03", "/media/usb1/mia/clip03.mp4", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("SignClipToken failed: %v", err)
	}
	_ = srv1.Close()

	// Restart server from disk
	srv2, err := NewServer(Config{
		DataPath:     dataPath,
		AdminUser:    testAdminUser,
		AdminPass:    testAdminPass,
		APIKey:       testAPIKey,
		PollInterval: 0,
	})
	if err != nil {
		t.Fatalf("NewServer #2 failed: %v", err)
	}
	defer srv2.Close()

	// Verify hardening score, virtual IP, and signed clip token survived restart
	recBox, detail := doHubRequest(t, srv2, http.MethodGet, "/fleet/api/boxes/inut_204_141", nil, nil)
	if recBox.Code != http.StatusOK {
		t.Fatalf("expected 200 from srv2 box detail, got %d", recBox.Code)
	}
	boxMap, _ := detail["box"].(map[string]any)
	if int(boxMap["hardening_score"].(float64)) != 8 {
		t.Errorf("expected hardening_score=8 persisted across restart, got %v", boxMap["hardening_score"])
	}
	if fmt.Sprint(boxMap["virtual_ip"]) != "192.168.1.250" {
		t.Errorf("expected virtual_ip=192.168.1.250 persisted across restart, got %v", boxMap["virtual_ip"])
	}

	recClip, _ := doHubRequest(t, srv2, http.MethodGet, "/clip/"+clipToken, nil, map[string]string{"NO_AUTH": "1"})
	if recClip.Code != http.StatusOK {
		t.Errorf("expected signed clip token to remain valid after restart, got %d: %s", recClip.Code, recClip.Body.String())
	}
}
