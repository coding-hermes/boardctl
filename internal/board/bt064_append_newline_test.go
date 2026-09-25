package board

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A board whose last line has no trailing newline must not have the next row glued onto
// it. Before the fix, appendBytes wrote straight through O_APPEND, so two rows landed on
// one line: every reader saw a single row that fails to parse ("Extra data") and BOTH ids
// vanished. Reproduced on a live board (heading INT-CI-2693).
func TestAppendDoesNotGlueOntoUnterminatedLine(t *testing.T) {
	dir := t.TempDir()
	tasks := filepath.Join(dir, "tasks.jsonl")
	first := `{"id":"BT-001","title":"first"}`
	if err := os.WriteFile(tasks, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendBytes(tasks, []byte(`{"id":"BT-002","title":"second"}`+"\n")); err != nil {
		t.Fatalf("appendBytes: %v", err)
	}
	content, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d — the rows were glued: %q", len(lines), content)
	}
	if lines[0] != first {
		t.Errorf("the pre-existing row changed:\n got %q\nwant %q", lines[0], first)
	}
	for i, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Errorf("line %d does not parse: %v (%q)", i+1, err, line)
		}
	}
}

// Appending to an absent file must not prepend a blank line, and appending to a
// properly terminated file must keep the byte-for-byte shape it had.
func TestAppendNewlineEdges(t *testing.T) {
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.jsonl")
	if err := appendBytes(fresh, []byte(`{"id":"BT-003"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(fresh)
	if string(got) != `{"id":"BT-003"}`+"\n" {
		t.Errorf("fresh file shape: got %q", got)
	}

	terminated := filepath.Join(dir, "terminated.jsonl")
	if err := os.WriteFile(terminated, []byte(`{"id":"BT-004"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := appendBytes(terminated, []byte(`{"id":"BT-005"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(terminated)
	want := `{"id":"BT-004"}` + "\n" + `{"id":"BT-005"}` + "\n"
	if string(got) != want {
		t.Errorf("terminated file shape:\n got %q\nwant %q", got, want)
	}
}
