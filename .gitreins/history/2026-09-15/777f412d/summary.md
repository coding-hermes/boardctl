# Verdict: BT-028

**Task:** gofmt is not gated: formatting rot is invisible to CI while vet stays green
**Evaluated:** 2026-09-15T11:43:27.249982
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m6:42AM[0m [32mINF[0m [1mscanned ~844700 bytes (844.70 KB) in 586ms[0m
[90m6:42AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.791s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ Makefile exposes a check-only gofmt gate target; .github/workflows/ci.yml runs the same gate as a failing step; a Go test in the repo runs gofmt and fails on any unformatted .go file so the local gate and CI agree. Live probe: with a deliberately unformatted temp .go file inside the repo the gate exits non-zero, and on the clean tree it exits 0 with empty gofmt -l output.: Makefile:26-27 defines check-only target `fmt-check: go test -count=1 -run TestGofmt ./internal/fmtcheck` (listed in .PHONY at Makefile:12; the writing helper is `fmt` = gofmt -w, so the gate never rewrites). .github/workflows/ci.yml:32-33 has `- name: Gofmt` / `run: go test -count=1 -run TestGofmt ./internal/fmtcheck` — byte-identical to the Makefile target — and grep for `continue-on-error`/`if:` in ci.yml returned none, so it is a genuinely failing step. internal/fmtcheck/fmtcheck_test.go TestGofmt calls RepoRoot(cwd)+FindUnformatted(root); fmtcheck.go compares go/format.Source output to on-disk bytes (the same criterion as gofmt -l) and t.Errorf's on any unformatted file, with no testing.Short() skip so it also runs under CI's `go test -short -count=1 ./...`. LIVE PROBE at /home/kara/coding-hermes-boardctl @ 5ff8e2e: clean tree `make fmt-check` -> `ok github.com/coding-hermes/boardctl/internal/fmtcheck 0.033s`, exit=0, and `gofmt -l ./cmd ./internal` -> empty output, exit=0; full `go test -count=1 ./...` -> all 4 packages ok, exit=0. Negative probe with deliberately unformatted internal/fmtcheck/zz_probe_tmp.go: `gofmt -l ./cmd ./internal` -> `internal/fmtcheck/zz_probe_tmp.go`; `make fmt-check` -> `--- FAIL: TestGofmt ... found 1 file(s) not gofmt-clean ... internal/fmtcheck/zz_probe_tmp.go`, exit=2 (`make: *** [Makefile:27: fmt-check] Error 1`); `go test -short -count=1 ./...` -> FAIL on internal/fmtcheck. Temp file removed and `git status --porcelain` clean afterward. Local gate and CI agree by construction via the single shared checker.
gofmt is now gated identically in the Makefile, CI, and go test via one shared checker; live probes confirm exit 0 with empty gofmt -l on the clean tree and non-zero on a deliberately unformatted file.

## Summary

Judge Result: BT-028

Stage tier1: PASS
    ✓ secrets: [90m6:42AM[0m [32mINF[0m [1mscanned ~844700 bytes (844.70 KB) in 586ms[0m
[90m6:42AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.791s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ Makefile exposes a check-only gofmt gate target; .github/workflows/ci.yml runs the same gate as a failing step; a Go test in the repo runs gofmt and fails on any unformatted .go file so the local gate and CI agree. Live probe: with a deliberately unformatted temp .go file inside the repo the gate exits non-zero, and on the clean tree it exits 0 with empty gofmt -l output.: Makefile:26-27 defines check-only target `fmt-check: go test -count=1 -run TestGofmt ./internal/fmtcheck` (listed in .PHONY at Makefile:12; the writing helper is `fmt` = gofmt -w, so the gate never rewrites). .github/workflows/ci.yml:32-33 has `- name: Gofmt` / `run: go test -count=1 -run TestGofmt ./internal/fmtcheck` — byte-identical to the Makefile target — and grep for `continue-on-error`/`if:` in ci.yml returned none, so it is a genuinely failing step. internal/fmtcheck/fmtcheck_test.go TestGofmt calls RepoRoot(cwd)+FindUnformatted(root); fmtcheck.go compares go/format.Source output to on-disk bytes (the same criterion as gofmt -l) and t.Errorf's on any unformatted file, with no testing.Short() skip so it also runs under CI's `go test -short -count=1 ./...`. LIVE PROBE at /home/kara/coding-hermes-boardctl @ 5ff8e2e: clean tree `make fmt-check` -> `ok github.com/coding-hermes/boardctl/internal/fmtcheck 0.033s`, exit=0, and `gofmt -l ./cmd ./internal` -> empty output, exit=0; full `go test -count=1 ./...` -> all 4 packages ok, exit=0. Negative probe with deliberately unformatted internal/fmtcheck/zz_probe_tmp.go: `gofmt -l ./cmd ./internal` -> `internal/fmtcheck/zz_probe_tmp.go`; `make fmt-check` -> `--- FAIL: TestGofmt ... found 1 file(s) not gofmt-clean ... internal/fmtcheck/zz_probe_tmp.go`, exit=2 (`make: *** [Makefile:27: fmt-check] Error 1`); `go test -short -count=1 ./...` -> FAIL on internal/fmtcheck. Temp file removed and `git status --porcelain` clean afterward. Local gate and CI agree by construction via the single shared checker.
gofmt is now gated identically in the Makefile, CI, and go test via one shared checker; live probes confirm exit 0 with empty gofmt -l on the clean tree and non-zero on a deliberately unformatted file.

Overall: PASS ✓
