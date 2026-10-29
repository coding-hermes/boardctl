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
- **JSONL invariants (GR-150):** every board file is line-oriented, and the
  one shape that breaks line-orientation is a line carrying MORE than one JSON
  value. Two rules, enforced on both ends:
  - *one JSON object per physical line.* A physical line holds exactly one
    complete JSON value — no second object, no trailing bytes. `ParseRow`
    enforces this (`ErrMultipleJSONValues` / `ErrTrailingData`, with the byte
    offset named); a fused line is an error everywhere it is read (validate,
    list, show, stats, doctor, render) instead of being read as its first
    object with the rest silently dropped.
  - *exactly one final newline.* Appends terminate the previous line first
    (`ensureTrailingNewline`), the append primitives refuse a payload that is
    not one terminated object, and the rewriting writers (update / normalize /
    dedupe / header) re-join lines with `TerminateLines` so a file that arrived
    without its final terminator is written back terminated — never left
    mid-line for the next append to fuse onto.
  - Recovery: the tolerant read (`--skip-bad-lines`) counts a fused line as
    SKIPPED-with-evidence and exits non-zero; `validate --repair` REFUSES a
    source with fused/trailing lines (it compares row ids and event records
    before/after and would otherwise pick one object and drop the rest).
  - Fix path: split the fused line(s) so each object is on its own line
    (`jq -c . <file >split`), then re-validate. Operators: docs/operations.md
    class 5.
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
## Run-15 addendum — 2026-09-24 (verify-the-fixes + never-touched subcommands)

Angle: v0.1.8 shipped three user-facing fixes from runs 8-9 (burndown per-day
labels, read tolerance, BT-048) and 33 commits followed the tag. This run
re-verified the promised workflow with the RELEASED binary on a fresh machine
and exercised the two subcommands no prior run touched (`sweep-status`,
`install`).

What the diagnostics trail should record for the next agent:

1. **The burndown fix can be proven without a browser, and that proof is the
   strongest kind available.** `render --json` gives the payload (`burndown.days`
   present, length-matched to `open`); the page's own `svgtag/svg/linePath/
   chartLines` functions extract cleanly into a node `vm` — feed them a 5-day
   synthetic series and count what `document.createElementNS` produces: two
   `<path>` elements whose `d` attributes carry five points each
   (`M30.00 10.00L135.50 37.60…`). The stub-drawing machine from DF-BOARDCTL-8
   is dead; `derive_test.go:106` is the guard that keeps it dead. The stub
   approach (intercept createElementNS, count tags, read `d` attrs) generalizes
   to any chart assertion in this repo.

2. **`install --dry-run` is a live view of hook composition, not just a
   preview.** On the boardctl repo itself it printed the FULL pre-commit file:
   the GitReins Tier-1 guard subshell, then the board-lint managed block with
   the chaining epilogue that honors both exit statuses (BT-054-R's fix, visible
   in the output). If a hook bug is ever suspected, `--dry-run` shows exactly
   what is on disk without touching it.

3. **`sweep-status` exits 0 in BOTH modes by design — the census is a finding,
   not a failure.** Verified: seeded a `todo` row, dry run reported
   `1/4 off-vocabulary: 1 fixable`, `--apply` canonicalized in place and the
   file stayed byte-stable everywhere else. Scripting rule: check the census
   line, not the exit code.

4. **`--force` on create does NOT override the dangling-dep check** (it only
   bypasses the id-format guard). Verified live: `create --depends-on DG-99
   --force` still refuses. The refusal message even tells you the order to fix
   it ("create the dependency task first").

5. **The checksum install failure mode is real and silent-by-design of sha256:**
   saving the release asset under a different name yields `sha256sum: SHA256SUMS:
   no file was verified` — which reads like corruption. The README explains the
   filename requirement in prose but the error never points there. Filed
   DF-BOARDCTL-12.

6. **Usage-closure meta-test gap:** `install` (BT-054) never entered the
   `--help` command list; the pinned-count bump (15→16 for sweep-status) had no
   chance to catch a 17th command. The fix is to walk the command switch, not
   pin a count (DF-BOARDCTL-14).

Right way to re-run this verification: see
`docs/dogfood/2026-09-24-run15-verify.md` — every step is a one-liner against
the released v0.1.8 asset, plus the node-VM chart proof.

## Run-16 addendum — 2026-09-27 (the release-cut path, from the releaser's seat)

Full report: `docs/dogfood/2026-09-27-run16-release-path-integration.md`.
Short version, as a lesson in how this project's release path behaves:

1. **Go VCS stamping has a host-layout trap that no boardctl gate covers.**
   Builds from any *linked worktree* whose path is nested under another git
   repo (this box: `/home/kara/.git`, a deliberate archive repo) stamp the
   OUTER repo's HEAD + dirty flag into `go version -m`. Mechanism is in the
   toolchain source (`vcsGit.RootNames` matches `.git` directories only, so a
   worktree's `.git` file doesn't stop `FromDir`'s parent walk). Manifested
   mid-day 09-27 when the home repo's HEAD moved; older worktrees started
   stamping the foreign SHA the same day. `sha256` and the `version` string
   pass; only `make install-check` catches it (proven). Filed DF-BOARDCTL-16;
   fix direction = stamp assertion in the `release` target + a docs warning.
2. **`version-check` enforces agreement, not currency.** It has no input for
   the tag being cut, so "move the README, then run the gate" is discipline,
   not enforcement. Filed DF-BOARDCTL-17.
3. **The install gate's soundness is an ordering invariant.** coreutils
   `sha256sum -c --ignore-missing` exits 0 when zero files matched; `make
   install` is safe only because the exact-name pre-check runs first. Filed
   DF-BOARDCTL-18 (calibrates run 15's DF-BOARDCTL-12 — refusal held E2E).
4. **What held:** gate failure UX (precise messages naming the drifted value
   and the fix), the VERSION-is-a-tag refusal on tagged checkouts, stale-dist
   wipe, install-check's honest DEGRADED arm, and the 6-second zero-dep
   install + full smoke on a fresh Debian bunker agent (las-bunker-03,
   ef54aa8a, destroyed).

Environment note for future runners: the mis-stamp applies to EVERY worktree
Go build on this host until the home repo's HEAD is considered — build
identity-sensitive things from the main checkout (stamps correctly) or CI.

## Run-17 addendum — 2026-09-28 (the gate family, attacked with tamper probes)

Full report: `docs/dogfood/2026-09-28-run17-gate-family-integration.md`.
The angle no prior run touched: `internal/{fmtcheck,versioncheck,speccheck,
workflowcheck,vulncheck,freshness}` and `cmd/gendocs` used as a user would —
by trying to sneak real drift past every enforcement surface (scratch clone,
every probe reverted).

1. **How the gates are built, and why the design is sound where it holds.**
   Each gate is a check-only Go package; the Makefile target and the CI step
   both run `go test` on THAT package, so Makefile/CI/go-test share one
   definition of "clean" by construction — there is no second hand-maintained
   rule list to drift. The right way to extend: grow the package, wire the
   Makefile target, and mirror the exact command into ci.yml.
2. **What held, tamper-proven:** fmtcheck (file named in the failure),
   versioncheck currency mode (`VERSION_TAG` — the DF-BOARDCTL-17 fix works),
   speccheck on real contract drift (operationId rename, deleted path block —
   both messages name both sides + `go run ./cmd/gendocs`), workflowcheck
   platform parity (both lists quoted, order noted), vulncheck's exit-code
   contract (non-0/3 exit = TOOL-ERROR, never a silent pass), freshness
   (BT-057: silent at HEAD, precise N-commits-behind warning naming the
   rebuild command and the silence switch), and gendocs determinism
   (regeneration at HEAD = empty diff).
3. **What did not hold (DF-BOARDCTL-19): the CI wiring of
   fmtcheck/versioncheck/speccheck is enforced by nobody.** Deleting those
   three steps from ci.yml leaves `go test -short ./...` AND all make targets
   green — "agreement by construction" covers only the two surfaces that
   execute the same Go code; the workflow file is unverified text. The
   precedent that this is a real gap, not pedantry: vulncheck self-asserts
   its ci.yml wiring and workflowcheck parses the workflow. Fix = step-presence
   census in workflowcheck or per-gate wiring tests.
4. **Minor (DF-BOARDCTL-20):** a local `make vuln-check` never checks the
   resolved govulncheck's VERSION — the v1.7.0 pin is asserted only against
   ci.yml's install step. One `-version` probe would close it.
5. **What a fresh machine proves (bunker-las-03, agent affa0eaf, destroyed):
   the zero-dep install path is real.** README quickstart verbatim: 4 seconds
   from empty box to checksum-verified working binary; stats/validate/doctor
   green on the repo's own 109-task board; create→update→show write probe
   first-try. **Transfer lesson recorded:** a per-file ssh `cat` relay must
   carry `< /dev/null` on the sibling `mkdir` ssh when driven from a
   `while read` loop (the mkdir ssh eats the loop's stdin — the loop dies
   after one file), and `ssh -n` on the relay instead zero-fills EVERY file
   while the counter says ok. Verify transfers with a sha256 census, never
   the loop counter.
6. **Method note for future dogfooders of this repo:** speccheck ignores
   summary/description edits by design — tamper the operationId or delete a
   path block or the gate (correctly) passes your "drift". The canonical
   fingerprint formula for hand-filed rows lives in
   `internal/board/fingerprint.go`: sha256(normalize(title) + \x1f +
   normalize(reasoning)); calibrate your re-implementation against a known
   row before appending.



## Run 18 — 2026-09-29 — the repair path, and what "complete" on the board does not mean

**Angle.** Runs 1–17 never exercised the damaged-board story (`validate
--repair` / `--skip-bad-lines`, the DF-BOARDCTL-9 remedy from daf0f3f). Run 18
drove it end to end, then audited the foreman's overnight closure of run-17's
rows.

**How the repair path actually works (the right way to use it).**

1. Default reads stay all-or-nothing: one unparseable line makes every read of
   the board exit 1. This is correct — a silent partial board is worse than a
   loud one.
2. `validate --skip-bad-lines` degrades with evidence: salvageable rows are
   validated and counted, each skipped line becomes its own `[error]` with the
   parse error and the 120-rune snippet, a SKIPPED-LINES block goes to stderr,
   and the command **exits 1** — degraded data must not read as success.
3. `validate --repair` re-serializes every salvageable row into
   `tasks.rewritten.jsonl` (board style detected from the last line), proves
   `tasks.jsonl` byte-identical before/after, prints salvaged/dropped counts
   with dropped line numbers, and exits 0. The original is NEVER touched.
4. The recovery loop: run repair → `diff tasks.jsonl tasks.rewritten.jsonl` →
   read the dropped-line numbers → if it looks right, swap by hand
   (`cp tasks.rewritten.jsonl tasks.jsonl`) → `validate` → expect RESULT: OK.

**What a new user would hit that the docs don't say.**

- A row SPLIT across lines by a raw newline (the helios dialect) is dropped,
  not rejoined: each half becomes its own skipped line. Repair is conservative
  by design; expect to rebuild split rows by hand.
- A banner line (ring-runner dialect) and a truncated row (consensus dialect)
  both drop cleanly with their line numbers named — the evidence block is good
  enough to reconstruct damage cause from the snippets alone.
- README documents `--skip-bad-lines` but not `--repair` (filed as
  REVIEW-BOARDCTL-011): the SKIPPED-LINES error tells users to run a command
  the README never explains.

**Fleet census (2026-09-29): zero unparseable lines on every enabled board.**
The 09-22 damage population (helios 2347 rows, consensus 1338, ring-runner 33)
is gone — those boards were rewritten clean since. DF-BOARDCTL-9's `complete`
is TRUE at HEAD; synthetic damage proved the path anyway.

**Closure audit — the load-bearing lesson of this run.** c97cd87 flipped
DF-BOARDCTL-19/20, BT-064 and REVIEW-BOARDCTL-009 to complete at 21:43 on
09-28 with a board-only commit (numstat: exactly 4 status flips). The fixes
existed only on `wt/` branches (21:26–21:38) and none were merged. Every
original probe still failed at c97cd87: CI-step deletion undetected, POST
/nonexistent → 400, stub govulncheck green. The fixes themselves WORK — all
three cherry-picked onto c97cd87 in a scratch clone pass their probes (with
one boundary note: CheckVersion skips banner-less `-version` output, filed as
DF-BOARDCTL-23). The same four ids were then re-dispatched by the 02:15 wave
while reading complete. **Verify the fix at the commit main actually serves,
not at the branch that carries it; a status flip is a claim, probes are
evidence.**

**Method notes for future runs on this repo.**

- The tamper probe for 19 is 20 lines of Python (delete `- name:` + `run:`
  line pairs from ci.yml, run the three make targets + the workflowcheck
  test); the 009 probe is two curls against a scratch serve; the 020 probe is
  a stub shell script on PATH. All three run in under a minute — a closure
  claim on these rows costs almost nothing to check.
- hyperfine on a command that CONTRACTUALLY exits 1 (validate on a damaged
  board) needs `-i` — the exit code is the feature, not a failure.
- The multi-tenant /tmp class is real on bunker boxes: `/tmp/smoke.sh` was
  owned by another agent user (`Permission denied`); write scratch scripts to
  `$HOME` on the agent.
- The dogfood log is append-only by convention — a wholesale rewrite of the
  file clobbers 8 prior runs; restore from `git show HEAD:` and re-append.

## Run-20 addendum (2026-10-09) — why the deferred/association features are the right shape, and where the seams are

BT-076 (deferred) and BT-077 (plural associations) both follow the nil-untouched
discipline the board engine already uses for worktree fields: an omitted flag never
creates or changes the key. That is what makes byte-stable diffs possible when
sibling lanes write the same board — a writer that does not know about `deferred`
cannot clobber it. The seams found by real use:

1. The README teaches HEAD behavior while the release pin serves v0.1.10 — the doc
   surface and the artifact surface moved at different speeds. The fix is a release,
   not a doc rewrite (DF-BOARDCTL-26).
2. Two integrity surfaces (validate, report parser) disagree on what counts as
   corrupt: the report parser flags unparseable/empty created_at and excludes those
   rows from time metrics; validate says OK. The repo's own board is the proof —
   two rows written 09-23 with `2026-09-09-22T...` timestamps sat through 16 days
   of green validates (DF-BOARDCTL-27).
3. The zip upload path in the browser now has a full live proof (run 20, CDP file
   input + real submit) — DF-BOARDCTL-6's "unverifiable" premise is closed.

## Run-21 addendum (2026-10-12) — dedupe-board --by-id: the repair works, the validator doesn't know

The recycled-id repair (`dedupe-board --by-id`, DF-BOARDCTL-10) is built on a
clean contract: keep the earliest LINE (the id is the recycled key, so line
number is the only unambiguous pointer), close later lines as superseded with
`superseded_by` + a summary naming the kept line, quote the closed row's
title+reasoning verbatim into an audit event, leave terminal lines alone, and
assert every untouched line round-trips byte-identical before the atomic
rewrite. Real use confirmed every clause of that contract on a scratch copy of
a live board (lore: 4 recycled ids, 1 actionable group) and byte-identically on
a fresh bunker box.

Where real use bit:

1. **The tool's own reason to exist is not yet true.** iddupe.go's header says
   validate "permanently errors (exit 1) on every such board ... Nothing
   repaired the HYGIENE. This file is the missing repair tool." But validate
   still exits 1 on the REPAIRED board — the five duplicate-id errors are
   identical pre- and post-repair. Any repo with an enforce-mode board-lint
   hook is permanently uncommitable even after a correct repair. The missing
   clause is one rule: a later duplicate carrying `superseded_by` is resolved
   (DF-BOARDCTL-31).
2. **The census numbers can contradict the verdict line.** "recycled ids: 4"
   followed by "nothing to do — no recycled task ids on this board" (bunker
   board: 722 rows, all 4 recycled ids' later lines already terminal) is true
   by the code's definitions and still reads as a bug to every human and every
   grep. `recycled_ids` counts ids with duplicate LINES; groups only materialize
   for actionable ones (DF-BOARDCTL-32).
3. **The verbatim-preservation contract has a hole for custom fields.** The
   audit event quotes title+reasoning only. Rows whose finding text lives in a
   project-specific field (lore's `releng_note`) are preserved on the closed
   ROW but not in the EVENT LOG — for a downstream consumer reading only
   events, the finding text is gone. Either quote all long-form text fields or
   say in the docs that preservation covers title+reasoning (DF-BOARDCTL-31
   scope note).
4. **Exit-code parity:** the `--lanes` refusal exits 0 in text mode and 2 in
   JSON mode (DF-BOARDCTL-33) — the 0/1/2 contract the CLI holds elsewhere
   leaks exactly where a guard is refusing to write.


## Run-22 addendum (2026-10-12) — the umbrella surface (BT-078)

Real view = boardctl's own board + warpfs's board via the links registry
(529 tasks). What the run taught about how the thing is built:

1. **Umbrella aggregation is composition, and composition is where
   validation stops.** `Stats()` sums whatever `ComputeStats` produced with
   no re-check of the priority vocabulary (umbrella.go ~:428) — so a corrupt
   embedded-JSON priority that writes would reject sails into a grand-total
   bucket the user reads first (DF-BOARDCTL-34). Rule: every aggregate over
   user-authored JSONL needs the same vocabulary gate the write path has.
2. **Doctor's tracked-set rule guards git history, not disk truth.** The
   check switches on `git ls-files` output (doctor.go:75-82); an untracked
   DuckDB cache regenerated by some local writer sits invisible while README
   says the JSONL are the board (DF-BOARDCTL-35). "Removed" claims need a
   presence check, not just a tracking check.
3. **The fleet's biggest board (warpfs, 369 rows) is a second dialect** —
   597 validate warnings, 127 rows off-canon, empty statuses, string
   depends_on. A cross-repo tool inherits all of it; an onboarding/remediation
   recipe is part of the product, not the user's problem (DF-BOARDCTL-36).
4. **Install leg evidence:** fresh bunker-mvp box, zero-dep path v0.1.10 in
   1s, checksum OK; HEAD b55d305 builds and validate/doctor green. las-02
   agent died mid-leg (network lost, destroy deadline-exceeded; TTL backstop
   armed) — first host-loss of the series, noted for the pitfall file.

## Run-23 addendum (2026-10-16) — repair-chain composition on a foreign dialect board

1. **The composition works, and the residual error set is now CLASSIFIED.** On a scratch
   copy of qa9-frozen-after (real task-router board, 395 rows, 20 recycled ids — the
   harshest live board found), dedupe --by-id --apply + normalize-board --apply ran
   cleanly and validate's 33→31 error census decomposes into exactly three named
   classes: 21 terminal duplicate-id lines (dedupe's scope is honest — it closes
   actionable dups only), 5 "superseded" statuses normalize refuses BY DESIGN
   (fail-closed beats guessed status), and 5 events.jsonl legacy id-descend errors
   NOTHING repairs (DF-BOARDCTL-38). A red validate after a correct repair chain is
   no longer "the tool failed" — each residual error has an owner and a reason.
2. **Exit codes need single-command probes.** The run-23 leg capture of
   `unknown command` on v0.1.10 read rc=0 while the flag-error path read rc=1 —
   capture indirection (heredoc/sh -c leg scripts) makes exit-code evidence ambiguous.
   DF-BOARDCTL-40 asks for the discriminating direct probe + a regression test pinning
   the unknown-command path's place in the 0/1/2 contract.
3. **The release boundary repeats (DF-BOARDCTL-39).** Same class as run 20's
   DF-BOARDCTL-26: README presents verbs the latest release rejects. Six fix-merges
   (BT-078 --status, normalize-board, dedupe census/refusal) have landed since v0.1.10
   without a cut; every dogfood that re-proves HEAD work now widens the docs-vs-artifact
   gap. The durable fix is cadence, not another docs pass.
4. **Install leg evidence:** bunker-mvp agent bctl-df23 (TTL 2h): no preinstalled Go —
   `go build` auto-downloaded go1.26.9; HEAD 4ad7543 from sha256-verified git-archive
   transfer built in 46s; release v0.1.10 checksum-verified and both artifacts probed on
   the SAME agent (normalize-board: HEAD ok / release unknown-command). Destroyed
   cleanly, 90s residue recheck clean. las-03/bunker2/bunker4 offline or inactive;
   bunker-mvp was the only live host — the one-host-is-enough rule held again.
5. **Misseeded prompt config fact:** the stored `boardctl-dogfood` lane prompt names
   warpfs as target (identical body to warpfs-dogfood). Lane name is authority — this
   run targeted boardctl. Recorded for the config owner; not edited by this lane.

## Run-25 addendum (2026-10-23) — the event-id repair repairs the sequence and edits the record

1. **How `--repair-events` is built, and why that matters more than whether it works.**
   `RepairEvents` (`internal/board/repair_events.go:70`) makes one classification pass
   over the parsed lines, then, for every descending id, calls `renumberEventID`
   (`:224`) which does `json.Unmarshal` into `map[string]any`, sets `id`, and
   re-marshals — followed by `bytes.ReplaceAll(out, []byte("  "), []byte(" "))`
   to "restore" single-line style. Consequences, all measured: every number on a
   repaired line becomes a `float64` (so `9007199254740993` prints as
   `9007199254740992`), every key is re-sorted alphabetically, and the
   double-space collapse runs over the *whole* re-encoded line — including
   inside string values, which rewrote a recorded command (`rsync -a  /x  /y`
   → `rsync -a /x /y`) and a title (`fix  double  spaces` → `fix double spaces`).
   The right shape is the one the row pass already uses: stage the raw line and
   replace only the id token. Lesson for future repair verbs in this repo: a
   "mechanical" repair that re-serializes a parsed map is never field-preserving,
   and the promise in the README is byte-level, not semantic.
2. **Renumbering is a left-to-right running max with no lookahead.** `runningMax+1`
   can land on an id that a LATER line already holds; the command then reports
   `duplicate ids: 0` because it counted the input, and `validate` only warns
   about the duplicate it just created. Census output must describe the file the
   command WROTE, not the one it read — the same class of mistake as run 22's
   `--stats` swallowing an alien priority vocabulary (DF-BOARDCTL-34).
3. **One unparseable line poisons three verbs at once.** The live warpfs board
   carries a duplicate `worker_status` key on line 412 (written 2026-10-23 by
   that project's own foreman). `validate` names it (FAIL, 1 error); `normalize-board`
   aborts on it and names no remedy; `normalize-board --skip-bad-lines` reports
   SKIPPED-LINES and then refuses to write ("board read was degraded"), which is
   correct fail-closed behaviour but leaves a user with 410 rows of drift and no
   next step; `umbrella` (no `--skip-bad-lines` at all) loses the whole
   cross-board view, exit 1, after printing only the first healthy board. The
   only working path is the README's damaged-board slogan (`validate --repair`
   → review `tasks.rewritten.jsonl` → swap by hand), which was verified again:
   417 salvaged, line 412 dropped, `tasks.jsonl` untouched.
4. **The verb surface is now fast, and the release boundary is finally even.**
   Warm p50 on the real 700-event board: validate 84.6 ms (fails), stats 55.9 ms,
   list 56.1 ms, show 72.5 ms, repair-events dry-run 93.0 ms; cold on a fresh
   box 87.5 ms. No PERF rows — nothing a user feels. And README's "Current
   release: v0.1.11" now matches a real asset that DOES carry `normalize-board`
   and `--repair-events`, closing the DF-BOARDCTL-26/39 class for the first time
   since run 20. The remaining release risk is cadence, not the docs this time:
   four new rows land on HEAD that v0.1.11 predates.
5. **Install leg.** bunker-mvp agent `boardctl-df25` (TTL 2h, destroyed after;
   residue rechecked): README quickstart verbatim → checksum OK, `boardctl
   version v0.1.11`, 1.18 s end to end, no Go, no sudo. The released binary then
   drove the same drifted boards, reproducing every defect — the findings are
   artifact-level, not checkout-level. `bunker exec` still expands `$HOME` on the
   controller, so stage scripts with `bunker cp` and invoke them by relative name.

## Run-25b addendum (2026-10-23) — the write lock protects appends, not rewrites

1. **Why this addendum exists.** Run 25 (same tick) closed the question "does the
   repair work". It never asked "what if the board is being written while the
   repair runs" — which is the normal state of a fleet board. Full note:
   `docs/dogfood/2026-10-23-run25b-concurrent-rewrite.md`; rows DF-BOARDCTL-48..50.
2. **The read is outside the guard, so the rewrite can only see a stale board.**
   `NormalizeBoard` classifies every line first and acquires the write guard only
   around the final `atomicRewrite` (`internal/board/normalizeboard.go:418-421`);
   `RepairEvents` does the same around a plain `os.WriteFile`
   (`internal/board/repair_events.go:209-214`); `DedupeByID` never takes the guard
   at all (`internal/board/iddupe.go:107-236`). Measured consequence: a row created
   while `normalize-board --apply` runs is present mid-run and gone afterwards
   (160/160 concurrent creates lost across 8 rounds, every create rc=0), and an
   event appended during `--repair-events --apply` is absent from the final file
   in 5/5 attempts on HEAD and 5/5 on the released v0.1.11.
3. **The loss is silent at every surface.** `validate` and `doctor` return rc=0 on
   the damaged board (the vanished row's `task_created` event survives as an
   orphan), so an operator who watched a create succeed has no signal that it was
   reverted. `lock.go:17-19` says the guard exists to prevent exactly this, and
   README:1256-1259 promises the guard covers "its whole check-then-act window":
   for the rewrite verbs the documentation is ahead of the code.
4. **Window size is a function of board size, and it is not small.** hyperfine,
   warm, 5 runs, `--apply` re-armed each run: 1,000 rows 16.0 ms ± 5.8, 10,000
   rows 84.3 ms ± 9.4, 50,000 rows 346.2 ms ± 25.6. That is why the repro used a
   big board — and why "repair is fast" in run 25 is a comfort, not a safety
   argument.
5. **The concurrency test suite covers only the half that works.** All seven tests
   in `internal/board/write_concurrency_test.go` are append-vs-append, same-id
   racing, lock exclusivity/release, or reader isolation; none interleaves a
   rewrite with an append. The fix's regression test has to fail on today's tree
   (checks in the rows' acceptance criteria).
6. **The sanctioned workflow forces the unsanctioned write path.** No `create`/
   `update` flag can set `acceptance_criteria` (README mentions it only as a key
   `normalize-board` repairs), so every lane that needs a dispatchable row edits
   raw JSONL — the bypass README:1262-1264 names as the source of duplicate-id
   corruption. This delivery's own three rows needed that hand edit (DF-BOARDCTL-50).
7. **Verified clean in the same pass.** 61 documented commands driven on both the
   v0.1.11 asset and a HEAD build: identical exit codes and output shapes, no drift;
   `render -o html --json sidecar` → `import --renumber` round-trips as documented;
   umbrella works over real git children; the write vocabularies, id law,
   dangling-`depends_on` and negative-header refusals all fire precisely; read verbs
   stay in the tens of milliseconds. Install leg inherited from run 25 (same HEAD,
   same docs): agent `boardctl-df25`, 1.18 s, v0.1.11 verified, destroyed cleanly.


## Run-27 addendum (2026-10-29) — current HEAD battery and clean CLI path

1. The current target was `7ef46b3` and the real-use probe built from a clean file clone, avoiding the stale installed binary that warned it was nine commits behind. `init`, `create` with acceptance criteria, `update` with an evidence run, `show --events`, `event --tick`, `stats`, `validate`, and `doctor` all worked against a fresh scratch board.
2. The scratch board ended with 2 non-fixture tasks, 4 events, `validate RESULT: OK`, and `key uniformity: 3/3`; `doctor` reported only the expected non-git scratch warning and a fully verified completion share.
3. `stats --json` cost 7.693 ms cold and 7.512 ms warm p50 over 10 runs (4.786–25.891 ms); this is below a user-noticeable threshold, so no PERF row or profile was justified.
4. The fresh-machine battery collected 20 rows on `bunker-mvp` agent `7274a462` and then destroyed it; the follow-up `bunker info` returned `not_found`. The source-build install and act CI pass were OK, resource/error paths passed, while upgrade failed at the documented missing-assets gate and network-cut failed on an uncached Go module download. Those failures are already owned by QA-BOARDCTL-FOREMAN-4 and QA-BOARDCTL-FOREMAN-5; the CLI/API UI-probe INFO is owned by QA-BOARDCTL-FOREMAN-6.
5. No new row was filed because the dogfood lane is read-only and the exact failure classes already have owner rows.
