# Verdict: DOC-22

**Task:** Document VULNCHECK_STRICT_SCANNER env var
**Evaluated:** 2026-10-04T05:33:42.266580
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README.md and skills/boardctl-usage/SKILL.md document VULNCHECK_STRICT_SCANNER=1 env var that turns banner-less -version output into a hard gate failure instead of warn-and-proceed: README.md:808-820 documents the banner-less blind spot and that setting VULNCHECK_STRICT_SCANNER=1 (or true/yes, case-insensitive) turns warn-and-proceed into a hard gate refusal, including accepted values (1/true/yes enable; 0/false/no/off/unset keep default) and malformed-value behavior (warn + stay permissive). skills/boardctl-usage/SKILL.md:218 documents 'VULNCHECK_STRICT_SCANNER=1 turns banner-less -version into a hard fail'. Docs match implementation: internal/vulncheck/vulncheck.go:75 defines StrictScannerEnv="VULNCHECK_STRICT_SCANNER"; :250 strictScanner() accepts 1/true/yes and warns otherwise; :179/:445 refuse the gate under strict mode. Verified by running `go test -count=1 ./internal/vulncheck` -> exit 0, 'ok github.com/coding-hermes/boardctl/internal/vulncheck 5.742s'; subtests 'no_scanner_line_fails_under_StrictScannerEnv=1' and 'strict_env_parsing_accepts_1/true/yes_and_warns_otherwise' PASS.
Both README.md and skills/boardctl-usage/SKILL.md document VULNCHECK_STRICT_SCANNER=1 accurately matching the implementation, with passing tests confirming the hard-fail vs warn-and-proceed behavior.

## Summary

Judge Result: DOC-22

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README.md and skills/boardctl-usage/SKILL.md document VULNCHECK_STRICT_SCANNER=1 env var that turns banner-less -version output into a hard gate failure instead of warn-and-proceed: README.md:808-820 documents the banner-less blind spot and that setting VULNCHECK_STRICT_SCANNER=1 (or true/yes, case-insensitive) turns warn-and-proceed into a hard gate refusal, including accepted values (1/true/yes enable; 0/false/no/off/unset keep default) and malformed-value behavior (warn + stay permissive). skills/boardctl-usage/SKILL.md:218 documents 'VULNCHECK_STRICT_SCANNER=1 turns banner-less -version into a hard fail'. Docs match implementation: internal/vulncheck/vulncheck.go:75 defines StrictScannerEnv="VULNCHECK_STRICT_SCANNER"; :250 strictScanner() accepts 1/true/yes and warns otherwise; :179/:445 refuse the gate under strict mode. Verified by running `go test -count=1 ./internal/vulncheck` -> exit 0, 'ok github.com/coding-hermes/boardctl/internal/vulncheck 5.742s'; subtests 'no_scanner_line_fails_under_StrictScannerEnv=1' and 'strict_env_parsing_accepts_1/true/yes_and_warns_otherwise' PASS.
Both README.md and skills/boardctl-usage/SKILL.md document VULNCHECK_STRICT_SCANNER=1 accurately matching the implementation, with passing tests confirming the hard-fail vs warn-and-proceed behavior.

Overall: PASS ✓
