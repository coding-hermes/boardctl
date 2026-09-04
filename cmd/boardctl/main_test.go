package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdout runs fn while capturing everything it writes to os.Stdout.
func captureStdout(fn func()) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()
	w.Close()
	return <-done, nil
}

// TestVersionDefault verifies the unstamped default: a plain `go build`
// reports "dev" because the Makefile release target is what stamps
// main.version via -ldflags.
func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version var must not be empty")
	}
	if version != "dev" {
		t.Fatalf("unstamped build should report %q, got %q", "dev", version)
	}
}

// TestCmdVersionOutput verifies cmdVersion prints the stamped version and
// exits cleanly (run() returns 0), matching the release stamping contract:
// -ldflags "-X main.version=$(VERSION)" must be observable on stdout.
func TestCmdVersionOutput(t *testing.T) {
	// Stamp through the same mechanism the Makefile uses, so the test
	// exercises the -X injection target itself.
	orig := version
	version = "20260903"
	t.Cleanup(func() { version = orig })

	got, err := captureStdout(func() {
		if code := run([]string{"version"}); code != 0 {
			t.Fatalf("run(version) exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	want := "boardctl version 20260903\n"
	if got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

// TestCmdVersionRejectsArgs verifies the subcommand rejects positional args
// like every other boardctl subcommand.
func TestCmdVersionRejectsArgs(t *testing.T) {
	if err := cmdVersion([]string{"extra"}); err == nil {
		t.Fatal("cmdVersion with positional arg should error")
	} else if !strings.Contains(err.Error(), "takes no positional args") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestCmdInitRejectsArgs verifies init follows the same no-positional-args
// convention as the other subcommands.
func TestCmdInitRejectsArgs(t *testing.T) {
	if err := cmdInit(t.TempDir(), []string{"extra"}); err == nil {
		t.Fatal("cmdInit with positional arg should error")
	} else if !strings.Contains(err.Error(), "takes no positional args") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestCmdInitThenCreateSmoke: the fresh-user path end to end at the CLI
// layer — init on an empty dir, then `boardctl create` resolving the freshly
// seeded board — both exit 0.
func TestCmdInitThenCreateSmoke(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Re-init on the initialized board exits 0 with "already initialized".
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "init"}); code != 0 {
			t.Fatalf("re-init exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "already initialized") {
		t.Fatalf("re-init output = %q, want an already-initialized note", got)
	}
	// create on the empty-but-initialized tasks.jsonl must succeed.
	got, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "create", "--id", "T-1", "--title", "First"}); code != 0 {
			t.Fatalf("create exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "created task T-1") {
		t.Fatalf("create output = %q, want a created-task note", got)
	}
}

// BT-006 exit-code contract: a normal command on a board exits 0; a command
// against a dir with NO board exits 2 (README: "2 usage/board-not-found"),
// not 1.
func TestCmdExitCodes(t *testing.T) {
	// 0: normal command against a seeded board.
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "total tasks: 0") {
		t.Fatalf("stats output = %q, want a zero-count summary", got)
	}

	// 2: board-not-found via stats.
	if code := run([]string{"-C", t.TempDir(), "stats"}); code != 2 {
		t.Fatalf("stats on boardless dir exit code = %d, want 2", code)
	}

	// 2: board-not-found via create (the wrapped openBoard error path).
	if code := run([]string{"-C", t.TempDir(), "create", "--id", "X-1", "--title", "X"}); code != 2 {
		t.Fatalf("create on boardless dir exit code = %d, want 2", code)
	}

	// 2: unknown command (unchanged).
	if code := run([]string{"-C", dir, "nope"}); code != 2 {
		t.Fatalf("unknown command exit code = %d, want 2", code)
	}
}

// BT-006 end-to-end user path: init a fresh repo, then run stats through the
// .coding-hermes dir — the exact -C form that used to fail.
func TestCmdInitThenStatsViaCodingHermesDir(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "demo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	got, err := captureStdout(func() {
		if code := run([]string{"-C", filepath.Join(dir, ".coding-hermes"), "stats"}); code != 0 {
			t.Fatalf("stats -C <repo>/.coding-hermes exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "total tasks: 0") {
		t.Fatalf("stats output = %q, want a zero-count summary", got)
	}
}
