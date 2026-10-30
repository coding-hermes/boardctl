# Verdict: RELEASE-2026-10-29-01

**Task:** v0.2.0 release: version-prep, changelog, tag, published-asset proof
**Evaluated:** 2026-10-30T20:59:57.029099
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README release surfaces (Current release line + every /releases/download/ URL) name v0.2.0 and make version-check VERSION_TAG=v0.2.0 exits 0; CHANGELOG.md exists with exactly one ## v0.2.0 heading and a ## Unreleased section; CI and Multi-Arch green on the exact prep SHA; annotated tag v0.2.0 is pushed and the GitHub Release carries the platform assets plus SHA256SUMS; the downloaded asset passes sha256sum -c --ignore-missing, ./boardctl version prints v0.2.0, and go version -m reports vcs.revision equal to the v0.2.0 commit with vcs.modified=false; make install-check RELEASE_TAG=v0.2.0 exits 0: README.md:17 'Current release: **v0.2.0**'; README.md:20-21 both /releases/download/ URLs use v0.2.0 (only other match at README.md:1758 is the `<tag>` template in the release procedure). `make version-check VERSION_TAG=v0.2.0` -> EXIT=0, 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.006s'. CHANGELOG.md has exactly one '## v0.2.0' (line 7, grep -c = 1) and '## Unreleased' (line 3). gh run list: CI run 41126561474 conclusion=success and Multi-Arch run 41127013817 conclusion=success, both headSha 2d3f2e6c63d220ab31656f93de3e642f5cde3a37 == v0.2.0^{}. Tag is annotated (git cat-file -p v0.2.0 -> 'object 2d3f2e6... type commit tag v0.2.0') and pushed (git ls-remote origin shows refs/tags/v0.2.0 b1c26c8 and ^{} 2d3f2e6). gh release view v0.2.0: isDraft=false, isPrerelease=false, assets = boardctl_{darwin_amd64,darwin_arm64,freebsd_amd64,linux_amd64,linux_arm,linux_arm64,windows_amd64.exe} + SHA256SUMS. Fresh download from the release URL in /tmp/relproof: `sha256sum -c --ignore-missing SHA256SUMS` -> 'boardctl_linux_amd64: OK' EXIT=0; `./boardctl_linux_amd64 version` -> 'boardctl version v0.2.0'; `go version -m` -> mod v0.2.0, vcs.revision=2d3f2e6c63d220ab31656f93de3e642f5cde3a37, vcs.modified=false. `make install-check RELEASE_TAG=v0.2.0` -> EXIT=0 with 'ok version == v0.2.0', 'ok module version == v0.2.0', 'ok vcs.modified=false', 'ok vcs.revision 2d3f2e6... is an ancestor of v0.2.0'.


## Summary

Judge Result: RELEASE-2026-10-29-01

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README release surfaces (Current release line + every /releases/download/ URL) name v0.2.0 and make version-check VERSION_TAG=v0.2.0 exits 0; CHANGELOG.md exists with exactly one ## v0.2.0 heading and a ## Unreleased section; CI and Multi-Arch green on the exact prep SHA; annotated tag v0.2.0 is pushed and the GitHub Release carries the platform assets plus SHA256SUMS; the downloaded asset passes sha256sum -c --ignore-missing, ./boardctl version prints v0.2.0, and go version -m reports vcs.revision equal to the v0.2.0 commit with vcs.modified=false; make install-check RELEASE_TAG=v0.2.0 exits 0: README.md:17 'Current release: **v0.2.0**'; README.md:20-21 both /releases/download/ URLs use v0.2.0 (only other match at README.md:1758 is the `<tag>` template in the release procedure). `make version-check VERSION_TAG=v0.2.0` -> EXIT=0, 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.006s'. CHANGELOG.md has exactly one '## v0.2.0' (line 7, grep -c = 1) and '## Unreleased' (line 3). gh run list: CI run 41126561474 conclusion=success and Multi-Arch run 41127013817 conclusion=success, both headSha 2d3f2e6c63d220ab31656f93de3e642f5cde3a37 == v0.2.0^{}. Tag is annotated (git cat-file -p v0.2.0 -> 'object 2d3f2e6... type commit tag v0.2.0') and pushed (git ls-remote origin shows refs/tags/v0.2.0 b1c26c8 and ^{} 2d3f2e6). gh release view v0.2.0: isDraft=false, isPrerelease=false, assets = boardctl_{darwin_amd64,darwin_arm64,freebsd_amd64,linux_amd64,linux_arm,linux_arm64,windows_amd64.exe} + SHA256SUMS. Fresh download from the release URL in /tmp/relproof: `sha256sum -c --ignore-missing SHA256SUMS` -> 'boardctl_linux_amd64: OK' EXIT=0; `./boardctl_linux_amd64 version` -> 'boardctl version v0.2.0'; `go version -m` -> mod v0.2.0, vcs.revision=2d3f2e6c63d220ab31656f93de3e642f5cde3a37, vcs.modified=false. `make install-check RELEASE_TAG=v0.2.0` -> EXIT=0 with 'ok version == v0.2.0', 'ok module version == v0.2.0', 'ok vcs.modified=false', 'ok vcs.revision 2d3f2e6... is an ancestor of v0.2.0'.


Overall: FAIL ✗
