package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const bt082TreeFP = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestCmdTreeEvidenceRoundTripAndInvalidNoWrite(t *testing.T) {
	dir := seedCLIBoard(t)
	tasks := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	if code := run([]string{"-C", dir, "create", "--id", "QA-BT82-1", "--title", "tree evidence", "--reasoning", "same finding", "--evidence-run-id", "run-bt82", "--evidence-fingerprint", bt082TreeFP, "--evidence-artifact", "reports/qa.txt"}); code != 0 {
		t.Fatalf("create tree evidence exit = %d", code)
	}
	line := cliTaskLine(t, dir, "QA-BT82-1")
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatal(err)
	}
	detail := row["detail"].(map[string]any)
	entry := detail["evidence"].([]any)[0].(map[string]any)
	if entry["tree_fp"] != bt082TreeFP || entry["run_id"] != "run-bt82" {
		t.Fatalf("evidence identity = %#v", entry)
	}
	if got := entry["artifacts"].([]any)[0]; got != "reports/qa.txt" {
		t.Fatalf("artifact = %v", got)
	}

	before, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "QA-BT82-2", "--title", "bad evidence", "--reasoning", "new", "--evidence-run-id", "run-bad", "--evidence-fingerprint", "bad"}); code != 1 {
		t.Fatalf("malformed fingerprint exit = %d", code)
	}
	after, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("malformed fingerprint mutated tasks.jsonl")
	}
}

func TestCmdTreeEvidenceKeepsFindingDedupeSeparate(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "QA-BT82-3", "--title", "same finding", "--reasoning", "same reasoning", "--evidence-run-id", "run-a", "--evidence-fingerprint", bt082TreeFP}); code != 0 {
		t.Fatal(code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "QA-BT82-4", "--title", "same finding", "--reasoning", "same reasoning", "--evidence-run-id", "run-b", "--evidence-fingerprint", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}); code != 2 {
		t.Fatalf("dedupe exit = %d, want 2", code)
	}
}
