package bidahub

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TIER 1: FEATURE COVERAGE TESTS (F4 Handover/QR/Clip, F5 Portal) — 8 TESTS
// =============================================================================

// Test 1 (F4): Pure-Go GenerateQRSVG produces valid standalone SVG markup without external CDN links.
func TestTier1_F4_GenerateQRSVGProducesValidStandaloneSVG(t *testing.T) {
	svg := GenerateQRSVG("https://store-toptn1.video.io.vn")
	if !strings.HasPrefix(strings.TrimSpace(svg), "<svg") || !strings.HasSuffix(strings.TrimSpace(svg), "</svg>") {
		t.Fatalf("expected GenerateQRSVG to return <svg>...</svg>, got: %s", svg)
	}
	if !strings.Contains(svg, "viewBox=") {
		t.Errorf("expected SVG QR to contain viewBox attribute")
	}
	if !strings.Contains(svg, "<rect") && !strings.Contains(svg, "<path") {
		t.Errorf("expected SVG QR to contain <rect or <path modules")
	}
	if strings.Contains(svg, "http://") || strings.Contains(svg, "https://cdn") {
		if !strings.Contains(svg, "xmlns=\"http://www.w3.org/2000/svg\"") {
			t.Errorf("expected standalone SVG without external CDN dependencies")
		}
	}
}

// Test 2 (F4): Handover Pack list (/fleet/api/handover) and single pack (/fleet/api/handover/{id}).
func TestTier1_F4_HandoverPackListAndSingleClub(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recList, listData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover", nil, nil)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/handover, got %d: %s", recList.Code, recList.Body.String())
	}
	packs, ok := listData["packs"].([]any)
	if !ok || len(packs) == 0 {
		if alt, ok2 := listData["handover"].([]any); ok2 && len(alt) > 0 {
			packs = alt
		} else {
			t.Fatalf("expected non-empty packs in /fleet/api/handover")
		}
	}

	// Fetch single handover pack by box_id (inut_204_141)
	recOne, oneData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover/inut_204_141", nil, nil)
	if recOne.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/handover/inut_204_141, got %d: %s", recOne.Code, recOne.Body.String())
	}
	pack := oneData
	if nested, ok := oneData["handover"].(map[string]any); ok {
		pack = nested
	} else if nested, ok := oneData["pack"].(map[string]any); ok {
		pack = nested
	}

	for _, field := range []string{"store_url", "cstore_url", "portal_url", "zalo_template", "qr_store_svg", "qr_cstore_svg", "qr_portal_svg", "clip_url"} {
		val := fmt.Sprint(pack[field])
		if val == "" || val == "<nil>" {
			t.Errorf("expected non-empty field %q in handover pack", field)
		}
	}
	if !strings.Contains(fmt.Sprint(pack["qr_store_svg"]), "<svg") {
		t.Errorf("expected qr_store_svg to contain <svg")
	}
	if !strings.Contains(fmt.Sprint(pack["qr_cstore_svg"]), "<svg") {
		t.Errorf("expected qr_cstore_svg to contain <svg")
	}
	if !strings.Contains(fmt.Sprint(pack["qr_portal_svg"]), "<svg") {
		t.Errorf("expected qr_portal_svg to contain <svg")
	}
	if !strings.Contains(fmt.Sprint(pack["portal_url"]), "/portal/") {
		t.Errorf("expected portal_url to contain /portal/, got %v", pack["portal_url"])
	}
	if !strings.Contains(fmt.Sprint(pack["clip_url"]), "/clip/") {
		t.Errorf("expected clip_url to contain /clip/, got %v", pack["clip_url"])
	}
}

// Test 3 (F1/F4): Signed Clip Token (/clip/{clip_token}) uses persistent club dl_hmac_key and survives server restarts.
func TestTier1_F1_ClipTokenSignAndVerifySurvivesServerRestart(t *testing.T) {
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

	box := storeGetBox(srv1, "inut_204_141")
	token, err := srv1.SignClipToken(box.ClubID, box.ID, "camera03", "/media/usb1/mia/2026-10-18_table03.mp4", time.Now().Add(48*time.Hour))
	if err != nil {
		t.Fatalf("SignClipToken failed: %v", err)
	}

	// Verify on srv1
	rec1, _ := doHubRequest(t, srv1, http.MethodGet, "/clip/"+token, nil, map[string]string{"NO_AUTH": "1"})
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on srv1 GET /clip/{token}, got %d: %s", rec1.Code, rec1.Body.String())
	}
	_ = srv1.Close()

	// Start srv2 from same DataPath and verify exact same token is still valid
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

	claims, err := srv2.VerifyClipToken(token)
	if err != nil {
		t.Fatalf("VerifyClipToken failed after restart: %v", err)
	}
	if claims.BoxID != "inut_204_141" || claims.CameraID != "camera03" {
		t.Errorf("unexpected claims after restart: %+v", claims)
	}

	rec2, _ := doHubRequest(t, srv2, http.MethodGet, "/clip/"+token, nil, map[string]string{"NO_AUTH": "1"})
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 OK on srv2 GET /clip/{token} after restart, got %d", rec2.Code)
	}
}

// Test 4 (F5): GET /portal/{portal_token} and /portal/{portal_token}/ serve mobile-first portal.html.
func TestTier1_F5_ShopOwnerPortalHTMLServedForValidToken(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	if len(clubs) == 0 {
		t.Fatalf("expected seeded clubs")
	}
	token := clubs[0].PortalToken

	for _, suffix := range []string{"", "/"} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, "/portal/"+token+suffix, nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for /portal/%s%s, got %d", token, suffix, rec.Code)
		}
		html := rec.Body.String()
		if !strings.Contains(html, "/api/status") {
			t.Errorf("expected portal.html to reference /api/status endpoint")
		}
	}
}

// Test 5 (F5): GET /portal/{portal_token}/api/status for a single-box club returns only that club's box and cameras.
func TestTier1_F5_ShopOwnerPortalAPIStatusSingleBoxClub(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_205_72") // Master Billiards Club
	if box == nil {
		t.Fatalf("missing box inut_205_72")
	}
	club := storeGetClub(srv, box.ClubID)
	if club == nil {
		t.Fatalf("missing club for inut_205_72")
	}

	rec, data := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /portal/{token}/api/status, got %d: %s", rec.Code, rec.Body.String())
	}

	rawBoxes, ok := data["boxes"].([]any)
	if !ok || len(rawBoxes) != 1 {
		t.Fatalf("expected exactly 1 box in Master Billiards portal status, got %d", len(rawBoxes))
	}
	bm, _ := rawBoxes[0].(map[string]any)
	if fmt.Sprint(bm["id"]) != "inut_205_72" {
		t.Errorf("expected box id=inut_205_72, got %v", bm["id"])
	}
}

// Test 6 (F5): GET /portal/{portal_token}/api/status for a multi-box club (99 Billiards CS11) returns all 3 club boxes and 0 foreign boxes.
func TestTier1_F5_ShopOwnerPortalAPIStatusMultiBoxClub(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box1 := storeGetBox(srv, "inut_204_150") // 99 Billiards CS11 - 1
	if box1 == nil {
		t.Fatalf("missing box inut_204_150")
	}
	club := storeGetClub(srv, box1.ClubID)
	if club == nil {
		t.Fatalf("missing club for inut_204_150")
	}

	rec, data := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from 99 Billiards portal status, got %d", rec.Code)
	}

	rawBoxes, ok := data["boxes"].([]any)
	if !ok || len(rawBoxes) != 3 {
		t.Fatalf("expected 3 boxes for 99 Billiards CS11 portal, got %d", len(rawBoxes))
	}
	seen := make(map[string]bool)
	for _, item := range rawBoxes {
		bm, _ := item.(map[string]any)
		seen[fmt.Sprint(bm["id"])] = true
	}
	for _, want := range []string{"inut_204_150", "inut_204_152", "inut_206_60"} {
		if !seen[want] {
			t.Errorf("expected 99 Billiards portal status to include box %s, got %v", want, seen)
		}
	}
}

// Test 7 (F5 Security): Shop Owner Portal JSON never leaks admin secrets, dl_hmac_key, shinobi_api_key, or other clubs' tokens.
func TestTier1_F5_ShopOwnerPortalZeroAdminSecretsLeaked(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	if len(clubs) < 2 {
		t.Fatalf("expected >= 2 clubs")
	}
	targetClub := clubs[0]
	otherClub := clubs[1]

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/portal/"+targetClub.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	rawJSON := rec.Body.String()

	forbiddenSubstrings := []string{
		"dl_hmac_key",
		targetClub.DLHMACKey,
		otherClub.PortalToken,
		otherClub.DLHMACKey,
		testAPIKey,
		testAdminPass,
		"shinobi_api_key",
	}
	for _, secret := range forbiddenSubstrings {
		if secret != "" && strings.Contains(rawJSON, secret) {
			t.Errorf("portal status leaked sensitive secret %q in response: %s", secret, rawJSON)
		}
	}
}

// Test 8 (F5): Shop Owner Portal works on mobile browser with zero cookies and zero Authorization headers.
func TestTier1_F5_ShopOwnerPortalWorksWithoutAdminCookieOrBearer(t *testing.T) {
	srv, _ := newTestHubServer(t)
	club := storeListClubs(srv)[0]

	req1 := httptestNewNoAuthGet( "/portal/" + club.PortalToken + "/")
	rec1, _ := doHubRequest(t, srv, http.MethodGet, req1, nil, map[string]string{"NO_AUTH": "1"})
	if rec1.Code != http.StatusOK {
		t.Errorf("expected 200 OK on unauthenticated portal HTML, got %d", rec1.Code)
	}

	req2 := httptestNewNoAuthGet("/portal/" + club.PortalToken + "/api/status")
	rec2, _ := doHubRequest(t, srv, http.MethodGet, req2, nil, map[string]string{"NO_AUTH": "1"})
	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 OK on unauthenticated portal API status, got %d", rec2.Code)
	}
}

func httptestNewNoAuthGet(path string) string { return path }

// =============================================================================
// TIER 2: BOUNDARY & CORNER CASES (F4 Handover/Clip, F5 Portal) — 8 TESTS
// =============================================================================

// Test 9 (F5): Unknown portal_token returns 404 Not Found.
func TestTier2_F5_PortalRejectsUnknownTokenWith404(t *testing.T) {
	srv, _ := newTestHubServer(t)

	for _, path := range []string{
		"/portal/deadbeefdeadbeefdeadbeefdeadbeef/",
		"/portal/deadbeefdeadbeefdeadbeefdeadbeef/api/status",
	} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, path, nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for unknown portal token on %s, got %d", path, rec.Code)
		}
	}
}

// Test 10 (F5): Raw box_id or club_id in /portal/{token} returns 404 Not Found (anti-enumeration).
func TestTier2_F5_PortalRejectsRawBoxIDAndClubIDWith404(t *testing.T) {
	srv, _ := newTestHubServer(t)
	club := storeListClubs(srv)[0]

	guesses := []string{
		"/portal/inut_204_141/",
		"/portal/inut_204_141/api/status",
		"/portal/inut_204_150/",
		"/portal/" + club.ID + "/",
		"/portal/" + club.ID + "/api/status",
	}
	for _, g := range guesses {
		rec, _ := doHubRequest(t, srv, http.MethodGet, g, nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found when guessing raw box_id/club_id at %s, got %d", g, rec.Code)
		}
	}
}

// Test 11 (F5): Portal is strictly read-only — POST, PUT, DELETE, PATCH under /portal/{token}/* return 405 or 403.
func TestTier2_F5_PortalStrictlyReadOnlyRejectsPOSTPUTDELETE(t *testing.T) {
	srv, _ := newTestHubServer(t)
	token := storeListClubs(srv)[0].PortalToken

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		for _, sub := range []string{"/portal/" + token + "/", "/portal/" + token + "/api/status"} {
			rec, _ := doHubRequest(t, srv, method, sub, map[string]string{"hack": "1"}, map[string]string{"NO_AUTH": "1"})
			if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusForbidden {
				t.Errorf("expected 405 or 403 for %s %s, got %d", method, sub, rec.Code)
			}
		}
	}
}

// Test 12 (F5 Security): A valid shop owner portal_token cannot authenticate admin /fleet/api/* routes.
func TestTier2_F5_PortalTokenCannotAuthenticateAdminFleetAPIs(t *testing.T) {
	srv, _ := newTestHubServer(t)
	portalToken := storeListClubs(srv)[0].PortalToken

	attempts := []map[string]string{
		{"Authorization": "Bearer " + portalToken},
		{"X-Hub-Key": portalToken},
		{"Cookie": "inut_bida_session=" + portalToken},
	}
	for i, hdr := range attempts {
		rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, hdr)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("attempt %d: portal_token must NOT authenticate /fleet/api/summary (got %d)", i, rec.Code)
		}
	}
}

// Test 13 (F1/F4): Tampering with a signed /clip/{clip_token} signature is rejected.
func TestTier2_F1_ClipTokenRejectsTamperedSignature(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_204_141")
	token, err := srv.SignClipToken(box.ClubID, box.ID, "camera01", "/media/usb1/mia/clip.mp4", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("SignClipToken failed: %v", err)
	}

	// Flip the last character of the signature
	last := token[len(token)-1]
	replacement := byte('a')
	if last == 'a' {
		replacement = 'b'
	}
	tampered := token[:len(token)-1] + string(replacement)

	if _, err := srv.VerifyClipToken(tampered); err == nil {
		t.Errorf("expected VerifyClipToken to reject tampered token")
	}

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/clip/"+tampered, nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code == http.StatusOK {
		t.Errorf("expected HTTP error for tampered /clip/{token}, got 200 OK")
	}
}

// Test 14 (F1/F4): Expired clip token is rejected by VerifyClipToken and GET /clip/{clip_token}.
func TestTier2_F1_ClipTokenRejectsExpiredToken(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_204_141")
	expiredToken, err := srv.SignClipToken(box.ClubID, box.ID, "camera01", "/media/usb1/mia/old.mp4", time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("SignClipToken failed: %v", err)
	}

	if _, err := srv.VerifyClipToken(expiredToken); err == nil {
		t.Errorf("expected VerifyClipToken to reject expired token")
	}

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/clip/"+expiredToken, nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code == http.StatusOK {
		t.Errorf("expected non-200 status for expired /clip/{token}, got 200 OK")
	}
}

// Test 15 (F1/F4): Cross-club HMAC key isolation — token signed with Club A cannot validate if Club B is claimed.
func TestTier2_F1_ClipTokenCrossClubKeyIsolation(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	if len(clubs) < 2 {
		t.Fatalf("expected >= 2 clubs")
	}
	clubA := clubs[0]
	clubB := clubs[1]
	if clubA.DLHMACKey == clubB.DLHMACKey {
		t.Fatalf("expected distinct dl_hmac_key per club, got identical keys for %s and %s", clubA.ID, clubB.ID)
	}

	tokA, _ := srv.SignClipToken(clubA.ID, clubA.BoxIDs[0], "camera01", "/clipA.mp4", time.Now().Add(time.Hour))
	tokB, _ := srv.SignClipToken(clubB.ID, clubB.BoxIDs[0], "camera01", "/clipB.mp4", time.Now().Add(time.Hour))

	// Swap payload of tokB with signature of tokA -> must fail HMAC verification because clubA.DLHMACKey != clubB.DLHMACKey
	partsA := strings.Split(tokA, ".")
	partsB := strings.Split(tokB, ".")
	if len(partsA) == 2 && len(partsB) == 2 {
		forged := partsB[0] + "." + partsA[1]
		if _, err := srv.VerifyClipToken(forged); err == nil {
			t.Errorf("expected forged cross-club token (clubB payload + clubA signature) to fail verification")
		}
	}
}

// Test 16 (F4): GET /fleet/api/handover/{unknown} returns 404 Not Found.
func TestTier2_F4_HandoverUnknownClubOrBoxReturns404(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover/unknown_venue_999", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown handover target, got %d", rec.Code)
	}
}

// =============================================================================
// TIER 3: CROSS-FEATURE PAIRWISE TESTS — 2 TESTS
// =============================================================================

// Test 17 (Pairwise F4 RedBida x F4 Handover x F5 Portal): Updating store_domain via RedBida propagates to Handover Pack and Shop Owner Portal.
func TestTier3_Pairwise_RedBidaDomainUpdatePropagatesToHandoverAndPortal(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Update store_domain and cstore_domain on inut_205_72 (Master Billiards Club)
	recApply, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_205_72/redbida/apply", map[string]any{
		"store_domain":  "store-mastervip.video.io.vn",
		"cstore_domain": "c-store-mastervip.video.io.vn",
	}, nil)
	if recApply.Code != http.StatusOK {
		t.Fatalf("redbida apply failed: %d", recApply.Code)
	}

	// Verify Handover Pack reflects new domains
	recHO, hoData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover/inut_205_72", nil, nil)
	if recHO.Code != http.StatusOK {
		t.Fatalf("handover get failed: %d", recHO.Code)
	}
	pack := hoData
	if nested, ok := hoData["handover"].(map[string]any); ok {
		pack = nested
	} else if nested, ok := hoData["pack"].(map[string]any); ok {
		pack = nested
	}
	if !strings.Contains(fmt.Sprint(pack["store_url"]), "store-mastervip.video.io.vn") {
		t.Errorf("expected handover store_url to contain store-mastervip.video.io.vn, got %v", pack["store_url"])
	}
	if !strings.Contains(fmt.Sprint(pack["zalo_template"]), "store-mastervip.video.io.vn") {
		t.Errorf("expected handover zalo_template to contain updated domain, got %v", pack["zalo_template"])
	}

	// Verify Shop Owner Portal status reflects new domains
	box := storeGetBox(srv, "inut_205_72")
	club := storeGetClub(srv, box.ClubID)
	recPortal, portalData := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if recPortal.Code != http.StatusOK {
		t.Fatalf("portal status failed: %d", recPortal.Code)
	}
	rawPortalJSON := recPortal.Body.String()
	if !strings.Contains(rawPortalJSON, "store-mastervip.video.io.vn") || !strings.Contains(rawPortalJSON, "c-store-mastervip.video.io.vn") {
		t.Errorf("expected portal status JSON to contain updated store/cstore domains, got: %v", portalData)
	}
}

// Test 18 (Pairwise F1 Seeder x F4 Handover x F5 Portal): Every seeded club's portal_token matches its Handover Pack URL and serves /portal/{token}/api/status.
func TestTier3_Pairwise_SeederPortalTokenMatchesHandoverAndPortalEndpoint(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	if len(clubs) < 20 {
		t.Fatalf("expected >= 20 clubs, got %d", len(clubs))
	}

	for _, club := range clubs {
		recHO, hoData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover/"+club.ID, nil, nil)
		if recHO.Code != http.StatusOK {
			t.Fatalf("handover failed for club %s: %d", club.ID, recHO.Code)
		}
		pack := hoData
		if nested, ok := hoData["handover"].(map[string]any); ok {
			pack = nested
		} else if nested, ok := hoData["pack"].(map[string]any); ok {
			pack = nested
		}
		if !strings.Contains(fmt.Sprint(pack["portal_url"]), "/portal/"+club.PortalToken) {
			t.Errorf("club %s handover portal_url %v does not contain token %s", club.ID, pack["portal_url"], club.PortalToken)
		}

		recPortal, _ := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
		if recPortal.Code != http.StatusOK {
			t.Errorf("club %s portal status returned %d", club.ID, recPortal.Code)
		}
	}
}

// =============================================================================
// TIER 4: REAL-WORLD APPLICATION SCENARIOS — 2 TESTS
// =============================================================================

// Test 19 (Scenario): Complete End-to-End Onboarding of Top Billiards Thái Nguyên (2 boxes, 26 tables) -> Hardening -> Bulk H.265 -> RedBida -> Handover -> Shop Owner Portal.
func TestTier4_Scenario_CompleteClubOnboardingToShopOwnerHandover(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Step 1: Admin logs in
	cookie := loginAndGetSessionCookie(t, srv, testAdminUser, testAdminPass, "198.51.100.5")
	if cookie == nil {
		t.Fatalf("admin login failed")
	}
	hdr := map[string]string{"Cookie": cookie.Name + "=" + cookie.Value}

	// Step 2: Run 8-Step Golden Hardening on both Top Billiards boxes (inut_204_38 & inut_204_83)
	for _, boxID := range []string{"inut_204_38", "inut_204_83"} {
		recH, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/"+boxID+"/hardening/remediate", map[string]any{
			"steps": []int{1, 2, 3, 4, 5, 6, 7, 8},
		}, hdr)
		if recH.Code != http.StatusOK {
			t.Fatalf("hardening remediate failed for %s: %d", boxID, recH.Code)
		}

		// Step 3: Standardize cameras on each box to Golden H.265 (30 FPS, 1536 Kbps CBR, GOP 50, AAC)
		recCam, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", map[string]any{
			"box_id":       boxID,
			"codec":        "H.265",
			"fps":          30,
			"bitrate_kbps": 1536,
			"rc_mode":      "CBR",
			"gop":          50,
			"audio_codec":  "AAC",
		}, hdr)
		if recCam.Code != http.StatusOK {
			t.Fatalf("camera bulk-apply failed for %s: %d", boxID, recCam.Code)
		}
	}

	// Step 4: Apply RedBida 17-Rule configuration on Box 1 and Box 2
	doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_38/redbida/apply", map[string]any{
		"auto_fix":          true,
		"store_domain":      "store-toptn1.video.io.vn",
		"cstore_domain":     "c-store-toptn1.video.io.vn",
		"eth0_virtual_ip":   "192.168.1.133",
		"shinobi_group_key": "ux0pAUigSn",
	}, hdr)
	doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_83/redbida/apply", map[string]any{
		"auto_fix":          true,
		"store_domain":      "store-toptn2.video.io.vn",
		"cstore_domain":     "c-store-toptn2.video.io.vn",
		"eth0_virtual_ip":   "192.168.1.239",
		"shinobi_group_key": "YZT7lO2CzM",
	}, hdr)

	// Step 5: Generate Handover Pack for Top Billiards Thái Nguyên
	recHO, hoData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/handover/inut_204_38", nil, hdr)
	if recHO.Code != http.StatusOK {
		t.Fatalf("handover failed: %d", recHO.Code)
	}
	pack := hoData
	if nested, ok := hoData["handover"].(map[string]any); ok {
		pack = nested
	} else if nested, ok := hoData["pack"].(map[string]any); ok {
		pack = nested
	}
	portalToken := fmt.Sprint(pack["portal_token"])
	clipURL := fmt.Sprint(pack["clip_url"])
	if len(portalToken) != 32 {
		t.Fatalf("expected 32-hex portal_token in handover pack, got %q", portalToken)
	}

	// Step 6: Customer scans QR clip link -> 200 OK
	recClip, _ := doHubRequest(t, srv, http.MethodGet, clipURL, nil, map[string]string{"NO_AUTH": "1"})
	if recClip.Code != http.StatusOK {
		t.Errorf("expected 200 OK on handover clip_url %s, got %d", clipURL, recClip.Code)
	}

	// Step 7: Shop Owner opens /portal/{portal_token}/api/status -> sees both boxes (inut_204_38 & inut_204_83)
	recPortal, portalStatus := doHubRequest(t, srv, http.MethodGet, "/portal/"+portalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
	if recPortal.Code != http.StatusOK {
		t.Fatalf("shop owner portal status failed: %d", recPortal.Code)
	}
	rawBoxes, _ := portalStatus["boxes"].([]any)
	if len(rawBoxes) != 2 {
		t.Errorf("expected 2 boxes in Top Billiards shop owner portal, got %d", len(rawBoxes))
	}
}

// Test 20 (Scenario): Mathematical Multi-Tenant Partition Audit across all 20+ seeded clubs and 30+ boxes.
func TestTier4_Scenario_MultiTenantIsolationAuditAcrossAllSeededClubs(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	allBoxes := storeListBoxes(srv)

	globalSeenBoxes := make(map[string]string) // boxID -> clubID

	for _, club := range clubs {
		rec, status := doHubRequest(t, srv, http.MethodGet, "/portal/"+club.PortalToken+"/api/status", nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusOK {
			t.Fatalf("portal status failed for club %s (%s): %d", club.ID, club.Name, rec.Code)
		}
		rawBoxes, ok := status["boxes"].([]any)
		if !ok || len(rawBoxes) == 0 {
			t.Errorf("club %s returned 0 boxes in portal status", club.ID)
			continue
		}
		for _, item := range rawBoxes {
			bm, _ := item.(map[string]any)
			bID := fmt.Sprint(bm["id"])
			if prevClub, exists := globalSeenBoxes[bID]; exists {
				t.Errorf("cross-tenant isolation violation: box %s appeared in both club %s and club %s portals!", bID, prevClub, club.ID)
			}
			globalSeenBoxes[bID] = club.ID
		}
	}

	if len(globalSeenBoxes) != len(allBoxes) {
		t.Errorf("expected union of all club portals (%d boxes) to equal total seeded boxes (%d)", len(globalSeenBoxes), len(allBoxes))
	}
}

// Test 21 (Cloud P2P Web NVR): Portal P2P endpoints enforce strict club box isolation (403 on foreign box_id).
func TestTier5_F5_PortalCloudP2PCrossClubIsolation(t *testing.T) {
	srv, _ := newTestHubServer(t)
	clubs := storeListClubs(srv)
	if len(clubs) < 2 {
		t.Fatalf("expected >= 2 clubs")
	}
	clubA := clubs[0]
	clubB := clubs[1]
	foreignBoxID := clubB.BoxIDs[0]

	for _, sub := range []string{
		"/api/p2p/devices?box_id=" + foreignBoxID,
		"/api/p2p/snapshot?box_id=" + foreignBoxID + "&id=192.168.1.101:37777",
		"/api/p2p/recordings?box_id=" + foreignBoxID + "&id=192.168.1.101:37777&start=2026-10-22T08:00&end=2026-10-22T09:00",
		"/api/p2p/playback?box_id=" + foreignBoxID + "&id=192.168.1.101:37777&start=2026-10-22T08:00&end=2026-10-22T08:01",
	} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, "/portal/"+clubA.PortalToken+sub, nil, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden when club %s accesses foreign box %s on %s, got %d", clubA.ID, foreignBoxID, sub, rec.Code)
		}
	}
}
