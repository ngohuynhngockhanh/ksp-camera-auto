package redbida

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdversarial_MultilineINIAndComplexPayloads stress tests ui_tabs_links and custom_hashtags
// with 20 standard sections, CRLF, Unix LF, Unicode, emojis, 2MB boundaries, etc.
func TestAdversarial_MultilineINIAndComplexPayloads(t *testing.T) {
	tabsMeta := metaForKey("ui_tabs_links", "", "")
	hashMeta := metaForKey("custom_hashtags", "", "")

	// 1. Standard 20-section INI matching camera-naming SKILL.md with Vietnamese diacritics
	var standard20SectionINI strings.Builder
	for i := 1; i <= 20; i++ {
		standard20SectionINI.WriteString(fmt.Sprintf("[C%02d]\n", i))
		standard20SectionINI.WriteString("stream_label=Video Trực tiếp\n")
		standard20SectionINI.WriteString("vid_list_label=Danh sách highlight\n")
		standard20SectionINI.WriteString("vid_play_label=Bida Hoàng Gia - Chi Nhánh 1 (Quận 1, TP.HCM 🎱)\n")
		standard20SectionINI.WriteString("list_refresh_label=Cập nhật highlight\n\n")
	}
	if err := validateValue(tabsMeta, standard20SectionINI.String()); err != nil {
		t.Fatalf("standard 20-section INI was rejected: %v", err)
	}

	// 2. Windows CRLF line endings in INI
	crlfINI := strings.ReplaceAll(standard20SectionINI.String(), "\n", "\r\n")
	if err := validateValue(tabsMeta, crlfINI); err != nil {
		t.Fatalf("CRLF 20-section INI was rejected: %v", err)
	}

	// 3. Mixed line endings and comments
	mixedINI := "[C01]\r\n# Comment line\r\nstream_label=Live\n; Another comment\r\nvid_play_label=Test\n"
	if err := validateValue(tabsMeta, mixedINI); err != nil {
		t.Fatalf("mixed line endings INI was rejected: %v", err)
	}

	// 4. Boundary payload sizes for strings (2MB limit in validateValue)
	// Exact 2MB string should pass
	twoMB := strings.Repeat("A", 2*1024*1024)
	if err := validateValue(tabsMeta, twoMB); err != nil {
		t.Fatalf("2MB string was rejected: %v", err)
	}
	// 2MB + 1 byte must be rejected
	twoMBPlusOne := strings.Repeat("A", 2*1024*1024+1)
	if err := validateValue(tabsMeta, twoMBPlusOne); err == nil {
		t.Fatalf("2MB + 1 byte string should have failed but passed")
	}

	// 5. Vietnamese hashtags with composite, precomposed UTF-8, emojis, special characters
	vietnameseHashtags := []string{
		"#CXKingLuxury #BILLIARDSlive #INUTlive #highlightsports",
		"#BidaHoàngGia #BilliardViệtNam #BidaQuận1 #BidaSàiGòn🎱🏆",
		"#CLBBidaĐỉnhCao #TrựcTiếpBàn01 #ĐườngCơThầnSầu #CơThủViệt",
		"#Bida_3_Băng #Bida_Lỗ_9_Ball #TảiClipTrậnĐấu",
		"", // empty string is valid string
		strings.Repeat("#HashtagDàiDàiDài ", 1000),
	}
	for idx, tag := range vietnameseHashtags {
		if err := validateValue(hashMeta, tag); err != nil {
			t.Errorf("vietnamese hashtag[%d] rejected: %v", idx, err)
		}
	}
}

// TestAdversarial_NumericBoundaries tests boundary numbers, scientific notation,
// floats, negative values, and non-numeric types for all numeric keys.
func TestAdversarial_NumericBoundaries(t *testing.T) {
	// 1. toolbar_show_count: [0, 4096] integer
	tbMeta := metaForKey("toolbar_show_count", "", "")

	validTB := []any{
		0, 1, 8, 16, 20, 100, 4096,
		float64(0), float64(4096), float64(8),
		int32(8), int64(4096), uint(20), uint64(4096),
	}
	for _, val := range validTB {
		if err := validateValue(tbMeta, val); err != nil {
			t.Errorf("valid toolbar_show_count %v (%T) rejected: %v", val, val, err)
		}
	}

	invalidTB := []struct {
		val  any
		desc string
	}{
		{-1, "negative integer"},
		{-0.0001, "negative float"},
		{4097, "exceeds upper bound by 1"},
		{4096.0001, "float exceeding upper bound"},
		{100000, "large number"},
		{8.5, "non-integer positive float"},
		{0.1, "non-integer fraction"},
		{"8", "string representation of number"},
		{"", "empty string"},
		{true, "boolean true"},
		{false, "boolean false"},
		{nil, "nil value"},
		{[]int{8}, "slice of int"},
		{map[string]any{"count": 8}, "map object"},
	}
	for _, tt := range invalidTB {
		if err := validateValue(tbMeta, tt.val); err == nil {
			t.Errorf("invalid toolbar_show_count %v (%s) should have failed but passed", tt.val, tt.desc)
		}
	}

	// 2. All other numeric keys in catalog
	for key, rule := range numericRules {
		meta := metaForKey(key, "", "")
		if meta.ValueType != TypeNumber {
			t.Errorf("key %s in numericRules has meta.ValueType = %v, want TypeNumber", key, meta.ValueType)
		}
		// Test exact min
		if err := validateValue(meta, rule.min); err != nil {
			t.Errorf("key %s min value %g rejected: %v", key, rule.min, err)
		}
		// Test exact max
		if err := validateValue(meta, rule.max); err != nil {
			t.Errorf("key %s max value %g rejected: %v", key, rule.max, err)
		}
		// Test below min
		belowMin := rule.min - 1
		if rule.min == 0 {
			belowMin = -1
		}
		if err := validateValue(meta, belowMin); err == nil {
			t.Errorf("key %s below min %g should fail", key, belowMin)
		}
		// Test above max
		aboveMax := rule.max + 1
		if rule.max < 1e12 {
			if err := validateValue(meta, aboveMax); err == nil {
				t.Errorf("key %s above max %g should fail", key, aboveMax)
			}
		}
	}
}

// TestAdversarial_CatalogSortingDeterminism verifies that List() is 100% deterministic
// under repeated invocations, dynamic discovery, and key mutations.
func TestAdversarial_CatalogSortingDeterminism(t *testing.T) {
	dir := t.TempDir()
	// Create several custom files in dir
	customFiles := []string{
		"logo_header", "ui_title", "camera_count", "toolbar_show_count",
		"custom_hashtags", "ui_tabs_links", "z_custom_key", "a_custom_key",
	}
	for _, f := range customFiles {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("val"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	catalog := NewCatalog(dir)

	// Run List() 100 times and verify slice is always strictly ordered and identical
	var firstList []KeyMeta
	for iter := 0; iter < 100; iter++ {
		list := catalog.List()
		if firstList == nil {
			firstList = list
		} else {
			if len(list) != len(firstList) {
				t.Fatalf("iter %d: length mismatch: got %d, want %d", iter, len(list), len(firstList))
			}
			for i := range list {
				if list[i].Key != firstList[i].Key || list[i].Group != firstList[i].Group {
					t.Fatalf("iter %d: ordering mismatch at index %d: got %s (%s), want %s (%s)",
						iter, i, list[i].Key, list[i].Group, firstList[i].Key, firstList[i].Group)
				}
			}
		}

		// Verify strict group and key sorting order
		for i := 1; i < len(list); i++ {
			prev := list[i-1]
			curr := list[i]
			if prev.Group > curr.Group {
				t.Fatalf("iter %d: group sorting violated: %q after %q", iter, curr.Group, prev.Group)
			} else if prev.Group == curr.Group && prev.Key >= curr.Key {
				t.Fatalf("iter %d: key sorting within group %q violated: %q after %q", iter, curr.Group, curr.Key, prev.Key)
			}
		}
	}
}

// TestAdversarial_CatalogRWMutexConcurrencyStress tests heavy concurrent access
// against Catalog (List, Meta, Observe, Status, Present, Empty) with race detector.
func TestAdversarial_CatalogRWMutexConcurrencyStress(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		fName := fmt.Sprintf("dynamic_key_%02d", i)
		_ = os.WriteFile(filepath.Join(dir, fName), []byte("data"), 0o600)
	}

	catalog := NewCatalog(dir)

	var wg sync.WaitGroup
	workers := 20
	iterations := 30
	var totalOperations atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(workerID) + time.Now().UnixNano()))
			for i := 0; i < iterations; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				op := rng.Intn(6)
				switch op {
				case 0:
					// List
					list := catalog.List()
					if len(list) == 0 {
						t.Errorf("worker %d: List returned empty", workerID)
					}
				case 1:
					// Meta lookup
					key := fmt.Sprintf("dynamic_key_%02d", rng.Intn(20))
					_, _ = catalog.Meta(key)
				case 2:
					// Observe new key or update existing
					key := fmt.Sprintf("observed_key_%02d", rng.Intn(10))
					catalog.Observe(key, "some_value")
				case 3:
					// Status
					_, _ = catalog.Status()
				case 4:
					// Present
					key := fmt.Sprintf("dynamic_key_%02d", rng.Intn(20))
					_ = catalog.Present(key)
				case 5:
					// Empty
					key := fmt.Sprintf("dynamic_key_%02d", rng.Intn(20))
					_ = catalog.Empty(key)
				}
				totalOperations.Add(1)
			}
		}()
	}

	wg.Wait()
	if totalOperations.Load() < int64(workers*iterations/2) {
		t.Fatalf("insufficient operations completed: %d", totalOperations.Load())
	}
}

// TestAdversarial_ApplyBatchStressAndEdgeCases tests Service.Apply with complex, mixed,
// and edge-case batches.
func TestAdversarial_ApplyBatchStressAndEdgeCases(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{
		"toolbar_show_count", "custom_hashtags", "ui_tabs_links", "ui_title",
		"camera_count", "hls_using_go2rtc", "logo_header",
	} {
		_ = os.WriteFile(filepath.Join(dir, k), []byte("init"), 0o600)
	}

	mockValues := map[string]any{
		"toolbar_show_count": float64(12),
		"custom_hashtags":    "#BidaHoangGia #BILLIARDSlive #INUTlive #highlightsports",
		"ui_tabs_links":      "[C01]\nstream_label=Live\nvid_play_label=Hoàng Gia\n\n[C02]\nstream_label=Live\nvid_play_label=Hoàng Gia",
		"ui_title":           "Bida Hoàng Gia",
		"camera_count":       float64(12),
		"hls_using_go2rtc":   true,
		"logo_header":        "https://example.com/logo.png",
	}
	mockAcks := map[string]WriteAck{}
	for k, v := range mockValues {
		mockAcks[k] = WriteAck{OldValue: "old", NewValue: v}
	}

	service := NewService(&challenger2Broker{values: mockValues, acks: mockAcks}, NewCatalog(dir), 50)

	// 1. Full batch apply with 7 valid keys (mixture of number, string, multiline INI, bool, image)
	ctx := context.Background()
	results, err := service.Apply(ctx, map[string]any{
		"toolbar_show_count": 12,
		"custom_hashtags":    "#BidaHoangGia #BILLIARDSlive #INUTlive #highlightsports",
		"ui_tabs_links":      "[C01]\nstream_label=Live\nvid_play_label=Hoàng Gia\n\n[C02]\nstream_label=Live\nvid_play_label=Hoàng Gia",
		"ui_title":           "Bida Hoàng Gia",
		"camera_count":       12,
		"hls_using_go2rtc":   true,
		"logo_header":        "https://example.com/logo.png",
	}, false)
	if err != nil {
		t.Fatalf("batch apply failed: %v", err)
	}
	if len(results) != 7 {
		t.Fatalf("expected 7 results, got %d", len(results))
	}
	for _, res := range results {
		if !res.Applied || !res.Verified || res.Error != "" {
			t.Errorf("result for %s failed: applied=%v verified=%v error=%q", res.Key, res.Applied, res.Verified, res.Error)
		}
	}

	// 2. Partial failure batch: valid keys + read-only protected keys + out-of-bounds number
	partialBatch := map[string]any{
		"toolbar_show_count": 5000,              // Out of bounds (>4096)
		"shinobi_group_key":  "NEW_GROUP_KEY",   // Read-only protected key
		"ggcode":             "G-TEST1234",      // Read-only protected key
		"custom_hashtags":    "#ValidTag #Bida", // Valid
		"ui_title":           "Valid Title",     // Valid
	}
	resPartial, err := service.Apply(ctx, partialBatch, false)
	if err != nil {
		t.Fatalf("partial batch apply errored: %v", err)
	}
	resMap := make(map[string]ChangeResult)
	for _, r := range resPartial {
		resMap[r.Key] = r
	}

	if resMap["toolbar_show_count"].Applied || resMap["toolbar_show_count"].Error == "" {
		t.Errorf("toolbar_show_count should have failed out-of-bounds check")
	}
	if resMap["shinobi_group_key"].Applied || resMap["shinobi_group_key"].Error != "key is read-only" {
		t.Errorf("shinobi_group_key should have failed with read-only error, got %q", resMap["shinobi_group_key"].Error)
	}
	if resMap["ggcode"].Applied || resMap["ggcode"].Error != "key is read-only" {
		t.Errorf("ggcode should have failed with read-only error, got %q", resMap["ggcode"].Error)
	}
	if !resMap["custom_hashtags"].Applied || resMap["custom_hashtags"].Error != "" {
		t.Errorf("custom_hashtags in partial batch should have succeeded, got error %q", resMap["custom_hashtags"].Error)
	}
	if !resMap["ui_title"].Applied || resMap["ui_title"].Error != "" {
		t.Errorf("ui_title in partial batch should have succeeded, got error %q", resMap["ui_title"].Error)
	}
}

type challenger2Broker struct {
	values map[string]any
	acks   map[string]WriteAck
}

func (m *challenger2Broker) Read(ctx context.Context, keys []string) (map[string]any, error) {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (m *challenger2Broker) Write(ctx context.Context, changes map[string]any) (map[string]WriteAck, error) {
	out := map[string]WriteAck{}
	for k, v := range changes {
		if m.values != nil {
			m.values[k] = v
		}
		if ack, ok := m.acks[k]; ok {
			out[k] = ack
		} else {
			out[k] = WriteAck{OldValue: "old", NewValue: v}
		}
	}
	return out, nil
}
