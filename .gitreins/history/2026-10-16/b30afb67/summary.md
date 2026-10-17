# Verdict: DF-BOARDCTL-39

**Task:** Cut v0.1.11 — normalize-board and other shipped features not in v0.1.10 release artifact
**Evaluated:** 2026-10-16T23:35:08.765952
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Cut v0.1.11 from origin/main. README pins moved to v0.1.11. Tag pushed. CI multi-arch release job publishes. Downloaded asset sha256sum -c verified, version prints v0.1.11, go version -m reports vcs.revision==tag commit, vcs.modified=false. Behavior probe: normalize-board --help works on downloaded binary.: Only the README pin bump was done (commit 3684847: 7 lines v0.1.10->v0.1.11). The actual release cut was never performed. (1) Tag pushed: FAIL — `git tag -l 'v0.1.11'` returns empty and `git ls-remote --tags origin refs/tags/v0.1.11` returns empty; origin's newest tag is v0.1.10. (2) CI multi-arch release job publishes: FAIL — .github/workflows/multiarch.yml release job only runs on a tag ref, and no v0.1.11 tag was pushed, so nothing published. (3) Downloaded asset sha256sum -c verified: FAIL — `curl -sL -o /dev/null -w '%{http_code}' https://github.com/coding-hermes/boardctl/releases/download/v0.1.11/SHA256SUMS` -> 404, and .../v0.1.11/boardctl_linux_amd64 -> 404 (control: v0.1.10/SHA256SUMS -> 200, proving the URL pattern is valid and the v0.1.11 release is genuinely absent). (4) version prints v0.1.11: FAIL — `./bin/boardctl version` -> 'boardctl version dev' (unstamped local build, not a v0.1.11 release asset). (5) go version -m vcs.revision==tag commit, vcs.modified=false: FAIL — no v0.1.11 tag commit exists to compare against and no downloaded release binary is available. (6) Behavior probe normalize-board --help: only works on the LOCAL ./bin/boardctl (prints usage), not on a downloaded v0.1.11 binary, which does not exist. Note: `go test -short -count=1 ./...` passes (all 12 packages ok), but that does not satisfy any part of this release-cut criterion.
README pins were bumped to v0.1.11, but the v0.1.11 tag was never created or pushed, no CI release was published (asset URLs return 404), and the local binary still reports 'dev' — the release cut itself is entirely missing.

## Summary

Judge Result: DF-BOARDCTL-39

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Cut v0.1.11 from origin/main. README pins moved to v0.1.11. Tag pushed. CI multi-arch release job publishes. Downloaded asset sha256sum -c verified, version prints v0.1.11, go version -m reports vcs.revision==tag commit, vcs.modified=false. Behavior probe: normalize-board --help works on downloaded binary.: Only the README pin bump was done (commit 3684847: 7 lines v0.1.10->v0.1.11). The actual release cut was never performed. (1) Tag pushed: FAIL — `git tag -l 'v0.1.11'` returns empty and `git ls-remote --tags origin refs/tags/v0.1.11` returns empty; origin's newest tag is v0.1.10. (2) CI multi-arch release job publishes: FAIL — .github/workflows/multiarch.yml release job only runs on a tag ref, and no v0.1.11 tag was pushed, so nothing published. (3) Downloaded asset sha256sum -c verified: FAIL — `curl -sL -o /dev/null -w '%{http_code}' https://github.com/coding-hermes/boardctl/releases/download/v0.1.11/SHA256SUMS` -> 404, and .../v0.1.11/boardctl_linux_amd64 -> 404 (control: v0.1.10/SHA256SUMS -> 200, proving the URL pattern is valid and the v0.1.11 release is genuinely absent). (4) version prints v0.1.11: FAIL — `./bin/boardctl version` -> 'boardctl version dev' (unstamped local build, not a v0.1.11 release asset). (5) go version -m vcs.revision==tag commit, vcs.modified=false: FAIL — no v0.1.11 tag commit exists to compare against and no downloaded release binary is available. (6) Behavior probe normalize-board --help: only works on the LOCAL ./bin/boardctl (prints usage), not on a downloaded v0.1.11 binary, which does not exist. Note: `go test -short -count=1 ./...` passes (all 12 packages ok), but that does not satisfy any part of this release-cut criterion.
README pins were bumped to v0.1.11, but the v0.1.11 tag was never created or pushed, no CI release was published (asset URLs return 404), and the local binary still reports 'dev' — the release cut itself is entirely missing.

Overall: FAIL ✗
