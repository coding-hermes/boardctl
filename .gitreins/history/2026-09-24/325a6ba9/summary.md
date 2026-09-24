# Verdict: REVIEW-BOARDCTL-001

**Task:** Fleet status vocabulary: marker field on dedupe merges, status-rejection regression test, re-runnable per-board sweep
**Evaluated:** 2026-09-24T10:37:06.744710
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.873s
- ✓ **tier2**
  - COMPLETE
  ✓ go build/vet/test green on merged tree; update --status duplicate and create --status duplicate exit 1 writing nothing; DedupeBackfill merged rows carry superseded_by=<kept>; new per-board sweep command dry-run by default reporting before/after off-vocab counts; committed on main: Build/vet/test: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 (all pkgs ok: cmd/boardctl 0.631s, internal/board 0.127s, etc.). Status rejection: cmd/boardctl/review_status_vocab_test.go TestCmdWriteRejectsStatusDuplicate asserts `run([... update EXIST-1 --status duplicate])==1` and byte-compares tasks.jsonl before/after (no mutation), then `create --id NEW-1 --status duplicate`==1 with byte-compare; PASS output 'boardctl: status "duplicate" not in write vocabulary'. internal/board/review_status_vocab_test.go TestWriteRejectsOffVocabularyStatus PASS (create/update reject duplicate/parked/resolved/retired + aliases, line counts unchanged). DedupeBackfill marker: internal/board/backfill.go:233 `m.row.SetGoValue("superseded_by", kept.id, mstyle)`; write_test.go:955 and :1098 assert `row["superseded_by"]=="QA-BF-1"`/"QA-DUP-1"; TestDedupeBackfill_MergesCollisions/DryRunIsIdempotent/DuplicateIDsCollapse all PASS. Sweep: internal/board/sweep.go StatusSweep(apply bool) with `if !apply { return rep, nil }` (dry-run default, writes nothing) and RenderText printing 'before: N off-vocabulary rows; after: M off-vocabulary rows'; cmd/boardctl/main.go:130 `case "sweep-status"`, cmdSweepStatus with `apply := fs.Bool("apply", false, ...)` and 'DRY-RUN — no files written' banner; TestCmdSweepStatusDryRunWritesNothing (byte-compare no write, '2/3 rows off-vocabulary'), TestCmdSweepStatusApplyNormalizesAliasOnly ('before: 2 ... after: 1', duplicate row untouched), TestCmdSweepStatusJSON all PASS. Committed on main: HEAD a028f2f19db26785bebf65f86c098f4bf8abf676 is on branch main (and origin/main), merge commit titled 'merge: REVIEW-BOARDCTL-001 ...'.
All five sub-requirements verified: build/vet/test green, duplicate status rejected with exit 1 and no writes on both create/update, DedupeBackfill sets superseded_by=<kept>, sweep-status is dry-run by default with before/after off-vocab counts, and the work is committed on main.

## Summary

Judge Result: REVIEW-BOARDCTL-001

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	0.873s

Stage tier2: PASS
  COMPLETE
  ✓ go build/vet/test green on merged tree; update --status duplicate and create --status duplicate exit 1 writing nothing; DedupeBackfill merged rows carry superseded_by=<kept>; new per-board sweep command dry-run by default reporting before/after off-vocab counts; committed on main: Build/vet/test: `go build ./...` exit 0, `go vet ./...` exit 0, `go test -count=1 ./...` exit 0 (all pkgs ok: cmd/boardctl 0.631s, internal/board 0.127s, etc.). Status rejection: cmd/boardctl/review_status_vocab_test.go TestCmdWriteRejectsStatusDuplicate asserts `run([... update EXIST-1 --status duplicate])==1` and byte-compares tasks.jsonl before/after (no mutation), then `create --id NEW-1 --status duplicate`==1 with byte-compare; PASS output 'boardctl: status "duplicate" not in write vocabulary'. internal/board/review_status_vocab_test.go TestWriteRejectsOffVocabularyStatus PASS (create/update reject duplicate/parked/resolved/retired + aliases, line counts unchanged). DedupeBackfill marker: internal/board/backfill.go:233 `m.row.SetGoValue("superseded_by", kept.id, mstyle)`; write_test.go:955 and :1098 assert `row["superseded_by"]=="QA-BF-1"`/"QA-DUP-1"; TestDedupeBackfill_MergesCollisions/DryRunIsIdempotent/DuplicateIDsCollapse all PASS. Sweep: internal/board/sweep.go StatusSweep(apply bool) with `if !apply { return rep, nil }` (dry-run default, writes nothing) and RenderText printing 'before: N off-vocabulary rows; after: M off-vocabulary rows'; cmd/boardctl/main.go:130 `case "sweep-status"`, cmdSweepStatus with `apply := fs.Bool("apply", false, ...)` and 'DRY-RUN — no files written' banner; TestCmdSweepStatusDryRunWritesNothing (byte-compare no write, '2/3 rows off-vocabulary'), TestCmdSweepStatusApplyNormalizesAliasOnly ('before: 2 ... after: 1', duplicate row untouched), TestCmdSweepStatusJSON all PASS. Committed on main: HEAD a028f2f19db26785bebf65f86c098f4bf8abf676 is on branch main (and origin/main), merge commit titled 'merge: REVIEW-BOARDCTL-001 ...'.
All five sub-requirements verified: build/vet/test green, duplicate status rejected with exit 1 and no writes on both create/update, DedupeBackfill sets superseded_by=<kept>, sweep-status is dry-run by default with before/after off-vocab counts, and the work is committed on main.

Overall: PASS ✓
