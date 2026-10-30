package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeStatusesCLIHelpAndDryRun(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(`{"id":"A","status":"done"}`+"\n"+`{"id":"B","status":"parked"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut string
	out, err = captureStdout(func() {
		errOut, err = captureStderr(func() {
			if code := run([]string{"-C", dir, "normalize-statuses", "--dry-run", "--json"}); code != 0 {
				t.Errorf("dry-run exit = %d", code)
			}
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if errOut != "" || !strings.Contains(out, `"dry_run": true`) {
		t.Fatalf("unexpected output stdout=%q stderr=%q", out, errOut)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report["alias_count"] != float64(1) || report["unknown_count"] != float64(1) {
		t.Fatalf("report = %v", report)
	}
	after, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("CLI dry-run mutated tasks.jsonl")
	}
}

func TestNormalizeStatusesCLIApplyLeavesUnknownRefused(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(`{"id":"A","status":"open"}`+"\n"+`{"id":"B","status":"wip"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out string
	var err error
	out, err = captureStdout(func() {
		_, err = captureStderr(func() {
			if code := run([]string{"-C", dir, "normalize-statuses", "--apply"}); code != 0 {
				t.Errorf("apply exit = %d", code)
			}
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"status":"pending"`) || !strings.Contains(string(got), `"status":"wip"`) {
		t.Fatalf("apply result = %s", got)
	}
	if !strings.Contains(out, "unknown: 1") {
		t.Fatalf("apply output did not report unknown refusal: %s", out)
	}
}

func TestNormalizeStatusesHelpDocumentsScope(t *testing.T) {
	got, err := captureStdout(func() { run([]string{"normalize-statuses", "--help"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"One resolved repository-local board only", "dry-run is the default", "Unknown and missing statuses are refused", "owner-gated"} {
		if !strings.Contains(got, want) {
			t.Errorf("help missing %q: %s", want, got)
		}
	}
}
