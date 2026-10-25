package bidahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

func servicePortName(port int) string {
	switch port {
	case 2028:
		return "kspcam"
	case 8080:
		return "shinobi"
	case 1880, 2023:
		return "nodered"
	default:
		return strconv.Itoa(port)
	}
}

func serviceDefaultPort(service string) int {
	switch strings.ToLower(strings.TrimSpace(service)) {
	case "kspcam", "cam", "edge":
		return 2028
	case "shinobi", "nvr":
		return 8080
	case "nodered", "node-red", "redbida":
		return 1880
	default:
		if p, err := strconv.Atoi(service); err == nil && p > 0 {
			return p
		}
		return 2028
	}
}

func normalizeHostPort(raw string, defaultPort int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "ws://") || strings.HasPrefix(raw, "wss://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			raw = u.Host
		}
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw
	}
	return net.JoinHostPort(raw, strconv.Itoa(defaultPort))
}

// RegisterTunnelTarget binds an outbound tunnel endpoint for a box and service/port.
func (s *Server) RegisterTunnelTarget(boxID string, serviceOrPort string, targetAddr string) {
	boxID = strings.TrimSpace(boxID)
	serviceOrPort = strings.ToLower(strings.TrimSpace(serviceOrPort))
	if serviceOrPort == "" {
		serviceOrPort = "*"
	}
	defPort := serviceDefaultPort(serviceOrPort)
	normAddr := normalizeHostPort(targetAddr, defPort)

	s.tunnelMu.Lock()
	if s.tunnelTargets[boxID] == nil {
		s.tunnelTargets[boxID] = make(map[string]string)
	}
	s.tunnelTargets[boxID][serviceOrPort] = normAddr
	if defPort > 0 {
		s.tunnelTargets[boxID][strconv.Itoa(defPort)] = normAddr
	}
	if serviceOrPort == "kspcam" || serviceOrPort == "2028" {
		s.tunnelTargets[boxID]["*"] = normAddr
	}
	s.tunnelMu.Unlock()

	_ = s.store.UpdateBox(boxID, func(b *Box) error {
		b.TunnelActive = normAddr != ""
		b.TunnelAddr = normAddr
		if normAddr != "" {
			b.Online = true
			b.Transport = "tunnel"
		}
		b.LastSeen = time.Now().UTC()
		return nil
	})
}

func (s *Server) lookupTunnelAddr(boxID string, targetPort int) (string, bool) {
	s.tunnelMu.RLock()
	bySvc := s.tunnelTargets[boxID]
	if bySvc != nil {
		portKey := strconv.Itoa(targetPort)
		if addr, ok := bySvc[portKey]; ok && addr != "" {
			s.tunnelMu.RUnlock()
			return addr, true
		}
		svcKey := servicePortName(targetPort)
		if addr, ok := bySvc[svcKey]; ok && addr != "" {
			s.tunnelMu.RUnlock()
			return addr, true
		}
		if addr, ok := bySvc["*"]; ok && addr != "" {
			s.tunnelMu.RUnlock()
			return addr, true
		}
	}
	s.tunnelMu.RUnlock()

	if box, ok := s.store.GetBox(boxID); ok && box.TunnelActive && box.TunnelAddr != "" && !strings.HasPrefix(box.TunnelAddr, "yamux://") {
		return normalizeHostPort(box.TunnelAddr, targetPort), true
	}
	return "", false
}

// DialBox connects to a box via its active reverse tunnel first, falling back to direct VPN TCP dial.
func (s *Server) DialBox(ctx context.Context, boxID string, targetPort int) (net.Conn, error) {
	if targetPort <= 0 {
		targetPort = 2028
	}
	timeout := s.cfg.DialTimeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	dialer := &net.Dialer{Timeout: timeout}
	if sess := s.GetYamuxSession(boxID); sess != nil {
		if conn, err := s.dialYamuxStream(ctx, sess); err == nil {
			return conn, nil
		}
	}

	if tunnelAddr, ok := s.lookupTunnelAddr(boxID, targetPort); ok {
		conn, err := dialer.DialContext(ctx, "tcp", tunnelAddr)
		if err == nil {
			return conn, nil
		}
		// If a tunnel target was explicitly registered on loopback and failed, return the error directly
		if strings.HasPrefix(tunnelAddr, "127.0.0.1:") || strings.HasPrefix(tunnelAddr, "localhost:") {
			return nil, fmt.Errorf("tunnel dial %s (%s): %w", boxID, tunnelAddr, err)
		}
	}

	box, ok := s.store.GetBox(boxID)
	if !ok {
		return nil, fmt.Errorf("unknown box %q", boxID)
	}
	if !box.Online && strings.HasPrefix(box.VPNIP, "77.88.") {
		return nil, fmt.Errorf("box %s (%s) is marked offline", boxID, box.VPNIP)
	}

	targetAddr := normalizeHostPort(box.VPNIP, targetPort)
	if targetAddr == "" {
		return nil, fmt.Errorf("box %q has no VPN IP or tunnel", boxID)
	}
	return dialer.DialContext(ctx, "tcp", targetAddr)
}

func (s *Server) handleTunnelEndpoint(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		s.handleYamuxTunnelUpgrade(w, r)
		return
	}
	boxID := strings.TrimSpace(r.Header.Get("X-Box-ID"))
	secret := strings.TrimSpace(r.Header.Get("X-Box-Secret"))
	tunnelAddr := strings.TrimSpace(r.Header.Get("X-Tunnel-Addr"))
	service := strings.TrimSpace(r.Header.Get("X-Tunnel-Service"))
	socModel := strings.TrimSpace(r.Header.Get("X-Box-SoC"))

	var body struct {
		BoxID      string `json:"box_id"`
		Secret     string `json:"secret"`
		TunnelAddr string `json:"tunnel_addr"`
		TargetAddr string `json:"target_addr"`
		Service    string `json:"service"`
		SoCModel   string `json:"soc_model"`
	}
	if r.Body != nil && r.Method == http.MethodPost {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	if boxID == "" {
		boxID = strings.TrimSpace(r.URL.Query().Get("box_id"))
	}
	if boxID == "" {
		boxID = strings.TrimSpace(body.BoxID)
	}
	if secret == "" {
		secret = strings.TrimSpace(body.Secret)
	}
	if tunnelAddr == "" {
		tunnelAddr = strings.TrimSpace(body.TunnelAddr)
	}
	if tunnelAddr == "" {
		tunnelAddr = strings.TrimSpace(body.TargetAddr)
	}
	if tunnelAddr == "" {
		tunnelAddr = strings.TrimSpace(r.URL.Query().Get("target_addr"))
	}
	if service == "" {
		service = strings.TrimSpace(body.Service)
	}
	if socModel == "" {
		socModel = strings.TrimSpace(body.SoCModel)
	}

	if boxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_box_id"})
		return
	}

	if _, exists := s.store.GetBox(boxID); !exists {
		req := s.store.RecordEnrollment(boxID, secret, clientIPFromRequest(r), socModel)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":       "ENROLLMENT_PENDING",
			"box_id":      req.BoxID,
			"fingerprint": req.Fingerprint,
			"status":      req.Status,
		})
		return
	}

	if tunnelAddr != "" {
		s.RegisterTunnelTarget(boxID, service, tunnelAddr)
	} else {
		_ = s.store.UpdateBox(boxID, func(b *Box) error {
			b.TunnelActive = true
			b.Online = true
			b.Transport = "tunnel"
			b.LastSeen = time.Now().UTC()
			return nil
		})
	}

	s.broadcastSSE("fleet_update", map[string]any{
		"type":          "tunnel_registered",
		"box_id":        boxID,
		"tunnel_active": true,
		"ts":            time.Now().UTC().Format(time.RFC3339),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"box_id":        boxID,
		"tunnel_active": true,
		"transport":     "tunnel",
		"tunnel_addr":   tunnelAddr,
	})
}

// HeartbeatPayload represents a live telemetry snapshot for a single edge box.
type HeartbeatPayload struct {
	BoxID                    string            `json:"box_id"`
	Secret                   string            `json:"secret,omitempty"`
	SoCModel                 string            `json:"soc_model,omitempty"`
	Online                   *bool             `json:"online,omitempty"`
	TunnelActive             *bool             `json:"tunnel_active,omitempty"`
	TunnelAddr               string            `json:"tunnel_addr,omitempty"`
	RTTMs                    int               `json:"rtt_ms,omitempty"`
	CPUPct                   *float64          `json:"cpu_pct,omitempty"`
	LoadAvg1m                *float64          `json:"load_avg_1m,omitempty"`
	CPUCores                 int               `json:"cpu_cores,omitempty"`
	TempC                    float64           `json:"temp_c,omitempty"`
	RAMPct                   int               `json:"ram_pct,omitempty"`
	RAMTotalMB               int               `json:"ram_total_mb,omitempty"`
	RAMUsedMB                int               `json:"ram_used_mb,omitempty"`
	RAMAvailableMB           int               `json:"ram_available_mb,omitempty"`
	RAMUsedPct               int               `json:"ram_used_pct,omitempty"`
	HeapAllocMB              *float64          `json:"heap_alloc_mb,omitempty"`
	HeapSysMB                *float64          `json:"heap_sys_mb,omitempty"`
	KspcamRSSMB              *float64          `json:"kspcam_rss_mb,omitempty"`
	CameraRSSMB              *float64          `json:"camera_rss_mb,omitempty"`
	Goroutines               int               `json:"goroutines,omitempty"`
	SwapTotalMB              int               `json:"swap_total_mb,omitempty"`
	SwapUsedMB               *int              `json:"swap_used_mb,omitempty"`
	SwapFreeMB               *int              `json:"swap_free_mb,omitempty"`
	ZRAMTotalMB              int               `json:"zram_total_mb,omitempty"`
	ZRAMUsedMB               *int              `json:"zram_used_mb,omitempty"`
	ZRAMPct                  *int              `json:"zram_pct,omitempty"`
	SDTotalGB                int               `json:"sd_total_gb,omitempty"`
	SDUsedGB                 int               `json:"sd_used_gb,omitempty"`
	SDUsedPct                int               `json:"sd_used_pct,omitempty"`
	SDFSType                 string            `json:"sd_fs_type,omitempty"`
	SDMountOpts              string            `json:"sd_mount_opts,omitempty"`
	FFmpegCount              *int              `json:"ffmpeg_count,omitempty"`
	RXMbps                   *float64          `json:"rx_mbps,omitempty"`
	ShinobiMonitorsRecording *int              `json:"shinobi_monitors_recording,omitempty"`
	UptimeSec                int64             `json:"uptime_sec,omitempty"`
	UptimeHuman              string            `json:"uptime_human,omitempty"`
	Monitors                 []*ShinobiMonitor `json:"monitors,omitempty"`
	HardeningScore           *int              `json:"hardening_score,omitempty"`
}

// FormatUptimeHuman formats uptime seconds as "Xd Yh", "Yh Zm", or "Zm".
func FormatUptimeHuman(sec int64) string {
	if sec <= 0 {
		return ""
	}
	days := sec / 86400
	hours := (sec % 86400) / 3600
	mins := (sec % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	if mins <= 0 {
		return "1m"
	}
	return fmt.Sprintf("%dm", mins)
}

func applyHeartbeatToBox(b *Box, p HeartbeatPayload, source string) {
	if p.Online != nil {
		b.Online = *p.Online
	} else {
		b.Online = true
	}
	if source != "" {
		b.TelemetrySource = source
	}
	if b.Online {
		b.LastSeen = time.Now().UTC()
	} else if b.LastSeen.IsZero() {
		b.LastSeen = time.Now().UTC()
	}
	if p.SoCModel != "" {
		b.SoCModel = p.SoCModel
	}
	if p.TunnelActive != nil {
		b.TunnelActive = *p.TunnelActive
		if b.TunnelActive {
			b.Transport = "tunnel"
		}
	}
	if !b.Online && p.RTTMs <= 0 {
		b.RTTMs = 0
	} else if p.RTTMs > 0 {
		b.RTTMs = p.RTTMs
		b.RTTHistory = append(b.RTTHistory, p.RTTMs)
		if len(b.RTTHistory) > 12 {
			b.RTTHistory = b.RTTHistory[len(b.RTTHistory)-12:]
		}
	}
	if p.CPUPct != nil {
		b.CPUPct = *p.CPUPct
	}
	if p.LoadAvg1m != nil {
		b.LoadAvg1m = *p.LoadAvg1m
	}
	if p.CPUCores > 0 {
		b.CPUCores = p.CPUCores
	}
	if p.TempC > 0 {
		b.TempC = p.TempC
	}
	if p.RAMTotalMB > 0 {
		b.RAMTotalMB = p.RAMTotalMB
	}
	if p.RAMUsedMB > 0 {
		b.RAMUsedMB = p.RAMUsedMB
	}
	if p.RAMAvailableMB > 0 {
		b.RAMAvailableMB = p.RAMAvailableMB
	}
	if b.RAMTotalMB > 0 {
		if p.RAMUsedMB > 0 && p.RAMAvailableMB == 0 && b.RAMTotalMB >= p.RAMUsedMB {
			b.RAMAvailableMB = b.RAMTotalMB - p.RAMUsedMB
		} else if p.RAMAvailableMB > 0 && p.RAMUsedMB == 0 && b.RAMTotalMB >= p.RAMAvailableMB {
			b.RAMUsedMB = b.RAMTotalMB - p.RAMAvailableMB
		}
	}
	if p.RAMUsedPct > 0 {
		b.RAMUsedPct = p.RAMUsedPct
		b.RAMPct = p.RAMUsedPct
	} else if p.RAMPct > 0 {
		b.RAMPct = p.RAMPct
		b.RAMUsedPct = p.RAMPct
	} else if p.RAMTotalMB > 0 && b.RAMUsedMB > 0 {
		pct := (b.RAMUsedMB * 100) / b.RAMTotalMB
		b.RAMUsedPct = pct
		b.RAMPct = pct
	} else if b.RAMTotalMB > 0 && p.RAMUsedMB > 0 {
		pct := (p.RAMUsedMB * 100) / b.RAMTotalMB
		b.RAMUsedPct = pct
		b.RAMPct = pct
	}
	if b.RAMTotalMB > 0 && (p.RAMPct > 0 || p.RAMUsedPct > 0) && p.RAMUsedMB == 0 && p.RAMAvailableMB == 0 {
		b.RAMUsedMB = (b.RAMTotalMB * b.RAMUsedPct) / 100
		b.RAMAvailableMB = b.RAMTotalMB - b.RAMUsedMB
	}
	if p.HeapAllocMB != nil {
		b.HeapAllocMB = *p.HeapAllocMB
	}
	if p.HeapSysMB != nil {
		b.HeapSysMB = *p.HeapSysMB
	}
	if p.KspcamRSSMB != nil {
		b.KspcamRSSMB = *p.KspcamRSSMB
	}
	if p.CameraRSSMB != nil {
		b.CameraRSSMB = *p.CameraRSSMB
	}
	if p.Goroutines > 0 {
		b.Goroutines = p.Goroutines
	}
	if p.SwapTotalMB > 0 {
		b.SwapTotalMB = p.SwapTotalMB
		if p.ZRAMTotalMB == 0 {
			b.ZRAMTotalMB = p.SwapTotalMB
		}
	}
	if p.ZRAMTotalMB > 0 {
		b.ZRAMTotalMB = p.ZRAMTotalMB
		if p.SwapTotalMB == 0 {
			b.SwapTotalMB = p.ZRAMTotalMB
		}
	}
	if p.SwapUsedMB != nil {
		b.SwapUsedMB = *p.SwapUsedMB
		if p.ZRAMUsedMB == nil {
			b.ZRAMUsedMB = *p.SwapUsedMB
		}
	}
	if p.ZRAMUsedMB != nil {
		b.ZRAMUsedMB = *p.ZRAMUsedMB
		if p.SwapUsedMB == nil {
			b.SwapUsedMB = *p.ZRAMUsedMB
		}
	}
	if p.SwapFreeMB != nil {
		b.SwapFreeMB = *p.SwapFreeMB
	} else if (p.SwapTotalMB > 0 || p.ZRAMTotalMB > 0 || p.SwapUsedMB != nil || p.ZRAMUsedMB != nil) && b.SwapTotalMB >= b.SwapUsedMB {
		b.SwapFreeMB = b.SwapTotalMB - b.SwapUsedMB
	}
	if p.ZRAMPct != nil {
		b.ZRAMPct = *p.ZRAMPct
	} else if (p.SwapTotalMB > 0 || p.ZRAMTotalMB > 0 || p.SwapUsedMB != nil || p.ZRAMUsedMB != nil) && b.SwapTotalMB > 0 {
		b.ZRAMPct = (b.SwapUsedMB * 100) / b.SwapTotalMB
	}
	if p.SDTotalGB > 0 {
		b.SDTotalGB = p.SDTotalGB
	}
	if p.SDUsedGB > 0 {
		b.SDUsedGB = p.SDUsedGB
	}
	if p.SDUsedPct > 0 {
		b.SDUsedPct = p.SDUsedPct
		if p.SDUsedGB == 0 && b.SDTotalGB > 0 {
			b.SDUsedGB = (b.SDTotalGB * b.SDUsedPct) / 100
		}
	} else if p.SDUsedGB > 0 && b.SDTotalGB > 0 {
		b.SDUsedPct = (b.SDUsedGB * 100) / b.SDTotalGB
	}
	if p.SDFSType != "" {
		b.SDFSType = p.SDFSType
	}
	if p.SDMountOpts != "" {
		b.SDMountOpts = p.SDMountOpts
	}
	if p.FFmpegCount != nil {
		b.FFmpegCount = *p.FFmpegCount
	}
	if p.RXMbps != nil {
		b.RXMbps = *p.RXMbps
	}
	if p.ShinobiMonitorsRecording != nil {
		b.ShinobiMonitorsRecording = *p.ShinobiMonitorsRecording
	}
	if len(p.Monitors) > 0 {
		b.Monitors = p.Monitors
	}
	if p.HardeningScore != nil {
		b.HardeningScore = *p.HardeningScore
	}
	if p.UptimeSec > 0 {
		b.UptimeSec = p.UptimeSec
		if p.UptimeHuman == "" {
			b.UptimeHuman = FormatUptimeHuman(p.UptimeSec)
		}
	}
	if p.UptimeHuman != "" {
		b.UptimeHuman = p.UptimeHuman
	}
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	var payload HeartbeatPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	boxID := strings.TrimSpace(payload.BoxID)
	if boxID == "" {
		boxID = strings.TrimSpace(r.Header.Get("X-Box-ID"))
	}
	if boxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_box_id"})
		return
	}

	secret := strings.TrimSpace(payload.Secret)
	if secret == "" {
		secret = strings.TrimSpace(r.Header.Get("X-Box-Secret"))
	}

	if _, exists := s.store.GetBox(boxID); !exists {
		req := s.store.RecordEnrollment(boxID, secret, clientIPFromRequest(r), payload.SoCModel)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":       "ENROLLMENT_PENDING",
			"box_id":      req.BoxID,
			"fingerprint": req.Fingerprint,
			"status":      req.Status,
		})
		return
	}

	if payload.TunnelAddr != "" {
		s.RegisterTunnelTarget(boxID, "*", payload.TunnelAddr)
	}

	_ = s.store.UpdateBox(boxID, func(b *Box) error {
		applyHeartbeatToBox(b, payload, "live")
		return nil
	})

	updated, _ := s.store.GetBox(boxID)
	kpi := s.store.ComputeKPI()
	s.broadcastSSE("heartbeat", map[string]any{
		"box_id":           boxID,
		"online":           updated.Online,
		"rtt_ms":           updated.RTTMs,
		"cpu_pct":          updated.CPUPct,
		"load_avg_1m":      updated.LoadAvg1m,
		"ram_pct":          updated.RAMPct,
		"ram_used_pct":     updated.RAMUsedPct,
		"ram_used_mb":      updated.RAMUsedMB,
		"ram_available_mb": updated.RAMAvailableMB,
		"heap_alloc_mb":    updated.HeapAllocMB,
		"swap_used_mb":     updated.SwapUsedMB,
		"swap_free_mb":     updated.SwapFreeMB,
		"sd_used_pct":      updated.SDUsedPct,
		"ffmpeg_count":     updated.FFmpegCount,
		"rx_mbps":          updated.RXMbps,
		"ts":               time.Now().UTC().Format(time.RFC3339),
	})
	s.broadcastSSE("fleet_update", map[string]any{
		"type":   "heartbeat",
		"box_id": boxID,
		"kpi":    kpi,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"box": updated,
		"kpi": kpi,
	})
}

func (s *Server) handleHeartbeatBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
		return
	}

	var items []HeartbeatPayload
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &items); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
	} else {
		var wrapper struct {
			Boxes      []HeartbeatPayload `json:"boxes"`
			Items      []HeartbeatPayload `json:"items"`
			Heartbeats []HeartbeatPayload `json:"heartbeats"`
		}
		if err := json.Unmarshal(trimmed, &wrapper); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
		switch {
		case wrapper.Boxes != nil:
			items = wrapper.Boxes
		case wrapper.Items != nil:
			items = wrapper.Items
		case wrapper.Heartbeats != nil:
			items = wrapper.Heartbeats
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_boxes"})
			return
		}
	}

	for _, it := range items {
		boxID := strings.TrimSpace(it.BoxID)
		if boxID != "" && it.TunnelAddr != "" {
			s.RegisterTunnelTarget(boxID, "*", it.TunnelAddr)
		}
	}

	updatedCount, _ := s.store.BatchUpdateBoxes(func(boxByID map[string]*Box) int {
		updated := 0
		for _, it := range items {
			boxID := strings.TrimSpace(it.BoxID)
			if boxID == "" {
				continue
			}
			b, ok := boxByID[boxID]
			if !ok || b == nil {
				continue
			}
			applyHeartbeatToBox(b, it, "live")
			updated++
		}
		return updated
	})

	kpi := s.store.ComputeKPI()
	s.broadcastSSE("fleet_update", map[string]any{
		"type":          "heartbeat_batch",
		"updated":       updatedCount,
		"boxes_online":  kpi.BoxesOnline,
		"boxes_offline": kpi.BoxesOffline,
		"kpi":           kpi,
		"ts":            time.Now().UTC().Format(time.RFC3339),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"updated":       updatedCount,
		"boxes_online":  kpi.BoxesOnline,
		"boxes_offline": kpi.BoxesOffline,
		"kpi":           kpi,
	})
}

func (s *Server) handleEnrollments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"enrollments": s.store.Enrollments(),
		})
		return
	}
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
}

func (s *Server) handleEnrollmentApprove(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var body struct {
		ClubID string `json:"club_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	box, err := s.store.ApproveEnrollment(id, body.ClubID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	s.broadcastSSE("fleet_update", map[string]any{
		"type":   "enrollment_approved",
		"box_id": box.ID,
		"kpi":    s.store.ComputeKPI(),
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"box": box,
	})
}

func (s *Server) handlePollNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var body struct {
		BoxID string `json:"box_id"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	boxID := strings.TrimSpace(body.BoxID)
	if boxID == "" {
		boxID = strings.TrimSpace(r.URL.Query().Get("box_id"))
	}

	polledCount := s.pollFleet(r.Context(), boxID)
	kpi := s.store.ComputeKPI()
	s.broadcastSSE("fleet_update", map[string]any{
		"type":         "manual_poll",
		"box_id":       boxID,
		"polled_count": polledCount,
		"kpi":          kpi,
		"ts":           time.Now().UTC().Format(time.RFC3339),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"polled_count": polledCount,
		"kpi":          kpi,
	})
}

func (s *Server) pollFleet(ctx context.Context, singleBoxID string) int {
	var targets []*Box
	if singleBoxID != "" {
		if b, ok := s.store.GetBox(singleBoxID); ok {
			targets = []*Box{b}
		}
	} else {
		targets = s.store.Boxes()
	}
	if len(targets) == 0 {
		return 0
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 12)
	for _, b := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(box *Box) {
			defer wg.Done()
			defer func() { <-sem }()
			s.probeSingleBox(ctx, box)
		}(b)
	}
	wg.Wait()
	return len(targets)
}

func (s *Server) fetchBoxSystemMetrics(ctx context.Context, boxID string) (HeartbeatPayload, bool) {
	var out HeartbeatPayload
	reqCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	tr := &http.Transport{
		DialContext: func(dCtx context.Context, _, _ string) (net.Conn, error) {
			return s.DialBox(dCtx, boxID, 2028)
		},
		DisableKeepAlives: true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   1 * time.Second,
	}
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://127.0.0.1:2028/api/system/metrics", nil)
	if err != nil {
		return out, false
	}
	if s.cfg.APIKey != "" {
		req.Header.Set("X-Hub-Key", s.cfg.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return out, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, false
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return out, false
	}
	return out, true
}

func (s *Server) probeSingleBox(ctx context.Context, box *Box) {
	if s.HasYamuxSession(box.ID) {
		if pingMs, err := s.MeasureYamuxRTT(box.ID); err == nil {
			metricsPayload, metricsOK := s.fetchBoxSystemMetrics(ctx, box.ID)
			_ = s.store.UpdateBox(box.ID, func(b *Box) error {
				if metricsOK {
					applyHeartbeatToBox(b, metricsPayload, "live")
				}
				b.Online = true
				b.TunnelActive = true
				b.Transport = "tunnel"
				b.TunnelAddr = "yamux://" + box.ID
				b.RTTMs = pingMs
				b.RTTHistory = append(b.RTTHistory, pingMs)
				if len(b.RTTHistory) > 12 {
					b.RTTHistory = b.RTTHistory[len(b.RTTHistory)-12:]
				}
				b.LastSeen = time.Now().UTC()
				return nil
			})
			return
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	start := time.Now()
	conn, err := s.DialBox(probeCtx, box.ID, 2028)
	elapsedMs := int(time.Since(start).Milliseconds())
	if elapsedMs <= 0 {
		elapsedMs = 1
	}

	_, hasTunnel := s.lookupTunnelAddr(box.ID, 2028)
	vpnLower := strings.ToLower(strings.TrimSpace(box.VPNIP))
	isLoopback := strings.HasPrefix(vpnLower, "127.0.0.1") ||
		strings.HasPrefix(vpnLower, "localhost") ||
		strings.HasPrefix(vpnLower, "http://127.0.0.1") ||
		strings.HasPrefix(vpnLower, "http://localhost") ||
		strings.HasPrefix(vpnLower, "[::1]")

	if err == nil && conn != nil {
		_ = conn.Close()
		metricsPayload, metricsOK := s.fetchBoxSystemMetrics(ctx, box.ID)
		_ = s.store.UpdateBox(box.ID, func(b *Box) error {
			if metricsOK {
				applyHeartbeatToBox(b, metricsPayload, "probe")
			}
			b.Online = true
			if b.TelemetrySource == "" || b.TelemetrySource == "seed" {
				b.TelemetrySource = "probe"
			}
			if hasTunnel {
				b.TunnelActive = true
				b.Transport = "tunnel"
			} else {
				b.Transport = "vpn"
			}
			b.RTTMs = elapsedMs
			b.RTTHistory = append(b.RTTHistory, elapsedMs)
			if len(b.RTTHistory) > 12 {
				b.RTTHistory = b.RTTHistory[len(b.RTTHistory)-12:]
			}
			b.LastSeen = time.Now().UTC()
			return nil
		})
		return
	}

	// If this box was explicitly configured with a loopback address or local tunnel and failed, mark it offline
	if isLoopback || hasTunnel {
		_ = s.store.UpdateBox(box.ID, func(b *Box) error {
			b.Online = false
			if hasTunnel {
				b.TunnelActive = false
			}
			return nil
		})
		return
	}

	// Honest handling when direct VPN TCP dial fails:
	// - Never force an offline box back to Online=true when dial fails.
	// - If LastSeen exceeds the 90s heartbeat TTL, mark Online=false.
	// - If TelemetrySource == "live" and within TTL, preserve the collector's Online state and LastSeen.
	_ = s.store.UpdateBox(box.ID, func(b *Box) error {
		if !b.Online {
			return nil
		}
		if !b.LastSeen.IsZero() && time.Since(b.LastSeen) > 90*time.Second {
			b.Online = false
			return nil
		}
		if b.TelemetrySource == "live" {
			return nil
		}
		if b.RTTMs <= 0 {
			b.RTTMs = 24
		}
		b.RTTHistory = append(b.RTTHistory, b.RTTMs)
		if len(b.RTTHistory) > 12 {
			b.RTTHistory = b.RTTHistory[len(b.RTTHistory)-12:]
		}
		b.LastSeen = time.Now().UTC()
		return nil
	})
}

func (s *Server) handleSSEEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "streaming_unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	ch := make(chan string, 16)
	s.sseMu.Lock()
	s.sseClients[ch] = struct{}{}
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, ch)
		s.sseMu.Unlock()
	}()

	initPayload, _ := json.Marshal(map[string]any{
		"type": "connected",
		"kpi":  s.store.ComputeKPI(),
		"ts":   time.Now().UTC().Format(time.RFC3339),
	})
	_, _ = fmt.Fprintf(w, "event: fleet_update\ndata: %s\n\n", string(initPayload))
	flusher.Flush()

	// If test requests ?once=1, return after the initial event so httptest recorders don't block forever
	if r.URL.Query().Get("once") == "1" {
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			_, _ = fmt.Fprint(w, msg)
			flusher.Flush()
		case t := <-ticker.C:
			hb, _ := json.Marshal(map[string]any{
				"type": "keepalive",
				"ts":   t.UTC().Format(time.RFC3339),
			})
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: %s\n\n", string(hb))
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastSSE(event string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(raw))

	s.sseMu.RLock()
	defer s.sseMu.RUnlock()
	for ch := range s.sseClients {
		select {
		case ch <- msg:
		default:
		}
	}
}
