# Verdict: SWEEP-STATUS-DECISIONS

**Task:** boardctl: explicit-decision sweep mode — decide+fix off-vocab result columns and duplicate/parked statuses fleet-wide
**Evaluated:** 2026-09-30T12:26:02.043891
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.876s
- ✓ **tier2**
  - COMPLETE
  ✓ 1) sweep-status (and a new --decide mode) accepts machine-checkable decisions: result columns (guard_result/ci_result) whose value CONTAINS a canonical token (e.g. 'PASS 5/5', 'PENDING', 'pipeline SUCCESS') normalize to PASS/FAIL/GREEN/RED/SKIP by prefix/keyword rule; 2) status 'duplicate' rows with a distinct-id earlier twin are closed via the dedupe-board --by-id machinery (status=complete, superseded_by=<kept id>, summary preserved); 3) unknown/ambiguous values remain refuse-to-guess; 4) regression tests cover each rule incl. a REFUSED ambiguous case; 5) full gate battery green (build/vet/test/fmt-check/version-check): (1) internal/board/sweepdecide.go:78 DecideResultValue implements prefix/keyword rules: first-token PASS|OK->PASS, FAIL|ERROR->FAIL, GREEN->GREEN, RED->RED; anywhere standalone SKIP or literal N/A->SKIP; standalone SUCCESS->GREEN(ci)/PASS(guard). Wired via cmd/boardctl/main.go --decide flag -> b.StatusSweepDecide(apply,decide) (sweep.go:132). TestDecideResultValueTable pins 'PASS 5/5'->PASS, 'pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)'->GREEN, 'N/A (...)'->SKIP. (2) sweep.go:305-315 routes duplicate-with-twin rows through b.CloseIDDupeLine (iddupe.go:313), which sets status=complete, superseded_by=<kept id>, worker_summary 'superseded-by-earliest: recycled task id — kept line K', completed_at, plus one audit event quoting original title/reasoning/status verbatim; TestStatusSweepDecideApplyWrites asserts status=complete, superseded_by=DEC-DUP, summary 'kept line 4', title/reasoning preserved. (3) Ambiguity gate sweepdecide.go:82 refuses values naming both PASS and FAIL; PENDING and unmatched values fall through to explicit (sweep.go:229 rep.Explicit). (4) Tests: internal/board/sweep_decide_test.go (TestDecideResultValueTable incl. REFUSED 'PASS but really FAIL', 'first FAIL then PASS on rerun', 'PENDING', 'PASSED is not PASS', unmatched) and cmd/boardctl/review_sweep_decide_test.go (TestCmdSweepStatusDecideDryRun asserts explicit refusals for AC-PENDING and twin-less AC-LONE). Ran `go test -count=1 -v -run 'Decide|SweepStatusDecide|CloseIDDupeLine|SetResultValue|ResultDecideRuleDoc' ./internal/board/ ./cmd/boardctl/` — all 12 tests PASS, exit 0. (5) Gate battery: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 (all packages ok, incl. internal/fmtcheck, internal/versioncheck); `make fmt-check` exit 0 (ok internal/fmtcheck); `make version-check` exit 0 (ok internal/versioncheck).


## Summary

Judge Result: SWEEP-STATUS-DECISIONS

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.876s

Stage tier2: PASS
  COMPLETE
  ✓ 1) sweep-status (and a new --decide mode) accepts machine-checkable decisions: result columns (guard_result/ci_result) whose value CONTAINS a canonical token (e.g. 'PASS 5/5', 'PENDING', 'pipeline SUCCESS') normalize to PASS/FAIL/GREEN/RED/SKIP by prefix/keyword rule; 2) status 'duplicate' rows with a distinct-id earlier twin are closed via the dedupe-board --by-id machinery (status=complete, superseded_by=<kept id>, summary preserved); 3) unknown/ambiguous values remain refuse-to-guess; 4) regression tests cover each rule incl. a REFUSED ambiguous case; 5) full gate battery green (build/vet/test/fmt-check/version-check): (1) internal/board/sweepdecide.go:78 DecideResultValue implements prefix/keyword rules: first-token PASS|OK->PASS, FAIL|ERROR->FAIL, GREEN->GREEN, RED->RED; anywhere standalone SKIP or literal N/A->SKIP; standalone SUCCESS->GREEN(ci)/PASS(guard). Wired via cmd/boardctl/main.go --decide flag -> b.StatusSweepDecide(apply,decide) (sweep.go:132). TestDecideResultValueTable pins 'PASS 5/5'->PASS, 'pipeline 1166 SUCCESS all 6 jobs (lint GREEN first time)'->GREEN, 'N/A (...)'->SKIP. (2) sweep.go:305-315 routes duplicate-with-twin rows through b.CloseIDDupeLine (iddupe.go:313), which sets status=complete, superseded_by=<kept id>, worker_summary 'superseded-by-earliest: recycled task id — kept line K', completed_at, plus one audit event quoting original title/reasoning/status verbatim; TestStatusSweepDecideApplyWrites asserts status=complete, superseded_by=DEC-DUP, summary 'kept line 4', title/reasoning preserved. (3) Ambiguity gate sweepdecide.go:82 refuses values naming both PASS and FAIL; PENDING and unmatched values fall through to explicit (sweep.go:229 rep.Explicit). (4) Tests: internal/board/sweep_decide_test.go (TestDecideResultValueTable incl. REFUSED 'PASS but really FAIL', 'first FAIL then PASS on rerun', 'PENDING', 'PASSED is not PASS', unmatched) and cmd/boardctl/review_sweep_decide_test.go (TestCmdSweepStatusDecideDryRun asserts explicit refusals for AC-PENDING and twin-less AC-LONE). Ran `go test -count=1 -v -run 'Decide|SweepStatusDecide|CloseIDDupeLine|SetResultValue|ResultDecideRuleDoc' ./internal/board/ ./cmd/boardctl/` — all 12 tests PASS, exit 0. (5) Gate battery: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 (all packages ok, incl. internal/fmtcheck, internal/versioncheck); `make fmt-check` exit 0 (ok internal/fmtcheck); `make version-check` exit 0 (ok internal/versioncheck).


Overall: PASS ✓
