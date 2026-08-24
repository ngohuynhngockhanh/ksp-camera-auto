package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/traffic"
)

// handleTrafficInterfaces lists all network interfaces excluding wlan* and lo.
func (s *Server) handleTrafficInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ifaces, err := traffic.ListEligibleInterfaces()
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	def := traffic.DetectDefaultInterface()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"interfaces": ifaces,
		"default":    def,
	})
}

// handleTrafficSnapshot returns a point-in-time traffic snapshot.
func (s *Server) handleTrafficSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.traffic == nil {
		http.Error(w, `{"error":"Traffic manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	iface := r.URL.Query().Get("iface")
	snap := s.traffic.GetSnapshot(iface)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

// handleTrafficStream streams live iftop traffic snapshots via Server-Sent Events (SSE).
func (s *Server) handleTrafficStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.traffic == nil {
		http.Error(w, `{"error":"Traffic manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	iface := strings.TrimSpace(r.URL.Query().Get("iface"))
	if iface == "" {
		iface = traffic.DetectDefaultInterface()
	}

	// Security/filter: prevent selecting wlan interfaces
	if strings.HasPrefix(strings.ToLower(iface), "wlan") || strings.HasPrefix(strings.ToLower(iface), "wl") {
		http.Error(w, "Wireless interfaces are disabled for traffic inspection", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Subscribe to live traffic stream
	ch, unsub := s.traffic.Subscribe(r.Context(), iface)
	defer unsub()

	// Send initial snapshot immediately
	initSnap := s.traffic.GetSnapshot(iface)
	if b, err := json.Marshal(initSnap); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case snap, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(snap)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
