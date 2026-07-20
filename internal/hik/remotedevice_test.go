package hik

import (
	"context"
	"net/http/httptest"
	"testing"
)

// TestGetRemoteDevicesConvertsToZeroBasedChannel verifies the live-verified
// DS-7632NXI-K2 InputProxy shape maps to dahua.RemoteChannel with the
// 0-based Channel numbering dahua.RemoteChannel already uses (id 1 -> 0),
// Enable always true (ISAPI carries no per-channel enable flag), and sorted
// order.
func TestGetRemoteDevicesConvertsToZeroBasedChannel(t *testing.T) {
	fake := &hikStorageChannelsServer{inputProxyBody: `<?xml version="1.0"?>
<InputProxyChannelList>
<InputProxyChannel><id>2</id><name>BAN 2</name><sourceInputPortDescriptor><ipAddress>192.168.1.221</ipAddress><managePortNo>8000</managePortNo></sourceInputPortDescriptor></InputProxyChannel>
<InputProxyChannel><id>1</id><name>BAN 1</name><sourceInputPortDescriptor><ipAddress>192.168.1.220</ipAddress><managePortNo>8000</managePortNo></sourceInputPortDescriptor></InputProxyChannel>
</InputProxyChannelList>`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newHikStorageTestClient(t, srv)

	remotes, err := c.GetRemoteDevices(context.Background())
	if err != nil {
		t.Fatalf("GetRemoteDevices: %v", err)
	}
	if len(remotes) != 2 {
		t.Fatalf("got %d remotes, want 2: %+v", len(remotes), remotes)
	}
	// Sorted by Channel regardless of the device's own response order.
	if remotes[0].Channel != 0 || remotes[0].Address != "192.168.1.220" || remotes[0].Name != "BAN 1" {
		t.Fatalf("remotes[0] = %+v, want Channel=0 Address=192.168.1.220 Name=BAN 1", remotes[0])
	}
	if remotes[1].Channel != 1 || remotes[1].Address != "192.168.1.221" || remotes[1].Name != "BAN 2" {
		t.Fatalf("remotes[1] = %+v, want Channel=1 Address=192.168.1.221 Name=BAN 2", remotes[1])
	}
	if !remotes[0].Enable || !remotes[1].Enable {
		t.Fatalf("Enable should always be true for ISAPI InputProxy channels: %+v", remotes)
	}
	if remotes[0].Port != 8000 || remotes[1].Port != 8000 {
		t.Fatalf("Port not carried through: %+v", remotes)
	}
}

// TestGetRemoteDevicesEmpty covers an NVR with no proxied channels.
func TestGetRemoteDevicesEmpty(t *testing.T) {
	fake := &hikStorageChannelsServer{inputProxyBody: `<?xml version="1.0"?><InputProxyChannelList></InputProxyChannelList>`}
	srv := httptest.NewServer(fake.handler())
	defer srv.Close()
	c := newHikStorageTestClient(t, srv)

	remotes, err := c.GetRemoteDevices(context.Background())
	if err != nil {
		t.Fatalf("GetRemoteDevices: %v", err)
	}
	if len(remotes) != 0 {
		t.Fatalf("got %d remotes, want 0", len(remotes))
	}
}
