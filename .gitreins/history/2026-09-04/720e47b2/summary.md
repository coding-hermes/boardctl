# Verdict: BT-010

**Task:** Topology B boards are read-only in practice — implement writes
**Evaluated:** 2026-09-04T23:12:18.090869
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m6:10PM[0m [32mINF[0m [1mscanned ~203108 bytes (203.11 KB) in 104ms[0m
[90m6:10PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.018s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ create/update/event/header writes must work on topology-B boards (header = line 1 of tasks.jsonl): (1) SetHeader rewrites line 1 of tasks.jsonl on topology B (board.jsonl on A), untouched lines round-trip byte-identical; (2) Create mirrors task-row schema skipping the line-1 header and append-only task rows work; (3) UpdateTask works with header line preserved; (4) AppendEvent works (events.jsonl has no header); (5) header subcommand reads/writes line-1 header on topology B; (6) validate/doctor check header counters on topology B by reading line 1 of tasks.jsonl (warn upgraded to real check); (7) README updated: topology B no longer read-only, migration still documented; (8) go build ./..., go vet ./..., go test ./... -count=1 -short all pass; regression tests cover each path: (1) write.go SetHeader uses headerPathFor() (tasksPath on B), rewrites only headerIdx line, asserts untouched lines byte-identical before atomicRewrite; TestSetHeaderOnTopologyB (board_test.go:560) verifies task row byte-identical. (2) Create uses lastTaskRow() which skips header via skipTaskLine, appends via O_APPEND; TestCreateOnTopologyBAppendsAfterHeader (board_test.go:441) verifies new row mirrors last TASK row keys not header keys. (3) UpdateTask (write.go:379) skips header when finding target, preserves line 1; TestUpdateTaskOnTopologyB (board_test.go:504). (4) AppendEvent (write.go:546) no topology guard; TestAppendEventOnTopologyB (board_test.go:611). (5) cmdHeader (main.go:608) removed topology special case; TestCmdTopologyBFullWorkflow AC4a/AC4b (main_test.go:207). (6) validate.go Validate() runs validateHeader on both topologies (removed 'not checked' warn), doctor.go doctorHeaderVsEvents uses HeaderRow; TestValidateTopologyBChecksHeaderCounters (validate_test.go:155), TestDoctorHeaderCounterDriftOnTopologyB (doctor_test.go:205). (7) README.md:92 'Both topologies are FULLY WRITABLE (BT-010)', migration documented as optional at README.md:69-73. (8) go build ./... exit 0, go vet ./... exit 0, go test ./... -count=1 -short exit 0 (ok cmd/boardctl, ok internal/board); regression tests cover each path incl. TestCmdTopologyBFullWorkflow, TestCmdValidateTopologyBFlagsNegativeCounter, TestCmdDoctorTopologyBChecksHeaderVsEvents, TestCmdInitOnTopologyBStillRefuses.
All topology-B write paths (SetHeader, Create, UpdateTask, AppendEvent, header subcommand, validate/doctor header checks) are implemented with byte-identical untouched-line guarantees, README updated, and build/vet/test all pass with regression tests covering each path.

## Summary

Judge Result: BT-010

Stage tier1: PASS
    ✓ secrets: [90m6:10PM[0m [32mINF[0m [1mscanned ~203108 bytes (203.11 KB) in 104ms[0m
[90m6:10PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.018s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ create/update/event/header writes must work on topology-B boards (header = line 1 of tasks.jsonl): (1) SetHeader rewrites line 1 of tasks.jsonl on topology B (board.jsonl on A), untouched lines round-trip byte-identical; (2) Create mirrors task-row schema skipping the line-1 header and append-only task rows work; (3) UpdateTask works with header line preserved; (4) AppendEvent works (events.jsonl has no header); (5) header subcommand reads/writes line-1 header on topology B; (6) validate/doctor check header counters on topology B by reading line 1 of tasks.jsonl (warn upgraded to real check); (7) README updated: topology B no longer read-only, migration still documented; (8) go build ./..., go vet ./..., go test ./... -count=1 -short all pass; regression tests cover each path: (1) write.go SetHeader uses headerPathFor() (tasksPath on B), rewrites only headerIdx line, asserts untouched lines byte-identical before atomicRewrite; TestSetHeaderOnTopologyB (board_test.go:560) verifies task row byte-identical. (2) Create uses lastTaskRow() which skips header via skipTaskLine, appends via O_APPEND; TestCreateOnTopologyBAppendsAfterHeader (board_test.go:441) verifies new row mirrors last TASK row keys not header keys. (3) UpdateTask (write.go:379) skips header when finding target, preserves line 1; TestUpdateTaskOnTopologyB (board_test.go:504). (4) AppendEvent (write.go:546) no topology guard; TestAppendEventOnTopologyB (board_test.go:611). (5) cmdHeader (main.go:608) removed topology special case; TestCmdTopologyBFullWorkflow AC4a/AC4b (main_test.go:207). (6) validate.go Validate() runs validateHeader on both topologies (removed 'not checked' warn), doctor.go doctorHeaderVsEvents uses HeaderRow; TestValidateTopologyBChecksHeaderCounters (validate_test.go:155), TestDoctorHeaderCounterDriftOnTopologyB (doctor_test.go:205). (7) README.md:92 'Both topologies are FULLY WRITABLE (BT-010)', migration documented as optional at README.md:69-73. (8) go build ./... exit 0, go vet ./... exit 0, go test ./... -count=1 -short exit 0 (ok cmd/boardctl, ok internal/board); regression tests cover each path incl. TestCmdTopologyBFullWorkflow, TestCmdValidateTopologyBFlagsNegativeCounter, TestCmdDoctorTopologyBChecksHeaderVsEvents, TestCmdInitOnTopologyBStillRefuses.
All topology-B write paths (SetHeader, Create, UpdateTask, AppendEvent, header subcommand, validate/doctor header checks) are implemented with byte-identical untouched-line guarantees, README updated, and build/vet/test all pass with regression tests covering each path.

Overall: PASS ✓
