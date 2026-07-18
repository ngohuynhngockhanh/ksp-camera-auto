// Package server hosts the web UI and its HTTP API. It provides session-cookie
// login (single admin account from config) and serves the embedded static UI.
package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/web"
)

const sessionCookie = "kspcam_session"

// Server is the HTTP server and its dependencies.
type Server struct {
	cfg     config.Config
	inv     *config.Inventory
	mux     *http.ServeMux
	static  fs.FS
	session *sessionStore
	limiter *loginLimiter
	snaps   *snapCache
}

// New builds a Server with routes registered.
func New(cfg config.Config, inv *config.Inventory) (*Server, error) {
	static, err := web.Static()
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:     cfg,
		inv:     inv,
		mux:     http.NewServeMux(),
		static:  static,
		session: newSessionStore(12 * time.Hour),
		limiter: newLoginLimiter(cfg.Server.LoginMaxAttempts, time.Duration(cfg.Server.LoginLockoutMinutes)*time.Minute),
		snaps:   newSnapCache(),
	}
	s.routes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	// Public endpoints.
	s.mux.HandleFunc("/login", s.handleLogin)
	s.mux.HandleFunc("/logout", s.handleLogout)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Authenticated JSON API. api() gates on the session and caps the request
	// body (DoS guard on constrained boxes).
	api := func(h http.HandlerFunc) http.Handler {
		return s.requireAuth(limitBody(8<<20, h))
	}
	s.mux.Handle("/api/cameras", api(s.handleCameras))
	s.mux.Handle("/api/cameras/delete", api(s.handleCamerasDelete))
	s.mux.Handle("/api/probe", api(s.handleProbe))
	s.mux.Handle("/api/apply", api(s.handleApply))
	s.mux.Handle("/api/scan", api(s.handleScan))
	s.mux.Handle("/api/import", api(s.handleImport))
	s.mux.Handle("/api/password", api(s.handlePassword))
	s.mux.Handle("/api/snapshot", api(s.handleSnapshot))
	s.mux.Handle("/api/channel-info", api(s.handleChannelInfo))
	s.mux.Handle("/api/channel-name", api(s.handleChannelName))
	s.mux.Handle("/api/osd", api(s.handleOSD))
	s.mux.Handle("/api/picture", api(s.handlePicture))
	s.mux.Handle("/api/network", api(s.handleNetwork))
	s.mux.Handle("/api/wifi", api(s.handleWiFi))
	s.mux.Handle("/api/wifi-scan", api(s.handleWiFiScan))
	s.mux.Handle("/api/scan/try-password", api(s.handleTryPassword))
	s.mux.Handle("/api/ptz", api(s.handlePTZ))
	s.mux.Handle("/api/reboot", api(s.handleReboot))
	s.mux.Handle("/api/storage", api(s.handleStorage))
	s.mux.Handle("/api/autoreboot", api(s.handleAutoReboot))
	s.mux.Handle("/api/recordings", api(s.handleRecordings))
	s.mux.Handle("/api/playback", api(s.handlePlayback))

	// Authenticated app + static assets.
	fileServer := http.FileServer(http.FS(s.static))
	s.mux.Handle("/", s.requireAuth(fileServer))
}

// requireAuth gates a handler behind a valid session, redirecting to the login
// page otherwise. The login page itself is served publicly from /login.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authed(r) {
			// Serve the login page for HTML navigations; 401 for API calls.
			if wantsHTML(r) {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitBody caps the request body size to guard constrained boxes against
// memory-exhaustion via oversized JSON payloads.
func limitBody(n int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

// loginLimiter throttles failed logins per client IP to blunt online brute
// force against an internet-exposed login. maxAttempts/lockout are set from
// config.Server.LoginMaxAttempts/LoginLockoutMinutes (see newLoginLimiter).
type loginLimiter struct {
	mu          sync.Mutex
	fail        map[string]*attempt
	maxAttempts int
	lockout     time.Duration
}
type attempt struct {
	count int
	until time.Time
}

// newLoginLimiter builds a limiter with the given threshold/lockout window.
// maxAttempts <= 0 falls back to 5, lockout <= 0 falls back to 30 minutes —
// config.Default() already sets both, so this only guards a caller that
// builds a loginLimiter directly with a zero-value config.
func newLoginLimiter(maxAttempts int, lockout time.Duration) *loginLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if lockout <= 0 {
		lockout = 30 * time.Minute
	}
	return &loginLimiter{fail: map[string]*attempt{}, maxAttempts: maxAttempts, lockout: lockout}
}

// blocked reports whether ip is currently locked out. Also opportunistically
// prunes ip's entry once its lockout has fully expired, so the map doesn't
// grow forever from IPs that failed once and never came back.
func (l *loginLimiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.fail[ip]
	if a == nil {
		return false
	}
	if time.Now().After(a.until) {
		delete(l.fail, ip)
		return false
	}
	return a.count >= l.maxAttempts
}

func (l *loginLimiter) fail1(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.fail[ip]
	if a == nil {
		a = &attempt{}
		l.fail[ip] = a
	}
	a.count++
	a.until = time.Now().Add(l.lockout)
}

func (l *loginLimiter) reset(ip string) {
	l.mu.Lock()
	delete(l.fail, ip)
	l.mu.Unlock()
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isHTTPS reports whether the request reached us over TLS (directly or via a
// trusted reverse proxy) so the session cookie can be marked Secure.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// wantsHTML reports whether the request looks like a browser navigation (so we
// can redirect to the login page) rather than an API/fetch call (which gets 401).
func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func (s *Server) authed(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.session.valid(c.Value)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Already logged in? go to app.
		if s.authed(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.serveStatic(w, r, "login.html")
	case http.MethodPost:
		ip := clientIP(r)
		if s.limiter.blocked(ip) {
			http.Redirect(w, r, "/login?err=locked", http.StatusSeeOther)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		user := r.PostFormValue("username")
		pass := r.PostFormValue("password")
		if s.checkCreds(user, pass) {
			s.limiter.reset(ip)
			token := s.session.create()
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   isHTTPS(r),
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.limiter.fail1(ip)
		http.Redirect(w, r, "/login?err=1", http.StatusSeeOther)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.session.destroy(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// checkCreds validates the login. The username is compared in constant time;
// the password is checked against a bcrypt hash when server.password_hash is
// set, else against the plaintext server.password (constant time).
func (s *Server) checkCreds(user, pass string) bool {
	if subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.Server.Username)) != 1 {
		return false
	}
	if h := s.cfg.Server.PasswordHash; h != "" {
		return bcrypt.CompareHashAndPassword([]byte(h), []byte(pass)) == nil
	}
	return subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Server.Password)) == 1
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request, name string) {
	f, err := s.static.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("serve %s: %v", name, err)
	}
}

// --- session store ---

type sessionStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	toks map[string]time.Time
}

func newSessionStore(ttl time.Duration) *sessionStore {
	return &sessionStore{ttl: ttl, toks: map[string]time.Time{}}
}

func (st *sessionStore) create() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	st.mu.Lock()
	st.toks[tok] = time.Now().Add(st.ttl)
	st.mu.Unlock()
	return tok
}

func (st *sessionStore) valid(tok string) bool {
	if tok == "" {
		return false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	exp, ok := st.toks[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(st.toks, tok)
		return false
	}
	return true
}

func (st *sessionStore) destroy(tok string) {
	st.mu.Lock()
	delete(st.toks, tok)
	st.mu.Unlock()
}
