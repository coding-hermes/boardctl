# boardctl dogfood — run 25: the event-id repair (DF-BOARDCTL-38) under real use

- Date: 2026-10-23
- Tick: `boardctl-dogfood-2026-10-23-10-52-34`
- Target HEAD: `900b7b5` (`boardctl version dev` when built from checkout)
- Released artifact exercised: **v0.1.11** (published 2026-10-18)
- Lane workdir: `/home/kara/.hermes/stand-in/dogfood/boardctl` (scheduler row), product checkout `/home/kara/coding-hermes-boardctl`
- Verdict: **SHIPPABLE** for the repair's FUNCTION, **PROMISING-BUT-ROUGH** for its DATA CONTRACT (it repairs the sequence and mutates neighbouring bytes).

## Promise under test

> A user whose `events.jsonl` has drifted (event ids descending below earlier ids)
> can run `boardctl normalize-board --repair-events`, read a census, stage the
> would-be bytes, `--apply` them, and get a board that validates — with
> **every field except `id` left untouched** (README, `--repair-events` section).

Secondary promises exercised in the same pass: the write vocabulary rejects
`closed` while reads alias it to `complete` (DF-BOARDCTL-41's fix), `validate
--repair` still salvages a damaged board (DF-BOARDCTL-9), the umbrella view
still stitches child boards together, and the README's zero-dependency install
quickstart still produces a verifiable binary on a machine with no Go.

## Real use — what was actually driven

Dataset: a scratch copy of the **live task-router board** — 700 events, 473
descending ids, 29 legacy non-numeric ids, 411 task rows. This is a genuine
fleet board with genuine drift, not a fixture.

```bash
cp -a ~/task-router/.coding-hermes/board /tmp/df25/boards/tr1/.coding-hermes/
boardctl -C boards/tr1 validate                       # FAIL: 418 errors (407 descend), 430 warnings
boardctl -C boards/tr1 normalize-board --repair-events
#   events.jsonl: .../events.jsonl
#   total events: 700
#   descending ids: 473
#   non-numeric/missing ids: 29
#   duplicate ids: 0
#   fixed: 473 event(s) renumbered to ascending order
#   DRY-RUN — no file written (pass --apply to rewrite events.jsonl)
#   ... would-be bytes staged at .../events.jsonl.renumbered.jsonl
sha256sum events.jsonl tasks.jsonl                    # unchanged: dry-run really writes nothing
boardctl -C boards/tr1 normalize-board --repair-events --apply
boardctl -C boards/tr1 validate                       # FAIL: 11 errors (0 descend), 410 warnings
cmp events.jsonl.renumbered.jsonl events.jsonl        # IDENTICAL — staged bytes == applied bytes
```

**It works.** 407 descend errors → 0. The 11 residual errors are a different
class (row-level), which the repair is honest about by not touching. Staged and
applied bytes are identical, and a second dry-run reports the board clean.

The data-integrity check is where it breaks. Comparing all 700 events
field-by-field before/after (`/tmp/df25/check_events.py`): 473 ids changed, and
**one event's `detail` body was rewritten** — its embedded JSON went from
2-space to 1-space indentation. Small, but the contract says *untouched*.

### The mutation, isolated on a fixture

Three events, ids 10 / 9 / 11 (the 9 descends). The renumbered line carries a
recorded command, a title, a large integer and a deliberately non-alphabetical
key order:

```json
{"id": 9, ..., "detail": "cmd: rsync -a  /x  /y", "title": "fix  double  spaces",
 "tick_number": 9007199254740993, "z_first": "a", "a_last": "z"}
```

After `normalize-board --repair-events --apply` (released **v0.1.11**, fresh
bunker box — identical on HEAD):

```json
{ "a_last": "z", "actor": "foreman", "detail": "cmd: rsync -a /x /y", "event_type": "task_completed",
  "id": 11, "task_id": "TASK-1", "tick_number": 9007199254740992, "timestamp": "...",
  "title": "fix double spaces", "z_first": "a"}
```

Three separate losses, all on non-id fields:

1. **Double spaces collapse inside string values** — the recorded command and
   the title no longer match what was written into the audit trail.
2. **Integers above 2^53 round** — `9007199254740993` → `9007199254740992`.
3. **Key order is rewritten** alphabetically (byte noise on every repaired line).

And one more, separate: renumbering takes `runningMax + 1` without consulting
ids that appear *later* in the file, so the fixture's line 2 was renumbered to
`11` — an id line 3 already carried. The command still reports
`duplicate ids: 0` (it counted the input), and `validate` answers with a benign
`[warn] duplicate event id 11` and `RESULT: OK`. The repair can therefore make
a board *less* canonical than it found it while its own census says otherwise.

Code: `internal/board/repair_events.go:224-250` (`renumberEventID`) unmarshals
into `map[string]any` (float64 for every number, sorted keys on re-marshal) and
then runs `bytes.ReplaceAll(out, []byte("  "), []byte(" "))` over the whole
re-encoded line — a style hack that reaches inside string values.

## Findings filed (board: `.coding-hermes/board/tasks.jsonl`, 187 → 191 rows)

| Row | P | One line |
|---|---|---|
| DF-BOARDCTL-44 | P2 | `--repair-events` mutates non-id fields (2^53 rounding, double-space collapse inside values, key reorder) vs. the documented "every other field untouched" |
| DF-BOARDCTL-45 | P3 | `--repair-events` can **create** a duplicate event id and still reports `duplicate ids: 0` |
| DF-BOARDCTL-46 | P3 | `normalize-board` hard-fails on one unparseable line with no remedy named; `--skip-bad-lines` on that command is a dead end — 410 rows of drift stay unrepaired |
| DF-BOARDCTL-47 | P2 | `umbrella` aborts the whole cross-board view on one unparseable line in one child board (no `--skip-bad-lines`), while every sibling verb has a degrade mode |

Each row carries command-level `acceptance_criteria`; validate on the target
board is `RESULT: OK` with the same 14 pre-existing warnings as before the
append (byte-diff: 4 lines added, 0 removed).

## Verified clean (no row filed — these held up)

- **Write vocabulary** — `create --status closed` and `update --status closed`
  both exit 1 naming the allowed set `{complete,failed,pending,in_progress,review,blocked}`;
  `closed` is read-only as an alias (`list --status closed` renders it under
  `complete`, `stats` counts it as complete). DF-BOARDCTL-41's fix holds live.
- **Evidence gate (TR-251)** — `update --id X --status complete` without
  `--evidence-run-id` is refused with the reason and the flag to add.
- **`depends_on` validation on create** — a nonexistent dependency aborts the
  write (`create aborted (create the dependency task first)`).
- **Damaged-board salvage chain (DF-BOARDCTL-9)** — on a copy of the live warpfs
  board (411 rows, one duplicate-key line): `validate --repair` wrote 417
  salvaged rows to `tasks.rewritten.jsonl`, dropped exactly line 412, and left
  `tasks.jsonl` untouched.
- **Install (fresh machine)** — see below.
- **Fleet id law** — `create` refuses `S-1`/`S-3` with the regex and points at
  `--force`; all four rows filed this run use the minted `DF-BOARDCTL-44..47`.

## Install leg — fresh machine (bunker-mvp, agent `boardctl-df25`)

README "Install" quickstart, verbatim sequence, on a bare Debian agent with no
Go and no sudo (`curl` the v0.1.11 asset + SHA256SUMS, `sha256sum -c
--ignore-missing`, `chmod +x`, `version`):

```
boardctl_linux_amd64: OK
boardctl version v0.1.11
QUICKSTART_RC=0 INSTALL_SECONDS=1.18
```

Then the same agent ran the released **v0.1.11** binary against the drifted
boards (this is the part prior runs could not do — v0.1.10 rejected
`normalize-board` outright):

- real board: `validate` FAIL 418 errors (407 descend) → `--repair-events
  --apply` → FAIL 11 errors (0 descend)
- synthetic fixture: reproduced all three mutations and the duplicate-id
  introduction on the *released* artifact, so DF-BOARDCTL-44/45 are not
  HEAD-only defects
- cold first command on the fresh box: `validate` on the 700-event board in
  **87.5 ms**

The release boundary that runs 20/23 filed (DF-BOARDCTL-26/39) is **closed**:
README's "Current release: v0.1.11" matches the asset, and the asset carries
`normalize-board` + `--repair-events`.

## Step 2b — what using it costs

Warm timings, p50 of 20 runs on the live 700-event/411-row board (host was at
84% CPU during the pass, so these are upper bounds; hyperfine σ up to 77 ms):

| Operation | p50 |
|---|---|
| `validate` (fails, 418 errors) | 84.6 ms |
| `stats` | 55.9 ms |
| `list --status pending` | 56.1 ms |
| `show TR-001` | 72.5 ms |
| `normalize-board --repair-events` (dry-run, 473 fixes) | 93.0 ms |

Cold (fresh bunker box, first invocation): 87.5 ms. **No PERF rows** — every
headline operation is under 100 ms and a user does not feel any of it. Nothing
here justified a profile.

## Still open for the next run

- DF-BOARDCTL-44's fix will need a byte-diff re-proof on a real board (the
  checker script is `/tmp/df25/check_events.py`; a durable copy belongs in the
  repo with the fix).
- `normalize-board --repair-events --json` output shape was not inspected this
  run (only text mode) — worth a pass when 44/45 land.
- The warpfs board's duplicate-key line (row `INT-CI-011`, written 2026-10-23
  by that project's foreman) is the live specimen behind DF-BOARDCTL-46/47 and
  belongs to warpfs's own lane, not to boardctl.
