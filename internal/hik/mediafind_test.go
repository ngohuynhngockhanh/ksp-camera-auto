package hik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// fakeHikServer emulates just enough of an NVR to exercise FindRecordings
// against a real HTTP+Digest round trip (no live device/credential gating):
// POST /ISAPI/ContentMgmt/search. Auth is checked against isapi's own
// exported Challenge/BuildAuthorization — the same credential math the real
// Client computes — so this doubles as an end-to-end proof the request
// actually authenticates. No /ISAPI/System/time handler: FindRecordings no
// longer does a DeviceLocation lookup (times are passed through verbatim,
// device-local wall-clock in and out — see mediafind.go/isapi.hikTimeLayout).
type fakeHikServer struct {
	realm, nonce, user, pass string

	mu         sync.Mutex
	searchReqs []string
}

func newFakeHikServer(user, pass string) *fakeHikServer {
	return &fakeHikServer{realm: "IP Camera", nonce: "hiktestnonce123", user: user, pass: pass}
}

func (s *fakeHikServer) checkAuth(header, method, uri string) bool {
	if header == "" || !strings.HasPrefix(header, "Digest ") {
		return false
	}
	params := map[string]string{}
	for _, part := range strings.Split(strings.TrimPrefix(header, "Digest "), ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		params[kv[0]] = strings.Trim(kv[1], `"`)
	}
	if params["username"] != s.user || params["nonce"] != s.nonce {
		return false
	}
	chal := isapi.Challenge{Realm: s.realm, Nonce: s.nonce}
	want := isapi.BuildAuthorization(chal, method, uri, s.user, s.pass, params["cnonce"], params["nc"])
	return extractField(want, "response") == params["response"]
}

func extractField(header, key string) string {
	idx := strings.Index(header, key+`="`)
	if idx < 0 {
		return ""
	}
	rest := header[idx+len(key)+2:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func (s *fakeHikServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.checkAuth(r.Header.Get("Authorization"), r.Method, r.URL.RequestURI()) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth"`, s.realm, s.nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/ISAPI/ContentMgmt/search" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.searchReqs = append(s.searchReqs, string(body))
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0"?><CMSearchResult>`+
				`<responseStatus>OK</responseStatus><numOfMatches>1</numOfMatches>`+
				`<matchList><searchMatchItem><trackID>101</trackID>`+
				`<timeSpan><startTime>2026-07-19T01:00:00Z</startTime><endTime>2026-07-19T02:00:00Z</endTime></timeSpan>`+
				`<mediaSegmentDescriptor><contentType>video</contentType><codecType>H.264-BP</codecType>`+
				`<size>123456</size>`+
				`<playbackURI>rtsp://192.168.1.215/Streaming/tracks/101/?starttime=20260719T010000Z&amp;endtime=20260719T020000Z&amp;name=1&amp;size=123456</playbackURI>`+
				`</mediaSegmentDescriptor></searchMatchItem></matchList></CMSearchResult>`)
		default:
			http.NotFound(w, r)
		}
	}
}

func newFakeHikClient(t *testing.T, srv *httptest.Server, user, pass string) *Client {
	t.Helper()
	host, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("could not split host:port from %s", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return Dial(host, port, false, user, pass, 5*time.Second)
}

// TestFindRecordingsPassesTimesVerbatim exercises FindRecordings end to end:
// the device-local wall-clock input must reach the search request body
// UNCHANGED, and the device-local wall-clock times in the response must
// reach dahua.Recording.StartTime/EndTime UNCHANGED — no UTC/offset
// conversion in either direction (this NVR treats ISAPI search's "Z"-suffixed
// times as device-local regardless of the zone designator; see
// isapi.hikTimeLayout). This is the regression test for the bug where an
// earlier version converted local<->UTC via a DeviceLocation lookup, shifting
// every listing and playback request by the device's whole UTC offset (7h
// for this project's live UTC+7 NVR).
func TestFindRecordingsPassesTimesVerbatim(t *testing.T) {
	fake := newFakeHikServer("admin", "duyanh68A")
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newFakeHikClient(t, srv, "admin", "duyanh68A")

	// Device-local wall-clock times, exactly as the web API's
	// parsePlaybackTime hands them (time.Parse with no zone -> UTC-tagged,
	// but the HOUR/MINUTE/SECOND fields are what actually matter here).
	start := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 19, 2, 0, 0, 0, time.UTC)

	recs, err := c.FindRecordings(context.Background(), 1, start, end)
	if err != nil {
		t.Fatalf("FindRecordings: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d recordings, want 1: %+v", len(recs), recs)
	}
	r := recs[0]
	if r.Channel != 1 {
		t.Fatalf("Channel = %d, want 1", r.Channel)
	}
	// The fake's segment is "2026-07-19T01:00:00Z" - "2026-07-19T02:00:00Z"
	// (device-local, decorative "Z" per hikTimeLayout) -> must come back as
	// that SAME wall-clock, not shifted by any offset.
	if r.StartTime != "2026-07-19 01:00:00" {
		t.Fatalf("StartTime = %q, want verbatim device-local 2026-07-19 01:00:00 (no offset applied)", r.StartTime)
	}
	if r.EndTime != "2026-07-19 02:00:00" {
		t.Fatalf("EndTime = %q, want verbatim device-local 2026-07-19 02:00:00 (no offset applied)", r.EndTime)
	}
	if r.Duration != 3600 {
		t.Fatalf("Duration = %d, want 3600", r.Duration)
	}
	if r.Type != "mp4" {
		t.Fatalf("Type = %q, want mp4", r.Type)
	}
	if r.Length != 123456 {
		t.Fatalf("Length = %d, want 123456", r.Length)
	}

	fake.mu.Lock()
	reqs := fake.searchReqs
	fake.mu.Unlock()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 search request, got %d", len(reqs))
	}
	if !strings.Contains(reqs[0], "<trackID>101</trackID>") {
		t.Fatalf("search request missing trackID=101 (channel 1 * 100 + 1): %s", reqs[0])
	}
	// The device-local 01:00-02:00 input must be sent VERBATIM, not shifted
	// by any UTC offset.
	if !strings.Contains(reqs[0], "<startTime>2026-07-19T01:00:00Z</startTime>") {
		t.Fatalf("search request startTime was not sent verbatim (no offset applied): %s", reqs[0])
	}
	if !strings.Contains(reqs[0], "<endTime>2026-07-19T02:00:00Z</endTime>") {
		t.Fatalf("search request endTime was not sent verbatim (no offset applied): %s", reqs[0])
	}
}
