package raysharp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SmartAbilityStatus summarizes one AI/Smart ability's state on a channel from
// /API/Event/ChnSmart/Get.
type SmartAbilityStatus struct {
	Category      string   `json:"category"`
	Ability       string   `json:"ability"` // e.g. "FaceDetection", "HumanVehicle", "LicensePlate", "LineCrossing", "Intrusion"
	State         string   `json:"state"`   // "On" or "Off"
	Enabled       bool     `json:"enabled"`
	MutualAbility []string `json:"mutualAbility,omitempty"`
}

// ChannelSmartSummary holds all AI/Smart abilities for one channel.
type ChannelSmartSummary struct {
	Channel     int                  `json:"channel"` // 0-based
	ChannelKey  string               `json:"channelKey"`
	ChannelName string               `json:"channelName"`
	Status      string               `json:"status"`
	Abilities   []SmartAbilityStatus `json:"abilities"`
}

// GetChnSmart reads /API/Event/ChnSmart/Get across all channels.
func (c *Client) GetChnSmart(ctx context.Context) ([]ChannelSmartSummary, error) {
	env, err := c.Call(ctx, "Event/ChnSmart/Get", map[string]any{})
	if err != nil {
		return nil, err
	}
	var payload struct {
		ChannelInfo map[string]struct {
			Status      string `json:"status"`
			ChannelName string `json:"channel_name"`
			Abilities   []struct {
				Title       string `json:"title"`
				AbilityInfo []struct {
					Ability       string `json:"ability"`
					State         string `json:"state"`
					MutualAbility []struct {
						Channel string   `json:"channel"`
						Ability []string `json:"ability"`
					} `json:"mutual_ability"`
				} `json:"ability_info"`
			} `json:"abilities"`
		} `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, fmt.Errorf("raysharp Event/ChnSmart/Get decode: %w", err)
	}

	var out []ChannelSmartSummary
	for k, v := range payload.ChannelInfo {
		chIdx, ok := ParseChannelKey(k)
		if !ok {
			continue
		}
		if strings.EqualFold(v.Status, "Offline") && len(v.Abilities) == 0 {
			continue
		}
		sum := ChannelSmartSummary{
			Channel:     chIdx,
			ChannelKey:  k,
			ChannelName: v.ChannelName,
			Status:      v.Status,
		}
		if sum.Status == "" {
			sum.Status = "Online"
		}
		for _, grp := range v.Abilities {
			for _, ab := range grp.AbilityInfo {
				var mut []string
				for _, m := range ab.MutualAbility {
					mut = append(mut, m.Ability...)
				}
				sum.Abilities = append(sum.Abilities, SmartAbilityStatus{
					Category:      grp.Title,
					Ability:       ab.Ability,
					State:         ab.State,
					Enabled:       strings.EqualFold(ab.State, "On"),
					MutualAbility: mut,
				})
			}
		}
		out = append(out, sum)
	}
	return out, nil
}

// GetAISetting reads /API/Event/SettingConfig/Get (and optionally /Range) for a
// specific channel and AI ability (e.g. "HumanVehicle", "FaceDetection",
// "LicensePlate", "LineCrossing", "Intrusion").
func (c *Client) GetAISetting(ctx context.Context, channel int, ability string) (data map[string]any, rng map[string]any, err error) {
	chKey := ChannelKey(channel)
	req := map[string]any{
		"channel": chKey,
		"ability": ability,
	}
	env, err := c.Call(ctx, "Event/SettingConfig/Get", req)
	if err != nil {
		return nil, nil, err
	}
	var payload struct {
		ChannelInfo map[string]map[string]any `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, nil, fmt.Errorf("raysharp Event/SettingConfig/Get decode: %w", err)
	}
	data = payload.ChannelInfo[chKey]
	if data == nil {
		data = map[string]any{}
	}

	if rEnv, err := c.Call(ctx, "Event/SettingConfig/Range", req); err == nil {
		_ = json.Unmarshal(rEnv.Data, &rng)
	}
	return data, rng, nil
}

// SetAISetting updates /API/Event/SettingConfig/Set for one channel and ability
// using GET-Modify-SET so rule_info polygons/lines and unrelated fields stay intact.
func (c *Client) SetAISetting(ctx context.Context, channel int, ability string, changes map[string]any) (map[string]any, error) {
	chKey := ChannelKey(channel)
	cur, _, err := c.GetAISetting(ctx, channel, ability)
	if err != nil {
		return nil, err
	}
	for k, v := range changes {
		cur[k] = v
	}
	setReq := map[string]any{
		"channel": chKey,
		"ability": ability,
		"channel_info": map[string]any{
			chKey: cur,
		},
	}
	if _, err := c.Call(ctx, "Event/SettingConfig/Set", setReq); err != nil {
		return nil, err
	}
	updated, _, err := c.GetAISetting(ctx, channel, ability)
	return updated, err
}

// GetAIAlarmLinkage reads /API/Event/AlarmLinkage/Get for a channel and ability.
func (c *Client) GetAIAlarmLinkage(ctx context.Context, channel int, ability string) (map[string]any, error) {
	chKey := ChannelKey(channel)
	env, err := c.Call(ctx, "Event/AlarmLinkage/Get", map[string]any{
		"channel": chKey,
		"ability": ability,
	})
	if err != nil {
		return nil, err
	}
	var payload struct {
		ChannelInfo map[string]map[string]any `json:"channel_info"`
	}
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, err
	}
	return payload.ChannelInfo[chKey], nil
}

// SetAIAlarmLinkage updates /API/Event/AlarmLinkage/Set for a channel and ability.
func (c *Client) SetAIAlarmLinkage(ctx context.Context, channel int, ability string, changes map[string]any) (map[string]any, error) {
	chKey := ChannelKey(channel)
	cur, err := c.GetAIAlarmLinkage(ctx, channel, ability)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		cur = map[string]any{}
	}
	for k, v := range changes {
		cur[k] = v
	}
	setReq := map[string]any{
		"channel": chKey,
		"ability": ability,
		"channel_info": map[string]any{
			chKey: cur,
		},
	}
	if _, err := c.Call(ctx, "Event/AlarmLinkage/Set", setReq); err != nil {
		return nil, err
	}
	return c.GetAIAlarmLinkage(ctx, channel, ability)
}

// ConfigureEventPush configures the NVR's /API/AlarmConfig/EventPush/Set or IPC's
// /API/Event/Notification/Set to push real-time HTTP webhook events to targetHost:targetPort/targetPath.
func (c *Client) ConfigureEventPush(ctx context.Context, enable bool, targetHost string, targetPort int, targetPath string) error {
	targetPath = strings.TrimPrefix(strings.TrimSpace(targetPath), "/")
	if targetPath == "" {
		targetPath = "api/events/webhook/raysharp"
	}
	// Try NVR EventPush first
	if env, err := c.Call(ctx, "AlarmConfig/EventPush/Get", map[string]any{}); err == nil {
		var p struct {
			Params map[string]any `json:"params"`
		}
		if json.Unmarshal(env.Data, &p) == nil && p.Params != nil {
			tbl, _ := p.Params["table"].(map[string]any)
			if tbl == nil {
				tbl = map[string]any{}
			}
			tbl["enable"] = enable
			tbl["addr"] = targetHost
			tbl["port"] = targetPort
			tbl["url"] = targetPath
			tbl["method"] = "POST"
			tbl["push_way"] = "HTTP"
			p.Params["table"] = tbl
			_, err := c.Call(ctx, "AlarmConfig/EventPush/Set", map[string]any{"params": p.Params})
			return err
		}
	}
	// Fallback to IPC Event/Notification/Set on CH1
	if env, err := c.Call(ctx, "Event/Notification/Get", map[string]any{"channel": "CH1", "ability": "HumanVehicle"}); err == nil {
		var p struct {
			ChannelInfo map[string]map[string]any `json:"channel_info"`
		}
		if json.Unmarshal(env.Data, &p) == nil && p.ChannelInfo["CH1"] != nil {
			ch1 := p.ChannelInfo["CH1"]
			ns, _ := ch1["notification_settings"].(map[string]any)
			if ns == nil {
				ns = map[string]any{}
			}
			url1, _ := ns["URL1"].(map[string]any)
			if url1 == nil {
				url1 = map[string]any{}
			}
			url1["enable"] = enable
			url1["push_way"] = "HTTP"
			url1["method"] = "POST"
			url1["url"] = fmt.Sprintf("http://%s:%d/%s", targetHost, targetPort, targetPath)
			ns["URL1"] = url1
			ch1["notification_settings"] = ns
			_, err := c.Call(ctx, "Event/Notification/Set", map[string]any{
				"channel":      "CH1",
				"ability":      "HumanVehicle",
				"channel_info": map[string]any{"CH1": ch1},
			})
			return err
		}
	}
	return fmt.Errorf("raysharp: neither AlarmConfig/EventPush nor Event/Notification supported")
}

// EventCheckCursor tracks the ring-buffer cursor returned by /API/Event/Check.
type EventCheckCursor struct {
	Initialized bool `json:"initialized"`
	ReaderID    int  `json:"reader_id"`
	Sequence    int  `json:"sequence"`
	LapNumber   int  `json:"lap_number"`
}

// CheckEventItem is a normalized real-time event extracted from /API/Event/Check
// or an inbound Raysharp webhook.
type CheckEventItem struct {
	Channel       int            `json:"channel"` // 0-based (-1 for system-level events)
	ChannelKey    string         `json:"channelKey,omitempty"`
	ChannelName   string         `json:"channelName,omitempty"`
	EventType     string         `json:"eventType"` // e.g. "human_vehicle", "face_detection", "license_plate", "line_crossing", "intrusion", "motion", "videoloss", "disk_error"
	RawSubType    string         `json:"rawSubType,omitempty"`
	Timestamp     string         `json:"timestamp"`
	Online        bool           `json:"online"`
	SnapshotB64   string         `json:"snapshotB64,omitempty"`
	BackgroundB64 string         `json:"backgroundB64,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// PollEventCheck calls POST /API/Event/Check with plus_eventchk="eventAiPushPic"
// and advances cursor.
func (c *Client) PollEventCheck(ctx context.Context, cursor *EventCheckCursor) ([]CheckEventItem, error) {
	req := map[string]any{
		"plus_eventchk": "eventAiPushPic",
		"ext_data": map[string]any{
			"subscribe_type": []map[string]any{
				{"event": []string{"all"}},
				{"aipushpic": []string{"all"}},
			},
		},
	}
	if cursor != nil && cursor.Initialized {
		req["reader_id"] = cursor.ReaderID
		req["sequence"] = cursor.Sequence
		req["lap_number"] = cursor.LapNumber
	}

	env, err := c.Call(ctx, "Event/Check", req)
	if err != nil {
		if env != nil && (env.ErrorCode == "readerID_invalid" || env.ErrorCode == "position_invalid") && cursor != nil {
			cursor.Initialized = false
		}
		return nil, err
	}

	var resp struct {
		ReaderID   *int   `json:"reader_id"`
		Sequence   *int   `json:"sequence"`
		LapNumber  *int   `json:"lap_number"`
		DateFormat string `json:"date_format"`
		AlarmList  []struct {
			Time        string `json:"time"`
			SystemAlarm []struct {
				DeviceName string `json:"device_name"`
				HDD        *struct {
					HDDAlarmType string `json:"hdd_alarm_type"`
					HDDAlarmInfo string `json:"hdd_alarm_info"`
				} `json:"hdd"`
			} `json:"system_alarm"`
			ChannelAlarm []struct {
				Channel             string `json:"channel"`
				ChannelName         string `json:"channel_name"`
				MotionAlarm         bool   `json:"motion_alarm"`
				IOAlarm             bool   `json:"io_alarm"`
				VideoLoss           bool   `json:"videoloss"`
				CameraConnectStatus struct {
					ConnectStatus string `json:"connect_status"`
				} `json:"camera_connect_status"`
				AlarmState []struct {
					IntSubType string `json:"int_subtype"`
				} `json:"alarm_state"`
			} `json:"channel_alarm"`
		} `json:"alarm_list"`
		AISnapPicture *struct {
			FaceInfo      []map[string]any `json:"FaceInfo"`
			SnapedObjInfo []map[string]any `json:"SnapedObjInfo"`
			PlateInfo     []map[string]any `json:"PlateInfo"`
		} `json:"ai_snap_picture"`
	}
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		return nil, fmt.Errorf("raysharp Event/Check decode: %w", err)
	}

	if cursor != nil {
		if resp.ReaderID != nil {
			cursor.ReaderID = *resp.ReaderID
			cursor.Initialized = true
		}
		if resp.Sequence != nil {
			cursor.Sequence = *resp.Sequence
		}
		if resp.LapNumber != nil {
			cursor.LapNumber = *resp.LapNumber
		}
	}

	var events []CheckEventItem
	for _, al := range resp.AlarmList {
		for _, sys := range al.SystemAlarm {
			if sys.HDD != nil && sys.HDD.HDDAlarmInfo != "" {
				events = append(events, CheckEventItem{
					Channel:    -1,
					EventType:  "disk_" + strings.ToLower(sys.HDD.HDDAlarmInfo),
					RawSubType: sys.HDD.HDDAlarmType + ":" + sys.HDD.HDDAlarmInfo,
					Timestamp:  al.Time,
					Online:     true,
					Metadata: map[string]any{
						"device_name":    sys.DeviceName,
						"hdd_alarm_type": sys.HDD.HDDAlarmType,
						"hdd_alarm_info": sys.HDD.HDDAlarmInfo,
					},
				})
			}
		}
		for _, ch := range al.ChannelAlarm {
			if strings.EqualFold(ch.CameraConnectStatus.ConnectStatus, "NotConfigured") {
				continue
			}
			chIdx, _ := ParseChannelKey(ch.Channel)
			online := strings.EqualFold(ch.CameraConnectStatus.ConnectStatus, "Online")
			if ch.MotionAlarm {
				events = append(events, CheckEventItem{
					Channel:     chIdx,
					ChannelKey:  ch.Channel,
					ChannelName: ch.ChannelName,
					EventType:   "motion",
					RawSubType:  "motion_alarm",
					Timestamp:   al.Time,
					Online:      online,
				})
			}
			for _, st := range ch.AlarmState {
				if st.IntSubType == "" {
					continue
				}
				events = append(events, CheckEventItem{
					Channel:     chIdx,
					ChannelKey:  ch.Channel,
					ChannelName: ch.ChannelName,
					EventType:   normalizeAISubType(st.IntSubType),
					RawSubType:  st.IntSubType,
					Timestamp:   al.Time,
					Online:      online,
				})
			}
		}
	}

	if resp.AISnapPicture != nil {
		for _, face := range resp.AISnapPicture.FaceInfo {
			events = append(events, parseAISnapItem("face_detection", face))
		}
		for _, obj := range resp.AISnapPicture.SnapedObjInfo {
			events = append(events, parseAISnapItem("human_vehicle", obj))
		}
		for _, plate := range resp.AISnapPicture.PlateInfo {
			events = append(events, parseAISnapItem("license_plate", plate))
		}
	}
	return events, nil
}

func normalizeAISubType(sub string) string {
	s := strings.ToLower(strings.TrimSpace(sub))
	switch s {
	case "pd_vd", "pd&vd", "pvd", "humanvehicle":
		return "human_vehicle"
	case "fd", "facedetection":
		return "face_detection"
	case "lpd", "licenseplate":
		return "license_plate"
	case "lcd", "linecrossing":
		return "line_crossing"
	case "sod", "objectdetection":
		return "object_detection"
	case "cc", "crosscount", "crosscounting":
		return "cross_counting"
	case "intrusion":
		return "intrusion"
	case "regionentrance", "enterregion":
		return "region_entrance"
	case "regionexiting", "exitregion":
		return "region_exiting"
	default:
		return s
	}
}

func parseAISnapItem(defaultType string, m map[string]any) CheckEventItem {
	chStr, _ := m["Chn"].(string)
	if chStr == "" {
		if chNum, ok := m["Chn"].(float64); ok {
			chStr = ChannelKey(int(chNum))
		}
	}
	chIdx, ok := ParseChannelKey(chStr)
	if !ok {
		chIdx = 0
		chStr = "CH1"
	}
	snapB64, _ := m["FaceImg"].(string)
	if snapB64 == "" {
		snapB64, _ = m["ObjectImg"].(string)
	}
	if snapB64 == "" {
		snapB64, _ = m["PlateImg"].(string)
	}
	bgB64, _ := m["BgImg"].(string)
	if bgB64 == "" {
		bgB64, _ = m["BackgroudImg"].(string)
	}
	ts, _ := m["StartTime"].(string)

	meta := make(map[string]any)
	for k, v := range m {
		if k == "FaceImg" || k == "ObjectImg" || k == "PlateImg" || k == "BgImg" || k == "BackgroudImg" {
			continue
		}
		meta[k] = v
	}
	return CheckEventItem{
		Channel:       chIdx,
		ChannelKey:    chStr,
		EventType:     defaultType,
		Timestamp:     ts,
		Online:        true,
		SnapshotB64:   snapB64,
		BackgroundB64: bgB64,
		Metadata:      meta,
	}
}
