package dahua

import "testing"

func TestParseNTPConfigLines(t *testing.T) {
	v := parseConfigLines("table.NTP.Enable=true\r\ntable.NTP.Address=time.google.com\r\ntable.NTP.TimeZone=12\r\n")
	if v["Enable"] != "true" || v["Address"] != "time.google.com" || v["TimeZone"] != "12" {
		t.Fatalf("unexpected config: %#v", v)
	}
}
