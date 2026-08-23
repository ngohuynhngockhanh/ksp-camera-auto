package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/bulk"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/camera"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

func registerCameraConfigTools(r *Registry, cfg *config.Config, inv *config.Inventory) {
	// 5. kspcam_apply_profile
	r.Register(Tool{
		Name:        "kspcam_apply_profile",
		Description: "Apply video encode settings (resolution, codec, FPS, GOP, bitrate, audio AAC, SmartCodec, OSD overlay) across one or multiple cameras in sequential order.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"deviceIds", "profile"},
			Properties: map[string]any{
				"deviceIds": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "List of target device IDs",
				},
				"profile": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"setResolution": map[string]any{"type": "boolean"},
						"width":         map[string]any{"type": "integer"},
						"height":        map[string]any{"type": "integer"},
						"setCodec":      map[string]any{"type": "boolean"},
						"codec":         map[string]any{"type": "string", "enum": []string{"H.264", "H.264H", "H.264B", "H.265", "MJPG"}},
						"codecProfile":  map[string]any{"type": "string", "enum": []string{"Main", "High", "Baseline"}},
						"setFps":        map[string]any{"type": "boolean"},
						"fps":           map[string]any{"type": "integer"},
						"setGop":        map[string]any{"type": "boolean"},
						"gop":           map[string]any{"type": "integer"},
						"setBitrate":    map[string]any{"type": "boolean"},
						"bitrate":       map[string]any{"type": "integer"},
						"bitrateMode":   map[string]any{"type": "string", "enum": []string{"CBR", "VBR"}},
						"setAudioAAC":   map[string]any{"type": "boolean"},
						"setSmartCodec": map[string]any{"type": "boolean"},
						"smartCodec":    map[string]any{"type": "boolean"},
						"setOsd":        map[string]any{"type": "boolean"},
						"osdLines": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"streams": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "integer"},
						},
						"channels": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "integer"},
						},
					},
				},
				"timeoutSeconds": map[string]any{
					"type":        "integer",
					"description": "Per-camera timeout in seconds",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			DeviceIDs      []string       `json:"deviceIds"`
			Profile        camera.Profile `json:"profile"`
			TimeoutSeconds int            `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}

		if len(req.DeviceIDs) == 0 {
			return NewErrorResult("deviceIds list is required and cannot be empty"), fmt.Errorf("empty deviceIds")
		}

		to := time.Duration(cfg.Defaults.TimeoutSeconds) * time.Second
		if req.TimeoutSeconds > 0 {
			to = time.Duration(req.TimeoutSeconds) * time.Second
		}
		if to <= 0 {
			to = 30 * time.Second
		}

		bulkReq := bulk.Request{
			DeviceIDs:      req.DeviceIDs,
			Profile:        req.Profile,
			TimeoutSeconds: int(to.Seconds()),
		}

		results := bulk.Apply(ctx, inv, bulkReq, to, nil)
		return NewJSONResult(results)
	})

	// 6. kspcam_set_channel_name
	r.Register(Tool{
		Name:        "kspcam_set_channel_name",
		Description: "Update the on-camera hardware channel title (distinct from the local inventory label).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "name"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index (default 0)",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "New channel name",
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
			Name           string `json:"name"`
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

		if err := cam.SetChannelName(callCtx, req.Channel, req.Name); err != nil {
			return NewErrorResult(fmt.Sprintf("set channel name failed: %v", err)), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"id":      req.ID,
			"channel": req.Channel,
			"name":    req.Name,
		})
	})

	// 7. kspcam_set_osd
	r.Register(Tool{
		Name:        "kspcam_set_osd",
		Description: "Configure up to 4 lines of free-text OSD text overlay and visibility on the camera video.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "lines"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index",
				},
				"lines": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Up to 4 text lines. Use '{name}' to substitute camera inventory name.",
				},
				"enabled": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "boolean"},
					"description": "Visibility flag per line",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string   `json:"id"`
			Channel        int      `json:"channel,omitempty"`
			Lines          []string `json:"lines"`
			Enabled        []bool   `json:"enabled,omitempty"`
			TimeoutSeconds int      `json:"timeoutSeconds,omitempty"`
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

		applied, err := cam.SetOSDLines(callCtx, req.Channel, req.Lines, req.Enabled)
		if err != nil {
			return NewErrorResult(fmt.Sprintf("set OSD lines failed: %v", err)), err
		}

		return NewJSONResult(map[string]any{
			"status":       "success",
			"appliedLines": applied,
		})
	})

	// 8. kspcam_reboot_camera
	r.Register(Tool{
		Name:        "kspcam_reboot_camera",
		Description: "Send a hardware reboot command to a camera or NVR (DVRIP magicBox.reboot or ISAPI /System/reboot).",
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

		rebooter, ok := cam.(camera.Rebooter)
		if !ok {
			return NewErrorResult("camera does not support remote reboot"), fmt.Errorf("unsupported reboot")
		}

		if err := rebooter.Reboot(callCtx); err != nil {
			return NewErrorResult(fmt.Sprintf("reboot failed: %v", err)), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"message": "Reboot command accepted by device",
		})
	})

	// 9. kspcam_change_password
	r.Register(Tool{
		Name:        "kspcam_change_password",
		Description: "Change the administrative password on the physical camera hardware and update the encrypted credentials in cameras.yaml.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "newPassword"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID",
				},
				"newUser": map[string]any{
					"type":        "string",
					"description": "New username if updating user, or omit to keep current",
				},
				"newPassword": map[string]any{
					"type":        "string",
					"description": "New plaintext password",
				},
				"timeoutSeconds": map[string]any{
					"type": "integer",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID             string `json:"id"`
			NewUser        string `json:"newUser,omitempty"`
			NewPassword    string `json:"newPassword"`
			TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.ID = strings.TrimSpace(req.ID)
		if req.ID == "" {
			return NewErrorResult("device id is required"), fmt.Errorf("device id is required")
		}
		if req.NewPassword == "" {
			return NewErrorResult("newPassword cannot be empty"), fmt.Errorf("empty password")
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

		if err := cam.ChangePassword(callCtx, req.NewUser, req.NewPassword); err != nil {
			return NewErrorResult(fmt.Sprintf("change password on camera hardware failed: %v", err)), err
		}

		// Update inventory
		dev.Password = req.NewPassword
		if req.NewUser != "" {
			dev.Username = req.NewUser
		}
		if err := inv.Upsert(dev); err != nil {
			return NewErrorResult("password changed on camera, but failed to save in inventory: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"id":      req.ID,
			"message": "Password changed and inventory updated",
		})
	})
}
