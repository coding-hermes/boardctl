package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// next-id exists because `create` requires an explicit --id: on a board worked by
// several lanes at once, every lane computed the same "next" id from the same
// snapshot, one create won and the rest aborted. These tests pin the allocation
// rule (max+1, never a reused/gap-filled id) and the read-only contract.

func writeBoard(t *testing.T, rows string) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(rows), 0o644); err != nil {
		t.Fatalf("write board: %v", err)
	}
	// The loader refuses a tasks.jsonl-only board, so every fixture carries the
	// event log too — the same shape a real lane writes.
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	return dir
}

func row(id, status string) string {
	return `{"id":"` + id + `","title":"t ` + id + `","status":"` + status + `","priority":"P3"}` + "\n"
}

func TestNextIDContinuesFromMaxNotFromGaps(t *testing.T) {
	// CF-002 is missing on purpose: a gap is history, not a free slot.
	dir := writeBoard(t, row("CF-001", "complete")+row("CF-003", "pending")+row("CF-009", "complete"))
	out, err := captureStdout(func() {
		if e := cmdNextID(dir, []string{"--family", "CF"}); e != nil {
			t.Fatalf("cmdNextID: %v", e)
		}
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if got := strings.TrimSpace(out); got != "CF-010" {
		t.Fatalf("next-id = %q, want CF-010 (max+1, never a gap)", got)
	}
}

func TestNextIDPreservesObservedWidth(t *testing.T) {
	dir := writeBoard(t, row("BT-0042", "complete")+row("BT-0011", "pending"))
	out, cErr := captureStdout(func() {
		if e := cmdNextID(dir, []string{"--family", "BT"}); e != nil {
			t.Fatalf("cmdNextID: %v", e)
		}
	})
	if cErr != nil {
		t.Fatalf("capture: %v", cErr)
	}
	if got := strings.TrimSpace(out); got != "BT-0043" {
		t.Fatalf("next-id = %q, want BT-0043 (width 4 preserved)", got)
	}
}

func TestNextIDStartsNewFamilyAtOne(t *testing.T) {
	dir := writeBoard(t, row("CF-001", "complete"))
	out, cErr := captureStdout(func() {
		if e := cmdNextID(dir, []string{"--family", "ZZZ"}); e != nil {
			t.Fatalf("cmdNextID: %v", e)
		}
	})
	if cErr != nil {
		t.Fatalf("capture: %v", cErr)
	}
	if got := strings.TrimSpace(out); got != "ZZZ-001" {
		t.Fatalf("next-id = %q, want ZZZ-001 for a family with no rows", got)
	}
}

func TestNextIDIsCaseInsensitiveOnFamilyAndNeverWrites(t *testing.T) {
	rows := row("cf-007", "pending") + row("CF-008", "complete")
	dir := writeBoard(t, rows)
	path := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	out, err := captureStdout(func() {
		if e := cmdNextID(dir, []string{"--family", "CF"}); e != nil {
			t.Fatalf("cmdNextID: %v", e)
		}
	})
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if got := strings.TrimSpace(out); got != "CF-009" {
		t.Fatalf("next-id = %q, want CF-009 (mixed-case rows count)", got)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("next-id must be read-only, but tasks.jsonl changed")
	}
}

func TestNextIDJSONShapeAndRefusals(t *testing.T) {
	dir := writeBoard(t, row("CF-005", "pending"))
	out, cErr := captureStdout(func() {
		if e := cmdNextID(dir, []string{"--family", "CF", "--json"}); e != nil {
			t.Fatalf("cmdNextID: %v", e)
		}
	})
	if cErr != nil {
		t.Fatalf("capture: %v", cErr)
	}
	var got struct {
		ID     string `json:"id"`
		Family string `json:"family"`
		Used   int    `json:"used"`
		Max    int    `json:"max"`
		Width  int    `json:"width"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json: %v (%q)", err, out)
	}
	if got.ID != "CF-006" || got.Family != "CF" || got.Used != 1 || got.Max != 5 || got.Width != 3 {
		t.Fatalf("unexpected payload: %+v", got)
	}
	if err := cmdNextID(dir, []string{}); err == nil || !strings.Contains(err.Error(), "--family") {
		t.Fatalf("missing --family must be refused, got %v", err)
	}
	if err := cmdNextID(dir, []string{"--family", "CF", "extra"}); err == nil {
		t.Fatal("positional args must be refused")
	}
}
