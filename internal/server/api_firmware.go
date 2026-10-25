package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

const (
	maxFirmwarePackageBytes = 32 << 20 // 32 MiB cap for embedded tmpfs safety
	firmwareStageTTL        = 30 * time.Minute
)

type stagedFirmware struct {
	ID        string                     `json:"fwId"`
	FileName  string                     `json:"fileName"`
	FilePath  string                     `json:"-"`
	Info      *dahua.FirmwarePackageInfo `json:"info"`
	CreatedAt time.Time                  `json:"createdAt"`
}

var (
	fwStoreMu sync.Mutex
	fwStore   = map[string]*stagedFirmware{}
)

func stagingFirmwareDir() string {
	dir := filepath.Join(os.TempDir(), "kspcam-fw")
	_ = os.MkdirAll(dir, 0o700)
	return dir
}

func pruneStagedFirmwaresLocked() {
	now := time.Now()
	for id, item := range fwStore {
		if now.Sub(item.CreatedAt) > firmwareStageTTL {
			_ = os.Remove(item.FilePath)
			delete(fwStore, id)
		}
	}
}

func stageFirmwareBytes(fileName string, data []byte) (*stagedFirmware, error) {
	if len(data) == 0 {
		return nil, errors.New("file firmware trống")
	}
	if len(data) > maxFirmwarePackageBytes {
		return nil, fmt.Errorf("file firmware vượt quá giới hạn an toàn 32 MB (%d bytes)", len(data))
	}
	info, err := dahua.InspectFirmwarePackage(data)
	if err != nil {
		return nil, err
	}

	fwStoreMu.Lock()
	defer fwStoreMu.Unlock()
	pruneStagedFirmwaresLocked()

	// Keep at most 1 staged file in /tmp/kspcam-fw on 1GB RAM edge boxes.
	for id, old := range fwStore {
		_ = os.Remove(old.FilePath)
		delete(fwStore, id)
	}

	var idBytes [8]byte
	_, _ = rand.Read(idBytes[:])
	fwID := "fw_" + hex.EncodeToString(idBytes[:])
	stagePath := filepath.Join(stagingFirmwareDir(), fwID+".pkg")
	if err := os.WriteFile(stagePath, data, 0o600); err != nil {
		return nil, fmt.Errorf("save staged firmware: %w", err)
	}

	if fileName == "" {
		fileName = "firmware.bin"
	}
	st := &stagedFirmware{
		ID:        fwID,
		FileName:  filepath.Base(fileName),
		FilePath:  stagePath,
		Info:      info,
		CreatedAt: time.Now(),
	}
	fwStore[fwID] = st
	return st, nil
}

func getStagedFirmware(fwID string) (*stagedFirmware, []byte, error) {
	fwStoreMu.Lock()
	pruneStagedFirmwaresLocked()
	st, ok := fwStore[fwID]
	fwStoreMu.Unlock()
	if !ok || st == nil {
		return nil, nil, errors.New("gói firmware không tồn tại hoặc đã hết hạn (30 phút), vui lòng tải file lên lại")
	}
	data, err := os.ReadFile(st.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("đọc gói firmware tạm thất bại: %w", err)
	}
	return st, data, nil
}

func dialDahuaWithFallback(d config.Device, timeout time.Duration) (*dahua.Client, int, error) {
	port := d.Port
	if port <= 0 {
		port = 37777
	}
	addr := fmt.Sprintf("%s:%d", d.Host, port)
	c, err := dahua.Dial(addr, d.Username, d.Password, timeout)
	if err != nil && errors.Is(err, dahua.ErrDialUnreachable) && port != 8888 {
		altAddr := fmt.Sprintf("%s:8888", d.Host)
		if c2, err2 := dahua.Dial(altAddr, d.Username, d.Password, timeout); err2 == nil {
			return c2, 8888, nil
		}
	}
	return c, port, err
}

// handleFirmwareInspect handles POST /api/firmware/inspect.
// Accepts multipart/form-data (field "file") or JSON {"fileName", "dataBase64"}.
func (s *Server) handleFirmwareInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var fwBytes []byte
	var fileName string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxFirmwarePackageBytes); err != nil {
			writeErr(w, http.StatusBadRequest, "parse multipart form: "+err.Error())
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			writeErr(w, http.StatusBadRequest, "thiếu trường file: "+err.Error())
			return
		}
		defer f.Close()
		if hdr != nil {
			fileName = hdr.Filename
		}
		data, err := io.ReadAll(io.LimitReader(f, maxFirmwarePackageBytes+1))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "đọc file thất bại: "+err.Error())
			return
		}
		fwBytes = data
	} else {
		var req struct {
			FileName   string `json:"fileName"`
			DataBase64 string `json:"dataBase64"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		fileName = req.FileName
		raw := req.DataBase64
		if idx := strings.Index(raw, ","); idx != -1 && strings.HasPrefix(raw, "data:") {
			raw = raw[idx+1:]
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
		if err != nil {
			writeErr(w, http.StatusBadRequest, "decode base64: "+err.Error())
			return
		}
		fwBytes = data
	}

	staged, err := stageFirmwareBytes(fileName, fwBytes)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, staged)
}

// FirmwarePreflightRow reports compatibility of one inventory camera against the staged firmware.
type FirmwarePreflightRow struct {
	DeviceID       string `json:"deviceId"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Vendor         string `json:"vendor"`
	Reachable      bool   `json:"reachable"`
	Compatible     bool   `json:"compatible"`
	HWID           string `json:"hwid,omitempty"`
	SN             string `json:"sn,omitempty"`
	CurrentVersion string `json:"currentVersion,omitempty"`
	CurrentBuild   string `json:"currentBuild,omitempty"`
	TargetVersion  string `json:"targetVersion,omitempty"`
	TargetBuild    string `json:"targetBuild,omitempty"`
	UpgraderState  string `json:"upgraderState,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// handleFirmwarePreflight handles POST /api/firmware/preflight.
func (s *Server) handleFirmwarePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		FwID           string   `json:"fwId"`
		DeviceIDs      []string `json:"deviceIds"`
		TimeoutSeconds int      `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	staged, _, err := getStagedFirmware(req.FwID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	devices := s.inv.List()
	if len(req.DeviceIDs) > 0 {
		selected := make(map[string]bool, len(req.DeviceIDs))
		for _, id := range req.DeviceIDs {
			selected[id] = true
		}
		filtered := make([]config.Device, 0, len(req.DeviceIDs))
		for _, d := range devices {
			if selected[d.ID] {
				filtered = append(filtered, d)
			}
		}
		devices = filtered
	}

	to := s.reqTimeout(req.TimeoutSeconds)
	rows := make([]FirmwarePreflightRow, 0, len(devices))
	for _, d := range devices {
		row := FirmwarePreflightRow{
			DeviceID:      d.ID,
			Name:          d.Name,
			Host:          d.Host,
			Port:          d.Port,
			Vendor:        string(d.Vendor),
			TargetVersion: staged.Info.ProductVersion,
			TargetBuild:   staged.Info.Build,
		}
		if d.Vendor != config.VendorDahua {
			row.Reason = fmt.Sprintf("Chỉ hỗ trợ Dahua/KBVision/Imou DVRIP (hiện tại: %s)", d.Vendor)
			rows = append(rows, row)
			continue
		}
		c, usedPort, err := dialDahuaWithFallback(d, to)
		if err != nil {
			row.Reason = "Không kết nối được DVRIP: " + err.Error()
			rows = append(rows, row)
			continue
		}
		row.Port = usedPort
		row.Reachable = true
		ident, err := c.GetUpgraderIdentity()
		_ = c.Close()
		if err != nil {
			row.Reason = "Không đọc được upgrader.getUpdateSerial: " + err.Error()
			rows = append(rows, row)
			continue
		}
		row.HWID = ident.UpdateSerial.Serial
		row.SN = ident.UpdateSerial.SN
		row.CurrentVersion = ident.Software.Version
		row.CurrentBuild = ident.Software.BuildDate
		row.UpgraderState = ident.State.State

		if staged.Info.SupportsHWID(ident.UpdateSerial.Serial) {
			row.Compatible = true
			row.Reason = "Khớp chính xác HWID"
		} else {
			row.Compatible = false
			row.Reason = fmt.Sprintf("Sai HWID: camera là %q, không nằm trong danh sách gói firmware", ident.UpdateSerial.Serial)
		}
		rows = append(rows, row)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"fwId":     staged.ID,
		"fileName": staged.FileName,
		"info":     staged.Info,
		"devices":  rows,
	})
}

// FirmwareSSEEvent is streamed over POST /api/firmware/apply.
type FirmwareSSEEvent struct {
	Type           string `json:"type"` // "device_start" | "phase" | "device_done" | "done"
	DeviceID       string `json:"deviceId,omitempty"`
	Name           string `json:"name,omitempty"`
	Host           string `json:"host,omitempty"`
	Index          int    `json:"index,omitempty"`
	Total          int    `json:"total,omitempty"`
	Phase          string `json:"phase,omitempty"`
	Progress       int    `json:"progress,omitempty"`
	BytesSent      int    `json:"bytesSent,omitempty"`
	TotalBytes     int    `json:"totalBytes,omitempty"`
	FlashingFile   string `json:"flashingFile,omitempty"`
	Message        string `json:"message,omitempty"`
	OK             bool   `json:"ok,omitempty"`
	Err            string `json:"err,omitempty"`
	BeforeVersion  string `json:"beforeVersion,omitempty"`
	AfterVersion   string `json:"afterVersion,omitempty"`
	BackupFilePath string `json:"backupFilePath,omitempty"`
	RTSPRestored   bool   `json:"rtspRestored,omitempty"`
}

func (s *Server) savePreFirmwareBackup(d config.Device, ident *dahua.UpgraderIdentity, tables map[string]json.RawMessage) string {
	dir := "/opt/ksp-cam/backups"
	if err := os.MkdirAll(dir, 0o700); err != nil {
		dir = filepath.Join(os.TempDir(), "kspcam-backups")
		_ = os.MkdirAll(dir, 0o700)
	}
	safeID := strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(d.ID)
	path := filepath.Join(dir, fmt.Sprintf("fw_backup_%s_%s.json", safeID, time.Now().Format("20060102_150405")))
	payload := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"deviceId":  d.ID,
		"host":      d.Host,
		"port":      d.Port,
		"identity":  ident,
		"configs":   tables,
	}
	if b, err := json.MarshalIndent(payload, "", "  "); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
	return path
}

func (s *Server) findMatchingShinobiMID(ctx context.Context, host string) string {
	if s.shinobi == nil {
		return ""
	}
	mons, err := s.shinobi.ListMonitors(ctx)
	if err != nil {
		return ""
	}
	for _, m := range mons {
		if strings.TrimSpace(m.Host) == strings.TrimSpace(host) {
			return m.Mid
		}
	}
	return ""
}

// handleFirmwareApply handles POST /api/firmware/apply via SSE.
func (s *Server) handleFirmwareApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		FwID                   string   `json:"fwId"`
		DeviceIDs              []string `json:"deviceIds"`
		DryRun                 bool     `json:"dryRun"`
		ConfirmBootloaderWrite bool     `json:"confirmBootloaderWrite"`
		TimeoutSeconds         int      `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.DeviceIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "deviceIds is required")
		return
	}
	staged, fwBytes, err := getStagedFirmware(req.FwID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.DryRun && staged.Info.TouchesBootloader && !req.ConfirmBootloaderWrite {
		writeErr(w, http.StatusBadRequest, "Gói firmware có ghi Bootloader (dhboot.bin.img). Bắt buộc xác nhận rủi ro (confirmBootloaderWrite=true) trước khi nạp thật.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	emit := func(ev FirmwareSSEEvent) {
		if r.Context().Err() != nil {
			return
		}
		b, err := json.Marshal(ev)
		if err != nil {
			return
		}
		_, _ = w.Write([]byte("data: " + string(b) + "\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}

	to := s.reqTimeout(req.TimeoutSeconds)
	if to < 25*time.Second {
		to = 25 * time.Second
	}

	total := len(req.DeviceIDs)
	for i, id := range req.DeviceIDs {
		if r.Context().Err() != nil {
			break
		}
		d, ok := s.inv.Get(id)
		emit(FirmwareSSEEvent{
			Type:     "device_start",
			DeviceID: id,
			Name:     d.Name,
			Host:     d.Host,
			Index:    i + 1,
			Total:    total,
		})
		if !ok {
			emit(FirmwareSSEEvent{Type: "device_done", DeviceID: id, OK: false, Err: "không tìm thấy camera trong kho"})
			continue
		}
		if d.Vendor != config.VendorDahua {
			emit(FirmwareSSEEvent{Type: "device_done", DeviceID: id, Name: d.Name, Host: d.Host, OK: false, Err: "chỉ hỗ trợ thiết bị Dahua/KBVision/Imou DVRIP"})
			continue
		}

		c, usedPort, err := dialDahuaWithFallback(d, to)
		if err != nil {
			emit(FirmwareSSEEvent{Type: "device_done", DeviceID: id, Name: d.Name, Host: d.Host, OK: false, Err: err.Error()})
			continue
		}

		identBefore, err := c.GetUpgraderIdentity()
		if err != nil {
			_ = c.Close()
			emit(FirmwareSSEEvent{Type: "device_done", DeviceID: id, Name: d.Name, Host: d.Host, OK: false, Err: err.Error()})
			continue
		}
		beforeVer := identBefore.Software.Version

		// Step 1: Backup critical config tables before touching upgrader
		configs := c.BackupCriticalConfigs()
		backupPath := s.savePreFirmwareBackup(d, identBefore, configs)
		emit(FirmwareSSEEvent{
			Type:           "phase",
			DeviceID:       id,
			Name:           d.Name,
			Host:           d.Host,
			Phase:          "backup",
			Progress:       100,
			BackupFilePath: backupPath,
			BeforeVersion:  beforeVer,
			Message:        "Đã sao lưu cấu hình Network/Encode/RTSP vào " + backupPath,
		})

		// Pause Shinobi monitor during real firmware flash to free Wi-Fi/LAN airtime
		var pausedMID string
		if !req.DryRun && s.shinobi != nil {
			if mid := s.findMatchingShinobiMID(r.Context(), d.Host); mid != "" {
				if err := s.shinobi.ChangeMonitorState(r.Context(), mid, "stop"); err == nil {
					pausedMID = mid
				}
			}
		}

		// Step 2: Run firmware transfer & (optional) flash
		_, err = c.RunFirmwareSession(r.Context(), fwBytes, staged.Info, req.DryRun, func(pev dahua.UpgradePhaseEvent) {
			emit(FirmwareSSEEvent{
				Type:          "phase",
				DeviceID:      id,
				Name:          d.Name,
				Host:          d.Host,
				Phase:         pev.Phase,
				Progress:      pev.Progress,
				BytesSent:     pev.BytesSent,
				TotalBytes:    pev.TotalBytes,
				FlashingFile:  pev.FlashingFile,
				BeforeVersion: beforeVer,
				Message:       pev.Message,
			})
		})
		_ = c.Close()

		if err != nil {
			if pausedMID != "" {
				_ = s.shinobi.ChangeMonitorState(context.Background(), pausedMID, "record")
			}
			emit(FirmwareSSEEvent{
				Type:           "device_done",
				DeviceID:       id,
				Name:           d.Name,
				Host:           d.Host,
				OK:             false,
				Err:            err.Error(),
				BeforeVersion:  beforeVer,
				BackupFilePath: backupPath,
			})
			continue
		}

		if req.DryRun {
			emit(FirmwareSSEEvent{
				Type:           "device_done",
				DeviceID:       id,
				Name:           d.Name,
				Host:           d.Host,
				OK:             true,
				BeforeVersion:  beforeVer,
				AfterVersion:   beforeVer,
				BackupFilePath: backupPath,
				Message:        "Kiểm chứng truyền tải 100% thành công (Dry-Run, đã hủy phiên không ghi Flash)",
			})
			continue
		}

		// Step 3: Wait for reboot and read back actual software version + restore RTSP 554
		emit(FirmwareSSEEvent{
			Type:          "phase",
			DeviceID:      id,
			Name:          d.Name,
			Host:          d.Host,
			Phase:         "rebooting",
			Progress:      100,
			BeforeVersion: beforeVer,
			Message:       "Đang chờ camera khởi động lại và kết nối lại cổng DVRIP...",
		})

		afterVer := ""
		rtspRestored := false
		reconnected := false
		// Wait briefly so we don't reconnect before the reboot actually begins
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}

		rebootDeadline := time.Now().Add(150 * time.Second)
		for time.Now().Before(rebootDeadline) {
			if r.Context().Err() != nil {
				break
			}
			dPost := d
			dPost.Port = usedPort
			cPost, _, dialErr := dialDahuaWithFallback(dPost, 8*time.Second)
			if dialErr != nil {
				select {
				case <-r.Context().Done():
				case <-time.After(3 * time.Second):
				}
				continue
			}
			if identAfter, idErr := cPost.GetUpgraderIdentity(); idErr == nil {
				afterVer = identAfter.Software.Version
				if changed, rtspErr := cPost.EnsureRTSPPort554(); rtspErr == nil && changed {
					rtspRestored = true
				}
				_ = cPost.Close()
				reconnected = true
				break
			}
			_ = cPost.Close()
			time.Sleep(2 * time.Second)
		}

		if pausedMID != "" {
			_ = s.shinobi.ChangeMonitorState(context.Background(), pausedMID, "record")
		}

		if !reconnected {
			emit(FirmwareSSEEvent{
				Type:           "device_done",
				DeviceID:       id,
				Name:           d.Name,
				Host:           d.Host,
				OK:             false,
				Err:            "Đã nạp 100% nhưng quá thời gian chờ camera kết nối lại sau reboot (150s)",
				BeforeVersion:  beforeVer,
				BackupFilePath: backupPath,
			})
			continue
		}

		emit(FirmwareSSEEvent{
			Type:           "phase",
			DeviceID:       id,
			Name:           d.Name,
			Host:           d.Host,
			Phase:          "verify_and_restore",
			Progress:       100,
			BeforeVersion:  beforeVer,
			AfterVersion:   afterVer,
			RTSPRestored:   rtspRestored,
			BackupFilePath: backupPath,
			Message:        fmt.Sprintf("Xác nhận sau khởi động: %s → %s (RTSP 554 restored: %v)", beforeVer, afterVer, rtspRestored),
		})
		emit(FirmwareSSEEvent{
			Type:           "device_done",
			DeviceID:       id,
			Name:           d.Name,
			Host:           d.Host,
			OK:             true,
			BeforeVersion:  beforeVer,
			AfterVersion:   afterVer,
			RTSPRestored:   rtspRestored,
			BackupFilePath: backupPath,
			Message:        fmt.Sprintf("Hoàn tất: %s → %s", beforeVer, afterVer),
		})
	}

	emit(FirmwareSSEEvent{Type: "done"})
}
