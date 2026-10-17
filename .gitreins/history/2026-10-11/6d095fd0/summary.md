# Verdict: DOC-26

**Task:** Annotate post-v0.1.10 flags in README usage block
**Evaluated:** 2026-10-11T11:49:56.567308
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README usage examples for --deferred, --pull-request, --assoc-branch, and --assoc-worktree carry since-v0.1.11 annotations; documentation change is committed and the merged tree's direct tests pass aside from the pre-existing host govulncheck v1.8.0 versus repository pin v1.7.0 environment mismatch.: README.md:941-942 show `--deferred` examples with `# (since v0.1.11)`; README.md:976 `--pull-request 42 --pull-request 43   # (since v0.1.11)`; README.md:977 `--assoc-branch wt/fix-a ...   # (since v0.1.11)`; README.md:978 `--assoc-worktree ~/worktrees/fix-a   # (since v0.1.11)`. Commit 5484722 'docs: annotate post-v0.1.10 flags in README usage block (DOC-26)' is an ancestor of HEAD (merged via ac608f9, `git merge-base --is-ancestor` confirmed). `go test -count=1 ./...` exit_code=1, but the sole failure is internal/vulncheck TestVulncheck at vulncheck_test.go:449: 'govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /home/kara/go/bin/govulncheck reports v1.8.0' — exactly the pre-existing host environment mismatch the criterion excludes. All other packages pass: ok cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/workflowcheck.
All four post-v0.1.10 flags carry since-v0.1.11 annotations in README, the doc commit is merged into HEAD, and the only test failure is the pre-existing govulncheck v1.8.0-vs-v1.7.0 environment mismatch.

## Summary

Judge Result: DOC-26

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README usage examples for --deferred, --pull-request, --assoc-branch, and --assoc-worktree carry since-v0.1.11 annotations; documentation change is committed and the merged tree's direct tests pass aside from the pre-existing host govulncheck v1.8.0 versus repository pin v1.7.0 environment mismatch.: README.md:941-942 show `--deferred` examples with `# (since v0.1.11)`; README.md:976 `--pull-request 42 --pull-request 43   # (since v0.1.11)`; README.md:977 `--assoc-branch wt/fix-a ...   # (since v0.1.11)`; README.md:978 `--assoc-worktree ~/worktrees/fix-a   # (since v0.1.11)`. Commit 5484722 'docs: annotate post-v0.1.10 flags in README usage block (DOC-26)' is an ancestor of HEAD (merged via ac608f9, `git merge-base --is-ancestor` confirmed). `go test -count=1 ./...` exit_code=1, but the sole failure is internal/vulncheck TestVulncheck at vulncheck_test.go:449: 'govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /home/kara/go/bin/govulncheck reports v1.8.0' — exactly the pre-existing host environment mismatch the criterion excludes. All other packages pass: ok cmd/boardctl, cmd/dedupe-board, cmd/gendocs, internal/board, internal/describe, internal/fmtcheck, internal/freshness, internal/render, internal/speccheck, internal/versioncheck, internal/workflowcheck.
All four post-v0.1.10 flags carry since-v0.1.11 annotations in README, the doc commit is merged into HEAD, and the only test failure is the pre-existing govulncheck v1.8.0-vs-v1.7.0 environment mismatch.

Overall: FAIL ✗
