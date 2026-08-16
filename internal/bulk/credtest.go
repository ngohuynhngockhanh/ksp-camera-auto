package bulk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// CredTestTarget is one device to try a candidate password against — built
// straight from a discovery.Result, never persisted to the inventory.
type CredTestTarget struct {
	IP     string `json:"ip"`
	Vendor string `json:"vendor"`
	// Port is 0 when the discovery method that found this device doesn't
	// report one (UDP discovery — ONVIF/Dahua/SADP); TryPasswords fills it
	// from config.Defaults (or the built-in default when config is partial).
	Port int `json:"port"`
	// Label is a human-readable hint (Model/MAC/Name from the scan result)
	// carried through purely for the caller's own display purposes.
	Label string `json:"label,omitempty"`
}

// CredTestEvent is one entry in the live progress log emitted while
// TryPasswords runs, mirroring bulk.Event's shape for the SSE stream.
type CredTestEvent struct {
	Type  string `json:"type"` // "start", "result", "done"
	Index int    `json:"index,omitempty"`
	Total int    `json:"total,omitempty"`
	IP    string `json:"ip,omitempty"`
	Label string `json:"label,omitempty"`
	OK    bool   `json:"ok"`
	Err   string `json:"err,omitempty"`
}

// TryPasswords attempts to log into every target with the same
// username/password, one device at a time and in order — never
// concurrently, so a slow or hung camera can't stall the whole batch behind
// it and a fleet isn't hit with a burst of simultaneous auth attempts.
// Targets with an unresolved Vendor are skipped with a clear error rather
// than guessed at. emit (may be nil) is called for every progress event as
// it happens, in addition to the final []CredTestEvent (one "result" per
// target, in input order) being returned once the run completes.
//
// Login verification asymmetry: for Dahua, camera.Open itself performs the
// DVRIP login, so a bad password fails there. For Hikvision, Open never
// touches the network (see camera.Open's doc comment) — only Probe issues a
// real authenticated request. Both vendors call Probe here so the check is
// uniform and doesn't rely on knowing which vendor's Open is decisive.
func TryPasswords(ctx context.Context, targets []CredTestTarget, username, password string, defaults config.Defaults, timeout time.Duration, emit func(CredTestEvent)) []CredTestEvent {
	if emit == nil {
		emit = func(CredTestEvent) {}
	}
	results := make([]CredTestEvent, len(targets))
	total := len(targets)

	for i, t := range targets {
		emit(CredTestEvent{Type: "start", Index: i + 1, Total: total, IP: t.IP, Label: t.Label})

		res := CredTestEvent{Type: "result", Index: i + 1, Total: total, IP: t.IP, Label: t.Label}
		if ctx.Err() != nil {
			res.Err = ctx.Err().Error()
			results[i] = res
			emit(res)
			continue
		}

		d, err := normalizeCredTestTarget(t, defaults)
		if err != nil {
			res.Err = err.Error()
			results[i] = res
			emit(res)
			continue
		}
		d.Username = username
		d.Password = password
		if err := tryOne(ctx, d, timeout); err != nil {
			res.Err = err.Error()
		} else {
			res.OK = true
		}
		results[i] = res
		emit(res)
	}

	emit(CredTestEvent{Type: "done", Total: total})
	return results
}

// normalizeCredTestTarget converts the small set of vendor aliases emitted by
// camera discovery into the canonical config vendor and fills in a missing
// control port. Unknown vendor values remain an explicit error: trying a
// private protocol against an arbitrary host can trigger lockouts or corrupt
// a non-camera service, so credential testing must never guess.
func normalizeCredTestTarget(t CredTestTarget, defaults config.Defaults) (config.Device, error) {
	vendor, ok := normalizeCredTestVendor(t.Vendor)
	if !ok {
		return config.Device{}, fmt.Errorf("không xác định được hãng, bỏ qua")
	}

	port := t.Port
	if port == 0 {
		// Server configuration is normally populated by config.Load. Keep this
		// helper safe for direct callers and tests that pass a zero Defaults.
		builtIn := config.Default().Defaults
		switch vendor {
		case config.VendorDahua:
			port = defaults.DahuaPort
			if port <= 0 {
				port = builtIn.DahuaPort
			}
		case config.VendorHikvision:
			port = defaults.HikvisionPort
			if port <= 0 {
				port = builtIn.HikvisionPort
			}
		}
	}
	if port <= 0 || port > 65535 {
		return config.Device{}, fmt.Errorf("cổng không hợp lệ: %d", port)
	}

	return config.Device{Host: t.IP, Port: port, Vendor: vendor}, nil
}

// normalizeCredTestVendor accepts canonical names plus the OEM aliases that
// discovery may report before its result reaches the browser/API boundary.
func normalizeCredTestVendor(raw string) (config.Vendor, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "dahua", "kbvision", "lechange", "imou", "lc":
		return config.VendorDahua, true
	case "hikvision", "hik":
		return config.VendorHikvision, true
	default:
		return "", false
	}
}

// tryOne opens a connection and issues one authenticated Probe call against
// d, returning nil only if both succeed.
func tryOne(ctx context.Context, d config.Device, timeout time.Duration) error {
	cam, err := camera.Open(ctx, d, timeout)
	if err != nil {
		return err
	}
	defer cam.Close()
	if _, err := cam.Probe(ctx); err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	return nil
}
