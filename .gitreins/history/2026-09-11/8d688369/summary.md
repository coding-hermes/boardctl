# Verdict: BT-017

**Task:** v0.1.2 release reproducibility + README verify flow
**Evaluated:** 2026-09-11T17:45:00.722502
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:44PM[0m [32mINF[0m [1mscanned ~325021 bytes (325.02 KB) in 217ms[0m
[90m12:44PM[0m [
- ✓ **tier2**
  - COMPLETE
  ✓ README checksum recipe passes verbatim (curl both files, sha256sum -c --ignore-missing -> OK); published v0.1.2 binaries report vcs.revision == the tagged commit with vcs.modified=false via go version -m; release assets re-uploaded and re-verified by download.: (1) README.md:17-21 recipe run verbatim in a clean dir: curl of boardctl-linux-amd64 and sha256sums.txt from .../releases/download/v0.1.2/ both returned http=200; `sha256sum -c --ignore-missing sha256sums.txt` printed 'boardctl-linux-amd64: OK' with exit=0; `chmod +x boardctl-linux-amd64 && ./boardctl-linux-amd64 version` printed 'boardctl version 20260911' exit=0. (2) `git rev-parse v0.1.2` == ac792795115d2ab5bc9f32d45d1d84bf18392b50; all 7 downloaded published assets (darwin-amd64/arm64, freebsd-amd64, linux-amd64/arm/arm64, windows-amd64.exe) report via `go version -m`: mod github.com/coding-hermes/boardctl v0.1.2, build vcs.revision=ac792795115d2ab5bc9f32d45d1d84bf18392b50, build vcs.modified=false. (3) All 7 assets re-downloaded from the v0.1.2 release and re-verified: `sha256sum -c --ignore-missing sha256sums.txt` -> all 7 OK, exit=0; published sha256sums.txt differs from the stale local dist/sha256sums.txt, confirming re-upload. (The stale local dist/ still holds the old dirty build v0.1.2+dirty/vcs.modified=true/revision 7785feb, but the published release assets are the clean re-upload and are what the criterion targets.)
README checksum recipe passes verbatim and all 7 published v0.1.2 release assets re-download, verify OK, and report vcs.revision == tagged commit ac792795 with vcs.modified=false.

## Summary

Judge Result: BT-017

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:44PM[0m [32mINF[0m [1mscanned ~325021 bytes (325.02 KB) in 217ms[0m
[90m12:44PM[0m [

Stage tier2: PASS
  COMPLETE
  ✓ README checksum recipe passes verbatim (curl both files, sha256sum -c --ignore-missing -> OK); published v0.1.2 binaries report vcs.revision == the tagged commit with vcs.modified=false via go version -m; release assets re-uploaded and re-verified by download.: (1) README.md:17-21 recipe run verbatim in a clean dir: curl of boardctl-linux-amd64 and sha256sums.txt from .../releases/download/v0.1.2/ both returned http=200; `sha256sum -c --ignore-missing sha256sums.txt` printed 'boardctl-linux-amd64: OK' with exit=0; `chmod +x boardctl-linux-amd64 && ./boardctl-linux-amd64 version` printed 'boardctl version 20260911' exit=0. (2) `git rev-parse v0.1.2` == ac792795115d2ab5bc9f32d45d1d84bf18392b50; all 7 downloaded published assets (darwin-amd64/arm64, freebsd-amd64, linux-amd64/arm/arm64, windows-amd64.exe) report via `go version -m`: mod github.com/coding-hermes/boardctl v0.1.2, build vcs.revision=ac792795115d2ab5bc9f32d45d1d84bf18392b50, build vcs.modified=false. (3) All 7 assets re-downloaded from the v0.1.2 release and re-verified: `sha256sum -c --ignore-missing sha256sums.txt` -> all 7 OK, exit=0; published sha256sums.txt differs from the stale local dist/sha256sums.txt, confirming re-upload. (The stale local dist/ still holds the old dirty build v0.1.2+dirty/vcs.modified=true/revision 7785feb, but the published release assets are the clean re-upload and are what the criterion targets.)
README checksum recipe passes verbatim and all 7 published v0.1.2 release assets re-download, verify OK, and report vcs.revision == tagged commit ac792795 with vcs.modified=false.

Overall: PASS ✓
