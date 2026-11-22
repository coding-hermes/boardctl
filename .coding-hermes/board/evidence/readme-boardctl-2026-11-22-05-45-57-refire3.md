# README health sweep — boardctl — tick boardctl-readme-2026-11-22-05-45-57 (re-fire 3)

Lane: coding-hermes-readme (skill `coding-hermes-readme`, local copy
`/home/kara/coding-hermes-skills/coding-hermes-readme/SKILL.md`).
Target: `/home/kara/coding-hermes-boardctl`.
Stand-in: `/home/kara/.hermes/stand-in/readme/boardctl`.

This is a REDELIVERY of a tick already delivered at c57abd7 (rows README-006-R2,
021, 022, 023) and re-verified at 6b6fa0b. Per the re-validation rule this fire
re-verifies instead of re-filing, and files only findings the earlier fires did
not carry.

## Identity of what was audited

```
$ git -C /home/kara/coding-hermes-boardctl log -1 --format='%H %ci %s'
6b6fa0be6b3d037d34072453eb9b380d148c7331 2026-11-22 02:42:52 -0500 board: readme tick 2026-11-22-05-45-57 re-fire 2 — verify-only readback + README-017 note/marker repair

$ git log -1 --format='%h %ad %s' --date=short -- README.md
ea8dcab 2026-11-21 Merge branch 'wt/DF-BOARDCTL-57'

$ git tag --sort=-creatordate | head -3
v0.5.0 (2026-11-15) · v0.4.0 (2026-11-11) · v0.3.0 (2026-11-09)
```

README.md is 2004 lines / 106457 bytes, last moved 2026-11-21 — AFTER the last
tag (v0.5.0, 2026-11-15). Freshness check 1 passes.

Probe binary: `go build -o boardctl-head ./cmd/boardctl` (go1.26.11 linux/amd64),
`boardctl version` → `boardctl version dev` (unstamped local build, as documented
at README.md:94-97). Published-asset probes used the v0.5.0 linux/amd64 asset
already staged in `dist/` (`./dist/boardctl_linux_amd64 version` →
`boardctl version v0.5.0`).

## Executed surface (T3)

Two probe matrices ran every documented command shape against throwaway boards
(`probe-matrix.sh`, `probe-matrix2.sh` in the stand-in):

- 60 commands, transcript `/home/kara/.hermes/stand-in/readme/boardctl/probe-out.txt`
- 27 commands, transcript `/home/kara/.hermes/stand-in/readme/boardctl/probe-out2.txt`

Matched the README's promised behaviour on: `init`, `create` (plain/ac/depends/
worktree+session/initiative/initiative-ref), `list` (+`--status`, `--actionable
--json`, `--all`, `--initiative`), `next`, `show` (+`--events`), `stats`/`stats
--all` (fixture exclusion 4 vs 5 as documented at README.md:84-89), `validate`
(lenient rc=0 / `--strict-keys` rc=1 / drift census / `update --normalize` fix /
`--repair` writing `tasks.rewritten.jsonl` and NOT touching tasks.jsonl),
`doctor` (headerless warn, git tracked-set, untracked `board.db` reported as a
cache fossil), `header` (+all four `--set-*`, negative counter refused),
`event` (+`--tick` header bump notice, `--no-tick-write`, `--detail @file`,
`--task-id/--actor`), `sweep-status` / `sweep-status --decide`, `normalize-board`
(+`--json`, `--repair-events`), `types --doctor/--json` and the documented
`types.jsonl` example (accepted, template applied: title `[RELEASE] Version 1.0`,
priority P1), `umbrella` census/list/status/stats/validate/json, `render`
(file + `-o -`), `serve` (loopback 200 on `/`, `/ui`, `/api/boards`; non-loopback
`--addr` refused rc=2; everything else 404 as README.md:486-487 promises),
`evidence baseline --record`, `install --dry-run` and `hook doctor` refusing
outside a git repo.

Gate targets run on the audited tree: `make fmt-check` rc=0, `make spec-check`
rc=0. `docs/muster/openapi.yaml` exists (5008 bytes, `paths:` at line 44), so the
README.md:491 reference resolves. Anchor check: `check-md-anchors.py README.md`
→ `anchors_checked=6 broken=0`. Link check:
`readme-link-check.py . README.md` → `local_missing=0 external_non_200=0` (3
local links, 2 external, no badges). Structure census: `h2_total=20
contents_bullets=0` (already filed as README-023).

Install path verified live against the published release:
`gh release view v0.5.0 --repo coding-hermes/boardctl` lists 7 binaries +
SHA256SUMS, `curl -sIL .../v0.5.0/boardctl_linux_amd64` → 200, and
`.../v0.5.0/SHA256SUMS` → 200 with 7 checksum lines. README-005's 404 premise is
gone; the block at README.md:19-23 is executable as written.

## NEW FINDING A — event vocabulary is 19 values, README teaches 18 (P4)

Command (source census, tip 6b6fa0b):

```
$ awk '/^var EventTypeVocabulary/,/^}/' internal/board/read.go \
    | grep -oE '"[a-z0-9_]+"' | tr -d '"' | sort -u
audit board_bootstrap board_init board_migration dogfood e2e_verified idle
spec_created task_added task_completed task_created task_dispatched
task_evidence task_started task_type_changed task_updated task_verified tick
worker_dispatched
                                                       -> 19 values (read.go:111-130)
```

Live refusal from a HEAD build (identical 19-value set):

```
$ boardctl-head -C <scratch> event --type bogus --tick 99 --detail-text x   # rc=1
boardctl: event type "bogus" not in vocabulary {audit,board_bootstrap,
board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,
task_completed,task_created,task_dispatched,task_evidence,task_started,
task_type_changed,task_updated,task_verified,tick,worker_dispatched} — use one
of the enumerated event_type values
```

README says otherwise, twice, and its quoted refusal is the 18-value set:

- README.md:899 "the same 18 values below, enforced at the write path"
- README.md:906 "The 18 allowed values, grouped by lifecycle stage:"
- README.md:936-942 quoted `not in vocabulary {…}` block — no `task_type_changed`

`grep -n 'task_type_changed' README.md` → 0 matches: the event the write path
emits on a type change is undocumented anywhere in the README.

It is a real, reachable event — `update <id> --type <slug>` appends it
(internal/board/write.go:1342):

```
$ boardctl-head -C <scratch> update FEAT-2 --type bug        # rc=0
updated task FEAT-2: type, updated_at
$ tail -1 <scratch>/.coding-hermes/board/events.jsonl
{"id": 11, "timestamp": "2026-11-22T08:15:21.150209+00:00",
 "event_type": "task_type_changed", "task_id": "FEAT-2", "actor": "foreman",
 "detail": "{\"old_type\":\"\",\"new_type\":\"bug\"}", "tick_number": null}
```

Provenance: the value arrived with BT-098 (269add6, "typed board rows"), which
is NOT contained in any tag — `git tag --contains 269add6` is empty, and the
published v0.5.0 asset still enumerates the 18-value set. So README.md:899/906
and the quoted block are TRUE of v0.5.0 and FALSE of the current tree and the
next release; a reader who follows the README's own `go build ./cmd/boardctl`
path gets a 19-value vocabulary the docs deny exists.

## NEW FINDING B — umbrella census counts fixture rows; README never says so (P5)

Fixture board: each of 3 boards holds 1 created row + the `NEVER-DONE` fixture
from `fixtures.jsonl`.

```
$ boardctl-head -C myproduct umbrella
UMBRELLA …/myproduct (3 board(s) in view)
  myproduct   2 task(s)  topology A  …/myproduct/.coding-hermes/board
  auth        2 task(s)  topology A  …/services/auth/.coding-hermes/board
  billing     2 task(s)  topology A  …/services/billing/.coding-hermes/board

$ boardctl-head -C myproduct umbrella --list
REPO      ID      PRI  STATUS   TITLE
myproduct PROD-1  P2   pending  own board work        (1 row per board)

$ boardctl-head -C myproduct umbrella --stats
repo myproduct (1 tasks):  total tasks: 1
umbrella total:            total tasks: 3

$ boardctl-head -C myproduct umbrella --list --all      # rc=1
flag provided but not defined: -all
```

README.md:272-273 calls the census "one line per board with per-repo task
counts", and README.md:84-89 teaches that fixture rows are hidden by default
(list prints 2 while `list --all` prints 3). On the umbrella surface the census
reports the fixture-inclusive 2 while `--list`/`--stats` report 1, and no
`--all` exists to reconcile the two — the reader cannot tell from the README
which number is the countable work.

## Re-verification of rows this lane already owns (no status changed)

All ten pending README rows were re-read at origin/main: each carries exactly
one ch:trace marker, and their cited premises still hold at tip 6b6fa0b —
README-013 (four "SOURCE BUILD / NEXT RELEASE ONLY … v0.1.11 asset does not
accept --acceptance-criteria" sites), README-015 (`make version-check
VERSION_TAG=v0.1.11` rc=2), README-016 (`Makefile:245-255` builds when `dist/`
is absent), README-017 (`grep -i vendor README.md` rc=1), README-020 (`grep -in
baseline README.md` rc=0 matches; `evidence` bare → `boardctl: evidence
requires: baseline --record`), README-006-R2 and README-021 (both refusals
reproduced above), README-022 (v0.1.11 examples at README.md:94/98 still
present), README-023 (structure census above).

Pre-existing doc rows README-002/003/004 filed at P3 are already `complete`, so
no priority re-triage applies to this board this tick.

## UNVERIFIED

- `make vuln-check`, `make cover-check` and the release-procedure commands were
  not run this fire (they need network/toolchain legs already covered by
  README-015/022); their README claims are untouched by this sweep.
- Release-time behaviour of the NEXT release (whether `task_type_changed` ships
  in it) is inferred from source, not observed — no tag contains 269add6.
- `serve` was exercised on a loopback port in this tick and killed afterwards;
  that endpoint does not outlive the tick and is not cited as a `live:` witness.
