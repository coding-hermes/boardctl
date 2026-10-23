package main

// DF-BOARDCTL-50 CLI half: the flag surfaces and the omission contract.
//
// internal/board/write_acceptance_criteria_test.go pins the write machinery;
// these tests hold the REAL CLI (run(), real init/create/update/validate
// paths on disposable boards) to the parts a caller actually sees:
//
//   1. `create --acceptance-criteria TEXT` exits 0, prints "created task
//      <id>" and persists the criteria — multi-line text included, as one
//      physical JSONL line;
//   2. `update <id> --acceptance-criteria TEXT` exits 0 and persists it too
//      (the row here never carried the key, so this is the append path);
//   3. omitting the flag writes NO acceptance_criteria key at all, and
//      `validate` still reports RESULT: OK with exit 0;
//   4. `create --help` and `update --help` document the flag, so the
//      dispatchability note stops implying a hand edit.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// df50Criteria is multi-line on purpose (see the internal/board twin).
const df50Criteria = "1) create a conforming row with the flag\n2) update the row and read it back\n3) — multi-line, \"quoted\",\tone JSON string"

// df50CLIRowLine returns the raw tasks.jsonl line for a task id, failing when
// absent. The bytes are ONE physical line — the point of the multi-line case.
func df50CLIRowLine(t *testing.T, dir, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(qaBoardDir(dir), "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue // topology-B header line
		}
		if s, ok := row["id"].(string); ok && s == id {
			return line
		}
	}
	t.Fatalf("task %q not found in tasks.jsonl", id)
	return ""
}

// df50CLIRow decodes the raw line of a task id into a key/value map.
func df50CLIRow(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(df50CLIRowLine(t, dir, id)), &row); err != nil {
		t.Fatalf("row %q does not parse: %v", id, err)
	}
	return row
}

// df50SeedCLIBoard bootstraps a fresh board through the real init path.
func df50SeedCLIBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "df50"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

// TestDF50CreateAcceptanceCriteriaCLI is criteria 1 (create half) and 3
// (multi-line value stays one physical line) at the CLI surface.
func TestDF50CreateAcceptanceCriteriaCLI(t *testing.T) {
	dir := df50SeedCLIBoard(t)
	code, out := qaRunCLI(t, dir, "create", "--id", "TEST-AA-1", "--title", "t",
		"--acceptance-criteria", df50Criteria)
	if code != 0 {
		t.Fatalf("create with --acceptance-criteria exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "created task TEST-AA-1") {
		t.Fatalf("create output missing the created-task line:\n%s", out)
	}

	line := df50CLIRowLine(t, dir, "TEST-AA-1")
	if strings.ContainsAny(line, "\r") {
		t.Fatalf("row carries a raw carriage return: %q", line)
	}
	if !strings.Contains(line, `\n`) {
		t.Fatalf("multi-line criteria were not JSON-escaped onto the row line: %s", line)
	}
	row := df50CLIRow(t, dir, "TEST-AA-1")
	got, ok := row["acceptance_criteria"].(string)
	if !ok {
		t.Fatalf("acceptance_criteria missing or not a string on the created row: %s", line)
	}
	if got != df50Criteria {
		t.Fatalf("criteria round-trip mismatch:\n got %q\nwant %q", got, df50Criteria)
	}
}

// TestDF50UpdateAcceptanceCriteriaCLI is criteria 1's update half: a row
// created WITHOUT the key gains it through update.
func TestDF50UpdateAcceptanceCriteriaCLI(t *testing.T) {
	dir := df50SeedCLIBoard(t)
	if code, out := qaRunCLI(t, dir, "create", "--id", "TEST-AA-1", "--title", "t"); code != 0 {
		t.Fatalf("seed create exit = %d, want 0\noutput:\n%s", code, out)
	}
	if strings.Contains(df50CLIRowLine(t, dir, "TEST-AA-1"), "acceptance_criteria") {
		t.Fatal("premise: seed row already carries acceptance_criteria")
	}

	code, out := qaRunCLI(t, dir, "update", "TEST-AA-1", "--acceptance-criteria", df50Criteria)
	if code != 0 {
		t.Fatalf("update with --acceptance-criteria exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "acceptance_criteria") {
		t.Fatalf("update output does not report the acceptance_criteria change:\n%s", out)
	}

	row := df50CLIRow(t, dir, "TEST-AA-1")
	got, ok := row["acceptance_criteria"].(string)
	if !ok {
		t.Fatalf("acceptance_criteria missing after update: %s", df50CLIRowLine(t, dir, "TEST-AA-1"))
	}
	if got != df50Criteria {
		t.Fatalf("criteria round-trip mismatch after update:\n got %q\nwant %q", got, df50Criteria)
	}
	if row["title"] != "t" {
		t.Fatalf("update clobbered the title: %v", row["title"])
	}
}

// TestDF50OmittedWritesNoKeyAndValidates is criterion 4: omission writes no
// acceptance_criteria key, and validate stays RESULT: OK / exit 0.
func TestDF50OmittedWritesNoKeyAndValidates(t *testing.T) {
	dir := df50SeedCLIBoard(t)
	if code, out := qaRunCLI(t, dir, "create", "--id", "TEST-AA-1", "--title", "t"); code != 0 {
		t.Fatalf("create without the flag exit = %d, want 0\noutput:\n%s", code, out)
	}
	line := df50CLIRowLine(t, dir, "TEST-AA-1")
	if strings.Contains(line, "acceptance_criteria") {
		t.Fatalf("omitted flag still wrote an acceptance_criteria key: %s", line)
	}

	code, out := qaRunCLI(t, dir, "validate")
	if code != 0 {
		t.Fatalf("validate exit = %d, want 0\noutput:\n%s", code, out)
	}
	if !strings.Contains(out, "RESULT: OK") {
		t.Fatalf("validate did not report RESULT: OK\noutput:\n%s", out)
	}
}

// TestDF50HelpDocumentsAcceptanceCriteria is criterion 2: both help surfaces
// name the flag (the create usage was the inconsistent one for every prior
// field-flag addition, so both are asserted).
func TestDF50HelpDocumentsAcceptanceCriteria(t *testing.T) {
	for _, verb := range []string{"create", "update"} {
		verb := verb
		t.Run(verb+" --help", func(t *testing.T) {
			got, err := captureStdout(func() {
				if code := run([]string{verb, "--help"}); code != 0 {
					t.Fatalf("%s --help exit code = %d, want 0", verb, code)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(got, "acceptance-criteria"); n < 1 {
				t.Fatalf("%s --help does not document --acceptance-criteria (%d matches):\n%s", verb, n, got)
			}
		})
	}
}
