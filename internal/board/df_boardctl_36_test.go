package board

// DF-BOARDCTL-36: NormalizeBoard tests. The fixture mirrors the MEASURED
// warpfs drift pattern (2026-10-13 census of /home/kara/warpfs, read-only:
// 377 rows / 83 distinct keys / 127 rows with off-canon keys / 38 null
// statuses / 27 string-valued depends_on / 158 rows with no created_at),
// compressed to the per-case rows the classification branches on:
//
//   DF36-ALIAS      alias keys only (acceptance, _pass_criteria) — mechanical
//   DF36-DEPEMPTY   depends_on "" + status "open" (alias) — mechanical
//   DF36-DEPREF     depends_on "GAP-000" (bare id string) — mechanical
//   DF36-DEPSTRARR  depends_on "[\"GAP-000\", \"GAP-001\"]" — mechanical
//   DF36-DEPNUM     depends_on 7 — REFUSED (ambiguous shape)
//   DF36-DEPOBJ     depends_on {"0":"GAP-000"} — REFUSED
//   DF36-DEPJUNK    depends_on "[GAP-000" (malformed inside the string) — REFUSED
//   DF36-ALIASCLASH alias "ac" + canonical acceptance_criteria — REFUSED
//   DF36-NOSTATUS   no status key — REFUSED (never guessed)
//   DF36-BADSTATUS  status "duplicate" — REFUSED (unknown, human decision)
//   DF36-NOCREATED  clean shape, no created_at — NOTE only (never fabricated)
//   DF36-CLEAN      fully canonical row — untouched
//
// Canonical rows and untouched lines must round-trip byte-identical; the
// apply path is ONE atomic rewrite; no audit event; dry-run writes only the
// review cache.

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// seedDF36Board writes the drift fixture as a topology-A board.
func seedDF36Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id":"DF36-CLEAN","title":"canonical row","status":"pending","priority":"P2","depends_on":[],"created_at":"2026-09-01 00:00:00"}` + "\n" +
			`{"id":"DF36-ALIAS","title":"two aliases of one canon key (the warpfs shape)","status":"review","priority":"P1","acceptance":"works","_pass_criteria":"gated"}` + "\n" +
			`{"id":"DF36-ALIAS2","title":"single alias key","status":"review","priority":"P1","acceptance":"works"}` + "\n" +
			`{"id":"DF36-DEPEMPTY","title":"empty string deps","status":"open","priority":"P3","depends_on":""}` + "\n" +
			`{"id":"DF36-DEPREF","title":"bare id deps","status":"done","priority":"P2","depends_on":"GAP-000"}` + "\n" +
			`{"id":"DF36-DEPSTRARR","title":"stringified array deps","status":"pending","priority":"P1","depends_on":"[\"GAP-000\", \"GAP-001\"]"}` + "\n" +
			`{"id":"DF36-DEPNUM","title":"number deps","status":"pending","priority":"P2","depends_on":7}` + "\n" +
			`{"id":"DF36-DEPOBJ","title":"object deps","status":"pending","priority":"P2","depends_on":{"0":"GAP-000"}}` + "\n" +
			`{"id":"DF36-DEPJUNK","title":"malformed string deps","status":"pending","priority":"P2","depends_on":"[GAP-000"}` + "\n" +
			`{"id":"DF36-ALIASCLASH","title":"alias over a present canonical key","status":"pending","priority":"P2","ac":"same","acceptance_criteria":"real"}` + "\n" +
			`{"id":"DF36-NOSTATUS","title":"no status at all","priority":"P2"}` + "\n" +
			`{"id":"DF36-BADSTATUS","title":"unknown status","status":"duplicate","priority":"P2"}` + "\n" +
			`{"id":"DF36-NOCREATED","title":"missing birth timestamp","status":"pending","priority":"P3"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"df36","namespace":"df36","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// df36ChangesFor returns the change entries for one task id.
func df36ChangesFor(rep *NormalizeBoardReport, id string) []NormalizeRowChange {
	var out []NormalizeRowChange
	for _, c := range rep.Changes {
		if c.ID == id {
			out = append(out, c)
		}
	}
	return out
}

// df36RefusalsFor returns the refusal entries for one task id.
func df36RefusalsFor(rep *NormalizeBoardReport, id string) []NormalizeRowRefusal {
	var out []NormalizeRowRefusal
	for _, r := range rep.Refusals {
		if r.ID == id {
			out = append(out, r)
		}
	}
	return out
}

// df36LineWithID parses the rewritten tasks.jsonl and returns the raw line
// bytes of the row carrying the given id.
func df36LineWithID(t *testing.T, path, id string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			t.Fatalf("rewritten line does not parse: %v: %s", err, l)
		}
		if row.String("id") == id {
			return l
		}
	}
	t.Fatalf("id %q missing from %s", id, path)
	return nil
}

// TestDF36DryRunClassifiesAndWritesNothing: the dry-run reports every
// mechanical change and every refusal, writes ONLY tasks.normalized.jsonl,
// and leaves tasks.jsonl + events.jsonl byte-identical.
func TestDF36DryRunClassifiesAndWritesNothing(t *testing.T) {
	b := seedDF36Board(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := b.NormalizeBoard(false)
	if err != nil {
		t.Fatalf("NormalizeBoard(dry-run): %v", err)
	}
	if rep.Applied {
		t.Fatal("dry-run reported Applied=true")
	}
	if rep.Rows != 13 {
		t.Fatalf("Rows = %d, want 13", rep.Rows)
	}

	// Mechanical changes, per row: single-alias keys, the three convertible
	// depends_on shapes, and the two alias statuses.
	if got := df36ChangesFor(rep, "DF36-ALIAS"); len(got) != 0 {
		t.Fatalf("DF36-ALIAS (two aliases of one canon key) = %+v, want a refusal, not changes", got)
	}
	if got := df36ChangesFor(rep, "DF36-ALIAS2"); len(got) != 1 ||
		got[0].Kind != "key" || got[0].From != "acceptance" || got[0].To != "acceptance_criteria" {
		t.Fatalf("DF36-ALIAS2 changes = %+v, want exactly acceptance->acceptance_criteria", got)
	}
	if got := df36ChangesFor(rep, "DF36-DEPEMPTY"); len(got) != 2 ||
		got[0].Kind != "depends_on" || got[0].From != `""` || got[0].To != `[]` ||
		got[1].Kind != "status" || got[1].From != "open" || got[1].To != "pending" {
		t.Fatalf("DF36-DEPEMPTY changes = %+v, want depends_on \"\"->[] + status open->pending", got)
	}
	if got := df36ChangesFor(rep, "DF36-DEPREF"); len(got) != 2 ||
		got[0].To != `["GAP-000"]` || got[1].From != "done" || got[1].To != "complete" {
		t.Fatalf("DF36-DEPREF changes = %+v, want [\"GAP-000\"] + done->complete", got)
	}
	if got := df36ChangesFor(rep, "DF36-DEPSTRARR"); len(got) != 1 ||
		got[0].From != `"[\"GAP-000\", \"GAP-001\"]"` || got[0].To != `["GAP-000","GAP-001"]` {
		t.Fatalf("DF36-DEPSTRARR changes = %+v", got)
	}

	// Refusals, per row: exactly the ambiguous shapes.
	for _, tc := range []struct{ id, kind, detailSub string }{
		{"DF36-ALIAS", "key", `both map to "acceptance_criteria"`},
		{"DF36-DEPNUM", "depends_on", "JSON number"},
		{"DF36-DEPOBJ", "depends_on", "JSON object"},
		{"DF36-DEPJUNK", "depends_on", "neither a JSON array nor a fleet task id"},
		{"DF36-ALIASCLASH", "key", "already carries"},
		{"DF36-NOSTATUS", "status", "absent or null"},
		{"DF36-BADSTATUS", "status", "not in the vocabulary"},
	} {
		got := df36RefusalsFor(rep, tc.id)
		if len(got) != 1 || got[0].Kind != tc.kind || !strings.Contains(got[0].Detail, tc.detailSub) {
			t.Fatalf("DF36 refusal for %s = %+v, want one %s refusal mentioning %q", tc.id, got, tc.kind, tc.detailSub)
		}
	}

	// The note: every row without a parseable created_at EXCEPT the clean
	// one is reported (12 of 13 — only DF36-CLEAN carries a created_at),
	// never repaired.
	if len(rep.Notes) != 1 || rep.Notes[0].Kind != "missing_created_at" {
		t.Fatalf("Notes = %+v, want exactly one missing_created_at note", rep.Notes)
	}
	if rep.Notes[0].Rows != 12 || !strings.HasPrefix(rep.Notes[0].IDs, "DF36-ALIAS,") || !strings.Contains(rep.Notes[0].IDs, "DF36-DEPEMPTY") {
		t.Fatalf("missing_created_at note = %+v, want 12 rows led by DF36-ALIAS", rep.Notes[0])
	}

	// Nothing on the board changed; the review cache holds the would-be bytes.
	after, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry-run modified tasks.jsonl")
	}
	eventsAfter, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatal("dry-run modified events.jsonl")
	}
	if rep.AppliedPath == "" || !strings.HasSuffix(rep.AppliedPath, "tasks.normalized.jsonl") {
		t.Fatalf("dry-run AppliedPath = %q, want the review cache path", rep.AppliedPath)
	}
}

// stringJSON is a test helper removed: the expected compact rendering is
// asserted as a string literal above.

// TestDF36ApplyRewritesExactlyTheClassifiedRows: --apply rewrites
// tasks.jsonl once; every repaired row carries the canonical shape, every
// refused/clean/untouched row is byte-identical, and a second --apply is a
// clean no-op (idempotent).
func TestDF36ApplyRewritesExactlyTheClassifiedRows(t *testing.T) {
	b := seedDF36Board(t)
	before, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := b.NormalizeBoard(true)
	if err != nil {
		t.Fatalf("NormalizeBoard(apply): %v", err)
	}
	if !rep.Applied || rep.AppliedPath != b.tasksPath {
		t.Fatalf("apply report = Applied:%v Path:%q, want tasks.jsonl", rep.Applied, rep.AppliedPath)
	}
	if len(rep.Refusals) == 0 {
		t.Fatal("apply reported no refusals — the fixture has seven")
	}
	if len(rep.Notes) != 1 {
		t.Fatalf("apply Notes = %+v, want the missing_created_at census", rep.Notes)
	}

	// Repaired rows: canonical shapes on disk.
	alias := df36LineWithID(t, b.tasksPath, "DF36-ALIAS2")
	aRow, err := ParseRow(alias)
	if err != nil {
		t.Fatal(err)
	}
	if aRow.Has("acceptance") {
		t.Fatalf("alias keys survived apply: %s", alias)
	}
	if !aRow.Has("acceptance_criteria") {
		t.Fatalf("canonical keys missing after apply: %s", alias)
	}
	if aRow.String("acceptance_criteria") != "works" {
		t.Fatalf("alias VALUES not carried verbatim through the rename: %s", alias)
	}

	dep := df36LineWithID(t, b.tasksPath, "DF36-DEPSTRARR")
	dRow, err := ParseRow(dep)
	if err != nil {
		t.Fatal(err)
	}
	if kind := jsonKindName(dRow.Get("depends_on")); kind != "array" {
		t.Fatalf("DF36-DEPSTRARR depends_on kind after apply = %s, want array: %s", kind, dep)
	}
	if deps := rowStringSlice(dRow, "depends_on"); len(deps) != 2 || deps[0] != "GAP-000" || deps[1] != "GAP-001" {
		t.Fatalf("DF36-DEPSTRARR depends_on after apply = %s", dep)
	}
	ref := df36LineWithID(t, b.tasksPath, "DF36-DEPREF")
	if !strings.Contains(string(ref), `"depends_on":["GAP-000"]`) {
		t.Fatalf("bare-id deps not converted to a one-element array: %s", ref)
	}
	if st := df36LineWithID(t, b.tasksPath, "DF36-DEPEMPTY"); !strings.Contains(string(st), `"status":"pending"`) {
		t.Fatalf("status alias open not canonicalized: %s", st)
	}

	// Refused rows stay byte-identical.
	for _, id := range []string{"DF36-ALIAS", "DF36-DEPNUM", "DF36-DEPOBJ", "DF36-DEPJUNK", "DF36-ALIASCLASH", "DF36-NOSTATUS", "DF36-BADSTATUS"} {
		want := df36RawLine(t, before, id)
		got := df36LineWithID(t, b.tasksPath, id)
		if !bytes.Equal(want, got) {
			t.Fatalf("refused row %s changed:\nbefore %s\nafter  %s", id, want, got)
		}
	}
	// Clean and note-only rows stay byte-identical too.
	for _, id := range []string{"DF36-CLEAN", "DF36-NOCREATED"} {
		want := df36RawLine(t, before, id)
		got := df36LineWithID(t, b.tasksPath, id)
		if !bytes.Equal(want, got) {
			t.Fatalf("untouched row %s changed:\nbefore %s\nafter  %s", id, want, got)
		}
	}

	// The board validates with exactly the errors the REFUSED rows deserve
	// (the unknown "duplicate" status on DF36-BADSTATUS stays an error —
	// the repair refused it, so validate still owns that failure) and NO
	// error from anything the repair wrote: the rewritten rows and every
	// untouched row contribute warnings at most.
	if err := b.SkipBad; err {
		t.Fatal("SkipBad leaked")
	}
	rep2, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep2.Errors() != 1 {
		t.Fatalf("post-apply validate errors = %d, want exactly the refused duplicate-status row:\n%+v", rep2.Errors(), rep2.Findings)
	}
	for _, f := range rep2.Findings {
		if f.IsError() && !strings.Contains(f.Msg, "DF36-BADSTATUS") {
			t.Fatalf("post-apply validate error outside the refused row: %s", f.Msg)
		}
	}
	// And the census moved: the two alias rows repaired leave only the
	// refused rows as key-drift.
	if got := rep2.Keys.Summary(); !strings.Contains(got, "drift 2 row(s)") {
		t.Fatalf("post-apply key census = %q, want only the 2 refused alias rows drifting", got)
	}

	// Idempotent: a second apply reports no CHANGES (the refused rows keep
	// their refusals — the board is not "clean", but nothing further is
	// mechanical) and the file is byte-identical.
	rep3, err := b.NormalizeBoard(true)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(rep3.Changes) != 0 {
		t.Fatalf("second apply reported %+v changes, want none", rep3.Changes)
	}
	if rep3.Applied {
		t.Fatal("second apply rewrote the file with zero changes")
	}
	after2, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	after1, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after1, after2) {
		t.Fatal("second apply rewrote the file")
	}
}

// df36RawLine returns the raw line bytes carrying id in the given file
// content.
func df36RawLine(t *testing.T, content []byte, id string) []byte {
	t.Helper()
	for _, l := range bytes.Split(content, []byte("\n")) {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			t.Fatalf("fixture line does not parse: %v: %s", err, l)
		}
		if row.String("id") == id {
			return l
		}
	}
	t.Fatalf("id %q not found in fixture", id)
	return nil
}

// TestDF36CleanBoardIsANoop: a canonical board reports clean in BOTH modes
// and writes NOTHING — no cache file, no rewrite.
func TestDF36CleanBoardIsANoop(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"DF36-OK","title":"fine","status":"pending","priority":"P2","depends_on":[],"created_at":"2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-04 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"t","namespace":"t","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, apply := range []bool{false, true} {
		rep, err := b.NormalizeBoard(apply)
		if err != nil {
			t.Fatalf("apply=%v: %v", apply, err)
		}
		if !rep.Clean || len(rep.Changes) != 0 || len(rep.Refusals) != 0 {
			t.Fatalf("clean board report = %+v, want clean", rep)
		}
		if _, err := os.Stat(b.Dir + "/tasks.normalized.jsonl"); err == nil {
			t.Fatal("clean board wrote a review cache file")
		}
	}
}

// TestDF36DryRunCacheHoldsTheApplyBytes: the staged review file is exactly
// what --apply later writes to tasks.jsonl (diff-then-trust contract).
func TestDF36DryRunCacheHoldsTheApplyBytes(t *testing.T) {
	b1 := seedDF36Board(t)
	dry, err := b1.NormalizeBoard(false)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(dry.AppliedPath)
	if err != nil {
		t.Fatal(err)
	}

	b2 := seedDF36Board(t) // fresh copy of the same fixture
	if _, err := b2.NormalizeBoard(true); err != nil {
		t.Fatal(err)
	}
	applied, err := os.ReadFile(b2.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, applied) {
		t.Fatalf("staged dry-run bytes != applied bytes:\nstaged  %s\napplied %s", staged, applied)
	}
}

// TestDF36ApplyWritesNoEvent: shape repair is not a state transition — no
// audit event is appended and updated_at is not refreshed.
func TestDF36ApplyWritesNoEvent(t *testing.T) {
	b := seedDF36Board(t)
	eventsBefore, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.NormalizeBoard(true); err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := os.ReadFile(b.eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(eventsBefore, eventsAfter) {
		t.Fatalf("normalize-board appended an event:\nbefore %s\nafter  %s", eventsBefore, eventsAfter)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if id := r.String("id"); id == "DF36-ALIAS" && r.Has("updated_at") {
			t.Fatalf("normalize-board refreshed updated_at on %s", id)
		}
	}
}

// TestDF36NormalizeBoardRenderText: the human report names every change,
// refusal, and note, plus the mode line.
func TestDF36NormalizeBoardRenderText(t *testing.T) {
	b := seedDF36Board(t)
	rep, err := b.NormalizeBoard(false)
	if err != nil {
		t.Fatal(err)
	}
	txt := rep.RenderText()
	for _, want := range []string{
		"DF36-ALIAS",
		`key "acceptance" -> "acceptance_criteria"`,
		"REFUSED depends_on",
		"REFUSED status",
		"missing_created_at",
		"missing_created_at on 12 row(s)",
		"DF36-DEPEMPTY",
		"DRY-RUN",
		"tasks.normalized.jsonl",
	} {
		if !strings.Contains(txt, want) {
			t.Errorf("rendered report missing %q", want)
		}
	}
}

// TestDF36JSONReport: the JSON shape carries the fields a lane needs.
func TestDF36JSONReport(t *testing.T) {
	b := seedDF36Board(t)
	rep, err := b.NormalizeBoard(false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"board", "rows", "changes", "refusals", "notes", "applied", "clean", "applied_path"} {
		if _, ok := back[key]; !ok {
			t.Errorf("JSON report missing %q", key)
		}
	}
}

// TestDF36RefusedRowKeepsOtherRowsRepairable: one refusal never blocks the
// mechanical repairs on other rows (the DF36-DEPNUM row is refused; the
// DF36-ALIAS2 row on the next line is still converted by the same apply).
func TestDF36RefusedRowKeepsOtherRowsRepairable(t *testing.T) {
	b := seedDF36Board(t)
	if _, err := b.NormalizeBoard(true); err != nil {
		t.Fatal(err)
	}
	if got := df36LineWithID(t, b.tasksPath, "DF36-ALIAS2"); !strings.Contains(string(got), `"acceptance_criteria"`) {
		t.Fatalf("a refusal on one row blocked the repair of another: %s", got)
	}
	if got := df36LineWithID(t, b.tasksPath, "DF36-DEPNUM"); !strings.Contains(string(got), `"depends_on":7`) {
		t.Fatalf("refused row was mutated: %s", got)
	}
}
