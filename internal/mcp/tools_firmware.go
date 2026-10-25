package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

func registerFirmwareTools(r *Registry, cfg *config.Config, inv *config.Inventory) {
	// 1. kspcam_inspect_firmware
	r.Register(
		Tool{
			Name:        "kspcam_inspect_firmware",
			Description: "Inspect a local Dahua/Imou/KBVision firmware package (.bin / .pkg), verify DH/PK zip CRC32 integrity, extract supported HWIDs, and optionally check compatibility against inventory cameras via upgrader.getUpdateSerial.",
			InputSchema: ToolInputSchema{
				Type:     "object",
				Required: []string{"filePath"},
				Properties: map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "Path to the local firmware .bin or .pkg file",
					},
					"deviceIds": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional list of inventory camera IDs to run live HWID preflight against",
					},
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var params struct {
				FilePath  string   `json:"filePath"`
				DeviceIDs []string `json:"deviceIds,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
			params.FilePath = strings.TrimSpace(params.FilePath)
			if params.FilePath == "" {
				return NewErrorResult("filePath is required"), errors.New("filePath is required")
			}
			data, err := os.ReadFile(params.FilePath)
			if err != nil {
				return NewErrorResult("read firmware file: " + err.Error()), err
			}
			info, err := dahua.InspectFirmwarePackage(data)
			if err != nil {
				return NewErrorResult("invalid firmware package: " + err.Error()), err
			}

			var preflight []map[string]any
			if len(params.DeviceIDs) > 0 && inv != nil {
				to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
				if to <= 0 {
					to = 15 * time.Second
				}
				for _, id := range params.DeviceIDs {
					d, ok := inv.Get(id)
					if !ok {
						preflight = append(preflight, map[string]any{"deviceId": id, "compatible": false, "reason": "device not found in inventory"})
						continue
					}
					if d.Vendor != config.VendorDahua {
						preflight = append(preflight, map[string]any{"deviceId": id, "name": d.Name, "host": d.Host, "compatible": false, "reason": "not a Dahua/KBVision device"})
						continue
					}
					port := d.Port
					if port <= 0 {
						port = 37777
					}
					c, err := dahua.Dial(fmt.Sprintf("%s:%d", d.Host, port), d.Username, d.Password, to)
					if err != nil && errors.Is(err, dahua.ErrDialUnreachable) && port != 8888 {
						c, err = dahua.Dial(fmt.Sprintf("%s:8888", d.Host), d.Username, d.Password, to)
					}
					if err != nil {
						preflight = append(preflight, map[string]any{"deviceId": id, "name": d.Name, "host": d.Host, "compatible": false, "reason": err.Error()})
						continue
					}
					ident, err := c.GetUpgraderIdentity()
					_ = c.Close()
					if err != nil {
						preflight = append(preflight, map[string]any{"deviceId": id, "name": d.Name, "host": d.Host, "compatible": false, "reason": err.Error()})
						continue
					}
					compat := info.SupportsHWID(ident.UpdateSerial.Serial)
					preflight = append(preflight, map[string]any{
						"deviceId":       id,
						"name":           d.Name,
						"host":           d.Host,
						"compatible":     compat,
						"hwid":           ident.UpdateSerial.Serial,
						"sn":             ident.UpdateSerial.SN,
						"currentVersion": ident.Software.Version,
					})
				}
			}

			return NewJSONResult(map[string]any{
				"filePath":  params.FilePath,
				"info":      info,
				"preflight": preflight,
			})
		},
	)

	// 2. kspcam_upgrade_firmware
	r.Register(
		Tool{
			Name:        "kspcam_upgrade_firmware",
			Description: "Perform a sequential Dahua/KBVision/Imou firmware dry-run upload or actual flash over DVRIP (37777/8888) with HWID enforcement, config backup, and RTSP 554 restoration.",
			InputSchema: ToolInputSchema{
				Type:     "object",
				Required: []string{"filePath", "deviceIds"},
				Properties: map[string]any{
					"filePath": map[string]any{
						"type":        "string",
						"description": "Path to the local firmware .bin or .pkg file",
					},
					"deviceIds": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of target inventory device IDs",
					},
					"dryRun": map[string]any{
						"type":        "boolean",
						"description": "If true (default), uploads 100% of chunks and calls upgrader.cancel without flashing.",
					},
					"confirmed": map[string]any{
						"type":        "boolean",
						"description": "Must be true when dryRun is false to authorize actual Flash/Bootloader write.",
					},
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var params struct {
				FilePath  string   `json:"filePath"`
				DeviceIDs []string `json:"deviceIds"`
				DryRun    *bool    `json:"dryRun,omitempty"`
				Confirmed bool     `json:"confirmed,omitempty"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
			if len(params.DeviceIDs) == 0 {
				return NewErrorResult("deviceIds is required"), errors.New("deviceIds is required")
			}
			dryRun := true
			if params.DryRun != nil {
				dryRun = *params.DryRun
			}
			if !dryRun && !params.Confirmed {
				return NewErrorResult("confirmed=true is required when dryRun=false to execute firmware flash"), errors.New("confirmation required")
			}
			data, err := os.ReadFile(strings.TrimSpace(params.FilePath))
			if err != nil {
				return NewErrorResult("read firmware file: " + err.Error()), err
			}
			info, err := dahua.InspectFirmwarePackage(data)
			if err != nil {
				return NewErrorResult("invalid firmware package: " + err.Error()), err
			}

			var results []map[string]any
			for _, id := range params.DeviceIDs {
				d, ok := inv.Get(id)
				if !ok {
					results = append(results, map[string]any{"deviceId": id, "ok": false, "err": "not found in inventory"})
					continue
				}
				port := d.Port
				if port <= 0 {
					port = 37777
				}
				c, err := dahua.Dial(fmt.Sprintf("%s:%d", d.Host, port), d.Username, d.Password, 25*time.Second)
				if err != nil && errors.Is(err, dahua.ErrDialUnreachable) && port != 8888 {
					c, err = dahua.Dial(fmt.Sprintf("%s:8888", d.Host), d.Username, d.Password, 25*time.Second)
				}
				if err != nil {
					results = append(results, map[string]any{"deviceId": id, "host": d.Host, "ok": false, "err": err.Error()})
					continue
				}
				ident, err := c.RunFirmwareSession(ctx, data, info, dryRun, nil)
				_ = c.Close()
				if err != nil {
					results = append(results, map[string]any{"deviceId": id, "host": d.Host, "ok": false, "err": err.Error()})
					continue
				}
				res := map[string]any{
					"deviceId":      id,
					"host":          d.Host,
					"ok":            true,
					"dryRun":        dryRun,
					"beforeVersion": ident.Software.Version,
					"sn":            ident.UpdateSerial.SN,
				}
				results = append(results, res)
			}

			return NewJSONResult(map[string]any{
				"dryRun":  dryRun,
				"info":    info,
				"results": results,
			})
		},
	)
}
