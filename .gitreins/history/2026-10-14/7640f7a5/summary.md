# Verdict: DF-BOARDCTL-33

**Task:** dedupe-board --by-id --lanes refusal exit-code unification
**Evaluated:** 2026-10-14T10:40:38.491678
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ dedupe-board --by-id --lanes <other> refusal must exit 2 in text mode (same as --json), proven by live CLI probe + regression test in cmd/dedupe-board: Live CLI probe on a real board fixture: `/tmp/dedupe-board --board-dir /tmp/probe2 --by-id --lanes crier` printed `dedupe-board: refusing to touch board /tmp/probe2/tasks.jsonl (lane "asce") — it is not in --lanes crier` and exited 2; the same command with `--json` also exited 2 — text and JSON modes are unified. Code: cmd/dedupe-board/main.go:139-149 — the --lanes guard executes before the `if *byID` dispatch (line 152) and before any --json branch, returning 2 unconditionally, so the refusal is mode-independent by construction. Regression test: cmd/dedupe-board/main_test.go:414-444 `TestCLI_ByIDLanesRefusalExitsTwoTextMode` (added in HEAD bd87fd1, +34 lines) asserts code==2, stderr contains "refusing to touch board", and the board snapshot is unchanged (no write on refusal). Fresh test runs (-count=1): `go test -count=1 -run TestCLI_ByIDLanesRefusalExitsTwoTextMode ./cmd/dedupe-board/ -v` → `--- PASS: TestCLI_ByIDLanesRefusalExitsTwoTextMode (0.00s)` / `ok github.com/coding-hermes/boardctl/cmd/dedupe-board 0.003s`; `go test -count=1 ./cmd/dedupe-board/` → `ok ... 0.008s`; `go test -count=1 ./...` → all packages ok, exit 0.
The --by-id --lanes refusal exits 2 in text mode (verified by live CLI probe matching the --json exit code) and is pinned by a passing regression test in cmd/dedupe-board.

## Summary

Judge Result: DF-BOARDCTL-33

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ dedupe-board --by-id --lanes <other> refusal must exit 2 in text mode (same as --json), proven by live CLI probe + regression test in cmd/dedupe-board: Live CLI probe on a real board fixture: `/tmp/dedupe-board --board-dir /tmp/probe2 --by-id --lanes crier` printed `dedupe-board: refusing to touch board /tmp/probe2/tasks.jsonl (lane "asce") — it is not in --lanes crier` and exited 2; the same command with `--json` also exited 2 — text and JSON modes are unified. Code: cmd/dedupe-board/main.go:139-149 — the --lanes guard executes before the `if *byID` dispatch (line 152) and before any --json branch, returning 2 unconditionally, so the refusal is mode-independent by construction. Regression test: cmd/dedupe-board/main_test.go:414-444 `TestCLI_ByIDLanesRefusalExitsTwoTextMode` (added in HEAD bd87fd1, +34 lines) asserts code==2, stderr contains "refusing to touch board", and the board snapshot is unchanged (no write on refusal). Fresh test runs (-count=1): `go test -count=1 -run TestCLI_ByIDLanesRefusalExitsTwoTextMode ./cmd/dedupe-board/ -v` → `--- PASS: TestCLI_ByIDLanesRefusalExitsTwoTextMode (0.00s)` / `ok github.com/coding-hermes/boardctl/cmd/dedupe-board 0.003s`; `go test -count=1 ./cmd/dedupe-board/` → `ok ... 0.008s`; `go test -count=1 ./...` → all packages ok, exit 0.
The --by-id --lanes refusal exits 2 in text mode (verified by live CLI probe matching the --json exit code) and is pinned by a passing regression test in cmd/dedupe-board.

Overall: PASS ✓
