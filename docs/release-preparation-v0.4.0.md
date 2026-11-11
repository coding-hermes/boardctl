# v0.4.0 release preparation

This artifact records the release-facing preparation for RELEASE-2026-11-09-07. It does not claim that the tag or hosted release exists.

## Prepared surfaces

- README.md is pinned to v0.4.0 in the release identity line and both download URLs.
- CHANGELOG.md contains a v0.4.0 section naming the graph-aware views, hook intake, unchanged coverage floors, and the remaining publication gate.
- cmd/boardctl/readme_acceptance_criteria_release_test.go asserts the v0.4.0 README contract.
- Makefile retains COVER_MIN_TOTAL=80 and COVER_MIN_PACKAGE=60.

## Release gate status

- Local coverage baseline at the published base was reported as 80.2% total; the floors remain 80.0% total and 60.0% per package.
- The v0.4.0 tag was not created by this worker.
- Hosted exact-head CI, Multi-Arch publication, seven release assets, SHA256SUMS, and buildinfo verification remain foreman-owned acceptance clauses and are not asserted here.

ch:trace row=RELEASE-2026-11-09-07 spec=.coding-hermes/board/tasks.jsonl#RELEASE-2026-11-09-07 wave=none:serial-worker test=./cmd/boardctl::TestREADMEAcceptanceCriteriaPublishedReleaseContract doc=docs/release-preparation-v0.4.0.md evidence=docs/release-preparation-v0.4.0.md witness=none:worker-local verdict=none:foreman-judge commit=none:worker-commit memory=none:worker-local
