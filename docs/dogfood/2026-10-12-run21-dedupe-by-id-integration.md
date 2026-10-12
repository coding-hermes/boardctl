# Run 21 — dedupe-board `--by-id` end-to-end (2026-10-12)

**Tick:** boardctl-dogfood-2026-10-12-22-27-24 · **binary:** HEAD build (9113ec2), `dedupe-board` source-only entrypoint built from `cmd/dedupe-board` · **angle:** the surface no prior run exercised — the README's recycled-id repair (`--by-id`/`--json`/`--apply`), documented 10-12 by DOC-30/31 but only ever dry-run in default fingerprint mode (run 15, 2026-09-22).

## The real-user workflow

A lane hygiene incident produces boards where DISTINCT findings share ONE recycled
task id. The README (lines 833-872) documents the repair: dry-run first, then
`--apply`, JSON for machines. Run against live fleet boards (all read-only
dry-runs on live trees; the apply leg ran on a scratch copy, never a live board):

| board | rows | recycled ids | groups | result |
|---|---|---|---|---|
| asce | 153 | 0 | 0 | clean |
| ai_plays_poke | 265 | 0 | 0 | clean |
| heading | 6 | 0 | 0 | clean |
| crier | 566 | 0 | 0 | clean |
| warpfs | 369 | 0 | 0 | clean |
| hivemind | 121 | 0 | 0 | clean |
| bunker | 722 | 4 | 0 — all later lines already terminal | "nothing to do" (self-contradictory, DF-BOARDCTL-32) |
| lore | 73 | 4 | 1 actionable + 3 already-terminal | applied on scratch |

## What the apply proved (scratch copy of lore board)

- Exactly ONE line changed (72), zero unexpected diffs vs the pre-apply board
  (byte-diff of the full file). Closed row: `status=complete`,
  `worker_summary="superseded-by-earliest: recycled task id — kept line 63
  (id-dupe backfill 2026-10-12)"`, `superseded_by=RELEASE-LORE-003`,
  `completed_at` set. Original title/reasoning bytes untouched on the row.
- One `audit` event appended (id 185, actor `boardctl`) whose detail quotes
  status_was/closed_line/kept_line verbatim.
- Idempotent: second `--apply` run reports "nothing to do", writes nothing.
- The row's finding text lived in a custom field (`releng_note`); the audit
  event quotes title/reasoning only (both empty) — the finding text is NOT in
  the event log for rows that carry their text in a non-standard field. Folded
  into DF-BOARDCTL-31's fix direction scope; the preserved-in-events contract
  (README: "the finding text stays recoverable from the event log") does not
  hold for this row shape.

## What validate says afterward (DF-BOARDCTL-31)

`boardctl validate` on the repaired board still exits 1 with the SAME five
duplicate-id errors as before the repair — plus the pre-existing event-id
error. The repair is invisible to the validator that motivated it, and any
`boardctl install --lint-mode enforce` hook permanently blocks commits on a
correctly-repaired board. Per iddupe.go's own comment (line 19-23), breaking
validate's permanent red was the tool's reason to exist.

## Fresh-machine install leg (bunker-mvp, ephemeral agent boardctl-df1012)

- Documented path tested: source-only second entrypoint, `go install ./cmd/dedupe-board`
  from a staged module tree (per-file ssh transfer, 143 files, go.mod/go.sum
  sha256-verified both sides). Cold install **21.7s** (curl Go 1.26.9 toolchain
  + extract + install), zero preinstalled Go, no sudo.
- Missing `events.jsonl` → precise error ("tasks.jsonl-only board: events.jsonl
  is missing") — good error UX, caught by accident before the fixture was
  transferred.
- Same apply on the ORIGINAL pre-apply lore board on the fresh box: tasks.jsonl
  byte-identical to the local applied copy except the two volatile timestamp
  fields; audit event identical modulo timestamp. Transfer fidelity + engine
  determinism proven cross-host.
- Agent destroyed, SSH key removed, verified gone.

## Performance (Step 2b)

- `dedupe-board --by-id` dry-run, 566-row board (crier): **17.5ms ±0.9**
  (hyperfine, 20 runs, -N). Cold ≈ warm (single-shot wall 0.00s + 1MB RSS).
- `boardctl validate` on the same board: 42.0ms ±1.9 (hyperfine -i, 10 runs —
  exit 1 is contractual on that board's pre-existing event-id error).
- Nothing a user feels; no PERF rows filed.

## Verdict

**SHIPPABLE for dedupe-board** (per-surface: the repair engine itself is
correct, deterministic, idempotent, fast, and installs clean on a fresh box;
the validator integration and census reporting around it are rough — rows
DF-BOARDCTL-31/32/33). Rows filed with boardctl itself on its own board
(self-hosting proof, 20th consecutive run), committed b55d305, pushed and
verified via merge-base.
