package nvrhealth

import (
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

func TestMergeCoverageClipsToBootAndMergesOverlap(t *testing.T) {
	boot := time.Date(2026, 7, 26, 10, 0, 0, 0, time.Local)
	now := boot.Add(2 * time.Hour)
	intervals := []Interval{
		{Start: boot.Add(-time.Hour), End: boot.Add(30 * time.Minute)},
		{Start: boot.Add(20 * time.Minute), End: boot.Add(70 * time.Minute)},
		{Start: boot.Add(90 * time.Minute), End: boot.Add(3 * time.Hour)},
	}
	if got := CoveredDuration(intervals, boot, now); got != 100*time.Minute {
		t.Fatalf("coverage = %v, want 100m", got)
	}
}

func TestClassifyPrioritizesRecordDisabled(t *testing.T) {
	in := Snapshot{Reachable: true, StorageHealthy: true, Channels: []Channel{{Enabled: true, RecordEnabled: false}}}
	status, reasons := Classify(in, time.Now())
	if status != StatusCritical || len(reasons) == 0 || reasons[0].Code != ReasonRecordDisabled {
		t.Fatalf("status=%s reasons=%#v", status, reasons)
	}
}

func TestClassifyRecordEnabledButStaleIsWarning(t *testing.T) {
	now := time.Now()
	in := Snapshot{Reachable: true, StorageHealthy: true, Channels: []Channel{{Enabled: true, RecordEnabled: true, RecordMode: 1, Timing24x7: true, LatestEnd: now.Add(-20 * time.Minute)}}}
	status, reasons := Classify(in, now)
	if status != StatusWarning || reasons[0].Code != ReasonChannelStale {
		t.Fatalf("status=%s reasons=%#v", status, reasons)
	}
}

func TestClassifyAcceptsHDDGrowthWhileActiveClipIsNotClosed(t *testing.T) {
	in := Snapshot{Reachable: true, StorageHealthy: true, StorageGrowing: true, HostTimeTrusted: true, Channels: []Channel{{Enabled: true, RecordEnabled: true, RecordMode: 1, Timing24x7: true}}}
	status, reasons := Classify(in, time.Now())
	if status != StatusHealthy || len(reasons) != 0 {
		t.Fatalf("status=%s reasons=%#v", status, reasons)
	}
}

func TestNextDelayHealthyAndBackoff(t *testing.T) {
	if got := NextDelay(StatusHealthy, 0); got != time.Minute {
		t.Fatalf("healthy delay=%v", got)
	}
	wants := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 10 * time.Minute}
	for i, want := range wants {
		if got := NextDelay(StatusCritical, i); got != want {
			t.Fatalf("attempt %d delay=%v want=%v", i, got, want)
		}
	}
}

func TestTimeSyncDecisionRequiresTrustedHostAndLargeDrift(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	if d := DecideTimeSync(false, now, now.Add(-time.Hour)); d.Sync || d.Reason != ReasonHostTimeUntrusted {
		t.Fatalf("untrusted=%#v", d)
	}
	if d := DecideTimeSync(true, now, now.Add(-30*time.Second)); d.Sync {
		t.Fatalf("small drift=%#v", d)
	}
	if d := DecideTimeSync(true, now, now.Add(-61*time.Second)); !d.Sync {
		t.Fatalf("large drift=%#v", d)
	}
}

func TestStorageHealthIncludesAllPartitions(t *testing.T) {
	devs := []dahua.StorageDevice{{State: "Success", Details: []dahua.StorageDetail{
		{Type: "ReadWrite", TotalBytes: 128_000_000_000}, {Type: "ReadWrite", TotalBytes: 354_000_000_000},
	}}}
	healthy, total, _ := StorageHealth(devs)
	if !healthy || total != 482_000_000_000 {
		t.Fatalf("healthy=%v total=%d", healthy, total)
	}
}
