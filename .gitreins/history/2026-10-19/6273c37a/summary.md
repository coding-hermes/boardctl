# Verdict: DOC-36

**Task:** Document list/stats fixture filtering
**Evaluated:** 2026-10-19T22:58:33.345177
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Traceability: row=DOC-36; README documents list --all and stats --all, explaining default fixture exclusion and exact counts; test/evidence=README.md plus live scratch-board probe; commit resolves the row; witness=none:docs-only: Partial. README.md:71 documents `list --all` ("include fixture rows (hidden by default)") and README.md:79-82 documents `stats --all` with the default-exclusion explanation ("By default, list and stats exclude fixture rows registered in fixtures.jsonl... this can make the count differ"). Commit c573466 ("docs: document fixture-inclusive list and stats counts / Addresses DOC-36.") touches only README.md (+6). Live scratch-board probe (/tmp/doc36probe with fixtures.jsonl registering NEVER-DONE) confirms behavior: `list` -> 2 rows (SCR-0, SCR-1) vs `list --all` -> 3 rows (NEVER-DONE, SCR-0, SCR-1); `stats` -> total tasks: 2 / pending 2 / P2 2 vs `stats --all` -> total tasks: 3 / pending 3 / P2 2 / P3 1. FAILURES: (1) "exact counts" not met — README never quotes the observed 2-vs-3 line difference that the task's ACCEPT clause explicitly requires; it only says counts "can differ". (2) "commit resolves the row" not met — .coding-hermes/board/tasks.jsonl:181 still shows DOC-36 status "pending" with no commit_hash/worker_summary/foreman_note, and .coding-hermes/board/events.jsonl contains only the task_created event (id 466) for DOC-36, no task_completed/closeout event. (3) No traceability marker: `grep ch:trace README.md` = 0 hits and DOC-36's row carries no ch:trace foreman_note (unlike DOC-34/DOC-35 rows). (4) Unrelated dirty file cmd/boardctl/board-report.html (rendered_at timestamp churn) left uncommitted.
README gained the list/stats --all fixture-exclusion documentation and the behavior is confirmed by a live probe, but the required exact 2-vs-3 counts are absent and the DOC-36 board row/commit traceability (status, completion event, ch:trace marker) was never closed out.

## Summary

Judge Result: DOC-36

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Traceability: row=DOC-36; README documents list --all and stats --all, explaining default fixture exclusion and exact counts; test/evidence=README.md plus live scratch-board probe; commit resolves the row; witness=none:docs-only: Partial. README.md:71 documents `list --all` ("include fixture rows (hidden by default)") and README.md:79-82 documents `stats --all` with the default-exclusion explanation ("By default, list and stats exclude fixture rows registered in fixtures.jsonl... this can make the count differ"). Commit c573466 ("docs: document fixture-inclusive list and stats counts / Addresses DOC-36.") touches only README.md (+6). Live scratch-board probe (/tmp/doc36probe with fixtures.jsonl registering NEVER-DONE) confirms behavior: `list` -> 2 rows (SCR-0, SCR-1) vs `list --all` -> 3 rows (NEVER-DONE, SCR-0, SCR-1); `stats` -> total tasks: 2 / pending 2 / P2 2 vs `stats --all` -> total tasks: 3 / pending 3 / P2 2 / P3 1. FAILURES: (1) "exact counts" not met — README never quotes the observed 2-vs-3 line difference that the task's ACCEPT clause explicitly requires; it only says counts "can differ". (2) "commit resolves the row" not met — .coding-hermes/board/tasks.jsonl:181 still shows DOC-36 status "pending" with no commit_hash/worker_summary/foreman_note, and .coding-hermes/board/events.jsonl contains only the task_created event (id 466) for DOC-36, no task_completed/closeout event. (3) No traceability marker: `grep ch:trace README.md` = 0 hits and DOC-36's row carries no ch:trace foreman_note (unlike DOC-34/DOC-35 rows). (4) Unrelated dirty file cmd/boardctl/board-report.html (rendered_at timestamp churn) left uncommitted.
README gained the list/stats --all fixture-exclusion documentation and the behavior is confirmed by a live probe, but the required exact 2-vs-3 counts are absent and the DOC-36 board row/commit traceability (status, completion event, ch:trace marker) was never closed out.

Overall: FAIL ✗
