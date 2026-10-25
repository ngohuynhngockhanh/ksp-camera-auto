package bidahub

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TIER 1: FEATURE COVERAGE TESTS (F3, F4 Hardening, Cameras, RedBida) — 10 TESTS
// =============================================================================

// Test 1 (F3): GET /fleet/api/hardening returns all boxes with max_score: 8 and all 8 Golden Hardening pillars.
func TestTier1_F3_HardeningMatrixReturnsAll8Pillars(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/hardening", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/hardening, got %d: %s", rec.Code, rec.Body.String())
	}

	rawBoxes, ok := data["boxes"].([]any)
	if !ok || len(rawBoxes) < 30 {
		if alt, ok2 := data["reports"].([]any); ok2 && len(alt) >= 30 {
			rawBoxes = alt
		} else {
			t.Fatalf("expected >= 30 boxes/reports in /fleet/api/hardening, got %d", len(rawBoxes))
		}
	}

	expectedPillarIDs := []string{
		"zram_unbounded",
		"pm2_memory_guard",
		"sysrq_disabled",
		"camera_watchdog",
		"storage_guard",
		"udev_automount",
		"clean_fstab",
		"shinobi_hardened",
	}

	first, _ := rawBoxes[0].(map[string]any)
	if int(first["max_score"].(float64)) != 8 {
		t.Errorf("expected max_score=8, got %v", first["max_score"])
	}
	pillars, ok := first["pillars"].([]any)
	if !ok || len(pillars) != 8 {
		t.Fatalf("expected 8 pillars per box, got %d", len(pillars))
	}
	for i, wantID := range expectedPillarIDs {
		pm, _ := pillars[i].(map[string]any)
		if gotID := fmt.Sprint(pm["id"]); gotID != wantID {
			t.Errorf("pillar[%d] expected id=%q, got %q", i, wantID, gotID)
		}
		if int(pm["step"].(float64)) != i+1 {
			t.Errorf("pillar[%d] expected step=%d, got %v", i, i+1, pm["step"])
		}
	}
}

// Test 2 (F3): POST /fleet/api/boxes/{box_id}/hardening/inspect re-evaluates all 8 pillars for a box.
func TestTier1_F3_HardeningInspectSingleBox(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/inspect", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from hardening/inspect, got %d: %s", rec.Code, rec.Body.String())
	}

	report := data
	if nested, ok := data["hardening"].(map[string]any); ok {
		report = nested
	} else if nested, ok := data["report"].(map[string]any); ok {
		report = nested
	}
	if int(report["max_score"].(float64)) != 8 {
		t.Errorf("expected max_score=8 on inspect report, got %v", report["max_score"])
	}
	pillars, _ := report["pillars"].([]any)
	if len(pillars) != 8 {
		t.Errorf("expected 8 pillars in inspect report, got %d", len(pillars))
	}
}

// Test 3 (F3): Safe 1-click executor (POST /fleet/api/boxes/{box_id}/hardening/remediate) brings degraded box to 8/8.
func TestTier1_F3_HardeningRemediateUpdatesScoreTo8Of8(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Degrade box hardening attributes first via store
	storeMutateBox(t, srv, "inut_204_167", func(b *Box) {
		b.ZRAMMemLimit = 524288000
		b.Swappiness = 0
		b.SysrqEnabled = 1
		b.CleanFstab = false
		b.ShinobiDoSnapshot = true
		b.ShinobiFragMP4 = false
	})
	box := storeGetBox(srv, "inut_204_167")
	// Inspect degraded box
	degReport := Build8StepHardening(box)
	if degReport.Score >= 8 {
		t.Fatalf("expected degraded box score < 8 before remediation, got %d", degReport.Score)
	}

	// Execute 1-click safe remediation for steps 1..8
	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_167/hardening/remediate", map[string]any{
		"steps": []int{1, 2, 3, 4, 5, 6, 7, 8},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from hardening/remediate, got %d: %s", rec.Code, rec.Body.String())
	}

	report := data
	if nested, ok := data["hardening"].(map[string]any); ok {
		report = nested
	} else if nested, ok := data["report"].(map[string]any); ok {
		report = nested
	}
	if int(report["score"].(float64)) != 8 {
		t.Errorf("expected score=8 after remediating steps 1..8, got %v", report["score"])
	}
}

// Test 4 (F3): Pillar 8 (shinobi_hardened) enforces doSnapshot: false, worker leak fix, and Fragmented MP4 cust_record.
func TestTier1_F3_HardeningPillar8ShinobiFragmentedMP4AndSnapshotGuard(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_204_141")
	if box == nil {
		t.Fatalf("missing box inut_204_141")
	}
	report, err := RemediateBoxHardening(box, []int{8}, "")
	if err != nil {
		t.Fatalf("RemediateBoxHardening step 8 failed: %v", err)
	}
	var pillar8 *HardeningPillar
	for i := range report.Pillars {
		if report.Pillars[i].ID == "shinobi_hardened" || report.Pillars[i].Step == 8 {
			pillar8 = &report.Pillars[i]
			break
		}
	}
	if pillar8 == nil {
		t.Fatalf("pillar 8 shinobi_hardened not found")
	}
	if !pillar8.Passed || pillar8.Status != "pass" {
		t.Errorf("expected pillar 8 to pass after remediation, got status=%q passed=%v", pillar8.Status, pillar8.Passed)
	}
	combined := pillar8.Detail + " " + pillar8.CommandPreview
	if !strings.Contains(combined, "frag_keyframe") || !strings.Contains(combined, "empty_moov") {
		t.Errorf("expected pillar 8 detail/command to reference frag_keyframe+empty_moov, got: %s", combined)
	}
}

// Test 5 (F3): Pillars 6 & 7 enforce udev automount (errors=continue,noatime) and clean /etc/fstab (no /dev/mmcblk0p1).
func TestTier1_F3_HardeningPillar6And7UdevAutomountWithoutFstab(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_205_123")
	if box == nil {
		t.Fatalf("missing box inut_205_123")
	}
	report, err := RemediateBoxHardening(box, []int{6, 7}, "")
	if err != nil {
		t.Fatalf("RemediateBoxHardening steps 6,7 failed: %v", err)
	}
	for _, p := range report.Pillars {
		if p.Step == 6 || p.ID == "udev_automount" {
			if !p.Passed {
				t.Errorf("expected step 6 udev_automount passed")
			}
			if !strings.Contains(p.Detail+p.CommandPreview, "errors=continue") && !strings.Contains(p.Detail+p.CommandPreview, "noatime") {
				t.Errorf("expected step 6 to mention errors=continue/noatime, got %s", p.Detail)
			}
		}
		if p.Step == 7 || p.ID == "clean_fstab" {
			if !p.Passed {
				t.Errorf("expected step 7 clean_fstab passed")
			}
		}
	}
}

// Test 6 (F4): Multi-box Camera inventory uses compound key (box_id:host:port) and reports double_stream_count.
func TestTier1_F4_CameraInventoryCompoundKeyAndDoubleStreamingDetection(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/cameras", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/cameras, got %d: %s", rec.Code, rec.Body.String())
	}

	rawCams, ok := data["cameras"].([]any)
	if !ok || len(rawCams) == 0 {
		t.Fatalf("expected non-empty cameras array in /fleet/api/cameras")
	}
	if _, hasDbl := data["double_stream_count"]; !hasDbl {
		t.Errorf("expected double_stream_count field in /fleet/api/cameras response")
	}
	if _, hasSum := data["summary"]; !hasSum {
		t.Errorf("expected summary object in /fleet/api/cameras response")
	}

	seenIDs := make(map[string]bool, len(rawCams))
	for _, item := range rawCams {
		cm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := fmt.Sprint(cm["id"])
		boxID := fmt.Sprint(cm["box_id"])
		if !strings.HasPrefix(id, boxID+":") {
			t.Errorf("camera id %q does not start with compound prefix %q", id, boxID+":")
		}
		if seenIDs[id] {
			t.Errorf("duplicate compound camera ID %q found", id)
		}
		seenIDs[id] = true
	}
}

// Test 7 (F4): Sequential Bulk Profile Apply enforces Golden Bida H.265 Standard (H.265, 30 FPS, 1536 Kbps CBR, GOP 50, AAC).
func TestTier1_F4_CameraBulkApplyGoldenH265Standard(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", map[string]any{
		"box_id":       "inut_207_44",
		"codec":        "H.265",
		"fps":          30,
		"bitrate_kbps": 1536,
		"rc_mode":      "CBR",
		"gop":          50,
		"audio_codec":  "AAC",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/cameras/bulk-apply, got %d: %s", rec.Code, rec.Body.String())
	}
	if data["ok"] == false {
		t.Errorf("expected bulk-apply to succeed, got %v", data)
	}

	// Verify cameras on inut_207_44 now have H.265 / 30fps / 1536kbps / CBR / GOP 50 / AAC
	recCams, camsData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/cameras?box_id=inut_207_44", nil, nil)
	if recCams.Code != http.StatusOK {
		t.Fatalf("expected 200 from filtered /fleet/api/cameras, got %d", recCams.Code)
	}
	cams, _ := camsData["cameras"].([]any)
	for _, item := range cams {
		cm, _ := item.(map[string]any)
		if fmt.Sprint(cm["box_id"]) != "inut_207_44" {
			continue
		}
		if fmt.Sprint(cm["codec"]) != "H.265" {
			t.Errorf("expected camera %v codec=H.265 after bulk apply, got %v", cm["id"], cm["codec"])
		}
		if int(cm["fps"].(float64)) != 30 {
			t.Errorf("expected camera %v fps=30, got %v", cm["id"], cm["fps"])
		}
		if int(cm["bitrate_kbps"].(float64)) != 1536 {
			t.Errorf("expected camera %v bitrate_kbps=1536, got %v", cm["id"], cm["bitrate_kbps"])
		}
		if fmt.Sprint(cm["audio_codec"]) != "AAC" {
			t.Errorf("expected camera %v audio_codec=AAC, got %v", cm["id"], cm["audio_codec"])
		}
	}
}

// Test 8 (F4): RedBida 17-Rule Inspector returns score/17, 17 rules, and editable keys per box.
func TestTier1_F4_RedBida17RuleInspectorAndCatalog(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/redbida", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /fleet/api/redbida, got %d: %s", rec.Code, rec.Body.String())
	}

	rawBoxes, ok := data["boxes"].([]any)
	if !ok || len(rawBoxes) == 0 {
		if alt, ok2 := data["reports"].([]any); ok2 && len(alt) > 0 {
			rawBoxes = alt
		} else {
			t.Fatalf("expected non-empty boxes/reports in /fleet/api/redbida")
		}
	}

	first, _ := rawBoxes[0].(map[string]any)
	if int(first["max_score"].(float64)) != 17 {
		t.Errorf("expected RedBida max_score=17, got %v", first["max_score"])
	}
	rules, _ := first["rules"].([]any)
	if len(rules) != 17 {
		t.Errorf("expected 17 rules in RedBida report, got %d", len(rules))
	}
	tabs, _ := first["tabs"].(map[string]any)
	if len(tabs) < 20 {
		t.Errorf("expected 20 tab keys ([C01]..[C20]) in RedBida report, got %d", len(tabs))
	}
}

// Test 9 (F4): RedBida preserves "pickleball" branding for Pickleball venues and "billiards" for Billiards clubs.
func TestTier1_F4_RedBidaPreservesPickleballVsBilliardsBranding(t *testing.T) {
	srv, _ := newTestHubServer(t)

	pickleballBoxes := []string{"inut_205_73", "inut_204_157", "inut_205_87", "inut_205_120", "inut_204_165"}
	for _, boxID := range pickleballBoxes {
		box := storeGetBox(srv, boxID)
		if box == nil {
			t.Fatalf("missing pickleball box %s", boxID)
		}
		if box.Branding != "pickleball" {
			t.Errorf("expected box %s (%s) to have branding=pickleball, got %q", boxID, box.ClubName, box.Branding)
		}
	}

	billiardBox := storeGetBox(srv, "inut_204_141")
	if billiardBox == nil || billiardBox.Branding != "billiards" {
		t.Errorf("expected inut_204_141 to have branding=billiards, got %v", billiardBox)
	}
}

// Test 10 (F4): POST /fleet/api/boxes/{box_id}/redbida/apply updates shinobi_group_key, ggcode, virtual IPs, frpc_config, and [C01]..[C20].
func TestTier1_F4_RedBidaApplyUpdatesEditableKeysAndGroupKey(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_38/redbida/apply", map[string]any{
		"auto_fix":          true,
		"shinobi_group_key": "ux0pAUigSn_v2",
		"ggcode":            "GG-TOPTN-01",
		"eth0_virtual_ip":   "192.168.1.133",
		"end0_virtual_ip":   "192.168.1.133",
		"store_domain":      "store-toptn1.video.io.vn",
		"cstore_domain":     "c-store-toptn1.video.io.vn",
		"tabs": map[string]string{
			"[C01]": "Bàn 01 - VIP",
			"[C20]": "Bàn 20 - Thi Đấu",
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from redbida/apply, got %d: %s", rec.Code, rec.Body.String())
	}

	report := data
	if nested, ok := data["redbida"].(map[string]any); ok {
		report = nested
	} else if nested, ok := data["report"].(map[string]any); ok {
		report = nested
	}
	if int(report["score"].(float64)) != 17 {
		t.Errorf("expected score=17 after RedBida auto_fix apply, got %v", report["score"])
	}
	if fmt.Sprint(report["shinobi_group_key"]) != "ux0pAUigSn_v2" {
		t.Errorf("expected shinobi_group_key=ux0pAUigSn_v2, got %v", report["shinobi_group_key"])
	}
	if fmt.Sprint(report["ggcode"]) != "GG-TOPTN-01" {
		t.Errorf("expected ggcode=GG-TOPTN-01, got %v", report["ggcode"])
	}
}

// =============================================================================
// TIER 2: BOUNDARY & CORNER CASES (F3, F4) — 12 TESTS
// =============================================================================

// Test 11 (F3 Safety): Hardening remediator strictly rejects mkfs disk formatting commands.
func TestTier2_F3_HardeningRemediateRejectsMkfsCommand(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{
		"steps":   []int{6},
		"command": "mkfs.f2fs -f /dev/mmcblk0p1",
	}, nil)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusForbidden {
		t.Errorf("expected 400/422/403 when attempting mkfs.f2fs via remediate API, got %d: %s", rec.Code, rec.Body.String())
	}

	box := storeGetBox(srv, "inut_204_141")
	if _, err := RemediateBoxHardening(box, []int{6}, "mkfs.ext4 /dev/mmcblk0p1"); err == nil {
		t.Errorf("expected RemediateBoxHardening to return error for mkfs.ext4")
	}
}

// Test 12 (F3 Safety): Hardening remediator rejects fdisk, parted, dd, wipefs, and format commands.
func TestTier2_F3_HardeningRemediateRejectsFdiskPartedDdAndFormat(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_204_141")

	forbiddenCommands := []string{
		"fdisk /dev/mmcblk0",
		"parted -s /dev/sda mklabel gpt",
		"dd if=/dev/zero of=/dev/mmcblk0p1 bs=1M",
		"wipefs -a /dev/mmcblk0p1",
		"format /dev/sda1",
	}
	for _, cmd := range forbiddenCommands {
		if _, err := RemediateBoxHardening(box, []int{6}, cmd); err == nil {
			t.Errorf("expected RemediateBoxHardening to reject forbidden command %q", cmd)
		}
		rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{
			"steps":   []int{6},
			"command": cmd,
		}, nil)
		if rec.Code == http.StatusOK {
			t.Errorf("expected HTTP error rejecting forbidden command %q, got 200 OK", cmd)
		}
	}
}

// Test 13 (F3): Hardening remediate rejects out-of-range step numbers (0, 9, -1) or empty steps.
func TestTier2_F3_HardeningRemediateRejectsOutOfRangeStepNumbers(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{
		"steps": []int{0, 9, -1},
	}, nil)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400/422 for out-of-range hardening steps [0, 9, -1], got %d", rec.Code)
	}
}

// Test 14 (F3): Hardening inspect and remediate for unknown box_id return 404 Not Found.
func TestTier2_F3_HardeningInspectAndRemediateUnknownBoxReturns404(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recInsp, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_nonexistent/hardening/inspect", nil, nil)
	if recInsp.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown box hardening inspect, got %d", recInsp.Code)
	}

	recRem, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_nonexistent/hardening/remediate", map[string]any{
		"steps": []int{1},
	}, nil)
	if recRem.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown box hardening remediate, got %d", recRem.Code)
	}
}

// Test 15 (F3 Safety): Every generated command_preview across all 8 pillars and all 30+ boxes is free of disk formatting commands.
func TestTier2_F3_HardeningCommandPreviewsNeverContainDiskFormat(t *testing.T) {
	srv, _ := newTestHubServer(t)
	for _, box := range storeListBoxes(srv) {
		rep := Build8StepHardening(box)
		for _, p := range rep.Pillars {
			lower := strings.ToLower(p.CommandPreview)
			for _, bad := range []string{"mkfs", "fdisk", "parted", "wipefs", "dd if="} {
				if strings.Contains(lower, bad) {
					t.Fatalf("box %s pillar %s command_preview contains forbidden disk format token %q: %s",
						box.ID, p.ID, bad, p.CommandPreview)
				}
			}
		}
	}
}

// Test 16 (F4): RedBida FRPC Subdomain Auditor detects duplicate store-* / c-store-* domains across boxes.
func TestTier2_F4_RedBidaDetectsFRPCSubdomainConflict(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Force two boxes to share the same store_domain ("store-lionking-2.video.io.vn")
	storeMutateBox(t, srv, "inut_205_85", func(b *Box) {
		b.StoreDomain = "store-lionking-2.video.io.vn"
	})
	storeMutateBox(t, srv, "inut_205_73", func(b *Box) {
		b.StoreDomain = "store-lionking-2.video.io.vn"
	})
	b2 := storeGetBox(srv, "inut_205_73")
	rep := BuildRedBidaReport(b2, storeListBoxes(srv))
	if !rep.DomainConflict {
		t.Errorf("expected BuildRedBidaReport to flag DomainConflict=true when two boxes share store-lionking-2.video.io.vn")
	}

	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/redbida", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /fleet/api/redbida, got %d", rec.Code)
	}
	conflicts, _ := data["conflicts"].([]any)
	if len(conflicts) == 0 {
		t.Errorf("expected non-empty conflicts array in /fleet/api/redbida when two boxes share a subdomain")
	}
}

// Test 17 (F4): RedBida auto_fix on Pickleball venue (Pika Sun inut_205_73) never overwrites Pickleball branding with Billiards.
func TestTier2_F4_RedBidaRejectsBrandingOverwriteForPickleballVenue(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_205_73/redbida/apply", map[string]any{
		"auto_fix": true,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from redbida/apply on inut_205_73, got %d", rec.Code)
	}
	report := data
	if nested, ok := data["redbida"].(map[string]any); ok {
		report = nested
	} else if nested, ok := data["report"].(map[string]any); ok {
		report = nested
	}
	if fmt.Sprint(report["branding"]) != "pickleball" {
		t.Errorf("expected Pickleball venue inut_205_73 to retain branding=pickleball after auto_fix, got %v", report["branding"])
	}
}

// Test 18 (F4): RedBida apply on unknown box returns 404 Not Found.
func TestTier2_F4_RedBidaApplyUnknownBoxReturns404(t *testing.T) {
	srv, _ := newTestHubServer(t)
	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_missing_999/redbida/apply", map[string]any{
		"auto_fix": true,
	}, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown box redbida/apply, got %d", rec.Code)
	}
}

// Test 19 (F4): Camera bulk-apply rejects negative/corrupt bitrate or normalizes to Golden H.265 standard.
func TestTier2_F4_CameraBulkApplyRejectsNonGoldenH264OrExcessiveBitrate(t *testing.T) {
	srv, _ := newTestHubServer(t)

	recBadJSON, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", "{invalid-json", nil)
	if recBadJSON.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON to /fleet/api/cameras/bulk-apply, got %d", recBadJSON.Code)
	}

	recNeg, data := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", map[string]any{
		"box_id":       "inut_204_141",
		"bitrate_kbps": -9999,
		"fps":          -10,
	}, nil)
	// Either rejects with 400/422 or clamps/enforces Golden Bida Standard (1536 kbps, 30 fps, H.265)
	if recNeg.Code == http.StatusOK {
		if profile, ok := data["profile"].(map[string]any); ok {
			if int(profile["bitrate_kbps"].(float64)) <= 0 {
				t.Errorf("expected positive Golden bitrate when clamping, got %v", profile["bitrate_kbps"])
			}
		}
	} else if recNeg.Code != http.StatusBadRequest && recNeg.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 400/422 or clamped 200 for negative bitrate, got %d", recNeg.Code)
	}
}

// Test 20 (F4): Camera inventory filter by box_id and vendor returns only matching cameras.
func TestTier2_F4_CameraInventoryFilterByBoxAndVendor(t *testing.T) {
	srv, _ := newTestHubServer(t)

	rec, data := doHubRequest(t, srv, http.MethodGet, "/fleet/api/cameras?box_id=inut_204_148&vendor=Hikvision", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from filtered /fleet/api/cameras, got %d", rec.Code)
	}
	cams, _ := data["cameras"].([]any)
	for _, item := range cams {
		cm, _ := item.(map[string]any)
		if fmt.Sprint(cm["box_id"]) != "inut_204_148" {
			t.Errorf("expected only cameras from box inut_204_148, got %v", cm["box_id"])
		}
		if !strings.EqualFold(fmt.Sprint(cm["vendor"]), "Hikvision") {
			t.Errorf("expected only Hikvision cameras for inut_204_148, got %v", cm["vendor"])
		}
	}
}

// Test 21 (F3): Monitor configuration on boxes satisfies Shinobi Golden SOP (input_map_choices, frag_keyframe+empty_moov, doSnapshot=false).
func TestTier2_F3_KspcamGoldenSOP_ShinobiSyncBuildMonitorConfig(t *testing.T) {
	srv, _ := newTestHubServer(t)
	box := storeGetBox(srv, "inut_204_141")
	if box == nil {
		t.Fatalf("missing box inut_204_141")
	}
	_, err := RemediateBoxHardening(box, []int{8}, "")
	if err != nil {
		t.Fatalf("RemediateBoxHardening step 8 failed: %v", err)
	}
	for _, mon := range box.Monitors {
		if !mon.InputMapChoicesOK {
			t.Errorf("monitor %s expected InputMapChoicesOK=true", mon.MID)
		}
		if !mon.DoSnapshotDisabled {
			t.Errorf("monitor %s expected DoSnapshotDisabled=true", mon.MID)
		}
		if !strings.Contains(mon.CustRecord, "-tag:v hvc1") || !strings.Contains(mon.CustRecord, "frag_keyframe+empty_moov+default_base_moof") {
			t.Errorf("monitor %s expected fragmented MP4 cust_record, got %q", mon.MID, mon.CustRecord)
		}
	}
}

// Test 22 (F4 Adversarial): Unicode, quotes, and shell meta-characters in RedBida keys are stored safely without corruption.
func TestTier2_F4_AdversarialUnicodeAndInjectionInRedBidaKeys(t *testing.T) {
	srv, _ := newTestHubServer(t)
	adversarialTitle := `CLB Bida Đỉnh Cao 🎱 "VIP" & <Tag> $(whoami) ; rm -rf /`

	rec, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/redbida/apply", map[string]any{
		"keys": map[string]string{
			"ui_title": adversarialTitle,
		},
		"tabs": map[string]string{
			"[C01]": "Bàn 01 — Đặc Biệt 🎱",
		},
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK saving Unicode RedBida keys, got %d: %s", rec.Code, rec.Body.String())
	}

	box := storeGetBox(srv, "inut_204_141")
	if got := box.RedBidaKeys["ui_title"]; got != adversarialTitle {
		t.Errorf("expected exact round-trip of adversarial Unicode string, got %q", got)
	}
}

// =============================================================================
// TIER 3: CROSS-FEATURE PAIRWISE TESTS — 2 TESTS
// =============================================================================

// Test 23 (Pairwise F3 x F2): Remediating 8-Step Hardening updates Fleet Summary KPI hardening_full and broadcasts SSE.
func TestTier3_Pairwise_HardeningRemediateUpdatesSummaryKPIAndSSE(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Degrade inut_204_141 first
	storeMutateBox(t, srv, "inut_204_141", func(b *Box) {
		b.SysrqEnabled = 1
		b.HardeningScore = 6
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/fleet/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer resp.Body.Close()

	// Remediate all 8 steps via HTTP
	recRem, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_204_141/hardening/remediate", map[string]any{
		"steps": []int{1, 2, 3, 4, 5, 6, 7, 8},
	}, nil)
	if recRem.Code != http.StatusOK {
		t.Fatalf("remediate failed: %d", recRem.Code)
	}

	// Check summary KPI updated
	_, sumData := doHubRequest(t, srv, http.MethodGet, "/fleet/api/summary", nil, nil)
	kpi, _ := sumData["kpi"].(map[string]any)
	if int(kpi["hardening_full"].(float64)) < 1 {
		t.Errorf("expected kpi.hardening_full >= 1 after remediation, got %v", kpi["hardening_full"])
	}

	// Verify SSE event received
	reader := bufio.NewReader(resp.Body)
	gotEvent := false
	for range 25 {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "event:") {
			gotEvent = true
			break
		}
	}
	if !gotEvent {
		t.Errorf("expected SSE event after hardening remediation")
	}
}

// Test 24 (Pairwise F4 Cameras x Box Detail): Bulk Golden H.265 Apply clears Double-Streaming and updates Box Detail.
func TestTier3_Pairwise_CameraBulkApplyClearsDoubleStreamingAndUpdatesBoxDetail(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Mark a camera on inut_205_31 (Khét Billiards) as double-streaming prior to bulk apply
	storeMutateBox(t, srv, "inut_205_31", func(b *Box) {
		if len(b.Cameras) > 0 {
			b.Cameras[0].DoubleStreaming = true
			b.Cameras[0].Codec = "H.264"
			b.Cameras[0].BitrateKbps = 2048
		}
	})

	recApply, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", map[string]any{
		"box_id":               "inut_205_31",
		"codec":                "H.265",
		"fps":                  30,
		"bitrate_kbps":         1536,
		"rc_mode":              "CBR",
		"gop":                  50,
		"audio_codec":          "AAC",
		"fix_double_streaming": true,
	}, nil)
	if recApply.Code != http.StatusOK {
		t.Fatalf("bulk-apply failed: %d: %s", recApply.Code, recApply.Body.String())
	}

	recDetail, detail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_205_31", nil, nil)
	if recDetail.Code != http.StatusOK {
		t.Fatalf("box detail failed: %d", recDetail.Code)
	}
	cams, _ := detail["cameras"].([]any)
	for _, item := range cams {
		cm, _ := item.(map[string]any)
		if cm["double_streaming"] == true {
			t.Errorf("expected double_streaming=false after bulk apply on inut_205_31, got true for %v", cm["id"])
		}
		if fmt.Sprint(cm["codec"]) != "H.265" {
			t.Errorf("expected codec=H.265 in box detail after bulk apply, got %v", cm["codec"])
		}
	}
}

// =============================================================================
// TIER 4: REAL-WORLD APPLICATION SCENARIOS — 2 TESTS
// =============================================================================

// Test 25 (Scenario): MIAPool Arena (inut_207_44) Double-Streaming & 100Mbps Fast Ethernet Congestion Rescue.
func TestTier4_Scenario_MIAPoolBandwidthAndDoubleStreamingRescue(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Simulate pre-rescue state on inut_207_44 from FLEET_WATCHLIST.md:
	// 13 cameras at 2048 Kbps causing 35 Mbps RX congestion + 4 cameras double-streaming + sysrq=1
	storeMutateBox(t, srv, "inut_207_44", func(b *Box) {
		b.RXMbps = 34.8
		b.SysrqEnabled = 1
		b.ZRAMMemLimit = 500000000
		for i, cam := range b.Cameras {
			cam.BitrateKbps = 2048
			if i < 4 {
				cam.DoubleStreaming = true
			}
		}
	})

	// Step 1: Run Bulk Golden H.265 Apply (30 FPS, 1536 Kbps CBR, H.265, AAC, route via NVR)
	recBulk, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/cameras/bulk-apply", map[string]any{
		"box_id":               "inut_207_44",
		"codec":                "H.265",
		"fps":                  30,
		"bitrate_kbps":         1536,
		"rc_mode":              "CBR",
		"gop":                  50,
		"audio_codec":          "AAC",
		"fix_double_streaming": true,
	}, nil)
	if recBulk.Code != http.StatusOK {
		t.Fatalf("MIAPool bulk camera apply failed: %d", recBulk.Code)
	}

	// Step 2: Run 8-Step Golden Hardening Remediation
	recHard, _ := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_207_44/hardening/remediate", map[string]any{
		"steps": []int{1, 2, 3, 4, 5, 6, 7, 8},
	}, nil)
	if recHard.Code != http.StatusOK {
		t.Fatalf("MIAPool hardening remediate failed: %d", recHard.Code)
	}

	// Step 3: Verify 0 double-streaming cameras and 8/8 hardening score
	_, detail := doHubRequest(t, srv, http.MethodGet, "/fleet/api/boxes/inut_207_44", nil, nil)
	boxMap, _ := detail["box"].(map[string]any)
	if int(boxMap["hardening_score"].(float64)) != 8 {
		t.Errorf("expected MIAPool hardening_score=8, got %v", boxMap["hardening_score"])
	}
	box := storeGetBox(srv, "inut_207_44")
	for _, cam := range box.Cameras {
		if cam.DoubleStreaming {
			t.Errorf("expected MIAPool camera %s DoubleStreaming=false after rescue", cam.ID)
		}
		if cam.BitrateKbps != 1536 || cam.Codec != "H.265" {
			t.Errorf("expected MIAPool camera %s H.265 @ 1536 Kbps, got %s @ %d", cam.ID, cam.Codec, cam.BitrateKbps)
		}
	}
}

// Test 26 (Scenario): Pika Sun Pickleball (inut_205_73) FRPC Subdomain Conflict Resolution & Pickleball Branding Preservation.
func TestTier4_Scenario_PikaSunPickleballFRPCConflictAndBrandingFix(t *testing.T) {
	srv, _ := newTestHubServer(t)

	// Simulate initial misconfiguration from FLEET_WATCHLIST.md:
	// inut_205_73 was accidentally given store-lionking-2 conflicting with inut_205_85
	storeMutateBox(t, srv, "inut_205_85", func(b *Box) {
		b.StoreDomain = "store-lionking-2.video.io.vn"
	})
	storeMutateBox(t, srv, "inut_205_73", func(b *Box) {
		b.StoreDomain = "store-lionking-2.video.io.vn"
	})
	bPika := storeGetBox(srv, "inut_205_73")
	repBefore := BuildRedBidaReport(bPika, storeListBoxes(srv))
	if !repBefore.DomainConflict {
		t.Fatalf("expected initial FRPC domain conflict on inut_205_73")
	}

	// Apply fix: change inut_205_73 domains to store-pika.video.io.vn & c-store-pika.video.io.vn and run auto_fix
	recFix, fixData := doHubRequest(t, srv, http.MethodPost, "/fleet/api/boxes/inut_205_73/redbida/apply", map[string]any{
		"auto_fix":          true,
		"store_domain":      "store-pika.video.io.vn",
		"cstore_domain":     "c-store-pika.video.io.vn",
		"shinobi_group_key": "arenaTuoIL",
	}, nil)
	if recFix.Code != http.StatusOK {
		t.Fatalf("expected 200 OK applying Pika Sun RedBida fix, got %d: %s", recFix.Code, recFix.Body.String())
	}

	repAfter := fixData
	if nested, ok := fixData["redbida"].(map[string]any); ok {
		repAfter = nested
	} else if nested, ok := fixData["report"].(map[string]any); ok {
		repAfter = nested
	}
	if repAfter["domain_conflict"] == true {
		t.Errorf("expected domain_conflict=false after updating to store-pika.video.io.vn")
	}
	if fmt.Sprint(repAfter["branding"]) != "pickleball" {
		t.Errorf("expected Pika Sun branding=pickleball preserved, got %v", repAfter["branding"])
	}
	if int(repAfter["score"].(float64)) != 17 {
		t.Errorf("expected Pika Sun RedBida score=17/17 after fix, got %v", repAfter["score"])
	}
}
