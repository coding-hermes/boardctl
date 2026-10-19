# Verdict: DF-BOARDCTL-43

**Task:** repo-tracked-board.db makes doctor FAIL
**Evaluated:** 2026-10-19T07:31:07.599183
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ git rm --cached board.db + .gitignore *.db/*.parquet + test asserts no .db in git ls-files output; doctor -C . exits 0 with 0 errors; test FAILS on a repo that tracks a board.db (RED-proven): (1) Untracked: HEAD commit 1a890b2 'fix: untrack board.db + gate tracked caches (DF-BOARDCTL-43)' shows `.coding-hermes/board/board.db | Bin 12288 -> 0 bytes`; `git ls-files | grep -E '\.(db|parquet)$'` returns no output (exit 1). (2) .gitignore lines 9-10 add `.coding-hermes/board/*.db` and `.coding-hermes/board/*.parquet`. (3) internal/board/doctor_test.go:274-301 TestDoctorRepoBoardNoTrackedCaches now runs `git -C <module root> ls-files .coding-hermes/board/` and t.Fatalf's on any .db/.parquet entry (the old version copied the live board into a fresh repo while skipping .db/.parquet, which is why it stayed green). (4) `go build ./cmd/boardctl` exit 0; `/tmp/boardctl doctor -C .` -> 'RESULT: OK (11 warning(s))', exit 0, zero errors (only warnings, incl. the untracked-set fossil warn for the now-untracked board.db). (5) RED-proven: injected a synthetic '.coding-hermes/board/board.db' line into the ls-files output -> `go test -count=1 ./internal/board/ -run TestDoctorRepoBoardNoTrackedCaches` FAILED with 'git-tracked board caches under .coding-hermes/board ... .coding-hermes/board/board.db'; file restored (grep for injection marker = 0 matches). (6) Supporting tests: TestDoctorGitTrackedDBError and TestGitTrackedSetRealExecParity (both subtests) PASS; full `go test -count=1 ./...` -> all packages 'ok' (internal/board ok 0.787s).
board.db is untracked and gitignored, the test now asserts against the real `git ls-files` tracked set and was RED-proven to fail on a tracked board.db, and `boardctl doctor -C .` exits 0 with 0 errors while the full suite passes.

## Summary

Judge Result: DF-BOARDCTL-43

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ git rm --cached board.db + .gitignore *.db/*.parquet + test asserts no .db in git ls-files output; doctor -C . exits 0 with 0 errors; test FAILS on a repo that tracks a board.db (RED-proven): (1) Untracked: HEAD commit 1a890b2 'fix: untrack board.db + gate tracked caches (DF-BOARDCTL-43)' shows `.coding-hermes/board/board.db | Bin 12288 -> 0 bytes`; `git ls-files | grep -E '\.(db|parquet)$'` returns no output (exit 1). (2) .gitignore lines 9-10 add `.coding-hermes/board/*.db` and `.coding-hermes/board/*.parquet`. (3) internal/board/doctor_test.go:274-301 TestDoctorRepoBoardNoTrackedCaches now runs `git -C <module root> ls-files .coding-hermes/board/` and t.Fatalf's on any .db/.parquet entry (the old version copied the live board into a fresh repo while skipping .db/.parquet, which is why it stayed green). (4) `go build ./cmd/boardctl` exit 0; `/tmp/boardctl doctor -C .` -> 'RESULT: OK (11 warning(s))', exit 0, zero errors (only warnings, incl. the untracked-set fossil warn for the now-untracked board.db). (5) RED-proven: injected a synthetic '.coding-hermes/board/board.db' line into the ls-files output -> `go test -count=1 ./internal/board/ -run TestDoctorRepoBoardNoTrackedCaches` FAILED with 'git-tracked board caches under .coding-hermes/board ... .coding-hermes/board/board.db'; file restored (grep for injection marker = 0 matches). (6) Supporting tests: TestDoctorGitTrackedDBError and TestGitTrackedSetRealExecParity (both subtests) PASS; full `go test -count=1 ./...` -> all packages 'ok' (internal/board ok 0.787s).
board.db is untracked and gitignored, the test now asserts against the real `git ls-files` tracked set and was RED-proven to fail on a tracked board.db, and `boardctl doctor -C .` exits 0 with 0 errors while the full suite passes.

Overall: PASS ✓
