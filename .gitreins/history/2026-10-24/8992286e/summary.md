# Verdict: README-010

**Task:** P4 README and CI Go toolchain patch-floor parity
**Evaluated:** 2026-10-24T15:31:14.177499
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README.md, go.mod, and .github/workflows/ci.yml express one verified Go 1.26.10 policy; setup-go uses go.mod as source of truth; documented grep and toolchain/build probe pass.: All three surfaces agree on 1.26.10: go.mod:3 `go 1.26.10`; .github/workflows/ci.yml:18-20 `actions/setup-go@v5` with `go-version-file: go.mod` and no `go-version:`/matrix selector; README.md:1493-1521 (anchor #go-toolchain-policy) states "`go.mod` declares `go 1.26.10`", documents `go-version-file: go.mod`, and gives the probe `go version` + `go build ./...`. Enforcement exists in code: internal/workflowcheck/workflowcheck.go:248-281 CheckToolchainParity parses the exact 3-component go directive, requires the README phrase, requires `go-version-file: go.mod` in both README and workflow, and rejects any `go-version:`/`matrix.go-version`. Verified by running: `go test -count=1 -run TestToolchainParity -v ./internal/workflowcheck` -> `--- PASS: TestToolchainParityWiredInRepo`, `--- PASS: TestToolchainParityRejectsMovingWorkflow`, `ok github.com/coding-hermes/boardctl/internal/workflowcheck 0.004s` (exit 0). Probe: `go version` -> `go version go1.26.10 linux/amd64`; `go build ./...` exit_code=0; `go vet ./...` exit_code=0; `go test -short -count=1 ./...` exit_code=0 with every package `ok` (cmd/boardctl, internal/board, internal/workflowcheck, internal/vulncheck, etc.).
README, go.mod, and ci.yml all pin the same Go 1.26.10 patch floor with setup-go reading go.mod, enforced by CheckToolchainParity, and the parity tests, go version, go build, go vet, and the full -short suite all pass.

## Summary

Judge Result: README-010

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README.md, go.mod, and .github/workflows/ci.yml express one verified Go 1.26.10 policy; setup-go uses go.mod as source of truth; documented grep and toolchain/build probe pass.: All three surfaces agree on 1.26.10: go.mod:3 `go 1.26.10`; .github/workflows/ci.yml:18-20 `actions/setup-go@v5` with `go-version-file: go.mod` and no `go-version:`/matrix selector; README.md:1493-1521 (anchor #go-toolchain-policy) states "`go.mod` declares `go 1.26.10`", documents `go-version-file: go.mod`, and gives the probe `go version` + `go build ./...`. Enforcement exists in code: internal/workflowcheck/workflowcheck.go:248-281 CheckToolchainParity parses the exact 3-component go directive, requires the README phrase, requires `go-version-file: go.mod` in both README and workflow, and rejects any `go-version:`/`matrix.go-version`. Verified by running: `go test -count=1 -run TestToolchainParity -v ./internal/workflowcheck` -> `--- PASS: TestToolchainParityWiredInRepo`, `--- PASS: TestToolchainParityRejectsMovingWorkflow`, `ok github.com/coding-hermes/boardctl/internal/workflowcheck 0.004s` (exit 0). Probe: `go version` -> `go version go1.26.10 linux/amd64`; `go build ./...` exit_code=0; `go vet ./...` exit_code=0; `go test -short -count=1 ./...` exit_code=0 with every package `ok` (cmd/boardctl, internal/board, internal/workflowcheck, internal/vulncheck, etc.).
README, go.mod, and ci.yml all pin the same Go 1.26.10 patch floor with setup-go reading go.mod, enforced by CheckToolchainParity, and the parity tests, go version, go build, go vet, and the full -short suite all pass.

Overall: PASS ✓
