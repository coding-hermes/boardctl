package board

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeStatusesDryRunPreservesBytesAndRefusesUnknown(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	before := []byte("{\"id\":\"A\",\"status\":\"done\",\"title\":\"alias\"}\n" +
		"{\"id\":\"B\",\"status\":\"parked\",\"title\":\"unknown\"}\n" +
		"{\"id\":\"C\",\"title\":\"missing\"}\n")
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), before, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.NormalizeStatuses(false)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.DryRun || rep.Applied || rep.AliasCount != 1 || rep.UnknownCount != 1 || rep.MissingCount != 1 {
		t.Fatalf("unexpected dry-run report: %+v", rep)
	}
	after, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("dry-run changed tasks.jsonl:\nbefore %s\nafter %s", before, after)
	}
}

func TestNormalizeStatusesApplyIsAtomicAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	before := []byte("{\"id\":\"A\", \"status\": \"done\", \"title\": \"alias\"}\n" +
		"{\"id\":\"B\",\"status\":\"parked\",\"title\":\"unknown\"}\n")
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), before, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.NormalizeStatuses(true)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Applied || rep.Changed != 1 || len(rep.Refused) != 1 {
		t.Fatalf("unexpected apply report: %+v", rep)
	}
	after, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte(`"status": "complete"`)) || !bytes.Contains(after, []byte(`"status":"parked"`)) {
		t.Fatalf("apply did not canonicalize only the alias: %s", after)
	}
	if bytes.Contains(after, []byte(`"id":"B","status":"complete"`)) {
		t.Fatal("unknown status was guessed")
	}
	second, err := b.NormalizeStatuses(true)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed != 0 || second.Applied {
		t.Fatalf("second apply was not an idempotent no-op: %+v", second)
	}
	final, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, final) {
		t.Fatal("second apply changed canonical bytes")
	}
}
