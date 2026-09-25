# Verdict: BT-066

**Task:** sweep-status --apply aborts mid-apply on off-vocab ci_result/guard_result with partial writes
**Evaluated:** 2026-09-25T19:45:48.923358
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.424s
- ✓ **tier2**
  - COMPLETE
  ✓ internal/board/sweep.go: off-vocab guard_result/ci_result rows are classified as Explicit up front (dry-run reports them, never advertised fixable alongside a status alias); apply skips-and-reports NormalizeTask failures instead of aborting mid-file; OffAfter measured from file counts blocked rows; regression tests in internal/board/sweep_result_block_test.go; go build ./... && go vet ./... && go test ./... green: sweep.go:113-135 classifies guard_result/ci_result vocab columns FIRST (before status), appending off-vocab rows to rep.Explicit with Blocked/BlockedValue and returning early so they never enter rep.Normalized (never advertised fixable alongside a status alias). sweep.go:170-181 apply loop: on NormalizeTask err it sets Skipped=true, appends to skipped, and `continue` — no abort mid-file. sweep.go:194 sets rep.OffAfter = b.offVocabularyCount() (measured from file), and offVocabularyCount (lines 205-243) counts rows with off-vocab guard_result/ci_result. Regression tests present in internal/board/sweep_result_block_test.go (4 BT066 tests). Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` all packages `ok` exit 0; `go test -count=1 ./internal/board/ -run BT066 -v` shows PASS for TestBT066DryRunNamesOffVocabResultAsExplicit, TestBT066ApplySkipsRefusedRowAndRepairsTheRest, TestBT066AfterCountCountsBlockedRows, TestBT066CleanVocabApplyIsByteIdenticalNoop.
All BT-066 requirements are implemented in sweep.go and covered by passing regression tests; build, vet, and full test suite are green.

## Summary

Judge Result: BT-066

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.424s

Stage tier2: PASS
  COMPLETE
  ✓ internal/board/sweep.go: off-vocab guard_result/ci_result rows are classified as Explicit up front (dry-run reports them, never advertised fixable alongside a status alias); apply skips-and-reports NormalizeTask failures instead of aborting mid-file; OffAfter measured from file counts blocked rows; regression tests in internal/board/sweep_result_block_test.go; go build ./... && go vet ./... && go test ./... green: sweep.go:113-135 classifies guard_result/ci_result vocab columns FIRST (before status), appending off-vocab rows to rep.Explicit with Blocked/BlockedValue and returning early so they never enter rep.Normalized (never advertised fixable alongside a status alias). sweep.go:170-181 apply loop: on NormalizeTask err it sets Skipped=true, appends to skipped, and `continue` — no abort mid-file. sweep.go:194 sets rep.OffAfter = b.offVocabularyCount() (measured from file), and offVocabularyCount (lines 205-243) counts rows with off-vocab guard_result/ci_result. Regression tests present in internal/board/sweep_result_block_test.go (4 BT066 tests). Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` all packages `ok` exit 0; `go test -count=1 ./internal/board/ -run BT066 -v` shows PASS for TestBT066DryRunNamesOffVocabResultAsExplicit, TestBT066ApplySkipsRefusedRowAndRepairsTheRest, TestBT066AfterCountCountsBlockedRows, TestBT066CleanVocabApplyIsByteIdenticalNoop.
All BT-066 requirements are implemented in sweep.go and covered by passing regression tests; build, vet, and full test suite are green.

Overall: PASS ✓
