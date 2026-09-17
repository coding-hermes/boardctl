# Verdict: QA-BOARDCTL-001

**Task:** Replace vacuous boardctl corruption coverage with isolated JSONL CLI tests; track external harness remainder
**Evaluated:** 2026-09-17T08:01:32.990202
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m3:00AM[0m [32mINF[0m [1mscanned ~911150 bytes (911.15 KB) in 320ms[0m
[90m3:00AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.508s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ Add repeatable CLI regression coverage in cmd/boardctl/qa_corruption_test.go using only disposable temporary boards, covering malformed tasks/events/header/fixtures JSONL with nonzero exit and target-file diagnostic, a clean baseline, and byte-preservation on validation. A decoy dagger.db remains unchanged and is not used as project state. Document the invocation and explicitly leave the external bunker-qa server flag and allocation-capacity issues unresolved; do not claim the external QA harness is fixed. go build, go vet and fresh short tests pass.: cmd/boardctl/qa_corruption_test.go (402 lines) uses only t.TempDir disposable boards (qaSeedValidBoard:61-95). TestQACorruption (216-278) table covers tasks.jsonl/events.jsonl/board.jsonl: corrupts exactly one named file, asserts validate exit==1, diagnostic names target file, RESULT: FAIL, no dagger.db mention, zero writes via sha256 snapshot hashing (qaBoardFileHashes:148, qaChangedFiles:170), restore returns clean. fixtures.jsonl covered by TestQACorruptionFixturesDetectedViaReader (291) via real `show` reader path (main.go:400 searches tasks.jsonl+fixtures.jsonl), exit 1 + target-file diagnostic. Clean baseline TestQACorruptionCleanBaseline (350) exit 0/RESULT: OK. Byte-preservation proven load-bearing by TestQACorruptionNoWriteAssertionDetectsWrites (379). Decoy planted at repo root dir/dagger.db (line 88), NOT in board dir .coding-hermes/board/ — asserted byte-identical (qaDecoyUnchanged:186) and never mentioned. docs/dogfood/qa-corruption.md documents invocation (lines 39-43) and explicitly leaves external items UNRESOLVED (lines 87-99: harness *.db glob, bunker-qa.sh --server, bunker capacity — all 'Unfixed', 'not claimed fixed'). Fresh command output: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1 -short` exit 0 (ok cmd/boardctl 0.466s, ok internal/board, ok internal/fmtcheck, ok internal/render, ok internal/versioncheck); `go test ./cmd/boardctl -run TestQACorruption -count=1 -v` → 4 tests/6 subtests all PASS; gofmt -l clean; make fmt-check exit 0.
All deliverables verified: isolated t.TempDir CLI corruption tests for tasks/events/header/fixtures JSONL with nonzero exit + target-file diagnostics, clean baseline, byte-preservation, unchanged dagger.db decoy, documented invocation and explicitly unresolved external harness items; go build, go vet and fresh short tests all pass.

## Summary

Judge Result: QA-BOARDCTL-001

Stage tier1: PASS
    ✓ secrets: [90m3:00AM[0m [32mINF[0m [1mscanned ~911150 bytes (911.15 KB) in 320ms[0m
[90m3:00AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.508s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ Add repeatable CLI regression coverage in cmd/boardctl/qa_corruption_test.go using only disposable temporary boards, covering malformed tasks/events/header/fixtures JSONL with nonzero exit and target-file diagnostic, a clean baseline, and byte-preservation on validation. A decoy dagger.db remains unchanged and is not used as project state. Document the invocation and explicitly leave the external bunker-qa server flag and allocation-capacity issues unresolved; do not claim the external QA harness is fixed. go build, go vet and fresh short tests pass.: cmd/boardctl/qa_corruption_test.go (402 lines) uses only t.TempDir disposable boards (qaSeedValidBoard:61-95). TestQACorruption (216-278) table covers tasks.jsonl/events.jsonl/board.jsonl: corrupts exactly one named file, asserts validate exit==1, diagnostic names target file, RESULT: FAIL, no dagger.db mention, zero writes via sha256 snapshot hashing (qaBoardFileHashes:148, qaChangedFiles:170), restore returns clean. fixtures.jsonl covered by TestQACorruptionFixturesDetectedViaReader (291) via real `show` reader path (main.go:400 searches tasks.jsonl+fixtures.jsonl), exit 1 + target-file diagnostic. Clean baseline TestQACorruptionCleanBaseline (350) exit 0/RESULT: OK. Byte-preservation proven load-bearing by TestQACorruptionNoWriteAssertionDetectsWrites (379). Decoy planted at repo root dir/dagger.db (line 88), NOT in board dir .coding-hermes/board/ — asserted byte-identical (qaDecoyUnchanged:186) and never mentioned. docs/dogfood/qa-corruption.md documents invocation (lines 39-43) and explicitly leaves external items UNRESOLVED (lines 87-99: harness *.db glob, bunker-qa.sh --server, bunker capacity — all 'Unfixed', 'not claimed fixed'). Fresh command output: `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1 -short` exit 0 (ok cmd/boardctl 0.466s, ok internal/board, ok internal/fmtcheck, ok internal/render, ok internal/versioncheck); `go test ./cmd/boardctl -run TestQACorruption -count=1 -v` → 4 tests/6 subtests all PASS; gofmt -l clean; make fmt-check exit 0.
All deliverables verified: isolated t.TempDir CLI corruption tests for tasks/events/header/fixtures JSONL with nonzero exit + target-file diagnostics, clean baseline, byte-preservation, unchanged dagger.db decoy, documented invocation and explicitly unresolved external harness items; go build, go vet and fresh short tests all pass.

Overall: PASS ✓
