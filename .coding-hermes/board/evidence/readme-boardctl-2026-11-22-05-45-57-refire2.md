# README health sweep — boardctl — tick boardctl-readme-2026-11-22-05-45-57 (re-fire 2)

Lane: coding-hermes-readme. Stand-in: /home/kara/.hermes/stand-in/readme/boardctl
Role of this fire: **verify-only readback battery + one inherited-row repair**. The sweep
itself (command matrix A–D, structure/anchors/links/gates, four findings) was produced by the
first delivery of this same tick and is published as
`.coding-hermes/board/evidence/readme-boardctl-2026-11-22-05-45-57.md` (sha256
`680b74bd1736dd0c23a3b7592a8969293c322368d2f99dacb1beaf6a36a0f6fa`). Re-deriving those arms is
forbidden by the lane procedure, so they are not re-executed here; they are listed as
not-re-executed below.

## Identity at this fire

```
repo toplevel:            /home/kara/coding-hermes-boardctl
remote:                   origin  git@github.com:coding-hermes/boardctl.git
upstream / published ref: origin/main
opening tip (HEAD):       33880e0999361a484cde050a07d607608840599f
remote HEAD (ls-remote):  33880e0999361a484cde050a07d607608840599f   (equal — nothing unpushed)
README blob at origin/main: 2b01a44f61a1fd46e46d37682df33dac0fb0e881
README content sha256:      990e509163e6799de4790a39e6fc9367bad8dae75a6e230ceb2c32a6cf07824b
  proof: sha256sum README.md  ==  git cat-file -p origin/main:README.md | sha256sum
  (identical to the digest the first fire recorded — the audited bytes did not move)
release boundary:         newest tag v0.5.0; README.md:17 "Current release: **v0.5.0**."
worktree cleanliness:     only untracked .gitreins/review-state.json + .lock (not board files)
```

## Tick-delivery census (step 2)

```
events citing boardctl-readme-2026-11-22-05-45-57 at origin/main: 1
  (763, 'audit', task_id=None, '2026-11-22 06:36:15')      <- untargeted audit event, correct shape
total events in events.jsonl: 767
rows filed by this tick (first delivery): README-006-R2, README-021, README-022, README-023
notes left by this tick on inherited rows: README-013, README-014, README-015, README-016, README-020
```

Verdict: the tick is a delivered re-fire. No id minted, no second audit event appended.

## Readback battery — every open README row at origin/main

Command:
`python3 ~/.hermes/skills/coding-hermes/coding-hermes-readme/scripts/board-row-marker-readback.py --repo /home/kara/coding-hermes-boardctl --ref origin/main --ids README-006-R2,README-013,README-014,README-015,README-016,README-017,README-020,README-021,README-022,README-023`

Raw output: `readback-pre.txt` (stand-in run dir refire2-20261122).

| row | status | prio | marker | marker field | evidence sha256 |
|---|---|---|---|---|---|
| README-006-R2 | pending | P4 | OK | foreman_note | 680b74bd… (refire artifact) |
| README-013 | pending | P4 | OK | foreman_note | 680b74bd… |
| README-014 | pending | P4 | OK | foreman_note | e890d5e9… (2026-11-15 refire-verify) |
| README-015 | pending | P4 | OK | foreman_note | 680b74bd… |
| README-016 | pending | P4 | OK | foreman_note | 680b74bd… |
| README-017 | pending | P5 | **COUNT=0** | — | (row detail: 11-08 sweep artifact) |
| README-020 | pending | P4 | OK | reasoning | e890d5e9… |
| README-021 | pending | P4 | OK | foreman_note | 680b74bd… |
| README-022 | pending | P5 | OK | foreman_note | 680b74bd… |
| README-023 | pending | P5 | OK | foreman_note | 680b74bd… |

`ROWS_FAILING=1` — the single failure is README-017's absent marker.

## Inherited debt vs this fire's regression (measured, not assumed)

Pending rows without any `ch:trace` marker, counted per tip with the board decoded line by line:

```
587a9e6^ (pre-fire):    pending=38  pending_without_marker=29   incl. README-013, README-015, README-016, README-017
587a9e6  (audit evt):   pending=38  pending_without_marker=29
33880e0  (this tip):    pending=38  pending_without_marker=26   incl. README-017  (013/015/016 repaired by this tick)
```

README-017 was already markerless before this tick fired (filed 2026-11-08), so its missing
marker is INHERITED debt, not a regression of this tick. The 25 other markerless pending rows
are outside this lane's family (BT-*, DF-*, QA-*, RELEASE-*, NEVER-DONE, PERF-001) and are
reported, not touched.

## README-017 re-probe (still-live reading + moved line numbers)

Row premise: CI runs a vendor gate (`make vendor-check` / `vendor-strip`) the README never
names. Re-probed at origin/main (README blob `2b01a44f…`):

```
$ grep -n -i 'vendor' README.md                       -> no match (rc=1)
$ grep -nE '^[a-z-]+:' Makefile | grep -i vendor      -> 101:vendor-check:   108:vendor-strip:
$ grep -n 'name:' .github/workflows/ci.yml            -> 41: Vendor-check   (run: go test -count=1 -run TestVendorcheck ./internal/vendorcheck)
$ sed -n '37,46p' .github/workflows/ci.yml            -> 37-40 the BT-087 comment naming `make vendor-strip` as the remedy
```

Premise STILL LIVE. Line citations have MOVED since filing:

| row says | actual at this tip |
|---|---|
| "Development block 1590-1600" | README.md:1713-1725 (gates at 1719-1724) |
| "ci.yml description 1571" | README.md:1689 (repo-layout table row) |
| "ci.yml:37-42" | unchanged: .github/workflows/ci.yml:37-42 |

Row acceptance criterion 3's step order still matches ci.yml exactly:
22 Build, 25 Vet, 28 Gofmt, 31 Version-check, 34 Spec-check, 41 Vendor-check,
47 Install govulncheck v1.7.0, 50 Vulnerability-check, 56 Coverage, 59 Test.

Action taken: appended a re-probe note with these current coordinates and repaired the row's
missing marker (marker cites THIS artifact). The row's status, title finding and priority (P5,
doc severity by the 2026-09-30 ruling) are unchanged; the close belongs to the owning foreman.

## Doc-severity re-triage census (step 10)

```
python3 ~/.hermes/skills/coding-hermes/coding-hermes-readme/scripts/readme-doc-severity-census.py \
    --repo /home/kara/coding-hermes-boardctl --ref origin/main
doc rows: 52 (pending: 13); by priority P2=8 P3=3 P4=33 P5=8; by status complete=39 pending=13
pending doc rows at P1-P3 below the P4/P5 ceiling: none
pending P1-P3 doc rows created before 2026-09-30: none
```

Duty discharged by measurement: no demotion owed, nothing re-filed.

## Board validator at this fire

```
boardctl -C /home/kara/coding-hermes-boardctl validate   (pre-write)
RESULT: OK — errors=0, warn lines=44, exit 0
```

## Not re-executed this fire (carried from the first delivery, NOT a claim here)

Matrix A reader-facing commands, Matrix B fingerprint/deferred/hook/dedupe contracts,
Control C completion path, Matrix D initiatives surface, structure/anchors/links censuses,
the five local make gates, and the stranger-install arm were executed by the first delivery of
this tick and are published with their records under the stand-in run dirs named in that
artifact. This fire re-read only row state, marker integrity, evidence digests, the README
blob identity, README-017's premise, and the doc-severity census.

Still UNVERIFIED fleet-wide (unchanged from the first delivery): published-release asset
execution (download + `./boardctl_linux_amd64 version`, `sha256sum -c`) was refused by the
unattended consent gate and not retried; `boardctl serve` was not started; `hook install
--apply` / `hook uninstall` not run; `make install`/`release`/`vuln-check`/`spec-check`/
`cover-check`/`fmt-check`/`install-check` not run this fire (README-015/016 carry those rows).
