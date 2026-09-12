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
	der := derive(d)

	bp := BoardPayload{
		Name:          d.name,
		Slug:          d.slug,
		Topology:      d.topology,
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

	tzName := loc.String()
	if tzName == "Local" {
		tzName = "Local"
	}
	rp := &ReportPayload{
		Schema:          SchemaName,
		RenderedAt:      now.Format(time.RFC3339),
		ReportTimezone:  tzName,
		Boards:          []BoardPayload{bp},
		GeneratedBy:     "boardctl render (board-report/v1)",
		NoDoneCompleted: noDoneCompletedIDs(d),
	}
	return rp, nil
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
