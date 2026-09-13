package board

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Import primitives (BT-022): the `boardctl import` command round-trips a
// board-report/v1 export back into a board. The command owns all planning
// (schema/board-identity validation, diffing, id renumbering); this file
// provides only the narrow append vocabulary it needs — appends that
// preserve the target file's serialization style, timestamp dialect, and
// MAX(id)+1 event sequencing, while never touching the board header
// (spec Q1 decision: headers are foreman-owned counters; import touches
// task, event, and fixture rows only).
//
// All three primitives are append-only (O_APPEND, never a rewrite) and
// validate their input BEFORE anything is written, so a rejected row never
// leaves a partial write behind.

// ImportEvent carries one exported event row destined for events.jsonl.
// The exported id and timestamp are deliberately not carried: import
// assigns fresh MAX(id)+1 ids and stamps the target's own timestamp
// dialect (spec 7.3.4 — never trust exported event ids).
type ImportEvent struct {
	Type   string          // event_type ("" -> "audit", same default as AppendEvent)
	TaskID string          // task_id ("" when the event is not task-scoped)
	Actor  string          // actor ("" -> "foreman", same default as AppendEvent)
	Detail json.RawMessage // verbatim JSON value bytes for detail (nil -> null)
}

// ImportEventWriter appends imported event rows with fresh MAX(id)+1 ids,
// the last event row's schema (unknown keys null), serialization style, and
// timestamp dialect. Unlike AppendEvent it never carries a tick_number and
// never bumps the header ticks_total: imported events are historic replays,
// not completed ticks (BT-022 spec Q1 decision).
type ImportEventWriter struct {
	b      *Board
	style  Style
	tsf    TSFormat
	nextID int64
	tpl    *Row // null-filled template of the last event row's key set
}

// NewImportEventWriter snapshots events.jsonl: the next free id, the last
// row's key set (template), serialization style, and timestamp dialect.
func (b *Board) NewImportEventWriter() (*ImportEventWriter, error) {
	rows, _, err := ReadAllRows(b.eventsPath)
	if err != nil {
		return nil, err
	}
	w := &ImportEventWriter{
		b:      b,
		style:  DefaultStyle(),
		tsf:    TSFormat{Layout: DefaultTSLayout},
		nextID: 1, // MAX(id)+1 over an empty file starts at 1 (AppendEvent rule)
	}
	var last *Row
	for _, r := range rows {
		if id, ok := r.Int("id"); ok && id >= w.nextID {
			w.nextID = id + 1
		}
		last = r
	}
	w.tpl = templateRow(last, "id", "timestamp", "event_type", "task_id", "actor", "detail", "tick_number")
	if last != nil {
		w.style = DetectStyle(rawLastLine(b.eventsPath))
		w.tsf = DetectTSLayout(last.String("timestamp"))
	}
	return w, nil
}

// Next reports the id the next Append will write.
func (w *ImportEventWriter) Next() int64 { return w.nextID }

// Append appends one imported event row with the next fresh id. The event
// type is vocabulary-checked (same set as AppendEvent) BEFORE any bytes are
// written. The board header is never touched.
func (w *ImportEventWriter) Append(ev ImportEvent) error {
	etype := ev.Type
	if etype == "" {
		etype = "audit"
	}
	if !EventTypeVocabulary[etype] {
		return fmt.Errorf("event type %q not in vocabulary {%s} — use one of the enumerated event_type values", etype, sortedEventVocab())
	}
	row := templateRow(w.tpl, "id") // null-filled copy; w.tpl is never mutated
	set := func(key string, v any) error { return row.SetGoValue(key, v, w.style) }
	if err := set("id", w.nextID); err != nil {
		return err
	}
	if err := set("timestamp", w.tsf.Now()); err != nil {
		return err
	}
	if err := set("event_type", etype); err != nil {
		return err
	}
	if err := set("task_id", ev.TaskID); err != nil {
		return err
	}
	actor := ev.Actor
	if actor == "" {
		actor = "foreman"
	}
	if err := set("actor", actor); err != nil {
		return err
	}
	if ev.Detail == nil {
		if err := set("detail", nil); err != nil {
			return err
		}
	} else {
		row.SetRaw("detail", bytes.TrimSpace(ev.Detail))
	}
	if err := appendBytes(w.b.eventsPath, append(row.Marshal(w.style), '\n')); err != nil {
		return err
	}
	w.nextID++
	return nil
}

// AppendImportFixture appends one imported fixture row — the exported id
// plus a null-filled mirror of the last fixtures.jsonl row's key set — to
// an EXISTING fixtures.jsonl. Import never creates fixtures.jsonl (spec
// 7.3.8): a missing file is an error; the caller screens that in the plan.
func (b *Board) AppendImportFixture(id string) error {
	if !fileExists(b.fixturesPath) {
		return fmt.Errorf("no fixtures.jsonl on target — import never creates it")
	}
	rows, raws, err := ReadAllRows(b.fixturesPath)
	if err != nil {
		return err
	}
	var last *Row
	var lastRaw []byte
	if n := len(rows); n > 0 {
		last, lastRaw = rows[n-1], raws[n-1]
	}
	style := DefaultStyle()
	if lastRaw != nil {
		style = DetectStyle(lastRaw)
	}
	row := templateRow(last, "id")
	if err := row.SetGoValue("id", id, style); err != nil {
		return err
	}
	return appendBytes(b.fixturesPath, append(row.Marshal(style), '\n'))
}

// templateRow returns a null-filled copy of last's key set; when last is
// nil the given default keys are used. The returned row is always a fresh
// copy — the source template is never mutated.
func templateRow(last *Row, defaultKeys ...string) *Row {
	r := &Row{Vals: map[string]json.RawMessage{}}
	if last != nil {
		r.Keys = append(r.Keys, last.Keys...)
	} else {
		r.Keys = defaultKeys
	}
	for _, k := range r.Keys {
		r.Vals[k] = json.RawMessage("null")
	}
	return r
}
