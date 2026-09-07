# Verdict: BT-008

**Task:** v0.1.1 release missing sha256sums.txt — regenerate + upload + README checksum guidance
**Evaluated:** 2026-09-05T00:22:21.686534
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m7:21PM[0m [32mINF[0m [1mscanned ~205008 bytes (205.01 KB) in 95.7ms[0m
[90m7:21PM[0m [3
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
- ✓ **tier2**
  - COMPLETE
  ✓ sha256sums.txt appears as a v0.1.1 release asset, with checksums computed from the exact published binaries (download the 7 assets and hash them, matching v0.1.0's format with bare filenames); 'gh release view v0.1.1' lists the asset; curl -sL <release>/sha256sums.txt works and each checksum verifies against its downloaded binary via sha256sum -c; README Install section shows curl + sha256sum -c verification for the static-binary path; a release checklist note stating 'make release is the only sanctioned cut path' is added; go build ./..., go vet ./..., go test ./... -count=1 -short all pass: gh release view v0.1.1 lists asset sha256sums.txt + 7 binaries. curl -sL <release>/sha256sums.txt downloaded OK; content uses bare filenames matching v0.1.0's format (verified v0.1.0 file). Downloaded all 7 binaries and `sha256sum -c sha256sums.txt` returned OK for all 7 (darwin-amd64, darwin-arm64, freebsd-amd64, linux-amd64, linux-arm, linux-arm64, windows-amd64.exe). README.md (HEAD 592fa35) Install section shows `curl -sL -o boardctl .../boardctl-linux-amd64`, `curl -sL -o sha256sums.txt .../sha256sums.txt`, `sha256sum -c sha256sums.txt`; Development section line 125: '`make release` is the only sanctioned path for cutting a release'. go build ./... exit 0; go vet ./... exit 0; go test ./... -count=1 -short exit 0 (ok cmd/boardctl, ok internal/board).


## Summary

Judge Result: BT-008

Stage tier1: PASS
    ✓ secrets: [90m7:21PM[0m [32mINF[0m [1mscanned ~205008 bytes (205.01 KB) in 95.7ms[0m
[90m7:21PM[0m [3
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/

Stage tier2: PASS
  COMPLETE
  ✓ sha256sums.txt appears as a v0.1.1 release asset, with checksums computed from the exact published binaries (download the 7 assets and hash them, matching v0.1.0's format with bare filenames); 'gh release view v0.1.1' lists the asset; curl -sL <release>/sha256sums.txt works and each checksum verifies against its downloaded binary via sha256sum -c; README Install section shows curl + sha256sum -c verification for the static-binary path; a release checklist note stating 'make release is the only sanctioned cut path' is added; go build ./..., go vet ./..., go test ./... -count=1 -short all pass: gh release view v0.1.1 lists asset sha256sums.txt + 7 binaries. curl -sL <release>/sha256sums.txt downloaded OK; content uses bare filenames matching v0.1.0's format (verified v0.1.0 file). Downloaded all 7 binaries and `sha256sum -c sha256sums.txt` returned OK for all 7 (darwin-amd64, darwin-arm64, freebsd-amd64, linux-amd64, linux-arm, linux-arm64, windows-amd64.exe). README.md (HEAD 592fa35) Install section shows `curl -sL -o boardctl .../boardctl-linux-amd64`, `curl -sL -o sha256sums.txt .../sha256sums.txt`, `sha256sum -c sha256sums.txt`; Development section line 125: '`make release` is the only sanctioned path for cutting a release'. go build ./... exit 0; go vet ./... exit 0; go test ./... -count=1 -short exit 0 (ok cmd/boardctl, ok internal/board).


Overall: PASS ✓
