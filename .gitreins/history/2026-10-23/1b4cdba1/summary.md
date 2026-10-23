# Verdict: DF-BOARDCTL-47

**Task:** [P2] umbrella aborts the WHOLE cross-board view on one unparseable line in one child board (no --skip-bad-lines) — every other verb has a degrade mode
**Evaluated:** 2026-10-23T20:50:59.178291
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ 1) `umbrella --skip-bad-lines` exists and degrades: every parseable row of every healthy child board is counted, each skipped board/line is reported with file:line on stderr, exit 1. 2) On the boardctl+warpfs pair above, `umbrella --stats` and `umbrella --list` print both boards' totals plus a SKIPPED block instead of aborting. 3) Without the flag the current fail-closed behaviour is unchanged (rc=1, same message). 4) `umbrella --help` documents both modes, and a regression test drives a fixture umbrella whose second child carries a duplicate-key line. 5) `go test ./...` exits 0.: All 5 sub-criteria verified with live binary runs + test output.

(1) Flag exists: cmd/boardctl/umbrella.go:63 `skipBad := useSkipBad(fs)`, :131-133 `if *skipBad { um.SetSkipBad() }`; internal/board/umbrella.go SetSkipBad() arms SkipBad on every board in the view. Live run on a boardctl+warpfs fixture (warpfs tasks.jsonl line 2 = duplicate `worker_status` key): `umbrella --stats --skip-bad-lines` printed `repo boardctl (2 tasks)` + `repo warpfs (1 tasks)` + `umbrella total: total tasks: 3` — every parseable row of both healthy boards counted — then `SKIPPED-LINES: 1 line(s) could not be parsed and were skipped:` / `warpfs (/tmp/umb/warpfs/.coding-hermes/board): /tmp/umb/warpfs/.coding-hermes/board/tasks.jsonl line 2: duplicate key "worker_status" in row` + snippet, rc=1. Stream check: stdout grep for SKIPPED-LINES = 0, stderr grep = 1 (evidence is on stderr); `--json --skip-bad-lines` stdout still parses as JSON (boards=2).

(2) `--stats` and `--list` both degrade instead of aborting: --stats output above; `--list --skip-bad-lines` printed rows `boardctl BC-1`, `boardctl BC-2`, `warpfs WF-1` plus the same SKIPPED-LINES block, rc=1.

(3) Default unchanged: `umbrella --stats` with no flag -> `boardctl: board warpfs (/tmp/umb/warpfs/.coding-hermes/board): /tmp/umb/warpfs/.coding-hermes/board/tasks.jsonl: line 2: duplicate key "worker_status" in row`, rc=1, no partial output. umbrella.go:186-189 makes the degraded clause conditional on skipped>0, so the default message is byte-identical to the pre-change wording at :192.

(4) `umbrella --help` (rc=0) prints a `Line-level read modes (DF-BOARDCTL-47):` section documenting both `(default) fail-closed: the FIRST unparseable line in ANY board aborts the whole view with that board's file and line (exit 1, no partial output).` and `--skip-bad-lines degraded: every parseable row from every healthy board is still counted/listed ... reported on stderr as SKIPPED-LINES ... exits 1`. Regression fixture matches the criterion exactly: internal/board/df_boardctl_47_test.go:62-70 df47Fixture() = childA healthy with 2 tasks, childB with 1 healthy task followed by one duplicate-key line (the SECOND child), with premise assertions that the line is valid JSON but ParseRow rejects it for `duplicate key`. Tests pass: TestUmbrellaDefaultFailClosed, TestUmbrellaSkipBadAggregatesHealthyBoards, TestUmbrellaSkipBadDedupesAcrossReads, TestUmbrellaSkipBadHealthyBoardsClean all `--- PASS`; CLI side TestDF47DefaultFailClosedUnchanged, TestDF47StatsSkipBadDegrades, TestDF47ListSkipBadDegrades, TestDF47CensusSkipBadDegrades, TestDF47JSONSkipBadKeepsStdoutParseable, TestDF47HelpDocumentsBothModes, TestDF47SkipBadOnHealthyUmbrellaStaysClean, TestDF47SkipBadIsReadOnly all `--- PASS`.

(5) `go test -count=1 ./...` -> TEST_EXIT=0, 12 packages reported `ok` (cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/vulncheck, internal/workflowcheck), zero FAIL/panic lines. `go build ./...` also exit 0.
`umbrella --skip-bad-lines` is implemented and verified end-to-end: it counts every parseable row across both boards, emits a stderr SKIPPED-LINES block with repo + file:line + parse error, exits 1, leaves the default fail-closed path byte-identical, documents both modes in --help, ships library+CLI regression tests on a second-child duplicate-key fixture, and `go test -count=1 ./...` exits 0.

## Summary

Judge Result: DF-BOARDCTL-47

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ 1) `umbrella --skip-bad-lines` exists and degrades: every parseable row of every healthy child board is counted, each skipped board/line is reported with file:line on stderr, exit 1. 2) On the boardctl+warpfs pair above, `umbrella --stats` and `umbrella --list` print both boards' totals plus a SKIPPED block instead of aborting. 3) Without the flag the current fail-closed behaviour is unchanged (rc=1, same message). 4) `umbrella --help` documents both modes, and a regression test drives a fixture umbrella whose second child carries a duplicate-key line. 5) `go test ./...` exits 0.: All 5 sub-criteria verified with live binary runs + test output.

(1) Flag exists: cmd/boardctl/umbrella.go:63 `skipBad := useSkipBad(fs)`, :131-133 `if *skipBad { um.SetSkipBad() }`; internal/board/umbrella.go SetSkipBad() arms SkipBad on every board in the view. Live run on a boardctl+warpfs fixture (warpfs tasks.jsonl line 2 = duplicate `worker_status` key): `umbrella --stats --skip-bad-lines` printed `repo boardctl (2 tasks)` + `repo warpfs (1 tasks)` + `umbrella total: total tasks: 3` — every parseable row of both healthy boards counted — then `SKIPPED-LINES: 1 line(s) could not be parsed and were skipped:` / `warpfs (/tmp/umb/warpfs/.coding-hermes/board): /tmp/umb/warpfs/.coding-hermes/board/tasks.jsonl line 2: duplicate key "worker_status" in row` + snippet, rc=1. Stream check: stdout grep for SKIPPED-LINES = 0, stderr grep = 1 (evidence is on stderr); `--json --skip-bad-lines` stdout still parses as JSON (boards=2).

(2) `--stats` and `--list` both degrade instead of aborting: --stats output above; `--list --skip-bad-lines` printed rows `boardctl BC-1`, `boardctl BC-2`, `warpfs WF-1` plus the same SKIPPED-LINES block, rc=1.

(3) Default unchanged: `umbrella --stats` with no flag -> `boardctl: board warpfs (/tmp/umb/warpfs/.coding-hermes/board): /tmp/umb/warpfs/.coding-hermes/board/tasks.jsonl: line 2: duplicate key "worker_status" in row`, rc=1, no partial output. umbrella.go:186-189 makes the degraded clause conditional on skipped>0, so the default message is byte-identical to the pre-change wording at :192.

(4) `umbrella --help` (rc=0) prints a `Line-level read modes (DF-BOARDCTL-47):` section documenting both `(default) fail-closed: the FIRST unparseable line in ANY board aborts the whole view with that board's file and line (exit 1, no partial output).` and `--skip-bad-lines degraded: every parseable row from every healthy board is still counted/listed ... reported on stderr as SKIPPED-LINES ... exits 1`. Regression fixture matches the criterion exactly: internal/board/df_boardctl_47_test.go:62-70 df47Fixture() = childA healthy with 2 tasks, childB with 1 healthy task followed by one duplicate-key line (the SECOND child), with premise assertions that the line is valid JSON but ParseRow rejects it for `duplicate key`. Tests pass: TestUmbrellaDefaultFailClosed, TestUmbrellaSkipBadAggregatesHealthyBoards, TestUmbrellaSkipBadDedupesAcrossReads, TestUmbrellaSkipBadHealthyBoardsClean all `--- PASS`; CLI side TestDF47DefaultFailClosedUnchanged, TestDF47StatsSkipBadDegrades, TestDF47ListSkipBadDegrades, TestDF47CensusSkipBadDegrades, TestDF47JSONSkipBadKeepsStdoutParseable, TestDF47HelpDocumentsBothModes, TestDF47SkipBadOnHealthyUmbrellaStaysClean, TestDF47SkipBadIsReadOnly all `--- PASS`.

(5) `go test -count=1 ./...` -> TEST_EXIT=0, 12 packages reported `ok` (cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/vulncheck, internal/workflowcheck), zero FAIL/panic lines. `go build ./...` also exit 0.
`umbrella --skip-bad-lines` is implemented and verified end-to-end: it counts every parseable row across both boards, emits a stderr SKIPPED-LINES block with repo + file:line + parse error, exits 1, leaves the default fail-closed path byte-identical, documents both modes in --help, ships library+CLI regression tests on a second-child duplicate-key fixture, and `go test -count=1 ./...` exits 0.

Overall: PASS ✓
