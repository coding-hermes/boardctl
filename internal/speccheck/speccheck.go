// Package speccheck is the boardctl ↔ OpenAPI contract parity gate
// (BT-062): a check-only package that fails when docs/muster/openapi.yaml
// drifts from the real serve wiring in cmd/boardctl/serve.go, shared by
// three enforcement surfaces (mirrors internal/fmtcheck and
// internal/versioncheck):
//
//   - `make spec-check` would run: go test -count=1 -run TestSpeccheck ./internal/speccheck
//     (the Makefile target itself is owned by the foreman; the command above
//     is the exact wiring point)
//   - .github/workflows/ci.yml has (or should gain) a step with the
//     byte-identical command — same ownership note
//   - plain `go test ./...` (incl. `-short`) picks this package up like any
//     other
//
// What counts as drift (each finding names the route and both sides):
//
//   - a route wired in serveServer.routes() but absent from the spec
//     (a new serve endpoint without regenerating the contract);
//   - a spec operation with no matching mux.HandleFunc registration
//     (a removed/renamed route without regenerating);
//   - same method+path on both sides but a different operationId;
//   - a spec operation without an operationId at all (muster cannot name
//     a CLI verb / MCP tool for it);
//   - a relative servers[].url (muster cannot dereference a relative
//     server URL from a local-file spec) or a servers[].url that stopped
//     matching the serve default 127.0.0.1:8787;
//   - an unparseable spec file.
//
// The gate re-derives the route table from serve.go source on every run
// (via internal/describe) — there is no second hand-maintained route list
// to drift. A derivation failure (the routes() anchor lost) is an ERROR,
// never a silent pass: a parity check that cannot see the routes must fail.
//
// This package is a CHECK, never a generator: nothing here rewrites the
// spec. The writing helper is `go run ./cmd/gendocs` for the contributor.
//
// Parser note (deliberate choice, recorded per BT-062): the structural
// parity parse uses gopkg.in/yaml.v3 (already resolved in the module
// cache) over the few fields the gate needs; the package adds no OpenAPI
// dependency. The published artifact is additionally validated with
// muster's own parser (github.com/getkin/kin-openapi, the exact loader
// muster's pkg/openapi uses) by TestSpeccheck_ParsesWithKinOpenAPI in this
// package's test — so the contract is proven against the real consumer
// parser on every `go test ./...`, while the gate itself stays dependency-
// light.
package speccheck

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/coding-hermes/boardctl/internal/describe"

	"gopkg.in/yaml.v3"
)

// Check runs the parity gate with the repo's default paths.
func Check(root string) error {
	return CheckSources(
		filepath.Join(root, "cmd", "boardctl", "serve.go"),
		filepath.Join(root, "docs", "muster", "openapi.yaml"),
	)
}

// CheckSources is the path-explicit form of the gate (the test drives this
// with temp-dir fixtures to prove the negative probes). serveGoPath is the
// serve surface wiring; specPath is the published OpenAPI document.
func CheckSources(serveGoPath, specPath string) error {
	wired, err := describe.DeriveHTTP(serveGoPath)
	if err != nil {
		// Fail loud: a lost derivation anchor must never read as parity.
		return fmt.Errorf("parity gate cannot derive the wired routes: %w", err)
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("parity gate cannot read the spec: %w", err)
	}
	parsed, err := ParseSpec(data)
	if err != nil {
		return fmt.Errorf("parity gate cannot parse %s: %w", specPath, err)
	}

	var findings []string

	wiredSet := map[string]describe.HTTPRoute{}
	for _, r := range wired {
		wiredSet[r.Method+" "+r.Path] = r
	}
	specSet := map[string]SpecOp{}
	for _, op := range parsed.Operations {
		specSet[op.Method+" "+op.Path] = op
	}

	// Wired-but-undescribed and described-but-unwired, named both ways.
	for _, r := range wired {
		key := r.Method + " " + r.Path
		op, ok := specSet[key]
		if !ok {
			findings = append(findings, fmt.Sprintf(
				"route %s is wired in %s but missing from %s — regenerate the contract: go run ./cmd/gendocs",
				key, serveGoPath, specPath))
			continue
		}
		if op.OperationID == "" {
			findings = append(findings, fmt.Sprintf(
				"spec operation %s has no operationId — muster cannot name a verb for it; curate it in internal/describe and regenerate",
				key))
		} else if r.OperationID != "" && op.OperationID != r.OperationID {
			findings = append(findings, fmt.Sprintf(
				"operationId drift on %s: spec says %q, registry says %q — regenerate the contract after curating internal/describe",
				key, op.OperationID, r.OperationID))
		}
	}
	for _, op := range parsed.Operations {
		key := op.Method + " " + op.Path
		if _, ok := wiredSet[key]; !ok {
			findings = append(findings, fmt.Sprintf(
				"spec declares %s but no mux.HandleFunc registration for it exists in %s — stale contract, regenerate: go run ./cmd/gendocs",
				key, serveGoPath))
		}
	}

	// servers[] must carry the absolute, dereferenceable default.
	if len(parsed.ServerURLs) == 0 {
		findings = append(findings, "spec declares no servers[] — muster would have no base URL when --base-url is omitted")
	} else {
		u := parsed.ServerURLs[0]
		if !isAbsoluteHTTPURL(u) {
			findings = append(findings, fmt.Sprintf(
				"servers[0].url %q is RELATIVE — muster cannot dereference a relative server URL from a local-file spec; publish an absolute url such as %s",
				u, describe.SpecServerURL))
		} else if u != describe.SpecServerURL {
			findings = append(findings, fmt.Sprintf(
				"servers[0].url %q does not match the serve default %q — update internal/describe.SpecServerURL and regenerate if the serve default moved",
				u, describe.SpecServerURL))
		}
	}

	if len(findings) == 0 {
		return nil
	}
	sort.Strings(findings)
	return fmt.Errorf("spec drifted from the wired serve surface (%d finding%s):\n  - %s",
		len(findings), plural(len(findings)), strings.Join(findings, "\n  - "))
}

// SpecOp is one described operation of the spec file.
type SpecOp struct {
	Path        string
	Method      string
	OperationID string
}

// ParsedSpec is the slice of the OpenAPI document the gate checks.
type ParsedSpec struct {
	OpenAPIVersion string // the openapi: field ("3.0.3")
	InfoVersion    string // info.version (the contract's own version)
	ServerURLs     []string
	Operations     []SpecOp // sorted by path then method
}

// httpMethods is the OpenAPI operation set the gate recognizes inside a
// path item. Everything else (parameters, servers, summary, ...) is
// ignored by the extraction.
var httpMethods = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// ParseSpec parses the fields the parity gate needs from an OpenAPI 3.x
// YAML document with gopkg.in/yaml.v3 (see the package doc for why the
// gate itself stays OpenAPI-dependency-free). It enforces: openapi: 3.x,
// at least one operation, an operationId on every operation, and no
// duplicate operationIds.
func ParseSpec(data []byte) (ParsedSpec, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ParsedSpec{}, fmt.Errorf("yaml: %w", err)
	}
	out := ParsedSpec{}

	ver, _ := doc["openapi"].(string)
	if !strings.HasPrefix(ver, "3.") {
		return ParsedSpec{}, fmt.Errorf("not an OpenAPI 3.x document (openapi: %q)", ver)
	}
	out.OpenAPIVersion = ver

	if info, ok := doc["info"].(map[string]interface{}); ok {
		out.InfoVersion, _ = info["version"].(string)
	}

	if servers, ok := doc["servers"].([]interface{}); ok {
		for _, s := range servers {
			if sm, ok := s.(map[string]interface{}); ok {
				if u, ok := sm["url"].(string); ok {
					out.ServerURLs = append(out.ServerURLs, u)
				}
			}
		}
	}

	paths, ok := doc["paths"].(map[string]interface{})
	if !ok || len(paths) == 0 {
		return ParsedSpec{}, fmt.Errorf("no paths described")
	}
	seen := map[string]bool{}
	for path, item := range paths {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return ParsedSpec{}, fmt.Errorf("path %q is not a mapping", path)
		}
		for method, op := range itemMap {
			if !httpMethods[method] {
				continue
			}
			opMap, ok := op.(map[string]interface{})
			if !ok {
				return ParsedSpec{}, fmt.Errorf("operation %s %s is not a mapping", strings.ToUpper(method), path)
			}
			opID, _ := opMap["operationId"].(string)
			if opID == "" {
				return ParsedSpec{}, fmt.Errorf("operation %s %s has no operationId", strings.ToUpper(method), path)
			}
			if seen[opID] {
				return ParsedSpec{}, fmt.Errorf("duplicate operationId %q", opID)
			}
			seen[opID] = true
			out.Operations = append(out.Operations, SpecOp{
				Path:        path,
				Method:      strings.ToUpper(method),
				OperationID: opID,
			})
		}
	}
	if len(out.Operations) == 0 {
		return ParsedSpec{}, fmt.Errorf("paths carries zero operations")
	}
	sort.Slice(out.Operations, func(i, j int) bool {
		if out.Operations[i].Path != out.Operations[j].Path {
			return out.Operations[i].Path < out.Operations[j].Path
		}
		return out.Operations[i].Method < out.Operations[j].Method
	})
	return out, nil
}

// isAbsoluteHTTPURL mirrors muster's own predicate (pkg/openapi
// .isAbsoluteHTTPURL, DF-MUSTER-1): parseable and scheme http/https.
func isAbsoluteHTTPURL(s string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// RepoRoot returns the module root by walking up from start to the nearest
// directory containing go.mod, falling back to this package's own source
// location so the result never depends on where the developer cd'd.
// (Same shape as internal/fmtcheck.RepoRoot.)
func RepoRoot(start string) (string, error) {
	if root, err := walkUpToGoMod(start); err == nil {
		return root, nil
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		if root, err := walkUpToGoMod(filepath.Dir(file)); err == nil {
			return root, nil
		}
	}
	return "", fmt.Errorf("no go.mod found walking up from %s", start)
}

func walkUpToGoMod(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", start)
		}
		dir = parent
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
