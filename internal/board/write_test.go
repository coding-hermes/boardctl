package board

import (
	"encoding/json"
	"errors"
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

// ---------- SG-126: finding-fingerprint dedupe ----------

// seedFindingBoard writes a topology-A board whose tasks.jsonl holds the given
// raw task rows (one JSON object per line, exactly as passed) and returns the
// resolved board. Used to plant rows the create path would never write (an
// open row with no fingerprint, pre-existing duplicate groups).
func seedFindingBoard(t *testing.T, taskLines ...string) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  strings.Join(taskLines, "\n") + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"test","namespace":"test","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// taskLineByID returns the tasks.jsonl line whose id matches, verbatim.
func taskLineByID(t *testing.T, b *Board, id string) string {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("tasks.jsonl line does not parse: %v (%s)", err, l)
		}
		if row["id"] == id {
			return l
		}
	}
	t.Fatalf("task %s not found in tasks.jsonl", id)
	return ""
}

// taskRowByID returns the parsed tasks.jsonl row (as a generic map).
func taskRowByID(t *testing.T, b *Board, id string) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(taskLineByID(t, b, id)), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

// lastEventRaw returns the last events.jsonl line parsed as a map.
func lastEventRaw(t *testing.T, b *Board) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(b.EventsPath())
	if err != nil {
		t.Fatal(err)
	}
	var last string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			last = l
		}
	}
	var e map[string]any
	if err := json.Unmarshal([]byte(last), &e); err != nil {
		t.Fatalf("last event line does not parse: %v (%s)", err, last)
	}
	return e
}

// eventDetailMap decodes an event row's detail (a JSON STRING carrying JSON)
// into a generic map.
func eventDetailMap(t *testing.T, e map[string]any) map[string]any {
	t.Helper()
	s, _ := e["detail"].(string)
	if s == "" {
		t.Fatalf("event detail is not a JSON string: %v", e["detail"])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("event detail is not JSON: %v (%s)", err, s)
	}
	return out
}

// detailFingerprint extracts detail.fingerprint from a parsed task row.
func detailFingerprint(t *testing.T, row map[string]any) string {
	t.Helper()
	d, ok := row["detail"].(map[string]any)
	if !ok {
		t.Fatalf("row detail is not the SG-126 envelope object: %v", row["detail"])
	}
	fp, _ := d["fingerprint"].(string)
	if fp == "" {
		t.Fatalf("row detail carries no fingerprint: %v", d)
	}
	return fp
}

// SG-126 normalizer: lowercase, strip the recycled finding-id prefix, collapse
// whitespace, strip trailing recurrence / run-id markers.
func TestNormalizeFindingTextStripsRecycledTokens(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[P2] Widget crashes on empty input", "[p2] widget crashes on empty input"},
		{"QA-CRIER-3 [P2] Widget crashes on empty input", "[p2] widget crashes on empty input"},
		{"DF-AI-PLAYS-POKE-12 [P2] Widget crashes on empty input", "[p2] widget crashes on empty input"},
		{"DOGFOOD-MESH-1 [P2] Widget crashes", "[p2] widget crashes"},
		{"[P2] Widget crashes (recurrence 3)", "[p2] widget crashes"},
		{"[P2] Widget crashes (3rd)", "[p2] widget crashes"},
		{"[P2] Widget crashes [run-id-7]", "[p2] widget crashes"},
		{"[P2] Widget crashes [run-id-7] (recurrence 2)", "[p2] widget crashes"},
		{"[P2]  Widget   crashes\non  empty input", "[p2] widget crashes on empty input"},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeFindingText(c.in); got != c.want {
			t.Errorf("NormalizeFindingText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// the separator keeps the two halves from colliding across the boundary
	a := FindingFingerprint("ab", "c")
	b := FindingFingerprint("a", "bc")
	if a == b {
		t.Errorf("fingerprint(%q,%q) == fingerprint(%q,%q) — the 0x1F separator is not doing its job", "ab", "c", "a", "bc")
	}
	if a != FindingFingerprint("AB", "C") {
		t.Error("fingerprint must be case-insensitive after normalization")
	}
}

// SG-126: object-form reasoning (qa-dagger rows carry {"note": ...}) is
// fingerprinted on the note, and a row whose reasoning is an object can be
// compared against a plain-string reasoning in a later create.
func TestRowFindingReasoningHandlesObjectForm(t *testing.T) {
	row, err := ParseRow([]byte(`{"id":"QA-X-1","title":"t","reasoning":{"note":"QA foreman cycle 2026-09-10"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := RowFindingReasoning(row); got != "QA foreman cycle 2026-09-10" {
		t.Fatalf("RowFindingReasoning(object) = %q", got)
	}
	plain, err := ParseRow([]byte(`{"id":"QA-X-2","title":"t","reasoning":"QA foreman cycle 2026-09-10"}`))
	if err != nil {
		t.Fatal(err)
	}
	if FindingFingerprintForRow(row) != FindingFingerprintForRow(plain) {
		t.Error("object-form and string-form reasoning with the same note must fingerprint identically")
	}
	nullRow, err := ParseRow([]byte(`{"id":"QA-X-3","title":"t","reasoning":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := RowFindingReasoning(nullRow); got != "" {
		t.Fatalf("null reasoning = %q, want empty", got)
	}
}

// SG-126 AC2: re-filing a finding under a fresh, suffix-variant id is REFUSED
// before anything is written, and the refusal is recorded as a task_evidence
// event on the existing row (the board gains an audit line, not a row).
func TestCreate_DedupeOnFingerprint(t *testing.T) {
	b := newTestBoard(t)
	title := "[P2] Widget crashes on empty input"
	reasoning := "probe: curl returns 500 on empty payload"
	if _, err := b.Create(TaskRowSpec{ID: "QA-TEST-1", Title: title, Reasoning: reasoning, EvidenceRunID: "run-1"}); err != nil {
		t.Fatalf("first filing rejected: %v", err)
	}
	fp := detailFingerprint(t, taskRowByID(t, b, "QA-TEST-1"))
	lineABefore := taskLineByID(t, b, "QA-TEST-1")
	tasksBefore := boardLineCount(t, b.TasksPath())
	eventsBefore := boardLineCount(t, b.EventsPath())

	// Same finding: recycled id prefix on the title, recurrence suffix, and
	// whitespace-only differences in the reasoning.
	_, err := b.Create(TaskRowSpec{
		ID:        "QA-TEST-2",
		Title:     "QA-TEST-9 " + title + " (recurrence 2)",
		Reasoning: "probe:   curl returns\n500 on empty payload",
	})
	if err == nil {
		t.Fatal("re-filed finding accepted — the fingerprint gate did not fire")
	}
	var dup *ErrDuplicateFinding
	if !errors.As(err, &dup) {
		t.Fatalf("error type = %T (%v), want *ErrDuplicateFinding", err, err)
	}
	if dup.ExistingID != "QA-TEST-1" {
		t.Fatalf("ExistingID = %q, want QA-TEST-1", dup.ExistingID)
	}
	if dup.Fingerprint != fp {
		t.Fatalf("Fingerprint = %q, want %q (the stored digest)", dup.Fingerprint, fp)
	}
	if want := "DUPLICATE-SUPPRESSED: QA-TEST-1 matches fingerprint " + fp; !strings.Contains(err.Error(), want) {
		t.Fatalf("error message %q does not start the machine-readable line %q", err.Error(), want)
	}

	// The board did not grow, and the existing row is byte-identical.
	if n := boardLineCount(t, b.TasksPath()); n != tasksBefore {
		t.Fatalf("tasks.jsonl grew from %d to %d lines on a suppressed duplicate", tasksBefore, n)
	}
	if got := taskLineByID(t, b, "QA-TEST-1"); got != lineABefore {
		t.Fatalf("existing row was rewritten by a suppressed create:\n got %s\nwant %s", got, lineABefore)
	}

	// The suppression is durable: exactly one task_evidence event on the
	// existing row, carrying both ids and the fingerprint.
	if n := boardLineCount(t, b.EventsPath()); n != eventsBefore+1 {
		t.Fatalf("events.jsonl has %d lines, want %d (exactly one task_evidence event)", n, eventsBefore+1)
	}
	e := lastEventRaw(t, b)
	if e["event_type"] != "task_evidence" {
		t.Fatalf("event_type = %v, want task_evidence", e["event_type"])
	}
	if e["task_id"] != "QA-TEST-1" {
		t.Fatalf("event task_id = %v, want the EXISTING row QA-TEST-1", e["task_id"])
	}
	d := eventDetailMap(t, e)
	if d["existing_task_id"] != "QA-TEST-1" || d["new_id_attempted"] != "QA-TEST-2" || d["fingerprint"] != fp {
		t.Fatalf("task_evidence detail wrong: %v", d)
	}
	ev, ok := d["evidence"].(map[string]any)
	if !ok {
		t.Fatalf("task_evidence detail carries no evidence object: %v", d)
	}
	if ts, _ := ev["ts"].(string); ts == "" {
		t.Fatalf("evidence has no ts: %v", ev)
	}

	// The open-fingerprint index the gate consults sees exactly the one row.
	fps, err := b.OpenFingerprints()
	if err != nil {
		t.Fatal(err)
	}
	if len(fps) != 1 || fps["QA-TEST-1"] != fp {
		t.Fatalf("OpenFingerprints() = %v, want {QA-TEST-1: %s}", fps, fp)
	}
}

// SG-126 AC3: a pre-existing OPEN row with no fingerprint is grandfathered —
// it stays open and visible but never produces a false match, so the new
// filing lands.
func TestCreate_GrandfatherExistingOpenRow(t *testing.T) {
	title := "[P1] Legacy finding filed before fingerprints existed"
	reasoning := "cell detail: spawn failed"
	b := seedFindingBoard(t,
		`{"id":"QA-LEGACY-1","title":"`+title+`","status":"pending","priority":"P1","reasoning":"`+reasoning+`"}`,
	)
	if fps, err := b.OpenFingerprints(); err != nil || len(fps) != 0 {
		t.Fatalf("seeded legacy row unexpectedly fingerprinted: %v (%v)", fps, err)
	}
	tasksBefore := boardLineCount(t, b.TasksPath())

	got, err := b.Create(TaskRowSpec{ID: "QA-NEW-1", Title: title, Reasoning: reasoning})
	if err != nil {
		t.Fatalf("create against a grandfathered open row was refused: %v", err)
	}
	if got != "QA-NEW-1" {
		t.Fatalf("created id = %q, want QA-NEW-1", got)
	}
	if n := boardLineCount(t, b.TasksPath()); n != tasksBefore+1 {
		t.Fatalf("tasks.jsonl has %d lines, want %d", n, tasksBefore+1)
	}
	// the legacy row is untouched and still unfingerprinted
	legacy := taskRowByID(t, b, "QA-LEGACY-1")
	if legacy["detail"] != nil {
		t.Fatalf("grandfathered row was rewritten: %v", legacy["detail"])
	}
	// the NEW row is fingerprinted, so the NEXT re-file is suppressed
	if fp := detailFingerprint(t, taskRowByID(t, b, "QA-NEW-1")); fp == "" {
		t.Fatal("new row carries no fingerprint")
	}
	if _, err := b.Create(TaskRowSpec{ID: "QA-NEW-2", Title: title, Reasoning: reasoning}); err == nil {
		t.Fatal("second re-file accepted after the finding became fingerprinted")
	}
}

// SG-126 AC4: --force files a variant deliberately, and the variant is itself
// fingerprinted so a later accidental re-file of it is still caught.
func TestCreate_ForceOverride(t *testing.T) {
	b := newTestBoard(t)
	title := "[P2] Widget crashes on empty input"
	reasoning := "probe: curl returns 500"
	if _, err := b.Create(TaskRowSpec{ID: "QA-TEST-1", Title: title, Reasoning: reasoning}); err != nil {
		t.Fatal(err)
	}
	fp := detailFingerprint(t, taskRowByID(t, b, "QA-TEST-1"))
	if _, err := b.Create(TaskRowSpec{ID: "QA-TEST-2", Title: title, Reasoning: reasoning, Force: true}); err != nil {
		t.Fatalf("--force duplicate filing refused: %v", err)
	}
	tasks, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range tasks {
		ids[r.String("id")] = true
	}
	if !ids["QA-TEST-1"] || !ids["QA-TEST-2"] {
		t.Fatalf("forced variant did not land: board ids %v (want QA-TEST-1 and QA-TEST-2)", ids)
	}
	if got := detailFingerprint(t, taskRowByID(t, b, "QA-TEST-2")); got != fp {
		t.Fatalf("forced row fingerprint = %q, want %q (variants are still fingerprinted)", got, fp)
	}
	// the forced create is an ordinary create: its event is task_created
	if e := lastEventRaw(t, b); e["event_type"] != "task_created" {
		t.Fatalf("last event = %v, want task_created", e["event_type"])
	}
}

// SG-126 AC5: the backfill collapses a 3-row fingerprint collision into ONE
// open row plus two closed rows whose worker_summary names the kept id and
// whose superseded_by marks the merge machine-readably, carries the merged
// evidence onto the kept row, and writes one audit event per group.
func TestDedupeBackfill_MergesCollisions(t *testing.T) {
	finding := `"title":"[P3] run_battery FAIL — port pool exhausted","reasoning":"cell detail: no free port ranges"`
	b := seedFindingBoard(t,
		`{"id":"QA-BF-1","status":"pending","priority":"P3",`+finding+`}`,
		`{"id":"OTHER-1","status":"complete","priority":"P2","title":"unrelated","reasoning":"nope"}`,
		`{"id":"QA-BF-2","status":"pending","priority":"P3",`+finding+`}`,
		`{"id":"QA-BF-3","status":"in_progress","priority":"P3",`+finding+`}`,
		`{"id":"QA-BF-4","status":"complete","priority":"P3",`+finding+`}`,
	)
	otherBefore := taskLineByID(t, b, "OTHER-1")
	closedBefore := taskLineByID(t, b, "QA-BF-4")
	eventsBefore := boardLineCount(t, b.EventsPath())

	rep, err := b.DedupeBackfill(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("merge groups = %d, want 1 (%+v)", len(rep.Groups), rep.Groups)
	}
	g := rep.Groups[0]
	if g.Kept != "QA-BF-1" {
		t.Fatalf("kept = %q, want the earliest filing QA-BF-1", g.Kept)
	}
	if strings.Join(g.Merged, ",") != "QA-BF-2,QA-BF-3" {
		t.Fatalf("merged = %v, want [QA-BF-2 QA-BF-3]", g.Merged)
	}
	if g.Fingerprint == "" {
		t.Fatal("merge group carries no fingerprint")
	}

	// kept row: still open, still the same finding, now carries the merged
	// rows' evidence.
	kept := taskRowByID(t, b, "QA-BF-1")
	if kept["status"] != "pending" {
		t.Fatalf("kept row status = %v, want pending", kept["status"])
	}
	kd, _ := kept["detail"].(map[string]any)
	if kd["fingerprint"] != g.Fingerprint {
		t.Fatalf("kept row fingerprint = %v, want %s", kd["fingerprint"], g.Fingerprint)
	}
	ev, _ := kd["evidence"].([]any)
	if len(ev) != 2 {
		t.Fatalf("kept row evidence = %v, want one entry per merged row", kd["evidence"])
	}
	seen := map[string]bool{}
	for _, e := range ev {
		em, _ := e.(map[string]any)
		id, _ := em["task_id"].(string)
		seen[id] = true
	}
	if !seen["QA-BF-2"] || !seen["QA-BF-3"] {
		t.Fatalf("merged evidence does not name both merged rows: %v", ev)
	}

	// merged-away rows: complete, summary names the kept id, completed_at set,
	// superseded_by marks the merge (REVIEW-BOARDCTL-001 — machine-readable,
	// not prose-parsed).
	for _, id := range []string{"QA-BF-2", "QA-BF-3"} {
		row := taskRowByID(t, b, id)
		if row["status"] != "complete" {
			t.Fatalf("%s status = %v, want complete", id, row["status"])
		}
		summary, _ := row["worker_summary"].(string)
		if !strings.HasPrefix(summary, "merged into QA-BF-1: dedupe backfill ") {
			t.Fatalf("%s worker_summary = %q, want \"merged into QA-BF-1: dedupe backfill <date>\"", id, summary)
		}
		if ts, _ := row["completed_at"].(string); ts == "" {
			t.Fatalf("%s has no completed_at", id)
		}
		if sb, _ := row["superseded_by"].(string); sb != "QA-BF-1" {
			t.Fatalf("%s superseded_by = %v, want \"QA-BF-1\" (the kept row id)", id, row["superseded_by"])
		}
	}

	// untouched lines round-trip byte-identical: a closed row with the same
	// finding is NOT a merge candidate, and an unrelated row is not rewritten.
	if got := taskLineByID(t, b, "QA-BF-4"); got != closedBefore {
		t.Fatalf("closed row was rewritten:\n got %s\nwant %s", got, closedBefore)
	}
	if got := taskLineByID(t, b, "OTHER-1"); got != otherBefore {
		t.Fatalf("unrelated row was rewritten:\n got %s\nwant %s", got, otherBefore)
	}

	// exactly one audit event, naming kept + merged + fingerprint
	if n := boardLineCount(t, b.EventsPath()); n != eventsBefore+1 {
		t.Fatalf("events.jsonl has %d lines, want %d", n, eventsBefore+1)
	}
	e := lastEventRaw(t, b)
	if e["event_type"] != "audit" || e["task_id"] != "QA-BF-1" {
		t.Fatalf("audit event wrong: %v", e)
	}
	d := eventDetailMap(t, e)
	if d["action"] != "dedupe-backfill" || d["kept"] != "QA-BF-1" || d["fingerprint"] != g.Fingerprint {
		t.Fatalf("audit detail wrong: %v", d)
	}
	merged, _ := d["merged"].([]any)
	if len(merged) != 2 || merged[0] != "QA-BF-2" || merged[1] != "QA-BF-3" {
		t.Fatalf("audit merged = %v, want [QA-BF-2 QA-BF-3]", d["merged"])
	}

	// idempotence: a second apply finds nothing to collapse and writes nothing.
	tasksAfter := mustRead(t, b.TasksPath())
	eventsAfter := mustRead(t, b.EventsPath())
	rep2, err := b.DedupeBackfill(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep2.Groups) != 0 || len(rep2.Fingerprinted) != 0 {
		t.Fatalf("second apply is not a no-op: %+v", rep2)
	}
	if string(mustRead(t, b.TasksPath())) != string(tasksAfter) || string(mustRead(t, b.EventsPath())) != string(eventsAfter) {
		t.Fatal("second apply rewrote a file")
	}
}

// SG-126 AC6: the dry run reports the identical collapse and leaves BOTH files
// byte-identical — no fingerprints, no merges, no audit events.
func TestDedupeBackfill_DryRunIsIdempotent(t *testing.T) {
	finding := `"title":"[P1] Leak fix not deployed","reasoning":"bunker version -> 0.1.3"`
	b := seedFindingBoard(t,
		`{"id":"QA-BF-1","status":"pending","priority":"P1",`+finding+`}`,
		`{"id":"QA-BF-2","status":"pending","priority":"P1",`+finding+`}`,
		`{"id":"QA-BF-3","status":"pending","priority":"P1",`+finding+`}`,
	)
	tasksBefore := mustRead(t, b.TasksPath())
	eventsBefore := mustRead(t, b.EventsPath())

	rep, err := b.DedupeBackfill(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Groups) != 1 || rep.Groups[0].Kept != "QA-BF-1" {
		t.Fatalf("dry-run report does not describe the collapse: %+v", rep.Groups)
	}
	if len(rep.MergedAway) != 2 {
		t.Fatalf("dry-run merged-away = %v, want 2 ids", rep.MergedAway)
	}
	if !rep.Changed() {
		t.Fatal("dry-run report claims nothing to do while a group exists")
	}
	if string(mustRead(t, b.TasksPath())) != string(tasksBefore) {
		t.Fatal("dry run rewrote tasks.jsonl")
	}
	if string(mustRead(t, b.EventsPath())) != string(eventsBefore) {
		t.Fatal("dry run wrote an audit event")
	}
	// and the report is stable across runs
	rep2, err := b.DedupeBackfill(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep2.Groups) != 1 || rep2.Groups[0].Kept != rep.Groups[0].Kept {
		t.Fatalf("dry-run report is not deterministic: %+v vs %+v", rep.Groups, rep2.Groups)
	}
}

// SG-126 regression (found on real boards): live QA boards put the SAME id on
// several lines, so the collapse must select group members by LINE INDEX. An
// id-keyed selection would treat both same-id rows as "kept" and merge neither,
// leaving the duplicate open while the report claimed a merge.
func TestDedupeBackfill_DuplicateIDsCollapse(t *testing.T) {
	finding := `"title":"[P2] Untracked .vfs cache stray in the repo","reasoning":"git status shows .vfs/"`
	b := seedFindingBoard(t,
		`{"id":"QA-DUP-1","status":"pending","priority":"P2",`+finding+`}`,
		`{"id":"QA-DUP-1","status":"pending","priority":"P2",`+finding+`}`,
	)
	rep, err := b.DedupeBackfill(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("merge groups = %d, want 1 (%+v)", len(rep.Groups), rep.Groups)
	}
	g := rep.Groups[0]
	if !g.DuplicateIDs {
		t.Fatalf("group with two same-id rows not flagged DuplicateIDs: %+v", g)
	}
	if g.KeptLine != 1 || len(g.MergedLines) != 1 || g.MergedLines[0] != 2 {
		t.Fatalf("group lines = kept %d merged %v, want kept 1 merged [2]", g.KeptLine, g.MergedLines)
	}
	if g.Kept != "QA-DUP-1" || len(g.Merged) != 1 || g.Merged[0] != "QA-DUP-1" {
		t.Fatalf("group ids = kept %q merged %v", g.Kept, g.Merged)
	}

	// exactly one of the two same-id rows is left OPEN; the other is complete.
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	var openCount, closedCount int
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatal(err)
		}
		if row["id"] != "QA-DUP-1" {
			continue
		}
		switch row["status"] {
		case "pending":
			openCount++
		case "complete":
			closedCount++
			summary, _ := row["worker_summary"].(string)
			if !strings.HasPrefix(summary, "merged into QA-DUP-1: dedupe backfill ") {
				t.Fatalf("merged same-id row summary = %q", summary)
			}
			// REVIEW-BOARDCTL-001: the marker names the KEPT row id; here the
			// kept row shares the id, so the marker equals it too.
			if sb, _ := row["superseded_by"].(string); sb != "QA-DUP-1" {
				t.Fatalf("merged same-id row superseded_by = %v, want \"QA-DUP-1\"", row["superseded_by"])
			}
		default:
			t.Fatalf("unexpected status %v", row["status"])
		}
	}
	if openCount != 1 || closedCount != 1 {
		t.Fatalf("same-id group left %d open / %d closed, want 1 / 1", openCount, closedCount)
	}

	// the audit event disambiguates by line
	d := eventDetailMap(t, lastEventRaw(t, b))
	if d["kept_line"] != float64(1) {
		t.Fatalf("audit kept_line = %v, want 1", d["kept_line"])
	}
	ml, _ := d["merged_lines"].([]any)
	if len(ml) != 1 || ml[0] != float64(2) {
		t.Fatalf("audit merged_lines = %v, want [2]", d["merged_lines"])
	}
}

// mustRead reads a file or fails the test.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
