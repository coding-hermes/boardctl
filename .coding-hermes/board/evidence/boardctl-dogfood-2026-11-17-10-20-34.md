# Dogfood evidence — tick boardctl-dogfood-2026-11-17-10-20-34 (run 30)

Target: boardctl, repo `/home/kara/coding-hermes-boardctl`, HEAD `0179f5b` = `origin/main` (parity 0/0).
Report: `docs/dogfood/2026-11-17-run30-typed-rows-integration.md`; trail: `docs/dogfood/diagnostics.md` Run-30 addendum; skill: `skills/boardctl-usage` 1.26.0.

## Files in this bundle

| file | what it is |
|---|---|
| `…-file_rows.py` | the script that minted nothing and filed DF-BOARDCTL-56/57/58 via `boardctl create` (argv lists, rc checked, read-back printed) |
| `…-file_row59.py` | same for DF-BOARDCTL-59 |
| `…-ci.log` | the act CI transcript pulled from the JIT agent (`qa-logs-4102008/ci.log`) — the Coverage step failure is at lines 48-51 |
| `…-bunker.jsonl` | the QA ledger, 24 rows, pulled by `bunker-qa.sh collect` |

The board writes themselves are in `tasks.jsonl` / `events.jsonl` (one `task_created` per filed id); the scratch boards used for the use-run are NOT durable (they lived in `/home/kara/.hermes/cache/scratch/df-2026-11-17/board1..4` and are reproducible from the commands below).

## Reproduce the P1 (DF-BOARDCTL-56) — 6 commands

```bash
boardctl init -C /tmp/df30
# write /tmp/df30/.coding-hermes/board/types.jsonl (record from skills/boardctl-usage "Typed rows (BT-098)")
boardctl -C /tmp/df30 create --id REL-7 --title 1.7.0 --type release \
  --meta channel=stable --meta source_commit=abcdefg --meta signoffs=ana@x,bo@x
jq -c 'select(.id=="REL-7")|.detail.metadata' /tmp/df30/.coding-hermes/board/tasks.jsonl
# {"channel":"stable","signoffs":["ana@x","bo@x"],"source_commit":"abcdefg"}
boardctl -C /tmp/df30 update REL-7 --meta channel=beta --status pending   # rc=0, "updated task REL-7: status, detail, updated_at"
jq -c 'select(.id=="REL-7")|.detail.metadata' /tmp/df30/.coding-hermes/board/tasks.jsonl
# {"channel":"beta","signoffs":null,"source_commit":"abcdefg"}   <-- list destroyed
grep -c ana@x /tmp/df30/.coding-hermes/board/tasks.jsonl /tmp/df30/.coding-hermes/board/events.jsonl   # 0 and 0
boardctl -C /tmp/df30 validate    # RESULT: OK
```

## Reproduce the coverage red (DF-BOARDCTL-59)

```bash
cd /home/kara/coding-hermes-boardctl && make cover-check          # exit 1, prints "80.0% is below floor 80.0%"
go test ./... -count=1 -short -coverprofile=/tmp/cov.out >/dev/null
awk -F'[: ,]' 'NR>1 {tot+=$4; if ($5>0) cov+=$4} END {printf "%.6f\n", 100*cov/tot}' /tmp/cov.out
# 79.986683   (statements=10513 covered=8409)
git diff --stat 20124ca..HEAD -- cmd internal Makefile .github    # empty: same product tree, green on GitHub CI 43810100006
```

## Verification of the board write

Pre-write baseline reconstructed from `origin/main` (`git show origin/main:.coding-hermes/board/<f>`):
274 rows, `boardctl validate -C <that copy>` → `RESULT: OK (44 warning(s))`.
After filing: 278 rows, `validate --fail-on dangling-dep` rc=0, `RESULT: OK (44 warning(s))`, no warning line naming DF-BOARDCTL-56/57/58/59, `jq -r .id | sort | uniq -d` empty, one `task_created` event per id.

## Install leg facts

`bunker-qa.sh launch` → agent `b11919a8` @ `bunker-mvp` (server 0.2.1, `/tmp` private, `Net mode: shared — NOT tenant isolation`), 24 ledger rows, `qa-run.finished` present, `ci.log` pulled by `bunker exec` before destroy, `bunker destroy b11919a8` rc=0 with its SSH key removed. Grade: install-proven / isolation-UNVERIFIED.

## Perf arms (no PERF row — nothing user-noticeable)

| arm | number |
|---|---|
| `stats` (274-row board, hyperfine warm 10+) | 33.9 ms ± 8.9 |
| `ready --json` | 48.8 ms ± 26.4 |
| `validate` | 82.9 ms ± 56.4 |
| `types` | 5.6 ms ± 3.0 |
| `stats` cold single (`/usr/bin/time`) | wall 0.02 s |
| task-router board (546 rows) `validate`/`stats`/`ready` | 0.08 / 0.04 / 0.05 s |

Host load average during the warm arms: 15.7 on 16 cores, so every figure is an upper bound. Page-cache-cold is unmeasurable without root.

## Not verified (named, not silently skipped)

- Page-cache-cold timings (needs root to drop caches).
- `serve` HTTP surface: not driven this run — QA-BOARDCTL-008 / FOREMAN-2-R2 already own that surface.
- Tenant isolation on the fresh host: the only reachable eligible host reports shared network mode.
