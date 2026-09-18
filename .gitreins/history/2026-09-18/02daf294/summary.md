# Verdict: BT-033

**Task:** toolchain vuln + no gate: go.mod go 1.26 (vulnerable go1.26.5 toolchain) with 3 reachable stdlib vulns and no govulncheck gate
**Evaluated:** 2026-09-18T02:42:37.230171
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m9:41PM[0m [32mINF[0m [1mscanned ~951960 bytes (951.96 KB) in 345ms[0m
[90m9:41PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.531s
ok  	github.com/coding-hermes/boardctl/in
- ✓ **tier2**
  - COMPLETE

(auto-parsed from non-JSON response) All criteria verified with live command output. Working tree restored to HEAD (no probe residue), no LSP diagnostics.

{"verdict":"COMPLETE","items":[{"criterion":"AC1 govulncheck ./... exits 0 'No vulnerabilities found.' at HEAD, with the BEFORE state (exit 3: GO-2026-6089 net/http, GO-2026-6090 cr

## Summary

Judge Result: BT-033

Stage tier1: PASS
    ✓ secrets: [90m9:41PM[0m [32mINF[0m [1mscanned ~951960 bytes (951.96 KB) in 345ms[0m
[90m9:41PM[0m [32
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.531s
ok  	github.com/coding-hermes/boardctl/in

Stage tier2: PASS
  COMPLETE

(auto-parsed from non-JSON response) All criteria verified with live command output. Working tree restored to HEAD (no probe residue), no LSP diagnostics.

{"verdict":"COMPLETE","items":[{"criterion":"AC1 govulncheck ./... exits 0 'No vulnerabilities found.' at HEAD, with the BEFORE state (exit 3: GO-2026-6089 net/http, GO-2026-6090 cr

Overall: PASS ✓
