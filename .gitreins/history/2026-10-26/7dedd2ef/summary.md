# Verdict: QA-BOARDCTL-RELENG-1

**Task:** boardctl releng battery second collect pass
**Evaluated:** 2026-10-26T11:25:40.240194
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ 1) bunker-qa launch against /home/kara/coding-hermes-boardctl produces status OK with agent id. 2) collect on the printed evidence path exits 0 with OK or RUNNING. 3) evidence contains no detail with no-agent.: 1) /tmp/qa-boardctl-releng1/launch.log: 'QA LAUNCH project: coding-hermes-boardctl — detached battery on bunker-mvp' and 'LAUNCHED agent=af9c28e8 evidence=/tmp/qa-boardctl-releng1/evidence.jsonl'; evidence.jsonl.meta confirms project=coding-hermes-boardctl, agent=af9c28e8, server=bunker-mvp, and the launch cell row is status OK with detail 'agent=af9c28e8 server=bunker-mvp ttl=4h ...'. 2) /tmp/qa-boardctl-releng1/poll.log attempts 1-4 show 'rc=0 ... RUNNING agent=af9c28e8' and attempt 5 shows 'rc=0 ... COLLECTED agent=af9c28e8 rows=19'; re-running `bash /home/kara/.hermes/scripts/bunker-qa.sh collect --evidence /tmp/qa-boardctl-releng1/evidence.jsonl` returned 'COLLECTED already-collected' with EXIT=0. 3) `grep -n 'no-agent' evidence.jsonl evidence.jsonl.chaos-disconnect.log` returned no matches (grep_rc=1, count=0); all 22 evidence rows carry real details (e.g. 'agent=af9c28e8 (bunker-mvp) — pulled 19 evidence row(s)').
Launch produced OK with agent af9c28e8 for coding-hermes-boardctl, collect on the printed evidence path exits 0 (RUNNING then COLLECTED), and the evidence contains no 'no-agent' detail.

## Summary

Judge Result: QA-BOARDCTL-RELENG-1

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ 1) bunker-qa launch against /home/kara/coding-hermes-boardctl produces status OK with agent id. 2) collect on the printed evidence path exits 0 with OK or RUNNING. 3) evidence contains no detail with no-agent.: 1) /tmp/qa-boardctl-releng1/launch.log: 'QA LAUNCH project: coding-hermes-boardctl — detached battery on bunker-mvp' and 'LAUNCHED agent=af9c28e8 evidence=/tmp/qa-boardctl-releng1/evidence.jsonl'; evidence.jsonl.meta confirms project=coding-hermes-boardctl, agent=af9c28e8, server=bunker-mvp, and the launch cell row is status OK with detail 'agent=af9c28e8 server=bunker-mvp ttl=4h ...'. 2) /tmp/qa-boardctl-releng1/poll.log attempts 1-4 show 'rc=0 ... RUNNING agent=af9c28e8' and attempt 5 shows 'rc=0 ... COLLECTED agent=af9c28e8 rows=19'; re-running `bash /home/kara/.hermes/scripts/bunker-qa.sh collect --evidence /tmp/qa-boardctl-releng1/evidence.jsonl` returned 'COLLECTED already-collected' with EXIT=0. 3) `grep -n 'no-agent' evidence.jsonl evidence.jsonl.chaos-disconnect.log` returned no matches (grep_rc=1, count=0); all 22 evidence rows carry real details (e.g. 'agent=af9c28e8 (bunker-mvp) — pulled 19 evidence row(s)').
Launch produced OK with agent af9c28e8 for coding-hermes-boardctl, collect on the printed evidence path exits 0 (RUNNING then COLLECTED), and the evidence contains no 'no-agent' detail.

Overall: FAIL ✗
