package server

import (
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var tincVPNNet = mustParseCIDR("77.88.0.0/16")

func mustParseCIDR(cidr string) *net.IPNet {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return n
}

// isLoopbackHostHeader reports whether Host header names loopback (127.0.0.1, ::1, localhost).
// Local Yamux proxies (inut-bida-agent) rewrite req.Host to 127.0.0.1:<port>,
// whereas FRPC HTTP vhost proxies always forward the public subdomain Host.
func isLoopbackHostHeader(hostHdr string) bool {
	h := strings.TrimSpace(hostHdr)
	if h == "" {
		return false
	}
	if sh, _, err := net.SplitHostPort(h); err == nil {
		h = sh
	}
	h = strings.Trim(h, "[]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// isValidStaffProxyRequest validates that a Yamux SSO staff request came from
// the local inut-bida-agent (loopback socket + loopback Host + well-formed
// 64-hex HMAC signature + fresh timestamp + SSO metadata), preventing forged
// X-Inut-Staff-Sig headers over FRPC HTTP vhost tunnels.
func isValidStaffProxyRequest(r *http.Request) bool {
	ip := clientIP(r)
	if ip != "127.0.0.1" && ip != "::1" && !strings.EqualFold(ip, "localhost") {
		return false
	}
	if !isLoopbackHostHeader(r.Host) {
		return false
	}
	sig := strings.TrimSpace(r.Header.Get("X-Inut-Staff-Sig"))
	if len(sig) != 64 {
		return false
	}
	if _, err := hex.DecodeString(sig); err != nil {
		return false
	}
	if strings.TrimSpace(r.Header.Get("X-Inut-SSO-User")) == "" ||
		strings.TrimSpace(r.Header.Get("X-Inut-Box-ID")) == "" {
		return false
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-Inut-SSO-Ts")), 10, 64)
	if err != nil {
		return false
	}
	diff := time.Now().Unix() - ts
	if diff < -600 || diff > 600 {
		return false
	}
	return true
}

// loginRateLimitIP extracts the real client IP for brute-force throttling when
// requests arrive via a local reverse proxy (FRPC on 127.0.0.1), so one WAN
// attacker cannot lock out 127.0.0.1 for every admin.
func loginRateLimitIP(r *http.Request) string {
	ip := clientIP(r)
	if parsed := net.ParseIP(strings.Trim(ip, "[]")); parsed != nil && parsed.IsLoopback() {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if net.ParseIP(first) != nil {
				return first
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" && net.ParseIP(xri) != nil {
			return xri
		}
	}
	return ip
}

// isTrustedInternalIP reports whether raw IP belongs to loopback, private LAN
// (RFC1918 / ULA), link-local, or the iNut Tinc VPN mesh (77.88.0.0/16).
func isTrustedInternalIP(raw string) bool {
	host := strings.TrimSpace(raw)
	if host == "" {
		return false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || tincVPNNet.Contains(ip)
}

// hasUntrustedForwardedClient detects public WAN traffic forwarded through a
// local reverse proxy (e.g. FRPC on 127.0.0.1) by checking X-Forwarded-For / X-Real-IP.
func hasUntrustedForwardedClient(r *http.Request) bool {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" && !isTrustedInternalIP(first) {
			return true
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if !isTrustedInternalIP(xri) {
			return true
		}
	}
	return false
}

// hasValidAPIKeyHeader accepts configured bridge, Shinobi, or MCP API keys via
// X-Kspcam-Key, Authorization: Bearer, or X-Hub-Key.
func (s *Server) hasValidAPIKeyHeader(r *http.Request) bool {
	candidates := []string{
		extractBridgeKey(r),
		strings.TrimSpace(r.Header.Get("X-Hub-Key")),
	}
	secrets := []string{
		strings.TrimSpace(s.cfg.Bridge.APIKey),
		strings.TrimSpace(s.cfg.Shinobi.APIKey),
		strings.TrimSpace(s.cfg.MCP.APIKey),
	}
	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		for _, sec := range secrets {
			if constantMatch(cand, sec) {
				return true
			}
		}
	}
	return false
}

// authorizeSystemMetrics keeps /api/system/metrics accessible to local agents,
// Yamux pollers, LAN/Tinc VPN monitors, and authenticated users while blocking
// unauthenticated public WAN requests (including those proxied via FRPC).
func (s *Server) authorizeSystemMetrics(r *http.Request) bool {
	if s.authed(r) || s.hasValidAPIKeyHeader(r) {
		return true
	}
	if hasUntrustedForwardedClient(r) {
		return false
	}
	return isTrustedInternalIP(clientIP(r))
}

// authorizeRaysharpWebhook allows inbound camera/NVR alarm webhooks from local
// LAN/loopback/VPN IPs, configured inventory camera hosts, or authenticated
// callers, while blocking unauthenticated public WAN webhook spoofing.
func (s *Server) authorizeRaysharpWebhook(r *http.Request) bool {
	if s.authed(r) || s.hasValidAPIKeyHeader(r) {
		return true
	}
	if hasUntrustedForwardedClient(r) {
		return false
	}
	ip := clientIP(r)
	if isTrustedInternalIP(ip) {
		return true
	}
	if s.inv != nil {
		for _, d := range s.inv.List() {
			if strings.EqualFold(strings.TrimSpace(d.Host), ip) {
				return true
			}
		}
	}
	return false
}
