package main

// BT-075 acceptance 3: the UI renders a REAL board. Beyond the synthetic
// fixtures of bt072_webui_test.go, this file proves the new surface against
// THIS repo's own live board (.coding-hermes/board) — read-only: the files
// are read to build an in-memory zip exactly as the webkitdirectory upload
// would, and TestBT072NoBoardWrites already proves the route exercise
// writes nothing. The same battery also pins the real-rows assertion of
// TestServeFolderUploadMatchesRender on the new endpoints.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// loadLiveBoardZip reads the repo's own board files (read-only) into the
// multipart field map a webkitdirectory upload would send.
func loadLiveBoardZip(t *testing.T) map[string][]byte {
	t.Helper()
	boardDir := filepath.Join(repoRoot(t), ".coding-hermes", "board")
	fields := map[string][]byte{}
	for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"} {
		data, err := os.ReadFile(filepath.Join(boardDir, name))
		if err != nil {
			t.Fatalf("live board file %s unreadable: %v", name, err)
		}
		fields[filepath.Join(".coding-hermes", "board", name)] = data
	}
	return fields
}

// TestBT072LiveBoardRenders: the repo's own board renders through the new
// surface — real rows, real events, real validation output in /api
// responses and the /ui island.
func TestBT072LiveBoardRenders(t *testing.T) {
	fields := loadLiveBoardZip(t)

	srv := newServeServer(nil)
	h := srv.routes()
	rec := postUpload(h, fields)
	if rec.Code != 200 {
		t.Fatalf("live board upload status = %d: %s", rec.Code, rec.Body.String())
	}
	served := mustIslandPayload(t, rec.Body.String())
	if len(served.Boards) != 1 {
		t.Fatalf("live upload produced %d boards, want 1", len(served.Boards))
	}
	slug := served.Boards[0].Slug
	if slug == "" {
		t.Fatal("live board has an empty slug")
	}

	// The /api read model carries REAL rows and events (>= the counts the
	// report payload derivation produced at upload).
	d := bt072APIDetail(t, h, slug)
	if len(d.Tasks) != len(served.Boards[0].Tasks) || len(d.Tasks) == 0 {
		t.Fatalf("live board rows: api %d, report payload %d (want equal, non-zero)", len(d.Tasks), len(served.Boards[0].Tasks))
	}
	if len(d.Events) != len(served.Boards[0].Events) {
		t.Fatalf("live board events: api %d, report payload %d", len(d.Events), len(served.Boards[0].Events))
	}
	// Real validation output: the census line and pill are populated.
	if d.Validate.Census == "" {
		t.Fatal("live board read model carries no key-uniformity census (R13)")
	}
	if d.Validate.Pill == "" {
		t.Fatal("live board read model carries no validate pill")
	}
	if d.Validate.Sweep.Rows == 0 {
		t.Fatal("live board sweep census examined zero rows")
	}

	// Real events flow through the timeline endpoint with decoded details.
	evs := bt072APIEvents(t, h, slug)
	if len(evs) == 0 {
		t.Fatal("live board timeline is empty")
	}

	// The /ui island embeds the same real board.
	body := bt072UIBody(t, h)
	island := bt072Island(t, body)
	var ib *uiBoard
	for i := range island.Boards {
		if island.Boards[i].Slug == slug {
			ib = &island.Boards[i]
		}
	}
	if ib == nil {
		t.Fatalf("live board %q missing from the /ui island (%d boards)", slug, len(island.Boards))
	}
	if len(ib.Tasks) != len(d.Tasks) || len(ib.Events) != len(d.Events) {
		t.Fatalf("island row/event counts drifted from the api read model")
	}
	if ib.Validate.Census != d.Validate.Census {
		t.Fatalf("island census != api census: %q vs %q", ib.Validate.Census, d.Validate.Census)
	}
	// The derived numbers are Go's — island equals the report payload for
	// the same board, byte-for-byte.
	gotJSON, _ := json.Marshal(ib.Derived)
	wantJSON, _ := json.Marshal(served.Boards[0].Derived)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("island derived != report derived for the live board:\n got  %s\n want %s", gotJSON, wantJSON)
	}

	// The snapshot registry holds the live board exactly once (identity dedupe).
	boards := df2APIBoards(t, h)
	n := 0
	for _, b := range boards {
		if b.Slug == slug {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("live board appears %d times in /api/boards, want 1", n)
	}
}
