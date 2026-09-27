package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-68: `update --note` must not silently destroy the row's existing
// foreman_note. The old write path REPLACED the value, so a second --note
// dropped the first with exit 0 and no warning. New contract:
//
//   - --note appends to a non-empty existing note: existing + "\n\n" + new
//   - an absent, null or empty existing note takes the new value verbatim
//     (first write, no leading separator)
//   - --replace (UpdateSpec.NoteReplace) restores the old overwrite
//   - an omitted --note never touches the key (nil-untouched discipline)
//
// seedBT068Board mirrors seedBT060UpdateBoard (spaced-style rows, topology A)
// with BT-1 carrying foreman_note in the exact shape each case needs.
func seedBT068Board(t *testing.T, bt1Extra string) *Board {
	t.Helper()
	dir := t.TempDir()
	bt1 := `{"id": "BT-1", "title": "first", "status": "pending", "priority": "P2"` + bt1Extra + `}`
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": bt1 + "\n" +
			`{"id": "BT-2", "title": "second", "status": "pending", "priority": "P2"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt068", "namespace": "bt068", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bt068NoteOf parses the tasks.jsonl row with the given id and returns its
// foreman_note plus whether the key is present at all.
func bt068NoteOf(t *testing.T, b *Board, id string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if !strings.Contains(l, `"id": "`+id+`"`) {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("tasks.jsonl line does not parse: %v (%s)", err, l)
		}
		v, ok := row["foreman_note"]
		if !ok {
			return "", false
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("foreman_note is not a JSON string: %T (%v)", v, v)
		}
		return s, true
	}
	t.Fatalf("task %s not found in tasks.jsonl", id)
	return "", false
}

// TestUpdateNoteAppendsToExisting: the BT-68 acceptance case 1 — a row with
// foreman_note "A" + update --note B ends with "A\n\nB", never "B".
// RED on the pre-fix tree: the old REPLACE wrote "B" and returned ok.
func TestUpdateNoteAppendsToExisting(t *testing.T) {
	b := seedBT068Board(t, `, "foreman_note": "A"`)

	changed, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("B")})
	if err != nil {
		t.Fatalf("update --note rejected: %v", err)
	}
	if !containsStr(changed, "foreman_note") {
		t.Fatalf("changed = %v, missing %q", changed, "foreman_note")
	}
	got, ok := bt068NoteOf(t, b, "BT-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from BT-1")
	}
	if got != "A\n\nB" {
		t.Fatalf("foreman_note = %q, want %q (old REPLACE dropped the existing note)", got, "A\n\nB")
	}
}

// TestUpdateNoteAppendsTwice: case 2 — B then C accumulates "A\n\nB\n\nC".
func TestUpdateNoteAppendsTwice(t *testing.T) {
	b := seedBT068Board(t, `, "foreman_note": "A"`)

	if _, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("B")}); err != nil {
		t.Fatalf("first update rejected: %v", err)
	}
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("C")}); err != nil {
		t.Fatalf("second update rejected: %v", err)
	}
	got, ok := bt068NoteOf(t, b, "BT-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from BT-1")
	}
	if got != "A\n\nB\n\nC" {
		t.Fatalf("foreman_note = %q, want %q", got, "A\n\nB\n\nC")
	}
}

// TestUpdateNoteFirstWriteNoSeparator: case 3 — a null foreman_note takes
// the new value verbatim (no leading "\n\n").
func TestUpdateNoteFirstWriteNoSeparator(t *testing.T) {
	b := seedBT068Board(t, `, "foreman_note": null`)

	if _, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("B")}); err != nil {
		t.Fatalf("update --note rejected: %v", err)
	}
	got, ok := bt068NoteOf(t, b, "BT-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from BT-1")
	}
	if got != "B" {
		t.Fatalf("foreman_note = %q, want %q (no leading separator on first write)", got, "B")
	}
}

// TestUpdateNoteReplaceOverwrites: case 4 — --replace (NoteReplace) writes
// exactly the new value, the pre-BT-68 behaviour.
func TestUpdateNoteReplaceOverwrites(t *testing.T) {
	b := seedBT068Board(t, `, "foreman_note": "A"`)

	if _, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("Z"), NoteReplace: true}); err != nil {
		t.Fatalf("update --note --replace rejected: %v", err)
	}
	got, ok := bt068NoteOf(t, b, "BT-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from BT-1")
	}
	if got != "Z" {
		t.Fatalf("foreman_note = %q, want %q (--replace must overwrite)", got, "Z")
	}
}

// TestUpdateNoteOmittedLeavesKeyUntouched: case 5 — an omitted --note never
// touches foreman_note (no key created on a row that lacks it, no value
// change on a row that carries it).
func TestUpdateNoteOmittedLeavesKeyUntouched(t *testing.T) {
	// row WITHOUT the key: --title alone must not create foreman_note
	b := seedBT068Board(t, ``)
	if _, err := b.UpdateTask("BT-1", UpdateSpec{Title: strPtr("retitled")}); err != nil {
		t.Fatalf("update rejected: %v", err)
	}
	if _, ok := bt068NoteOf(t, b, "BT-1"); ok {
		t.Fatalf("foreman_note key created by an update that did not set --note")
	}

	// row WITH the key: --status alone must leave the value verbatim
	b2 := seedBT068Board(t, `, "foreman_note": "A"`)
	if _, err := b2.UpdateTask("BT-1", UpdateSpec{Status: strPtr("in_progress")}); err != nil {
		t.Fatalf("update rejected: %v", err)
	}
	got, ok := bt068NoteOf(t, b2, "BT-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from BT-1")
	}
	if got != "A" {
		t.Fatalf("foreman_note = %q, want %q (omitted --note must leave the value alone)", got, "A")
	}
}

// TestUpdateNoteAppendPreservesEveryOtherLine: case 7 — the house rule. After
// an append, every tasks.jsonl line except the target row is byte-identical,
// and within the target row the sibling fields keep their verbatim bytes.
func TestUpdateNoteAppendPreservesEveryOtherLine(t *testing.T) {
	b := seedBT068Board(t, `, "foreman_note": "A"`)
	beforeLines, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := b.UpdateTask("BT-1", UpdateSpec{Note: strPtr("B")}); err != nil {
		t.Fatalf("update --note rejected: %v", err)
	}

	afterRaw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	before := strings.Split(strings.TrimRight(string(beforeLines), "\n"), "\n")
	after := strings.Split(strings.TrimRight(string(afterRaw), "\n"), "\n")
	if len(before) != len(after) {
		t.Fatalf("line count changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if strings.Contains(before[i], `"id": "BT-1"`) {
			continue // the target row is the one allowed to differ
		}
		if before[i] != after[i] {
			t.Fatalf("untouched line %d changed:\nbefore %s\nafter  %s", i+1, before[i], after[i])
		}
	}
	// target row: the fields before foreman_note keep their spaced bytes
	if !strings.HasPrefix(after[0], `{"id": "BT-1", "title": "first", "status": "pending", "priority": "P2", "foreman_note": "A\n\nB"`) {
		t.Fatalf("target row drifted outside the note value: %s", after[0])
	}
}
