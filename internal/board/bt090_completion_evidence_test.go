package board

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func bt090Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	if len(args) > 0 && args[0] == "commit" {
		return bt090Git(t, dir, "rev-parse", "HEAD")
	}
	return strings.TrimSpace(string(out))
}

func bt090Repo(t *testing.T) (*Board, string) {
	t.Helper()
	root := t.TempDir()
	boardDir := filepath.Join(root, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"id":"BT-090-TARGET","title":"target","status":"pending","priority":"P2"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-11-10T00:00:00Z","event_type":"audit","task_id":null,"actor":"test","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"bt090","namespace":"test","version":1,"ticks_total":1,"ticks_idle":0}` + "\n",
	})
	bt090Git(t, root, "init", "-q")
	bt090Git(t, root, "config", "user.email", "test@example.invalid")
	bt090Git(t, root, "config", "user.name", "BT-090 test")
	bt090Git(t, root, "add", ".")
	bt090Git(t, root, "commit", "-qm", "fixture")
	if err := os.MkdirAll(filepath.Join(root, ".gitreins", "history", "synthetic-run"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	return b, root
}

func bt090Complete(t *testing.T, b *Board, hash string, recordDebt bool) error {
	t.Helper()
	status := "complete"
	_, err := b.UpdateTask("BT-090-TARGET", UpdateSpec{
		Status:        &status,
		CommitHash:    &hash,
		EvidenceRunID: "synthetic-run",
		RecordDebt:    recordDebt,
	})
	return err
}

func TestBT090CompletionEvidenceGateTable(t *testing.T) {
	tests := []struct {
		name      string
		prepare   func(*testing.T, string) string
		wantError string
	}{
		{
			name: "unknown sha",
			prepare: func(t *testing.T, root string) string {
				return strings.Repeat("0", 40)
			},
			wantError: "not a resolvable commit",
		},
		{
			name: "empty commit",
			prepare: func(t *testing.T, root string) string {
				return bt090Git(t, root, "commit", "--allow-empty", "-qm", "empty")
			},
			wantError: "zero product files",
		},
		{
			name: "bookkeeping-only commit",
			prepare: func(t *testing.T, root string) string {
				path := filepath.Join(root, ".coding-hermes", "note.json")
				if err := os.WriteFile(path, []byte(`{"note":true}`+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				bt090Git(t, root, "add", path)
				return bt090Git(t, root, "commit", "-qm", "bookkeeping")
			},
			wantError: "zero product files",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, root := bt090Repo(t)
			hash := tc.prepare(t, root)
			if _, err := b.RecordCompletionBaseline(); err != nil {
				t.Fatal(err)
			}
			err := bt090Complete(t, b, hash, false)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("completion error = %v, want %q", err, tc.wantError)
			}
			if _, err := b.TaskRows(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBT090WarnBaselineAndRecordedDebt(t *testing.T) {
	b, root := bt090Repo(t)
	bookkeeping := filepath.Join(root, ".coding-hermes", "note.json")
	if err := os.WriteFile(bookkeeping, []byte(`{"note":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt090Git(t, root, "add", bookkeeping)
	debtHash := bt090Git(t, root, "commit", "-qm", "bookkeeping")
	if err := bt090Complete(t, b, debtHash, false); err != nil {
		t.Fatalf("no baseline must WARN and allow debt: %v", err)
	}
	if got := len(b.CompletionWarnings()); got != 1 {
		t.Fatalf("warning count = %d, want 1", got)
	}

	b, root = bt090Repo(t)
	if err := os.WriteFile(filepath.Join(root, "product.go"), []byte("package product\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt090Git(t, root, "add", "product.go")
	productHash := bt090Git(t, root, "commit", "-qm", "product")
	status := "complete"
	if _, err := b.UpdateTask("BT-090-TARGET", UpdateSpec{Status: &status, CommitHash: &productHash, EvidenceRunID: "synthetic-run"}); err != nil {
		t.Fatalf("product commit must be accepted: %v", err)
	}

	b, root = bt090Repo(t)
	bookkeeping = filepath.Join(root, ".coding-hermes", "note.json")
	if err := os.WriteFile(bookkeeping, []byte(`{"note":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt090Git(t, root, "add", bookkeeping)
	debtHash = bt090Git(t, root, "commit", "-qm", "bookkeeping")
	if _, err := b.RecordCompletionBaseline(); err != nil {
		t.Fatalf("record baseline: %v", err)
	}
	// Baseline records existing complete rows only; a new pending row with the
	// debt hash is new debt and fails until explicitly recorded.
	if err := bt090Complete(t, b, debtHash, false); err == nil || !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("baseline-backed new debt must fail, got %v", err)
	}
	if err := bt090Complete(t, b, debtHash, true); err != nil {
		t.Fatalf("--record-debt must allow and count debt: %v", err)
	}
}
