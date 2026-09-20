# Verdict: DF-BOARDCTL-1

**Task:** Burndown chart never renders: template reads d.burndown.days but the payload's burndown se
**Evaluated:** 2026-09-20T05:36:23.597889
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.179s
- ✓ **tier2**
  - COMPLETE
  ✓ Fix implemented in worktree, merged to main, full Go gate green (build+vet+test), pushed to origin: Fix in worktree /home/kara/worktrees/coding-hermes-boardctl-DF-BOARDCTL-1 @ 238fa0e [wt/DF-BOARDCTL-1]: internal/render/template.go:714 and :725 changed d.burndown.days -> d.burndown.window, matching payload struct internal/render/derive.go:41-44 (BurndownSeries{Window []string `json:"window"`; Open []int `json:"open"`} — no 'days' key). Merged to main via 957225f; `git merge-base --is-ancestor 957225f origin/main` => YES. Pushed: local main == origin/main == cfff3634a70151528e4bafa4171cebd7d82ecc17, `git rev-list --left-right --count main...origin/main` => 0 0. Go gate green: `go build ./...` exit=0; `go vet ./...` exit=0; `go test -count=1 ./...` TEST EXIT=0 (ok cmd/boardctl, internal/board, internal/fmtcheck, internal/render, internal/versioncheck, internal/vulncheck). Regression test `go test -count=1 -run TestRenderBurndownUsesWindow -v ./cmd/boardctl/` => '--- PASS: TestRenderBurndownUsesWindow (0.00s)', ok, EXIT=0. No stale d.burndown.days in template (only the test's negative assertion at render_test.go:464-466).
DF-BOARDCTL-1 fix (burndown reads d.burndown.window) is implemented in the worktree, merged to main, pushed to origin, and the full Go build+vet+test gate is green.

## Summary

Judge Result: DF-BOARDCTL-1

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.179s

Stage tier2: PASS
  COMPLETE
  ✓ Fix implemented in worktree, merged to main, full Go gate green (build+vet+test), pushed to origin: Fix in worktree /home/kara/worktrees/coding-hermes-boardctl-DF-BOARDCTL-1 @ 238fa0e [wt/DF-BOARDCTL-1]: internal/render/template.go:714 and :725 changed d.burndown.days -> d.burndown.window, matching payload struct internal/render/derive.go:41-44 (BurndownSeries{Window []string `json:"window"`; Open []int `json:"open"`} — no 'days' key). Merged to main via 957225f; `git merge-base --is-ancestor 957225f origin/main` => YES. Pushed: local main == origin/main == cfff3634a70151528e4bafa4171cebd7d82ecc17, `git rev-list --left-right --count main...origin/main` => 0 0. Go gate green: `go build ./...` exit=0; `go vet ./...` exit=0; `go test -count=1 ./...` TEST EXIT=0 (ok cmd/boardctl, internal/board, internal/fmtcheck, internal/render, internal/versioncheck, internal/vulncheck). Regression test `go test -count=1 -run TestRenderBurndownUsesWindow -v ./cmd/boardctl/` => '--- PASS: TestRenderBurndownUsesWindow (0.00s)', ok, EXIT=0. No stale d.burndown.days in template (only the test's negative assertion at render_test.go:464-466).
DF-BOARDCTL-1 fix (burndown reads d.burndown.window) is implemented in the worktree, merged to main, pushed to origin, and the full Go build+vet+test gate is green.

Overall: PASS ✓
