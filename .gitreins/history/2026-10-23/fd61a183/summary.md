# Verdict: DF-BOARDCTL-49

**Task:** normalize-board repair-events concurrent append preservation and atomic rewrite
**Evaluated:** 2026-10-23T21:35:19.003469
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ 1) Concurrent event append survives normalize-board --repair-events --apply: internal/board/repair_events.go:64-75 — RepairEvents wraps the entire repairEventsLocked body (read/classify/plan/rewrite) in b.withWriteLock(b.eventsPath, ...). AppendEvent takes the same guard at internal/board/write.go:1457-1468 (withWriteLock(b.eventsPath, ...)), so an append cannot interleave between the repair's snapshot and its rewrite. Verified by TestConcurrentRepairEventsAppendCannotBeLost which asserts the PROBE-EV-1 marker count == 1 after a concurrent append during the pre-rewrite hook.
  ✓ 2) events.jsonl replacement is atomic under kill -9: internal/board/repair_events.go:227 calls atomicRewrite(b.eventsPath, out). atomicRewrite at internal/board/write.go:186-206 creates a same-directory temp file (os.CreateTemp(dir, ".boardctl-rewrite-*")), writes, Chmods, Closes, then os.Rename(tmpName, path). A same-filesystem rename is atomic on POSIX, so a kill -9 at any point leaves either the complete old file or the complete new file — never a truncated/partial events.jsonl. (No fsync before rename, so power-loss durability is not guaranteed, but the criterion is process-kill atomicity, which rename satisfies.)
  ✓ 3) DedupeByID guards its read-rewrite window or has an explicit exemption: internal/board/iddupe.go:109-120 — DedupeByID runs b.dedupeByIDLocked(apply) inside b.withWriteLock(b.tasksPath, ...), with the explicit doc comment at iddupe.go:105-108: 'The write guard covers the complete read/classify/plan/rewrite-and-audit sequence so a concurrent writer cannot be lost between the snapshot and atomicRewrite.' The locked body reads via ReadJSONLLines (iddupe.go:123) and rewrites via atomicRewrite (iddupe.go:249, 430) — both inside the guard.
  ✓ 4) concurrent regression test is present and passes: internal/board/repair_events_test.go:19 defines TestConcurrentRepairEventsAppendCannotBeLost. It uses the test-only seam repairEventsBeforeRewriteHook (repair_events.go:57-59) to pause repair after planning and before the atomic replacement, starts a concurrent AppendEvent from a second Board handle, then asserts the marker appears exactly once. Ran: `go test -count=1 -race -run 'TestConcurrentRepairEventsAppendCannotBeLost' -v ./internal/board/` -> `=== RUN TestConcurrentRepairEventsAppendCannotBeLost` / `--- PASS: TestConcurrentRepairEventsAppendCannotBeLost (0.01s)` / `ok github.com/coding-hermes/boardctl/internal/board 1.023s`, EXIT=0. The test also documents the RED control via writeLockBypass (lock.go:100-113, write_concurrency_test.go:248-249), proving it exercises the production guard rather than sequential behavior.
  ✓ 5) go test ./... passes: Ran `go test -count=1 ./...` (cache-defeating flag) at repo root: EXIT=0, 12 packages reported `ok` (cmd/boardctl 6.642s, cmd/dedupe-board, cmd/gendocs, internal/board 2.038s, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/vulncheck, internal/workflowcheck); grep for FAIL/panic returned no matches.
All five criteria are satisfied: RepairEvents and DedupeByID both hold the board write guard across their full read-plan-rewrite windows, events.jsonl is replaced via same-directory temp+rename (atomic under kill -9), the concurrent regression test exists and passes under -race, and the full `go test -count=1 ./...` suite exits 0 with 12 packages ok.

## Summary

Judge Result: DF-BOARDCTL-49

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ 1) Concurrent event append survives normalize-board --repair-events --apply: internal/board/repair_events.go:64-75 — RepairEvents wraps the entire repairEventsLocked body (read/classify/plan/rewrite) in b.withWriteLock(b.eventsPath, ...). AppendEvent takes the same guard at internal/board/write.go:1457-1468 (withWriteLock(b.eventsPath, ...)), so an append cannot interleave between the repair's snapshot and its rewrite. Verified by TestConcurrentRepairEventsAppendCannotBeLost which asserts the PROBE-EV-1 marker count == 1 after a concurrent append during the pre-rewrite hook.
  ✓ 2) events.jsonl replacement is atomic under kill -9: internal/board/repair_events.go:227 calls atomicRewrite(b.eventsPath, out). atomicRewrite at internal/board/write.go:186-206 creates a same-directory temp file (os.CreateTemp(dir, ".boardctl-rewrite-*")), writes, Chmods, Closes, then os.Rename(tmpName, path). A same-filesystem rename is atomic on POSIX, so a kill -9 at any point leaves either the complete old file or the complete new file — never a truncated/partial events.jsonl. (No fsync before rename, so power-loss durability is not guaranteed, but the criterion is process-kill atomicity, which rename satisfies.)
  ✓ 3) DedupeByID guards its read-rewrite window or has an explicit exemption: internal/board/iddupe.go:109-120 — DedupeByID runs b.dedupeByIDLocked(apply) inside b.withWriteLock(b.tasksPath, ...), with the explicit doc comment at iddupe.go:105-108: 'The write guard covers the complete read/classify/plan/rewrite-and-audit sequence so a concurrent writer cannot be lost between the snapshot and atomicRewrite.' The locked body reads via ReadJSONLLines (iddupe.go:123) and rewrites via atomicRewrite (iddupe.go:249, 430) — both inside the guard.
  ✓ 4) concurrent regression test is present and passes: internal/board/repair_events_test.go:19 defines TestConcurrentRepairEventsAppendCannotBeLost. It uses the test-only seam repairEventsBeforeRewriteHook (repair_events.go:57-59) to pause repair after planning and before the atomic replacement, starts a concurrent AppendEvent from a second Board handle, then asserts the marker appears exactly once. Ran: `go test -count=1 -race -run 'TestConcurrentRepairEventsAppendCannotBeLost' -v ./internal/board/` -> `=== RUN TestConcurrentRepairEventsAppendCannotBeLost` / `--- PASS: TestConcurrentRepairEventsAppendCannotBeLost (0.01s)` / `ok github.com/coding-hermes/boardctl/internal/board 1.023s`, EXIT=0. The test also documents the RED control via writeLockBypass (lock.go:100-113, write_concurrency_test.go:248-249), proving it exercises the production guard rather than sequential behavior.
  ✓ 5) go test ./... passes: Ran `go test -count=1 ./...` (cache-defeating flag) at repo root: EXIT=0, 12 packages reported `ok` (cmd/boardctl 6.642s, cmd/dedupe-board, cmd/gendocs, internal/board 2.038s, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/vulncheck, internal/workflowcheck); grep for FAIL/panic returned no matches.
All five criteria are satisfied: RepairEvents and DedupeByID both hold the board write guard across their full read-plan-rewrite windows, events.jsonl is replaced via same-directory temp+rename (atomic under kill -9), the concurrent regression test exists and passes under -race, and the full `go test -count=1 ./...` suite exits 0 with 12 packages ok.

Overall: PASS ✓
