package bidahub

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestHubAIEventsIngestAndFilter(t *testing.T) {
	tmp := t.TempDir()
	testKey := t.Name()
	srv, err := NewServer(Config{
		DataPath: filepath.Join(tmp, "hub.json"),
		APIKey:   testKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// 1. Push AI event from edge agent via POST /api/agent/events
	pushBody, _ := json.Marshal(map[string]any{
		"box_id": "inut_207_44",
		"events": []map[string]any{
			{
				"device_id":    "192.168.2.100:80",
				"device_name":  "Cam AI 3K",
				"vendor":       "raysharp",
				"host":         "192.168.2.100",
				"channel":      2,
				"channel_key":  "CH2",
				"event_type":   "human_vehicle",
				"snapshot_b64": "dGVzdA==",
			},
		},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/agent/events", bytes.NewReader(pushBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Hub-Key", testKey)
	rec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("POST /api/agent/events status=%d body=%s", rec1.Code, rec1.Body.String())
	}

	// 2. Query GET /fleet/api/ai-events?box_id=inut_207_44&event_type=human_vehicle
	req2 := httptest.NewRequest(http.MethodGet, "/fleet/api/ai-events?box_id=inut_207_44&event_type=human_vehicle", nil)
	req2.Header.Set("X-Hub-Key", testKey)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET /fleet/api/ai-events status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	var listResp struct {
		OK     bool       `json:"ok"`
		Total  int        `json:"total"`
		Events []*AIEvent `json:"events"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if !listResp.OK || listResp.Total != 1 || listResp.Events[0].EventType != "human_vehicle" {
		t.Fatalf("unexpected AI events list response: %+v", listResp)
	}
}
