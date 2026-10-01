# BT-063 — muster surface E2E proof (2026-10-01)

This row's deliverable was the LIVE proof, not new code: muster's own binaries
(`openapi-cli`, `openapi-mcp`, built from source at `~/muster`, private repo
`github.com/wojons/muster` — build with `make build-cli build-mcp`, binaries land in
`~/muster/bin`) ingesting `docs/muster/openapi.yaml` and driving a live
`boardctl serve`. Reproduce with the exact commands below.

## Environment

- boardctl: v0.1.10 (`~/.local/bin/boardctl`), serve on `127.0.0.1:18924`
- fixture board: `/tmp/bt063/board` (topology-A dir: `board.jsonl` header +
  `tasks.jsonl` (2 rows) + `events.jsonl` (1 row))
- spec: `docs/muster/openapi.yaml` copied to `/tmp/bt063/openapi.yaml`

## 1. Spec ingestion + CLI generation

```
$ openapi-cli discover /tmp/bt063/openapi.yaml
API: boardctl serve (v0.1.0)
Base URL: http://127.0.0.1:8787

boards
  GET    /api/boards                              listLoadedBoards
uploader
  GET    /                                        getUploaderForm
  POST   /                                        uploadBoardReport

$ openapi-cli generate /tmp/bt063/openapi.yaml --base-url http://127.0.0.1:18924
✓ Parsed spec (version: 0.1.0)
✓ Generated 2 command groups
✓ Commands persisted to storage
✓ Registered command: boards
✓ Registered command: uploader
```

## 2. Typed CLI verbs execute against the live service (real data)

```
$ openapi-cli list-loaded-boards --base-url http://127.0.0.1:18924
name                       slug                       topology  task_total  done  open
boardctl-serve-1714143778  boardctl-serve-1714143778  B         2           2     0

$ openapi-cli get-uploader-form --base-url http://127.0.0.1:18924 -r | head -3
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
```

Both verbs drove the live serve (HTTP 200, real payload). Note: generated verbs
default to the spec's absolute `servers[].url` (8787); pass `--base-url` (or
`MUSTER_BASE_URL`) for any other port.

## 3. MCP tools listed and a READ tool returns real data

stdio JSON-RPC against `openapi-mcp --spec /tmp/bt063/openapi.yaml
--base-url http://127.0.0.1:18924`:

```
tools/list →
  ["getUploaderForm","uploadBoardReport","listLoadedBoards"]

tools/call {name: "listLoadedBoards", arguments: {}} →
  [{"done":2,"name":"boardctl-serve-1714143778","open":0,
    "slug":"boardctl-serve-1714143778","task_total":2,"topology":"B"}]
```

## 4. Mutation boundary (the load-bearing negative case)

The muster spec contains exactly the three serve HTTP operations
(quickstart.md "What the contract describes"). Board-write verbs
(`create`, `update`, `event`, `header`, `import`) are CLI-local with NO
HTTP route, so no tool can exist for them — spec-level exclusion IS the
gate (scope row §1.2). Proven live:

```
tools/call {name: "createTask", arguments: {}} →
  {"isError":true,
   "structuredContent":{"error":{"class":"client_error",
     "code":"unknown_operation","message":"tool \"createTask\" not found in registry",
     "details":{"tool":"createTask"}}}}
```

`uploadBoardReport` is the one state-touching op intentionally exposed
(interactive uploader work, documented in quickstart.md); it never writes
board files — consistent with the scope row's three-reason carve-out.

## Notes for a verifier

- muster is private: `GOPRIVATE=github.com/wojons/muster` + GitHub auth needed
  to `go build` it; the built binaries in `~/muster/bin` were used here.
- Fixture boards must be uploaded as raw `files` parts (task+events JSONL);
  a zipped `board.zip` with a `.coding-hermes/` prefix registered no board —
  zip-layout sensitivity worth a follow-up row if zips are the documented path.
