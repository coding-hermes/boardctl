# Verdict: README-012

**Task:** README-012 published v0.1.11 acceptance-criteria flag drift
**Evaluated:** 2026-10-24T16:11:36.624772
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ README-012 acceptance: from a fresh checkout or published v0.1.11 asset, execute the README create/update examples and create --help; either the published asset accepts --acceptance-criteria and writes/replaces acceptance_criteria, or README current-release and acceptance-criteria text no longer presents the flag as available in v0.1.11. Preserve P4 policy. Add a regression/documentation proof and a ch:trace marker.: The fix was never merged into the delivered tree. Commit ec5d086 ('docs: reconcile README acceptance criteria release contract. Addresses README-012.') exists only on branch wt/README-012; `git merge-base --is-ancestor ec5d086 HEAD` -> NO, main HEAD is 14592ed. On main: (a) published asset `./dist/boardctl_linux_amd64 version` -> 'boardctl version v0.1.11' and `create --help`/`update --help | grep -i acceptance` -> empty output (flag absent), so the asset does NOT accept --acceptance-criteria; (b) README.md:1310 still reads 'Both verbs now take `--acceptance-criteria TEXT`:' and README.md:106 still reads 'sanctioned write path for the field, so never hand-edit tasks.jsonl for it.', with only the weak 'Not in the published v0.1.11 binary' note at README.md:1337-1338 — i.e. the flag is still presented as available in v0.1.11; (c) the regression proof is missing: cmd/boardctl/readme_acceptance_criteria_release_test.go does not exist on main and `go test -count=1 -run TestREADMEAcceptanceCriteriaPublishedReleaseContract ./cmd/boardctl/` -> 'ok ... [no tests to run]'; (d) no ch:trace marker for README-012 in README.md on main (only the README-010 marker at README.md:1520) — the marker lives only in the unmerged commit message and the board row's worker_summary. Full suite on main `go test -count=1 ./...` exits 0 (all 12 packages ok) but does not exercise this criterion. In the unmerged worktree /home/kara/worktrees/boardctl-README-012 the fix is correct (test passes: 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.004s'; README wording qualified as current-HEAD/next-release; P4 dispatchability policy preserved), but it is not present in the evaluated checkout.
The README-012 fix (README qualification + regression test + ch:trace) exists only on the unmerged wt/README-012 branch, so the delivered tree still ships a v0.1.11 asset that rejects --acceptance-criteria alongside README text presenting the flag as available.

## Summary

Judge Result: README-012

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ README-012 acceptance: from a fresh checkout or published v0.1.11 asset, execute the README create/update examples and create --help; either the published asset accepts --acceptance-criteria and writes/replaces acceptance_criteria, or README current-release and acceptance-criteria text no longer presents the flag as available in v0.1.11. Preserve P4 policy. Add a regression/documentation proof and a ch:trace marker.: The fix was never merged into the delivered tree. Commit ec5d086 ('docs: reconcile README acceptance criteria release contract. Addresses README-012.') exists only on branch wt/README-012; `git merge-base --is-ancestor ec5d086 HEAD` -> NO, main HEAD is 14592ed. On main: (a) published asset `./dist/boardctl_linux_amd64 version` -> 'boardctl version v0.1.11' and `create --help`/`update --help | grep -i acceptance` -> empty output (flag absent), so the asset does NOT accept --acceptance-criteria; (b) README.md:1310 still reads 'Both verbs now take `--acceptance-criteria TEXT`:' and README.md:106 still reads 'sanctioned write path for the field, so never hand-edit tasks.jsonl for it.', with only the weak 'Not in the published v0.1.11 binary' note at README.md:1337-1338 — i.e. the flag is still presented as available in v0.1.11; (c) the regression proof is missing: cmd/boardctl/readme_acceptance_criteria_release_test.go does not exist on main and `go test -count=1 -run TestREADMEAcceptanceCriteriaPublishedReleaseContract ./cmd/boardctl/` -> 'ok ... [no tests to run]'; (d) no ch:trace marker for README-012 in README.md on main (only the README-010 marker at README.md:1520) — the marker lives only in the unmerged commit message and the board row's worker_summary. Full suite on main `go test -count=1 ./...` exits 0 (all 12 packages ok) but does not exercise this criterion. In the unmerged worktree /home/kara/worktrees/boardctl-README-012 the fix is correct (test passes: 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.004s'; README wording qualified as current-HEAD/next-release; P4 dispatchability policy preserved), but it is not present in the evaluated checkout.
The README-012 fix (README qualification + regression test + ch:trace) exists only on the unmerged wt/README-012 branch, so the delivered tree still ships a v0.1.11 asset that rejects --acceptance-criteria alongside README text presenting the flag as available.

Overall: FAIL ✗
