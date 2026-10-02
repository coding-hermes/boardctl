# UX: boardctl web UI — flows, states and interaction model

Status: DESIGN for BT-074. Derives from docs/web-ui-spec.md (BT-072, DESIGN
AUTHORITY) and docs/web-ui-design.md (BT-073, visual system); inherits
docs/specs/board-analytics-report.md §2 (data contracts), §5 (UX layers),
§6.2 (payload shapes), §6.3 (single-file constraints) verbatim. Where this
document, the design, and the spec disagree, BT-072 wins and this document
is amended (the design doc's derivation rule, restated).
Date: 2026-10-02
Refs: BT-072 (spec), BT-073 (design), BT-074 (this row), BT-075
(implementation), BT-021 (serve loopback posture), DF-BOARDCTL-2 (identity
dedupe post-fix), REVIEW-BOARDCTL-001 (sweep-status read census), BT-070
(priority vocabulary), BT-025 (status read aliases)

Derivation rule (binding on BT-075): every flow, state, and interaction
below answers a spec requirement (R1–R20), a named analytics-spec section,
or a BT-073 component/decision. This document adds NO endpoint, NO write
surface, NO external asset, and NO client-side derivation — it sequences
exactly what the spec authorizes. Section 7 maps R1–R20 to flows and
screens.

---

## 1. Posture the flows run under

- One self-contained HTML document at `GET /ui` (R1), served by the same
  serve process as the uploader (D1). Three regions top-to-bottom: shell
  header, notices band, content (design §1). The uploader stays at `/`; the
  UI links to it and never extends it (R2).
- Two views, hash-addressed, no path changes ever (analytics §5.4,
  design §7 anti-inventory "no tabs-with-routes"): the overview grid (the
  landing, R11) and the compare view (rendered only when ≥2 boards,
  design §1). A board page replaces the grid in place under
  `#b=<slug>`.
- Everything renders from a SNAPSHOT (R9, F5): the embedded island at
  load; `/api/` is the refresh mechanism, the island is the floor (R18).
  There is no live re-read, no watching, no auto-refresh (NG3). "The data
  changed" is answered by re-upload or serve restart — the flows in §5.5
  make that visible, never hidden.
- Read-only (D3, R20): zero write endpoints, zero mutating controls
  (§3). Loopback-only, no auth, no TLS, no cookies (D4, R3, R4) — so no
  flow below includes a login, a session, or a permission step, and
  "unreachable daemon" (E4) means the loopback process, nothing else.
- Provide-dont-force (D6, R5): every flow starts with a human starting
  serve and opening a browser. No flow is scheduled, hooked, or
  agent-triggered; nothing breaks when the UI is never opened.
- Untrusted data everywhere (R16, analytics §5.7): every state below is
  rendered with textContent/setAttribute-only components (design §7); the
  page never throws on bad input. This is a standing render rule for all
  screens and flows; edge E6 records its consequences.

### 1.1 Hash grammar (addressability, analytics §5.4 + §5.5, extended)

One document, hash-only state. The full grammar BT-075 implements:

| hash | renders |
|---|---|
| `` (empty) | overview grid (S0) |
| `#compare` | compare view (S5); if <2 boards loaded, overview + inline note |
| `#b=<slug>` | board page (S1) for slug |
| `#b=<slug>&t=<task-id>` | board page with that row expanded, scrolled to, FOCUSED |
| `#b=<slug>&q=<text>` | board page with the free-text filter applied (URL-encoded) |
| `#b=<slug>&s=<status>` | board page with the status filter applied (post-alias bucket name, including `(none)`) |
| `#b=<slug>&ev=<task-id>` | board page with the events timeline filtered to that task id |

Rules:

- Combinations compose (`#b=x&q=foo&s=blocked&t=BT-012` is valid).
- `q`/`s` reflection into the hash is analytics §5.5 (shareable filtered
  views). `t` and `ev` are BT-074 additions for deep links from TaskLinks
  and the row-detail "show all N" affordance. Sort state is NOT reflected
  (local presentation only).
- A row named by `t=` is ALWAYS rendered — expanded and focused — even
  when active `q`/`s` filters would exclude it; the count line appends
  "· 1 pinned by link". Uniform on load and on `hashchange`. This keeps
  every deep link honest (the link always lands) without destroying
  filter context.
- Unknown slug in `b=` → R8's JSON-404 semantics surface as the S1 error
  state. Known board, unknown `t=`/`ev=` id → the board renders normally
  plus one inline notice line at the top of the affected region: "task
  <id> is not in this snapshot" (analytics §5.7: no match messages are
  inline; the page never throws).
- Malformed hash (bad encoding, unrecognized params) → unrecognized
  params are ignored, the rest renders. Every hashchange is processed
  defensively; no hash value can blank the page.
- Browser Back/Forward walk this hash history — board switches, row
  traversal (F5), and filter reflections are all undoable with Back.
  This is load-bearing for the no-dead-ends rule.

## 2. Flow map

```
            human starts `boardctl serve` (loopback, D1/D4)
            human opens http://127.0.0.1:8787/ui  (R1, D6)
                                |
                                v
              +---------------------------------------+
              | S0 OVERVIEW  (#)                      |
              | one card per loaded board (R10/R11)  |
              +---+---------------+---------------+---+
        click card |        Compare tab |     footer/card link |
                   v (any count)       v (only >=2 boards)   v
        +---------------------+  +-----------+      +----------------+
        | S1 BOARD PAGE       |  | S5 COMPARE|      | S6 UPLOADER /  |
        | #b=<slug>           |  | #compare  |      | (existing, R2) |
        | header strip        |  +-----------+      +----------------+
        | toolbar             |
        | row table (S2 rows) |
        | validation  (S3)    |
        | analytics           |
        | events      (S4)    |
        +---------------------+
           |      |      |
           v      v      v
        expand  panel  timeline
        (S2)    (S3)   (S4)      every arrow has a return:
           ^      ^      ^      breadcrumb, tabs, Escape,
           |      |      |      browser Back over the hash
```

Flow inventory (each: trigger, path, states it can hit, exits):

| # | Flow | Trigger | Path | Exits |
|---|---|---|---|---|
| F1 | First run / land | human opens `/ui` | island parses → S0 renders immediately (R18 floor) | card click → F2; tabs; footer |
| F2 | Browse a board | card click or `#b=` | S0 → S1; island renders, `/api/board/{slug}` upgrade follows (design §6) | breadcrumb → S0; in-page regions; Back |
| F3 | Filter / search / sort | toolbar input | stays on S1; hash reflects `q=`/`s=` (§1.1) | clear filters; Back |
| F4 | Open a row | row click / Enter / Space, or `t=` deep link | S1 → S2 expansion in place (analytics §5.3; R12, no refetch) | same row toggles closed; Escape |
| F5 | depends_on traversal | TaskLink click in S2 | hash `t=` changes → target row expands + focuses; Back walks the chain | Back; breadcrumb |
| F6 | Inspect validation | scroll / pill click on card or strip | S3 panel (pill, findings, census, sweep census) | passive region — page nav exits |
| F7 | Events timeline | scroll, or TaskLink "show all N" from S2 | S4; `ev=` sets the task filter | clear filter; task links back into S2 |
| F8 | Compare | Compare tab (≥2 boards), `#compare` | S5 table + overlaid burn-up | tab back to Boards |
| F9 | Refresh / re-upload | header "refresh"; or footer "upload a board →" | §5.5: `/api/` refetch from island floor; upload leaves /ui to S6 | S0/S1 re-rendered; OfflineNotice on failure |
| F10 | Error recovery | any failure | inline ErrorState / OfflineNotice (never blank, R18) → "back to overview" or island-continued browsing | always ≥1 exit (§6) |

Dead-end audit: every screen's exits are enumerated in §4 and all of them
terminate at S0, another region of S1, or the browser's own Back — no
screen lacks a return path. The one surface outside this document's
control is the uploader page (S6): its way back is the browser's back
control or re-entering `/ui`, because R2 forbids changing that existing
page; stated here so BT-075 does not "fix" it.

## 3. The write path: out of scope, by citation

The board row asks for "the write/edit path IF the spec licenses one (with
confirmations + rollback)". It does not: **writes are out of scope and
this document specifies no write confirmations because there is no write
to confirm.**

- D3: "The UI adds ZERO write endpoints. Board files are never written
  (F10 is a hard invariant the UI inherits, not relaxes). validate and
  sweep-status are surfaced as findings/census only — their
  --repair/--apply/--decide write arms stay CLI-only."
- R20: beyond POST /, no handler creates, modifies, or deletes any file
  outside serve's temp roots.
- NG1 (no create/update/event/import), NG4 (no --repair), NG5 (no
  --apply/--decide) hard-exclude every write surface; NG7 excludes
  dedupe-board. The design's anti-inventory (§7) bans the controls:
  no forms beyond filters, no modals, and every action-looking control is
  a filter, expander, or link.
- The §6 capability table is the closed inventory: read-shaped verbs in,
  write/host-shaped verbs out. BT-075 implementing any confirmation
  dialog, mutation button, or "apply" affordance is wrong by NG1.

The two state-changing things a user can still do are both OUTSIDE the
UI's own surface, and the UI's only job is to be honest about them:

1. **Upload (POST /, unchanged, R2).** Reached via the footer link
   "upload a board →" (design D3 answer) — it LEAVES /ui. Confirmation
   and consequence live on the uploader page, which BT-021 owns. The UI
   never re-implements or wraps it. Rollback: none needed in /ui; the
   only state an upload changes is serve's in-memory session registry +
   temp trees (F10), and a re-upload of the same identity REPLACES the
   prior snapshot (F6) — which is the closest thing to "undo" and is
   visible as one entry with new counts, never as versions (R10).
2. **Restart serve (shell action, not a UI action).** Kills the registry
   and temp trees (F5, NG10: nothing persists). An open /ui tab notices
   only via failed fetches → E4's offline state. Recovery = start serve
   again and re-open /ui or hit refresh after restart (§5.5).

## 4. Screen inventory and per-screen states

Every screen lists empty / loading / error / success explicitly. Where a
state is structurally impossible, the table says so and why. "Success"
includes the degraded-but-usable island mode (R18) — it is marked
`success (degraded)` where it applies. Copy tone follows analytics §5.7
and design §6: honest, specific, never cute.

### S0 — Overview grid (`#`, the landing)

Purpose: every loaded board as a card — name, slug, topology badge,
task total / done / open, validate pill, link to the board (R11); one
card per identity, no versions UI (R10). Version string in the footer
(R11, §6 inventory). Compare tab visible only when ≥2 boards.

| state | rendering |
|---|---|
| success | the grid of cards, fields equal to /api/boards (R6). ≥1 board always (F2: the -C board exists from startup) |
| success (degraded) | /api/boards unreachable but the island parses → identical grid from the island + OfflineNotice banner (R18) |
| empty | zero boards — unreachable in production (F2) but reachable in tests: EmptyState "no boards loaded" (R11, analytics §5.7 tone) |
| loading | none on first paint (island is embedded, R18). Explicit refresh (F9): dimmed card skeletons, no shimmer (design §6) |
| error | island itself unparseable (broken build, not a data condition): boundary ErrorState — "the page could not render its snapshot", detail line, and exits: link to the uploader `/`, retry. Never a blank page (R18) |

Exits (no dead end): every card → S1; Compare tab (≥2 boards) → S5;
footer uploader link → S6; card pill → S1 validation panel.

### S1 — Board page (`#b=<slug>`)

Purpose: one board, top-to-bottom per design §2: header strip
(project, namespace, topology badge, ticks_total/ticks_idle — omitted
honestly when the board has no header, analytics §5.1 — last_commit),
toolbar, row table, validation panel, analytics, events timeline, and
the per-view footer strip: "snapshot loaded <loaded_at> · Refresh =
re-upload or restart serve" (R9's three loaded_at surfaces, design §9).

| state | rendering |
|---|---|
| success | all regions render from the island; the /api upgrade (design §6) reconciles numbers silently — loaded_at in the header meta updates if the server holds a newer snapshot |
| success (degraded) | /api/board/{slug} or /api/events/{slug} unreachable → full page from the island + OfflineNotice (R18) |
| empty | 0-task board (valid, analytics §2.2): EmptyState "no tasks yet" in the table; every chart its own empty state ("no completions yet", "no events"); key numbers render 0 honestly; streaks render `0 (no activity)` (§5.7) |
| loading | none on first paint (island). Refresh path: dimmed-row skeleton per affected region (design §6). Board switch: island-first, so no spinner — the /api upgrade is background |
| error | unknown slug (R8): ErrorState — "this board could not be loaded", the slug from the JSON error (escaped), "back to overview" affordance. Render throw anywhere: boundary ErrorState with the same exits. Never blank |

Exits: breadcrumb → S0; row expansion → S2; validation → S3; events →
S4; Compare tab; footer uploader link; Back.

### S2 — Row expansion (`#b=<slug>&t=<task-id>`)

Purpose: the full row in place — RowDetail per design §3: title + chips,
prose blocks (each omitted entirely when absent — no empty headers),
verdict chips, timestamps, depends_on/blocks TaskLinks, last 8 related
events + "show all N", raw JSON `<pre>` (the `boardctl show` parity
surface).

| state | rendering |
|---|---|
| success | RowDetail renders whatever the row carries; sparse rows show chips + raw JSON at minimum — an expansion is never visually empty by construction (design §3 block-omission rule) |
| empty | n/a — see success: absence of data omits blocks, never leaves a hole |
| loading | structurally none — expansion MUST work from the embedded payload without refetching (R12) |
| error | structurally none — a row cannot fail to expand; hostile strings render inert as text (R16, E6); unknown depends_on/blocks ids render as plain text (analytics §5.3) |

Exits: toggle closed (click / Enter / Escape on the focused row); any
TaskLink → F5; "show all N" → S4 filtered to the task; page nav.

### S3 — Validation panel (inside S1)

Purpose: "is this snapshot healthy?" — findings, not enforcement (R13,
D3). Pill PASS/WARN/FAIL with counts; findings list with verbatim
line-numbered messages (F9); the census line `key uniformity: N/M rows
carry only canon keys` quoted byte-for-byte; the sweep-status read
census (off-vocabulary counts, unknown statuses NAMED with their row
ids, and the explicit line "no decisions applied — CLI-only" — never
whitespace where that statement belongs, design §4).

| state | rendering |
|---|---|
| success | pill + findings + census + sweep census. FAIL: ≥1 error (pill red-bright). WARN: warnings only, 0 errors — the panel carries the documented note that WARN is validate's exit-0-with-warnings shape (R13 acceptance: "exit-equivalent 0 semantics documented in the panel") |
| empty | PASS with zero findings: findings list omitted (no empty headers); census and sweep census STILL render fully (counts 0, the explicit no-decisions line) — the panel's census is never absent |
| loading | structurally none — findings are computed once per snapshot load and ride in the read model (R13); a snapshot-time read, never a live re-check |
| error | n/a as a fetch state (payload-borne). A board WITH a validate ERROR is a success-state panel showing FAIL — the board stays fully browsable (E3). Unparseable board data surfaces as parse-warning findings + the notices banner (E8) |

Exits: passive region — all S1 exits apply. No repair/apply/decide
control exists anywhere (NG4/NG5); a user who needs to fix a finding
follows the panel's CLI pointer: the message text is verbatim what
`boardctl validate` prints, so the CLI command is copyable from the
finding itself.

### S4 — Events timeline (inside S1)

Purpose: chronological list (payload order = file order = append-only,
analytics §2.4), each item timestamp · event_type chip · actor · task
link · detail in the ladder's DECODED form (JSONPre for decoded JSON;
raw string as text on fall-through; hostile strings inert, R16).
Filters: event_type select (populated from the board's actual types,
legacy types stay visible) + task-id text filter; compose AND (R14,
design §5).

| state | rendering |
|---|---|
| success | the full list (batched per E2 past 500 items with "show 500 more") |
| empty | board with 0 events: EmptyState "no events" (§5.7). Filtered-to-zero (type + task + `ev=` compose): EmptyState "no match for the current filters" + "clear filters" affordance |
| loading | none from the island; refresh path dims the list (design §6) |
| error | structurally none as a render state — malformed timestamps leave the raw string shown and exclude the row from time metrics (§2.5, parse warning); malformed/hostile details render as text (R16). A failed events FETCH is S1's degraded state, not a separate error |

Exits: clear filters; TaskLink on any item → F5 (jumps into S2);
"show all N" inverse — the timeline IS the full list; page nav.

### S5 — Compare (`#compare`, ≥2 boards only)

Purpose: multi-board compare (R15, analytics §3.10/§5.6): selectable
boards (checkbox chips, default all), compare table + overlaid burn-up,
payload-driven.

| state | rendering |
|---|---|
| success | table of the selected boards; unchecking a chip removes its column; overlay redraws |
| empty | user unchecks everything: EmptyState "select at least one board" — chips remain usable, so this is an in-screen recovery, not a dead end |
| loading | refresh path only (dimmed regions, design §6) |
| error | a board whose data failed to load renders on S0 as an error-state card and is EXCLUDED from compare with a named note (analytics §5.7); the compare renders the remainder. All boards excluded → EmptyState naming the exclusions |

Entry guard: with <2 boards the tab is not rendered (design §1); a
`#compare` deep link in that case lands on S0 with an inline note.
Exits: Boards tab → S0; chip re-selection; Back.

### S6 — Uploader (`/`, existing surface — listed for boundary completeness)

NOT restyled, NOT extended (R2, D3): the UI links to it ("upload a board
→", footer + design §9) and the link leaves /ui. Its states are BT-021's
(upload → report; a zero-board upload still answers 200 with the -C
board per F2). After an upload the user is on the report page (a
separate static artifact); the way back to the UI is the browser's back
control or re-entering `/ui` — R2 forbids adding a return link to that
page, and this document records that as the deliberate boundary rather
than a dead end.

## 5. Interaction model

### 5.1 Browse

Card click → S1 via hash (`#b=<slug>`) — a real navigation the browser
records. Breadcrumb back. Tabs switch views. All of it works from the
island alone (R18); the /api upgrade (design §6: "board switch re-renders
from the island first, then upgrades from /api") reconciles numbers
without a loading state.

### 5.2 Filter, search, sort

- Free text: case-insensitive substring over `id`, `title`,
  `worker_summary`, `commit_hash` (analytics §5.5), debounced 150ms,
  applied on input. Status: select of the board's actual buckets
  (post-alias, including `(none)` and unknown pass-throughs) + "all".
  Fixtures toggle default OFF (hidden); ON shows fixture rows with the
  fixture chip and re-renders counts (stats --all semantics); the count
  line states fixtures hidden/shown honestly (design §2 correction).
- Filters compose AND; a zero-match renders the S1/S4 empty states with
  a "clear filters" affordance — never a blank region (§5.5/§5.7).
- Sort (BT-074 decision — the design explicitly leaves this open,
  design §2): a toolbar select with exactly File order (default) ·
  id A→Z · priority P0→P5 · priority P5→P0. Lexical only — no rank
  table is invented, mirroring BT-070's "lexical, never re-defined"
  rule. Presentation-order only over the SHOWN rows; never implies a
  payload change (R6); not hash-reflected.
- Batching rule (E2): with NO active filter/sort, rendering batches at
  500 rows/timeline items with "show 500 more" (threshold confirmed at
  design §2's 500). With any filter or sort active, ALL matches render —
  a filtered view never hides matches behind a batch button, and the
  count line ("N rows · M shown") proves it (aria-live=polite, design
  §10).

### 5.3 Open a row and traverse depends_on

- Expansion in place (analytics §5.3; R12) — the drawer variant is
  NOT adopted. Decision: inline-only, because R12 + §5.3 pin in-place
  expansion, the inline pattern needs no focus trap (design §10 leaves
  trap machinery only for a drawer), and one pattern keeps the keyboard
  model small. The drawer remains available to a future row; BT-075
  builds inline.
- TaskLink semantics (analytics §5.3): click scrolls to the target row
  and expands it; unknown ids render as plain text (no link). The hash
  `t=` updates on each hop, so Back walks the traversal chain in
  reverse (F5) — a cycle (a→b→a) is just two hops; expansion is
  idempotent, no recursion, cycle-safe by construction.
- No graph visualization: the payload carries per-row arrays only
  (§2.3), a laid-out graph would be a new client-side derivation
  (NG8, R6), and no such component exists in the design inventory
  (§7). The "depends_on graph" is traversed by hops — each hop is one
  user action with a visible target, which is also the honest answer
  for boards where arrays reference ids from other boards' namespaces
  (they render as plain text).
- The pinned-row rule (§1.1): a `t=` target is always shown, even
  against active filters; the count line appends "· 1 pinned by link".

### 5.4 Events linkage

RowDetail's related-events block shows the last 8 events by top-level
`task_id` match (§2.4) with "show all N" → scrolls focus to the
timeline and applies the task filter (`ev=<task-id>`, R14 client-side
filter). Timeline items link back into rows (F5). null task_id
(board_init) renders a muted "—" (design §5).

### 5.5 Refresh and re-upload (the snapshot-age answer, R9/R18/NG3)

The page shows a snapshot; three sub-cases of "the data moved" and what
the user does:

1. **Board files changed on disk after load.** Invisible and correctly
   so (R9: editing files MUST NOT change what /api or the UI shows).
   loaded_at + the footer line ("Refresh = re-upload or restart serve")
   make the age honest. A "refresh" click re-fetches /api — against the
   SAME snapshot, so numbers stay put; the flow's honest outcome is
   "unchanged — this tab shows the loaded snapshot; to see file changes,
   re-upload or restart serve". The offline banner copy carries the same
   sentence (design §6) so the semantics are learnable without docs.
2. **Another tab/session re-uploaded (identity replace, F6).** Refresh
   shows one entry with the new counts (R10); a board page for that
   slug re-renders with the new payload and loaded_at updates. No
   versions UI ever (R10).
3. **Serve restarted or died.** Restarted (same port): refresh succeeds
   against the new registry — new loaded_at, possibly different boards.
   Died: every fetch fails → OfflineNotice banner; the page stays fully
   browsable from the island (R18 floor) with the banner as the
   error-state marker. Recovery: restart serve, then refresh.

Refresh control: a "refresh" button in the header meta cluster
(design §1's server-meta cluster); skeleton states per design §6 (dimmed
blocks, 200ms fade, no shimmer); banners aria-live=polite (design §10).
No auto-refresh, no polling, no websocket (NG3) — refresh is always an
explicit human act.

### 5.6 Compare selection

Checkbox chips (design/analytics §5.6), default all. Unchecking removes
a board's column and overlay series; checking adds. Zero selected → the
S5 empty state with recovery in place. Selection is local (not
hash-reflected) — the view is exploratory, and `#compare` plus default-all
is the shareable state.

### 5.7 Keyboard model

Inherited baseline (analytics §5.4 + design §10 — must-not-change):

| key | where | action |
|---|---|---|
| Tab / Shift+Tab | anywhere | focus in visual order: skip link → header tabs → notices dismiss → board toolbar → row table → validation → analytics → events → footer; focus ring always visible (2px --accent, no suppression) |
| Enter / Space | collapsed row (button semantics, aria-expanded) | toggle expansion |
| Enter / Space | any control (tabs, filters, chips, batch button, banner dismiss, task links) | activate |
| Escape | focused expanded row | collapse that row |
| (deep link) | page load / hashchange | target row expanded + scrolled + FOCUSED (design §10 adds focus to the §5.4 scroll) |

BT-074 additions (the enhancement layer — small, suppressible):

| key | where | action |
|---|---|---|
| `/` | board page, focus NOT in an input/select/textarea | focus the toolbar search input |
| `e` | board page, same guard | expand all rows (current board's table) |
| `c` | board page, same guard | collapse all rows |
| Escape | focus inside the events filter controls with an active filter | clear the timeline filter |
| Back / Forward | browser chrome | walk the hash history (view switches, row hops, filter reflections) |

Explicitly NOT wired: arrow-key row navigation (Tab already reaches each
row; adding a second traversal model doubles the a11y surface for no
gain). All letter shortcuts no-op while focus is in any text entry (the
`/`-conflict guard), so typing a filter never triggers them.

## 6. Edge cases

Every case names its states and its exit — none is a dead end.

- **E1 — Empty board (0 tasks).** S1 empty state per §5.7: table
  EmptyState "no tasks yet"; every chart its empty line; numbers 0
  honestly; streaks `0 (no activity)`. A 0-task board still has a
  validation panel (census `0/0` shape), events (likely empty → S4
  empty), and full exits. F2's zero-boards overview is the S0 empty
  state.
- **E2 — Very large board (1000+ rows).** R19 keeps the guardrail
  payload-side (serve stderr note past ~5 MB); the UI's counterpart is
  the batch affordance: first paint renders 500 rows + "show 500 more"
  (repeatable), timeline identically. Expansion, deep links, and the
  pinned-row rule all work because the FULL payload is already on the
  page (R12: expansion without refetch) — batching is render-scheduling,
  never data-loading. Filters/sort lift batching entirely (§5.2).
  Analytics render from derived arrays whose length is the data window;
  the work clock is a fixed 7×24 grid. No virtualization, no lazy
  loading, no chart library (NG8).
- **E3 — Board with a validate ERROR.** Pill FAIL with counts; findings
  list with verbatim line numbers; census quoted; sweep census names
  off-vocabulary statuses with their row ids and the explicit
  "no decisions applied — CLI-only" line. The board remains FULLY
  browsable — validation is a panel, not a gate (R13, D3). Duplicate-id
  errors agree across surfaces: the row table renders the LAST row for
  a duplicated id (§2.7 fleet doctrine), the parse-warning banner counts
  the superseded row, and the finding names the error — one id, one row,
  honestly annotated. Repair paths are CLI-only (NG4/NG5/NG7); the
  finding text is exactly what the CLI prints, so the fix command is
  copyable.
- **E4 — Unreachable / offline daemon.** First paint never fails
  (island, R18). Subsequent fetch failures (refresh, /api upgrade,
  board switch) → OfflineNotice banner ("API unreachable — showing the
  snapshot embedded at load (loaded_at). Refresh = re-upload or restart
  serve.", design §6), page fully usable from the island, banner is the
  R18 error-state marker, role=status aria-live=polite. A hard error
  with no island (broken build) is S0's boundary ErrorState with two
  exits (retry, uploader link). Loopback means "daemon unreachable" is
  always "the process died or moved ports" — the banner's restart
  sentence is the complete recovery story.
- **E5 — Stale render.** The snapshot contract makes staleness a
  VISIBLE property, not a bug: loaded_at in three places (header meta
  freshest, board header strip line, board footer strip), the footer
  sentence, and the refresh flow's three sub-cases (§5.5). The UI never
  guesses freshness and never auto-refreshes (NG3); it labels the
  snapshot's age everywhere and gives the human the two levers
  (re-upload, restart).
- **E6 — Hostile strings (standing rule).** Titles, details, actors,
  statuses, ids — any payload string — render as text via
  textContent/setAttribute only; `</script>` cannot occur in the island
  (§6.3 escaping); no innerHTML anywhere (R16); the page never throws
  (§5.7). The mockups demonstrate the escaped `</script>` title as inert
  text (design §11); BT-075 inherits the test, not just the rule.
- **E7 — Unknown hash target.** Unknown slug → S1 ErrorState with the
  slug (R8's JSON 404 surfaced). Known board + unknown task id in `t=`
  or `ev=` → board renders + one inline "task <id> is not in this
  snapshot" line (§1.1). Malformed hash → params ignored, nearest valid
  state. Nothing blanks, nothing throws (§5.7).
- **E8 — Parse warnings.** Torn lines, duplicate ids, unparseable
  timestamps: counted per file in parse_warnings (§2.7), surfaced as the
  dismissible notices-band banner ("Loaded board `name` with N parse
  warnings") with the first 20 notes behind "show notes", plus the
  per-card parse-warning badge on S0 (analytics §5.1) that jumps to the
  banner. Zero warnings → no banner. Warned boards remain fully
  browsable — degraded load, not failed load (the tolerant reader never
  aborts on a bad line, §2.7).
- **E9 — Identity replacement mid-session (DF-BOARDCTL-2 post-fix).**
  Another tab re-uploads the same identity: this tab's island is stale
  until refresh; refresh shows ONE entry with the new counts (R10), no
  versions, no dupes. A board with a distinct topology is a distinct
  identity → a distinct card (F6). An open board page for a replaced
  identity re-renders to the new payload on refresh with loaded_at
  advancing — the only observable difference.

## 7. Requirement traceability — R1–R20 → flows and screens

| Req | Flows / screens / sections |
|---|---|
| R1 | F1; S0 (the /ui landing); §1 (route set stays closed, hash-only state, uploader stays at `/`); §1.1 (no path changes) |
| R2 | S6 boundary (§4, §2 dead-end audit); F9 (upload leaves /ui, never wrapped); §3 item 1 |
| R3 | §1 posture (every flow is loopback-local); E4 (daemon = loopback process); footer posture surface |
| R4 | §1 posture — no flow includes login/session/permission; no auth affordances exist to flow through |
| R5 | F1 (a human starts serve and opens a browser — the only entry); §1 posture |
| R6 | §5.2 sort (presentation-only, no derivation); §4 all screens render payload values verbatim; E2 (batching is render-scheduling) |
| R7 | §2 map data sources: S0 ← /api/boards; S1/S2/S3/S5 ← /api/board/{slug}; S4 ← /api/events/{slug}; F9 refresh scope |
| R8 | S1 error state; E7 (unknown slug → JSON-404 semantics, ErrorState with slug) |
| R9 | S1 header strip + footer strip; §5.5 (snapshot-age honesty); E5 |
| R10 | S0 one card per identity; E9; §5.5 case 2; no versions UI anywhere |
| R11 | S0 all four states; footer version string |
| R12 | S1 structure; S2 expansion (inline, island-driven, no refetch); §5.3; E2 batching |
| R13 | S3 all states; E3; §3 (no repair/apply/decide surfaces) |
| R14 | S4 all states; F7; §5.4 linkage; §1.1 `ev=` |
| R15 | S5 compare; S1 analytics region; §5.6 selection model |
| R16 | E6 standing rule; §1 posture; every state table's render rule (textContent/setAttribute, never throw) |
| R17 | F9 (refresh fetches same-origin /api/ only); §1 posture — no flow references any external resource |
| R18 | Every screen's `success (degraded)` rows; E4 island floor; S0/S1 error states (never blank); §5.5 |
| R19 | E2 (batch affordance as the UI-side counterpart; the guardrail note itself is serve stderr) |
| R20 | §3 (writes out of scope, by citation; no confirmation flows because no writes) |

Decisions D1–D6 are inherited and bind flows where cited: D1/D4 (§1
posture, E4), D2 (S5 entry guard, one-board-fleet renders fully), D3/D6
(§3, F1, S6), D5 (the §6 capability table — every YES row has a screen:
list/show→S2, stats/header→S1 strip + S0 cards, render→S1 analytics +
S5, validate/sweep read→S3, events→S4, version→footer; every NO row has
a named exclusion in §3).

## 8. What BT-075 inherits from this document

- The hash grammar (§1.1) including the pinned-row rule and the
  defensive-hashchange obligation.
- The flow map's screen set (S0–S6) with their state tables — the
  acceptance shape "every screen has its states; no dead ends" is
  testable per table row.
- The interaction decisions left open by BT-073 §12, now decided:
  inline expansion (drawer NOT adopted); sort = the four-option lexical
  select (not hash-reflected); batch threshold = 500, lifted under any
  filter/sort; compare selection = checkbox chips, default all, local
  state; refresh = explicit header button + island-first board switches.
- The keyboard model (§5.7): the inherited baseline unchanged, plus the
  small enhancement layer with its input-focus guard.
- The no-dead-ends audit (§2) and the S6 boundary (the uploader page
  gains no return link — R2).
- Everything NOT decided here stays where BT-073 pinned it: tokens,
  components, a11y baseline, and the must-not-change list (design §12);
  any write surface, endpoint, external asset, auth affordance, or
  client-side derivation remains wrong without a BT-072 amendment.
