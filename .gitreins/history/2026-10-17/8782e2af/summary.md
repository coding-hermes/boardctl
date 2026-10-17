# Verdict: DF-BOARDCTL-40

**Task:** P3 - verify unknown-command exit code claim
**Evaluated:** 2026-10-17T12:27:34.908313
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Close row with the discrimination probe: download v0.1.10 binary and verify its exit code on an unknown command is 2, not 0, as the row claims. If exit code is 2, close the row as premise-false with the probe evidence.: Probe independently reproduced on the v0.1.10 asset at /tmp/boardctl-probe-v0110/boardctl_linux_amd64 (sha256 20ebddf6...f406ccc5). Identity confirmed genuine v0.1.10: `version` -> 'boardctl version v0.1.10' rc=0; `version --json` -> {"version":"v0.1.10","build":"v0.1.10"}. Authenticity cross-check: the binary LACKS normalize-board ('unknown command') while HEAD cmd/boardctl/main.go:210 has `case "normalize-board":` — proving it is the old release asset, not a HEAD build. PROBE 1 (direct $?): `./boardctl_linux_amd64 bogus-cmd` -> unknown_cmd_rc=2 with stderr 'boardctl: unknown command "bogus-cmd"' + usage. PROBE 2 (the row's own AC, pipeline capture): `out=$(./boardctl_linux_amd64 normalize-board --help 2>&1); rc=$?` -> captured_rc=2. PROBE 3 (direct): normalize-board --help -> direct_rc=2. Exit code is 2, NOT 0, so the row premise is false and the criterion's conditional ('If exit code is 2, close the row as premise-false with the probe evidence') is satisfied. Row closed at .coding-hermes/board/tasks.jsonl:170: status="complete", worker_status="complete", completed_at="2026-10-17 08:42:00", guard_result="PASS", commit_hash="99a69d3", worker_summary="Premise-false: discrimination probe confirms v0.1.10 exits 2 on unknown commands. Direct probe: exit 2. Pipeline capture probe (the row's own AC): exit 2...", foreman_note records both probe commands and their exit 2, and detail.evidence carries the new run_id boardctl-foreman-2026-10-17-12-13-53 (ts 2026-10-17 12:18:23). Source contract corroborates: cmd/boardctl/main.go:231-232 `default: fmt.Fprintf(os.Stderr, "boardctl: unknown command %q..."); return 2`. Board validates clean with the closure: `boardctl validate` -> 'RESULT: OK (8 warning(s))' exit 0. Tests: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` TEST_EXIT=0 with 12 'ok' packages and 0 FAIL. Minor non-disqualifying caveat: the row closure is staged (git status 'M ') rather than yet in HEAD — `git show HEAD:.coding-hermes/board/tasks.jsonl` still shows status=pending — which matches this repo's tick-commit workflow (cf. DF-BOARDCTL-38 closed in a84f255/99a69d3); the criterion's requirement to close the row is met in the working-tree board.
The discrimination probe was genuinely run against the real v0.1.10 release binary and shows exit code 2 (not 0) on unknown commands both directly and via pipeline capture, and the row was closed as premise-false with that probe evidence recorded in the board.

## Summary

Judge Result: DF-BOARDCTL-40

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Close row with the discrimination probe: download v0.1.10 binary and verify its exit code on an unknown command is 2, not 0, as the row claims. If exit code is 2, close the row as premise-false with the probe evidence.: Probe independently reproduced on the v0.1.10 asset at /tmp/boardctl-probe-v0110/boardctl_linux_amd64 (sha256 20ebddf6...f406ccc5). Identity confirmed genuine v0.1.10: `version` -> 'boardctl version v0.1.10' rc=0; `version --json` -> {"version":"v0.1.10","build":"v0.1.10"}. Authenticity cross-check: the binary LACKS normalize-board ('unknown command') while HEAD cmd/boardctl/main.go:210 has `case "normalize-board":` — proving it is the old release asset, not a HEAD build. PROBE 1 (direct $?): `./boardctl_linux_amd64 bogus-cmd` -> unknown_cmd_rc=2 with stderr 'boardctl: unknown command "bogus-cmd"' + usage. PROBE 2 (the row's own AC, pipeline capture): `out=$(./boardctl_linux_amd64 normalize-board --help 2>&1); rc=$?` -> captured_rc=2. PROBE 3 (direct): normalize-board --help -> direct_rc=2. Exit code is 2, NOT 0, so the row premise is false and the criterion's conditional ('If exit code is 2, close the row as premise-false with the probe evidence') is satisfied. Row closed at .coding-hermes/board/tasks.jsonl:170: status="complete", worker_status="complete", completed_at="2026-10-17 08:42:00", guard_result="PASS", commit_hash="99a69d3", worker_summary="Premise-false: discrimination probe confirms v0.1.10 exits 2 on unknown commands. Direct probe: exit 2. Pipeline capture probe (the row's own AC): exit 2...", foreman_note records both probe commands and their exit 2, and detail.evidence carries the new run_id boardctl-foreman-2026-10-17-12-13-53 (ts 2026-10-17 12:18:23). Source contract corroborates: cmd/boardctl/main.go:231-232 `default: fmt.Fprintf(os.Stderr, "boardctl: unknown command %q..."); return 2`. Board validates clean with the closure: `boardctl validate` -> 'RESULT: OK (8 warning(s))' exit 0. Tests: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` TEST_EXIT=0 with 12 'ok' packages and 0 FAIL. Minor non-disqualifying caveat: the row closure is staged (git status 'M ') rather than yet in HEAD — `git show HEAD:.coding-hermes/board/tasks.jsonl` still shows status=pending — which matches this repo's tick-commit workflow (cf. DF-BOARDCTL-38 closed in a84f255/99a69d3); the criterion's requirement to close the row is met in the working-tree board.
The discrimination probe was genuinely run against the real v0.1.10 release binary and shows exit code 2 (not 0) on unknown commands both directly and via pipeline capture, and the row was closed as premise-false with that probe evidence recorded in the board.

Overall: PASS ✓
