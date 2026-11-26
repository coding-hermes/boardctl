# Evidence index — boardctl dogfood run 32

Tick: `boardctl-dogfood-2026-11-26-15-03-41` (first and only delivery; gateway inbound
census = 1 distinct timestamp). Measured at HEAD `c6924e7` (= `origin/main`, parity 0/0).
Scratch origin: `/home/kara/.hermes/cache/scratch/df-run32/` (prunable; every artifact
that carries a claim is copied here).

## Probe scripts (committed so the next fire can re-run the arms verbatim)

| file | what it proves |
|---|---|
| `probe1-happy.sh` | BT-099 happy path: init, parent with `--expected-files`/`--verify-steps`, decompose parallel + sequential (cost line), comment thread + reply, tasks.jsonl md5 invariance, header tick invariance, show rollups, ready/next suppression. NOTE: the original run was piped through `head -120`, which SIGPIPE-killed it at the ready step (`probe1.out` is truncated there — harness trap, disclosed in the report); ready/next/complete-the-children/validate were re-run directly and in `probe3-refusals.sh` (probe1 tail section). |
| `probe2b-typed.sh` | Typed vs untyped decompose inheritance contrast (DF-BOARDCTL-063), `show --json` shape, DF-BOARDCTL-56-class `update --meta` survival on the new arrays, untyped `create --meta` silent drop vs `update --meta` refusal (DF-BOARDCTL-064), undeclared-key warning on typed create, `--parent` re-point + removal. |
| `probe3-refusals.sh` | Comment refusal battery (6 classes, events count + MAX(id) invariance), parent-link refusals (self/missing/complete/2-node cycle), decompose refusals (complete parent rc=1, `--count 0` rc=2), BT-105 completion-event evidence id, probe1 tail (children complete → parent released), DF-BOARDCTL-061 re-proof, release-boundary arm (v0.6.0 refuses comment/decompose rc=2), zero-migration arm (HEAD vs v0.6.0 on the real 300-row board). |
| `probe4-redo.sh` | Re-runs of the two arms probe3 mis-measured: bare-comment true rc (1, unpiped), `--body-file` multi-line round-trip, next-id width parity (CH-012 — refutes the width-drift hypothesis), validate dangling-parent + 2-node-cycle via jq-inserted keys (sed missed compact JSONL), doctor. |
| `probe5-perf.sh` | Step 2b perf: 5,001-row parent/child synthetic board + 1,000-row depth-999 chain generated inline (python inside the script), warm averages via `date +%s%N` loops with restore excluded, cold first-call arms. |
| `file_rows.py` | The filing writer: argv-list `boardctl create` for DF-BOARDCTL-063/064/065 with id-free precheck and post-write row+paired-event verification. |

## Outputs (verbatim stdout+stderr)

`probe1.out` (truncated at ready by the SIGPIPE trap above), `probe2b.out`, `probe3.out`,
`probe4.out`, `probe5-perf.out`, `validate-303-rows.out` (post-filing board validate:
RESULT OK, 44 standing warnings, rc=0).

## Install leg (bunker-qa.sh)

- `battery-qa-evidence.jsonl` — the full synced ledger copy (agent `55ca1074` @
  bunker-mvp, 23 agent rows; wrapper adds target-binding/launch/collect cells).
  Durable copy: `~/.hermes/qa/evidence/boardctl/qa-20261126T152418Z/`
  (`EVIDENCE-SHA256=6eac13c45d9cfe330120b943b02cd4f6744a1c8a99e0e5e244883425ed2d7b32`,
  27 rows incl. wrapper).
- `battery-cells.txt` — human-readable cell|verdict|detail listing.
- Grade: install-proven / isolation-UNVERIFIED (host net shared — NOT tenant isolation).
  0 FAIL / 0 UNVERIFIED. Agent destroyed post-collect, SSH key removed, absence verified
  in `bunker list`. Foreign agents on the host (ci-e2e-01, 746162f3) left untouched.

## Transformations

None beyond copying: outputs are byte-identical to the scratch originals; no elision was
needed (no credentials, no private-IP literals; agent home paths `/home/bunker-55ca1074/`
are the battery's own synthetic sandbox paths, committed in runs 30/31 the same way).

## Rows filed this tick

DF-BOARDCTL-063 (P2), DF-BOARDCTL-064 (P2), DF-BOARDCTL-065 (P3) — each with one paired
`task_created` event; board 300 → 303 rows; `boardctl validate --fail-on dangling-dep`
rc=0 RESULT OK (44 warnings = standing legacy baseline, none naming a filed id).
