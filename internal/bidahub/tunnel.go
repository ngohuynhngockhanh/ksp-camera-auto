package bidahub

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/hashicorp/yamux"
)

const yamuxTunnelReadLimit = 32 << 20 // 32 MiB frame limit for multiplexed streams

func isWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return true
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		r.Header.Get("Sec-WebSocket-Key") != ""
}

// HasYamuxSession returns true if the box currently has an active outbound Yamux session.
func (s *Server) HasYamuxSession(boxID string) bool {
	return s.GetYamuxSession(boxID) != nil
}

// GetYamuxSession returns the active Yamux session for a box, pruning closed sessions.
func (s *Server) GetYamuxSession(boxID string) *yamux.Session {
	boxID = strings.TrimSpace(boxID)
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	if s.yamuxSessions == nil {
		return nil
	}
	sess := s.yamuxSessions[boxID]
	if sess != nil && sess.IsClosed() {
		delete(s.yamuxSessions, boxID)
		return nil
	}
	return sess
}

// SetYamuxSession binds a new Yamux session for boxID and returns any displaced session.
func (s *Server) SetYamuxSession(boxID string, sess *yamux.Session) *yamux.Session {
	boxID = strings.TrimSpace(boxID)
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	if s.yamuxSessions == nil {
		s.yamuxSessions = make(map[string]*yamux.Session)
	}
	old := s.yamuxSessions[boxID]
	s.yamuxSessions[boxID] = sess
	return old
}

// DelYamuxSession removes sess for boxID if it is still the active session.
func (s *Server) DelYamuxSession(boxID string, sess *yamux.Session) bool {
	boxID = strings.TrimSpace(boxID)
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	if s.yamuxSessions == nil {
		return false
	}
	if cur, ok := s.yamuxSessions[boxID]; ok && cur == sess {
		delete(s.yamuxSessions, boxID)
		return true
	}
	return false
}

func (s *Server) closeAllYamuxSessions() {
	s.tunnelMu.Lock()
	defer s.tunnelMu.Unlock()
	for id, sess := range s.yamuxSessions {
		if sess != nil {
			_ = sess.Close()
		}
		delete(s.yamuxSessions, id)
	}
}

// MeasureYamuxRTT actively pings the box's Yamux session and returns the round-trip time in milliseconds.
func (s *Server) MeasureYamuxRTT(boxID string) (int, error) {
	sess := s.GetYamuxSession(boxID)
	if sess == nil {
		return 0, errors.New("no active yamux session")
	}
	dur, err := sess.Ping()
	if err != nil {
		return 0, err
	}
	ms := int(dur.Milliseconds())
	if ms <= 0 {
		ms = 1
	}
	return ms, nil
}

func (s *Server) dialYamuxStream(ctx context.Context, sess *yamux.Session) (net.Conn, error) {
	type res struct {
		conn net.Conn
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := sess.Open()
		ch <- res{conn: c, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.conn, r.err
	}
}

func (s *Server) handleYamuxTunnelUpgrade(w http.ResponseWriter, r *http.Request) {
	boxID := strings.TrimSpace(r.URL.Query().Get("box_id"))
	if boxID == "" {
		boxID = strings.TrimSpace(r.Header.Get("X-Box-ID"))
	}
	secret := strings.TrimSpace(r.Header.Get("X-Box-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.Header.Get("X-Hub-Key"))
	}
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("secret"))
	}
	socModel := strings.TrimSpace(r.Header.Get("X-Box-SoC"))
	if socModel == "" {
		socModel = strings.TrimSpace(r.URL.Query().Get("soc_model"))
	}

	if boxID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_box_id"})
		return
	}

	if _, exists := s.store.GetBox(boxID); !exists {
		req := s.store.RecordEnrollment(boxID, secret, clientIPFromRequest(r), socModel)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"error":       "ENROLLMENT_PENDING",
			"box_id":      req.BoxID,
			"fingerprint": req.Fingerprint,
			"status":      req.Status,
		})
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("[bidahub-yamux] websocket upgrade failed for %s: %v", boxID, err)
		return
	}
	ws.SetReadLimit(yamuxTunnelReadLimit)

	nc := websocket.NetConn(context.Background(), ws, websocket.MessageBinary)
	ycfg := yamux.DefaultConfig()
	ycfg.EnableKeepAlive = true
	ycfg.KeepAliveInterval = 20 * time.Second
	ycfg.LogOutput = io.Discard

	sess, err := yamux.Client(nc, ycfg)
	if err != nil {
		_ = ws.Close(websocket.StatusInternalError, "yamux client init failed")
		log.Printf("[bidahub-yamux] yamux init failed for %s: %v", boxID, err)
		return
	}

	if old := s.SetYamuxSession(boxID, sess); old != nil {
		_ = old.Close()
	}

	rttMs := 15
	if pingMs, err := s.MeasureYamuxRTT(boxID); err == nil && pingMs > 0 {
		rttMs = pingMs
	}

	_ = s.store.UpdateBox(boxID, func(b *Box) error {
		b.Online = true
		b.TunnelActive = true
		b.Transport = "tunnel"
		b.TunnelAddr = "yamux://" + boxID
		if socModel != "" {
			b.SoCModel = socModel
		}
		b.RTTMs = rttMs
		b.RTTHistory = append(b.RTTHistory, rttMs)
		if len(b.RTTHistory) > 12 {
			b.RTTHistory = b.RTTHistory[len(b.RTTHistory)-12:]
		}
		b.LastSeen = time.Now().UTC()
		return nil
	})

	log.Printf("[bidahub-yamux] tunnel connected: box=%s rtt=%dms remote=%s", boxID, rttMs, clientIPFromRequest(r))
	s.broadcastSSE("fleet_update", map[string]any{
		"type":          "tunnel_registered",
		"box_id":        boxID,
		"tunnel_active": true,
		"rtt_ms":        rttMs,
		"kpi":           s.store.ComputeKPI(),
		"ts":            time.Now().UTC().Format(time.RFC3339),
	})

	// Immediately fetch fresh system metrics over the new Yamux session in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if payload, ok := s.fetchBoxSystemMetrics(ctx, boxID); ok {
			_ = s.store.UpdateBox(boxID, func(b *Box) error {
				applyHeartbeatToBox(b, payload, "live")
				b.Online = true
				b.TunnelActive = true
				b.Transport = "tunnel"
				b.TunnelAddr = "yamux://" + boxID
				return nil
			})
		}
	}()

	<-sess.CloseChan()

	if s.DelYamuxSession(boxID, sess) {
		_ = s.store.UpdateBox(boxID, func(b *Box) error {
			b.TunnelActive = false
			return nil
		})
		log.Printf("[bidahub-yamux] tunnel disconnected: box=%s", boxID)
		s.broadcastSSE("fleet_update", map[string]any{
			"type":          "tunnel_disconnected",
			"box_id":        boxID,
			"tunnel_active": false,
			"kpi":           s.store.ComputeKPI(),
			"ts":            time.Now().UTC().Format(time.RFC3339),
		})
	}
}
