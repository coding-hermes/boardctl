package board

import (
	"bytes"
	"fmt"
	"sort"
)

// REVIEW-BOARDCTL-001: re-runnable per-board STATUS sweep.
//
// The write vocabulary (StatusVocabulary) is enforced on every write surface,
// yet boards still drift — imported rows, hand edits, and pre-boardctl boards
// carry statuses outside the canon. validate reports them and BT-025's
// `update <id> --normalize` repairs them ONE row at a time; this sweep is the
// board-wide driver for the same two outcomes:
//
//   - a status that is a KNOWN read alias (StatusAliases: todo, done, open,
//     completed, in-progress, ...) is "fixable by --normalize" — with --apply
//     the sweep canonicalizes it by calling NormalizeTask per row, so the
//     rewrite reuses the BT-025 machinery (only the status spelling changes,
//     every other byte of the row round-trips, refusal on any unresolvable
//     sibling value) instead of a hand-rolled JSON rewrite here;
//   - a status that is NEITHER canonical NOR a known alias (duplicate, parked,
//     resolved, retired, ...) is NEVER touched: coercing it is a human
//     decision (which canonical status? is a superseded_by marker needed?) and
//     refusing to guess is the house rule. The row is reported with the
//     explicit action needed.
//
// DRY-RUN BY DEFAULT: apply=false computes and reports only. The report
// carries the off-vocabulary census before AND after an apply, so the printed
// line doubles as the evidence of what the run changed.

// StatusSweepRow is one offending row: current on-disk spelling and the action
// needed to clear it.
type StatusSweepRow struct {
	ID        string `json:"id"`
	Line      int    `json:"line"` // 1-based file line
	Status    string `json:"status"`
	Canonical string `json:"canonical,omitempty"` // set for alias rows (the normalize target)
	Action    string `json:"action"`
	Fixed     bool   `json:"fixed,omitempty"` // set on the after-apply report when --apply rewrote the row
}

// StatusSweepReport is the result of one sweep pass. Rows are in file order
// (first occurrence wins when an id repeats).
type StatusSweepReport struct {
	BoardPath  string           `json:"board_path"`
	Apply      bool             `json:"apply"`
	Rows       int              `json:"rows"`       // task rows examined
	OffBefore  int              `json:"off_before"` // off-vocabulary rows before any write
	Normalized []StatusSweepRow `json:"normalized"` // alias rows (fixable)
	Explicit   []StatusSweepRow `json:"explicit"`   // unknown rows (human decision)
	OffAfter   int              `json:"off_after"`  // off-vocabulary rows after the apply (== OffBefore when dry-run)
}

// Summarize renders the one-line census: X/Y rows off-vocabulary, split into
// normalize-fixable and explicit-decision rows.
func (r *StatusSweepReport) Summarize() string {
	return fmt.Sprintf("%d/%d rows off-vocabulary: %d fixable by --normalize, %d need explicit decision",
		r.OffBefore, r.Rows, len(r.Normalized), len(r.Explicit))
}

// StatusSweep scans the board's task rows and classifies every status that is
// not already canonical. With apply=true it then normalizes the ALIAS rows
// (one NormalizeTask call each — the sanctioned BT-025 rewrite) and recounts.
// Unknown rows are never written in either mode.
func (b *Board) StatusSweep(apply bool) (*StatusSweepReport, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	rep := &StatusSweepReport{BoardPath: b.tasksPath, Apply: apply}

	// Classify only; nothing is mutated here.
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: the header is not a task row
		}
		id := row.String("id")
		if id == "" {
			return nil // nameless rows are validate's business, not the status sweep's
		}
		rep.Rows++
		st := row.String("status")
		if st == "" || StatusVocabulary[st] {
			return nil // canonical (or absent — validate owns that warning)
		}
		canonical, ok, _ := ResolveStatus(st)
		entry := StatusSweepRow{ID: id, Line: idx + 1, Status: st}
		if !ok {
			entry.Action = "needs explicit --status <canonical> + superseded_by marker decision"
			rep.Explicit = append(rep.Explicit, entry)
			return nil
		}
		entry.Canonical = canonical
		entry.Action = fmt.Sprintf("--normalize (status %q -> %q)", st, canonical)
		rep.Normalized = append(rep.Normalized, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.tasksPath, err)
	}
	rep.OffBefore = len(rep.Normalized) + len(rep.Explicit)
	rep.OffAfter = rep.OffBefore

	if !apply {
		return rep, nil
	}

	// Apply: canonicalize ONLY the alias rows, through the existing normalize
	// machinery. A row that fails to normalize (an unresolvable guard/ci
	// value next to an alias status) aborts the whole run with nothing past
	// that row written — the same refuse-rather-than-guess contract
	// NormalizeTask already enforces per row.
	var fixed []StatusSweepRow
	for _, entry := range rep.Normalized {
		changes, err := b.NormalizeTask(entry.ID, false)
		if err != nil {
			return rep, fmt.Errorf("sweep aborted mid-apply (rows before %q are written): %w", entry.ID, err)
		}
		if len(changes) == 0 {
			continue // already canonical by the time the write ran — keep OffAfter honest
		}
		e := entry
		e.Fixed = true
		fixed = append(fixed, e)
	}
	if len(fixed) > 0 {
		rep.Normalized = fixed
	}

	// Recount from the file (never by arithmetic — the write path is the truth).
	rep.OffAfter = b.offVocabularyCount()
	return rep, nil
}

// offVocabularyCount re-reads tasks.jsonl and counts rows whose status is
// neither canonical nor a known read alias. The AFTER number of an apply is
// measured, not computed, so a normalize that silently skipped a row shows up
// as a worse OffAfter instead of being papered over.
func (b *Board) offVocabularyCount() int {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return -1
	}
	n := 0
	if err := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil
		}
		if row.String("id") == "" {
			return nil
		}
		st := row.String("status")
		if st == "" {
			return nil
		}
		if _, ok, _ := ResolveStatus(st); !ok {
			n++
		}
		return nil
	}); err != nil {
		return -1
	}
	return n
}

// RenderText renders the human report: one line per offending row, then the
// census. When apply ran, a second census line shows the before/after counts.
func (r *StatusSweepReport) RenderText() string {
	var sb bytes.Buffer
	rows := append(append([]StatusSweepRow{}, repSorted(r.Normalized)...), repSorted(r.Explicit)...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Line < rows[j].Line })
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf("  line %d  %s: status %q — %s\n", row.Line, row.ID, row.Status, row.Action))
	}
	sb.WriteString(r.Summarize() + "\n")
	if r.Apply {
		sb.WriteString(fmt.Sprintf("before: %d off-vocabulary rows; after: %d off-vocabulary rows\n", r.OffBefore, r.OffAfter))
	}
	return sb.String()
}

// repSorted copies a report row slice so the sort in RenderText never aliases
// the report's own slices.
func repSorted(rows []StatusSweepRow) []StatusSweepRow {
	return append([]StatusSweepRow{}, rows...)
}
