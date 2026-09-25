# Verdict: DF-BOARDCTL-15

**Task:** Usage skill entry-points freshness
**Evaluated:** 2026-09-24T23:10:36.883958
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.954s
- ✗ **tier2**
  - INCOMPLETE
  ✓ skills/boardctl-usage/SKILL.md carries an automated freshness gate: a test (or make target wired into CI/guard) derives the CLI command list from the binary's --help output and asserts every top-level command appears in the skill's agent-facing entry-points/commands section: cmd/boardctl/df15_skill_closure_test.go:144 TestUsageSkillCoversEveryTopLevelCommand calls usageSkillCommands() which parses the real usageText constant (cmd/boardctl/main.go:43, the same text printed for --help at main.go:120/170) and asserts each command appears in the '## Entry points' or '## Proven commands' section. Wired into CI: .github/workflows/ci.yml:48 runs `go test -short -count=1 ./...` (test has no testing.Short guard, verified passing under -short) and Makefile `test: go test ./...`. Fresh run: `go test -count=1 ./cmd/boardctl/ -run 'TestUsageSkill|TestSkillCoverage'` => ok github.com/coding-hermes/boardctl/cmd/boardctl 0.006s.
  ✓ A headless test asserting skill-vs-CLI closure exists and passes at HEAD; the assertion provably fails when a command is added to the CLI but not the skill (verified by a temporary fixture run, not committed): At HEAD the gate passes: `go test -count=1 ./cmd/boardctl/ -run 'TestUsageSkill|TestSkillCoverage' -v` => PASS TestUsageSkillCoversEveryTopLevelCommand, PASS TestSkillCoverageParserProvesGatesBite, PASS TestUsageSkillVersionTracksGateResult. Negative fixture (temporary, restored): added `zz-newcmd` to usageText in main.go -> FAIL 'usage skill drift: top-level command "zz-newcmd" appears in the real --help output but in NEITHER the "Entry points" list NOR the "Proven commands" block'; also removing `sweep-status` from the skill -> FAIL. main.go and SKILL.md restored (git diff clean; only runtime .coding-hermes/board/events.jsonl and .gitreins logs modified).
  ✗ The gate covers the entry-points AND Proven commands sections; version and date in the skill header agree with the gate result: Section coverage is met: skillEntrySections requires both '## Entry points' and '## Proven commands' (df15_skill_closure_test.go:104-107, 149-155); renaming the '## Proven commands' heading -> FAIL 'usage skill lost its "Proven commands" section'. Version is asserted: TestUsageSkillVersionTracksGateResult (line 238) requires frontmatter `version: X.Y.Z` >= 1.7.0 and SKILL.md:7 has `version: 1.7.0`. HOWEVER the date half is missing: grep for date/Date/Parse/time in df15_skill_closure_test.go returns no date assertion, and the skill frontmatter has no date field (date '2026-09-24' appears only in body text at SKILL.md:19). The criterion requires the date in the skill header to agree with the gate result, which no test verifies.
The freshness gate exists, is CI-wired, passes at HEAD, and provably fails on CLI/skill drift, but criterion 3 is unmet because the gate asserts only the version and never the date in the skill header.

## Summary

Judge Result: DF-BOARDCTL-15

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	2.954s

Stage tier2: FAIL
  INCOMPLETE
  ✓ skills/boardctl-usage/SKILL.md carries an automated freshness gate: a test (or make target wired into CI/guard) derives the CLI command list from the binary's --help output and asserts every top-level command appears in the skill's agent-facing entry-points/commands section: cmd/boardctl/df15_skill_closure_test.go:144 TestUsageSkillCoversEveryTopLevelCommand calls usageSkillCommands() which parses the real usageText constant (cmd/boardctl/main.go:43, the same text printed for --help at main.go:120/170) and asserts each command appears in the '## Entry points' or '## Proven commands' section. Wired into CI: .github/workflows/ci.yml:48 runs `go test -short -count=1 ./...` (test has no testing.Short guard, verified passing under -short) and Makefile `test: go test ./...`. Fresh run: `go test -count=1 ./cmd/boardctl/ -run 'TestUsageSkill|TestSkillCoverage'` => ok github.com/coding-hermes/boardctl/cmd/boardctl 0.006s.
  ✓ A headless test asserting skill-vs-CLI closure exists and passes at HEAD; the assertion provably fails when a command is added to the CLI but not the skill (verified by a temporary fixture run, not committed): At HEAD the gate passes: `go test -count=1 ./cmd/boardctl/ -run 'TestUsageSkill|TestSkillCoverage' -v` => PASS TestUsageSkillCoversEveryTopLevelCommand, PASS TestSkillCoverageParserProvesGatesBite, PASS TestUsageSkillVersionTracksGateResult. Negative fixture (temporary, restored): added `zz-newcmd` to usageText in main.go -> FAIL 'usage skill drift: top-level command "zz-newcmd" appears in the real --help output but in NEITHER the "Entry points" list NOR the "Proven commands" block'; also removing `sweep-status` from the skill -> FAIL. main.go and SKILL.md restored (git diff clean; only runtime .coding-hermes/board/events.jsonl and .gitreins logs modified).
  ✗ The gate covers the entry-points AND Proven commands sections; version and date in the skill header agree with the gate result: Section coverage is met: skillEntrySections requires both '## Entry points' and '## Proven commands' (df15_skill_closure_test.go:104-107, 149-155); renaming the '## Proven commands' heading -> FAIL 'usage skill lost its "Proven commands" section'. Version is asserted: TestUsageSkillVersionTracksGateResult (line 238) requires frontmatter `version: X.Y.Z` >= 1.7.0 and SKILL.md:7 has `version: 1.7.0`. HOWEVER the date half is missing: grep for date/Date/Parse/time in df15_skill_closure_test.go returns no date assertion, and the skill frontmatter has no date field (date '2026-09-24' appears only in body text at SKILL.md:19). The criterion requires the date in the skill header to agree with the gate result, which no test verifies.
The freshness gate exists, is CI-wired, passes at HEAD, and provably fails on CLI/skill drift, but criterion 3 is unmet because the gate asserts only the version and never the date in the skill header.

Overall: FAIL ✗
