# Verdict: BT-68

**Task:** boardctl update --note must not silently destroy the existing foreman_note
**Evaluated:** 2026-09-27T17:45:05.129552
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.272s
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl update <id> --note APPENDS to foreman_note with an explicit separator by default; replacing requires an explicit --replace; the update usage closure documents both; a regression test proves append semantics and that --replace still replaces; the full go build/vet/test -count=1 suite is green.: Append default: internal/board/write.go:789-806 — `noteVal := *spec.Note; if !spec.NoteReplace { if existing := target.String("foreman_note"); existing != "" { noteVal = existing + "\n\n" + noteVal } }` then set("foreman_note", noteVal) (explicit "\n\n" separator). --replace: cmd/boardctl/main.go:692 `replace := fs.Bool("replace", false, "with --note: overwrite foreman_note instead of appending")`, wired at main.go:778 `NoteReplace: *replace`; UpdateSpec.NoteReplace documented at write.go:622-631. Usage closure documents both: main.go:714-719 prints "[--note S (appends)]" and "[--replace]"; top-level usageText main.go:57-59 likewise. Regression tests: internal/board/bt068_note_append_test.go TestUpdateNoteAppendsToExisting (asserts "A\n\nB"), TestUpdateNoteAppendsTwice ("A\n\nB\n\nC"), TestUpdateNoteFirstWriteNoSeparator, TestUpdateNoteReplaceOverwrites (asserts "Z"), TestUpdateNoteOmittedLeavesKeyUntouched, TestUpdateNoteAppendPreservesEveryOtherLine; cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends, TestCmdUpdateNoteReplaceFlagOverwrites, TestCmdUpdateNoteOmittedLeavesNoteUntouched, TestCmdUpdateHelpListsReplace. Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 — all packages ok (cmd/boardctl ok 2.077s, internal/board ok 0.293s, internal/render ok, etc.); targeted `go test -count=1 -run 'BT068|NoteAppend|NoteReplace|UpdateNote|HelpListsReplace'` shows all 10 BT-68 tests --- PASS.
BT-68 is fully implemented: --note appends with an explicit "\n\n" separator by default, --replace restores overwrite, both usage surfaces document the flags, regression tests cover append and replace semantics, and build/vet/test -count=1 are all green.

## Summary

Judge Result: BT-68

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.272s

Stage tier2: PASS
  COMPLETE
  ✓ boardctl update <id> --note APPENDS to foreman_note with an explicit separator by default; replacing requires an explicit --replace; the update usage closure documents both; a regression test proves append semantics and that --replace still replaces; the full go build/vet/test -count=1 suite is green.: Append default: internal/board/write.go:789-806 — `noteVal := *spec.Note; if !spec.NoteReplace { if existing := target.String("foreman_note"); existing != "" { noteVal = existing + "\n\n" + noteVal } }` then set("foreman_note", noteVal) (explicit "\n\n" separator). --replace: cmd/boardctl/main.go:692 `replace := fs.Bool("replace", false, "with --note: overwrite foreman_note instead of appending")`, wired at main.go:778 `NoteReplace: *replace`; UpdateSpec.NoteReplace documented at write.go:622-631. Usage closure documents both: main.go:714-719 prints "[--note S (appends)]" and "[--replace]"; top-level usageText main.go:57-59 likewise. Regression tests: internal/board/bt068_note_append_test.go TestUpdateNoteAppendsToExisting (asserts "A\n\nB"), TestUpdateNoteAppendsTwice ("A\n\nB\n\nC"), TestUpdateNoteFirstWriteNoSeparator, TestUpdateNoteReplaceOverwrites (asserts "Z"), TestUpdateNoteOmittedLeavesKeyUntouched, TestUpdateNoteAppendPreservesEveryOtherLine; cmd/boardctl/bt068_cli_test.go TestCmdUpdateNoteAppends, TestCmdUpdateNoteReplaceFlagOverwrites, TestCmdUpdateNoteOmittedLeavesNoteUntouched, TestCmdUpdateHelpListsReplace. Command evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 — all packages ok (cmd/boardctl ok 2.077s, internal/board ok 0.293s, internal/render ok, etc.); targeted `go test -count=1 -run 'BT068|NoteAppend|NoteReplace|UpdateNote|HelpListsReplace'` shows all 10 BT-68 tests --- PASS.
BT-68 is fully implemented: --note appends with an explicit "\n\n" separator by default, --replace restores overwrite, both usage surfaces document the flags, regression tests cover append and replace semantics, and build/vet/test -count=1 are all green.

Overall: PASS ✓
