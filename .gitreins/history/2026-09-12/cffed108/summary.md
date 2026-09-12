# Verdict: BT-023

**Task:** create --id accepts any string — enforce fleet id format at write time
**Evaluated:** 2026-09-12T18:47:36.109801
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.039s
ok  	github.com/coding-hermes/boardctl/in
  ✓ secrets: [90m1:47PM[0m [32mINF[0m [1mscanned ~620829 bytes (620.83 KB) in 386ms[0m
[90m1:47PM[0m [32
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl create --id 'bad id!' exits 1 naming the offending value; --force bypass writes it; update path validated too; doctor warns (not errors) on pre-existing junk ids; validate stays silent-by-design for legacy; go build+vet+test green: End-to-end with built binary: `create --id 'bad id!'` -> EXIT=1 with stderr `boardctl: task id "bad id!" does not match the fleet id format ^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$ ...`; `create --id 'bad id!' --force` -> EXIT=0 'created task bad id!'; `update 'bad id!' --status in_progress` -> EXIT=1, with --force -> EXIT=0 'updated task bad id!'; `doctor` -> EXIT=0 with `[warn] tasks.jsonl line 2: task id "bad id!" ...` and `RESULT: OK (2 warning(s))` (warn not error, HasErrors untouched per validate.go:49); `validate` -> EXIT=0 `RESULT: OK (0 warning(s))` silent about junk id. Code: internal/board/read.go ValidateFleetTaskID/MatchesFleetTaskID, write.go Create/UpdateTask Force gate, doctor.go doctorTaskIDFormat, cmd/boardctl/main.go --force flags. `go build ./...` EXIT=0, `go vet ./...` EXIT=0, `go test -count=1 ./...` all ok (cmd/boardctl, internal/board, internal/render); TestCmdIDFormatEnforcement PASS, TestMatchesFleetTaskID PASS.
All BT-023 requirements verified: junk ids rejected on create/update with --force escape hatch, doctor warns (exit 0) on legacy junk ids, validate stays silent, and build/vet/test are green.

## Summary

Judge Result: BT-023

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.039s
ok  	github.com/coding-hermes/boardctl/in
  ✓ secrets: [90m1:47PM[0m [32mINF[0m [1mscanned ~620829 bytes (620.83 KB) in 386ms[0m
[90m1:47PM[0m [32

Stage tier2: PASS
  COMPLETE
  ✓ boardctl create --id 'bad id!' exits 1 naming the offending value; --force bypass writes it; update path validated too; doctor warns (not errors) on pre-existing junk ids; validate stays silent-by-design for legacy; go build+vet+test green: End-to-end with built binary: `create --id 'bad id!'` -> EXIT=1 with stderr `boardctl: task id "bad id!" does not match the fleet id format ^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$ ...`; `create --id 'bad id!' --force` -> EXIT=0 'created task bad id!'; `update 'bad id!' --status in_progress` -> EXIT=1, with --force -> EXIT=0 'updated task bad id!'; `doctor` -> EXIT=0 with `[warn] tasks.jsonl line 2: task id "bad id!" ...` and `RESULT: OK (2 warning(s))` (warn not error, HasErrors untouched per validate.go:49); `validate` -> EXIT=0 `RESULT: OK (0 warning(s))` silent about junk id. Code: internal/board/read.go ValidateFleetTaskID/MatchesFleetTaskID, write.go Create/UpdateTask Force gate, doctor.go doctorTaskIDFormat, cmd/boardctl/main.go --force flags. `go build ./...` EXIT=0, `go vet ./...` EXIT=0, `go test -count=1 ./...` all ok (cmd/boardctl, internal/board, internal/render); TestCmdIDFormatEnforcement PASS, TestMatchesFleetTaskID PASS.
All BT-023 requirements verified: junk ids rejected on create/update with --force escape hatch, doctor warns (exit 0) on legacy junk ids, validate stays silent, and build/vet/test are green.

Overall: PASS ✓
