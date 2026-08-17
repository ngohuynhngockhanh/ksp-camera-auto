package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestHandleCamerasDeleteBulkDeletesAndReportsSkipped(t *testing.T) {
	t.Setenv("KSPCAM_KEY", "bulk-delete-test-key")
	inv, err := config.LoadInventory(filepath.Join(t.TempDir(), "cameras.yaml"))
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	for _, d := range []config.Device{
		{ID: "cam-1", Host: "192.168.1.1", Port: 37777, Vendor: config.VendorDahua},
		{ID: "cam-2", Host: "192.168.1.2", Port: 37777, Vendor: config.VendorDahua},
	} {
		if err := inv.Upsert(d); err != nil {
			t.Fatalf("seed %s: %v", d.ID, err)
		}
	}
	srv, err := New(config.Default(), inv)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Close()

	body, _ := json.Marshal(map[string]any{"ids": []string{"cam-1", "cam-1", "missing"}})
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/delete-bulk", bytes.NewReader(body))
	res := httptest.NewRecorder()
	srv.handleCamerasDeleteBulk(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var got struct {
		OK      bool `json:"ok"`
		Deleted int  `json:"deleted"`
		Skipped int  `json:"skipped"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.OK || got.Deleted != 1 || got.Skipped != 1 {
		t.Fatalf("response = %+v, want ok/deleted/skipped true/1/1", got)
	}
	if _, ok := inv.Get("cam-1"); ok {
		t.Error("cam-1 still exists")
	}
	if _, ok := inv.Get("cam-2"); !ok {
		t.Error("cam-2 was deleted unexpectedly")
	}
}

func TestHandleCamerasDeleteBulkRejectsEmptyIDsAndWrongMethod(t *testing.T) {
	inv, err := config.LoadInventory(filepath.Join(t.TempDir(), "cameras.yaml"))
	if err != nil {
		t.Fatalf("load inventory: %v", err)
	}
	srv, err := New(config.Default(), inv)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Close()

	for _, tc := range []struct {
		name string
		method string
		body string
		want int
	}{
		{name: "empty", method: http.MethodPost, body: `{"ids":[]}`, want: http.StatusBadRequest},
		{name: "invalid json", method: http.MethodPost, body: `{`, want: http.StatusBadRequest},
		{name: "method", method: http.MethodDelete, body: `{}`, want: http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/cameras/delete-bulk", bytes.NewBufferString(tc.body))
			res := httptest.NewRecorder()
			srv.handleCamerasDeleteBulk(res, req)
			if res.Code != tc.want {
				t.Fatalf("status = %d, body = %s, want %d", res.Code, res.Body.String(), tc.want)
			}
		})
	}
}
