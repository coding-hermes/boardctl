package board

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func boardGuard003Fixture(t *testing.T, targetStatus string, events string) (*Board, string) {
	t.Helper()
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
	tasks := strings.Replace(tr251Board(), `"status": "pending"`, `"status": "`+targetStatus+`"`, 1)
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tasks,
		"events.jsonl": events,
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b, boardDir
}

const boardGuard003Audit = `{"id":1,"timestamp":"2026-09-25 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n"

func TestBoardGuard003RejectsSymbolicCommitHashWithoutWrites(t *testing.T) {
	b, dir := boardGuard003Fixture(t, "pending", boardGuard003Audit)
	tasksBefore, err := os.ReadFile(dir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(dir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr("HEAD"),
		EvidenceRunID: "synthetic-run",
	})
	if err == nil || !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("symbolic commit hash must be refused with completion REFUSED, got %v", err)
	}
	tasksAfter, err := os.ReadFile(dir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := os.ReadFile(dir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) {
		t.Fatal("refused completion changed tasks.jsonl")
	}
	if string(eventsBefore) != string(eventsAfter) {
		t.Fatal("refused completion appended an event")
	}
}

func TestBoardGuard003AcceptsFullCommitHashWithoutGitRepo(t *testing.T) {
	b, _ := boardGuard003Fixture(t, "pending", boardGuard003Audit)
	_, err := b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr(strings.Repeat("a", 40)),
		EvidenceRunID: "synthetic-run",
	})
	if err != nil {
		t.Fatalf("full hash and synthetic evidence run should pass without a git repo: %v", err)
	}
}

func TestBoardGuard003RealRepoRejectsUnprovenProvenance(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(wd, "..", "..", ".tick-cache")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	repo, err := os.MkdirTemp(scratch, "board-guard-003-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  tr251Board(),
		"events.jsonl": boardGuard003Audit,
	})
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		CommitHash:    strPtr(strings.Repeat("0", 40)),
		EvidenceRunID: "deadbeef",
	})
	if err == nil {
		t.Fatal("real-repo completion with unresolvable provenance must be refused")
	}
	if !strings.Contains(err.Error(), "not a resolvable commit") || !strings.Contains(err.Error(), "0000000000000000000000000000000000000000") {
		t.Fatalf("refusal must include the git resolution error, got: %v", err)
	}

	_, err = b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: "deadbeef",
	})
	if err == nil || !strings.Contains(err.Error(), ".gitreins/history") {
		t.Fatalf("missing evidence run must be refused on a real repo, got: %v", err)
	}
}

func TestIsTestEnvironmentUsesRepositoryRoot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(wd, "..", "..", ".tick-cache")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	repo, err := os.MkdirTemp(scratch, "board-guard-003-classifier-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{name: "repo-root board", dir: boardDir, want: false},
		{name: "board dir directly", dir: repo, want: false},
		{name: "tmp fixture", dir: filepath.Join(t.TempDir(), "board"), want: true},
		{name: "testdata fixture", dir: filepath.Join(repo, "testdata", "board"), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTestEnvironment(tc.dir); got != tc.want {
				t.Fatalf("isTestEnvironment(%q) = %v, want %v", tc.dir, got, tc.want)
			}
		})
	}
}

func TestBoardGuard003RejectsDuplicateCloseOfCompleteRow(t *testing.T) {
	events := boardGuard003Audit + `{"id":2,"timestamp":"2026-09-25 00:00:01.000000","event_type":"task_completed","task_id":"TR-902","actor":"boardctl","detail":null,"tick_number":null}` + "\n"
	b, dir := boardGuard003Fixture(t, "pending", events)
	before, err := os.ReadFile(dir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("TR-902", UpdateSpec{Status: strPtr("complete")})
	if err == nil || !strings.Contains(err.Error(), "task_completed event already exists") {
		t.Fatalf("duplicate close must be refused, got %v", err)
	}
	after, err := os.ReadFile(dir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("duplicate refusal changed tasks.jsonl")
	}
}

func TestBoardGuard003FailedToCompleteWithPriorEventPasses(t *testing.T) {
	events := boardGuard003Audit + `{"id":2,"timestamp":"2026-09-25 00:00:01.000000","event_type":"task_completed","task_id":"TR-901","actor":"boardctl","detail":null,"tick_number":null}` + "\n"
	b, dir := boardGuard003Fixture(t, "failed", events)
	_, err := b.UpdateTask("TR-901", UpdateSpec{
		Status:        strPtr("complete"),
		EvidenceRunID: "synthetic-run",
	})
	if err != nil {
		t.Fatalf("failed->complete is a fresh transition despite prior completion event: %v", err)
	}
	lines, err := ReadJSONLLines(dir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	if err := IterParsed(lines, func(row *Row, _ int, _ []byte) error {
		if row.String("id") == "TR-901" {
			found = row.String("status") == "complete"
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("target row did not transition to complete")
	}
}
