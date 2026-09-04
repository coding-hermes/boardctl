# Verdict: BT-003

**Task:** cut v0.1.1 release — binaries include doctor + version stamp
**Evaluated:** 2026-09-03T20:38:05.473869
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m3:37PM[0m [32mINF[0m [1mscanned ~96348 bytes (96.35 KB) in 46.5ms[0m
[90m3:37PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ v0.1.1 GitHub release exists with cross-compiled binaries built from a commit containing doctor (7dc0b9a) and version stamp (f649df9); release notes mention doctor and version subcommand; tag v0.1.1 pushed to origin: gh release view v0.1.1 confirms published (not draft/prerelease) release at github.com/coding-hermes/boardctl/releases/tag/v0.1.1 with 7 cross-compiled assets (darwin-amd64/arm64, linux-amd64/arm/arm64, windows-amd64.exe, freebsd-amd64). Downloaded boardctl-linux-amd64 asset runs: `version` -> 'boardctl version 20260903', `--help` lists doctor+version. Tag v0.1.1 -> a15a115; both 7dc0b9a (doctor) and f649df9 (version stamp) are ancestors (git merge-base --is-ancestor = YES). Release notes body: 'doctor subcommand: deep board validation... — 7dc0b9a' and 'version stamp: version var + version subcommand... — f649df9'. Tag pushed to origin: git ls-remote origin v0.1.1 -> a15a115 refs/tags/v0.1.1; gh api git/ref/tags/v0.1.1 sha=a15a115.
v0.1.1 GitHub release exists with cross-compiled binaries containing doctor and version stamp, built from commit a15a115 which includes both 7dc0b9a and f649df9; release notes mention doctor and version subcommand; tag v0.1.1 is pushed to origin.

## Summary

Judge Result: BT-003

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m3:37PM[0m [32mINF[0m [1mscanned ~96348 bytes (96.35 KB) in 46.5ms[0m
[90m3:37PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ v0.1.1 GitHub release exists with cross-compiled binaries built from a commit containing doctor (7dc0b9a) and version stamp (f649df9); release notes mention doctor and version subcommand; tag v0.1.1 pushed to origin: gh release view v0.1.1 confirms published (not draft/prerelease) release at github.com/coding-hermes/boardctl/releases/tag/v0.1.1 with 7 cross-compiled assets (darwin-amd64/arm64, linux-amd64/arm/arm64, windows-amd64.exe, freebsd-amd64). Downloaded boardctl-linux-amd64 asset runs: `version` -> 'boardctl version 20260903', `--help` lists doctor+version. Tag v0.1.1 -> a15a115; both 7dc0b9a (doctor) and f649df9 (version stamp) are ancestors (git merge-base --is-ancestor = YES). Release notes body: 'doctor subcommand: deep board validation... — 7dc0b9a' and 'version stamp: version var + version subcommand... — f649df9'. Tag pushed to origin: git ls-remote origin v0.1.1 -> a15a115 refs/tags/v0.1.1; gh api git/ref/tags/v0.1.1 sha=a15a115.
v0.1.1 GitHub release exists with cross-compiled binaries containing doctor and version stamp, built from commit a15a115 which includes both 7dc0b9a and f649df9; release notes mention doctor and version subcommand; tag v0.1.1 is pushed to origin.

Overall: PASS ✓
