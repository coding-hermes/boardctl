# boardctl run 25b — concurrent-rewrite data loss (2026-10-23)

Second delivery of tick `boardctl-dogfood-2026-10-23-10-52-34`. The first
delivery covered `normalize-board --repair-events` under real use (rows
DF-BOARDCTL-44..47, `docs/dogfood/2026-10-23-run25-repair-events-integration.md`).
This note records the angle that delivery did not open: **what happens when a
repair runs on a board another process is writing**. Rows DF-BOARDCTL-48..50.

Target: `coding-hermes-boardctl` HEAD `900b7b5` (built `dev`) and the released
`v0.1.11` asset, driven side by side.

## Promise tested

README:1256-1259: *"every write verb takes an exclusive advisory flock on
`.coding-hermes/board/.lock` for its whole check-then-act window, so
simultaneous writers cannot interleave appends … or double-file the same task
id."* `internal/board/lock.go:17-19` adds that the guard also protects
read-modify-write rewrites, because two writers rewriting from one snapshot
would "silently drop the winner's change".

That is exactly the claim a fleet operator relies on: lanes append to a board
while someone runs `normalize-board --apply` on it.

## What a real user hits

### 1. `normalize-board --apply` deletes a row created while it runs (P1)

Board with 20,000 drifted rows so the repair has work. Control and treatment on
the same board, both binaries:

```
control : create CTRL-1                     -> rc=0, row present
treatment: normalize-board --apply  (start)
           create PROBE-MID-1               -> rc=0 "created task PROBE-MID-1"
           row present while apply running  -> True
           row present after apply finished -> False
           task_created events for PROBE-MID-1 in events.jsonl -> 1 (orphaned)
```

Repeated 8 rounds × 20 concurrent creates = **160 creates, every one rc=0, zero
surviving**. `validate` afterwards: rc=0, `RESULT: OK (0 warning(s))`. `doctor`:
rc=0. The board looks healthy and is missing rows.

Mechanism: `NormalizeBoard` reads and classifies the whole file first; the write
guard is acquired only around the final `atomicRewrite`
(`internal/board/normalizeboard.go:418-421`). The rewrite bytes come from the
pre-append snapshot, so anything appended in between is discarded. Same shape on
the released v0.1.11 — this is product code, not deployment drift.

Window size (hyperfine, warm, 5 runs, `--apply` re-armed each run):
1,000 rows 16.0 ms ± 5.8 · 10,000 rows 84.3 ms ± 9.4 · 50,000 rows
346.2 ms ± 25.6. A second-scale repair of a big board is a wide window.

### 2. `--repair-events --apply` deletes events appended while it runs (P2)

60,000 events with descending ids, `event … --detail-text PROBE-EV-<n>`
appended 0.10 s into the apply: the probe detail is absent from the final file
in **5/5 attempts on HEAD and 5/5 on v0.1.11** (apply rc=0 each time). The guard
again wraps only the write (`internal/board/repair_events.go:209-214`), which is
a plain `os.WriteFile` rather than the `atomicRewrite` used elsewhere — so the
same window also has a truncation risk on a crash. `dedupe-board`'s
`DedupeByID` has the identical unlocked read-then-`atomicRewrite` shape
(`internal/board/iddupe.go:107-236`, guard only in `CloseIDupeLine` at :329);
not probed here because that tool is a source-only one-shot.

Test gap that let this ship: `internal/board/write_concurrency_test.go` has
seven tests, all append-vs-append, same-id racing, lock exclusivity/release or
reader isolation. Nothing interleaves a rewrite with an append.

### 3. No verb can author `acceptance_criteria` (P2)

`create --acceptance-criteria …` → `flag provided but not defined`; `update` has
no such flag either (34 flags checked). README mentions the key only as drift the
`normalize-board` repair rewrites. Every fleet agent that must make a row
dispatchable therefore edits `tasks.jsonl` by hand — the exact bypass
README:1262-1264 warns produces duplicate-id corruption. Rows DF-BOARDCTL-48..50
needed it while being filed, which is the proof.

## What held up (no row)

* **Documented command surface / release parity.** 61 documented commands+flags
  driven end to end against the v0.1.11 asset and a HEAD build: identical exit
  codes and output shapes, zero drift. `render -o html --json sidecar` →
  `import --renumber` round-trips as documented; `umbrella` works on real git
  children; the write vocabularies, id law, dangling-`depends_on` and
  negative-header refusals all fire with precise messages.
* **Single-writer locking.** Concurrent creates of distinct ids and same-id
  races behave as documented (the lock is real for the append path).
* **Speed.** Read verbs are all in the tens of ms on real boards; the only
  seconds-scale operation is a wholesale rewrite of a 50k-row board (346 ms) —
  fast, but that is the window described above, so it is correctness evidence
  rather than a PERF finding. No PERF row: nothing here is slow enough that a
  user would feel it.

## Install leg (inherited, not re-run)

The first delivery of this tick proved the fresh-box leg at this same HEAD:
bunker-mvp agent `boardctl-df25`, README quickstart verbatim, checksum OK,
`boardctl version v0.1.11`, 1.18 s, no Go/sudo, agent destroyed and residue
rechecked. HEAD and the install docs are unchanged since, so this delivery cites
that proof instead of spawning again.

## Repro

```
/tmp/df1023/repro.sh          # decisive control/treatment (tasks side)
/tmp/df1023/conc.sh           # 8 rounds x 20 concurrent creates
/tmp/df1023/repro_events3.sh  # events side, N attempts
/tmp/df1023/measure_rewrite.sh# window vs board size
/tmp/df1023/census.sh         # 61 documented commands, release vs HEAD
```

## Correct use until DF-BOARDCTL-48/49 are fixed

Do **not** run a rewrite verb (`normalize-board --apply`,
`--repair-events --apply`, `dedupe-board --apply`) on a board a lane may be
writing. Freeze writers, or copy the board to a scratch dir, repair there, and
swap the file while nothing writes — then re-read the tail to confirm the rows
you expect are present.
