# Verdict: DF-BOARDCTL-43

**Task:** repo-tracked-board.db makes doctor FAIL
**Evaluated:** 2026-10-19T07:31:59.684271
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ git rm --cached board.db + .gitignore *.db/*.parquet + test asserts no .db in git ls-files output; doctor -C . exits 0 with 0 errors; test FAILS on a repo that tracks a board.db (RED-proven): All parts verified with real command output. (1) Untracked: HEAD commit 1a890b2 'fix: untrack board.db + gate tracked caches (DF-BOARDCTL-43)' shows '.coding-hermes/board/board.db | Bin 12288 -> 0 bytes'; `git ls-files .coding-hermes/board/` now returns only board.jsonl/events.jsonl/fixtures.jsonl/tasks.jsonl, and `git ls-files | grep -E '\.(db|parquet)$'` exits 1 (no matches). (2) .gitignore contains '.coding-hermes/board/*.db' and '.coding-hermes/board/*.parquet'. (3) Test assertion: internal/board/doctor_test.go:275-300 TestDoctorRepoBoardNoTrackedCaches resolves the module root, runs `git -C root ls-files .coding-hermes/board/`, and t.Fatalf's on any .db/.parquet entry (line 297). (4) doctor clean: built ./cmd/boardctl and ran `/tmp/boardctl doctor -C .` => exit_code 0, 'RESULT: OK (10 warning(s))', zero errors. (5) RED-proven: temporarily `git add -f .coding-hermes/board/board.db` then `go test -count=1 ./internal/board/ -run TestDoctorRepoBoardNoTrackedCaches -v` => '--- FAIL: TestDoctorRepoBoardNoTrackedCaches (0.00s)' with 'doctor_test.go:297: git-tracked board caches under .coding-hermes/board ... .coding-hermes/board/board.db', and doctor printed '[error] git-tracked-set: .coding-hermes/board/board.db is tracked under .coding-hermes/board' with 'RESULT: FAIL (1 error(s), 10 warning(s))'. Index restored afterwards; full package run `go test -count=1 ./internal/board/` => 'ok github.com/coding-hermes/boardctl/internal/board 0.994s'.
board.db is untracked and gitignored, the test reads the real `git ls-files` set and fails RED when a board.db is tracked, and `boardctl doctor -C .` exits 0 with 0 errors on the clean repo.

## Summary

Judge Result: DF-BOARDCTL-43

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ git rm --cached board.db + .gitignore *.db/*.parquet + test asserts no .db in git ls-files output; doctor -C . exits 0 with 0 errors; test FAILS on a repo that tracks a board.db (RED-proven): All parts verified with real command output. (1) Untracked: HEAD commit 1a890b2 'fix: untrack board.db + gate tracked caches (DF-BOARDCTL-43)' shows '.coding-hermes/board/board.db | Bin 12288 -> 0 bytes'; `git ls-files .coding-hermes/board/` now returns only board.jsonl/events.jsonl/fixtures.jsonl/tasks.jsonl, and `git ls-files | grep -E '\.(db|parquet)$'` exits 1 (no matches). (2) .gitignore contains '.coding-hermes/board/*.db' and '.coding-hermes/board/*.parquet'. (3) Test assertion: internal/board/doctor_test.go:275-300 TestDoctorRepoBoardNoTrackedCaches resolves the module root, runs `git -C root ls-files .coding-hermes/board/`, and t.Fatalf's on any .db/.parquet entry (line 297). (4) doctor clean: built ./cmd/boardctl and ran `/tmp/boardctl doctor -C .` => exit_code 0, 'RESULT: OK (10 warning(s))', zero errors. (5) RED-proven: temporarily `git add -f .coding-hermes/board/board.db` then `go test -count=1 ./internal/board/ -run TestDoctorRepoBoardNoTrackedCaches -v` => '--- FAIL: TestDoctorRepoBoardNoTrackedCaches (0.00s)' with 'doctor_test.go:297: git-tracked board caches under .coding-hermes/board ... .coding-hermes/board/board.db', and doctor printed '[error] git-tracked-set: .coding-hermes/board/board.db is tracked under .coding-hermes/board' with 'RESULT: FAIL (1 error(s), 10 warning(s))'. Index restored afterwards; full package run `go test -count=1 ./internal/board/` => 'ok github.com/coding-hermes/boardctl/internal/board 0.994s'.
board.db is untracked and gitignored, the test reads the real `git ls-files` set and fails RED when a board.db is tracked, and `boardctl doctor -C .` exits 0 with 0 errors on the clean repo.

Overall: PASS ✓
