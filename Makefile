BINARY := boardctl
CMD := ./cmd/boardctl
VERSION ?= $(shell date -u +%Y%m%d)
DIST := dist

PLATFORMS := \
	linux/amd64 linux/arm64 linux/arm \
	darwin/amd64 darwin/arm64 \
	windows/amd64 \
	freebsd/amd64

.PHONY: build test vet fmt fmt-check release clean

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

vet:
	go vet ./...

release: test
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
