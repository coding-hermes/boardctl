package board

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// BT-056: the key canon. Before this existed, validate's only shape test was
// "the row parsed", so a row could carry any spelling at all.

func TestIsSanctionedTaskKey(t *testing.T) {
	allow := []string{"id", "title", "status", "priority", // required
		"detail", "source", "ts", "acceptance_criteria", "model_used", // declared extras
		"reasoning", "complexity", "guard_result", "review_notes", // create schema
		"guard_note", "ci_note", "stale_note", "complexity_note"} // _note family
	for _, k := range allow {
		if !IsSanctionedTaskKey(k) {
			t.Errorf("key %q should be sanctioned", k)
		}
	}
	deny := []string{"why", "acceptance", "review_otes", "pri", "cpx", "_note",
		"Not a key at all", "why_"}
	for _, k := range deny {
		if IsSanctionedTaskKey(k) {
			t.Errorf("key %q should NOT be sanctioned (it is drift)", k)
		}
	}
}

// A clean board must REPORT zero drift explicitly, not merely stay silent —
// the summary line is what a reviewer quotes as evidence.
func TestValidateCleanBoardReportsZeroDrift(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"OK-1","title":"clean","status":"pending","priority":"P2","detail":"d","capture_note":"n"}` + "\n" +
			`{"id":"OK-2","title":"clean too","status":"complete","priority":"P3"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Keys.Rows != 2 || rep.Keys.Clean != 2 {
		t.Fatalf("census wrong: %+v", rep.Keys)
	}
	summary := rep.Keys.Summary()
	if !strings.Contains(summary, "2/2") || !strings.Contains(summary, "0 drift") {
		t.Fatalf("clean summary must quote 2/2 and 0 drift, got %q", summary)
	}
	if !strings.Contains(rep.RenderText(), "key uniformity:") {
		t.Fatalf("summary missing from the rendered report:\n%s", rep.RenderText())
	}
}

// Drift is reported per row, names the offending keys, and distinguishes the
// repairable kind (a known alias) from the kind that needs a human.
func TestValidateReportsKeyDrift(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"ALIAS-1","title":"one-key dialect","status":"pending","priority":"P2","why":"prose"}` + "\n" +
			`{"id":"WEIRD-1","title":"unfixable","status":"pending","priority":"P2","mystery_field":"x"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Keys.DriftRows() != 2 {
		t.Fatalf("expected 2 drift rows, got %d (%+v)", rep.Keys.DriftRows(), rep.Keys)
	}
	// the alias row is fixable, the mystery-key row is not
	if rep.Keys.Fixable != 1 {
		t.Fatalf("expected exactly 1 fixable row (the alias one), got %d", rep.Keys.Fixable)
	}
	aliasFinding := findWarn(rep, "ALIAS-1")
	if !strings.Contains(aliasFinding, "why") || !strings.Contains(aliasFinding, "--normalize") {
		t.Fatalf("alias row finding must name the key and the fix path, got %q", aliasFinding)
	}
	weirdFinding := findWarn(rep, "WEIRD-1")
	if !strings.Contains(weirdFinding, "mystery_field") {
		t.Fatalf("unfixable row finding must name the key, got %q", weirdFinding)
	}
	if strings.Contains(weirdFinding, "--normalize") {
		t.Fatalf("a key with no canonical spelling must NOT advertise --normalize: %q", weirdFinding)
	}
	// shape drift warns — a legacy board must still validate OK
	if rep.HasErrors() {
		t.Fatalf("key drift must warn by default, not error: %+v", rep.Findings)
	}
}

// RenameKey must move the key in place: same position, same value bytes.
func TestRenameKeyPreservesPositionAndValue(t *testing.T) {
	row, err := ParseRow([]byte(`{"id":"A-1","title":"t","why":"because","status":"pending","priority":"P2"}`))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]string{}, row.Keys...)
	beforeVal := string(row.Get("why"))
	if err := row.RenameKey("why", "reasoning"); err != nil {
		t.Fatal(err)
	}
	if len(row.Keys) != len(before) {
		t.Fatalf("key count changed: %v -> %v", before, row.Keys)
	}
	for i := range before {
		want := before[i]
		if want == "why" {
			want = "reasoning"
		}
		if row.Keys[i] != want {
			t.Fatalf("position %d: got %q want %q (order must be preserved)", i, row.Keys[i], want)
		}
	}
	if row.Keys[2] != "reasoning" {
		t.Fatalf("the renamed key must stay at index 2, got %v", row.Keys)
	}
	if got := string(row.Get("reasoning")); got != beforeVal {
		t.Fatalf("value changed on rename: %q -> %q", beforeVal, got)
	}
	if row.Has("why") {
		t.Fatalf("old key still present after rename")
	}
	// the marshalled line differs ONLY in the key spelling
	out := string(row.Marshal(DetectStyle([]byte(`{"id":"A-1","title":"t","why":"because","status":"pending","priority":"P2"}`))))
	if out != `{"id":"A-1","title":"t","reasoning":"because","status":"pending","priority":"P2"}` {
		t.Fatalf("unexpected rewrite: %s", out)
	}
}

// A rename that would destroy one of two values must refuse, not pick.
func TestRenameKeyRefusals(t *testing.T) {
	// canonical key already present
	row, err := ParseRow([]byte(`{"id":"A-1","why":"one","reasoning":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := row.RenameKey("why", "reasoning"); err == nil {
		t.Fatalf("must refuse when the canonical key is already present")
	} else if !strings.Contains(err.Error(), "already present") {
		t.Fatalf("unhelpful refusal: %v", err)
	}
	// source key duplicated. ParseRow already refuses duplicate keys, so this
	// contract can only be exercised by building the Row directly — the guard
	// stays because RenameKey is a public method on a mutable struct.
	dup := &Row{Keys: []string{"id", "why", "why"}, Vals: map[string]json.RawMessage{
		"id":  json.RawMessage(`"A-1"`),
		"why": json.RawMessage(`"two"`),
	}}
	if err := dup.RenameKey("why", "reasoning"); err == nil {
		t.Fatalf("must refuse an ambiguous duplicate source key")
	} else if !strings.Contains(err.Error(), "appears 2 times") {
		t.Fatalf("unhelpful refusal: %v", err)
	}
	// absent key
	ok, err := ParseRow([]byte(`{"id":"A-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := ok.RenameKey("why", "reasoning"); err == nil {
		t.Fatalf("must refuse a key that is not on the row")
	}
}

// The end-to-end repair: --normalize renames the alias, rewrites ONLY that row,
// and is idempotent on the second run.
func TestNormalizeTaskRenamesKeyAlias(t *testing.T) {
	dir := t.TempDir()
	other := `{"id":"OTHER-1","title":"untouched","status":"pending","priority":"P2","why":"also prose"}` + "\n"
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": other +
			`{"id":"ALIAS-1","title":"drift","status":"pending","priority":"P2","why":"the reason"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := b.NormalizeTask("ALIAS-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !changes[0].KeyRename || changes[0].Old != "why" || changes[0].New != "reasoning" {
		t.Fatalf("unexpected changes: %+v", changes)
	}
	raw := mustRead(t, b.tasksPath)
	if !strings.Contains(string(raw), `"reasoning":"the reason"`) {
		t.Fatalf("alias not renamed:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"why":"also prose"`) {
		t.Fatalf("an UNTOUCHED row must round-trip byte-identical:\n%s", raw)
	}
	// second run is a byte-identical no-op
	before := append([]byte{}, raw...)
	again, err := b.NormalizeTask("ALIAS-1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second normalize must be a no-op, got %+v", again)
	}
	if !bytes.Equal(before, mustRead(t, b.tasksPath)) {
		t.Fatalf("second normalize rewrote the file")
	}
	// and the board is now clean of that drift
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rep.Keys.Keys["why"]; !ok {
		t.Fatalf("the untouched OTHER-1 row should still carry why: %+v", rep.Keys)
	}
	if rep.Keys.Keys["why"] != 1 {
		t.Fatalf("expected exactly 1 remaining why row, got %+v", rep.Keys)
	}
}
