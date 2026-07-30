package dahua

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// RecordChannelState is the recording switch/mode read from the NVR.
type RecordChannelState struct {
	Channel    int  `json:"channel"`
	Enable     bool `json:"enable"`
	Mode       int  `json:"mode"`
	Timing24x7 bool `json:"timing24x7"`
}

func parseRecordState(record, mode []any, count int) []RecordChannelState {
	out := make([]RecordChannelState, 0, count)
	for i := 0; i < count; i++ {
		state := RecordChannelState{Channel: i}
		if i < len(record) {
			if row, ok := record[i].(map[string]any); ok {
				state.Enable, _ = row["Enable"].(bool)
				state.Timing24x7 = isTiming24x7(row["TimeSection"])
			}
		}
		if i < len(mode) {
			if row, ok := mode[i].(map[string]any); ok {
				state.Mode = anyInt(row["Mode"])
			}
		}
		out = append(out, state)
	}
	return out
}

func anyInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func timingSchedule() []any {
	out := make([]any, 7)
	for day := range out {
		periods := make([]any, 6)
		periods[0] = "1 00:00:00-24:00:00"
		for period := 1; period < len(periods); period++ {
			periods[period] = "0 00:00:00-24:00:00"
		}
		out[day] = periods
	}
	return out
}

func isTiming24x7(value any) bool {
	return countEnabledFullDays(value) >= 7
}

func countEnabledFullDays(value any) int {
	switch x := value.(type) {
	case string:
		if x == "1 00:00:00-24:00:00" {
			return 1
		}
	case []any:
		total := 0
		for _, item := range x {
			total += countEnabledFullDays(item)
		}
		return total
	case map[string]any:
		total := 0
		for _, item := range x {
			total += countEnabledFullDays(item)
		}
		return total
	}
	return 0
}

func cloneRows(in []any) []any {
	out := make([]any, len(in))
	for i, value := range in {
		if row, ok := value.(map[string]any); ok {
			copyRow := make(map[string]any, len(row))
			for k, v := range row {
				copyRow[k] = v
			}
			out[i] = copyRow
		} else {
			out[i] = value
		}
	}
	return out
}

func repairRecordTables(record, mode []any, channels []int) ([]any, []any) {
	record, mode = cloneRows(record), cloneRows(mode)
	for _, ch := range channels {
		for len(record) <= ch {
			record = append(record, map[string]any{})
		}
		for len(mode) <= ch {
			mode = append(mode, map[string]any{})
		}
		r, ok := record[ch].(map[string]any)
		if !ok {
			r = map[string]any{}
		}
		m, ok := mode[ch].(map[string]any)
		if !ok {
			m = map[string]any{}
		}
		r["Enable"] = true
		r["TimeSection"] = timingSchedule()
		r["MaxRecordTime"] = 300
		m["Mode"] = 1
		record[ch], mode[ch] = r, m
	}
	return record, mode
}

func setRecordModes(mode []any, channels []int, value int) []any {
	mode = cloneRows(mode)
	for _, ch := range channels {
		for len(mode) <= ch {
			mode = append(mode, map[string]any{})
		}
		row, ok := mode[ch].(map[string]any)
		if !ok {
			row = map[string]any{}
		}
		row["Mode"] = value
		mode[ch] = row
	}
	return mode
}

func tableArray(table any) ([]any, error) {
	rows, ok := table.([]any)
	if !ok {
		return nil, fmt.Errorf("expected config array, got %T", table)
	}
	return rows, nil
}

// GetRecordState reads Record and RecordMode without changing the schedule.
func (c *Client) GetRecordState(channelCount int) ([]RecordChannelState, error) {
	recordRaw, err := c.getTable("Record")
	if err != nil {
		return nil, err
	}
	modeRaw, err := c.getTable("RecordMode")
	if err != nil {
		return nil, err
	}
	record, err := tableArray(recordRaw)
	if err != nil {
		return nil, err
	}
	mode, err := tableArray(modeRaw)
	if err != nil {
		return nil, err
	}
	if channelCount <= 0 {
		channelCount = len(record)
		if len(mode) > channelCount {
			channelCount = len(mode)
		}
	}
	return parseRecordState(record, mode, channelCount), nil
}

// EnableTimingRecord repairs only selected channels and preserves unrelated fields.
func (c *Client) EnableTimingRecord(channels []int) error {
	recordRaw, err := c.getTable("Record")
	if err != nil {
		return err
	}
	modeRaw, err := c.getTable("RecordMode")
	if err != nil {
		return err
	}
	record, err := tableArray(recordRaw)
	if err != nil {
		return err
	}
	mode, err := tableArray(modeRaw)
	if err != nil {
		return err
	}
	record, mode = repairRecordTables(record, mode, channels)
	if err := c.setTable("Record", record); err != nil {
		return err
	}
	return c.setTable("RecordMode", mode)
}

// RestartRecording creates the mode transition this firmware needs after a
// power loss: off, then manual continuous. Merely reading Mode=1 is not enough
// because its recorder process may not have started.
func (c *Client) RestartRecording(channels []int) error {
	mode, err := c.getTable("RecordMode")
	if err != nil {
		return err
	}
	if err := c.setTable("RecordMode", setRecordModes(mode, channels, 2)); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	return c.setTable("RecordMode", setRecordModes(mode, channels, 1))
}

// Uptime returns seconds since the NVR booted.
func (c *Client) Uptime() (int64, error) {
	resp, err := c.Call("magicBox.getUpTime", nil)
	if err != nil {
		return 0, err
	}
	return parseUptime(resp)
}

func parseUptime(resp rpcResp) (int64, error) {
	var direct any
	if len(resp.Result) > 0 && json.Unmarshal(resp.Result, &direct) == nil {
		if n := int64(anyInt(direct)); n > 0 {
			return n, nil
		}
	}
	var params map[string]any
	if json.Unmarshal(resp.Params, &params) == nil {
		for _, key := range []string{"upTime", "UpTime", "uptime"} {
			if n := int64(anyInt(params[key])); n > 0 {
				return n, nil
			}
		}
		if n := findPositiveNumber(params); n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("magicBox.getUpTime returned no uptime (result=%s params=%s)", resp.Result, resp.Params)
}

func findPositiveNumber(v any) int64 {
	if n := int64(anyInt(v)); n > 0 {
		return n
	}
	switch x := v.(type) {
	case map[string]any:
		for _, value := range x {
			if n := findPositiveNumber(value); n > 0 {
				return n
			}
		}
	case []any:
		for _, value := range x {
			if n := findPositiveNumber(value); n > 0 {
				return n
			}
		}
	}
	return 0
}
