package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// Action represents an extensible scheduled operation executable on inventory devices.
type Action interface {
	// Type returns the unique identifier for this action type.
	Type() string
	// Validate checks that the action parameters are well-formed.
	Validate() error
	// Execute executes the action against inventory devices.
	Execute(ctx context.Context, inv *config.Inventory, timeout time.Duration) (string, error)
	// Summary returns a human-readable one-line description of the action.
	Summary() string
}

// TargetedAction is an optional interface for actions that operate on explicit inventory targets.
type TargetedAction interface {
	Action
	TargetDevices() []string
}

// ReconcilableAction is an optional interface for idempotent state-setting actions (e.g. Day/Night color mode)
// that can be safely reconciled/caught up upon system boot, clock jump, or power restoration.
type ReconcilableAction interface {
	Action
	// StateGroupKeys returns partition keys representing the managed resource and parameter
	// (e.g. "cam-202:ch0:picture_day_night"). If multiple scheduled jobs share a key,
	// only the most recent scheduled slot that should currently be active is applied.
	StateGroupKeys(inv *config.Inventory) []string
}

var (
	deviceLocksMu sync.Mutex
	deviceLocks   = make(map[string]*sync.Mutex)
)

func getDeviceMutex(deviceID string) *sync.Mutex {
	deviceLocksMu.Lock()
	defer deviceLocksMu.Unlock()
	m, ok := deviceLocks[deviceID]
	if !ok {
		m = &sync.Mutex{}
		deviceLocks[deviceID] = m
	}
	return m
}

// ActionFactory produces a fresh Action instance for unmarshaling.
type ActionFactory func() Action

var (
	registryMu     sync.RWMutex
	actionRegistry = make(map[string]ActionFactory)

	openerMu     sync.RWMutex
	cameraOpener = camera.Open
)

// RegisterAction registers an ActionFactory for the given actionType.
func RegisterAction(actionType string, factory ActionFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	actionRegistry[strings.ToLower(strings.TrimSpace(actionType))] = factory
}

// RegisteredActions returns a sorted list of registered action type names.
func RegisteredActions() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(actionRegistry))
	for k := range actionRegistry {
		out = append(out, k)
	}
	return out
}

// ParseAction creates and populates an Action instance from raw JSON.
// If actionType is blank, it attempts to infer it from the JSON payload ("type" or "actionType").
func ParseAction(actionType string, raw json.RawMessage) (Action, error) {
	cleanType := strings.ToLower(strings.TrimSpace(actionType))
	if cleanType == "" && len(raw) > 0 {
		var probe struct {
			Type       string `json:"type"`
			ActionType string `json:"actionType"`
		}
		_ = json.Unmarshal(raw, &probe)
		if probe.Type != "" {
			cleanType = strings.ToLower(strings.TrimSpace(probe.Type))
		} else if probe.ActionType != "" {
			cleanType = strings.ToLower(strings.TrimSpace(probe.ActionType))
		}
	}

	if cleanType == "" {
		return nil, errors.New("scheduler: action type is required")
	}

	registryMu.RLock()
	factory, ok := actionRegistry[cleanType]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("scheduler: unknown action type %q", cleanType)
	}

	act := factory()
	if len(raw) > 0 && string(raw) != "null" {
		// Check if payload is wrapped in {"type": "...", "params": {...}}
		var wrapped struct {
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Params) > 0 && string(wrapped.Params) != "null" {
			raw = wrapped.Params
		}
		if err := json.Unmarshal(raw, act); err != nil {
			return nil, fmt.Errorf("scheduler: unmarshal action %q: %w", cleanType, err)
		}
	}

	if err := act.Validate(); err != nil {
		return nil, fmt.Errorf("scheduler: invalid action %q: %w", cleanType, err)
	}

	return act, nil
}

// SetCameraOpener overrides camera.Open (useful in tests).
func SetCameraOpener(fn func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error)) {
	openerMu.Lock()
	defer openerMu.Unlock()
	if fn == nil {
		cameraOpener = camera.Open
	} else {
		cameraOpener = fn
	}
}

// ResetCameraOpener restores default camera.Open.
func ResetCameraOpener() {
	SetCameraOpener(nil)
}

func getCameraOpener() func(ctx context.Context, d config.Device, timeout time.Duration) (camera.Camera, error) {
	openerMu.RLock()
	defer openerMu.RUnlock()
	return cameraOpener
}

// resolveDevice finds a device in inventory by exact ID, Name, Host, or Host:Port.
func resolveDevice(inv *config.Inventory, target string) (config.Device, bool) {
	if inv == nil {
		return config.Device{}, false
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return config.Device{}, false
	}

	// 1. Exact ID
	if d, ok := inv.Get(target); ok {
		return d, true
	}

	devices := inv.List()

	// 2. Exact Name (case-insensitive) or exact Host
	for _, d := range devices {
		if strings.EqualFold(d.Name, target) || strings.EqualFold(d.Host, target) || strings.EqualFold(d.ID, target) {
			return d, true
		}
	}

	// 3. Compare host part if target includes port
	targetHost := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		targetHost = h
	}
	for _, d := range devices {
		if strings.EqualFold(d.Host, targetHost) {
			return d, true
		}
	}

	// 4. Substring / normalized match (e.g. target "Cam 2" vs "Camera02")
	targetNorm := normalizeDevName(target)
	for _, d := range devices {
		if normalizeDevName(d.Name) == targetNorm && targetNorm != "" {
			return d, true
		}
	}

	return config.Device{}, false
}

func normalizeDevName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "camera", "cam")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "0", "")
	return s
}
