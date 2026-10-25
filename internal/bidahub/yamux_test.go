package bidahub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func parsePortFromURL(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse url %q: %v", rawURL, err)
	}
	parts := strings.Split(u.Host, ":")
	p, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("parse port from %q: %v", u.Host, err)
	}
	return p
}

func TestYamuxReverseTunnel_MultiPortProxyAndLiveTelemetry(t *testing.T) {
	srv, _ := newTestHubServer(t)
	hubTS := httptest.NewServer(srv.Handler())
	defer hubTS.Close()

	// Spin up 3 distinct local services representing kspcam (:2028), shinobi (:8080), and nodered (:1880)
	mockKspcam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "kspcam",
			"path":    r.URL.Path,
			"sso":     r.Header.Get("X-Inut-SSO-User"),
		})
	}))
	defer mockKspcam.Close()

	mockShinobi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "shinobi",
			"path":    r.URL.Path,
		})
	}))
	defer mockShinobi.Close()

	mockNodeRed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "nodered",
			"path":    r.URL.Path,
		})
	}))
	defer mockNodeRed.Close()

	agent := NewEdgeAgent(AgentConfig{
		HubURL:      hubTS.URL,
		BoxID:       "inut_204_141",
		Secret:      testAPIKey,
		KspcamPort:  parsePortFromURL(t, mockKspcam.URL),
		ShinobiPort: parsePortFromURL(t, mockShinobi.URL),
		NodeRedPort: parsePortFromURL(t, mockNodeRed.URL),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = agent.RunTunnelOnce(ctx)
	}()

	// Wait for Yamux session to establish
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.HasYamuxSession("inut_204_141") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !srv.HasYamuxSession("inut_204_141") {
		t.Fatalf("expected active Yamux session for inut_204_141")
	}

	// Verify Yamux Ping RTT measurement
	rttMs, err := srv.MeasureYamuxRTT("inut_204_141")
	if err != nil || rttMs <= 0 {
		t.Fatalf("MeasureYamuxRTT failed: rtt=%d err=%v", rttMs, err)
	}

	// Verify multi-service reverse proxy over the single Yamux session
	for _, tc := range []struct {
		service string
		subpath string
	}{
		{"kspcam", "api/cameras"},
		{"shinobi", "super/status"},
		{"nodered", "ui/dashboard"},
	} {
		req, err := http.NewRequest(http.MethodGet, hubTS.URL+"/fleet/proxy/inut_204_141/"+tc.service+"/"+tc.subpath, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("X-Hub-Key", testAPIKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("proxy %s over yamux failed: %v", tc.service, err)
		}
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("proxy %s returned status %d", tc.service, resp.StatusCode)
		}
		if body["service"] != tc.service {
			t.Fatalf("proxy %s routed to wrong service: got %v", tc.service, body["service"])
		}
	}

	// Verify pollFleet probes live metrics over the Yamux session
	srv.pollFleet(context.Background(), "inut_204_141")
	box, ok := srv.Store().GetBox("inut_204_141")
	if !ok {
		t.Fatalf("box inut_204_141 not found")
	}
	if !box.Online || !box.TunnelActive || box.Transport != "tunnel" {
		t.Fatalf("expected Online=true TunnelActive=true Transport=tunnel, got Online=%v TunnelActive=%v Transport=%q",
			box.Online, box.TunnelActive, box.Transport)
	}
	if box.RAMTotalMB <= 0 {
		t.Fatalf("expected live RAMTotalMB > 0 from Yamux metrics probe, got %d", box.RAMTotalMB)
	}
}
