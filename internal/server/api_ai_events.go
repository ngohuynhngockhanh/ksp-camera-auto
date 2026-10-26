package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/raysharp"
)

func (s *Server) openAICamera(ctx context.Context, id string, timeoutSec int) (camera.Camera, camera.AIConfig, error) {
	d, ok := s.inv.Get(strings.TrimSpace(id))
	if !ok {
		return nil, nil, fmt.Errorf("không tìm thấy camera %q", id)
	}
	cam, err := camera.Open(ctx, d, s.reqTimeout(timeoutSec))
	if err != nil {
		return nil, nil, err
	}
	ai, ok := cam.(camera.AIConfig)
	if !ok {
		_ = cam.Close()
		return nil, nil, fmt.Errorf("camera %q (vendor %s) không hỗ trợ cấu hình AI", id, d.Vendor)
	}
	return cam, ai, nil
}

// handleAICapabilities handles GET/POST /api/ai/capabilities — reads all AI
// abilities and their On/Off status across channels (/API/Event/ChnSmart/Get).
func (s *Server) handleAICapabilities(w http.ResponseWriter, r *http.Request) {
	var id string
	var timeoutSec int
	if r.Method == http.MethodGet {
		id = r.URL.Query().Get("id")
	} else if r.Method == http.MethodPost {
		var req struct {
			ID             string `json:"id"`
			TimeoutSeconds int    `json:"timeoutSeconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		id = req.ID
		timeoutSec = req.TimeoutSeconds
	} else {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if strings.TrimSpace(id) == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(timeoutSec))
	defer cancel()

	cam, ai, err := s.openAICamera(ctx, id, timeoutSec)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer cam.Close()

	caps, err := ai.GetChnSmart(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       id,
		"channels": caps,
	})
}

// handleAISetting handles GET (read AI rule setting + range) and POST (update
// AI rule setting via GET-Modify-SET) on /api/ai/setting.
func (s *Server) handleAISetting(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		ability := strings.TrimSpace(r.URL.Query().Get("ability"))
		ch, _ := strconv.Atoi(r.URL.Query().Get("channel"))
		if id == "" || ability == "" {
			writeErr(w, http.StatusBadRequest, "id and ability are required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(0))
		defer cancel()
		cam, ai, err := s.openAICamera(ctx, id, 0)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cam.Close()
		data, rng, err := ai.GetAISetting(ctx, ch, ability)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      id,
			"channel": ch,
			"ability": ability,
			"data":    data,
			"range":   rng,
		})

	case http.MethodPost:
		var req struct {
			ID             string         `json:"id"`
			Channel        int            `json:"channel"`
			Ability        string         `json:"ability"`
			Changes        map[string]any `json:"changes"`
			TimeoutSeconds int            `json:"timeoutSeconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.Ability == "" {
			writeErr(w, http.StatusBadRequest, "id and ability are required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(req.TimeoutSeconds))
		defer cancel()
		cam, ai, err := s.openAICamera(ctx, req.ID, req.TimeoutSeconds)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cam.Close()
		updated, err := ai.SetAISetting(ctx, req.Channel, req.Ability, req.Changes)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"id":      req.ID,
			"channel": req.Channel,
			"ability": req.Ability,
			"data":    updated,
		})

	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAILinkage handles GET and POST on /api/ai/linkage (/API/Event/AlarmLinkage).
func (s *Server) handleAILinkage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		ability := strings.TrimSpace(r.URL.Query().Get("ability"))
		ch, _ := strconv.Atoi(r.URL.Query().Get("channel"))
		if id == "" || ability == "" {
			writeErr(w, http.StatusBadRequest, "id and ability are required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(0))
		defer cancel()
		cam, ai, err := s.openAICamera(ctx, id, 0)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cam.Close()
		data, err := ai.GetAIAlarmLinkage(ctx, ch, ability)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":      id,
			"channel": ch,
			"ability": ability,
			"data":    data,
		})

	case http.MethodPost:
		var req struct {
			ID             string         `json:"id"`
			Channel        int            `json:"channel"`
			Ability        string         `json:"ability"`
			Changes        map[string]any `json:"changes"`
			TimeoutSeconds int            `json:"timeoutSeconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if req.ID == "" || req.Ability == "" {
			writeErr(w, http.StatusBadRequest, "id and ability are required")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(req.TimeoutSeconds))
		defer cancel()
		cam, ai, err := s.openAICamera(ctx, req.ID, req.TimeoutSeconds)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cam.Close()
		updated, err := ai.SetAIAlarmLinkage(ctx, req.Channel, req.Ability, req.Changes)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"id":      req.ID,
			"channel": req.Channel,
			"ability": req.Ability,
			"data":    updated,
		})

	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAIEventPush handles POST /api/ai/event-push to configure an NVR/IPC to
// push webhooks to kspcam.
func (s *Server) handleAIEventPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		Enable         bool   `json:"enable"`
		TargetHost     string `json:"targetHost"`
		TargetPort     int    `json:"targetPort"`
		TargetPath     string `json:"targetPath"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" || req.TargetHost == "" {
		writeErr(w, http.StatusBadRequest, "id and targetHost are required")
		return
	}
	if req.TargetPort <= 0 {
		req.TargetPort = 2028
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(req.TimeoutSeconds))
	defer cancel()
	cam, ai, err := s.openAICamera(ctx, req.ID, req.TimeoutSeconds)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	defer cam.Close()
	if err := ai.ConfigureEventPush(ctx, req.Enable, req.TargetHost, req.TargetPort, req.TargetPath); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleRaysharpRaw handles POST /api/raysharp/raw — direct pass-through to any
// Raysharp /API/<Path> endpoint (including /API/AI/FDGroup/*, /API/AI/SnapedFaces/*,
// /API/AI/SnapedObjects/*, /API/AI/PlateGroup/*, etc.).
func (s *Server) handleRaysharpRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		Path           string `json:"path"`
		Data           any    `json:"data"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.Path) == "" {
		writeErr(w, http.StatusBadRequest, "id and path are required")
		return
	}
	d, ok := s.inv.Get(strings.TrimSpace(req.ID))
	if !ok {
		writeErr(w, http.StatusNotFound, "camera not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.reqTimeout(req.TimeoutSeconds))
	defer cancel()

	cam, err := camera.Open(ctx, d, s.reqTimeout(req.TimeoutSeconds))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer cam.Close()

	rawCaller, ok := cam.(camera.RawRPC)
	if !ok {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("camera %q (vendor %s) does not support raw RPC", req.ID, d.Vendor))
		return
	}
	res, err := rawCaller.CallRaw(ctx, req.Path, req.Data)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":       false,
			"error":    err.Error(),
			"response": res,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"response": res,
	})
}

// handleEventsStatus handles GET /api/events/status.
func (s *Server) handleEventsStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.events.GetStatus())
}

// handleEventsConfig handles GET and POST /api/events/config.
func (s *Server) handleEventsConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.events.GetConfig())
	case http.MethodPost:
		var cfg config.EventsConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		s.events.UpdateConfig(cfg)
		s.cfg.Events = s.events.GetConfig()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"status": s.events.GetStatus(),
		})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleEventsList handles GET /api/events/list?device_id=&event_type=&limit=.
func (s *Server) handleEventsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	devID := strings.TrimSpace(r.URL.Query().Get("device_id"))
	evType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events": s.events.ListEvents(devID, evType, limit),
		"status": s.events.GetStatus(),
	})
}

// handleEventsStream streams real-time AI/Alarm events over SSE (GET /api/events/stream).
func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, unsub := s.events.Subscribe()
	defer unsub()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

// handleEventsPollNow triggers an immediate Event/Check poll across all Raysharp devices.
func (s *Server) handleEventsPollNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	cursors := make(map[string]*raysharp.EventCheckCursor)
	count := s.events.PollOnce(ctx, cursors)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"emitted": count,
		"status":  s.events.GetStatus(),
	})
}

// handleRaysharpWebhook receives unauthenticated HTTP webhook pushes from
// Raysharp NVRs (/API/AlarmConfig/EventPush) and IPCs (/API/Event/Notification)
// on the local LAN.
func (s *Server) handleRaysharpWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		// Handle GET query-string notification webhooks
		qMap := make(map[string]any)
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				qMap[k] = v[0]
			}
		}
		body, _ = json.Marshal(qMap)
	}
	emitted, err := s.events.IngestRaysharpWebhook(remoteHost, body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"ingested": len(emitted),
	})
}
