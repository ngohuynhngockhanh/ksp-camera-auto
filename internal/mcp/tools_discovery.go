package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/discovery"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/nvrhealth"
)

func registerDiscoveryDiagnosisTools(r *Registry, cfg *config.Config, inv *config.Inventory) {
	// 10. kspcam_scan_lan
	r.Register(Tool{
		Name:        "kspcam_scan_lan",
		Description: "Discover IP cameras on the local network (via ONVIF WS-Discovery UDP 3702, Dahua DHDiscover UDP 37810, Hikvision SADP UDP 37020) or across routed subnets (via Nmap TCP port scan).",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"method": map[string]any{
					"type":        "string",
					"enum":        []string{"auto", "all", "onvif", "dahua", "sadp", "hikvision", "nmap"},
					"description": "Discovery protocol. 'auto' or 'all' broadcasts all 3 UDP protocols.",
				},
				"subnet": map[string]any{
					"type":        "string",
					"description": "Subnet in CIDR format (e.g. '192.168.1.0/24') for Nmap scans",
				},
				"timeoutSeconds": map[string]any{
					"type":        "integer",
					"description": "Scan timeout in seconds (default 5)",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			Method         string `json:"method,omitempty"`
			Subnet         string `json:"subnet,omitempty"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		to := 5 * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}

		scanCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		method := strings.ToLower(strings.TrimSpace(req.Method))
		if method == "" || method == "auto" || method == "all" {
			results, err := discovery.Scan(scanCtx, to)
			if err != nil {
				return NewErrorResult("discovery scan failed: " + err.Error()), err
			}
			return NewJSONResult(results)
		}

		if method == "nmap" {
			if strings.TrimSpace(req.Subnet) == "" {
				return NewErrorResult("subnet is required for nmap scan (e.g. '192.168.1.0/24')"), fmt.Errorf("missing subnet")
			}
			results, err := discovery.ScanSubnet(scanCtx, req.Subnet)
			if err != nil {
				return NewErrorResult("nmap scan failed: " + err.Error()), err
			}
			return NewJSONResult(results)
		}

		// Filter for specific UDP method
		targetVia := method
		if method == "sadp" || method == "hikvision" {
			targetVia = "hikvision-sadp"
		}

		all, err := discovery.Scan(scanCtx, to)
		if err != nil {
			return NewErrorResult("scan failed: " + err.Error()), err
		}

		filtered := make([]discovery.Result, 0, len(all))
		for _, item := range all {
			if item.Via == targetVia {
				filtered = append(filtered, item)
			}
		}

		return NewJSONResult(filtered)
	})

	// 11. kspcam_try_password
	r.Register(Tool{
		Name:        "kspcam_try_password",
		Description: "Test a matrix of usernames and passwords sequentially against discovered cameras to verify working credentials.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"devices"},
			Properties: map[string]any{
				"devices": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":     "object",
						"required": []string{"ip", "vendor"},
						"properties": map[string]any{
							"ip":     map[string]any{"type": "string"},
							"port":   map[string]any{"type": "integer"},
							"vendor": map[string]any{"type": "string", "enum": []string{"hikvision", "dahua", "tiandy"}},
							"label":  map[string]any{"type": "string"},
						},
					},
				},
				"credentials": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":     "object",
						"required": []string{"username", "password"},
						"properties": map[string]any{
							"username": map[string]any{"type": "string"},
							"password": map[string]any{"type": "string"},
						},
					},
				},
				"targets": map[string]any{
					"type":        "array",
					"description": "Alias for devices",
				},
				"username": map[string]any{
					"type":        "string",
					"description": "Single candidate username (if credentials array omitted)",
				},
				"password": map[string]any{
					"type":        "string",
					"description": "Single candidate password (if credentials array omitted)",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		type CredPair struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		var req struct {
			Devices        []bulk.CredTestTarget `json:"devices"`
			Targets        []bulk.CredTestTarget `json:"targets"`
			Credentials    []CredPair            `json:"credentials"`
			Username       string                `json:"username"`
			Password       string                `json:"password"`
			TimeoutSeconds int                   `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}

		targets := req.Devices
		if len(targets) == 0 {
			targets = req.Targets
		}
		if len(targets) == 0 {
			return NewErrorResult("devices/targets list is required"), fmt.Errorf("empty targets")
		}

		creds := req.Credentials
		if len(creds) == 0 && req.Username != "" {
			creds = []CredPair{{Username: req.Username, Password: req.Password}}
		}
		if len(creds) == 0 {
			creds = []CredPair{{Username: cfg.Defaults.Username, Password: cfg.Defaults.Password}}
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 10 * time.Second
		}

		var allEvents []bulk.CredTestEvent
		for _, c := range creds {
			evs := bulk.TryPasswords(ctx, targets, c.Username, c.Password, cfg.Defaults, to, nil)
			allEvents = append(allEvents, evs...)
		}

		return NewJSONResult(allEvents)
	})

	// 12. kspcam_wifi_scan
	r.Register(Tool{
		Name:        "kspcam_wifi_scan",
		Description: "Trigger an over-the-air Wi-Fi scan on a wireless camera to inspect available Access Points (SSID, signal strength RSSI, auth mode).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		dev, ok := inv.Get(req.ID)
		if !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 30 * time.Second
		}

		callCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(callCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		ns, ok := cam.(camera.NetworkSettings)
		if !ok {
			return NewErrorResult("camera does not support Wi-Fi scanning"), fmt.Errorf("unsupported wifi scan")
		}

		aps, err := ns.ScanWiFi(callCtx)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("wifi scan failed: %v", err)), err
		}

		return NewJSONResult(aps)
	})

	// 13. kspcam_get_network
	r.Register(Tool{
		Name:        "kspcam_get_network",
		Description: "Retrieve network interface configurations (IP, Subnet Mask, Gateway, DNS, DHCP state) from a camera.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		dev, ok := inv.Get(req.ID)
		if !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 30 * time.Second
		}

		callCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(callCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		ns, ok := cam.(camera.NetworkSettings)
		if !ok {
			return NewErrorResult("camera does not support reading network configuration"), fmt.Errorf("unsupported network settings")
		}

		netCfg, err := ns.GetNetworkConfig(callCtx)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("get network config failed: %v", err)), err
		}

		return NewJSONResult(netCfg)
	})

	// 14. kspcam_get_nvr_health
	r.Register(Tool{
		Name:        "kspcam_get_nvr_health",
		Description: "Check the recording health of an NVR or linked camera (timing record state, uptime, storage disk status, gaps).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "NVR or Camera Device ID",
				},
				"forceCheck": map[string]any{
					"type":        "boolean",
					"description": "Force fresh probe ignoring cache",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			ForceCheck     bool   `json:"forceCheck,omitempty"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		dev, ok := inv.Get(req.ID)
		if !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 30 * time.Second
		}

		callCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(callCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		healthReport := map[string]any{
			"nvrId":            dev.ID,
			"reachable":        true,
			"watchdogEnabled":  dev.NVRWatchdog,
			"syncTimeFromHost": dev.NVRSyncTimeFromHost,
		}

		if sm, ok := cam.(camera.StorageManager); ok {
			if disks, err := sm.GetStorageInfo(callCtx); err == nil {
				healthReport["disks"] = disks
				healthy, total, used := nvrhealth.StorageHealth(disks)
				healthReport["storageHealthy"] = healthy
				healthReport["storageTotalBytes"] = total
				healthReport["storageUsedBytes"] = used
			}
		}

		if hc, ok := cam.(camera.NVRHealthConfig); ok {
			if uptime, err := hc.GetUptime(callCtx); err == nil {
				healthReport["uptimeSeconds"] = int64(uptime.Seconds())
				healthReport["uptimeMinutes"] = int64(uptime.Minutes())
			}

			channelCount := 0
			if rdl, ok := cam.(camera.RemoteDeviceLister); ok {
				if remotes, err := rdl.GetRemoteDevices(callCtx); err == nil {
					healthReport["remoteChannels"] = remotes
					for _, ch := range remotes {
						if ch.Channel+1 > channelCount {
							channelCount = ch.Channel + 1
						}
					}
				}
			}
			if channelCount == 0 {
				channelCount = 16
			}

			if states, err := hc.GetRecordState(callCtx, channelCount); err == nil {
				healthReport["channels"] = states
			}
		}

		if tc, ok := cam.(camera.DeviceTimeConfig); ok {
			if tcfg, err := tc.GetTimeConfig(callCtx); err == nil {
				healthReport["deviceTime"] = tcfg
			}
		}

		return NewJSONResult(healthReport)
	})

	// 15. kspcam_get_recordings
	r.Register(Tool{
		Name:        "kspcam_get_recordings",
		Description: "Query stored recording segments on a camera or NVR channel for a specific date/time range.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "startTime", "endTime"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index",
				},
				"startTime": map[string]any{
					"type":        "string",
					"description": "Start timestamp in ISO 8601 / RFC 3339 format",
				},
				"endTime": map[string]any{
					"type":        "string",
					"description": "End timestamp in ISO 8601 / RFC 3339 format",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			Channel        int    `json:"channel,omitempty"`
			StartTime      string `json:"startTime"`
			EndTime        string `json:"endTime"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		start, err := parseFlexTime(req.StartTime)
		if err != nil {
			return NewErrorResult("invalid startTime: " + err.Error()), err
		}
		end, err := parseFlexTime(req.EndTime)
		if err != nil {
			return NewErrorResult("invalid endTime: " + err.Error()), err
		}

		dev, ok := inv.Get(req.ID)
		if !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 30 * time.Second
		}

		callCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(callCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		rec, ok := cam.(camera.Recorder)
		if !ok {
			return NewErrorResult("camera does not support querying recordings"), fmt.Errorf("unsupported recorder")
		}

		segments, err := rec.FindRecordings(callCtx, req.Channel, start, end)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("find recordings failed: %v", err)), err
		}

		return NewJSONResult(segments)
	})

	// 16. kspcam_get_snapshot
	r.Register(Tool{
		Name:        "kspcam_get_snapshot",
		Description: "Fetch a single live JPEG snapshot from a camera channel and return it formatted as base64 image data.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index",
				},
				"stream": map[string]any{
					"type":        "integer",
					"description": "0=Main, 1=Sub1",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			Channel        int    `json:"channel,omitempty"`
			Stream         int    `json:"stream,omitempty"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		dev, ok := inv.Get(req.ID)
		if !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 15 * time.Second
		}

		callCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(callCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		jpegBytes, err := cam.Snapshot(callCtx, req.Channel, req.Stream)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("snapshot failed: %v", err)), err
		}

		b64 := base64.StdEncoding.EncodeToString(jpegBytes)
		return NewImageResult("image/jpeg", b64), nil
	})
}

func parseFlexTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format %q", s)
}
