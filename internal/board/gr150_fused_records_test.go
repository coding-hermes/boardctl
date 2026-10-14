package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- GR-150: fused / multi-value JSONL board records ----------
//
// The committed GitReins board once carried 445 complete JSON objects
// concatenated on ONE physical line. Every line-based reader saw a single
// line: the strict path stopped at the FIRST object (encoding/json's Decoder
// decodes one value and reports no error for the rest), and the tolerant path
// dropped the whole line as unparseable — so `boardctl validate` and `list`
// reported 1 task and RESULT OK while 444 records were invisible.
//
// The invariants these tests pin:
//   - a physical line carries exactly ONE JSON value; anything after the
//     first object (a second value, a truncated second value, arbitrary
//     trailing bytes) is a parse ERROR naming the line and the problem;
//   - the tolerant scan counts such a line as SKIPPED-WITH-EVIDENCE, never as
//     one of the records it fused;
//   - validate FAILS (never RESULT OK) when any line was skipped or carries
//     trailing data;
//   - every writer path emits exactly one JSON object per physical line plus
//     a single final newline, including an append onto a file whose last line
//     lacks its terminator;
//   - the repair path REFUSES an ambiguous (fused/trailing) line instead of
//     normalizing it, and compares the row ids and event records before and
//     after any rewrite.

const (
	gr150RowA = `{"id":"GR150-A","title":"alpha","status":"pending","priority":"P1"}`
	gr150RowB = `{"id":"GR150-B","title":"beta","status":"pending","priority":"P2"}`
	gr150RowC = `{"id":"GR150-C","title":"gamma","status":"complete","priority":"P3"}`
	gr150Ev1  = `{"id":1,"timestamp":"2026-10-13 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}`
	gr150Ev2  = `{"id":2,"timestamp":"2026-10-13 00:01:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":2}`
	gr150Hdr  = `{"project":"gr150","namespace":"gr150","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}`
)

// seedGR150Board writes a topology-A board whose tasks.jsonl content is given
// verbatim (trailing newline or not) and resolves it.
func seedGR150Board(t *testing.T, tasks, events string) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  tasks,
		"events.jsonl": events,
		"board.jsonl":  gr150Hdr + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// assertJSONLInvariants is the shared writer-contract probe: every non-empty
// physical line carries exactly one complete JSON object, no line is blank,
// and the file ends with exactly one newline.
func assertJSONLInvariants(t *testing.T, path string, wantLines int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", path)
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		t.Fatalf("%s does not end with a newline (append would fuse the next row):\n%q", path, data)
	}
	if bytes.HasSuffix(data, []byte("\n\n")) {
		t.Fatalf("%s ends with more than one newline:\n%q", path, data)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != wantLines {
		t.Fatalf("%s has %d physical lines, want %d:\n%s", path, len(lines), wantLines, data)
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			t.Errorf("%s line %d is blank (writers emit one object per line)", path, i+1)
			continue
		}
		if _, err := ParseRow([]byte(line)); err != nil {
			t.Errorf("%s line %d is not exactly one JSON object: %v\n  line: %.120s", path, i+1, err, line)
		}
	}
}

func tasksPath(t *testing.T, b *Board) string { t.Helper(); return b.TasksPath() }

// TestGR150ParseRowRejectsMultiValueAndTrailingLines is the parse-layer table:
// the valid control parses, every fused/malformed line fails with the right
// sentinel class, and the pre-existing EOF class (a single object truncated
// mid-value) is NOT reclassified as a fused record.
func TestGR150ParseRowRejectsMultiValueAndTrailingLines(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		valid    bool
		sentinel error // when non-nil: errors.Is(err, sentinel) must hold
		notEOF   bool  // must NOT be the mixed-value/trailing class
		subs     []string
	}{
		{name: "valid control", line: gr150RowA, valid: true},
		{name: "valid control with trailing whitespace", line: gr150RowA + "  \t", valid: true},
		{
			name: "two objects fused with no separator", line: gr150RowA + gr150RowB,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values", "one JSON object per line"},
		},
		{
			name: "two objects fused space separated", line: gr150RowA + " " + gr150RowB,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values"},
		},
		{
			name: "three objects fused", line: gr150RowA + gr150RowB + gr150RowC,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values", "further data"},
		},
		{
			name: "fused event tail (two events on one line)", line: gr150Ev1 + gr150Ev2,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values"},
		},
		{
			name: "fused tail with a truncated second event", line: gr150Ev1 + `{"id":3,`,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values", "incomplete"},
		},
		{
			name: "malformed tail", line: gr150RowA + "oops",
			sentinel: ErrTrailingData, subs: []string{"trailing data", "not a second JSON value"},
		},
		{
			name: "malformed object tail", line: gr150RowA + "{oops}",
			sentinel: ErrTrailingData, subs: []string{"trailing data"},
		},
		{
			name: "unterminated tail", line: gr150RowA + " " + `{"id":"GR150-B"`,
			sentinel: ErrMultipleJSONValues, subs: []string{"multiple JSON values", "incomplete"},
		},
		{
			// The pre-existing truncation class: one object cut mid-value. It
			// keeps its own diagnostic (a bare decoder EOF, which IterParsed's
			// pretty-print special case keys on) — it is not a fused record.
			name: "truncated single object", line: `{"id":"GR150-A"`,
			notEOF: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row, err := ParseRow([]byte(tc.line))
			if tc.valid {
				if err != nil {
					t.Fatalf("ParseRow(%q) = %v, want a clean parse", tc.line, err)
				}
				if row == nil {
					t.Fatalf("ParseRow(%q) returned a nil row with no error", tc.line)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseRow(%q) accepted the line (row id %q) — want a rejection", tc.line, row.String("id"))
			}
			if tc.sentinel != nil && !errors.Is(err, tc.sentinel) {
				t.Errorf("ParseRow(%q) err = %v, want errors.Is(_, %v)", tc.line, err, tc.sentinel)
			}
			if tc.notEOF {
				if errors.Is(err, ErrMultipleJSONValues) || errors.Is(err, ErrTrailingData) {
					t.Errorf("truncated single object misclassified as a fused record: %v", err)
				}
			}
			for _, sub := range tc.subs {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("ParseRow(%q) err = %v, want it to mention %q", tc.line, err, sub)
				}
			}
		})
	}
}

// TestGR150ValidateRejectsFusedLine: a fused line is an ERROR finding naming
// the physical line and the problem — never RESULT OK — and only the records
// that actually stand alone are counted.
func TestGR150ValidateRejectsFusedLine(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n"+gr150RowB+gr150RowC+"\n", gr150Ev1+"\n")
	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("validate on a fused board = RESULT OK, want FAIL; findings %+v", rep.Findings)
	}
	var hit bool
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "line 2") && strings.Contains(f.Msg, "multiple JSON values") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("no finding names tasks.jsonl line 2 as multiple JSON values:\n%s", rep.RenderText())
	}
	if rep.Tasks != 1 {
		t.Fatalf("validate counted %d tasks, want 1 (the fused line carries no countable standalone record)", rep.Tasks)
	}

	// list / show / stats all refuse the same line rather than reporting a
	// smaller board.
	if _, err := b.TaskRows(); err == nil {
		t.Fatal("TaskRows on a fused board = no error (the fused line was read as one task)")
	} else if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("TaskRows err = %v, want file:line + the multi-value diagnosis", err)
	}
	if _, err := b.ListTasks(TaskFilter{}); err == nil {
		t.Fatal("ListTasks on a fused board = no error")
	} else if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("ListTasks err = %v, want the line number", err)
	}
	if _, _, err := b.ShowTask("GR150-B"); err == nil {
		t.Fatal("ShowTask on a fused board = no error")
	}
	if _, err := b.ComputeStats(TaskFilter{}); err == nil {
		t.Fatal("ComputeStats on a fused board = no error")
	}

	// control: the same rows on their own lines validate clean.
	clean := seedGR150Board(t, gr150RowA+"\n"+gr150RowB+"\n"+gr150RowC+"\n", gr150Ev1+"\n")
	crep, err := clean.Validate()
	if err != nil {
		t.Fatalf("Validate(clean): %v", err)
	}
	if crep.HasErrors() {
		t.Fatalf("clean control board FAILED validation:\n%s", crep.RenderText())
	}
	if crep.Tasks != 3 {
		t.Fatalf("clean control counted %d tasks, want 3", crep.Tasks)
	}
}

// TestGR150ValidateRejectsTrailingDataLine: "{...}oops" is the other rejection
// class — same file:line diagnostic, same FAIL.
func TestGR150ValidateRejectsTrailingDataLine(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n"+"\n"+gr150RowB+"oops"+"\n", gr150Ev1+"\n")
	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("validate on a trailing-data board = RESULT OK, want FAIL:\n%s", rep.RenderText())
	}
	var hit bool
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "line 3") && strings.Contains(f.Msg, "trailing data") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("no finding names tasks.jsonl line 3 as trailing data:\n%s", rep.RenderText())
	}
}

// TestGR150ValidateRejectsFusedEventLine: the event stream is held to the same
// invariant — a fused events.jsonl line fails validate.
func TestGR150ValidateRejectsFusedEventLine(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n", gr150Ev1+gr150Ev2+"\n")
	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("validate on a fused events.jsonl = RESULT OK, want FAIL:\n%s", rep.RenderText())
	}
	var hit bool
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "events.jsonl") && strings.Contains(f.Msg, "line 1") && strings.Contains(f.Msg, "multiple JSON values") {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("no finding names events.jsonl line 1 as multiple JSON values:\n%s", rep.RenderText())
	}
	if rep.Events != 0 {
		t.Fatalf("validate counted %d events on a fused events line, want 0", rep.Events)
	}
}

// TestGR150TolerantScanCountsFusedLineAsSkipped: the tolerant reader records
// the fused line as skipped-WITH-EVIDENCE (naming the line, the snippet and
// the multi-value failure). It never silently reports the fused records as
// missing: HasSkipped() is the signal the CLI turns into a non-zero exit, and
// validate itemizes a SKIPPED finding.
func TestGR150TolerantScanCountsFusedLineAsSkipped(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n"+gr150RowB+gr150RowC+"\n", gr150Ev1+"\n")
	b.SkipBad = true
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatalf("tolerant TaskRows: %v", err)
	}
	if len(rows) != 1 || rows[0].String("id") != "GR150-A" {
		t.Fatalf("tolerant TaskRows = %d rows (%v), want only the standalone GR150-A row", len(rows), rows)
	}
	if !b.HasSkipped() {
		t.Fatal("tolerant read of a fused board reported NO skipped evidence — the 2 fused records vanished silently")
	}
	skips := b.SkippedLines()
	if len(skips) != 1 {
		t.Fatalf("skipped evidence = %d entries, want 1: %+v", len(skips), skips)
	}
	s := skips[0]
	if s.Line != 2 || s.Path != b.TasksPath() {
		t.Errorf("skip evidence = %s line %d, want %s line 2", s.Path, s.Line, b.TasksPath())
	}
	if !strings.Contains(s.Err, "multiple JSON values") {
		t.Errorf("skip evidence err = %q, want the multi-value failure named", s.Err)
	}
	if !strings.Contains(s.Snippet, "GR150-B") {
		t.Errorf("skip evidence snippet = %q, want the fused line's bytes", s.Snippet)
	}
	rendered := b.RenderSkippedLines()
	for _, want := range []string{"SKIPPED-LINES: 1", "tasks.jsonl line 2", "multiple JSON values"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("RenderSkippedLines missing %q:\n%s", want, rendered)
		}
	}

	lrows, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatalf("tolerant ListTasks: %v", err)
	}
	if len(lrows) != 1 || !b.HasSkipped() {
		t.Fatalf("tolerant list reported %d rows with HasSkipped=%v — a degraded read must carry evidence, not a quietly smaller board", len(lrows), b.HasSkipped())
	}

	// validate (fresh board instance: skip evidence accumulates per Board, and
	// the assertion below counts the findings for EXACTLY one skipped line).
	vb := seedGR150Board(t, gr150RowA+"\n"+gr150RowB+gr150RowC+"\n", gr150Ev1+"\n")
	vb.SkipBad = true
	rep, err := vb.Validate()
	if err != nil {
		t.Fatalf("tolerant Validate: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("tolerant validate on a fused board = RESULT OK, want FAIL:\n%s", rep.RenderText())
	}
	var skipped int
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "SKIPPED unparseable") {
			skipped++
			if !strings.Contains(f.Msg, "line 2") || !strings.Contains(f.Msg, "multiple JSON values") {
				t.Errorf("SKIPPED finding does not name line 2 + the problem: %s", f.Msg)
			}
		}
	}
	if skipped != 1 {
		t.Fatalf("tolerant validate emitted %d SKIPPED findings, want 1:\n%s", skipped, rep.RenderText())
	}
}

// TestGR150CreateAppendsOntoUnterminatedFile: the writer path terminates the
// previous line before appending, so the new row never fuses onto it — and the
// pre-existing (unterminated) line is preserved byte-for-byte.
func TestGR150CreateAppendsOntoUnterminatedFile(t *testing.T) {
	b := seedGR150Board(t, gr150RowA, gr150Ev1+"\n") // tasks.jsonl has NO final newline
	if _, err := b.Create(TaskRowSpec{ID: "GR150-NEW", Title: "new row", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("Create onto an unterminated file: %v", err)
	}
	path := tasksPath(t, b)
	assertJSONLInvariants(t, path, 2)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), gr150RowA+"\n") {
		t.Fatalf("the pre-existing line changed or stayed unterminated:\n%q", data)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatalf("TaskRows after append: %v", err)
	}
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.String("id"))
	}
	if got := strings.Join(ids, ","); got != "GR150-A,GR150-NEW" {
		t.Fatalf("ids = %q, want both rows readable (the fused append would have hidden one)", got)
	}
}

// TestGR150EventAppendOntoUnterminatedFile: same contract for the append-only
// event stream.
func TestGR150EventAppendOntoUnterminatedFile(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n", gr150Ev1) // events.jsonl has NO final newline
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman"}); err != nil {
		t.Fatalf("AppendEvent onto an unterminated events.jsonl: %v", err)
	}
	assertJSONLInvariants(t, b.EventsPath(), 2)
	rows, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatalf("ReadAllRows(events): %v", err)
	}
	if len(rows) != 2 || rows[1].IntOrZero("id") != 2 {
		t.Fatalf("events after append = %d rows, last id %d; want 2 rows ending at id 2", len(rows), rows[len(rows)-1].IntOrZero("id"))
	}
}

// TestGR150UpdateKeepsOneObjectPerLineAndFinalNewline: the rewriting writers
// normalize the terminator too — an update on a file whose last line lacked
// its newline leaves a valid single-object-per-line file with exactly one
// final newline, and every untouched line byte-identical.
func TestGR150UpdateKeepsOneObjectPerLineAndFinalNewline(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n"+gr150RowB, gr150Ev1+"\n") // last line unterminated
	status := "complete"
	if _, err := b.UpdateTask("GR150-B", UpdateSpec{Status: &status}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	assertJSONLInvariants(t, tasksPath(t, b), 2)
	data, err := os.ReadFile(tasksPath(t, b))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), gr150RowA+"\n") {
		t.Fatalf("the untouched first line was rewritten:\n%q", data)
	}
}

// TestGR150AppendRefusesMultiValuePayload: the append primitives are the last
// line of defence — a payload that would fuse two records onto one line (or is
// not one JSON object at all) is refused before anything is written.
func TestGR150AppendRefusesMultiValuePayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.jsonl")
	if err := os.WriteFile(path, []byte(gr150RowA+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"two records in one payload", gr150RowB + gr150RowC + "\n", "one JSON object per line"},
		{"missing terminator", gr150RowB, "newline"},
		{"two terminators", gr150RowB + "\n\n", "newline"},
		{"not JSON", "oops\n", "one JSON object per line"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := appendBytes(path, []byte(tc.payload))
			if err == nil {
				t.Fatalf("appendBytes(%q) accepted a payload that breaks the one-object-per-line invariant", tc.payload)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("appendBytes(%q) err = %v, want it to mention %q", tc.payload, err, tc.want)
			}
		})
	}
	// Valid payload still appends, and the refused writes left the file alone.
	if err := appendBytes(path, []byte(gr150RowB+"\n")); err != nil {
		t.Fatalf("appendBytes(valid): %v", err)
	}
	assertJSONLInvariants(t, path, 2)
}

// TestGR150RepairRefusesAmbiguousLines: before any normalization the repair
// compares the source's records; a fused/trailing line is refused (it holds
// more than one record's data, so re-serializing it would silently drop the
// remainder), nothing is written, and tasks.jsonl stays byte-identical.
func TestGR150RepairRefusesAmbiguousLines(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n"+gr150RowB+gr150RowC+"\n", gr150Ev1+"\n")
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	res, err := b.Repair()
	if err == nil {
		t.Fatalf("Repair on a fused board succeeded (%+v) — an ambiguous stream must refuse repair", res)
	}
	if !strings.Contains(err.Error(), "line 2") || !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("Repair err = %v, want it to name line 2 and the multi-value problem", err)
	}
	if !strings.Contains(err.Error(), "refus") {
		t.Errorf("Repair err = %v, want an explicit refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(b.Dir, "tasks.rewritten.jsonl")); statErr == nil {
		t.Error("refused repair still wrote tasks.rewritten.jsonl")
	}
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("refused repair modified tasks.jsonl")
	}
}

// TestGR150RepairSalvagesAndVerifiesIDsBeforeAfter: on an unambiguous broken
// board the repair still salvages, and the review file is verified to carry
// exactly the source's row-id stream (order and duplicates included) with the
// event records untouched.
func TestGR150RepairSalvagesAndVerifiesIDsBeforeAfter(t *testing.T) {
	b := seedGR150Board(t,
		gr150RowA+"\n"+`{"id":"GR150-TRUNC","title":"trunca`+"\n"+gr150RowC+"\n",
		gr150Ev1+"\n"+gr150Ev2+"\n")
	eventsBefore, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	res, err := b.Repair()
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if res.Salvaged != 2 || res.Dropped != 1 {
		t.Fatalf("repair salvaged/dropped = %d/%d, want 2/1", res.Salvaged, res.Dropped)
	}
	lines, err := ReadJSONLLines(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			t.Fatalf("review file line %d is not exactly one JSON object: %v", i+1, err)
		}
		ids = append(ids, row.String("id"))
	}
	if got := strings.Join(ids, ","); got != "GR150-A,GR150-C" {
		t.Fatalf("review file ids = %q, want the source's parseable ids in order", got)
	}
	assertJSONLInvariants(t, res.OutputPath, 2)
	eventsAfter, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatal("repair modified events.jsonl")
	}
}

// TestGR150FusedDetectionIsSemantic: the classifier used by the repair path
// must key on "more than one JSON value / trailing data", not on any parse
// failure — a truncated or embedded-newline fragment is a DIFFERENT class and
// stays repairable.
func TestGR150FusedDetectionIsSemantic(t *testing.T) {
	lines := [][]byte{
		[]byte(gr150RowA),
		[]byte(gr150RowB + gr150RowC),
		[]byte(`{"id":"X","title":"first half`),
		[]byte(`of a pasted row","status":"pending"}`),
		[]byte(gr150RowA + "oops"),
		[]byte(""),
	}
	got := fusedLineNumbers(lines)
	want := []int{2, 5}
	if len(got) != len(want) {
		t.Fatalf("fusedLineNumbers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fusedLineNumbers = %v, want %v", got, want)
		}
	}
}

// TestGR150CompactRoundTripStaysSingleLine guards the writer's own JSON shape:
// whatever the styles/marshalers produce for a row (including an escaped
// newline inside a value) must remain ONE physical line.
func TestGR150CompactRoundTripStaysSingleLine(t *testing.T) {
	b := seedGR150Board(t, gr150RowA+"\n", gr150Ev1+"\n")
	note := "line one\nline two"
	if _, err := b.UpdateTask("GR150-A", UpdateSpec{Note: &note}); err != nil {
		t.Fatalf("UpdateTask with a multi-line note: %v", err)
	}
	assertJSONLInvariants(t, tasksPath(t, b), 1)
	data, err := os.ReadFile(tasksPath(t, b))
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &parsed); err != nil {
		t.Fatalf("rewritten row does not parse: %v", err)
	}
	if parsed["foreman_note"] != note {
		t.Fatalf("foreman_note = %q, want the raw multi-line value", parsed["foreman_note"])
	}
}
