# README health — boardctl — tick boardctl-readme-2026-11-22-05-45-57

Re-fire **7** (7th delivery of this tick within the same scheduling window).
Lane: `coding-hermes-readme` (skill file `~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md`).
Target: `/home/kara/coding-hermes-boardctl`. Stand-in: `/home/kara/.hermes/stand-in/readme/boardctl`.

**Result: NOTHING-TO-DO — zero new rows, zero amended rows, zero README edits.**

Earlier deliveries of this tick: re-fire 1-2 marker repairs (33880e0, 6b6fa0b), re-fire 3 filed
README-024/-025 (a9b1ff7), re-fire 4 filed README-026 (34d6843), re-fire 5 verify-only with 7
re-probe notes (59c56a2), re-fire 6 repaired README-026's scratch-only evidence pointer and
event 766's empty task_id (556c48e). This fire re-measured independently rather than citing them.

## 1. Identity of the artifact audited

| fact | value |
|---|---|
| published tip at start | `556c48e66b59fef077b443fec1c075d56f904293` (origin/main) |
| README blob at tip | `2b01a44f61a1fd46e46d37682df33dac0fb0e881` |
| README content sha256 | `990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b` |
| README last commit | `ea8dcab` 2026-11-21 (merge of wt/DF-BOARDCTL-57) |
| last release tag | `v0.5.0` 2026-11-15 → README is **newer** than the last release (no freshness defect) |

Product-code commits after the README's last move, other than board bookkeeping:
`c8b5e69` (2026-11-21, `.gitreins/config.yaml` + a judge-lane evidence doc). Neither file is
described by any README sentence, so no documented claim was invalidated by it.

Local checkout was 1 commit behind origin/main at start (556c48e not present); fast-forwarded
(`git merge --ff-only origin/main`) before probing, so every probe below ran on the published tree.

## 2. Sweep arms executed this fire

| arm | command | result |
|---|---|---|
| release identity | `./dist/boardctl_linux_amd64 version` | `boardctl version v0.5.0` rc=0 — matches README.md:17/36 |
| release identity json | `./dist/boardctl_linux_amd64 version --json` | `{"version":"v0.5.0","build":"v0.5.0"}` rc=0 |
| board validation | `./dist/boardctl_linux_amd64 -C . validate` | `RESULT: OK (44 warning(s))` rc=0 (all 44 warnings pre-existing legacy rows, same set as re-fire 5/6) |
| README release-pin gate | `make version-check` | rc=0 (`ok ... internal/versioncheck`) |
| gate negative control | `make version-check VERSION_TAG=v0.1.11` | rc=2, `fix: move every release surface ... FAIL` — the documented wrong-tag arm still refuses |
| quickstart (stranger path) | `init` → `create --id QS-1 --title "first task" --priority 1 --acceptance-criteria "1) exists"` → `list --status pending` → `stats --all` | rc=0/0/0/0; list shows `QS-1 P1 pending`; `stats --all` prints `pending 2` = 1 real row + NEVER-DONE fixture, exactly the semantics README.md:84-89 documents |
| anchors | `check-md-anchors.py README.md` | `anchors_checked=6 broken=0` rc=0 |
| links / badges | `readme-link-check.py <repo>` | `local_missing=0 external_non_200=0` rc=0 (3 local files exist; 2 external 200 — github.com/coding-hermes, .../boardctl/releases) |

No `make docs-check` exists, but `make version-check` is the repo's README release-pin gate and it
runs green (CI enforces the same rule per README.md:1720), so the "docs-check target absent"
finding class does not apply here.

## 3. Re-verification of open rows (measured this fire, not inherited)

| row | premise | probe (exact) | tier | verdict |
|---|---|---|---|---|
| README-015 | README's five-gate block runs `make version-check VERSION_TAG=v0.1.11`, which fails on a clean tip | `make version-check VERSION_TAG=v0.1.11` → rc=2 `repo README is stale vs the expected release` | T3 | TRUE, stays open |
| README-020 | README teaches the doctor debt metric but never names `evidence baseline --record` | grep of README.md → 0 hits for `evidence baseline --record`; binary top-level usage lists `evidence baseline --record` | T2 | TRUE, stays open |
| README-021 | `create --deferred` without `--defer-reason` is refused, and README never mentions the flag | `boardctl -C <scratch> create --id DEF-1 --title "deferred row" --deferred` → rc=1 `boardctl: create: --deferred requires --defer-reason (a deferral without a named blocker is a phantom row; nothing written)`; control with `--defer-reason "waiting on owner"` → rc=0 row created; grep README for `--defer-reason` → 0 hits | T3 | TRUE, stays open |
| README-022 | current-state version examples still name v0.1.11 while README.md:17 says v0.5.0 | README.md:17 `Current release: **v0.5.0**.` vs README.md:94/98 `prints e.g. "boardctl version v0.1.11"` / `{"version":"v0.1.11",...}`; 34 v0.1.11 mentions remain; the shipped binary prints v0.5.0 | T2 | TRUE, stays open |
| README-023 | no contents/index above 20 H2 sections | no `Contents`/`Index` heading in the first 120 lines; `grep -c '^## '` = 20 | T2 | TRUE, stays open |
| README-024 | README teaches 18 event types, tree holds 19 | `internal/board/read.go` `EventTypeVocabulary` = **19** entries incl. `task_type_changed`; README.md:899 and :906 say 18; the quoted refusal block (README.md:936-942) omits `task_type_changed` | T2 | TRUE, stays open |
| README-026 | import refuses a differently-named board and README never says the export is board-scoped | `boardctl -C <src> render --json export.json` rc=0; `boardctl -C <dst> import export.json --dry-run --renumber` → rc=1 `boardctl: board mismatch: export is for board "impexp-src"; target board is "impexp-dst"` | T3 | TRUE, stays open |

Not re-probed this fire (unchanged premises; README blob and product code are byte-identical to
what re-fire 5 measured): README-013, -014, -016, -017, -025, -006-R2. README-014 carries a
"core finding obsolete — verify and close" note; closing it needs a worker's re-run plus a bound
evidence run, which is the owning foreman's decision, not an audit-lane write.

## 4. Row/marker census at the published tip

27 README-family rows; 13 pending (013, 014, 015, 016, 017, 020, 021, 022, 023, 024, 025, 026, 006-R2).
Every pending row carries exactly one trace marker. Marker **field** differs by writer:
013/014/015/016/017/021/022/023/026 and 006-R2 in `foreman_note`; 020/024/025 in `reasoning`
(a census keyed only on `foreman_note` reports those three as markerless — that false zero is
exactly why the all-string-fields census was run; no repair was needed).
README-001..012 are complete rows from before the marker contract: 0 markers, historical debt,
not reopened by this lane (they close nothing and file nothing).

## 5. Residuals observed, deliberately not "fixed"

- **Two audit events carry this tick**: events.jsonl id 763 (line 767) and id 766 (line 770), both
  `event_type=audit`, both now `task_id=null`. Both are published history from re-fires 1-3; a
  published event is not correctable in place, so they are left as-is and reported instead.
- README-018 (complete) carries markers in both `foreman_note` (2) and `reasoning` (1) — structural
  debt inherited from an earlier closeout, counted once, not rewritten.

## 6. Considered and rejected as a finding

README.md:249 and :531 name the bare flag `render --json` in prose. Following that literally gives
`boardctl: flag needs an argument: -json`, rc=1. Not filed: the same section documents the
requirement at README.md:530-532 (`--json <path>` — "ALSO write the raw board-report/v1 payload")
and shows `render ... --json report.json` at README.md:515, so the documented surface is accurate
and the two prose mentions are shorthand, not a false promise. Filed instead as a note here so a
later fire does not re-litigate it.

## 7. UNVERIFIED this fire

- **Stranger install download leg**: `curl -sL .../releases/download/v0.5.0/boardctl_linux_amd64`,
  `curl ... SHA256SUMS`, `sha256sum -c --ignore-missing`, then `./boardctl_linux_amd64 version`.
  The unattended surface GATED that network command chain (no consent, no run). Release tags
  resolve (`git ls-remote --tags` shows v0.5.0 and its peeled commit) and the locally shipped
  `dist/` asset reports `v0.5.0`, but **the download+checksum path was not executed this fire**.
  No row filed about it: a gated probe is not a defect.
- Rows listed in §3 "not re-probed" were not executed this fire; they are neither confirmed nor
  refuted here.

## 8. Trace markers for the rows re-verified by this fire

ch:trace row=README-015 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:make-version-check-wrongtag doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-020 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:readme-grep-probe doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-021 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:create-deferred-refusal-probe doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-022 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:version-surface-differential doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-023 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:structure-heading-census doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-024 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:event-vocabulary-differential doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7

ch:trace row=README-026 spec=none:no-traceability-doctrine-in-repo wave=boardctl-readme-2026-11-22-05-45-57#step7 test=none:import-board-mismatch-probe doc=README.md prompt=~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md evidence=.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire7.md witness=none:locally-reproduced-open-finding verdict=none:open-finding commit=none:open memory=/boardctl/readme-audit/2026-11-22-refire7
