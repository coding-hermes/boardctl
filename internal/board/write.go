package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrDuplicateTaskID is returned by Create when the id already exists.
type ErrDuplicateTaskID struct{ ID string }

func (e *ErrDuplicateTaskID) Error() string {
	return fmt.Sprintf("task id %q already exists in tasks.jsonl — create aborted", e.ID)
}

func vocabList() string {
	var ks []string
	for k := range StatusVocabulary {
		ks = append(ks, k)
	}
	return strings.Join(ks, ",")
}

// DefaultTaskRowKeys is the built-in default task schema used when creating
// a task on an EMPTY tasks.jsonl (the freshly initialized state `init`
// produces): the standard fields the fleet's own board writes, in fleet
// order. Values come from neutralValue + the create overrides; every key is
// present so the first row doubles as the mirror template for the second.
var DefaultTaskRowKeys = []string{
	"id", "title", "status", "priority", "complexity",
	"depends_on", "blocks",
	"primary_model", "primary_provider", "fallback_model", "fallback_provider",
	"reasoning", "capability_tags",
	"worker_status", "dispatched_at", "completed_at",
	"attempts", "exit_code", "commit_hash",
	"files_changed", "lines_added", "lines_removed",
	"guard_result", "ci_result",
	"worker_summary", "foreman_note", "blocked_reason", "review_notes",
	"created_at", "updated_at", "blocked_since",
}

// rawLastLine returns the last non-empty line of a JSONL file ("" if none).
func rawLastLine(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := bytes.Split(content, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		if len(bytes.TrimSpace(lines[i])) > 0 {
			return lines[i]
		}
	}
	return nil
}

// appendBytes appends with O_APPEND semantics — never rewrites the file.
func appendBytes(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// atomicRewrite replaces file content via a same-directory temp file + rename.
// Callers must verify untouched content beforehand.
func atomicRewrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".boardctl-rewrite-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// TaskRowSpec carries the user-supplied create fields.
type TaskRowSpec struct {
	ID             string
	Title          string
	Status         string
	Priority       string
	Complexity     *int64
	DependsOn      []string
	HasDependsOn   bool
	Reasoning      string
	CapabilityTags []string
	HasTags        bool
}

// Create appends a new task row, deep-copying the schema (key set, key
// order, serialization style, timestamp dialect) of the LAST tasks.jsonl row
// and overriding values: id/title are required, status defaults to "pending",
// priority "P2", complexity 3. Values the user did not supply are reset to
// pending-neutral defaults rather than inheriting the mirrored row's values,
// so a fresh row is always a clean pending task no matter what the mirrored
// row held. Append-only; fails when the parsed id already exists, or on
// topology B (boardctl never writes caches).
//
// On an EMPTY tasks.jsonl (the freshly initialized state `init` produces)
// there is no row to mirror; Create then builds the row from the BUILT-IN
// DEFAULT TASK SCHEMA (DefaultTaskRow: the standard fields the fleet's own
// board writes — id, title, status, priority, complexity, depends_on, blocks,
// primary/fallback model+provider, reasoning, capability_tags, worker_status,
// dispatch/completion timestamps, counters, guard/ci results, summaries,
// created_at/updated_at) instead of failing.
func (b *Board) Create(spec TaskRowSpec) (string, error) {
	if b.Topology != "A" {
		return "", TopologyBWriteError
	}
	if spec.ID == "" {
		return "", fmt.Errorf("create requires --id")
	}
	if spec.Title == "" {
		return "", fmt.Errorf("create requires --title")
	}
	status := spec.Status
	if status == "" {
		status = "pending"
	}
	if status == "completed" {
		return "", fmt.Errorf("write status %q is not allowed — the vocabulary writes 'complete' (singular); 'completed' is a read-side alias only", status)
	}
	if !StatusVocabulary[status] {
		return "", fmt.Errorf("status %q not in write vocabulary {%s}", status, vocabList())
	}

	rows, last, err := b.lastTaskRow()
	if err != nil {
		return "", err
	}
	for _, r := range rows { // duplicate check on PARSED ids, never substrings
		if r.String("id") == spec.ID {
			return "", &ErrDuplicateTaskID{ID: spec.ID}
		}
	}

	var style Style
	var nowStr string
	var neutral func(key string) json.RawMessage
	row := &Row{Keys: nil, Vals: map[string]json.RawMessage{}}
	if last != nil {
		// Mirror key order, neutralize values, then apply spec overrides.
		style = DetectStyle(rawLastLine(b.tasksPath))
		nowStr = b.tasksTSFormat(last).Now()
		neutral = neutralValue
		row.Keys = append(row.Keys, last.Keys...)
		for _, k := range last.Keys {
			row.Vals[k] = neutral(k)
		}
	} else {
		// Empty tasks.jsonl: fresh schema, fleet default serialization.
		style = DefaultStyle()
		nowStr = TSFormat{Layout: DefaultTSLayout}.Now()
		neutral = neutralValue // same pending-neutral semantics as mirroring
		row.Keys = append(row.Keys, DefaultTaskRowKeys...)
		for _, k := range DefaultTaskRowKeys {
			row.Vals[k] = neutral(k)
		}
	}
	set := func(key string, v any) error { return row.SetGoValue(key, v, style) }

	priority := spec.Priority
	if priority == "" {
		priority = "P2"
	}
	complexity := int64(3)
	if spec.Complexity != nil {
		complexity = *spec.Complexity
	}
	overrides := map[string]any{
		"id":         spec.ID,
		"title":      spec.Title,
		"status":     status,
		"priority":   priority,
		"complexity": complexity,
	}
	if spec.HasDependsOn {
		overrides["depends_on"] = spec.DependsOn
	}
	if spec.HasTags {
		overrides["capability_tags"] = spec.CapabilityTags
	}
	if spec.Reasoning != "" {
		overrides["reasoning"] = spec.Reasoning
	}
	overrides["created_at"] = nowStr
	overrides["updated_at"] = nowStr
	for k, v := range overrides {
		if row.Has(k) {
			if err := set(k, v); err != nil {
				return "", err
			}
		}
	}
	// Keys the user asked for but the mirrored schema lacks: append at end.
	for _, k := range []string{"depends_on", "capability_tags", "reasoning"} {
		if v, ok := overrides[k]; ok && !row.Has(k) {
			if err := set(k, v); err != nil {
				return "", err
			}
		}
	}
	// Guarantee the core schema even for degenerate mirrored rows.
	for _, k := range []string{"priority", "complexity", "created_at", "updated_at", "id", "title", "status"} {
		if !row.Has(k) {
			if err := set(k, overrides[k]); err != nil {
				return "", err
			}
		}
	}

	line := append(row.Marshal(style), '\n')
	if err := appendBytes(b.tasksPath, line); err != nil {
		return "", err
	}
	return spec.ID, nil
}

// lastTaskRow parses tasks.jsonl, returning all rows and the last one.
func (b *Board) lastTaskRow() (rows []*Row, last *Row, err error) {
	rows, _, err = ReadAllRows(b.tasksPath)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) > 0 {
		last = rows[len(rows)-1]
	}
	return rows, last, nil
}

// tasksTSFormat samples the timestamp dialect of tasks.jsonl from the last
// row's updated_at (fallback created_at), else the spec default.
func (b *Board) tasksTSFormat(last *Row) TSFormat {
	if last == nil {
		if rows, _, err := ReadAllRows(b.tasksPath); err == nil && len(rows) > 0 {
			last = rows[len(rows)-1]
		}
	}
	sample := ""
	if last != nil {
		sample = last.String("updated_at")
		if sample == "" {
			sample = last.String("created_at")
		}
	}
	if sample == "" {
		return TSFormat{Layout: DefaultTSLayout}
	}
	return DetectTSLayout(sample)
}

// neutralValue returns a pending-neutral value for a freshly created task
// row key. depends_on/blocks/capability_tags-style keys keep an array shape;
// counters zero; everything else nulls.
func neutralValue(key string) json.RawMessage {
	switch key {
	case "depends_on", "blocks", "capability_tags", "files_changed":
		return json.RawMessage("[]")
	case "attempts", "lines_added", "lines_removed":
		return json.RawMessage("0")
	case "status":
		return json.RawMessage(`"pending"`)
	case "worker_status":
		return json.RawMessage(`"pending"`)
	default:
		return json.RawMessage("null")
	}
}

// UpdateSpec carries the user-supplied update fields (nil pointer = untouched).
type UpdateSpec struct {
	Status        *string
	WorkerStatus  *string
	CommitHash    *string
	Guard         *string // stored upper-cased (PASS|FAIL|SKIP)
	CI            *string // stored upper-cased (GREEN|RED|SKIP)
	Summary       *string // worker_summary
	Note          *string // foreman_note
	BlockedReason *string
	CompletedAt   *string
}

// UpdateTask surgically updates ONE task row. Every untouched line of
// tasks.jsonl — and every untouched field byte of the target row (kept as
// verbatim RawMessage) — round-trips byte-identical. Implemented as
// read-lines -> replace exactly one line -> rejoin -> assert untouched lines
// identical -> atomic rewrite. updated_at is refreshed (when the row carries
// the key) in the row's own timestamp dialect. Fields the flag sets but the
// row lacks are appended at the end of the row.
func (b *Board) UpdateTask(id string, spec UpdateSpec) ([]string, error) {
	if b.Topology != "A" {
		return nil, TopologyBWriteError
	}
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	targetIdx := -1
	var target *Row
	if err := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if row.String("id") != id {
			return nil
		}
		if targetIdx != -1 {
			return fmt.Errorf("task id %q appears on multiple lines (%d and %d) — refusing ambiguous update", id, targetIdx+1, idx+1)
		}
		targetIdx = idx
		target = row
		return nil
	}); err != nil {
		return nil, err
	}
	if targetIdx == -1 {
		return nil, fmt.Errorf("task %q not found in tasks.jsonl", id)
	}

	style := DetectStyle(lines[targetIdx])
	now := DetectTSLayout(target.String("updated_at")).Now()

	var changed []string
	set := func(key string, v any) error {
		if err := target.SetGoValue(key, v, style); err != nil {
			return err
		}
		changed = append(changed, key)
		return nil
	}
	if spec.Status != nil {
		s := *spec.Status
		if s == "completed" {
			return nil, fmt.Errorf("write status %q is not allowed — the vocabulary writes 'complete' (singular); 'completed' is a read-side alias only", s)
		}
		if !StatusVocabulary[s] {
			return nil, fmt.Errorf("status %q not in write vocabulary {%s}", s, vocabList())
		}
		if err := set("status", s); err != nil {
			return nil, err
		}
	}
	if spec.WorkerStatus != nil {
		if err := set("worker_status", *spec.WorkerStatus); err != nil {
			return nil, err
		}
	}
	if spec.CommitHash != nil {
		if err := set("commit_hash", *spec.CommitHash); err != nil {
			return nil, err
		}
	}
	if spec.Guard != nil {
		if err := set("guard_result", strings.ToUpper(*spec.Guard)); err != nil {
			return nil, err
		}
	}
	if spec.CI != nil {
		if err := set("ci_result", strings.ToUpper(*spec.CI)); err != nil {
			return nil, err
		}
	}
	if spec.Summary != nil {
		if err := set("worker_summary", *spec.Summary); err != nil {
			return nil, err
		}
	}
	if spec.Note != nil {
		if err := set("foreman_note", *spec.Note); err != nil {
			return nil, err
		}
	}
	if spec.BlockedReason != nil {
		if err := set("blocked_reason", *spec.BlockedReason); err != nil {
			return nil, err
		}
	}
	if spec.CompletedAt != nil {
		if err := set("completed_at", *spec.CompletedAt); err != nil {
			return nil, err
		}
	}
	if target.Has("updated_at") {
		if err := set("updated_at", now); err != nil {
			return nil, err
		}
	}
	if len(changed) == 0 {
		return nil, fmt.Errorf("update requires at least one change flag (--status/--worker-status/--commit-hash/--guard/--ci/--summary/--note/--blocked-reason/--completed-at)")
	}

	newLines := make([][]byte, len(lines))
	copy(newLines, lines)
	newLines[targetIdx] = target.Marshal(style)
	// Assert every untouched line round-trips byte-identical BEFORE writing.
	for i, l := range lines {
		if i != targetIdx && !bytes.Equal(l, newLines[i]) {
			return nil, fmt.Errorf("internal error: untouched line %d would change — update aborted (nothing written)", i+1)
		}
	}
	if err := atomicRewrite(b.tasksPath, JoinLines(newLines)); err != nil {
		return nil, err
	}
	return changed, nil
}

// EventSpec carries the user-supplied event fields.
type EventSpec struct {
	Type       string
	TaskID     string
	Actor      string
	Detail     []byte // raw JSON payload from --detail @file (nil if unused)
	DetailText *string
	Tick       *int64
}

// AppendEvent appends one event row: id = MAX(existing numeric id)+1,
// timestamp in the last row's dialect, schema/key-order mirrored from the
// last event row (unknown keys nulled, tick_number added when --tick is given
// and the mirrored row lacks it). Append-only — never rewrites events.jsonl.
func (b *Board) AppendEvent(spec EventSpec) (int64, error) {
	if b.Topology != "A" {
		return 0, TopologyBWriteError
	}
	rows, _, err := ReadAllRows(b.eventsPath)
	if err != nil {
		return 0, err
	}
	var maxID int64
	var last *Row
	for _, r := range rows {
		if id, ok := r.Int("id"); ok && id > maxID {
			maxID = id
		}
		last = r
	}
	nextID := maxID + 1

	style := DefaultStyle()
	tsf := TSFormat{Layout: DefaultTSLayout}
	rawLast := rawLastLine(b.eventsPath)
	if last != nil {
		style = DetectStyle(rawLast)
		tsf = DetectTSLayout(last.String("timestamp"))
	}

	row := &Row{Keys: nil, Vals: map[string]json.RawMessage{}}
	if last != nil {
		row.Keys = append(row.Keys, last.Keys...)
		for _, k := range row.Keys {
			row.Vals[k] = json.RawMessage("null")
		}
	} else {
		row.Keys = []string{"id", "timestamp", "event_type", "task_id", "actor", "detail", "tick_number"}
		for _, k := range row.Keys {
			row.Vals[k] = json.RawMessage("null")
		}
	}
	if spec.Tick != nil && !row.Has("tick_number") {
		row.SetRaw("tick_number", json.RawMessage("null"))
	}

	etype := spec.Type
	if etype == "" {
		etype = "audit"
	}
	actor := spec.Actor
	if actor == "" {
		actor = "foreman"
	}
	set := func(key string, v any) error { return row.SetGoValue(key, v, style) }
	if err := set("id", nextID); err != nil {
		return 0, err
	}
	if err := set("timestamp", tsf.Now()); err != nil {
		return 0, err
	}
	if err := set("event_type", etype); err != nil {
		return 0, err
	}
	if err := set("task_id", spec.TaskID); err != nil {
		return 0, err
	}
	if err := set("actor", actor); err != nil {
		return 0, err
	}
	if row.Has("tick_number") {
		if spec.Tick != nil {
			if err := set("tick_number", *spec.Tick); err != nil {
				return 0, err
			}
		}
	}
	// detail: --detail @file (raw JSON payload embedded as an escaped string),
	// then --detail-text (plain string), else null. Single JSON-encoding,
	// matching the fleet appenders; style.ASCII escapes raw non-ASCII content.
	switch {
	case spec.Detail != nil:
		content := bytes.TrimSpace(spec.Detail)
		if err := set("detail", embedDetail(content, style)); err != nil {
			return 0, err
		}
	case spec.DetailText != nil:
		if err := set("detail", embedDetail([]byte(*spec.DetailText), style)); err != nil {
			return 0, err
		}
	default:
		if err := set("detail", nil); err != nil {
			return 0, err
		}
	}
	line := append(row.Marshal(style), '\n')
	if err := appendBytes(b.eventsPath, line); err != nil {
		return 0, err
	}
	return nextID, nil
}

// embedDetail encodes detail content as the JSON-string VALUE of the detail
// field: raw := jsonString(content) makes the outer string; the content's own
// JSON quoting is preserved so decoding detail yields the original payload.
func embedDetail(content []byte, s Style) []byte {
	if len(content) == 0 {
		return jsonString("", s)
	}
	return jsonString(string(content), s)
}

// HeaderUpdate carries --set fields (nil pointer = untouched).
type HeaderUpdate struct {
	TicksTotal *int64
	TicksIdle  *int64
	LastCommit *string
}

// SetHeader rewrites ONLY line 1 of board.jsonl (topology A) with the given
// --set fields; every untouched header field keeps its verbatim bytes and any
// other lines round-trip byte-identical (asserted before the atomic write).
func (b *Board) SetHeader(u HeaderUpdate) ([]string, error) {
	if b.Topology != "A" {
		return nil, TopologyBWriteError
	}
	lines, err := ReadJSONLLines(b.headerPath)
	if err != nil {
		return nil, err
	}
	headerIdx := -1
	var header *Row
	for i, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return nil, fmt.Errorf("board.jsonl line %d: %w", i+1, err)
		}
		header = row
		headerIdx = i
		break
	}
	if header == nil {
		return nil, fmt.Errorf("board.jsonl is empty")
	}
	if u.TicksTotal == nil && u.TicksIdle == nil && u.LastCommit == nil {
		return nil, fmt.Errorf("header requires at least one --set flag (--set-ticks-total/--set-ticks-idle/--set-last-commit)")
	}
	style := DetectStyle(lines[headerIdx])
	var changed []string
	set := func(key string, v any) error {
		if err := header.SetGoValue(key, v, style); err != nil {
			return err
		}
		changed = append(changed, key)
		return nil
	}
	if u.TicksTotal != nil {
		if err := set("ticks_total", *u.TicksTotal); err != nil {
			return nil, err
		}
	}
	if u.TicksIdle != nil {
		if err := set("ticks_idle", *u.TicksIdle); err != nil {
			return nil, err
		}
	}
	if u.LastCommit != nil {
		if err := set("last_commit", *u.LastCommit); err != nil {
			return nil, err
		}
	}
	newLines := make([][]byte, len(lines))
	copy(newLines, lines)
	newLines[headerIdx] = header.Marshal(style)
	for i, l := range lines {
		if i != headerIdx && !bytes.Equal(l, newLines[i]) {
			return nil, fmt.Errorf("internal error: untouched line %d of board.jsonl would change — header update aborted (nothing written)", i+1)
		}
	}
	if err := atomicRewrite(b.headerPath, JoinLines(newLines)); err != nil {
		return nil, err
	}
	return changed, nil
}
