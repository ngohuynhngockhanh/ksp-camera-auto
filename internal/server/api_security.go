package server

import (
	"net"
	"net/http"
	"strings"
)

var tincVPNNet = mustParseCIDR("77.88.0.0/16")

func mustParseCIDR(cidr string) *net.IPNet {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return n
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
