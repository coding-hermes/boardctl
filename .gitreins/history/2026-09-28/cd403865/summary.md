# Verdict: BT-050

**Task:** wave tick boardctl-2026-09-27-23-52-37: BT-050
**Evaluated:** 2026-09-28T00:55:38.450092
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.978s
- ✓ **tier2**
  - COMPLETE
  ✓ see board row BT-050 acceptance criteria: BT-050 acceptance criteria (from /tmp/brief-BT-050.txt) all met in commit 6a3981e (worktree /home/kara/worktrees/coding-hermes-boardctl-BT-050). (a) Concurrent writers serialize: TestConcurrentWritersSerializeUnderLock PASS — 3 rounds x 6 real goroutines each resolving its own *Board, auditBoard asserts every line valid JSON, task ids unique, event ids unique+strictly increasing, exact counts (37 task/73 event lines). RED control TestConcurrentWritersWithoutLockIsUnsafe PASS and reproduced real corruption on round 0: 'events.jsonl line 4: DUPLICATE event id 3 / id 3 not strictly increasing'; GREEN counterpart 0 problems. (b) create safe under concurrency: TestParallelCreateSameIDExactlyOneWinner PASS — 12 racers x 20 rounds, exactly 1 winner, 11 ErrDuplicateTaskID, RACE-1 appears once on disk. (c) advisory flock on sidecar .lock: lock_unix.go flock(2) LOCK_EX, lock_windows.go LockFileEx, boardLockFileName=".lock"; TestWriteLockIsExclusive PASS, TestLockReleasesWhenHolderDiesWithoutCleanup PASS (fd closed without release -> next acquirer succeeds, no stale-lock path), TestReadersDoNotTakeTheWriteLock PASS (validate under held lock, 3s timeout); no read path calls withWriteLock (grep confirms only 5 write verbs). (d) Byte-preservation + BT-68 note-append tests PASS: TestUpdateNoteAppendsToExisting/Twice/FirstWriteNoSeparator/ReplaceOverwrites/OmittedLeavesKeyUntouched/AppendPreservesEveryOtherLine, TestUpdatePriorityRewritesRowBytePreserving, TestUpdateWritesWorktreeBranchSessionsBytePreserving. (e) Gates: gofmt -l internal/board/ empty (clean); go build ./... exit 0; go vet ./... exit 0; go test ./internal/board/ -count=1 -race ok; cross-compiles windows/darwin/freebsd exit 0. Full go test ./... shows ONE FAIL: cmd/boardctl TestUsageSkillDateGateBitesOnHeaderMutations/date_just_inside_tolerance_passes — reproduced identically on pristine base ae429f1 (wall-clock date arithmetic, 45-day tolerance boundary), and the commit touches no cmd/ files, so it is an attributed pre-existing failure outside this task's scope. (f) docs note: README.md +10 lines in the write-path section stating writers must go through boardctl create/update and what the lock guarantees. (g) file scope respected: commit touches only README.md, internal/board/{board.go,lock.go,lock_unix.go,lock_windows.go,write.go,write_concurrency_test.go} — no Makefile, no cmd/, no iddupe.go. No LSP diagnostics.


## Summary

Judge Result: BT-050

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.978s

Stage tier2: PASS
  COMPLETE
  ✓ see board row BT-050 acceptance criteria: BT-050 acceptance criteria (from /tmp/brief-BT-050.txt) all met in commit 6a3981e (worktree /home/kara/worktrees/coding-hermes-boardctl-BT-050). (a) Concurrent writers serialize: TestConcurrentWritersSerializeUnderLock PASS — 3 rounds x 6 real goroutines each resolving its own *Board, auditBoard asserts every line valid JSON, task ids unique, event ids unique+strictly increasing, exact counts (37 task/73 event lines). RED control TestConcurrentWritersWithoutLockIsUnsafe PASS and reproduced real corruption on round 0: 'events.jsonl line 4: DUPLICATE event id 3 / id 3 not strictly increasing'; GREEN counterpart 0 problems. (b) create safe under concurrency: TestParallelCreateSameIDExactlyOneWinner PASS — 12 racers x 20 rounds, exactly 1 winner, 11 ErrDuplicateTaskID, RACE-1 appears once on disk. (c) advisory flock on sidecar .lock: lock_unix.go flock(2) LOCK_EX, lock_windows.go LockFileEx, boardLockFileName=".lock"; TestWriteLockIsExclusive PASS, TestLockReleasesWhenHolderDiesWithoutCleanup PASS (fd closed without release -> next acquirer succeeds, no stale-lock path), TestReadersDoNotTakeTheWriteLock PASS (validate under held lock, 3s timeout); no read path calls withWriteLock (grep confirms only 5 write verbs). (d) Byte-preservation + BT-68 note-append tests PASS: TestUpdateNoteAppendsToExisting/Twice/FirstWriteNoSeparator/ReplaceOverwrites/OmittedLeavesKeyUntouched/AppendPreservesEveryOtherLine, TestUpdatePriorityRewritesRowBytePreserving, TestUpdateWritesWorktreeBranchSessionsBytePreserving. (e) Gates: gofmt -l internal/board/ empty (clean); go build ./... exit 0; go vet ./... exit 0; go test ./internal/board/ -count=1 -race ok; cross-compiles windows/darwin/freebsd exit 0. Full go test ./... shows ONE FAIL: cmd/boardctl TestUsageSkillDateGateBitesOnHeaderMutations/date_just_inside_tolerance_passes — reproduced identically on pristine base ae429f1 (wall-clock date arithmetic, 45-day tolerance boundary), and the commit touches no cmd/ files, so it is an attributed pre-existing failure outside this task's scope. (f) docs note: README.md +10 lines in the write-path section stating writers must go through boardctl create/update and what the lock guarantees. (g) file scope respected: commit touches only README.md, internal/board/{board.go,lock.go,lock_unix.go,lock_windows.go,write.go,write_concurrency_test.go} — no Makefile, no cmd/, no iddupe.go. No LSP diagnostics.


Overall: PASS ✓
