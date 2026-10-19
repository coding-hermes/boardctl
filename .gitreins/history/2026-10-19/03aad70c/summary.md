# Verdict: DOC-37

**Task:** Document normalize-board repair-events help
**Evaluated:** 2026-10-19T23:35:20.950205
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Top-level and subcommand help both expose --repair-events; README documents events.jsonl descending-id repair, dry-run default, and --apply; trace row/spec/test/evidence/commit is resolvable.: Verified on branch wt/DOC-37 (worktree /home/kara/worktrees/boardctl-DOC-37, HEAD 9eaf3e5). (a) Top-level help: `go run ./cmd/boardctl --help` prints "normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines]" (cmd/boardctl/main.go:87). (b) Subcommand help: `go run ./cmd/boardctl normalize-board --help` line 1 = "boardctl normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines] [-C dir]" (main.go:1363; flag registered at main.go:1359). (c) README.md:861-887 new section "### `--repair-events`: descending event ids (DF-BOARDCTL-38)" documents events.jsonl descending-id renumber to running max+1 with other fields untouched, "dry-run by default: no file is written, and the would-be bytes are staged at `<boarddir>/events.jsonl.renumbered.jsonl`", the `--apply` rewrite arm, and runnable examples `normalize-board --repair-events` / `--repair-events --apply`. (d) Trace resolvable: board row .gitreins/tasks.yaml:905 (DOC-37, status complete), commit 9eaf3e5 ("docs: document normalize-board event repair (DOC-37)" / "Addresses DOC-37."), regression test cmd/boardctl/doc37_repair_events_help_test.go, spec/evidence DF-BOARDCTL-38 in internal/board/repair_events.go + repair_events_test.go. Test evidence: `go test -count=1 -v -run TestDoc37 ./cmd/boardctl/` => PASS TestDoc37RootUsageNamesRepairEvents, PASS TestDoc37SubcommandHelpNamesRepairEvents, PASS TestDoc37ReadmeDocumentsRepairEvents, "ok github.com/coding-hermes/boardctl/cmd/boardctl 0.003s", exit 0; full `go test -count=1 ./...` => all 12 packages "ok" (cmd/boardctl 6.129s), exit 0. Note: the deliverable exists only on wt/DOC-37 and is not merged to main (main working tree has zero `repair-events` hits in main.go/README).
Both help surfaces expose --repair-events, README documents the events.jsonl descending-id repair with dry-run default and --apply, and the trace (board row, commit 9eaf3e5, doc37 test, DF-BOARDCTL-38 evidence) resolves with all tests green — though the work is only on branch wt/DOC-37, not merged to main.

## Summary

Judge Result: DOC-37

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Top-level and subcommand help both expose --repair-events; README documents events.jsonl descending-id repair, dry-run default, and --apply; trace row/spec/test/evidence/commit is resolvable.: Verified on branch wt/DOC-37 (worktree /home/kara/worktrees/boardctl-DOC-37, HEAD 9eaf3e5). (a) Top-level help: `go run ./cmd/boardctl --help` prints "normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines]" (cmd/boardctl/main.go:87). (b) Subcommand help: `go run ./cmd/boardctl normalize-board --help` line 1 = "boardctl normalize-board [--apply] [--repair-events] [--json] [--skip-bad-lines] [-C dir]" (main.go:1363; flag registered at main.go:1359). (c) README.md:861-887 new section "### `--repair-events`: descending event ids (DF-BOARDCTL-38)" documents events.jsonl descending-id renumber to running max+1 with other fields untouched, "dry-run by default: no file is written, and the would-be bytes are staged at `<boarddir>/events.jsonl.renumbered.jsonl`", the `--apply` rewrite arm, and runnable examples `normalize-board --repair-events` / `--repair-events --apply`. (d) Trace resolvable: board row .gitreins/tasks.yaml:905 (DOC-37, status complete), commit 9eaf3e5 ("docs: document normalize-board event repair (DOC-37)" / "Addresses DOC-37."), regression test cmd/boardctl/doc37_repair_events_help_test.go, spec/evidence DF-BOARDCTL-38 in internal/board/repair_events.go + repair_events_test.go. Test evidence: `go test -count=1 -v -run TestDoc37 ./cmd/boardctl/` => PASS TestDoc37RootUsageNamesRepairEvents, PASS TestDoc37SubcommandHelpNamesRepairEvents, PASS TestDoc37ReadmeDocumentsRepairEvents, "ok github.com/coding-hermes/boardctl/cmd/boardctl 0.003s", exit 0; full `go test -count=1 ./...` => all 12 packages "ok" (cmd/boardctl 6.129s), exit 0. Note: the deliverable exists only on wt/DOC-37 and is not merged to main (main working tree has zero `repair-events` hits in main.go/README).
Both help surfaces expose --repair-events, README documents the events.jsonl descending-id repair with dry-run default and --apply, and the trace (board row, commit 9eaf3e5, doc37 test, DF-BOARDCTL-38 evidence) resolves with all tests green — though the work is only on branch wt/DOC-37, not merged to main.

Overall: PASS ✓
