package isapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
)

// InputChannel is one NVR channel's connected (proxied) IP camera, from
// GET /ISAPI/ContentMgmt/InputProxy/channels — the ISAPI analog of Dahua's
// RemoteDevice config. Verified live against a DS-7632NXI-K2 (23 channels,
// e.g. id 1 "BAN 1" 192.168.1.220).
type InputChannel struct {
	ID         int
	Name       string
	IPAddress  string
	ManagePort int // sourceInputPortDescriptor/managePortNo; 0 if the device omits it
}

// inputProxyChannelListDoc mirrors the InputProxyChannelList document:
// <InputProxyChannelList><InputProxyChannel><id>1</id><name>BAN 1</name>
// <sourceInputPortDescriptor><addressingFormatType>ipaddress</addressingFormatType>
// <ipAddress>192.168.1.220</ipAddress><managePortNo>8000</managePortNo>
// </sourceInputPortDescriptor></InputProxyChannel>...</InputProxyChannelList>.
type inputProxyChannelListDoc struct {
	XMLName  xml.Name `xml:"InputProxyChannelList"`
	Channels []struct {
		ID     string `xml:"id"`
		Name   string `xml:"name"`
		Source struct {
			IPAddress    string `xml:"ipAddress"`
			ManagePortNo int    `xml:"managePortNo"`
		} `xml:"sourceInputPortDescriptor"`
	} `xml:"InputProxyChannel"`
}

// GetInputProxyChannels reads an NVR's channel-to-camera map via
// GET /ISAPI/ContentMgmt/InputProxy/channels — one GET returns every
// channel, unlike Dahua's map-shaped RemoteDevice config which is fetched in
// the same single-call fashion. A channel entry with a non-integer <id> is
// skipped (defensive; not seen live). A missing <managePortNo> element
// unmarshals to ManagePort 0, which callers should treat as "unknown", not a
// literal port 0.
func (c *Client) GetInputProxyChannels(ctx context.Context) ([]InputChannel, error) {
	body, err := c.do(ctx, http.MethodGet, inputProxyChannelsPath(), nil)
	if err != nil {
		return nil, err
	}
	var doc inputProxyChannelListDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("isapi: decode InputProxyChannelList: %w (body: %s)", err, truncate(body, 200))
	}
	out := make([]InputChannel, 0, len(doc.Channels))
	for _, ch := range doc.Channels {
		id, err := strconv.Atoi(ch.ID)
		if err != nil {
			continue
		}
		out = append(out, InputChannel{
			ID:         id,
			Name:       ch.Name,
			IPAddress:  ch.Source.IPAddress,
			ManagePort: ch.Source.ManagePortNo,
		})
	}
	return out, nil
}
