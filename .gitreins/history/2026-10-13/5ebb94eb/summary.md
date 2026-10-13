# Verdict: DF-BOARDCTL-31

**Task:** validate treats superseded_by duplicates as resolved
**Evaluated:** 2026-10-13T11:23:45.572457
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ after dedupe-board --by-id --apply closes later duplicate lines, boardctl validate on the same board exits 0 (resolved duplicates warn or count separately); plain duplicates without superseded_by still exit 1; build+vet+test green: internal/board/validate.go:132-138 downgrades a later duplicate to a warning when strings.TrimSpace(row.String("superseded_by")) != "", else keeps the byte-identical error. Independently reproduced with freshly built real binaries on a fresh board: pre-repair `boardctl validate` printed `[error] tasks.jsonl line 2: duplicate task id "QA-ASCE-1" (also line 1)` / `RESULT: FAIL (1 error(s), 0 warning(s))` EXIT=1; `dedupe-board --board-dir $BD --by-id --apply` EXIT=0 wrote status=complete, superseded_by="QA-ASCE-1", completed_at; post-repair `boardctl validate` printed `[warn] tasks.jsonl line 2: duplicate task id "QA-ASCE-1" (also line 1) RESOLVED — later line closed as superseded (superseded_by="QA-ASCE-1")` / `RESULT: OK (1 warning(s))` EXIT=0. Plain duplicate (no superseded_by) EXIT=1 with the unchanged error; empty-string superseded_by also EXIT=1. Tests: `go test -count=1 -v -run 'DF31|IDDupe' ./internal/board/... ./cmd/boardctl/...` -> all PASS (TestValidateIDDupeSupersededResolves, TestValidateIDDupePlainStillErrors, TestValidateIDDupeEmptySupersededByStillErrors, TestValidateIDDupeRepairMakesBoardGreen, TestDF31ValidateSupersededDuplicateExitZero, TestDF31ValidatePlainDuplicateStillExitOne, TestDF31DedupeBoardApplyThenValidateGreen). `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` all packages `ok` (cmd/boardctl 11.286s, internal/board 9.057s) with no FAIL; LSP diagnostics empty. README.md --by-id section documents the resolution.
validate now downgrades superseded_by duplicates to a RESOLVED warning (exit 0) while plain duplicates still exit 1, verified end-to-end with real binaries and a fully green build/vet/test run.

## Summary

Judge Result: DF-BOARDCTL-31

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ after dedupe-board --by-id --apply closes later duplicate lines, boardctl validate on the same board exits 0 (resolved duplicates warn or count separately); plain duplicates without superseded_by still exit 1; build+vet+test green: internal/board/validate.go:132-138 downgrades a later duplicate to a warning when strings.TrimSpace(row.String("superseded_by")) != "", else keeps the byte-identical error. Independently reproduced with freshly built real binaries on a fresh board: pre-repair `boardctl validate` printed `[error] tasks.jsonl line 2: duplicate task id "QA-ASCE-1" (also line 1)` / `RESULT: FAIL (1 error(s), 0 warning(s))` EXIT=1; `dedupe-board --board-dir $BD --by-id --apply` EXIT=0 wrote status=complete, superseded_by="QA-ASCE-1", completed_at; post-repair `boardctl validate` printed `[warn] tasks.jsonl line 2: duplicate task id "QA-ASCE-1" (also line 1) RESOLVED — later line closed as superseded (superseded_by="QA-ASCE-1")` / `RESULT: OK (1 warning(s))` EXIT=0. Plain duplicate (no superseded_by) EXIT=1 with the unchanged error; empty-string superseded_by also EXIT=1. Tests: `go test -count=1 -v -run 'DF31|IDDupe' ./internal/board/... ./cmd/boardctl/...` -> all PASS (TestValidateIDDupeSupersededResolves, TestValidateIDDupePlainStillErrors, TestValidateIDDupeEmptySupersededByStillErrors, TestValidateIDDupeRepairMakesBoardGreen, TestDF31ValidateSupersededDuplicateExitZero, TestDF31ValidatePlainDuplicateStillExitOne, TestDF31DedupeBoardApplyThenValidateGreen). `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` all packages `ok` (cmd/boardctl 11.286s, internal/board 9.057s) with no FAIL; LSP diagnostics empty. README.md --by-id section documents the resolution.
validate now downgrades superseded_by duplicates to a RESOLVED warning (exit 0) while plain duplicates still exit 1, verified end-to-end with real binaries and a fully green build/vet/test run.

Overall: PASS ✓
