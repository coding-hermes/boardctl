package describe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realServeWiring is the exact routes() body of cmd/boardctl/serve.go at
// the time internal/describe was introduced — the fixture for derivation
// tests, kept textually faithful (registration order included).
const realServeWiring = `package main

import "net/http"

func (s *serveServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /", s.handleUpload)
	mux.HandleFunc("GET /api/boards", s.handleAPIBoards)
	return mux
}
`

// TestDeriveCLI_RealMain pins the CLI verb derivation against the repo's
// live main.go: the registry must see every run() switch command (16 at
// the derivation's introduction) and none besides, help excluded because
// its case body prints usageText.
func TestDeriveCLI_RealMain(t *testing.T) {
	verbs, err := DeriveCLI(filepath.Join("..", "..", "cmd", "boardctl", "main.go"))
	if err != nil {
		t.Fatalf("derive from the real main.go: %v", err)
	}
	want := []string{
		"init", "list", "show", "create", "update", "event", "header",
		"validate", "sweep-status", "doctor", "version", "stats",
		"render", "import", "serve", "install",
	}
	if len(verbs) != len(want) {
		t.Fatalf("derived %d verbs, want %d: %v", len(verbs), len(want), names(verbs))
	}
	for i, w := range want {
		if verbs[i].Name != w {
			t.Fatalf("verb[%d] = %q, want %q (full: %v)", i, verbs[i].Name, w, names(verbs))
		}
	}
	if verbs[0].Summary == "" {
		t.Fatalf("curated summary missing for %q", verbs[0].Name)
	}
}

// TestDeriveCLI_AnchorLostFails proves the derivation cannot silently
// return an empty census when the switch is gone.
func TestDeriveCLI_AnchorLostFails(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "main.go")
	if err := os.WriteFile(p, []byte("package main\n\nfunc run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DeriveCLI(p); err == nil {
		t.Fatal("DeriveCLI succeeded on a main.go without the switch — must fail loud")
	}
}

// TestDeriveHTTP_RealServe pins the HTTP route derivation against the
// repo's live serve.go: order and curation.
func TestDeriveHTTP_RealServe(t *testing.T) {
	routes, err := DeriveHTTP(filepath.Join("..", "..", "cmd", "boardctl", "serve.go"))
	if err != nil {
		t.Fatalf("derive from the real serve.go: %v", err)
	}
	wantKeys := []string{"GET /", "POST /", "GET /api/boards"}
	if len(routes) != len(wantKeys) {
		t.Fatalf("derived %d routes, want %d", len(routes), len(wantKeys))
	}
	for i, w := range wantKeys {
		if routes[i].Method+" "+routes[i].Path != w {
			t.Fatalf("route[%d] = %s %s, want %s", i, routes[i].Method, routes[i].Path, w)
		}
		if routes[i].OperationID == "" {
			t.Fatalf("route %s has no curated operationId", w)
		}
	}
	if routes[0].OperationID != "getUploaderForm" || routes[1].OperationID != "uploadBoardReport" || routes[2].OperationID != "listLoadedBoards" {
		t.Fatalf("operationId curation drifted: %v", routes)
	}
}

// TestDeriveHTTP_AnchorLostFails proves the loud failure on a lost routes()
// anchor (the property speccheck's derivation-loss arm relies on).
func TestDeriveHTTP_AnchorLostFails(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "serve.go")
	if err := os.WriteFile(p, []byte("package main\n\nfunc notRoutes() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DeriveHTTP(p); err == nil || !strings.Contains(err.Error(), "routes()") {
		t.Fatalf("want loud anchor-loss failure, got: %v", err)
	}
}

// TestDeriveSwitchVerbs_NegativeControls exercises the pure verb parser
// with synthetic bodies: help exclusion, non-verb cases ignored, duplicate
// anchor refusal.
func TestDeriveSwitchVerbs_NegativeControls(t *testing.T) {
	body := `func run(args []string) int {
	switch cmd {
	case "init":
		return cmdInit(args)
	case "help", "-h", "--help":
		fmt.Print(usageText)
	case "list":
		return cmdList(args)
	case "someFlag", "list":
		return cmdList(args)
	default:
		fmt.Print(usageText)
	}
	return 2
}`
	verbs, err := deriveSwitchVerbs(body)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	got := strings.Join(verbs, ",")
	if got != "init,list" {
		t.Fatalf("verbs = %q, want \"init,list\" (help excluded by usageText, non-verb-shaped case ignored)", got)
	}

	dup := "switch cmd {\ncase \"a\":\n}\nswitch cmd {\ncase \"b\":\n}"
	if _, err := deriveSwitchVerbs(dup); err == nil || !strings.Contains(err.Error(), "more than one") {
		t.Fatalf("want duplicate-anchor refusal, got: %v", err)
	}
}

// TestDeriveRoutes_NegativeControls covers the route parser's edges.
func TestDeriveRoutes_NegativeControls(t *testing.T) {
	noRoutes := "func (s *serveServer) routes() http.Handler {\n\tmux := http.NewServeMux()\n\treturn mux\n}"
	if _, err := deriveRoutes(noRoutes); err == nil || !strings.Contains(err.Error(), "zero mux.HandleFunc") {
		t.Fatalf("want zero-registration refusal, got: %v", err)
	}
	unclosed := "func (s *serveServer) routes() http.Handler {\n\tmux.HandleFunc(\"GET /\", h)"
	if _, err := deriveRoutes(unclosed); err == nil || !strings.Contains(err.Error(), "never closes") {
		t.Fatalf("want unclosed-block refusal, got: %v", err)
	}
}

// TestRenderOpenAPI_RefusesUncuratedRoute proves the fail-loud curation
// gate: a derived route without an httpOperations entry stops the render.
func TestRenderOpenAPI_RefusesUncuratedRoute(t *testing.T) {
	reg := Registry{HTTP: []HTTPRoute{{Method: "GET", Path: "/api/future"}}}
	if _, err := RenderOpenAPI(reg); err == nil || !strings.Contains(err.Error(), "no curated operationId") {
		t.Fatalf("want curation refusal, got: %v", err)
	}
}

// TestRenderOpenAPI_ServerURLAbsolute pins the contract invariant the
// muster consumer depends on: the published servers[].url is the absolute
// loopback default.
func TestRenderOpenAPI_ServerURLAbsolute(t *testing.T) {
	doc, err := RenderOpenAPI(Registry{HTTP: []HTTPRoute{{
		Method: "GET", Path: "/", OperationID: "getUploaderForm", Summary: "s",
	}}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(doc, "url: "+SpecServerURL) {
		t.Fatalf("document does not publish the absolute server URL %s:\n%s", SpecServerURL, doc)
	}
	if strings.Contains(doc, "url: /") && !strings.Contains(doc, "url: http") {
		t.Fatalf("document carries a relative server url:\n%s", doc)
	}
}

func names(cmds []CLICommand) []string {
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, c.Name)
	}
	return out
}
