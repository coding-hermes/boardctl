# Run 16 — the release path, replayed from the releaser's seat (2026-09-27)

**Lane:** boardctl-releng (stand-in workdir) · **Repo HEAD under test:** `a3a1dea`
**Previous runs:** 2026-09-04, 09-12, 09-20, 09-22, 09-24 (run 15) — none replayed the
release-cut procedure itself. That is this run's angle.

## Promise under test

> "Cutting a release is one command: push a `vX.Y.Z` tag" — with the offline
> `make release` equivalent producing byte-compatible assets, and every step
> verifiable by a gate that catches drift.

## What I actually did (all numbers real)

| Step (README "Cutting a release") | Command | Result |
|---|---|---|
| 1. version gate, full move | `sed v0.1.8→v0.1.9 README.md && make version-check` | green, 0.23s |
| 1. version gate, partial move | revert one URL to v0.1.8, re-run | **FAIL exit 1**, names "URL #1 pins v0.1.8 but the release pin line says v0.1.9" + fix |
| 2. offline cut (rehearsal, tagged form) | `make release` from tagged checkout | rc=0, **75s**, 7 assets + SHA256SUMS |
| 2. offline cut (candidate form) | `make release VERSION=v0.1.9` | rc=0, **69s**, `version` prints v0.1.9 |
| 3. artifact identity | `./dist/boardctl_linux_amd64 version` | stamped correctly (tag string) |
| 4. install (checksum gate) | `make install DIST=… INSTALL_DIR=…` | refuses wrong-named asset exit 2; installs verified asset |
| 4. drift probe | `make install-check RELEASE_TAG=v0.1.8` | **FAIL exit 2** on the offline artifact — see finding 1 |
| ephemeral bunker | README quickstart verbatim on las-bunker-03 | **6s**, checksum OK, v0.1.8, full board smoke green (init→create→update→validate→stats) |

Scratch everything: builds ran in a throwaway worktree
(`~/dogfood-wt-boardctl`, branch `dogfood/run16-release-replay`), installs into
`/tmp` dirs. No tag was pushed; nothing was published; the real deploy at
`~/.local/bin` was not touched.

## The big finding: the offline cut stamps a FOREIGN repo's commit

`go version -m` on my freshly-cut "v0.1.9" assets reported:

```
vcs.revision=c6af73f1b4da97a112e634be98285598961fed26
vcs.modified=true
```

`c6af73f` is **not a boardctl commit** — it is the HEAD of `/home/kara/.git`,
an unrelated repo that lives at the HOME level. Reproduced three times,
including in the older sibling worktree `~/worktrees/boardctl-BT-RELEASE-0902`;
a build in the main checkout stamps correctly (`a3a1dea`). CI's shipped v0.1.8
asset stamps clean (tag commit, `modified=false`), so published releases are
unaffected — this is a local-host trap.

**Mechanism (proven against the go1.26.6 toolchain source,
`cmd/go/internal/vcs/vcs.go`):** `vcsGit.RootNames` matches only a `.git`
*directory*. A linked worktree's `.git` *file* does not stop `FromDir`'s parent
walk (srcRoot empty → walks to `/`), so any build path nested under another
git repo resolves VCS status from that outer repo — HEAD, dirty flag, and all.
The trigger on this box: the home repo's HEAD moved to `c6af73f` at 11:20
today; yesterday the same worktree stamped the correct boardctl SHA.

**Why the checksum gate didn't save you:** SHA256SUMS verifies what you built,
not that you built the right thing; `version` prints the tag string you passed.
The only probe that caught it is the repo's own `make install-check` — 3/4
arms FAIL (module `(devel)`, `vcs.modified=true`, revision-not-in-history
DEGRADED notice), exit 2, honest output. Filed **DF-BOARDCTL-16**.

## Secondary findings

- **DF-BOARDCTL-17:** `version-check` validates *internal README agreement*
  only — nothing ties the README to the *candidate* tag. Skip step 1's README
  edit and every gate stays green while you cut a release whose install URLs
  fetch the previous tag. (Drift is caught only *after* the fact.)
- **DF-BOARDCTL-18:** `sha256sum -c --ignore-missing` exits **0** when zero
  files matched ("no file was verified"). `make install` is sound today only
  because the exact-name pre-check runs first — an ordering invariant nothing
  documents. Calibrates run 15's DF-BOARDCTL-12 (end-to-end refusal held).

## Verified clean (evidence, not vibes)

- version-check: fails loud and precise on real drift (message names the
  surface, the conflicting tag, and the fix).
- `make release` refuses a non-`vX.Y.Z` VERSION on a tagged checkout (the
  "date-stamp would ship" guard); `rm -rf dist/` prevents stale-asset mixing.
- install: wrong-named asset refused exit 2; matching SUMS → verified install.
- install-check: 4-arm probe incl. ancestry; DEGRADED notice is honest about
  what it could not check offline.
- Bunker leg: zero-dep install path works on a bare Debian user (no Go, no
  sudo) in 6s; smoke board lifecycle green on the v0.1.8 release binary.
- Non-features re-checked: `help` exits 0 (BT-041 ✓), `install`/`sweep-status`
  in `--help` (DF-14 ✓). `stats` excludes `init`'s NEVER-DONE seed row while
  `validate` counts it — semantics, not a bug (checked, not filed).

## What a releaser should take from this

1. Never trust `version`'s tag string alone: after ANY offline cut, run
   `make install-check RELEASE_TAG=<tag>` against the staged asset. It is the
   only gate that reads build provenance.
2. Cut from CI when possible — it provably stamps clean — or from a checkout
   with no other `.git` directory anywhere above it (main checkouts are safe;
   linked worktrees under a nested-repo path are not, on go ≥ 1.18 forever
   until RootNames learns `.git` files).
3. `make release VERSION=vX.Y.Z` (explicit tag) is the correct rehearsal form
   from a tagged checkout; the bare form resolves `git describe` and would
   stamp the OLD tag onto new assets.
