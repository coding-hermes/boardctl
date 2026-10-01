# Verdict: BT-064

**Task:** vulncheck gate refuses disagreeing govulncheck version
**Evaluated:** 2026-09-29T10:41:16.610040
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.238s
- ✓ **tier2**
  - COMPLETE
  ✓ RED->GREEN test + live proof on installed binary: RED proven: reverting Check() to `res, err := Run(dir)` (removing the CheckVersion probe) makes TestCheckWithFakeBinary/wrong_-version_fails_naming_the_mismatch fail — 'vulncheck_test.go:239: Check() = nil for a binary reporting a scanner version other than the pin, want an error' (exit 2). GREEN proven after restore: `go test -count=1 ./internal/vulncheck/` -> 'ok github.com/coding-hermes/boardctl/internal/vulncheck 2.966s'; subtests TestCheckWithFakeBinary 6/6 PASS (incl. wrong -version refused, correct -version proceeds, missing Scanner line skips), TestCheckVersionOffline 5/5 PASS, TestSameMinorLine PASS. Live proof on the installed binary: `/home/kara/go/bin/govulncheck -version` reports 'Scanner: govulncheck@v1.7.0' matching PinnedToolVersion (internal/vulncheck/vulncheck.go:62); TestVulncheck PASS (2.89s) and `make vuln-check` -> 'ok ... 2.776s'. Live refusal demonstrated end-to-end: GOVULNCHECK=/tmp/fakebin/govulncheck (self-reports v1.5.2) makes the real gate fail with 'govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /tmp/fakebin/govulncheck reports v1.5.2 — a scan by a different scanner rule set must not stand in for the pinned gate (fix: go install golang.org/x/vuln/cmd/govulncheck@v1.7.0)'. Supporting: Check() (vulncheck.go:369-380) probes via CheckVersion before RunBinary; MismatchErr.Error() names expected/actual/install hint; go build ./... OK, go vet OK, full `go test -count=1 ./...` all packages ok.
The vulncheck gate now refuses a govulncheck binary whose -version disagrees with the v1.7.0 pin, proven RED->GREEN by tests and live against the installed binary (and a disagreeing stand-in).

## Summary

Judge Result: BT-064

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.238s

Stage tier2: PASS
  COMPLETE
  ✓ RED->GREEN test + live proof on installed binary: RED proven: reverting Check() to `res, err := Run(dir)` (removing the CheckVersion probe) makes TestCheckWithFakeBinary/wrong_-version_fails_naming_the_mismatch fail — 'vulncheck_test.go:239: Check() = nil for a binary reporting a scanner version other than the pin, want an error' (exit 2). GREEN proven after restore: `go test -count=1 ./internal/vulncheck/` -> 'ok github.com/coding-hermes/boardctl/internal/vulncheck 2.966s'; subtests TestCheckWithFakeBinary 6/6 PASS (incl. wrong -version refused, correct -version proceeds, missing Scanner line skips), TestCheckVersionOffline 5/5 PASS, TestSameMinorLine PASS. Live proof on the installed binary: `/home/kara/go/bin/govulncheck -version` reports 'Scanner: govulncheck@v1.7.0' matching PinnedToolVersion (internal/vulncheck/vulncheck.go:62); TestVulncheck PASS (2.89s) and `make vuln-check` -> 'ok ... 2.776s'. Live refusal demonstrated end-to-end: GOVULNCHECK=/tmp/fakebin/govulncheck (self-reports v1.5.2) makes the real gate fail with 'govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /tmp/fakebin/govulncheck reports v1.5.2 — a scan by a different scanner rule set must not stand in for the pinned gate (fix: go install golang.org/x/vuln/cmd/govulncheck@v1.7.0)'. Supporting: Check() (vulncheck.go:369-380) probes via CheckVersion before RunBinary; MismatchErr.Error() names expected/actual/install hint; go build ./... OK, go vet OK, full `go test -count=1 ./...` all packages ok.
The vulncheck gate now refuses a govulncheck binary whose -version disagrees with the v1.7.0 pin, proven RED->GREEN by tests and live against the installed binary (and a disagreeing stand-in).

Overall: PASS ✓
