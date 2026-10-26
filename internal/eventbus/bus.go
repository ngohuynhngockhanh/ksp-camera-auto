// Package eventbus implements the unified AI & Alarm Event Bus for kspcam.
//
// It combines:
//  1. Active Long-Polling on Raysharp NVRs and AI IP Cameras (/API/Event/Check
//     with plus_eventchk="eventAiPushPic" and ring-buffer cursor tracking).
//  2. Inbound HTTP Webhook ingestion (POST /api/events/webhook/raysharp) for
//     devices configured via /API/AlarmConfig/EventPush or /API/Event/Notification.
//  3. Automatic deduplication and bounded in-memory retention.
//  4. Real-time MQTT Event Publishing to configurable MQTT brokers
//     (topic: <prefix>/<box_id>/<device_id>/<channel>/<event_type>) with
//     optional Base64 snapshot stripping for low-bandwidth brokers.
//  5. Direct HTTP/Yamux Event Pushing to kspcam Hub (inut-bida-hub).
package eventbus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/raysharp"
)

// Event represents a normalized AI or system alarm event emitted by a camera or NVR.
type Event struct {
	ID            string         `json:"id"`
	BoxID         string         `json:"box_id"`
	DeviceID      string         `json:"device_id"`
	DeviceName    string         `json:"device_name,omitempty"`
	Vendor        string         `json:"vendor"`
	Host          string         `json:"host"`
	Channel       int            `json:"channel"`                // 1-based channel number (0 for system-level)
	ChannelKey    string         `json:"channel_key,omitempty"`  // e.g. "CH2"
	ChannelName   string         `json:"channel_name,omitempty"` // e.g. "Camera"
	EventType     string         `json:"event_type"`             // e.g. "human_vehicle", "face_detection", "license_plate", "line_crossing", "intrusion", "motion", "disk_nodisk"
	RawSubType    string         `json:"raw_sub_type,omitempty"`
	Source        string         `json:"source"` // "longpoll", "webhook", "audit"
	Timestamp     string         `json:"timestamp"`
	ReceivedAt    time.Time      `json:"received_at"`
	SnapshotB64   string         `json:"snapshot_b64,omitempty"`
	BackgroundB64 string         `json:"background_b64,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Status summarizes the runtime health and counters of the Event Bus.
type Status struct {
	Enabled             bool      `json:"enabled"`
	BoxID               string    `json:"box_id"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
	MQTTEnabled         bool      `json:"mqtt_enabled"`
	MQTTConnected       bool      `json:"mqtt_connected"`
	MQTTBroker          string    `json:"mqtt_broker"`
	MQTTTopicPrefix     string    `json:"mqtt_topic_prefix"`
	MQTTIncludeSnapshot bool      `json:"mqtt_include_snapshot"`
	HubPushEnabled      bool      `json:"hub_push_enabled"`
	HubURL              string    `json:"hub_url,omitempty"`
	TotalEvents         int64     `json:"total_events"`
	MQTTPublished       int64     `json:"mqtt_published"`
	HubPushed           int64     `json:"hub_pushed"`
	RetainedCount       int       `json:"retained_count"`
	LastEventAt         time.Time `json:"last_event_at,omitempty"`
}

// Bus manages event polling, webhook ingestion, MQTT publishing, Hub pushing,
// and local SSE broadcasting.
type Bus struct {
	mu          sync.RWMutex
	cfg         config.EventsConfig
	inv         *config.Inventory
	events      []Event
	dedup       map[string]time.Time
	subs        map[chan Event]struct{}
	mqttClient  mqtt.Client
	httpClient  *http.Client
	totalEvents int64
	mqttPubCnt  int64
	hubPushCnt  int64
	lastEventAt time.Time
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// New creates a new Event Bus instance.
func New(cfg config.EventsConfig, inv *config.Inventory) *Bus {
	if cfg.BoxID == "" {
		cfg.BoxID = detectBoxID()
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 2
	}
	if cfg.MQTTBrokerHost == "" {
		cfg.MQTTBrokerHost = "127.0.0.1"
	}
	if cfg.MQTTBrokerPort <= 0 {
		cfg.MQTTBrokerPort = 12369
	}
	if cfg.MQTTTopicPrefix == "" {
		cfg.MQTTTopicPrefix = "kspcam/events"
	}
	if cfg.MaxRetainedEvents <= 0 {
		cfg.MaxRetainedEvents = 200
	}
	return &Bus{
		cfg:        cfg,
		inv:        inv,
		events:     make([]Event, 0, cfg.MaxRetainedEvents),
		dedup:      make(map[string]time.Time),
		subs:       make(map[chan Event]struct{}),
		httpClient: &http.Client{Timeout: 8 * time.Second},
		stopCh:     make(chan struct{}),
	}
}

func detectBoxID() string {
	for _, k := range []string{"INUT_BOX_ID", "BOX_ID"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	if raw, err := os.ReadFile("/etc/tinc/inut/tinc.conf"); err == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
			if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Name") {
				if v := strings.TrimSpace(parts[1]); v != "" {
					return v
				}
			}
		}
	}
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return strings.ReplaceAll(strings.TrimSpace(h), "-", "_")
	}
	return "kspcam_edge"
}

// Start launches the background MQTT connection and Raysharp Event/Check poller.
func (b *Bus) Start(ctx context.Context) {
	b.mu.Lock()
	if b.cfg.MQTTEnabled {
		b.connectMQTTLocked()
	}
	enabled := b.cfg.Enabled
	b.mu.Unlock()

	if enabled {
		go b.pollLoop(ctx)
	}
}

// Close stops background polling and disconnects the MQTT client.
func (b *Bus) Close() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
		b.mu.Lock()
		if b.mqttClient != nil && b.mqttClient.IsConnected() {
			b.mqttClient.Disconnect(250)
		}
		b.mu.Unlock()
	})
}

func (b *Bus) connectMQTTLocked() {
	if b.mqttClient != nil && b.mqttClient.IsConnected() {
		return
	}
	brokerURL := fmt.Sprintf("tcp://%s:%d", b.cfg.MQTTBrokerHost, b.cfg.MQTTBrokerPort)
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(fmt.Sprintf("kspcam-events-%s-%d", b.cfg.BoxID, time.Now().UnixNano()%100000)).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectTimeout(5 * time.Second)
	if b.cfg.MQTTUsername != "" {
		opts.SetUsername(b.cfg.MQTTUsername)
		opts.SetPassword(b.cfg.MQTTPassword)
	}
	cl := mqtt.NewClient(opts)
	tok := cl.Connect()
	tok.WaitTimeout(3 * time.Second)
	b.mqttClient = cl
}

// UpdateConfig dynamically updates the Event Bus configuration.
func (b *Bus) UpdateConfig(cfg config.EventsConfig) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if cfg.BoxID == "" {
		cfg.BoxID = b.cfg.BoxID
	}
	if cfg.PollIntervalSeconds <= 0 {
		cfg.PollIntervalSeconds = 2
	}
	if cfg.MQTTBrokerHost == "" {
		cfg.MQTTBrokerHost = "127.0.0.1"
	}
	if cfg.MQTTBrokerPort <= 0 {
		cfg.MQTTBrokerPort = 12369
	}
	if cfg.MQTTTopicPrefix == "" {
		cfg.MQTTTopicPrefix = "kspcam/events"
	}
	if cfg.MaxRetainedEvents <= 0 {
		cfg.MaxRetainedEvents = 200
	}
	b.cfg = cfg
	if b.mqttClient != nil && b.mqttClient.IsConnected() {
		b.mqttClient.Disconnect(100)
		b.mqttClient = nil
	}
	if cfg.MQTTEnabled {
		b.connectMQTTLocked()
	}
}

// GetConfig returns a copy of the current EventsConfig.
func (b *Bus) GetConfig() config.EventsConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cfg
}

// GetStatus returns the current status and metrics of the Event Bus.
func (b *Bus) GetStatus() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	mqttConn := b.mqttClient != nil && b.mqttClient.IsConnected()
	return Status{
		Enabled:             b.cfg.Enabled,
		BoxID:               b.cfg.BoxID,
		PollIntervalSeconds: b.cfg.PollIntervalSeconds,
		MQTTEnabled:         b.cfg.MQTTEnabled,
		MQTTConnected:       mqttConn,
		MQTTBroker:          fmt.Sprintf("%s:%d", b.cfg.MQTTBrokerHost, b.cfg.MQTTBrokerPort),
		MQTTTopicPrefix:     b.cfg.MQTTTopicPrefix,
		MQTTIncludeSnapshot: b.cfg.MQTTIncludeSnapshot,
		HubPushEnabled:      b.cfg.HubPushEnabled,
		HubURL:              b.cfg.HubURL,
		TotalEvents:         b.totalEvents,
		MQTTPublished:       b.mqttPubCnt,
		HubPushed:           b.hubPushCnt,
		RetainedCount:       len(b.events),
		LastEventAt:         b.lastEventAt,
	}
}

// ListEvents returns retained events (newest first), optionally filtered by
// deviceID and eventType, up to limit.
func (b *Bus) ListEvents(deviceID, eventType string, limit int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if limit <= 0 || limit > len(b.events) {
		limit = len(b.events)
	}
	var out []Event
	for i := len(b.events) - 1; i >= 0 && len(out) < limit; i-- {
		ev := b.events[i]
		if deviceID != "" && !strings.EqualFold(ev.DeviceID, deviceID) {
			continue
		}
		if eventType != "" && !strings.EqualFold(ev.EventType, eventType) {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// Subscribe registers a live event channel for SSE streaming.
func (b *Bus) Subscribe() (chan Event, func()) {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	unsub := func() {
		b.mu.Lock()
		delete(b.subs, ch)
		close(ch)
		b.mu.Unlock()
	}
	return ch, unsub
}

// Publish records an event, deduplicates bursts, broadcasts to SSE subscribers,
// publishes to MQTT, and pushes to Hub.
func (b *Bus) Publish(ev Event) bool {
	now := time.Now().UTC()
	if ev.ReceivedAt.IsZero() {
		ev.ReceivedAt = now
	}
	b.mu.Lock()
	if ev.BoxID == "" {
		ev.BoxID = b.cfg.BoxID
	}
	if ev.Timestamp == "" {
		ev.Timestamp = now.Format(time.RFC3339)
	}
	dedupKey := fmt.Sprintf("%s|%s|%d|%s|%s", ev.DeviceID, ev.Host, ev.Channel, ev.EventType, ev.Timestamp)
	if last, seen := b.dedup[dedupKey]; seen && now.Sub(last) < 5*time.Second {
		b.mu.Unlock()
		return false
	}
	// Prune old dedup entries
	if len(b.dedup) > 1000 {
		for k, t := range b.dedup {
			if now.Sub(t) > 30*time.Second {
				delete(b.dedup, k)
			}
		}
	}
	b.dedup[dedupKey] = now

	if ev.ID == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", dedupKey, now.UnixNano())))
		ev.ID = "ev_" + hex.EncodeToString(sum[:8])
	}

	b.totalEvents++
	b.lastEventAt = now
	b.events = append(b.events, ev)
	if len(b.events) > b.cfg.MaxRetainedEvents {
		b.events = b.events[len(b.events)-b.cfg.MaxRetainedEvents:]
	}

	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}

	cfg := b.cfg
	mqttCl := b.mqttClient
	b.mu.Unlock()

	if cfg.MQTTEnabled {
		b.publishMQTT(cfg, mqttCl, ev)
	}
	if cfg.HubPushEnabled && cfg.HubURL != "" {
		go b.pushToHub(cfg, ev)
	}
	return true
}

func (b *Bus) publishMQTT(cfg config.EventsConfig, cl mqtt.Client, ev Event) {
	if cl == nil || !cl.IsConnected() {
		b.mu.Lock()
		b.connectMQTTLocked()
		cl = b.mqttClient
		b.mu.Unlock()
		if cl == nil || !cl.IsConnected() {
			return
		}
	}
	outEv := ev
	if !cfg.MQTTIncludeSnapshot {
		outEv.SnapshotB64 = ""
		outEv.BackgroundB64 = ""
	}
	data, err := json.Marshal(outEv)
	if err != nil {
		return
	}
	prefix := strings.TrimSuffix(cfg.MQTTTopicPrefix, "/")
	devSlug := sanitizeTopicSegment(ev.DeviceID)
	if devSlug == "" {
		devSlug = sanitizeTopicSegment(ev.Host)
	}
	topic := fmt.Sprintf("%s/%s/%s/ch%d/%s", prefix, sanitizeTopicSegment(ev.BoxID), devSlug, ev.Channel, sanitizeTopicSegment(ev.EventType))
	tok := cl.Publish(topic, 0, false, data)
	if tok.WaitTimeout(2*time.Second) && tok.Error() == nil {
		b.mu.Lock()
		b.mqttPubCnt++
		b.mu.Unlock()
	}
}

func sanitizeTopicSegment(s string) string {
	s = strings.TrimSpace(s)
	var out strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			out.WriteRune(r)
		} else {
			out.WriteByte('_')
		}
	}
	return out.String()
}

func (b *Bus) pushToHub(cfg config.EventsConfig, ev Event) {
	url := strings.TrimRight(cfg.HubURL, "/") + "/api/agent/events"
	body, err := json.Marshal(map[string]any{
		"box_id": cfg.BoxID,
		"events": []Event{ev},
	})
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Box-ID", cfg.BoxID)
	if cfg.HubSecret != "" {
		req.Header.Set("X-Box-Secret", cfg.HubSecret)
		req.Header.Set("X-Hub-Key", cfg.HubSecret)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		b.mu.Lock()
		b.hubPushCnt++
		b.mu.Unlock()
	}
}

// IngestRaysharpWebhook parses an inbound Raysharp HTTP webhook payload (from
// NVR /API/AlarmConfig/EventPush or IPC /API/Event/Notification) and publishes
// the normalized events to the bus.
func (b *Bus) IngestRaysharpWebhook(remoteHost string, rawBody []byte) ([]Event, error) {
	var raw map[string]any
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return nil, fmt.Errorf("invalid webhook json: %w", err)
	}

	devID := remoteHost
	devName := remoteHost
	if b.inv != nil {
		if d, ok := b.inv.FindByHost(remoteHost, 0); ok {
			devID = d.ID
			if d.Name != "" {
				devName = d.Name
			}
		}
	}

	// Check if payload wraps data in {"data": ...}
	dataMap := raw
	if d, ok := raw["data"].(map[string]any); ok {
		dataMap = d
	}

	var emitted []Event
	// Case 1: Event/Check / AlarmList style webhook
	if _, hasAlarmList := dataMap["alarm_list"]; hasAlarmList || dataMap["ai_snap_picture"] != nil {
		// Reuse raysharp CheckEventItem parser via roundtrip
		var env struct {
			AlarmList     json.RawMessage `json:"alarm_list"`
			AISnapPicture *struct {
				FaceInfo      []map[string]any `json:"FaceInfo"`
				SnapedObjInfo []map[string]any `json:"SnapedObjInfo"`
				PlateInfo     []map[string]any `json:"PlateInfo"`
			} `json:"ai_snap_picture"`
		}
		bData, _ := json.Marshal(dataMap)
		if json.Unmarshal(bData, &env) == nil && env.AISnapPicture != nil {
			for _, f := range env.AISnapPicture.FaceInfo {
				ev := b.fromCheckItem(devID, devName, remoteHost, "webhook", raysharp.CheckEventItem{
					Channel:     0,
					EventType:   "face_detection",
					SnapshotB64: stringField(f, "FaceImg"),
					Metadata:    f,
				})
				b.Publish(ev)
				emitted = append(emitted, ev)
			}
			for _, o := range env.AISnapPicture.SnapedObjInfo {
				ev := b.fromCheckItem(devID, devName, remoteHost, "webhook", raysharp.CheckEventItem{
					Channel:     0,
					EventType:   "human_vehicle",
					SnapshotB64: stringField(o, "ObjectImg"),
					Metadata:    o,
				})
				b.Publish(ev)
				emitted = append(emitted, ev)
			}
			for _, p := range env.AISnapPicture.PlateInfo {
				ev := b.fromCheckItem(devID, devName, remoteHost, "webhook", raysharp.CheckEventItem{
					Channel:     0,
					EventType:   "license_plate",
					SnapshotB64: stringField(p, "PlateImg"),
					Metadata:    p,
				})
				b.Publish(ev)
				emitted = append(emitted, ev)
			}
		}
	}

	if len(emitted) == 0 {
		evType := stringField(dataMap, "event_type", "alarm_type", "type", "ability", "sub_type")
		if evType == "" {
			evType = "raysharp_webhook"
		}
		chKey := stringField(dataMap, "channel", "Chn", "chn")
		chIdx, ok := raysharp.ParseChannelKey(chKey)
		chNum := 1
		if ok {
			chNum = chIdx + 1
		}
		snap := stringField(dataMap, "snapshot_b64", "img_data", "FaceImg", "ObjectImg", "PlateImg")
		ev := Event{
			DeviceID:    devID,
			DeviceName:  devName,
			Vendor:      "raysharp",
			Host:        remoteHost,
			Channel:     chNum,
			ChannelKey:  chKey,
			EventType:   strings.ToLower(evType),
			Source:      "webhook",
			SnapshotB64: snap,
			Metadata:    dataMap,
		}
		b.Publish(ev)
		emitted = append(emitted, ev)
	}
	return emitted, nil
}

func stringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (b *Bus) fromCheckItem(devID, devName, host, source string, item raysharp.CheckEventItem) Event {
	ch1 := item.Channel + 1
	if item.Channel < 0 {
		ch1 = 0
	}
	return Event{
		DeviceID:      devID,
		DeviceName:    devName,
		Vendor:        "raysharp",
		Host:          host,
		Channel:       ch1,
		ChannelKey:    item.ChannelKey,
		ChannelName:   item.ChannelName,
		EventType:     item.EventType,
		RawSubType:    item.RawSubType,
		Source:        source,
		Timestamp:     item.Timestamp,
		SnapshotB64:   item.SnapshotB64,
		BackgroundB64: item.BackgroundB64,
		Metadata:      item.Metadata,
	}
}

// PollOnce actively polls all Raysharp devices in the inventory once via
// POST /API/Event/Check and publishes any detected events.
func (b *Bus) PollOnce(ctx context.Context, cursors map[string]*raysharp.EventCheckCursor) int {
	if b.inv == nil {
		return 0
	}
	devs := b.inv.List()
	seenHosts := make(map[string]bool)
	emitted := 0

	for _, d := range devs {
		if d.Vendor != config.VendorRaysharp || d.Host == "" {
			continue
		}
		hostKey := fmt.Sprintf("%s:%d", d.Host, d.Port)
		if seenHosts[hostKey] {
			continue
		}
		seenHosts[hostKey] = true

		cur := cursors[hostKey]
		if cur == nil {
			cur = &raysharp.EventCheckCursor{}
			cursors[hostKey] = cur
		}

		cl := raysharp.New(d.Host, d.Port, d.Username, d.Password, 6*time.Second)
		items, err := cl.PollEventCheck(ctx, cur)
		_ = cl.Close()
		if err != nil {
			continue
		}
		for _, item := range items {
			ev := b.fromCheckItem(d.ID, d.Name, d.Host, "longpoll", item)
			if b.Publish(ev) {
				emitted++
			}
		}
	}
	return emitted
}

func (b *Bus) pollLoop(ctx context.Context) {
	cursors := make(map[string]*raysharp.EventCheckCursor)
	for {
		b.mu.RLock()
		enabled := b.cfg.Enabled
		interval := time.Duration(b.cfg.PollIntervalSeconds) * time.Second
		b.mu.RUnlock()
		if interval < time.Second {
			interval = 2 * time.Second
		}

		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-time.After(interval):
			if !enabled {
				continue
			}
			pCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			b.PollOnce(pCtx, cursors)
			cancel()
		}
	}
}
