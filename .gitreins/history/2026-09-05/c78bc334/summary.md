# Verdict: BT-009

**Task:** Install docs assume a Go toolchain first; static binary path is the zero-dep one — reorder and add bootstrap example
**Evaluated:** 2026-09-05T01:29:28.428857
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m8:28PM[0m [32mINF[0m [1mscanned ~206137 bytes (206.14 KB) in 95.9ms[0m
[90m8:28PM[0m [3
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/
- ✓ **tier2**
  - COMPLETE
  ✓ README Install section reorders: (1) static binary + sha256 verify, (2) go install for Go users, (3) build from source; adds a 'Start a board' example using boardctl init so time-to-first-success is documented; go test ./... and go vet ./... still pass; README renders correctly: README.md Install section reordered correctly: (1) static binary + sha256 verify first (lines 12-13 'The zero-dependency path: grab a static binary from [releases]... Verify the binary's checksum before running it'), (2) go install for Go users second (lines 23-24 'With a Go toolchain, go install works too'), (3) build from source third (lines 28-29 'Or build directly from a checkout... go build ./cmd/boardctl'). 'Start a board' section (line 70) documents time-to-first-success with boardctl init/create/stats example; init command confirmed in cmd/boardctl/main.go:87,230. go test -count=1 ./... exit 0 (ok cmd/boardctl, ok internal/board). go vet ./... exit 0. README renders correctly: 14 code fences balanced (7 pairs), well-formed headings, [Development](#development) resolves to ## Development at line 128.
README Install section correctly reordered (static binary+sha256 first, go install second, build-from-source third), adds a boardctl init 'Start a board' example, and go test/go vet both pass with README rendering valid.

## Summary

Judge Result: BT-009

Stage tier1: PASS
    ✓ secrets: [90m8:28PM[0m [32mINF[0m [1mscanned ~206137 bytes (206.14 KB) in 95.9ms[0m
[90m8:28PM[0m [3
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	(cached)
ok  	github.com/coding-hermes/boardctl/

Stage tier2: PASS
  COMPLETE
  ✓ README Install section reorders: (1) static binary + sha256 verify, (2) go install for Go users, (3) build from source; adds a 'Start a board' example using boardctl init so time-to-first-success is documented; go test ./... and go vet ./... still pass; README renders correctly: README.md Install section reordered correctly: (1) static binary + sha256 verify first (lines 12-13 'The zero-dependency path: grab a static binary from [releases]... Verify the binary's checksum before running it'), (2) go install for Go users second (lines 23-24 'With a Go toolchain, go install works too'), (3) build from source third (lines 28-29 'Or build directly from a checkout... go build ./cmd/boardctl'). 'Start a board' section (line 70) documents time-to-first-success with boardctl init/create/stats example; init command confirmed in cmd/boardctl/main.go:87,230. go test -count=1 ./... exit 0 (ok cmd/boardctl, ok internal/board). go vet ./... exit 0. README renders correctly: 14 code fences balanced (7 pairs), well-formed headings, [Development](#development) resolves to ## Development at line 128.
README Install section correctly reordered (static binary+sha256 first, go install second, build-from-source third), adds a boardctl init 'Start a board' example, and go test/go vet both pass with README rendering valid.

Overall: PASS ✓
