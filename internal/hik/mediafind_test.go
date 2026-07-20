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
// GET /ISAPI/System/time and POST /ISAPI/ContentMgmt/search. Auth is checked
// against isapi's own exported Challenge/BuildAuthorization — the same
// credential math the real Client computes — so this doubles as an
// end-to-end proof the request actually authenticates.
type fakeHikServer struct {
	realm, nonce, user, pass string
	localTime                string

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
		case r.URL.Path == "/ISAPI/System/time" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<Time><localTime>%s</localTime><timeZone>CST-7:00:00</timeZone></Time>`, s.localTime)
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

// TestFindRecordingsConvertsLocalToUTCAndBack exercises the full
// device-local -> UTC (request) -> device-local (response) round trip
// FindRecordings does via DeviceLocation, against a device whose <localTime>
// reports UTC+7 (this project's live Hik NVR is UTC+7, per memory).
func TestFindRecordingsConvertsLocalToUTCAndBack(t *testing.T) {
	fake := newFakeHikServer("admin", "duyanh68A")
	fake.localTime = "2026-07-19T12:00:00+07:00"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newFakeHikClient(t, srv, "admin", "duyanh68A")

	// Zone-less "device-local wall clock" times, exactly as the web API's
	// parsePlaybackTime hands them (time.Parse with no zone -> UTC-tagged,
	// but the HOUR/MINUTE/SECOND fields are what actually matter here).
	start := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)

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
	// The fake's segment is UTC 01:00-02:00 -> device-local (+07:00) 08:00-09:00.
	if r.StartTime != "2026-07-19 08:00:00" {
		t.Fatalf("StartTime = %q, want device-local 2026-07-19 08:00:00", r.StartTime)
	}
	if r.EndTime != "2026-07-19 09:00:00" {
		t.Fatalf("EndTime = %q, want device-local 2026-07-19 09:00:00", r.EndTime)
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
	// The device-local 08:00-09:00 input must be sent as UTC 01:00-02:00.
	if !strings.Contains(reqs[0], "<startTime>2026-07-19T01:00:00Z</startTime>") {
		t.Fatalf("search request startTime not converted device-local -> UTC: %s", reqs[0])
	}
	if !strings.Contains(reqs[0], "<endTime>2026-07-19T02:00:00Z</endTime>") {
		t.Fatalf("search request endTime not converted device-local -> UTC: %s", reqs[0])
	}
}
