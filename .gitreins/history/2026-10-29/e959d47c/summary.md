# Verdict: QA-BOARDCTL-FOREMAN-3

**Task:** finalize source-build make install fallback
**Evaluated:** 2026-10-29T09:50:37.264915
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Makefile has one install target, preserves SHA256SUMS missing-assets diagnostic in install-release, has one install -m 0755 release install, and contains TestDF12QuickstartSHA256SUMSPublishesBinaryAsset: Makefile:201 defines the single `install:` target (target-line grep shows only install:201, install-release:216, install-check:253 — no duplicate install target). install-release at Makefile:216-219 preserves the missing-assets diagnostic: `echo "install: $(DIST)/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first" >&2; exit 1`. Exactly one release install: `grep -c 'install -m 0755' Makefile` = 1, at Makefile:235 (`@install -m 0755 $(DIST)/$(ASSET) $(INSTALL_BIN)`); the source-build fallback in install (Makefile:202-208) uses cp+chmod, not install -m 0755. TestDF12QuickstartSHA256SUMSPublishesBinaryAsset is defined at cmd/boardctl/df12_readme_quickstart_test.go:274. Test evidence: `go test -count=1 -run TestDF12 ./cmd/boardctl/` => `ok github.com/coding-hermes/boardctl/cmd/boardctl 0.009s` (exit 0); full `go test -count=1 ./...` => all packages `ok`, exit 0; `go build ./...` exit 0.
Makefile has a single install target with source-build fallback, install-release retains the SHA256SUMS missing-assets diagnostic, exactly one install -m 0755 release install, and the DF12 SHA256SUMS test exists and the suite passes.

## Summary

Judge Result: QA-BOARDCTL-FOREMAN-3

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Makefile has one install target, preserves SHA256SUMS missing-assets diagnostic in install-release, has one install -m 0755 release install, and contains TestDF12QuickstartSHA256SUMSPublishesBinaryAsset: Makefile:201 defines the single `install:` target (target-line grep shows only install:201, install-release:216, install-check:253 — no duplicate install target). install-release at Makefile:216-219 preserves the missing-assets diagnostic: `echo "install: $(DIST)/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first" >&2; exit 1`. Exactly one release install: `grep -c 'install -m 0755' Makefile` = 1, at Makefile:235 (`@install -m 0755 $(DIST)/$(ASSET) $(INSTALL_BIN)`); the source-build fallback in install (Makefile:202-208) uses cp+chmod, not install -m 0755. TestDF12QuickstartSHA256SUMSPublishesBinaryAsset is defined at cmd/boardctl/df12_readme_quickstart_test.go:274. Test evidence: `go test -count=1 -run TestDF12 ./cmd/boardctl/` => `ok github.com/coding-hermes/boardctl/cmd/boardctl 0.009s` (exit 0); full `go test -count=1 ./...` => all packages `ok`, exit 0; `go build ./...` exit 0.
Makefile has a single install target with source-build fallback, install-release retains the SHA256SUMS missing-assets diagnostic, exactly one install -m 0755 release install, and the DF12 SHA256SUMS test exists and the suite passes.

Overall: FAIL ✗
