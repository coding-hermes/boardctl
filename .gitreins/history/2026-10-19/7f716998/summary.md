# Verdict: DOC-34

**Task:** Document update --replace note semantics
**Evaluated:** 2026-10-19T21:44:54.056092
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ DOC-34 implementation commit is 58e11d2b4d50a73e6444c3e0574cebf142d33322, merged at HEAD. Verify README.md documents all three update note behaviors: --note appends to an existing foreman_note with a blank line, --replace --note overwrites foreman_note, and omitting --note leaves foreman_note untouched; verify README contains runnable examples for both --note and --replace --note; verify the exact trace marker is present in the board row closeout or completion event for DOC-34 and resolves README.md plus commit 58e11d2b4d50a73e6444c3e0574cebf142d33322. The acceptance evidence is README.md and the focused regression tests cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends and TestCmdUpdateNoteReplaceFlagOverwrites. Do not require a new source test because existing tests already cover the behavior.: Commit 58e11d2b4d50a73e6444c3e0574cebf142d33322 exists and `git merge-base --is-ancestor 58e11d2 HEAD` succeeds; HEAD is 75dfb45 "Merge branch 'wt/DOC-34'". README.md:109-120 documents all three behaviors: (1) lines 109-112 "--note targets foreman_note and APPENDS to a non-empty existing note (existing note + blank line + new note)"; (2) lines 115-117 "--replace is a flag (takes no value) used together with --note: it OVERWRITES foreman_note instead of appending"; (3) lines 119-120 "Omitting --note leaves foreman_note untouched". Runnable examples present: README.md:113 `boardctl -C ~/myproject update FEAT-1 --note "Verified on the staging board"`, README.md:114 second-append form, README.md:118 `boardctl -C ~/myproject update FEAT-1 --replace --note "Replacement note"`. Trace marker present in the DOC-34 completion event at .coding-hermes/board/events.jsonl:483 (event id 480, event_type task_completed, task_id DOC-34, 2026-10-19 21:44:06): "ch:trace row=DOC-34 spec=README.md#update-options ... doc=README.md ... evidence=README.md ... commit=58e11d2b4d50a73e6444c3e0574cebf142d33322" — resolves README.md plus the exact commit. Tests run fresh: `go test -count=1 -run 'TestCmdUpdateNoteAppends|TestCmdUpdateNoteReplaceFlagOverwrites' -v ./cmd/boardctl/` exit_code 0, output `--- PASS: TestCmdUpdateNoteAppends (0.00s)`, `--- PASS: TestCmdUpdateNoteReplaceFlagOverwrites (0.00s)`, `ok github.com/coding-hermes/boardctl/cmd/boardctl 0.009s`. Test bodies at cmd/boardctl/bt068_cli_test.go:74 assert foreman_note == "A\n\nB" (blank-line append) and :115 assert "Z" (overwrite).
Commit 58e11d2 is merged at HEAD, README.md documents all three note behaviors with runnable --note and --replace --note examples, the ch:trace marker in the DOC-34 task_completed event resolves README.md plus the exact commit, and both focused regression tests pass.

## Summary

Judge Result: DOC-34

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ DOC-34 implementation commit is 58e11d2b4d50a73e6444c3e0574cebf142d33322, merged at HEAD. Verify README.md documents all three update note behaviors: --note appends to an existing foreman_note with a blank line, --replace --note overwrites foreman_note, and omitting --note leaves foreman_note untouched; verify README contains runnable examples for both --note and --replace --note; verify the exact trace marker is present in the board row closeout or completion event for DOC-34 and resolves README.md plus commit 58e11d2b4d50a73e6444c3e0574cebf142d33322. The acceptance evidence is README.md and the focused regression tests cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends and TestCmdUpdateNoteReplaceFlagOverwrites. Do not require a new source test because existing tests already cover the behavior.: Commit 58e11d2b4d50a73e6444c3e0574cebf142d33322 exists and `git merge-base --is-ancestor 58e11d2 HEAD` succeeds; HEAD is 75dfb45 "Merge branch 'wt/DOC-34'". README.md:109-120 documents all three behaviors: (1) lines 109-112 "--note targets foreman_note and APPENDS to a non-empty existing note (existing note + blank line + new note)"; (2) lines 115-117 "--replace is a flag (takes no value) used together with --note: it OVERWRITES foreman_note instead of appending"; (3) lines 119-120 "Omitting --note leaves foreman_note untouched". Runnable examples present: README.md:113 `boardctl -C ~/myproject update FEAT-1 --note "Verified on the staging board"`, README.md:114 second-append form, README.md:118 `boardctl -C ~/myproject update FEAT-1 --replace --note "Replacement note"`. Trace marker present in the DOC-34 completion event at .coding-hermes/board/events.jsonl:483 (event id 480, event_type task_completed, task_id DOC-34, 2026-10-19 21:44:06): "ch:trace row=DOC-34 spec=README.md#update-options ... doc=README.md ... evidence=README.md ... commit=58e11d2b4d50a73e6444c3e0574cebf142d33322" — resolves README.md plus the exact commit. Tests run fresh: `go test -count=1 -run 'TestCmdUpdateNoteAppends|TestCmdUpdateNoteReplaceFlagOverwrites' -v ./cmd/boardctl/` exit_code 0, output `--- PASS: TestCmdUpdateNoteAppends (0.00s)`, `--- PASS: TestCmdUpdateNoteReplaceFlagOverwrites (0.00s)`, `ok github.com/coding-hermes/boardctl/cmd/boardctl 0.009s`. Test bodies at cmd/boardctl/bt068_cli_test.go:74 assert foreman_note == "A\n\nB" (blank-line append) and :115 assert "Z" (overwrite).
Commit 58e11d2 is merged at HEAD, README.md documents all three note behaviors with runnable --note and --replace --note examples, the ch:trace marker in the DOC-34 task_completed event resolves README.md plus the exact commit, and both focused regression tests pass.

Overall: FAIL ✗
