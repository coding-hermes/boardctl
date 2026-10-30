# Verdict: DF-BOARDCTL-10

**Task:** ID-recycled near-dupes: add id-aware dedupe mode to doctor
**Evaluated:** 2026-10-30T19:16:44.260738
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ doctor --json outputs id-groups; validate flags dupes; new 'dedupe --by-id' collapses superseded lines while preserving text in events: Three of four sub-claims hold, but the 'preserving text in events' half is missing. (a) doctor --json: PASS — live run emitted "id_groups":[{"id":"DUP-1","lines":[1,2,3],"row_ids":["DUP-1","DUP-1","DUP-1"]}] (internal/board/doctor.go:60-64, internal/board/validate.go:31 json tag `id_groups`); TestDFBoardctl10DoctorJSONAndDedupeDryRun PASS. (b) validate flags dupes: PASS — `boardctl validate` printed `[error] tasks.jsonl line 2: duplicate task id "DUP-1" (also line 1)` and line 3, RESULT: FAIL (2 errors), rc=1 (internal/board/validate.go:127-140). (c) dedupe --by-id collapses superseded lines: PASS — later rows rewritten to "status":"superseded" with "superseded by line 1" and their title/reasoning folded into the kept row (internal/board/dedupe.go DedupeByIDSupersede). (d) preserving text in EVENTS: FAIL — after `dedupe --by-id` events.jsonl was byte-identical to before (only the seed audit event id=1); no task_superseded/task_updated event appended. grep for AppendEvent|task_superseded in internal/board/dedupe.go returns 0 matches; preserved text exists only in the kept row's reasoning field, not in events. The sibling repair internal/board/recycled.go:167-180 does append a task_updated event with task_superseded detail, so the dedupe path is the outlier. Additionally `dedupe --by-id --apply` is rejected with `flag provided but not defined: -apply` (cmd/boardctl/main.go:1526-1540 defines only --by-id and --dry-run; apply is implicit), so the expected apply form fails. Test evidence: `go test ./cmd/boardctl -run TestDFBoardctl10 -count=1 -v` -> PASS; full `go test ./... -count=1 -short` exit 0 (all packages ok) — the suite passes only because the test asserts the dry-run diff and never asserts an event is written.
doctor --json id_groups, validate duplicate errors, and dedupe --by-id supersession all work, but dedupe writes no event preserving the superseded text (and --apply is not a recognized flag), so the criterion's 'preserving text in events' requirement is unmet.

## Summary

Judge Result: DF-BOARDCTL-10

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ doctor --json outputs id-groups; validate flags dupes; new 'dedupe --by-id' collapses superseded lines while preserving text in events: Three of four sub-claims hold, but the 'preserving text in events' half is missing. (a) doctor --json: PASS — live run emitted "id_groups":[{"id":"DUP-1","lines":[1,2,3],"row_ids":["DUP-1","DUP-1","DUP-1"]}] (internal/board/doctor.go:60-64, internal/board/validate.go:31 json tag `id_groups`); TestDFBoardctl10DoctorJSONAndDedupeDryRun PASS. (b) validate flags dupes: PASS — `boardctl validate` printed `[error] tasks.jsonl line 2: duplicate task id "DUP-1" (also line 1)` and line 3, RESULT: FAIL (2 errors), rc=1 (internal/board/validate.go:127-140). (c) dedupe --by-id collapses superseded lines: PASS — later rows rewritten to "status":"superseded" with "superseded by line 1" and their title/reasoning folded into the kept row (internal/board/dedupe.go DedupeByIDSupersede). (d) preserving text in EVENTS: FAIL — after `dedupe --by-id` events.jsonl was byte-identical to before (only the seed audit event id=1); no task_superseded/task_updated event appended. grep for AppendEvent|task_superseded in internal/board/dedupe.go returns 0 matches; preserved text exists only in the kept row's reasoning field, not in events. The sibling repair internal/board/recycled.go:167-180 does append a task_updated event with task_superseded detail, so the dedupe path is the outlier. Additionally `dedupe --by-id --apply` is rejected with `flag provided but not defined: -apply` (cmd/boardctl/main.go:1526-1540 defines only --by-id and --dry-run; apply is implicit), so the expected apply form fails. Test evidence: `go test ./cmd/boardctl -run TestDFBoardctl10 -count=1 -v` -> PASS; full `go test ./... -count=1 -short` exit 0 (all packages ok) — the suite passes only because the test asserts the dry-run diff and never asserts an event is written.
doctor --json id_groups, validate duplicate errors, and dedupe --by-id supersession all work, but dedupe writes no event preserving the superseded text (and --apply is not a recognized flag), so the criterion's 'preserving text in events' requirement is unmet.

Overall: FAIL ✗
