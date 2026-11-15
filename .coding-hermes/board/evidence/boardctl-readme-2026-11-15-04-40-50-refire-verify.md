# boardctl README sweep — re-fire verification

Tick: `boardctl-readme-2026-11-15-04-40-50` (lane `boardctl-readme`, stand-in `/home/kara/.hermes/stand-in/readme/boardctl`)
Produced: 2026-11-15 ~05:45Z, by the readme audit lane.
This artifact is committed with the row it evidences; it is the frozen record of what was RUN, not a plan.

## Published-tip identity and equivalence proof

| fact | value |
|---|---|
| tip when the prior delivery produced its transcript | `70715c5` (== origin/main then) |
| tip when this re-fire probed | `ea6c4a8` (fast-forwarded; sibling commit `chore: close recovered boardctl wave manifest`) |
| paths changed between the two tips | `.coding-hermes/waves/boardctl-foreman-2026-11-14-22-23-50.json` only (`git diff --stat 70715c5..ea6c4a84` = 1 file, +2 lines) |
| README.md blob at BOTH tips | `d4e411f6115c808e1b0263c272074e97c688174e` |
| README.md sha256 (working tree, == tip) | `30c8b0a5354c0dc37505154ec109aa5b8288a0d3439f21c8ca3b837eafbb6155` |
| latest tag (`git describe --tags --abbrev=0`) | `v0.4.0` |
| README.md:17 declares | `Current release: **v0.4.0**.` → freshness arm: MATCH |
| release asset probed | `boardctl_linux_amd64` v0.4.0, sha256 `b07ef620c9bf4c87a0ab13e4a668d2f94b0414680ffc3db664c222513b7bcc55`, downloaded via the README install block |

Re-fire classification: rows `README-018` and `README-019` exist (filed 2026-11-15 04:58:03Z by the prior
delivery of this same tick label) and are published at `70715c5`; NO `audit` event for this tick label existed
in `events.jsonl` (`grep -c '04-40-50' events.jsonl` = 0). Per the skill's re-fire rule this fire is
verify-only: no ID re-minting for the same findings, one audit event appended, plus one correction to a row
whose premise the v0.4.0 publication killed.

## Re-verification of the two already-filed rows

A. `README-018` (P4, broken anchor) — RE-VERIFIED LIVE at the published tip:

```
$ python3 /home/kara/.hermes/skills/coding-hermes/coding-hermes-readme/scripts/check-md-anchors.py README.md
BROKEN README.md:1802: #go-toolchain-policy
anchors_checked=7 broken=1
rc=1
```

README.md:1802 is a link to `#go-toolchain-policy`; `grep -n 'go-toolchain-policy' README.md` returns that one
line, and no heading in the document declares that slug (checker checked 7, 1 broken). Defect stands.

B. `README-019` (P4, `../../releases` link) — RE-VERIFIED LIVE at the published tip:

```
$ curl -s -o /dev/null -w '%{http_code}' -L https://github.com/coding-hermes/releases      -> 404
$ curl -s -o /dev/null -w '%{http_code}' -L https://github.com/coding-hermes/boardctl/releases -> 200
```

`grep -n '](\.\./\.\./' README.md` finds the Install block's `[releases](../../releases)` at README.md:13,
which resolves to the org page (404) instead of the repo's releases page (200). Defect stands.

## Correction to README-014 — its premise died with the v0.4.0 release

`README-014` (filed 2026-11-08) claimed the initiatives surface is presented as current while "the published
v0.2.0 asset refuses it". The published asset is now v0.4.0, which the README itself names at line 17, and
every command in the section's fenced block (README.md:1381-1386) exits 0 on it. Each command was run as its
own invocation against a fresh board created by `boardctl init`:

```
$ boardctl create --id FEAT-9 --title "arm the leg" --initiative lane-hooks
created task FEAT-9 (appended to <scratch>/.coding-hermes/board/tasks.jsonl)                 rc=0
$ boardctl update FEAT-9 --initiative lane-hooks
updated task FEAT-9: initiative, updated_at                                                  rc=0
$ boardctl update FEAT-9 --clear-initiative
updated task FEAT-9: initiative, updated_at                                                  rc=0
$ boardctl list --initiative lane-hooks --json
[ { "id": "FEAT-9", ... }, ... ]                                                             rc=0
$ boardctl initiatives
INITIATIVE  TOTAL  ACTIONABLE  DEFERRED  BY STATUS
lane-hooks  1      1           0         pending=1                                           rc=0
$ boardctl initiatives --json --status pending
[ { "initiative": "lane-hooks", "total": 1, "actionable": 1, "deferred": 0,
    "by_status": { "pending": 1 } } ]                                                        rc=0
$ make version-check
ok  github.com/coding-hermes/boardctl/internal/versioncheck  0.007s                           rc=0
```

Disposition: `README-014`'s Option B acceptance criterion is satisfied by the v0.4.0 publication. The row was
amended in place (title + dated CORRECTION block + acceptance criteria) rather than closed by this lane —
status transitions and completion evidence belong to the owning foreman.

## New finding filed this fire — `README-020`

`boardctl evidence baseline --record` is the only command that surrenders historic completion debt and turns
on FAIL-on-new-debt, and the README never names it, while the README *does* teach the debt metric a reader
will hit:

```
$ grep -n 'verified_complete_share' README.md
1268:`verified_complete_share` plus the itemized complete rows lacking a run:
$ grep -in 'baseline' README.md
(no output; 0 matches in 1936 lines)
$ grep -n 'boardctl evidence' README.md
(no output)
```

The command is real and read-back-proven on the v0.4.0 asset:

```
$ boardctl evidence baseline
boardctl: evidence requires: baseline --record                                             rc=1
$ boardctl evidence baseline --record
completion evidence baseline recorded: 0 debt entries                                      rc=0
```

Reader harm: a stranger told by `doctor` that they carry an itemized "complete rows lacking a run" debt has no
documented remedy path; the only working form (`evidence baseline --record`) appears nowhere in the 1936-line
front door, and the bare `evidence` form they would guess exits 1.

## Dedupe performed before minting

- `ready` / `direction` are undocumented in the README — ALREADY FILED as `DOC-41` (P4, pending, filed
  2026-11-10). Not re-filed; `DOC-41` already carries the executed probes and the graph_cmd.go file:lines.
- `next-id` is undocumented too, but it is a read-only convenience for the same minting duty that
  `~/.hermes/scripts/board_next_id.py` already covers fleet-wide; recorded here as a note, not filed, to avoid
  a second sibling row for the same "undocumented verb" class.
- Open `README-013` (acceptance-criteria release-boundary text) still describes README text that is still
  present at the tip (README.md:106/144/1518/1547 assert the v0.1.11 limitation while v0.4.0 accepts the flag);
  left untouched — still true.
- No open README/DOC row mentions `baseline`, so `README-020` is not a duplicate.

## Arms and their status this fire

| arm | status |
|---|---|
| freshness vs last release | RUN — tag v0.4.0 == README.md:17 |
| documented-command matrix (all 25 verbs) | REUSED from the prior delivery's transcript (README blob identical at both tips) + freshly executed for the initiatives/evidence/ready/direction/next-id subset |
| stranger install | REUSED — prior delivery: public clone + `go install`, and release-asset download + SHA256SUMS verify (asset re-verified here: `boardctl version` = v0.4.0) |
| reader-question structure | PARTIAL — not re-audited this fire; prior delivery's structure pass stands, no defect filed |
| links / anchors / badges | RUN — anchor checker rc=1 (1 of 7 broken), org-level release link 404 vs control 200 |
| external link sweep of every URL | UNVERIFIED this fire — only the release links and the anchor targets were re-probed |
