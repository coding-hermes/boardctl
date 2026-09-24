package main

import (
	"os"
	"path/filepath"
	"testing"
)

// REVIEW-BOARDCTL-001 CLI: the writer rejects off-vocabulary statuses, and the
// notorious fleet offender "duplicate" specifically must be refused on BOTH
// create and update — byte-compare before/after per house style, so a rejection
// that mutated the file cannot pass.
func TestCmdWriteRejectsStatusDuplicate(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--status", "duplicate"}); code != 1 {
		t.Fatalf("update --status duplicate exit code = %d, want 1", code)
	}
	mid, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(mid) {
		t.Fatalf("rejected update --status duplicate mutated tasks.jsonl:\nbefore %s\nafter  %s", before, mid)
	}
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "n", "--status", "duplicate"}); code != 1 {
		t.Fatalf("create --status duplicate exit code = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected create --status duplicate appended to tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// REVIEW-BOARDCTL-001 CLI: "parked" is the other notorious fleet offender —
// same refusal on both write surfaces, nothing written.
func TestCmdWriteRejectsStatusParked(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--status", "parked"}); code != 1 {
		t.Fatalf("update --status parked exit code = %d, want 1", code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "n", "--status", "parked"}); code != 1 {
		t.Fatalf("create --status parked exit code = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected parked writes mutated tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// REVIEW-BOARDCTL-001 CLI: the alias path stays correct — reads accept aliases
// but writes NEVER do, and that contract is proven here at the CLI level too
// (BT-025 already proved it at the library level and for update; this pins
// create as well, since create grew a --status flag).
func TestCmdCreateRejectsAliasStatus(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--status", "todo"}); code != 1 {
		t.Fatalf("update --status todo exit code = %d, want 1 (writes reject aliases)", code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "n", "--status", "todo"}); code != 1 {
		t.Fatalf("create --status todo exit code = %d, want 1 (writes reject aliases)", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected alias writes mutated tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}
