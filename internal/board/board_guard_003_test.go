package board

import (
	"os"
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
