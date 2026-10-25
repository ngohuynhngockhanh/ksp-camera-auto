package bidahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultMailAPIURL = "https://ksp-pdf-signer.p2p.inut.io.vn/api/mail/send"
	DefaultAdminEmail = "khanhnhn@inut.vn"
)

// Mailer dispatches transactional emails strictly via the ksp-pdfsign HTTP Mail API.
type Mailer struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewMailer creates a new HTTP Mailer targeting the ksp-pdfsign email endpoint.
func NewMailer(apiURL, apiKey string) *Mailer {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		apiURL = DefaultMailAPIURL
	}
	return &Mailer{
		apiURL: apiURL,
		apiKey: strings.TrimSpace(apiKey),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// APIURL returns the configured HTTP mail dispatch endpoint.
func (m *Mailer) APIURL() string {
	if m == nil {
		return DefaultMailAPIURL
	}
	return m.apiURL
}

// SendPasswordResetOTP sends a 6-digit password reset OTP email via the ksp-pdfsign HTTP Mail API.
func (m *Mailer) SendPasswordResetOTP(ctx context.Context, toEmail, username, otpCode string, ttlMinutes int) error {
	if m == nil {
		m = NewMailer(DefaultMailAPIURL, "")
	}
	toEmail = strings.TrimSpace(toEmail)
	if toEmail == "" {
		return fmt.Errorf("missing recipient email address")
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 15
	}

	subject := "[iNut Bida Hub] Mã xác thực đặt lại mật khẩu (OTP)"
	plainBody := fmt.Sprintf(
		"Xin chào %s,\n\nMã OTP xác thực đặt lại mật khẩu iNut Bida Hub của bạn là: %s (hiệu lực %d phút).\n\nTuyệt đối không chia sẻ mã này cho bất kỳ ai. Nếu bạn không yêu cầu thao tác này, vui lòng bỏ qua email này.",
		username, otpCode, ttlMinutes,
	)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #eef5f4; margin: 0; padding: 24px; color: #0f172a; }
    .card { max-width: 540px; margin: 0 auto; background: #ffffff; border-radius: 12px; padding: 32px; box-shadow: 0 4px 16px rgba(7,26,36,0.08); border: 1px solid #cbd5e1; }
    .logo { font-size: 19px; font-weight: 800; color: #0d9488; letter-spacing: -0.02em; margin-bottom: 18px; }
    .title { font-size: 17px; font-weight: 700; color: #071a24; margin-bottom: 12px; }
    .text { font-size: 14px; line-height: 1.6; color: #334155; margin-bottom: 22px; }
    .otp-box { background: #f0fdfa; border: 2px dashed #0d9488; border-radius: 10px; padding: 18px; text-align: center; margin-bottom: 22px; }
    .otp-code { font-size: 32px; font-weight: 800; letter-spacing: 6px; color: #0f766e; font-family: monospace; }
    .warning { font-size: 12px; color: #64748b; line-height: 1.5; border-top: 1px solid #e2e8f0; padding-top: 14px; }
  </style>
</head>
<body>
  <div class="card">
    <div class="logo">iNut Bida Central Fleet Hub</div>
    <div class="title">Yêu Cầu Đặt Lại Mật Khẩu Quản Trị</div>
    <div class="text">
      Xin chào <strong>%s</strong>,<br>
      Hệ thống nhận được yêu cầu đặt lại mật khẩu cho tài khoản quản trị liên kết với địa chỉ email này trên iNut Bida Hub.
    </div>
    <div class="otp-box">
      <div style="font-size: 11px; text-transform: uppercase; font-weight: 700; color: #0f766e; margin-bottom: 6px;">Mã OTP Xác Thực (Hiệu lực %d phút)</div>
      <div class="otp-code">%s</div>
    </div>
    <div class="text" style="font-size: 13px;">
      Vui lòng nhập mã OTP 6 chữ số trên vào màn hình đặt lại mật khẩu để hoàn tất. Hệ thống không bao giờ lưu mật khẩu dạng rõ.
    </div>
    <div class="warning">
      Nếu bạn không yêu cầu hành động này, vui lòng bỏ qua email.<br>
      Thời gian tạo yêu cầu: %s UTC
    </div>
  </div>
</body>
</html>`, username, ttlMinutes, otpCode, time.Now().UTC().Format("15:04:05 02/01/2006"))

	payload := map[string]any{
		"to":             toEmail,
		"subject":        subject,
		"body":           plainBody,
		"html":           htmlBody,
		"username":       username,
		"otp_code":       otpCode,
		"expiry_minutes": ttlMinutes,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal mail api payload: %w", err)
	}

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create mail api request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if m.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.apiKey)
		req.Header.Set("X-API-Key", m.apiKey)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("call mail api %s: %w", m.apiURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("mail api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// MaskEmail obscures the local part of an email address (e.g. "khanhnhn@inut.vn" -> "k***n@inut.vn").
func MaskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.IndexByte(email, '@')
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) == 1 {
		return local + "***@" + domain
	}
	return string(local[0]) + "***" + string(local[len(local)-1]) + "@" + domain
}
