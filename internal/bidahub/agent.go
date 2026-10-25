package bidahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

// AgentConfig configures the edge-side inut-bida-agent daemon.
type AgentConfig struct {
	HubURL            string
	BoxID             string
	Secret            string
	HeartbeatInterval time.Duration
	KspcamPort        int
	ShinobiPort       int
	NodeRedPort       int
}

// EdgeAgent maintains a persistent outbound WebSocket + Yamux reverse tunnel to inut-bida-hub
// and pushes periodic live system telemetry heartbeats without SSH or VPN dependencies.
type EdgeAgent struct {
	cfg        AgentConfig
	httpClient *http.Client

	mu          sync.Mutex
	prevCPUTot  uint64
	prevCPUIdle uint64
	prevRXBytes uint64
	prevRXTime  time.Time
	lastMetrics HeartbeatPayload
}

// NewEdgeAgent creates a new EdgeAgent instance.
func NewEdgeAgent(cfg AgentConfig) *EdgeAgent {
	if strings.TrimSpace(cfg.HubURL) == "" {
		cfg.HubURL = "https://inut-bida.p2p.inut.io.vn"
	}
	if strings.TrimSpace(cfg.BoxID) == "" {
		cfg.BoxID = DetectLocalBoxID()
	}
	if strings.TrimSpace(cfg.Secret) == "" {
		cfg.Secret = "inut-bida-hub-key-2026"
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 20 * time.Second
	}
	if cfg.KspcamPort <= 0 {
		cfg.KspcamPort = 2028
	}
	if cfg.ShinobiPort <= 0 {
		cfg.ShinobiPort = 8080
	}
	if cfg.NodeRedPort <= 0 {
		cfg.NodeRedPort = 1880
	}
	return &EdgeAgent{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// DetectLocalBoxID auto-discovers the canonical box ID (e.g. "inut_204_141") from
// environment variables, /etc/tinc/inut/tinc.conf, or system hostname.
func DetectLocalBoxID() string {
	for _, envKey := range []string{"INUT_BOX_ID", "BOX_ID"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return normalizeBoxID(v)
		}
	}
	if raw, err := os.ReadFile("/etc/tinc/inut/tinc.conf"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Name") {
				val := strings.TrimSpace(parts[1])
				if strings.HasPrefix(val, "inut_") {
					return val
				}
			}
		}
	}
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		return normalizeBoxID(host)
	}
	return "inut_unknown"
}

func normalizeBoxID(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "inut-") {
		s = strings.ReplaceAll(s, "-", "_")
	}
	return s
}

// Run starts both the periodic heartbeat loop and the persistent outbound Yamux tunnel loop.
func (a *EdgeAgent) Run(ctx context.Context) error {
	// Warm up CPU & RX delta counters
	_ = a.CollectTelemetry()

	go a.runHeartbeatLoop(ctx)
	return a.runTunnelLoop(ctx)
}

func (a *EdgeAgent) runHeartbeatLoop(ctx context.Context) {
	// Send initial heartbeat right away
	_ = a.PushHeartbeatOnce(ctx)

	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.PushHeartbeatOnce(ctx)
		}
	}
}

func (a *EdgeAgent) runTunnelLoop(ctx context.Context) error {
	backoff := 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()
		err := a.RunTunnelOnce(ctx)
		if time.Since(start) > 10*time.Second {
			backoff = 2 * time.Second
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			log.Printf("[inut-bida-agent] yamux tunnel disconnected (%v), reconnecting in %v...", err, backoff)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			if backoff < 15*time.Second {
				backoff *= 2
				if backoff > 15*time.Second {
					backoff = 15 * time.Second
				}
			}
		}
	}
}

// RunTunnelOnce establishes a single WebSocket + Yamux reverse tunnel session to the Hub and serves streams until closed.
func (a *EdgeAgent) RunTunnelOnce(ctx context.Context) error {
	wsURL := toWebSocketURL(a.cfg.HubURL, "/fleet/tunnel", a.cfg.BoxID)

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	headers := http.Header{}
	headers.Set("X-Box-ID", a.cfg.BoxID)
	headers.Set("X-Box-Secret", a.cfg.Secret)
	headers.Set("X-Hub-Key", a.cfg.Secret)

	ws, _, err := websocket.Dial(dialCtx, wsURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	cancel()
	if err != nil {
		return fmt.Errorf("dial hub websocket %s: %w", wsURL, err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "agent closed")

	ws.SetReadLimit(yamuxTunnelReadLimit)
	nc := websocket.NetConn(ctx, ws, websocket.MessageBinary)

	ycfg := yamux.DefaultConfig()
	ycfg.EnableKeepAlive = true
	ycfg.KeepAliveInterval = 20 * time.Second
	ycfg.LogOutput = io.Discard

	sess, err := yamux.Server(nc, ycfg)
	if err != nil {
		return fmt.Errorf("yamux server init: %w", err)
	}
	defer sess.Close()

	log.Printf("[inut-bida-agent] established Yamux reverse tunnel: box=%s -> %s", a.cfg.BoxID, wsURL)

	// Push an immediate heartbeat once tunnel is up
	go func() {
		_ = a.PushHeartbeatOnce(ctx)
	}()

	srv := &http.Server{
		Handler:           http.HandlerFunc(a.serveYamuxHTTP),
		ReadHeaderTimeout: 30 * time.Second,
	}
	return srv.Serve(sess)
}

func (a *EdgeAgent) serveYamuxHTTP(w http.ResponseWriter, r *http.Request) {
	// Serve live system metrics directly from the agent when Hub polls /api/system/metrics
	if r.URL.Path == "/api/system/metrics" || r.URL.Path == "/__agent/metrics" {
		payload := a.CollectTelemetry()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}

	targetPort := a.resolveTargetPort(r)
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = "127.0.0.1:" + strconv.Itoa(targetPort)
			req.Host = req.URL.Host
		},
		FlushInterval: -1,
	}
	rp.ServeHTTP(w, r)
}

func (a *EdgeAgent) resolveTargetPort(r *http.Request) int {
	if pStr := strings.TrimSpace(r.Header.Get("X-Target-Port")); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			return a.mapLocalPort(p)
		}
	}
	if svc := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Target-Service"))); svc != "" {
		switch svc {
		case "kspcam", "cam", "edge":
			return a.cfg.KspcamPort
		case "shinobi", "nvr":
			return a.cfg.ShinobiPort
		case "nodered", "node-red", "redbida":
			return a.cfg.NodeRedPort
		}
	}
	if _, portStr, err := net.SplitHostPort(r.Host); err == nil {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			return a.mapLocalPort(p)
		}
	}
	return a.cfg.KspcamPort
}

func (a *EdgeAgent) mapLocalPort(port int) int {
	switch port {
	case 2028:
		return a.cfg.KspcamPort
	case 8080:
		return a.cfg.ShinobiPort
	case 1880, 2023:
		return a.cfg.NodeRedPort
	default:
		return port
	}
}

// PushHeartbeatOnce collects live system telemetry and posts it to /fleet/api/heartbeat.
func (a *EdgeAgent) PushHeartbeatOnce(ctx context.Context) error {
	payload := a.CollectTelemetry()
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	hbURL := strings.TrimRight(a.cfg.HubURL, "/") + "/fleet/api/heartbeat"
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, hbURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Box-ID", a.cfg.BoxID)
	req.Header.Set("X-Box-Secret", a.cfg.Secret)
	req.Header.Set("X-Hub-Key", a.cfg.Secret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("heartbeat status %d", resp.StatusCode)
	}
	return nil
}

// CollectTelemetry gathers real-time Linux kernel & process metrics directly from /proc and /sys.
func (a *EdgeAgent) CollectTelemetry() HeartbeatPayload {
	online := true
	tunnelActive := true
	cores := runtime.NumCPU()
	if cores <= 0 {
		cores = 4
	}

	cpuPct, loadAvg1m := a.sampleCPUAndLoad(cores)
	tempC := sampleThermalTempC()
	ramTotalMB, ramUsedMB, ramAvailMB, ramPct, swapTotalMB, swapUsedMB, swapFreeMB, zramPct := sampleMemAndSwap()
	zramTotalMB := sampleZRAMTotalMB(swapTotalMB)
	zramUsedMB := swapUsedMB

	ffmpegCount, kspcamRSSMB, heapAllocMB, heapSysMB, cameraRSSMB := sampleProcessMetrics()
	sdTotalGB, sdUsedGB, sdUsedPct, sdFSType, sdMountOpts := sampleStorageMetrics()
	rxMbps := a.sampleRXMbps()
	uptimeSec := sampleUptimeSec()
	uptimeHuman := FormatUptimeHuman(uptimeSec)

	payload := HeartbeatPayload{
		BoxID:          a.cfg.BoxID,
		Online:         &online,
		TunnelActive:   &tunnelActive,
		CPUPct:         &cpuPct,
		LoadAvg1m:      &loadAvg1m,
		CPUCores:       cores,
		TempC:          tempC,
		RAMPct:         ramPct,
		RAMTotalMB:     ramTotalMB,
		RAMUsedMB:      ramUsedMB,
		RAMAvailableMB: ramAvailMB,
		RAMUsedPct:     ramPct,
		HeapAllocMB:    &heapAllocMB,
		HeapSysMB:      &heapSysMB,
		KspcamRSSMB:    &kspcamRSSMB,
		CameraRSSMB:    &cameraRSSMB,
		Goroutines:     runtime.NumGoroutine(),
		SwapTotalMB:    swapTotalMB,
		SwapUsedMB:     &swapUsedMB,
		SwapFreeMB:     &swapFreeMB,
		ZRAMTotalMB:    zramTotalMB,
		ZRAMUsedMB:     &zramUsedMB,
		ZRAMPct:        &zramPct,
		SDTotalGB:      sdTotalGB,
		SDUsedGB:       sdUsedGB,
		SDUsedPct:      sdUsedPct,
		SDFSType:       sdFSType,
		SDMountOpts:    sdMountOpts,
		FFmpegCount:    &ffmpegCount,
		RXMbps:         &rxMbps,
		UptimeSec:      uptimeSec,
		UptimeHuman:    uptimeHuman,
	}

	a.mu.Lock()
	a.lastMetrics = payload
	a.mu.Unlock()
	return payload
}

func (a *EdgeAgent) sampleCPUAndLoad(cores int) (cpuPct, loadAvg1m float64) {
	if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) > 0 {
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				loadAvg1m = math.Round(v*100) / 100
			}
		}
	}
	if raw, err := os.ReadFile("/proc/stat"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "cpu ") {
				fields := strings.Fields(line)
				if len(fields) >= 5 {
					var total, idle uint64
					for i := 1; i < len(fields); i++ {
						v, _ := strconv.ParseUint(fields[i], 10, 64)
						total += v
						if i == 4 || i == 5 {
							idle += v
						}
					}
					a.mu.Lock()
					dTotal := total - a.prevCPUTot
					dIdle := idle - a.prevCPUIdle
					hasPrev := a.prevCPUTot > 0 && total >= a.prevCPUTot && idle >= a.prevCPUIdle
					a.prevCPUTot = total
					a.prevCPUIdle = idle
					a.mu.Unlock()

					if hasPrev && dTotal > 0 {
						busy := float64(dTotal-dIdle) * 100.0 / float64(dTotal)
						if busy < 0 {
							busy = 0
						}
						if busy > 100 {
							busy = 100
						}
						cpuPct = math.Round(busy*10) / 10
						return cpuPct, loadAvg1m
					}
				}
				break
			}
		}
	}
	if loadAvg1m > 0 && cores > 0 {
		est := (loadAvg1m / float64(cores)) * 100.0
		if est > 100 {
			est = 100
		}
		cpuPct = math.Round(est*10) / 10
	}
	return cpuPct, loadAvg1m
}

func sampleThermalTempC() float64 {
	for _, p := range []string{
		"/sys/class/thermal/thermal_zone0/temp",
		"/sys/class/hwmon/hwmon0/temp1_input",
	} {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
		if err != nil || v <= 0 {
			continue
		}
		if v > 1000 {
			v /= 1000.0
		}
		return math.Round(v*10) / 10
	}
	return 0
}

func sampleMemAndSwap() (ramTotalMB, ramUsedMB, ramAvailMB, ramPct, swapTotalMB, swapUsedMB, swapFreeMB, zramPct int) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}
	var memTotalKB, memFreeKB, memAvailKB, buffersKB, cachedKB, swapTotalKB, swapFreeKB int64
	hasAvail := false
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			memTotalKB = val
		case "MemFree":
			memFreeKB = val
		case "MemAvailable":
			memAvailKB = val
			hasAvail = true
		case "Buffers":
			buffersKB = val
		case "Cached":
			cachedKB = val
		case "SwapTotal":
			swapTotalKB = val
		case "SwapFree":
			swapFreeKB = val
		}
	}
	if !hasAvail {
		memAvailKB = memFreeKB + buffersKB + cachedKB
	}
	ramTotalMB = int(memTotalKB / 1024)
	ramAvailMB = int(memAvailKB / 1024)
	if ramTotalMB > 0 {
		ramUsedMB = ramTotalMB - ramAvailMB
		if ramUsedMB < 0 {
			ramUsedMB = 0
		}
		ramPct = (ramUsedMB * 100) / ramTotalMB
	}
	swapTotalMB = int(swapTotalKB / 1024)
	swapFreeMB = int(swapFreeKB / 1024)
	if swapTotalMB > 0 {
		swapUsedMB = swapTotalMB - swapFreeMB
		if swapUsedMB < 0 {
			swapUsedMB = 0
		}
		zramPct = (swapUsedMB * 100) / swapTotalMB
	}
	return
}

func sampleZRAMTotalMB(swapTotalMB int) int {
	if raw, err := os.ReadFile("/sys/block/zram0/disksize"); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64); err == nil && v > 0 {
			return int(v / (1024 * 1024))
		}
	}
	return swapTotalMB
}

func sampleProcessMetrics() (ffmpegCount int, kspcamRSSMB, heapAllocMB, heapSysMB, cameraRSSMB float64) {
	entries, err := os.ReadDir("/proc")
	if err == nil {
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			pid := ent.Name()
			if pid[0] < '0' || pid[0] > '9' {
				continue
			}
			cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", pid, "cmdline"))
			if err != nil || len(cmdlineBytes) == 0 {
				continue
			}
			cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
			if strings.Contains(cmdline, "ffmpeg") {
				ffmpegCount++
			}
			if strings.Contains(cmdline, "kspcam") && !strings.Contains(cmdline, "inut-bida-agent") {
				rssKB, dataKB := readProcStatusRSSAndDataKB(pid)
				rssM := math.Round((float64(rssKB)/1024.0)*10) / 10
				if rssM > kspcamRSSMB {
					kspcamRSSMB = rssM
					if dataKB > 0 {
						heapSysMB = math.Round((float64(dataKB)/1024.0)*10) / 10
						heapAllocMB = math.Round(rssM*0.62*10) / 10
					}
				}
			}
			if strings.Contains(cmdline, "camera.js") && !strings.Contains(cmdline, "singleCamera") {
				rssKB, _ := readProcStatusRSSAndDataKB(pid)
				rssM := math.Round((float64(rssKB)/1024.0)*10) / 10
				if rssM > cameraRSSMB {
					cameraRSSMB = rssM
				}
			}
		}
	}
	if kspcamRSSMB <= 0 {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		heapAllocMB = math.Round((float64(ms.HeapAlloc)/(1024*1024))*100) / 100
		heapSysMB = math.Round((float64(ms.HeapSys)/(1024*1024))*100) / 100
		kspcamRSSMB = math.Round((float64(ms.Sys)/(1024*1024))*10) / 10
	}
	return
}

func readProcStatusRSSAndDataKB(pid string) (rssKB, dataKB int64) {
	raw, err := os.ReadFile(filepath.Join("/proc", pid, "status"))
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				rssKB, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		} else if strings.HasPrefix(line, "VmData:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				dataKB, _ = strconv.ParseInt(fields[1], 10, 64)
			}
		}
	}
	return
}

func sampleStorageMetrics() (sdTotalGB, sdUsedGB, sdUsedPct int, sdFSType, sdMountOpts string) {
	targetMount := "/media/usb1"
	if raw, err := os.ReadFile("/proc/mounts"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 4 && fields[1] == "/media/usb1" {
				sdFSType = fields[2]
				sdMountOpts = fields[3]
				break
			}
		}
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(targetMount, &st); err != nil || st.Blocks == 0 {
		targetMount = "/"
		_ = syscall.Statfs(targetMount, &st)
	}
	if st.Blocks > 0 {
		totalBytes := st.Blocks * uint64(st.Bsize)
		availBytes := st.Bavail * uint64(st.Bsize)
		usedBytes := totalBytes - (st.Bfree * uint64(st.Bsize))
		sdTotalGB = int(totalBytes / (1024 * 1024 * 1024))
		sdUsedGB = int(usedBytes / (1024 * 1024 * 1024))
		denom := usedBytes + availBytes
		if denom > 0 {
			sdUsedPct = int((usedBytes * 100) / denom)
		}
	}
	return
}

func (a *EdgeAgent) sampleRXMbps() float64 {
	var rxBytes uint64
	for _, nic := range []string{"end0", "eth0", "eno1", "wlan0"} {
		raw, err := os.ReadFile(filepath.Join("/sys/class/net", nic, "statistics/rx_bytes"))
		if err != nil {
			continue
		}
		if v, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 64); err == nil && v > 0 {
			rxBytes = v
			break
		}
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	var mbps float64
	if a.prevRXBytes > 0 && rxBytes >= a.prevRXBytes && !a.prevRXTime.IsZero() {
		dt := now.Sub(a.prevRXTime).Seconds()
		if dt > 0.05 {
			mbps = math.Round(((float64(rxBytes-a.prevRXBytes)*8.0)/(dt*1_000_000.0))*100) / 100
		}
	}
	a.prevRXBytes = rxBytes
	a.prevRXTime = now
	return mbps
}

func sampleUptimeSec() int64 {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(v)
}

func toWebSocketURL(hubURL, path, boxID string) string {
	base := strings.TrimSpace(hubURL)
	base = strings.TrimRight(base, "/")
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	case !strings.HasPrefix(base, "ws://") && !strings.HasPrefix(base, "wss://"):
		base = "wss://" + base
	}
	return fmt.Sprintf("%s%s?box_id=%s", base, path, boxID)
}
