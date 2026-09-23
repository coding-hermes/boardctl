package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// StatusVocabulary is the canonical write vocabulary. Every write surface
// (create/update/import) rejects anything outside this set — the read-alias
// policy below never relaxes that.
var StatusVocabulary = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"review":      true,
	"blocked":     true,
	"complete":    true,
	"failed":      true,
}

// StatusAliases is the read-alias policy (BT-025): statuses OBSERVED on live
// fleet boards that unambiguously mean one canonical value. Reads (validate,
// list, stats, filters) accept these spellings; writes never do — a write
// must use the canonical form, and validate downgrades an alias hit from an
// error to a warning naming the canonical value so operators can see (and
// fix with `boardctl update <id> --normalize`) the drift instead of learning
// to ignore validate. Lookup keys are the trimmed, lower-cased spelling.
// Deliberately NOT aliases: ambiguous values such as retired, closed, wip,
// or malformed junk — those stay unknown and keep failing validate, because
// coercing them needs a human decision, not a table.
var StatusAliases = map[string]string{
	"completed":   "complete",
	"done":        "complete",
	"todo":        "pending",
	"open":        "pending",
	"in-progress": "in_progress",
	"inprogress":  "in_progress",
	"in_progress": "in_progress",
	"reopen":      "pending",
	"reopened":    "pending",
}

// ResolveStatus classifies a raw status spelling for the read path:
//   - a canonical vocabulary value (or the empty status, which callers treat
//     as its own "missing" case) -> ok=true, canonical=s, alias=false
//   - a known alias (case-insensitive, whitespace-trimmed) -> ok=true,
//     canonical=the canonical form, alias=true
//   - anything else -> ok=false (unknown: validate errors, write paths reject)
func ResolveStatus(s string) (canonical string, ok bool, alias bool) {
	if s == "" || StatusVocabulary[s] {
		return s, true, false
	}
	if c, hit := StatusAliases[strings.ToLower(strings.TrimSpace(s))]; hit {
		return c, true, true
	}
	return s, false, false
}

// NormalizeStatus maps read-side aliases onto the canonical vocabulary
// (completed->complete, done->complete, todo/open->pending, hyphen/case
// variants -> in_progress, reopen/reopened -> pending; see StatusAliases).
// Canonical and unknown statuses pass through unchanged.
func NormalizeStatus(s string) string {
	if c, ok, _ := ResolveStatus(s); ok {
		return c
	}
	return s
}

// GuardResultVocabulary is the canonical write vocabulary for guard_result
// (the update --guard flag's documented set). Values are stored upper-cased.
var GuardResultVocabulary = map[string]bool{
	"PASS": true,
	"FAIL": true,
	"SKIP": true,
}

// CIResultVocabulary is the canonical write vocabulary for ci_result (the
// update --ci flag's documented set). Values are stored upper-cased.
var CIResultVocabulary = map[string]bool{
	"GREEN": true,
	"RED":   true,
	"SKIP":  true,
}

// PriorityVocabulary is the canonical priority set used by every live fleet
// board (P0 through P3).
var PriorityVocabulary = map[string]bool{
	"P0": true,
	"P1": true,
	"P2": true,
	"P3": true,
}

// EventTypeVocabulary is the canonical event_type set enforced by
// AppendEvent: the types the help text enumerates (task_created,
// task_dispatched, task_completed, audit) plus every type boardctl itself
// writes (task_updated) and the task/tick/board lifecycle types observed on
// live fleet boards. Legacy free-form event_type rows already on boards are
// NOT flagged by validate — only new writes are restricted.
var EventTypeVocabulary = map[string]bool{
	"audit":             true,
	"board_bootstrap":   true,
	"board_init":        true,
	"board_migration":   true,
	"dogfood":           true,
	"e2e_verified":      true,
	"idle":              true,
	"spec_created":      true,
	"task_added":        true,
	"task_completed":    true,
	"task_created":      true,
	"task_dispatched":   true,
	"task_evidence":     true,
	"task_started":      true,
	"task_updated":      true,
	"task_verified":     true,
	"tick":              true,
	"worker_dispatched": true,
}

// NormalizeResultValue canonicalizes a guard/ci result spelling: trimmed and
// upper-cased (the form the write path stores).
func NormalizeResultValue(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}

// NormalizePriority maps accepted priority spellings onto the canonical
// P0-P3 vocabulary: bare digits ("0".."3") and case/whitespace variants
// (" p2 ") normalize to P0..P3. Anything else passes through unchanged for
// the caller to reject.
func NormalizePriority(p string) string {
	p = strings.ToUpper(strings.TrimSpace(p))
	switch p {
	case "0":
		return "P0"
	case "1":
		return "P1"
	case "2":
		return "P2"
	case "3":
		return "P3"
	}
	return p
}

// FleetTaskIDPattern is the fleet task-id format: an uppercase PREFIX-SEGMENT
// id with at least one hyphenated segment — BT-023, QA-BOARDCTL-001,
// NEVER-DONE, GAP-104, INT-CI-3. Digits are allowed after the first letter.
// Single-segment ids ("TODO") and anything with lowercase/spaces/punctuation
// ("bad id!", "bt-023") are rejected: downstream fleet tooling parses and
// dedupes by id, so one junk id can wedge task-router and board-scan.
const FleetTaskIDPattern = `^[A-Z][A-Z0-9]+(-[A-Z0-9]+)+$`

var fleetTaskIDRe = regexp.MustCompile(FleetTaskIDPattern)

// MatchesFleetTaskID reports whether id conforms to the fleet task-id format.
func MatchesFleetTaskID(id string) bool {
	return fleetTaskIDRe.MatchString(id)
}

// ValidateFleetTaskID returns nil when id conforms to the fleet task-id
// format; otherwise an error naming the offending value and the expected
// pattern (write paths reject with this; --force bypasses at the caller).
func ValidateFleetTaskID(id string) error {
	if MatchesFleetTaskID(id) {
		return nil
	}
	return fmt.Errorf("task id %q does not match the fleet id format %s (uppercase PREFIX-SEGMENT ids like QA-BOARDCTL-001 or BT-023; pass --force to write it anyway)", id, FleetTaskIDPattern)
}

// TaskFilter restricts `list`/`stats` output.
type TaskFilter struct {
	Status   string // "" = any; canonicalized via NormalizeStatus
	Priority string // "" = any; compared case-insensitively on string form
	All      bool   // include tasks whose id lives in fixtures.jsonl
}

// Match reports whether a task row satisfies the filter.
func (f TaskFilter) Match(row *Row, fixtureIDs map[string]bool) bool {
	id := row.String("id")
	if !f.All && id != "" && fixtureIDs[id] {
		return false
	}
	if f.Status != "" && NormalizeStatus(row.String("status")) != f.Status {
		return false
	}
	if f.Priority != "" {
		got := strings.ToUpper(strings.TrimSpace(row.String("priority")))
		if got != strings.ToUpper(strings.TrimSpace(f.Priority)) {
			return false
		}
	}
	return true
}

// TaskRows reads tasks.jsonl into parsed task rows, skipping the line-1
// header row on topology B (the header is board metadata, not a task).
// Topology A returns every row.
//
// DF-BOARDCTL-9: in SkipBad mode a line that fails to parse is recorded as
// skipped-line evidence and the read continues; in the default mode the
// first bad line aborts the read (status quo).
func (b *Board) TaskRows() ([]*Row, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	if b.SkipBad {
		var rows []*Row
		b.iterParsedTolerant(lines, b.tasksPath, func(row *Row, idx int, _ []byte) error {
			if b.skipTaskLine(lines, idx) {
				return nil
			}
			rows = append(rows, row)
			return nil
		})
		return rows, nil
	}
	var rows []*Row
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil
		}
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.tasksPath, err)
	}
	return rows, nil
}

// ListTasks returns task rows (in file order) matching the filter.
//
// DF-BOARDCTL-9: in SkipBad mode the tasks.jsonl scan is tolerant (bad lines
// ride the skipped-line evidence); fixtures.jsonl, which only contributes an
// exclusion set, is read tolerantly in BOTH modes so one malformed fixture
// row cannot hide task rows — it never aborts the listing.
func (b *Board) ListTasks(f TaskFilter) ([]*Row, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	fid, err := b.FixtureIDs()
	if err != nil {
		return nil, err
	}
	var out []*Row
	for _, r := range rows {
		if f.Match(r, fid) {
			out = append(out, r)
		}
	}
	return out, nil
}

// ShowTask finds a task by parsed id. It searches tasks.jsonl first, then
// fixtures.jsonl (a row may live in either). Returns nil, nil when absent.
// DF-BOARDCTL-9: in SkipBad mode both scans run tolerantly.
func (b *Board) ShowTask(id string) (row *Row, file string, err error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, "", err
	}
	for _, r := range rows {
		if r.String("id") == id {
			return r, b.tasksPath, nil
		}
	}
	if fp := b.FixturesPath(); fp != "" {
		rows, _, err := b.ReadAllRowsTolerant(fp)
		if err != nil {
			return nil, "", err
		}
		for _, r := range rows {
			if r.String("id") == id {
				return r, fp, nil
			}
		}
	}
	return nil, "", nil
}

// OpenFingerprints returns id -> stored finding fingerprint for every OPEN
// row (see OpenFindingStatuses) whose detail carries one. Rows written before
// SG-126 have no fingerprint and are grandfathered: they stay open and
// visible, they simply never match an incoming finding. The first occurrence
// in file order wins when two open rows somehow share a fingerprint.
func (b *Board) OpenFingerprints() (map[string]string, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		if !RowIsOpenFinding(r) {
			continue
		}
		fp := StoredFingerprint(r)
		if fp == "" {
			continue
		}
		id := r.String("id")
		if id == "" {
			continue
		}
		if _, seen := out[id]; !seen {
			out[id] = fp
		}
	}
	return out, nil
}

// EventsForTask returns events whose top-level task_id equals id, in file
// order (detail is opaque; task_id matching is top-level only).
// DF-BOARDCTL-9: in SkipBad mode the events scan is tolerant.
func (b *Board) EventsForTask(id string) ([]*Row, error) {
	var rows []*Row
	var err error
	if b.SkipBad {
		rows, _, err = b.ReadAllRowsTolerant(b.eventsPath)
	} else {
		rows, _, err = ReadAllRows(b.eventsPath)
	}
	if err != nil {
		return nil, err
	}
	var out []*Row
	for _, r := range rows {
		if r.String("task_id") == id {
			out = append(out, r)
		}
	}
	return out, nil
}

// Stats holds status/priority counts over the selectable task set.
type Stats struct {
	Total    int            `json:"total"`
	Status   map[string]int `json:"status"`
	Priority map[string]int `json:"priority"`
}

// ComputeStats tallies counts by status and by priority (string form; numeric
// priorities like 1/2/3 appear as "1"/"2"/"3", distinct from "P1").
// DF-BOARDCTL-9: in SkipBad mode the underlying reads are tolerant; bad lines
// ride the skipped-line evidence instead of aborting the tally.
func (b *Board) ComputeStats(f TaskFilter) (*Stats, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return nil, err
	}
	fid, err := b.FixtureIDs()
	if err != nil {
		return nil, err
	}
	st := &Stats{Status: map[string]int{}, Priority: map[string]int{}}
	for _, r := range rows {
		if !f.Match(r, fid) {
			continue
		}
		st.Total++
		status := NormalizeStatus(r.String("status"))
		if status == "" {
			status = "(none)"
		}
		st.Status[status]++
		if p, ok := priorityLabel(r); ok {
			st.Priority[p]++
		}
	}
	return st, nil
}

// priorityLabel renders a row's priority as its display string: strings as-is
// (P0..P3), JSON numbers as their decimal form ("1".."3"). Boards mix both.
func priorityLabel(r *Row) (string, bool) {
	raw := r.Get("priority")
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
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return "", false
	}
}

// SortedKeys is a small helper for deterministic map rendering.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RenderText renders the stats for humans.
func (s *Stats) RenderText() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("total tasks: %d\n", s.Total))
	sb.WriteString("by status:\n")
	for _, k := range SortedKeys(s.Status) {
		sb.WriteString(fmt.Sprintf("  %-12s %d\n", k, s.Status[k]))
	}
	sb.WriteString("by priority:\n")
	for _, k := range SortedKeys(s.Priority) {
		sb.WriteString(fmt.Sprintf("  %-4s %d\n", k, s.Priority[k]))
	}
	return sb.String()
}

// MarshalRowJSON pretty-prints one row's raw JSON for `show`.
func MarshalRowJSON(row *Row) ([]byte, error) {
	// Rebuild the row object from verbatim field bytes so key order and
	// original values survive, then pretty-print it.
	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range row.Keys {
		if i > 0 {
			sb.WriteString(", ")
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteString(": ")
		sb.Write(row.Vals[k])
	}
	sb.WriteString("}")
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(sb.String()), "", "  "); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// RowJSONCompact renders a row as a single compact JSON object (used by
// `list --json`, preserving each row's verbatim field bytes).
func RowJSONCompact(row *Row) []byte {
	var sb strings.Builder
	sb.WriteString("{")
	for i, k := range row.Keys {
		if i > 0 {
			sb.WriteString(",")
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteString(":")
		sb.Write(row.Vals[k])
	}
	sb.WriteString("}")
	return []byte(sb.String())
}
