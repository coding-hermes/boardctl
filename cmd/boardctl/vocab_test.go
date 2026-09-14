package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedCLIBoard writes a minimal board under dir and returns the -C target.
func seedCLIBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl":  `{"id":"EXIST-1","title":"Existing","status":"pending","priority":"P2","guard_result":null,"ci_result":null,"depends_on":[]}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"demo","namespace":"demo","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// BT-007 CLI: `update --guard MAYBE` must be rejected (exit 1) and must not
// write MAYBE into tasks.jsonl.
func TestCmdUpdateRejectsGuardMaybe(t *testing.T) {
	dir := seedCLIBoard(t)
	code := run([]string{"-C", dir, "update", "EXIST-1", "--guard", "MAYBE"})
	if code != 1 {
		t.Fatalf("update --guard MAYBE exit code = %d, want 1", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "MAYBE") {
		t.Fatalf("MAYBE written to tasks.jsonl: %s", raw)
	}
	// the in-vocab spelling still works
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--guard", "FAIL"}); code != 0 {
		t.Fatalf("update --guard FAIL exit code = %d, want 0", code)
	}
	raw, err = os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"guard_result":"FAIL"`) {
		t.Fatalf("guard FAIL not stored: %s", raw)
	}
}

// BT-007 CLI: `update --ci BANANA` rejected; GREEN accepted.
func TestCmdUpdateRejectsCIBanana(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--ci", "BANANA"}); code != 1 {
		t.Fatalf("update --ci BANANA exit code = %d, want 1", code)
	}
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--ci", "GREEN"}); code != 0 {
		t.Fatalf("update --ci GREEN exit code = %d, want 0", code)
	}
}

// BT-007 CLI: `create --priority 1` succeeds and stores the normalized "P1".
func TestCmdCreateNormalizesPriorityOne(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "n", "--priority", "1"}); code != 0 {
		t.Fatalf("create --priority 1 exit code = %d, want 0", code)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"priority":"P1"`) {
		t.Fatalf("priority 1 not normalized to P1: %s", raw)
	}
	// a garbage priority is rejected
	if code := run([]string{"-C", dir, "create", "--id", "NEW-2", "--title", "n", "--priority", "banana"}); code != 1 {
		t.Fatalf("create --priority banana exit code = %d, want 1", code)
	}
}

// BT-007 CLI: `create --depends-on GHOST-9` exits 1 and appends nothing.
func TestCmdCreateRejectsGhostDependency(t *testing.T) {
	dir := seedCLIBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "create", "--id", "NEW-1", "--title", "n", "--depends-on", "GHOST-9"}); code != 1 {
		t.Fatalf("create --depends-on GHOST-9 exit code = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected create mutated tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// BT-007 CLI: `event --type nonsense` exits 1; the enumerated types work.
func TestCmdEventRejectsUnknownType(t *testing.T) {
	dir := seedCLIBoard(t)
	if code := run([]string{"-C", dir, "event", "--type", "nonsense"}); code != 1 {
		t.Fatalf("event --type nonsense exit code = %d, want 1", code)
	}
	if code := run([]string{"-C", dir, "event", "--type", "tick", "--tick", "2"}); code != 0 {
		t.Fatalf("event --type tick exit code = %d, want 0", code)
	}
}

// BT-007 CLI: `header --set-ticks-total -5` exits 1 and leaves board.jsonl
// untouched.
func TestCmdHeaderRejectsNegativeTicks(t *testing.T) {
	dir := seedCLIBoard(t)
	hdrPath := filepath.Join(dir, ".coding-hermes", "board", "board.jsonl")
	before, err := os.ReadFile(hdrPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "header", "--set-ticks-total", "-5"}); code != 1 {
		t.Fatalf("header --set-ticks-total -5 exit code = %d, want 1", code)
	}
	after, err := os.ReadFile(hdrPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("rejected header update mutated board.jsonl:\nbefore %s\nafter  %s", before, after)
	}
}

// BT-007 CLI: validate flags the junk row (warn) while staying exit 0, and
// the exact dogfood scenario (free-form guard) is visible in the report.
func TestCmdValidateFlagsJunkRow(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	junk := `{"id":"JUNK-1","title":"junk","status":"pending","priority":"P2","guard_result":"MAYBE","depends_on":["GHOST-9"]}` + "\n"
	f, err := os.OpenFile(tasksPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(junk); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "validate"}); code != 0 {
			t.Fatalf("validate exit code = %d, want 0 (junk rows warn, not error)", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "MAYBE") {
		t.Fatalf("validate report does not mention the free-form guard value:\n%s", got)
	}
	if !strings.Contains(got, "GHOST-9") {
		t.Fatalf("validate report does not mention the dangling dependency:\n%s", got)
	}
	if !strings.Contains(got, "RESULT: OK") {
		t.Fatalf("validate should stay OK (warnings only):\n%s", got)
	}
}

// BT-025 CLI: validate on a board using todo/done/open exits 0 with alias
// warnings (zero errors); `update --status todo` still exits 1 and writes
// nothing; `update <id> --normalize` canonicalizes the row reporting each
// rewrite; a second --normalize is an already-canonical no-op.
func TestCmdUpdateNormalizeAliasWorkflow(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	f, err := os.OpenFile(tasksPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"ALIAS-1","title":"alias","status":"todo","priority":"P2","guard_result":"pass","ci_result":null}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// validate: alias rows warn, zero errors, exit 0
	got, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "validate"}); code != 0 {
			t.Fatalf("validate exit code = %d, want 0 (alias rows warn, not error)", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, `"todo" is a read alias for "pending"`) || !strings.Contains(got, "--normalize") {
		t.Fatalf("validate missing todo alias warning:\n%s", got)
	}
	if !strings.Contains(got, "RESULT: OK") {
		t.Fatalf("validate should stay OK on alias rows:\n%s", got)
	}

	// write enforcement unchanged: an alias spelling is still rejected and
	// writes nothing
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "ALIAS-1", "--status", "todo"}); code != 1 {
		t.Fatalf("update --status todo exit code = %d, want 1 (writes reject aliases)", code)
	}
	mid, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(mid) {
		t.Fatalf("rejected alias write mutated tasks.jsonl:\nbefore %s\nafter  %s", before, mid)
	}

	// normalize: exactly the dirty fields, reported old -> new
	got, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "update", "ALIAS-1", "--normalize"}); code != 0 {
			t.Fatalf("update --normalize exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, `status "todo" -> "pending"`) || !strings.Contains(got, `guard_result "pass" -> "PASS"`) {
		t.Fatalf("normalize output missing field reports:\n%s", got)
	}
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"status":"pending"`) || !strings.Contains(string(raw), `"guard_result":"PASS"`) {
		t.Fatalf("normalize did not canonicalize the row: %s", raw)
	}
	if strings.Contains(string(raw), `"todo"`) {
		t.Fatalf("alias spelling still on disk: %s", raw)
	}

	// idempotent: second normalize is an already-canonical no-op
	got, err = captureStdout(func() {
		if code := run([]string{"-C", dir, "update", "ALIAS-1", "--normalize"}); code != 0 {
			t.Fatalf("second normalize exit code = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if !strings.Contains(got, "already canonical") {
		t.Fatalf("second normalize should report already canonical:\n%s", got)
	}
	raw2, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(raw2) {
		t.Fatalf("second normalize rewrote the file:\nbefore %s\nafter  %s", raw, raw2)
	}
}

// BT-025 CLI: --normalize refuses an unknown status (exit 1, nothing
// written) and rejects combination with other change flags.
func TestCmdUpdateNormalizeRefusesUnknownAndCombo(t *testing.T) {
	dir := seedCLIBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	f, err := os.OpenFile(tasksPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"OLD-1","title":"retired","status":"retired","priority":"P2"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-C", dir, "update", "OLD-1", "--normalize"}); code != 1 {
		t.Fatalf("normalize of retired exit code = %d, want 1", code)
	}
	after, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("refused normalize mutated tasks.jsonl:\nbefore %s\nafter  %s", before, after)
	}
	// combination with an explicit change flag is a usage error
	if code := run([]string{"-C", dir, "update", "EXIST-1", "--normalize", "--status", "complete"}); code != 1 {
		t.Fatalf("normalize --status combo exit code = %d, want 1", code)
	}
}
