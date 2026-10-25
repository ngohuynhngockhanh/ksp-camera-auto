package bidahub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var reForbiddenDiskFormat = regexp.MustCompile(`(?i)\b(mkfs(?:\.[a-z0-9]+)?|fdisk|sfdisk|cfdisk|parted|wipefs|format_disk|format_sd|format|mkswap)\b|\bdd\s+if=`)

// ValidateNoDiskFormat enforces the Golden SOP storage safety invariant:
// never execute or accept any disk-formatting or partition-wiping command.
func ValidateNoDiskFormat(input string) error {
	if m := reForbiddenDiskFormat.FindString(input); m != "" {
		return fmt.Errorf("storage safety violation: forbidden disk formatting keyword %q detected", strings.TrimSpace(m))
	}
	lower := strings.ToLower(input)
	if strings.Contains(lower, "\"format\"") || strings.Contains(lower, "format=true") || strings.Contains(lower, "format_card") {
		return fmt.Errorf("storage safety violation: disk formatting is strictly prohibited by Golden SOP")
	}
	return nil
}

// Build8StepHardening evaluates all 8 Golden Hardening SOP pillars for a Box.
func Build8StepHardening(box *Box) *HardeningReport {
	step1OK := box.ZRAMMemLimit == 0 && box.Swappiness == 80 && box.ZRAMTotalMB > 0
	step2OK := box.PM2CameraLimitMB >= 160 && box.PM2InutLimitMB >= 240 && box.PM2OtaLimitMB >= 100 && box.PM2LogsDevNull
	step3OK := box.SysrqEnabled == 0 && box.SysrqHooksRemoved
	step4OK := box.CameraWatchdogActive && (box.FFmpegCount >= box.CamerasCount || box.CamerasCount == 0)
	step5OK := box.StorageGuardActive && box.SDUsedPct < 90
	step6OK := box.UdevAutomount && (strings.Contains(box.SDMountOpts, "noatime") || strings.Contains(box.SDMountOpts, "errors=continue"))
	step7OK := box.CleanFstab
	step8OK := !box.ShinobiDoSnapshot && box.ShinobiWorkerFix && box.ShinobiFragMP4 && box.CorruptedMP4Count == 0

	makeStatus := func(ok bool) string {
		if ok {
			return "pass"
		}
		return "fail"
	}

	pillars := []HardeningPillar{
		{
			ID:             "zram_unbounded",
			Step:           1,
			Name:           "ZRAM Swap 100% Unbounded (mem_limit=0, swappiness=80)",
			Status:         makeStatus(step1OK),
			Passed:         step1OK,
			Detail:         fmt.Sprintf("zram=%dMB (%s), mem_limit=%d, vm.swappiness=%d", box.ZRAMTotalMB, box.ZRAMAlgo, box.ZRAMMemLimit, box.Swappiness),
			CommandPreview: "echo 0 > /sys/block/zram0/mem_limit && sysctl -w vm.swappiness=80 vm.vfs_cache_pressure=100",
		},
		{
			ID:             "pm2_memory_guard",
			Step:           2,
			Name:           "PM2 Memory Guard & /dev/null Logs",
			Status:         makeStatus(step2OK),
			Passed:         step2OK,
			Detail:         fmt.Sprintf("camera=%dM, inut=%dM, ota-mqtt=%dM, logs_dev_null=%v", box.PM2CameraLimitMB, box.PM2InutLimitMB, box.PM2OtaLimitMB, box.PM2LogsDevNull),
			CommandPreview: fmt.Sprintf("pm2 startOrReload ecosystem.config.js --max-memory-restart %dM --output /dev/null --error /dev/null && pm2 save", box.PM2CameraLimitMB),
		},
		{
			ID:             "sysrq_disabled",
			Step:           3,
			Name:           "Neutralize sysrq-trigger Suicide Hooks (kernel.sysrq=0)",
			Status:         makeStatus(step3OK),
			Passed:         step3OK,
			Detail:         fmt.Sprintf("kernel.sysrq=%d, nodered/netwatch hooks removed=%v", box.SysrqEnabled, box.SysrqHooksRemoved),
			CommandPreview: "sysctl -w kernel.sysrq=0 && sed -i 's/echo b > \\/proc\\/sysrq-trigger/true/g' /root/.node-red/flows*.json /usr/local/bin/force_reboot.sh",
		},
		{
			ID:             "camera_watchdog",
			Step:           4,
			Name:           "Smart Watchdog 60s (shinobi-camera-watchdog.timer)",
			Status:         makeStatus(step4OK),
			Passed:         step4OK,
			Detail:         fmt.Sprintf("timer_active=%v, ffmpeg_active=%d/%d", box.CameraWatchdogActive, box.FFmpegCount, box.FFmpegExpected),
			CommandPreview: "systemctl enable --now shinobi-camera-watchdog.timer",
		},
		{
			ID:             "storage_guard",
			Step:           5,
			Name:           "Storage Guard 15m (90% -> 80% Auto-Trim)",
			Status:         makeStatus(step5OK),
			Passed:         step5OK,
			Detail:         fmt.Sprintf("timer_active=%v, sd_used=%d%% (%d/%d GB)", box.StorageGuardActive, box.SDUsedPct, box.SDUsedGB, box.SDTotalGB),
			CommandPreview: "systemctl enable --now shinobi-storage-guard.timer && /usr/local/bin/shinobi-storage-guard.sh",
		},
		{
			ID:             "udev_automount",
			Step:           6,
			Name:           "Udev Automount (errors=continue,noatime,users — Zero Format)",
			Status:         makeStatus(step6OK),
			Passed:         step6OK,
			Detail:         fmt.Sprintf("fs=%s, opts=%s, udev_profile=%v", box.SDFSType, box.SDMountOpts, box.UdevAutomount),
			CommandPreview: "printf 'AUTOMOUNT_OPTS=\"errors=continue,noatime,users\"\\n' | tee /etc/media-automount.d/{f2fs,ext4,auto} && mount -o remount,noatime,errors=continue /media/usb1",
		},
		{
			ID:             "clean_fstab",
			Step:           7,
			Name:           "Clean /etc/fstab (No External SD Card Boot Trap)",
			Status:         makeStatus(step7OK),
			Passed:         step7OK,
			Detail:         fmt.Sprintf("clean_fstab=%v (no /dev/mmcblk0p1 in /etc/fstab)", box.CleanFstab),
			CommandPreview: "sed -i '/mmcblk0p1/d' /etc/fstab && systemctl daemon-reload",
		},
		{
			ID:             "shinobi_hardened",
			Step:           8,
			Name:           "Shinobi Hardened (doSnapshot=false, Worker Fix & Frag MP4 moov)",
			Status:         makeStatus(step8OK),
			Passed:         step8OK,
			Detail:         fmt.Sprintf("doSnapshot=%v, worker_fix=%v, frag_mp4=%v (-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof), corrupted_mp4=%d", box.ShinobiDoSnapshot, box.ShinobiWorkerFix, box.ShinobiFragMP4, box.CorruptedMP4Count),
			CommandPreview: "node /usr/local/bin/shinobi-golden-patch.js --doSnapshot=false --cust-record='-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof' --input-map-choices",
		},
	}

	score := 0
	for _, p := range pillars {
		if p.Passed {
			score++
		}
	}

	return &HardeningReport{
		BoxID:             box.ID,
		BoxName:           box.Name,
		ClubID:            box.ClubID,
		ClubName:          box.ClubName,
		VPNIP:             box.VPNIP,
		Score:             score,
		MaxScore:          8,
		CorruptedMP4Count: box.CorruptedMP4Count,
		Pillars:           pillars,
		AuditedAt:         time.Now().UTC(),
	}
}

var pillarIDToStep = map[string]int{
	"zram_unbounded":   1,
	"pm2_memory_guard": 2,
	"sysrq_disabled":   3,
	"camera_watchdog":  4,
	"storage_guard":    5,
	"udev_automount":   6,
	"clean_fstab":      7,
	"shinobi_hardened": 8,
}

// RemediateBoxHardening safely remediates requested Golden Hardening steps (1..8) without formatting any storage.
func RemediateBoxHardening(box *Box, steps []int, rawCommandCheck string) (*HardeningReport, error) {
	if err := ValidateNoDiskFormat(rawCommandCheck); err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		steps = []int{1, 2, 3, 4, 5, 6, 7, 8}
	}
	stepSet := make(map[int]bool, len(steps))
	for _, st := range steps {
		if st < 1 || st > 8 {
			return nil, fmt.Errorf("invalid hardening step %d (must be 1..8)", st)
		}
		stepSet[st] = true
	}

	if stepSet[1] {
		box.ZRAMMemLimit = 0
		box.Swappiness = 80
		if box.ZRAMTotalMB <= 0 {
			box.ZRAMTotalMB = box.RAMTotalMB
		}
		if box.ZRAMAlgo == "" {
			box.ZRAMAlgo = "lz4"
		}
	}
	if stepSet[2] {
		minCamMB := 120 + box.CamerasCount*8
		if minCamMB < 220 {
			minCamMB = 280
		}
		if box.PM2CameraLimitMB < minCamMB {
			box.PM2CameraLimitMB = minCamMB
		}
		box.PM2InutLimitMB = 280
		box.PM2OtaLimitMB = 120
		box.PM2LogsDevNull = true
	}
	if stepSet[3] {
		box.SysrqEnabled = 0
		box.SysrqHooksRemoved = true
	}
	if stepSet[4] {
		box.CameraWatchdogActive = true
		if box.FFmpegExpected == 0 && box.CamerasCount > 0 {
			box.FFmpegExpected = box.CamerasCount * 2
		}
		if box.FFmpegCount < box.FFmpegExpected {
			box.FFmpegCount = box.FFmpegExpected
		}
	}
	if stepSet[5] {
		box.StorageGuardActive = true
		if box.SDUsedPct >= 90 {
			box.SDUsedPct = 78
			box.SDUsedGB = (box.SDTotalGB * box.SDUsedPct) / 100
		}
	}
	if stepSet[6] {
		box.UdevAutomount = true
		box.SDMountOpts = "rw,noatime,errors=continue,users"
	}
	if stepSet[7] {
		box.CleanFstab = true
	}
	if stepSet[8] {
		box.ShinobiDoSnapshot = false
		box.ShinobiWorkerFix = true
		box.ShinobiFragMP4 = true
		box.CorruptedMP4Count = 0
		for _, mon := range box.Monitors {
			mon.InputMapChoicesOK = true
			mon.DoSnapshotDisabled = true
			mon.FragMP4OK = true
			mon.CustRecord = "-tag:v hvc1 -segment_format_options movflags=frag_keyframe+empty_moov+default_base_moof"
		}
	}

	report := Build8StepHardening(box)
	box.HardeningScore = report.Score
	box.HardeningMax = 8
	box.LastSeen = time.Now().UTC()
	return report, nil
}

func (s *Server) handleHardeningList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	boxes := s.store.Boxes()
	reports := make([]*HardeningReport, 0, len(boxes))
	fullCount := 0
	needsCount := 0
	for _, b := range boxes {
		rep := Build8StepHardening(b)
		reports = append(reports, rep)
		if rep.Score >= 8 {
			fullCount++
		} else {
			needsCount++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"boxes": reports,
		"summary": map[string]any{
			"total_boxes":       len(reports),
			"fully_hardened":    fullCount,
			"needs_remediation": needsCount,
		},
	})
}

func (s *Server) handleBoxHardeningInspect(w http.ResponseWriter, r *http.Request, boxID string) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var report *HardeningReport
	err := s.store.UpdateBox(boxID, func(b *Box) error {
		report = Build8StepHardening(b)
		b.HardeningScore = report.Score
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "box_not_found", "box_id": boxID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"box_id":    boxID,
		"report":    report,
		"score":     report.Score,
		"max_score": report.MaxScore,
		"pillars":   report.Pillars,
	})
}

func (s *Server) handleBoxHardeningRemediate(w http.ResponseWriter, r *http.Request, boxID string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	rawBody, _ := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	rawStr := string(rawBody)

	if err := ValidateNoDiskFormat(rawStr); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "forbidden_disk_format",
			"message": "CẤM TUYỆT ĐỐI lệnh format ổ đĩa/thẻ nhớ (mkfs, fdisk, parted, dd, wipefs). Chỉ sử dụng remount nóng errors=continue,noatime.",
			"detail":  err.Error(),
		})
		return
	}

	var req struct {
		Steps      []int    `json:"steps"`
		Step       int      `json:"step"`
		Pillars    []string `json:"pillars"`
		PillarID   string   `json:"pillar_id"`
		Command    string   `json:"command"`
		RawCommand string   `json:"raw_command"`
		Action     string   `json:"action"`
	}
	if len(rawBody) > 0 {
		_ = json.Unmarshal(rawBody, &req)
	}
	if req.Step > 0 {
		req.Steps = append(req.Steps, req.Step)
	}
	if req.PillarID != "" {
		req.Pillars = append(req.Pillars, req.PillarID)
	}
	for _, pid := range req.Pillars {
		if st, ok := pillarIDToStep[strings.ToLower(strings.TrimSpace(pid))]; ok {
			req.Steps = append(req.Steps, st)
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":  "invalid_pillar",
				"pillar": pid,
			})
			return
		}
	}

	checkCmd := strings.Join([]string{req.Command, req.RawCommand, req.Action}, " ")
	var report *HardeningReport
	var remediateErr error

	err := s.store.UpdateBox(boxID, func(b *Box) error {
		report, remediateErr = RemediateBoxHardening(b, req.Steps, checkCmd)
		return remediateErr
	})
	if remediateErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "remediation_rejected",
			"message": remediateErr.Error(),
		})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":  "box_not_found",
			"box_id": boxID,
		})
		return
	}

	updatedBox, _ := s.store.GetBox(boxID)
	s.broadcastSSE("fleet_update", map[string]any{
		"type":            "hardening_remediated",
		"box_id":          boxID,
		"hardening_score": report.Score,
		"kpi":             s.store.ComputeKPI(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"box_id":    boxID,
		"score":     report.Score,
		"max_score": report.MaxScore,
		"pillars":   report.Pillars,
		"report":    report,
		"box":       updatedBox,
	})
}

func (s *Server) handleCamerasList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	boxFilter := strings.TrimSpace(r.URL.Query().Get("box_id"))
	clubFilter := strings.TrimSpace(r.URL.Query().Get("club_id"))
	vendorFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("vendor")))

	allCams, _ := s.store.AllCameras()
	filtered := make([]*Camera, 0, len(allCams))
	onlineCount := 0
	recCount := 0
	h265Count := 0
	doubleCount := 0

	for _, cam := range allCams {
		if boxFilter != "" && cam.BoxID != boxFilter {
			continue
		}
		if clubFilter != "" && cam.ClubID != clubFilter {
			continue
		}
		if vendorFilter != "" && strings.ToLower(cam.Vendor) != vendorFilter {
			continue
		}
		filtered = append(filtered, cam)
		if cam.Online {
			onlineCount++
		}
		if cam.Recording {
			recCount++
		}
		if strings.EqualFold(cam.Codec, "H.265") && cam.BitrateKbps <= 1536 {
			h265Count++
		}
		if cam.DoubleStreaming {
			doubleCount++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cameras":             filtered,
		"double_stream_count": doubleCount,
		"summary": map[string]any{
			"total":            len(filtered),
			"online":           onlineCount,
			"recording":        recCount,
			"h265_compliant":   h265Count,
			"double_streaming": doubleCount,
		},
	})
}

func (s *Server) handleCamerasBulkApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}

	var req struct {
		BoxID       string   `json:"box_id"`
		ClubID      string   `json:"club_id"`
		CameraIDs   []string `json:"camera_ids"`
		Codec       string   `json:"codec"`
		FPS         int      `json:"fps"`
		BitrateKbps int      `json:"bitrate_kbps"`
		RCMode      string   `json:"rc_mode"`
		GOP         int      `json:"gop"`
		AudioCodec  string   `json:"audio_codec"`
		Profile     struct {
			Codec               string `json:"codec"`
			FPS                 int    `json:"fps"`
			BitrateKbps         int    `json:"bitrate_kbps"`
			RCMode              string `json:"rc_mode"`
			GOP                 int    `json:"gop"`
			AudioCodec          string `json:"audio_codec"`
			ResolveDoubleStream *bool  `json:"resolve_double_stream"`
		} `json:"profile"`
	}
	if r.Body != nil {
		rawBody, _ := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if len(strings.TrimSpace(string(rawBody))) > 0 {
			if err := json.Unmarshal(rawBody, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error":   "invalid_json",
					"message": err.Error(),
				})
				return
			}
		}
	}

	// Enforce Golden Bida H.265 Standard (aligned with Anti-A Guardian: H.265, 30 FPS, 1536 Kbps CBR, GOP 50, AAC)
	codec := "H.265"
	fps := 30
	if req.Profile.FPS > 0 && req.Profile.FPS <= 60 {
		fps = req.Profile.FPS
	} else if req.FPS > 0 && req.FPS <= 60 {
		fps = req.FPS
	}
	bitrate := 1536
	if req.Profile.BitrateKbps > 0 && req.Profile.BitrateKbps <= 2048 {
		bitrate = req.Profile.BitrateKbps
	} else if req.BitrateKbps > 0 && req.BitrateKbps <= 2048 {
		bitrate = req.BitrateKbps
	}
	rcMode := "CBR"
	if req.Profile.RCMode != "" {
		rcMode = strings.ToUpper(req.Profile.RCMode)
	} else if req.RCMode != "" {
		rcMode = strings.ToUpper(req.RCMode)
	}
	gop := 50
	if req.Profile.GOP > 0 {
		gop = req.Profile.GOP
	} else if req.GOP > 0 {
		gop = req.GOP
	}
	audioCodec := "AAC"
	if req.Profile.AudioCodec != "" {
		audioCodec = strings.ToUpper(req.Profile.AudioCodec)
	} else if req.AudioCodec != "" {
		audioCodec = strings.ToUpper(req.AudioCodec)
	}
	resolveDouble := true
	if req.Profile.ResolveDoubleStream != nil {
		resolveDouble = *req.Profile.ResolveDoubleStream
	}

	camIDSet := make(map[string]bool, len(req.CameraIDs))
	for _, id := range req.CameraIDs {
		camIDSet[strings.TrimSpace(id)] = true
	}

	now := time.Now().UTC()
	var updatedCams []*Camera
	boxes := s.store.Boxes()

	for _, b := range boxes {
		if req.BoxID != "" && b.ID != req.BoxID {
			continue
		}
		if req.ClubID != "" && b.ClubID != req.ClubID {
			continue
		}
		boxChanged := false
		_ = s.store.UpdateBox(b.ID, func(box *Box) error {
			for _, cam := range box.Cameras {
				if len(camIDSet) > 0 && !camIDSet[cam.ID] && !camIDSet[cam.Name] {
					continue
				}
				cam.Codec = codec
				cam.SmartCodec = true
				cam.FPS = fps
				cam.BitrateKbps = bitrate
				cam.RCMode = rcMode
				cam.GOP = gop
				cam.AudioCodec = audioCodec
				if resolveDouble && cam.DoubleStreaming {
					cam.DoubleStreaming = false
					if cam.NVRHost != "" {
						cam.StreamSource = "nvr"
					} else {
						cam.StreamSource = "direct"
					}
				}
				cam.LastProbedAt = now
				cp := *cam
				updatedCams = append(updatedCams, &cp)
				boxChanged = true
			}
			if boxChanged {
				box.RXMbps = float64(int(float64(len(box.Cameras))*float64(bitrate)/1000.0*0.65*10)) / 10.0
				box.LastSeen = now
			}
			return nil
		})
	}

	_, remainingDouble := s.store.AllCameras()
	s.broadcastSSE("fleet_update", map[string]any{
		"type":                "cameras_bulk_applied",
		"updated_count":       len(updatedCams),
		"double_stream_count": remainingDouble,
		"kpi":                 s.store.ComputeKPI(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"updated_count":       len(updatedCams),
		"double_stream_count": remainingDouble,
		"profile": map[string]any{
			"codec":        codec,
			"smart_codec":  true,
			"fps":          fps,
			"bitrate_kbps": bitrate,
			"rc_mode":      rcMode,
			"gop":          gop,
			"audio_codec":  audioCodec,
		},
		"cameras": updatedCams,
	})
}
