---
name: boardctl-usage
description: >-
  How to use boardctl — the CLI for coding-hermes JSONL foreman boards
  (tasks/events/board/fixtures under .coding-hermes/board/). Entry points,
  proven commands, error meanings, and pitfalls from a real-use dogfood run.
version: 1.1.0
category: software-development
---

# boardctl usage

`boardctl` manages a coding-hermes foreman board: four JSONL files under
`.coding-hermes/board/` — `tasks.jsonl` (task rows), `events.jsonl`
(append-only audit), `board.jsonl` (header: project/namespace/tick
counters/last_commit), `fixtures.jsonl` (perpetual fixture tasks). No
database, no caches: what git tracks is the board.

## Entry points

- Binary: `boardctl` (install: `go install
  github.com/coding-hermes/boardctl/cmd/boardctl@latest`, or a static binary
  from GitHub releases — the zero-dep path, ~4s).
- Source: `cmd/boardctl` (main), `internal/board` (read/write/validate/
  doctor logic). Build: `go build -o bin/boardctl ./cmd/boardctl`.
- Exit codes: `0` ok, `1` validation failure / runtime error, `2` usage
  (unknown command/flag) — and board-not-found also exits `2`, matching the
  README contract (BT-006 fixed the old exit-1 behavior). Script on:
  `0` = proceed, `1` = your input was rejected, `2` = wrong invocation or
  no board at the given path.

## The `-C` flag (read this first)

`-C` resolves the board dir. All three forms work (BT-006 fixed the
`.coding-hermes` case): a repo root (finds `.coding-hermes/board`), the
`.coding-hermes` dir itself, or the board dir itself
(`.coding-hermes/board`). Default (no `-C`) = current directory walked the
same way.

```bash
boardctl -C ~/myrepo stats                 # repo root — exit 0
boardctl -C ~/myrepo/.coding-hermes stats  # .coding-hermes dir — exit 0
boardctl -C ~/myrepo/.coding-hermes/board validate   # board dir — exit 0
boardctl -C ~/does-not-exist list          # exit 2, board-not-found
```

## Proven commands

```bash
# read
boardctl -C R stats [--json] [--all]       # counts by status/priority
boardctl -C R list [--status pending] [--priority P1] [--json] [--all]
boardctl -C R show <ID> [--events]         # searches tasks AND fixtures
boardctl -C R header --json
boardctl -C R validate                     # shape: JSONL parse, header, ids
boardctl -C R doctor                       # validate + git tracked-set (no
                                           # .db/.parquet), counter drift,
                                           # fixture orphans — run this FIRST
                                           # on any board you didn't create

# write (foreman loop)
boardctl -C R create --id FEAT-1 --title "T" --priority P1 \
    --complexity 2 --depends-on A,B --reasoning "why" \
    --capability-tags go,cli
boardctl -C R update FEAT-1 --status complete \
    --commit-hash <sha> --guard PASS --ci GREEN --summary "done (+80/-12)"
boardctl -C R event --type audit --tick 42 --detail-text "..."   # or --detail @file
boardctl -C R header --set-ticks-total 42 --set-last-commit <sha>
boardctl init --project myproject          # bootstrap a fresh board:
                                           # writes tasks/events/board.jsonl
boardctl version                           # date-stamp on release builds, "dev" otherwise
```

## Write vocabularies (all enforced since BT-007)

- `--status`: `failed, pending, in_progress, review, blocked, complete` —
  anything else rejected with exit 1.
- `--guard`: `{PASS,FAIL,SKIP}`; `--ci`: `{GREEN,RED,SKIP}` — junk values
  rejected at write (BT-007 closed the old accept-anything hole; trust
  these fields on boards written by recent binaries).
- `--priority`: `P0..P3` — bare `0`–`3` are normalized to `P0`–`P3`
  automatically; `P9`-style out-of-vocab values are rejected.
- `--depends-on`: every referenced id must already exist as a task, or the
  create/update aborts (create the dependency task first).
- Header counters (`--set-ticks-total`, `--set-ticks-idle`, ...): negative
  values rejected — counters must be `>= 0`.
- `event --type`: must be one of the enumerated event types (`audit`,
  `tick`, `task_created`, `task_completed`, `task_updated`, `idle`,
  `dogfood`, `e2e_verified`, ... — the error message lists them all).

## Common pitfalls

1. **Bootstrap with `boardctl init` (BT-005).** `boardctl init --project X`
   in an empty repo writes `tasks.jsonl` + `events.jsonl` +
   `board.jsonl` under `.coding-hermes/board/` and exits 0. `create` on the
   empty board works too and emits the full default row schema — no copying
   files from other boards, no hand-seeding needed.
2. **`create` mirrors the LAST row's schema.** On boards with legacy rows your
   new row inherits their key style — that's a feature (byte-stable diffs),
   but check `stats` output for `(none)` status groups after creating on
   odd boards.
3. **Topology B (header on line 1 of tasks.jsonl) is fully writable
   (BT-010).** `create` appends after the header line, `update` rewrites the
   row and bumps the header's `last_commit`, events append normally. The
   old stale-`board.db` write dead-end is gone. Migrating to topology A
   (splitting line 1 into `board.jsonl`) is now an optional manual step,
   not a requirement. Note: `init` still refuses on an existing topology-B
   board (it's for fresh boards only) — and tells you topology B is
   writable when it does.
4. **Prefer `create`/`update` over hand-editing tasks.jsonl.** `update` also
   writes the audit event and bumps `updated_at`; hand edits skip the event
   trail and can mix serialization styles.
5. **`doctor` is your preflight.** It catches tracked `.db`/`.parquet` caches,
   header counter drift, and fixture orphans in ~0.02s on real fleet boards.

## Minimal task-row schema (reference only)

`init` and `create` handle bootstrapping (BT-005), so hand-seeding is no
longer needed — kept here as a reference for what a minimal row looks like:

```json
{"id":"FIRST-1","title":"First task","status":"pending","priority":"P2","created_at":"2026-09-04 09:00:00"}
```

Plus empty `events.jsonl` and a one-line `board.jsonl`:
`{"project":"p","namespace":"ns","version":1,"ticks_total":0,"ticks_idle":0,"cooldown_s":21600,"last_commit":""}`

## CI

The repo has a GitHub Actions workflow since BT-004: build + vet + test on
every push/PR (recent runs green). `boardctl validate` is fast and strict
enough to gate any PR that touches `.coding-hermes/` — a malformed board
fails the workflow.

## Performance envelope (measured 2026-09-04)

0.01–0.02s for stats/list/validate/doctor on the largest real fleet board
(259 rows). Boards are tiny; there is no need to batch or cache.
