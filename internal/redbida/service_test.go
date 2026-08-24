package redbida

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeBroker struct {
	readValues map[string]any
	writeAck   map[string]WriteAck
	readKeys   []string
	writes     map[string]any
	writeErr   error
}

func (f *fakeBroker) Read(_ context.Context, keys []string) (map[string]any, error) {
	f.readKeys = append([]string(nil), keys...)
	return f.readValues, nil
}

func (f *fakeBroker) Write(_ context.Context, changes map[string]any) (map[string]WriteAck, error) {
	f.writes = changes
	return f.writeAck, f.writeErr
}

func TestCatalogDiscoversKeysAndClassifiesRisk(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "logo_header", "https://example/logo.png")
	writeTestFile(t, dir, "mqtt_password", "hidden")
	writeTestFile(t, dir, "new_operator_key", "x")
	catalog := NewCatalog(dir)
	metas := catalog.List()
	if len(metas) < 20 {
		t.Fatalf("got only %d catalog keys", len(metas))
	}
	logo, ok := catalog.Meta("logo_header")
	if !ok || logo.Group != "Branding / Logo" || !logo.Editable || logo.ValueType != TypeImage {
		t.Fatalf("unexpected logo metadata: %+v", logo)
	}
	secret, ok := catalog.Meta("mqtt_password")
	if !ok || secret.Editable || !secret.Secret || secret.Risk != RiskProtected {
		t.Fatalf("unexpected secret metadata: %+v", secret)
	}
	unknown, ok := catalog.Meta("new_operator_key")
	if !ok || unknown.Editable || unknown.Risk != RiskUnknown {
		t.Fatalf("unexpected unknown metadata: %+v", unknown)
	}
}

func TestRefreshRedactsSecretsAndPreservesTypes(t *testing.T) {
	catalog := testCatalog(t, "logo_header", "mqtt_password", "show_toolbar")
	broker := &fakeBroker{readValues: map[string]any{
		"logo_header":   "data:image/png;base64,AAAA",
		"mqtt_password": "top-secret",
		"show_toolbar":  true,
	}}
	service := NewService(broker, catalog, 20)
	values, err := service.Refresh(context.Background(), []string{"logo_header", "mqtt_password", "show_toolbar"})
	if err != nil {
		t.Fatal(err)
	}
	if len(broker.readKeys) != 3 || values[1].Value != "********" {
		t.Fatalf("unexpected refresh: %+v", values)
	}
	if values[0].Meta.ValueType != TypeImage || values[2].Meta.ValueType != TypeBoolean {
		t.Fatalf("unexpected inferred types: %+v", values)
	}
}

func TestApplyRejectsProtectedUnknownAndUnconfirmedKeys(t *testing.T) {
	catalog := testCatalog(t, "logo_header")
	broker := &fakeBroker{writeAck: map[string]WriteAck{
		"logo_header": {OldValue: "https://example.test/old.png", NewValue: "https://example.test/new.png"},
	}, readValues: map[string]any{"logo_header": "https://example.test/new.png"}}
	service := NewService(broker, catalog, 20)
	results, err := service.Apply(context.Background(), map[string]any{
		"logo_header":   "https://example.test/new.png",
		"mqtt_password": "new-secret",
		"unknown_key":   "new",
		"button_reboot": true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(broker.writes) != 1 {
		t.Fatalf("writes = %+v, want only editable logo", broker.writes)
	}
	byKey := map[string]ChangeResult{}
	for _, result := range results {
		byKey[result.Key] = result
	}
	if !byKey["logo_header"].Applied || byKey["mqtt_password"].Error == "" || byKey["unknown_key"].Error == "" || byKey["button_reboot"].Error == "" {
		t.Fatalf("unexpected results: %+v", byKey)
	}
}

func TestApplyAllowsConfirmedChangeAndRejectsOversizedImage(t *testing.T) {
	catalog := testCatalog(t, "button_reboot")
	broker := &fakeBroker{
		writeAck:   map[string]WriteAck{"button_reboot": {OldValue: false, NewValue: true}},
		readValues: map[string]any{"button_reboot": true},
	}
	service := NewService(broker, catalog, 20)
	results, err := service.Apply(context.Background(), map[string]any{"button_reboot": true}, true)
	if err != nil || len(results) != 1 || !results[0].Applied {
		t.Fatalf("confirmed apply failed: %+v %v", results, err)
	}
	large := "data:image/png;base64," + string(make([]byte, 700*1024))
	results, err = service.Apply(context.Background(), map[string]any{"logo_header": large}, false)
	if err != nil || len(results) != 1 || results[0].Error == "" {
		t.Fatalf("oversized image should be rejected: %+v %v", results, err)
	}
}

func TestApplyFailsClosedWhenReadBackDoesNotMatch(t *testing.T) {
	broker := &fakeBroker{
		writeAck:   map[string]WriteAck{"logo_header": {OldValue: "https://example.test/old.png", NewValue: "https://example.test/new.png"}},
		readValues: map[string]any{"logo_header": "https://example.test/old.png"},
	}
	service := NewService(broker, testCatalog(t, "logo_header"), 20)
	results, err := service.Apply(context.Background(), map[string]any{"logo_header": "https://example.test/new.png"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Applied || !results[0].Acknowledged || results[0].Verified || !results[0].ReadBack || results[0].Error == "" {
		t.Fatalf("read-back mismatch was accepted: %+v", results)
	}
}

func TestNumericValidationUsesPerKeyRanges(t *testing.T) {
	tests := []struct {
		key   string
		value any
	}{
		{key: "fps_default", value: float64(0)},
		{key: "livestream_default_bitrate", value: float64(1_000_000)},
		{key: "camera_count", value: 1.5},
		{key: "default_delay_camera", value: float64(-1)},
		{key: "max_free_ram_force_reboot", value: float64(-1)},
	}
	for _, tt := range tests {
		if err := validateValue(metaForKey(tt.key, "", ""), tt.value); err == nil {
			t.Errorf("%s accepted out-of-range value %#v", tt.key, tt.value)
		}
	}
	if err := validateValue(metaForKey("fps_default", "", ""), float64(25)); err != nil {
		t.Fatalf("valid FPS rejected: %v", err)
	}
}

func TestNumberValueAcceptsGoNumericTypes(t *testing.T) {
	values := []any{
		float64(1), float32(1), int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
	}
	for _, value := range values {
		if number, ok := numberValue(value); !ok || number != 1 {
			t.Errorf("numberValue(%T) = %v, %v", value, number, ok)
		}
	}
	if _, ok := numberValue("1"); ok {
		t.Fatal("numeric string was accepted")
	}
}

func TestMaintenanceMemoryThresholdRequiresConfirmation(t *testing.T) {
	meta := metaForKey("max_free_ram_force_reboot", "", "")
	if meta.Risk != RiskConfirm || !meta.Editable {
		t.Fatalf("maintenance threshold is not confirmation-gated: %+v", meta)
	}
}

func TestCatalogFallsBackToKnownKeysWhenDirectoryUnavailable(t *testing.T) {
	catalog := NewCatalog(filepath.Join(t.TempDir(), "missing"))
	metas := catalog.List()
	if len(metas) < 20 {
		t.Fatalf("fallback catalog has only %d keys", len(metas))
	}
	logo, ok := catalog.Meta("logo_livestream")
	if !ok || !logo.Editable || logo.ValueType != TypeImage {
		t.Fatalf("fallback logo metadata = %+v, ok=%v", logo, ok)
	}
}

func TestImageValidationRejectsUnsupportedDataURL(t *testing.T) {
	meta := metaForKey("logo_header", "", "")
	if err := validateValue(meta, "data:image/svg+xml;base64,PHN2Zz48L3N2Zz4="); err == nil {
		t.Fatal("SVG data URL should be rejected")
	}
}

func TestCatalogDoesNotGrantWriteAccessFromANameHeuristic(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "button_factory_reset", "false")
	writeTestFile(t, dir, "ui_admin_action", "false")
	catalog := NewCatalog(dir)
	for _, key := range []string{"button_factory_reset", "ui_admin_action"} {
		meta, ok := catalog.Meta(key)
		if !ok || meta.Editable || meta.Risk != RiskUnknown {
			t.Fatalf("dangerous future key %s was not fail-closed: %+v", key, meta)
		}
	}
}

func TestApplyFailsClosedWhenCatalogSourceUnavailable(t *testing.T) {
	broker := &fakeBroker{}
	service := NewService(broker, NewCatalog(filepath.Join(t.TempDir(), "missing")), 20)
	if _, err := service.Apply(context.Background(), map[string]any{"logo_header": "https://example.test/logo.png"}, false); err == nil {
		t.Fatal("apply should fail when the catalog source is unavailable")
	}
	if broker.writes != nil {
		t.Fatalf("broker received a write during catalog outage: %+v", broker.writes)
	}
}

func TestStringValidationRejectsStructuredValues(t *testing.T) {
	meta := metaForKey("ui_title", "", "")
	if err := validateValue(meta, map[string]any{"bad": true}); err == nil {
		t.Fatal("structured value was accepted for a string key")
	}
}

func TestApplyRecoversFromAckTimeoutUsingReadBack(t *testing.T) {
	broker := &fakeBroker{
		writeErr:   &AckTimeoutError{Topic: "/private/i_sets/ack"},
		readValues: map[string]any{"logo_header": "https://example.test/new.png"},
	}
	service := NewService(broker, testCatalog(t, "logo_header"), 20)
	results, err := service.Apply(context.Background(), map[string]any{"logo_header": "https://example.test/new.png"}, false)
	if err != nil || len(results) != 1 || !results[0].Applied || !results[0].Verified || !results[0].ReadBack || results[0].Acknowledged {
		t.Fatalf("ack timeout was not recovered by read-back: results=%+v err=%v", results, err)
	}
}

func writeTestFile(t *testing.T, dir, name, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func testCatalog(t *testing.T, keys ...string) *Catalog {
	t.Helper()
	dir := t.TempDir()
	for _, key := range keys {
		writeTestFile(t, dir, key, "test")
	}
	return NewCatalog(dir)
}
