// Package bulk drives the camera package across many devices concurrently,
// so the web UI can push one profile to a whole inventory in one request.
package bulk

import (
	"context"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// maxWorkers bounds how many devices are contacted at once, so a large
// inventory doesn't open hundreds of TCP connections simultaneously.
const maxWorkers = 8

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

// Apply runs req.Profile against every device in req.DeviceIDs concurrently
// (bounded to maxWorkers) and returns results in the same order as
// req.DeviceIDs regardless of completion order.
func Apply(ctx context.Context, inv *config.Inventory, req Request, timeout time.Duration) []DeviceResult {
	results := make([]DeviceResult, len(req.DeviceIDs))
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, id := range req.DeviceIDs {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = applyOne(ctx, inv, id, req.Profile, timeout)
		}(i, id)
	}
	wg.Wait()
	return results
}

// applyOne opens, applies, and closes a single device, translating any
// connection error into a DeviceResult rather than propagating it.
func applyOne(ctx context.Context, inv *config.Inventory, id string, profile camera.Profile, timeout time.Duration) DeviceResult {
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

	steps, err := cam.Apply(ctx, profile)
	res.Steps = steps
	if err != nil {
		res.Err = err.Error()
		return res
	}

	res.OK = true
	for _, st := range steps {
		if !st.OK {
			res.OK = false
			break
		}
	}
	return res
}
