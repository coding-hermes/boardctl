package render

import (
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// DF-BOARDCTL-3: the report parser (parseStamp/stampLayouts) must accept
// every timestamp dialect board.DetectTSLayout accepts — including the
// colon-less numeric-offset forms (-0500) — so detector-valid stamps are
// never silently excluded from time-based metrics.

func TestDF3ColonLessOffsetsAccepted(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		in   string
		want time.Time
	}{
		// the acceptance pair from the ticket
		{"2026-09-19 22:15:03 -0500", time.Date(2026, 9, 20, 3, 15, 3, 0, time.UTC)},
		{"2026-09-19T22:15:03-0500", time.Date(2026, 9, 20, 3, 15, 3, 0, time.UTC)},
		// fraction variants of the new layouts
		{"2026-09-19 22:15:03.92693 -0500", time.Date(2026, 9, 20, 3, 15, 3, 926930000, time.UTC)},
		{"2026-09-19T22:15:03.558464-0500", time.Date(2026, 9, 20, 3, 15, 3, 558464000, time.UTC)},
		// positive colon-less offsets
		{"2026-09-19T22:15:03+0530", time.Date(2026, 9, 19, 16, 45, 3, 0, time.UTC)},
		{"2026-09-19 22:15:03 +0530", time.Date(2026, 9, 19, 16, 45, 3, 0, time.UTC)},
	}
	for _, c := range cases {
		got, ok := parseStamp(c.in, loc)
		if !ok {
			t.Fatalf("parseStamp(%q) rejected — detector-valid dialect must parse", c.in)
		}
		if !got.Equal(c.want) {
			t.Fatalf("parseStamp(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestDF3ParserCoversDetectorDialects walks the full DetectTSLayout dialect
// matrix (space/T base x none/fixed/trimmed fraction x no-zone/Z/±hh:mm/
// ±hhmm zone, glued and space-separated zone spellings), renders a reference
// instant through the DETECTED layout, and requires parseStamp to accept it
// back at the same instant. No dialect the detector calls valid may be
// excluded by the report parser.
func TestDF3ParserCoversDetectorDialects(t *testing.T) {
	fracs := []string{"", ".000000", ".92693"} // none, constant-width, trimmed
	zones := []string{"", "Z", "+00:00", "-0500"}
	ref := time.Date(2026, 9, 19, 22, 15, 3, 926930000, time.UTC)
	var samples []string
	add := func(s string) { samples = append(samples, s) }
	for _, frac := range fracs {
		for _, zone := range zones {
			add("2026-09-19T22:15:03" + frac + zone)
			add("2026-09-19 22:15:03" + frac + zone) // glued zone spelling
			if zone != "" {
				add("2026-09-19 22:15:03" + frac + " " + zone) // spaced spelling
			}
		}
	}
	for _, sample := range samples {
		layout := board.DetectTSLayout(sample).Layout
		stamp := ref.Format(layout) // canonical rendering of the dialect
		got, ok := parseStamp(stamp, time.UTC)
		if !ok {
			t.Errorf("detector-valid dialect %q (layout %q) renders %q — rejected by parseStamp", sample, layout, stamp)
			continue
		}
		if got.Unix() != ref.Unix() {
			t.Errorf("parseStamp(%q) (layout %q) = %v, want instant %v", stamp, layout, got, ref)
		}
	}
}

// TestDF3ColonLessStampsNotExcluded loads a board whose rows carry
// colon-less-offset stamps and asserts zero timestamp exclusions.
func TestDF3ColonLessStampsNotExcluded(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": `{"id":"DF3A","title":"x","status":"pending","created_at":"2026-09-19 22:15:03 -0500"}` + "\n" +
			`{"id":"DF3B","title":"y","status":"complete","created_at":"2026-09-19T22:15:03-0500","completed_at":"2026-09-20T02:15:03-0500"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-19T22:15:03-0500","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n",
	})
	d, _ := loadSeed(t, root)
	if d.warns.tsExcl != 0 {
		t.Fatalf("TimestampExcluded = %d, want 0 (detector-valid dialects must not be excluded); notes: %v",
			d.warns.tsExcl, d.warns.notes)
	}
	if w := d.warns.payload(); w.TimestampExcluded != 0 || w.Tasks != 0 || w.Events != 0 || w.DuplicateIDs != 0 {
		t.Fatalf("parse warnings = %+v, want zero", w)
	}
	if len(d.tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(d.tasks))
	}
	for _, rec := range d.tasks {
		if rec.birth == nil {
			t.Fatalf("task %s: birth nil — excluded from time metrics", rec.id)
		}
		if rec.id == "DF3B" && rec.done == nil {
			t.Fatalf("task DF3B: done nil — completed_at not parsed")
		}
	}
	if len(d.events) != 1 || d.events[0].ts == nil {
		t.Fatalf("event timestamp not parsed: %+v", d.events)
	}
}
