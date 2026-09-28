# Verdict: DF-BOARDCTL-17

**Task:** wave tick boardctl-2026-09-27-23-52-37: DF-BOARDCTL-17
**Evaluated:** 2026-09-28T00:50:03.553960
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.680s
- ✓ **tier2**
  - COMPLETE
  ✓ see board row DF-BOARDCTL-17 acceptance criteria: Resolved the row's acceptance criteria from /tmp/brief-DF-BOARDCTL-17.txt + .coding-hermes/board/tasks.jsonl:107. (A1) `make version-check VERSION_TAG=v0.1.9` (README names v0.1.8) FAILS exit 2 with precise message 'stale release pin line #1: says "v0.1.8", expected "v0.1.9"' + 'stale download URL #1/#2: pins "v0.1.8", expected "v0.1.9"'; `VERSION_TAG=v0.1.8` PASSES exit 0 — versioncheck.go CheckExpected/check() + Makefile VERSION_TAG export. (A2) `make version-check` (no tag) exit 0, agreement-only unchanged; table cell 'agreement-only mode passes a consistent older tag' passes. (A3) versioncheck_test.go TestCheckTable 9 cells incl. expected-tag pass, stale-URL fail, stale-pin-line fail, malformed tag, agreement-only; `go test ./internal/versioncheck/ -count=1 -v` all PASS ok 0.010s. (A4) README.md:663-671 (### Cutting a release (BT-030)) now says 'run `make version-check VERSION_TAG=<new tag>`... The gate checks agreement, not currency: without `VERSION_TAG` a README that consistently names the previous release passes, so pin the tag you are about to cut.' — section not restructured. (A5) gofmt clean (exit 0), `go build ./...` exit 0, `go vet ./...` exit 0; `go test ./... -count=1` red ONLY at cmd/boardctl TestUsageSkillDateGateBitesOnHeaderMutations/date_just_inside_tolerance_passes, which I verified is PRE-EXISTING: it fails identically at the task base ae429f1 (detached worktree) and at branch 6e4cfa8, in cmd/boardctl/df15_skill_closure_test.go — a file DF-BOARDCTL-17 did not touch (commit changed only Makefile, README.md, versioncheck.go, versioncheck_test.go); cause is time.Since(d) instant-vs-calendar date arithmetic, unrelated to versioncheck. (A6) exactly one commit 6e4cfa8 on the branch since base ae429f1, message 'feat: version-check expected-tag currency gate (DF-BOARDCTL-17)' with a body naming test evidence; only the 4 allowed files changed. LSP diagnostics: 0 findings.
DF-BOARDCTL-17's expected-tag currency gate is fully implemented and verified (currency FAIL/PASS, back-compat, table tests, README sentence, single correct commit); the only red test is a pre-existing, out-of-scope date flake present at the task's base commit.

## Summary

Judge Result: DF-BOARDCTL-17

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.680s

Stage tier2: PASS
  COMPLETE
  ✓ see board row DF-BOARDCTL-17 acceptance criteria: Resolved the row's acceptance criteria from /tmp/brief-DF-BOARDCTL-17.txt + .coding-hermes/board/tasks.jsonl:107. (A1) `make version-check VERSION_TAG=v0.1.9` (README names v0.1.8) FAILS exit 2 with precise message 'stale release pin line #1: says "v0.1.8", expected "v0.1.9"' + 'stale download URL #1/#2: pins "v0.1.8", expected "v0.1.9"'; `VERSION_TAG=v0.1.8` PASSES exit 0 — versioncheck.go CheckExpected/check() + Makefile VERSION_TAG export. (A2) `make version-check` (no tag) exit 0, agreement-only unchanged; table cell 'agreement-only mode passes a consistent older tag' passes. (A3) versioncheck_test.go TestCheckTable 9 cells incl. expected-tag pass, stale-URL fail, stale-pin-line fail, malformed tag, agreement-only; `go test ./internal/versioncheck/ -count=1 -v` all PASS ok 0.010s. (A4) README.md:663-671 (### Cutting a release (BT-030)) now says 'run `make version-check VERSION_TAG=<new tag>`... The gate checks agreement, not currency: without `VERSION_TAG` a README that consistently names the previous release passes, so pin the tag you are about to cut.' — section not restructured. (A5) gofmt clean (exit 0), `go build ./...` exit 0, `go vet ./...` exit 0; `go test ./... -count=1` red ONLY at cmd/boardctl TestUsageSkillDateGateBitesOnHeaderMutations/date_just_inside_tolerance_passes, which I verified is PRE-EXISTING: it fails identically at the task base ae429f1 (detached worktree) and at branch 6e4cfa8, in cmd/boardctl/df15_skill_closure_test.go — a file DF-BOARDCTL-17 did not touch (commit changed only Makefile, README.md, versioncheck.go, versioncheck_test.go); cause is time.Since(d) instant-vs-calendar date arithmetic, unrelated to versioncheck. (A6) exactly one commit 6e4cfa8 on the branch since base ae429f1, message 'feat: version-check expected-tag currency gate (DF-BOARDCTL-17)' with a body naming test evidence; only the 4 allowed files changed. LSP diagnostics: 0 findings.
DF-BOARDCTL-17's expected-tag currency gate is fully implemented and verified (currency FAIL/PASS, back-compat, table tests, README sentence, single correct commit); the only red test is a pre-existing, out-of-scope date flake present at the task's base commit.

Overall: PASS ✓
