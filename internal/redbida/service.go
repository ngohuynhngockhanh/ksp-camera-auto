package redbida

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	broker   Broker
	catalog  *Catalog
	maxBatch int
}

func NewService(broker Broker, catalog *Catalog, maxBatch int) *Service {
	if maxBatch < 1 {
		maxBatch = 200
	}
	if catalog == nil {
		catalog = NewCatalog("")
	}
	return &Service{broker: broker, catalog: catalog, maxBatch: maxBatch}
}

func (s *Service) Catalog() []KeyMeta { return s.catalog.List() }

func (s *Service) CatalogStatus() (bool, string) { return s.catalog.Status() }

func (s *Service) Refresh(ctx context.Context, keys []string) ([]KeyValue, error) {
	keys, err := normalizeKeys(keys, s.maxBatch)
	if err != nil {
		return nil, err
	}
	s.catalog.List()
	available, sourceErr := s.catalog.Status()
	if !available {
		return nil, fmt.Errorf("catalog source unavailable: %s", sourceErr)
	}
	readKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if s.catalog.Present(key) && !s.catalog.Empty(key) {
			readKeys = append(readKeys, key)
		}
	}
	values := map[string]any{}
	if len(readKeys) > 0 {
		values, err = s.broker.Read(ctx, readKeys)
		if err != nil {
			return nil, err
		}
	}
	out := make([]KeyValue, 0, len(keys))
	for _, key := range keys {
		value, returned := values[key]
		exists := s.catalog.Present(key)
		if exists && s.catalog.Empty(key) {
			value = ""
		}
		meta, ok := s.catalog.Meta(key)
		if !ok {
			meta = metaForKey(key, "", "")
			meta.ValueType = valueTypeForKey(key, value)
		}
		value = normalizeValue(meta, value)
		if returned || (exists && s.catalog.Empty(key)) {
			s.catalog.Observe(key, value)
		}
		out = append(out, KeyValue{Key: key, Meta: meta, Value: redact(meta, cloneJSONValue(value)), Exists: exists})
	}
	return out, nil
}

func (s *Service) Apply(ctx context.Context, changes map[string]any, confirmed bool) ([]ChangeResult, error) {
	if len(changes) == 0 {
		return nil, fmt.Errorf("changes is required")
	}
	if len(changes) > s.maxBatch {
		return nil, fmt.Errorf("too many keys: max %d", s.maxBatch)
	}
	s.catalog.List()
	if available, sourceErr := s.catalog.Status(); !available {
		return nil, fmt.Errorf("catalog source unavailable: %s", sourceErr)
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	validated := make(map[string]any, len(changes))
	results := make([]ChangeResult, 0, len(changes))
	resultIndex := make(map[string]int, len(changes))
	for _, key := range keys {
		value := changes[key]
		meta, ok := s.catalog.Meta(key)
		if !ok {
			meta = metaForKey(key, "", "")
		}
		result := ChangeResult{Key: key, Meta: meta}
		switch {
		case !validKeyName(key):
			result.Error = "invalid key"
		case !meta.Editable:
			result.Error = "key is read-only"
		case meta.Risk == RiskConfirm && !confirmed:
			result.Error = "confirmation is required"
		default:
			if err := validateValue(meta, value); err != nil {
				result.Error = err.Error()
			} else {
				validated[key] = value
				result.NewValue = redact(meta, cloneJSONValue(value))
			}
		}
		resultIndex[key] = len(results)
		results = append(results, result)
	}
	if len(validated) == 0 {
		return results, nil
	}

	acks, err := s.broker.Write(ctx, validated)
	if err != nil {
		var ackTimeout *AckTimeoutError
		if errors.As(err, &ackTimeout) {
			readBack, readErr := s.readBack(ctx, validated)
			for key, expected := range validated {
				result := &results[resultIndex[key]]
				if readErr != nil {
					result.Error = "write acknowledgement timed out; read-back failed: " + readErr.Error()
					continue
				}
				actual, ok := readBack[key]
				if !ok {
					result.Error = "write acknowledgement timed out; state unknown"
					continue
				}
				actual = normalizeValue(result.Meta, actual)
				result.NewValue = redact(result.Meta, cloneJSONValue(actual))
				result.ReadBack = true
				if !valuesEqual(expected, actual) {
					result.Error = "write acknowledgement timed out; read-back mismatch"
					continue
				}
				result.Verified = true
				result.Applied = true
			}
			return results, nil
		}
		return results, err
	}
	acknowledged := make(map[string]any, len(validated))
	for key, expected := range validated {
		result := &results[resultIndex[key]]
		ack, ok := acks[key]
		if !ok {
			result.Error = "missing acknowledgement"
			continue
		}
		result.Acknowledged = true
		result.OldValue = redact(result.Meta, cloneJSONValue(normalizeValue(result.Meta, ack.OldValue)))
		result.NewValue = redact(result.Meta, cloneJSONValue(normalizeValue(result.Meta, ack.NewValue)))
		acknowledged[key] = expected
	}
	if len(acknowledged) == 0 {
		return results, nil
	}

	readBack, readErr := s.readBack(ctx, acknowledged)
	for key, expected := range acknowledged {
		result := &results[resultIndex[key]]
		if readErr != nil {
			result.Error = "read-back failed: " + readErr.Error()
			continue
		}
		actual, ok := readBack[key]
		if !ok {
			result.Error = "missing read-back value"
			continue
		}
		actual = normalizeValue(result.Meta, actual)
		result.NewValue = redact(result.Meta, cloneJSONValue(actual))
		result.ReadBack = true
		result.Changed = !valuesEqual(result.OldValue, actual)
		if !valuesEqual(expected, actual) {
			result.Error = "read-back mismatch"
			continue
		}
		result.Verified = true
		result.Applied = true
	}
	return results, nil
}

func normalizeValue(meta KeyMeta, value any) any {
	text, isString := value.(string)
	if !isString {
		return value
	}
	switch meta.ValueType {
	case TypeBoolean:
		if parsed, err := strconv.ParseBool(strings.TrimSpace(text)); err == nil {
			return parsed
		}
	case TypeNumber:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
			return parsed
		}
	}
	return value
}

func (s *Service) readBack(ctx context.Context, expected map[string]any) (map[string]any, error) {
	var last map[string]any
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		s.catalog.List()
		last = make(map[string]any, len(expected))
		readKeys := make([]string, 0, len(expected))
		for key, expectedValue := range expected {
			if !s.catalog.Present(key) {
				continue
			}
			if text, ok := expectedValue.(string); ok && text == "" && s.catalog.Empty(key) {
				last[key] = ""
				continue
			}
			readKeys = append(readKeys, key)
		}
		sort.Strings(readKeys)
		lastErr = nil
		if len(readKeys) > 0 {
			var readValues map[string]any
			readValues, lastErr = s.broker.Read(ctx, readKeys)
			for key, value := range readValues {
				last[key] = value
			}
		}
		if lastErr == nil {
			matched := true
			for key, value := range expected {
				actual, ok := last[key]
				if !ok || !valuesEqual(value, actual) {
					matched = false
					break
				}
			}
			if matched {
				return last, nil
			}
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return last, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return last, lastErr
}

func normalizeKeys(keys []string, max int) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys))
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			continue
		}
		if !validKeyName(key) {
			return nil, fmt.Errorf("invalid key: %s", key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("keys is required")
	}
	if len(out) > max {
		return nil, fmt.Errorf("too many keys: max %d", max)
	}
	return out, nil
}

func validateValue(meta KeyMeta, value any) error {
	if value == nil {
		return fmt.Errorf("value cannot be null")
	}
	if text, ok := value.(string); ok && len(text) > 2*1024*1024 {
		return fmt.Errorf("value is too large")
	}
	switch meta.ValueType {
	case TypeImage:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("image value must be a string")
		}
		return validateImageValue(text)
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("value must be boolean")
		}
	case TypeNumber:
		number, ok := numberValue(value)
		if !ok {
			return fmt.Errorf("value must be numeric")
		}
		rule, ok := numericRules[meta.Key]
		if !ok {
			return fmt.Errorf("numeric policy is not defined for this key")
		}
		if number < rule.min || number > rule.max {
			return fmt.Errorf("value must be between %g and %g", rule.min, rule.max)
		}
		if rule.integer && number != float64(int64(number)) {
			return fmt.Errorf("value must be an integer")
		}
	case TypeJSON:
		switch value.(type) {
		case map[string]any, []any:
		default:
			return fmt.Errorf("value must be a JSON object or array")
		}
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("value must be a string")
		}
	}
	return nil
}

type numericRule struct {
	min     float64
	max     float64
	integer bool
}

var numericRules = map[string]numericRule{
	"camera_count":                      {min: 0, max: 4096, integer: true},
	"db_check_range":                    {min: 0, max: 1_000_000_000, integer: true},
	"db_check_rmlm":                     {min: 0, max: 1_000_000_000, integer: true},
	"db_check_size_lm":                  {min: 0, max: 1_000_000_000, integer: true},
	"default_delay_camera":              {min: 0, max: 86_400, integer: true},
	"default_delay_go2rtc":              {min: 0, max: 86_400, integer: true},
	"default_tiso_type":                 {min: 0, max: 10, integer: true},
	"fps_default":                       {min: 1, max: 120, integer: true},
	"livestream_default_bitrate":        {min: 64, max: 100_000, integer: true},
	"max_free_ram_force_reboot":         {min: 0, max: 1_099_511_627_776, integer: true},
	"max_free_ram_force_restart_camera": {min: 0, max: 1_099_511_627_776, integer: true},
	"max_free_ram_restart_camera":       {min: 0, max: 1_099_511_627_776, integer: true},
	"max_shared_ram_camera":             {min: 0, max: 1_099_511_627_776, integer: true},
}

func numberValue(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	default:
		return 0, false
	}
}

func validateImageValue(text string) error {
	if text == "" {
		return nil
	}
	if strings.HasPrefix(text, "data:") {
		allowed := map[string]string{
			"data:image/png;base64,":  "image/png",
			"data:image/jpeg;base64,": "image/jpeg",
			"data:image/webp;base64,": "image/webp",
		}
		var encoded, expectedType string
		for prefix, mimeType := range allowed {
			if strings.HasPrefix(text, prefix) {
				encoded = strings.TrimPrefix(text, prefix)
				expectedType = mimeType
				break
			}
		}
		if expectedType == "" {
			return fmt.Errorf("image must be PNG, JPEG or WebP")
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("invalid image data")
		}
		if len(decoded) > 512*1024 {
			return fmt.Errorf("image data is too large")
		}
		if detected := http.DetectContentType(decoded); detected != expectedType {
			return fmt.Errorf("image content does not match its media type")
		}
		return nil
	}
	if strings.HasPrefix(text, "/") && !strings.HasPrefix(text, "//") {
		return nil
	}
	parsed, err := url.ParseRequestURI(text)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("image must be an HTTP(S) URL, absolute path or supported data URL")
	}
	return nil
}

func valuesEqual(a, b any) bool {
	left, errLeft := json.Marshal(a)
	right, errRight := json.Marshal(b)
	return errLeft == nil && errRight == nil && bytes.Equal(left, right)
}
