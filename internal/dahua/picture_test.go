package dahua

import "testing"

func TestVideoColorObj(t *testing.T) {
	table := []any{
		[]any{map[string]any{"Brightness": float64(50)}},
	}
	obj, err := videoColorObj(table, 0)
	if err != nil {
		t.Fatalf("channel 0: %v", err)
	}
	if toInt(obj["Brightness"]) != 50 {
		t.Errorf("wrong brightness: %v", obj["Brightness"])
	}
	if _, err := videoColorObj(table, 5); err == nil {
		t.Error("expected out-of-range channel error")
	}
	empty := []any{[]any{}}
	if _, err := videoColorObj(empty, 0); err == nil {
		t.Error("expected error for empty color config array")
	}
}

func TestMergeNested(t *testing.T) {
	dst := map[string]any{
		"Flip": false,
		"NightOptions": map[string]any{
			"GainRed":   float64(50),
			"GainGreen": float64(50),
		},
	}
	src := map[string]any{
		"Flip": true,
		"NightOptions": map[string]any{
			"GainRed": float64(80),
		},
	}
	mergeNested(dst, src)
	if dst["Flip"] != true {
		t.Errorf("Flip not merged: %v", dst["Flip"])
	}
	night := dst["NightOptions"].(map[string]any)
	if toInt(night["GainRed"]) != 80 {
		t.Errorf("GainRed not merged: %v", night["GainRed"])
	}
	if toInt(night["GainGreen"]) != 50 {
		t.Errorf("GainGreen clobbered, want untouched 50: %v", night["GainGreen"])
	}
}

func TestMergeNestedReplacesArraysWholesale(t *testing.T) {
	dst := map[string]any{
		"NightOptions": map[string]any{
			"BacklightRegion": []any{float64(1), float64(2), float64(3), float64(4)},
		},
	}
	src := map[string]any{
		"NightOptions": map[string]any{
			"BacklightRegion": []any{float64(9), float64(9)},
		},
	}
	mergeNested(dst, src)
	region := dst["NightOptions"].(map[string]any)["BacklightRegion"].([]any)
	if len(region) != 2 {
		t.Fatalf("expected array replaced wholesale (len 2), got %v", region)
	}
}

func TestParseCapsLines(t *testing.T) {
	body := "caps.Flip=true\r\ncaps.Mirror=false\r\ncaps.WhiteBalance=2\r\n\r\n"
	caps := parseCapsLines(body)
	if caps["Flip"] != "true" {
		t.Errorf("Flip = %v", caps["Flip"])
	}
	if caps["Mirror"] != "false" {
		t.Errorf("Mirror = %v", caps["Mirror"])
	}
	if caps["WhiteBalance"] != "2" {
		t.Errorf("WhiteBalance = %v", caps["WhiteBalance"])
	}
}
