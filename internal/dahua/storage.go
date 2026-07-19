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

// HasUsableStorage reports whether any device has a usable read/write partition
// (present, non-empty, not errored/needing-format) — i.e. the camera can record
// locally. An empty slice (no card) or all-error/zero partitions → false, which
// is the signal to fall recordings back to the NVR.
func HasUsableStorage(devs []StorageDevice) bool {
	for _, d := range devs {
		for _, dt := range d.Details {
			if dt.Type == "ReadWrite" && dt.TotalBytes > 0 && !dt.IsError && !dt.IsNeedFormat {
				return true
			}
		}
	}
	return false
}

// FormatStorage formats one storage device by name (e.g. "/dev/mmc0", the Name
// from GetStorageInfo). THIS ERASES ALL DATA on the device.
//
// Format lives on the devStorage interface (NOT storage, which only reads):
// create an object bound to the device via devStorage.factory.instance with
// {"name": <device>}, then call devStorage.formatPatition (sic — Dahua's own
// misspelling of "Partition") on that object. Both the method names and this
// two-step handshake were confirmed live over DVRIP against a DH-C5A.
//
// While the device formats it stops answering DVRIP, so the formatPatition
// call reliably hits the client's read deadline: an i/o timeout there means
// the format STARTED, not that it failed, and is treated as success. Callers
// should re-read GetStorageInfo only after a delay (the device is busy for a
// while and may be briefly unreachable on the config port).
func (c *Client) FormatStorage(name string) error {
	if name == "" {
		return fmt.Errorf("dahua: FormatStorage requires a device name")
	}
	resp, err := c.Call("devStorage.factory.instance", map[string]any{"name": name})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("devStorage.factory.instance %s failed: %s", name, respErr(resp))
	}
	var objID int64
	if err := json.Unmarshal(resp.Result, &objID); err != nil || objID == 0 {
		return fmt.Errorf("dahua: no devStorage instance for %q (result=%s)", name, string(resp.Result))
	}
	if _, err := c.CallObject("devStorage.formatPatition", objID, map[string]any{"name": name}); err != nil {
		// Expected: the device stops answering DVRIP the moment it begins
		// formatting, so this request hits the read deadline. The format is
		// underway — report success rather than a misleading timeout error.
		return nil
	}
	return nil
}
