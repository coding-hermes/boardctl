package board

// BT-059: create's key schema must be CANONICAL, not inherited from the
// board's drift.
//
// The bug (reproduced on a temp board 2026-09-24): `create` deep-copied the
// LAST task row's key set, so a legacy row carrying a stray key ("repo":null)
// stamped that key onto every new row. The BT-056 key-uniformity census then
// warned each new row for the very drift create had copied in — the census
// became self-perpetuating: repair the old rows and new ones are still born
// dirty, so a board can never reach zero drift while a legacy row exists.
// Contrast GAP-145 on the bunker board: rows created minutes earlier carried
// no repo key at all.
//
// The contract now: create inherits ONLY the canon-sanctioned part of the
// mirrored row's schema (last row's keys INTERSECT IsSanctionedTaskKey). The
// measured census union keeps feeding validate's reporting (warning severity,
// exit 0 unchanged) but never the create path again. A board extra key that
// IS deliberate belongs in the canon (like labels/perpetual) — sanctioned
// keys still flow to new rows, so fresh-init self-similarity is preserved.
// On a clean board the filtered mirror is the mirror itself: byte-identical
// shape to before the fix.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// seedBT059DriftedBoard writes a topology-A board whose single task row is a
// LEGACY row carrying keys create never asked for: repo (per-project tooling,
// the field GAP-145 showed newly created rows must not carry) and reviewer
// (an unwritten phase key). If create inherits its schema from this row
// verbatim, every new row is born carrying both — the BT-059 defect.
func seedBT059DriftedBoard(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"LEG-1","title":"legacy drift row","status":"pending","priority":"P2","repo":null,"reviewer":"alex"}` + "\n",
		"events.jsonl": `{ "id": 1, "timestamp": "2026-09-24 00:00:00.000000", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1 }` + "\n",
		"board.jsonl":  `{ "project": "bt059", "namespace": "bt059", "version": 1, "ticks_total": 0, "ticks_idle": 0, "last_commit": null }` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bt059DriftKeys are the keys the legacy row carries beyond the core four —
// none of them may ever reach a new row.
var bt059DriftKeys = []string{"repo", "reviewer"}

// bt059SparseCreateSchema is the canonical shape a create inherits from a
// sparse 4-key mirror row: the row's own four keys, then the core keys the
// guarantee step adds (complexity + the two timestamps), then the detail
// fingerprint stamp. Both a clean sparse board and a drifted one must produce
// exactly this key list — the drift must have ZERO influence on the shape.
var bt059SparseCreateSchema = []string{
	"id", "title", "status", "priority",
	"complexity", "created_at", "updated_at",
	"detail",
}

// TestCreateOnDriftedBoardWritesCanonicalSchema — the BT-059 RED test. Create
// on a board whose last row carries drift keys must produce a row that (a)
// carries none of them, (b) has EXACTLY the key list a clean-board create
// produces, and (c) passes the canon check outright (every key sanctioned),
// so the census has nothing new to warn about.
func TestCreateOnDriftedBoardWritesCanonicalSchema(t *testing.T) {
	b := seedBT059DriftedBoard(t)
	if _, err := b.Create(TaskRowSpec{ID: "TEST-1", Title: "fresh row", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("create on drifted board: %v", err)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("task rows = %d, want 2 (LEG-1 + TEST-1)", len(rows))
	}
	fresh := rows[1]

	// (a) the drift keys create never asked for are gone
	for _, k := range bt059DriftKeys {
		if fresh.Has(k) {
			t.Fatalf("new row inherited drift key %q from the legacy row: %s", k, RowJSONCompact(fresh))
		}
	}
	// (b) the shape is exactly the clean canonical one — same key list a
	// clean sparse board produces, in the same order.
	if strings.Join(fresh.Keys, ",") != strings.Join(bt059SparseCreateSchema, ",") {
		t.Fatalf("new row key list is not the canonical sparse create schema:\n got %v\nwant %v", fresh.Keys, bt059SparseCreateSchema)
	}
	// (c) the whole row passes the canon check — create wrote the canonical
	// schema, not the drifted measured union
	for _, k := range fresh.Keys {
		if !IsSanctionedTaskKey(k) {
			t.Fatalf("new row carries non-canon key %q: %s", k, RowJSONCompact(fresh))
		}
	}
	// neutral values survived the filtering
	if n, ok := fresh.Int("complexity"); !ok || n != 3 {
		t.Fatalf("complexity = %s, want 3", fresh.Get("complexity"))
	}
	if ts := fresh.String("created_at"); ts == "" || ts != fresh.String("updated_at") {
		t.Fatalf("created_at/updated_at not stamped in one dialect: %q / %q", fresh.String("created_at"), fresh.String("updated_at"))
	}

	// The legacy row is untouched byte-for-byte (append-only create).
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	wantLegacy := `{"id":"LEG-1","title":"legacy drift row","status":"pending","priority":"P2","repo":null,"reviewer":"alex"}`
	if lines[0] != wantLegacy {
		t.Fatalf("legacy row was rewritten by create:\n got %s\nwant %s", lines[0], wantLegacy)
	}
}

// TestCreateOnCleanBoardSchemaUnchanged pins the no-regression half: on a
// board whose rows carry only canonical keys, create's output is byte-shape
// identical to before BT-059 — the mirrored keys in the mirror's order, then
// the core keys, then the detail stamp; compact style; neutral values.
func TestCreateOnCleanBoardSchemaUnchanged(t *testing.T) {
	b := newTestBoard(t) // EXIST-1: id/title/status/priority, nothing else
	if _, err := b.Create(TaskRowSpec{ID: "CLEAN-2", Title: "fresh on clean board", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("create on clean board: %v", err)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("task rows = %d, want 2", len(rows))
	}
	fresh := rows[len(rows)-1] // file order: EXIST-1 first, the create target last

	if strings.Join(fresh.Keys, ",") != strings.Join(bt059SparseCreateSchema, ",") {
		t.Fatalf("clean-board create key order changed:\n got %v\nwant %v", fresh.Keys, bt059SparseCreateSchema)
	}
	for _, k := range bt059DriftKeys {
		if fresh.Has(k) {
			t.Fatalf("clean board must not grow drift keys either: %s", RowJSONCompact(fresh))
		}
	}

	// Neutral values, exactly as before BT-059.
	if n, ok := fresh.Int("complexity"); !ok || n != 3 {
		t.Fatalf("complexity = %s, want 3", fresh.Get("complexity"))
	}
	if ts := fresh.String("created_at"); ts == "" || ts != fresh.String("updated_at") {
		t.Fatalf("created_at/updated_at not stamped in one dialect: %q / %q", fresh.String("created_at"), fresh.String("updated_at"))
	}
	if raw := fresh.Get("detail"); raw == nil || raw[0] != '{' || !strings.Contains(string(raw), `"fingerprint":"`) {
		t.Fatalf("detail fingerprint stamp missing or malformed: %s", raw)
	}

	// Spotted serialization: compact, like the seeded file, core fields in
	// the mirrored order with complexity immediately after priority.
	raw, err := os.ReadFile(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.SplitN(strings.TrimRight(string(raw), "\n"), "\n", 2)[1]
	if strings.Contains(line, ": ") || strings.Contains(line, ", ") {
		t.Fatalf("clean-board create lost the file's compact style: %s", line)
	}
	if !strings.HasPrefix(line, `{"id":"CLEAN-2","title":"fresh on clean board","status":"pending","priority":"P2","complexity":3,"created_at":"`) {
		t.Fatalf("clean-board create shape drifted:\n%s", line)
	}
}

// TestCensusStillReportsLegacyRowOnDriftedBoard: the fix stops INHERITING
// drift, it does not forgive it — the census keeps reporting the legacy row
// (warning severity, exit 0 semantics), and the new row adds nothing to it.
func TestCensusStillReportsLegacyRowOnDriftedBoard(t *testing.T) {
	b := seedBT059DriftedBoard(t)
	if _, err := b.Create(TaskRowSpec{ID: "TEST-1", Title: "fresh row", Status: "pending", Priority: "P2"}); err != nil {
		t.Fatalf("create on drifted board: %v", err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("key drift must warn, not error: %+v", rep.Findings)
	}
	if rep.Keys.DriftRows() != 1 {
		t.Fatalf("census must report exactly the legacy row as drift, got %d rows (%+v)", rep.Keys.DriftRows(), rep.Keys)
	}
	for _, k := range bt059DriftKeys {
		if rep.Keys.Keys[k] != 1 {
			t.Fatalf("census must still count %q on the legacy row, got %+v", k, rep.Keys.Keys)
		}
	}
	found := findWarn(rep, "LEG-1")
	if found == "" || !strings.Contains(found, "repo") {
		t.Fatalf("census warning for LEG-1 lost the drift key name, got %q", found)
	}
	// the summary the fleet quotes still names the offending keys
	if !strings.Contains(rep.Keys.Summary(), "repo(1)") {
		t.Fatalf("summary must still name repo(1): %q", rep.Keys.Summary())
	}
}

// TestCreateExplicitFieldsLandOnCanonicalRow: explicitly passed fields all
// land on the new row even on a drifted board — the fix narrows the INHERITED
// schema, never the caller's own fields.
func TestCreateExplicitFieldsLandOnCanonicalRow(t *testing.T) {
	b := seedBT059DriftedBoard(t)
	cx := int64(2)
	reasoning := "because the fix is narrow"
	tags := []string{"bt059", "schema"}
	deps := []string{"LEG-1"}
	wt := "/home/kara/worktrees/coding-hermes-boardctl-BT-059"
	br := "wt/BT-059"
	sessions := []string{"sess-1", "sess-2"}
	if _, err := b.Create(TaskRowSpec{
		ID:             "TEST-1",
		Title:          "explicit fields",
		Status:         "pending",
		Priority:       "P1",
		Complexity:     &cx,
		Reasoning:      reasoning,
		HasDependsOn:   true,
		DependsOn:      deps,
		HasTags:        true,
		CapabilityTags: tags,
		Worktree:       &wt,
		Branch:         &br,
		Sessions:       &sessions,
	}); err != nil {
		t.Fatalf("create with explicit fields: %v", err)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	fresh := rows[len(rows)-1]
	if fresh.String("id") != "TEST-1" || fresh.String("priority") != "P1" {
		t.Fatalf("core explicit fields missing: %s", RowJSONCompact(fresh))
	}
	if n, ok := fresh.Int("complexity"); !ok || n != 2 {
		t.Fatalf("complexity = %s, want 2", fresh.Get("complexity"))
	}
	if got := fresh.String("reasoning"); got != reasoning {
		t.Fatalf("reasoning = %q, want %q", got, reasoning)
	}
	var gotDeps []string
	if err := json.Unmarshal(fresh.Get("depends_on"), &gotDeps); err != nil || len(gotDeps) != 1 || gotDeps[0] != "LEG-1" {
		t.Fatalf("depends_on = %s, want [LEG-1]", fresh.Get("depends_on"))
	}
	var gotTags []string
	if err := json.Unmarshal(fresh.Get("capability_tags"), &gotTags); err != nil || len(gotTags) != 2 || gotTags[0] != "bt059" || gotTags[1] != "schema" {
		t.Fatalf("capability_tags = %s, want [bt059 schema]", fresh.Get("capability_tags"))
	}
	if got := fresh.String("worktree"); got != wt {
		t.Fatalf("worktree = %q, want %q", got, wt)
	}
	if got := fresh.String("branch"); got != br {
		t.Fatalf("branch = %q, want %q", got, br)
	}
	var gotSessions []string
	if err := json.Unmarshal(fresh.Get("sessions"), &gotSessions); err != nil || len(gotSessions) != 2 || gotSessions[0] != "sess-1" || gotSessions[1] != "sess-2" {
		t.Fatalf("sessions = %s, want [sess-1 sess-2]", fresh.Get("sessions"))
	}
	// the drift keys are still absent, and the row is canonical — explicit
	// fields are all sanctioned keys
	for _, k := range bt059DriftKeys {
		if fresh.Has(k) {
			t.Fatalf("explicit-field create still inherited drift key %q: %s", k, RowJSONCompact(fresh))
		}
	}
	for _, k := range fresh.Keys {
		if !IsSanctionedTaskKey(k) {
			t.Fatalf("explicit-field row carries non-canon key %q: %s", k, RowJSONCompact(fresh))
		}
	}
}
