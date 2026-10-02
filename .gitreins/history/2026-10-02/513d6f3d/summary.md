# Verdict: DF-BOARDCTL-25

**Task:** doctor distinguishes topology B header from corrupt tasks.jsonl line 1
**Evaluated:** 2026-10-02T09:20:53.251850
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	160.618s
- ✓ **tier2**
  - COMPLETE
  ✓ doctor reports salvageable topology B line-1 header differently from genuinely corrupt line-1 data; tests RED-proven on both board shapes: internal/board/doctor.go: doctorTopologyBLineOne (called at line 39, defined ~line 192) emits three distinct outcomes: topology B + valid header row on line 1 -> warn 'legacy topology B: tasks.jsonl line 1 is the board header ... not corruption; the board is fully writable' (zero errors); topology B + unparseable line 1 -> error containing df25TopologyBHeaderCorruptionMsg='corruption of the topology-B header line' plus the 'fix: boardctl validate --repair ...' salvage hint; topology A/headerless -> silent. Test TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne (cmd/boardctl/df_boardctl_25_test.go, added in f9dc8a4) drives the real CLI over both board shapes, asserting exit codes, output lines, and zero board-file writes via sha256 snapshots. GREEN: `go test -count=1 -run TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne ./cmd/boardctl/` => 'ok github.com/coding-hermes/boardctl/cmd/boardctl 1.687s' (rerun 3.968s). RED-PROVEN live: reverting internal/board/doctor.go to pre-fix commit c0170f6 while keeping the new test produced '--- FAIL: .../valid_line-1_header... df_boardctl_25_test.go:124: doctor did not report the legacy topology-B header as a distinct, healthy layout' and '--- FAIL: .../corrupt_line_1... df_boardctl_25_test.go:154: doctor did not name corrupt line 1 as topology-B header corruption' — exactly the claimed RED messages; restoring the fix (git diff clean) returned PASS. go build ./... exit 0, go vet clean, LSP diagnostics empty.
doctor now distinguishes a salvageable topology-B line-1 header (warn, exit 0) from genuinely corrupt line-1 data (error + validate --repair hint, exit 1), and the acceptance test is RED-proven on both board shapes and GREEN post-fix.

## Summary

Judge Result: DF-BOARDCTL-25

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	160.618s

Stage tier2: PASS
  COMPLETE
  ✓ doctor reports salvageable topology B line-1 header differently from genuinely corrupt line-1 data; tests RED-proven on both board shapes: internal/board/doctor.go: doctorTopologyBLineOne (called at line 39, defined ~line 192) emits three distinct outcomes: topology B + valid header row on line 1 -> warn 'legacy topology B: tasks.jsonl line 1 is the board header ... not corruption; the board is fully writable' (zero errors); topology B + unparseable line 1 -> error containing df25TopologyBHeaderCorruptionMsg='corruption of the topology-B header line' plus the 'fix: boardctl validate --repair ...' salvage hint; topology A/headerless -> silent. Test TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne (cmd/boardctl/df_boardctl_25_test.go, added in f9dc8a4) drives the real CLI over both board shapes, asserting exit codes, output lines, and zero board-file writes via sha256 snapshots. GREEN: `go test -count=1 -run TestDF25DoctorDistinguishesTopologyBFromCorruptLineOne ./cmd/boardctl/` => 'ok github.com/coding-hermes/boardctl/cmd/boardctl 1.687s' (rerun 3.968s). RED-PROVEN live: reverting internal/board/doctor.go to pre-fix commit c0170f6 while keeping the new test produced '--- FAIL: .../valid_line-1_header... df_boardctl_25_test.go:124: doctor did not report the legacy topology-B header as a distinct, healthy layout' and '--- FAIL: .../corrupt_line_1... df_boardctl_25_test.go:154: doctor did not name corrupt line 1 as topology-B header corruption' — exactly the claimed RED messages; restoring the fix (git diff clean) returned PASS. go build ./... exit 0, go vet clean, LSP diagnostics empty.
doctor now distinguishes a salvageable topology-B line-1 header (warn, exit 0) from genuinely corrupt line-1 data (error + validate --repair hint, exit 1), and the acceptance test is RED-proven on both board shapes and GREEN post-fix.

Overall: PASS ✓
