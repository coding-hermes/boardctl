package board

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-9: --skip-bad-lines degrade-with-evidence reads,
// and validate --repair to tasks.rewritten.jsonl ----------
//
// The fixture is the measured dogfood corruption shape (5 physical lines):
// a clean row, a truncated row, a row with an embedded raw newline (two
// failing fragments), and a clean row. Default reads must abort on the first
// bad line exactly as before; tolerant reads must salvage the clean rows and
// record per-line evidence; repair must write ONLY the salvageable rows to a
// review file without touching tasks.jsonl.

const (
	df9LineA     = `{"id": "DF9-A", "title": "clean alpha", "status": "pending", "priority": "P1"}`
	df9LineTrunc = `{"id": "DF9-BADTRUNC", "title": "trunca`
	df9LineNL1   = `{"id": "DF9-BADNL", "title": "first half`
	df9LineNL2   = `of a pasted row", "status": "pending"}`
	df9LineC     = `{"id": "DF9-C", "title": "clean gamma", "status": "complete", "priority": "P2"}`
)

// seedDF9Board writes the 5-line broken fixture as a topology-A board and
// resolves it. Every test that needs an independent evidence list calls this
// fresh — the skipped-line evidence accumulates on the Board instance.
func seedDF9Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": df9LineA + "\n" +
			df9LineTrunc + "\n" +
			df9LineNL1 + "\n" +
			df9LineNL2 + "\n" +
			df9LineC + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-04 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "df9", "namespace": "df9", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDF9DefaultReadAborts: without the flag, one bad line kills the whole
// read (status quo), and NO skip evidence is collected in default mode.
func TestDF9DefaultReadAborts(t *testing.T) {
	b := seedDF9Board(t)
	rows, err := b.TaskRows()
	if err == nil {
		t.Fatalf("default TaskRows on broken board = %d rows, want an abort error", len(rows))
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("default TaskRows err = %v, want line-2 context (first bad line)", err)
	}
	if b.HasSkipped() {
		t.Fatalf("default mode collected skip evidence: %+v", b.SkippedLines())
	}
	if _, _, err := ReadAllRows(b.tasksPath); err == nil {
		t.Fatal("default ReadAllRows on broken board = no error, want abort")
	}
}

// TestDF9SkipBadSalvagesAndRecordsEvidence: with the flag, the two clean rows
// survive in file order and every bad line (2, 3, 4) carries evidence with
// the 1-based physical line number, a snippet, and the underlying parse error.
func TestDF9SkipBadSalvagesAndRecordsEvidence(t *testing.T) {
	b := seedDF9Board(t)
	b.SkipBad = true
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatalf("tolerant TaskRows: %v", err)
	}
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.String("id"))
	}
	if got, want := strings.Join(ids, ","), "DF9-A,DF9-C"; got != want {
		t.Fatalf("tolerant TaskRows ids = %q, want %q (both clean rows, file order)", got, want)
	}
	skips := b.SkippedLines()
	if len(skips) != 3 {
		t.Fatalf("skipped evidence = %d entries, want 3 (lines 2, 3, 4): %+v", len(skips), skips)
	}
	for i, wantLine := range []int{2, 3, 4} {
		s := skips[i]
		if s.Line != wantLine {
			t.Errorf("skip[%d].Line = %d, want %d", i, s.Line, wantLine)
		}
		if s.Path != b.tasksPath {
			t.Errorf("skip[%d].Path = %q, want %q", i, s.Path, b.tasksPath)
		}
		if s.Snippet == "" {
			t.Errorf("skip[%d].Snippet empty", i)
		}
		if s.Err == "" {
			t.Errorf("skip[%d].Err empty — the underlying parse error must ride the evidence", i)
		}
	}
	if !strings.Contains(skips[0].Snippet, "DF9-BADTRUNC") {
		t.Errorf("skip[0].Snippet = %q, want the truncated row's head", skips[0].Snippet)
	}
	if !strings.Contains(skips[1].Snippet, "DF9-BADNL") || !strings.Contains(skips[2].Snippet, "of a pasted row") {
		t.Errorf("embedded-newline fragments not both recorded: %+v / %+v", skips[1], skips[2])
	}
	if !b.HasSkipped() {
		t.Fatal("HasSkipped = false with 3 recorded skips")
	}
	rendered := b.RenderSkippedLines()
	for _, want := range []string{
		"SKIPPED-LINES: 3",
		"tasks.jsonl line 2:",
		"tasks.jsonl line 3:",
		"tasks.jsonl line 4:",
		"DF9-BADTRUNC",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSkippedLines missing %q:\n%s", want, rendered)
		}
	}
}

// TestDF9SkipSnippetCappedAt120: the snippet contract is "first ~120 chars".
func TestDF9SkipSnippetCappedAt120(t *testing.T) {
	b := &Board{SkipBad: true}
	long := `{"id":"X","pad":"` + strings.Repeat("p", 500) + `"}`
	b.skipFailedLine("x.jsonl", 9, []byte(long), errors.New("boom"))
	s := b.SkippedLines()[0]
	if got := len([]rune(s.Snippet)); got != 121 { // 120 runes + the ellipsis
		t.Fatalf("snippet runes = %d, want 121 (120 + ellipsis)", got)
	}
	if !strings.HasSuffix(s.Snippet, "…") {
		t.Fatalf("capped snippet must end with the ellipsis, got %q", s.Snippet)
	}
}

// TestDF9SkipBadStatsAndList: stats/list salvage the countable rows tolerantly.
func TestDF9SkipBadStatsAndList(t *testing.T) {
	b := seedDF9Board(t)
	if _, err := b.ComputeStats(TaskFilter{}); err == nil {
		t.Fatal("default ComputeStats on broken board = no error, want abort")
	}
	b.SkipBad = true
	st, err := b.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatalf("tolerant ComputeStats: %v", err)
	}
	if st.Total != 2 {
		t.Fatalf("tolerant stats total = %d, want 2", st.Total)
	}
	if st.Status["pending"] != 1 || st.Status["complete"] != 1 {
		t.Fatalf("tolerant stats status = %v, want pending:1 complete:1", st.Status)
	}
	rows, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatalf("tolerant ListTasks: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("tolerant ListTasks = %d rows, want 2", len(rows))
	}
}

// TestDF9ValidateDefaultAndTolerant: default validate keeps its abort finding
// (unchanged); tolerant validate itemizes each skipped line as its own error
// finding AND counts the salvageable rows.
func TestDF9ValidateDefaultAndTolerant(t *testing.T) {
	b := seedDF9Board(t) // default mode
	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("default Validate returned error: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("default validate on broken board = OK, want FAIL; findings %+v", rep.Findings)
	}
	foundAbort := false
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "line 2") {
			foundAbort = true
		}
		if strings.Contains(f.Msg, "SKIPPED unparseable") {
			t.Fatalf("default mode emitted a SKIPPED finding: %s", f.Msg)
		}
	}
	if !foundAbort {
		t.Fatalf("default validate lost the line-2 abort finding: %+v", rep.Findings)
	}
	if rep.Tasks != 1 {
		t.Fatalf("default validate counted %d tasks, want 1 (line 1 validated before the abort)", rep.Tasks)
	}

	b2 := seedDF9Board(t)
	b2.SkipBad = true
	rep2, err := b2.Validate()
	if err != nil {
		t.Fatalf("tolerant Validate returned error: %v", err)
	}
	if !rep2.HasErrors() {
		t.Fatal("tolerant validate on a board with skipped lines must still FAIL")
	}
	skippedFindings := 0
	for _, f := range rep2.Findings {
		if strings.Contains(f.Msg, "SKIPPED unparseable") {
			skippedFindings++
		}
	}
	if skippedFindings != 3 {
		t.Fatalf("tolerant validate emitted %d SKIPPED findings, want 3:\n%+v", skippedFindings, rep2.Findings)
	}
	for _, want := range []string{"line 2", "line 3", "line 4"} {
		hit := false
		for _, f := range rep2.Findings {
			if strings.Contains(f.Msg, "SKIPPED unparseable") && strings.Contains(f.Msg, want) {
				hit = true
			}
		}
		if !hit {
			t.Errorf("no SKIPPED finding names %s:\n%+v", want, rep2.Findings)
		}
	}
	if rep2.Tasks != 2 {
		t.Fatalf("tolerant validate counted %d tasks, want 2 (the salvageable rows)", rep2.Tasks)
	}
}

// TestDF9RepairSalvagesToReviewFile: repair writes ONLY the salvageable rows
// to <boarddir>/tasks.rewritten.jsonl in the board's own style, reports
// salvaged/dropped counts with the dropped line numbers, and leaves
// tasks.jsonl (and every sibling) byte-identical.
func TestDF9RepairSalvagesToReviewFile(t *testing.T) {
	b := seedDF9Board(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}

	res, err := b.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if res.Salvaged != 2 {
		t.Fatalf("Salvaged = %d, want 2", res.Salvaged)
	}
	if res.Dropped != 3 {
		t.Fatalf("Dropped = %d, want 3", res.Dropped)
	}
	if got, want := res.DroppedLines, []int{2, 3, 4}; !intSliceEqual(got, want) {
		t.Fatalf("DroppedLines = %v, want %v", got, want)
	}
	if want := filepath.Join(b.Dir, "tasks.rewritten.jsonl"); res.OutputPath != want {
		t.Fatalf("OutputPath = %q, want %q", res.OutputPath, want)
	}

	// tasks.jsonl byte-identical after the repair; siblings untouched.
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("tasks.jsonl was modified by --repair — the review file is the only write")
	}
	eventsAfter, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatal("events.jsonl was modified by --repair")
	}

	// The review file carries exactly the salvageable rows, re-serialized in
	// the board's own (spaced, here) style: line 1 round-trips
	// byte-identically, and the last clean row serializes back to its exact
	// source bytes.
	data, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	var lines [][]byte
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("review file has %d lines, want exactly the 2 salvageable rows:\n%s", len(lines), data)
	}
	if !bytes.Equal(lines[0], []byte(df9LineA)) {
		t.Fatalf("review line 1 = %s, want byte-identical round-trip of the source row", lines[0])
	}
	if !bytes.Equal(lines[1], []byte(df9LineC)) {
		t.Fatalf("review line 2 = %s, want byte-identical re-serialization in the board's own style", lines[1])
	}
	// The review file must itself be clean JSONL.
	rows, raws, err := ReadAllRows(res.OutputPath)
	if err != nil {
		t.Fatalf("review file does not parse as JSONL: %v", err)
	}
	if len(rows) != 2 || len(raws) != 2 {
		t.Fatalf("review file parsed to %d rows, want 2", len(rows))
	}

	txt := res.RenderRepairText()
	for _, want := range []string{
		"2 salvaged row(s)",
		"tasks.rewritten.jsonl",
		"dropped 3 unparseable line(s)",
		"line(s): 2, 3, 4",
		"FOR MANUAL REVIEW",
		"NOT modified",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("repair summary missing %q:\n%s", want, txt)
		}
	}
}

// TestDF9RepairOnCleanBoardIsNoopRewrite: on a fully-parseable board the
// repair still writes the review file (idempotent salvage) and drops nothing.
func TestDF9RepairOnCleanBoardIsNoopRewrite(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  df9LineA + "\n" + df9LineC + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-04 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "df9", "namespace": "df9", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := b.Repair()
	if err != nil {
		t.Fatalf("Repair on clean board: %v", err)
	}
	if res.Salvaged != 2 || res.Dropped != 0 || len(res.DroppedLines) != 0 {
		t.Fatalf("clean-board repair = %+v, want 2 salvaged / 0 dropped", res)
	}
	data, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if want := df9LineA + "\n" + df9LineC + "\n"; string(data) != want {
		t.Fatalf("clean-board review file = %q, want byte-identical to tasks.jsonl content", data)
	}
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
