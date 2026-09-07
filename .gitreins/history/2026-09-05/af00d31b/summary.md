# Verdict: BT-013

**Task:** doctor/validate drift errors should print the remediation command
**Evaluated:** 2026-09-05T07:24:00.923670
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m2:22AM[0m [32mINF[0m [1mscanned ~214256 bytes (214.26 KB) in 102ms[0m
[90m2:22AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.023s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE
  ✓ The boardctl CLI's doctor and validate commands, when they report header-counter drift (ticks_total < max events tick_number), must print an actionable remediation hint naming the fixing command (e.g. boardctl header --set-ticks-total=N) right after the error line. Add at least one regression test asserting the hint text appears for a drifted board. go build ./..., go vet ./..., go test ./... -count=1 -short all pass.: internal/board/doctor.go:129-130 appends "\nfix: boardctl header --set-ticks-total=%d" to the drift error in doctorHeaderVsEvents (the only place drift is reported; Doctor wraps Validate). RenderText (validate.go:332) prints "[error] <msg>\n", so the hint lands on the line immediately after the error line. Regression tests added: doctor_test.go TestDoctorHeaderCounterDrift & TestDoctorHeaderCounterDriftOnTopologyB assert "fix: boardctl header --set-ticks-total=3"; cmd/boardctl/main_test.go TestCmdDoctorTopologyBChecksHeaderVsEvents asserts "fix: boardctl header --set-ticks-total=1" in CLI output. go build ./... exit 0; go vet ./... exit 0; go test ./... -count=1 -short exit 0 (both packages ok).
The doctor drift error now prints the remediation hint (boardctl header --set-ticks-total=N) on the line right after the error, regression tests assert the hint, and build/vet/test all pass.

## Summary

Judge Result: BT-013

Stage tier1: PASS
    ✓ secrets: [90m2:22AM[0m [32mINF[0m [1mscanned ~214256 bytes (214.26 KB) in 102ms[0m
[90m2:22AM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.023s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE
  ✓ The boardctl CLI's doctor and validate commands, when they report header-counter drift (ticks_total < max events tick_number), must print an actionable remediation hint naming the fixing command (e.g. boardctl header --set-ticks-total=N) right after the error line. Add at least one regression test asserting the hint text appears for a drifted board. go build ./..., go vet ./..., go test ./... -count=1 -short all pass.: internal/board/doctor.go:129-130 appends "\nfix: boardctl header --set-ticks-total=%d" to the drift error in doctorHeaderVsEvents (the only place drift is reported; Doctor wraps Validate). RenderText (validate.go:332) prints "[error] <msg>\n", so the hint lands on the line immediately after the error line. Regression tests added: doctor_test.go TestDoctorHeaderCounterDrift & TestDoctorHeaderCounterDriftOnTopologyB assert "fix: boardctl header --set-ticks-total=3"; cmd/boardctl/main_test.go TestCmdDoctorTopologyBChecksHeaderVsEvents asserts "fix: boardctl header --set-ticks-total=1" in CLI output. go build ./... exit 0; go vet ./... exit 0; go test ./... -count=1 -short exit 0 (both packages ok).
The doctor drift error now prints the remediation hint (boardctl header --set-ticks-total=N) on the line right after the error, regression tests assert the hint, and build/vet/test all pass.

Overall: PASS ✓
