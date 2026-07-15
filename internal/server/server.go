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

	// Authenticated JSON API.
	s.mux.Handle("/api/cameras", s.requireAuth(http.HandlerFunc(s.handleCameras)))
	s.mux.Handle("/api/cameras/delete", s.requireAuth(http.HandlerFunc(s.handleCamerasDelete)))
	s.mux.Handle("/api/probe", s.requireAuth(http.HandlerFunc(s.handleProbe)))
	s.mux.Handle("/api/apply", s.requireAuth(http.HandlerFunc(s.handleApply)))
	s.mux.Handle("/api/scan", s.requireAuth(http.HandlerFunc(s.handleScan)))
	s.mux.Handle("/api/import", s.requireAuth(http.HandlerFunc(s.handleImport)))
	s.mux.Handle("/api/password", s.requireAuth(http.HandlerFunc(s.handlePassword)))

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
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		user := r.PostFormValue("username")
		pass := r.PostFormValue("password")
		if s.checkCreds(user, pass) {
			token := s.session.create()
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
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
