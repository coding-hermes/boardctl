# Run 32 — BT-099 subtasks/comments/decomposition under real use (2026-11-26)

Tick: `boardctl-dogfood-2026-11-26-15-03-41` · HEAD `c6924e7` (= `origin/main`, parity 0/0) ·
binary under test: HEAD build (`go build ./cmd/boardctl`, version `dev`), with the deployed
release `v0.6.0` (`~/.local/bin/boardctl`) driven beside it for boundary and zero-migration arms.

Verdict: **SHIPPABLE** for the BT-099 core (comment threads, parent links, decompose,
ready/next suppression, validate coverage all hold under real use with precise refusals),
with two P2 data-shape findings on the untyped-row path and one P3 discovery finding.

## Promise

"A foreman can decompose one board row into child rows (`decompose`), discuss on rows with
append-only typed comment events (`comment`), carry decomposition metadata
(`--expected-files`, `--verify-steps`, `--parent` on create/update), and trust `ready`/`next`
to dispatch children before parents — with `validate` catching hand-edited dangling parents
and cycles, per `specs/BT-099-subtasks-comments.md` and its seven binding panel amendments."

## The working recipe (what a real user does)

```bash
boardctl -C <repo> init --project p --namespace ns            # 0.03s
# parent shell with decomposition metadata (flags work but are UNDOCUMENTED — see DF-BOARDCTL-065)
boardctl -C <repo> create --id DF-EX-100 --title "Ship the widget refactor" \
  --priority P2 --complexity 3 --capability-tags go,board \
  --expected-files "cmd/widget.go, internal/widget/render.go" \
  --verify-steps "go test ./..." --verify-steps "prove render goldens" \
  --acceptance-criteria "AC1 widget renders" --status pending
# scaffold children (default --parallel per amendment 2)
boardctl -C <repo> decompose DF-EX-100 --count 3 --first-id DF-EX-101
# -> DF-EX-101..103 created, each parent=DF-EX-100, no inter-child deps,
#    title "<parent title> — step i/3", priority/complexity/tags inherited;
#    parent gets ONE task_updated event {"decomposed_into":[ids]}
boardctl -C <repo> comment DF-EX-101 --body "taking this step first" --author bane
boardctl -C <repo> comment DF-EX-101 --body "agreed" --reply-to <event-id>
boardctl -C <repo> show DF-EX-101        # row + COMMENTS (n) thread, replies indented
boardctl -C <repo> show DF-EX-100        # CHILDREN: 3/3 open + CHILD DEPS (union)
boardctl -C <repo> ready                 # children listed; parent in WITHHELD block
boardctl -C <repo> next                  # picks a CHILD, never the withheld parent
```

Sequential mode: `decompose <id> --count 3 --sequential` chains `child_i` on `child_{i-1}`
and prints the amendment-2 cost line verbatim: `wave absorbs 1 worker (sequential chain —
use --parallel for N)`.

## Verified clean (every arm rc-checked, nothing written on refusals)

- **Comment doctrine holds**: `tasks.jsonl` md5-identical across three comments; header
  `ticks_total` NOT bumped (amendment 5); event schema `{body, in_reply_to}` exact;
  `--author` honored, default `foreman`; `--body-file` round-trips a multi-line body;
  `show` renders the thread in id order with replies indented one level; `show --json`
  emits the stable array `{event_id, ts, author, body, in_reply_to}` — and by its own flag
  help it replaces the row body (documented, not a defect).
- **All six comment refusal classes** exit 1 appending nothing (events count + MAX(id)
  unchanged): unknown task, empty-after-trim body, both body flags, neither body flag,
  `--reply-to` a non-comment event, `--reply-to` a comment on a different task. Each
  message names the id and says "nothing appended".
- **Parent-link write-time validation**: self-parent, nonexistent parent, complete parent,
  and a 2-node cycle via re-parenting all refused rc=1 with messages naming the ids /
  printing the cycle `PR-1 -> CH-1 -> PR-1`; refusals write nothing (0 rows added).
- **`validate` coverage of hand-edited rows**: dangling `parent` → `[error] tasks.jsonl
  line 3 (task VA-2): parent references nonexistent task id "VA-9"`, RESULT: FAIL rc=1;
  2-node cycle → `parent cycle detected: VA-1 -> VA-3 -> VA-1 — ... (fix by clearing one
  parent with 'boardctl update <id> --parent ""')` — the remedy is in the message.
  `--fail-on parent-cycle` rc=1.
- **Suppression in the shared engine (amendment 3)**: `ready` lists the 4 ready children,
  prints `WITHHELD (open children ready): DF-EX-100 — dispatch DF-EX-101, DF-EX-102,
  DF-EX-103 first`; `next` picks `DF-EX-101`; after all three children complete, the
  parent appears in `ready` and the withheld block shrinks to the still-open family.
- **Re-parenting / removal**: `update <id> --parent OTHER` re-points (all checks re-run);
  `--parent ""` removes the key entirely (row reads `KEY-ABSENT`), rollups recompute.
- **decompose id minting** skips occupied ids (`--first-id CH-8` with `CH-9` on the board
  minted `CH-8, CH-010, CH-011`) — the width jump is `next-id`'s own allocation rule
  (min width 3): `next-id --family CH` answers `CH-012` on the same board, so decompose
  and next-id are consistent. (Refuted before filing as a defect.)
- **decompose refusals**: complete parent rc=1 "open rows only"; `--count 0` rc=2 usage.
- **Typed-parent inheritance is perfect**: parent `REL-100` (type `release`,
  `detail.metadata={channel, expected_files, signoffs, verify_steps}`) → both children
  carry all four keys byte-identical, including the `signoffs` list.
- **DF-BOARDCTL-56 (P1, run 30) fix holds on the new arrays**: `update REL-100 --meta
  channel=stable` preserved `signoffs`, `expected_files`, `verify_steps` byte-for-byte;
  `--note` preserved them; `--expected-files new.go` REPLACED only its own array as
  documented.
- **BT-105 fix verified live**: `update EV-1 --status complete --evidence-run-id
  abc123def456 --commit-hash <sha>` → the `task_completed` event detail is
  `{"status":"complete","evidence_run_id":"abc123def456"}`.
- **DF-BOARDCTL-061 fix verified live**: `create --initiative myprog` stores the singular
  key and `validate` reports `key uniformity: 2/2 rows carry only canon keys (0 drift)`,
  RESULT: OK 0 warnings.
- **Zero migration (spec §8 T10)**: on a copy of this repo's real 300-row board, HEAD and
  v0.6.0 produce byte-identical `stats` and the same validate warning count (44, the
  standing legacy baseline); `validate` rc=0 on both.
- **Malformed `types.jsonl` fails closed**: one bad line (`display is required`) refuses
  typed creates with the remedy text and writes nothing, while untyped creates keep working.

## Findings filed this run

| id | P | summary |
|---|---|---|
| DF-BOARDCTL-063 | P2 | `decompose` silently drops `detail.metadata` (expected_files/verify_steps) for UNTYPED parents — `internal/board/decompose.go:203-207` gates inheritance on `parentType != ""` while spec §5 promises the keys unconditionally. Typed arm inherits everything; untyped arm loses everything, rc=0, no warning. Most fleet rows are untyped. |
| DF-BOARDCTL-064 | P2 | `create --meta foo=bar` on an untyped row silently drops the key (rc=0, no warning, nothing stored) while `update --meta` on the same row refuses rc=1 `row type "" is not configured` — fail-open vs fail-closed asymmetry; DF-BOARDCTL-58's fix covered typed rows only. |
| DF-BOARDCTL-065 | P3 | The BT-099 row flags `--parent`, `--expected-files`, `--verify-steps` are absent from the `--help` create/update usage blocks and README (grep: 0/0/0 hits), and the help `show` line omits `--json` — spec §7's count-pinned-surface rule was honored for the verbs but not the flags. The record schema for the flags is recoverable only from source. Same class as DF-BOARDCTL-54/57. |

## Release boundary (measured, owned by the releng lane)

The deployed release `v0.6.0` (cut 2026-11-22, before BT-099 landed 2026-11-24) refuses
both verbs: `unknown command "comment"` / `"decompose"`, rc=2. `RELEASE-2026-11-26-01`
(complete, this morning) verified the v0.7.0 candidate READY; the GitHub releases API
still lists v0.6.0 as latest at run time. A user installing per the release path gets a
binary that cannot do what main's README shows — transient by design, owned by the
release cut, no row filed (the mechanism has a completed owner row).

## Perf (Step 2b) — honest non-finding

Host at load 18.3 on 16 cores during measurement (all numbers are upper bounds;
`date +%s%N` loop timing, restore excluded for mutating verbs):

| op | board | warm |
|---|---|---|
| validate | real 300 rows | 99 ms |
| stats | real 300 rows | 82 ms |
| ready | real 300 rows | 33 ms |
| list --json | real 300 rows | 61 ms |
| validate | synthetic 5,001 rows (500 parents × 9 children) | 124 ms |
| ready | same | 93 ms |
| next | same | 152 ms |
| show (rollup) | same | 90 ms |
| next | 1,000-row depth-999 parent+depends chain | 184 ms |
| comment append | scratch | 23 ms |
| decompose --count 5 | fresh board per run | 68 ms |

Cold (fresh copy, first call): validate 55 ms, stats 40 ms, ready 32 ms. Scaling is
gentle — 16.7× the rows costs ~1.3–1.5× the time; the deliberate worst case (depth-999
cycle-walk chain) tops out at 184 ms. Nothing a user feels; no profile justified; no
PERF row. Run 31's smaller numbers (23–44 ms at load 12.7) are the same envelope at a
quieter host.

## Install leg (bunker-qa.sh, ephemeral JIT agent)

`launch` → `collect` on **bunker-mvp** (preflight passed this fire; disk admission OK),
agent `55ca1074`, target-binding `bound` (repo identity from origin remote). 23 rows
synced, `qa-run.finished`, evidence persisted at
`~/.hermes/qa/evidence/boardctl/qa-20261126T152418Z` (`EVIDENCE-SHA256=6eac13c4…`,
27 rows incl. wrapper cells). Cells: **fresh-install OK** (`go build ./...`), **lint OK**
(golangci-lint rc=0 — provisioned this fire, unlike run 30's INFO), **tests OK**
(`go test ./... -count=1`), **ci-pass OK** (act, 2 jobs green), **upgrade OK** (previous
release tree v0.5.0 via git archive → HEAD reinstall clean; note the harness picked
v0.5.0, not the latest v0.6.0, as the upgrade baseline), **chaos-disconnect OK**,
**chaos-resource PASS** (3G cap), **chaos-errorpath OK** (rc=2 on missing config);
docker-deploy + chaos-shutdown SKIPPED (safety gate: controller is the live Hermes
host), chaos-corruption N/A, ui-probe INFO (no server surface). **0 FAIL / 0
UNVERIFIED.** Agent destroyed after collect, SSH key removed, absence verified.
Grade: **install-proven / isolation-UNVERIFIED** (host reports net shared — NOT tenant
isolation, same field as runs 28–31).

## Harness traps hit this run (mine, not the product's)

- `bash probe.sh | tee file | head -120` SIGPIPE-killed the probe script mid-run at the
  ready step — the truncated output file reads like a hang. Redirect to a file and read
  the file; never put `head` on a live pipeline you still need.
- `cmd | head -2; echo rc=$?` reported head's 0 for a refusal that actually exited 1
  (re-run unpiped: rc=1). The skill's own trap list says this; it still bit once.
- `sed` against board JSONL missed because create writes compact JSON — insert keys with
  `jq -c '. + {parent:"X"}'` on scratch copies instead.
- The `types.jsonl` record schema is still only in `internal/board/testdata/` (run 30's
  DF-BOARDCTL-57 fix documented the typed-row surface in the usage skill, but my first
  hand-authored line was still refused — the refusal text `display is required` is good
  and got me to the testdata shape in one step).

## Evidence

`.coding-hermes/evidence/boardctl-dogfood-2026-11-26-15-03-41/` — probe scripts
(probe1-happy, probe2b-typed, probe3-refusals, probe4-redo, probe5-perf, file_rows.py),
their verbatim outputs, the battery ledger copy, and INDEX.md.
