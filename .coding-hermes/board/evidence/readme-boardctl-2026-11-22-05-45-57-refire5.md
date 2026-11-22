# README health sweep — boardctl — tick boardctl-readme-2026-11-22-05-45-57 (re-fire 5)

Lane: coding-hermes-readme. Target: /home/kara/coding-hermes-boardctl.
Stand-in: /home/kara/.hermes/stand-in/readme/boardctl.

Result: filed_rows = 0. This is a redelivery of a tick whose earlier fires
already filed README-013..026. It re-ran the documented command matrix on a
fresh HEAD build and re-measured the premise of every pending README row; no
new defect was found. Seven pending rows received a plain correction note
(refreshed line references / a durable evidence pointer / a re-measured
obsolete-finding record). No row changed status or priority.

## Identity of what was audited

```
$ git -C /home/kara/coding-hermes-boardctl rev-parse HEAD
34d68437dd430af94858f56fbd1a27f3d13523d2
$ git rev-parse origin/main            # equal to HEAD
34d68437dd430af94858f56fbd1a27f3d13523d2
$ git log -1 --format='%h %ad %s' --date=short -- README.md
ea8dcab 2026-11-21 Merge branch 'wt/DF-BOARDCTL-57'
$ git tag --sort=-creatordate | head -3
v0.5.0 (2026-11-15) · v0.4.0 (2026-11-11) · v0.3.0 (2026-11-09)
$ git rev-parse 6b6fa0b:README.md ; git rev-parse HEAD:README.md
2b01a44f61a1fd46e46d37682df33dac0fb0e881
2b01a44f61a1fd46e46d37682df33dac0fb0e881   # identical blob: the doc did not move
$ git diff --stat 6b6fa0b..HEAD -- . ':(exclude).coding-hermes/board' ':(exclude).gitreins'
(empty)                                    # no product/doc change since the refire-3 sweep
```

Freshness (check 1) passes: README last moved 2026-11-21, after the last tag
(v0.5.0, 2026-11-15).

Probe binaries:
- HEAD build: `go build -o boardctl-refire5 ./cmd/boardctl` → `boardctl version dev` (unstamped, as README.md:97-98 describes).
- Published asset: repo-staged `dist/boardctl_linux_amd64` → `boardctl version v0.5.0`.

## Executed this fire (T3)

76 command arms against throwaway boards, plus targeted checks:

- `refire5-matrix.sh` — 56 arms, transcript `refire5-matrix.out` (stand-in).
  Covers init, create (plain/AC/depends/worktree+session/initiative),
  list (+--status, --actionable --json, --all), next, show --events,
  stats / stats --all, validate, doctor, version --json, header,
  update (note/done-alias/type), event (--tick, bogus type, --no-tick-write,
  --detail @file, --task-id/--actor), sweep-status, normalize-board,
  create --deferred, initiatives, types --doctor, create with a malformed id,
  install --dry-run, hook doctor, render -o, evidence, evidence baseline
  --record, and the two documented completion shapes.
- `refire5-premises.sh` — 20 arms, transcript `refire5-premises.out`:
  the published v0.5.0 asset (init/create/AC create/show/AC update/initiatives
  x5/deferred x3/version) and the import board-scope gate.

Behavioural highlights (rcs as observed):

```
$ boardctl-refire5 -C <fresh> create --id FEAT-5 --title "Maybe later" --deferred
boardctl: create: --deferred requires --defer-reason (a deferral without a named blocker is a phantom row; nothing written)   rc=1
$ boardctl-refire5 -C <fresh> update FEAT-1 --status complete --commit-hash abc1234 ...
boardctl: completion REFUSED: --commit-hash "abc1234" must be a full 40- or 64-hex commit id   rc=1
$ boardctl-refire5 -C <fresh> event --type bogus --tick 43 --detail-text x
boardctl: event type "bogus" not in vocabulary {audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_type_changed,task_updated,task_verified,tick,worker_dispatched}   rc=1   # 19 values
$ dist/boardctl_linux_amd64 -C <fresh> create --id REL-002 --title "ac row" --acceptance-criteria "1) x; 2) y"
created task REL-002 ...   rc=0        # published v0.5.0 ACCEPTS the flag
$ dist/boardctl_linux_amd64 -C <fresh> initiatives
INITIATIVE  TOTAL  ACTIONABLE  DEFERRED  BY STATUS
lane-hooks  1      1           0         pending=1     rc=0
$ boardctl-refire5 -C <src-board> render --json export.json   # board-report/v1 payload
$ boardctl-refire5 -C <differently-named-board> import export.json --dry-run --renumber
boardctl: board mismatch: export is for board "proj"; target board is "dstboard" (-C .../dstboard/.coding-hermes/board)   rc=1
```

Note on the render probe: `render`'s flag is `--json out.json` (a filename), not
a boolean; an earlier probe of mine combined `--json` with `-o` and the error it
produced ("render takes no positional args") was my invocation error, not a doc
defect. `render --json` also writes the default `board-report.html`, which is
documented at README.md:515.

## Gates and drift checks run on the audited tree

```
$ make fmt-check                                  rc=0
$ make spec-check                                 rc=0
$ make version-check                              rc=0    (no VERSION_TAG: README pin matches v0.5.0)
$ make version-check VERSION_TAG=v0.5.0           rc=0
$ make version-check VERSION_TAG=v0.1.11          rc=2    (README-015 premise, reproduced)
    --- FAIL: TestVersioncheckExpectedTag
        repo README is stale vs the expected release v0.1.11:
          stale release pin line #1: says "v0.5.0", expected "v0.1.11"
          stale download URL #1/#2: pins "v0.5.0", expected "v0.1.11"
$ check-md-anchors.py README.md                   anchors_checked=6 broken=0
$ readme-link-check.py . README.md                local_missing=0 external_non_200=0 (3 local, 2 external, no badges)
$ readme-structure-census.py README.md            h2_total=20 contents_bullets=0
```

Documented-flag drift arm (new this fire): 91 distinct `--flag` tokens in
README.md compared against the root help plus all 28 subcommand help texts;
11 tokens are absent, and every one is explained — `--abbrev`, `--tags`,
`--generate-notes`, `--verify-tag` (gh), `--ignore-missing` (sha256sum),
`--help`, `--board-dir`/`--by-id`/`--lanes` (the separate `dedupe-board`
binary), `--exempt` (the coverage checker), and `--title-prefix` which
README.md:668 explicitly denies ("there is no `--title-prefix` flag").
No flag drift finding.

## Re-verification of the pending README rows (no status changed)

| row | this fire's measurement at 34d6843 | premise |
|---|---|---|
| README-006-R2 | `--commit-hash abc1234` → rc=1 "must be a full 40- or 64-hex commit id" | holds |
| README-013 | 5 published-asset claims at README.md:106-108, 144-146, 1586-1589, 1619-1621, 1674-1676; v0.5.0 accepts the flag (create/update rc=0) | holds |
| README-014 | v0.5.0 runs create/update/list/initiatives/initiatives --json with rc=0 | core finding OBSOLETE |
| README-015 | five-gate block at README.md:1762-1770, `VERSION_TAG=v0.1.11` at 1766 → rc=2; current tag rc=0 | holds |
| README-016 | README.md:1910-1913 (and 43-48) vs Makefile:245-255 source-build fallback | holds |
| README-017 | `grep -c vendor README.md` → 0; Makefile:101/108; ci.yml:41 | holds |
| README-020 | `grep -c 'evidence baseline' README.md` → 0; verified_complete_share now at README.md:1311 | holds |
| README-021 | bare `--deferred` refused on HEAD and on v0.5.0 (create and update); `grep -c defer-reason README.md` → 0 | holds |
| README-022 | 29 v0.1.11 mentions; README.md:17 says v0.5.0 | holds |
| README-023 | h2_total=20, contents_bullets=0 | holds |
| README-024 | README.md:899/906 say 18; HEAD refuses with 19 (task_type_changed); `grep -c task_type_changed README.md` → 0 | holds |
| README-025 | umbrella census prints 2 task(s) while `--list`/`--stats` print 1 on a fixture-bearing board; `umbrella --list --all` → "flag provided but not defined: -all" rc=1 | holds |
| README-026 | import board-mismatch refusal reproduced (rc=1); README.md:531 promises "round-trips a report into another board" | holds |

Line references refreshed in 7 rows by plain notes (no status/priority change):
README-013, 014, 015, 016, 017, 020, 026.

Priority re-triage: the board's only open pre-2026-09-30 doc row is
DF-BOARDCTL-54 (P3), whose finding is that two documented recipes are refused —
a broken documented path, which the ruling keeps out of the P4/P5 demotion set.
No re-triage applied.

## UNVERIFIED

- The published-release download leg was NOT executed this fire: the
  `curl -sL https://github.com/coding-hermes/boardctl/releases/download/v0.5.0/...`
  fetch was blocked by the unattended-surface gate (no consent within the
  window) and was not retried in another form. The v0.5.0 probes therefore used
  the repo-staged `dist/boardctl_linux_amd64` (identity `boardctl version
  v0.5.0`), not a freshly downloaded asset; the README install block's live
  URLs remain verified only by the earlier fire (HTTP 200).
- `make vuln-check` and `make cover-check` were not run (network/toolchain legs,
  unchanged by this sweep).
- The `serve` HTTP surface and the four header `--set-*` flags were verified at
  refire 3 and were not re-exercised this fire.
- `update --clear-initiative` was not run in the README-014 re-measure
  (the other five documented initiative commands were).
