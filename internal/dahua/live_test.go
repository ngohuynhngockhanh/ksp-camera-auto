package dahua

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// Live tests run only when KSPCAM_LIVE_DAHUA is set, e.g.
//
//	KSPCAM_LIVE_DAHUA=host:port KSPCAM_LIVE_DAHUA_AUTH=user:pass \
//	  go test ./internal/dahua -run TestLive -v
//
// They exercise a real device and are skipped in normal CI.
func liveTarget(t *testing.T) (addr, user, pass string) {
	t.Helper()
	addr = os.Getenv("KSPCAM_LIVE_DAHUA")
	if addr == "" {
		t.Skip("set KSPCAM_LIVE_DAHUA to run live Dahua tests")
	}
	auth := os.Getenv("KSPCAM_LIVE_DAHUA_AUTH")
	if auth == "" {
		t.Skip("set KSPCAM_LIVE_DAHUA_AUTH=user:pass to run live Dahua tests")
	}
	parts := strings.SplitN(auth, ":", 2)
	user = parts[0]
	if len(parts) > 1 {
		pass = parts[1]
	}
	return addr, user, pass
}

func TestLiveLoginAndDumpEncode(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 10*time.Second)
	if err != nil {
		t.Fatalf("dial/login: %v", err)
	}
	defer c.Close()
	t.Logf("logged in (session established)")

	for _, name := range []string{"Encode", "VideoEncodeROI", "VideoImageControl", "SmartEncode", "EncodeCapability", "VideoInMode"} {
		resp, err := c.Call("configManager.getConfig", map[string]any{"name": name})
		if err != nil {
			t.Logf("getConfig %s: error %v", name, err)
			continue
		}
		pretty, _ := json.MarshalIndent(json.RawMessage(resp.Params), "", "  ")
		_ = os.WriteFile("/tmp/claude-1000/-home-ksp-ksp-camera-auto/6d15c45e-b245-408d-a2ee-1bcfedb28a07/scratchpad/dahua_"+name+".json", pretty, 0o644)
		t.Logf("getConfig %s ok=%v (%d bytes) -> saved", name, resp.ok(), len(resp.Params))
	}
}

func TestLiveSettersRoundTrip(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 10*time.Second)
	if err != nil {
		t.Fatalf("dial/login: %v", err)
	}
	defer c.Close()

	// Read current main-stream state.
	before, err := c.GetStreamInfo(0, StreamMain)
	if err != nil {
		t.Fatalf("GetStreamInfo: %v", err)
	}
	t.Logf("before: %+v", before)

	// 1) Smart codec: toggle and verify, then restore.
	if err := c.SetSmartCodec(0, !before.SmartCodec); err != nil {
		t.Fatalf("SetSmartCodec toggle: %v", err)
	}
	mid, _ := c.GetStreamInfo(0, StreamMain)
	if mid.SmartCodec == before.SmartCodec {
		t.Errorf("smart codec did not change: still %v", mid.SmartCodec)
	} else {
		t.Logf("smart codec %v -> %v OK", before.SmartCodec, mid.SmartCodec)
	}
	if err := c.SetSmartCodec(0, before.SmartCodec); err != nil {
		t.Errorf("SetSmartCodec restore: %v", err)
	}

	// 2) Audio AAC (idempotent-safe: force AAC).
	if err := c.SetAudioAAC(0, StreamMain); err != nil {
		t.Errorf("SetAudioAAC: %v", err)
	} else {
		after, _ := c.GetStreamInfo(0, StreamMain)
		t.Logf("audio codec now %q enable=%v", after.AudioCodec, after.AudioEnable)
	}

	// 3) Resolution round-trip on the main stream using 1920x1080 (a widely
	//    supported 16:9 mode), then restore the original. The device rejects
	//    unsupported resolutions with an explicit error, so valid values must
	//    come from the device's capability list (enumerated by the UI).
	if err := c.SetResolution(0, StreamMain, 1920, 1080); err != nil {
		t.Fatalf("SetResolution 1920x1080: %v", err)
	}
	chk, _ := c.GetStreamInfo(0, StreamMain)
	if chk.Width != 1920 || chk.Height != 1080 {
		t.Errorf("resolution not applied: got %dx%d", chk.Width, chk.Height)
	} else {
		t.Logf("resolution 1920x1080 applied OK")
	}
	if before.Width > 0 && before.Height > 0 {
		if err := c.SetResolution(0, StreamMain, before.Width, before.Height); err != nil {
			t.Errorf("restore resolution: %v", err)
		}
		fin, _ := c.GetStreamInfo(0, StreamMain)
		t.Logf("restored main -> %dx%d", fin.Width, fin.Height)
	}
}

func TestLiveGOPBitrateRoundTrip(t *testing.T) {
	addr, user, pass := liveTarget(t)
	c, err := Dial(addr, user, pass, 10*time.Second)
	if err != nil {
		t.Fatalf("dial/login: %v", err)
	}
	defer c.Close()

	before, err := c.GetStreamInfo(0, StreamMain)
	if err != nil {
		t.Fatalf("GetStreamInfo: %v", err)
	}
	t.Logf("before: GOP=%d BitRate=%d BitRateControl=%q", before.GOP, before.BitRate, before.BitRateControl)

	// 1) GOP round-trip: pick a value different from the current one.
	newGOP := 50
	if before.GOP == 50 {
		newGOP = 60
	}
	if err := c.SetGOP(0, StreamMain, newGOP); err != nil {
		t.Fatalf("SetGOP %d: %v", newGOP, err)
	}
	after, err := c.GetStreamInfo(0, StreamMain)
	if err != nil {
		t.Fatalf("GetStreamInfo after SetGOP: %v", err)
	}
	if after.GOP != newGOP {
		t.Errorf("GOP not applied: want %d, got %d", newGOP, after.GOP)
	} else {
		t.Logf("GOP %d -> %d OK", before.GOP, after.GOP)
	}
	if before.GOP > 0 {
		if err := c.SetGOP(0, StreamMain, before.GOP); err != nil {
			t.Errorf("restore GOP: %v", err)
		}
		fin, _ := c.GetStreamInfo(0, StreamMain)
		t.Logf("restored GOP -> %d", fin.GOP)
	}

	// 2) Bitrate round-trip. The device may clamp to a supported step, so only
	//    require that the value CHANGED from before OR equals what we asked for.
	newBR := before.BitRate + 512
	if newBR <= 0 {
		newBR = 2048
	}
	if err := c.SetBitrate(0, StreamMain, newBR, ""); err != nil {
		t.Fatalf("SetBitrate %d: %v", newBR, err)
	}
	after, err = c.GetStreamInfo(0, StreamMain)
	if err != nil {
		t.Fatalf("GetStreamInfo after SetBitrate: %v", err)
	}
	t.Logf("bitrate: requested %d Kbps, device now %d Kbps (control %q)", newBR, after.BitRate, after.BitRateControl)
	if after.BitRate == before.BitRate && after.BitRate != newBR {
		t.Errorf("bitrate did not change: still %d Kbps (wanted %d)", after.BitRate, newBR)
	}
	if before.BitRate > 0 {
		if err := c.SetBitrate(0, StreamMain, before.BitRate, ""); err != nil {
			t.Errorf("restore bitrate: %v", err)
		}
		fin, _ := c.GetStreamInfo(0, StreamMain)
		t.Logf("restored bitrate -> %d Kbps", fin.BitRate)
	}

	// 3) Optionally flip the bitrate control mode. Not all streams support both
	//    CBR and VBR, so log rather than hard-fail on the mode flip.
	if before.BitRateControl == "VBR" {
		if err := c.SetBitrate(0, StreamMain, newBR, "CBR"); err != nil {
			t.Logf("SetBitrate CBR (mode flip not supported?): %v", err)
		} else {
			flip, _ := c.GetStreamInfo(0, StreamMain)
			t.Logf("mode flip VBR -> %q (bitrate %d Kbps)", flip.BitRateControl, flip.BitRate)
			// Restore original mode + bitrate.
			if before.BitRate > 0 {
				if err := c.SetBitrate(0, StreamMain, before.BitRate, before.BitRateControl); err != nil {
					t.Errorf("restore bitrate+mode: %v", err)
				}
				fin, _ := c.GetStreamInfo(0, StreamMain)
				t.Logf("restored bitrate+mode -> %d Kbps %q", fin.BitRate, fin.BitRateControl)
			}
		}
	}
}
