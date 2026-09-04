package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// sortedEventVocab renders EventTypeVocabulary deterministically for error
// messages.
func sortedEventVocab() string {
	out := make([]string, 0, len(EventTypeVocabulary))
	for k := range EventTypeVocabulary {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
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
// row held. Append-only; fails when the parsed id already exists.
// Works on BOTH topologies (BT-010): on topology B the header is line 1 of
// tasks.jsonl and is neither mirrored, dup-checked, nor overwritten — the
// append lands after the last task row.
//
// On an EMPTY tasks.jsonl (the freshly initialized state `init` produces)
// there is no row to mirror; Create then builds the row from the BUILT-IN
// DEFAULT TASK SCHEMA (DefaultTaskRow: the standard fields the fleet's own
// board writes — id, title, status, priority, complexity, depends_on, blocks,
// primary/fallback model+provider, reasoning, capability_tags, worker_status,
// dispatch/completion timestamps, counters, guard/ci results, summaries,
// created_at/updated_at) instead of failing.
func (b *Board) Create(spec TaskRowSpec) (string, error) {
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
	// BT-007: priority is normalized-or-rejected at write time — bare
	// digits ("1") and case variants ("p2") map onto the canonical
	// P0-P3 set every fleet board uses, so stats never splits groups.
	priority := NormalizePriority(spec.Priority)
	if priority == "" {
		priority = "P2"
	}
	if !PriorityVocabulary[priority] {
		return "", fmt.Errorf("priority %q not in vocabulary {P0,P1,P2,P3} (bare 0-3 are normalized; use e.g. --priority P1)", spec.Priority)
	}
	spec.Priority = priority

	rows, last, err := b.lastTaskRow()
	if err != nil {
		return "", err
	}
	for _, r := range rows { // duplicate check on PARSED ids, never substrings
		if r.String("id") == spec.ID {
			return "", &ErrDuplicateTaskID{ID: spec.ID}
		}
	}

	// BT-007: dependency ids must exist in tasks.jsonl. REJECTED at create
	// (not merely warned): the create is a no-op abort — nothing is written
	// — and the append-only store otherwise bakes a dangling reference into
	// board history that no later edit can repair.
	if spec.HasDependsOn {
		known := make(map[string]bool, len(rows))
		for _, r := range rows {
			if id := r.String("id"); id != "" {
				known[id] = true
			}
		}
		var missing []string
		for _, dep := range spec.DependsOn {
			if !known[dep] {
				missing = append(missing, dep)
			}
		}
		if len(missing) > 0 {
			return "", fmt.Errorf("depends_on references nonexistent task id(s): %s — create aborted (create the dependency task first)", strings.Join(missing, ", "))
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

	// spec.Priority is already normalized (P0-P3, default P2) and validated
	// above; complexity defaults to 3.
	complexity := int64(3)
	if spec.Complexity != nil {
		complexity = *spec.Complexity
	}
	overrides := map[string]any{
		"id":         spec.ID,
		"title":      spec.Title,
		"status":     status,
		"priority":   spec.Priority,
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
	// README-promised audit trail: every successful create appends a
	// task_created event. Runs only after the task row is durably appended;
	// the task row is kept even if the event append fails (the row IS the
	// board state; the event is the trail), but the error surfaces.
	if _, err := b.AppendEvent(EventSpec{
		Type:   "task_created",
		TaskID: spec.ID,
		Detail: []byte(fmt.Sprintf(`{"id":%q,"status":%q}`, spec.ID, status)),
	}); err != nil {
		return "", fmt.Errorf("task row appended but task_created event failed: %w", err)
	}
	return spec.ID, nil
}

// lastTaskRow parses tasks.jsonl, returning all task rows and the last one.
// Topology B: line 1 is the board header (metadata, no task id) and is
// skipped — it must never become the mirror template nor part of the
// duplicate-id scan.
func (b *Board) lastTaskRow() (rows []*Row, last *Row, err error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, nil, err
	}
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil
		}
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", b.tasksPath, err)
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
		lastRows, _, err := b.lastTaskRow()
		if err == nil && len(lastRows) > 0 {
			last = lastRows[len(lastRows)-1]
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
// Works on topology B too (BT-010): the header is line 1 of tasks.jsonl and
// is never an update target; with --commit-hash the header's last_commit is
// bumped IN LINE 1 via SetHeader, leaving the rest of the file untouched.
func (b *Board) UpdateTask(id string, spec UpdateSpec) ([]string, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	targetIdx := -1
	var target *Row
	if err := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: the header is not a task row
		}
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
		// BT-007: guard_result is vocabulary-checked at write time (the
		// update --guard flag's documented PASS|FAIL|SKIP set), stored
		// upper-cased like the existing write path.
		g := NormalizeResultValue(*spec.Guard)
		if !GuardResultVocabulary[g] {
			return nil, fmt.Errorf("guard %q not in vocabulary {PASS,FAIL,SKIP}", *spec.Guard)
		}
		if err := set("guard_result", g); err != nil {
			return nil, err
		}
	}
	if spec.CI != nil {
		// BT-007: ci_result is vocabulary-checked at write time (the
		// update --ci flag's documented GREEN|RED|SKIP set), stored
		// upper-cased like the existing write path.
		c := NormalizeResultValue(*spec.CI)
		if !CIResultVocabulary[c] {
			return nil, fmt.Errorf("ci %q not in vocabulary {GREEN,RED,SKIP}", *spec.CI)
		}
		if err := set("ci_result", c); err != nil {
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
	// README-promised audit trail, appended only after the row rewrite
	// succeeds. A status flag produces exactly one event: task_completed when
	// the write form "complete" is set, task_updated for any other vocabulary
	// value. A commit hash bumps the board header's last_commit (the "header
	// bump" the README promises) without an extra event.
	if spec.Status != nil {
		etype := "task_updated"
		if *spec.Status == "complete" {
			etype = "task_completed"
		}
		if _, err := b.AppendEvent(EventSpec{
			Type:   etype,
			TaskID: id,
			Detail: []byte(fmt.Sprintf(`{"status":%q}`, *spec.Status)),
		}); err != nil {
			return changed, fmt.Errorf("task row updated but %s event failed: %w", etype, err)
		}
	}
	if spec.CommitHash != nil {
		if _, err := b.SetHeader(HeaderUpdate{LastCommit: spec.CommitHash}); err != nil {
			return changed, fmt.Errorf("task row updated but header bump failed: %w", err)
		}
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
// events.jsonl has no header in ANY topology, so this works unchanged on
// topology-B boards (BT-010).
func (b *Board) AppendEvent(spec EventSpec) (int64, error) {
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
	// BT-007: event_type is vocabulary-checked at write time — the set the
	// help text enumerates plus every type boardctl itself writes and the
	// task/tick/board lifecycle types observed on live fleet boards. Legacy
	// free-form rows already on boards are tolerated (validate does not flag
	// them); only NEW event rows are restricted.
	if !EventTypeVocabulary[etype] {
		return 0, fmt.Errorf("event type %q not in vocabulary {%s} — use one of the enumerated event_type values", etype, sortedEventVocab())
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
	// embedDetail already returns the exact JSON string VALUE, so it is
	// written verbatim via SetRaw — routing it through SetGoValue would run
	// encodeValue's default branch (json.Marshal on []byte) and base64-mangle
	// the payload.
	switch {
	case spec.Detail != nil:
		row.SetRaw("detail", embedDetail(bytes.TrimSpace(spec.Detail), style))
	case spec.DetailText != nil:
		row.SetRaw("detail", embedDetail([]byte(*spec.DetailText), style))
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

// SetHeader rewrites ONLY the header row — line 1 of board.jsonl (topology A)
// or line 1 of tasks.jsonl (topology B) — with the given --set fields; every
// untouched header field keeps its verbatim bytes and any other lines
// round-trip byte-identical (asserted before the atomic write).
func (b *Board) SetHeader(u HeaderUpdate) ([]string, error) {
	headerPath := b.headerPathFor()
	lines, err := ReadJSONLLines(headerPath)
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
			return nil, fmt.Errorf("%s line %d: %w", filepath.Base(headerPath), i+1, err)
		}
		header = row
		headerIdx = i
		break
	}
	if header == nil {
		return nil, fmt.Errorf("%s is empty", filepath.Base(headerPath))
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
		// BT-007: header counters are non-negative by definition — a
		// negative value is rejected at write time.
		if *u.TicksTotal < 0 {
			return nil, fmt.Errorf("--set-ticks-total: %d is negative — header counters must be >= 0", *u.TicksTotal)
		}
		if err := set("ticks_total", *u.TicksTotal); err != nil {
			return nil, err
		}
	}
	if u.TicksIdle != nil {
		if *u.TicksIdle < 0 {
			return nil, fmt.Errorf("--set-ticks-idle: %d is negative — header counters must be >= 0", *u.TicksIdle)
		}
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
			return nil, fmt.Errorf("internal error: untouched line %d of %s would change — header update aborted (nothing written)", i+1, filepath.Base(headerPath))
		}
	}
	if err := atomicRewrite(headerPath, JoinLines(newLines)); err != nil {
		return nil, err
	}
	return changed, nil
}
