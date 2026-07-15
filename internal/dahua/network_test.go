package dahua

import "testing"

func TestValidIPv4(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"192.168.1.10", true},
		{"255.255.255.0", true},
		{"0.0.0.0", true},
		{"", false},
		{"not-an-ip", false},
		{"::1", false}, // IPv6 rejected — every device field here is IPv4-only
		{"999.1.1.1", false},
	}
	for _, c := range cases {
		if got := validIPv4(c.in); got != c.want {
			t.Errorf("validIPv4(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestValidateStaticIPRejectsInvalidInput exercises the validation
// SetStaticIP runs before touching the network — kept as a standalone
// function (validateStaticIP) specifically so this doesn't need a live/fake
// connection.
func TestValidateStaticIPRejectsInvalidInput(t *testing.T) {
	if err := validateStaticIP(false, "not-an-ip", "255.255.255.0", "192.168.1.1", nil); err == nil {
		t.Error("expected error for invalid IP")
	}
	if err := validateStaticIP(false, "192.168.1.10", "bad-mask", "192.168.1.1", nil); err == nil {
		t.Error("expected error for invalid subnet mask")
	}
	if err := validateStaticIP(false, "192.168.1.10", "255.255.255.0", "bad-gateway", nil); err == nil {
		t.Error("expected error for invalid gateway")
	}
	if err := validateStaticIP(false, "192.168.1.10", "255.255.255.0", "192.168.1.1", []string{"bad-dns"}); err == nil {
		t.Error("expected error for invalid DNS server")
	}
	if err := validateStaticIP(false, "192.168.1.10", "255.255.255.0", "192.168.1.1", []string{"8.8.8.8", "1.1.1.1"}); err != nil {
		t.Errorf("valid input rejected: %v", err)
	}
}

// TestValidateStaticIPSkipsChecksWhenDHCP confirms enabling DHCP never
// requires well-formed ip/mask/gateway — the device will supply them.
func TestValidateStaticIPSkipsChecksWhenDHCP(t *testing.T) {
	if err := validateStaticIP(true, "", "", "", nil); err != nil {
		t.Errorf("DHCP path should skip validation entirely: %v", err)
	}
}

func TestParseWlanDevices(t *testing.T) {
	body := "Found Num:2\r\n" +
		"wlanDevice[0].SSID=home_wifi\r\n" +
		"wlanDevice[0].BSSID=aa:bb:cc:dd:ee:ff\r\n" +
		"wlanDevice[0].AuthMode=7\r\n" +
		"wlanDevice[0].LinkQuality=80\r\n" +
		"wlanDevice[1].SSID=office_wifi\r\n" +
		"wlanDevice[1].LinkQuality=40\r\n"
	aps := parseWlanDevices(body)
	if len(aps) != 2 {
		t.Fatalf("got %d APs, want 2", len(aps))
	}
	if aps[0].SSID != "home_wifi" || aps[0].BSSID != "aa:bb:cc:dd:ee:ff" || aps[0].AuthMode != "7" || aps[0].LinkQuality != 80 {
		t.Errorf("ap0 = %+v", aps[0])
	}
	if aps[1].SSID != "office_wifi" || aps[1].LinkQuality != 40 {
		t.Errorf("ap1 = %+v", aps[1])
	}
}

func TestParseWlanDevicesEmpty(t *testing.T) {
	if aps := parseWlanDevices("Found Num:0\r\n"); len(aps) != 0 {
		t.Errorf("expected no APs, got %v", aps)
	}
}
