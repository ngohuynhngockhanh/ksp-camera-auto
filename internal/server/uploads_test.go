package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHandleUploads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kspcam-uploads-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := &Server{
		uploadsDir: tmpDir,
	}

	testFile := filepath.Join(tmpDir, "clip.mp4")
	if err := os.WriteFile(testFile, []byte("fake-mp4-data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// 1. Success GET
	req := httptest.NewRequest("GET", "/uploads/clip.mp4", nil)
	rr := httptest.NewRecorder()
	s.handleUploads(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", rr.Code)
	}
	if rr.Body.String() != "fake-mp4-data" {
		t.Errorf("expected body 'fake-mp4-data', got %q", rr.Body.String())
	}

	// 2. 404 Not Found
	req404 := httptest.NewRequest("GET", "/uploads/nonexistent.mp4", nil)
	rr404 := httptest.NewRecorder()
	s.handleUploads(rr404, req404)
	if rr404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rr404.Code)
	}

	// 3. Path traversal protection
	reqTrav := httptest.NewRequest("GET", "/uploads/../secret.txt", nil)
	rrTrav := httptest.NewRecorder()
	s.handleUploads(rrTrav, reqTrav)
	if rrTrav.Code != http.StatusBadRequest && rrTrav.Code != http.StatusNotFound {
		t.Errorf("expected 400 or 404 on path traversal, got %d", rrTrav.Code)
	}

	// 4. Method not allowed
	reqPost := httptest.NewRequest("POST", "/uploads/clip.mp4", nil)
	rrPost := httptest.NewRecorder()
	s.handleUploads(rrPost, reqPost)
	if rrPost.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rrPost.Code)
	}
}

func TestUploadsCleaner(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kspcam-cleaner-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldFile := filepath.Join(tmpDir, "old.mp4")
	newFile := filepath.Join(tmpDir, "new.mp4")

	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set oldFile modtime to 25 hours ago
	oldTime := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	s := &Server{
		uploadsDir:  tmpDir,
		uploadsStop: make(chan struct{}),
	}
	s.startUploadsCleaner(24 * time.Hour)
	defer close(s.uploadsStop)

	// Cleaner runs clean() immediately on start in goroutine; give it a brief moment
	time.Sleep(100 * time.Millisecond)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old.mp4 to be deleted by cleaner, but it still exists")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Errorf("expected new.mp4 to still exist, but got: %v", err)
	}
}
