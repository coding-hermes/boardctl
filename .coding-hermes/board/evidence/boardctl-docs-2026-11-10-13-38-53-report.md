FINDINGS
Tick: boardctl-docs-2026-11-10-13-38-53
Target revision audited: 13fb3708036972a3eabdf828beb152f8f8c3c823
Frozen exact-output evidence: .coding-hermes/board/evidence/boardctl-docs-2026-11-10-13-38-53-frozen.md
Frozen evidence sha256: 256664ce16b5d7e4a23a76e8d4a6343135de63ce869b950ebb13d02814e5ea0b

DOC-41 — P4 — document ready and direction for operators in the README front door
Evidence: source registers ready/direction at cmd/boardctl/main.go:54-55 and implements flags/help at cmd/boardctl/graph_cmd.go:91-102,225-232. Fresh binary probes returned ready now: 25 row(s) across 3 initiative group(s), blocked: 7 row(s), and direction 2 open initiative(s). Full exact commands/output are frozen in the evidence artifact.
Existing docs: ready/direction appear in the spec and usage skill, but not README.md or docs/; this is a README front-door gap.
ch:trace row=DOC-41 spec=none:repository-contract-absent wave=none:serial-audit test=cmd/boardctl/graph_cmd.go::cmdReady-and-cmdDirection doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-docs/SKILL.md evidence=.coding-hermes/board/evidence/boardctl-docs-2026-11-10-13-38-53-frozen.md witness=none:source-versus-reader-docs verdict=none:open-finding commit=none:open memory=none:unresolved-namespace

DOC-42 — P4 — document normalize-statuses for operators in the README front door
Evidence: source dispatches normalize-statuses at cmd/boardctl/main.go:263-264 and defines --apply/--dry-run/--json and refusal semantics at cmd/boardctl/statusnormalize.go:11-22. Fresh binary dry-run JSON returned rows 255, alias_count 3, unknown_count 0, missing_count 0, changed 3, applied false. Exact commands/output are frozen in the evidence artifact.
Existing docs: docs/operations.md and skills/boardctl-usage/SKILL.md cover the command, but README.md does not; this is a README front-door gap.
ch:trace row=DOC-42 spec=none:repository-contract-absent wave=none:serial-audit test=cmd/boardctl/statusnormalize.go::cmdNormalizeStatuses doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-docs/SKILL.md evidence=.coding-hermes/board/evidence/boardctl-docs-2026-11-10-13-38-53-frozen.md witness=none:source-versus-reader-docs verdict=none:open-finding commit=none:open memory=none:unresolved-namespace

DOC-43 — P4 — add the repository-local traceability doctrine for maintainers and operators
Evidence: test -e docs/traceability-doctrine.md returned exit 1 at the published revision. The repository-local contract is absent; the exact source and board controls are frozen in the evidence artifact.
ch:trace row=DOC-43 spec=none:repository-contract-absent wave=none:serial-audit test=none:repository-doctrine-missing-probe doc=docs/traceability-doctrine.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-docs/SKILL.md evidence=.coding-hermes/board/evidence/boardctl-docs-2026-11-10-13-38-53-frozen.md witness=none:repository-contract-absent verdict=none:open-finding commit=none:open memory=none:unresolved-namespace

Unverified: no live long-running serve endpoint or external store; destructive-command scan; release-asset/fresh-clone reproduction; worker fixes and documentation changes.

STATUS: PARTIAL
TIER: T3 -- fresh binary build/version and real ready, direction, normalize-statuses invocations; target board readback and validation; no live serve/store boundary
EVIDENCE: .coding-hermes/board/evidence/boardctl-docs-2026-11-10-13-38-53-frozen.md
