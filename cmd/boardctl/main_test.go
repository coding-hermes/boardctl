package main

import (
	"io"
	"os"
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
