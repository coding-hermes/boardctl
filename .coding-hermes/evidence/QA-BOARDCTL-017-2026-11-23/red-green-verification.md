# QA-BOARDCTL-017 — make install error-suppression: red/green live verification

Tick: boardctl-foreman-2026-11-23-03-42-22
Row: QA-BOARDCTL-017 (P2) — "Makefile install: recipe ';' separators swallow the from-source
build failure — install copies a STALE pre-existing bin/boardctl, prints 'installed from
source', and exits 0 when 'make build' inside the recipe errored"

## Verdict: defect CONFIRMED at the measured tree, ALREADY FIXED at current HEAD

- Measured at f088ec1 (`Makefile` from-source fallback inside one `@if` recipe, no
  `set -e`): reproduced exactly — a failed sub-make did not propagate; the recipe
  continued, copied the stale `bin/boardctl` built at f088ec1, printed
  "installed from source", and exited rc=0.
- Fixed at HEAD by commit fdd8234 ("fix: make install refuses without staged release
  assets - no from-HEAD fallback (RELEASE-2026-11-16-02)"): install no longer has a
  from-source fallback at all. Assets absent -> loud staging-economics refusal, rc
  nonzero, nothing installed.

## Acceptance criterion (row): "make install in a checkout with no dist/ assets and a
## failing from-source build (e.g. toolchain unavailable) must exit nonzero or install nothing"

GREEN proof at judged tree (HEAD ff898e4 at time of run, DIST overridden to an empty
directory because the control checkout carries staged release assets in dist/; the
Makefile from-source path no longer exists, so a failing build is additionally poisoned
via GOFLAGS=-invalidflag):

    make install DIST=<empty> INSTALL_DIR=<scratch>/green-out INSTALL_BIN=<scratch>/green-out/boardctl
    -> rc=2, "install: refusing — never a from-HEAD build (a checkout build reports dev /
       vcs.modified=true and fails install-check; BT-042)"
    -> <scratch>/green-out does not exist (nothing installed)
    -> GREEN_ASSERT_rc_nonzero=0, GREEN_ASSERT_no_install=0

RED proof at f088ec1 (detached worktree, stale `bin/boardctl` pre-built, from-source build
forced to fail deterministically via GOFLAGS=-invalidflag):

    env PATH=/usr/bin:/bin GOFLAGS=-invalidflag make install INSTALL_DIR=<scratch>/red-out2 INSTALL_BIN=<scratch>/red-out2/boardctl
    -> "go: parsing $GOFLAGS: unknown flag -invalidflag"
    -> make[1]: *** [Makefile:44: build] Error 1
    -> recipe CONTINUES: "install: ... installed from source;" rc=0
    -> stale binary present at INSTALL_BIN (12,426,599 bytes, "boardctl version dev")

Both full transcripts follow. (Red v1 used PATH=/usr/bin:/bin alone; /usr/bin/go resolved,
so the build did not actually fail — v2 is the authoritative red run. One stray header-echo
error line from the tick shell is noted, not part of the make run.)

## Disposition

Row closes as SATISFIED BY LANDED FIX: the acceptance criterion holds at the current
canonical tree (commit fdd8234 and successors through the judged tree). No new product
change is required by this row.

---

### Transcript: green (judged tree, HEAD ff898e4)

=== GREEN PROOF: QA-BOARDCTL-017 at HEAD ff898e4 (current Makefile) ===
repo carries staged release assets in dist/ (RELEASE-2026-11-16-02), so the no-assets
scenario is simulated via DIST override to an empty directory; from-source build
additionally poisoned via GOFLAGS=-invalidflag (there is no from-source path left to fail)
--- begin transcript
make[1]: Entering directory '/home/kara/coding-hermes-boardctl'
install: /home/kara/.hermes/cache/scratch/bt017/empty-dist/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first:
  curl -sSL -o /home/kara/.hermes/cache/scratch/bt017/empty-dist/boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/boardctl_linux_amd64
  curl -sSL -o /home/kara/.hermes/cache/scratch/bt017/empty-dist/SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/SHA256SUMS
  make install
install: refusing — never a from-HEAD build (a checkout build reports dev / vcs.modified=true and fails install-check; BT-042)
make[1]: *** [Makefile:288: install-release] Error 1
make[1]: Leaving directory '/home/kara/coding-hermes-boardctl'
make: *** [Makefile:282: install] Error 2
--- rc=2
--- installed artifact check (must be empty/absent):
ls: cannot access '/home/kara/.hermes/cache/scratch/bt017/green-out': No such file or directory
GREEN_ASSERT_rc_nonzero=0
GREEN_ASSERT_no_install=0

### Transcript: red v2 (f088ec1, failing from-source build)

=== RED PROOF v2: QA-BOARDCTL-017 at f088ec1 — from-source build FORCED to fail (GOFLAGS=-invalidflag) ===
run: env PATH=/usr/bin:/bin GOFLAGS=-invalidflag make install INSTALL_DIR=<scratch>/red-out2 INSTALL_BIN=<scratch>/red-out2/boardctl
--- begin transcript
== install: no release assets found; building from source
make[1]: Entering directory '/home/kara/.hermes/cache/scratch/bt017/red-f088ec1'
go build -o bin/boardctl \
	-ldflags "-X main.buildCommit=f088ec1034b8aeadf8bcecbe2e623c1cd82f5898" ./cmd/boardctl
go: parsing $GOFLAGS: unknown flag -invalidflag
make[1]: *** [Makefile:44: build] Error 1
make[1]: Leaving directory '/home/kara/.hermes/cache/scratch/bt017/red-f088ec1'
install: /home/kara/.hermes/cache/scratch/bt017/red-out2/boardctl installed from source; verify the deployed identity with 'make install-check'
--- rc=0
--- installed artifact check:
total 12144
drwxrwxr-x 2 kara kara     4096 Nov 22 22:44 .
drwxrwxr-x 5 kara kara     4096 Nov 22 22:44 ..
-rwxr-xr-x 1 kara kara 12426599 Nov 22 22:44 boardctl
boardctl version dev

### Fix attribution

f088ec1 (measured, defective):
    install:
    	@if [ ! -f "$(DIST)/SHA256SUMS" ]; then \
    		 echo "== install: no release assets found; building from source"; \
    		 $(MAKE) build; \
    		 mkdir -p "$(INSTALL_DIR)"; \
    		 cp "bin/$(BINARY)" "$(INSTALL_BIN)"; ... rc of last echo = 0

fdd8234 (fix, RELEASE-2026-11-16-02):
    install:      @$(MAKE) install-release        (unconditional; no ';' continuation)
    install-refusal: [ ! -f "$(DIST)/SHA256SUMS" ] -> exit 1 before any sub-make
    checksum gate propagates rc nonzero explicitly.
