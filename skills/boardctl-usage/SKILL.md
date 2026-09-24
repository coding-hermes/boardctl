---
name: boardctl-usage
description: >-
  How to use boardctl — the CLI for coding-hermes JSONL foreman boards
  (tasks/events/board/fixtures under .coding-hermes/board/). Entry points,
  proven commands, error meanings, and pitfalls from a real-use dogfood run.
version: 1.6.0
category: software-development
---

# boardctl usage

`boardctl` manages a coding-hermes foreman board: four JSONL files under
`.coding-hermes/board/` — `tasks.jsonl` (task rows), `events.jsonl`
(append-only audit), `board.jsonl` (header: project/namespace/tick
counters/last_commit), `fixtures.jsonl` (perpetual fixture tasks). No
database, no caches: what git tracks is the board.

Run-15 (2026-09-24) additions: `sweep-status` + `install` in Proven commands;
`--force` on create does NOT bypass the dangling-dep check (only the id-format
guard); `update` still has no `--priority/--title` (BT-060 half-open).

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

# worktree / branch / Hermes session audit trail (first-class row fields).
# Omitted flag = NO key written (main-checkout work has no worktree);
# --session is repeatable and REPLACES the row's sessions array, in order.
boardctl -C R create --id FEAT-2 --title "T" \
    --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session <hermes-session-id> --session <second-session-id>
boardctl -C R update FEAT-2 --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session <hermes-session-id>
boardctl -C R event --type audit --tick 42 --detail-text "..."   # or --detail @file
boardctl -C R header --set-ticks-total 42 --set-last-commit <sha>
boardctl init --project myproject          # bootstrap a fresh board: writes
                                           # tasks/events/board/fixtures.jsonl
                                           # + the NEVER-DONE seed row
boardctl version                           # release tag on release builds
                                           # (e.g. "v0.1.3"), "dev" otherwise;
                                           # --json for scripts

# status sweep (run 15, v0.1.8+, REVIEW-BOARDCTL-001)
boardctl -C R sweep-status                 # census of off-vocabulary statuses:
                                           # X/Y rows, N fixable --normalize,
                                           # M need explicit decision.
                                           # EXITS 0 IN BOTH MODES — the census
                                           # is a finding, not a failure. DRY
                                           # RUN by default; --apply in-place
                                           # normalizes ONLY alias rows
                                           # (todo/done/open/... -> canon) via
                                           # the BT-025 byte-preserving path;
                                           # explicit rows (duplicate/parked/
                                           # retired) are NEVER touched.

# pre-commit board lint (run 15, v0.1.8+, BT-054)
boardctl -C R install [--dry-run]          # writes .git/hooks/pre-commit with
                                           # a managed `boardctl validate`
                                           # block. A slow/missing/wedged
                                           # boardctl SKIPS (never wedges
                                           # commits); timeout(1) required.
                                           # --dry-run prints the hook block
                                           # (useful to inspect an existing
                                           # install's chaining). Chained
                                           # installs (gitreins + board-lint)
                                           # honor BOTH exit statuses.
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
- `--worktree` / `--branch` / `--session` (create and update): free-form
  values — `--worktree` is the absolute path of the git worktree the task is
  built in, `--branch` its branch, `--session` the Hermes session id that
  worked the row (repeatable; the array is REPLACED wholesale, never merged).
  They are change flags for `update` and are refused in combination with
  `--normalize`.
- `event --type`: must be one of the enumerated event types (`audit`,
  `tick`, `task_created`, `task_completed`, `task_updated`, `idle`,
  `dogfood`, `e2e_verified`, ... — the error message lists them all).

## Common pitfalls

1. **Bootstrap with `boardctl init` (BT-005, four files since BT-016).**
   `boardctl init --project X` in an empty repo writes `tasks.jsonl` +
   `events.jsonl` + `board.jsonl` + `fixtures.jsonl` (with the fleet
   NEVER-DONE perpetual fixture row) under `.coding-hermes/board/` and
   exits 0. Re-running is a no-clobber no-op (exit 0). `create` on the
   empty board works too and emits the full default row schema — no
   copying files from other boards, no hand-seeding needed.
2. **Task ids are machine keys — writes enforce the fleet format (fixed after
   BT-023).** `create --id "bad id!"` exits 1 (`^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$`);
   `--force` writes it anyway for legacy junk. `validate` stays silent about
   ids already on a board (legacy boards must not newly fail); `doctor`
   itemizes them as warnings. NEVER recycle an id across cycles: QA lanes
   that refile `QA-X-1` each cycle instead of advancing produce boards with
   6 DISTINCT findings under one id (asce, ai_plays_poke, hivemind-work,
   dexdat-memory, speclang, 2026-09-22) — validate errors, and dedupe-board
   correctly refuses to collapse them because fingerprints are over text,
   not ids. Always mint a fresh id for a new finding.
3. **`update --commit-hash` stays on the task row (BT-024 fixed).** The
   header's `last_commit` drift pointer moves only via
   `header --set-last-commit`; keep the two apart when writing tooling.
4. **`validate` enforces a status vocabulary the fleet doesn't fully
   speak (BT-025).** Real foreman rows with `todo`/`done`/`open` produce
   ERRORs (exit 1) — on boards with legacy statuses, treat validate's
   status errors as an alias-policy question, not data corruption.
5. **tasks.jsonl-only boards refuse with a precise error (BT-026 fixed).**
   The message names the missing events.jsonl and the exit is 2. Treat it
   as board-not-found (the board is not a board without its audit trail).
   Related, CORRECTED 2026-09-22: the old note here claimed the legacy
   pretty-printed boards were "valid-on-disk, jq-readable". They are NOT —
   jq fails on all of them (helios: raw newlines inside strings; consensus:
   truncated rows; ring-runner: a bunker banner pasted mid-row). All-or-
   nothing parse failure on one bad line is correct; never rewrite those
   boards by hand (fleet law) — wait for a repair path (DF-BOARDCTL-9).
   A duplicate-id row makes EVERY read of the board exit 1 until the ids
   are fixed — no flag skips it (by design).
6. **`create` mirrors the LAST row's schema.** On boards with legacy rows your
   new row inherits their key style — that's a feature (byte-stable diffs),
   but check `stats` output for `(none)` status groups after creating on
   odd boards.
7. **Topology B (header on line 1 of tasks.jsonl) is fully writable
   (BT-010).** `create` appends after the header line, `update` rewrites the
   row and bumps the header's `last_commit`, events append normally. The
   old stale-`board.db` write dead-end is gone. Migrating to topology A
   (splitting line 1 into `board.jsonl`) is now an optional manual step,
   not a requirement. Note: `init` still refuses on an existing topology-B
   board (it's for fresh boards only) — and tells you topology B is
   writable when it does.
8. **Prefer `create`/`update` over hand-editing tasks.jsonl.** `update` also
   writes the audit event and keeps `updated_at` coherent; hand edits skip
   the event trail and can mix serialization styles.
9. **`doctor` is your preflight.** It catches tracked `.db`/`.parquet` caches,
   header counter drift, and fixture orphans in ~0.02s on real fleet boards.
   And never `git add -A` a board COPY — an untracked `board.db` rides
   along and doctor (correctly) fails the copy.
10. **After a DUPLICATE-SUPPRESSED refusal, remediate via CREATE, not
    update.** `--evidence-run-id RUN` exists on `create` only — re-running
    the same create with it appends a task_evidence event on the existing
    row (exit 0). `update` rejects the flag (README wording fixed by
    DF-BOARDCTL-11). `--force` files a deliberate fingerprinted variant.
11. **Fleet-wide sweeps are free — run them.** validate across all ~53 live
    fleet boards costs ~624ms total (v0.1.7, 2026-09-22); per-board cost is
    ~1ms/100 rows and process startup dominates. A sweep is the cheapest
    integrity ritual available and it found real debt on day one.
12. **dedupe-board dry-run groups=[] can be the CORRECT answer.** Same id on
    N rows with different titles/reasoning = different findings (fingerprint
    is sha over normalized title+reasoning). Never force it; fix the lane
    that recycled the id (see pitfall 2).

## Report surface — `render`, `serve`, `import`

Added BT-020..022; dogfooded 2026-09-20 and found rough. Use it, but know its
shape before you trust a number.

```bash
boardctl -C R render -o report.html                 # self-contained HTML, no network
boardctl -C R render -o r.html --json r.json        # ALSO emit the payload
boardctl serve --addr 127.0.0.1:8787                # loopback-only uploader
boardctl serve --addr 127.0.0.1:8787 -C ~/myrepo    # -C board prepended to every report
boardctl -C R import r.json --dry-run               # report -> board, plan only
```

Rules that are real (verified):

- `render` is **read-only** and **self-contained** — sha256 of the board files is
  unchanged after render + serve upload, and the HTML has zero external
  `src`/`href` (no CDN, no fonts, no chart libs).
- `serve` refuses any non-loopback `--addr` with **exit 2** (no auth, no TLS — it
  must never leave the machine).
- `import` refuses a **board-name mismatch** (`export is for board "X"; target
  board is "Y"`) and is a no-op when rows are identical. Always `--dry-run` first.
- Fixture rows (`NEVER-DONE`, `fixtures.jsonl` membership, `perpetual:true`) are
  **excluded** from the report's counts — so report `tasks` is one less than
  `wc -l tasks.jsonl` when the perpetual fixture exists. That is correct, not drift.

**Pitfalls that will burn you (measured 2026-09-20, rows DF-BOARDCTL-1..6):**

- **Read the report's charts with suspicion.** `derived.burndown` ships as
  `{window, open}` while `derived.burnup` ships as `{window, days, cum}`; the
  template asks both for `.days`, so the **burndown panel shows "no tasks yet" on
  every report**, even a 42-task board. Check `--json` output, not the picture.
  (DF-BOARDCTL-1)
- **`velocity` can under-report by 2x.** It counts row timestamps only
  (`completed_at`, else `updated_at`); completions recorded only as a
  `task_completed` event are invisible to it. crier: 391 complete rows vs 199 in
  velocity. Never read velocity as "work delivered" without cross-checking
  `derived.complete_count`. (DF-BOARDCTL-4)
- **The data-quality footnote is not per-board in `serve` reports.**
  `completed_no_done_ids` is a payload-level union across every uploaded board,
  so a multi-board report shows another board's ids under one board's view.
  (DF-BOARDCTL-5)
- **`serve` accumulates uploads for the life of the process.** Re-uploading the
  same zip duplicates boards in the report, the compare table and `/api/boards`;
  there is no reset short of restarting. Upload once, or restart between runs.
  The `-C` board also appears twice even before any upload. (DF-BOARDCTL-2)
- **The `-0500` timestamp dialect is written but not read.** `internal/board`
  detects and preserves `2026-09-18T03:47:31-0500`, while the report parser's
  layout list accepts only `Z`/`+00:00` forms — those rows land in
  `parse_warnings.timestamp_excluded` and drop out of every time metric. Check
  `timestamp_excluded` before trusting any time series on a board you did not
  create. (DF-BOARDCTL-3)
- `boardctl <cmd> --help` exits **1** (only the bare `help`/`--help` exits 0).

**Verify a report before quoting it:** render with `--json`, then check that
`derived.complete_count` and the velocity total agree, and that
`parse_warnings.timestamp_excluded` is 0 (or explain it). Two numbers describing
the same board that disagree means one of them is lying.

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

## Performance envelope (measured 2026-09-04, re-verified 2026-09-22 on v0.1.7)

0.01–0.04s for stats/list/validate/doctor on the largest real fleet boards
(259–1163 rows; 43.0ms ± 1.8 validate on the 1163-row hermes-dagger board).
Render of a 53-task board: 13.6ms ± 0.8. Full 53-board fleet sweep: ~624ms.
Boards are tiny; there is no need to batch or cache.
