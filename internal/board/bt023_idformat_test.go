package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bt023FleetIDs are the ids from the board task that MUST pass the fleet id
// format (at least one hyphenated segment; uppercase PREFIX-SEGMENT shape).
var bt023FleetIDs = []string{
	"NEVER-DONE",
	"QA-BOARDCTL-001",
	"GITREINS-JUDGE",
	"DOC-VERSION-1",
	"GAP-104",
	"BT-023",
	"INT-CI-3",
}

// BT-023 (a): create with a junk id ("bad id!") is rejected, names the
// offending value, and writes NOTHING — not to tasks.jsonl, not an event to
// events.jsonl.
func TestCreateRejectsJunkID(t *testing.T) {
	b := newTestBoard(t)
	tasksBefore := boardLineCount(t, b.tasksPath)
	eventsBefore := boardLineCount(t, b.eventsPath)
	created, err := b.Create(TaskRowSpec{ID: "bad id!", Title: "junk"})
	if err == nil {
		t.Fatalf("create with junk id %q accepted", "bad id!")
	}
	if created != "" {
		t.Fatalf("created = %q on rejected create, want empty", created)
	}
	if !strings.Contains(err.Error(), "bad id!") {
		t.Fatalf("error must name the offending id, got: %v", err)
	}
	if !strings.Contains(err.Error(), FleetTaskIDPattern) {
		t.Fatalf("error must name the expected pattern %s, got: %v", FleetTaskIDPattern, err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error must point at the --force escape hatch, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != tasksBefore {
		t.Fatalf("tasks.jsonl grew on rejected create: %d lines, want %d", n, tasksBefore)
	}
	if n := boardLineCount(t, b.eventsPath); n != eventsBefore {
		t.Fatalf("events.jsonl grew on rejected create: %d lines, want %d", n, eventsBefore)
	}
}

// BT-023 (b): --force writes the junk id anyway (the escape hatch for
// legacy-style ids).
func TestCreateForceWritesJunkID(t *testing.T) {
	b := newTestBoard(t)
	created, err := b.Create(TaskRowSpec{ID: "bad id!", Title: "junk", Force: true})
	if err != nil {
		t.Fatalf("create --force with junk id rejected: %v", err)
	}
	if created != "bad id!" {
		t.Fatalf("created = %q, want the forced id", created)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.String("id") == "bad id!" {
			found = true
		}
	}
	if !found {
		t.Fatalf("forced junk id missing from tasks.jsonl rows: %d rows", len(rows))
	}
	// The README-promised task_created event must still be written for the
	// forced row (the audit trail is not part of what --force skips).
	events, _, err := ReadAllRows(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.String("event_type") != "task_created" || last.String("task_id") != "bad id!" {
		t.Fatalf("forced create lost its task_created event: %s", RowJSONCompact(last))
	}
}

// BT-023 (c): every id named by the board task as fleet-valid is accepted by
// create.
func TestCreateAcceptsAllFleetIDs(t *testing.T) {
	for _, id := range bt023FleetIDs {
		t.Run(id, func(t *testing.T) {
			b := newTestBoard(t)
			if _, err := b.Create(TaskRowSpec{ID: id, Title: "fleet id"}); err != nil {
				t.Fatalf("fleet id %q rejected: %v", id, err)
			}
			rows, err := b.TaskRows()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, r := range rows {
				if r.String("id") == id {
					found = true
				}
			}
			if !found {
				t.Fatalf("fleet id %q missing from tasks.jsonl after create", id)
			}
		})
	}
}

// BT-023 (d): a single-segment id ("TODO" — no hyphen-segment) is rejected,
// exactly like other junk ids.
func TestCreateRejectsSingleSegmentID(t *testing.T) {
	b := newTestBoard(t)
	before := boardLineCount(t, b.tasksPath)
	if _, err := b.Create(TaskRowSpec{ID: "TODO", Title: "single segment"}); err == nil {
		t.Fatal("create with single-segment id TODO accepted")
	} else if !strings.Contains(err.Error(), "TODO") {
		t.Fatalf("error must name the offending id, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != before {
		t.Fatalf("tasks.jsonl grew on rejected create: %d lines, want %d", n, before)
	}
}

// BT-023 (e): update on a row whose id violates the fleet format is rejected
// without --force (nothing written) and succeeds with --force — the escape
// hatch keeps legacy junk rows updatable. A conforming id needs no flag.
func TestUpdateJunkIDRequiresForce(t *testing.T) {
	junkRow := `{"id":"bad id!","title":"legacy junk","status":"pending","priority":"P2"}` + "\n"
	dir := t.TempDir()
	files := map[string]string{
		"tasks.jsonl": junkRow,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Rejected without --force; the row must be untouched.
	if _, err := b.UpdateTask("bad id!", UpdateSpec{Status: strPtr("complete")}); err == nil {
		t.Fatal("update on junk-id row accepted without --force")
	} else if !strings.Contains(err.Error(), "bad id!") {
		t.Fatalf("error must name the offending id, got: %v", err)
	}
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != junkRow {
		t.Fatalf("rejected update mutated tasks.jsonl:\n got %q\nwant %q", raw, junkRow)
	}

	// With --force the legacy junk row stays updatable.
	status := "complete"
	if _, err := b.UpdateTask("bad id!", UpdateSpec{Status: &status, Force: true}); err != nil {
		t.Fatalf("update --force on junk-id row rejected: %v", err)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].String("id") != "bad id!" || rows[0].String("status") != "complete" {
		t.Fatalf("forced update landed wrong: %s", RowJSONCompact(rows[0]))
	}

	// A conforming id never needs the flag.
	b2 := newTestBoard(t)
	if _, err := b2.UpdateTask("EXIST-1", UpdateSpec{Status: strPtr("review")}); err != nil {
		t.Fatalf("update on conforming id rejected without --force: %v", err)
	}
}

// BT-023 (f): doctor WARNS (never errors) per existing junk id, with the
// tasks.jsonl line number; a board with only junk-id warnings stays green
// (HasErrors() == false), and a clean-id board produces no id-format
// findings.
func TestDoctorWarnsOnJunkIDWithoutFailing(t *testing.T) {
	junk := `{"id":"bad id!","title":"legacy junk","status":"pending","priority":"P2"}` + "\n"
	clean := `{"id":"WORK-1","title":"Work","status":"pending","priority":"P1"}` + "\n"
	b := newGitTestBoard(t, map[string]string{
		"tasks.jsonl": junk + clean,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
	})
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("junk id must WARN, not error: %+v", rep.Findings)
	}
	warns := 0
	for _, f := range rep.Findings {
		if f.IsError() {
			continue
		}
		if !strings.Contains(f.Msg, "bad id!") {
			continue
		}
		warns++
		if !strings.Contains(f.Msg, "line 1") {
			t.Fatalf("junk-id warning must carry the line number, got: %s", f.Msg)
		}
		if !strings.Contains(f.Msg, FleetTaskIDPattern) {
			t.Fatalf("junk-id warning must name the expected pattern, got: %s", f.Msg)
		}
	}
	if warns != 1 {
		t.Fatalf("want exactly 1 junk-id warning, got %d: %+v", warns, rep.Findings)
	}

	// The clean row's line must NOT be flagged.
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "WORK-1") && strings.Contains(f.Msg, "fleet id format") {
			t.Fatalf("conforming id flagged: %s", f.Msg)
		}
	}

	// A board with only clean ids emits no id-format findings.
	b2 := newGitTestBoard(t, map[string]string{
		"tasks.jsonl": clean,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
	})
	rep2, err := b2.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep2.Findings {
		if strings.Contains(f.Msg, "fleet id format") {
			t.Fatalf("clean board produced an id-format finding: %s", f.Msg)
		}
	}
}

// BT-023: validate stays SILENT on legacy junk ids — the board task forbids
// making validate fail on existing rows; doctor carries the warning.
func TestValidateSilentOnJunkID(t *testing.T) {
	junk := `{"id":"bad id!","title":"legacy junk","status":"pending","priority":"P2"}` + "\n"
	b := newGitTestBoard(t, map[string]string{
		"tasks.jsonl": junk,
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"cooldown_s":21600}` + "\n",
	})
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("junk id must not fail validate: %+v", rep.Findings)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "bad id!") {
			t.Fatalf("validate must stay silent on legacy junk ids, found: %s", f.Msg)
		}
	}
}

// BT-023: the pattern helper itself — acceptance/rejection table around the
// documented edge cases (hyphen segments, digits, lowercase, spaces,
// punctuation, leading digit, trailing hyphen).
func TestMatchesFleetTaskID(t *testing.T) {
	for _, id := range bt023FleetIDs {
		if !MatchesFleetTaskID(id) {
			t.Errorf("MatchesFleetTaskID(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"bad id!", "TODO", "bt-023", "BT-023 ", "BT023", "-BT-023", "BT-", "BT--023", "BT-023!", "1-2", "a-B-1"} {
		if MatchesFleetTaskID(id) {
			t.Errorf("MatchesFleetTaskID(%q) = true, want false", id)
		}
	}
	// ValidateFleetTaskID agrees with the matcher and names the value.
	if err := ValidateFleetTaskID("BT-023"); err != nil {
		t.Errorf("ValidateFleetTaskID(BT-023) = %v, want nil", err)
	}
	if err := ValidateFleetTaskID("bad id!"); err == nil {
		t.Error("ValidateFleetTaskID(bad id!) = nil, want error")
	}
}
