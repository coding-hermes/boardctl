---
name: boardctl-usage
description: >-
  How to use boardctl — the CLI for coding-hermes JSONL foreman boards
  (tasks/events/board/fixtures under .coding-hermes/board/). Entry points,
  proven commands, error meanings, and pitfalls from a real-use dogfood run.
version: 1.0.0
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
  (unknown command/flag). NOTE: board-not-found currently exits `1`, not the
  README-documented `2` (BT-006).

## The `-C` flag (read this first)

`-C` resolves the board dir. **Proven working:** a repo root (finds
`.coding-hermes/board`) or the board dir itself
(`.coding-hermes/board`). **Broken despite docs:** passing the
`.coding-hermes` dir itself fails (BT-006). Default (no `-C`) = current
directory walked the same way.

```bash
boardctl -C ~/myrepo stats                 # repo root — works
boardctl -C ~/myrepo/.coding-hermes/board validate   # board dir — works
boardctl -C ~/myrepo/.coding-hermes stats  # FAILS (docs say otherwise, BT-006)
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
boardctl version                           # date-stamp on release builds, "dev" otherwise
```

## Status vocabulary (the only enforced one)

`failed, pending, in_progress, review, blocked, complete` — anything else is
rejected with exit 1. `--guard` (PASS/FAIL/SKIP) and `--ci` (GREEN/RED/SKIP)
look enforced but are NOT: any string is accepted and stored (BT-007), so
don't trust those fields blindly on boards written by others.

## Common pitfalls

1. **You cannot bootstrap a new board with boardctl (BT-005).** No board
   files → "no JSONL foreman board found"; empty files → topology-B
   misdetect with a stale DuckDB error; header-only board.jsonl →
   "tasks.jsonl is empty — no row to mirror the schema from". Until BT-005
   lands, copy the four files from an existing board's `.coding-hermes/board/`
   and edit ids — or hand-append a first row matching the schema below.
2. **`create` mirrors the LAST row's schema.** On boards with legacy rows your
   new row inherits their key style — that's a feature (byte-stable diffs),
   but check `stats` output for `(none)` status groups after creating on
   odd boards.
3. **Topology B (header on line 1 of tasks.jsonl) is read-only in practice**
   — writes dead-end with a stale `board.db` error (BT-010). Migrate by
   splitting line 1 into `board.jsonl`.
4. **Prefer `create`/`update` over hand-editing tasks.jsonl.** `update` also
   writes the audit event and bumps `updated_at`; hand edits skip the event
   trail and can mix serialization styles.
5. **`doctor` is your preflight.** It catches tracked `.db`/`.parquet` caches,
   header counter drift, and fixture orphans in ~0.02s on real fleet boards.

## Minimal task-row schema (for hand-seeding, until BT-005)

```json
{"id":"FIRST-1","title":"First task","status":"pending","priority":"P2","created_at":"2026-09-04 09:00:00"}
```

Plus empty `events.jsonl` and a one-line `board.jsonl`:
`{"project":"p","namespace":"ns","version":1,"ticks_total":0,"ticks_idle":0,"cooldown_s":21600,"last_commit":""}`

## Performance envelope (measured 2026-09-04)

0.01–0.02s for stats/list/validate/doctor on the largest real fleet board
(259 rows). Boards are tiny; there is no need to batch or cache.
