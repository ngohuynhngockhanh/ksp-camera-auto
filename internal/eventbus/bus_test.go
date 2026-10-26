package eventbus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestEventBusWebhookDedupAndHubPush(t *testing.T) {
	var (
		hubMu     sync.Mutex
		hubEvents []Event
	)
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/events" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			BoxID  string  `json:"box_id"`
			Events []Event `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		hubMu.Lock()
		hubEvents = append(hubEvents, payload.Events...)
		hubMu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer hubSrv.Close()

	invPath := filepath.Join(t.TempDir(), "cameras.yaml")
	inv, err := config.LoadInventory(invPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = inv.Upsert(config.Device{
		ID:       "192.168.2.100:80",
		Name:     "Cam AI 3K",
		Host:     "192.168.2.100",
		Port:     80,
		Vendor:   config.VendorRaysharp,
		Username: "admin",
		Password: "admin1234",
	})

	bus := New(config.EventsConfig{
		Enabled:           true,
		BoxID:             "inut_207_70",
		HubPushEnabled:    true,
		HubURL:            hubSrv.URL,
		HubSecret:         "test-secret",
		MaxRetainedEvents: 50,
	}, inv)
	defer bus.Close()

	subCh, unsub := bus.Subscribe()
	defer unsub()

	rawWebhook := []byte(`{"channel":"CH1","event_type":"human_vehicle","snapshot_b64":"ZmFrZQ=="}`)
	ingested, err := bus.IngestRaysharpWebhook("192.168.2.100", rawWebhook)
	if err != nil || len(ingested) != 1 {
		t.Fatalf("IngestRaysharpWebhook failed: err=%v len=%d", err, len(ingested))
	}

	select {
	case ev := <-subCh:
		if ev.DeviceID != "192.168.2.100:80" || ev.EventType != "human_vehicle" {
			t.Fatalf("unexpected SSE event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE event")
	}

	// Wait briefly for async Hub push
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hubMu.Lock()
		n := len(hubEvents)
		hubMu.Unlock()
		if n >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	hubMu.Lock()
	defer hubMu.Unlock()
	if len(hubEvents) != 1 || hubEvents[0].BoxID != "inut_207_70" {
		t.Fatalf("expected 1 hub event from inut_207_70, got %+v", hubEvents)
	}

	list := bus.ListEvents("192.168.2.100:80", "human_vehicle", 10)
	if len(list) != 1 {
		t.Fatalf("expected 1 retained event, got %d", len(list))
	}
}
