package web_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/web"
)

func TestEmbeddedStaticAssets(t *testing.T) {
	staticFS, err := web.Static()
	if err != nil {
		t.Fatalf("web.Static() failed: %v", err)
	}

	// 1. Verify index.html embedding
	indexFile, err := staticFS.Open("index.html")
	if err != nil {
		t.Fatalf("open embedded index.html: %v", err)
	}
	defer indexFile.Close()

	indexData, err := io.ReadAll(indexFile)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}

	indexSubstrings := []string{
		`id="view-redbida"`,
		`id="redbida-preset-panel"`,
		`id="redbida-knowledge-hub"`,
		`class="redbida-status-grid"`,
		`id="redbida-node-status"`,
		`id="redbida-key-count"`,
		`id="redbida-time-status"`,
		`id="redbida-ntp-status"`,
		`id="redbida-broker-status"`,
		`id="redbida-draft-count"`,
		`id="redbida-preset-swatches"`,
		`id="redbida-preset-bg-preview"`,
		`class="redbida-pillar-card pillar-branding"`,
		`class="redbida-pillar-card pillar-streaming"`,
		`class="redbida-pillar-card pillar-shinobi"`,
		`class="redbida-pillar-card pillar-system"`,
		`data-testid="redbida-refresh"`,
		`data-testid="redbida-apply"`,
		`data-testid="redbida-search"`,
		`data-testid="redbida-group"`,
		`id="redbida-dirty-only"`,
		`id="redbida-table"`,
		`id="redbida-tbody"`,
	}

	for _, sub := range indexSubstrings {
		if !bytes.Contains(indexData, []byte(sub)) {
			t.Errorf("embedded index.html missing substring: %s", sub)
		}
	}

	// 2. Verify style.css embedding
	cssFile, err := staticFS.Open("style.css")
	if err != nil {
		t.Fatalf("open embedded style.css: %v", err)
	}
	defer cssFile.Close()

	cssData, err := io.ReadAll(cssFile)
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	cssSubstrings := []string{
		"--glass-bg:",
		"--glass-bg-card:",
		"--glass-border:",
		"--glass-blur: blur(16px) saturate(180%);",
		"--glass-blur-sm: blur(8px) saturate(160%);",
		"--glass-shadow:",
		"--glass-glow-accent:",
		".redbida-status-grid",
		".redbida-metric-card",
		"#redbida-preset-panel",
		".redbida-swatch",
		".redbida-gradient-preview",
		"#redbida-knowledge-hub",
		".redbida-pillar-card",
		".pillar-branding",
		".pillar-streaming",
		".pillar-shinobi",
		".pillar-system",
		".redbida-toolbar",
		".redbida-table",
		".redbida-dirty",
	}

	for _, sub := range cssSubstrings {
		if !bytes.Contains(cssData, []byte(sub)) {
			t.Errorf("embedded style.css missing substring: %s", sub)
		}
	}

	// 3. Verify redbida.js embedding
	jsFile, err := staticFS.Open("redbida.js")
	if err != nil {
		t.Fatalf("open embedded redbida.js: %v", err)
	}
	defer jsFile.Close()

	jsData, err := io.ReadAll(jsFile)
	if err != nil {
		t.Fatalf("read redbida.js: %v", err)
	}

	jsSubstrings := []string{
		"redbidaState",
		"redbidaGeneratePreset",
		"redbidaResetPresetForm",
		"redbidaTriggerGo2RTCStream",
		"redbidaRenderPresetDiff",
		"redbidaInitSwatches",
		"redbidaInitPillarButtons",
		"redbidaInitToggles",
		"redbidaLoadCatalog",
		"redbidaRefresh",
		"redbidaApply",
		"redbidaTimeStatus",
		"redbidaOnShow",
		"removeVietnameseTones",
		"ui_tabs_links",
		"custom_hashtags",
		"redbida-preset-bg-preview",
		"redbida-preset-swatches",
	}

	for _, sub := range jsSubstrings {
		if !bytes.Contains(jsData, []byte(sub)) {
			t.Errorf("embedded redbida.js missing substring: %s", sub)
		}
	}
}

func TestAllStaticAssetsLoadable(t *testing.T) {
	staticFS, err := web.Static()
	if err != nil {
		t.Fatalf("web.Static() failed: %v", err)
	}

	requiredFiles := []string{
		"index.html",
		"login.html",
		"style.css",
		"redbida.js",
		"app.js",
		"ui-core.js",
		"review.js",
		"help.js",
		"qrcode.min.js",
		"vis-timeline-graph2d.min.js",
		"vis-timeline-graph2d.min.css",
	}

	for _, file := range requiredFiles {
		f, err := staticFS.Open(file)
		if err != nil {
			t.Errorf("expected embedded file %s not found: %v", file, err)
			continue
		}
		buf := make([]byte, 128)
		n, err := f.Read(buf)
		f.Close()
		if err != nil && err != io.EOF {
			t.Errorf("error reading embedded file %s: %v", file, err)
		}
		if n == 0 {
			t.Errorf("embedded file %s is empty", file)
		}
	}
}

