package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-9: CLI surface for --skip-bad-lines and
// validate --repair ----------
//
// The fixture is the measured dogfood corruption shape: {clean, truncated,
// embedded-raw-newline (2 fragments), clean} = 5 physical lines, 2 salvageable.

// df9SeedBrokenBoard bootstraps a topology-A board via cmdInit, then plants
// the broken 5-line tasks.jsonl. Returns the -C target and the board dir.
func df9SeedBrokenBoard(t *testing.T) (dir, boardDir string) {
	t.Helper()
	dir = t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df9"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir = filepath.Join(dir, ".coding-hermes", "board")
	tasks := `{"id": "DF9-A", "title": "clean alpha", "status": "pending", "priority": "P1"}` + "\n" +
		`{"id": "DF9-BADTRUNC", "title": "trunca` + "\n" +
		`{"id": "DF9-BADNL", "title": "first half` + "\n" +
		`of a pasted row", "status": "pending"}` + "\n" +
		`{"id": "DF9-C", "title": "clean gamma", "status": "complete", "priority": "P2"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, boardDir
}

// df9AssertCleanRowsListed holds the normal output to the salvageable rows.
func df9AssertCleanRowsListed(t *testing.T, where, out string) {
	t.Helper()
	for _, want := range []string{"DF9-A", "DF9-C"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s: clean row %s missing from output:\n%s", where, want, out)
		}
	}
	for _, bad := range []string{"DF9-BADTRUNC", "DF9-BADNL"} {
		if strings.Contains(out, bad) {
			t.Errorf("%s: unsalvageable row %s leaked into output:\n%s", where, bad, out)
		}
	}
}

// AC1a: default `list` aborts (exit 1) with the first-bad-line diagnostic and
// NO rows output — the status-quo arm of the acceptance criteria.
func TestDF9CLIDefaultListAborts(t *testing.T) {
	dir, _ := df9SeedBrokenBoard(t)
	got, err := captureStderr(func() {
		if code := run([]string{"-C", dir, "list"}); code != 1 {
			t.Errorf("default list on broken board exit = %d, want 1", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "line 2") {
		t.Fatalf("default list stderr = %q, want line-2 abort diagnostic", got)
	}
	if strings.Contains(got, "DF9-A") {
		t.Fatalf("default list must NOT emit rows: %q", got)
	}
}

// AC1b: `list --skip-bad-lines` lists the 2 clean rows, reports both skip
// classes with line numbers + reasons, and exits 1 (degraded != success).
func TestDF9CLIListSkipBad(t *testing.T) {
	dir, _ := df9SeedBrokenBoard(t)
	var out, errOut string
	var code int
	// Capture BOTH streams in the fixed order stdout-then-stderr.
	var err error
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "list", "--skip-bad-lines"})
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
		t.Fatalf("list --skip-bad-lines on broken board exit = %d, want 1 (degraded data must not read as success)", code)
	}
	df9AssertCleanRowsListed(t, "list --skip-bad-lines stdout", out)
	for _, want := range []string{
		"SKIPPED-LINES: 3",
		"tasks.jsonl line 2:",
		"tasks.jsonl line 3:",
		"tasks.jsonl line 4:",
		"DF9-BADTRUNC",
		"of a pasted row",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr missing %q:\n%s", want, errOut)
		}
	}
	// "reasons": each evidence line carries the underlying parse error text.
	if !strings.Contains(errOut, "not valid JSON") {
		t.Errorf("skip evidence lacks the underlying parse error:\n%s", errOut)
	}
}

// The flag must appear in the command's usage output (acceptance 3).
func TestDF9CLIFlagsInUsage(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"list", "boardctl list [--status S] [--priority P] [--json] [--all] [--skip-bad-lines] [-C dir]"},
		{"show", "boardctl show <id> [--events] [--skip-bad-lines] [-C dir]"},
		{"stats", "boardctl stats [--json] [--all] [--skip-bad-lines] [-C dir]"},
		{"validate", "boardctl validate [--skip-bad-lines] [--repair] [-C dir]"},
		{"render", "boardctl render [-C dir] [-o out.html] [--tz Zone] [--json out.json] [--skip-bad-lines]"},
	}
	for _, tc := range cases {
		got, err := captureStderr(func() { run([]string{tc.cmd, "-h"}) })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s -h output missing %q:\n%s", tc.cmd, tc.want, got)
		}
		if !strings.Contains(got, "skip-bad-lines") {
			t.Errorf("%s -h does not document --skip-bad-lines:\n%s", tc.cmd, got)
		}
	}
	// validate -h must also document --repair.
	got, err := captureStderr(func() { run([]string{"validate", "-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "repair") {
		t.Errorf("validate -h does not document --repair:\n%s", got)
	}
	// The top-level usage text lists the new flags too.
	for _, want := range []string{"[--skip-bad-lines]", "[--repair]"} {
		if !strings.Contains(usageText, want) {
			t.Errorf("usageText lost %q", want)
		}
	}
}

// AC2: `validate --repair` writes exactly the salvageable rows to
// tasks.rewritten.jsonl, reports counts + dropped line numbers, and leaves
// tasks.jsonl (and all board-state siblings) byte-identical.
func TestDF9CLIRepair(t *testing.T) {
	dir, boardDir := df9SeedBrokenBoard(t)
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	boardBefore, err := os.ReadFile(filepath.Join(boardDir, "board.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	var out string
	var code int
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "validate", "--repair"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if serr != "" {
			t.Errorf("validate --repair wrote to stderr: %q", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("validate --repair exit = %d, want 0 (successful rewrite-to-review-file)\nstdout:\n%s", code, out)
	}
	for _, want := range []string{
		"2 salvaged row(s)",
		"dropped 3 unparseable line(s)",
		"line(s): 2, 3, 4",
		"FOR MANUAL REVIEW",
		"NOT modified",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("repair stdout missing %q:\n%s", want, out)
		}
	}

	// tasks.jsonl byte-identical; no sibling touched; exactly one new file.
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("tasks.jsonl was modified by validate --repair")
	}
	eventsAfter, err := os.ReadFile(filepath.Join(boardDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatal("events.jsonl was modified by validate --repair")
	}
	boardAfter, err := os.ReadFile(filepath.Join(boardDir, "board.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(boardBefore, boardAfter) {
		t.Fatal("board.jsonl was modified by validate --repair")
	}
	rewrittenPath := filepath.Join(boardDir, "tasks.rewritten.jsonl")
	data, err := os.ReadFile(rewrittenPath)
	if err != nil {
		t.Fatalf("tasks.rewritten.jsonl missing: %v", err)
	}
	want := `{"id": "DF9-A", "title": "clean alpha", "status": "pending", "priority": "P1"}` + "\n" +
		`{"id": "DF9-C", "title": "clean gamma", "status": "complete", "priority": "P2"}` + "\n"
	if string(data) != want {
		t.Fatalf("tasks.rewritten.jsonl =\n%q\nwant exactly the salvageable rows:\n%q", data, want)
	}

	// The review file is itself valid JSONL the CLI can read back.
	clean := t.TempDir()
	if err := cmdInit(clean, []string{"--project", "df9review"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clean, ".coding-hermes", "board", "tasks.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	var lcode int
	lout, err := captureStdout(func() {
		serr, _ := captureStderr(func() {
			lcode = run([]string{"-C", clean, "list"})
		})
		_ = serr
	})
	if err != nil {
		t.Fatal(err)
	}
	if lcode != 0 {
		t.Fatalf("review file not listable as a board (exit %d):\n%s", lcode, lout)
	}
	if !strings.Contains(lout, "DF9-A") || !strings.Contains(lout, "DF9-C") {
		t.Fatalf("review file list missing salvageable rows:\n%s", lout)
	}
}

// On a CLEAN board the flags must be transparent no-ops (exit 0, no
// SKIPPED-LINES block, no spurious review file).
func TestDF9CLICleanBoardNoop(t *testing.T) {
	dir, boardDir := df9SeedBrokenBoard(t)
	// Heal the fixture back to a clean two-row board.
	cleanTasks := `{"id": "DF9-A", "title": "clean alpha", "status": "pending", "priority": "P1"}` + "\n" +
		`{"id": "DF9-C", "title": "clean gamma", "status": "complete", "priority": "P2"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(cleanTasks), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	out, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "list", "--skip-bad-lines"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if strings.Contains(serr, "SKIPPED-LINES") {
			t.Errorf("clean board produced a SKIPPED-LINES block:\n%s", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("clean board list --skip-bad-lines exit = %d, want 0:\n%s", code, out)
	}
	var vcode int
	_, err = captureStdout(func() {
		serr, _ := captureStderr(func() {
			vcode = run([]string{"-C", dir, "validate", "--repair"})
		})
		if strings.Contains(serr, "SKIPPED-LINES") {
			t.Errorf("clean board repair produced a SKIPPED-LINES block:\n%s", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if vcode != 0 {
		t.Fatalf("clean board validate --repair exit = %d, want 0", vcode)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "tasks.rewritten.jsonl")); err == nil {
		// Allowed but must be a faithful rewrite; verify it parses clean.
		data, rerr := os.ReadFile(filepath.Join(boardDir, "tasks.rewritten.jsonl"))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(data) != cleanTasks {
			t.Fatalf("clean-board review file = %q, want the tasks content", data)
		}
	}
}

// Degraded stats also degrade with evidence and exit 1 (flag on stats path).
func TestDF9CLIStatsSkipBad(t *testing.T) {
	dir, _ := df9SeedBrokenBoard(t)
	var code int
	out, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "stats", "--skip-bad-lines"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if !strings.Contains(serr, "SKIPPED-LINES: 3") {
			t.Errorf("stats --skip-bad-lines stderr missing SKIPPED-LINES block:\n%s", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Fatalf("stats --skip-bad-lines on broken board exit = %d, want 1", code)
	}
	if !strings.Contains(out, "total tasks: 2") {
		t.Fatalf("stats stdout = %q, want the 2 salvageable rows counted", out)
	}
}
