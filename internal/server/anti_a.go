package server

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// antiALogEntry represents an event or infraction caught by Anti-A Guardian.
type antiALogEntry struct {
	Timestamp     string `json:"timestamp"`
	CameraID      string `json:"cameraId"`
	CameraName    string `json:"cameraName"`
	Host          string `json:"host"`
	DetectedCodec string `json:"detectedCodec"`
	SmartCodec    bool   `json:"smartCodec"`
	ActionTaken   string `json:"actionTaken"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
}

// antiAStatusView is returned to Web UI and MCP clients.
type antiAStatusView struct {
	Enabled          bool            `json:"enabled"`
	IntervalMinutes  int             `json:"intervalMinutes"`
	Mode             string          `json:"mode"`
	Running          bool            `json:"running"`
	LastCheck        string          `json:"lastCheck"`
	NextCheck        string          `json:"nextCheck"`
	InfractionsCount int             `json:"infractionsCount"`
	LastInfraction   string          `json:"lastInfraction,omitempty"`
	StatusMessage    string          `json:"statusMessage"`
	RecentLogs       []antiALogEntry `json:"recentLogs"`
}

type antiAConfigReq struct {
	Enabled         *bool   `json:"enabled"`
	IntervalMinutes *int    `json:"intervalMinutes"`
	Mode            *string `json:"mode"`
}

// antiAGuardian continuously monitors and enforces H.265 + Smart Codec + Audio AAC
// across all registered cameras to prevent rival software from hijacking streams.
type antiAGuardian struct {
	mu               sync.RWMutex
	inv              *config.Inventory
	server           *Server
	cfgPath          string
	cfg              config.AntiAConfig
	cancel           context.CancelFunc
	running          bool
	lastCheck        time.Time
	lastInfraction   time.Time
	infractionsCount int
	statusMsg        string
	logs             []antiALogEntry
	maxLogs          int
}

func newAntiAGuardian(server *Server, inv *config.Inventory, cfgPath string, cfg config.AntiAConfig) *antiAGuardian {
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = 30
	}
	if cfg.Mode == "" {
		cfg.Mode = "random"
	}
	return &antiAGuardian{
		server:    server,
		inv:       inv,
		cfgPath:   cfgPath,
		cfg:       cfg,
		maxLogs:   50,
		statusMsg: "Anti-A Guardian đã sẵn sàng",
	}
}

func (g *antiAGuardian) start(ctx context.Context) {
	g.mu.Lock()
	if g.running || !g.cfg.Enabled {
		g.mu.Unlock()
		return
	}
	loopCtx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	g.running = true
	g.statusMsg = fmt.Sprintf("Đang bảo vệ (quét %s mỗi %d phút)", g.cfg.Mode, g.cfg.IntervalMinutes)
	g.mu.Unlock()

	go g.runLoop(loopCtx)
}

func (g *antiAGuardian) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	g.running = false
	g.statusMsg = "Anti-A Guardian đã tạm dừng"
}

func (g *antiAGuardian) updateConfig(ctx context.Context, req antiAConfigReq) error {
	g.mu.Lock()
	if req.Enabled != nil {
		g.cfg.Enabled = *req.Enabled
	}
	if req.IntervalMinutes != nil && *req.IntervalMinutes > 0 {
		g.cfg.IntervalMinutes = *req.IntervalMinutes
	}
	if req.Mode != nil && (*req.Mode == "random" || *req.Mode == "full") {
		g.cfg.Mode = *req.Mode
	}

	// Persist to config.yaml if server config is available
	if g.server != nil && g.cfgPath != "" {
		if curCfg, err := config.Load(g.cfgPath); err == nil {
			curCfg.AntiA = g.cfg
			_ = curCfg.Save(g.cfgPath)
		}
	}

	shouldRun := g.cfg.Enabled
	g.mu.Unlock()

	if shouldRun {
		g.stop()
		g.start(ctx)
	} else {
		g.stop()
	}
	return nil
}

func (g *antiAGuardian) runLoop(ctx context.Context) {
	g.mu.RLock()
	interval := time.Duration(g.cfg.IntervalMinutes) * time.Minute
	g.mu.RUnlock()

	if interval <= 0 {
		interval = 30 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check on startup after 5 seconds grace period
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
		_, _ = g.runCheck(ctx, false)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = g.runCheck(ctx, false)
		}
	}
}

func (g *antiAGuardian) triggerNow(ctx context.Context) (int, error) {
	return g.runCheck(ctx, true)
}

func (g *antiAGuardian) addLog(entry antiALogEntry) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339)
	}
	g.logs = append([]antiALogEntry{entry}, g.logs...)
	if len(g.logs) > g.maxLogs {
		g.logs = g.logs[:g.maxLogs]
	}
}

func isStreamH265AndSmartCodec(stream camera.StreamInfo) bool {
	codecUpper := strings.ToUpper(strings.TrimSpace(stream.Compression))
	isH265 := strings.Contains(codecUpper, "H.265") || strings.Contains(codecUpper, "H265") || strings.Contains(codecUpper, "HEVC")
	return isH265 && stream.SmartCodec
}

func (g *antiAGuardian) runCheck(ctx context.Context, forceFull bool) (int, error) {
	g.mu.Lock()
	g.lastCheck = time.Now()
	mode := g.cfg.Mode
	g.mu.Unlock()

	cams := g.inv.List()
	if len(cams) == 0 {
		return 0, nil
	}

	enforcedCount := 0

	if mode == "random" && !forceFull {
		idx := rand.Intn(len(cams))
		targetCam := cams[idx]

		cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		cam, err := camera.Open(cctx, targetCam, 15*time.Second)
		if err != nil {
			log.Printf("[Anti-A] Không thể kết nối camera mẫu %s (%s): %v", targetCam.Name, targetCam.Host, err)
			return 0, nil
		}
		defer cam.Close()

		streams, err := cam.Probe(cctx)
		if err != nil || len(streams) == 0 {
			log.Printf("[Anti-A] Lỗi probe camera mẫu %s: %v", targetCam.Name, err)
			return 0, nil
		}

		mainStream := streams[0]
		for _, s := range streams {
			if s.Stream == camera.StreamMain {
				mainStream = s
				break
			}
		}

		if !isStreamH265AndSmartCodec(mainStream) {
			// INFRACTION DETECTED!
			g.mu.Lock()
			g.infractionsCount++
			g.lastInfraction = time.Now()
			g.statusMsg = fmt.Sprintf("⚠️ Phát hiện %s bị đổi về %s (Smart: %v)! Đang kích hoạt khóa toàn bộ fleet...", targetCam.Name, mainStream.Compression, mainStream.SmartCodec)
			g.mu.Unlock()

			log.Printf("[Anti-A ALERT] Camera %s (%s) bị lệch chuẩn! Codec: %s, SmartCodec: %v. Đang ép toàn bộ camera về H.265...",
				targetCam.Name, targetCam.Host, mainStream.Compression, mainStream.SmartCodec)

			g.addLog(antiALogEntry{
				CameraID:      targetCam.ID,
				CameraName:    targetCam.Name,
				Host:          targetCam.Host,
				DetectedCodec: mainStream.Compression,
				SmartCodec:    mainStream.SmartCodec,
				ActionTaken:   "Phát hiện lệch chuẩn -> Kích hoạt khóa toàn bộ hệ thống về H.265/SmartCodec/AAC",
				Success:       true,
			})

			// Enforce ALL cameras
			enforcedCount = g.enforceAll(ctx)
		} else {
			g.mu.Lock()
			g.statusMsg = fmt.Sprintf("Hệ thống an toàn (Đã kiểm tra mẫu %s: %s / SmartCodec: BẬT)", targetCam.Name, mainStream.Compression)
			g.mu.Unlock()
		}
	} else {
		// Full fleet scan
		enforcedCount = g.enforceAll(ctx)
	}

	return enforcedCount, nil
}

func (g *antiAGuardian) enforceAll(ctx context.Context) int {
	cams := g.inv.List()
	enforced := 0

	for _, camCfg := range cams {
		select {
		case <-ctx.Done():
			return enforced
		default:
		}

		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		func() {
			defer cancel()
			cam, err := camera.Open(cctx, camCfg, 20*time.Second)
			if err != nil {
				g.addLog(antiALogEntry{
					CameraID:    camCfg.ID,
					CameraName:  camCfg.Name,
					Host:        camCfg.Host,
					ActionTaken: "Không thể kết nối thiết bị",
					Success:     false,
					Error:       err.Error(),
				})
				return
			}
			defer cam.Close()

			// Check current probe
			streams, err := cam.Probe(cctx)
			if err == nil && len(streams) > 0 {
				mainStream := streams[0]
				for _, s := range streams {
					if s.Stream == camera.StreamMain {
						mainStream = s
						break
					}
				}
				if isStreamH265AndSmartCodec(mainStream) {
					// Already compliant
					return
				}
			}

			// Apply H.265 + Smart Codec + Audio AAC
			profile := camera.Profile{
				SetCodec:      true,
				Codec:         "H.265",
				SetSmartCodec: true,
				SmartCodec:    true,
				SetAudioAAC:   true,
				Streams:       []int{camera.StreamMain},
			}

			steps := cam.Apply(cctx, profile, nil)
			hasErr := false
			var errMsgs []string
			for _, step := range steps {
				if !step.OK && step.Err != "" {
					hasErr = true
					errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", step.Step, step.Err))
				}
			}

			detectedCodec := "Unknown"
			smartCodec := false
			if len(streams) > 0 {
				detectedCodec = streams[0].Compression
				smartCodec = streams[0].SmartCodec
			}

			if !hasErr {
				enforced++
				g.addLog(antiALogEntry{
					CameraID:      camCfg.ID,
					CameraName:    camCfg.Name,
					Host:          camCfg.Host,
					DetectedCodec: detectedCodec,
					SmartCodec:    smartCodec,
					ActionTaken:   "Đã ép thành công về H.265 + Smart Codec + Audio AAC",
					Success:       true,
				})
				log.Printf("[Anti-A] Đã khóa thành công %s (%s) về H.265+ / AAC", camCfg.Name, camCfg.Host)
			} else {
				g.addLog(antiALogEntry{
					CameraID:      camCfg.ID,
					CameraName:    camCfg.Name,
					Host:          camCfg.Host,
					DetectedCodec: detectedCodec,
					SmartCodec:    smartCodec,
					ActionTaken:   "Lỗi khi áp dụng H.265",
					Success:       false,
					Error:         strings.Join(errMsgs, "; "),
				})
			}
		}()
	}

	g.mu.Lock()
	if enforced > 0 {
		g.statusMsg = fmt.Sprintf("Đã khóa an toàn %d camera về H.265/SmartCodec/AAC", enforced)
	} else {
		g.statusMsg = "Toàn bộ camera đều đang ở chuẩn H.265/SmartCodec/AAC"
	}
	g.mu.Unlock()

	return enforced
}

func (g *antiAGuardian) getStatus() antiAStatusView {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastCheckStr := ""
	if !g.lastCheck.IsZero() {
		lastCheckStr = g.lastCheck.Format(time.RFC3339)
	}

	nextCheckStr := ""
	if g.running && !g.lastCheck.IsZero() {
		nextCheckStr = g.lastCheck.Add(time.Duration(g.cfg.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	} else if g.running {
		nextCheckStr = time.Now().Add(time.Duration(g.cfg.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	}

	lastInfractionStr := ""
	if !g.lastInfraction.IsZero() {
		lastInfractionStr = g.lastInfraction.Format(time.RFC3339)
	}

	logsCopy := make([]antiALogEntry, len(g.logs))
	copy(logsCopy, g.logs)

	return antiAStatusView{
		Enabled:          g.cfg.Enabled,
		IntervalMinutes:  g.cfg.IntervalMinutes,
		Mode:             g.cfg.Mode,
		Running:          g.running,
		LastCheck:        lastCheckStr,
		NextCheck:        nextCheckStr,
		InfractionsCount: g.infractionsCount,
		LastInfraction:   lastInfractionStr,
		StatusMessage:    g.statusMsg,
		RecentLogs:       logsCopy,
	}
}
