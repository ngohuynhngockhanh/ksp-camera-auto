package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/traffic"
)

func registerTrafficTools(r *Registry, cfg *config.Config, inv *config.Inventory, trafficMgr *traffic.Manager) {
	if trafficMgr == nil {
		trafficMgr = traffic.NewManager(inv)
	}

	r.Register(
		Tool{
			Name:        "kspcam_get_network_traffic",
			Description: "Inspect real-time network bandwidth and socket connections (iftop-style) on ethernet interfaces (wlan0 excluded), providing live 2s/10s/40s bitrate for every camera stream and service.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"iface": map[string]any{
						"type":        "string",
						"description": "Network interface to inspect (e.g. 'eth0'). Wlan interfaces are excluded.",
					},
					"only_cameras": map[string]any{
						"type":        "boolean",
						"description": "Filter results to only flows involving known camera IPs in cameras.yaml",
					},
					"duration_seconds": map[string]any{
						"type":        "integer",
						"description": "Sampling duration in seconds (default: 2, min: 1, max: 10)",
					},
				},
			},
		},
		func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
			var params struct {
				Iface           string `json:"iface"`
				OnlyCameras     bool   `json:"only_cameras"`
				DurationSeconds int    `json:"duration_seconds"`
			}
			if len(args) > 0 {
				_ = json.Unmarshal(args, &params)
			}

			iface := strings.TrimSpace(params.Iface)
			if iface == "" {
				iface = traffic.DetectDefaultInterface()
			}

			if strings.HasPrefix(strings.ToLower(iface), "wlan") || strings.HasPrefix(strings.ToLower(iface), "wl") {
				return NewErrorResult("Wireless interfaces (wlan0) are excluded from traffic inspection"), nil
			}

			duration := 2 * time.Second
			if params.DurationSeconds > 0 {
				if params.DurationSeconds > 10 {
					params.DurationSeconds = 10
				}
				duration = time.Duration(params.DurationSeconds) * time.Second
			}

			// Subscribe to traffic for sampling duration
			sampleCtx, cancel := context.WithTimeout(ctx, duration)
			defer cancel()

			ch, unsub := trafficMgr.Subscribe(sampleCtx, iface)
			defer unsub()

			var latestSnap traffic.Snapshot
			// Drain snapshots until duration expires
		drainLoop:
			for {
				select {
				case <-sampleCtx.Done():
					break drainLoop
				case snap, ok := <-ch:
					if !ok {
						break drainLoop
					}
					latestSnap = snap
				}
			}

			// If no snap received from ticker, take direct snapshot
			if latestSnap.Interface == "" {
				latestSnap = trafficMgr.GetSnapshot(iface)
			}

			// Filter if only_cameras is requested
			var filteredFlows []traffic.FlowEntry
			for _, f := range latestSnap.Flows {
				if params.OnlyCameras && !f.IsCamera {
					continue
				}
				filteredFlows = append(filteredFlows, f)
			}
			latestSnap.Flows = filteredFlows

			return NewJSONResult(map[string]any{
				"interface":   iface,
				"stats":       latestSnap.Stats,
				"flows":       latestSnap.Flows,
				"totalFlows":  len(latestSnap.Flows),
				"sampleTime":  duration.String(),
			})
		},
	)
}
