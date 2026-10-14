# Verdict: DF-BOARDCTL-32

**Task:** recycled-id census miscounts pre-repair boards
**Evaluated:** 2026-10-14T10:59:21.456517
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ On a board with recycled ids whose later lines are ALL already terminal, dedupe-board --by-id dry-run text output must say 'nothing to do — N recycled ids, all later lines already terminal' (not 'no recycled task ids on this board'); --json must carry per-id already_terminal_lines in groups (and/or a top-level field) so 4-1=3 is explainable: Live run on a board with 2 recycled ids whose later lines are all terminal: dry-run text printed exactly 'nothing to do — 2 recycled id(s), all later lines already terminal (no files written)' (exit 0), NOT the old 'no recycled task ids on this board'. Implemented in cmd/dedupe-board/main.go nothingToDoLine(): RecycledIDs==0 keeps the old line verbatim, else emits the new line; called in both dry-run and applied branches. --json emitted {"recycled_ids":2,"groups":null,"already_terminal_ids":["QA-ASCE-1","QA-TB-1"]} — internal/board/iddupe.go adds AlreadyTerminalIDs []string `json:"already_terminal_ids,omitempty"`, populated in DedupeByID's else-branch when len(g.ClosedLines)==0. The 4-1=3 case is explainable: mixed board gave recycled_ids=4, groups=1, already_terminal_ids=['QA-B-1','QA-C-1','QA-D-1'] (4 = 1 + 3), with the invariant recycled_ids == len(Groups)+len(AlreadyTerminalIDs) asserted at internal/board/df_boardctl_32_test.go:40,115. Tests: `go test -count=1 ./...` exit 0 (all packages ok); the 7 DF-BOARDCTL-32 tests PASS (TestCLI_ByID32_AllTerminalRecycledIDs, _JSONExplainsAllTerminal, _NoRecycledIDsKeepsOldMessage, _MixedGroupPlusTerminalID; TestIDDupe32_AllTerminalRecycledIDsExplained, _AlreadyTerminalIDsOmittedUnlessTerminal, _MixedGroupPlusTerminalID). go build ./... exit 0, go vet exit 0, LSP diagnostics empty.
The dedupe-board --by-id census now correctly reports 'nothing to do — N recycled id(s), all later lines already terminal' for pre-repair boards and exposes already_terminal_ids in --json so recycled_ids is fully explainable, verified by live runs and a green full test suite.

## Summary

Judge Result: DF-BOARDCTL-32

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ On a board with recycled ids whose later lines are ALL already terminal, dedupe-board --by-id dry-run text output must say 'nothing to do — N recycled ids, all later lines already terminal' (not 'no recycled task ids on this board'); --json must carry per-id already_terminal_lines in groups (and/or a top-level field) so 4-1=3 is explainable: Live run on a board with 2 recycled ids whose later lines are all terminal: dry-run text printed exactly 'nothing to do — 2 recycled id(s), all later lines already terminal (no files written)' (exit 0), NOT the old 'no recycled task ids on this board'. Implemented in cmd/dedupe-board/main.go nothingToDoLine(): RecycledIDs==0 keeps the old line verbatim, else emits the new line; called in both dry-run and applied branches. --json emitted {"recycled_ids":2,"groups":null,"already_terminal_ids":["QA-ASCE-1","QA-TB-1"]} — internal/board/iddupe.go adds AlreadyTerminalIDs []string `json:"already_terminal_ids,omitempty"`, populated in DedupeByID's else-branch when len(g.ClosedLines)==0. The 4-1=3 case is explainable: mixed board gave recycled_ids=4, groups=1, already_terminal_ids=['QA-B-1','QA-C-1','QA-D-1'] (4 = 1 + 3), with the invariant recycled_ids == len(Groups)+len(AlreadyTerminalIDs) asserted at internal/board/df_boardctl_32_test.go:40,115. Tests: `go test -count=1 ./...` exit 0 (all packages ok); the 7 DF-BOARDCTL-32 tests PASS (TestCLI_ByID32_AllTerminalRecycledIDs, _JSONExplainsAllTerminal, _NoRecycledIDsKeepsOldMessage, _MixedGroupPlusTerminalID; TestIDDupe32_AllTerminalRecycledIDsExplained, _AlreadyTerminalIDsOmittedUnlessTerminal, _MixedGroupPlusTerminalID). go build ./... exit 0, go vet exit 0, LSP diagnostics empty.
The dedupe-board --by-id census now correctly reports 'nothing to do — N recycled id(s), all later lines already terminal' for pre-repair boards and exposes already_terminal_ids in --json so recycled_ids is fully explainable, verified by live runs and a green full test suite.

Overall: PASS ✓
