package shinobi

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

var (
	shortChannelRe = regexp.MustCompile(`^/(\d+)/\d+/?$`)
	hikChannelRe   = regexp.MustCompile(`(?i)/streaming/channels/(\d+)`)
	sanitizerRe    = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
)

// DeviceToMid generates a URL-safe, deterministic Shinobi monitor ID from a config.Device.
func DeviceToMid(d config.Device) string {
	sanitizedHost := sanitizerRe.ReplaceAllString(d.Host, "_")
	if d.NVRChannel > 0 {
		return fmt.Sprintf("cam_%s_c%d", sanitizedHost, d.NVRChannel)
	}
	if d.Port > 0 {
		return fmt.Sprintf("cam_%s_%d", sanitizedHost, d.Port)
	}
	return fmt.Sprintf("cam_%s", sanitizedHost)
}

// BuildMonitorConfig builds a Shinobi MonitorConfig from an inventory Device.
func (c *Client) BuildMonitorConfig(d config.Device) MonitorConfig {
	mid := DeviceToMid(d)
	name := strings.TrimSpace(d.Name)
	if name == "" {
		name = d.ID
	}

	channel := 1
	if d.NVRChannel > 0 {
		channel = d.NVRChannel
	}

	var path string
	switch d.Vendor {
	case config.VendorDahua:
		path = fmt.Sprintf("/cam/realmonitor?channel=%d&subtype=0", channel)
	case config.VendorHikvision:
		path = fmt.Sprintf("/Streaming/Channels/%d", channel*100+1)
	case config.VendorTiandy:
		path = fmt.Sprintf("/cam/realmonitor?channel=%d&subtype=0", channel)
	default:
		// Default to Dahua/Hik standard path
		path = fmt.Sprintf("/cam/realmonitor?channel=%d&subtype=0", channel)
	}

	var autoHost string
	if d.Username != "" && d.Password != "" {
		autoHost = fmt.Sprintf("rtsp://%s:%s@%s:554%s",
			url.QueryEscape(d.Username),
			url.QueryEscape(d.Password),
			d.Host,
			path)
	} else if d.Username != "" {
		autoHost = fmt.Sprintf("rtsp://%s@%s:554%s",
			url.QueryEscape(d.Username),
			d.Host,
			path)
	} else {
		autoHost = fmt.Sprintf("rtsp://%s:554%s", d.Host, path)
	}

	return MonitorConfig{
		Mid:      mid,
		Ke:       c.groupKey,
		Name:     name,
		Type:     "h264",
		Mode:     "record",
		Host:     d.Host,
		Port:     "554",
		Protocol: "rtsp",
		Path:     path,
		Ext:      "mp4",
		Details: MonitorDetails{
			AutoHost:           autoHost,
			Muser:              d.Username,
			Mpass:              d.Password,
			Port:               "554",
			Protocol:           "rtsp",
			StreamType:         "hls",
			StreamFlvType:      "ws",
			StreamVcodec:       "copy",
			StreamAcodec:       "copy",
			Vcodec:             "copy",
			Acodec:             "copy",
			RecordVcodec:       "copy",
			RecordAcodec:       "aac",
			CustInput:          "",            // Input flag để trống
			CustStream:         "",            // Stream flag để trống
			CustRecord:         "-tag:v hvc1", // Recording flag chuẩn H.265 MP4
			Detector:           "0",
		},
	}
}

// SyncToShinobi pushes all cameras from the inventory to Shinobi monitors.
// (Manual trigger: cameras.yaml -> Shinobi monitors)
func (c *Client) SyncToShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error) {
	if inv == nil {
		return nil, fmt.Errorf("inventory is nil")
	}

	report := &SyncReport{
		Direction: "to_shinobi",
		Errors:    make([]string, 0),
	}

	existingMons, err := c.ListMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch existing monitors from shinobi: %w", err)
	}

	existingByMid := make(map[string]Monitor, len(existingMons))
	for _, m := range existingMons {
		existingByMid[m.Mid] = m
	}

	devices := inv.List()
	for _, d := range devices {
		monCfg := c.BuildMonitorConfig(d)
		existing, found := existingByMid[monCfg.Mid]

		if !found {
			if err := c.AddMonitor(ctx, monCfg); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("add monitor %s (%s): %v", monCfg.Mid, d.Host, err))
			} else {
				report.Created++
			}
			continue
		}

		// Check if config has changed
		existingDet := existing.ParseDetails()
		changed := existing.Name != monCfg.Name ||
			existing.Host != monCfg.Host ||
			existing.Path != monCfg.Path ||
			existingDet.AutoHost != monCfg.Details.AutoHost ||
			existingDet.Muser != monCfg.Details.Muser ||
			existingDet.Mpass != monCfg.Details.Mpass

		if changed {
			if err := c.EditMonitor(ctx, monCfg.Mid, monCfg); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("edit monitor %s (%s): %v", monCfg.Mid, d.Host, err))
			} else {
				report.Updated++
			}
		} else {
			report.Unchanged++
		}
	}

	return report, nil
}

// SyncFromShinobi pulls all monitors from Shinobi and imports/updates cameras in the inventory.
// (Manual trigger: Shinobi monitors -> cameras.yaml)
func (c *Client) SyncFromShinobi(ctx context.Context, inv *config.Inventory) (*SyncReport, error) {
	if inv == nil {
		return nil, fmt.Errorf("inventory is nil")
	}

	report := &SyncReport{
		Direction: "from_shinobi",
		Errors:    make([]string, 0),
	}

	mons, err := c.ListMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch monitors from shinobi: %w", err)
	}

	type parsedItem struct {
		dev        config.Device
		nvrChannel int
	}

	var candidates []parsedItem

	for _, m := range mons {
		det := m.ParseDetails()
		host := strings.TrimSpace(m.Host)
		user := strings.TrimSpace(det.Muser)
		pass := strings.TrimSpace(det.Mpass)
		path := strings.TrimSpace(m.Path)

		if det.AutoHost != "" {
			if u, err := url.Parse(strings.TrimSpace(det.AutoHost)); err == nil && u.Host != "" {
				if h := u.Hostname(); h != "" {
					host = h
				}
				if u.Path != "" {
					path = u.Path
				}
				if u.User != nil {
					user = u.User.Username()
					if p, ok := u.User.Password(); ok {
						pass = p
					}
				}
			}
		}

		if host == "" {
			report.Skipped++
			continue
		}

		vendor := detectVendorFromPath(path)
		configPort := 8000
		if vendor == config.VendorDahua {
			configPort = 37777
		} else if vendor == config.VendorTiandy {
			configPort = 554
		}

		// Check if host already exists in inventory with a different port (e.g. 8888 fallback)
		if existing, ok := inv.FindByHost(host, 0); ok && existing.Port > 0 {
			configPort = existing.Port
		}

		nvrChannel := parseChannelFromURLAndPath(det.AutoHost, path)

		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = m.Mid
		}

		candidates = append(candidates, parsedItem{
			dev: config.Device{
				Name:       name,
				Host:       host,
				Port:       configPort,
				Vendor:     vendor,
				Username:   user,
				Password:   pass,
				NVRChannel: nvrChannel,
			},
			nvrChannel: nvrChannel,
		})
	}

	// Group candidates by host:port
	byAddr := make(map[string][]int)
	for i := range candidates {
		addr := candidates[i].dev.Addr()
		byAddr[addr] = append(byAddr[addr], i)
	}

	for addr, idxs := range byAddr {
		if len(idxs) >= 2 {
			// Multi-channel host
			for seq, i := range idxs {
				item := &candidates[i]
				ch := item.nvrChannel
				if ch <= 0 {
					ch = seq + 1
				}
				item.dev.NVRChannel = ch
				item.dev.ID = fmt.Sprintf("%s-c%d", addr, ch)
			}
		} else {
			// Single monitor for this host
			item := &candidates[idxs[0]]
			// If existing in inventory with ID = addr, or with -c1
			if existing, ok := inv.Get(addr); ok {
				item.dev.ID = existing.ID
				item.dev.NVRChannel = existing.NVRChannel
			} else if item.nvrChannel > 1 {
				item.dev.ID = fmt.Sprintf("%s-c%d", addr, item.nvrChannel)
			} else if existing, ok := inv.FindByHost(item.dev.Host, item.dev.Port); ok {
				item.dev.ID = existing.ID
				item.dev.NVRChannel = existing.NVRChannel
			} else {
				item.dev.ID = addr
				if item.nvrChannel == 1 {
					item.dev.NVRChannel = 0 // single camera default
				}
			}
		}
	}

	for _, item := range candidates {
		dev := item.dev
		existingDev, found := inv.Get(dev.ID)
		if !found {
			if fallback, ok := inv.FindByHost(dev.Host, dev.Port); ok && fallback.NVRChannel == dev.NVRChannel {
				existingDev = fallback
				found = true
				dev.ID = fallback.ID
			}
		}

		if found {
			// Preserve existing NVR linking settings
			dev.ID = existingDev.ID
			dev.NVRID = existingDev.NVRID
			dev.NVRName = existingDev.NVRName
			dev.NoStorage = existingDev.NoStorage
			dev.IsNVR = existingDev.IsNVR
			dev.NVRWatchdog = existingDev.NVRWatchdog
			dev.NVRSyncTimeFromHost = existingDev.NVRSyncTimeFromHost
			if dev.Password == "" {
				dev.Password = existingDev.Password
			}
			if dev.Username == "" {
				dev.Username = existingDev.Username
			}
			if err := inv.Upsert(dev); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("update device %s: %v", dev.ID, err))
			} else {
				report.Skipped++
			}
		} else {
			if err := inv.Upsert(dev); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("insert device %s: %v", dev.ID, err))
			} else {
				report.Added++
			}
		}
	}

	return report, nil
}

func detectVendorFromPath(path string) config.Vendor {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "/cam/realmonitor"):
		return config.VendorDahua
	case strings.Contains(p, "/streaming/channels"):
		return config.VendorHikvision
	default:
		return config.VendorHikvision
	}
}

func parseChannelFromURLAndPath(rawURL, path string) int {
	if rawURL != "" {
		if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
			if n, err := strconv.Atoi(u.Query().Get("channel")); err == nil && n > 0 {
				return n
			}
		}
	}
	if path != "" {
		if m := shortChannelRe.FindStringSubmatch(path); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > 0 {
				return n
			}
		}
		if m := hikChannelRe.FindStringSubmatch(path); m != nil {
			if id, _ := strconv.Atoi(m[1]); id >= 100 {
				return id / 100
			} else if id > 0 {
				return id
			}
		}
	}
	return 0
}
