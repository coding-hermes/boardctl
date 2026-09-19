package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-037 CLI coverage: --worktree/--branch/--session on create and update.
// The board-level contract is pinned in internal/board (bt037_fields_test.go);
// these tests pin the CLI surface — flag parsing through reorderArgs (any
// order, repeated --session, --session=ID form), exit codes, and the
// --normalize refusal list.

// cliLastTaskRow parses the last non-empty tasks.jsonl row of a seeded CLI
// board.
func cliLastTaskRow(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var last string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			last = l
		}
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(last), &row); err != nil {
		t.Fatalf("last tasks.jsonl row does not parse: %v (%s)", err, last)
	}
	return row
}

// cliTaskLine returns the verbatim tasks.jsonl line carrying id.
func cliTaskLine(t *testing.T, dir, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("tasks.jsonl line does not parse: %v (%s)", err, l)
		}
		if row["id"] == id {
			return l
		}
	}
	t.Fatalf("task %s not found in tasks.jsonl", id)
	return ""
}

func cliSessions(t *testing.T, dir, id string) []string {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(cliTaskLine(t, dir, id)), &row); err != nil {
		t.Fatal(err)
	}
	raw, ok := row["sessions"]
	if !ok {
		t.Fatalf("row %s has no sessions key: %v", id, row)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("sessions is not a JSON array: %T (%v)", raw, raw)
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			t.Fatalf("sessions element is not a string: %v", e)
		}
		out = append(out, s)
	}
	return out
}

// TestCmdCreateWorktreeBranchSessions: create with the three flags writes
// worktree/branch and a two-element sessions array in the order given.
func TestCmdCreateWorktreeBranchSessions(t *testing.T) {
	dir := seedCLIBoard(t)
	code := run([]string{
		"-C", dir, "create", "--id", "NEW-1", "--title", "wt task",
		"--worktree", "/tmp/wt-bt-schema", "--branch", "bt/session-worktree-fields",
		"--session", "sess-a", "--session", "sess-b",
	})
	if code != 0 {
		t.Fatalf("create with worktree/branch/session exit = %d, want 0", code)
	}
	row := cliLastTaskRow(t, dir)
	if row["worktree"] != "/tmp/wt-bt-schema" {
		t.Fatalf("worktree = %v", row["worktree"])
	}
	if row["branch"] != "bt/session-worktree-fields" {
		t.Fatalf("branch = %v", row["branch"])
	}
	got := cliSessions(t, dir, "NEW-1")
	if len(got) != 2 || got[0] != "sess-a" || got[1] != "sess-b" {
		t.Fatalf("sessions = %v, want [sess-a sess-b] in order", got)
	}
	// A single --session is a ONE-ELEMENT array, not a bare string.
	if code := run([]string{"-C", dir, "create", "--id", "NEW-2", "--title", "single",
		"--session", "solo"}); code != 0 {
		t.Fatalf("create single session exit = %d, want 0", code)
	}
	if !strings.Contains(cliTaskLine(t, dir, "NEW-2"), `"sessions":["solo"]`) &&
		!strings.Contains(cliTaskLine(t, dir, "NEW-2"), `"sessions": ["solo"]`) {
		t.Fatalf("single --session must be a one-element array: %s", cliTaskLine(t, dir, "NEW-2"))
	}
	if got := cliSessions(t, dir, "NEW-2"); len(got) != 1 || got[0] != "solo" {
		t.Fatalf("sessions = %v, want [solo]", got)
	}
}

// TestCmdCreateWithoutWorktreeFlagsAddsNoKeys: the CLI never writes an empty
// worktree/branch/sessions value.
func TestCmdCreateWithoutWorktreeFlagsAddsNoKeys(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "plain"}); code != 0 {
		t.Fatalf("create exit = %d, want 0", code)
	}
	for _, k := range []string{"worktree", "branch", "sessions"} {
		if strings.Contains(cliTaskLine(t, dir, "NEW-1"), `"`+k+`"`) {
			t.Fatalf("create without %q flag wrote the key: %s", k, cliTaskLine(t, dir, "NEW-1"))
		}
	}
}

// TestCmdUpdateWorktreeBranchSessionsAnyOrder: update parses the new flags
// after the positional id, interleaved with other flags, and supports both
// `--session X` and `--session=X`.
func TestCmdUpdateWorktreeBranchSessionsAnyOrder(t *testing.T) {
	dir := seedCLIBoard(t)
	code := run([]string{
		"-C", dir, "update", "EXIST-1",
		"--session", "s1", "--status", "complete",
		"--worktree", "/tmp/wt-1", "--branch", "wt/one", "--session=s2",
	})
	if code != 0 {
		t.Fatalf("update with the new flags exit = %d, want 0", code)
	}
	row := cliLastTaskRow(t, dir)
	if row["status"] != "complete" {
		t.Fatalf("status = %v, want complete", row["status"])
	}
	if row["worktree"] != "/tmp/wt-1" || row["branch"] != "wt/one" {
		t.Fatalf("worktree/branch not stored: %s", cliTaskLine(t, dir, "EXIST-1"))
	}
	if got := cliSessions(t, dir, "EXIST-1"); len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
		t.Fatalf("sessions = %v, want [s1 s2] in order", got)
	}
}

// TestCmdUpdateWithoutWorktreeFlagsCreatesNoKeys: an update that passes only
// --status leaves the three keys absent.
func TestCmdUpdateWithoutWorktreeFlagsCreatesNoKeys(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--status", "in_progress"}); code != 0 {
		t.Fatalf("update exit = %d, want 0", code)
	}
	line := cliTaskLine(t, dir, "EXIST-1")
	for _, k := range []string{"worktree", "branch", "sessions"} {
		if strings.Contains(line, `"`+k+`"`) {
			t.Fatalf("update without %q flag created the key: %s", k, line)
		}
	}
}

// TestCmdNormalizeRefusesWorktreeFlags: --normalize refuses to combine with
// each of the three new flags (usage error, nothing written).
func TestCmdNormalizeRefusesWorktreeFlags(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, combo := range [][]string{
		{"--worktree", "/tmp/x"},
		{"--branch", "wt/x"},
		{"--session", "s1"},
	} {
		args := append([]string{"-C", dir, "update", "EXIST-1", "--normalize"}, combo...)
		if code := run(args); code != 1 {
			t.Fatalf("update --normalize %s exit = %d, want 1", combo[0], code)
		}
		after, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("refused --normalize %s mutated tasks.jsonl:\nbefore %s\nafter  %s", combo[0], before, after)
		}
	}
}
