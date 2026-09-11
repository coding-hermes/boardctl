# Verdict: BT-016

**Task:** README Start-a-board truth: init writes four files, not three
**Evaluated:** 2026-09-11T17:41:35.953171
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:41PM[0m [32mINF[0m [1mscanned ~323590 bytes (323.59 KB) in 243ms[0m
[90m12:41PM[0m [
- ✓ **tier2**
  - COMPLETE
  ✓ README.md Start a board section states init writes four topology-A files (tasks.jsonl seeded with the NEVER-DONE perpetual fixture row, events.jsonl, board.jsonl header, fixtures.jsonl registry) instead of the stale three files nothing else; no other stale init claims remain in README; commit pushed.: README.md:72-77 now reads 'init bootstraps one — it writes the four topology-A files (tasks.jsonl, events.jsonl, the board.jsonl header, and fixtures.jsonl) under <dir>/.coding-hermes/board, nothing else. tasks.jsonl starts seeded with the fleet-standard NEVER-DONE perpetual audit fixture row, which is registered in fixtures.jsonl (the perpetual fixture registry) as well.' This matches code: internal/board/init.go:88-93 seeds exactly 4 files {tasks.jsonl, events.jsonl, board.jsonl, fixtures.jsonl} and defaultFixtureLine() (init.go:121-155) sets id=NEVER-DONE, perpetual:true written to both tasks.jsonl and fixtures.jsonl. No stale 'three files' claim remains — grep for 'three|3 files' hits only unrelated lines (7, 72, 91). Commit d7f6419 'docs: README start-a-board - init writes four files... Addresses BT-016' is present in origin/main (git log origin/main shows it), so it is pushed. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/` => 'ok github.com/coding-hermes/boardctl/internal/board 0.078s' and 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.017s', exit_code 0; init_test.go:47-68 asserts len(wrote)==4 and NEVER-DONE seed in both files.
README Start a board section correctly documents init writing four topology-A files with the NEVER-DONE perpetual fixture seed, matching the code, with no stale claims and the commit pushed to origin/main.

## Summary

Judge Result: BT-016

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m12:41PM[0m [32mINF[0m [1mscanned ~323590 bytes (323.59 KB) in 243ms[0m
[90m12:41PM[0m [

Stage tier2: PASS
  COMPLETE
  ✓ README.md Start a board section states init writes four topology-A files (tasks.jsonl seeded with the NEVER-DONE perpetual fixture row, events.jsonl, board.jsonl header, fixtures.jsonl registry) instead of the stale three files nothing else; no other stale init claims remain in README; commit pushed.: README.md:72-77 now reads 'init bootstraps one — it writes the four topology-A files (tasks.jsonl, events.jsonl, the board.jsonl header, and fixtures.jsonl) under <dir>/.coding-hermes/board, nothing else. tasks.jsonl starts seeded with the fleet-standard NEVER-DONE perpetual audit fixture row, which is registered in fixtures.jsonl (the perpetual fixture registry) as well.' This matches code: internal/board/init.go:88-93 seeds exactly 4 files {tasks.jsonl, events.jsonl, board.jsonl, fixtures.jsonl} and defaultFixtureLine() (init.go:121-155) sets id=NEVER-DONE, perpetual:true written to both tasks.jsonl and fixtures.jsonl. No stale 'three files' claim remains — grep for 'three|3 files' hits only unrelated lines (7, 72, 91). Commit d7f6419 'docs: README start-a-board - init writes four files... Addresses BT-016' is present in origin/main (git log origin/main shows it), so it is pushed. Tests: `go test -count=1 ./internal/board/ ./cmd/boardctl/` => 'ok github.com/coding-hermes/boardctl/internal/board 0.078s' and 'ok github.com/coding-hermes/boardctl/cmd/boardctl 0.017s', exit_code 0; init_test.go:47-68 asserts len(wrote)==4 and NEVER-DONE seed in both files.
README Start a board section correctly documents init writing four topology-A files with the NEVER-DONE perpetual fixture seed, matching the code, with no stale claims and the commit pushed to origin/main.

Overall: PASS ✓
