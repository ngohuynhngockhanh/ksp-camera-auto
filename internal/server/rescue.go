package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mcp"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/tvt"
)

var (
	rescueMidNumRe     = regexp.MustCompile(`(?i)^camera0*([1-9][0-9]*)$`)
	rescueHikTrackChRe = regexp.MustCompile(`(?i)/Streaming/Channels/([0-9]+)`)
	rescueDahuaChRe    = regexp.MustCompile(`(?i)[?&]channel=([0-9]+)`)
)

type rescuePeerSpec struct {
	Name          string   `json:"name"`
	Enabled       bool     `json:"enabled"`
	Iface         string   `json:"iface"`
	VIP           string   `json:"vip"`
	MatchHosts    []string `json:"match_hosts"`
	CameraOffset  int      `json:"camera_offset"`
	CameraCount   int      `json:"camera_count"`
	PublicSiteURL string   `json:"public_site_url,omitempty"`
	FRPCSubdomain string   `json:"frpc_subdomain,omitempty"`
	UITitle       string   `json:"ui_title,omitempty"`
}

type rescueActivateReq struct {
	PeerName      string          `json:"peerName"`
	Iface         string          `json:"iface"`
	VIP           string          `json:"vip"`
	MatchHosts    []string        `json:"matchHosts"`
	FRPCSubdomain string          `json:"frpcSubdomain"`
	PublicSiteURL string          `json:"publicSiteUrl"`
	UITitle       string          `json:"uiTitle"`
	NVRHTTPPort   int             `json:"nvrHttpPort"`
	CameraOffset  int             `json:"cameraOffset"`
	MonitorsJSON  json.RawMessage `json:"monitorsJson"`
}

type rawBackupMonitor struct {
	Mid      string          `json:"mid"`
	Ke       string          `json:"ke"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Mode     string          `json:"mode"`
	Host     string          `json:"host"`
	Port     any             `json:"port"`
	Protocol string          `json:"protocol"`
	Path     string          `json:"path"`
	Ext      string          `json:"ext"`
	FPS      any             `json:"fps"`
	Width    any             `json:"width"`
	Height   any             `json:"height"`
	Details  json.RawMessage `json:"details"`
}

func (s *Server) rescueKeyDir() string {
	if d := strings.TrimSpace(s.cfg.Redbida.KeyDir); d != "" {
		return d
	}
	return "/root/ota-mqtt/change_ok"
}

func (s *Server) loadRescuePeerSpecs() []rescuePeerSpec {
	p := filepath.Join(s.rescueKeyDir(), "rescue_box_json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	var list []rescuePeerSpec
	if json.Unmarshal([]byte(trimmed), &list) == nil {
		return list
	}
	var single rescuePeerSpec
	if json.Unmarshal([]byte(trimmed), &single) == nil && single.Name != "" {
		return []rescuePeerSpec{single}
	}
	return nil
}

func (s *Server) saveRescuePeerSpecs(ctx context.Context, list []rescuePeerSpec) error {
	b, err := json.Marshal(list)
	if err != nil {
		return err
	}
	jsonStr := string(b)
	keyDir := s.rescueKeyDir()
	if err := os.MkdirAll(keyDir, 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(keyDir, "rescue_box_json"), []byte(jsonStr), 0o644)
	}
	if s.redbida != nil {
		_, _ = s.redbida.Apply(ctx, map[string]any{"rescue_box_json": jsonStr}, true)
		_, _ = s.redbida.Apply(ctx, map[string]any{"button_generate_go2rtc_stream": "1"}, true)
	}
	return nil
}

func parseBackupMonitors(raw json.RawMessage) ([]rawBackupMonitor, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("monitorsJson is required")
	}
	if trimmed[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(raw, &unquoted); err != nil {
			return nil, err
		}
		raw = []byte(unquoted)
		trimmed = strings.TrimSpace(unquoted)
	}
	if strings.HasPrefix(trimmed, "[") {
		var mons []rawBackupMonitor
		if err := json.Unmarshal(raw, &mons); err != nil {
			return nil, err
		}
		return mons, nil
	}
	var wrap struct {
		Monitors []rawBackupMonitor `json:"monitors"`
		Data     []rawBackupMonitor `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Monitors) > 0 {
		return wrap.Monitors, nil
	}
	return wrap.Data, nil
}

func extractRTSPChannelAndVendor(autoHost, pathStr string) (config.Vendor, int, string, string, string, int) {
	vendor := config.VendorHikvision
	ch := 0
	host := ""
	rtspPort := 554
	user := ""
	pass := ""

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(autoHost)), "rtsp://") {
		if u, err := url.Parse(strings.TrimSpace(autoHost)); err == nil {
			host = u.Hostname()
			if p, errP := strconv.Atoi(u.Port()); errP == nil && p > 0 {
				rtspPort = p
			}
			if u.User != nil {
				user = u.User.Username()
				pass, _ = u.User.Password()
			}
			if pathStr == "" {
				pathStr = u.RequestURI()
			}
		}
	}

	combined := autoHost + " " + pathStr
	if strings.Contains(strings.ToLower(combined), "/cam/realmonitor") {
		vendor = config.VendorDahua
		if m := rescueDahuaChRe.FindStringSubmatch(combined); len(m) == 2 {
			ch, _ = strconv.Atoi(m[1])
		}
		if ch <= 0 {
			ch = 1
		}
	} else if m := rescueHikTrackChRe.FindStringSubmatch(combined); len(m) == 2 {
		vendor = config.VendorHikvision
		rawTrack, _ := strconv.Atoi(m[1])
		if rawTrack >= 100 {
			ch = rawTrack / 100
		} else if rawTrack > 0 {
			ch = rawTrack
		}
	} else if strings.Contains(strings.ToLower(combined), "/rtsp/streaming") {
		vendor = config.VendorRaysharp
		if m := rescueDahuaChRe.FindStringSubmatch(combined); len(m) == 2 {
			ch, _ = strconv.Atoi(m[1])
		}
	} else if lc := strings.ToLower(combined); strings.Contains(lc, "real_stream") || strings.Contains(lc, "_stream=") || (strings.Contains(lc, "user=") && strings.Contains(lc, "_password=")) {
		vendor = config.VendorXiongmai
		if m := rescueDahuaChRe.FindStringSubmatch(combined); len(m) == 2 {
			ch, _ = strconv.Atoi(m[1])
		}
	} else if tvt.IsTVTRTSPPath(combined) {
		vendor = config.VendorTVT
		if n, ok := tvt.ParseChannelFromRTSP(autoHost, pathStr); ok {
			ch = n
		}
	}
	return vendor, ch, host, user, pass, rtspPort
}

func (s *Server) primaryCameraCount() int {
	maxNum := 0
	count := 0
	for _, d := range s.inv.List() {
		if d.IsNVR || strings.TrimSpace(d.RescueFrom) != "" {
			continue
		}
		count++
		if m := rescueMidNumRe.FindStringSubmatch(strings.TrimSpace(d.Name)); len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	if maxNum > count {
		return maxNum
	}
	return count
}

// handleBoxRescueStatus serves GET /api/rescue/status.
func (s *Server) handleBoxRescueStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	specs := s.loadRescuePeerSpecs()
	var rescuedDevices []map[string]any
	for _, d := range s.inv.List() {
		if strings.TrimSpace(d.RescueFrom) != "" {
			rescuedDevices = append(rescuedDevices, map[string]any{
				"id":         d.ID,
				"name":       d.Name,
				"host":       d.Host,
				"port":       d.Port,
				"vendor":     d.Vendor,
				"nvrChannel": d.NVRChannel,
				"rescueFrom": d.RescueFrom,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"primaryCameras": s.primaryCameraCount(),
		"rescuePeers":    specs,
		"rescuedCameras": rescuedDevices,
	})
}

// handleBoxRescueActivate serves POST /api/rescue/activate:
// Imports a dead box's Shinobi monitor backup into Zero-Idle Rescue Mode:
//   - Registers Camera(offset+1)..Camera(offset+K) in cameras.yaml for 0%-CPU NVR Bridge
//   - Creates idle monitors camera(offset+1)..camera(offset+K) in go-shinobi-ksp (0 FFmpeg processes, 0 SD writes!)
//   - Binds VIP (e.g. 192.168.5.122/24 on lan0) + Gratuitous ARP + FRPC subdomain + go2rtc lazy streams!
func (s *Server) handleBoxRescueActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.shinobi == nil {
		writeErr(w, http.StatusPreconditionFailed, "shinobi integration is not configured in config.yaml")
		return
	}

	var req rescueActivateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.PeerName = strings.TrimSpace(req.PeerName)
	if req.PeerName == "" {
		writeErr(w, http.StatusBadRequest, "peerName is required (e.g. inut_205_122)")
		return
	}
	if strings.TrimSpace(req.Iface) == "" {
		req.Iface = "lan0"
	}

	rawMons, err := parseBackupMonitors(req.MonitorsJSON)
	if err != nil || len(rawMons) == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid or empty monitorsJson: %v", err))
		return
	}

	type indexedMon struct {
		srcSlot int
		mon     rawBackupMonitor
	}
	indexed := make([]indexedMon, 0, len(rawMons))
	for i, m := range rawMons {
		slot := i + 1
		if match := rescueMidNumRe.FindStringSubmatch(strings.TrimSpace(m.Mid)); len(match) == 2 {
			if n, errN := strconv.Atoi(match[1]); errN == nil && n > 0 {
				slot = n
			}
		}
		indexed = append(indexed, indexedMon{srcSlot: slot, mon: m})
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		return indexed[i].srcSlot < indexed[j].srcSlot
	})

	offset := req.CameraOffset
	if offset <= 0 {
		offset = s.primaryCameraCount()
	}
	if offset <= 0 {
		writeErr(w, http.StatusBadRequest, "cameraOffset must be > 0 when box has 0 primary cameras")
		return
	}

	// Clean any stale cameras/monitors previously rescued from the same peerName.
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	s.purgeRescuedPeerAssets(ctx, req.PeerName)

	maxSlot := 0
	var createdMids []string
	for _, item := range indexed {
		if item.srcSlot > maxSlot {
			maxSlot = item.srcSlot
		}
		targetNum := offset + item.srcSlot
		targetMid := fmt.Sprintf("camera%02d", targetNum)
		targetCamName := fmt.Sprintf("Camera%02d", targetNum)

		var detMap map[string]any
		if len(item.mon.Details) > 0 {
			if item.mon.Details[0] == '"' {
				var sDet string
				if json.Unmarshal(item.mon.Details, &sDet) == nil {
					_ = json.Unmarshal([]byte(sDet), &detMap)
				}
			} else {
				_ = json.Unmarshal(item.mon.Details, &detMap)
			}
		}
		if detMap == nil {
			detMap = map[string]any{}
		}
		autoHost, _ := detMap["auto_host"].(string)
		muser, _ := detMap["muser"].(string)
		mpass, _ := detMap["mpass"].(string)

		vendor, ch, rtspHost, uUser, uPass, rtspPort := extractRTSPChannelAndVendor(autoHost, item.mon.Path)
		host := strings.TrimSpace(item.mon.Host)
		if host == "" {
			host = rtspHost
		}
		isMasked := func(v string) bool {
			t := strings.TrimSpace(v)
			return t == "" || strings.Contains(t, "•") || t == "***" || t == "********"
		}
		// Prefer live credentials inside auto_host RTSP URI (Shinobi exports often mask details.mpass as "••••••••")!
		if !isMasked(uUser) {
			muser = uUser
		} else if isMasked(muser) {
			muser = ""
		}
		if !isMasked(uPass) {
			mpass = uPass
		} else if isMasked(mpass) {
			mpass = ""
		}
		if muser == "" || mpass == "" {
			for _, existing := range s.inv.List() {
				if existing.Host == host && existing.Password != "" {
					if muser == "" {
						muser = existing.Username
					}
					if mpass == "" {
						mpass = existing.Password
					}
					break
				}
			}
		}
		if pStr := strings.TrimSpace(fmt.Sprintf("%v", item.mon.Port)); pStr != "" && pStr != "<nil>" {
			if pInt, errP := strconv.Atoi(pStr); errP == nil && pInt > 0 {
				rtspPort = pInt
			}
		}

		ctrlPort := req.NVRHTTPPort
		if ctrlPort <= 0 {
			// Inherit existing control port if same NVR host already exists in inventory (e.g. 192.168.1.50:82)!
			for _, existing := range s.inv.List() {
				if existing.Host == host && existing.Port > 0 {
					ctrlPort = existing.Port
					break
				}
			}
		}
		if ctrlPort <= 0 {
			if vendor == config.VendorDahua {
				ctrlPort = s.cfg.Defaults.DahuaPort
			} else {
				ctrlPort = 80
			}
		}

		devID := fmt.Sprintf("%s:%d", host, ctrlPort)
		if ch > 0 {
			devID = fmt.Sprintf("%s:%d-c%d", host, ctrlPort, ch)
		}
		dev := config.Device{
			ID:         devID,
			Name:       targetCamName,
			Host:       host,
			Port:       ctrlPort,
			Vendor:     vendor,
			Username:   muser,
			Password:   mpass,
			NVRChannel: ch,
			NoStorage:  true,
			RescueFrom: req.PeerName,
		}
		if err := s.inv.Upsert(dev); err != nil {
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("upsert rescued camera %s: %v", targetCamName, err))
			return
		}

		if autoHost == "" && host != "" {
			u := url.URL{
				Scheme: "rtsp",
				User:   url.UserPassword(muser, mpass),
				Host:   fmt.Sprintf("%s:%d", host, rtspPort),
				Path:   item.mon.Path,
			}
			autoHost = u.String()
		}

		// Save monitor in go-shinobi-ksp with mode="idle" (0 FFmpeg processes, 0 SD writes!)
		// + nvr_direct="1" (72h NVR Bridge playback) + auto_host (lazy go2rtc live stream!).
		monCfg := shinobi.MonitorConfig{
			Mid:      targetMid,
			Ke:       s.shinobi.GroupKey(),
			Name:     targetCamName,
			Type:     "h264",
			Mode:     "idle",
			Host:     host,
			Port:     strconv.Itoa(rtspPort),
			Protocol: "rtsp",
			Path:     item.mon.Path,
			Ext:      "mp4",
			FPS:      "20",
			Width:    "1920",
			Height:   "1080",
			Details: shinobi.MonitorDetails{
				AutoHost:                 autoHost,
				Muser:                    muser,
				Mpass:                    mpass,
				Port:                     strconv.Itoa(rtspPort),
				Protocol:                 "rtsp",
				StreamType:               "hls",
				StreamVcodec:             "copy",
				StreamAcodec:             "no",
				Vcodec:                   "copy",
				Acodec:                   "no",
				RecordVcodec:             "copy",
				RecordAcodec:             "no",
				CustInput:                "",
				CustStream:               "",
				CustRecord:               "-tag:v hvc1 -segment_format_options moov_size=262144:movflags=+faststart",
				WallClockTimestampIgnore: "0",
				Cutoff:                   "5",
				Detector:                 "0",
				NVRDirect:                "1",
				RescueFrom:               req.PeerName,
			},
		}
		if err := s.shinobi.AddMonitor(ctx, monCfg); err != nil {
			writeErr(w, http.StatusBadGateway, fmt.Sprintf("configure idle shinobi monitor %s: %v", targetMid, err))
			return
		}
		createdMids = append(createdMids, targetMid)
	}

	// Bind VIP immediately on the network interface + send Gratuitous ARP!
	vipBound := false
	if vip := strings.TrimSpace(req.VIP); vip != "" {
		cidr := vip
		if !strings.Contains(cidr, "/") {
			cidr = vip + "/24"
		}
		if ip, _, errP := net.ParseCIDR(cidr); errP == nil && ip.To4() != nil {
			_ = exec.Command("ip", "address", "add", cidr, "dev", req.Iface).Run()
			ip4 := ip.To4()
			bcast := fmt.Sprintf("%d.%d.%d.255", ip4[0], ip4[1], ip4[2])
			_ = exec.Command("ping", "-c", "1", "-W", "1", "-b", "-I", ip4.String(), bcast).Run()
			vipBound = true
		}
	}

	matchHosts := req.MatchHosts
	if vipRaw := strings.Split(strings.TrimSpace(req.VIP), "/")[0]; vipRaw != "" {
		hasVIP := false
		for _, h := range matchHosts {
			if strings.EqualFold(strings.TrimSpace(h), vipRaw) {
				hasVIP = true
				break
			}
		}
		if !hasVIP {
			matchHosts = append([]string{vipRaw}, matchHosts...)
		}
	}
	if req.FRPCSubdomain != "" && req.PublicSiteURL == "" {
		req.PublicSiteURL = "http://" + strings.TrimSpace(req.FRPCSubdomain) + ".video.io.vn"
	}

	specs := s.loadRescuePeerSpecs()
	var updated []rescuePeerSpec
	for _, sp := range specs {
		if !strings.EqualFold(sp.Name, req.PeerName) {
			updated = append(updated, sp)
		}
	}
	newSpec := rescuePeerSpec{
		Name:          req.PeerName,
		Enabled:       true,
		Iface:         req.Iface,
		VIP:           req.VIP,
		MatchHosts:    matchHosts,
		CameraOffset:  offset,
		CameraCount:   maxSlot,
		PublicSiteURL: req.PublicSiteURL,
		FRPCSubdomain: req.FRPCSubdomain,
		UITitle:       req.UITitle,
	}
	updated = append(updated, newSpec)
	_ = s.saveRescuePeerSpecs(ctx, updated)
	if s.bridge != nil {
		s.bridge.FlushRecordingsCache()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"peer":         newSpec,
		"vipBound":     vipBound,
		"cameraOffset": offset,
		"cameraCount":  maxSlot,
		"rescuedMids":  createdMids,
	})
}

func (s *Server) purgeRescuedPeerAssets(ctx context.Context, peerName string) (removedDevs int, removedMons int) {
	peerName = strings.TrimSpace(peerName)
	if peerName == "" {
		return 0, 0
	}
	for _, d := range s.inv.List() {
		if strings.EqualFold(strings.TrimSpace(d.RescueFrom), peerName) {
			if s.shinobi != nil {
				if m := rescueMidNumRe.FindStringSubmatch(strings.TrimSpace(d.Name)); len(m) == 2 {
					if n, err := strconv.Atoi(m[1]); err == nil {
						mid := fmt.Sprintf("camera%02d", n)
						if s.shinobi.DeleteMonitor(ctx, mid) == nil {
							removedMons++
						}
					}
				}
			}
			if err := s.inv.Delete(d.ID); err == nil {
				removedDevs++
			}
		}
	}
	return removedDevs, removedMons
}

// handleBoxRescueDeactivate serves POST /api/rescue/deactivate.
func (s *Server) handleBoxRescueDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		PeerName string `json:"peerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.PeerName = strings.TrimSpace(req.PeerName)
	if req.PeerName == "" {
		writeErr(w, http.StatusBadRequest, "peerName is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	specs := s.loadRescuePeerSpecs()
	var remaining []rescuePeerSpec
	for _, sp := range specs {
		if strings.EqualFold(sp.Name, req.PeerName) {
			if vip := strings.TrimSpace(sp.VIP); vip != "" {
				cidr := vip
				if !strings.Contains(cidr, "/") {
					cidr = vip + "/24"
				}
				iface := sp.Iface
				if iface == "" {
					iface = "lan0"
				}
				_ = exec.Command("ip", "address", "del", cidr, "dev", iface).Run()
			}
			continue
		}
		remaining = append(remaining, sp)
	}
	removedDevs, removedMons := s.purgeRescuedPeerAssets(ctx, req.PeerName)
	_ = s.saveRescuePeerSpecs(ctx, remaining)
	if s.bridge != nil {
		s.bridge.FlushRecordingsCache()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"removedCameras":  removedDevs,
		"removedMonitors": removedMons,
		"remainingPeers":  remaining,
	})
}

func (s *Server) registerRescueMCPTools() {
	if s.mcp == nil {
		return
	}
	s.mcp.Registry().Register(mcp.Tool{
		Name:        "kspcam_rescue_box",
		Description: "Activate, deactivate, or inspect Zero-Idle Box Rescue / Peer Failover Mode: binds dead peer's Virtual IP on lan0/eth0 + FRPC subdomain, registers rescued cameras as idle monitors (0% idle CPU/disk, on-demand go2rtc live stream + 0%-CPU NVR Bridge replay/cut).",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "One of 'status', 'activate', or 'deactivate'",
				},
				"peerName": map[string]any{
					"type":        "string",
					"description": "Dead box identifier, e.g. 'inut_205_122'",
				},
				"iface": map[string]any{
					"type":        "string",
					"description": "Network interface for Virtual IP (default 'lan0')",
				},
				"vip": map[string]any{
					"type":        "string",
					"description": "Virtual IP / CIDR to take over, e.g. '192.168.5.122/24'",
				},
				"frpcSubdomain": map[string]any{
					"type":        "string",
					"description": "Optional rescued FRPC subdomain, e.g. 'store-magic-1'",
				},
				"publicSiteUrl": map[string]any{
					"type":        "string",
					"description": "Optional public URL for 4G QR codes, e.g. 'http://store-magic-1.video.io.vn'",
				},
				"nvrHttpPort": map[string]any{
					"type":        "integer",
					"description": "NVR HTTP/DVRIP control port (e.g. 82 or 37777; auto-inherited from inventory if 0)",
				},
				"cameraOffset": map[string]any{
					"type":        "integer",
					"description": "Camera slot offset (auto-computed from primary camera count if 0)",
				},
				"monitorsJson": map[string]any{
					"type":        "string",
					"description": "Shinobi monitor export JSON string or array from the dead box backup",
				},
			},
			Required: []string{"action"},
		},
	}, func(ctx context.Context, args json.RawMessage) (mcp.ToolResult, error) {
		var base struct {
			Action string `json:"action"`
		}
		if len(args) > 0 {
			_ = json.Unmarshal(args, &base)
		}
		act := strings.ToLower(strings.TrimSpace(base.Action))
		if act == "" {
			act = "status"
		}

		rec := httptest.NewRecorder()
		switch act {
		case "status":
			req := httptest.NewRequest(http.MethodGet, "/api/rescue/status", nil).WithContext(ctx)
			s.handleBoxRescueStatus(rec, req)
		case "activate":
			req := httptest.NewRequest(http.MethodPost, "/api/rescue/activate", bytes.NewReader(args)).WithContext(ctx)
			s.handleBoxRescueActivate(rec, req)
		case "deactivate":
			req := httptest.NewRequest(http.MethodPost, "/api/rescue/deactivate", bytes.NewReader(args)).WithContext(ctx)
			s.handleBoxRescueDeactivate(rec, req)
		default:
			return mcp.NewErrorResult("unsupported action: " + act), fmt.Errorf("unsupported action %q", act)
		}
		if rec.Code >= 300 {
			return mcp.NewErrorResult(rec.Body.String()), fmt.Errorf("rescue %s HTTP %d: %s", act, rec.Code, rec.Body.String())
		}
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return mcp.NewJSONResult(out)
	})
}
