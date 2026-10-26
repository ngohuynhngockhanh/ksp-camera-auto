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

func registerAIAndEventTools(r *Registry, cfg *config.Config, inv *config.Inventory, srv *Server) {
	// 1. kspcam_ai_get_capabilities
	r.Register(Tool{
		Name:        "kspcam_ai_get_capabilities",
		Description: "Read all AI / Smart Event capabilities and their On/Off status across channels for a camera or NVR (e.g. FaceDetection, HumanVehicle, LicensePlate, LineCrossing, Intrusion).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID in inventory",
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
		d, ok := inv.Get(strings.TrimSpace(req.ID))
		if !ok {
			return NewErrorResult(fmt.Sprintf("camera %q not found", req.ID)), fmt.Errorf("not found")
		}
		cam, err := camera.Open(ctx, d, 15*time.Second)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		defer cam.Close()
		ai, ok := cam.(camera.AIConfig)
		if !ok {
			return NewErrorResult("camera vendor does not support AI capabilities"), fmt.Errorf("unsupported")
		}
		caps, err := ai.GetChnSmart(ctx)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		return NewJSONResult(map[string]any{
			"id":       req.ID,
			"channels": caps,
		})
	})

	// 2. kspcam_ai_get_setting
	r.Register(Tool{
		Name:        "kspcam_ai_get_setting",
		Description: "Read detailed AI rule configuration and range for a specific channel and ability (FaceDetection, HumanVehicle, LicensePlate, LineCrossing, Intrusion).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "ability"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID in inventory",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index (default 0)",
				},
				"ability": map[string]any{
					"type":        "string",
					"description": "AI ability name (e.g. HumanVehicle, FaceDetection, LicensePlate, LineCrossing, Intrusion)",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID      string `json:"id"`
			Channel int    `json:"channel"`
			Ability string `json:"ability"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		d, ok := inv.Get(strings.TrimSpace(req.ID))
		if !ok {
			return NewErrorResult(fmt.Sprintf("camera %q not found", req.ID)), fmt.Errorf("not found")
		}
		cam, err := camera.Open(ctx, d, 15*time.Second)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		defer cam.Close()
		ai, ok := cam.(camera.AIConfig)
		if !ok {
			return NewErrorResult("camera vendor does not support AI settings"), fmt.Errorf("unsupported")
		}
		data, rng, err := ai.GetAISetting(ctx, req.Channel, req.Ability)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		return NewJSONResult(map[string]any{
			"id":      req.ID,
			"channel": req.Channel,
			"ability": req.Ability,
			"data":    data,
			"range":   rng,
		})
	})

	// 3. kspcam_ai_set_setting
	r.Register(Tool{
		Name:        "kspcam_ai_set_setting",
		Description: "Update AI rule configuration (e.g. switch, sensitivity, detection_type, rule_info) for a channel and ability with automatic read-back verification.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "ability", "changes"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID in inventory",
				},
				"channel": map[string]any{
					"type":        "integer",
					"description": "0-based channel index",
				},
				"ability": map[string]any{
					"type":        "string",
					"description": "AI ability name (e.g. HumanVehicle, FaceDetection, LicensePlate, LineCrossing, Intrusion)",
				},
				"changes": map[string]any{
					"type":        "object",
					"description": "Key-value changes to merge into the channel's AI setting",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID      string         `json:"id"`
			Channel int            `json:"channel"`
			Ability string         `json:"ability"`
			Changes map[string]any `json:"changes"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		d, ok := inv.Get(strings.TrimSpace(req.ID))
		if !ok {
			return NewErrorResult(fmt.Sprintf("camera %q not found", req.ID)), fmt.Errorf("not found")
		}
		cam, err := camera.Open(ctx, d, 15*time.Second)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		defer cam.Close()
		ai, ok := cam.(camera.AIConfig)
		if !ok {
			return NewErrorResult("camera vendor does not support AI settings"), fmt.Errorf("unsupported")
		}
		updated, err := ai.SetAISetting(ctx, req.Channel, req.Ability, req.Changes)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		return NewJSONResult(map[string]any{
			"ok":      true,
			"id":      req.ID,
			"channel": req.Channel,
			"ability": req.Ability,
			"data":    updated,
		})
	})

	// 4. kspcam_raysharp_raw
	r.Register(Tool{
		Name:        "kspcam_raysharp_raw",
		Description: "Execute any raw Raysharp /API/<Path> endpoint (e.g. AI/FDGroup/Get, AI/SnapedFaces/Search, AI/SnapedObjects/Search, Event/AlarmLinkage/Get).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"id", "path"},
			Properties: map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Device ID in inventory",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Raysharp API path without /API/ prefix (e.g. 'AI/Model/Get', 'SystemInfo/Base/Get')",
				},
				"data": map[string]any{
					"type":        "object",
					"description": "JSON request data payload",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			ID   string `json:"id"`
			Path string `json:"path"`
			Data any    `json:"data"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		d, ok := inv.Get(strings.TrimSpace(req.ID))
		if !ok {
			return NewErrorResult(fmt.Sprintf("camera %q not found", req.ID)), fmt.Errorf("not found")
		}
		cam, err := camera.Open(ctx, d, 20*time.Second)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		defer cam.Close()
		rawCaller, ok := cam.(camera.RawRPC)
		if !ok {
			return NewErrorResult("camera vendor does not support raw Raysharp RPC"), fmt.Errorf("unsupported")
		}
		res, err := rawCaller.CallRaw(ctx, req.Path, req.Data)
		if err != nil {
			return NewErrorResult(err.Error()), err
		}
		return NewJSONResult(res)
	})

	// 5. kspcam_events_list
	r.Register(Tool{
		Name:        "kspcam_events_list",
		Description: "List recent AI and alarm events captured by kspcam's Event Bus (Long-Polling + Webhook) and inspect MQTT / Hub publishing status.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "Optional device ID filter",
				},
				"event_type": map[string]any{
					"type":        "string",
					"description": "Optional event type filter (e.g. human_vehicle, face_detection, license_plate, motion)",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of events to return (default 50)",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			DeviceID  string `json:"device_id"`
			EventType string `json:"event_type"`
			Limit     int    `json:"limit"`
		}
		if len(args) > 0 {
			_ = json.Unmarshal(args, &req)
		}
		if req.Limit <= 0 {
			req.Limit = 50
		}
		bus := srv.EventBus()
		if bus == nil {
			return NewJSONResult(map[string]any{"events": []any{}, "note": "eventbus not bound"})
		}
		return NewJSONResult(map[string]any{
			"status": bus.GetStatus(),
			"events": bus.ListEvents(req.DeviceID, req.EventType, req.Limit),
		})
	})
}
