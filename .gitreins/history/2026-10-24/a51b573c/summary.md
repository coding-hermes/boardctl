# Verdict: README-011

**Task:** README docs drift check
**Evaluated:** 2026-10-24T17:44:44.020476
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README command/reference drift procedure is documented with explicit pass/fail semantics and four required make gates remain green.: README.md:1459-1515 contains section '### README command/reference drift check (manual-only)' with a copy-paste inventory script (awk shell-block extraction, inline-reference grep, `go run ./cmd/boardctl --help`, `make -qp` target list, workflow command grep, `git ls-files`), per-item classification rules, and explicit semantics: 'Result: **PASS** only when every listed command/reference is classified as executable and verified, intentionally manual/network-dependent with a stated reason, or historical/example-only. Result: **FAIL** for an undocumented or missing command, stale flag/target/path, or unexplained exception.' README.md:1496-1501 lists the four gates (make fmt-check; make version-check VERSION_TAG=v0.1.11; make spec-check; make vuln-check). All four verified green with fresh runs: `make fmt-check` -> 'ok github.com/coding-hermes/boardctl/internal/fmtcheck 0.155s' exit 0; `make version-check VERSION_TAG=v0.1.11` -> 'ok .../internal/versioncheck 0.006s' exit 0 (README.md:17 and :20-21 pin v0.1.11); `make spec-check` -> 'ok .../internal/speccheck 0.049s' exit 0; `make vuln-check` -> 'ok .../internal/vulncheck 3.623s' exit 0.
README documents the manual README command/reference drift procedure with explicit PASS/FAIL semantics, and all four required make gates (fmt-check, version-check, spec-check, vuln-check) pass with exit code 0.

## Summary

Judge Result: README-011

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README command/reference drift procedure is documented with explicit pass/fail semantics and four required make gates remain green.: README.md:1459-1515 contains section '### README command/reference drift check (manual-only)' with a copy-paste inventory script (awk shell-block extraction, inline-reference grep, `go run ./cmd/boardctl --help`, `make -qp` target list, workflow command grep, `git ls-files`), per-item classification rules, and explicit semantics: 'Result: **PASS** only when every listed command/reference is classified as executable and verified, intentionally manual/network-dependent with a stated reason, or historical/example-only. Result: **FAIL** for an undocumented or missing command, stale flag/target/path, or unexplained exception.' README.md:1496-1501 lists the four gates (make fmt-check; make version-check VERSION_TAG=v0.1.11; make spec-check; make vuln-check). All four verified green with fresh runs: `make fmt-check` -> 'ok github.com/coding-hermes/boardctl/internal/fmtcheck 0.155s' exit 0; `make version-check VERSION_TAG=v0.1.11` -> 'ok .../internal/versioncheck 0.006s' exit 0 (README.md:17 and :20-21 pin v0.1.11); `make spec-check` -> 'ok .../internal/speccheck 0.049s' exit 0; `make vuln-check` -> 'ok .../internal/vulncheck 3.623s' exit 0.
README documents the manual README command/reference drift procedure with explicit PASS/FAIL semantics, and all four required make gates (fmt-check, version-check, spec-check, vuln-check) pass with exit code 0.

Overall: FAIL ✗
