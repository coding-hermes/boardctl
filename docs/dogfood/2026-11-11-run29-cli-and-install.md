# boardctl dogfood — run 29

Date: 2026-11-11
Tick: boardctl-dogfood-2026-11-11-04-58-53
Target HEAD: 1a582091c2b9117c6358277a15a9c6dc4fe8d074
Published ref: origin/main, parity 0/0
Canonical product checkout: /home/kara/coding-hermes-boardctl
Scheduler stand-in: /home/kara/.hermes/stand-in/dogfood/boardctl (not a Git checkout)

Verdict: PROMISING-BUT-ROUGH

Promise tested
A real user can build or install boardctl, initialize a JSONL board, create and defer tasks, inspect actionable work, update notes, validate/diagnose the board, render a report, and use the documented release/install paths.

Local real-use path
A fresh scratch board was initialized, populated with one actionable task and one deferred task, then read through list, stats, show --events, next, ready --json, direction --json, validate, doctor, render, and version --json. All commands exited 0. The resulting board had 3 tasks including the fixture, 2 events, deferred=1, actionable=2, ready_rows=1, blocked_rows=1, and a 69,447-byte HTML report plus 4,816-byte JSON report. The deferred task was excluded from ready work and retained in stats/list output as documented.

The README examples were also driven literally against a current source build. `create --id FEAT-1 --title "Add retry"` succeeded; the documented completion example using `--commit-hash abc1234` was refused because the current guard requires a full commit id; the documented bare `--deferred` example was refused because current code requires `--defer-reason`. These reproduce existing DF-BOARDCTL-54; no sibling row was filed.

Build and gates
`go build -o <scratch>/boardctl ./cmd/boardctl`: rc=0, 0.88s build wall, binary reports version dev.
`go vet ./...`: rc=0.
`go test -short -p 1 ./...`: rc=0; cmd/boardctl 27.407s and internal/board 20.506s were the largest package durations.

Performance
The completing user operation `boardctl -C <scratch-board> stats --json` completed warm and under an isolated HOME with rc=0; `/usr/bin/time -p` reported 0.00s real/user/sys for each arm and five repeated warm samples at the timer's displayed precision. This is below a user-noticeable threshold for the tiny two-task board, so no PERF row was justified. A hyperfine export attempt was blocked by the unattended execution gate before running; it is not timing evidence and is not reported as a product failure.

Ephemeral install leg
The bunker leg was run through `bunker-qa.sh` on bunker-mvp, agent e3a6786d, against the canonical checkout and exact target HEAD. Launch succeeded; the agent-side ledger reached 21 rows and `qa-run.finished` was present. Direct harvest recorded:

- fresh-install: OK, source build-only install to `~/.local/bin/boardctl`
- native ci-pass: OK
- ci-act: UNVERIFIED, rc=1 in the vulnerability-check step; hosted CI is authoritative
- upgrade: OK, previous release v0.2.0 tree to HEAD reinstall
- chaos-disconnect: OK, fails fast with rc=1
- chaos-resource: PASS under 3 GiB cap
- chaos-errorpath: OK, missing config rc=2
- docker-deploy, chaos-shutdown, chaos-corruption: N/A by repository shape
- ui-probe: INFO; the Go-native start command is not a long-running HTML server

The wrapper's later collect call exceeded the local bounded wait while the agent was still running. A direct `bunker exec` probe observed `qa-run.finished` and 21 agent-side rows, but the wrapper's local evidence file retained only its two launch rows before teardown; the complete agent ledger was not persisted locally before destruction, so the cell bundle is incomplete as a durable artifact. The run-owned agent was destroyed successfully and `bunker info e3a6786d --server bunker-mvp` returned not_found. The host reports private /tmp but shared network namespace and open egress, so the install is install-proven / isolation-UNVERIFIED, not an unqualified pass. No SKIPPED-install-bunker row was filed because bunker-mvp was reachable and the install path ran.

Existing-row linkage
- DF-BOARDCTL-52..55: current source-build probes confirm the already-open provenance, retry, documentation, and asymmetric-deferral subjects; no duplicate rows.
- QA-BOARDCTL-004: the fresh act leg again reports a vulnerability-check failure; native tests pass, so the failure remains environment/toolchain/scan evidence already tracked there.
- QA-BOARDCTL-009: the fresh upgrade cell reports v0.2.0 as its previous-release tree while README identifies v0.3.0 as current; the cell is not proof of the shipped v0.3.0-to-HEAD path.
- QA-BOARDCTL-006: chaos-corruption remains N/A in the generic battery even though boardctl's real state is JSONL; no new row.

The install evidence has a harness boundary: the local wrapper artifact is non-empty but incomplete (2 launch rows), while the 21-row direct agent ledger was observed but not saved before teardown. No new board rows were created. No code, board files, scheduler state, visibility, permissions, or credentials were changed. This report, the diagnostics addendum, the usage-skill lessons, and the run-log entry are the only repository changes from this tick.
