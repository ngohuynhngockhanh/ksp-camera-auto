package isapi

import (
	"context"
	"net/http/httptest"
	"testing"
)

// TestGetInputProxyChannelsParsesLiveShape verifies the live-verified
// DS-7632NXI-K2 shape: id/name/ipAddress/managePortNo, multiple channels.
func TestGetInputProxyChannelsParsesLiveShape(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	fake.inputProxyBody = `<?xml version="1.0" encoding="UTF-8"?>
<InputProxyChannelList version="2.0" xmlns="http://www.hikvision.com/ver20/XMLSchema">
<InputProxyChannel>
<id>1</id>
<name>BAN 1</name>
<sourceInputPortDescriptor>
<addressingFormatType>ipaddress</addressingFormatType>
<ipAddress>192.168.1.220</ipAddress>
<managePortNo>8000</managePortNo>
</sourceInputPortDescriptor>
</InputProxyChannel>
<InputProxyChannel>
<id>2</id>
<name>BAN 2</name>
<sourceInputPortDescriptor>
<addressingFormatType>ipaddress</addressingFormatType>
<ipAddress>192.168.1.221</ipAddress>
<managePortNo>8000</managePortNo>
</sourceInputPortDescriptor>
</InputProxyChannel>
</InputProxyChannelList>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	chs, err := c.GetInputProxyChannels(context.Background())
	if err != nil {
		t.Fatalf("GetInputProxyChannels: %v", err)
	}
	if len(chs) != 2 {
		t.Fatalf("got %d channels, want 2: %+v", len(chs), chs)
	}
	if chs[0].ID != 1 || chs[0].Name != "BAN 1" || chs[0].IPAddress != "192.168.1.220" || chs[0].ManagePort != 8000 {
		t.Fatalf("unexpected channel[0]: %+v", chs[0])
	}
	if chs[1].ID != 2 || chs[1].Name != "BAN 2" || chs[1].IPAddress != "192.168.1.221" {
		t.Fatalf("unexpected channel[1]: %+v", chs[1])
	}
}

// TestGetInputProxyChannelsMissingManagePort covers firmware that omits
// managePortNo — ManagePort should default to 0 (callers treat that as
// "unknown", not a literal port), not error.
func TestGetInputProxyChannelsMissingManagePort(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	fake.inputProxyBody = `<?xml version="1.0" encoding="UTF-8"?>
<InputProxyChannelList>
<InputProxyChannel>
<id>3</id>
<name>BAN 3</name>
<sourceInputPortDescriptor>
<addressingFormatType>ipaddress</addressingFormatType>
<ipAddress>192.168.1.222</ipAddress>
</sourceInputPortDescriptor>
</InputProxyChannel>
</InputProxyChannelList>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	chs, err := c.GetInputProxyChannels(context.Background())
	if err != nil {
		t.Fatalf("GetInputProxyChannels: %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("got %d channels, want 1: %+v", len(chs), chs)
	}
	if chs[0].ManagePort != 0 {
		t.Fatalf("ManagePort = %d, want 0 (missing managePortNo)", chs[0].ManagePort)
	}
	if chs[0].IPAddress != "192.168.1.222" {
		t.Fatalf("IPAddress = %q, want 192.168.1.222", chs[0].IPAddress)
	}
}

// TestGetInputProxyChannelsEmpty covers a device with no proxied channels
// configured — no error, just an empty slice.
func TestGetInputProxyChannelsEmpty(t *testing.T) {
	fake := newFakeContentMgmtServer("admin", "duyanh68A")
	fake.inputProxyBody = `<?xml version="1.0" encoding="UTF-8"?><InputProxyChannelList></InputProxyChannelList>`
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newContentMgmtTestClient(t, srv, "admin", "duyanh68A")

	chs, err := c.GetInputProxyChannels(context.Background())
	if err != nil {
		t.Fatalf("GetInputProxyChannels: %v", err)
	}
	if len(chs) != 0 {
		t.Fatalf("got %d channels, want 0", len(chs))
	}
}
