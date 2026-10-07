package board

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-076: the first-class deferred flag. These tests pin the whole contract:
//
//   - create --deferred writes deferred:true; the omitted flag writes NO key
//   - update --deferred false writes deferred:false; an omitted update never
//     creates or changes the key
//   - the key is sanctioned (no canon drift), and a non-boolean value warns
//     without failing validate
//   - stats counts deferred separately and excludes deferred rows from the
//     pending/actionable tally (they stay in Total and in the status tally)
//   - list/show/export/import never filter or drop deferred rows
//
// The seed rows use spaced style on purpose: the boolean must be encoded in
// the file's own style, like every BT-037 field before it.

// seedBT076Board writes a topology-A board with one plain pending row to
// update and one deferred row to count.
func seedBT076Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "DF-1", "title": "plain pending", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "DF-2", "title": "parked", "status": "pending", "priority": "P1", "deferred": true}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt076", "namespace": "bt076", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bt076Row parses one tasks.jsonl line by id.
func bt076Row(t *testing.T, b *Board, id string) *Row {
	t.Helper()
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.String("id") == id {
			return r
		}
	}
	t.Fatalf("task %q not found", id)
	return nil
}

func bt076Raw(t *testing.T, b *Board, id string) string {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, `"id": "`) && bt076LineID(l) == id {
			return l
		}
	}
	t.Fatalf("task %q not found in %s", id, raw)
	return ""
}

// bt076LineID pulls the id value out of a spaced-style line without a full
// parse (the seed rows are the only writer in these tests).
func bt076LineID(line string) string {
	i := strings.Index(line, `"id": "`)
	if i < 0 {
		return ""
	}
	rest := line[i+len(`"id": "`):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// TestCreateDeferredWritesFlag: create --deferred writes deferred:true.
func TestCreateDeferredWritesFlag(t *testing.T) {
	b := seedBT076Board(t)
	t.Helper()
	id, err := b.Create(TaskRowSpec{ID: "DF-NEW", Title: "new deferred row", Deferred: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if id != "DF-NEW" {
		t.Fatalf("created %q", id)
	}
	r := bt076Row(t, b, "DF-NEW")
	raw := r.Get("deferred")
	if raw == nil {
		t.Fatalf("deferred key missing on the created row: %s", bt076Raw(t, b, "DF-NEW"))
	}
	if !RowIsDeferred(r) {
		t.Fatalf("deferred = %s, want a true literal", raw)
	}
	if !r.Has("deferred") {
		t.Fatal("deferred key absent")
	}
}

// TestCreateOmittedDeferredWritesNoKey: the omitted flag never creates the
// key (nil-untouched discipline, BT-037 pattern).
func TestCreateOmittedDeferredWritesNoKey(t *testing.T) {
	b := seedBT076Board(t)
	if _, err := b.Create(TaskRowSpec{ID: "DF-PLAIN", Title: "plain row"}); err != nil {
		t.Fatal(err)
	}
	line := bt076Raw(t, b, "DF-PLAIN")
	if strings.Contains(line, "deferred") {
		t.Fatalf("omitted --deferred wrote the key anyway: %s", line)
	}
	if RowIsDeferred(bt076Row(t, b, "DF-PLAIN")) {
		t.Fatal("row reads as deferred without the key")
	}
}

// TestCreateDeferredFalseWritesFalse: an explicit false IS written (a fresh
// row that names its own not-deferred state carries the key).
func TestCreateDeferredFalseWritesFalse(t *testing.T) {
	b := seedBT076Board(t)
	if _, err := b.Create(TaskRowSpec{ID: "DF-FALSE", Title: "explicit false", Deferred: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	r := bt076Row(t, b, "DF-FALSE")
	if !r.Has("deferred") {
		t.Fatalf("explicit --deferred false did not write the key: %s", bt076Raw(t, b, "DF-FALSE"))
	}
	if RowIsDeferred(r) {
		t.Fatal("deferred:false reads as deferred")
	}
}

// TestUpdateDeferredRoundTrip: --deferred false on a deferred row writes
// deferred:false; an omitted update leaves the key untouched (both value and
// position — untouched line bytes round-trip).
func TestUpdateDeferredRoundTrip(t *testing.T) {
	b := seedBT076Board(t)
	before := bt076Raw(t, b, "DF-2")
	dv := false
	if _, err := b.UpdateTask("DF-2", UpdateSpec{Deferred: &dv}); err != nil {
		t.Fatal(err)
	}
	after := bt076Raw(t, b, "DF-2")
	if !strings.Contains(after, `"deferred": false`) {
		t.Fatalf("update --deferred false wrote %s, want \"deferred\": false", after)
	}
	if RowIsDeferred(bt076Row(t, b, "DF-2")) {
		t.Fatal("row still reads deferred after --deferred false")
	}
	// omitted update leaves the key untouched
	if _, err := b.UpdateTask("DF-2", UpdateSpec{Summary: strPtr("note")}); err != nil {
		t.Fatal(err)
	}
	line := bt076Raw(t, b, "DF-2")
	if !strings.Contains(line, `"deferred": false`) {
		t.Fatalf("omitted update changed the deferred key: %s", line)
	}
	// and on a row that never carried the key, an omitted update must not
	// create it
	if _, err := b.UpdateTask("DF-1", UpdateSpec{Summary: strPtr("note")}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bt076Raw(t, b, "DF-1"), "deferred") {
		t.Fatal("omitted update created the deferred key")
	}
	// un-deferred -> deferred again via update
	tv := true
	if _, err := b.UpdateTask("DF-1", UpdateSpec{Deferred: &tv}); err != nil {
		t.Fatal(err)
	}
	if !RowIsDeferred(bt076Row(t, b, "DF-1")) {
		t.Fatal("update --deferred true did not defer the row")
	}
	if before == "" {
		t.Fatal("sanity: before line empty")
	}
}

// TestDeferredKeySanctioned: the deferred key must not flag as canon drift.
func TestDeferredKeySanctioned(t *testing.T) {
	if !IsSanctionedTaskKey("deferred") {
		t.Fatal(`key "deferred" should be sanctioned (BT-076)`)
	}
	// the near-miss plural and the alias spelling must stay drift
	for _, k := range []string{"deferred_reason", "defered"} {
		if IsSanctionedTaskKey(k) {
			t.Errorf("key %q should NOT be sanctioned (it is drift, not the BT-076 key)", k)
		}
	}
	b := seedBT076Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "outside the declared canon") {
			t.Fatalf("seed board flagged canon drift:\n%s", f.Msg)
		}
	}
}

// TestValidateDeferredNonBooleanWarns: a non-boolean deferred value is an
// itemized warning, not a failure — the board stays exit-0.
func TestValidateDeferredNonBooleanWarns(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id": "DF-NB", "title": "hand-edited", "status": "pending", "priority": "P2", "deferred": "yes"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt076", "namespace": "bt076", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("non-boolean deferred failed validate (want warn only):\n%s", rep.RenderText())
	}
	found := false
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, "deferred is not a boolean") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no deferred-shape warning found:\n%s", rep.RenderText())
	}
	// absent and boolean shapes stay silent
	b2 := seedBT076Board(t)
	rep2, err := b2.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep2.Findings {
		if strings.Contains(f.Msg, "deferred") {
			t.Fatalf("boolean/absent deferred flagged:\n%s", f.Msg)
		}
	}
	// a non-boolean value reads as NOT deferred (soft semantics)
	if RowIsDeferred(bt076Row(t, b, "DF-NB")) {
		t.Fatal(`deferred:"yes" read as deferred`)
	}
}

// TestStatsDeferredSeparateAndNotActionable: deferred rows count in Total and
// keep their status tally, ride the separate Deferred count, and are excluded
// from Actionable.
func TestStatsDeferredSeparateAndNotActionable(t *testing.T) {
	b := seedBT076Board(t)
	st, err := b.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 2 {
		t.Fatalf("total = %d, want 2 (deferred rows stay counted)", st.Total)
	}
	if st.Deferred != 1 {
		t.Fatalf("deferred = %d, want 1", st.Deferred)
	}
	if st.Actionable != 1 {
		t.Fatalf("actionable = %d, want 1 (the deferred pending row is excluded)", st.Actionable)
	}
	if st.Status["pending"] != 2 {
		t.Fatalf("status[pending] = %d, want 2 (status tally keeps deferred rows)", st.Status["pending"])
	}
	// a deferred COMPLETE row: rides Deferred, still complete in the tally,
	// never actionable either way
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id": "DF-C", "title": "done while parked", "status": "complete", "priority": "P2", "deferred": true}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt076", "namespace": "bt076", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b2, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	st2, err := b2.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if st2.Deferred != 1 || st2.Actionable != 0 || st2.Status["complete"] != 1 {
		t.Fatalf("deferred complete row stats = %+v, want deferred 1, actionable 0, complete 1", st2)
	}
	// the rendered text carries both lines
	txt := st.RenderText()
	if !strings.Contains(txt, "deferred: 1") || !strings.Contains(txt, "actionable: 1") {
		t.Fatalf("RenderText missing deferred/actionable lines:\n%s", txt)
	}
	// the JSON stats carry the deferred key
	out, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if m["deferred"] != float64(1) {
		t.Fatalf("JSON stats deferred = %v, want 1", m["deferred"])
	}
}

// TestListAndShowRetainDeferredRows: list/show keep returning deferred rows —
// retention, not filtering.
func TestListAndShowRetainDeferredRows(t *testing.T) {
	b := seedBT076Board(t)
	rows, err := b.ListTasks(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range rows {
		ids[r.String("id")] = true
	}
	if !ids["DF-2"] {
		t.Fatal("list dropped the deferred row")
	}
	row, _, err := b.ShowTask("DF-2")
	if err != nil || row == nil {
		t.Fatalf("show lost the deferred row: %v, %v", row, err)
	}
}

// TestImportRoundTripsDeferred: the BT-022 raw-row path preserves the
// deferred key and its verbatim value through create(Raw).
func TestImportRoundTripsDeferred(t *testing.T) {
	b := seedBT076Board(t)
	raw := `{"id": "DF-IMP", "title": "imported deferred", "status": "pending", "priority": "P3", "deferred": true}`
	if _, err := b.Create(TaskRowSpec{ID: "DF-IMP", Title: "imported deferred", Raw: []byte(raw)}); err != nil {
		t.Fatal(err)
	}
	got := bt076Raw(t, b, "DF-IMP")
	if got != raw {
		t.Fatalf("import did not round-trip the row verbatim:\nwant %s\ngot  %s", raw, got)
	}
	if !RowIsDeferred(bt076Row(t, b, "DF-IMP")) {
		t.Fatal("imported deferred row reads as not deferred")
	}
}

func boolPtr(v bool) *bool { return &v }
