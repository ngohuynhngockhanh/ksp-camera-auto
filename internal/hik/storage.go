package hik

import (
	"context"
	"regexp"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// GetStorageInfo reads an NVR's HDD bays via ISAPI
// (/ISAPI/ContentMgmt/Storage) and maps them onto dahua.StorageDevice/
// StorageDetail so the vendor-neutral StorageManager interface and
// dahua.HasUsableStorage (used by the NVR scan flow) work unchanged for
// Hikvision. Each HDD becomes one StorageDevice with a single Detail — Hik's
// ISAPI model has no separate "partition" concept the way Dahua's
// storage.getDeviceAllInfo does, so Path/Type both derive from the HDD
// itself.
//
// Field mapping (see isapi.HDD's own doc for the live-verified XML shape):
//   - CapacityMB/FreeMB are in MEGABYTES on the wire (unlike Dahua's
//     TotalBytes/UsedBytes, already bytes) — converted here via <<20.
//   - Type is derived from HDD.Property ("RW"/"RO"), NOT HDD.Type (which is
//     the physical bay kind, e.g. "SATA") — this is a deliberate deviation
//     from a naive same-named-field mapping: dahua.HasUsableStorage gates on
//     StorageDetail.Type == "ReadWrite", the same vocabulary Dahua's own
//     storage.getDeviceAllInfo uses for its ReadWrite/ReadOnly partition
//     type. Mapping hddType ("SATA") into that field instead would make
//     HasUsableStorage permanently false for every Hik NVR, even a healthy
//     one with recordings — silently forcing every camera onto NVR fallback
//     regardless of its actual storage state. Property carries the
//     read/write-capability signal Dahua's Type field actually encodes.
//   - IsError is true for any status other than the healthy "ok" (Hikvision
//     doesn't use Dahua's separate error/needFormat flags; status is the one
//     signal, e.g. "abnormal", "error").
//   - IsNeedFormat additionally flags an unformatted/uninitialized bay, so a
//     freshly-inserted-but-blank disk is distinguishable from a hard error —
//     both still fail HasUsableStorage's Type=="ReadWrite" check today, but
//     callers building UI messaging benefit from the distinction.
//
// A device with no HDD bays (e.g. a standalone camera, or an NVR whose
// Storage resource returns an empty hddList) returns an empty slice, which
// is exactly the signal dahua.HasUsableStorage needs to report NoStorage and
// trigger the NVR-fallback path.
func (c *Client) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	hdds, err := c.isapi.GetStorage(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dahua.StorageDevice, 0, len(hdds))
	for _, h := range hdds {
		healthy := h.Status == "ok"
		out = append(out, dahua.StorageDevice{
			Name:  h.Name,
			State: h.Status,
			Details: []dahua.StorageDetail{{
				Path:         h.Name,
				Type:         partitionType(h.Property),
				TotalBytes:   h.CapacityMB * (1 << 20),
				UsedBytes:    (h.CapacityMB - h.FreeMB) * (1 << 20),
				IsError:      !healthy,
				IsNeedFormat: !healthy && needFormatStatus.MatchString(h.Status),
			}},
		})
	}
	return out, nil
}

// partitionType maps ISAPI's <property> ("RW"/"RO") to Dahua's own
// ReadWrite/ReadOnly vocabulary for dahua.StorageDetail.Type. Any other
// value passes through unchanged (defensive; not seen live) so it's at
// least visible for debugging rather than silently coerced.
func partitionType(property string) string {
	switch property {
	case "RW":
		return "ReadWrite"
	case "RO":
		return "ReadOnly"
	default:
		return property
	}
}

// needFormatStatus matches the ISAPI status words that mean "present but
// unformatted/uninitialized" as opposed to a hardware error — not verified
// against every firmware's exact vocabulary, so it's deliberately permissive
// (case-insensitive substring) rather than an exact-match list.
var needFormatStatus = regexp.MustCompile(`(?i)unformat|uninitial`)
