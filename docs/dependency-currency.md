# Dependency currency — audit and policy

Measured at `HEAD ed8fcfe` (tick 171/172 window, 2026-11-04) by the foreman for
row **BT-86**. This document records *how* currency is measured, *what* the
current state is, and *why* the graph-wide "N modules upgradable" number is not
the number that matters for this module.

## Why "upgradable modules" is not one number

`go list -mod=mod -m -u all` prints the whole **module graph**. That graph is a
superset of the modules this repo actually compiles. Three sets matter:

| Set | Definition | Effect of being behind |
|---|---|---|
| **Build set** | modules whose packages are compiled into the binary (the `require` block of `go.mod`, vendored under `vendor/`) | real: shipped bytes carry the old code |
| **Graph residue** | modules pulled in only to satisfy a *dependency's* own build/test needs | none: no package from them is compiled or vendored |
| **Toolchain** | the `go` directive / `GOTOOLCHAIN` | real: compiled into the binary's runtime |

A currency pass is only meaningful for the **build set** and the **toolchain**.

## Measurement recipe

```sh
go list -mod=mod -m -u all                 # graph-wide; bracket = upgrade available
go mod why -m <module>                     # is it reachable from our packages at all?
go list -mod=mod -m -u <module>            # per-module: print "(latest)" if current
go mod vendor                              # only after a require-set change
make vendor-strip                          # BT-087: drop upstream .golangci* configs
```

`-mod=mod` is mandatory: under the default vendor mode, `go list -m -u` fails
with `can't determine available upgrades using the vendor directory`.

## State at `ed8fcfe`

**Build set — 8 modules, all at latest** (`-u` prints no bracket for any):

| Module | Version |
|---|---|
| `github.com/getkin/kin-openapi` | v0.152.0 |
| `gopkg.in/yaml.v3` | v3.0.1 |
| `github.com/go-openapi/jsonpointer` | v1.0.2 |
| `github.com/kr/pretty` | v0.3.1 |
| `github.com/oasdiff/yaml` | v0.1.3 |
| `github.com/oasdiff/yaml3` | v0.0.16 |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.3 |
| `golang.org/x/text` | v0.43.0 |

Toolchain: `go 1.26.10` directive, `go1.26.10` installed — current.

**Graph residue — 9 modules show an upgrade.** None of them provides a package
this module builds or vendors:

| Module | Current | Available | Reachable from our code? |
|---|---|---|---|
| `github.com/stretchr/testify` | v1.9.0 | v1.12.1 | only via `kin-openapi/openapi3` test deps |
| `github.com/dlclark/regexp2` | v1.11.0 | v1.12.0 | only via `kin-openapi/openapi3` test deps |
| `github.com/rogpeppe/go-internal` | v1.9.0 | v1.16.0 | only via `yaml.v3` test deps |
| `github.com/creack/pty` | v1.1.9 | v1.1.24 | `go mod why`: main module does not need it |
| `github.com/gorilla/mux` | v1.8.0 | v1.8.1 | `go mod why`: main module does not need it |
| `github.com/pkg/diff` | v0.0.0-20210226163009 | v0.0.0-20241224192749 | `go mod why`: main module does not need it |
| `github.com/go-openapi/swag/jsonname` | v0.25.5 | v0.29.2 | `go mod why`: main module does not need it |
| `golang.org/x/mod` | v0.41.0 | v0.42.0 | `go mod why`: main module does not need it |
| `golang.org/x/tools` | v0.51.0 | v0.52.0 | `go mod why`: main module does not need it |

`govulncheck ./...` reports `No vulnerabilities found` at this HEAD, so none of
this is a security exposure.

## Why the graph residue is deliberately left alone

Promoting any graph-residue module to a newer version requires an explicit
`require` line, because the module is not in `go.mod` today. Measured on a
scratch clone of `ed8fcfe`:

```
$ go get github.com/stretchr/testify@v1.12.1
go: upgraded github.com/stretchr/testify v1.9.0 => v1.12.1
$ git diff go.mod
+	github.com/stretchr/testify v1.12.1 // indirect
$ go build ./...
go: inconsistent vendoring in ...:
	github.com/stretchr/testify@v1.12.1: is explicitly required in go.mod,
	but not marked as explicit in vendor/modules.txt
$ go mod vendor            # required to restore the build
$ git status --porcelain
 M go.mod
 M go.sum
 M vendor/modules.txt
?? vendor/github.com/go-openapi/jsonpointer/.golangci.yml
?? vendor/github.com/oasdiff/yaml/.golangci.yml
?? vendor/github.com/santhosh-tekuri/jsonschema/v6/.golangci.yml
```

So every such bump costs: an indirect dependency this code never compiles, a
`go.sum` rewrite, `vendor/modules.txt` churn, and re-planted upstream lint
configs that must be removed with `make vendor-strip` (BT-087). The compiled
binary is byte-for-byte unchanged. That is exactly the "new indirect dependency
the code does not need" that BT-86's acceptance criterion 2 forbids.

**Policy: bump the build set; do not bump graph residue.** A graph-residue
module is upgraded only when a *vendored* module's requirement moves it, or when
`govulncheck` names it as reachable.

## Reconciliation with BT-86's filing evidence (2026-11-03)

The row was filed with a graph-wide snapshot that no longer describes HEAD:

| Row claim (2026-11-03) | HEAD (2026-11-04) |
|---|---|
| `golang.org/x/text v0.14.0 -> v0.43.0` | already v0.43.0 (latest) |
| `golang.org/x/sys v0.5.0 -> v0.49.0` | not a module of this build at all |
| `github.com/getkin/kin-openapi v0.149.0 -> v0.152.0` | already v0.152.0 (latest) |
| `15+ upgradable` | 9 graph-residue entries, 0 build-set entries |

The build-set half of that drift was closed by the kin-openapi v0.152.0 and
`x/text` bumps already on main; what remains is graph residue.

## Cadence

There is no scheduled pass. The follow-up row (`BT-89`) adds a report-only
build-set currency check plus a documented cadence, so the next real drift (a
vendored module falling behind) is found by a routine tick instead of by a
one-off audit.

## Verification at this HEAD

`go build ./...`, `go vet ./...`, `go test ./... -count=1`,
`go test -race ./... -count=1 -short`, `govulncheck ./...`,
`make fmt-check version-check spec-check vendor-check vuln-check cover-check`
(83.1% total, floor 80.0%, 15 packages) — all green.
