# Verdict: DF-BOARDCTL-42

**Task:** Document events.jsonl prerequisite for normalize-board --apply
**Evaluated:** 2026-10-17T22:39:49.707949
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Normalize-board --apply fails with 'no JSONL foreman board found' on tasks.jsonl-only boards because board resolution requires events.jsonl. The README normalize-board section must document this prerequisite. Fix: add a note to README.md normalize-board section stating that events.jsonl must exist (even if empty) for --apply to succeed, and explain the board resolution path.: README.md:736-750 (commit c56758b, 'docs: document normalize-board events.jsonl prerequisite (DF-BOARDCTL-42)') adds a '**Board resolution prerequisite for `--apply`:**' note inside the `## normalize-board` section (heading README.md:696). It states the resolver looks for tasks.jsonl + events.jsonl, that a tasks.jsonl-only dir exits 2 with 'no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)', that board.jsonl is optional (topology A vs B), and that such boards 'just need an empty `events.jsonl` so the resolver can confirm the directory is a complete board.' Code matches: internal/board/board.go:160 requires fileExists(tasks) && fileExists(events); board.go:176 emits the exact 'a tasks.jsonl-only board: events.jsonl is missing' hint; board.go:55 ErrBoardNotFound = 'no JSONL foreman board found'; boardFromDir (board.go:196-212) sets Topology A only when board.jsonl exists. Empirical probe with freshly built binary: tasks.jsonl-only board -> `normalize-board --apply` printed 'boardctl: no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)' EXIT=2; after creating an EMPTY events.jsonl the same command printed 'board already canonical — nothing to change (nothing written)' EXIT=0, confirming the 'even if empty' claim. Test suite: `go test -count=1 ./...` exit_code 0, all packages ok (cmd/boardctl 4.900s, internal/board 0.972s, internal/render 0.028s, etc.).
The README normalize-board section now documents the events.jsonl prerequisite and board resolution path, and both the documented exit-2 failure and the empty-events.jsonl success were empirically confirmed with a passing full test suite.

## Summary

Judge Result: DF-BOARDCTL-42

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Normalize-board --apply fails with 'no JSONL foreman board found' on tasks.jsonl-only boards because board resolution requires events.jsonl. The README normalize-board section must document this prerequisite. Fix: add a note to README.md normalize-board section stating that events.jsonl must exist (even if empty) for --apply to succeed, and explain the board resolution path.: README.md:736-750 (commit c56758b, 'docs: document normalize-board events.jsonl prerequisite (DF-BOARDCTL-42)') adds a '**Board resolution prerequisite for `--apply`:**' note inside the `## normalize-board` section (heading README.md:696). It states the resolver looks for tasks.jsonl + events.jsonl, that a tasks.jsonl-only dir exits 2 with 'no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)', that board.jsonl is optional (topology A vs B), and that such boards 'just need an empty `events.jsonl` so the resolver can confirm the directory is a complete board.' Code matches: internal/board/board.go:160 requires fileExists(tasks) && fileExists(events); board.go:176 emits the exact 'a tasks.jsonl-only board: events.jsonl is missing' hint; board.go:55 ErrBoardNotFound = 'no JSONL foreman board found'; boardFromDir (board.go:196-212) sets Topology A only when board.jsonl exists. Empirical probe with freshly built binary: tasks.jsonl-only board -> `normalize-board --apply` printed 'boardctl: no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)' EXIT=2; after creating an EMPTY events.jsonl the same command printed 'board already canonical — nothing to change (nothing written)' EXIT=0, confirming the 'even if empty' claim. Test suite: `go test -count=1 ./...` exit_code 0, all packages ok (cmd/boardctl 4.900s, internal/board 0.972s, internal/render 0.028s, etc.).
The README normalize-board section now documents the events.jsonl prerequisite and board resolution path, and both the documented exit-2 failure and the empty-events.jsonl success were empirically confirmed with a passing full test suite.

Overall: PASS ✓
