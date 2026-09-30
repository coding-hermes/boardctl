package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// DF-BOARDCTL-10: id-aware dedupe for RECYCLED task ids.
//
// The SG-126 fingerprint dedupe (fingerprint.go / backfill.go) collapses rows
// that share a normalized title+reasoning. It deliberately refuses to touch a
// different failure class: many DISTINCT findings filed under ONE id. QA lanes
// re-used a fresh cycle number instead of advancing the id, so live boards
// carry e.g. six QA-ASCE-1 rows where every title+reasoning differs
// (header-frozen, spawn-pool exhausted, port-range exhausted x2,
// bunker-dropped). validate permanently errors (exit 1) on every such board
// ("duplicate task id ... also line N") and every id-keyed consumer
// (board-scan keep-last, update-by-id, foreman dispatch) reads the wrong row —
// while the fingerprint tool correctly keeps them apart, because their
// fingerprints all differ. Nothing repaired the HYGIENE. This file is the
// missing repair tool.
//
// Root cause and prevention (the rule this tool's help text documents): a lane
// must ALWAYS mint a fresh task id for a new finding. An id-recycled filing —
// a new finding under an old key — is never a re-observation of the same task.
//
// What DedupeByID does (fix direction (b) of the board row):
//
//   - group task rows by their id (file order; the board is append-only, so
//     the EARLIEST line per id is the row that id originally named);
//   - keep the earliest line per id, untouched byte-for-byte;
//   - close each later line as superseded-by-earliest: status=complete,
//     worker_summary "superseded-by-earliest: recycled task id — kept line N
//     (id-dupe backfill <date>)", superseded_by <the kept id>,
//     completed_at now (updated_at refreshed only when the row carries the
//     key). The row's own title/reasoning/detail bytes are NOT rewritten;
//   - append one `audit` event per closed line whose detail quotes that
//     line's original title and reasoning VERBATIM — the finding text stays
//     recoverable from the event log even though the row is closed;
//   - leave already-terminal later lines (complete/failed) exactly as they
//     are: they are closed, so they no longer masquerade as open work, and
//     rewriting them would destroy history for no repair value;
//   - REFUSE the whole run (nothing written) when a later duplicate carries a
//     status that is neither canonical nor a known read alias — an ambiguous
//     row needs a human decision, the same discipline as normalize.
//
// dry-run (apply=false) computes the identical plan and writes NOTHING.
// apply=true is idempotent: after one pass every later line is terminal, so a
// second run finds no actionable group and writes nothing.
//
// Selection is by LINE INDEX, never by id-lookup (the id is the recycled
// key here), and every untouched line round-trips byte-identical — asserted
// before the atomic rewrite, the same discipline as UpdateTask/DedupeBackfill.

// IDDupeGroup is one recycled-id group in the dedupe plan. KeptLine is the
// 1-based line that survives; ClosedLines (1-based) are the later lines the
// run closes as superseded-by-earliest, in file order. ClosedTitles and
// ClosedStatusWas are parallel to ClosedLines — the preserved original text
// and the pre-close status, so the dry-run plan already shows what the events
// will quote. AlreadyTerminal lists later lines that were already terminal
// (complete/failed) and are left untouched.
type IDDupeGroup struct {
	ID              string   `json:"id"`
	KeptLine        int      `json:"kept_line"`
	ClosedLines     []int    `json:"closed_lines"`
	ClosedTitles    []string `json:"closed_titles"`
	ClosedStatusWas []string `json:"closed_status_was"`
	AlreadyTerminal []int    `json:"already_terminal_lines,omitempty"`
}

// IDDupeReport is the result of one id-aware dedupe run. Summary only;
// everything is derived from the parsed board.
type IDDupeReport struct {
	BoardPath   string        `json:"board_path"`
	Apply       bool          `json:"apply"`
	TaskRows    int           `json:"task_rows"`
	DistinctIDs int           `json:"distinct_ids"`
	RecycledIDs int           `json:"recycled_ids"` // ids appearing on more than one line
	Groups      []IDDupeGroup `json:"groups"`
	// ClosedAway labels each closed line as "<id> (line N)", in file order.
	ClosedAway []string `json:"closed_away"`
}

// Changed reports whether the run has anything to write.
func (r *IDDupeReport) Changed() bool { return len(r.ClosedAway) > 0 }

// idDupeRow is one parsed task row plus its file line index and id.
type idDupeRow struct {
	idx int
	id  string
	row *Row
}

// DedupeByID runs (or previews) the DF-BOARDCTL-10 id-aware collapse on this
// board. See the package-file comment above for the exact contract.
func (b *Board) DedupeByID(apply bool) (*IDDupeReport, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	rep := &IDDupeReport{BoardPath: b.tasksPath, Apply: apply}

	var entries []*idDupeRow
	var lastRow *Row
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: the header is not a task row
		}
		id := row.String("id")
		if id == "" {
			return nil // rows without ids cannot carry a recycled key
		}
		entries = append(entries, &idDupeRow{idx: idx, id: id, row: row})
		lastRow = row
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.tasksPath, err)
	}
	rep.TaskRows = len(entries)

	// Pass 1 — classify. First-seen id order keeps the plan deterministic and
	// readable; members stay in file order (append-only board == filing
	// order, so members[0] is the earliest line for the id).
	byID := map[string][]*idDupeRow{}
	var firstSeen []string
	seenID := map[string]bool{}
	for _, e := range entries {
		if !seenID[e.id] {
			seenID[e.id] = true
			firstSeen = append(firstSeen, e.id)
		}
		byID[e.id] = append(byID[e.id], e)
	}
	rep.DistinctIDs = len(firstSeen)

	for _, id := range firstSeen {
		members := byID[id]
		if len(members) < 2 {
			continue
		}
		rep.RecycledIDs++
		g := IDDupeGroup{ID: id, KeptLine: members[0].idx + 1}
		for _, later := range members[1:] {
			st := later.row.String("status")
			canonical, ok, _ := ResolveStatus(st)
			if !ok {
				return nil, fmt.Errorf("%s line %d: task %q status %q is not a canonical value or a known read alias — decide the correct status before running the id-aware dedupe (nothing written)",
					b.tasksPath, later.idx+1, id, st)
			}
			if canonical == "complete" || canonical == "failed" {
				g.AlreadyTerminal = append(g.AlreadyTerminal, later.idx+1)
				continue
			}
			g.ClosedLines = append(g.ClosedLines, later.idx+1)
			g.ClosedTitles = append(g.ClosedTitles, later.row.String("title"))
			g.ClosedStatusWas = append(g.ClosedStatusWas, st)
			rep.ClosedAway = append(rep.ClosedAway, fmt.Sprintf("%s (line %d)", id, later.idx+1))
		}
		if len(g.ClosedLines) > 0 {
			rep.Groups = append(rep.Groups, g)
		}
	}

	// Pass 2 — dry-run (or nothing to do) stops here: the plan is the product.
	if !apply || !rep.Changed() {
		return rep, nil
	}

	newLines := make([][]byte, len(lines))
	copy(newLines, lines)
	now := b.tasksTSFormat(lastRow).Now()
	today := time.Now().UTC().Format("2006-01-02")
	planned := map[int]bool{}

	for _, g := range rep.Groups {
		kept := byID[g.ID][0]
		for _, later := range byID[g.ID][1:] {
			if !slices.Contains(g.ClosedLines, later.idx+1) {
				continue // already terminal, or the kept line itself
			}
			style := DetectStyle(lines[later.idx])
			if err := later.row.SetGoValue("status", "complete", style); err != nil {
				return nil, err
			}
			summary := fmt.Sprintf("superseded-by-earliest: recycled task id — kept line %d (id-dupe backfill %s)", g.KeptLine, today)
			if err := later.row.SetGoValue("worker_summary", summary, style); err != nil {
				return nil, err
			}
			// Machine-readable supersede marker (REVIEW-BOARDCTL-001
			// vocabulary): names the surviving row's id. Because the id is
			// the recycled key, the kept LINE number in worker_summary and
			// in the audit event is the unambiguous half of the pointer.
			if err := later.row.SetGoValue("superseded_by", kept.id, style); err != nil {
				return nil, err
			}
			if err := later.row.SetGoValue("completed_at", now, style); err != nil {
				return nil, err
			}
			if later.row.Has("updated_at") {
				if err := later.row.SetGoValue("updated_at", now, style); err != nil {
					return nil, err
				}
			}
			newLines[later.idx] = later.row.Marshal(style)
			planned[later.idx] = true
		}
	}

	// Assert every line the run did not intend to touch round-trips
	// byte-identical BEFORE writing.
	for i, l := range lines {
		if !bytes.Equal(l, newLines[i]) && !planned[i] {
			return nil, fmt.Errorf("internal error: untouched line %d would change — id-aware dedupe aborted (nothing written)", i+1)
		}
	}
	if err := atomicRewrite(b.tasksPath, JoinLines(newLines)); err != nil {
		return nil, err
	}

	// One audit event per closed line, AFTER the row rewrite succeeded. The
	// detail quotes the closed line's original title and reasoning verbatim
	// (RowFindingReasoning renders object-form reasonings on their human
	// readable member) — the finding text survives in the event log.
	for _, g := range rep.Groups {
		kept := byID[g.ID][0]
		for _, later := range byID[g.ID][1:] {
			if !planned[later.idx] {
				continue
			}
			pos := slices.Index(g.ClosedLines, later.idx+1)
			detail, err := idDupeEventDetail(idDupeEvent{
				ID:         g.ID,
				ClosedLine: later.idx + 1,
				KeptLine:   g.KeptLine,
				KeptID:     kept.id,
				StatusWas:  g.ClosedStatusWas[pos],
				Title:      g.ClosedTitles[pos],
				Reasoning:  RowFindingReasoning(later.row),
			})
			if err != nil {
				return rep, fmt.Errorf("board rewritten but id-dupe audit event detail failed: %w", err)
			}
			if _, err := b.AppendEvent(EventSpec{
				Type:   "audit",
				TaskID: g.ID,
				Actor:  "boardctl",
				Detail: detail,
			}); err != nil {
				return rep, fmt.Errorf("board rewritten but id-dupe audit event failed: %w", err)
			}
		}
	}
	return rep, nil
}

// idDupeEvent is the payload quoted into the audit event of one closed line.
type idDupeEvent struct {
	ID         string
	ClosedLine int
	KeptLine   int
	KeptID     string
	StatusWas  string
	Title      string
	Reasoning  string
}

// idDupeEventDetail renders the event detail as compact JSON. SetEscapeHTML
// is off so titles keep their literal <, >, & bytes — the quoting must be
// verbatim, that is its whole point.
func idDupeEventDetail(e idDupeEvent) ([]byte, error) {
	payload := struct {
		Action     string `json:"action"`
		ClosedLine int    `json:"closed_line"`
		KeptLine   int    `json:"kept_line"`
		KeptID     string `json:"kept_id"`
		StatusWas  string `json:"status_was"`
		Title      string `json:"title"`
		Reasoning  string `json:"reasoning"`
		ClosedAs   string `json:"closed_as"`
		RecycledID string `json:"recycled_task_id"`
	}{Action: "id-dupe", ClosedLine: e.ClosedLine, KeptLine: e.KeptLine, KeptID: e.KeptID,
		StatusWas: e.StatusWas, Title: e.Title, Reasoning: e.Reasoning,
		ClosedAs: "superseded-by-earliest", RecycledID: e.ID}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

// CloseIDDupeLine closes the ONE task line line1 (1-based) exactly the way
// DedupeByID closes a later duplicate: status=complete, worker_summary
// "superseded-by-earliest: recycled task id — kept line K (id-dupe backfill
// <date>)", superseded_by <the kept row's id>, completed_at now (updated_at
// refreshed only when the row carries the key), then one `audit` event
// quoting the closed line's original title and reasoning verbatim.
//
// REVIEW-BOARDCTL-001 --decide uses this as the sanctioned per-row write
// for a status="duplicate" row whose earlier same-id twin exists. Refusals
// (nothing written): the line number out of range, the line is a topology-B
// header, the line parses to no task id, or the line is ALREADY the
// earliest line of its id (there is no earlier twin to supersede — the
// keep-side must never close itself). Unlike DedupeByID this deliberately
// does NOT require the line's old status to resolve: the caller has
// already decided the row is a recyclable duplicate; "duplicate" is the
// exact state this close exists to repair.
func (b *Board) CloseIDDupeLine(line1 int) error {
	var closeErr error
	lockErr := b.withWriteLock(b.tasksPath, func() error {
		closeErr = b.closeIDDupeLineLocked(line1)
		return nil // err rides closeErr; the guard must drop regardless
	})
	if lockErr != nil {
		return lockErr
	}
	return closeErr
}

// closeIDDupeLineLocked is CloseIDDupeLine's body, running under the write guard.
func (b *Board) closeIDDupeLineLocked(line1 int) error {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return err
	}
	if line1 < 1 || line1 > len(lines) {
		return fmt.Errorf("close id-dupe line: line %d out of range (file has %d lines)", line1, len(lines))
	}
	idx := line1 - 1
	if b.skipTaskLine(lines, idx) {
		return fmt.Errorf("close id-dupe line: line %d is the board header, not a task row", line1)
	}
	closed, err := ParseRow(bytes.TrimSpace(lines[idx]))
	if err != nil {
		return fmt.Errorf("close id-dupe line: line %d does not parse: %w", line1, err)
	}
	id := closed.String("id")
	if id == "" {
		return fmt.Errorf("close id-dupe line: line %d carries no task id", line1)
	}
	// Capture the event's verbatim quotes BEFORE the close mutates the row —
	// the whole point of the event is the line's ORIGINAL text (the
	// pre-close status included), same as DedupeByID's ClosedStatusWas.
	statusWas := closed.String("status")
	title := closed.String("title")
	reasoning := RowFindingReasoning(closed)
	// Locate the EARLIEST line carrying the same id; the caller's line must
	// be a LATER duplicate of it — closing the keep-side would strand the id.
	keptIdx := -1
	if err := IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) || row.String("id") != id {
			return nil
		}
		if keptIdx == -1 {
			keptIdx = idx
		}
		return nil
	}); err != nil {
		return err
	}
	if keptIdx == -1 || keptIdx == idx {
		return fmt.Errorf("close id-dupe line: line %d (task %q) has no EARLIER same-id line to supersede — refusing to close the kept side", line1, id)
	}

	// Rewrite exactly the one line (same shape as DedupeByID pass 2).
	row := closed
	style := DetectStyle(lines[idx])
	if err := row.SetGoValue("status", "complete", style); err != nil {
		return err
	}
	summary := fmt.Sprintf("superseded-by-earliest: recycled task id — kept line %d (id-dupe backfill %s)", keptIdx+1, time.Now().UTC().Format("2006-01-02"))
	if err := row.SetGoValue("worker_summary", summary, style); err != nil {
		return err
	}
	if err := row.SetGoValue("superseded_by", id, style); err != nil {
		return err
	}
	now := b.tasksTSFormat(nil).Now()
	if err := row.SetGoValue("completed_at", now, style); err != nil {
		return err
	}
	if row.Has("updated_at") {
		if err := row.SetGoValue("updated_at", now, style); err != nil {
			return err
		}
	}
	newLines := make([][]byte, len(lines))
	copy(newLines, lines)
	newLines[idx] = row.Marshal(style)
	// Assert every untouched line round-trips byte-identical BEFORE writing.
	for i, l := range lines {
		if i != idx && !bytes.Equal(l, newLines[i]) {
			return fmt.Errorf("internal error: untouched line %d would change — id-dupe line close aborted (nothing written)", i+1)
		}
	}
	if err := atomicRewrite(b.tasksPath, JoinLines(newLines)); err != nil {
		return err
	}
	// The audit event quotes the closed line's original title, reasoning and
	// pre-close status verbatim — same contract as DedupeByID's per-line
	// events.
	detail, err := idDupeEventDetail(idDupeEvent{
		ID:         id,
		ClosedLine: line1,
		KeptLine:   keptIdx + 1,
		KeptID:     id,
		StatusWas:  statusWas,
		Title:      title,
		Reasoning:  reasoning,
	})
	if err != nil {
		return fmt.Errorf("line rewritten but id-dupe audit event detail failed: %w", err)
	}
	if _, err := b.AppendEvent(EventSpec{
		Type:   "audit",
		TaskID: id,
		Actor:  "boardctl",
		Detail: detail,
	}); err != nil {
		return fmt.Errorf("line rewritten but id-dupe audit event failed: %w", err)
	}
	return nil
}
