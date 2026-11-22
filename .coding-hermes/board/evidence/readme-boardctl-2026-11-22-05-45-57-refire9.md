# README health — boardctl — tick boardctl-readme-2026-11-22-05-45-57

Re-fire **9** (ninth delivery of this tick). Lane: `coding-hermes-readme`
(`~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md`).
Target: `/home/kara/coding-hermes-boardctl`. Stand-in: `/home/kara/.hermes/stand-in/readme/boardctl`.

**Result: NOTHING-TO-DO — zero new rows, zero amended rows, zero board writes,
zero events, zero README edits.**

Verify-only by step 2 of the skill: the tick's sweep was delivered and published at
`c57abd7`, and fires 2-8 already re-verified, re-probed and repaired what was owed.
This fire re-measured independently at the published tip instead of citing those
reports.

## Identity at the published tip

| surface | reading |
|---|---|
| `origin/main` | `2d8542ea7a8f390cf69f68276bb866629046d47f` |
| local `HEAD` | `2d8542ea7a8f390cf69f68276bb866629046d47f` (identical) |
| `ls-remote --symref origin HEAD` | `ref: refs/heads/main`, same SHA |
| README git blob | `2b01a44f61a1fd46e46d37682df33dac0fb0e881` |
| README content sha256 (tip) | `990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b` |
| README content sha256 (working file) | `990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b` |
| README lines | 2004 |

The blob is byte-identical to the tree every earlier fire audited (`2b01a44`, content
`990e5091`). The tip commit `2d8542e` touches only the board and its evidence
directory (`git show --stat`: `tasks.jsonl`, one evidence file) — no product file,
no README, so the audited artifact has not moved under any open row.

## Readback battery

`bash <skill>/scripts/refire-readback-battery.sh --repo /home/kara/coding-hermes-boardctl
--ref origin/main --tick boardctl-readme-2026-11-22-05-45-57 --ids <13 open README rows>
--out readback-refire9.txt` → transcript sha256
`d753e57162a50e5b952e7c1f53f72733256d5805f753b618e8cdfa1e0140f552`, **battery exit 1**
(one arm: tick event census, explained under Residual below).

Arms:

- row-marker readback — `rows_parsed=13/13`, `ROWS_FAILING=0`. All thirteen open
  README rows (`README-013,014,015,016,017,020,006-R2,021,022,023,024,025,026`)
  read `pending` at their filed priority (P4/P5), exactly one `ch:trace` marker
  each, and every `evidence=` artifact resolves at its recorded sha256
  (`680b74bd…`, `e890d5e9…`, `93b1a1d0…`, `8837fb0c…`, `8a7e8d97…`).
- doc-severity census — 55 doc rows, 16 pending; `by priority: P2=8, P3=3, P4=34,
  P5=10`. Pending doc rows below the P4/P5 ceiling: **none**. Pre-2026-09-30
  re-triage candidates: **none**. The doc-severity duty is discharged by census,
  not by memory of an earlier tick.
- `boardctl validate` — `RESULT: OK (44 warning(s))`, 0 errors; 287 tasks, 771
  events, 1 fixture, header parsed. Every warning is inherited board history
  (canon-key drift, `closed` read aliases, missing `created_at`, legacy event-id
  shapes, one legacy duplicate event id 127) and none belongs to a README row.
- anchors — `anchors_checked=6 broken=0`.
- links — `local_links=3 external_links=2`, `local_missing=0 external_non_200=0`
  (both external links, the org page and `/releases`, answer 200 live).
- tick event census — `tick_hits=2` (ids 763, 766), exit 1. See Residual.

## Residual: two published audit events carry this tick

Decoded independently (`refire9-events.py`, sha256 `b5e59ac5…`):

- event **763** — `fire: re-fire (verify-only)`, opens at `c57abd7`, closes at
  `9b8414e`, verifies `README-006-R2/021/022/023`, notes five re-probe rows.
- event **766** — `fire: re-fire 3 (full sweep re-executed; files the 2 findings the
  earlier fires did not carry)`, opens at `6b6fa0b`, files `README-024` and
  `README-025`, records 87 executed probe arms.

They document distinct work, so neither is a duplicate of the other and neither was
removed. This fire appended no event — a third record for one tick would deepen the
residual rather than resolve it. The battery's nonzero exit is attributable to this
pair alone.

## Additional readings the battery does not take

- **Source-side premises.** Every cited source file has **0 commits** since its
  row's `created_at` at the tip (`read.go`, `write.go`, `Makefile`, `ci.yml`,
  `evidence_gate.go`, `import.go`) → all source-side premises hold by construction.
  Transcript `refire9-source-and-anchors.txt`, sha256 `92956cae…`.
- **`doc=` anchors.** All six resolve by heading slug against the audited blob
  (`#usage`, `#install`, `#development`, `#boardctl`, `#the-deferred-flag-bt-076`,
  `#event-type-vocabulary-doc-27`); `README-026`'s `resolves(line 509)` re-checked —
  line 509 is `### render`. The three rows with bare `doc=README.md` owe no anchor.
- **`memory=` keys.** All five keys cited by open rows resolve by exact-key recall in
  namespace `boardctl`: `/project/boardctl/readme/2026-11-22-0545-audit`,
  `…-refire-R2`, `…-refire2`, `…-refire3`,
  `/reviews/boardctl/ticks/boardctl-readme-2026-11-15-04-40-50/refire-verify`.
  `README-026`'s is an explicit `none:not-written`.
- **Release boundary.** README.md:17 `Current release: **v0.5.0**`; newest tag is
  `v0.5.0`; `gh release view v0.5.0` → `draft:false pre:false`, 8 assets
  (7 platform binaries + `SHA256SUMS`) — the documented installer's asset set exists.
  `CHANGELOG.md` head is `## v0.5.0 — 2026-11-15`. No release-boundary discrepancy.
- **Toolchain parity (README-009's family).** README.md:1872 says `go.mod` declares
  `go 1.26.10`; `go.mod:3` declares `go 1.26.10` (toolchain `go1.26.11`). No drift.

## Write set

This artifact only. `tasks.jsonl` and `events.jsonl` are byte-identical to the
pre-fire tip; no row was created, noted, amended, demoted or closed; no event was
appended. Nothing was minted (no id owed) and the target README was not edited —
the foreman dispatches a worker for every filed row.

## UNVERIFIED this fire

Command-matrix re-execution (forbidden on a re-fire: re-fire 3's 87 executed arms and
the delivery's 118-arm sweep cover it against this same README blob); release-asset
download and execution (the consent gate refused it on earlier fires; asset
existence was re-checked here via `gh release view` only); `boardctl serve`; hook
`install --apply`/`uninstall`; and the `make` gates (`version-check`,
`install-check`, `vendor-check`, `vuln-check`, `cover-check`, release procedure).
