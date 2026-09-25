package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
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
	// Keys is the BT-056 key-uniformity census (see keys.go). It is always
	// populated, so a clean board can QUOTE "N/N rows carry only canon keys"
	// as evidence of zero drift rather than only the absence of a warning.
	Keys KeyDriftSummary
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
//   - task status: canonical vocabulary {pending,in_progress,review,blocked,
//     complete,failed} passes; a known read alias (BT-025: completed, done,
//     todo, open, in-progress/inprogress/in_progress, reopen/reopened) is a
//     WARNING naming the canonical value — writes still reject aliases, and
//     `boardctl update <id> --normalize` is the sanctioned fix; anything else
//     (retired, closed, wip, malformed junk) is an ERROR
//   - BT-007: guard_result/ci_result values on existing rows are itemized
//     warnings when free-form (legacy prose tolerated, not failed)
//   - BT-007: depends_on ids that reference no task row are itemized warnings
//   - BT-055: a depends_on key holding a NON-ARRAY value (legacy
//     hand-written rows; the write path stores []string) is an itemized
//     warning — the cross-check cannot see through the wrong shape, so the
//     warning says the value was treated as no dependencies, and a
//     reference carried as a bare string additionally warns it was not
//     cross-checked
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
	// validateTaskRow itemizes one salvageable row (shared by the default
	// and the DF-BOARDCTL-9 tolerant scan).
	validateTaskRow := func(row *Row, idx int) {
		if b.skipTaskLine(lines, idx) {
			return // topology B: line 1 is the header, not a task row
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
		} else if canonical, ok, alias := ResolveStatus(st); !ok {
			// BT-025: not canonical and not a known read alias — stay an
			// ERROR. Ambiguous spellings (retired, closed, wip, junk) need
			// a human decision; they are never silently coerced.
			rep.Add("error", "tasks.jsonl line %d (task %s): status %q not in vocabulary {%s} (accepted read aliases: %s)",
				idx+1, id, st, strings.Join(sortedVocab(), ","), strings.Join(sortedAliases(), ", "))
		} else if alias {
			// BT-025: known read alias — downgrade to a WARNING naming the
			// canonical value and the sanctioned fix path. Writes still
			// reject the alias spelling; nothing is auto-fixed on read.
			rep.Add("warn", "tasks.jsonl line %d (task %s): status %q is a read alias for %q — canonicalize with 'boardctl update %s --normalize'",
				idx+1, id, st, canonical, id)
		}
		// BT-007: guard_result/ci_result values already on boards must be
		// in the write vocabulary. The canonical form is upper-cased
		// (PASS/FAIL/SKIP and GREEN/RED/SKIP); case is tolerated on read
		// so a lowercase spelling does not flag. Empty/null (never run)
		// rows are skipped; free-form prose values ARE flagged (warn) —
		// legacy boards carry prose in these columns that predates the
		// vocabulary, and flagging (not failing) is the point.
		//
		// BT-049: the message names the FIELD and that field's OWN
		// vocabulary — a single value sits in exactly one column, so the
		// BT-007 union string made a guard-vocab value in ci_result ("PASS")
		// read as if it were rejected everywhere.
		for _, c := range []struct {
			key   string
			vocab map[string]bool
			str   string
		}{
			{"guard_result", GuardResultVocabulary, "{PASS,FAIL,SKIP}"},
			{"ci_result", CIResultVocabulary, "{GREEN,RED,SKIP}"},
		} {
			v := row.String(c.key)
			if v == "" {
				continue // absent, null, or never-run
			}
			if !c.vocab[NormalizeResultValue(v)] {
				rep.Add("warn", "tasks.jsonl line %d (task %s): %s %q is free-form — not in the %s vocabulary %s (writes now enforce this; hand-edit the row or rewrite via boardctl update)",
					idx+1, id, c.key, v, c.key, c.str)
			}
		}
		// BT-048: the priority column has a canonical on-disk vocabulary
		// {P0,P1,P2,P3} (the write path normalizes bare digits and case
		// variants, rejecting the rest, since BT-007), but hand-edited or
		// legacy rows carry other spellings (28 live rows measured
		// 2026-09-22: bare digits, P4, lowercase p2, prose). Unlike the
		// guard/ci checks above, membership is tested on the RAW stored
		// value with NO case/digit tolerance: those spellings are exactly
		// the drift class this warning exists to catch. Empty/absent
		// priority stays silent (no new missing-field class);
		// NormalizePriority only names the canonical form when one exists.
		p := row.String("priority")
		if p != "" && !PriorityVocabulary[p] {
			if canon := NormalizePriority(p); PriorityVocabulary[canon] {
				rep.Add("warn", "tasks.jsonl line %d (task %s): priority %q is not in vocabulary {P0,P1,P2,P3} — canonical form is %q (writes normalize bare 0-3 and case variants; hand-edit the row or rewrite via boardctl update)",
					idx+1, id, p, canon)
			} else {
				rep.Add("warn", "tasks.jsonl line %d (task %s): priority %q is not in vocabulary {P0,P1,P2,P3} (writes reject this value; hand-edit the row or rewrite via boardctl update)",
					idx+1, id, p)
			}
		}
		// BT-060: a title carrying a Pn token that DISAGREES with the row's
		// priority field is a drift tell — the fleet titles rows "[P1] ..."
		// by hand and then re-priorities the row, so the token goes stale.
		// The priority FIELD wins for what the row means (BT-048: it is the
		// machine-read value); the warning only points at the stale token.
		// Rules: exactly one P0-P3 token is searched (the same regex used in
		// the warning text); out-of-vocabulary spellings (P4, P9) are NOT
		// this class — the BT-048 priority check already owns the row's
		// priority drift, and guessing what P4 "should" be would invent a
		// decision. Multiple tokens each get their own warning (a title like
		// "[P2] fix P1 regression" carries two independent tells). No token,
		// no finding; exit stays 0 (warn severity, like every cross-check).
		for _, tok := range TitlePriorityTokens(row.String("title")) {
			if PriorityVocabulary[tok] && tok != p {
				rep.Add("warn", "tasks.jsonl line %d (task %s): title carries %q but priority is %q — the priority field wins; rewrite the title with 'boardctl update %s --title', or re-file the row with 'boardctl create --id %s --force --priority <Pn>' if the field is the stale half (update has no --priority)",
					idx+1, id, tok, p, id, id)
			}
		}
		// BT-055: the depends_on shape is checked before the cross-check
		// below. rowStringSlice deliberately returns nil for any non-array
		// value, which made a malformed shape (legacy/hand-written rows:
		// the write path stores []string, so boardctl itself can never
		// emit one) read as "no dependencies" — and a real reference in
		// the wrong shape then escaped the dangling-id warning entirely.
		// The shape problem is itemized here (WARN: existing boards with
		// legacy rows must stay exit-0), and a non-empty string value
		// additionally warns that the reference it may carry was NOT
		// cross-checked. Absent keys and null stay silent (no-dependencies
		// is the write path's own neutral spelling for unset).
		if raw := row.Get("depends_on"); raw != nil {
			if kind := jsonKindName(raw); kind != "" && kind != "array" {
				rep.Add("warn", "tasks.jsonl line %d (task %s): depends_on is not an array (%s) — treated as no dependencies",
					idx+1, id, kind)
				if kind == "string" {
					if s, ok := decodeJSONString(raw); ok && strings.TrimSpace(s) != "" {
						rep.Add("warn", "tasks.jsonl line %d (task %s): depends_on value %q looks like a reference but is not an array element — it was not cross-checked against task ids",
							idx+1, id, s)
					}
				}
			}
		}
		// BT-007: collect depends_on for the existence cross-check below.
		for _, dep := range rowStringSlice(row, "depends_on") {
			depends[dep] = append(depends[dep], depRef{line: idx + 1, id: id})
		}
		// BT-056: the row's KEY SET must stay inside the declared canon
		// (keys.go). Until this check existed, "the row parsed" was the only
		// shape test, so the fleet drifted to ~230 distinct keys — one-key
		// dialects, composite squatters and typos — with nothing reporting
		// it. A known alias still counts as drift here: it has a named
		// repair (`update --normalize`), not an exemption.
		rep.Keys.AddRow(row)
		if unknown := UnknownTaskKeys(row); len(unknown) > 0 {
			who := "task " + id
			if id == "" {
				who = "row"
			}
			fix := ""
			if fixableByAlias(row, unknown) {
				fix = " — fixable with 'boardctl update " + id + " --normalize'"
			}
			rep.Add("warn", "tasks.jsonl line %d (%s): key(s) {%s} outside the declared canon%s",
				idx+1, who, strings.Join(unknown, ", "), fix)
		}
	}
	if b.SkipBad {
		// DF-BOARDCTL-9: the tolerant pass keeps validating the salvageable
		// rows; every skipped line rides the board's evidence list and is
		// itemized below as an error finding, so a degraded board still
		// reports FAIL with the evidence of WHAT is broken — while the
		// surviving rows are listed and counted.
		b.iterParsedTolerant(lines, b.tasksPath, func(row *Row, idx int, _ []byte) error {
			validateTaskRow(row, idx)
			return nil
		})
	} else {
		ierr := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
			validateTaskRow(row, idx)
			return nil
		})
		if ierr != nil {
			rep.Add("error", "tasks.jsonl: %v", ierr)
		}
	}
	for _, s := range b.SkippedLines() {
		if s.Path == b.tasksPath {
			rep.Add("error", "tasks.jsonl line %d: SKIPPED unparseable row — %s (salvage with 'boardctl validate --repair'; reads need --skip-bad-lines)", s.Line, s.Err)
		}
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

// DanglingDepMsg is the exact substring of the dangling-depends_on warning
// class. BT-054-R: the pre-commit hook needs a stable text anchor to promote
// this one warning class to a blocker (via `boardctl validate --fail-on
// dangling-dep`) without re-parsing the report; the wording above stays
// byte-identical so existing tests pinning the message keep passing.
const DanglingDepMsg = "depends_on references nonexistent task id"

// CountDanglingDepWarns returns the number of dangling depends_on warnings in
// a validate report — the exact class --fail-on dangling-dep promotes.
func (r *Report) CountDanglingDepWarns() int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, DanglingDepMsg) {
			n++
		}
	}
	return n
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

// jsonKindName names the JSON kind of a stored field value — "array",
// "string", "number", "boolean", "object", "null", or "" when the bytes are
// not a complete JSON value (the field never parsed cleanly). Used by the
// BT-055 depends_on shape warning to describe what is actually stored.
func jsonKindName(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return ""
	}
	switch v.(type) {
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return ""
	}
}

// decodeJSONString decodes a stored field value as a JSON string, reporting
// ok=false for anything else (BT-055: only the string kind needs its content
// named in the not-cross-checked warning).
func decodeJSONString(raw []byte) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var s string
	if err := dec.Decode(&s); err != nil {
		return "", false
	}
	return s, true
}

// titlePriorityToken matches a P0-P3 token standing ALONE in a title: the
// letter P (any case — the drift class includes hand-typed "[p1]") preceded
// by the start of the string or a non-alphanumeric character, followed by a
// single digit 0-3, followed by the end of the string or a non-alphanumeric
// character. "P1" inside "P10" or "API2" does not match (the digit and the
// leading letter are part of a larger token); "[P1]" and "P1:" and "p1 " do.
var titlePriorityToken = regexp.MustCompile(`(^|[^A-Za-z0-9])([Pp][0-3])([^A-Za-z0-9]|$)`)

// TitlePriorityTokens returns the priority tokens a title carries, in
// order of appearance (BT-060's title-priority cross-check). Multiple
// tokens each yield an entry — "[P1] a (P2) b" and the adjacent form
// "P1 P2" both yield two tokens, each an independent drift tell when it
// disagrees with the row's priority field. The scan restarts at the end
// of the TOKEN, not the end of the whole match: RE2 has no lookahead, so
// the boundary character after a token must stay consumable as the START
// boundary of the next one.
func TitlePriorityTokens(title string) []string {
	var out []string
	pos := 0
	for pos <= len(title) {
		loc := titlePriorityToken.FindStringSubmatchIndex(title[pos:])
		if loc == nil {
			break
		}
		tokStart, tokEnd := pos+loc[4], pos+loc[5]
		out = append(out, strings.ToUpper(title[tokStart:tokEnd]))
		if tokEnd <= pos { // cannot happen ([Pp][0-3] is 2 bytes); guard anyway
			break
		}
		pos = tokEnd
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
//
// BT-037: a HEADERLESS board (no board.jsonl, tasks.jsonl line 1 is an
// ordinary task row) has no header to check. Reading that task row as if it
// were a header invented three false "not an integer counter" errors on a
// clean board, so the counter checks are skipped with ONE informational line
// instead.
func (b *Board) validateHeader(rep *Report) error {
	if !b.HasHeader() {
		rep.Add("warn", "headerless board (no board.jsonl): tasks.jsonl line 1 is a task row, not board metadata — no header to validate; header counter checks skipped")
		return nil
	}
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
	ierr := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
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
	if ierr != nil {
		rep.Add("error", "fixtures.jsonl: %v", ierr)
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

// sortedAliases renders the read-alias table deterministically (as
// "alias->canonical" pairs) for validate's unknown-status error message.
func sortedAliases() []string {
	out := make([]string, 0, len(StatusAliases))
	for alias, canonical := range StatusAliases {
		out = append(out, alias+"->"+canonical)
	}
	sort.Strings(out)
	return out
}

// RenderText renders the itemized report.
func (r *Report) RenderText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "board: %s (topology %s)\n", r.Dir, r.Topology)
	fmt.Fprintf(&sb, "rows: %d tasks, %d events, %d fixtures%s\n", r.Tasks, r.Events, r.Fixtures, headerNote(r.Header))
	fmt.Fprintf(&sb, "%s\n", r.Keys.Summary())
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
