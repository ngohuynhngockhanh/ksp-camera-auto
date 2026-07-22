package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/importer"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/mediaexport"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/tiandy"
)

// exportJob is the observable state of one chunked playback export, keyed by
// the client-chosen job id (&job= on /api/playback). The review page polls
// /api/export-progress with the same id to draw a MEGA-style in-page progress
// bar — a plain navigation download gives the page no visibility otherwise.
type exportJob struct {
	Done, Total int
	Phase       string // "start" | "fetch" | "concat" | "send" | "done" | "error"
	Error       string
	updated     time.Time
}

var (
	exportJobsMu sync.Mutex
	exportJobs   = map[string]*exportJob{}
)

// setExportJob updates (creating if needed) a job under the lock and prunes
// stale entries so the map can't grow unboundedly.
func setExportJob(id string, f func(*exportJob)) {
	exportJobsMu.Lock()
	defer exportJobsMu.Unlock()
	j := exportJobs[id]
	if j == nil {
		j = &exportJob{Phase: "start"}
		exportJobs[id] = j
	}
	f(j)
	j.updated = time.Now()
	for k, v := range exportJobs {
		if time.Since(v.updated) > 10*time.Minute {
			delete(exportJobs, k)
		}
	}
}

// handleExportProgress handles GET /api/export-progress?job= — the polling
// side of the in-page download progress bar.
func (s *Server) handleExportProgress(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("job")
	exportJobsMu.Lock()
	j, ok := exportJobs[id]
	var cp exportJob
	if ok {
		cp = *j
	}
	exportJobsMu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"done": cp.Done, "total": cp.Total, "phase": cp.Phase, "error": cp.Error})
}

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

// deviceView is the JSON projection of config.Device sent to the (already
// authenticated) web UI. Password is included in plain text — deliberately,
// at the operator's request, so the fleet's stored credentials can be seen
// without a blind reset — but this only ever reaches a session that already
// passed login, the same trust boundary every other admin action here relies
// on (ChangePassword, network/Wi-Fi config, etc.).
type deviceView struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Vendor      config.Vendor `json:"vendor"`
	Username    string        `json:"username"`
	Password    string        `json:"password"`
	NVRID       string        `json:"nvrId,omitempty"`
	NVRChannel  int           `json:"nvrChannel,omitempty"`
	NVRName     string        `json:"nvrName,omitempty"`
	ChannelName string        `json:"channelName,omitempty"`
	NoStorage   bool          `json:"noStorage,omitempty"`
	IsNVR       bool          `json:"isNvr,omitempty"`
}

func toView(d config.Device) deviceView {
	return deviceView{
		ID:          d.ID,
		Name:        d.Name,
		Host:        d.Host,
		Port:        d.Port,
		Vendor:      d.Vendor,
		Username:    d.Username,
		Password:    d.Password,
		NVRID:       d.NVRID,
		NVRChannel:  d.NVRChannel,
		NVRName:     d.NVRName,
		ChannelName: d.ChannelName,
		NoStorage:   d.NoStorage,
		IsNVR:       d.IsNVR,
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
	if req.Vendor != config.VendorDahua && req.Vendor != config.VendorHikvision && req.Vendor != config.VendorTiandy {
		writeErr(w, http.StatusBadRequest, "vendor must be dahua, hikvision or tiandy")
		return
	}
	if req.Port == 0 {
		switch req.Vendor {
		case config.VendorDahua:
			req.Port = s.cfg.Defaults.DahuaPort
		case config.VendorTiandy:
			req.Port = s.cfg.Defaults.TiandyPort
		default:
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
	// The NVR link fields aren't in this form — preserve them when editing an
	// existing device so a name/password edit doesn't wipe the fallback mapping.
	id := d.ID
	if id == "" {
		id = fmt.Sprintf("%s:%d", d.Host, d.Port)
	}
	if existing, ok := s.inv.Get(id); ok {
		d.NVRID, d.NVRChannel, d.NVRName, d.NoStorage, d.IsNVR = existing.NVRID, existing.NVRChannel, existing.NVRName, existing.NoStorage, existing.IsNVR
		d.ChannelName = existing.ChannelName
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
		// Update the stored credential so we can still connect. Re-read the
		// entry first: Open() may have just hard-set a fallback DVRIP port
		// (OnDahuaPortFallback) and the local d predates that.
		if cur, ok := s.inv.Get(id); ok {
			d = cur
		}
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

// handleFPSCapability returns a safe per-stream FPS ceiling. Vendor
// capability errors are swallowed by the camera adapter and reported as a
// fallback source, so opening the editor never fails just because caps do.
func (s *Server) handleFPSCapability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		Channel        int    `json:"channel"`
		Stream         int    `json:"stream"`
		Width          int    `json:"width"`
		Height         int    `json:"height"`
		Codec          string `json:"codec"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ID == "" || req.Channel < 0 || req.Stream < 0 || req.Stream > 2 {
		writeErr(w, http.StatusBadRequest, "id, channel and stream are required")
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()
	fps, ok := cam.(camera.FPSSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, "camera này không hỗ trợ chỉnh FPS")
		return
	}
	cap, err := fps.GetFPSCapability(ctx, req.Channel, req.Stream, req.Width, req.Height, req.Codec)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cap)
}

// openDeviceCamera resolves id from the inventory and opens a Camera with
// the request's (or default) timeout, writing a JSON error and returning
// ok=false on any failure. Callers must Close() the returned camera when
// ok is true.
func (s *Server) openDeviceCamera(w http.ResponseWriter, r *http.Request, id string, timeoutSeconds int) (cam camera.Camera, ctx context.Context, cancel context.CancelFunc, ok bool) {
	d, found := s.inv.Get(id)
	if !found {
		writeErr(w, http.StatusNotFound, "device not found")
		return nil, nil, nil, false
	}
	to := s.reqTimeout(timeoutSeconds)
	ctx, cancel = context.WithTimeout(r.Context(), to)
	cam, err := camera.Open(ctx, d, to)
	if err != nil {
		cancel()
		writeErr(w, http.StatusBadGateway, err.Error())
		return nil, nil, nil, false
	}
	return cam, ctx, cancel, true
}

// nvrDeviceFrom builds an NVR config.Device from a scan/link request, defaulting
// the port (per-vendor: Dahua's DVRIP port or Hik's ISAPI port) and carrying an
// existing stored password when the field is blank. vendor is whatever the
// request specified (see nvrScanReq.Vendor/nvrLinkReq.NVR.Vendor); an empty or
// unrecognized value falls back to VendorDahua, which is the ONLY vendor this
// endpoint supported before Hik NVR scanning existed — so old requests that
// never set the field (the frontend doesn't send it yet) keep behaving
// byte-identically to before this method grew a vendor parameter.
func (s *Server) nvrDeviceFrom(host string, port int, user, pass, name string, vendor config.Vendor) config.Device {
	if vendor != config.VendorHikvision && vendor != config.VendorTiandy {
		vendor = config.VendorDahua
	}
	if port == 0 {
		switch vendor {
		case config.VendorHikvision:
			port = s.cfg.Defaults.HikvisionPort
		case config.VendorTiandy:
			port = s.cfg.Defaults.TiandyPort
		default:
			port = s.cfg.Defaults.DahuaPort
		}
	}
	if user == "" {
		user = s.cfg.Defaults.Username
	}
	id := fmt.Sprintf("%s:%d", host, port)
	if pass == "" {
		if ex, ok := s.inv.Get(id); ok && ex.Password != "" {
			pass = ex.Password
		} else {
			pass = s.cfg.Defaults.Password
		}
	}
	return config.Device{ID: id, Name: name, Host: host, Port: port, Vendor: vendor, Username: user, Password: pass, IsNVR: true}
}

type nvrScanReq struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	// Vendor is optional and additive: omitted/empty defaults to
	// VendorDahua (nvrDeviceFrom's fallback), so existing callers that never
	// send it keep scanning a Dahua NVR exactly as before. Set to
	// "hikvision" to scan a Hik NVR's InputProxy channels instead.
	Vendor config.Vendor `json:"vendor,omitempty"`
}

type nvrScanRow struct {
	NVRChannel          int    `json:"nvrChannel"` // 1-based
	NVRCamIP            string `json:"nvrCamIP"`
	NVRCamName          string `json:"nvrCamName"`
	Enable              bool   `json:"enable"`
	SuggestedCameraID   string `json:"suggestedCameraId"`
	SuggestedCameraName string `json:"suggestedCameraName"`
	NoStorage           bool   `json:"noStorage"`
}

// handleNVRScan reads an NVR's channel→camera map (RemoteDevice) and, for each
// channel, suggests the matching inventory camera (by IP, then name) and whether
// that camera lacks usable local storage. It does NOT persist anything — the UI
// shows the result as an editable table, then POSTs /api/nvr/link to save.
func (s *Server) handleNVRScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req nvrScanReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Host == "" {
		writeErr(w, http.StatusBadRequest, "host is required")
		return
	}
	nvr := s.nvrDeviceFrom(req.Host, req.Port, req.Username, req.Password, "", req.Vendor)
	to := s.reqTimeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(r.Context(), to)
	defer cancel()
	// camera.Open dispatches on nvr.Vendor (Dahua DVRIP vs Hik ISAPI); the
	// rest of this function only ever touches the result through the
	// camera.Camera/RemoteDeviceLister/StorageManager interfaces below, so it
	// works unchanged for either vendor once the concrete type implements them
	// (hikCamera does, since Milestone 2).
	cam, err := camera.Open(ctx, nvr, to)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "mở đầu ghi lỗi: "+err.Error())
		return
	}
	defer cam.Close()
	rdl, ok := cam.(camera.RemoteDeviceLister)
	if !ok {
		writeErr(w, http.StatusBadRequest, "thiết bị này không đọc được danh sách kênh (không phải NVR?)")
		return
	}
	remotes, err := rdl.GetRemoteDevices(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "đọc RemoteDevice lỗi: "+err.Error())
		return
	}

	// Index inventory cameras by host (IP) and lower-cased name for matching.
	byHost := map[string]config.Device{}
	byName := map[string]config.Device{}
	for _, d := range s.inv.List() {
		if d.IsNVR {
			continue
		}
		byHost[d.Host] = d
		byName[strings.ToLower(strings.TrimSpace(d.Name))] = d
	}

	rows := make([]nvrScanRow, 0, len(remotes))
	type match struct {
		row int
		dev config.Device
	}
	var toCheck []match
	for _, rc := range remotes {
		row := nvrScanRow{NVRChannel: rc.Channel + 1, NVRCamIP: rc.Address, NVRCamName: rc.Name, Enable: rc.Enable}
		cand, ok := byHost[rc.Address]
		if !ok {
			cand, ok = byName[strings.ToLower(strings.TrimSpace(rc.Name))]
		}
		if ok {
			row.SuggestedCameraID = cand.ID
			row.SuggestedCameraName = cand.Name
			toCheck = append(toCheck, match{row: len(rows), dev: cand})
		}
		rows = append(rows, row)
	}

	// Check each matched camera's storage in parallel (bounded). A camera that's
	// unreachable stays NoStorage=false (unknown) — the operator can tick it.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for _, m := range toCheck {
		wg.Add(1)
		go func(m match) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cctx, ccancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer ccancel()
			cc, err := camera.Open(cctx, m.dev, 12*time.Second)
			if err != nil {
				return
			}
			defer cc.Close()
			sm, ok := cc.(camera.StorageManager)
			if !ok {
				return
			}
			devs, err := sm.GetStorageInfo(cctx)
			if err == nil && !dahua.HasUsableStorage(devs) {
				rows[m.row].NoStorage = true
			}
		}(m)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, map[string]any{"nvr": toView(nvr), "rows": rows})
}

type nvrLinkReq struct {
	NVR struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Name     string `json:"name"`
		// Vendor is optional and additive — see nvrScanReq.Vendor. Omitted/
		// empty defaults to VendorDahua, matching /api/nvr/scan.
		Vendor config.Vendor `json:"vendor,omitempty"`
	} `json:"nvr"`
	Mappings []struct {
		CameraID   string `json:"cameraId"`
		NVRChannel int    `json:"nvrChannel"` // 1-based
		NVRName    string `json:"nvrName"`
		NoStorage  bool   `json:"noStorage"`
	} `json:"mappings"`
}

// handleNVRLink persists the NVR device and writes each camera's fallback
// mapping (NVR id + channel + no-storage flag). Cameras not listed are left
// untouched; a mapping with an empty cameraId is skipped.
func (s *Server) handleNVRLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req nvrLinkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NVR.Host == "" {
		writeErr(w, http.StatusBadRequest, "NVR host is required")
		return
	}
	nvr := s.nvrDeviceFrom(req.NVR.Host, req.NVR.Port, req.NVR.Username, req.NVR.Password, req.NVR.Name, req.NVR.Vendor)
	if nvr.Name == "" {
		nvr.Name = "Đầu ghi " + req.NVR.Host
	}
	if err := s.inv.Upsert(nvr); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	linked := 0
	for _, m := range req.Mappings {
		if m.CameraID == "" {
			continue
		}
		cam, ok := s.inv.Get(m.CameraID)
		if !ok {
			continue
		}
		cam.NVRID = nvr.ID
		cam.NVRChannel = m.NVRChannel
		cam.NVRName = m.NVRName
		cam.NoStorage = m.NoStorage
		if err := s.inv.Upsert(cam); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		linked++
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nvrId": nvr.ID, "linked": linked})
}

type channelNamesReq struct {
	IDs            []string `json:"ids"` // empty = all Dahua cameras
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

// handleChannelNames probes each camera's on-device channel/OSD title (channel
// 0) and stores it as Device.ChannelName, so the review dropdown can show
// "Camera01 - <channel name>". Batch (ids empty = every Dahua camera), parallel.
func (s *Server) handleChannelNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req channelNamesReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	var targets []config.Device
	if len(req.IDs) > 0 {
		for _, id := range req.IDs {
			if d, ok := s.inv.Get(id); ok {
				targets = append(targets, d)
			}
		}
	} else {
		for _, d := range s.inv.List() {
			if d.Vendor == config.VendorDahua && !d.IsNVR {
				targets = append(targets, d)
			}
		}
	}
	to := s.reqTimeout(req.TimeoutSeconds)
	names := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for _, d := range targets {
		wg.Add(1)
		go func(d config.Device) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(r.Context(), to)
			defer cancel()
			cam, err := camera.Open(ctx, d, to)
			if err != nil {
				return
			}
			defer cam.Close()
			name, _, _, _, err := cam.ChannelInfo(ctx, 0)
			if err != nil || strings.TrimSpace(name) == "" {
				return
			}
			d.ChannelName = strings.TrimSpace(name)
			if err := s.inv.Upsert(d); err == nil {
				mu.Lock()
				names[d.ID] = d.ChannelName
				mu.Unlock()
			}
		}(d)
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "names": names, "count": len(names)})
}

// recordingSource returns the device + 0-based channel that a camera's
// RECORDINGS (timeline, playback, .dav download) should be read from. A camera
// with no local storage that is linked to an NVR reads from that NVR's channel;
// otherwise it reads from itself at reqChannel.
func (s *Server) recordingSource(cam config.Device, reqChannel int) (config.Device, int) {
	if cam.NoStorage && cam.NVRID != "" {
		if nvr, ok := s.inv.Get(cam.NVRID); ok {
			return nvr, cam.NVRChannel - 1
		}
	}
	return cam, reqChannel
}

// liveSource returns the device + 0-based channel that a camera's LIVE view /
// snapshot should come from. The camera itself is primary; only when it's
// unreachable on its DVRIP port and it's linked to an NVR do we fall back to the
// NVR channel (so an offline camera still shows a picture).
func (s *Server) liveSource(cam config.Device, reqChannel int) (config.Device, int) {
	if cam.NVRID == "" {
		return cam, reqChannel
	}
	conn, err := net.DialTimeout("tcp", cam.Addr(), 2*time.Second)
	if err == nil {
		conn.Close()
		return cam, reqChannel
	}
	if nvr, ok := s.inv.Get(cam.NVRID); ok {
		return nvr, cam.NVRChannel - 1
	}
	return cam, reqChannel
}

// handleSnapshot handles GET /api/snapshot?id=&channel=&stream=&timeoutSeconds=:
// fetch a single JPEG frame. Deliberately GET-with-query-params (unlike the
// rest of this API's POST-with-JSON-body convention) so a plain <img
// src="/api/snapshot?..."> can load it directly — the session cookie is sent
// automatically, no fetch/blob plumbing needed in the UI.
func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	id := q.Get("id")
	channel := atoiDefault(q.Get("channel"), 0)
	stream := atoiDefault(q.Get("stream"), 0)
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)

	// A cache hit skips both opening the device (a DVRIP login for Dahua) and
	// the ffmpeg decode — the point of the cache on low-RAM boxes. Resolve
	// the device first so a bad id is still a clean 404 rather than a cached
	// miss. `nocache` (or `_r` cache-bust) forces a fresh grab (the UI's
	// reload buttons send it).
	d, found := s.inv.Get(id)
	if !found {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	key := fmt.Sprintf("%s|%d|%d", id, channel, stream)
	force := q.Get("nocache") != "" || q.Get("_r") != ""

	fetch := func() ([]byte, error) {
		// On a cache miss, if the camera is offline fall back to its NVR channel
		// (liveSource only dials for NVR-linked cameras, so others are unaffected).
		src, ch := s.liveSource(d, channel)
		to := s.reqTimeout(timeoutSeconds)
		ctx, cancel := context.WithTimeout(r.Context(), to)
		defer cancel()
		cam, err := camera.Open(ctx, src, to)
		if err != nil {
			return nil, err
		}
		defer cam.Close()
		return cam.Snapshot(ctx, ch, stream)
	}

	var (
		data []byte
		err  error
	)
	if force {
		data, err = fetch()
	} else {
		data, err = s.snaps.get(key, fetch)
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// atoiDefault parses s as an int, returning def on empty/invalid input.
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// handleChannelInfo handles GET /api/channel-info?id=&channel=&timeoutSeconds=:
// read back one channel's device-side name and OSD text lines, for
// prefilling the "sửa tên & OSD" edit panel.
func (s *Server) handleChannelInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	id := q.Get("id")
	channel := atoiDefault(q.Get("channel"), 0)
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)

	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, id, timeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	name, osdLines, osdEnabled, osdSupported, err := cam.ChannelInfo(ctx, channel)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name": name, "osdLines": osdLines, "osdEnabled": osdEnabled, "osdSupported": osdSupported,
	})
}

// channelWriteReq is the shared body shape for /api/channel-name and /api/osd.
// Enabled carries each OSD line's on-screen toggle for /api/osd (ignored by
// /api/channel-name); a shorter/omitted Enabled falls back to enabling
// exactly the lines with non-empty text, so callers that don't care about
// enable state keep the old behavior for free.
type channelWriteReq struct {
	ID             string   `json:"id"`
	Channel        int      `json:"channel"`
	Name           string   `json:"name"`
	Lines          []string `json:"lines"`
	Enabled        []bool   `json:"enabled"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

// handleChannelName handles POST /api/channel-name: write the device's own
// channel name (distinct from our inventory label, which POST /api/cameras
// already covers).
func (s *Server) handleChannelName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req channelWriteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	if err := cam.SetChannelName(ctx, req.Channel, req.Name); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleOSD handles POST /api/osd: write free-text OSD overlay lines for a
// channel. appliedLines may be less than len(lines) when the device has
// fewer overlay slots than lines supplied.
func (s *Server) handleOSD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req channelWriteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	applied, err := cam.SetOSDLines(ctx, req.Channel, req.Lines, req.Enabled)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "appliedLines": applied})
}

// notDahuaErr is the message returned when a vendor-specific endpoint is hit
// for a device whose Camera implementation doesn't support that surface — e.g.
// picture tuning / PTZ (Dahua/KBVision only), or network config on a vendor
// that implements neither the Dahua nor the Hikvision path.
const notDahuaErr = "camera này không hỗ trợ tính năng này"

// handlePicture handles GET /api/picture?id=&channel=&timeoutSeconds= (read
// color+options+caps) and POST /api/picture (write changes), mirroring the
// GET/POST split already used by /api/channel-info + /api/channel-name.
// Dahua-only: the underlying camera.Camera must implement
// camera.PictureSettings (Hikvision does not).
func (s *Server) handlePicture(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handlePictureGet(w, r)
	case http.MethodPost:
		s.handlePictureSet(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handlePictureGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	channel := atoiDefault(q.Get("channel"), 0)
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)

	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, id, timeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ps, ok := cam.(camera.PictureSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	color, options, err := ps.GetPicture(ctx, channel)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	resp := map[string]any{"color": color, "options": options}
	if caps, err := ps.GetPictureCaps(ctx, channel); err == nil {
		resp["caps"] = caps
	} else {
		// Capability flags are best-effort (a separate HTTP:80 CGI call,
		// often unreachable when only the DVRIP port is forwarded/open) —
		// the UI still gets color/options and just skips capability-based
		// disabling.
		resp["capsError"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// pictureSetReq is the body shape for POST /api/picture. Color/Options carry
// only the fields being changed (merged server-side onto the live device
// config), matching dahua.Client.SetPicture's GET-modify-SET semantics.
type pictureSetReq struct {
	ID             string         `json:"id"`
	Channel        int            `json:"channel"`
	Color          map[string]any `json:"color"`
	Options        map[string]any `json:"options"`
	TimeoutSeconds int            `json:"timeoutSeconds"`
}

func (s *Server) handlePictureSet(w http.ResponseWriter, r *http.Request) {
	var req pictureSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ps, ok := cam.(camera.PictureSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	color, options, err := ps.SetPicture(ctx, req.Channel, req.Color, req.Options)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "color": color, "options": options})
}

// handleNetwork handles GET /api/network?id=&timeoutSeconds= (read the
// device's static IP / DHCP config for every interface) and POST
// /api/network (write one interface's static IP). Dahua-only. This is a
// high-risk write (a bad IP/mask/gateway can make the device unreachable) —
// dahua.Client.SetStaticIP validates every address before sending anything,
// and the UI is expected to require an explicit user confirmation before
// calling the POST at all.
func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleNetworkGet(w, r)
	case http.MethodPost:
		s.handleNetworkSet(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleNetworkGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)

	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, id, timeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ns, ok := cam.(camera.NetworkSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	cfg, err := ns.GetNetworkConfig(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// staticIPReq is the body shape for POST /api/network.
type staticIPReq struct {
	ID             string   `json:"id"`
	Interface      string   `json:"interface"`
	DhcpEnable     bool     `json:"dhcpEnable"`
	IPAddress      string   `json:"ipAddress"`
	SubnetMask     string   `json:"subnetMask"`
	Gateway        string   `json:"gateway"`
	DNS            []string `json:"dns"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

func (s *Server) handleNetworkSet(w http.ResponseWriter, r *http.Request) {
	var req staticIPReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Interface == "" {
		writeErr(w, http.StatusBadRequest, "interface is required")
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ns, ok := cam.(camera.NetworkSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := ns.SetStaticIP(ctx, req.Interface, req.DhcpEnable, req.IPAddress, req.SubnetMask, req.Gateway, req.DNS); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	// Reading the config back confirms it applied — but changing a static IP
	// moves the device onto its NEW address, so this GET (still aimed at the
	// old one) legitimately fails on success. Treat a read-back failure as a
	// soft note, not an error: the write already returned OK. The UI shows the
	// note and, for an IP change, the operator must re-add the device at the
	// new address.
	cfg, err := ns.GetNetworkConfig(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":   true,
			"note": "đã áp dụng cấu hình. Không đọc lại được — thiết bị có thể đang khởi động lại và/hoặc đã chuyển sang IP mới. Hãy chờ ~30–60s rồi kết nối lại ở địa chỉ mới.",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "network": cfg})
}

// handleWiFi handles GET /api/wifi?id=&timeoutSeconds= (read the currently
// configured SSID/security per Wi-Fi interface) and POST /api/wifi (write
// SSID/password). Dahua-only. Reading is cheap/safe (rides the existing
// DVRIP session); writing carries the same "could disconnect the device"
// risk as /api/network, so the UI should require confirmation before POSTing.
func (s *Server) handleWiFi(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleWiFiGet(w, r)
	case http.MethodPost:
		s.handleWiFiSet(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleWiFiGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id := q.Get("id")
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)

	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, id, timeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ns, ok := cam.(camera.NetworkSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	cfg, err := ns.GetWiFiConfig(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// wifiSetReq is the body shape for POST /api/wifi.
type wifiSetReq struct {
	ID             string `json:"id"`
	Interface      string `json:"interface"`
	SSID           string `json:"ssid"`
	Password       string `json:"password"`
	Encryption     string `json:"encryption"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func (s *Server) handleWiFiSet(w http.ResponseWriter, r *http.Request) {
	var req wifiSetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Interface == "" || req.SSID == "" {
		writeErr(w, http.StatusBadRequest, "interface and ssid are required")
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ns, ok := cam.(camera.NetworkSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := ns.SetWiFiConfig(ctx, req.Interface, req.SSID, req.Password, req.Encryption); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleWiFiScan handles POST /api/wifi-scan: trigger a live Wi-Fi
// access-point scan. Separate from GET /api/wifi (which just reads the
// currently configured SSID) since a scan is a slow, active operation over a
// different transport (HTTP CGI port 80, not the DVRIP session) — it may
// fail with a clean error on setups where only the DVRIP port is reachable
// (see docs/GOTCHAS.md's snapshot.cgi note for the same caveat).
func (s *Server) handleWiFiScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ns, ok := cam.(camera.NetworkSettings)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	aps, err := ns.ScanWiFi(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": aps})
}

// handlePTZ handles POST /api/ptz: issue one PTZ start/stop command for a
// Dahua camera. The UI's PTZ pad posts {start:true} on button press and
// {start:false} on release (same code), so pan/tilt runs while held.
func (s *Server) handlePTZ(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		Channel        int    `json:"channel"`
		Code           string `json:"code"`
		Speed          int    `json:"speed"`
		Start          bool   `json:"start"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ptz, ok := cam.(camera.PTZControl)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := ptz.PTZMove(ctx, req.Channel, req.Code, req.Speed, req.Start); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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

// handleReboot handles POST /api/reboot — restart one device now. Works for
// any camera implementing camera.Rebooter (Dahua via DVRIP, Hikvision via
// ISAPI). High-impact but reversible; the UI requires a confirmation.
func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ID             string `json:"id"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	rb, ok := cam.(camera.Rebooter)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := rb.Reboot(ctx); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": "đã gửi lệnh khởi động lại. Camera sẽ mất kết nối ~30–60s."})
}

// handleStorage handles GET /api/storage?id=&timeoutSeconds= (read SD-card /
// storage status) and POST /api/storage (format one device — ERASES ALL DATA).
// Dahua-only.
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleStorageGet(w, r)
	case http.MethodPost:
		s.handleStorageFormat(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleStorageGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, q.Get("id"), atoiDefault(q.Get("timeoutSeconds"), 0))
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	sm, ok := cam.(camera.StorageManager)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	info, err := sm.GetStorageInfo(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": info})
}

func (s *Server) handleStorageFormat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             string `json:"id"`
		Name           string `json:"name"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name (thiết bị lưu trữ) là bắt buộc")
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	sm, ok := cam.(camera.StorageManager)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := sm.FormatStorage(ctx, req.Name); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "note": "đã gửi lệnh format. Thẻ đang được định dạng, đọc lại sau ít giây."})
}

// handleAutoReboot handles GET /api/autoreboot?id=&timeoutSeconds= (read the
// scheduled auto-reboot) and POST /api/autoreboot (write it). Dahua-only.
func (s *Server) handleAutoReboot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAutoRebootGet(w, r)
	case http.MethodPost:
		s.handleAutoRebootSet(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAutoRebootGet(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, q.Get("id"), atoiDefault(q.Get("timeoutSeconds"), 0))
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	ar, ok := cam.(camera.AutoRebootConfig)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	cfg, err := ar.GetAutoReboot(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleAutoRebootSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             string `json:"id"`
		Enable         bool   `json:"enable"`
		Day            int    `json:"day"`
		Hour           int    `json:"hour"`
		Minute         int    `json:"minute"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, req.ID, req.TimeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	arc, ok := cam.(camera.AutoRebootConfig)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	if err := arc.SetAutoReboot(ctx, dahua.AutoReboot{Enable: req.Enable, Day: req.Day, Hour: req.Hour, Minute: req.Minute}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	cfg, err := arc.GetAutoReboot(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "autoReboot": cfg})
}

// parsePlaybackTime accepts the wall-clock time formats the recordings/playback
// UI sends (datetime-local gives "2006-01-02T15:04", with or without seconds;
// the API also accepts a space separator). The time is treated as the device's
// own local wall clock — parsed and re-formatted without any timezone shift, so
// the digits reach the camera unchanged.
func parsePlaybackTime(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("thời gian %q không hợp lệ (định dạng YYYY-MM-DDTHH:MM:SS)", s)
}

// handleRecordings handles GET /api/recordings?id=&channel=&start=&end= —
// list stored recording segments (the playback timeline) for one channel over
// a time range. Works for any vendor whose camera.Camera implements
// camera.Recorder (currently Dahua and Hikvision).
func (s *Server) handleRecordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	start, err := parsePlaybackTime(q.Get("start"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parsePlaybackTime(q.Get("end"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	channel := atoiDefault(q.Get("channel"), 0)
	d, found := s.inv.Get(q.Get("id"))
	if !found {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	// A camera with no local storage reads its recordings from the linked NVR
	// channel instead (transparent to the client).
	src, ch := s.recordingSource(d, channel)
	to := s.reqTimeout(atoiDefault(q.Get("timeoutSeconds"), 0))
	ctx, cancel := context.WithTimeout(r.Context(), to)
	defer cancel()
	cam, err := camera.Open(ctx, src, to)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	defer cam.Close()

	rec, ok := cam.(camera.Recorder)
	if !ok {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	list, err := rec.FindRecordings(ctx, ch, start, end)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recordings": list})
}

// countingWriter tracks whether any bytes have reached the client yet, so the
// playback handler knows if it can still change the HTTP status on error (once
// a byte is sent, the 200 + headers are committed).
type countingWriter struct {
	w io.Writer
	n int64
	// beforeFirst, if set, runs once right before the first byte is written —
	// the last moment HTTP headers can still be set. Lets the playback handler
	// keep the response error-able (plain JSON) until a stream actually
	// produces output.
	beforeFirst func()
}

func (c *countingWriter) Write(p []byte) (int, error) {
	if c.n == 0 && len(p) > 0 && c.beforeFirst != nil {
		c.beforeFirst()
	}
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// SetContentLength forwards the final file size to the HTTP response, so the
// browser's download UI can show a real percentage and time-remaining. Only
// the exports that build the whole file before sending (mediaexport's fast
// MP4/MKV — see fastRemux) know their size up front and call this; it must
// arrive before the first Write, after which headers are already flushed.
func (c *countingWriter) SetContentLength(size int64) {
	if c.n > 0 || size <= 0 {
		return
	}
	if rw, ok := c.w.(http.ResponseWriter); ok {
		rw.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
}

// handlePlayback handles GET /api/playback?id=&channel=&start=&end=&download= —
// stream one channel's [start,end] recording to the client as fragmented MP4,
// remuxed from the device's own RTSP playback with nothing buffered on the
// box (dahua.StreamPlayback / hik.StreamPlayback). download=1 forces a file
// download; otherwise it plays inline (HTML5 <video>). format=native (legacy
// alias: format=dav) switches to each vendor's most-original container instead
// of the MP4 remux: Dahua's DHAV .dav, Hikvision's IMKH, Tiandy's
// stream-copied MKV — see the per-vendor branch below.
func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	start, err := parsePlaybackTime(q.Get("start"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	end, err := parsePlaybackTime(q.Get("end"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if !end.After(start) {
		writeErr(w, http.StatusBadRequest, "thời gian kết thúc phải sau thời gian bắt đầu")
		return
	}
	maxHours := s.cfg.Defaults.MaxReviewHours
	if maxHours <= 0 {
		maxHours = 72
	}
	if end.Sub(start) > time.Duration(maxHours)*time.Hour {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("khoảng thời gian tối đa là %d giờ", maxHours))
		return
	}
	channel := atoiDefault(q.Get("channel"), 0)

	// Playback is pure RTSP+ffmpeg and deliberately does NOT open a DVRIP
	// session: the recorded stream comes over port 554, and skipping the DVRIP
	// login means a download isn't blocked when the camera's config port is
	// busy (these field cameras are often also recorded by another system).
	// So resolve the device straight from inventory instead of openDeviceCamera.
	d, found := s.inv.Get(q.Get("id"))
	if !found {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	// A camera with no local storage serves its recordings/downloads from the
	// linked NVR channel instead.
	d, channel = s.recordingSource(d, channel)
	if d.Vendor != config.VendorDahua && d.Vendor != config.VendorHikvision && d.Vendor != config.VendorTiandy {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	// A long range streams far faster than realtime but can still take a
	// minute+; give it a generous floor so a multi-hour download isn't cut off.
	timeoutSeconds := atoiDefault(q.Get("timeoutSeconds"), 0)
	if timeoutSeconds < 1800 {
		timeoutSeconds = 1800
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	native := q.Get("format") == "native" || q.Get("format") == "dav"
	ext, ctype := "mp4", "video/mp4"
	var streamErr error

	switch d.Vendor {
	case config.VendorDahua:
		// Playback is pure RTSP+ffmpeg and deliberately does NOT open a DVRIP
		// session: the recorded stream comes over port 554, and skipping the
		// DVRIP login means a download isn't blocked when the camera's config
		// port is busy (these field cameras are often also recorded by
		// another system). So it streams straight off d, resolved from
		// inventory above — no camera.Open here.
		//
		// Always use normal (paced) playback: it is fragmented MP4 with EVERY
		// frame, playable everywhere. The RTSP "Rate-Control: no" fast path
		// (StreamPlaybackFast) is retained but NOT used for downloads — on
		// these camera firmwares that mode only emits keyframes (~1 fps), so
		// the file looked choppy/frozen. fast=1 is accepted for backward
		// compat but routed to the same full-frame stream.
		// format=dav downloads the camera's native .dav (DHAV) over DVRIP,
		// byte-exact with no remux; anything else is the default fragmented
		// MP4 over RTSP. Both stream funcs share the same signature, so this
		// is a drop-in swap.
		stream := dahua.StreamPlayback
		if native {
			// StreamDav additionally needs the DVRIP config port (which may be
			// the KBVision 8888 instead of 37777); adapt it to the shared
			// RTSP-shaped signature by capturing d.Port.
			stream = func(ctx context.Context, w io.Writer, host, user, pass string, channel int, start, end time.Time) error {
				return dahua.StreamDav(ctx, w, host, d.Port, user, pass, channel, start, end)
			}
			ext, ctype = "dav", "application/octet-stream"
		}
		fname := fmt.Sprintf("playback_ch%d_%s.%s", channel, start.Format("20060102_150405"), ext)
		w.Header().Set("Content-Type", ctype)
		if q.Get("download") != "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fname))
		}
		cw := &countingWriter{w: w}
		streamErr = stream(ctx, cw, d.Host, d.Username, d.Password, channel, start, end)
		if streamErr != nil {
			if cw.n == 0 {
				writeErr(w, http.StatusBadGateway, streamErr.Error())
				return
			}
			log.Printf("playback %s ch%d: stream error after %d bytes: %v", q.Get("id"), channel, cw.n, streamErr)
		}
		return
	case config.VendorHikvision, config.VendorTiandy:
		// Hik and Tiandy playback/download both go through camera.Open and the
		// camera.Recorder interface (Hik over ISAPI; Tiandy over RTSP-by-time,
		// remuxed to MP4).
		cam, err := camera.Open(ctx, d, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		defer cam.Close()
		rec, ok := cam.(camera.Recorder)
		if !ok {
			writeErr(w, http.StatusBadRequest, notDahuaErr)
			return
		}
		// format=native on Hik is its own proprietary container (magic "IMKH" —
		// see hik.StreamNative): keep Hik's own ".mp4" file-naming convention
		// (that's what the device itself calls it) but an octet-stream
		// content-type since it's NOT a standard, browser-playable MP4. On
		// Tiandy it's a stream-copied MKV (no byte-download API on that
		// firmware — see tiandy.Client.StreamNative), so name it what it is.
		fast := q.Get("format") == "fastmp4"
		if native {
			ctype = "application/octet-stream"
			if d.Vendor == config.VendorTiandy {
				ext, ctype = "mkv", "video/x-matroska"
			}
		}
		// The chunked exporters (fast MP4, and Tiandy's native MKV) build the
		// whole file in the box's /tmp before sending — which is a ~1 GB tmpfs
		// on the deploy boxes, holding chunks AND the concat result at peak.
		// Cap those exports at 20 minutes (~760 MB peak for 2560×1440 HEVC) so
		// a long range fails with a clear message instead of dying mid-build
		// with an opaque one.
		if fast || (native && d.Vendor == config.VendorTiandy) {
			if end.Sub(start) > 20*time.Minute {
				writeErr(w, http.StatusBadRequest, "tải nhanh tối đa 20 phút mỗi lần — hãy cắt đoạn ngắn hơn (tốt nhất 5 phút mỗi clip)")
				return
			}
		}
		fname := fmt.Sprintf("playback_ch%d_%s.%s", channel, start.Format("20060102_150405"), ext)
		// Headers go out lazily, on the first body byte (beforeFirst): the
		// chunked exporters spend up to a minute building the file first, and
		// if that build FAILS the response must still be able to become a
		// plain JSON error the browser shows — headers already sent as
		// video/mp4 + attachment would instead turn the error into a broken
		// 1 KB "download" (or nothing at all, which reads as "nút tải không
		// ra clip").
		cw := &countingWriter{w: w}
		cw.beforeFirst = func() {
			w.Header().Set("Content-Type", ctype)
			if q.Get("download") != "" {
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fname))
			}
		}
		// &job= opts into progress reporting: the chunked exporters publish
		// fetch/concat/send phases the review page polls for its progress bar.
		jobID := q.Get("job")
		if jobID != "" {
			setExportJob(jobID, func(j *exportJob) {})
			ctx = mediaexport.WithProgress(ctx, func(done, total int, phase string) {
				setExportJob(jobID, func(j *exportJob) { j.Done, j.Total, j.Phase = done, total, phase })
			})
		}
		switch {
		case native:
			streamErr = rec.StreamNative(ctx, cw, channel, start, end)
		case fast:
			// Fast MP4: parallel RTSP chunks (Hik/Tiandy realtime → ~10× faster).
			streamErr = rec.StreamPlaybackFast(ctx, cw, channel, start, end)
		default:
			streamErr = rec.StreamPlayback(ctx, cw, channel, start, end)
		}
		if jobID != "" {
			msg := ""
			if streamErr != nil {
				msg = streamErr.Error()
			}
			setExportJob(jobID, func(j *exportJob) {
				if msg != "" {
					j.Phase, j.Error = "error", msg
				} else {
					j.Phase = "done"
				}
			})
		}
		if streamErr != nil {
			if cw.n == 0 {
				writeErr(w, http.StatusBadGateway, streamErr.Error())
				return
			}
			log.Printf("playback %s ch%d: stream error after %d bytes: %v", q.Get("id"), channel, cw.n, streamErr)
		}
	}
}

// handleLive handles GET /api/live?id=&channel=&fps= — a low-latency live view
// for realtime PTZ operation, streamed as multipart/x-mixed-replace MJPEG (an
// <img> shows it natively, no ffmpeg/HEVC-in-browser problem). It grabs frames
// over ONE DVRIP session (dahua.StreamMJPEG), writes nothing to the box's disk,
// and is capped at 5 minutes; the client reconnects to extend. Leaving the page
// drops the connection, which stops the stream (no lingering process). Dahua-only.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	d, found := s.inv.Get(q.Get("id"))
	if !found {
		writeErr(w, http.StatusNotFound, "device not found")
		return
	}
	channel := atoiDefault(q.Get("channel"), 0)
	// If the camera is offline, fall the live view back to its NVR channel.
	d, channel = s.liveSource(d, channel)
	if d.Vendor != config.VendorDahua && d.Vendor != config.VendorTiandy {
		writeErr(w, http.StatusBadRequest, notDahuaErr)
		return
	}
	fps := atoiDefault(q.Get("fps"), 6)

	// Cap a live session at 5 minutes so a forgotten tab can't hold a DVRIP
	// connection + frame loop forever; the UI reconnects to extend.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	const boundary = "kspcamframe"
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+boundary)
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}
	var liveErr error
	if d.Vendor == config.VendorTiandy {
		liveErr = tiandy.StreamMJPEG(ctx, w, flush, d.Host, d.Username, d.Password, channel, fps, boundary)
	} else {
		liveErr = dahua.StreamMJPEG(ctx, w, flush, d.Host, d.Port, d.Username, d.Password, channel, fps, boundary)
	}
	if liveErr != nil {
		log.Printf("live %s ch%d: %v", q.Get("id"), channel, liveErr)
	}
}

// playbackSig is the HMAC that authorizes a specific tokenized playback link
// (used by the QR download so a phone with no session cookie can fetch a
// pre-authorized clip). It binds the token to the exact playback params + expiry.
func (s *Server) playbackSig(id string, channel int, start, end, fast, download, exp string) string {
	mac := hmac.New(sha256.New, s.dlKey)
	fmt.Fprintf(mac, "%s|%d|%s|%s|%s|%s|%s", id, channel, start, end, fast, download, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// validPlaybackToken reports whether the request carries a valid, unexpired
// signed playback token matching its own query params.
func (s *Server) validPlaybackToken(r *http.Request) bool {
	q := r.URL.Query()
	tok, exp := q.Get("token"), q.Get("exp")
	if tok == "" || exp == "" {
		return false
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return false
	}
	want := s.playbackSig(q.Get("id"), atoiDefault(q.Get("channel"), 0), q.Get("start"), q.Get("end"), q.Get("fast"), q.Get("download"), exp)
	return hmac.Equal([]byte(tok), []byte(want))
}

// handlePlaybackToken issues a short-lived signed token for the playback params
// the caller intends to use, so the review UI can build a QR download link that
// works on a phone without a session. Session-gated (only an authenticated
// operator can mint tokens).
func (s *Server) handlePlaybackToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	exp := strconv.FormatInt(time.Now().Add(6*time.Hour).Unix(), 10)
	tok := s.playbackSig(q.Get("id"), atoiDefault(q.Get("channel"), 0), q.Get("start"), q.Get("end"), q.Get("fast"), q.Get("download"), exp)
	writeJSON(w, http.StatusOK, map[string]string{"token": tok, "exp": exp})
}

// handleConfig exposes a small bootstrap payload the web UI needs (currently
// just the review-window cap).
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	max := s.cfg.Defaults.MaxReviewHours
	if max <= 0 {
		max = 72
	}
	writeJSON(w, http.StatusOK, map[string]any{"maxReviewHours": max, "role": s.sessionRole(r)})
}
