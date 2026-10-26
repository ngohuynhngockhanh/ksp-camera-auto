package bidahub

import (
	"context"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// Config defines the runtime configuration for inut-bida-hub.
type Config struct {
	BindAddr      string
	Port          int
	DataPath      string
	WatchlistPath string
	AdminUser     string
	AdminPass     string
	AdminEmail    string
	ViewerUser    string
	ViewerPass    string
	APIKey        string
	StaffSecret   string
	MailAPIURL    string
	MailAPIKey    string
	PollInterval  time.Duration
	DialTimeout   time.Duration
	MQTTBroker    string
	MQTTUser      string
	MQTTPass      string
	MQTTTopic     string
}

// Server is the Central Fleet Management Hub HTTP server for inut-bida-hub (:2090).
type Server struct {
	cfg           Config
	store         *Store
	mailer        *Mailer
	forgotMu      sync.Mutex
	forgotHits    map[string][]time.Time
	tunnelMu      sync.RWMutex
	tunnelTargets map[string]map[string]string
	yamuxSessions map[string]*yamux.Session
	sseMu         sync.RWMutex
	sseClients    map[chan string]struct{}
	mqttSub       *hubMQTTSubscriber
	stopOnce      sync.Once
	stopCh        chan struct{}
}

// NewServer initializes a new inut-bida-hub Server and seeds/loads its persistent Store.
func NewServer(cfg Config) (*Server, error) {
	if cfg.BindAddr == "" {
		cfg.BindAddr = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = 2090
	}
	if cfg.AdminUser == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_ADMIN_USER")); v != "" {
			cfg.AdminUser = v
		} else {
			cfg.AdminUser = "admin"
		}
	}
	if cfg.AdminPass == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_ADMIN_PASS")); v != "" {
			cfg.AdminPass = v
		} else {
			cfg.AdminPass = "inut2026"
		}
	}
	if strings.TrimSpace(cfg.AdminEmail) == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_ADMIN_EMAIL")); v != "" {
			cfg.AdminEmail = v
		} else {
			cfg.AdminEmail = DefaultAdminEmail
		}
	}
	if cfg.ViewerUser == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_VIEWER_USER")); v != "" {
			cfg.ViewerUser = v
		} else {
			cfg.ViewerUser = "viewer"
		}
	}
	if cfg.ViewerPass == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_VIEWER_PASS")); v != "" {
			cfg.ViewerPass = v
		} else {
			cfg.ViewerPass = "viewer2026"
		}
	}
	if cfg.APIKey == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_API_KEY")); v != "" {
			cfg.APIKey = v
		} else {
			cfg.APIKey = "inut-bida-hub-key-2026"
		}
	}
	if cfg.StaffSecret == "" {
		if v := strings.TrimSpace(os.Getenv("INUT_BIDA_STAFF_SECRET")); v != "" {
			cfg.StaffSecret = v
		} else {
			cfg.StaffSecret = "inut-bida-staff-sso-secret-2026"
		}
	}
	if strings.TrimSpace(cfg.MailAPIURL) == "" {
		for _, envKey := range []string{"INUT_BIDA_SMTP_API_URL", "KSP_PDFSIGN_SMTP_API_URL", "SMTP_API_URL"} {
			if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
				cfg.MailAPIURL = v
				break
			}
		}
		if cfg.MailAPIURL == "" {
			cfg.MailAPIURL = DefaultMailAPIURL
		}
	}
	if strings.TrimSpace(cfg.MailAPIKey) == "" {
		for _, envKey := range []string{"INUT_BIDA_SMTP_API_KEY", "KSP_PDFSIGN_API_KEY", "SMTP_API_KEY"} {
			if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
				cfg.MailAPIKey = v
				break
			}
		}
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 1500 * time.Millisecond
	}

	store, err := NewStore(cfg.DataPath, cfg.WatchlistPath)
	if err != nil {
		return nil, err
	}
	store.SeedDefaultUsers(cfg.AdminUser, cfg.AdminPass, cfg.AdminEmail, cfg.ViewerUser, cfg.ViewerPass)
	// Wipe plaintext seed passwords from memory immediately after seeding Store
	cfg.AdminPass = ""
	cfg.ViewerPass = ""

	s := &Server{
		cfg:           cfg,
		store:         store,
		mailer:        NewMailer(cfg.MailAPIURL, cfg.MailAPIKey),
		forgotHits:    make(map[string][]time.Time),
		tunnelTargets: make(map[string]map[string]string),
		yamuxSessions: make(map[string]*yamux.Session),
		sseClients:    make(map[chan string]struct{}),
		stopCh:        make(chan struct{}),
	}

	if cfg.PollInterval > 0 {
		go s.backgroundPollLoop(cfg.PollInterval)
	}
	if strings.TrimSpace(cfg.MQTTBroker) != "" {
		s.StartMQTTSubscriber(cfg.MQTTBroker, cfg.MQTTUser, cfg.MQTTPass, cfg.MQTTTopic)
	}

	return s, nil
}

func (s *Server) backgroundPollLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			s.pollFleet(ctx, "")
			cancel()
			s.broadcastSSE("fleet_update", map[string]any{
				"type": "periodic_poll",
				"kpi":  s.store.ComputeKPI(),
				"ts":   time.Now().UTC().Format(time.RFC3339),
			})
		}
	}
}

// Store returns the underlying persistent Store.
func (s *Server) Store() *Store {
	return s.store
}

// Handler returns the http.Handler for the server.
func (s *Server) Handler() http.Handler {
	return s
}

// Close stops background polling and persists state.
func (s *Server) Close() error {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		s.closeAllYamuxSessions()
		s.stopMQTTSubscriber()
	})
	return s.store.Save()
}

// ServeHTTP dispatches all HTTP requests for inut-bida-hub.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// 1. Public health check
	if path == "/api/health" || path == "/healthz" {
		s.handleHealth(w, r)
		return
	}

	// 2. Public authentication & static assets
	if path == "/login" {
		if r.Method == http.MethodPost {
			s.handleLoginPost(w, r)
			return
		}
		s.handleLoginGet(w, r)
		return
	}
	if path == "/logout" || path == "/api/auth/logout" {
		s.handleLogout(w, r)
		return
	}
	if path == "/api/auth/me" || path == "/fleet/api/auth/me" {
		s.handleAuthMe(w, r)
		return
	}
	if path == "/api/auth/forgot-password" || path == "/fleet/api/auth/forgot-password" {
		s.handleForgotPassword(w, r)
		return
	}
	if path == "/api/auth/reset-password" || path == "/fleet/api/auth/reset-password" {
		s.handleResetPassword(w, r)
		return
	}
	if path == "/api/auth/change-password" || path == "/fleet/api/auth/change-password" {
		s.handleChangePassword(w, r)
		return
	}
	if strings.HasPrefix(path, "/static/") {
		rel := strings.TrimPrefix(path, "/")
		s.serveStaticFile(w, r, rel, "")
		return
	}
	if path == "/app.js" || path == "/style.css" || path == "/portal.html" {
		s.serveStaticFile(w, r, "static"+path, "")
		return
	}

	// 3. Multi-tenant Read-Only Shop Owner Portal (/portal/{portal_token}/*)
	if path == "/portal" || strings.HasPrefix(path, "/portal/") {
		s.handlePortalRequest(w, r)
		return
	}

	// 4. Public signed clip verification (/clip/{clip_token})
	if strings.HasPrefix(path, "/clip/") {
		tok := strings.TrimPrefix(path, "/clip/")
		s.handleClipVerify(w, r, tok)
		return
	}

	// 5. Edge-to-Hub Uplink (/fleet/tunnel & /fleet/api/heartbeat)
	if path == "/fleet/tunnel" {
		s.handleTunnelEndpoint(w, r)
		return
	}
	if path == "/fleet/api/heartbeat" || path == "/api/heartbeat" {
		s.handleHeartbeat(w, r)
		return
	}
	if path == "/fleet/api/heartbeat/batch" || path == "/api/heartbeat/batch" {
		s.handleHeartbeatBatch(w, r)
		return
	}
	if path == "/api/agent/events" || path == "/fleet/api/agent/events" {
		s.handleAgentEventsPush(w, r)
		return
	}
	// 6. All remaining routes require Hub authentication (cookie, X-Hub-Key, or Bearer token)
	principal, ok := s.authenticateRequest(r)
	if !ok {
		if path == "/" || (!strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/fleet/api/") && !wantsJSON(r)) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error":   "unauthorized",
			"message": "Yêu cầu đăng nhập hoặc cung cấp X-Hub-Key / Bearer token.",
		})
		return
	}

	// Enforce Read-Only constraint for viewer role
	if principal.Role == "viewer" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":   "forbidden",
			"message": "Tài khoản viewer chỉ có quyền xem (Read-Only).",
		})
		return
	}

	// Root SPA
	if path == "/" || path == "/index.html" {
		s.serveStaticFile(w, r, "static/index.html", "text/html; charset=utf-8")
		return
	}

	// Streaming Reverse Proxy: /fleet/proxy/{box_id}/{service}/*
	if strings.HasPrefix(path, "/fleet/proxy/") {
		s.handleBoxProxy(w, r, principal)
		return
	}

	// Fleet Summary & Boxes
	if path == "/fleet/api/summary" || path == "/api/fleet/summary" || path == "/api/summary" {
		s.handleFleetSummary(w, r)
		return
	}
	if path == "/fleet/api/clubs" || path == "/api/fleet/clubs" || path == "/api/clubs" {
		s.handleFleetSummary(w, r)
		return
	}
	if path == "/fleet/api/boxes" || path == "/api/fleet/boxes" || path == "/api/boxes" {
		s.handleFleetBoxes(w, r)
		return
	}
	if strings.HasPrefix(path, "/fleet/api/boxes/") || strings.HasPrefix(path, "/api/boxes/") {
		s.handleBoxSubroutes(w, r)
		return
	}

	// Poller & Real-time SSE
	if path == "/fleet/api/poll" || path == "/api/poll" || path == "/api/fleet/poll" {
		s.handlePollNow(w, r)
		return
	}
	if path == "/fleet/api/events" {
		s.handleSSEEvents(w, r)
		return
	}

	// Enrollment Queue
	if path == "/fleet/api/enrollments" {
		s.handleEnrollments(w, r)
		return
	}
	if strings.HasPrefix(path, "/fleet/api/enrollments/") && strings.HasSuffix(path, "/approve") {
		id := strings.TrimPrefix(path, "/fleet/api/enrollments/")
		id = strings.TrimSuffix(id, "/approve")
		id = strings.Trim(id, "/")
		s.handleEnrollmentApprove(w, r, id)
		return
	}

	// 8-Step Golden Hardening
	if path == "/fleet/api/hardening" {
		s.handleHardeningList(w, r)
		return
	}

	// Multi-box Camera & NVR Inventory + Bulk Apply
	if path == "/fleet/api/cameras" {
		s.handleCamerasList(w, r)
		return
	}
	if path == "/fleet/api/cameras/bulk-apply" {
		s.handleCamerasBulkApply(w, r)
		return
	}

	// RedBida 17-Rule & FRPC Subdomain Auditor
	if path == "/fleet/api/redbida" {
		s.handleRedBidaList(w, r)
		return
	}

	// 1-Click Handover Pack & Persistent Clip Signing
	if path == "/fleet/api/handover" {
		s.handleHandoverList(w, r)
		return
	}
	if strings.HasPrefix(path, "/fleet/api/handover/") {
		id := strings.Trim(strings.TrimPrefix(path, "/fleet/api/handover/"), "/")
		s.handleHandoverSingle(w, r, id)
		return
	}
	if path == "/fleet/api/clips/sign" {
		s.handleClipSign(w, r)
		return
	}
	if path == "/fleet/api/ai-events" || path == "/api/ai-events" {
		s.handleAIEventsList(w, r)
		return
	}
	if path == "/fleet/api/ai-events/mqtt" || path == "/api/ai-events/mqtt" {
		s.handleAIEventsMQTTConfig(w, r)
		return
	}
	if path == "/fleet/api/remote-config" || path == "/api/remote-config" {
		s.handleFleetRemoteConfig(w, r)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]any{
		"error": "not_found",
		"path":  path,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	kpi := s.store.ComputeKPI()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"service":       "inut-bida-hub",
		"version":       "1.0.0",
		"port":          s.cfg.Port,
		"boxes_total":   kpi.BoxesTotal,
		"boxes_online":  kpi.BoxesOnline,
		"boxes_offline": kpi.BoxesOffline,
		"clubs_total":   kpi.ClubsTotal,
	})
}

func (s *Server) handleFleetSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"kpi":   s.store.ComputeKPI(),
		"clubs": s.store.Clubs(),
		"boxes": s.store.Boxes(),
	})
}

func (s *Server) handleFleetBoxes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"kpi":   s.store.ComputeKPI(),
		"clubs": s.store.Clubs(),
		"boxes": s.store.Boxes(),
	})
}

func (s *Server) handleBoxSubroutes(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/fleet/api/boxes/")
	relPath = strings.TrimPrefix(relPath, "/api/boxes/")
	trimmed := strings.Trim(relPath, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_box_id"})
		return
	}
	boxID := parts[0]

	if len(parts) == 1 {
		s.handleBoxDetail(w, r, boxID)
		return
	}

	sub := strings.Join(parts[1:], "/")
	switch sub {
	case "hardening", "hardening/inspect", "golden-audit":
		s.handleBoxHardeningInspect(w, r, boxID)
	case "hardening/remediate", "golden-remediate":
		s.handleBoxHardeningRemediate(w, r, boxID)
	case "redbida":
		box, ok := s.store.GetBox(boxID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "box_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, BuildRedBidaReport(box, s.store.Boxes()))
	case "redbida/apply":
		s.handleBoxRedBidaApply(w, r, boxID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown_box_subroute", "subroute": sub})
	}
}

func (s *Server) handleBoxDetail(w http.ResponseWriter, r *http.Request, boxID string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	box, ok := s.store.GetBox(boxID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":  "box_not_found",
			"box_id": boxID,
		})
		return
	}
	club, _ := s.store.GetClub(box.ClubID)
	allBoxes := s.store.Boxes()
	hardening := Build8StepHardening(box)
	redbida := BuildRedBidaReport(box, allBoxes)
	var handover *HandoverPack
	if club != nil {
		handover = BuildHandoverPack(club, s.store.GetBoxesByClub(club.ID), requestBaseURL(r))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"box":       box,
		"club":      club,
		"cameras":   box.Cameras,
		"monitors":  box.Monitors,
		"hardening": hardening,
		"redbida":   redbida,
		"handover":  handover,
	})
}

func (s *Server) serveStaticFile(w http.ResponseWriter, _ *http.Request, relPath string, contentType string) {
	relPath = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(relPath)), "/")
	if !strings.HasPrefix(relPath, "static/") {
		relPath = "static/" + relPath
	}
	data, err := fs.ReadFile(StaticFS, relPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "static_asset_not_found", "path": relPath})
		return
	}
	if contentType == "" {
		ext := filepath.Ext(relPath)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
