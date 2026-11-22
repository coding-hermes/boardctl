# boardctl README health — tick boardctl-readme-2026-11-22-05-45-57, re-fire 8

Lane: coding-hermes-readme (verify-only). Target: /home/kara/coding-hermes-boardctl.
Produced: 2026-11-22T09:44Z by the lane's own probe scripts in the stand-in
(/home/kara/.hermes/stand-in/readme/boardctl).

## Classification

Eighth delivery of this tick. Re-fire 7 (commit d3ed8c3) landed a verify-only sweep
with zero new rows. The tick's audit events (763, from the delivery; 766, from re-fire
3's two new findings) are published, and all thirteen open README rows are pending at
the tip. Per the re-validation rule this fire re-reads rather than re-files: 0 rows
minted, 0 closures, 2 row notes (fresh live re-proofs, below).

## Identities (independently resolved this fire)

| item | value |
|---|---|
| published tip (origin/main) | d3ed8c3408f1f2772dc6766bc06d2325bd1f8749 |
| local HEAD | d3ed8c3408f1f2772dc6766bc06d2325bd1f8749 (identical) |
| upstream | origin/main |
| README blob | 2b01a44f61a1fd46e46d37682df33dac0fb0e881 |
| README content sha256 | 990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b |
| README last moved | ea8dcab 2026-11-21T19:34:25-05:00 (merge wt/DF-BOARDCTL-57) |
| newest tag | v0.5.0 (7b3c618c, peeled 3acca082) |
| working README sha256 | 990e5091… (same bytes, clean tree) |

The README is byte-identical to the blob every earlier fire audited, so all
documentation-side premises sit on the same page they were filed against.

## Frozen artifacts

| artifact | sha256 |
|---|---|
| readback-refire8.txt (readback battery) | 36e24dc768e62a103cb07e85911c850746fc83c86c7eb080aa3ca88992beabd9 |
| premise-probes-refire8.txt (premise re-proofs) | 55a39eec06aa714b713c0e8ea229a8f65c7f2aa5845eb4e864234b507adcf140 |

Both were written by the bundled battery / a file-authored probe script and were not
edited afterwards.

## Readback battery (13 open rows)

Command: `bash <skills>/coding-hermes-readme/scripts/refire-readback-battery.sh --repo
/home/kara/coding-hermes-boardctl --ref origin/main --tick
boardctl-readme-2026-11-22-05-45-57 --ids <13 ids> --out readback-refire8.txt`
— invoked bare, rc captured in the same call: **battery-exit=1**.

| arm | reading | attribution |
|---|---|---|
| ref identity (5 arms) | HEAD == origin/main == d3ed8c3 | pass |
| README blob + content digests | 2b01a44 / 990e5091 | pass |
| row marker readback | rows_parsed=13/13, ROWS_FAILING=0, exactly one marker each, every evidence= artifact resolves with the recorded sha256 | pass |
| doc-severity census | 55 doc rows, 16 pending, 0 pending below the P4/P5 ceiling, 0 pre-cutoff P1-P3 | pass — re-triage duty discharged, nothing to demote |
| boardctl validate | RESULT: OK (44 warnings) rc=0 | pass; warnings are inherited legacy key/status drift, no duplicate ids |
| anchor check | anchors_checked=6 broken=0 | pass |
| link check | local_links=3 external_links=2, local_missing=0, external_non_200=0 | pass |
| tick event census | tick_hits=2 (ids 763, 766) | **KNOWN RESIDUAL, not new** — see below |

The battery's non-zero exit is driven solely by the event-census arm, whose contract is
"exactly one audit event per tick". Two published audit events carry this tick because
re-fire 3 executed a full sweep and filed two findings after the delivery had already
written its own audit; both records document distinct work, so neither is a duplicate
and neither may be deleted. Re-fire 7 reported the same pair as a residual. This fire
adds no event (the artifact commit is the record), so the residual is unchanged, not
worsened.

## Readings the battery does not take

Doc anchors — resolved by slugging each referenced heading in the audited blob (never by
grepping the slug): README.md#usage OK, #install OK, #development OK, #boardctl OK,
#the-deferred-flag-bt-076 OK, #event-type-vocabulary-doc-27 OK. No dead doc anchor.
README-026's `spec=README.md#render resolves(line 509)` re-checked: line 509 of the
published blob is `### render`.

Memory keys — all five distinct keys cited by the open rows resolve by exact-key recall
in namespace `boardctl`:

| key | record id |
|---|---|
| /project/boardctl/readme/2026-11-22-0545-audit | 6c138093-41d6-453a-bb33-a7c1b3b5b9ca |
| /project/boardctl/readme/2026-11-22-0545-refire-R2 | b658d008-6e92-4678-b9fc-eb5b605191b2 |
| /project/boardctl/readme/2026-11-22-0545-refire2 | 3439fd77-ce22-457f-99ac-65c9f17c107f |
| /project/boardctl/readme/2026-11-22-0545-refire3 | b6d9ce0c-9167-4ca3-9e7d-9aa827eb9f67 |
| /reviews/boardctl/ticks/boardctl-readme-2026-11-15-04-40-50/refire-verify | 25e547b8-f823-4b77-813f-d97ac7f414e5 |

README-026 carries `memory=none:not-written` — an explicit stated value, allowed, and
its row still names a repo-local evidence artifact.

Source-side premise drift — the README cannot drift (identical bytes), but the code each
row cites could. Commits touching each cited file since its row was created:

| row | cited source | commits since filing |
|---|---|---|
| README-024 | internal/board/read.go, internal/board/write.go | 0 |
| README-021 | internal/board/write.go | 0 |
| README-016 | Makefile | 0 |
| README-017 | .github/workflows/ci.yml, Makefile | 0 |
| README-006-R2 | internal/board/evidence_gate.go | 0 |
| README-026 | internal/board/import.go | 0 |

Every premise therefore holds by construction at this tip: unchanged doc bytes and
unchanged cited source.

## Live re-proofs executed this fire (T3)

Run against the HEAD build (`bin/boardctl`, version-independent; the binary reports its
own refusal strings), on scratch boards under the stand-in — the target board was never
written by these arms.

1. README-021 premise — `create --id P8-1 --title "deferred probe" --deferred`
   → rc=1, `boardctl: create: --deferred requires --defer-reason (a deferral without a
   named blocker is a phantom row; nothing written)`. Still true: README.md:1473's
   example and the prose form at :1452 remain refused.
2. README-024 premise — `event --type bogus_type` → rc=1 enumerating 19 values
   (audit, board_bootstrap, board_init, board_migration, dogfood, e2e_verified, idle,
   spec_created, task_added, task_completed, task_created, task_dispatched,
   task_evidence, task_started, **task_type_changed**, task_updated, task_verified,
   tick, worker_dispatched). README.md:899 and :906 both still say 18, and
   `grep -c task_type_changed README.md` = 0.

One measurement correction, recorded rather than hidden: the first deferred-control run
was piped through `tail`, so the shell reported tail's rc (0) for a command that exits 1.
The re-run redirects to a file and captured rc=1; the frozen transcript holds the
corrected form.

## Board write set this fire

Two notes appended (`--note` appends to the row; it writes no event):

- README-021 — this fire's dated re-proof of the --defer-reason refusal.
- README-024 — this fire's dated re-count (19 in the binary, 18 in the doc).

No row was created, closed, demoted or re-prioritised. Severity policy is already
satisfied: every pending doc row sits at P4/P5.

## Not verified this fire

- Release-asset download/execution (v0.5.0 asset + SHA256SUMS) — not re-run; the
  consent gate refused this arm on earlier fires and this fire did not attempt it.
- `boardctl serve` arms, `hook install --apply` / `uninstall`.
- `make version-check`, `install-check`, `vendor-check`, `vuln-check`, `cover-check`, the
  release procedure. Not re-run: unchanged README bytes plus re-fire 3's executed matrix
  and re-fire 7's version arms already cover them, and re-deriving published arms is
  what the re-fire rule forbids.
- Whether the next release ships task_type_changed (source-inferred; no tag contains
  269add6).

STATUS: PASS
TIER: T3 -- executed the HEAD binary (`./bin/boardctl validate` rc=0, `create --deferred` rc=1,
`event --type` rc=1) plus published-tip readback of README blob, 13 rows, 2 audit events
and 5 memory keys from origin/main d3ed8c3
EVIDENCE: .coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire8.md
(transcripts: readback-refire8.txt sha256 36e24dc7…, premise-probes-refire8.txt sha256 55a39eec…)
