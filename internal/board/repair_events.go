package board

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// DF-BOARDCTL-38: repair descending event ids in events.jsonl.
//
// The validateEvents function (validate.go:534) reports errors when
// event ids descend below earlier ids. This function renumbers those
// descending ids to restore ascending order, starting from max(existing)+1
// for the first descending event.
//
// The repair is mechanical: preserve every field except "id".
// Legacy events without numeric ids (missing or string-valued) are skipped.
// Duplicate ids are warned but not repaired (benign legacy duplicates).

// RepairEventsReport holds the result of repairing event ids.
type RepairEventsReport struct {
	EventsPath  string `json:"events_path"`
	TotalEvents int    `json:"total_events"`
	FixedCount  int    `json:"fixed_count"` // events whose id was renumbered
	Descending  int    `json:"descending"`  // count of descending id occurrences found
	NonNumeric  int    `json:"non_numeric"` // legacy events without numeric ids
	Duplicates  int    `json:"duplicates"`  // duplicate ids found (warned, not fixed)
	DryRun      bool   `json:"dry_run"`
	AppliedPath string `json:"applied_path,omitempty"`
	Applied     bool   `json:"applied"`
	Clean       bool   `json:"clean"` // no descending ids found
}

// RenderText renders the human report for event id repair.
func (r *RepairEventsReport) RenderText() string {
	out := fmt.Sprintf("events.jsonl: %s\n", r.EventsPath)
	out += fmt.Sprintf("total events: %d\n", r.TotalEvents)
	out += fmt.Sprintf("descending ids: %d\n", r.Descending)
	out += fmt.Sprintf("non-numeric/missing ids: %d\n", r.NonNumeric)
	out += fmt.Sprintf("duplicate ids: %d\n", r.Duplicates)

	if r.Clean {
		out += "no descending ids found — already canonical\n"
	} else {
		out += fmt.Sprintf("fixed: %d event(s) renumbered to ascending order\n", r.FixedCount)
	}
	if r.Applied {
		out += fmt.Sprintf("applied: %s rewritten\n", r.AppliedPath)
	} else if r.DryRun {
		out += "DRY-RUN — no file written (pass --apply to rewrite events.jsonl)\n"
	}
	return out
}

// RepairEvents repairs descending event ids in events.jsonl.
// When dryRun=true, it only reports what would be fixed.
// When dryRun=false, it rewrites events.jsonl atomically.
func (b *Board) RepairEvents(dryRun bool) (*RepairEventsReport, error) {
	lines, err := ReadJSONLLines(b.eventsPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", b.eventsPath, err)
	}

	rep := &RepairEventsReport{
		EventsPath: b.eventsPath,
		DryRun:     dryRun,
	}

	// First pass: classify events and find descending ids.
	type eventRecord struct {
		lineIdx    int
		line       []byte
		id         int64
		hasID      bool
		isInt      bool
		legacy     bool // missing or non-numeric id
		NonNumeric int
		Duplicates int
	}

	var records []eventRecord
	seenIDs := map[int64]int{} // id -> first line index
	var maxID int64
	var haveMax bool

	ierr := IterParsed(lines, func(row *Row, idx int, raw []byte) error {
		// Keep a copy of the raw line for later.
		rec := eventRecord{lineIdx: idx, line: raw}

		hasID := row.Has("id")
		rec.hasID = hasID
		if !hasID {
			rec.legacy = true
			rep.NonNumeric++
			records = append(records, rec)
			return nil
		}

		id, isInt := row.Int("id")
		rec.isInt = isInt
		if !isInt {
			rec.legacy = true
			rep.NonNumeric++
			records = append(records, rec)
			return nil
		}
		rec.id = id

		// Check for duplicates first (benign legacy).
		if prev, dup := seenIDs[id]; dup {
			rec.Duplicates++
			rep.Duplicates++
			_ = prev // first occurrence line
		}
		seenIDs[id] = idx + 1

		// Track max id seen so far.
		if !haveMax || id > maxID {
			maxID = id
			haveMax = true
		}

		records = append(records, rec)
		return nil
	})
	if ierr != nil {
		return nil, fmt.Errorf("parse events: %w", ierr)
	}

	rep.TotalEvents = len(records)

	// Second pass: identify descending ids and renumber.
	// We need to track the "running max" as we go left-to-right.
	// Any id that descends below the running max needs renumbering.
	// The new id = running_max + 1.
	runningMax := int64(0)
	hasRunningMax := false
	fixedCount := 0
	descendingCount := 0

	// We'll collect lines that need changes.
	changeMap := map[int][]byte{} // lineIdx -> new line bytes

	// Re-iterate to find descending ids.
	runningMax = 0
	hasRunningMax = false
	for _, rec := range records {
		if rec.legacy || !rec.isInt {
			// Non-numeric events don't affect the running max.
			continue
		}

		id := rec.id
		lineIdx := rec.lineIdx

		if hasRunningMax && id < runningMax {
			// Descending! Fix it.
			runningMax++
			newID := runningMax
			descendingCount++
			fixedCount++

			// Re-encode the line with the new id.
			// We'll parse and modify the row.
			newLine, err := renumberEventID(rec.line, newID)
			if err != nil {
				return nil, fmt.Errorf("line %d: renumber %d -> %d: %w", lineIdx+1, id, newID, err)
			}
			changeMap[lineIdx] = newLine

			// The running max stays at the new id.
			runningMax = newID
		} else if id > runningMax {
			runningMax = id
		}
		hasRunningMax = true
	}

	rep.Descending = descendingCount
	rep.FixedCount = fixedCount
	rep.Clean = descendingCount == 0

	if rep.Clean {
		return rep, nil
	}

	if dryRun {
		// Write the would-be bytes to a review cache file.
		cachePath := b.eventsPath + ".renumbered.jsonl"
		out, err := renumberEventsFile(lines, changeMap)
		if err != nil {
			return nil, fmt.Errorf("generate dry-run output: %w", err)
		}
		if err := os.WriteFile(cachePath, out, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", cachePath, err)
		}
		rep.AppliedPath = cachePath
		return rep, nil
	}

	// Apply: rewrite events.jsonl with changes.
	out, err := renumberEventsFile(lines, changeMap)
	if err != nil {
		return nil, fmt.Errorf("generate rewritten events.jsonl: %w", err)
	}

	// Guarded atomic rewrite.
	lockErr := b.withWriteLock(b.eventsPath, func() error {
		if err := os.WriteFile(b.eventsPath, out, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", b.eventsPath, err)
		}
		return nil
	})
	if lockErr != nil {
		return nil, lockErr
	}

	rep.Applied = true
	rep.AppliedPath = b.eventsPath
	return rep, nil
}

// renumberEventID takes one events.jsonl line (JSON) and returns a new line
// with ONLY the numeric value token of the top-level "id" field replaced.
//
// DF-BOARDCTL-44: this used to unmarshal the line into a map[string]any and
// re-marshal it, which (a) rounded integers above 2^53 through float64, (b)
// dropped the writer's key order (map iteration is sorted on marshal), and
// (c) collapsed runs of spaces inside string values while it "restored"
// spacing. The promise — every field except id is preserved — is now literal:
// the id value's byte range is replaced in place, so key order, all separator
// spacing, every other value's raw bytes (double spaces included), JSON string
// escaping, and the trailing newline/surrounding bytes survive verbatim.
//
// The line must be a JSON object whose top-level "id" holds a JSON number.
// A missing or non-numeric id is refused here; RepairEvents never routes such
// events to this helper (legacy events are skipped and reported as
// non-numeric), so the observable CLI behavior for them is unchanged.
func renumberEventID(raw []byte, newID int64) ([]byte, error) {
	span, err := topLevelFieldSpan(raw, "id")
	if err != nil {
		return nil, err
	}
	value := raw[span.start:span.end]
	if _, ok := decodeJSONNumber(value); !ok {
		return nil, fmt.Errorf("id value is not a JSON number: %s", diagnosticSnippet(value))
	}

	out := make([]byte, 0, len(raw)+20)
	out = append(out, raw[:span.start]...)
	out = strconv.AppendInt(out, newID, 10)
	out = append(out, raw[span.end:]...)
	return out, nil
}

// fieldSpan is the byte range [start,end) of one object field's VALUE within a
// raw JSON line.
type fieldSpan struct {
	start int
	end   int
}

// topLevelFieldSpan locates the value bound to key in the TOP-LEVEL object of
// line, returning its byte range. It is a scanner, not a decoder: nothing is
// re-serialized, so the caller can splice a new value in without disturbing a
// single byte elsewhere. Keys nested inside child objects/arrays are never
// considered. A missing key is an error.
func topLevelFieldSpan(line []byte, key string) (fieldSpan, error) {
	i := skipJSONSpace(line, 0)
	if i >= len(line) || line[i] != '{' {
		return fieldSpan{}, fmt.Errorf("not a JSON object")
	}
	i++
	for {
		i = skipJSONSpace(line, i)
		if i >= len(line) {
			return fieldSpan{}, fmt.Errorf("unterminated JSON object")
		}
		if line[i] == '}' {
			return fieldSpan{}, fmt.Errorf("no top-level %q field", key)
		}

		keyStart := i
		keyEnd, err := scanJSONString(line, i)
		if err != nil {
			return fieldSpan{}, err
		}
		var name string
		if err := json.Unmarshal(line[keyStart:keyEnd], &name); err != nil {
			return fieldSpan{}, fmt.Errorf("unreadable object key: %w", err)
		}

		i = skipJSONSpace(line, keyEnd)
		if i >= len(line) || line[i] != ':' {
			return fieldSpan{}, fmt.Errorf("expected ':' after object key %q at offset %d", name, i)
		}
		valueStart := skipJSONSpace(line, i+1)
		valueEnd, err := scanJSONValue(line, valueStart)
		if err != nil {
			return fieldSpan{}, err
		}
		if name == key {
			return fieldSpan{start: valueStart, end: valueEnd}, nil
		}

		i = skipJSONSpace(line, valueEnd)
		if i >= len(line) {
			return fieldSpan{}, fmt.Errorf("unterminated JSON object")
		}
		switch line[i] {
		case ',':
			i++
		case '}':
			return fieldSpan{}, fmt.Errorf("no top-level %q field", key)
		default:
			return fieldSpan{}, fmt.Errorf("unexpected byte %q after object key %q at offset %d", line[i], name, i)
		}
	}
}

// skipJSONSpace advances past JSON insignificant whitespace.
func skipJSONSpace(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '	', '\r', '\n':
			i++
		default:
			return i
		}
	}
	return i
}

// scanJSONString returns the offset just past the closing quote of the JSON
// string literal starting at i. Backslash escapes are honored: an escaped
// quote never terminates the literal, and the payload of a \uXXXX escape can
// never be mistaken for a terminator.
func scanJSONString(b []byte, i int) (int, error) {
	if i >= len(b) || b[i] != '"' {
		return 0, fmt.Errorf("expected a JSON string at offset %d", i)
	}
	for j := i + 1; j < len(b); j++ {
		switch b[j] {
		case '\\':
			j++ // skip the escaped byte; \\ \" \n \uXXXX all keep the quote-shape opaque
		case '"':
			return j + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated JSON string starting at offset %d", i)
}

// scanJSONValue returns the offset just past the JSON value starting at i.
// Strings are scanned with escape awareness; objects and arrays are walked to
// their matching close; every other token is a scalar that ends at the next
// structural delimiter or whitespace.
func scanJSONValue(b []byte, i int) (int, error) {
	if i >= len(b) {
		return 0, fmt.Errorf("missing JSON value at end of line")
	}
	switch b[i] {
	case '"':
		return scanJSONString(b, i)
	case '{', '[':
		return scanJSONComposite(b, i)
	}
	for j := i; j < len(b); j++ {
		switch b[j] {
		case ',', '}', ']', ' ', '	', '\r', '\n':
			if j == i {
				return 0, fmt.Errorf("empty JSON value at offset %d", i)
			}
			return j, nil
		}
	}
	return len(b), nil
}

// scanJSONComposite walks an object/array starting at i to the offset just
// past its matching close, skipping over string contents (so braces or
// brackets inside strings never affect nesting) and nested values.
func scanJSONComposite(b []byte, i int) (int, error) {
	depth := 0
	for j := i; j < len(b); j++ {
		switch b[j] {
		case '"':
			end, err := scanJSONString(b, j)
			if err != nil {
				return 0, err
			}
			j = end - 1
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return j + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated JSON value starting at offset %d", i)
}

// decodeJSONNumber reports whether b is exactly one JSON number and returns
// it. Numbers are kept as text (json.Number), so nothing is widened to
// float64 and integers above 2^53 stay exact.
func decodeJSONNumber(b []byte) (json.Number, bool) {
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return "", false
	}
	// json.Unmarshal treats a JSON null as a no-op on any destination, so an
	// untouched empty Number is the null/absent case, not a number.
	if n == "" {
		return "", false
	}
	return n, true
}

// renumberEventsFile takes all lines and a map of lineIdx -> newBytes,
// and returns the complete rewritten file.
func renumberEventsFile(lines [][]byte, changeMap map[int][]byte) ([]byte, error) {
	out := make([][]byte, len(lines))
	for i, line := range lines {
		if replacement, ok := changeMap[i]; ok {
			out[i] = replacement
		} else {
			out[i] = line
		}
	}
	return joinLines(out), nil
}

// joinLines joins JSONL lines with newlines.
func joinLines(lines [][]byte) []byte {
	var buf []byte
	for i, line := range lines {
		if i > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, line...)
	}
	return buf
}
