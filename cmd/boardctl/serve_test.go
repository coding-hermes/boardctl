package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/render"
)

// seedServeBoard writes a board under dir with the given topology and
// returns the board dir path. tasks carry 2 completions + 1 pending so the
// derived numbers are non-trivial.
func seedServeBoard(t *testing.T, dir string, topology string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var tasks string
	if topology == "A" {
		if err := os.WriteFile(filepath.Join(dir, "board.jsonl"), []byte(`{"project": "zipboarda", "namespace": "ns", "version": 1, "ticks_total": 40, "ticks_idle": 10, "last_commit": null}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		tasks = ""
	} else {
		// Topology B: header is line 1 of tasks.jsonl (header shape: no id).
		tasks = `{"project": "zipboardb", "namespace": "ns", "version": 1, "ticks_total": 69, "ticks_idle": 7}` + "\n"
	}
	tasks += `{"id": "Z-1", "title": "first", "status": "complete", "priority": "P1", "primary_model": "glm-5.3-flash", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n" +
		`{"id": "Z-2", "title": "second", "status": "complete", "priority": "P2", "primary_model": "kimi-k3", "created_at": "2026-09-03 00:00:00", "completed_at": "2026-09-04 00:00:00", "updated_at": "2026-09-04 00:00:00"}` + "\n" +
		`{"id": "Z-3", "title": "third", "status": "pending", "priority": "P2", "created_at": "2026-09-05 00:00:00", "updated_at": "2026-09-05 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	events := `{"id":1,"timestamp":"2026-09-02 00:00:00","event_type":"task_completed","task_id":"Z-1","actor":"foreman","detail":"{\"status\":\"complete\"}","tick_number":1}` + "\n" +
		`{"id":2,"timestamp":"2026-09-04 00:30:00","event_type":"idle","task_id":null,"actor":"foreman","detail":null,"tick_number":2}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// zipFromPaths builds an in-memory zip whose entries replicate the given
// filesystem paths relative to base (entries are byte copies).
func zipFromPaths(t *testing.T, base string, paths ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range paths {
		rel, err := filepath.Rel(base, p)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// postUpload POSTs a multipart form to h and returns the recorder.
func postUpload(h http.Handler, fields map[string][]byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, data := range fields {
		fw, err := mw.CreateFormFile(name, name)
		if err != nil {
			panic(err)
		}
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// mustIslandPayload extracts the report JSON island from an HTML response
// body (the report embeds it as the single application/json script).
func mustIslandPayload(t *testing.T, body string) render.ReportPayload {
	t.Helper()
	start := strings.Index(body, `<script type="application/json" id="report-data">`)
	if start < 0 {
		t.Fatalf("no JSON island in response (%d bytes)", len(body))
	}
	start += len(`<script type="application/json" id="report-data">`)
	end := strings.Index(body[start:], "</script>")
	if end < 0 {
		t.Fatal("unterminated JSON island")
	}
	var escaped = body[start : start+end]
	// The island is HTML-escaped JSON (<,>,& as \u00XX); Go's decoder
	// accepts the escapes back. U+2028/29 lines are also lossless.
	dec := json.NewDecoder(strings.NewReader(escaped))
	var p render.ReportPayload
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("island does not parse: %v", err)
	}
	return p
}

// fileSnapshot hashes (path,size) pairs of every file under dir.
func fileSnapshot(t *testing.T, dir string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		out[rel] = info.Size()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestServeIndexPage: GET / -> 200, self-contained uploader form present
// (7.2.1) with no CDN/external references (6.3).
func TestServeIndexPage(t *testing.T) {
	h := newServeServer(nil).routes()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<form", "webkitdirectory", `type="file"`, ".zip"} {
		if !strings.Contains(body, want) {
			t.Errorf("uploader page missing %q", want)
		}
	}
	for _, banned := range []string{"http://", "https://", "fetch(", "XMLHttpRequest", "src=\"", "href=\"http"} {
		if strings.Contains(body, banned) {
			t.Errorf("uploader page contains external/dynamic reference %q (6.3: no CDN, no network)", banned)
		}
	}
}

// TestServeLoopbackAddrValidation: non-loopback --addr refuses with exit 2
// and an explicit non-loopback message (7.2.2).
func TestServeLoopbackAddrValidation(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:8787", "10.0.0.1:8787", "[fe80::1]:8787", "example.com:8787"} {
		err := cmdServe("", []string{"--addr", addr})
		if err == nil {
			t.Fatalf("addr %q: expected refusal, got nil", addr)
		}
		if !strings.Contains(err.Error(), "non-loopback") {
			t.Errorf("addr %q: error %v does not mention non-loopback", addr, err)
		}
		code := run([]string{"serve", "--addr", addr})
		if code != 2 {
			t.Errorf("run(serve --addr %q) exit = %d, want 2", addr, code)
		}
	}
	for _, addr := range []string{"127.0.0.1:0", "localhost:0", "[::1]:0"} {
		// loopbackHost is the gate; these must pass validation (they are
		// only never actually bound here — port 0 would race in CI).
		host, _, err := net.SplitHostPort(addr)
		if err != nil || !loopbackHost(host) {
			t.Errorf("loopback host %q must be accepted (err=%v)", addr, err)
		}
	}
}

// TestServeFolderUploadMatchesRender: a webkitdirectory-style multi-file
// POST of THIS repo's own board yields the same `derived` block as
// `render -C .` on the same board (7.2.4).
func TestServeFolderUploadMatchesRender(t *testing.T) {
	boardDir := filepath.Join(repoRoot(t), ".coding-hermes", "board")
	before := fileSnapshot(t, repoRoot(t))

	// Serve path: upload the board files with relative paths preserved,
	// exactly how a browser sends a webkitdirectory selection rooted at the
	// repo (filenames carry the full relative path).
	fields := map[string][]byte{}
	for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"} {
		data, err := os.ReadFile(filepath.Join(boardDir, name))
		if err != nil {
			t.Fatal(err)
		}
		fields[filepath.Join(".coding-hermes", "board", name)] = data
	}
	// Field names carry the relative path (this is what browsers do).
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, data := range fields {
		fw, err := mw.CreateFormFile(name, name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(data)
	}
	mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	newServeServer(nil).routes().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("folder upload status = %d, body: %s", rec.Code, rec.Body.String())
	}
	served := mustIslandPayload(t, rec.Body.String())
	if len(served.Boards) != 1 {
		t.Fatalf("folder upload produced %d boards, want 1", len(served.Boards))
	}
	// Render path on the same board.
	ref, err := render.Build(repoRoot(t), render.Options{Now: servedRenderTime(served), Zone: time.Local})
	if err != nil {
		t.Fatal(err)
	}
	servedJSON, _ := json.Marshal(served.Boards[0].Derived)
	refJSON, _ := json.Marshal(ref.Boards[0].Derived)
	if string(servedJSON) != string(refJSON) {
		t.Fatalf("serve-derived != render-derived:\n serve: %s\n render: %s", servedJSON, refJSON)
	}
	// The board card must carry the boardctl project identity.
	if ref.Boards[0].Name != "boardctl" {
		t.Fatalf("board name = %q, want boardctl (header from board.jsonl)", ref.Boards[0].Name)
	}

	// Zero writes to sources (7.2.8): identical snapshot after the upload.
	after := fileSnapshot(t, repoRoot(t))
	if len(before) != len(after) {
		t.Fatalf("source tree changed: %d files before, %d after", len(before), len(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("source file %s changed (size %d -> %d)", k, v, after[k])
		}
	}
}

func servedRenderTime(p render.ReportPayload) time.Time {
	ts, _ := time.Parse(time.RFC3339, p.RenderedAt)
	return ts
}

// TestServeTwoBoardZip: a zip with one topology-A and one topology-B board
// yields len(boards)==2 (7.2.3) with the 5.6 compare data present.
func TestServeTwoBoardZip(t *testing.T) {
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "projA", ".coding-hermes", "board"), "A")
	dirB := seedServeBoard(t, filepath.Join(root, "projB"), "B")
	zipBytes := zipFromPaths(t, root,
		filepath.Join(dirA, "board.jsonl"), filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"),
		filepath.Join(dirB, "tasks.jsonl"), filepath.Join(dirB, "events.jsonl"),
	)

	rec := postUpload(newServeServer(nil).routes(), map[string][]byte{"zip": zipBytes})
	if rec.Code != 200 {
		t.Fatalf("zip upload status = %d, body: %s", rec.Code, rec.Body.String())
	}
	payload := mustIslandPayload(t, rec.Body.String())
	if len(payload.Boards) != 2 {
		t.Fatalf("payload has %d boards, want 2", len(payload.Boards))
	}
	var names []string
	for _, b := range payload.Boards {
		names = append(names, b.Name)
		if b.Error != "" {
			t.Errorf("board %s unexpected error state: %s", b.Name, b.Error)
		}
	}
	sort.Strings(names)
	if names[0] != "zipboarda" || names[1] != "zipboardb" {
		t.Fatalf("board names = %v, want [zipboarda zipboardb]", names)
	}
	if payload.Boards[0].Topology != "A" || payload.Boards[1].Topology != "B" {
		t.Fatalf("topologies = %s/%s, want A/B", payload.Boards[0].Topology, payload.Boards[1].Topology)
	}
	// ticks carried through the header per topology (compare table 3.10).
	if got := headerTicks(t, payload.Boards[0].Header); got != 40 {
		t.Fatalf("board A ticks_total = %v, want 40", got)
	}
	if got := headerTicks(t, payload.Boards[1].Header); got != 69 {
		t.Fatalf("board B ticks_total = %v, want 69 (BT-010 B-header)", got)
	}
	// derived sanity: 2 complete, 1 open on each seeded board.
	for _, b := range payload.Boards {
		if b.Derived.CompleteCount != 2 || b.Derived.OpenCount != 1 || b.Derived.TotalNonFix != 3 {
			t.Errorf("board %s derived counts complete/open/total = %d/%d/%d, want 2/1/3",
				b.Name, b.Derived.CompleteCount, b.Derived.OpenCount, b.Derived.TotalNonFix)
		}
	}
}

// headerTicks pulls ticks_total out of a raw header row.
func headerTicks(t *testing.T, h render.BoardHeader) int {
	t.Helper()
	if len(h) == 0 {
		t.Fatal("header absent")
	}
	var m struct {
		TicksTotal int `json:"ticks_total"`
	}
	if err := json.Unmarshal(h, &m); err != nil {
		t.Fatalf("header parse: %v", err)
	}
	return m.TicksTotal
}

// TestServeZipSlipDefense: an entry named ../../evil.txt never escapes the
// temp extraction dir; the report still renders (7.2.5).
func TestServeZipSlipDefense(t *testing.T) {
	// outside: a canary file NEXT to the temp root that must not appear.
	outsideDir := t.TempDir()
	canary := filepath.Join(outsideDir, "evil.txt")

	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "solo"), "A")
	zipBytes := zipFromPaths(t, root,
		filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))

	// Inject zip-slip entries aimed at the canary path.
	var withSlip bytes.Buffer
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(&withSlip)
	for _, f := range zr.File {
		zw.Copy(f)
	}
	for _, evil := range []string{"../../evil.txt", filepath.Join("..", "..", filepath.Base(outsideDir), "evil.txt")} {
		w, err := zw.Create(evil)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte("pwned"))
	}
	zw.Close()

	srv := newServeServer(nil)
	rec := postUpload(srv.routes(), map[string][]byte{"zip": withSlip.Bytes()})
	if rec.Code != 200 {
		t.Fatalf("zip-slip upload status = %d (want 200: report still renders), body: %s", rec.Code, rec.Body.String())
	}
	payload := mustIslandPayload(t, rec.Body.String())
	if len(payload.Boards) != 1 {
		t.Fatalf("boards = %d, want 1 (report still renders despite rejected entries)", len(payload.Boards))
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("zip-slip: evil.txt was created OUTSIDE the extraction dir")
	}
}

// TestServeOversizeCap413: a payload over the cap is rejected 413 with a
// readable message and the server still serves afterwards (7.2.6).
func TestServeOversizeCap413(t *testing.T) {
	h := newServeServer(nil).routes()
	big := make([]byte, maxUploadBytes+1)
	rec := postUpload(h, map[string][]byte{"zip": big})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d, want 413", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "too large") {
		t.Fatalf("413 body not readable: %q", rec.Body.String())
	}
	// Server stays up: a following valid upload still works.
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "okboard"), "A")
	good := zipFromPaths(t, root, filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))
	rec2 := postUpload(h, map[string][]byte{"zip": good})
	if rec2.Code != 200 {
		t.Fatalf("post-413 upload status = %d, want 200 (server must stay up)", rec2.Code)
	}
	if p := mustIslandPayload(t, rec2.Body.String()); len(p.Boards) != 1 {
		t.Fatalf("post-413 boards = %d, want 1", len(p.Boards))
	}
}

// TestServeMalformedZip400: a non-zip payload is rejected 400 with a clear
// error and the server stays up.
func TestServeMalformedZip400(t *testing.T) {
	h := newServeServer(nil).routes()
	rec := postUpload(h, map[string][]byte{"zip": []byte("this is definitely not a zip file")})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed zip status = %d, want 400", rec.Code)
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "zip") {
		t.Fatalf("400 body not clear: %q", rec.Body.String())
	}
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "after"), "A")
	good := zipFromPaths(t, root, filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))
	rec2 := postUpload(h, map[string][]byte{"zip": good})
	if rec2.Code != 200 {
		t.Fatalf("post-400 upload status = %d, want 200 (server must stay up)", rec2.Code)
	}
}

// TestServeAPIBoards: GET /api/boards returns the loaded board list (name,
// slug, topology, task total, done/open) — including a -C board when set.
func TestServeAPIBoards(t *testing.T) {
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "apiboard"), "A")
	zipBytes := zipFromPaths(t, root, filepath.Join(dirA, "board.jsonl"), filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))

	srv := newServeServer(nil)
	h := srv.routes()
	if rec := postUpload(h, map[string][]byte{"zip": zipBytes}); rec.Code != 200 {
		t.Fatalf("setup upload status = %d: %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest("GET", "/api/boards", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("/api/boards status = %d", rec.Code)
	}
	var boards []struct {
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		Topology string `json:"topology"`
		Total    int    `json:"task_total"`
		Done     int    `json:"done"`
		Open     int    `json:"open"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &boards); err != nil {
		t.Fatalf("/api/boards json: %v (%q)", err, rec.Body.String())
	}
	if len(boards) != 1 {
		t.Fatalf("/api/boards len = %d, want 1", len(boards))
	}
	b := boards[0]
	if b.Name != "zipboarda" || b.Slug != "zipboarda" || b.Topology != "A" {
		t.Errorf("board identity = %+v", b)
	}
	if b.Total != 3 || b.Done != 2 || b.Open != 1 {
		t.Errorf("board counts = %+v, want total 3 done 2 open 1", b)
	}

	// Empty server: empty list, not an error.
	rec2 := httptest.NewRecorder()
	newServeServer(nil).routes().ServeHTTP(rec2, httptest.NewRequest("GET", "/api/boards", nil))
	if rec2.Code != 200 || strings.TrimSpace(rec2.Body.String()) != "[]" {
		t.Fatalf("empty /api/boards = %d %q, want 200 []", rec2.Code, rec2.Body.String())
	}
}

// TestServeTempRemovedOnShutdown: removeAllTemps deletes per-upload temp
// trees (7.2.8: extraction removed on shutdown).
func TestServeTempRemovedOnShutdown(t *testing.T) {
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "tempkill"), "A")
	zipBytes := zipFromPaths(t, root, filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))
	srv := newServeServer(nil)
	if rec := postUpload(srv.routes(), map[string][]byte{"zip": zipBytes}); rec.Code != 200 {
		t.Fatalf("upload status = %d: %s", rec.Code, rec.Body.String())
	}
	var tempRoots []string
	for _, s := range srv.sessions {
		tempRoots = append(tempRoots, s.root)
	}
	if len(tempRoots) == 0 {
		t.Fatal("no session temp dir recorded")
	}
	for _, tr := range tempRoots {
		if _, err := os.Stat(tr); err != nil {
			t.Fatalf("temp dir %s missing before shutdown: %v", tr, err)
		}
	}
	srv.removeAllTemps()
	for _, tr := range tempRoots {
		if _, err := os.Stat(tr); !os.IsNotExist(err) {
			t.Fatalf("temp dir %s still exists after shutdown cleanup", tr)
		}
	}
}

// repoRoot finds the module root from the test's working dir.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found: not inside the boardctl repo")
		}
		dir = parent
	}
}
