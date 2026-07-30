package dahua

import (
	"encoding/json"
	"testing"
)

func TestParseSerialNumberSupportsDahuaResponseVariants(t *testing.T) {
	tests := []struct {
		name   string
		result string
		params string
		want   string
	}{
		{name: "serial in params", result: `true`, params: `{"serialNumber":"8K01234PAZ56789"}`, want: "8K01234PAZ56789"},
		{name: "serialNo key", result: `true`, params: `{"SerialNo":"DH-ABC-123"}`, want: "DH-ABC-123"},
		{name: "result string", result: `"SN37777XYZ"`, params: `null`, want: "SN37777XYZ"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSerialNumber(json.RawMessage(tt.result), json.RawMessage(tt.params))
			if got != tt.want {
				t.Fatalf("parseSerialNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}
