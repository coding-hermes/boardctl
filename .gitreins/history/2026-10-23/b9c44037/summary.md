# Verdict: DF-BOARDCTL-46

**Task:** [P3] normalize-board hard-fails on ONE unparseable line and names no remedy — and --skip-bad-lines on the same command is a dead end (refuses to write), so 400+ rows of drift stay unrepaired
**Evaluated:** 2026-10-23T20:57:19.451077
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ 1) The unparseable-line refusal from normalize-board names the remedy inline (line number + `validate --repair` → review tasks.rewritten.jsonl → swap → re-run normalize-board). 2) Either `--skip-bad-lines` on normalize-board becomes a usable degraded path that writes the repaired tasks.jsonl and preserves the dropped lines in a sidecar (e.g. tasks.skipped-lines.jsonl) with an explicit exit code, or the flag is removed from that command's help and the help points at validate --repair. 3) A fixture reproducing the duplicate-key line asserts the remedy text (grep for 'validate --repair' in the error output). 4) `go test ./...` exits 0.: (1) internal/board/normalizeboard.go:189 defines NormalizeBoardRepairHint and :323 wraps the parse error as fmt.Errorf("%s: %w — %s", b.tasksPath, err, NormalizeBoardRepairHint). Live CLI run on a duplicate-key fixture printed: `boardctl: /tmp/.../tasks.jsonl: line 2: duplicate key "status" in row — remedy: run 'boardctl validate --repair' (salvages every parseable row into tasks.rewritten.jsonl), review tasks.rewritten.jsonl, swap the reviewed file into tasks.jsonl, then re-run 'boardctl normalize-board'` with EXIT=1 — file + 1-based line number + all four ordered steps. (2) Flag removed: cmd/boardctl/main.go:1363-1368 registers only --apply/--json/--repair-events/-C (no useSkipBad); live `normalize-board --skip-bad-lines` -> `flag provided but not defined: -skip-bad-lines` EXIT=1. Help at main.go:1380-1381 prints 'an UNPARSEABLE tasks.jsonl line REFUSES the whole command ... this repair never degrades. The remedy is printed on the refusal itself:' followed by NormalizeBoardRepairHint (validate --repair path); top-level usageText main.go:87 lists no --skip-bad-lines for normalize-board. (3) internal/board/df_boardctl_46_test.go:29 df46DupKeyRow is the duplicate-key line; TestDF46NormalizeBoardRefusalCarriesRemedy asserts strings.Contains(err.Error(), "validate --repair") (line 105) plus tasks.rewritten.jsonl/swap/re-run (line 74), and TestDF46RepairHintNamesEveryStep (line 117) pins every step; `go test -count=1 -run DF46 -v ./internal/board/` -> 3x PASS, ok github.com/coding-hermes/boardctl/internal/board. (4) `go test -count=1 ./...` -> all 12 packages `ok`, EXIT=0.
normalize-board now fails closed with an inline, ordered validate --repair remedy naming the file and line, the dead-end --skip-bad-lines flag is removed from the command and its help points at validate --repair, a duplicate-key fixture asserts the remedy text, and the full suite passes (exit 0).

## Summary

Judge Result: DF-BOARDCTL-46

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ 1) The unparseable-line refusal from normalize-board names the remedy inline (line number + `validate --repair` → review tasks.rewritten.jsonl → swap → re-run normalize-board). 2) Either `--skip-bad-lines` on normalize-board becomes a usable degraded path that writes the repaired tasks.jsonl and preserves the dropped lines in a sidecar (e.g. tasks.skipped-lines.jsonl) with an explicit exit code, or the flag is removed from that command's help and the help points at validate --repair. 3) A fixture reproducing the duplicate-key line asserts the remedy text (grep for 'validate --repair' in the error output). 4) `go test ./...` exits 0.: (1) internal/board/normalizeboard.go:189 defines NormalizeBoardRepairHint and :323 wraps the parse error as fmt.Errorf("%s: %w — %s", b.tasksPath, err, NormalizeBoardRepairHint). Live CLI run on a duplicate-key fixture printed: `boardctl: /tmp/.../tasks.jsonl: line 2: duplicate key "status" in row — remedy: run 'boardctl validate --repair' (salvages every parseable row into tasks.rewritten.jsonl), review tasks.rewritten.jsonl, swap the reviewed file into tasks.jsonl, then re-run 'boardctl normalize-board'` with EXIT=1 — file + 1-based line number + all four ordered steps. (2) Flag removed: cmd/boardctl/main.go:1363-1368 registers only --apply/--json/--repair-events/-C (no useSkipBad); live `normalize-board --skip-bad-lines` -> `flag provided but not defined: -skip-bad-lines` EXIT=1. Help at main.go:1380-1381 prints 'an UNPARSEABLE tasks.jsonl line REFUSES the whole command ... this repair never degrades. The remedy is printed on the refusal itself:' followed by NormalizeBoardRepairHint (validate --repair path); top-level usageText main.go:87 lists no --skip-bad-lines for normalize-board. (3) internal/board/df_boardctl_46_test.go:29 df46DupKeyRow is the duplicate-key line; TestDF46NormalizeBoardRefusalCarriesRemedy asserts strings.Contains(err.Error(), "validate --repair") (line 105) plus tasks.rewritten.jsonl/swap/re-run (line 74), and TestDF46RepairHintNamesEveryStep (line 117) pins every step; `go test -count=1 -run DF46 -v ./internal/board/` -> 3x PASS, ok github.com/coding-hermes/boardctl/internal/board. (4) `go test -count=1 ./...` -> all 12 packages `ok`, EXIT=0.
normalize-board now fails closed with an inline, ordered validate --repair remedy naming the file and line, the dead-end --skip-bad-lines flag is removed from the command and its help points at validate --repair, a duplicate-key fixture asserts the remedy text, and the full suite passes (exit 0).

Overall: PASS ✓
