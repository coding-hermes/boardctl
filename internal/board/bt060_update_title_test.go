package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-060: `update` accepts --title. create has always carried the flag, but
// the only way to rewrite a row's title was hand-editing tasks.jsonl — which
// skips the audit event, skips updated_at, and is exactly the path BT-060's
// validate warning tells operators to stop using. These tests pin the whole
// write contract, following the bt059_create_schema_test.go style:
//
//   - --title rewrites the title in place (key order preserved, not appended)
//   - untouched rows stay byte-identical; untouched fields stay verbatim
//   - the task_updated audit event is appended (a title rewrite is a task
//     update — the trail must show it even with no other flag present)
//   - an omitted --title never touches the key (nil-untouched discipline)
//   - --title counts as a change flag (satisfies the ≥1-change gate alone)
//   - --normalize refuses --title like every other change flag
//
// seedBT060UpdateBoard mirrors seedBT037Board: spaced-style rows, so the
// value must be encoded in the file's own style.
func seedBT060UpdateBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "BT-1", "title": "first", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "BT-2", "title": "second", "status": "pending", "priority": "P2"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt060", "namespace": "bt060", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "A" {
		t.Fatalf("topology = %q, want A", b.Topology)
	}
	return b
}

// TestUpdateTitleRewritesRow: --title replaces the title, keeps the key in
// its original position, leaves every other byte of the row and every other
// row byte-identical, and appends the task_updated event.
func TestUpdateTitleRewritesRow(t *testing.T) {
	b := seedBT060UpdateBoard(t)
	otherBefore := taskLines(t, b)[1]

	newTitle := "renamed by BT-060"
	changed, err := b.UpdateTask("BT-1", UpdateSpec{Title: &newTitle})
	if err != nil {
		t.Fatalf("update --title rejected: %v", err)
	}
	if !containsStr(changed, "title") {
		t.Fatalf("changed = %v, missing %q", changed, "title")
	}

	lines := taskLines(t, b)
	// the sibling row is untouched byte-for-byte
	if lines[1] != otherBefore {
		t.Fatalf("untouched row changed:\nbefore %s\nafter  %s", otherBefore, lines[1])
	}
	row, err := ParseRow([]byte(lines[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got := row.String("title"); got != newTitle {
		t.Fatalf("title = %q, want %q", got, newTitle)
	}
	// title keeps its ORIGINAL key position (2nd key), not appended at the
	// end — the field exists on the row, SetRaw replaces in place.
	if row.Keys[1] != "title" {
		t.Fatalf("title key moved: key order %v", row.Keys)
	}
	// every other field keeps its verbatim spaced style
	if !strings.HasPrefix(lines[0], `{"id": "BT-1", "title": "renamed by BT-060", "status": "pending", "priority": "P2"`) {
		t.Fatalf("existing field bytes/order changed: %s", lines[0])
	}

	// the audit trail shows the rewrite
	var e map[string]any
	if err := json.Unmarshal(lastEventLine(t, b), &e); err != nil {
		t.Fatal(err)
	}
	if e["event_type"] != "task_updated" || e["task_id"] != "BT-1" {
		t.Fatalf("event wrong: %s", lastEventLine(t, b))
	}
}

// TestUpdateTitleAloneSatisfiesChangeFlagGate: --title is a change flag in
// its own right — a title-only update writes without the "requires at least
// one change flag" refusal.
func TestUpdateTitleAloneSatisfiesChangeFlagGate(t *testing.T) {
	b := seedBT060UpdateBoard(t)
	newTitle := "title-only change"
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Title: &newTitle}); err != nil {
		t.Fatalf("title-only update refused: %v", err)
	}
	if got := taskRowByID(t, b, "BT-1")["title"]; got != newTitle {
		t.Fatalf("title = %v, want %q", got, newTitle)
	}
}

// TestUpdateWithoutTitleLeavesKeyUntouched: nil Title means UNTOUCHED — an
// update with other flags must neither rewrite nor append the title key.
func TestUpdateWithoutTitleLeavesKeyUntouched(t *testing.T) {
	b := seedBT060UpdateBoard(t)
	lineBefore := taskLines(t, b)[0]
	complete := "complete"
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Status: &complete}); err != nil {
		t.Fatal(err)
	}
	line := taskLines(t, b)[0]
	if !strings.HasPrefix(line, `{"id": "BT-1", "title": "first",`) {
		t.Fatalf("title rewritten by a status-only update: %s", line)
	}
	if strings.Count(line, `"title"`) != strings.Count(lineBefore, `"title"`) {
		t.Fatalf("title key count changed: %s", line)
	}
}

// TestUpdateTitleFileUntouchedOnSiblingRows pins the file-level contract the
// README promises: exactly one line differs after a title rewrite.
func TestUpdateTitleFileUntouchedOnSiblingRows(t *testing.T) {
	b := seedBT060UpdateBoard(t)
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	newTitle := "second try"
	if _, err := b.UpdateTask("BT-2", UpdateSpec{Title: &newTitle}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Split(strings.TrimRight(string(before), "\n"), "\n")
	afterLines := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if i == 1 { // BT-2's line is the one allowed to differ
			continue
		}
		if beforeLines[i] != afterLines[i] {
			t.Fatalf("line %d changed:\nbefore %s\nafter  %s", i+1, beforeLines[i], afterLines[i])
		}
	}
}
