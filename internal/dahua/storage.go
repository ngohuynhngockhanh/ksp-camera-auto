package dahua

import (
	"encoding/json"
	"fmt"
)

// StorageDetail is one partition/mount inside a storage device (a Dahua SD card
// exposes a single ReadWrite partition mounted at /mnt/sd).
type StorageDetail struct {
	Path         string `json:"path"`
	Type         string `json:"type"` // ReadWrite / ReadOnly
	TotalBytes   int64  `json:"totalBytes"`
	UsedBytes    int64  `json:"usedBytes"`
	IsError      bool   `json:"isError"`
	IsNeedFormat bool   `json:"isNeedFormat"`
}

// StorageDevice is one physical storage device, e.g. the SD card /dev/mmc0.
type StorageDevice struct {
	Name    string          `json:"name"`
	State   string          `json:"state"` // Success / ... (device's own status word)
	Details []StorageDetail `json:"details"`
}

// GetStorageInfo reads every storage device (SD card / eMMC) via the DVRIP RPC
// storage.getDeviceAllInfo. An empty slice means no removable storage is
// present (no card inserted). Confirmed live against a DH-C5A: the response is
// {"info":[{"Name":"/dev/mmc0","State":"Success","Detail":[{"Path":"/mnt/sd",
// "Type":"ReadWrite","TotalBytes":...,"UsedBytes":...,"IsError":...,
// "IsNeedFormat":...}]}]}.
func (c *Client) GetStorageInfo() ([]StorageDevice, error) {
	resp, err := c.Call("storage.getDeviceAllInfo", nil)
	if err != nil {
		return nil, err
	}
	if !resp.ok() {
		return nil, fmt.Errorf("storage.getDeviceAllInfo failed: %s", respErr(resp))
	}
	var p struct {
		Info []struct {
			Name   string `json:"Name"`
			State  string `json:"State"`
			Detail []struct {
				Path         string `json:"Path"`
				Type         string `json:"Type"`
				TotalBytes   int64  `json:"TotalBytes"`
				UsedBytes    int64  `json:"UsedBytes"`
				IsError      bool   `json:"IsError"`
				IsNeedFormat bool   `json:"IsNeedFormat"`
			} `json:"Detail"`
		} `json:"info"`
	}
	if err := json.Unmarshal(resp.Params, &p); err != nil {
		return nil, fmt.Errorf("storage.getDeviceAllInfo: decode: %w", err)
	}
	out := make([]StorageDevice, 0, len(p.Info))
	for _, d := range p.Info {
		dev := StorageDevice{Name: d.Name, State: d.State}
		for _, dt := range d.Detail {
			dev.Details = append(dev.Details, StorageDetail{
				Path:         dt.Path,
				Type:         dt.Type,
				TotalBytes:   dt.TotalBytes,
				UsedBytes:    dt.UsedBytes,
				IsError:      dt.IsError,
				IsNeedFormat: dt.IsNeedFormat,
			})
		}
		out = append(out, dev)
	}
	return out, nil
}

// FormatStorage formats one storage device by name (e.g. "/dev/mmc0", the Name
// from GetStorageInfo). THIS ERASES ALL DATA on the device. It is a DVRIP RPC
// (storage.format); the device runs the format and may briefly drop the DVRIP
// session, so callers should re-read GetStorageInfo after a short delay rather
// than trust an immediate read-back.
func (c *Client) FormatStorage(name string) error {
	if name == "" {
		return fmt.Errorf("dahua: FormatStorage requires a device name")
	}
	resp, err := c.Call("storage.format", map[string]any{"name": name})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("storage.format %s failed: %s", name, respErr(resp))
	}
	return nil
}
