package render

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// ---------- DF-BOARDCTL-5: per-board data-quality footnote ----------

const noDoneTaskB1 = `{"id":"B1-NODONE","title":"t","status":"complete"}` // complete, no created_at/done -> uncomputable
const noDoneTaskB2 = `{"id":"B2-NODONE","title":"t","status":"complete"}` // same shape, different board

// islandMaps parses the report-data JSON island (the exact payload the
// client JS reads; footnotes render client-side from it).
func islandMaps(t *testing.T, html string) (map[string]any, []any) {
	t.Helper()
	var raw map[string]any
	dec := json.NewDecoder(strings.NewReader(html[strings.Index(html, `{"schema"`):]))
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("island unparseable: %v", err)
	}
	boards, _ := raw["boards"].([]any)
	return raw, boards
}

// boardNoDone returns a board island object's completed_no_done_ids and
// whether the key is present at all.
func boardNoDone(t *testing.T, b any) ([]string, bool) {
	t.Helper()
	m, ok := b.(map[string]any)
	if !ok {
		t.Fatalf("board island entry not an object: %v", b)
	}
	v, present := m["completed_no_done_ids"]
	if !present {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		t.Fatalf("completed_no_done_ids not an array: %v", v)
	}
	var ids []string
	for _, x := range arr {
		s, _ := x.(string)
		ids = append(ids, s)
	}
	return ids, true
}

// TestBuildBoardsNoDoneFootnotePerBoard pins DF-BOARDCTL-5's acceptance: in
// a multi-board (serve) report each board's payload — and therefore each
// board's client-rendered footnote — carries only its OWN no-done ids, and
// the old payload-level union key is gone from the island.
func TestBuildBoardsNoDoneFootnotePerBoard(t *testing.T) {
	rootA := seedBoard(t, map[string]string{
		"board.jsonl":  `{"project": "alpha", "namespace": "ns", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": null}` + "\n",
		"tasks.jsonl":  taskLine("A1", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" + noDoneTaskB1 + "\n",
		"events.jsonl": emptyEvents,
	})
	rootB := seedBoard(t, map[string]string{
		"board.jsonl":  `{"project": "beta", "namespace": "ns", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": null}` + "\n",
		"tasks.jsonl":  taskLine("B1", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" + noDoneTaskB2 + "\n",
		"events.jsonl": emptyEvents,
	})
	rp, err := BuildBoards([]*board.Board{resolveSeed(t, rootA), resolveSeed(t, rootB)},
		Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if len(rp.Boards) != 2 {
		t.Fatalf("boards = %d, want 2", len(rp.Boards))
	}
	if got := rp.Boards[0].NoDoneCompleted; !reflect.DeepEqual(got, []string{"B1-NODONE"}) {
		t.Fatalf("alpha no-done = %v, want [B1-NODONE]", got)
	}
	if got := rp.Boards[1].NoDoneCompleted; !reflect.DeepEqual(got, []string{"B2-NODONE"}) {
		t.Fatalf("beta no-done = %v, want [B2-NODONE]", got)
	}

	html, err := RenderHTML(rp)
	if err != nil {
		t.Fatal(err)
	}
	raw, boards := islandMaps(t, html)
	if _, present := raw["completed_no_done_ids"]; present {
		t.Fatal("payload-level completed_no_done_ids union must be gone from the island")
	}
	idsA, okA := boardNoDone(t, boards[0])
	idsB, okB := boardNoDone(t, boards[1])
	if !okA || !reflect.DeepEqual(idsA, []string{"B1-NODONE"}) {
		t.Fatalf("alpha island no-done = %v present=%v, want [B1-NODONE]", idsA, okA)
	}
	if !okB || !reflect.DeepEqual(idsB, []string{"B2-NODONE"}) {
		t.Fatalf("beta island no-done = %v present=%v, want [B2-NODONE]", idsB, okB)
	}
}

// TestBuildSingleBoardNoDoneFootnote pins the single-board (render) path:
// the footnote ids list only that board's rows.
func TestBuildSingleBoardNoDoneFootnote(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" + noDoneTaskB1 + "\n",
		"events.jsonl": emptyEvents,
	})
	rp, err := Build(root, Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.Boards[0].NoDoneCompleted; !reflect.DeepEqual(got, []string{"B1-NODONE"}) {
		t.Fatalf("no-done = %v, want [B1-NODONE]", got)
	}
	html, err := RenderHTML(rp)
	if err != nil {
		t.Fatal(err)
	}
	raw, boards := islandMaps(t, html)
	if _, present := raw["completed_no_done_ids"]; present {
		t.Fatal("single-board island must not carry a payload-level completed_no_done_ids")
	}
	ids, ok := boardNoDone(t, boards[0])
	if !ok || !reflect.DeepEqual(ids, []string{"B1-NODONE"}) {
		t.Fatalf("island no-done = %v present=%v, want [B1-NODONE]", ids, ok)
	}
}

// A board whose rows all have computable done days must carry no footnote
// ids at all (omitempty drops the key entirely).
func TestNoDoneFootnoteAbsentWhenClean(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n",
		"events.jsonl": emptyEvents,
	})
	rp, err := BuildBoards([]*board.Board{resolveSeed(t, root)},
		Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if got := rp.Boards[0].NoDoneCompleted; len(got) != 0 {
		t.Fatalf("clean board no-done = %v, want empty", got)
	}
	html, err := RenderHTML(rp)
	if err != nil {
		t.Fatal(err)
	}
	_, boards := islandMaps(t, html)
	if ids, present := boardNoDone(t, boards[0]); present {
		t.Fatalf("clean board island must omit completed_no_done_ids, got %v", ids)
	}
}

// TestTemplateFootnotePerBoard proves the client renders the footnote from
// the PER-BOARD field: the old payload-level read must be gone.
func TestTemplateFootnotePerBoard(t *testing.T) {
	h := mustTemplateHTML(t)
	for _, want := range []string{
		`bd.completed_no_done_ids`,
		`"Data quality — " + bd.name`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("template missing per-board footnote piece %q", want)
		}
	}
	if strings.Contains(h, `payload.completed_no_done_ids`) {
		t.Fatal("template still reads the payload-level completed_no_done_ids (fleet-global footnote)")
	}
}
