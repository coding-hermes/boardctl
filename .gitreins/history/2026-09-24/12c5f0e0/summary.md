# Verdict: BT-059

**Task:** create writes the canonical schema, not the drifted measured union
**Evaluated:** 2026-09-24T12:04:09.001345
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	4.327s
- ✓ **tier2**
  - COMPLETE
  ✓ create emits only the canonical create schema on a drifted board (no inheritance of legacy extras like repo:null); census still reports legacy rows as drift; test proves new row on a drifted board is canon-clean; build/vet/test green: write.go:348 filters the mirrored key set through IsSanctionedTaskKey (BT-059 commit 8a5b04d touches only write.go + bt059_create_schema_test.go). TestCreateOnDriftedBoardWritesCanonicalSchema (bt059_create_schema_test.go:71) asserts no repo/reviewer key, exact canonical key list, and every key sanctioned. TestCensusStillReportsLegacyRowOnDriftedBoard (:180) asserts DriftRows()==1, repo(1)/reviewer(1), warning not error; validate.go:220-229 census path unchanged. RED-proof: reverting write.go to 034128f yields 3 FAILs ('new row inherited drift key "repo"'). go build ./... exit 0; go vet ./... exit 0; go test -count=1 ./... all 'ok' exit 0 (internal/board 1.454s).


## Summary

Judge Result: BT-059

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	4.327s

Stage tier2: PASS
  COMPLETE
  ✓ create emits only the canonical create schema on a drifted board (no inheritance of legacy extras like repo:null); census still reports legacy rows as drift; test proves new row on a drifted board is canon-clean; build/vet/test green: write.go:348 filters the mirrored key set through IsSanctionedTaskKey (BT-059 commit 8a5b04d touches only write.go + bt059_create_schema_test.go). TestCreateOnDriftedBoardWritesCanonicalSchema (bt059_create_schema_test.go:71) asserts no repo/reviewer key, exact canonical key list, and every key sanctioned. TestCensusStillReportsLegacyRowOnDriftedBoard (:180) asserts DriftRows()==1, repo(1)/reviewer(1), warning not error; validate.go:220-229 census path unchanged. RED-proof: reverting write.go to 034128f yields 3 FAILs ('new row inherited drift key "repo"'). go build ./... exit 0; go vet ./... exit 0; go test -count=1 ./... all 'ok' exit 0 (internal/board 1.454s).


Overall: PASS ✓
