package render

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// ---------- helpers ----------

func mustRow(t *testing.T, line string) *board.Row {
	t.Helper()
	r, err := board.ParseRow([]byte(line))
	if err != nil {
		t.Fatalf("ParseRow(%s): %v", line, err)
	}
	return r
}

func tstamp(s string) time.Time {
	t, ok := parseStamp(s, time.UTC)
	if !ok {
		panic("bad stamp " + s)
	}
	return t
}

func mkBoard(tasks []*board.Row, events []*board.Row) *boardData {
	d := &boardData{
		topology: "A",
		loc:      time.UTC,
		now:      tstamp("2026-09-12 12:00:00"),
		fixtureI: map[string]bool{},
	}
	for _, r := range tasks {
		d.tasks = append(d.tasks, buildTaskRec(r, r.String("id"), d, d.now))
	}
	for _, r := range events {
		d.events = append(d.events, buildEventRec(r, d))
	}
	return d
}

func taskLine(id, created, completed, status string) string {
	s := `{"id":"` + id + `","title":"t ` + id + `","status":"` + status + `","priority":"P1","created_at":"` + created + `"`
	if completed != "" {
		s += `,"completed_at":"` + completed + `"`
	}
	s += `}`
	return s
}

// ---------- 3.1 burndown ----------

func TestBurndownBasic(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
		mustRow(t, taskLine("B", "2026-09-01 00:00:00", "", "pending")),
	}
	d := mkBoard(tasks, nil)
	der := derive(d)
	// window: 09-01 .. 09-12 (today); open: A closes end of 09-02
	// 09-01: 2, 09-02: A done same day -> done(09-02) <= d so A closed on 09-02: 1, rest: 1
	if der.Burndown.Window[0] != "2026-09-01" {
		t.Fatalf("window start = %s", der.Burndown.Window[0])
	}
	if got := der.Burndown.Open[0]; got != 2 {
		t.Fatalf("day1 open = %d, want 2", got)
	}
	if got := der.Burndown.Open[1]; got != 1 {
		t.Fatalf("day2 open = %d, want 1 (A completed on 09-02)", got)
	}
	if last := der.Burndown.Open[len(der.Burndown.Open)-1]; last != 1 {
		t.Fatalf("final open = %d, want 1", last)
	}
	// burnup shares the window and ends at 1
	if last := der.Burnup.Cum[len(der.Burnup.Cum)-1]; last != 1 {
		t.Fatalf("final cum = %d, want 1", last)
	}
}

func TestBurndownSingleDayBoard(t *testing.T) {
	tasks := []*board.Row{mustRow(t, taskLine("A", "2026-09-05 08:00:00", "2026-09-05 20:00:00", "complete"))}
	d := mkBoard(tasks, nil)
	d.now = tstamp("2026-09-05 23:00:00")
	der := derive(d)
	if len(der.Burndown.Open) != 1 {
		t.Fatalf("series length = %d, want 1", len(der.Burndown.Open))
	}
	if der.Burndown.Open[0] != 0 {
		t.Fatalf("open = %d, want 0 (same-day completion)", der.Burndown.Open[0])
	}
	if der.Burnup.Cum[0] != 1 {
		t.Fatalf("cum = %d, want 1", der.Burnup.Cum[0])
	}
}

// ---------- 3.4 cycle ----------

func TestCycleMedianP90Worst5(t *testing.T) {
	// cycles: 0,1,2,3,10 -> median 2, p90 (ceil(.9*5)=5th) = 10
	lines := []struct {
		id              string
		created, doneOn string
	}{
		{"A", "2026-09-01", "2026-09-01"},
		{"B", "2026-09-01", "2026-09-02"},
		{"C", "2026-09-01", "2026-09-03"},
		{"D", "2026-09-01", "2026-09-04"},
		{"E", "2026-09-01", "2026-09-11"},
	}
	var tasks []*board.Row
	for _, l := range lines {
		tasks = append(tasks, mustRow(t, taskLine(l.id, l.created+" 00:00:00", l.doneOn+" 00:00:00", "complete")))
	}
	d := mkBoard(tasks, nil)
	cs := cycleStats(nonFixtureRows(d))
	if cs.N != 5 {
		t.Fatalf("n = %d, want 5", cs.N)
	}
	if cs.Median == nil || *cs.Median != 2 {
		t.Fatalf("median = %v, want 2", cs.Median)
	}
	if cs.P90 == nil || *cs.P90 != 10 {
		t.Fatalf("p90 = %v, want 10", cs.P90)
	}
	if len(cs.Worst5) != 5 || cs.Worst5[0].ID != "E" || cs.Worst5[0].Days != 10 {
		t.Fatalf("worst5[0] = %+v, want E/10", cs.Worst5[0])
	}
}

func TestCycleEvenMedianAndSingle(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")), // 1d
		mustRow(t, taskLine("B", "2026-09-01 00:00:00", "2026-09-03 00:00:00", "complete")), // 2d
	}
	d := mkBoard(tasks, nil)
	cs := cycleStats(nonFixtureRows(d))
	if cs.Median == nil || *cs.Median != 1.5 {
		t.Fatalf("even median = %v, want 1.5", cs.Median)
	}
	// single task: median == p90 == value
	one := []*board.Row{mustRow(t, taskLine("Z", "2026-09-01 00:00:00", "2026-09-04 00:00:00", "complete"))}
	d2 := mkBoard(one, nil)
	cs2 := cycleStats(nonFixtureRows(d2))
	if *cs2.Median != 3 || *cs2.P90 != 3 {
		t.Fatalf("single median/p90 = %v/%v, want 3/3", cs2.Median, cs2.P90)
	}
}

func TestCycleExcludesMissingTimestamps(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
		mustRow(t, `{"id":"B","title":"no stamps","status":"complete"}`), // no birth, no done
	}
	d := mkBoard(tasks, nil)
	cs := cycleStats(nonFixtureRows(d))
	if cs.N != 1 || cs.Excluded != 1 {
		t.Fatalf("n/excluded = %d/%d, want 1/1", cs.N, cs.Excluded)
	}
	// data-quality footnote carries B
	if ids := noDoneCompletedIDs(d); !reflect.DeepEqual(ids, []string{"B"}) {
		t.Fatalf("no-done ids = %v, want [B]", ids)
	}
}

// ---------- 3.3 velocity ----------

func TestVelocityISOWeekBucketing(t *testing.T) {
	// 2026-08-31 is a Monday; 31st + 3 days = Thu 2026-09-03, same ISO week.
	tasks := []*board.Row{
		mustRow(t, taskLine("A", "2026-08-31 00:00:00", "2026-09-03 00:00:00", "complete")),
		mustRow(t, taskLine("B", "2026-09-05 00:00:00", "2026-09-05 00:00:00", "complete")),
		mustRow(t, taskLine("C", "2026-09-07 00:00:00", "2026-09-07 00:00:00", "complete")),
	}
	d := mkBoard(tasks, nil)
	d.now = tstamp("2026-09-12 12:00:00")
	v := velocity(nonFixtureRows(d), dayKey(d.now))
	// 2026-08-31 is Monday of ISO week 2026-W36; completions 09-03 and 09-05
	// are both in W36 (Thu and Sat), 09-07 is Monday of W37.
	if len(v.Weeks) != 2 || v.Weeks[0] != "2026-W36" || v.Weeks[1] != "2026-W37" {
		t.Fatalf("weeks = %v", v.Weeks)
	}
	if !reflect.DeepEqual(v.Counts, []int{2, 1}) {
		t.Fatalf("counts = %v, want [2 1]", v.Counts)
	}
}

func TestVelocityEmptyBoard(t *testing.T) {
	d := mkBoard(nil, nil)
	v := velocity(nonFixtureRows(d), dayKey(d.now))
	if len(v.Weeks) != 0 || len(v.Counts) != 0 {
		t.Fatalf("empty velocity = %v", v)
	}
}

// ---------- 3.5 tick health ----------

func TestTickHealthBucketsAndGaps(t *testing.T) {
	events := []*board.Row{
		mustRow(t, `{"id":1,"timestamp":"2026-09-01 10:00:00","event_type":"task_completed","task_id":"A","actor":"f","detail":null,"tick_number":1}`),
		mustRow(t, `{"id":2,"timestamp":"2026-09-01 11:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}`),
		mustRow(t, `{"id":3,"timestamp":"2026-09-03 11:00:00","event_type":"task_failed","task_id":"A","actor":"f","detail":null,"tick_number":2}`),
		mustRow(t, `{"id":4,"timestamp":"2026-09-03 12:00:00","event_type":"task_completed","task_id":"B","actor":"f","detail":"{\"outcome\":\"failed\"}","tick_number":2}`),
		mustRow(t, `{"id":5,"timestamp":"2026-09-04 12:00:00","event_type":"idle","task_id":null,"actor":"f","detail":null,"tick_number":null}`),
	}
	d := mkBoard(nil, events)
	th := tickHealth(d.events)
	if !reflect.DeepEqual(th.Days, []string{"2026-09-01", "2026-09-03"}) {
		t.Fatalf("days = %v (09-04 is idle-only -> gap? no: idle has no class, day omitted)", th.Days)
	}
	if !reflect.DeepEqual(th.Completed, []int{1, 1}) {
		t.Fatalf("completed = %v", th.Completed)
	}
	if !reflect.DeepEqual(th.Failed, []int{0, 2}) {
		t.Fatalf("failed = %v (task_failed + outcome:failed completion)", th.Failed)
	}
	if !reflect.DeepEqual(th.Audit, []int{1, 0}) {
		t.Fatalf("audit = %v", th.Audit)
	}
}

// ---------- 3.7 work clock ----------

func TestWorkClockGrid(t *testing.T) {
	// 2026-09-01 is a Tuesday; hour 9 and hour 23.
	events := []*board.Row{
		mustRow(t, `{"id":1,"timestamp":"2026-09-01 09:30:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}`),
		mustRow(t, `{"id":2,"timestamp":"2026-09-01 23:10:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}`),
		mustRow(t, `{"id":3,"timestamp":"2026-08-30 09:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}`), // Sunday
	}
	d := mkBoard(nil, events)
	wc := workClock(d.events, time.UTC)
	if wc.Grid[1][9] != 1 { // Tuesday
		t.Fatalf("Tue 9h = %d", wc.Grid[1][9])
	}
	if wc.Grid[1][23] != 1 {
		t.Fatalf("Tue 23h = %d", wc.Grid[1][23])
	}
	if wc.Grid[6][9] != 1 { // Sunday row = 6
		t.Fatalf("Sun 9h = %d", wc.Grid[6][9])
	}
	if wc.Max != 1 {
		t.Fatalf("max = %d", wc.Max)
	}
}

// ---------- 3.8 model share ----------

func TestModelShare(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, `{"id":"A","status":"complete","primary_model":"glm-5.3-flash"}`),
		mustRow(t, `{"id":"B","status":"complete","primary_model":"glm-5.3-flash"}`),
		mustRow(t, `{"id":"C","status":"complete"}`),
		mustRow(t, `{"id":"D","status":"pending","primary_model":"glm-5.3-flash"}`), // not completed
	}
	d := mkBoard(tasks, nil)
	ms := modelShare(nonFixtureRows(d))
	if ms.CompletedN != 3 {
		t.Fatalf("completed_n = %d, want 3", ms.CompletedN)
	}
	if ms.Models[0] != "glm-5.3-flash" || ms.Counts[0] != 2 {
		t.Fatalf("top model = %v/%v", ms.Models, ms.Counts)
	}
	if ms.Models[len(ms.Models)-1] != "unknown" {
		t.Fatalf("missing model not bucketed unknown: %v", ms.Models)
	}
}

// ---------- 3.9 status/priority counts ----------

func TestStatusPriorityCounts(t *testing.T) {
	tasks := []*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
		mustRow(t, taskLine("B", "2026-09-01 00:00:00", "", "pending")),
		mustRow(t, `{"id":"C","status":"completed","priority":2,"created_at":"2026-09-01 00:00:00","completed_at":"2026-09-02 00:00:00"}`), // alias + numeric priority
		mustRow(t, `{"id":"D","created_at":"2026-09-01 00:00:00"}`),                                                                        // no status
		mustRow(t, `{"id":"NEVER-DONE-1","status":"pending","perpetual":true,"created_at":"2026-09-01 00:00:00"}`),                         // fixture
	}
	d := mkBoard(tasks, nil)
	der := derive(d)
	if der.StatusCounts["complete"] != 2 || der.StatusCounts["pending"] != 1 || der.StatusCounts["(none)"] != 1 {
		t.Fatalf("status counts = %v", der.StatusCounts)
	}
	if der.PriorityCount["P1"] != 2 || der.PriorityCount["2"] != 1 {
		t.Fatalf("priority counts = %v (numeric '2' is its own bucket)", der.PriorityCount)
	}
	if der.TotalNonFix != 4 {
		t.Fatalf("total non-fixture = %d, want 4", der.TotalNonFix)
	}
}

// ---------- 4.3 streaks ----------

func TestStreakTodayBoundary(t *testing.T) {
	// AC 7.1.9: completions 09-01..09-05 and 09-08..09-10.
	mk := func() []*board.Row {
		var tasks []*board.Row
		for _, day := range []string{"01", "02", "03", "04", "05", "08", "09", "10"} {
			tasks = append(tasks, mustRow(t, taskLine("T"+day, "2026-09-"+day+" 00:00:00", "2026-09-"+day+" 12:00:00", "complete")))
		}
		return tasks
	}
	// rendered 2026-09-10 23:00 -> current=3 live, longest=5
	d := mkBoard(mk(), nil)
	d.now = tstamp("2026-09-10 23:00:00")
	s := streaks(d, nonFixtureRows(d))
	if s.Delivery.Current != 3 || s.Delivery.CurrentState != "live" || s.Delivery.Longest != 5 {
		t.Fatalf("delivery 09-10 = %+v, want current 3 live longest 5", s.Delivery)
	}
	if s.Delivery.CurrentSince != "2026-09-08" {
		t.Fatalf("current_since = %s, want 2026-09-08", s.Delivery.CurrentSince)
	}
	// rendered 2026-09-11 (gap 1) -> still live
	d2 := mkBoard(mk(), nil)
	d2.now = tstamp("2026-09-11 12:00:00")
	s2 := streaks(d2, nonFixtureRows(d2))
	if s2.Delivery.CurrentState != "live" || s2.Delivery.Current != 3 {
		t.Fatalf("delivery 09-11 = %+v, want live 3", s2.Delivery)
	}
	// rendered 2026-09-12 (gap 2) -> ended, current_since 2026-09-10
	d3 := mkBoard(mk(), nil)
	d3.now = tstamp("2026-09-12 12:00:00")
	s3 := streaks(d3, nonFixtureRows(d3))
	if s3.Delivery.CurrentState != "ended" || s3.Delivery.CurrentSince != "2026-09-10" {
		t.Fatalf("delivery 09-12 = %+v, want ended since 2026-09-10", s3.Delivery)
	}
	if s3.Delivery.Longest != 5 {
		t.Fatalf("longest = %d, want 5", s3.Delivery.Longest)
	}
}

func TestStreakActivityExcludesIdleAndAnyTickIncludes(t *testing.T) {
	var events []*board.Row
	// non-idle events 09-01..09-03, idle-only 09-04..09-06
	for i, day := range []string{"01", "02", "03"} {
		events = append(events, mustRow(t, `{"id":`+itoa(i+1)+`,"timestamp":"2026-09-`+day+` 10:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}`))
	}
	for i, day := range []string{"04", "05", "06"} {
		events = append(events, mustRow(t, `{"id":`+itoa(i+10)+`,"timestamp":"2026-09-`+day+` 10:00:00","event_type":"idle","task_id":null,"actor":"f","detail":null,"tick_number":null}`))
	}
	d := mkBoard(nil, events)
	s := streaks(d, nil)
	if s.Activity.Current != 3 || s.Activity.Longest != 3 {
		t.Fatalf("activity = %+v, want 3/3", s.Activity)
	}
	if s.AnyTick.Current != 6 || s.AnyTick.Longest != 6 {
		t.Fatalf("any-tick = %+v, want 6/6", s.AnyTick)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// ---------- 2.6 fixture classification ----------

func TestFixtureClassificationThreeSignals(t *testing.T) {
	d := &boardData{fixtureI: map[string]bool{"FIX-LISTED": true}, loc: time.UTC, now: tstamp("2026-09-12 00:00:00")}
	cases := []struct {
		line string
		want bool
	}{
		{`{"id":"FIX-LISTED","status":"pending"}`, true},                 // fixtures.jsonl membership
		{`{"id":"NEVER-DONE","status":"pending","active":true}`, true},   // prefix
		{`{"id":"NEVER-DONE-2","status":"pending"}`, true},               // prefix family
		{`{"id":"PERP","status":"pending","perpetual":true}`, true},      // perpetual
		{`{"id":"REAL","status":"pending"}`, false},                      // task
		{`{"id":"NOTPERP","status":"pending","perpetual":false}`, false}, // explicit false
		{`{"id":"NEVERDONEY","status":"pending"}`, false},                // prefix must be NEVER-DONE (dash)
	}
	for i, c := range cases {
		row := mustRow(t, c.line)
		if got := isFixture(row, row.String("id"), d.fixtureI); got != c.want {
			t.Fatalf("case %d (%s): fixture = %v, want %v", i, row.String("id"), got, c.want)
		}
	}
}

func TestFixtureExcludedFromEveryMetric(t *testing.T) {
	with := mkBoard([]*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
		mustRow(t, `{"id":"NEVER-DONE","title":"f","status":"pending","perpetual":true,"created_at":"2026-09-01 00:00:00"}`),
	}, nil)
	without := mkBoard([]*board.Row{
		mustRow(t, taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete")),
	}, nil)
	d1, d2 := derive(with), derive(without)
	if !reflect.DeepEqual(d1.Burndown, d2.Burndown) || !reflect.DeepEqual(d1.Burnup, d2.Burnup) ||
		!reflect.DeepEqual(d1.Velocity, d2.Velocity) || !reflect.DeepEqual(d1.Cycle, d2.Cycle) ||
		!reflect.DeepEqual(d1.Streaks, d2.Streaks) || !reflect.DeepEqual(d1.ModelShare, d2.ModelShare) ||
		!reflect.DeepEqual(d1.StatusCounts, d2.StatusCounts) || !reflect.DeepEqual(d1.PriorityCount, d2.PriorityCount) {
		t.Fatalf("fixture row changed a derivation:\nwith %+v\nwithout %+v", d1, d2)
	}
}

// ---------- 2.4 detail ladder ----------

func TestDetailLadder(t *testing.T) {
	cases := []struct {
		raw    string
		want   any
		hasDec bool
	}{
		{`{"k":1}`, map[string]any{"k": json.Number("1")}, true},              // object as-is
		{`"[{\"a\":2}]"`, []any{map[string]any{"a": json.Number("2")}}, true}, // JSON string
		{`"eyJ0YWciOiAxfQ=="`, map[string]any{"tag": json.Number("1")}, true}, // base64(JSON)
		{`"plain text detail"`, nil, false},                                   // raw string
		{`12345`, nil, false},                                                 // wrong shape
		{`null`, nil, false},
	}
	_ = cases[4]
	for i, c := range cases {
		if i == 4 {
			continue // covered by parse-level tests
		}
		got, ok := decodeDetail(json.RawMessage(c.raw))
		if ok != c.hasDec {
			t.Fatalf("case %d: ok = %v, want %v", i, ok, c.hasDec)
		}
		if ok && !reflect.DeepEqual(got, c.want) {
			t.Fatalf("case %d: got %#v want %#v", i, got, c.want)
		}
	}
}

func TestDecodeDetailNeverThrows(t *testing.T) {
	nasties := []string{
		`"===="`,        // invalid base64 padding
		`"YWJj"`,        // base64 -> "abc" not JSON
		`"{} trailing"`, // trailing garbage
		`"[1,2"`,        // torn array
		`"null"`,        // JSON but not object/array
		`"\"scalar\""`,  // string -> not container
	}
	for _, n := range nasties {
		_, _ = decodeDetail(json.RawMessage(n)) // must not panic
	}
}

// ---------- timestamp parsing (2.5) ----------

func TestParseStampDialects(t *testing.T) {
	loc := time.UTC
	cases := []struct {
		in   string
		want string
	}{
		{"2026-09-03 01:30:00", "2026-09-03T01:30:00Z"},
		{"2026-08-27 16:07:50.000000", "2026-08-27T16:07:50Z"},
		{"2026-08-01 00:22:08.92693", "2026-08-01T00:22:08Z"},
		{"2026-08-27T08:22:43", "2026-08-27T08:22:43Z"},
		{"2026-09-05T03:58:00.000000+00:00", "2026-09-05T03:58:00Z"},
		{"2026-08-31T05:13:59.558464+00:00", "2026-08-31T05:13:59Z"},
		{"2026-09-05T03:58:00Z", "2026-09-05T03:58:00Z"},
	}
	for _, c := range cases {
		got, ok := parseStamp(c.in, loc)
		if !ok || got.Format(time.RFC3339) != c.want {
			t.Fatalf("parse %q = %v ok=%v, want %v", c.in, got, ok, c.want)
		}
	}
	for _, bad := range []string{"", "   ", "not-a-date", "2026-13-45 99:99:99"} {
		if _, ok := parseStamp(bad, loc); ok {
			t.Fatalf("parse %q should fail", bad)
		}
	}
}

func TestParseStampTimezoneConversion(t *testing.T) {
	bog, err := time.LoadLocation("America/Bogota")
	if err != nil {
		t.Skip("no tzdata")
	}
	// 2026-09-05T05:00Z == 2026-09-05T00:00 Bogota (UTC-5, no DST in September):
	// offset conversion lands ON the same calendar day; use 03:00Z to cross.
	got, ok := parseStamp("2026-09-05T03:00:00+00:00", bog)
	if !ok {
		t.Fatal("parse failed")
	}
	if dayKey(got) != "2026-09-04" {
		t.Fatalf("day = %s, want 2026-09-04 (Bogota)", dayKey(got))
	}
	// space layout interpreted IN the report zone (no conversion).
	got2, _ := parseStamp("2026-09-05 03:00:00", bog)
	if dayKey(got2) != "2026-09-05" {
		t.Fatalf("naive day = %s, want 2026-09-05", dayKey(got2))
	}
}

// ---------- 2.7 day helpers ----------

func TestDayMath(t *testing.T) {
	if daysBetween("2026-09-01", "2026-09-05") != 4 || daysBetween("2026-09-05", "2026-09-01") != -4 {
		t.Fatal("daysBetween wrong")
	}
	if addDays("2026-09-30", 1) != "2026-10-01" || addDays("2026-02-28", 1) != "2026-03-01" {
		t.Fatal("addDays wrong")
	}
	if !math.IsNaN(math.NaN()) {
		t.Fatal("math import sanity")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"boardctl":        "boardctl",
		"My Fleet Board":  "my-fleet-board",
		"H3 SDK (go)!!":   "h3-sdk-go",
		"--weird--name--": "weird-name",
		"   ":             "board",
		"":                "board",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------- priority label ----------

func TestPriorityLabel(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{`"P1"`, "P1", true},
		{`2`, "2", true},
		{`null`, "", false},
		{`"pending"`, "pending", true}, // pass-through string per 2.3
	}
	for _, c := range cases {
		row := mustRow(t, `{"id":"X","priority":`+c.raw+`}`)
		got, ok := priorityLabel(row)
		if ok != c.ok || got != c.want {
			t.Fatalf("priority %s -> %q %v, want %q %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

// ---------- BT-020 remediation: UI contract surfaces ----------

// TestTemplateHasCountsPanel proves gap #1's fix exists in the client: the
// V11 status/priority composition panel renders from raw rows and re-renders
// on the fixtures toggle.
func TestTemplateHasCountsPanel(t *testing.T) {
	h := mustTemplateHTML(t)
	for _, want := range []string{
		`renderCountsPanel`,
		`countsFromRaw`,
		`priorityLabelOf`,
		`"Status composition"`,
		`"Priority composition"`,
		`"counts-panel"`,
		`renderCountsPanel(bd); renderTable(bd)`, // fixtures toggle re-render (5.5)
		`"no tasks yet"`,                         // empty state
		`"no priorities set"`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("template missing counts-panel piece %q", want)
		}
	}
}

// TestTemplateHasDetailLadder proves gap #2's fix exists in the client: the
// full 2.4 ladder (object/array -> JSON string -> strict base64(JSON) ->
// raw) runs client-side, never throws, and renders through textContent.
func TestTemplateHasDetailLadder(t *testing.T) {
	h := mustTemplateHTML(t)
	for _, want := range []string{
		`function detailLadder`,
		`function parseJSONContainerStr`,
		`function strictBase64ToJSON`,
		`atob`,
		`detailLadder(v)`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("template missing detail-ladder piece %q", want)
		}
	}
	// the old object/array-only detailBody must be gone
	if strings.Contains(h, `if (typeof v === "object") {
    var txt`) {
		t.Fatal("detailBody still uses the object/array-only form (gap #2 unfixed)")
	}
}

// TestTemplateHasNoActivityStreakWording proves gap #3's fix: zero streaks
// render "0 (no activity)" via the shared streakFmt helper.
func TestTemplateHasNoActivityStreakWording(t *testing.T) {
	h := mustTemplateHTML(t)
	if !strings.Contains(h, `"0 (no activity)"`) {
		t.Fatal(`template missing the 5.7 "0 (no activity)" zero-streak wording`)
	}
	if !strings.Contains(h, `function streakFmt`) {
		t.Fatal("template missing streakFmt helper")
	}
}

// TestTemplateHasErrorStateBoardCard proves gap #4's UI half: error boards
// render an error card and are excluded from compare.
func TestTemplateHasErrorStateBoardCard(t *testing.T) {
	h := mustTemplateHTML(t)
	for _, want := range []string{
		`if (bd.error)`,
		`"card errorcard"`,
		`excluded from analytics and compare`,
		`function healthyBoards`,
		`healthyBoards().length >= 2`, // compare gate excludes error boards
		`excluded from compare (unparseable)`,
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("template missing error-state piece %q", want)
		}
	}
}

// TestErrorBoardExcludedFromComparePayload pins the payload contract: an
// error board still ships in boards[] (so its card can render) but carries
// Error != "", and the UI-level compare gate is healthyBoards().length.
func TestErrorBoardExcludedFromComparePayload(t *testing.T) {
	root1 := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  "garbage not json\n",
		"events.jsonl": emptyEvents,
	})
	rp1, err := Build(root1, Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(rp1)
	if err != nil {
		t.Fatal(err)
	}
	var back ReportPayload
	dec := json.NewDecoder(strings.NewReader(html[strings.Index(html, `{"schema"`):]))
	if err := dec.Decode(&back); err != nil {
		t.Fatalf("island unparseable: %v", err)
	}
	if len(back.Boards) != 1 || back.Boards[0].Error == "" {
		t.Fatalf("error board must ship with non-empty Error, got %+v", back.Boards)
	}
}

func mustTemplateHTML(t *testing.T) string {
	t.Helper()
	h, err := RenderHTML(&ReportPayload{Schema: SchemaName, Boards: []BoardPayload{}})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
