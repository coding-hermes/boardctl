# Verdict: README-006

**Task:** README examples omit TR-251 --evidence-run-id
**Evaluated:** 2026-10-19T05:03:51.646619
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Two README examples (lines 100-101 and 958-959) show 'update --status complete' without the TR-251-required --evidence-run-id flag. Fix: append --evidence-run-id <run-id> to both lines. Acceptance: both examples run verbatim on a fresh board and exit 0.: Both examples fixed in commit f6d54e6 ("docs: README-006 — add --evidence-run-id to two update examples (TR-251 compliance)", 2 insertions/2 deletions in README.md). Example 1 at README.md:100-101 now ends `--guard PASS --ci GREEN --summary "retry added (+80/-12)" --evidence-run-id qa-run-1`; example 2 at README.md:974-975 (shifted from 958-959 by the edit) now ends `--worktree /home/me/wt/ft-2 --branch wt/ft-2 --session 91ab77c2 --evidence-run-id qa-run-2`. Acceptance verified by execution, not inspection: `go build -o /tmp/boardctl ./cmd/boardctl` exit 0; on fresh board /tmp/fresh1 (`boardctl init` exit 0, create FEAT-1 exit 0) example 1 verbatim printed "updated task FEAT-1: status, commit_hash, guard_result, ci_result, worker_summary, detail, updated_at" with EXAMPLE1_EXIT=0; on fresh board /tmp/fresh2 (init exit 0, create FEAT-2 exit 0) example 2 verbatim printed "updated task FEAT-2: status, worktree, branch, sessions, detail, updated_at" with EXAMPLE2_EXIT=0. Control run of the pre-fix form (no flag) on a fresh board exits 1 with "evidence run required: completion REFUSED for \"FEAT-1\": --evidence-run-id is REQUIRED when setting status \"complete\" (TR-251)", confirming the flag is genuinely required and is what makes both examples pass. Full suite `go test -count=1 ./...` reported `ok` for all 12 packages (cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, describe, fmtcheck, freshness, render, speccheck, versioncheck, vulncheck, workflowcheck) with zero FAIL lines.
Both README update examples now carry --evidence-run-id and were verified to run verbatim on fresh boards with exit 0, while the pre-fix form is correctly refused with exit 1 under TR-251.

## Summary

Judge Result: README-006

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Two README examples (lines 100-101 and 958-959) show 'update --status complete' without the TR-251-required --evidence-run-id flag. Fix: append --evidence-run-id <run-id> to both lines. Acceptance: both examples run verbatim on a fresh board and exit 0.: Both examples fixed in commit f6d54e6 ("docs: README-006 — add --evidence-run-id to two update examples (TR-251 compliance)", 2 insertions/2 deletions in README.md). Example 1 at README.md:100-101 now ends `--guard PASS --ci GREEN --summary "retry added (+80/-12)" --evidence-run-id qa-run-1`; example 2 at README.md:974-975 (shifted from 958-959 by the edit) now ends `--worktree /home/me/wt/ft-2 --branch wt/ft-2 --session 91ab77c2 --evidence-run-id qa-run-2`. Acceptance verified by execution, not inspection: `go build -o /tmp/boardctl ./cmd/boardctl` exit 0; on fresh board /tmp/fresh1 (`boardctl init` exit 0, create FEAT-1 exit 0) example 1 verbatim printed "updated task FEAT-1: status, commit_hash, guard_result, ci_result, worker_summary, detail, updated_at" with EXAMPLE1_EXIT=0; on fresh board /tmp/fresh2 (init exit 0, create FEAT-2 exit 0) example 2 verbatim printed "updated task FEAT-2: status, worktree, branch, sessions, detail, updated_at" with EXAMPLE2_EXIT=0. Control run of the pre-fix form (no flag) on a fresh board exits 1 with "evidence run required: completion REFUSED for \"FEAT-1\": --evidence-run-id is REQUIRED when setting status \"complete\" (TR-251)", confirming the flag is genuinely required and is what makes both examples pass. Full suite `go test -count=1 ./...` reported `ok` for all 12 packages (cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, describe, fmtcheck, freshness, render, speccheck, versioncheck, vulncheck, workflowcheck) with zero FAIL lines.
Both README update examples now carry --evidence-run-id and were verified to run verbatim on fresh boards with exit 0, while the pre-fix form is correctly refused with exit 1 under TR-251.

Overall: PASS ✓
