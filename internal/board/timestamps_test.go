package board

import (
	"strings"
	"testing"
)

// DF-BOARDCTL-27: validate must carry the SAME timestamp integrity verdict
// the report parser applies. The repo's own board carried created_at values
// in a double-dash dialect ("2026-09-09-22T15:13:00Z") and rows with NO
// created_at at all; validate passed them silently (exit 0, no warning)
// while internal/render's report excluded those rows from time-based
// metrics. These tests hold TimestampFieldFindings (wired into validate) to
// the report's grammar AND its per-field exclusion rules.

// df27TimestampJunkBoard seeds one board with the three offending shapes and
// one fully valid control row:
//   - DF27-DD: created_at in the double-dash dialect (RELEASE-04/05 shape)
//   - DF27-NOBIRTH: no created_at key at all (DF-BOARDCTL-24/25 shape)
//   - DF27-EMPTY: created_at present but "" (the empty-string dialect)
//   - DF27-OK: every stamp parseable (must never be flagged)
func df27TimestampJunkBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"DF27-DD","title":"double-dash birth","status":"pending","priority":"P1","created_at":"2026-09-09-22T15:13:00Z"}` + "\n" +
			`{"id":"DF27-NOBIRTH","title":"no created_at key","status":"complete","priority":"P1"}` + "\n" +
			`{"id":"DF27-EMPTY","title":"empty created_at","status":"pending","priority":"P2","created_at":"","completed_at":"2026-10-01 10:00:00"}` + "\n" +
			`{"id":"DF27-OK","title":"valid stamps","status":"pending","priority":"P2","created_at":"2026-09-01 00:00:00","completed_at":"2026-10-02T04:01:03","updated_at":"2026-10-02 05:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df27","namespace":"df27","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// df27CountTimestampWarns counts warn findings carrying the fixed
// "time-based metrics" clause of the DF-BOARDCTL-27 warning.
func df27CountTimestampWarns(rep *Report) int {
	n := 0
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "time-based metrics") {
			n++
		}
	}
	return n
}

// TestDFBoardctl27WarnsOnUnparseableCreatedAndCompleted: the three offending
// rows each get exactly one warning naming id, field and raw value; the
// valid control row gets none; the board still validates OK (exit 0).
func TestDFBoardctl27WarnsOnUnparseableCreatedAndCompleted(t *testing.T) {
	b := df27TimestampJunkBoard(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("timestamp defects are warnings, not errors: %+v", rep.Findings)
	}
	if got := df27CountTimestampWarns(rep); got != 3 {
		t.Fatalf("timestamp warnings = %d, want 3; findings: %+v", got, rep.Findings)
	}
	// DF27-DD: the double-dash dialect, named with its raw value.
	got := findWarn(rep, "DF27-DD")
	if got == "" || !strings.Contains(got, "created_at") ||
		!strings.Contains(got, `unparseable "2026-09-09-22T15:13:00Z"`) {
		t.Fatalf("double-dash created_at not flagged with id+field+value: %q", got)
	}
	// DF27-NOBIRTH: absent key, named as absent.
	got = findWarn(rep, "DF27-NOBIRTH")
	if got == "" || !strings.Contains(got, "created_at absent") {
		t.Fatalf("missing created_at not flagged as absent: %q", got)
	}
	// DF27-EMPTY: empty string named with its value.
	got = findWarn(rep, "DF27-EMPTY")
	if got == "" || !strings.Contains(got, "created_at") || !strings.Contains(got, `""`) {
		t.Fatalf("empty-string created_at not flagged with id+field+value: %q", got)
	}
	// every warning names the fix hint for its field
	for _, f := range rep.Findings {
		if !strings.Contains(f.Msg, "time-based metrics") {
			continue
		}
		if !strings.Contains(f.Msg, "hand-edit the row") && !strings.Contains(f.Msg, "--completed-at") {
			t.Fatalf("timestamp warning carries no fix hint: %s", f.Msg)
		}
	}
	// the control row is clean
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "DF27-OK") && strings.Contains(f.Msg, "time-based metrics") {
			t.Fatalf("valid row DF27-OK flagged: %s", f.Msg)
		}
	}
}

// TestDFBoardctl27CleanBoardZeroTimestampWarns: a board whose rows all carry
// parseable stamps (the write paths' own output shapes) yields ZERO
// timestamp warnings — the check must not cry wolf on healthy rows.
func TestDFBoardctl27CleanBoardZeroTimestampWarns(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"CLEAN-A","title":"space naive","status":"complete","priority":"P1","created_at":"2026-09-01 00:00:00","completed_at":"2026-09-02 00:00:00","updated_at":"2026-09-02 00:00:00"}` + "\n" +
			`{"id":"CLEAN-B","title":"T offset micros","status":"in_progress","priority":"P2","created_at":"2026-08-31T05:13:59.558464+00:00","dispatched_at":"2026-09-01T05:13:59.558464+00:00"}` + "\n" +
			`{"id":"CLEAN-C","title":"zulu","status":"pending","priority":"P3","created_at":"2026-09-19T22:15:03-0500","blocked_since":"2026-09-20 02:15:03 -0500"}` + "\n" +
			`{"id":"CLEAN-D","title":"date-only done on complete row (report falls back)","status":"complete","priority":"P3","created_at":"2026-10-01 12:26:05","completed_at":"2026-10-01","updated_at":"2026-10-01 12:26:05"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df27","namespace":"df27","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("clean board has errors: %+v", rep.Findings)
	}
	if got := df27CountTimestampWarns(rep); got != 0 {
		t.Fatalf("clean board produced %d timestamp warnings: %+v", got, rep.Findings)
	}
}

// TestDFBoardctl27CompletedAtFollowsReportFallback pins the per-field rule
// the report applies (buildTaskRec): completed_at "2026-10-01" on a COMPLETE
// row falls back to updated_at for the done day and never excludes (no
// warning), while the same value on a non-complete row DOES exclude (one
// warning). QA-BOARDCTL-2 on the live board is the complete-row shape.
func TestDFBoardctl27CompletedAtFollowsReportFallback(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"FALLBACK-OK","title":"complete with date-only done","status":"complete","priority":"P1","created_at":"2026-10-01 12:26:05","completed_at":"2026-10-01","updated_at":"2026-10-01 12:26:05"}` + "\n" +
			`{"id":"FALLBACK-BAD","title":"pending with date-only done","status":"pending","priority":"P1","created_at":"2026-10-01 12:26:05","completed_at":"2026-10-01"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df27","namespace":"df27","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got := df27CountTimestampWarns(rep); got != 1 {
		t.Fatalf("timestamp warnings = %d, want 1 (only the non-complete row): %+v", got, rep.Findings)
	}
	got := findWarn(rep, "FALLBACK-BAD")
	if got == "" || !strings.Contains(got, "completed_at") || !strings.Contains(got, `"2026-10-01"`) {
		t.Fatalf("non-complete date-only completed_at not flagged: %q", got)
	}
	if !strings.Contains(got, "--completed-at") {
		t.Fatalf("completed_at warning missing the update --completed-at repair hint: %q", got)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "FALLBACK-OK") && strings.Contains(f.Msg, "time-based metrics") {
			t.Fatalf("complete row with date-only completed_at must not warn (report falls back to updated_at): %s", f.Msg)
		}
	}
}

// TestDFBoardctl27ValidTimestampGrammar: the mirrored grammar accepts every
// stampLayouts dialect (plus the date-only day shape) and rejects the
// offending shapes this work exists to catch.
func TestDFBoardctl27ValidTimestampGrammar(t *testing.T) {
	accept := []string{
		"2026-09-01 00:00:00",
		"2026-09-01 00:00:00.92693",
		"2026-08-31T05:13:59.558464+00:00",
		"2026-09-19T22:15:03-0500",
		"2026-09-19 22:15:03 -0500",
		"2026-09-19T22:15:03Z",
		"2026-09-28T00:13:12Z",
		"2026-10-02 04:01:03",
		"  2026-09-01 00:00:00  ", // surrounding whitespace trims
	}
	reject := []string{
		"",
		"   ",
		"2026-09-09-22T15:13:00Z", // the double-dash dialect
		"2026-09-09-21T13:07:00Z",
		"not-a-date",
		"banana",
		"2026-13-45 99:99:99", // ISO-shaped but not a real instant
		"2026-10-01",          // date-only day: parseStamp has no day layout — a
		// complete row reads past it via updated_at (TimestampFieldFindings
		// mirrors that fallback); on a non-complete row it excludes.
	}
	for _, s := range accept {
		if !ValidTimestamp(s) {
			t.Errorf("ValidTimestamp(%q) = false, want true", s)
		}
	}
	for _, s := range reject {
		if ValidTimestamp(s) {
			t.Errorf("ValidTimestamp(%q) = true, want false", s)
		}
	}
}
