package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

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
