package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func registerCameraInventoryTools(r *Registry, cfg *config.Config, inv *config.Inventory) {
	// 1. kspcam_list_cameras
	r.Register(Tool{
		Name:        "kspcam_list_cameras",
		Description: "List all cameras currently registered in the kspcam inventory (cameras.yaml), including host, port, vendor, serial number, and NVR link status.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"vendor": map[string]any{
					"type":        "string",
					"enum":        []string{"hikvision", "dahua", "tiandy"},
					"description": "Filter cameras by vendor family",
				},
				"isNvr": map[string]any{
					"type":        "boolean",
					"description": "Filter to only NVR devices or standalone cameras",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			Vendor string `json:"vendor,omitempty"`
			IsNVR  *bool  `json:"isNvr,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		devices := inv.List()
		filtered := make([]map[string]any, 0, len(devices))
		for _, d := range devices {
			if req.Vendor != "" && !strings.EqualFold(string(d.Vendor), req.Vendor) {
				continue
			}
			if req.IsNVR != nil && d.IsNVR != *req.IsNVR {
				continue
			}

			// Mask password for safety in tool output
			filtered = append(filtered, map[string]any{
				"id":                  d.ID,
				"name":                d.Name,
				"host":                d.Host,
				"port":                d.Port,
				"vendor":              d.Vendor,
				"username":            d.Username,
				"serialNumber":        d.SerialNumber,
				"nvrId":               d.NVRID,
				"nvrChannel":          d.NVRChannel,
				"nvrName":             d.NVRName,
				"noStorage":           d.NoStorage,
				"isNvr":               d.IsNVR,
				"nvrWatchdog":         d.NVRWatchdog,
				"nvrSyncTimeFromHost": d.NVRSyncTimeFromHost,
			})
		}

		return NewJSONResult(filtered)
	})

	// 2. kspcam_upsert_camera
	r.Register(Tool{
		Name:        "kspcam_upsert_camera",
		Description: "Add a new camera or update an existing camera in the inventory. Passwords are automatically encrypted at rest using AES-256-GCM in cameras.yaml.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"host", "port", "vendor"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Unique device ID. Defaults to host:port if omitted.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Human-readable label for camera location (e.g. 'Bàn 8 VIP')",
				},
				"host": map[string]any{
					"type":        "string",
					"description": "IP address or hostname of camera",
				},
				"port": map[string]any{
					"type":        "integer",
					"description": "Control port (37777/8888 for Dahua, 80 for Hikvision ISAPI, 554 for Tiandy)",
				},
				"vendor": map[string]any{
					"type":        "string",
					"enum":        []string{"hikvision", "dahua", "tiandy"},
					"description": "Vendor family identifier",
				},
				"username": map[string]any{
					"type":        "string",
					"description": "Admin username (default: admin)",
				},
				"password": map[string]any{
					"type":        "string",
					"description": "Plaintext device password (will be encrypted at rest)",
				},
				"serialNumber": map[string]any{
					"type":        "string",
					"description": "Hardware serial number if known",
				},
				"nvrId": map[string]any{
					"type":        "string",
					"description": "Fallback NVR device ID",
				},
				"nvrChannel": map[string]any{
					"type":        "integer",
					"description": "1-based channel on the fallback NVR",
				},
				"nvrName": map[string]any{
					"type":        "string",
					"description": "Display name of NVR channel",
				},
				"noStorage": map[string]any{
					"type":        "boolean",
					"description": "True if camera has no local SD card and relies on NVR",
				},
				"isNvr": map[string]any{
					"type":        "boolean",
					"description": "True if this device is an NVR",
				},
				"nvrWatchdog": map[string]any{
					"type":        "boolean",
					"description": "Enable automated recording health repair watchdog",
				},
				"nvrSyncTimeFromHost": map[string]any{
					"type":        "boolean",
					"description": "Enable host clock synchronization watchdog",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID                  string `json:"id,omitempty"`
			Name                string `json:"name,omitempty"`
			Host                string `json:"host"`
			Port                int    `json:"port"`
			Vendor              string `json:"vendor"`
			Username            string `json:"username,omitempty"`
			Password            string `json:"password,omitempty"`
			SerialNumber        string `json:"serialNumber,omitempty"`
			NVRID               string `json:"nvrId,omitempty"`
			NVRChannel          int    `json:"nvrChannel,omitempty"`
			NVRName             string `json:"nvrName,omitempty"`
			NoStorage           bool   `json:"noStorage,omitempty"`
			IsNVR               bool   `json:"isNvr,omitempty"`
			NVRWatchdog         bool   `json:"nvrWatchdog,omitempty"`
			NVRSyncTimeFromHost bool   `json:"nvrSyncTimeFromHost,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}

		req.Host = strings.TrimSpace(req.Host)
		if req.Host == "" {
			return NewErrorResult("host is required"), fmt.Errorf("host is required")
		}
		if req.Port <= 0 || req.Port > 65535 {
			return NewErrorResult("valid port (1-65535) is required"), fmt.Errorf("invalid port")
		}

		vendor := config.Vendor(strings.ToLower(strings.TrimSpace(req.Vendor)))
		switch vendor {
		case config.VendorDahua, config.VendorHikvision, config.VendorTiandy:
			// valid
		default:
			return NewErrorResult(fmt.Sprintf("unsupported vendor %q (must be dahua, hikvision, or tiandy)", req.Vendor)), fmt.Errorf("unsupported vendor")
		}

		id := strings.TrimSpace(req.ID)
		if id == "" {
			id = fmt.Sprintf("%s:%d", req.Host, req.Port)
		}

		dev := config.Device{
			ID:                  id,
			Name:                strings.TrimSpace(req.Name),
			Host:                req.Host,
			Port:                req.Port,
			Vendor:              vendor,
			Username:            strings.TrimSpace(req.Username),
			Password:            req.Password,
			SerialNumber:        strings.TrimSpace(req.SerialNumber),
			NVRID:               strings.TrimSpace(req.NVRID),
			NVRChannel:          req.NVRChannel,
			NVRName:             strings.TrimSpace(req.NVRName),
			NoStorage:           req.NoStorage,
			IsNVR:               req.IsNVR,
			NVRWatchdog:         req.NVRWatchdog,
			NVRSyncTimeFromHost: req.NVRSyncTimeFromHost,
		}

		if dev.Username == "" {
			if existing, ok := inv.Get(dev.ID); ok && existing.Username != "" {
				dev.Username = existing.Username
			} else if cfg.Defaults.Username != "" {
				dev.Username = cfg.Defaults.Username
			} else {
				dev.Username = "admin"
			}
		}

		if dev.Password == "" {
			if existing, ok := inv.Get(dev.ID); ok && existing.Password != "" {
				dev.Password = existing.Password
			} else if cfg.Defaults.Password != "" {
				dev.Password = cfg.Defaults.Password
			}
		}

		if err := inv.Upsert(dev); err != nil {
			return NewErrorResult("failed to save camera: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status": "success",
			"device": map[string]any{
				"id":     dev.ID,
				"name":   dev.Name,
				"host":   dev.Host,
				"port":   dev.Port,
				"vendor": dev.Vendor,
			},
		})
	})

	// 3. kspcam_delete_camera
	r.Register(Tool{
		Name:        "kspcam_delete_camera",
		Description: "Remove one camera device from the inventory.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID to delete (e.g. '192.168.1.108:37777')",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}

		if _, ok := inv.Get(req.ID); !ok {
			return NewErrorResult(fmt.Sprintf("device %q not found in inventory", req.ID)), fmt.Errorf("device not found")
		}

		if err := inv.Delete(req.ID); err != nil {
			return NewErrorResult("failed to delete device: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status":    "success",
			"deletedId": req.ID,
		})
	})

	// 4. kspcam_probe_camera
	r.Register(Tool{
		Name:        "kspcam_probe_camera",
		Description: "Connect live to camera hardware and probe stream encoding capabilities (resolutions, codec, FPS, GOP, bitrate, audio), channel title, OSD overlay lines, and hardware serial number.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID in inventory",
				},
				"timeoutSeconds": map[string]any{
					"type":        "integer",
					"description": "Probe timeout in seconds (default: 30, min: 5, max: 600)",
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

		probeCtx, cancel := context.WithTimeout(ctx, to)
		defer cancel()

		cam, err := camera.Open(probeCtx, dev, to)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("failed to connect to camera %s: %v", req.ID, err)), err
		}
		defer cam.Close()

		streams, err := cam.Probe(probeCtx)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("probe failed on %s: %v", req.ID, err)), err
		}

		serial := dev.SerialNumber
		if idCam, ok := cam.(camera.DeviceIdentity); ok {
			if s, sErr := idCam.GetSerialNumber(probeCtx); sErr == nil && s != "" {
				serial = s
				if dev.SerialNumber != serial {
					dev.SerialNumber = serial
					_ = inv.Upsert(dev)
				}
			}
		}

		return NewJSONResult(map[string]any{
			"deviceId":     dev.ID,
			"serialNumber": serial,
			"streams":      streams,
		})
	})
}
