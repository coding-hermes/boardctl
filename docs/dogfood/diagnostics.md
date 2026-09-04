# boardctl diagnostics — how it's built, why, and the right way to use it

Written 2026-09-04 from a real-use dogfood run (scratch-board probing, real
fleet-board reads/writes, ephemeral-bunker install test). This is the
explanatory trail: what the tool is, how it works internally, the errors
encountered along the way (its own history AND this run's), and the right
way to do things. Not a log dump.

## How the thing is built

- **Language/runtime:** Go 1.26+ (module
  `github.com/coding-hermes/boardctl`), zero external dependencies (go.mod
  has none — stdlib only). Single binary, `cmd/boardctl` main +
  `internal/board` package.
- **Storage model:** the board IS four JSONL files under
  `.coding-hermes/board/`:
  - `tasks.jsonl` — task rows, one JSON object per line.
  - `events.jsonl` — append-only audit trail (explicit `MAX(id)+1`
    sequencing).
  - `board.jsonl` — one line, the board header (project, namespace, version,
    ticks_total/ticks_idle, cooldown_s, last_commit).
  - `fixtures.jsonl` — perpetual fixture tasks (NEVER-DONE etc.).
- **Two read topologies:** A = all four files as separate tracked files
  (current standard); B = legacy boards where the header is line 1 of
  `tasks.jsonl` (auto-detected, read-tolerated, write-dead-end — BT-010).
- **Why no database:** the board previously carried a DuckDB `board.db`
  cache. It always lagged the JSONL, spawned parity-probe/resync/repair
  ceremony, and produced false-divergence alarms. Removed 2026-09-03. The
  JSONL files are the board; query with boardctl, jq, or
  DuckDB `read_json_auto()`.
- **Write model:** `events.jsonl` is append-only; task-row updates REWRITE
  `tasks.jsonl` but preserve each line's original serialization style
  (per-board detection: sorted vs insertion-order keys, compact vs spaced
  separators) so git diffs stay minimal and byte-stable. This is the design
  win of the whole tool — proven by reformatting a line by hand and watching
  the next update normalize it back to board style, changing only the edited
  field.
- **Performance:** everything is a single linear scan of small files. 0.01s
  stats/validate, 0.02s doctor on the largest real fleet board (259 rows).

## The errors this project's own history produced

1. **The DuckDB-cache era** (pre-2026-09-03): parity probes, cache drift,
   false-divergence alarms — the entire genre of problems the JSONL-canon
   doctrine was designed to kill. `doctor`'s tracked-set check ("no .db or
   .parquet tracked under the board dir") exists precisely to prevent a
   relapse.
2. **The unstamped-version trap:** `go install` builds print `version dev`
   because the module zip isn't stamped; only `make release` binaries get
   the date-stamp. Not a bug — but expect `dev` from @latest installs
   (BT-009 documents this in the install story).
3. **The release-cut gap:** v0.1.1 shipped without `sha256sums.txt` even
   though the Makefile generates it — evidence the cut bypassed
   `make release` (BT-008). The Makefile's `release:` target depends on
   `test` and stamps `main.version` — it is the only sanctioned cut path.
4. **The stale-message residue:** error text still references `board.db` and
   a nonexistent `scripts/` dir — fossils of the removed cache era now
   pinned by BT-005/BT-010.

## Errors hit during this run (and what each taught)

| # | Error | Lesson |
|---|---|---|
| 1 | `no JSONL foreman board found` on an empty scratch repo | Resolver probes `<dir>`, `<dir>/.coding-hermes`, `<dir>/.coding-hermes/board` only — and there is no `init`. Bootstrap gap = BT-005. |
| 2 | `topology B: header lives in board.db …` on empty board files | Empty tasks.jsonl+events.jsonl misdetects as topology B; the message is a DuckDB-era fossil. BT-005/BT-010. |
| 3 | `tasks.jsonl is empty — no row to mirror the schema from` | `create` mirrors the last row's serialization; on a header-only board there's nothing to mirror. Reinforces BT-005. |
| 4 | `-C <repo>/.coding-hermes` → board-not-found | Docs promise this works; resolver doesn't check `<given>/board`. BT-006. |
| 5 | board-not-found exits 1 (README says 2) | Scriptable-contract leak. BT-006. |
| 6 | `--guard MAYBE` / `--ci BANANA` accepted and stored | Only `--status` is vocabulary-checked; junk values persist silently and validate says OK. BT-007. |
| 7 | `--priority 1` stored, real boards use P1/P2/P3 | Two priority vocabularies in the wild; stats groups them separately. BT-007. |
| 8 | `--depends-on GHOST-9` accepted | No existence check on dependency ids. BT-007. |
| 9 | `--set-ticks-total -5` accepted | Negative counters writable; doctor only compares ticks_idle vs ticks_total. BT-007. |
| 10 | v0.1.1 missing sha256sums.txt | Release-cut bypassed `make release`. BT-008. |

## The right way (patterns that survived real use)

1. **Doctor first.** `boardctl -C <repo> doctor` before working any board you
   didn't create — one 0.02s command catches cache relapses, counter drift,
   and orphaned fixtures, exit 1 on real problems.
2. **File tasks with the tool.** `create`/`update` keep row schema and
   serialization coherent and write the audit events; hand-edits split
   styles and skip the event trail. This dogfood run filed BT-005..010 with
   boardctl itself and the board validated clean after.
3. **`update` is the only sanctioned status flip.** It writes the completion
   event and keeps `updated_at` coherent.
4. **Install via static binary** (4s, zero deps, works under a no-sudo bare
   Debian user) — `go install` is for Go users; both proven in the bunker.
   Expect `version dev` from @latest builds.
5. **CI gate:** once BT-004 (GitHub Actions) lands, `boardctl validate` is
   fast and strict enough to gate every PR touching `.coding-hermes/`.

## What a new agent should check when picking this project up

- `go build ./cmd/boardctl && go test ./...` (fast, stdlib-only).
- The board files are git-tracked — NEVER commit a `board.db`/`*.parquet`
  under `.coding-hermes/board/` (doctor will fail the board on purpose).
- BT-005..BT-010 on the board are the open dogfood findings; BT-005 (init)
  and BT-007 (write vocabulary) are the highest-value fixes.
