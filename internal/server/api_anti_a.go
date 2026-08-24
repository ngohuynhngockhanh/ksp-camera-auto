package server

import (
	"encoding/json"
	"net/http"
)

// handleAntiA handles GET (status) and POST (config update) for Anti-A Guardian.
func (s *Server) handleAntiA(w http.ResponseWriter, r *http.Request) {
	if s.antiA == nil {
		http.Error(w, `{"error":"Anti-A Guardian not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		status := s.antiA.getStatus()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)

	case http.MethodPost:
		if s.sessionRole(r) != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var req antiAConfigReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
			return
		}
		if err := s.antiA.updateConfig(r.Context(), req); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		status := s.antiA.getStatus()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     true,
			"status": status,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAntiATrigger handles POST /api/anti-a/trigger to immediately scan and enforce.
func (s *Server) handleAntiATrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.sessionRole(r) != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.antiA == nil {
		http.Error(w, `{"error":"Anti-A Guardian not initialized"}`, http.StatusServiceUnavailable)
		return
	}

	enforced, err := s.antiA.triggerNow(r.Context())
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	status := s.antiA.getStatus()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"enforced": enforced,
		"status":   status,
	})
}
