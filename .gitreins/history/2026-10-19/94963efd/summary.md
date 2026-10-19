# Verdict: DOC-37

**Task:** Document normalize-board repair-events help
**Evaluated:** 2026-10-19T23:32:33.111592
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Top-level and subcommand help both expose --repair-events; README documents events.jsonl descending-id repair, dry-run default, and --apply; trace row/spec/test/evidence/commit is resolvable.: Work is committed on branch wt/DOC-37 at 9eaf3e5 ("docs: document normalize-board event repair (DOC-37)"; not yet merged to main 8c659d5). On that tree: (a) top-level help — `go run ./cmd/boardctl --help` line 45 = "normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines]", from the usageText edit at cmd/boardctl/main.go:87 (diff `-  normalize-board [--apply] [--json]...` / `+  normalize-board [--apply] [--repair-events]...`); (b) subcommand help — `go run ./cmd/boardctl normalize-board --help` line 1 = "boardctl normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines] [-C dir]" (closure at main.go:1363); (c) README — new section "### `--repair-events`: descending event ids (DF-BOARDCTL-38)" at 9eaf3e5:README.md:861-887 documents events.jsonl descending-id renumber to running max+1, "dry-run by default: no file is written, and the would-be bytes are staged at `<boarddir>/events.jsonl.renumbered.jsonl`", the `--apply` rewrite arm, and runnable examples `normalize-board --repair-events` / `--repair-events --apply`; (d) trace resolvable — board row .gitreins/tasks.yaml:905 (DOC-37, status complete) + commit 9eaf3e5 ("Addresses DOC-37.") + regression test cmd/boardctl/doc37_repair_events_help_test.go + spec/evidence DF-BOARDCTL-38 (internal/board/repair_events.go, repair_events_test.go). Test evidence: `go test -count=1 -v -run TestDoc37 ./cmd/boardctl/` => PASS TestDoc37RootUsageNamesRepairEvents, PASS TestDoc37SubcommandHelpNamesRepairEvents, PASS TestDoc37ReadmeDocumentsRepairEvents, exit 0; full `go test -count=1 ./...` => all 12 packages "ok" (cmd/boardctl 17.577s), exit 0. Note: the current main working tree still lacks all three changes (main.go:87 has no --repair-events, `grep -rn repair-events README.md docs/` = 0 hits, no doc37 test file) — the deliverable exists only on wt/DOC-37 and is unmerged.
All three surfaces (top-level help, subcommand help, README with events.jsonl descending-id repair + dry-run default + --apply) plus a resolvable trace and passing regression tests exist in commit 9eaf3e5 on wt/DOC-37, though that commit is not yet merged into main.

## Summary

Judge Result: DOC-37

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Top-level and subcommand help both expose --repair-events; README documents events.jsonl descending-id repair, dry-run default, and --apply; trace row/spec/test/evidence/commit is resolvable.: Work is committed on branch wt/DOC-37 at 9eaf3e5 ("docs: document normalize-board event repair (DOC-37)"; not yet merged to main 8c659d5). On that tree: (a) top-level help — `go run ./cmd/boardctl --help` line 45 = "normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines]", from the usageText edit at cmd/boardctl/main.go:87 (diff `-  normalize-board [--apply] [--json]...` / `+  normalize-board [--apply] [--repair-events]...`); (b) subcommand help — `go run ./cmd/boardctl normalize-board --help` line 1 = "boardctl normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines] [-C dir]" (closure at main.go:1363); (c) README — new section "### `--repair-events`: descending event ids (DF-BOARDCTL-38)" at 9eaf3e5:README.md:861-887 documents events.jsonl descending-id renumber to running max+1, "dry-run by default: no file is written, and the would-be bytes are staged at `<boarddir>/events.jsonl.renumbered.jsonl`", the `--apply` rewrite arm, and runnable examples `normalize-board --repair-events` / `--repair-events --apply`; (d) trace resolvable — board row .gitreins/tasks.yaml:905 (DOC-37, status complete) + commit 9eaf3e5 ("Addresses DOC-37.") + regression test cmd/boardctl/doc37_repair_events_help_test.go + spec/evidence DF-BOARDCTL-38 (internal/board/repair_events.go, repair_events_test.go). Test evidence: `go test -count=1 -v -run TestDoc37 ./cmd/boardctl/` => PASS TestDoc37RootUsageNamesRepairEvents, PASS TestDoc37SubcommandHelpNamesRepairEvents, PASS TestDoc37ReadmeDocumentsRepairEvents, exit 0; full `go test -count=1 ./...` => all 12 packages "ok" (cmd/boardctl 17.577s), exit 0. Note: the current main working tree still lacks all three changes (main.go:87 has no --repair-events, `grep -rn repair-events README.md docs/` = 0 hits, no doc37 test file) — the deliverable exists only on wt/DOC-37 and is unmerged.
All three surfaces (top-level help, subcommand help, README with events.jsonl descending-id repair + dry-run default + --apply) plus a resolvable trace and passing regression tests exist in commit 9eaf3e5 on wt/DOC-37, though that commit is not yet merged into main.

Overall: PASS ✓
