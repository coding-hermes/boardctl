package board

import (
	"bytes"
	"fmt"
)

// StatusNormalizationRow records one non-canonical status found on a board.
// Aliases have a canonical target; unknown and missing values remain refused.
type StatusNormalizationRow struct {
	ID        string `json:"id"`
	Line      int    `json:"line"`
	Status    string `json:"status"`
	Canonical string `json:"canonical,omitempty"`
	Action    string `json:"action"`
	Applied   bool   `json:"applied,omitempty"`
}

// StatusNormalizationReport is a single-board, status-only normalization
// census. It deliberately does not infer a fleet or scan sibling boards.
type StatusNormalizationReport struct {
	BoardPath    string                   `json:"board_path"`
	Rows         int                      `json:"rows"`
	AliasCount   int                      `json:"alias_count"`
	UnknownCount int                      `json:"unknown_count"`
	MissingCount int                      `json:"missing_count"`
	Changed      int                      `json:"changed"`
	Refused      []StatusNormalizationRow `json:"refused"`
	Normalized   []StatusNormalizationRow `json:"normalized"`
	DryRun       bool                     `json:"dry_run"`
	Applied      bool                     `json:"applied"`
}

// NormalizeStatuses classifies status values on exactly this board. Dry-run
// does not write anything. Apply performs one guarded atomic rewrite: only
// known aliases change, untouched lines retain their original bytes, and
// unknown/missing statuses are reported but never guessed.
func (b *Board) NormalizeStatuses(apply bool) (*StatusNormalizationReport, error) {
	var rep *StatusNormalizationReport
	var opErr error
	lockErr := b.withWriteLock(b.tasksPath, func() error {
		rep, opErr = b.normalizeStatusesLocked(apply)
		return nil
	})
	if lockErr != nil {
		return nil, lockErr
	}
	return rep, opErr
}

func (b *Board) normalizeStatusesLocked(apply bool) (*StatusNormalizationReport, error) {
	lines, err := ReadJSONLLines(b.tasksPath)
	if err != nil {
		return nil, err
	}
	rep := &StatusNormalizationReport{
		BoardPath: b.tasksPath,
		DryRun:    !apply,
	}
	out := append([][]byte(nil), lines...)

	for idx, raw := range lines {
		if b.skipTaskLine(lines, idx) {
			continue
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		row, err := ParseRow(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: line %d: %w", b.tasksPath, idx+1, err)
		}
		if row.String("id") == "" {
			continue
		}
		rep.Rows++
		status := row.String("status")
		if status == "" {
			rep.MissingCount++
			rep.Refused = append(rep.Refused, StatusNormalizationRow{
				ID:     row.String("id"),
				Line:   idx + 1,
				Action: "refused: status is missing; set an explicit canonical status",
			})
			continue
		}
		canonical, ok, alias := ResolveStatus(status)
		if !ok {
			rep.UnknownCount++
			rep.Refused = append(rep.Refused, StatusNormalizationRow{
				ID:     row.String("id"),
				Line:   idx + 1,
				Status: status,
				Action: "refused: status is unknown; set an explicit canonical status",
			})
			continue
		}
		if !alias {
			continue
		}
		rep.AliasCount++
		entry := StatusNormalizationRow{
			ID:        row.String("id"),
			Line:      idx + 1,
			Status:    status,
			Canonical: canonical,
			Action:    fmt.Sprintf("normalize: status %q -> %q", status, canonical),
		}
		rep.Normalized = append(rep.Normalized, entry)
		style := DetectStyle(raw)
		if err := row.SetGoValue("status", canonical, style); err != nil {
			return nil, fmt.Errorf("%s: line %d: normalize status: %w", b.tasksPath, idx+1, err)
		}
		out[idx] = row.Marshal(style)
	}

	rep.Changed = len(rep.Normalized)
	if !apply || len(rep.Normalized) == 0 {
		return rep, nil
	}
	for idx := range lines {
		changedLine := false
		for _, row := range rep.Normalized {
			if row.Line == idx+1 {
				changedLine = true
				break
			}
		}
		if !changedLine && !bytes.Equal(lines[idx], out[idx]) {
			return nil, fmt.Errorf("internal error: untouched line %d would change", idx+1)
		}
	}
	if err := atomicRewrite(b.tasksPath, JoinLines(out)); err != nil {
		return nil, err
	}
	rep.Applied = true
	rep.DryRun = false
	rep.Changed = len(rep.Normalized)
	for i := range rep.Normalized {
		rep.Normalized[i].Applied = true
	}
	return rep, nil
}
