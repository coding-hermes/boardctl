# Verdict: DF-BOARDCTL-27

**Task:** validate warns on unparseable created_at
**Evaluated:** 2026-10-09T23:21:09.731141
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ validate warns (exit 0) on unparseable/empty created_at using report-parser rules; repo board copy yields exactly 4 warnings; suite green: internal/board/timestamps.go (new) mirrors the report parser: timestampLayouts is byte-identical to internal/render/load.go stampLayouts (16 layouts, verified), ValidTimestamp mirrors parseStamp (TrimSpace-empty-fails), and TimestampFieldFindings mirrors buildTaskRec's exclusion rules (created_at always; completed_at only on non-complete rows; updated_at/dispatched_at/blocked_since when present+broken). internal/board/validate.go:298-303 emits rep.Add("warn", ...) per finding, so severity is warn and exit stays 0. Live CLI on the repo board: `/tmp/boardctl -C .coding-hermes/board validate` → exit 0, RESULT: OK (18 warning(s)), and `grep -c "time-based metrics"` = 4 (RELEASE-2026-09-23-04, RELEASE-2026-09-23-05, DF-BOARDCTL-24, DF-BOARDCTL-25). Parity confirmed by an ad-hoc render test on the repo board: report tsExcl=7 (4 task rows + 3 event rows) and board.TimestampFieldFindings flags exactly the same 4 task rows. Suite green: `go test -count=1 ./...` → all packages ok (cmd/boardctl 2.490s, internal/board 0.461s, internal/render 0.033s, etc.); all 8 DF-27 tests PASS (TestDFBoardctl27ValidateWarnsOnBadTimestamps, TestDFBoardctl27CleanBoardValidatesQuiet, TestDFBoardctl27GrammarParityWithParseStamp, TestDFBoardctl27ExclusionVerdictParity, TestDFBoardctl27WarnsOnUnparseableCreatedAndCompleted, TestDFBoardctl27CleanBoardZeroTimestampWarns, TestDFBoardctl27CompletedAtFollowsReportFallback, TestDFBoardctl27ValidTimestampGrammar). go vet and gofmt clean.
validate now warns (exit 0) on unparseable/empty created_at using grammar and exclusion rules identical to the report parser, the repo board yields exactly 4 such warnings, and the full test suite is green.

## Summary

Judge Result: DF-BOARDCTL-27

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ validate warns (exit 0) on unparseable/empty created_at using report-parser rules; repo board copy yields exactly 4 warnings; suite green: internal/board/timestamps.go (new) mirrors the report parser: timestampLayouts is byte-identical to internal/render/load.go stampLayouts (16 layouts, verified), ValidTimestamp mirrors parseStamp (TrimSpace-empty-fails), and TimestampFieldFindings mirrors buildTaskRec's exclusion rules (created_at always; completed_at only on non-complete rows; updated_at/dispatched_at/blocked_since when present+broken). internal/board/validate.go:298-303 emits rep.Add("warn", ...) per finding, so severity is warn and exit stays 0. Live CLI on the repo board: `/tmp/boardctl -C .coding-hermes/board validate` → exit 0, RESULT: OK (18 warning(s)), and `grep -c "time-based metrics"` = 4 (RELEASE-2026-09-23-04, RELEASE-2026-09-23-05, DF-BOARDCTL-24, DF-BOARDCTL-25). Parity confirmed by an ad-hoc render test on the repo board: report tsExcl=7 (4 task rows + 3 event rows) and board.TimestampFieldFindings flags exactly the same 4 task rows. Suite green: `go test -count=1 ./...` → all packages ok (cmd/boardctl 2.490s, internal/board 0.461s, internal/render 0.033s, etc.); all 8 DF-27 tests PASS (TestDFBoardctl27ValidateWarnsOnBadTimestamps, TestDFBoardctl27CleanBoardValidatesQuiet, TestDFBoardctl27GrammarParityWithParseStamp, TestDFBoardctl27ExclusionVerdictParity, TestDFBoardctl27WarnsOnUnparseableCreatedAndCompleted, TestDFBoardctl27CleanBoardZeroTimestampWarns, TestDFBoardctl27CompletedAtFollowsReportFallback, TestDFBoardctl27ValidTimestampGrammar). go vet and gofmt clean.
validate now warns (exit 0) on unparseable/empty created_at using grammar and exclusion rules identical to the report parser, the repo board yields exactly 4 such warnings, and the full test suite is green.

Overall: PASS ✓
