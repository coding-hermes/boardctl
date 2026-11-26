package board

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// INT-CI-746: a foreign reader-schema evidence-baseline.json (owned by
// scripts/board-evidence-audit.py) must never be decoded as an empty
// completion baseline and rewritten by --record-debt. Every fixture reuses
// the bt090 repo harness so the full gate path (git resolution included)
// runs.

const intCI746ForeignBaseline = `{"rows": [
  {"id": "DF-085", "status": "complete", "signatures": 3},
  {"id": "DF-072", "status": "complete", "signatures": 1}
], "signatures": [
  {"task_id": "DF-085", "sha256": "abc"},
  {"task_id": "DF-072", "sha256": "def"}
]}
`

func intCI746DebtHash(t *testing.T, root string) string {
	t.Helper()
	note := filepath.Join(root, ".coding-hermes", "note.json")
	if err := os.WriteFile(note, []byte(`{"note":true}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bt090Git(t, root, "add", note)
	return bt090Git(t, root, "commit", "-qm", "bookkeeping")
}

// (i) foreign reader-schema file + --record-debt -> refusal, bytes UNCHANGED,
// row NOT mutated to complete.
func TestINTCI746ForeignBaselineRecordDebtRefused(t *testing.T) {
	b, root := bt090Repo(t)
	path := filepath.Join(root, ".coding-hermes", "board", "evidence-baseline.json")
	if err := os.WriteFile(path, []byte(intCI746ForeignBaseline), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := intCI746DebtHash(t, root)
	if err := bt090Complete(t, b, hash, true); err == nil {
		t.Fatal("record-debt against a foreign reader-schema baseline must be refused")
	} else if !strings.Contains(err.Error(), "reader schema (rows/signatures)") {
		t.Fatalf("refusal must name the foreign reader schema, got: %v", err)
	} else if !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("refusal must surface as a completion REFUSED, got: %v", err)
	} else if !errors.Is(err, ErrForeignBaselineSchema) {
		t.Fatalf("error must wrap ErrForeignBaselineSchema, got: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("a foreign-schema baseline must be left byte-for-byte UNCHANGED")
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.String("id") == "BT-090-TARGET" && row.String("status") == "complete" {
			t.Fatal("the refused completion must NOT mutate the row to complete")
		}
	}
}

// (ii) valid own-schema baseline + --record-debt -> entry appended and the
// pre-existing debt entries preserved.
func TestINTCI746ValidBaselineRecordDebtAppendsAndPreserves(t *testing.T) {
	b, root := bt090Repo(t)
	path := filepath.Join(root, ".coding-hermes", "board", "evidence-baseline.json")
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "debt": [
    {"commit_hash": "`+strings.Repeat("1", 40)+`", "task_ids": ["DF-085"], "reason": "historic zero-product commit"}
  ]
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := intCI746DebtHash(t, root)
	if err := bt090Complete(t, b, hash, true); err != nil {
		t.Fatalf("--record-debt against a VALID baseline must succeed: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var baseline completionEvidenceBaseline
	if err := json.Unmarshal(content, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Version != 1 {
		t.Fatalf("version = %d, want 1 (pre-existing document preserved)", baseline.Version)
	}
	if len(baseline.Debt) != 2 {
		t.Fatalf("debt entries = %d, want 2 (historic + newly recorded)", len(baseline.Debt))
	}
	if baseline.Debt[0].TaskIDs == nil || len(baseline.Debt[0].TaskIDs) != 1 || baseline.Debt[0].TaskIDs[0] != "DF-085" {
		t.Fatalf("pre-existing debt entry must be preserved, got %+v", baseline.Debt[0])
	}
	if baseline.Debt[1].CommitHash != hash {
		t.Fatalf("new debt entry must carry the recorded hash %q, got %+v", hash, baseline.Debt[1])
	}
}

// (iii) no baseline file + --record-debt -> succeeds (recorded=false path).
func TestINTCI746NoBaselineRecordDebtSucceeds(t *testing.T) {
	b, root := bt090Repo(t)
	hash := intCI746DebtHash(t, root)
	if err := bt090Complete(t, b, hash, true); err != nil {
		t.Fatalf("record-debt with NO baseline file must succeed (warn-and-permit): %v", err)
	}
	if got := len(b.CompletionWarnings()); got != 1 {
		t.Fatalf("warning count = %d, want 1", got)
	}
}

// (iv) valid baseline WITHOUT --record-debt and the hash not recorded ->
// still refused (existing behavior).
func TestINTCI746ValidBaselineWithoutRecordDebtStillRefused(t *testing.T) {
	b, root := bt090Repo(t)
	path := filepath.Join(root, ".coding-hermes", "board", "evidence-baseline.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "debt": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := intCI746DebtHash(t, root)
	err := bt090Complete(t, b, hash, false)
	if err == nil || !strings.Contains(err.Error(), "completion REFUSED") {
		t.Fatalf("new debt against a recorded baseline without --record-debt must be refused, got: %v", err)
	}
}

// The foreign-schema refusal must fire even WITHOUT --record-debt (the gate
// reads the baseline on every debt path), and the boardctl evidence command
// (RecordCompletionBaseline) must refuse to overwrite a foreign-schema file.
func TestINTCI746ForeignBaselineOtherPathsRefused(t *testing.T) {
	b, root := bt090Repo(t)
	path := filepath.Join(root, ".coding-hermes", "board", "evidence-baseline.json")
	if err := os.WriteFile(path, []byte(intCI746ForeignBaseline), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := intCI746DebtHash(t, root)
	if err := bt090Complete(t, b, hash, false); err == nil || !errors.Is(err, ErrForeignBaselineSchema) {
		t.Fatalf("foreign baseline must be refused even without --record-debt, got: %v", err)
	}
	if _, err := b.RecordCompletionBaseline(); err == nil || !errors.Is(err, ErrForeignBaselineSchema) {
		t.Fatalf("RecordCompletionBaseline must refuse a foreign-schema file, got: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("foreign-schema baseline must remain byte-for-byte UNCHANGED")
	}
}

// A file that is not a JSON object at all is NOT silently accepted: the
// strict decode must still report it as invalid.
func TestINTCI746NonObjectBaselineStillInvalid(t *testing.T) {
	b, root := bt090Repo(t)
	path := filepath.Join(root, ".coding-hermes", "board", "evidence-baseline.json")
	if err := os.WriteFile(path, []byte(`{"version": 1, "debt": [`), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := intCI746DebtHash(t, root)
	err := bt090Complete(t, b, hash, true)
	if err == nil || !strings.Contains(err.Error(), "invalid completion evidence baseline") {
		t.Fatalf("malformed baseline must still be refused as invalid, got: %v", err)
	}
}
