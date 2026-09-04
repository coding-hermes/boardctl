# boardctl integration report — dogfood run 2026-09-04

## What this is

`boardctl` is a single-binary Go CLI that reads and writes a coding-hermes
foreman board: four JSONL files (`tasks.jsonl`, `events.jsonl`, `board.jsonl`
header, `fixtures.jsonl`) under `.coding-hermes/board/`. No database, no
caches — what git tracks is the board. This report is from a real-use run:
scratch-board probing of every command, read/write runs against real fleet
boards (hermes-canopy 259 rows, duckbrain 126 rows, scheduler 176 rows), an
ephemeral-bunker install test, and using boardctl to file its own dogfood
findings (BT-005..BT-010) on its own board.

## The 5-minute real workflow (proven)

```bash
# 1. install (fastest proven path — 4s)
curl -sL -o ~/bin/boardctl \
  https://github.com/coding-hermes/boardctl/releases/download/v0.1.1/boardctl-linux-amd64
chmod +x ~/bin/boardctl

# 2. point it at ANY repo that has a .coding-hermes/board/ with the JSONL files
boardctl -C ~/myproject stats            # counts by status/priority
boardctl -C ~/myproject list --status pending
boardctl -C ~/myproject show BT-005 --events
boardctl -C ~/myproject validate         # JSONL shape + header parse
boardctl -C ~/myproject doctor           # + git tracked-set, counter drift, fixture orphans

# 3. the foreman loop verbs
boardctl -C ~/myproject create --id FEAT-7 --title "Add retry" --priority P1 \
    --reasoning "why" --capability-tags go,net
boardctl -C ~/myproject update FEAT-7 --status complete --commit-hash $(git rev-parse --short HEAD) \
    --guard PASS --ci SKIP --summary "retry added"
boardctl -C ~/myproject event --type audit --tick 42 --detail-text "tick summary"
boardctl -C ~/myproject header --set-ticks-total 43
```

## Measured numbers (this run)

| Probe | Result |
|---|---|
| `stats`/`list`/`validate` on 259-row board | 0.01s |
| `doctor` on 259-row board | 0.02s |
| Release-binary install on bare Debian 13 (no sudo, no Go) | 4s to working CLI |
| `go install` path (after 66MB toolchain setup) | 16s |
| `go install ...@latest` version stamp | prints `dev` (module zip is unstamped — expected) |
| Byte-stability on rewrite | held: a hand-mangled line was rewritten back to the board's own serialization style with only the changed field differing |
| Filing 6 tasks on the real board | all appended, `validate` + `doctor` clean after |

## What works well

- **Speed.** Sub-0.02s on the largest real boards. No daemon, no cache to warm.
- **Zero deps.** Static binary; the bunker agent (bare Debian user, no sudo)
  ran everything from `~/bin` with nothing but curl.
- **Byte-stable rewrites.** Task-row updates preserve each line's original
  serialization style, so `git diff` after an update is exactly the changed
  field. This is the core promise and it held under a hostile test (a line
  hand-reformatted with extra spaces got normalized back to board style, not
  exploded to a full-file reformat).
- **`doctor` deep checks are real.** It caught: a `.db` file tracked under the
  board dir, an orphaned fixture (fixture id with no tasks.jsonl row), and
  header counter drift (`ticks_idle > ticks_total`). Exit 1 with precise
  messages.
- **Self-hosting.** The tool manages its own board; this run filed BT-005..010
  with boardctl itself and the board validated clean afterwards.

## Where it bites (all filed as BT-005..BT-010)

1. **No bootstrap.** A brand-new project cannot get a board from the CLI. With
   no board files at all, every command says "no JSONL foreman board found".
   Hand-make two empty files → misdetected as legacy topology B with a stale
   DuckDB-era error (`header lives in board.db` — board.db was removed
   2026-09-03 and no `scripts/` dir exists). Seed a header-only
   `board.jsonl` → `create` refuses: "tasks.jsonl is empty — no row to mirror
   the schema from". Only boards that already have ≥1 task row are writable.
   → BT-005 (add `boardctl init`).
2. **`-C` contract leak.** README and help promise `-C` accepts "a
   .coding-hermes dir"; it doesn't (it looks for `.coding-hermes/board`
   *under* the given dir). Board-not-found exits `1`, not the documented `2`.
   → BT-006.
3. **Loose write vocabulary.** `--guard MAYBE`, `--ci BANANA`, `--priority 1`
   (real boards use `P1/P2/P3`), `--depends-on <nonexistent>`,
   `--set-ticks-total -5`, and free-form `--type` are all accepted; `validate`
   stays silent on the resulting junk. Only `--status` is checked.
   → BT-007.
4. **v0.1.1 has no `sha256sums.txt`** (v0.1.0 did; `make release` generates
   one, so the cut bypassed it). → BT-008.
5. **Install docs lead with the Go path**; the static binary is the
   zero-dep path a fresh user actually wants. → BT-009.
6. **Topology B (header-on-line-1) is read-only in practice**, with the stale
   error text from (1). → BT-010.
7. **The events trail is silent for the main verbs.** README says `create`
   appends a `task_created` event and `update` a completion event + header
   bump — neither happens (source: `AppendEvent` is only reachable from the
   `event` subcommand). Discovered because 7 task filings left
   `events.jsonl` untouched. → BT-011.

## Errors hit and their meaning

| Error | Meaning / right way |
|---|---|
| `no JSONL foreman board found: <dir> (looked for ...)` | The resolver checked `<dir>`, `<dir>/.coding-hermes`, `<dir>/.coding-hermes/board`. Pass the repo root or the board dir itself — NOT `.coding-hermes` (BT-006). |
| `topology B: header lives in board.db (DuckDB cache) — use scripts/update header manually` | Stale message. Real topology B = header on line 1 of `tasks.jsonl`. Writes to topology-B boards dead-end here (BT-005/BT-010). |
| `tasks.jsonl is empty — no row to mirror the schema from` | `create` mirrors the last row's key style; on an empty board there is nothing to mirror. Bootstrap is impossible without hand-editing (BT-005). |
| `status "banana" not in write vocabulary {failed,pending,in_progress,review,blocked,complete}` | The ONE enforced vocabulary. Note `guard`/`ci` look similar but are NOT checked (BT-007). |
| `task "X" not found (searched tasks.jsonl and fixtures.jsonl)` | `show`/`update` also search fixtures — perpetual fixture rows are first-class. |

## Right-way patterns for agents

- Always run `boardctl -C <repo> doctor` before touching a board you didn't
  create — it catches tracked caches, counter drift, and fixture orphans in
  one shot, exit 1 on real problems.
- File findings with the tool, not a text editor: `create` keeps the row
  schema consistent with the board's own serialization; hand-appended rows
  can split the file into mixed styles.
- `update` is the only sanctioned way to flip status — it writes the
  completion event and keeps `updated_at` coherent. Hand-editing tasks.jsonl
  skips the event trail.
- For CI: `boardctl validate` is fast (0.01s) and strict about shape; use it
  as a PR gate once BT-004 (GitHub Actions) lands.
