package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleRedbidaCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.redbida == nil {
		writeErr(w, http.StatusNotFound, "redbida integration is disabled")
		return
	}
	keys := s.redbida.Catalog()
	sourceAvailable, sourceError := s.redbida.CatalogStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"keys":            keys,
		"sourceAvailable": sourceAvailable,
		"sourceError":     sourceError,
	})
}

func (s *Server) handleRedbidaRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.redbida == nil {
		writeErr(w, http.StatusNotFound, "redbida integration is disabled")
		return
	}
	var req struct {
		Keys []string `json:"keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Keys) == 0 {
		for _, meta := range s.redbida.Catalog() {
			req.Keys = append(req.Keys, meta.Key)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.redbidaTimeout())
	defer cancel()
	values, err := s.redbida.Refresh(ctx, req.Keys)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values, "refreshedAt": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleRedbidaApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.redbida == nil {
		writeErr(w, http.StatusNotFound, "redbida integration is disabled")
		return
	}
	var req struct {
		Changes   map[string]any `json:"changes"`
		Confirmed bool           `json:"confirmed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.redbidaTimeout())
	defer cancel()
	results, err := s.redbida.Apply(ctx, req.Changes, req.Confirmed)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "appliedAt": time.Now().Format(time.RFC3339)})
}

func (s *Server) handleRedbidaTimeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	now := time.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"hostTime":              now.Format("2006-01-02 15:04:05"),
		"hostTimeRFC3339":       now.Format(time.RFC3339),
		"ntpSynchronized":       hostTimeTrusted(ctx),
		"driftThresholdSeconds": 60,
		"policy":                "sync only when host NTP is trusted and measured drift exceeds 60 seconds",
		"nodeRedReadOnly":       true,
	})
}

func (s *Server) redbidaTimeout() time.Duration {
	sec := s.cfg.Redbida.TimeoutSeconds
	if sec < 2 {
		sec = 10
	}
	sec *= 3 // Apply may perform write, acknowledgement handling, and read-back.
	if sec > 120 {
		sec = 120
	}
	return time.Duration(sec) * time.Second
}
