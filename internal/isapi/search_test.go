package isapi

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSearchServer emulates just enough of an NVR's ContentMgmt surface to
// exercise SearchTrack/DownloadStream/DeviceLocation: Digest auth (reusing
// fakeISAPIServer's credential check, same as isapi_test.go) plus canned
// /ISAPI/ContentMgmt/{search,download} and /ISAPI/System/time handlers.
type fakeSearchServer struct {
	auth *fakeISAPIServer

	mu           sync.Mutex
	searchPages  []cmSearchResult // consumed in order, one per POST
	searchReqs   []cmSearchDescription
	downloadReqs []string // raw <playbackURI> bodies seen
	downloadBody []byte
	localTime    string
}

func newFakeSearchServer(user, pass string) *fakeSearchServer {
	return &fakeSearchServer{auth: newFakeISAPIServer(user, pass)}
}

func (s *fakeSearchServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.checkAuth(r.Header.Get("Authorization"), r.Method, r.URL.RequestURI()) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth"`, s.auth.realm, s.auth.nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/ISAPI/ContentMgmt/search" && r.Method == http.MethodPost:
			s.handleSearch(w, r)
		case r.URL.Path == "/ISAPI/ContentMgmt/download" && r.Method == http.MethodPost:
			s.handleDownload(w, r)
		case r.URL.Path == "/ISAPI/System/time" && r.Method == http.MethodGet:
			s.handleTime(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func (s *fakeSearchServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req cmSearchDescription
	if err := xml.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.searchReqs = append(s.searchReqs, req)
	var page cmSearchResult
	if len(s.searchPages) > 0 {
		page = s.searchPages[0]
		s.searchPages = s.searchPages[1:]
	} else {
		page = cmSearchResult{ResponseStatus: "NO MATCHES"}
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(mustXML(page))
}

func (s *fakeSearchServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	// Proper XML decode (not extractXMLString's raw byte-slice scan) so an
	// escaped "&" (mandatory for a URI embedded in an XML body) round-trips
	// back to the literal character, exactly as a real ISAPI-compliant NVR
	// would parse it.
	var req struct {
		PlaybackURI string `xml:"playbackURI"`
	}
	_ = xml.Unmarshal(body, &req)
	uri := req.PlaybackURI
	s.mu.Lock()
	s.downloadReqs = append(s.downloadReqs, uri)
	data := s.downloadBody
	s.mu.Unlock()

	w.Header().Set("Content-Type", "Opaque/data")
	_, _ = w.Write(data)
}

func (s *fakeSearchServer) handleTime(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	lt := s.localTime
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<Time><localTime>%s</localTime><timeZone>CST-7:00:00</timeZone></Time>`, lt)
}

func newSearchTestClient(t *testing.T, srv *httptest.Server, user, pass string) *Client {
	t.Helper()
	host, portStr, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("could not split host:port from %s", srv.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return New(host, port, false, user, pass, 5*time.Second)
}

func matchItem(trackID int, start, end string, size int64, uri string) cmSearchMatchItem {
	return cmSearchMatchItem{
		TrackID:  trackID,
		TimeSpan: cmTimeSpan{StartTime: start, EndTime: end},
		MediaSegmentDescriptor: cmMediaSegmentDescriptor{
			ContentType: "video",
			CodecType:   "H.264-BP",
			Size:        size,
			PlaybackURI: uri,
		},
	}
}

// TestSearchTrackPagination verifies SearchTrack pages through
// searchResultPostion until numOfMatches is fully collected, and that each
// segment's fields (times, codec, size, URI) round-trip correctly.
func TestSearchTrackPagination(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	fake.searchPages = []cmSearchResult{
		{
			ResponseStatus: "MORE",
			NumOfMatches:   5,
			MatchList: cmMatchList{SearchMatchItem: []cmSearchMatchItem{
				matchItem(101, "2026-07-19T00:00:00Z", "2026-07-19T01:00:00Z", 1062807408, "rtsp://192.168.1.215/Streaming/tracks/101/?starttime=20260719T000000Z&endtime=20260719T010000Z&name=1&size=1062807408"),
				matchItem(101, "2026-07-19T01:00:00Z", "2026-07-19T02:00:00Z", 2000, "rtsp://x/2"),
				matchItem(101, "2026-07-19T02:00:00Z", "2026-07-19T03:00:00Z", 3000, "rtsp://x/3"),
			}},
		},
		{
			ResponseStatus: "OK",
			NumOfMatches:   5,
			MatchList: cmMatchList{SearchMatchItem: []cmSearchMatchItem{
				matchItem(101, "2026-07-19T03:00:00Z", "2026-07-19T04:00:00Z", 4000, "rtsp://x/4"),
				matchItem(101, "2026-07-19T04:00:00Z", "2026-07-19T05:00:00Z", 5000, "rtsp://x/5"),
			}},
		},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	start, _ := time.Parse(hikTimeLayout, "2026-07-19T00:00:00Z")
	end, _ := time.Parse(hikTimeLayout, "2026-07-20T23:59:59Z")
	segs, err := c.SearchTrack(context.Background(), 101, start, end, 3)
	if err != nil {
		t.Fatalf("SearchTrack: %v", err)
	}
	if len(segs) != 5 {
		t.Fatalf("got %d segments, want 5: %+v", len(segs), segs)
	}
	if segs[0].Size != 1062807408 {
		t.Fatalf("segs[0].Size = %d, want 1062807408 (from XML <size>)", segs[0].Size)
	}
	if !segs[0].Start.Equal(time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("segs[0].Start = %v", segs[0].Start)
	}
	if segs[0].CodecType != "H.264-BP" {
		t.Fatalf("segs[0].CodecType = %q", segs[0].CodecType)
	}

	fake.mu.Lock()
	reqs := fake.searchReqs
	fake.mu.Unlock()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(reqs))
	}
	if reqs[0].SearchResultPostion != 0 {
		t.Fatalf("first request searchResultPostion = %d, want 0", reqs[0].SearchResultPostion)
	}
	if reqs[1].SearchResultPostion != 3 {
		t.Fatalf("second request searchResultPostion = %d, want 3 (page-1 count)", reqs[1].SearchResultPostion)
	}
	// Both pages must reuse the SAME searchID (correlates pages of one search).
	if reqs[0].SearchID == "" || reqs[0].SearchID != reqs[1].SearchID {
		t.Fatalf("searchID not stable across pages: %q vs %q", reqs[0].SearchID, reqs[1].SearchID)
	}
	// The exact required shape (contentTypeList/metadataList), per the
	// live-verified working body — a request missing these is rejected by a
	// real NVR with "Invalid XML Content".
	if reqs[0].ContentTypeList.ContentType != "video" {
		t.Fatalf("contentTypeList = %+v, want ContentType=video", reqs[0].ContentTypeList)
	}
	if reqs[0].MetadataList.MetadataDescriptor != "//recordType.meta.std-cgi.com" {
		t.Fatalf("metadataList = %+v", reqs[0].MetadataList)
	}
	if reqs[0].TrackList.TrackID != 101 {
		t.Fatalf("trackList.trackID = %d, want 101", reqs[0].TrackList.TrackID)
	}
}

// TestSearchTrackNoMatches verifies a "NO MATCHES" response (a range with no
// recordings, not an error condition) yields an empty, error-free result.
func TestSearchTrackNoMatches(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	fake.searchPages = []cmSearchResult{{ResponseStatus: "NO MATCHES", NumOfMatches: 0}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	segs, err := c.SearchTrack(context.Background(), 101, time.Now(), time.Now(), 40)
	if err != nil {
		t.Fatalf("SearchTrack: %v", err)
	}
	if len(segs) != 0 {
		t.Fatalf("got %d segments, want 0", len(segs))
	}
}

// TestSearchTrackXMLShapeAndTimes checks the wire body: XML declaration, and
// the request's <startTime>/<endTime> carrying the SAME wall-clock digits as
// the input time — no UTC conversion (the device treats the "Z" suffix as
// decorative and reads the value as its own local clock; see hikTimeLayout).
func TestSearchTrackXMLShapeAndTimes(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	fake.searchPages = []cmSearchResult{{ResponseStatus: "NO MATCHES"}}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	// Device-local wall-clock input (tagged with an arbitrary +07:00 zone, as
	// a caller might hand in a zone-aware time.Time). The request body must
	// carry these SAME wall-clock digits verbatim, not shifted to UTC.
	loc := time.FixedZone("device", 7*3600)
	start := time.Date(2026, 7, 19, 0, 0, 0, 0, loc)
	end := time.Date(2026, 7, 19, 23, 59, 59, 0, loc)
	if _, err := c.SearchTrack(context.Background(), 101, start, end, 40); err != nil {
		t.Fatalf("SearchTrack: %v", err)
	}

	fake.mu.Lock()
	req := fake.searchReqs[0]
	fake.mu.Unlock()
	if req.TimeSpanList.TimeSpan.StartTime != "2026-07-19T00:00:00Z" {
		t.Fatalf("startTime = %q, want verbatim wall-clock 2026-07-19T00:00:00Z (no offset applied)", req.TimeSpanList.TimeSpan.StartTime)
	}
	if req.TimeSpanList.TimeSpan.EndTime != "2026-07-19T23:59:59Z" {
		t.Fatalf("endTime = %q, want verbatim wall-clock 2026-07-19T23:59:59Z (no offset applied)", req.TimeSpanList.TimeSpan.EndTime)
	}

	body, err := xml.Marshal(cmSearchDescription{
		SearchID:  "x",
		TrackList: cmTrackList{TrackID: 101},
		TimeSpanList: cmTimeSpanList{TimeSpan: cmTimeSpan{
			StartTime: "2026-07-19T00:00:00Z",
			EndTime:   "2026-07-20T23:59:59Z",
		}},
		ContentTypeList:     cmContentTypeList{ContentType: "video"},
		MaxResults:          40,
		SearchResultPostion: 0,
		MetadataList:        cmMetadataList{MetadataDescriptor: "//recordType.meta.std-cgi.com"},
	})
	if err != nil {
		t.Fatalf("marshal reference body: %v", err)
	}
	full := string(append([]byte(xml.Header), body...))
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<CMSearchDescription>`,
		`<trackList><trackID>101</trackID></trackList>`,
		`<contentTypeList><contentType>video</contentType></contentTypeList>`,
		`<metadataList><metadataDescriptor>//recordType.meta.std-cgi.com</metadataDescriptor></metadataList>`,
	} {
		if !strings.Contains(full, want) {
			t.Errorf("marshaled CMSearchDescription missing %q: %s", want, full)
		}
	}
}

// TestSearchTrackNoOffsetAppliedRegardlessOfInputZone is the regression test
// for the timezone-conversion bug: the SAME wall-clock start (10:00:00),
// tagged with two different zones (UTC and +07:00), must produce the
// IDENTICAL request <startTime> — proof no offset-driven shift happens on
// the way out. Before the fix, SearchTrack called start.UTC() before
// formatting, which would have shifted the +07:00 case to 03:00:00.
func TestSearchTrackNoOffsetAppliedRegardlessOfInputZone(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	fake.searchPages = []cmSearchResult{
		{ResponseStatus: "NO MATCHES"},
		{ResponseStatus: "NO MATCHES"},
	}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	utcStart := time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	plus7 := time.FixedZone("device", 7*3600)
	plus7Start := time.Date(2026, 7, 19, 10, 0, 0, 0, plus7)

	if _, err := c.SearchTrack(context.Background(), 101, utcStart, utcStart, 40); err != nil {
		t.Fatalf("SearchTrack (UTC): %v", err)
	}
	if _, err := c.SearchTrack(context.Background(), 101, plus7Start, plus7Start, 40); err != nil {
		t.Fatalf("SearchTrack (+07:00): %v", err)
	}

	fake.mu.Lock()
	reqs := fake.searchReqs
	fake.mu.Unlock()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 search requests, got %d", len(reqs))
	}
	if reqs[0].TimeSpanList.TimeSpan.StartTime != "2026-07-19T10:00:00Z" {
		t.Fatalf("UTC-tagged startTime = %q, want 2026-07-19T10:00:00Z", reqs[0].TimeSpanList.TimeSpan.StartTime)
	}
	if reqs[1].TimeSpanList.TimeSpan.StartTime != reqs[0].TimeSpanList.TimeSpan.StartTime {
		t.Fatalf("startTime depends on the input's zone (UTC %q vs +07:00 %q) — must be identical wall-clock, no offset applied",
			reqs[0].TimeSpanList.TimeSpan.StartTime, reqs[1].TimeSpanList.TimeSpan.StartTime)
	}
}

// TestDownloadStreamSendsExactURIAndStreamsBody verifies DownloadStream sends
// the playbackURI VERBATIM (including its name/size query params — trimming
// them gets a real NVR's HTTP 400) and streams the response body to w.
func TestDownloadStreamSendsExactURIAndStreamsBody(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	payload := append([]byte("IMKH"), bytes.Repeat([]byte{0xAB}, 1<<16)...) // > the old 1 MiB cap's neighborhood in spirit, cheap here
	fake.downloadBody = payload
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	const uri = "rtsp://192.168.1.215/Streaming/tracks/101/?starttime=20260719T020941Z&endtime=20260719T030000Z&name=00000001195000000&size=1062807408"
	var buf bytes.Buffer
	n, err := c.DownloadStream(context.Background(), &buf, uri)
	if err != nil {
		t.Fatalf("DownloadStream: %v", err)
	}
	if n != int64(len(payload)) {
		t.Fatalf("n = %d, want %d", n, len(payload))
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatalf("streamed body mismatch (got %d bytes)", buf.Len())
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("IMKH")) {
		t.Fatalf("expected IMKH magic, got %x", buf.Bytes()[:4])
	}

	fake.mu.Lock()
	got := fake.downloadReqs
	fake.mu.Unlock()
	if len(got) != 1 || got[0] != uri {
		t.Fatalf("download request playbackURI = %q, want %q (verbatim, incl. name/size)", got, uri)
	}
}

// TestDeviceLocationParsesLocalTimeOffsetIgnoringTimeZoneTag confirms
// DeviceLocation uses <localTime>'s own RFC3339 offset and NOT <timeZone>
// (whose POSIX-notation sign is inverted from what it looks like — a
// confirmed live trap: CST-7:00:00 actually means UTC+7).
func TestDeviceLocationParsesLocalTimeOffsetIgnoringTimeZoneTag(t *testing.T) {
	fake := newFakeSearchServer("admin", "duyanh68A")
	fake.localTime = "2026-07-20T12:38:51+07:00"
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newSearchTestClient(t, srv, "admin", "duyanh68A")

	loc, err := c.DeviceLocation(context.Background())
	if err != nil {
		t.Fatalf("DeviceLocation: %v", err)
	}
	_, offset := time.Now().In(loc).Zone()
	if offset != 7*3600 {
		t.Fatalf("offset = %d, want %d (7h, from localTime — NOT the inverted timeZone tag)", offset, 7*3600)
	}
}
