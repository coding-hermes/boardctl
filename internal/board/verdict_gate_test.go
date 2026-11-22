package board

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// INFRA-LORE-045: completion provenance must read the BIND RUN'S STORED
// verdict.json, not merely locate its directory. A run graded for another
// task (the live defect: lore run d46cce23 graded LORE-EXPORT-1 but closed
// INFRA-LORE-045) must be refused at completion. Fixtures that pass run ids
// with no history at all keep passing (test-env exemption unchanged).

const verdictDefectTaskID = "INFRA-LORE-045"
const verdictDefectForeignTaskID = "LORE-EXPORT-1"
const verdictDefectForeignCommit = "eda0c96f9a09a37361b4eebd11356300fcd66699"
const verdictDefectRunID = "d46cce23"

// verdictGateFixture writes a temp board carrying one pending row and an
// audit event, returns the board plus the board dir.
func verdictGateFixture(t *testing.T) (*Board, string) {
	t.Helper()
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	row := fmt.Sprintf(`{"id": %q, "title": "verdict gate target", "status": "pending", "priority": "P1", "created_at": "2026-11-22 00:00:00", "updated_at": "2026-11-22 00:00:00"}`, verdictDefectTaskID)
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  row + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b, boardDir
}

// verdictGateGitRepo turns the temp board's repo root into a real git repo
// (so candidate (i) toplevel/.gitreins/history is the production layout) and
// returns repo root + the number of commits made so far.
func verdictGateGitRepo(t *testing.T, boardDir string) string {
	t.Helper()
	repo := filepath.Dir(filepath.Dir(boardDir)) // boardDir = repo/.coding-hermes/board
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	if _, err := os.Stat(filepath.Join(repo, ".git")); err == nil {
		return repo // already inited (verdictGateFixture is t.TempDir-based)
	}
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}

func verdictGateCommitAll(t *testing.T, repo string, message string) string {
	t.Helper()
	identity := []string{"-c", "user.email=verdict-gate-test@local", "-c", "user.name=verdict-gate-test"}
	for _, args := range [][]string{
		{"-C", repo, "add", "-A"},
		{"-C", repo, "commit", "--allow-empty", "-m", message},
	} {
		cmd := exec.Command("git", append(identity, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args[1:], err, out)
		}
	}
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// writeVerdictRun creates .gitreins/history/<date>/<runID>/ under root and
// stores a verdict for taskID at commit (commit may be "" to omit the field).
func writeVerdictRun(t *testing.T, root, date, runID, taskID, commit string, raw string) {
	t.Helper()
	runDir := filepath.Join(root, ".gitreins", "history", date, runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		raw = fmt.Sprintf(`{"task_id": %q, "commit": %q, "passed": true}`, taskID, commit)
	}
	if err := os.WriteFile(filepath.Join(runDir, "verdict.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readVerdictRowStatus(t *testing.T, boardDir string) string {
	t.Helper()
	content, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	return text
}

// TestVerdictGateRefusesRunGradedForForeignTask pins the live defect: a run
// dir that exists but whose verdict graded another task must be REFUSED, and
// the refusal must name the run, the graded task, its commit, and this row.
func TestVerdictGateRefusesRunGradedForForeignTask(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	verdictGateCommitAll(t, repo, "scratch commit before completion")
	_ = verdictGateCommitAll(t, repo, "product commit graded by foreign verdict")
	// verdict graded a DIFFERENT task (the INFRA-LORE-045 live defect shape)
	writeVerdictRun(t, repo, "2026-11-21", verdictDefectRunID, verdictDefectForeignTaskID, verdictDefectForeignCommit, "")
	before, _ := os.ReadFile(boardDir + "/tasks.jsonl")
	_, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: verdictDefectRunID,
	})
	if err == nil {
		t.Fatal("completion citing a run graded for another task must be REFUSED")
	}
	for _, want := range []string{
		verdictDefectRunID,
		verdictDefectForeignTaskID,
		"INFRA-LORE-045",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal must name %q, got: %v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "graded task") {
		t.Fatalf("refusal must say the run graded a task, got: %v", err)
	}
	after, _ := os.ReadFile(boardDir + "/tasks.jsonl")
	if string(before) != string(after) {
		t.Fatal("a refused completion must write NOTHING")
	}
}

// TestVerdictGateAcceptsRunGradedForThisTask: a matching verdict still closes.
func TestVerdictGateAcceptsRunGradedForThisTask(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	product := verdictGateCommitAll(t, repo, "product commit graded by own verdict")
	writeVerdictRun(t, repo, "2026-11-22", "a1b2c3d4", verdictDefectTaskID, product, "")
	if _, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr(product),
		EvidenceRunID: "a1b2c3d4",
	}); err != nil {
		t.Fatalf("matching-verdict completion must close: %v", err)
	}
	if status := readVerdictRowStatus(t, boardDir); !strings.Contains(status, `"status": "complete"`) {
		t.Fatalf("row must now read complete, got: %s", status)
	}
}

// TestVerdictGateRefusesRunWithoutVerdictFile: a located run dir with no
// verdict.json must be refused (today the gate stops at directory existence).
func TestVerdictGateRefusesRunWithoutVerdictFile(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	_ = verdictGateCommitAll(t, repo, "scratch commit")
	writeVerdictRun(t, repo, "2026-11-21", verdictDefectRunID, verdictDefectTaskID, "", "")
	_ = os.Remove(filepath.Join(repo, ".gitreins", "history", "2026-11-21", verdictDefectRunID, "verdict.json"))
	_, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: verdictDefectRunID,
	})
	if err == nil {
		t.Fatal("completion citing a run without a verdict.json must be REFUSED")
	}
	if !strings.Contains(err.Error(), "carries no verdict.json") || !strings.Contains(err.Error(), verdictDefectRunID) {
		t.Fatalf("refusal must name the run and the missing verdict, got: %v", err)
	}
}

// TestVerdictGateRefusesUnparsableVerdictJSON.
func TestVerdictGateRefusesUnparsableVerdictJSON(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	_ = verdictGateCommitAll(t, repo, "scratch commit")
	writeVerdictRun(t, repo, "2026-11-21", verdictDefectRunID, verdictDefectTaskID, "", `{this is not json]`)
	_, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: verdictDefectRunID,
	})
	if err == nil {
		t.Fatal("completion citing an unparsable verdict must be REFUSED")
	}
	if !strings.Contains(err.Error(), "verdict.json") || !strings.Contains(err.Error(), verdictDefectRunID) {
		t.Fatalf("refusal must name the run and the bad verdict, got: %v", err)
	}
}

// TestVerdictGateRefusesCommitMismatchVerdict: verdict task matches but the
// verdict graded a different (resolvable) commit than the row's --commit-hash.
func TestVerdictGateRefusesCommitMismatchVerdict(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	graded := verdictGateCommitAll(t, repo, "run graded this one")
	claimed := verdictGateCommitAll(t, repo, "row claims this one")
	writeVerdictRun(t, repo, "2026-11-22", "e5f6a7b8", verdictDefectTaskID, graded, "")
	_, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr(claimed),
		EvidenceRunID: "e5f6a7b8",
	})
	if err == nil {
		t.Fatal("completion whose run graded a different commit must be REFUSED")
	}
	if !strings.Contains(err.Error(), graded) || !strings.Contains(err.Error(), claimed) {
		t.Fatalf("refusal must name BOTH commits (graded=%s claimed=%s), got: %v", graded, claimed, err)
	}
	if !strings.Contains(err.Error(), "e5f6a7b8") {
		t.Fatalf("refusal must name the run, got: %v", err)
	}
}

// TestVerdictGateForceSkipsVerdictContent: --force is the documented
// deliberate bypass of the verdict content checks (format checks stay).
func TestVerdictGateForceSkipsVerdictContent(t *testing.T) {
	b, boardDir := verdictGateFixture(t)
	repo := verdictGateGitRepo(t, boardDir)
	product := verdictGateCommitAll(t, repo, "product commit")
	writeVerdictRun(t, repo, "2026-11-21", verdictDefectRunID, verdictDefectForeignTaskID, verdictDefectForeignCommit, "")
	_, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr(product),
		EvidenceRunID: verdictDefectRunID,
		Force:         true,
	})
	if err != nil {
		t.Fatalf("--force must bypass the verdict content check: %v", err)
	}
	if status := readVerdictRowStatus(t, boardDir); !strings.Contains(status, `"status": "complete"`) {
		t.Fatalf("row must read complete, got: %s", status)
	}
}

// TestVerdictGateEnforcedOnFlatBoardDirWithoutGit: the new carve-out — a
// flat /tmp-style board dir (no git repo) with the run dir under
// <boardDir>/.gitreins/history IS enforced now: foreign verdict refused,
// matching verdict accepted.
func TestVerdictGateEnforcedOnFlatBoardDirWithoutGit(t *testing.T) {
	t.Run("foreign verdict refused", func(t *testing.T) {
		dir := t.TempDir()
		row := fmt.Sprintf(`{"id": %q, "title": "flat board target", "status": "pending", "priority": "P1", "created_at": "2026-11-22 00:00:00", "updated_at": "2026-11-22 00:00:00"}`, verdictDefectTaskID)
		writeBoardFiles(t, dir, map[string]string{
			"tasks.jsonl":  row + "\n",
			"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		})
		writeVerdictRun(t, dir, "2026-11-21", verdictDefectRunID, verdictDefectForeignTaskID, verdictDefectForeignCommit, "")
		b, err := Resolve(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, err = b.UpdateTask(verdictDefectTaskID, UpdateSpec{
			Status:        strPtr("complete"),
			EvidenceRunID: verdictDefectRunID,
		})
		if err == nil || !strings.Contains(err.Error(), "graded task") {
			t.Fatalf("flat board dir must enforce verdict content, got: %v", err)
		}
	})
	t.Run("matching verdict closes", func(t *testing.T) {
		dir := t.TempDir()
		row := fmt.Sprintf(`{"id": %q, "title": "flat board target", "status": "pending", "priority": "P1", "created_at": "2026-11-22 00:00:00", "updated_at": "2026-11-22 00:00:00"}`, verdictDefectTaskID)
		writeBoardFiles(t, dir, map[string]string{
			"tasks.jsonl":  row + "\n",
			"events.jsonl": `{"id": 1, "timestamp": "2026-09-25 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		})
		writeVerdictRun(t, dir, "2026-11-22", "b3c4d5e6", verdictDefectTaskID, "00112233445566778899aabbccddeeff00112233", "")
		b, err := Resolve(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
			Status:        strPtr("complete"),
			EvidenceRunID: "b3c4d5e6",
		}); err != nil {
			t.Fatalf("matching verdict must close on a flat board dir: %v", err)
		}
	})
}

// TestVerdictGateRunWithoutHistoryStillPasses: regression — the existing
// fixture behavior (arbitrary run ids on test-env boards with NO git repo
// and NO history anywhere) keeps passing via the not-found exemption. A real
// git-repo board keeps its stricter existing contract: an unknown run id on
// a board with no .gitreins/history is refused (board_guard_003 pins that).
func TestVerdictGateRunWithoutHistoryStillPasses(t *testing.T) {
	b, _ := verdictGateFixture(t) // temp board, no git, no history
	if _, err := b.UpdateTask(verdictDefectTaskID, UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: "synthetic-run",
	}); err != nil {
		t.Fatalf("run id on a test-env board with no history dir must keep passing: %v", err)
	}
}
