package board

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedImportPrimitivesBoard writes a minimal topology-A board (spaced-style
// tasks, compact events) for the import primitive tests.
func seedImportPrimitivesBoard(t *testing.T) (*Board, string) {
	t.Helper()
	dir := t.TempDir()
	bd := filepath.Join(dir, "board")
	if err := os.MkdirAll(bd, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"board.jsonl": `{"project":"p","namespace":"p","version":1,"ticks_total":2,"ticks_idle":0,"last_commit":null}` + "\n",
		"tasks.jsonl": `{"id": "T-1", "title": "one", "status": "pending", "priority": "P2", "reasoning": "original"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-01 00:00:00","event_type":"task_created","task_id":"T-1","actor":"foreman","detail":null,"tick_number":null}` + "\n" +
			`{"id":2,"timestamp":"2026-09-02 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":null}` + "\n",
		"fixtures.jsonl": `{"id": "FIX-1", "status": "pending"}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(bd, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Resolve(bd)
	if err != nil {
		t.Fatal(err)
	}
	return b, bd
}

func nonEmptyLines(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// TestAppendImportTaskLineRoundTrip pins the byte-preserving append
// contract: the raw line lands verbatim as its own terminated line.
func TestAppendImportTaskLineRoundTrip(t *testing.T) {
	b, bd := seedImportPrimitivesBoard(t)
	before := nonEmptyLines(filepath.Join(bd, "tasks.jsonl"))
	line := `{"id": "T-2", "title": "two", "status": "complete", "priority": "P1", "nested": {"b": 1, "a": [1, 2]}}`
	if err := b.AppendImportTaskLine([]byte(line)); err != nil {
		t.Fatal(err)
	}
	after := nonEmptyLines(filepath.Join(bd, "tasks.jsonl"))
	if len(after) != len(before)+1 {
		t.Fatalf("tasks.jsonl grew by %d, want 1", len(after)-len(before))
	}
	if after[len(after)-1] != line {
		t.Fatalf("appended line not byte-identical:\n got %s\nwant %s", after[len(after)-1], line)
	}
	// Header untouched.
	hdr := nonEmptyLines(filepath.Join(bd, "board.jsonl"))
	if len(hdr) != 1 || !strings.Contains(hdr[0], `"project"`) {
		t.Fatal("board.jsonl header changed")
	}
}

// TestAppendImportTaskLineRejectsBad pins validation-before-write.
func TestAppendImportTaskLineRejectsBad(t *testing.T) {
	b, bd := seedImportPrimitivesBoard(t)
	before, _ := os.ReadFile(filepath.Join(bd, "tasks.jsonl"))
	for _, bad := range []string{"", "   ", `[1,2]`, `{"title":"no id"}`, `not json`} {
		if err := b.AppendImportTaskLine([]byte(bad)); err == nil {
			t.Fatalf("AppendImportTaskLine(%q) should fail", bad)
		}
	}
	after, _ := os.ReadFile(filepath.Join(bd, "tasks.jsonl"))
	if string(before) != string(after) {
		t.Fatal("rejected task lines changed tasks.jsonl")
	}
}

// TestImportEventWriterSequencing: fresh MAX(id)+1 ids, target template and
// dialect, header untouched, vocabulary enforced before write.
func TestImportEventWriterSequencing(t *testing.T) {
	b, bd := seedImportPrimitivesBoard(t)
	w, err := b.NewImportEventWriter()
	if err != nil {
		t.Fatal(err)
	}
	if w.Next() != 3 {
		t.Fatalf("Next() = %d, want 3", w.Next())
	}
	detail := json.RawMessage(`{"k":"v"}`)
	if err := w.Append(ImportEvent{Type: "task_started", TaskID: "T-1", Detail: detail}); err != nil {
		t.Fatal(err)
	}
	if err := w.Append(ImportEvent{}); err != nil { // defaults: audit + foreman
		t.Fatal(err)
	}
	if w.Next() != 5 {
		t.Fatalf("Next() after appends = %d, want 5", w.Next())
	}
	lines := nonEmptyLines(filepath.Join(bd, "events.jsonl"))
	if len(lines) != 4 {
		t.Fatalf("events.jsonl has %d rows, want 4", len(lines))
	}
	var e3, e4 map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &e3); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[3]), &e4); err != nil {
		t.Fatal(err)
	}
	if e3["id"].(float64) != 3 || e3["event_type"] != "task_started" || e3["detail"] == nil {
		t.Fatalf("event 3 wrong: %v", e3)
	}
	if e4["id"].(float64) != 4 || e4["event_type"] != "audit" || e4["actor"] != "foreman" || e4["detail"] != nil {
		t.Fatalf("event 4 wrong: %v", e4)
	}
	// Compact style mirrored from the last existing event row (not the
	// spaced default): the appended lines must not contain ", ".
	if strings.Contains(lines[2], `", "`) {
		t.Fatalf("appended event is spaced; target style is compact: %s", lines[2])
	}
	// Timestamp dialect: target uses "2006-01-02 15:04:05".
	if ts, _ := e3["timestamp"].(string); len(ts) != 19 || ts[10] != ' ' {
		t.Fatalf("appended timestamp %q not in target dialect", ts)
	}
	// Header ticks_total untouched (import never bumps).
	hdrLines := nonEmptyLines(filepath.Join(bd, "board.jsonl"))
	if !strings.Contains(hdrLines[0], `"ticks_total":2`) {
		t.Fatalf("header ticks_total changed: %s", hdrLines[0])
	}
	// Vocabulary rejection before write.
	n := len(nonEmptyLines(filepath.Join(bd, "events.jsonl")))
	if err := w.Append(ImportEvent{Type: "not-a-type"}); err == nil {
		t.Fatal("unknown event_type must be rejected")
	}
	if after := len(nonEmptyLines(filepath.Join(bd, "events.jsonl"))); after != n {
		t.Fatal("rejected event left a partial write")
	}
}

// TestImportEventWriterEmptyFile: an empty events.jsonl yields id 1 and the
// built-in default key set.
func TestImportEventWriterEmptyFile(t *testing.T) {
	b, bd := seedImportPrimitivesBoard(t)
	if err := os.WriteFile(filepath.Join(bd, "events.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := b.NewImportEventWriter()
	if err != nil {
		t.Fatal(err)
	}
	if w.Next() != 1 {
		t.Fatalf("Next() on empty file = %d, want 1", w.Next())
	}
	if err := w.Append(ImportEvent{Type: "audit", Detail: json.RawMessage(`"note"`)}); err != nil {
		t.Fatal(err)
	}
	lines := nonEmptyLines(filepath.Join(bd, "events.jsonl"))
	if len(lines) != 1 {
		t.Fatalf("want 1 event row, got %d", len(lines))
	}
	var e map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatal(err)
	}
	if e["id"].(float64) != 1 || e["event_type"] != "audit" || e["detail"] != "note" {
		t.Fatalf("event row wrong: %v", e)
	}
}

// TestAppendImportFixture pins id-append semantics and the never-create rule.
func TestAppendImportFixture(t *testing.T) {
	b, bd := seedImportPrimitivesBoard(t)
	if err := b.AppendImportFixture("FIX-2"); err != nil {
		t.Fatal(err)
	}
	lines := nonEmptyLines(filepath.Join(bd, "fixtures.jsonl"))
	if len(lines) != 2 {
		t.Fatalf("fixtures.jsonl has %d rows, want 2", len(lines))
	}
	var f map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &f); err != nil {
		t.Fatal(err)
	}
	if f["id"] != "FIX-2" || f["status"] != nil {
		t.Fatalf("fixture row wrong: %v (missing keys null-filled from template)", f)
	}
	// Never create: remove the file and expect an error.
	if err := os.Remove(filepath.Join(bd, "fixtures.jsonl")); err != nil {
		t.Fatal(err)
	}
	if err := b.AppendImportFixture("FIX-3"); err == nil {
		t.Fatal("AppendImportFixture on missing fixtures.jsonl must fail")
	}
	if fileExists(filepath.Join(bd, "fixtures.jsonl")) {
		t.Fatal("fixtures.jsonl was created by import")
	}
}

// TestTemplateRowIsACopy guards the no-mutation contract between appends.
func TestTemplateRowIsACopy(t *testing.T) {
	last := &Row{Keys: []string{"id", "timestamp"}, Vals: map[string]json.RawMessage{
		"id": json.RawMessage(`1`), "timestamp": json.RawMessage(`"x"`),
	}}
	tpl := templateRow(last, "id", "timestamp")
	if err := tpl.SetGoValue("id", 9, Style{}); err != nil {
		t.Fatal(err)
	}
	if string(last.Vals["id"]) != `1` {
		t.Fatal("templateRow mutated its source row")
	}
}
