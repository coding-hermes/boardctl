# Verdict: BT-026

**Task:** Legacy and partial board error UX
**Evaluated:** 2026-09-14T13:25:24.400096
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m8:24AM[0m [32mINF[0m [1mscanned ~819987 bytes (819.99 KB) in 455ms[0m
[90m8:24AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.705s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ When tasks.jsonl exists but events.jsonl is missing, board resolution fails clearly as a partial or legacy board and explicitly names the missing events.jsonl file.: internal/board/board.go Resolve() tracks a `partial` candidate and returns fmt.Errorf("%w: %s — found %s", ErrBoardNotFound, target, partial) where partial = "<path>/tasks.jsonl (a tasks.jsonl-only board: events.jsonl is missing)". Live probe: tasks.jsonl-only board at repo root, .coding-hermes, and board dir all exit 2 with stderr 'no JSONL foreman board found: <target> — found <path>/tasks.jsonl (a tasks.jsonl-only board: events.jsonl is missing)'. errors.Is(ErrBoardNotFound) preserved. Tests TestResolveTasksOnlyBoardNamesMissingEvents, TestResolveTasksOnlyAtRepoRoot, TestResolveEventsOnlyBoardNamesMissingTasks PASS.
  ✓ Line-oriented JSON parse failures identify the exact file and 1-based line or row context; pretty-printed multi-line task JSON is diagnosed as valid JSON that is not JSONL-line-loadable with an actionable migration hint.: board.go IterParsed emits fmt.Errorf("line %d: %w ...", i+1, err) (1-based) and, when isEOFJSONErr(err) && wholeFileIsValidJSON(lines), appends 'the source is valid JSON, but it is not line-loadable JSONL — JSONL requires one complete JSON object per line; pretty-printed objects span lines. Compact the file to one line per object, e.g. with jq -c'. ReadAllRows wraps with "%s: %w" (path). Live probe pretty-printed tasks.jsonl: exit 1, stderr '<path>/tasks.jsonl: line 1: EOF (the source is valid JSON, but it is not line-loadable JSONL ... jq -c)'. Corrupt JSON keeps plain 'line 1: EOF' (not mislabeled). Tests TestReadAllRowsPrettyPrintedDiagnosed, TestReadAllRowsCorruptJSONKeepsPlainError, TestIterParsedPrettyPrintedDiagnosisWithoutPath PASS.
  ✓ Regression tests cover partial tasks-only boards and pretty-printed task boards through the CLI, preserving documented exit-code behavior and existing topology A/B behavior.: cmd/boardctl/bt026_errorux_test.go: TestCmdPartialBoardExit2AndMessage (exit 2 at 3 -C depths, names missing events.jsonl), TestCmdPrettyPrintedTasksExit1AndMessage (exit 1, file+line 1+valid JSON+not line-loadable JSONL), TestCmdCorruptTasksKeepsPlainError (exit 1, no mislabel), TestCmdTopologyBStillResolves (exit 0, 'total tasks: 1'). internal/board/bt026_errorux_test.go covers Resolve partial/events-only/full-beats-partial/generic-no-hint + ReadAllRows + IterParsed. All PASS. Live probes confirm exit codes 2/1/0 and topology A (exit 0) + topology B (exit 0, total tasks: 1) preserved; README documents exit codes 0/1/2.
  ✓ go build ./..., go vet ./..., go test ./... -count=1 -short, GitReins guard, and live temp-board probes pass.: go build ./... exit 0 ('build OK'); go vet ./... exit 0 ('vet OK'); go test ./... -count=1 -short exit 0 (ok cmd/boardctl 0.650s, ok internal/board 0.125s, ok internal/render 0.014s; 206 PASS / 0 FAIL); gitreins guard exit 0 'Tier 1 Guards: PASS (test mode: full) — secrets clean, go_build ok, go_lint ok, go_tests'. Live temp-board probes: partial board exit 2, pretty-printed exit 1, corrupt exit 1, topology A exit 0, topology B exit 0.
All four BT-026 criteria verified: partial/legacy board resolution names the missing events.jsonl (exit 2), pretty-printed JSONL is diagnosed with file+1-based line and a jq -c migration hint (exit 1), CLI regression tests cover both cases plus topology A/B, and build/vet/test/guard/live probes all pass.

## Summary

Judge Result: BT-026

Stage tier1: PASS
    ✓ secrets: [90m8:24AM[0m [32mINF[0m [1mscanned ~819987 bytes (819.99 KB) in 455ms[0m
[90m8:24AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.705s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ When tasks.jsonl exists but events.jsonl is missing, board resolution fails clearly as a partial or legacy board and explicitly names the missing events.jsonl file.: internal/board/board.go Resolve() tracks a `partial` candidate and returns fmt.Errorf("%w: %s — found %s", ErrBoardNotFound, target, partial) where partial = "<path>/tasks.jsonl (a tasks.jsonl-only board: events.jsonl is missing)". Live probe: tasks.jsonl-only board at repo root, .coding-hermes, and board dir all exit 2 with stderr 'no JSONL foreman board found: <target> — found <path>/tasks.jsonl (a tasks.jsonl-only board: events.jsonl is missing)'. errors.Is(ErrBoardNotFound) preserved. Tests TestResolveTasksOnlyBoardNamesMissingEvents, TestResolveTasksOnlyAtRepoRoot, TestResolveEventsOnlyBoardNamesMissingTasks PASS.
  ✓ Line-oriented JSON parse failures identify the exact file and 1-based line or row context; pretty-printed multi-line task JSON is diagnosed as valid JSON that is not JSONL-line-loadable with an actionable migration hint.: board.go IterParsed emits fmt.Errorf("line %d: %w ...", i+1, err) (1-based) and, when isEOFJSONErr(err) && wholeFileIsValidJSON(lines), appends 'the source is valid JSON, but it is not line-loadable JSONL — JSONL requires one complete JSON object per line; pretty-printed objects span lines. Compact the file to one line per object, e.g. with jq -c'. ReadAllRows wraps with "%s: %w" (path). Live probe pretty-printed tasks.jsonl: exit 1, stderr '<path>/tasks.jsonl: line 1: EOF (the source is valid JSON, but it is not line-loadable JSONL ... jq -c)'. Corrupt JSON keeps plain 'line 1: EOF' (not mislabeled). Tests TestReadAllRowsPrettyPrintedDiagnosed, TestReadAllRowsCorruptJSONKeepsPlainError, TestIterParsedPrettyPrintedDiagnosisWithoutPath PASS.
  ✓ Regression tests cover partial tasks-only boards and pretty-printed task boards through the CLI, preserving documented exit-code behavior and existing topology A/B behavior.: cmd/boardctl/bt026_errorux_test.go: TestCmdPartialBoardExit2AndMessage (exit 2 at 3 -C depths, names missing events.jsonl), TestCmdPrettyPrintedTasksExit1AndMessage (exit 1, file+line 1+valid JSON+not line-loadable JSONL), TestCmdCorruptTasksKeepsPlainError (exit 1, no mislabel), TestCmdTopologyBStillResolves (exit 0, 'total tasks: 1'). internal/board/bt026_errorux_test.go covers Resolve partial/events-only/full-beats-partial/generic-no-hint + ReadAllRows + IterParsed. All PASS. Live probes confirm exit codes 2/1/0 and topology A (exit 0) + topology B (exit 0, total tasks: 1) preserved; README documents exit codes 0/1/2.
  ✓ go build ./..., go vet ./..., go test ./... -count=1 -short, GitReins guard, and live temp-board probes pass.: go build ./... exit 0 ('build OK'); go vet ./... exit 0 ('vet OK'); go test ./... -count=1 -short exit 0 (ok cmd/boardctl 0.650s, ok internal/board 0.125s, ok internal/render 0.014s; 206 PASS / 0 FAIL); gitreins guard exit 0 'Tier 1 Guards: PASS (test mode: full) — secrets clean, go_build ok, go_lint ok, go_tests'. Live temp-board probes: partial board exit 2, pretty-printed exit 1, corrupt exit 1, topology A exit 0, topology B exit 0.
All four BT-026 criteria verified: partial/legacy board resolution names the missing events.jsonl (exit 2), pretty-printed JSONL is diagnosed with file+1-based line and a jq -c migration hint (exit 1), CLI regression tests cover both cases plus topology A/B, and build/vet/test/guard/live probes all pass.

Overall: PASS ✓
