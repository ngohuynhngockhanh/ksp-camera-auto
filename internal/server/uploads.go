package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultUploadsDir = "/opt/ksp-cam/uploads"

// handleUploads serves publicly accessible files placed into uploadsDir (e.g. temporary customer clip downloads).
func (s *Server) handleUploads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/uploads/")
	rel = filepath.Clean(rel)
	if rel == "." || rel == "/" || strings.HasPrefix(rel, "..") || strings.Contains(rel, string(filepath.Separator)+"..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(s.uploadsDir, rel)
	fi, err := os.Stat(fullPath)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, fullPath)
}

// startUploadsCleaner purges files older than maxAge from uploadsDir periodically.
func (s *Server) startUploadsCleaner(maxAge time.Duration) {
	go func() {
		clean := func() {
			if s.uploadsDir == "" {
				return
			}
			entries, err := os.ReadDir(s.uploadsDir)
			if err != nil {
				return
			}
			now := time.Now()
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				if now.Sub(info.ModTime()) > maxAge {
					p := filepath.Join(s.uploadsDir, e.Name())
					log.Printf("uploads cleaner: removing expired file %s (age %v > %v)", p, now.Sub(info.ModTime()), maxAge)
					_ = os.Remove(p)
				}
			}
		}

		clean()
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-s.uploadsStop:
				return
			case <-ticker.C:
				clean()
			}
		}
	}()
}
