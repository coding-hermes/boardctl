package board

import (
	"errors"
	"fmt"
	"strings"
)

// TR-251 mechanism (1): a transition into a terminal completion status is
// REFUSED unless the caller binds an evidence run (--evidence-run-id RUN).
//
// "complete" is the only status a completion can claim: the write vocabulary
// (StatusVocabulary) has no other terminal-success value ("done"/"completed"
// are READ aliases; the write paths reject them before this gate runs).
// Boards that treat "failed" as terminal are explicitly out of scope — the
// quorum row (TR-251) and docs/traceability-doctrine.md (task-router) both
// name complete-only.
//
// The gate is deliberately at the WRITE layer (every caller surface inherits
// it: CLI update, tests, future HTTP) rather than validate/doctor — doctor
// (AC2) reports the HISTORIC debt; this gate stops NEW debt. It fires only on
// a real TRANSITION: a row already complete (either spelling, canonical or a
// read alias, resolved through ResolveStatus) re-writing complete is an
// idempotent no-op transition and passes un-gated, mirroring the task-router
// append-only doctrine where history is never re-judged.
//
// The refusal names the missing flag (--evidence-run-id) so the operator's
// next command is copy-pasteable.

// EvidenceRunRefusal is the typed error UpdateTask/CreateTask return when a
// completion transition carries no evidence run.
type EvidenceRunRefusal struct {
	ID     string
	Status string
}

func (e *EvidenceRunRefusal) Error() string {
	return fmt.Sprintf(
		"completion REFUSED for %q: --evidence-run-id is REQUIRED when setting status %q (TR-251); re-run with --evidence-run-id <run-id> — the run id is recorded on the row's detail.evidence[] and must resolve to a stored run (e.g. .gitreins/history/<date>/<run-id>/verdict.json)",
		e.ID, e.Status)
}

// ErrEvidenceRunRequired is the sentinel wrapper callers can errors.Is against.
var ErrEvidenceRunRequired = errors.New("evidence run required")

// completionStatuses are the statuses the evidence gate treats as completion.
// Complete is the only canonical write spelling today; the set exists so the
// gate's definition of "completion" has one named home.
var completionStatuses = map[string]bool{
	"complete": true,
}

// isCompletionStatus reports whether s is a completion status in the WRITE
// vocabulary. Aliases never reach the gate: the write paths already reject
// them ("completed"/"done" are read-only spellings).
func isCompletionStatus(s string) bool {
	return completionStatuses[strings.ToLower(strings.TrimSpace(s))]
}

// RowIsCompleteCanonical reports whether a row's CURRENT status is complete
// (canonical spelling or a read alias, resolved via ResolveStatus). Used by
// the gate to distinguish a fresh completion (gated) from an idempotent
// re-write of an already-complete row (passes un-gated). Rows with an absent,
// unknown, or failed status are NOT complete: a failed→complete rewrite is a
// real transition and is gated.
func RowIsCompleteCanonical(row *Row) bool {
	if row == nil {
		return false
	}
	canon, ok, _ := ResolveStatus(row.String("status"))
	return ok && canon == "complete"
}

// gateCompletionEvidence enforces AC1 on a completion transition. target is
// the row's PRE-WRITE state (nil = the row is being created); status is the
// status being written; hasEvidenceRun reports the caller bound one.
func gateCompletionEvidence(target *Row, status string, hasEvidenceRun bool) error {
	if !isCompletionStatus(status) || hasEvidenceRun {
		return nil
	}
	if target != nil && RowIsCompleteCanonical(target) {
		return nil // idempotent re-write of an already-complete row
	}
	id := "<new-row>"
	if target != nil {
		id = target.String("id")
	}
	// The refusal NAMES the missing flag (AC1 wording) and wraps the typed
	// sentinel so both string-matching callers and errors.Is callers work.
	return fmt.Errorf("%w: %v", ErrEvidenceRunRequired, (&EvidenceRunRefusal{ID: id, Status: status}).Error())
}

// RowEvidenceRunIDs returns every evidence run id a row carries, in stored
// order. The evidence lives in the row's detail envelope
// (detail.evidence[].run_id, the SG-126 shape --evidence-run-id writes);
// a top-level evidence_run_id field is honored too — the task-router seed
// and several audit writers stamp that spelling directly on the row. Absent
// shapes contribute nothing; malformed detail JSON degrades to no runs (a
// row whose evidence cannot be parsed is unproven by definition).
func RowEvidenceRunIDs(row *Row) []string {
	if row == nil {
		return nil
	}
	var runs []string
	seen := map[string]bool{}
	for _, e := range ParseFindingDetail(row.Get("detail")).Finding.Evidence {
		if e.RunID != "" && !seen[e.RunID] {
			seen[e.RunID] = true
			runs = append(runs, e.RunID)
		}
	}
	if v := strings.TrimSpace(row.String("evidence_run_id")); v != "" && !seen[v] {
		runs = append(runs, v)
	}
	return runs
}

// VerifiedCompleteStats is the TR-251 mechanism (5) census over one board's
// task rows: how much of the "complete" mass is verified, and which rows
// claim completion without a bound evidence run. Total counts every row whose
// status resolves to complete (canonical or read alias); Verified counts the
// subset carrying at least one evidence run; UnverifiedIDs lists the rest in
// file order.
type VerifiedCompleteStats struct {
	Total         int
	Verified      int
	UnverifiedIDs []string
}

// Share is verified_complete_share in [0,1]; 0 when no complete rows exist
// (a division guard, not a claim that nothing is verified — callers decide
// how to present an empty denominator).
func (s VerifiedCompleteStats) Share() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Verified) / float64(s.Total)
}

// VerifiedCompleteStats computes the census over the board's task rows.
// Tolerant when the board was opened with SkipBad (validate itemizes the
// unreadable lines); the topology-B header line is never a task row.
func (b *Board) VerifiedCompleteCensus() (VerifiedCompleteStats, error) {
	var stats VerifiedCompleteStats
	rows, err := b.TaskRows()
	if err != nil {
		return stats, err
	}
	for _, row := range rows {
		canon, ok, _ := ResolveStatus(row.String("status"))
		if !ok || canon != "complete" {
			continue
		}
		stats.Total++
		if len(RowEvidenceRunIDs(row)) > 0 {
			stats.Verified++
		} else if id := row.String("id"); id != "" {
			stats.UnverifiedIDs = append(stats.UnverifiedIDs, id)
		}
	}
	return stats, nil
}
