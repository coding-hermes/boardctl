# Changelog

## Unreleased

Changes for the next release will be recorded here.

## v0.4.0 — 2026-11-11

This release contains the graph-aware dispatch views and git-hook intake
surface added since v0.3.0. The release also restores the committed coverage
contract: `make cover-check` reports 80.2% total coverage with the existing
80.0% total and 60.0% per-package floors unchanged.

### Features

- Graph-aware `ready` and `direction` views expose dependency and initiative
  relationships in both human-readable and JSON output (`d6d9fa3`, `135d336`,
  `c7d394c`).
- BOARDCTL-HOOK-001 adds hook install, uninstall, doctor, dispatch and flush
  commands for git-hook intake, including spool and dead-letter handling
  (`2667a63`).

### Release preparation

- README install URLs and the release identity are pinned to `v0.4.0`.
- The release remains pending until exact-head CI and Multi-Arch are green and
  the tag-triggered publication has produced all seven platform assets plus
  `SHA256SUMS`.

## v0.3.0 — 2026-11-09

This release contains 53 commits since v0.2.0 (51 non-merge, 12 material),
summarized by the commit categories below. The cited short SHAs are taken
directly from the release range (`git log v0.2.0..HEAD --oneline`).

### Features

- `boardctl next-id` allocates the next free board id without a read-then-write
  race (`5a4fbda`).
- DEFER-EVIDENCE: a deferral now carries `defer_reason` and
  `defer_review_by`, the audit keys a deferral owes (`1cda136`).
- Initiative: a first-class grouping key so work can be scoped and rolled up
  across a board (`cbd817a`).
- Coverage floor gate (`make cover-check`) with reviewable floors — total 80%,
  per-package 60%, `internal/secretscheck` exempt (`56418f5`, `1a57984`,
  `0b820c2`).
- Canon: sanction the premise-probe key, which accounted for 52 of 55
  warnings on the code-factory board (`39eb2f7`).

### Fixes

- BOARD-GUARD-003: refuse a symbolic commit hash and validate evidence-run-id
  provenance before a row is accepted (`79ebc73`).
- Guard: raise `guards.hook_timeout` to 1020s so it sits above
  `test_timeout` (fail-open risk, BT-088) (`6cefd6c`).
- Lint: pin the golangci config to the repo root and gate vendored configs, so
  `go_lint` no longer aborts on `vendor/*.go` commits (BT-087) (`fec2b4a`).
- Coverage: honor the coverage-floor override in `cover-check` and drop the
  undocumented `MIN_PACKAGE_COVERAGE` override in `cmd/covercheck`
  (`d446cf2`, `d772169`).
- Generated artifacts (render fixture, tests) are deterministic and no longer
  churn the tree (BT-84) (`2a7ef7f`, `d7ea140`).
- Gitleaks: anchor the `sk-api-key` regex to a non-alnum boundary so mid-word
  matches stop raising false findings (BT-83) (`236bed7`).
- Tests: stop the suite pinning the host's temp-root spelling (`d051111`).

### Documentation

- `docs/cli`: name `--initiative`/`--clear-initiative` in both usage surfaces
  (`6268f6f`).
- Dependency-currency audit: the build set is current; graph residue is
  justified (`77b0b56`).
- README-health findings filed from a read-through of the install, release and
  command surfaces (`779325c`).

### Maintenance

- Board, QA, dogfood, specification and release-readiness evidence across the
  release range, including BT-083 through BT-102 filings, the 2026-11-08 QA
  battery, dogfood run 28 (DEFER-EVIDENCE + BOARD-GUARD-003), and the
  `[ci skip]` gitignore rescue classes (`037835f`, `9482755`, `abfa33d`,
  `fc60955`, `67328f1`, `f251101`, `39efb5b`).
- Board/harness bookkeeping commits are not listed individually.

## v0.2.0

This release contains 143 commits since v0.1.11, summarized by the commit
categories below. The cited short SHAs are taken directly from the release
range (`git log v0.1.11..HEAD --oneline`).

### Features

- Scoped status normalization and report verification counts (`e81a8c8`,
  `cd2eebf`).
- Tree evidence provenance (`9e3723e`).
- Acceptance-criteria writes through boardctl (`7d65737`).
- Degraded board reads (`5e66cd6`).

### Fixes

- Vendored dependencies for offline tests (`3e71129`).
- Release install-mode and missing-assets handling (`2d6d9d3`, `d2e38fb`,
  `1a5d1cd`).
- Atomic event repair and rewrite behavior (`0c3ef82`, `ef1fcd6`, `a5c3593`).
- Event-field preservation and normalize-board refusal diagnostics
  (`de3383d`, `8076308`).
- Go toolchain security pin (`fbcc6a4`).
- Tracked-cache test coverage and cleanup (`446baed`, `1a890b2`).

### Documentation

- Refined install evidence and dogfood records (`96fe1e3`, `5a4f1dd`).
- Documented degraded reads, command-surface drift, release-contract drift,
  and the Go toolchain floor (`882af6b`, `b04d88e`, `5db74b5`, `0b09c7c`,
  `09e00a2`, `ec5d086`, `9315266`, `7138683`).
- Documented board commands, repair behavior, fixture counts, header fields,
  update semantics, and event flags (`c1dd660`, `0a7a1c4`, `9eaf3e5`,
  `195eeba`, `c573466`, `a9d8357`, `8cec2a1`, `58e11d2`, `0b69cfe`).
- Added release/install documentation for v0.1.11 (`f6d54e6`, `7149ad0`).

### Maintenance

- Board, QA, dogfood, release-readiness, and foreman evidence updates across
  the release range, including `865c2f8`, `60f6b14`, `232ac56`, `dc97f8b`,
  `11ac9b0`, `d863d33`, and `e76c28d`.
- Security, dependency, and repository-hygiene maintenance, including
  `0804cd2`, `900b7b5`, `a9fdd69`, and `8de9710`.
- Merge and closeout maintenance for completed work, including `227494b`,
  `51d08ad`, `e9d50f3`, `102e8c0`, `75dfb45`, `baa17cf`, and `4468779`.
