## 2026-08-24T15:37:58Z
You are the Remediation Worker for Milestone 2 (M2: Full Overhaul of `/#redbida`) in `ksp-camera-auto`.
Your working directory is: /home/ksp/ksp-camera-auto/.agents/worker_redbida_m2_fix

Read the following files before starting:
- `/home/ksp/ksp-camera-auto/.agents/ORIGINAL_REQUEST.md`
- `/home/ksp/ksp-camera-auto/PROJECT.md`
- `/home/ksp/ksp-camera-auto/.agents/challenger_m2_1/handoff.md` (Challenger 1 findings & test suite `tests/ui/redbida_m2_adversarial.spec.js`)
- `/home/ksp/ksp-camera-auto/.agents/challenger_m2_2/handoff.md` (Challenger 2 findings)
- `/home/ksp/ksp-camera-auto/.agents/worker_redbida_m2/handoff.md`

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope and Tasks:
You own `web/static/redbida.js`, `web/static/index.html`, `web/static/style.css`.
Fix the following 3 defects:
1. **`ui_bg` trailing semicolon stripping & fallback**:
   - In `web/static/redbida.js` (lines 236, 694, 802, 947, and wherever `ui_bg` is cleaned), replace `.replace(/;\s*$/, '')` with `.replace(/[;\s]+$/, '')` to strip any and all trailing semicolons and trailing whitespace.
   - In `ui_bg.fix` inside `GOLDEN_STANDARD_RULES`, if the value is not a valid CSS gradient (e.g. does not contain `'gradient'`), return the default standard gradient `REDBIDA_GRADIENT_PALETTE[0].css` (`linear-gradient(135deg, #0b192c 0%, #1e3e62 50%, #000000 100%)`).
2. **`custom_hashtags` Unicode Diacritics Check**:
   - In `custom_hashtags.check` inside `GOLDEN_STANDARD_RULES`, ensure the regex matches both lowercase and uppercase Vietnamese vowels or uses the `/i` flag (e.g. `!/[àáạảãâầấậẩẫăằắặẳẵèéẹẻẽêềếệểễìíịỉĩòóọỏõôồốộổỗơờớợởỡùúụủũưừứựửữỳýỵỷỹđ]/i.test(val)`).
3. **`company_name` Golden Check**:
   - In `company_name.check` inside `GOLDEN_STANDARD_RULES`, ensure that if `title` (`ui_title`) is set, `val` matches `title`. If `title` is not set, ensure `val` is a non-empty string.
4. **Verification**:
   - Run Go unit tests: `go test ./...`
   - Run Challenger test suites: `npx playwright test tests/ui/redbida_m2_adversarial.spec.js tests/ui/redbida_m2_challenger_deep.spec.js tests/ui/redbida_m2_overhaul.spec.js`
   - Run full Playwright test suite: `npx playwright test`
   - Ensure 100% tests pass.
   - Write your handoff report to `/home/ksp/ksp-camera-auto/.agents/worker_redbida_m2_fix/handoff.md`.
   - Send message to parent when completed.
