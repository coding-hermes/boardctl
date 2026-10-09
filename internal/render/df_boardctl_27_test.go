package render

import (
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// DF-BOARDCTL-27: validate now carries the SAME timestamp verdict the report
// parser applies. This file is the one place that can hold the two surfaces
// side by side (internal/board cannot import this package's parser), so it
// pins the parity from here:
//
//   - ValidTimestamp must accept exactly what parseStamp accepts
//     (grammar parity — render's df3 dialect matrix, reused as the oracle);
//   - the exclusion rules TimestampFieldFindings encodes (created_at always;
//     completed_at only off the fallback; other fields when present and
//     broken) must produce the same "row excluded from time metrics"
//     verdict the live parser reaches on the same rows.

// TestDFBoardctl27GrammarParityWithParseStamp drives board.ValidTimestamp
// over the DF3 detector-dialect matrix plus the offending shapes: every
// stamp parseStamp accepts must parse under ValidTimestamp, and the
// double-dash/empty dialects must fail BOTH (a value one surface flags,
// both flag).
func TestDFBoardctl27GrammarParityWithParseStamp(t *testing.T) {
	loc := time.UTC
	fracs := []string{"", ".000000", ".92693"}
	zones := []string{"", "Z", "+00:00", "-0500"}
	ref := time.Date(2026, 9, 19, 22, 15, 3, 926930000, time.UTC)
	var samples []string
	add := func(s string) { samples = append(samples, s) }
	for _, frac := range fracs {
		for _, zone := range zones {
			add("2026-09-19T22:15:03" + frac + zone)
			add("2026-09-19 22:15:03" + frac + zone)
			if zone != "" {
				add("2026-09-19 22:15:03" + frac + " " + zone)
			}
		}
	}
	samples = append(samples, "2026-10-02 04:01:03", "2026-10-02T04:01:03")
	for _, sample := range samples {
		layout := board.DetectTSLayout(sample).Layout
		stamp := ref.Format(layout)
		_, parseOK := parseStamp(stamp, loc)
		valid := board.ValidTimestamp(stamp)
		if parseOK != valid {
			t.Errorf("grammar mismatch on %q (layout %q): parseStamp ok=%v, ValidTimestamp=%v",
				stamp, layout, parseOK, valid)
		}
	}
	// the offending dialects: BOTH surfaces must reject
	for _, bad := range []string{
		"2026-09-09-22T15:13:00Z", // the live board's double-dash dialect
		"2026-09-09-21T13:07:00Z",
		"",
		"banana",
	} {
		_, parseOK := parseStamp(bad, loc)
		valid := board.ValidTimestamp(bad)
		if parseOK || valid {
			t.Errorf("offending value %q accepted (parseStamp ok=%v, ValidTimestamp=%v) — both surfaces must reject", bad, parseOK, valid)
		}
	}
}

// TestDFBoardctl27ExclusionVerdictParity seeds the live board's four
// offending shapes plus a valid control, loads them through the REAL report
// parser, and asserts the exclusion set matches what TimestampFieldFindings
// (the validate warning) reports for the same rows.
func TestDFBoardctl27ExclusionVerdictParity(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": `{"id":"REL-04","title":"double-dash","status":"complete","created_at":"2026-09-09-22T15:13:00Z","completed_at":"2026-09-28T00:13:12Z"}` + "\n" +
			`{"id":"REL-05","title":"double-dash two","status":"complete","created_at":"2026-09-09-21T13:07:00Z","completed_at":"2026-09-28T00:13:12Z"}` + "\n" +
			`{"id":"DF24","title":"no birth","status":"complete","completed_at":"2026-10-02 00:30:08"}` + "\n" +
			`{"id":"DF25","title":"no birth two","status":"complete","completed_at":"2026-10-02 04:01:03"}` + "\n" +
			`{"id":"OK","title":"valid","status":"pending","created_at":"2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-19T22:15:03Z","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n",
	})
	d, _ := loadSeed(t, root)

	// report surface: exactly the four offending rows carry birth=nil
	reportExcluded := map[string]bool{}
	for _, rec := range d.tasks {
		if rec.birth == nil {
			reportExcluded[rec.id] = true
		}
	}
	want := map[string]bool{"REL-04": true, "REL-05": true, "DF24": true, "DF25": true}
	for id := range want {
		if !reportExcluded[id] {
			t.Errorf("report parsed %s WITH a birth — premise broken (created_at must fail)", id)
		}
	}
	if reportExcluded["OK"] {
		t.Errorf("control row OK excluded by the report parser — premise broken")
	}
	if d.warns.tsExcl != 4 {
		t.Errorf("report timestamp_excluded = %d, want 4; notes: %v", d.warns.tsExcl, d.warns.notes)
	}

	// validate surface: TimestampFieldFindings must flag the same four rows
	// and no others, on RAW rows (the same entry point validate uses).
	raw := map[string]*board.Row{}
	b, err := board.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		raw[r.String("id")] = r
	}
	validateFlagged := map[string]bool{}
	for id := range raw {
		if fs := board.TimestampFieldFindings(raw[id]); len(fs) > 0 {
			validateFlagged[id] = true
		}
	}
	if len(validateFlagged) != len(want) {
		t.Errorf("validate flagged %v, want exactly the report's exclusion set %v", validateFlagged, want)
	}
	for id := range want {
		if !validateFlagged[id] {
			t.Errorf("report excludes %s but validate does not flag it — surfaces disagree", id)
		}
	}
	if validateFlagged["OK"] {
		t.Errorf("validate flags the control row the report accepts")
	}
}
