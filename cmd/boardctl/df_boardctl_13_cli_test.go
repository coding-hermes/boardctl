package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DF-BOARDCTL-13 CLI coverage: --priority and --evidence-run-id on update.
// The board-level write contract is pinned in internal/board
// (df_boardctl_13_test.go); these tests pin the CLI surface — flag parsing
// through reorderArgs (any order, --flag=value form), the write-time
// vocabulary rejection (exit 1, nothing written), the usage closure listing
// every registered value flag, and the --normalize refusal list.

// TestCmdUpdatePriorityRewritesRow: `update <id> --priority P1` rewrites the
// row's priority field (dogfood run 15 measured this exact invocation
// exiting "flag provided but not defined: -priority"); untouched rows stay
// byte-identical; no event is appended.
func TestCmdUpdatePriorityRewritesRow(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--priority", "P1"}); code != 0 {
		t.Fatalf("update --priority exit = %d, want 0", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Split(strings.TrimRight(string(before), "\n"), "\n")
	afterLines := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if strings.Contains(beforeLines[i], `"id":"EXIST-1"`) {
			continue // the target row is the one allowed to differ
		}
		if beforeLines[i] != afterLines[i] {
			t.Fatalf("line %d changed:\nbefore %s\nafter  %s", i+1, beforeLines[i], afterLines[i])
		}
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(afterLines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row["priority"] != "P1" {
		t.Fatalf("priority = %v, want %q", row["priority"], "P1")
	}
	// flag-only priority update: no audit event
	eventsAfter, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(eventsBefore) != string(eventsAfter) {
		t.Fatalf("priority-only update appended an event:\nbefore %s\nafter  %s", eventsBefore, eventsAfter)
	}
}

// TestCmdUpdatePriorityEqualsFormAndOrder: --priority=PX-style value forms
// and flag-after-positional ordering both parse (reorderArgs handles any
// order; = form is consumed by the flagset itself).
func TestCmdUpdatePriorityEqualsFormAndOrder(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "update", "--priority=P0", "EXIST-1"}); code != 0 {
		t.Fatalf("update --priority=P0 <id> exit = %d, want 0", code)
	}
	row, err := cliTaskRow(t, filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"), "EXIST-1")
	if err != nil {
		t.Fatal(err)
	}
	if row["priority"] != "P0" {
		t.Fatalf("priority = %v, want P0", row["priority"])
	}
}

// TestCmdUpdateRejectsJunkPriority: `update <id> --priority PX` exits 1 with
// create's vocabulary-gate error and writes nothing.
func TestCmdUpdateRejectsJunkPriority(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	code := run([]string{"-C", dir, "update", "EXIST-1", "--priority", "PX"})
	if code != 1 {
		t.Fatalf("update --priority PX exit = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected --priority PX mutated tasks.jsonl")
	}
}

// TestCmdUpdateEvidenceRunIDWritesDetail: `update <id> --evidence-run-id X`
// records the run evidence in the row's detail field (SG-126).
func TestCmdUpdateEvidenceRunIDWritesDetail(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--evidence-run-id", "DF-RUN-15"}); code != 0 {
		t.Fatalf("update --evidence-run-id exit = %d, want 0", code)
	}
	line := cliTaskLine(t, dir, "EXIST-1")
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatal(err)
	}
	detail, ok := row["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail missing or not an object: %v (%s)", row["detail"], line)
	}
	evidence, ok := detail["evidence"].([]any)
	if !ok || len(evidence) != 1 {
		t.Fatalf("detail.evidence missing or not one entry: %v (%s)", detail["evidence"], line)
	}
	entry, ok := evidence[0].(map[string]any)
	if !ok || entry["run_id"] != "DF-RUN-15" || entry["ts"] == "" {
		t.Fatalf("evidence entry wrong: %v (%s)", evidence[0], line)
	}
}

// TestCmdUpdateHelpListsPriorityAndEvidenceFlags: the acceptance criterion —
// update -h documents the two new value flags (the closure stayed complete
// through the BT-060 fix; DF-BOARDCTL-13 must not regress it).
func TestCmdUpdateHelpListsPriorityAndEvidenceFlags(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"update", "-h"}); code != 0 {
			t.Fatalf("update -h exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--priority", "--evidence-run-id"} {
		if !strings.Contains(got, flag) {
			t.Errorf("update -h usage output missing %q; got:\n%q", flag, got)
		}
	}
}

// TestCmdUpdateNormalizeRefusesPriorityAndEvidence: both new flags are change
// flags, so --normalize refuses the combination (same contract as --title,
// TestCmdNormalizeRefusesTitle).
func TestCmdUpdateNormalizeRefusesPriorityAndEvidence(t *testing.T) {
	for _, flag := range []string{"priority", "evidence-run-id"} {
		t.Run(flag, func(t *testing.T) {
			dir := seedCLIBoard(t)
			tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
			before, err := os.ReadFile(tasksPath)
			if err != nil {
				t.Fatal(err)
			}
			if code := run([]string{"-C", dir, "update", "EXIST-1", "--normalize", "--" + flag, "x"}); code != 1 {
				t.Fatalf("update --normalize --%s exit = %d, want 1", flag, code)
			}
			after, err := os.ReadFile(tasksPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatalf("refused --normalize --%s mutated tasks.jsonl", flag)
			}
		})
	}
}

// cliTaskRow parses the tasks.jsonl row carrying id out of a raw file.
func cliTaskRow(t *testing.T, tasksPath, id string) (map[string]any, error) {
	t.Helper()
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			continue
		}
		if row["id"] == id {
			return row, nil
		}
	}
	t.Fatalf("task %s not found in %s", id, tasksPath)
	return nil, nil
}
