package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// mockCamera implements camera.Camera, camera.PictureSettings, and camera.Rebooter for testing.
type mockCamera struct {
	deviceID              string
	color                 map[string]any
	options               map[string]any
	caps                  map[string]any
	rebootErr             error
	setPicErr             error
	getPicErr             error
	closed                bool
	rebootDone            bool
	ignoreDayNightChanges bool
	mu                    sync.Mutex
}

func (m *mockCamera) Probe(ctx context.Context) ([]camera.StreamInfo, error) { return nil, nil }
func (m *mockCamera) Apply(ctx context.Context, profile camera.Profile, emit func(camera.StepResult)) []camera.StepResult {
	return nil
}
func (m *mockCamera) ChangePassword(ctx context.Context, newUser, newPass string) error { return nil }
func (m *mockCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return nil, nil
}
func (m *mockCamera) ChannelInfo(ctx context.Context, channel int) (name string, osdLines []string, osdEnabled []bool, osdSupported bool, err error) {
	return "", nil, nil, true, nil
}
func (m *mockCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return nil
}
func (m *mockCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (applied int, err error) {
	return 0, nil
}
func (m *mockCamera) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockCamera) GetPicture(ctx context.Context, channel int) (map[string]any, map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getPicErr != nil {
		return nil, nil, m.getPicErr
	}
	optCopy := make(map[string]any)
	for k, v := range m.options {
		optCopy[k] = v
	}
	return m.color, optCopy, nil
}

func (m *mockCamera) SetPicture(ctx context.Context, channel int, colorChanges, optionsChanges map[string]any) (map[string]any, map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.setPicErr != nil {
		return nil, nil, m.setPicErr
	}
	if m.options == nil {
		m.options = make(map[string]any)
	}
	for k, v := range optionsChanges {
		if k == "DayNightColor" && m.ignoreDayNightChanges {
			continue
		}
		m.options[k] = v
	}
	optCopy := make(map[string]any)
	for k, v := range m.options {
		optCopy[k] = v
	}
	return m.color, optCopy, nil
}

func (m *mockCamera) GetPictureCaps(ctx context.Context, channel int) (map[string]any, error) {
	return m.caps, nil
}

func (m *mockCamera) Reboot(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rebootErr != nil {
		return m.rebootErr
	}
	m.rebootDone = true
	return nil
}

// mockPlainCamera implements only camera.Camera (no PictureSettings, no Rebooter).
type mockPlainCamera struct{}

func (m *mockPlainCamera) Probe(ctx context.Context) ([]camera.StreamInfo, error) { return nil, nil }
func (m *mockPlainCamera) Apply(ctx context.Context, profile camera.Profile, emit func(camera.StepResult)) []camera.StepResult {
	return nil
}
func (m *mockPlainCamera) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return nil
}
func (m *mockPlainCamera) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return nil, nil
}
func (m *mockPlainCamera) ChannelInfo(ctx context.Context, channel int) (name string, osdLines []string, osdEnabled []bool, osdSupported bool, err error) {
	return "", nil, nil, true, nil
}
func (m *mockPlainCamera) SetChannelName(ctx context.Context, channel int, name string) error {
	return nil
}
func (m *mockPlainCamera) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (applied int, err error) {
	return 0, nil
}
func (m *mockPlainCamera) Close() error { return nil }

func newTestInventory(t *testing.T) *config.Inventory {
	t.Helper()
	tmpFile := filepath.Join(t.TempDir(), "cameras.yaml")
	inv, err := config.LoadInventory(tmpFile)
	if err != nil {
		t.Fatalf("load test inventory: %v", err)
	}

	_ = inv.Upsert(config.Device{
		ID:     "192.168.1.201:37777",
		Name:   "Camera01",
		Host:   "192.168.1.201",
		Port:   37777,
		Vendor: config.VendorDahua,
	})
	_ = inv.Upsert(config.Device{
		ID:     "192.168.1.202:37777",
		Name:   "Camera02",
		Host:   "192.168.1.202",
		Port:   37777,
		Vendor: config.VendorDahua,
	})
	_ = inv.Upsert(config.Device{
		ID:     "192.168.1.203:37777",
		Name:   "Camera03",
		Host:   "192.168.1.203",
		Port:   37777,
		Vendor: config.VendorDahua,
	})
	return inv
}

func TestActionRegistryAndParse(t *testing.T) {
	actions := RegisteredActions()
	if len(actions) == 0 {
		t.Fatal("expected registered actions, got 0")
	}

	// 1. Parse PictureDayNightAction directly
	raw := json.RawMessage(`{"deviceIds": ["192.168.1.202:37777"], "channel": 0, "dayNightColor": 0}`)
	act, err := ParseAction(ActionTypePictureDayNight, raw)
	if err != nil {
		t.Fatalf("ParseAction picture_day_night: %v", err)
	}
	if act.Type() != ActionTypePictureDayNight {
		t.Fatalf("unexpected type: %s", act.Type())
	}
	pdn, ok := act.(*PictureDayNightAction)
	if !ok || pdn.DayNightColor != 0 || len(pdn.DeviceIDs) != 1 {
		t.Fatalf("unexpected action content: %+v", act)
	}

	// 2. Parse from polymorphic payload with type inferred
	wrappedRaw := json.RawMessage(`{"type": "reboot", "params": {"deviceIds": ["192.168.1.202:37777"]}}`)
	act2, err := ParseAction("", wrappedRaw)
	if err != nil {
		t.Fatalf("ParseAction wrapped reboot: %v", err)
	}
	if act2.Type() != ActionTypeReboot {
		t.Fatalf("unexpected type: %s", act2.Type())
	}

	// 3. Unknown action type
	_, err = ParseAction("non_existent_action", nil)
	if err == nil {
		t.Fatal("expected error for unknown action type")
	}
}

func TestPictureDayNightActionValidationAndSummary(t *testing.T) {
	tests := []struct {
		name    string
		act     PictureDayNightAction
		wantErr bool
	}{
		{
			name:    "valid full color",
			act:     PictureDayNightAction{DeviceIDs: []string{"cam1"}, Channel: 0, DayNightColor: 0},
			wantErr: false,
		},
		{
			name:    "valid auto",
			act:     PictureDayNightAction{DeviceIDs: []string{"cam1"}, Channel: 1, DayNightColor: 1},
			wantErr: false,
		},
		{
			name:    "valid b&w",
			act:     PictureDayNightAction{DeviceIDs: []string{"cam1"}, Channel: 0, DayNightColor: 2},
			wantErr: false,
		},
		{
			name:    "empty devices",
			act:     PictureDayNightAction{DeviceIDs: nil, Channel: 0, DayNightColor: 0},
			wantErr: true,
		},
		{
			name:    "blank device id",
			act:     PictureDayNightAction{DeviceIDs: []string{" "}, Channel: 0, DayNightColor: 0},
			wantErr: true,
		},
		{
			name:    "invalid negative channel",
			act:     PictureDayNightAction{DeviceIDs: []string{"cam1"}, Channel: -1, DayNightColor: 0},
			wantErr: true,
		},
		{
			name:    "invalid color mode",
			act:     PictureDayNightAction{DeviceIDs: []string{"cam1"}, Channel: 0, DayNightColor: 5},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.act.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}

	act := PictureDayNightAction{DeviceIDs: []string{"c1", "c2"}, Channel: 0, DayNightColor: 0}
	if act.Summary() == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestPictureDayNightActionExecute(t *testing.T) {
	inv := newTestInventory(t)

	// Mock camera map
	cams := map[string]*mockCamera{
		"192.168.1.202:37777": {
			deviceID: "192.168.1.202:37777",
			options:  map[string]any{"DayNightColor": 1},
		},
		"192.168.1.203:37777": {
			deviceID: "192.168.1.203:37777",
			options:  map[string]any{"DayNightColor": 1},
		},
	}

	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		if cam, ok := cams[d.ID]; ok {
			return cam, nil
		}
		return nil, errors.New("connection refused")
	})
	defer ResetCameraOpener()

	// 1. Success execution & read-back verification
	act := PictureDayNightAction{
		DeviceIDs:     []string{"192.168.1.202:37777", "Camera03"},
		Channel:       0,
		DayNightColor: DayNightFullColor, // 0
	}

	summary, err := act.Execute(context.Background(), inv, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if summary == "" {
		t.Fatal("expected summary, got empty")
	}

	// Verify both cameras updated and read-back verified
	if cams["192.168.1.202:37777"].options["DayNightColor"] != DayNightFullColor {
		t.Fatalf("cam 2 DayNightColor = %v, want 0", cams["192.168.1.202:37777"].options["DayNightColor"])
	}
	if cams["192.168.1.203:37777"].options["DayNightColor"] != DayNightFullColor {
		t.Fatalf("cam 3 DayNightColor = %v, want 0", cams["192.168.1.203:37777"].options["DayNightColor"])
	}

	// 2. Read-back verification mismatch test
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		return &mockCamera{
			deviceID:              d.ID,
			options:               map[string]any{"DayNightColor": 2}, // returns B&W
			ignoreDayNightChanges: true,
		}, nil
	})

	_, err = act.Execute(context.Background(), inv, 5*time.Second)
	if err == nil {
		t.Fatal("expected error on read-back verification mismatch")
	}

	// 3. Unsupported camera (not implementing PictureSettings)
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		return &mockPlainCamera{}, nil
	})
	_, err = act.Execute(context.Background(), inv, 5*time.Second)
	if err == nil {
		t.Fatal("expected error for camera not implementing PictureSettings")
	}

	// 4. Missing device in inventory
	actMissing := PictureDayNightAction{
		DeviceIDs:     []string{"192.168.1.250:37777"},
		Channel:       0,
		DayNightColor: DayNightAuto,
	}
	_, err = actMissing.Execute(context.Background(), inv, 5*time.Second)
	if err == nil {
		t.Fatal("expected error when device not found in inventory")
	}
}

func TestRebootActionExecute(t *testing.T) {
	inv := newTestInventory(t)

	cam := &mockCamera{deviceID: "192.168.1.202:37777"}
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		return cam, nil
	})
	defer ResetCameraOpener()

	act := RebootAction{DeviceIDs: []string{"Camera02"}}
	if err := act.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	summary, err := act.Execute(context.Background(), inv, 5*time.Second)
	if err != nil {
		t.Fatalf("Reboot execute failed: %v", err)
	}
	if summary == "" || !cam.rebootDone {
		t.Fatalf("expected reboot done, got summary=%s, done=%v", summary, cam.rebootDone)
	}

	// Reboot error test
	cam.rebootErr = errors.New("simulated reboot error")
	_, err = act.Execute(context.Background(), inv, 5*time.Second)
	if err == nil {
		t.Fatal("expected error on reboot failure")
	}
}

func TestScheduleValidationAndNextRun(t *testing.T) {
	// 1. Validation
	validSched := Schedule{Time: "07:00", DaysOfWeek: []int{1, 2, 3}}
	if err := validSched.Validate(); err != nil {
		t.Fatalf("valid schedule failed: %v", err)
	}

	invalidSched := Schedule{Time: "25:00"}
	if err := invalidSched.Validate(); err == nil {
		t.Fatal("expected error for 25:00")
	}

	// 2. NextRun daily (same day before target time)
	daily := Schedule{Time: "07:00"}
	base := time.Date(2026, 11, 16, 6, 30, 0, 0, time.UTC) // Monday 06:30
	next := daily.NextRun(base)
	want := time.Date(2026, 11, 16, 7, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("NextRun = %v, want %v", next, want)
	}

	// 3. NextRun daily (same day after target time -> rolls to tomorrow)
	baseAfter := time.Date(2026, 11, 16, 7, 0, 0, 0, time.UTC) // exactly 07:00
	nextTomorrow := daily.NextRun(baseAfter)
	wantTomorrow := time.Date(2026, 11, 17, 7, 0, 0, 0, time.UTC)
	if !nextTomorrow.Equal(wantTomorrow) {
		t.Fatalf("NextRun = %v, want %v", nextTomorrow, wantTomorrow)
	}

	// 4. NextRun with days of week filter (Mon, Wed, Fri = 1, 3, 5)
	mwf := Schedule{Time: "17:00", DaysOfWeek: []int{1, 3, 5}}
	// Monday at 18:00 -> should jump to Wednesday 17:00
	nextWed := mwf.NextRun(time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC))
	wantWed := time.Date(2026, 11, 18, 17, 0, 0, 0, time.UTC)
	if !nextWed.Equal(wantWed) {
		t.Fatalf("NextRun MWF = %v, want %v", nextWed, wantWed)
	}

	// 5. Schedule unmarshal with string day names and cron 7
	raw := []byte(`{"time": "08:30", "daysOfWeek": ["mon", "friday", 7]}`)
	var s Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("Unmarshal schedule: %v", err)
	}
	// 7 maps to 0 (Sunday), mon=1, friday=5 -> [0, 1, 5]
	if len(s.DaysOfWeek) != 3 || s.DaysOfWeek[0] != 0 || s.DaysOfWeek[1] != 1 || s.DaysOfWeek[2] != 5 {
		t.Fatalf("unexpected parsed days: %v", s.DaysOfWeek)
	}

	// 6. Summary strings
	if daily.Summary() != "Daily at 07:00" {
		t.Fatalf("summary = %s", daily.Summary())
	}
}

func TestJobJSONPolymorphism(t *testing.T) {
	orig := &Job{
		ID:         "test-job-1",
		Name:       "Test Color Job",
		Enabled:    true,
		Schedule:   Schedule{Time: "07:00"},
		ActionType: ActionTypePictureDayNight,
		Action: &PictureDayNightAction{
			DeviceIDs:     []string{"192.168.1.202:37777"},
			Channel:       0,
			DayNightColor: DayNightFullColor,
		},
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal Job: %v", err)
	}

	var loaded Job
	if err := json.Unmarshal(b, &loaded); err != nil {
		t.Fatalf("Unmarshal Job: %v", err)
	}

	if loaded.ID != orig.ID || loaded.Name != orig.Name || loaded.ActionType != orig.ActionType {
		t.Fatalf("mismatch: %+v vs %+v", loaded, orig)
	}
	pdn, ok := loaded.Action.(*PictureDayNightAction)
	if !ok || pdn.DayNightColor != DayNightFullColor || len(pdn.DeviceIDs) != 1 {
		t.Fatalf("action unmarshaled incorrectly: %+v", loaded.Action)
	}
}

func TestSchedulerDefaultSeedingAndCRUD(t *testing.T) {
	inv := newTestInventory(t)
	tmpFile := filepath.Join(t.TempDir(), "schedules.json")

	// 1. Initial creation on empty store -> starts clean with 0 jobs (never auto-seeds arbitrary jobs across fleet)
	sched, err := NewScheduler(tmpFile, inv)
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	jobs := sched.ListJobs()
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs in clean empty store, got %d", len(jobs))
	}

	// 1b. Explicitly seed jobs for 4.168
	if err := sched.SeedDefault4168Jobs(); err != nil {
		t.Fatalf("SeedDefault4168Jobs: %v", err)
	}
	if len(sched.ListJobs()) != 2 {
		t.Fatalf("expected 2 seeded default jobs, got %d", len(sched.ListJobs()))
	}
	job1, ok := sched.GetJob("job-0700-fullcolor")
	if !ok {
		t.Fatal("job-0700-fullcolor missing")
	}
	if job1.Schedule.Time != "07:00" || !job1.Enabled {
		t.Fatalf("unexpected job1 schedule: %+v", job1)
	}
	act1, ok := job1.Action.(*PictureDayNightAction)
	if !ok || act1.DayNightColor != DayNightFullColor {
		t.Fatalf("unexpected job1 action: %+v", job1.Action)
	}

	// Verify Job 2: 17:00 Cam 2 & 3 Auto
	job2, ok := sched.GetJob("job-1700-auto")
	if !ok {
		t.Fatal("job-1700-auto missing")
	}
	if job2.Schedule.Time != "17:00" || !job2.Enabled {
		t.Fatalf("unexpected job2 schedule: %+v", job2)
	}
	act2, ok := job2.Action.(*PictureDayNightAction)
	if !ok || act2.DayNightColor != DayNightAuto {
		t.Fatalf("unexpected job2 action: %+v", job2.Action)
	}

	// 2. AddJob
	newJob := &Job{
		ID:       "custom-job",
		Name:     "Custom Reboot",
		Enabled:  true,
		Schedule: Schedule{Time: "03:00"},
		Action:   &RebootAction{DeviceIDs: []string{"Camera02"}},
	}
	if err := sched.AddJob(newJob); err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if len(sched.ListJobs()) != 3 {
		t.Fatalf("expected 3 jobs after add, got %d", len(sched.ListJobs()))
	}

	// 3. UpdateJob
	newJob.Name = "Updated Reboot Name"
	if err := sched.UpdateJob(newJob); err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	updated, _ := sched.GetJob("custom-job")
	if updated.Name != "Updated Reboot Name" {
		t.Fatalf("updated name mismatch: %s", updated.Name)
	}

	// 4. ToggleJob
	toggled, err := sched.ToggleJob("custom-job")
	if err != nil {
		t.Fatalf("ToggleJob: %v", err)
	}
	if toggled.Enabled {
		t.Fatal("expected job to be disabled after toggle")
	}
	toggledBack, err := sched.ToggleJob("custom-job", true)
	if err != nil {
		t.Fatalf("ToggleJob explicit: %v", err)
	}
	if !toggledBack.Enabled {
		t.Fatal("expected job to be enabled after explicit toggle")
	}

	// 5. DeleteJob
	if err := sched.DeleteJob("custom-job"); err != nil {
		t.Fatalf("DeleteJob: %v", err)
	}
	if len(sched.ListJobs()) != 2 {
		t.Fatalf("expected 2 jobs after delete, got %d", len(sched.ListJobs()))
	}

	// 6. Persistence across restart
	sched2, err := NewScheduler(tmpFile, inv)
	if err != nil {
		t.Fatalf("NewScheduler restart: %v", err)
	}
	if len(sched2.ListJobs()) != 2 {
		t.Fatalf("expected 2 jobs after reload, got %d", len(sched2.ListJobs()))
	}
}

func TestSchedulerRunJobNowAndCircularHistory(t *testing.T) {
	inv := newTestInventory(t)
	tmpFile := filepath.Join(t.TempDir(), "schedules.json")

	cams := map[string]*mockCamera{
		"192.168.1.202:37777": {deviceID: "192.168.1.202:37777", options: map[string]any{"DayNightColor": 1}},
		"192.168.1.203:37777": {deviceID: "192.168.1.203:37777", options: map[string]any{"DayNightColor": 1}},
	}
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		if cam, ok := cams[d.ID]; ok {
			return cam, nil
		}
		return nil, errors.New("not found")
	})
	defer ResetCameraOpener()

	sched, err := NewScheduler(tmpFile, inv)
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	if err := sched.SeedDefault4168Jobs(); err != nil {
		t.Fatalf("SeedDefault4168Jobs: %v", err)
	}
	rec, err := sched.RunJobNow(context.Background(), "job-0700-fullcolor")
	if err != nil {
		t.Fatalf("RunJobNow: %v", err)
	}
	if rec.Status != "success" || rec.JobID != "job-0700-fullcolor" {
		t.Fatalf("unexpected record: %+v", rec)
	}

	// Check history
	hist := sched.History()
	if len(hist) != 1 || hist[0].JobID != "job-0700-fullcolor" {
		t.Fatalf("unexpected history: %+v", hist)
	}

	// Test circular history cap at MaxHistoryEntries (50)
	for range 60 {
		_, _ = sched.RunJobNow(context.Background(), "job-0700-fullcolor")
	}

	histAfter := sched.History()
	if len(histAfter) != MaxHistoryEntries {
		t.Fatalf("history length = %d, want capped at %d", len(histAfter), MaxHistoryEntries)
	}

	// Test ClearHistory
	if err := sched.ClearHistory(); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}
	if len(sched.History()) != 0 {
		t.Fatalf("expected empty history after clear, got %d", len(sched.History()))
	}
}

func TestSchedulerTickerLoop(t *testing.T) {
	inv := newTestInventory(t)
	tmpFile := filepath.Join(t.TempDir(), "schedules.json")

	cams := map[string]*mockCamera{
		"192.168.1.202:37777": {deviceID: "192.168.1.202:37777", options: map[string]any{"DayNightColor": 1}},
		"192.168.1.203:37777": {deviceID: "192.168.1.203:37777", options: map[string]any{"DayNightColor": 1}},
	}
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		if cam, ok := cams[d.ID]; ok {
			return cam, nil
		}
		return nil, errors.New("not found")
	})
	defer ResetCameraOpener()

	sched, err := NewScheduler(tmpFile, inv, WithTickInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	// Add an immediate due job
	dueJob := &Job{
		ID:         "immediate-test-job",
		Name:       "Immediate Test Job",
		Enabled:    true,
		Schedule:   Schedule{Time: time.Now().Format("15:04")},
		ActionType: ActionTypePictureDayNight,
		Action: &PictureDayNightAction{
			DeviceIDs:     []string{"192.168.1.202:37777"},
			Channel:       0,
			DayNightColor: DayNightFullColor,
		},
		NextRunAt: time.Now().Add(-1 * time.Second), // already due
	}
	if err := sched.AddJob(dueJob); err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	// Force NextRunAt into the past
	sched.mu.Lock()
	sched.jobs["immediate-test-job"].NextRunAt = time.Now().Add(-1 * time.Second)
	sched.mu.Unlock()

	sched.Start()
	if !sched.IsRunning() {
		t.Fatal("expected scheduler to be running")
	}

	// Wait up to 500ms for ticker to trigger the due job
	deadline := time.Now().Add(500 * time.Millisecond)
	triggered := false
	for time.Now().Before(deadline) {
		if len(sched.History()) > 0 {
			triggered = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	sched.Stop()
	if sched.IsRunning() {
		t.Fatal("expected scheduler to be stopped")
	}

	if !triggered {
		t.Fatal("expected immediate due job to be triggered by ticker loop")
	}
}

func TestConcurrentSchedulerAccess(t *testing.T) {
	inv := newTestInventory(t)
	tmpFile := filepath.Join(t.TempDir(), "schedules.json")

	sched, err := NewScheduler(tmpFile, inv, WithTickInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	if err := sched.SeedDefault4168Jobs(); err != nil {
		t.Fatalf("SeedDefault4168Jobs: %v", err)
	}

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range 20 {
				_ = sched.ListJobs()
				_ = sched.History()
				_, _ = sched.GetJob("job-0700-fullcolor")
				_ = sched.AddJob(&Job{
					ID:       fmt.Sprintf("worker-%d-%d", workerID, j),
					Name:     "Worker Job",
					Enabled:  false,
					Schedule: Schedule{Time: "12:00"},
					Action:   &RebootAction{DeviceIDs: []string{"Camera02"}},
				})
				_, _ = sched.ToggleJob(fmt.Sprintf("worker-%d-%d", workerID, j))
				_ = sched.DeleteJob(fmt.Sprintf("worker-%d-%d", workerID, j))
			}
		}(i)
	}
	wg.Wait()
}

func TestSchedulePrevRun(t *testing.T) {
	// 1. Daily 17:00 tested at 18:00 on same day -> PrevRun should be today at 17:00
	daily := Schedule{Time: "17:00"}
	now := time.Date(2026, 11, 16, 18, 0, 0, 0, time.UTC)
	prev := daily.PrevRun(now)
	want := time.Date(2026, 11, 16, 17, 0, 0, 0, time.UTC)
	if !prev.Equal(want) {
		t.Fatalf("PrevRun = %v, want %v", prev, want)
	}

	// 2. Daily 17:00 tested at 16:00 on same day -> PrevRun should be yesterday at 17:00
	before := time.Date(2026, 11, 16, 16, 0, 0, 0, time.UTC)
	prevYest := daily.PrevRun(before)
	wantYest := time.Date(2026, 11, 15, 17, 0, 0, 0, time.UTC)
	if !prevYest.Equal(wantYest) {
		t.Fatalf("PrevRun = %v, want %v", prevYest, wantYest)
	}
}

func TestStartupReconciliation(t *testing.T) {
	inv := newTestInventory(t)
	tmpFile := filepath.Join(t.TempDir(), "schedules.json")

	cam2 := &mockCamera{deviceID: "192.168.1.202:37777", options: map[string]any{"DayNightColor": 0}}
	cam3 := &mockCamera{deviceID: "192.168.1.203:37777", options: map[string]any{"DayNightColor": 0}}
	SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		if d.ID == "192.168.1.202:37777" {
			return cam2, nil
		}
		if d.ID == "192.168.1.203:37777" {
			return cam3, nil
		}
		return nil, errors.New("not found")
	})
	defer ResetCameraOpener()

	sched, err := NewScheduler(tmpFile, inv, WithTickInterval(10*time.Millisecond), WithClockCheck(func() bool { return true }))
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}
	if err := sched.SeedDefault4168Jobs(); err != nil {
		t.Fatalf("SeedDefault4168Jobs: %v", err)
	}

	// Simulate system booting at 18:30 (past 17:00 Auto switch), while cameras were left in FullColor (0)
	simNow := time.Date(2026, 11, 16, 18, 30, 0, 0, time.Local)

	// Trigger tick with simNow -> should reconcile the 17:00 Auto job!
	sched.tick(simNow)

	// Wait for execution goroutine
	time.Sleep(100 * time.Millisecond)

	hist := sched.History()
	if len(hist) == 0 {
		t.Fatal("expected at least 1 reconciled job execution in history")
	}
	if hist[0].JobID != "job-1700-auto" {
		t.Fatalf("expected job-1700-auto to be reconciled, got %s", hist[0].JobID)
	}
	if cam2.options["DayNightColor"] != DayNightAuto {
		t.Fatalf("expected cam2 to be reconciled to Auto (1), got %v", cam2.options["DayNightColor"])
	}
}
