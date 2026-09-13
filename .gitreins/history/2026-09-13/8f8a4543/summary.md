# Verdict: BT-024

**Task:** update --commit-hash must not overwrite header last_commit; header updates refresh updated_at
**Evaluated:** 2026-09-13T11:39:57.661704
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m6:38AM[0m [32mINF[0m [1mscanned ~2528781 bytes (2.53 MB) in 442ms[0m
[90m6:38AM[0m [32m
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.463s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ Task commit_hash remains on the task row without mutating header last_commit; explicit header --set-last-commit remains supported; every successful SetHeader mutation refreshes an existing header updated_at in its timestamp dialect and preserves untouched bytes for topology A and B; regression tests cover both topologies; go build ./..., go vet ./..., gofmt -l cmd internal empty, go test ./... -count=1 -short, and gitreins guard all pass.: UpdateTask no longer calls SetHeader for CommitHash (internal/board/write.go:562-566 removed the block); commit_hash stays on the task row — board_test.go TestUpdateCommitHashLeavesHeaderLastCommitUntouched asserts header bytes identical before/after and row commit_hash==deadbeef. Explicit flag intact: cmd/boardctl/main.go:602 fs.String("set-last-commit",...), exercised at cmd/boardctl/main_test.go:303. SetHeader (write.go:801-808) unconditionally calls set("updated_at", b.headerTSFormat(header).Now()) on every successful mutation; headerTSFormat (write.go:348-367) samples existing updated_at, else last_tick/created_at/started_at/generated_at, else DefaultTSLayout. Untouched bytes preserved: write.go:810-817 asserts non-header lines byte-identical before atomicRewrite. Topology A tests: TestSetHeaderRefreshesUpdatedAtTopologyA, TestSetHeaderUpdatedAtDialectPreserved, TestSetHeaderUpdatedAtDialectFromLastTick, TestSetHeaderCombinedFlagsRefreshUpdatedAt, TestUpdateCommitHashLeavesHeaderLastCommitUntouched. Topology B tests: TestSetHeaderRefreshesUpdatedAtOnTopologyB (asserts task row round-trips byte-identical), TestUpdateCommitHashPreservesLine1HeaderOnTopologyB, TestCmdTopologyBFullWorkflow. Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l cmd internal` empty (exit 0); `go test ./... -count=1 -short` -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.469s / ok .../internal/board 0.126s / ok .../internal/render 0.017s', exit 0; `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full) ... go_build ok, go_lint ok, go_tests', exit 0. Targeted run of all BT-024/SetHeader/CommitHash tests: all PASS.
BT-024 fully implemented and verified: task commit_hash no longer touches header last_commit, explicit --set-last-commit works, SetHeader refreshes updated_at in the header's dialect while preserving untouched bytes on both topologies, with regression tests and all build/vet/gofmt/test/guard checks passing.

## Summary

Judge Result: BT-024

Stage tier1: PASS
    ✓ secrets: [90m6:38AM[0m [32mINF[0m [1mscanned ~2528781 bytes (2.53 MB) in 442ms[0m
[90m6:38AM[0m [32m
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.463s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ Task commit_hash remains on the task row without mutating header last_commit; explicit header --set-last-commit remains supported; every successful SetHeader mutation refreshes an existing header updated_at in its timestamp dialect and preserves untouched bytes for topology A and B; regression tests cover both topologies; go build ./..., go vet ./..., gofmt -l cmd internal empty, go test ./... -count=1 -short, and gitreins guard all pass.: UpdateTask no longer calls SetHeader for CommitHash (internal/board/write.go:562-566 removed the block); commit_hash stays on the task row — board_test.go TestUpdateCommitHashLeavesHeaderLastCommitUntouched asserts header bytes identical before/after and row commit_hash==deadbeef. Explicit flag intact: cmd/boardctl/main.go:602 fs.String("set-last-commit",...), exercised at cmd/boardctl/main_test.go:303. SetHeader (write.go:801-808) unconditionally calls set("updated_at", b.headerTSFormat(header).Now()) on every successful mutation; headerTSFormat (write.go:348-367) samples existing updated_at, else last_tick/created_at/started_at/generated_at, else DefaultTSLayout. Untouched bytes preserved: write.go:810-817 asserts non-header lines byte-identical before atomicRewrite. Topology A tests: TestSetHeaderRefreshesUpdatedAtTopologyA, TestSetHeaderUpdatedAtDialectPreserved, TestSetHeaderUpdatedAtDialectFromLastTick, TestSetHeaderCombinedFlagsRefreshUpdatedAt, TestUpdateCommitHashLeavesHeaderLastCommitUntouched. Topology B tests: TestSetHeaderRefreshesUpdatedAtOnTopologyB (asserts task row round-trips byte-identical), TestUpdateCommitHashPreservesLine1HeaderOnTopologyB, TestCmdTopologyBFullWorkflow. Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l cmd internal` empty (exit 0); `go test ./... -count=1 -short` -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.469s / ok .../internal/board 0.126s / ok .../internal/render 0.017s', exit 0; `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full) ... go_build ok, go_lint ok, go_tests', exit 0. Targeted run of all BT-024/SetHeader/CommitHash tests: all PASS.
BT-024 fully implemented and verified: task commit_hash no longer touches header last_commit, explicit --set-last-commit works, SetHeader refreshes updated_at in the header's dialect while preserving untouched bytes on both topologies, with regression tests and all build/vet/gofmt/test/guard checks passing.

Overall: PASS ✓
