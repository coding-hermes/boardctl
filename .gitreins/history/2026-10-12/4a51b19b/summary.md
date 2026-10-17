# Verdict: DF-BOARDCTL-34

**Task:** umbrella stats reject invalid priority buckets
**Evaluated:** 2026-10-12T23:37:21.388137
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Stats must never emit non-vocabulary priority bucket keys; invalid stored priorities are reported as warnings and valid P0..P5 totals remain correct, with regression tests and go build/vet/test passing.: Fix commit 837ac46 (worktree /home/kara/worktrees/boardctl-DF-BOARDCTL-34). internal/board/umbrella.go adds sanitizeBoardStats()/sanitizePriorityMap() gating both per-repo and grand-total priority maps on PriorityVocabulary, plus appendPriorityValueFindings() emitting warn findings with repo/board/value/count; cmd/boardctl/umbrella.go umbrellaExitStats() renders them on stderr and fails under --strict. E2E repro with corrupt priority `P1","source":"qa-dagger` and `P9`: `umbrella --stats` printed only canonical `P2` buckets (no non-vocabulary key), emitted `[warn] board childA ... priority "P1","source":"qa-dagger" on 1 row(s) is not in vocabulary {P0,P1,P2,P3,P4,P5}` and the childB P9 warning, kept `total tasks: 4` (invalid rows still counted), exit 0; `--strict` exit 1. Edge case (board with only invalid priorities) yields an empty priority map + warning. Regression tests: internal/board/df_boardctl_34_test.go TestDFBoardCTL34UmbrellaStatsRejectsInvalidPriorities PASS; cmd/boardctl/df_boardctl_34_cli_test.go TestDFBoardCTL34UmbrellaStatsCLIGatesPriorityBuckets PASS and TestDFBoardCTL34UmbrellaStatsStrictFailsOnPriorityWarnings PASS. `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` all ok (internal/board ok 0.572s, cmd/boardctl ok 2.892s); gofmt clean; LSP diagnostics empty.
Umbrella stats now gates priority buckets on the P0..P5 vocabulary, reports invalid stored priorities as provenance-carrying warnings (failing under --strict), keeps valid totals correct, and all regression tests plus go build/vet/test pass.

## Summary

Judge Result: DF-BOARDCTL-34

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Stats must never emit non-vocabulary priority bucket keys; invalid stored priorities are reported as warnings and valid P0..P5 totals remain correct, with regression tests and go build/vet/test passing.: Fix commit 837ac46 (worktree /home/kara/worktrees/boardctl-DF-BOARDCTL-34). internal/board/umbrella.go adds sanitizeBoardStats()/sanitizePriorityMap() gating both per-repo and grand-total priority maps on PriorityVocabulary, plus appendPriorityValueFindings() emitting warn findings with repo/board/value/count; cmd/boardctl/umbrella.go umbrellaExitStats() renders them on stderr and fails under --strict. E2E repro with corrupt priority `P1","source":"qa-dagger` and `P9`: `umbrella --stats` printed only canonical `P2` buckets (no non-vocabulary key), emitted `[warn] board childA ... priority "P1","source":"qa-dagger" on 1 row(s) is not in vocabulary {P0,P1,P2,P3,P4,P5}` and the childB P9 warning, kept `total tasks: 4` (invalid rows still counted), exit 0; `--strict` exit 1. Edge case (board with only invalid priorities) yields an empty priority map + warning. Regression tests: internal/board/df_boardctl_34_test.go TestDFBoardCTL34UmbrellaStatsRejectsInvalidPriorities PASS; cmd/boardctl/df_boardctl_34_cli_test.go TestDFBoardCTL34UmbrellaStatsCLIGatesPriorityBuckets PASS and TestDFBoardCTL34UmbrellaStatsStrictFailsOnPriorityWarnings PASS. `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` all ok (internal/board ok 0.572s, cmd/boardctl ok 2.892s); gofmt clean; LSP diagnostics empty.
Umbrella stats now gates priority buckets on the P0..P5 vocabulary, reports invalid stored priorities as provenance-carrying warnings (failing under --strict), keeps valid totals correct, and all regression tests plus go build/vet/test pass.

Overall: FAIL ✗
