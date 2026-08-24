## 2026-08-24T15:18:36Z
You are the Implementation Worker for Milestone 2 (M2: Full Overhaul of `/#redbida`) in `ksp-camera-auto`.
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_redbida_m2

Read the following files before starting:
- `/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md` (Specifically R2: `/#redbida` Overhaul)
- `/home/ksp/ksp-camera-auto/PROJECT.md`
- `/home/ksp/ksp-camera-auto/.agents/explorer_survey_redbida/analysis.md` (Comprehensive technical specification & architecture)
- `/home/ksp/ksp-camera-auto/.agents/explorer_survey_redbida/handoff.md`

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope and Write Ownership:
You own:
- `web/static/index.html` (specifically `#view-redbida` section)
- `web/static/redbida.js`
- `web/static/style.css` (redbida glassmorphism components)

Your tasks for Milestone 2:
1. **Golden Standard Inspector & 1-Click Auto-Fix**:
   - Implement the 15-key Golden Standard audit engine in `redbida.js` comparing live `redbidaState.values` with standard rules.
   - Calculate `% Chuẩn Bida` ($	ext{Score} = (	ext{Passed} / 15) 	imes 100\%$) and render a visual progress bar with status badge (Xanh 100% Đạt Chuẩn, Vàng 70-99% Cần Hiệu Chỉnh, Đỏ <70% Lệch Chuẩn).
   - Provide "⚡ Sửa nhanh" (1-Click Auto-Fix) button on each out-of-spec key row, plus a top-level "⚡ 1-Click Sửa Tất Cả" (Auto-Fix All) button that applies standard values for all failing keys and refreshes the diff preview.
2. **Curated 8 CSS Gradient Palette with Live Canvas Preview**:
   - Implement 8 curated gradient presets:
     1. Royal Deep Blue Glow: `linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)`
     2. Midnight Emerald Cyber: `linear-gradient(135deg, #051f20 0%, #0b2b26 40%, #163832 70%, #001414 100%)`
     3. Cyberpunk Neon: `linear-gradient(135deg, #1f1035 0%, #341247 45%, #0d0221 100%)`
     4. Golden Velvet: `linear-gradient(135deg, #2b1e05 0%, #4a3508 50%, #171003 100%)`
     5. Obsidian Carbon: `linear-gradient(135deg, #121212 0%, #242424 50%, #0a0a0a 100%)`
     6. Crimson Elegance: `linear-gradient(135deg, #2c0b0e 0%, #52151c 50%, #140507 100%)`
     7. Sapphire Blue: `linear-gradient(135deg, #0a1128 0%, #1c2541 50%, #000814 100%)`
     8. Ruby Luxury: `linear-gradient(135deg, #3d0c11 0%, #68131d 50%, #200407 100%)`
   - Render interactive swatches with active selection indicator.
   - Build a Live Canvas Preview widget showing dynamic background gradient, logo header, venue title, subtitle, hashtags, and 20-tab player simulation.
   - Ensure `ui_bg` strings never contain trailing semicolons (`;`).
3. **Visual 20-Tab INI Editor `[C01]`..`[C20]`**:
   - Render a 20-tab matrix grid (`C01` to `C20`) allowing users to click each table button, edit table-specific properties (`title`, `url`, `vid_play_label`), quick-copy stream URLs, and 1-click sync venue name to all 20 tables.
   - Provide seamless toggle between Visual Matrix Grid and Raw INI Textarea with bidirectional synchronization.
4. **Smart Hashtag Generator**:
   - Implement dynamic Unicode normalization (NFC/NFD) in `redbida.js` that strips Vietnamese diacritics in realtime on typing the venue name (`#red-preset-title`), updating the hashtag field with `#<CleanTitle> #BILLIARDSlive #INUTlive #highlightsports`.
5. **Enhanced Key Management**:
   - Group filtering pills, fast search input, Risk Badges, inline logo checkerboard preview, and inline gradient preview bar.
6. **Strict Backward Compatibility**:
   - Preserve all existing test selectors (`[data-testid="redbida-refresh"]`, `[data-testid="redbida-apply"]`, `[data-red-row="..."]`, `[data-red-key="..."]`, `#redbida-preset-panel`, `#redbida-preset-diff`, `#redbida-preset-gen-btn`, `#redbida-preset-reset-btn`, `#redbida-knowledge-hub`).
7. **Verification**:
   - Run Go unit tests: `go test ./...`
   - Run Playwright test suites: `npx playwright test tests/ui/redbida.spec.js` and all existing tests.
   - Write your handoff report to `/home/ksp/ksp-camera-auto/.agents/worker_redbida_m2/handoff.md`.
   - Send message to parent when completed.
