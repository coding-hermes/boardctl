# Verdict: DF-BOARDCTL-37

**Task:** umbrella --list --status filter
**Evaluated:** 2026-10-14T01:30:35.071222
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl umbrella --list --status <s> filters listed boards/tasks by status and is documented; tests cover it: Implementation: cmd/boardctl/umbrella.go:44-56 registers --status (reorderArgs valueFlags("C","status")), :~70-80 refuses --status without --list as errUsage, parseStatuses() splits CSV + NormalizeStatus + StatusVocabulary gate with dedupe; internal/board/read.go:184-222 adds TaskFilter.Statuses (OR match on NormalizeStatus(row status), composes with single Status, empty = any). Live E2E with a built binary against a git-inited umbrella+child board: unfiltered --list shows DF37-A1 pending + DF37-A2 complete; '--status pending' => only A1; '--status complete' => only A2; '--status done' (alias) => A2; '--status pending,complete' => both; '--status bogus' => exit 2 with 'not in vocabulary {pending,in_progress,review,blocked,complete,failed}'; '--status pending' without --list => exit 2; valid filter => exit 0. Tests: `go test ./... -count=1` => all 12 packages ok (cmd/boardctl 4.326s, internal/board 0.787s); targeted run shows PASS for TestDFBoardCTL37UmbrellaListSingleStatus, ...CommaSeparatedStatuses, ...AliasesNormalize, ...UnknownStatusUsageError, ...StatusIsListOnly and TestTaskFilterStatuses{SingleStatus,CommaListIsOR,MatchesStoredAliasRowsCanonically,UnknownMatchesNothing,ComposesWithStatus,EmptyIsAny}. Docs: README.md:167-171 (examples + alias/unknown notes), skills/boardctl-usage/SKILL.md:82-86, cmd/boardctl/main.go:118,122-123 usage string.
umbrella --list --status filtering (single, comma-list, aliases, usage errors) is implemented, documented in README/SKILL.md/usage, and covered by 11 passing tests with a clean full-suite run.

## Summary

Judge Result: DF-BOARDCTL-37

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ boardctl umbrella --list --status <s> filters listed boards/tasks by status and is documented; tests cover it: Implementation: cmd/boardctl/umbrella.go:44-56 registers --status (reorderArgs valueFlags("C","status")), :~70-80 refuses --status without --list as errUsage, parseStatuses() splits CSV + NormalizeStatus + StatusVocabulary gate with dedupe; internal/board/read.go:184-222 adds TaskFilter.Statuses (OR match on NormalizeStatus(row status), composes with single Status, empty = any). Live E2E with a built binary against a git-inited umbrella+child board: unfiltered --list shows DF37-A1 pending + DF37-A2 complete; '--status pending' => only A1; '--status complete' => only A2; '--status done' (alias) => A2; '--status pending,complete' => both; '--status bogus' => exit 2 with 'not in vocabulary {pending,in_progress,review,blocked,complete,failed}'; '--status pending' without --list => exit 2; valid filter => exit 0. Tests: `go test ./... -count=1` => all 12 packages ok (cmd/boardctl 4.326s, internal/board 0.787s); targeted run shows PASS for TestDFBoardCTL37UmbrellaListSingleStatus, ...CommaSeparatedStatuses, ...AliasesNormalize, ...UnknownStatusUsageError, ...StatusIsListOnly and TestTaskFilterStatuses{SingleStatus,CommaListIsOR,MatchesStoredAliasRowsCanonically,UnknownMatchesNothing,ComposesWithStatus,EmptyIsAny}. Docs: README.md:167-171 (examples + alias/unknown notes), skills/boardctl-usage/SKILL.md:82-86, cmd/boardctl/main.go:118,122-123 usage string.
umbrella --list --status filtering (single, comma-list, aliases, usage errors) is implemented, documented in README/SKILL.md/usage, and covered by 11 passing tests with a clean full-suite run.

Overall: PASS ✓
