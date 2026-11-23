# Dogfood run 31 evidence — tick boardctl-dogfood-2026-11-23-14-28-25

Run 31, 2026-11-23. Measured at HEAD 4ab8463 (product tree moved 67 files since run 30's
measured tip 0179f5b; binary under test = v0.6.0 release asset at ~/.local/bin/boardctl,
identity verified by `make install-check`).

## Files

- `initiative-canon-verify.txt` — DF-BOARDCTL-061 proof. Scratch board: `create --initiative`
  succeeds, `validate` warns `{initiative} outside the declared canon` with `0 fixable`,
  `update --normalize` refuses ("already canonical"). Real board: drift census shows
  `initiative(5)` naming BT-091..095 (the rows that implemented the feature).
- `import-roundtrip-verify.txt` — DF-BOARDCTL-062 proof. Same-board import round-trip is
  idempotent (plan-first, 3 identical no-op); cross-board import is refused by the README-026
  mismatch gate with no override flag; the `import` flag surface is only `--dry-run`/`--renumber`.
- `perf-arms.txt` — warm hyperfine (6 verbs, 23–44 ms), cold first-run (0.02–0.03 s),
  gzip serve A/B (69,394 → 18,834 bytes on /ui; 543 → 217 on the events API route).
  Verdict: honest non-finding, no profile by doctrine.
- `battery-qa-evidence.jsonl` + `battery-cells.txt` — ephemeral-bunker battery ledger
  (agent=a9c51a53 @ bunker-las-03, launched from HEAD 463d4cf, adopted by collect as
  `recent tick-death-orphan`). 23 collected rows; 0 FAIL, 0 UNVERIFIED; docker-deploy +
  chaos-shutdown SKIPPED (forced off on the live Hermes host), docker_build/compose_config/
  chaos-corruption N/A, evidence-outside-repo WARN (the collect path lived in scratch —
  this copy is the durable one; byte-identical, no transformation).
- `battery-target-binding.txt` — HEAD + dirty-count captured at launch (463d4cf, 4 dirty
  lines: a sibling's events.jsonl touch since committed as 4ab8463, plus foreign untracked
  files; `git archive HEAD` ships none of them).

## What was NOT committed here

The battery's own adopted evidence dir `.coding-hermes/evidence/qa-20261123T150253Z/`
(evidence.jsonl + logs + manifest) is committed alongside this directory. The sibling
QA dir `qa-20261123T002047Z/` (2026-11-22) is NOT this run's — left untracked, untouched.
