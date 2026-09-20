package main

// DF-BOARDCTL-6: end-to-end tests for serve's BROWSER folder-upload arm.
// Prior verification covered the form markup, the "files" field name and
// the zip path by reading them; nothing POSTed a real webkitdirectory-
// shaped multipart request through the HTTP surface. These tests do
// exactly that: GET the form, POST field name "files" with directory-
// relative filenames (the browser wire shape), then assert the board lands
// in /api/boards exactly once — including under DF-BOARDCTL-2
// replace-on-re-upload.

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// df6UploaderFormAttrs are the exact markup contracts the folder-upload
// E2E depends on — the attributes a real browser form posts against.
var df6UploaderFormAttrs = []string{
	`<form id="up" method="post" action="/" enctype="multipart/form-data">`,
	`<input type="file" id="folder" name="files" webkitdirectory multiple>`,
	`<input type="file" id="zipfile" name="zip" accept=".zip,application/zip">`,
}

// df6FolderParts reads a seeded board dir and returns webkitdirectory-style
// parts keyed by the FILENAME a browser sends: the forward-slash path
// relative to the chosen folder (directory depth preserved).
func df6FolderParts(t *testing.T, prefix, dir string) map[string][]byte {
	t.Helper()
	parts := map[string][]byte{}
	for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue // topology B has no board.jsonl
		}
		if err != nil {
			t.Fatal(err)
		}
		parts[path.Join(prefix, name)] = data
	}
	return parts
}

// postFolderUpload mirrors a browser directory submission: every part rides
// the form's folder input field name "files", its filename carrying the
// directory-relative path (the wire contract of webkitdirectory + multiple).
// Parts are sent in sorted order for deterministic bodies.
func postFolderUpload(t *testing.T, h http.Handler, parts map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, name := range names {
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(parts[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// df6APIBoards GETs /api/boards and decodes into the production entry type.
func df6APIBoards(t *testing.T, h http.Handler) []apiBoard {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/boards", nil))
	if rec.Code != 200 {
		t.Fatalf("/api/boards status = %d: %s", rec.Code, rec.Body.String())
	}
	var out []apiBoard
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("/api/boards json: %v (%q)", err, rec.Body.String())
	}
	return out
}

// df6CountNamed counts /api/boards entries with the given report name.
func df6CountNamed(boards []apiBoard, name string) int {
	n := 0
	for _, b := range boards {
		if b.Name == name {
			n++
		}
	}
	return n
}

// TestServeBrowserFolderUploadEndToEnd (DF-BOARDCTL-6 acceptance): GET the
// form -> POST a webkitdirectory-shaped multipart body on field "files" ->
// the boards render and appear in /api/boards EXACTLY ONCE, and a
// same-identity re-upload (files arm, then zip arm) replaces instead of
// duplicating (DF-BOARDCTL-2 behavior through the browser arm).
func TestServeBrowserFolderUploadEndToEnd(t *testing.T) {
	srv := newServeServer(nil)
	h := srv.routes()

	// 1. The form the browser renders must post what we are about to send:
	// multipart enctype, folder field "files", zip field "zip".
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range df6UploaderFormAttrs {
		if !strings.Contains(body, want) {
			t.Errorf("uploader form missing exact markup %q", want)
		}
	}

	// 2. Folder arm: ONE webkitdirectory POST of a folder holding two
	// boards at different depths — exactly what a browser sends when the
	// user picks a folder above two repos. Filenames carry the directory
	// path; the handler must preserve it.
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "repo-a", ".coding-hermes", "board"), "A")
	dirB := seedServeBoard(t, filepath.Join(root, "repo-b", "board"), "B")
	parts := df6FolderParts(t, "repo-a/.coding-hermes/board", dirA)
	for k, v := range df6FolderParts(t, "repo-b/board", dirB) {
		parts[k] = v
	}
	if len(parts) != 5 {
		t.Fatalf("fixture parts = %d, want 5 (3 for A, 2 for B)", len(parts))
	}
	recUp := postFolderUpload(t, h, parts)
	if recUp.Code != 200 {
		t.Fatalf("folder upload status = %d, body: %s", recUp.Code, recUp.Body.String())
	}
	payload := mustIslandPayload(t, recUp.Body.String())
	if len(payload.Boards) != 2 {
		t.Fatalf("folder upload registered %d boards, want 2 (browser filenames carry directory depth)", len(payload.Boards))
	}
	if payload.Boards[0].Name != "zipboarda" || payload.Boards[0].Topology != "A" ||
		payload.Boards[1].Name != "zipboardb" || payload.Boards[1].Topology != "B" {
		t.Fatalf("folder upload boards = %+v, want zipboarda/A + zipboardb/B", payload.Boards)
	}
	if payload.UploadNotice != "" {
		t.Errorf("clean folder upload set upload notice %q", payload.UploadNotice)
	}

	// 3. The live listing shows each board EXACTLY ONCE with the derived
	// counts (2 complete / 1 open / 3 total per seeded board).
	api := df6APIBoards(t, h)
	if len(api) != 2 {
		t.Fatalf("/api/boards entries = %d, want 2", len(api))
	}
	if got := df6CountNamed(api, "zipboarda"); got != 1 {
		t.Errorf("zipboarda appears %d times in /api/boards, want 1", got)
	}
	if got := df6CountNamed(api, "zipboardb"); got != 1 {
		t.Errorf("zipboardb appears %d times in /api/boards, want 1", got)
	}
	for _, b := range api {
		if b.Total != 3 || b.Done != 2 || b.Open != 1 {
			t.Errorf("board %s counts = total %d done %d open %d, want 3/2/1", b.Name, b.Total, b.Done, b.Open)
		}
	}

	// 4. DF-BOARDCTL-2 through the BROWSER arm: re-upload the same board
	// (same header name, different source folder) via the files field —
	// must REPLACE, never duplicate.
	root2 := t.TempDir()
	dirA2 := seedServeBoard(t, filepath.Join(root2, "repo-c", ".coding-hermes", "board"), "A")
	recRe := postFolderUpload(t, h, df6FolderParts(t, "repo-c/.coding-hermes/board", dirA2))
	if recRe.Code != 200 {
		t.Fatalf("folder re-upload status = %d, body: %s", recRe.Code, recRe.Body.String())
	}
	if p := mustIslandPayload(t, recRe.Body.String()); len(p.Boards) != 1 || p.Boards[0].Name != "zipboarda" {
		t.Fatalf("re-upload report boards wrong: %+v", p.Boards)
	}
	api = df6APIBoards(t, h)
	if len(api) != 2 || df6CountNamed(api, "zipboarda") != 1 || df6CountNamed(api, "zipboardb") != 1 {
		t.Fatalf("after re-upload /api/boards = %+v, want zipboarda once + zipboardb once", api)
	}

	// 5. Zip arm of the same form: field "zip" with a fresh copy of the B
	// board replaces its identity too — still exactly two entries.
	root3 := t.TempDir()
	dirB2 := seedServeBoard(t, filepath.Join(root3, "zipper"), "B")
	zipBytes := zipFromPaths(t, root3,
		filepath.Join(dirB2, "tasks.jsonl"), filepath.Join(dirB2, "events.jsonl"))
	recZip := postUpload(h, map[string][]byte{"zip": zipBytes})
	if recZip.Code != 200 {
		t.Fatalf("zip upload status = %d, body: %s", recZip.Code, recZip.Body.String())
	}
	api = df6APIBoards(t, h)
	if len(api) != 2 || df6CountNamed(api, "zipboarda") != 1 || df6CountNamed(api, "zipboardb") != 1 {
		t.Fatalf("after zip re-upload /api/boards = %+v, want 2 distinct entries", api)
	}

	// Session bookkeeping after two replace rounds: the A+B session was
	// compacted to B-only, the A' and B' sessions appended, and the fully
	// superseded session's temp tree dropped with it.
	srv.lock()
	sessions := len(srv.sessions)
	var roots []string
	for _, s := range srv.sessions {
		roots = append(roots, s.root)
	}
	srv.unlock()
	if sessions != 2 {
		t.Errorf("live sessions = %d, want 2 (superseded boards dropped)", sessions)
	}
	for _, r := range roots {
		if _, err := os.Stat(r); err != nil {
			t.Errorf("surviving session temp dir %s missing: %v", r, err)
		}
	}
}

// TestServeFolderUploadEscapePartStaysInside mirrors the zip arm's zip-slip
// defense for the folder arm: a part whose FILENAME tries to escape
// (../evil.txt) must never land outside the extraction root, and the
// report still renders. (Filename normalization must keep flowing through
// safeJoin even after the raw wire filename is recovered.)
func TestServeFolderUploadEscapePartStaysInside(t *testing.T) {
	outside := t.TempDir()
	canary := filepath.Join(outside, "evil.txt")

	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "ok"), "A")
	parts := df6FolderParts(t, "board", dirA)
	parts["../evil.txt"] = []byte("pwned")

	srv := newServeServer(nil)
	rec := postFolderUpload(t, srv.routes(), parts)
	if rec.Code != 200 {
		t.Fatalf("status = %d (report must still render), body: %s", rec.Code, rec.Body.String())
	}
	if p := mustIslandPayload(t, rec.Body.String()); len(p.Boards) != 1 || p.Boards[0].Name != "zipboarda" {
		t.Fatalf("boards = %+v, want zipboarda alone", p.Boards)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Fatal("escape-named part created a file OUTSIDE the extraction root")
	}
	// Escape parts are anchored INSIDE the extraction root (same semantics
	// as the zip arm's ../ entries). Soft-logged: a future hardening that
	// rejects such parts outright is an improvement, not a regression.
	srv.lock()
	var sessRoot string
	if len(srv.sessions) == 1 {
		sessRoot = srv.sessions[0].root
	}
	srv.unlock()
	if sessRoot == "" {
		t.Fatal("no live session after upload")
	}
	if _, err := os.Stat(filepath.Join(sessRoot, "evil.txt")); err != nil {
		t.Logf("note: escape part anchored at %s/evil.txt not found: %v", sessRoot, err)
	}
}
