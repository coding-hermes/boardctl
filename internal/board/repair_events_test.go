package board

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestRepairEvents_descendingIds verifies that --repair-events renumbers
// descending event ids to ascending order (DF-BOARDCTL-38 AC1-AC2).
func TestRepairEvents_descendingIds(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	// IDs: 1, 2, 5, 3, 4, 10, 7, 8, 9
	// Running max:
	//   Line 1 (id=1): runningMax=1
	//   Line 2 (id=2): runningMax=2
	//   Line 3 (id=5): runningMax=5
	//   Line 4 (id=3 < 5): fix to 6, runningMax=6
	//   Line 5 (id=4 < 6): fix to 7, runningMax=7
	//   Line 6 (id=10): runningMax=10
	//   Line 7 (id=7 < 10): fix to 11, runningMax=11
	//   Line 8 (id=8 < 11): fix to 12, runningMax=12
	//   Line 9 (id=9 < 12): fix to 13, runningMax=13
	// 5 descending, 5 fixed. Expected after: 1,2,5,6,7,10,11,12,13
	input := `{ "id": 1, "action": "create" }
{ "id": 2, "action": "update" }
{ "id": 5, "action": "assign" }
{ "id": 3, "action": "resolve" }
{ "id": 4, "action": "comment" }
{ "id": 10, "action": "close" }
{ "id": 7, "action": "reopen" }
{ "id": 8, "action": "reassign" }
{ "id": 9, "action": "relabel" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	// Dry-run first.
	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents(dry=true): %v", err)
	}
	if rep.Clean {
		t.Fatal("expected descending ids to be found, but board was clean")
	}
	if rep.Descending != 5 {
		t.Errorf("expected 5 descending ids, got %d", rep.Descending)
	}
	if rep.FixedCount != 5 {
		t.Errorf("expected 5 fixed events, got %d", rep.FixedCount)
	}
	if rep.TotalEvents != 9 {
		t.Errorf("expected 9 total events, got %d", rep.TotalEvents)
	}
	if rep.Applied {
		t.Error("dry-run should not mark applied")
	}

	// Apply.
	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents(apply): %v", err)
	}
	if !rep2.Applied {
		t.Error("apply should mark applied")
	}
	if rep2.FixedCount != 5 {
		t.Errorf("expected 5 fixed events on apply, got %d", rep2.FixedCount)
	}

	// Verify the file was renumbered correctly.
	expectedIDs := []int64{1, 2, 5, 6, 7, 10, 11, 12, 13}
	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events after apply: %v", err)
	}
	if len(lines) != 9 {
		t.Fatalf("expected 9 lines after apply, got %d", len(lines))
	}

	for i, expID := range expectedIDs {
		var m map[string]any
		if err := json.Unmarshal(lines[i], &m); err != nil {
			t.Fatalf("line %d parse: %v", i+1, err)
		}
		id, ok := m["id"].(float64)
		if !ok {
			t.Errorf("line %d: expected numeric id, got %T", i+1, m["id"])
			continue
		}
		if int64(id) != expID {
			t.Errorf("line %d: expected id %d, got %d", i+1, expID, int64(id))
		}
	}

	// After repair, running again should be clean.
	rep3, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents after repair: %v", err)
	}
	if !rep3.Clean {
		t.Errorf("expected clean after repair, got %d descending", rep3.Descending)
	}
}

// TestRepairEvents_alreadyCanonical verifies no-op when ids are ascending.
func TestRepairEvents_alreadyCanonical(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "create" }
{ "id": 2, "action": "update" }
{ "id": 3, "action": "assign" }
{ "id": 4, "action": "resolve" }
{ "id": 5, "action": "close" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if !rep.Clean {
		t.Errorf("expected clean board, got %d descending", rep.Descending)
	}
	if rep.TotalEvents != 5 {
		t.Errorf("expected 5 events, got %d", rep.TotalEvents)
	}
}

// TestRepairEvents_legacyShape verifies events without numeric ids are skipped.
func TestRepairEvents_legacyShape(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	// 7 lines: id=1,5,3,10 are numeric; "legacy-ref" is string; line 5 has no id
	// Descending: line 4 (id=3 < 5), line 7 (id=10 > 3 but then... wait)
	// Running max: 1→1, 5→5, (skip string), 3<5 fix to 6→6, (skip no-id), 10→10, 8<10 fix to 11→11
	// That's 2 descending.
	input := `{ "id": 1, "action": "create" }
{ "id": 5, "action": "assign" }
{ "id": "legacy-ref", "action": "legacy_event" }
{ "id": 3, "action": "resolve" }
{ "action": "no_id_event" }
{ "id": 10, "action": "close" }
{ "id": 8, "action": "reopen" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "B",
		eventsPath: eventsPath,
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if rep.Clean {
		t.Fatal("expected descending ids")
	}
	if rep.NonNumeric != 2 {
		t.Errorf("expected 2 non-numeric events (string id + missing id), got %d", rep.NonNumeric)
	}
	if rep.Descending != 2 {
		t.Errorf("expected 2 descending ids (id=3 < 5, id=8 < 10), got %d", rep.Descending)
	}

	// Apply and verify.
	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	// Verify specific lines.
	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	testLines := []struct {
		line      int
		expID     any
		expectKey string
	}{
		{1, int64(1), "id"},
		{2, int64(5), "id"},
		{3, "legacy-ref", "id"},
		{4, int64(6), "id"}, // 3->6 (descends below 5)
		{5, nil, ""},        // no id
		{6, int64(10), "id"},
		{7, int64(11), "id"}, // 8->11 (descends below 10)
	}
	for _, tc := range testLines {
		var m map[string]any
		if err := json.Unmarshal(lines[tc.line-1], &m); err != nil {
			t.Fatalf("line %d parse: %v", tc.line, err)
		}
		if tc.expectKey == "" {
			if _, has := m["id"]; has {
				t.Errorf("line %d: expected no id, got: %v", tc.line, m)
			}
			continue
		}
		switch v := tc.expID.(type) {
		case int64:
			if m["id"] != float64(v) {
				t.Errorf("line %d: expected id=%d, got %v", tc.line, v, m["id"])
			}
		case string:
			if m["id"] != v {
				t.Errorf("line %d: expected id=%q, got %v", tc.line, v, m["id"])
			}
		}
	}
}
