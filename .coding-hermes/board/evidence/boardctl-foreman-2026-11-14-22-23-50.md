# Boardctl release-row verification evidence

Tick: boardctl-foreman-2026-11-14-22-23-50
Repository: https://github.com/coding-hermes/boardctl
Selected row: RELEASE-2026-11-09-07
Mode: verification-only landed-drift reconciliation; no worker dispatched.

## Scope and decision

The row's historical v0.4.0-due and exact-head-red premises are superseded by the published v0.4.0 release and later green release evidence. This pass does not create, move, or delete a tag or Release and does not mutate the board row. A commit-bound GitReins verification is required before a row closeout can be attempted.

## Fresh repository evidence at 84d50024fbc3b83766c075ed621859d3e60793e6

The dedicated integration worktree was clean and matched origin/main before verification. `go build -o bin/boardctl ./cmd/boardctl` passed. `make test` passed all listed Go packages. `make vet` passed. `make vuln-check` passed `TestVulncheck`. `make cover-check` passed with 80.2% total coverage against the unchanged 80.0% floor and 66.5% lowest package coverage against the 60.0% floor, evaluating 15 packages. `make version-check VERSION_TAG=v0.4.0` passed.

The product-path diff from v0.4.0 to this published tip contains six repository changes outside the release-excluded bookkeeping/docs paths; this is why the current verification is bound to the exact published tip rather than claiming the release baseline itself is unchanged. No tag or Release mutation was performed by this tick.

Recent GitHub Actions listing at the control checkout showed terminal success for the current published tip 84d50024fbc3b83766c075ed621859d3e60793e6 and its predecessor 687969df3eed8faeaa9ce672dcd729871bb36323. No recent failure was present in the required three-run listing.

## Prior evidence classification

The prior row evidence naming `.coding-hermes/board/evidence/releng-boardctl-2026-11-13-11-13-57.md` records the v0.4.0 tag, public Release, seven platform binaries plus SHA256SUMS, and green CI anchors 43426472508 and 43426473385. The older `.gitreins/history/2026-11-11/8907c118/verdict.json` is not candidate-bound to this tip and is overall failed on inherited Tier-1 audit-transcript secret findings despite Tier 2 passing; it is retained as historical evidence, not promoted.

## Required disposition

This evidence commit is a verification artifact only. It does not close the still-pending board row. The separate GitReins task RELEASE-2026-11-09-07-V-20261114 is the commit-bound verification lifecycle for this artifact.

ch:trace row=RELEASE-2026-11-09-07 spec=.coding-hermes/board/tasks.jsonl#RELEASE-2026-11-09-07 wave=.coding-hermes/waves/boardctl-foreman-2026-11-14-22-23-50.json#selected test=make test; make vet; make vuln-check; make cover-check; make version-check VERSION_TAG=v0.4.0 doc=.coding-hermes/board/evidence/boardctl-foreman-2026-11-14-22-23-50.md evidence=.coding-hermes/board/evidence/boardctl-foreman-2026-11-14-22-23-50.md witness=none:release-reconciliation-is-repo-and-ci-bound verdict=none:verification-task-pending commit=84d50024fbc3b83766c075ed621859d3e60793e6 memory=none:not-written
The first verification verdict is intentionally preserved as FAIL: its criterion was false because seven product files differ from v0.4.0. This correction records the actual evidence commit and verdict, not a new pass.
