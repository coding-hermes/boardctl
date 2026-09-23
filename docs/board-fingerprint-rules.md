# Board finding fingerprints (SG-126)

QA and dogfood lanes used to re-file the same finding 3-8x. The lane detected
the same symptom on a new tick, picked a fresh id, recycled (or lightly
reworded) the title, and appended another row. Every create succeeded, so the
board grew while the real finding count stayed flat — operators paging the
board saw noise, and re-enable audits read the lanes as churn.

The fix is a **finding fingerprint** plus a gate on the create path.

## The rule

A finding's identity is a SHA-256 over its **normalized** title and reasoning:

```
fingerprint = sha256hex( normalize(title) || 0x1F || normalize(reasoning) )
```

`normalize()` is deliberately small and total (it never errors, so the write
gate and the backfill always agree byte-for-byte):

| step | what it does | example |
|---|---|---|
| lowercase | `strings.ToLower` | `Widget CRASH` → `widget crash` |
| strip id prefix | drops a leading recycled finding-id token (`QA-<PROJ>-N `, `DF-<PROJ>-N `, `DOGFOOD-<PROJ>-N `) | `QA-CRIER-3 [P2] Widget crash` → `[p2] widget crash` |
| collapse whitespace | every whitespace run (spaces, tabs, newlines) becomes one space | `probe:  curl returns\n500` → `probe: curl returns 500` |
| strip trailing markers | removes `(recurrence N)`, `(Nth)` and `[run-id-N]` suffixes, repeatedly | `Widget crash [run-id-7] (recurrence 2)` → `widget crash` |

The `0x1F` unit separator between the halves prevents a cross-boundary
collision: `("ab","c")` and `("a","bc")` hash differently.

Deliberately **not** normalized:

- a leading priority tag — `[P1]` and `[P3]` produce **different**
  fingerprints, so escalating a symptom is a distinct row (use `--force` when
  you want to file a variant anyway);
- object-form `reasoning` (`{"note":"QA foreman cycle …"}` on qa-dagger rows)
  is fingerprinted on its `note` member — the human-readable half — falling
  back to the canonical JSON when there is no note.

## Where it is stored

The fingerprint is **advisory metadata inside the existing `detail` field** —
the board schema is git-tracked JSONL and gains no new column:

```json
"detail": {
  "fingerprint": "9f2c…",
  "evidence": [{"run_id": "qa-2026-09-18T04:11Z", "ts": "2026-09-18T04:12:07Z"}],
  "text": "cell detail: spawn failed (original prose, when the row had any)"
}
```

`detail` keeps its original prose under `text`, and if a row's detail was
already a JSON object, every unknown key is preserved verbatim. Rows written
before this rule carry no fingerprint: they are **grandfathered** — still open
and visible, they simply never match an incoming filing (they are picked up by
the backfill below).

## Evidence over refile

When the same finding is detected again, **do not refile it**. Either:

1. run the create anyway and let the gate refuse it — the refusal is recorded
   as a `task_evidence` event on the existing row, so the re-detection is
   counted rather than lost:

   ```
   $ boardctl create --id QA-CRIER-9 --title "QA-CRIER-3 [P2] Widget crash" --reasoning "probe:  curl returns
   500"
   DUPLICATE-SUPPRESSED: QA-CRIER-3 matches fingerprint 9f2c… — the finding is already open …
   $ echo $?
   2
   ```

   Exit code **2** and a `DUPLICATE-SUPPRESSED: <existing-id> matches fingerprint
   <hex>` line on stderr make it dispatcher-detectable (a plain failure is
   exit 1). `boardctl show <existing-id> --events` lists every re-detection.

2. or, on a NEW filing only, stamp evidence explicitly with
   `--evidence-run-id` (a `create`-only flag — `update` does not accept it;
   to record evidence on an EXISTING row use `boardctl update <id>
   --summary/--note` carrying the run id):

   ```
   $ boardctl create --id QA-CRIER-9 --title "…" --reasoning "…" --evidence-run-id qa-2026-09-18T04:11Z
   ```

**A genuinely different row that happens to share a title is filed with
`--force`:**

```
$ boardctl create --id QA-CRIER-9 --title "[P2] Widget crash" --reasoning "probe: curl returns 500" --force
```

The forced variant still gets fingerprinted, so a later *accidental* re-file of
it is caught.

The gate never runs on the import path (`boardctl import`): imported rows land
byte-for-byte as exported and the export/import round trip is unchanged.

## Collapsing the duplicates that already exist

Old boards still hold their collision groups. `dedupe-board` is the one-shot
backfill:

```
# always start with the dry run — it writes nothing at all
$ dedupe-board --board-dir ~/crier
lane:  crier
board: /home/kara/crier/.coding-hermes/board/tasks.jsonl
open rows: 12 (already fingerprinted: 0, gained: 12)
merge groups: 2 (rows merged away: 3)
  9f2c1a8b0e44  keep QA-CRIER-3  merge QA-CRIER-9, QA-CRIER-11
DRY-RUN — no files written (pass --apply to collapse)

$ dedupe-board --board-dir ~/crier --lanes crier --apply
```

What `--apply` does, per collision group (≥ 2 open rows sharing a fingerprint):

- keeps the **earliest** row (append-only file order = filing order);
- closes the others with `status=complete`,
  `worker_summary="merged into <kept>: dedupe backfill <date>"`,
  `completed_at=<now>`;
- carries the merged-away rows' evidence onto the kept row (one entry per
  merged row) so the re-observation count survives;
- appends ONE `audit` event per group:
  `{"action":"dedupe-backfill","kept":"…","merged":["…","…"],"fingerprint":"…"}`.

Every other line of `tasks.jsonl` round-trips byte-identical (asserted before
the write), and a second `--apply` is a no-op.

Rails, in order of importance:

1. **One board per run.** `--board-dir` is required; there is no fleet sweep,
   so the tool can never touch a board the operator did not name.
2. **Dry-run by default.** No rows, no fingerprints, no audit events until
   `--apply`.
3. **`--lanes a,b,c` is a guard, not a target.** When given, the board's lane
   (header `project`/`namespace`, else the repo directory name) must be in the
   list or the run refuses with exit 2.

Exit codes: `0` success (dry run included), `1` runtime error, `2` usage or
lane refusal.

## Open rows

A row counts as open for dedupe when its status is `pending`, `in_progress`,
`dispatched`, `blocked`, or `review`. Anything else — `complete`, `failed`,
`duplicate`, an unknown or legacy spelling — never suppresses an incoming
filing: the tool fails open rather than silently swallowing a create.
