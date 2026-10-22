package bidahub

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type portalBoxView struct {
	ID                       string `json:"id"`
	Name                     string `json:"name"`
	Online                   bool   `json:"online"`
	TunnelActive             bool   `json:"tunnel_active"`
	NVRHost                  string `json:"nvr_host,omitempty"`
	CamerasCount             int    `json:"cameras_count"`
	ShinobiMonitorsRecording int    `json:"shinobi_monitors_recording"`
	SDTotalGB                int    `json:"sd_total_gb"`
	SDUsedGB                 int    `json:"sd_used_gb"`
	SDUsedPct                int    `json:"sd_used_pct"`
	SDFSType                 string `json:"sd_fs_type"`
	StorageHealthy           bool   `json:"storage_healthy"`
	StoreURL                 string `json:"store_url"`
	CStoreURL                string `json:"cstore_url"`
}

type portalCameraView struct {
	ID         string `json:"id"`
	DeviceID   string `json:"device_id"`
	BoxID      string `json:"box_id"`
	BoxName    string `json:"box_name"`
	TableNo    int    `json:"table_no"`
	Name       string `json:"name"`
	TableName  string `json:"table_name"`
	Vendor     string `json:"vendor"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	NVRChannel int    `json:"nvr_channel"`
	Online     bool   `json:"online"`
	Recording  bool   `json:"recording"`
	Codec      string `json:"codec"`
	Resolution string `json:"resolution"`
	FPS        int    `json:"fps"`
}
func (s *Server) handlePortalRequest(w http.ResponseWriter, r *http.Request) {
	// Strictly Read-Only: reject any mutating HTTP method
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error":   "method_not_allowed",
			"message": "Cổng Chủ Quán chỉ hỗ trợ chế độ Xem (Read-Only).",
		})
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/portal/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "portal_not_found"})
		return
	}

	parts := strings.SplitN(trimmed, "/", 2)
	token := strings.TrimSpace(parts[0])
	subPath := ""
	if len(parts) == 2 {
		subPath = strings.Trim(parts[1], "/")
	}

	// Strictly lookup by 32-hex portal_token only (never match raw box_id or club_id)
	club, ok := s.store.GetClubByPortalToken(token)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":   "portal_not_found",
			"message": "Không tìm thấy Cổng Chủ Quán hoặc mã truy cập không hợp lệ.",
		})
		return
	}

	if subPath == "" {
		s.serveStaticFile(w, r, "static/portal.html", "text/html; charset=utf-8")
		return
	}
	switch subPath {
	case "api/status", "status":
		s.handlePortalStatus(w, r, club)
		return
	case "api/p2p/devices":
		s.handlePortalP2PDevices(w, r, club, "/api/cameras")
		return
	case "api/p2p/snapshot":
		s.handlePortalP2PProxy(w, r, club, "/api/snapshot", 15*time.Second)
		return
	case "api/p2p/live":
		s.handlePortalP2PProxy(w, r, club, "/api/live", 30*time.Second)
		return
	case "api/p2p/recordings":
		s.handlePortalP2PProxy(w, r, club, "/api/recordings", 60*time.Second)
		return
	case "api/p2p/playback":
		s.handlePortalP2PProxy(w, r, club, "/api/playback", 120*time.Second)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]any{"error": "portal_endpoint_not_found"})
}

func (s *Server) validateClubBox(w http.ResponseWriter, r *http.Request, club *Club) (string, bool) {
	boxID := strings.TrimSpace(r.URL.Query().Get("box_id"))
	rawBoxes := s.store.GetBoxesByClub(club.ID)
	if boxID == "" && len(rawBoxes) > 0 {
		return rawBoxes[0].ID, true
	}
	for _, b := range rawBoxes {
		if b.ID == boxID {
			return b.ID, true
		}
	}
	writeJSON(w, http.StatusForbidden, map[string]any{
		"error":   "box_forbidden",
		"message": "Thiết bị Box không thuộc CLB này.",
	})
	return "", false
}

func (s *Server) handlePortalP2PDevices(w http.ResponseWriter, r *http.Request, club *Club, targetPath string) {
	boxID, ok := s.validateClubBox(w, r, club)
	if !ok {
		return
	}
	const targetPort = 2028
	ts := time.Now().UTC().Unix()
	staffSig := s.SignStaffRequest(boxID, "portal_viewer", "viewer", ts)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				return s.DialBox(dialCtx, boxID, targetPort)
			},
		},
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+boxID+":2028"+targetPath, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	req.Header.Set("X-Inut-Staff-Sig", staffSig)
	req.Header.Set("X-Inut-SSO-User", "portal_viewer")
	req.Header.Set("X-Inut-SSO-Role", "viewer")
	req.Header.Set("X-Inut-SSO-Ts", strconv.FormatInt(ts, 10))
	req.Header.Set("X-Inut-Box-ID", boxID)
	req.Header.Set("X-Target-Port", "2028")
	req.Header.Set("X-Target-Service", "kspcam")

	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  "p2p_tunnel_unreachable",
			"box_id": boxID,
			"detail": err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var rawDevs []map[string]any
	if err := json.Unmarshal(body, &rawDevs); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, bytes.NewReader(body))
		return
	}
	for _, d := range rawDevs {
		delete(d, "password")
		d["box_id"] = boxID
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"box_id":  boxID,
		"devices": rawDevs,
	})
}

func (s *Server) handlePortalP2PProxy(w http.ResponseWriter, r *http.Request, club *Club, targetPath string, headerTimeout time.Duration) {
	boxID, ok := s.validateClubBox(w, r, club)
	if !ok {
		return
	}
	const targetPort = 2028
	ts := time.Now().UTC().Unix()
	staffSig := s.SignStaffRequest(boxID, "portal_viewer", "viewer", ts)

	targetURL := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(boxID, strconv.Itoa(targetPort)),
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return s.DialBox(ctx, boxID, targetPort)
		},
		DisableCompression:    true,
		ResponseHeaderTimeout: headerTimeout,
		IdleConnTimeout:       30 * time.Second,
	}
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = targetPath
			req.URL.RawPath = ""
			req.Host = targetURL.Host
			req.Header.Del("Accept-Encoding")
			req.Header.Set("X-Inut-Staff-Sig", staffSig)
			req.Header.Set("X-Inut-SSO-User", "portal_viewer")
			req.Header.Set("X-Inut-SSO-Role", "viewer")
			req.Header.Set("X-Inut-SSO-Ts", strconv.FormatInt(ts, 10))
			req.Header.Set("X-Inut-Box-ID", boxID)
			req.Header.Set("X-Target-Port", strconv.Itoa(targetPort))
			req.Header.Set("X-Target-Service", "kspcam")
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			writeJSON(rw, http.StatusBadGateway, map[string]any{
				"error":  "p2p_tunnel_unreachable",
				"box_id": boxID,
				"detail": err.Error(),
			})
		},
	}
	rp.ServeHTTP(w, r)
}

func (s *Server) handlePortalStatus(w http.ResponseWriter, _ *http.Request, club *Club) {
	rawBoxes := s.store.GetBoxesByClub(club.ID)
	boxViews := make([]portalBoxView, 0, len(rawBoxes))
	camViews := make([]portalCameraView, 0)

	boxesOnline := 0
	camsTotal := 0
	camsOnline := 0
	camsRecording := 0
	storageHealthy := true

	for _, b := range rawBoxes {
		if b.Online {
			boxesOnline++
		}
		healthy := b.SDUsedPct < 90 && b.CorruptedMP4Count == 0
		if !healthy {
			storageHealthy = false
		}
		storeURL := ""
		if b.StoreDomain != "" {
			storeURL = "https://" + b.StoreDomain
		}
		cstoreURL := ""
		if b.CStoreDomain != "" {
			cstoreURL = "https://" + b.CStoreDomain
		}
		boxViews = append(boxViews, portalBoxView{
			ID:                       b.ID,
			Name:                     b.Name,
			Online:                   b.Online,
			TunnelActive:             b.TunnelActive,
			NVRHost:                  b.NVRHost,
			CamerasCount:             len(b.Cameras),
			ShinobiMonitorsRecording: b.ShinobiMonitorsRecording,
			SDTotalGB:                b.SDTotalGB,
			SDUsedGB:                 b.SDUsedGB,
			SDUsedPct:                b.SDUsedPct,
			SDFSType:                 b.SDFSType,
			StorageHealthy:           healthy,
			StoreURL:                 storeURL,
			CStoreURL:                cstoreURL,
		})

		for _, c := range b.Cameras {
			camsTotal++
			if c.Online {
				camsOnline++
			}
			if c.Recording {
				camsRecording++
			}
			port := c.Port
			if port <= 0 {
				port = 37777
			}
			devID := c.Host + ":" + strconv.Itoa(port)
			camViews = append(camViews, portalCameraView{
				ID:         c.Name,
				DeviceID:   devID,
				BoxID:      b.ID,
				BoxName:    b.Name,
				TableNo:    c.TableNo,
				Name:       c.Name,
				TableName:  c.TableName,
				Vendor:     c.Vendor,
				Host:       c.Host,
				Port:       port,
				NVRChannel: c.NVRChannel,
				Online:     c.Online,
				Recording:  c.Recording,
				Codec:      c.Codec,
				Resolution: c.Resolution,
				FPS:        c.FPS,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"club": map[string]any{
			"id":        club.ID,
			"name":      club.Name,
			"branding":  club.Branding,
			"store_url": club.StoreURL,
			"cstore_url": club.CStoreURL,
			"portal_url": "/portal/" + club.PortalToken + "/",
		},
		"summary": map[string]any{
			"boxes_total":       len(boxViews),
			"boxes_online":      boxesOnline,
			"cameras_total":     camsTotal,
			"cameras_online":    camsOnline,
			"cameras_recording": camsRecording,
			"storage_healthy":   storageHealthy,
		},
		"boxes":   boxViews,
		"cameras": camViews,
	})
}
