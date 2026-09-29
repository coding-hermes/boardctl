# boardctl dogfood run 18 — the repair path and a closure audit (2026-09-29)

Lane: boardctl-review (stand-in workdir); repo: coding-hermes-boardctl at
c97cd87 (= origin/main at run time). Verdict:
**PROMISING-BUT-ROUGH on the process surface; the CLI/board core remains
SHIPPABLE** (runs 1–17 consensus, unchanged here).

## Promise under test

Two promises, both from the repo's own surfaces:

1. *The repair promise (README + SKIPPED-LINES error):* a board with
   unparseable lines — unreadable by default — can be degraded-with-evidence
   (`--skip-bad-lines`) and salvaged (`validate --repair` →
   `tasks.rewritten.jsonl`, `tasks.jsonl` never modified).
2. *The board's promise (implicit, tested by the closure audit):* a row marked
   `complete` means a user of main gets the fix.

## Result 1 — the repair path delivers exactly its contract

Synthetic damage board (`/tmp/dogfood-bctl18/dmg-run1`, replicate with the
three historical dialects): 6 good rows + a raw-newline split row (helios), a
truncated row (consensus), a banner line (ring-runner).

| Step | Result |
|---|---|
| default validate | exit 1, `unexpected EOF` — board unreadable (correct) |
| validate --skip-bad-lines | 8 rows validated, 4 SKIPPED `[error]`s with parse error + snippet + line numbers, SKIPPED-LINES block, exit 1 |
| validate --repair | `wrote 8 salvaged row(s)`, `dropped 4 (lines 4, 5, 8, 9)`, exit 0 |
| byte-identity | sha256(tasks.jsonl) identical before/after: YES |
| swap + validate | RESULT: OK, exit 0 |

Caveats a user needs (also in diagnostics.md):

- The split row is dropped, not rejoined — repair is conservative; split rows
  need manual reconstruction.
- `--repair` is NOT in the README (REVIEW-BOARDCTL-011); only
  `--skip-bad-lines` is.

Fresh-box proof (bunker-las-03, agent e3a33f30, destroyed; network clone of
c97cd87 + checksum-verified v0.1.9 release binary, install 4s): the published
binary carries the path — repair on a damaged board salvaged 1 / dropped 1,
exit 0, byte-identity held. Full CRUD smoke green. The v0.1.9 asset predates
the three unmerged fix branches, which is exactly why result 2 matters.

Perf on a 3700-row damaged board: validate 33.7ms ± 3.0, --skip-bad-lines
30.1ms ± 2.2, --repair 28.0ms ± 2.6 (hyperfine, warm, -i because exit 1 is the
contract). Nothing a user feels; no PERF row filed.

## Result 2 — four rows closed complete while main lacks the fixes

c97cd87 (board-only commit, 2026-09-28 21:43, message says "removed duplicate
rows"; numstat is exactly 4 status flips) marked complete:

| Row | Fix branch | Probe at c97cd87 | Fix works? |
|---|---|---|---|
| DF-BOARDCTL-19 | wt/DF-BOARDCTL-19 48d2af9 (+265) | delete Gofmt/Version-check/Spec-check from ci.yml → `go test ./internal/workflowcheck` + 3 make targets ALL GREEN | yes: with the branch, the same deletion FAILS the suite |
| DF-BOARDCTL-20 | wt/BT-064 9393ed0 (+253) | stub govulncheck on PATH → `make vuln-check` GREEN | yes for parseable output (`Scanner: govulncheck@v0.0.9` → rc=2, precise message); banner-less output still sails through (DF-BOARDCTL-23) |
| REVIEW-BOARDCTL-009 | wt/REVIEW-BOARDCTL-009 b52a4c8 (+120) | `POST /nonexistent` → 400 | yes: 404 on both probes |
| REVIEW-BOARDCTL-010 | wt/REVIEW-BOARDCTL-010 4f79145 | `event --help` enumerates flags | yes — and this one IS merged into main |

None of the first three branches is merged; c97cd87 is pushed to origin/main.
The 02:15 wave (`boardctl-2026-09-29-02-15-25`) re-dispatched the same four
ids while the rows read complete — workers double-booked on closed rows.

Fix direction (DF-BOARDCTL-22): merge the three branches, then re-run each
row's original probe at the new HEAD before the status claim ships. Until
then, the board overstates what main (and any released binary) delivers.

## What a fresh user gets on the README path

Install → verified v0.1.9 in 4s, zero friction. Reading/validating a healthy
board: instant, strict, trustworthy (validate/doctor/stats all verified
against the repo's own board on the fresh box). Recovering a damaged board:
works as the error messages promise. Getting the three still-open fixes:
impossible from any published release — they are branch-only.

## Friction count

2 substantive (closure-process gap = DF-BOARDCTL-22; undocumented --repair =
REVIEW-BOARDCTL-011) + 1 boundary note (DF-BOARDCTL-23, skip-by-design but
pin-defeating). Zero CLI friction: every documented command worked first-try
on the fresh box.
