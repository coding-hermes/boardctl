# Verdict: DF-BOARDCTL-26

**Task:** README annotate post-v0.1.10 features since-v0.1.11
**Evaluated:** 2026-10-09T23:20:19.432429
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README sections for --deferred and BT-077 association flags carry since-v0.1.11/unreleased annotations; Current release pin unchanged at v0.1.10; version-check passes: README.md:721-728 carries '> **Since v0.1.11 (unreleased): plural association flags.** ... (BT-077) ... These are NOT in the published v0.1.10 binary'; README.md:760-765 '### The deferred flag (BT-076)' with '> **Since v0.1.11 (unreleased).** The flag documented below is NOT in the published v0.1.10 binary'. Release pin unchanged: README.md:17 'Current release: **v0.1.10**.' and download URLs at README.md:20-21 still /releases/download/v0.1.10/. Version-check passes: `make version-check` -> 'go test -count=1 -run TestVersioncheck ./internal/versioncheck' / 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.003s' (exit 0); fresh `go test -count=1 ./internal/versioncheck` -> 'ok ... 0.005s' (exit 0).
README annotates the --deferred (BT-076) and BT-077 association-flag sections as since-v0.1.11/unreleased, the Current release pin remains v0.1.10, and make version-check passes.

## Summary

Judge Result: DF-BOARDCTL-26

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README sections for --deferred and BT-077 association flags carry since-v0.1.11/unreleased annotations; Current release pin unchanged at v0.1.10; version-check passes: README.md:721-728 carries '> **Since v0.1.11 (unreleased): plural association flags.** ... (BT-077) ... These are NOT in the published v0.1.10 binary'; README.md:760-765 '### The deferred flag (BT-076)' with '> **Since v0.1.11 (unreleased).** The flag documented below is NOT in the published v0.1.10 binary'. Release pin unchanged: README.md:17 'Current release: **v0.1.10**.' and download URLs at README.md:20-21 still /releases/download/v0.1.10/. Version-check passes: `make version-check` -> 'go test -count=1 -run TestVersioncheck ./internal/versioncheck' / 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.003s' (exit 0); fresh `go test -count=1 ./internal/versioncheck` -> 'ok ... 0.005s' (exit 0).
README annotates the --deferred (BT-076) and BT-077 association-flag sections as since-v0.1.11/unreleased, the Current release pin remains v0.1.10, and make version-check passes.

Overall: PASS ✓
