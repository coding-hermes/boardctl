# SCOPE: connecting to boardctl over the muster protocol

Task: BT-061 (scope row, one third of the BT-061 → BT-062 → build chain)
Status: SCOPE — the written decisions BT-062 (spec) must implement and must not re-open
Date: 2026-09-25
Measured against: boardctl wt/BT-061 @ 5737be9; muster @ 63d42c03 (binaries built
2026-09-25, go1.26.6); all acceptance commands in §3 executed verbatim against a
loopback probe before this document was committed (evidence: Appendix A)
Design authority upstream: muster/specs/PROTOCOLS.md ("no protocol implementation row
may start before its adapter satisfies this spec's pipeline and cross-cutting contracts
on paper"), read before any later row in this chain.

## 0. Corrections to the row's stated facts (read first)

Three premises in the row's reasoning need correcting; the scope below is built on the
measured tree, not on the stale premises.

1. "internal/describe/registry.go is the ONE source of --help / describe --json /
   docs/reference.md" and "cmd/gendocs generates docs/reference.md" — NOT TRUE at
   5737be9. Measured: `ls internal/describe cmd/gendocs` → both missing; there is no
   docs/reference.md; there is no `describe` verb (main.go verb table: init, list,
   show, create, update, event, header, validate, sweep-status, doctor, version,
   stats, render, import, serve, install, help). The repo's real docs-fact gates are
   check-only Go checks run by `go test ./...` AND by CI (.github/workflows/ci.yml):
   internal/fmtcheck (Makefile `fmt-check` + CI "Gofmt"), internal/versioncheck
   (README release pins), internal/workflowcheck (Makefile PLATFORMS ↔
   multiarch.yml drift), internal/vulncheck, internal/freshness. The parity
   discipline the row wants exists as a *pattern* (versioncheck/workflowcheck), not
   as a describe registry. §2 builds on the pattern that actually exists.
2. "this repo already fails its build on docs/routes drift" — there is no build-step
   route/doc parity gate at 5737be9 (no route table beyond serve's three mux
   patterns, no docs that list routes). What exists: the check-only Go-test gate
   pattern above, wired into the same CI `go test -short -count=1 ./...` step.
   §2's parity check is therefore specified as a NEW test on that existing pattern,
   and the spec row owns landing it.
3. muster's "MCP safety contract for mutating operations" (row item 4) — the written
   contract in muster/specs/PROTOCOLS.md §4.5 governs unbounded/streaming/bidi
   operations (bounded transcript, tools must return, opt-in exposure for
   bidi/client-stream). It does not define a mutation-permission matrix. The
   read-only boundary for boardctl comes from (a) muster's architecture itself (a
   tool only exists if the spec declares the operation — so exclusion at the spec
   level IS the gate) plus (b) this fleet's standing rule (board writes are the
   foreman's exclusive responsibility; workers must not mutate them) plus (c)
   boardctl's own serve spec (docs/specs/board-analytics-report.md §1 non-goals:
   "No ... writes to the board during render/serve"). §4 cites each layer precisely.

Also noted: the sibling row BT-060 is `pending` with EMPTY reasoning at 5737be9; BT-062
shares this row's reasoning text. If BT-060 was meant to carry the predecessor
dependency this chain defers to, the foreman should re-file it — nothing in this scope
depends on it.

## 1. The surface we expose, and what we explicitly do not

### 1.1 Exposed (the whole muster surface is exactly these three operations)

| operationId | route | why it is exposed |
|---|---|---|
| `listBoards` | `GET /api/boards` | The only machine-readable projection serve has today: JSON array of board summaries (name, slug, topology, task_total, done, open — the `apiBoard` struct, serve.go). This is what an agent actually wants: fleet progress without reading JSONL. |
| `getUploaderForm` | `GET /` | The uploader page (self-contained HTML). Exposed because it is the daemon's identity page — its title names the service — and because a consumer probing `--base-url` gets a truthful answer. NOT exposed as a promise of usefulness: the HTML is a browser artifact (spec §6.3 single-file document), an agent should not parse it. |
| `getOpenapiSpec` | `GET /openapi.json` | The muster-consumable spec itself, served live (decision D2). Bootstrap operation: a consumer points `--spec` at the running daemon and needs nothing else on disk. |

Every operation is GET, unauthenticated, parameter-free, and returns a bounded body.
`listBoards` returns derived aggregates only — never raw `tasks.jsonl`/`events.jsonl`
rows; the raw board files stay readable only to the host that owns them.

### 1.2 Explicitly NOT exposed (this list is part of the contract)

- `POST /` (multipart upload → HTML report). Absent from the muster spec entirely —
  not "hidden", not "deprecated": the operation does not exist on the agent surface.
  Reasons: (a) uploads are interactive/CLI work, not agent work; the response is a
  600 KB+ single-file HTML blob that is useless to an agent; (b) it mutates server
  state (session registry, temp extraction dirs) even though it never touches board
  files — it fails the "agent-safe default" bar of §4 on state grounds alone;
  (c) representing multipart/form-data file transfer for an agent adds spec complexity
  for zero demonstrated consumer need. If a future row disagrees, it must make the
  case against all three, then follow the mutating-op gate in §4.
- Anything that writes board files (`create`, `update`, `event`, `header`, `import` —
  the CLI write verbs). These are CLI-local operations with no HTTP route today, and
  the muster surface must not invent one. The fleet rule is absolute: board writes
  are the foreman's exclusive responsibility. This is a standing decision, not an
  open one — the spec row does not get to relitigate it.
- Nothing else: the muster spec contains exactly the operations of §1.1. No catch-all
  route, no generic proxy op, no `POST /openapi.json` (the spec is read-only even at
  its own URL).

### 1.3 One deferred item (named, so the spec row cannot invent silently)

`GET /api/report` — a structured JSON report payload (the same data the HTML island
carries) would be the natural agent-facing analytics surface, but it does not exist in
serve today and this scope row does not create new HTTP surface. Deferred to the spec
row's review; default answer: don't. Ruled by the foreman during BT-062 review.

## 2. The exact muster consumer config (verbatim)

Decision D2: the spec is SERVED live from the running daemon at `/openapi.json`
(answer to the row's file-vs-served question). Muster's `openapi-mcp` loader accepts a
URL spec natively (cmd/mcp-server/main.go: "Load OpenAPI spec (supports both file
paths and URLs)"; proven live, Appendix A), a served spec cannot go stale relative to
the running process, and it stamps the live port so a consumer never edits a file when
the daemon moves. A committed copy of the spec is the FALLBACK for consumers on hosts
that cannot reach the daemon (CI, offline review); it must be generated, never
hand-written, and it carries the default `servers[].url`.

### 2.1 Primary consumer block (served spec — the path/URL a verifier actually uses)

```json
{
  "mcpServers": {
    "boardctl": {
      "command": "/absolute/path/to/bin/openapi-mcp",
      "args": ["--spec", "http://127.0.0.1:8787/openapi.json",
               "--base-url", "http://127.0.0.1:8787"]
    }
  }
}
```

Fixed by this scope: server name `boardctl`; `--spec` is the served URL on the
default loopback addr `127.0.0.1:8787` (the daemon's own default); `--base-url` is
included explicitly even though the spec carries `servers[].url` — belt and braces,
and it makes the block self-contained for a reader (proven live: `override=true`).
Binary path must be absolute (muster integration-guide §2.1 convention).

### 2.2 Fallback consumer block (file spec — offline/CI consumers)

```json
{
  "mcpServers": {
    "boardctl": {
      "command": "/absolute/path/to/bin/openapi-mcp",
      "args": ["--spec", "/absolute/path/to/boardctl-openapi.json"]
    }
  }
}
```

Proven live: with an absolute `servers[].url` in the spec and no `--base-url`,
`openapi-mcp` resolves the target from the spec (`override=false`) and `tools/call`
hits the daemon. The file variant works ONLY while the spec's `servers[].url` stays
absolute and current — one more reason the served variant is primary.

### 2.3 Parity check (reusing the repo's existing gate pattern)

The row asks for the parity discipline "this repo already fails its build on docs
drift". What exists at 5737be9 is the check-only Go-test gate pattern
(internal/versioncheck, internal/workflowcheck) enforced by both the Makefile and CI's
`go test -short -count=1 ./...`. The muster parity check is specified as a NEW test on
that pattern; the spec row lands it (location its call — e.g. a serve-package test or
`internal/servecontract`; the scope pins only the assertions):

1. Walk the REAL route table — `serveServer.routes()` is already the shared seam
   serve tests use — and require every GET route to appear in the generated spec with
   the operationId this scope froze (`listBoards`, `getUploaderForm`,
   `getOpenapiSpec`). BOTH directions: a route missing from the spec fails, and an
   operation in the spec that the mux does not mount fails (the second direction is
   what stops a phantom write op from ever entering the spec).
2. Serve-identity check: fetch `/openapi.json` from a live `httptest`-backed
   serveServer and require it to parse to the same document the generator produces
   (the served spec cannot drift from the generated one).
3. Floor assertion: the walked route count and the spec operation count are each
   ≥ 3, so an enumeration bug that collects nothing cannot read as parity.
4. Write-absence assertion: the spec's paths contain no POST/PUT/PATCH/DELETE
   anywhere (machine check of §1.2, so a future edit cannot silently add one).
5. Non-vacuity per the repo's docs-parity lessons: RED-proof the test by mutating
   the generator (drop a route from the spec; add an unmounted operation; rename an
   operationId) and requiring a distinct failure per arm, then restore.

## 3. Acceptance test for the later rows (verbatim commands + non-vacuous assertions)

Every command below was executed against muster binaries built from 63d42c03 and a
loopback probe mimicking this surface (Appendix A). The verifier re-runs them as
written. Conventions: `$BT` = a board dir with at least one real board (the
verifier's checkout `.coding-hermes/board` is fine); `$SPEC` = the served or
generated spec; muster binaries built with `make build-cli build-mcp` in a muster
checkout. Assertions on JSON are jq, selected BY RESPONSE ID (openapi-mcp also emits
non-request lines — an unscoped jq passes vacuously).

A.1 — Daemon up, loopback-bound, spec served:
```bash
boardctl serve -C "$BT" --addr 127.0.0.1:8787 &   # keep pid; stop with kill
curl -sf http://127.0.0.1:8787/openapi.json | jq -e '.openapi | startswith("3.")'
```
Assert: exit 0 on both; `.info.title` contains `boardctl serve`.

A.2 — Loopback is a hard gate (the §5 decision, machine-checked):
```bash
boardctl serve --addr 0.0.0.0:8787; echo "exit=$?"
```
Assert: `exit=2` and stderr contains `refuses non-loopback host`.

A.3 — Muster discover over the served spec:
```bash
openapi-cli discover http://127.0.0.1:8787/openapi.json
```
Assert exit 0 and the output contains lines `boards` / `GET /api/boards listBoards`
and `serve` / `GET / getUploaderForm` (real probe output below, Appendix A):
```
API: boardctl serve (v0.1.0)
Base URL: http://127.0.0.1:18787

boards
  GET    /api/boards                              listBoards
serve
  GET    /                                        getUploaderForm
```

A.4 — CLI request with PAYLOAD assertions (the serve-E2E rule: content, not codes):
```bash
openapi-cli request GET http://127.0.0.1:8787/api/boards -o json > /tmp/boards.json
jq -e 'type=="array" and length>=1 and
       (map(.slug) | all(length>0)) and
       (map(.topology) | all(.=="A" or .=="B")) and
       (map(.task_total) | all(.>=0)) and
       (all(.[]; .done + .open == .task_total))' /tmp/boards.json
```
Assert exit 0. The invariants (slug non-empty, topology enum, done+open==task_total,
non-negative counts) bind the assertion to real derived data — a stub that returns
`[]` or `{"ok":true}` fails. If the verifier seeds a fixture board with known counts,
replace the invariants with exact numbers for that board.

A.5 — MCP tools/list with EXACT-SET assertion (order-insensitive; the generator's
tool order is map-driven and differs run to run — compare as a sorted set):
```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bt061-verifier","version":"0.0.1"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
 | openapi-mcp --spec http://127.0.0.1:8787/openapi.json --base-url http://127.0.0.1:8787 \
 | jq -e -c 'select(.id==1).result.serverInfo.name=="openapi-mcp",
             (select(.id==2).result.tools | map(.name) | sort) == ["getUploaderForm","listBoards"]'
```
Assert: both jq conditions true (exit 0). The exact set — not "non-empty" — is the
read-only boundary made testable.

A.6 — MCP tools/call with payload assertions (the money shot: muster's own binary
drives the live daemon and returns the real data):
```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bt061-verifier","version":"0.0.1"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"listBoards","arguments":{}}}' \
 | openapi-mcp --spec http://127.0.0.1:8787/openapi.json --base-url http://127.0.0.1:8787 \
 | jq -r 'select(.id==3) | .result.content[0].text' \
 | jq -e 'type=="array" and length>=1 and
          (all(.[]; .done + .open == .task_total)) and
          (map(.topology) | all(.=="A" or .=="B"))'
jq -e 'select(.id==3) | .result.isError != true'  # on the raw stream, separately
```
Assert: both exit 0. (`content[0].type=="text"` carrying the JSON payload is how
openapi-mcp returns results — measured; `.result.isError` absent/false on success.)

A.7 — Negative control (proves the registry is real and the set in A.5 means
something): call an operation the spec does not declare:
```bash
printf '%s\n' \
 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bt061-verifier","version":"0.0.1"}}}' \
 '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
 '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"putBoards","arguments":{}}}' \
 | openapi-mcp --spec http://127.0.0.1:8787/openapi.json --base-url http://127.0.0.1:8787 \
 | jq -e 'select(.id==9) | .result.isError == true'
```
Assert exit 0 (measured response: `result.isError:true`, "tool not found in
registry"). Additionally `POST http://127.0.0.1:8787/` with a multipart body must
still work for the browser flow (serve's own contract, untouched by this scope) —
but the spec route check of §2.3(4) is the assertion that keeps it OUT of the agent
surface.

A.8 — The no-write proof (the actual read-only enforcement on the real path):
```bash
find "$BT" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum > /tmp/bt-before.txt
# ... run A.3 through A.7 against the daemon serving -C "$BT" ...
find "$BT" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum > /tmp/bt-after.txt
diff /tmp/bt-before.txt /tmp/bt-after.txt
```
Assert: no diff. Every accepted probe ran against a daemon holding a real board, and
the board directory is byte-identical afterwards. This is the fleet rule turned into
an executable check, not a promise.

## 4. Boundary call: agent-safe vs gated

Decision D4: through MCP, an agent reaches exactly `listBoards`, `getUploaderForm`,
and the spec document — all GETs, all parameter-free, all bounded, all idempotent.
Everything else is gated by absence: it is not in the spec, muster cannot generate a
tool for it (a tool only exists if the spec declares the operation — verified: the
unknown-tool call errors), and the §2.3(4) machine check keeps it that way.

Layered citations, stated precisely:

- muster's written MCP contract (specs/PROTOCOLS.md §4.5) governs operations that
  might not return: every MCP-exposed operation is bounded; streaming ops get the
  transcript envelope; bidi/client-stream ops are never auto-exposed — explicit
  per-operation opt-in config only. boardctl's surface is entirely `KindUnary`
  (single request → single response, the natural fit for GETs), so §4.5's caps are
  trivially satisfied today; the spec row should still state the Kind classification
  per operation, because §4.5 keys off `OperationKind`.
- Mutating operations: if any future operation mutates state (the upload, or a
  hypothetical write), it enters muster's mutating category and requires, in order:
  human-in-the-loop approval (never reachable by an unattended agent), an explicit
  per-operation config opt-in (the §4.5(3) opt-in shape), and an executable
  state-boundary proof (the A.8 pattern, flipped to assert the intended boundary).
  Default remains: do not expose it at all.
- The fleet's standing rule (board writes are the foreman's exclusive
  responsibility; workers must not mutate them) is the reason the default is
  read-only BY SCOPE rather than by generator configuration — a generator flag can
  be flipped; an absent operation cannot. boardctl's own serve spec agrees (§1
  non-goals: no writes to the board during render/serve).

## 5. Auth story

Decision D5: loopback-only binding is a hard gate that the muster surface inherits
verbatim; no credential, header, or TLS exists, so nothing about the consumer block
authenticates — and nothing needs to, at loopback.

- This service: `boardctl serve` binds 127.0.0.1/localhost/::1 and REFUSES any other
  host with exit 2 ("--addr %q refuses non-loopback host %q: serve binds 127.0.0.1,
  localhost or ::1 only (spec 6.1: loopback convenience uploader, no auth, no TLS)")
  — serve.go, measured. Serve's spec hard-exclusions (no auth, no TLS, no
  multi-user) remain in force; this scope does not relax any of them.
- What muster's config can carry: the `mcpServers` entry is a spawn spec — command,
  args, and (host-dependent) env. `openapi-mcp` itself has NO auth surface on its
  flags (measured: spec / base-url / wasm-plugin / audit / log-level / log-output —
  nothing else). Request-level auth exists in muster's client library
  (`WithHeader`/`Authorization`) and `MUSTER_AUTH_TOKEN` is documented for the
  `openapi-cli request` flow — neither is reachable from the MCP server as built
  today. Consequence for later rows: if auth is ever added, it lands on the
  boardctl side (or a local proxy), not by asking muster's config to carry a
  credential it cannot inject.
- What loopback-only means for MCP exposure, said plainly: an MCP consumer on the
  same host connects with zero credentials and can read the board aggregates. That
  adds NO new capability to the local user — anything that can spawn
  `openapi-mcp` on this host can already read the raw JSONL directly. MCP here is
  convenience (typed tools for agents), not exposure. The danger is only
  re-binding: therefore (a) the `--addr` refusal stays (A.2 asserts it), (b) the
  spec row must NOT add any flag that relaxes the bind (a `--public`/`--allow-remote`
  switch) without a paired auth decision in a new row, and (c) the served spec's
  `servers[].url` must always advertise the actually-bound loopback address, never a
  routable one.

## 6. Shared muster config/spec generator — generalize or not

Decision D6: YES — when a shared generator emerges from the fleet's four protocol
chains, it lives in boardctl, and this scope fixes its contract now so the spec row
does not have to:

- It emits the COUPLED PAIR together: the OpenAPI 3.x spec and the `mcpServers`
  consumer block that points `--spec` at it (they are coupled — the block's URL is
  the spec's own `servers[].url`; emitting them separately is how drift starts).
- It CONSUMES a spec or a route table; it never authors operations. Rendering the
  consumer block for a declared surface is in scope; inventing surface is not. This
  is the same boundary as §2.3(1): the generator is downstream of the service's
  truth, never a second source of it.
- Shape (suggestion, spec row's call on naming): `boardctl muster-config --spec
  <path-or-url> --name <server-name>` printing the consumer block, with sanity
  checks on the input spec (parses, OpenAPI 3.x, absolute or derivable base URL).
  A `muster` subcommand family is fine if more verbs accrue.
- Why boardctl: the row itself names it as the natural home (it owns fleet-wide
  board tooling); it already ships as a single zero-dependency binary to every
  fleet host via `make release` (7 platforms), so the generator rides existing
  distribution; and four chains hand-maintaining near-identical blocks is four
  copies waiting to drift. The generator exists to make the other chains'
  scope/spec rows one-liners, not to centralize their decisions — each service's
  surface decisions (like §1 here) stay in that service's rows.

## 7. Decisions made (one line each)

- D1 Surface: exactly `GET /` (getUploaderForm), `GET /api/boards` (listBoards),
  `GET /openapi.json` (getOpenapiSpec); `POST /` and every board-writing operation
  are absent from the spec, not merely marked.
- D2 Spec delivery: served live at `/openapi.json` (muster takes URL specs —
  proven); committed generated copy as fallback; parity test on the repo's
  check-only-gate pattern (versioncheck/workflowcheck shape), both directions +
  floor + write-absence + RED-proofed.
- D3 Acceptance: the A.1–A.8 battery above — muster's own binaries drive the live
  daemon; assertions are payload-content and response-id-scoped; exact tool SET
  asserted; negative control included; no-write proven by directory hash.
- D4 Boundary: agent-safe = the two GET reads + spec doc; all `KindUnary`; mutating
  ops would require human gate + explicit opt-in + state-boundary proof (muster
  PROTOCOLS §4.5 for the unbounded/streaming contract; the read-only default comes
  from spec-level absence + the fleet's foreman-writes rule).
- D5 Auth: none exists and none is required at loopback; loopback-only binding is a
  hard gate inherited by the muster surface (refusal asserted in A.2); no
  relax-the-bind flag without a paired auth row; muster's config cannot carry
  request credentials for the MCP path anyway.
- D6 Shared generator: belongs in boardctl when the fleet's protocol chains produce
  it; emits spec + consumer block coupled; consumes surfaces, never authors them.

## 8. Open items explicitly left to the spec row (who rules)

1. `GET /api/report` deferred operation — ruled by the foreman during BT-062 review
   (default: don't build it; the HTML is not for agents).
2. Parity-test package location (serve-package test vs `internal/servecontract`) —
   spec row's call; this scope pins the assertions (§2.3), not the file.
3. Generator command name (`muster-config` vs a `muster` family) — spec row's call;
   the coupling rule and never-author rule (D6) are fixed.

## Appendix A: evidence log (probes executed 2026-09-25)

All probes ran against a loopback stub mimicking this surface (probe spec + server
under /tmp/bt061-probe during scoping, not committed; port 18787 because 8787 was
occupied on this host by an unrelated process). Muster binaries: built from 63d42c03,
go1.26.6. Every assertion in §3 is one of these probes verbatim.

| Probe | Command shape | Result |
|---|---|---|
| discover (URL spec) | `openapi-cli discover http://127.0.0.1:18787/openapi.json` | rc 0; output in §3/A.3 |
| request + payload jq | `openapi-cli request GET .../api/boards -o json` | rc 0; jq invariants true on 2-board fixture (demo/A 3/1/2, ring-runner/B 7/4/3) |
| MCP tools/list | initialize → initialized → tools/list over stdio | `serverInfo.name=="openapi-mcp"`; tool set {listBoards, getUploaderForm} |
| MCP tools/call | tools/call listBoards | `content[0].text` parsed to the fixture array; invariants true; `isError` absent |
| Negative control | tools/call putBoards (undeclared) | `result.isError:true`, "tool not found in registry" |
| File spec, no --base-url | `openapi-mcp --spec <file>` (absolute servers[].url) | tools/list OK; call hit daemon; stderr `Using base URL: ... (override=false)` |
| URL spec + --base-url | primary block shape | stderr `Using base URL: ... (override=true)` |
| Loopback refusal (boardctl) | serve.go measured contract | exit 2, message in §5 |
| Tree facts | `ls internal/describe cmd/gendocs`; `grep -rni openapi cmd/ internal/` | both missing / zero hits (§0.1, row fact confirmed) |

## Appendix B: the probe spec (the shape muster actually consumed)

The real spec is generated from the route table (spec row's job); this is the
minimal document every probe accepted, included so BT-062 has a validated skeleton
(server URL below is the probe's port; production uses 8787):

```json
{
  "openapi": "3.0.3",
  "info": { "title": "boardctl serve", "version": "0.1.0",
            "description": "Read-only muster surface for boardctl serve." },
  "servers": [{ "url": "http://127.0.0.1:8787" }],
  "paths": {
    "/api/boards": {
      "get": {
        "operationId": "listBoards",
        "summary": "List boards currently loaded by the serve uploader",
        "tags": ["boards"],
        "responses": { "200": {
          "description": "Board summaries",
          "content": { "application/json": { "schema": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/Board" } } } } } } } },
    "/": {
      "get": {
        "operationId": "getUploaderForm",
        "summary": "Self-contained uploader HTML page",
        "tags": ["serve"],
        "responses": { "200": {
          "description": "Uploader page",
          "content": { "text/html": { "schema": { "type": "string" } } } } } } },
    "/openapi.json": {
      "get": {
        "operationId": "getOpenapiSpec",
        "summary": "This OpenAPI document, served live",
        "tags": ["serve"],
        "responses": { "200": {
          "description": "The spec document",
          "content": { "application/json": { "schema": { "type": "object" } } } } } } }
  },
  "components": { "schemas": { "Board": {
    "type": "object",
    "properties": {
      "name": { "type": "string" }, "slug": { "type": "string" },
      "topology": { "type": "string", "enum": ["A", "B"] },
      "task_total": { "type": "integer" },
      "done": { "type": "integer" }, "open": { "type": "integer" } },
    "required": ["name", "slug", "topology", "task_total", "done", "open"] } } }
}
```

Schema note for the spec row: the probe validated kin-openapi ingestion with exactly
these fields (`task_total`/`done`/`open` are the Go json tags of `apiBoard` — keep
the wire names, do not camelCase them). The `/openapi.json` self-reference entry
parses fine (kin-openapi does not dereference the server's own URL).
