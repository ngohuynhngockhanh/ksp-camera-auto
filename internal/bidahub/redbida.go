package bidahub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DefaultRedBidaKeys constructs the 17-Rule Golden Standard RedBida OTA-MQTT key map,
// strictly preserving Pickleball vs Billiards venue branding.
func DefaultRedBidaKeys(clubName, branding, virtualIP, groupKey, ggcode, frpcConfig string) map[string]string {
	isPickle := strings.EqualFold(branding, "pickleball") || strings.Contains(strings.ToLower(clubName), "pickleball")
	companyName := "Billiard Live"
	hashtag := "#BilliardLive"
	tablePrefix := "Bàn"
	if isPickle {
		companyName = "Pickleball Live"
		hashtag = "#PickleballLive"
		tablePrefix = "Sân"
	}
	if virtualIP == "" {
		virtualIP = "192.168.1.254"
	}
	if groupKey == "" {
		groupKey = "inut"
	}
	if ggcode == "" {
		ggcode = strings.ToUpper(groupKey)
	}
	return map[string]string{
		"ui_title":                    clubName,
		"company_name":                companyName,
		"hashtag":                     hashtag,
		"table_prefix":                tablePrefix,
		"shinobi_group_key":           groupKey,
		"ggcode":                      ggcode,
		"eth0_virtual_ip":             virtualIP,
		"end0_virtual_ip":             virtualIP,
		"wifi_ip":                     virtualIP,
		"qr_ip":                       virtualIP,
		"frpc_config":                 frpcConfig,
		"cut_duration_sec":            "300",
		"playback_days":               "3",
		"video_format":                "h265_frag_mp4",
		"ntp_server":                  "time.google.com",
		"max_free_ram_restart_camera": "80",
		"auto_clean_disk_pct":         "90",
		"iptables_redirect_80_2023":   "enabled",
	}
}

func ensureAll20Tabs(tabs map[string]string, branding string) map[string]string {
	if tabs == nil {
		tabs = make(map[string]string, 20)
	}
	prefix := "Bàn"
	if strings.EqualFold(branding, "pickleball") {
		prefix = "Sân"
	}
	for c := 1; c <= 20; c++ {
		k := fmt.Sprintf("C%02d", c)
		if _, ok := tabs[k]; !ok {
			tabs[k] = fmt.Sprintf("[%s]\nname=%s %02d\nhost=192.168.1.%d\nchannel=%d\n", k, prefix, c, 100+c, c)
		}
	}
	return tabs
}

var (
	reSubdomainStore  = regexp.MustCompile(`(?m)^\s*subdomain\s*=\s*(store-[a-zA-Z0-9_-]+)`)
	reSubdomainCStore = regexp.MustCompile(`(?m)^\s*subdomain\s*=\s*(c-store-[a-zA-Z0-9_-]+)`)
)

func extractDomainsFromFRPC(frpcIni string) (storeDomain, cstoreDomain string) {
	if m := reSubdomainStore.FindStringSubmatch(frpcIni); len(m) > 1 {
		storeDomain = m[1] + ".video.io.vn"
	}
	if m := reSubdomainCStore.FindStringSubmatch(frpcIni); len(m) > 1 {
		cstoreDomain = m[1] + ".video.io.vn"
	}
	return storeDomain, cstoreDomain
}

// BuildRedBidaReport evaluates the 17-Rule RedBida Golden Inspector and FRPC subdomain status for a Box.
func BuildRedBidaReport(box *Box, allBoxes []*Box) *RedBidaReport {
	if box.RedBidaKeys == nil {
		box.RedBidaKeys = DefaultRedBidaKeys(box.ClubName, box.Branding, box.VirtualIP, box.ShinobiGroupKey, box.GGCode, box.FRPCConfig)
	}
	keys := box.RedBidaKeys

	isPickle := strings.EqualFold(box.Branding, "pickleball") || strings.Contains(strings.ToLower(box.ClubName), "pickleball")
	expectedCompany := "Billiard Live"
	expectedHashtag := "#BilliardLive"
	expectedBranding := "billiards"
	if isPickle {
		expectedCompany = "Pickleball Live"
		expectedHashtag = "#PickleballLive"
		expectedBranding = "pickleball"
	}

	// Detect FRPC subdomain conflicts across allBoxes
	storeOwners := []string{}
	cstoreOwners := []string{}
	for _, other := range allBoxes {
		if other.StoreDomain != "" && strings.EqualFold(other.StoreDomain, box.StoreDomain) {
			storeOwners = append(storeOwners, other.ID)
		}
		if other.CStoreDomain != "" && strings.EqualFold(other.CStoreDomain, box.CStoreDomain) {
			cstoreOwners = append(cstoreOwners, other.ID)
		}
	}
	var conflictDetails []string
	if len(storeOwners) > 1 {
		conflictDetails = append(conflictDetails, fmt.Sprintf("Subdomain %s trùng lặp trên: %s", box.StoreDomain, strings.Join(storeOwners, ", ")))
	}
	if len(cstoreOwners) > 1 {
		conflictDetails = append(conflictDetails, fmt.Sprintf("Subdomain %s trùng lặp trên: %s", box.CStoreDomain, strings.Join(cstoreOwners, ", ")))
	}
	hasConflict := len(conflictDetails) > 0

	makeRule := func(id int, key, title, expected, actual string, pass bool) RedBidaRule {
		st := "pass"
		if !pass {
			st = "fail"
		}
		return RedBidaRule{
			ID:       id,
			Key:      key,
			Title:    title,
			Expected: expected,
			Actual:   actual,
			Status:   st,
			Passed:   pass,
		}
	}

	vip := keys["eth0_virtual_ip"]
	if vip == "" {
		vip = box.VirtualIP
	}
	end0VIP := keys["end0_virtual_ip"]
	if end0VIP == "" {
		end0VIP = box.VirtualIP
	}

	rules := []RedBidaRule{
		makeRule(1, "ui_title", "Tiêu đề quán (ui_title)", box.ClubName, keys["ui_title"], strings.TrimSpace(keys["ui_title"]) != ""),
		makeRule(2, "company_name", "Thương hiệu bộ môn (Billiards / Pickleball)", expectedCompany, keys["company_name"], strings.EqualFold(keys["company_name"], expectedCompany)),
		makeRule(3, "hashtag", "Hashtag chuẩn bộ môn", expectedHashtag, keys["hashtag"], strings.EqualFold(keys["hashtag"], expectedHashtag)),
		makeRule(4, "shinobi_group_key", "Shinobi Group Key đồng bộ", box.ShinobiGroupKey, keys["shinobi_group_key"], keys["shinobi_group_key"] != "" && keys["shinobi_group_key"] == box.ShinobiGroupKey),
		makeRule(5, "ggcode", "Mã định danh GGCode", box.GGCode, keys["ggcode"], strings.TrimSpace(keys["ggcode"]) != ""),
		makeRule(6, "eth0_virtual_ip", "Virtual IP eth0:1 đồng bộ", box.VirtualIP, vip, vip != "" && vip == box.VirtualIP),
		makeRule(7, "end0_virtual_ip", "Virtual IP end0:1 đồng bộ", box.VirtualIP, end0VIP, end0VIP != "" && end0VIP == box.VirtualIP),
		makeRule(8, "wifi_ip", "Địa chỉ Wi-Fi IP đồng bộ Virtual IP", box.VirtualIP, keys["wifi_ip"], keys["wifi_ip"] == box.VirtualIP),
		makeRule(9, "qr_ip", "Địa chỉ QR IP đồng bộ Virtual IP", box.VirtualIP, keys["qr_ip"], keys["qr_ip"] == box.VirtualIP),
		makeRule(10, "frpc_config", "Cấu hình FRPC store-* & c-store-* không xung đột", "Unique store-* & c-store-*", box.StoreDomain+" / "+box.CStoreDomain, !hasConflict && box.StoreDomain != "" && box.CStoreDomain != ""),
		makeRule(11, "store_http_200", "Tên miền Store trả về HTTP 200", "200", fmt.Sprintf("%d", box.StoreHTTPCode), box.StoreHTTPCode == 200 && !hasConflict),
		makeRule(12, "cstore_http_200", "Tên miền C-Store trả về HTTP 200", "200", fmt.Sprintf("%d", box.CStoreHTTPCode), box.CStoreHTTPCode == 200 && !hasConflict),
		makeRule(13, "cut_duration_sec", "Thời lượng cắt clip nhanh (300s / 5 phút)", "300", keys["cut_duration_sec"], keys["cut_duration_sec"] == "300"),
		makeRule(14, "playback_days", "Số ngày lưu trữ xem lại (3 ngày)", "3", keys["playback_days"], keys["playback_days"] == "3"),
		makeRule(15, "video_format", "Định dạng H.265 Fragmented MP4", "h265_frag_mp4", keys["video_format"], keys["video_format"] == "h265_frag_mp4"),
		makeRule(16, "max_free_ram_restart_camera", "Ngưỡng RAM an toàn không gây restart lặp (<=100MB)", "80", keys["max_free_ram_restart_camera"], keys["max_free_ram_restart_camera"] == "80" || keys["max_free_ram_restart_camera"] == "60"),
		makeRule(17, "iptables_redirect_80_2023", "Chuyển tiếp cổng iptables 80 -> 2023", "enabled", keys["iptables_redirect_80_2023"], keys["iptables_redirect_80_2023"] == "enabled"),
	}

	score := 0
	for _, r := range rules {
		if r.Passed {
			score++
		}
	}

	subdomains := []FRPCSubdomainEntry{
		{
			Subdomain: strings.TrimSuffix(box.StoreDomain, ".video.io.vn"),
			FullURL:   "https://" + box.StoreDomain,
			Kind:      "store",
			BoxID:     box.ID,
			BoxName:   box.Name,
			ClubName:  box.ClubName,
			HTTPCode:  box.StoreHTTPCode,
			Conflict:  len(storeOwners) > 1,
		},
		{
			Subdomain: strings.TrimSuffix(box.CStoreDomain, ".video.io.vn"),
			FullURL:   "https://" + box.CStoreDomain,
			Kind:      "c-store",
			BoxID:     box.ID,
			BoxName:   box.Name,
			ClubName:  box.ClubName,
			HTTPCode:  box.CStoreHTTPCode,
			Conflict:  len(cstoreOwners) > 1,
		},
	}

	return &RedBidaReport{
		BoxID:           box.ID,
		BoxName:         box.Name,
		ClubID:          box.ClubID,
		ClubName:        box.ClubName,
		VPNIP:           box.VPNIP,
		Branding:        expectedBranding,
		Score:           score,
		MaxScore:        17,
		StoreDomain:     box.StoreDomain,
		CStoreDomain:    box.CStoreDomain,
		StoreHTTPCode:   box.StoreHTTPCode,
		CStoreHTTPCode:  box.CStoreHTTPCode,
		DomainConflict:  hasConflict,
		ConflictDetails: conflictDetails,
		ShinobiGroupKey: box.ShinobiGroupKey,
		GGCode:          box.GGCode,
		Eth0VirtualIP:   vip,
		End0VirtualIP:   end0VIP,
		FRPCConfig:      box.FRPCConfig,
		Keys:            keys,
		Tabs:            ensureAll20Tabs(box.RedBidaTabs, expectedBranding),
		Rules:           rules,
		Subdomains:      subdomains,
	}
}

func computeFleetFRPCAudit(allBoxes []*Box) ([]FRPCSubdomainEntry, []FRPCConflict) {
	byDomain := make(map[string][]string)
	for _, b := range allBoxes {
		if b.StoreDomain != "" {
			d := strings.ToLower(b.StoreDomain)
			byDomain[d] = append(byDomain[d], b.ID)
		}
		if b.CStoreDomain != "" {
			d := strings.ToLower(b.CStoreDomain)
			byDomain[d] = append(byDomain[d], b.ID)
		}
	}

	var entries []FRPCSubdomainEntry
	var conflicts []FRPCConflict
	seenConflict := make(map[string]bool)

	for _, b := range allBoxes {
		sOwners := byDomain[strings.ToLower(b.StoreDomain)]
		cOwners := byDomain[strings.ToLower(b.CStoreDomain)]
		sConflict := len(sOwners) > 1
		cConflict := len(cOwners) > 1

		entries = append(entries,
			FRPCSubdomainEntry{
				Subdomain: strings.TrimSuffix(b.StoreDomain, ".video.io.vn"),
				FullURL:   "https://" + b.StoreDomain,
				Kind:      "store",
				BoxID:     b.ID,
				BoxName:   b.Name,
				ClubName:  b.ClubName,
				HTTPCode:  b.StoreHTTPCode,
				Conflict:  sConflict,
			},
			FRPCSubdomainEntry{
				Subdomain: strings.TrimSuffix(b.CStoreDomain, ".video.io.vn"),
				FullURL:   "https://" + b.CStoreDomain,
				Kind:      "c-store",
				BoxID:     b.ID,
				BoxName:   b.Name,
				ClubName:  b.ClubName,
				HTTPCode:  b.CStoreHTTPCode,
				Conflict:  cConflict,
			},
		)

		if sConflict && !seenConflict[strings.ToLower(b.StoreDomain)] {
			seenConflict[strings.ToLower(b.StoreDomain)] = true
			conflicts = append(conflicts, FRPCConflict{
				Subdomain: b.StoreDomain,
				BoxIDs:    sOwners,
				Message:   fmt.Sprintf("Xung đột FRPC subdomain %s giữa các Box: %s", b.StoreDomain, strings.Join(sOwners, ", ")),
			})
		}
		if cConflict && !seenConflict[strings.ToLower(b.CStoreDomain)] {
			seenConflict[strings.ToLower(b.CStoreDomain)] = true
			conflicts = append(conflicts, FRPCConflict{
				Subdomain: b.CStoreDomain,
				BoxIDs:    cOwners,
				Message:   fmt.Sprintf("Xung đột FRPC subdomain %s giữa các Box: %s", b.CStoreDomain, strings.Join(cOwners, ", ")),
			})
		}
	}
	return entries, conflicts
}

func (s *Server) handleRedBidaList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	boxes := s.store.Boxes()
	reports := make([]*RedBidaReport, 0, len(boxes))
	compliantCount := 0
	for _, b := range boxes {
		rep := BuildRedBidaReport(b, boxes)
		reports = append(reports, rep)
		if rep.Score == 17 && !rep.DomainConflict {
			compliantCount++
		}
	}
	domains, conflicts := computeFleetFRPCAudit(boxes)
	domainsOK := 0
	for _, d := range domains {
		if d.HTTPCode == 200 && !d.Conflict {
			domainsOK++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"boxes":     reports,
		"domains":   domains,
		"conflicts": conflicts,
		"summary": map[string]any{
			"total_boxes":     len(reports),
			"compliant_boxes": compliantCount,
			"domains_total":   len(domains),
			"domains_ok":      domainsOK,
			"conflicts_count": len(conflicts),
		},
	})
}

func (s *Server) handleBoxRedBidaApply(w http.ResponseWriter, r *http.Request, boxID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	var req struct {
		AutoFix         bool              `json:"auto_fix"`
		Unlocked        bool              `json:"unlocked"`
		PreserveBrand   *bool             `json:"preserve_branding"`
		Branding        string            `json:"branding"`
		ShinobiGroupKey string            `json:"shinobi_group_key"`
		GGCode          string            `json:"ggcode"`
		Eth0VirtualIP   string            `json:"eth0_virtual_ip"`
		End0VirtualIP   string            `json:"end0_virtual_ip"`
		VirtualIP       string            `json:"virtual_ip"`
		FRPCConfig      string            `json:"frpc_config"`
		StoreDomain     string            `json:"store_domain"`
		CStoreDomain    string            `json:"cstore_domain"`
		Keys            map[string]string `json:"keys"`
		Tabs            map[string]string `json:"tabs"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	err := s.store.UpdateBox(boxID, func(b *Box) error {
		if b.RedBidaKeys == nil {
			b.RedBidaKeys = DefaultRedBidaKeys(b.ClubName, b.Branding, b.VirtualIP, b.ShinobiGroupKey, b.GGCode, b.FRPCConfig)
		}
		if b.RedBidaTabs == nil {
			b.RedBidaTabs = make(map[string]string)
		}

		isPickleVenue := strings.EqualFold(b.Branding, "pickleball") || strings.Contains(strings.ToLower(b.ClubName), "pickleball")

		// Apply caller-supplied keys first
		for k, v := range req.Keys {
			b.RedBidaKeys[k] = v
		}
		for k, v := range req.Tabs {
			b.RedBidaTabs[k] = v
		}

		// Sync explicit top-level or key-level fields
		if req.ShinobiGroupKey != "" {
			b.ShinobiGroupKey = req.ShinobiGroupKey
			b.RedBidaKeys["shinobi_group_key"] = req.ShinobiGroupKey
		} else if v := req.Keys["shinobi_group_key"]; v != "" {
			b.ShinobiGroupKey = v
		}

		if req.GGCode != "" {
			b.GGCode = req.GGCode
			b.RedBidaKeys["ggcode"] = req.GGCode
		} else if v := req.Keys["ggcode"]; v != "" {
			b.GGCode = v
		}

		newVIP := req.VirtualIP
		if newVIP == "" {
			newVIP = req.Eth0VirtualIP
		}
		if newVIP == "" {
			newVIP = req.End0VirtualIP
		}
		if newVIP == "" {
			newVIP = req.Keys["eth0_virtual_ip"]
		}
		if newVIP != "" {
			b.VirtualIP = newVIP
			b.RedBidaKeys["eth0_virtual_ip"] = newVIP
			b.RedBidaKeys["end0_virtual_ip"] = newVIP
			b.RedBidaKeys["wifi_ip"] = newVIP
			b.RedBidaKeys["qr_ip"] = newVIP
		}

		if req.FRPCConfig != "" {
			b.FRPCConfig = req.FRPCConfig
			b.RedBidaKeys["frpc_config"] = req.FRPCConfig
			sDom, cDom := extractDomainsFromFRPC(req.FRPCConfig)
			if sDom != "" {
				b.StoreDomain = sDom
			}
			if cDom != "" {
				b.CStoreDomain = cDom
			}
		} else if v := req.Keys["frpc_config"]; v != "" {
			b.FRPCConfig = v
			sDom, cDom := extractDomainsFromFRPC(v)
			if sDom != "" {
				b.StoreDomain = sDom
			}
			if cDom != "" {
				b.CStoreDomain = cDom
			}
		}

		if req.StoreDomain != "" {
			dom := req.StoreDomain
			if !strings.Contains(dom, ".") {
				dom += ".video.io.vn"
			}
			b.StoreDomain = dom
		}
		if req.CStoreDomain != "" {
			dom := req.CStoreDomain
			if !strings.Contains(dom, ".") {
				dom += ".video.io.vn"
			}
			b.CStoreDomain = dom
		}

		// Auto-fix 17 Golden Rules while strictly preserving Pickleball vs Billiards branding
		if req.AutoFix {
			defaults := DefaultRedBidaKeys(b.ClubName, b.Branding, b.VirtualIP, b.ShinobiGroupKey, b.GGCode, b.FRPCConfig)
			for k, v := range defaults {
				b.RedBidaKeys[k] = v
			}
			b.StoreHTTPCode = 200
			b.CStoreHTTPCode = 200
		}

		// Guard: Never allow Pickleball venues (e.g. inut_205_73 Pika Sun Pickleball) to lose Pickleball branding
		if isPickleVenue {
			b.Branding = "pickleball"
			b.RedBidaKeys["company_name"] = "Pickleball Live"
			b.RedBidaKeys["hashtag"] = "#PickleballLive"
			b.RedBidaKeys["table_prefix"] = "Sân"
		} else if req.Branding == "billiards" || b.Branding == "" {
			b.Branding = "billiards"
		}

		b.LastSeen = time.Now().UTC()
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "box_not_found", "box_id": boxID})
		return
	}

	allBoxes := s.store.Boxes()
	updatedBox, _ := s.store.GetBox(boxID)
	report := BuildRedBidaReport(updatedBox, allBoxes)
	_ = s.store.UpdateBox(boxID, func(b *Box) error {
		b.RedBidaScore = report.Score
		return nil
	})
	updatedBox, _ = s.store.GetBox(boxID)
	_, conflicts := computeFleetFRPCAudit(allBoxes)

	s.broadcastSSE("fleet_update", map[string]any{
		"type":          "redbida_applied",
		"box_id":        boxID,
		"redbida_score": report.Score,
		"branding":      report.Branding,
		"kpi":           s.store.ComputeKPI(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"box_id":          boxID,
		"branding":        report.Branding,
		"score":           report.Score,
		"max_score":       report.MaxScore,
		"domain_conflict": report.DomainConflict,
		"conflicts":       conflicts,
		"report":          report,
		"box":             updatedBox,
	})
}
