package traffic

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func TestTraffic_Manager(t *testing.T) {
	tmpDir := t.TempDir()
	invFile := filepath.Join(tmpDir, "cameras.yaml")
	_ = os.WriteFile(invFile, []byte(`
- id: "192.168.1.201:81"
  name: "Camera01"
  host: "192.168.1.201"
  port: 81
`), 0644)
	inv, err := config.LoadInventory(invFile)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}

	mgr := NewManager(inv)

	// Test interface listing
	ifaces, err := ListEligibleInterfaces()
	if err != nil {
		t.Logf("ListEligibleInterfaces: %v", err)
	}
	for _, ifi := range ifaces {
		if ifi == "wlan0" || ifi == "lo" {
			t.Errorf("expected wlan0/lo excluded, got %s", ifi)
		}
	}

	// Test subscription
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, unsub := mgr.Subscribe(ctx, "eth0")
	defer unsub()

	// Simulate recording packets
	mgr.recordPacket("192.168.1.201", "192.168.1.157", 554, 45678, "TCP", 1400)
	mgr.recordPacket("192.168.1.201", "192.168.1.157", 554, 45678, "TCP", 1400)

	snap := mgr.GetSnapshot("eth0")
	if snap.Interface != "eth0" {
		t.Errorf("expected iface eth0, got %s", snap.Interface)
	}

	if len(snap.Flows) == 0 {
		t.Errorf("expected at least 1 flow, got 0")
	} else {
		f := snap.Flows[0]
		if !f.IsCamera || f.CameraName != "Camera01" {
			t.Errorf("expected Camera01, got %+v", f)
		}
		if f.Service != "RTSP (Video Stream)" {
			t.Errorf("expected RTSP, got %s", f.Service)
		}
	}

	// Verify channel receives
	select {
	case <-ch:
	case <-time.After(2500 * time.Millisecond):
		// pulse takes 1s
	}
}

func TestTraffic_ResolveService(t *testing.T) {
	tests := []struct {
		src, dst int
		want     string
	}{
		{554, 12345, "RTSP (Video Stream)"},
		{12345, 80, "HTTP/ISAPI"},
		{12345, 1984, "Go2RTC (API/WebRTC)"},
		{12345, 37777, "Dahua DVRIP"},
		{12345, 12369, "MQTT"},
		{12345, 9999, "TCP/UDP"},
	}

	for _, tc := range tests {
		got := resolveService(tc.src, tc.dst)
		if got != tc.want {
			t.Errorf("resolveService(%d, %d) = %s, want %s", tc.src, tc.dst, got, tc.want)
		}
	}
}
