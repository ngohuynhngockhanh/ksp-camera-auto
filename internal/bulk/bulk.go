// Package bulk drives the camera package across many devices, so the web UI
// can push one profile to a whole inventory in one request. Devices are
// processed one at a time (never concurrently): applying encode changes is a
// disruptive operation (it can drop the stream mid-recording), so bulk runs
// stay sequential for safety and predictability.
package bulk

import (
	"context"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// Request names the devices to touch and the profile to apply to each.
type Request struct {
	DeviceIDs []string       `json:"deviceIds"`
	Profile   camera.Profile `json:"profile"`
}

// DeviceResult is the per-device outcome of an Apply run.
type DeviceResult struct {
	DeviceID string              `json:"deviceId"`
	Name     string              `json:"name"`
	Host     string              `json:"host"`
	OK       bool                `json:"ok"`
	Steps    []camera.StepResult `json:"steps,omitempty"`
	Err      string              `json:"err,omitempty"`
}

// Event is one entry in the live progress log emitted while Apply runs, so
// the web UI can render a transparent, real-time trail of what's happening
// to each camera.
type Event struct {
	Type     string `json:"type"` // "device_start", "step", "device_done", "done"
	DeviceID string `json:"deviceId,omitempty"`
	Name     string `json:"name,omitempty"`
	Host     string `json:"host,omitempty"`
	Step     string `json:"step,omitempty"`
	Detail   string `json:"detail,omitempty"`
	OK       bool   `json:"ok"`
	Err      string `json:"err,omitempty"`
	Index    int    `json:"index,omitempty"`
	Total    int    `json:"total,omitempty"`
}

// Apply runs req.Profile against every device in req.DeviceIDs, one device at
// a time and in order, so a bad profile or a stuck camera can't take down a
// whole batch concurrently. emit (may be nil) is called for every progress
// event as it happens, in addition to the final []DeviceResult being
// returned once the whole run completes.
func Apply(ctx context.Context, inv *config.Inventory, req Request, timeout time.Duration, emit func(Event)) []DeviceResult {
	if emit == nil {
		emit = func(Event) {}
	}
	results := make([]DeviceResult, len(req.DeviceIDs))
	total := len(req.DeviceIDs)

	for i, id := range req.DeviceIDs {
		if ctx.Err() != nil {
			res := DeviceResult{DeviceID: id, Err: ctx.Err().Error()}
			results[i] = res
			emit(Event{Type: "device_start", Index: i + 1, Total: total, DeviceID: id})
			emit(Event{Type: "device_done", DeviceID: id, OK: false, Err: res.Err})
			continue
		}

		d, ok := inv.Get(id)
		name, host := "", ""
		if ok {
			name, host = d.Name, d.Host
		}
		emit(Event{Type: "device_start", Index: i + 1, Total: total, DeviceID: id, Name: name, Host: host})

		res := applyOne(ctx, inv, id, req.Profile, timeout, func(sr camera.StepResult) {
			emit(Event{Type: "step", DeviceID: id, Name: name, Host: host, Step: sr.Step, Detail: sr.Detail, OK: sr.OK, Err: sr.Err})
		})
		results[i] = res
		emit(Event{Type: "device_done", DeviceID: id, Name: res.Name, Host: res.Host, OK: res.OK, Err: res.Err})
	}

	emit(Event{Type: "done", Total: total})
	return results
}

// applyOne opens, applies, and closes a single device, translating any
// connection error into a DeviceResult rather than propagating it. stepEmit
// (may be nil) is forwarded to camera.Apply so each step can be streamed as
// it completes.
func applyOne(ctx context.Context, inv *config.Inventory, id string, profile camera.Profile, timeout time.Duration, stepEmit func(camera.StepResult)) DeviceResult {
	res := DeviceResult{DeviceID: id}

	d, ok := inv.Get(id)
	if !ok {
		res.Err = "device not found in inventory"
		return res
	}
	res.Name = d.Name
	res.Host = d.Host

	cam, err := camera.Open(ctx, d, timeout)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer cam.Close()

	steps := cam.Apply(ctx, profile, stepEmit)
	res.Steps = steps

	res.OK = true
	for _, st := range steps {
		if !st.OK {
			res.OK = false
			break
		}
	}
	return res
}
