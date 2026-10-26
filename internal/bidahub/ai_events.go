package bidahub

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const maxHubRetainedAIEvents = 500

// AppendAIEvents adds one or more AI/Alarm events to the Hub Store, enriching
// each event with BoxName, ClubID, and ClubName from the registered Box.
func (s *Store) AppendAIEvents(events []*AIEvent) []*AIEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	var added []*AIEvent
	for _, ev := range events {
		if ev == nil {
			continue
		}
		cp := *ev
		if cp.ReceivedAt.IsZero() {
			cp.ReceivedAt = now
		}
		if cp.Timestamp == "" {
			cp.Timestamp = now.Format(time.RFC3339)
		}
		if b, ok := s.boxByID[cp.BoxID]; ok && b != nil {
			if cp.BoxName == "" {
				cp.BoxName = b.Name
			}
			if cp.ClubID == "" {
				cp.ClubID = b.ClubID
			}
			if cp.ClubName == "" {
				cp.ClubName = b.ClubName
			}
		}
		if cp.ID == "" {
			sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%s|%d",
				cp.BoxID, cp.DeviceID, cp.Channel, cp.EventType, cp.Timestamp, now.UnixNano())))
			cp.ID = "hub_ev_" + hex.EncodeToString(sum[:8])
		}
		// Deduplicate by ID
		dup := false
		for i := len(s.aiEvents) - 1; i >= 0 && i >= len(s.aiEvents)-50; i-- {
			if s.aiEvents[i].ID == cp.ID {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		s.aiEvents = append(s.aiEvents, &cp)
		added = append(added, &cp)
	}
	if len(s.aiEvents) > maxHubRetainedAIEvents {
		s.aiEvents = s.aiEvents[len(s.aiEvents)-maxHubRetainedAIEvents:]
	}
	_ = s.saveLocked()
	return added
}

// ListAIEvents returns retained AI/Alarm events (newest first), optionally
// filtered by clubID, boxID, deviceID, and eventType.
func (s *Store) ListAIEvents(clubID, boxID, deviceID, eventType string, limit int) []*AIEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.aiEvents) {
		limit = len(s.aiEvents)
	}
	var out []*AIEvent
	for i := len(s.aiEvents) - 1; i >= 0 && len(out) < limit; i-- {
		ev := s.aiEvents[i]
		if clubID != "" && !strings.EqualFold(ev.ClubID, clubID) {
			continue
		}
		if boxID != "" && !strings.EqualFold(ev.BoxID, boxID) {
			continue
		}
		if deviceID != "" && !strings.EqualFold(ev.DeviceID, deviceID) {
			continue
		}
		if eventType != "" && !strings.EqualFold(ev.EventType, eventType) {
			continue
		}
		cp := *ev
		out = append(out, &cp)
	}
	return out
}

type hubMQTTSubscriber struct {
	mu        sync.RWMutex
	broker    string
	username  string
	password  string
	topic     string
	client    mqtt.Client
	connected bool
	received  int64
}

// StartMQTTSubscriber connects the Hub to an MQTT broker and subscribes to
// kspcam event topics (default "kspcam/events/#") to ingest real-time AI/alarm events.
func (s *Server) StartMQTTSubscriber(broker, username, password, topic string) error {
	s.stopMQTTSubscriber()
	broker = strings.TrimSpace(broker)
	if broker == "" {
		return nil
	}
	if !strings.Contains(broker, "://") {
		if _, _, err := net.SplitHostPort(broker); err != nil {
			broker = net.JoinHostPort(broker, "1883")
		}
		broker = "tcp://" + broker
	}
	if strings.TrimSpace(topic) == "" {
		topic = "kspcam/events/#"
	}

	sub := &hubMQTTSubscriber{
		broker:   broker,
		username: username,
		password: password,
		topic:    topic,
	}

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("inut-bida-hub-%d", time.Now().UnixNano()%100000)).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectTimeout(5 * time.Second)
	if username != "" {
		opts.SetUsername(username)
		opts.SetPassword(password)
	}
	opts.OnConnect = func(cl mqtt.Client) {
		sub.mu.Lock()
		sub.connected = true
		sub.mu.Unlock()
		cl.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
			if msg.Retained() {
				return
			}
			var ev AIEvent
			if err := json.Unmarshal(msg.Payload(), &ev); err != nil {
				return
			}
			if ev.Source == "" {
				ev.Source = "mqtt"
			}
			sub.mu.Lock()
			sub.received++
			sub.mu.Unlock()
			added := s.store.AppendAIEvents([]*AIEvent{&ev})
			for _, a := range added {
				s.broadcastSSE("ai_event", a)
			}
		})
	}
	opts.OnConnectionLost = func(_ mqtt.Client, _ error) {
		sub.mu.Lock()
		sub.connected = false
		sub.mu.Unlock()
	}

	cl := mqtt.NewClient(opts)
	sub.client = cl
	s.mqttSub = sub

	tok := cl.Connect()
	if !tok.WaitTimeout(3 * time.Second) {
		return fmt.Errorf("mqtt connect timeout to %s", broker)
	}
	return tok.Error()
}

func (s *Server) stopMQTTSubscriber() {
	if s.mqttSub != nil {
		s.mqttSub.mu.Lock()
		if s.mqttSub.client != nil && s.mqttSub.client.IsConnected() {
			s.mqttSub.client.Disconnect(100)
		}
		s.mqttSub.connected = false
		s.mqttSub.mu.Unlock()
		s.mqttSub = nil
	}
}

func (s *Server) mqttSubscriberStatus() map[string]any {
	if s.mqttSub == nil {
		return map[string]any{
			"enabled":   false,
			"connected": false,
		}
	}
	s.mqttSub.mu.RLock()
	defer s.mqttSub.mu.RUnlock()
	return map[string]any{
		"enabled":   true,
		"broker":    s.mqttSub.broker,
		"topic":     s.mqttSub.topic,
		"connected": s.mqttSub.connected && s.mqttSub.client != nil && s.mqttSub.client.IsConnected(),
		"received":  s.mqttSub.received,
	}
}

// handleAgentEventsPush handles POST /api/agent/events from edge kspcam EventBus.
func (s *Server) handleAgentEventsPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	secret := strings.TrimSpace(r.Header.Get("X-Box-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.Header.Get("X-Hub-Key"))
	}
	if s.cfg.APIKey != "" && secret != "" && secret != s.cfg.APIKey {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_secret"})
		return
	}

	var req struct {
		BoxID  string     `json:"box_id"`
		Events []*AIEvent `json:"events"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		// Also accept a single AIEvent JSON object
		var single AIEvent
		if err2 := json.Unmarshal(body, &single); err2 == nil && single.EventType != "" {
			req.BoxID = single.BoxID
			req.Events = []*AIEvent{&single}
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
	}
	hdrBoxID := strings.TrimSpace(r.Header.Get("X-Box-ID"))
	for _, ev := range req.Events {
		if ev == nil {
			continue
		}
		if ev.BoxID == "" {
			if req.BoxID != "" {
				ev.BoxID = req.BoxID
			} else {
				ev.BoxID = hdrBoxID
			}
		}
		if ev.Source == "" {
			ev.Source = "http_push"
		}
	}

	added := s.store.AppendAIEvents(req.Events)
	for _, a := range added {
		s.broadcastSSE("ai_event", a)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"ingested": len(added),
	})
}

// handleAIEventsList handles GET /fleet/api/ai-events and POST (manual inject/test).
func (s *Server) handleAIEventsList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		clubID := strings.TrimSpace(q.Get("club_id"))
		boxID := strings.TrimSpace(q.Get("box_id"))
		deviceID := strings.TrimSpace(q.Get("device_id"))
		eventType := strings.TrimSpace(q.Get("event_type"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 100
		}
		events := s.store.ListAIEvents(clubID, boxID, deviceID, eventType, limit)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"events": events,
			"total":  len(events),
			"mqtt":   s.mqttSubscriberStatus(),
		})
	case http.MethodPost:
		s.handleAgentEventsPush(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
	}
}

// handleAIEventsMQTTConfig handles GET/POST /fleet/api/ai-events/mqtt to configure
// the Hub's MQTT subscriber bridge dynamically.
func (s *Server) handleAIEventsMQTTConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.mqttSubscriberStatus())
	case http.MethodPost:
		var req struct {
			Enable   bool   `json:"enable"`
			Broker   string `json:"broker"`
			Username string `json:"username"`
			Password string `json:"password"`
			Topic    string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		if !req.Enable {
			s.stopMQTTSubscriber()
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":   true,
				"mqtt": s.mqttSubscriberStatus(),
			})
			return
		}
		err := s.StartMQTTSubscriber(req.Broker, req.Username, req.Password, req.Topic)
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    err == nil,
			"error": errStr,
			"mqtt":  s.mqttSubscriberStatus(),
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
	}
}

// handleFleetRemoteConfig handles POST /fleet/api/remote-config to dispatch any
// kspcam configuration action (e.g. /api/ai/setting, /api/ai/linkage,
// /api/raysharp/raw, /api/events/config, /api/apply) to a target Box's kspcam
// service over the Yamux tunnel or VPN.
func (s *Server) handleFleetRemoteConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	var req struct {
		BoxID   string `json:"box_id"`
		Method  string `json:"method"`  // "GET" or "POST" (default "POST")
		Path    string `json:"path"`    // e.g. "/api/ai/setting", "/api/raysharp/raw", "/api/events/config"
		Payload any    `json:"payload"` // JSON body forwarded to box's kspcam
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json", "message": err.Error()})
		return
	}
	req.BoxID = strings.TrimSpace(req.BoxID)
	req.Path = strings.TrimSpace(req.Path)
	if req.BoxID == "" || req.Path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "box_id and path are required"})
		return
	}
	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	var bodyReader io.Reader
	if req.Payload != nil && method != http.MethodGet {
		b, err := json.Marshal(req.Payload)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		bodyReader = bytes.NewReader(b)
	}

	targetURL := fmt.Sprintf("http://%s:2028%s", req.BoxID, req.Path)
	outReq, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	outReq.Header.Set("Content-Type", "application/json")
	ts := time.Now().UTC().Unix()
	outReq.Header.Set("X-Inut-Staff-Sig", s.SignStaffRequest(req.BoxID, "admin", "superadmin", ts))
	outReq.Header.Set("X-Inut-SSO-User", "admin")
	outReq.Header.Set("X-Inut-SSO-Role", "superadmin")
	outReq.Header.Set("X-Inut-SSO-Ts", strconv.FormatInt(ts, 10))
	outReq.Header.Set("X-Inut-Box-ID", req.BoxID)
	outReq.SetBasicAuth("admin", "smarthome12345")

	tr := &http.Transport{
		DialContext: func(dCtx context.Context, _, _ string) (net.Conn, error) {
			return s.DialBox(dCtx, req.BoxID, 2028)
		},
		ResponseHeaderTimeout: 20 * time.Second,
	}
	cl := &http.Client{Transport: tr, Timeout: 25 * time.Second}
	resp, err := cl.Do(outReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":     false,
			"box_id": req.BoxID,
			"path":   req.Path,
			"error":  err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	var parsed any
	if json.Unmarshal(respBytes, &parsed) != nil {
		parsed = string(respBytes)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"box_id":      req.BoxID,
		"path":        req.Path,
		"status_code": resp.StatusCode,
		"data":        parsed,
	})
}
