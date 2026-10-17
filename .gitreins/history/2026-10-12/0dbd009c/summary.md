# Verdict: DOC-31

**Task:** document dedupe-board --by-id and --json flag
**Evaluated:** 2026-10-12T04:09:27.744918
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README dedupe-board section documents the default fingerprint mode but not the --by-id recycled-id repair mode (DF-BOARDCTL-10) or the --json flag; add a subsection or example showing both: README.md:832-852 (commit 9bfb257) adds a '**Recycled-id repair (`--by-id`)**' subsection with three bash examples (dry-run, --apply, --json) plus prose covering earliest-line-kept, superseded-by-earliest closure, terminal lines untouched, and '--json emits the report as JSON (only supported with --by-id)'. All claims match the implementation: cmd/dedupe-board/main.go:75 (--by-id flag), :77 (--json flag), :124-125 (--json rejected without --by-id), internal/board/iddupe.go:189/287 (superseded-by-earliest), :155 (AlreadyTerminal left untouched), :225-247 (audit event preserves original title+reasoning). Tests: `go test -count=1 ./...` exit_code 0, 12 packages ok, no FAIL/panic.
README dedupe-board section now documents both the --by-id recycled-id repair mode and the --json flag with examples, matching the verified implementation and passing test suite.

## Summary

Judge Result: DOC-31

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README dedupe-board section documents the default fingerprint mode but not the --by-id recycled-id repair mode (DF-BOARDCTL-10) or the --json flag; add a subsection or example showing both: README.md:832-852 (commit 9bfb257) adds a '**Recycled-id repair (`--by-id`)**' subsection with three bash examples (dry-run, --apply, --json) plus prose covering earliest-line-kept, superseded-by-earliest closure, terminal lines untouched, and '--json emits the report as JSON (only supported with --by-id)'. All claims match the implementation: cmd/dedupe-board/main.go:75 (--by-id flag), :77 (--json flag), :124-125 (--json rejected without --by-id), internal/board/iddupe.go:189/287 (superseded-by-earliest), :155 (AlreadyTerminal left untouched), :225-247 (audit event preserves original title+reasoning). Tests: `go test -count=1 ./...` exit_code 0, 12 packages ok, no FAIL/panic.
README dedupe-board section now documents both the --by-id recycled-id repair mode and the --json flag with examples, matching the verified implementation and passing test suite.

Overall: PASS ✓
