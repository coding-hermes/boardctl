package main

// DF-BOARDCTL-2 regression tests: serve used to append EVERY upload to the
// live board list, so re-uploading the same board duplicated it in the
// report, /api/boards and the compare view. The fix replaces by stable
// identity (report-slug + topology). These tests pin replace-on-same /
// append-on-different over the HTTP surface (httptest against
// newServeServer().routes()) plus the identity/merge units.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// df2APIBoards GETs /api/boards and decodes the listing (name/slug/...).
func df2APIBoards(t *testing.T, h http.Handler) []struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Topology string `json:"topology"`
	Total    int    `json:"task_total"`
	Done     int    `json:"done"`
	Open     int    `json:"open"`
} {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/boards", nil))
	if rec.Code != 200 {
		t.Fatalf("/api/boards status = %d: %s", rec.Code, rec.Body.String())
	}
	var out []struct {
		Name     string `json:"name"`
		Slug     string `json:"slug"`
		Topology string `json:"topology"`
		Total    int    `json:"task_total"`
		Done     int    `json:"done"`
		Open     int    `json:"open"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("/api/boards json: %v (%q)", err, rec.Body.String())
	}
	return out
}

// df2BoardZip seeds one board under a fresh temp root and returns its zip.
// The board NAME comes from the topology header (seedServeBoard: A ->
// "zipboarda", B -> "zipboardb"), never from the directory name, so two
// uploads of the same seed are the same identity from different paths.
func df2BoardZip(t *testing.T, topology string) []byte {
	t.Helper()
	root := t.TempDir()
	// B is a plain dir (no board.jsonl to include in the zip).
	sub := "boarddir"
	if topology != "A" {
		sub = filepath.Join("b", "boarddir")
	}
	dir := seedServeBoard(t, filepath.Join(root, sub), topology)
	paths := []string{filepath.Join(dir, "tasks.jsonl"), filepath.Join(dir, "events.jsonl")}
	if topology == "A" {
		paths = append([]string{filepath.Join(dir, "board.jsonl")}, paths...)
	}
	return zipFromPaths(t, root, paths...)
}

// TestServeReuploadSameBoardReplaces (DF-BOARDCTL-2 acceptance 1): upload a
// board, then re-upload the SAME board (same report name, different source
// path): /api/boards shows ONE entry, the upload response report has no
// duplicated boards, and only the newest upload session survives (the
// superseded session's temp tree is gone).
func TestServeReuploadSameBoardReplaces(t *testing.T) {
	srv := newServeServer(nil)
	h := srv.routes()

	rec1 := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "A")})
	if rec1.Code != 200 {
		t.Fatalf("first upload status = %d: %s", rec1.Code, rec1.Body.String())
	}
	if p := mustIslandPayload(t, rec1.Body.String()); len(p.Boards) != 1 {
		t.Fatalf("first upload boards = %d, want 1", len(p.Boards))
	}

	// Same board identity again (fresh seed: same project name, different
	// extraction path) — must REPLACE, not append.
	rec2 := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "A")})
	if rec2.Code != 200 {
		t.Fatalf("re-upload status = %d: %s", rec2.Code, rec2.Body.String())
	}
	p2 := mustIslandPayload(t, rec2.Body.String())
	if len(p2.Boards) != 1 {
		t.Fatalf("re-upload report boards = %d, want 1 (same-identity re-upload must replace)", len(p2.Boards))
	}

	api := df2APIBoards(t, h)
	if len(api) != 1 {
		t.Fatalf("/api/boards entries = %d, want 1 after re-upload", len(api))
	}
	if api[0].Name != "zipboarda" || api[0].Topology != "A" {
		t.Errorf("/api/boards entry = %+v, want zipboarda/A", api[0])
	}
	if api[0].Total != 3 || api[0].Done != 2 || api[0].Open != 1 {
		t.Errorf("/api/boards counts = %+v, want the newest board's 3/2/1", api[0])
	}

	// Session bookkeeping: exactly one live session (the newest), and the
	// superseded session's temp tree was removed with it.
	srv.lock()
	sessions := len(srv.sessions)
	var roots []string
	for _, s := range srv.sessions {
		roots = append(roots, s.root)
	}
	srv.unlock()
	if sessions != 1 {
		t.Fatalf("live sessions = %d, want 1 (superseded session dropped)", sessions)
	}
	for _, r := range roots {
		if _, err := os.Stat(r); err != nil {
			t.Errorf("surviving session temp dir %s missing: %v", r, err)
		}
	}
}

// TestServeReuploadDifferentBoardAppends (DF-BOARDCTL-2 acceptance 2): a
// DIFFERENT board still appends, re-uploads of either leave two entries in
// first-seen order, and the compare payload carries both boards.
func TestServeReuploadDifferentBoardAppends(t *testing.T) {
	srv := newServeServer(nil)
	h := srv.routes()

	recA := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "A")})
	recB := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "B")})
	if recA.Code != 200 || recB.Code != 200 {
		t.Fatalf("uploads: A=%d B=%d (%s / %s)", recA.Code, recB.Code, recA.Body.String(), recB.Body.String())
	}
	// Per-upload response semantics (pre-existing): each report carries
	// -C plus THAT upload's boards, so B's response shows B alone. The
	// append to the LIVE listing is what /api/boards proves below.
	pB := mustIslandPayload(t, recB.Body.String())
	if len(pB.Boards) != 1 || pB.Boards[0].Name != "zipboardb" || pB.Boards[0].Topology != "B" {
		t.Fatalf("B upload report = %+v, want single zipboardb/B", pB.Boards)
	}
	for _, b := range pB.Boards {
		if b.Error != "" {
			t.Errorf("board %s unexpected error state: %s", b.Name, b.Error)
		}
	}

	api := df2APIBoards(t, h)
	if len(api) != 2 {
		t.Fatalf("/api/boards entries = %d, want 2", len(api))
	}
	if api[0].Name != "zipboarda" || api[1].Name != "zipboardb" {
		t.Fatalf("/api/boards order = %s,%s; want zipboarda,zipboardb (first-seen)", api[0].Name, api[1].Name)
	}

	// Re-upload the B board: still two entries, same order, B replaced.
	recB2 := postUpload(h, map[string][]byte{"zip": df2BoardZip(t, "B")})
	if recB2.Code != 200 {
		t.Fatalf("B re-upload status = %d: %s", recB2.Code, recB2.Body.String())
	}
	api = df2APIBoards(t, h)
	if len(api) != 2 {
		t.Fatalf("/api/boards entries after B re-upload = %d, want 2", len(api))
	}
	if api[0].Name != "zipboarda" || api[1].Name != "zipboardb" {
		t.Fatalf("/api/boards order after B re-upload = %s,%s; want zipboarda,zipboardb", api[0].Name, api[1].Name)
	}

	// Two live sessions (A's and the newest B's), not three.
	srv.lock()
	sessions := len(srv.sessions)
	srv.unlock()
	if sessions != 2 {
		t.Fatalf("live sessions = %d, want 2 (B's first session superseded)", sessions)
	}
}

// checkIdentityMatchesRender uses render as the oracle: serve's identity
// slug part must equal the slug render puts in the report for this board.
func checkIdentityMatchesRender(t *testing.T, b *board.Board) {
	t.Helper()
	payload, err := render.BuildBoards([]*board.Board{b}, render.Options{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	want := payload.Boards[0].Slug + "|" + b.Topology
	if got := serveBoardIdentity(b); got != want {
		t.Errorf("serveBoardIdentity(%s) = %q, want %q (render slug %q)", b.Dir, got, want, payload.Boards[0].Slug)
	}
}

// TestServeBoardIdentityMatchesRender pins the identity mirror: header-name
// boards (topology A and B) and a headerless dir-fallback board all get the
// exact slug render displays.
func TestServeBoardIdentityMatchesRender(t *testing.T) {
	root := t.TempDir()

	dirA := seedServeBoard(t, filepath.Join(root, "projA"), "A")
	bA, err := board.Resolve(dirA)
	if err != nil {
		t.Fatal(err)
	}
	checkIdentityMatchesRender(t, bA)
	if got := serveBoardIdentity(bA); got != "zipboarda|A" {
		t.Errorf("topology A identity = %q, want zipboarda|A", got)
	}

	dirB := seedServeBoard(t, filepath.Join(root, "projB"), "B")
	bB, err := board.Resolve(dirB)
	if err != nil {
		t.Fatal(err)
	}
	checkIdentityMatchesRender(t, bB)
	if got := serveBoardIdentity(bB); got != "zipboardb|B" {
		t.Errorf("topology B identity = %q, want zipboardb|B", got)
	}

	// Headerless board (no board.jsonl, tasks line 1 is a task row): name
	// falls back to the directory base name — slugified just like render.
	fall := filepath.Join(root, "My Weird Board!")
	if err := copySeedHeaderless(t, fall); err != nil {
		t.Fatal(err)
	}
	bF, err := board.Resolve(fall)
	if err != nil {
		t.Fatal(err)
	}
	checkIdentityMatchesRender(t, bF)
	if got := serveBoardIdentity(bF); got != "my-weird-board|B" {
		t.Errorf("fallback identity = %q, want my-weird-board|B", got)
	}
}

// copySeedHeaderless writes a minimal headerless board (tasks line 1 = task
// row, no board.jsonl) into dir.
func copySeedHeaderless(t *testing.T, dir string) error {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tasks := `{"id": "H-1", "title": "only", "status": "pending", "created_at": "2026-09-01 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(""), 0o644)
}

// TestMergeByIdentityUnit: later same-identity entries replace earlier ones
// at the first-seen position; different identities keep order; different
// topologies with the same name stay distinct.
func TestMergeByIdentityUnit(t *testing.T) {
	root := t.TempDir()
	dirA1 := seedServeBoard(t, filepath.Join(root, "a1"), "A")
	dirA2 := seedServeBoard(t, filepath.Join(root, "a2"), "A") // same name zipboarda
	dirB := seedServeBoard(t, filepath.Join(root, "b1"), "B")
	bA1, _ := board.Resolve(dirA1)
	bA2, _ := board.Resolve(dirA2)
	bB, _ := board.Resolve(dirB)

	merged := mergeByIdentity([]*board.Board{bA1, bB, bA2})
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2 (A1 replaced by A2)", len(merged))
	}
	if merged[0] != bA2 || merged[1] != bB {
		t.Fatalf("merged order wrong: want [A2 B], got [%s %s]", merged[0].Dir, merged[1].Dir)
	}

	// Same display name but different topologies: distinct identities.
	dupA := filepath.Join(root, "dup", "ta")
	dupB := filepath.Join(root, "dup", "tb")
	if err := writeNamedBoard(t, dupA, "A", "samename"); err != nil {
		t.Fatal(err)
	}
	if err := writeNamedBoard(t, dupB, "B", "samename"); err != nil {
		t.Fatal(err)
	}
	bdA, _ := board.Resolve(dupA)
	bdB, _ := board.Resolve(dupB)
	if got := serveBoardIdentity(bdA); got != "samename|A" {
		t.Errorf("dup A identity = %q, want samename|A", got)
	}
	if got := serveBoardIdentity(bdB); got != "samename|B" {
		t.Errorf("dup B identity = %q, want samename|B", got)
	}
	if merged := mergeByIdentity([]*board.Board{bdA, bdB}); len(merged) != 2 {
		t.Fatalf("same-name different-topology merged len = %d, want 2", len(merged))
	}

	// nil entries are dropped, not merged.
	if merged := mergeByIdentity([]*board.Board{nil, bA1}); len(merged) != 1 || merged[0] != bA1 {
		t.Fatalf("nil handling wrong: len=%d", len(merged))
	}
}

// writeNamedBoard seeds a board whose display name is the given project
// (topology A: board.jsonl header; topology B: tasks line-1 header).
func writeNamedBoard(t *testing.T, dir, topology, project string) error {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tasks := `{"id": "N-1", "title": "t", "status": "pending", "created_at": "2026-09-01 00:00:00"}` + "\n"
	if topology == "A" {
		hdr := `{"project": "` + project + `", "namespace": "ns", "version": 1, "ticks_total": 5}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "board.jsonl"), []byte(hdr), 0o644); err != nil {
			return err
		}
	} else {
		tasks = `{"project": "` + project + `", "namespace": "ns", "version": 1, "ticks_total": 5}` + "\n" + tasks
	}
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(""), 0o644)
}

// TestRegisterSessionSupersedes pins the session-level replacement: a
// full-identity match removes the old session (temp tree deleted), a
// partial match keeps the surviving boards with their identities aligned.
func TestRegisterSessionSupersedes(t *testing.T) {
	root := t.TempDir()
	bA1, _ := board.Resolve(seedServeBoard(t, filepath.Join(root, "s1"), "A"))
	bA2, _ := board.Resolve(seedServeBoard(t, filepath.Join(root, "s2"), "A"))
	bB, _ := board.Resolve(seedServeBoard(t, filepath.Join(root, "s3"), "B"))

	srv := newServeServer(nil)
	first := &uploadSession{root: filepath.Join(root, "tmp1"), boards: []*board.Board{bA1}, identities: []string{serveBoardIdentity(bA1)}}
	srv.registerSession(first)

	second := &uploadSession{root: filepath.Join(root, "tmp2"), boards: []*board.Board{bA2}, identities: []string{serveBoardIdentity(bA2)}}
	srv.registerSession(second)

	srv.lock()
	if len(srv.sessions) != 1 {
		srv.unlock()
		t.Fatalf("sessions = %d, want 1 after full supersede", len(srv.sessions))
	}
	got := srv.sessions[0]
	srv.unlock()
	if got.root != second.root {
		t.Errorf("surviving session root = %s, want the newest %s", got.root, second.root)
	}
	if _, err := os.Stat(first.root); !os.IsNotExist(err) {
		t.Errorf("superseded session temp dir %s still exists (err=%v)", first.root, err)
	}

	// Partial supersede: a session carrying B and A; re-upload A keeps B.
	third := &uploadSession{
		root:       filepath.Join(root, "tmp3"),
		boards:     []*board.Board{bB, bA2},
		identities: []string{serveBoardIdentity(bB), serveBoardIdentity(bA2)},
	}
	srv.registerSession(third)
	replacement := &uploadSession{root: filepath.Join(root, "tmp4"), boards: []*board.Board{bA1}, identities: []string{serveBoardIdentity(bA1)}}
	srv.registerSession(replacement)

	srv.lock()
	n := len(srv.sessions)
	srv.unlock()
	if n != 2 {
		t.Fatalf("sessions after partial supersede = %d, want 2 (third keeps B; replacement added)", n)
	}
	cb := srv.currentBoards()
	if len(cb) != 2 {
		t.Fatalf("currentBoards = %d, want 2", len(cb))
	}
	if cb[0] != bB || cb[1] != bA1 {
		t.Errorf("currentBoards order = [%s %s], want [B A1]", cb[0].Dir, cb[1].Dir)
	}
}
