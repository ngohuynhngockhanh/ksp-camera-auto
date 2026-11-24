package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

const (
	// ActionTypeReboot is the action type string for RebootAction.
	ActionTypeReboot = "reboot"
)

func init() {
	RegisterAction(ActionTypeReboot, func() Action {
		return &RebootAction{}
	})
	RegisterAction("restart", func() Action {
		return &RebootAction{}
	})
}

// RebootAction issues a remote reboot command to target devices.
type RebootAction struct {
	DeviceIDs []string `json:"deviceIds"`
}

func (a *RebootAction) Type() string {
	return ActionTypeReboot
}

func (a *RebootAction) Validate() error {
	if len(a.DeviceIDs) == 0 {
		return errors.New("at least one device ID is required")
	}
	for _, id := range a.DeviceIDs {
		if strings.TrimSpace(id) == "" {
			return errors.New("device ID cannot be blank")
		}
	}
	return nil
}

func (a *RebootAction) Summary() string {
	return fmt.Sprintf("Reboot %d camera(s)", len(a.DeviceIDs))
}

// TargetDevices returns the list of target device IDs.
func (a *RebootAction) TargetDevices() []string {
	return a.DeviceIDs
}

func (a *RebootAction) Execute(ctx context.Context, inv *config.Inventory, timeout time.Duration) (string, error) {
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

			rb, ok := cam.(camera.Rebooter)
			if !ok {
				return fmt.Errorf("camera %s (%s) does not support Reboot", devLabel, dev.Vendor)
			}

			if err := rb.Reboot(cctx); err != nil {
				return fmt.Errorf("camera %s reboot failed: %w", devLabel, err)
			}
			return nil
		}()

		if err != nil {
			errs = append(errs, err)
		} else {
			succeeded = append(succeeded, devLabel)
		}
	}

	if len(errs) > 0 {
		var combinedErr error
		if len(errs) == 1 {
			combinedErr = errs[0]
		} else {
			combinedErr = errors.Join(errs...)
		}
		summary := fmt.Sprintf("Rebooted %d/%d camera(s) (success: [%s]; failed: %d)",
			len(succeeded), len(a.DeviceIDs), strings.Join(succeeded, ", "), len(errs))
		return summary, combinedErr
	}

	summary := fmt.Sprintf("Successfully rebooted %d camera(s): [%s]", len(succeeded), strings.Join(succeeded, ", "))
	return summary, nil
}
