# Verdict: BT-84

**Task:** Repo hygiene: deterministic report and non-churning guard logs
**Evaluated:** 2026-10-31T12:07:34.761380
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Iteration cap (250) reached (250.1 used). Increase max_iterations or split criteria.

## Summary

Judge Result: BT-84

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Iteration cap (250) reached (250.1 used). Increase max_iterations or split criteria.

Overall: FAIL ✗
