# Verdict: BT-048

**Task:** validate WARNs on out-of-vocabulary priority on disk
**Evaluated:** 2026-09-23T05:39:42.778766
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.707s
- ✓ **tier2**
  - COMPLETE
  ✓ Table-driven test proves rows with priority values outside P0-P3 yield exactly one WARNING each naming row id and value; canonical rows yield none; exit code unchanged; build/vet/test green: internal/board/validate_priority_test.go: TestValidateWarnsOnOffVocabularyPriority is table-driven over {"3","P3"},{"P4",""},{"p2","P2"},{"urgent",""}; each case asserts countPriorityWarns(rep)==1 (exactly one WARNING), msg contains row id "PRIO-A" and the quoted raw value `priority "<raw>"`, and names vocabulary {P0,P1,P2,P3}. TestValidateCanonicalPrioritiesSilent seeds P0-P3 and asserts 0 priority warns. TestValidatePriorityWarningsKeepBoardOK asserts warnings do not error and RenderText contains "RESULT: OK" (exit code unchanged: cmd/boardctl/main.go:805 returns error only on rep.HasErrors(), and validate.go:49 HasErrors counts errors only). Implementation at internal/board/validate.go:158-165 adds exactly one warn per off-vocabulary row naming id and raw value. Evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` -> ok internal/board, ok cmd/boardctl, all packages ok; `go test -count=1 ./internal/board/ -run Priority -v` -> PASS TestValidateWarnsOnOffVocabularyPriority, PASS TestValidatePriorityWarningsKeepBoardOK, PASS TestValidateMissingPriorityUntouched.
Table-driven test proves off-vocabulary priorities yield exactly one WARNING naming row id and value, canonical rows yield none, exit code unchanged, and build/vet/test are all green.

## Summary

Judge Result: BT-048

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.707s

Stage tier2: PASS
  COMPLETE
  ✓ Table-driven test proves rows with priority values outside P0-P3 yield exactly one WARNING each naming row id and value; canonical rows yield none; exit code unchanged; build/vet/test green: internal/board/validate_priority_test.go: TestValidateWarnsOnOffVocabularyPriority is table-driven over {"3","P3"},{"P4",""},{"p2","P2"},{"urgent",""}; each case asserts countPriorityWarns(rep)==1 (exactly one WARNING), msg contains row id "PRIO-A" and the quoted raw value `priority "<raw>"`, and names vocabulary {P0,P1,P2,P3}. TestValidateCanonicalPrioritiesSilent seeds P0-P3 and asserts 0 priority warns. TestValidatePriorityWarningsKeepBoardOK asserts warnings do not error and RenderText contains "RESULT: OK" (exit code unchanged: cmd/boardctl/main.go:805 returns error only on rep.HasErrors(), and validate.go:49 HasErrors counts errors only). Implementation at internal/board/validate.go:158-165 adds exactly one warn per off-vocabulary row naming id and raw value. Evidence: `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` -> ok internal/board, ok cmd/boardctl, all packages ok; `go test -count=1 ./internal/board/ -run Priority -v` -> PASS TestValidateWarnsOnOffVocabularyPriority, PASS TestValidatePriorityWarningsKeepBoardOK, PASS TestValidateMissingPriorityUntouched.
Table-driven test proves off-vocabulary priorities yield exactly one WARNING naming row id and value, canonical rows yield none, exit code unchanged, and build/vet/test are all green.

Overall: PASS ✓
