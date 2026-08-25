package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/redbida"
)

func registerRedbidaTools(r *Registry, cfg *config.Config, redbidaSvc *redbida.Service) {
	checkService := func() error {
		if redbidaSvc == nil {
			return fmt.Errorf("redbida integration is disabled or not configured in config.yaml")
		}
		return nil
	}

	// 1. redbida_list_catalog
	r.Register(Tool{
		Name:        "redbida_list_catalog",
		Description: "List all configuration keys in the RedBida / OTA-MQTT catalog with their metadata, functional group, risk classification (editable, confirm-required, protected), data type, and storage source availability.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"group": map[string]any{
					"type":        "string",
					"description": "Filter by group (e.g. 'UI / Display', 'Livestream', 'Branding / Logo', 'Schedule / Maintenance', 'Security / Credentials', 'Network / MQTT')",
				},
				"editableOnly": map[string]any{
					"type":        "boolean",
					"description": "Only return keys that can be edited",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkService(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Group        string `json:"group,omitempty"`
			EditableOnly bool   `json:"editableOnly,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		allMetas := redbidaSvc.Catalog()
		sourceAvailable, sourceError := redbidaSvc.CatalogStatus()

		filtered := make([]redbida.KeyMeta, 0, len(allMetas))
		for _, m := range allMetas {
			if req.Group != "" && !strings.EqualFold(m.Group, req.Group) {
				continue
			}
			if req.EditableOnly && !m.Editable {
				continue
			}
			filtered = append(filtered, m)
		}

		return NewJSONResult(map[string]any{
			"keys":            filtered,
			"count":           len(filtered),
			"sourceAvailable": sourceAvailable,
			"sourceError":     sourceError,
		})
	})

	// 2. redbida_get_keys
	r.Register(Tool{
		Name:        "redbida_get_keys",
		Description: "Read the live values of one or more configuration keys from the local ota-mqtt broker (via /private/i_gets). Sensitive credentials are automatically masked.",
		InputSchema: ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"keys": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "List of key names to fetch",
				},
				"all": map[string]any{
					"type":        "boolean",
					"description": "If true and keys is empty, fetch all available keys from the catalog",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkService(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Keys []string `json:"keys,omitempty"`
			All  bool     `json:"all,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		if len(req.Keys) == 0 {
			for _, m := range redbidaSvc.Catalog() {
				req.Keys = append(req.Keys, m.Key)
			}
		}

		values, err := redbidaSvc.Refresh(ctx, req.Keys)
		if err != nil {
			return NewErrorResult("failed to get redbida keys: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"values":      values,
			"count":       len(values),
			"refreshedAt": time.Now().Format(time.RFC3339),
		})
	})

	// 3. redbida_set_keys
	r.Register(Tool{
		Name:        "redbida_set_keys",
		Description: "Write one or more key-value pairs to the local ota-mqtt broker (via /private/i_sets) with mandatory read-back verification. High-risk maintenance keys require confirmed=true.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"changes"},
			Properties: map[string]any{
				"changes": map[string]any{
					"type":        "object",
					"description": "Key-value map of configuration changes",
				},
				"confirmed": map[string]any{
					"type":        "boolean",
					"description": "Must be true to apply confirm-required maintenance or restart keys",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkService(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		var req struct {
			Changes   map[string]any `json:"changes"`
			Confirmed bool           `json:"confirmed,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		if len(req.Changes) == 0 {
			return NewErrorResult("changes map cannot be empty"), nil
		}

		results, err := redbidaSvc.Apply(ctx, req.Changes, req.Confirmed)
		if err != nil {
			return NewErrorResult("failed to apply redbida changes: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"results":   results,
			"count":     len(results),
			"appliedAt": time.Now().Format(time.RFC3339),
		})
	})

	// 4. redbida_apply_onboarding_preset
	r.Register(Tool{
		Name:        "redbida_apply_onboarding_preset",
		Description: "1-Click Bida Onboarding Tool: Automatically synthesizes and applies the 15 standard golden template parameters (title, company_name, sanitized ui_bg gradient, diacritic-free custom_hashtags, 20-tab INI ui_tabs_links, camera_count, toolbar_show_count, video_config, go2rtc flags, scoreboard, logos, and go2rtc trigger flag) with full read-back verification.",
		InputSchema: ToolInputSchema{
			Type:     "object",
			Required: []string{"title", "cameraCount"},
			Properties: map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Shop name / venue label (e.g. 'CX King Luxury')",
				},
				"cameraCount": map[string]any{
					"type":        "integer",
					"description": "Number of active cameras / monitors (1 to 20)",
				},
				"bg": map[string]any{
					"type":        "string",
					"description": "CSS gradient background string (trailing semicolons will be automatically stripped)",
				},
				"groupKey": map[string]any{
					"type":        "string",
					"description": "Shinobi group key / shinobi_camera_id",
				},
				"shinobiToken": map[string]any{
					"type":        "string",
					"description": "Shinobi view token (API key with view stream/video rights)",
				},
				"shinobiMonitorToken": map[string]any{
					"type":        "string",
					"description": "Shinobi monitor token (API key with get monitor rights)",
				},
				"ggcode": map[string]any{
					"type":        "string",
					"description": "Google Analytics measurement ID (e.g. 'G-SFSDZPR95Z')",
				},
				"customHashtags": map[string]any{
					"type":        "string",
					"description": "Custom hashtags override. If omitted, generated as 'Tìm hiểu thêm tại BilliardLive.IO.VN\\n#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports'",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "If true, returns the synthesized 15 keys without writing to MQTT broker",
				},
				"confirmed": map[string]any{
					"type":        "boolean",
					"description": "Whether to auto-confirm maintenance keys (default: true)",
				},
			},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		var req struct {
			Title               string `json:"title"`
			CameraCount         int    `json:"cameraCount"`
			BG                  string `json:"bg,omitempty"`
			GroupKey            string `json:"groupKey,omitempty"`
			ShinobiToken        string `json:"shinobiToken,omitempty"`
			ShinobiMonitorToken string `json:"shinobiMonitorToken,omitempty"`
			GGCode              string `json:"ggcode,omitempty"`
			CustomHashtags      string `json:"customHashtags,omitempty"`
			DryRun              bool   `json:"dryRun,omitempty"`
			Confirmed           *bool  `json:"confirmed,omitempty"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args, &req); err != nil {
				return NewErrorResult("invalid arguments: " + err.Error()), err
			}
		}

		title := strings.TrimSpace(req.Title)
		if title == "" {
			return NewErrorResult("title is required"), nil
		}
		if req.CameraCount < 1 || req.CameraCount > 20 {
			return NewErrorResult("cameraCount must be between 1 and 20"), nil
		}

		confirmed := true
		if req.Confirmed != nil {
			confirmed = *req.Confirmed
		}

		bg := sanitizeCSSGradient(req.BG)

		customHashtags := strings.TrimSpace(req.CustomHashtags)
		if customHashtags == "" {
			cleanTitle := sanitizeCleanTitle(title)
			if cleanTitle != "" {
				customHashtags = fmt.Sprintf("Tìm hiểu thêm tại BilliardLive.IO.VN\n#%s #BILLIARDSlive #INUTlive #highlightsports", cleanTitle)
			} else {
				customHashtags = "Tìm hiểu thêm tại BilliardLive.IO.VN\n#BILLIARDSlive #INUTlive #highlightsports"
			}
		}

		iniTabs := generate20TabINITabs(title)

		presetChanges := map[string]any{
			"ui_title":                      title,
			"company_name":                  title,
			"ui_bg":                         bg,
			"custom_hashtags":               customHashtags,
			"ui_tabs_links":                 iniTabs,
			"camera_count":                  req.CameraCount,
			"toolbar_show_count":            req.CameraCount,
			"video_config":                  "range=72",
			"hls_using_go2rtc":              true,
			"hls_using_go2rtc_livestream":   true,
			"hls_using_go2rtc_tiktok":       true,
			"ui_scoreboard":                 true,
			"logo_header":                   "https://vnmap-backend.inut.vn/uploads/bidalive_efd101c4e6.png",
			"logo_header_text":              "Billiard Live - Tải clip bàn bida và livestream",
			"button_generate_go2rtc_stream": true,
			"api_count":                     0,
			"api_model_count":               0,
		}

		if strings.TrimSpace(req.GroupKey) != "" {
			gk := strings.TrimSpace(req.GroupKey)
			presetChanges["shinobi_camera_id"] = gk
			presetChanges["shinobi_group_key"] = gk
		}
		if strings.TrimSpace(req.GGCode) != "" {
			presetChanges["ggcode"] = strings.TrimSpace(req.GGCode)
		}
		if strings.TrimSpace(req.ShinobiToken) != "" {
			presetChanges["shinobi_token"] = strings.TrimSpace(req.ShinobiToken)
		}
		if strings.TrimSpace(req.ShinobiMonitorToken) != "" {
			presetChanges["shinobi_monitor_token"] = strings.TrimSpace(req.ShinobiMonitorToken)
		}

		if req.DryRun {
			return NewJSONResult(map[string]any{
				"dryRun":         true,
				"title":          title,
				"cameraCount":    req.CameraCount,
				"parameterCount": len(presetChanges),
				"parameters":     presetChanges,
			})
		}

		if err := checkService(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		results, err := redbidaSvc.Apply(ctx, presetChanges, confirmed)
		if err != nil {
			return NewErrorResult("failed to apply onboarding preset: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"ok":             true,
			"title":          title,
			"cameraCount":    req.CameraCount,
			"parameterCount": len(presetChanges),
			"results":        results,
			"appliedAt":      time.Now().Format(time.RFC3339),
		})
	})

	// 5. redbida_trigger_go2rtc
	r.Register(Tool{
		Name:        "redbida_trigger_go2rtc",
		Description: "Trigger Node-RED :2023 to generate /root/go2rtc.yaml stream configurations by publishing button_generate_go2rtc_stream: 'true' over MQTT /private/i_sets.",
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		if err := checkService(); err != nil {
			return NewErrorResult(err.Error()), err
		}

		results, err := redbidaSvc.Apply(ctx, map[string]any{
			"button_generate_go2rtc_stream": true,
		}, true)
		if err != nil {
			return NewErrorResult("failed to trigger go2rtc stream generation: " + err.Error()), err
		}

		return NewJSONResult(map[string]any{
			"ok":          true,
			"message":     "Go2RTC stream generation triggered via MQTT button_generate_go2rtc_stream",
			"results":     results,
			"triggeredAt": time.Now().Format(time.RFC3339),
		})
	})

	// 6. redbida_get_time_status
	r.Register(Tool{
		Name:        "redbida_get_time_status",
		Description: "Check host system clock, RFC 3339 timestamp, and NTP synchronization status via timedatectl to ensure accurate video playback timelines.",
		InputSchema: ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{},
		},
	}, func(ctx context.Context, args json.RawMessage) (ToolResult, error) {
		now := time.Now()
		ntpSync := queryNTPSynchronized(ctx)

		return NewJSONResult(map[string]any{
			"hostTime":              now.Format("2006-01-02 15:04:05"),
			"hostTimeRFC3339":       now.Format(time.RFC3339),
			"ntpSynchronized":       ntpSync,
			"driftThresholdSeconds": 60,
			"policy":                "sync only when host NTP is trusted and measured drift exceeds 60 seconds",
			"nodeRedReadOnly":       true,
		})
	})
}

// removeVietnameseTones strips all Vietnamese accent marks from both precomposed (NFC)
// and decomposed (NFD) strings in pure Go.
func removeVietnameseTones(str string) string {
	if str == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(str))
	for _, r := range str {
		// Skip combining diacritical marks (U+0300 to U+036F)
		if r >= 0x0300 && r <= 0x036F {
			continue
		}
		switch r {
		case 'à', 'á', 'ả', 'ã', 'ạ', 'ă', 'ằ', 'ắ', 'ẳ', 'ẵ', 'ặ', 'â', 'ầ', 'ấ', 'ẩ', 'ẫ', 'ậ':
			b.WriteRune('a')
		case 'À', 'Á', 'Ả', 'Ã', 'Ạ', 'Ă', 'Ằ', 'Ắ', 'Ẳ', 'Ẵ', 'Ặ', 'Â', 'Ầ', 'Ấ', 'Ẩ', 'Ẫ', 'Ậ':
			b.WriteRune('A')
		case 'è', 'é', 'ẻ', 'ẽ', 'ẹ', 'ê', 'ề', 'ế', 'ể', 'ễ', 'ệ':
			b.WriteRune('e')
		case 'È', 'É', 'Ẻ', 'Ẽ', 'Ẹ', 'Ê', 'Ề', 'Ế', 'Ể', 'Ễ', 'Ệ':
			b.WriteRune('E')
		case 'ì', 'í', 'ỉ', 'ĩ', 'ị':
			b.WriteRune('i')
		case 'Ì', 'Í', 'Ỉ', 'Ĩ', 'Ị':
			b.WriteRune('I')
		case 'ò', 'ó', 'ỏ', 'õ', 'ọ', 'ô', 'ồ', 'ố', 'ổ', 'ỗ', 'ộ', 'ơ', 'ờ', 'ớ', 'ở', 'ỡ', 'ợ':
			b.WriteRune('o')
		case 'Ò', 'Ó', 'Ỏ', 'Õ', 'Ọ', 'Ô', 'Ồ', 'Ố', 'Ổ', 'Ỗ', 'Ộ', 'Ơ', 'Ờ', 'Ớ', 'Ở', 'Ỡ', 'Ợ':
			b.WriteRune('O')
		case 'ù', 'ú', 'ủ', 'ũ', 'ụ', 'ư', 'ừ', 'ứ', 'ử', 'ữ', 'ự':
			b.WriteRune('u')
		case 'Ù', 'Ú', 'Ủ', 'Ũ', 'Ụ', 'Ư', 'Ừ', 'Ứ', 'Ử', 'Ữ', 'Ự':
			b.WriteRune('U')
		case 'ỳ', 'ý', 'ỷ', 'ỹ', 'ỵ':
			b.WriteRune('y')
		case 'Ỳ', 'Ý', 'Ỷ', 'Ỹ', 'Ỵ':
			b.WriteRune('Y')
		case 'đ':
			b.WriteRune('d')
		case 'Đ':
			b.WriteRune('D')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// sanitizeCleanTitle removes Vietnamese tones and strips all non-alphanumeric characters.
func sanitizeCleanTitle(title string) string {
	noTones := removeVietnameseTones(title)
	var b strings.Builder
	for _, r := range noTones {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// generate20TabINITabs builds the standard 20-section INI configuration from [C01] to [C20].
func generate20TabINITabs(title string) string {
	var sections []string
	for i := 1; i <= 20; i++ {
		sections = append(sections, fmt.Sprintf("[C%02d]\nstream_label=Video Trực tiếp\nvid_list_label=Danh sách highlight\nvid_play_label=%s\nlist_refresh_label=Cập nhật highlight", i, title))
	}
	return strings.Join(sections, "\n\n")
}

// sanitizeCSSGradient ensures CSS background gradients have no trailing semicolon.
func sanitizeCSSGradient(rawBg string) string {
	bg := strings.TrimSpace(rawBg)
	if bg == "" {
		bg = "radial-gradient( circle farthest-corner at 10% 20%, rgba(2,37,78,1) 0%, rgba(4,4,16,1) 90.1% )"
	}
	for strings.HasSuffix(bg, ";") {
		bg = strings.TrimSpace(strings.TrimSuffix(bg, ";"))
	}
	return bg
}

// queryNTPSynchronized checks timedatectl NTP status with a 2-second timeout.
func queryNTPSynchronized(ctx context.Context) bool {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "timedatectl", "show", "-p", "NTPSynchronized", "--value").Output()
	if err != nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(string(out)))
	if v == "yes" || v == "1" {
		return true
	}
	b, _ := strconv.ParseBool(v)
	return b
}
