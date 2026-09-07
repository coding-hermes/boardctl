# Verdict: BT-006

**Task:** Fix -C resolution: .coding-hermes dir must resolve to its board/; board-not-found exits 2
**Evaluated:** 2026-09-04T19:49:36.319719
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m2:48PM[0m [32mINF[0m [1mscanned ~146128 bytes (146.13 KB) in 83.2ms[0m
[90m2:48PM[0m [3
- ✓ **tier2**
  - COMPLETE
  ✓ go build/vet/test PASS; boardctl -C <repo>/.coding-hermes stats works; -C <repo>/.coding-hermes/board works; -C <repo> works; board-not-found exits 2; README exit-code table still consistent (0 ok, 1 validation failure, 2 usage/board-not-found): go build ./... exit 0; go vet ./... exit 0; go test -count=1 ./... exit 0 (ok cmd/boardctl, ok internal/board). Functional binary checks: /tmp/boardctl -C /tmp/bttest/.coding-hermes stats exit 0; -C /tmp/bttest/.coding-hermes/board stats exit 0; -C /tmp/bttest stats exit 0; board-not-found on boardless dir: stats exit 2 and create exit 2 (cmd/boardctl/main.go:117-122 returns 2 on errors.Is(err, board.ErrBoardNotFound)). README.md:73 exit-code table '0 ok, 1 validation failure, 2 usage/board-not-found' consistent. BT-006 tests pass: TestCmdExitCodes, TestCmdInitThenStatsViaCodingHermesDir, TestResolveRepoRootFindsNestedBoard, TestResolveCodingHermesDirFindsItsBoard, TestResolveBoardDirItself, TestResolveEmptyDirNotFound, TestInitOnCodingHermesDirUsesItsBoardSubdir. boardDirCandidates in internal/board/board.go:44-47 special-cases a .coding-hermes target to probe <given>/board.
All BT-006 requirements verified: build/vet/test pass, all three -C resolution forms work (exit 0), board-not-found exits 2, and README exit-code table remains consistent.

## Summary

Judge Result: BT-006

Stage tier1: PASS
    ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
  ✓ secrets: [90m2:48PM[0m [32mINF[0m [1mscanned ~146128 bytes (146.13 KB) in 83.2ms[0m
[90m2:48PM[0m [3

Stage tier2: PASS
  COMPLETE
  ✓ go build/vet/test PASS; boardctl -C <repo>/.coding-hermes stats works; -C <repo>/.coding-hermes/board works; -C <repo> works; board-not-found exits 2; README exit-code table still consistent (0 ok, 1 validation failure, 2 usage/board-not-found): go build ./... exit 0; go vet ./... exit 0; go test -count=1 ./... exit 0 (ok cmd/boardctl, ok internal/board). Functional binary checks: /tmp/boardctl -C /tmp/bttest/.coding-hermes stats exit 0; -C /tmp/bttest/.coding-hermes/board stats exit 0; -C /tmp/bttest stats exit 0; board-not-found on boardless dir: stats exit 2 and create exit 2 (cmd/boardctl/main.go:117-122 returns 2 on errors.Is(err, board.ErrBoardNotFound)). README.md:73 exit-code table '0 ok, 1 validation failure, 2 usage/board-not-found' consistent. BT-006 tests pass: TestCmdExitCodes, TestCmdInitThenStatsViaCodingHermesDir, TestResolveRepoRootFindsNestedBoard, TestResolveCodingHermesDirFindsItsBoard, TestResolveBoardDirItself, TestResolveEmptyDirNotFound, TestInitOnCodingHermesDirUsesItsBoardSubdir. boardDirCandidates in internal/board/board.go:44-47 special-cases a .coding-hermes target to probe <given>/board.
All BT-006 requirements verified: build/vet/test pass, all three -C resolution forms work (exit 0), board-not-found exits 2, and README exit-code table remains consistent.

Overall: PASS ✓
