# BT-099 — Subtasks, Comments, and Decomposition Metadata

Status: DRAFT spec for panel review
Depends on: BT-098 (types/templates/configurable metadata) — this spec builds on its
`detail.metadata` typing and validator; coordinates with BT-097 (work directions /
`ready`/`direction`) — the required interface from that spec is stated in §6.
Owner quote (2026-11-08): "look at Axiom — it has a baby steps skill and a way it
breaks down work; we want boardctl to have subtasks and metadata for that — the
foreman should know the complexity of the work, what files it thinks are needed, the
substeps, verification steps; this is just a board AND COMMENTS included that lets
the system keep track of everything."

---

## 1. Doctrine (non-negotiable)

1. **Comments are TYPED APPEND-ONLY EVENTS, never row rewrites.** A comment appends
   one row to `events.jsonl` through the existing `AppendEvent` write path
   (`internal/board/write.go: appendEventLocked`, id = MAX(existing id)+1, write
   guard held, event_type vocabulary-checked). `tasks.jsonl` is never touched by a
   comment. The audit trail IS the feature.
2. **Parent links are ROW FIELDS, single-valued, write-time validated.** `parent`
   is a new canon row key (string, one id). Rollups are **computed at read time** —
   nothing about children is ever stored on the parent row.
3. **JSONL canonical, no database.** No new files. Everything lands in the existing
   `tasks.jsonl` + `events.jsonl` pair.
4. **Zero migration for existing boards.** Rows without `parent` behave exactly as
   today (219+ complete rows unchanged). `comment`/`decompose` are additive verbs.

## 2. Verb surface

Two new CLI verbs in `cmd/boardctl/main.go`'s `run()` switch (the switch is the
describe-derivation anchor — adding the cases automatically adds the verbs to
`internal/describe`'s derived list):

```
boardctl comment <ID> (--body "text" | --body-file <path>)
                 [--author NAME] [--reply-to <event-id>] [-C dir]

boardctl decompose <PARENT-ID> --count N [--parallel] [--first-id ID]
                 [--title-prefix "string"] [-C dir]
```

- `comment` today is **not a verb**: `boardctl comment X` falls through to
  `default` → usage, exit 2, nothing written. This spec makes it one.
- `comment --author` defaults to `foreman` (same default as `EventSpec.Actor`).
- `comment --reply-to` takes a numeric `events.jsonl` event id; it must reference an
  existing comment event **on the same task** (refusal names the id and the task it
  belongs to). Threading is one level deep in rendering (§4); the data model allows
  arbitrary chains.
- `decompose --count` is required and must be ≥ 1 (0 or negative = exit 2 usage).
  `--parallel` switches child deps from sequential chaining to independent
  (§5). `--first-id` optionally pins the first child id; subsequent ids are minted
  `next-id`-style (max+1, observed width preserved). Default: all ids auto-minted.
- Both verbs respect `-C` per the standard three-form resolution.

New/extended create/update flags (both verbs of row writing):

```
boardctl create ... --expected-files "a.go,b/c_test.go"   # comma list → array
boardctl create ... --verify-steps "run go test" --verify-steps "prove X"  # repeatable → array
boardctl create ... --complexity N        # ALREADY EXISTS (canon key) — unchanged
boardctl update <ID> --expected-files ... --verify-steps ... --complexity N
```

## 3. Comment event schema

Appended row in `events.jsonl` (inherits the last numeric-id row's key order/style/
timestamp layout, per `appendEventLocked`):

```json
{"id": 4183, "timestamp": "2026-11-08 05:20:11.123456", "event_type": "comment",
 "task_id": "BT-099", "actor": "bane",
 "detail": "{\"body\":\"split BT-098's validator first\",\"in_reply_to\":4179}",
 "tick_number": null}
```

- `event_type`: `"comment"` — **added to `EventTypeVocabulary`**
  (`internal/board/read.go`), the write-time check in `appendEventLocked`.
- `task_id`: the commented row's id (required; the verb refuses a nonexistent id —
  comment must land on a real task row, tasks or fixtures both acceptable via the
  same lookup `show` uses).
- `actor`: `--author`, default `foreman`.
- `detail`: a JSON **object string** with exactly:
  - `body` (string, required, non-empty after trim — empty body = exit 1 refusal;
    `--body` and `--body-file` are mutually exclusive, exactly one required);
  - `in_reply_to` (int, optional, present only when `--reply-to` given).
- Refusals (exit 1, nothing appended, event id not consumed — MAX+1 recomputes):
  unknown task id, empty body, both/ neither of `--body`/`--body-file`,
  `--reply-to` not an existing comment event on the same task.
- The comment does NOT bump `updated_at` on the task row and does NOT rewrite
  `tasks.jsonl` in any way. (`event` today bumps the header tick counter;
  `comment` inherits `AppendEvent`'s tick behavior — see Open Question Q4.)

### show rendering

`boardctl show <ID>` gains a thread block after the row body:

```
COMMENTS (2)
  #4179 2026-11-08T04:55 bane: split BT-098's validator first
    #4183 2026-11-08T05:20 foreman: agreed, filed as BT-098a
```

Ordered by event id; `in_reply_to` indents one level under its parent comment
(deeper chains still indent under their ancestor). `--json` on show emits the
thread as a stable array: `{event_id, ts, author, body, in_reply_to}`.

## 4. Parent/child rules and refusals

New row key `parent` (string, ≤1 value) — sanctioned in `internal/board/keys.go`
(`sanctionedExtras`, with a census-style comment: no live rows carry it today, so
this is the declared start of the concept, not a measured drift).

Write-time validation on **create and update** (the only paths that may write
`parent`; hand-edited rows are caught by `validate`'s new check, §4.1):

| Condition | Behavior |
|---|---|
| `parent` value == row's own id | refuse: "self-parent" |
| `parent` id does not exist on the board | refuse: names the id |
| `parent` row status is `complete` (or any closed terminal status) | refuse: "parent is complete — open rows only" |
| setting `parent` creates a cycle (follow parent links from the new parent upward; the row is reachable) | refuse: print the cycle `A → B → C → A` |
| re-parenting (update changes `parent`) | allowed; re-run all checks above from the row's perspective |
| removing `parent` (update `--parent ""`) | allowed — child becomes a standalone row; rollups recompute |

A row has **at most one parent**; a parent has unbounded children. `parent` is
orthogonal to `depends_on` (a child may or may not depend on siblings; decompose
sets `depends_on` per §5 and `parent` always).

### 4.1 Read-time rollups (computed, never stored)

`boardctl show <PARENT>` prints, when the row has children (rows whose `parent`
== this id):

```
CHILDREN: 2/5 open (complete 3, pending 1, in_progress 1)
CHILD DEPS (union): BT-098, BT-096   ← union of children's depends_on not yet satisfied
```

- open/total children: children with status in the open vocabulary vs total.
- **union of child deps**: the union of every child's `depends_on` refs that are
  not themselves complete — the "what's still in the way of this parent" line the
  foreman reads before dispatching. Computed in the same graph pass `ready` uses
  (§6) so the two surfaces cannot disagree.
- `validate` reports a dangling `parent` (points at nothing), a complete-parent
  violation, and a parent cycle as **errors** naming the ids (same classification
  as the dangling-`depends_on` family; `--fail-on parent-cycle` promotes to exit-1
  the same way `--fail-on dangling-dep` does).

## 5. `boardctl decompose` semantics

`decompose <PARENT>` scaffolds N child rows from the parent as template. Each
child:

- inherits: `priority`, `capability_tags`, `type` + configured metadata (BT-098
  template fields), `expected_files`, `verify_steps` — **the parent template**;
  the intent is the foreman edits per-child after scaffolding, not that decompose
  guesses.
- carries `parent: <PARENT-ID>` (validated by §4 machinery — parent must be open;
  decomposing a complete row is refused).
- does **NOT** carry `depends_on: [<parent>]` — the parent is a tracking shell,
  not a blocker; the linkage is the `parent` key. (DRAFT decision; panel Q2.)
- ids: `next-id`-style auto-mint in one family (e.g. BT-101..BT-105), or
  `--first-id` pins the first.
- title: `<PARENT title> — step i/N` unless `--title-prefix` is given
  (then `<prefix> i/N`). Status `pending`, priority inherited.
- deps:
  - **sequential (default)**: `child_i.depends_on = [child_{i-1}]`, `child_1` has
    none — the Axiom baby-steps chain: work one step at a time.
  - `--parallel`: no inter-child deps — children are independent, the wave can
    take all N at once.

Write path: one `Create` per child through the ordinary guarded create path
(duplicate scan, id-format check) — decompose is a LOOP over Create, not a bulk
writer, so every refusal (e.g. id collision mid-loop) is the standard named
refusal and previously-created children of a failed decompose remain (documented,
not rolled back: the board is append-only doctrine; re-running with `--first-id`
resumes). Each successful create emits its ordinary `task_created` event. The
parent gets one `task_updated` event with detail
`{"decomposed_into": ["BT-101","BT-102","BT-103"]}` so the audit trail shows the
scaffolding act.

### Example (spec-must-include)

```
$ boardctl decompose BT-099 --count 3 --first-id BT-101
BT-101 created: "SUBTASKS + COMMENTS… — step 1/3" (parent BT-099, deps: —)
BT-102 created: "SUBTASKS + COMMENTS… — step 2/3" (parent BT-099, deps: BT-101)
BT-103 created: "SUBTASKS + COMMENTS… — step 3/3" (parent BT-099, deps: BT-102)

$ boardctl -C . show BT-099
CHILDREN: 3/3 open
CHILD DEPS (union): (none complete yet)
```

## 6. Interface required from BT-097 (`ready`/`direction`)

BT-097's spec (`specs/BT-097-work-directions.md`) does not exist yet. BT-099 needs
exactly one thing from it, stated here so the two specs can be reconciled:

> **`ready`'s dispatchability predicate must be a named, testable function over
> the row graph** (e.g. `Dispatchable(row, board) bool` behind the transitive
> dep resolution), not inline logic in the command — so BT-099 can compose a
> parent-suppression rule with it instead of forking the graph walk.

BT-099's suppression rule, composed on top of BT-097's predicate:

- A parent row with ≥1 open child is listed by `ready` **only when** (a) the
  parent itself satisfies BT-097's transitive-deps predicate, **and** (b) none of
  its open children are also in the ready set. Children first, then the parent —
  the same seam is never double-dispatched in one wave.
- When suppression fires, `ready` prints the parent in a withheld block:
  `WITHHELD (open children ready): BT-099 — dispatch BT-101..BT-103 first`.
- `direction` (BT-097's reverse view) shows parent rollups using §4's read-time
  computation — one shared helper (`ParentRollup(row, board)`) consumed by both
  `show`, `ready`, and `direction`, so the numbers cannot drift between surfaces.
- If BT-097 lands first without the predicate extracted, BT-099's first task is
  that extraction (behavior-neutral, covered by BT-097's own tests).

## 7. Count-pinned surfaces (must be updated WITH the verbs named)

1. `skills/boardctl-usage/SKILL.md` — **Entry points** section gains one bullet
   per new verb (`comment`, `decompose` — and, if landed in the same wave,
   BT-097's `ready`/`direction`); **Proven commands** gains a commented example
   block for comment threads + decompose, in the file's established style.
2. `internal/describe` — the switch-derivation picks up the new verbs
   automatically; `cliSummaries` gets a curated one-line description per verb (a
   derived verb without a curated line is reported but bare — add the lines).
   Any pinned verb-count assertion in `describe_test.go` / usage tests updates in
   the same commit.
3. `docs/` + README quickstart verb list if it enumerates verbs (check at
   implementation time; the enumerate-or-die rule: if a surface pins the verb
   list by count, the count changes in the same commit).

## 8. Test plan (falsifiable)

Each test names the falsifier it kills:

1. **Comment thread append-only proof** — record `events.jsonl` bytes before
   `comment`; append; assert pre-bytes are an EXACT PREFIX of post-bytes
   (`bytes.HasPrefix`), `tasks.jsonl` bytes are byte-identical, and exactly one
   line was added. Falsifies: comment-as-row-rewrite.
2. **Comment schema** — appended event parses; `event_type == "comment"`, actor
   default `foreman`, `--author` honored, `detail` round-trips to
   `{body, in_reply_to}`; `in_reply_to` present only when passed. Falsifies:
   schema drift / loose keys.
3. **Comment refusals** — unknown id, empty body, both/neither body flag,
   `--reply-to` to a non-comment or foreign-task event: each exits 1, appends
   NOTHING (event count and MAX(id) unchanged). Falsifies: half-written audit.
4. **show prints the thread** — 3 comments with one reply render in id order with
   the reply indented; `--json` array matches the schema. Falsifies: silent
   thread.
5. **Parent refusals** — self-parent, nonexistent parent, complete parent, 2-node
   and 3-node cycle: each exits 1 with a message naming the ids/cycle; row
   unchanged. Falsifies: cycle hangs or silent accept.
6. **Rollup correctness** — parent with 5 children (3 complete): show prints
   `2/5 open`; union of child deps omits satisfied deps and includes unsatisfied.
   Mutating a child's status flips the rollup on the NEXT read with no write to
   the parent row (parent row bytes identical). Falsifies: stored/stale rollups.
7. **decompose scaffolding** — `--count 3` sequential: children i>1 depend on
   i−1, all carry `parent`, template fields inherited, parent's audit event lists
   all three ids; `--parallel` produces no inter-child deps. Falsifies: wrong
   chain shape.
8. **decompose refusals** — complete parent, `--count 0`, mid-loop id collision
   (documented partial-scaffold behavior). Falsifies: unguarded bulk write.
9. **No-double-dispatch** — board with parent P (deps satisfied) + open child C
   (deps satisfied): `ready` lists C and withholds P; after C completes, `ready`
   lists P. With the child blocked, P (deps satisfied) may appear. Falsifies:
   parent+child same-wave dispatch.
10. **Zero-migration** — a board with 219 legacy rows and no `parent` anywhere:
    validate clean, stats/show/ready byte-identical to pre-change binary output.
    Falsifies: accidental shape change to existing boards.

## 9. Open questions for the panel

1. **Where decomposition metadata lives**: this spec writes `expected_files` and
   `verify_steps` as top-level **canon row keys** (typed arrays, sanctioned in
   `keys.go`), so boards without BT-098's `types.jsonl` work unchanged. BT-098's
   doctrine says row-level metadata belongs in `detail.metadata`. Fold them into
   `detail.metadata` once BT-098 lands (and keep `--complexity` on the existing
   top-level canon key it already is)? Panel to pick one home before implementation.
2. **Should children depend on the parent?** This spec says no (parent is a
   tracking shell; `parent` key is the linkage). Alternative: child_1 depends on
   the parent so the parent can't close before step 1. Panel call.
3. **Partial-failure decompose**: keep the documented leave-what-landed behavior
   (append-only doctrine) or add a `--dry-run` printing the N rows first? Cheap to
   add both; opinion: add `--dry-run`, keep no-rollback.
4. **Tick bump on comment**: `AppendEvent` bumps the header `ticks_total` like
   every event. A comment is human chatter, not tick work — inherit the bump
   (uniform, simplest) or pass `NoTickWrite` (quiet board header, but two event
   classes now behave differently)? Opinion: inherit; the bump is already the
   norm and the header counter is cheap.
5. **Umbrella scope**: parent/child and comments are single-repo only (writes
   never cross child symlinks, BT-078). Does `umbrella --list` need a parent
   column in v1, or defer? Opinion: defer.
