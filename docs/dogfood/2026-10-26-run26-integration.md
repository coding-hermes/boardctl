# boardctl dogfood — run 26 (2026-10-26, tick boardctl-dogfood-2026-10-26-11-48-19)

verdict=PROMISING-BUT-ROUGH

promise: A real user can install boardctl, bootstrap a JSONL foreman board, create/update tasks including dispatchability metadata, inspect audit events, validate/diagnose the board, and recover readable rows from a damaged board; the CLI should be fast enough for interactive use.

## Real-use evidence

- Built a detached HEAD worktree at commit 11ac9b0 and drove the documented CLI against a fresh scratch board outside the repository. `version --json` returned `{"version":"dev","build":"dev"}`.
- `init` created the documented four topology-A files and printed the next create command; rc=0, wall=3ms.
- `create --acceptance-criteria` wrote DF-REAL-001; rc=0, wall=3ms. A second identical create refused with rc=1 and `task id "DF-REAL-001" already exists ... — create aborted`, preserving the board.
- `update --acceptance-criteria --note` returned rc=0, wall=4ms. `show --events` read the stored acceptance criteria and task_created audit event. Independent JSON read confirmed the stored row retained the title, status, priority, acceptance_criteria and foreman_note.
- `validate` returned rc=0, `RESULT: OK`, 2 task rows, 1 event, 1 fixture, and `key uniformity: 2/2 rows carry only canon keys`. `stats --json` returned total=1, pending=1, P2=1, actionable=1. `doctor`, `list --json`, `sweep-status --json`, and `normalize-board --json` all worked on the fresh board.
- Corruption path: appended one malformed JSON line to a copy. Default `show` failed closed rc=1 naming `tasks.jsonl: line 3`; `show --skip-bad-lines` returned the valid row, emitted a `SKIPPED-LINES` block, and still exited rc=1; `validate --skip-bad-lines` returned rc=1 with the exact line and repair hint.

## Performance

Measured the user-visible `stats --json` on the 2-task scratch board using the release-mode-equivalent optimized Go binary built from HEAD: `hyperfine --warmup 3 --runs 20 '<binary> -C <scratch> stats --json'` mean 11.3ms ±1.3ms, range 9.1–12.7ms over 10 runs; a separate first/warm time capture was 3ms for both invocations on the same scratch board. Hyperfine warned that sub-5ms timings approach shell-startup calibration limits. This is not user-noticeable; no PERF row and no profile were warranted.

## Fresh-machine install leg

`bunker-qa.sh launch` and `collect` ran on bunker-mvp, agent 2b9afbbf, against HEAD 11ac9b0. The collector evidence file is `/var/tmp/hermes-scratch/boardctl-bunker-evidence-20261026-1159.jsonl`; collect appended 19 per-cell rows plus the collect row and reported the temporary agent destroyed. The aggregate battery was incomplete: fresh-install reported no artifact, native ci-pass failed with an ENOSPC/toolchain-download error, `chaos-disconnect=UNVERIFIED` because ci-pass failed first, `docker-deploy=N/A`, `ui-probe=INFO` because this is a CLI, `chaos-corruption=N/A`, `chaos-resource=PASS`, and `chaos-errorpath=OK`. As an additive close-out arm, the documented README release path was then run literally on fresh agent ed04d531: SHA256SUMS verified, boardctl version v0.1.11, init, create, and stats --json all succeeded, and install-to-smoke time was 1.198s. The agent was destroyed and bunker info ed04d531 returned not_found. This proves the published release install path, but not a clean HEAD build on the fresh box; the battery HEAD-install result remains unverified.

## Findings

No new product defect was isolated in the local real-use path. The install battery's native failure is currently classified as unverified/harness-or-toolchain evidence because its detail is a Go module download line and no agent-side native log was available after collect; it is not filed as a boardctl product row. Existing open rows were not duplicated.

## Residue

Scratch root: `/var/tmp/hermes-scratch/boardctl-dogfood-027j5icy` (product scratch board and captured local-run material). The detached worktree was removed. The fresh bunker agent was destroyed and absent from the final bunker census.

ch:trace row=none:dogfood-no-new-finding spec=README.md#usage+skills/boardctl-usage/SKILL.md#proven-commands wave=none:serial test=manual-cli-real-use+hyperfine+fresh-bunker-battery doc=docs/dogfood/2026-10-26-run26-integration.md evidence=/var/tmp/hermes-scratch/boardctl-bunker-evidence-20261026-1159.jsonl witness=11ac9b0+captured-command-output verdict=PROMISING-BUT-ROUGH commit=none:read-only-lane memory=none:not-written
