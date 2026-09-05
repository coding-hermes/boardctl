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
  `tasks.jsonl` (auto-detected, fully writable since BT-010: `create`
  appends after the header line, `update` rewrites the row and bumps the
  header's `last_commit`, events append normally; `events.jsonl` stays
  headerless in both topologies; migrating B → A is an optional manual
  step).
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
- **CI:** GitHub Actions (`.github/workflows/ci.yml`) runs build + vet +
  test on every push/PR (BT-004, landed 2026-09-05; recent runs green).

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
4. **The stale-message residue:** the pre-BT-005/BT-010 binary referenced
   `board.db` and a nonexistent `scripts/` dir in error text — fossils of
   the removed cache era. Rewritten since; the current binary's messages
   carry no cache-era residue (grep-verified in `internal/`+`cmd/`).

## Errors hit during this run (and what each taught)

| # | Error | Lesson |
|---|---|---|
| 1 | `no JSONL foreman board found` on an empty scratch repo | Resolver probes `<dir>`, `<dir>/.coding-hermes`, `<dir>/.coding-hermes/board` only. Was a bootstrap gap (BT-005); `boardctl init --project X` now bootstraps a fresh board there. |
| 2 | `topology B: header lives in board.db …` on empty board files | Empty tasks.jsonl+events.jsonl misdetected as topology B with a DuckDB-era message. Fixed (BT-005/BT-010): fresh boards get `init`, and real topology-B boards are fully writable — the stale message is gone from the binary. |
| 3 | `tasks.jsonl is empty — no row to mirror the schema from` | `create` mirrors the last row's serialization; on a header-only board there was nothing to mirror. Fixed (BT-005): `create` on an empty board emits the full default row schema instead. |
| 4 | `-C <repo>/.coding-hermes` → board-not-found | Docs promised this works; resolver didn't check `<given>/board`. Fixed (BT-006): all three `-C` forms (repo root, `.coding-hermes`, board dir) exit 0. |
| 5 | board-not-found exits 1 (README says 2) | Scriptable-contract leak. Fixed (BT-006): board-not-found now exits 2, matching the README (0 ok / 1 validation / 2 usage-or-not-found). |
| 6 | `--guard MAYBE` / `--ci BANANA` accepted and stored | Only `--status` was vocabulary-checked. Fixed (BT-007): guard `{PASS,FAIL,SKIP}` and ci `{GREEN,RED,SKIP}` are enforced at write — junk values exit 1. |
| 7 | `--priority 1` stored, real boards use P1/P2/P3 | Two priority vocabularies in the wild; stats grouped them separately. Fixed (BT-007): bare 0–3 normalized to P0–P3, other out-of-vocab values rejected. |
| 8 | `--depends-on GHOST-9` accepted | No existence check on dependency ids. Fixed (BT-007): create/update aborts when a referenced id doesn't exist. |
| 9 | `--set-ticks-total -5` accepted | Negative counters were writable. Fixed (BT-007): header counters must be >= 0 — negatives exit 1. |
| 10 | v0.1.1 missing sha256sums.txt | Release-cut bypassed `make release`. Fixed (BT-008): checksums documented and generated on the sanctioned cut path. |

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
5. **CI gate:** live since BT-004 — the GitHub Actions workflow runs
   build + vet + test on every push/PR and recent runs are green, so
   `boardctl validate` effectively gates every PR touching
   `.coding-hermes/`.

## What a new agent should check when picking this project up

- `go build ./cmd/boardctl && go test ./...` (fast, stdlib-only).
- The board files are git-tracked — NEVER commit a `board.db`/`*.parquet`
  under `.coding-hermes/board/` (doctor will fail the board on purpose).
- BT-005..BT-011 are all COMPLETE (init, `-C` resolution contract, write
  vocabulary, topology-B writability, audit events, CI, release checksums,
  README install story) — the board is clean except the NEVER-DONE
  perpetual fixture. The open thread is docs freshness: this handbook and
  `skills/boardctl-usage/SKILL.md` were refreshed against the live binary
  (BT-012); when behavior changes again, update those two files in the same
  change-set as the code.
