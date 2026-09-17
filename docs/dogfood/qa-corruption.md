# QA-BOARDCTL-001 — corruption coverage for boardctl (replacing the vacuous chaos cell)

Status: DONE (boardctl-owned slice). Date: 2026-09-17. HEAD at start: 2c96df1.

## Background: why the old cell was vacuous

The QA chaos-corruption cell truncated `dagger.db` in the project workdir,
believing it was project state. It is not:

- `dagger.db` (+ `-wal`/`-shm`) is the **live dagger-engine run store** —
  untracked, held open by a running engine, `integrity_check ok`.
- boardctl **never reads it**. Board state is exactly four JSONL files under
  `.coding-hermes/board/`: `tasks.jsonl`, `events.jsonl`, `board.jsonl`
  (topology-A header), `fixtures.jsonl`. Truncating `dagger.db` therefore
  proved nothing about boardctl, and any "corruption detected" verdict from
  that cell was fabricated by the harness, not by the tool.

## What this slice delivers

`cmd/boardctl/qa_corruption_test.go` — repeatable, isolated CLI regression
coverage. Every board is bootstrapped through the real CLI paths (`init`,
`create`, `event`) inside `t.TempDir()`; every assertion drives the real
`run()` entrypoint (no fake validator). A **dagger.db decoy** is planted in
each repo root to mirror the incident shape and prove it is ignored.

| Test | What it proves |
|---|---|
| `TestQACorruption` | Table subtests corrupt exactly ONE named file (tasks / events / board header / fixtures jsonl on the full-matrix runs) to a truncated NONEMPTY row (premise-asserted: not valid JSON): validate exits 1, diagnostic names the target file, `RESULT: FAIL` printed, no panic, no `dagger.db` mention; snapshot hashing proves ZERO writes after mutation; restoring the original bytes returns validate to exit 0 and hashes to the clean baseline. |
| `TestQACorruptionFixturesDetectedViaReader` | A truncated `fixtures.jsonl` IS detectable with exit 1 + target-file diagnostic through the real reader path (`show <registry-fixture-id>`), plus the same zero-write / restore / decoy contract. NOT asserted via validate — see the known gap below. |
| `TestQACorruptionCleanBaseline` | Clean board + dagger.db decoy: validate exits 0 (`RESULT: OK`), performs zero writes, decoy byte-identical and never mentioned. |
| `TestQACorruptionNoWriteAssertionDetectsWrites` | The byte-preservation assertion is load-bearing: a single appended byte to one board file is caught by the hash comparator (would-be-"repairing" writer is detectable). |

Byte-preservation covers all four board files **and** `dagger.db`,
`dagger.db-wal`, `dagger.db-shm` in one comparator; restoration (the last
step of each corruption case) returns the board to clean — detection, never
destruction. Tests are not parallel (in-process stdout/stderr capture) and
run under `-short`.

Invocation:

```bash
go test ./cmd/boardctl -run 'TestQACorruption' -count=1 -v
```

Result at commit time: 4 tests / 6 subtests (3 corruption cases + 3
top-level), all PASS in ~0.05s. Full gates: `go build ./...`,
`go vet ./...`, `go test ./... -count=1 -short`,
`make fmt-check`, `make version-check` — all green (see the task report for
the recorded output).

## Live evidence (real binary, /tmp scratch board, real corruption)

Corrupt `tasks.jsonl` (first line truncated mid-JSON), then:

```
$ boardctl -C <scratch> validate
board: /tmp/.../.coding-hermes/board (topology A)
rows: 0 tasks, 2 events, 1 fixtures, header parsed
[error] tasks.jsonl: line 1: unexpected EOF
RESULT: FAIL (1 error(s), 0 warning(s))
boardctl: validation failed        # exit 1
```

Restoring the original bytes returns `RESULT: OK`, exit 0. The decoy
`dagger.db` never appears in any output and stays byte-identical.

## Known gap DISCOVERED by this slice (NOT fixed here — foreman-owned)

**`boardctl validate` silently passes a truncated `fixtures.jsonl`.**
`validateFixtures` (internal/board/validate.go) calls `IterParsed` but never
captures its returned error — the subsequent `if err != nil` re-checks the
outer `ReadJSONLLines` error, which is nil at that point. Sibling functions
`validateTasks` / `validateEvents` both capture `ierr := IterParsed(...)` and
report it. Live evidence: with ONLY `fixtures.jsonl` corrupted, validate
counts `0 fixtures` and prints `RESULT: OK`, exit 0 — the corrupt file
produces no finding. The correct fix is one captured error + one
`rep.Add("error", "fixtures.jsonl: %v", ierr)`, plus the validate-side
corruption subtest moved into `TestQACorruption`. Deliberately not attempted:
this task is tests+docs only, the brief forbids production edits and commits
only the two deliverable files.

Also worth noting for the gap file: because validate silently ignores
fixtures parse errors, the `fixtures_jsonl_truncated_row` case was removed
from the validate table rather than weakened into asserting exit 0 — a test
that enshrines a silent-pass defect would be worse than no test.

## External QA-harness issues — explicitly UNRESOLVED by this slice

These are the board reasoning's external items. They are **not verified,
not fixed, and not claimed fixed** here; no SSH, no skills edits, no server
configuration changes were made:

1. **Harness derives state targets by globbing `*.db`** — corruption must
   derive state files from repo config (the `.coding-hermes/board/*.jsonl`
   set), never a glob. Unfixed in the harness.
2. **bunker-qa.sh ignores `--server`** — `BUNKER_QA_SERVER` env reportedly
   works; unverified by this slice. Unfixed.
3. **Default bunker server capacity exhausted** — allocation capacity issue
   stands. Unfixed.

Until those land, the harness-level chaos cell remains vacuous for boardctl;
the guarantee added by this slice is at the CLI level: boardctl detects
corrupted board JSONL with a targeted nonzero-exit diagnostic, never writes
during validation, and never mistakes `dagger.db` for project state.
