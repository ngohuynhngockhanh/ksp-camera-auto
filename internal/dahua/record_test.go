package dahua

import (
	"encoding/json"
	"testing"
)

func TestParseRecordState(t *testing.T) {
	record := []any{
		map[string]any{"Enable": false, "TimeSection": []any{"0 00:00:00-24:00:00 1"}},
		map[string]any{"Enable": true},
	}
	mode := []any{map[string]any{"Mode": float64(0)}, map[string]any{"Mode": float64(1)}}
	got := parseRecordState(record, mode, 2)
	if len(got) != 2 || got[0].Enable || got[0].Mode != 0 || !got[1].Enable || got[1].Mode != 1 {
		t.Fatalf("unexpected state: %#v", got)
	}
}

func TestRepairRecordTablesPreservesUnrelatedConfig(t *testing.T) {
	record := []any{map[string]any{"Enable": false, "PreRecord": float64(7), "TimeSection": []any{"old"}}}
	mode := []any{map[string]any{"Mode": float64(2), "Extra": "keep"}}
	newRecord, newMode := repairRecordTables(record, mode, []int{0})
	r := newRecord[0].(map[string]any)
	m := newMode[0].(map[string]any)
	if r["Enable"] != true || r["PreRecord"] != float64(7) || m["Mode"] != 1 || m["Extra"] != "keep" {
		t.Fatalf("repair lost config: record=%#v mode=%#v", r, m)
	}
	if r["MaxRecordTime"] != 300 {
		t.Fatalf("MaxRecordTime=%#v, want 300 seconds", r["MaxRecordTime"])
	}
	sections, ok := r["TimeSection"].([]any)
	if !ok || len(sections) != 7 {
		t.Fatalf("expected seven-day timing schedule, got %#v", r["TimeSection"])
	}
	day := sections[0].([]any)
	if len(day) != 6 || day[0] != "1 00:00:00-24:00:00" || day[1] != "0 00:00:00-24:00:00" {
		t.Fatalf("unexpected daily schedule: %#v", day)
	}
}

func TestParseUptimeVariants(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{`{"result":86400}`, 86400},
		{`{"params":{"upTime":123}}`, 123},
		{`{"params":{"UpTime":"456"}}`, 456},
	} {
		var resp rpcResp
		if err := json.Unmarshal([]byte(tc.raw), &resp); err != nil {
			t.Fatal(err)
		}
		got, err := parseUptime(resp)
		if err != nil || got != tc.want {
			t.Fatalf("parse %s = %d, %v; want %d", tc.raw, got, err, tc.want)
		}
	}
}

func TestSetRecordModesPreservesExtraFields(t *testing.T) {
	in := []any{map[string]any{"Mode": float64(1), "ModeExtra1": float64(2)}}
	out := setRecordModes(in, []int{0}, 2)
	row := out[0].(map[string]any)
	if row["Mode"] != 2 || row["ModeExtra1"] != float64(2) {
		t.Fatalf("unexpected row: %#v", row)
	}
}
