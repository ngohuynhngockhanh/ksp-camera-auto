package isapi

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// DeviceLocation resolves the device's own wall-clock UTC offset from
// GET /ISAPI/System/time, returned as a *time.Location callers can use to
// convert between device-local times (what the review UI sends/displays) and
// the UTC ContentMgmt's search/download interfaces require.
//
// It parses <localTime> (RFC3339, e.g. "2026-07-20T12:38:51+07:00") and uses
// ITS offset. <timeZone> (e.g. "CST-7:00:00") is deliberately ignored: it's
// POSIX TZ notation, whose sign is inverted from what it looks like at a
// glance (CST-7 means UTC+7, not UTC-7) — a confirmed live trap not worth
// parsing when localTime's own offset is unambiguous.
func (c *Client) DeviceLocation(ctx context.Context) (*time.Location, error) {
	body, err := c.do(ctx, http.MethodGet, "/ISAPI/System/time", nil)
	if err != nil {
		return nil, err
	}
	local := extractXMLString(body, "localTime")
	if local == "" {
		return nil, fmt.Errorf("isapi: /ISAPI/System/time: no <localTime> (body: %s)", truncate(body, 200))
	}
	t, err := time.Parse(time.RFC3339, local)
	if err != nil {
		return nil, fmt.Errorf("isapi: parse localTime %q: %w", local, err)
	}
	_, offset := t.Zone()
	return time.FixedZone("device", offset), nil
}
