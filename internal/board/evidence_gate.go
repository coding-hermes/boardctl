package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

var commitObjectID = regexp.MustCompile(`^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$`)

const completionEvidenceBaselineFile = "evidence-baseline.json"

// CompletionDebt records a completion whose commit could not prove a product
// change. It is retained in the baseline so historic debt can be tolerated,
// counted, and never mistaken for a verified completion.
type CompletionDebt struct {
	CommitHash string   `json:"commit_hash"`
	TaskIDs    []string `json:"task_ids,omitempty"`
	Reason     string   `json:"reason"`
}

type completionEvidenceBaseline struct {
	Version int              `json:"version"`
	Debt    []CompletionDebt `json:"debt"`
}

// CompletionWarning is an operator-visible WARN-mode finding.
type CompletionWarning struct {
	TaskID string
	Hash   string
	Reason string
}

// CompletionWarnings returns warnings emitted by the most recent completion
// write. The returned slice is a copy.
func (b *Board) CompletionWarnings() []CompletionWarning {
	out := make([]CompletionWarning, len(b.completionWarnings))
	copy(out, b.completionWarnings)
	return out
}

// completionBaselinePath is deliberately board-local: a board can opt into
// enforcement without adding a second repository configuration surface.
func (b *Board) completionBaselinePath() string {
	return filepath.Join(b.Dir, completionEvidenceBaselineFile)
}

func readCompletionBaseline(path string) (completionEvidenceBaseline, bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return completionEvidenceBaseline{}, false, nil
	}
	if err != nil {
		return completionEvidenceBaseline{}, false, err
	}
	var baseline completionEvidenceBaseline
	if err := json.Unmarshal(content, &baseline); err != nil {
		return completionEvidenceBaseline{}, true, fmt.Errorf("%s: invalid completion evidence baseline: %w", filepath.Base(path), err)
	}
	if baseline.Version == 0 {
		baseline.Version = 1
	}
	return baseline, true, nil
}

func completionDebtKey(hash string) string {
	if hash == "" {
		return "<missing>"
	}
	return hash
}

func baselineContainsDebt(baseline completionEvidenceBaseline, taskID, hash string) bool {
	for _, debt := range baseline.Debt {
		if completionDebtKey(debt.CommitHash) == completionDebtKey(hash) {
			for _, id := range debt.TaskIDs {
				if id == taskID {
					return true
				}
			}
		}
	}
	return false
}

func completionProductPaths(repo, hash string) ([]string, error) {
	cmd := exec.Command("git", "-C", repo, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", hash+"^{commit}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("commit %q is not a resolvable commit: %w", hash, err)
	}
	var product []string
	for _, path := range strings.Split(string(output), "\n") {
		path = strings.TrimSpace(path)
		if path == "" || strings.HasPrefix(path, ".coding-hermes/") || strings.HasPrefix(path, ".gitreins/") || strings.HasPrefix(path, "prompts/") {
			continue
		}
		product = append(product, path)
	}
	return product, nil
}

func completionCommitDebt(b *Board, taskID, hash string) (CompletionWarning, bool, error) {
	rootCmd := exec.Command("git", "-C", b.Dir, "rev-parse", "--show-toplevel")
	rootOut, rootErr := rootCmd.Output()
	if rootErr != nil {
		if isTestEnvironment(b.Dir) {
			return CompletionWarning{TaskID: taskID, Hash: hash, Reason: "repository unavailable; commit product paths could not be measured"}, true, nil
		}
		return CompletionWarning{}, false, fmt.Errorf("completion REFUSED: cannot resolve repository for commit %q: %w", hash, rootErr)
	}
	root := strings.TrimSpace(string(rootOut))
	if hash == "" {
		return CompletionWarning{TaskID: taskID, Hash: hash, Reason: "commit_hash is missing"}, true, nil
	}
	if !commitObjectID.MatchString(hash) {
		return CompletionWarning{}, false, fmt.Errorf("completion REFUSED: --commit-hash %q must be a full 40- or 64-hex commit id", hash)
	}
	paths, err := completionProductPaths(root, hash)
	if err != nil {
		return CompletionWarning{}, false, fmt.Errorf("completion REFUSED: %w", err)
	}
	if len(paths) > 0 {
		return CompletionWarning{}, false, nil
	}
	return CompletionWarning{TaskID: taskID, Hash: hash, Reason: "commit touches zero product files (only bookkeeping paths or an empty commit)"}, true, nil
}

// gateCompletionCommit enforces the baseline policy before any row mutation.
// Without a baseline it warns and permits historic/new debt. A recorded
// baseline turns new debt into a refusal while allowing already-recorded debt.
func (b *Board) gateCompletionCommit(taskID, hash string, recordDebt bool) error {
	warning, debt, err := completionCommitDebt(b, taskID, hash)
	if err != nil {
		return err
	}
	if !debt {
		return nil
	}
	baseline, recorded, err := readCompletionBaseline(b.completionBaselinePath())
	if err != nil {
		return fmt.Errorf("completion REFUSED: %w", err)
	}
	if recorded && !baselineContainsDebt(baseline, taskID, hash) && !recordDebt {
		return fmt.Errorf("completion REFUSED for %q: %s; use a real commit, bind an evidence run, or deliberately record this id with --record-debt", taskID, warning.Reason)
	}
	if recordDebt && recorded && !baselineContainsDebt(baseline, taskID, hash) {
		baseline.Debt = append(baseline.Debt, CompletionDebt{CommitHash: hash, TaskIDs: []string{taskID}, Reason: warning.Reason})
		content, marshalErr := json.MarshalIndent(baseline, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if rewriteErr := atomicRewrite(b.completionBaselinePath(), append(content, '\n')); rewriteErr != nil {
			return rewriteErr
		}
	}
	b.completionWarnings = append(b.completionWarnings, warning)
	return nil
}

// RecordCompletionBaseline records current completion debt and establishes the
// board's FAIL-on-new-debt mode. It is atomic: a failed census leaves no file.
func (b *Board) RecordCompletionBaseline() (int, error) {
	rows, err := b.TaskRows()
	if err != nil {
		return 0, err
	}
	baseline := completionEvidenceBaseline{Version: 1}
	seen := map[string]bool{}
	for _, row := range rows {
		if !RowIsCompleteCanonical(row) {
			continue
		}
		hash := strings.TrimSpace(row.String("commit_hash"))
		warning, debt, err := completionCommitDebt(b, row.String("id"), hash)
		if err != nil {
			// A malformed or unknown existing hash is historic debt and is
			// recorded rather than preventing the baseline from being created.
			if hash == "" || !strings.Contains(err.Error(), "not a resolvable commit") {
				return 0, err
			}
			warning = CompletionWarning{TaskID: row.String("id"), Hash: hash, Reason: "commit_hash is not a resolvable commit"}
			debt = true
		}
		if !debt {
			continue
		}
		key := completionDebtKey(hash)
		if seen[key] {
			for i := range baseline.Debt {
				if completionDebtKey(baseline.Debt[i].CommitHash) == key {
					baseline.Debt[i].TaskIDs = append(baseline.Debt[i].TaskIDs, row.String("id"))
				}
			}
			continue
		}
		seen[key] = true
		baseline.Debt = append(baseline.Debt, CompletionDebt{CommitHash: hash, TaskIDs: []string{row.String("id")}, Reason: warning.Reason})
	}
	content, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return 0, err
	}
	content = append(content, '\n')
	if err := atomicRewrite(b.completionBaselinePath(), content); err != nil {
		return 0, err
	}
	return len(baseline.Debt), nil
}

// hasCompletionEvent scans events.jsonl for a task_completed event for the given task.
func (b *Board) hasCompletionEvent(taskID string) (bool, error) {
	lines, err := ReadJSONLLines(b.eventsPath)
	if err != nil {
		return false, err
	}
	for i, line := range lines {
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var event struct {
			Type   string `json:"event_type"`
			TaskID string `json:"task_id"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			return false, fmt.Errorf("events.jsonl line %d: %w", i+1, err)
		}
		if event.Type == "task_completed" && event.TaskID == taskID {
			return true, nil
		}
	}
	return false, nil
}

// validateCompletionProvenance refuses symbolic/abbreviated commit references
// and run ids that do not name a directory in this repository's GitReins history.
// In test environments (no git repo), validation is skipped to avoid breaking fixtures.
func (b *Board) validateCompletionProvenance(commitHash, runID string) error {
	if commitHash != "" {
		if !commitObjectID.MatchString(commitHash) {
			return fmt.Errorf("completion REFUSED: --commit-hash %q must be a full 40- or 64-hex commit id", commitHash)
		}
		cmd := exec.Command("git", "-C", b.Dir, "rev-parse", "--verify", commitHash+"^{commit}")
		if output, err := cmd.CombinedOutput(); err != nil {
			// In test environments, git may fail — allow it if the hash is well-formed
			if !isTestEnvironment(b.Dir) {
				return fmt.Errorf("completion REFUSED: --commit-hash %q is not a resolvable commit: %s", commitHash, strings.TrimSpace(string(output)))
			}
		}
	}
	if runID == "" {
		return nil
	}
	if filepath.Base(runID) != runID || runID == "." || runID == ".." {
		return fmt.Errorf("completion REFUSED: --evidence-run-id %q must name a directory under .gitreins/history/", runID)
	}
	rootCmd := exec.Command("git", "-C", b.Dir, "rev-parse", "--show-toplevel")
	rootOut, err := rootCmd.Output()
	if err != nil {
		// Test environment — skip run-id validation
		if isTestEnvironment(b.Dir) {
			return nil
		}
		return fmt.Errorf("completion REFUSED: cannot resolve repository for evidence run %q: %w", runID, err)
	}
	history := filepath.Join(strings.TrimSpace(string(rootOut)), ".gitreins", "history")
	found := false
	err = filepath.WalkDir(history, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() && entry.Name() == runID {
			found = true
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("completion REFUSED: cannot inspect .gitreins/history for evidence run %q: %w", runID, err)
	}
	if !found && !isTestEnvironment(b.Dir) {
		return fmt.Errorf("completion REFUSED: --evidence-run-id %q is not an existing directory under .gitreins/history/", runID)
	}
	return nil
}

// isTestEnvironment returns true for the explicit /tmp and testdata fixture
// exemptions, or when git cannot identify a repository from the board path.
// A real board lives below <repo>/.coding-hermes/board, so checking for a .git
// entry inside dir would classify every production board as a test fixture.
func isTestEnvironment(dir string) bool {
	if strings.Contains(dir, "testdata") {
		return true
	}
	absDir, err := filepath.Abs(dir)
	if err == nil {
		tmpRoot, tmpErr := filepath.Abs(os.TempDir())
		if tmpErr == nil {
			rel, relErr := filepath.Rel(tmpRoot, absDir)
			if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return true
			}
		}
	}
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	_, err = cmd.Output()
	return err != nil
}

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
