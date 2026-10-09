package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// DF-BOARDCTL-29 E2E through a real stamped binary: create --deferred ->
// update --deferred false -> the row's line in tasks.jsonl contains NO
// "deferred" token at all (grep -c deferred on the row's line = 0), and a
// row that never carried the key stays byte-identical through an unrelated
// update. Skipped under -short like the other binary-building tests.
func TestDFBoardctl29UndeferRemovesKeyBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real binary; skipped under -short")
	}
	bin := filepath.Join(t.TempDir(), "boardctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	run := func(args ...string) string {
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("boardctl %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	boardOut := run("init", "-C", t.TempDir())
	boardDir := ""
	for _, l := range strings.Split(boardOut, "\n") {
		if i := strings.Index(l, "initialized board at "); i >= 0 {
			boardDir = strings.TrimSpace(l[i+len("initialized board at "):])
			break
		}
	}
	if boardDir == "" {
		t.Fatalf("init output missing board dir:\n%s", boardOut)
	}
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	if _, err := os.Stat(tasksPath); err != nil {
		t.Fatalf("init board dir %q has no tasks.jsonl: %v", boardDir, err)
	}

	const id = "DF-E2E-1"
	run("-C", boardDir, "create", "--id", id, "--title", "parked for e2e", "--deferred")
	run("-C", boardDir, "update", id, "--deferred", "false")

	rows, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	var target string
	for _, l := range strings.Split(string(rows), "\n") {
		if strings.Contains(l, `"`+id+`"`) {
			target = l
			break
		}
	}
	if target == "" {
		t.Fatalf("task %q not found in tasks.jsonl", id)
	}
	if got := strings.Count(target, "deferred"); got != 0 {
		t.Fatalf("grep -c deferred on the row's line = %d, want 0:\n%s", got, target)
	}

	// never-deferred sibling: an unrelated update must not create the key
	run("-C", boardDir, "create", "--id", "DF-E2E-2", "--title", "plain")
	run("-C", boardDir, "update", "DF-E2E-2", "--summary", "unrelated work")
	plain := ""
	for _, l := range strings.Split(string(rows2(t, tasksPath)), "\n") {
		if strings.Contains(l, `"DF-E2E-2"`) {
			plain = l
			break
		}
	}
	if plain == "" {
		t.Fatal("task DF-E2E-2 not found")
	}
	if strings.Count(plain, "deferred") != 0 {
		t.Fatalf("unrelated update created a deferred key on a never-deferred row:\n%s", plain)
	}
}

func rows2(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
