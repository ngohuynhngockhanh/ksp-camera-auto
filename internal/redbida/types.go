// Package redbida integrates kspcam with the local ota-mqtt key/value bridge.
// Node-RED is intentionally treated as a read-only survey surface.
package redbida

import (
	"context"
	"encoding/json"
)

type Risk string

const (
	RiskEditable  Risk = "editable"
	RiskConfirm   Risk = "confirm-required"
	RiskProtected Risk = "read-only-protected"
	RiskUnknown   Risk = "unknown"
)

type ValueType string

const (
	TypeString  ValueType = "string"
	TypeNumber  ValueType = "number"
	TypeBoolean ValueType = "boolean"
	TypeJSON    ValueType = "json"
	TypeImage   ValueType = "image"
	TypeUnknown ValueType = "unknown"
)

type KeyMeta struct {
	Key         string    `json:"key"`
	Label       string    `json:"label"`
	Group       string    `json:"group"`
	Description string    `json:"description,omitempty"`
	Risk        Risk      `json:"risk"`
	ValueType   ValueType `json:"valueType"`
	Editable    bool      `json:"editable"`
	Secret      bool      `json:"secret"`
}

type KeyValue struct {
	Key    string  `json:"key"`
	Meta   KeyMeta `json:"meta"`
	Value  any     `json:"value,omitempty"`
	Exists bool    `json:"exists"`
}

type WriteAck struct {
	OldValue any `json:"oldValue,omitempty"`
	NewValue any `json:"newValue,omitempty"`
}

type ChangeResult struct {
	Key          string  `json:"key"`
	Meta         KeyMeta `json:"meta"`
	OldValue     any     `json:"oldValue,omitempty"`
	NewValue     any     `json:"newValue,omitempty"`
	Changed      bool    `json:"changed"`
	Acknowledged bool    `json:"acknowledged"`
	ReadBack     bool    `json:"readBack"`
	Verified     bool    `json:"verified"`
	Applied      bool    `json:"applied"`
	Error        string  `json:"error,omitempty"`
}

type Broker interface {
	Read(ctx context.Context, keys []string) (map[string]any, error)
	Write(ctx context.Context, changes map[string]any) (map[string]WriteAck, error)
}

func inferType(v any, key string) ValueType {
	if isLogoKey(key) {
		return TypeImage
	}
	switch v.(type) {
	case bool:
		return TypeBoolean
	case float64, float32, int, int64, uint64:
		return TypeNumber
	case map[string]any, []any:
		return TypeJSON
	case string:
		return TypeString
	default:
		return TypeUnknown
	}
}

func isLogoKey(key string) bool {
	return key == "logo_header" || key == "logo_livestream" || key == "logo_cat_cam"
}

func redact(meta KeyMeta, value any) any {
	if meta.Secret {
		return "********"
	}
	return value
}

func cloneJSONValue(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(b, &out) != nil {
		return v
	}
	return out
}
