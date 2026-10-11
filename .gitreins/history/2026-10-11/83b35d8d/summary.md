# Verdict: DOC-32

**Task:** Fix skill rot: Run-20 lessons contradict DF-BOARDCTL-29 (--deferred false now REMOVES the key)
**Evaluated:** 2026-10-11T05:02:40.040560
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ skills/boardctl-usage/SKILL.md:535-536 says 'update --deferred false stores an explicit "deferred":false key (does not remove it)'. README.md:775 and commit 4ab5527 (DF-BOARDCTL-29) say the opposite and the code implements removal (internal/board/write.go:1037). Live-verified at HEAD 9ba0b61: boardctl update DOC-TEST --deferred false leaves the row with NO deferred key. Fix: update SKILL.md:535-536 to state that --deferred false REMOVES the key, matching the code and README.: SKILL.md:547-548 now reads '`update --deferred false` REMOVES the `deferred` key entirely (DF-BOARDCTL-29, commit 4ab5527); the row is byte-identical to one that was never deferred.' The old contradictory text is gone (grep for 'does not remove'/'stores an explicit' returns nothing). This matches the code at internal/board/write.go:967-969 (else if target.Has("deferred") { target.DeleteKey("deferred") }) and README.md:876-878 ('un-defers by REMOVING the key entirely (DF-BOARDCTL-29)'). Verified via git show HEAD (commit a3a6dff, 2 insertions/2 deletions in SKILL.md) and go test -count=1 ./internal/board/... => 'ok github.com/coding-hermes/boardctl/internal/board 1.386s'. Note: the criterion cited lines 535-536, but the corrected text now sits at 547-548 due to surrounding content; the required semantic fix is present and correct.
SKILL.md's Run-20 lesson now correctly states that `update --deferred false` REMOVES the deferred key, matching the code (write.go:967-969) and README (876-878), with the board test suite passing.

## Summary

Judge Result: DOC-32

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ skills/boardctl-usage/SKILL.md:535-536 says 'update --deferred false stores an explicit "deferred":false key (does not remove it)'. README.md:775 and commit 4ab5527 (DF-BOARDCTL-29) say the opposite and the code implements removal (internal/board/write.go:1037). Live-verified at HEAD 9ba0b61: boardctl update DOC-TEST --deferred false leaves the row with NO deferred key. Fix: update SKILL.md:535-536 to state that --deferred false REMOVES the key, matching the code and README.: SKILL.md:547-548 now reads '`update --deferred false` REMOVES the `deferred` key entirely (DF-BOARDCTL-29, commit 4ab5527); the row is byte-identical to one that was never deferred.' The old contradictory text is gone (grep for 'does not remove'/'stores an explicit' returns nothing). This matches the code at internal/board/write.go:967-969 (else if target.Has("deferred") { target.DeleteKey("deferred") }) and README.md:876-878 ('un-defers by REMOVING the key entirely (DF-BOARDCTL-29)'). Verified via git show HEAD (commit a3a6dff, 2 insertions/2 deletions in SKILL.md) and go test -count=1 ./internal/board/... => 'ok github.com/coding-hermes/boardctl/internal/board 1.386s'. Note: the criterion cited lines 535-536, but the corrected text now sits at 547-548 due to surrounding content; the required semantic fix is present and correct.
SKILL.md's Run-20 lesson now correctly states that `update --deferred false` REMOVES the deferred key, matching the code (write.go:967-969) and README (876-878), with the board test suite passing.

Overall: FAIL ✗
