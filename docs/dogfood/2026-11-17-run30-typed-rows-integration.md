# Run 30 — BT-098 typed rows used for real: a silent metadata wipe and an invisible feature

- Tick: `boardctl-dogfood-2026-11-17-10-20-34`
- Target: boardctl — repo `/home/kara/coding-hermes-boardctl`, HEAD `0179f5b` (`= origin/main`, parity 0/0)
- Lane workdir: `/home/kara/.hermes/stand-in/dogfood/boardctl` (its `board` symlink resolves to the target repo's board)
- Verdict: **PROMISING-BUT-ROUGH** — the typed-row feature works and refuses bad input precisely, but one write path destroys stored data silently and the whole surface is undiscoverable from `--help` / README.
- Promise: *"a foreman can declare its own ticket types (template + required metadata), file conformant rows with `--type/--meta`, and trust that the stored metadata survives the rest of the board's verbs"* (BT-098, complete).

## What was actually used

The BT-098 surface landed since run 29 (2,290 inserted lines across `internal/board/types.go`, `write.go`, `validate.go`, `cmd/boardctl/types.go`), and no dogfood run had touched it. A scratch board was driven by a HEAD build (`go build ./cmd/boardctl`, `version dev`) exactly the way a user with a types.jsonl would:

1. `boardctl init` — writes `tasks/events/board/fixtures.jsonl`, **no `types.jsonl`**.
2. `boardctl types` → `bug chore feat task` (built-ins). `types --doctor` → `types.jsonl: absent — built-in defaults active (task, bug, feat, chore)`.
3. Authoring a custom type: **the record schema is documented nowhere** — not in `boardctl --help` (`grep -c -i types` = 0), not in `README.md` (`types.jsonl` 0 hits, `--meta` 0 hits), and `skills/boardctl-usage` names the command and one `--meta` example but never the field list. The shape was recoverable only from `internal/board/testdata/bt098-release-type.jsonl`.
4. `boardctl types` after adding a `release` type renders the template and every field:
   `release (Release) template: prefix="[RELEASE] " priority=P1 skeleton metadata: channel(enum, required: stable|beta|nightly), signoffs(list), source_commit(string, required), approval_ticket(ref)`
5. `create --id REL-1 --title 1.2.0 --type release --meta channel=stable --meta source_commit=abc1234` — template applied: title stored as `[RELEASE] 1.2.0`, priority defaulted to `P1`, `acceptance_criteria` filled from `ac_skeleton`, metadata stored under `detail.metadata`.
6. Refusals are precise and write nothing (row count unchanged after each):
   - missing required → lists both fields with their types/descriptions and the `--meta field=value` remedy, rc=1;
   - `channel=weekly` → `allowed: stable|beta|nightly`, rc=1;
   - `approval_ticket=NO-SUCH-ROW` → `task id does not exist`, rc=1;
   - unknown type `deploy` → `unknown ticket type "deploy" — configured types: …`, rc=1;
   - retagging a plain row with `update --type release` → `type "release" is not conformant; missing=…`, rc=1.
7. `--meta signoffs=a@x,b@x` → stored as a JSON array `["a@x","b@x"]` (list field works).
8. `update --status pending` and `update --note …` preserve the array byte-for-byte.

## The defect (DF-BOARDCTL-56, P1)

`update --meta channel=beta` on that same row (rc=0, reports `updated task REL-7: status, detail, updated_at`) rewrites `detail.metadata` as:

```
{"channel":"beta","signoffs":null,"source_commit":"abcdefg"}
```

The list is gone. `grep ana@x` over `tasks.jsonl` = 0 and over `events.jsonl` = 0 — the `task_updated` events carry only `{"status":"pending"}`, so the value is unrecoverable. `boardctl validate` afterwards: `RESULT: OK` — the loss is invisible to validate, show (renders `signoffs: null`), and the render/export JSON.

Root cause (`internal/board/write.go:1225-1231`): the update path decodes stored metadata into `map[string]string` and discards the error (`_ = json.Unmarshal(pd.Extra["metadata"], &values)`); a JSON array cannot decode into a string, so the key never enters `values`, and `encodeMetadata` at `:1249` then emits every declared field, writing the absent list as `null`. Same family as DF-BOARDCTL-44 (non-string values mangled on re-serialization), but total loss instead of a byte change.

## The invisible feature (DF-BOARDCTL-57, P2)

`types` is a working subcommand that appears in **neither** `boardctl --help` nor the README; `create`'s help advertises `[--meta k=v]...` with no vocabulary; the `types.jsonl` record schema exists only in Go source and testdata. A user cannot discover the feature, and cannot author a type without reading source. `types --doctor`'s "types.jsonl: absent" is the only pointer that exists, and it is unreachable from help.

## The smaller one (DF-BOARDCTL-58, P3)

`create --type bug --meta severity=high` writes silently (no warning) while the same undeclared key on `release` warns at write time — the warning lives inside the per-declared-field loop (`internal/board/write.go:1240-1242`), so a type with an empty metadata list never reaches it, and the user learns about the non-conformant row only at the next `validate`.

## Gates and the install leg (DF-BOARDCTL-59, P2)

`make cover-check` at HEAD exits 1 on this host (go1.26.11): `total 80.0% of statements (floor 80.0%)` then `FAIL total coverage 80.0% is below floor 80.0%`. The raw total from the same profile is **79.986683%** (statements=10513, covered=8409) — the `%.1f` format rounds it to the floor it fails (`internal/covercheck/covercheck.go:258` compares exactly, `:263` formats). The same step is red on the fresh JIT agent (act, step `Coverage`, 15.129s) while every other step of the same job passed — and the identical product tree passes on GitHub (`43810100006` green; `git diff --stat 20124ca..HEAD -- cmd internal Makefile .github` is empty). A floor set at the exact achieved value flips on a host, and its failure text shows two equal numbers.

## Performance (Step 2b) — nothing a user feels, no PERF row

Warm (hyperfine, host load average 15.7 on 16 cores, so upper bounds):

| verb | board (274 rows) | task-router board (546 rows) |
|---|---|---|
| `stats` | 33.9 ms ± 8.9 | 0.04 s wall |
| `ready --json` | 48.8 ms ± 26.4 | 0.05 s wall |
| `validate` | 82.9 ms ± 56.4 | 0.08 s wall |
| `types` (new) | 5.6 ms ± 3.0 | — |

Cold single run of the headline `stats`: `wall=0.02 rc=0`. Page-cache-cold is unmeasurable without root here. Nothing here is slow enough for a user to notice, so no profile was taken and no PERF row was filed — a win nobody can feel is not a finding.

## Install leg (fresh machine)

`bunker-qa.sh launch` → agent `b11919a8` on `bunker-mvp` (0.2.1, `/tmp` private, **net shared — NOT tenant isolation**), 24 evidence rows, per-cell log `ci.log` pulled by `bunker exec` before destroy, agent destroyed and its SSH key removed. Cells: `fresh-install` OK, `tests` OK (rc=0), `upgrade` OK (v0.4.0 tree → HEAD reinstall), `chaos-disconnect` OK, `chaos-resource` PASS (3G cap honoured), `chaos-errorpath` OK, **`ci-pass` FAIL** (Coverage step), `ci-act` UNVERIFIED (same act run), `lint` INFO (`golangci-lint` never provisioned on the agent), `ui-probe` INFO (no long-running server), `docker-deploy`/`chaos-shutdown` SKIPPED by the docker-lifecycle safety gate, `chaos-corruption` N/A (no db/state files). Grade: **install-proven / isolation-UNVERIFIED**.

## Time-to-first-success

`init` → `types` → first conformant typed row: about a minute *after* the `types.jsonl` shape was known; the discovery step (schema by source reading) is the real cost and is DF-BOARDCTL-57. Refusal messages are good enough that the negative arms cost zero source reading.

## Findings filed

| id | priority | finding |
|---|---|---|
| DF-BOARDCTL-56 | P1 | `update --meta` nulls stored LIST metadata silently; unrecoverable from board or events; validate stays OK |
| DF-BOARDCTL-57 | P2 | typed-row surface undiscoverable: `types` missing from `--help`, types.jsonl schema documented only in testdata |
| DF-BOARDCTL-58 | P3 | undeclared `--meta` key warns on types that declare fields, writes silently on types that declare none |
| DF-BOARDCTL-59 | P2 | `make cover-check` red at HEAD (79.9867% vs floor 80.0) with a self-contradictory failure line |

Board after filing: 278 rows (was 274), 0 duplicate ids, exactly one `task_created` event per filed id, `validate --fail-on dangling-dep` rc=0 with the **same 44-warning baseline** as the pre-write board reconstructed from `origin/main` (44 before, 44 after, no warning naming a filed id).
