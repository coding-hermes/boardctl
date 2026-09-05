# boardctl

CLI for managing [coding-hermes](https://github.com/coding-hermes) JSONL foreman
boards — the single canonical board store (`tasks.jsonl` + `events.jsonl` +
`board.jsonl` header + `fixtures.jsonl`).

`boardctl` reads and writes the JSONL files **directly**. No DuckDB, no parquet,
no cache files, no parity probes. What git tracks is what you get.

## Install

The zero-dependency path: grab a static binary from
[releases](../../releases) (linux/darwin/windows/freebsd × amd64/arm64/arm) —
no Go toolchain needed. Verify the binary's checksum before running it:

```bash
curl -sL -o boardctl https://github.com/coding-hermes/boardctl/releases/download/v0.1.1/boardctl-linux-amd64
curl -sL -o sha256sums.txt https://github.com/coding-hermes/boardctl/releases/download/v0.1.1/sha256sums.txt
sha256sum -c sha256sums.txt   # verify the binary's checksum
chmod +x boardctl && ./boardctl version
```

Move `boardctl` somewhere on your `PATH` (or invoke it as `./boardctl`).

With a Go toolchain, `go install` works too:

```bash
go install github.com/coding-hermes/boardctl/cmd/boardctl@latest
```

Or build directly from a checkout (see [Development](#development)):

```bash
go build ./cmd/boardctl
```

## Usage

```bash
# -C resolves a repo root, .coding-hermes, or the board dir itself:
#   -C ~/myproject                    -> ~/myproject/.coding-hermes/board
#   -C ~/myproject/.coding-hermes     -> ~/myproject/.coding-hermes/board
#   -C ~/myproject/.coding-hermes/board -> the board dir itself
boardctl -C ~/myproject stats
boardctl -C ~/myproject list --status pending
boardctl -C ~/myproject show DF-MYPROJECT-1 --events
boardctl -C ~/myproject validate
boardctl -C ~/myproject doctor     # validate + deep checks: git tracked-set
                                   # (no .db/.parquet), header vs events ticks,
                                   # fixture orphans
boardctl version                   # prints e.g. "boardctl version 20260903"
                                   # ("dev" for unstamped go build/go install)

# create a task row (appends tasks.jsonl + task_created event)
boardctl -C ~/myproject create --id FEAT-1 --title "Add retry" --priority 1 \
    --reasoning "Idempotent retry with backoff" --capability-tags go,net

# update a task row (status flip + completion event + header bump)
boardctl -C ~/myproject update FEAT-1 --status complete --commit-hash abc1234 \
    --guard PASS --ci GREEN --summary "retry added (+80/-12)"

# append a raw audit event
boardctl -C ~/myproject event --type audit --tick 42 --detail-text 'tick 42 summary'

# read/patch the board.jsonl header
boardctl -C ~/myproject header --json
boardctl -C ~/myproject header --set-ticks-total 42 --set-last-commit abc1234
```

## Start a board

A fresh project has no board yet. `init` bootstraps one — it writes the three
topology-A files (`tasks.jsonl`, `events.jsonl`, and the `board.jsonl` header)
under `<dir>/.coding-hermes/board`, nothing else. No git init, no commits, and
it is no-clobber: re-running on an initialized board just prints "already
initialized" (exit 0).

```bash
cd ~/myproject                     # any dir; --project defaults to its name
boardctl init                      # optional: --project NAME --namespace NS
boardctl create --id TASK-1 --title "First task"
boardctl stats
```

The first `create` runs against an empty `tasks.jsonl`, so it builds the row
from a built-in default schema (the standard fleet task fields) instead of
mirroring a previous row. If a directory holds a legacy topology-B board (the
header is line 1 of `tasks.jsonl`), `init` refuses — init bootstraps NEW
boards, and that board already exists and is fully writable as-is. Migration
to topology A (splitting line 1 of `tasks.jsonl` into `board.jsonl`) is an
optional modernization, done by hand outside init.

Exit codes: `0` ok, `1` validation failure, `2` usage/board-not-found.

## Board topology

```
.coding-hermes/board/
  tasks.jsonl     task rows (id, title, status, priority, model assignment, ...)
  events.jsonl    append-only audit/events (explicit MAX(id)+1 sequence)
  board.jsonl     header: project, namespace, tick counters, cooldown, last_commit
  fixtures.jsonl  perpetual fixture tasks (NEVER-DONE, E2E-001, ...)
```

Two tracked topologies are tolerated:
- **A** — all four files tracked in git (current standard)
- **B** — legacy boards where `board.jsonl` is a header object on line 1 of
  `tasks.jsonl` (auto-detected)

Both topologies are FULLY WRITABLE (BT-010): on topology B the header is read
from and rewritten on line 1 of `tasks.jsonl`, task rows are appended after
it, and `validate`/`doctor` check the line-1 header counters the same way
they do on topology A. `init` remains for fresh boards only.

Writes are append-only for `events.jsonl`; task-row updates rewrite
`tasks.jsonl` preserving each line's original serialization style (detected
per-board: sorted vs insertion-order keys, compact vs spaced separators) so
git diffs stay minimal and byte-stable.

## Why no database

The board used to carry a DuckDB `board.db` cache. It always lagged the JSONL,
spawned an entire genre of parity-probe/resync/repair ceremony, and produced
false-divergence alarms. Removed 2026-09-03: the JSONL files **are** the board.
Query them with `boardctl`, `jq`, or DuckDB's `read_json_auto()` — no cache to
drift.

## Development

```bash
go build ./cmd/boardctl
go test ./...
make release   # cross-compile all targets into dist/
```

`make release` is the only sanctioned path for cutting a release — it builds
every target and generates `dist/sha256sums.txt`, which must be uploaded as a
release asset alongside the binaries. Cutting a release by hand (tag + manual
asset upload) skips the checksum file; that is how v0.1.1 shipped without one
(BT-008).

## License

MIT
