# Verdict: BT-68

**Task:** boardctl update --note must not silently destroy the existing foreman_note
**Evaluated:** 2026-09-27T17:40:43.389182
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.413s
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl update <id> --note APPENDS to foreman_note with an explicit separator by default; replacing requires an explicit --replace; the update usage closure documents both; a regression test proves append semantics and that --replace still replaces; the full go build/vet/test -count=1 suite is green.: Append default: internal/board/write.go:788-806 — `if spec.Note != nil { noteVal := *spec.Note; if !spec.NoteReplace { if existing := target.String("foreman_note"); existing != "" { noteVal = existing + "\n\n" + noteVal } }; set("foreman_note", noteVal) }` (explicit "\n\n" separator). --replace: UpdateSpec.NoteReplace bool at write.go:630; CLI `replace := fs.Bool("replace", false, ...)` at cmd/boardctl/main.go:692, wired via NoteReplace: *replace at main.go:778. Usage closure documents both: main.go:687 note help "foreman_note (appends to the existing note; --replace overwrites)", main.go:692 replace help, main.go:717 usage line "[--replace]", and top-level usage main.go:59 "[--note S (appends)] ... [--replace]". Regression tests: internal/board/bt068_note_append_test.go TestUpdateNoteAppendsToExisting (expects "A\n\nB"), TestUpdateNoteAppendsTwice ("A\n\nB\n\nC"), TestUpdateNoteReplaceOverwrites (expects "Z"), TestUpdateNoteFirstWriteNoSeparator, TestUpdateNoteOmittedLeavesKeyUntouched, TestUpdateNoteAppendPreservesEveryOtherLine; cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends, TestCmdUpdateNoteReplaceFlagOverwrites, TestCmdUpdateReplaceIsNotAValueFlag, TestCmdUpdateNoteOmittedLeavesNoteUntouched, TestCmdUpdateHelpListsReplace. Suite green (fresh runs): `go build ./...` exit 0; `go vet ./...` exit 0 (VET_EXIT=0); `go test -count=1 ./...` exit 0 (TEST_EXIT=0) with all packages "ok" (cmd/boardctl 3.442s, internal/board 0.411s); targeted -v run shows all 11 BT-68 tests --- PASS.
BT-68 fully implemented: --note appends with "\n\n" separator by default, --replace restores overwrite, both documented in the update usage closure, regression tests cover append and replace semantics, and build/vet/test -count=1 are all green.

## Summary

Judge Result: BT-68

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.413s

Stage tier2: PASS
  COMPLETE
  ✓ boardctl update <id> --note APPENDS to foreman_note with an explicit separator by default; replacing requires an explicit --replace; the update usage closure documents both; a regression test proves append semantics and that --replace still replaces; the full go build/vet/test -count=1 suite is green.: Append default: internal/board/write.go:788-806 — `if spec.Note != nil { noteVal := *spec.Note; if !spec.NoteReplace { if existing := target.String("foreman_note"); existing != "" { noteVal = existing + "\n\n" + noteVal } }; set("foreman_note", noteVal) }` (explicit "\n\n" separator). --replace: UpdateSpec.NoteReplace bool at write.go:630; CLI `replace := fs.Bool("replace", false, ...)` at cmd/boardctl/main.go:692, wired via NoteReplace: *replace at main.go:778. Usage closure documents both: main.go:687 note help "foreman_note (appends to the existing note; --replace overwrites)", main.go:692 replace help, main.go:717 usage line "[--replace]", and top-level usage main.go:59 "[--note S (appends)] ... [--replace]". Regression tests: internal/board/bt068_note_append_test.go TestUpdateNoteAppendsToExisting (expects "A\n\nB"), TestUpdateNoteAppendsTwice ("A\n\nB\n\nC"), TestUpdateNoteReplaceOverwrites (expects "Z"), TestUpdateNoteFirstWriteNoSeparator, TestUpdateNoteOmittedLeavesKeyUntouched, TestUpdateNoteAppendPreservesEveryOtherLine; cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends, TestCmdUpdateNoteReplaceFlagOverwrites, TestCmdUpdateReplaceIsNotAValueFlag, TestCmdUpdateNoteOmittedLeavesNoteUntouched, TestCmdUpdateHelpListsReplace. Suite green (fresh runs): `go build ./...` exit 0; `go vet ./...` exit 0 (VET_EXIT=0); `go test -count=1 ./...` exit 0 (TEST_EXIT=0) with all packages "ok" (cmd/boardctl 3.442s, internal/board 0.411s); targeted -v run shows all 11 BT-68 tests --- PASS.
BT-68 fully implemented: --note appends with "\n\n" separator by default, --replace restores overwrite, both documented in the update usage closure, regression tests cover append and replace semantics, and build/vet/test -count=1 are all green.

Overall: PASS ✓
