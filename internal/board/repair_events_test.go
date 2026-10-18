package board

import (
	"bytes"
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
		t.Errorf("expected 2 non-numeric events, got %d", rep.NonNumeric)
	}
	if rep.Descending != 2 {
		t.Errorf("expected 2 descending ids, got %d", rep.Descending)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("RepairEvents apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

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
		{4, int64(6), "id"},
		{5, nil, ""},
		{6, int64(10), "id"},
		{7, int64(11), "id"},
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

// TestRepairEvents_topologyA verifies event repair on a topology-A board
// (board.jsonl header present).
func TestRepairEvents_topologyA(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	// IDs: 1, 5, 3, 10, 7 — descending at line 3 (3<5) and line 5 (7<10)
	// Running max: 1→1, 5→5, 3→6, 10→10, 7→11
	input := `{ "id": 1, "action": "start" }
{ "id": 5, "action": "update" }
{ "id": 3, "action": "descend" }
{ "id": 10, "action": "max" }
{ "id": 7, "action": "descend2" }`
	if err := os.WriteFile(eventsPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}

	b := &Board{
		Dir:        dir,
		Topology:   "A",
		eventsPath: eventsPath,
	}

	if b.Topology != "A" {
		t.Fatalf("expected topology A, got %s", b.Topology)
	}

	rep, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("RepairEvents: %v", err)
	}
	if rep.Descending != 2 {
		t.Errorf("expected 2 descending ids, got %d", rep.Descending)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	lines, err := ReadJSONLLines(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	expected := []int64{1, 5, 6, 10, 11}
	for i, exp := range expected {
		var m map[string]any
		if err := json.Unmarshal(lines[i], &m); err != nil {
			t.Fatalf("line %d: %v", i+1, err)
		}
		got, ok := m["id"].(float64)
		if !ok {
			t.Errorf("line %d: expected numeric id, got %T", i+1, m["id"])
			continue
		}
		if int64(got) != exp {
			t.Errorf("line %d: expected id=%d, got %d", i+1, exp, int64(got))
		}
	}

	rep3, err := b.RepairEvents(true)
	if err != nil {
		t.Fatalf("post-repair: %v", err)
	}
	if !rep3.Clean {
		t.Errorf("expected clean after topology-A repair, got %d descending", rep3.Descending)
	}
}

// TestRepairEvents_dryRunCache verifies dry-run cache matches applied output.
func TestRepairEvents_dryRunCache(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	input := `{ "id": 1, "action": "a" }
{ "id": 5, "action": "b" }
{ "id": 3, "action": "c" }`
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
		t.Fatalf("dry-run: %v", err)
	}
	if rep.AppliedPath == "" {
		t.Fatal("dry-run should write cache file")
	}

	cacheData, err := os.ReadFile(rep.AppliedPath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	rep2, err := b.RepairEvents(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !rep2.Applied {
		t.Error("expected applied")
	}

	afterData, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	if !bytes.Equal(cacheData, afterData) {
		t.Errorf("dry-run cache != applied output:\ncache: %s\napplied: %s", cacheData, afterData)
	}
}
