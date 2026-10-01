package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-071: paperwork lanes (-docs/-readme/-review) default to P4/P5 priority.
//
// The lanes whose deliverable is a page (docs, readme, review satellites) are
// real work but never more urgent than the build/QA/PM rows above them, yet
// they filed at the P2 create default and crowded the top of every board view
// and the scheduler's pick order. When `create` resolves priority from the
// DEFAULT (no explicit --priority), a board whose lane ends in -docs/-readme
// defaults to P5 and one ending in -review defaults to P4. The rule keys on
// the lane-CLASS suffix — the last hyphen-separated segment of the board's
// lane name (header project, else namespace, else the board dir's parent) —
// matching how the scheduler derives lane classes. An explicit --priority
// always wins, and non-paperwork lanes keep the existing P2 default.

// seedLaneBoard builds a topology-A board whose lane identity (header
// project + namespace) is the given lane name, like seedPriorityBoard.
func seedLaneBoard(t *testing.T, lane string) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"` + lane + `","namespace":"` + lane + `","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := b.LaneName(); got != lane {
		t.Fatalf("LaneName() = %q, want %q", got, lane)
	}
	return b
}

// createPriorityOf runs one create on the board and returns the new row's
// stored priority.
func createPriorityOf(t *testing.T, b *Board, spec TaskRowSpec) string {
	t.Helper()
	if _, err := b.Create(spec); err != nil {
		t.Fatalf("create %s: %v", spec.ID, err)
	}
	raw, err := os.ReadFile(b.TasksPath())
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

// TestCreatePaperworkLaneDefaultPriority: with no --priority, a -docs or
// -readme lane lands P5, a -review lane lands P4, and every non-paperwork
// lane (primary, -qa, and the operational -pm/-sync satellites) keeps the
// pre-BT-071 P2 default.
func TestCreatePaperworkLaneDefaultPriority(t *testing.T) {
	cases := []struct {
		lane string
		want string
	}{
		{"my-project-docs", "P5"},
		{"my-project-readme", "P5"},
		{"my-project-review", "P4"},
		{"my-project-qa", "P2"},   // control: non-paperwork satellite unchanged
		{"my-project", "P2"},      // control: primary lane unchanged
		{"my-project-pm", "P2"},   // operational, NOT paperwork
		{"my-project-sync", "P2"}, // operational, NOT paperwork
	}
	for _, c := range cases {
		b := seedLaneBoard(t, c.lane)
		got := createPriorityOf(t, b, TaskRowSpec{ID: "PAPER-1", Title: "Write the report"})
		if got != c.want {
			t.Fatalf("lane %q default priority = %q, want %q", c.lane, got, c.want)
		}
	}
}

// TestCreateExplicitPriorityOverridesPaperworkDefault: --priority always
// wins over the paperwork default (a human can always promote a doc row).
func TestCreateExplicitPriorityOverridesPaperworkDefault(t *testing.T) {
	for _, tc := range []struct {
		lane, given, want string
	}{
		{"my-project-docs", "P2", "P2"},
		{"my-project-review", "P1", "P1"},
		{"my-project-docs", "2", "P2"}, // bare-digit override normalizes then wins
	} {
		b := seedLaneBoard(t, tc.lane)
		got := createPriorityOf(t, b, TaskRowSpec{ID: "PAPER-2", Title: "t", Priority: tc.given})
		if got != tc.want {
			t.Fatalf("lane %q explicit --priority %q: stored %q, want %q", tc.lane, tc.given, got, tc.want)
		}
	}
}

// TestPaperworkLaneDefault pins the suffix derivation itself: the LAST
// hyphen-separated segment decides, the match is exact and case-sensitive
// (lane classes are lowercase in the fleet), and non-paperwork lanes return
// "" (caller keeps its own default).
func TestPaperworkLaneDefault(t *testing.T) {
	cases := []struct {
		lane string
		want string // "" = not paperwork
	}{
		{"my-project-docs", "P5"},
		{"docs", "P5"},
		{"my-docs", "P5"},
		{"my-project-readme", "P5"},
		{"readme", "P5"},
		{"my-project-review", "P4"},
		{"review", "P4"},
		{"my-project-docs-review", "P4"}, // last segment wins
		{"my-project-qa", ""},
		{"my-project-pm", ""},
		{"my-project-sync", ""},
		{"my-project", ""},
		{"", ""},
		{"my-project-docsx", ""}, // exact suffix match only
		{"project-docs-v2", ""},  // last segment is "v2", not "docs"
		{"my-project-Docs", ""},  // lane classes are lowercase
	}
	for _, c := range cases {
		if got := PaperworkLaneDefault(c.lane); got != c.want {
			t.Fatalf("PaperworkLaneDefault(%q) = %q, want %q", c.lane, got, c.want)
		}
	}
}

// TestUpdateNeverAppliesPaperworkDefault: the default is a CREATE-side
// default for NEW writes only — update repairs existing rows and never
// invents a priority, so a no-field update on a paperwork lane is still
// refused by the change-flag gate and the row's priority is untouched.
func TestUpdateNeverAppliesPaperworkDefault(t *testing.T) {
	b := seedLaneBoard(t, "my-project-docs")
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{}); err == nil {
		t.Fatal("update with no change flags must be refused")
	} else if !strings.Contains(err.Error(), "at least one change flag") {
		t.Fatalf("refusal should name the change-flag gate, got: %v", err)
	}
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	// The seeded row is P2: update must never move it to the paperwork
	// defaults (nor to anything at all).
	if strings.Contains(string(raw), `"priority":"P4"`) || strings.Contains(string(raw), `"priority":"P5"`) {
		t.Fatalf("update must never apply the paperwork default; tasks.jsonl:\n%s", raw)
	}
}
