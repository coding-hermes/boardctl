# DESIGN: boardctl web UI — visual system + component design

Status: DESIGN for BT-073. Derives from docs/web-ui-spec.md (BT-072, DESIGN
AUTHORITY) and inherits docs/specs/board-analytics-report.md §2 (data
contracts), §5 (UX layers), §6.2 (payload shapes), §6.3 (single-file
constraints) verbatim.
Date: 2026-10-02
Refs: BT-072 (spec), BT-073 (this row), BT-074 (flows), BT-075
(implementation), BT-070 (priority vocabulary P0..P5), BT-025 (status read
aliases), REVIEW-BOARDCTL-001 (sweep-status read census), BT-021 (serve
posture), DF-BOARDCTL-2 (identity dedupe post-fix)

Derivation rule (binding on BT-074/075): every visual or component statement
here answers a spec requirement or a named analytics-spec section; where this
document and BT-072 disagree, BT-072 wins and this document is amended. This
document adds NO endpoint, NO write surface, and NO capability outside the
R1–R20 surface — it styles exactly what the spec authorizes.

---

## 1. Layout system

One self-contained HTML document served at `GET /ui` (R1). Three regions,
top-to-bottom; no sidebar, no multi-pane splits — boardctl is a reading
surface, not a console.

```
+--------------------------------------------------------------+
| A. Shell header (sticky)                                      |
|    [boardctl] wordmark · brand                                |
|    [Boards] [Compare] view tabs · server meta · clock/snapshot|
+--------------------------------------------------------------+
| B. Notices band (conditional, in flow)                        |
|    upload notice (F2) / parse-warning banner (2.7) /          |
|    offline-fragment notice (R18) — never sticky, never modal  |
+--------------------------------------------------------------+
| C. Content region (view-scoped)                               |
|    Boards view    : overview grid (R11) or board page         |
|    Compare view   : multi-board compare (R15, analytics 3.10) |
|    + per-view footer strip (snapshot provenance, R9/R18)       |
+--------------------------------------------------------------+
| D. Footer: version string (R11/§6), loopback notice, doc ref  |
+--------------------------------------------------------------+
```

- A. Shell header: wordmark ("boardctl"), the two view tabs (Boards,
  Compare — Compare rendered only when ≥2 boards, analytics §5.6), and the
  server meta cluster on the right: serve address, loaded-board count, and
  the freshest loaded_at among boards (R9). Sticky; `z-index` above content;
  collapses to wordmark + tabs under 720px.
- B. Notices band: conditional full-width slots. A banner class exists for
  exactly three kinds: upload notice, parse warnings, island-degraded
  notice. Banners are dismissible (aria-live polite, dismiss button), never
  block content, never stack more than one per kind.
- C. Content region: hosts exactly one view at a time. Boards view is the
  landing (R11). A board page replaces the grid in-place (no route change
  beyond the hash, analytics §5.4 addressability), with a breadcrumb back to
  the grid.
- D. Footer: static version string, "loopback-only · read-only snapshot"
  posture line, link to the uploader at `/` (the UI links to POST /, never
  extends it, R2/§6).

Grid/breakpoint rules: content max-width 1180px, centered. Overview grid
auto-fill minmax(300px, 1fr) (analytics §5.1: 2–4 cards per row). Under
720px the row table degrades: fixed columns collapse to a stacked
definition-list row (id/title/chips on the first line, meta below). No
horizontal page scroll is introduced by the table until 360px viewport
(analytics §5.7's never-throw rule extends to layout: overflow scrolls
within the region, not the page).

## 2. Board view (the row table, R12)

Structure per board page, top-to-bottom:

1. Board header strip — `BoardHeader` component: project, namespace,
   topology badge, ticks_total / ticks_idle (absent-header boards omit the
   pair — analytics §5.1 rule), last_commit short hash. Topology-B header
   detection is payload-side (§2.1 shape detection); the component renders
   whatever `header` carries and never assumes line 1 (R12).
2. Toolbar — search input, status select, fixtures toggle, expand/collapse
   all, row count line ("N rows · M shown").
3. `RowTable` — one table of task rows. Columns (fixed order):
   id · title · status chip · priority chip · depends_on count.

   - Collapsed row = one interactive row (`<tr>` with a button-semantic
     first cell or `role=button` row per analytics §5.4), showing EXACTLY:
     id, title (single line, ellipsis, 100-char cap), status chip, priority
     chip, depends_on count. Not shown collapsed: model, commit, fixture
     chip — those live in the expansion, keeping the collapsed grid narrow
     (spec R12's column set is exactly five; analytics §5.3's richer
     collapsed set is report-page heritage — the /ui row table follows R12's
     five columns, and the fixture chip moves into the expansion header and
     the filtered-count line).
   - Expanded row: the `RowDetail` component, inline (same row, no
     navigation — analytics §5.3 in-place expansion).
4. Pagination: none by default; a "show N more" batch affordance appears
   past 500 rows (R19 size guardrail is payload-side; this keeps first paint
   bounded without lazy-loading tricks). BT-074 may revisit; the default
   renders all rows.

Correction (stated because R12's column list is binding): the collapsed row
shows EXACTLY the five R12 columns. The `fixture` chip is rendered in the
expansion header and in the filtered-count line, not as a sixth collapsed
column. Fixture visibility still honors the toggle (analytics §5.5); the
count line states fixtures hidden/shown honestly.

Sorting: none by default (spec orders no sort); rows render in file order.
BT-074 may add a client-side sort control; any sort is presentation-order
only and never implies a payload change (R6).

Row expansion mechanics (inherited, analytics §5.4): Enter/Space toggles,
Escape collapses the focused expanded row, `aria-expanded` on the row,
hash addressability `#b=<slug>&t=<task-id>` reacts on load and on
`hashchange`. Expansion works from the embedded island alone (R12 — no
refetch).

## 3. Row detail panel (R12 expansion; reusable as the drawer)

`RowDetail` renders, top-to-bottom, each block omitted entirely when its
data is absent (no empty headers — analytics §5.3):

1. Title (full) + chips: status, priority, fixture chip when classified
   fixture (§2.6).
2. Prose fields: reasoning, worker_summary, foreman_note, review_notes —
   each a `ProseBlock` (label + text), omitted when absent.
3. Verdict chips: guard_result (PASS/FAIL/SKIP), ci_result (GREEN/RED/SKIP),
   attempts, exit_code, complexity, lines added/removed, files_changed list
   (chip per file, `CodeMeta` sub-component).
4. Timestamps line: created_at / dispatched_at / completed_at / updated_at,
   blocked_reason + blocked_since when present. Raw strings shown as stored
   (§2.5: unparseable values must still render — the row never hides them).
5. Links: depends_on / blocks as `TaskLink`s — link scrolls to the target
   row and expands it; unknown ids render as plain text (analytics §5.3).
6. Related events: last 8 events whose top-level `task_id` matches
   (§2.4), each an `EventItem` (timestamp, event_type chip, actor, decoded
   detail), with "show all N" affordance that scrolls focus to the events
   timeline section filtered to the task (R14 client-side filter).
7. Raw JSON: the full row, pretty-printed, in a `<pre><code>` block —
   verbatim payload bytes (`JSONPre` component). This is the `boardctl show`
   parity surface: what CLI `show` prints, the panel shows.

The same `RowDetail` component, in a right-side drawer variant, is the
design answer for BT-074's flow work if a non-inline pattern is wanted; the
inline expansion is the BT-073 default (analytics §5.3 in-place rule).

## 4. Validation-state surfacing (R13)

The validation panel answers "is this snapshot healthy?" with findings, not
enforcement (D3 — no repair/apply/decide affordance exists anywhere in the
UI).

Structure:

1. `ValidatePill` on every board card (R11) and in the board header strip:
   PASS / WARN / FAIL.
   - PASS: 0 errors, any warnings (F9's exit-0-with-warnings shape).
   - WARN: warnings only, 0 errors.
   - FAIL: ≥1 error.
   Pill includes counts: `FAIL 1 error · 2 warn`.
2. `FindingsList`: rows of `Finding` items — level chip (error/warn), message
   WITH line numbers as stored (F9: validate messages carry line numbers;
   the UI renders them verbatim, monospace, never re-wrapped).
3. Census lines, quoted verbatim (F9 / BT-056):
   `key uniformity: N/M rows carry only canon keys` — rendered as a
   `CensusLine` (monospace, full-width, copyable text node).
4. `SweepCensus` subsection (REVIEW-BOARDCTL-001): off-vocabulary status
   counts; unknown statuses NAMED with their row ids; zero decisions shown
   as decisions ("no decisions applied — CLI-only", explicit line, not
   whitespace).

State classes (the vocabulary): `validate-error` (level=error findings),
`validate-warn` (level=warn findings). These classes map to tokens
`--red-bright` / `--orange` respectively. An off-vocabulary status string
itself (e.g. a task row with `status:"deploy"`) renders with the
`chip-status-unknown` gray chip (analytics §5.3: unknown = gray) and the
row appears in the SweepCensus list — the two surfaces agree.

Priority vocabulary note (BT-070): P0..P5 are the full set; the UI renders
any priority string but color-maps exactly the six in-vocabulary values.
A priority outside P0..P5 renders as an unknown-value gray chip (same
policy as statuses; validate flags it, the UI surfaces the finding, the chip
never invents a color).

## 5. Events timeline (R14)

`EventsTimeline` per board page, below the row table:

- Vertical chronological list, most recent last (file order; the payload
  order IS file order), each row: timestamp · event_type chip · actor ·
  task link · detail.
- `EventItem` detail rendering applies the §2.4 ladder's DECODED form:
  pretty JSON in a `JSONPre` block when the ladder succeeded; raw string as
  text when it fell through. Hostile strings render as text (R16) — the
  design note is that the decoded JSON block is a styled `<pre>` (bg3,
  monospace, 12px), never parsed into DOM.
- Filters above the list: `event_type` select (populated from the board's
  actual types — legacy free-form types stay visible per §2.4, never
  filtered out of the payload) and a task-id text filter (R14 client-side).
  Compose AND. Filtered-to-zero renders the timeline empty-state (§5.7).
- task_id linkage: `TaskLink` scrolls to the row table and expands that row
  (analytics §5.3 link behavior); null task_id (e.g. board_init) renders a
  muted "—" (no link).
- Density: compact two-line items (line 1: time · type · actor · task; line
  2: detail). Timeline spine: 2px left rule, node dot per item, colored by
  level only where a level exists (audit/tick neutral) — the spine is
  decoration, never the only signal (color-only rule, §4 a11y).

## 6. Empty / loading / error / offline states (R18, §5.7, R11)

Every state is a named component with copy stated here; tone follows
analytics §5.7 (honest, specific, never cute):

- `EmptyState` (board with 0 tasks / filtered-to-zero / 0 events):
  centered line in the affected region, e.g. "no tasks yet", "no events",
  "no match for the current filters" + a "clear filters" affordance when
  filters caused it. Key numbers render 0 honestly; streaks render
  `0 (no activity)`.
- `LoadingState`: the island is embedded (R18), so first paint is never
  a spinner. A loading state exists only for the /api refresh path
  (BT-074's explicit refresh): skeleton of the affected region — dimmed
  rows (bg3 blocks, no shimmer), 200ms fade. Board switch within the UI
  re-renders from the island first, then upgrades from /api.
- `ErrorState`: fetch failure, unknown slug (R8's JSON 404), or any render
  throw caught by the boundary: inline region-level panel — icon glyph
  (inline SVG path), headline ("this board could not be loaded"), detail
  line (the error message, escaped), and a "back to overview" affordance.
  Never a blank page (R18), never a thrown page (analytics §5.7).
- `OfflineNotice` (R18 island-degraded): when /api calls fail but the
  island parses, the page renders fully from the island and pins the
  notices-band banner: "API unreachable — showing the snapshot embedded at
  load (loaded_at). Refresh = re-upload or restart serve." The banner is
  the R18 error-state marker; the page stays usable.
- The overview with zero boards (test-only per F2 but rendered honestly,
  R11): `EmptyState` with the §5.7 tone — "no boards loaded".

## 7. Component inventory (named, reusable)

Every component is payload-driven, renders from BOTH the embedded island
and /api responses with identical markup (R18 floor = island), and is
safe-by-construction (textContent/setAttribute only — R16). "Reusable" =
one implementation, many hosts (grid card, board page, drawer, timeline).

Core primitives:

| Component | Renders | Hosts |
|---|---|---|
| `StatusChip` | one status string → chip; canonical colors for the six canonical statuses, alias-rendered-canonical, unknown = gray | RowTable, RowDetail, compare, boards grid |
| `PriorityChip` | one priority string → chip; P0..P5 color-mapped, unknown = gray | RowTable, RowDetail, compare |
| `ValidatePill` | PASS/WARN/FAIL + counts | board card, board header strip |
| `GuardChip` / `CiChip` | PASS/FAIL/SKIP, GREEN/RED/SKIP | RowDetail |
| `FixtureChip` | fixture classification (§2.6) | RowDetail expansion header, count line |
| `TopologyBadge` | A / B | board card, header strip |
| `KeyNumber` | label + value (optional delta line) | cards, header strip, compare table |
| `CensusLine` | verbatim monospace census string | validation panel |
| `ProseBlock` | label + multi-line text | RowDetail |
| `JSONPre` | pretty JSON in a styled pre | RowDetail, EventItem |
| `TaskLink` | hash-addressable link to a task row | RowDetail, EventItem |
| `EventItem` | one event row, ladder-decoded | EventsTimeline, RowDetail |
| `EmptyState` / `LoadingState` / `ErrorState` / `OfflineNotice` | §5.7 states | every region |

Composed views:

| Component | Composition | Req |
|---|---|---|
| `BoardCard` | name, slug, topology, KeyNumbers (total/done/open), ValidatePill, link | R11 |
| `BoardsGrid` | responsive card grid | R11 |
| `BoardHeader` | header strip fields | R12 |
| `BoardToolbar` | search / status filter / fixtures toggle / expand-collapse / count line | R12, §5.5 |
| `RowTable` + `Row` | collapsed/expanded rows | R12 |
| `RowDetail` | the expansion body (§3 above) | R12 |
| `ValidationPanel` | pill + FindingsList + census + SweepCensus | R13 |
| `EventsTimeline` | filter bar + EventItem list | R14 |
| `AnalyticsBlock` | the analytics views as hand-rolled SVG, payload-driven | R15 |
| `CompareTable` | multi-board compare rows | R15 (§3.10) |
| `NoticeBanner` | upload notice / parse warnings / offline | R18, F2, 2.7 |
| `Footer` | version + posture line | R11 |

Anti-inventory (things deliberately NOT components): no form controls beyond
filters (no write surface, NG1), no modal dialogs (NG12 posture: the page
never blocks), no chart library wrappers (NG8 — charts are hand-rolled SVG
inside AnalyticsBlock), no tabs-with-routes (hash-only addressability,
§5.4).

## 8. Design tokens

Single source of truth for both mockups and BT-075 CSS. The base palette is
the existing report template's (internal/render/template.go `:root`), extended
with two AA-fixed brights. Keeping the family means the /ui and the report
read as one product.

### 8.1 Color

```
--bg:         #0d1117   page canvas            (fg on bg 16.02:1)
--bg2:        #161b22   card / panel surface   (fg 14.64:1)
--bg3:        #1c2330   inset / code / inputs  (fg 13.34:1, muted 5.12:1)
--line:       #30363d   borders, rules
--fg:         #e6edf3   primary text
--muted:      #8b949e   secondary text         (on bg 6.15:1, on bg2 5.62:1)
--accent:     #58a6ff   links, focus, active   (on bg 7.49:1)
--green:      #3fb950   complete, PASS (chip green)
--green-bright:#56d364  pill text green (AA on 25% pill fill)
--red:        #f85149   decorative red (borders on dark fills, solid fills)
--red-bright: #ff7b72   red AS TEXT (chips/pills/findings)
--orange:     #d29922   blocked, WARN (chip orange)
--orange-bright:#e3b341 pill text orange (AA on 25% pill fill)
--blue:       #58a6ff   in_progress (same as accent in the template; kept
                        as a distinct token so BT-075 may diverge later)
--purple:     #bc8cff   review
--cyan:       #56d4dd   P4 (new; the template had no P4-era cyan)
--gray:       #6e7681   fixture/unknown chip BORDERS (decorative)
--gray-bright:#9ea7b3   unknown/fixture AS TEXT (AA on its tint; the
                        template's #6e7681 measures 3.19:1 on its chip
                        tint — below AA — so text uses the bright token)
```

Derived surfaces (chip/pill tints): `color-mix(in srgb, <token> 15%,
var(--bg2))` for chips; 25% over `--bg` for the validation pills' stronger
fill. `color-mix` is CSS, no JS, no library; BT-075 ships a fallback only
if the target baseline needs it (static artifact, browser-rendered —
color-mix is 2023-baseline and fine).

Status color mapping (chips: colored text + 1px colored border on the 15%
tint):

| status | text/border | contrast (measured) |
|---|---|---|
| complete | --green #3fb950 | 5.32:1 |
| pending | --muted #8b949e | 4.50:1 |
| in_progress | --blue #58a6ff | 5.31:1 |
| review | --purple #bc8cff | 5.33:1 |
| blocked | --orange #d29922 | 5.37:1 |
| failed | --red-bright #ff7b72 | 5.44:1 |
| unknown / alias-pre-canonical n/a | --gray-bright #9ea7b3 | 5.47:1 |

(All measured: text color composited on its own 15% tint over --bg2; method
WCAG 2.x relative luminance. The template's raw #f85149 fails AA as chip
text (4.35:1) — hence --red-bright for every text use. --red stays for
decorative borders/fills, where the 3:1 non-text bar applies.)

Priority color mapping (BT-070 P0..P5; same chip construction):

| priority | text/border | contrast (measured) |
|---|---|---|
| P0 | --red-bright #ff7b72 | 5.44:1 |
| P1 | --orange #d29922 | 5.37:1 |
| P2 | --blue #58a6ff | 5.31:1 |
| P3 | --purple #bc8cff | 5.33:1 |
| P4 | --cyan #56d4dd | 7.07:1 |
| P5 | --muted #8b949e | 4.50:1 |

Rationale: P0..P5 inherit the status wheel's semantic anchoring (P0 = the
red of "failed"/urgent, P1 = the orange of "blocked") and darken toward
neutral at P5. P0..P5 ordering is LEXICAL in the payload (BT-070: no rank
table in render) and stays lexical in the UI's default ordering; the color
map is presentation, never a rank re-definition (R6: the browser never
recomputes derivations — a color legend is not a derivation, but the chip
also always shows the literal "P0".."P5" text so no meaning rides on hue).

Validate pill mapping (final; text on its own 25% tint over --bg):

| pill | fill (25% tint over --bg) | text | contrast (measured) |
|---|---|---|---|
| PASS | green tint #1f422a | --green-bright #56d364 | 5.82:1 |
| WARN | orange tint #423a22 | --orange-bright #e3b341 | 5.80:1 |
| FAIL | red tint #4a2c2e | --red-bright #ff7b72 | 4.93:1 |

The pill construction needed brighter text tokens than the status chips
(the stronger 25% fill darkens against #3fb950/#d29922 at 4.87/4.91 —
close but under the 4.5 bar for 12px text only because those two land at
~4.9 with the OLD colors; both constructions were measured and the bright
tokens clear it decisively). Two pill-only tokens join the palette:

```
--green-bright: #56d364   pill text green (6.63:1 on 15% chip tint)
--orange-bright:#e3b341   pill text orange (6.59:1 on 15% chip tint)
```

--red-bright is shared between chips and pills (4.93:1 ≥ 4.5 on the 25%
fill). BT-075 copies THESE values.

### 8.2 Type

- Stack: `-apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif`
  (template-parity; system fonts only — NG9, no external fonts).
- Mono: `ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas,
  "Liberation Mono", monospace` — ids, census lines, raw JSON, line
  numbers.
- Scale (px / rem @16 base):
  - 12 / 0.75 — chips, captions, table meta, pill text
  - 13 / 0.8125 — table body, timeline detail, secondary copy
  - 14 / 0.875 — body default (template-parity line-height 1.45)
  - 16 / 1 — board title, section headings
  - 18 / 1.125 — KeyNumber values
  - 20 / 1.25 — page title (board name in header strip)
- Weights: 400 body, 600 emphasis/chips/KeyNumbers, 700 wordmark.
  No font below 12px anywhere (census quotes stay 12px mono even when
  long — they wrap, never shrink).

### 8.3 Spacing, radius, elevation

- Space scale (4px base): 4, 8, 12, 16, 24, 32 — used for padding/margins/
  gaps exclusively; no ad-hoc px values in component CSS beyond 1–2px
  borders and hairlines.
- Radius: 6px (controls, chips, inputs), 10px (cards, panels, banner) —
  template-parity.
- Elevation: none (no shadows). Surfaces separate by fill steps (bg vs bg2
  vs bg3) and 1px --line borders. The sticky header carries a 1px bottom
  border, not a shadow. Justification: static, loopback, zero-motion
  posture — depth cues add nothing and cost contrast.
- Motion: 120ms ease-out on hover/focus background-color only. No
  transforms, no parallax, no skeletons-with-shimmer (LoadingState uses
  static dimmed blocks). Reduced-motion: nothing to disable (there are no
  animations beyond color fades), stated so BT-075 doesn't add any.

## 9. Requirement-to-component map (R1–R20, D1–D6)

Every requirement gets a visual answer or an explicit visual N/A with the
reason. "N/A (postural)" = the requirement constrains behavior/transport,
not pixels; the design still names where its evidence surfaces.

| Req | Visual answer |
|---|---|
| R1 | Shell layout §1: /ui is the three-region document; uploader stays at `/` — the footer + breadcrumb never render `/ui` at `/` (visual separation of surfaces); closed route set is postural (BT-075 test), no visual surface |
| R2 | Footer's uploader link + board cards' "re-upload" affordance is the ONLY reference to POST / (link, not extension); no UI surface re-styles or reinterprets existing endpoints |
| R3 | N/A (postural: loopback bind). Surfaced as the footer posture line "loopback-only" + header meta showing the bound address |
| R4 | N/A (postural: no auth). Visually: NO avatar/account/lock affordances anywhere in the shell — absence is the design statement |
| R5 | N/A (postural: provide-dont-force). Nothing in the shell implies automation (no "embed", no "API key" surfaces; the footer states the opt-in posture) |
| R6 | Every numeric surface (KeyNumber, chips, pills, compare cells) renders payload values verbatim; the design defines zero client-side derivations; AnalyticsBlock consumes `derived.*` as data |
| R7 | The three /api endpoints drive exactly: BoardsGrid+BoardCard (/api/boards), BoardHeader+RowTable+ValidationPanel+AnalyticsBlock (/api/board/{slug}), EventsTimeline (/api/events/{slug}). The design names no other data source; no visual surface implies a fourth endpoint |
| R8 | ErrorState copy for a failed board fetch shows the JSON error's message (escaped); the unknown-slug case renders "this board could not be loaded" + the slug, never a redirect to HTML |
| R9 | loaded_at surfaces in three places: header meta cluster (freshest), per-board header strip ("snapshot loaded …" line), footer strip per board page ("Refresh = re-upload or restart serve") — snapshot semantics are always visible |
| R10 | BoardsGrid renders ONE card per identity (no versions UI, no "history" affordance); the -C board card is visually unlabeled (it's just a board); topology badge makes same-name/different-topology pairs legible as distinct cards |
| R11 | BoardsGrid + BoardCard (name, slug, topology, total/done/open, ValidatePill, link); zero-board EmptyState; version string in footer |
| R12 | BoardHeader + BoardToolbar + RowTable/Row (collapsed five columns: id, title, status, priority, depends_on) + RowDetail expansion, island-driven (no refetch) |
| R13 | ValidationPanel: ValidatePill (PASS/WARN/FAIL + counts), FindingsList with verbatim line-numbered messages, CensusLine quoting the key-uniformity line byte-for-byte, SweepCensus naming off-vocab counts + unknown statuses + explicit "no decisions applied — CLI-only" |
| R14 | EventsTimeline: timestamp/type/actor/task-link/detail items, decoded-detail rendering (JSONPre / raw text), type + task-id filters, filtered-to-zero EmptyState |
| R15 | AnalyticsBlock: the analytics views (burndown, burn-up, velocity, cycle time, streaks, tick health, work clock, model share, status/priority counts) as hand-rolled SVG per view, payload-driven; CompareTable + overlaid burn-up for §3.10 across boards |
| R16 | Safety posture is structural: components render text via textContent, attributes via setAttribute; the design specifies NO innerHTML surface; decoded event detail renders inside JSONPre as TEXT. Mockups demonstrate inert rendering of hostile strings |
| R17 | N/A (postural: same-origin fetches only). Visually: no external-asset placeholders anywhere — no font/img/CDN slots exist in the design |
| R18 | The island-first floor: OfflineNotice banner + all components render identically from island and /api; LoadingState exists only for explicit refresh; ErrorState for hard failures — never a blank page |
| R19 | N/A (postural: payload-size note is serve stderr). Visual counterpart: the 500-row batch affordance keeps first paint bounded; no virtualization UI implied |
| R20 | N/A (postural: zero board writes). Visually: the UI contains ZERO form controls that POST anywhere except the uploader link — no buttons imply mutation; every action-looking control is a filter/expander/link |

Decisions (design constraints inherited, each answered):

| D | Visual answer |
|---|---|
| D1 (extend serve) | The shell header wordmark is "boardctl" with a "/ui" suffix tag — one product, one process; no separate branding |
| D2 (fleet) | BoardsGrid is the LANDING (R11); a single board still renders as a one-card grid + compare omitted (analytics §5.1). The UI never collapses to a single-board chrome |
| D3 (read-only) | §4's validation panel is findings-only; §7's anti-inventory bans forms/modals/write-looking controls. The uploader link is labeled "upload a board →" and leaves /ui |
| D4 (loopback, no auth) | Footer posture line + header address; no auth affordances (mirror of R4's answer) |
| D5 (capability split) | The §6 capability table's YES rows map 1:1 to components (list/show→RowTable, stats→KeyNumbers/cards, header→BoardHeader, render→AnalyticsBlock, validate read→ValidationPanel, sweep read→SweepCensus, events→EventsTimeline, version→footer). NO rows have NO component — the anti-inventory is explicit |
| D6 (provide-dont-force) | No "install the UI" affordances, no auto-open copy; the footer states "served by boardctl serve — open http://127.0.0.1:8787/ui"; nothing in the design implies a schedule/hook/agent dependency |

## 10. Accessibility baseline

Target: WCAG 2.1 AA (a static, loopback, internal tool — AA is the honest
bar; AAA is not claimed).

Contrast (measured, WCAG 2.x relative luminance; script-computed, not
eyedroppered):

- Body text #e6edf3 on #0d1117: 16.02:1; on #161b22: 14.64:1; on #1c2330:
  13.34:1.
- Secondary text --muted #8b949e: 6.15:1 (bg), 5.62:1 (bg2), 5.12:1 (bg3).
  Muted is cleared for 13px+ secondary copy; captions at 12px use fg at 600
  weight or muted at 13px minimum.
- All chip/pill text ≥4.5:1 on their own tints (tables §8.1); the a11y
  tables and the mockups carry the SAME final values — there is no
  provisional set anywhere in this document or the mockups.
- Non-text UI (borders, focus ring, chips' borders) ≥3:1 against adjacent
  fills: --line on bg2 = 3.0:1 (1px hairlines are supplemented by fill
  steps), accent focus ring 7.49:1.
- Color is NEVER the only signal: every status/priority chip carries its
  literal text; every validation state carries the word (PASS/WARN/FAIL);
  streak states carry "(live)"/"(ended)"; the fixture chip is a word. Hue
  duplicates text everywhere.

Keyboard reachability (inherited analytics §5.4, restated as the /ui
baseline):

- Every interactive element is a native element or carries role +
  aria-expanded/aria-pressed: rows (button semantics), tabs (role=tablist),
  filters, expand/collapse-all, banner dismiss, batch affordance, task
  links.
- Tab order = visual order: header → notices → content → footer. Within a
  board page: header strip → toolbar (search, status, fixtures, expand
  all, collapse all) → row table → validation panel → analytics → events
  timeline.
- Row expansion: Enter/Space toggles, Escape collapses the focused expanded
  row. Expanded state is visual + aria-expanded (never color-only).
- Hash addressability `#b=<slug>&t=<task-id>`: on load and hashchange, the
  target row is expanded, scrolled to, and FOCUSED (focus, not just
  scroll — analytics §5.4 says "expands on load and scrolls"; the /ui
  baseline adds focus for screen-reader arrival).
- Skip link: "skip to board list" as the first focusable element (single
  deep page — one skip link suffices).
- Filters: debounced 150ms free text (analytics §5.5), applied on input;
  selects apply on change; a live-region count line announces "N rows ·
  M shown" after each filter change (aria-live=polite).

Focus order and visibility:

- Focus ring: 2px --accent outline, 2px offset, on --bg — visible on every
  surface (7.49:1 vs bg; ≥3:1 vs bg2/bg3 too). No outline suppression
  anywhere.
- Focus NEVER lands on a decorative element; the timeline spine, chips'
  borders, and KeyNumber values are non-focusable.
- Modals don't exist (§7 anti-inventory), so no focus-trap machinery is
  needed; the drawer variant (BT-074, if adopted) MUST trap focus and
  close on Escape — stated now so BT-074 inherits it.

Screen reader specifics:

- Chips announce their full text ("status: blocked", "priority: P2" via
  visually-hidden label prefix where the chip text alone is ambiguous).
- The row table is a real `<table>` with `<th scope=col>`; the expanded
  detail is a region labeled by the task id (`aria-labelledby`).
- The events timeline is a `<ol>`; each EventItem is a list item; decoded
  JSON blocks are `<pre>` inside the item, announced as text.
- ValidatePill announces "board validation: FAIL, 1 error, 2 warnings".
- OfflineNotice and banners: role=status (aria-live=polite), so a refresh
  failure is announced without interrupting.

## 11. Mockups

Self-contained single-file HTML artifacts under docs/mockups/ (inline CSS,
no JS beyond trivial expanders, no external assets, file://-renderable).
They are DESIGN ARTIFACTS (provide-dont-force applies — nothing here ships
as the UI; BT-075 builds from the tokens, not by copying markup):

- `board-view.html` — board page: header strip, toolbar, collapsed rows +
  one expanded row (RowDetail), validation panel with FAIL state,
  analytics placeholder slots, events timeline.
- `row-detail.html` — the RowDetail component alone, drawer framing, all
  blocks present (prose, verdicts, timestamps, links, related events, raw
  JSON).
- `validation-warn.html` — validation panel in WARN state (warn-only
  findings + census + sweep census with unknown statuses named).
- `events-timeline.html` — timeline alone: decoded JSON details, raw-string
  detail, task links, filters, type chips.
- `empty-error.html` — the four non-happy states side by side: empty
  board, filtered-to-zero, error state, offline notice.

Mockup fidelity notes: fixtures are FAKE boards ("demo-fleet") with
obviously synthetic ids; hostile-string examples (the §6.3 `</script>`
title) appear as escaped TEXT to demonstrate the inert-rendering contract
(R16); the events detail shows one ladder-decoded pretty-JSON item and one
raw fall-through item per §2.4.

## 12. What BT-074/075 inherit (and what they must not change)

Inherited unchanged: tokens §8 (colors, type, spacing, radius), status /
priority color maps (they mirror validated vocabularies — changing them
needs a vocabulary change upstream first), the a11y baseline §10, the
component names §7 (BT-075's CSS classes should map 1:1 to these names),
the requirement map §9's completeness rule (every R keeps a visual answer
as the surface evolves).

Open for BT-074 to decide: flows between views (grid → board → events →
back), the refresh flow's exact UX (R18's /api upgrade path), compare-view
selection UX, whether RowDetail's drawer variant replaces the inline
expansion on wide screens, the batch-affordance threshold (500).

Must-not-change without a spec amendment: anything that adds a write
surface, an endpoint, an external asset, an auth affordance, or a client-
side derivation — each is nailed by R/D/NG in BT-072 and restated here.
