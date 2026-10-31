# Verdict: BT-84

**Task:** Repo hygiene: deterministic report and non-churning guard logs
**Evaluated:** 2026-10-31T11:57:48.586177
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Iteration cap (200) reached (200.1 used). Increase max_iterations or split criteria.

## Summary

Judge Result: BT-84

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Iteration cap (200) reached (200.1 used). Increase max_iterations or split criteria.

Overall: FAIL ✗
