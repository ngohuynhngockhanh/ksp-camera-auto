package server

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/discovery"
)

// scanTimeout bounds how long a single /api/scan request may take: UDP
// discovery methods run concurrently and the nmap fallback is a bounded TCP
// connect scan, so a modest ceiling keeps the request from hanging the UI.
const scanTimeout = 6 * time.Second

// scanReq is the body of POST /api/scan.
type scanReq struct {
	Method string `json:"method"` // "all" (default), "onvif", "dahua", "hikvision", "nmap"
	Subnet string `json:"subnet"` // required when method == "nmap", e.g. "192.168.1.0/24"
}

// handleScan handles POST /api/scan: run LAN camera discovery and return the
// candidates found so the UI can offer a one-click "add to inventory".
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req scanReq
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
	}
	if req.Method == "" {
		req.Method = "all"
	}

	ctx, cancel := context.WithTimeout(r.Context(), scanTimeout)
	defer cancel()

	var (
		results []discovery.Result
		err     error
	)
	switch req.Method {
	case "all":
		results, err = discovery.Scan(ctx, scanTimeout)
	case "onvif", "dahua", "hikvision":
		results, err = scanAndFilter(ctx, req.Method)
	case "nmap":
		if req.Subnet == "" {
			writeErr(w, http.StatusBadRequest, "subnet is required for nmap scan")
			return
		}
		results, err = discovery.ScanSubnet(ctx, req.Subnet)
	default:
		writeErr(w, http.StatusBadRequest, "method must be one of: all, onvif, dahua, hikvision, nmap")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if results == nil {
		results = []discovery.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

// tryPasswordReq is the body of POST /api/scan/try-password.
type tryPasswordReq struct {
	Targets        []bulk.CredTestTarget `json:"targets"`
	Username       string                `json:"username"`
	Password       string                `json:"password"`
	TimeoutSeconds int                   `json:"timeoutSeconds"`
}

// handleTryPassword handles POST /api/scan/try-password: sequentially try
// one username/password against a caller-supplied list of scanned (not yet
// in inventory) devices, streaming a "result" event per device as it
// completes (SSE, same wire format as /api/apply) so the UI can highlight
// each row live rather than waiting for the whole batch.
func (s *Server) handleTryPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req tryPasswordReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Targets) == 0 {
		writeErr(w, http.StatusBadRequest, "targets is required")
		return
	}
	if req.Username == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}

	// Sequential run can take a while for a large selection; scale the
	// overall deadline by target count, same reasoning as handleApply.
	to := s.reqTimeout(req.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(r.Context(), to*time.Duration(len(req.Targets)+1))
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)

	emit := func(ev bulk.CredTestEvent) {
		if r.Context().Err() != nil {
			return
		}
		b, err := json.Marshal(ev)
		if err != nil {
			log.Printf("encode try-password event: %v", err)
			return
		}
		if _, err := w.Write([]byte("data: " + string(b) + "\n\n")); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	bulk.TryPasswords(ctx, req.Targets, req.Username, req.Password, s.cfg.Defaults, to, emit)
}

// viaForMethod maps a request "method" value to the discovery.Result.Via tag
// it corresponds to.
func viaForMethod(method string) string {
	if method == "hikvision" {
		return "hikvision-sadp"
	}
	return method
}

// scanAndFilter runs the full multi-method Scan and keeps only the results
// that came from the requested method, so a single-protocol request still
// benefits from Scan's concurrent fan-out instead of needing a bespoke
// single-method entry point.
func scanAndFilter(ctx context.Context, method string) ([]discovery.Result, error) {
	all, err := discovery.Scan(ctx, scanTimeout)
	if err != nil {
		return nil, err
	}
	via := viaForMethod(method)
	out := make([]discovery.Result, 0, len(all))
	for _, r := range all {
		if r.Via == via {
			out = append(out, r)
		}
	}
	return out, nil
}
