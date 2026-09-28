# boardctl ↔ muster quickstart

[muster](https://github.com/wojons/muster) turns any OpenAPI 3.x spec into a
typed CLI (`openapi-cli`), an MCP server (`openapi-mcp`), a daemon, and a Go
library. This repo ships the contract muster consumes: the loopback
uploader/report HTTP surface of `boardctl serve`.

**The contract: [`docs/muster/openapi.yaml`](./openapi.yaml)** — OpenAPI
3.0.3, generated from the live route wiring (see "Generation rule" below).

## Connect over muster (consumer config)

```json
{
  "mcpServers": {
    "boardctl": {
      "command": "openapi-mcp",
      "args": [
        "--spec", "docs/muster/openapi.yaml",
        "--base-url", "http://127.0.0.1:8787"
      ]
    }
  }
}
```

## Quickstart (CLI form)

```sh
boardctl serve                       # loopback uploader + report server
openapi-cli --spec docs/muster/openapi.yaml --base-url http://127.0.0.1:8787
openapi-cli get-uploader-form        # one verb per operationId; try list-loaded-boards too
```

## The servers[] case: absolute, on purpose

The generated spec publishes `servers: [{url: http://127.0.0.1:8787}]` —
**absolute**. This is the case that applies here, and it matters:

- muster falls back to the spec's own `servers[].url` when `--base-url` is
  omitted, so the generated verbs work with no extra flags as long as
  `boardctl serve` runs on its default address;
- a **relative** `servers[].url` (e.g. `/api`) canNOT be dereferenced
  standalone from a local-file spec — muster resolves relative server URLs
  only against the absolute URL the spec itself was fetched from, which a
  local file does not have. A relative entry would strand every verb with
  an empty/relative base URL (muster fails with an actionable `--base-url` /
  `MUSTER_BASE_URL` hint).

`boardctl serve` refuses any non-loopback `--addr` host (127.0.0.1,
`localhost`, `::1` only), so the absolute loopback URL is the only honest
published stand-in. Running on another loopback spelling or port? Override
with `--base-url` (or `MUSTER_BASE_URL`, or the `mcpServers` args above) —
do not edit the spec.

The parity gate enforces the invariant: a relative `servers[0].url`, or one
that stops matching the serve default, fails `internal/speccheck`.

## Generation rule (why the spec cannot drift)

The registry (`internal/describe`) is the single source: it re-derives the
surface from the real wiring in `cmd/boardctl` — every
`mux.HandleFunc("<METHOD> <path>", …)` in `serveServer.routes()` for HTTP,
plus run()'s command switch for the CLI verbs — and `cmd/gendocs` renders
that registry as the OpenAPI document. **The spec is never hand-written or
hand-edited** (a hand-edit is overwritten by the next regeneration anyway);
there is no recorded exception on this row.

After any serve-surface change:

```sh
go run ./cmd/gendocs              # regenerate docs/muster/openapi.yaml
go test ./internal/speccheck      # parity gate: spec must match the wiring
```

## The parity rule (executable, not a comment)

`internal/speccheck` re-derives the route table from `serve.go` source on
every `go test ./...` run and fails — naming the route and both sides — on:

- a route wired in `routes()` but missing from the spec (added without
  regenerating);
- a spec operation with no matching registration (removed or renamed
  without regenerating);
- operationId drift between spec and registry, or a missing operationId
  (muster cannot name a verb for an unnamed operation);
- a relative or default-mismatched `servers[].url`;
- an unparseable spec, or a lost derivation anchor (the gate can never be
  fooled into passing by going blind).

Negative probes live in `internal/speccheck/speccheck_test.go`
(`TestCheck_FakeRouteAddedFailsNamingTheDrift` and the spec-mutation arms).
`TestSpeccheck_ParsesWithKinOpenAPI` additionally loads and validates the
published contract with muster's own parser (`kin-openapi`, pinned to
muster's version) on every test run.

Wiring points: the gate runs in plain `go test ./...` today; the
`make spec-check` / CI step shape is
`go test -count=1 -run TestSpeccheck ./internal/speccheck` (mirroring
`fmt-check`/`version-check`), left to the repo's Makefile owner.

## What the contract describes

Three operations, as `boardctl serve` actually behaves:

| operationId | route | what it is |
|---|---|---|
| `getUploaderForm` | `GET /` | the self-contained uploader HTML page |
| `uploadBoardReport` | `POST /` | multipart upload (`.zip` part or `files` folder parts) → the rendered multi-board HTML report; 512 MB cap, 50-board cap, 400/413 error surfaces |
| `listLoadedBoards` | `GET /api/boards` | JSON array of the loaded boards (`name`, `slug`, `topology`, `task_total`, `done`, `open`) |

Board files are JSONL (`tasks.jsonl` / `events.jsonl`, plus a topology-A
`board.jsonl` header). The CLI verbs (`boardctl list`, `create`, `update`,
…) are deliberately NOT in the contract: muster consumes HTTP surfaces, and
the serve HTTP API is the part a remote consumer can drive.
