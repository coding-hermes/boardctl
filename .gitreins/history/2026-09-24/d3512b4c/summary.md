# Verdict: BT-052

**Task:** Pin reusable multiarch workflow ref and gate platform-list drift
**Evaluated:** 2026-09-24T11:58:30.145588
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.971s
- ✓ **tier2**
  - COMPLETE
  ✓ multiarch.yml calls a pinned immutable ref (tag or SHA) recorded beside the platform list; a test fails on Makefile PLATFORMS vs workflow platforms input drift; both triggers (push main + v* tag) proven by captured run evidence; build/vet/test green: Pin: .github/workflows/multiarch.yml:22 `uses: coding-hermes/.github/.github/workflows/go-multiarch.yml@917f3216b505a2decefd8d7878b22d24fbf2a89e` (full 40-hex SHA, not @main), with pin comment 'Pinned 2026-09-24 to 917f3216...' at lines 16-21 immediately beside the platforms input (line 33). TestRefPinned (workflowcheck_test.go) enforces SHA-or-vX.Y.Z and requires the date comment. Drift gate: internal/workflowcheck TestWorkflowcheck/TestPlatformDriftDetected/TestPlatformOrderDriftDetected/TestDuplicatePlatformFails; live RED proof reproduced — sed Makefile freebsd/amd64->freebsd/amd999 then `go test -run TestWorkflowcheck ./internal/workflowcheck/` => FAIL 'platform drift: workflow platforms [linux/amd64 ... freebsd/amd64] != Makefile PLATFORMS [... freebsd/amd999]' (restored, git diff clean). Triggers: multiarch.yml lines 4-9 `push: branches: [main]` + `tags: ["v*"]`; captured run evidence in /tmp/w-bt052.log: main-push run 35988430750 (event=push, headBranch=main, success) and tag run 35829487898 (ref v0.1.8, headSha a3328d7, success) publishing 7 platform assets + SHA256SUMS. Build/vet/test: `go build ./...` => BUILD_OK; `go vet ./...` => VET_OK; `go test -count=1 ./...` => all ok (cmd/boardctl, internal/board, fmtcheck, render, versioncheck, vulncheck, workflowcheck).
Pinned SHA ref with documented pin comment, a live-RED-proven platform-drift gate, both triggers with captured run evidence, and green build/vet/test all verified.

## Summary

Judge Result: BT-052

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.971s

Stage tier2: PASS
  COMPLETE
  ✓ multiarch.yml calls a pinned immutable ref (tag or SHA) recorded beside the platform list; a test fails on Makefile PLATFORMS vs workflow platforms input drift; both triggers (push main + v* tag) proven by captured run evidence; build/vet/test green: Pin: .github/workflows/multiarch.yml:22 `uses: coding-hermes/.github/.github/workflows/go-multiarch.yml@917f3216b505a2decefd8d7878b22d24fbf2a89e` (full 40-hex SHA, not @main), with pin comment 'Pinned 2026-09-24 to 917f3216...' at lines 16-21 immediately beside the platforms input (line 33). TestRefPinned (workflowcheck_test.go) enforces SHA-or-vX.Y.Z and requires the date comment. Drift gate: internal/workflowcheck TestWorkflowcheck/TestPlatformDriftDetected/TestPlatformOrderDriftDetected/TestDuplicatePlatformFails; live RED proof reproduced — sed Makefile freebsd/amd64->freebsd/amd999 then `go test -run TestWorkflowcheck ./internal/workflowcheck/` => FAIL 'platform drift: workflow platforms [linux/amd64 ... freebsd/amd64] != Makefile PLATFORMS [... freebsd/amd999]' (restored, git diff clean). Triggers: multiarch.yml lines 4-9 `push: branches: [main]` + `tags: ["v*"]`; captured run evidence in /tmp/w-bt052.log: main-push run 35988430750 (event=push, headBranch=main, success) and tag run 35829487898 (ref v0.1.8, headSha a3328d7, success) publishing 7 platform assets + SHA256SUMS. Build/vet/test: `go build ./...` => BUILD_OK; `go vet ./...` => VET_OK; `go test -count=1 ./...` => all ok (cmd/boardctl, internal/board, fmtcheck, render, versioncheck, vulncheck, workflowcheck).
Pinned SHA ref with documented pin comment, a live-RED-proven platform-drift gate, both triggers with captured run evidence, and green build/vet/test all verified.

Overall: PASS ✓
