# BT-094 verification-only evidence

Tick: boardctl-foreman-2026-11-14-09-03-05
Repository: coding-hermes/boardctl
Tested commit: 61e3d4a2f503948a0af8296892840f5f0b097a38
Selected row: BT-094
Disposition: PENDING; no worker dispatched

## Owner-gated classification

BT-094's acceptance targets the Hermes gateway runtime's mem-sync executor. That runtime is outside this repository, so this boardctl tick cannot implement the requested fix or perform the required restarted-gateway >=2h witness. The row is already deferred with the explicit owner reason and review date 2026-11-17.

## Fresh runtime probe

Command: `ps aux | grep mem-sync | wc -l`
Observed result: `2`
Interpretation: the literal command does not produce the required <=1 result on this host. The count includes the command's own grep/process text, so it is not a trustworthy named-thread count by itself.

Command: `ps -eo pid,ppid,etime,comm,args | grep mem-sync | grep -v grep`
Observed result: no matching `mem-sync` process lines.

Command: `pgrep -af mem-sync`
Observed result: only the probe shell's command text matched; no runtime `mem-sync_0` process was enumerated.

This does not satisfy BT-094. It neither proves the executor fix nor the required restarted gateway after >=2h uptime. The prior owner-gate evidence recorded 408 `mem-sync_0` threads on 2026-11-10; the runtime owner must supply the authoritative post-fix witness.

## Repository verification at tested commit

The dedicated worktree was created from `origin/main` at `61e3d4a2f503948a0af8296892840f5f0b097a38` and was clean before this artifact was authored.

## Repository gates observed

All listed gates exited 0 at the tested commit. The test suite ended with passing packages including `internal/vulncheck`, `internal/vendorcheck`, `internal/versioncheck`, and `internal/workflowcheck`.

- `go build -o bin/boardctl ./cmd/boardctl` — rc 0
- `go vet ./...` — rc 0
- `go test -count=1 -short ./...` — rc 0
- `make fmt-check` — rc 0 (`TestGofmt` passed)
- `make version-check VERSION_TAG=v0.4.0` — rc 0
- `make vendor-check` — rc 0
- `./bin/boardctl validate -C .` — rc 0, `RESULT: OK (44 warning(s))`
- `./bin/boardctl doctor -C .` — rc 0, `RESULT: OK (45 warning(s))`

The warnings are inherited board hygiene/evidence warnings; no error was reported by either board command.

## Trace

ch:trace row=BT-094 spec=.coding-hermes/board/tasks.jsonl#BT-094 wave=.coding-hermes/waves/boardctl-foreman-2026-11-14-09-03-05.json#selected test=none:cross-repo-runtime-owner-gated doc=none evidence=.coding-hermes/board/evidence/boardctl-foreman-2026-11-14-09-03-05.md witness=live:ps-mem-sync-thread-census verdict=none:verification-evidence-not-yet-judged commit=61e3d4a2f503948a0af8296892840f5f0b097a38 memory=none:tick-local
STATUS: partial
TIER: T3
EVIDENCE: fresh runtime census plus repository gate transcript; owner-controlled gateway fix and >=2h post-restart witness remain unmet
