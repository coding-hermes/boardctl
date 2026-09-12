// Package render — payload types (Section 6.2 board-report/v1) and the
// Section 3 derivation formulas, all computed in GO. The payload is the
// single source of truth for both the HTML view and the BT-022 round-trip.
package render

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// SchemaName is the payload schema identifier (6.2).
const SchemaName = "board-report/v1"

// ParseWarnings carries per-file parse-warning counts plus the first 20
// notes truncated to 200 chars each (2.7).
type ParseWarnings struct {
	Tasks             int      `json:"tasks"`
	Events            int      `json:"events"`
	Fixtures          int      `json:"fixtures"`
	TimestampExcluded int      `json:"timestamp_excluded"`
	DuplicateIDs      int      `json:"duplicate_ids"`
	Notes             []string `json:"notes"`
}

// Series is a day-keyed integer series with an explicit window (kept for
// potential future callers; burndown/burnup use their own typed series).
type Series struct {
	Window []string `json:"window"`
	Days   []string `json:"days,omitempty"`
	Values []int    `json:"values,omitempty"`
}

// BurndownSeries is open-tasks-per-day (3.1).
type BurndownSeries struct {
	Window []string `json:"window"`
	Open   []int    `json:"open"`
}

// BurnupSeries is cumulative completions per day (3.2), same window as
// burndown so the two charts share an x-axis.
type BurnupSeries struct {
	Window []string `json:"window"`
	Days   []string `json:"days"`
	Cum    []int    `json:"cum"`
}

// VelocitySeries is weekly completion counts (3.3), ISO Monday-start weeks.
type VelocitySeries struct {
	Weeks  []string `json:"weeks"`
	Counts []int    `json:"counts"`
}

// CycleWorst is one row of the worst-5 cycle-time table.
type CycleWorst struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Days   int    `json:"days"`
	Status string `json:"status"`
}

// CycleStats is cycle time over completed tasks (3.4).
type CycleStats struct {
	N        int          `json:"n"`
	Excluded int          `json:"excluded"`
	Median   *float64     `json:"median_days"`
	P90      *float64     `json:"p90_days"`
	Worst5   []CycleWorst `json:"worst5"`
}

// StreakState is current/longest with the today-boundary state (4.3).
type StreakState struct {
	Current      int    `json:"current"`
	CurrentState string `json:"current_state"` // "live" | "ended"
	CurrentSince string `json:"current_since"` // day key of the run start, "" when current==0
	Longest      int    `json:"longest"`
}

// Streaks bundles the three streak families (3.6).
type Streaks struct {
	Delivery StreakState `json:"delivery"`
	Activity StreakState `json:"activity"`
	AnyTick  struct {
		Current int `json:"current"`
		Longest int `json:"longest"`
	} `json:"any_tick"`
}

// TickHealth is the per-day completed/failed/audit counts (3.5).
type TickHealth struct {
	Days      []string `json:"days"`
	Completed []int    `json:"completed"`
	Failed    []int    `json:"failed"`
	Audit     []int    `json:"audit"`
}

// WorkClock is the 7x24 weekday x hour event heatmap (3.7). Max is included
// so the UI legend always matches the data.
type WorkClock struct {
	Grid [7][24]int `json:"grid"`
	Max  int        `json:"max"`
}

// ModelShare is counts by primary_model over completed non-fixture tasks
// (3.8).
type ModelShare struct {
	Models     []string `json:"models"`
	Counts     []int    `json:"counts"`
	CompletedN int      `json:"completed_n"`
}

// BoardHeader is the raw header row re-serialized (nil when absent).
type BoardHeader json.RawMessage

// MarshalJSON renders the header verbatim, or null.
func (h BoardHeader) MarshalJSON() ([]byte, error) {
	if len(h) == 0 {
		return []byte("null"), nil
	}
	return h, nil
}

// UnmarshalJSON stores raw bytes.
func (h *BoardHeader) UnmarshalJSON(b []byte) error { *h = b; return nil }

// Derived is the derived block of one board (Section 3).
type Derived struct {
	Burndown      BurndownSeries `json:"burndown"`
	Burnup        BurnupSeries   `json:"burnup"`
	Velocity      VelocitySeries `json:"velocity"`
	Cycle         CycleStats     `json:"cycle"`
	Streaks       Streaks        `json:"streaks"`
	TickHealth    TickHealth     `json:"tick_health"`
	WorkClock     WorkClock      `json:"work_clock"`
	ModelShare    ModelShare     `json:"model_share"`
	StatusCounts  map[string]int `json:"status_counts"`
	PriorityCount map[string]int `json:"priority_counts"`
	OpenCount     int            `json:"open_count"`
	CompleteCount int            `json:"complete_count"`
	TotalNonFix   int            `json:"total_nonfixture"`
	LastActivity  string         `json:"last_activity_day"`
}

// BoardPayload is one board's entry in the report payload (6.2). Tasks and
// events are the RAW rows re-serialized so the expander works without
// recomputation and import can round-trip verbatim bytes.
type BoardPayload struct {
	Name          string            `json:"name"`
	Slug          string            `json:"slug"`
	Topology      string            `json:"topology"`
	Header        BoardHeader       `json:"header"`
	Tasks         []json.RawMessage `json:"tasks"`
	Events        []json.RawMessage `json:"events"`
	FixtureIDs    []string          `json:"fixture_ids"`
	ParseWarnings ParseWarnings     `json:"parse_warnings"`
	Derived       Derived           `json:"derived"`
}

// ReportPayload is the top-level board-report/v1 object. rendered_at is set
// by the caller; NoDoneCompleted carries the data-quality footnote ids
// (completed with no computable done day, 3.0).
type ReportPayload struct {
	Schema          string         `json:"schema"`
	RenderedAt      string         `json:"rendered_at"`
	ReportTimezone  string         `json:"report_timezone"`
	Boards          []BoardPayload `json:"boards"`
	GeneratedBy     string         `json:"generated_by"`
	NoDoneCompleted []string       `json:"completed_no_done_ids,omitempty"`
}

// fixtureRows / taskRows split the deduped task set.
func fixtureRows(d *boardData) []*taskRec {
	var out []*taskRec
	for _, t := range d.tasks {
		if t.fixture {
			out = append(out, t)
		}
	}
	return out
}

func nonFixtureRows(d *boardData) []*taskRec {
	var out []*taskRec
	for _, t := range d.tasks {
		if !t.fixture {
			out = append(out, t)
		}
	}
	return out
}

// derive computes the Section 3 metrics for one board.
func derive(d *boardData) Derived {
	var der Derived
	tasks := nonFixtureRows(d)
	der.TotalNonFix = len(tasks)
	der.StatusCounts = map[string]int{}
	der.PriorityCount = map[string]int{}

	// 3.9 status/priority counts: fixture rows EXCLUDED (stats defaults).
	for _, t := range tasks {
		der.StatusCounts[t.status]++
		if t.status == "complete" {
			der.CompleteCount++
		} else {
			der.OpenCount++
		}
		if t.priority != nil {
			der.PriorityCount[*t.priority]++
		}
	}
	// Note: status counts for the payload are map[string]int — the
	// default complete/open split above drives key numbers; the full
	// per-status buckets ride StatusCounts.

	// Time-series window (3.0): [min(birth) .. max(today, last event day,
	// last done day)].
	var minBirth, maxDone, lastEvent string
	haveMin := false
	for _, t := range tasks {
		if t.birth != nil {
			k := dayKey(*t.birth)
			if !haveMin || k < minBirth {
				minBirth, haveMin = k, true
			}
		}
		if t.done != nil {
			k := dayKey(*t.done)
			if k > maxDone {
				maxDone = k
			}
		}
	}
	for _, e := range d.events {
		if e.ts != nil {
			k := dayKey(*e.ts)
			if k > lastEvent {
				lastEvent = k
			}
		}
	}
	today := dayKey(d.now)
	end := today
	if lastEvent > end {
		end = lastEvent
	}
	if maxDone > end {
		end = maxDone
	}
	der.LastActivity = lastEvent
	if lastEvent == "" {
		for _, t := range tasks {
			if t.done != nil {
				k := dayKey(*t.done)
				if k > der.LastActivity {
					der.LastActivity = k
				}
			}
		}
	}

	if haveMin && end >= minBirth {
		window := dayRange(minBirth, end)
		der.Burndown = burndown(tasks, window)
		der.Burnup = burnup(tasks, window)
	}
	der.Velocity = velocity(tasks, today)
	der.Cycle = cycleStats(tasks)
	der.Streaks = streaks(d, tasks)
	der.TickHealth = tickHealth(d.events)
	der.WorkClock = workClock(d.events, d.loc)
	der.ModelShare = modelShare(tasks)
	return der
}

// dayRange returns the inclusive day keys from d1 to d2.
func dayRange(d1, d2 string) []string {
	if d2 < d1 {
		return nil
	}
	n := daysBetween(d1, d2) + 1
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, addDays(d1, i))
	}
	return out
}

// burndown: open(d) = |{ t : birth(t) <= d AND (done(t) absent OR done(t) > d) }|
func burndown(tasks []*taskRec, window []string) BurndownSeries {
	out := BurndownSeries{Window: []string{window[0], window[len(window)-1]}}
	for _, d := range window {
		open := 0
		for _, t := range tasks {
			if t.birth == nil || dayKey(*t.birth) > d {
				continue
			}
			if t.done != nil && dayKey(*t.done) <= d {
				continue
			}
			open++
		}
		out.Open = append(out.Open, open)
	}
	return out
}

// burnup: cum(d) = |{ t : done(t) exists AND done(t) <= d }|
func burnup(tasks []*taskRec, window []string) BurnupSeries {
	out := BurnupSeries{Window: []string{window[0], window[len(window)-1]}, Days: window}
	for _, d := range window {
		cum := 0
		for _, t := range tasks {
			if t.done != nil && dayKey(*t.done) <= d {
				cum++
			}
		}
		out.Cum = append(out.Cum, cum)
	}
	return out
}

// isoWeekKey returns the ISO week key YYYY-Www (Monday-start) for t.
func isoWeekKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

// isoWeekMonday returns the Monday-starting ISO week key sequence from the
// week containing d1 to the week containing d2.
func isoWeekSpan(d1, d2 string) []string {
	t1, _ := time.ParseInLocation("2006-01-02", d1, time.UTC)
	t2, _ := time.ParseInLocation("2006-01-02", d2, time.UTC)
	if t2.Before(t1) {
		return nil
	}
	// rewind t1 to its Monday
	for t1.Weekday() != time.Monday {
		t1 = t1.AddDate(0, 0, -1)
	}
	var out []string
	for !t1.After(t2) {
		out = append(out, isoWeekKey(t1))
		t1 = t1.AddDate(0, 0, 7)
	}
	return out
}

// velocity: completions per ISO week (3.3), payload = all computed weeks;
// the display window (most recent 12 active weeks) is a UI concern.
func velocity(tasks []*taskRec, today string) VelocitySeries {
	// window: weeks spanning min(birth) .. max(today, last done)
	var minBirth, maxDone string
	haveMin := false
	for _, t := range tasks {
		if t.birth != nil {
			k := dayKey(*t.birth)
			if !haveMin || k < minBirth {
				minBirth, haveMin = k, true
			}
		}
		if t.done != nil {
			k := dayKey(*t.done)
			if k > maxDone {
				maxDone = k
			}
		}
	}
	if !haveMin {
		return VelocitySeries{Weeks: []string{}, Counts: []int{}}
	}
	end := today
	if maxDone > end {
		end = maxDone
	}
	if end < minBirth {
		end = minBirth
	}
	weeks := isoWeekSpan(minBirth, end)
	if weeks == nil {
		weeks = []string{}
	}
	counts := make([]int, len(weeks))
	idx := map[string]int{}
	for i, w := range weeks {
		idx[w] = i
	}
	for _, t := range tasks {
		if t.done == nil {
			continue
		}
		wk := isoWeekKey(*t.done)
		if i, ok := idx[wk]; ok {
			counts[i]++
		}
	}
	return VelocitySeries{Weeks: weeks, Counts: counts}
}

// cycleStats: cycle(t) = done - birth in whole days (3.4).
func cycleStats(tasks []*taskRec) CycleStats {
	var cs CycleStats
	cs.Worst5 = []CycleWorst{}
	var days []int
	var excludedNoDone, excludedNoBirth, completed int
	for _, t := range tasks {
		if t.status != "complete" {
			continue
		}
		completed++
		if t.done == nil || t.birth == nil {
			// a completion with no computable done/birth day
			if t.birth == nil {
				excludedNoBirth++
			} else {
				excludedNoDone++
			}
			continue
		}
		d := daysBetween(dayKey(*t.birth), dayKey(*t.done))
		if d < 0 {
			d = 0 // clock skew: clamp (dip is honest in burndown, cycle is a duration)
		}
		days = append(days, d)
	}
	cs.N = len(days)
	cs.Excluded = excludedNoDone + excludedNoBirth
	if n := len(days); n > 0 {
		sort.Ints(days)
		med := medianSorted(days)
		p90 := float64(days[nearestRank(n, 0.90)-1])
		cs.Median = &med
		cs.P90 = &p90
		// worst-5 by cycle days, ties broken by id for determinism
		type wc struct {
			id    string
			title string
			days  int
			st    string
		}
		var all []wc
		for _, t := range tasks {
			if t.status != "complete" || t.done == nil || t.birth == nil {
				continue
			}
			d := daysBetween(dayKey(*t.birth), dayKey(*t.done))
			if d < 0 {
				d = 0
			}
			all = append(all, wc{id: t.id, title: t.row.String("title"), days: d, st: t.status})
		}
		sort.Slice(all, func(i, j int) bool {
			if all[i].days != all[j].days {
				return all[i].days > all[j].days
			}
			return all[i].id < all[j].id
		})
		for i, w := range all {
			if i >= 5 {
				break
			}
			title := w.title
			if r := []rune(title); len(r) > 60 {
				title = string(r[:59]) + "…"
			}
			cs.Worst5 = append(cs.Worst5, CycleWorst{ID: w.id, Title: title, Days: w.days, Status: w.st})
		}
	}
	_ = completed
	return cs
}

// medianSorted: even count -> mean of the two middle values.
func medianSorted(s []int) float64 {
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return float64(s[n/2-1]+s[n/2]) / 2
}

// nearestRank returns the 1-based nearest-rank index for percentile p.
func nearestRank(n int, p float64) int {
	r := int(math.Ceil(p * float64(n)))
	if r < 1 {
		r = 1
	}
	if r > n {
		r = n
	}
	return r
}

// runInfo is a consecutive-day run result.
type runInfo struct {
	longest      int
	current      int
	currentLast  string // last day key of the current run
	currentStart string
}

// computeRuns walks the sorted day set: a run breaks when the day gap > 1.
func computeRuns(keys []string) runInfo {
	var ri runInfo
	if len(keys) == 0 {
		return ri
	}
	run := 1
	longest := 1
	for i := 1; i < len(keys); i++ {
		if daysBetween(keys[i-1], keys[i]) == 1 {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	ri.longest = longest
	// current run = the run ending at the last element
	cur := 1
	start := len(keys) - 1
	for i := len(keys) - 1; i > 0; i-- {
		if daysBetween(keys[i-1], keys[i]) == 1 {
			cur++
			start = i - 1
		} else {
			break
		}
	}
	ri.current = cur
	ri.currentLast = keys[len(keys)-1]
	ri.currentStart = keys[start]
	return ri
}

// applyTodayBoundary renders current/current_state/current_since per 4.3.
// current_since is the run START day while the streak is live, and the run
// END day when it has ended (AC 7.1.9: "ended" carries the day the streak
// stopped, so the UI can print 'ended 2026-09-10 (N days ago)').
func applyTodayBoundary(ri runInfo, today string) StreakState {
	st := StreakState{Longest: ri.longest}
	if ri.current == 0 {
		st.CurrentState = "live"
		return st
	}
	st.Current = ri.current
	st.CurrentSince = ri.currentStart
	gap := daysBetween(ri.currentLast, today)
	if gap <= 1 {
		st.CurrentState = "live"
	} else {
		st.CurrentState = "ended"
		st.CurrentSince = ri.currentLast
	}
	return st
}

// streaks: delivery / activity / any-tick (3.6 + 4.x).
func streaks(d *boardData, tasks []*taskRec) Streaks {
	var s Streaks
	today := dayKey(d.now)

	// (a) delivery: done days of non-fixture tasks
	deliv := map[string]bool{}
	for _, t := range tasks {
		if t.done != nil {
			deliv[dayKey(*t.done)] = true
		}
	}
	s.Delivery = applyTodayBoundary(computeRuns(sortedDayKeys(deliv)), today)

	// (b) activity: non-idle event days
	act := map[string]bool{}
	all := map[string]bool{}
	for _, e := range d.events {
		if e.ts == nil {
			continue
		}
		k := dayKey(*e.ts)
		all[k] = true
		if e.etype != "idle" {
			act[k] = true
		}
	}
	s.Activity = applyTodayBoundary(computeRuns(sortedDayKeys(act)), today)

	// (c) any-tick: all event days (context only)
	at := computeRuns(sortedDayKeys(all))
	s.AnyTick.Current = at.current
	s.AnyTick.Longest = at.longest
	return s
}

// tickHealth: per-day completed/failed/audit over ALL events (3.5). Days
// with zero of all three are omitted (gaps render as gaps).
func tickHealth(events []*eventRec) TickHealth {
	type cell struct{ c, f, a int }
	byDay := map[string]*cell{}
	var order []string
	for _, e := range events {
		if e.ts == nil {
			continue
		}
		k := dayKey(*e.ts)
		c := byDay[k]
		if c == nil {
			c = &cell{}
			byDay[k] = c
			order = append(order, k)
		}
		et := e.etype
		low := strings.ToLower(et)
		failed := strings.Contains(low, "fail")
		if !failed && et == "task_completed" && e.hasDec {
			if m, ok := e.decoded.(map[string]any); ok {
				if outc, ok2 := m["outcome"]; ok2 {
					if so, ok3 := outc.(string); ok3 && strings.EqualFold(so, "failed") {
						failed = true
					}
				}
			}
		}
		switch {
		case et == "task_completed":
			c.c++
		}
		if failed {
			c.f++
		}
		if et == "audit" {
			c.a++
		}
	}
	sort.Strings(order)
	th := TickHealth{Days: []string{}, Completed: []int{}, Failed: []int{}, Audit: []int{}}
	for _, k := range order {
		c := byDay[k]
		if c.c == 0 && c.f == 0 && c.a == 0 {
			continue
		}
		th.Days = append(th.Days, k)
		th.Completed = append(th.Completed, c.c)
		th.Failed = append(th.Failed, c.f)
		th.Audit = append(th.Audit, c.a)
	}
	return th
}

// workClock: 7x24 weekday x hour counts over ALL events (3.7). Row 0 is
// Monday per ISO convention.
func workClock(events []*eventRec, loc *time.Location) WorkClock {
	var wc WorkClock
	for _, e := range events {
		if e.ts == nil {
			continue
		}
		t := e.ts.In(loc)
		wd := int(t.Weekday()) // Sun=0..Sat=6
		if wd == 0 {
			wd = 6
		} else {
			wd = wd - 1
		}
		wc.Grid[wd][t.Hour()]++
		if wc.Grid[wd][t.Hour()] > wc.Max {
			wc.Max = wc.Grid[wd][t.Hour()]
		}
	}
	return wc
}

// modelShare: counts by primary_model over completed non-fixture tasks (3.8).
func modelShare(tasks []*taskRec) ModelShare {
	ms := ModelShare{}
	counts := map[string]int{}
	for _, t := range tasks {
		if t.status != "complete" {
			continue
		}
		ms.CompletedN++
		m := t.model
		if m == "" {
			m = "unknown"
		}
		counts[m]++
	}
	ms.Models = make([]string, 0, len(counts))
	for k := range counts {
		ms.Models = append(ms.Models, k)
	}
	sort.Strings(ms.Models)
	// order by count desc, then name asc for rendering
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for _, m := range ms.Models {
		pairs = append(pairs, kv{m, counts[m]})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	ms.Models = []string{}
	ms.Counts = []int{}
	for _, p := range pairs {
		ms.Models = append(ms.Models, p.k)
		ms.Counts = append(ms.Counts, p.v)
	}
	return ms
}
