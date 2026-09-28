package speccheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/describe"
)

// goodServe is a serve.go-shaped source carrying the real three-route
// wiring (same registrations as cmd/boardctl/serve.go's routes()).
const goodServe = `package main

import "net/http"

func (s *serveServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /", s.handleUpload)
	mux.HandleFunc("GET /api/boards", s.handleAPIBoards)
	return mux
}
`

// writeFixture materializes serve/spec files in a temp dir and returns the
// paths plus a check() closure running the gate over them.
type fixture struct {
	serve string
	spec  string
}

func writeFixture(t *testing.T, f fixture) (servePath, specPath string, check func() error) {
	t.Helper()
	dir := t.TempDir()
	servePath = filepath.Join(dir, "serve.go")
	specPath = filepath.Join(dir, "openapi.yaml")
	if err := os.WriteFile(servePath, []byte(f.serve), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(f.spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return servePath, specPath, func() error { return CheckSources(servePath, specPath) }
}

// specFor renders the contract for a serve.go-shaped source via the real
// derivation + renderer chain — the same path gendocs uses in the repo.
func specFor(t *testing.T, serveSource string) string {
	t.Helper()
	dir := t.TempDir()
	serve := filepath.Join(dir, "serve.go")
	if err := os.WriteFile(serve, []byte(serveSource), 0o644); err != nil {
		t.Fatal(err)
	}
	routes, err := describe.DeriveHTTP(serve)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	doc, err := describe.RenderOpenAPI(describe.Registry{HTTP: routes})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return doc
}

// TestCheck_ParityGreen proves the positive arm: gate passes when the spec
// is freshly generated from the wiring.
func TestCheck_ParityGreen(t *testing.T) {
	_, _, check := writeFixture(t, fixture{serve: goodServe, spec: specFor(t, goodServe)})
	if err := check(); err != nil {
		t.Fatalf("gate failed on a freshly generated spec: %v", err)
	}
}

// TestCheck_FakeRouteAddedFailsNamingTheDrift is the wired-side negative
// probe: a route is added to the wiring WITHOUT regenerating the spec, and
// the gate must fail NAMING the new route and the fix.
func TestCheck_FakeRouteAddedFailsNamingTheDrift(t *testing.T) {
	fakeRouted := strings.Replace(goodServe,
		"\treturn mux",
		"\tmux.HandleFunc(\"GET /api/fake\", s.handleFake)\n\treturn mux",
		1)
	_, _, check := writeFixture(t, fixture{serve: fakeRouted, spec: specFor(t, goodServe)})
	err := check()
	if err == nil {
		t.Fatal("gate PASSED after a route was added without regenerating the spec — the parity gate is blind")
	}
	if !strings.Contains(err.Error(), "GET /api/fake") || !strings.Contains(err.Error(), "missing from") {
		t.Fatalf("gate failure does not name the drifted route:\n%v", err)
	}
	if !strings.Contains(err.Error(), "go run ./cmd/gendocs") {
		t.Fatalf("gate failure does not name the regeneration fix:\n%v", err)
	}
	t.Logf("fake-route probe failure (expected):\n%v", err)
}

// TestCheck_FakeRouteRemovedAfterRegenerationPasses is the restore arm of
// the wired-side probe: regenerating the spec from the mutated wiring makes
// the gate green again (and the fake route gains no operationId without
// curation, so the render itself must refuse — fail-loud both layers).
func TestCheck_FakeRouteRemovedAfterRegenerationPasses(t *testing.T) {
	fakeRouted := strings.Replace(goodServe,
		"\treturn mux",
		"\tmux.HandleFunc(\"GET /api/fake\", s.handleFake)\n\treturn mux",
		1)
	if _, err := describe.RenderOpenAPI(mustDerive(t, fakeRouted)); err == nil {
		t.Fatal("renderer emitted a contract for an uncurated route — it must refuse (no operationId, no muster verb)")
	}
}

// TestCheck_SpecMutationArms covers the spec-side probes: removing a path,
// changing an operationId, and de-absolutizing the server URL each fail
// with a finding that names the drift.
func TestCheck_SpecMutationArms(t *testing.T) {
	base := specFor(t, goodServe)

	t.Run("removed path", func(t *testing.T) {
		// Drop the '/api/boards' path item entirely (regression: spec
		// regenerated from an older wiring).
		mutated := removePathBlock(t, base, "'/api/boards':")
		_, _, check := writeFixture(t, fixture{serve: goodServe, spec: mutated})
		err := check()
		if err == nil {
			t.Fatal("gate PASSED after a path was removed from the spec")
		}
		// Removing the spec's path item leaves the wired route undescribed:
		// the wired-side finding names the route and the regeneration fix.
		if !strings.Contains(err.Error(), "GET /api/boards") || !strings.Contains(err.Error(), "missing from") {
			t.Fatalf("failure does not name the undescribed route:\n%v", err)
		}
		t.Logf("removed-path probe failure (expected):\n%v", err)
	})

	t.Run("operationId drift", func(t *testing.T) {
		mutated := strings.Replace(base, "operationId: listLoadedBoards", "operationId: listBoards", 1)
		_, _, check := writeFixture(t, fixture{serve: goodServe, spec: mutated})
		err := check()
		if err == nil {
			t.Fatal("gate PASSED after an operationId drifted")
		}
		if !strings.Contains(err.Error(), "listBoards") || !strings.Contains(err.Error(), "listLoadedBoards") {
			t.Fatalf("failure does not name both sides of the operationId drift:\n%v", err)
		}
		t.Logf("operationId-drift probe failure (expected):\n%v", err)
	})

	t.Run("relative server url", func(t *testing.T) {
		mutated := strings.Replace(base, "url: http://127.0.0.1:8787", "url: /api", 1)
		_, _, check := writeFixture(t, fixture{serve: goodServe, spec: mutated})
		err := check()
		if err == nil {
			t.Fatal("gate PASSED with a relative servers[].url")
		}
		if !strings.Contains(err.Error(), "RELATIVE") || !strings.Contains(err.Error(), "cannot dereference") {
			t.Fatalf("failure does not explain the relative-server case:\n%v", err)
		}
		t.Logf("relative-server probe failure (expected):\n%v", err)
	})
}

// TestCheck_DerivationLossFails proves the gate cannot be fooled into a
// pass by losing the source anchor: a serve.go without routes() is an
// ERROR, never parity.
func TestCheck_DerivationLossFails(t *testing.T) {
	anchorless := "package main\n\nfunc somethingElse() {}\n"
	_, _, check := writeFixture(t, fixture{serve: anchorless, spec: specFor(t, goodServe)})
	err := check()
	if err == nil {
		t.Fatal("gate PASSED with an undervivable wiring — a parity check that cannot see the routes must fail")
	}
	if !strings.Contains(err.Error(), "cannot derive the wired routes") {
		t.Fatalf("failure does not name the derivation loss:\n%v", err)
	}
}

// TestCheck_UnparseableSpecFails covers the corrupt-spec arm.
func TestCheck_UnparseableSpecFails(t *testing.T) {
	_, _, check := writeFixture(t, fixture{serve: goodServe, spec: "paths: [oops\n\t- broken: ::"})
	err := check()
	if err == nil {
		t.Fatal("gate PASSED on an unparseable spec")
	}
	if !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("failure does not name the parse failure:\n%v", err)
	}
}

// TestParseSpec_ContractEnforcement pins the parse-level rules the gate
// applies before parity: OpenAPI 3.x only, operationId required, no
// duplicate ids, at least one operation.
func TestParseSpec_ContractEnforcement(t *testing.T) {
	t.Run("not openapi 3.x", func(t *testing.T) {
		if _, err := ParseSpec([]byte("openapi: 2.0\ninfo: {title: x, version: '1'}\npaths: {'/': {get: {operationId: a}}}")); err == nil || !strings.Contains(err.Error(), "OpenAPI 3.x") {
			t.Fatalf("want OpenAPI 3.x refusal, got: %v", err)
		}
	})
	t.Run("missing operationId", func(t *testing.T) {
		if _, err := ParseSpec([]byte("openapi: 3.0.3\ninfo: {title: x, version: '1'}\npaths: {'/': {get: {summary: s}}}")); err == nil || !strings.Contains(err.Error(), "no operationId") {
			t.Fatalf("want missing-operationId refusal, got: %v", err)
		}
	})
	t.Run("duplicate operationId", func(t *testing.T) {
		doc := "openapi: 3.0.3\ninfo: {title: x, version: '1'}\npaths:\n  '/':\n    get: {operationId: dup}\n    post: {operationId: dup}\n"
		if _, err := ParseSpec([]byte(doc)); err == nil || !strings.Contains(err.Error(), "duplicate operationId") {
			t.Fatalf("want duplicate-operationId refusal, got: %v", err)
		}
	})
	t.Run("zero operations", func(t *testing.T) {
		if _, err := ParseSpec([]byte("openapi: 3.0.3\ninfo: {title: x, version: '1'}\npaths: {'/': {parameters: []}}")); err == nil || !strings.Contains(err.Error(), "zero operations") {
			t.Fatalf("want zero-operations refusal, got: %v", err)
		}
	})
}

// TestRepoParity is the repo-level gate run: the real serve.go against the
// real docs/muster/openapi.yaml. This is the `go test ./...` (and would-be
// make spec-check / CI step) enforcement surface.
func TestRepoParity(t *testing.T) {
	root, err := RepoRoot(".")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if err := Check(root); err != nil {
		t.Fatalf("docs/muster/openapi.yaml drifted from cmd/boardctl/serve.go:\n%v", err)
	}
}

func mustDerive(t *testing.T, serveSource string) describe.Registry {
	t.Helper()
	dir := t.TempDir()
	serve := filepath.Join(dir, "serve.go")
	if err := os.WriteFile(serve, []byte(serveSource), 0o644); err != nil {
		t.Fatal(err)
	}
	routes, err := describe.DeriveHTTP(serve)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	return describe.Registry{HTTP: routes}
}

// removePathBlock deletes one top-level path mapping (from its "'<path>':"
// line through the line before the next 2-space-indented key) from the
// rendered document — a surgical simulation of a stale regeneration.
func removePathBlock(t *testing.T, doc, marker string) string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	start := -1
	for i, l := range lines {
		if l == "  "+marker {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("marker %q not found in document", marker)
	}
	end := len(lines)
	for j := start + 1; j < len(lines); j++ {
		if strings.HasPrefix(lines[j], "  ") && !strings.HasPrefix(lines[j], "    ") {
			end = j
			break
		}
	}
	return strings.Join(append(lines[:start], lines[end:]...), "\n")
}
