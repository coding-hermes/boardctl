# Verdict: BT-RELEASE-0902

**Task:** Dedicated regression test for event --type refusal
**Evaluated:** 2026-09-25T17:00:50.980210
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.474s
- ✓ **tier2**
  - COMPLETE
  ✓ New test file cmd/boardctl/bt041_event_refusal_test.go proves: event with no --type exits 1 with 'event: --type is required' and writes NOTHING (hash-proven zero-writes); explicit --type arms append exactly one event; go build/vet/test -short green: File exists (183 lines, added in commit 7b1fc16). Arm 1 (lines 124-155): run(["-C",dir,"event","--detail-text","no type given"]) asserts exit==1, stderr contains "event: --type is required", and hash-proven zero-writes via bt041rBoardFileHashes() sha256-hashing every board file before/after (same file set, no deletion, byte-identical hashes) plus events.jsonl has 0 lines. Arm 2 (lines 157-168): --type audit exits 0 and events.jsonl grows to exactly 1 line with JSON-parsed event_type=="audit". Arm 3 (lines 170-182): --type task_created exits 0, exactly 2 lines, event_type=="task_created". Gate confirmed at cmd/boardctl/main.go:812-815. Mutation test: removing the gate made the test FAIL ("exit code = 0, want 1"; "refusal stderr = \"\""; "refused event modified board file events.jsonl (zero-write contract broken)"; "left 1 event row(s)"), proving genuine regression coverage; main.go restored, git status clean. Commands: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 -short ./...` => ok cmd/boardctl 4.036s and all internal packages ok; targeted `go test -count=1 -short -run TestEventRefusalRequiresType -v ./cmd/boardctl/` => --- PASS: TestEventRefusalRequiresType (0.00s), ok.
The new bt041_event_refusal_test.go proves the --type refusal (exit 1, exact message, hash-verified zero writes) and explicit-type appends, fails when the gate is removed, and build/vet/test -short are all green.

## Summary

Judge Result: BT-RELEASE-0902

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.474s

Stage tier2: PASS
  COMPLETE
  ✓ New test file cmd/boardctl/bt041_event_refusal_test.go proves: event with no --type exits 1 with 'event: --type is required' and writes NOTHING (hash-proven zero-writes); explicit --type arms append exactly one event; go build/vet/test -short green: File exists (183 lines, added in commit 7b1fc16). Arm 1 (lines 124-155): run(["-C",dir,"event","--detail-text","no type given"]) asserts exit==1, stderr contains "event: --type is required", and hash-proven zero-writes via bt041rBoardFileHashes() sha256-hashing every board file before/after (same file set, no deletion, byte-identical hashes) plus events.jsonl has 0 lines. Arm 2 (lines 157-168): --type audit exits 0 and events.jsonl grows to exactly 1 line with JSON-parsed event_type=="audit". Arm 3 (lines 170-182): --type task_created exits 0, exactly 2 lines, event_type=="task_created". Gate confirmed at cmd/boardctl/main.go:812-815. Mutation test: removing the gate made the test FAIL ("exit code = 0, want 1"; "refusal stderr = \"\""; "refused event modified board file events.jsonl (zero-write contract broken)"; "left 1 event row(s)"), proving genuine regression coverage; main.go restored, git status clean. Commands: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 -short ./...` => ok cmd/boardctl 4.036s and all internal packages ok; targeted `go test -count=1 -short -run TestEventRefusalRequiresType -v ./cmd/boardctl/` => --- PASS: TestEventRefusalRequiresType (0.00s), ok.
The new bt041_event_refusal_test.go proves the --type refusal (exit 1, exact message, hash-verified zero writes) and explicit-type appends, fails when the gate is removed, and build/vet/test -short are all green.

Overall: PASS ✓
