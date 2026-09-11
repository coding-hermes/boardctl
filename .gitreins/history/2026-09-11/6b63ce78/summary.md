# Verdict: BT-015

**Task:** cut v0.1.2 release (published binary drift)
**Evaluated:** 2026-09-11T17:42:29.722427
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:41PM[0m [32mINF[0m [1mscanned ~323590 bytes (323.59 KB) in 218ms[0m
[90m12:41PM[0m [
- ✗ **tier2**
  - INCOMPLETE
  ✗ make release produces all 7 targets + dist/sha256sums.txt; tag v0.1.2 pushed; gh release v0.1.2 created with binaries + sha256sums.txt; README install pins moved to v0.1.2; sha256sum -c passes on the downloaded artifacts.: Compound criterion; 4 of 6 sub-checks pass, 2 fail. PASS: (a) `make release` exit 0, emits all 7 platforms (linux/amd64, linux/arm64, linux/arm, darwin/amd64, darwin/arm64, windows/amd64, freebsd/amd64) + dist/sha256sums.txt (617B). (b) `git ls-remote --tags origin` shows refs/tags/v0.1.2 -> 7785febd997428c3cfb52da6392cc3708503384b (== HEAD) — tag pushed. (c) `gh release view v0.1.2` -> draft:false, published 2026-09-11T17:40:49Z, 8 assets (7 binaries + sha256sums.txt), sizes match local dist exactly. (d) README.md:17-18 pins moved to v0.1.2 (commit 7785feb diff v0.1.1->v0.1.2). FAIL E — 'sha256sum -c passes on the downloaded artifacts': ran the README flow verbatim (curl -o boardctl .../v0.1.2/boardctl-linux-amd64; curl -o sha256sums.txt; sha256sum -c sha256sums.txt) => EXIT=1, 'boardctl-darwin-amd64: FAILED open or read' ... 'WARNING: 7 listed files could not be read'. sha256sums.txt lists 7 platform filenames but the README downloads a single file named `boardctl`; even after renaming to boardctl-linux-amd64 it still exits 1 (4 listed files absent). The README's own documented verification command does not pass. (Local dist/ `sha256sum -c` does pass 7x OK exit 0, but the criterion specifies downloaded artifacts.) FAIL F — published binary drift, the task's core subject: `go version -m` on the published v0.1.2 binaries (linux-amd64, darwin-arm64, windows-amd64.exe) all record build vcs.revision=d7f641904b2c6457240301f967054474968819de with vcs.modified=true — d7f6419 is the PREVIOUS commit (BT-016), not the tagged release commit 7785feb. Rebuilding from HEAD with VERSION=20260911 yields all 7 checksums differing from the published sha256sums.txt, confirming the published artifacts do not correspond to the tagged source. Drift is functionally benign (only README.md differs d7f6419..7785feb; no go:embed) but the release is not reproducible from the tag and embeds the wrong revision. Tests: `go test -count=1 ./...` exit 0 (cmd/boardctl ok 0.014s, internal/board ok 0.069s).


## Summary

Judge Result: BT-015

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:41PM[0m [32mINF[0m [1mscanned ~323590 bytes (323.59 KB) in 218ms[0m
[90m12:41PM[0m [

Stage tier2: FAIL
  INCOMPLETE
  ✗ make release produces all 7 targets + dist/sha256sums.txt; tag v0.1.2 pushed; gh release v0.1.2 created with binaries + sha256sums.txt; README install pins moved to v0.1.2; sha256sum -c passes on the downloaded artifacts.: Compound criterion; 4 of 6 sub-checks pass, 2 fail. PASS: (a) `make release` exit 0, emits all 7 platforms (linux/amd64, linux/arm64, linux/arm, darwin/amd64, darwin/arm64, windows/amd64, freebsd/amd64) + dist/sha256sums.txt (617B). (b) `git ls-remote --tags origin` shows refs/tags/v0.1.2 -> 7785febd997428c3cfb52da6392cc3708503384b (== HEAD) — tag pushed. (c) `gh release view v0.1.2` -> draft:false, published 2026-09-11T17:40:49Z, 8 assets (7 binaries + sha256sums.txt), sizes match local dist exactly. (d) README.md:17-18 pins moved to v0.1.2 (commit 7785feb diff v0.1.1->v0.1.2). FAIL E — 'sha256sum -c passes on the downloaded artifacts': ran the README flow verbatim (curl -o boardctl .../v0.1.2/boardctl-linux-amd64; curl -o sha256sums.txt; sha256sum -c sha256sums.txt) => EXIT=1, 'boardctl-darwin-amd64: FAILED open or read' ... 'WARNING: 7 listed files could not be read'. sha256sums.txt lists 7 platform filenames but the README downloads a single file named `boardctl`; even after renaming to boardctl-linux-amd64 it still exits 1 (4 listed files absent). The README's own documented verification command does not pass. (Local dist/ `sha256sum -c` does pass 7x OK exit 0, but the criterion specifies downloaded artifacts.) FAIL F — published binary drift, the task's core subject: `go version -m` on the published v0.1.2 binaries (linux-amd64, darwin-arm64, windows-amd64.exe) all record build vcs.revision=d7f641904b2c6457240301f967054474968819de with vcs.modified=true — d7f6419 is the PREVIOUS commit (BT-016), not the tagged release commit 7785feb. Rebuilding from HEAD with VERSION=20260911 yields all 7 checksums differing from the published sha256sums.txt, confirming the published artifacts do not correspond to the tagged source. Drift is functionally benign (only README.md differs d7f6419..7785feb; no go:embed) but the release is not reproducible from the tag and embeds the wrong revision. Tests: `go test -count=1 ./...` exit 0 (cmd/boardctl ok 0.014s, internal/board ok 0.069s).


Overall: FAIL ✗
