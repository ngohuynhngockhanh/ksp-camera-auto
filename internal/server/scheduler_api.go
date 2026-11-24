package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/scheduler"
)

// handleSchedulerJobs handles GET (list all scheduled jobs) and POST (create a new job).
func (s *Server) handleSchedulerJobs(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"jobs":    []*scheduler.Job{},
				"running": false,
			})
			return
		}
		writeErr(w, http.StatusServiceUnavailable, "scheduler is not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		jobs := s.scheduler.ListJobs()
		if jobs == nil {
			jobs = []*scheduler.Job{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"jobs":    jobs,
			"running": s.scheduler.IsRunning(),
		})

	case http.MethodPost:
		var job scheduler.Job
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid job json: "+err.Error())
			return
		}
		if err := s.scheduler.AddJob(&job); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":  true,
			"job": job,
		})

	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSchedulerJobUpdate handles POST /api/scheduler/jobs/update to modify an existing job.
func (s *Server) handleSchedulerJobUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, "scheduler is not initialized")
		return
	}

	var job scheduler.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid job json: "+err.Error())
		return
	}
	if strings.TrimSpace(job.ID) == "" {
		writeErr(w, http.StatusBadRequest, "job id is required")
		return
	}

	if err := s.scheduler.UpdateJob(&job); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"job": job,
	})
}

// handleSchedulerJobDelete handles POST /api/scheduler/jobs/delete to remove a job by ID.
func (s *Server) handleSchedulerJobDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, "scheduler is not initialized")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.ID) == "" {
		req.ID = r.URL.Query().Get("id")
	}
	if strings.TrimSpace(req.ID) == "" {
		writeErr(w, http.StatusBadRequest, "job id is required")
		return
	}

	if err := s.scheduler.DeleteJob(req.ID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "job deleted",
		"id":      req.ID,
	})
}

// handleSchedulerJobToggle handles POST /api/scheduler/jobs/toggle to toggle or set job enabled state.
func (s *Server) handleSchedulerJobToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, "scheduler is not initialized")
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled *bool  `json:"enabled,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.ID) == "" {
		req.ID = r.URL.Query().Get("id")
	}
	if strings.TrimSpace(req.ID) == "" {
		writeErr(w, http.StatusBadRequest, "job id is required")
		return
	}

	var job *scheduler.Job
	var err error
	if req.Enabled != nil {
		job, err = s.scheduler.ToggleJob(req.ID, *req.Enabled)
	} else {
		job, err = s.scheduler.ToggleJob(req.ID)
	}

	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"job": job,
	})
}

// handleSchedulerJobRun handles POST /api/scheduler/jobs/run to execute a job immediately.
func (s *Server) handleSchedulerJobRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.scheduler == nil {
		writeErr(w, http.StatusServiceUnavailable, "scheduler is not initialized")
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if strings.TrimSpace(req.ID) == "" {
		req.ID = r.URL.Query().Get("id")
	}
	if strings.TrimSpace(req.ID) == "" {
		writeErr(w, http.StatusBadRequest, "job id is required")
		return
	}

	rec, err := s.scheduler.RunJobNow(r.Context(), req.ID)
	if rec == nil && err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     err == nil,
		"record": rec,
		"error":  errMsg,
	})
}

// handleSchedulerHistory handles GET /api/scheduler/history to retrieve execution logs.
func (s *Server) handleSchedulerHistory(w http.ResponseWriter, r *http.Request) {
	if s.scheduler == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"history": []scheduler.ExecutionRecord{},
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		history := s.scheduler.History()
		if history == nil {
			history = []scheduler.ExecutionRecord{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"history": history,
		})

	case http.MethodDelete:
		_ = s.scheduler.ClearHistory()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"history": []scheduler.ExecutionRecord{},
		})

	case http.MethodPost:
		if r.URL.Query().Get("clear") == "1" {
			_ = s.scheduler.ClearHistory()
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"history": []scheduler.ExecutionRecord{},
			})
			return
		}
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")

	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
