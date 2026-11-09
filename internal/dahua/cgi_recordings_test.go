package dahua

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFindRecordingsCGIAndStreamDavCGI(t *testing.T) {
	var closedCount int32
	var destroyedCount int32
	var lastFindChannel string
	var lastLoadChannel string

	mux := http.NewServeMux()
	mux.HandleFunc("/cgi-bin/mediaFileFind.cgi", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch q.Get("action") {
		case "factory.create":
			fmt.Fprint(w, "result=987654\r\n")
		case "findFile":
			lastFindChannel = q.Get("condition.Channel")
			fmt.Fprint(w, "OK\r\n")
		case "findNextFile":
			fmt.Fprint(w, strings.Join([]string{
				"found=2",
				"items[0].Channel=13",
				"items[0].StartTime=2026-11-09 15:00:00",
				"items[0].EndTime=2026-11-09 16:00:00",
				"items[0].Length=275513344",
				"items[0].FilePath=/mnt/dvr/2026-11-09/013/dav/15.00.00-16.00.00.dav",
				"items[0].Type=dav",
				"items[1].Channel=13",
				"items[1].StartTime=2026-11-09 16:00:00",
				"items[1].EndTime=2026-11-09 17:00:00",
				"items[1].Length=274202624",
				"items[1].FilePath=/mnt/dvr/2026-11-09/013/dav/16.00.00-17.00.00.dav",
				"items[1].Type=dav",
			}, "\r\n"))
		case "close":
			atomic.AddInt32(&closedCount, 1)
			fmt.Fprint(w, "OK\r\n")
		case "destroy":
			atomic.AddInt32(&destroyedCount, 1)
			fmt.Fprint(w, "OK\r\n")
		}
	})
	var lastLoadRawQuery string
	mux.HandleFunc("/cgi-bin/loadfile.cgi", func(w http.ResponseWriter, r *http.Request) {
		lastLoadChannel = r.URL.Query().Get("channel")
		lastLoadRawQuery = r.URL.RawQuery
		payload := bytes.Repeat([]byte("DHII"), 512) // 2048 bytes
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, `Digest username="root"`) {
			w.Header().Set("WWW-Authenticate", `Digest realm="Login to NVR4X", nonce="12345678", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	defer ts.Close()

	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(ts.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)

	ctx := context.Background()
	start := time.Date(2026, 11, 9, 15, 0, 0, 0, time.Local)
	end := time.Date(2026, 11, 9, 17, 0, 0, 0, time.Local)

	// Pass 0-based channel=13 -> must query 1-based condition.Channel=14!
	recs, err := FindRecordingsCGI(ctx, host, port, "root", "1234abcd", 13, start, end)
	if err != nil {
		t.Fatalf("FindRecordingsCGI failed: %v", err)
	}
	if lastFindChannel != "14" {
		t.Fatalf("expected 1-based condition.Channel=14, got %q", lastFindChannel)
	}
	if len(recs) != 2 || recs[0].Duration != 3600 || recs[0].Channel != 13 {
		t.Fatalf("unexpected recs: %+v", recs)
	}
	if atomic.LoadInt32(&closedCount) != 1 || atomic.LoadInt32(&destroyedCount) != 1 {
		t.Fatalf("expected finder close+destroy=1, got %d/%d", closedCount, destroyedCount)
	}

	var buf bytes.Buffer
	if err := StreamDavCGI(ctx, &buf, host, port, "root", "1234abcd", 13, start, start.Add(5*time.Minute)); err != nil {
		t.Fatalf("StreamDavCGI failed: %v", err)
	}
	if lastLoadChannel != "14" {
		t.Fatalf("expected 1-based loadfile channel=14, got %q", lastLoadChannel)
	}
	if strings.Contains(lastLoadRawQuery, "+") || !strings.Contains(lastLoadRawQuery, "%20") {
		t.Fatalf("expected %%20 space encoding in loadfile RawQuery, got %q", lastLoadRawQuery)
	}
	if buf.Len() != 2048 {
		t.Fatalf("expected 2048 bytes, got %d", buf.Len())
	}
}
