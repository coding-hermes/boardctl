# boardctl integration report — 2026-09-12 re-dogfood

Second full dogfood run (first: 2026-09-04, which filed BT-005..011 — all
since fixed and shipped in v0.1.2). This run re-tested the same promise
against the current binary at HEAD `237e594`, plus the ephemeral-bunker
install leg. Run duration ~50 min.

## Promise (null hypothesis)

*A user can manage a coding-hermes foreman board (tasks/events/board/
fixtures JSONL) with a fast, database-free CLI — create/update tasks,
append audit events, read stats/header, validate/doctor integrity — with
byte-stable diffs and exit codes 0/1/2.*

## What was actually done (real use, not tests)

### Leg A — fresh-project quickstart (/tmp scratch, built binary)

`init` → `create` ×3 → `list`/`show`/`update` → `event --tick` → `header`
→ `stats`/`validate`/`doctor`. Time-to-first-success (init→validated
board): **under 5 seconds**. Everything the README quickstart shows works
exactly as written, including the no-clobber re-`init` (exit 0) and the
four-file bootstrap with the NEVER-DONE seed row.

Two surprises in leg A became findings:

- `create --id "bad id!"` exits 0. The junk id lands in `tasks.jsonl` AND
  a `task_created` event, and `validate`/`doctor` both report OK. The
  write-vocabulary work (BT-007) covers guard/ci/priority/depends-on but
  not the id format → **BT-023**.
- No other command misbehaved: dupe-create exits 1 with a clear message,
  `show`/`update` on a missing id exit 1, `--guard PASS` on `create`
  correctly rejected (undefined flag; guard/ci belong to `update`).

### Leg B — real fleet board (copy of coding-hermes-scheduler, 186 rows + 715 events)

Copied the live board to `/tmp`, `git init`'d the copy, then:

- `stats`/`list`/`show --events`: correct on real foreman-written data;
  whole leg ran in 122 ms wall including git ops. Boardctl itself is
  effectively instant (single linear scans).
- Byte-stability proven on a 186-row board: `update DOC-VERSION-1
  --status complete --commit-hash cafe123 --guard SKIP --ci SKIP` produced
  a **1-line diff** in `tasks.jsonl` (git numstat `1 1`) plus the appended
  completion event. The serialization-style preservation works on real,
  heterogeneous rows.
- BT-014 regression check: `event --type audit --tick 50` auto-synced the
  header (`ticks_total 42→50` on the scratch board; on the copy the tick
  counter followed) — the next `doctor` no longer self-implies drift. The
  BT-014 trap is genuinely closed.
- `validate` on the real board exits 1 with **23 errors, 117 warnings**:
  every `todo`/`done`/`open` status written by the fleet's own foremen is
  "not in vocabulary". The writers enforce a vocabulary the fleet doesn't
  speak → **BT-025** (decide: aliases or fleet-wide enforcement).
- `update --commit-hash` side effect caught by diffing the header: the
  task's FIX commit (`cafe123`) was written into the header's
  `last_commit` — the board-edit drift pointer — while the header's
  `updated_at` stayed stale. Two meanings collide in one field → **BT-024**.
- `doctor` correctly errored on `board.db` being "tracked" — an artifact
  of my scratch copy (cp copied the untracked file into a fresh repo where
  I `git add -A`'d everything). The REAL scheduler repo does not track it
  (verified with `git ls-files`). The check works; lesson: never
  `git add -A` a board copy — the doctor will (correctly) bite.
- Legacy boards: helios (only `tasks.jsonl` present) exits 2 "no board
  found" with no hint the file exists; consensus (all four files, pretty-
  printed multi-line JSON) exits 1 with a raw `invalid character ';' in
  string escape code`. Same underlying condition class, two different
  codes, neither message explains the legacy-board story the README tells
  → **BT-026**.

### Leg C — ephemeral bunker install (las-bunker-03, agent fcfd3f4b, destroyed after)

- `bunker spawn --ttl 2h` on `bunker-las-03` (bunkerd active, Docker
  26.1.5); bare Debian 13.6 user, **no sudo, no Go toolchain**.
- Clone via the README's implied public HTTPS URL:
  `git clone https://github.com/coding-hermes/boardctl.git` → OK, HEAD
  `237e594` matches local. Repo visibility untouched.
- README quickstart verbatim: download v0.1.2 `boardctl-linux-amd64` +
  `sha256sums.txt`, `sha256sum -c --ignore-missing` → `boardctl-linux-amd64: OK`.
  **install_seconds=3**.
- Smoke on the cloned repo's own board: `stats` / `list --status pending`
  / `validate` (RESULT: OK, 0 warnings) / `doctor` (RESULT: OK) / `show
  BT-020` / `header --json` — all exit 0, binary self-reports `version
  20260911`.
- Fresh-board smoke in a mktemp dir: `init` → `create T-1` → `update T-1
  --status complete --guard PASS --ci GREEN` → `stats` — INIT_SMOKE_OK.
- Agent destroyed (`bunker destroy fcfd3f4b` → exit 0, `bunker list` empty).

Install leg verdict: **a fresh user on a clean machine can install and
use boardctl in seconds using only the README.** No SKIPPED row needed.

## Friction count

7 frictions hit across ~25 commands (2 P1 code findings BT-023/BT-024,
2 P2 validation/UX findings BT-025/BT-026, 1 stale skill-file claim fixed
in this commit, 1 self-inflicted `git add -A` trap, 1 docs-order nit from
the 09-04 run already fixed). Zero blockers.

## Verdict

**SHIPPABLE.** The promise holds: the documented workflows work verbatim,
install is trivial (3 s binary path proven on a bare Debian box), writes
are byte-stable on real boards, and the audit-event/header machinery
(BT-011/BT-014) that was broken at the last dogfood is now correct. What
remains is enforcement symmetry (ids, status aliases) and error UX on
legacy boards — filed as BT-023..026 for the foreman.

## Findings filed (on this repo's own board, with boardctl itself)

- **BT-023** (P1) — `create --id` accepts any string; junk ids flow into
  tasks + events while validate/doctor stay silent.
- **BT-024** (P1) — `update --commit-hash` overwrites header `last_commit`
  (drift pointer) with the fix commit and leaves header `updated_at` stale.
- **BT-025** (P2) — status vocabulary asymmetry: real foreman rows
  (`todo`/`done`/`open`) fail their own board's validate (23 errors).
- **BT-026** (P2) — legacy/partial-board error UX: `tasks.jsonl`-only dir
  reports "no board found" (exit 2, no file hint); pretty-printed boards
  report a raw JSON error with no row context.
