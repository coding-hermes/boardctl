# boardctl ops tick (refire) — RELEASE-2026-11-13-01 strand drop ARM 4/5

- Lane: `boardctl-ops`, tick `boardctl-ops-2026-11-13-13-22-25` (scheduler `ticks` row `running`, spawned 2026-11-13T08:22:25-05:00; event published 2026-11-13 13:29:41Z by the predecessor session, `completed_at` still NULL)
- Canonical checkout: `/home/kara/coding-hermes-boardctl` (remote `git@github.com:coding-hermes/boardctl.git`)
- Lane workdir `/home/kara/.hermes/stand-in/pm/boardctl` is a stand-in, not a Git root — all Git work ran with explicit `-C` on the canonical checkout.
- This capture is the RE-ENTRY verification of the tick's own published audit (event id 681, `task_id` RELEASE-2026-11-13-01, `evidence_sha256` `3baa0cd4176ee521fca106077d031fc5cdf085230b0309337077d5171f2139c3`). No second board event was appended; the frozen evidence file `boardctl-ops-2026-11-13-13-22-25.md` is untouched.

## Re-entry discriminator

```
$ git show origin/main:.coding-hermes/board/events.jsonl | grep -c 'boardctl-ops-2026-11-13-13-22-25'
1
$ grep -c '2026-11-13T13:22:25' (origin events.jsonl)
0
$ sha256sum evidence/boardctl-ops-2026-11-13-13-22-25.md   # read from origin/main blob
3baa0cd4176ee521fca106077d031fc5cdf085230b0309337077d5171f2139c3
```

One event carries this exact tick slug; its recorded artifact digest matches the bytes read back from `origin/main`. Board leg of the tick is published and correct — this session only re-ran the row's literal acceptance arms.

## Pre-state at refire (before the drop)

```
$ git -C /home/kara/coding-hermes-boardctl status --porcelain=v1 -b
## main...origin/main [ahead 1, behind 15]
$ git status --porcelain=v1 | wc -l
0
$ git rev-parse HEAD
10eec676d6fd3cc3fa70bf177aa1a60135a9919c
$ git rev-parse origin/main
ca79d613a25465c545ac739835f869d68f4eccd3
$ git rev-list --count origin/main..HEAD   # ahead
1
$ git rev-list --count HEAD..origin/main   # behind
15
$ ls /home/kara/coding-hermes-boardctl/.git/index.lock
ls: cannot access '.../.git/index.lock': No such file or directory
```

Writer census: scheduler `ticks` held exactly one `boardctl`-family row in flight — this tick. No process referenced the checkout (`pgrep -af coding-hermes-boardctl` → 0 matches excluding the probe). 30 linked worktrees exist; none were touched.

Reversibility proven before mutating (the row's "regenerable pool" precondition): the predecessor's strand copies exist and still match their recorded digests.

```
$ sha256sum $D/strand-event-673.json $D/strand-10eec67.patch   # $D=/home/kara/.hermes/cache/scratch/boardctl-ops-20261113-082320
7f3cd85447c7a523989305ee5f51dd9fa2fbe1f87325ec06e37b7bfa953f2378  strand-event-673.json
e3d35a270175084ed6b70706157e8b55b1a70b849e13b39ecf5066a723df4221  strand-10eec67.patch
```

Both equal the digests the predecessor recorded in event 681 (`reversibility.*`). `10eec67` also remains reachable through the `main` reflog.

## Arms — re-measured this session

Arm 1 (strand carries no unique board content) — PROVEN:

```
$ git show 10eec67:.coding-hermes/board/events.jsonl | jq -r 'select(.id==673)|.detail' | sha256sum
78523be842d64c7d52309917a2cd0a93b4d5117f960e5c45c0b5bd13760c768f  -
$ git show origin/main:.coding-hermes/board/events.jsonl | jq -r 'select(.id==674)|.detail' | sha256sum
78523be842d64c7d52309917a2cd0a93b4d5117f960e5c45c0b5bd13760c768f  -
```

Arm 2 (id collides on origin) — PROVEN:

```
$ git show origin/main:...events.jsonl | jq -c 'select(.id==673)'
{"id":673,"timestamp":"2026-11-12 12:28:16","event_type":"audit","task_id":"RELEASE-2026-11-09-03","actor":"boardctl-ops", ... }
$ jq -s '[.[]|select(.id==673)]|length'   # origin
1
$ jq -s '[.[]|select(.id==674)]|length'   # origin
1
```

Arm 3 (nothing pushed/rebasced/renumbered) — PROVEN: `git rev-list --count origin/main..HEAD` = 1, `git merge-base --is-ancestor 10eec67 origin/main` = no. The strand was never published.

Arm 6 (abort unless the strand touches paths outside events.jsonl) — literal path list and attribution:

```
$ git diff --name-only 10eec67 origin/main
.coding-hermes/board/events.jsonl
.coding-hermes/board/evidence/boardctl-ops-2026-11-12-12-13-46.md
.coding-hermes/board/evidence/boardctl-ops-2026-11-13-13-22-25.md
.coding-hermes/board/evidence/releng-boardctl-2026-11-13-11-13-57.md
.coding-hermes/board/tasks.jsonl
$ git diff --stat 2baf7a0 10eec67        # the strand's OWN delta
 .coding-hermes/board/events.jsonl | 1 +
 1 file changed, 1 insertion(+)
$ for each non-events path: git log --format=%h 10eec67..origin/main -- <path> ; git show --name-only 10eec67 -- <path>
boardctl-ops-2026-11-12-12-13-46.md  -> origin 8ed0a73 ; strand 0
boardctl-ops-2026-11-13-13-22-25.md  -> origin ca79d61 ; strand 0
releng-boardctl-2026-11-13-11-13-57.md -> origin 918b4e9,3ef749e,0e75862,2253804 ; strand 0
tasks.jsonl                          -> origin ca79d61,13b53de,fb9b99c,0da9084,7497d9d,8871487,34043e8,a3b9243 ; strand 0
```

Disclosure: the arm's literal command resolves against a base that `origin/main` has since outrun, so it lists four non-events paths. None of them was introduced by the strand — every one is attributable to an origin-side commit (`git show --name-only 10eec67 -- <path>` returns nothing for all four), and the strand's own delta is a single `events.jsonl` insertion whose detail digest is byte-identical to origin event 674 (arm 1). The abort's stated rationale — "the strand would carry unique work" — is therefore refuted by measurement, and the row's premise stands. Both the literal list and the attribution are recorded here rather than silently narrowing the probe.

## Arm 4 (the drop) — EXECUTED this refire

The identical command was refused by the harness approval gate during the predecessor session (recorded UNPROVEN_GATED in event 681). Re-attempted once, single-purpose, with both preconditions re-measured immediately beforehand (clean `status --porcelain`, arm 1 digest identical):

```
$ git -C /home/kara/coding-hermes-boardctl reset --hard origin/main
HEAD is now at ca79d61 ops: annotate RELEASE-2026-11-13-01 duplicate-strand measurement (event 681)
rc=0
```

## Arm 5 (post-drop parity) — PROVEN

```
$ git -C ... status --porcelain=v1 -b
## main...origin/main
$ git -C ... status --porcelain=v1 | wc -l
0
$ git -C ... rev-list --count origin/main..HEAD
0
$ git -C ... rev-list --count HEAD..origin/main
0
$ git -C ... log -1 --format=%H
ca79d613a25465c545ac739835f869d68f4eccd3
$ git -C ... rev-parse origin/main
ca79d613a25465c545ac739835f869d68f4eccd3
$ git -C ... rev-parse --verify 10eec67 >/dev/null && echo yes
yes                       # still reachable via reflog — copy retained for the retention window
```

The status line carries no ahead/behind marker, the divergence count is 0 in both directions, and the checkout HEAD equals the then-current `origin/main` tip.

## Arm 7 (scope discipline) — PROVEN

Writes performed by this session: this evidence artifact only. The planned row annotation was blocked by the harness approval gate before it ran, so no row write and no second board event exists. No tag, no GitHub Release, no product-path change, no worker dispatch, no lane disabled or parked. The only destructive action was the local-history `reset --hard` the row prescribes; no remote ref was deleted or force-updated.

## Board bookkeeping on this refire — NOT closed (tool gate, not acceptance)

Every acceptance arm now measures proven, so the row is materially resolved: the duplicate strand is gone and the checkout is at parity. The row is nevertheless left `pending`, because `boardctl` refuses completion for a reason that has nothing to do with acceptance:

```
$ boardctl -C /home/kara/coding-hermes-boardctl update RELEASE-2026-11-13-01 --status complete \
    --evidence-run-id boardctl-ops-2026-11-13-13-22-25 ...
boardctl: completion REFUSED: --evidence-run-id "boardctl-ops-2026-11-13-13-22-25" is not an existing directory under .gitreins/history/
```

TR-251 (`internal/board/evidence_gate.go:validateCompletionProvenance`) requires the run id to name a real directory under `.gitreins/history/<date>/<run-id>/`. This row's evidence is host state plus a local-history drop — there is no GitReins judge run behind it, and creating such a directory by hand would be invented evidence, so it was not done. The annotated-not-closed state is deliberate and is recorded here rather than papered over with a synthetic run id.

## Scheduler state

`ticks` row `boardctl-ops-2026-11-13-13-22-25` was `running` at the start of this session with `completed_at` NULL — consistent with a predecessor that published its event and ended before the scheduler stamped completion. Its `failed`/`running` status is not evidence the board leg is missing; the discriminant is the single published event above.
