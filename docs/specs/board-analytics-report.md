# SPEC: board analytics + self-contained HTML report

Status: DESIGN AUTHORITY for BT-020 (render), BT-021 (serve), BT-022 (import)
Date: 2026-09-12
Refs: BT-010 (topology B writable), BT-005 (init), SCHED-GAP-106 (perpetual
fixtures must never pollute board analytics), docs/dogfood/2026-09-04-integration.md

---

## 1. Problem statement, goals, non-goals

Every coding-hermes fleet board lives as plain JSONL on disk, and the only way
to see project progress is to read the files or run `list`/`stats` per board.
Nobody can see progress at a glance, compare boards, or hand a status view to
someone without CLI access. Meanwhile boards get uploaded as folders or zips
to machines with no git access at all.

Goal: boardctl renders one self-contained HTML report from one or more board
folders, with zero infrastructure.

Goals

- G1: Derive analytics (burndown, burn-up, velocity, cycle time, streaks,
  tick health, work clock, model share) purely from `tasks.jsonl` and
  `events.jsonl`.
- G2: The report works on ANY uploaded board folder or zip. No git access, no
  repo assumptions, no backend at view time.
- G3: Three new subcommands (`render`, `serve`, `import`) that match the
  existing CLI contract exactly (Section 6.1).
- G4: Collapse-first UX so a 300-row board is readable; rows expand in place.
- G5: Round trip: a report's data payload can be re-imported into a board
  (BT-022) with a diff preview and a dry-run mode.

Non-goals (hard exclusions — a PR that adds any of these is wrong)

- No network access at render time or view time.
- No CDN, no external fonts, no external images, no chart libraries.
- No backend server as part of `render` output; `serve` is a local convenience
  uploader, not a hosted app.
- No database, no caches, no writes to the board during render/serve.
- No git access of any kind (no `git` subprocess, no `.git` reads).
- No auth, no TLS, no multi-user anything. `serve` binds loopback only.
- No editing of board files from the report UI. `import` is CLI-only.

---

## 2. Data contracts

### 2.1 Input files

A board is located exactly like `board.Resolve` does it today: probe the
target dir, then `.coding-hermes/`, then `.coding-hermes/board/`, and accept
the first candidate containing BOTH `tasks.jsonl` and `events.jsonl`.

| File | Required | Shape |
|---|---|---|
| `tasks.jsonl` | yes | one JSON object per line; on topology B line 1 is the header row |
| `events.jsonl` | yes | one JSON object per line, append-only |
| `board.jsonl` | no | exactly one line (topology A header) |
| `fixtures.jsonl` | no | fixture rows (optional file) |

Topology rule (BT-010 semantics, do not regress):

- Topology A: `board.jsonl` exists; the header is its single line; every
  non-blank line of `tasks.jsonl` is a task row.
- Topology B: no `board.jsonl`; the header is line 1 of `tasks.jsonl`, shaped
  as in `rowIsHeaderShape`: no `id` key plus at least one of
  `project`, `namespace`, `version`, `ticks_total`. The report loader MUST
  skip that line via shape detection (never by "first line always"), and MUST
  NOT assume `board.jsonl` exists.
- A blank line anywhere in any board file is skipped silently (current
  `IterParsed` behavior).

### 2.2 Missing files — the empty board must still render

- `tasks.jsonl` missing entirely → the board did not resolve; that candidate
  is not a board.
- `tasks.jsonl` present but zero task rows (or only a topology-B header) →
  valid empty board. Render the full report skeleton with empty-state
  messages on every chart and "no tasks" in the table (Section 5.7).
- `events.jsonl` present but zero rows → valid; time-based charts render
  empty states; streaks are 0.
- `board.jsonl` missing (topology B) → read the header from line 1 of
  `tasks.jsonl`. If neither exists, the board has no header: render with
  `project` = directory name, `namespace`/`version` = null, and
  `ticks_total`/`ticks_idle` absent from the key-numbers card (not 0 —
  absent).
- `fixtures.jsonl` missing → empty fixture set (current `FixtureIDs`
  behavior).

### 2.3 Task row key union (33 keys, sparse)

The union observed across this repo's 25 live rows — rows carry a SUBSET of
these; every accessor must tolerate absence:

```
id, title, status, priority, active, reasoning, created_at, updated_at,
completed_at, capability_tags, worker_status, worker_summary, commit_hash,
guard_result, ci_result, attempts, complexity, foreman_note, review_notes,
depends_on, blocks, blocked_reason, blocked_since, dispatched_at, exit_code,
files_changed, lines_added, lines_removed, perpetual, primary_model,
primary_provider, fallback_model, fallback_provider
```

Types/normalization (read-side only; the report never writes task files):

| Key | Observed type | Normalization for the report |
|---|---|---|
| `id` | string | required for a row to be a task; rows without `id` are skipped + parse warning |
| `status` | string | `NormalizeStatus`: `"completed"` → `"complete"`; unknown values pass through as their own bucket; empty/missing → bucket `"(none)"` (matches `ComputeStats`) |
| `priority` | string OR JSON number | string form as-is (`P0..P3`), numeric form rendered as decimal string `"1".."3"` (matches `priorityLabel`); missing → excluded from priority counts |
| `created_at`, `updated_at`, `completed_at`, `blocked_since`, `dispatched_at` | timestamp strings | parsed per 2.5 |
| `attempts`, `complexity`, `lines_added`, `lines_removed`, `exit_code` | JSON number | int; missing/null → shown as `—` |
| `guard_result` | string | upper-cased display; PASS / FAIL / SKIP expected, anything else shown verbatim |
| `ci_result` | string | upper-cased display; GREEN / RED / SKIP expected |
| `capability_tags`, `depends_on`, `blocks`, `files_changed` | array of strings | rendered as chips/list; missing → none |
| `perpetual`, `active` | JSON bool | fixture classification per 2.6 |
| `primary_model`, `primary_provider`, `fallback_model`, `fallback_provider` | string | model share per Section 3.8 |

Sparse-key tolerance is absolute: NO metric may require a key that is not in
this union, and no renderer may crash on a missing/`null`/wrong-typed value.
Wrong-typed values (e.g. `attempts` as string) are rendered as the raw JSON
and do not fail the load.

### 2.4 Event row contract

Key union is exactly 7 keys:

```
id, timestamp, event_type, task_id, actor, detail, tick_number
```

- `task_id` may be JSON `null` (e.g. `board_init`). `EventsForTask` matches
  top-level only — the report's "related events for a task" uses the same
  top-level match.
- `detail` tolerant handling — REQUIRED ladder, in order:
  1. `detail` is a JSON object/array → use as-is.
  2. `detail` is a string that parses as JSON object/array → use the parsed
     value (87/92 rows on this board: JSON-encoded strings).
  3. `detail` is a string that strict-decodes as base64 AND the decoded bytes
     parse as JSON object/array → use the parsed value (5/92 rows on this
     board: base64-wrapped JSON, e.g. foreman task_completed payloads).
  4. otherwise → keep the raw string as-is.
  The rendered event shows the decoded form when the ladder succeeded and the
  raw string when it fell through. The ladder must never throw: any step that
  errors moves to the next step.
- `event_type` free text on read (legacy rows are tolerated; only NEW writes
  are restricted by `EventTypeVocabulary` — the report applies no vocabulary
  filter, it bucketizes per Section 3.7).
- `tick_number` may be 0 or null.

### 2.5 Timestamps — dual layouts, parse rule, fallback

Measured reality on this board (91/92 + 1): event timestamps are
`"2006-01-02 15:04:05"` space layout; the single exception is RFC3339
`"2026-09-05T03:58:00.000000+00:00"`. Task rows carry
`"2026-09-12 05:38:03"`-style `created_at`/`updated_at`/`completed_at`.
`timefmt.go` documents five dialects the fleet writes; the report parser must
accept AT MINIMUM:

1. `2006-01-02 15:04:05` (space-naive) — and the fractional variants
   `2006-01-02 15:04:05.000000` / `.999999`.
2. RFC3339-ish `2006-01-02T15:04:05` with optional fraction and offset
   (`+00:00`, `Z`).

Parse rule, in order:

- Space layout → interpreted in the REPORT TIMEZONE (2.6). Fractional part
  optional, sub-second precision truncated (not rounded) for day bucketing.
- T layout with offset → converted to the report timezone.
- T layout naive → treated as report-timezone wall clock.
- Unknown/unparseable/missing → the value is excluded from every time-based
  metric and counted once per row in that board's `parse_warnings` (the
  warning names the task/event id and the offending field). The row itself
  still renders in the task table with the raw string shown.

Report timezone:

- Default: the rendering machine's local zone (Go `time.Local`).
- Override: `boardctl render --tz <IANA name>` (e.g. `--tz America/Bogota`)
  for deterministic output; tests MUST use `--tz` to be reproducible.
- All day bucketing for a board happens in ONE report timezone — never mixed.

### 2.6 Fixture classification and exclusion

A row is a FIXTURE if ANY of:

- its `id` appears in `fixtures.jsonl` (membership by exact id), OR
- its `id` starts with `NEVER-DONE`, OR
- its `perpetual` field is JSON true.

(Note: on this board the `fixtures.jsonl` row carries `active:true` and no
`perpetual` key, while the same task's `tasks.jsonl` copy carries
`perpetual:true` — hence the three-signal union, not a single field.)

Consequences:

- Fixtures are EXCLUDED from: burndown, burn-up, weekly velocity, cycle time,
  delivery streak, activity streak, model share, and the default
  status/priority counts (same default as `stats`, which hides fixture rows
  unless `--all`).
- Fixtures STAY VISIBLE in the task table behind a toggle (default: hidden),
  rendered with a distinct chip (`fixture`).
- Raw any-tick streak (3.6c) counts ALL events and is therefore unaffected by
  fixture exclusion (fixtures do not emit events on this board).

### 2.7 Torn-line tolerance and parse warnings

The report uses a NEW tolerant reader — deliberately NOT `IterParsed` (which
aborts the file on the first bad line). Report loader semantics:

- A line that fails `json.Decode` (torn write, truncated tail, garbage) is
  SKIPPED with a warning. A torn/incomplete trailing line must never fail the
  load.
- Blank lines are skipped silently (unchanged).
- Duplicate task `id`s: the LAST row in file order wins (fleet board-scan
  doctrine: re-filed ids — last is live); each superseded duplicate adds a
  parse warning.
- Duplicate event `id`s are NOT deduped (events are append-log facts; both
  render).
- Warnings are counted per file and carried in the payload as
  `parse_warnings: {tasks: N, events: N, fixtures: N, timestamp_excluded: N,
  duplicate_ids: N, notes: [first 20 messages, truncated to 200 chars each]}`.
- The report UI surfaces the counts in a dismissible banner: "Loaded board
  `name` with N parse warnings" (Section 5.7). Zero warnings → no banner.

---

## 3. Derivation spec — every metric, exact formula

### 3.0 View catalog — the user question each view answers

Every chart, table and heatmap in the report exists to answer exactly one
question a board owner actually asks. This catalog is normative: an
implementation that renders a view whose question cannot be named here does
not belong in the report. "Source" points at the formula subsection; "UI
layer" points at the Section 5 surface that draws it.

| # | View | The user question it answers | Source | UI layer |
|---|---|---|---|---|
| V1 | Burndown (open tasks per day) | "Is the backlog actually shrinking, or are we opening work as fast as we close it?" | 3.1 | project page |
| V2 | Burn-up (cumulative completions per day) | "How much real work has this board delivered over time, and is the slope still alive?" | 3.2 | project page, overview sparkline |
| V3 | Weekly velocity bars | "Are we completing more or fewer tasks per week than last month?" | 3.3 | project page |
| V4 | Cycle time per task (median, p90, worst-5 table) | "How long does a task take from filing to done, and which tasks are the outliers dragging the tail?" | 3.4 | project page + worst-5 table |
| V5 | Delivery streak (current / longest) | "Have we shipped something real every day this week — is the board genuinely moving?" | 3.6, 4.1 | overview card + project page (headline) |
| V6 | Activity streak (current / longest) | "Is the project being worked on at all, even on days without a completion?" | 3.6, 4.1 | project page (secondary) |
| V7 | Raw any-tick streak | "How long has the scheduler been firing?" — context only, never a headline (4.1/4.2) | 3.6 | project page, muted footnote style |
| V8 | Tick-health timeline (completed / failed / audit per day) | "When did things happen, and were the ticks healthy or throwing audits and failures?" | 3.5 | project page |
| V9 | Work clock (weekday x hour heatmap) | "When does this fleet actually work — which hours/days carry the load?" | 3.7 | project page |
| V10 | Model share across completed tasks | "Which worker models are doing the delivery, and is one model carrying everything?" | 3.8 | project page |
| V11 | Status / priority counts | "What is the board's composition right now — how much is pending, and at which priority?" | 3.9 | overview card + project page |
| V12 | 26-week calendar heatmap | "What does the last half-year of activity look like at a glance — are there dead weeks?" | 3.6 day-count input, 3.5 | overview + project page |
| V13 | Multi-board compare (overlaid burn-up, velocity race, side-by-side key numbers) | "Which of these boards is healthiest — who is delivering fastest, who is stuck and idle?" | 3.10 | compare view (5.6) |
| V14 | Collapsible task table | "What is the actual state and history of this one task — who touched it, what did the guard and CI say?" | 5.3 (renders 2.3/2.4 fields) | project page |

Rule: the report footer must not claim a metric that has no catalog row here.
Adding a view later means adding its row (question + source + layer) in the
same change.

Conventions used by every formula below:

- Day key: `YYYY-MM-DD` string in the report timezone (2.5).
- `days(d1, d2)` = calendar-day difference of day keys.
- A task's `birth` = day(created_at); if created_at missing/unparseable, the
  task is excluded from time-based metrics (counted in
  `parse_warnings.timestamp_excluded`) but still renders in the table.
- A task's `done` = day(completed_at); fallback: if completed_at missing and
  status is `complete`, use day(updated_at); if that is also missing the task
  never counts as completed in time series (data-quality note in the report
  footer, with the id list).
- All task-scoped metrics operate on the non-fixture task set (2.6) unless
  stated otherwise.
- Window: every daily series spans `[min(relevant day), max(relevant day)]`
  with NO gap filling beyond that range, and every series carries its window
  explicitly in the payload. Empty input → empty series, not an error.

### 3.1 Burndown (open tasks per day)

For each day d in window `[min(birth) .. max(today_or_last_activity)]`:

```
open(d) = |{ t : birth(t) <= d  AND  (done(t) absent OR done(t) > d) }|
```

- `today_or_last_activity` = max(today, last event day, last done day). If
  the board is empty → empty series.
- Tasks created before the window: there IS no "before the window" — the
  window starts at the earliest birth, so nothing is clipped. (If a
  future `--from` filter lands, tasks born before `--from` count as open on
  the first windowed day.)
- Tasks with no completed_at and status != complete are open forever — that
  is correct behavior, not an edge case.
- Edge: single-day board → series of length 1.
- Edge: a task whose `done` < `birth` (clock skew) is treated as open until
  `done` normally (the dip is honest) and adds a parse warning note.

### 3.2 Burn-up (cumulative completions per day)

```
cum(d) = |{ t : done(t) exists AND done(t) <= d }|
```

Window `[min(birth) .. max(today_or_last_activity)]` — same window as
burndown so the two charts share an x-axis and can be overlaid.
Completions counted once per task id (duplicates already collapsed by 2.7).

### 3.3 Weekly velocity

- Bucket: ISO week, Monday-start. Key `YYYY-Www` (ISO week number).
- Value: `|{ t : done(t) in week }|` — completions per week, non-fixture.
- Display window: the most recent 12 weeks that contain at least one
  completion OR at least one birth; all computed weeks stay in the payload.
- Zero-completion weeks inside the display window render as 0-height bars
  (never skipped — a skipped week lies).
- Edge: board younger than one week → single bar.

### 3.4 Cycle time per task

```
cycle(t) = done(t) - birth(t)          # whole days, floor
```

- Population: non-fixture tasks with a computable `done` AND a computable
  `birth`. Tasks missing either timestamp are excluded and the exclusion
  count renders next to the metric ("cycle time over N of M completed").
- Reported: median (even count → mean of the two middle values),
  p90 (nearest-rank: `ceil(0.9 * n)`-th of the ascending sort), and a
  worst-5 table (task id, title truncated to 60 chars, cycle days, status).
- Edge: n = 0 → "no completed tasks"; n = 1 → median = p90 = that value.
- Edge: cycle = 0 days (same-day completion) is valid and included; the
  median renders as "0d".

### 3.5 Tick-health timeline

Per day d, over ALL events (fixtures do not emit events):

```
completed(d) = |{ e : event_type == "task_completed" }|
failed(d)    = |{ e : event_type contains "fail" (case-insensitive)
                     OR (event_type == "task_completed"
                         AND detail (per 2.4 ladder).outcome == "failed") }|
audit(d)     = |{ e : event_type == "audit" }|
```

- Stacked daily bars or a 3-line series; the spec mandates the counts, not
  the chart type.
- On this board today `failed` = 0 everywhere (no failure events exist); the
  definition still stands for boards that carry them.
- Days with zero events of all three classes are omitted from the series
  (gaps render as gaps, not zeros — this chart answers "when did things
  happen", unlike velocity where zero-weeks must render).

### 3.6 Streaks — see Section 4 for semantics; formulas:

- (a) Delivery streak — day set `D = { done(t) : t non-fixture, done exists
  }`; longest run and current run of consecutive calendar days in D.
- (b) Activity streak — day set `A = { day(e.timestamp) : e.event_type !=
  "idle" }`; same run computation.
- (c) Raw any-tick streak (context only, never headline) — day set = ALL
  event days including `idle`; displayed in small muted text next to the
  activity streak, never in the overview card's headline numbers.

Run computation (shared): sort the day set ascending; walk once; a run breaks
when `days(prev, next) > 1`. Longest = max run. Current = the run containing
the last element, evaluated per the today-boundary rule in Section 4.3.

### 3.7 Work clock (weekday x hour heatmap)

- Grid: 7 rows (Mon..Sun) x 24 columns (hour 0..23) in the report timezone.
- Cell value: count of ALL events (no exclusion — fixtures emit none) whose
  timestamp falls in that weekday/hour cell.
- Color scale: 0 = background, then a fixed 4-step ramp (1, 2-3, 4-7, 8+).
  The exact buckets are part of the payload so the legend always matches.
- Purpose: shows when the fleet actually ticks. Days-of-week are labeled; the
  payload carries the raw 7x24 matrix.

### 3.8 Model share across completed tasks

- Population: non-fixture tasks with status `complete` (post-alias).
- Value: count by `primary_model`; missing/empty → bucket `"unknown"`.
  `primary_provider` is ignored for grouping (models repeat across providers)
  but rendered in the table tooltip.
- Render: horizontal bars with counts + percentages; percentages of the
  completed population.

### 3.9 Status and priority counts

- Exactly the `ComputeStats` semantics: status buckets post-`NormalizeStatus`
  with `"(none)"` for empty; priority buckets as display strings
  (string form and numeric form are DISTINCT buckets — live boards mix
  `"P1"` and `1`); fixture rows excluded unless the fixtures toggle is on
  (the counts re-render live when the toggle flips).

### 3.10 Multi-board compare

The report renders one section per board plus a compare view when ≥2 boards
are present:

- Overlaid burn-up: one cumulative-completions line per board, x-axis
  normalized to days-since-that-board's-first-birth (NOT calendar dates —
  boards start on different days), y-axis = cumulative completions.
- Velocity race: weekly completions per board as adjacent bars for the first
  12 weeks of each board's life, aligned by week-index.
- Side-by-side key numbers table, one row per board: project name,
  `ticks_total`, `ticks_idle`, idle% (`ticks_idle/ticks_total`, "—" when the
  header is absent), task total, open, complete, completion%, current
  delivery streak, median cycle time, last activity day.
- All figures come from each board's own derivations (3.1–3.8); no cross-
  board computation beyond presentation.

---

## 4. Streak semantics (normative)

### 4.1 Definitions

- **DeliveryStreak** = consecutive days with >= 1 real (non-fixture) task
  completion, where a completion is a task's `done` day (3.6a). This is the
  HEADLINE metric. It is shown in the overview card and the project header.
- **ActivityStreak** = consecutive days with >= 1 non-noise event, where
  noise is `event_type == "idle"` (3.6b). Secondary metric, shown on the
  project page only.
- **Raw any-tick streak** = consecutive days with ANY event including idle
  (3.6c). Display-only context in muted small text. NEVER a headline, never
  in compare key numbers.

### 4.2 Why each is or is not gameable (must be stated in the report footer)

- DeliveryStreak is the least-gameable of the three: it requires a real task
  row to move to complete with a real `completed_at`. Remaining attack: a
  foreman could re-open and re-complete an old task to keep the streak alive
  (one completion re-dated today). Mitigations already in this spec: a task's
  completion day is counted ONCE (duplicate ids collapse, 2.7), fixtures are
  excluded (SCHED-GAP-106), and the worst-5 cycle table (3.4) makes
  suspiciously short re-completions visible. Residual risk accepted.
- ActivityStreak is gameable by emitting one cheap non-idle event per day
  (an `audit` with empty detail is enough). That is exactly why it is not
  the headline.
- Raw any-tick streak is trivially gameable (idle ticks count by
  definition) — hence context-only. It exists to explain "the scheduler ran
  but nothing happened" versus "the scheduler did not run".

### 4.3 Today-boundary rule (exact)

Let `lastActive` = the last day in the metric's day set, `today` = today in
the report timezone, `gap = days(lastActive, today)`.

- `gap <= 1` → the current streak is LIVE and equals the run ending at
  `lastActive`. An incomplete today neither breaks nor inflates the streak:
  today contributes nothing until it has an event/completion, and its absence
  costs nothing until tomorrow.
- `gap >= 2` → the current streak ENDED at `lastActive`; the UI renders
  `ended YYYY-MM-DD (N days ago)` in muted text next to the number.
- Longest streak is independent of today (pure historical max).
- The report is a static snapshot: `today` is the render-time date, and the
  payload stamps `rendered_at` + `report_timezone` so a stale report's
  streaks can be reinterpreted correctly.

---

## 5. UX spec

Three layers, collapse-first. Single-page app inside the one HTML file; all
rendering is client-side from the JSON island (Section 6.2).

### 5.1 Layer 1 — overview card grid (default view)

One card per board, 2-4 per row (responsive grid):

- Board name (= `project`, fallback directory name) + topology badge
  (`A`/`B`) + namespace.
- Mini sparkline: burn-up series (3.2), pure SVG polyline, ~120x28px, no axes.
- Key numbers on the card (exactly these): total tasks, open, completion%,
  current DeliveryStreak, `ticks_total`/`ticks_idle` (absent-header boards
  omit the tick pair rather than showing 0).
- Parse-warning badge when warnings exist (click → the banner, 5.7).
- Click card → Layer 2 for that board.

With exactly one board the grid still renders (one card) and the compare
section is omitted entirely.

### 5.2 Layer 2 — project page (per board)

Order top-to-bottom:

1. Key numbers strip: the same numbers as the card plus median/p90 cycle
   time and last activity day.
2. Charts: burndown + burn-up (overlay toggle, default separate), weekly
   velocity bars, tick-health timeline, work clock heatmap, model share
   bars. All hand-rolled SVG (Section 6.3).
3. Task table (Layer 3) below the charts, with filters.

### 5.3 Layer 3 — task rows, COLLAPSED by default

Collapsed row shows EXACTLY: `id`, `title` (single line, ellipsis, 100-char
cap consistent with `list`), status chip (color per status; `complete` green,
`pending` neutral, `failed` red, `blocked` orange, `in_progress` blue,
`review` purple, unknown = gray), priority, primary model (or `—`), commit
short-hash (or `—`), and the `fixture` chip when 2.6 classifies it so.

Click (or Enter/Space) expands IN PLACE (same row, no navigation) to full
detail:

- `title` full, `reasoning`, `worker_summary`, `foreman_note`,
  `review_notes` (each omitted entirely when absent — no empty headers).
- guard/ci verdicts as chips (PASS/FAIL/SKIP, GREEN/RED/SKIP), attempts,
  exit_code, complexity, lines added/removed, files_changed list.
- Timestamps: created_at, dispatched_at, completed_at, updated_at,
  blocked_reason/blocked_since when present.
- depends_on / blocks as links to those task ids (link scrolls to the row
  and expands it; unknown ids render as plain text).
- Last 8 related events from `events.jsonl` (top-level `task_id` match, most
  recent 8 by file order), each showing timestamp, event_type, actor, and
  detail per the 2.4 ladder (decoded pretty JSON when the ladder succeeded,
  raw string otherwise), with a "show all N" affordance.
- Full pretty JSON of the raw task row in a `<pre>` (the verbatim field
  bytes, matching `boardctl show` output style).

### 5.4 Expand-all / collapse-all, addressability, keyboard

- Toolbar buttons: Expand all / Collapse all (scope: current board's table).
- Every row is addressable: URL hash `#b=<board-slug>&t=<task-id>` expands
  that row on load and scrolls to it. Board slug = lowercased project name,
  non-alphanumerics → `-`. Changing hash without reload must react (hashchange
  listener).
- Keyboard: Tab reaches each collapsed row (rows are `<button>`-like:
  `role="button"`, `aria-expanded`), Enter and Space toggle, Escape collapses
  the focused expanded row. Expand/Collapse-all are real buttons.
- Expanded state is visual + `aria-expanded` — never color-only.

### 5.5 Filters

- Free-text: case-insensitive substring over `id`, `title`, `worker_summary`,
  `commit_hash`. Debounced 150ms.
- Status: dropdown of the board's actual buckets (post-alias, including
  `"(none)"` and any unknown pass-through statuses) + "all".
- Fixtures toggle: default OFF (hidden). ON shows fixture rows, each with the
  distinct `fixture` chip; status/priority counts re-render to include them
  (matching `stats --all` semantics).
- Filter state is reflected into the URL hash (`q=`, `s=`) so a filtered view
  is shareable.
- Filters compose (AND). A filter that matches 0 rows renders the table
  empty-state (5.7), not a blank region.

### 5.6 Multi-board compare view

Reached from the overview when >=2 boards: "Compare" tab renders Section
3.10. Boards are selectable (checkbox chips, default all).

### 5.7 Empty, error, and warning states (all mandatory)

- Empty board (0 tasks): every chart shows its centered empty-state line
  ("no tasks yet", "no completions yet", "no events"); key numbers render 0
  honestly except streaks which render `0 (no activity)`.
- Board with tasks but 0 events: time charts empty-state; DeliveryStreak can
  still be nonzero (task rows carry done days); ActivityStreak 0.
- Unparseable board (a required file is not JSONL at all): the board card
  renders in an error state with the first error message and the board is
  excluded from compare; other boards are unaffected.
- Parse warnings banner: dismissible, lists counts (2.7) and the first 20
  notes collapsed behind "show notes".
- Filtered-to-zero, unknown hash target (bad board slug or task id): inline
  "no match" messages. The page NEVER throws on bad input — every render
  path is defensive (this is a hard requirement; a board is untrusted data).

---

## 6. Constraints

### 6.1 CLI contract (must match the existing house style exactly)

Existing style (verified against `cmd/boardctl/main.go`): global `-C` may
appear before OR after the subcommand (`-C` and `-C=` forms both pre-scanned;
each subcommand also registers `-C`), flags parsed with `flag.FlagSet` +
`reorderArgs` so flags may follow positionals, exit codes `0` ok / `1`
error / `2` usage-or-board-not-found (`ErrBoardNotFound` → 2). New
subcommands slot into the same dispatch and the same usage text block:

```
render   [-C dir] [-o out.html] [--tz Zone] [--json out.json]
serve    [-C dir] [--addr 127.0.0.1:8787]
import   <export.json> [--dry-run] [-C dir]
```

- `render` reads the board, writes the self-contained HTML to `-o`
  (default `board-report.html` in the working directory; `-o -` writes to
  stdout). `--tz` pins the report timezone (2.5). `--json` additionally
  writes the raw payload (6.2) as JSON — this file is the BT-022 export
  format. Exit `2` when `-C` resolves to no board, `1` on write failure,
  `0` otherwise. Render NEVER writes board files.
- `serve` starts an HTTP server bound EXACTLY to the requested address,
  default `127.0.0.1:8787`. Startup refuses (exit `2`, message) any
  `--addr` host that is not `127.0.0.1`, `localhost`, or `::1`. It serves
  the uploader page at `/`, accepts an upload (BT-021), renders, and returns
  the report. Clean shutdown on SIGINT/SIGINT-equivalent exits `0`.
- `import` reads an export.json (6.2) and applies it to the `-C` board with
  a printed diff plan; `--dry-run` prints the identical plan and writes
  nothing (exit `0`). See BT-022 acceptance criteria (8.3) for semantics.

No other flags are authorized by this spec. Anything else needs a spec
amendment first.

### 6.2 Payload shape (the JSON island and the export format)

One JSON object, schema name `board-report/v1` (the payload carries
`"schema": "board-report/v1"`; import refuses other schema values with a
clear error, exit `1`):

```
{
  "schema": "board-report/v1",
  "rendered_at": "<RFC3339>",
  "report_timezone": "<IANA name or Local>",
  "boards": [
    {
      "name": "<project or dirname>",
      "slug": "<url-safe>",
      "topology": "A" | "B",
      "header": { ...raw header row, or null when absent... },
      "tasks":   [ ...raw task rows, verbatim field bytes re-serialized... ],
      "events":  [ ...raw event rows... ],
      "fixture_ids": [ ...ids from fixtures.jsonl... ],
      "parse_warnings": { ...per 2.7... },
      "derived": {
        "burndown":   { "window": [d1, d2], "open": [int...] },
        "burnup":     { "window": [d1, d2], "days": ["YYYY-MM-DD"...],
                        "cum": [int...] },
        "velocity":   { "weeks": ["YYYY-Www"...], "counts": [int...] },
        "cycle":      { "n": int, "excluded": int, "median_days": num,
                        "p90_days": num, "worst5": [ {id,title,days,status} ] },
        "streaks":    { "delivery": {"current": int, "current_state":
                        "live"|"ended", "current_since": "YYYY-MM-DD"|"",
                        "longest": int},
                        "activity": {...same shape...},
                        "any_tick": {"current": int, "longest": int} },
        "tick_health":{ "days": [...], "completed": [...], "failed": [...],
                        "audit": [...] },
        "work_clock": { "grid": [7][24] int, "max": int },
        "model_share":{ "models": [...], "counts": [...], "completed_n": int },
        "status_counts": {...}, "priority_counts": {...}
      }
    }
  ]
}
```

Derived values are computed in GO, not in the browser — the payload is the
single source of truth for both the HTML view and `import` round-trips.
Raw rows are embedded so the task-table expander (5.3) works without any
recomputation.

### 6.3 Single-file HTML constraints

- ONE self-contained `.html` file. All CSS in one `<style>`, all JS in one
  `<script>`, both inline. Zero CDN, zero external fonts, zero images
  (icons are inline SVG paths), works from `file://`.
- No chart libraries. All charts are hand-rolled SVG generated by the inline
  JS (sparkline, line/overlay, bars, stacked bars, heatmap, h-bars). No
  canvas, no d3.
- **JSON island + exact escaping rule:** the payload (6.2) is embedded as

  `<script type="application/json" id="report-data">…</script>`

  serialized with `encoding/json` and then byte-escaped:
  `<` → `\u003c`, `>` → `\u003e`, `&` → `\u0026`, U+2028 → `\u2028`,
  U+2029 → `\u2029`. These are lossless JSON escapes, so the parsed payload
  is byte-identical after `JSON.parse`, and the sequence `</script>` (any
  case, any attribute form) cannot occur inside the island — a task title
  containing `</script>` cannot break the page. This is Go's
  `json.HTMLEscape` behavior; BT-020 must use it (or byte-equivalent) and a
  regression test must prove a `<img src=x onerror=…>`/`</script>` title
  round-trips inertly.
- Second half of the XSS rule: all dynamic DOM insertion uses
  `textContent` / element creation — `innerHTML` is FORBIDDEN anywhere in
  the inline JS (one exception: static template literals containing no
  payload-derived data). Payload data reaching attributes goes through
  `setAttribute`.
- The inline JS must run with no network: no `fetch`, no `XHR`, no
  `import()`. (`serve`'s uploader page is a separate, equally inline page —
  the REPORT page itself stays fully static.)
- File size guardrail: the report for a 300-task/2000-event board should
  stay under ~5 MB. No action if exceeded; log a note.

---

## 7. Acceptance criteria

### 7.1 BT-020 — `boardctl render` (this spec's first implementation target)

1. `boardctl render -C <this repo> -o /tmp/r.html` exits 0 and produces a
   file whose size is >50KB and <10MB. Observable: `ls -la /tmp/r.html`.
2. `/tmp/r.html` opens correctly from `file://` in a headless browser:
   page title renders, overview card shows `boardctl` with 25 tasks
   (20 complete / 5 pending), and the browser console shows zero errors.
   Observable: any headless run (chrome `--dump-dom` or playwright) with
   console capture.
3. Topology B: copy a fixture board, delete `board.jsonl`, move the header
   object to line 1 of `tasks.jsonl` → `render` exits 0, the card shows the
   same `ticks_total` (69) and the topology badge `B`. Observable: diff of
   key-numbers card payload between A and B renders.
4. Empty board: `init` a fresh board in a temp dir (`tasks.jsonl` with only
   a header, empty `events.jsonl`) → `render` exits 0 and every chart shows
   its empty-state. Observable: payload `tasks == []`, `events == []`, HTML
   contains the empty-state strings.
5. Torn line: append `{"id":"X","title":"torn"` (no closing brace) to
   `tasks.jsonl` → `render` exits 0, the warning banner reports
   `tasks: 1`, and the X row does not appear. Observable: grep the banner
   text in the HTML.
6. Fixture exclusion: the NEVER-DONE row must not appear in the burndown/
   burn-up/velocity/cycle/streak derivations. Observable: with the fixtures
   toggle OFF the payload's `derived.*` are identical when the fixture row
   is removed from a copy of the board; the task table shows 24 rows OFF,
   25 ON.
7. `</script>`-in-title: create a temp task whose title contains
   `</script><img src=x onerror=alert(1)>` → `render` exits 0, the page has
   exactly one `<script type="application/json"` island that still parses,
   no `onerror` handler exists in the DOM, and the title renders as inert
   text. Observable: headless DOM walk + `JSON.parse(island)` succeeds.
8. Determinism: `render --tz America/Bogota` twice on the same board
   produces byte-identical output EXCEPT the `rendered_at` line.
   Observable: `diff <(sed 's/"rendered_at":[^,]*//' a) <(… b)` is empty.
9. Streak correctness on a synthetic board: completions on
   2026-09-01..09-05 and 09-08..09-10, rendered any time on 09-10..09-11 →
   delivery `current=3 (live)`, `longest=5`; rendered on 09-12 or later →
   `current_state="ended"`, `current_since=2026-09-10`. Observable: payload
   `derived.streaks.delivery`.
10. `go build ./... && go vet ./... && go test ./... -count=1 -short` all
    pass; no board file is modified by a render run (`git status --short`
    clean on a board-only fixture repo after render).

### 7.2 BT-021 — `boardctl serve` (folder + zip upload, multi-board)

1. `boardctl serve` binds `127.0.0.1:8787`; `curl -s http://127.0.0.1:8787/`
   returns the uploader page. Observable: curl 200 + form present.
2. `boardctl serve --addr 0.0.0.0:8787` refuses to start (exit 2, explicit
   non-loopback message). Same for `--addr 10.0.0.1:8787`.
3. Zip upload containing TWO board dirs (one topology A, one topology B)
   renders the multi-board report: two overview cards + the compare section
   with overlaid burn-up and the key-numbers table including `ticks_total`
   and `ticks_idle` per board. Observable: payload `len(boards) == 2`.
4. Folder upload (multi-file `webkitdirectory` POST, relative paths
   preserved) of one board renders the same report as `render -C` on that
   folder. Observable: payload equality on `derived` between the two paths.
5. Zip-slip defense: an upload containing an entry named
   `../../evil.txt` must not write outside the temp extraction dir; serve
   either rejects the entry (warning) or sanitizes it, and the report still
   renders. Observable: no file created outside the temp dir; extract via a
   crafted zip in a test.
6. Upload size cap: a payload above the cap (default 512 MB, constant in
   code) is rejected with HTTP 413 and a readable message; the server stays
   up (next upload works).
7. Loopback-only runtime check: during a serve session,
   `ss -tlnp | grep 8787` shows the listener on `127.0.0.1` only.
8. Zero board writes: serve never writes to the uploaded sources or the
   `-C` board; extraction happens under a fresh temp dir per upload,
   removed on shutdown.

### 7.3 BT-022 — `boardctl import <export.json> [--dry-run]`

1. `render --json /tmp/export.json` then
   `boardctl import /tmp/export.json --dry-run` on the SAME board exits 0
   and prints a plan of zero appends and zero updates (idempotent
   round-trip: "0 new tasks, 0 updates, 0 events"). Observable: the plan
   line.
2. Same command WITHOUT `--dry-run` exits 0, and `git status --short` on the
   board repo is CLEAN (nothing changed because nothing differed).
3. Filtered export: delete 2 completed tasks from a COPY of the board, then
   import the full export into the copy with `--dry-run` → plan says
   `2 new tasks`; without `--dry-run` the 2 rows are appended verbatim
   (same serialization style as the file's last line — spaced/compact and
   ASCII probes per `DetectStyle`), `validate` passes, and a second import
   run is again a no-op (criterion 1 holds on the copy).
4. Events: importing an export whose events are absent from the target
   appends them with NEW ids (`MAX(id)+1` sequencing, never the export's
   ids); `--dry-run` states the count ("N events appended"). Observable:
   `wc -l events.jsonl` delta equals N.
5. Conflicting task (same id, different title in export vs board): default
   behavior SKIPS the row, counts it in the plan ("1 skipped (differs)"),
   and writes nothing for it. Observable: plan line + unchanged row bytes.
6. `--dry-run` applies NOTHING: run against a board, `git status --short`
   must be clean immediately after (this is the criterion the brief calls
   out by name).
7. Wrong schema: `echo '{}' > bad.json; boardctl import bad.json` exits 1
   with "unsupported schema" (missing `schema` field), writes nothing.
8. Fixture rows in the export (id in `fixture_ids`): appended to
   `fixtures.jsonl` when the board has one; when it does not, skipped with
   a plan warning ("no fixtures.jsonl on target; 1 fixture row skipped") —
   import never creates `fixtures.jsonl`.

---

## 8. Risks, open questions, future work

Risks

- R1: Timestamp dialect drift. The fleet writes five dialects today
  (`timefmt.go`); a sixth would silently fall into `timestamp_excluded`
  warnings. Mitigation: the parse-warning surface (2.7) makes any drift
  visible in every report.
- R2: Large boards blow up the single file (payload embeds raw rows).
  Guardrail 6.3 logs a note at ~5 MB; a future `--lean` flag could drop raw
  rows for chart-only reports (open question below).
- R3: `detail` base64 detection (2.4 step 3) can false-positive on a plain
  sentence that happens to be valid base64 AND valid JSON after decode —
  accepted as negligible (step 3 requires the decoded bytes to parse as a
  JSON object/array), and the raw string is preserved in the payload either
  way.
- R4: Day-boundary semantics depend on the report timezone; the same board
  rendered in two zones can show different streak numbers. Accepted and
  made explicit via `report_timezone` in the payload and the footer note.

Open questions (decided by default unless the foreman says otherwise)

- Q1: Should `import` also merge the board HEADER (ticks_total etc.)?
  Default: NO — headers are foreman-owned counters; import touches task and
  event rows only.
- Q2: Compare-view x-axis "days since first birth" vs calendar alignment?
  Default: days-since-birth (3.10); calendar overlay can come later as a
  toggle.
- Q3: Serve upload retention: Default: temp dir per upload, deleted on
  shutdown, nothing persisted.

Future work (explicitly out of scope here)

- F1: `--lean` render (drop raw rows, charts only).
- F2: Report diffing (two exports of the same board over time).
- F3: Per-lane filtering (primary/-dogfood/-qa boards in one zip rendered
  as one project with lane chips).
- F4: i18n of the report shell (the fleet's docs are English; not now).
