package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-68 CLI coverage: `update --note` appends to the existing foreman_note
// (the old REPLACE silently destroyed it), `--replace` opts back into the
// overwrite, and both usage surfaces document the flag. The board-level
// write contract is pinned in internal/board (bt068_note_append_test.go);
// these tests pin the CLI surface — flag parsing through reorderArgs,
// exit codes, and the -h usage closure — following bt037_cli_test.go's shape.

// bt068SeedNote gives the seeded EXIST-1 row a foreman_note with the given
// JSON string literal (compact style, appended after depends_on).
func bt068SeedNote(t *testing.T, dir, jsonLiteral string) {
	t.Helper()
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	old := `"depends_on":[]`
	new := `"depends_on":[],"foreman_note":` + jsonLiteral
	out := strings.Replace(string(raw), old, new, 1)
	if out == string(raw) {
		t.Fatalf("seed anchor %s not found in tasks.jsonl", old)
	}
	if err := os.WriteFile(tasksPath, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bt068CmdNote reads the foreman_note of the tasks.jsonl row with the given
// id, plus whether the key is present.
func bt068CmdNote(t *testing.T, dir, id string) (string, bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("tasks.jsonl line does not parse: %v (%s)", err, l)
		}
		if row["id"] == id {
			v, ok := row["foreman_note"]
			if !ok {
				return "", false
			}
			s, ok := v.(string)
			if !ok {
				t.Fatalf("foreman_note is not a JSON string: %T (%v)", v, v)
			}
			return s, true
		}
	}
	t.Fatalf("task %s not found in tasks.jsonl", id)
	return "", false
}

// TestCmdUpdateNoteAppends: the acceptance case — a row with foreman_note
// "A" + `update --note B` ends with "A\n\nB" (exit 0), every other
// tasks.jsonl line byte-identical. RED on the pre-fix CLI: the old REPLACE
// wrote exactly "B".
func TestCmdUpdateNoteAppends(t *testing.T) {
	dir := seedCLIBoard(t)
	bt068SeedNote(t, dir, `"A"`)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}

	if code := run([]string{"-C", dir, "update", "EXIST-1", "--note", "B"}); code != 0 {
		t.Fatalf("update --note exit = %d, want 0", code)
	}

	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Split(strings.TrimRight(string(before), "\n"), "\n")
	afterLines := strings.Split(strings.TrimRight(string(after), "\n"), "\n")
	if len(beforeLines) != len(afterLines) {
		t.Fatalf("line count changed: %d -> %d", len(beforeLines), len(afterLines))
	}
	for i := range beforeLines {
		if strings.Contains(beforeLines[i], `"id":"EXIST-1"`) {
			continue // the target row is the one allowed to differ
		}
		if beforeLines[i] != afterLines[i] {
			t.Fatalf("untouched line %d changed:\nbefore %s\nafter  %s", i+1, beforeLines[i], afterLines[i])
		}
	}
	got, ok := bt068CmdNote(t, dir, "EXIST-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from EXIST-1")
	}
	if got != "A\n\nB" {
		t.Fatalf("foreman_note = %q, want %q (old REPLACE dropped the existing note)", got, "A\n\nB")
	}
}

// TestCmdUpdateNoteReplaceFlagOverwrites: `--replace` restores the pre-BT-68
// overwrite — exactly "Z", no accumulation.
func TestCmdUpdateNoteReplaceFlagOverwrites(t *testing.T) {
	dir := seedCLIBoard(t)
	bt068SeedNote(t, dir, `"A"`)

	if code := run([]string{"-C", dir, "update", "EXIST-1", "--note", "Z", "--replace"}); code != 0 {
		t.Fatalf("update --note --replace exit = %d, want 0", code)
	}
	got, ok := bt068CmdNote(t, dir, "EXIST-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from EXIST-1")
	}
	if got != "Z" {
		t.Fatalf("foreman_note = %q, want %q (--replace must overwrite)", got, "Z")
	}
}

// TestCmdUpdateReplaceIsNotAValueFlag: `--replace` takes no value, so it
// must NOT sit in the reorderArgs valueFlags list — spelled BEFORE --note
// with no `=`, a wrongly-registered flag would swallow `--note` as its
// value, leave "Z" as a positional, and die on "exactly one task id".
func TestCmdUpdateReplaceIsNotAValueFlag(t *testing.T) {
	dir := seedCLIBoard(t)
	bt068SeedNote(t, dir, `"A"`)

	if code := run([]string{"-C", dir, "update", "EXIST-1", "--replace", "--note", "Z"}); code != 0 {
		t.Fatalf("update --replace --note exit = %d, want 0 (--replace must not eat the next flag)", code)
	}
	got, _ := bt068CmdNote(t, dir, "EXIST-1")
	if got != "Z" {
		t.Fatalf("foreman_note = %q, want %q (--replace parsed as a value flag?)", got, "Z")
	}
}

// TestCmdUpdateNoteOmittedLeavesNoteUntouched: an omitted --note never
// touches foreman_note — no key created on a row that lacks it, no value
// change on a row that carries it.
func TestCmdUpdateNoteOmittedLeavesNoteUntouched(t *testing.T) {
	// seeded EXIST-1 has NO foreman_note key
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--title", "retitled"}); code != 0 {
		t.Fatalf("update --title exit = %d, want 0", code)
	}
	if _, ok := bt068CmdNote(t, dir, "EXIST-1"); ok {
		t.Fatalf("foreman_note key created by an update that did not set --note")
	}

	// row WITH the key: an unrelated flag leaves the value alone
	dir2 := seedCLIBoard(t)
	bt068SeedNote(t, dir2, `"A"`)
	if code := run([]string{"-C", dir2, "update", "EXIST-1", "--status", "in_progress"}); code != 0 {
		t.Fatalf("update --status exit = %d, want 0", code)
	}
	got, ok := bt068CmdNote(t, dir2, "EXIST-1")
	if !ok {
		t.Fatalf("foreman_note key vanished from EXIST-1")
	}
	if got != "A" {
		t.Fatalf("foreman_note = %q, want %q (omitted --note must leave the value alone)", got, "A")
	}
}

// TestCmdUpdateHelpListsReplace: the update -h closure documents --replace
// and the appending --note (extends TestCmdUpdateHelpListsEveryValueFlag's
// shape; does not weaken it).
func TestCmdUpdateHelpListsReplace(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"update", "-h"}); code != 0 {
			t.Fatalf("update -h exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--replace", "--note S (appends)"} {
		if !strings.Contains(got, want) {
			t.Errorf("update -h usage output missing %q; got:\n%q", want, got)
		}
	}
}
