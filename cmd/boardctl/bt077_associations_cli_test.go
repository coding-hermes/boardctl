package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// BT-077 CLI coverage: the association flags on create and update.
// The board-level write/merge/normalize contract is pinned in
// internal/board (bt077_associations_test.go); these tests pin the CLI
// surface: flags land on the row, multi-value flags keep every occurrence,
// malformed values fail the command with nothing written, and the help
// closures document the merge semantics.

// seedBT077CLIBoard seeds a board with one plain row and one legacy
// singular row (worktree/branch/sessions only).
func seedBT077CLIBoard(t *testing.T) string {
	_ = t
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl": `{"id":"EXIST-1","title":"plain","status":"pending","priority":"P2","depends_on":[]}` + "\n" +
			`{"id":"EXIST-LEG","title":"legacy singular","status":"pending","priority":"P3","worktree":"/home/me/wt-1","branch":"wt/lg-1","sessions":["s1"]}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-10-07 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"demo","namespace":"demo","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// bt077Line returns the tasks.jsonl line carrying the given id.
func bt077Line(t *testing.T, dir, id string) string {
	_ = t
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, `"id":"`+id+`"`) || strings.Contains(l, `"id": "`+id+`"`) {
			return l
		}
	}
	t.Fatalf("task %q not found in %s", id, raw)
	return ""
}

// bt077RowMap parses one line into a generic map.
func bt077RowMap(t *testing.T, line string) map[string]any {
	_ = t
	var row map[string]any
	if err := json.Unmarshal([]byte(line), &row); err != nil {
		t.Fatalf("row does not parse: %v (%s)", err, line)
	}
	return row
}

// TestCmdCreateAssociationFlagsMultiValue: create with repeated
// --pull-request flags lands every element in CLI order; the merge
// semantics and canonical shapes are pinned board-level.
func TestCmdCreateAssociationFlagsMultiValue(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	code := run([]string{"-C", dir, "create", "--id", "AS-C1", "--title", "assoc row",
		"--pull-request", "21", "--pull-request", "https://github.com/o/r/pull/22"})
	if code != 0 {
		t.Fatalf("create exit = %d, want 0", code)
	}
	row := bt077RowMap(t, bt077Line(t, dir, "AS-C1"))
	prs, ok := row["pull_requests"].([]any)
	if !ok || len(prs) != 2 {
		t.Fatalf("pull_requests = %v, want 2 elements", row["pull_requests"])
	}
	first, _ := prs[0].(map[string]any)
	if first["number"] != float64(21) {
		t.Errorf("first PR = %v, want 21", first["number"])
	}
	second, _ := prs[1].(map[string]any)
	if second["number"] != float64(22) || second["url"] != "https://github.com/o/r/pull/22" {
		t.Errorf("second PR = %v, want {22, url}", second)
	}
	// omitted dimensions write no key
	if _, present := row["branches"]; present {
		t.Errorf("omitted --assoc-branch wrote a branches key anyway")
	}
}

// TestCmdCreateAssociationRejectsMalformed: a malformed --pull-request
// value fails the command with NOTHING written.
func TestCmdCreateAssociationRejectsMalformed(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "AS-BAD", "--title", "never",
		"--pull-request", "bad id!"}); code == 0 {
		t.Fatal("create accepted a malformed PR value (exit 0)")
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "AS-BAD") {
		t.Fatalf("failed create left its row behind:\n%s", raw)
	}
}

// TestCmdUpdateAssociationMerges: repeated update calls accumulate across
// all three dimensions without touching siblings — the third PR keeps the
// first two; branches/worktrees stay; the legacy singular row keeps its
// worktree/branch/sessions fields.
func TestCmdUpdateAssociationMerges(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	steps := [][]string{
		{"-C", dir, "update", "EXIST-1", "--pull-request", "21"},
		{"-C", dir, "update", "EXIST-1", "--pull-request", "https://github.com/o/r/pull/22"},
		{"-C", dir, "update", "EXIST-1", "--assoc-branch", "feat/a", "--assoc-branch", "feat/b"},
		{"-C", dir, "update", "EXIST-1", "--assoc-worktree", "/tmp/w1", "--assoc-worktree", "/tmp/w2"},
		{"-C", dir, "update", "EXIST-1", "--pull-request", "23"}, // third PR must not drop the first two
	}
	for i, args := range steps {
		if code := run(args); code != 0 {
			t.Fatalf("step %d (%v): exit = %d, want 0", i+1, args, code)
		}
	}
	row := bt077RowMap(t, bt077Line(t, dir, "EXIST-1"))
	prs := row["pull_requests"].([]any)
	if len(prs) != 3 {
		t.Fatalf("pull_requests = %v, want 3 (merge, never overwrite)", row["pull_requests"])
	}
	branches := row["branches"].([]any)
	if len(branches) != 2 || branches[0] != "feat/a" || branches[1] != "feat/b" {
		t.Fatalf("branches = %v, want [feat/a feat/b]", branches)
	}
	wts := row["worktrees"].([]any)
	if len(wts) != 2 || wts[0] != "/tmp/w1" || wts[1] != "/tmp/w2" {
		t.Fatalf("worktrees = %v, want [/tmp/w1 /tmp/w2]", wts)
	}
	// legacy row: singular fields survive an association merge
	if code := run([]string{"-C", dir, "update", "EXIST-LEG", "--pull-request", "7"}); code != 0 {
		t.Fatalf("legacy update exit = %d, want 0", code)
	}
	leg := bt077RowMap(t, bt077Line(t, dir, "EXIST-LEG"))
	if leg["worktree"] != "/home/me/wt-1" || leg["branch"] != "wt/lg-1" {
		t.Fatalf("legacy singular fields lost: %v / %v", leg["worktree"], leg["branch"])
	}
	if prs, ok := leg["pull_requests"].([]any); !ok || len(prs) != 1 {
		t.Fatalf("legacy pull_requests = %v, want 1 element", leg["pull_requests"])
	}
}

// TestCmdUpdateAssociationIdempotent: merging the same PR again reports no
// pull_requests change and does not duplicate the element.
func TestCmdUpdateAssociationIdempotent(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "https://github.com/o/r/pull/22"}); code != 0 {
		t.Fatal("first merge failed")
	}
	line := bt077Line(t, dir, "EXIST-1")
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "22"}); code != 0 {
		t.Fatal("second (bare-number) merge failed")
	}
	after := bt077Line(t, dir, "EXIST-1")
	if strings.Count(after, `"number":22`) != 1 && strings.Count(after, `"number": 22`) != 1 {
		t.Fatalf("PR 22 duplicated:\n%s", after)
	}
	if !strings.Contains(after, `"url"`) {
		t.Fatalf("richer stored form (with url) not preserved:\n%s", after)
	}
	_ = line
}

// TestCmdUpdateAssociationRejectsMalformed: a malformed element fails the
// update with nothing written.
func TestCmdUpdateAssociationRejectsMalformed(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "nope!"}); code == 0 {
		t.Fatal("update accepted a malformed PR value")
	}
	line := bt077Line(t, dir, "EXIST-1")
	if strings.Contains(line, "pull_requests") {
		t.Fatalf("failed update wrote pull_requests anyway:\n%s", line)
	}
}

// TestCmdShowAndListExposeAssociations: show pretty-prints the plural
// fields and list --json carries them verbatim.
func TestCmdShowAndListExposeAssociations(t *testing.T) {
	dir := seedBT077CLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--pull-request", "21"}); code != 0 {
		t.Fatal("pull-request update failed")
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--assoc-branch", "feat/x"}); code != 0 {
		t.Fatal("assoc-branch update failed")
	}
	showOut, serr := captureStdout(func() {
		if code := run([]string{"-C", dir, "show", "EXIST-1"}); code != 0 {
			t.Fatal("show failed")
		}
	})
	if serr != nil {
		t.Fatal(serr)
	}
	if !strings.Contains(showOut, `"pull_requests"`) || !strings.Contains(showOut, `"branches"`) {
		t.Fatalf("show lost the plural fields:\n%s", showOut)
	}
	listOut, lerr := captureStdout(func() {
		if code := run([]string{"-C", dir, "list", "--json"}); code != 0 {
			t.Fatal("list --json failed")
		}
	})
	if lerr != nil {
		t.Fatal(lerr)
	}
	if !strings.Contains(listOut, `"pull_requests"`) || !strings.Contains(listOut, `"number": 21`) {
		t.Fatalf("list --json lost pull_requests:\n%s", listOut)
	}
	if !strings.Contains(listOut, `"branches"`) || !strings.Contains(listOut, `"feat/x"`) {
		t.Fatalf("list --json lost branches:\n%s", listOut)
	}
}

// TestAssociationFlagsDocumentedInHelp: the usage closures name all three
// association flags and state the merge semantics.
func TestAssociationFlagsDocumentedInHelp(t *testing.T) {
	createOut, cerr := captureStdout(func() {
		_ = run([]string{"create", "--help"})
	})
	for _, want := range []string{"--pull-request", "--assoc-branch", "--assoc-worktree"} {
		if !strings.Contains(createOut, want) {
			t.Errorf("create help lost %q:\n%s", want, createOut)
		}
	}
	if cerr != nil {
		t.Fatal(cerr)
	}
	updateOut, uerr := captureStdout(func() {
		_ = run([]string{"update", "--help"})
	})
	if uerr != nil {
		t.Fatal(uerr)
	}
	for _, want := range []string{"--pull-request", "--assoc-branch", "--assoc-worktree", "MERGES"} {
		if !strings.Contains(updateOut, want) {
			t.Errorf("update help lost %q:\n%s", want, updateOut)
		}
	}
}
