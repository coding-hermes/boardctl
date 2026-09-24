package board

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// SG-126 one-shot backfill.
//
// Boards written before the fingerprint rule carry duplicate findings: the same
// normalized title+reasoning filed 3-8x, all still open. DedupeBackfill
// collapses each such collision group:
//
//   - every OPEN row without a fingerprint gains one (computed from its own
//     title + reasoning), so future filings are suppressed;
//   - each group of >= 2 open rows sharing a fingerprint keeps the EARLIEST
//     row (append-only file order == filing order) and closes the rest with
//     status=complete, worker_summary "merged into <kept>: dedupe backfill
//     <date>", superseded_by <kept> (REVIEW-BOARDCTL-001: a machine-readable
//     merge marker — consumers no longer parse the worker_summary prose),
//     completed_at now;
//   - the merged-away rows' evidence is appended to the kept row's detail, so
//     the re-observation count survives the collapse;
//   - one `audit` event per merge group records kept / merged / fingerprint.
//
// dry-run (apply=false) computes the identical report and writes NOTHING —
// neither tasks.jsonl nor events.jsonl is touched. apply=true is idempotent:
// a second run finds every open row fingerprinted and no collision groups, so
// it writes nothing either.
//
// Group members are identified by LINE INDEX, never by id: live QA/dogfood
// boards carry the same id on several lines (a per-cycle id slot reused by
// later ticks), so an id-keyed selection would keep two rows and silently skip
// the merge. Report entries therefore carry the line numbers alongside the ids,
// and a group whose members share an id is flagged.
//
// Every untouched line round-trips byte-identical (same discipline as
// UpdateTask): the pre-write assertion aborts the whole run if any line other
// than an intended target would change.

// DedupeMerge is one collapsed collision group. Kept/Merged are the ids (the
// audit-event shape); the line numbers disambiguate boards that reuse an id on
// several rows.
type DedupeMerge struct {
	Kept         string   `json:"kept"`
	KeptLine     int      `json:"kept_line"`
	Merged       []string `json:"merged"`
	MergedLines  []int    `json:"merged_lines"`
	Fingerprint  string   `json:"fingerprint"`
	DuplicateIDs bool     `json:"duplicate_ids,omitempty"`
}

// DedupeReport is the result of one backfill run. Summary only counts; nothing
// here is invented — every list is derived from the parsed board.
type DedupeReport struct {
	BoardPath     string        `json:"board_path"`
	Apply         bool          `json:"apply"`
	OpenRows      int           `json:"open_rows"`
	AlreadySet    int           `json:"already_fingerprinted"`
	Fingerprinted []string      `json:"fingerprinted"` // open rows that gained one
	Drifted       []string      `json:"drifted"`       // stored fingerprint != recomputed
	Groups        []DedupeMerge `json:"groups"`
	MergedAway    []string      `json:"merged_away"`
}

// Changed reports whether the run has anything to write.
func (r *DedupeReport) Changed() bool {
	return len(r.Fingerprinted) > 0 || len(r.Groups) > 0 || len(r.Drifted) > 0
}

// findingRow is one parsed task row plus its file line index and effective
// fingerprint.
type findingRow struct {
	idx      int
	id       string
	row      *Row
	stored   string
	computed string
	open     bool
}

func (f *findingRow) fingerprint() string {
	if f.stored != "" {
		return f.stored
	}
	return f.computed
}

// DedupeBackfill runs (or previews) the SG-126 dedupe collapse on this board.
func (b *Board) DedupeBackfill(apply bool) (*DedupeReport, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	rep := &DedupeReport{BoardPath: b.tasksPath, Apply: apply}

	var entries []*findingRow
	err = IterParsed(lines, func(row *Row, idx int, _ []byte) error {
		if b.skipTaskLine(lines, idx) {
			return nil // topology B: the header is not a task row
		}
		id := row.String("id")
		if id == "" {
			return nil
		}
		entries = append(entries, &findingRow{
			idx:      idx,
			id:       id,
			row:      row,
			stored:   StoredFingerprint(row),
			computed: FindingFingerprintForRow(row),
			open:     RowIsOpenFinding(row),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", b.tasksPath, err)
	}

	// Pass 1 — classify. Nothing is mutated here.
	byFingerprint := map[string][]*findingRow{}
	for _, f := range entries {
		if !f.open {
			continue
		}
		rep.OpenRows++
		if f.stored == "" {
			rep.Fingerprinted = append(rep.Fingerprinted, f.id)
		} else {
			rep.AlreadySet++
			if f.stored != f.computed {
				// A stored fingerprint that no longer matches its own row is
				// reported, never silently rewritten: the stored value is the
				// one the gate already matched against.
				rep.Drifted = append(rep.Drifted, f.id)
			}
		}
		byFingerprint[f.fingerprint()] = append(byFingerprint[f.fingerprint()], f)
	}
	var groupRows [][]*findingRow
	for _, fp := range SortedKeys(byFingerprint) {
		members := byFingerprint[fp]
		if len(members) < 2 {
			continue
		}
		g := DedupeMerge{
			Kept:        members[0].id,
			KeptLine:    members[0].idx + 1,
			Fingerprint: fp,
		}
		seenID := map[string]bool{members[0].id: true}
		for _, m := range members[1:] {
			g.Merged = append(g.Merged, m.id)
			g.MergedLines = append(g.MergedLines, m.idx+1)
			if seenID[m.id] {
				g.DuplicateIDs = true
			}
			seenID[m.id] = true
			rep.MergedAway = append(rep.MergedAway, m.id)
		}
		rep.Groups = append(rep.Groups, g)
		groupRows = append(groupRows, members)
	}

	// Pass 2 — dry-run stops here: the report is the whole product.
	if !apply {
		return rep, nil
	}

	newLines := make([][]byte, len(lines))
	copy(newLines, lines)
	tsf := b.tasksTSFormat(lastTaskOf(entries))
	now := tsf.Now()
	today := time.Now().UTC().Format("2006-01-02")

	// planned holds the line indexes the run intends to rewrite; the untouched
	// assertion below is scoped to exactly this set.
	planned := map[int]bool{}

	// Fingerprint-only write-back for every open row that lacks one.
	for _, f := range entries {
		if !f.open || f.stored != "" {
			continue
		}
		pd := ParseFindingDetail(f.row.Get("detail")).withFingerprint(f.computed)
		enc, err := EncodeFindingDetail(pd, DetectStyle(lines[f.idx]))
		if err != nil {
			return nil, err
		}
		f.row.SetRaw("detail", enc)
		newLines[f.idx] = f.row.Marshal(DetectStyle(lines[f.idx]))
		planned[f.idx] = true
	}

	// Collapse each group: keep the earliest row (lowest line index), close the
	// rest, carry the merged rows' evidence onto the kept row.
	for _, members := range groupRows {
		kept := members[0]
		style := DetectStyle(lines[kept.idx])
		pd := ParseFindingDetail(kept.row.Get("detail"))
		if pd.Finding.Fingerprint == "" {
			pd = pd.withFingerprint(kept.fingerprint())
		}
		var carried []EvidenceEntry
		for _, m := range members[1:] {
			carried = append(carried, ParseFindingDetail(m.row.Get("detail")).Finding.Evidence...)
			carried = append(carried, EvidenceEntry{TaskID: m.id, TS: now})
		}
		pd = pd.withEvidence(carried)
		enc, err := EncodeFindingDetail(pd, style)
		if err != nil {
			return nil, err
		}
		kept.row.SetRaw("detail", enc)
		newLines[kept.idx] = kept.row.Marshal(style)
		planned[kept.idx] = true

		for _, m := range members[1:] {
			mstyle := DetectStyle(lines[m.idx])
			summary := fmt.Sprintf("merged into %s: dedupe backfill %s", kept.id, today)
			if err := m.row.SetGoValue("status", "complete", mstyle); err != nil {
				return nil, err
			}
			if err := m.row.SetGoValue("worker_summary", summary, mstyle); err != nil {
				return nil, err
			}
			// REVIEW-BOARDCTL-001: machine-readable merge marker naming the
			// surviving row. superseded_by is a sanctioned key (keys.go), so
			// this is pure value-writing, no canon change.
			if err := m.row.SetGoValue("superseded_by", kept.id, mstyle); err != nil {
				return nil, err
			}
			if err := m.row.SetGoValue("completed_at", now, mstyle); err != nil {
				return nil, err
			}
			if m.row.Has("updated_at") {
				if err := m.row.SetGoValue("updated_at", now, mstyle); err != nil {
					return nil, err
				}
			}
			newLines[m.idx] = m.row.Marshal(mstyle)
			planned[m.idx] = true
		}
	}

	// Assert every line the run did not intend to touch round-trips
	// byte-identical BEFORE writing.
	for i, l := range lines {
		if !bytes.Equal(l, newLines[i]) && !planned[i] {
			return nil, fmt.Errorf("internal error: untouched line %d would change — dedupe aborted (nothing written)", i+1)
		}
	}
	if err := atomicRewrite(b.tasksPath, JoinLines(newLines)); err != nil {
		return nil, err
	}

	// One audit event per merge group, AFTER the row rewrite succeeded.
	for _, g := range rep.Groups {
		var buf bytes.Buffer
		buf.WriteString(`{"action":"dedupe-backfill","kept":`)
		buf.Write(jsonString(g.Kept, DefaultStyle()))
		buf.WriteString(`,"kept_line":`)
		fmt.Fprintf(&buf, "%d", g.KeptLine)
		buf.WriteString(`,"merged":[`)
		for i, id := range g.Merged {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.Write(jsonString(id, DefaultStyle()))
		}
		buf.WriteString(`],"merged_lines":[`)
		for i, ln := range g.MergedLines {
			if i > 0 {
				buf.WriteString(",")
			}
			fmt.Fprintf(&buf, "%d", ln)
		}
		buf.WriteString(`],"fingerprint":`)
		buf.Write(jsonString(g.Fingerprint, DefaultStyle()))
		buf.WriteString(`}`)
		if _, err := b.AppendEvent(EventSpec{
			Type:   "audit",
			TaskID: g.Kept,
			Actor:  "boardctl",
			Detail: buf.Bytes(),
		}); err != nil {
			return rep, fmt.Errorf("board rewritten but dedupe-backfill audit event failed: %w", err)
		}
	}
	return rep, nil
}

// lastTaskOf returns the last parsed task row (the board's serialization
// dialect sample), or nil for an empty board.
func lastTaskOf(entries []*findingRow) *Row {
	if len(entries) == 0 {
		return nil
	}
	return entries[len(entries)-1].row
}

// LaneName derives the lane a board belongs to: the header's project (or
// namespace) when the board carries one, else the directory name of the repo
// the board lives in (…/<lane>/.coding-hermes/board). Used by the backfill CLI
// to refuse a board the operator did not name.
func (b *Board) LaneName() string {
	if hdr, err := b.HeaderRow(); err == nil {
		if p := strings.TrimSpace(hdr.String("project")); p != "" {
			return p
		}
		if n := strings.TrimSpace(hdr.String("namespace")); n != "" {
			return n
		}
	}
	dir := filepath.Dir(b.Dir)
	if filepath.Base(dir) == ".coding-hermes" {
		return filepath.Base(filepath.Dir(dir))
	}
	return filepath.Base(dir)
}
