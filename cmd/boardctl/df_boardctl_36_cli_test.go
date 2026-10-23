package main

// DF-BOARDCTL-36 CLI tests: the normalize-board verb's usage surface,
// dry-run/apply behavior end-to-end through run(), and exit codes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df36SeedCLIBoard bootstraps a topology-A board via cmdInit, then plants a
// drifted SPACED-style tasks.jsonl (the warpfs board's serialization style:
// python json.dumps defaults, ", " separators) exercising all three repair
// families plus one refused row.
func df36SeedCLIBoard(t *testing.T) (dir, boardDir string) {
	t.Helper()
	dir = t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df36cli"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	boardDir = filepath.Join(dir, ".coding-hermes", "board")
	tasks := `{"id": "DF36C-1", "title": "alias row", "status": "review", "priority": "P1", "acceptance": "works", "created_at": "2026-09-01 00:00:00"}` + "\n" +
		`{"id": "DF36C-2", "title": "string deps + alias status", "status": "open", "priority": "P2", "depends_on": "[]", "created_at": "2026-09-01 00:00:00"}` + "\n" +
		`{"id": "DF36C-3", "title": "bare id deps", "status": "done", "priority": "P3", "depends_on": "DF36C-1", "created_at": "2026-09-01 00:00:00"}` + "\n" +
		`{"id": "DF36C-4", "title": "no status at all", "priority": "P2", "created_at": "2026-09-01 00:00:00"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, boardDir
}

// AC1: `normalize-board` -h documents the flags (BT-041 help contract).
// DF-BOARDCTL-46: --skip-bad-lines is GONE from this verb — it promised a
// degraded read the command cannot honor — and the help now points at the
// validate --repair salvage path instead.
func TestDF36CLIHelpAndUsage(t *testing.T) {
	got, err := captureStdout(func() { run([]string{"normalize-board", "-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--apply", "--json", "normalize-board", "validate --repair", "tasks.rewritten.jsonl"} {
		if !strings.Contains(got, want) {
			t.Errorf("normalize-board -h missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "--skip-bad-lines") {
		t.Errorf("normalize-board -h still advertises --skip-bad-lines (removed by DF-BOARDCTL-46; the command is fail-closed):\n%s", got)
	}
	if !strings.Contains(usageText, "normalize-board") {
		t.Errorf("usageText lost the normalize-board verb")
	}
	// The root usage entry and the closure must agree flag-for-flag: the
	// entry's usage line (the one that opens a flag bracket) must not carry
	// the removed flag. Prose continuation lines that NAME the flag as
	// deliberately absent are the documentation, not the usage list.
	for _, line := range strings.Split(usageText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "normalize-board [") && strings.Contains(trimmed, "--skip-bad-lines") {
			t.Errorf("usageText normalize-board usage line still lists --skip-bad-lines: %q", line)
		}
	}
	// The registry derives verbs from run()'s switch: the new verb must be
	// visible or docs tooling renders a stale command set.
	if _, ok := cliCommandNames()["normalize-board"]; !ok {
		t.Errorf("run() switch does not dispatch normalize-board (describe derivation will miss it)")
	}
}

// cliCommandNames re-derives the verb set the same way internal/describe
// does (switch-case scan), so the test fails when the switch anchor moves.
func cliCommandNames() map[string]bool {
	out := map[string]bool{}
	for _, line := range strings.Split(usageText, "\n") {
		if strings.HasPrefix(line, "  ") {
			fields := strings.Fields(line)
			if len(fields) > 0 && !strings.HasPrefix(fields[0], "-") {
				out[fields[0]] = true
			}
		}
	}
	return out
}

// AC2: dry-run exits 0, writes ONLY tasks.normalized.jsonl, and prints the
// changes + refusals + DRY-RUN banner.
func TestDF36CLIDryRun(t *testing.T) {
	dir, boardDir := df36SeedCLIBoard(t)
	tasksBefore, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out string
	out, err = captureStdout(func() {
		serr, cerr := captureStderr(func() {
			if code := run([]string{"-C", dir, "normalize-board"}); code != 0 {
				t.Errorf("dry-run exit = %d, want 0", code)
			}
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if serr != "" {
			t.Errorf("dry-run stderr = %q", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"DF36C-1",
		`key "acceptance" -> "acceptance_criteria"`,
		`depends_on "[]" -> []`,
		"status open -> pending",
		`["DF36C-1"]`,
		"REFUSED status",
		"DF36C-4",
		"DRY-RUN",
		"tasks.normalized.jsonl",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run stdout missing %q:\n%s", want, out)
		}
	}
	tasksAfter, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) {
		t.Fatal("dry-run modified tasks.jsonl")
	}
}

// AC3: --apply rewrites tasks.jsonl in place (spaced style preserved on
// untouched fields), and a validate after apply keeps the refused row as
// the only error-class finding outside warnings.
func TestDF36CLIApply(t *testing.T) {
	dir, boardDir := df36SeedCLIBoard(t)
	var out string
	out, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "normalize-board", "--apply"}); code != 0 {
			t.Errorf("apply exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "applied:") || !strings.Contains(out, "tasks.jsonl") {
		t.Fatalf("apply stdout missing the applied line:\n%s", out)
	}
	data, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"acceptance_criteria": "works"`,
		`"depends_on": []`,
		`"status": "pending"`,
		`"depends_on": ["DF36C-1"]`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("applied tasks.jsonl missing %q:\n%s", want, text)
		}
	}
	for _, gone := range []string{`"acceptance":`, `"depends_on": "[]"`, `"status": "done"`} {
		if strings.Contains(text, gone) {
			t.Errorf("applied tasks.jsonl still carries %q:\n%s", gone, text)
		}
	}
	// The refused row survives byte-identical.
	if !strings.Contains(text, `"id": "DF36C-4"`) {
		t.Fatalf("refused row lost:\n%s", text)
	}
}

// AC4: a clean board is a no-op with exit 0 and NO cache file.
func TestDF36CLICleanNoop(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df36ok"}); err != nil {
		t.Fatal(err)
	}
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	// init seeds the NEVER-DONE fixture (full canonical schema).
	var out string
	var code int
	out, err := captureStdout(func() {
		serr, cerr := captureStderr(func() {
			code = run([]string{"-C", dir, "normalize-board"})
		})
		if cerr != nil {
			t.Fatal(cerr)
		}
		if serr != "" {
			t.Errorf("clean-board stderr = %q", serr)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("clean-board exit = %d, want 0\nstdout:\n%s", code, out)
	}
	if !strings.Contains(out, "already canonical") {
		t.Fatalf("clean-board stdout missing the no-op line:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "tasks.normalized.jsonl")); err == nil {
		t.Fatal("clean board wrote a review cache file")
	}
}

// AC5: --json mirrors the report (the scripting surface).
func TestDF36CLIJSON(t *testing.T) {
	dir, _ := df36SeedCLIBoard(t)
	out, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "normalize-board", "--json"}); code != 0 {
			t.Errorf("json exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"changes"`, `"refusals"`, `"notes"`, `"applied": false`, `"rows": 4`} {
		if !strings.Contains(out, want) {
			t.Errorf("--json output missing %q:\n%s", want, out)
		}
	}
}
