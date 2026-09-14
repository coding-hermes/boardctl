package board

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- BT-026: error UX on partial and pretty-printed boards ----------

// writeTasksOnly seeds dir with ONLY tasks.jsonl (no events.jsonl) — a
// partial/legacy board.
func writeTasksOnly(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "tasks.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertPartialBoardErr checks the BT-026 partial-board error contract: the
// ErrBoardNotFound sentinel still matches (exit-2 class at the CLI), the
// detected tasks.jsonl path is identified, and the missing events.jsonl file
// is explicitly named.
func assertPartialBoardErr(t *testing.T, err error, wantTasksPath string) {
	t.Helper()
	if err == nil {
		t.Fatal("Resolve succeeded on a partial board, want an error")
	}
	if !errors.Is(err, ErrBoardNotFound) {
		t.Fatalf("err = %v, want errors.Is(ErrBoardNotFound) (the CLI exit-2 class)", err)
	}
	if !strings.Contains(err.Error(), wantTasksPath) {
		t.Fatalf("err = %v, want it to identify the detected tasks.jsonl path %q", err, wantTasksPath)
	}
	if !strings.Contains(err.Error(), "events.jsonl is missing") {
		t.Fatalf("err = %v, want it to explicitly name the missing events.jsonl file", err)
	}
}

// TestResolveTasksOnlyBoardNamesMissingEvents: a tasks.jsonl-only board must
// fail as a PARTIAL board naming the missing events.jsonl — at all three
// supported -C depths (repo root, .coding-hermes, board dir).
func TestResolveTasksOnlyBoardNamesMissingEvents(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeTasksOnly(t, boardDir)

	targets := []string{repo, filepath.Join(repo, ".coding-hermes"), boardDir}
	for _, tgt := range targets {
		_, err := Resolve(tgt)
		if err == nil {
			t.Errorf("Resolve(-C %s) succeeded on a tasks-only board, want an error", tgt)
			continue
		}
		if !errors.Is(err, ErrBoardNotFound) {
			t.Errorf("Resolve(-C %s) err = %v, want errors.Is(ErrBoardNotFound)", tgt, err)
		}
		assertPartialBoardErr(t, err, filepath.Join(boardDir, "tasks.jsonl"))
	}
}

// TestResolveTasksOnlyAtRepoRoot: the partial candidate must win over the
// generic not-found even when it is the repo root itself.
func TestResolveTasksOnlyAtRepoRoot(t *testing.T) {
	repo := t.TempDir()
	writeTasksOnly(t, repo)
	_, err := Resolve(repo)
	assertPartialBoardErr(t, err, filepath.Join(repo, "tasks.jsonl"))
}

// TestResolveEventsOnlyBoardNamesMissingTasks: the symmetric direction —
// events.jsonl without tasks.jsonl is also a partial board, naming tasks.jsonl.
func TestResolveEventsOnlyBoardNamesMissingTasks(t *testing.T) {
	repo := t.TempDir()
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(repo)
	if err == nil {
		t.Fatal("Resolve succeeded on an events-only board, want an error")
	}
	if !errors.Is(err, ErrBoardNotFound) {
		t.Fatalf("err = %v, want errors.Is(ErrBoardNotFound)", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(boardDir, "events.jsonl")) {
		t.Fatalf("err = %v, want it to identify the detected events.jsonl path", err)
	}
	if !strings.Contains(err.Error(), "tasks.jsonl is missing") {
		t.Fatalf("err = %v, want it to explicitly name the missing tasks.jsonl file", err)
	}
}

// A full board must still resolve even when a shallower candidate directory
// holds a partial board (the full pair always wins over the partial).
func TestResolveFullBoardBeatsPartialCandidate(t *testing.T) {
	repo := t.TempDir()
	writeTasksOnly(t, repo) // partial at the repo root
	boardDir := filepath.Join(repo, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"id":"E-1","title":"Existing","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	b, err := Resolve(repo)
	if err != nil {
		t.Fatalf("Resolve with a full nested board: %v", err)
	}
	if b.Dir != boardDir {
		t.Fatalf("Resolve found %q, want the full board %q", b.Dir, boardDir)
	}
}

// The generic not-found error must stay generic: no phantom "missing" hints
// for directories with no board files at all.
func TestResolveGenericNotFoundHasNoPartialHint(t *testing.T) {
	for _, dir := range []string{t.TempDir(), filepath.Join(t.TempDir(), "unrelated")} {
		if strings.Contains(dir, "unrelated") {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := Resolve(dir)
		if !errors.Is(err, ErrBoardNotFound) {
			t.Fatalf("Resolve(%s) err = %v, want ErrBoardNotFound", dir, err)
		}
		if strings.Contains(err.Error(), " is missing") {
			t.Fatalf("generic not-found for %s grew a partial hint: %v", dir, err)
		}
	}
}

// ---------- BT-026: pretty-printed / non-JSONL diagnosis ----------

// TestReadAllRowsPrettyPrintedDiagnosed: a tasks.jsonl holding ONE valid JSON
// object pretty-printed across multiple lines must fail naming the file, the
// failing 1-based line, and the valid-JSON-but-not-JSONL diagnosis with an
// actionable compaction hint.
func TestReadAllRowsPrettyPrintedDiagnosed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks.jsonl")
	pretty := "{\n  \"id\": \"E-1\",\n  \"title\": \"Existing\",\n  \"status\": \"pending\"\n}\n"
	if err := os.WriteFile(p, []byte(pretty), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadAllRows(p)
	if err == nil {
		t.Fatal("ReadAllRows succeeded on a pretty-printed tasks.jsonl, want an error")
	}
	for _, want := range []string{
		p,                                   // exact file
		"line 1",                            // exact 1-based failing line
		"valid JSON",                        // the source may be valid JSON...
		"not line-loadable JSONL",           // ...but is not JSONL
		"one complete JSON object per line", // the JSONL contract
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %v, want substring %q", err, want)
		}
	}
}

// TestReadAllRowsCorruptJSONKeepsPlainError: genuinely corrupt JSON must NOT
// be mislabeled as valid-but-pretty JSON — it keeps the plain per-line error
// (with file + line context).
func TestReadAllRowsCorruptJSONKeepsPlainError(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "tasks.jsonl")
	// invalid as a single line AND as a whole document
	if err := os.WriteFile(p, []byte("{\"id\": \"E-1\",\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := ReadAllRows(p)
	if err == nil {
		t.Fatal("ReadAllRows succeeded on corrupt JSON, want an error")
	}
	if !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("err = %v, want file + line context", err)
	}
	if strings.Contains(err.Error(), "valid JSON") {
		t.Fatalf("corrupt JSON mislabeled as valid: %v", err)
	}
}

// TestIterParsedPrettyPrintedDiagnosisWithoutPath: the diagnosis is added at
// the IterParsed seam, so shared read/write paths (write.go, validate.go)
// inherit it; the caller still supplies the path wrapper.
func TestIterParsedPrettyPrintedDiagnosisWithoutPath(t *testing.T) {
	lines, err := ReadJSONLLines("no-such-file")
	if err == nil {
		t.Fatal("expected read error")
	}
	_ = lines
	pretty := "{\n  \"id\": 1\n}"
	raw := strings.Split(pretty, "\n")
	rawBytes := make([][]byte, len(raw))
	for i, l := range raw {
		rawBytes[i] = []byte(l)
	}
	err = IterParsed(rawBytes, func(*Row, int, []byte) error { return nil })
	if err == nil {
		t.Fatal("IterParsed succeeded on pretty-printed JSON, want an error")
	}
	if !strings.Contains(err.Error(), "line 1") || !strings.Contains(err.Error(), "not line-loadable JSONL") {
		t.Fatalf("IterParsed err = %v, want the pretty-print diagnosis with line context", err)
	}
}
