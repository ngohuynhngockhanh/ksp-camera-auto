package dahua

import (
	"context"
	"encoding/hex"
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
