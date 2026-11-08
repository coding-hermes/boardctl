# Dogfood run 28 — DEFER-EVIDENCE and BOARD-GUARD-003, used the way a lane uses them

Tick: `boardctl-dogfood-2026-11-08-04-06-52`
Target: `coding-hermes/boardctl` @ `1cda136f85e753ea1de006ca79e3966ba9db5034` (origin/main, 0 ahead / 0 behind at start)
Binaries: HEAD build from the canonical checkout (`go build ./cmd/boardctl`, 1.21 s, `boardctl version dev`);
released `v0.1.11` left alone (the two surfaces under test are HEAD-only by construction).
Date: 2026-11-08

## What was promised (the two surfaces this run targeted)

Two write guards landed since the last dogfood run (run 27, 2026-10-29, HEAD `7ef46b3`):

- `79ebc73` **BOARD-GUARD-003** — "refuse symbolic commit-hash + validate evidence-run-id
  provenance": a completion must carry a full 40/64-hex `--commit-hash` that `git rev-parse`
  resolves, and an `--evidence-run-id` that names an existing directory under
  `.gitreins/history/`.
- `1cda136` **DEFER-EVIDENCE** — `defer_reason` + `defer_review_by`: `create`/`update` REFUSE
  `--deferred` without `--defer-reason` (nothing written); un-deferring removes both audit keys;
  `validate` WARNS (never fails) on hand-edited deferred rows with no reason.

A third surface, `cbd817a` **initiative** grouping (`--initiative` / `--clear-initiative` /
`list --initiative` / `initiatives`), was re-proven as part of the same run.

## How it was used (real user path, not a test harness)

Fresh scratch board (`init`, 1 fixture row), then the documented workflows:

1. **The README's deferred recipe, verbatim** (`README:1365`):
   `boardctl -C <dir> create --id FEAT-3 --title "Maybe later" --deferred`
   → `boardctl: create: --deferred requires --defer-reason (…)`, rc=1, **nothing written**.
2. **The working deferral path**: `--deferred --defer-reason "…" --defer-review-by 2026-11-20`
   → row carries `deferred:true` + both audit keys; `stats` reports `actionable: 2 / deferred: 1`.
3. **Defer an existing row** (`update --deferred true --defer-reason … --defer-review-by …`) → rc=0;
   **un-defer** (`--deferred false`) removes all three keys, row returns to its pre-deferral shape.
4. **Defer a row that already carries a reason** (`update FEAT-4 --deferred true`) → rc=0, reuses the
   stored reason. Good ergonomics: the reason need not be retyped.
5. **Legacy hand-edited deferred row with no reason** → `validate` prints the itemized warning and stays
   `RESULT: OK`, rc=0, exactly as the commit claims (message names both remedies).
6. **The README's headline update example, verbatim** (`README:136-137`):
   `update FEAT-1 --status complete --commit-hash abc1234 …` → `completion REFUSED: --commit-hash
   "abc1234" must be a full 40- or 64-hex commit id`, rc=1.
7. **Provenance on a real repo** (no-hardlinks clone of the published HEAD, real `.git`,
   `.gitreins/history/` with five date dirs): completions carrying a **phantom commit**
   (40 zeros) and a **phantom evidence run** (`deadbeef`) both succeeded at rc=0 — see
   DF-BOARDCTL-52.
8. **Retried closeout**: the identical successful `update --status complete` command re-run exits 1
   (`task_completed event already exists`) — see DF-BOARDCTL-53.
9. **Initiative surface**: scoop (`list --initiative lane-hooks --json`), rollup (`initiatives`),
   deferred accounting inside an initiative (`ACTIONABLE 2 / DEFERRED 1`), slug refusal with a
   suggested spelling (`Lane_Hooks` → `lane-hooks`, rc=1). All as documented.

## Findings filed (this board, status `pending`)

| row | pri | subject |
|---|---|---|
| **DF-BOARDCTL-52** | P1 | BOARD-GUARD-003's provenance check is inert on every real board: `isTestEnvironment()` (`internal/board/evidence_gate.go:134-143`) inspects the BOARD dir for `.git`, which no real layout has, so both git checks short-circuit — a phantom commit hash and a nonexistent evidence run complete at rc=0. Discriminating arm: planting `.git` **inside** the board dir makes both checks fire. |
| **DF-BOARDCTL-53** | P2 | An identical re-run of `update --status complete` exits 1 although `evidence_gate.go:26-30` documents that re-write as an ungated idempotent no-op; `--force` does not bypass, and the refusal names no remedy (hash/note-only update works, undocumented). |
| **DF-BOARDCTL-54** | P3 | Two documented recipes are refused at HEAD (`README:136` abbreviated hash, `README:1365` bare `--deferred`) and `--defer-reason` / `--defer-review-by` appear nowhere in README, `docs/` or `skills/`. Asks for a mechanical README-example gate. |
| **DF-BOARDCTL-55** | P3 | `--defer-reason` / `--defer-review-by` are accepted on rows that are **not** deferred, so a typo'd deferral leaves an actionable row carrying a parked reason with no `validate` finding. |

Re-proof of existing rows (no duplicates filed):

- **BT-091** (list has no deferred filter) — reproduced at HEAD: `list --deferred` → `flag provided but
  not defined: -deferred`, rc=1. Workaround: `list --json` + `jq 'select(.deferred==true)'`.
- **BT-093** (initiatives does not aggregate across an umbrella) — not re-probed this run; no evidence
  either way, row left untouched.

## Verified clean in the same pass

- Deferral refusal writes **nothing** (board byte-identical after the refused create) and its message
  names the required flag.
- Un-deferral removes `deferred`, `defer_reason`, `defer_review_by` together; the row is
  byte-identical to a never-deferred row apart from `updated_at`.
- Legacy deferred-without-reason rows: WARN, `RESULT: OK`, rc=0 on a live board (the promised
  "debt visible, exit 0" policy holds).
- Initiative slug rule enforced with a suggested spelling; absence is not a value (`--clear-initiative`
  removes the key); rollup's `actionable` excludes deferred rows; ordering total-descending then slug.
- Symbolic (`HEAD`) and abbreviated (`abc1234`) commit refs refused at the write layer with a precise
  message — the half of BOARD-GUARD-003 that does work.
- Board integrity after filing: 228 → 232 rows, append-only (pre-image is an exact byte prefix),
  0 duplicate ids, 4 `task_created` events (one per row), `validate` `RESULT: OK (44 warnings)` —
  the same 44 pre-existing warnings as before the write, none new.

## Performance (Step 2b)

Measured with `hyperfine` on the HEAD build; the headline operations are the read commands a
lane/foreman runs constantly.

| operation | board | warm mean ± σ (20 runs) |
|---|---|---|
| `stats` | this board (228 rows) | 24.5 ms ± 2.9 |
| `list --json` | this board | 32.4 ms ± 2.5 |
| `initiatives` | this board | 22.9 ms ± 2.9 |
| `next-id --family PERF` | this board | 22.4 ms ± 3.2 |
| `stats` | hermes-dagger (1443 rows) | 116.4 ms ± 8.5 (10 runs) |
| `list --json` | hermes-dagger | 132.7 ms ± 19.2 |
| `initiatives` | hermes-dagger | 99.2 ms ± 10.2 |
| `validate` | hermes-dagger | 181.7 ms ± 13.6 |
| `stats` (first invocation, isolated `HOME`) | hermes-dagger | 0.10 s wall |

Nothing here is user-noticeable at the interactive scale boardctl is used at (a lane runs one of these
per tick, and the largest real board measured is 4x this one). **No PERF row filed** — a win nobody can
feel is not a finding. Cold (page-cache-cold) is unmeasured: dropping caches needs root on this host,
and quoting a warm number as cold would invert the finding.

## Install leg

Ephemeral bunker battery via `bunker-qa.sh launch` / `collect` on `bunker-mvp`
(agent `fc323100`, ttl 4h), evidence
`/home/kara/.hermes/cache/scratch/df-2026-11-08-04-06-52/bunker-evidence.jsonl`.
Host census first: `bunker-mvp` ONLINE (v0.2.1, `/tmp: private`, **`Net mode: shared — NOT tenant
isolation`**); `bunker-las-02` ONLINE but `/tmp: HOST-SHARED` + `net: not reported`; las-01 / las-03 /
las-04 / cube-las-00 unreachable at `/health` (000). No registered host is isolation-eligible, so this
leg is graded **install-proven / isolation-UNVERIFIED** with `Net mode: shared` as the disqualifying
field — not a skip (a usable host existed and the install cells ran) and not a silent pass.

23 cells collected, agent destroyed and verified gone (`bunker info fc323100` → `not_found`; fresh
census keeps the four pre-existing agents, none of them this run's):

| cell | result | note |
|---|---|---|
| toolchain-bootstrap | OK | Go 1.27.3 bootstrapped into `~/tools` (ambient 1.22.2, floor 1.26.10), gcc/g++, make, jq, compose+buildx, node 22.15 |
| fresh-install | OK | `go build ./...` from a synced tree on a bare Debian user — no artifact expected for a compile leg |
| tests | OK | native suite rc=0 |
| ci-pass | OK | `act` ran 1 triggerable workflow, 1 job green |
| upgrade | **OK** | previous release v0.1.11 tree → HEAD reinstall clean (this cell FAILED in run 27; QA-BOARDCTL-FOREMAN-4 is already `complete` — the fix holds now) |
| chaos-disconnect | **OK** | fails fast with `NETWORK_CUT_APPLIED` (FAILED in run 27; QA-BOARDCTL-FOREMAN-5 `complete` — holds now) |
| chaos-resource | PASS | suite survives a 3 GiB memory cap (3× cap observed by child/subshell) |
| chaos-errorpath | OK | clean error (rc=2) on a missing config |
| ui-probe | INFO | go-native start is not a long-running server — no HTML surface (QA-BOARDCTL-FOREMAN-6, `complete`) |
| lint | **INFO, not a pass** | `golangci-lint` unavailable on the agent, so lint did not run; the CI `act` job covers `go vet`/build/test but not the repo's lint gate |
| docker / compose / shutdown / corruption | N/A | no Dockerfile, no compose file, no state files |

## Boundaries

- No product code changed. No row on this board was edited or closed; only four rows appended.
- Nothing was pushed by the probe steps; the only board mutation is the four appended rows plus their
  `task_created` events, committed with the run's documentation.
- Scratch (binaries, clones, fixture boards, benchmark JSON): `/home/kara/.hermes/cache/scratch/df-2026-11-08-04-06-52`.
