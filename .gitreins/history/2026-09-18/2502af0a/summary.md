# Verdict: SCHED-GAP-126

**Task:** Board-row dedupe: refuse (or merge evidence) on a finding fingerprint already open
**Evaluated:** 2026-09-18T12:17:47.202523
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.899s
- ✓ **tier2**
  - COMPLETE
  ✓ Acceptance: 6 unit tests in internal/board/write_test.go covering (1) re-file refused + task_evidence event, (2) grandfathered open row, (3) --force override, (4) backfill merges collisions, (5) backfill dry-run is idempotent, (6) duplicate-id collapse. New files: internal/board/fingerprint.go (normalizer + SHA-256), internal/board/backfill.go (DedupeBackfill), cmd/dedupe-board/main.go (CLI with --board-dir, --lanes, --json, --apply). Gate in Board.Create returns *ErrDuplicateFinding, CLI surfaces DUPLICATE-SUPPRESSED + exit 2. RED mutation: gate bypassed -> TestCreate_DedupeOnFingerprint fails with 're-filed finding accepted'. Docs in README.md + docs/board-fingerprint-rules.md. Scope = boardctl repo only. Commit 562659e.: All 6 tests exist in internal/board/write_test.go: TestCreate_DedupeOnFingerprint (L732, asserts refusal + task_evidence event with existing_task_id/new_id_attempted/fingerprint/evidence.ts), TestCreate_GrandfatherExistingOpenRow (L813), TestCreate_ForceOverride (L850), TestDedupeBackfill_MergesCollisions (L884), TestDedupeBackfill_DryRunIsIdempotent (L997), TestDedupeBackfill_DuplicateIDsCollapse (L1040). `go test -count=1 -v -run '<6 tests>' ./internal/board/` -> all 6 `--- PASS`; full `go test -count=1 ./...` -> `ok github.com/coding-hermes/boardctl/internal/board 0.147s` (exit 0). New files present: internal/board/fingerprint.go (NormalizeFindingText L63, FindingFingerprint sha256 L82, findingFingerprintSep='\x1f' L36), internal/board/backfill.go (DedupeBackfill L91), cmd/dedupe-board/main.go (flags --board-dir L47, --apply L48, --lanes L49, --json L50). Gate at internal/board/write.go:255-269 returns *ErrDuplicateFinding (type L27, Error() L33-34 prints 'DUPLICATE-SUPPRESSED: ...'). CLI: cmd/boardctl/main.go:146-150 errors.As(*board.ErrDuplicateFinding) -> prints bare line + return 2; verified live: first create exit 0, re-file printed 'DUPLICATE-SUPPRESSED: QA-T-1 matches fingerprint 88951a...' with exit 2. RED mutation verified: replacing the gate condition with `if false` made TestCreate_DedupeOnFingerprint FAIL with 'write_test.go:752: re-filed finding accepted — the fingerprint gate did not fire'; file restored (git diff clean). Docs: README.md:219-263 ('Finding fingerprints (SG-126)') and docs/board-fingerprint-rules.md (150 lines). Commit 562659e == HEAD, changes confined to boardctl repo (README.md, cmd/boardctl, cmd/dedupe-board, docs, internal/board).


## Summary

Judge Result: SCHED-GAP-126

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.899s

Stage tier2: PASS
  COMPLETE
  ✓ Acceptance: 6 unit tests in internal/board/write_test.go covering (1) re-file refused + task_evidence event, (2) grandfathered open row, (3) --force override, (4) backfill merges collisions, (5) backfill dry-run is idempotent, (6) duplicate-id collapse. New files: internal/board/fingerprint.go (normalizer + SHA-256), internal/board/backfill.go (DedupeBackfill), cmd/dedupe-board/main.go (CLI with --board-dir, --lanes, --json, --apply). Gate in Board.Create returns *ErrDuplicateFinding, CLI surfaces DUPLICATE-SUPPRESSED + exit 2. RED mutation: gate bypassed -> TestCreate_DedupeOnFingerprint fails with 're-filed finding accepted'. Docs in README.md + docs/board-fingerprint-rules.md. Scope = boardctl repo only. Commit 562659e.: All 6 tests exist in internal/board/write_test.go: TestCreate_DedupeOnFingerprint (L732, asserts refusal + task_evidence event with existing_task_id/new_id_attempted/fingerprint/evidence.ts), TestCreate_GrandfatherExistingOpenRow (L813), TestCreate_ForceOverride (L850), TestDedupeBackfill_MergesCollisions (L884), TestDedupeBackfill_DryRunIsIdempotent (L997), TestDedupeBackfill_DuplicateIDsCollapse (L1040). `go test -count=1 -v -run '<6 tests>' ./internal/board/` -> all 6 `--- PASS`; full `go test -count=1 ./...` -> `ok github.com/coding-hermes/boardctl/internal/board 0.147s` (exit 0). New files present: internal/board/fingerprint.go (NormalizeFindingText L63, FindingFingerprint sha256 L82, findingFingerprintSep='\x1f' L36), internal/board/backfill.go (DedupeBackfill L91), cmd/dedupe-board/main.go (flags --board-dir L47, --apply L48, --lanes L49, --json L50). Gate at internal/board/write.go:255-269 returns *ErrDuplicateFinding (type L27, Error() L33-34 prints 'DUPLICATE-SUPPRESSED: ...'). CLI: cmd/boardctl/main.go:146-150 errors.As(*board.ErrDuplicateFinding) -> prints bare line + return 2; verified live: first create exit 0, re-file printed 'DUPLICATE-SUPPRESSED: QA-T-1 matches fingerprint 88951a...' with exit 2. RED mutation verified: replacing the gate condition with `if false` made TestCreate_DedupeOnFingerprint FAIL with 'write_test.go:752: re-filed finding accepted — the fingerprint gate did not fire'; file restored (git diff clean). Docs: README.md:219-263 ('Finding fingerprints (SG-126)') and docs/board-fingerprint-rules.md (150 lines). Commit 562659e == HEAD, changes confined to boardctl repo (README.md, cmd/boardctl, cmd/dedupe-board, docs, internal/board).


Overall: PASS ✓
