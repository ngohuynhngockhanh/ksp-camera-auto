package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/importer"
)

// reqTimeout resolves a per-request device timeout: the request's
// timeoutSeconds (clamped to 5..600s) if given, else the configured default.
// The user can wait for slow NVRs by raising it from the web UI.
func (s *Server) reqTimeout(sec int) time.Duration {
	if sec <= 0 {
		sec = s.cfg.Defaults.TimeoutSeconds
	}
	if sec < 5 {
		sec = 5
	}
	if sec > 600 {
		sec = 600
	}
	return time.Duration(sec) * time.Second
}

// deviceView is the JSON-safe projection of config.Device: passwords never
// leave the server.
type deviceView struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Vendor   config.Vendor `json:"vendor"`
	Username string        `json:"username"`
}

func toView(d config.Device) deviceView {
	return deviceView{
		ID:       d.ID,
		Name:     d.Name,
		Host:     d.Host,
		Port:     d.Port,
		Vendor:   d.Vendor,
		Username: d.Username,
	}
}

// writeJSON encodes v as the response body with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent; nothing more to do but log.
		log.Printf("encode response: %v", err)
	}
}

// writeErr writes a {"error": msg} JSON body.
func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// handleCameras handles GET (list inventory) and POST (add/update a camera).
func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleCamerasList(w, r)
	case http.MethodPost:
		s.handleCamerasUpsert(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleCamerasList(w http.ResponseWriter, r *http.Request) {
	devices := s.inv.List()
	out := make([]deviceView, 0, len(devices))
	for _, d := range devices {
		out = append(out, toView(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// cameraUpsertReq is the body of POST /api/cameras.
type cameraUpsertReq struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Vendor   config.Vendor `json:"vendor"`
	Username string        `json:"username"`
	Password string        `json:"password"`
}

func (s *Server) handleCamerasUpsert(w http.ResponseWriter, r *http.Request) {
	var req cameraUpsertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Host == "" {
		writeErr(w, http.StatusBadRequest, "host is required")
		return
	}
	if req.Vendor != config.VendorDahua && req.Vendor != config.VendorHikvision {
		writeErr(w, http.StatusBadRequest, "vendor must be dahua or hikvision")
		return
	}
	if req.Port == 0 {
		if req.Vendor == config.VendorDahua {
			req.Port = s.cfg.Defaults.DahuaPort
		} else {
			req.Port = s.cfg.Defaults.HikvisionPort
		}
	}
	if req.Username == "" {
		req.Username = s.cfg.Defaults.Username
	}
	if req.Password == "" {
		// Editing an existing camera with a blank password keeps the stored one
		// (so users can fix the name/username without re-typing the password);
		// a brand-new camera falls back to the configured default.
		id := req.ID
		if id == "" {
			id = fmt.Sprintf("%s:%d", req.Host, req.Port)
		}
		if existing, ok := s.inv.Get(id); ok && existing.Password != "" {
			req.Password = existing.Password
		} else {
			req.Password = s.cfg.Defaults.Password
		}
	}

	d := config.Device{
		ID:       req.ID,
		Name:     req.Name,
		Host:     req.Host,
		Port:     req.Port,
		Vendor:   req.Vendor,
		Username: req.Username,
		Password: req.Password,
	}
	if err := s.inv.Upsert(d); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Upsert derives the ID (host:port) for new entries; mirror that so we
	// echo back the saved device rather than an empty lookup.
	if d.ID == "" {
		d.ID = d.Addr()
	}
	saved, _ := s.inv.Get(d.ID)
	writeJSON(w, http.StatusOK, toView(saved))
}

// importReq is the body of POST /api/import (Shinobi monitor JSON).
type importReq struct {
	JSON      string `json:"json"`
	HikPort   int    `json:"hikPort"`
	DahuaPort int    `json:"dahuaPort"`
}

// handleImport parses a Shinobi monitor config, auto-detecting vendor +
// credentials from each RTSP URL, and adds the cameras to the inventory.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req importReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.HikPort == 0 {
		req.HikPort = s.cfg.Defaults.HikvisionPort
	}
	if req.DahuaPort == 0 {
		req.DahuaPort = s.cfg.Defaults.DahuaPort
	}
	res, err := importer.ParseShinobi([]byte(req.JSON), req.HikPort, req.DahuaPort)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	added := make([]deviceView, 0, len(res.Devices))
	for _, d := range res.Devices {
		if err := s.inv.Upsert(d); err != nil {
			continue
		}
		if d.ID == "" {
			d.ID = d.Addr()
		}
		if saved, ok := s.inv.Get(d.ID); ok {
			added = append(added, toView(saved))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "skipped": res.Skipped})
}

// passwordReq is the body of POST /api/password.
type passwordReq struct {
	DeviceIDs      []string `json:"deviceIds"`
	NewUsername    string   `json:"newUsername"`
	NewPassword    string   `json:"newPassword"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

// handlePassword changes the password on a set of devices, one at a time,
// streaming progress like /api/apply. On success it updates the stored
// credential in lock-step so the tool keeps working (no lockout).
func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req passwordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.DeviceIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "deviceIds is required")
		return
	}
	if req.NewUsername == "" {
		req.NewUsername = s.cfg.Defaults.Username
	}
	if req.NewPassword == "" {
		req.NewPassword = s.cfg.Defaults.NewPassword
	}
	to := s.reqTimeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(r.Context(), to*time.Duration(len(req.DeviceIDs)+1))
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	emit := func(ev bulk.Event) {
		if r.Context().Err() != nil {
			return
		}
		b, err := json.Marshal(ev)
		if err != nil {
			return
		}
		if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	total := len(req.DeviceIDs)
	for i, id := range req.DeviceIDs {
		if ctx.Err() != nil {
			break
		}
		d, ok := s.inv.Get(id)
		emit(bulk.Event{Type: "device_start", DeviceID: id, Name: d.Name, Host: d.Host, Index: i + 1, Total: total})
		if !ok {
			emit(bulk.Event{Type: "device_done", DeviceID: id, OK: false, Err: "không có trong kho"})
			continue
		}
		cam, err := camera.Open(ctx, d, to)
		if err != nil {
			emit(bulk.Event{Type: "device_done", DeviceID: id, Name: d.Name, OK: false, Err: err.Error()})
			continue
		}
		err = cam.ChangePassword(ctx, req.NewUsername, req.NewPassword)
		cam.Close()
		if err != nil {
			emit(bulk.Event{Type: "step", DeviceID: id, Name: d.Name, Step: "đổi mật khẩu", OK: false, Err: err.Error()})
			emit(bulk.Event{Type: "device_done", DeviceID: id, Name: d.Name, OK: false, Err: err.Error()})
			continue
		}
		// Update the stored credential so we can still connect.
		d.Username, d.Password = req.NewUsername, req.NewPassword
		_ = s.inv.Upsert(d)
		emit(bulk.Event{Type: "step", DeviceID: id, Name: d.Name, Step: "đổi mật khẩu", Detail: "OK — đã cập nhật kho", OK: true})
		emit(bulk.Event{Type: "device_done", DeviceID: id, Name: d.Name, OK: true})
	}
	emit(bulk.Event{Type: "done"})
}

// idReq is a body carrying only a device ID, used by delete/probe.
type idReq struct {
	ID             string `json:"id"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

// handleCamerasDelete handles POST /api/cameras/delete.
func (s *Server) handleCamerasDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := s.inv.Delete(req.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleProbe handles POST /api/probe: connect to a device and read back its
// current stream settings.
func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req idReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	d, ok := s.inv.Get(req.ID)
	if !ok {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}

	to := s.reqTimeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(r.Context(), to)
	defer cancel()

	cam, err := camera.Open(ctx, d, to)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer cam.Close()

	info, err := cam.Probe(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// handleApply handles POST /api/apply: push a profile to a set of devices,
// one at a time via the bulk orchestrator, streaming each progress event to
// the client as a Server-Sent-Events-style body so the UI can render a live,
// transparent log instead of waiting for the whole batch to finish.
func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req bulk.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.DeviceIDs) == 0 {
		writeErr(w, http.StatusBadRequest, "deviceIds is required")
		return
	}

	// Sequential apply can take a while for a large batch; scale the overall
	// deadline by device count so a big inventory isn't cut off mid-run.
	to := s.reqTimeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(r.Context(), to*time.Duration(len(req.DeviceIDs)+1))
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)

	emit := func(ev bulk.Event) {
		if r.Context().Err() != nil {
			return
		}
		b, err := json.Marshal(ev)
		if err != nil {
			log.Printf("encode apply event: %v", err)
			return
		}
		if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	bulk.Apply(ctx, s.inv, req, to, emit)
}
