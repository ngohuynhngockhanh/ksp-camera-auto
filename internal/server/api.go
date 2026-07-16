package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

	cam, ctx, cancel, ok := s.openDeviceCamera(w, r, id, timeoutSeconds)
	if !ok {
		return
	}
	defer cancel()
	defer cam.Close()

	data, err := cam.Snapshot(ctx, channel, stream)
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

// notDahuaErr is the message returned when a picture/network endpoint is hit
// for a device whose Camera implementation doesn't support that
// vendor-specific surface (i.e. anything but Dahua/KBVision).
const notDahuaErr = "camera này không hỗ trợ tính năng này (chỉ Dahua/KBVision)"

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
	cfg, err := ns.GetNetworkConfig(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
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
