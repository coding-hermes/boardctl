package main

// DF-BOARDCTL-46: normalize-board's refusal on an unparseable tasks.jsonl
// line must be ACTIONABLE, not just diagnostic. The task's measured complaint
// was a refusal that named the line and the duplicate key but offered no
// remedy while the help advertised --skip-bad-lines — a degraded read that
// this WRITING command cannot honor without rewriting rows it never
// classified. The contract pinned here:
//
//   - a duplicate-key line refuses the command, exit 1, nothing written (no
//     tasks.jsonl change, no tasks.normalized.jsonl staging file), in BOTH
//     dry-run and --apply;
//   - the refusal names the file, the 1-based line number, the parse defect
//     AND the remedy chain in order: validate --repair -> review
//     tasks.rewritten.jsonl -> swap -> re-run normalize-board;
//   - --skip-bad-lines is gone from the verb's help (and from the root usage
//     entry), replaced by the validate --repair pointer.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df46SeedDuplicateKeyBoard bootstraps a topology-A board via cmdInit, then
// plants a tasks.jsonl whose SECOND line is a duplicate-key row (the shape
// ParseRow refuses: two "status" keys, one slot).
func df46SeedDuplicateKeyBoard(t *testing.T) (dir, boardDir string) {
	t.Helper()
	dir = t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df46"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir = filepath.Join(dir, ".coding-hermes", "board")
	tasks := `{"id": "DF46-OK", "title": "clean row", "status": "pending", "priority": "P2"}` + "\n" +
		`{"id": "DF46-DUPKEY", "title": "duplicate key row", "status": "pending", "status": "review", "priority": "P3"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, boardDir
}

// df46AssertRemedy holds the refusal text to the actionable contract: line
// number first, then every remedy step in the documented order.
func df46AssertRemedy(t *testing.T, where, text string) {
	t.Helper()
	for _, want := range []string{
		"tasks.jsonl",
		"line 2",
		`duplicate key "status"`,
		"validate --repair",
		"tasks.rewritten.jsonl",
		"swap",
		"re-run",
		"normalize-board",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("%s: refusal missing %q:\n%s", where, want, text)
		}
	}
	// Order matters: the operator must salvage BEFORE swapping, and swap
	// before re-running. A remedy printed out of order is a wrong recipe.
	steps := []string{"validate --repair", "tasks.rewritten.jsonl", "swap", "re-run"}
	prev := -1
	for _, s := range steps {
		i := strings.Index(text, s)
		if i <= prev {
			t.Errorf("%s: remedy step %q out of order (index %d, previous %d):\n%s", where, s, i, prev, text)
		}
		prev = i
	}
}

// TestDF46CLIDuplicateKeyRefusalIsActionable: dry-run refuses with the full
// remedy, exit 1, and zero writes.
func TestDF46CLIDuplicateKeyRefusalIsActionable(t *testing.T) {
	dir, boardDir := df46SeedDuplicateKeyBoard(t)
	before, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var code int
	var errOut string
	out, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "normalize-board"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("normalize-board on a duplicate-key board exit = %d, want 1 (fail-closed):\nstdout:\n%s\nstderr:\n%s", code, out, errOut)
	}
	if out != "" {
		t.Errorf("refused normalize-board must not emit a report on stdout, got:\n%s", out)
	}
	df46AssertRemedy(t, "dry-run stderr", errOut)

	after, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused dry-run rewrote tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "tasks.normalized.jsonl")); err == nil {
		t.Error("refused dry-run staged tasks.normalized.jsonl (nothing may be written when the read failed)")
	}
}

// TestDF46CLIApplyAlsoRefuses: --apply is not a way around the refusal —
// the same remedy, exit 1, and tasks.jsonl untouched.
func TestDF46CLIApplyAlsoRefuses(t *testing.T) {
	dir, boardDir := df46SeedDuplicateKeyBoard(t)
	before, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var code int
	var errOut string
	_, err = captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "normalize-board", "--apply"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("normalize-board --apply on a duplicate-key board exit = %d, want 1", code)
	}
	df46AssertRemedy(t, "--apply stderr", errOut)
	after, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused --apply rewrote tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// TestDF46CLISkipBadLinesFlagRemoved: the flag is not merely undocumented —
// it no longer parses on this verb, so a script still passing it fails loud
// instead of silently getting the refusal it was trying to bypass.
func TestDF46CLISkipBadLinesFlagRemoved(t *testing.T) {
	dir, _ := df46SeedDuplicateKeyBoard(t)
	var code int
	var errOut string
	_, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "normalize-board", "--skip-bad-lines"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		errOut = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 {
		t.Fatalf("normalize-board --skip-bad-lines exited 0; the flag must be gone (stderr:\n%s)", errOut)
	}
	if !strings.Contains(errOut, "skip-bad-lines") {
		t.Errorf("removed-flag refusal does not name the flag:\n%s", errOut)
	}
}
