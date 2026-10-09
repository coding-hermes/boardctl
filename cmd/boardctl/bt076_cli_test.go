package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-076 CLI coverage: --deferred on create (bare bool flag), --deferred
// true|false on update (strconv.ParseBool value flag), the help closures
// documenting both, and the stats surface carrying the separate deferred
// count with deferred rows excluded from actionable. The board-level write
// contract is pinned in internal/board (bt076_deferred_test.go).

// seedBT076CLIBoard seeds a board with one deferred pending row and one
// plain pending row, mirroring vocab_test.go's seedCLIBoard shape.
func seedBT076CLIBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"id":"EXIST-1","title":"plain","status":"pending","priority":"P2","depends_on":[]}` + "\n" +
			`{"id":"EXIST-2","title":"parked","status":"pending","priority":"P1","deferred":true}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"demo","namespace":"demo","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func bt076TaskLine(t *testing.T, dir, id string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, `"id":"`+id+`"`) {
			return l
		}
	}
	t.Fatalf("task %q not found in %s", id, raw)
	return ""
}

// TestCmdCreateDeferredWritesFlag: `create --deferred` writes deferred:true.
func TestCmdCreateDeferredWritesFlag(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "DF-C1", "--title", "deferred row", "--deferred"}); code != 0 {
		t.Fatalf("create --deferred exit = %d, want 0", code)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(bt076TaskLine(t, dir, "DF-C1")), &row); err != nil {
		t.Fatal(err)
	}
	if row["deferred"] != true {
		t.Fatalf("create --deferred wrote deferred = %v, want true", row["deferred"])
	}
}

// TestCmdCreateOmittedDeferredWritesNoKey: the omitted flag writes no key,
// even though the mirrored last row carries deferred:true (no null
// placeholder may leak into the new row).
func TestCmdCreateOmittedDeferredWritesNoKey(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "DF-C2", "--title", "plain row"}); code != 0 {
		t.Fatalf("create exit = %d, want 0", code)
	}
	line := bt076TaskLine(t, dir, "DF-C2")
	if strings.Contains(line, "deferred") {
		t.Fatalf("omitted --deferred wrote the key anyway (mirrored?): %s", line)
	}
}

// TestCmdUpdateDeferredTrueFalse: `update --deferred false` REMOVES the
// deferred key on a deferred row (DF-BOARDCTL-29 — the row reads exactly
// like a never-deferred one afterwards); an omitted update never creates
// the key; `--deferred true` defers a plain row.
func TestCmdUpdateDeferredTrueFalse(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-2", "--deferred", "false"}); code != 0 {
		t.Fatalf("update --deferred false exit = %d, want 0", code)
	}
	line := bt076TaskLine(t, dir, "EXIST-2")
	if strings.Contains(line, "deferred") {
		t.Fatalf("update --deferred false left a deferred key behind: %s", line)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatal(err)
	}
	if _, present := row["deferred"]; present {
		t.Fatalf("deferred key still present after --deferred false: %s", line)
	}
	// an omitted update never recreates the key
	if code := run([]string{"-C", dir, "update", "EXIST-2", "--summary", "note"}); code != 0 {
		t.Fatalf("omitted update exit = %d, want 0", code)
	}
	if v, ok := rowOf(bt076TaskLine(t, dir, "EXIST-2"))["deferred"]; ok {
		t.Fatalf("omitted update created deferred = %v", v)
	}
	// defer a plain row
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--deferred", "true"}); code != 0 {
		t.Fatalf("update --deferred true exit = %d, want 0", code)
	}
	if v := rowOf(bt076TaskLine(t, dir, "EXIST-1"))["deferred"]; v != true {
		t.Fatalf("deferred = %v, want true", v)
	}
}

// rowOf parses one task line into a map (helper shared by this file).
func rowOf(line string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return map[string]any{}
	}
	return m
}

// TestCmdCreateThenUndeferRemovesKey: the full DF-BOARDCTL-29 lifecycle via
// the CLI surface — create --deferred writes the key, update --deferred
// false removes it entirely (the parsed row carries no deferred key), and
// stats afterwards counts the row as NOT deferred (deferred drops to 0,
// actionable picks the row back up).
func TestCmdCreateThenUndeferRemovesKey(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "DF-29A", "--title", "parked", "--deferred"}); code != 0 {
		t.Fatalf("create --deferred exit = %d, want 0", code)
	}
	if v := rowOf(bt076TaskLine(t, dir, "DF-29A"))["deferred"]; v != true {
		t.Fatalf("create --deferred did not defer: %s", bt076TaskLine(t, dir, "DF-29A"))
	}
	out, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "update", "DF-29A", "--deferred", "false"}); code != 0 {
			t.Fatalf("update --deferred false exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	line := bt076TaskLine(t, dir, "DF-29A")
	if strings.Contains(line, "deferred") {
		t.Fatalf("update --deferred false left a deferred key behind: %s", line)
	}
	if _, present := rowOf(line)["deferred"]; present {
		t.Fatalf("deferred key still present after un-defer: %s", line)
	}
	if !strings.Contains(out, "deferred") {
		t.Fatalf("update output did not report the deferred change:\n%s", out)
	}
	// stats: the un-deferred pending row is actionable again. After the
	// un-defer only EXIST-2 (still deferred) stays out of actionable:
	// 3 pending rows total − 1 deferred = 2.
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "deferred: 1") {
		t.Fatalf("stats deferred = want 1 (only EXIST-2 still deferred):\n%s", got)
	}
	if !strings.Contains(got, "actionable: 2") {
		t.Fatalf("stats actionable = want 2 (DF-29A is pending and no longer deferred):\n%s", got)
	}
	// un-defer EXIST-2 too: the seeded deferred row un-defers to a row with
	// NO deferred key at all (the DF-BOARDCTL-29 nil-out on pre-existing
	// boards), and stats drop to deferred: 0.
	if code := run([]string{"-C", dir, "update", "EXIST-2", "--deferred", "false"}); code != 0 {
		t.Fatalf("update EXIST-2 --deferred false exit = %d, want 0", code)
	}
	if strings.Contains(bt076TaskLine(t, dir, "EXIST-2"), "deferred") {
		t.Fatalf("un-defer of the seeded deferred row left the key: %s", bt076TaskLine(t, dir, "EXIST-2"))
	}
	got2, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got2, "deferred: 0") {
		t.Fatalf("stats deferred = want 0 after un-deferring both rows:\n%s", got2)
	}
	if !strings.Contains(got2, "actionable: 3") {
		t.Fatalf("stats actionable = want 3 (all 3 pending rows, none deferred):\n%s", got2)
	}
}

// TestCmdUpdateDeferredInvalidValue: a non-boolean spelling is a usage
// error (exit 2, the flag-level failure class) and writes nothing.
func TestCmdUpdateDeferredInvalidValue(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	code := 0
	stderr, err := captureStderr(func() {
		code = run([]string{"-C", dir, "update", "EXIST-1", "--deferred", "maybe"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 && code != 2 {
		t.Fatalf("update --deferred maybe exit = %d, want 1 or 2 (usage/validation)", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("rejected --deferred value mutated tasks.jsonl")
	}
	if !strings.Contains(stderr, "not a boolean") {
		t.Fatalf("stderr %q missing \"not a boolean\"", stderr)
	}
}

// TestCmdHelpListsDeferredFlag: both closures and the root usage document
// the new flags.
func TestCmdHelpListsDeferredFlag(t *testing.T) {
	createHelp, err := captureStdout(func() {
		if code := run([]string{"create", "-h"}); code != 0 {
			t.Fatalf("create -h exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(createHelp, "--deferred") {
		t.Fatalf("create -h missing --deferred:\n%s", createHelp)
	}
	updateHelp, err := captureStdout(func() {
		if code := run([]string{"update", "-h"}); code != 0 {
			t.Fatalf("update -h exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updateHelp, "--deferred") {
		t.Fatalf("update -h missing --deferred:\n%s", updateHelp)
	}
	if !strings.Contains(usageText, "--deferred") {
		t.Fatal("root usageText missing --deferred")
	}
}

// TestCmdStatsDeferredCounts: stats output carries the separate deferred
// count, excludes deferred rows from actionable, and keeps them in total and
// in the status tally — via the CLI surface.
func TestCmdStatsDeferredCounts(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats"}); code != 0 {
			t.Fatalf("stats exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "total tasks: 2") {
		t.Fatalf("stats output missing total tasks: 2:\n%s", got)
	}
	if !strings.Contains(got, "actionable: 1") {
		t.Fatalf("stats output missing actionable: 1 (deferred pending row must be excluded):\n%s", got)
	}
	if !strings.Contains(got, "deferred: 1") {
		t.Fatalf("stats output missing deferred: 1:\n%s", got)
	}
	if !strings.Contains(got, "pending") {
		t.Fatalf("stats output lost the status tally:\n%s", got)
	}
	// JSON stats carry the key too
	gotJSON, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats", "--json"}); code != 0 {
			t.Fatalf("stats --json exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Total      int `json:"total"`
		Deferred   int `json:"deferred"`
		Actionable int `json:"actionable"`
		Status     map[string]int
	}
	if err := json.Unmarshal([]byte(gotJSON), &st); err != nil {
		t.Fatalf("stats --json not parseable: %v\n%s", err, gotJSON)
	}
	if st.Deferred != 1 || st.Actionable != 1 || st.Total != 2 || st.Status["pending"] != 2 {
		t.Fatalf("JSON stats = %+v, want deferred 1, actionable 1, total 2, pending 2", st)
	}
}
