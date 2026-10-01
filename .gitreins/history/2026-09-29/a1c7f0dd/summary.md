# Verdict: REVIEW-BOARDCTL-011

**Task:** README: document boardctl validate --repair after the --skip-bad-lines bullet — what it writes (tasks.rewritten.jsonl), that tasks.jsonl is never modified, and the manual-review-then-swap procedure. Docs-only.
**Evaluated:** 2026-09-29T11:05:22.905008
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.256s
- ✓ **tier2**
  - COMPLETE
  ✓ worker commit lands; build+tests green: Worker commit 36cec9b 'merge: REVIEW-BOARDCTL-011 README documents validate --repair' landed (README.md +30 lines). `go build ./...` exit_code=0. `go test -count=1 ./...` exit_code=0 with 11 packages 'ok' and no FAIL lines. README.md:204-231 documents --repair writing tasks.rewritten.jsonl, that tasks.jsonl is NEVER modified, and the manual review/diff/validate/swap procedure.
Worker commit landed and build+tests are green, with README documenting validate --repair as required.

## Summary

Judge Result: REVIEW-BOARDCTL-011

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	3.256s

Stage tier2: PASS
  COMPLETE
  ✓ worker commit lands; build+tests green: Worker commit 36cec9b 'merge: REVIEW-BOARDCTL-011 README documents validate --repair' landed (README.md +30 lines). `go build ./...` exit_code=0. `go test -count=1 ./...` exit_code=0 with 11 packages 'ok' and no FAIL lines. README.md:204-231 documents --repair writing tasks.rewritten.jsonl, that tasks.jsonl is NEVER modified, and the manual review/diff/validate/swap procedure.
Worker commit landed and build+tests are green, with README documenting validate --repair as required.

Overall: PASS ✓
