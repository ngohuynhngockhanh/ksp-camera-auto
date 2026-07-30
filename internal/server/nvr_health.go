package server

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/nvrhealth"
)

type nvrHealthReport struct {
	ID                 string              `json:"id"`
	Status             nvrhealth.Status    `json:"status"`
	Reasons            []nvrhealth.Reason  `json:"reasons"`
	Reachable          bool                `json:"reachable"`
	WatchdogEnabled    bool                `json:"watchdogEnabled"`
	SyncTimeFromHost   bool                `json:"syncTimeFromHost"`
	HostTime           string              `json:"hostTime"`
	HostTimeTrusted    bool                `json:"hostTimeTrusted"`
	NVRTime            string              `json:"nvrTime,omitempty"`
	ClockDriftSeconds  int64               `json:"clockDriftSeconds"`
	UptimeMinutes      int64               `json:"uptimeMinutes,omitempty"`
	BootTime           string              `json:"bootTime,omitempty"`
	CoverageIncomplete bool                `json:"coverageIncomplete,omitempty"`
	StorageHealthy     bool                `json:"storageHealthy"`
	StorageTotalBytes  int64               `json:"storageTotalBytes"`
	StorageUsedBytes   int64               `json:"storageUsedBytes"`
	StorageGrowing     bool                `json:"storageGrowing"`
	Channels           []nvrhealth.Channel `json:"channels"`
	LastCheck          string              `json:"lastCheck"`
	LastRepair         string              `json:"lastRepair,omitempty"`
	NextCheck          string              `json:"nextCheck,omitempty"`
	LastError          string              `json:"lastError,omitempty"`
}

type nvrRuntime struct {
	report        nvrHealthReport
	failedRepairs int
	checking      bool
	previousUsed  int64
	lastKick      time.Time
	lastDiskWrite time.Time
}
type nvrWatchdog struct {
	server *Server
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	states map[string]*nvrRuntime
	wake   chan string
}

func newNVRWatchdog(s *Server) *nvrWatchdog {
	ctx, cancel := context.WithCancel(context.Background())
	w := &nvrWatchdog{server: s, ctx: ctx, cancel: cancel, states: map[string]*nvrRuntime{}, wake: make(chan string, 32)}
	go w.loop()
	return w
}
func (w *nvrWatchdog) stop() { w.cancel() }
func (w *nvrWatchdog) trigger(id string) {
	select {
	case w.wake <- id:
	default:
	}
}

func (w *nvrWatchdog) loop() {
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case id := <-w.wake:
			go w.check(id, true)
		case <-timer.C:
			now := time.Now()
			for _, d := range w.server.inv.List() {
				if !d.IsNVR || !d.NVRWatchdog {
					continue
				}
				w.mu.RLock()
				st := w.states[d.ID]
				due := st == nil || st.report.NextCheck == ""
				if st != nil && st.report.NextCheck != "" {
					t, _ := time.Parse(time.RFC3339, st.report.NextCheck)
					due = !t.After(now)
				}
				w.mu.RUnlock()
				if due {
					go w.check(d.ID, false)
				}
			}
			timer.Reset(15 * time.Second)
		}
	}
}

func (w *nvrWatchdog) get(id string) (nvrHealthReport, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	st, ok := w.states[id]
	if !ok {
		return nvrHealthReport{}, false
	}
	return st.report, true
}

func (w *nvrWatchdog) check(id string, manual bool) {
	w.mu.Lock()
	st := w.states[id]
	if st == nil {
		st = &nvrRuntime{}
		w.states[id] = st
	}
	if st.checking {
		w.mu.Unlock()
		return
	}
	st.checking = true
	previousUsed := st.previousUsed
	failed := st.failedRepairs
	lastKick := st.lastKick
	lastDiskWrite := st.lastDiskWrite
	w.mu.Unlock()
	defer func() { w.mu.Lock(); st.checking = false; w.mu.Unlock() }()
	d, ok := w.server.inv.Get(id)
	if !ok || !d.IsNVR {
		return
	}
	report := w.inspect(d, previousUsed)
	now := time.Now()
	if previousUsed == 0 || report.StorageUsedBytes != previousUsed {
		lastDiskWrite = now
	}
	if lastDiskWrite.IsZero() {
		lastDiskWrite = now
	}
	if report.StorageGrowing {
		// A successful disk write proves the previous kick worked. Clear its
		// cooldown so a later reboot can be repaired without waiting seven minutes.
		lastKick = time.Time{}
	}
	if hasReason(report.Reasons, nvrhealth.ReasonChannelStale) && storageRecentlyGrowing(lastDiskWrite, now) {
		report.Reasons = withoutReason(report.Reasons, nvrhealth.ReasonChannelStale)
		if len(report.Reasons) == 0 {
			report.Status = nvrhealth.StatusHealthy
		}
	}
	needsConfigRepair := hasReason(report.Reasons, nvrhealth.ReasonRecordDisabled) ||
		hasReason(report.Reasons, nvrhealth.ReasonScheduleDisabled) ||
		hasReason(report.Reasons, nvrhealth.ReasonRecordModeWrong)
	if !needsConfigRepair && recordingDiskStalled(report, lastDiskWrite, now) {
		if !hasReason(report.Reasons, nvrhealth.ReasonChannelStale) {
			report.Reasons = append(report.Reasons, nvrhealth.Reason{Code: nvrhealth.ReasonChannelStale, Message: "HDD không tăng dữ liệu trong 2 phút dù ghi hình đang bật."})
		}
		report.Status = nvrhealth.StatusWarning
	}
	needsRecorderKick := hasReason(report.Reasons, nvrhealth.ReasonChannelStale)
	if d.NVRWatchdog && needsConfigRepair {
		report.Status = nvrhealth.StatusRepairing
		if err := w.repair(d, report.Channels); err != nil {
			report.LastError = err.Error()
			failed++
		} else {
			report.LastRepair = time.Now().Format(time.RFC3339)
			failed = 0
			report = w.inspect(d, report.StorageUsedBytes)
			report.LastRepair = time.Now().Format(time.RFC3339)
		}
	} else if d.NVRWatchdog && needsRecorderKick && shouldKickRecorder(lastKick, time.Now()) {
		repairAt := time.Now()
		report.Status = nvrhealth.StatusRepairing
		if err := w.restartRecording(d, report.Channels); err != nil {
			report.LastError = "không kích lại được recorder: " + err.Error()
			failed++
		} else {
			lastKick = repairAt
			lastDiskWrite = repairAt
			report.LastRepair = repairAt.Format(time.RFC3339)
			report.LastError = "đã kích lại recorder; đang chờ clip mới hoặc dung lượng HDD tăng"
			failed = 0
		}
	} else if report.Status == nvrhealth.StatusHealthy {
		failed = 0
	}
	delay := nvrhealth.NextDelay(report.Status, failed)
	if manual && !d.NVRWatchdog {
		report.NextCheck = ""
	} else {
		report.NextCheck = time.Now().Add(delay).Format(time.RFC3339)
	}
	w.mu.Lock()
	st.report = report
	st.failedRepairs = failed
	st.previousUsed = report.StorageUsedBytes
	st.lastKick = lastKick
	st.lastDiskWrite = lastDiskWrite
	w.mu.Unlock()
}

func storageRecentlyGrowing(lastWrite, now time.Time) bool {
	return !lastWrite.IsZero() && now.Sub(lastWrite) < 2*time.Minute
}

func recordingDiskStalled(report nvrHealthReport, lastWrite, now time.Time) bool {
	if !report.Reachable || !report.StorageHealthy || lastWrite.IsZero() || now.Sub(lastWrite) < 2*time.Minute {
		return false
	}
	for _, channel := range report.Channels {
		if channel.Enabled && channel.RecordEnabled && channel.RecordMode == 1 && channel.Timing24x7 {
			return true
		}
	}
	return false
}

func withoutReason(reasons []nvrhealth.Reason, code string) []nvrhealth.Reason {
	out := make([]nvrhealth.Reason, 0, len(reasons))
	for _, reason := range reasons {
		if reason.Code != code {
			out = append(out, reason)
		}
	}
	return out
}

func shouldKickRecorder(lastKick, now time.Time) bool {
	// Recording segments are five minutes long. Leave an extra two minutes so
	// an active segment can close and become visible before another mode edge.
	return lastKick.IsZero() || now.Sub(lastKick) >= 7*time.Minute
}

func (w *nvrWatchdog) inspect(d config.Device, previousUsed int64) nvrHealthReport {
	now := time.Now()
	trusted := hostTimeTrusted(w.ctx)
	r := nvrHealthReport{ID: d.ID, Status: nvrhealth.StatusUnknown, WatchdogEnabled: d.NVRWatchdog, SyncTimeFromHost: d.NVRSyncTimeFromHost, HostTime: now.Format("2006-01-02 15:04:05"), HostTimeTrusted: trusted, LastCheck: now.Format(time.RFC3339)}
	ctx, cancel := context.WithTimeout(w.ctx, w.server.reqTimeout(0))
	defer cancel()
	cam, err := camera.Open(ctx, d, w.server.reqTimeout(0))
	if err != nil {
		r.Status = nvrhealth.StatusCritical
		r.Reasons = []nvrhealth.Reason{{Code: nvrhealth.ReasonNVRUnreachable, Message: err.Error()}}
		r.LastError = err.Error()
		return r
	}
	defer cam.Close()
	r.Reachable = true
	health, ok := cam.(camera.NVRHealthConfig)
	if !ok {
		r.LastError = "firmware không hỗ trợ kiểm tra cấu hình ghi hình"
		return r
	}
	remotes, _ := cam.(camera.RemoteDeviceLister).GetRemoteDevices(ctx)
	channelCount := 0
	for _, x := range remotes {
		if x.Channel+1 > channelCount {
			channelCount = x.Channel + 1
		}
	}
	states, err := health.GetRecordState(ctx, channelCount)
	if err != nil {
		r.LastError = err.Error()
		return r
	}
	storage, err := cam.(camera.StorageManager).GetStorageInfo(ctx)
	if err == nil {
		r.StorageHealthy, r.StorageTotalBytes, r.StorageUsedBytes = nvrhealth.StorageHealth(storage)
		r.StorageGrowing = previousUsed > 0 && r.StorageUsedBytes > previousUsed
	}
	uptime, upErr := health.GetUptime(ctx)
	if upErr == nil {
		r.UptimeMinutes = int64(uptime.Minutes())
		boot := now.Add(-uptime)
		r.BootTime = boot.Format(time.RFC3339)
		r.CoverageIncomplete = uptime > 30*24*time.Hour
	} else {
		r.LastError = "không đọc được uptime: " + upErr.Error()
	}
	deviceTime := now
	if tc, ok := cam.(camera.DeviceTimeConfig); ok {
		if cfg, e := tc.GetTimeConfig(ctx); e == nil {
			r.NVRTime = cfg.CurrentTime
			if parsed, e := time.ParseInLocation("2006-01-02 15:04:05", cfg.CurrentTime, time.Local); e == nil {
				deviceTime = parsed
				r.ClockDriftSeconds = int64(deviceTime.Sub(now).Seconds())
				if d.NVRSyncTimeFromHost {
					decision := nvrhealth.DecideTimeSync(trusted, now, deviceTime)
					if decision.Sync {
						_ = tc.SetTimeConfig(ctx, dahua.TimeConfig{CurrentTime: now.Format("2006-01-02 15:04:05"), NTPEnable: true, NTPAddress: "0.vn.pool.ntp.org", NTPPort: 123, UpdatePeriod: 60, TimeZone: 12, TimeZoneDesc: "Bangkok"})
						r.NVRTime = now.Format("2006-01-02 15:04:05")
						r.ClockDriftSeconds = 0
					}
				}
			}
		}
	}
	remoteByChannel := map[int]dahua.RemoteChannel{}
	for _, x := range remotes {
		remoteByChannel[x.Channel] = x
	}
	boot := now.Add(-time.Duration(r.UptimeMinutes) * time.Minute)
	if r.UptimeMinutes <= 0 {
		boot = now.Add(-24 * time.Hour)
	}
	windowStart := boot
	if windowStart.Before(now.Add(-30 * 24 * time.Hour)) {
		windowStart = now.Add(-30 * 24 * time.Hour)
	}
	for _, state := range states {
		remote, exists := remoteByChannel[state.Channel]
		ch := nvrhealth.Channel{Channel: state.Channel, Name: remote.Name, Enabled: !exists || remote.Enable, RecordEnabled: state.Enable, RecordMode: state.Mode}
		ch.Timing24x7 = state.Timing24x7
		if rec, ok := cam.(camera.Recorder); ok {
			if files, e := rec.FindRecordings(ctx, state.Channel, windowStart, now); e == nil {
				intervals := make([]nvrhealth.Interval, 0, len(files))
				for _, f := range files {
					s, e1 := parseDeviceTimes(f)
					if !e1.IsZero() {
						if e1.After(ch.LatestEnd) {
							ch.LatestEnd = e1
						}
						intervals = append(intervals, nvrhealth.Interval{Start: s, End: e1})
					}
				}
				covered := nvrhealth.CoveredDuration(intervals, boot, now)
				ch.RecordedMinutes = int(covered.Minutes())
				if r.UptimeMinutes > 0 {
					ch.CoveragePercent = float64(ch.RecordedMinutes) * 100 / float64(r.UptimeMinutes)
				}
				if !ch.LatestEnd.IsZero() {
					ch.StaleMinutes = int(now.Sub(ch.LatestEnd).Minutes())
				}
			}
		}
		r.Channels = append(r.Channels, ch)
	}
	snap := nvrhealth.Snapshot{Reachable: true, StorageHealthy: r.StorageHealthy, StorageGrowing: r.StorageGrowing, ClockDriftSeconds: r.ClockDriftSeconds, HostTimeTrusted: trusted, Channels: r.Channels}
	r.Status, r.Reasons = nvrhealth.Classify(snap, now)
	return r
}

func parseDeviceTimes(rec dahua.Recording) (time.Time, time.Time) {
	s, _ := time.ParseInLocation("2006-01-02 15:04:05", rec.StartTime, time.Local)
	e, _ := time.ParseInLocation("2006-01-02 15:04:05", rec.EndTime, time.Local)
	return s, e
}
func hasReason(rs []nvrhealth.Reason, code string) bool {
	for _, r := range rs {
		if r.Code == code {
			return true
		}
	}
	return false
}
func (w *nvrWatchdog) repair(d config.Device, channels []nvrhealth.Channel) error {
	ctx, cancel := context.WithTimeout(w.ctx, w.server.reqTimeout(0))
	defer cancel()
	cam, err := camera.Open(ctx, d, w.server.reqTimeout(0))
	if err != nil {
		return err
	}
	defer cam.Close()
	hc, ok := cam.(camera.NVRHealthConfig)
	if !ok {
		return fmt.Errorf("NVR không hỗ trợ sửa cấu hình ghi")
	}
	ids := []int{}
	for _, ch := range channels {
		if ch.Enabled {
			ids = append(ids, ch.Channel)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("không có kênh đang bật")
	}
	if err := hc.EnableTimingRecord(ctx, ids); err != nil {
		return err
	}
	states, err := hc.GetRecordState(ctx, len(channels))
	if err != nil {
		return err
	}
	for _, s := range states {
		if containsInt(ids, s.Channel) && (!s.Enable || s.Mode != 1 || !s.Timing24x7) {
			return fmt.Errorf("kênh %d không nhận cấu hình ghi", s.Channel+1)
		}
	}
	return nil
}

func (w *nvrWatchdog) restartRecording(d config.Device, channels []nvrhealth.Channel) error {
	ctx, cancel := context.WithTimeout(w.ctx, w.server.reqTimeout(0)+5*time.Second)
	defer cancel()
	cam, err := camera.Open(ctx, d, w.server.reqTimeout(0))
	if err != nil {
		return err
	}
	defer cam.Close()
	hc, ok := cam.(camera.NVRHealthConfig)
	if !ok {
		return fmt.Errorf("NVR không hỗ trợ kích lại recorder")
	}
	ids := enabledChannelIDs(channels)
	if len(ids) == 0 {
		return fmt.Errorf("không có kênh đang bật")
	}
	return hc.RestartRecording(ctx, ids)
}

func enabledChannelIDs(channels []nvrhealth.Channel) []int {
	ids := make([]int, 0, len(channels))
	for _, ch := range channels {
		if ch.Enabled {
			ids = append(ids, ch.Channel)
		}
	}
	return ids
}
func containsInt(xs []int, n int) bool {
	for _, x := range xs {
		if x == n {
			return true
		}
	}
	return false
}

func hostTimeTrusted(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "timedatectl", "show", "-p", "NTPSynchronized", "--value").Output()
	if err != nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(string(out)))
	if v == "yes" || v == "1" {
		return true
	}
	b, _ := strconv.ParseBool(v)
	return b
}

func (w *nvrWatchdog) logState(id string) {
	if r, ok := w.get(id); ok {
		log.Printf("nvr watchdog %s: %s (%s)", id, r.Status, r.LastError)
	}
}
