---
name: boardctl-usage
description: >-
  How to use boardctl — the CLI for coding-hermes JSONL foreman boards
  (tasks/events/board/fixtures under .coding-hermes/board/). Entry points,
  proven commands, error meanings, and pitfalls from a real-use dogfood run.
version: 1.22.0
date: 2026-10-26
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
guard). BT-060 (2026-09-25) closed the half-open item: `update` now accepts
`--title`, and `validate` warns when a title's Pn token disagrees with the
row's priority field (the priority field wins; fix via `update --title` or
`update --priority`). Run-16 (2026-09-27) added pitfalls 13-15 from replaying
the release-cut path: Go VCS stamping's nested-repo trap, version-check's
agreement-not-currency scope, and the sha256 zero-match exit-0 note. Run-17
(2026-09-28) added "Gates are real code" below and pitfalls 16-17 from
tamper-probing the internal checker family (fmtcheck, versioncheck, speccheck,
workflowcheck, vulncheck, freshness) — including the one gap found: CI wiring
of three gates is unverified (DF-BOARDCTL-19). Run-18 (2026-09-29) added the
damaged-board recovery section below (validate --repair / --skip-bad-lines,
the DF-BOARDCTL-9 remedy) and pitfalls 18-19: closed-complete rows whose fixes
sit unmerged on wt/ branches, and the vulncheck version pin's banner-less
blind spot.

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
- `umbrella` (BT-078): read-only cross-repo view — see Proven commands.

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
# umbrella repos (BT-078): READ-ONLY unified view over this repo + every
# child board symlinked under .coding-hermes/links/ (one symlink per child
# repo, NAMED after the repo, aimed at <child>/.coding-hermes). Tasks keep
# repo provenance; identity across the view is <repo>/<id> so colliding ids
# stay distinct. Cross-board depends_on refs resolve only when unambiguous —
# AMBIGUOUS is reported with its candidate repos, never guessed. Broken /
# looping / out-of-repo links are findings (exit 1), never followed.
# NEVER writes through child symlinks: writes stay single-repo via -C.
mkdir -p ~/myproduct/.coding-hermes/links
ln -s ~/services/auth/.coding-hermes    ~/myproduct/.coding-hermes/links/auth
ln -s ~/services/billing/.coding-hermes ~/myproduct/.coding-hermes/links/billing
boardctl -C ~/myproduct umbrella            # census: one line per board + task counts
boardctl -C ~/myproduct umbrella --list     # unified task table (REPO column)
boardctl -C ~/myproduct umbrella --list --status pending,in_progress
                                            # --list only: filter by status —
                                            # comma-separated OR list, aliases
                                            # (done/todo) normalize; unknown =
                                            # exit 2
boardctl -C ~/myproduct umbrella --stats    # per-repo stats + grand totals
boardctl -C ~/myproduct umbrella --validate # per-board validate + dep resolution
boardctl -C ~/myproduct umbrella --json     # machine-readable census
boardctl -C ~/myproduct umbrella --strict   # warnings also fail (exit 1)

# read
boardctl -C R stats [--json] [--all]       # counts by status/priority
boardctl -C R list [--status pending] [--priority P1] [--json] [--all]
boardctl -C R show <ID> [--events] [--skip-bad-lines]  # searches tasks AND
                                                       # fixtures; --skip-bad-lines
                                                       # keeps a parseable row
                                                       # readable on a damaged
                                                       # board (SKIPPED-LINES
                                                       # block on stderr, exit
                                                       # still 1)
boardctl -C R header --json
boardctl -C R validate                     # shape: JSONL parse, header, ids.
                                           # Flags: --repair (salvage every
                                           # parseable row into
                                           # tasks.rewritten.jsonl for manual
                                           # review — tasks.jsonl is NEVER
                                           # modified, exits 0 even when rows
                                           # were dropped), --strict-keys
                                           # (promote BT-056 key-uniformity
                                           # drift from warn to error), and
                                           # --fail-on dangling-dep (promote
                                           # ONE named warning class to exit-1
                                           # error; unknown class = exit 2.
                                           # The pre-commit hook installs this
                                           # so a dangling depends_on blocks
                                           # the commit)
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
boardctl -C R update FEAT-1 --title "renamed: retry with backoff"  # BT-060

# worktree / branch / Hermes session audit trail (first-class row fields).
# Omitted flag = NO key written (main-checkout work has no worktree);
# --session is repeatable and REPLACES the row's sessions array, in order.
boardctl -C R create --id FEAT-2 --title "T" \
    --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session <hermes-session-id> --session <second-session-id>
boardctl -C R update FEAT-2 --worktree /home/me/wt/ft-2 --branch wt/ft-2 \
    --session <hermes-session-id>
# update metadata: --worker-status is separate from task --status;
# --note appends; --blocked-reason records the blocking explanation;
# --completed-at sets the completion timestamp (use the board timestamp format);
# --deferred parks a row as not-yet-actionable: create takes a bare flag,
# update takes true|false (--deferred false REMOVES the key; HEAD-only
# until the next release — see Run-20 lessons below).
boardctl -C R create --id FEAT-3 --title "T" --deferred
boardctl -C R update FEAT-2 --worker-status complete \
    --completed-at "2026-10-10 19:30:00"
boardctl -C R update FEAT-2 --note "Verified on the staging board"
boardctl -C R update FEAT-2 --status blocked \
    --blocked-reason "Waiting for release authorization"
boardctl -C R update FEAT-3 --deferred false          # un-defers: key removed
boardctl -C R event --type audit --tick 42 --detail-text "..."   # or --detail @file;
                                            # --tick N ALSO bumps the header
                                            # ticks_total to max(old,N) (stderr
                                            # notice; legacy topology-B boards:
                                            # line 1 of tasks.jsonl)
boardctl -C R event --type audit --tick 43 --no-tick-write       # event appends,
                                            # header NOT rewritten -> counter
                                            # can go stale and doctor FAILS
                                            # ("ticks_total < max events
                                            # tick_number"); escape hatch for
                                            # boards/tools that own the header
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
boardctl -C R sweep-status --decide        # machine-checkable re-classification
                                           # of off-vocab rows (REVIEW-BOARDCTL-001,
                                           # see README "sweep-status --decide"):
                                           # guard/ci_result values are decided by
                                           # rule (PASS|OK->PASS, FAIL|ERROR->FAIL,
                                           # GREEN, RED, standalone SKIP/N/A->SKIP,
                                           # SUCCESS->GREEN, else stays explicit);
                                           # a "duplicate" WITH an earlier same-id
                                           # twin closes as superseded-by-earliest.
                                           # --decide ALONE never writes; only
                                           # --decide --apply does. PENDING and
                                           # unmatched values stay explicit
                                           # decisions.

# drifted row SHAPES, board-wide (DF-BOARDCTL-36)
boardctl -C R normalize-board              # classify + stage tasks.normalized.jsonl:
                                           # key aliases -> canon (BT-056 table),
                                           # string depends_on -> array when
                                           # unambiguous ("[]"/"" -> [], a JSON
                                           # array string -> the array, one bare
                                           # fleet id -> [id]), alias statuses ->
                                           # canon. AMBIGUOUS rows are REFUSED and
                                           # listed (alias over a present key,
                                           # number/object/malformed depends_on,
                                           # absent/null/unknown status) — decide
                                           # those by hand. Rows without
                                           # created_at are counted, NEVER
                                           # fabricated. DRY RUN by default;
                                           # --apply rewrites tasks.jsonl in ONE
                                           # guarded atomic pass. EXITS 0 in both
                                           # modes (validate owns exit semantics).

# drifted EVENT IDS, events.jsonl (DF-BOARDCTL-38, in v0.1.11)
boardctl -C R normalize-board --repair-events
                                           # renumbers descending event ids to
                                           # ascending (running max + 1), stages
                                           # events.jsonl.renumbered.jsonl in
                                           # dry-run; --apply rewrites the file.
                                           # PROVEN on a 700-event board: 473
                                           # fixed, validate 418 err -> 11, 0
                                           # descend. DF-BOARDCTL-44: the repair
                                           # is now byte-level surgery — ONLY the
                                           # id value token changes; key order,
                                           # separator spacing, double spaces in
                                           # string values and ints >2^53 stay
                                           # verbatim (see README). Still lacks
                                           # duplicate lookahead (DF-BOARDCTL-45).

# pre-commit board lint (run 15, v0.1.8+, BT-054)
boardctl -C R install [--hook-path P] [--timeout S] [--dry-run]
       [--fleet PAT] [--fleet-root DIR] [--lint-mode enforce|warn]
                                           # writes .git/hooks/pre-commit (or
                                           # --hook-path P) with a managed
                                           # `boardctl validate` block.
                                           # --timeout S caps validate at S
                                           # seconds (default 30, min 1): a
                                           # slow/missing/wedged boardctl
                                           # SKIPS (never wedges commits);
                                           # timeout(1) required. --dry-run
                                           # prints the hook block (useful to
                                           # inspect an existing install's
                                           # chaining). Chained installs
                                           # (gitreins + board-lint) honor
                                           # BOTH exit statuses.
                                           # --fleet PAT rolls out the same
                                           # hook to every repo under
                                           # --fleet-root (default $HOME)
                                           # matching PAT (comma-separated
                                           # globs/paths); fleet mode REFUSES
                                           # -C/--hook-path, honors the rest.
                                           # --lint-mode warn makes a validate
                                           # failure print a WARN-ONLY banner
                                           # and ALLOW the commit (default
                                           # enforce blocks); the mode is
                                           # written into every hook, unknown
                                           # values are a usage error.

# report surface (BT-020..022 — details + pitfalls in "Report surface" below)
boardctl -C R render -o report.html        # self-contained HTML report:
                                           # read-only, zero external assets
boardctl -C R import r.json --dry-run      # import a report export into the
                                           # board; --dry-run first, always
boardctl serve --addr 127.0.0.1:8787       # loopback-only report uploader
                                           # (non-loopback --addr refused,
                                           # exit 2)
```

## Write vocabularies (all enforced since BT-007)

- `--status`: `failed, pending, in_progress, review, blocked, complete` —
  anything else rejected with exit 1. (Reads: `dispatched` is tolerated as a
  non-writable scheduler state on live boards — `list`/`show`/`stats` pass it
  through and finding-fingerprint dedupe counts it as an OPEN row, while
  `validate` still reports it off-vocabulary. It is never a legal write.)
- `--guard`: `{PASS,FAIL,SKIP}`; `--ci`: `{GREEN,RED,SKIP}` — junk values
  rejected at write (BT-007 closed the old accept-anything hole; trust
  these fields on boards written by recent binaries).
- `--priority`: `P0..P5` — bare `0`–`5` are normalized to `P0`–`P5`
  automatically; `P9`-style out-of-vocab values are rejected. CREATE-side
  default is lane-dependent (BT-071): paperwork lanes default BELOW the
  operational ones — `-review` → P4, `-docs`/`-readme` → P5, everything
  else (primary, `-qa`, `-pm`, `-sync`) → P2. An explicit `--priority`
  always wins; `update` never invents a default.
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
- `--title` (create and update, BT-060): free text, no vocabulary. On update
  an omitted `--title` leaves the row untouched; a title rewrite appends the
  `task_updated` event. If `validate` warns that a title's Pn token
  disagrees with the row's `priority` field, the priority FIELD wins — fix
  with `update <id> --title ...` (or `--priority` if the field is the stale
  half) instead of hand-editing tasks.jsonl.
- `--acceptance-criteria` (create and update, DF-BOARDCTL-50): the
  dispatchability field — foremen will not dispatch a row without it. Free
  text; multi-line values stay ONE JSON string on one physical row line.
  Omitted = the key is NOT written (no empty/null placeholder, and `create`
  never neutral-mirrors it from the preceding row); on `update` a named flag
  REPLACES the row's criteria. This is the sanctioned path for the field —
  do not hand-edit tasks.jsonl for it.
- `event --type`: must be one of the 18 enumerated event types (DOC-27 —
  the complete vocabulary, alphabetical; the CLI refuses unknown values
  with `not in vocabulary {…}`):
  - Board lifecycle: `board_bootstrap`, `board_init`, `board_migration`
  - Task lifecycle: `task_added`, `task_created`, `task_dispatched`,
    `task_started`, `task_updated`, `task_evidence`, `task_completed`,
    `task_verified`
  - Operational: `audit`, `tick`, `idle`, `dogfood`, `e2e_verified`,
    `spec_created`, `worker_dispatched`

  Legacy free-form `event_type` rows already on a board are not flagged by
  `validate`; only new writes are restricted.
- `event --tick N` / `--no-tick-write`: by default the tick also rewrites
  the header `ticks_total` to `max(old, N)`; `--no-tick-write` skips that
  rewrite, so the counter can go stale and `doctor` fails its drift check
  until `header --set-ticks-total` catches up.

## Gates are real code (run 17)

The repo's quality gates are Go packages under `internal/`, not shell
incantations — run them, don't re-implement them:

```bash
make fmt-check                           # gofmt conformance over cmd/ + internal/
make version-check                       # README release-pin agreement
VERSION_TAG=vX.Y.Z make version-check    # ... plus CURRENCY vs the tag being cut
make spec-check     # docs/muster/openapi.yaml == serve wiring (BT-062)
make vuln-check     # govulncheck exit contract: 0 clean / 3 findings / else TOOL-ERROR
                      # VULNCHECK_STRICT_SCANNER=1 turns banner-less -version into a hard fail
go test ./internal/workflowcheck         # Makefile PLATFORMS == CI platforms, order counts
make build          # stamps main.buildCommit -> freshness gate at CLI runtime
```

Tamper-proven 2026-09-28 (run 17): a misformatted file fails fmtcheck on all
surfaces naming the file; a stale README pin fails currency mode quoting each
stale surface; a renamed operationId or a deleted path block fails speccheck
naming both sides + the regen command; a platform swap fails workflowcheck
with both lists; a broken scan (non-0/3 exit) fails vulncheck as TOOL-ERROR,
never a pass; a stale binary warns on every CLI call until rebuilt (silence
switch: `BOARDCTL_SKIP_FRESHNESS=1`). `go run ./cmd/gendocs` regenerates the
OpenAPI spec deterministically — after any serve/describe change; never
hand-edit the yaml.

## Damaged-board recovery — `--skip-bad-lines` and `validate --repair` (run 18)

One unparseable line makes every default read of a board exit 1 (correct: a
silent partial board is worse than a loud one). `show` joins the tolerant-read
surface (`--skip-bad-lines`, run 21): add the flag to read one row off the
damaged board — its line prints when it parses while the `SKIPPED-LINES`
evidence block rides stderr and the exit stays 1 until the board is repaired.
The recovery loop, proven
end-to-end on synthetic damage replicating all three historical fleet dialects
(helios raw-newline split rows, consensus truncated rows, ring-runner banner
lines):

```bash
boardctl -C R show <ID> --skip-bad-lines   # read one row off a damaged board:
                                           # its line prints when it parses,
                                           # every skipped line a SKIPPED-LINES
                                           # block on stderr (file, 1-based
                                           # line, parse error, snippet);
                                           # exit stays 1 while anything was
                                           # skipped — degraded output must
                                           # not read as success. The damaged
                                           # line itself yields no row (show
                                           # reports task not found for it);
                                           # compose with --events freely
boardctl -C R validate --skip-bad-lines   # census: salvageable rows validated,
                                          # each skipped line an [error] with the
                                          # parse error + 120-rune snippet + line
                                          # number; SKIPPED-LINES block on stderr;
                                          # exit 1 (degraded data must not read as
                                          # success)
boardctl -C R validate --repair           # re-serializes every salvageable row
                                          # into <boarddir>/tasks.rewritten.jsonl,
                                          # proves tasks.jsonl byte-identical,
                                          # prints salvaged/dropped counts with
                                          # dropped line numbers, exit 0
diff R/.coding-hermes/board/tasks.jsonl R/.coding-hermes/board/tasks.rewritten.jsonl
# read the dropped-line numbers; if it looks right, swap BY HAND:
cp R/.coding-hermes/board/tasks.rewritten.jsonl R/.coding-hermes/board/tasks.jsonl
boardctl -C R validate                    # expect RESULT: OK
```

Rules that are real (verified live + on a fresh bunker box with the published
v0.1.9 binary):

- `tasks.jsonl` is NEVER modified by `--repair`; the original board is
  byte-identical before/after (sha256-checked in the run).
- A row SPLIT across lines by a raw newline is dropped, not rejoined — each
  half is its own skipped line. Repair is conservative; rebuild split rows by
  hand from the two snippets.
- Banner lines and truncated rows drop cleanly with their line numbers; the
  evidence block is usually enough to identify the damage cause.
- `--repair` is absent from the README (REVIEW-BOARDCTL-011) — the
  SKIPPED-LINES error names it; this skill section is the full documentation
  until the README lands it.
- Fleet state 2026-09-29: zero unparseable lines on every enabled board — the
  09-22 damage population (helios/consensus/ring-runner) is repaired; treat
  any new damaged board as fresh damage, not legacy debt.

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
   odd boards. Exception: `deferred` and `acceptance_criteria` are NEVER
   neutral-mirrored — when the flag is omitted those keys are absent from the
   new row, whatever the mirrored row carried.
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
    is sha over normalized title+reasoning). Never force it; fix the lane that
    recycled the id (see pitfall 2).
13. **`version`'s tag string is not provenance — check `go version -m` before
    trusting a locally-built or offline-cut binary (run 16, DF-BOARDCTL-16).**
    On hosts where the build path is nested under another git repo (here: a
    `.git` directory at `$HOME`), Go stamps binaries built from linked
    worktrees with the OUTER repo's HEAD + `vcs.modified=true`; sha256 and
    the `version` string still pass. After any offline `make release`, run
    `make install-check RELEASE_TAG=<tag>` — it is the only gate that reads
    build provenance (module version, vcs.modified, revision ancestry).
    Builds from the main checkout stamp correctly; CI stamps clean.
14. **`version-check` enforces README *agreement*, not *currency* (run 16,
    DF-BOARDCTL-17).** The gate has no input for the tag you are cutting, so
    moving the README surfaces before `make version-check` is procedure, not
    enforcement — skipping it leaves every gate green while the install URLs
    keep fetching the previous release. (Drift fails loudly once present.)
15. **`sha256sum -c --ignore-missing` exits 0 when zero files matched (run 16,
    DF-BOARDCTL-18).** "no file was verified" is exit 0, not an error —
    `make install` stays sound only because its exact-asset-name pre-check
    runs before the checksum. If you re-implement the gate, count OK lines.
    (Run-17 note: the zero-match hole is now hardened in the Makefile — the
    gate fails loudly when the checksum step verifies ZERO files.)
16. **speccheck's parity scope is routes/operationIds/servers[].url — not
    prose (run 17).** Editing an operation's `summary` in openapi.yaml passes
    the gate by design (matches its doc comment). Structural drift — a
    renamed operationId, a deleted path block — fails loudly naming both
    sides and the regen command. Summary-level parity would be a new gate,
    not a bug in this one.
17. **CI wiring of fmtcheck/versioncheck/speccheck is unverified (run 17,
    DF-BOARDCTL-19).** vulncheck asserts its ci.yml step (govulncheck pin)
    and workflowcheck parses the workflow — but nothing fails when the
    Gofmt / Version-check / Spec-check steps are deleted from ci.yml (proven:
    `go test ./...` and all make targets stay green with the steps gone).
    Until DF-BOARDCTL-19 closes, "the CI runs it" is a README claim for those
    three gates — check the workflow file when gate coverage matters.
    Related (run 17, DF-BOARDCTL-20): a local `make vuln-check` never checks
    WHICH govulncheck version is on PATH — only CI's install step is pinned.

18. **A row marked `complete` means the fix is claimed, not that main has it
    (run 18, DF-BOARDCTL-22).** The overnight wave closed DF-BOARDCTL-19/20,
    BT-064 and REVIEW-BOARDCTL-009 via a board-only commit while all three
    fixes sat unmerged on `wt/` branches — every original probe still failed
    at the commit main served. Verify a closure at the commit MAIN actually
    serves (`git rev-parse origin/main`), re-running the row's original
    probe; the probes are cheap (a 20-line tamper script, two curls, a stub
    on PATH). Also: a board-only closure commit can carry a misleading
    message ("removed duplicate rows" = 4 status flips) — read the numstat,
    not the subject. And a wave can RE-DISPATCH ids that already read
    complete, double-booking workers on closed rows.
19. **The vulncheck version pin has a banner-less blind spot (run 18,
    DF-BOARDCTL-23).** `CheckVersion` (wt/BT-064) refuses a parseable wrong
    version (`Scanner: govulncheck@v0.0.9` -> gate fails, message names
    expected/actual/fix) but SKIPS a binary whose `-version` output lacks the
    `Scanner:` line — a stub or reimplementation defeats the pin by staying
    silent. Real govulncheck prints the line; treat silent `-version` output
    from any wrapper as unverified. (Applies once BT-064 merges; today even
    the parseable case is unenforced on main.)
20. **Umbrella aggregates trust per-board stats blindly (run 22,
    DF-BOARDCTL-34).** `umbrella --stats` sums `st.Priority` with no
    vocabulary re-check, so a corrupt embedded-JSON priority on any child
    board becomes its own bucket in the per-repo AND grand-total maps
    (`P1","source":"qa-dagger  1`). Read `--stats` buckets as advisory;
    cross-check odd keys against `umbrella --validate` warnings.
    `--list` takes no `--status` either (DF-BOARDCTL-37) — filter with jq
    over `--json` when you need "pending across all boards".
21. **Doctor's tracked-set check guards git history, not disk truth (run 22,
    DF-BOARDCTL-35).** It switches over `git ls-files` output only; an
    UNTRACKED `board.db` regenerated by a local writer sits invisible while
    README says the JSONL are the board. Grep for cache fossils by hand
    (`ls .coding-hermes/board/`) until the untracked-warning lands.
22. **Expect the child board's dialect, not yours (run 22, DF-BOARDCTL-36).**
    A 369-row board speaking its own key/status dialect yields ~600 validate
    warnings under `umbrella --validate` — exit still OK, all warn-level.
    Skim the summary line (`drift N row(s)`), don't read every warning.

## Report surface — `render`, `serve`, `import`

Added BT-020..022; dogfooded 2026-09-20 and found rough. Use it, but know its
shape before you trust a number.

```bash
boardctl -C R render -o report.html                 # self-contained HTML, no network
boardctl -C R render -o r.html --tz America/Bogota  # --tz: report timezone (IANA name);
                                                    # default machine local; pin for
                                                    # reproducible output
boardctl -C R render -o r.html --json r.json        # ALSO emit the payload
boardctl render -C R -o -                           # -o - writes HTML to stdout
boardctl serve --addr 127.0.0.1:8787                # loopback-only uploader
boardctl serve --addr 127.0.0.1:8787 -C ~/myrepo    # -C board prepended to every report
boardctl -C R import r.json --dry-run               # report -> board, plan only
boardctl -C R import r.json --renumber              # same-id-DIFFERENT-content rows are
                                                    # appended under the next free
                                                    # fleet-style id instead of skipped
                                                    # (preview with --dry-run --renumber)
```

BT-075 added a read-only web UI inside `serve` (added 2026-10-02):

- `GET /ui` — one self-contained page listing every loaded board: rows,
  validation findings (pill + verbatim census), analytics, event timeline.
  Everything shown is a SNAPSHOT (loaded at `-C` resolve or upload); file
  edits after load do not show until a re-upload or restart. Zero write
  surfaces — repairs stay CLI (`update --normalize`, `sweep-status --apply`).
- `GET /api/boards` — loaded boards (+ additive `validate_pill`,
  `loaded_at`); `GET /api/board/{slug}` — one board's full read model;
  `GET /api/events/{slug}` — event rows with the detail ladder already
  applied (JSON-string and base64-wrapped details arrive DECODED). Unknown
  slug: JSON 404. The route set is exactly `GET /`, `POST /`,
  `GET /api/boards`, `GET /ui`, `GET /api/board/{slug}`,
  `GET /api/events/{slug}`; docs/muster/openapi.yaml describes all six
  operations (regenerate with `go run ./cmd/gendocs` after serve changes).

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

### Run-20 lessons (2026-10-09, BT-076/077/UI surface)

- `create --deferred` / `update --deferred true|false` are HEAD-only until the next
  release; the README quickstart's Current-release pin (v0.1.10) rejects both flags.
  On any checkout, verify flag availability against `boardctl <cmd> --help`, not the
  README.
- Plural association flags (`--pull-request`, `--assoc-branch`, `--assoc-worktree`)
  merge ONE dimension per invocation; combining dimensions in a single update is a
  usage error. Repeatable: each flag instance appends/merges one element; duplicate
  elements are kept once; singular worktree/branch and other dimensions survive.
- `update --deferred false` REMOVES the `deferred` key entirely (DF-BOARDCTL-29,
  commit 4ab5527); the row is byte-identical to one that was never deferred.
- Hand-edited non-boolean `deferred` values validate as WARN (treated as not
  deferred) — the report parser flags unparseable created_at/timestamps the same
  way; validate does NOT warn on unparseable created_at (gap filed DF-BOARDCTL-27).
- `serve` takes `--addr 127.0.0.1:PORT`, not `--port`. Verify the port you curl is
  actually boardctl's (`ss -ltnp`) — shared dev hosts carry foreign listeners.
- `serve /ui` is the fastest integrity cross-check for a board: upload the folder or
  zip, read the parse-warning notes panel, and compare TASKS/OPEN/COMPLETE against
  an independent `jq` census before trusting any report number.

### Run-21 lessons (2026-10-12, dedupe-board --by-id surface)

- `dedupe-board --by-id --apply` is verified correct end-to-end: closes exactly
  the later duplicate lines (status=complete, `superseded_by`, worker_summary
  naming the kept LINE number), one audit event per closed line, terminal
  later lines untouched, untouched lines byte-identical, idempotent on re-run.
  Apply it to a SCRATCH copy first anyway — validate stays red after the
  repair (DF-BOARDCTL-31), so you cannot use validate to confirm the repair.
- The repair does NOT clear validate's duplicate-id errors — a later duplicate
  carrying `superseded_by` still fails `duplicate task id` (DF-BOARDCTL-31).
  Until that lands, a repaired board still fails an enforce-mode board-lint
  hook; treat validate's duplicate-id errors on a repaired board as resolved
  by checking `superseded_by` yourself (grep the board).
- `recycled ids: N` in the summary counts ids with duplicate LINES, not
  actionable groups. `N` then `nothing to do` is the all-already-terminal
  case (bunker board: 4 recycled, 0 groups) — not a contradiction in the
  engine, but read `--json` groups to see what would actually close
  (DF-BOARDCTL-32).
- `--json` reports per-id `already_terminal_lines` only inside actionable
  groups; ids whose later lines are ALL terminal never appear in `groups` —
  the 4-vs-1 gap on lore (3 recycled ids invisible) is that omission
  (DF-BOARDCTL-32).
- The `--lanes` refusal exits 0 in text mode, 2 in JSON mode (DF-BOARDCTL-33).

### Run-23 lessons (2026-10-16, repair-chain re-proof at HEAD + composition)

- The 10-14 fixes are all live and verified under real use: `--by-id` census now
  says "20 recycled id(s), all later lines already terminal (no files written)"
  after apply (DF-BOARDCTL-32); `--lanes` refusal exits 2 in BOTH modes
  (DF-BOARDCTL-33); `superseded_by`-carrying later duplicates grade as WARN
  naming the closed line, exit 0 — validate can confirm a by-id repair again
  (DF-BOARDCTL-31); umbrella `--list --status a,b` works with aliases and exit 2
  on unknown status (DF-BOARDCTL-37); umbrella --stats excludes off-vocabulary
  embedded-JSON priorities (DF-BOARDCTL-34).
- COMPOSITION recipe for a foreign dialect board (proven on a real 395-row
  task-router board): `dedupe-board --by-id --apply` (closes actionable dup
  lines) → `boardctl normalize-board` dry-run (10 mechanical repairs offered,
  6 REFUSED with reasons — never guess a status) → `normalize-board --apply` →
  `boardctl validate` and READ THE ERROR CLASSES: terminal duplicate-id lines
  stay by scope, "superseded" statuses need a human decision, and legacy
  events.jsonl id-descend errors had NO repair verb at that time (DF-BOARDCTL-38
  — since shipped as `normalize-board --repair-events`, proven in run 25) — a
  red validate after a correct chain is classified, not a failure of the chain.
- normalize-board's default pass touches tasks.jsonl only; event-id drift is
  repaired by the SAME command with `--repair-events` (see the run-25 lessons
  below — DF-BOARDCTL-38 shipped and is in v0.1.11, so the old "event ids are
  out of scope for every verb" line is no longer true).
- RELEASE TRAP — RESOLVED as of v0.1.11 (2026-10-18), keep the habit: the
  README's "Current release" line must name an asset that actually carries the
  verb you are about to drive. Run 25 verified v0.1.11 ships `normalize-board`
  and `--repair-events`; the run-20/23 gap (v0.1.10 rejected them) is closed.
  Still check `boardctl version` against the README pin before driving a
  newly-documented verb — the risk is now cut cadence, not stale docs.
- Exit-code capture: probe exit codes with a single direct command and `$?` —
  capture indirection through leg scripts has read rc=0 off an unknown-command
  failure. (DF-BOARDCTL-40 was closed as premise-false for this reason: v0.1.10
  does exit 2 on an unknown command.) In scripts, check output text, not exit
  code, for the refusal — or pass `--json`.

### Run-25 lessons (2026-10-23, `--repair-events` under real use)

- `--repair-events` WORKS and its staging contract holds: on a real 700-event
  board (473 descending ids) the dry-run wrote only `events.jsonl.renumbered.jsonl`,
  `--apply` landed bytes identical to the staged file, and validate went from
  418 errors (407 descend) to 11 (0 descend) with the residual class untouched.
- **But renumbered lines are re-serialized, so non-id fields move
  (DF-BOARDCTL-44).** Every number on the line becomes a float64
  (`9007199254740993` → `9007199254740992`), keys re-sort alphabetically, and a
  `bytes.ReplaceAll("  ", " ")` style fix collapses double spaces INSIDE string
  values — a recorded `detail` command and a `title` both came out changed.
  If the audit trail's byte fidelity matters to you: keep a copy of
  `events.jsonl` before `--apply`, and diff field-by-field, not by eye. This is
  a live artifact-level defect in v0.1.11 too, not a HEAD-only one.
- Renumbering has no lookahead (DF-BOARDCTL-45): `runningMax+1` can equal an id
  a later line already carries, the report still prints `duplicate ids: 0`
  (it counted the input), and validate only warns. Re-run `validate` after apply
  and read the duplicate warnings, not the census.
- One unparseable line blocks the repair verbs (DF-BOARDCTL-46/47): with a
  duplicate-key row present, `normalize-board` aborts without naming a remedy,
  and `normalize-board --skip-bad-lines` degrades and then refuses to write.
  (The run-25 line "`umbrella` has NO `--skip-bad-lines`" is stale — the flag
  has since shipped, DF-BOARDCTL-47.) `umbrella` has two explicit line-level
  modes: DEFAULT is fail-closed — the FIRST unparseable line in ANY child
  board aborts the whole view with that board's file and line (exit 1, no
  partial output) — while `umbrella --skip-bad-lines` degrades with evidence:
  every parseable row from every healthy board is still counted/listed, each
  unparseable line is reported on stderr as a SKIPPED-LINES block (<repo>,
  file, line, parse error, snippet), and the command still exits 1, so a
  degraded view never reads as clean success. The path that works for
  repairing the damaged row itself is unchanged: `validate --repair` →
  review `tasks.rewritten.jsonl` → swap by hand → re-run normalize-board.
- Timings (warm p50, 700-event board, loaded host): validate 84.6 ms, stats
  55.9 ms, list 56.1 ms, show 72.5 ms, repair-events dry-run 93.0 ms; cold on a
  fresh box 87.5 ms. Fast enough that no perf work is warranted.
- The audit event preserves title+reasoning verbatim; finding text stored in a
  custom row field (e.g. lore's `releng_note`) is preserved on the closed ROW
  but NOT quoted into the event log. If a lane's finding text lives outside
  title/reasoning, capture it before applying (DF-BOARDCTL-31 scope note).
- Install (fresh box, verified): source-only entrypoint — stage the module
  tree (go.mod/go.sum + cmd/ + internal/), `go install ./cmd/dedupe-board`.
  Cold: 21.7s including a Go toolchain download; the binary alone is ~2MB of
  source, stdlib-only.

### Run-25b lessons (2026-10-23, rewrites vs a live board)

- **Rewrite verbs hold the write lock for their whole window (fixed).**
  `normalize-board --apply`, `normalize-board --repair-events --apply` and
  `dedupe-board --by-id` used to read and plan the whole file and take the
  write lock only for the final write, so anything appended in between was
  discarded by the rewrite (DF-BOARDCTL-48/49: 160/160 concurrent creates
  lost, 5/5 appended events lost, both binaries). They now classify INSIDE
  the guard, and `--repair-events` replaces `events.jsonl` atomically
  (temp file + rename) instead of `os.WriteFile`. An append issued while a
  repair runs queues on the guard and lands after the rewrite — it survives.
  Binaries built before this fix (including v0.1.11) still lose it: check
  `boardctl version` before repairing a board a lane is writing.
- **The loss was invisible while it existed.** `validate` and `doctor` return
  rc=0 even on a board a pre-fix rewrite damaged — the vanished row leaves an
  orphan `task_created` event and nothing reconciles it. If you repaired a
  board with a pre-fix binary while a lane was writing, reconcile the event
  trail against the rows afterwards; a green validate proves nothing there.
  Pre-fix fallback: copy the board to a scratch dir, repair there, and swap
  the file while writers are frozen.
- **The window is proportional to board size** (measured: 16 ms at 1k rows,
  84 ms at 10k, 346 ms at 50k). That is why a big board made the pre-fix race
  easy to hit — and why the repair now holds the guard for that whole stretch
  (writers wait; readers never do).
- **You cannot author `acceptance_criteria` with the CLI** (DF-BOARDCTL-50): no
  `--acceptance-criteria` on `create`/`update`, so a dispatchable row is written
  by hand-editing `tasks.jsonl` — the bypass that produces duplicate ids. When
  you do it, edit exactly the target line, keep every other line byte-identical,
  and re-run `validate` + a duplicate-id census afterwards.
- **Documented surface and release parity are clean as of v0.1.11**: 61
  documented commands give identical exit codes/output on the released asset and
  a HEAD build, and `render -o x.html --json x.json` + `import --renumber`
  round-trips as written. Don't re-chase a docs-vs-artifact gap that this pass
  closed — check the release tag first.


## Run-27 (2026-10-29; tick boardctl-dogfood-2026-10-29-14-26-36) — current HEAD real-use and fresh-machine battery

- Build the probe from a clean clone of the target HEAD before trusting CLI output; the installed binary may be stale and emits a freshness warning.
- The real-user path is: `init` → `create --acceptance-criteria` → `update --evidence-run-id` → `show --events` → `event --tick` → `stats --json` → `validate` → `doctor`. On a fresh scratch board, all returned rc=0 and `validate` reported 0 warnings.
- The current fresh-machine battery can install from source and pass the act CI/resource/error legs while still failing upgrade and network-cut cells. Grade each cell independently; an install OK is not an aggregate battery pass.
- `stats --json` was single-digit milliseconds (7.693 ms cold, 7.512 ms warm p50 over 10 runs on three records), below a user-noticeable threshold; no performance finding was warranted.
