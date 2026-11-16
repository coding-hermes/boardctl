# boardctl releng — fire B of tick boardctl-releng-2026-11-16-13-20-45

Measured 2026-11-16T14:28Z–14:5xZ UTC from the canonical checkout
`/home/kara/coding-hermes-boardctl` (verify-only lane; no product code touched,
no tag created, no GitHub Release published).

Decision: NO CUT, unchanged. This artifact does not supersede
`releng-boardctl-2026-11-16-13-20-45.md`; it re-verifies that the release
premise has not moved and files one structural finding that artifact did not
own.

## Duplicate-fire disclosure

This is the second dispatch of scheduler tick `boardctl-releng-2026-11-16-13-20-45`.
The first fire is already committed at `e77283bc`:

- artifact `.coding-hermes/board/evidence/releng-boardctl-2026-11-16-13-20-45.md`
- row `RELEASE-2026-11-16-01` (status `pending`, P1 NO-CUT) carries that fire's
  `run_id` with `ts 2026-11-16T13:39:58Z`.

So no second NO-CUT row was minted: `boardctl create` refuses a re-detected
finding, and a duplicate id would be a board-lint error rather than a signal.
The one row filed here (`RELEASE-2026-11-16-02`) covers material that the first
fire deliberately deferred, with a fresh measurement.

## Premise re-verification (nothing moved)

| Probe | Command | Result |
| --- | --- | --- |
| Ref parity | `git status --short --branch` | `## main...origin/main`, worktree clean, 0 ahead / 0 behind |
| Head | `git rev-parse HEAD` | `e77283bcf6754ac720967f1c08c6b50e499a3078` |
| Head subject | `git log -1 --format=%s` | `board: file RELEASE-2026-11-16-01 — v0.6.0 readiness sweep is NO-CUT [ci skip]` |
| Product tree vs first-fire candidate | `git diff --quiet 20124caf..HEAD -- cmd/ internal/ Makefile go.mod go.sum vendor/ .github/` | exit 0 — **IDENTICAL**; only `.coding-hermes` board files differ |
| Delta since tag | `git rev-list --no-merges --count v0.5.0..HEAD` | 25 non-merge commits (24 product + 1 board-only) |
| Prefix census | sed prefix attribution over `v0.5.0..HEAD` | board 12, chore 8, feat 2, test 1, fix 1, docs 1 |
| Breaking markers | footer `BREAKING CHANGE` / bang-subject grep | 0 |
| Material product delta | `git diff --stat 3acca08..HEAD -- cmd internal Makefile go.mod go.sum .github` | 24 files changed, +2171 / −24 |

Feature content still present and unchanged: `269add6 feat: typed board rows
with templates and required metadata (BT-098)`, `baf6348 feat: add initiative
cross-repo refs (BT-092)`, `53e22fc fix: persist typed row discriminator
(BT-098)`. Semver surface therefore stays **v0.6.0 (minor)**; no major
confirmation is required because no breaking marker is measurable.

## CI truth

| Scope | Command | Result |
| --- | --- | --- |
| Check-runs on HEAD | `gh api repos/coding-hermes/boardctl/commits/e77283bc.../check-runs` | `total_count: 0` |
| Workflow runs on HEAD | `gh api 'repos/coding-hermes/boardctl/actions/runs?head_sha=e77283bc...'` | `total_count: 0` |

Zero runs on HEAD is by design, not a failure: the head commit is board-only and
carries `[ci skip]`. The newest commit that carries CI is `20124caf`, whose
product tree is byte-identical to HEAD (row 4 above), and it is green — CI run
`43810100006` success, Multi-Arch run `43810101324` success, both attempt 1 on
`20124caf`. No assertion is made that CI ran on `e77283bc`; it did not, and no
such claim appears in this artifact or the filed row.

## Release tooling present (Bane doctrine satisfied)

The repo ships its own release path; `gh release create` remains unused.

- `Makefile` `release` target cross-compiles the declared platforms and writes
  `SHA256SUMS`; the `.github/workflows/multiarch.yml` tag-push publisher
  publishes the same asset set on a `v*` tag.
- `v0.5.0` remains the only relevant published artifact: non-draft,
  non-prerelease, 8 assets (7 platform binaries + `SHA256SUMS`), peeled commit
  `3acca082120e44f8a9dc699189739a14d2dac2ff`. Not touched by this fire.

## Version hygiene (freshly measured this fire)

`make version-check VERSION_TAG=v0.6.0` → **exit 2**:

```
--- FAIL: TestVersioncheckExpectedTag (0.00s)
    repo README is stale vs the expected release v0.6.0:
      stale release pin line #1: says "v0.5.0", expected "v0.6.0"
      stale download URL #1: pins "v0.5.0", expected "v0.6.0"
      stale download URL #2: pins "v0.5.0", expected "v0.6.0"
```

Control: `make version-check VERSION_TAG=v0.5.0` → exit 0. The gate itself is
sound; only the surfaces are stale. CHANGELOG.md's newest section is
`## v0.5.0 — 2026-11-15`, so the v0.6.0 notes section does not exist yet.
This work stays owned by `RELEASE-2026-11-16-01` criterion 1.

## Deployed-binary staleness (freshly measured this fire)

`make install-check RELEASE_TAG=v0.5.0` → **exit 2** (DRIFT):

```
install-check: FAIL version: '/home/kara/.local/bin/boardctl version' prints 'dev' but the release is v0.5.0
install-check: FAIL module version: go version -m reports 'v0.5.1-0.20261115213304-2585c2701b1e' but the release is v0.5.0
install-check: ok vcs.modified=false
install-check: FAIL vcs.revision: 2585c2701b1e8f16d50da08073c87f5551145bb1 (test: typed-row test plan and unknown-metadata warning (BT-098), committed 2026-11-15) is NOT an ancestor of v0.5.0 — the binary was built from an unrelated commit
install-check: DRIFT detected — deployed /home/kara/.local/bin/boardctl does not match release v0.5.0
```

`/home/kara/go/bin/boardctl` (secondary copy) is clean: `v0.5.0`,
`vcs.revision=3acca08...`, `vcs.modified=false`. The PATH-resolved copy is the
drifting one.

Recurrence timing matters: `RELEASE-2026-11-15-02` was closed `complete` at
`2026-11-15T22:34:17Z` asserting the copies were reconciled, and the drifting
binary's mtime is `2026-11-16T01:45Z` — i.e. it was rebuilt from source **after**
that closeout. Five separate DEPLOY DRIFT rows have now been opened and closed
on this same artifact (`RELEASE-2026-09-23-05`, `-09-25-03`, `-09-27-01`,
`-11-09-04`, `-11-15-02`), which is the signature of a structural cause rather
than repeated operator error.

## Structural cause filed this fire

`Makefile` (BT-042 block) states the invariant in its own comment —

> `install:` install the RELEASED binary … **NEVER a from-HEAD build**: a
> checkout build reports `dev` (or a date) and `vcs.modified=true`, which is
> exactly the deployed-binary drift this target exists to prevent.

— and the recipe's first branch then does precisely that:

```
install:
	@if [ ! -f "$(DIST)/SHA256SUMS" ]; then
		echo "== install: no release assets found; building from source";
		$(MAKE) build; ... cp bin/boardctl $(INSTALL_BIN) ...
```

so the documented remediation path (`fix: make install`, which is literally what
`install-check` tells the operator to run) installs a from-source binary whenever
`dist/` is not staged, and `install-check` then fails with "an unrelated commit"
for exactly that binary. The remediation and the gate disagree, so each deploy
reconciliation closes by reinstalling drift. Filed as `RELEASE-2026-11-16-02`
with `install-check` cleanliness as its acceptance gate, not another reinstall.

## What was NOT done (release safety)

- No tag created; `v0.6.0` does not exist locally or on origin.
- `v0.5.0` tag and Release untouched — no force-move, no re-upload.
- No release artifacts built; no `dist/` staged; no binaries published.
- No product code, docs, or version surfaces edited (verify-only lane).
- `RELEASE-2026-11-16-01` not rewritten; its acceptance criteria still gate the cut.
