# Verdict: DF-BOARDCTL-35

**Task:** doctor warns on untracked db/parquet fossils in the board dir
**Evaluated:** 2026-10-13T11:24:50.385908
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ doctor exits 0 with a warning naming an untracked board.db/parquet in the board dir; tracked .db still errors; clean dir unchanged; build+vet+test green: Code: internal/board/doctor.go doctorUntrackedCacheFossils (commit 14366e2) walks the board dir, warns on untracked .db/.parquet via rep.Add("warn", "untracked-set: %s ... cache fossil"), skips tracked files (pathTracked) and the .lock sidecar. E2E with built binary /tmp/boardctl in a temp git repo: (1) untracked board.db -> '[warn] untracked-set: .coding-hermes/board/board.db sits in the board dir but is not tracked by git — a leftover db/parquet cache fossil', 'RESULT: OK (2 warning(s))', EXIT=0; untracked events.parquet and gitignored board.db likewise warn with EXIT=0. (2) tracked board.db -> '[error] git-tracked-set: .coding-hermes/board/board.db is tracked under .coding-hermes/board', 'RESULT: FAIL (1 error(s), 1 warning(s))', EXIT=1, and zero fossil warns (no double-report). (3) clean dir -> no untracked-set finding, 'RESULT: OK (1 warning(s))', EXIT=0; board.lock sidecar -> 0 untracked-set warns; no-repo dir -> 'not inside a git repo ... cannot be classified' warn, EXIT=0. Tests: `go test ./internal/board/ -count=1 -run 'DF35|Fossil|Doctor' -v` -> all PASS including TestDoctorUntrackedDBFossilWarns, TestDoctorUntrackedParquetFossilWarns, TestDoctorTrackedDBNoFossilWarn, TestDoctorCleanDirNoFossilWarn, TestDoctorFossilWarnOutsideGitRepo, TestDoctorFossilWarnRealExecParity (3/3 subtests). Full suite `go test ./... -count=1` -> every package 'ok' (cmd/boardctl 7.751s, internal/board 3.788s, etc.). `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l internal/board/ cmd/` empty.
doctor warns (exit 0) on untracked board.db/parquet fossils, still errors on tracked .db, leaves clean dirs unchanged, and build+vet+full test suite are green.

## Summary

Judge Result: DF-BOARDCTL-35

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ doctor exits 0 with a warning naming an untracked board.db/parquet in the board dir; tracked .db still errors; clean dir unchanged; build+vet+test green: Code: internal/board/doctor.go doctorUntrackedCacheFossils (commit 14366e2) walks the board dir, warns on untracked .db/.parquet via rep.Add("warn", "untracked-set: %s ... cache fossil"), skips tracked files (pathTracked) and the .lock sidecar. E2E with built binary /tmp/boardctl in a temp git repo: (1) untracked board.db -> '[warn] untracked-set: .coding-hermes/board/board.db sits in the board dir but is not tracked by git — a leftover db/parquet cache fossil', 'RESULT: OK (2 warning(s))', EXIT=0; untracked events.parquet and gitignored board.db likewise warn with EXIT=0. (2) tracked board.db -> '[error] git-tracked-set: .coding-hermes/board/board.db is tracked under .coding-hermes/board', 'RESULT: FAIL (1 error(s), 1 warning(s))', EXIT=1, and zero fossil warns (no double-report). (3) clean dir -> no untracked-set finding, 'RESULT: OK (1 warning(s))', EXIT=0; board.lock sidecar -> 0 untracked-set warns; no-repo dir -> 'not inside a git repo ... cannot be classified' warn, EXIT=0. Tests: `go test ./internal/board/ -count=1 -run 'DF35|Fossil|Doctor' -v` -> all PASS including TestDoctorUntrackedDBFossilWarns, TestDoctorUntrackedParquetFossilWarns, TestDoctorTrackedDBNoFossilWarn, TestDoctorCleanDirNoFossilWarn, TestDoctorFossilWarnOutsideGitRepo, TestDoctorFossilWarnRealExecParity (3/3 subtests). Full suite `go test ./... -count=1` -> every package 'ok' (cmd/boardctl 7.751s, internal/board 3.788s, etc.). `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l internal/board/ cmd/` empty.
doctor warns (exit 0) on untracked board.db/parquet fossils, still errors on tracked .db, leaves clean dirs unchanged, and build+vet+full test suite are green.

Overall: PASS ✓
