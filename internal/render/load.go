// Package render builds the board-analytics payload and the self-contained
// HTML report (BT-020) per docs/specs/board-analytics-report.md — the design
// authority for render/serve/import. Render is strictly read-only toward
// board files: it never writes tasks.jsonl, events.jsonl, board.jsonl, or
// fixtures.jsonl.
//
// The loader in this file is deliberately NOT board.IterParsed (which aborts
// on the first malformed line): the spec mandates a tolerant reader that
// skips torn rows with parse warnings, collapses duplicate task ids
// last-row-wins, keeps duplicate events, and classifies fixtures by the
// three-signal union (fixtures.jsonl membership, NEVER-DONE prefix,
// perpetual:true).
package render

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// headerShapeKeys are the keys that make a row a topology-B header (rowIsHeaderShape
// semantics: no id + at least one of these).
var headerShapeKeys = []string{"project", "namespace", "version", "ticks_total"}

// isHeaderShape reports whether a parsed row looks like a board header
// (metadata without a task id). Mirrors board.rowIsHeaderShape.
func isHeaderShape(row *board.Row) bool {
	if row.String("id") != "" {
		return false
	}
	for _, k := range headerShapeKeys {
		if row.Has(k) {
			return true
		}
	}
	return false
}

// warningSink accumulates parse warnings per 2.7: per-file bad-line counts,
// timestamp exclusions, duplicate-id collapses, and the first 20 notes
// truncated to 200 chars each.
type warningSink struct {
	tasks    int
	events   int
	fixtures int
	tsExcl   int
	dupIDs   int
	notes    []string
}

func (w *warningSink) note(format string, args ...any) {
	if len(w.notes) >= 20 {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if len(msg) > 200 {
		msg = msg[:200]
	}
	w.notes = append(w.notes, msg)
}

func (w *warningSink) total() int {
	return w.tasks + w.events + w.fixtures + w.tsExcl + w.dupIDs
}

func (w *warningSink) payload() ParseWarnings {
	notes := w.notes
	if notes == nil {
		notes = []string{}
	}
	return ParseWarnings{
		Tasks:             w.tasks,
		Events:            w.events,
		Fixtures:          w.fixtures,
		TimestampExcluded: w.tsExcl,
		DuplicateIDs:      w.dupIDs,
		Notes:             notes,
	}
}

// taskRec is the derived view of one (deduped) task row.
type taskRec struct {
	row        *board.Row
	id         string
	status     string // normalized; "(none)" when empty/missing
	fixture    bool
	priority   *string // display label; nil when absent/wrong-typed
	birth      *time.Time
	done       *time.Time
	doneByFall bool // done came from updated_at fallback (status complete)
	model      string
}

// eventRec is the derived view of one event row (never deduped).
type eventRec struct {
	row     *board.Row
	ts      *time.Time
	etype   string
	decoded any
	hasDec  bool
}

// boardData is everything the derivations need from one board folder.
type boardData struct {
	name     string
	slug     string
	topology string
	header   *board.Row // nil when neither topology carries a header
	loc      *time.Location
	now      time.Time  // render time in loc
	tasks    []*taskRec // deduped, file order of the winning row
	events   []*eventRec
	fixtureI map[string]bool // ids from fixtures.jsonl only
	warns    warningSink
}

// slugify builds the URL-safe board slug: lowercased, runs of
// non-alphanumerics collapsed to a single "-", trimmed at the edges.
func slugify(name string) string {
	var sb strings.Builder
	prev := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prev = false
			continue
		}
		if !prev && sb.Len() > 0 {
			sb.WriteByte('-')
			prev = true
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		out = "board"
	}
	return out
}

// loadBoard tolerantly reads one resolved board. now supplies the
// report-timezone wall clock used for every day-bucketing decision.
func loadBoard(b *board.Board, now time.Time) (*boardData, error) {
	d := &boardData{
		topology: b.Topology,
		fixtureI: map[string]bool{},
		loc:      now.Location(),
		now:      now,
	}
	// Topology A header: board.jsonl single line. Topology B: line 1 of
	// tasks.jsonl when it has header shape. Neither -> no header (name
	// falls back to the directory base name).
	tasksLines, err := tolerantLines(b.TasksPath())
	if err != nil {
		return nil, err
	}
	eventsLines, err := tolerantLines(b.EventsPath())
	if err != nil {
		return nil, err
	}

	firstTaskIdx := 0
	if b.IsTopologyA() {
		if hdr := firstNonBlankRow(b.HeaderPath()); hdr != nil {
			if isHeaderShape(hdr) {
				d.header = hdr
			} else {
				d.warns.note("board.jsonl line 1 has no header shape; treated as absent")
			}
		}
	} else {
		hdr := firstNonBlankRowIdx(tasksLines)
		if hdr.row != nil && isHeaderShape(hdr.row) {
			d.header = hdr.row
			firstTaskIdx = hdr.idx + 1
		}
	}
	d.name = ""
	if d.header != nil {
		d.name = strings.TrimSpace(d.header.String("project"))
	}
	if d.name == "" {
		d.name = filepath.Base(b.Dir)
	}
	d.slug = slugify(d.name)

	// Fixture ids from fixtures.jsonl (tolerant; malformed rows warn).
	if fp := b.FixturesPath(); fp != "" {
		fixLines, err := tolerantLines(fp)
		if err != nil {
			return nil, err
		}
		for i, line := range fixLines {
			if len(bytes.TrimSpace(line)) == 0 {
				continue // blank lines skip silently
			}
			row, err := board.ParseRow(line)
			if err != nil {
				d.warns.fixtures++
				d.warns.note("fixtures.jsonl line %d: skipped (%v)", i+1, err)
				continue
			}
			if id := row.String("id"); id != "" {
				d.fixtureI[id] = true
			}
		}
	}

	// Task rows: tolerant scan from firstTaskIdx, last-row-wins dedup.
	type slot struct {
		rec  *taskRec
		line int
	}
	byID := map[string]*slot{}
	var order []string
	for i := firstTaskIdx; i < len(tasksLines); i++ {
		line := tasksLines[i]
		if len(bytes.TrimSpace(line)) == 0 {
			continue // blank lines skip silently
		}
		row, err := board.ParseRow(line)
		if err != nil {
			d.warns.tasks++
			d.warns.note("tasks.jsonl line %d: skipped malformed row (%v)", i+1, err)
			continue
		}
		id := row.String("id")
		if id == "" {
			d.warns.tasks++
			d.warns.note("tasks.jsonl line %d: row without id skipped", i+1)
			continue
		}
		rec := buildTaskRec(row, id, d, now)
		if prev, ok := byID[id]; ok {
			d.warns.dupIDs++
			d.warns.note("tasks.jsonl line %d: duplicate id %q — line %d wins (last-row-wins)", prev.line+1, id, i+1)
			prev.rec = rec
			prev.line = i
			continue
		}
		byID[id] = &slot{rec: rec, line: i}
		order = append(order, id)
	}
	for _, id := range order {
		d.tasks = append(d.tasks, byID[id].rec)
	}

	// Events: tolerant scan, duplicates kept (append-log facts).
	for i, line := range eventsLines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		row, err := board.ParseRow(line)
		if err != nil {
			d.warns.events++
			d.warns.note("events.jsonl line %d: skipped malformed row (%v)", i+1, err)
			continue
		}
		rec := buildEventRec(row, d)
		d.events = append(d.events, rec)
	}
	return d, nil
}

// buildTaskRec derives the view fields of one task row. Timestamp fields that
// fail to parse are excluded from time-based metrics and counted once per row
// in timestamp_excluded with a note naming the id and field (2.5).
func buildTaskRec(row *board.Row, id string, d *boardData, now time.Time) *taskRec {
	rec := &taskRec{row: row, id: id}
	st := board.NormalizeStatus(row.String("status"))
	if st == "" {
		st = "(none)"
	}
	rec.status = st
	tsExcluded := false
	excl := func(field string) {
		if !tsExcluded {
			tsExcluded = true
			d.warns.tsExcl++
		}
		d.warns.note("task %s: unparseable %s %q — excluded from time-based metrics", id, field, row.String(field))
	}
	// 3.0: birth = day(created_at); missing OR unparseable excludes the
	// task from time-based metrics and counts in timestamp_excluded.
	if t, ok := parseStamp(row.String("created_at"), d.loc); ok {
		rec.birth = &t
	} else {
		excl("created_at")
	}
	// done day: completed_at, fallback updated_at when status is complete.
	// A completed task with no computable done day is a data-quality
	// footnote (3.0), not a timestamp warning.
	if t, ok := parseStamp(row.String("completed_at"), d.loc); ok {
		rec.done = &t
	} else if st == "complete" {
		if t, ok := parseStamp(row.String("updated_at"), d.loc); ok {
			rec.done = &t
			rec.doneByFall = true
		}
	} else if v := row.String("completed_at"); strings.TrimSpace(v) != "" {
		// non-complete task with a present-but-unparseable completed_at:
		// counted once per row (2.5).
		excl("completed_at")
	}
	// unparseable non-empty values in other timestamp fields also count
	// once per row (2.5); absent/null is the normal sparse shape.
	for _, f := range []string{"updated_at", "dispatched_at", "blocked_since"} {
		if v := row.String(f); strings.TrimSpace(v) != "" {
			if _, ok := parseStamp(v, d.loc); !ok {
				excl(f)
			}
		}
	}
	rec.fixture = isFixture(row, id, d.fixtureI)
	if p, ok := priorityLabel(row); ok {
		rec.priority = &p
	}
	rec.model = row.String("primary_model")
	return rec
}

// isFixture classifies a row per 2.6: fixtures.jsonl membership by exact id,
// NEVER-DONE id prefix, or perpetual:true.
func isFixture(row *board.Row, id string, fixtureIDs map[string]bool) bool {
	if fixtureIDs[id] {
		return true
	}
	if strings.HasPrefix(id, "NEVER-DONE") {
		return true
	}
	raw := row.Get("perpetual")
	if raw == nil {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	return false
}

// buildEventRec derives the view fields of one event row, running the 2.4
// detail ladder (object/array -> JSON string -> base64(JSON) -> raw).
func buildEventRec(row *board.Row, d *boardData) *eventRec {
	rec := &eventRec{row: row, etype: row.String("event_type")}
	if t, ok := parseStamp(row.String("timestamp"), d.loc); ok {
		rec.ts = &t
	} else {
		d.warns.tsExcl++
		id := row.String("id")
		if id == "" {
			// event ids are JSON numbers on live boards; String() only
			// decodes strings — fall back to the raw bytes for the note.
			id = strings.TrimSpace(string(row.Get("id")))
		}
		d.warns.note("event %s: unparseable timestamp %q — excluded from time-based metrics", id, row.String("timestamp"))
	}
	rec.decoded, rec.hasDec = decodeDetail(row.Get("detail"))
	return rec
}

// decodeDetail implements the 2.4 REQUIRED ladder, never throwing:
//  1. detail is a JSON object/array -> use as-is
//  2. detail is a string parsing as a JSON object/array -> parsed value
//  3. detail is a strict base64 string whose bytes parse as a JSON
//     object/array -> parsed value
//  4. otherwise -> raw string kept as-is (ok=false)
func decodeDetail(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	switch t := v.(type) {
	case map[string]any, []any:
		return t, true
	case string:
		if v2, ok := parseJSONContainer(t); ok {
			return v2, true
		}
		if b, ok := strictBase64(t); ok {
			if v3, ok := parseJSONContainer(string(b)); ok {
				return v3, true
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

// parseJSONContainer parses s strictly as a JSON object or array (no
// trailing garbage).
func parseJSONContainer(s string) (any, bool) {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(t))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	switch v.(type) {
	case map[string]any, []any:
		return v, true
	}
	return nil, false
}

// strictBase64 strict-decodes s as standard (padded or raw) or URL-safe
// base64. Any error moves the ladder on.
func strictBase64(s string) ([]byte, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	encs := []*base64.Encoding{
		base64.StdEncoding.Strict(),
		base64.RawStdEncoding.Strict(),
		base64.URLEncoding.Strict(),
		base64.RawURLEncoding.Strict(),
	}
	for _, enc := range encs {
		if b, err := enc.DecodeString(s); err == nil {
			return b, true
		}
	}
	return nil, false
}

// priorityLabel mirrors board.priorityLabel: string form as-is (P0..P3),
// JSON numbers as decimal strings ("1".."3"); missing/null/wrong-typed are
// excluded. String and numeric forms are DISTINCT buckets (live boards mix).
func priorityLabel(row *board.Row) (string, bool) {
	raw := row.Get("priority")
	if raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	default:
		return "", false
	}
}

// stampLayouts are the timestamp dialects the report parser accepts (2.5):
// space-naive with optional fraction (report tz), RFC3339-ish with optional
// fraction and offset (converted), T-naive (report tz wall clock).
var stampLayouts = []struct {
	layout string
	offset bool
}{
	{"2006-01-02 15:04:05.999999999", false},
	{"2006-01-02 15:04:05", false},
	{"2006-01-02T15:04:05.999999999Z07:00", true},
	{"2006-01-02T15:04:05Z07:00", true},
	{"2006-01-02T15:04:05.999999999", false},
	{"2006-01-02T15:04:05", false},
}

// parseStamp parses one timestamp string into the report timezone. Sub-second
// precision is irrelevant to day bucketing (day keys truncate). Unparseable
// or empty values return ok=false and are excluded from time metrics.
func parseStamp(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, l := range stampLayouts {
		if l.offset {
			if t, err := time.Parse(l.layout, s); err == nil {
				return t.In(loc), true
			}
			continue
		}
		if t, err := time.ParseInLocation(l.layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// dayKey renders a report-timezone day key YYYY-MM-DD.
func dayKey(t time.Time) string { return t.Format("2006-01-02") }

// tolerantLines reads a JSONL file split on "\n" (no parsing). Missing files
// are an error only when the caller requires the file.
func tolerantLines(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.Split(data, []byte("\n")), nil
}

type lineRef struct {
	row *board.Row
	idx int
}

// firstNonBlankRow parses the first non-blank line of a file path (nil when
// none / unreadable).
func firstNonBlankRow(path string) *board.Row {
	lines, err := tolerantLines(path)
	if err != nil {
		return nil
	}
	return firstNonBlankRowIdx(lines).row
}

func firstNonBlankRowIdx(lines [][]byte) lineRef {
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := board.ParseRow(l)
		if err != nil {
			return lineRef{}
		}
		return lineRef{row: row, idx: i}
	}
	return lineRef{}
}

// sortedDayKeys returns the sorted unique day keys of a set.
func sortedDayKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// daysBetween returns the calendar-day difference of two YYYY-MM-DD day keys
// (d2 - d1).
func daysBetween(d1, d2 string) int {
	t1, err1 := time.ParseInLocation("2006-01-02", d1, time.UTC)
	t2, err2 := time.ParseInLocation("2006-01-02", d2, time.UTC)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(t2.Sub(t1).Hours() / 24)
}

// addDays returns the day key n calendar days after d.
func addDays(d string, n int) string {
	t, err := time.ParseInLocation("2006-01-02", d, time.UTC)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, n).Format("2006-01-02")
}
