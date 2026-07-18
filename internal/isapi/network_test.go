package isapi

import (
	"context"
	"strings"
	"testing"
)

// Real documents captured from a live Hikvision camera (firmware serving
// /ISAPI/System/Network/interfaces). Kept verbatim so the parser and the
// raw-XML mutator are tested against the exact bytes a device returns —
// including the IPv6 fields SetStaticIP must preserve and the repeated
// <ipAddress> tags (device IP vs. gateway/DNS) it must edit block-scoped.
const realInterfaceList = `<?xml version="1.0" encoding="UTF-8"?>
<NetworkInterfaceList version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<NetworkInterface version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<id>1</id>
<IPAddress version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<ipVersion>dual</ipVersion>
<addressingType>static</addressingType>
<ipAddress>192.168.1.2</ipAddress>
<subnetMask>255.255.255.0</subnetMask>
<ipv6Address>::</ipv6Address>
<bitMask>0</bitMask>
<DefaultGateway>
<ipAddress>192.168.1.1</ipAddress>
<ipv6Address>::</ipv6Address>
</DefaultGateway>
<PrimaryDNS>
<ipAddress>192.168.1.1</ipAddress>
</PrimaryDNS>
<SecondaryDNS>
<ipAddress>0.0.0.0</ipAddress>
</SecondaryDNS>
<Ipv6Mode>
<ipV6AddressingType>ra</ipV6AddressingType>
</Ipv6Mode>
</IPAddress>
<Link version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<MACAddress>08:3b:c1:9d:60:5e</MACAddress>
<MTU>1500</MTU>
</Link>
</NetworkInterface>
</NetworkInterfaceList>`

const realIPAddress = `<?xml version="1.0" encoding="UTF-8"?>
<IPAddress version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<ipVersion>dual</ipVersion>
<addressingType>static</addressingType>
<ipAddress>192.168.1.2</ipAddress>
<subnetMask>255.255.255.0</subnetMask>
<ipv6Address>::</ipv6Address>
<bitMask>0</bitMask>
<DefaultGateway>
<ipAddress>192.168.1.1</ipAddress>
<ipv6Address>::</ipv6Address>
</DefaultGateway>
<PrimaryDNS>
<ipAddress>192.168.1.1</ipAddress>
</PrimaryDNS>
<SecondaryDNS>
<ipAddress>0.0.0.0</ipAddress>
</SecondaryDNS>
<Ipv6Mode>
<ipV6AddressingType>ra</ipV6AddressingType>
</Ipv6Mode>
</IPAddress>`

const okResponseStatus = `<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<statusCode>1</statusCode><statusString>OK</statusString><subStatusCode>ok</subStatusCode>
</ResponseStatus>`

const rebootRequiredStatus = `<?xml version="1.0" encoding="UTF-8"?>
<ResponseStatus version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<statusCode>7</statusCode><statusString>Reboot Required</statusString><subStatusCode>rebootRequired</subStatusCode>
</ResponseStatus>`

// fakeNetTransport serves the captured network documents and records the last
// PUT body so tests can assert exactly what SetStaticIP sent to the device.
type fakeNetTransport struct {
	list        string
	ipAddress   string
	putResponse string // response the /ipAddress PUT returns; okResponseStatus if empty
	lastPut     []byte
	lastPath    string
	rebooted    bool
}

func (f *fakeNetTransport) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	switch {
	case method == "GET" && path == "/ISAPI/System/Network/interfaces":
		return []byte(f.list), nil
	case method == "GET" && strings.HasSuffix(path, "/ipAddress"):
		return []byte(f.ipAddress), nil
	case method == "PUT" && strings.HasSuffix(path, "/ipAddress"):
		f.lastPut = append([]byte(nil), body...)
		f.lastPath = path
		if f.putResponse != "" {
			return []byte(f.putResponse), nil
		}
		return []byte(okResponseStatus), nil
	case method == "PUT" && path == "/ISAPI/System/reboot":
		f.rebooted = true
		return []byte(okResponseStatus), nil
	}
	return nil, nil
}

func newNetClient() (*Client, *fakeNetTransport) {
	ft := &fakeNetTransport{list: realInterfaceList, ipAddress: realIPAddress}
	return NewWithTransport(ft), ft
}

func TestGetNetworkInterfaces(t *testing.T) {
	c, _ := newNetClient()
	ifaces, err := c.GetNetworkInterfaces(context.Background())
	if err != nil {
		t.Fatalf("GetNetworkInterfaces: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("want 1 interface, got %d", len(ifaces))
	}
	got := ifaces[0]
	if got.ID != "1" {
		t.Errorf("ID = %q, want 1", got.ID)
	}
	if got.DhcpEnable {
		t.Errorf("DhcpEnable = true, want false (addressingType static)")
	}
	if got.IPAddress != "192.168.1.2" {
		t.Errorf("IPAddress = %q, want 192.168.1.2", got.IPAddress)
	}
	if got.SubnetMask != "255.255.255.0" {
		t.Errorf("SubnetMask = %q", got.SubnetMask)
	}
	if got.DefaultGateway != "192.168.1.1" {
		t.Errorf("DefaultGateway = %q, want 192.168.1.1", got.DefaultGateway)
	}
	// Primary DNS kept; secondary (0.0.0.0 = unset) dropped.
	if len(got.DNS) != 1 || got.DNS[0] != "192.168.1.1" {
		t.Errorf("DNS = %v, want [192.168.1.1]", got.DNS)
	}
	if got.MAC != "08:3b:c1:9d:60:5e" {
		t.Errorf("MAC = %q", got.MAC)
	}
	if got.MTU != 1500 {
		t.Errorf("MTU = %d, want 1500", got.MTU)
	}
}

func TestSetStaticIP_ChangesDeviceIPOnly(t *testing.T) {
	c, ft := newNetClient()
	err := c.SetStaticIP(context.Background(), "1", false, "192.168.1.100", "255.255.255.0", "192.168.1.1", []string{"8.8.8.8"})
	if err != nil {
		t.Fatalf("SetStaticIP: %v", err)
	}
	put := string(ft.lastPut)
	if ft.lastPath != "/ISAPI/System/Network/interfaces/1/ipAddress" {
		t.Errorf("PUT path = %q", ft.lastPath)
	}
	// The device's own IP is the FIRST <ipAddress> — it must become .100 while
	// the gateway's <ipAddress> (inside <DefaultGateway>) stays .1.
	if !strings.Contains(put, "<ipAddress>192.168.1.100</ipAddress>") {
		t.Errorf("device IP not set to .100:\n%s", put)
	}
	if strings.Contains(put, "<ipAddress>192.168.1.2</ipAddress>") {
		t.Errorf("old device IP .2 still present:\n%s", put)
	}
	if !strings.Contains(put, "<PrimaryDNS>\n<ipAddress>8.8.8.8</ipAddress>") {
		t.Errorf("primary DNS not updated to 8.8.8.8:\n%s", put)
	}
	// addressingType stays static; IPv6 fields preserved untouched.
	if !strings.Contains(put, "<addressingType>static</addressingType>") {
		t.Errorf("addressingType not static:\n%s", put)
	}
	if !strings.Contains(put, "<Ipv6Mode>") || !strings.Contains(put, "<ipVersion>dual</ipVersion>") {
		t.Errorf("IPv6/ipVersion fields not preserved:\n%s", put)
	}
	// The gateway block's own address is unchanged (we passed the same .1).
	if !strings.Contains(put, "<DefaultGateway>\n<ipAddress>192.168.1.1</ipAddress>") {
		t.Errorf("gateway block malformed:\n%s", put)
	}
}

func TestSetStaticIP_RebootRequired(t *testing.T) {
	c, ft := newNetClient()
	ft.putResponse = rebootRequiredStatus
	// statusCode 7 "Reboot Required" is acceptance, not failure: the write
	// must succeed AND the device must be rebooted to apply the new IP.
	if err := c.SetStaticIP(context.Background(), "1", false, "192.168.1.100", "255.255.255.0", "192.168.1.1", nil); err != nil {
		t.Fatalf("SetStaticIP rebootRequired should succeed, got: %v", err)
	}
	if !ft.rebooted {
		t.Fatal("device was not rebooted to apply the staged IP change")
	}
}

func TestSetStaticIP_DHCP(t *testing.T) {
	c, ft := newNetClient()
	if err := c.SetStaticIP(context.Background(), "1", true, "", "", "", nil); err != nil {
		t.Fatalf("SetStaticIP dhcp: %v", err)
	}
	put := string(ft.lastPut)
	if !strings.Contains(put, "<addressingType>dynamic</addressingType>") {
		t.Errorf("DHCP did not set addressingType dynamic:\n%s", put)
	}
	// IP/mask left as the device had them — DHCP overrides at runtime.
	if !strings.Contains(put, "<ipAddress>192.168.1.2</ipAddress>") {
		t.Errorf("DHCP path should not rewrite the device IP:\n%s", put)
	}
}

func TestSetStaticIP_RejectsBadInput(t *testing.T) {
	c, ft := newNetClient()
	cases := []struct {
		name              string
		ip, mask, gateway string
		dns               []string
	}{
		{"bad ip", "192.168.1.999", "255.255.255.0", "192.168.1.1", nil},
		{"bad mask", "192.168.1.100", "not-a-mask", "192.168.1.1", nil},
		{"bad gateway", "192.168.1.100", "255.255.255.0", "gw", nil},
		{"bad dns", "192.168.1.100", "255.255.255.0", "192.168.1.1", []string{"8.8.8.x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ft.lastPut = nil
			if err := c.SetStaticIP(context.Background(), "1", false, tc.ip, tc.mask, tc.gateway, tc.dns); err == nil {
				t.Fatal("want validation error, got nil")
			}
			if ft.lastPut != nil {
				t.Fatal("nothing should be sent to the device on invalid input")
			}
		})
	}
}
