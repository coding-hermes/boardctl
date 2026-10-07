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

// TestCmdUpdateDeferredTrueFalse: `update --deferred false` writes
// deferred:false on a deferred row; an omitted update leaves the key
// untouched; `--deferred true` defers a plain row.
func TestCmdUpdateDeferredTrueFalse(t *testing.T) {
	dir := seedBT076CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-2", "--deferred", "false"}); code != 0 {
		t.Fatalf("update --deferred false exit = %d, want 0", code)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(bt076TaskLine(t, dir, "EXIST-2")), &row); err != nil {
		t.Fatal(err)
	}
	if row["deferred"] != false {
		t.Fatalf("deferred = %v, want false", row["deferred"])
	}
	// omitted update leaves the key untouched
	before := bt076TaskLine(t, dir, "EXIST-2")
	if code := run([]string{"-C", dir, "update", "EXIST-2", "--summary", "note"}); code != 0 {
		t.Fatalf("omitted update exit = %d, want 0", code)
	}
	after := bt076TaskLine(t, dir, "EXIST-2")
	if !strings.Contains(after, `"deferred":false`) && !strings.Contains(after, `"deferred": false`) {
		t.Fatalf("omitted update changed the deferred key:\nbefore %s\nafter  %s", before, after)
	}
	if v, ok := rowOf(after)["deferred"]; !ok || v != false {
		t.Fatalf("omitted update altered deferred = %v (present=%v)", v, ok)
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
