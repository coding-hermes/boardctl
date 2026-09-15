package main

// BT-027: serve upload UX — an upload that registers 0 boards must warn
// VISIBLY in the report (payload upload_notice + #banner lead) when -C
// carries the response, and the -C-prepend rule is pinned by tests.
// The notice literal is re-declared here (not referenced from serve.go) so
// the RED proof can `git stash` the serve.go fix and still compile.

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// bt027Notice is the user-visible notice string (spec-facing literal).
const bt027Notice = "0 boards found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl"

// bt027CBoard seeds a real board dir and resolves it as the -C board. The
// header project is rewritten to "cboard" so the -C board is
// DISTINGUISHABLE in the payload from a same-seed uploaded board — that is
// what makes the prepend-order assertion able to fail.
func bt027CBoard(t *testing.T) *board.Board {
	t.Helper()
	dir := seedServeBoard(t, t.TempDir()+"/cproj/.coding-hermes/board", "A")
	header := `{"project": "cboard", "namespace": "ns", "version": 1, "ticks_total": 40, "ticks_idle": 10, "last_commit": null}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "board.jsonl"), []byte(header), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := board.Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// postUploadNamed is postUpload with an explicit part name (postUpload
// names parts after the map key; here the NAME is the bug class under
// test — a part not named `zip` must behave as a folder-upload entry).
func postUploadNamed(h http.Handler, name string, data []byte) *httptest.ResponseRecorder {
	return postUpload(h, map[string][]byte{name: data})
}

// TestBT027ZeroBoardUploadWithCWarnsVisibly: a request WITH file parts that
// registers 0 boards + a -C board -> HTTP 200 (the -C board still renders)
// AND the notice present in the HTML body (visible to browser AND curl).
func TestBT027ZeroBoardUploadWithCWarnsVisibly(t *testing.T) {
	cBoard := bt027CBoard(t)
	h := newServeServer([]*board.Board{cBoard}).routes()

	// The curl -F file=@x.jsonl mistake: a file part with the WRONG field
	// name and no board pair anywhere in it.
	rec := postUploadNamed(h, "file", []byte("{\"id\": \"X-1\", \"title\": \"not a board\"}\n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (-C board still renders); body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), bt027Notice) {
		t.Fatalf("200 report does not carry the visible notice %q", bt027Notice)
	}
	p := mustIslandPayload(t, rec.Body.String())
	if len(p.Boards) != 1 {
		t.Fatalf("payload boards = %d, want 1 (only the -C board)", len(p.Boards))
	}

	// Same verdict for a FOLDER upload whose tree lacks the board pair.
	loose := postUploadNamed(h, "files", []byte("just a stray text file\n"))
	if loose.Code != http.StatusOK {
		t.Fatalf("folder variant status = %d, want 200; body: %s", loose.Code, loose.Body.String())
	}
	if !strings.Contains(loose.Body.String(), bt027Notice) {
		t.Fatalf("folder variant report does not carry the notice %q", bt027Notice)
	}
}

// TestBT027ZeroBoardUploadNoCStill400: the same 0-board upload with NO -C
// board keeps the pre-existing 400 + message (nothing renders at all).
func TestBT027ZeroBoardUploadNoCStill400(t *testing.T) {
	h := newServeServer(nil).routes()
	rec := postUploadNamed(h, "file", []byte("{\"id\": \"X-1\"}\n"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (no board anywhere)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no board found in upload") {
		t.Fatalf("400 body lost the pair-rule message: %q", rec.Body.String())
	}
}

// TestBT027EmptyUploadStill400: a multipart request with NO file parts at
// all keeps the pre-existing 400 "no files uploaded" message.
func TestBT027EmptyUploadStill400(t *testing.T) {
	h := newServeServer([]*board.Board{bt027CBoard(t)}).routes()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormField("comment")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("no file parts here"))
	mw.Close()
	req := httptest.NewRequest("POST", "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "no files uploaded") {
		t.Fatalf("400 body = %q, want the no-files message", rec.Body.String())
	}
}

// TestBT027CombinedCountCPlusZip: spec 7.2.3's len(boards)==2 holds only
// when -C resolves nothing; with a -C board the SAME two-board zip yields
// len(boards)==3, -C board FIRST, and NO notice.
func TestBT027CombinedCountCPlusZip(t *testing.T) {
	cBoard := bt027CBoard(t)
	root := t.TempDir()
	dirA := seedServeBoard(t, root+"/projA/.coding-hermes/board", "A")
	dirB := seedServeBoard(t, root+"/projB", "B")
	zipBytes := zipFromPaths(t, root,
		filepath.Join(dirA, "board.jsonl"), filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"),
		filepath.Join(dirB, "tasks.jsonl"), filepath.Join(dirB, "events.jsonl"),
	)

	h := newServeServer([]*board.Board{cBoard}).routes()
	rec := postUploadNamed(h, "zip", zipBytes)
	if rec.Code != http.StatusOK {
		t.Fatalf("zip upload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	p := mustIslandPayload(t, rec.Body.String())
	if len(p.Boards) != 3 {
		t.Fatalf("combined payload boards = %d, want 3 (1 -C + 2 uploaded)", len(p.Boards))
	}
	// -C board is prepended: it must be FIRST in payload order (its header
	// project is "cboard", distinct from the seeded uploads).
	if p.Boards[0].Name != "cboard" {
		t.Fatalf("first board name = %q, want cboard (-C board must be prepended)", p.Boards[0].Name)
	}
	uploaded := map[string]bool{}
	for _, b := range p.Boards[1:] {
		if b.Name != "zipboarda" && b.Name != "zipboardb" {
			t.Errorf("unexpected uploaded board %q", b.Name)
		}
		uploaded[b.Name] = true
	}
	if len(uploaded) != 2 {
		t.Fatalf("uploaded boards after -C = %v, want zipboarda+zipboardb", uploaded)
	}
	if strings.Contains(rec.Body.String(), bt027Notice) {
		t.Fatal("successful combined upload must NOT carry the 0-boards notice")
	}
}

// TestBT027NoNoticeOnSuccessfulUploadWithC: a normal successful upload with
// -C present renders clean — no notice anywhere in body or payload.
func TestBT027NoNoticeOnSuccessfulUploadWithC(t *testing.T) {
	cBoard := bt027CBoard(t)
	root := t.TempDir()
	dirA := seedServeBoard(t, root+"/solo/.coding-hermes/board", "A")
	zipBytes := zipFromPaths(t, root,
		filepath.Join(dirA, "board.jsonl"), filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))

	h := newServeServer([]*board.Board{cBoard}).routes()
	rec := postUploadNamed(h, "zip", zipBytes)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d; body: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), bt027Notice) {
		t.Fatal("successful upload must NOT carry the 0-boards notice")
	}
	p := mustIslandPayload(t, rec.Body.String())
	if p.UploadNotice != "" {
		t.Fatalf("payload upload_notice = %q, want empty", p.UploadNotice)
	}
	if len(p.Boards) != 2 {
		t.Fatalf("boards = %d, want 2 (1 -C + 1 uploaded)", len(p.Boards))
	}
}

// TestBT027NoticeInPayloadAndRenderPathIsolation: the notice rides the
// payload as upload_notice (omitempty), and render.Build (single-board
// path) never sets it.
func TestBT027NoticeInPayloadAndRenderPathIsolation(t *testing.T) {
	// render -C path: no notice field in the payload or HTML.
	cBoard := bt027CBoard(t)
	ref, err := render.Build(cBoard.Dir, render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if ref.UploadNotice != "" {
		t.Fatalf("render.Build set upload_notice = %q, want empty", ref.UploadNotice)
	}
	html, err := render.RenderHTML(ref)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, bt027Notice) {
		t.Fatal("render -C HTML must not contain the serve notice")
	}

	// serve path: 0-board upload + -C -> notice in payload AND rendered HTML.
	h := newServeServer([]*board.Board{cBoard}).routes()
	rec := postUploadNamed(h, "file", []byte("stray\n"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	p := mustIslandPayload(t, rec.Body.String())
	if p.UploadNotice != bt027Notice {
		t.Fatalf("payload upload_notice = %q, want %q", p.UploadNotice, bt027Notice)
	}
	if !strings.Contains(rec.Body.String(), bt027Notice) {
		t.Fatal("rendered HTML does not carry the notice")
	}
}

// newMultipartWriter builds a multipart body from plain form FIELDS only
// (no file parts) — the genuinely-empty-upload shape for the 400 test.
