# Verdict: BT-055

**Task:** validate warns on non-array depends_on
**Evaluated:** 2026-09-24T11:59:40.875799
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	5.191s
- ✓ **tier2**
  - COMPLETE
  ✓ validate emits a warning naming file+line+task id for a depends_on that is not an array; real reference in wrong shape is surfaced; existing boards stay exit 0 (warning level); table tests RED-proven; build/vet/test green: validate.go:197-206 adds a warn-level finding 'tasks.jsonl line %d (task %s): depends_on is not an array (%s) — treated as no dependencies' (names file+line+task id+JSON kind via jsonKindName), plus a second warn at :203-205 'depends_on value %q looks like a reference but is not an array element — it was not cross-checked against task ids' surfacing a real reference in the wrong shape. Real board run `go run ./cmd/boardctl validate` emitted '[warn] tasks.jsonl line 39 (task BT-035): depends_on is not an array (string) — treated as no dependencies' and '...depends_on value "[]" looks like a reference but is not an array element — it was not cross-checked...' with RESULT: OK (43 warning(s)) and REAL_EXIT=0; cmd/boardctl/main.go:996 only returns error on rep.HasErrors() (error-level), so warn-level keeps exit 0. Table test TestBT055DependsOnShapeWarnings (bt055_depends_shape_test.go:71-135) covers 9 shape cases (array/absent/emptystr/ref/strarr/num/null/obj/bool) asserting count, file+line+task id, and kind; RED-proven: deleting the validate.go BT-055 block made it FAIL ('want 2 depends_on-shape warning(s), got 0' for 7 rows) and TestBT055MalformedRefSkipsCrossCheck FAIL, restored => PASS. `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -count=1` all ok (internal/board 5.288s, cmd/boardctl 6.694s), `go test ./internal/board/ -run BT055 -count=1 -v` => 3 PASS; LSP diagnostics 0.
BT-055 fully implemented and verified: warn-level depends_on shape finding names file+line+task id, surfaces wrong-shape references, keeps existing boards at exit 0, is RED-proven by table tests, and build/vet/test are green.

## Summary

Judge Result: BT-055

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	5.191s

Stage tier2: PASS
  COMPLETE
  ✓ validate emits a warning naming file+line+task id for a depends_on that is not an array; real reference in wrong shape is surfaced; existing boards stay exit 0 (warning level); table tests RED-proven; build/vet/test green: validate.go:197-206 adds a warn-level finding 'tasks.jsonl line %d (task %s): depends_on is not an array (%s) — treated as no dependencies' (names file+line+task id+JSON kind via jsonKindName), plus a second warn at :203-205 'depends_on value %q looks like a reference but is not an array element — it was not cross-checked against task ids' surfacing a real reference in the wrong shape. Real board run `go run ./cmd/boardctl validate` emitted '[warn] tasks.jsonl line 39 (task BT-035): depends_on is not an array (string) — treated as no dependencies' and '...depends_on value "[]" looks like a reference but is not an array element — it was not cross-checked...' with RESULT: OK (43 warning(s)) and REAL_EXIT=0; cmd/boardctl/main.go:996 only returns error on rep.HasErrors() (error-level), so warn-level keeps exit 0. Table test TestBT055DependsOnShapeWarnings (bt055_depends_shape_test.go:71-135) covers 9 shape cases (array/absent/emptystr/ref/strarr/num/null/obj/bool) asserting count, file+line+task id, and kind; RED-proven: deleting the validate.go BT-055 block made it FAIL ('want 2 depends_on-shape warning(s), got 0' for 7 rows) and TestBT055MalformedRefSkipsCrossCheck FAIL, restored => PASS. `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -count=1` all ok (internal/board 5.288s, cmd/boardctl 6.694s), `go test ./internal/board/ -run BT055 -count=1 -v` => 3 PASS; LSP diagnostics 0.
BT-055 fully implemented and verified: warn-level depends_on shape finding names file+line+task id, surfaces wrong-shape references, keeps existing boards at exit 0, is RED-proven by table tests, and build/vet/test are green.

Overall: PASS ✓
