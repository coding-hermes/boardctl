# BT-098 — Ticket Types + Templates + Configurable Metadata

Status: SPEC (awaiting panel review) · Depends on: BT-097 · Complexity: 3
Board: `boardctl -C . show BT-098` · Spec path: `specs/BT-098-ticket-types.md`

## 1. Problem Statement

boardctl rows are all one shape: a free-text title, a prose detail, and whatever
keys a writer happened to invent. Bane wants Jira-style typed tickets — the board
declares its own ticket types, each type carries a template (title prefix,
acceptance-criteria skeleton, default priority) and a metadata schema (required
and optional named fields), and `create`/`update`/`validate` enforce it at write
time. The point is foreman-visible structure: type + required metadata make a
row's complexity and shape machine-readable at filing time instead of buried in
prose. The config is board-local, append-only, git-backed JSONL — the same
doctrine as the board itself. No database, ever.

## 2. Config surface: `types.jsonl`

### 2.1 Location and file discipline

- Path: `<board>/types.jsonl` (sibling of `tasks.jsonl`, `events.jsonl`,
  `board.jsonl`, `fixtures.jsonl`). One JSON object per line, LF-terminated.
- **Append-only**: a type is never edited in place and never deleted. To change
  a type, append a new line with the same `slug` and a bumped `rev`; the LAST
  line for a slug wins. To retire a type, append `{"slug": X, "retired": true, ...}`.
  This keeps the file a git-diffable audit log, exactly like the board events.
- **Git-backed**: the file lives in the board dir the operator already commits.
  boardctl itself never runs git (same rule as every other board file).
- **Absent file = built-in defaults.** When `types.jsonl` does not exist (or is
  empty), the type registry is the four built-ins:

| slug    | display | template | required metadata |
|---------|---------|----------|-------------------|
| `task`  | Task    | none     | none              |
| `bug`   | Bug     | none     | none              |
| `feat`  | Feature | none     | none              |
| `chore` | Chore   | none     | none              |

  Built-ins carry NO template and NO required metadata, so every existing
  behavior is byte-for-byte unchanged until an operator writes a types.jsonl.
  **Zero migration** for the 219+ complete rows on live boards.

### 2.2 Row schema

Each line is one type-declaration object. Unknown keys inside a type row are a
WARN (printed to stderr), not an error — canon-drift doctrine: report, never
guess, never refuse a config file the operator owns.

| field                 | type              | required | notes |
|-----------------------|-------------------|----------|-------|
| `slug`                | string            | yes      | lowercase slug, same grammar as initiative slugs (`^[a-z0-9]+(-[a-z0-9]+)*$`, max 64) — reuse `ValidateInitiativeSlug` semantics via a shared slug rule, so `--type release_v2` is refused with the "did you mean" hint |
| `display`             | string            | yes      | human name, e.g. `"Release"` |
| `rev`                 | int               | no       | defaults 1; later lines for the same slug must have a strictly greater `rev` or the file is rejected at load (two lines, one truth) |
| `retired`             | bool              | no       | true = type stays resolvable for existing rows but `create --type` refuses it |
| `template`            | object or null    | no       | see 2.3 |
| `metadata`            | array of fields   | no       | see 2.4; each field is `{name, type, required, description, values, ref}` |
| `metadata[].name`     | string            | yes*     | *required when the field object is present; must be a lowercase snake_case identifier (`^[a-z][a-z0-9_]*$`, max 64) |
| `metadata[].type`     | enum of string    | yes*     | one of `string`, `enum`, `list`, `ref` |
| `metadata[].required` | bool              | no       | default false |
| `metadata[].description` | string         | no       | shown by `boardctl types` |
| `metadata[].values`   | array of string   | enum only| the closed enum vocabulary; empty/absent values on an enum field = config error at load |
| `metadata[].ref`      | string            | ref only | what the value references; `ref: "task_id"` is the only meaning v1 resolves (validate checks the id exists on the board) |

### 2.3 Template object

```json
"template": {
  "title_prefix": "[RELEASE] ",
  "ac_skeleton": "- [ ] staged\n- [ ] signed\n- [ ] changelog entry",
  "default_priority": "P1"
}
```

All three keys optional; a key that is absent simply does nothing.

### 2.4 Full worked example — the `release` type

A release type with a **required enum field** (`channel`) and an **optional
list field** (`signoffs`), plus a required `string` and an optional `ref`:

```json
{"slug":"release","display":"Release","rev":1,
 "template":{"title_prefix":"[RELEASE] ","default_priority":"P1",
             "ac_skeleton":"- [ ] artifact built and signed\n- [ ] deploy notes written\n- [ ] rollback path verified"},
 "metadata":[
  {"name":"channel","type":"enum","required":true,"values":["stable","beta","nightly"],
   "description":"release channel this artifact ships to"},
  {"name":"signoffs","type":"list","required":false,
   "description":"reviewers who signed off (names or emails)"},
  {"name":"source_commit","type":"string","required":true,
   "description":"commit the release was cut from"},
  {"name":"approval_ticket","type":"ref","ref":"task_id","required":false,
   "description":"task id of the approval record"}
 ]}
```

### 2.5 Where data lands on a row

- The type itself is the row's top-level `type` key — **already sanctioned** by
  `internal/board/keys.go` `sanctionedExtras`, so no canon change is needed.
- Metadata is a **typed object at `detail.metadata`**, never loose top-level
  keys. `detail` is already sanctioned and already carries fleet sub-objects
  (e.g. `detail.fingerprint`); `metadata` is a new sub-object keyed by field
  name. A row created with the release type above carries:

```json
{"type":"release","detail":{"metadata":{"channel":"beta","source_commit":"abc1234","signoffs":["a@x","b@x"]}}}
```

  Rationale (canon drift doctrine, BT-056): loose top-level metadata keys would
  re-create the ~230-distinct-keys drift the canon exists to stop. One
  container key keeps the row shape closed; the vocabulary inside it is
  board-local and evolves with types.jsonl.

## 3. Behavior spec

### 3.1 Template semantics — explicit flags ALWAYS win

Precedence rule (mirrors BT-071's priority rule): an explicitly passed flag
always beats the template; the template fills only what the caller omitted.

| template key       | applied when                                              | conflict behavior |
|--------------------|-----------------------------------------------------------|-------------------|
| `title_prefix`     | `--title` given AND does not already start with the prefix | if the title already starts with the prefix, it is NOT doubled; an explicit `--title` is never rewritten beyond prefixing |
| `ac_skeleton`      | `--acceptance-criteria` OMITTED (per DF-BOARDCTL-50, omitted = no key; the template makes the skeleton the value instead) | an explicit `--acceptance-criteria` replaces the skeleton entirely (skeleton is not appended to it) |
| `default_priority` | `--priority` omitted                                       | explicit `--priority` wins outright |

### 3.2 create — validation order and exact refusals

`create --type <slug>` runs these checks IN ORDER under the existing write
lock; every refusal exits 1 and writes NOTHING (no row, no event):

1. **Load the registry.** `types.jsonl` absent → built-ins. Present but
   malformed (bad JSON line, duplicate slug with equal `rev`, enum field with
   no `values`) → refuse the whole command:
   `create refused: types.jsonl line N: <reason> — fix the file and re-run (nothing written)`
2. **Unknown slug.**
   `create refused: unknown ticket type "shipit" — configured types: bug, chore, feat, release, task (run 'boardctl types' to list them)`
   (For a retired slug the message is: `create refused: ticket type "release" is retired in types.jsonl — pick an active type`)
3. **Template applied** (2.3 precedence).
4. **Required-metadata check** — field-by-field, ALL missing fields listed in
   one refusal (never fail-first), in types.jsonl declaration order:

```
create refused: type "release" requires metadata fields that are missing:
  - source_commit (string) — required: commit the release was cut from
  - channel (enum: stable|beta|nightly) — required: release channel this artifact ships to
supply each with --meta field=value (repeat the flag per field); nothing was written
```

5. **Value conformance.** An enum value outside `values`, or a `ref: task_id`
   value that matches no row id, is refused the same way, one line each.

`--meta` flag shape: repeatable `--meta key=value` (same `stringListFlag`
pattern as `--session` / `--evidence-artifact`; value flags join
`reorderArgs`). `--meta channel=stable` writes `detail.metadata.channel`.
`--meta` without `--type` on a board with a types.jsonl: allowed only for keys
declared on the row's own type — otherwise warn-and-write is NOT granted at
create (create is the filing gate; see AC-6).

Omitting `--type` entirely: writes no `type` key and runs NO type checks
(behavior identical to today). There is no implicit default type on boards
with a types.jsonl — being explicit is the point of the feature.

### 3.3 update

- **Removing a required metadata field is refused.** There is no `--clear-meta`
  in v1; the only way `update` can drop a required field is writing
  `--meta channel=` (empty). Empty value on a required field:
  `update refused: "channel" is REQUIRED by type "release" (enum: stable|beta|nightly) — supply a value or change the row's type first`
  Exit 1, nothing written.
- **Unknown metadata key → WARN, not error.** `--meta weirdkey=v` on a row
  whose type does not declare `weirdkey` succeeds, and prints to stderr:
  `warning: metadata key "weirdkey" is not declared by type "release" — it was written, but 'boardctl validate' will report it`
  (canon-drift doctrine: the write is tolerated, the drift is named and
  counted; validate lists it as a warning finding).
- Updating a metadata value that stays conformant is a plain rewrite, existing
  update discipline (write lock, event append, updated_at).

### 3.4 validate — type-conformance check

New check in `internal/board/validate.go`, running only when a valid
types.jsonl exists (built-ins have no required fields, so a registry-less
board adds zero findings):

- Row carries `type: X`, X unknown or retired → **error**:
  `BT-098: row <ID> claims type "shipit" which types.jsonl does not declare`
- X requires field F, F absent from `detail.metadata` (or `detail.metadata`
  absent entirely) → **error naming the field**:
  `BT-098: row <ID> (type "release") is missing required metadata field "channel" (enum: stable|beta|nightly)`
- F present with a value outside the enum → **error naming the field and value**.
- Unknown metadata key present → **warning** (never an error).
- Rows with NO `type` key → no finding (legacy tolerance; the fleet's 219 rows
  must stay validate-clean).

### 3.5 New verb: `types`

```
boardctl types [-C dir] [--json]
```

Read-only listing of the configured registry: for each active type, slug,
display, template summary (prefix / priority / skeleton present), and its
metadata fields as `name(type, required?, values…)`. Retired types are listed
in a trailing `retired:` block. Exit 0. This is a NEW verb and must be added to
both count-pinned surfaces (§5).

## 4. Acceptance Criteria

- **AC-1:** Given a board with no types.jsonl, when `create` runs without
  `--type`, then the written row is byte-identical to the pre-BT-098 writer —
  proof: golden-bytes test `TestBT098_ZeroMigration_Create` in
  `internal/board/` (T) + the AC-9 board-hash control (C).
- **AC-2:** Given a board with no types.jsonl, when `boardctl types` runs, then
  it lists exactly `bug, chore, feat, task` with no required metadata and exits
  0 — proof: `TestBT098_DefaultRegistry` (T).
- **AC-3:** Given the §2.4 release config on disk, when
  `create --type release --title "R1" --meta channel=beta --meta source_commit=abc`
  runs, then the row carries `type:"release"`, `priority:"P1"`, a
  `[RELEASE] `-prefixed title, the AC skeleton, and
  `detail.metadata.{channel,source_commit}` — proof:
  `TestBT098_CreateTemplateAndMetadata` (T).
- **AC-4:** Given the same config, when create omits `--meta channel` or
  `--meta source_commit`, then create exits 1 with the §3.2 field-by-field
  refusal naming BOTH missing fields and appends nothing to tasks.jsonl or
  events.jsonl — proof: `TestBT098_CreateRefusalFieldByField` (T).
- **AC-5:** Given an enum value outside `values` (`--meta channel=rc`), create
  exits 1 naming the field, the bad value, and the closed vocabulary — proof:
  `TestBT098_EnumConformance` (T).
- **AC-6:** Given a conformant release row, when `update --meta channel=` runs,
  then update exits 1 with the §3.3 required-field refusal and the row's
  metadata is unchanged on disk — proof: `TestBT098_UpdateProtectsRequired` (T).
- **AC-7:** Given a release row, when `update --meta undeclared_key=1` runs,
  then update exits 0, writes the key, and prints the §3.3 undeclared-key
  warning on stderr — proof: `TestBT098_UnknownMetaWarns` (T).
- **AC-8:** Given a release row missing `channel` in detail.metadata (hand-edited
  board), when `validate` runs, then it reports an error finding naming the
  field, and exit is 1 — proof: `TestBT098_ValidateConformance` (T).
- **AC-9 (anti-gaming / zero-migration control):** Given a copy of the live
  boardctl repo board, when every new command (`types`, `create` without
  `--type`, `update`, `validate`) runs against it, then
  `sha256(tasks.jsonl + events.jsonl + board.jsonl + fixtures.jsonl)` is
  unchanged before vs after — proof: `cmd/boardctl/bt098_zero_migration_test.go`
  hashes all four files around a full command battery (C).
- **AC-10:** Explicit flags beat templates: create with `--priority P3` and a
  title already beginning `[RELEASE] ` writes priority P3 and does NOT double
  the prefix — proof: `TestBT098_ExplicitFlagsWin` (T).
- **AC-11:** `boardctl types` on a registry with a retired slug lists the
  active types normally and the retired one only in the `retired:` block, and
  `create --type <retired>` refuses — proof: `TestBT098_RetiredType` (T).
- **AC-12 (count-pinned):** `internal/describe` reports the new `types` verb
  with a curated summary, and `skills/boardctl-usage/SKILL.md` names `types`
  in BOTH its Entry points and Proven commands lists — proof:
  `go test ./internal/speccheck/...` passes and `grep -c 'types' SKILL.md ≥ 2`
  in the skill's count-pinned test (T).
- **AC-13 (anti-gaming):** A green-but-did-nothing implementation that skips
  the template/metadata writer cannot pass AC-3 + AC-4 + AC-6: each asserts
  on-disk row bytes (not return values), including line-count invariance of
  tasks.jsonl after every refusal — proof: those three tests assert
  `rawLastLine`/file bytes directly (T).

## 5. Count-pinned surfaces (must be updated WITH the verb named)

1. `internal/describe/describe.go` — add `"types": "list configured ticket types + their required metadata fields"` to `cliSummaries`. The verb is derived automatically from `run()`'s switch once `case "types":` exists; `speccheck` fails until the summary is curated.
2. `skills/boardctl-usage/SKILL.md` — add `types` to the **Entry points** bullet list and add a proven-command block:
   `boardctl -C R types                # configured ticket types + required metadata fields`
   plus a note under the create flag list: `--type SLUG` / `--meta k=v` with the explicit-flags-win rule.

## 6. Edge Cases

- `types.jsonl` present but 0 bytes → treated as absent (built-ins), matching how empty tasks.jsonl is handled.
- Two lines with the same slug and equal `rev` → registry load error (§3.2 step 1); the operator disambiguates by appending a higher rev, never by editing history.
- `--meta channel=stable --meta channel=beta` (same key twice) → usage error before the board opens: `--meta "channel" given twice — one value per field`.
- Metadata value containing `=` or newlines: split on the FIRST `=`; value is free text, JSON-escaped per existing row discipline (one physical line).
- `ref: task_id` pointing at the row's own id, or at a fixture id → refused (a self-approval and a fixture are not approvals).
- Topology-B boards (header inside tasks.jsonl): types.jsonl lives in the same board dir; create/update already handle the header offset — no new topology code.
- `detail` already present with non-object shape on an update target → refuse: `row <ID>: detail is not a JSON object — refusing to attach metadata (fix by hand)`.
- Concurrent create/update under types.jsonl load: registry is read inside the existing write-lock window, same as row reads.

## 7. Non-Goals

- No database, no SQLite, no sidecar — JSONL stays canonical.
- No migration, backfill, or default `type` stamping of the 219 existing rows.
- No per-project or global (outside the board dir) type registry.
- No `--clear-meta` flag, no type editing via `update` (changing a row's type is a panel question, §9).
- No enforcement in `umbrella` beyond what per-board validate already reports (umbrella stays read-only).
- No new event types; type/metadata changes ride the existing `task_updated` event payload.
- No web UI or render changes.

## 8. Dependencies

- BT-097 (must land first; BT-098 stacks on its board-config groundwork).
- Existing: `internal/board` write lock, `Row`/`ParseRow`, `ValidateInitiativeSlug` slug grammar, `stringListFlag` pattern in `cmd/boardctl/main.go`, `speccheck` describe parity gate.
- Go toolchain only; no new modules.

## 9. Open Questions for the Panel

1. **Can a row change type after creation?** If `update --type` is allowed, must
   the new type's required metadata be satisfied at the moment of the change
   (refuse otherwise), and should the change append a distinct event
   (`task_type_changed`) for history? Recommendation: yes with full
   re-validation and a dedicated event; v1 may ship create-only and defer
   `update --type` to a follow-up if the panel prefers.
2. **What happens to history?** Type changes vs append-only doctrine: does the
   old type survive only in events.jsonl, or should `detail.metadata` carry a
   `type_history` array? (History in-row makes rows self-describing; events-only
   keeps rows lean.)
3. **Retired types with live rows:** hard validate error forever, or downgrade
   to a warning after a grace period?
4. **`ref` targets beyond `task_id`** (events, fixtures, external urls) — out of
   scope for v1; confirm the `ref` field is spelled so v2 can extend it.
5. **Should `list` gain `--type` filtering** and `stats` a by-type rollup now,
   or in a follow-up row?
6. **Title-prefix doubling rule:** refuse a `--title` that already contains the
   prefix in a non-prefix position, or only ever check the prefix case (as
   specified)? Spec assumes prefix-check only.

## 10. Test Plan

Runner: `go test ./...` (no new deps). New files:
`internal/board/bt098_types_test.go` (registry load, template precedence,
create/update/validate behavior against on-disk bytes),
`internal/board/bt098_zero_migration_test.go` (golden-bytes + AC-9 hash control),
`cmd/boardctl/bt098_cli_test.go` (verb wiring, exit codes, stderr warnings,
describe census). Fixtures: the §2.4 release config verbatim as a testdata
string so schema and tests cannot drift apart.

Falsifiers, per AC: every refusal test asserts (a) exit 1, (b) the exact
message text, (c) tasks.jsonl and events.jsonl line counts unchanged — the
triple that makes "refused" unforgeable. The zero-migration control (AC-9) is
the board-level falsifier for "built-in defaults change nothing": run the full
new-command battery on a copy of the live repo board and diff SHA-256 of all
four board files before/after.
