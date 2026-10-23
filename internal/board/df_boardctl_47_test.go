package board

// DF-BOARDCTL-47 library regressions: the umbrella's line-level read modes.
//
// Umbrella reads aggregated the single-repo read paths, so before this row a
// single unparseable line in one child board aborted Stats/ListTasks/census
// with that board's error and no partial view. SetSkipBad arms the existing
// DF-BOARDCTL-9 tolerant reader on EVERY board in the view; SkippedLines /
// RenderSkippedLines carry the evidence with repo provenance, and the CLI maps
// that evidence to exit 1. The default (SetSkipBad never called) is unchanged.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// df47BadLine is the live shape (warpfs line 412): a duplicate JSON key. It IS
// valid JSON — only the JSONL row parser rejects it.
const df47BadLine = `{"id": "DF47-BAD", "title": "duplicate worker_status", "status": "pending", "priority": "P2", "worker_status": "complete", "worker_status": "review"}`

// df47AppendBadLine appends the duplicate-key row to repo's tasks.jsonl and
// returns its 1-based physical line number. The premises are asserted so the
// fixture cannot rot into a vacuous one.
func df47AppendBadLine(t *testing.T, repo string) int {
	t.Helper()
	if !json.Valid([]byte(df47BadLine)) {
		t.Fatalf("premise broken: the bad line is not valid JSON: %s", df47BadLine)
	}
	if _, err := ParseRow([]byte(df47BadLine)); err == nil {
		t.Fatal("premise broken: ParseRow accepted a duplicate-key line")
	} else if !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("premise broken: ParseRow refused it for another reason: %v", err)
	}
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(b.TasksPath(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(df47BadLine + "\n")); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "\n")
}

// df47Fixture is the dogfood shape: childA healthy with 2 tasks, childB with 1
// healthy task followed by one duplicate-key line (the SECOND child).
func df47Fixture(t *testing.T) (umbrella, childA, childB string, badLine int) {
	t.Helper()
	umbrella, childA, childB = umbFixture(t)
	umbSeedTask(t, childA, "CHILDA-1", "childA one")
	umbSeedTask(t, childA, "CHILDA-2", "childA two")
	umbSeedTask(t, childB, "CHILDB-1", "childB one")
	badLine = df47AppendBadLine(t, childB)
	return umbrella, childA, childB, badLine
}

// TestUmbrellaDefaultFailClosed: without SetSkipBad every aggregate read keeps
// the pre-existing abort — same wrapped "board <repo> (<dir>): <file>: line N"
// message, and no skip evidence is collected.
func TestUmbrellaDefaultFailClosed(t *testing.T) {
	umbrella, childA, childB, badLine := df47Fixture(t)
	_ = childA

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := um.Stats(TaskFilter{}); err == nil {
		t.Fatal("default Stats on a malformed child board = no error, want the fail-closed abort")
	} else {
		msg := err.Error()
		for _, want := range []string{"board childB", "tasks.jsonl", "line " + strconv.Itoa(badLine), `duplicate key "worker_status"`} {
			if !strings.Contains(msg, want) {
				t.Fatalf("default abort message missing %q: %v", want, err)
			}
		}
	}
	if _, err := um.ListTasks(TaskFilter{}); err == nil {
		t.Fatal("default ListTasks on a malformed child board = no error, want the abort")
	}
	if um.HasSkipped() || len(um.SkippedLines()) != 0 {
		t.Fatalf("default mode collected skip evidence: %+v", um.SkippedLines())
	}
	if got := um.RenderSkippedLines(); got != "" {
		t.Fatalf("default mode rendered a SKIPPED-LINES block: %q", got)
	}
	_ = childB
}

// TestUmbrellaSkipBadAggregatesHealthyBoards: armed, the healthy rows of the
// healthy board AND of the damaged board still count, and the evidence names
// the board + file + 1-based line.
func TestUmbrellaSkipBadAggregatesHealthyBoards(t *testing.T) {
	umbrella, _, childB, badLine := df47Fixture(t)

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	um.SetSkipBad()
	us, err := um.Stats(TaskFilter{})
	if err != nil {
		t.Fatalf("degraded Stats returned an error: %v", err)
	}
	if us.Total != 3 {
		t.Fatalf("degraded total = %d, want 3 (childA 2 + childB 1; the bad line is not a row, the board is still read)", us.Total)
	}
	byRepo := map[string]int{}
	for _, rs := range us.Repos {
		byRepo[rs.Repo] = rs.Stats.Total
	}
	if byRepo["childA"] != 2 || byRepo["childB"] != 1 {
		t.Fatalf("per-repo totals = %v, want childA:2 childB:1", byRepo)
	}
	if !um.HasSkipped() {
		t.Fatal("degraded read recorded no skip evidence")
	}
	skips := um.SkippedLines()
	if len(skips) != 1 {
		t.Fatalf("skipped entries = %d, want 1: %+v", len(skips), skips)
	}
	s := skips[0]
	if s.Repo != "childB" {
		t.Fatalf("skipped entry repo = %q, want childB (provenance)", s.Repo)
	}
	if want := filepath.Join(childB, ".coding-hermes", "board"); s.BoardDir != want {
		t.Fatalf("skipped entry board dir = %q, want %q", s.BoardDir, want)
	}
	if want := filepath.Join(childB, ".coding-hermes", "board", "tasks.jsonl"); s.Line.Path != want {
		t.Fatalf("skipped entry file = %q, want %q", s.Line.Path, want)
	}
	if s.Line.Line != badLine {
		t.Fatalf("skipped entry line = %d, want the bad line %d", s.Line.Line, badLine)
	}
	if !strings.Contains(s.Line.Err, `duplicate key "worker_status"`) {
		t.Fatalf("skipped entry parse error = %q", s.Line.Err)
	}
	if !strings.Contains(s.Line.Snippet, "DF47-BAD") {
		t.Fatalf("skipped entry snippet = %q, want the raw line", s.Line.Snippet)
	}
	rendered := um.RenderSkippedLines()
	for _, want := range []string{"SKIPPED-LINES: 1", "childB", s.Line.Path, "line " + strconv.Itoa(badLine), "DF47-BAD"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("RenderSkippedLines missing %q:\n%s", want, rendered)
		}
	}
}

// TestUmbrellaSkipBadDedupesAcrossReads: --validate reads a board's tasks more
// than once (validate + depends_on classification), so the same physical bad
// line would otherwise be reported repeatedly. One line = one entry.
func TestUmbrellaSkipBadDedupesAcrossReads(t *testing.T) {
	umbrella, _, _, _ := df47Fixture(t)

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	um.SetSkipBad()
	if _, err := um.Stats(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := um.ListTasks(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := um.DepResolutions(); err != nil {
		t.Fatal(err)
	}
	if _, err := um.ValidateAll(); err != nil {
		t.Fatal(err)
	}
	if got := len(um.SkippedLines()); got != 1 {
		t.Fatalf("skipped entries after 4 aggregate reads = %d, want 1 (deduped on repo+file+line): %+v", got, um.SkippedLines())
	}
}

// TestUmbrellaSkipBadHealthyBoardsClean: arming the mode on a healthy view
// collects nothing (no false degradation).
func TestUmbrellaSkipBadHealthyBoardsClean(t *testing.T) {
	umbrella, childA, childB := umbFixture(t)
	umbSeedTask(t, childA, "CHILDA-1", "childA one")
	umbSeedTask(t, childB, "CHILDB-1", "childB one")

	um, err := OpenUmbrella(umbrella)
	if err != nil {
		t.Fatal(err)
	}
	um.SetSkipBad()
	if _, err := um.Stats(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := um.ListTasks(TaskFilter{}); err != nil {
		t.Fatal(err)
	}
	if um.HasSkipped() {
		t.Fatalf("healthy view reported skipped lines: %+v", um.SkippedLines())
	}
	if got := um.RenderSkippedLines(); got != "" {
		t.Fatalf("healthy view rendered a SKIPPED-LINES block: %q", got)
	}
}
