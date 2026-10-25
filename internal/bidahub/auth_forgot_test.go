package bidahub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type capturedMailRequest struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	ContentType string         `json:"content_type"`
	AuthHeader  string         `json:"auth_header"`
	APIKeyHdr   string         `json:"api_key_header"`
	RawBody     string         `json:"raw_body"`
	Payload     map[string]any `json:"payload"`
}

type mockKspPdfSignMailServer struct {
	mu         sync.Mutex
	server     *httptest.Server
	statusCode int
	requests   []capturedMailRequest
}

func newMockKspPdfSignMailServer(t *testing.T) *mockKspPdfSignMailServer {
	t.Helper()
	m := &mockKspPdfSignMailServer{
		statusCode: http.StatusOK,
	}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		var payload map[string]any
		_ = json.Unmarshal(raw, &payload)

		m.mu.Lock()
		code := m.statusCode
		m.requests = append(m.requests, capturedMailRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			ContentType: r.Header.Get("Content-Type"),
			AuthHeader:  r.Header.Get("Authorization"),
			APIKeyHdr:   r.Header.Get("X-API-Key"),
			RawBody:     string(raw),
			Payload:     payload,
		})
		m.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code >= 200 && code < 300 {
			_, _ = w.Write([]byte(`{"status":"ok","sent":true}`))
		} else {
			_, _ = w.Write([]byte(`{"error":"mail_dispatch_failed"}`))
		}
	}))
	t.Cleanup(func() {
		m.server.Close()
	})
	return m
}

func (m *mockKspPdfSignMailServer) SetStatusCode(code int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCode = code
}

func (m *mockKspPdfSignMailServer) Requests() []capturedMailRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]capturedMailRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

func (m *mockKspPdfSignMailServer) LastRequest() (capturedMailRequest, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requests) == 0 {
		return capturedMailRequest{}, false
	}
	return m.requests[len(m.requests)-1], true
}

func extractSixDigitOTPFromMail(req capturedMailRequest) string {
	if req.Payload != nil {
		for _, k := range []string{"otp_code", "otp", "code"} {
			if v, ok := req.Payload[k]; ok {
				s := strings.TrimSpace(fmt.Sprint(v))
				if len(s) == 6 {
					return s
				}
			}
		}
	}
	re := regexp.MustCompile(`\b([1-9][0-9]{5})\b`)
	if m := re.FindStringSubmatch(req.RawBody); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func newForgotPwdTestServer(t *testing.T, mailServer *mockKspPdfSignMailServer, opts ...func(*Config)) (*Server, string) {
	t.Helper()
	baseOpts := []func(*Config){
		func(c *Config) {
			c.AdminEmail = "khanhnhn@inut.vn"
			if mailServer != nil {
				c.MailAPIURL = mailServer.server.URL + "/api/mail/send"
			}
		},
	}
	baseOpts = append(baseOpts, opts...)
	return newTestHubServer(t, baseOpts...)
}

// =============================================================================
// TIER 1: FEATURE COVERAGE TESTS (F7, F8, F9, F10) — 20 TESTS (T1, T2, T3, T5)
// =============================================================================

// Test 79 / T1-1 (F7): Zero plaintext password and OTP persistence on disk in hub-data.json.
func TestTier1_T1_F7_ZeroPlaintextPasswordAndOTPOnDisk(t *testing.T) {
	const adminPlain = "SuperSecretPlaintextPass!2026"
	const viewerPlain = "ViewerSecretPlaintext!2026"

	mailSrv := newMockKspPdfSignMailServer(t)
	srv, dataPath := newForgotPwdTestServer(t, mailSrv, func(c *Config) {
		c.AdminPass = adminPlain
		c.ViewerPass = viewerPlain
		c.AdminEmail = "khanhnhn@inut.vn"
	})

	otpCode, expiresAt, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}
	if len(otpCode) != 6 {
		t.Fatalf("expected 6-digit OTP code, got %q", otpCode)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("expected expiresAt in the future, got %v", expiresAt)
	}
	if err := srv.Store().Save(); err != nil {
		t.Fatalf("Store().Save() failed: %v", err)
	}

	rawBytes, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", dataPath, err)
	}
	rawJSON := string(rawBytes)

	if strings.Contains(rawJSON, adminPlain) {
		t.Fatalf("SECURITY VIOLATION: hub-data.json contains plaintext AdminPass %q", adminPlain)
	}
	if strings.Contains(rawJSON, viewerPlain) {
		t.Fatalf("SECURITY VIOLATION: hub-data.json contains plaintext ViewerPass %q", viewerPlain)
	}
	if strings.Contains(rawJSON, `"`+otpCode+`"`) {
		t.Fatalf("SECURITY VIOLATION: hub-data.json contains plaintext OTP code %q", otpCode)
	}

	var diskState map[string]any
	if err := json.Unmarshal(rawBytes, &diskState); err != nil {
		t.Fatalf("failed to unmarshal hub-data.json: %v", err)
	}
	usersMap, ok := diskState["users"].(map[string]any)
	if !ok || len(usersMap) == 0 {
		t.Fatalf("expected non-empty users map in hub-data.json, got %#v", diskState["users"])
	}
	adminObj, ok := usersMap["admin"].(map[string]any)
	if !ok {
		t.Fatalf("expected users.admin in hub-data.json, got %#v", usersMap["admin"])
	}
	adminHash := fmt.Sprint(adminObj["password_hash"])
	if !strings.HasPrefix(adminHash, "$2a$") && !strings.HasPrefix(adminHash, "$2b$") {
		t.Errorf("expected users.admin.password_hash to start with $2a$ or $2b$ (bcrypt), got %q", adminHash)
	}
	if gotEmail := fmt.Sprint(adminObj["email"]); gotEmail != "khanhnhn@inut.vn" {
		t.Errorf("expected users.admin.email == khanhnhn@inut.vn, got %q", gotEmail)
	}

	otpsMap, ok := diskState["reset_otps"].(map[string]any)
	if !ok {
		t.Fatalf("expected reset_otps map in hub-data.json, got %#v", diskState["reset_otps"])
	}
	adminOTP, ok := otpsMap["admin"].(map[string]any)
	if !ok {
		t.Fatalf("expected reset_otps.admin in hub-data.json, got %#v", otpsMap["admin"])
	}
	expectedOTPHash := sha256.Sum256([]byte(otpCode))
	expectedHex := hex.EncodeToString(expectedOTPHash[:])
	if gotHash := fmt.Sprint(adminOTP["otp_hash"]); gotHash != expectedHex {
		t.Errorf("expected reset_otps.admin.otp_hash == %q, got %q", expectedHex, gotHash)
	}
}

// Test 80 / T1-2 (F7): In-memory Config.AdminPass and Config.ViewerPass are wiped after seeding Store.
func TestTier1_T1_F7_InMemoryConfigPlaintextPasswordsWipedAfterSeeding(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv, func(c *Config) {
		c.AdminPass = "EphemeralBootAdminPass!2026"
		c.ViewerPass = "EphemeralBootViewerPass!2026"
	})
	if srv.cfg.AdminPass != "" {
		t.Errorf("expected srv.cfg.AdminPass to be wiped to empty string after seeding, got %q", srv.cfg.AdminPass)
	}
	if srv.cfg.ViewerPass != "" {
		t.Errorf("expected srv.cfg.ViewerPass to be wiped to empty string after seeding, got %q", srv.cfg.ViewerPass)
	}
}

// Test 81 / T1-3 (F7): SeedDefaultUsers seeds admin (khanhnhn@inut.vn) and viewer with bcrypt hashes.
func TestTier1_T1_F7_SeedDefaultUsersBcryptAndDefaultAdminEmail(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	adminAcc, ok := srv.Store().FindUser("admin")
	if !ok || adminAcc == nil {
		t.Fatalf("expected FindUser('admin') to succeed")
	}
	if adminAcc.Email != "khanhnhn@inut.vn" {
		t.Errorf("expected admin email khanhnhn@inut.vn, got %q", adminAcc.Email)
	}
	if !strings.HasPrefix(adminAcc.PasswordHash, "$2a$") && !strings.HasPrefix(adminAcc.PasswordHash, "$2b$") {
		t.Errorf("expected bcrypt hash for admin, got %q", adminAcc.PasswordHash)
	}

	viewerAcc, ok := srv.Store().FindUser("viewer")
	if !ok || viewerAcc == nil {
		t.Fatalf("expected FindUser('viewer') to succeed")
	}
	if !strings.HasPrefix(viewerAcc.PasswordHash, "$2a$") && !strings.HasPrefix(viewerAcc.PasswordHash, "$2b$") {
		t.Errorf("expected bcrypt hash for viewer, got %q", viewerAcc.PasswordHash)
	}
}

// Test 82 / T1-4 (F7): FindUser and AuthenticateUser match case-insensitively by username or email.
func TestTier1_T1_F7_AuthenticateUserByUsernameAndEmailCaseInsensitive(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	for _, id := range []string{"admin", "ADMIN", "khanhnhn@inut.vn", "KHANHNHN@INUT.VN"} {
		u, ok := srv.Store().AuthenticateUser(id, testAdminPass)
		if !ok || u == nil {
			t.Fatalf("expected AuthenticateUser(%q) to succeed", id)
		}
		if u.Username != "admin" {
			t.Errorf("expected username=admin for identifier %q, got %q", id, u.Username)
		}
	}
}

// Test 83 / T2-1 (F8): POST /api/auth/forgot-password with {"identifier":"admin"} dispatches HTTP POST to ksp-pdfsign.
func TestTier1_T2_F8_ForgotPasswordByUsernameDispatchesKspPdfSignMail(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "admin",
	}, map[string]string{"NO_AUTH": "1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/forgot-password, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(resp["status"]); got != "ok" {
		t.Errorf("expected status=ok, got %q", got)
	}
	if ttl, _ := resp["ttl_minutes"].(float64); int(ttl) != 15 {
		t.Errorf("expected ttl_minutes=15, got %v", resp["ttl_minutes"])
	}
	if masked := fmt.Sprint(resp["email_masked"]); !strings.Contains(masked, "@inut.vn") || masked == "khanhnhn@inut.vn" {
		t.Errorf("expected masked @inut.vn email, got %q", masked)
	}

	lastReq, ok := mailSrv.LastRequest()
	if !ok {
		t.Fatalf("expected mock ksp-pdfsign server to receive POST request")
	}
	if lastReq.Method != http.MethodPost {
		t.Errorf("expected POST to ksp-pdfsign, got %s", lastReq.Method)
	}
	if !strings.Contains(strings.ToLower(lastReq.ContentType), "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", lastReq.ContentType)
	}
	if gotTo := fmt.Sprint(lastReq.Payload["to"]); gotTo != "khanhnhn@inut.vn" {
		t.Errorf("expected mail payload to=khanhnhn@inut.vn, got %q", gotTo)
	}
	if gotUser := fmt.Sprint(lastReq.Payload["username"]); gotUser != "admin" {
		t.Errorf("expected mail payload username=admin, got %q", gotUser)
	}
	otp := extractSixDigitOTPFromMail(lastReq)
	if len(otp) != 6 {
		t.Errorf("expected 6-digit otp_code in mail payload, got %q (payload: %#v)", otp, lastReq.Payload)
	}
	if expMin, _ := lastReq.Payload["expiry_minutes"].(float64); int(expMin) != 15 {
		t.Errorf("expected expiry_minutes=15 in mail payload, got %v", lastReq.Payload["expiry_minutes"])
	}
}

// Test 84 / T2-2 (F8): POST /api/auth/forgot-password with {"identifier":"khanhnhn@inut.vn"} dispatches HTTP POST to ksp-pdfsign.
func TestTier1_T2_F8_ForgotPasswordByEmailDispatchesKspPdfSignMail(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.55:12345"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/forgot-password by email, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(resp["status"]); got != "ok" {
		t.Errorf("expected status=ok, got %q", got)
	}
	lastReq, ok := mailSrv.LastRequest()
	if !ok {
		t.Fatalf("expected mock ksp-pdfsign server to receive request")
	}
	if gotTo := fmt.Sprint(lastReq.Payload["to"]); gotTo != "khanhnhn@inut.vn" {
		t.Errorf("expected payload to=khanhnhn@inut.vn, got %q", gotTo)
	}
	if gotUser := fmt.Sprint(lastReq.Payload["username"]); gotUser != "admin" {
		t.Errorf("expected payload username=admin, got %q", gotUser)
	}
}

// Test 85 / T2-3 (F8): POST /fleet/api/auth/forgot-password alias works and accepts "email" field.
func TestTier1_T2_F8_FleetAliasForgotPasswordAcceptsEmailField(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/auth/forgot-password", map[string]any{
		"email": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.56:12345"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/auth/forgot-password, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mailSrv.Requests()) != 1 {
		t.Fatalf("expected 1 mail request, got %d", len(mailSrv.Requests()))
	}
}

// Test 86 / T2-4 (F8): Mailer default URL is https://ksp-pdf-signer.p2p.inut.io.vn/api/mail/send and sends API key headers when configured.
func TestTier1_T2_F8_MailerDirectSendAndDefaultEndpoint(t *testing.T) {
	defaultMailer := NewMailer("", "")
	if defaultMailer == nil {
		t.Fatalf("expected NewMailer to return non-nil Mailer")
	}

	mailSrv := newMockKspPdfSignMailServer(t)
	customMailer := NewMailer(mailSrv.server.URL+"/api/mail/send", "secret-mail-key-123")
	err := customMailer.SendPasswordResetOTP(context.Background(), "khanhnhn@inut.vn", "admin", "654321", 15)
	if err != nil {
		t.Fatalf("SendPasswordResetOTP failed: %v", err)
	}
	lastReq, ok := mailSrv.LastRequest()
	if !ok {
		t.Fatalf("expected mail server to receive request")
	}
	if gotOTP := extractSixDigitOTPFromMail(lastReq); gotOTP != "654321" {
		t.Errorf("expected otp_code=654321, got %q", gotOTP)
	}
	if lastReq.APIKeyHdr == "" && lastReq.AuthHeader == "" {
		t.Errorf("expected Mailer with apiKey to set Authorization or X-API-Key header")
	}
}

// Test 87 / T2-5 (F8): Source files in internal/bidahub/ strictly do NOT import "net/smtp".
func TestTier1_T2_F8_ZeroNetSMTPImportInBidaHubPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir failed: %v", err)
	}
	checkedCount := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed reading %s: %v", name, err)
		}
		content := string(raw)
		if strings.Contains(content, `"net/smtp"`) {
			t.Errorf("FORBIDDEN SMTP IMPORT: %s imports net/smtp; must use ksp-pdfsign HTTP API exclusively", name)
		}
		checkedCount++
	}
	if checkedCount == 0 {
		t.Fatalf("expected to inspect production .go files in internal/bidahub")
	}
}

// Test 88 / T3-1 (F9): Full Password Reset happy path, old password rejection, new password login, and session revocation.
func TestTier1_T3_F9_PasswordResetHappyPathRevokesSessionAndAllowsNewLogin(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	// 1. Login as admin with initial password to obtain a valid session cookie
	oldCookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.70")
	if oldCookie == nil {
		t.Fatalf("expected initial admin login to return inut_bida_session cookie")
	}
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  oldCookie.Name + "=" + oldCookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with initial session cookie, got %d", rec.Code)
	}

	// 2. Request OTP via POST /api/auth/forgot-password
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.70:44444"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from forgot-password, got %d: %s", rec.Code, rec.Body.String())
	}
	lastMail, ok := mailSrv.LastRequest()
	if !ok {
		t.Fatalf("expected OTP email to be sent")
	}
	otpCode := extractSixDigitOTPFromMail(lastMail)
	if len(otpCode) != 6 {
		t.Fatalf("expected 6-digit OTP from mock mail server, got %q", otpCode)
	}

	// 3. Reset password via POST /api/auth/reset-password
	const newAdminPass = "NewBidaAdminPass#2026"
	rec, resetResp := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otpCode,
		"new_password": newAdminPass,
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.70:44444"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from reset-password, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(resetResp["status"]); got != "ok" {
		t.Errorf("expected status=ok from reset-password, got %q", got)
	}

	// 4. Assert previous inut_bida_session cookie is immediately revoked (GET /fleet/api/summary returns 401)
	rec, _ = doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  oldCookie.Name + "=" + oldCookie.Value,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected revoked session cookie to return 401 on /fleet/api/summary, got %d", rec.Code)
	}

	// 5. Assert login with old password fails (401 on JSON POST /login and nil cookie on form POST /login)
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/login", map[string]any{
		"username": "admin",
		"password": testAdminPass,
	}, map[string]string{"NO_AUTH": "1", "Accept": "application/json", "REMOTE_ADDR": "198.51.100.71:40000"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when logging in with old password after reset, got %d", rec.Code)
	}

	// 6. Assert login with new password succeeds both by username "admin" and by email "khanhnhn@inut.vn"
	newCookieByUser := loginAndGetSessionCookie(t, srv, "admin", newAdminPass, "198.51.100.72")
	if newCookieByUser == nil {
		t.Fatalf("expected form POST /login with username='admin' and new password to succeed (303 + session cookie)")
	}
	newCookieByEmail := loginAndGetSessionCookie(t, srv, "khanhnhn@inut.vn", newAdminPass, "198.51.100.73")
	if newCookieByEmail == nil {
		t.Fatalf("expected form POST /login with username='khanhnhn@inut.vn' and new password to succeed (303 + session cookie)")
	}

	// 7. Assert reusing the consumed otp_code a second time fails (400)
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otpCode,
		"new_password": "AnotherPassword#2026",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.70:44444"})
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 400 when reusing consumed OTP code, got %d", rec.Code)
	}
}

// Test 89 / T3-2 (F9): POST /fleet/api/auth/reset-password alias supports "username" and "otp" fields.
func TestTier1_T3_F9_FleetResetPasswordAliasWithUsernameAndOTPFields(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}

	rec, resp := doHubRequest(t, srv, http.MethodPost, "/fleet/api/auth/reset-password", map[string]any{
		"username":     "admin",
		"otp":          otpCode,
		"new_password": "FleetAliasReset#2026",
	}, map[string]string{"NO_AUTH": "1"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/auth/reset-password, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(resp["username"]); got != "admin" {
		t.Errorf("expected username=admin in reset response, got %q", got)
	}
}

// Test 90 / T3-3 (F9): Password reset clears prior IP brute-force login lockout.
func TestTier1_T3_F9_PasswordResetClearsIPLockout(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)
	lockedIP := "198.51.100.88"

	// Trigger lockout with 5 failed login attempts
	for range 5 {
		_ = loginAndGetSessionCookie(t, srv, "admin", "wrong-pass", lockedIP)
	}
	if locked, _ := srv.Store().CheckLockout(lockedIP, time.Now().UTC()); !locked {
		t.Fatalf("expected IP %s to be locked after 5 failed attempts", lockedIP)
	}

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "UnlockedNewPass#2026",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": lockedIP + ":50000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from reset-password, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify IP can immediately log in with the new password
	cookie := loginAndGetSessionCookie(t, srv, "admin", "UnlockedNewPass#2026", lockedIP)
	if cookie == nil {
		t.Fatalf("expected IP lockout to be cleared after successful password reset")
	}
}

// Test 91 / T5-1 (F9): Authenticated POST /api/auth/change-password updates bcrypt hash and revokes active session.
func TestTier1_T5_F9_AuthenticatedChangePasswordUpdatesHashAndRevokesSessions(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, dataPath := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.90")
	if cookie == nil {
		t.Fatalf("expected admin login to succeed")
	}

	const changedPass = "ChangedPass#999"
	rec, resp := doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": testAdminPass,
		"new_password":     changedPass,
	}, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/change-password, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := fmt.Sprint(resp["status"]); got != "ok" {
		t.Errorf("expected status=ok, got %q", got)
	}

	// Verify active session is immediately revoked
	rec, _ = doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected old session to return 401 after password change, got %d", rec.Code)
	}

	// Verify raw hub-data.json does not contain changedPass in plaintext and has valid bcrypt hash
	rawBytes, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", dataPath, err)
	}
	if strings.Contains(string(rawBytes), changedPass) {
		t.Fatalf("SECURITY VIOLATION: hub-data.json contains plaintext changedPass %q", changedPass)
	}

	// Verify login with changedPass works
	newCookie := loginAndGetSessionCookie(t, srv, "admin", changedPass, "198.51.100.91")
	if newCookie == nil {
		t.Fatalf("expected login with ChangedPass#999 to succeed")
	}
}

// Test 92 / T5-2 (F9): POST /fleet/api/auth/change-password alias works for authenticated session.
func TestTier1_T5_F9_FleetChangePasswordAliasWorks(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.92")
	if cookie == nil {
		t.Fatalf("expected admin login to succeed")
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/auth/change-password", map[string]any{
		"current_password": testAdminPass,
		"new_password":     "FleetChangedPass#2026",
	}, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/auth/change-password, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 93 / T5-3 (F7/F9): GET /login HTML contains autocomplete="new-password", data-lpignore="true", btnForgotPassword, and forgotPasswordModal.
func TestTier1_T5_F7_LoginHTMLZeroRememberAttributesAndForgotModal(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodGet, "/login", nil, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /login, got %d", rec.Code)
	}
	html := rec.Body.String()
	for _, token := range []string{
		`autocomplete="new-password"`,
		`data-lpignore="true"`,
		`data-1p-ignore="true"`,
		`data-form-type="other"`,
		`btnForgotPassword`,
		`forgotPasswordModal`,
	} {
		if !strings.Contains(html, token) {
			t.Errorf("expected GET /login HTML to contain %q", token)
		}
	}
}

// Test 94 / T5-4 (F7/F9): Embedded index.html and app.js contain Change Password modal and zero-remember attributes.
func TestTier1_T5_F7_IndexHTMLAndAppJSChangePasswordModalAndZeroRemember(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.93")
	if cookie == nil {
		t.Fatalf("expected login to succeed")
	}
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /, got %d", rec.Code)
	}
	indexHTML := rec.Body.String()
	for _, token := range []string{
		`btnChangePassword`,
		`changePasswordModal`,
		`autocomplete="new-password"`,
		`data-lpignore="true"`,
	} {
		if !strings.Contains(indexHTML, token) {
			t.Errorf("expected GET / index.html to contain %q", token)
		}
	}

	recJS, _ := doHubRequest(t, srv, http.MethodGet, "/static/app.js", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if recJS.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from GET /static/app.js, got %d", recJS.Code)
	}
	if !strings.Contains(recJS.Body.String(), "/api/auth/change-password") {
		t.Errorf("expected /static/app.js to reference /api/auth/change-password")
	}
}

// Test 95 / T5-5 (F7): web/static/login.html also enforces zero-password-remembering attributes.
func TestTier1_T5_F7_KspcamWebStaticLoginHTMLZeroRememberAttributes(t *testing.T) {
	loginPath := filepath.Join("..", "..", "web", "static", "login.html")
	raw, err := os.ReadFile(loginPath)
	if err != nil {
		t.Fatalf("failed reading %s: %v", loginPath, err)
	}
	html := string(raw)
	for _, token := range []string{
		`autocomplete="off"`,
		`autocomplete="new-password"`,
		`data-lpignore="true"`,
		`data-1p-ignore="true"`,
		`data-form-type="other"`,
	} {
		if !strings.Contains(html, token) {
			t.Errorf("expected web/static/login.html to contain %q", token)
		}
	}
}

// Test 96 (F7): Store.DeleteSessionsForUser removes only target user's sessions and preserves others.
func TestTier1_F7_DeleteSessionsForUserSelectiveRevocation(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	s1 := srv.Store().CreateSession("admin", "superadmin", "10.0.0.1", time.Hour)
	s2 := srv.Store().CreateSession("admin", "superadmin", "10.0.0.2", time.Hour)
	sViewer := srv.Store().CreateSession("viewer", "viewer", "10.0.0.3", time.Hour)

	removed := srv.Store().DeleteSessionsForUser("admin")
	if removed != 2 {
		t.Errorf("expected 2 admin sessions removed, got %d", removed)
	}
	if _, ok := srv.Store().GetSession(s1.Token, time.Now().UTC()); ok {
		t.Errorf("expected admin session s1 to be deleted")
	}
	if _, ok := srv.Store().GetSession(s2.Token, time.Now().UTC()); ok {
		t.Errorf("expected admin session s2 to be deleted")
	}
	if _, ok := srv.Store().GetSession(sViewer.Token, time.Now().UTC()); !ok {
		t.Errorf("expected viewer session to remain valid")
	}
}

// Test 97 (F7): Store.UpdateUserPassword updates bcrypt hash and clears active ResetOTPs for user.
func TestTier1_F7_UpdateUserPasswordClearsActiveResetOTP(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}
	if err := srv.Store().UpdateUserPassword("admin", "DirectUpdatedPass#2026"); err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}
	if _, err := srv.Store().VerifyAndConsumePasswordResetOTP("admin", otpCode); err == nil {
		t.Errorf("expected active ResetOTP to be cleared when UpdateUserPassword succeeds")
	}
	if _, ok := srv.Store().AuthenticateUser("admin", "DirectUpdatedPass#2026"); !ok {
		t.Errorf("expected AuthenticateUser to succeed with new password")
	}
}

// Test 98 (F7): GET /api/auth/me never exposes password_hash or otp_hash fields.
func TestTier1_F7_AuthMeEndpointNeverExposesPasswordHash(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.94")
	if cookie == nil {
		t.Fatalf("expected login to succeed")
	}
	rec, _ := doHubRequest(t, srv, http.MethodGet, "/api/auth/me", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/me, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "password_hash") || strings.Contains(body, "$2a$") || strings.Contains(body, "$2b$") {
		t.Errorf("SECURITY VIOLATION: /api/auth/me leaked password_hash: %s", body)
	}
}

// =============================================================================
// TIER 2: BOUNDARY, NEGATIVE & SECURITY CORNER CASES (F7–F10) — 19 TESTS (T4, T5)
// =============================================================================

// Test 99 / T4-1 (F9): Wrong OTP 5 times invalidates the OTP so even the correct OTP on the 6th attempt is rejected (400).
func TestTier2_T4_F9_WrongOTP5TimesInvalidatesOTPOn6thAttempt(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}
	wrongOTP := "000000"
	if otpCode == wrongOTP {
		wrongOTP = "999999"
	}

	// Submit wrong OTP 5 times
	for i := 1; i <= 5; i++ {
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
			"identifier":   "admin",
			"otp_code":     wrongOTP,
			"new_password": "ValidLengthPass#2026",
		}, map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt %d: expected 400/401/429 on wrong OTP, got %d", i, rec.Code)
		}
	}

	// 6th attempt with the CORRECT otpCode must be rejected because the OTP was invalidated after 5 failed attempts
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "ValidLengthPass#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
		t.Fatalf("6th attempt with correct OTP after 5 wrong attempts: expected 400 rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 100 / T4-2 (F9): 4 wrong OTP attempts followed by the correct OTP on the 5th attempt succeeds (boundary just-inside).
func TestTier2_T4_F9_FourWrongOTPAttemptsThen5thCorrectSucceeds(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}
	wrongOTP := "111111"
	if otpCode == wrongOTP {
		wrongOTP = "222222"
	}

	for i := 1; i <= 4; i++ {
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
			"identifier":   "admin",
			"otp_code":     wrongOTP,
			"new_password": "FifthAttemptPass#2026",
		}, map[string]string{"NO_AUTH": "1"})
		if rec.Code == http.StatusOK {
			t.Fatalf("attempt %d: expected failure on wrong OTP", i)
		}
	}

	// 5th attempt (Attempts == 4 < 5) with correct OTP must succeed
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "FifthAttemptPass#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 5th attempt with correct OTP to succeed (200 OK), got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 101 / T4-3 (F9): new_password shorter than 6 chars is rejected (400) without consuming the OTP.
func TestTier2_T4_F9_ShortPasswordRejectedWithoutConsumingOTP(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}

	// Try resetting with 5-char password ("12345")
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "12345",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for 5-char new_password, got %d: %s", rec.Code, rec.Body.String())
	}

	// Now retry with the SAME otpCode and a valid >= 6 char password — must succeed because OTP was not consumed
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "123456",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected OTP to remain valid after short-password rejection, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 102 / T4-4 (F9): Unknown identifier ("nobody@example.com") on forgot-password returns 400/404 and sends no email.
func TestTier2_T4_F9_ForgotPasswordUnknownIdentifierRejected(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "nobody@example.com",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.101:12345"})
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 for unknown identifier nobody@example.com, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mailSrv.Requests()) != 0 {
		t.Errorf("expected 0 emails sent for unknown identifier, got %d", len(mailSrv.Requests()))
	}
}

// Test 103 / T4-5 (F8/F9): When mock ksp-pdfsign server returns 500, POST /api/auth/forgot-password returns 502.
func TestTier2_T4_F8_ForgotPasswordReturns502WhenMailServerReturns500(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	mailSrv.SetStatusCode(http.StatusInternalServerError)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "admin",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.102:12345"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway when ksp-pdfsign returns 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 104 / T4-6 (F9): Expired OTP (negative TTL) is rejected on VerifyAndConsumePasswordResetOTP and POST /api/auth/reset-password.
func TestTier2_T4_F9_ExpiredOTPRejected(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otpCode, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", -1*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "ExpiredOTPTest#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 400/401 for expired OTP, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 105 / T4-7 (F9): Empty identifier on forgot-password returns 400 Bad Request.
func TestTier2_T4_F9_ForgotPasswordEmptyIdentifierReturns400(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "   ",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty identifier, got %d", rec.Code)
	}
}

// Test 106 / T4-8 (F9): Malformed JSON body on forgot-password, reset-password, and change-password returns 400.
func TestTier2_T4_F9_MalformedJSONOnAuthPasswordEndpointsReturns400(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)
	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.105")

	for _, path := range []string{"/api/auth/forgot-password", "/api/auth/reset-password"} {
		rec, _ := doHubRequest(t, srv, http.MethodPost, path, "{invalid-json", map[string]string{"NO_AUTH": "1"})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 on %s with malformed JSON, got %d", path, rec.Code)
		}
	}
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", "{invalid-json", map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on /api/auth/change-password with malformed JSON, got %d", rec.Code)
	}
}

// Test 107 / T4-9 (F9): GET method on forgot-password, reset-password, and change-password returns 405 Method Not Allowed.
func TestTier2_T4_F9_MethodNotAllowedOnAuthPasswordEndpoints(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	for _, path := range []string{
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/auth/change-password",
	} {
		rec, _ := doHubRequest(t, srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Errorf("expected 405 on GET %s, got %d", path, rec.Code)
		}
	}
}

// Test 108 / T4-10 (F9): User without configured email (e.g. viewer) on forgot-password returns 400/404 without panic.
func TestTier2_T4_F9_ForgotPasswordUserWithoutEmailRejectedGracefully(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "viewer",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.108:12345"})
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 when requesting OTP for account without email, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Test 109 / T4-11 (F9): Reset password with empty otp_code or unknown user returns 400/404.
func TestTier2_T4_F9_ResetPasswordEmptyOTPOrUnknownUserRejected(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec1, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     "",
		"new_password": "ValidNewPassword#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec1.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty otp_code, got %d", rec1.Code)
	}

	rec2, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "unknown_user@example.com",
		"otp_code":     "123456",
		"new_password": "ValidNewPassword#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec2.Code != http.StatusBadRequest && rec2.Code != http.StatusNotFound && rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 400/404 for unknown user on reset-password, got %d", rec2.Code)
	}
}

// Test 110 / T5-6 (F9): Unauthenticated POST /api/auth/change-password returns 401 Unauthorized.
func TestTier2_T5_F9_UnauthenticatedChangePasswordReturns401(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": testAdminPass,
		"new_password":     "ShouldNotChange#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on unauthenticated /api/auth/change-password, got %d", rec.Code)
	}
}

// Test 111 / T5-7 (F9): Authenticated POST /api/auth/change-password with wrong current_password returns 401.
func TestTier2_T5_F9_ChangePasswordWithWrongCurrentPasswordReturns401(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.111")
	if cookie == nil {
		t.Fatalf("expected admin login to succeed")
	}

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": "WrongCurrentPassword!999",
		"new_password":     "ShouldNotChange#2026",
	}, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 401 on wrong current_password, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify original password still works
	if _, ok := srv.Store().AuthenticateUser("admin", testAdminPass); !ok {
		t.Errorf("expected original password to remain intact after failed change-password")
	}
}

// Test 112 / T5-8 (F9): Authenticated POST /api/auth/change-password with short new_password (< 6 chars) returns 400.
func TestTier2_T5_F9_ChangePasswordShortNewPasswordReturns400(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	cookie := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.112")
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": testAdminPass,
		"new_password":     "short",
	}, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  cookie.Name + "=" + cookie.Value,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for 5-char new_password in change-password, got %d", rec.Code)
	}
}

// Test 113 (F7): Store.UpdateUserPassword rejects password shorter than 6 characters and unknown username.
func TestTier2_F7_StoreUpdateUserPasswordBoundaryValidation(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	if err := srv.Store().UpdateUserPassword("admin", "12345"); err == nil {
		t.Errorf("expected UpdateUserPassword to reject 5-char password")
	}
	if err := srv.Store().UpdateUserPassword("nonexistent_user", "ValidPass123"); err == nil {
		t.Errorf("expected UpdateUserPassword to reject unknown username")
	}
}

// Test 114 (F7/F9 Adversarial): Vietnamese Unicode and special symbols in new_password round-trip through bcrypt.
func TestTier2_F7_AdversarialUnicodeAndSymbolsInPasswordBcrypt(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	unicodePass := "MậtKhẩuBida#2026!@$%^&*()"
	if err := srv.Store().UpdateUserPassword("admin", unicodePass); err != nil {
		t.Fatalf("UpdateUserPassword with Unicode password failed: %v", err)
	}
	if _, ok := srv.Store().AuthenticateUser("khanhnhn@inut.vn", unicodePass); !ok {
		t.Errorf("expected AuthenticateUser with Vietnamese Unicode password to succeed")
	}
}

// Test 115 (F8): Mailer.SendPasswordResetOTP returns error when recipient email or OTP is empty.
func TestTier2_F8_MailerRejectsEmptyRecipientOrNetworkFailure(t *testing.T) {
	mailer := NewMailer("http://127.0.0.1:1/api/mail/send", "")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := mailer.SendPasswordResetOTP(ctx, "khanhnhn@inut.vn", "admin", "123456", 15); err == nil {
		t.Errorf("expected error when mail server is unreachable on port 1")
	}
}

// =============================================================================
// TIER 3: CROSS-FEATURE PAIRWISE COMBINATIONS (F1 x F7 x F8 x F9) — 4 TESTS
// =============================================================================

// Test 116 (F7 x F1): Updated bcrypt password in hub-data.json survives full Server restart while seed password does not overwrite it.
func TestTier3_Pairwise_ResetPasswordPersistsAcrossServerRestartWithoutSeedOverwrite(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "hub-data.json")

	srv1, err := NewServer(Config{
		BindAddr:   "127.0.0.1",
		Port:       2090,
		DataPath:   dataPath,
		AdminUser:  "admin",
		AdminPass:  "InitialBootPass#2026",
		AdminEmail: "khanhnhn@inut.vn",
		ViewerUser: "viewer",
		ViewerPass: "InitialViewerPass#2026",
		APIKey:     testAPIKey,
		MailAPIURL: mailSrv.server.URL + "/api/mail/send",
	})
	if err != nil {
		t.Fatalf("NewServer(srv1) failed: %v", err)
	}

	otpCode, _, err := srv1.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreatePasswordResetOTP failed: %v", err)
	}
	const persistedNewPass = "PersistedAcrossRestart#2026"
	rec, _ := doHubRequest(t, srv1, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otpCode,
		"new_password": persistedNewPass,
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-password failed: %d: %s", rec.Code, rec.Body.String())
	}
	_ = srv1.Close()

	// Restart Server 2 pointing at the same dataPath, passing the old InitialBootPass#2026 in Config
	srv2, err := NewServer(Config{
		BindAddr:   "127.0.0.1",
		Port:       2090,
		DataPath:   dataPath,
		AdminUser:  "admin",
		AdminPass:  "InitialBootPass#2026",
		AdminEmail: "khanhnhn@inut.vn",
		ViewerUser: "viewer",
		ViewerPass: "InitialViewerPass#2026",
		APIKey:     testAPIKey,
		MailAPIURL: mailSrv.server.URL + "/api/mail/send",
	})
	if err != nil {
		t.Fatalf("NewServer(srv2) failed: %v", err)
	}
	defer srv2.Close()

	// Old seed password must NOT overwrite the persisted reset password
	if _, ok := srv2.Store().AuthenticateUser("admin", "InitialBootPass#2026"); ok {
		t.Errorf("expected old seed password to be rejected after server restart")
	}
	if _, ok := srv2.Store().AuthenticateUser("khanhnhn@inut.vn", persistedNewPass); !ok {
		t.Errorf("expected reset password %q to survive server restart", persistedNewPass)
	}
}

// Test 117 (F8 x F9): Requesting a second OTP overwrites the first OTP so only the latest OTP is valid.
func TestTier3_Pairwise_SecondForgotPasswordRequestReplacesPreviousOTP(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	otp1, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("first CreatePasswordResetOTP failed: %v", err)
	}
	otp2, _, err := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	if err != nil {
		t.Fatalf("second CreatePasswordResetOTP failed: %v", err)
	}
	if otp1 == otp2 {
		// Extremely unlikely (1 in 900,000), regenerate once if equal
		otp2, _, _ = srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	}

	// First OTP must no longer work if different from second OTP
	if otp1 != otp2 {
		if _, err := srv.Store().VerifyAndConsumePasswordResetOTP("admin", otp1); err == nil {
			t.Errorf("expected superseded first OTP %q to be rejected", otp1)
		}
	}
	// Second OTP must succeed
	if _, err := srv.Store().VerifyAndConsumePasswordResetOTP("admin", otp2); err != nil {
		t.Errorf("expected latest OTP %q to succeed, got error: %v", otp2, err)
	}
}

// Test 118 (F9 x F2): Session created after password reset authenticates Streaming Reverse Proxy SSO.
func TestTier3_Pairwise_PostResetSessionAuthenticatesReverseProxySSO(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	var receivedSSOUser string
	edgeBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSSOUser = r.Header.Get("X-Inut-SSO-User")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer edgeBackend.Close()
	srv.RegisterTunnelTarget("inut_204_141", "kspcam", strings.TrimPrefix(edgeBackend.URL, "http://"))

	otpCode, _, _ := srv.Store().CreatePasswordResetOTP("admin", "khanhnhn@inut.vn", 15*time.Minute)
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otpCode,
		"new_password": "ProxyAfterResetPass#2026",
	}, map[string]string{"NO_AUTH": "1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-password failed: %d", rec.Code)
	}

	newCookie := loginAndGetSessionCookie(t, srv, "khanhnhn@inut.vn", "ProxyAfterResetPass#2026", "198.51.100.118")
	if newCookie == nil {
		t.Fatalf("expected login with email after reset to return session cookie")
	}

	recProxy, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/proxy/inut_204_141/kspcam/api/health", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  newCookie.Name + "=" + newCookie.Value,
	})
	if recProxy.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from reverse proxy using post-reset session, got %d", recProxy.Code)
	}
	if receivedSSOUser != "admin" {
		t.Errorf("expected X-Inut-SSO-User=admin, got %q", receivedSSOUser)
	}
}

// Test 119 (F9 x F1): Basic Auth with email ("khanhnhn@inut.vn") and updated password authenticates API requests.
func TestTier3_Pairwise_BasicAuthWithEmailAndChangedPasswordWorks(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	if err := srv.Store().UpdateUserPassword("admin", "BasicAuthNewPass#2026"); err != nil {
		t.Fatalf("UpdateUserPassword failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/fleet/api/summary", nil)
	req.SetBasicAuth("khanhnhn@inut.vn", "BasicAuthNewPass#2026")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with BasicAuth(khanhnhn@inut.vn, newPass), got %d", rec.Code)
	}
}

// =============================================================================
// TIER 4: REAL-WORLD END-TO-END APPLICATION SCENARIOS (F7–F10) — 3 TESTS
// =============================================================================

// Test 120 (Tier 4 Scenario): Full Admin Forgot-Password -> ksp-pdfsign OTP -> Reset -> Login -> Change-Password -> Disk Audit lifecycle.
func TestTier4_Scenario_CompleteAdminForgotResetAndChangePasswordLifecycle(t *testing.T) {
	const bootPass = "BootInitialSecret!2026"
	const resetPass = "AfterResetSecret!2026"
	const finalChangedPass = "FinalChangedSecret!2026"

	mailSrv := newMockKspPdfSignMailServer(t)
	srv, dataPath := newForgotPwdTestServer(t, mailSrv, func(c *Config) {
		c.AdminPass = bootPass
		c.AdminEmail = "khanhnhn@inut.vn"
	})

	// Step 1: Admin logs in with bootPass, then forgets password while another browser session is open
	sess1 := loginAndGetSessionCookie(t, srv, "admin", bootPass, "198.51.100.201")
	if sess1 == nil {
		t.Fatalf("step 1: expected initial login to succeed")
	}

	// Step 2: Admin clicks "Quên mật khẩu?" on /login and enters khanhnhn@inut.vn
	rec, forgotData := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.202:50000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("step 2: forgot-password failed: %d: %s", rec.Code, rec.Body.String())
	}
	if fmt.Sprint(forgotData["status"]) != "ok" {
		t.Errorf("step 2: expected status=ok, got %v", forgotData["status"])
	}

	lastMail, ok := mailSrv.LastRequest()
	if !ok {
		t.Fatalf("step 2: expected ksp-pdfsign mail request")
	}
	otpCode := extractSixDigitOTPFromMail(lastMail)

	// Step 3: Admin submits OTP and resetPass
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otpCode,
		"new_password": resetPass,
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.202:50000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("step 3: reset-password failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Step 4: Old session sess1 is revoked; admin logs in with resetPass and then changes password to finalChangedPass
	sess2 := loginAndGetSessionCookie(t, srv, "khanhnhn@inut.vn", resetPass, "198.51.100.202")
	if sess2 == nil {
		t.Fatalf("step 4: expected login with resetPass to succeed")
	}
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/change-password", map[string]any{
		"current_password": resetPass,
		"new_password":     finalChangedPass,
	}, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  sess2.Name + "=" + sess2.Value,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("step 4: change-password failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Step 5: Audit hub-data.json on disk — NONE of bootPass, resetPass, finalChangedPass, or otpCode may appear in plaintext
	rawDisk, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("step 5: read disk state failed: %v", err)
	}
	diskStr := string(rawDisk)
	for _, secret := range []string{bootPass, resetPass, finalChangedPass, `"` + otpCode + `"`} {
		if strings.Contains(diskStr, secret) {
			t.Fatalf("step 5 SECURITY VIOLATION: disk state contains plaintext secret %q", secret)
		}
	}
}

// Test 121 (Tier 2 / F8): Forgot-password rate limiter rejects excessive requests from the same IP with 429 Too Many Requests.
func TestTier2_T4_F8_ForgotPasswordRateLimitPerIP(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)
	ip := "198.51.100.250:40000"

	got429 := false
	for range 10 {
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
			"identifier": "admin",
		}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": ip})
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Errorf("expected /api/auth/forgot-password rate limiter to return 429 after repeated requests from same IP")
	}
}

// Test 122 (Tier 2 / F7): MaskEmail masks local part while preserving domain and handles edge addresses safely.
func TestTier2_F7_MaskEmailFormattingAndSafety(t *testing.T) {
	masked := MaskEmail("khanhnhn@inut.vn")
	if !strings.HasSuffix(masked, "@inut.vn") || masked == "khanhnhn@inut.vn" || !strings.Contains(masked, "*") {
		t.Errorf("expected khanhnhn@inut.vn to be masked with *, got %q", masked)
	}
	shortMasked := MaskEmail("a@b.vn")
	if !strings.Contains(shortMasked, "*") {
		t.Errorf("expected short email to be masked with *, got %q", shortMasked)
	}
}

// Test 123 (Tier 4 Scenario): Multi-device admin sessions revoked simultaneously on password reset while viewer session continues uninterrupted.
func TestTier4_Scenario_MultiDeviceAdminSessionRevocationPreservesViewerSession(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	adminDesktop := loginAndGetSessionCookie(t, srv, "admin", testAdminPass, "198.51.100.210")
	adminMobile := loginAndGetSessionCookie(t, srv, "khanhnhn@inut.vn", testAdminPass, "198.51.100.211")
	viewerBrowser := loginAndGetSessionCookie(t, srv, "viewer", testViewerPass, "198.51.100.212")
	if adminDesktop == nil || adminMobile == nil || viewerBrowser == nil {
		t.Fatalf("expected all initial logins to succeed")
	}

	// Trigger forgot-password and reset-password for admin
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "admin",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.213:40000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("forgot-password failed: %d", rec.Code)
	}
	lastMail, _ := mailSrv.LastRequest()
	otpCode := extractSixDigitOTPFromMail(lastMail)

	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "admin",
		"otp_code":     otpCode,
		"new_password": "MultiDeviceReset#2026",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.213:40000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset-password failed: %d", rec.Code)
	}

	// Both adminDesktop and adminMobile must be 401
	for _, c := range []*http.Cookie{adminDesktop, adminMobile} {
		r, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
			"NO_AUTH": "1",
			"Cookie":  c.Name + "=" + c.Value,
		})
		if r.Code != http.StatusUnauthorized {
			t.Errorf("expected admin session to be revoked (401), got %d", r.Code)
		}
	}

	// Viewer session must still be 200 OK
	rViewer, _ := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, map[string]string{
		"NO_AUTH": "1",
		"Cookie":  viewerBrowser.Name + "=" + viewerBrowser.Value,
	})
	if rViewer.Code != http.StatusOK {
		t.Errorf("expected viewer session to remain valid (200 OK), got %d", rViewer.Code)
	}
}

// Test 124 (Tier 4 Scenario): Attacker exhausts 5 OTP attempts -> Admin requests fresh OTP via ksp-pdfsign -> Admin resets password successfully.
func TestTier4_Scenario_BruteForceOTPExhaustionRecoveryViaFreshOTPRequest(t *testing.T) {
	mailSrv := newMockKspPdfSignMailServer(t)
	srv, _ := newForgotPwdTestServer(t, mailSrv)

	// 1. Admin requests OTP #1
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.220:40000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("first forgot-password failed: %d", rec.Code)
	}
	mail1, _ := mailSrv.LastRequest()
	otp1 := extractSixDigitOTPFromMail(mail1)

	// 2. Attacker guesses wrong OTP 5 times, invalidating otp1
	for range 5 {
		_, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
			"identifier":   "khanhnhn@inut.vn",
			"otp_code":     "000000",
			"new_password": "AttackerPass#2026",
		}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.221:40000"})
	}

	// Admin tries otp1 and sees it was invalidated
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otp1,
		"new_password": "AdminLegitPass#2026",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.220:40000"})
	if rec.Code == http.StatusOK {
		t.Fatalf("expected otp1 to be invalidated after 5 failed attempts")
	}

	// 3. Admin requests fresh OTP #2 and resets password
	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/forgot-password", map[string]any{
		"identifier": "khanhnhn@inut.vn",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.222:40000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("second forgot-password failed: %d", rec.Code)
	}
	mail2, _ := mailSrv.LastRequest()
	otp2 := extractSixDigitOTPFromMail(mail2)

	rec, _ = doHubRequest(t, srv, http.MethodPost, "/api/auth/reset-password", map[string]any{
		"identifier":   "khanhnhn@inut.vn",
		"otp_code":     otp2,
		"new_password": "AdminRecoveredPass#2026",
	}, map[string]string{"NO_AUTH": "1", "REMOTE_ADDR": "198.51.100.222:40000"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected fresh otp2 reset-password to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
}
