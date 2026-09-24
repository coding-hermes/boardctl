# Verdict: BT-041

**Task:** CLI contract: subcommand --help exits 1; event without --type silently appends
**Evaluated:** 2026-09-24T11:57:11.328843
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	10.044s
- ✓ **tier2**
  - COMPLETE
  ✓ Subcommand --help exits a decided non-validation code across all subcommands with a pinned test; event without --type refuses with exit 1 and writes nothing; README exit-code contract updated; build/vet/test green: Help: all 16 subcommands route through parseFlags (cmd/boardctl/main.go:269; call sites main.go:389,429,521,593,679,768,849,937,1022,1064,1142,1182 + import.go:94, install.go:70, render.go:40, serve.go:79); only remaining fs.Parse is inside parseFlags (main.go:275). Runtime on built binary: every `<sub> --help` (init,list,show,create,update,event,header,validate,sweep-status,doctor,version,stats,render,import,serve,install) exits 0 — a decided non-validation code. Pinned test TestBT041HelpExitCodeZeroPerSubcommand (cmd/boardctl/bt041_help_exit_test.go:44) tables all 16 subcommands x {-h,--help} asserting exit 0 + usage on stdout; PASS. Event: cmdEvent gate (main.go:774+) refuses before board open; runtime `event` with no --type exits 1, stderr 'event: --type is required (no default; allowed event_type values: audit,...)', events.jsonl md5 d41d8cd98f00b204e9800998ecf8427e unchanged before/after; explicit `--type audit` still appends (exit 0). Test TestBT041EventWithoutTypeRefusesAndWritesNothing PASS. README.md:192-208 exit-code contract updated (help=0, event requires --type, exit 1, writes nothing). Build/vet/test: `go build ./... && go vet ./...` => BUILD_VET_OK; `go test -count=1 ./...` => ok cmd/boardctl 7.965s, ok internal/board 5.670s, all packages ok; `go test -run BT041 -v` => all 4 tests PASS. LSP diagnostics: 0.
All BT-041 requirements verified: help exits a decided non-validation code (0) across all 16 subcommands with a pinned test, event without --type refuses with exit 1 and writes nothing, README exit-code contract updated, and build/vet/test are green.

## Summary

Judge Result: BT-041

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/coding-hermes/boardctl/cmd/boardctl	10.044s

Stage tier2: PASS
  COMPLETE
  ✓ Subcommand --help exits a decided non-validation code across all subcommands with a pinned test; event without --type refuses with exit 1 and writes nothing; README exit-code contract updated; build/vet/test green: Help: all 16 subcommands route through parseFlags (cmd/boardctl/main.go:269; call sites main.go:389,429,521,593,679,768,849,937,1022,1064,1142,1182 + import.go:94, install.go:70, render.go:40, serve.go:79); only remaining fs.Parse is inside parseFlags (main.go:275). Runtime on built binary: every `<sub> --help` (init,list,show,create,update,event,header,validate,sweep-status,doctor,version,stats,render,import,serve,install) exits 0 — a decided non-validation code. Pinned test TestBT041HelpExitCodeZeroPerSubcommand (cmd/boardctl/bt041_help_exit_test.go:44) tables all 16 subcommands x {-h,--help} asserting exit 0 + usage on stdout; PASS. Event: cmdEvent gate (main.go:774+) refuses before board open; runtime `event` with no --type exits 1, stderr 'event: --type is required (no default; allowed event_type values: audit,...)', events.jsonl md5 d41d8cd98f00b204e9800998ecf8427e unchanged before/after; explicit `--type audit` still appends (exit 0). Test TestBT041EventWithoutTypeRefusesAndWritesNothing PASS. README.md:192-208 exit-code contract updated (help=0, event requires --type, exit 1, writes nothing). Build/vet/test: `go build ./... && go vet ./...` => BUILD_VET_OK; `go test -count=1 ./...` => ok cmd/boardctl 7.965s, ok internal/board 5.670s, all packages ok; `go test -run BT041 -v` => all 4 tests PASS. LSP diagnostics: 0.
All BT-041 requirements verified: help exits a decided non-validation code (0) across all 16 subcommands with a pinned test, event without --type refuses with exit 1 and writes nothing, README exit-code contract updated, and build/vet/test are green.

Overall: PASS ✓
