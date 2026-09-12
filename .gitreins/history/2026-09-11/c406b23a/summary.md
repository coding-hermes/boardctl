# Verdict: BT-018

**Task:** Narrow gitleaks allowlist so docs and specs are scanned
**Evaluated:** 2026-09-11T23:58:47.432436
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m6:58PM[0m [32mINF[0m [1mscanned ~359952 bytes (359.95 KB) in 250ms[0m
[90m6:58PM[0m [32
- ✓ **tier2**
  - COMPLETE
  ✓ Remove broad specs/, docs/, .*\.spec\.md, and .*\.md path allowlists; preserve only generated/dependency/audit paths; gitleaks detects an sk-shaped secret in a temporary Markdown fixture; full repository gitleaks scan, go build, go vet, go test, and GitReins guard pass.: Commit 771b2f4 diff removes '''specs/''', '''docs/''', '''.*\.spec\.md''', '''.*\.md''' from [allowlist].paths in .gitleaks.toml; remaining paths are only '''\.git/''', '''\.gitreins/''', '''\.gitreins/history/''', '''.*\.log''', '''vendor/''' (generated/dependency/audit). Fixture test: temp repo with fixture.md containing 'sk-proj-Xk9mQ2vL8nR4tY6wZ1aB3cD5eF7gH0jK' scanned with repo config yields report RuleID=sk-api-key, File=fixture.md (leaks found: 1); the OLD config (with .*\.md allowlist) scanned ~0 bytes and reported no leaks, proving the narrowing is what enables detection. Full repo scan: `gitleaks detect --source . --config .gitleaks.toml` -> '136 commits scanned', 'scanned ~1034384 bytes (1.03 MB)', 'no leaks found', exit 0. `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.347s' and 'ok github.com/coding-hermes/boardctl/internal/board 0.073s', exit 0. `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full)' with secrets clean, go_build ok, go_lint ok, go_tests, exit 0.
The gitleaks allowlist was narrowed to only generated/dependency/audit paths, gitleaks now detects an sk-shaped secret in a Markdown fixture, and the full repo scan, go build, go vet, go test, and GitReins guard all pass.

## Summary

Judge Result: BT-018

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m6:58PM[0m [32mINF[0m [1mscanned ~359952 bytes (359.95 KB) in 250ms[0m
[90m6:58PM[0m [32

Stage tier2: PASS
  COMPLETE
  ✓ Remove broad specs/, docs/, .*\.spec\.md, and .*\.md path allowlists; preserve only generated/dependency/audit paths; gitleaks detects an sk-shaped secret in a temporary Markdown fixture; full repository gitleaks scan, go build, go vet, go test, and GitReins guard pass.: Commit 771b2f4 diff removes '''specs/''', '''docs/''', '''.*\.spec\.md''', '''.*\.md''' from [allowlist].paths in .gitleaks.toml; remaining paths are only '''\.git/''', '''\.gitreins/''', '''\.gitreins/history/''', '''.*\.log''', '''vendor/''' (generated/dependency/audit). Fixture test: temp repo with fixture.md containing 'sk-proj-Xk9mQ2vL8nR4tY6wZ1aB3cD5eF7gH0jK' scanned with repo config yields report RuleID=sk-api-key, File=fixture.md (leaks found: 1); the OLD config (with .*\.md allowlist) scanned ~0 bytes and reported no leaks, proving the narrowing is what enables detection. Full repo scan: `gitleaks detect --source . --config .gitleaks.toml` -> '136 commits scanned', 'scanned ~1034384 bytes (1.03 MB)', 'no leaks found', exit 0. `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.347s' and 'ok github.com/coding-hermes/boardctl/internal/board 0.073s', exit 0. `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full)' with secrets clean, go_build ok, go_lint ok, go_tests, exit 0.
The gitleaks allowlist was narrowed to only generated/dependency/audit paths, gitleaks now detects an sk-shaped secret in a Markdown fixture, and the full repo scan, go build, go vet, go test, and GitReins guard all pass.

Overall: PASS ✓
