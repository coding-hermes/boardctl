# Verdict: DOC-27

**Task:** document complete event_type vocabulary
**Evaluated:** 2026-10-11T05:21:13.013580
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README gains a section enumerating all 18 event_type values from cmd/boardctl/main.go:1036 (audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_updated,task_verified,tick,worker_dispatched); skills/boardctl-usage/SKILL.md gains the same enumeration; tests or CLI probe confirm the list matches the live refusal message: README.md:533-580 adds '## Event type vocabulary (DOC-27)' enumerating all 18 values (programmatic check: README missing=[]). skills/boardctl-usage/SKILL.md:226-237 enumerates the same 18 (SKILL missing=[]). Live set internal/board/read.go:107-126 EventTypeVocabulary contains exactly those 18 keys. CLI probe: `boardctl event --type bogus` -> exit=1, stderr 'event type "bogus" not in vocabulary {audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_updated,task_verified,tick,worker_dispatched} — use one of the enumerated event_type values' — matches README's documented refusal verbatim. Tests: cmd/boardctl/bt041_help_exit_test.go:123 iterates board.EventTypeVocabulary asserting each member appears in the refusal, and :177 asserts 'not in vocabulary'; `go test ./cmd/boardctl/ -run TestBT041 -count=1` -> ok github.com/coding-hermes/boardctl/cmd/boardctl 0.010s.
README and SKILL.md both enumerate all 18 event_type values matching the live EventTypeVocabulary, confirmed by a CLI probe and passing TestBT041 tests.

## Summary

Judge Result: DOC-27

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README gains a section enumerating all 18 event_type values from cmd/boardctl/main.go:1036 (audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_updated,task_verified,tick,worker_dispatched); skills/boardctl-usage/SKILL.md gains the same enumeration; tests or CLI probe confirm the list matches the live refusal message: README.md:533-580 adds '## Event type vocabulary (DOC-27)' enumerating all 18 values (programmatic check: README missing=[]). skills/boardctl-usage/SKILL.md:226-237 enumerates the same 18 (SKILL missing=[]). Live set internal/board/read.go:107-126 EventTypeVocabulary contains exactly those 18 keys. CLI probe: `boardctl event --type bogus` -> exit=1, stderr 'event type "bogus" not in vocabulary {audit,board_bootstrap,board_init,board_migration,dogfood,e2e_verified,idle,spec_created,task_added,task_completed,task_created,task_dispatched,task_evidence,task_started,task_updated,task_verified,tick,worker_dispatched} — use one of the enumerated event_type values' — matches README's documented refusal verbatim. Tests: cmd/boardctl/bt041_help_exit_test.go:123 iterates board.EventTypeVocabulary asserting each member appears in the refusal, and :177 asserts 'not in vocabulary'; `go test ./cmd/boardctl/ -run TestBT041 -count=1` -> ok github.com/coding-hermes/boardctl/cmd/boardctl 0.010s.
README and SKILL.md both enumerate all 18 event_type values matching the live EventTypeVocabulary, confirmed by a CLI probe and passing TestBT041 tests.

Overall: FAIL ✗
