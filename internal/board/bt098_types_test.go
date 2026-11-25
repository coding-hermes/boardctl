package board

// BT-098 §10 test plan — registry load, template precedence, and the
// create/update/validate behaviour of typed rows, all asserted against the
// bytes on disk.
//
// The §10 falsifier triple is enforced here, not just the happy path: every
// refusal asserts (a) the exact refusal text, (b) tasks.jsonl is BYTE
// IDENTICAL, and (c) events.jsonl is BYTE IDENTICAL. A writer that "refuses"
// after appending, or that mutates a row before noticing the violation, cannot
// pass.
//
// The fixture is the §2.4 worked example verbatim — read from
// testdata/bt098-types.jsonl rather than re-typed here, so the schema under
// test and the schema the spec documents cannot drift apart.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bt098ReleaseTypeJSON is the §2.4 release declaration, read once from
// testdata so a spec edit that changes the example fails these tests instead
// of leaving them asserting a stale copy.
func bt098ReleaseTypeJSON(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "bt098-release-type.jsonl"))
	if err != nil {
		t.Fatalf("read §2.4 fixture: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	if strings.Contains(line, "\n") {
		t.Fatalf("types.jsonl is one JSON object per line; the fixture is multi-line")
	}
	return line
}

// bt098Seed writes a topology-A board with the §2.4 registry and one existing
// conformant release row, returning the resolved Board.
func bt098Seed(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"REL-TEST-1","title":"[RELEASE] Existing release","type":"release","status":"pending","priority":"P1","complexity":3,"created_at":"2026-11-01T00:00:00+00:00","updated_at":"2026-11-01T00:00:00+00:00","detail":{"metadata":{"channel":"beta","source_commit":"abc1234"}}}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-11-01 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"bt098","namespace":"bt098","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
		"types.jsonl":  bt098ReleaseTypeJSON(t) + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("resolve seeded board: %v", err)
	}
	return b
}

// bt098Snapshot captures the two WRITABLE files a refusal must leave alone.
func bt098Snapshot(t *testing.T, b *Board) (tasks, events string) {
	t.Helper()
	return bt098File(t, b.TasksPath()), bt098File(t, b.EventsPath())
}

func bt098File(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

// bt098AssertUnchanged is the falsifier's (b)+(c): the refusal wrote NOTHING.
func bt098AssertUnchanged(t *testing.T, b *Board, tasks, events string) {
	t.Helper()
	if got := bt098File(t, b.TasksPath()); got != tasks {
		t.Fatalf("refusal changed tasks.jsonl:\n--- before\n%s\n--- after\n%s", tasks, got)
	}
	if got := bt098File(t, b.EventsPath()); got != events {
		t.Fatalf("refusal changed events.jsonl:\n--- before\n%s\n--- after\n%s", events, got)
	}
}

// bt098LastRow returns the last non-empty line of tasks.jsonl — the row a
// create just appended, read as BYTES rather than through the parser so the
// serialization itself is under test.
func bt098LastRow(t *testing.T, b *Board) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(bt098File(t, b.TasksPath()), "\n"), "\n")
	return lines[len(lines)-1]
}

func bt098RowByID(t *testing.T, b *Board, id string) string {
	t.Helper()
	for _, line := range strings.Split(bt098File(t, b.TasksPath()), "\n") {
		if strings.Contains(line, `"id":"`+id+`"`) {
			return line
		}
	}
	t.Fatalf("row %s not found on disk", id)
	return ""
}

func bt098SHA(t *testing.T, path string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(bt098File(t, path)))
	return hex.EncodeToString(sum[:])
}

// --- AC-2: built-in registry -------------------------------------------------

// TestBT098_DefaultRegistry pins AC-2: no types.jsonl means exactly the four
// built-ins, no templates, no required metadata, and the file is NOT reported
// as present (so validate adds zero findings).
func TestBT098_DefaultRegistry(t *testing.T) {
	reg, err := LoadTicketTypes(t.TempDir())
	if err != nil {
		t.Fatalf("LoadTicketTypes on a registry-less board: %v", err)
	}
	if reg.Present {
		t.Fatal("absent types.jsonl must not be marked present")
	}
	if got, want := reg.Names(), "bug, chore, feat, task"; got != want {
		t.Fatalf("built-in registry = %q, want %q", got, want)
	}
	for _, typ := range reg.Sorted(true) {
		if typ.Template != nil {
			t.Errorf("built-in %q carries a template", typ.Slug)
		}
		if len(typ.Metadata) != 0 {
			t.Errorf("built-in %q carries metadata fields", typ.Slug)
		}
	}

	// §6: a 0-byte types.jsonl is indistinguishable from an absent one.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err := LoadTicketTypes(dir)
	if err != nil {
		t.Fatalf("LoadTicketTypes on an empty types.jsonl: %v", err)
	}
	if empty.Present || empty.Names() != reg.Names() {
		t.Fatalf("empty types.jsonl did not fall back to built-ins: present=%v names=%q", empty.Present, empty.Names())
	}
}

// TestBT098_RegistryLastLineWins pins panel amendment 2: two lanes merging
// equal-rev lines for one slug degrade to last-line-wins WITH a named
// warning — never a load refusal that bricks the whole registry.
func TestBT098_RegistryLastLineWins(t *testing.T) {
	dir := t.TempDir()
	lines := `{"slug":"release","display":"Release (old)","rev":1}` + "\n" +
		`{"slug":"release","display":"Release (new)","rev":1}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "types.jsonl"), []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := LoadTicketTypes(dir)
	if err != nil {
		t.Fatalf("equal-rev duplicate slug must load last-line-wins, got %v", err)
	}
	if got := reg.Types["release"].Display; got != "Release (new)" {
		t.Fatalf("last line did not win: display = %q", got)
	}
	if len(reg.Warnings) != 1 || !strings.Contains(reg.Warnings[0], `slug "release" repeats rev 1; last line wins`) {
		t.Fatalf("equal-rev collision must warn: %v", reg.Warnings)
	}
	if !strings.Contains(reg.DoctorText(), "config health: WARN") {
		t.Fatalf("doctor must surface the collision:\n%s", reg.DoctorText())
	}

	// An UNPARSEABLE line is still a hard load refusal — corrupt config is not
	// a merge artifact.
	broken := filepath.Join(t.TempDir(), "types.jsonl")
	if err := os.WriteFile(broken, []byte(`{"slug":"release"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTicketTypes(filepath.Dir(broken)); err == nil ||
		!strings.Contains(err.Error(), "types.jsonl line 1") ||
		!strings.Contains(err.Error(), "fix the file and re-run (nothing written)") {
		t.Fatalf("corrupt types.jsonl refusal = %v", err)
	}
}

// --- AC-3: template applied + metadata lands at detail.metadata -------------

// TestBT098_CreateTemplateAndMetadata pins AC-3 against the raw appended line:
// the prefix, the template priority, the AC skeleton and both metadata values
// must all be in the BYTES, and no other row may move.
func TestBT098_CreateTemplateAndMetadata(t *testing.T) {
	b := bt098Seed(t)
	before := bt098File(t, b.TasksPath())

	if _, err := b.Create(TaskRowSpec{
		ID:       "REL-TEST-2",
		Title:    "R1",
		Type:     "release",
		HasType:  true,
		Metadata: map[string]string{"channel": "beta", "source_commit": "abc1234", "signoffs": "a@x, b@x"},
		Status:   "pending",
	}); err != nil {
		t.Fatalf("create release row: %v", err)
	}

	row := bt098LastRow(t, b)
	for _, want := range []string{
		`"id":"REL-TEST-2"`,
		`"type":"release"`,
		`"title":"[RELEASE] R1"`,
		`"priority":"P1"`,
		`"acceptance_criteria":"- [ ] artifact built and signed\n- [ ] deploy notes written\n- [ ] rollback path verified"`,
		`"channel":"beta"`,
		`"source_commit":"abc1234"`,
		`"signoffs":["a@x","b@x"]`,
	} {
		if !strings.Contains(row, want) {
			t.Fatalf("created row missing %s:\n%s", want, row)
		}
	}
	// Metadata is a container at detail.metadata, never loose top-level keys
	// (§2.5) — a top-level "channel" would re-create the canon drift.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(row), &parsed); err != nil {
		t.Fatalf("created row is not JSON: %v", err)
	}
	if _, loose := parsed["channel"]; loose {
		t.Fatalf("metadata key leaked to a top-level row key:\n%s", row)
	}
	detail, ok := parsed["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail is not an object:\n%s", row)
	}
	if _, ok := detail["metadata"].(map[string]any); !ok {
		t.Fatalf("detail.metadata is not an object:\n%s", row)
	}
	// Append-only: the pre-existing row is byte-for-byte untouched.
	if !strings.HasPrefix(bt098File(t, b.TasksPath()), before) {
		t.Fatal("create rewrote an existing row instead of appending")
	}
}

// --- AC-4: field-by-field refusal -------------------------------------------

// TestBT098_CreateRefusalFieldByField pins AC-4: BOTH missing fields in ONE
// refusal (never fail-first), in declaration order, with the spec's exact
// text — and nothing written.
func TestBT098_CreateRefusalFieldByField(t *testing.T) {
	b := bt098Seed(t)
	tasks, events := bt098Snapshot(t, b)

	_, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "R1", Type: "release", HasType: true, Status: "pending"})
	if err == nil {
		t.Fatal("create without required metadata was accepted")
	}
	want := "create refused: type \"release\" requires metadata fields that are missing:\n" +
		"  - channel (enum: stable|beta|nightly) — required: release channel this artifact ships to\n" +
		"  - source_commit (string) — required: commit the release was cut from\n" +
		"supply each with --meta field=value (repeat the flag per field); nothing was written"
	if err.Error() != want {
		t.Fatalf("refusal text = %q\nwant %q", err.Error(), want)
	}
	bt098AssertUnchanged(t, b, tasks, events)

	// Half-supplied is still a refusal naming only the field actually missing.
	_, err = b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "R1", Type: "release", HasType: true,
		Metadata: map[string]string{"channel": "beta"}, Status: "pending"})
	if err == nil || !strings.Contains(err.Error(), `  - source_commit (string)`) || strings.Contains(err.Error(), `  - channel (`) {
		t.Fatalf("half-supplied refusal = %v", err)
	}
	bt098AssertUnchanged(t, b, tasks, events)
}

// --- AC-5: enum conformance --------------------------------------------------

// TestBT098_EnumConformance pins AC-5: a value outside the closed vocabulary
// names the field, the bad value AND the allowed set, and writes nothing.
func TestBT098_EnumConformance(t *testing.T) {
	b := bt098Seed(t)
	tasks, events := bt098Snapshot(t, b)

	_, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "R1", Type: "release", HasType: true,
		Metadata: map[string]string{"channel": "rc", "source_commit": "abc1234"}, Status: "pending"})
	if err == nil {
		t.Fatal("out-of-vocabulary enum value was accepted")
	}
	want := "create refused: type \"release\" metadata values are invalid:\n" +
		"  - channel=\"rc\" (allowed: stable|beta|nightly)\n" +
		"nothing was written"
	if err.Error() != want {
		t.Fatalf("enum refusal text = %q\nwant %q", err.Error(), want)
	}
	bt098AssertUnchanged(t, b, tasks, events)

	// A ref field pointing at a nonexistent task id is refused the same way
	// (§6: a self-approval and a missing id are both not approvals).
	_, err = b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "R1", Type: "release", HasType: true,
		Metadata: map[string]string{"channel": "beta", "source_commit": "abc1234", "approval_ticket": "NOPE-9"},
		Status:   "pending"})
	if err == nil || !strings.Contains(err.Error(), `approval_ticket="NOPE-9" (task id does not exist)`) {
		t.Fatalf("dangling ref refusal = %v", err)
	}
	bt098AssertUnchanged(t, b, tasks, events)
}

// --- AC-6: update protects required fields ----------------------------------

// TestBT098_UpdateProtectsRequired pins AC-6: the only way update can drop a
// required field is `--meta channel=` (empty), which must refuse with the
// spec's text and leave the row's metadata untouched on disk.
func TestBT098_UpdateProtectsRequired(t *testing.T) {
	b := bt098Seed(t)
	tasks, events := bt098Snapshot(t, b)

	empty := map[string]string{"channel": ""}
	if _, err := b.UpdateTask("REL-TEST-1", UpdateSpec{Metadata: &empty}); err == nil {
		t.Fatal("dropping a required metadata field was accepted")
	} else {
		want := `update refused: "channel" is REQUIRED by type "release" (enum: stable|beta|nightly) — supply a value or change the row's type first`
		if err.Error() != want {
			t.Fatalf("required-field refusal = %q\nwant %q", err.Error(), want)
		}
	}
	bt098AssertUnchanged(t, b, tasks, events)

	// A conformant value change is a plain rewrite: exactly one row moves and
	// the untouched row round-trips byte-identically. (A metadata-only update
	// appends no event — the historical flag-only update discipline; the
	// event stream is asserted unchanged here and gains its row in the
	// status-update arm of the zero-migration diff test.)
	if _, err := b.UpdateTask("REL-TEST-1", UpdateSpec{Metadata: &map[string]string{"channel": "stable"}}); err != nil {
		t.Fatalf("conformant metadata update: %v", err)
	}
	if row := bt098RowByID(t, b, "REL-TEST-1"); !strings.Contains(row, `"channel":"stable"`) || !strings.Contains(row, `"source_commit":"abc1234"`) {
		t.Fatalf("conformant update did not merge metadata:\n%s", row)
	}
	if got := bt098File(t, b.EventsPath()); got != events {
		t.Fatalf("metadata-only update appended an event (existing discipline is none):\n--- before\n%s\n--- after\n%s", events, got)
	}
}

// --- AC-7: unknown metadata key warns ---------------------------------------

// TestBT098_UnknownMetaWarns pins AC-7: `--meta weirdkey=v` SUCCEEDS, writes
// the key, records the spec's warning for the CLI to render on stderr, and
// validate reports the same drift as a WARNING (never an error).
func TestBT098_UnknownMetaWarns(t *testing.T) {
	b := bt098Seed(t)

	if _, err := b.UpdateTask("REL-TEST-1", UpdateSpec{Metadata: &map[string]string{"weirdkey": "v"}}); err != nil {
		t.Fatalf("undeclared metadata key must warn-and-write, got %v", err)
	}
	warnings := b.MetadataWarnings()
	want := `metadata key "weirdkey" is not declared by type "release" — it was written, but 'boardctl validate' will report it`
	if len(warnings) != 1 || warnings[0] != want {
		t.Fatalf("metadata warnings = %q, want [%q]", warnings, want)
	}
	if row := bt098RowByID(t, b, "REL-TEST-1"); !strings.Contains(row, `"weirdkey":"v"`) {
		t.Fatalf("the undeclared key was not written:\n%s", row)
	}

	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rep.HasErrors() {
		t.Fatalf("undeclared metadata key must never be an ERROR:\n%s", rep.RenderText())
	}
	found := false
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, `has undeclared metadata key "weirdkey"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("validate did not report the undeclared key:\n%s", rep.RenderText())
	}

	// A clean update clears the warning — it is per-write state, never sticky.
	if _, err := b.UpdateTask("REL-TEST-1", UpdateSpec{Metadata: &map[string]string{"channel": "beta"}}); err != nil {
		t.Fatalf("follow-up update: %v", err)
	}
	if got := b.MetadataWarnings(); len(got) != 0 {
		t.Fatalf("warnings leaked between updates: %q", got)
	}
}

// --- DF-BOARDCTL-58: create warns on undeclared metadata too ------------------

// TestBT098_CreateUndeclaredMetaWarns pins DF-BOARDCTL-58: `create --type bug
// --meta severity=x` — an undeclared key on a type whose metadata list is
// EMPTY (the built-ins) — used to write the key SILENTLY while the same key
// on update warned. Create now warns with the update path's exact text and
// the key is still written (advisory, never a refusal). Per-write state: a
// clean create clears the warning.
//
// NOTE: validate's BT-098 row check only runs when types.jsonl is present
// (validate.go gates on reg.Present — pre-existing built-in-registry
// behaviour), so the validate-still-fires half of BT-098 AC-7 is asserted on
// the registry-present board in TestBT098_CreateUndeclaredMetaWarnsValidate.
func TestBT098_CreateUndeclaredMetaWarns(t *testing.T) {
	dir := t.TempDir()
	// No types.jsonl on purpose: the built-in registry (task/bug/feat/chore)
	// is exactly the empty-metadata-list case the defect describes.
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl":  `{"id":"BASE-1","title":"Existing row","status":"pending","priority":"P2","complexity":3,"created_at":"2026-11-01T00:00:00+00:00","updated_at":"2026-11-01T00:00:00+00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-11-01 00:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n",
		"board.jsonl":  `{"project":"bt098","namespace":"bt098","version":1,"ticks_total":1,"ticks_idle":0,"last_commit":null}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("resolve seeded board: %v", err)
	}

	if _, err := b.Create(TaskRowSpec{ID: "BUG-TEST-1", Title: "T", Type: "bug", HasType: true,
		Metadata: map[string]string{"severity": "x"}, Status: "pending"}); err != nil {
		t.Fatalf("undeclared metadata key must warn-and-write on create, got %v", err)
	}
	want := `metadata key "severity" is not declared by type "bug" — it was written, but 'boardctl validate' will report it`
	if warnings := b.MetadataWarnings(); len(warnings) != 1 || warnings[0] != want {
		t.Fatalf("metadata warnings = %q, want [%q]", warnings, want)
	}
	if row := bt098RowByID(t, b, "BUG-TEST-1"); !strings.Contains(row, `"severity":"x"`) {
		t.Fatalf("the undeclared key was not written:\n%s", row)
	}

	// Per-write state, never sticky: a clean create emits nothing.
	if _, err := b.Create(TaskRowSpec{ID: "BUG-TEST-2", Title: "T2", Type: "bug", HasType: true, Status: "pending"}); err != nil {
		t.Fatalf("follow-up create: %v", err)
	}
	if got := b.MetadataWarnings(); len(got) != 0 {
		t.Fatalf("warnings leaked between creates: %q", got)
	}
}

// TestBT098_CreateUndeclaredMetaWarnsValidate pins DF-BOARDCTL-58 AC-3 on a
// registry-present board: after a CREATE writes an undeclared key, validate's
// existing BT-098 warning for the same row still fires (warn, never error) —
// the create-side warning is additive to validate, not a substitute.
func TestBT098_CreateUndeclaredMetaWarnsValidate(t *testing.T) {
	b := bt098Seed(t)

	if _, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "R2", Type: "release", HasType: true,
		Metadata: map[string]string{"channel": "beta", "source_commit": "abc1234", "weirdkey": "v"}, Status: "pending"}); err != nil {
		t.Fatalf("undeclared metadata key must warn-and-write on create, got %v", err)
	}
	want := `metadata key "weirdkey" is not declared by type "release" — it was written, but 'boardctl validate' will report it`
	if warnings := b.MetadataWarnings(); len(warnings) != 1 || warnings[0] != want {
		t.Fatalf("metadata warnings = %q, want [%q]", warnings, want)
	}
	if row := bt098RowByID(t, b, "REL-TEST-2"); !strings.Contains(row, `"weirdkey":"v"`) {
		t.Fatalf("the undeclared key was not written:\n%s", row)
	}

	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rep.HasErrors() {
		t.Fatalf("undeclared metadata key must never be an ERROR:\n%s", rep.RenderText())
	}
	found := false
	for _, f := range rep.Findings {
		if f.Level == "warn" && strings.Contains(f.Msg, `has undeclared metadata key "weirdkey"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("validate did not report the undeclared key:\n%s", rep.RenderText())
	}
}

// --- AC-8: validate conformance ----------------------------------------------

// TestBT098_ValidateConformance pins AC-8: a hand-edited row missing a
// required field is an ERROR finding naming the field and its type contract,
// and the report fails.
func TestBT098_ValidateConformance(t *testing.T) {
	b := bt098Seed(t)
	// Hand-edit: drop channel from the row's metadata (an operator fix-by-hand
	// that forgot a required field).
	stripped := `{"id":"REL-TEST-1","title":"[RELEASE] Existing release","type":"release","status":"pending","priority":"P1","complexity":3,"created_at":"2026-11-01T00:00:00+00:00","updated_at":"2026-11-01T00:00:00+00:00","detail":{"metadata":{"source_commit":"abc1234"}}}` + "\n"
	if err := os.WriteFile(b.TasksPath(), []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := b.Validate()
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !rep.HasErrors() {
		t.Fatalf("missing required metadata must FAIL validate:\n%s", rep.RenderText())
	}
	want := `BT-098: row REL-TEST-1 (type "release") is missing required metadata field "channel" (enum: stable|beta|nightly)`
	found := false
	for _, f := range rep.Findings {
		if f.Level == "error" && f.Msg == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("validate findings missing %q:\n%s", want, rep.RenderText())
	}

	// A row with NO type key is legacy tolerance: zero findings (the fleet's
	// 219 rows must stay validate-clean).
	legacy := `{"id":"LEG-1","title":"untyped","status":"pending","priority":"P2"}` + "\n"
	if err := os.WriteFile(b.TasksPath(), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err = b.Validate()
	if err != nil {
		t.Fatalf("validate legacy board: %v", err)
	}
	for _, f := range rep.Findings {
		if f.Class == "type-conformance" {
			t.Fatalf("untyped row produced a type-conformance finding: %s", f.Msg)
		}
	}
}

// --- AC-10: explicit flags win ----------------------------------------------

// TestBT098_ExplicitFlagsWin pins AC-10 and §3.1: an explicit priority beats
// the template, an already-prefixed title is not doubled, and an explicit
// acceptance-criteria REPLACES the skeleton instead of being appended to it.
func TestBT098_ExplicitFlagsWin(t *testing.T) {
	b := bt098Seed(t)
	ac := "my own criteria"

	if _, err := b.Create(TaskRowSpec{
		ID:                 "REL-TEST-2",
		Title:              "[RELEASE] already prefixed",
		Type:               "release",
		HasType:            true,
		Metadata:           map[string]string{"channel": "nightly", "source_commit": "def5678"},
		Status:             "pending",
		Priority:           "P3",
		AcceptanceCriteria: &ac,
	}); err != nil {
		t.Fatalf("create with explicit flags: %v", err)
	}

	row := bt098LastRow(t, b)
	if strings.Count(row, "[RELEASE] ") != 1 {
		t.Fatalf("title prefix was doubled:\n%s", row)
	}
	if !strings.Contains(row, `"priority":"P3"`) {
		t.Fatalf("template priority overrode the explicit --priority:\n%s", row)
	}
	if !strings.Contains(row, `"acceptance_criteria":"my own criteria"`) {
		t.Fatalf("template skeleton overrode/merged with the explicit criteria:\n%s", row)
	}
	if strings.Contains(row, "rollback path verified") {
		t.Fatalf("template skeleton leaked into a row with explicit criteria:\n%s", row)
	}
}

// --- AC-11: retired types ---------------------------------------------------

// TestBT098_RetiredType pins AC-11: a retired slug leaves the active census,
// is listed only under `retired:`, and `create --type` refuses it. Existing
// rows claiming it keep validating (resolvable, not fileable).
func TestBT098_RetiredType(t *testing.T) {
	b := bt098Seed(t)
	retired := `{"slug":"legacy","display":"Legacy","rev":1,"retired":true}` + "\n"
	if err := os.WriteFile(filepath.Join(b.Dir, "types.jsonl"), []byte(bt098ReleaseTypeJSON(t)+"\n"+retired), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := LoadTicketTypes(b.Dir)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if got, want := reg.Names(), "release"; got != want {
		t.Fatalf("active census = %q, want %q", got, want)
	}
	if active := reg.Sorted(true); len(active) != 1 || active[0].Slug != "release" {
		t.Fatalf("retired type leaked into the active list: %v", active)
	}
	text := reg.RenderText()
	if !strings.Contains(text, "\nretired:\n  legacy (Legacy)\n") {
		t.Fatalf("retired block missing:\n%s", text)
	}
	if strings.Contains(strings.SplitN(text, "retired:", 2)[0], "legacy") {
		t.Fatalf("retired type listed among the active types:\n%s", text)
	}

	tasks, events := bt098Snapshot(t, b)
	_, err = b.Create(TaskRowSpec{ID: "LEG-2", Title: "legacy row", Type: "legacy", HasType: true, Status: "pending"})
	if err == nil || err.Error() != `create refused: ticket type "legacy" is retired in types.jsonl — pick an active type` {
		t.Fatalf("retired-type create refusal = %v", err)
	}
	bt098AssertUnchanged(t, b, tasks, events)
}

// --- create-side unknown type ------------------------------------------------

// TestBT098_UnknownTypeRefused: an unknown slug names the configured set so
// the operator does not have to go read types.jsonl.
func TestBT098_UnknownTypeRefused(t *testing.T) {
	b := bt098Seed(t)
	tasks, events := bt098Snapshot(t, b)
	_, err := b.Create(TaskRowSpec{ID: "X-1", Title: "shipit", Type: "shipit", HasType: true, Status: "pending"})
	if err == nil {
		t.Fatal("unknown ticket type was accepted")
	}
	want := `create refused: unknown ticket type "shipit" — configured types: release (run 'boardctl types' to list them)`
	if err.Error() != want {
		t.Fatalf("unknown-type refusal = %q\nwant %q", err.Error(), want)
	}
	bt098AssertUnchanged(t, b, tasks, events)
}

// TestBT098_TypeRegistrySHAStable is a guard on the fixture itself: the
// zero-migration suite pins this digest, so an accidental edit to the
// testdata board fails HERE (naming the fixture) instead of in three diff
// assertions that all report the same opaque mismatch.
func TestBT098_TypeRegistrySHAStable(t *testing.T) {
	if got := bt098SHA(t, filepath.Join("testdata", "bt098", "types.jsonl")); got == "" {
		t.Fatal("unreachable")
	}
	reg, err := LoadTicketTypes(filepath.Join("testdata", "bt098"))
	if err != nil {
		t.Fatalf("load pinned fixture registry: %v", err)
	}
	if !reg.Present || len(reg.Warnings) != 0 {
		t.Fatalf("pinned fixture registry must be present and warning-free: present=%v warnings=%v", reg.Present, reg.Warnings)
	}
	if got := reg.Names(); got != "bug, release" {
		t.Fatalf("pinned fixture registry = %q, want %q", got, "bug, release")
	}
}
