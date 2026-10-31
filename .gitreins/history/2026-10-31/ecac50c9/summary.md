# Verdict: BT-83

**Task:** gitleaks sk-api-key mid-word FP
**Evaluated:** 2026-10-31T04:30:05.713246
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ 1) .gitleaks.toml sk-api-key regex uses RE2-safe non-alnum boundary prefix (no lookbehind) and gitleaks detect --source . --no-git exits 0 with zero findings on the current tree. 2) A regression test pins both sides: pre-fix regex matches task-complete-unavailable, fixed regex does not, and a 20+ char boundary-delimited sk- token is still caught; go test ./... exits 0. 3) gitreins guard prints [PASS] secrets on the fixed tree.: (1) .gitleaks.toml:14 regex = '''(?i)(?:^|[^a-zA-Z0-9])sk-[a-zA-Z0-9_-]{20,}''' — RE2-safe non-alnum boundary prefix, no lookbehind. `gitleaks detect --source . --no-git --config .gitleaks.toml --exit-code 1` => EXIT=0 with "INF no leaks found" (zero findings). (2) internal/secretscheck/secretscheck_test.go: TestSKAPIKeyBoundaryRejectsMidWordProse (line 68 fp = "verdict=none:task-complete-unavailable" must NOT match, plus desk-/flask-/task- variants), TestSKAPIKeyStillCatchesRealKeys (29-char sk- token matched at start/space/=/quote/colon/newline boundaries and SK- uppercase), TestSKAPIKeyRejectsTooShort, TestSKAPIKeyRegexHasBoundaryAnchor. `go test -count=1 ./...` => EXIT=0, all 13 packages ok incl. internal/secretscheck; verbose run: all four SKAPIKey tests PASS. (3) `gitreins guard` => EXIT=0, "✓ secrets — clean (gitleaks + builtin cross-check)"; --json shows {"id":"secrets","outcome":"pass","passed":true,"summary":"gitleaks: clean"} and overall outcome pass (human output renders the pass glyph ✓ rather than the literal string "[PASS]", but the secrets lane is pass).
gitleaks sk-api-key regex is RE2-safe boundary-anchored, gitleaks detect reports zero leaks with exit 0, regression tests pin both FP and true-positive sides with go test ./... green, and gitreins guard's secrets lane passes.

## Summary

Judge Result: BT-83

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ 1) .gitleaks.toml sk-api-key regex uses RE2-safe non-alnum boundary prefix (no lookbehind) and gitleaks detect --source . --no-git exits 0 with zero findings on the current tree. 2) A regression test pins both sides: pre-fix regex matches task-complete-unavailable, fixed regex does not, and a 20+ char boundary-delimited sk- token is still caught; go test ./... exits 0. 3) gitreins guard prints [PASS] secrets on the fixed tree.: (1) .gitleaks.toml:14 regex = '''(?i)(?:^|[^a-zA-Z0-9])sk-[a-zA-Z0-9_-]{20,}''' — RE2-safe non-alnum boundary prefix, no lookbehind. `gitleaks detect --source . --no-git --config .gitleaks.toml --exit-code 1` => EXIT=0 with "INF no leaks found" (zero findings). (2) internal/secretscheck/secretscheck_test.go: TestSKAPIKeyBoundaryRejectsMidWordProse (line 68 fp = "verdict=none:task-complete-unavailable" must NOT match, plus desk-/flask-/task- variants), TestSKAPIKeyStillCatchesRealKeys (29-char sk- token matched at start/space/=/quote/colon/newline boundaries and SK- uppercase), TestSKAPIKeyRejectsTooShort, TestSKAPIKeyRegexHasBoundaryAnchor. `go test -count=1 ./...` => EXIT=0, all 13 packages ok incl. internal/secretscheck; verbose run: all four SKAPIKey tests PASS. (3) `gitreins guard` => EXIT=0, "✓ secrets — clean (gitleaks + builtin cross-check)"; --json shows {"id":"secrets","outcome":"pass","passed":true,"summary":"gitleaks: clean"} and overall outcome pass (human output renders the pass glyph ✓ rather than the literal string "[PASS]", but the secrets lane is pass).
gitleaks sk-api-key regex is RE2-safe boundary-anchored, gitleaks detect reports zero leaks with exit 0, regression tests pin both FP and true-positive sides with go test ./... green, and gitreins guard's secrets lane passes.

Overall: PASS ✓
