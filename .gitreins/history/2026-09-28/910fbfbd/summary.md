# Verdict: DF-BOARDCTL-10

**Task:** wave tick boardctl-2026-09-27-23-52-37: DF-BOARDCTL-10
**Evaluated:** 2026-09-28T00:53:45.068900
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.435s
- ✓ **tier2**
  - COMPLETE
  ✓ see board row DF-BOARDCTL-10 acceptance criteria: Board row DF-BOARDCTL-10 (from /tmp/brief-DF-BOARDCTL-10.txt) acceptance criteria all met. (1) New --by-id mode: cmd/dedupe-board/main.go:75 defines the flag, :152 dispatches to runByID, :203 runByID; internal/board/iddupe.go DedupeByID groups rows by id, keeps the EARLIEST line per id byte-identical (members[0]), closes later lines as superseded-by-earliest (status=complete, superseded_by=<kept id>, worker_summary naming kept line, completed_at now), and appends one audit event per closed line quoting the original title+reasoning verbatim (idDupeEventDetail with SetEscapeHTML(false)); dry-run is the default and returns before any write (iddupe.go:171). (2) Default fingerprint mode unchanged: main.go:157 DedupeBackfill path untouched; TestCLI_DefaultFingerprintModeUnchanged PASS. (3) All 5 required tests present and PASS: asce-shaped 6-rows-one-id (TestIDDupe_AsceShapeSixRowsOneID), mixed fixture (TestIDDupe_MixedFixtureFingerprintDupesAndRecycledIDs), dry-run sha256 snapshot writes nothing (TestIDDupe_DryRunWritesNothing + TestCLI_ByIDDryRunWritesNothing), idempotent apply (TestIDDupe_ApplyIsIdempotent), CLI e2e through real main path (TestCLI_ByIDAsceShapeEndToEnd); the [no test files] residual is retired (go test -list shows 7 CLI tests). (4) Help text documents the fresh-id rule: main.go:95 'FRESH-ID RULE: a lane must ALWAYS mint a fresh task id for a new finding'; TestCLI_HelpDocumentsFreshIDRule PASS. (5) Gates: `go build ./...` exit 0; `gofmt -l` on the 4 touched files returned empty (clean); `go vet ./...` exit 0; `go test ./cmd/dedupe-board ./internal/board -count=1` => 'ok github.com/coding-hermes/boardctl/cmd/dedupe-board 0.010s' and 'ok github.com/coding-hermes/boardctl/internal/board 0.180s'. Full `go test ./... -count=1` shows one failure, TestUsageSkillDateGateBitesOnHeaderMutations in cmd/boardctl/df15_skill_closure_test.go — a pre-existing, time-dependent failure (hardcoded skill date 2026-08-14 vs run date 2026-09-27) in a file NOT touched by this change; verified it fails identically on base commit ae429f1 in a separate worktree, so it is unrelated to DF-BOARDCTL-10. LSP diagnostics: 0 findings.
All DF-BOARDCTL-10 acceptance criteria are satisfied: the new --by-id id-aware dedupe mode, unchanged default behavior, all five required tests, documented fresh-id rule, and clean build/gofmt/vet plus passing task-relevant tests (the sole full-suite failure is a pre-existing time-dependent test in an untouched file).

## Summary

Judge Result: DF-BOARDCTL-10

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.435s

Stage tier2: PASS
  COMPLETE
  ✓ see board row DF-BOARDCTL-10 acceptance criteria: Board row DF-BOARDCTL-10 (from /tmp/brief-DF-BOARDCTL-10.txt) acceptance criteria all met. (1) New --by-id mode: cmd/dedupe-board/main.go:75 defines the flag, :152 dispatches to runByID, :203 runByID; internal/board/iddupe.go DedupeByID groups rows by id, keeps the EARLIEST line per id byte-identical (members[0]), closes later lines as superseded-by-earliest (status=complete, superseded_by=<kept id>, worker_summary naming kept line, completed_at now), and appends one audit event per closed line quoting the original title+reasoning verbatim (idDupeEventDetail with SetEscapeHTML(false)); dry-run is the default and returns before any write (iddupe.go:171). (2) Default fingerprint mode unchanged: main.go:157 DedupeBackfill path untouched; TestCLI_DefaultFingerprintModeUnchanged PASS. (3) All 5 required tests present and PASS: asce-shaped 6-rows-one-id (TestIDDupe_AsceShapeSixRowsOneID), mixed fixture (TestIDDupe_MixedFixtureFingerprintDupesAndRecycledIDs), dry-run sha256 snapshot writes nothing (TestIDDupe_DryRunWritesNothing + TestCLI_ByIDDryRunWritesNothing), idempotent apply (TestIDDupe_ApplyIsIdempotent), CLI e2e through real main path (TestCLI_ByIDAsceShapeEndToEnd); the [no test files] residual is retired (go test -list shows 7 CLI tests). (4) Help text documents the fresh-id rule: main.go:95 'FRESH-ID RULE: a lane must ALWAYS mint a fresh task id for a new finding'; TestCLI_HelpDocumentsFreshIDRule PASS. (5) Gates: `go build ./...` exit 0; `gofmt -l` on the 4 touched files returned empty (clean); `go vet ./...` exit 0; `go test ./cmd/dedupe-board ./internal/board -count=1` => 'ok github.com/coding-hermes/boardctl/cmd/dedupe-board 0.010s' and 'ok github.com/coding-hermes/boardctl/internal/board 0.180s'. Full `go test ./... -count=1` shows one failure, TestUsageSkillDateGateBitesOnHeaderMutations in cmd/boardctl/df15_skill_closure_test.go — a pre-existing, time-dependent failure (hardcoded skill date 2026-08-14 vs run date 2026-09-27) in a file NOT touched by this change; verified it fails identically on base commit ae429f1 in a separate worktree, so it is unrelated to DF-BOARDCTL-10. LSP diagnostics: 0 findings.
All DF-BOARDCTL-10 acceptance criteria are satisfied: the new --by-id id-aware dedupe mode, unchanged default behavior, all five required tests, documented fresh-id rule, and clean build/gofmt/vet plus passing task-relevant tests (the sole full-suite failure is a pre-existing time-dependent test in an untouched file).

Overall: PASS ✓
