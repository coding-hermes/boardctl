package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TR-251 AC1 CLI coverage: `boardctl update <id> --status complete` without
// --evidence-run-id exits non-zero, names the missing flag, and writes
// NOTHING (no row rewrite, no event). The flagged form exits 0 and binds the
// run. The board-level contract is pinned in internal/board
// (tr251_evidence_gate_test.go); these tests pin the CLI surface (errors go
// to stderr, exit 1, per the README exit-code contract).

const tr251CLIEvents = `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n"

func tr251CLIBoard(t *testing.T) (dir, tasksPath, eventsPath string) {
	t.Helper()
	dir = t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasksPath = filepath.Join(boardDir, "tasks.jsonl")
	eventsPath = filepath.Join(boardDir, "events.jsonl")
	tasks := `{"id": "TR-950", "title": "cli gate target", "status": "pending", "priority": "P1", "created_at": "2026-10-16 00:00:00", "updated_at": "2026-10-16 00:00:00"}` + "\n"
	if err := os.WriteFile(tasksPath, []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eventsPath, []byte(tr251CLIEvents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, tasksPath, eventsPath
}

// TestCmdUpdateCompleteRefusedWithoutEvidenceRun: the exact AC1 proof shape —
// a real completion attempt on a scratch board copy, refused with exit != 0
// and the flag named in the output.
func TestCmdUpdateCompleteRefusedWithoutEvidenceRun(t *testing.T) {
	dir, tasksPath, eventsPath := tr251CLIBoard(t)
	beforeTasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	var code int
	var got string
	got, _ = captureStderr(func() { code = run([]string{"-C", dir, "update", "TR-950", "--status", "complete"}) })
	if code == 0 {
		t.Fatalf("completion without --evidence-run-id must exit non-zero (stderr=%q)", got)
	}
	if !strings.Contains(got, "--evidence-run-id") {
		t.Fatalf("refusal must NAME the missing flag, got: %q", got)
	}
	afterTasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeTasks) != string(afterTasks) {
		t.Fatal("refused completion must not rewrite tasks.jsonl")
	}
	afterEvents, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeEvents) != string(afterEvents) {
		t.Fatal("refused completion must not append an event")
	}
}

// TestCmdUpdateCompleteAcceptedWithEvidenceRun: the flagged form exits 0,
// binds the run on the row, and appends task_completed.
func TestCmdUpdateCompleteAcceptedWithEvidenceRun(t *testing.T) {
	dir, tasksPath, eventsPath := tr251CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "TR-950", "--status", "complete", "--evidence-run-id", "run-tr950-cli"}); code != 0 {
		t.Fatalf("flagged completion must exit 0")
	}
	tasks, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tasks), `"run-tr950-cli"`) {
		t.Fatalf("evidence run must be bound on the row, got: %s", tasks)
	}
	events, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(events), "task_completed") {
		t.Fatalf("completion must append task_completed, events: %s", events)
	}
}

// TestCmdCreateCompleteRefusedWithoutEvidenceRun: create carries the gate too.
func TestCmdCreateCompleteRefusedWithoutEvidenceRun(t *testing.T) {
	dir, _, _ := tr251CLIBoard(t)
	var code int
	var got string
	got, _ = captureStderr(func() {
		code = run([]string{"-C", dir, "create", "--id", "TR-951", "--title", "born complete", "--status", "complete"})
	})
	if code == 0 {
		t.Fatalf("create --status complete without --evidence-run-id must exit non-zero (stderr=%q)", got)
	}
	if !strings.Contains(got, "--evidence-run-id") {
		t.Fatalf("refusal must NAME the missing flag, got: %q", got)
	}
}
