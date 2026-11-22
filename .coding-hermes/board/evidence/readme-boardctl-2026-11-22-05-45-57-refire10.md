# README health — boardctl — tick boardctl-readme-2026-11-22-05-45-57

Re-fire **10** (tenth delivery of this tick). Lane: `coding-hermes-readme`
(`~/.hermes/skills/coding-hermes/coding-hermes-readme/SKILL.md`).
Target: `/home/kara/coding-hermes-boardctl`. Audit worktree (clean, published tip):
`/home/kara/wt-boardctl-readme-refire10`. Stand-in: `/home/kara/.hermes/stand-in/readme/boardctl`.

**Result: NOTHING-TO-DO — zero new rows, zero amended rows, zero board writes,
zero events, zero README edits.**

Verify-only by step 2 of the skill: this tick's sweep was delivered and published at
`c57abd7` and fires 2-9 re-verified, re-probed and repaired what was owed. This fire
re-measured independently at the published tip instead of citing those reports, and
carries one new measurement the earlier fires could not have made — the tip advanced
past them (see "Tip movement" below).

## Identity at the published tip

| surface | reading |
|---|---|
| `origin/main` | `9f7b52e0e8b5482835550a4beec40bc84ab79723` |
| worktree `HEAD` | `9f7b52e0e8b5482835550a4beec40bc84ab79723` (identical) |
| `ls-remote --symref origin HEAD` | `ref: refs/heads/main`, same SHA |
| README git blob | `2b01a44f61a1fd46e46d37682df33dac0fb0e881` |
| README content sha256 (tip) | `990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b` |
| README content sha256 (working file) | `990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b` |

The README blob is byte-identical to the one every earlier fire audited
(`2b01a44`, content `990e5091`). No product-facing sentence has moved under an
open row.

## Tip movement since fire 9

Fire 9 read the tip at `2d8542e`. Two commits landed since:

- `bf17d57` — `feat(serve): gzip compression middleware for ui and api endpoints
  (PERF-001)`: `cmd/boardctl/gzipmw.go` (+117), `cmd/boardctl/gzipmw_test.go`
  (+340), `cmd/boardctl/serve.go` (±1). **Product code, no README change.**
- `9f7b52e` — board-only foreman closeout of PERF-001 (`tasks.jsonl`,
  `events.jsonl`; `git diff --stat bf17d57 origin/main` = 2 files, 2 insertions,
  1 deletion).

Because the product surface changed under this lane, one targeted documentation
arm was added (documented-command execution on the affected surface), below.

## Readback battery

`bash <skill>/scripts/refire-readback-battery.sh --repo
/home/kara/wt-boardctl-readme-refire10 --ref origin/main --tick
boardctl-readme-2026-11-22-05-45-57 --ids <13 open README rows> --out
readback-refire11b.txt` → transcript sha256
`4717dfccd97a5b8b9534be2dbc4d095118a39fedc9315c91788dc1c54b660597`,
**battery exit 1** (one arm: tick event census, explained under Residual).

Arms:

- row-marker readback — `rows_parsed=13/13`, `ROWS_FAILING=0`. All thirteen open
  README rows (`README-013,014,015,016,017,020,006-R2,021,022,023,024,025,026`)
  read `pending` at their filed priority (P4/P5), exactly one `ch:trace` marker
  each, and every `evidence=` artifact resolves at its recorded sha256
  (`680b74bd…`, `e890d5e9…`, `93b1a1d0…`, `8837fb0c…`, `8a7e8d97…`). No
  `memory=` key was re-declared dead by this fire: the field-scoped extraction of
  the earlier attempt returned the marker text truncated at the next `\n`, which
  is an extractor artifact, not a missing key.
- doc-severity census — 55 doc rows, 16 pending; `by priority: P2=8, P3=3,
  P4=34, P5=10`. Pending doc rows below the P4/P5 ceiling: **none**. Pre-2026-09-30
  re-triage candidates: **none**. Duty discharged by census, not by memory of an
  earlier tick.
- `boardctl validate` — `RESULT: OK (44 warning(s))`, 0 errors; 287 tasks, 772
  events, 1 fixture, header parsed. Every warning is inherited board history
  (canon-key drift, `closed` read aliases, missing `created_at`, legacy event-id
  shapes, one legacy duplicate event id 127) and none belongs to a README row.
- anchors — `anchors_checked=6 broken=0`.
- links — `local_links=3 external_links=2`, `local_missing=0
  external_non_200=0` (org page and `/releases` both answer 200 live).
- tick event census — `events_parsed=772 tick_hits=2` (ids 763, 766), exit 1.
  See Residual.

## Documented-command arm on the moved surface (new this fire)

Built from the audited tip (`go build ./cmd/boardctl`, go1.26.11) →
`/home/kara/.hermes/stand-in/readme/boardctl/boardctl-refire10`.

- README.md:451-453 promises: "a non-loopback `--addr` (0.0.0.0, 10.x, a
  hostname) is refused with exit 2 — serve has no auth and must never leave the
  machine". Executed `boardctl-refire10 serve --addr 0.0.0.0:8787` → **exit 2**,
  stderr: `boardctl: usage: --addr "0.0.0.0:8787" refuses non-loopback host
  "0.0.0.0": serve binds 127.0.0.1, localhost or ::1 only (spec 6.1: loopback
  convenience uploader, no auth, no TLS)`. The documented refusal holds.
- `go test ./cmd/boardctl -run Gzip -count=1` → `ok
  github.com/coding-hermes/boardctl/cmd/boardctl 0.028s`, rc=0: the middleware
  PERF-001 landed does not disturb the serve/UI handler tests.
- The README makes **no claim about response compression** (`grep -niE
  'gzip|compress' README.md` → no hits), so the new middleware creates no
  documented-surface drift and no row. A doc lane does not invent a requirement
  to describe a transport optimisation the README never promised.

## Residual: two published audit events carry this tick

`event-tick-census.py` exits 1 because ids 763 (`re-fire (verify-only)`) and 766
(`re-fire 3`) both carry this tick string. Both are inherited, published before
this fire, and both are audits of the same attempt — "keep one canonical audit",
not a defect. The arm is attributed, not silently passed.

## Tier

- **T3** for the executed arms: the freshly built binary was invoked (`serve
  --addr 0.0.0.0:8787` → exit 2) and the gzip unit tests were run from the
  audited tree.
- **T4** for the readback arms: board rows, event log, README blob and evidence
  artifacts were read back out of `origin/main`, not out of the working copy.

## Boundary

No board row, event, README line or evidence file was created or edited by this
fire; the only write is THIS document, committed beside the board it describes.
The three untracked paths in the canonical checkout (`.gitreins/review-state.json`,
`.gitreins/review-state.lock`, `cmd/boardctl/boardctl`) are build/harness
artifacts owned by earlier probe and hook runs; the audit therefore wrote from a
clean detached worktree at the published tip rather than from that checkout.

STATUS: PASS
TIER: T4 -- board rows, event log, README blob and evidence blobs read back from origin/main at 9f7b52e; documented serve refusal executed against a binary built from that tree (T3)
EVIDENCE: .coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57-refire10.md (this file); producer transcript readback-refire11b.txt sha256 4717dfccd97a5b8b9534be2dbc4d095118a39fedc9315c91788dc1c54b660597
