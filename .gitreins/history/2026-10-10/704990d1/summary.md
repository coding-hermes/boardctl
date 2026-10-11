# Verdict: DOC-23

**Task:** document install --fleet/--fleet-root/--lint-mode
**Evaluated:** 2026-10-10T19:06:50.100254
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README Board-lint section documents --fleet rollout (comma-separated glob-or-path, one outcome line per repo, refuses -C/--hook-path in fleet mode), --fleet-root, and --lint-mode enforce|warn (enforce default blocks commit; warn prints WARN-ONLY evidence and allows), matching install --help; validate passes: README.md:361-386 documents --fleet (comma-separated glob-or-path e.g. '/home/me/*' or 'axiom,crier', one outcome line per repo, -C/--hook-path refused as usage error), --fleet-root (default home dir), and --lint-mode enforce|warn (enforce default blocks commit; warn prints 'WARN-ONLY — commit allowed (boardctl validate failed)' and allows). Matches install --help output 'boardctl install [-C repo] [--hook-path P] [--timeout S] [--dry-run] [--fleet PAT] [--fleet-root DIR] [--lint-mode enforce|warn]' (cmd/boardctl/install.go:108). Implementation confirms: fleet.go:158-166 fleetRootDir defaults to home; install.go:122-124 refuses -C/--hook-path in fleet mode; install.go:75 boardLintWarnBanner. validate passes: 'RESULT: OK (3 warning(s))' exit 0. Tests: go test -count=1 ./cmd/boardctl/ -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 2.653s' exit 0.
README Board-lint section accurately documents --fleet/--fleet-root/--lint-mode matching install --help, validate passes, and the test suite is green.

## Summary

Judge Result: DOC-23

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README Board-lint section documents --fleet rollout (comma-separated glob-or-path, one outcome line per repo, refuses -C/--hook-path in fleet mode), --fleet-root, and --lint-mode enforce|warn (enforce default blocks commit; warn prints WARN-ONLY evidence and allows), matching install --help; validate passes: README.md:361-386 documents --fleet (comma-separated glob-or-path e.g. '/home/me/*' or 'axiom,crier', one outcome line per repo, -C/--hook-path refused as usage error), --fleet-root (default home dir), and --lint-mode enforce|warn (enforce default blocks commit; warn prints 'WARN-ONLY — commit allowed (boardctl validate failed)' and allows). Matches install --help output 'boardctl install [-C repo] [--hook-path P] [--timeout S] [--dry-run] [--fleet PAT] [--fleet-root DIR] [--lint-mode enforce|warn]' (cmd/boardctl/install.go:108). Implementation confirms: fleet.go:158-166 fleetRootDir defaults to home; install.go:122-124 refuses -C/--hook-path in fleet mode; install.go:75 boardLintWarnBanner. validate passes: 'RESULT: OK (3 warning(s))' exit 0. Tests: go test -count=1 ./cmd/boardctl/ -> 'ok github.com/coding-hermes/boardctl/cmd/boardctl 2.653s' exit 0.
README Board-lint section accurately documents --fleet/--fleet-root/--lint-mode matching install --help, validate passes, and the test suite is green.

Overall: PASS ✓
