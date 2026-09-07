# Verdict: BT-007

**Task:** Write vocabulary unenforced on guard/ci/priority/depends-on; validate+doctor silent on junk rows
**Evaluated:** 2026-09-04T21:30:59.094878
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m4:29PM[0m [32mINF[0m [1mscanned ~176079 bytes (176.08 KB) in 122ms[0m
[90m4:29PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.111s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ create/update reject or normalize out-of-vocabulary guard_result/ci_result/priority values; validate warns on depends_on ids that don't exist and header counters < 0; validate flags out-of-vocab values already on boards: (a) write.go: Create normalizes priority via NormalizePriority (bare digits 0-3 -> P0-P3) and rejects out-of-vocab values (write.go ~155-165); Update rejects guard_result not in {PASS,FAIL,SKIP} and ci_result not in {GREEN,RED,SKIP} (write.go ~410-440). (b) validate.go ~145-150 warns on depends_on ids referencing no task row. (c) validate.go ~215-220 flags negative header counters. (d) validate.go ~120-135 warns on out-of-vocab guard_result/ci_result already on boards; Doctor() calls Validate() so surfaces same findings (doctor.go:22). Tests: `go test -count=1 ./...` -> ok cmd/boardctl and internal/board (exit 0); targeted BT-007 tests (TestUpdateGuardRejectsOutOfVocab, TestUpdateCIRejectsOutOfVocab, TestCreatePriorityNormalizesBareDigits, TestCreatePriorityRejectsGarbage, TestCreateDependsOnGhostRejected, TestValidateFlagsFreeFormGuardAndCI, TestValidateWarnsOnDanglingDependsOn, TestValidateErrorsOnNegativeHeaderCounters, TestDoctorSurfacesVocabularyAndDepFindings, TestCmdValidateFlagsJunkRow) all pass. go vet/build clean, no LSP diagnostics.
Write path rejects/normalizes out-of-vocab guard/ci/priority values, validate warns on dangling depends_on and flags out-of-vocab values already on boards, and negative header counters are flagged; all covered by passing tests.

## Summary

Judge Result: BT-007

Stage tier1: PASS
    ✓ secrets: [90m4:29PM[0m [32mINF[0m [1mscanned ~176079 bytes (176.08 KB) in 122ms[0m
[90m4:29PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.111s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ create/update reject or normalize out-of-vocabulary guard_result/ci_result/priority values; validate warns on depends_on ids that don't exist and header counters < 0; validate flags out-of-vocab values already on boards: (a) write.go: Create normalizes priority via NormalizePriority (bare digits 0-3 -> P0-P3) and rejects out-of-vocab values (write.go ~155-165); Update rejects guard_result not in {PASS,FAIL,SKIP} and ci_result not in {GREEN,RED,SKIP} (write.go ~410-440). (b) validate.go ~145-150 warns on depends_on ids referencing no task row. (c) validate.go ~215-220 flags negative header counters. (d) validate.go ~120-135 warns on out-of-vocab guard_result/ci_result already on boards; Doctor() calls Validate() so surfaces same findings (doctor.go:22). Tests: `go test -count=1 ./...` -> ok cmd/boardctl and internal/board (exit 0); targeted BT-007 tests (TestUpdateGuardRejectsOutOfVocab, TestUpdateCIRejectsOutOfVocab, TestCreatePriorityNormalizesBareDigits, TestCreatePriorityRejectsGarbage, TestCreateDependsOnGhostRejected, TestValidateFlagsFreeFormGuardAndCI, TestValidateWarnsOnDanglingDependsOn, TestValidateErrorsOnNegativeHeaderCounters, TestDoctorSurfacesVocabularyAndDepFindings, TestCmdValidateFlagsJunkRow) all pass. go vet/build clean, no LSP diagnostics.
Write path rejects/normalizes out-of-vocab guard/ci/priority values, validate warns on dangling depends_on and flags out-of-vocab values already on boards, and negative header counters are flagged; all covered by passing tests.

Overall: PASS ✓
