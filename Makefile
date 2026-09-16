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

.PHONY: build test vet fmt fmt-check version-check release clean

build:
	go build -o bin/$(BINARY) $(CMD)

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
version-check:
	go test -count=1 -run TestVersioncheck ./internal/versioncheck

vet:
	go vet ./...

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
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "== $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath \
			-ldflags "-s -w -X main.version=$(VERSION)" \
			-o $(DIST)/$(BINARY)-$$os-$$arch$$ext $(CMD) || exit 1; \
	done
	@cd $(DIST) && sha256sum $(BINARY)-* > sha256sums.txt
	@echo "release artifacts in $(DIST)/"

clean:
	rm -rf bin $(DIST)
