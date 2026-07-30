package dahua

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GetSerialNumber reads the device serial through the authenticated DVRIP
// session. Firmware variants return it either directly in result or nested
// below params with differently-cased field names.
func (c *Client) GetSerialNumber() (string, error) {
	resp, err := c.Call("magicBox.getSerialNo", nil)
	if err != nil {
		return "", err
	}
	if !resp.ok() {
		return "", fmt.Errorf("magicBox.getSerialNo failed: %s", respErr(resp))
	}
	serial := parseSerialNumber(resp.Result, resp.Params)
	if serial == "" {
		return "", fmt.Errorf("magicBox.getSerialNo returned no serial number")
	}
	return serial, nil
}

func parseSerialNumber(result, params json.RawMessage) string {
	for _, raw := range []json.RawMessage{params, result} {
		var node any
		if len(raw) == 0 || json.Unmarshal(raw, &node) != nil {
			continue
		}
		if serial, ok := node.(string); ok {
			return strings.TrimSpace(serial)
		}
		if serial := findSerialNumber(node); serial != "" {
			return serial
		}
	}
	return ""
}

func findSerialNumber(node any) string {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			normalized := strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(key))
			if normalized == "serialnumber" || normalized == "serialno" || normalized == "sn" {
				if serial, ok := child.(string); ok {
					return strings.TrimSpace(serial)
				}
			}
		}
		for _, child := range value {
			if serial := findSerialNumber(child); serial != "" {
				return serial
			}
		}
	case []any:
		for _, child := range value {
			if serial := findSerialNumber(child); serial != "" {
				return serial
			}
		}
	}
	return ""
}
