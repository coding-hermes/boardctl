package board

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestBoard builds a minimal topology-A board on disk.
func newTestBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("tasks.jsonl", `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}`+"\n")
	write("events.jsonl", `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}`+"\n")
	write("board.jsonl", `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}`+"\n")
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "A" {
		t.Fatalf("topology = %q, want A", b.Topology)
	}
	return b
}

func TestCreateRoundtripStylePreserved(t *testing.T) {
	b := newTestBoard(t)

	cx := int64(3)
	if _, err := b.Create(TaskRowSpec{
		ID:         "NEW-1",
		Title:      "New task",
		Status:     "pending",
		Priority:   "P2",
		Complexity: &cx,
		Reasoning:  "because testing",
	}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("tasks.jsonl has %d lines, want 2", len(lines))
	}
	// line 1 untouched byte-for-byte
	want1 := `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}`
	if string(lines[0]) != want1 {
		t.Fatalf("existing row mutated:\n got %s\nwant %s", lines[0], want1)
	}
	// appended row parses as valid JSON with the expected fields
	var row map[string]any
	if err := json.Unmarshal(lines[1], &row); err != nil {
		t.Fatal(err)
	}
	if row["id"] != "NEW-1" || row["status"] != "pending" || row["priority"] != "P2" {
		t.Fatalf("appended row wrong: %s", lines[1])
	}
	if row["complexity"] != float64(3) {
		t.Fatalf("complexity = %v, want 3", row["complexity"])
	}
}

func TestCreateDuplicateIDRejected(t *testing.T) {
	b := newTestBoard(t)
	_, err := b.Create(TaskRowSpec{ID: "EXIST-1", Title: "dup", Status: "pending", Priority: "P1"})
	if err == nil {
		t.Fatal("duplicate id accepted")
	}
	if _, ok := err.(*ErrDuplicateTaskID); !ok {
		t.Fatalf("error type = %T, want *ErrDuplicateTaskID", err)
	}
}

func TestAppendEventSequencing(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"tick 2 summary"`), Tick: i64Ptr(2)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AppendEvent(EventSpec{Type: "task_completed", TaskID: "EXIST-1", Actor: "foreman", DetailText: strPtr(`{"commit":"abc"}`), Tick: i64Ptr(2)}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("events.jsonl has %d lines, want 3", len(lines))
	}
	var e map[string]any
	if err := json.Unmarshal(lines[1], &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(2) || e["tick_number"] != float64(2) {
		t.Fatalf("second event wrong: %s", lines[1])
	}
	if err := json.Unmarshal(lines[2], &e); err != nil {
		t.Fatal(err)
	}
	if e["id"] != float64(3) || e["event_type"] != "task_completed" || e["task_id"] != "EXIST-1" {
		t.Fatalf("third event wrong: %s", lines[2])
	}
}

func TestUpdateTaskRewritesOnlyTargetRow(t *testing.T) {
	b := newTestBoard(t)
	status := "complete"
	commit := "deadbee"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status, CommitHash: &commit}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, `"status":"complete"`) || !strings.Contains(body, `"commit_hash":"deadbee"`) {
		t.Fatalf("update not applied: %s", body)
	}
	// the appended NEW row must be gone — only EXIST-1 remains, single line
	lines := bytes.Split(bytes.TrimRight(raw, "\n"), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("tasks.jsonl has %d lines, want 1", len(lines))
	}
}

func TestSetHeaderBump(t *testing.T) {
	b := newTestBoard(t)
	tt := int64(9)
	lc := "fff000"
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &tt, LastCommit: &lc}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	var hdr map[string]any
	if err := json.Unmarshal(raw, &hdr); err != nil {
		t.Fatal(err)
	}
	if hdr["ticks_total"] != float64(9) || hdr["last_commit"] != "fff000" {
		t.Fatalf("header not updated: %s", raw)
	}
	if hdr["project"] != "test" {
		t.Fatalf("unrelated header key lost: %s", raw)
	}
}

func TestValidateTopologyA(t *testing.T) {
	b := newTestBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Tasks != 1 || rep.Events != 1 {
		t.Fatalf("counts = %d/%d, want 1/1", rep.Tasks, rep.Events)
	}
	errs := 0
	for _, f := range rep.Findings {
		if f.Level == "error" {
			errs++
		}
	}
	if errs != 0 {
		t.Fatalf("unexpected errors on clean board: %+v", rep.Findings)
	}
}

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }
