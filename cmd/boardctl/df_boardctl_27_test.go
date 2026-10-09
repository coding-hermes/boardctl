package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-27: validate warns on unparseable/empty created_at/completed_at ----------

// The repo's own board carried created_at values in a double-dash dialect
// ("2026-09-09-22T15:13:00Z") and rows with no created_at at all. validate
// exited OK silently while the report surface (internal/render) excluded
// those rows from time-based metrics — the two integrity surfaces
// disagreed. These tests hold the REAL CLI (run(), not a fake validator) to
// the new contract, in the TestQACorruption shape: bootstrap a real board
// via cmdInit + run() into a t.TempDir, insert the offending rows, and
// assert validate exits 0 with the warnings naming the rows; plus a clean
// board yielding zero such warnings.

// df27InsertRows appends rows to a seeded board's tasks.jsonl (the hand-edit
// path — boardctl create cannot produce these shapes, which is the point).
func df27InsertRows(t *testing.T, dir string, rows ...string) {
	t.Helper()
	path := filepath.Join(qaBoardDir(dir), "tasks.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if _, err := f.WriteString(r + "\n"); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// df27CountTimestampWarns counts validate output lines carrying the fixed
// "time-based metrics" clause of the DF-BOARDCTL-27 warning.
func df27CountTimestampWarns(out string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "time-based metrics") {
			n++
		}
	}
	return n
}

// TestDFBoardctl27ValidateWarnsOnBadTimestamps: the offending shapes each
// get one warning naming id, field and raw value; validate stays exit 0.
func TestDFBoardctl27ValidateWarnsOnBadTimestamps(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df27"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "DF27-SEED", "--title", "seed task", "--priority", "P2"}); code != 0 {
		t.Fatalf("create exit code = %d, want 0", code)
	}
	// premise: the freshly seeded board validates clean
	code, out := qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("premise: seeded board validate exit = %d, want 0\noutput:\n%s", code, out)
	}
	if n := df27CountTimestampWarns(out); n != 0 {
		t.Fatalf("premise: seeded board already carries %d timestamp warnings:\n%s", n, out)
	}
	// insert the three offending shapes + one hand-valid control
	df27InsertRows(t, dir,
		`{"id":"DF27-DD","title":"double-dash birth","status":"pending","priority":"P1","created_at":"2026-09-09-22T15:13:00Z"}`,
		`{"id":"DF27-NOBIRTH","title":"no created_at key","status":"complete","priority":"P1"}`,
		`{"id":"DF27-EMPTY","title":"empty created_at","status":"pending","priority":"P2","created_at":""}`,
	)
	code, out = qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("validate must stay exit 0 on timestamp defects (warn severity), got %d\noutput:\n%s", code, out)
	}
	if strings.Contains(out, "RESULT: FAIL") {
		t.Fatalf("timestamp defects must not fail the board:\n%s", out)
	}
	if n := df27CountTimestampWarns(out); n != 3 {
		t.Fatalf("timestamp warnings = %d, want 3\noutput:\n%s", n, out)
	}
	for _, want := range []struct{ id, frag string }{
		{"DF27-DD", `created_at unparseable "2026-09-09-22T15:13:00Z"`},
		{"DF27-NOBIRTH", "created_at absent"},
		{"DF27-EMPTY", `created_at empty string ""`},
	} {
		hit := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "task "+want.id) && strings.Contains(line, want.frag) {
				hit = true
				break
			}
		}
		if !hit {
			t.Fatalf("no warning naming %s with %s\noutput:\n%s", want.id, want.frag, out)
		}
	}
	// every timestamp warning names a repair
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "time-based metrics") {
			continue
		}
		if !strings.Contains(line, "hand-edit the row") && !strings.Contains(line, "--completed-at") {
			t.Fatalf("timestamp warning carries no fix hint: %s", line)
		}
	}
	// the valid seed row is never flagged
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "DF27-SEED") && strings.Contains(line, "time-based metrics") {
			t.Fatalf("valid row DF27-SEED flagged: %s", line)
		}
	}
}

// TestDFBoardctl27CleanBoardValidatesQuiet: a board whose rows all carry
// parseable stamps yields ZERO timestamp warnings — the fresh-init + create
// path itself must be warning-free under the new check.
func TestDFBoardctl27CleanBoardValidatesQuiet(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df27-clean"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "DF27-CLEAN-A", "--title", "clean one", "--priority", "P1"}); code != 0 {
		t.Fatalf("create exit code = %d, want 0", code)
	}
	if code := run([]string{"-C", dir, "create", "--id", "DF27-CLEAN-B", "--title", "clean two", "--priority", "P2"}); code != 0 {
		t.Fatalf("create exit code = %d, want 0", code)
	}
	if code := run([]string{"-C", dir, "update", "DF27-CLEAN-A", "--status", "complete", "--completed-at", "2026-10-09T12:00:00Z"}); code != 0 {
		t.Fatalf("update exit code = %d, want 0", code)
	}
	code, out := qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("clean board validate exit = %d, want 0\noutput:\n%s", code, out)
	}
	if n := df27CountTimestampWarns(out); n != 0 {
		t.Fatalf("clean board produced %d timestamp warnings:\n%s", n, out)
	}
}
