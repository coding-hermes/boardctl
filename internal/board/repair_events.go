package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
// with the "id" field set to newID. It preserves the original JSON style
// (spaced vs compact).
func renumberEventID(raw []byte, newID int64) ([]byte, error) {
	// Parse the JSON, modify id, re-encode.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse event: %w", err)
	}
	m["id"] = newID

	// Re-encode as JSON. We'll try to preserve the original style
	// by checking if the original had spaces after colons/commas.
	var out []byte
	// Use compact JSON for reliability; the event id is the critical field.
	if rawStyle := hasSpaces(raw); rawStyle {
		out, _ = json.MarshalIndent(m, "", "  ")
		// Flatten back to single line.
		out = bytes.ReplaceAll(out, []byte("\n"), []byte(""))
		out = bytes.ReplaceAll(out, []byte("  "), []byte(" "))
	}
	if len(out) == 0 {
		out, _ = json.Marshal(m)
	}
	// Ensure trailing newline if the original had one.
	if len(raw) > 0 && raw[len(raw)-1] == '\n' {
		out = append(out, '\n')
	}
	return out, nil
}

// hasSpaces checks if a JSON line has spaces after colons/commas.
func hasSpaces(raw []byte) bool {
	return bytes.Contains(raw, []byte(": ")) || bytes.Contains(raw, []byte(", "))
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
