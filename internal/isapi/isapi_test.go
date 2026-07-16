package isapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeISAPIServer emulates just enough of a Hikvision device to exercise the
// Client: Digest auth on every request, and a StreamingChannel resource per
// compound channel id that GET returns and PUT replaces (checked for
// statusCode==1 semantics).
type fakeISAPIServer struct {
	mu       sync.Mutex
	realm    string
	nonce    string
	user     string
	pass     string
	channels map[int]*StreamingChannel

	// requestsSeen counts authenticated (non-401) requests, so tests can
	// assert the digest handshake actually happened rather than every
	// request silently 401ing.
	requestsSeen int
}

func newFakeISAPIServer(user, pass string) *fakeISAPIServer {
	return &fakeISAPIServer{
		realm:    "IP Camera",
		nonce:    "testnonce1234567890",
		user:     user,
		pass:     pass,
		channels: map[int]*StreamingChannel{},
	}
}

func (s *fakeISAPIServer) seedChannel(id int, sc *StreamingChannel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[id] = sc
}

func (s *fakeISAPIServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !s.checkAuth(auth, r.Method, r.URL.RequestURI()) {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Digest realm="%s", nonce="%s", qop="auth"`, s.realm, s.nonce))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		s.mu.Lock()
		s.requestsSeen++
		s.mu.Unlock()

		var id int
		var isSmartCodec bool
		path := strings.TrimPrefix(r.URL.Path, "/ISAPI/Streaming/channels/")
		if strings.HasSuffix(path, "/smartCodec") {
			isSmartCodec = true
			path = strings.TrimSuffix(path, "/smartCodec")
		}
		if _, err := fmt.Sscanf(path, "%d", &id); err != nil {
			http.NotFound(w, r)
			return
		}

		switch {
		case isSmartCodec && r.Method == http.MethodGet:
			s.mu.Lock()
			sc := s.channels[id]
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/xml")
			on := sc != nil && sc.Video != nil && sc.Video.SmartCodec != nil && sc.Video.SmartCodec.Enabled
			_, _ = w.Write(mustXML(SmartCodec{Enabled: on}))
		case isSmartCodec && r.Method == http.MethodPut:
			var body SmartCodec
			if err := xml.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			s.mu.Lock()
			sc := s.channels[id]
			if sc != nil {
				if sc.Video == nil {
					sc.Video = &Video{}
				}
				sc.Video.SmartCodec = &SmartCodec{Enabled: body.Enabled}
			}
			s.mu.Unlock()
			writeOK(w)
		case r.Method == http.MethodGet:
			s.mu.Lock()
			sc, ok := s.channels[id]
			s.mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(mustXML(sc))
		case r.Method == http.MethodPut:
			var body StreamingChannel
			if err := xml.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			s.mu.Lock()
			s.channels[id] = &body
			s.mu.Unlock()
			writeOK(w)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

// checkAuth validates a Digest Authorization header against this server's
// single known user/pass. It recomputes the expected response exactly as
// BuildAuthorization does, which doubles as an end-to-end proof that the
// Client's real header matches what an RFC 2617 server expects.
func (s *fakeISAPIServer) checkAuth(header, method, uri string) bool {
	if header == "" {
		return false
	}
	if !strings.HasPrefix(header, "Digest ") {
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
	chal := Challenge{Realm: s.realm, Nonce: s.nonce}
	want := BuildAuthorization(chal, method, uri, s.user, s.pass, params["cnonce"], params["nc"])
	// BuildAuthorization embeds a response= field computed the same way the
	// client did; compare just that field so we're independently validating
	// the credential math rather than string-matching the whole header.
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

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(mustXML(responseStatus{StatusCode: 1, StatusString: "OK", SubStatusCode: "ok"}))
}

func mustXML(v any) []byte {
	b, err := xml.Marshal(v)
	if err != nil {
		panic(err)
	}
	return append([]byte(xml.Header), b...)
}

func newTestClient(t *testing.T, srv *httptest.Server, user, pass string) *Client {
	t.Helper()
	u := strings.TrimPrefix(srv.URL, "http://")
	host, portStr, ok := strings.Cut(u, ":")
	if !ok {
		t.Fatalf("could not split host:port from %s", srv.URL)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return New(host, port, false, user, pass, 5*time.Second)
}

func TestGetStreamChannelRoundTrip(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{
		ID:      "101",
		Enabled: true,
		Video: &Video{
			Enabled:               true,
			VideoCodecType:        CodecH264,
			VideoResolutionWidth:  1920,
			VideoResolutionHeight: 1080,
			MaxFrameRate:          2500,
		},
		Audio: &Audio{Enabled: false, AudioCompressionType: "G.711ulaw"},
	})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	sc, err := c.GetStreamChannel(context.Background(), 101)
	if err != nil {
		t.Fatalf("GetStreamChannel: %v", err)
	}
	if sc.Video == nil || sc.Video.VideoCodecType != CodecH264 {
		t.Fatalf("unexpected video: %+v", sc.Video)
	}
	if sc.Video.VideoResolutionWidth != 1920 || sc.Video.VideoResolutionHeight != 1080 {
		t.Fatalf("unexpected resolution: %dx%d", sc.Video.VideoResolutionWidth, sc.Video.VideoResolutionHeight)
	}
	if sc.Video.MaxFrameRate != 2500 {
		t.Fatalf("maxFrameRate = %d, want 2500", sc.Video.MaxFrameRate)
	}

	fake.mu.Lock()
	seen := fake.requestsSeen
	fake.mu.Unlock()
	if seen == 0 {
		t.Fatalf("expected at least one authenticated request to reach the handler")
	}
}

func TestGetStreamChannelWrongCredsFails(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{ID: "101", Video: &Video{}})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "wrongpassword")

	if _, err := c.GetStreamChannel(context.Background(), 101); err == nil {
		t.Fatalf("expected auth failure with wrong password")
	}
}

func TestSetResolution(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	// Real devices always return these fields; raw-XML mutation replaces the
	// existing tags rather than inventing them.
	fake.seedChannel(101, &StreamingChannel{ID: "101", Video: &Video{VideoCodecType: CodecH264, VideoResolutionWidth: 1920, VideoResolutionHeight: 1080, MaxFrameRate: 3000}})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetResolution(context.Background(), 1, 0, 1280, 720, 25); err != nil {
		t.Fatalf("SetResolution: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	if sc.Video.VideoResolutionWidth != 1280 || sc.Video.VideoResolutionHeight != 720 {
		t.Fatalf("resolution not written: %+v", sc.Video)
	}
	if sc.Video.MaxFrameRate != 2500 {
		t.Fatalf("maxFrameRate = %d, want 2500 (25fps*100)", sc.Video.MaxFrameRate)
	}
	// The codec set before this call must survive the GET-modify-PUT cycle.
	if sc.Video.VideoCodecType != CodecH264 {
		t.Fatalf("codec clobbered by SetResolution: %q", sc.Video.VideoCodecType)
	}
}

func TestSetCodec(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{ID: "101", Video: &Video{VideoCodecType: CodecH264, VideoResolutionWidth: 1920, VideoResolutionHeight: 1080}})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetCodec(context.Background(), 1, 0, CodecH265); err != nil {
		t.Fatalf("SetCodec: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	if sc.Video.VideoCodecType != CodecH265 {
		t.Fatalf("codec = %q, want H.265", sc.Video.VideoCodecType)
	}
	if sc.Video.VideoResolutionWidth != 1920 {
		t.Fatalf("resolution clobbered by SetCodec: %+v", sc.Video)
	}
}

func TestSetAudioAAC(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{ID: "101", Video: &Video{}, Audio: &Audio{Enabled: false, AudioCompressionType: "G.711ulaw"}})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetAudioAAC(context.Background(), 1, 0); err != nil {
		t.Fatalf("SetAudioAAC: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	// SetAudioAAC changes only the audio codec (audioCompressionType); it does
	// not force the enabled flag, matching the raw-mutation behaviour.
	if sc.Audio == nil || sc.Audio.AudioCompressionType != "AAC" {
		t.Fatalf("audio not set to AAC: %+v", sc.Audio)
	}
}

func TestSetSmartCodecEnablesH265First(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{ID: "101", Video: &Video{VideoCodecType: CodecH264}})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetSmartCodec(context.Background(), 1, 0, true); err != nil {
		t.Fatalf("SetSmartCodec: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	if sc.Video.VideoCodecType != CodecH265 {
		t.Fatalf("expected base codec switched to H.265, got %q", sc.Video.VideoCodecType)
	}
	if sc.Video.SmartCodec == nil || !sc.Video.SmartCodec.Enabled {
		t.Fatalf("smart codec not enabled: %+v", sc.Video.SmartCodec)
	}

	info, err := c.GetStreamInfo(context.Background(), 1, 0)
	if err != nil {
		t.Fatalf("GetStreamInfo: %v", err)
	}
	if !info.SmartCodec {
		t.Fatalf("GetStreamInfo did not report smart codec enabled: %+v", info)
	}
}

func TestGetStreamInfo(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(102, &StreamingChannel{
		ID: "102",
		Video: &Video{
			VideoCodecType:          CodecH264,
			VideoResolutionWidth:    640,
			VideoResolutionHeight:   480,
			MaxFrameRate:            1500,
			VideoQualityControlType: "VBR",
			GovLength:               40,
			VBRUpperCap:             2048,
		},
		Audio: &Audio{Enabled: true, AudioCompressionType: "AAC"},
	})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	info, err := c.GetStreamInfo(context.Background(), 1, 1) // channel 1, sub1 -> id 102
	if err != nil {
		t.Fatalf("GetStreamInfo: %v", err)
	}
	if info.Width != 640 || info.Height != 480 {
		t.Fatalf("unexpected resolution: %+v", info)
	}
	if info.FPS != 15 {
		t.Fatalf("fps = %d, want 15 (1500/100)", info.FPS)
	}
	if info.Codec != CodecH264 {
		t.Fatalf("codec = %q", info.Codec)
	}
	if !info.AudioEnable || info.AudioCodec != "AAC" {
		t.Fatalf("unexpected audio: %+v", info)
	}
	if info.GOP != 40 {
		t.Fatalf("GOP = %d, want 40", info.GOP)
	}
	if info.BitrateMode != "VBR" {
		t.Fatalf("BitrateMode = %q, want VBR", info.BitrateMode)
	}
	if info.BitrateKbps != 2048 {
		t.Fatalf("BitrateKbps = %d, want 2048", info.BitrateKbps)
	}
}

func TestSetGOP(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{
		ID: "101",
		Video: &Video{
			VideoCodecType:        CodecH264,
			VideoResolutionWidth:  1920,
			VideoResolutionHeight: 1080,
			GovLength:             25,
		},
	})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetGOP(context.Background(), 1, 0, 50); err != nil {
		t.Fatalf("SetGOP: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	if sc.Video.GovLength != 50 {
		t.Fatalf("GovLength = %d, want 50", sc.Video.GovLength)
	}
	// The codec set before this call must survive the GET-modify-PUT cycle.
	if sc.Video.VideoCodecType != CodecH264 {
		t.Fatalf("codec clobbered by SetGOP: %q", sc.Video.VideoCodecType)
	}
}

func TestSetBitrate(t *testing.T) {
	fake := newFakeISAPIServer("admin", "hunter2")
	fake.seedChannel(101, &StreamingChannel{
		ID: "101",
		Video: &Video{
			VideoCodecType:          CodecH264,
			VideoQualityControlType: "VBR",
			VBRUpperCap:             1024,
		},
	})
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newTestClient(t, srv, "admin", "hunter2")

	if err := c.SetBitrate(context.Background(), 1, 0, 4096, ""); err != nil {
		t.Fatalf("SetBitrate: %v", err)
	}

	fake.mu.Lock()
	sc := fake.channels[101]
	fake.mu.Unlock()
	if sc.Video.VBRUpperCap != 4096 {
		t.Fatalf("VBRUpperCap = %d, want 4096", sc.Video.VBRUpperCap)
	}
	if sc.Video.VideoCodecType != CodecH264 {
		t.Fatalf("codec clobbered by SetBitrate: %q", sc.Video.VideoCodecType)
	}
}

func TestGopEditsPrefersGovLength(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><GovLength>40</GovLength></Video></StreamingChannel>`)
	edits, err := gopEdits(doc, 50, false)
	if err != nil {
		t.Fatalf("gopEdits: %v", err)
	}
	if edits["GovLength"] != "50" {
		t.Fatalf("edits = %+v, want GovLength=50", edits)
	}
	if _, ok := edits["keyFrameInterval"]; ok {
		t.Fatalf("edits = %+v, should not touch keyFrameInterval when GovLength present", edits)
	}
}

func TestGopEditsKeyFrameIntervalMs(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><keyFrameInterval>2000</keyFrameInterval><maxFrameRate>2500</maxFrameRate></Video></StreamingChannel>`)
	edits, err := gopEdits(doc, 50, true)
	if err != nil {
		t.Fatalf("gopEdits: %v", err)
	}
	// 50 frames @ 25fps (maxFrameRate=2500 -> fps=25) = 2000ms.
	if edits["keyFrameInterval"] != "2000" {
		t.Fatalf("edits = %+v, want keyFrameInterval=2000", edits)
	}
}

func TestGopEditsMissingTagFails(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video></Video></StreamingChannel>`)
	if _, err := gopEdits(doc, 50, false); err == nil {
		t.Fatal("expected error when neither GovLength nor keyFrameInterval is present")
	}
}

func TestBitrateEditsCBR(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><videoQualityControlType>VBR</videoQualityControlType><constantBitRate>1024</constantBitRate></Video></StreamingChannel>`)
	edits, err := bitrateEdits(doc, false, 2048, "CBR")
	if err != nil {
		t.Fatalf("bitrateEdits: %v", err)
	}
	if edits["constantBitRate"] != "2048" {
		t.Fatalf("edits = %+v, want constantBitRate=2048", edits)
	}
	if edits["videoQualityControlType"] != "CBR" {
		t.Fatalf("edits = %+v, want videoQualityControlType=CBR", edits)
	}
}

func TestBitrateEditsVBRUpperCap(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><videoQualityControlType>VBR</videoQualityControlType><vbrUpperCap>1024</vbrUpperCap></Video></StreamingChannel>`)
	edits, err := bitrateEdits(doc, false, 2048, "")
	if err != nil {
		t.Fatalf("bitrateEdits: %v", err)
	}
	if edits["vbrUpperCap"] != "2048" {
		t.Fatalf("edits = %+v, want vbrUpperCap=2048", edits)
	}
	if _, ok := edits["videoQualityControlType"]; ok {
		t.Fatalf("edits = %+v, mode empty should not touch videoQualityControlType", edits)
	}
}

func TestBitrateEditsSmartAverage(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><videoQualityControlType>VBR</videoQualityControlType><vbrAverageCap>1024</vbrAverageCap><vbrUpperCap>2048</vbrUpperCap></Video></StreamingChannel>`)
	edits, err := bitrateEdits(doc, true, 3072, "")
	if err != nil {
		t.Fatalf("bitrateEdits: %v", err)
	}
	if edits["vbrAverageCap"] != "3072" {
		t.Fatalf("edits = %+v, want vbrAverageCap=3072", edits)
	}
	if _, ok := edits["vbrUpperCap"]; ok {
		t.Fatalf("edits = %+v, should prefer vbrAverageCap over vbrUpperCap when smart on", edits)
	}
}

func TestBitrateEditsSmartFallsBackToUpperCap(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><videoQualityControlType>VBR</videoQualityControlType><vbrUpperCap>2048</vbrUpperCap></Video></StreamingChannel>`)
	edits, err := bitrateEdits(doc, true, 3072, "")
	if err != nil {
		t.Fatalf("bitrateEdits: %v", err)
	}
	if edits["vbrUpperCap"] != "3072" {
		t.Fatalf("edits = %+v, want vbrUpperCap=3072 (no vbrAverageCap tag on this firmware)", edits)
	}
}

func TestBitrateEditsLowercaseMode(t *testing.T) {
	doc := []byte(`<StreamingChannel><Video><videoQualityControlType>vbr</videoQualityControlType><constantBitRate>1024</constantBitRate></Video></StreamingChannel>`)
	edits, err := bitrateEdits(doc, false, 2048, "CBR")
	if err != nil {
		t.Fatalf("bitrateEdits: %v", err)
	}
	if edits["videoQualityControlType"] != "cbr" {
		t.Fatalf("edits = %+v, want lowercase videoQualityControlType=cbr to match device casing", edits)
	}
	if edits["constantBitRate"] != "2048" {
		t.Fatalf("edits = %+v, want constantBitRate=2048", edits)
	}
}

func TestChannelIDMapping(t *testing.T) {
	cases := []struct {
		ch, stream, want int
	}{
		{1, 0, 101}, // channel 1, main
		{1, 1, 102}, // channel 1, sub1
		{1, 2, 103}, // channel 1, sub2
		{2, 0, 201}, // channel 2, main
	}
	for _, tc := range cases {
		if got := channelID(tc.ch, tc.stream); got != tc.want {
			t.Errorf("channelID(%d, %d) = %d, want %d", tc.ch, tc.stream, got, tc.want)
		}
	}
}

func TestReplaceXMLTagInNthBlock(t *testing.T) {
	doc := []byte(`<VideoOverlay><TextOverlayList>` +
		`<TextOverlay><id>1</id><enabled>false</enabled><displayText></displayText></TextOverlay>` +
		`<TextOverlay><id>2</id><enabled>false</enabled><displayText></displayText></TextOverlay>` +
		`</TextOverlayList></VideoOverlay>`)

	out := replaceXMLTagInNthBlock(doc, "TextOverlay", 0, "displayText", "Cổng chính")
	out = replaceXMLTagInNthBlock(out, "TextOverlay", 0, "enabled", "true")
	// The second block must be untouched by edits scoped to the first.
	if extractXMLInBlock(out, "TextOverlayList", "id") != "1" {
		t.Fatalf("unexpected first id after edit: %s", out)
	}
	firstBlockEnd := strings.Index(string(out), "</TextOverlay>")
	first := out[:firstBlockEnd]
	if !strings.Contains(string(first), "<displayText>Cổng chính</displayText>") {
		t.Errorf("first block displayText not set: %s", first)
	}
	if !strings.Contains(string(first), "<enabled>true</enabled>") {
		t.Errorf("first block enabled not set: %s", first)
	}
	second := out[firstBlockEnd:]
	if !strings.Contains(string(second), "<displayText></displayText>") || !strings.Contains(string(second), "<enabled>false</enabled>") {
		t.Errorf("second block was modified by an edit scoped to the first: %s", second)
	}

	// Out-of-range block index leaves the document unchanged.
	unchanged := replaceXMLTagInNthBlock(doc, "TextOverlay", 5, "displayText", "x")
	if string(unchanged) != string(doc) {
		t.Error("out-of-range block index should leave doc unchanged")
	}
}

func TestGetOverlayTextUnsupported(t *testing.T) {
	// A device whose overlays document has no TextOverlayList at all.
	mux := http.NewServeMux()
	mux.HandleFunc("/ISAPI/System/Video/inputs/channels/1/overlays", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<VideoOverlay></VideoOverlay>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	c := New(host, port, false, "u", "p", 5*time.Second)

	if _, _, err := c.GetOverlayText(context.Background(), 1); err != ErrOverlayUnsupported {
		t.Fatalf("want ErrOverlayUnsupported, got %v", err)
	}
}
