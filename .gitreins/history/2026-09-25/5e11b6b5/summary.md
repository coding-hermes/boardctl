# Verdict: BT-060

**Task:** CLI flag documentation closure: update usage text, --title on update, validate title-priority warning
**Evaluated:** 2026-09-25T22:27:36.486407
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	10.643s
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl update -h documents all twelve value flags (worker-status, commit-hash, guard, ci, summary, note, blocked-reason, completed-at, worktree, branch, session, title); root help create/update lines agree with their own usage closures; boardctl update accepts --title and rewrites the row title in place; validate exits 0 with a warning when a title carries a Pn token that disagrees with the priority field (BT-025 warning pattern); go build/vet/test -count=1 green: (1) `go run ./cmd/boardctl update -h` prints all twelve value flags: --title, --worker-status, --commit-hash, --guard, --ci, --summary, --note, --blocked-reason, --completed-at, --worktree, --branch, --session (cmd/boardctl/main.go:700-704 usage closure; flags registered main.go:676-696); TestCmdUpdateHelpListsEveryValueFlag PASS. (2) Root usageText (main.go:54-62) create line now lists [--worktree PATH] [--branch NAME] [--session ID]... and update line lists [--title T] ... [--worktree] [--branch] [--session]; TestRootUsageTextCreateUpdateMatchClosures PASS (flag-for-flag both directions). (3) Live run `update BT-900 --title "[P1] renamed"` exit 0: row title rewritten in place, priority P2 preserved, task_updated event appended (write.go:722-727 set("title"), write.go:844-855 event); TestUpdateTitleRewritesRow + TestCmdUpdateTitleRewritesAndAudits PASS. (4) Live `validate` printed `[warn] tasks.jsonl line 2 (task BT-900): title carries "P1" but priority is "P2" ...` with `RESULT: OK (1 warning(s))` and VALIDATE_EXIT=0 — warn severity, BT-025 pattern (validate.go:187-203); TestBT060TitlePriorityMismatchWarns/SilentCases/MultipleTokens PASS. (5) `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` all packages ok (cmd/boardctl ok 3.125s, internal/board ok 1.078s, no FAIL).
All BT-060 sub-criteria verified: twelve update flags documented, root help agrees with closures, --title rewrites rows in place, validate warns (exit 0) on title/priority Pn drift, and build/vet/test -count=1 are green.

## Summary

Judge Result: BT-060

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	10.643s

Stage tier2: PASS
  COMPLETE
  ✓ boardctl update -h documents all twelve value flags (worker-status, commit-hash, guard, ci, summary, note, blocked-reason, completed-at, worktree, branch, session, title); root help create/update lines agree with their own usage closures; boardctl update accepts --title and rewrites the row title in place; validate exits 0 with a warning when a title carries a Pn token that disagrees with the priority field (BT-025 warning pattern); go build/vet/test -count=1 green: (1) `go run ./cmd/boardctl update -h` prints all twelve value flags: --title, --worker-status, --commit-hash, --guard, --ci, --summary, --note, --blocked-reason, --completed-at, --worktree, --branch, --session (cmd/boardctl/main.go:700-704 usage closure; flags registered main.go:676-696); TestCmdUpdateHelpListsEveryValueFlag PASS. (2) Root usageText (main.go:54-62) create line now lists [--worktree PATH] [--branch NAME] [--session ID]... and update line lists [--title T] ... [--worktree] [--branch] [--session]; TestRootUsageTextCreateUpdateMatchClosures PASS (flag-for-flag both directions). (3) Live run `update BT-900 --title "[P1] renamed"` exit 0: row title rewritten in place, priority P2 preserved, task_updated event appended (write.go:722-727 set("title"), write.go:844-855 event); TestUpdateTitleRewritesRow + TestCmdUpdateTitleRewritesAndAudits PASS. (4) Live `validate` printed `[warn] tasks.jsonl line 2 (task BT-900): title carries "P1" but priority is "P2" ...` with `RESULT: OK (1 warning(s))` and VALIDATE_EXIT=0 — warn severity, BT-025 pattern (validate.go:187-203); TestBT060TitlePriorityMismatchWarns/SilentCases/MultipleTokens PASS. (5) `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` all packages ok (cmd/boardctl ok 3.125s, internal/board ok 1.078s, no FAIL).
All BT-060 sub-criteria verified: twelve update flags documented, root help agrees with closures, --title rewrites rows in place, validate warns (exit 0) on title/priority Pn drift, and build/vet/test -count=1 are green.

Overall: PASS ✓
