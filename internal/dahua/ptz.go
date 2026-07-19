package dahua

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// PTZControl drives PTZ over the existing DVRIP JSON-RPC session (no HTTP CGI,
// so it works on firmware whose ptz.cgi:80 is unreachable/rejected). It creates
// a PTZ control object (ptz.factory.instance) then issues ptz.start (or ptz.stop
// when start=false) with the same {code, arg1, arg2, arg3} the CGI uses — so
// every code works, including Focus*/Iris*. This is the method that actually
// succeeds on these cameras: ptz.moveContinuously (Direction vector) returns
// result=false on this protocol/firmware, whereas ptz.start returns true.
// channel is 0-based; speed is clamped to [1,8] and passed as arg2.
func (c *Client) PTZControl(channel int, code string, speed int, start bool) error {
	if !ptzCodes[code] {
		return fmt.Errorf("dahua: unknown PTZ code %q", code)
	}
	if speed < 1 {
		speed = 1
	}
	if speed > 8 {
		speed = 8
	}

	inst, err := c.Call("ptz.factory.instance", map[string]any{"channel": channel})
	if err != nil {
		return err
	}
	if !inst.ok() {
		return fmt.Errorf("dahua: ptz.factory.instance failed: %s", respErr(inst))
	}
	var objID int64
	if err := json.Unmarshal(inst.Result, &objID); err != nil {
		return fmt.Errorf("dahua: ptz.factory.instance: unexpected result %.60s: %w", inst.Result, err)
	}

	method := "ptz.start"
	if !start {
		method = "ptz.stop"
	}
	resp, err := c.CallObject(method, objID, map[string]any{
		"channel": channel, "code": code, "arg1": 0, "arg2": speed, "arg3": 0,
	})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("dahua: %s failed: %s", method, respErr(resp))
	}
	return nil
}

// ptzCodes are the PTZ operation codes this package accepts, matching Dahua's
// HTTP API (dahua_http_api_for_ipcsd-v1.40.pdf §7.2.3). Restricting to a
// known set keeps an arbitrary attacker-supplied string out of the CGI query
// (the code is otherwise interpolated straight into the URL).
var ptzCodes = map[string]bool{
	"Up": true, "Down": true, "Left": true, "Right": true,
	"LeftUp": true, "LeftDown": true, "RightUp": true, "RightDown": true,
	"ZoomWide": true, "ZoomTele": true,
	"FocusNear": true, "FocusFar": true,
	"IrisLarge": true, "IrisSmall": true,
}

// PTZMove issues one PTZ control command over Dahua's HTTP CGI
// (GET /cgi-bin/ptz.cgi?action=start|stop&channel=<n>&code=<code>&arg1=0&arg2=<speed>&arg3=0),
// per dahua_http_api_for_ipcsd-v1.40.pdf §7.2.3. A PTZ pad works by issuing
// action="start" on press and action="stop" on release (same code), so the
// caller drives continuous motion; a brief start/stop pair is a single nudge.
// channel is 0-based. speed is clamped to the documented [1,8] range (used
// for pan/tilt; zoom/focus/iris ignore it, arg2 = multiple). Like GetSnapshot
// this is plain HTTP+Digest on port 80, separate from the DVRIP session, so
// host must be bare (no port).
func PTZMove(ctx context.Context, host, user, pass string, channel int, code string, speed int, start bool, timeout time.Duration) error {
	if !ptzCodes[code] {
		return fmt.Errorf("dahua: unknown PTZ code %q", code)
	}
	if speed < 1 {
		speed = 1
	}
	if speed > 8 {
		speed = 8
	}
	action := "stop"
	if start {
		action = "start"
	}

	digest := isapi.NewDigestTransport(user, pass, nil)
	client := &http.Client{Transport: digest, Timeout: timeout}
	url := fmt.Sprintf("http://%s:80/cgi-bin/ptz.cgi?action=%s&channel=%d&code=%s&arg1=0&arg2=%d&arg3=0",
		host, action, channel, code, speed)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("dahua: build ptz request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dahua: ptz %s: %w", host, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dahua: ptz %s: HTTP %d: %s", host, resp.StatusCode, snapshotTruncate(body, 200))
	}
	return nil
}
