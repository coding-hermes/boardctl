# boardctl

CLI for managing [coding-hermes](https://github.com/coding-hermes) JSONL foreman
boards — the single canonical board store (`tasks.jsonl` + `events.jsonl` +
`board.jsonl` header + `fixtures.jsonl`).

`boardctl` reads and writes the JSONL files **directly**. No DuckDB, no parquet,
no cache files, no parity probes. What git tracks is what you get.

## Install

The zero-dependency path: grab a static binary from
[releases](../../releases) (linux amd64/arm64/arm, darwin amd64/arm64, windows
amd64, freebsd amd64) — no Go toolchain needed. Verify the binary's checksum
before running it:

Current release: **v0.1.8**.

```bash
curl -sL -o boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/v0.1.8/boardctl_linux_amd64
curl -sL -o SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/v0.1.8/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS   # verify the binary you downloaded
chmod +x boardctl_linux_amd64 && ./boardctl_linux_amd64 version
```

Assets are named `<binary>_<goos>_<goarch>` (underscores; `.exe` on windows,
e.g. `boardctl_windows_amd64.exe`) plus the `SHA256SUMS` checksum file. Both cut
paths emit exactly these names: the tag-push CI cut and the offline
`make release` (see [Cutting a release](#cutting-a-release-bt-030)).

`SHA256SUMS` lists every published platform, so `--ignore-missing` skips the
ones you did not download (without it, `sha256sum -c` exits 1 on the absent
files). The downloaded binary must keep its release filename for this to verify.

The last line prints the binary's release identity — for a v0.1.8 asset it
prints `boardctl version v0.1.8`. Release binaries are stamped from the release
tag, so the printed version must match the release you downloaded; a bare date
stamp (e.g. `20260915`) or `dev` means the binary was not cut from a tagged
checkout (see [Development](#development)).

Move `boardctl` somewhere on your `PATH` (or invoke it as `./boardctl_linux_amd64`).

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
boardctl version                   # prints e.g. "boardctl version v0.1.8"
                                   # (the release tag; a UTC date for release
                                   # builds from untagged checkouts, "dev" for
                                   # unstamped local builds)
boardctl version --json            # {"version":"v0.1.8","build":"v0.1.8"}
                                   # ("build" is the raw build stamp)

# create a task row (appends tasks.jsonl + task_created event).
# A re-detected finding is REFUSED (exit 2) unless --force; see
# "Finding fingerprints" below.
boardctl -C ~/myproject create --id FEAT-1 --title "Add retry" --priority 1 \
    --reasoning "Idempotent retry with backoff" --capability-tags go,net
boardctl -C ~/myproject create --id FEAT-1 --title "Add retry" \
    --reasoning "Idempotent retry with backoff" --evidence-run-id qa-2026-09-18T04:11Z

# rows can carry WHERE the work happened and WHICH agent runs did it
# (see "Worktree, branch and session fields" below)
boardctl -C ~/myproject create --id FEAT-2 --title "Add caching" \
    --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session 2f8c41a0 --session 91ab77c2

# update a task row (status flip + completion event; --commit-hash stays on
# the task row — the header drift pointer only moves via `header --set-last-commit`)
boardctl -C ~/myproject update FEAT-1 --status complete --commit-hash abc1234 \
    --guard PASS --ci GREEN --summary "retry added (+80/-12)"
boardctl -C ~/myproject update FEAT-1 --worktree /home/me/wt/ft-1 \
    --branch wt/ft-1 --session 2f8c41a0

# canonicalize one row's status/guard_result/ci_result spelling in place
# (read-alias fix path; no-op + "already canonical" when nothing is dirty)
boardctl -C ~/myproject update FEAT-1 --normalize

# append a raw audit event
boardctl -C ~/myproject event --type audit --tick 42 --detail-text 'tick 42 summary'

# read/patch the board.jsonl header
boardctl -C ~/myproject header --json
boardctl -C ~/myproject header --set-ticks-total 42 --set-last-commit abc1234
# on a board with NO header (no board.jsonl and line 1 of tasks.jsonl is a
# task row) both forms refuse with "no header on this board" (exit 1) —
# nothing is written, so a header write can never land in a task row
```

### Headerless boards

A board can hold just `tasks.jsonl` + `events.jsonl` with no `board.jsonl` and
no metadata row — line 1 is an ordinary task row (real case:
`coding-hermes-tools`). That is a **headerless** board: it has no header to
read and no header row to rewrite.

```
boardctl -C <dir> header --set-ticks-total 90   # refused, exit 1,
                                                # "no header on this board"
boardctl -C <dir> header --json                 # refused the same way
boardctl -C <dir> validate                      # exit 0 with one
                                                # [warn] headerless board
                                                # (no board.jsonl) line
```

Reads are unaffected (`list`, `show`, `stats`, `render`, `doctor`, `create`,
`update`, `event` all work — line 1 is enumerated as the task it is), and
`validate`/`doctor` skip the header counter checks with that one note instead
of reporting a task row as a malformed header. Both the refusal and the note
are gated on line 1's SHAPE: a legacy **topology-B** board (line 1 *is* a
header object) stays fully writable, and the write path additionally refuses to
stamp header keys into any row carrying `id`/`title`/`status`.

### serve

`boardctl serve` starts a loopback-only upload server: open it in a browser,
pick a board folder or a `.zip` of board folders, and get the self-contained
multi-board HTML report back. Nothing is written to your board files.

```bash
boardctl serve                       # http://127.0.0.1:8787
boardctl serve --addr 127.0.0.1:9000 # any loopback host/port; a non-loopback
                                     # --addr (0.0.0.0, 10.x, a hostname) is
                                     # refused with exit 2 — serve has no auth
                                     # and must never leave the machine
boardctl serve -C ~/myproject        # the -C board is included in every report
```

The `-C` board, when it resolves, is PREPENDED to every report the server
renders — reports are always `-C` board first, then uploaded boards, so a
two-board zip alongside `-C` renders three boards. A `-C` that resolves to
nothing is fine; the server starts anyway and prints a note.

Every directory that contains BOTH `tasks.jsonl` and `events.jsonl` (at any
depth, in a folder upload or inside a zip) is registered as a board. When the
upload registers none — a file posted with the wrong field name, or a folder
without that pair — the report still renders (the `-C` board if present,
otherwise HTTP 400) and carries a visible banner notice:

```
0 boards found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl
```

## Start a board

A fresh project has no board yet. `init` bootstraps one — it writes the four
topology-A files (`tasks.jsonl`, `events.jsonl`, the `board.jsonl` header, and
`fixtures.jsonl`) under `<dir>/.coding-hermes/board`, nothing else.
`tasks.jsonl` starts seeded with the fleet-standard NEVER-DONE perpetual audit
fixture row, which is registered in `fixtures.jsonl` (the perpetual fixture
registry) as well. No git init, no commits, and
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

## Status vocabulary and read aliases (BT-025)

boardctl WRITES one canonical status vocabulary:

```
pending, in_progress, review, blocked, complete, failed
```

READS additionally accept a fixed alias table (case-insensitive, whitespace
trimmed), because those spellings exist on live fleet boards and validate
would otherwise fail the very boards it exists to check:

```
completed, done                     -> complete
todo, open, reopen, reopened        -> pending
in-progress, inprogress             -> in_progress
```

An alias status is a WARNING on `validate`/`doctor` naming the canonical
value — e.g. `status "todo" is a read alias for "pending"` — so a board full
of legacy spellings still validates OK while the drift stays visible.
Anything NOT canonical and NOT an alias (`retired`, `closed`, `wip`,
malformed junk) remains an ERROR: ambiguous statuses need a human decision,
not a silent coercion.

Aliases are read-only: every write surface (`create`, `update`, `import`)
still rejects them, and nothing is auto-fixed on read. The sanctioned fix
path is `boardctl update <id> --normalize`, which rewrites that row's
`status` (via the alias table) and `guard_result`/`ci_result` (upper-cased)
to their canonical forms in place — every untouched line stays
byte-identical, it refuses to write anything when a present field cannot be
resolved, it is a no-op ("already canonical") on a clean row, and it never
requires `--force`.

```bash
boardctl -C ~/myproject validate                    # alias rows warn (exit 0)
boardctl -C ~/myproject update FEAT-1 --status done # exit 1 — writes reject aliases
boardctl -C ~/myproject update FEAT-1 --normalize   # status "done" -> "complete"
```

`sweep-status` drives the same policy board-wide on ONE board: it reports every
row whose status is off-vocabulary, splitting them into known-alias rows
(fixable by --normalize) and unknown rows (`duplicate`, `parked`, ... — never
guessed at; they need an explicit `--status` decision). Dry-run by default;
`--apply` canonicalizes ONLY the alias rows through the same normalize
machinery and prints before/after counts; `--json` mirrors the report for
scripting.

```bash
boardctl -C ~/myproject sweep-status                # report only, zero writes
boardctl -C ~/myproject sweep-status --apply        # normalize the alias rows
```

## Task id format

Task ids are machine keys — downstream fleet tooling (task-router chains,
board-scan dedupe) parses and groups by id, so `create` and `update` enforce
the fleet id format at write time:

```
^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$     e.g. BT-023, QA-BOARDCTL-001, NEVER-DONE
```

An id is an uppercase prefix, optionally followed by digits, plus at least
one hyphenated segment. Ids like `"bad id!"`, `bt-023`, or single-segment
`TODO` are rejected with exit 1. `--force` (on both `create` and `update`)
is the escape hatch: it writes a non-conforming id anyway. `update --force`
exists so legacy junk rows already on a board stay updatable.

`validate` stays silent about ids already on a board (legacy boards must not
newly fail validation); `doctor` itemizes each non-conforming id as a
warning with its `tasks.jsonl` line number — never an error.

```bash
boardctl -C ~/myproject create --id "bad id!" --title x          # exit 1
boardctl -C ~/myproject create --id "bad id!" --title x --force  # written
boardctl -C ~/myproject doctor    # ... task id "bad id!" does not match ... (warn)
```

## Finding fingerprints (SG-126)

QA and dogfood lanes used to re-file the same finding 3-8x: a fresh id against
a recycled title, so every `create` succeeded and the board grew while the real
finding count stayed flat. Board rows now carry a **finding fingerprint** — a
SHA-256 over the normalized `title` + `reasoning` (lowercased, leading
`QA-<PROJ>-N`/`DF-<PROJ>-N`/`DOGFOOD-<PROJ>-N` prefix stripped, whitespace
collapsed, trailing `(recurrence N)` / `(Nth)` / `[run-id-N]` stripped).

The digest lives INSIDE the existing `detail` field — no new column, no
migration — with the original prose preserved:

```json
"detail": {"fingerprint":"9f2c…","evidence":[{"run_id":"qa-…","ts":"…"}],"text":"original prose"}
```

`create` refuses a row whose fingerprint is already OPEN, prints

```
DUPLICATE-SUPPRESSED: <existing-id> matches fingerprint <hex>
```

on stderr and records the re-detection as a `task_evidence` event on the
existing row — the board gains an audit line, never another row. The exit
code is **2**, and that is a rule a lane dispatcher can branch on, not a
one-off:

```
exit 0  success (create accepted)
exit 1  plain failure (validation, duplicate task id, runtime errors)
exit 2  duplicate-suppressed re-detection: DUPLICATE-SUPPRESSED on stderr
        plus a task_evidence event on the existing row (2 doubles as the
        usage/board-not-found code used elsewhere)
```

Evidence the EXISTING row with `boardctl update <id>` — carry the run id in
`--summary` or `--note`; `update` does not accept `--evidence-run-id`. That
flag exists ONLY on `create`, stamping evidence on a NEW filing:

```bash
boardctl -C ~/myproject update QA-X-3 --summary "re-detected on run qa-1"     # evidence on an existing row
boardctl -C ~/myproject create --id QA-X-9 --title "…" --evidence-run-id qa-1 # new filing only
```

`--force` files a deliberate
variant (still fingerprinted, so a later accidental re-file is caught). Rows
written before this rule have no fingerprint and are grandfathered: still open
and visible, they simply never match an incoming filing. An open row is
`pending`, `in_progress`, `dispatched`, `blocked`, or `review` — anything else
never suppresses a create.

Existing duplicate groups are collapsed by the one-shot backfill, which is
**dry-run by default**, runs on exactly ONE board (`--board-dir` is required —
there is no fleet sweep), and refuses a board whose lane is not in `--lanes`:

```bash
dedupe-board --board-dir ~/myproject            # report only, zero writes
dedupe-board --board-dir ~/myproject --apply    # collapse the groups
```

`dedupe-board` is **source-only** — it is a second entrypoint, not a release
asset (every published asset is the `boardctl` CLI). Install it from the module:

```bash
go install github.com/coding-hermes/boardctl/cmd/dedupe-board@latest
```

Per group (≥ 2 open rows sharing a fingerprint) it keeps the earliest row, closes
the rest with `worker_summary="merged into <kept>: dedupe backfill <date>"`,
carries the merged evidence onto the kept row, and writes one `audit` event.
Full rules: [`docs/board-fingerprint-rules.md`](docs/board-fingerprint-rules.md).

## Worktree, branch and session fields

A task row carries WHERE the work happened and WHICH agent runs worked it.
Both `create` and `update` accept three first-class fields for that:

| Flag | Row field | Type | Meaning |
|------|-----------|------|---------|
| `--worktree PATH` | `worktree` | string | absolute path of the git worktree the task is built in |
| `--branch NAME` | `branch` | string | that worktree's git branch, e.g. `wt/cht-031` |
| `--session ID` | `sessions` | array of strings | Hermes session id(s) that worked the row — repeatable |

```bash
boardctl -C ~/myproject create --id FEAT-2 --title "Add caching" \
    --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session 2f8c41a0 --session 91ab77c2
boardctl -C ~/myproject update FEAT-2 --status complete \
    --worktree /home/me/wt/ft-2 --branch wt/ft-2 --session 91ab77c2
```

Rules that hold on both commands:

- **Absent is valid.** A flag that was not passed writes NO key — never an
  empty string and never an empty array. Work done in the main checkout simply
  carries no `worktree`/`branch`; a row whose runs were never recorded carries
  no `sessions` array. Nothing is inferred from the environment.
- **Appended key order, byte preservation.** A newly set field is APPENDED at
  the end of the target row's key order; existing keys keep their order and
  their verbatim value bytes, and every other line of `tasks.jsonl` stays
  byte-identical (the usual surgery contract, enforced by assertion before the
  write).
- **`sessions` is an ARRAY, replaced wholesale.** `--session A` writes
  `["A"]`; `--session A --session B` writes `["A","B"]` in the order given.
  Setting it REPLACES the row's whole list — the array is never merged
  element-wise, so the row reflects the run(s) recorded last instead of
  accumulating stale ids. `--session=ID` works too, and the flags parse in any
  order (before or after the task id).
- **The board's own style wins.** The array and strings are encoded through the
  same serializer every other field uses, so a compact board stays compact and
  a spaced board stays spaced.
- **`--normalize` refuses them.** `--worktree`, `--branch` and `--session` are
  change flags: `update` needs at least one change flag (or `--normalize`), and
  `--normalize` rejects a combination with any of the three — a usage error
  with nothing written, like its existing change-flag siblings.

No other command needed changing: `validate`, `doctor`, `stats`, `render`,
`list`, `show` tolerate unknown keys (the fields never produce a finding), and
`import` copies exported rows verbatim, so the three fields round-trip with the
row.

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

A third, degenerate layout is a **HEADERLESS** board: no `board.jsonl` AND
line 1 of `tasks.jsonl` is not a header object but an ordinary task row. It
has no header, so `header` refuses to read or write one (exit 1, "no header on
this board"), `validate`/`doctor` note it and skip the header counter checks,
and every other command treats line 1 as the task it is. The distinction from
topology B is line 1's SHAPE, and the write path refuses on top of it: a
header key is never written into a row carrying `id`/`title`/`status`.

Writes are append-only for `events.jsonl`; task-row updates rewrite
`tasks.jsonl` preserving each line's original serialization style (detected
per-board: sorted vs insertion-order keys, compact vs spaced separators) so
git diffs stay minimal and byte-stable.

## Repository layout

| Path | Purpose |
|------|---------|
| `cmd/boardctl/` | CLI entrypoint (`main.go`) |
| `cmd/dedupe-board/` | SG-126 one-shot dedupe backfill (dry-run by default; source-only — `go install github.com/coding-hermes/boardctl/cmd/dedupe-board@latest`) |
| `internal/board/` | Board engine — read/write/validate/doctor/init plus JSONL style handling |
| `docs/board-fingerprint-rules.md` | Finding-fingerprint rule, evidence-over-refile, backfill usage |
| `.coding-hermes/board/` | This repo's own dogfood board (`tasks.jsonl`, `events.jsonl`, `board.jsonl`, `fixtures.jsonl`) |
| `docs/dogfood/` | Dogfood diagnostics and integration notes |
| `skills/boardctl-usage/` | Fleet skill: how agents drive `boardctl` |
| `.github/workflows/ci.yml` | CI (build, vet, test) |
| `.gitreins/` | GitReins config + `history/<date>/<hash>/` judge verdicts (tracked audit records — keep) |
| `Makefile`, `go.mod` | Build/test/release targets (see [Development](#development)) |
| `LICENSE`, `README.md`, `.gitleaks.toml`, `.gitignore` | Repo metadata and hygiene |

Intentionally not tracked (gitignored, regenerated on demand):

- `bin/`, `dist/` — build and release outputs (`make build`, `make release`); `make clean` removes them
- `dagger.db` (+ `-shm`/`-wal`) — local DAGger cache, recreated when DAGger runs

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
make fmt-check        # gofmt gate (CI enforces it too); `make fmt` fixes
make version-check    # README release-pin gate (CI enforces it too)
make vuln-check       # dependency-vulnerability gate (CI enforces it too)
make release          # cross-compile all targets into dist/ (same asset names + SHA256SUMS the CI cut publishes)
```

`make vuln-check` (BT-033) runs `govulncheck` over the module and fails on any
reachable vulnerability, or on a scan that did not complete. A non-zero exit
that is neither 0 (clean) nor 3 (findings) is reported as a `TOOL-ERROR` —
never silently accepted as a pass. Failures name the vulnerability ids and the
version that fixes each one. Local runs need the pinned scanner:

```bash
go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
```

CI installs that same pinned version in its own step, so the check runs (and
never skips) on every push and PR. `GOVULNCHECK=/path/to/binary` overrides the
PATH lookup.

The toolchain requirement: go.mod declares `go 1.26.6`. The Go 1.26.5 standard
library shipped reachable advisories that 1.26.6 fixes — `net/http`
(GO-2026-6089), `crypto/tls` (GO-2026-6090) and `encoding/asn1`
(GO-2026-5972) — so a lower `go` directive, or a build with an older
toolchain, fails this gate. With `GOTOOLCHAIN=auto` (the default) the pinned
patch toolchain is fetched automatically. Do not lower the directive to
silence the gate; raise it to the patched release instead.

Cutting a release is one command: push a `vX.Y.Z` tag. The repo's
[`.github/workflows/multiarch.yml`](.github/workflows/multiarch.yml) calls the
org reusable multi-arch workflow (`coding-hermes/.github`), which cross-builds
every published target, writes `SHA256SUMS`, attests build provenance and
creates the GitHub release. `make release` is the offline/local equivalent and
emits the SAME asset names (see the procedure below). Never cut a release by
hand (tag + manual asset upload) without its checksum file — that is how v0.1.1
shipped without one (BT-008).

### Cutting a release (BT-030)

The release identity must flow from the git tag into every shipped binary —
the published v0.1.3 assets stamped `20260915` (a build date) instead of the
release, which is exactly what this procedure prevents. The tag-push CI path is
the primary cut:

1. Move every README release surface to the new tag — the
   `Current release: **vX.Y.Z**` line and every `/releases/download/<tag>/`
   install URL — then run `make version-check`, which FAILS naming the drifted
   values until they all agree (CI runs the identical check on every push and
   PR). Commit and push `main`.
2. Tag the release commit and push the tag:
   `git tag v0.1.8 && git push origin v0.1.8`. The tag push triggers
   `.github/workflows/multiarch.yml`, whose Release job runs on tag refs only:
   it cross-builds the platform set in the workflow's `platforms:` input
   (`boardctl_<goos>_<goarch>`, `.exe` on windows), writes `SHA256SUMS`,
   attests provenance, and publishes the release. Push the branch too — the
   Release job runs on the tag ref and the tag must point at a pushed commit.
3. Verify the published release: `gh release view v0.1.8` lists the seven
   platform binaries plus `SHA256SUMS`; then download per the install block and
   confirm the identity — `./boardctl_linux_amd64 version` prints
   `boardctl version v0.1.8`.
4. Offline / local equivalent, when CI cannot publish: `make release` from the
   tagged checkout. VERSION resolves via `git describe --tags --abbrev=0` (a UTC
   date only on checkouts with no tags), and every binary reports it:
   `./dist/boardctl_linux_amd64 version` prints `boardctl version v0.1.8`. If
   the repo HAS tags but VERSION resolved to a non-tag, the target FAILS instead
   of silently shipping a date stamp; `make release VERSION=v0.1.8` overrides
   deliberately. It writes the same underscore names and `dist/SHA256SUMS` the
   CI job publishes, so publish them verbatim:

   ```bash
   gh release create v0.1.8 --title v0.1.8 --generate-notes --verify-tag dist/*
   ```

   Never delete and re-push a tag to re-cut a release: that re-drafts the
   published release.

Scripts read the identity as JSON: `boardctl version --json` prints
`{"version":"v0.1.8","build":"v0.1.8"}` — `build` is the raw build stamp and
differs from `version` only when a binary was hand-stamped with a non-tag
value while carrying an embedded tag.

## License

MIT
