package nvrhealth

import (
	"sort"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusRepairing Status = "repairing"
	StatusWarning   Status = "warning"
	StatusCritical  Status = "critical"
	StatusUnknown   Status = "unknown"
)

const (
	ReasonRecordDisabled    = "RECORD_DISABLED"
	ReasonScheduleDisabled  = "RECORD_SCHEDULE_DISABLED"
	ReasonRecordModeWrong   = "RECORD_MODE_NOT_MANUAL"
	ReasonChannelStale      = "CHANNEL_STALE"
	ReasonDiskError         = "DISK_ERROR"
	ReasonClockDrift        = "CLOCK_DRIFT"
	ReasonHostTimeUntrusted = "HOST_TIME_UNTRUSTED"
	ReasonCameraDisabled    = "CAMERA_DISABLED"
	ReasonRTSPUnavailable   = "RTSP_UNAVAILABLE"
	ReasonNVRUnreachable    = "NVR_UNREACHABLE"
)

type Reason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Interval struct {
	Start time.Time
	End   time.Time
}
type Channel struct {
	Channel         int       `json:"channel"`
	Name            string    `json:"name,omitempty"`
	Enabled         bool      `json:"enabled"`
	RecordEnabled   bool      `json:"recordEnabled"`
	RecordMode      int       `json:"recordMode"`
	Timing24x7      bool      `json:"timing24x7"`
	LatestEnd       time.Time `json:"latestEnd,omitempty"`
	StaleMinutes    int       `json:"staleMinutes"`
	RecordedMinutes int       `json:"recordedMinutes"`
	CoveragePercent float64   `json:"coveragePercent"`
}
type Snapshot struct {
	Reachable         bool
	StorageHealthy    bool
	StorageGrowing    bool
	ClockDriftSeconds int64
	HostTimeTrusted   bool
	Channels          []Channel
}

func CoveredDuration(in []Interval, start, end time.Time) time.Duration {
	clipped := make([]Interval, 0, len(in))
	for _, x := range in {
		if x.Start.Before(start) {
			x.Start = start
		}
		if x.End.After(end) {
			x.End = end
		}
		if x.End.After(x.Start) {
			clipped = append(clipped, x)
		}
	}
	sort.Slice(clipped, func(i, j int) bool { return clipped[i].Start.Before(clipped[j].Start) })
	var total time.Duration
	for i := 0; i < len(clipped); {
		stop := clipped[i].End
		j := i + 1
		for j < len(clipped) && !clipped[j].Start.After(stop) {
			if clipped[j].End.After(stop) {
				stop = clipped[j].End
			}
			j++
		}
		total += stop.Sub(clipped[i].Start)
		i = j
	}
	return total
}

func Classify(s Snapshot, now time.Time) (Status, []Reason) {
	if !s.Reachable {
		return StatusCritical, []Reason{{ReasonNVRUnreachable, "Không kết nối được đầu ghi."}}
	}
	if !s.StorageHealthy {
		return StatusCritical, []Reason{{ReasonDiskError, "Ổ cứng không ở trạng thái đọc/ghi tốt."}}
	}
	var disabled, scheduleOff, modeWrong, stale, cameraOff bool
	for _, ch := range s.Channels {
		if !ch.Enabled {
			cameraOff = true
			continue
		}
		if !ch.RecordEnabled {
			disabled = true
		}
		if ch.RecordEnabled && !ch.Timing24x7 {
			scheduleOff = true
		}
		if ch.RecordEnabled && ch.RecordMode != 1 {
			modeWrong = true
		}
		if ch.RecordEnabled && !s.StorageGrowing && (ch.LatestEnd.IsZero() || now.Sub(ch.LatestEnd) > 15*time.Minute) {
			stale = true
		}
	}
	if disabled {
		return StatusCritical, []Reason{{ReasonRecordDisabled, "Chế độ ghi hình đang tắt trên ít nhất một kênh."}}
	}
	if scheduleOff {
		return StatusCritical, []Reason{{ReasonScheduleDisabled, "Lịch ghi hình 24/7 đang tắt hoặc sai cấu hình."}}
	}
	if modeWrong {
		return StatusCritical, []Reason{{ReasonRecordModeWrong, "Firmware cần chế độ ghi thủ công liên tục để recorder thực sự chạy."}}
	}
	var reasons []Reason
	if stale {
		reasons = append(reasons, Reason{ReasonChannelStale, "Có kênh chưa tạo clip mới trong 15 phút."})
	}
	if cameraOff {
		reasons = append(reasons, Reason{ReasonCameraDisabled, "Có camera/kênh đang bị tắt."})
	}
	if abs64(s.ClockDriftSeconds) > 60 {
		reasons = append(reasons, Reason{ReasonClockDrift, "Giờ đầu ghi lệch quá 60 giây."})
	}
	if !s.HostTimeTrusted {
		reasons = append(reasons, Reason{ReasonHostTimeUntrusted, "Giờ INUT chưa được NTP xác nhận."})
	}
	if len(reasons) > 0 {
		return StatusWarning, reasons
	}
	return StatusHealthy, nil
}

func NextDelay(status Status, failedRepairs int) time.Duration {
	if status == StatusHealthy {
		return time.Minute
	}
	backoff := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute}
	if failedRepairs < 0 {
		failedRepairs = 0
	}
	if failedRepairs >= len(backoff) {
		failedRepairs = len(backoff) - 1
	}
	return backoff[failedRepairs]
}

type TimeSyncDecision struct {
	Sync         bool
	Reason       string
	DriftSeconds int64
}

func DecideTimeSync(trusted bool, host, device time.Time) TimeSyncDecision {
	drift := int64(host.Sub(device).Seconds())
	if !trusted {
		return TimeSyncDecision{Reason: ReasonHostTimeUntrusted, DriftSeconds: drift}
	}
	return TimeSyncDecision{Sync: abs64(drift) > 60, DriftSeconds: drift}
}
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func StorageHealth(devs []dahua.StorageDevice) (healthy bool, total, used int64) {
	if len(devs) == 0 {
		return false, 0, 0
	}
	healthy = true
	for _, dev := range devs {
		if !strings.EqualFold(dev.State, "Success") || len(dev.Details) == 0 {
			healthy = false
		}
		for _, part := range dev.Details {
			total += part.TotalBytes
			used += part.UsedBytes
			if part.Type != "ReadWrite" || part.IsError || part.IsNeedFormat || part.TotalBytes <= 0 {
				healthy = false
			}
		}
	}
	return
}
