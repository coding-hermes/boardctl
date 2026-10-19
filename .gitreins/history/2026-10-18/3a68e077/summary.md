# Verdict: DF-BOARDCTL-39

**Task:** README normalize-board docs match release v0.1.11
**Evaluated:** 2026-10-18T14:49:53.171099
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ v0.1.11 release binary accepts normalize-board; README install block points to v0.1.11; quickstart verified end-to-end on release asset: Release asset ./boardctl_linux_amd64: `version` prints `boardctl version v0.1.11` (exit 0); `go version -m` shows mod v0.1.11, vcs.revision=3f3dd16ce3590d28a3438461ffd0166625eefb92, vcs.modified=false (matches README:1264/1272 expectations). `normalize-board --help` exits 0 and lists the documented flags (--apply/--repair-events/--json/--skip-bad-lines). End-to-end quickstart on the release asset against a fresh board (/tmp/qs with board.jsonl+tasks.jsonl+events.jsonl): dry-run exit 0 staging tasks.normalized.jsonl, `--apply` exit 0 rewriting tasks.jsonl to {"status":"pending","depends_on":[],"acceptance_criteria":"x"}, `--json` exit 0 — matching README:753-755 and the documented repairs/notes. README install block README.md:17-21 points to https://github.com/coding-hermes/boardctl/releases/download/v0.1.11/boardctl_linux_amd64 and SHA256SUMS; no remaining 'unreleased' markers (README:978, 1016 now say 'Since v0.1.11'); install-check table bumped to v0.1.11/3f3dd16 (README:1259-1272). Old dist/boardctl_linux_amd64 is v0.1.10 and rejects normalize-board, but it is a stale untracked build artifact, not the release asset. `go test -count=1 ./...` exit 0, all packages ok (cmd/boardctl 5.809s, internal/board 1.279s, etc.).
Release v0.1.11 binary accepts normalize-board and the documented quickstart works end-to-end; README install block and version references point to v0.1.11, and the full Go test suite passes.

## Summary

Judge Result: DF-BOARDCTL-39

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ v0.1.11 release binary accepts normalize-board; README install block points to v0.1.11; quickstart verified end-to-end on release asset: Release asset ./boardctl_linux_amd64: `version` prints `boardctl version v0.1.11` (exit 0); `go version -m` shows mod v0.1.11, vcs.revision=3f3dd16ce3590d28a3438461ffd0166625eefb92, vcs.modified=false (matches README:1264/1272 expectations). `normalize-board --help` exits 0 and lists the documented flags (--apply/--repair-events/--json/--skip-bad-lines). End-to-end quickstart on the release asset against a fresh board (/tmp/qs with board.jsonl+tasks.jsonl+events.jsonl): dry-run exit 0 staging tasks.normalized.jsonl, `--apply` exit 0 rewriting tasks.jsonl to {"status":"pending","depends_on":[],"acceptance_criteria":"x"}, `--json` exit 0 — matching README:753-755 and the documented repairs/notes. README install block README.md:17-21 points to https://github.com/coding-hermes/boardctl/releases/download/v0.1.11/boardctl_linux_amd64 and SHA256SUMS; no remaining 'unreleased' markers (README:978, 1016 now say 'Since v0.1.11'); install-check table bumped to v0.1.11/3f3dd16 (README:1259-1272). Old dist/boardctl_linux_amd64 is v0.1.10 and rejects normalize-board, but it is a stale untracked build artifact, not the release asset. `go test -count=1 ./...` exit 0, all packages ok (cmd/boardctl 5.809s, internal/board 1.279s, etc.).
Release v0.1.11 binary accepts normalize-board and the documented quickstart works end-to-end; README install block and version references point to v0.1.11, and the full Go test suite passes.

Overall: PASS ✓
