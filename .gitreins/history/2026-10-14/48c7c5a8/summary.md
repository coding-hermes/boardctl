# Verdict: GR-150

**Task:** Reject and prevent fused JSONL board records
**Evaluated:** 2026-10-14T00:14:49.340665
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl validate/list/show reject a physical line containing multiple JSON values (fused records), malformed trailing data, and unterminated tails with a file/line diagnostic — never silently skip; validate on a fused fixture exits non-zero, valid legacy boards still pass: Implementation in worktree /home/kara/worktrees/coding-hermes-boardctl-GR-150 (commit 0561f02): internal/board/jsonstyle.go adds ErrMultipleJSONValues/ErrTrailingData sentinels and checkOneValuePerLine() (jsonstyle.go:143-172) which rejects a second JSON value, a truncated second value, and arbitrary trailing bytes, naming the byte offset; ParseRow calls it. Strict reader IterParsed wraps with 'line %d' (board.go:402-419); validate.go:346-362 emits error findings; list/show route through TaskRows->IterParsed (read.go:214-243,270-289). Independent CLI run of the built binary on a fused fixture (two objects on line 2): validate exit=1 with '[error] tasks.jsonl: line 2: multiple JSON values on one physical line (a fused record — JSONL requires exactly one JSON object per line): a second JSON value starts at byte 67'; list exit=1 with the same file:line stderr and zero rows; show B exit=1 same diagnostic; show A exit=1 (not a silent not-found). Malformed trailing data ('{...}oops'): validate exit=1, 'line 2: trailing data after the JSON object ... oops at byte 67'. Unterminated tail ('{...}{"id":"C","title":"gam'): validate exit=1, 'line 2: multiple JSON values ... (a fused, damaged record ...) the value after the first object is incomplete at byte 67: unexpected EOF'. Never silently skip: list --skip-bad-lines exits 1 and prints the salvageable row plus 'SKIPPED-LINES: 1 line(s) ... tasks.jsonl line 2: multiple JSON values ...'. Valid legacy boards still pass: a board whose last line lacks its final newline validates exit=0 'rows: 2 tasks ... RESULT: OK', and the real repo board validates exit=0 'RESULT: OK (6 warning(s))'. Tests: `go test -count=1 -run GR150 ./internal/board/... ./cmd/boardctl/...` -> ok both packages (all 20 GR150 tests PASS); full suite `go test -count=1 ./...` -> all packages ok. Non-vacuous: the same test files copied onto pre-fix revision 64104dc FAIL (internal/board build failed; all 5 CLI tests FAIL).
GR-150 is fully implemented and verified: validate/list/show reject fused, trailing-data, and unterminated-tail lines with file:line diagnostics and exit 1, never silently skip, while valid legacy boards still validate OK.

## Summary

Judge Result: GR-150

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ boardctl validate/list/show reject a physical line containing multiple JSON values (fused records), malformed trailing data, and unterminated tails with a file/line diagnostic — never silently skip; validate on a fused fixture exits non-zero, valid legacy boards still pass: Implementation in worktree /home/kara/worktrees/coding-hermes-boardctl-GR-150 (commit 0561f02): internal/board/jsonstyle.go adds ErrMultipleJSONValues/ErrTrailingData sentinels and checkOneValuePerLine() (jsonstyle.go:143-172) which rejects a second JSON value, a truncated second value, and arbitrary trailing bytes, naming the byte offset; ParseRow calls it. Strict reader IterParsed wraps with 'line %d' (board.go:402-419); validate.go:346-362 emits error findings; list/show route through TaskRows->IterParsed (read.go:214-243,270-289). Independent CLI run of the built binary on a fused fixture (two objects on line 2): validate exit=1 with '[error] tasks.jsonl: line 2: multiple JSON values on one physical line (a fused record — JSONL requires exactly one JSON object per line): a second JSON value starts at byte 67'; list exit=1 with the same file:line stderr and zero rows; show B exit=1 same diagnostic; show A exit=1 (not a silent not-found). Malformed trailing data ('{...}oops'): validate exit=1, 'line 2: trailing data after the JSON object ... oops at byte 67'. Unterminated tail ('{...}{"id":"C","title":"gam'): validate exit=1, 'line 2: multiple JSON values ... (a fused, damaged record ...) the value after the first object is incomplete at byte 67: unexpected EOF'. Never silently skip: list --skip-bad-lines exits 1 and prints the salvageable row plus 'SKIPPED-LINES: 1 line(s) ... tasks.jsonl line 2: multiple JSON values ...'. Valid legacy boards still pass: a board whose last line lacks its final newline validates exit=0 'rows: 2 tasks ... RESULT: OK', and the real repo board validates exit=0 'RESULT: OK (6 warning(s))'. Tests: `go test -count=1 -run GR150 ./internal/board/... ./cmd/boardctl/...` -> ok both packages (all 20 GR150 tests PASS); full suite `go test -count=1 ./...` -> all packages ok. Non-vacuous: the same test files copied onto pre-fix revision 64104dc FAIL (internal/board build failed; all 5 CLI tests FAIL).
GR-150 is fully implemented and verified: validate/list/show reject fused, trailing-data, and unterminated-tail lines with file:line diagnostics and exit 1, never silently skip, while valid legacy boards still validate OK.

Overall: PASS ✓
