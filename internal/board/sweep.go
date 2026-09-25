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
//     every other byte of the row round-trips) instead of a hand-rolled JSON
//     rewrite here;
//   - a status that is NEITHER canonical NOR a known alias (duplicate, parked,
//     resolved, retired, ...) is NEVER touched: coercing it is a human
//     decision (which canonical status? is a superseded_by marker needed?) and
//     refusing to guess is the house rule. The row is reported with the
//     explicit action needed.
//
// BT-066: the sweep classifies ALL the vocab columns up front, not just
// status. An off-vocabulary guard_result/ci_result value (a legacy "pending"
// in ci_result, a guard value in the ci column, free-form prose) makes the
// row EXPLICIT — a human must decide the correct value — and an alias status
// sitting next to such a value is NEVER advertised as fixable, because
// NormalizeTask would refuse the row (refuse-rather-than-guess is its
// contract) and the sweep would die mid-file with earlier rows already
// written. Both modes report these rows naming the column and the offending
// value, and --apply SKIPS-and-reports instead of aborting: a per-row
// NormalizeTask refusal leaves the row byte-identical and the sweep
// continues; the measured after-count keeps refused rows visible so nothing
// looks repaired that is not.
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
	// BT-066: Blocked names the column that made the row un-decidable by the
	// sweep ("ci_result", "guard_result", or a human-decision status) and
	// BlockedValue the offending on-disk value; set on Explicit rows.
	Blocked      string `json:"blocked,omitempty"`
	BlockedValue string `json:"blocked_value,omitempty"`
	// BT-066: Skipped is set on the after-apply report when the row was
	// advertised as fixable but NormalizeTask refused it (nothing was
	// written); the census stays honest about what did not land.
	Skipped bool `json:"skipped,omitempty"`
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

	// Classify only; nothing is mutated here. BT-066: every vocab column is
	// classified up front — an off-vocabulary guard_result/ci_result makes
	// the row explicit (never fixable), because NormalizeTask would refuse
	// it and an apply would otherwise die mid-file on partial writes.
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: the header is not a task row
		}
		id := row.String("id")
		if id == "" {
			return nil // nameless rows are validate's business, not the status sweep's
		}
		rep.Rows++
		// BT-066: off-vocab result columns first — they block the row
		// regardless of what the status column holds. Empty/absent values
		// are never results (never run); only present values classify.
		for _, c := range []struct {
			key   string
			vocab map[string]bool
		}{
			{"guard_result", GuardResultVocabulary},
			{"ci_result", CIResultVocabulary},
		} {
			raw := row.String(c.key)
			if raw == "" {
				continue
			}
			if c.vocab[NormalizeResultValue(raw)] {
				continue // canonical (or case-tolerant spelling of one)
			}
			rep.Explicit = append(rep.Explicit, StatusSweepRow{
				ID:           id,
				Line:         idx + 1,
				Status:       row.String("status"),
				Action:       fmt.Sprintf("needs explicit decision: %s %q is not in vocabulary {%s} — decide the correct value and set it explicitly", c.key, raw, vocabKeys(c.vocab)),
				Blocked:      c.key,
				BlockedValue: raw,
			})
			return nil // the row is explicit; a status fix would be refused anyway
		}
		st := row.String("status")
		if st == "" || StatusVocabulary[st] {
			return nil // canonical (or absent — validate owns that warning)
		}
		canonical, ok, _ := ResolveStatus(st)
		entry := StatusSweepRow{ID: id, Line: idx + 1, Status: st}
		if !ok {
			entry.Action = "needs explicit --status <canonical> + superseded_by marker decision"
			entry.Blocked = "status"
			entry.BlockedValue = st
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
	// machinery. BT-066: a row that fails to normalize is SKIPPED AND
	// REPORTED, never a fatal error — the abort-on-first-refusal behavior
	// left earlier rows written and the rest unrepaired while the run looked
	// failed mid-file. The refused row stays byte-identical on disk, the
	// report marks it skipped, and the measured recount below keeps it
	// counted as still off-vocabulary.
	var fixed, skipped []StatusSweepRow
	for _, entry := range rep.Normalized {
		changes, err := b.NormalizeTask(entry.ID, false)
		if err != nil {
			e := entry
			e.Skipped = true
			e.Action = fmt.Sprintf("normalize refused: %v — row left unchanged", err)
			skipped = append(skipped, e)
			continue
		}
		if len(changes) == 0 {
			continue // already canonical by the time the write ran — keep OffAfter honest
		}
		e := entry
		e.Fixed = true
		fixed = append(fixed, e)
	}
	if len(fixed) > 0 || len(skipped) > 0 {
		kept := append(append([]StatusSweepRow{}, fixed...), skipped...)
		sort.Slice(kept, func(i, j int) bool { return kept[i].Line < kept[j].Line })
		rep.Normalized = kept
	}

	// Recount from the file (never by arithmetic — the write path is the truth).
	rep.OffAfter = b.offVocabularyCount()
	return rep, nil
}

// offVocabularyCount re-reads tasks.jsonl and counts rows that still carry
// off-vocabulary values: a status neither canonical nor a known read alias,
// or (BT-066) a present guard_result/ci_result outside its own vocabulary —
// the same classification the sweep reports. The AFTER number of an apply is
// measured, not computed, so a normalize that silently skipped a row — or a
// blocked row the sweep never claimed to fix — shows up in the count instead
// of being papered over.
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
		// BT-066: a blocked result column makes the row count, status aside.
		for _, c := range []struct {
			key   string
			vocab map[string]bool
		}{
			{"guard_result", GuardResultVocabulary},
			{"ci_result", CIResultVocabulary},
		} {
			if raw := row.String(c.key); raw != "" && !c.vocab[NormalizeResultValue(raw)] {
				n++
				return nil
			}
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
		if row.Skipped {
			// BT-066: the apply refused this row — make the non-repair
			// visible right where the row is listed, not only in the census.
			sb.WriteString("    SKIPPED — normalize refused; the row was left byte-identical\n")
		}
	}
	sb.WriteString(r.Summarize() + "\n")
	if r.Apply {
		sb.WriteString(fmt.Sprintf("before: %d off-vocabulary rows; after: %d off-vocabulary rows\n", r.OffBefore, r.OffAfter))
		if skipped := len(repSkipped(r.Normalized)); skipped > 0 {
			sb.WriteString(fmt.Sprintf("skipped: %d row(s) normalize refused — left byte-identical, decide those rows by hand\n", skipped))
		}
	}
	return sb.String()
}

// repSkipped returns the report rows marked skipped (BT-066).
func repSkipped(rows []StatusSweepRow) []StatusSweepRow {
	var out []StatusSweepRow
	for _, r := range rows {
		if r.Skipped {
			out = append(out, r)
		}
	}
	return out
}

// repSorted copies a report row slice so the sort in RenderText never aliases
// the report's own slices.
func repSorted(rows []StatusSweepRow) []StatusSweepRow {
	return append([]StatusSweepRow{}, rows...)
}
