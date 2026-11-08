package xiongmai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// SystemInfoData represents Xiongmai's SystemInfo response block.
type SystemInfoData struct {
	SerialNo        string
	SoftWareVersion string
	Hardware        string
	DeviceModel     string
	BuildTime       string
	VideoInChannel  int
	DigChannel      int
	AudioInChannel  int
	UpdataTime      string
}

// TotalChannels returns analog + digital channels reported by SystemInfo.
func (s SystemInfoData) TotalChannels() int {
	n := s.VideoInChannel + s.DigChannel
	if n <= 0 {
		return 1
	}
	return n
}

// GetSystemInfo reads and caches SystemInfo from the device.
func (c *Client) GetSystemInfo(ctx context.Context) (SystemInfoData, error) {
	c.mu.Lock()
	if c.sysInfo != nil {
		cached := *c.sysInfo
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	tr, err := c.Transport(ctx)
	if err != nil {
		return SystemInfoData{}, err
	}
	resp, err := tr.GetInfo(ctx, "SystemInfo")
	if err != nil {
		return SystemInfoData{}, err
	}
	m, _ := resp["SystemInfo"].(map[string]any)
	if m == nil {
		return SystemInfoData{}, fmt.Errorf("xiongmai: missing SystemInfo object")
	}
	data := SystemInfoData{
		SerialNo:        strings.TrimSpace(stringFieldAny(m, "SerialNo")),
		SoftWareVersion: strings.TrimSpace(stringFieldAny(m, "SoftWareVersion")),
		Hardware:        strings.TrimSpace(stringFieldAny(m, "Hardware")),
		DeviceModel:     strings.TrimSpace(stringFieldAny(m, "DeviceModel")),
		BuildTime:       strings.TrimSpace(stringFieldAny(m, "BuildTime")),
		VideoInChannel:  intFieldAny(m, "VideoInChannel"),
		DigChannel:      intFieldAny(m, "DigChannel"),
		AudioInChannel:  intFieldAny(m, "AudioInChannel"),
		UpdataTime:      strings.TrimSpace(stringFieldAny(m, "UpdataTime")),
	}
	// Fallback to LoginInfo ChannelNum if both VideoInChannel and DigChannel are 0.
	if data.VideoInChannel == 0 && data.DigChannel == 0 {
		if li := tr.LoginInfo(); li != nil && li.ChannelNum > 0 {
			data.VideoInChannel = li.ChannelNum
		}
	}
	c.mu.Lock()
	c.sysInfo = &data
	c.mu.Unlock()
	return data, nil
}

// GetSerialNumber returns the hardware SerialNo from SystemInfo.
func (c *Client) GetSerialNumber(ctx context.Context) (string, error) {
	sys, err := c.GetSystemInfo(ctx)
	if err != nil {
		return "", err
	}
	return sys.SerialNo, nil
}

// Reboot triggers an immediate device restart via OPMachine Action="Reboot".
func (c *Client) Reboot(ctx context.Context) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	_, err = tr.OpDev(ctx, "OPMachine", map[string]any{"Action": "Reboot"})
	return err
}

// ChangePassword updates the password for user (defaults to current client username).
func (c *Client) ChangePassword(ctx context.Context, newUser, newPass string) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	targetUser := strings.TrimSpace(newUser)
	if targetUser == "" {
		targetUser = c.username
	}
	if err := tr.ModifyPassword(ctx, targetUser, c.password, newPass); err != nil {
		return err
	}
	c.password = newPass
	return nil
}

// parseHexOrDecMB parses Xiongmai StorageInfo Partition TotalSpace/RemainSpace
// (typically hex string in MB like "0x0000EC00" = 60,416 MB, or float64).
func parseHexOrDecBytesFromMB(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val) * 1024 * 1024
	case int:
		return int64(val) * 1024 * 1024
	case string:
		s := strings.TrimSpace(val)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			if n, err := strconv.ParseUint(s[2:], 16, 64); err == nil {
				return int64(n) * 1024 * 1024
			}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n * 1024 * 1024
		}
	}
	return 0
}

// GetStorageInfo queries StorageInfo and maps partitions to dahua.StorageDevice.
func (c *Client) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := tr.GetInfo(ctx, "StorageInfo")
	if err != nil {
		if isUnsupportedErr(err) {
			return []dahua.StorageDevice{}, nil
		}
		return nil, err
	}
	rawArr, ok := resp["StorageInfo"].([]any)
	if !ok || len(rawArr) == 0 {
		return []dahua.StorageDevice{}, nil
	}
	out := make([]dahua.StorageDevice, 0)
	for i, item := range rawArr {
		disk, ok := item.(map[string]any)
		if !ok {
			continue
		}
		physNo := intFieldAny(disk, "PlysicalNo", "PhysicalNo")
		if physNo == 0 && i > 0 {
			physNo = i
		}
		var totalBytes, freeBytes int64
		health := "Normal"
		parts, _ := disk["Partition"].([]any)
		for _, pRaw := range parts {
			pm, ok := pRaw.(map[string]any)
			if !ok {
				continue
			}
			totalBytes += parseHexOrDecBytesFromMB(pm["TotalSpace"])
			freeBytes += parseHexOrDecBytesFromMB(pm["RemainSpace"])
			if st := intFieldAny(pm, "Status"); st != 0 {
				health = fmt.Sprintf("Status:%d", st)
			}
		}
		if totalBytes <= 0 {
			continue
		}
		diskName := fmt.Sprintf("Disk-%d", physNo)
		isErr := health != "Normal"
		out = append(out, dahua.StorageDevice{
			Name:  diskName,
			State: health,
			Details: []dahua.StorageDetail{
				{
					Path:       diskName,
					Type:       "ReadWrite",
					TotalBytes: totalBytes,
					UsedBytes:  totalBytes - freeBytes,
					IsError:    isErr,
				},
			},
		})
	}
	return out, nil
}

// FormatStorage clears partition 0 of the specified disk ("Disk-0" -> PartNo 0, SerialNo 0).
func (c *Client) FormatStorage(ctx context.Context, name string) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	serialNo := 0
	trimmed := strings.TrimPrefix(strings.TrimSpace(name), "Disk-")
	if n, nerr := strconv.Atoi(trimmed); nerr == nil && n >= 0 {
		serialNo = n
	}
	_, err = tr.OpDev(ctx, "OPStorageManager", map[string]any{
		"Action":   "Clear",
		"PartNo":   0,
		"SerialNo": serialNo,
	})
	return err
}

// GetNetworkConfig reads NetWork.NetCommon + NetWork.NetDHCP + NetWork.NetDNS.
func (c *Client) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return dahua.NetworkConfig{}, err
	}
	resp, err := tr.GetConfig(ctx, "NetWork.NetCommon")
	if err != nil {
		return dahua.NetworkConfig{}, err
	}
	nc, ok := resp["NetWork.NetCommon"].(map[string]any)
	if !ok {
		return dahua.NetworkConfig{}, fmt.Errorf("xiongmai: missing NetWork.NetCommon")
	}

	ip, _ := HexIPToDecIP(stringFieldAny(nc, "HostIP"))
	mask, _ := HexIPToDecIP(stringFieldAny(nc, "Submask"))
	gw, _ := HexIPToDecIP(stringFieldAny(nc, "GateWay"))
	mac := stringFieldAny(nc, "MAC")

	dhcpEnable := false
	if dResp, derr := tr.GetConfig(ctx, "NetWork.NetDHCP"); derr == nil {
		if arr, ok := dResp["NetWork.NetDHCP"].([]any); ok && len(arr) > 0 {
			if m0, ok := arr[0].(map[string]any); ok {
				dhcpEnable = boolFieldAny(m0, "Enable")
			}
		}
	}

	var dns []string
	if dnsResp, derr := tr.GetConfig(ctx, "NetWork.NetDNS"); derr == nil {
		if dm, ok := dnsResp["NetWork.NetDNS"].(map[string]any); ok {
			if d1, err := HexIPToDecIP(stringFieldAny(dm, "Address")); err == nil && d1 != "0.0.0.0" {
				dns = append(dns, d1)
			}
			if d2, err := HexIPToDecIP(stringFieldAny(dm, "SpareAddress")); err == nil && d2 != "0.0.0.0" {
				dns = append(dns, d2)
			}
		}
	}

	return dahua.NetworkConfig{
		DefaultInterface: "eth0",
		Interfaces: map[string]map[string]any{
			"eth0": {
				"DhcpEnable":      dhcpEnable,
				"IPAddress":       ip,
				"SubnetMask":      mask,
				"DefaultGateway":  gw,
				"DnsServers":      dns,
				"PhysicalAddress": strings.ToLower(mac),
			},
		},
	}, nil
}

// SetStaticIP updates NetWork.NetCommon, NetWork.NetDHCP, and NetWork.NetDNS.
func (c *Client) SetStaticIP(ctx context.Context, _ string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	if !dhcpEnable {
		hexIP, err := DecIPToHexIP(ip)
		if err != nil {
			return err
		}
		hexMask, err := DecIPToHexIP(mask)
		if err != nil {
			return err
		}
		hexGW, err := DecIPToHexIP(gateway)
		if err != nil {
			return err
		}
		resp, err := tr.GetConfig(ctx, "NetWork.NetCommon")
		if err != nil {
			return err
		}
		nc, ok := resp["NetWork.NetCommon"].(map[string]any)
		if !ok {
			return fmt.Errorf("xiongmai: missing NetWork.NetCommon")
		}
		nc["HostIP"] = hexIP
		nc["Submask"] = hexMask
		nc["GateWay"] = hexGW
		if _, err := tr.SetConfig(ctx, "NetWork.NetCommon", nc); err != nil {
			return err
		}
	}

	if dResp, derr := tr.GetConfig(ctx, "NetWork.NetDHCP"); derr == nil {
		if arr, ok := dResp["NetWork.NetDHCP"].([]any); ok && len(arr) > 0 {
			if m0, ok := arr[0].(map[string]any); ok {
				m0["Enable"] = dhcpEnable
				_, _ = tr.SetConfig(ctx, "NetWork.NetDHCP", arr)
			}
		}
	}

	if len(dns) > 0 {
		if dnsResp, derr := tr.GetConfig(ctx, "NetWork.NetDNS"); derr == nil {
			if dm, ok := dnsResp["NetWork.NetDNS"].(map[string]any); ok {
				if h1, err := DecIPToHexIP(dns[0]); err == nil {
					dm["Address"] = h1
				}
				if len(dns) > 1 {
					if h2, err := DecIPToHexIP(dns[1]); err == nil {
						dm["SpareAddress"] = h2
					}
				}
				_, _ = tr.SetConfig(ctx, "NetWork.NetDNS", dm)
			}
		}
	}
	return nil
}

// GetTimeConfig queries OPTimeQuery and NetWork.NetNTP.
func (c *Client) GetTimeConfig(ctx context.Context) (dahua.TimeConfig, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return dahua.TimeConfig{}, err
	}
	var out dahua.TimeConfig
	if tResp, terr := tr.OpDev(ctx, "OPTimeQuery", nil); terr == nil {
		out.CurrentTime = strings.TrimSpace(stringFieldAny(tResp, "OPTimeQuery"))
	}
	if nResp, nerr := tr.GetConfig(ctx, "NetWork.NetNTP"); nerr == nil {
		if nm, ok := nResp["NetWork.NetNTP"].(map[string]any); ok {
			out.NTPEnable = boolFieldAny(nm, "Enable")
			out.NTPPort = intFieldAny(nm, "UpdatePeriod")
			if srv, ok := nm["Server"].(map[string]any); ok {
				out.NTPAddress = stringFieldAny(srv, "Name")
				if p := intFieldAny(srv, "Port"); p > 0 {
					out.NTPPort = p
				}
			}
			// TimeZone on Xiongmai is stored in minutes West of UTC when |tzMin| >= 60
			// (e.g. -420 = UTC+7); Sofia IPCs store a table enum index (0..32) instead.
			tzMin := intFieldAny(nm, "TimeZone")
			if tzMin <= -60 || tzMin >= 60 {
				hours := -tzMin / 60
				mins := (-tzMin) % 60
				if mins < 0 {
					mins = -mins
				}
				out.TimeZoneDesc = fmt.Sprintf("GMT%+03d:%02d", hours, mins)
			}
		}
	}
	return out, nil
}

// SetTimeConfig sets device clock (OPTimeSetting) and/or NetWork.NetNTP.
func (c *Client) SetTimeConfig(ctx context.Context, cfg dahua.TimeConfig) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	if cfg.NTPAddress != "" {
		if nResp, nerr := tr.GetConfig(ctx, "NetWork.NetNTP"); nerr == nil {
			if nm, ok := nResp["NetWork.NetNTP"].(map[string]any); ok {
				nm["Enable"] = cfg.NTPEnable
				srv, _ := nm["Server"].(map[string]any)
				if srv == nil {
					srv = map[string]any{}
					nm["Server"] = srv
				}
				srv["Name"] = cfg.NTPAddress
				if cfg.NTPPort > 0 {
					srv["Port"] = cfg.NTPPort
				}
				_, _ = tr.SetConfig(ctx, "NetWork.NetNTP", nm)
			}
		}
	}
	if strings.TrimSpace(cfg.CurrentTime) != "" {
		ts := strings.ReplaceAll(strings.TrimSpace(cfg.CurrentTime), "T", " ")
		if t, perr := time.Parse("2006-01-02 15:04:05", ts); perr == nil {
			ts = t.Format("2006-01-02 15:04:05")
		}
		if _, err := tr.OpDev(ctx, "OPTimeSetting", ts); err != nil {
			return err
		}
	}
	return nil
}

var xmAutoRebootDays = []string{
	"Everyday", "EverySunday", "EveryMonday", "EveryTuesday",
	"EveryWednesday", "EveryThursday", "EveryFriday", "EverySaturday", "Never",
}

// GetAutoReboot reads General.AutoMaintain.
func (c *Client) GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error) {
	tr, err := c.Transport(ctx)
	if err != nil {
		return dahua.AutoReboot{}, err
	}
	resp, err := tr.GetConfig(ctx, "General.AutoMaintain")
	if err != nil {
		return dahua.AutoReboot{}, err
	}
	am, ok := resp["General.AutoMaintain"].(map[string]any)
	if !ok {
		return dahua.AutoReboot{}, fmt.Errorf("xiongmai: missing General.AutoMaintain")
	}
	dayStr := stringFieldAny(am, "AutoRebootDay")
	dayIdx := 0 // 0 = Never in dahua.AutoReboot convention: check dahua mapping
	for i, d := range xmAutoRebootDays {
		if strings.EqualFold(d, dayStr) {
			dayIdx = i
			break
		}
	}
	enable := !strings.EqualFold(dayStr, "Never") && dayStr != ""
	if strings.EqualFold(dayStr, "Never") {
		dayIdx = 0
	} else if strings.EqualFold(dayStr, "Everyday") {
		dayIdx = 8
	}
	return dahua.AutoReboot{
		Enable: enable,
		Day:    dayIdx,
		Hour:   intFieldAny(am, "AutoRebootHour"),
		Minute: intFieldAny(am, "AutoRebootMinute"),
	}, nil
}

// SetAutoReboot writes General.AutoMaintain.
func (c *Client) SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	resp, err := tr.GetConfig(ctx, "General.AutoMaintain")
	if err != nil {
		return err
	}
	am, ok := resp["General.AutoMaintain"].(map[string]any)
	if !ok {
		am = map[string]any{}
	}
	dayName := "Never"
	if !ar.Enable {
		ar.Day = 0
	}
	switch ar.Day {
	case 0:
		dayName = "Never"
	case 1:
		dayName = "EverySunday"
	case 2:
		dayName = "EveryMonday"
	case 3:
		dayName = "EveryTuesday"
	case 4:
		dayName = "EveryWednesday"
	case 5:
		dayName = "EveryThursday"
	case 6:
		dayName = "EveryFriday"
	case 7:
		dayName = "EverySaturday"
	default:
		dayName = "Everyday"
	}
	am["AutoRebootDay"] = dayName
	am["AutoRebootHour"] = ar.Hour
	am["AutoRebootMinute"] = ar.Minute
	_, err = tr.SetConfig(ctx, "General.AutoMaintain", am)
	return err
}

// PTZMove maps standard kspcam PTZ direction codes to Xiongmai OPPTZControl.
func (c *Client) PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error {
	tr, err := c.Transport(ctx)
	if err != nil {
		return err
	}
	if speed <= 0 {
		speed = 4
	}
	xmCmd := mapPTZCommand(code)
	step := speed
	if !start {
		step = 0
	}
	_, err = tr.PTZ(ctx, map[string]any{
		"Command": xmCmd,
		"Parameter": map[string]any{
			"AUX":      map[string]any{"Number": 0, "Status": "Off"},
			"Channel":  channel,
			"MenuOpts": "Enter",
			"POINT":    map[string]any{"bottom": 0, "left": 0, "right": 0, "top": 0},
			"Pattern":  "SetBegin",
			"Preset":   65535,
			"Step":     step,
			"Tour":     0,
		},
	})
	return err
}

func mapPTZCommand(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "up":
		return "DirectionUp"
	case "down":
		return "DirectionDown"
	case "left":
		return "DirectionLeft"
	case "right":
		return "DirectionRight"
	case "leftup":
		return "DirectionLeftUp"
	case "rightup":
		return "DirectionRightUp"
	case "leftdown":
		return "DirectionLeftDown"
	case "rightdown":
		return "DirectionRightDown"
	case "zoomtele", "zoomin":
		return "ZoomTile"
	case "zoomwide", "zoomout":
		return "ZoomWide"
	case "focusnear":
		return "FocusNear"
	case "focusfar":
		return "FocusFar"
	default:
		return code
	}
}

func isUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	if re, ok := err.(*RetError); ok {
		return re.Is(ErrUnsupported)
	}
	return false
}
