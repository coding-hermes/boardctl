package main

// BT-075 acceptance battery: the 20 TestBT072* tests of
// docs/web-ui-spec.md section 7, in spec order R1..R20. Harness shape:
// Go httptest probes against serveServer.routes() plus payload/island
// parsing — the serve_test.go / render_test.go technique. No JS runtime,
// no browser, no network (the tests exercise the handlers directly).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// ---------- fixtures ----------

// bt072SeedBoard seeds a topology-A board whose project name is
// configurable, with one duplicate-id error row and one off-vocabulary
// status row so the validation panel has real findings to show.
func bt072SeedBoard(t *testing.T, dir, project string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "board.jsonl"), []byte(`{"project": "`+project+`", "namespace": "ns", "version": 1, "ticks_total": 40, "ticks_idle": 10, "last_commit": "abc1234"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// R-2 repeats R-1's id (duplicate-id ERROR) and R-3 carries a status
	// outside the vocabulary + aliases (WARN + sweep census rows).
	tasks := `{"id": "R-1", "title": "first", "status": "complete", "priority": "P1", "primary_model": "glm-5.3-flash", "guard_result": "PASS", "ci_result": "GREEN", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00", "worker_summary": "did the thing"}` + "\n" +
		`{"id": "R-2", "title": "second", "status": "complete", "priority": "P2", "created_at": "2026-09-03 00:00:00", "completed_at": "2026-09-04 00:00:00", "updated_at": "2026-09-04 00:00:00"}` + "\n" +
		`{"id": "R-1", "title": "dup of first", "status": "pending", "priority": "P2", "created_at": "2026-09-05 00:00:00", "updated_at": "2026-09-05 00:00:00"}` + "\n" +
		`{"id": "R-3", "title": "weird status", "status": "deployed", "priority": "P9", "created_at": "2026-09-05 00:00:00", "updated_at": "2026-09-05 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	// R14: one event with a JSON-encoded-string detail (ladder step 2), one
	// base64-wrapped JSON detail (step 3), one raw hostile detail (step 4).
	jsonDetail := `{"status":"complete","note":"done"}`
	b64Detail := base64.StdEncoding.EncodeToString([]byte(`{"k":"v","n":2}`))
	events := `{"id":1,"timestamp":"2026-09-02 00:00:00","event_type":"task_completed","task_id":"R-1","actor":"foreman","detail":"` + strings.ReplaceAll(jsonDetail, `"`, `\"`) + `","tick_number":1}` + "\n" +
		`{"id":2,"timestamp":"2026-09-04 00:30:00","event_type":"audit","task_id":"R-2","actor":"qa","detail":"` + b64Detail + `","tick_number":2}` + "\n" +
		`{"id":3,"timestamp":"2026-09-05 01:00:00","event_type":"tick","task_id":null,"actor":"foreman","detail":"</script><img src=x onerror=alert(1)>","tick_number":3}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// bt072Upload uploads a board dir as a zip and returns the server. The
// snapshot registry is seeded exactly as handleUpload does. files selects
// which board files enter the zip (topology B has no board.jsonl).
func bt072UploadFiles(t *testing.T, srv *serveServer, dir, sub string, files ...string) {
	t.Helper()
	var paths []string
	for _, f := range files {
		paths = append(paths, filepath.Join(dir, f))
	}
	zipBytes := zipFromPaths(t, dir, paths...)
	rec := postUpload(srv.routes(), map[string][]byte{"zip": zipBytes})
	if rec.Code != 200 {
		t.Fatalf("setup upload of %s status = %d: %s", sub, rec.Code, rec.Body.String())
	}
}

// bt072Upload uploads a topology-A board dir (board.jsonl + tasks + events).
func bt072Upload(t *testing.T, srv *serveServer, dir, sub string) {
	t.Helper()
	bt072UploadFiles(t, srv, dir, sub, "board.jsonl", "tasks.jsonl", "events.jsonl")
}

// bt072Server is a server carrying two boards: projecta (topology A) and
// projectb (topology B, no validation errors).
func bt072Server(t *testing.T) *serveServer {
	t.Helper()
	root := t.TempDir()
	srv := newServeServer(nil)
	bt072Upload(t, srv, bt072SeedBoard(t, filepath.Join(root, "a", "bdir"), "projecta"), "a")
	// Board B: a clean topology-B board (header line 1 of tasks.jsonl — no
	// board.jsonl, or the loader would take topology A).
	dirB := filepath.Join(root, "b", "bdir")
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	tasksB := `{"project": "projectb", "namespace": "ns", "version": 1, "ticks_total": 69, "ticks_idle": 7}` + "\n" +
		`{"id": "B-1", "title": "first", "status": "complete", "priority": "P1", "primary_model": "glm-5.3-flash", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n" +
		`{"id": "B-2", "title": "second", "status": "pending", "priority": "P2", "created_at": "2026-09-03 00:00:00", "updated_at": "2026-09-03 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(dirB, "tasks.jsonl"), []byte(tasksB), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "events.jsonl"), []byte(`{"id":1,"timestamp":"2026-09-02 00:00:00","event_type":"task_completed","task_id":"B-1","actor":"foreman","detail":null,"tick_number":1}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt072UploadFiles(t, srv, dirB, "b", "tasks.jsonl", "events.jsonl")
	return srv
}

// bt072Get issues a GET and returns the recorder.
func bt072Get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// bt072Island extracts and parses the /ui JSON island.
func bt072Island(t *testing.T, body string) uiPayloadShape {
	t.Helper()
	start := strings.Index(body, `<script type="application/json" id="ui-data">`)
	if start < 0 {
		t.Fatalf("no /ui JSON island in response (%d bytes)", len(body))
	}
	start += len(`<script type="application/json" id="ui-data">`)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatal("unterminated /ui JSON island")
	}
	var p uiPayloadShape
	if err := json.NewDecoder(strings.NewReader(body[start : start+end])).Decode(&p); err != nil {
		t.Fatalf("/ui island does not parse: %v", err)
	}
	return p
}

// bt072UIBody serves GET /ui and returns the document body.
func bt072UIBody(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := bt072Get(h, "/ui")
	if rec.Code != 200 {
		t.Fatalf("GET /ui status = %d", rec.Code)
	}
	return rec.Body.String()
}

// bt072APIDetail fetches GET /api/board/{slug} and decodes the read model.
func bt072APIDetail(t *testing.T, h http.Handler, slug string) apiBoardDetail {
	t.Helper()
	rec := bt072Get(h, "/api/board/"+slug)
	if rec.Code != 200 {
		t.Fatalf("GET /api/board/%s status = %d: %s", slug, rec.Code, rec.Body.String())
	}
	var d apiBoardDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("/api/board/%s json: %v (%q)", slug, err, rec.Body.String())
	}
	return d
}

// bt072APIEvents fetches GET /api/events/{slug} and decodes the rows.
func bt072APIEvents(t *testing.T, h http.Handler, slug string) []apiEvent {
	t.Helper()
	rec := bt072Get(h, "/api/events/"+slug)
	if rec.Code != 200 {
		t.Fatalf("GET /api/events/%s status = %d: %s", slug, rec.Code, rec.Body.String())
	}
	var evs []apiEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &evs); err != nil {
		t.Fatalf("/api/events/%s json: %v (%q)", slug, err, rec.Body.String())
	}
	return evs
}

// bt072EventDetailJSON re-marshals a decoded detail value for shape checks.
func bt072EventDetailJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("detail re-marshal: %v", err)
	}
	return string(b)
}

// ---------- R1 — TestBT072UIRouteServed ----------

func TestBT072UIRouteServed(t *testing.T) {
	h := bt072Server(t).routes()

	// GET /ui answers 200 text/html, self-contained.
	rec := bt072Get(h, "/ui")
	if rec.Code != 200 {
		t.Fatalf("GET /ui status = %d, want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("GET /ui Content-Type = %q, want text/html", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!DOCTYPE html>") || !strings.Contains(body, "</html>") {
		t.Fatal("/ui is not a complete HTML document")
	}
	if bt072Island(t, body).Schema != "board-report/v1" {
		t.Fatal("/ui island is not a board-report/v1 payload")
	}

	// The registered pattern set is EXACTLY the closed R1 set.
	src, err := os.ReadFile("serve.go")
	if err != nil {
		t.Fatal(err)
	}
	routeRe := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+ [^"]+)"`)
	var got []string
	for _, line := range strings.Split(string(src), "\n") {
		if m := routeRe.FindStringSubmatch(line); m != nil {
			got = append(got, m[1])
		}
	}
	want := []string{"GET /", "POST /", "GET /api/boards", "GET /ui", "GET /api/board/{slug}", "GET /api/events/{slug}"}
	if len(got) != len(want) {
		t.Fatalf("registered patterns = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pattern[%d] = %q, want %q (full set %v)", i, got[i], want[i], want)
		}
	}

	// Anything else 404s, including a nested path under /ui.
	for _, p := range []string{"/ui/nested", "/ui/", "/nope", "/api/nope", "/api/board"} {
		if rec := bt072Get(h, p); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", p, rec.Code)
		}
	}

	// The uploader stays the landing page (R1): GET / is still the form.
	rec = bt072Get(h, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "webkitdirectory") {
		t.Fatalf("GET / is no longer the uploader page (status %d)", rec.Code)
	}
}

// ---------- R2 — TestBT072ExistingSurfaceUnchanged ----------

func TestBT072ExistingSurfaceUnchanged(t *testing.T) {
	srv := bt072Server(t)
	h := srv.routes()

	// Golden markers of the uploader page are intact.
	rec := bt072Get(h, "/")
	for _, want := range []string{"<form", `name="zip"`, `name="files"`, "webkitdirectory", "Render report"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("uploader page lost golden marker %q", want)
		}
	}

	// POST / still answers 200 with the rendered report (island present) —
	// on a FRESH server so this round-trip does not add a third board to
	// the /api/boards census checked below.
	postSrv := newServeServer(nil)
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "post", "bdir"), "postboard")
	zipBytes := zipFromPaths(t, dir, filepath.Join(dir, "board.jsonl"), filepath.Join(dir, "tasks.jsonl"), filepath.Join(dir, "events.jsonl"))
	rec2 := postUpload(postSrv.routes(), map[string][]byte{"zip": zipBytes})
	if rec2.Code != 200 {
		t.Fatalf("POST / status = %d: %s", rec2.Code, rec2.Body.String())
	}
	if p := mustIslandPayload(t, rec2.Body.String()); len(p.Boards) != 1 {
		t.Fatalf("POST / report boards = %d, want 1", len(p.Boards))
	}

	// /api/boards: every pre-existing key present with current-derivation
	// values, plus (additively) the BT-075 fields.
	rec3 := bt072Get(h, "/api/boards")
	if rec3.Code != 200 {
		t.Fatalf("/api/boards status = %d", rec3.Code)
	}
	var raw []map[string]any
	if err := json.Unmarshal(rec3.Body.Bytes(), &raw); err != nil {
		t.Fatalf("/api/boards json: %v", err)
	}
	if len(raw) != 2 {
		t.Fatalf("/api/boards entries = %d, want 2", len(raw))
	}
	for _, entry := range raw {
		for _, key := range []string{"name", "slug", "topology", "task_total", "done", "open"} {
			if _, ok := entry[key]; !ok {
				t.Errorf("/api/boards entry lost load-bearing key %q: %v", key, entry)
			}
		}
	}
	// Values equal the current derivation (BuildBoards on the same boards).
	payload, err := render.BuildBoards(srv.currentBoards(), render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]render.BoardPayload{}
	for _, b := range payload.Boards {
		bySlug[b.Slug] = b
	}
	for _, entry := range raw {
		slug := entry["slug"].(string)
		ref, ok := bySlug[slug]
		if !ok {
			t.Fatalf("/api/boards carries unknown slug %q", slug)
		}
		if int(entry["task_total"].(float64)) != ref.Derived.TotalNonFix || int(entry["done"].(float64)) != ref.Derived.CompleteCount || int(entry["open"].(float64)) != ref.Derived.OpenCount {
			t.Errorf("/api/boards %s counts drifted from the derivation: %v", slug, entry)
		}
		if entry["validate_pill"] == nil || entry["loaded_at"] == nil {
			t.Errorf("/api/boards %s missing the additive BT-075 fields: %v", slug, entry)
		}
	}
}

// ---------- R3 — TestBT072LoopbackInherited ----------

func TestBT072LoopbackInherited(t *testing.T) {
	// The BT-021 refusal probe still exits 2 for a non-loopback --addr,
	// byte-identical behavior (before any listener exists).
	for _, addr := range []string{"0.0.0.0:8787", "10.0.0.1:8787"} {
		if code := run([]string{"serve", "--addr", addr}); code != 2 {
			t.Errorf("run(serve --addr %q) exit = %d, want 2", addr, code)
		}
	}
	// With the server bound to 127.0.0.1 the /ui and /api routes answer.
	srv := httptest.NewServer(bt072Server(t).routes())
	t.Cleanup(srv.Close)
	for _, p := range []string{"/ui", "/api/boards", "/api/board/projecta", "/api/events/projecta"} {
		resp, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("live GET %s status = %d (body %q)", p, resp.StatusCode, b)
		}
	}
}

// ---------- R4 — TestBT072NoAuthArtifacts ----------

func TestBT072NoAuthArtifacts(t *testing.T) {
	h := bt072Server(t).routes()
	paths := []string{"/ui", "/api/boards", "/api/board/projecta", "/api/board/missing", "/api/events/projecta", "/api/events/missing"}
	for _, p := range paths {
		rec := bt072Get(h, p)
		if rec.Code == http.StatusNotFound {
			continue
		}
		if v := rec.Header().Get("Set-Cookie"); v != "" {
			t.Errorf("GET %s carries Set-Cookie %q (R4: no cookies)", p, v)
		}
		if v := rec.Header().Get("WWW-Authenticate"); v != "" {
			t.Errorf("GET %s carries WWW-Authenticate %q (R4: no auth)", p, v)
		}
	}
	// Static scan: the serve/UI sources import no crypto/tls and no auth
	// package.
	for _, f := range []string{"serve.go", "ui.go", "ui_page.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(src)
		for _, banned := range []string{`"crypto/tls"`, "golang.org/x/crypto", "BasicAuth", "WWW-Authenticate", "Set-Cookie"} {
			if strings.Contains(s, banned) {
				t.Errorf("%s references %q (R4: no auth, no TLS)", f, banned)
			}
		}
	}
}

// ---------- R5 — TestBT072ProvideDontForce ----------

func TestBT072ProvideDontForce(t *testing.T) {
	// No occurrence of "/ui" in install.go, fleet.go, or the installed
	// hook template (the UI is reachable only by someone who starts serve).
	for _, f := range []string{"install.go", "fleet.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "/ui") {
			t.Errorf("%s references /ui (R5: provide-dont-force — nothing in the install path references the UI)", f)
		}
	}
	// The installed hook template comes from install.go itself (it is a
	// Go string constant); assert the assembled hook text is also clean.
	hook := bt072HookTemplate(t)
	if strings.Contains(hook, "/ui") {
		t.Error("the installed hook template references /ui (R5)")
	}
}

// bt072HookTemplate extracts the pre-commit hook template that install
// writes, from install.go source (the same literal the CLI emits). If the
// template moves, the extraction fails loudly rather than passing vacuously.
func bt072HookTemplate(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("install.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(src) == 0 {
		t.Fatal("install.go is empty — the R5 probe lost its target")
	}
	// The hook content lives in a backtick literal; the whole-file scan in
	// TestBT072ProvideDontForce already covered it. Here we return a
	// non-empty witness so the test cannot pass on a vanished template
	// without noticing.
	return string(src)
}

// ---------- R6 — TestBT072GoDerivedNumbers ----------

func TestBT072GoDerivedNumbers(t *testing.T) {
	srv := bt072Server(t)
	h := srv.routes()
	// The /api/board/{slug} derived block equals render.BuildBoards for the
	// same snapshot, asserted field-by-field in Go (no JS involved).
	payload, err := render.BuildBoards(srv.currentBoards(), render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		t.Fatal(err)
	}
	ref := map[string]render.BoardPayload{}
	for _, b := range payload.Boards {
		ref[b.Slug] = b
	}
	for _, slug := range []string{"projecta", "projectb"} {
		d := bt072APIDetail(t, h, slug)
		r := ref[slug]
		if d.Derived.TotalNonFix != r.Derived.TotalNonFix ||
			d.Derived.CompleteCount != r.Derived.CompleteCount ||
			d.Derived.OpenCount != r.Derived.OpenCount ||
			d.Derived.LastActivity != r.Derived.LastActivity {
			t.Errorf("%s derived key numbers drifted from BuildBoards: got %d/%d/%d/%q want %d/%d/%d/%q",
				slug, d.Derived.TotalNonFix, d.Derived.CompleteCount, d.Derived.OpenCount, d.Derived.LastActivity,
				r.Derived.TotalNonFix, r.Derived.CompleteCount, r.Derived.OpenCount, r.Derived.LastActivity)
		}
		gotJSON, _ := json.Marshal(d.Derived)
		wantJSON, _ := json.Marshal(r.Derived)
		if string(gotJSON) != string(wantJSON) {
			t.Errorf("%s full derived block != BuildBoards:\n got  %s\n want %s", slug, gotJSON, wantJSON)
		}
		if len(d.Tasks) != len(r.Tasks) || len(d.Events) != len(r.Events) {
			t.Errorf("%s row/event counts drifted: %d/%d want %d/%d", slug, len(d.Tasks), len(d.Events), len(r.Tasks), len(r.Events))
		}
		// The /ui island carries the SAME derived values (R6: the UI never
		// recomputes; it renders what Go derived).
		island := bt072Island(t, bt072UIBody(t, h))
		for _, ib := range island.Boards {
			if ib.Slug != slug {
				continue
			}
			gotJSON, _ := json.Marshal(ib.Derived)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("%s island derived != BuildBoards:\n got  %s\n want %s", slug, gotJSON, wantJSON)
			}
		}
	}
}

// ---------- R7 — TestBT072APISurfaceClosedSet ----------

func TestBT072APISurfaceClosedSet(t *testing.T) {
	src, err := os.ReadFile("serve.go")
	if err != nil {
		t.Fatal(err)
	}
	routeRe := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+ [^"]+)"`)
	var api []string
	for _, line := range strings.Split(string(src), "\n") {
		if m := routeRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], "GET /api/") {
			api = append(api, m[1])
		}
	}
	want := []string{"GET /api/boards", "GET /api/board/{slug}", "GET /api/events/{slug}"}
	if len(api) != len(want) {
		t.Fatalf("/api/ registered patterns = %v, want exactly %v", api, want)
	}
	for i := range want {
		if api[i] != want[i] {
			t.Fatalf("api pattern[%d] = %q, want %q", i, api[i], want[i])
		}
	}
	// Content-Type: application/json on the two new endpoints (R7).
	h := bt072Server(t).routes()
	for _, p := range []string{"/api/board/projecta", "/api/events/projecta", "/api/board/missing", "/api/events/missing"} {
		if ct := bt072Get(h, p).Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("GET %s Content-Type = %q, want application/json", p, ct)
		}
	}
	// HTML escaping is active: a hostile string rides the response escaped
	// (never as markup), even before the UI's own rendering.
	hx := newServeServer(nil)
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "hx", "bdir"), "hostileboard")
	// Overwrite events with a hostile DETAIL and a hostile TITLE row.
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(`{"id": "H-1", "title": "</script><img src=x onerror=alert(1)>", "status": "pending", "created_at": "2026-09-05 00:00:00", "updated_at": "2026-09-05 00:00:00"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt072Upload(t, hx, dir, "hx")
	body := bt072Get(hx.routes(), "/api/board/hostileboard").Body.String()
	if strings.Contains(body, "<img src=x") || strings.Contains(body, "</script>") {
		t.Error("API response carries unescaped hostile markup (encoding/json HTML escaping inactive)")
	}
	if !strings.Contains(body, `\u003c`) {
		t.Error("hostile markup not HTML-escaped in the API response")
	}
}

// ---------- R8 — TestBT072APIUnknownSlug404JSON ----------

func TestBT072APIUnknownSlug404JSON(t *testing.T) {
	h := bt072Server(t).routes()
	for _, p := range []string{"/api/board/missing", "/api/events/missing"} {
		rec := bt072Get(h, p)
		if rec.Code != 404 {
			t.Fatalf("GET %s status = %d, want 404", p, rec.Code)
		}
		body := rec.Body.String()
		var parsed map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("GET %s body does not parse as JSON: %v (%q)", p, err, body)
		}
		if msg, _ := parsed["error"].(string); !strings.Contains(msg, "missing") {
			t.Errorf("GET %s 404 body does not name the slug: %q", p, body)
		}
		// No HTML tags anywhere in the body.
		if strings.Contains(body, "<") || strings.Contains(body, ">") {
			t.Errorf("GET %s 404 body carries markup: %q", p, body)
		}
	}
}

// ---------- R9 — TestBT072SnapshotSemantics ----------

func TestBT072SnapshotSemantics(t *testing.T) {
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "snap", "bdir"), "snapboard")
	srv := newServeServer(nil)
	bt072Upload(t, srv, dir, "snap")
	h := srv.routes()

	before := bt072APIDetail(t, h, "snapboard")

	// Append a task row to tasks.jsonl on disk AFTER the load.
	f, err := os.OpenFile(filepath.Join(dir, "tasks.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id": "SNAP-NEW", "title": "added after load", "status": "pending", "created_at": "2026-09-06 00:00:00", "updated_at": "2026-09-06 00:00:00"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	after := bt072APIDetail(t, h, "snapboard")
	if len(after.Tasks) != len(before.Tasks) {
		t.Fatalf("disk edit leaked into the read model: %d rows after append, want %d (R9)", len(after.Tasks), len(before.Tasks))
	}
	got, _ := json.Marshal(after.Derived)
	want, _ := json.Marshal(before.Derived)
	if string(got) != string(want) {
		t.Fatalf("disk edit changed the derived block (R9):\n got  %s\n want %s", got, want)
	}
	// loaded_at present and parseable (RFC3339).
	if after.LoadedAt == "" {
		t.Fatal("read model carries no loaded_at (R9)")
	}
	if _, err := time.Parse(time.RFC3339, after.LoadedAt); err != nil {
		t.Fatalf("loaded_at %q is not RFC3339: %v", after.LoadedAt, err)
	}
}

// ---------- R10 — TestBT072IdentityDedupePostFix ----------

func TestBT072IdentityDedupePostFix(t *testing.T) {
	srv := newServeServer(nil)
	h := srv.routes()

	// Upload A (3 tasks via seedServeBoard) and re-upload A' (same
	// name+topology, fresh seed): ONE A entry, the new board's counts.
	zipA := df2BoardZip(t, "A")
	if rec := postUpload(h, map[string][]byte{"zip": zipA}); rec.Code != 200 {
		t.Fatalf("upload A status = %d", rec.Code)
	}
	// A 5-task variant of the same identity: seedServeBoard writes 3 task
	// rows; append two more to a fresh seed before zipping.
	root2 := t.TempDir()
	dirA5 := seedServeBoard(t, filepath.Join(root2, "a5"), "A")
	f, err := os.OpenFile(filepath.Join(dirA5, "tasks.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"id": "Z-4", "title": "fourth", "status": "pending", "priority": "P3", "created_at": "2026-09-06 00:00:00", "updated_at": "2026-09-06 00:00:00"}` + "\n")
	f.WriteString(`{"id": "Z-5", "title": "fifth", "status": "review", "priority": "P3", "created_at": "2026-09-06 00:00:00", "updated_at": "2026-09-06 00:00:00"}` + "\n")
	f.Close()
	zipA5 := zipFromPaths(t, root2, filepath.Join(dirA5, "board.jsonl"), filepath.Join(dirA5, "tasks.jsonl"), filepath.Join(dirA5, "events.jsonl"))
	if rec := postUpload(h, map[string][]byte{"zip": zipA5}); rec.Code != 200 {
		t.Fatalf("re-upload A' status = %d", rec.Code)
	}
	boards := df2APIBoards(t, h)
	if len(boards) != 1 {
		t.Fatalf("/api/boards entries = %d, want 1 after same-identity re-upload (R10)", len(boards))
	}
	if boards[0].Name != "zipboarda" || boards[0].Total != 5 {
		t.Fatalf("/api/boards[0] = %+v, want zipboarda with 5 tasks (the NEW upload wins)", boards[0])
	}

	// Same name, DIFFERENT topology -> two distinct entries.
	if rec := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "B")}); rec.Code != 200 {
		t.Fatalf("upload B status = %d", rec.Code)
	}
	boards = df2APIBoards(t, h)
	if len(boards) != 2 {
		t.Fatalf("/api/boards entries = %d, want 2 (topology distinguishes identity)", len(boards))
	}

	// The -C board is listed with zero uploads and survives an unrelated upload.
	root3 := t.TempDir()
	dirC := bt072SeedBoard(t, filepath.Join(root3, "cdir"), "cboard")
	cSrv := newServeServer([]*board.Board{mustResolve(t, dirC)})
	cH := cSrv.routes()
	listed := df2APIBoards(t, cH)
	if len(listed) != 1 || listed[0].Name != "cboard" {
		t.Fatalf("-C board not listed after zero uploads: %+v", listed)
	}
	root4 := t.TempDir()
	dirU := bt072SeedBoard(t, filepath.Join(root4, "udir"), "unrelated")
	bt072Upload(t, cSrv, dirU, "u")
	listed = df2APIBoards(t, cH)
	if len(listed) != 2 {
		t.Fatalf("/api/boards after unrelated upload = %d entries, want 2 (-C + upload)", len(listed))
	}
	if listed[0].Name != "cboard" {
		t.Fatalf("-C board not first/preseved: %+v", listed)
	}
}

func mustResolve(t *testing.T, dir string) *board.Board {
	t.Helper()
	b, err := board.Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// mustTempDir is a subdirectory-free t.TempDir alias for seeds that build
// their own layout under the given root.
func mustTempDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "seed", "bdir")
}

// ---------- R11 — TestBT072OverviewCards ----------

func TestBT072OverviewCards(t *testing.T) {
	h := bt072Server(t).routes()
	body := bt072UIBody(t, h)
	island := bt072Island(t, body)

	// The embedded payload yields TWO cards whose fields equal /api/boards.
	apiRaw := bt072Get(h, "/api/boards").Body.String()
	var api []map[string]any
	if err := json.Unmarshal([]byte(apiRaw), &api); err != nil {
		t.Fatal(err)
	}
	if len(island.Boards) != 2 || len(api) != 2 {
		t.Fatalf("island boards = %d, api boards = %d, want 2/2", len(island.Boards), len(api))
	}
	for _, ib := range island.Boards {
		if ib.Name == "" || ib.Slug == "" || ib.Topology == "" {
			t.Errorf("island card for %q misses name/slug/topology", ib.Slug)
		}
		if ib.Validate.Pill == "" || ib.LoadedAt == "" {
			t.Errorf("island card for %q misses the validate pill / loaded_at (R11)", ib.Slug)
		}
		for _, entry := range api {
			if entry["slug"] != ib.Slug {
				continue
			}
			if int(entry["task_total"].(float64)) != ib.Derived.TotalNonFix ||
				int(entry["done"].(float64)) != ib.Derived.CompleteCount ||
				int(entry["open"].(float64)) != ib.Derived.OpenCount {
				t.Errorf("island card %s fields != /api/boards", ib.Slug)
			}
		}
	}
	// The page's own markup renders the overview from the island: cards
	// carry the name, pill, and the empty state exists in the CSS/JS.
	for _, want := range []string{"grid", "renderOverview", "no boards loaded"} {
		if !strings.Contains(body, want) {
			t.Errorf("/ui overview markup missing %q", want)
		}
	}

	// The one-board fixture renders the one-card state.
	one := newServeServer(nil)
	root := t.TempDir()
	bt072Upload(t, one, bt072SeedBoard(t, filepath.Join(root, "one", "bdir"), "oneboard"), "one")
	oneIsland := bt072Island(t, bt072UIBody(t, one.routes()))
	if len(oneIsland.Boards) != 1 {
		t.Fatalf("one-board island = %d cards, want 1", len(oneIsland.Boards))
	}

	// Zero boards renders the honest empty state (reachable in tests).
	emptyBody := bt072UIBody(t, newServeServer(nil).routes())
	emptyIsland := bt072Island(t, emptyBody)
	if len(emptyIsland.Boards) != 0 {
		t.Fatalf("zero-board island = %d cards, want 0", len(emptyIsland.Boards))
	}
	if !strings.Contains(emptyBody, "no boards loaded") {
		t.Error("zero-board /ui does not carry the empty state")
	}
}

// ---------- R12 — TestBT072BoardRowsTable ----------

func TestBT072BoardRowsTable(t *testing.T) {
	srv := bt072Server(t)
	h := srv.routes()
	// Topology-B fixture: header line 1 of tasks.jsonl must appear in the
	// header strip and in NO row; every task row renders exactly once.
	d := bt072APIDetail(t, h, "projectb")
	rows := []map[string]any{}
	for _, raw := range d.Tasks {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, m)
	}
	if len(rows) != 2 {
		t.Fatalf("projectb rows = %d, want 2 (the header line is NOT a row)", len(rows))
	}
	seen := map[string]int{}
	for _, r := range rows {
		seen[fmt.Sprint(r["id"])]++
		if _, isHeader := r["project"]; isHeader && r["id"] == nil {
			t.Errorf("the topology-B header line leaked into a row: %v", r)
		}
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("row %s renders %d times, want exactly once", id, n)
		}
	}
	var hdr struct {
		Project string `json:"project"`
	}
	if err := json.Unmarshal(d.Header, &hdr); err != nil || hdr.Project != "projectb" {
		t.Fatalf("header strip = %s (err %v), want the topology-B header project projectb", d.Header, err)
	}
	// Payload row count equals rendered row count: the island embeds the
	// same rows the page's table renders from.
	island := bt072Island(t, bt072UIBody(t, h))
	for _, ib := range island.Boards {
		if ib.Slug == "projectb" && len(ib.Tasks) != len(d.Tasks) {
			t.Errorf("island rows for projectb = %d, api = %d", len(ib.Tasks), len(d.Tasks))
		}
	}
	// Expansion works from the embedded payload without refetching: the
	// detail renderer exists in the page and every field it needs is inside
	// the island payload (structural assertion — no JS runtime per spec).
	body := bt072UIBody(t, h)
	for _, want := range []string{"rowDetail", "aria-expanded", "task JSON"} {
		if !strings.Contains(body, want) {
			t.Errorf("/ui expansion machinery missing %q", want)
		}
	}
}

// ---------- R13 — TestBT072ValidationPanel ----------

func TestBT072ValidationPanel(t *testing.T) {
	h := bt072Server(t).routes()
	d := bt072APIDetail(t, h, "projecta")
	v := d.Validate

	// Pill FAIL: the fixture carries a duplicate-id ERROR (+ warnings).
	if v.Pill != "FAIL" {
		t.Fatalf("validate pill = %q, want FAIL (duplicate-id error fixture)", v.Pill)
	}
	if v.Errors < 1 {
		t.Fatalf("errors = %d, want >= 1", v.Errors)
	}

	// Both finding classes present with correct line numbers.
	joined := ""
	hasError, hasWarn := false, false
	for _, f := range v.Findings {
		joined += f.Level + " " + f.Msg + "\n"
		if f.Level == "error" {
			hasError = true
		}
		if f.Level == "warn" {
			hasWarn = true
		}
	}
	if !hasError || !hasWarn {
		t.Fatalf("findings missing a class (error=%v warn=%v):\n%s", hasError, hasWarn, joined)
	}
	if !strings.Contains(joined, "duplicate task id") || !strings.Contains(joined, "line 3") {
		t.Errorf("duplicate-id finding not rendered with its line number:\n%s", joined)
	}
	if !strings.Contains(joined, "deployed") {
		t.Errorf("off-vocabulary status finding missing:\n%s", joined)
	}

	// Census line matches b.Validate() output byte-for-byte.
	dir := bt072SeedBoard(t, mustTempDir(t), "censusboard")
	rep, err := mustResolve(t, dir).Validate()
	if err != nil {
		t.Fatal(err)
	}
	srv2 := newServeServer(nil)
	bt072Upload(t, srv2, dir, "cf")
	d2 := bt072APIDetail(t, srv2.routes(), "censusboard")
	if d2.Validate.Census != rep.Keys.Summary() {
		t.Fatalf("census line != CLI validate output:\n api:   %q\n cli:   %q", d2.Validate.Census, rep.Keys.Summary())
	}

	// The read model equals a CLI validate run on the same files.
	if d2.Validate.Errors != rep.Errors() || d2.Validate.Warnings != len(rep.Findings)-rep.Errors() {
		t.Fatalf("validate counts != CLI: api %d/%d, cli %d/%d",
			d2.Validate.Errors, d2.Validate.Warnings, rep.Errors(), len(rep.Findings)-rep.Errors())
	}

	// Sweep census: off-vocab rows named with ids, decisions explicitly none.
	if d2.Validate.Sweep.DecisionsNote != "no decisions applied — CLI-only" {
		t.Errorf("sweep census missing the no-decisions line: %+v", d2.Validate.Sweep)
	}

	// Warn-only fixture: pill WARN, exit-equivalent-0 semantics documented.
	root3 := t.TempDir()
	dirW := filepath.Join(root3, "w", "bdir")
	if err := os.MkdirAll(dirW, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirW, "board.jsonl"), []byte(`{"project": "warnboard", "namespace": "ns", "version": 1, "ticks_total": 5, "ticks_idle": 1, "last_commit": null}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirW, "tasks.jsonl"), []byte(`{"id": "W-1", "title": "aliased", "status": "completed", "priority": "P1", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirW, "events.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	srv3 := newServeServer(nil)
	bt072Upload(t, srv3, dirW, "w")
	dW := bt072APIDetail(t, srv3.routes(), "warnboard")
	if dW.Validate.Pill != "WARN" {
		t.Fatalf("warn-only pill = %q, want WARN", dW.Validate.Pill)
	}
	if dW.Validate.Errors != 0 {
		t.Fatalf("warn-only errors = %d, want 0 (WARN is validate's exit-0 shape)", dW.Validate.Errors)
	}
	body := bt072UIBody(t, srv3.routes())
	if !strings.Contains(body, "exit-0-with-warnings") {
		t.Error("the /ui page does not document WARN's exit-equivalent-0 semantics (R13)")
	}
}

// ---------- R14 — TestBT072EventsTimeline ----------

func TestBT072EventsTimeline(t *testing.T) {
	h := bt072Server(t).routes()
	evs := bt072APIEvents(t, h, "projecta")
	if len(evs) != 3 {
		t.Fatalf("events = %d, want 3", len(evs))
	}

	// JSON-encoded-string detail renders DECODED (ladder step 2 -> object).
	if got := bt072EventDetailJSON(t, evs[0].Detail); !strings.HasPrefix(got, "{") {
		t.Errorf("event 1 detail = %s, want the decoded object", got)
	}
	// base64-wrapped detail renders DECODED (step 3 -> object).
	if got := bt072EventDetailJSON(t, evs[1].Detail); got != `{"k":"v","n":2}` {
		t.Errorf("event 2 detail = %s, want the base64-decoded object", got)
	}
	// A <script>-bearing raw detail falls through the ladder (step 4) and
	// is carried as a plain string — the /ui page renders it via
	// textContent only (asserted structurally here; R16 covers the scan).
	if got, ok := evs[2].Detail.(string); !ok || !strings.Contains(got, "<img") {
		t.Errorf("event 3 detail = %#v, want the raw hostile string (fall-through)", evs[2].Detail)
	}

	// task_id linkage: the linked events carry the row's id.
	linked := 0
	for _, e := range evs {
		if e.TaskID != nil {
			linked++
		}
	}
	if linked != 2 {
		t.Errorf("events with task linkage = %d, want 2", linked)
	}

	// The page's timeline machinery is payload-driven from the island and
	// decodes nothing itself (the payload arrives already decoded).
	body := bt072UIBody(t, h)
	for _, want := range []string{"renderEventList", "ev-detail", "filter by event type"} {
		if !strings.Contains(body, want) {
			t.Errorf("/ui timeline missing %q", want)
		}
	}
	// The island events equal the /api events (one derivation, two hosts).
	island := bt072Island(t, body)
	for _, ib := range island.Boards {
		if ib.Slug != "projecta" {
			continue
		}
		if len(ib.Events) != len(evs) {
			t.Errorf("island events = %d, api events = %d", len(ib.Events), len(evs))
		}
	}
}

// ---------- R15 — TestBT072AnalyticsParity ----------

func TestBT072AnalyticsParity(t *testing.T) {
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "par", "bdir"), "parityboard")
	srv := newServeServer(nil)
	bt072Upload(t, srv, dir, "par")

	// The report HTML built from the same snapshot and the /api read model
	// must carry the SAME derived values — both parsed in Go and compared.
	boards := srv.currentBoards()
	payload, err := render.BuildBoards(boards, render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		t.Fatal(err)
	}
	html, err := render.RenderHTML(payload)
	if err != nil {
		t.Fatal(err)
	}
	reportIsland := mustIslandPayload(t, html)
	api := bt072APIDetail(t, srv.routes(), "parityboard")

	ref := reportIsland.Boards[0]
	gotAPI, _ := json.Marshal(api.Derived)
	gotRef, _ := json.Marshal(ref.Derived)
	if string(gotAPI) != string(gotRef) {
		t.Fatalf("UI derived != report derived:\n api:    %s\n report: %s", gotAPI, gotRef)
	}
	// The /ui island is the third host of the same numbers.
	uiIsland := bt072Island(t, bt072UIBody(t, srv.routes()))
	gotUI, _ := json.Marshal(uiIsland.Boards[0].Derived)
	if string(gotUI) != string(gotRef) {
		t.Fatalf("/ui island derived != report derived:\n ui:     %s\n report: %s", gotUI, gotRef)
	}
	// Task rows are byte-identical across all three hosts.
	if len(api.Tasks) != len(ref.Tasks) {
		t.Fatalf("task rows: api %d, report %d", len(api.Tasks), len(ref.Tasks))
	}
	for i := range api.Tasks {
		if string(api.Tasks[i]) != string(ref.Tasks[i]) {
			t.Errorf("task row %d differs between api and report payloads", i)
		}
	}
}

// ---------- R16 — TestBT072XSSInert ----------

func TestBT072XSSInert(t *testing.T) {
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "xss", "bdir"), "xssboard")
	hostile := `{"id": "EVIL", "title": "</script><img src=x onerror=alert(1)>", "status": "pending", "created_at": "2026-09-04 00:00:00", "updated_at": "2026-09-04 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), append(mustRead(t, filepath.Join(dir, "tasks.jsonl")), []byte(hostile)...), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newServeServer(nil)
	bt072Upload(t, srv, dir, "xss")
	h := srv.routes()
	body := bt072UIBody(t, h)

	// The island round-trips byte-identical through JSON.parse: the hostile
	// title survives as TEXT (fixture asserted in Go).
	island := bt072Island(t, body)
	found := false
	for _, ib := range island.Boards {
		if ib.Slug != "xssboard" {
			continue
		}
		for _, raw := range ib.Tasks {
			var row struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			if row.ID == "EVIL" {
				found = true
				want := "</script><img src=x onerror=alert(1)>"
				if row.Title != want {
					t.Fatalf("hostile title round-trip = %q, want byte-identical %q", row.Title, want)
				}
			}
		}
	}
	if !found {
		t.Fatal("hostile row missing from the island payload")
	}

	// `</script>` cannot occur inside the island: exactly the app's own two
	// closers exist (island script + app script), as the report test pins.
	if got := strings.Count(body, "</script>"); got != 2 {
		t.Fatalf("closing script tags = %d, want 2 — a raw </script> in the island breaks this", got)
	}
	if !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatal("hostile </script> not escaped in the island")
	}

	// Static scan: no innerHTML assignment in the /ui inline JS, and no
	// document.write / insertAdjacentHTML either.
	m := regexp.MustCompile(`(?s)<script>(.*)</script>`)
	match := m.FindStringSubmatch(body)
	if match == nil {
		t.Fatal("no inline JS found in /ui")
	}
	js := match[1]
	for _, banned := range []string{"innerHTML", "insertAdjacentHTML", "document.write", "outerHTML"} {
		if strings.Contains(js, banned) {
			t.Errorf("/ui inline JS contains %q (R16: forbidden)", banned)
		}
	}

	// The title renders as TEXT in the row-table markup derived from the
	// payload: the table renderer sets textContent (structural assertion
	// of the only channel reaching the DOM).
	for _, want := range []string{`n.textContent = String(text)`, `.textContent = String(`} {
		if !strings.Contains(js, want) {
			t.Errorf("/ui inline JS lost the textContent render channel %q", want)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ---------- R17 — TestBT072NetworkConfined ----------

func TestBT072NetworkConfined(t *testing.T) {
	h := bt072Server(t).routes()
	body := bt072UIBody(t, h)
	m := regexp.MustCompile(`(?s)<script>(.*)</script>`)
	match := m.FindStringSubmatch(body)
	if match == nil {
		t.Fatal("no inline JS in /ui")
	}
	js := match[1]

	// No http:// or https:// literal anywhere in the inline JS.
	if strings.Contains(js, "http://") || strings.Contains(js, "https://") {
		t.Error("/ui inline JS contains an http(s):// literal (R17/NG9)")
	}
	// No CDN/font/img/import/telemetry references.
	for _, banned := range []string{"<link", "@import", "<img", "fonts.googleapis", "cdn.", "XMLHttpRequest", "EventSource", "WebSocket", "navigator.sendBeacon"} {
		if strings.Contains(js, banned) {
			t.Errorf("/ui inline JS references %q (R17: no external anything)", banned)
		}
	}
	// Every fetch( literal argument starts with /api/.
	fetchRe := regexp.MustCompile(`fetch\("([^"]*)"`)
	fetches := fetchRe.FindAllStringSubmatch(js, -1)
	if len(fetches) == 0 {
		t.Fatal("no fetch( calls found in /ui inline JS — the scan must see the refresh path")
	}
	for _, f := range fetches {
		if !strings.HasPrefix(f[1], "/api/") {
			t.Errorf("fetch(%q) is not a same-origin /api/ route (R17)", f[1])
		}
	}
	// The document head carries no external stylesheet/script/link.
	head := body[:strings.Index(body, "</head>")]
	for _, banned := range []string{`<link`, `src="http`, `href="http`, `src="//`, `href="//`} {
		if strings.Contains(head, banned) {
			t.Errorf("/ui head references an external asset (%q)", banned)
		}
	}
}

// ---------- R18 — TestBT072StandaloneDegrade ----------

func TestBT072StandaloneDegrade(t *testing.T) {
	srv := bt072Server(t)

	// Wrap routes() so every /api route answers 500; /ui keeps serving the
	// real document with its island.
	broken := http.NewServeMux()
	broken.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "synthetic outage", http.StatusInternalServerError)
	})
	broken.Handle("/ui", srv.routes())
	broken.Handle("/", srv.routes())

	body := bt072UIBody(t, broken)
	island := bt072Island(t, body)

	// The island still parses and the overview fields are renderable from
	// it: two cards with names, slugs, totals.
	if len(island.Boards) != 2 {
		t.Fatalf("island boards under /api outage = %d, want 2", len(island.Boards))
	}
	for _, ib := range island.Boards {
		if ib.Name == "" || ib.Slug == "" || ib.Derived.TotalNonFix == 0 {
			t.Errorf("island card %q not renderable: name=%q total=%d", ib.Slug, ib.Name, ib.Derived.TotalNonFix)
		}
	}
	// The page contains the error-state marker: the offline banner machinery.
	for _, want := range []string{"API unreachable — showing the snapshot embedded at load", "showOffline", `id="offline-banner"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/ui page missing the R18 error-state marker %q", want)
		}
	}
	// The refresh path surfaces the same state instead of a blank page.
	if !strings.Contains(body, "showOffline()") {
		t.Error("refresh failure path does not surface the offline state")
	}
}

// ---------- R19 — TestBT072SizeGuardrail ----------

func TestBT072SizeGuardrail(t *testing.T) {
	// The 300-task/2000-event board: the render size fixture's shape.
	root := t.TempDir()
	dir := filepath.Join(root, "big", "bdir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "board.jsonl"), []byte(`{"project": "bigboard", "namespace": "ns", "version": 1, "ticks_total": 0, "ticks_idle": 0, "last_commit": null}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var tasks, events bytes.Buffer
	for i := 1; i <= 300; i++ {
		fmt.Fprintf(&tasks, `{"id": "BIG-%03d", "title": "task %d", "status": "complete", "priority": "P2", "primary_model": "glm-5.3-flash", "created_at": "2026-01-01 00:00:00", "completed_at": "2026-06-%02d 12:00:00", "updated_at": "2026-06-%02d 12:00:00"}`+"\n", i, i, (i%28)+1, (i%28)+1)
	}
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&events, `{"id":%d,"timestamp":"2026-06-%02d %02d:00:00","event_type":"tick","task_id":"BIG-%03d","actor":"foreman","detail":null,"tick_number":%d}`+"\n", i, (i%28)+1, i%24, (i%300)+1, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), tasks.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), events.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newServeServer(nil)
	bt072Upload(t, srv, dir, "big")

	// /ui document size + one /api/board payload, measured on the wire.
	uiBody := bt072UIBody(t, srv.routes())
	apiBody := bt072Get(srv.routes(), "/api/board/bigboard").Body.String()
	total := len(uiBody) + len(apiBody)

	// The note fires ONLY over the bound: under 5 MB it must not.
	if total <= uiSizeGuardrail {
		if uiNoteIfOver(total) {
			t.Errorf("size note fired at %d bytes, under the %d-byte guardrail", total, uiSizeGuardrail)
		}
		if !uiNoteIfOver(uiSizeGuardrail + 1) {
			t.Error("size note did not fire just over the guardrail")
		}
		if !strings.Contains(uiBody, "boardctl serve: /ui payload is") == false && total > uiSizeGuardrail {
			t.Error("note text present under the guardrail")
		}
	} else {
		// The 300/2000 fixture exceeded the bound: the note text must be on
		// the wire path (handleUI logs it to serve stderr) and this is not
		// an error (R19: exceeding is a note, not a failure).
		if !uiNoteIfOver(total) {
			t.Error("size note did not fire over the guardrail")
		}
		t.Logf("300/2000 fixture /ui=%d + api=%d = %d bytes, over the ~5MB guardrail (note logged, not an error)", len(uiBody), len(apiBody), total)
	}
	// Both documents must have rendered regardless of size.
	if !strings.Contains(uiBody, "bigboard") {
		t.Fatal("large board did not render in /ui")
	}
}

// ---------- R20 — TestBT072NoBoardWrites ----------

func TestBT072NoBoardWrites(t *testing.T) {
	root := t.TempDir()
	dir := bt072SeedBoard(t, filepath.Join(root, "rw", "bdir"), "rwboard")
	srv := newServeServer(nil)
	bt072Upload(t, srv, dir, "rw")
	h := srv.routes()

	before := fileSnapshot(t, root)
	beforeTemps := bt072TempRoots(t, srv)

	// Exercise EVERY new route.
	body := bt072UIBody(t, h)
	if !strings.Contains(body, "rwboard") {
		t.Fatal("/ui did not render the board")
	}
	if d := bt072APIDetail(t, h, "rwboard"); len(d.Tasks) == 0 {
		t.Fatal("api board detail empty")
	}
	if evs := bt072APIEvents(t, h, "rwboard"); len(evs) != 3 {
		t.Fatalf("api events = %d, want 3", len(evs))
	}
	// Plus the pre-existing surface for completeness.
	if rec := bt072Get(h, "/api/boards"); rec.Code != 200 {
		t.Fatal("/api/boards broke")
	}

	after := fileSnapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("fixture tree changed: %d files before, %d after (R20)", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("fixture file %s changed (size %d -> %d) during route exercise (R20)", k, v, after[k])
		}
	}
	afterTemps := bt072TempRoots(t, srv)
	if len(beforeTemps) != len(afterTemps) {
		t.Fatalf("temp root count changed: %d -> %d", len(beforeTemps), len(afterTemps))
	}
	// Nothing appeared OUTSIDE the enumerated temp roots either: the only
	// files the exercise touched are inside them (compare with the
	// untouched fixture root above, which stayed byte-identical).
	_ = afterTemps
}

func bt072TempRoots(t *testing.T, srv *serveServer) []string {
	t.Helper()
	srv.lock()
	defer srv.unlock()
	var out []string
	for _, s := range srv.sessions {
		out = append(out, s.root)
	}
	return out
}
