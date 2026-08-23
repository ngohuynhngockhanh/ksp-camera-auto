package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

// handleShinobiStatus returns connection status and monitor count.
// GET /api/shinobi/status
func (s *Server) handleShinobiStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.shinobi == nil {
		writeJSON(w, http.StatusOK, shinobi.ShinobiStatus{
			Configured: false,
			Connected:  false,
		})
		return
	}
	st, err := s.shinobi.Status(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, shinobi.ShinobiStatus{
			Configured: true,
			Connected:  false,
			APIURL:     s.shinobi.APIURL(),
			GroupKey:   s.shinobi.GroupKey(),
			Error:      err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// shinobiMonitorView is a clean view for web UI display
type shinobiMonitorView struct {
	Mid      string                 `json:"mid"`
	Ke       string                 `json:"ke"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Mode     string                 `json:"mode"`
	Host     string                 `json:"host"`
	Port     string                 `json:"port"`
	Protocol string                 `json:"protocol"`
	Path     string                 `json:"path"`
	Ext      string                 `json:"ext"`
	Details  shinobi.MonitorDetails `json:"details"`
}

type shinobiMonitorActionReq struct {
	Action    string                `json:"action"` // "add", "edit", "delete", "state"
	MonitorID string                `json:"monitorId"`
	State     string                `json:"state"` // "start", "stop", "record", "idle", "restart"
	Monitor   shinobi.MonitorConfig `json:"monitor"`
}

// handleShinobiMonitors handles listing, creating, editing, deleting, or state toggling monitors.
// GET /api/shinobi/monitors -> list monitors
// POST /api/shinobi/monitors -> action dispatcher (add, edit, delete, state)
func (s *Server) handleShinobiMonitors(w http.ResponseWriter, r *http.Request) {
	if s.shinobi == nil {
		writeErr(w, http.StatusBadRequest, "Shinobi is not configured in config.yaml")
		return
	}

	switch r.Method {
	case http.MethodGet:
		mons, err := s.shinobi.ListMonitors(r.Context())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		views := make([]shinobiMonitorView, 0, len(mons))
		for _, m := range mons {
			views = append(views, shinobiMonitorView{
				Mid:      m.Mid,
				Ke:       m.Ke,
				Name:     m.Name,
				Type:     m.Type,
				Mode:     m.Mode,
				Host:     m.Host,
				Port:     string(m.Port),
				Protocol: m.Protocol,
				Path:     m.Path,
				Ext:      m.Ext,
				Details:  m.ParseDetails(),
			})
		}
		writeJSON(w, http.StatusOK, views)

	case http.MethodPost:
		var req shinobiMonitorActionReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "bad request json: "+err.Error())
			return
		}
		req.Action = strings.ToLower(strings.TrimSpace(req.Action))
		switch req.Action {
		case "add":
			if err := s.shinobi.AddMonitor(r.Context(), req.Monitor); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Monitor added"})

		case "edit":
			mid := req.MonitorID
			if mid == "" {
				mid = req.Monitor.Mid
			}
			if err := s.shinobi.EditMonitor(r.Context(), mid, req.Monitor); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Monitor updated"})

		case "delete":
			if req.MonitorID == "" {
				writeErr(w, http.StatusBadRequest, "monitorId is required for delete")
				return
			}
			if err := s.shinobi.DeleteMonitor(r.Context(), req.MonitorID); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Monitor deleted"})

		case "state":
			if req.MonitorID == "" {
				writeErr(w, http.StatusBadRequest, "monitorId is required for state change")
				return
			}
			if err := s.shinobi.ChangeMonitorState(r.Context(), req.MonitorID, req.State); err != nil {
				writeErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Monitor state changed to " + req.State})

		default:
			writeErr(w, http.StatusBadRequest, "unknown action (must be add, edit, delete, or state)")
		}

	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleShinobiSyncToShinobi pushes cameras from cameras.yaml to Shinobi monitors.
// POST /api/shinobi/sync-to-shinobi
func (s *Server) handleShinobiSyncToShinobi(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.shinobi == nil {
		writeErr(w, http.StatusBadRequest, "Shinobi is not configured in config.yaml")
		return
	}
	report, err := s.shinobi.SyncToShinobi(r.Context(), s.inv)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleShinobiSyncFromShinobi pulls monitors from Shinobi to cameras.yaml.
// POST /api/shinobi/sync-from-shinobi
func (s *Server) handleShinobiSyncFromShinobi(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.shinobi == nil {
		writeErr(w, http.StatusBadRequest, "Shinobi is not configured in config.yaml")
		return
	}
	report, err := s.shinobi.SyncFromShinobi(r.Context(), s.inv)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// handleShinobiVideos returns recorded videos for a given monitor.
// GET /api/shinobi/videos?mid=<mid>&limit=<limit>
func (s *Server) handleShinobiVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.shinobi == nil {
		writeErr(w, http.StatusBadRequest, "Shinobi is not configured in config.yaml")
		return
	}
	mid := strings.TrimSpace(r.URL.Query().Get("mid"))
	if mid == "" {
		writeErr(w, http.StatusBadRequest, "mid query parameter is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	videos, err := s.shinobi.GetVideos(r.Context(), mid, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if videos == nil {
		videos = []shinobi.Video{}
	}
	writeJSON(w, http.StatusOK, videos)
}
