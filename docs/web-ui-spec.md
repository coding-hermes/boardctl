# SPEC: boardctl web UI — scope + specification

Status: DESIGN AUTHORITY for BT-073 (visual design), BT-074 (UX flows),
BT-075 (implementation)
Date: 2026-10-01
Refs: BT-072 (this spec's board row), docs/specs/board-analytics-report.md
(§2 data contracts, §3 derivations, §5.6/5.7 UX states, §6.1 CLI contract,
§6.2 payload, §6.3 single-file HTML constraints — all inherited verbatim),
BT-021 (serve loopback uploader contract), BT-056 (key uniformity census),
BT-054-R (--fail-on promotion), DF-BOARDCTL-2 (board identity + dedupe, the
POST-FIX semantics), REVIEW-BOARDCTL-001 (sweep-status), the
provide-dont-force doctrine (explained in D6)

---

## 1. Problem statement

boardctl already understands fleet boards end to end — both topologies,
validation, status vocabulary, analytics, self-contained report — but every
capability is CLI-only. A human who wants to see where the fleet stands must
run verbs by hand, or open a rendered HTML file that was true only when
written.

Goal: a browser UI, served by boardctl itself, where a human points at a
running `serve` instance and sees the loaded boards — rows, validation
state, events, analytics — without hand-running CLI verbs.

This spec settles WHAT the UI is and is not. It does not settle HOW it
looks (BT-073), how it flows (BT-074), or how it is built (BT-075); those
rows derive from this document and may not contradict it. Requirements use
MUST / SHOULD / MAY; every requirement names its acceptance test. "Board
data" always means the untrusted JSONL content of tasks.jsonl /
events.jsonl / board.jsonl.

## 2. Ground truth (verified on origin/main 7f36a13 — do not re-litigate)

These facts constrain every requirement below. They were grep-verified, not
assumed; a downstream row that needs a different fact amends this section.

- F1. serve's route table is exactly three patterns
  (cmd/boardctl/serve.go `routes()`): `GET /` (uploader form), `POST /`
  (multipart upload: single `zip=@board.zip` or `files` folder upload),
  `GET /api/boards` (loaded boards as JSON). NO endpoints exist for rows,
  events, validate output, or per-board metrics today.
- F2. POST / always prepends the `-C` board (BT-027): an upload that
  registers zero boards answers 200 with only the -C board in the report.
- F3. The report is a server-rendered SPA payload:
  `<script type="application/json" id="report-data">` built by
  internal/render from the board snapshot at upload time. internal/render
  and docs/specs/board-analytics-report.md are the governing analytics
  authority (§5.6/5.7/§6.1).
- F4. serve flags: `-C dir`, `--addr` (default 127.0.0.1:8787). A non-loopback
  `--addr` host is refused via errUsage → exit 2, BEFORE any listener exists.
  BT-021 line: "serve is a loopback-only convenience uploader".
- F5. The -C board is resolved ONCE at startup (cmdServe pre-load) — an
  in-memory snapshot thereafter. Uploaded boards extract into per-upload
  temp trees, copied never referenced; all temps are deleted at shutdown
  (spec 7.2.8).
- F6. DF-BOARDCTL-2 (post-fix): re-uploading a board whose identity equals
  a loaded board's REPLACES it instead of duplicating. Identity = the
  report-display name's slug + topology (`serveBoardIdentity`). Superseded
  sessions lose the replaced board and are deleted when empty, with their
  temp trees.
- F7. CLI verbs a UI could surface: init, list, show (with --events),
  create, update, event, header, validate (--repair/--strict-keys/--fail-on),
  sweep-status (--apply/--decide), doctor, version, stats, render, import,
  serve, install, dedupe-board.
- F8. Exit codes (main.go): 0 success, 1 validation/writer failure, 2
  usage/board-not-found. serve's SIGINT/SIGTERM shutdown exits 0.
- F9. validate produces Finding{Level: "error"|"warn", message with line
  numbers}; every run prints the census line `key uniformity: N/M rows
  carry only canon keys`. sweep-status (read mode) classifies
  off-vocabulary statuses and reports unknown ones without guessing;
  --apply/--decide are the write arms.
- F10. Board files are NEVER written by render or serve: the only state a
  serve process mutates is its in-memory session registry and temp trees.

## 3. The six decisions (binding; a downstream row that contradicts one is wrong)

### D1 — Reuse vs new verb vs serve extension: EXTEND `serve`

The UI is part of `boardctl serve` — not a new verb, not a separate
process. Named reasons:

1. serve already owns the multi-board session model, the DF-BOARDCTL-2
   identity dedupe, and the loopback server loop — a new verb duplicates all
   three.
2. BT-021 fixed and tested serve's security posture (loopback bind,
   non-loopback refusal exit 2, clean shutdown, temp cleanup); reusing it
   inherits those tests instead of re-earning them.
3. Analytics §6.3 pins the REPORT page as fully static and file://-capable;
   hosting a UI inside `render` output would violate that pinned contract.
4. Operator cost: one process, one port, already documented.

Rejected alternatives: a `boardctl ui` verb (duplicates server + session +
identity machinery, second loopback posture to audit forever); enriching the
report HTML into the UI (the report must stay a static file:// artifact).

### D2 — Single-board vs fleet: FLEET (multi-board), single is the degenerate case

Named reasons: serve's session model is already multi-board (F2, F6);
/api/boards already lists N boards (F1); and multi-board compare (analytics
§3.10) exists — a single-board UI would discard it. One loaded board still
renders fully: a one-board fleet.

### D3 — Read-only vs write: READ-ONLY

The UI adds ZERO write endpoints. Board files are never written (F10 is a
hard invariant the UI inherits, not relaxes). validate and sweep-status are
surfaced as findings/census only — their --repair/--apply/--decide write arms
stay CLI-only. The one pre-existing state-changing path, POST / (upload →
temp trees + session registry), stays exactly as it is; the UI links to it
but does not extend it. Reason: boards are the fleet's operational memory; a
browser tab that can mutate a live foreman's board turns a viewer into an
unreviewed writer, and violates provide-dont-force (D6). The uploader's own
promise ("nothing is written to your board files") becomes the UI's too.

### D4 — Auth and bind posture: loopback-only, no auth, no TLS, no cookies

Inherited wholesale from BT-021 / analytics §1. Named reasons: serve is a
local convenience surface; adding auth to a loopback port adds session state
the report explicitly excludes and creates a false sense of security; any
LAN/Tailscale exposure is a DIFFERENT threat model requiring tokens and is
future work (Section 8), not a knob in this scope.

### D5 — Capability inventory: read-shaped verbs in, write/host-shaped out

Section 6 decides every verb IN or OUT with a reason. The rule: read-shaped
verbs surface in the UI; write-shaped and host-shaped verbs stay CLI.

### D6 — Provide-dont-force: the UI is opt-in at every layer

A human starts serve. A human opens the browser. No fleet scheduler, hook,
install step, or agent code path references, schedules, or requires the UI.
Nothing breaks when it is never used. Named reason: the doctrine — a tool
the agent CHOOSES, nothing enforced; a UI that automation depends on becomes
load-bearing infrastructure with an uptime contract, which this is not.

## 4. Architecture the design inherits

- The UI is served by the SAME process and listener as today's uploader
  (D1). No second port, no sidecar, no database, no background jobs.
- All derivations stay in Go (analytics §6.2): the browser renders numbers
  from payloads; it never recomputes analytics.
- What a view shows is always a SNAPSHOT (F5): the -C board as resolved at
  startup, or an upload as extracted. No live re-read, no file watching.
  "Refresh" means re-upload (or restart serve).
- The payload machinery (render.BuildBoards, schema `board-report/v1`) is
  the single source of truth for both the existing report and the new UI.

## 5. Requirements

### 5.1 Surface and posture

**R1 — The UI lives at `GET /ui` inside serve; the route set stays closed.**
serve MUST serve the UI shell at `GET /ui` — one self-contained HTML
document. The complete route set after BT-075 MUST be exactly: `GET /`,
`POST /`, `GET /api/boards`, `GET /ui`, and the read-only /api/ endpoints
named in R7; any other path MUST 404 as today. The uploader stays the
landing page — the UI MUST NOT be served at `/`.
Acceptance: `TestBT072UIRouteServed` — httptest against `routes()`:
`GET /ui` answers 200 text/html with a self-contained document; a scan of
the registered patterns equals the closed set above; `GET /ui/nested`
404s.

**R2 — The existing surface is unchanged.** `GET /` (uploader page),
`POST /` (upload → report), and `GET /api/boards` MUST keep their current
semantics and response shapes; /api/boards MAY gain additive JSON fields
but MUST NOT lose or repurpose an existing key (name, slug, topology,
task_total, done, open).
Acceptance: `TestBT072ExistingSurfaceUnchanged` — golden marker probe of the
uploader page, a zip upload round-trip answering 200 with the report, and a
/api/boards shape check on a fixture (all existing keys present, values
equal to the current derivation).

**R3 — Loopback-only is inherited, not re-argued.** Every new route is
served by the same listener; the non-loopback `--addr` refusal (exit 2
before listen, F4) MUST remain byte-identical in behavior.
Acceptance: `TestBT072LoopbackInherited` — the BT-021 refusal probe still
exits 2 for `--addr 0.0.0.0:8787`; with the server bound to 127.0.0.1 the
/ui and /api routes answer.

**R4 — No auth, no TLS, no cookies, no server-side session state beyond
upload sessions.** No new route sets cookies, demands credentials, or
requires state beyond the existing upload-session registry.
Acceptance: `TestBT072NoAuthArtifacts` — responses from /ui and every /api/
route carry no Set-Cookie and no WWW-Authenticate header; the serve source
imports no crypto/tls and no auth package (static scan in the test).

**R5 — Provide-dont-force (D6).** Nothing in this repository's write or
install paths (install, install --fleet, hooks) MAY reference /ui or any
new route; the UI is reachable only by someone who starts serve and opens a
browser.
Acceptance: `TestBT072ProvideDontForce` — grep-level test: no occurrence of
"/ui" in install.go, fleet.go, or the installed hook template.

### 5.2 Read model

**R6 — One derivation engine: Go.** Every number the UI shows (totals,
done/open, analytics, validate counts) comes from the render payload
machinery built on the loaded snapshot (F1, F3). The UI's inline JS MUST
NOT recompute derived values (analytics §6.2 restated for /ui).
Acceptance: `TestBT072GoDerivedNumbers` — on a fixture board, the counts in
`GET /api/board/{slug}` equal the values render.BuildBoards produces for
the same snapshot (asserted field-by-field in Go, no JS involved).

**R7 — The read-only API is a closed set of three endpoints.**
- `GET /api/boards` — exists today (F1); MAY gain additive fields.
- `GET /api/board/{slug}` — one board's full read model: header, task
  rows, derived block (analytics §6.2 shapes), parse warnings, validate
  findings (R13), loaded_at timestamp. Identity key: the report slug.
- `GET /api/events/{slug}` — event rows, each with the detail ladder
  already applied (analytics §2.4: object → parsed-JSON string →
  base64-wrapped JSON → raw string).
No other /api/ route may exist. Responses MUST be `Content-Type:
application/json` with encoding/json's HTML escaping active so JSON can
never smuggle markup.
Acceptance: `TestBT072APISurfaceClosedSet` — the registered-pattern scan
names exactly these; `TestBT072APIXSSInert` (with R16) proves the escaping.

**R8 — Unknown slug is a JSON 404, not HTML.** `GET /api/board/{missing}`
and `GET /api/events/{missing}` MUST answer 404 with a JSON body
`{"error": "..."}` naming the slug.
Acceptance: `TestBT072APIUnknownSlug404JSON` — status 404, body parses as
JSON, contains the requested slug, and contains no HTML tags.

**R9 — Snapshot semantics are visible.** Every board view MUST carry the
time its snapshot was loaded (loaded_at, RFC3339). Editing the underlying
files after load MUST NOT change what /api/ or the UI shows until a new
upload or a serve restart.
Acceptance: `TestBT072SnapshotSemantics` — load a board from a temp dir,
append a task row to its tasks.jsonl on disk, assert the /api read model
is unchanged and loaded_at is present and parseable.

**R10 — Identity and dedupe: the CURRENT post-fix semantics (F6), stated
precisely because older docs describe the defect.** A re-uploaded board
whose identity (report-name slug + topology) equals a loaded board's
identity REPLACES it: /api/boards shows one entry with the NEW upload's
counts. The -C board plays the same game: always present from startup
(even with zero uploads, F2), replaced only by an upload of its exact
identity. Boards sharing a name but differing in topology stay DISTINCT
entries. The UI shows one entry per identity and offers no "versions of
one board" concept.
Acceptance: `TestBT072IdentityDedupePostFix` — extend the DF-BOARDCTL-2
battery: upload A (3 tasks), re-upload A' (5 tasks, same name+topology) →
/api/boards has ONE A entry with 5; upload B (same name, topology A vs B) →
two entries; the -C board is listed after zero uploads and remains listed
after an unrelated upload.

### 5.3 Views

**R11 — Overview: all loaded boards.** The /ui landing view MUST list every
loaded board as a card: name, slug, topology, task total / done / open
(the /api/boards fields, R2), its validate pill (R13), and a link to the
board view. One board → one card. Zero boards — unreachable via serve
(F2) but reachable in tests — renders an honest empty state (analytics
§5.7 tone).
Acceptance: `TestBT072OverviewCards` — fixture with two boards: /ui's
embedded payload yields two cards whose fields equal /api/boards; the
one-board and zero-board fixtures render the one-card and empty states.

**R12 — Board view: the row table, collapse-first.** Per board: a table of
task rows (id, status, priority, title, depends_on count) COLLAPSED by
default, expanding to the full raw row (analytics §5.3 pattern); a header
strip showing the board header (project, namespace, ticks_total,
ticks_idle, last_commit — the `header` verb's fields, F7); topology-B
header lines MUST be skipped (shape detection, never "first line always",
analytics §2.1). Expansion MUST work from the embedded payload without
refetching.
Acceptance: `TestBT072BoardRowsTable` — a topology-B fixture: every task
row renders exactly once, the header line appears in the header strip and
in no row; row count in the payload equals rendered row count.

**R13 — Validation panel: findings, not enforcement.** Per board the UI
MUST show the `boardctl validate` result for its snapshot: a pill — PASS
(0 errors, any warnings), WARN (warnings only), FAIL (≥1 error) — plus
the findings (level, message with line numbers) and the census line
`key uniformity: N/M rows carry only canon keys` quoted verbatim (F9,
BT-056), plus the sweep-status READ census (REVIEW-BOARDCTL-001):
off-vocabulary status counts, unknown statuses named with their rows, no
decisions applied. No repair/apply/decide action exists in the UI (D3).
Findings are computed once per snapshot load (upload or -C resolve) and
ride in the read model — a snapshot-time read, never a live re-check.
Acceptance: `TestBT072ValidationPanel` — fixture with a duplicate-id error
and an off-vocab warning: pill FAIL, both findings present with correct
line numbers, census line matches `b.Validate()` output byte-for-byte; a
warn-only fixture: pill WARN, exit-equivalent 0 semantics documented in the
panel; the read model equals a CLI validate run on the same files.

**R14 — Events timeline.** Per board: a chronological timeline (event_type,
timestamp, actor, top-level task_id linkage per analytics §2.4) showing
the detail ladder's DECODED form — raw string only when the ladder fell
through. Client-side filters by event_type and by task id. Malformed or
hostile detail strings render as text, never markup (R16 applies).
Acceptance: `TestBT072EventsTimeline` — fixture events include a
JSON-encoded-string detail and a base64-wrapped detail: both render
decoded; an event with task_id renders linked to that task's row; a
`<script>`-bearing raw detail renders inertly.

**R15 — Analytics are reused, never reimplemented.** Every Section-3 view
of the analytics spec (burndown, burn-up, velocity, cycle time, streaks,
tick health, work clock, model share, status/priority counts) and the
multi-board compare view (§3.10, §5.6) MUST be available per board and
across boards, driven by the same derived payload as the report. The UI
defines NO new derivation; where report and UI could disagree, the
report's payload is authoritative.
Acceptance: `TestBT072AnalyticsParity` — for a fixture snapshot, each
derived value rendered by the UI equals the value in the report HTML built
from the same snapshot (both parsed from their payloads and compared in
Go).

### 5.4 Safety and shape

**R16 — Untrusted-data hardening, both surfaces.** The analytics §6.3
rules apply to /ui and every /api response: the JSON island is serialized
with encoding/json + HTML escaping so the sequence `</script>` cannot
occur inside it; `innerHTML` is FORBIDDEN in the /ui inline JS (static
template literals with no payload-derived data excepted); payload data
reaching attributes goes through `setAttribute`. The page MUST never
throw on bad input (analytics §5.7).
Acceptance: `TestBT072XSSInert` — a payload whose task title is
`</script><img src=x onerror=alert(1)>`: the island round-trips
byte-identical through JSON.parse (fixture asserted in Go), the static scan
of the /ui script finds no `innerHTML` assignment, and the title renders as
text in the row-table markup derived from the payload.

**R17 — Network confinement.** The /ui inline JS MAY fetch ONLY same-origin
`/api/` routes. No CDN, fonts, telemetry, external anything (analytics
§6.3 restated for /ui).
Acceptance: `TestBT072NetworkConfined` — static scan of the /ui document:
every `fetch(` literal argument starts with `/api/`; no `http://` or
`https://` literal appears in the inline JS.

**R18 — Standalone degradation.** /ui MUST embed an initial snapshot island
(same payload as /api/board at load time) so the overview renders even when
/api calls fail; /api is the refresh mechanism, the island is the floor. A
failed fetch surfaces an inline error state, never a blank page (§5.7).
Acceptance: `TestBT072StandaloneDegrade` — httptest serving /ui with every
/api route swapped to 500: the island still parses and the overview fields
are renderable from it; the page contains an error-state marker.

**R19 — Size guardrail.** The analytics §6.3 guidance extends to /ui: the
report for a 300-task/2000-event board SHOULD stay under ~5 MB (document +
embedded payloads combined). Exceeding it is not an error; a note is
logged (serve stderr, like parse warnings today).
Acceptance: `TestBT072SizeGuardrail` — generate the 300/2000 fixture used
by the render size test, measure /ui + one /api/board payload, assert the
note fires only over the bound.

**R20 — Zero board writes (the D3 invariant, made testable).** Beyond
POST / (which writes only serve-owned temp trees, F10), NO handler in the
BT-075 surface creates, modifies, or deletes any file outside serve's
temp roots.
Acceptance: `TestBT072NoBoardWrites` — integration probe: start serve on a
fixture board, exercise every new route, assert the fixture tree's file
hashes are unchanged and no file outside the temp root was created during
the window (temp root enumerated before/after).

## 6. Capability inventory (D5, decided; each row cites its requirement or non-goal)

| CLI capability | In the UI? | Where / why |
|---|---|---|
| list / show | YES | board row table (R12) |
| stats | YES | overview cards + header strip (R11, R12) |
| header | YES | board header strip (R12) |
| render (analytics + compare) | YES | analytics + compare views, reused (R15) |
| validate (read) | YES | validation panel (R13) |
| sweep-status (read census) | YES | validation panel subsection (R13) |
| show --events / event log | YES | events timeline (R14) |
| version | FOOTER ONLY | static string in /ui footer (R11) |
| serve upload | UNCHANGED | existing POST /; UI links to it, never extends it (R2, R20) |
| create / update / event (writes) | NO | NON-GOAL NG1 (D3) |
| validate --repair | NO | NON-GOAL NG4 |
| sweep-status --apply / --decide | NO | NON-GOAL NG5 (write arms) |
| doctor | NO | NON-GOAL NG6 (host diagnostics, not board data) |
| install / import | NO | NON-GOAL NG6 (host/file writes) |
| dedupe-board | NO | NON-GOAL NG7 (repair tool; duplicate-id findings already surface via validate, R13) |
| init | NO | NON-GOAL NG6 (creates boards; the UI views them) |

## 7. Acceptance test index (binding; AC2 — every requirement names its test)

| Req | Acceptance test (cmd/boardctl, house naming) |
|---|---|
| R1 | TestBT072UIRouteServed |
| R2 | TestBT072ExistingSurfaceUnchanged |
| R3 | TestBT072LoopbackInherited |
| R4 | TestBT072NoAuthArtifacts |
| R5 | TestBT072ProvideDontForce |
| R6 | TestBT072GoDerivedNumbers |
| R7 | TestBT072APISurfaceClosedSet (+ TestBT072APIXSSInert w/ R16) |
| R8 | TestBT072APIUnknownSlug404JSON |
| R9 | TestBT072SnapshotSemantics |
| R10 | TestBT072IdentityDedupePostFix |
| R11 | TestBT072OverviewCards |
| R12 | TestBT072BoardRowsTable |
| R13 | TestBT072ValidationPanel |
| R14 | TestBT072EventsTimeline |
| R15 | TestBT072AnalyticsParity |
| R16 | TestBT072XSSInert |
| R17 | TestBT072NetworkConfined |
| R18 | TestBT072StandaloneDegrade |
| R19 | TestBT072SizeGuardrail |
| R20 | TestBT072NoBoardWrites |

Harness shape: Go httptest probes against `serveServer.routes()` plus
payload/island parsing (the technique render_test.go and serve_test.go
already use). No JS runtime, no browser, no network dependency.

## 8. NON-GOALS (hard exclusions — a PR that adds any of these is wrong)

- NG1. No board writes from the UI: no create/update/event/status
  endpoints, no forms that mutate boards, no import. Writes are CLI-only
  (D3, analytics §1).
- NG2. No auth, no TLS, no multi-user anything, no non-loopback bind, no
  token flag. (D4; LAN/Tailscale exposure is future work, Section 9.)
- NG3. No live file watching, no auto-refresh, no polling daemon, no
  websocket/streaming. The view is the snapshot; refresh = re-upload.
- NG4. No validate --repair surface: tasks.rewritten.jsonl and every
  salvage flow stay CLI.
- NG5. No sweep-status --apply/--decide surfaces; the UI renders the
  census and names the decisions needed, it never applies them.
- NG6. No doctor, install (--fleet included), import, or init surfaces:
  host-shaped and file-writing verbs stay CLI.
- NG7. No dedupe-board surface (no repair tools in the UI).
- NG8. No new analytics derivations in the browser; no chart libraries,
  no canvas, no d3 (analytics §6.3 inheritance).
- NG9. No external network from /ui or /api responses: no CDN, no external
  fonts, no images, no telemetry.
- NG10. No persistence across serve restarts: no database, no caches, no
  config writes; temp trees die at shutdown exactly as today (F5, 7.2.8).
- NG11. No visual design, component inventory, design tokens, mockups, or
  implementation code in THIS deliverable — those are BT-073, BT-074,
  BT-075. This spec settles scope and contracts only.
- NG12. Not a hosted app: no background jobs, no webhooks, no scheduled
  anything, no dependency from any agent or scheduler on the UI existing
  (D6).

## 9. Risks, open questions, future work

- LAN/Tailscale exposure: a real want (check fleet state from a phone) but
  a different threat model (D4): token auth, bind semantics, explicit
  threats — its own spec row first; NG2 stands until that row exists.
- Live refresh (re-read the -C board on demand): breaks the snapshot
  contract R9/R13 lean on. Park until a row asks with a staleness story.
- /api/boards additive fields: BT-075 may add fields (validate pill,
  loaded_at) but MUST keep R2's shape contract; outside consumers are
  unknown, so treat every existing key as load-bearing.
- Report vs UI drift: R15 pins the payload as tiebreaker; if BT-073/074
  want a view the payload cannot drive, the payload schema is amended
  FIRST (analytics spec amendment), never bypassed.
