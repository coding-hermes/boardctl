package speccheck

import (
	"context"
	"net/url"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/describe"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestSpeccheck_ParsesWithKinOpenAPI validates the PUBLISHED contract with
// muster's own parser: muster's pkg/openapi.Parser parses specs through
// openapi3.NewLoader().LoadFromData (kin-openapi), so a document that
// passes the loader here is proven against the exact consumer parser
// BT-062 targets. The loader resolves $refs and Validate applies the full
// OpenAPI 3 structural validation (schemas, responses, security of the
// document itself) — far beyond the structural yaml.v3 parse the gate
// needs, which is why this test pins the dependency to muster's exact
// kin-openapi version instead of the gate (recorded choice, see package
// doc).
//
// A relative servers[].url would survive muster's loader untouched (it is
// resolved only against an absolute spec source URL, which a local file is
// not), so the dereferenceability invariant is asserted explicitly below
// rather than delegated to Validate.
func TestSpeccheck_ParsesWithKinOpenAPI(t *testing.T) {
	root, err := RepoRoot(".")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	specPath := filepath.Join(root, "docs", "muster", "openapi.yaml")

	doc, err := openapi3.NewLoader().LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("muster's parser (kin-openapi %s) cannot load %s: %v", kinOpenAPIVersion(t), specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("muster's parser rejects the published contract: %v", err)
	}

	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Fatalf("published contract declares openapi %q, want 3.x", doc.OpenAPI)
	}

	// servers[] dereferenceability: absolute http(s) URL, matching the
	// curated serve default.
	if len(doc.Servers) == 0 {
		t.Fatal("published contract declares no servers[]")
	}
	srv := doc.Servers[0].URL
	u, err := url.Parse(srv)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		t.Fatalf("servers[0].url %q is not an absolute http(s) URL — muster cannot dereference it from a local-file spec", srv)
	}
	if srv != describe.SpecServerURL {
		t.Fatalf("servers[0].url %q does not match the serve default %q", srv, describe.SpecServerURL)
	}

	// Every operation carries an operationId (muster names CLI verbs /
	// MCP tools from them), and the count matches the wired surface
	// (BT-075: the three read-only UI routes joined the contract).
	want := map[string]bool{
		"getUploaderForm":   false,
		"uploadBoardReport": false,
		"listLoadedBoards":  false,
		"getWebUI":          false,
		"getBoardDetail":    false,
		"getBoardEvents":    false,
	}
	n := 0
	for path, pathItem := range doc.Paths.Map() {
		for _, op := range pathItem.Operations() {
			n++
			if op.OperationID == "" {
				t.Fatalf("operation on %s has no operationId — muster cannot name a verb for it", path)
			}
			if _, ok := want[op.OperationID]; !ok {
				t.Fatalf("unexpected operationId %q in the published contract", op.OperationID)
			}
			want[op.OperationID] = true
		}
	}
	if n != len(want) {
		t.Fatalf("published contract describes %d operations, want %d", n, len(want))
	}
	for id, seen := range want {
		if !seen {
			t.Fatalf("published contract is missing operationId %q", id)
		}
	}
}

// kinOpenAPIVersion reports the resolved kin-openapi version from the build
// info, so a test failure names the exact parser version being asserted
// against.
func kinOpenAPIVersion(t *testing.T) string {
	t.Helper()
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, d := range info.Deps {
		if d.Path == "github.com/getkin/kin-openapi" {
			return d.Version
		}
	}
	return "unknown"
}
