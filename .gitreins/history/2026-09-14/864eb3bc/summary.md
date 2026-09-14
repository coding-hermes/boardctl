# Verdict: BT-025

**Task:** Status vocabulary read-alias policy + normalize path
**Evaluated:** 2026-09-14T02:29:54.424728
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m9:29PM[0m [32mINF[0m [1mscanned ~799450 bytes (799.45 KB) in 306ms[0m
[90m9:29PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.501s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ validate on a real fleet board using todo/done/open statuses must exit 0 with per-row WARNINGS naming the canonical value, while genuinely unknown/ambiguous statuses (e.g. retired) still exit non-zero; 'boardctl update <id> --normalize' rewrites alias status/guard/ci values on the row to canonical form, exit 0, idempotent; write-side vocabulary enforcement unchanged (create/update still reject out-of-vocabulary values); go build ./... && go vet ./... && go test ./... -count=1 green; README documents the alias+normalize policy: Empirically verified end-to-end with a built binary on an init'd fleet board. (1) validate with todo/done/open rows: exit 0, per-row warnings e.g. 'tasks.jsonl line 2 (task FEAT-1): status "todo" is a read alias for "pending" — canonicalize with boardctl update FEAT-1 --normalize'; adding a 'retired' row produced '[error] ... status "retired" not in vocabulary' and EXIT=1 (validate.go:113-127 ResolveStatus branch). (2) 'update FEAT-2 --normalize' rewrote done->complete, exit 0; second run printed 'already canonical — nothing to rewrite (file untouched)', exit 0 (idempotent); guard/ci normalize confirmed (pass->PASS, green->GREEN) via write.go:651 NormalizeTask; diff showed only the target line changed. (3) Write-side unchanged: create --status done exit 1, update --status todo exit 1, update --status retired exit 1 (write.go:169-174, 515-521). (4) go build ./... => BUILD_OK; go vet ./... => VET_OK; go test ./... -count=1 => ok cmd/boardctl 0.468s, ok internal/board 0.131s, ok internal/render 0.014s (all green); targeted alias/normalize tests all PASS. (5) README.md lines 107-145 document the canonical vocabulary, alias table, warning behavior, and the --normalize fix path. LSP diagnostics: 0.


## Summary

Judge Result: BT-025

Stage tier1: PASS
    ✓ secrets: [90m9:29PM[0m [32mINF[0m [1mscanned ~799450 bytes (799.45 KB) in 306ms[0m
[90m9:29PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.501s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ validate on a real fleet board using todo/done/open statuses must exit 0 with per-row WARNINGS naming the canonical value, while genuinely unknown/ambiguous statuses (e.g. retired) still exit non-zero; 'boardctl update <id> --normalize' rewrites alias status/guard/ci values on the row to canonical form, exit 0, idempotent; write-side vocabulary enforcement unchanged (create/update still reject out-of-vocabulary values); go build ./... && go vet ./... && go test ./... -count=1 green; README documents the alias+normalize policy: Empirically verified end-to-end with a built binary on an init'd fleet board. (1) validate with todo/done/open rows: exit 0, per-row warnings e.g. 'tasks.jsonl line 2 (task FEAT-1): status "todo" is a read alias for "pending" — canonicalize with boardctl update FEAT-1 --normalize'; adding a 'retired' row produced '[error] ... status "retired" not in vocabulary' and EXIT=1 (validate.go:113-127 ResolveStatus branch). (2) 'update FEAT-2 --normalize' rewrote done->complete, exit 0; second run printed 'already canonical — nothing to rewrite (file untouched)', exit 0 (idempotent); guard/ci normalize confirmed (pass->PASS, green->GREEN) via write.go:651 NormalizeTask; diff showed only the target line changed. (3) Write-side unchanged: create --status done exit 1, update --status todo exit 1, update --status retired exit 1 (write.go:169-174, 515-521). (4) go build ./... => BUILD_OK; go vet ./... => VET_OK; go test ./... -count=1 => ok cmd/boardctl 0.468s, ok internal/board 0.131s, ok internal/render 0.014s (all green); targeted alias/normalize tests all PASS. (5) README.md lines 107-145 document the canonical vocabulary, alias table, warning behavior, and the --normalize fix path. LSP diagnostics: 0.


Overall: PASS ✓
