package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- BT-026: CLI error UX on partial and pretty-printed boards ----------

// TestCmdPartialBoardExit2AndMessage: a tasks.jsonl-only board (no
// events.jsonl) must keep the documented exit 2 (board-not-found class) at
// all three supported -C depths, while the message identifies the detected
// tasks.jsonl and explicitly names the missing events.jsonl.
func TestCmdPartialBoardExit2AndMessage(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tgt := range []string{repo, filepath.Join(repo, ".coding-hermes"), boardDir} {
		got, err := captureStderr(func() {
			if code := run([]string{"-C", tgt, "stats"}); code != 2 {
				t.Fatalf("stats on tasks-only board (-C %s) exit code = %d, want 2", tgt, code)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"no JSONL foreman board found",
			filepath.Join(boardDir, "tasks.jsonl"),
			"events.jsonl is missing",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("stderr (-C %s) = %q, want substring %q", tgt, got, want)
			}
		}
	}
}

// TestCmdPrettyPrintedTasksExit1AndMessage: a board with a valid-JSON but
// pretty-printed (multi-line) tasks.jsonl keeps the runtime parse-error exit
// 1, and the message carries the exact file + 1-based line plus the
// valid-JSON-but-not-JSONL diagnosis with the actionable hint.
func TestCmdPrettyPrintedTasksExit1AndMessage(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	pretty := "{\n  \"id\": \"E-1\",\n  \"title\": \"Existing\",\n  \"status\": \"pending\"\n}\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(pretty), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := captureStderr(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 1 {
			t.Fatalf("stats on pretty-printed board exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		filepath.Join(boardDir, "tasks.jsonl"), // exact file
		"line 1",                               // exact 1-based failing line
		"valid JSON",                           // the source may be valid JSON...
		"not line-loadable JSONL",              // ...but is not JSONL-line-loadable
		"one complete JSON object per line",    // the JSONL contract
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr = %q, want substring %q", got, want)
		}
	}
}

// The pretty-print diagnosis must not mislabel genuinely corrupt JSON:
// corrupt input keeps the plain parse error, with file + line context.
func TestCmdCorruptTasksKeepsPlainError(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte("{\"id\": \"E-1\",\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := captureStderr(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 1 {
			t.Fatalf("stats on corrupt tasks.jsonl exit code = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "line 1") {
		t.Fatalf("stderr = %q, want line context", got)
	}
	if strings.Contains(got, "valid JSON") {
		t.Fatalf("corrupt JSON mislabeled as valid: %q", got)
	}
}

// Topology-B behavior must be preserved end to end on the same error paths:
// a well-formed topology-B board still resolves and `stats` exits 0.
func TestCmdTopologyBStillResolves(t *testing.T) {
	dir := seedCLITopologyBBoard(t)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats on topology-B board exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "total tasks: 1") {
		t.Fatalf("stats output = %q, want total tasks: 1", got)
	}
}
