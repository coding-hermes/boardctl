# Verdict: BT-061

**Task:** SCOPE: connecting to boardctl over the muster protocol
**Evaluated:** 2026-09-25T21:54:41.972975
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.009s
- ✓ **tier2**
  - COMPLETE
  ✓ A written scope doc (no code) in the worktree covering all five row items: (1) exposed surface and explicit exclusions with read-only-by-default boundary call, (2) the verbatim mcpServers consumer config block with real --spec and --base-url, (3) acceptance test commands with non-vacuous assertions, (4) agent-safe vs gated operations per muster MCP safety contract, (5) auth story; foreman lands the decisions on the board row; no production code changes: docs/muster-protocol-scope.md (469 lines) is the only file changed in commit 44fe805 (git diff 5737be9..HEAD --name-only = docs/muster-protocol-scope.md only; no production code). (1) §1.1 exposed-surface table (listBoards GET /api/boards, getUploaderForm GET /, getOpenapiSpec GET /openapi.json) + §1.2 explicit exclusions (POST /, all board-write verbs); read-only-by-default boundary call in §4 D4: 'read-only BY SCOPE rather than by generator configuration — a generator flag can be flipped; an absent operation cannot.' (2) §2.1 verbatim mcpServers block with real --spec http://127.0.0.1:8787/openapi.json and --base-url http://127.0.0.1:8787 (plus §2.2 file fallback). (3) §3 A.1–A.8 acceptance commands; I executed the jq assertions: A.4 fails on [] stub (exit 1) and on done+open!=task_total (exit 1), passes on valid data (exit 0); A.5 exact-set assertion fails when an extra tool is present (exit 1) and passes on the exact set (exit 0); A.7 negative control fails on isError:false (exit 1) — all non-vacuous. (4) §4 cites muster PROTOCOLS.md §4.5 (KindUnary, streaming/bidi caps, mutating ops require human gate + opt-in + state-boundary proof) and gates everything else by spec-level absence. (5) §5 auth story: loopback-only hard gate, no credential/TLS; quoted refusal string matches cmd/boardctl/serve.go:94 verbatim. Foreman landed decisions D1–D6 in the board row: tasks.jsonl BT-061 foreman_note = 'Decisions D1-D6 landed: D1 read-only surface; D2 served spec...; D3 consumer block verbatim; D4 agent-safe = GET-only...; D5 auth = loopback-only...; D6 generator generalizes ONLY if a second fleet chain adopts muster'. Doc's factual corrections verified against tree (no internal/describe or cmd/gendocs; no openapi code; routes() at serve.go:378; apiBoard at serve.go:741). Test suite green: `go test -count=1 ./...` exit 0 (all packages ok).


## Summary

Judge Result: BT-061

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.009s

Stage tier2: PASS
  COMPLETE
  ✓ A written scope doc (no code) in the worktree covering all five row items: (1) exposed surface and explicit exclusions with read-only-by-default boundary call, (2) the verbatim mcpServers consumer config block with real --spec and --base-url, (3) acceptance test commands with non-vacuous assertions, (4) agent-safe vs gated operations per muster MCP safety contract, (5) auth story; foreman lands the decisions on the board row; no production code changes: docs/muster-protocol-scope.md (469 lines) is the only file changed in commit 44fe805 (git diff 5737be9..HEAD --name-only = docs/muster-protocol-scope.md only; no production code). (1) §1.1 exposed-surface table (listBoards GET /api/boards, getUploaderForm GET /, getOpenapiSpec GET /openapi.json) + §1.2 explicit exclusions (POST /, all board-write verbs); read-only-by-default boundary call in §4 D4: 'read-only BY SCOPE rather than by generator configuration — a generator flag can be flipped; an absent operation cannot.' (2) §2.1 verbatim mcpServers block with real --spec http://127.0.0.1:8787/openapi.json and --base-url http://127.0.0.1:8787 (plus §2.2 file fallback). (3) §3 A.1–A.8 acceptance commands; I executed the jq assertions: A.4 fails on [] stub (exit 1) and on done+open!=task_total (exit 1), passes on valid data (exit 0); A.5 exact-set assertion fails when an extra tool is present (exit 1) and passes on the exact set (exit 0); A.7 negative control fails on isError:false (exit 1) — all non-vacuous. (4) §4 cites muster PROTOCOLS.md §4.5 (KindUnary, streaming/bidi caps, mutating ops require human gate + opt-in + state-boundary proof) and gates everything else by spec-level absence. (5) §5 auth story: loopback-only hard gate, no credential/TLS; quoted refusal string matches cmd/boardctl/serve.go:94 verbatim. Foreman landed decisions D1–D6 in the board row: tasks.jsonl BT-061 foreman_note = 'Decisions D1-D6 landed: D1 read-only surface; D2 served spec...; D3 consumer block verbatim; D4 agent-safe = GET-only...; D5 auth = loopback-only...; D6 generator generalizes ONLY if a second fleet chain adopts muster'. Doc's factual corrections verified against tree (no internal/describe or cmd/gendocs; no openapi code; routes() at serve.go:378; apiBoard at serve.go:741). Test suite green: `go test -count=1 ./...` exit 0 (all packages ok).


Overall: PASS ✓
