package bidahub

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const SessionCookieName = "inut_bida_session"

type authPrincipal struct {
	Username string
	Role     string
	Via      string // "cookie", "api_key", "bearer"
}

func clientIPFromRequest(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "127.0.0.1"
}

func wantsJSON(r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	acc := strings.ToLower(r.Header.Get("Accept"))
	return strings.Contains(ct, "application/json") || strings.Contains(acc, "application/json")
}

func (s *Server) authenticateRequest(r *http.Request) (*authPrincipal, bool) {
	now := time.Now().UTC()

	// 1. Check X-Hub-Key header
	if key := strings.TrimSpace(r.Header.Get("X-Hub-Key")); key != "" {
		if s.cfg.APIKey != "" && subtle.ConstantTimeCompare([]byte(key), []byte(s.cfg.APIKey)) == 1 {
			return &authPrincipal{Username: s.cfg.AdminUser, Role: "superadmin", Via: "api_key"}, true
		}
		if sess, ok := s.store.GetSession(key, now); ok {
			return &authPrincipal{Username: sess.Username, Role: sess.Role, Via: "api_key"}, true
		}
	}

	// 2. Check Authorization: Bearer <token> or Basic auth
	if authz := strings.TrimSpace(r.Header.Get("Authorization")); authz != "" {
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			tok := strings.TrimSpace(authz[7:])
			if s.cfg.APIKey != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(s.cfg.APIKey)) == 1 {
				return &authPrincipal{Username: s.cfg.AdminUser, Role: "superadmin", Via: "bearer"}, true
			}
			if sess, ok := s.store.GetSession(tok, now); ok {
				return &authPrincipal{Username: sess.Username, Role: sess.Role, Via: "bearer"}, true
			}
		} else if user, pass, ok := r.BasicAuth(); ok {
			if canonicalUser, role, valid := s.verifyCredentials(user, pass); valid {
				return &authPrincipal{Username: canonicalUser, Role: role, Via: "basic"}, true
			}
		}
	}

	// 3. Check session cookie inut_bida_session
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		if sess, ok := s.store.GetSession(c.Value, now); ok {
			return &authPrincipal{Username: sess.Username, Role: sess.Role, Via: "cookie"}, true
		}
	}

	return nil, false
}

func (s *Server) verifyCredentials(username, password string) (string, string, bool) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return "", "", false
	}
	if u, ok := s.store.AuthenticateUser(username, password); ok {
		role := u.Role
		if role == "" || role == "admin" {
			role = "superadmin"
		}
		return u.Username, role, true
	}
	return "", "", false
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	s.serveStaticFile(w, r, "static/login.html", "text/html; charset=utf-8")
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	now := time.Now().UTC()
	ip := clientIPFromRequest(r)
	isJSON := wantsJSON(r)
	if locked, until := s.store.CheckLockout(ip, now); locked {
		if isJSON {
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":        "locked",
				"message":      "IP đã bị tạm khóa 30 phút do đăng nhập sai nhiều lần.",
				"locked_until": until.Format(time.RFC3339),
			})
			return
		}
		http.Redirect(w, r, "/login?err=locked", http.StatusSeeOther)
		return
	}

	var username, password string
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var body struct {
			Username   string `json:"username"`
			Identifier string `json:"identifier"`
			Email      string `json:"email"`
			Password   string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		username = strings.TrimSpace(body.Username)
		if username == "" {
			username = strings.TrimSpace(body.Identifier)
		}
		if username == "" {
			username = strings.TrimSpace(body.Email)
		}
		password = body.Password
	} else {
		_ = r.ParseForm()
		username = strings.TrimSpace(r.FormValue("username"))
		if username == "" {
			username = strings.TrimSpace(r.FormValue("identifier"))
		}
		if username == "" {
			username = strings.TrimSpace(r.FormValue("email"))
		}
		password = r.FormValue("password")
	}

	canonicalUser, role, valid := s.verifyCredentials(username, password)
	if !valid {
		locked, count, until := s.store.RecordLoginFailure(ip, now)
		if locked {
			if isJSON {
				writeJSON(w, http.StatusTooManyRequests, map[string]any{
					"error":        "locked",
					"message":      "IP đã bị tạm khóa 30 phút do đăng nhập sai nhiều lần.",
					"failed_count": count,
					"locked_until": until.Format(time.RFC3339),
				})
				return
			}
			http.Redirect(w, r, "/login?err=locked", http.StatusSeeOther)
			return
		}
		if isJSON {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error":        "invalid_credentials",
				"message":      "Sai tài khoản hoặc mật khẩu.",
				"failed_count": count,
			})
			return
		}
		http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
		return
	}

	s.store.ClearLockout(ip)
	sess := s.store.CreateSession(canonicalUser, role, ip, 24*time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	if isJSON {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"authenticated": true,
			"username":      sess.Username,
			"role":          sess.Role,
			"token":         sess.Token,
		})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(SessionCookieName); err == nil && c.Value != "" {
		s.store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	if r.Method == http.MethodPost || wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	p, ok := s.authenticateRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"authenticated": false,
			"error":         "unauthorized",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      p.Username,
		"role":          p.Role,
		"via":           p.Via,
	})
}

// SignStaffRequest generates an HMAC-SHA256 signature for 1-click SSO into edge boxes.
func (s *Server) SignStaffRequest(boxID, username, role string, ts int64) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.StaffSecret))
	_, _ = fmt.Fprintf(mac, "%s:%s:%s:%d", boxID, username, role, ts)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) allowForgotPasswordRequest(ip string, now time.Time) bool {
	s.forgotMu.Lock()
	defer s.forgotMu.Unlock()
	if s.forgotHits == nil {
		s.forgotHits = make(map[string][]time.Time)
	}
	cutoff := now.Add(-10 * time.Minute)
	hits := s.forgotHits[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= 5 {
		s.forgotHits[ip] = kept
		return false
	}
	kept = append(kept, now)
	s.forgotHits[ip] = kept
	return true
}

// handleForgotPassword handles POST /api/auth/forgot-password and /fleet/api/auth/forgot-password.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	ip := clientIPFromRequest(r)
	if !s.allowForgotPasswordRequest(ip, time.Now().UTC()) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":   "rate_limited",
			"message": "Quá nhiều yêu cầu gửi mã OTP từ địa chỉ IP này. Vui lòng thử lại sau ít phút.",
		})
		return
	}
	var req struct {
		Identifier string `json:"identifier"`
		Email      string `json:"email"`
		Username   string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_json",
			"message": "Dữ liệu yêu cầu không hợp lệ.",
		})
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.Username)
	}
	if identifier == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "missing_identifier",
			"message": "Vui lòng nhập tên tài khoản hoặc email quản trị.",
		})
		return
	}

	user, ok := s.store.FindUser(identifier)
	if !ok || strings.TrimSpace(user.Email) == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":   "user_not_found",
			"message": "Không tìm thấy tài khoản quản trị hoặc email khôi phục tương ứng.",
		})
		return
	}

	const ttlMinutes = 15
	otpCode, expiresAt, err := s.store.CreatePasswordResetOTP(user.Username, user.Email, ttlMinutes*time.Minute)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "otp_generation_failed",
			"message": "Không thể khởi tạo mã OTP xác thực.",
		})
		return
	}

	if err := s.mailer.SendPasswordResetOTP(r.Context(), user.Email, user.Username, otpCode, ttlMinutes); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   "mail_send_failed",
			"message": "Không thể gửi email OTP qua ksp-pdfsign: " + err.Error(),
		})
		return
	}

	masked := MaskEmail(user.Email)
	log.Printf("PASSWORD_RESET_OTP_SENT username=%s email=%s ip=%s", user.Username, masked, clientIPFromRequest(r))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"status":       "ok",
		"message":      "Đã gửi mã xác thực OTP 6 chữ số tới " + masked,
		"username":     user.Username,
		"email_masked": masked,
		"ttl_minutes":  ttlMinutes,
		"expires_at":   expiresAt.Format(time.RFC3339),
	})
}

// handleResetPassword handles POST /api/auth/reset-password and /fleet/api/auth/reset-password.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	var req struct {
		Identifier  string `json:"identifier"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		OTPCode     string `json:"otp_code"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_json",
			"message": "Dữ liệu yêu cầu không hợp lệ.",
		})
		return
	}

	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}
	if identifier == "" {
		identifier = strings.TrimSpace(req.Username)
	}
	otpCode := strings.TrimSpace(req.OTPCode)
	if otpCode == "" {
		otpCode = strings.TrimSpace(req.OTP)
	}

	if identifier == "" || otpCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "missing_fields",
			"message": "Vui lòng nhập đầy đủ tài khoản/email và mã OTP 6 chữ số.",
		})
		return
	}

	if len(req.NewPassword) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "weak_password",
			"message": "Mật khẩu mới phải có ít nhất 6 ký tự.",
		})
		return
	}

	user, err := s.store.VerifyAndConsumePasswordResetOTP(identifier, otpCode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_otp",
			"message": err.Error(),
		})
		return
	}

	if err := s.store.UpdateUserPassword(user.Username, req.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "update_failed",
			"message": err.Error(),
		})
		return
	}

	revoked := s.store.DeleteSessionsForUser(user.Username)
	ip := clientIPFromRequest(r)
	s.store.ClearLockout(ip)
	s.store.ClearAllLockouts()

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	log.Printf("PASSWORD_RESET_SUCCESS username=%s ip=%s revoked_sessions=%d", user.Username, ip, revoked)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"status":           "ok",
		"message":          "Đặt lại mật khẩu thành công! Vui lòng đăng nhập bằng mật khẩu mới.",
		"username":         user.Username,
		"sessions_revoked": revoked,
	})
}

// handleChangePassword handles POST /api/auth/change-password and /fleet/api/auth/change-password.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	principal, ok := s.authenticateRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":   "unauthorized",
			"message": "Yêu cầu đăng nhập để đổi mật khẩu.",
		})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_json",
			"message": "Dữ liệu yêu cầu không hợp lệ.",
		})
		return
	}

	currentPassword := req.CurrentPassword
	if currentPassword == "" {
		currentPassword = req.OldPassword
	}
	if currentPassword == "" || req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "missing_fields",
			"message": "Vui lòng nhập mật khẩu hiện tại và mật khẩu mới.",
		})
		return
	}
	if len(req.NewPassword) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "weak_password",
			"message": "Mật khẩu mới phải có ít nhất 6 ký tự.",
		})
		return
	}

	user, valid := s.store.AuthenticateUser(principal.Username, currentPassword)
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":   "invalid_current_password",
			"message": "Mật khẩu hiện tại không chính xác.",
		})
		return
	}

	if err := s.store.UpdateUserPassword(user.Username, req.NewPassword); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "update_failed",
			"message": err.Error(),
		})
		return
	}

	revoked := s.store.DeleteSessionsForUser(user.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	log.Printf("PASSWORD_CHANGE_SUCCESS username=%s ip=%s revoked_sessions=%d", user.Username, clientIPFromRequest(r), revoked)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"status":           "ok",
		"message":          "Đổi mật khẩu thành công. Vui lòng đăng nhập lại bằng mật khẩu mới.",
		"username":         user.Username,
		"sessions_revoked": revoked,
	})
}
