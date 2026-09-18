# Verdict: BT-034

**Task:** cut v0.1.5 so the published binaries stop shipping the vulnerable go1.26.5 toolchain
**Evaluated:** 2026-09-18T02:47:49.900764
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m9:47PM[0m [32mINF[0m [1mscanned ~951960 bytes (951.96 KB) in 396ms[0m
[90m9:47PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.596s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ AC1 README pin line + both download URLs name v0.1.5 and make version-check exits 0. AC2 gh release view v0.1.5 shows 8 assets non-draft and releases/latest resolves v0.1.5. AC3 the DOWNLOADED v0.1.5 linux-amd64 asset reports go build info go1.26.6 (not go1.26.5) with vcs.revision == the v0.1.5 tag commit and vcs.modified=false, with the v0.1.4 go1.26.5 state as the before/after pair. AC4 sha256sum -c passes on the downloaded assets and the asset answers 'boardctl version v0.1.5'. AC5 0 unpushed and the remote tag resolves to the released commit. AC6 go build/vet/test -short/make fmt-check/make version-check/make vuln-check green and gofmt -l clean. AC7 no repo-root boardctl binary and .coding-hermes/board/* in no commit.: AC1: README.md:16 'Current release: **v0.1.5**.'; README.md:19-20 both URLs https://github.com/coding-hermes/boardctl/releases/download/v0.1.5/{boardctl-linux-amd64,sha256sums.txt}; `make version-check` -> 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.002s', EXIT=0. AC2: `gh release view v0.1.5` -> draft:false, prerelease:false, 8 assets (darwin-amd64/arm64, freebsd-amd64, linux-amd64/arm/arm64, windows-amd64.exe, sha256sums.txt); `gh api repos/coding-hermes/boardctl/releases/latest --jq .tag_name` -> v0.1.5. AC3: downloaded v0.1.5 linux-amd64, `go version -m` -> 'go1.26.6', vcs.revision=f9b40415f1642bc3493c821f711e923e09537b8c (== `git rev-parse v0.1.5`), vcs.modified=false; v0.1.4 asset -> 'go1.26.5' (before/after pair confirmed). AC4: `sha256sum -c --ignore-missing sha256sums.txt` -> 'boardctl-linux-amd64: OK', SHA_EXIT=0; `./boardctl-linux-amd64 version` -> 'boardctl version v0.1.5'. AC5: `git rev-list --count origin/main..HEAD` = 0; `git ls-remote origin refs/tags/v0.1.5` -> f9b40415f1642bc3493c821f711e923e09537b8c == released commit (origin/main also f9b4041). AC6: go build ./... BUILD=0; go vet ./... VET=0; go test -short ./... TEST_EXIT=0 (all 6 pkgs ok); make fmt-check FMTCHECK=0; make version-check EXIT=0; make vuln-check VULNCHECK=0; `gofmt -l ./cmd ./internal` empty (clean). AC7: no repo-root boardctl binary (`ls boardctl` -> No such file; `git ls-files | grep '^boardctl$'` empty; no commit ever added it); the BT-034 commit f9b4041 touches only README.md — no .coding-hermes/board/* path in it. (.coding-hermes/board/*.jsonl are tracked by design as the project's own board data across all prior commits, not a stray artifact introduced here.)


## Summary

Judge Result: BT-034

Stage tier1: PASS
    ✓ secrets: [90m9:47PM[0m [32mINF[0m [1mscanned ~951960 bytes (951.96 KB) in 396ms[0m
[90m9:47PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.596s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ AC1 README pin line + both download URLs name v0.1.5 and make version-check exits 0. AC2 gh release view v0.1.5 shows 8 assets non-draft and releases/latest resolves v0.1.5. AC3 the DOWNLOADED v0.1.5 linux-amd64 asset reports go build info go1.26.6 (not go1.26.5) with vcs.revision == the v0.1.5 tag commit and vcs.modified=false, with the v0.1.4 go1.26.5 state as the before/after pair. AC4 sha256sum -c passes on the downloaded assets and the asset answers 'boardctl version v0.1.5'. AC5 0 unpushed and the remote tag resolves to the released commit. AC6 go build/vet/test -short/make fmt-check/make version-check/make vuln-check green and gofmt -l clean. AC7 no repo-root boardctl binary and .coding-hermes/board/* in no commit.: AC1: README.md:16 'Current release: **v0.1.5**.'; README.md:19-20 both URLs https://github.com/coding-hermes/boardctl/releases/download/v0.1.5/{boardctl-linux-amd64,sha256sums.txt}; `make version-check` -> 'ok github.com/coding-hermes/boardctl/internal/versioncheck 0.002s', EXIT=0. AC2: `gh release view v0.1.5` -> draft:false, prerelease:false, 8 assets (darwin-amd64/arm64, freebsd-amd64, linux-amd64/arm/arm64, windows-amd64.exe, sha256sums.txt); `gh api repos/coding-hermes/boardctl/releases/latest --jq .tag_name` -> v0.1.5. AC3: downloaded v0.1.5 linux-amd64, `go version -m` -> 'go1.26.6', vcs.revision=f9b40415f1642bc3493c821f711e923e09537b8c (== `git rev-parse v0.1.5`), vcs.modified=false; v0.1.4 asset -> 'go1.26.5' (before/after pair confirmed). AC4: `sha256sum -c --ignore-missing sha256sums.txt` -> 'boardctl-linux-amd64: OK', SHA_EXIT=0; `./boardctl-linux-amd64 version` -> 'boardctl version v0.1.5'. AC5: `git rev-list --count origin/main..HEAD` = 0; `git ls-remote origin refs/tags/v0.1.5` -> f9b40415f1642bc3493c821f711e923e09537b8c == released commit (origin/main also f9b4041). AC6: go build ./... BUILD=0; go vet ./... VET=0; go test -short ./... TEST_EXIT=0 (all 6 pkgs ok); make fmt-check FMTCHECK=0; make version-check EXIT=0; make vuln-check VULNCHECK=0; `gofmt -l ./cmd ./internal` empty (clean). AC7: no repo-root boardctl binary (`ls boardctl` -> No such file; `git ls-files | grep '^boardctl$'` empty; no commit ever added it); the BT-034 commit f9b4041 touches only README.md — no .coding-hermes/board/* path in it. (.coding-hermes/board/*.jsonl are tracked by design as the project's own board data across all prior commits, not a stray artifact introduced here.)


Overall: PASS ✓
