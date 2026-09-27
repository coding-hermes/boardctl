# Verdict: BT-067

**Task:** Sanction the multi-family evidence keys in the task-row key canon
**Evaluated:** 2026-09-26T08:57:04.209848
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.876s
- ✓ **tier2**
  - COMPLETE
  ✓ keys attributes,assignee,judge,worker,evidence are sanctioned in internal/board/keys.go with census-evidence comments; new tests in keys_test.go prove zero key-drift for rows carrying them and fail without the entries; go build ./... and go test ./... green; gofmt clean: internal/board/keys.go:75-103 adds a census-evidence comment block ('Evidence envelopes and ownership keys — sanctioned on the 2026-09-26 census (125 boards / 26,088 rows)') with per-key evidence (attributes 124 rows/2 families line 78-85; assignee 104/4 line 86-88; judge 66/~10 line 89-93; worker 14/4 line 94-96; evidence 70/6 line 97-98) and appends "attributes","assignee","judge","worker","evidence" to sanctionedExtras (lines 99-103). keys_test.go:30-47 (TestIsSanctionedTaskKey) asserts the 5 keys are sanctioned and near-misses attribute/evidences/judgement/workers/asignee stay drift; keys_test.go:287-323 (TestValidateEvidenceKeysAreNotDrift) builds a row carrying all 5 keys and asserts Rows==1, Clean==1, len(Keys)==0, summary contains '1/1' and '0 drift', and rendered text lacks 'outside the declared canon'. Mutation proof: after deleting the 5 sanctionedExtras lines, `go test -count=1 ./internal/board/` produced '--- FAIL: TestIsSanctionedTaskKey — keys_test.go:36: key "attributes" should be sanctioned (BT-067 census evidence)' (all 5 keys) and '--- FAIL: TestValidateEvidenceKeysAreNotDrift — keys_test.go:312: expected 1 clean row, got 0 ({Rows:1 Clean:0 Keys:map[assignee:1 attributes:1 evidence:1 judge:1 worker:1] Fixable:0})'; file restored (git diff empty, grep count of 5 keys == 5). `go build ./...` exit 0 with no output. `go test -count=1 ./...` all packages ok (cmd/boardctl ok, internal/board ok, internal/fmtcheck ok, internal/freshness ok, internal/render ok, internal/versioncheck ok, internal/vulncheck ok, internal/workflowcheck ok), no FAIL. `gofmt -l .` exit 0 with empty output (clean).
All five multi-family evidence keys are sanctioned in keys.go with census-evidence comments, keys_test.go proves zero drift and fails when the entries are removed, and build/test/gofmt are all green.

## Summary

Judge Result: BT-067

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	1.876s

Stage tier2: PASS
  COMPLETE
  ✓ keys attributes,assignee,judge,worker,evidence are sanctioned in internal/board/keys.go with census-evidence comments; new tests in keys_test.go prove zero key-drift for rows carrying them and fail without the entries; go build ./... and go test ./... green; gofmt clean: internal/board/keys.go:75-103 adds a census-evidence comment block ('Evidence envelopes and ownership keys — sanctioned on the 2026-09-26 census (125 boards / 26,088 rows)') with per-key evidence (attributes 124 rows/2 families line 78-85; assignee 104/4 line 86-88; judge 66/~10 line 89-93; worker 14/4 line 94-96; evidence 70/6 line 97-98) and appends "attributes","assignee","judge","worker","evidence" to sanctionedExtras (lines 99-103). keys_test.go:30-47 (TestIsSanctionedTaskKey) asserts the 5 keys are sanctioned and near-misses attribute/evidences/judgement/workers/asignee stay drift; keys_test.go:287-323 (TestValidateEvidenceKeysAreNotDrift) builds a row carrying all 5 keys and asserts Rows==1, Clean==1, len(Keys)==0, summary contains '1/1' and '0 drift', and rendered text lacks 'outside the declared canon'. Mutation proof: after deleting the 5 sanctionedExtras lines, `go test -count=1 ./internal/board/` produced '--- FAIL: TestIsSanctionedTaskKey — keys_test.go:36: key "attributes" should be sanctioned (BT-067 census evidence)' (all 5 keys) and '--- FAIL: TestValidateEvidenceKeysAreNotDrift — keys_test.go:312: expected 1 clean row, got 0 ({Rows:1 Clean:0 Keys:map[assignee:1 attributes:1 evidence:1 judge:1 worker:1] Fixable:0})'; file restored (git diff empty, grep count of 5 keys == 5). `go build ./...` exit 0 with no output. `go test -count=1 ./...` all packages ok (cmd/boardctl ok, internal/board ok, internal/fmtcheck ok, internal/freshness ok, internal/render ok, internal/versioncheck ok, internal/vulncheck ok, internal/workflowcheck ok), no FAIL. `gofmt -l .` exit 0 with empty output (clean).
All five multi-family evidence keys are sanctioned in keys.go with census-evidence comments, keys_test.go proves zero drift and fails when the entries are removed, and build/test/gofmt are all green.

Overall: PASS ✓
