package dahua

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// TestLiveStreamDav exercises the pure-Go StreamDav end-to-end against a live
// camera (or the frp tunnel), downloading a short range and asserting the output
// is a genuine DHAV .dav.
//
//	KSPCAM_LIVE_DAHUA=host KSPCAM_LIVE_DAHUA_AUTH=user:pass \
//	KSPCAM_DAV_START="2026-01-02 15:04:05" KSPCAM_DAV_END="..." \
//	  go test ./internal/dahua -run TestLiveStreamDav -v
func TestLiveStreamDav(t *testing.T) {
	addr, user, pass := liveTarget(t)
	host := bareHost(addr)
	layout := "2006-01-02 15:04:05"
	startEnv, endEnv := os.Getenv("KSPCAM_DAV_START"), os.Getenv("KSPCAM_DAV_END")
	if startEnv == "" || endEnv == "" {
		t.Skip("set KSPCAM_DAV_START and KSPCAM_DAV_END (device-local 'YYYY-MM-DD HH:MM:SS')")
	}
	start, err := time.ParseInLocation(layout, startEnv, time.Local)
	if err != nil {
		t.Fatalf("bad KSPCAM_DAV_START: %v", err)
	}
	end, err := time.ParseInLocation(layout, endEnv, time.Local)
	if err != nil {
		t.Fatalf("bad KSPCAM_DAV_END: %v", err)
	}
	out := "/tmp/streamdav_test.dav"
	_ = os.Remove(out)
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := StreamDav(ctx, f, host, user, pass, 0, start, end); err != nil {
		t.Fatalf("StreamDav: %v", err)
	}
	fi, _ := f.Stat()
	head := make([]byte, 4)
	f.Seek(0, 0)
	f.Read(head)
	t.Logf("StreamDav wrote %s (%d bytes), magic=%q", out, fi.Size(), head)
	if string(head) != "DHAV" {
		t.Fatalf("not a DHAV file: magic=%x", head)
	}
	if fi.Size() < 100000 {
		t.Fatalf("suspiciously small: %d bytes", fi.Size())
	}
}

// TestLiveSnapReplay replays the post-login bytes of a captured NetSDK
// SnapPictureEx session over a fresh DVRIP login, to confirm which frames
// trigger the JPEG snapshot (returned in 0xbc frames). SNAP_C2S = path to the
// captured client->server dump.
func TestLiveSnapReplay(t *testing.T) {
	addr, user, pass := liveTarget(t)
	capPath := os.Getenv("SNAP_C2S")
	if capPath == "" {
		t.Skip("set SNAP_C2S to the captured c2s dump")
	}
	raw, err := os.ReadFile(capPath)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Dial(addr, user, pass, 15*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	t.Logf("logged in, session=%d", c.sessionID)

	// Post-login region: skip the capture's own two \xa0 login frames (realm req
	// 32B @0, login-hash frame 32B+71B @0x20 -> ends 0x87). SNAP_OFF/SNAP_END
	// (hex) narrow the replayed byte range to isolate the snapshot command.
	startOff, endOff := 0x87, len(raw)
	if v := os.Getenv("SNAP_OFF"); v != "" {
		fmt.Sscanf(v, "%x", &startOff)
	}
	if v := os.Getenv("SNAP_END"); v != "" {
		fmt.Sscanf(v, "%x", &endOff)
	}
	post := raw[startOff:endOff]
	t.Logf("replaying bytes [0x%x:0x%x] (%d bytes)", startOff, endOff, len(post))
	if err := c.writeRaw(post); err != nil {
		t.Fatalf("replay write: %v", err)
	}
	// Read frames until we collect a full JPEG (0xbc frames ending with len 0).
	var jpeg []byte
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		hdr, payload, err := c.readFrame()
		if err != nil {
			t.Logf("read end: %v", err)
			break
		}
		t.Logf("frame magic=0x%02x len=%d", hdr[0], len(payload))
		if hdr[0] == 0xbc {
			if len(payload) == 0 {
				break
			}
			jpeg = append(jpeg, payload...)
		}
	}
	if len(jpeg) == 0 {
		t.Fatal("no JPEG received from replay")
	}
	out := "/tmp/snap_replay.jpg"
	_ = os.WriteFile(out, jpeg, 0o644)
	t.Logf("got %d JPEG bytes -> %s (magic %x)", len(jpeg), out, jpeg[:4])
	if jpeg[0] != 0xff || jpeg[1] != 0xd8 {
		t.Fatalf("not a JPEG: %x", jpeg[:4])
	}
}

func TestLiveSnapDVRIP(t *testing.T) {
	addr, user, pass := liveTarget(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ch := 0
	if v := os.Getenv("SNAP_CH"); v != "" {
		fmt.Sscanf(v, "%d", &ch)
	}
	t0 := time.Now()
	jpeg, err := GetSnapshotDVRIP(ctx, bareHost(addr), user, pass, ch, 10*time.Second)
	if err != nil {
		t.Fatalf("GetSnapshotDVRIP: %v", err)
	}
	_ = os.WriteFile("/tmp/snap_dvrip.jpg", jpeg, 0o644)
	t.Logf("got %d-byte JPEG in %v (magic %x) -> /tmp/snap_dvrip.jpg", len(jpeg), time.Since(t0), jpeg[:4])
	if jpeg[0] != 0xff || jpeg[1] != 0xd8 {
		t.Fatalf("not JPEG: %x", jpeg[:4])
	}
}

func TestLivePTZProbe(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 15*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	// device model
	for _, m := range []string{"magicBox.getDeviceType", "magicBox.getProductDefinition"} {
		r, _ := c.Call(m, nil)
		t.Logf("%s ok=%v params=%.160s", m, r.ok(), string(r.Params))
	}
	// PTZ config present?
	r, _ := c.Call("configManager.getConfig", map[string]any{"name": "Ptz"})
	t.Logf("getConfig Ptz ok=%v err=%q params=%.200s", r.ok(), r.errMessage(), string(r.Params))
	// instance + caps + a move
	inst, _ := c.Call("ptz.factory.instance", map[string]any{"channel": 0})
	t.Logf("ptz.factory.instance ok=%v result=%.40s err=%q", inst.ok(), string(inst.Result), inst.errMessage())
	var oid int64
	_ = json.Unmarshal(inst.Result, &oid)
	if oid == 0 {
		return
	}
	lm, _ := c.CallObject("ptz.listMethod", oid, nil)
	t.Logf("ptz.listMethod: %.400s", string(lm.Params)+string(lm.Result))
	// Try move variants; log which succeeds.
	variants := []struct {
		method string
		params any
	}{
		{"ptz.moveContinuously", map[string]any{"Direction": []int{0, 3, 0}}},
		{"ptz.moveContinuously", map[string]any{"channel": 0, "Direction": []int{0, 3, 0}}},
		{"ptz.start", map[string]any{"code": "Up", "arg1": 0, "arg2": 3, "arg3": 0}},
		{"ptz.start", map[string]any{"channel": 0, "code": "Up", "arg1": 0, "arg2": 3, "arg3": 0}},
		{"ptz.move", map[string]any{"Direction": []int{0, 3, 0}}},
		{"ptz.moveDirectly", map[string]any{"Direction": []int{0, 3, 0}}},
	}
	for _, v := range variants {
		r, _ := c.CallObject(v.method, oid, v.params)
		t.Logf("%-22s %v -> ok=%v result=%.40s err=%q", v.method, v.params, r.ok(), string(r.Result), r.errMessage())
		st, _ := c.CallObject("ptz.stop", oid, map[string]any{"code": "Up", "arg1": 0, "arg2": 0, "arg3": 0})
		_ = st
		time.Sleep(300 * time.Millisecond)
	}
}

func TestLivePTZControl(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 15*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if err := c.PTZControl(0, "Up", 3, true); err != nil {
		t.Fatalf("PTZControl start: %v", err)
	}
	t.Log("PTZ start Up OK")
	time.Sleep(700 * time.Millisecond)
	if err := c.PTZControl(0, "Up", 3, false); err != nil {
		t.Fatalf("PTZControl stop: %v", err)
	}
	t.Log("PTZ stop Up OK")
}

type mjpegCounter struct{ frames, bytes int }

func (m *mjpegCounter) Write(p []byte) (int, error) {
	if i := indexJPEG(p); i >= 0 {
		m.frames++
	}
	m.bytes += len(p)
	return len(p), nil
}
func indexJPEG(p []byte) int {
	for i := 0; i+1 < len(p); i++ {
		if p[i] == 0xff && p[i+1] == 0xd8 {
			return i
		}
	}
	return -1
}

func TestLiveMJPEG(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 10*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m := &mjpegCounter{}
	err = StreamMJPEG(ctx, m, nil, bareHost(addr), user, pass, 0, 8, "frame")
	t.Logf("StreamMJPEG err=%v frames=%d bytes=%d", err, m.frames, m.bytes)
	if m.frames < 3 {
		t.Fatalf("too few frames: %d", m.frames)
	}
}

func TestLiveWiFiProbe(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 15*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	log := func(tag string, r rpcResp, e error) {
		t.Logf("%-46s err=%v ok=%v rpcErr=%q params=%.220s", tag, e, r.ok(), r.errMessage(), string(r.Params))
	}
	// current impl
	r, e := c.Call("netApp.scanWLanDevices", map[string]any{"Name": "wlan0"})
	log("Call netApp.scanWLanDevices{Name:wlan0}", r, e)
	// via instance
	inst, _ := c.Call("netApp.factory.instance", nil)
	t.Logf("netApp.factory.instance ok=%v result=%.20s", inst.ok(), string(inst.Result))
	var oid int64
	_ = json.Unmarshal(inst.Result, &oid)
	if oid != 0 {
		r, e = c.CallObject("netApp.scanWLanDevices", oid, map[string]any{"Name": "wlan0"})
		log("obj netApp.scanWLanDevices{Name:wlan0}", r, e)
		lm, _ := c.CallObject("netApp.listMethod", oid, nil)
		t.Logf("netApp methods: %.400s", string(lm.Params))
	}
	// scan-then-get async pattern candidates
	for _, m := range []struct {
		method string
		params any
	}{
		{"netApp.getWlanDevices", map[string]any{"Name": "wlan0"}},
		{"netApp.refreshWlanDevices", map[string]any{"Name": "wlan0"}},
		{"WlanManager.getScanResult", map[string]any{"Name": "wlan0"}},
		{"configManager.getConfig", map[string]any{"name": "WLan"}},
	} {
		rr, ee := c.Call(m.method, m.params)
		log("Call "+m.method, rr, ee)
	}
}

// davMagic is the 4-byte signature at the start of a genuine Dahua .dav
// (DHAV container) file. The probe uses it to tell a real native recording
// from an HTML/JSON error body.
var davMagic = []byte("DHAV")

// bareHost strips the :port from a DVRIP addr ("host:37777" -> "host") so the
// HTTP probes can hit the CGI web port (:80), like ptz.go / network.go do.
func bareHost(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return addr
}

// pickRecording logs in, finds recordings in the last `within`, and returns the
// most recent segment. Skips the test if the device has none.
func pickRecording(t *testing.T, c *Client, channel int, within time.Duration) Recording {
	t.Helper()
	end := time.Now()
	start := end.Add(-within)
	recs, err := c.FindRecordings(channel, start, end)
	if err != nil {
		t.Fatalf("FindRecordings: %v", err)
	}
	if len(recs) == 0 {
		t.Skipf("no recordings on ch%d in the last %s", channel, within)
	}
	r := recs[len(recs)-1]
	t.Logf("picked segment: ch%d %s..%s dur=%ds len=%d bytes\n  FilePath=%q",
		r.Channel, r.StartTime, r.EndTime, r.Duration, r.Length, r.FilePath)
	return r
}

// TestLiveDavProbe is the Phase-0 discovery instrument. It does NOT assert a
// working download — it exercises the candidate native-.dav transports against
// a real camera and dumps what comes back, so we can decide which to implement.
//
//	KSPCAM_LIVE_DAHUA=host:37777 KSPCAM_LIVE_DAHUA_AUTH=user:pass \
//	  go test ./internal/dahua -run TestLiveDavProbe -v
//
// Optional: KSPCAM_LIVE_CHANNEL (default 0).
func TestLiveDavProbe(t *testing.T) {
	addr, user, pass := liveTarget(t)
	channel := 0
	c, err := Dial(addr, user, pass, 15*time.Second)
	if err != nil {
		t.Fatalf("dial/login: %v", err)
	}
	defer c.Close()
	t.Logf("logged in to %s (session established)", addr)

	rec := pickRecording(t, c, channel, 24*time.Hour)
	host := bareHost(addr)

	// ---- Track A: HTTP RPC_Loadfile by on-device path (digest, :80) ----------
	// Dahua HTTP API: GET /cgi-bin/RPC_Loadfile<FilePath> streams the native
	// .dav. FilePath already starts with "/", so the URL path is
	// /cgi-bin/RPC_Loadfile/mnt/... — kept literal (the [ ] @ chars in Dahua
	// paths must NOT be percent-encoded).
	t.Run("http_rpc_loadfile_bypath", func(t *testing.T) {
		if rec.FilePath == "" {
			t.Skip("segment has no FilePath")
		}
		url := fmt.Sprintf("http://%s:80/cgi-bin/RPC_Loadfile%s", host, rec.FilePath)
		probeHTTP(t, user, pass, url)
	})

	// ---- Track A': HTTP loadfile.cgi by time range --------------------------
	// The by-time variant (spans segments) — if this streams DHAV it satisfies
	// "range -> one .dav" in a single request.
	t.Run("http_loadfile_bytime", func(t *testing.T) {
		const f = "2006-01-02%2015:04:05"
		st, _ := time.Parse(deviceTimeLayout, rec.StartTime)
		et, _ := time.Parse(deviceTimeLayout, rec.EndTime)
		if st.IsZero() || et.IsZero() {
			t.Skip("segment times unparseable")
		}
		url := fmt.Sprintf("http://%s:80/cgi-bin/loadfile.cgi?action=startLoad&channel=%d&startTime=%s&endTime=%s&subtype=0",
			host, channel+1, st.Format(f), et.Format(f))
		probeHTTP(t, user, pass, url)
	})

	// ---- Track B: enumerate the device's RPC methods -------------------------
	// HTTP:80 is dead here and RPC_Loadfile isn't a JSON method, so discover what
	// the DVRIP socket actually exposes. Dump the full method/service list to the
	// dump dir and log any entry hinting at file load/download/playback/transfer.
	t.Run("dvrip_enumerate", func(t *testing.T) {
		dump := os.Getenv("KSPCAM_DUMP_DIR")
		for _, m := range []string{"system.listMethod", "system.listService", "console.listServices"} {
			resp, err := c.Call(m, nil)
			if err != nil {
				t.Logf("%s: transport error: %v", m, err)
				continue
			}
			blob := string(resp.Params) + string(resp.Result)
			t.Logf("%s: ok=%v err=%q (%d bytes)", m, resp.ok(), resp.errMessage(), len(blob))
			if dump != "" && len(blob) > 4 {
				_ = os.WriteFile(dump+"/"+strings.ReplaceAll(m, ".", "_")+".json", []byte(blob), 0o644)
			}
			// Log matching method names so signal shows even if output is tailed.
			for _, kw := range []string{"load", "file", "download", "playback", "transfer", "dav", "record"} {
				for _, tok := range extractTokens(blob, kw) {
					t.Logf("  [%s] %s", kw, tok)
				}
			}
		}
	})

	// ---- Track C: candidate download factories/methods -----------------------
	// Native download on newer Dahua is an object interface (like mediaFileFind).
	// Try the likely factory/method names and dump what each returns.
	t.Run("dvrip_candidates", func(t *testing.T) {
		const f = "2006-01-02 15:04:05"
		byName := map[string]any{"Channel": channel, "FilePath": rec.FilePath, "PlayMode": "ByName", "StartTime": rec.StartTime, "EndTime": rec.EndTime}
		byTime := map[string]any{"Channel": channel, "StartTime": rec.StartTime, "EndTime": rec.EndTime, "Types": []string{"dav"}}
		_ = f
		for _, cand := range []struct {
			method string
			params any
		}{
			{"system.getCaps", nil},
			{"magicBox.getSystemInfo", nil},
			{"mediaFileFind.factory.create", nil}, // known-good control
			{"RecordManager.factory.create", nil},
			{"RecordManager.getCaps", nil},
			{"RecordFinder.factory.create", nil},
			{"NetApp.factory.instance", nil},
			{"DownLoad.factory.create", nil},
			{"Download.factory.create", nil},
			{"downLoad.factory.create", nil},
			{"RecordDownload.factory.create", nil},
			{"MediaFileDownload.factory.create", nil},
			{"mediaFileFind.factory.instance", nil},
			{"mediaRealMonitor.factory.create", nil},
			{"RPC_LoadfileByTime.factory.create", nil},
			{"loadfileManager.factory.create", nil},
			{"playbackByTime.factory.create", nil},
			{"RecordFilePlayback.factory.create", nil},
			{"streamManager.factory.create", nil},
			{"RPC_Loadfile", map[string]any{"filepath": rec.FilePath}},
			{"RPC_LoadfileByTime", byTime},
			{"OPPlayBack", map[string]any{"OPPlayBack": byName}},
		} {
			resp, err := c.Call(cand.method, cand.params)
			if err != nil {
				t.Logf("%-32s transport error: %v", cand.method, err)
				continue
			}
			t.Logf("%-32s ok=%v err=%q result=%.120s params=%.120s",
				cand.method, resp.ok(), resp.errMessage(), string(resp.Result), string(resp.Params))
		}
	})
}

// extractTokens returns short quoted-string tokens in blob that contain kw
// (case-insensitive), deduped and capped — used to surface interesting method
// names from a big listMethod dump.
func extractTokens(blob, kw string) []string {
	lower := strings.ToLower(blob)
	seen := map[string]bool{}
	var out []string
	for i := 0; i+len(kw) <= len(lower) && len(out) < 12; i++ {
		if lower[i:i+len(kw)] != kw {
			continue
		}
		// widen to a token boundary of [A-Za-z0-9_.]
		s, e := i, i+len(kw)
		for s > 0 && isTokChar(blob[s-1]) {
			s--
		}
		for e < len(blob) && isTokChar(blob[e]) {
			e++
		}
		tok := blob[s:e]
		if !seen[tok] && len(tok) < 60 {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	return out
}

func isTokChar(b byte) bool {
	return b == '.' || b == '_' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// probeHTTP does one digest GET, reports status/content-type/content-length,
// checks the first bytes for the DHAV magic, and saves a small head sample to
// the scratchpad for inspection.
func probeHTTP(t *testing.T, user, pass, url string) {
	t.Helper()
	t.Logf("GET %s", url)
	digest := isapi.NewDigestTransport(user, pass, nil)
	client := &http.Client{Transport: digest, Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("  request error: %v", err)
		return
	}
	defer resp.Body.Close()
	head := make([]byte, 64)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	isDav := n >= 4 && string(head[:4]) == string(davMagic)
	t.Logf("  HTTP %d  Content-Type=%q  Content-Length=%s  first%dB=%s  DHAV=%v",
		resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"),
		n, hex.EncodeToString(head), isDav)
	if !isDav && n > 0 {
		t.Logf("  (head as text) %q", strings.TrimSpace(string(head)))
	}
	if isDav {
		// Drain a bit more to confirm the stream really flows, then report size.
		more, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<20))
		t.Logf("  ✅ genuine .dav — streamed %d more bytes past the header", more)
		if dir := os.Getenv("KSPCAM_DUMP_DIR"); dir != "" {
			_ = os.WriteFile(dir+"/probe_head.dav", head, 0o644)
			t.Logf("  saved head sample to %s/probe_head.dav", dir)
		}
	}
}
