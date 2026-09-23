// Package render — report assembly: resolve + load + derive + payload.
package render

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// Options carries render-time settings.
type Options struct {
	Now  time.Time // render time (defaults: time.Now()); location = report tz
	Zone *time.Location
}

// Build resolves target and produces the board-report/v1 payload. Render is
// read-only toward board files.
func Build(target string, opts Options) (*ReportPayload, error) {
	b, err := board.Resolve(target)
	if err != nil {
		return nil, err
	}
	return buildFromBoard(b, opts)
}

// buildFromBoard builds the payload for an already-resolved board.
//
// BuildFromBoard is the exported wrapper (DF-BOARDCTL-9): `render
// --skip-bad-lines` resolves the board itself so it can arm the tolerant
// flag set before loading, then hands the pre-armed board here. The render
// loader is tolerant by design, so this path ignores Board.SkipBad; arming
// it is what lets the CLI attach the SKIPPED-LINES evidence to the same
// read it renders from.
//
// godoc: rendered output — reports render fully even on degraded boards.
func BuildFromBoard(b *board.Board, opts Options) (*ReportPayload, error) {
	return buildFromBoard(b, opts)
}

func buildFromBoard(b *board.Board, opts Options) (*ReportPayload, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	loc := opts.Zone
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)

	d, err := loadBoard(b, now)
	if err != nil {
		return nil, err
	}
	return &ReportPayload{
		Schema:         SchemaName,
		RenderedAt:     now.Format(time.RFC3339),
		ReportTimezone: loc.String(),
		Boards:         []BoardPayload{newBoardPayload(d)},
		GeneratedBy:    "boardctl render (board-report/v1)",
	}, nil
}

// BuildBoards produces the multi-board board-report/v1 payload for
// already-resolved boards, preserving order (BT-021: serve's upload path
// assembles N boards; the 5.6 compare view lights up automatically at >=2).
// Per-board load problems that the tolerant reader can represent surface as
// the 5.7 error state on that board's payload; only infrastructure failures
// (unreadable required file) return an error.
func BuildBoards(boards []*board.Board, opts Options) (*ReportPayload, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	loc := opts.Zone
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)

	rp := &ReportPayload{
		Schema:         SchemaName,
		RenderedAt:     now.Format(time.RFC3339),
		ReportTimezone: loc.String(),
		Boards:         []BoardPayload{},
		GeneratedBy:    "boardctl serve (board-report/v1)",
	}
	for _, b := range boards {
		if b == nil {
			continue
		}
		d, err := loadBoard(b, now)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Dir, err)
		}
		// DF-BOARDCTL-5: the data-quality footnote ids stay on each board's
		// own payload (newBoardPayload) — never unioned across boards, so a
		// board's report lists only its own uncomputable rows.
		rp.Boards = append(rp.Boards, newBoardPayload(d))
	}
	return rp, nil
}

// newBoardPayload assembles one board's payload entry (shared by the
// single-board Build path and the multi-board BuildBoards path).
func newBoardPayload(d *boardData) BoardPayload {
	der := derive(d)
	bp := BoardPayload{
		Name:          d.name,
		Slug:          d.slug,
		Topology:      d.topology,
		Error:         d.fatal,
		Header:        nil,
		Tasks:         []json.RawMessage{},
		Events:        []json.RawMessage{},
		FixtureIDs:    sortedKeys(d.fixtureI),
		ParseWarnings: d.warns.payload(),
		Derived:       der,
	}
	if d.header != nil {
		bp.Header = BoardHeader(compactRow(d.header))
	}
	for _, t := range d.tasks {
		bp.Tasks = append(bp.Tasks, compactRow(t.row))
	}
	for _, e := range d.events {
		bp.Events = append(bp.Events, compactRow(e.row))
	}
	// DF-BOARDCTL-5: this board's own data-quality footnote ids only.
	bp.NoDoneCompleted = noDoneCompletedIDs(d)
	return bp
}

// noDoneCompletedIDs returns the sorted ids of non-fixture tasks with status
// complete and no computable done day (the 3.0 data-quality footnote list).
func noDoneCompletedIDs(d *boardData) []string {
	var ids []string
	for _, t := range nonFixtureRows(d) {
		if t.status == "complete" && t.done == nil {
			ids = append(ids, t.id)
		}
	}
	sort.Strings(ids)
	return ids
}

// compactRow renders one row as compact JSON preserving key order and
// verbatim field bytes.
func compactRow(row *board.Row) json.RawMessage {
	return json.RawMessage(board.RowJSONCompact(row))
}

// EscapeJSONIsland serializes v and applies the exact 6.3 escaping (Go's
// json.HTMLEscape behavior): <, >, & escaped, U+2028/U+2029 escaped, so no
// `</script>` sequence can terminate the island and the parsed payload is
// byte-identical after JSON.parse.
func EscapeJSONIsland(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true) // Go default: <, >, & -> \u003c, \u003e, \u0026
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n")), nil
}

// RenderHTML produces the complete self-contained HTML report string.
func RenderHTML(payload *ReportPayload) (string, error) {
	island, err := EscapeJSONIsland(payload)
	if err != nil {
		return "", fmt.Errorf("serialize payload: %w", err)
	}
	return BuildHTML(string(island)), nil
}

// sortedKeys returns the sorted keys of a string set.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
