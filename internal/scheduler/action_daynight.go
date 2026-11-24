package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

const (
	// ActionTypePictureDayNight is the action type string for PictureDayNightAction.
	ActionTypePictureDayNight = "picture_day_night"

	// DayNightColor values per Dahua / camera specification:
	// 0 = Full Color (Always Color)
	// 1 = Auto (Brightness-based Day/Night switch)
	// 2 = B&W (Always Black & White)
	DayNightFullColor  = 0
	DayNightAuto       = 1
	DayNightBlackWhite = 2
)

func init() {
	RegisterAction(ActionTypePictureDayNight, func() Action {
		return &PictureDayNightAction{}
	})
	// Convenience aliases
	RegisterAction("picture_daynight", func() Action {
		return &PictureDayNightAction{}
	})
	RegisterAction("day_night", func() Action {
		return &PictureDayNightAction{}
	})
}

// PictureDayNightAction sets the Day/Night color mode on target cameras and verifies via read-back.
type PictureDayNightAction struct {
	DeviceIDs       []string `json:"deviceIds"`
	Channel         int      `json:"channel"`                   // 0-based channel (default 0)
	DayNightColor   int      `json:"dayNightColor"`             // 0=Full Color, 1=Auto, 2=B&W
	SupplementLight string   `json:"supplementLight,omitempty"` // "auto", "keep", "off", "InfraredMode", "WhiteLightMode", "AIMixLightMode"
}

func (a *PictureDayNightAction) Type() string {
	return ActionTypePictureDayNight
}

func (a *PictureDayNightAction) Validate() error {
	if len(a.DeviceIDs) == 0 {
		return errors.New("at least one device ID is required")
	}
	for _, id := range a.DeviceIDs {
		if strings.TrimSpace(id) == "" {
			return errors.New("device ID cannot be blank")
		}
	}
	if a.Channel < 0 {
		return errors.New("channel cannot be negative")
	}
	if a.DayNightColor < 0 || a.DayNightColor > 2 {
		return fmt.Errorf("invalid dayNightColor: %d (must be 0=Full Color, 1=Auto, 2=B&W)", a.DayNightColor)
	}
	if sl := strings.TrimSpace(a.SupplementLight); sl != "" {
		switch strings.ToLower(sl) {
		case "auto", "keep", "off", "infraredmode", "whitelightmode", "aimixlightmode":
		default:
			return fmt.Errorf("invalid supplementLight: %q (must be auto, keep, off, InfraredMode, WhiteLightMode, AIMixLightMode)", a.SupplementLight)
		}
	}
	return nil
}

// DayNightColorName returns the friendly name for a DayNightColor value.
func DayNightColorName(mode int) string {
	switch mode {
	case DayNightFullColor:
		return "Full-Color"
	case DayNightAuto:
		return "Auto"
	case DayNightBlackWhite:
		return "B&W"
	default:
		return fmt.Sprintf("Mode(%d)", mode)
	}
}

func (a *PictureDayNightAction) Summary() string {
	modeStr := DayNightColorName(a.DayNightColor)
	return fmt.Sprintf("Set Day/Night %s (%d) on %d camera(s) ch %d", modeStr, a.DayNightColor, len(a.DeviceIDs), a.Channel)
}

// TargetDevices returns the list of target device IDs.
func (a *PictureDayNightAction) TargetDevices() []string {
	return a.DeviceIDs
}

// StateGroupKeys returns distinct keys for the resource states managed by this action.
func (a *PictureDayNightAction) StateGroupKeys(inv *config.Inventory) []string {
	var keys []string
	for _, target := range a.DeviceIDs {
		target = strings.TrimSpace(target)
		devID := target
		if inv != nil {
			if dev, found := resolveDevice(inv, target); found {
				devID = dev.ID
			}
		}
		keys = append(keys, fmt.Sprintf("%s:ch%d:picture_day_night", devID, a.Channel))
	}
	return keys
}

func (a *PictureDayNightAction) Execute(ctx context.Context, inv *config.Inventory, timeout time.Duration) (string, error) {
	if inv == nil {
		return "", errors.New("scheduler: inventory is nil")
	}
	if err := a.Validate(); err != nil {
		return "", err
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	perCamTimeout := timeout
	if len(a.DeviceIDs) > 0 {
		perCamTimeout = timeout / time.Duration(len(a.DeviceIDs))
		if perCamTimeout < 5*time.Second {
			perCamTimeout = 5 * time.Second
		}
	}

	opener := getCameraOpener()
	var succeeded []string
	var errs []error

	for _, target := range a.DeviceIDs {
		target = strings.TrimSpace(target)
		dev, found := resolveDevice(inv, target)
		if !found {
			errs = append(errs, fmt.Errorf("device %q not found in inventory", target))
			continue
		}

		devLabel := dev.Name
		if devLabel == "" {
			devLabel = dev.ID
		}

		err := func() error {
			lock := getDeviceMutex(dev.ID)
			lock.Lock()
			defer lock.Unlock()

			cctx, cancel := context.WithTimeout(ctx, perCamTimeout)
			defer cancel()
			cam, err := opener(cctx, dev, timeout)
			if err != nil {
				return fmt.Errorf("failed to open camera %s (%s): %w", devLabel, dev.Addr(), err)
			}
			defer cam.Close()

			ps, ok := cam.(camera.PictureSettings)
			if !ok {
				return fmt.Errorf("camera %s (%s) does not support PictureSettings", devLabel, dev.Vendor)
			}

			// Apply DayNightColor and optional SupplementLight
			optionsChanges := map[string]any{
				"DayNightColor": a.DayNightColor,
			}
			if strings.TrimSpace(a.SupplementLight) != "" {
				optionsChanges["_SupplementLight"] = strings.TrimSpace(a.SupplementLight)
			}
			_, postOptions, err := ps.SetPicture(cctx, a.Channel, nil, optionsChanges)
			if err != nil {
				return fmt.Errorf("camera %s set picture failed: %w", devLabel, err)
			}

			// Read-back verification
			actualDNC, ok := extractDayNightColor(postOptions)
			if !ok {
				// Fallback to explicit GetPicture read-back
				_, getOptions, err := ps.GetPicture(cctx, a.Channel)
				if err == nil {
					actualDNC, ok = extractDayNightColor(getOptions)
				}
			}

			if !ok {
				return fmt.Errorf("camera %s: DayNightColor missing in read-back verification", devLabel)
			}

			if actualDNC != a.DayNightColor {
				return fmt.Errorf("camera %s: read-back verification mismatch: expected DayNightColor=%d (%s), got %d (%s)",
					devLabel, a.DayNightColor, DayNightColorName(a.DayNightColor), actualDNC, DayNightColorName(actualDNC))
			}

			return nil
		}()

		if err != nil {
			errs = append(errs, err)
		} else {
			succeeded = append(succeeded, devLabel)
		}
	}

	modeName := DayNightColorName(a.DayNightColor)
	if len(errs) > 0 {
		var combinedErr error
		if len(errs) == 1 {
			combinedErr = errs[0]
		} else {
			combinedErr = errors.Join(errs...)
		}
		summary := fmt.Sprintf("Set %s on %d/%d camera(s) (success: [%s]; failed: %d)",
			modeName, len(succeeded), len(a.DeviceIDs), strings.Join(succeeded, ", "), len(errs))
		return summary, combinedErr
	}

	summary := fmt.Sprintf("Successfully set %s (DayNightColor=%d) on %d camera(s): [%s] (read-back verified)",
		modeName, a.DayNightColor, len(succeeded), strings.Join(succeeded, ", "))
	return summary, nil
}

// extractDayNightColor extracts integer DayNightColor from a map.
func extractDayNightColor(opts map[string]any) (int, bool) {
	if opts == nil {
		return 0, false
	}
	if v, exists := opts["DayNightColor"]; exists {
		return toIntVal(v)
	}
	// Check nested NightOptions or NormalOptions if top-level was absent
	for _, subKey := range []string{"NightOptions", "NormalOptions"} {
		if sub, ok := opts[subKey].(map[string]any); ok {
			if v, exists := sub["DayNightColor"]; exists {
				return toIntVal(v)
			}
		}
	}
	return 0, false
}

func toIntVal(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case int32:
		return int(n), true
	case float64:
		return int(n), true
	case float32:
		return int(n), true
	case string:
		val, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return val, true
		}
	}
	return 0, false
}
