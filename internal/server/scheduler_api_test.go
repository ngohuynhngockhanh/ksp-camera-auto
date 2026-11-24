package server

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/scheduler"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestServerWithScheduler(t *testing.T) (*Server, string) {
	t.Helper()
	t.Setenv("KSPCAM_KEY", "test-scheduler-crypto-key")
	tmp := t.TempDir()
	invPath := filepath.Join(tmp, "cameras.yaml")
	schedulesPath := filepath.Join(tmp, "schedules.json")

	_ = os.WriteFile(invPath, []byte("[]"), 0644)
	inv, err := config.LoadInventory(invPath)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}

	_ = inv.Upsert(config.Device{
		ID:       "cam-1",
		Name:     "Cam 1",
		Host:     "192.168.1.101",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "pw",
	})
	_ = inv.Upsert(config.Device{
		ID:       "cam-2",
		Name:     "Cam 2",
		Host:     "192.168.1.102",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "pw",
	})
	_ = inv.Upsert(config.Device{
		ID:       "cam-3",
		Name:     "Cam 3",
		Host:     "192.168.1.103",
		Port:     37777,
		Vendor:   config.VendorDahua,
		Username: "admin",
		Password: "pw",
	})

	cfg := config.Default()
	cfg.Events.Enabled = false
	cfg.Scheduler.Enabled = true
	cfg.Scheduler.SchedulesFile = schedulesPath
	srv, err := New(cfg, inv)
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	if srv.scheduler != nil {
		_ = srv.scheduler.SeedDefault4168Jobs()
	}

	return srv, schedulesPath
}

type mockSchedulerCam struct{}

func (m *mockSchedulerCam) Probe(ctx context.Context) ([]camera.StreamInfo, error) { return nil, nil }
func (m *mockSchedulerCam) Apply(ctx context.Context, profile camera.Profile, emit func(camera.StepResult)) []camera.StepResult {
	return nil
}
func (m *mockSchedulerCam) ChangePassword(ctx context.Context, newUser, newPass string) error {
	return nil
}
func (m *mockSchedulerCam) Snapshot(ctx context.Context, channel, stream int) ([]byte, error) {
	return nil, nil
}
func (m *mockSchedulerCam) ChannelInfo(ctx context.Context, channel int) (string, []string, []bool, bool, error) {
	return "", nil, nil, true, nil
}
func (m *mockSchedulerCam) SetChannelName(ctx context.Context, channel int, name string) error {
	return nil
}
func (m *mockSchedulerCam) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	return 0, nil
}
func (m *mockSchedulerCam) Close() error { return nil }
func (m *mockSchedulerCam) GetPicture(ctx context.Context, channel int) (map[string]any, map[string]any, error) {
	return nil, map[string]any{"DayNightColor": 1}, nil
}
func (m *mockSchedulerCam) SetPicture(ctx context.Context, channel int, c, o map[string]any) (map[string]any, map[string]any, error) {
	return nil, map[string]any{"DayNightColor": o["DayNightColor"]}, nil
}
func (m *mockSchedulerCam) GetPictureCaps(ctx context.Context, channel int) (map[string]any, error) {
	return nil, nil
}

func TestSchedulerAPI_Endpoints(t *testing.T) {
	scheduler.SetCameraOpener(func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
		return &mockSchedulerCam{}, nil
	})
	defer scheduler.ResetCameraOpener()

	srv, _ := setupTestServerWithScheduler(t)
	defer srv.Close()

	adminToken := srv.session.create("admin")
	authCookie := &http.Cookie{Name: sessionCookie, Value: adminToken}

	// 1. GET /api/scheduler/jobs - should return seeded default jobs (07:00 and 17:00)
	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/jobs", nil)
	req.AddCookie(authCookie)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/scheduler/jobs: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var listResp struct {
		OK      bool             `json:"ok"`
		Jobs    []*scheduler.Job `json:"jobs"`
		Running bool             `json:"running"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if !listResp.OK || len(listResp.Jobs) < 2 {
		t.Fatalf("expected at least 2 seeded default jobs, got %d", len(listResp.Jobs))
	}

	// 2. POST /api/scheduler/jobs - create a new job
	newJob := map[string]any{
		"name":       "Test Night Color",
		"enabled":    true,
		"actionType": "picture_day_night",
		"schedule": map[string]any{
			"time":       "22:30",
			"daysOfWeek": []int{1, 2, 3, 4, 5},
		},
		"action": map[string]any{
			"deviceIds":     []string{"cam-1"},
			"dayNightColor": 2, // B&W
			"channel":       0,
		},
	}
	jobBody, _ := json.Marshal(newJob)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs", bytes.NewReader(jobBody))
	reqCreate.AddCookie(authCookie)
	recCreate := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusOK {
		t.Fatalf("POST /api/scheduler/jobs: expected 200, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createResp struct {
		OK  bool          `json:"ok"`
		Job scheduler.Job `json:"job"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.Job.ID == "" || createResp.Job.Name != "Test Night Color" {
		t.Fatalf("unexpected created job: %+v", createResp.Job)
	}
	createdID := createResp.Job.ID

	// 3. POST /api/scheduler/jobs/update - update the job
	updatePayload := map[string]any{
		"id":         createdID,
		"name":       "Test Night Color Updated",
		"enabled":    true,
		"actionType": "picture_day_night",
		"schedule": map[string]any{
			"time":       "23:00",
			"daysOfWeek": []int{0, 1, 2, 3, 4, 5, 6},
		},
		"action": map[string]any{
			"deviceIds":     []string{"cam-1", "cam-2"},
			"dayNightColor": 1, // Auto
			"channel":       0,
		},
	}
	updateBody, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/update", bytes.NewReader(updateBody))
	reqUpdate.AddCookie(authCookie)
	recUpdate := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("POST /api/scheduler/jobs/update: expected 200, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	// 4. POST /api/scheduler/jobs/toggle - toggle job enabled
	togglePayload := map[string]any{
		"id": createdID,
	}
	toggleBody, _ := json.Marshal(togglePayload)
	reqToggle := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/toggle", bytes.NewReader(toggleBody))
	reqToggle.AddCookie(authCookie)
	recToggle := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recToggle, reqToggle)

	if recToggle.Code != http.StatusOK {
		t.Fatalf("POST /api/scheduler/jobs/toggle: expected 200, got %d: %s", recToggle.Code, recToggle.Body.String())
	}
	var toggleResp struct {
		OK  bool          `json:"ok"`
		Job scheduler.Job `json:"job"`
	}
	if err := json.Unmarshal(recToggle.Body.Bytes(), &toggleResp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}
	if toggleResp.Job.Enabled != false {
		t.Fatalf("expected job to be toggled to false, got %v", toggleResp.Job.Enabled)
	}

	// 5. POST /api/scheduler/jobs/run - trigger test run
	runPayload := map[string]any{
		"id": createdID,
	}
	runBody, _ := json.Marshal(runPayload)
	reqRun := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/run", bytes.NewReader(runBody))
	reqRun.AddCookie(authCookie)
	recRun := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recRun, reqRun)

	// Since mock/real camera dial may fail, run should return 200 with record
	if recRun.Code != http.StatusOK {
		t.Fatalf("POST /api/scheduler/jobs/run: expected 200, got %d: %s", recRun.Code, recRun.Body.String())
	}
	var runResp struct {
		OK     bool                       `json:"ok"`
		Record *scheduler.ExecutionRecord `json:"record"`
		Error  string                     `json:"error"`
	}
	if err := json.Unmarshal(recRun.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if runResp.Record == nil || runResp.Record.JobID != createdID {
		t.Fatalf("expected run record for %s, got %+v", createdID, runResp.Record)
	}

	// 6. GET /api/scheduler/history - verify history contains the run
	reqHist := httptest.NewRequest(http.MethodGet, "/api/scheduler/history", nil)
	reqHist.AddCookie(authCookie)
	recHist := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recHist, reqHist)

	if recHist.Code != http.StatusOK {
		t.Fatalf("GET /api/scheduler/history: expected 200, got %d: %s", recHist.Code, recHist.Body.String())
	}
	var histResp struct {
		OK      bool                        `json:"ok"`
		History []scheduler.ExecutionRecord `json:"history"`
	}
	if err := json.Unmarshal(recHist.Body.Bytes(), &histResp); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	if len(histResp.History) == 0 {
		t.Fatalf("expected non-empty execution history")
	}

	// 7. POST /api/scheduler/jobs/delete - delete the job
	delPayload := map[string]any{
		"id": createdID,
	}
	delBody, _ := json.Marshal(delPayload)
	reqDel := httptest.NewRequest(http.MethodPost, "/api/scheduler/jobs/delete", bytes.NewReader(delBody))
	reqDel.AddCookie(authCookie)
	recDel := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("POST /api/scheduler/jobs/delete: expected 200, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// Verify job is gone
	j, found := srv.scheduler.GetJob(createdID)
	if found || j != nil {
		t.Fatalf("expected job %s to be deleted, but still found", createdID)
	}
}

func TestSchedulerAPI_Unauthenticated(t *testing.T) {
	srv, _ := setupTestServerWithScheduler(t)
	defer srv.Close()

	// API fetch with Accept: application/json should return 401
	req := httptest.NewRequest(http.MethodGet, "/api/scheduler/jobs", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rec.Code)
	}

	// Browser navigation without Accept header should redirect to /login
	reqNav := httptest.NewRequest(http.MethodGet, "/api/scheduler/jobs", nil)
	recNav := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recNav, reqNav)
	if recNav.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect to /login, got %d", recNav.Code)
	}
}
