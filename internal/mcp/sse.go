package mcp

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// Session represents an active SSE client session.
type Session struct {
	ID       string
	outgoing chan []byte
	created  time.Time
}

// HTTPHandler implements the HTTP / SSE transport for MCP.
type HTTPHandler struct {
	server *Server
	cfg    config.MCPConfig
	mu     sync.RWMutex
}

// NewHTTPHandler creates a new HTTP transport handler for the MCP server.
func NewHTTPHandler(server *Server, cfg config.MCPConfig) *HTTPHandler {
	return &HTTPHandler{
		server: server,
		cfg:    cfg,
	}
}

// ServeHTTP routes incoming requests to SSE stream or message POST endpoints.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case path == "/mcp" && r.Method == http.MethodGet:
		h.ServeSSE(w, r)
	case path == "/mcp" && r.Method == http.MethodPost:
		h.ServeDirect(w, r)
	case path == "/mcp/messages" && r.Method == http.MethodPost:
		h.ServeMessages(w, r)
	case r.Method == http.MethodOptions:
		h.handleCORS(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (h *HTTPHandler) handleCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-MCP-Key, X-Session-ID")
	w.WriteHeader(http.StatusOK)
}

// checkAuth validates the request against configured MCP security policies.
func (h *HTTPHandler) checkAuth(r *http.Request) bool {
	// If loopback requests are allowed unauthenticated, verify client IP
	if h.cfg.AllowUnauthenticatedLoopback {
		ip := clientIP(r)
		if isLoopbackIP(ip) {
			return true
		}
	}

	// If no API key is configured and loopback not required, allow (or fallback)
	if h.cfg.APIKey == "" {
		return true
	}

	// Check header "X-MCP-Key"
	if key := r.Header.Get("X-MCP-Key"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(h.cfg.APIKey)) == 1 {
			return true
		}
	}

	// Check Bearer Token in "Authorization"
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.cfg.APIKey)) == 1 {
			return true
		}
	}

	// Check query param "?key=" or "?apiKey="
	q := r.URL.Query()
	if key := q.Get("key"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(h.cfg.APIKey)) == 1 {
			return true
		}
	}
	if key := q.Get("apiKey"); key != "" {
		if subtle.ConstantTimeCompare([]byte(key), []byte(h.cfg.APIKey)) == 1 {
			return true
		}
	}

	return false
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isLoopbackIP(ipStr string) bool {
	ipStr = strings.Trim(ipStr, "[]")
	if ipStr == "localhost" {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) registerSession(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

func (s *Server) unregisterSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func (s *Server) getSession(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

// ServeSSE handles GET /mcp by establishing an SSE stream and sending the endpoint notification.
func (h *HTTPHandler) ServeSSE(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	sessionID := generateSessionID()
	sess := &Session{
		ID:       sessionID,
		outgoing: make(chan []byte, 64),
		created:  time.Now(),
	}
	h.server.registerSession(sess)
	defer h.server.unregisterSession(sessionID)

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	// Send initial endpoint event
	endpointURL := fmt.Sprintf("/mcp/messages?sessionId=%s", sessionID)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-sess.outgoing:
			if !ok {
				return
			}
			_, writeErr := fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			if writeErr != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// ServeMessages handles POST /mcp/messages by ingesting client JSON-RPC requests.
func (h *HTTPHandler) ServeMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = r.URL.Query().Get("session_id")
	}
	if sessionID == "" {
		sessionID = r.Header.Get("X-Session-ID")
	}

	if sessionID == "" {
		http.Error(w, "missing sessionId parameter", http.StatusBadRequest)
		return
	}

	sess, ok := h.server.getSession(sessionID)
	if !ok {
		http.Error(w, "session not found or expired", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	respBytes, isNotification, err := h.server.ProcessMessage(r.Context(), body)
	if err != nil {
		log.Printf("mcp process message error: %v", err)
	}

	if !isNotification && len(respBytes) > 0 {
		select {
		case sess.outgoing <- respBytes:
		default:
			log.Printf("mcp session %s outgoing buffer full", sessionID)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

// ServeDirect handles stateless POST /mcp requests directly returning the JSON-RPC response.
func (h *HTTPHandler) ServeDirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if !h.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	respBytes, isNotification, err := h.server.ProcessMessage(r.Context(), body)
	if err != nil {
		http.Error(w, "internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isNotification || len(respBytes) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}
