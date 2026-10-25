package dahua

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// UpgraderChunkSize is the default DVRIP binary payload chunk size (32 KiB)
	// used for upgrader.appendData to keep Wi-Fi/embedded socket buffers safe.
	UpgraderChunkSize = 32 * 1024
)

// UpdateSerialEntry represents one hardware identity row returned by upgrader.getUpdateSerial.
type UpdateSerialEntry struct {
	Chip          string `json:"Chip,omitempty"`
	Main          string `json:"Main,omitempty"`
	Vendor        string `json:"Vendor,omitempty"`
	Serial        string `json:"serial"`
	SN            string `json:"sn"`
	SWVersion     string `json:"SWVersion"`
	Build         string `json:"build,omitempty"`
	Language      string `json:"Language,omitempty"`
	UsingLanguage string `json:"usingLanguage,omitempty"`
}

// SoftwareVersionInfo holds the version metadata returned by magicBox.getSoftwareVersion.
type SoftwareVersionInfo struct {
	Version                 string `json:"Version"`
	BuildDate               string `json:"BuildDate"`
	WebVersion              string `json:"WebVersion,omitempty"`
	SecurityBaselineVersion string `json:"SecurityBaseLineVersion,omitempty"`
}

// UpgraderState represents the device's current upgrade state from upgrader.getState.
type UpgraderState struct {
	File     string `json:"File"`
	Progress int    `json:"Progress"`
	State    string `json:"State"`
}

// UpgradePhaseEvent is emitted to callers during preflight, upload, flash, and post-reboot verification.
type UpgradePhaseEvent struct {
	Phase        string `json:"phase"`    // "backup" | "prepare" | "upload" | "dry_run_done" | "execute" | "flashing" | "rebooting" | "verify_and_restore" | "done" | "error"
	Progress     int    `json:"progress"` // 0..100
	BytesSent    int    `json:"bytesSent,omitempty"`
	TotalBytes   int    `json:"totalBytes,omitempty"`
	FlashingFile string `json:"flashingFile,omitempty"` // e.g. "kernel.img", "romfs-x.squashfs.img", "dhboot.bin.img"
	State        string `json:"state,omitempty"`
	Message      string `json:"message"`
}

// UpgraderIdentity summarizes the camera's hardware & software identity for preflight checks.
type UpgraderIdentity struct {
	ObjectHandle int64               `json:"objectHandle"`
	UpdateSerial UpdateSerialEntry   `json:"updateSerial"`
	Software     SoftwareVersionInfo `json:"software"`
	State        UpgraderState       `json:"state"`
}

// UpgraderInstance acquires the singleton upgrader object handle via upgrader.factory.instance.
func (c *Client) UpgraderInstance() (int64, error) {
	resp, err := c.Call("upgrader.factory.instance", nil)
	if err != nil {
		return 0, fmt.Errorf("upgrader.factory.instance: %w", err)
	}
	var obj int64
	if err := json.Unmarshal(resp.Result, &obj); err != nil || obj <= 0 {
		return 0, fmt.Errorf("upgrader.factory.instance invalid handle (result=%s error=%s)", resp.Result, resp.Error)
	}
	return obj, nil
}

// GetUpgraderIdentity queries upgrader.factory.instance, upgrader.getUpdateSerial,
// upgrader.getState, and magicBox.getSoftwareVersion.
func (c *Client) GetUpgraderIdentity() (*UpgraderIdentity, error) {
	obj, err := c.UpgraderInstance()
	if err != nil {
		return nil, err
	}
	serialResp, err := c.CallObject("upgrader.getUpdateSerial", obj, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrader.getUpdateSerial: %w", err)
	}
	if !serialResp.ok() {
		return nil, fmt.Errorf("upgrader.getUpdateSerial failed: %s", respErr(serialResp))
	}
	var listWrap struct {
		List []UpdateSerialEntry `json:"list"`
	}
	if err := json.Unmarshal(serialResp.Params, &listWrap); err != nil || len(listWrap.List) == 0 {
		return nil, fmt.Errorf("upgrader.getUpdateSerial empty list: %s", serialResp.Params)
	}

	var state UpgraderState
	if stateResp, err := c.CallObject("upgrader.getState", obj, nil); err == nil && len(stateResp.Params) > 0 {
		_ = json.Unmarshal(stateResp.Params, &state)
	}

	var sw SoftwareVersionInfo
	if swResp, err := c.Call("magicBox.getSoftwareVersion", nil); err == nil && len(swResp.Params) > 0 {
		var wrap struct {
			Version SoftwareVersionInfo `json:"version"`
		}
		if json.Unmarshal(swResp.Params, &wrap) == nil && wrap.Version.Version != "" {
			sw = wrap.Version
		} else {
			_ = json.Unmarshal(swResp.Params, &sw)
		}
	}
	if sw.Version == "" {
		sw.Version = listWrap.List[0].SWVersion
	}

	return &UpgraderIdentity{
		ObjectHandle: obj,
		UpdateSerial: listWrap.List[0],
		Software:     sw,
		State:        state,
	}, nil
}

// GetUpgraderState reads the current upgrade state on an open upgrader object handle.
func (c *Client) GetUpgraderState(obj int64) (UpgraderState, error) {
	resp, err := c.CallObject("upgrader.getState", obj, nil)
	if err != nil {
		return UpgraderState{}, err
	}
	if !resp.ok() {
		return UpgraderState{}, fmt.Errorf("upgrader.getState failed: %s", respErr(resp))
	}
	var st UpgraderState
	if err := json.Unmarshal(resp.Params, &st); err != nil {
		return UpgraderState{}, fmt.Errorf("decode upgrader.getState: %w", err)
	}
	return st, nil
}

// BackupCriticalConfigs dumps key configuration tables before a firmware operation
// so RTSP port (554) and stream settings can be restored if the firmware package
// contains //IPC_RestoreDefault.
func (c *Client) BackupCriticalConfigs() map[string]json.RawMessage {
	tables := []string{"Network", "WLan", "Encode", "SmartEncode", "RTSP", "ChannelTitle", "VideoWidget", "T2UServer"}
	backup := make(map[string]json.RawMessage, len(tables))
	for _, name := range tables {
		resp, err := c.Call("configManager.getConfig", map[string]any{"name": name})
		if err == nil && resp.ok() && len(resp.Params) > 0 {
			backup[name] = resp.Params
		}
	}
	return backup
}

// EnsureRTSPPort554 checks the camera's RTSP config table and restores Port=554 and Enable=true
// if a firmware flash reset it (e.g. to 8554 on factory image defaults).
func (c *Client) EnsureRTSPPort554() (changed bool, err error) {
	resp, err := c.Call("configManager.getConfig", map[string]any{"name": "RTSP"})
	if err != nil {
		return false, err
	}
	if !resp.ok() {
		return false, fmt.Errorf("getConfig RTSP: %s", respErr(resp))
	}
	var holder struct {
		Table map[string]any `json:"table"`
	}
	if err := json.Unmarshal(resp.Params, &holder); err != nil || holder.Table == nil {
		return false, fmt.Errorf("decode RTSP config: %w", err)
	}
	portVal, _ := holder.Table["Port"].(float64)
	enVal, _ := holder.Table["Enable"].(bool)
	if int(portVal) == 554 && enVal {
		return false, nil
	}
	holder.Table["Enable"] = true
	holder.Table["Port"] = 554
	setResp, err := c.Call("configManager.setConfig", map[string]any{
		"name":  "RTSP",
		"table": holder.Table,
	})
	if err != nil {
		return false, err
	}
	if !setResp.ok() {
		return false, fmt.Errorf("setConfig RTSP: %s", respErr(setResp))
	}
	return true, nil
}

// RunFirmwareSession performs a complete, safety-checked firmware transfer over DVRIP:
//  1. Parses and CRC-checks the package (unless pkgInfo is already provided).
//  2. Queries upgrader.getUpdateSerial and enforces exact HWID match against the package.
//  3. Calls upgrader.prepare (canceling an abandoned 0-progress prepare if needed).
//  4. Streams the firmware in 32 KiB chunks via upgrader.appendData with binary attachment.
//  5. If dryRun is true, calls upgrader.cancel after 100% upload ACK and returns without flashing.
//  6. If dryRun is false, calls upgrader.execute({"autoReboot": true}) exactly once and polls upgrader.getState until 100% or reboot disconnect.
func (c *Client) RunFirmwareSession(ctx context.Context, fwData []byte, pkgInfo *FirmwarePackageInfo, dryRun bool, emit func(UpgradePhaseEvent)) (*UpgraderIdentity, error) {
	if emit == nil {
		emit = func(UpgradePhaseEvent) {}
	}
	var err error
	if pkgInfo == nil {
		pkgInfo, err = InspectFirmwarePackage(fwData)
		if err != nil {
			return nil, err
		}
	}

	ident, err := c.GetUpgraderIdentity()
	if err != nil {
		return nil, err
	}

	if !pkgInfo.SupportsHWID(ident.UpdateSerial.Serial) {
		return ident, fmt.Errorf("%w: camera serial=%q not in package HWIDs %v", ErrHWIDMismatch, ident.UpdateSerial.Serial, pkgInfo.HWIDs)
	}

	if ident.State.Progress != 0 || ident.State.File != "" || (ident.State.State != "" && ident.State.State != "Downloading" && ident.State.State != "Idle" && ident.State.State != "Normal") {
		return ident, fmt.Errorf("camera upgrader busy (state=%q file=%q progress=%d%%)", ident.State.State, ident.State.File, ident.State.Progress)
	}

	emit(UpgradePhaseEvent{
		Phase:      "prepare",
		Progress:   0,
		TotalBytes: len(fwData),
		Message:    fmt.Sprintf("Chuẩn bị phiên nạp cho %s (SN: %s, FW hiện tại: %s)", ident.UpdateSerial.Serial, ident.UpdateSerial.SN, ident.Software.Version),
	})

	obj := ident.ObjectHandle
	prep, err := c.CallObject("upgrader.prepare", obj, nil)
	if err != nil {
		return ident, fmt.Errorf("upgrader.prepare: %w", err)
	}
	if !prep.ok() {
		// An earlier session may have called prepare and disconnected with 0 bytes transferred.
		cancelResp, cancelErr := c.CallObject("upgrader.cancel", obj, nil)
		if cancelErr != nil || !cancelResp.ok() {
			return ident, fmt.Errorf("upgrader.prepare failed (%s) and cancel recovery failed (%v)", respErr(prep), cancelErr)
		}
		prep, err = c.CallObject("upgrader.prepare", obj, nil)
		if err != nil || !prep.ok() {
			return ident, fmt.Errorf("upgrader.prepare retry failed: %v %s", err, respErr(prep))
		}
	}

	prepared := true
	defer func() {
		if prepared {
			_, _ = c.CallObject("upgrader.cancel", obj, nil)
		}
	}()

	total := len(fwData)
	lastReportedPct := -1
	for offset := 0; offset < total; {
		if err := ctx.Err(); err != nil {
			return ident, err
		}
		end := offset + UpgraderChunkSize
		if end > total {
			end = total
		}
		resp, err := c.CallObjectWithBinary(
			"upgrader.appendData",
			obj,
			map[string]any{"totalLength": total},
			fwData[offset:end],
		)
		if err != nil {
			return ident, fmt.Errorf("upgrader.appendData at %d/%d: %w", offset, total, err)
		}
		if !resp.ok() {
			return ident, fmt.Errorf("upgrader.appendData rejected at %d/%d: %s", offset, total, respErr(resp))
		}
		offset = end
		pct := (offset * 100) / total
		if pct >= lastReportedPct+5 || offset == total {
			lastReportedPct = pct
			emit(UpgradePhaseEvent{
				Phase:      "upload",
				Progress:   pct,
				BytesSent:  offset,
				TotalBytes: total,
				Message:    fmt.Sprintf("Đã truyền %d / %d KB (%d%%)", offset/1024, total/1024, pct),
			})
		}
	}

	if dryRun {
		cancelResp, err := c.CallObject("upgrader.cancel", obj, nil)
		prepared = false
		if err != nil || !cancelResp.ok() {
			return ident, fmt.Errorf("dry-run upgrader.cancel failed: %v %s", err, respErr(cancelResp))
		}
		emit(UpgradePhaseEvent{
			Phase:      "dry_run_done",
			Progress:   100,
			BytesSent:  total,
			TotalBytes: total,
			Message:    "Kiểm chứng truyền tải 100% thành công và đã hủy phiên an toàn (không ghi Flash).",
		})
		return ident, nil
	}

	// Execute flash ONCE. Never retry execute if connection drops afterward.
	prepared = false
	emit(UpgradePhaseEvent{
		Phase:      "execute",
		Progress:   0,
		BytesSent:  total,
		TotalBytes: total,
		Message:    "Đã gửi lệnh upgrader.execute — bắt đầu ghi Flash trên camera...",
	})
	execResp, err := c.CallObject("upgrader.execute", obj, map[string]any{"autoReboot": true})
	if err != nil {
		return ident, fmt.Errorf("upgrader.execute: %w", err)
	}
	if !execResp.ok() {
		return ident, fmt.Errorf("upgrader.execute refused: %s", respErr(execResp))
	}

	// Poll upgrader.getState until Progress reaches 100 or the camera reboots and closes the TCP connection after flashing.
	sawFlashing := false
	lastProgress := 0
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ident, ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
		st, stateErr := c.GetUpgraderState(obj)
		if stateErr != nil {
			// If we already observed flashing progress (e.g. >= 80% or dhboot), a connection drop means the camera is rebooting.
			if sawFlashing && lastProgress >= 80 {
				emit(UpgradePhaseEvent{
					Phase:    "rebooting",
					Progress: 100,
					Message:  "Camera đã hoàn tất ghi Flash và đang khởi động lại...",
				})
				return ident, nil
			}
			return ident, fmt.Errorf("upgrader.getState during flash: %w", stateErr)
		}
		if st.File != "" || st.Progress > 0 || strings.EqualFold(st.State, "Upgrading") {
			sawFlashing = true
		}
		if st.Progress > lastProgress {
			lastProgress = st.Progress
		}
		emit(UpgradePhaseEvent{
			Phase:        "flashing",
			Progress:     st.Progress,
			FlashingFile: st.File,
			State:        st.State,
			Message:      fmt.Sprintf("Đang ghi %s (%s) — %d%%", nonEmpty(st.File, "firmware"), nonEmpty(st.State, "Upgrading"), st.Progress),
		})
		if st.Progress >= 100 || strings.EqualFold(st.State, "Succeeded") {
			return ident, nil
		}
		if strings.EqualFold(st.State, "Failed") || strings.EqualFold(st.State, "Error") {
			return ident, fmt.Errorf("camera reported upgrade failure: state=%s file=%s progress=%d", st.State, st.File, st.Progress)
		}
	}

	return ident, errors.New("upgrade state polling timed out")
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
