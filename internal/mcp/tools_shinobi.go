package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/shinobi"
)

func registerShinobiTools(r *Registry, cfg *config.Config, inv *config.Inventory, sc *shinobi.Client) {
	checkClient := func() error {
		if sc == nil || sc.APIURL() == "" {
			return fmt.Errorf("shinobi client not configured (missing shinobi.api_url or api_key in config.yaml)")
		}
		return nil
	}

	// 17. shinobi_list_monitors
	r.Register(Tool{
		Name:        "shinobi_list_monitors",
		Description: "List all monitors configured in the Shinobi NVR instance with their status, stream URLs, and recording modes.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"groupKey": map[string]any{
					"type":        "string",
					"description": "Optional group key override",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		mons, err := sc.ListMonitors(ctx)
		if err != nil {
			return NewErrorResult("failed to list shinobi monitors: " + err.Error()), err
		}

		return NewJSONResult(mons)
	})

	// 18. shinobi_add_monitor
	r.Register(Tool{
		Name:        "shinobi_add_monitor",
		Description: "Add a new camera monitor stream into Shinobi NVR with RTSP URL, credentials, stream dimensions, audio, and recording mode.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"mid", "name", "rtspUrl"},
			Properties: map[string]any{
				"mid": map[string]any{
					"type":        "string",
					"description": "Unique alphanumeric monitor ID",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Monitor display name",
				},
				"rtspUrl": map[string]any{
					"type":        "string",
					"description": "Full RTSP stream URL with credentials (e.g. rtsp://admin:pass@192.168.1.108:554/cam/realmonitor?channel=1&subtype=0)",
				},
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"idle", "start", "record", "stop"},
					"default":     "record",
					"description": "Stream mode: 'record', 'start', 'idle', or 'stop'",
				},
				"width": map[string]any{
					"type":    "integer",
					"default": 1920,
				},
				"height": map[string]any{
					"type":    "integer",
					"default": 1080,
				},
				"fps": map[string]any{
					"type":    "integer",
					"default": 25,
				},
				"audioCodec": map[string]any{
					"type":        "string",
					"default":     "aac",
					"description": "Audio codec: 'aac', 'copy', or 'no'",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Mid        string `json:"mid"`
			Name       string `json:"name"`
			RTSPURL    string `json:"rtspUrl"`
			Mode       string `json:"mode,omitempty"`
			Width      int    `json:"width,omitempty"`
			Height     int    `json:"height,omitempty"`
			FPS        int    `json:"fps,omitempty"`
			AudioCodec string `json:"audioCodec,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}

		req.Mid = strings.TrimSpace(req.Mid)
		req.Name = strings.TrimSpace(req.Name)
		req.RTSPURL = strings.TrimSpace(req.RTSPURL)
		if req.Mid == "" || req.Name == "" || req.RTSPURL == "" {
			return NewErrorResult("mid, name, and rtspUrl are required"), fmt.Errorf("missing required fields")
		}

		mode := strings.ToLower(strings.TrimSpace(req.Mode))
		if mode == "" {
			mode = "record"
		}
		audioCodec := strings.ToLower(strings.TrimSpace(req.AudioCodec))
		if audioCodec == "" {
			audioCodec = "aac"
		}

		host := ""
		port := "554"
		path := ""
		muser := ""
		mpass := ""
		if u, err := url.Parse(req.RTSPURL); err == nil {
			if h := u.Hostname(); h != "" {
				host = h
			}
			if p := u.Port(); p != "" {
				port = p
			}
			path = u.Path
			if u.RawQuery != "" {
				path += "?" + u.RawQuery
			}
			if u.User != nil {
				muser = u.User.Username()
				if p, ok := u.User.Password(); ok {
					mpass = p
				}
			}
		}

		wStr := "1920"
		if req.Width > 0 {
			wStr = strconv.Itoa(req.Width)
		}
		hStr := "1080"
		if req.Height > 0 {
			hStr = strconv.Itoa(req.Height)
		}
		fpsStr := "25"
		if req.FPS > 0 {
			fpsStr = strconv.Itoa(req.FPS)
		}

		mon := shinobi.MonitorConfig{
			Mid:      req.Mid,
			Ke:       sc.GroupKey(),
			Name:     req.Name,
			Type:     "h264",
			Mode:     mode,
			Host:     host,
			Port:     port,
			Protocol: "rtsp",
			Path:     path,
			Ext:      "mp4",
			FPS:      fpsStr,
			Width:    wStr,
			Height:   hStr,
			Details: shinobi.MonitorDetails{
				AutoHost:           req.RTSPURL,
				Muser:              muser,
				Mpass:              mpass,
				Port:               port,
				Protocol:           "rtsp",
				StreamType:         "mp4",
				StreamFlvType:      "ws",
				StreamMjpegClients: "",
				Vcodec:             "copy",
				Acodec:             "copy",
				RecordVcodec:       "copy",
				RecordAcodec:       audioCodec,
				Detector:           "0",
			},
		}

		if err := sc.AddMonitor(ctx, mon); err != nil {
			return NewErrorResult("add monitor failed: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"mid":     req.Mid,
			"message": "Monitor added to Shinobi",
		})
	})

	// 19. shinobi_edit_monitor
	r.Register(Tool{
		Name:        "shinobi_edit_monitor",
		Description: "Edit an existing monitor in Shinobi NVR (modify RTSP URL, resolution, FPS, or mode).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"mid"},
			Properties: map[string]any{
				"mid": map[string]any{
					"type":        "string",
					"description": "Monitor ID to update",
				},
				"name": map[string]any{
					"type": "string",
				},
				"rtspUrl": map[string]any{
					"type": "string",
				},
				"mode": map[string]any{
					"type": "string",
					"enum": []string{"idle", "start", "record", "stop"},
				},
				"width": map[string]any{
					"type": "integer",
				},
				"height": map[string]any{
					"type": "integer",
				},
				"fps": map[string]any{
					"type": "integer",
				},
				"audioCodec": map[string]any{
					"type": "string",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Mid        string `json:"mid"`
			Name       string `json:"name,omitempty"`
			RTSPURL    string `json:"rtspUrl,omitempty"`
			Mode       string `json:"mode,omitempty"`
			Width      int    `json:"width,omitempty"`
			Height     int    `json:"height,omitempty"`
			FPS        int    `json:"fps,omitempty"`
			AudioCodec string `json:"audioCodec,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.Mid = strings.TrimSpace(req.Mid)
		if req.Mid == "" {
			return NewErrorResult("mid is required"), fmt.Errorf("missing mid")
		}

		// Retrieve existing monitor config
		existing, err := sc.GetMonitor(ctx, req.Mid)
		if err != nil {
			return NewErrorResult("failed to get existing monitor: " + err.Error()), err
		}
		det := existing.ParseDetails()

		name := existing.Name
		if req.Name != "" {
			name = req.Name
		}
		mode := existing.Mode
		if req.Mode != "" {
			mode = req.Mode
		}
		rtspURL := det.AutoHost
		if req.RTSPURL != "" {
			rtspURL = req.RTSPURL
		}
		wStr := string(existing.Width)
		if req.Width > 0 {
			wStr = strconv.Itoa(req.Width)
		}
		hStr := string(existing.Height)
		if req.Height > 0 {
			hStr = strconv.Itoa(req.Height)
		}
		fpsStr := string(existing.FPS)
		if req.FPS > 0 {
			fpsStr = strconv.Itoa(req.FPS)
		}
		audioCodec := det.RecordAcodec
		if req.AudioCodec != "" {
			audioCodec = req.AudioCodec
		}

		host := existing.Host
		port := string(existing.Port)
		path := existing.Path
		muser := det.Muser
		mpass := det.Mpass
		if req.RTSPURL != "" {
			if u, err := url.Parse(req.RTSPURL); err == nil {
				if h := u.Hostname(); h != "" {
					host = h
				}
				if p := u.Port(); p != "" {
					port = p
				}
				path = u.Path
				if u.RawQuery != "" {
					path += "?" + u.RawQuery
				}
				if u.User != nil {
					muser = u.User.Username()
					if p, ok := u.User.Password(); ok {
						mpass = p
					}
				}
			}
		}

		mon := shinobi.MonitorConfig{
			Mid:      req.Mid,
			Ke:       sc.GroupKey(),
			Name:     name,
			Type:     "h264",
			Mode:     mode,
			Host:     host,
			Port:     port,
			Protocol: "rtsp",
			Path:     path,
			Ext:      "mp4",
			FPS:      fpsStr,
			Width:    wStr,
			Height:   hStr,
			Details: shinobi.MonitorDetails{
				AutoHost:           rtspURL,
				Muser:              muser,
				Mpass:              mpass,
				Port:               port,
				Protocol:           "rtsp",
				StreamType:         "mp4",
				StreamFlvType:      "ws",
				StreamMjpegClients: det.StreamMjpegClients,
				Vcodec:             "copy",
				Acodec:             "copy",
				RecordVcodec:       "copy",
				RecordAcodec:       audioCodec,
				Detector:           det.Detector,
			},
		}

		if err := sc.EditMonitor(ctx, req.Mid, mon); err != nil {
			return NewErrorResult("edit monitor failed: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"mid":     req.Mid,
			"message": "Monitor updated",
		})
	})

	// 20. shinobi_delete_monitor
	r.Register(Tool{
		Name:        "shinobi_delete_monitor",
		Description: "Delete a monitor configuration from Shinobi NVR.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"mid"},
			Properties: map[string]any{
				"mid": map[string]any{
					"type":        "string",
					"description": "Monitor ID to delete",
				},
				"deleteRecordings": map[string]any{
					"type":        "boolean",
					"default":     false,
					"description": "Whether to delete recorded files on disk",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Mid              string `json:"mid"`
			DeleteRecordings bool   `json:"deleteRecordings,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.Mid = strings.TrimSpace(req.Mid)
		if req.Mid == "" {
			return NewErrorResult("mid is required"), fmt.Errorf("missing mid")
		}

		if err := sc.DeleteMonitor(ctx, req.Mid); err != nil {
			return NewErrorResult("delete monitor failed: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status":  "success",
			"mid":     req.Mid,
			"message": "Monitor deleted",
		})
	})

	// 21. shinobi_sync_to_shinobi (Push cameras.yaml -> Shinobi monitors)
	r.Register(Tool{
		Name:        "shinobi_sync_to_shinobi",
		Description: "Push / Export cameras from kspcam inventory (cameras.yaml) into Shinobi NVR monitors.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"dryRun": map[string]any{
					"type":    "boolean",
					"default": false,
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		report, err := sc.SyncToShinobi(ctx, inv)
		if err != nil {
			return NewErrorResult("sync to shinobi failed: " + err.Error()), err
		}

		return NewJSONResult(report)
	})

	// 22. shinobi_sync_from_shinobi (Pull Shinobi monitors -> cameras.yaml)
	r.Register(Tool{
		Name:        "shinobi_sync_from_shinobi",
		Description: "Pull / Import monitor streams from Shinobi NVR into kspcam inventory (cameras.yaml).",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"dryRun": map[string]any{
					"type":    "boolean",
					"default": false,
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		report, err := sc.SyncFromShinobi(ctx, inv)
		if err != nil {
			return NewErrorResult("sync from shinobi failed: " + err.Error()), err
		}

		return NewJSONResult(report)
	})

	// 23. shinobi_sync_inventory (General directional sync)
	r.Register(Tool{
		Name:        "shinobi_sync_inventory",
		Description: "Reconcile cameras between kspcam inventory (cameras.yaml) and Shinobi NVR monitors in either or both directions.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"direction": map[string]any{
					"type":        "string",
					"enum":        []string{"both", "to_shinobi", "from_shinobi"},
					"default":     "both",
					"description": "'to_shinobi' exports inventory to Shinobi, 'from_shinobi' imports Shinobi monitors to inventory, 'both' runs push then pull",
				},
				"dryRun": map[string]any{
					"type":    "boolean",
					"default": false,
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Direction string `json:"direction,omitempty"`
			DryRun    bool   `json:"dryRun,omitempty"`
		}
		if len(args) > 0 {
			_ = json.Unmarshal(args, &req)
		}

		dir := strings.ToLower(strings.TrimSpace(req.Direction))
		if dir == "" {
			dir = "both"
		}

		var toReport, fromReport *shinobi.SyncReport
		var err error

		if dir == "to_shinobi" || dir == "both" {
			toReport, err = sc.SyncToShinobi(ctx, inv)
			if err != nil && dir == "to_shinobi" {
				return NewErrorResult("sync to shinobi failed: " + err.Error()), err
			}
		}

		if dir == "from_shinobi" || dir == "both" {
			fromReport, err = sc.SyncFromShinobi(ctx, inv)
			if err != nil && dir == "from_shinobi" {
				return NewErrorResult("sync from shinobi failed: " + err.Error()), err
			}
		}

		return NewJSONResult(map[string]any{
			"status":      "success",
			"direction":   dir,
			"toShinobi":   toReport,
			"fromShinobi": fromReport,
		})
	})

	// 24. shinobi_change_monitor_state
	r.Register(Tool{
		Name:        "shinobi_change_monitor_state",
		Description: "Change the active execution state of a Shinobi monitor (idle/stop, start watching, start recording, or restart process).",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"mid", "state"},
			Properties: map[string]any{
				"mid": map[string]any{
					"type":        "string",
					"description": "Monitor ID",
				},
				"state": map[string]any{
					"type":        "string",
					"enum":        []string{"idle", "start", "record", "stop", "restart"},
					"description": "Target monitor state",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Mid   string `json:"mid"`
			State string `json:"state"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.Mid = strings.TrimSpace(req.Mid)
		req.State = strings.TrimSpace(req.State)
		if req.Mid == "" || req.State == "" {
			return NewErrorResult("mid and state are required"), fmt.Errorf("missing mid or state")
		}

		if err := sc.ChangeMonitorState(ctx, req.Mid, req.State); err != nil {
			return NewErrorResult("change monitor state failed: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"status": "success",
			"mid":    req.Mid,
			"state":  req.State,
		})
	})

	// 25. shinobi_get_videos
	r.Register(Tool{
		Name:        "shinobi_get_videos",
		Description: "Query stored video recordings for a Shinobi monitor within a date range.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"mid"},
			Properties: map[string]any{
				"mid": map[string]any{
					"type":        "string",
					"description": "Monitor ID",
				},
				"limit": map[string]any{
					"type":    "integer",
					"default": 50,
				},
				"startTime": map[string]any{
					"type":        "string",
					"description": "Start timestamp in ISO 8601 / RFC 3339",
				},
				"endTime": map[string]any{
					"type":        "string",
					"description": "End timestamp in ISO 8601 / RFC 3339",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkClient(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Mid       string `json:"mid"`
			Limit     int    `json:"limit,omitempty"`
			StartTime string `json:"startTime,omitempty"`
			EndTime   string `json:"endTime,omitempty"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return NewErrorResult("invalid arguments: " + err.Error()), err
		}
		req.Mid = strings.TrimSpace(req.Mid)
		if req.Mid == "" {
			return NewErrorResult("mid is required"), fmt.Errorf("missing mid")
		}
		if req.Limit <= 0 {
			req.Limit = 50
		}

		videos, err := sc.GetVideos(ctx, req.Mid, req.Limit)
		if err != nil {
			return NewErrorResult("get videos failed: " + err.Error()), err
		}

		return NewJSONResult(videos)
	})
}
