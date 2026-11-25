# README-016 closeout evidence — both `make install` arms executed in a published-tip archive

Tick: boardctl-foreman-2026-11-25-15-43-56
Tested tree: origin/main == HEAD 40f52f2c7d04e825fad9b4c33f7d4621058581b0 (0 ahead, 0 behind after fetch)
Published-tip archive: `git archive origin/main | tar -x -C /home/kara/.hermes/cache/scratch/readme016-20261125` (no .git; fixture left in scratch for inspection)
Fresh CLI built from control checkout before any board mutation: `go build -o bin/boardctl ./cmd/boardctl` → BUILD_OK.

## Premise state at test time

The from-source fallback is GONE at HEAD: RELEASE-2026-11-16-02 landed fdd8234 ("make install refuses
without staged release assets - no from-HEAD fallback"); Makefile `install` delegates to `install-release`
whose no-assets guard exits non-zero. `grep -c 'building from source' Makefile` = 0.
README.md:47-52 and README.md:2088-2091 already state the refusal (verified by the COS re-probe
recorded in commit 40f52f2). Criterion 1 is met via its "Makefile change has landed" arm.

## Arm 2 — no dist/ (refusal arm)

Command (run from the archive root, dist/ absent — verified NO_DIST):

    make install INSTALL_DIR=/home/kara/.hermes/cache/scratch/readme016-20261125/install-no-dist

Output (verbatim):

    install: dist/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first:
      curl -sSL -o dist/boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/boardctl_linux_amd64
      curl -sSL -o dist/SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/SHA256SUMS
      make install
    install: refusing — never a from-HEAD build (a checkout build reports dev / vcs.modified=true and fails install-check; BT-042)
    make[1]: *** [Makefile:288: install-release] Error 1

ARM2_RC=2. `ls install-no-dist` → NO_BINARY_WRITTEN. No source build ran.

## Arm 1 — dist/ staged with the published v0.6.0 assets (install arm)

Staged from the release URL documented at README.md:20-21:

    curl -sSL -o dist/boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/v0.6.0/boardctl_linux_amd64
    curl -sSL -o dist/SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/v0.6.0/SHA256SUMS
    cd dist && sha256sum -c --ignore-missing SHA256SUMS   → boardctl_linux_amd64: OK, rc=0

Command (archive root):

    make install INSTALL_DIR=/home/kara/.hermes/cache/scratch/readme016-20261125/install-dist

Output (verbatim):

    == install: verifying dist/boardctl_linux_amd64 against dist/SHA256SUMS
    boardctl_linux_amd64: OK
    == install: dist/boardctl_linux_amd64 verified, installing to …/readme016-20261125/install-dist/boardctl
    install: … installed; verify the deployed identity with 'make install-check'

ARM1_RC=0. The INSTALL_DIR retarget documented in the same README paragraph works (criterion 4).
Installed identity: `/home/kara/.hermes/cache/scratch/readme016-20261125/install-dist/boardctl version`
→ `boardctl version v0.6.0` (the checksum-verified RELEASED asset, not a checkout build).

## Criterion 5 — version-check

    make version-check   (published-tip archive)  → TestVersioncheck ok, ARCHIVE_VC_RC=0
    make version-check   (control checkout HEAD)  → TestVersioncheck ok, CONTROL_VC_RC=0

## Verdict against the acceptance criteria

1. README describes both arms AND the refusal landed in the Makefile (fdd8234), quoted above — MET.
2. Both arms executed in a published-tip archive of origin/main; commands, rcs and `boardctl version`
   outputs pasted in this artifact and the row summary — MET.
3. No unqualified guarantee remains: README's "never builds from the checkout" now matches the live
   refusal (arm 2). — MET.
4. `make install INSTALL_DIR=<scratch>` works in the dist-staged arm — MET (ARM1_RC=0).
5. `make version-check` exits 0 on the edited tree — MET (rc=0 both surfaces; the README text needed
   no edit, so the tree under test is the published tip itself).
