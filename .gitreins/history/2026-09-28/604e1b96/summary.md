# Verdict: DF-BOARDCTL-13

**Task:** declared-lifecycle re-judge: update --priority/--title (merged 806b1df)
**Evaluated:** 2026-09-28T01:10:02.489783
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.799s
- ✓ **tier2**
  - COMPLETE
  ✓ update --priority and --title work through the sanctioned writer; --evidence-run-id present on update: CLI: cmd/boardctl/main.go:676 registers --priority/--evidence-run-id in reorderArgs valueFlags; :705-706 define the flags; :771-787 build board.UpdateSpec{Title, Priority, EvidenceRunID}; :790 calls b.UpdateTask (the sanctioned writer). Writer: internal/board/write.go:782-786 sets title, :882-890 normalizes priority through NormalizePriority + PriorityVocabulary (create's BT-007 gate, rejects junk with nothing written), :899-908 merges {run_id,ts} evidence into the row's detail via withEvidence. Usage lists both flags (main.go:715-718) and --normalize refuses them (main.go:741). Tests: `go test -count=1 ./internal/board/ -run 'DF13|Update|Priority|Title'` => exit 0, 'ok github.com/coding-hermes/boardctl/internal/board 0.065s' with PASS on TestUpdatePriorityRewritesRowBytePreserving, TestUpdatePriorityNormalizesLikeCreate, TestUpdateRejectsJunkPriority, TestUpdateEvidenceRunIDRecordsDetail, TestUpdateEvidenceRunIDMergesObjectDetailLosslessly. `go test -count=1 ./cmd/boardctl/ -run 'DF13|Update|Priority|Title|Normalize'` => exit 0, 'ok .../cmd/boardctl 0.033s' with PASS on TestCmdUpdatePriorityRewritesRow, TestCmdUpdateEvidenceRunIDWritesDetail, TestCmdUpdateHelpListsPriorityAndEvidenceFlags, TestCmdUpdateNormalizeRefusesPriorityAndEvidence. Full `go test -count=1 ./...` => all packages ok; `go build ./... && go vet ./...` => exit 0, no output.
update --priority/--title and --evidence-run-id are wired through the sanctioned b.UpdateTask writer with write-time vocabulary gating, and all board/CLI tests plus the full suite pass.

## Summary

Judge Result: DF-BOARDCTL-13

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.799s

Stage tier2: PASS
  COMPLETE
  ✓ update --priority and --title work through the sanctioned writer; --evidence-run-id present on update: CLI: cmd/boardctl/main.go:676 registers --priority/--evidence-run-id in reorderArgs valueFlags; :705-706 define the flags; :771-787 build board.UpdateSpec{Title, Priority, EvidenceRunID}; :790 calls b.UpdateTask (the sanctioned writer). Writer: internal/board/write.go:782-786 sets title, :882-890 normalizes priority through NormalizePriority + PriorityVocabulary (create's BT-007 gate, rejects junk with nothing written), :899-908 merges {run_id,ts} evidence into the row's detail via withEvidence. Usage lists both flags (main.go:715-718) and --normalize refuses them (main.go:741). Tests: `go test -count=1 ./internal/board/ -run 'DF13|Update|Priority|Title'` => exit 0, 'ok github.com/coding-hermes/boardctl/internal/board 0.065s' with PASS on TestUpdatePriorityRewritesRowBytePreserving, TestUpdatePriorityNormalizesLikeCreate, TestUpdateRejectsJunkPriority, TestUpdateEvidenceRunIDRecordsDetail, TestUpdateEvidenceRunIDMergesObjectDetailLosslessly. `go test -count=1 ./cmd/boardctl/ -run 'DF13|Update|Priority|Title|Normalize'` => exit 0, 'ok .../cmd/boardctl 0.033s' with PASS on TestCmdUpdatePriorityRewritesRow, TestCmdUpdateEvidenceRunIDWritesDetail, TestCmdUpdateHelpListsPriorityAndEvidenceFlags, TestCmdUpdateNormalizeRefusesPriorityAndEvidence. Full `go test -count=1 ./...` => all packages ok; `go build ./... && go vet ./...` => exit 0, no output.
update --priority/--title and --evidence-run-id are wired through the sanctioned b.UpdateTask writer with write-time vocabulary gating, and all board/CLI tests plus the full suite pass.

Overall: PASS ✓
