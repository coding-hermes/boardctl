# Verdict: DF-BOARDCTL-42

**Task:** Document events.jsonl prerequisite for normalize-board --apply
**Evaluated:** 2026-10-17T22:38:31.380117
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Normalize-board --apply fails with 'no JSONL foreman board found' on tasks.jsonl-only boards because board resolution requires events.jsonl. The README normalize-board section must document this prerequisite. Fix: add a note to README.md normalize-board section stating that events.jsonl must exist (even if empty) for --apply to succeed, and explain the board resolution path.: The note was added (commit 82cf26c, README.md:736-750, inside the '## normalize-board' section at README.md:696) and the events.jsonl half is correct and verified empirically: a tasks.jsonl-only board gives `boardctl -C $D normalize-board --apply` -> EXIT=2 'no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)', and adding an empty events.jsonl makes it EXIT=0 'applied: ... rewritten in one atomic pass'. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/` -> exit 0, 'ok internal/board 0.974s', 'ok cmd/boardctl 4.900s'. HOWEVER the required 'explain the board resolution path' is documented INCORRECTLY. README.md:737-739 states normalize-board 'requires the board dir to contain `board.jsonl` (the header) AND `events.jsonl` (even if empty)'. That is false: internal/board/board.go:159-190 Resolve() checks only tasks.jsonl + events.jsonl, and boardFromDir (board.go:203-211) treats board.jsonl as optional (present -> Topology A, absent -> Topology B). Verified empirically: a board with tasks.jsonl + empty events.jsonl and NO board.jsonl runs `normalize-board --apply` successfully with EXIT=0 ('applied: ... rewritten in one atomic pass'). The README's follow-on sentence at :747-750 ('If you are repairing a legacy board that still uses topology B (header on line 1 of tasks.jsonl, no board.jsonl), create an empty events.jsonl first') compounds the error by implying topology-B boards need board.jsonl. The doc must be corrected to state that only tasks.jsonl + events.jsonl are required and that board.jsonl is optional (topology A vs B).


## Summary

Judge Result: DF-BOARDCTL-42

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Normalize-board --apply fails with 'no JSONL foreman board found' on tasks.jsonl-only boards because board resolution requires events.jsonl. The README normalize-board section must document this prerequisite. Fix: add a note to README.md normalize-board section stating that events.jsonl must exist (even if empty) for --apply to succeed, and explain the board resolution path.: The note was added (commit 82cf26c, README.md:736-750, inside the '## normalize-board' section at README.md:696) and the events.jsonl half is correct and verified empirically: a tasks.jsonl-only board gives `boardctl -C $D normalize-board --apply` -> EXIT=2 'no JSONL foreman board found: ... (a tasks.jsonl-only board: events.jsonl is missing)', and adding an empty events.jsonl makes it EXIT=0 'applied: ... rewritten in one atomic pass'. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/` -> exit 0, 'ok internal/board 0.974s', 'ok cmd/boardctl 4.900s'. HOWEVER the required 'explain the board resolution path' is documented INCORRECTLY. README.md:737-739 states normalize-board 'requires the board dir to contain `board.jsonl` (the header) AND `events.jsonl` (even if empty)'. That is false: internal/board/board.go:159-190 Resolve() checks only tasks.jsonl + events.jsonl, and boardFromDir (board.go:203-211) treats board.jsonl as optional (present -> Topology A, absent -> Topology B). Verified empirically: a board with tasks.jsonl + empty events.jsonl and NO board.jsonl runs `normalize-board --apply` successfully with EXIT=0 ('applied: ... rewritten in one atomic pass'). The README's follow-on sentence at :747-750 ('If you are repairing a legacy board that still uses topology B (header on line 1 of tasks.jsonl, no board.jsonl), create an empty events.jsonl first') compounds the error by implying topology-B boards need board.jsonl. The doc must be corrected to state that only tasks.jsonl + events.jsonl are required and that board.jsonl is optional (topology A vs B).


Overall: FAIL ✗
