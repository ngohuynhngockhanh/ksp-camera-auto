package server

import (
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/nvrhealth"
)

func TestShouldKickRecorderAllowsFiveMinuteSegmentToClose(t *testing.T) {
	now := time.Date(2026, 7, 27, 22, 0, 0, 0, time.Local)
	if !shouldKickRecorder(time.Time{}, now) {
		t.Fatal("first stale check must kick the recorder")
	}
	if shouldKickRecorder(now.Add(-6*time.Minute-59*time.Second), now) {
		t.Fatal("recorder must not be kicked again inside the cooldown")
	}
	if !shouldKickRecorder(now.Add(-7*time.Minute), now) {
		t.Fatal("recorder should be eligible after seven minutes")
	}
}

func TestEnabledChannelIDs(t *testing.T) {
	channels := []nvrhealth.Channel{
		{Channel: 0, Enabled: true},
		{Channel: 1, Enabled: false},
		{Channel: 3, Enabled: true},
	}
	got := enabledChannelIDs(channels)
	if len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Fatalf("enabledChannelIDs() = %v, want [0 3]", got)
	}
}

func TestStorageRecentlyGrowing(t *testing.T) {
	now := time.Date(2026, 7, 27, 22, 0, 0, 0, time.Local)
	if !storageRecentlyGrowing(now.Add(-119*time.Second), now) {
		t.Fatal("a recent HDD write must protect an active recording")
	}
	if storageRecentlyGrowing(now.Add(-2*time.Minute), now) {
		t.Fatal("two minutes without HDD growth must be treated as stalled")
	}
}

func TestRecordingDiskStalled(t *testing.T) {
	now := time.Date(2026, 7, 27, 22, 0, 0, 0, time.Local)
	report := nvrHealthReport{Reachable: true, StorageHealthy: true, Channels: []nvrhealth.Channel{{Enabled: true, RecordEnabled: true, RecordMode: 1, Timing24x7: true}}}
	if !recordingDiskStalled(report, now.Add(-2*time.Minute), now) {
		t.Fatal("enabled recorder with no disk writes for two minutes must be stalled")
	}
	if recordingDiskStalled(report, now.Add(-time.Minute), now) {
		t.Fatal("one minute without a write is still inside the grace period")
	}
}
