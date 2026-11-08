# BT-097 — WORK DIRECTIONS: graph-aware dispatchability (`ready` + `direction`)

Spec status: DRAFT for review panel.
Row: BT-097 (P1, complexity 3). Builds on BT-096 (`boardctl next`, the single-row
decision) — **this spec does not re-implement `next`**; it defines the
WAVE/DIRECTION layer above it and the shared dispatchability primitive both
verbs consume.

## 0. Problem

The foreman picks work by priority scan. `depends_on` and `initiative` are
already on rows (normalized arrays via the BT-035/BT-055 shape discipline;
initiative slugs via `ValidateInitiativeSlug`), but nothing follows the edges.
BT-096 answers "the ONE row to dispatch next"; BT-097 answers the owner's two
graph questions:

- **WAVE**: which rows are dispatchable *right now*, grouped by initiative,
  and how many workers each group could absorb.
- **DIRECTION**: for each open initiative, how much remains, how much is ready
  now, and what each successive wave would complete — "working in this
  direction gets these things done".

## 1. Scope and non-goals

In scope: two new read-only CLI verbs (`ready`, `direction`), one shared
readiness engine in `internal/board`, cycle detection with named cycles,
blocker-chain naming, JSON output schemas, count-pinned surface updates, tests.

Non-goals (named so the panel can veto):

- **No write path.** Neither verb appends to events.jsonl, mutates
  tasks.jsonl, or creates any file. Same read-only class as `stats` /
  `initiatives` (those append no audit event; neither do these — a read has no
  board effect to audit).
- **No new database, no cache.** Every call is an in-process scan of
  tasks.jsonl (JSONL doctrine). Nothing is memoized across calls.
- **No umbrella (cross-board) integration in this row.** Cross-board
  `depends_on` resolution exists in umbrella.go's `DepResolutions` and is
  richer (repo-qualified identity); wiring `ready` over an umbrella view is a
  follow-up (Open Question 5). Single-board only here.
- **Does not implement `next`.** BT-096 owns it. If BT-096 has not landed when
  this row is implemented, land the shared `Dispatchability` engine first
  (Section 3) and have `next` consume it — one predicate, two consumers,
  never two divergent definitions of "dispatchable".

## 2. Verb surface

Two new verbs in the `cmd/boardctl/main.go` dispatch switch (this alone makes
internal/describe derive them; see Section 8 for the count-pinned duties).

```
ready     [-C dir] [--initiative SLUG] [--json] [--blocked] [--skip-bad-lines]
direction [-C dir] [--json] [--skip-bad-lines]
```

`-C` resolution is the existing three-form `Resolve` path (repo root /
`.coding-hermes` / board dir); board-not-found exits 2 like every other verb.
`--skip-bad-lines` matches `list`'s DF-BOARDCTL-9 degraded-read flag (default
fail-closed on an unparseable line).

### 2.1 `boardctl ready`

Prints every DISPATCHABLE row — status pending (after `NormalizeStatus` alias
resolution), not `deferred`, and every transitive `depends_on` ancestor
complete — grouped by initiative.

Human output shape (illustrative):

```
board: /home/x/proj/.coding-hermes/board/tasks.jsonl
ready now: 7 row(s) across 3 initiative group(s)

  wave-hooks (3 ready — absorbs 3 workers)
    BT-101  P1  title of the first ready row
    BT-104  P2  title of the second ready row
    BT-107  P0  title of the third ready row
  lane-hooks (2 ready — absorbs 2 workers)
    BT-115  P1  ...
  (no initiative) (2 ready — absorbs 2 workers)
    BT-120  P2  ...

blocked: 4 row(s) — re-run with --blocked to name each blocker chain
```

Rules:

- Rows inside each group sort by priority (P0 first, then P1…P5; non-vocabulary
  priorities sort last, file order within a priority). Groups sort by
  descending ready count, then slug (same ordering discipline as
  `Initiatives()`), so output is stable for diffing.
- Rows with **no** `initiative` key form an explicit `(no initiative)` group.
  This deliberately diverges from `Initiatives()`, which refuses to invent an
  unscoped bucket: `initiatives` is a rollup where a synthetic bucket would
  overstate programme coverage, while `ready` is a work LIST — hiding
  dispatchable rows that happen to be unscoped would read as "no work left",
  the exact expensive-wrongness the BT-006 exit-code fix and the BT-096
  motivation call out. (Open Question 3 lets the panel overrule.)
- `--initiative SLUG` filters to one group (exact, trimmed/lowercased compare,
  same as `summarizeRows`). A slug that matches no row prints
  `no rows carry initiative "X" on this board` and exits 0 — never a bare
  empty set, which reads as "no work left" (the initiative.go slug-rationale
  applies verbatim).
- `--blocked` appends a BLOCKED section naming each non-ready open row's
  blocker chain (Section 5) instead of the one-line count. Blocked rows are
  otherwise *named, not hidden*: the trailing count always appears, because
  "ready printed 3 rows" must never silently mean "3 of 7".
- Wave phrasing: `absorbs N workers` = one worker per ready row (a wave is one
  worker, one row, to completion; BT-096's single-row decision composes N of
  them).

Exit codes: `0` rows printed (even when a `--initiative` filter matches
nothing), `1` board error (unreadable file, cycle found — Section 4.3),
`2` usage / board-not-found.

### 2.2 `boardctl direction`

The reverse view, per open initiative (the owner's "working in this direction"
view). An initiative is *open* when it holds ≥1 actionable row (the
`Initiatives()` actionable rule: not complete/failed, deferred counted
separately). Initiatives whose remaining work is all deferred appear with
`ready_now: 0` and are not omitted — the foreman must see the graveyard to
trust the map.

Human output shape:

```
board: /home/x/proj/.coding-hermes/board/tasks.jsonl
3 open initiative(s)

  wave-hooks     total 12 | complete 5 | failed 0 | deferred 1 | remaining 6
                 ready now: 2 (BT-101, BT-104)
                 blocked:   4 (chains: boardctl ready --blocked)
                 wave projection: 2 → 1 → 2 → 1   (6 completions in 4 waves)

  lane-hooks     total 8 | complete 8 | ... (no open rows — done)
```

`wave projection` is the per-wave completion counts under unlimited workers,
assuming every dispatched row completes: wave 1 is the ready-now set; wave k+1
is the set whose deps are all complete-or-in-waves-≤k (topological levels of
the open subgraph, Section 6). It is a *shape* signal (how front-loaded the
initiative is), not a schedule.

### 2.3 JSON schemas (stable — the foreman consumes these)

`ready --json`:

```json
{
  "board": "/abs/path/tasks.jsonl",
  "verb": "ready",
  "initiative_filter": "",
  "total_ready": 7,
  "groups": [
    {
      "initiative": "wave-hooks",
      "count": 3,
      "rows": [
        {"id": "BT-101", "priority": "P1", "title": "...",
         "depends_on": ["BT-096"]}
      ]
    },
    {"initiative": "", "count": 2, "rows": []}
  ],
  "blocked": [
    {
      "id": "BT-110", "priority": "P1", "title": "...",
      "initiative": "wave-hooks",
      "blocker_class": "deps",
      "chain": ["BT-110", "BT-108", "BT-101"],
      "chain_text": "BT-110 <- BT-108 (in_progress) <- BT-101 (pending)",
      "unsatisfied": [{"id": "BT-108", "status": "in_progress"}]
    }
  ],
  "cycles": [["BT-130", "BT-131", "BT-130"]],
  "dangling": [{"id": "BT-140", "dep": "BT-999"}],
  "stats": {"open_rows": 25, "ready_rows": 7, "blocked_rows": 4, "deferred_open": 3}
}
```

Schema contracts:

- Every key above is ALWAYS present (`initiative_filter` empty string, `groups`
  `[]` not null) — a consumer must never branch on key existence. Only `ready`
  populates `blocked` fully (it is computed either way; see Section 5); `blocked`
  is emitted by both verbs' JSON so one schema serves both.
- `groups[].initiative` is `""` for the unscoped group. `rows[].depends_on`
  echoes the row's direct deps as stored (post-normalization array) so a
  foreman can re-derive without a second read.
- `blocker_class` ∈ `deps | dangling_dep | cycle | status | deferred`. A row
  with normalized status ≠ pending gets `status`; a deferred pending row gets
  `deferred` (deferred rows appear in `blocked` — they are "not dispatchable,
  for a named reason", which is the point of acceptance 3).
- `cycles` lists each detected cycle as the closed walk `A, B, …, A`. Rows on
  a cycle never appear in `groups` (they can never become ready); they DO
  appear in `blocked` with `blocker_class: "cycle"` and `chain` naming the
  cycle.
- `direction --json`:

```json
{
  "board": "/abs/path/tasks.jsonl",
  "verb": "direction",
  "initiatives": [
    {
      "initiative": "wave-hooks",
      "total": 12, "complete": 5, "failed": 0, "deferred": 1, "remaining": 6,
      "ready_now": 2,
      "blocked": 4,
      "ready_ids": ["BT-101", "BT-104"],
      "wave_projection": [2, 1, 2, 1]
    }
  ],
  "unscoped": {"total": 4, "remaining": 2, "ready_now": 2, "wave_projection": [2]},
  "cycles": [],
  "dangling": [],
  "stats": {"open_initiatives": 3, "total_ready": 7}
}
```

`unscoped` is a distinct top-level key (never inside `initiatives`) so the
"unscoped is not an initiative" invariant of `Initiatives()` survives in
machine output too. Golden-keys tests pin both schemas (Section 9, T8).

## 3. The shared dispatchability engine (internal/board/ready.go)

One new file in `internal/board` (no new package — it needs `Row`,
`NormalizeStatus`, `rowDeferred`, `TaskRows`, `rowStringSlice`, all already
there):

```go
// The single definition of "dispatchable". BT-096's `next` picks the top of
// ReadyRows; BT-097's ready/direction render the whole set. One predicate.
type GraphReport struct {
    Ready      []*Row            // dispatchable now
    Blocked    []BlockedRow      // open rows that are not, with chains
    Cycles    [][]string         // each cycle as a closed walk
    Dangling   []DanglingRef     // depends_on ids that resolve to no row
}
type BlockedRow struct {
    Row      *Row
    Class    string            // deps | dangling_dep | cycle | status | deferred
    Chain    []string          // row first, root blocker last
    Unsatisfied []UnsatisfiedDep // direct deps that are not complete
}
func (b *Board) AnalyzeDispatchability() (*GraphReport, error)
```

### 3.1 Semantics (all classifications use `NormalizeStatus`, so read-side
aliases like `done`/`open` behave exactly as `list` behaves — never a second
vocabulary)

- **Satisfied dep**: the dep id resolves to a task row on THIS board whose
  normalized status is `complete`. Nothing else satisfies — `failed`,
  `in_progress`, `review`, `blocked`, `pending` all block. A self-edge
  (`A` depends on `A`) is a cycle of length 1, reported as such.
- **Dangling dep**: dep id resolves to no task row (fixture rows excluded —
  `Initiatives()`' fixture filter is the precedent). The referencing row is
  never ready, class `dangling_dep`, and the missing id is named. A dangling
  dep must never silently read as satisfied or as a cycle — validate already
  warns on dangling deps and this surface names them again where they bite.
- **Dispatchable**: normalized status == `pending` AND not `rowDeferred` AND
  all transitive ancestors satisfied AND not on a cycle AND no dangling dep.
- **Transitive resolution**: the full ancestor closure, not direct deps only.
  Acceptance 7's falsifier is exactly this: A ← B ← C (C depends on B, B on
  A) with A pending must free **only A** — a shallow implementation would
  wrongly free C once B completes... and wrongly free nothing today. The test
  pins both directions (T1).
- **Deferred ancestors**: a deferred ancestor is NOT complete, so it blocks.
  Its chain names it (`BT-108 (pending, deferred)`), so a wave plan surfaces
  "this stalls behind a deferral decision" rather than a mystery.

### 3.2 Blocker chains

For every open row that is not ready, the chain is the path from the row
through unsatisfied direct deps, recursing into each unsatisfied dep's own
unsatisfied deps, terminating at the root blocker(s). Concretely: BFS/DFS from
the row along dep edges, following ONLY edges into unsatisfied rows; the first
satisfied (complete) row reached is a leaf and stops the branch; rows already
visited in this walk are not revisited (shared blockers appear once). Chain
text format: `X <- Y (status) <- Z (status)` with statuses normalized and
deferred/failed annotated. The chain names the FIRST unsatisfied hop at each
level with its status — acceptance 3's "which dependency, at which status".

### 3.3 Cycle detection

Iterative DFS over the dep graph with white/gray/black coloring; the gray
stack at backtrack IS the cycle, reported as a closed walk. Iterative, not
recursive: a 10k-row degenerate chain must not depend on goroutine stack
growth (and Go's resizable stacks would survive, but the bound is the spec).
Multiple independent cycles each appear in `cycles`. Rows on any cycle: class
`cycle`, excluded from ready, chain names the walk. The command exits 1 when
`cycles` is non-empty (an error finding, like umbrella's error-class findings)
**while still printing the report including non-cyclic ready rows** — degrade
with evidence (DF-BOARDCTL-9 house rule); a poisoned programme must not erase
the sight of healthy work. (Open Question 2.)

## 4. Initiative grouping

- Grouping key: `strings.ToLower(strings.TrimSpace(row.String("initiative")))`
  — the exact `summarizeRows` rule, reused, not reimplemented. Slugs were
  validated at the write surface; read-side rows with garbage initiative
  values (pre-validation boards) group under their raw normalized value —
  `ready` never refuses a read.
- `direction` reuses `Initiatives()`-shaped accounting (total/deferred/
  actionable per slug) plus the graph counts. Where `Initiatives()` computes
  Actionable, `direction` refines it: remaining = actionable, ready_now =
  dispatchable per Section 3, blocked = remaining − ready − in-flight note.
- Deferred rows count in `total` and their own bucket (BT-076 discipline:
  deferred is not actionable, never silently merged into remaining).
- Umbrella boards: out of scope (Section 1); nothing in `ready.go` reads the
  links registry, so an umbrella `-C` target behaves exactly as it does for
  `stats` today (the umbrella's own board only, plus the standard warning).

## 5. Performance approach

- One pass: `TaskRows()` already streams tasks.jsonl once (the same read path
  `list` uses to return 227 rows instantly). Build `id → *Row` and the
  dependency adjacency during that pass.
- Readiness: Kahn topological sort over the open subgraph — O(rows + edges).
  Wave levels for `direction` fall out of the same sort for free (level =
  1 + max(level of deps)); no second traversal.
- Blocker chains: bounded single walk per blocked row along unsatisfied edges
  with a visited set — O(rows + edges) total across all rows.
- Memory: O(rows + edges). No locks (reads take none; `NormalizeBoard` locks
  because it writes). No caching between verbs or calls.
- Budget: 10k rows / ~10k edges must render text AND JSON well under 2s
  (acceptance 5). `list` already proves the scan cost; the graph pass adds
  linear work. T6 enforces with a real timing assertion, not a claim.
- `--json` marshals the structs above directly; no intermediate rendering.

## 6. Count-pinned surfaces (acceptance 8 — all three bumped together, verbs named)

1. `cmd/boardctl/main.go` switch: add `case "ready"` and `case "direction"`,
   plus both verbs' usage lines in `usageText` (the help case).
2. `internal/describe/describe.go` `cliSummaries`: add curated entries —
   `"ready": "list dispatchable rows grouped by initiative (graph-aware wave sizing)"`
   and `"direction": "per-initiative direction view: remaining, ready now, wave projection"`.
   The derived-verb count test asserts the switch and the curated list agree;
   describe_test.go's count must be bumped WITH the verb names.
3. `skills/boardctl-usage/SKILL.md`: name BOTH verbs in **Entry points** AND
   add proven-command blocks under **Proven commands** (the df_boardctl_7
   usage test refuses a command absent from both — a one-section edit fails
   the gate):

```
# graph-aware dispatch (BT-097): what can a wave start NOW?
boardctl -C R ready [--initiative SLUG] [--blocked] [--json]
                                              # dispatchable rows (pending, not
                                              # deferred, TRANSITIVE deps complete)
                                              # grouped by initiative, per-group
                                              # worker capacity; --blocked names
                                              # each blocker chain; cycles exit 1
boardctl -C R direction [--json]              # per-initiative: remaining / ready
                                              # now / wave projection — the
                                              # "which direction pays" view
```

Also update the `boardctl next` entry (BT-096's skill contract) to say
`ready` is the multi-row view of the same predicate.

## 7. Migration / back-compat

- **Zero migration.** No schema change, no row writes, no events, no new
  files under the board dir, no header fields. A board that pre-dates
  `depends_on` needs nothing: with zero dep edges every pending non-deferred
  row is dispatchable, and `ready` degenerates exactly to
  "`list --status pending` minus deferred, grouped by initiative". T9 pins
  that equivalence row-for-row on a dep-free fixture board.
- Old boards with string `depends_on` drift: unchanged behavior —
  `rowStringSlice` reads what normalize-board would canonicalize; a drifted
  row still parses to the same array `list` sees. `ready` never repairs;
  it points at `boardctl normalize-board` in its dangling/refusal notes when
  relevant.
- No flag or exit-code already in use is repurposed. Exit 3 (BT-096's
  spec'd "board empty of dispatchable work") is NOT introduced by this row —
  `ready` with zero ready rows exits 0 and prints
  `ready now: 0 row(s)` + the blocked census, because "empty" is a normal,
  informative answer here, not a failure (Open Question 1 defers the exit-3
  question to BT-096's implementer).
- JSON schema keys are namespaced fresh (`ready`/`direction` verbs); nothing
  existing (`stats`, `initiatives --json`) changes shape.

## 8. Test plan (every test carries a control that proves it can fail)

File layout following house convention: `internal/board/bt097_ready_test.go`
(engine) and `cmd/boardctl/bt097_cli_test.go` (CLI, using the established
build-binary-and-run pattern from bt078_umbrella_cli_test.go).

| # | Test | Falsifier it kills | Control |
|---|------|--------------------|---------|
| T1 | Transitive chain: C→B→A (A pending) ⇒ ready = {A} only | Shallow direct-dep resolution | Same board with A `complete` ⇒ ready = {B} exactly, C still blocked — proves the assertion distinguishes levels |
| T2 | Cycle A→B→C→A: exit 1, `cycles` names the closed walk, no cyclic row in `groups` | Infinite loop / silent never-ready | Same board minus edge C→A ⇒ exit 0, all three rows classified — proves the test can pass and the failure is the edge, not the harness |
| T3 | Dangling dep: names the missing id, class `dangling_dep`, row never ready | Silent satisfaction or crash | Add the missing row as `complete` ⇒ row flips ready |
| T4 | Initiative grouping: 3 slugs + unscoped; `--initiative` filter; ordering stable across two runs | Bucket leaks / nondeterministic order | Unset one row's initiative ⇒ it moves to `(no initiative)` group in the same run |
| T5 | Blocked chains: `X <- Y (in_progress) <- Z (pending)` text and JSON `chain`/`unsatisfied`; deferred ancestor annotated | Acceptance-3 regression (blocked rows "just disappear") | Complete Z ⇒ Y's chain shortens by one hop in the same output |
| T6 | Perf: synthetic 10k-row board (chains + fan-out), text and JSON each render < 2s (`testing.AllocsPerRun`-free wall clock, `t.Deadline`-guarded) | O(rows²) resolution / recursion blowups | Assert the 10k board's row count really is 10k before timing (a 10-row board passes any timer — the control is the size assertion) |
| T7 | Status gating: `in_progress`/`review`/`blocked`/`failed` rows never ready; alias `done`/`open` inputs behave as complete/pending (read-alias parity with `list`) | A second status vocabulary | Row spelled `done` satisfies a dep exactly as `complete` would — and `blocked` never does |
| T8 | JSON schema stability: golden key-set test on both verbs' output (every Section 2.3 key present, nil-as-`[]`) | Silent key drift breaking foreman consumers | Corrupt one key in a fixture expectation ⇒ test fails (proves the golden comparison is live) |
| T9 | Back-compat: dep-free board ⇒ `ready --json` rows == `list --status pending` rows minus deferred, ids equal as sets | Regression for pre-dep boards | Add one dep edge ⇒ sets diverge by exactly that row |
| T10 | Count-pinned: describe registry contains `ready`+`direction` WITH summaries; usage-skill test passes (Entry points AND Proven commands name both) | Verb lands, pin-test forgotten | Revert one cliSummaries entry ⇒ gate refuses |

Engine tests build boards via `write.go`'s guarded append (the same helpers
bt055_depends_shape_test.go uses) — no hand-rolled JSONL in tests.

## 9. Open questions for the review panel

1. **Exit code for "nothing ready"**: this spec keeps exit 0 with an explicit
   zero-count line for `ready`; BT-096's row spec'd exit 3 for `next` on an
   empty board. Should `ready` match exit 3 for script symmetry, or is 0
   correct since zero-ready is diagnostic input (the blocked census) rather
   than a dead end?
2. **Cycle exit semantics**: exit 1 with the full report still printed
   (degrade-with-evidence, chosen here) vs. hard refuse printing only the
   cycle. The panel should confirm which the foreman contract prefers.
3. **Unscoped group**: this spec prints an explicit `(no initiative)` group in
   `ready` (work-list semantics) while `initiatives` refuses synthetic
   buckets (rollup semantics). Keep the divergence, or hide unscoped rows
   behind a `--all`-style flag?
4. **`wave_projection` assumptions**: unlimited workers, every dispatched row
   completes, no cross-initiative dep stalling. Good enough as a shape signal,
   or should the projection account for per-row `complexity` (worker-seconds)
   once the fleet standardizes on interpreting it?
5. **Umbrella `ready`**: cross-board deps already resolve in `DepResolutions`.
   Follow-up row to render a repo-qualified ready view over the umbrella
   (identity `<repo>/<id>`), or permanently out of scope because dispatch
   stays single-repo by doctrine?
6. **`blocked` status rows**: a row explicitly marked `blocked` with all deps
   complete is class `status`, not dispatchable. Should `direction` count it
   in `blocked` (chosen here — it is remaining work with a named reason) or
   in a separate "parked" bucket so wave math ignores it?

## PANEL VERDICTS — 2026-11-08 (binding amendments)

Endorsed with fixes. The following override or amend the body of this spec:

1. **Export the per-row predicate NOW (was deferred to BT-099).** `AnalyzeDispatchability` ships with a named, testable per-row `Dispatchable(row, board)` alongside the batch report — BT-099's suppression composes into THE ENGINE (a `Withheld` set in `GraphReport`), not into the `ready` CLI, so BT-096's `next` inherits suppression and can never dispatch a withheld parent.
2. **T8 is a FULL-DOCUMENT golden** (values AND types) on fixtures — a key-presence test cannot pin nil-as-`[]` (Go marshals nil slices as `null` with the key present).
3. **T6 drops the fixed wall clock.** Assert the SCALING RATIO (10k rows vs 100 rows < 20×) with the measured time logged; strict bound skipped under `-short`. Load on the shared box (measured 13.9/15.4) makes a fixed 2s assertion a flake. Keep the size-assertion control.
4. **`blocked` schema divergence named explicitly** in §2.3: `ready` emits the blocker array; `direction` emits per-initiative counts. Consumers are told which shape each verb returns — or both emit the array (implementer's choice, documented either way).
5. **Routing note:** `blocker_class` `deps` folds waiting vs failed ancestors; routing uses class + `unsatisfied[].status` (a pending ancestor = waiting; a failed ancestor = rework). Splitting into `deps_waiting`/`deps_failed` is sanctioned if cleaner. Owner-gated rows are outside boardctl's field set — name that boundary in the help text.
6. **Phrasing:** "absorbs N workers" → "could absorb N workers" (board-pending ≠ unowned; sibling lanes hold rows mid-dispatch).
7. **Comment events (BT-099) are never rewritten by repair/normalize tooling** — state in the repair-path docs. Comment-event retention/compaction doctrine tracked on BT-100.
8. **Initiative sparsity:** only 5 of 73 dep-carrying rows name an initiative — `direction` is near-empty at launch. Backfill of `initiative` keys on the live boards is filed as its own row (BT-102).
