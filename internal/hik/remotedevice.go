package hik

import (
	"context"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// GetRemoteDevices reads an NVR's channel-to-camera map via ISAPI
// (/ISAPI/ContentMgmt/InputProxy/channels) and maps it onto
// dahua.RemoteChannel so the vendor-neutral RemoteDeviceLister interface
// (the NVR scan flow) works unchanged for Hikvision. Channel is converted
// from ISAPI's 1-based InputProxyChannel id to the 0-based convention
// dahua.RemoteChannel already uses (id 1 -> Channel 0), matching Dahua's own
// 0-based RemoteDevice channel numbering — the scan UI's "nvrChannel" field
// then adds 1 back for display, identically for both vendors. Enable is
// always true: unlike Dahua's RemoteDevice config, ISAPI's
// InputProxyChannel document carries no per-channel enable flag (every
// listed channel is, by definition, a configured/proxied camera). Returned
// sorted by Channel (InputProxyChannel is already returned in id order by
// every device seen live, but this doesn't rely on that).
func (c *Client) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	chs, err := c.isapi.GetInputProxyChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dahua.RemoteChannel, 0, len(chs))
	for _, ch := range chs {
		out = append(out, dahua.RemoteChannel{
			Channel: ch.ID - 1,
			Address: ch.IPAddress,
			Port:    ch.ManagePort,
			Name:    ch.Name,
			Enable:  true,
		})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Channel > out[j].Channel; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out, nil
}
