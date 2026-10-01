# Verdict: DF-BOARDCTL-20

**Task:** vulncheck version pin enforcement
**Evaluated:** 2026-09-29T10:40:24.236884
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.239s
- ✓ **tier2**
  - COMPLETE
  ✓ make vuln-check refuses a binary whose -version disagrees with PinnedToolVersion; tests pass: internal/vulncheck/vulncheck.go:62 pins PinnedToolVersion="v1.7.0"; CheckVersion (:151) probes `<bin> -version`, parses the "Scanner: govulncheck@X" line (regex :135) and returns *MismatchErr when it differs; Check (:381) runs CheckVersion before scanning and fails the gate. Live proof: with a fake binary reporting v1.5.2, `GOVULNCHECK=/tmp/fakebin/govulncheck make vuln-check` exited 2 with "dependency-vulnerability gate refused: govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /tmp/fakebin/govulncheck reports v1.5.2"; v1.7.3 (patch drift) also refused (exit 2); v1.7.0 -> exit 0. Tests: `make vuln-check` exit 0 ("ok github.com/coding-hermes/boardctl/internal/vulncheck 2.690s") and `go test -count=1 ./...` exit 0 (all packages ok); verbose run shows PASS for TestVulncheck, TestCheckWithFakeBinary/wrong_-version_fails_naming_the_mismatch, /correct_-version_proceeds_to_the_scan, /missing_-version_skips_the_check_and_the_scan_decides, TestCheckVersionOffline, TestSameMinorLine. CI parity: .github/workflows/ci.yml:45 installs govulncheck@v1.7.0 and :48 runs the identical command.
make vuln-check refuses a govulncheck binary whose -version disagrees with PinnedToolVersion (v1.7.0) and the full test suite passes.

## Summary

Judge Result: DF-BOARDCTL-20

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.239s

Stage tier2: PASS
  COMPLETE
  ✓ make vuln-check refuses a binary whose -version disagrees with PinnedToolVersion; tests pass: internal/vulncheck/vulncheck.go:62 pins PinnedToolVersion="v1.7.0"; CheckVersion (:151) probes `<bin> -version`, parses the "Scanner: govulncheck@X" line (regex :135) and returns *MismatchErr when it differs; Check (:381) runs CheckVersion before scanning and fails the gate. Live proof: with a fake binary reporting v1.5.2, `GOVULNCHECK=/tmp/fakebin/govulncheck make vuln-check` exited 2 with "dependency-vulnerability gate refused: govulncheck version mismatch: this repo's gates pin v1.7.0 but the resolved binary /tmp/fakebin/govulncheck reports v1.5.2"; v1.7.3 (patch drift) also refused (exit 2); v1.7.0 -> exit 0. Tests: `make vuln-check` exit 0 ("ok github.com/coding-hermes/boardctl/internal/vulncheck 2.690s") and `go test -count=1 ./...` exit 0 (all packages ok); verbose run shows PASS for TestVulncheck, TestCheckWithFakeBinary/wrong_-version_fails_naming_the_mismatch, /correct_-version_proceeds_to_the_scan, /missing_-version_skips_the_check_and_the_scan_decides, TestCheckVersionOffline, TestSameMinorLine. CI parity: .github/workflows/ci.yml:45 installs govulncheck@v1.7.0 and :48 runs the identical command.
make vuln-check refuses a govulncheck binary whose -version disagrees with PinnedToolVersion (v1.7.0) and the full test suite passes.

Overall: PASS ✓
