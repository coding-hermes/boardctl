// Package describe is boardctl's registry: one structured description of the
// surface this binary actually serves, DERIVED from the real wiring in
// cmd/boardctl (run()'s command switch and serveServer.routes' mux
// registrations) instead of maintained as a hand-copied list.
//
// Consumers:
//
//   - cmd/gendocs generates docs/muster/openapi.yaml (the OpenAPI 3.x
//     contract muster consumes) from this registry's HTTP routes;
//   - internal/speccheck re-derives the registry from source and fails when
//     the generated contract drifts from it;
//   - anything that needs to name the real command set (reports, docs
//     tooling) can read it here instead of grepping main.go again.
//
// Derivation rules mirror the house meta-test discipline (cf.
// cmd/boardctl/df14_closure_switch_test.go, whose switchVerbsFromLines is
// the same rule inside package main — main packages cannot be imported, so
// the rule is carried here too; both copies document each other's anchor):
//
//   - CLI verbs: locate `switch cmd {` in main.go, brace-match the switch's
//     closing brace, take every `case "X":` whose body does not print the
//     top-level usageText (the help case) and whose X is verb-shaped
//     (lowercase letter first, then letters/digits/hyphens).
//   - HTTP routes: locate `func (s *serveServer) routes()` in serve.go,
//     brace-match the function body, take every
//     `mux.HandleFunc("<METHOD> <path>", ...)` in appearance order.
//
// Both derivations fail LOUD when their anchor is lost (the switch or the
// routes function moved or was renamed): callers get an error, never an
// empty-but-successful census. speccheck turns that into a hard gate — a
// parity check that cannot see the routes must fail, not pass.
//
// This package is a pure reader: it never writes files and never mutates
// the sources it derives from.
package describe

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CLICommand is one command of run()'s switch. Summary is the one-line
// human description curated in this package (cliSummaries); it is empty
// when a derived verb has no curated line yet.
type CLICommand struct {
	Name    string
	Summary string
}

// HTTPRoute is one mux registration of serveServer.routes(). Method and
// Path come from the source wiring; OperationID and Summary are curated per
// route in this package (httpOperations) so the OpenAPI generator and the
// parity checker agree on one naming.
type HTTPRoute struct {
	Method      string // "GET", "POST", ...
	Path        string // "/", "/api/boards", ...
	OperationID string // OpenAPI operationId (curated; empty = not yet curated)
	Summary     string // one-line human description (curated; may be empty)
}

// cliSummaries carries the curated one-line description per CLI verb. A new
// verb derived from the switch without an entry here is still reported (Name
// only) — the registry never hides a real command — but add a line so the
// docs tooling renders it with a description.
var cliSummaries = map[string]string{
	"init":         "bootstrap a fresh board (tasks.jsonl + events.jsonl + header)",
	"list":         "list task rows",
	"show":         "show one task row",
	"create":       "create a task row",
	"update":       "update one task row",
	"event":        "append a board event row",
	"header":       "show or set board header fields",
	"validate":     "validate board files (key uniformity, dangling deps)",
	"sweep-status": "report (and with --apply fix) off-vocabulary task statuses",
	"doctor":       "report board health",
	"version":      "print version and build commit",
	"stats":        "board task statistics",
	"render":       "render the self-contained HTML analytics report",
	"import":       "import a board export.json",
	"serve":        "run the loopback uploader/report HTTP server",
	"install":      "install the board-lint pre-commit hook",
}

// httpOperations carries the curated OpenAPI operationId + summary per
// method+path of the serve surface. A derived route without an entry has an
// empty OperationID — gendocs refuses to emit the contract and speccheck
// fails until the route is curated here (fail-loud, never silently dropped).
// BT-075 added the three read-only UI routes; their summaries state the
// snapshot semantics (R9) and the closed read model (R7).
var httpOperations = map[string]HTTPRoute{
	"GET /": {
		OperationID: "getUploaderForm",
		Summary:     "the self-contained uploader page (HTML form for folder and .zip uploads)",
	},
	"POST /": {
		OperationID: "uploadBoardReport",
		Summary:     "upload a board folder or .zip and receive the rendered multi-board HTML report",
	},
	"GET /api/boards": {
		OperationID: "listLoadedBoards",
		Summary:     "list the boards currently loaded by the running serve instance",
	},
	"GET /ui": {
		OperationID: "getWebUI",
		Summary:     "the self-contained web UI (board viewer over the loaded snapshots; read-only)",
	},
	"GET /api/board/{slug}": {
		OperationID: "getBoardDetail",
		Summary:     "one board's full read model (rows, header, derived analytics, validate findings, loaded_at) from its load-time snapshot",
	},
	"GET /api/events/{slug}": {
		OperationID: "getBoardEvents",
		Summary:     "one board's event rows with the detail ladder already applied, from its load-time snapshot",
	},
}

// Registry is the derived description of the binary's surface.
type Registry struct {
	CLI   []CLICommand
	HTTP  []HTTPRoute
	Main  string // path of the main.go the CLI verbs were derived from
	Serve string // path of the serve.go the HTTP routes were derived from
}

// Load derives the registry from the named sources. Empty paths mean "use
// the repo-relative defaults" (cmd/boardctl/main.go, cmd/boardctl/serve.go)
// resolved against root.
func Load(root, mainPath, servePath string) (Registry, error) {
	if mainPath == "" {
		mainPath = "cmd/boardctl/main.go"
	}
	if servePath == "" {
		servePath = "cmd/boardctl/serve.go"
	}
	mainFile := joinRoot(root, mainPath)
	serveFile := joinRoot(root, servePath)

	verbs, err := DeriveCLI(mainFile)
	if err != nil {
		return Registry{}, err
	}
	routes, err := DeriveHTTP(serveFile)
	if err != nil {
		return Registry{}, err
	}
	return Registry{CLI: verbs, HTTP: routes, Main: mainFile, Serve: serveFile}, nil
}

func joinRoot(root, p string) string {
	if root == "" {
		return p
	}
	return root + "/" + p
}

// DeriveCLI reads mainGoPath and returns run()'s command verbs in switch
// order, with curated summaries attached.
func DeriveCLI(mainGoPath string) ([]CLICommand, error) {
	body, err := os.ReadFile(mainGoPath)
	if err != nil {
		return nil, fmt.Errorf("describe: %w", err)
	}
	names, err := deriveSwitchVerbs(string(body))
	if err != nil {
		return nil, fmt.Errorf("describe: %s: %w", mainGoPath, err)
	}
	out := make([]CLICommand, 0, len(names))
	for _, n := range names {
		out = append(out, CLICommand{Name: n, Summary: cliSummaries[n]})
	}
	return out, nil
}

// DeriveHTTP reads serveGoPath and returns the routes wired in
// serveServer.routes(), in registration order, with curated operationIds
// and summaries attached.
func DeriveHTTP(serveGoPath string) ([]HTTPRoute, error) {
	body, err := os.ReadFile(serveGoPath)
	if err != nil {
		return nil, fmt.Errorf("describe: %w", err)
	}
	wired, err := deriveRoutes(string(body))
	if err != nil {
		return nil, fmt.Errorf("describe: %s: %w", serveGoPath, err)
	}
	out := make([]HTTPRoute, 0, len(wired))
	for _, r := range wired {
		curated, ok := httpOperations[r.Method+" "+r.Path]
		if ok {
			r.OperationID = curated.OperationID
			r.Summary = curated.Summary
		}
		out = append(out, r)
	}
	return out, nil
}

var (
	switchCaseRe = regexp.MustCompile(`case\s+"([^"]+)"`)
	switchVerbRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	routeRe      = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+)\s+([^"]+)"`)
	routesFnRe   = regexp.MustCompile(`func \(s \*serveServer\) routes\(\)`)
)

// deriveSwitchVerbs extracts the command verbs from a main.go-shaped body:
// locate `switch cmd {`, brace-match the switch, take every `case "X":`
// whose body does not print usageText directly (the help case) and whose X
// is verb-shaped. Pure function over the source text so tests can feed
// synthetic bodies.
func deriveSwitchVerbs(body string) ([]string, error) {
	lines := strings.Split(body, "\n")
	switchIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "switch cmd {") {
			if switchIdx >= 0 {
				return nil, fmt.Errorf("more than one 'switch cmd {' (lines %d and %d) — re-anchor the registry derivation", switchIdx+1, i+1)
			}
			switchIdx = i
		}
	}
	if switchIdx < 0 {
		return nil, fmt.Errorf("no 'switch cmd {' found — run()'s command dispatch moved or was renamed; re-anchor internal/describe")
	}
	end, err := braceEnd(lines, switchIdx)
	if err != nil {
		return nil, err
	}

	var verbs []string
	for i := switchIdx; i <= end; i++ {
		m := switchCaseRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		bodyEnd := end
		for j := i + 1; j <= end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.HasPrefix(trimmed, "case ") || strings.HasPrefix(trimmed, "default:") {
				bodyEnd = j - 1
				break
			}
		}
		caseBody := strings.Join(lines[i+1:bodyEnd+1], "\n")
		if strings.Contains(caseBody, "usageText") {
			continue // prints top-level usage itself (help); not a command
		}
		if switchVerbRe.MatchString(m[1]) {
			verbs = append(verbs, m[1])
		}
	}
	if len(verbs) == 0 {
		return nil, fmt.Errorf("switch starting at line %d yielded zero command cases — the derivation broke", switchIdx+1)
	}
	return verbs, nil
}

// deriveRoutes extracts the mux registrations from a serve.go-shaped body:
// locate `func (s *serveServer) routes()`, brace-match the function, take
// every `mux.HandleFunc("<METHOD> <path>", ...)` in order.
func deriveRoutes(body string) ([]HTTPRoute, error) {
	lines := strings.Split(body, "\n")
	fnIdx := -1
	for i, l := range lines {
		if routesFnRe.MatchString(l) {
			if fnIdx >= 0 {
				return nil, fmt.Errorf("more than one serveServer routes() definition (lines %d and %d) — re-anchor the registry derivation", fnIdx+1, i+1)
			}
			fnIdx = i
		}
	}
	if fnIdx < 0 {
		return nil, fmt.Errorf("no `func (s *serveServer) routes()` found — the serve route wiring moved or was renamed; re-anchor internal/describe")
	}
	end, err := braceEnd(lines, fnIdx)
	if err != nil {
		return nil, err
	}

	var routes []HTTPRoute
	for i := fnIdx; i <= end; i++ {
		m := routeRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		routes = append(routes, HTTPRoute{Method: m[1], Path: m[2]})
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("routes() starting at line %d yielded zero mux.HandleFunc registrations — the derivation broke", fnIdx+1)
	}
	return routes, nil
}

// braceEnd finds the line where the brace block opened on/after line start
// (0-indexed) closes again, by depth counting. Returns an error when the
// block never closes — a truncated or heavily refactored source must fail
// loud, not yield a partial census.
func braceEnd(lines []string, start int) (int, error) {
	depth := 0
	opened := false
	for j := start; j < len(lines); j++ {
		depth += strings.Count(lines[j], "{")
		if strings.Contains(lines[j], "{") {
			opened = true
		}
		depth -= strings.Count(lines[j], "}")
		if opened && depth <= 0 {
			return j, nil
		}
	}
	return 0, fmt.Errorf("brace block opened at line %d never closes — source truncated or too unusual to derive from", start+1)
}
