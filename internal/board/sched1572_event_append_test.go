package board

import (
	"os"
	"strings"
	"testing"
)

// SCHED-GAP-1572 (event side): a legacy TAIL row — missing or non-numeric
// id — must not become the template for the next appended event. Pre-fix,
// appendEventLocked took style/timestamp layout from the LAST row whatever
// its id shape, so an append after a legacy tail inherited the legacy shape
// (or, with NO numeric row anywhere, an inherited-legacy row) and the board
// never healed by writing. Post-fix the template is the last row carrying a
// NUMERIC id (falling back to the default schema when none exists), and the
// appended row's id is MAX(numeric)+1.
func seed1572EventsBoard(t *testing.T, events string) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"T-1","title":"t","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": events,
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":3,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// last1572EventLine returns the last non-empty line of the board's events
// file as a string (board_test.go's lastEventLine returns []byte; a
// different name avoids the redeclaration).
func last1572EventLine(t *testing.T, b *Board) string {
	t.Helper()
	raw, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	return lines[len(lines)-1]
}

func Test1572AppendAfterLegacyTails(t *testing.T) {
	const numericRow = `{"id": 7, "timestamp": "2026-10-06 10:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 3}` + "\n"
	tails := map[string]string{
		"no-id row": numericRow +
			`{"timestamp": "2026-10-06T20:20:00Z", "event_type": "tick_close", "task_id": "X-1", "actor": "foreman", "outcome": "FAILED"}` + "\n",
		"string id": numericRow +
			`{"id": "EVT-025-close", "timestamp": "2026-10-06T21:00:00Z", "event_type": "task_completed", "task_id": "X-1", "actor": "foreman", "detail": null}` + "\n",
		"no-id + str": numericRow +
			`{"timestamp": "2026-10-06T21:10:00Z", "event_type": "audit", "actor": "releng:crier", "detail": "x"}` + "\n" +
			`{"id": "1322", "type": "audit", "actor": "releng:crier", "task_id": "X-2", "created_at": "2026-10-06T23:50:00Z", "detail": "y"}` + "\n",
	}
	for name, events := range tails {
		t.Run(name, func(t *testing.T) {
			b := seed1572EventsBoard(t, events)
			id, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "s1572"})
			if err != nil {
				t.Fatalf("append after %s: %v", name, err)
			}
			if id != 8 {
				t.Fatalf("appended id = %d, want MAX(numeric 7)+1 = 8 (%s)", id, name)
			}
			last := last1572EventLine(t, b)
			// The new row carries the CANONICAL event schema: numeric id
			// first, the numeric row's space-separated timestamp LAYOUT (the
			// ISO-8601 "T" of the legacy tail must NOT be inherited), the
			// event_type vocabulary field, and none of the legacy tail's
			// foreign keys.
			if !strings.HasPrefix(last, `{"id": 8, "timestamp": "`) {
				t.Fatalf("appended row does not start with the canonical id/timestamp pair:\n%s", last)
			}
			ts := between(last, `"timestamp": "`, `"`)
			if !strings.Contains(ts, " ") || strings.Contains(ts, "T") {
				t.Fatalf("appended row inherited the legacy timestamp layout %q; want the numeric row's space layout", ts)
			}
			if !strings.Contains(last, `"event_type": "audit"`) {
				t.Fatalf("appended row lost the canonical event_type field:\n%s", last)
			}
			for _, foreign := range []string{`"outcome"`, `"created_at"`, `"type":`} {
				if strings.Contains(last, foreign) {
					t.Fatalf("appended row inherited legacy key %s:\n%s", foreign, last)
				}
			}
		})
	}
}

// Test1572AppendOnLegacyOnlyBoard: a board whose events are ALL legacy-shape
// still appends — id starts at 1 with the default schema (the no-template
// default), never inheriting the legacy row's keys.
func Test1572AppendOnLegacyOnlyBoard(t *testing.T) {
	b := seed1572EventsBoard(t, `{"actor": "foreman", "event_type": "audit", "detail": "only legacy rows here"}`+"\n")
	id, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "s1572"})
	if err != nil {
		t.Fatalf("append on legacy-only board: %v", err)
	}
	if id != 1 {
		t.Fatalf("appended id = %d, want 1 (no numeric ids on the board)", id)
	}
	last := last1572EventLine(t, b)
	if !strings.Contains(last, `"id": 1`) && !strings.Contains(last, `"id":1`) {
		t.Fatalf("appended row does not carry numeric id 1:\n%s", last)
	}
	for _, foreign := range []string{"only legacy rows here", `"created_at"`} {
		if strings.Contains(last, foreign) {
			t.Fatalf("appended row inherited the legacy row's content/keys (%s):\n%s", foreign, last)
		}
	}
}

// between returns the substring of s between the first occurrence of start
// (after it) and the next occurrence of end.
func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	rest := s[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		return rest
	}
	return rest[:j]
}
