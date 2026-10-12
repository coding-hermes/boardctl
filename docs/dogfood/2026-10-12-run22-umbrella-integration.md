# Dogfood Run 22 — the umbrella surface (BT-078) over real fleet boards

Date: 2026-10-12 · Verdict: PROMISING-BUT-ROUGH (umbrella surface; CLI/board core remains SHIPPABLE from runs 15-21)

## Promise under test

`boardctl umbrella` is a read-only unified view over one repo's board plus every
child board symlinked under `.coding-hermes/links/`, with repo-qualified ids
(`<repo>/<id>`), per-repo provenance, and unambiguous cross-board depends_on
resolution (README §"Umbrella repos").

## The real user's setup (verbatim)

```bash
mkdir -p /tmp/df-u/.coding-hermes/links
ln -s /home/kara/coding-hermes-boardctl/.coding-hermes /tmp/df-u/.coding-hermes/links/boardctl
ln -s /home/kara/warpfs/.coding-hermes          /tmp/df-u/.coding-hermes/links/warpfs
/tmp/bc -C /tmp/df-u umbrella            # census
/tmp/bc -C /tmp/df-u umbrella --stats    # per-repo + grand totals
/tmp/bc -C /tmp/df-u umbrella --validate # per-board reports + dep resolution
/tmp/bc -C /tmp/df-u umbrella --list     # unified table
/tmp/bc -C /tmp/df-u umbrella --json     # machine census
```

This is the real fleet pairing: boardctl (163 tasks, clean canon) + warpfs (369
tasks, its own dialect). First command to useful output: ~1 minute including
reading the README section once.

## What worked, exactly as documented

- Census: `2 board(s) in view`, per-repo task counts, topology labels, exit 0.
- `--json`: clean machine shape (`boards[].repo/tasks`, `findings: []`,
  `total`), verified against the text census.
- `--list`: full 529-row unified table with REPO column and repo-qualified ids.
- `--validate`: per-board reports plus the cross-board depends_on resolution —
  17 local resolutions printed, dedupe of same-target links warned correctly.
- Safety contract: broken symlink → `[error]`, exit 1, nothing followed;
  two links to one target → deduplicated with a warning; `--strict` on the
  healthy view exits 0. All read-only (board files untouched).

## What broke or bruised (rows filed)

1. **DF-BOARDCTL-34 (P1)** `--stats` tallies warpfs's embedded-JSON priority
   values (`P1","source":"qa-dagger`) verbatim into their own buckets in BOTH
   the per-repo and grand-total maps — internal/board/umbrella.go `Stats()`
   sums `st.Priority` with no vocabulary check, while writes reject the same
   value. The summary the user reads first is lying by key.
2. **DF-BOARDCTL-35 (P2)** doctor's git-tracked-set check only inspects
   *tracked* paths; an untracked 12KB DuckDB `board.db` (mtime today, v1.5.5)
   sits in this repo's own board dir while README line ~1073 declares "the
   JSONL files **are** the board". Doctor exits 0; the fossil regenerates.
3. **DF-BOARDCTL-36 (P2)** warpfs's board is a second dialect: 597 warnings —
   127/369 rows with non-canon keys, 38 empty statuses, string depends_on,
   missing created_at. Cross-repo views drown with no prioritized onboarding
   recipe.
4. **DF-BOARDCTL-37 (P2)** `--list` takes no `--status`; the natural next
   question ("what's pending across all my boards?") has no umbrella answer.

## Errors hit and their meaning

- `flag provided but not defined: -status` → usage printed correctly (exit 2
  contract held); the gap is the missing capability, not the message.

## Install leg (ephemeral bunker, 2026-10-12)

- las-bunker-03 ssh timeout; las-bunker-02 spawn failed once (slice-limits
  containment, transient), retry spawned OK — but the agent's network then
  died mid-leg (ssh + API both unreachable, destroy deadline-exceeded; TTL
  backstop 2h; escalation note, host las-02 last-seen healthy).
- Completing leg on bunker-mvp agent d2ea6a81: clone of HEAD b55d305
  (network git fetch worked), README zero-dep path (v0.1.10 asset +
  SHA256SUMS) **INSTALL_SECONDS=1**, checksum OK, version prints
  `boardctl version v0.1.10`; `go build ./cmd/boardctl` at HEAD → `dev`
  binary; validate + doctor on the cloned repo board both `RESULT: OK (6
  warning(s))`; init→create→update→validate smoke green (0 warnings).
  Agent destroyed, key removed. Toolchains found: go + pip only (documented
  zero-dep path unaffected).

## Perf (Step 2b)

- `umbrella --stats` (2 boards, 529 rows): 85.4ms ± 38.2 (hyperfine, 20 runs)
- `umbrella` census: 73.9ms ± 33.4
- single-board `validate`: 55.0ms ± 18.3
Nothing a user feels; no PERF rows filed.

## Verdict reasoning

The umbrella surface delivers its structural promises (provenance, read-only,
refusal semantics) but its aggregate views are where un-vocabularied data
leaks through — the exact composition trap the dogfood skill warns about.
Core CLI remains SHIPPABLE; the cross-repo layer is PROMISING-BUT-ROUGH.
