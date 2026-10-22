package bidahub

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleBoxProxy(w http.ResponseWriter, r *http.Request, principal *authPrincipal) {
	// Path format: /fleet/proxy/{box_id}/{service}[/...]
	trimmed := strings.TrimPrefix(r.URL.Path, "/fleet/proxy/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "invalid_proxy_path",
			"message": "Đường dẫn proxy cần dạng /fleet/proxy/{box_id}/{kspcam|shinobi|nodered}/",
		})
		return
	}

	boxID := parts[0]
	service := strings.ToLower(parts[1])
	restPath := "/"
	if len(parts) == 3 {
		restPath = "/" + parts[2]
	} else if !strings.HasSuffix(r.URL.Path, "/") {
		// Redirect /fleet/proxy/{box_id}/{service} -> /fleet/proxy/{box_id}/{service}/
		target := fmt.Sprintf("/fleet/proxy/%s/%s/", boxID, service)
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	if _, ok := s.store.GetBox(boxID); !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":   "box_not_found",
			"box_id":  boxID,
			"message": fmt.Sprintf("Không tìm thấy Box %s", boxID),
		})
		return
	}

	targetPort, okSvc := resolveProxyServicePort(service)
	if !okSvc {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "unknown_proxy_service",
			"service": service,
			"message": fmt.Sprintf("Dịch vụ proxy %q không hợp lệ (hỗ trợ: kspcam, shinobi, nodered).", service),
		})
		return
	}
	basePrefix := fmt.Sprintf("/fleet/proxy/%s/%s/", boxID, service)
	noSlashPrefix := fmt.Sprintf("/fleet/proxy/%s/%s", boxID, service)

	username := "admin"
	role := "superadmin"
	if principal != nil {
		if principal.Username != "" {
			username = principal.Username
		}
		if principal.Role != "" {
			role = principal.Role
		}
	}
	ts := time.Now().UTC().Unix()
	staffSig := s.SignStaffRequest(boxID, username, role, ts)

	targetURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(boxID, strconv.Itoa(targetPort)),
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return s.DialBox(ctx, boxID, targetPort)
		},
		DisableCompression:    true,
		ResponseHeaderTimeout: 120 * time.Second,
		IdleConnTimeout:       60 * time.Second,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = restPath
			req.URL.RawPath = ""
			req.Host = targetURL.Host

			req.Header.Del("Accept-Encoding")
			req.Header.Set("X-Inut-Staff-Sig", staffSig)
			req.Header.Set("X-Inut-SSO-User", username)
			req.Header.Set("X-Inut-SSO-Role", role)
			req.Header.Set("X-Inut-SSO-Ts", strconv.FormatInt(ts, 10))
			req.Header.Set("X-Forwarded-Prefix", noSlashPrefix)
			req.Header.Set("X-Inut-Box-ID", boxID)
			req.Header.Set("X-Target-Port", strconv.Itoa(targetPort))
			req.Header.Set("X-Target-Service", service)
		},
		Transport:     transport,
		FlushInterval: -1,
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("X-Proxied-By", "inut-bida-hub")
			resp.Header.Set("X-Proxied-Box", boxID)
			resp.Header.Set("X-Proxied-Service", service)

			if loc := resp.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "/fleet/proxy/") {
				resp.Header.Set("Location", noSlashPrefix+loc)
			}

			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			if strings.Contains(ct, "text/html") && resp.Body != nil {
				rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
				_ = resp.Body.Close()
				if err != nil {
					return err
				}
				baseTag := fmt.Sprintf(`<base href="%s">`, basePrefix)
				modified := injectBaseTag(rawBody, baseTag)
				resp.Body = io.NopCloser(bytes.NewReader(modified))
				resp.ContentLength = int64(len(modified))
				resp.Header.Set("Content-Length", strconv.Itoa(len(modified)))
			}
			return nil
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			if wantsJSON(req) {
				writeJSON(rw, http.StatusBadGateway, map[string]any{
					"error":   "bad_gateway",
					"box_id":  boxID,
					"service": service,
					"port":    targetPort,
					"detail":  err.Error(),
					"message": fmt.Sprintf("Không thể kết nối tới Box %s (%s:%d) qua Tunnel hoặc VPN.", boxID, service, targetPort),
				})
				return
			}
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(rw, `<!doctype html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="5">
<title>502 Bad Gateway — Box %s Offline</title>
<style>
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#071a24;color:#e2f1f0;font-family:system-ui,-apple-system,sans-serif;padding:24px}
.card{max-width:520px;background:#0d2633;border:1px solid rgba(239,68,68,.4);border-radius:14px;padding:28px;box-shadow:0 18px 40px rgba(0,0,0,.45)}
h1{margin:0 0 10px;font-size:20px;color:#f87171}
p{margin:8px 0;color:#9cc2c9;line-height:1.5;font-size:14px}
code{background:rgba(255,255,255,.08);padding:2px 6px;border-radius:4px;color:#38bdf8}
.btn{display:inline-block;margin-top:14px;padding:8px 16px;background:#0ea5e9;color:#fff;text-decoration:none;border-radius:8px;font-weight:600;font-size:13px}
</style>
</head>
<body>
<div class="card">
  <h1>⚠️ Không thể kết nối tới Box <code>%s</code> (%s:%d)</h1>
  <p>Đường hầm Yamux hoặc tuyến VPN tới trạm này hiện đang gián đoạn (<code>502 Bad Gateway</code>).</p>
  <p>Chi tiết kỹ thuật: <code>%s</code></p>
  <p>Trang sẽ tự động thử kết nối lại sau 5 giây...</p>
  <a class="btn" href="/#fleet">← Quay lại Bảng điều khiển Đội Box</a>
</div>
</body>
</html>`, boxID, boxID, service, targetPort, err.Error())
		},
	}

	proxy.ServeHTTP(w, r)
}

func injectBaseTag(htmlBytes []byte, baseTag string) []byte {
	if bytes.Contains(bytes.ToLower(htmlBytes), []byte("<base ")) {
		return htmlBytes
	}
	lower := bytes.ToLower(htmlBytes)
	if idx := bytes.Index(lower, []byte("<head>")); idx >= 0 {
		insertAt := idx + len("<head>")
		out := make([]byte, 0, len(htmlBytes)+len(baseTag))
		out = append(out, htmlBytes[:insertAt]...)
		out = append(out, []byte(baseTag)...)
		out = append(out, htmlBytes[insertAt:]...)
		return out
	}
	out := make([]byte, 0, len(htmlBytes)+len(baseTag))
	out = append(out, []byte(baseTag)...)
	out = append(out, htmlBytes...)
	return out
}

func resolveProxyServicePort(service string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "kspcam", "cam", "edge":
		return 2028, true
	case "shinobi", "nvr":
		return 8080, true
	case "nodered", "node-red", "redbida":
		return 1880, true
	default:
		if p, err := strconv.Atoi(service); err == nil && p > 0 && p <= 65535 {
			return p, true
		}
		return 0, false
	}
}
