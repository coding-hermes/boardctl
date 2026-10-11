# Verdict: DOC-25

**Task:** document validate --strict-keys in README
**Evaluated:** 2026-10-11T04:47:25.733999
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README reference section documents the --strict-keys flag added by BT-056, including its purpose (enforce key uniformity across task rows), default behavior, and usage examples: README.md:333-397 adds section '### validate --strict-keys: key-uniformity enforcement (BT-056)' in the reference/usage area (after the validate section, before the pre-commit hook section). Purpose: declares the task-row key canon (required keys id/title/status/priority, sanctioned extras, _note family) and reports off-canon keys as drift — matches internal/board/keys.go:43 (RequiredTaskRowKeys) and :48-69 (sanctionedExtras). Default behavior: 'by default (lenient mode) drift is only a [warn] and the command exits 0' — matches cmd/boardctl/main.go:1217-1219 where drift is promoted to error only when *strictKeys is set. Usage examples: `validate --strict-keys` (exit 1, RESULT: FAIL) and repair via `update <id> --normalize` (exit 0) — output formats match main.go:875 ('normalized task %s: %s %q -> %q') and keys.go:337-351 (Summary census line 'key uniformity: N/M rows carry only canon keys'). Alias examples why->reasoning and acceptance->acceptance_criteria confirmed at keys.go:176-177. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/ -run 'Keys|StrictKeys|Normalize|Validate|Usage'` all PASS (TestValidateReportsKeyDrift, TestNormalizeTaskRenamesKeyAlias, TestValidateCleanBoardReportsZeroDrift); full `go test -count=1 ./...` only fails TestVulncheck due to an environment govulncheck binary version mismatch (v1.8.0 vs pinned v1.7.0), unrelated to this docs change.
README.md:333-397 fully documents the --strict-keys flag (purpose, lenient default, and usage examples) with all claims verified against the BT-056 implementation and passing tests.

## Summary

Judge Result: DOC-25

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README reference section documents the --strict-keys flag added by BT-056, including its purpose (enforce key uniformity across task rows), default behavior, and usage examples: README.md:333-397 adds section '### validate --strict-keys: key-uniformity enforcement (BT-056)' in the reference/usage area (after the validate section, before the pre-commit hook section). Purpose: declares the task-row key canon (required keys id/title/status/priority, sanctioned extras, _note family) and reports off-canon keys as drift — matches internal/board/keys.go:43 (RequiredTaskRowKeys) and :48-69 (sanctionedExtras). Default behavior: 'by default (lenient mode) drift is only a [warn] and the command exits 0' — matches cmd/boardctl/main.go:1217-1219 where drift is promoted to error only when *strictKeys is set. Usage examples: `validate --strict-keys` (exit 1, RESULT: FAIL) and repair via `update <id> --normalize` (exit 0) — output formats match main.go:875 ('normalized task %s: %s %q -> %q') and keys.go:337-351 (Summary census line 'key uniformity: N/M rows carry only canon keys'). Alias examples why->reasoning and acceptance->acceptance_criteria confirmed at keys.go:176-177. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/ -run 'Keys|StrictKeys|Normalize|Validate|Usage'` all PASS (TestValidateReportsKeyDrift, TestNormalizeTaskRenamesKeyAlias, TestValidateCleanBoardReportsZeroDrift); full `go test -count=1 ./...` only fails TestVulncheck due to an environment govulncheck binary version mismatch (v1.8.0 vs pinned v1.7.0), unrelated to this docs change.
README.md:333-397 fully documents the --strict-keys flag (purpose, lenient default, and usage examples) with all claims verified against the BT-056 implementation and passing tests.

Overall: FAIL ✗
