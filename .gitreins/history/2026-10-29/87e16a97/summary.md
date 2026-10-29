# Verdict: QA-BOARDCTL-FOREMAN-3

**Task:** make install fails at Makefile:201 when dist/ is empty
**Evaluated:** 2026-10-29T09:35:12.938478
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: Makefile install target (lines 201-210) branches on `[ ! -f "$(DIST)/SHA256SUMS" ]`: absent -> `$(MAKE) build` then `install -m 0755 "bin/$(BINARY)" "$(INSTALL_BIN)"`; present -> `$(MAKE) install-release` (verify+install preserved at lines 213-230). Verified empirically in /home/kara/coding-hermes-boardctl @1a5d1cd: (A) dist/ emptied -> 'make install INSTALL_DIR=/tmp/qa3-prefix' printed '== install: no release assets found; building from source', ran go build, installed 11MB binary, MAKE_EXIT=0, `boardctl version` -> dev; (D) clean clone via `git archive HEAD | tar -x` into /tmp/qa3-clone (no dist/ dir at all) -> MAKE_EXIT=0, /tmp/qa3-p5/boardctl installed and executable; (B2) valid asset+SHA256SUMS -> 'boardctl_linux_amd64: OK' then installed the asset (verify+install path intact); (C) mismatched asset -> checksum gate still fails loudly (exit 2), gate not weakened. `go test -count=1 ./...` -> GO_TEST_EXIT=0, 12 packages ok, no FAIL. Minor stale doc: README.md:1705 still claims 'make install never builds from the checkout'.
make install now falls back to a source build and installs to PREFIX when dist/SHA256SUMS is absent (verified from a clean git-archive clone, exit 0), while the checksum-verified release-asset path is preserved and still fails loudly on mismatch.

## Summary

Judge Result: QA-BOARDCTL-FOREMAN-3

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: Makefile install target (lines 201-210) branches on `[ ! -f "$(DIST)/SHA256SUMS" ]`: absent -> `$(MAKE) build` then `install -m 0755 "bin/$(BINARY)" "$(INSTALL_BIN)"`; present -> `$(MAKE) install-release` (verify+install preserved at lines 213-230). Verified empirically in /home/kara/coding-hermes-boardctl @1a5d1cd: (A) dist/ emptied -> 'make install INSTALL_DIR=/tmp/qa3-prefix' printed '== install: no release assets found; building from source', ran go build, installed 11MB binary, MAKE_EXIT=0, `boardctl version` -> dev; (D) clean clone via `git archive HEAD | tar -x` into /tmp/qa3-clone (no dist/ dir at all) -> MAKE_EXIT=0, /tmp/qa3-p5/boardctl installed and executable; (B2) valid asset+SHA256SUMS -> 'boardctl_linux_amd64: OK' then installed the asset (verify+install path intact); (C) mismatched asset -> checksum gate still fails loudly (exit 2), gate not weakened. `go test -count=1 ./...` -> GO_TEST_EXIT=0, 12 packages ok, no FAIL. Minor stale doc: README.md:1705 still claims 'make install never builds from the checkout'.
make install now falls back to a source build and installs to PREFIX when dist/SHA256SUMS is absent (verified from a clean git-archive clone, exit 0), while the checksum-verified release-asset path is preserved and still fails loudly on mismatch.

Overall: FAIL ✗
