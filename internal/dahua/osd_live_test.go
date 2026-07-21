package dahua

// Live probe for the KBVision-on-8888 CustomTitle write puzzle: setConfig
// VideoWidget returns result=true but the device silently keeps the old
// values (seen on inut_204_110's H5AE/SSC377 IPCs, 2026-07-21), while
// ChannelTitle writes persist fine. This test tries write variants to find
// one the firmware actually honors. Gated: needs KSPCAM_LIVE_OSD=1 plus
// KSPCAM_ADDR/KSPCAM_USER/KSPCAM_PASS.
//
//	GOOS=linux GOARCH=arm64 go test -c ./internal/dahua -o dahua_osd.test
//	KSPCAM_LIVE_OSD=1 KSPCAM_ADDR=192.168.0.173:8888 KSPCAM_USER=admin \
//	  KSPCAM_PASS=... ./dahua_osd.test -test.run TestLiveOSDVariants -test.v

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func liveOSDClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("KSPCAM_LIVE_OSD") == "" {
		t.Skip("set KSPCAM_LIVE_OSD=1 to run")
	}
	addr, user, pass := os.Getenv("KSPCAM_ADDR"), os.Getenv("KSPCAM_USER"), os.Getenv("KSPCAM_PASS")
	if addr == "" || user == "" {
		t.Skip("set KSPCAM_ADDR and KSPCAM_USER")
	}
	c, err := Dial(addr, user, pass, 10*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// readSlot0 returns CustomTitle[0].Text and .EncodeBlend for channel 0.
func readSlot0(t *testing.T, c *Client) (string, bool) {
	t.Helper()
	table, err := c.getTable("VideoWidget")
	if err != nil {
		t.Fatalf("getTable VideoWidget: %v", err)
	}
	slots, err := customTitleSlots(table, 0)
	if err != nil {
		t.Fatalf("slots: %v", err)
	}
	obj := slots[0].(map[string]any)
	text, _ := obj["Text"].(string)
	eb, _ := obj["EncodeBlend"].(bool)
	return text, eb
}

// mutate returns a fresh VideoWidget table with slot0 Text/EncodeBlend set.
func mutatedTable(t *testing.T, c *Client, text string) []any {
	t.Helper()
	table, err := c.getTable("VideoWidget")
	if err != nil {
		t.Fatalf("getTable: %v", err)
	}
	slots, err := customTitleSlots(table, 0)
	if err != nil {
		t.Fatalf("slots: %v", err)
	}
	obj := slots[0].(map[string]any)
	obj["Text"] = text
	obj["EncodeBlend"] = true
	obj["PreviewBlend"] = true
	return table
}

func TestLiveOSDVariants(t *testing.T) {
	c := liveOSDClient(t)

	before, eb := readSlot0(t, c)
	t.Logf("initial slot0: text=%q encodeBlend=%v", before, eb)

	check := func(name, want string) bool {
		got, gotEB := readSlot0(t, c)
		t.Logf("%s: read-back text=%q encodeBlend=%v (want %q)", name, got, gotEB, want)
		return got == want
	}

	// V1: plain full-table round-trip (the current production path).
	v1 := "v1|test||"
	if err := c.setTable("VideoWidget", mutatedTable(t, c, v1)); err != nil {
		t.Logf("V1 setTable err: %v", err)
	} else if check("V1 full-table", v1) {
		t.Log("V1 WORKS")
		return
	}

	// V2: full table minus the GBMode subtree (suspected read-only key that
	// may make the firmware discard the whole write).
	v2 := "v2|test||"
	table := mutatedTable(t, c, v2)
	for _, chAny := range table {
		if obj, ok := chAny.(map[string]any); ok {
			delete(obj, "GBMode")
		}
	}
	if err := c.setTable("VideoWidget", table); err != nil {
		t.Logf("V2 setTable err: %v", err)
	} else if check("V2 no-GBMode", v2) {
		t.Log("V2 WORKS")
		return
	}

	// V3: channel objects carrying ONLY the CustomTitle key.
	v3 := "v3|test||"
	full := mutatedTable(t, c, v3)
	slim := make([]any, len(full))
	for i, chAny := range full {
		obj, _ := chAny.(map[string]any)
		slim[i] = map[string]any{"CustomTitle": obj["CustomTitle"]}
	}
	if err := c.setTable("VideoWidget", slim); err != nil {
		t.Logf("V3 setTable err: %v", err)
	} else if check("V3 CustomTitle-only", v3) {
		t.Log("V3 WORKS")
		return
	}

	// V4: setConfig with an explicit empty options array.
	v4 := "v4|test||"
	resp, err := c.Call("configManager.setConfig", map[string]any{
		"name": "VideoWidget", "table": mutatedTable(t, c, v4), "options": []any{},
	})
	if err != nil || !resp.ok() {
		t.Logf("V4 err: %v resp: %s / %s", err, resp.Result, resp.Error)
	} else if check("V4 options[]", v4) {
		t.Log("V4 WORKS")
		return
	}

	// V5: per-channel name "VideoWidget[0]" with the channel OBJECT as table.
	v5 := "v5|test||"
	full = mutatedTable(t, c, v5)
	resp, err = c.Call("configManager.setConfig", map[string]any{
		"name": "VideoWidget[0]", "table": full[0],
	})
	if err != nil || !resp.ok() {
		t.Logf("V5 err: %v resp: %s / %s", err, resp.Result, resp.Error)
	} else if check("V5 per-channel name", v5) {
		t.Log("V5 WORKS")
		return
	}

	// V6: NetSDK CLIENT_SetNewDevConfig shape — per-channel object with an
	// explicit "channel" param (what the vendor plugin actually sends).
	v6 := "v6|test||"
	full = mutatedTable(t, c, v6)
	resp, err = c.Call("configManager.setConfig", map[string]any{
		"name": "VideoWidget", "table": full[0], "channel": 0,
	})
	if err != nil || !resp.ok() {
		t.Logf("V6 err: %v resp: %s / %s", err, resp.Result, resp.Error)
	} else if check("V6 channel-param obj", v6) {
		t.Log("V6 WORKS")
		return
	}

	// V7: same but table stays an array.
	v7 := "v7|test||"
	resp, err = c.Call("configManager.setConfig", map[string]any{
		"name": "VideoWidget", "table": mutatedTable(t, c, v7), "channel": 0,
	})
	if err != nil || !resp.ok() {
		t.Logf("V7 err: %v resp: %s / %s", err, resp.Result, resp.Error)
	} else if check("V7 channel-param array", v7) {
		t.Log("V7 WORKS")
		return
	}

	// V8: object-oriented configManager instance, setConfig on the handle.
	v8 := "v8|test||"
	inst, err := c.Call("configManager.factory.instance", map[string]any{})
	if err == nil && inst.ok() {
		var objID int64
		if jerr := json.Unmarshal(inst.Result, &objID); jerr == nil && objID != 0 {
			resp, err = c.CallObject("configManager.setConfig", objID, map[string]any{
				"name": "VideoWidget", "table": mutatedTable(t, c, v8),
			})
			if err != nil || !resp.ok() {
				t.Logf("V8 err: %v resp: %s / %s", err, resp.Result, resp.Error)
			} else if check("V8 object setConfig", v8) {
				t.Log("V8 WORKS")
				return
			}
			_, _ = c.CallObject("configManager.destroy", objID, map[string]any{})
		} else {
			t.Logf("V8 instance result not an id: %s", inst.Result)
		}
	} else {
		t.Logf("V8 instance err: %v resp: %s / %s", err, inst.Result, inst.Error)
	}

	// Diagnostics: dump slot0's full JSON and enumerate configManager methods.
	table, _ = c.getTable("VideoWidget")
	if slots, err := customTitleSlots(table, 0); err == nil {
		b, _ := json.Marshal(slots[0])
		t.Logf("slot0 raw: %s", b)
	}
	for _, m := range []string{"configManager.listMethod", "ConfigManager.listMethod"} {
		if resp, err := c.Call(m, map[string]any{}); err == nil && resp.ok() {
			t.Logf("%s: params=%s result=%s", m, resp.Params, resp.Result)
		}
	}
	t.Log(fmt.Sprintf("no variant persisted; slot0 still %q", before))
}
