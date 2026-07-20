// Package importer parses external camera lists (currently Shinobi monitor
// config) into the tool's inventory, auto-detecting vendor and credentials from
// the RTSP URL of each monitor ("nhìn rtsp là biết").
package importer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// shinobiMonitor is the subset of a Shinobi monitor object we read. Details is
// raw because the live Shinobi API returns it as a JSON-ENCODED STRING, while an
// exported/pretty JSON has it as a nested object — we handle both.
type shinobiMonitor struct {
	Name    string          `json:"name"`
	Mid     string          `json:"mid"`
	Host    string          `json:"host"`
	Details json.RawMessage `json:"details"`
}

// shinobiDetails is the subset of a monitor's details we use.
type shinobiDetails struct {
	AutoHost string `json:"auto_host"` // rtsp://user:pass@host:port/path
	Muser    string `json:"muser"`
	Mpass    string `json:"mpass"`
}

// parseDetails decodes a monitor's details whether it's a JSON object or a
// JSON-encoded string (Shinobi's API returns the latter).
func parseDetails(raw json.RawMessage) shinobiDetails {
	var d shinobiDetails
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return d
	}
	if raw[0] == '"' { // a JSON string containing JSON
		var s string
		if json.Unmarshal(raw, &s) == nil {
			_ = json.Unmarshal([]byte(s), &d)
		}
		return d
	}
	_ = json.Unmarshal(raw, &d)
	return d
}

// Result reports what a parse produced.
type Result struct {
	Devices []config.Device
	Skipped int
}

// ParseShinobi parses a Shinobi monitor-config JSON (a bare array, or an object
// with a "monitors"/"data" array) into devices. hikPort/dahuaPort are the
// config ports assigned per detected vendor (the RTSP port, usually 554, is not
// the config port). Monitors with no resolvable host are skipped.
func ParseShinobi(data []byte, hikPort, dahuaPort int) (Result, error) {
	mons, err := unwrap(data)
	if err != nil {
		return Result{}, err
	}
	var res Result
	for _, m := range mons {
		det := parseDetails(m.Details)
		host, user, pass := m.Host, det.Muser, det.Mpass
		var path string
		if u, err := url.Parse(strings.TrimSpace(det.AutoHost)); err == nil && u.Host != "" {
			host = u.Hostname()
			path = u.Path
			if u.User != nil {
				user = u.User.Username()
				if p, ok := u.User.Password(); ok {
					pass = p
				}
			}
		}
		if host == "" {
			res.Skipped++
			continue
		}
		vendor := detectVendor(path)
		port := hikPort
		if vendor == config.VendorDahua {
			port = dahuaPort
		}
		name := m.Name
		if name == "" {
			name = m.Mid
		}
		res.Devices = append(res.Devices, config.Device{
			Name:       name,
			Host:       host,
			Port:       port,
			Vendor:     vendor,
			Username:   user,
			Password:   pass,
			NVRChannel: parseChannel(det.AutoHost, path),
		})
	}
	// An NVR's channels arrive as several monitors sharing one host:port (e.g.
	// rtsp://nvr/1/1, rtsp://nvr/2/1 …). Keyed only by host:port they'd collapse
	// to one device on import — so give each a distinct id (host:port-cN) by
	// channel. Single-monitor hosts keep the plain host:port id.
	byAddr := map[string][]int{}
	for i := range res.Devices {
		byAddr[res.Devices[i].Addr()] = append(byAddr[res.Devices[i].Addr()], i)
	}
	for _, idxs := range byAddr {
		if len(idxs) < 2 {
			continue
		}
		for seq, i := range idxs {
			d := &res.Devices[i]
			if d.NVRChannel <= 0 {
				d.NVRChannel = seq + 1 // fall back to arrival order when unparsable
			}
			d.ID = fmt.Sprintf("%s-c%d", d.Addr(), d.NVRChannel)
			if d.Name == "" {
				d.Name = fmt.Sprintf("Kênh %d", d.NVRChannel)
			}
		}
	}
	return res, nil
}

var (
	shortChannelRe = regexp.MustCompile(`^/(\d+)/\d+/?$`) // Tiandy NVR: /channel/stream
	hikChannelRe   = regexp.MustCompile(`(?i)/streaming/channels/(\d+)`)
)

// parseChannel extracts the NVR/camera channel from a monitor's RTSP URL so
// several monitors sharing one host become distinct cameras: ?channel=N
// (Dahua/Tiandy realmonitor), the short /N/M path (Tiandy NVR), or
// /Streaming/Channels/<id> (Hik, id = channel*100+stream). 0 when none.
func parseChannel(rawURL, path string) int {
	if u, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
		if n, err := strconv.Atoi(u.Query().Get("channel")); err == nil && n > 0 {
			return n
		}
	}
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
	return 0
}

// unwrap accepts a bare array or a wrapping object ({"monitors":[...]} etc).
func unwrap(data []byte) ([]shinobiMonitor, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) > 0 && data[0] == '[' {
		var arr []shinobiMonitor
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, fmt.Errorf("parse shinobi array: %w", err)
		}
		return arr, nil
	}
	var obj struct {
		Monitors []shinobiMonitor `json:"monitors"`
		Data     []shinobiMonitor `json:"data"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("parse shinobi json: %w", err)
	}
	if len(obj.Monitors) > 0 {
		return obj.Monitors, nil
	}
	return obj.Data, nil
}

// detectVendor infers the camera vendor from the RTSP path. Hikvision uses
// /Streaming/Channels/<n>; Dahua/KBVision uses /cam/realmonitor. Unknown paths
// default to Hikvision (the most common), which the user can change on review.
func detectVendor(path string) config.Vendor {
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
