# Verdict: BT-030

**Task:** Shipped binaries cannot name their release: version prints a build date; tag/README-pin/binary versions ungated
**Evaluated:** 2026-09-16T01:07:50.525223
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m8:07PM[0m [32mINF[0m [1mscanned ~877651 bytes (877.65 KB) in 325ms[0m
[90m8:07PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.651s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ make release stamps the release identity from the git tag; boardctl version surfaces it; a single check-only checker (internal/versioncheck) invoked from make version-check + CI + go test fails on README pin drift, with a negative probe proving it fails: (1) Tag stamping: Makefile:7 `VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || date -u +%Y%m%d)`; repo tags v0.1.0..v0.1.3 so VERSION=v0.1.3; Makefile:48-56 release target builds with `-ldflags "-s -w -X main.version=$(VERSION)"`. Ran `make release` -> exit=0; `./dist/boardctl-linux-amd64 version` -> 'boardctl version v0.1.3' and `version --json` -> {"version":"v0.1.3","build":"v0.1.3"}. Guard: `make release VERSION=20260915` on tagged repo -> exit=2 with 'versioncheck: refusing to release: VERSION ... is not a vX.Y.Z tag but this repo HAS tags'. (2) Surfacing: cmd/boardctl/main.go:805-816 releaseVersion() precedence + main.go:828-858 cmdVersion prints identity/--json; covered by TestCmdVersionOutput, TestCmdVersionTaggedBuild, TestCmdVersionUnstampedModuleBuild, TestReleaseVersionTable, TestModuleVersionRe. (3) Single check-only checker: Makefile:37-38 `version-check: go test -count=1 -run TestVersioncheck ./internal/versioncheck`; .github/workflows/ci.yml:35-36 'Version-check' step runs the byte-identical command; internal/versioncheck/versioncheck_test.go:129 TestVersioncheck runs under plain `go test ./...` and `-short` (no testing.Short skip). Check-only confirmed: grep for WriteFile/os.Create/os.OpenFile in versioncheck.go returns NONE. (4) Negative probe: injected README drift (download URL v0.1.3 -> v9.9.9) -> `make version-check` FAIL exit=2 'download URL #1 pins "v9.9.9" but the release pin line says "v0.1.3"' and `go test -count=1 ./...` FAIL internal/versioncheck; README restored (git diff clean). In-repo probes: TestCheckTable (URL mismatch, missing pin line, zero URLs) and TestCheckNamesFixCommand. Fresh test runs: `go test -count=1 ./...` all ok; `go test -short -count=1 ./...` all ok; `go test -count=1 -run TestVersioncheck ./internal/versioncheck -v` -> PASS. LSP diagnostics: 0.
make release stamps the git tag into shipped binaries (verified: dist binary prints 'boardctl version v0.1.3'), boardctl version surfaces it, and the single check-only internal/versioncheck gate is wired into make version-check + CI + go test and provably fails on README pin drift.

## Summary

Judge Result: BT-030

Stage tier1: PASS
    ✓ secrets: [90m8:07PM[0m [32mINF[0m [1mscanned ~877651 bytes (877.65 KB) in 325ms[0m
[90m8:07PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.651s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ make release stamps the release identity from the git tag; boardctl version surfaces it; a single check-only checker (internal/versioncheck) invoked from make version-check + CI + go test fails on README pin drift, with a negative probe proving it fails: (1) Tag stamping: Makefile:7 `VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || date -u +%Y%m%d)`; repo tags v0.1.0..v0.1.3 so VERSION=v0.1.3; Makefile:48-56 release target builds with `-ldflags "-s -w -X main.version=$(VERSION)"`. Ran `make release` -> exit=0; `./dist/boardctl-linux-amd64 version` -> 'boardctl version v0.1.3' and `version --json` -> {"version":"v0.1.3","build":"v0.1.3"}. Guard: `make release VERSION=20260915` on tagged repo -> exit=2 with 'versioncheck: refusing to release: VERSION ... is not a vX.Y.Z tag but this repo HAS tags'. (2) Surfacing: cmd/boardctl/main.go:805-816 releaseVersion() precedence + main.go:828-858 cmdVersion prints identity/--json; covered by TestCmdVersionOutput, TestCmdVersionTaggedBuild, TestCmdVersionUnstampedModuleBuild, TestReleaseVersionTable, TestModuleVersionRe. (3) Single check-only checker: Makefile:37-38 `version-check: go test -count=1 -run TestVersioncheck ./internal/versioncheck`; .github/workflows/ci.yml:35-36 'Version-check' step runs the byte-identical command; internal/versioncheck/versioncheck_test.go:129 TestVersioncheck runs under plain `go test ./...` and `-short` (no testing.Short skip). Check-only confirmed: grep for WriteFile/os.Create/os.OpenFile in versioncheck.go returns NONE. (4) Negative probe: injected README drift (download URL v0.1.3 -> v9.9.9) -> `make version-check` FAIL exit=2 'download URL #1 pins "v9.9.9" but the release pin line says "v0.1.3"' and `go test -count=1 ./...` FAIL internal/versioncheck; README restored (git diff clean). In-repo probes: TestCheckTable (URL mismatch, missing pin line, zero URLs) and TestCheckNamesFixCommand. Fresh test runs: `go test -count=1 ./...` all ok; `go test -short -count=1 ./...` all ok; `go test -count=1 -run TestVersioncheck ./internal/versioncheck -v` -> PASS. LSP diagnostics: 0.
make release stamps the git tag into shipped binaries (verified: dist binary prints 'boardctl version v0.1.3'), boardctl version surfaces it, and the single check-only internal/versioncheck gate is wired into make version-check + CI + go test and provably fails on README pin drift.

Overall: PASS ✓
