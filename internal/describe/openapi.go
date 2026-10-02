package describe

import (
	"fmt"
	"sort"
	"strings"
)

// SpecServerURL is the servers[].url published in the generated contract.
// The serve surface binds the loopback addresses of --addr only
// (127.0.0.1, localhost, ::1; non-loopback binds are refused in cmdServe),
// so the default absolute loopback URL is the only dereferenceable stand-in.
// muster falls back to servers[].url when --base-url is omitted, and a
// RELATIVE server URL is NOT dereferenceable from a local-file spec (muster
// only resolves relative server URLs against an absolute http(s) spec
// source URL — pkg/openapi.ResolveRelativeServerURL), so a relative entry
// here would strand every generated command. Consumers pointing at any
// other loopback spelling override with --base-url / MUSTER_BASE_URL.
const SpecServerURL = "http://127.0.0.1:8787"

// SpecVersion is the contract's own info.version — bumped when the served
// CONTRACT changes shape (a new route, a schema change). It tracks the
// contract, not the binary; boardctl's release version is a different
// version line and is not stamped here.
const SpecVersion = "0.1.0"

// RenderOpenAPI renders the registry's HTTP surface as an OpenAPI 3.0.3
// YAML document: one path item per route, operationId + summary + schemas
// on every operation. Routes are sorted by path then method for stable
// output (the mux registration order is wiring order, not document order).
// Every described operation must carry a curated operationId (the
// httpOperations table); a route missing one fails the render — the
// contract never ships an operation muster cannot name a verb for.
func RenderOpenAPI(reg Registry) (string, error) {
	if len(reg.HTTP) == 0 {
		return "", fmt.Errorf("render: registry carries zero HTTP routes — derivation broken")
	}
	for _, r := range reg.HTTP {
		if r.OperationID == "" {
			return "", fmt.Errorf("render: %s %s has no curated operationId in internal/describe — add it to httpOperations (fail-loud: an unnamed operation cannot become a muster verb)", r.Method, r.Path)
		}
	}

	var b strings.Builder
	b.WriteString("# GENERATED FILE — do not edit by hand.\n")
	b.WriteString("#\n")
	b.WriteString("# Generation rule (boardctl's contract doctrine): this document is\n")
	b.WriteString("# GENERATED from the live route wiring via internal/describe (the\n")
	b.WriteString("# registry) by `go run ./cmd/gendocs`. The registry re-derives the\n")
	b.WriteString("# surface from cmd/boardctl/serve.go's serveServer.routes() mux\n")
	b.WriteString("# registrations and run()'s command switch; a hand-edit here is\n")
	b.WriteString("# overwritten by the next regeneration and the parity gate\n")
	b.WriteString("# (internal/speccheck) fails a document that drifted from the wiring.\n")
	b.WriteString("#\n")
	b.WriteString("# Consumed by muster (github.com/wojons/muster): openapi-cli generates\n")
	b.WriteString("# one verb per operationId below; openapi-mcp exposes them as tools.\n")
	b.WriteString("# Regenerate + verify after every serve-surface change:\n")
	b.WriteString("#\n")
	b.WriteString("#\tgo run ./cmd/gendocs && go test ./internal/speccheck\n")
	b.WriteString("#\n")
	b.WriteString("# See docs/muster/quickstart.md for the consumer wiring.\n")
	b.WriteString("openapi: 3.0.3\n")
	b.WriteString("info:\n")
	b.WriteString("  title: boardctl serve\n")
	fmt.Fprintf(&b, "  description: |\n")
	b.WriteString("    Loopback uploader/report HTTP surface of `boardctl serve` (BT-021),\n")
	b.WriteString("    published as an OpenAPI 3.0.3 contract for muster consumption\n")
	b.WriteString("    (BT-062). serve is loopback-only by design: it refuses any\n")
	b.WriteString("    non-loopback --addr host, keeps no auth, and writes nothing to the\n")
	b.WriteString("    uploaded board files. Board files themselves are JSONL\n")
	b.WriteString("    (tasks.jsonl / events.jsonl / board.jsonl); the report is a\n")
	b.WriteString("    self-contained HTML page.\n")
	fmt.Fprintf(&b, "  version: %s\n", SpecVersion)
	fmt.Fprintf(&b, "servers:\n")
	fmt.Fprintf(&b, "  - url: %s\n", SpecServerURL)
	b.WriteString("    description: |\n")
	b.WriteString("      Default loopback listener of `boardctl serve` (--addr default\n")
	b.WriteString("      127.0.0.1:8787). ABSOLUTE on purpose: muster dereferences\n")
	b.WriteString("      servers[].url as-is when --base-url is omitted, and a relative\n")
	b.WriteString("      server URL cannot be dereferenced from a local-file spec. Override\n")
	b.WriteString("      with --base-url (openapi-cli) or the mcpServers args (openapi-mcp)\n")
	b.WriteString("      when the server runs on another loopback spelling or port.\n")
	b.WriteString("tags:\n")
	b.WriteString("  - name: uploader\n")
	b.WriteString("    description: browser-facing uploader form and report rendering\n")
	b.WriteString("  - name: boards\n")
	b.WriteString("    description: live board inventory of the running serve instance\n")
	b.WriteString("paths:\n")

	routes := append([]HTTPRoute(nil), reg.HTTP...)
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Path != routes[j].Path {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Method < routes[j].Method
	})

	sortedPaths := make([]string, 0, len(routes))
	seen := map[string]bool{}
	for _, r := range routes {
		if !seen[r.Path] {
			seen[r.Path] = true
			sortedPaths = append(sortedPaths, r.Path)
		}
	}

	for _, p := range sortedPaths {
		fmt.Fprintf(&b, "  %s:\n", quoteYAMLKey(p))
		for _, r := range routes {
			if r.Path != p {
				continue
			}
			writeOperation(&b, r)
		}
	}

	writeComponents(&b)
	return b.String(), nil
}

// writeComponents renders the named schemas referenced by the operations.
// They describe the wire shapes as they EXIST in the code:
//
//   - BoardUpload mirrors handleUpload's ParseMultipartForm consumption:
//     field `zip` (zero or one .zip part, serve.go takes zparts[0]) and
//     field `files` (the webkitdirectory folder upload — every file part
//     NOT named zip, relative paths preserved in the wire filenames). At
//     least one part is required (an all-empty upload answers 400). The
//     filenames are binary content, hence type: string format: binary.
//   - ApiBoard mirrors cmd/boardctl.serveServer's apiBoard struct
//     field-for-field, including its json tags (name, slug, topology,
//     task_total, done, open) plus the BT-075 additive validate_pill and
//     loaded_at fields (R2: additive only, every existing key
//     load-bearing).
//   - BoardDetail / ApiEvent / ApiError mirror the BT-075 read model of
//     GET /api/board/{slug}, GET /api/events/{slug} and the R8 JSON 404.
func writeComponents(b *strings.Builder) {
	b.WriteString("components:\n")
	b.WriteString("  schemas:\n")
	b.WriteString("    BoardUpload:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: |\n")
	b.WriteString("        multipart/form-data body of POST `/`. Exactly one of the two\n")
	b.WriteString("        shapes is required: a single `zip` part (a .zip archive of one\n")
	b.WriteString("        or more board directories, max 512 MB) OR a `files` folder\n")
	b.WriteString("        upload (one part per file, wire filenames carrying the\n")
	b.WriteString("        directory-relative paths). A `zip` part wins when both are\n")
	b.WriteString("        present (handleUpload checks zip first).\n")
	b.WriteString("      properties:\n")
	b.WriteString("        zip:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          format: binary\n")
	b.WriteString("          description: one .zip archive containing one or more board directories (a directory holding BOTH tasks.jsonl and events.jsonl)\n")
	b.WriteString("        files:\n")
	b.WriteString("          type: array\n")
	b.WriteString("          description: webkitdirectory folder upload — every file under the chosen directory, directory-relative paths preserved in the part filenames\n")
	b.WriteString("          items:\n")
	b.WriteString("            type: string\n")
	b.WriteString("            format: binary\n")
	b.WriteString("    ApiBoard:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: one entry of GET /api/boards (mirrors cmd/boardctl apiBoard's json wire shape)\n")
	b.WriteString("      properties:\n")
	b.WriteString("        name:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: board display name (header project, else the directory base name)\n")
	b.WriteString("        slug:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: report-display slug of the name\n")
	b.WriteString("        topology:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: board topology (A = board.jsonl header + tasks.jsonl, B = tasks.jsonl only)\n")
	b.WriteString("        task_total:\n")
	b.WriteString("          type: integer\n")
	b.WriteString("          description: non-fixture task rows counted by the report payload derivation\n")
	b.WriteString("          format: int64\n")
	b.WriteString("        done:\n")
	b.WriteString("          type: integer\n")
	b.WriteString("          description: complete task count\n")
	b.WriteString("          format: int64\n")
	b.WriteString("        open:\n")
	b.WriteString("          type: integer\n")
	b.WriteString("          description: open task count\n")
	b.WriteString("          format: int64\n")
	b.WriteString("        validate_pill:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: BT-075 additive — snapshot-time validate verdict PASS/WARN/FAIL (R2 keeps every pre-existing key load-bearing)\n")
	b.WriteString("        loaded_at:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: BT-075 additive — RFC3339 time the board's snapshot was loaded (R9)\n")
	b.WriteString("    BoardDetail:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: one board's full read model of GET /api/board/{slug} (BT-075 R7) — values are exactly what render.BuildBoards produces for the snapshot\n")
	b.WriteString("      properties:\n")
	b.WriteString("        name:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        slug:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        topology:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        header:\n")
	b.WriteString("          type: object\n")
	b.WriteString("          nullable: true\n")
	b.WriteString("          description: the raw board header row (null when the board has none)\n")
	b.WriteString("        tasks:\n")
	b.WriteString("          type: array\n")
	b.WriteString("          description: raw task rows, verbatim as the report payload embeds them\n")
	b.WriteString("          items:\n")
	b.WriteString("            type: object\n")
	b.WriteString("        events:\n")
	b.WriteString("          type: array\n")
	b.WriteString("          description: raw event rows, verbatim as the report payload embeds them\n")
	b.WriteString("          items:\n")
	b.WriteString("            type: object\n")
	b.WriteString("        fixture_ids:\n")
	b.WriteString("          type: array\n")
	b.WriteString("          items:\n")
	b.WriteString("            type: string\n")
	b.WriteString("        parse_warnings:\n")
	b.WriteString("          type: object\n")
	b.WriteString("          description: per-file parse-warning counts plus the first notes (analytics 2.7)\n")
	b.WriteString("        derived:\n")
	b.WriteString("          type: object\n")
	b.WriteString("          description: the board-report/v1 derived analytics block (burndown, burnup, velocity, cycle, streaks, tick_health, work_clock, model_share, status/priority counts)\n")
	b.WriteString("        validate:\n")
	b.WriteString("          type: object\n")
	_ = b // description rendered below via writeValidateSchema
	writeValidateSchema(b)
	b.WriteString("        loaded_at:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: RFC3339 time this snapshot was loaded (R9)\n")
	b.WriteString("    ApiEvent:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: one event row of GET /api/events/{slug} with the detail ladder already applied\n")
	b.WriteString("      properties:\n")
	b.WriteString("        id:\n")
	b.WriteString("          description: event id as stored (number or string)\n")
	b.WriteString("        timestamp:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        event_type:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        task_id:\n")
	b.WriteString("          nullable: true\n")
	b.WriteString("          description: top-level task linkage (null for board-level events)\n")
	b.WriteString("        actor:\n")
	b.WriteString("          type: string\n")
	b.WriteString("        detail:\n")
	b.WriteString("          nullable: true\n")
	b.WriteString("          description: the DECODED detail (object/array; raw string only on ladder fall-through)\n")
	b.WriteString("        tick_number:\n")
	b.WriteString("          nullable: true\n")
	b.WriteString("    ApiError:\n")
	b.WriteString("      type: object\n")
	b.WriteString("      description: the R8 JSON 404 body of the read-only endpoints\n")
	b.WriteString("      properties:\n")
	b.WriteString("        error:\n")
	b.WriteString("          type: string\n")
	b.WriteString("          description: names the requested slug\n")
}

// writeValidateSchema renders the validate block of BoardDetail (the
// uiValidate wire shape): pill, counts, findings, census, sweep census.
func writeValidateSchema(b *strings.Builder) {
	w := func(s string) { b.WriteString(s) }
	w("          description: the snapshot-time validate result per R13 — pill + findings + census + sweep READ census\n")
	w("          properties:\n")
	w("            pill:\n")
	w("              type: string\n")
	w("              description: PASS (0 errors, any warnings), WARN (warnings only), FAIL (>=1 error)\n")
	w("            errors:\n")
	w("              type: integer\n")
	w("            warnings:\n")
	w("              type: integer\n")
	w("            findings:\n")
	w("              type: array\n")
	w("              items:\n")
	w("                type: object\n")
	w("                properties:\n")
	w("                  level:\n")
	w("                    type: string\n")
	w("                    description: error | warn\n")
	w("                  message:\n")
	w("                    type: string\n")
	w("                    description: verbatim validate message with line numbers (F9)\n")
	w("            census:\n")
	w("              type: string\n")
	w("              description: the key-uniformity census line, verbatim (F9/BT-056)\n")
	w("            sweep_census:\n")
	w("              type: object\n")
	w("              description: REVIEW-BOARDCTL-001 read census — no decisions are ever applied\n")
	w("              properties:\n")
	w("                rows:\n")
	w("                  type: integer\n")
	w("                off_vocabulary:\n")
	w("                  type: integer\n")
	w("                fixable_by_normalize:\n")
	w("                  type: integer\n")
	w("                need_explicit_decision:\n")
	w("                  type: integer\n")
	w("                unknown_rows:\n")
	w("                  type: array\n")
	w("                  items:\n")
	w("                    type: string\n")
	w("                  description: ids of rows carrying unknown statuses, named (never guessed at)\n")
	w("                decisions_note:\n")
	w("                  type: string\n")
}

// writeOperation renders one operation block. The three operations are the
// whole serve surface today; their request/response schemas are described
// as they EXIST (per BT-062: describe the surface, do not redesign it).
// A fourth route reaching this function without a schema arm here fails the
// build of the contract (unknown verb -> error), so schemas can never
// silently lag the route table.
func writeOperation(b *strings.Builder, r HTTPRoute) {
	key := strings.ToLower(r.Method)
	fmt.Fprintf(b, "    %s:\n", key)
	fmt.Fprintf(b, "      operationId: %s\n", r.OperationID)
	if r.Summary != "" {
		fmt.Fprintf(b, "      summary: %s\n", r.Summary)
	}
	switch r.OperationID {
	case "getWebUI":
		fmt.Fprintf(b, "      tags: [uploader]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Serves the BT-075 web UI: one self-contained HTML document (inline\n")
		b.WriteString("        CSS+JS, no external assets, R17) with the initial snapshot island\n")
		b.WriteString("        embedded (R18) so the overview renders even when the /api routes\n")
		b.WriteString("        fail. Read-only: the UI holds zero write surfaces (D3/R20); the\n")
		b.WriteString("        uploader stays at `/` and the UI only links to it (R2). The UI is\n")
		b.WriteString("        provide-dont-force: nothing schedules, hooks or requires it (D6).\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the web UI page (self-contained HTML with the embedded snapshot island)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/html:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
	case "getBoardDetail":
		fmt.Fprintf(b, "      tags: [boards]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Answers with one board's full read model (R7): header, task rows,\n")
		b.WriteString("        events, fixture ids, parse warnings, the derived analytics block\n")
		b.WriteString("        (the same values render.BuildBoards produces for the snapshot),\n")
		b.WriteString("        the validate findings (level + verbatim message with line\n")
		b.WriteString("        numbers), the key-uniformity census line, the sweep-status READ\n")
		b.WriteString("        census, and loaded_at (R9). Identity key: the report slug. The\n")
		b.WriteString("        snapshot was computed once at load time (the -C resolve or an\n")
		b.WriteString("        upload): files changing on disk afterwards never change this\n")
		b.WriteString("        answer until a new upload or a serve restart. Unknown slug:\n")
		b.WriteString("        404 with a JSON body (R8).\n")
		fmt.Fprintf(b, "      parameters:\n")
		fmt.Fprintf(b, "        - name: slug\n")
		fmt.Fprintf(b, "          in: path\n")
		fmt.Fprintf(b, "          required: true\n")
		fmt.Fprintf(b, "          description: the board's report slug (the slugified report name)\n")
		fmt.Fprintf(b, "          schema:\n")
		fmt.Fprintf(b, "            type: string\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the board's snapshot read model\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            application/json:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                $ref: '#/components/schemas/BoardDetail'\n")
		fmt.Fprintf(b, "        '404':\n")
		fmt.Fprintf(b, "          description: unknown slug (JSON body naming the requested slug)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            application/json:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                $ref: '#/components/schemas/ApiError'\n")
	case "getBoardEvents":
		fmt.Fprintf(b, "      tags: [boards]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Answers with one board's event rows (R7), each carrying the\n")
		b.WriteString("        analytics 2.4 detail ladder ALREADY APPLIED (R6: the decoding is\n")
		b.WriteString("        Go's job): detail is an object/array when stored as one or\n")
		b.WriteString("        wrapped as a JSON string or strict base64-wrapped JSON, and the\n")
		b.WriteString("        raw string only on ladder fall-through (kept inert downstream).\n")
		b.WriteString("        Served from the same load-time snapshot as GET\n")
		b.WriteString("        /api/board/{slug}. Unknown slug: 404 with a JSON body (R8).\n")
		fmt.Fprintf(b, "      parameters:\n")
		fmt.Fprintf(b, "        - name: slug\n")
		fmt.Fprintf(b, "          in: path\n")
		fmt.Fprintf(b, "          required: true\n")
		fmt.Fprintf(b, "          description: the board's report slug (the slugified report name)\n")
		fmt.Fprintf(b, "          schema:\n")
		fmt.Fprintf(b, "            type: string\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the board's event rows with decoded details\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            application/json:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: array\n")
		fmt.Fprintf(b, "                items:\n")
		fmt.Fprintf(b, "                  $ref: '#/components/schemas/ApiEvent'\n")
		fmt.Fprintf(b, "        '404':\n")
		fmt.Fprintf(b, "          description: unknown slug (JSON body naming the requested slug)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            application/json:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                $ref: '#/components/schemas/ApiError'\n")
	case "getUploaderForm":
		fmt.Fprintf(b, "      tags: [uploader]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Serves the self-contained uploader page (inline CSS+JS, no\n")
		b.WriteString("        external fetches). A browser form POSTs to `/` with\n")
		b.WriteString("        multipart/form-data: field `files` (webkitdirectory folder\n")
		b.WriteString("        upload, relative paths preserved) or field `zip` (one .zip\n")
		b.WriteString("        archive). Paths other than `/` answer 404 via mux routing.\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the uploader HTML page\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/html:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
	case "uploadBoardReport":
		fmt.Fprintf(b, "      tags: [uploader]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Accepts a multipart/form-data upload of a board folder\n")
		b.WriteString("        (webkitdirectory: every file under the chosen directory, one\n")
		b.WriteString("        part per file, relative paths preserved in the filenames) or a\n")
		b.WriteString("        single .zip part. Every directory containing BOTH\n")
		b.WriteString("        tasks.jsonl and events.jsonl registers as a board (Topology A\n")
		b.WriteString("        or B); a `board.jsonl` header row is honored for Topology A.\n")
		b.WriteString("        Caps: 512 MB payload (HTTP 413 above), 50 boards per upload\n")
		b.WriteString("        (HTTP 413 above). The response IS the rendered multi-board\n")
		b.WriteString("        HTML report; nothing is written to the uploaded board files.\n")
		b.WriteString("        Note: an upload that registers 0 boards while a -C board is\n")
		b.WriteString("        loaded still answers 200 with the -C board's report plus a\n")
		b.WriteString("        visible upload_notice banner (BT-027) — it does not error.\n")
		fmt.Fprintf(b, "      requestBody:\n")
		fmt.Fprintf(b, "        required: true\n")
		fmt.Fprintf(b, "        content:\n")
		fmt.Fprintf(b, "          multipart/form-data:\n")
		fmt.Fprintf(b, "            schema:\n")
		fmt.Fprintf(b, "              $ref: '#/components/schemas/BoardUpload'\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the rendered multi-board analytics report (self-contained HTML; carries an upload_notice banner when the upload registered 0 boards but a -C board rendered)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/html:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
		fmt.Fprintf(b, "        '400':\n")
		fmt.Fprintf(b, "          description: bad multipart form, unreadable zip, no files, or no board found in the upload (plain-text error body)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/plain:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
		fmt.Fprintf(b, "        '413':\n")
		fmt.Fprintf(b, "          description: payload exceeds the 512 MB cap, or the upload registers more than 50 boards\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/plain:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
	case "listLoadedBoards":
		fmt.Fprintf(b, "      tags: [boards]\n")
		fmt.Fprintf(b, "      description: |\n")
		b.WriteString("        Answers with the boards currently loaded by the running serve\n")
		b.WriteString("        instance: the optional -C board plus every board registered by\n")
		b.WriteString("        uploads so far (superseded re-uploads dropped). Board task\n")
		b.WriteString("        totals come from the same payload derivation as the report\n")
		b.WriteString("        (fixture exclusion, last-row-wins dedup), so the numbers match\n")
		b.WriteString("        what the report page shows.\n")
		fmt.Fprintf(b, "      responses:\n")
		fmt.Fprintf(b, "        '200':\n")
		fmt.Fprintf(b, "          description: the loaded boards (possibly an empty array)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            application/json:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: array\n")
		fmt.Fprintf(b, "                items:\n")
		fmt.Fprintf(b, "                  $ref: '#/components/schemas/ApiBoard'\n")
		fmt.Fprintf(b, "        '500':\n")
		fmt.Fprintf(b, "          description: board payload could not be built (plain-text error body)\n")
		fmt.Fprintf(b, "          content:\n")
		fmt.Fprintf(b, "            text/plain:\n")
		fmt.Fprintf(b, "              schema:\n")
		fmt.Fprintf(b, "                type: string\n")
	default:
		// Unreachable while httpOperations only contains curated entries;
		// a future route must add both an httpOperations entry (checked
		// above) and a schema arm here, in that order.
		//nolint:staticcheck // ST1005: operator-facing error, kept lowercase-free intentionally
		panic(fmt.Sprintf("render: no schema arm for operationId %q — add one to internal/describe.writeOperation", r.OperationID))
	}
}

// quoteYAMLKey renders a path as a YAML mapping key. Paths begin with `/`
// (a YAML plain scalar can start with / — but we quote defensively so a
// future path with YAML-special leading characters cannot break the
// document), using single-quote style compatible with muster's YAML parse.
func quoteYAMLKey(p string) string {
	return "'" + strings.ReplaceAll(p, "'", "''") + "'"
}
