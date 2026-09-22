# boardctl dogfood — 2026-09-22 — fleet-wide / multi-board angle

**Run:** boardctl-dogfood-2026-09-22-13-25-18 · binary: v0.1.7 release asset (sha256-verified against published SHA256SUMS) · angle: the surface no prior run touched — real LIVE fleet boards at scale (53 boards, 11,883 task rows), headerless/topology-B reality, fingerprints + dedupe-board, worktree/branch/session fields, --normalize.

Prior runs (dogfood-log.md): 2026-09-04 + 2026-09-12 CLI core; 2026-09-20 render/serve/import.

## What a real fleet operator does with this tool

boardctl's actual daily user is the fleet operator (or a lane script) sweeping boards across
~50 repos: is the board healthy, what's pending, did the last tick get recorded. Nothing in
the README promises a fleet sweep, so I ran it the way the tool invites: `boardctl -C <board>
validate/doctor/stats` in a loop. It took **624ms for all 53 boards**. That loop is the
headline: the tool is instant at fleet scale, and it turns up real integrity debt —
which is the point of validate.

## Results across 53 live boards (11,883 task rows scanned)

- 23 boards validate clean (exit 0).
- 4 boards exit 2: tasks.jsonl-only boards (helios 2372 rows, h3-sdk-go-foreman,
  h3-sdk-python-foreman, sdk-typescript). The error text is honest and exact — this is the
  BT-026 class, kept.
- 27 boards exit 1. Two distinct causes:
  - **9 boards carry spec-invalid JSON lines** (independently confirmed with jq): consensus
    1338 bad lines, helios 2347, ring-runner 33, plus 1–2 each on crier, deepseek-dashboard,
    musterflow, speclang, task-router, h3-sdk-python-foreman. One bad line aborts the whole
    read — `consensus` boardctl list/show/stats all exit 1 on line 167 alone; 164 valid rows
    are unreachable. Note: these boards were remembered as "legacy pretty-printed but VALID";
    the raw bytes say otherwise (unescaped control characters, truncated rows, a bunker banner
    pasted mid-row). They were never JSONL-valid; only line-count similarity made them look it.
  - **Vocabulary debt**: free-form guard_result/ci_result strings ("PASS 4/4", "n/a",
    "pre-push") on ~19 boards — warnings, correctly non-blocking. Non-canonical statuses:
    `parked` x63 (hermes4friends-infra), `duplicate` x15 (hermes-dagger), `closed`, `retired`,
    `resolved`. And **duplicate task ids** from id-recycling on asce/ai_plays_poke/
    hivemind-work/dexdat-memory/speclang — 6 different findings all filed as QA-ASCE-1.

## Feature verdicts (scratch board + live reads)

| Feature | Verdict |
|---|---|
| init (fresh board) | works; 4 files, seeded NEVER-DONE fixture, no-clobber |
| create + fingerprints | DUPLICATE-SUPPRESSED exit 2 + task_evidence event: correct, both with and without --evidence-run-id |
| id format guard | "bad id!" refused exit 1; --force writes it: correct |
| update (status/guard/ci/summary/commit) | works; byte-stable (only target line changes) |
| --worktree/--branch/--session | round-trip as documented; sessions replaced wholesale |
| --normalize | canonicalizes alias statuses; refuses ambiguous guard_result ("4/4 PASS") with a decisive error; refuses change-flag combos |
| event --tick | appends + auto-syncs header ticks_total: works |
| headerless / topology-B boards | header refusal message is precise; validate warns and continues (coding-hermes-tools validates OK, exit 0, topology B) |
| dedupe-board (dry-run) | runs on a live board; correctly reports zero groups on asce despite 6x QA-ASCE-1 — those rows have different titles+reasoning, so they are NOT fingerprint duplicates. id-recycling is a lane-hygiene defect, not a dedupe case. |
| render v0.1.7 | burndown bug NOT actually fixed — see DF-BOARDCTL-8 |

## Performance (Step 2b, hyperfine, warm, release binary v0.1.7)

Nothing here is slow enough for a user to notice, so no PERF rows are filed — the numbers:

- validate, hermes-dagger board (1163 rows, biggest healthy): 43.0ms ± 1.8 (20 runs)
- stats, same board: 28.9ms ± 1.7 · render of a 53-task board: 13.6ms ± 0.8
- full fleet sweep (53 boards, validate each): 624ms (3 runs: 621/624/626ms)
- consensus validate fails fast at the first bad line: 11.7ms ± 0.7

"Fast, database-free" holds up: per-board cost is ~1ms/100 rows, process startup dominates.

## Install leg (ephemeral bunker, las-bunker-03)

- Fresh Debian 13 agent a136ce93 (spawn OK), repo cloned from github.com/coding-hermes/boardctl
  at HEAD 9676c28 — no credentials touched, no visibility/permission changes.
- README quickstart verbatim (curl release binary + SHA256SUMS verify): **INSTALL_SECONDS=4**,
  `boardctl version` → v0.1.7.
- Smoke on the box: validate/doctor/stats on the cloned repo's own board (all exit 0),
  init→create→duplicate-refusal→update→event battery (all green), render 356,178 bytes with
  0 external refs — and the burndown 2-of-20 defect reproduced from the fresh clone.
- Agent destroyed cleanly (destroy exit 0, key removed). No SKIPPED row needed this run.

## Value judgment

- **Does it work?** Yes for any spec-valid board — reads, writes, integrity checks, byte
  stability, exit-code contract all held under a 53-board sweep and a fresh-machine battery.
- **Is it useful?** The 624ms fleet sweep is a genuinely useful operator ritual, and it found
  real debt (9 unreadable boards, id-recycling, parked statuses) on day one.
- **Is it usable?** High. Error messages name the exact line and reason. Friction found:
  no recovery path for spec-invalid boards (DF-BOARDCTL-9), and the --evidence-run-id docs
  gap (DF-BOARDCTL-11).
- **Is it trustworthy?** The dry-run guarantee held (git-verified zero writes on asce), and
  dedupe-board refusing to collapse text-distinct rows (even same-id ones) is correct, not a bug.
- **Verdict: PROMISING-BUT-ROUGH** — the CLI core is shippable and fast; the blocker is that
  ~10 of the fleet's 53 real boards are unreadable to it, and the v0.1.7 burndown "fix"
  degrades data silently instead of blankly.

Rows filed this run: DF-BOARDCTL-8 (P1 burndown 2-of-20), DF-BOARDCTL-9 (P1 one-bad-line
kills the whole board), DF-BOARDCTL-10 (P2 id-recycled near-dupes hygiene), DF-BOARDCTL-11
(P2 --evidence-run-id docs). Verified at board tail (events 198–201) and via validate OK.
