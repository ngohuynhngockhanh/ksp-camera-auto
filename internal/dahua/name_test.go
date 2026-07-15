package dahua

import "testing"

func TestChannelObj(t *testing.T) {
	table := []any{
		map[string]any{"Name": "Cổng chính"},
		map[string]any{"Name": "Sân sau"},
	}
	obj, err := channelObj(table, "ChannelTitle", 0)
	if err != nil {
		t.Fatalf("channel 0: %v", err)
	}
	if obj["Name"] != "Cổng chính" {
		t.Errorf("wrong name: %v", obj["Name"])
	}
	if _, err := channelObj(table, "ChannelTitle", 5); err == nil {
		t.Error("expected out-of-range channel error")
	}
}

func TestCustomTitleSlots(t *testing.T) {
	table := []any{
		map[string]any{
			"CustomTitle": []any{
				map[string]any{"Text": "line1", "EncodeBlend": true},
				map[string]any{"Text": "", "EncodeBlend": false},
			},
		},
	}
	slots, err := customTitleSlots(table, 0)
	if err != nil {
		t.Fatalf("channel 0: %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("want 2 slots, got %d", len(slots))
	}
	first := slots[0].(map[string]any)
	if first["Text"] != "line1" {
		t.Errorf("wrong text: %v", first["Text"])
	}

	// A channel object with no CustomTitle array (e.g. firmware exposes the
	// table without the nested array) must fail loudly, not return nil,nil.
	noArray := []any{map[string]any{}}
	if _, err := customTitleSlots(noArray, 0); err == nil {
		t.Error("expected error for missing CustomTitle array")
	}
}
