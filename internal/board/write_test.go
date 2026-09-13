package board

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// boardLineCount counts non-empty lines of a board file.
func boardLineCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

// lastTaskRaw returns the last non-empty tasks.jsonl line parsed as a map.
func lastTaskRaw(t *testing.T, b *Board) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	var last []byte
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			last = []byte(l)
		}
	}
	var row map[string]any
	if err := json.Unmarshal(last, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// BT-007: update --guard MAYBE must be REJECTED — only PASS|FAIL|SKIP are in
// the guard_result vocabulary — and nothing may be written.
func TestUpdateGuardRejectsOutOfVocab(t *testing.T) {
	b := newTestBoard(t)
	maybe := "MAYBE"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Guard: &maybe}); err == nil {
		t.Fatal("update --guard MAYBE accepted")
	} else if !strings.Contains(err.Error(), "guard") || !strings.Contains(err.Error(), "PASS,FAIL,SKIP") {
		t.Fatalf("error should name the guard vocabulary, got: %v", err)
	}
	// the row must be untouched (single line, original bytes)
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if n := boardLineCount(t, b.tasksPath); n != 1 {
		t.Fatalf("tasks.jsonl has %d lines after rejected update, want 1", n)
	}
	if strings.Contains(string(raw), "MAYBE") {
		t.Fatalf("rejected value leaked into tasks.jsonl: %s", raw)
	}
}

// BT-007: guard is normalized (trimmed + upper-cased) before the vocabulary
// check, so "pass" stores PASS — case tolerance on input, canonical on disk.
func TestUpdateGuardNormalizesCase(t *testing.T) {
	b := newTestBoard(t)
	pass := "pass"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Guard: &pass}); err != nil {
		t.Fatalf("update --guard pass rejected: %v", err)
	}
	row := lastTaskRaw(t, b)
	if row["guard_result"] != "PASS" {
		t.Fatalf("guard_result = %v, want PASS", row["guard_result"])
	}
}

// BT-007: update --ci BANANA must be rejected; GREEN|RED|SKIP are accepted.
func TestUpdateCIRejectsOutOfVocab(t *testing.T) {
	b := newTestBoard(t)
	banana := "BANANA"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CI: &banana}); err == nil {
		t.Fatal("update --ci BANANA accepted")
	} else if !strings.Contains(err.Error(), "GREEN,RED,SKIP") {
		t.Fatalf("error should name the ci vocabulary, got: %v", err)
	}
	green := "green"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{CI: &green}); err != nil {
		t.Fatalf("update --ci green rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["ci_result"] != "GREEN" {
		t.Fatalf("ci_result = %v, want GREEN", row["ci_result"])
	}
}

// BT-007: create --priority 1 is NORMALIZED to P1 (fleet boards use P0-P3;
// bare digits must not fork the stats grouping).
func TestCreatePriorityNormalizesBareDigits(t *testing.T) {
	b := newTestBoard(t)
	one := "1"
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "n1", Priority: one}); err != nil {
		t.Fatalf("create --priority 1 rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["priority"] != "P1" {
		t.Fatalf("priority = %v, want P1 (normalized from %q)", row["priority"], one)
	}
	// case variant normalizes too
	if _, err := b.Create(TaskRowSpec{ID: "NEW-2", Title: "n2", Priority: "p3"}); err != nil {
		t.Fatalf("create --priority p3 rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["priority"] != "P3" {
		t.Fatalf("priority = %v, want P3", row["priority"])
	}
}

// BT-007: a priority outside {P0,P1,P2,P3} (after normalization) is rejected.
func TestCreatePriorityRejectsGarbage(t *testing.T) {
	b := newTestBoard(t)
	before := boardLineCount(t, b.tasksPath)
	if _, err := b.Create(TaskRowSpec{ID: "NEW-1", Title: "n1", Priority: "banana"}); err == nil {
		t.Fatal("create --priority banana accepted")
	} else if !strings.Contains(err.Error(), "P0,P1,P2,P3") {
		t.Fatalf("error should name the priority vocabulary, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != before {
		t.Fatalf("tasks.jsonl grew from %d to %d lines on rejected create", before, n)
	}
}

// BT-007: create --depends-on GHOST-9 is rejected — the dependency must exist
// in tasks.jsonl — and NOTHING is written (no task row, no task_created
// event) because the append-only store cannot repair a dangling ref later.
func TestCreateDependsOnGhostRejected(t *testing.T) {
	b := newTestBoard(t)
	tasksBefore := boardLineCount(t, b.tasksPath)
	eventsBefore := boardLineCount(t, b.eventsPath)
	_, err := b.Create(TaskRowSpec{
		ID: "NEW-1", Title: "n1", Priority: "P2",
		HasDependsOn: true, DependsOn: []string{"GHOST-9"},
	})
	if err == nil {
		t.Fatal("create --depends-on GHOST-9 accepted")
	}
	if !strings.Contains(err.Error(), "GHOST-9") {
		t.Fatalf("error should name the missing id, got: %v", err)
	}
	if n := boardLineCount(t, b.tasksPath); n != tasksBefore {
		t.Fatalf("tasks.jsonl grew from %d to %d lines on rejected create", tasksBefore, n)
	}
	if n := boardLineCount(t, b.eventsPath); n != eventsBefore {
		t.Fatalf("events.jsonl grew from %d to %d lines on rejected create (no task_created event may fire)", eventsBefore, n)
	}
}

// BT-007: a depends_on id that DOES exist is accepted and stored.
func TestCreateDependsOnExistingAccepted(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.Create(TaskRowSpec{
		ID: "NEW-1", Title: "n1", Priority: "P2",
		HasDependsOn: true, DependsOn: []string{"EXIST-1"},
	}); err != nil {
		t.Fatalf("create with existing dependency rejected: %v", err)
	}
	if row := lastTaskRaw(t, b); row["depends_on"] == nil {
		t.Fatalf("depends_on not stored: %v", row)
	}
}

// BT-007: event --type outside the vocabulary is rejected at append time.
func TestAppendEventRejectsUnknownType(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.AppendEvent(EventSpec{Type: "banana"}); err == nil {
		t.Fatal("event --type banana accepted")
	} else if !strings.Contains(err.Error(), "event type") {
		t.Fatalf("error should name the event type vocabulary, got: %v", err)
	}
	// empty type still defaults to audit; every enumerated type is accepted
	for _, et := range []string{"", "audit", "task_created", "task_dispatched",
		"task_completed", "task_updated", "tick", "idle", "board_init"} {
		if _, err := b.AppendEvent(EventSpec{Type: et}); err != nil {
			t.Fatalf("event type %q rejected: %v", et, err)
		}
	}
}

// BT-007: header --set-ticks-total/-idle reject negative counters at write
// time, leaving the header untouched.
func TestSetHeaderRejectsNegativeCounters(t *testing.T) {
	b := newTestBoard(t)
	before, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	neg := int64(-5)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &neg}); err == nil {
		t.Fatal("header --set-ticks-total -5 accepted")
	} else if !strings.Contains(err.Error(), "negative") {
		t.Fatalf("error should say negative, got: %v", err)
	}
	if _, err := b.SetHeader(HeaderUpdate{TicksIdle: &neg}); err == nil {
		t.Fatal("header --set-ticks-idle -5 accepted")
	}
	after, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected header update mutated board.jsonl:\nbefore %s\nafter  %s", before, after)
	}
	// zero stays legal (a fresh board's counters are 0)
	zero := int64(0)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &zero, TicksIdle: &zero}); err != nil {
		t.Fatalf("zero counters rejected: %v", err)
	}
}

// BT-014: an event appended with a tick number is a completed tick —
// AppendEvent must raise header ticks_total to the event's tick so the next
// doctor run does not fail the BT-013 counter-drift check.
func TestAppendEventTickBumpsHeaderTicksTotal(t *testing.T) {
	b := newTestBoard(t) // header ticks_total=1, seed event tick_number=1
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"tick 5 summary"`), Tick: i64Ptr(5)}); err != nil {
		t.Fatal(err)
	}
	hdr, err := b.HeaderRow()
	if err != nil {
		t.Fatal(err)
	}
	total, ok := hdr.Int("ticks_total")
	if !ok {
		t.Fatal("header ticks_total missing after tick-bearing event")
	}
	if total != 5 {
		t.Fatalf("ticks_total = %d, want 5 (max(current, tick) — no regression)", total)
	}
	// the bump must keep the drift check green: max event tick 5 == total 5
	rep, err := b.Doctor()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.IsError() && strings.Contains(f.Msg, "header ticks_total") {
			t.Fatalf("doctor reports counter drift after the auto-bump: %+v", f)
		}
	}
}

// BT-014: a tick-bearing event must never REGRESS the header counter — an
// out-of-order event (tick 2 on a board already at 5) leaves ticks_total 5.
func TestAppendEventTickNeverRegressesTicksTotal(t *testing.T) {
	b := newTestBoard(t) // header ticks_total=1
	five := int64(5)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &five}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"late tick 2"`), Tick: i64Ptr(2)}); err != nil {
		t.Fatal(err)
	}
	hdr, err := b.HeaderRow()
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := hdr.Int("ticks_total"); total != 5 {
		t.Fatalf("ticks_total = %d, want 5 (older tick must not regress the counter)", total)
	}
}

// BT-014 acceptance: a tick-LESS event (plain audit row) is not a tick
// completion — the header must stay untouched, byte for byte.
func TestAppendEventWithoutTickLeavesHeaderUntouched(t *testing.T) {
	b := newTestBoard(t)
	before, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"plain audit, no tick"`)}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("tick-less event mutated the header:\nbefore %s\nafter  %s", before, after)
	}
}

// BT-014 acceptance: topology-B boards (header on line 1 of tasks.jsonl)
// get the same auto-bump — SetHeader already targets the line-1 header.
func TestAppendEventTickBumpsHeaderOnTopologyB(t *testing.T) {
	b := newTestBoardB(t) // line-1 header ticks_total=1, seed event tick 1
	if _, err := b.AppendEvent(EventSpec{Type: "audit", Actor: "foreman", DetailText: strPtr(`"tick 3 summary"`), Tick: i64Ptr(3)}); err != nil {
		t.Fatal(err)
	}
	hdr, err := b.HeaderRow()
	if err != nil {
		t.Fatal(err)
	}
	if total, _ := hdr.Int("ticks_total"); total != 3 {
		t.Fatalf("topology-B ticks_total = %d, want 3 after event --tick 3", total)
	}
	// the line-1 bump must not have disturbed the task row below it
	tasks, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].String("id") != "EXIST-1" {
		t.Fatalf("topology-B task rows disturbed: %v", tasks)
	}
}

// ---------- BT-024: SetHeader stamps header updated_at ----------

// seedStaleHeaderBoard builds a topology-A board whose header carries a STALE
// updated_at (2026-09-01T00:00:00Z) and a drift pointer last_commit. Used to
// prove SetHeader refreshes updated_at in the header's own dialect.
func seedStaleHeaderBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"drift-pointer","updated_at":"2026-09-01T00:00:00Z"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// parseHeaderLine1 unmarshals line 1 of the given file into a map.
func parseHeaderLine1(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)[0]
	var hdr map[string]any
	if err := json.Unmarshal([]byte(line), &hdr); err != nil {
		t.Fatalf("line 1 of %s does not parse: %v (%s)", path, err, line)
	}
	return hdr
}

// TestSetHeaderRefreshesUpdatedAtTopologyA: a ticks_total bump refreshes the
// STALE header updated_at to a later time in the header's own dialect, and
// every untouched header field keeps its value.
func TestSetHeaderRefreshesUpdatedAtTopologyA(t *testing.T) {
	b := seedStaleHeaderBoard(t)
	five := int64(5)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &five}); err != nil {
		t.Fatal(err)
	}
	hdr := parseHeaderLine1(t, b.headerPath)
	got, _ := hdr["updated_at"].(string)
	if got == "" {
		t.Fatalf("updated_at missing from header after SetHeader: %v", hdr)
	}
	// later than the seeded stale value, in the header's own dialect (Z)
	stale, err := time.Parse(time.RFC3339, "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("refreshed updated_at %q does not parse: %v", got, err)
	}
	if !fresh.After(stale) {
		t.Fatalf("updated_at = %q, want later than the seeded %s", got, stale)
	}
	if !strings.HasSuffix(got, "Z") {
		t.Fatalf("updated_at = %q, want the header's Z-suffixed dialect", got)
	}
	if hdr["last_commit"] != "drift-pointer" {
		t.Fatalf("last_commit disturbed: %v", hdr["last_commit"])
	}
	if hdr["project"] != "test" {
		t.Fatalf("header identity disturbed: %v", hdr["project"])
	}
}

// TestSetHeaderRefreshesUpdatedAtOnTopologyB: the same refresh on a
// topology-B header — a last_commit change via SetHeader stamps updated_at
// (added when the header lacks it) without disturbing the task rows.
func TestSetHeaderRefreshesUpdatedAtOnTopologyB(t *testing.T) {
	b := newTestBoardB(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	taskBefore := strings.SplitN(strings.TrimRight(string(before), "\n"), "\n", 2)[1]
	sha := "board-edit-sha"
	if _, err := b.SetHeader(HeaderUpdate{LastCommit: &sha}); err != nil {
		t.Fatal(err)
	}
	hdr := parseHeaderLine1(t, b.tasksPath)
	if hdr["last_commit"] != sha {
		t.Fatalf("last_commit = %v, want %s", hdr["last_commit"], sha)
	}
	got, _ := hdr["updated_at"].(string)
	if got == "" {
		t.Fatalf("updated_at not added by SetHeader: %v", hdr)
	}
	if !strings.HasPrefix(got, "2026-") {
		t.Fatalf("updated_at = %q, want a current timestamp (spec default dialect)", got)
	}
	// the task row below the header round-trips byte-identical
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	taskAfter := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)[1]
	if taskAfter != taskBefore {
		t.Fatalf("task row mutated by SetHeader:\n got %s\nwant %s", taskAfter, taskBefore)
	}
}

// TestSetHeaderUpdatedAtDialectPreserved: the refreshed updated_at speaks the
// header's own dialect (space-naive with constant .000000 fraction here), and
// a failed update (negative counter) writes nothing.
func TestSetHeaderUpdatedAtDialectPreserved(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"drift-pointer","updated_at":"2026-09-01 00:00:00.000000"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	neg := int64(-1)
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &neg}); err == nil {
		t.Fatal("negative ticks_total accepted")
	}
	mid, err := os.ReadFile(b.headerPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(mid) {
		t.Fatalf("failed header update mutated the header:\nbefore %s\nafter  %s", before, mid)
	}
	zero := int64(0)
	if _, err := b.SetHeader(HeaderUpdate{TicksIdle: &zero}); err != nil {
		t.Fatal(err)
	}
	hdr := parseHeaderLine1(t, b.headerPath)
	got, _ := hdr["updated_at"].(string)
	if got == "" {
		t.Fatalf("updated_at missing after SetHeader: %v", hdr)
	}
	// space-naive 6-digit-fraction dialect preserved (the seeded ".000000"
	// sample maps to a zero-padded fraction; Go renders the actual digits)
	if !strings.Contains(got, " ") {
		t.Fatalf("updated_at = %q, want the header's space-naive dialect", got)
	}
	if !regexp.MustCompile(`\.\d{6}$`).MatchString(got) {
		t.Fatalf("updated_at = %q, want the header's 6-digit fraction dialect", got)
	}
}

// TestSetHeaderUpdatedAtDialectFromLastTick: a topology-A header with NO
// updated_at but a last_tick in the RFC3339/Z dialect stamps the newly added
// updated_at in that dialect (BT-024 AC3 — the fallback samples the header's
// real timestamp fields; last_tick is first in the fallback order and the
// duplicated updated_at entry is gone).
func TestSetHeaderUpdatedAtDialectFromLastTick(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"complete","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"drift-pointer","last_tick":"2026-09-11T17:55:00Z"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	sha := "board-edit-sha"
	if _, err := b.SetHeader(HeaderUpdate{LastCommit: &sha}); err != nil {
		t.Fatal(err)
	}
	hdr := parseHeaderLine1(t, b.headerPath)
	if hdr["last_commit"] != sha {
		t.Fatalf("last_commit = %v, want %s", hdr["last_commit"], sha)
	}
	if hdr["last_tick"] != "2026-09-11T17:55:00Z" {
		t.Fatalf("last_tick disturbed: %v", hdr["last_tick"])
	}
	got, _ := hdr["updated_at"].(string)
	if got == "" {
		t.Fatalf("updated_at not added by SetHeader: %v", hdr)
	}
	seeded, err := time.Parse(time.RFC3339, "2026-09-11T17:55:00Z")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("added updated_at %q does not parse as RFC3339: %v", got, err)
	}
	if !fresh.After(seeded) {
		t.Fatalf("updated_at = %q, want later than the seeded last_tick %s", got, seeded)
	}
	if !strings.HasSuffix(got, "Z") {
		t.Fatalf("updated_at = %q, want the header's RFC3339/Z dialect from last_tick", got)
	}
}

// TestHeaderTSFormatLastTickPreferred: with no updated_at, the fallback
// samples last_tick BEFORE created_at (last_tick is the live tick clock;
// created_at is board birth) and never re-samples updated_at.
func TestHeaderTSFormatLastTickPreferred(t *testing.T) {
	row := &Row{Keys: nil, Vals: map[string]json.RawMessage{}}
	row.SetRaw("created_at", json.RawMessage(`"2026-08-01 00:00:00.000000"`))
	row.SetRaw("last_tick", json.RawMessage(`"2026-09-11T17:55:00Z"`))
	b := &Board{}
	got := b.headerTSFormat(row).Layout
	if got != "2006-01-02T15:04:05Z07:00" {
		t.Fatalf("headerTSFormat layout = %q, want the last_tick RFC3339/Z layout 2006-01-02T15:04:05Z07:00", got)
	}
}

// TestSetHeaderCombinedFlagsRefreshUpdatedAt: --set-ticks-total and
// --set-last-commit together refresh updated_at exactly once and keep both
// requested values.
func TestSetHeaderCombinedFlagsRefreshUpdatedAt(t *testing.T) {
	b := seedStaleHeaderBoard(t)
	seven := int64(7)
	sha := "board-edit-sha"
	if _, err := b.SetHeader(HeaderUpdate{TicksTotal: &seven, LastCommit: &sha}); err != nil {
		t.Fatal(err)
	}
	hdr := parseHeaderLine1(t, b.headerPath)
	if hdr["ticks_total"] != float64(7) || hdr["last_commit"] != sha {
		t.Fatalf("combined flags lost values: %v", hdr)
	}
	got, _ := hdr["updated_at"].(string)
	if got == "" {
		t.Fatalf("updated_at missing after combined SetHeader: %v", hdr)
	}
	stale, err := time.Parse(time.RFC3339, "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("refreshed updated_at %q does not parse: %v", got, err)
	}
	if !fresh.After(stale) {
		t.Fatalf("updated_at = %q, want refreshed past the seeded %s", got, stale)
	}
}
