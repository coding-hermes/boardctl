package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionRetryCLI(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "RETRY-1", "--title", "retryable closeout"}); code != 0 {
		t.Fatalf("create: %d", code)
	}
	args := []string{"-C", dir, "update", "RETRY-1", "--status", "complete", "--evidence-run-id", "run-retry", "--commit-hash", strings.Repeat("a", 40), "--summary", "done"}
	events := filepath.Join(dir, ".coding-hermes", "board", "events.jsonl")
	for i := 0; i < 2; i++ {
		var code int
		stderr, err := captureStderr(func() { code = run(args) })
		if err != nil || code != 0 {
			t.Fatalf("completion %d: rc=%d stderr=%s err=%v", i+1, code, stderr, err)
		}
		if i == 1 && (!strings.Contains(stderr, "duplicate status transition collapsed to a no-op") || !strings.Contains(stderr, "nothing changed")) {
			t.Fatalf("retry stderr: %q", stderr)
		}
		data, err := os.ReadFile(events)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(data), `"event_type":"task_completed"`) != 1 {
			t.Fatalf("completion %d events: %s", i+1, data)
		}
	}
}
