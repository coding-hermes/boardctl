# Verdict: DOC-33

**Task:** Document event --task-id, --actor and --detail @file flags
**Evaluated:** 2026-10-19T19:59:11.479192
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README names all three flags with row keys; documented invocation runs against scratch board and events.jsonl row carries task_id/actor/detail keys: README.md:551-568 '### Event flags (DOC-33)' names all three flags with their row keys: `--task-id ID` (sets `task_id`), `--actor NAME` (sets `actor`), `--detail @file` (sets `detail`, mutually exclusive with `--detail-text`); README.md:130-134 and 575-579 show the combined invocation. Verified by execution: `go build -o /tmp/doc33/boardctl ./cmd/boardctl` → BUILD_OK; `boardctl init --project demo --namespace demo` in /tmp/doc33; then the documented command `./boardctl -C /tmp/doc33 event --type audit --tick 44 --task-id BT-042 --actor foreman --detail @/tmp/detail.json` → exit=0, 'event id 1 appended'. Resulting .coding-hermes/board/events.jsonl row: {"id": 1, "timestamp": "2026-10-19T19:58:41...", "event_type": "audit", "task_id": "BT-042", "actor": "foreman", "detail": "{\"note\":\"structured\",\"n\":1}", "tick_number": 44} — all three keys present. Flag wiring confirmed at cmd/boardctl/main.go:1012-1016, 1063-1090. Test suite: `go test -count=1 ./...` exit_code=1, but all failures are pre-existing sandbox-environment failures unrelated to this doc change (e.g. internal/freshness TestAtHEADSilent: 'git commit --allow-empty -q -m c1: exit status 1 / gate: GitReins NOT RUN — .gitreins/config.yaml is absent'; same for internal/board TestDoctorFossilWarnRealExecParity and cmd/boardctl TestBT057StaleBinaryForeignRepoSilent); packages internal/render, internal/speccheck, internal/vulncheck, internal/workflowcheck, internal/fmtcheck all report 'ok'. The only changed file in the diff is cmd/boardctl/board-report.html (rendered_at timestamp), which cannot affect these tests.
README documents all three event flags with their row keys and the documented invocation was executed against a scratch board, producing an events.jsonl row carrying task_id, actor and detail.

## Summary

Judge Result: DOC-33

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README names all three flags with row keys; documented invocation runs against scratch board and events.jsonl row carries task_id/actor/detail keys: README.md:551-568 '### Event flags (DOC-33)' names all three flags with their row keys: `--task-id ID` (sets `task_id`), `--actor NAME` (sets `actor`), `--detail @file` (sets `detail`, mutually exclusive with `--detail-text`); README.md:130-134 and 575-579 show the combined invocation. Verified by execution: `go build -o /tmp/doc33/boardctl ./cmd/boardctl` → BUILD_OK; `boardctl init --project demo --namespace demo` in /tmp/doc33; then the documented command `./boardctl -C /tmp/doc33 event --type audit --tick 44 --task-id BT-042 --actor foreman --detail @/tmp/detail.json` → exit=0, 'event id 1 appended'. Resulting .coding-hermes/board/events.jsonl row: {"id": 1, "timestamp": "2026-10-19T19:58:41...", "event_type": "audit", "task_id": "BT-042", "actor": "foreman", "detail": "{\"note\":\"structured\",\"n\":1}", "tick_number": 44} — all three keys present. Flag wiring confirmed at cmd/boardctl/main.go:1012-1016, 1063-1090. Test suite: `go test -count=1 ./...` exit_code=1, but all failures are pre-existing sandbox-environment failures unrelated to this doc change (e.g. internal/freshness TestAtHEADSilent: 'git commit --allow-empty -q -m c1: exit status 1 / gate: GitReins NOT RUN — .gitreins/config.yaml is absent'; same for internal/board TestDoctorFossilWarnRealExecParity and cmd/boardctl TestBT057StaleBinaryForeignRepoSilent); packages internal/render, internal/speccheck, internal/vulncheck, internal/workflowcheck, internal/fmtcheck all report 'ok'. The only changed file in the diff is cmd/boardctl/board-report.html (rendered_at timestamp), which cannot affect these tests.
README documents all three event flags with their row keys and the documented invocation was executed against a scratch board, producing an events.jsonl row carrying task_id, actor and detail.

Overall: FAIL ✗
