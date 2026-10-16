# boardctl dogfood — run 24: normalize-board + umbrella --status closure re-verification (2026-10-16)

Sibling note: run 23 (same day, docs/dogfood/2026-10-16-run23-repair-chain-reproof.md) re-proved
the 2026-10-14 repair-chain fixes on a different board; this run 24 took the normalize-board +
umbrella --status closures with no overlap.

Tick `boardctl-dogfood-2026-10-16-00-13-43`. HEAD `4ad7543` (binary built from checkout, `boardctl version dev`).

## Angle
Runs 1-22 left two landed-but-never-re-dogfooded closures: DF-BOARDCTL-36 (normalize-board,
shipped in tick 117) and DF-BOARDCTL-37 (umbrella `--list --status`). This run drives both at
HEAD over real data — the 553-task umbrella view (boardctl 167 + warpfs 386) and warpfs's
second-dialect board, the exact measured case the README cites.

## What a user does, and what happened

### Umbrella --list --status (DF-BOARDCTL-37 closure) — PROVEN
Setup from README verbatim: a scratch umbrella dir with `.coding-hermes/links/{boardctl,warpfs}`
symlinks; `boardctl -C <dir> umbrella` census prints both boards, topology A, rc 0.

- `--list --status pending` → 41 warpfs rows in the documented table; boardctl contributes 0 —
  cross-checked against the board: its only pending row is NEVER-DONE, a fixtures.jsonl row,
  excluded by design. The filter count is right.
- `--status pending,in_progress` CSV accepted.
- `--status bogus` → exit 2, message quotes the full vocabulary and the `completed` alias.

### normalize-board on the live warpfs board (DF-BOARDCTL-36 closure) — PROVEN, with a live limit
- Dry-run: 386 rows → 97 mechanical repairs (55 key aliases, mostly `_pass_criteria` →
  `acceptance_criteria`; 27 string `depends_on`; 15 status aliases), 42 refusals, 161
  missing-`created_at` rows reported, 0.028s, ZERO board writes (staged
  `tasks.normalized.jsonl` only — byte-verified, and the staged file was removed after the run).
- `--apply` on a SCRATCH copy (this lane never mutates board state): all 97 repairs applied in
  one pass, value bytes intact (spot-check GAP-041), refusals untouched, line count unchanged,
  validate then fails on exactly the 5 refused rows — honest reporting, no silent massaging.
- The limit: warpfs's foreman appender writes status `"closed"` (38 rows, writer active as of
  2026-10-15 22:30) — outside boardctl's vocabulary AND alias table. normalize-board refuses all
  of them ("needs an explicit decision"), so the board stays validate-red after every mechanical
  repair and the drift regrows each foreman tick. Filed as DF-BOARDCTL-41 (P1).

## Findings
- **DF-BOARDCTL-41 (P1)** — warpfs appender emits `closed`; decide the mapping and fix the writer.
- **DF-BOARDCTL-42 (P3)** — `normalize-board --apply` requires events.jsonl; a tasks.jsonl-only
  board is refused with a generic "no board found" while the dry-run leg works on the same dir.
- Resolved as NOT a finding: umbrella's 0 boardctl rows under `--status pending` (fixtures
  exclusion, correct by design).

## Perf (Step 2b)
`umbrella --list --status pending`: 30.2ms ± 2.1 (hyperfine, 20 runs, 553-row view). Nothing a
user feels; no PERF rows.

## Install leg (bunker-mvp, agent boardctl-df1023 — spawned + destroyed, no residue at +90s)
README quickstart verbatim on the fresh agent: checksum `boardctl_linux_amd64: OK`, version
v0.1.10, `install_seconds=1`, clone + `validate` on the repo's own board rc=0 (17 warnings, incl.
the legacy `EVT-ND-*` non-numeric event-id shape). Note: the released v0.1.10 binary has NEITHER
normalize-board NOR umbrella --status — both postdate the release (docs-ahead-of-release class,
same as DF-BOARDCTL-26). bunker2/3/4 ssh timeout; mvp is the substitute leg, named as such.

## Infra / lane notes
- Config finding for the config owner: `boardctl-dogfood`'s stored scheduler prompt is a verbatim
  copy of warpfs-dogfood's (targets warpfs, warpfs workdir). Lane name + tick header say
  boardctl; per skill law this run followed the lane name.
- Sibling collision during row filing: DF-BOARDCTL-38/39 were taken mid-run; rows renumbered to
  DF-BOARDCTL-41/42 after a fresh max-id census (collision check in the append script refused
  the first append — the guard worked).
