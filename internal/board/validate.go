package board

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Finding is one itemized validate result.
type Finding struct {
	Level string // "error" or "warn"
	Msg   string
}

// IsError reports whether a finding blocks (exit non-zero).
func (f Finding) IsError() bool { return f.Level == "error" }

// Report is the full validate output.
type Report struct {
	Dir      string
	Topology string
	Findings []Finding
	Tasks    int
	Events   int
	Fixtures int
	Header   bool
}

// Add appends a finding.
func (r *Report) Add(level, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{Level: level, Msg: fmt.Sprintf(format, args...)})
}

// Errors returns the count of error-level findings.
func (r *Report) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.IsError() {
			n++
		}
	}
	return n
}

// HasErrors reports whether the board failed validation.
func (r *Report) HasErrors() bool { return r.Errors() > 0 }

// Validate checks a board:
//   - every line of tasks.jsonl / events.jsonl / board.jsonl / fixtures.jsonl parses
//   - task ids are unique (error on duplicate)
//   - event ids are ascending with tolerated gaps; duplicate and non-numeric
//     ids are itemized warnings (benign legacy rows exist on live boards)
//   - board.jsonl header (topology A) parses with integer counters
//   - task status is in the vocabulary {pending,in_progress,review,blocked,
//     complete,failed}; read-side alias "completed" is accepted
//
// Exit non-zero (HasErrors) with the itemized report on any error.
func (b *Board) Validate() (*Report, error) {
	rep := &Report{Dir: b.Dir, Topology: b.Topology}
	b.validateTasks(rep)
	if err := b.validateEvents(rep); err != nil {
		return nil, err
	}
	if b.Topology == "A" {
		if err := b.validateHeader(rep); err != nil {
			return nil, err
		}
	} else {
		rep.Add("warn", "topology B: no board.jsonl — header is line 1 of tasks.jsonl (read-only legacy layout); header counters not checked")
	}
	if fp := b.FixturesPath(); fp != "" {
		b.validateFixtures(rep, fp)
	}
	return rep, nil
}

func (b *Board) validateTasks(rep *Report) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		rep.Add("error", "tasks.jsonl: %v", err)
		return
	}
	seen := map[string]int{}
	rows := 0
	ierr := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		rows++
		id := row.String("id")
		if id == "" {
			rep.Add("warn", "tasks.jsonl line %d: row has no id", idx+1)
		} else if prev, dup := seen[id]; dup {
			rep.Add("error", "tasks.jsonl line %d: duplicate task id %q (also line %d)", idx+1, id, prev)
		} else {
			seen[id] = idx + 1
		}
		st := row.String("status")
		if st == "" {
			rep.Add("warn", "tasks.jsonl line %d (task %s): missing status", idx+1, id)
		} else if !StatusVocabulary[NormalizeStatus(st)] {
			rep.Add("error", "tasks.jsonl line %d (task %s): status %q not in vocabulary {%s} ('completed' accepted as read alias)",
				idx+1, id, st, strings.Join(sortedVocab(), ","))
		}
		return nil
	})
	if ierr != nil {
		rep.Add("error", "tasks.jsonl: %v", ierr)
	}
	rep.Tasks = rows
}

// validateEvents parses events.jsonl: parse errors are errors; numeric ids
// must ascend with tolerated gaps (duplicates and non-numeric/missing ids are
// itemized warnings — live boards carry benign legacy rows of both kinds).
func (b *Board) validateEvents(rep *Report) error {
	lines, err := ReadJSONLLines(b.eventsPath)
	if err != nil {
		rep.Add("error", "events.jsonl: %v", err)
		return nil
	}
	var lastID int64
	haveLast := false
	seen := map[int64]int{}
	count := 0
	ierr := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		count++
		id, isInt := row.Int("id")
		switch {
		case !row.Has("id"):
			rep.Add("warn", "events.jsonl line %d: event has no id field (legacy shape tolerated)", idx+1)
		case !isInt:
			rep.Add("warn", "events.jsonl line %d: event id %q is not a JSON number (legacy shape tolerated)", idx+1, row.String("id"))
		default:
			if prev, dup := seen[id]; dup {
				rep.Add("warn", "events.jsonl line %d: duplicate event id %d (also line %d) — benign legacy duplicates exist on live boards", idx+1, id, prev)
				return nil // duplicates are warned, not ordering errors
			}
			seen[id] = idx + 1
			if haveLast && id < lastID {
				rep.Add("error", "events.jsonl line %d: event id %d descends below earlier id %d (ids must ascend; gaps tolerated)", idx+1, id, lastID)
			}
			if id > lastID {
				lastID = id
				haveLast = true
			}
		}
		return nil
	})
	if ierr != nil {
		rep.Add("error", "events.jsonl: %v", ierr)
	}
	rep.Events = count
	return nil
}

func (b *Board) validateHeader(rep *Report) error {
	lines, err := ReadJSONLLines(b.headerPath)
	if err != nil {
		rep.Add("error", "board.jsonl: %v", err)
		return nil
	}
	rep.Header = true
	if len(lines) == 0 || len(bytes.TrimSpace(lines[0])) == 0 {
		rep.Add("error", "board.jsonl: empty — topology A requires a header row on line 1")
		return nil
	}
	row, err := ParseRow(bytes.TrimSpace(lines[0]))
	if err != nil {
		rep.Add("error", "board.jsonl line 1: %v", err)
		return nil
	}
	for _, k := range []string{"version", "ticks_total", "ticks_idle"} {
		if _, ok := row.Int(k); !ok {
			rep.Add("error", "board.jsonl header: %s is not an integer counter", k)
		}
	}
	if _, ok := row.Int("cooldown_s"); !ok {
		if raw := row.Get("cooldown_s"); raw != nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			rep.Add("error", "board.jsonl header: cooldown_s is not an integer (or null)")
		}
	}
	for _, k := range []string{"project", "namespace"} {
		if row.String(k) == "" {
			rep.Add("warn", "board.jsonl header: missing %s", k)
		}
	}
	if len(lines) > 1 {
		n := 0
		for _, l := range lines[1:] {
			if len(bytes.TrimSpace(l)) > 0 {
				n++
			}
		}
		if n > 0 {
			rep.Add("warn", "board.jsonl: %d extra non-empty line(s) after line 1 (header must be line 1)", n)
		}
	}
	return nil
}

func (b *Board) validateFixtures(rep *Report, path string) {
	lines, err := ReadJSONLLines(path)
	if err != nil {
		rep.Add("error", "fixtures.jsonl: %v", err)
		return
	}
	seen := map[string]int{}
	count := 0
	IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		count++
		id := row.String("id")
		if id == "" {
			rep.Add("warn", "fixtures.jsonl line %d: fixture row has no id", idx+1)
			return nil
		}
		if prev, dup := seen[id]; dup {
			rep.Add("error", "fixtures.jsonl line %d: duplicate fixture id %q (also line %d)", idx+1, id, prev)
		} else {
			seen[id] = idx + 1
		}
		return nil
	})
	if err != nil {
		rep.Add("error", "fixtures.jsonl: %v", err)
	}
	rep.Fixtures = count
}

func sortedVocab() []string {
	out := make([]string, 0, len(StatusVocabulary))
	for k := range StatusVocabulary {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// RenderText renders the itemized report.
func (r *Report) RenderText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "board: %s (topology %s)\n", r.Dir, r.Topology)
	fmt.Fprintf(&sb, "rows: %d tasks, %d events, %d fixtures%s\n", r.Tasks, r.Events, r.Fixtures, headerNote(r.Header))
	for _, f := range r.Findings {
		fmt.Fprintf(&sb, "[%s] %s\n", f.Level, f.Msg)
	}
	if r.HasErrors() {
		fmt.Fprintf(&sb, "RESULT: FAIL (%d error(s), %d warning(s))\n", r.Errors(), len(r.Findings)-r.Errors())
	} else {
		fmt.Fprintf(&sb, "RESULT: OK (%d warning(s))\n", len(r.Findings))
	}
	return sb.String()
}

func headerNote(has bool) string {
	if has {
		return ", header parsed"
	}
	return ""
}
