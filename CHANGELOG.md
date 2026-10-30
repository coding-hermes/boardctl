# Changelog

## Unreleased

Changes for the next release will be recorded here.

## v0.2.0

This release contains 119 commits since v0.1.11, summarized by the commit
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
