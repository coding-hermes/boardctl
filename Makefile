BINARY := boardctl
CMD := ./cmd/boardctl
# BT-030: stamp the RELEASE IDENTITY (the newest tag) when the checkout has
# one, so `make release` from a tagged checkout ships binaries whose
# `version` names the release. An untagged checkout falls back to a UTC date
# stamp. An explicit override still wins: `make release VERSION=v9.9.9`.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || date -u +%Y%m%d)
DIST := dist

PLATFORMS := \
	linux/amd64 linux/arm64 linux/arm \
	darwin/amd64 darwin/arm64 \
	windows/amd64 \
	freebsd/amd64

.PHONY: build test vet fmt fmt-check version-check vuln-check release install install-check clean

# BT-057: the build commit stamped into every make-built binary. The guard
# keeps non-git builds working (a tarball without .git yields an empty
# stamp and the freshness gate stays silent); inside a checkout it is HEAD.
# main.buildCommit feeds internal/freshness.Check — see README Development.
BUILD_COMMIT := $(shell git rev-parse HEAD 2>/dev/null)

# BT-042: install surface. `make install` installs the RELEASED asset (the
# dist/ download staged per README "## Install") — never a from-HEAD build;
# `make install-check` is the standing drift probe for the deployed copy.
# INSTALL_DIR/INSTALL_BIN are overridable for non-default deploy targets.
INSTALL_DIR ?= $(HOME)/.local/bin
INSTALL_BIN ?= $(INSTALL_DIR)/$(BINARY)
# ASSET resolves to the local platform so the default name is the one the
# release publishes on this machine (boardctl_linux_amd64, .exe on windows).
ASSET_OS := $(shell go env GOOS 2>/dev/null || echo linux)
ASSET_ARCH := $(shell go env GOARCH 2>/dev/null || echo amd64)
ASSET_EXT := $(if $(filter windows,$(ASSET_OS)),.exe,)
ASSET := $(BINARY)_$(ASSET_OS)_$(ASSET_ARCH)$(ASSET_EXT)
# Expected release identity for install-check: the newest tag (same
# resolution as VERSION). Pin a specific release explicitly:
# `make install-check RELEASE_TAG=v0.1.8`. Empty on an untagged checkout —
# the target then fails loudly instead of guessing.
RELEASE_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null)

build:
	go build -o bin/$(BINARY) \
		-ldflags "-X main.buildCommit=$(BUILD_COMMIT)" $(CMD)

test:
	go test ./...

# Check-only gofmt gate (never rewrites files). It runs the Go checker in
# internal/fmtcheck, so the Makefile, CI, and `go test ./...` all enforce the
# exact same rule over the exact same file set. The writing helper is `fmt`.
fmt:
	gofmt -w $(CMD) ./internal

fmt-check:
	go test -count=1 -run TestGofmt ./internal/fmtcheck

# BT-030: check-only README release-pin gate (never rewrites files). It runs
# the Go checker in internal/versioncheck, so the Makefile, CI, and
# `go test ./...` all enforce the exact same rule over README.md's release
# surfaces (the "Current release:" line and every /releases/download/ URL).
# DF-BOARDCTL-17: pass the candidate tag to also enforce CURRENCY, not just
# agreement: `make version-check VERSION_TAG=vX.Y.Z` fails while any README
# surface still names an older tag. Unset, the gate stays agreement-only.
# (Sibling convention: install-check's RELEASE_TAG resolves the newest tag
# when unset; here there is deliberately no default — the checker stays
# offline, so the candidate tag is data in, never a tag lookup.)
VERSION_TAG ?=
export VERSION_TAG
version-check:
	go test -count=1 -run TestVersioncheck ./internal/versioncheck

# BT-033: dependency-vulnerability gate (check-only, never rewrites go.mod).
# It runs the Go checker in internal/vulncheck, which shells out to
# govulncheck and FAILS on any reachable finding (exit 3) or on a broken scan
# (any other non-zero exit). CI installs govulncheck@v1.7.0 and runs this
# byte-identical command; see internal/vulncheck for the full contract.
# Local runs need the tool: go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
vuln-check:
	go test -count=1 -run TestVulncheck ./internal/vulncheck

vet:
	go vet ./...

# BT-036: the asset names this target emits are the CI names — the org
# multi-arch workflow (.github/workflows/multiarch.yml) builds
# `${binary}_${goos}_${goarch}${ext}` and publishes dist/SHA256SUMS, so a
# `make release` cut and a tag-push cut are interchangeable byte-name-for-name.
# Changing PLATFORMS means changing the workflow's `platforms:` input too —
# this is enforced, not just commented: internal/workflowcheck fails
# `go test ./...` when the two lists drift (same platforms, same order).
#
# BT-030 guard: fail loudly instead of silently shipping a date-stamped
# release when the repo HAS tags — that is exactly the v0.1.3 bug this
# task fixes (published binaries said 20260915, not v0.1.3). Escaping the
# guard is a deliberate, explicit VERSION that names the tag.
release: test
	@if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && [ -n "$$(git tag)" ]; then \
		if ! printf '%s' '$(VERSION)' | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
			echo "versioncheck: refusing to release: VERSION '$(VERSION)' is not a vX.Y.Z tag but this repo HAS tags — a date-stamped binary would ship instead of the release identity" >&2; \
			echo "  fix: cut from the tagged checkout (make release) or pass an explicit tag: make release VERSION=<tag>" >&2; \
			exit 1; \
		fi; \
	fi
	@echo "== release VERSION=$(VERSION)"
	@rm -rf $(DIST)   # a stale dist/ would ship leftover names as extra assets
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "== $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION) -X main.buildCommit=$(BUILD_COMMIT)" \
			-o $(DIST)/$(BINARY)_$${os}_$${arch}$${ext} $(CMD) || exit 1; \
	done
	@cd $(DIST) && sha256sum $(BINARY)_* > SHA256SUMS
	@echo "release artifacts in $(DIST)/"

# BT-042: install the RELEASED binary — the dist/ asset + SHA256SUMS staged by
# README "## Install" (curl to <repo>/dist) or any release audit's dist/ — into
# $(INSTALL_BIN). NEVER a from-HEAD build: a checkout build reports `dev` (or a
# date) and vcs.modified=true, which is exactly the deployed-binary drift this
# target exists to prevent. Refuses loudly when dist/ is missing or the
# checksum check fails; installs only a verified asset.
#
#   curl -sSL -o boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/boardctl_linux_amd64
#   curl -sSL -o SHA256SUMS          https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/SHA256SUMS
#   mkdir -p dist && mv boardctl_linux_amd64 SHA256SUMS dist/ && make install
#
# (Recipe lines reach the shell once; `$$` is the recipe escape for the
# shell's own variables.)
#
# DF-BOARDCTL-18: the checksum gate does NOT trust `sha256sum`'s exit code
# alone. With --ignore-missing, a SHA256SUMS whose entries match NONE of the
# staged files ("SHA256SUMS: no file was verified") exited 0 on older
# coreutils, which would install an UNVERIFIED binary (the classic trigger:
# the release asset saved under a slightly wrong name). The gate therefore
# also requires at least one ": OK" verification line, so the soundness no
# longer depends on the coreutils version installed on the deploy host.
#
# ORDERING INVARIANT (load-bearing): the exact-asset-name pre-check below
# (the second `if [ ! -f "$(DIST)/$(ASSET)" ]`) must stay BEFORE/with the
# checksum gate — it is what turns a wrongly-named download into a precise
# "$(DIST)/$(ASSET) not found" failure before the checksum step runs. The
# zero-match gate then covers the case the pre-check cannot see (a file that
# exists under the right name but matches no SHA256SUMS entry). If the two
# checks are ever reordered or the gate is ported elsewhere, both halves must
# move together — the checksum step alone is not a complete gate.
install:
	@if [ ! -f "$(DIST)/SHA256SUMS" ]; then \
		echo "install: $(DIST)/SHA256SUMS not found — download the released asset + SHA256SUMS per README '## Install' first" >&2; \
		echo "  curl -sSL -o $(DIST)/boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/boardctl_linux_amd64" >&2; \
		echo "  curl -sSL -o $(DIST)/SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/vX.Y.Z/SHA256SUMS" >&2; \
		echo "  make install" >&2; \
		exit 1; \
	fi
	@if [ ! -f "$(DIST)/$(ASSET)" ]; then \
		echo "install: $(DIST)/$(ASSET) not found — download the asset for this platform ($(ASSET_OS)/$(ASSET_ARCH)) into $(DIST)/" >&2; \
		exit 1; \
	fi
	@echo "== install: verifying $(DIST)/$(ASSET) against $(DIST)/SHA256SUMS"
	@out=`cd $(DIST) && sha256sum -c --ignore-missing SHA256SUMS`; rc=$$?; \
	echo "$$out"; \
	if [ $$rc -ne 0 ] || ! printf '%s\n' "$$out" | grep -q ': OK$$'; then \
		echo "install: checksum gate FAILED (sha256sum exit $$rc): the checksum step verified ZERO files ('no file was verified') — nothing in $(DIST)/SHA256SUMS matches a staged file; expected $(ASSET)" >&2; \
		echo "  fix: download the asset for this platform ($(ASSET_OS)/$(ASSET_ARCH)) under its RELEASED name into $(DIST)/ (README '## Install')" >&2; \
		exit 1; \
	fi
	@echo "== install: $(DIST)/$(ASSET) verified, installing to $(INSTALL_BIN)"
	@mkdir -p $(INSTALL_DIR)
	@install -m 0755 $(DIST)/$(ASSET) $(INSTALL_BIN)
	@echo "install: $(INSTALL_BIN) installed; verify the deployed identity with 'make install-check'"

# BT-042: the standing drift probe for the DEPLOYED binary (the live shared
# artifact at $(INSTALL_BIN) — never a target `make install` runs against).
# Verifies the released identity and exits 1 loudly on any drift:
#
#   1. `boardctl version` names RELEASE_TAG
#   2. `go version -m` module version == RELEASE_TAG
#      (a from-HEAD build reports e.g. v0.1.9-0.<sha>+dirty instead)
#   3. vcs.modified=false
#      (a dirty from-HEAD build stamps vcs.modified=true)
#   4. vcs.revision is an ANCESTOR of the release tag — degrades to checks
#      1-3 with a stated degraded notice when the ancestry cannot be
#      determined offline (no git binary, or the revision is absent from the
#      local checkout's history, e.g. the deploy host has no checkout)
#
# Failures name the observed values and the fix (make install).
install-check:
	@if [ -z "$(RELEASE_TAG)" ]; then \
		echo "install-check: no RELEASE_TAG resolved (untagged checkout) — pass one: make install-check RELEASE_TAG=v0.1.8" >&2; \
		exit 1; \
	fi
	@if [ ! -x "$(INSTALL_BIN)" ]; then \
		echo "install-check: $(INSTALL_BIN) not found or not executable — install the released asset first (README '## Install' / make install)" >&2; \
		exit 1; \
	fi
	@echo "== install-check: deployed $(INSTALL_BIN) vs release $(RELEASE_TAG)"
	@drift=0; \
	observed_version=`$(INSTALL_BIN) version 2>/dev/null | awk '{print $$3}'`; \
	if [ "$$observed_version" != "$(RELEASE_TAG)" ]; then \
		echo "install-check: FAIL version: '$(INSTALL_BIN) version' prints '$${observed_version:-<none>}' but the release is $(RELEASE_TAG)" >&2; \
		drift=1; \
	else \
		echo "install-check: ok version == $(RELEASE_TAG)"; \
	fi; \
	mod_version=`go version -m $(INSTALL_BIN) 2>/dev/null | awk '$$1=="mod" {print $$3}'`; \
	if [ -z "$$mod_version" ]; then \
		echo "install-check: FAIL: 'go version -m $(INSTALL_BIN)' produced no mod line — not a Go binary built from this module?" >&2; \
		drift=1; \
	elif [ "$$mod_version" != "$(RELEASE_TAG)" ]; then \
		echo "install-check: FAIL module version: go version -m reports '$$mod_version' but the release is $(RELEASE_TAG) (a from-HEAD build reports e.g. v0.1.9-0.<sha>+dirty)" >&2; \
		drift=1; \
	else \
		echo "install-check: ok module version == $(RELEASE_TAG)"; \
	fi; \
	vcs_modified=`go version -m $(INSTALL_BIN) 2>/dev/null | awk '$$1=="build" && $$2 ~ /^vcs\\.modified=/ {sub(/^vcs\\.modified=/, "", $$2); print $$2}'`; \
	if [ "$$vcs_modified" != "false" ]; then \
		echo "install-check: FAIL vcs.modified: '$${vcs_modified:-<absent>}' (want false — a dirty from-HEAD build stamps true)" >&2; \
		drift=1; \
	else \
		echo "install-check: ok vcs.modified=false"; \
	fi; \
	vcs_rev=`go version -m $(INSTALL_BIN) 2>/dev/null | awk '$$1=="build" && $$2 ~ /^vcs\\.revision=/ {sub(/^vcs\\.revision=/, "", $$2); print $$2}'`; \
	if [ -z "$$vcs_rev" ]; then \
		echo "install-check: FAIL vcs.revision absent from 'go version -m' — binary not built from a git checkout?" >&2; \
		drift=1; \
	elif rev_date=`git log -1 --format=%cd --date=short $$vcs_rev 2>/dev/null` && [ -n "$$rev_date" ]; then \
		if git merge-base --is-ancestor $$vcs_rev $(RELEASE_TAG) >/dev/null 2>&1; then \
			echo "install-check: ok vcs.revision $$vcs_rev is an ancestor of $(RELEASE_TAG)"; \
		else \
			rev_subject=`git log -1 --format=%s $$vcs_rev`; \
			echo "install-check: FAIL vcs.revision: $$vcs_rev ($$rev_subject, committed $$rev_date) is NOT an ancestor of $(RELEASE_TAG) — the binary was built from an unrelated commit" >&2; \
			drift=1; \
		fi; \
	else \
		echo "install-check: DEGRADED: vcs.revision $$vcs_rev is not in this checkout's history (or git is unavailable) — the ancestry check cannot run offline; version, module-version and vcs.modified checks still enforced" >&2; \
	fi; \
	if [ "$$drift" = "1" ]; then \
		echo "install-check: DRIFT detected — deployed $(INSTALL_BIN) does not match release $(RELEASE_TAG); fix: make install (after staging the released asset + SHA256SUMS in $(DIST)/, see README '## Install')" >&2; \
		exit 1; \
	fi; \
	echo "install-check: deployed $(INSTALL_BIN) matches release $(RELEASE_TAG)"

clean:
	rm -rf bin $(DIST)
