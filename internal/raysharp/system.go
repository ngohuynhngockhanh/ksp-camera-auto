package raysharp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// SystemBaseInfo holds device identity metadata from /API/SystemInfo/Base/Get.
type SystemBaseInfo struct {
	DeviceID        string `json:"device_id"`
	DeviceName      string `json:"device_name"`
	DeviceType      string `json:"device_type"`
	HardwareVersion string `json:"hardware_version"`
	SoftwareVersion string `json:"software_version"`
	IEClientVersion string `json:"ie_client_version"`
	MACAddress      string `json:"mac_address"`
	P2PID           string `json:"p2p_id"`
	QRCodeStr       string `json:"qrcode_str"`
	HDDVolume       string `json:"hdd_volume"`
	Language        string `json:"language"`
}

// GetSystemBase queries /API/SystemInfo/Base/Get.
func (c *Client) GetSystemBase(ctx context.Context) (SystemBaseInfo, error) {
	env, err := c.Call(ctx, "SystemInfo/Base/Get", map[string]any{})
	if err != nil {
		return SystemBaseInfo{}, err
	}
	var info SystemBaseInfo
	if err := json.Unmarshal(env.Data, &info); err != nil {
		return SystemBaseInfo{}, fmt.Errorf("raysharp SystemInfo/Base/Get decode: %w", err)
	}
	return info, nil
}

// GetSerialNumber returns the stable hardware serial/P2P ID (or MAC if P2P ID is empty).
func (c *Client) GetSerialNumber(ctx context.Context) (string, error) {
	info, err := c.GetSystemBase(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(info.P2PID) != "" {
		return strings.TrimSpace(info.P2PID), nil
	}
	if strings.TrimSpace(info.MACAddress) != "" {
		return strings.TrimSpace(info.MACAddress), nil
	}
	return strings.TrimSpace(info.DeviceID), nil
}

// GetChannelOSD reads channel name and OSD overlay lines from /API/ChannelConfig/OSD/Get.
func (c *Client) GetChannelOSD(ctx context.Context, channel int) (name string, lines []string, enabled []bool, supported bool, err error) {
	env, err := c.Call(ctx, "ChannelConfig/OSD/Get", map[string]any{})
	if err != nil {
		return "", nil, nil, false, err
	}
	var payload struct {
		ChannelInfo map[string]struct {
			Reason string `json:"reason"`
			Name   struct {
				Show bool   `json:"show"`
				Text string `json:"text"`
			} `json:"name"`
			Alarm struct {
				Show bool   `json:"show"`
				Text string `json:"text"`
			} `json:"alarm"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return "", nil, nil, false, fmt.Errorf("raysharp OSD/Get decode: %w", err)
	}
	chKey := ChannelKey(channel)
	chData, ok := payload.ChannelInfo[chKey]
	if !ok || (chData.Reason != "" && chData.Name.Text == "") {
		return "", nil, nil, false, nil
	}
	name = chData.Name.Text
	lines = []string{chData.Name.Text, chData.Alarm.Text}
	enabled = []bool{chData.Name.Show, chData.Alarm.Show}
	return name, lines, enabled, true, nil
}

// SetChannelName updates the channel name in /API/ChannelConfig/OSD/Set and
// (when supported on NVR) /API/ChannelConfig/IPChannel/Set.
func (c *Client) SetChannelName(ctx context.Context, channel int, name string) error {
	env, err := c.Call(ctx, "ChannelConfig/OSD/Get", map[string]any{})
	if err != nil {
		return err
	}
	var payload struct {
		ChannelInfo map[string]map[string]any `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return fmt.Errorf("raysharp OSD/Get decode: %w", err)
	}
	chKey := ChannelKey(channel)
	chMap, ok := payload.ChannelInfo[chKey]
	if !ok {
		return fmt.Errorf("raysharp OSD/Get: channel %s not found", chKey)
	}
	nameObj, _ := chMap["name"].(map[string]any)
	if nameObj == nil {
		nameObj = map[string]any{"pos": map[string]any{"x": 290, "y": 0}}
	}
	nameObj["text"] = name
	nameObj["show"] = strings.TrimSpace(name) != ""
	chMap["name"] = nameObj

	_, err = c.Call(ctx, "ChannelConfig/OSD/Set", map[string]any{
		"channel_info": map[string]any{chKey: chMap},
	})
	return err
}

// SetOSDLines writes up to 2 OSD text slots (name.text and alarm.text) on a Raysharp channel.
func (c *Client) SetOSDLines(ctx context.Context, channel int, lines []string, enabled []bool) (int, error) {
	env, err := c.Call(ctx, "ChannelConfig/OSD/Get", map[string]any{})
	if err != nil {
		return 0, err
	}
	var payload struct {
		ChannelInfo map[string]map[string]any `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return 0, fmt.Errorf("raysharp OSD/Get decode: %w", err)
	}
	chKey := ChannelKey(channel)
	chMap, ok := payload.ChannelInfo[chKey]
	if !ok {
		return 0, fmt.Errorf("raysharp OSD/Get: channel %s not found", chKey)
	}

	applied := 0
	if len(lines) > 0 {
		nameObj, _ := chMap["name"].(map[string]any)
		if nameObj == nil {
			nameObj = map[string]any{"pos": map[string]any{"x": 290, "y": 0}}
		}
		nameObj["text"] = lines[0]
		en := strings.TrimSpace(lines[0]) != ""
		if len(enabled) > 0 {
			en = enabled[0]
		}
		nameObj["show"] = en
		chMap["name"] = nameObj
		applied++
	}
	if len(lines) > 1 {
		alarmObj, _ := chMap["alarm"].(map[string]any)
		if alarmObj != nil {
			alarmObj["text"] = lines[1]
			en := strings.TrimSpace(lines[1]) != ""
			if len(enabled) > 1 {
				en = enabled[1]
			}
			alarmObj["show"] = en
			chMap["alarm"] = alarmObj
			applied++
		}
	}

	if _, err := c.Call(ctx, "ChannelConfig/OSD/Set", map[string]any{
		"channel_info": map[string]any{chKey: chMap},
	}); err != nil {
		return 0, err
	}
	return applied, nil
}

// Snapshot fetches a single JPEG frame via POST /API/Snapshot/Get and decodes
// its Base64 img_data payload.
func (c *Client) Snapshot(ctx context.Context, channel int) ([]byte, error) {
	chKey := ChannelKey(channel)
	req := map[string]any{
		"channel":               chKey,
		"snapshot_resolution":   "720 x 480",
		"reset_session_timeout": false,
	}
	env, err := c.Call(ctx, "Snapshot/Get", req)
	if err != nil {
		// Retry with "704 x 480" (used by NVR main.js) if 720 x 480 failed
		req["snapshot_resolution"] = "704 x 480"
		env, err = c.Call(ctx, "Snapshot/Get", req)
		if err != nil {
			return nil, err
		}
	}
	var snap struct {
		ImgFormat string `json:"img_format"`
		ImgData   string `json:"img_data"`
	}
	if err := json.Unmarshal(env.Data, &snap); err != nil {
		return nil, fmt.Errorf("raysharp Snapshot/Get decode: %w", err)
	}
	rawB64 := strings.TrimSpace(snap.ImgData)
	if idx := strings.Index(rawB64, ","); idx >= 0 {
		rawB64 = rawB64[idx+1:]
	}
	if rawB64 == "" {
		return nil, fmt.Errorf("raysharp Snapshot/Get returned empty img_data")
	}
	jpg, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		// Try RawStdEncoding if padding is omitted
		jpg, err = base64.RawStdEncoding.DecodeString(rawB64)
		if err != nil {
			return nil, fmt.Errorf("raysharp Snapshot/Get base64 decode: %w", err)
		}
	}
	return jpg, nil
}

// GetRemoteDevices lists connected IP cameras on a Raysharp NVR via
// /API/ChannelConfig/IPChannel/Get.
func (c *Client) GetRemoteDevices(ctx context.Context) ([]dahua.RemoteChannel, error) {
	env, err := c.Call(ctx, "ChannelConfig/IPChannel/Get", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		ChannelInfo map[string]struct {
			State       string `json:"state"`
			IPAddress   string `json:"ip_address"`
			Port        int    `json:"port"`
			ForwardPort int    `json:"forward_port"`
			ChannelName string `json:"channel_name"`
			DeviceType  string `json:"device_type"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, fmt.Errorf("raysharp IPChannel/Get decode: %w", err)
	}
	var out []dahua.RemoteChannel
	for k, v := range payload.ChannelInfo {
		chIdx, ok := ParseChannelKey(k)
		if !ok {
			continue
		}
		ip := NormalizeIPv4(v.IPAddress)
		enabled := !strings.EqualFold(v.State, "NotConfigured") && ip != "" && ip != "0.0.0.0"
		if !enabled && ip == "" {
			continue
		}
		name := strings.TrimSpace(v.ChannelName)
		if name == "" {
			name = v.DeviceType
		}
		port := v.Port
		if port <= 0 {
			port = 80
		}
		out = append(out, dahua.RemoteChannel{
			Channel: chIdx,
			Address: ip,
			Port:    port,
			Name:    name,
			Enable:  strings.EqualFold(v.State, "Online") || enabled,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Channel < out[j].Channel })
	return out, nil
}

// GetNetworkConfig reads /API/NetworkConfig/NetBase/Get and converts both IPC
// ("wan") and NVR ("lan1", "lan2") structures into dahua.NetworkConfig.
func (c *Client) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	env, err := c.Call(ctx, "NetworkConfig/NetBase/Get", map[string]any{})
	if err != nil {
		return dahua.NetworkConfig{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return dahua.NetworkConfig{}, fmt.Errorf("raysharp NetBase/Get decode: %w", err)
	}

	cfg := dahua.NetworkConfig{
		Interfaces: make(map[string]map[string]any),
	}
	topDNS1, _ := raw["dns1"].(string)
	topDNS2, _ := raw["dns2"].(string)

	normalizeIface := func(m map[string]any) map[string]any {
		out := make(map[string]any, len(m)+6)
		for k, v := range m {
			out[k] = v
		}
		dhcp, _ := m["dhcp"].(bool)
		ip, _ := m["ip_address"].(string)
		mask, _ := m["subnet_mask"].(string)
		gw, _ := m["gateway"].(string)
		mac, _ := m["mac_address"].(string)
		d1, _ := m["dns1"].(string)
		d2, _ := m["dns2"].(string)
		if d1 == "" {
			d1 = topDNS1
		}
		if d2 == "" {
			d2 = topDNS2
		}
		var dns []string
		if d1 != "" {
			dns = append(dns, NormalizeIPv4(d1))
		}
		if d2 != "" {
			dns = append(dns, NormalizeIPv4(d2))
		}
		out["DhcpEnable"] = dhcp
		out["IPAddress"] = NormalizeIPv4(ip)
		out["SubnetMask"] = NormalizeIPv4(mask)
		out["DefaultGateway"] = NormalizeIPv4(gw)
		out["DnsServers"] = dns
		if mac != "" {
			out["PhysicalAddress"] = strings.ReplaceAll(strings.ToLower(mac), "-", ":")
		}
		return out
	}

	if wan, ok := raw["wan"].(map[string]any); ok {
		cfg.DefaultInterface = "wan"
		cfg.Interfaces["wan"] = normalizeIface(wan)
	}
	for _, k := range []string{"lan1", "lan2"} {
		if lan, ok := raw[k].(map[string]any); ok {
			if cfg.DefaultInterface == "" {
				cfg.DefaultInterface = k
			}
			cfg.Interfaces[k] = normalizeIface(lan)
		}
	}
	return cfg, nil
}

// SetStaticIP updates an interface ("wan", "lan1", "lan2", or "eth0" mapped to default)
// via GET-Modify-SET on /API/NetworkConfig/NetBase/Set.
func (c *Client) SetStaticIP(ctx context.Context, iface string, dhcpEnable bool, ip, mask, gateway string, dns []string) error {
	env, err := c.Call(ctx, "NetworkConfig/NetBase/Get", map[string]any{})
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		return err
	}
	target := strings.TrimSpace(iface)
	if target == "" || target == "eth0" {
		if _, ok := raw["wan"]; ok {
			target = "wan"
		} else if _, ok := raw["lan1"]; ok {
			target = "lan1"
		}
	}
	ifMap, ok := raw[target].(map[string]any)
	if !ok {
		return fmt.Errorf("raysharp NetBase: interface %q not found", target)
	}
	ifMap["dhcp"] = dhcpEnable
	if !dhcpEnable {
		if ip != "" {
			ifMap["ip_address"] = NormalizeIPv4(ip)
		}
		if mask != "" {
			ifMap["subnet_mask"] = NormalizeIPv4(mask)
		}
		if gateway != "" {
			ifMap["gateway"] = NormalizeIPv4(gateway)
		}
	}
	if len(dns) > 0 && dns[0] != "" {
		if _, hasTopDNS := raw["dns1"]; hasTopDNS {
			raw["dns1"] = NormalizeIPv4(dns[0])
		} else {
			ifMap["dns1"] = NormalizeIPv4(dns[0])
		}
	}
	if len(dns) > 1 && dns[1] != "" {
		if _, hasTopDNS := raw["dns2"]; hasTopDNS {
			raw["dns2"] = NormalizeIPv4(dns[1])
		} else {
			ifMap["dns2"] = NormalizeIPv4(dns[1])
		}
	}
	raw[target] = ifMap
	_, err = c.Call(ctx, "NetworkConfig/NetBase/Set", raw)
	return err
}

// Reboot triggers an immediate device reboot via /API/Maintenance/DeviceReboot/Set.
func (c *Client) Reboot(ctx context.Context) error {
	if _, err := c.Call(ctx, "Maintenance/DeviceReboot/Set", map[string]any{}); err != nil {
		_, err2 := c.Call(ctx, "Maintenance/DeviceReboot", map[string]any{})
		return err2
	}
	return nil
}

// GetStorageInfo queries /API/StorageConfig/Disk/Get and converts disk_info[]
// into []dahua.StorageDevice.
func (c *Client) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	env, err := c.Call(ctx, "StorageConfig/Disk/Get", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		DiskInfo []struct {
			DiskID     any    `json:"disk_id"`
			DiskName   string `json:"disk_name"`
			Status     string `json:"status"`
			Property   string `json:"property"`
			TotalSpace int64  `json:"total_space"` // MB or bytes depending on firmware
			FreeSpace  int64  `json:"free_space"`
		} `json:"disk_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, fmt.Errorf("raysharp StorageConfig/Disk/Get decode: %w", err)
	}
	var out []dahua.StorageDevice
	for i, d := range payload.DiskInfo {
		name := d.DiskName
		if name == "" {
			name = fmt.Sprintf("HDD-%v", d.DiskID)
			if name == "HDD-<nil>" {
				name = fmt.Sprintf("HDD-%d", i+1)
			}
		}
		totalBytes := d.TotalSpace
		freeBytes := d.FreeSpace
		// Raysharp reports disk capacity in MB when < 100,000,000
		if totalBytes > 0 && totalBytes < 100*1024*1024 {
			totalBytes *= 1024 * 1024
			freeBytes *= 1024 * 1024
		}
		usedBytes := totalBytes - freeBytes
		if usedBytes < 0 {
			usedBytes = 0
		}
		rwType := "ReadWrite"
		if strings.EqualFold(d.Property, "ReadOnly") || strings.EqualFold(d.Property, "RO") {
			rwType = "ReadOnly"
		}
		st := strings.ToLower(d.Status)
		isErr := strings.Contains(st, "err") || strings.Contains(st, "bad")
		needFmt := strings.Contains(st, "unformat") || strings.Contains(st, "noformat")
		out = append(out, dahua.StorageDevice{
			Name:  name,
			State: d.Status,
			Details: []dahua.StorageDetail{
				{
					Path:         name,
					Type:         rwType,
					TotalBytes:   totalBytes,
					UsedBytes:    usedBytes,
					IsError:      isErr,
					IsNeedFormat: needFmt,
				},
			},
		})
	}
	return out, nil
}

// FormatStorage formats a storage disk on the Raysharp NVR/IPC.
func (c *Client) FormatStorage(ctx context.Context, name string) error {
	_, err := c.Call(ctx, "StorageConfig/Disk/Set", map[string]any{
		"hdd_format_type": "AllHddData",
		"disk_name":       name,
	})
	return err
}

// GetAutoReboot queries /API/Maintenance/AutoReboot/Get.
func (c *Client) GetAutoReboot(ctx context.Context) (dahua.AutoReboot, error) {
	env, err := c.Call(ctx, "Maintenance/AutoReboot/Get", map[string]any{})
	if err != nil {
		return dahua.AutoReboot{}, err
	}
	var p struct {
		AutoReboot          bool   `json:"auto_reboot"`
		MaintenanceInterval string `json:"maintenance_interval"`
		MaintenanceTime     string `json:"maintenance_time"`
	}
	if err := json.Unmarshal(env.Data, &p); err != nil {
		return dahua.AutoReboot{}, err
	}
	hour := 0
	min := 0
	if parts := strings.Split(p.MaintenanceTime, "-"); len(parts) > 0 {
		hm := strings.Split(strings.TrimSpace(parts[0]), ":")
		if len(hm) >= 2 {
			hour, _ = strconv.Atoi(hm[0])
			min, _ = strconv.Atoi(hm[1])
		}
	}
	return dahua.AutoReboot{
		Enable: p.AutoReboot,
		Day:    0,
		Hour:   hour,
		Minute: min,
	}, nil
}

// SetAutoReboot updates /API/Maintenance/AutoReboot/Set.
func (c *Client) SetAutoReboot(ctx context.Context, ar dahua.AutoReboot) error {
	env, err := c.Call(ctx, "Maintenance/AutoReboot/Get", map[string]any{})
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(env.Data, &raw); err != nil {
		raw = map[string]any{}
	}
	raw["auto_reboot"] = ar.Enable
	if ar.Enable {
		endHour := (ar.Hour + 2) % 24
		raw["maintenance_time"] = fmt.Sprintf("%02d:%02d-%02d:00", ar.Hour, ar.Minute, endHour)
		if _, ok := raw["maintenance_interval"]; !ok {
			raw["maintenance_interval"] = "7 Day"
		}
	}
	_, err = c.Call(ctx, "Maintenance/AutoReboot/Set", raw)
	return err
}

// GetTimeConfig queries /API/SystemConfig/DateTime/Get and /API/SystemConfig/NTP/Get.
func (c *Client) GetTimeConfig(ctx context.Context) (dahua.TimeConfig, error) {
	dtEnv, err := c.Call(ctx, "SystemConfig/DateTime/Get", map[string]any{})
	if err != nil {
		return dahua.TimeConfig{}, err
	}
	var dt struct {
		Date     string `json:"date"`
		Time     string `json:"time"`
		TimeZone string `json:"time_zone"`
	}
	_ = json.Unmarshal(dtEnv.Data, &dt)

	var ntp struct {
		NTPEnable    bool   `json:"ntp_enable"`
		Server       string `json:"server"`
		CustomServer string `json:"custom_server"`
	}
	if ntpEnv, err := c.Call(ctx, "SystemConfig/NTP/Get", map[string]any{}); err == nil {
		_ = json.Unmarshal(ntpEnv.Data, &ntp)
	}
	srv := ntp.Server
	if strings.EqualFold(srv, "Custom") && ntp.CustomServer != "" {
		srv = ntp.CustomServer
	}
	return dahua.TimeConfig{
		CurrentTime:  strings.TrimSpace(dt.Date + " " + dt.Time),
		NTPEnable:    ntp.NTPEnable,
		NTPAddress:   srv,
		NTPPort:      123,
		TimeZoneDesc: dt.TimeZone,
	}, nil
}

// SetTimeConfig writes /API/SystemConfig/DateTime/Set and /API/SystemConfig/NTP/Set.
func (c *Client) SetTimeConfig(ctx context.Context, cfg dahua.TimeConfig) error {
	if cfg.CurrentTime != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", cfg.CurrentTime, time.Local); err == nil {
			if dtEnv, err := c.Call(ctx, "SystemConfig/DateTime/Get", map[string]any{}); err == nil {
				var raw map[string]any
				if json.Unmarshal(dtEnv.Data, &raw) == nil {
					df, _ := raw["date_format"].(string)
					if strings.EqualFold(df, "MM/DD/YYYY") {
						raw["date"] = t.Format("01/02/2006")
					} else {
						raw["date"] = t.Format("2006-01-02")
					}
					raw["time"] = t.Format("15:04:05")
					_, _ = c.Call(ctx, "SystemConfig/DateTime/Set", raw)
				}
			}
		}
	}
	if ntpEnv, err := c.Call(ctx, "SystemConfig/NTP/Get", map[string]any{}); err == nil {
		var raw map[string]any
		if json.Unmarshal(ntpEnv.Data, &raw) == nil {
			raw["ntp_enable"] = cfg.NTPEnable
			if cfg.NTPAddress != "" {
				raw["server"] = cfg.NTPAddress
			}
			_, err = c.Call(ctx, "SystemConfig/NTP/Set", raw)
			return err
		}
	}
	return nil
}

// GetPicture reads /API/ChannelConfig/Color/Get and /API/ChannelConfig/ImageControl/Get.
func (c *Client) GetPicture(ctx context.Context, channel int) (color, options map[string]any, err error) {
	chKey := ChannelKey(channel)
	color = map[string]any{}
	options = map[string]any{}
	if cEnv, err := c.Call(ctx, "ChannelConfig/Color/Get", map[string]any{}); err == nil {
		var p struct {
			ChannelInfo map[string]map[string]any `json:"channel_info"`
		}
		if json.Unmarshal(cEnv.Data, &p) == nil && p.ChannelInfo[chKey] != nil {
			color = p.ChannelInfo[chKey]
		}
	}
	if iEnv, err := c.Call(ctx, "ChannelConfig/ImageControl/Get", map[string]any{}); err == nil {
		var p struct {
			ChannelInfo map[string]map[string]any `json:"channel_info"`
		}
		if json.Unmarshal(iEnv.Data, &p) == nil && p.ChannelInfo[chKey] != nil {
			options = p.ChannelInfo[chKey]
		}
	}
	return color, options, nil
}

// SetPicture merges colorChanges and optionsChanges into Color/Set and ImageControl/Set.
func (c *Client) SetPicture(ctx context.Context, channel int, colorChanges, optionsChanges map[string]any) (color, options map[string]any, err error) {
	chKey := ChannelKey(channel)
	curColor, curOpts, err := c.GetPicture(ctx, channel)
	if err != nil {
		return nil, nil, err
	}
	if len(colorChanges) > 0 {
		for k, v := range colorChanges {
			curColor[k] = v
		}
		if _, err := c.Call(ctx, "ChannelConfig/Color/Set", map[string]any{
			"channel_info": map[string]any{chKey: curColor},
		}); err != nil {
			return nil, nil, err
		}
	}
	if len(optionsChanges) > 0 {
		for k, v := range optionsChanges {
			curOpts[k] = v
		}
		if _, err := c.Call(ctx, "ChannelConfig/ImageControl/Set", map[string]any{
			"channel_info": map[string]any{chKey: curOpts},
		}); err != nil {
			return nil, nil, err
		}
	}
	return c.GetPicture(ctx, channel)
}

// GetPictureCaps reads /API/ChannelConfig/ImageControl/Range.
func (c *Client) GetPictureCaps(ctx context.Context, channel int) (map[string]any, error) {
	env, err := c.Call(ctx, "ChannelConfig/ImageControl/Range", map[string]any{})
	if err != nil {
		return map[string]any{}, nil
	}
	var out map[string]any
	_ = json.Unmarshal(env.Data, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// PTZMove maps standard PTZ codes to /API/PreviewChannel/PTZ/Control.
func (c *Client) PTZMove(ctx context.Context, channel int, code string, speed int, start bool) error {
	if speed < 1 {
		speed = 1
	}
	if speed > 8 {
		speed = 8
	}
	// Map Dahua PTZ code to Raysharp Ptz_Cmd_*
	cmdMap := map[string]string{
		"Up":        "Ptz_Cmd_Up",
		"Down":      "Ptz_Cmd_Down",
		"Left":      "Ptz_Cmd_Left",
		"Right":     "Ptz_Cmd_Right",
		"LeftUp":    "Ptz_Cmd_UpLeft",
		"LeftDown":  "Ptz_Cmd_DownLeft",
		"RightUp":   "Ptz_Cmd_UpRight",
		"RightDown": "Ptz_Cmd_DownRight",
		"ZoomWide":  "Ptz_Cmd_ZoomDec",
		"ZoomTele":  "Ptz_Cmd_ZoomInc",
		"FocusNear": "Ptz_Cmd_FocusDec",
		"FocusFar":  "Ptz_Cmd_FocusInc",
		"IrisLarge": "Ptz_Cmd_IrisInc",
		"IrisSmall": "Ptz_Cmd_IrisDec",
	}
	rsCmd := cmdMap[code]
	if rsCmd == "" {
		rsCmd = code
	}
	state := "Start"
	if !start {
		state = "Stop"
	}
	_, err := c.Call(ctx, "PreviewChannel/PTZ/Control", map[string]any{
		"channel": ChannelKey(channel),
		"cmd":     rsCmd,
		"speed":   speed * 10,
		"state":   state,
	})
	return err
}

// FindRecordings queries /API/Playback/SearchRecord/Search for recorded video segments.
func (c *Client) FindRecordings(ctx context.Context, channel int, start, end time.Time) ([]dahua.Recording, error) {
	chKey := ChannelKey(channel)
	req := map[string]any{
		"channel":             []string{chKey},
		"start_date":          start.Format("2006-01-02"),
		"start_time":          start.Format("15:04:05"),
		"end_date":            end.Format("2006-01-02"),
		"end_time":            end.Format("15:04:05"),
		"stream_mode":         "Mainstream",
		"record_type":         0,
		"record_type_ex":      []string{},
		"enable_smart_search": 0,
		"smart_region":        []int{},
		"record_type_arr":     []string{"normal", "motion", "io", "sound", "pd&vd", "fd", "lcd", "sod", "cc", "cd", "qd", "lpd", "rsd", "intrusion"},
	}
	env, err := c.Call(ctx, "Playback/SearchRecord/Search", req)
	if err != nil {
		return nil, err
	}
	var payload struct {
		ChannelInfo map[string]struct {
			RecordInfos []struct {
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
				Size      int64  `json:"size"`
				Type      string `json:"type"`
			} `json:"record_infos"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, nil
	}
	chRecs := payload.ChannelInfo[chKey].RecordInfos
	out := make([]dahua.Recording, 0, len(chRecs))
	for _, r := range chRecs {
		out = append(out, dahua.Recording{
			Channel:     channel,
			StartTime:   r.StartTime,
			EndTime:     r.EndTime,
			Type:        "mp4",
			VideoStream: "Main",
			Length:      r.Size,
			Events:      []string{r.Type},
		})
	}
	return out, nil
}

// LiveRTSPURL builds the verified Raysharp 1-based RTSP URL:
// rtsp://user:pass@host:554/rtsp/streaming?channel=<1..N>&subtype=<0|1|2>
func LiveRTSPURL(host, user, pass string, channel0, stream int) string {
	ch1 := channel0 + 1
	if ch1 <= 0 {
		ch1 = 1
	}
	if stream < 0 || stream > 2 {
		stream = 0
	}
	u := url.URL{
		Scheme:   "rtsp",
		User:     url.UserPassword(user, pass),
		Host:     fmt.Sprintf("%s:554", host),
		Path:     "/rtsp/streaming",
		RawQuery: fmt.Sprintf("channel=%d&subtype=%d", ch1, stream),
	}
	return u.String()
}

// StreamPlayback streams a fragmented MP4 from the Raysharp RTSP endpoint.
func StreamPlayback(ctx context.Context, w io.Writer, host, user, pass string, channel int, start, end time.Time) error {
	dur := end.Sub(start)
	if dur <= 0 || dur > 20*time.Minute {
		dur = 60 * time.Second
	}
	rtsp := LiveRTSPURL(host, user, pass, channel, 0)
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-rtsp_transport", "tcp",
		"-i", rtsp,
		"-t", strconv.Itoa(int(dur.Seconds())),
		"-map", "0:v:0",
		"-c:v", "copy",
		"-tag:v", "hvc1",
		"-an",
		"-fflags", "+genpts",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y", "pipe:1",
	)
	cmd.Stdout = w
	return cmd.Run()
}
