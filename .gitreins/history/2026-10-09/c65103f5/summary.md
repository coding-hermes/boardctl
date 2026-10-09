# Verdict: DF-BOARDCTL-28

**Task:** README documents BT-077 association fields
**Evaluated:** 2026-10-09T23:21:42.346012
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README subsection covers flags, one-dimension rule, merge semantics; version-check passes: README.md:796-830 adds '### Association fields (BT-077)' covering all three required elements: (1) flags — README:800-803 documents --pull-request PR (bare number or GitHub URL), --assoc-branch B, --assoc-worktree PATH, matching cmd/boardctl/main.go:823-825; (2) one-dimension rule — README:805-809 states mixing flags from two dimensions is a usage error with nothing written, quoting the exact error text found at cmd/boardctl/main.go:727,947,949 ('apply one dimension per invocation — run a second update for the next dimension'); (3) merge semantics — README:810-815 documents MERGE-never-overwrite, idempotent duplicates, sibling dimensions and singular worktree/branch preserved, matching MergeAssociation in internal/board/associations.go:322-355. Version-check passes: `go test -count=1 -run TestVersioncheck ./internal/versioncheck` => 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.004s', exit_code 0. Full changed-package suite `go test -count=1 ./cmd/boardctl/... ./internal/board/... ./internal/versioncheck/...` => all 'ok', exit_code 0.
README subsection documents BT-077 flags, one-dimension rule, and merge semantics consistent with the implementation, and the version-check gate passes.

## Summary

Judge Result: DF-BOARDCTL-28

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README subsection covers flags, one-dimension rule, merge semantics; version-check passes: README.md:796-830 adds '### Association fields (BT-077)' covering all three required elements: (1) flags — README:800-803 documents --pull-request PR (bare number or GitHub URL), --assoc-branch B, --assoc-worktree PATH, matching cmd/boardctl/main.go:823-825; (2) one-dimension rule — README:805-809 states mixing flags from two dimensions is a usage error with nothing written, quoting the exact error text found at cmd/boardctl/main.go:727,947,949 ('apply one dimension per invocation — run a second update for the next dimension'); (3) merge semantics — README:810-815 documents MERGE-never-overwrite, idempotent duplicates, sibling dimensions and singular worktree/branch preserved, matching MergeAssociation in internal/board/associations.go:322-355. Version-check passes: `go test -count=1 -run TestVersioncheck ./internal/versioncheck` => 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.004s', exit_code 0. Full changed-package suite `go test -count=1 ./cmd/boardctl/... ./internal/board/... ./internal/versioncheck/...` => all 'ok', exit_code 0.
README subsection documents BT-077 flags, one-dimension rule, and merge semantics consistent with the implementation, and the version-check gate passes.

Overall: PASS ✓
