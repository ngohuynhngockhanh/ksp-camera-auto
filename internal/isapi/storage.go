package isapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
)

// HDD is one physical storage device (hard disk / SD-in-a-bay) reported by
// an NVR's /ISAPI/ContentMgmt/Storage — the ISAPI analog of Dahua's
// storage.getDeviceAllInfo. Verified live against a DS-7632NXI-K2: capacity
// and freeSpace are reported in MEGABYTES (not bytes, unlike Dahua's
// TotalBytes/UsedBytes), so callers converting to dahua.StorageDevice must
// multiply by 1<<20.
type HDD struct {
	ID         int
	Name       string // hddName, e.g. "hdd1"
	Type       string // hddType, e.g. "SATA"
	Status     string // "ok" = healthy; any other value (unformatted/error/...) = not usable
	Property   string // "RW"/"RO" — read/write capability
	CapacityMB int64
	FreeMB     int64
}

// storageDoc mirrors the subset of ISAPI's /ISAPI/ContentMgmt/Storage
// document this package understands:
// <storage><hddList><hdd><id>1</id><hddName>hdd1</hddName>
// <hddType>SATA</hddType><status>ok</status><capacity>3815447</capacity>
// <freeSpace>0</freeSpace><property>RW</property></hdd>...</hddList></storage>
// (verified live against a DS-7632NXI-K2 NVR — note the root element is
// lowercase <storage>, unlike <System/time>'s capitalized roots). No XMLName
// field is declared on purpose: encoding/xml then decodes the root's children
// without asserting the root's name, so firmware that spells the root either
// way (<storage> or <Storage>) parses cleanly.
type storageDoc struct {
	HddList struct {
		HDD []struct {
			ID        int    `xml:"id"`
			HddName   string `xml:"hddName"`
			HddType   string `xml:"hddType"`
			Status    string `xml:"status"`
			Capacity  int64  `xml:"capacity"`
			FreeSpace int64  `xml:"freeSpace"`
			Property  string `xml:"property"`
		} `xml:"hdd"`
	} `xml:"hddList"`
}

// GetStorage reads every storage device (HDD bay) on an NVR via
// GET /ISAPI/ContentMgmt/Storage. A device with no bays populated (or a
// standalone camera that doesn't expose this resource) returns an empty
// slice rather than an error where the device replies with a body that
// simply has no <hdd> entries; a genuinely unreachable/erroring endpoint
// still surfaces as an error. This is deliberately the small, capped `do`
// path (like GetStreamChannel) — the document is tiny, unlike
// SearchTrack/DownloadStream's unbounded video payloads.
func (c *Client) GetStorage(ctx context.Context) ([]HDD, error) {
	body, err := c.do(ctx, http.MethodGet, "/ISAPI/ContentMgmt/Storage", nil)
	if err != nil {
		return nil, err
	}
	var doc storageDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("isapi: decode Storage: %w (body: %s)", err, truncate(body, 200))
	}
	out := make([]HDD, 0, len(doc.HddList.HDD))
	for _, h := range doc.HddList.HDD {
		out = append(out, HDD{
			ID:         h.ID,
			Name:       h.HddName,
			Type:       h.HddType,
			Status:     h.Status,
			Property:   h.Property,
			CapacityMB: h.Capacity,
			FreeMB:     h.FreeSpace,
		})
	}
	return out, nil
}
