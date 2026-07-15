// Package importer parses external camera lists (currently Shinobi monitor
// config) into the tool's inventory, auto-detecting vendor and credentials from
// the RTSP URL of each monitor ("nhìn rtsp là biết").
package importer

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

// shinobiMonitor is the subset of a Shinobi monitor object we read.
type shinobiMonitor struct {
	Name    string `json:"name"`
	Mid     string `json:"mid"`
	Host    string `json:"host"`
	Details struct {
		AutoHost string `json:"auto_host"` // rtsp://user:pass@host:port/path
		Muser    string `json:"muser"`
		Mpass    string `json:"mpass"`
	} `json:"details"`
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
		host, user, pass := m.Host, m.Details.Muser, m.Details.Mpass
		var path string
		if u, err := url.Parse(strings.TrimSpace(m.Details.AutoHost)); err == nil && u.Host != "" {
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
			Name:     name,
			Host:     host,
			Port:     port,
			Vendor:   vendor,
			Username: user,
			Password: pass,
		})
	}
	return res, nil
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
