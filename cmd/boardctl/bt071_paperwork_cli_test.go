package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-071 CLI coverage: the paperwork-lane priority default at the CLI
// surface. The board-level rule is pinned in internal/board
// (bt071_paperwork_priority_test.go); these tests drive `boardctl create`
// end to end. There is no --lane flag: a board's lane is its identity (the
// header project/namespace, else the board dir's parent — Board.LaneName),
// so each case builds a board whose lane name carries the suffix under test.

// seedLaneCLIBoard writes a topology-A board in a temp dir whose lane
// identity is the given name, following seedCLIBoard's shape.
func seedLaneCLIBoard(t *testing.T, lane string) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2","guard_result":null,"ci_result":null,"depends_on":[]}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"` + lane + `","namespace":"` + lane + `","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// lastTaskPriorityOf reads the LAST tasks.jsonl row's priority from the
// board under dir.
func lastTaskPriorityOf(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &row); err != nil {
		t.Fatal(err)
	}
	p, _ := row["priority"].(string)
	return p
}

// TestCmdCreatePaperworkLanePriorities: acceptance cases 1-5 — the default
// lands P5 on -docs/-readme, P4 on -review, honors an explicit --priority,
// and keeps the historical P2 on a non-paperwork lane (-qa).
func TestCmdCreatePaperworkLanePriorities(t *testing.T) {
	cases := []struct {
		lane     string
		priority string // "" = omit --priority
		want     string
	}{
		{"my-project-docs", "", "P5"},
		{"my-project-readme", "", "P5"},
		{"my-project-review", "", "P4"},
		{"my-project-docs", "P2", "P2"}, // explicit wins over the default
		{"my-project-qa", "", "P2"},     // non-paperwork keeps the old default
		{"my-project-pm", "", "P2"},     // operational, NOT paperwork
		{"my-project-sync", "", "P2"},   // operational, NOT paperwork
		{"my-project", "", "P2"},        // primary lane unchanged
	}
	for _, c := range cases {
		dir := seedLaneCLIBoard(t, c.lane)
		args := []string{"-C", dir, "create", "--id", "PAPER-CLI-1", "--title", "Write the report"}
		if c.priority != "" {
			args = append(args, "--priority", c.priority)
		}
		if code := run(args); code != 0 {
			t.Fatalf("lane %q: create exit = %d, want 0", c.lane, code)
		}
		if got := lastTaskPriorityOf(t, dir); got != c.want {
			t.Fatalf("lane %q (priority flag %q): row priority = %q, want %q", c.lane, c.priority, got, c.want)
		}
	}
}

// TestCmdCreateHelpDocumentsPaperworkDefault: the create usage closure names
// the lane-dependent default, so a --help reader learns the boundary without
// the README.
func TestCmdCreateHelpDocumentsPaperworkDefault(t *testing.T) {
	out, err := captureStdout(func() { run([]string{"create", "-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-review → P4") || !strings.Contains(out, "-docs/-readme → P5") {
		t.Fatalf("create -h does not document the paperwork default; got:\n%s", out)
	}
}
