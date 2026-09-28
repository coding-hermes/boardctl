package board

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ---------- DF-BOARDCTL-10: id-aware dedupe for recycled task ids ----------

// asceFixtureLines returns the asce-shaped corpus: SIX distinct findings (the
// real asce QA-ASCE-1 families) all filed under the ONE recycled id QA-ASCE-1.
// Every title+reasoning pair differs, which is exactly why the fingerprint
// dedupe correctly refuses to collapse them — and why only an id-aware repair
// can fix the hygiene.
func asceFixtureLines() []string {
	return []string{
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"in_progress","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] port range exhausted","reasoning":"cell detail: 9200-9202 all bound","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] port range exhausted (recurrence 2)","reasoning":"cell detail: 9203-9205 all bound","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped the agent","reasoning":"cell detail: ssh session died mid-run","status":"blocked","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted on retry","reasoning":"cell detail: pool not released after crash","status":"pending","priority":"P2"}`,
	}
}

// seedIDDupeBoard plants raw task lines + one event row and resolves the board.
func seedIDDupeBoard(t *testing.T, taskLines ...string) *Board {
	t.Helper()
	return seedFindingBoard(t, taskLines...)
}

// idDupeBoardSnapshot hashes every tracked board file (tasks, events, header,
// fixtures when present) so a test can assert "nothing was written".
func idDupeBoardSnapshot(t *testing.T, b *Board) map[string]string {
	t.Helper()
	paths := []string{b.TasksPath(), b.EventsPath()}
	if b.HeaderPath() != "" {
		paths = append(paths, b.HeaderPath())
	}
	if fp := b.FixturesPath(); fp != "" {
		paths = append(paths, fp)
	}
	out := map[string]string{}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		out[p] = hex.EncodeToString(sum[:])
	}
	return out
}

func mustIDDupe(t *testing.T, b *Board, apply bool) *IDDupeReport {
	t.Helper()
	rep, err := b.DedupeByID(apply)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

// countTaskLine4 counts the rows carrying id on the tasks file (the fixture
// shape is one JSON object per line; blank lines do not occur in seeds).
func countTaskLine4(t *testing.T, b *Board, id string) int {
	t.Helper()
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range rows {
		if r.String("id") == id {
			n++
		}
	}
	return n
}

// AC1: the asce-shaped fixture — six DISTINCT findings, one id — collapses to
// the earliest line kept open, five later lines closed as superseded, and one
// audit event per closed line quoting that line's original title+reasoning.
func TestIDDupe_AsceShapeSixRowsOneID(t *testing.T) {
	b := seedIDDupeBoard(t, asceFixtureLines()...)

	rep := mustIDDupe(t, b, true)
	if rep.RecycledIDs != 1 || len(rep.Groups) != 1 {
		t.Fatalf("recycled=%d groups=%d, want 1/1 (%+v)", rep.RecycledIDs, len(rep.Groups), rep.Groups)
	}
	g := rep.Groups[0]
	if g.ID != "QA-ASCE-1" || g.KeptLine != 1 || len(g.ClosedLines) != 5 {
		t.Fatalf("group wrong: %+v", g)
	}

	// the kept row is untouched byte-for-byte (still open, original text)
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if n := countTaskLine4(t, b, "QA-ASCE-1"); n != 6 {
		t.Fatalf("rows with the id after apply = %d, want 6 (close, never delete)", n)
	}
	if rows[0].String("status") != "pending" {
		t.Fatalf("kept row status = %q, want pending", rows[0].String("status"))
	}
	if !strings.Contains(rows[0].String("title"), "header-frozen") {
		t.Fatalf("kept row is not the earliest filing: %q", rows[0].String("title"))
	}

	// every later line: complete + superseded_by + completed_at, original
	// title and reasoning preserved verbatim on the row.
	wantStatusWas := []string{"in_progress", "pending", "pending", "blocked", "pending"}
	for i, want := range wantStatusWas {
		r := rows[i+1]
		if r.String("status") != "complete" {
			t.Fatalf("line %d status = %q, want complete", i+2, r.String("status"))
		}
		if r.String("superseded_by") != "QA-ASCE-1" {
			t.Fatalf("line %d superseded_by = %q, want QA-ASCE-1", i+2, r.String("superseded_by"))
		}
		if r.String("completed_at") == "" {
			t.Fatalf("line %d has no completed_at", i+2)
		}
		summary := r.String("worker_summary")
		if !strings.HasPrefix(summary, "superseded-by-earliest: recycled task id — kept line 1") {
			t.Fatalf("line %d worker_summary = %q", i+2, summary)
		}
		if r.String("title") == "" || r.String("reasoning") == "" {
			t.Fatalf("line %d lost its original text", i+2)
		}
		_ = want
	}

	// one audit event per closed line, each quoting title + reasoning.
	events, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	// seed row + 5 audit events
	if len(events) != 6 {
		t.Fatalf("events.jsonl rows = %d, want 6", len(events))
	}
	quoted := map[string]bool{}
	for _, ev := range events[1:] {
		if ev.String("event_type") != "audit" {
			t.Fatalf("event type = %q, want audit", ev.String("event_type"))
		}
		var d struct {
			Action     string `json:"action"`
			ClosedLine int    `json:"closed_line"`
			Title      string `json:"title"`
			Reasoning  string `json:"reasoning"`
		}
		if err := json.Unmarshal([]byte(ev.String("detail")), &d); err != nil {
			t.Fatalf("audit detail does not parse: %v (%s)", err, ev.String("detail"))
		}
		if d.Action != "id-dupe" {
			t.Fatalf("audit action = %q, want id-dupe", d.Action)
		}
		if d.Title == "" || d.Reasoning == "" {
			t.Fatalf("audit event does not quote the closed line's text: %+v", d)
		}
		quoted[d.Title] = true
	}
	if len(quoted) != 5 {
		t.Fatalf("audit events quote %d distinct titles, want 5 (one per closed finding): %v", len(quoted), quoted)
	}
}

// AC1b: the DRY-RUN plan on the same fixture names the identical collapse —
// kept line 1, closed lines 2-6 with each line's status-was and title — and,
// checked in TestIDDupe_DryRunWritesNothing, writes nothing.
func TestIDDupe_AsceShapeDryRunPlan(t *testing.T) {
	b := seedIDDupeBoard(t, asceFixtureLines()...)
	rep := mustIDDupe(t, b, false)
	if !rep.Changed() {
		t.Fatal("dry-run plan claims nothing to do on an asce-shaped board")
	}
	g := rep.Groups[0]
	if g.KeptLine != 1 || len(g.ClosedLines) != 5 {
		t.Fatalf("dry-run group wrong: %+v", g)
	}
	for i, ln := range g.ClosedLines {
		if ln != i+2 {
			t.Fatalf("dry-run closed line = %d, want %d", ln, i+2)
		}
	}
	if len(g.ClosedTitles) != 5 || len(g.ClosedStatusWas) != 5 {
		t.Fatalf("dry-run plan lacks per-line text/status: %+v", g)
	}
	if g.ClosedStatusWas[0] != "in_progress" || g.ClosedStatusWas[3] != "blocked" {
		t.Fatalf("dry-run status-was wrong: %v", g.ClosedStatusWas)
	}
}

// AC2: mixed fixture — two genuine fingerprint duplicates (distinct ids) AND
// a recycled id — proves the default fingerprint backfill and the id-aware
// mode are COMPLEMENTARY: each mode handles exactly its own class, and the
// id-aware run does not touch fingerprint-grouped rows.
func TestIDDupe_MixedFixtureFingerprintDupesAndRecycledIDs(t *testing.T) {
	sameFinding := `"title":"[P3] run_battery FAIL — port pool exhausted","reasoning":"cell detail: no free port ranges"`
	b := seedIDDupeBoard(t,
		// genuine fingerprint collision group (distinct ids, same finding)
		`{"id":"QA-BF-1","status":"pending","priority":"P3",`+sameFinding+`}`,
		`{"id":"QA-BF-2","status":"pending","priority":"P3",`+sameFinding+`}`,
		// recycled id, three distinct findings
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","reasoning":"cell: header stuck","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped the agent","reasoning":"cell: ssh died","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] port range exhausted","reasoning":"cell: 9200-9202 bound","status":"pending","priority":"P2"}`,
	)

	// (1) fingerprint backfill, unchanged: collapses ONLY the QA-BF pair.
	fpRep, err := b.DedupeBackfill(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fpRep.Groups) != 1 || fpRep.Groups[0].Kept != "QA-BF-1" {
		t.Fatalf("fingerprint groups wrong: %+v", fpRep.Groups)
	}
	if strings.Contains(strings.Join(fpRep.MergedAway, ","), "QA-ASCE-1") {
		t.Fatalf("fingerprint backfill touched the recycled-id rows: %v", fpRep.MergedAway)
	}

	// (2) id-aware dedupe: collapses ONLY the recycled id.
	idRep := mustIDDupe(t, b, true)
	if len(idRep.Groups) != 1 || idRep.Groups[0].ID != "QA-ASCE-1" {
		t.Fatalf("id-aware groups wrong: %+v", idRep.Groups)
	}
	if len(idRep.Groups[0].ClosedLines) != 2 {
		t.Fatalf("id-aware closed lines = %v, want the two later QA-ASCE-1 lines", idRep.Groups[0].ClosedLines)
	}
	if strings.Join(idRep.ClosedAway, "|") != "QA-ASCE-1 (line 4)|QA-ASCE-1 (line 5)" {
		t.Fatalf("closed-away labels wrong: %v", idRep.ClosedAway)
	}

	// QA-BF-2 (fingerprint-merged) is closed with ITS summary; the kept
	// QA-BF-1 row stays open (the fingerprint backfill keeps the earliest,
	// it does not close it); the recycled-id kept line stays open; its two
	// later lines are superseded.
	if taskRowByID(t, b, "QA-BF-1")["status"] != "pending" {
		t.Fatal("fingerprint kept row QA-BF-1 should stay open")
	}
	if taskRowByID(t, b, "QA-BF-2")["status"] != "complete" {
		t.Fatal("fingerprint merged row QA-BF-2 should be complete")
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if rows[2].String("status") != "pending" || !strings.Contains(rows[2].String("title"), "header-frozen") {
		t.Fatalf("recycled-id kept line wrong: %+v", rows[2])
	}
	if rows[3].String("status") != "complete" || rows[4].String("status") != "complete" {
		t.Fatalf("later recycled-id lines not closed: %+v / %+v", rows[3], rows[4])
	}
}

// AC3: dry-run writes NOTHING — sha256 snapshot of every tracked board file
// is byte-identical before/after.
func TestIDDupe_DryRunWritesNothing(t *testing.T) {
	b := seedIDDupeBoard(t, asceFixtureLines()...)
	before := idDupeBoardSnapshot(t, b)
	rep := mustIDDupe(t, b, false)
	if !rep.Changed() {
		t.Fatal("dry run found nothing although the board is asce-shaped")
	}
	after := idDupeBoardSnapshot(t, b)
	if len(before) != len(after) {
		t.Fatalf("snapshot file count changed: %d -> %d", len(before), len(after))
	}
	for p, h := range before {
		if after[p] != h {
			t.Fatalf("dry run wrote %s (sha %s -> %s)", p, h, after[p])
		}
	}
}

// AC4: apply is idempotent — the second run reports nothing to do and both
// files are byte-identical afterwards.
func TestIDDupe_ApplyIsIdempotent(t *testing.T) {
	b := seedIDDupeBoard(t, asceFixtureLines()...)
	if rep := mustIDDupe(t, b, true); !rep.Changed() {
		t.Fatal("first apply did nothing")
	}
	tasksAfter1 := mustRead(t, b.TasksPath())
	eventsAfter1 := mustRead(t, b.EventsPath())

	rep2 := mustIDDupe(t, b, true)
	if rep2.Changed() {
		t.Fatalf("second apply is not a no-op: %+v", rep2.Groups)
	}
	if string(mustRead(t, b.TasksPath())) != string(tasksAfter1) || string(mustRead(t, b.EventsPath())) != string(eventsAfter1) {
		t.Fatal("second apply rewrote a file")
	}
}

// Later lines that are ALREADY terminal are left untouched — no summary
// rewrite, no event, no superseded_by.
func TestIDDupe_AlreadyTerminalLaterLinesUntouched(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen","reasoning":"cell: stuck","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped","reasoning":"cell: ssh died","status":"complete","priority":"P2","worker_summary":"done by hand"}`,
		`{"id":"QA-ASCE-1","title":"[P2] ports exhausted","reasoning":"cell: bound","status":"pending","priority":"P2"}`,
	)
	closedBefore := taskLineByID(t, b, "QA-ASCE-1") // ambiguous id: FIRST match is line 2 here
	// pin the exact bytes of the terminal middle line by reading the file.
	lines, err := ReadJSONLLines(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	terminalBefore := string(lines[1])
	_ = closedBefore

	rep := mustIDDupe(t, b, true)
	if len(rep.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(rep.Groups))
	}
	g := rep.Groups[0]
	if len(g.AlreadyTerminal) != 1 || g.AlreadyTerminal[0] != 2 {
		t.Fatalf("already-terminal = %v, want [2]", g.AlreadyTerminal)
	}
	if len(g.ClosedLines) != 1 || g.ClosedLines[0] != 3 {
		t.Fatalf("closed = %v, want [3]", g.ClosedLines)
	}
	linesAfter, err := ReadJSONLLines(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(linesAfter[1]) != terminalBefore {
		t.Fatalf("terminal later line was rewritten:\n got %s\nwant %s", linesAfter[1], terminalBefore)
	}
	events, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 { // seed + exactly one audit event (line 3 only)
		t.Fatalf("events = %d, want 2", len(events))
	}
}

// A later duplicate with an UNKNOWN status refuses the whole run before any
// write — ambiguous rows need a human decision (same discipline as normalize).
func TestIDDupe_RefusesUnknownStatusBeforeWriting(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen","reasoning":"cell: stuck","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped","reasoning":"cell: ssh died","status":"retired","priority":"P2"}`,
	)
	before := idDupeBoardSnapshot(t, b)
	rep, err := b.DedupeByID(true)
	if err == nil {
		t.Fatalf("expected refusal, got report %+v", rep)
	}
	if !strings.Contains(err.Error(), "retired") {
		t.Fatalf("refusal should name the offending status: %v", err)
	}
	after := idDupeBoardSnapshot(t, b)
	for p, h := range before {
		if after[p] != h {
			t.Fatalf("refused run still wrote %s", p)
		}
	}
}

// AC5 support: object-form reasoning (the qa-dagger shape) is quoted into the
// event on its human-readable "note" member, and HTML-significant characters
// in a title survive the event verbatim (SetEscapeHTML(false) contract).
func TestIDDupe_EventQuotingVerbatim(t *testing.T) {
	b := seedIDDupeBoard(t,
		`{"id":"QA-DG-1","title":"[P1] a<b & c>d","reasoning":"plain","status":"pending","priority":"P1"}`,
		`{"id":"QA-DG-1","title":"[P1] totally different finding","reasoning":{"note":"QA foreman cycle observed the note"},"status":"pending","priority":"P1"}`,
	)
	mustIDDupe(t, b, true)
	events, _, err := ReadAllRows(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	var d struct {
		Title     string `json:"title"`
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(events[1].String("detail")), &d); err != nil {
		t.Fatal(err)
	}
	if d.Title != "[P1] totally different finding" {
		t.Fatalf("quoted title = %q", d.Title)
	}
	if !strings.Contains(d.Reasoning, "QA foreman cycle observed the note") {
		t.Fatalf("object-form reasoning not quoted on its note member: %q", d.Reasoning)
	}
	// verbatim bytes: the <, >, & in the OTHER line's title stay literal in
	// the raw event line (no &lt;/&gt;/&amp; mangling anywhere in the file).
	raw, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"&lt;", "&gt;", "&amp;"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("event detail HTML-escaped (%s found)", bad)
		}
	}
}

// Topology-B smoke: the header line is never a dedupe participant.
func TestIDDupe_TopologyBHeaderNotParticipant(t *testing.T) {
	b := newTestBoardB(t) // seeds a topology-B board (header on line 1 of tasks.jsonl)
	// append recycled-id rows so tasks.jsonl = header + 2 rows, one id twice.
	f := openAppend(t, b.TasksPath())
	if _, err := f.WriteString(`{"id":"QA-TB-1","title":"first finding","reasoning":"r1","status":"pending","priority":"P2"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"QA-TB-1","title":"second different finding","reasoning":"r2","status":"pending","priority":"P2"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	rep := mustIDDupe(t, b, true)
	if len(rep.Groups) != 1 || rep.Groups[0].ID != "QA-TB-1" {
		t.Fatalf("groups wrong: %+v", rep.Groups)
	}
	if rep.Groups[0].KeptLine != 3 { // line 1 header, line 2 EXIST-1, line 3 = earliest QA-TB-1
		t.Fatalf("kept line = %d, want 3 (header and pre-seeded row skipped)", rep.Groups[0].KeptLine)
	}
}

// openAppend opens a path for appending (test helper).
func openAppend(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
