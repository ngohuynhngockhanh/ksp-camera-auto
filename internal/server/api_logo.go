package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultLogoPath = "/opt/ksp-cam/logo.png"

func (s *Server) handleLogoFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	candidates := []string{
		defaultLogoPath,
		"logo.png",
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, p)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) handleUploadLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var imgBytes []byte
	var keyName string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			writeErr(w, http.StatusBadRequest, "parse form: "+err.Error())
			return
		}
		keyName = r.FormValue("key")
		file, _, err := r.FormFile("file")
		if err != nil {
			file, _, err = r.FormFile("logo")
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, "missing file field: "+err.Error())
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "read file: "+err.Error())
			return
		}
		imgBytes = data
	} else {
		var req struct {
			Image string `json:"image"`
			Key   string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		keyName = req.Key
		raw := req.Image
		if idx := strings.Index(raw, ","); idx != -1 && strings.HasPrefix(raw, "data:") {
			raw = raw[idx+1:]
		}
		data, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "decode base64: "+err.Error())
			return
		}
		imgBytes = data
	}

	if len(imgBytes) == 0 {
		writeErr(w, http.StatusBadRequest, "empty image data")
		return
	}

	targetPath := defaultLogoPath
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		targetPath = "logo.png"
	}
	if err := os.WriteFile(targetPath, imgBytes, 0644); err != nil {
		// Fallback to local
		targetPath = "logo.png"
		if err := os.WriteFile(targetPath, imgBytes, 0644); err != nil {
			writeErr(w, http.StatusInternalServerError, "save logo: "+err.Error())
			return
		}
	}

	logoURL := "http://127.0.0.1:2028/logo.png"
	if keyName == "" {
		keyName = "logo_livestream"
	}

	if s.redbida != nil && keyName != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, _ = s.redbida.Apply(ctx, map[string]any{keyName: logoURL}, true)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"url":     logoURL,
		"key":     keyName,
		"path":    targetPath,
		"bytes":   len(imgBytes),
		"savedAt": time.Now().Format(time.RFC3339),
	})
}
