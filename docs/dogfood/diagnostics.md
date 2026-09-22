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

## 2026-09-12 re-run (addendum)

Second dogfood run against HEAD `237e594` (post v0.1.2). Full report:
`2026-09-12-integration.md`. What it added to this trail:

- **BT-014's fix is real.** `event --tick N` auto-syncs the header now —
  the "doctor fails on the very next run" trap from tick 16 is closed and
  regression-checked on a real board copy.
- **Byte-stability proven at scale:** one `update` on the 186-row live
  scheduler board produced a 1-line git diff in `tasks.jsonl`.
- **New failure classes found (BT-023..026):** id format is the one write
  still unvalidated (`create --id "bad id!"` exits 0, validate silent);
  `update --commit-hash` hijacks the header's `last_commit` drift pointer
  with the task's fix commit and leaves header `updated_at` stale;
  real foreman rows (`todo`/`done`/`open`) fail validate's status
  vocabulary (23 errors on the live scheduler board — writers enforce a
  vocabulary the fleet doesn't speak); and legacy/partial boards get
  error UX that doesn't match the README's own topology story (a
  `tasks.jsonl`-only dir reads as "no board found", exit 2; a
  pretty-printed board surfaces a raw JSON escape error, no row context).
- **Bunker install re-proven:** fresh agent fcfd3f4b on las-bunker-03
  (bare Debian 13.6, no sudo/no Go) cloned the public repo and ran the
  README quickstart verbatim — checksum-verified v0.1.2 binary in 3 s,
  validate+doctor OK on the cloned board, init→create→update smoke OK.
  One lesson for future dogfooders: never `git add -A` a board copy —
  the untracked `board.db` comes along and doctor (correctly) fails it.
- **Skipped-leg reminder:** `SHA256SUMS` from the README flow only
  verifies files still present; if a release ever drops the checksum
  asset again (the v0.1.1 failure, BT-008), `curl -sL` writes the 404
  JSON page to `SHA256SUMS` and `sha256sum -c` fails with
  "no file was verified" — that failure mode is the canary, not noise.

## 2026-09-20 re-run — the render/serve surface (addendum)

Runs 4 and 12 dogfooded the CLI and board-write surface. This run took the
analytics stack the README advertises above the fold — `render`, `serve`,
`import`, governed by `docs/specs/board-analytics-report.md` — which had never
been exercised by a user.

### How the report is actually assembled (the part that matters)

Three layers, each independently tested, and the seam between two of them is
where this run's findings live:

```
board.Resolve / loadBoard      →  boardData: raw rows + typed task/event recs
  internal/render/load.go          (birth = day(created_at),
                                    done  = day(completed_at), else
                                            day(updated_at) when status=complete)
derive(d)                      →  Derived: burndown/burnup/velocity/cycle/
  internal/render/derive.go        streaks/tick_health/work_clock/model_share
newBoardPayload / Build        →  board-report/v1 payload (JSON island)
  internal/render/report.go
BuildHTML(island)              →  template.go: one big JS renderer that reads
                                   the island and draws every panel
```

The load and derive layers are sound: measured against ground truth they agree
(velocity's window totals, status/priority counts, fixture exclusion). The
losses happen at the **template contract**, because `template.go` is a JavaScript
renderer reading a JSON shape that no Go struct declares as a contract:

- `burndown` is `{window, open}`; `burnup` is `{window, days, cum}`. The
  template asks **both** for `.days` (`template.go:714`). Burn-up draws;
  burndown returns the `"no tasks yet"` placeholder **on every report ever
  generated**, discarding a correct `open` series. `chartLines`
  (`template.go:387-388`) bails on an empty `days` before it ever reads
  `values`.
- `completed_no_done_ids` is payload-level (`report.go:80-94` unions all
  boards), but the footer renders it unscoped — so a multi-board report
  attributes one board's uncomputable rows to every board.

**Lesson to carry forward:** when a Go struct feeds a hand-written JS template,
the struct's JSON tags ARE an API and nothing tests them. Every finding in this
class is invisible to `go test` because derive is correct and the template
"works" — it just draws a placeholder. A report-HTML test (render, parse, assert
the panel is not the empty state) is the missing gate.

### The velocity trap: correct code, missing input

`velocity` (`derive.go:361`) counts completions per ISO week from row
timestamps. It has a fallback for a complete row with no `completed_at` — use
`updated_at` (`load.go:335-338`) — which is why boardctl's own board reconciles
(12 rows with `completed_at` + 4 using `updated_at` = the 16 the payload shows).
That fallback is the trap: it makes the metric look trustworthy on boards whose
rows carry timestamps, and silently halves it on boards whose rows do not.

crier: 391 complete rows → velocity totals 199. 192 of the missing ones carry a
`task_completed` event in `events.jsonl` — the completion is recorded, just not
on the row. So velocity and `complete_count` describe the same board and differ
by 2x, with no visible caveat. The general rule this run produced:

> **When two numbers describe the same board, either they agree or the
> difference must be visible.** A report that quietly reports half the work is
> worse than one that reports none, because half looks plausible.

### The dialect asymmetry

`internal/board/timefmt.go` accepts and **preserves** the colon-less UTC-offset
dialect `2026-09-18T03:47:31-0500` (`zoneRe = (?:Z|[+-]\d{2}:?\d{2})$`) — it
detects it, writes new timestamps in the same shape, and does not normalize it
away. `internal/render/load.go:507` `stampLayouts` has only the colon and `Z`
forms, so the report parser rejects it and drops those rows from every
time-based metric.

So boardctl writes what boardctl cannot read. Verified the writer itself is not
the hazard: `boardctl create` on a `-0500` board emitted `+00:00` (the
DetectTSLayout default), so the asymmetry is read-side only — but a fresh
writer/reader round trip inside one tool that loses rows is a trust defect.

Fleet scan (52 boards, 8034 task rows): the literal `-NNNN` form appears on 4
rows of 1 board (`coding-hermes-tools`). Narrow today; the row was filed with
the blast radius named rather than inflated.

### serve: the state nobody can see

`serve.go:366` appends every upload session to `s.sessions`;
`currentBoards()` (`serve.go:201-207`) concatenates all of them plus the `-C`
board on every request; there is no dedupe and no reset route. Uploading the
same zip three times yields `GET /api/boards` = 2→4 `boardctl` and 1→3 `b2`, and
the compare table diffs a board against itself. The `-C` board also appears
**twice** before any upload at all, which suggests the registration path
double-adds it independently of the accumulation.

### Errors hit during this run

1. **`render --help` exits 1** — every subcommand's `--help` returns 1 while the
   bare `--help` returns 0. Documented as deliberate (a usage error) in
   `docs/dogfood/diagnostics.md`; recorded, not re-filed. Cost: one double-take.
2. **My own probe bug, and it proved a guard works**: `create --id X-2` was
   refused — `task id "X-2" does not match the fleet id format
   ^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$` — because a single-digit segment is not a
   valid id. The id-format gate is live and correctly worded; `X-002` is also
   rejected (segment must be `[A-Z0-9]+` after the hyphen, and `X-002` has no
   qualifying prefix split), so the probe used `PROBE-002`. Good guard, wrong
   probe.
3. **A script path bug on my side** (binary fetched into `$HOME`, smoke script
   looked in `$HOME/dog`) produced a wall of `No such file or directory` and
   `EXIT=0` lines. Lesson worth keeping: `EXIT=$?` after a pipeline reports the
   **pipeline's** status, not the command's — `PIPESTATUS[0]` is the honest
   read, and an `exit 0` next to a "file not found" line is the tell.
4. **`serve` port 8787 was already bound** by an unrelated `python3` process on
   this host, so the first `serve` attempt died with
   `bind: address already in use`. Not a boardctl defect — but it means a failed
   `serve` start is easy to miss, since the process exits and the caller's
   `curl` gets a 404 from whatever else holds the port. Use an explicit
   `--addr` port when dogfooding.

### The right way to dogfood this surface

- Render with `--json` as well as `-o`. The HTML hides the payload; the JSON is
  where a shape defect is visible in one `jq` call.
- Check `burndown` and `burnup` **together** — they are siblings and any
  asymmetry between them is a defect by construction.
- Open the HTML in a real browser and read the DOM, not the file. The burndown
  bug is invisible in `grep` on the HTML (the placeholder string is a JS
  argument) and obvious the moment you query `chartLines`'s inputs live.
- Compare the report's headline numbers against ground truth computed from
  `tasks.jsonl` AND `events.jsonl` independently — never one against the other.
- For `serve`, hash the board files before and after to check the read-only
  claim, and re-upload to expose accumulation.

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

---

## Run-9 addendum — 2026-09-22 (fleet-wide / multi-board dogfood)

**How the multi-board reality actually behaves, and why.**

1. "Legacy pretty-printed boards fail line-wise loads but are VALID" (fleet memory, 08-28)
   is FALSE for 3 of the 4 boards it named. Raw bytes: helios rows contain literal newlines
   INSIDE string values (detail fields pasted from multi-line tool output without escaping),
   consensus has truncated rows that never close (line 167: raw `;`-escape garbage) plus
   whole blank-line separators between row groups, ring-runner has a literal `bunker info`
   banner pasted mid-row (lines 121-128). jq agrees with boardctl on every one. The right
   lesson: a board file is only as valid as its worst escape; count similarity to JSONL is
   not validity. The tool's all-or-nothing parse is the CORRECT default (a silent skip would
   corrupt every downstream count); what's missing is a degrade-with-evidence mode
   (DF-BOARDCTL-9), not tolerance.

2. The fingerprint design survived its first adversarial test: 6 rows sharing one id
   (QA-ASCE-1 x6 on asce) turned out to be 6 DIFFERENT findings (per-row title+reasoning
   compared: all distinct). dedupe-board returning groups=[] there is correct — the
   fingerprint is over text, not ids. The actual defect is upstream lane behavior: QA lanes
   recycled cycle numbers into the id (QA-ASCE-1, -2, -3 refiled each cycle) instead of
   advancing. Filing `QA-ASCE-1` a second time should have been `QA-ASCE-6`. Until lanes
   fix that, validate will keep (correctly) erroring on id-recycled boards.

3. The burndown bug class recurred ONE layer deeper than DF-BOARDCTL-1 fixed: same
   symptom-discipline failure (payload shape vs template expectation never asserted by a
   test), new expression: v0.1.6 read a key that didn't exist (blank panel, honest); v0.1.7
   passes window bounds [start,end] as the x array, so the chart draws 2 of 20 points and
   looks plausible (dishonest). Lesson: shape-mismatch asserts must compare SERIES LENGTHS
   (days.length === values.length), not key existence. A "fixed" chart bug that still
   renders wrong is worse than the original because review stops looking at it.

4. Skill-level reminder honored this run: `${PIPESTATUS[0]}` after pipes (an early
   `stats | tail` printed exit=0 through a failing parse — the exit was tail's). And
   `render -o out.html` is the documented form; the positional-arg refusal is correct
   behavior, not a bug (RTFM before filing).

5. Fleet-sweep cost is negligible (624ms / 53 boards / release v0.1.7); performance is not
   the story on any surface measured this run. Process startup dominates; no PERF rows filed.
