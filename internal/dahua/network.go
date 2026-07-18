package dahua

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

// getObjectTable fetches configManager.getConfig <name> and returns
// params.table as a map, for tables keyed by field/interface name (Network,
// WLan, PPPoE, NTP, UPnP...) rather than indexed by channel. See getTable in
// encode.go for the array-shaped counterpart used by
// ChannelTitle/VideoWidget/VideoColor/VideoInOptions/Encode.
func (c *Client) getObjectTable(name string) (map[string]any, error) {
	resp, err := c.Call("configManager.getConfig", map[string]any{"name": name})
	if err != nil {
		return nil, err
	}
	if !resp.ok() {
		return nil, fmt.Errorf("getConfig %s failed: %s", name, respErr(resp))
	}
	var p struct {
		Table map[string]any `json:"table"`
	}
	if err := json.Unmarshal(resp.Params, &p); err != nil {
		return nil, fmt.Errorf("getConfig %s: decode table: %w", name, err)
	}
	return p.Table, nil
}

// setObjectTable writes configManager.setConfig <name> with the full
// object-shaped table.
func (c *Client) setObjectTable(name string, table map[string]any) error {
	resp, err := c.Call("configManager.setConfig", map[string]any{"name": name, "table": table})
	if err != nil {
		return err
	}
	if !resp.ok() {
		return fmt.Errorf("setConfig %s failed: %s", name, respErr(resp))
	}
	return nil
}

// NetworkConfig is the device's Network table: the default interface name
// plus one raw field map per network interface (typically just "eth0"; a
// Wi-Fi-capable model adds a second, e.g. "eth2", per
// dahua_http_api_for_ipcsd-v1.40.pdf §5.2). Each interface map carries
// whatever the device returns verbatim — DhcpEnable/IPAddress/SubnetMask/
// DefaultGateway/DnsServers[]/MTU/PhysicalAddress.
type NetworkConfig struct {
	DefaultInterface string                    `json:"defaultInterface"`
	Interfaces       map[string]map[string]any `json:"interfaces"`
}

// nonInterfaceNetworkKeys are the Network table's scalar fields, not nested
// per-interface objects (see §5.2: DefaultInterface/Domain/Hostname sit
// alongside interface-named keys like "eth0" at the same table level).
var nonInterfaceNetworkKeys = map[string]bool{"DefaultInterface": true, "Domain": true, "Hostname": true}

// GetNetworkConfig reads the device's Network table (static IP / DHCP config
// for every interface) via the existing DVRIP session — no HTTP/port 80
// needed.
func (c *Client) GetNetworkConfig() (NetworkConfig, error) {
	table, err := c.getObjectTable("Network")
	if err != nil {
		return NetworkConfig{}, err
	}
	cfg := NetworkConfig{Interfaces: map[string]map[string]any{}}
	cfg.DefaultInterface, _ = table["DefaultInterface"].(string)
	for k, v := range table {
		if nonInterfaceNetworkKeys[k] {
			continue
		}
		if iface, ok := v.(map[string]any); ok {
			cfg.Interfaces[k] = iface
		}
	}
	return cfg, nil
}

// validIPv4 reports whether s is a dotted-decimal IPv4 address, as required
// by every field SetStaticIP writes (IPAddress/SubnetMask/DefaultGateway/
// DnsServers). Rejecting malformed input here — rather than letting the
// device try to parse it — matters because a bad value written to a camera's
// only network interface can make it unreachable.
func validIPv4(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil && ip.To4() != nil
}

// validateStaticIP checks ip/mask/gateway/dns are well-formed dotted-decimal
// IPv4 addresses, skipping the check entirely when dhcpEnable is true (DHCP
// will supply them, so whatever the caller passed is irrelevant). Split out
// from SetStaticIP so it's unit-testable without a live/fake connection.
func validateStaticIP(dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	if dhcpEnable {
		return nil
	}
	if !validIPv4(ip) {
		return fmt.Errorf("dahua: invalid IP address %q", ip)
	}
	if !validIPv4(mask) {
		return fmt.Errorf("dahua: invalid subnet mask %q", mask)
	}
	if gateway != "" && !validIPv4(gateway) {
		return fmt.Errorf("dahua: invalid gateway %q", gateway)
	}
	for _, d := range dns {
		if d != "" && !validIPv4(d) {
			return fmt.Errorf("dahua: invalid DNS server %q", d)
		}
	}
	return nil
}

// SetStaticIP writes one interface's IP configuration (Network.<iface>.*) via
// GET-modify-SET on the Network table. ip/mask/gateway/dns are validated as
// dotted-decimal IPv4 addresses before anything is sent. When dhcpEnable is
// true, ip/mask/gateway/dns are ignored (DHCP will supply them) and only
// Network.<iface>.DhcpEnable is flipped on.
func (c *Client) SetStaticIP(iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	if err := validateStaticIP(dhcpEnable, ip, mask, gateway, dns); err != nil {
		return err
	}
	table, err := c.getObjectTable("Network")
	if err != nil {
		return err
	}
	ifaceObj, ok := table[iface].(map[string]any)
	if !ok {
		return fmt.Errorf("dahua: Network interface %q not found", iface)
	}
	ifaceObj["DhcpEnable"] = dhcpEnable
	if !dhcpEnable {
		ifaceObj["IPAddress"] = ip
		ifaceObj["SubnetMask"] = mask
		if gateway != "" {
			ifaceObj["DefaultGateway"] = gateway
		}
		if len(dns) > 0 {
			dnsArr := make([]any, len(dns))
			for i, d := range dns {
				dnsArr[i] = d
			}
			ifaceObj["DnsServers"] = dnsArr
		}
	}
	return c.setObjectTable("Network", table)
}

// GetWiFiConfig reads the device's WLan table (SSID/security per Wi-Fi
// interface) via the existing DVRIP session. Devices with no Wi-Fi radio
// return an empty map (the device rejects the getConfig call, which callers
// should treat as "not supported" rather than an error) — see camera.go's
// NetworkSettings wiring for how that's surfaced.
func (c *Client) GetWiFiConfig() (map[string]map[string]any, error) {
	table, err := c.getObjectTable("WLan")
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]any{}
	for k, v := range table {
		if iface, ok := v.(map[string]any); ok {
			out[k] = iface
		}
	}
	return out, nil
}

// SetWiFiConfig writes one Wi-Fi interface's SSID/password
// (WLan.<iface>.SSID/Keys[0]/Encryption/KeyType/KeyID/KeyFlag) via
// GET-modify-SET, using slot 0 of the 4-slot Keys[] array. encryption is one
// of the device's supported values (e.g. "WPA2PSK"; "Off" disables security)
// per dahua_http_api_for_ipcsd-v1.40.pdf §5.6.2, which varies by firmware —
// left as-is (zero value skips the field) when empty.
func (c *Client) SetWiFiConfig(iface, ssid, password, encryption string) error {
	table, err := c.getObjectTable("WLan")
	if err != nil {
		return err
	}
	ifaceObj, ok := table[iface].(map[string]any)
	if !ok {
		return fmt.Errorf("dahua: WLan interface %q not found", iface)
	}
	ifaceObj["SSID"] = ssid
	ifaceObj["Enable"] = true
	if encryption != "" {
		ifaceObj["Encryption"] = encryption
	}
	if password != "" {
		keys, _ := ifaceObj["Keys"].([]any)
		for len(keys) < 4 {
			keys = append(keys, "")
		}
		keys[0] = password
		ifaceObj["Keys"] = keys
		ifaceObj["KeyID"] = float64(0)
		ifaceObj["KeyFlag"] = true
		ifaceObj["KeyType"] = "ASCII"
	}
	return c.setObjectTable("WLan", table)
}

// WiFiAP is one access point returned by ScanWiFi.
type WiFiAP struct {
	SSID        string `json:"ssid"`
	BSSID       string `json:"bssid"`
	AuthMode    string `json:"authMode"`
	LinkQuality int    `json:"linkQuality"`
}

// ScanWiFiRPC triggers a live Wi-Fi access-point scan over the existing DVRIP
// session via netApp.scanWLanDevices, which returns the AP list in one call:
// {"wlanDevice":[{"SSID","BSSID","Encryption","LinkQuality","AuthMode",...}]}.
// This is the DVRIP-native counterpart to the CGI-only ScanWiFi (wlan.cgi on
// port 80), which the NAT'd/DVRIP-only cameras in the field don't serve —
// confirmed live against a DH-C5A. iface defaults to "wlan0" when empty.
func (c *Client) ScanWiFiRPC(iface string) ([]WiFiAP, error) {
	if iface == "" {
		iface = "wlan0"
	}
	resp, err := c.Call("netApp.scanWLanDevices", map[string]any{"Name": iface})
	if err != nil {
		return nil, err
	}
	if !resp.ok() {
		return nil, fmt.Errorf("netApp.scanWLanDevices failed: %s", respErr(resp))
	}
	var p struct {
		WlanDevice []struct {
			SSID        string `json:"SSID"`
			BSSID       string `json:"BSSID"`
			Encryption  string `json:"Encryption"`
			LinkQuality int    `json:"LinkQuality"`
		} `json:"wlanDevice"`
	}
	if err := json.Unmarshal(resp.Params, &p); err != nil {
		return nil, fmt.Errorf("netApp.scanWLanDevices: decode: %w", err)
	}
	out := make([]WiFiAP, 0, len(p.WlanDevice))
	for _, d := range p.WlanDevice {
		auth := "Open"
		if d.Encryption != "" && d.Encryption != "Off" {
			auth = "Encrypted"
		}
		out = append(out, WiFiAP{SSID: d.SSID, BSSID: d.BSSID, AuthMode: auth, LinkQuality: d.LinkQuality})
	}
	return out, nil
}

var wlanDeviceLineRe = regexp.MustCompile(`^wlanDevice\[(\d+)\]\.(\w+)=(.*)$`)

// parseWlanDevices decodes the text/plain "wlanDevice[N].Field=Value" body
// documented in dahua_http_api_for_ipcsd-v1.40.pdf §5.6.3, preserving the
// device's own ordering (by index).
func parseWlanDevices(body string) []WiFiAP {
	byIdx := map[int]*WiFiAP{}
	var order []int
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		m := wlanDeviceLineRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		ap, ok := byIdx[idx]
		if !ok {
			ap = &WiFiAP{}
			byIdx[idx] = ap
			order = append(order, idx)
		}
		switch m[2] {
		case "SSID":
			ap.SSID = m[3]
		case "BSSID":
			ap.BSSID = m[3]
		case "AuthMode":
			ap.AuthMode = m[3]
		case "LinkQuality":
			q, _ := strconv.Atoi(m[3])
			ap.LinkQuality = q
		}
	}
	sort.Ints(order)
	out := make([]WiFiAP, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIdx[idx])
	}
	return out
}

// ScanWiFi triggers the device's Wi-Fi access-point scan (GET
// /cgi-bin/wlan.cgi?action=scanWlanDevices), confirmed against
// dahua_http_api_for_ipcsd-v1.40.pdf §5.6.3. Like GetSnapshot, this is plain
// HTTP+Digest on port 80 (the CGI web port) — there is no configManager/DVRIP
// equivalent for a live scan, so this opens a separate connection from the
// DVRIP session. host must be bare (no port).
func ScanWiFi(ctx context.Context, host, user, pass string, timeout time.Duration) ([]WiFiAP, error) {
	digest := isapi.NewDigestTransport(user, pass, nil)
	client := &http.Client{Transport: digest, Timeout: timeout}
	url := fmt.Sprintf("http://%s:80/cgi-bin/wlan.cgi?action=scanWlanDevices", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("dahua: build wifi scan request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dahua: wifi scan %s: %w", host, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("dahua: read wifi scan %s: %w", host, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dahua: wifi scan %s: HTTP %d: %s", host, resp.StatusCode, snapshotTruncate(body, 200))
	}
	return parseWlanDevices(string(body)), nil
}
