# Verdict: QA-BOARDCTL-FOREMAN-3

**Task:** make install fails at Makefile:201 when dist/ is empty
**Evaluated:** 2026-10-29T09:45:09.450667
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: Makefile:201-210 now branches on dist/SHA256SUMS. Clean-clone test (copied Makefile+cmd+internal+go.mod/go.sum to /tmp/insttest/repo with NO dist/): `make install INSTALL_DIR=/tmp/insttest/prefix` printed '== install: no release assets found; building from source', ran `$(MAKE) build`, and installed to PREFIX — EXIT=0, /tmp/insttest/prefix/boardctl created (11186007 bytes) and runs ('boardctl version dev'). Release-asset path preserved: with dist/boardctl_linux_amd64 + SHA256SUMS staged, `make install INSTALL_DIR=/tmp/insttest/prefix2` printed '== install: verifying dist/boardctl_linux_amd64 against dist/SHA256SUMS' / 'boardctl_linux_amd64: OK' / 'verified, installing to ...' — EXIT=0, binary installed. Checksum gate still sound: zeroed SHA256SUMS -> 'install: checksum gate FAILED ... no file was verified', make Error 2, nothing installed. install-release invoked directly still emits the missing-assets diagnostic (Makefile:216-219, exit 2). Test suite: `go test -count=1 ./...` -> GO_TEST_EXIT=0, 12 packages 'ok', no FAIL/panic. (Minor non-blocking: README lines 43-46 and 1705-1708 still say 'never builds from the checkout', now stale.)


## Summary

Judge Result: QA-BOARDCTL-FOREMAN-3

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ make install works from a clean clone without pre-downloaded release assets: if dist/SHA256SUMS is absent, build from source and install the built binary to PREFIX; preserve the existing verify+install path when dist/ has release assets: Makefile:201-210 now branches on dist/SHA256SUMS. Clean-clone test (copied Makefile+cmd+internal+go.mod/go.sum to /tmp/insttest/repo with NO dist/): `make install INSTALL_DIR=/tmp/insttest/prefix` printed '== install: no release assets found; building from source', ran `$(MAKE) build`, and installed to PREFIX — EXIT=0, /tmp/insttest/prefix/boardctl created (11186007 bytes) and runs ('boardctl version dev'). Release-asset path preserved: with dist/boardctl_linux_amd64 + SHA256SUMS staged, `make install INSTALL_DIR=/tmp/insttest/prefix2` printed '== install: verifying dist/boardctl_linux_amd64 against dist/SHA256SUMS' / 'boardctl_linux_amd64: OK' / 'verified, installing to ...' — EXIT=0, binary installed. Checksum gate still sound: zeroed SHA256SUMS -> 'install: checksum gate FAILED ... no file was verified', make Error 2, nothing installed. install-release invoked directly still emits the missing-assets diagnostic (Makefile:216-219, exit 2). Test suite: `go test -count=1 ./...` -> GO_TEST_EXIT=0, 12 packages 'ok', no FAIL/panic. (Minor non-blocking: README lines 43-46 and 1705-1708 still say 'never builds from the checkout', now stale.)


Overall: FAIL ✗
