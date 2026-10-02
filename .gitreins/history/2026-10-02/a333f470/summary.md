# Verdict: BT-072

**Task:** SPEC: new web UI for boardctl — scope + specification
**Evaluated:** 2026-10-02T03:52:57.888563
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	152.870s
- ✓ **tier2**
  - COMPLETE
  ✓ docs/web-ui-spec.md committed; every requirement testable with a named acceptance test; reuse-vs-new-vs-serve decided with a named reason; single-board vs fleet decided; read-only vs write posture decided; auth/bind posture decided; explicit NON-GOALS section; no requirement rests on an undecided item; spec cites the existing serve surface (GET /, POST /, GET /api/boards) and DF-BOARDCTL-2 dedupe behavior accurately: docs/web-ui-spec.md is committed/tracked (git ls-files confirms; commit ded9f67, merged c84fc1e). (1) Every requirement testable: 20 requirements R1-R20, each with an 'Acceptance:' line naming a TestBT072* test (grep: 20 Acceptance lines, 21 unique test names incl. R7's extra TestBT072APIXSSInert); §7 test index lists all. (2) D1 = EXTEND serve with 4 named reasons + rejected alternatives (lines 82-100). (3) D2 = FLEET with named reasons (lines 102-108). (4) D3 = READ-ONLY with named reason (lines 110-116). (5) D4 = loopback-only, no auth/TLS/cookies with named reasons (lines 118-124). (6) §8 NON-GOALS NG1-NG12 explicit (lines 407-436). (7) No requirement rests on an undecided item: §9 open questions (LAN/Tailscale, live refresh) are explicitly excluded by NG2/NG3; grep for TBD/TODO/undecided returns only the §9 heading. (8) Serve surface cited accurately: F1 matches cmd/boardctl/serve.go:380-382 (exactly GET /, POST /, GET /api/boards); R2's field list (name, slug, topology, task_total, done, open) matches apiBoard struct at serve.go:751-757. (9) DF-BOARDCTL-2 dedupe accurate: F6/R10 match serveBoardIdentity (serve.go:193-201, slug+topology), mergeByIdentity (serve.go:301-317, later replaces earlier), registerSession superseded-session deletion with temp trees (serve.go:338-372); F2 -C prepend matches serve.go:564. F4 loopback refusal (serve.go:93 loopbackHost, errUsage exit 2) and F5 -C preload (serve.go:98-107) + temp cleanup (serve.go:391) also verified. Doc-only deliverable — no test suite applies to the spec file itself.
docs/web-ui-spec.md is committed and satisfies all sub-criteria: R1-R20 each name an acceptance test, all six decisions (D1-D6) are made with named reasons, NON-GOALS NG1-NG12 are explicit, no requirement rests on an undecided item, and its cited serve surface and DF-BOARDCTL-2 dedupe behavior match the actual code.

## Summary

Judge Result: BT-072

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	152.870s

Stage tier2: PASS
  COMPLETE
  ✓ docs/web-ui-spec.md committed; every requirement testable with a named acceptance test; reuse-vs-new-vs-serve decided with a named reason; single-board vs fleet decided; read-only vs write posture decided; auth/bind posture decided; explicit NON-GOALS section; no requirement rests on an undecided item; spec cites the existing serve surface (GET /, POST /, GET /api/boards) and DF-BOARDCTL-2 dedupe behavior accurately: docs/web-ui-spec.md is committed/tracked (git ls-files confirms; commit ded9f67, merged c84fc1e). (1) Every requirement testable: 20 requirements R1-R20, each with an 'Acceptance:' line naming a TestBT072* test (grep: 20 Acceptance lines, 21 unique test names incl. R7's extra TestBT072APIXSSInert); §7 test index lists all. (2) D1 = EXTEND serve with 4 named reasons + rejected alternatives (lines 82-100). (3) D2 = FLEET with named reasons (lines 102-108). (4) D3 = READ-ONLY with named reason (lines 110-116). (5) D4 = loopback-only, no auth/TLS/cookies with named reasons (lines 118-124). (6) §8 NON-GOALS NG1-NG12 explicit (lines 407-436). (7) No requirement rests on an undecided item: §9 open questions (LAN/Tailscale, live refresh) are explicitly excluded by NG2/NG3; grep for TBD/TODO/undecided returns only the §9 heading. (8) Serve surface cited accurately: F1 matches cmd/boardctl/serve.go:380-382 (exactly GET /, POST /, GET /api/boards); R2's field list (name, slug, topology, task_total, done, open) matches apiBoard struct at serve.go:751-757. (9) DF-BOARDCTL-2 dedupe accurate: F6/R10 match serveBoardIdentity (serve.go:193-201, slug+topology), mergeByIdentity (serve.go:301-317, later replaces earlier), registerSession superseded-session deletion with temp trees (serve.go:338-372); F2 -C prepend matches serve.go:564. F4 loopback refusal (serve.go:93 loopbackHost, errUsage exit 2) and F5 -C preload (serve.go:98-107) + temp cleanup (serve.go:391) also verified. Doc-only deliverable — no test suite applies to the spec file itself.
docs/web-ui-spec.md is committed and satisfies all sub-criteria: R1-R20 each name an acceptance test, all six decisions (D1-D6) are made with named reasons, NON-GOALS NG1-NG12 are explicit, no requirement rests on an undecided item, and its cited serve surface and DF-BOARDCTL-2 dedupe behavior match the actual code.

Overall: PASS ✓
