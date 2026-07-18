package isapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"time"
)

// NetworkInterface is one device network interface's IP configuration, as
// read from /ISAPI/System/Network/interfaces. Field names mirror the shape
// internal/camera maps into (and the Dahua NetworkConfig the web UI already
// renders): DhcpEnable/IPAddress/SubnetMask/DefaultGateway/DNS plus the
// read-only MAC/MTU shown for context.
type NetworkInterface struct {
	// ID is the device's own interface id ("1" = wired LAN, "2" = Wi-Fi on
	// wireless-capable models). SetStaticIP addresses one interface by this id.
	ID             string
	DhcpEnable     bool
	IPAddress      string
	SubnetMask     string
	DefaultGateway string
	DNS            []string
	MAC            string
	MTU            int
}

// ipAddressDoc mirrors the subset of the <IPAddress> element (returned both
// standalone at /interfaces/{id}/ipAddress and nested inside each
// <NetworkInterface>) this package reads back. Only used for GET decoding;
// writes go through raw-XML edits (see SetStaticIP) so IPv6 and other fields
// this struct omits survive the round-trip untouched.
type ipAddressDoc struct {
	AddressingType string `xml:"addressingType"`
	IPAddress      string `xml:"ipAddress"`
	SubnetMask     string `xml:"subnetMask"`
	DefaultGateway struct {
		IPAddress string `xml:"ipAddress"`
	} `xml:"DefaultGateway"`
	PrimaryDNS struct {
		IPAddress string `xml:"ipAddress"`
	} `xml:"PrimaryDNS"`
	SecondaryDNS struct {
		IPAddress string `xml:"ipAddress"`
	} `xml:"SecondaryDNS"`
}

// networkInterfaceList mirrors /ISAPI/System/Network/interfaces: one
// <NetworkInterface> per physical/virtual interface, each carrying its
// <IPAddress> block and a <Link> block with MAC/MTU.
type networkInterfaceList struct {
	XMLName    xml.Name `xml:"NetworkInterfaceList"`
	Interfaces []struct {
		ID        string       `xml:"id"`
		IPAddress ipAddressDoc `xml:"IPAddress"`
		Link      struct {
			MACAddress string `xml:"MACAddress"`
			MTU        int    `xml:"MTU"`
		} `xml:"Link"`
	} `xml:"NetworkInterface"`
}

// GetNetworkInterfaces reads every network interface's IP configuration in one
// GET (/ISAPI/System/Network/interfaces). addressingType "dynamic" maps to
// DhcpEnable=true. DNS entries equal to "0.0.0.0" (the device's "unset" slot
// value) are dropped so the UI shows empty fields rather than 0.0.0.0.
func (c *Client) GetNetworkInterfaces(ctx context.Context) ([]NetworkInterface, error) {
	body, err := c.do(ctx, http.MethodGet, "/ISAPI/System/Network/interfaces", nil)
	if err != nil {
		return nil, err
	}
	var list networkInterfaceList
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("isapi: decode network interfaces: %w (body: %s)", err, truncate(body, 200))
	}
	out := make([]NetworkInterface, 0, len(list.Interfaces))
	for _, ni := range list.Interfaces {
		iface := NetworkInterface{
			ID:             ni.ID,
			DhcpEnable:     ni.IPAddress.AddressingType == "dynamic",
			IPAddress:      ni.IPAddress.IPAddress,
			SubnetMask:     ni.IPAddress.SubnetMask,
			DefaultGateway: ni.IPAddress.DefaultGateway.IPAddress,
			MAC:            ni.Link.MACAddress,
			MTU:            ni.Link.MTU,
		}
		for _, dns := range []string{ni.IPAddress.PrimaryDNS.IPAddress, ni.IPAddress.SecondaryDNS.IPAddress} {
			if dns != "" && dns != "0.0.0.0" {
				iface.DNS = append(iface.DNS, dns)
			}
		}
		out = append(out, iface)
	}
	return out, nil
}

// validIPv4 reports whether s is a dotted-decimal IPv4 address. Every field
// SetStaticIP writes must pass this: a bad value written to a camera's only
// network interface can make it unreachable, so malformed input is rejected
// here before anything is sent rather than handed to the device.
func validIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// ipAddressPath is the ISAPI resource for one interface's IP configuration.
func ipAddressPath(ifaceID string) string {
	return "/ISAPI/System/Network/interfaces/" + ifaceID + "/ipAddress"
}

// SetStaticIP writes one interface's IP configuration via GET-modify-PUT on
// its /ipAddress resource. ip/mask/gateway/dns are validated as dotted-decimal
// IPv4 before anything is sent. When dhcpEnable is true, ip/mask/gateway/dns
// are ignored (the device's DHCP client supplies them) and only
// <addressingType> is flipped to "dynamic".
//
// The edit is done on the raw device document (not a re-marshalled struct):
// the real <IPAddress> document carries IPv6 fields (ipv6Address, Ipv6Mode,
// bitMask, ...) this package does not model, and Hikvision firmware rejects a
// PUT that drops them ("Invalid Content"). Each tag is confirmed present
// before it is edited, so a firmware whose schema differs fails loudly instead
// of silently no-op'ing a network change.
//
// NOTE: applying a new IP moves the device off its current address — the PUT's
// success envelope is the last thing that returns on the old connection;
// reading the config back afterwards must target the new IP.
func (c *Client) SetStaticIP(ctx context.Context, ifaceID string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	if !dhcpEnable {
		if !validIPv4(ip) {
			return fmt.Errorf("isapi: invalid IP address %q", ip)
		}
		if !validIPv4(mask) {
			return fmt.Errorf("isapi: invalid subnet mask %q", mask)
		}
		if gateway != "" && !validIPv4(gateway) {
			return fmt.Errorf("isapi: invalid gateway %q", gateway)
		}
		for _, d := range dns {
			if d != "" && !validIPv4(d) {
				return fmt.Errorf("isapi: invalid DNS server %q", d)
			}
		}
	}

	path := ipAddressPath(ifaceID)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if !hasXMLTag(raw, "addressingType") {
		return fmt.Errorf("isapi: interface %s: no <addressingType> in ipAddress document (firmware does not expose it)", ifaceID)
	}

	if dhcpEnable {
		raw = replaceXMLTag(raw, "addressingType", "dynamic")
	} else {
		if !hasXMLTag(raw, "ipAddress") || !hasXMLTag(raw, "subnetMask") {
			return fmt.Errorf("isapi: interface %s: no <ipAddress>/<subnetMask> in ipAddress document", ifaceID)
		}
		raw = replaceXMLTag(raw, "addressingType", "static")
		// The FIRST <ipAddress> in the document is the interface's own address;
		// gateway/DNS addresses live inside their own <DefaultGateway>/<PrimaryDNS>/
		// <SecondaryDNS> blocks and are edited block-scoped below.
		raw = replaceXMLTag(raw, "ipAddress", ip)
		raw = replaceXMLTag(raw, "subnetMask", mask)
		if gateway != "" {
			raw = replaceXMLTagInBlock(raw, "DefaultGateway", "ipAddress", gateway)
		}
		if len(dns) > 0 && dns[0] != "" {
			raw = replaceXMLTagInBlock(raw, "PrimaryDNS", "ipAddress", dns[0])
		}
		if len(dns) > 1 && dns[1] != "" {
			raw = replaceXMLTagInBlock(raw, "SecondaryDNS", "ipAddress", dns[1])
		}
	}

	resp, err := c.do(ctx, http.MethodPut, path, raw)
	if err != nil {
		return err
	}

	// Some Hikvision firmware applies an IP change live (statusCode 1); others
	// stage it and answer statusCode 7 "Reboot Required". The latter is NOT a
	// failure — the config was accepted — so we reboot the device to apply it.
	// Without the reboot the camera would keep its old address and the operator
	// would think the change silently failed.
	var rs responseStatus
	if uerr := xml.Unmarshal(resp, &rs); uerr != nil {
		return fmt.Errorf("isapi: PUT %s: decode ResponseStatus: %w (body: %s)", path, uerr, truncate(resp, 200))
	}
	switch {
	case rs.StatusCode == 1:
		return nil
	case rs.StatusCode == 7 || rs.SubStatusCode == "rebootRequired":
		if err := c.Reboot(ctx); err != nil {
			return fmt.Errorf("isapi: static IP accepted but reboot to apply it failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("isapi: PUT %s: statusCode=%d statusString=%q subStatusCode=%q", path, rs.StatusCode, rs.StatusString, rs.SubStatusCode)
	}
}

// Reboot restarts the device (PUT /ISAPI/System/reboot), retrying because a
// reboot issued immediately after a config write can hit a momentarily busy
// device (observed live: the first request errors, a retry a couple seconds
// later returns a clean statusCode 1). A clean OK envelope is definitive
// success. If every attempt fails at the transport level — which is also what a
// device already going down looks like — the last attempt is treated as
// best-effort "reboot started" rather than a hard error, since the config was
// already accepted. A well-formed non-OK ResponseStatus (endpoint unsupported /
// unauthorized) is returned as an error so the caller learns the reboot was
// rejected outright.
func (c *Client) Reboot(ctx context.Context) error {
	const attempts = 3
	var transportErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			// Let the device settle after the preceding config write / failed try.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		resp, err := c.do(ctx, http.MethodPut, "/ISAPI/System/reboot", nil)
		if err != nil {
			transportErr = err
			continue
		}
		// Clean response: honor its status (OK = rebooting; anything else = a
		// real rejection the caller should see).
		return checkResponseStatus(resp)
	}
	// All attempts errored at the transport level. That is indistinguishable
	// from the device having already begun rebooting, so don't fail the whole
	// static-IP operation over it; the config write itself already succeeded.
	_ = transportErr
	return nil
}
