# Dogfood run 23 — repair-chain re-proof at HEAD (DF-BOARDCTL-31/32/33/36/37) + umbrella cross-check

- **Date:** 2026-10-16 · **Run:** 23 (boardctl-dogfood lane) · **Builds:** HEAD 4ad7543 (local `dev` build) + release v0.1.10
- **Angle:** re-proof of the four rows fixed on 2026-10-14 (ticks 117-118) — runs 1-22 never drove the fixed code through real use. Plus the skill's unexercised-surface rule: the *composition* (dedupe → normalize → validate) on a foreign dialect board.

## Promise under test

A user with a drifted/id-recycled foreign board can (1) dry-run the recycled-id repair and read the census explanation, (2) apply it, (3) run normalize-board for the mechanical status/shape drift, (4) read validate's residual error set, and (5) get a filtered umbrella --list — with the 0/1/2 exit contract holding.

## Re-proofs (all at HEAD 4ad7543, scratch copies only; real boards untouched)

### DF-BOARDCTL-32 — dedupe already-terminal census — FIXED, VERIFIED
Board: scratch copy of `qa9-frozen-after` (real task-router board, 395 rows, 20 recycled ids, 18 groups already terminal).
- Dry-run: `task rows: 395 (distinct ids: 372, recycled ids: 20)`, text census names kept/supersede lines with `[already terminal, left as-is: line N]`.
- After `--apply` (2 actionable lines closed): re-run prints **"nothing to do — 20 recycled id(s), all later lines already terminal (no files written)"** — the run-21 contradiction is gone.
- `--json`: `recycled_ids(20) == len(groups)(0) + len(already_terminal_ids)(20)`; pre-apply `groups[0]` carries `already_terminal_lines`.
- Zero-write proof: dry-run md5-identical. Audit events appended with original titles preserved.

### DF-BOARDCTL-33 — `--by-id --lanes` refusal exit 2 — FIXED, VERIFIED
Text mode exit **2**, stderr `refusing to touch board ... — it is not in --lanes NoSuchLane`; JSON mode same. Match-lane run proceeds (rc 0, "no recycled ids"). No positional-arg confusion.

### DF-BOARDCTL-36 — normalize-board — FIXED, VERIFIED
Dry-run on the applied qa9 scratch: 10 mechanical repairs across 9 rows (`done→complete`, `in-progress→in_progress`), 6 rows REFUSED with reasons (5 × `"superseded" is not in the vocabulary — needs an explicit decision`, 1 × absent status) — fail-closed as designed. Missing-`created_at` note on 129 rows: repairs never fabricate timestamps. `--apply` rewrote in one atomic pass.

### DF-BOARDCTL-31 — validate red post-repair — re-proven PARTIALLY FIXED (new residual classes)
Error census 33 → 31 after the full chain. Gone: the 2 duplicate-id errors my apply closed (later lines now carry `superseded_by` → graded **warn** naming the closed line, exit contract holds). Remaining 31, all named classes:
- 21 × duplicate task id (all already-terminal lines, dedupe correctly leaves them — the tool's scope is honest)
- 5 × status `"superseded"` out-of-vocabulary (normalize refuses by design — needs an explicit human decision)
- **5 × events.jsonl `event id descends below earlier id`** — legacy epoch-millis ids (e.g. 1791314878350 < 20261006170002). **No verb repairs event ids** → filed **DF-BOARDCTL-38 (P2)**.

### DF-BOARDCTL-37 — umbrella `--list --status` — FIXED, VERIFIED
`--status pending` (40 rows, matches --stats), `--status pending,complete` (506), alias `done`, bogus → exit 2 with vocabulary in the error. --stats on boardctl+warpfs view (551 tasks) tallies priorities clean: 375 valid P-values counted, 10 embedded-JSON garbage values excluded (grep ground truth: 385 warpfs rows = 375 counted + 10 excluded) — **DF-BOARDCTL-34 verified fixed**.

### DF-BOARDCTL-26 premise — TRUE AGAIN at HEAD vs release (new row)
README documents normalize-board; checksum-verified release v0.1.10 says `unknown command "normalize-board"` (exit 2 on fresh agent). HEAD has it. → **DF-BOARDCTL-39 (P2): cut v0.1.11.**

## Install leg (bunker-mvp; las-03/bunker2/bunker4 unreachable or down — one live host is enough)

- Spawn `bctl-df23` TTL 2h. Fresh agent: no Go → `go build` auto-downloaded go1.26.9; HEAD built from git-archive transfer (sha256 d08b3faa… verified on both ends) in **46s**.
- Release path verbatim: curl+SHA256SUMS verify OK, `boardctl version v0.1.10`.
- Smoke on the repo's own board: validate OK (6 warnings), dedupe clean, normalize "already canonical". Release-vs-HEAD discriminating probes captured on the agent (normalize unknown on release).
- Destroyed cleanly; 90s recheck: no residue. **install_seconds≈47 for the full release-verify + HEAD-build path.**

## Perf (Step 2b, hyperfine 20 runs, HEAD build, warm)

validate 18.2ms ±1.0 (167-row board) · dedupe --by-id dry-run 22.0ms ±1.3 (395-row board) · umbrella --stats 29.4ms ±1.1 (551-task view). Nothing a user feels → **no PERF rows** (a win nobody can feel is not a finding).

## Verdict

SHIPPABLE at HEAD — all four 10-14 fixes hold under real use on the harshest real board available; the release boundary is again the weak point (docs ahead of artifact; event-id errors unrepairable → 38/39/40).
