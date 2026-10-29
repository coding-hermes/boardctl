# boardctl dogfood — run 27

verdict=PROMISING-BUT-ROUGH

run date: 2026-10-29
tick: boardctl-dogfood-2026-10-29-14-26-36
promise: A real user can install boardctl, initialize a JSONL foreman board, create and complete work with evidence, inspect the audit trail, validate/diagnose the board, and use the CLI interactively; a fresh machine should be able to install the current checkout and survive the documented QA battery.

## Target and identity

- Scheduler target: boardctl-dogfood; canonical checkout: `/home/kara/coding-hermes-boardctl`; stand-in: `/home/kara/.hermes/stand-in/dogfood/boardctl`; canonical board: `/home/kara/coding-hermes-boardctl/.coding-hermes/board`.
- Target HEAD and origin/main: `7ef46b37206e031c89cc44790ed143a989a5cf84`; parity was 0 ahead / 0 behind.
- The scheduler row's stored workdir resolves to the boardctl stand-in and its repo URL is `local:/home/kara/coding-hermes-boardctl`. The lane name and target agree; no sibling target was used.
- The checkout had pre-existing unrelated residue (`cmd/boardctl/board-report.html` modified and `.gitreins` history/log files); no source or board files were changed by this run.

## Real user path

Built a fresh binary from a clean file clone at the target HEAD (`/tmp/boardctl-dogfood-20261029/boardctl`) and drove a scratch board at `/tmp/boardctl-dogfood-user`:

- `version --json`: rc=0, `{"version":"dev","build":"dev"}`.
- `init --project dogfood-user`: rc=0; created tasks/events/board/fixtures JSONL files.
- `create` with `--acceptance-criteria`: rc=0; acceptance metadata persisted.
- Second `create`: rc=0 for a distinct task; `list --status pending`: rc=0 and showed both tasks.
- `update ... --status complete ... --evidence-run-id dogfood-2026-10-29`: rc=0; `show --events` exposed the stored evidence run and completion event.
- `event --type audit --task-id ... --actor operator --detail-text ... --tick 29`: rc=0; header tick counter advanced to 29.
- `stats --json`: rc=0, total=2, complete=1, pending=1, actionable=1.
- `validate`: rc=0, 3 task records including the fixture, 4 events, `key uniformity: 3/3`, 0 warnings.
- `doctor`: rc=0; 2 expected warnings only (scratch is outside git and the complete row is fully evidence-backed).

The local end-to-end CLI path is usable and trustworthy for a clean scratch board. The fresh binary was built from the target commit, not the stale `/home/kara/.local/bin/boardctl` (which reports a nine-commit freshness warning).

## Performance

Headline operation: `boardctl stats --json` on the 3-record scratch board, optimized Go binary built from HEAD. Timed inside one Python process with `time.monotonic()` around each subprocess:

- cold first process: 7.693 ms, rc=0;
- warm p50: 7.512 ms over 10 runs, rc=0;
- warm min/max: 4.786 / 25.891 ms.

The single 25.9 ms outlier is host noise; the typical cost is single-digit milliseconds. Nothing here is user-noticeable, so no PERF row and no profile were warranted.

## Fresh-machine install leg

Executed the prescribed `bash ~/.hermes/scripts/bunker-qa.sh launch /home/kara/coding-hermes-boardctl` then collected the exact evidence path `/tmp/boardctl-dogfood-20261029-bunker.jsonl` on `bunker-mvp`, agent `7274a462`. The battery collected 20 agent evidence rows plus the collect row; the run-owned agent was destroyed, and `bunker info 7274a462` returned `not_found` after collection. The final server census had no `7274a462` residue.

Per-cell result:

- repo-head: OK, exact HEAD `7ef46b3...`.
- fresh-install: OK, source-build install rc=0; deployed `~/.local/bin/boardctl`; no standalone artifact was expected.
- ci-pass: OK, act reported 1 triggerable workflow and 1 green job, rc=0.
- chaos-resource: PASS under the 3G cap.
- chaos-errorpath: OK, clean missing-config rc=2.
- upgrade: FAIL, the prior-release path lacks `dist/SHA256SUMS` and exits at `Makefile:201`; this is the existing `QA-BOARDCTL-FOREMAN-4` row, not a new row.
- chaos-disconnect: FAIL because the network-cut suite attempts to download `gopkg.in/yaml.v3 v3.0.1`; this is the existing `QA-BOARDCTL-FOREMAN-5` row, not a new row.
- ui-probe: INFO because boardctl is a CLI/API surface rather than an HTML UI; this is the existing `QA-BOARDCTL-FOREMAN-6` row, not a new row.
- docker-deploy, chaos-shutdown, chaos-corruption: N/A for this repository's declared surface.

This is not a silent install pass: the install cell passed, but the aggregate battery verdict is PROMISING-BUT-ROUGH because upgrade and network-cut cells failed. The evidence is retained at `/tmp/boardctl-dogfood-20261029-bunker.jsonl` for this run; the source of record is the committed report.

## Findings and handoff

No new board row was minted or appended: this lane is read-only and the three actionable battery findings already have owner rows QA-BOARDCTL-FOREMAN-4, QA-BOARDCTL-FOREMAN-5, and QA-BOARDCTL-FOREMAN-6. The local product path produced no additional defect. Existing open work remains visible in the canonical board; this run does not close, update, or duplicate it.

## Artifacts and boundaries

- Report: `docs/dogfood/2026-10-29-run27-integration.md`.
- Diagnostics addendum: `docs/dogfood/diagnostics.md`, Run-27 section.
- Usage skill addendum: `skills/boardctl-usage/SKILL.md`, Run-27 section.
- Canonical log: `.coding-hermes/dogfood-log.md`, one entry for this exact tick.
- Raw local probe output: `/tmp/boardctl-dogfood-probe.out`.
- Raw bunker evidence: `/tmp/boardctl-dogfood-20261029-bunker.jsonl`.
- No product source, board task, board event, scheduler state, repository visibility, permission, or credential was changed.

ch:trace row=none:dogfood-no-new-finding spec=README.md#usage+skills/boardctl-usage/SKILL.md#proven-commands wave=none:serial test=manual-cli-real-use+source-build+timed-stats+bunker-qa-launch-collect doc=docs/dogfood/2026-10-29-run27-integration.md evidence=/tmp/boardctl-dogfood-20261029-bunker.jsonl witness=live:bunker-mvp/agent-7274a462+scratch-cli verdict=PROMISING-BUT-ROUGH commit=pending:owned-artifacts-only memory=none:not-written
