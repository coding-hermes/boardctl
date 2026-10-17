# Verdict: DOC-31

**Task:** document dedupe-board --by-id and --json flag
**Evaluated:** 2026-10-12T04:07:58.854038
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ README dedupe-board section documents the default fingerprint mode but not the --by-id recycled-id repair mode (DF-BOARDCTL-10) or the --json flag; add a subsection or example showing both: No documentation was added. README.md:820-850 (the dedupe-board section) still shows only the default fingerprint mode (`dedupe-board --board-dir ~/myproject` / `--apply`, lines 828-829) plus the source-only install note (840-850). `grep -n "by-id" README.md` -> exit 1 (ZERO matches); `grep -n "by-id" docs/board-fingerprint-rules.md` -> exit 1 (ZERO matches); no `--json` in the dedupe-board section (README.md:80/634/933 --json hits are version/sweep-status/render). The flags exist in code (cmd/dedupe-board/main.go:75 --by-id, :77 --json, :101 usage, :124-125 --json requires --by-id, :206 runByID) but are undocumented. No commit anywhere adds the docs: `git log --all --grep="by-id"` returns only c4d56a0 (the audit that FILED DOC-31) and 1ff31cc (the feature commit, which touched only cmd/dedupe-board/main.go, main_test.go, internal/board/iddupe.go, iddupe_test.go — no README). Working tree is clean, no stash, no uncommitted README diff; board row .coding-hermes/board/tasks.jsonl:158 is still status=pending/worker_status=pending with commit_hash=null. Guard log .gitreins/logs/guard-20261012T040647.853681Z.log shows go_build/go_lint/go_tests SKIPPED ("No Go files staged"), i.e. no change was staged at all.
The README dedupe-board section still documents only the default fingerprint mode — neither --by-id (DF-BOARDCTL-10) nor --json appears anywhere in README.md or docs/board-fingerprint-rules.md, and no commit or working-tree change adds them.

## Summary

Judge Result: DOC-31

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ README dedupe-board section documents the default fingerprint mode but not the --by-id recycled-id repair mode (DF-BOARDCTL-10) or the --json flag; add a subsection or example showing both: No documentation was added. README.md:820-850 (the dedupe-board section) still shows only the default fingerprint mode (`dedupe-board --board-dir ~/myproject` / `--apply`, lines 828-829) plus the source-only install note (840-850). `grep -n "by-id" README.md` -> exit 1 (ZERO matches); `grep -n "by-id" docs/board-fingerprint-rules.md` -> exit 1 (ZERO matches); no `--json` in the dedupe-board section (README.md:80/634/933 --json hits are version/sweep-status/render). The flags exist in code (cmd/dedupe-board/main.go:75 --by-id, :77 --json, :101 usage, :124-125 --json requires --by-id, :206 runByID) but are undocumented. No commit anywhere adds the docs: `git log --all --grep="by-id"` returns only c4d56a0 (the audit that FILED DOC-31) and 1ff31cc (the feature commit, which touched only cmd/dedupe-board/main.go, main_test.go, internal/board/iddupe.go, iddupe_test.go — no README). Working tree is clean, no stash, no uncommitted README diff; board row .coding-hermes/board/tasks.jsonl:158 is still status=pending/worker_status=pending with commit_hash=null. Guard log .gitreins/logs/guard-20261012T040647.853681Z.log shows go_build/go_lint/go_tests SKIPPED ("No Go files staged"), i.e. no change was staged at all.
The README dedupe-board section still documents only the default fingerprint mode — neither --by-id (DF-BOARDCTL-10) nor --json appears anywhere in README.md or docs/board-fingerprint-rules.md, and no commit or working-tree change adds them.

Overall: FAIL ✗
