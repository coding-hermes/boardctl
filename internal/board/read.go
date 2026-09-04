package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// StatusVocabulary is the canonical write vocabulary. Rows read as
// "completed" are accepted everywhere as an alias for "complete".
var StatusVocabulary = map[string]bool{
	"pending":     true,
	"in_progress": true,
	"review":      true,
	"blocked":     true,
	"complete":    true,
	"failed":      true,
}

// NormalizeStatus maps read-side aliases onto the canonical vocabulary:
// "completed" -> "complete". Unknown statuses pass through unchanged.
func NormalizeStatus(s string) string {
	if s == "completed" {
		return "complete"
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

// ListTasks returns task rows (in file order) matching the filter.
func (b *Board) ListTasks(f TaskFilter) ([]*Row, error) {
	rows, _, err := ReadAllRows(b.tasksPath)
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
func (b *Board) ShowTask(id string) (row *Row, file string, err error) {
	rows, _, err := ReadAllRows(b.tasksPath)
	if err != nil {
		return nil, "", err
	}
	for _, r := range rows {
		if r.String("id") == id {
			return r, b.tasksPath, nil
		}
	}
	if fp := b.FixturesPath(); fp != "" {
		rows, _, err := ReadAllRows(fp)
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

// EventsForTask returns events whose top-level task_id equals id, in file
// order (detail is opaque; task_id matching is top-level only).
func (b *Board) EventsForTask(id string) ([]*Row, error) {
	rows, _, err := ReadAllRows(b.eventsPath)
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
func (b *Board) ComputeStats(f TaskFilter) (*Stats, error) {
	rows, _, err := ReadAllRows(b.tasksPath)
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
