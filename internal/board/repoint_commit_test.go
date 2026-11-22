package board

// DF-DIGEST-068: --repoint-commit, the re-judge re-point door for COMPLETED
// rows. A closed row whose commit_hash no longer matches the tip a Tier-2
// judge would grade can never be re-judged (gitreins task complete demands
// row commit_hash == graded tip); this flag replaces the sha and appends a
// disclosure note, emitting no event. Every fixture is a tmp_path board —
// never a live one; the real-repo refusal arm follows the BOARD-GUARD-003
// .tick-cache pattern (outside TempDir/testdata so the test-environment
// exemption does not soften the resolvability check).

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	repointOldSha    = "1111111111111111111111111111111111111111"
	repointNewSha    = "2222222222222222222222222222222222222222"
	repointAbsentSha = "0000000000000000000000000000000000000000"
)

// repointBoardRows is the header-less shape (plain rows, no board.jsonl, no
// header line) — the digest-board shape the flag was built for.
func repointBoardRows() string {
	return `{"id": "REP-1", "title": "closed row awaiting re-judge", "status": "complete", "priority": "P1", "commit_hash": "` + repointOldSha + `", "foreman_note": "original closeout note", "created_at": "2026-11-22 00:00:00", "updated_at": "2026-11-22 00:00:00"}` + "\n" +
		`{"id": "REP-2", "title": "still open", "status": "pending", "priority": "P2", "created_at": "2026-11-22 00:00:00", "updated_at": "2026-11-22 00:00:00"}` + "\n"
}

var repointEvents = `{"id":1,"timestamp":"2026-11-22 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n" +
	`{"id":2,"timestamp":"2026-11-22 00:00:01.000000","event_type":"task_completed","task_id":"REP-1","actor":"foreman","detail":null,"tick_number":1}` + "\n"

func repointFixture(t *testing.T, tasks, events string) (*Board, string) {
	t.Helper()
	dir := t.TempDir()
	boardDir := dir + "/.coding-hermes/board"
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

func repointReadLine(t *testing.T, path string, lineIdx int) (map[string]any, string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lineIdx >= len(lines) {
		t.Fatalf("%s has %d lines, want at least %d", path, len(lines), lineIdx+1)
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(lines[lineIdx]), &row); err != nil {
		t.Fatalf("line %d of %s is not JSON: %v", lineIdx+1, path, err)
	}
	return row, lines[lineIdx]
}

// AC: a repoint on a completed row succeeds — commit_hash replaced, the
// disclosure note appended (old sha -> new sha, re-judge named, original
// note preserved), the sibling row byte-identical, and NO second
// task_completed event (events.jsonl byte-identical).
func TestRepointCommitOnCompletedRowSucceeds(t *testing.T) {
	b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
	tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	result, err := b.UpdateTaskDetailed("REP-1", UpdateSpec{RepointCommit: repointNewSha})
	if err != nil {
		t.Fatalf("repoint on a completed row must succeed: %v", err)
	}
	joined := strings.Join(result.Changed, ",")
	if !strings.Contains(joined, "commit_hash") || !strings.Contains(joined, "foreman_note") {
		t.Fatalf("changed list must name commit_hash and foreman_note, got: %v", result.Changed)
	}
	row, _ := repointReadLine(t, boardDir+"/tasks.jsonl", 0)
	if row["commit_hash"] != repointNewSha {
		t.Fatalf("commit_hash = %v, want %s", row["commit_hash"], repointNewSha)
	}
	if row["status"] != "complete" {
		t.Fatalf("status must stay complete, got %v", row["status"])
	}
	note, _ := row["foreman_note"].(string)
	if !strings.Contains(note, "original closeout note") {
		t.Fatalf("disclosure note must APPEND (original note preserved), got: %q", note)
	}
	if !strings.Contains(note, "re-judge re-point") {
		t.Fatalf("note must name the re-judge re-point, got: %q", note)
	}
	if !strings.Contains(note, repointOldSha) || !strings.Contains(note, repointNewSha) {
		t.Fatalf("note must record old -> new sha, got: %q", note)
	}
	// Disclosure law: even with commit_hash replaced, the ORIGINAL product
	// sha stays visible in the row (via the note).
	if !strings.Contains(string(tasksBefore), repointOldSha) || !strings.Contains(note, repointOldSha) {
		t.Fatalf("original sha must remain visible in the row")
	}
	_, rep2Line := repointReadLine(t, boardDir+"/tasks.jsonl", 1)
	var before map[string]any
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimRight(string(tasksBefore), "\n"), "\n")[1]), &before); err != nil {
		t.Fatal(err)
	}
	if _, ok := before["commit_hash"]; ok {
		t.Fatalf("sibling row fixture unexpectedly carries commit_hash")
	}
	eventsAfter, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(eventsBefore) != string(eventsAfter) {
		t.Fatalf("a repoint must emit NO event; events.jsonl changed:\n%s\n->\n%s", eventsBefore, eventsAfter)
	}
	if got := strings.Count(string(eventsAfter), `"event_type":"task_completed"`); got != 1 {
		t.Fatalf("exactly one task_completed event expected, found %d", got)
	}
	_ = rep2Line
}

// AC: a repoint on a non-complete row is refused and writes NOTHING.
func TestRepointCommitRefusedOnPendingRow(t *testing.T) {
	b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
	tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("REP-2", UpdateSpec{RepointCommit: repointNewSha})
	if err == nil || !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("repoint on a pending row must be refused with completion REFUSED, got %v", err)
	}
	if !strings.Contains(err.Error(), "REP-2") {
		t.Fatalf("refusal must name the row, got: %v", err)
	}
	tasksAfter, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) {
		t.Fatal("refused repoint changed tasks.jsonl")
	}
	if string(eventsBefore) != string(eventsAfter) {
		t.Fatal("refused repoint appended an event")
	}
}

// AC: the completed-row requirement is BOTH status complete AND a
// task_completed event. A complete-canonical row without the event is not a
// re-judge target (its completion was never recorded as an event).
func TestRepointCommitRefusedWithoutCompletionEvent(t *testing.T) {
	auditOnly := `{"id":1,"timestamp":"2026-11-22 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n"
	b, boardDir := repointFixture(t, repointBoardRows(), auditOnly)
	tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("REP-1", UpdateSpec{RepointCommit: repointNewSha})
	if err == nil || !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("repoint without a completion event must be refused, got %v", err)
	}
	tasksAfter, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) {
		t.Fatal("refused repoint changed tasks.jsonl")
	}
}

// AC: the plain duplicate-completion behavior stays intact on a completed
// row — with --force and with --replace alike, a `--status complete` update
// on an already-completed row applies no completion, leaves commit_hash
// untouched, and never appends a second task_completed event. (Since
// DF-BOARDCTL-53 this collapses to an idempotent no-op with a warning rather
// than a hard refusal; the load-bearing property — no second completion, row
// unchanged — is what this test pins.)
func TestDuplicateCompletionStaysCollapsedWithForceAndReplace(t *testing.T) {
	for name, spec := range map[string]UpdateSpec{
		"force":   {Status: strPtr("complete"), Force: true},
		"replace": {Status: strPtr("complete"), NoteReplace: true},
	} {
		spec := spec
		t.Run(name, func(t *testing.T) {
			b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
			tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			eventsBefore, err := os.ReadFile(boardDir + "/events.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			result, err := b.UpdateTaskDetailed("REP-1", spec)
			if err != nil {
				t.Fatalf("duplicate completion must stay non-fatal, got %v", err)
			}
			if !result.CompletionAlreadyRecorded {
				t.Fatal("duplicate completion must be reported as already recorded")
			}
			if len(result.Changed) != 0 {
				t.Fatalf("duplicate completion must change nothing, got %v", result.Changed)
			}
			row, _ := repointReadLine(t, boardDir+"/tasks.jsonl", 0)
			if row["commit_hash"] != repointOldSha {
				t.Fatalf("commit_hash must stay %s, got %v", repointOldSha, row["commit_hash"])
			}
			tasksAfter, err := os.ReadFile(boardDir + "/tasks.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			eventsAfter, err := os.ReadFile(boardDir + "/events.jsonl")
			if err != nil {
				t.Fatal(err)
			}
			if string(tasksBefore) != string(tasksAfter) {
				t.Fatal("duplicate completion must leave tasks.jsonl byte-identical")
			}
			if string(eventsBefore) != string(eventsAfter) {
				t.Fatal("duplicate completion must not append any event")
			}
		})
	}
}

// AC: a well-formed sha that does not resolve in the row's repository is
// refused (real repo — outside TempDir so the test-env exemption cannot
// soften the check) and nothing is written.
func TestRepointCommitRefusesUnresolvableShaRealRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(wd, "..", "..", ".tick-cache")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		t.Fatal(err)
	}
	repo, err := os.MkdirTemp(scratch, "repoint-commit-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  repointBoardRows(),
		"events.jsonl": repointEvents,
	})
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.email", "t@example.invalid").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	b, err := Resolve(repo)
	if err != nil {
		t.Fatal(err)
	}
	tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("REP-1", UpdateSpec{RepointCommit: repointAbsentSha})
	if err == nil {
		t.Fatal("repoint to an unresolvable sha must be refused in a real repo")
	}
	if !strings.Contains(err.Error(), "completion REFUSED") || !strings.Contains(err.Error(), "not a resolvable commit") || !strings.Contains(err.Error(), repointAbsentSha) {
		t.Fatalf("refusal must speak the completion REFUSED shape and name the sha, got: %v", err)
	}
	tasksAfter, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := os.ReadFile(boardDir + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) || string(eventsBefore) != string(eventsAfter) {
		t.Fatal("refused repoint must write NOTHING")
	}
}

// AC: symbolic and abbreviated references are refused unconditionally (the
// same format gate the completion --commit-hash path applies).
func TestRepointCommitRefusesMalformedSha(t *testing.T) {
	b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
	for _, bad := range []string{"HEAD", "abc1234", repointOldSha[:39]} {
		_, err := b.UpdateTask("REP-1", UpdateSpec{RepointCommit: bad})
		if err == nil || !strings.Contains(err.Error(), "completion REFUSED") || !strings.Contains(err.Error(), "must be a full 40- or 64-hex commit id") {
			t.Fatalf("repoint with %q must be refused with the format gate, got %v", bad, err)
		}
	}
	row, _ := repointReadLine(t, boardDir+"/tasks.jsonl", 0)
	if row["commit_hash"] != repointOldSha {
		t.Fatalf("commit_hash must stay untouched after refusals, got %v", row["commit_hash"])
	}
}

// AC: the repoint never rides an event-bearing or same-field combination.
func TestRepointCommitRefusesStatusAndCommitHashCombination(t *testing.T) {
	b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
	old := repointOldSha
	if _, err := b.UpdateTask("REP-1", UpdateSpec{RepointCommit: repointNewSha, Status: strPtr("complete")}); err == nil || !strings.Contains(err.Error(), "--repoint-commit cannot be combined with --status") {
		t.Fatalf("repoint + --status must be refused, got %v", err)
	}
	if _, err := b.UpdateTask("REP-1", UpdateSpec{RepointCommit: repointNewSha, CommitHash: &old}); err == nil || !strings.Contains(err.Error(), "same field") {
		t.Fatalf("repoint + --commit-hash must be refused, got %v", err)
	}
	row, _ := repointReadLine(t, boardDir+"/tasks.jsonl", 0)
	if row["commit_hash"] != repointOldSha {
		t.Fatalf("commit_hash must stay untouched after refusals, got %v", row["commit_hash"])
	}
}

// AC: same-sha repoint is refused (idempotence guard — no duplicate note
// spam), nothing written.
func TestRepointCommitRefusesSameSha(t *testing.T) {
	b, boardDir := repointFixture(t, repointBoardRows(), repointEvents)
	tasksBefore, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("REP-1", UpdateSpec{RepointCommit: repointOldSha})
	if err == nil || !strings.Contains(err.Error(), "completion REFUSED") || !strings.Contains(err.Error(), "nothing to re-point") {
		t.Fatalf("same-sha repoint must be refused, got %v", err)
	}
	tasksAfter, err := os.ReadFile(boardDir + "/tasks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if string(tasksBefore) != string(tasksAfter) {
		t.Fatal("same-sha repoint must write NOTHING")
	}
}

// AC: the flag works on the header-style board shape too (topology B: the
// header is line 1 of tasks.jsonl) and the header line stays byte-identical —
// a re-point is a task fix, never a drift-pointer write (BT-024).
func TestRepointCommitOnTopologyBHeaderBoard(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"project":"legacy","namespace":"legacy","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n" +
			`{"id":"EXIST-1","title":"closed","status":"complete","priority":"P1","commit_hash":"` + repointOldSha + `"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n" +
			`{"id":2,"timestamp":"2026-09-03 00:00:01.000000","event_type":"task_completed","task_id":"EXIST-1","actor":"foreman","detail":null,"tick_number":1}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Topology != "B" {
		t.Fatalf("topology = %q, want B", b.Topology)
	}
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{RepointCommit: repointNewSha}); err != nil {
		t.Fatalf("repoint on topology B: %v", err)
	}
	lines := strings.SplitN(strings.TrimRight(string(before), "\n"), "\n", 2)
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	afterLines := strings.SplitN(strings.TrimRight(string(after), "\n"), "\n", 2)
	if afterLines[0] != lines[0] {
		t.Fatalf("topology-B header line mutated by repoint:\n got %s\nwant %s", afterLines[0], lines[0])
	}
	var row map[string]any
	if err := json.Unmarshal([]byte(afterLines[1]), &row); err != nil {
		t.Fatal(err)
	}
	if row["commit_hash"] != repointNewSha {
		t.Fatalf("commit_hash = %v, want %s", row["commit_hash"], repointNewSha)
	}
	if !strings.Contains(afterLines[1], repointOldSha) {
		t.Fatalf("disclosure note must keep the original sha visible, got: %s", afterLines[1])
	}
}
