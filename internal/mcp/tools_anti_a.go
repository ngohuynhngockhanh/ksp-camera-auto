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

func registerAntiATools(r *Registry, cfg *config.Config, inv *config.Inventory) {
	// 1. kspcam_get_anti_a_status
	r.Register(
		Tool{
			Name:        "kspcam_get_anti_a_status",
			Description: "Get the current operating status, check interval, and security enforcement parameters of Anti-A Guardian (H.265 Lock).",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]any{},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			return NewJSONResult(map[string]any{
				"enabled":         cfg.AntiA.Enabled,
				"intervalMinutes": cfg.AntiA.IntervalMinutes,
				"mode":            cfg.AntiA.Mode,
				"description":     "Anti-A Guardian locks cameras to H.265 (HEVC), Smart Codec (H.265+), and Audio AAC to prevent unauthorized stream usage.",
			})
		},
	)

	// 2. kspcam_set_anti_a_config
	r.Register(
		Tool{
			Name:        "kspcam_set_anti_a_config",
			Description: "Configure Anti-A Guardian watchdog parameters (enable/disable, check interval in minutes, scan mode 'random' or 'full').",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"enabled": map[string]any{
						"type":        "boolean",
						"description": "Enable or disable Anti-A Guardian",
					},
					"interval_minutes": map[string]any{
						"type":        "integer",
						"description": "Check interval in minutes (default: 30, min: 1)",
					},
					"mode": map[string]any{
						"type":        "string",
						"enum":        []string{"random", "full"},
						"description": "Detection mode: 'random' (pick 1 random camera) or 'full' (scan all cameras)",
					},
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var params struct {
				Enabled         *bool   `json:"enabled"`
				IntervalMinutes *int    `json:"interval_minutes"`
				Mode            *string `json:"mode"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}

			if params.Enabled != nil {
				cfg.AntiA.Enabled = *params.Enabled
			}
			if params.IntervalMinutes != nil && *params.IntervalMinutes > 0 {
				cfg.AntiA.IntervalMinutes = *params.IntervalMinutes
			}
			if params.Mode != nil && (*params.Mode == "random" || *params.Mode == "full") {
				cfg.AntiA.Mode = *params.Mode
			}

			return NewJSONResult(map[string]any{
				"ok":              true,
				"enabled":         cfg.AntiA.Enabled,
				"intervalMinutes": cfg.AntiA.IntervalMinutes,
				"mode":            cfg.AntiA.Mode,
			})
		},
	)

	// 3. kspcam_trigger_anti_a
	r.Register(
		Tool{
			Name:        "kspcam_trigger_anti_a",
			Description: "Immediately trigger Anti-A Guardian to probe cameras and enforce H.265 + Smart Codec + Audio AAC across the fleet.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"force_all": map[string]any{
						"type":        "boolean",
						"description": "If true, enforce all cameras without checking probe first. Defaults to false (only enforce non-H265/non-smartcodec).",
					},
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var params struct {
				ForceAll bool `json:"force_all"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args, &params)
			}

			cams := inv.List()
			if len(cams) == 0 {
				return NewJSONResult(map[string]any{
					"ok":       true,
					"enforced": 0,
					"message":  "No cameras in inventory",
				})
			}

			enforced := 0
			var results []map[string]any

			for _, camCfg := range cams {
				cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
				cam, err := camera.Open(cctx, camCfg, 20*time.Second)
				if err != nil {
					cancel()
					results = append(results, map[string]any{
						"cameraId": camCfg.ID,
						"name":     camCfg.Name,
						"host":     camCfg.Host,
						"status":   "error",
						"error":    err.Error(),
					})
					continue
				}

				streams, err := cam.Probe(cctx)
				cancel()
				cam.Close()

				detectedCodec := "Unknown"
				smartCodec := false
				isH265 := false
				if err == nil && len(streams) > 0 {
					detectedCodec = streams[0].Compression
					smartCodec = streams[0].SmartCodec
					isH265 = strings.Contains(strings.ToUpper(detectedCodec), "H.265") || strings.Contains(strings.ToUpper(detectedCodec), "HEVC")
				}
				isCompliant := isH265 && smartCodec

				if isCompliant && !params.ForceAll {
					results = append(results, map[string]any{
						"cameraId":   camCfg.ID,
						"name":       camCfg.Name,
						"host":       camCfg.Host,
						"codec":      detectedCodec,
						"smartCodec": smartCodec,
						"status":     "already_compliant",
					})
					continue
				}

				// Apply H.265 + Smart Codec + Audio AAC
				applyCtx, applyCancel := context.WithTimeout(ctx, 25*time.Second)
				camApply, err := camera.Open(applyCtx, camCfg, 25*time.Second)
				if err != nil {
					applyCancel()
					results = append(results, map[string]any{
						"cameraId": camCfg.ID,
						"name":     camCfg.Name,
						"host":     camCfg.Host,
						"status":   "error",
						"error":    err.Error(),
					})
					continue
				}

				profile := camera.Profile{
					SetCodec:      true,
					Codec:         "H.265",
					SetSmartCodec: true,
					SmartCodec:    true,
					SetAudioAAC:   true,
					Streams:       []int{camera.StreamMain},
				}

				steps := camApply.Apply(applyCtx, profile, nil)
				applyCancel()
				camApply.Close()

				hasErr := false
				var errMsgs []string
				for _, s := range steps {
					if !s.OK && s.Err != "" {
						hasErr = true
						errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", s.Step, s.Err))
					}
				}

				if !hasErr {
					enforced++
					results = append(results, map[string]any{
						"cameraId":   camCfg.ID,
						"name":       camCfg.Name,
						"host":       camCfg.Host,
						"status":     "enforced",
						"codec":      "H.265",
						"smartCodec": true,
					})
				} else {
					results = append(results, map[string]any{
						"cameraId": camCfg.ID,
						"name":     camCfg.Name,
						"host":     camCfg.Host,
						"status":   "apply_error",
						"error":    strings.Join(errMsgs, "; "),
					})
				}
			}

			return NewJSONResult(map[string]any{
				"ok":       true,
				"enforced": enforced,
				"total":    len(cams),
				"results":  results,
			})
		},
	)
}
