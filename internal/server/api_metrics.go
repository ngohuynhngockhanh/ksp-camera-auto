package server

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	cpuStatMu    sync.Mutex
	prevCPUTotal uint64
	prevCPUIdle  uint64
)

// SystemMetricsResponse represents the live system & Go runtime telemetry of a kspcam node.
type SystemMetricsResponse struct {
	Online         bool    `json:"online"`
	CPUPct         float64 `json:"cpu_pct"`
	LoadAvg1m      float64 `json:"load_avg_1m"`
	CPUCores       int     `json:"cpu_cores"`
	TempC          float64 `json:"temp_c"`
	RAMTotalMB     int     `json:"ram_total_mb"`
	RAMUsedMB      int     `json:"ram_used_mb"`
	RAMAvailableMB int     `json:"ram_available_mb"`
	RAMUsedPct     int     `json:"ram_used_pct"`
	RAMPct         int     `json:"ram_pct"`
	HeapAllocMB    float64 `json:"heap_alloc_mb"`
	HeapSysMB      float64 `json:"heap_sys_mb"`
	KspcamRSSMB    float64 `json:"kspcam_rss_mb"`
	CameraRSSMB    float64 `json:"camera_rss_mb"`
	Goroutines     int     `json:"goroutines"`
	SwapTotalMB    int     `json:"swap_total_mb"`
	SwapUsedMB     int     `json:"swap_used_mb"`
	SwapFreeMB     int     `json:"swap_free_mb"`
	ZRAMTotalMB    int     `json:"zram_total_mb"`
	ZRAMUsedMB     int     `json:"zram_used_mb"`
	ZRAMPct        int     `json:"zram_pct"`
	SDTotalGB      int     `json:"sd_total_gb"`
	SDUsedGB       int     `json:"sd_used_gb"`
	SDUsedPct      int     `json:"sd_used_pct"`
	FFmpegCount    int     `json:"ffmpeg_count"`
	UptimeSec      int64   `json:"uptime_sec"`
	UptimeHuman    string  `json:"uptime_human"`
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func formatMetricsUptime(sec int64) string {
	if sec <= 0 {
		return "0m"
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

func readProcMeminfo() (ramTotalMB, ramUsedMB, ramAvailMB, ramPct, swapTotalMB, swapUsedMB, swapFreeMB, zramPct int) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0
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

func readProcLoadAndCPU(cores int) (cpuPct, loadAvg1m float64) {
	if raw, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(raw))
		if len(fields) > 0 {
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				loadAvg1m = round2(v)
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
					cpuStatMu.Lock()
					dTotal := total - prevCPUTotal
					dIdle := idle - prevCPUIdle
					hasPrev := prevCPUTotal > 0 && total >= prevCPUTotal && idle >= prevCPUIdle
					prevCPUTotal = total
					prevCPUIdle = idle
					cpuStatMu.Unlock()

					if hasPrev && dTotal > 0 {
						busy := float64(dTotal-dIdle) * 100.0 / float64(dTotal)
						if busy < 0 {
							busy = 0
						}
						if busy > 100 {
							busy = 100
						}
						cpuPct = round1(busy)
						return cpuPct, loadAvg1m
					}
				}
				break
			}
		}
	}
	if cores <= 0 {
		cores = 4
	}
	if loadAvg1m > 0 {
		est := (loadAvg1m / float64(cores)) * 100.0
		if est > 100 {
			est = 100
		}
		cpuPct = round1(est)
	}
	return cpuPct, loadAvg1m
}

func readThermalTempC() float64 {
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
		if v > 200 {
			v = v / 1000.0
		}
		return round1(v)
	}
	return 0
}

func readProcessRSSMB(statusPath string) float64 {
	raw, err := os.ReadFile(statusPath)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseFloat(fields[1], 64); err == nil && kb > 0 {
					return round1(kb / 1024.0)
				}
			}
			break
		}
	}
	return 0
}

func scanProcessesFFmpegAndCameraRSS() (ffmpegCount int, cameraRSSMB float64) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) == 0 || name[0] < '0' || name[0] > '9' {
			continue
		}
		pidDir := filepath.Join("/proc", name)
		cmdRaw, err := os.ReadFile(filepath.Join(pidDir, "cmdline"))
		if err != nil || len(cmdRaw) == 0 {
			continue
		}
		cmd := strings.ReplaceAll(string(cmdRaw), "\x00", " ")
		arg0 := cmd
		if idx := strings.IndexByte(cmd, ' '); idx > 0 {
			arg0 = cmd[:idx]
		}
		base0 := filepath.Base(arg0)
		if base0 == "ffmpeg" || strings.HasPrefix(base0, "ffmpeg") {
			ffmpegCount++
			continue
		}
		if strings.Contains(cmd, "camera.js") {
			rss := readProcessRSSMB(filepath.Join(pidDir, "status"))
			if rss > cameraRSSMB {
				cameraRSSMB = rss
			}
		}
	}
	return ffmpegCount, cameraRSSMB
}

func readStorageUsageGB() (totalGB, usedGB, usedPct int) {
	for _, mount := range []string{"/media/usb1", "/"} {
		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err == nil && st.Blocks > 0 {
			totalBytes := st.Blocks * uint64(st.Bsize)
			availBytes := st.Bavail * uint64(st.Bsize)
			usedBytes := uint64(0)
			if totalBytes > availBytes {
				usedBytes = totalBytes - availBytes
			}
			totalGB = int(totalBytes / (1024 * 1024 * 1024))
			usedGB = int(usedBytes / (1024 * 1024 * 1024))
			if totalBytes > 0 {
				usedPct = int((usedBytes * 100) / totalBytes)
			}
			if totalGB > 0 {
				return totalGB, usedGB, usedPct
			}
		}
	}
	return 0, 0, 0
}

func readUptimeSec() int64 {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || v < 0 {
		return 0
	}
	return int64(v)
}

func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	cores := runtime.NumCPU()
	if cores <= 0 {
		cores = 4
	}
	cpuPct, loadAvg1m := readProcLoadAndCPU(cores)
	ramTotalMB, ramUsedMB, ramAvailMB, ramPct, swapTotalMB, swapUsedMB, swapFreeMB, zramPct := readProcMeminfo()

	heapAllocMB := round1(float64(ms.HeapAlloc) / (1024.0 * 1024.0))
	if heapAllocMB == 0 && ms.HeapAlloc > 0 {
		heapAllocMB = 0.1
	}
	heapSysMB := round1(float64(ms.HeapSys) / (1024.0 * 1024.0))
	if heapSysMB == 0 && ms.HeapSys > 0 {
		heapSysMB = 0.1
	}

	kspcamRSSMB := readProcessRSSMB("/proc/self/status")
	if kspcamRSSMB <= 0 {
		kspcamRSSMB = round1(float64(ms.Sys) / (1024.0 * 1024.0))
	}

	ffmpegCount, cameraRSSMB := scanProcessesFFmpegAndCameraRSS()
	sdTotalGB, sdUsedGB, sdUsedPct := readStorageUsageGB()
	uptimeSec := readUptimeSec()

	resp := SystemMetricsResponse{
		Online:         true,
		CPUPct:         cpuPct,
		LoadAvg1m:      loadAvg1m,
		CPUCores:       cores,
		TempC:          readThermalTempC(),
		RAMTotalMB:     ramTotalMB,
		RAMUsedMB:      ramUsedMB,
		RAMAvailableMB: ramAvailMB,
		RAMUsedPct:     ramPct,
		RAMPct:         ramPct,
		HeapAllocMB:    heapAllocMB,
		HeapSysMB:      heapSysMB,
		KspcamRSSMB:    kspcamRSSMB,
		CameraRSSMB:    cameraRSSMB,
		Goroutines:     runtime.NumGoroutine(),
		SwapTotalMB:    swapTotalMB,
		SwapUsedMB:     swapUsedMB,
		SwapFreeMB:     swapFreeMB,
		ZRAMTotalMB:    swapTotalMB,
		ZRAMUsedMB:     swapUsedMB,
		ZRAMPct:        zramPct,
		SDTotalGB:      sdTotalGB,
		SDUsedGB:       sdUsedGB,
		SDUsedPct:      sdUsedPct,
		FFmpegCount:    ffmpegCount,
		UptimeSec:      uptimeSec,
		UptimeHuman:    formatMetricsUptime(uptimeSec),
	}

	writeJSON(w, http.StatusOK, resp)
}
