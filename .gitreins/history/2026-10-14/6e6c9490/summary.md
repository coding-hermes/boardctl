# Verdict: DF-BOARDCTL-36

**Task:** warpfs board canon drift
**Evaluated:** 2026-10-14T01:32:15.251121
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ warpfs board rows normalized into boardctl canon where safe (repair path or documented exclusion), evidence from live board: Repair path: internal/board/normalizeboard.go (564 lines) NormalizeBoard(apply) classifies every row — key aliases via BT-056 CanonicalKeyFor/RenameKey (keys.go:181-188), string depends_on -> array when unambiguous, alias statuses via BT-025 ResolveStatus; missing created_at reported only, never fabricated; dry-run default stages tasks.normalized.jsonl, --apply does ONE guarded atomic rewrite (withWriteLock+atomicRewrite) with untouched lines asserted byte-identical. CLI cmd/boardctl/main.go `normalize-board [--apply] [--json] [--skip-bad-lines]`; internal/describe/describe.go command map carries the verb. LIVE BOARD EVIDENCE (built binary run against a copy of /home/kara/warpfs/.coding-hermes/board, 381 rows): before apply validate reported 'key uniformity: 248/381 rows carry only canon keys; drift 133 row(s): _pass_criteria(31), acceptance(44), pass_criteria(10), added_by(22), project(22), cycle(19)...' RESULT FAIL (1 error, 626 warnings); normalize-board dry-run printed '105 mechanical repair(s) across 104 row(s); 38 row(s) refused (left untouched)' + missing_created_at note on 161 rows, exit 0, no board write; after --apply validate reported 'key uniformity: 278/381 rows carry only canon keys; drift 103 row(s)' with warnings 626 -> 527 — all alias keys repaired, 30 rows moved into canon. Documented exclusion: remaining drift keys (added_by, project, cycle, severity, verdict...) are absent from TaskKeyAliases (keys.go:181-188, 7 entries) and keys.go:164-178 documents the evidence-based exclusion discipline; README.md:693-737, docs/operations.md §1b, skills/boardctl-usage/SKILL.md v1.17.0 document the repair path, refusals, and never-fabricate rule. Tests: go build ./... exit 0; go test -count=1 ./... => all 12 packages ok; DF36-specific 13/13 PASS (8 engine + 5 CLI); go vet clean; LSP diagnostics empty.
normalize-board provides a verified repair path that normalized the live warpfs board's alias keys/string depends_on/alias statuses into canon (drift 133->103 rows, warnings 626->527) with documented exclusions for non-alias keys and never-fabricated created_at, backed by 13 passing tests.

## Summary

Judge Result: DF-BOARDCTL-36

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ warpfs board rows normalized into boardctl canon where safe (repair path or documented exclusion), evidence from live board: Repair path: internal/board/normalizeboard.go (564 lines) NormalizeBoard(apply) classifies every row — key aliases via BT-056 CanonicalKeyFor/RenameKey (keys.go:181-188), string depends_on -> array when unambiguous, alias statuses via BT-025 ResolveStatus; missing created_at reported only, never fabricated; dry-run default stages tasks.normalized.jsonl, --apply does ONE guarded atomic rewrite (withWriteLock+atomicRewrite) with untouched lines asserted byte-identical. CLI cmd/boardctl/main.go `normalize-board [--apply] [--json] [--skip-bad-lines]`; internal/describe/describe.go command map carries the verb. LIVE BOARD EVIDENCE (built binary run against a copy of /home/kara/warpfs/.coding-hermes/board, 381 rows): before apply validate reported 'key uniformity: 248/381 rows carry only canon keys; drift 133 row(s): _pass_criteria(31), acceptance(44), pass_criteria(10), added_by(22), project(22), cycle(19)...' RESULT FAIL (1 error, 626 warnings); normalize-board dry-run printed '105 mechanical repair(s) across 104 row(s); 38 row(s) refused (left untouched)' + missing_created_at note on 161 rows, exit 0, no board write; after --apply validate reported 'key uniformity: 278/381 rows carry only canon keys; drift 103 row(s)' with warnings 626 -> 527 — all alias keys repaired, 30 rows moved into canon. Documented exclusion: remaining drift keys (added_by, project, cycle, severity, verdict...) are absent from TaskKeyAliases (keys.go:181-188, 7 entries) and keys.go:164-178 documents the evidence-based exclusion discipline; README.md:693-737, docs/operations.md §1b, skills/boardctl-usage/SKILL.md v1.17.0 document the repair path, refusals, and never-fabricate rule. Tests: go build ./... exit 0; go test -count=1 ./... => all 12 packages ok; DF36-specific 13/13 PASS (8 engine + 5 CLI); go vet clean; LSP diagnostics empty.
normalize-board provides a verified repair path that normalized the live warpfs board's alias keys/string depends_on/alias statuses into canon (drift 133->103 rows, warnings 626->527) with documented exclusions for non-alias keys and never-fabricated created_at, backed by 13 passing tests.

Overall: PASS ✓
