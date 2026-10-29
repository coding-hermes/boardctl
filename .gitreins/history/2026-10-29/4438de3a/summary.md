# Verdict: QA-BOARDCTL-FOREMAN-3

**Task:** make install fails at Makefile:201 when dist/ is empty
**Evaluated:** 2026-10-29T09:25:31.237360
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: No implementation exists. Makefile:200-207 still hard-fails when dist/SHA256SUMS is absent: `@if [ ! -f "$(DIST)/SHA256SUMS" ]; then echo "install: $(DIST)/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first" >&2; ... exit 1; fi`. There is no source-build fallback in the install target (the only `go build` invocations are Makefile:43 in the `build` target and Makefile:132 in the `release` target). Empirical probe from a clean tree with no dist/ directory: `make install PREFIX=/tmp/probe3/prefix` printed 'install: dist/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first' followed by 'make: *** [Makefile:201: install] Error 1', EXIT=2, and nothing was installed to PREFIX — i.e. the exact reported bug at Makefile:201 is still present. `git diff` confirms the only changes are a timestamp bump in cmd/boardctl/board-report.html and deletion of .gitreins log files; the Makefile is untouched. Test suite for reference: `go test ./... -count=1` — all packages ok (cmd/boardctl 24.143s, internal/board, internal/versioncheck, etc.), but no test covers the required clean-clone install path.
The Makefile install target was never modified — `make install` still exits 2 at Makefile:201 when dist/SHA256SUMS is absent, so the reported bug remains unfixed.

## Summary

Judge Result: QA-BOARDCTL-FOREMAN-3

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: No implementation exists. Makefile:200-207 still hard-fails when dist/SHA256SUMS is absent: `@if [ ! -f "$(DIST)/SHA256SUMS" ]; then echo "install: $(DIST)/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first" >&2; ... exit 1; fi`. There is no source-build fallback in the install target (the only `go build` invocations are Makefile:43 in the `build` target and Makefile:132 in the `release` target). Empirical probe from a clean tree with no dist/ directory: `make install PREFIX=/tmp/probe3/prefix` printed 'install: dist/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first' followed by 'make: *** [Makefile:201: install] Error 1', EXIT=2, and nothing was installed to PREFIX — i.e. the exact reported bug at Makefile:201 is still present. `git diff` confirms the only changes are a timestamp bump in cmd/boardctl/board-report.html and deletion of .gitreins log files; the Makefile is untouched. Test suite for reference: `go test ./... -count=1` — all packages ok (cmd/boardctl 24.143s, internal/board, internal/versioncheck, etc.), but no test covers the required clean-clone install path.
The Makefile install target was never modified — `make install` still exits 2 at Makefile:201 when dist/SHA256SUMS is absent, so the reported bug remains unfixed.

Overall: FAIL ✗
