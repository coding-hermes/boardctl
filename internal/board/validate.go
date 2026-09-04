package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
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
//   - BT-007: guard_result/ci_result values on existing rows are itemized
//     warnings when free-form (legacy prose tolerated, not failed)
//   - BT-007: depends_on ids that reference no task row are itemized warnings
//   - BT-007: negative header counters are errors
//
// Exit non-zero (HasErrors) with the itemized report on any error.
func (b *Board) Validate() (*Report, error) {
	rep := &Report{Dir: b.Dir, Topology: b.Topology}
	b.validateTasks(rep)
	if err := b.validateEvents(rep); err != nil {
		return nil, err
	}
	// BT-010: the header checks run in BOTH topologies — on topology B the
	// header is line 1 of tasks.jsonl (validateHeader reads it via
	// HeaderRow/headerPathFor), so nothing is downgraded to "not checked".
	if err := b.validateHeader(rep); err != nil {
		return nil, err
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
	depends := map[string][]depRef{} // dependency id -> rows that reference it
	rows := 0
	ierr := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: line 1 is the header, not a task row
		}
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
		// BT-007: guard_result/ci_result values already on boards must be
		// in the write vocabulary. The canonical form is upper-cased
		// (PASS/FAIL/SKIP and GREEN/RED/SKIP); case is tolerated on read
		// so a lowercase spelling does not flag. Empty/null (never run)
		// rows are skipped; free-form prose values ARE flagged (warn) —
		// legacy boards carry prose in these columns that predates the
		// vocabulary, and flagging (not failing) is the point.
		for _, c := range []struct {
			key   string
			vocab map[string]bool
		}{
			{"guard_result", GuardResultVocabulary},
			{"ci_result", CIResultVocabulary},
		} {
			v := row.String(c.key)
			if v == "" {
				continue // absent, null, or never-run
			}
			if !c.vocab[NormalizeResultValue(v)] {
				rep.Add("warn", "tasks.jsonl line %d (task %s): %s %q is free-form — not in vocabulary {PASS,FAIL,SKIP} / {GREEN,RED,SKIP} (writes now enforce this; hand-edit the row or rewrite via boardctl update)",
					idx+1, id, c.key, v)
			}
		}
		// BT-007: collect depends_on for the existence cross-check below.
		for _, dep := range rowStringSlice(row, "depends_on") {
			depends[dep] = append(depends[dep], depRef{line: idx + 1, id: id})
		}
		return nil
	})
	if ierr != nil {
		rep.Add("error", "tasks.jsonl: %v", ierr)
	}
	// BT-007: dependency ids must exist in tasks.jsonl (WARN, per the board
	// wording "validate depends_on ids exist (warn)") — legacy boards carry
	// dangling refs that a read-only check must surface but not fail on.
	for _, dep := range SortedKeys(depends) {
		if _, ok := seen[dep]; !ok {
			for _, ref := range depends[dep] {
				rep.Add("warn", "tasks.jsonl line %d (task %s): depends_on references nonexistent task id %q",
					ref.line, ref.id, dep)
			}
		}
	}
	rep.Tasks = rows
}

// depRef records which row references a depends_on id (for itemized findings).
type depRef struct {
	line int
	id   string
}

// rowStringSlice returns the string elements of an array-valued key (e.g.
// depends_on). Non-array / malformed values yield nil.
func rowStringSlice(row *Row, key string) []string {
	raw := row.Get(key)
	if raw == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range arr {
		if s, ok := e.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
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

// validateHeader checks the board header: integer, non-negative tick
// counters, an integer-or-null cooldown_s, and project/namespace identity.
// Works in BOTH topologies (BT-010): the header is read via headerPathFor —
// board.jsonl line 1 on topology A, tasks.jsonl line 1 on topology B — with
// findings worded for whichever file carries it.
func (b *Board) validateHeader(rep *Report) error {
	headerPath := b.headerPathFor()
	name := filepath.Base(headerPath)
	lines, err := ReadJSONLLines(headerPath)
	if err != nil {
		rep.Add("error", "%s: %v", name, err)
		return nil
	}
	rep.Header = true
	if len(lines) == 0 || len(bytes.TrimSpace(lines[0])) == 0 {
		rep.Add("error", "%s: empty — a header row is required on line 1", name)
		return nil
	}
	row, err := ParseRow(bytes.TrimSpace(lines[0]))
	if err != nil {
		rep.Add("error", "%s line 1: %v", name, err)
		return nil
	}
	for _, k := range []string{"version", "ticks_total", "ticks_idle"} {
		if v, ok := row.Int(k); !ok {
			rep.Add("error", "%s header: %s is not an integer counter", name, k)
		} else if v < 0 {
			// BT-007: counters are non-negative by definition — a
			// negative header counter is junk, not a tick count.
			rep.Add("error", "%s header: %s is negative (%d) — counters must be >= 0", name, k, v)
		}
	}
	if _, ok := row.Int("cooldown_s"); !ok {
		if raw := row.Get("cooldown_s"); raw != nil && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			rep.Add("error", "%s header: cooldown_s is not an integer (or null)", name)
		}
	}
	for _, k := range []string{"project", "namespace"} {
		if row.String(k) == "" {
			rep.Add("warn", "%s header: missing %s", name, k)
		}
	}
	// Topology A pins the header to board.jsonl line 1: extra content after
	// it is a shape violation. On topology B the header lives on line 1 of
	// tasks.jsonl (task rows legitimately follow it) and board.jsonl does
	// not exist, so there is nothing extra to flag.
	if b.IsTopologyA() && len(lines) > 1 {
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
