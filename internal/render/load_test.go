package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

// seedBoard writes a board under t.TempDir()-based root and returns the -C
// target. files maps file name -> content (nil map value = skip file).
func seedBoard(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	bd := filepath.Join(root, ".coding-hermes", "board")
	if err := os.MkdirAll(bd, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(bd, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const topoAHeader = `{"project": "demo", "namespace": "ns", "version": 1, "ticks_total": 69, "ticks_idle": 12, "last_commit": null}` + "\n"
const emptyEvents = ""

func resolveSeed(t *testing.T, root string) *board.Board {
	t.Helper()
	b, err := board.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return b
}

func loadSeed(t *testing.T, root string) (*boardData, *board.Board) {
	t.Helper()
	b := resolveSeed(t, root)
	d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	return d, b
}

// ---------- topology A happy path + fixture exclusion ----------

func TestLoadTopologyAWithFixtureExclusion(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" +
			`{"id": "NEVER-DONE", "title": "fixture", "status": "pending", "active": true, "created_at": "2026-09-01 00:00:00"}` + "\n",
		"events.jsonl":   emptyEvents,
		"fixtures.jsonl": `{"id":"NEVER-DONE","title":"fixture","status":"pending","active":true,"created_at":"2026-09-01 00:00:00"}` + "\n",
	})
	d, b := loadSeed(t, root)
	if b.Topology != "A" {
		t.Fatalf("topology = %s", b.Topology)
	}
	if d.name != "demo" {
		t.Fatalf("name = %q", d.name)
	}
	if len(d.tasks) != 2 {
		t.Fatalf("tasks = %d", len(d.tasks))
	}
	byID := map[string]bool{}
	for _, rec := range d.tasks {
		byID[rec.id] = rec.fixture
	}
	if !byID["NEVER-DONE"] {
		t.Fatal("NEVER-DONE row must classify fixture (membership + prefix)")
	}
	if byID["A"] {
		t.Fatal("task A must NOT classify fixture")
	}
	if d.warns.total() != 0 {
		t.Fatalf("warnings = %+v", d.warns)
	}
}

// ---------- topology B ----------

func TestLoadTopologyBHeaderShape(t *testing.T) {
	root := seedBoard(t, map[string]string{
		// NO board.jsonl: header is line 1 of tasks.jsonl (header shape:
		// no id + project/version/ticks_total).
		"tasks.jsonl": `{"project": "legacy", "namespace": "lns", "version": 2, "ticks_total": 69, "ticks_idle": 3}` + "\n" +
			taskLine("B1", "2026-09-02 00:00:00", "2026-09-03 00:00:00", "complete") + "\n",
		"events.jsonl": emptyEvents,
	})
	d, b := loadSeed(t, root)
	if b.Topology != "B" {
		t.Fatalf("topology = %s", b.Topology)
	}
	if d.header == nil || d.header.IntOrZero("ticks_total") != 69 {
		t.Fatalf("header = %v, want ticks_total 69", d.header)
	}
	if len(d.tasks) != 1 || d.tasks[0].id != "B1" {
		t.Fatalf("tasks = %v — header must not load as a task", d.tasks)
	}
	if d.name != "legacy" {
		t.Fatalf("name = %q", d.name)
	}
}

func TestTopologyBHeaderNotAssumedOnLine1(t *testing.T) {
	// A tasks.jsonl whose line 1 IS a task (no header anywhere) must load
	// both rows — shape detection, never "first line always".
	root := seedBoard(t, map[string]string{
		"tasks.jsonl": taskLine("T1", "2026-09-01 00:00:00", "", "pending") + "\n" +
			taskLine("T2", "2026-09-01 00:00:00", "", "pending") + "\n",
		"events.jsonl": emptyEvents,
	})
	d, b := loadSeed(t, root)
	if b.Topology != "B" {
		t.Fatalf("topology = %s", b.Topology)
	}
	if d.header != nil {
		t.Fatal("header should be absent")
	}
	if len(d.tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(d.tasks))
	}
	if d.name == "" {
		t.Fatal("name must fall back to the directory name")
	}
}

// ---------- missing / empty inputs (2.2) ----------

func TestEmptyBoardRenders(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  emptyEvents,
		"events.jsonl": emptyEvents,
	})
	d, _ := loadSeed(t, root)
	if len(d.tasks) != 0 || len(d.events) != 0 {
		t.Fatalf("tasks/events = %d/%d, want 0/0", len(d.tasks), len(d.events))
	}
	der := derive(d)
	if der.Burndown.Open != nil {
		t.Fatalf("empty burndown = %+v", der.Burndown)
	}
	if der.Streaks.Delivery.Current != 0 || der.Streaks.Activity.Current != 0 {
		t.Fatalf("streaks = %+v", der.Streaks)
	}
}

func TestNoHeaderAnywhereUsesDirname(t *testing.T) {
	// Topology A without a parseable board.jsonl header row: project falls
	// back to the directory name and ticks are absent (not 0).
	root := seedBoard(t, map[string]string{
		"board.jsonl":  "not json at all\n",
		"tasks.jsonl":  taskLine("A", "2026-09-01 00:00:00", "", "pending") + "\n",
		"events.jsonl": emptyEvents,
	})
	b := resolveSeed(t, root)
	d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	if d.header != nil {
		t.Fatal("header should be nil for unparseable board.jsonl")
	}
	if d.name != "board" {
		t.Fatalf("name = %q, want dir basename", d.name)
	}
}

// ---------- torn lines, blank lines, duplicate ids (2.7) ----------

func TestTornLinesWarnAndSkip(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": taskLine("A", "2026-09-01 00:00:00", "", "pending") + "\n" +
			`{"id":"X","title":"torn"` + "\n" + // torn tail (AC 7.1.5 shape)
			"\n" + // blank line: silent
			taskLine("B", "2026-09-02 00:00:00", "", "pending") + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 10:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n" +
			`{"id":2,"timestamp":"2026-09-0`, // torn event
	})
	d, _ := loadSeed(t, root)
	if d.warns.tasks != 1 || d.warns.events != 1 {
		t.Fatalf("warnings = %+v, want 1 torn task + 1 torn event", d.warns)
	}
	if d.warns.total() != 2 || len(d.warns.notes) != 2 {
		t.Fatalf("total/notes = %d/%d", d.warns.total(), len(d.warns.notes))
	}
	ids := map[string]bool{}
	for _, rec := range d.tasks {
		ids[rec.id] = true
	}
	if ids["X"] {
		t.Fatal("torn row X must not load")
	}
	if !ids["A"] || !ids["B"] {
		t.Fatalf("good rows lost: %v", ids)
	}
}

func TestRowsWithoutIdWarnAndSkip(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": `{"title":"no id here","status":"pending"}` + "\n" +
			taskLine("A", "2026-09-01 00:00:00", "", "pending") + "\n",
		"events.jsonl": emptyEvents,
	})
	d, _ := loadSeed(t, root)
	if d.warns.tasks != 1 {
		t.Fatalf("warnings = %+v", d.warns)
	}
	if len(d.tasks) != 1 || d.tasks[0].id != "A" {
		t.Fatalf("tasks = %v", d.tasks)
	}
}

func TestDuplicateTaskIDsLastWins(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": taskLine("DUP", "2026-09-01 00:00:00", "", "pending") + "\n" +
			`{"id":"DUP","title":"re-filed","status":"complete","priority":"P0","created_at":"2026-09-01 00:00:00","completed_at":"2026-09-05 00:00:00"}` + "\n",
		"events.jsonl": emptyEvents,
	})
	d, _ := loadSeed(t, root)
	if d.warns.dupIDs != 1 {
		t.Fatalf("dup warnings = %d", d.warns.dupIDs)
	}
	if len(d.tasks) != 1 {
		t.Fatalf("tasks = %d, want 1 (collapsed)", len(d.tasks))
	}
	if d.tasks[0].status != "complete" {
		t.Fatalf("last row did not win: status %s", d.tasks[0].status)
	}
}

func TestDuplicateEventIDsKept(t *testing.T) {
	ev := `{"id":1,"timestamp":"2026-09-03 10:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n"
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  emptyEvents,
		"events.jsonl": ev + ev, // same id twice: both are facts
	})
	d, _ := loadSeed(t, root)
	if len(d.events) != 2 {
		t.Fatalf("events = %d, want 2 (never deduped)", len(d.events))
	}
	if d.warns.dupIDs != 0 {
		t.Fatalf("dup warnings = %d, want 0", d.warns.dupIDs)
	}
}

func TestSparseWrongTypedKeysDoNotCrash(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  `{"id":"W1","title":null,"status":42,"attempts":"three","priority":[1],"created_at":null,"perpetual":"yes"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":12345,"event_type":{"nested":true},"task_id":null,"actor":7,"detail":null,"tick_number":"2"}` + "\n",
	})
	d, _ := loadSeed(t, root)
	if len(d.tasks) != 1 || len(d.events) != 1 {
		t.Fatalf("rows lost: %d/%d", len(d.tasks), len(d.events))
	}
	// wrong-typed timestamps are excluded from time metrics with a warning
	if d.warns.tsExcl != 2 {
		t.Fatalf("tsExcl = %d, want 2", d.warns.tsExcl)
	}
	// wrong-typed status decodes to "(none)" via String()=="" semantics
	if d.tasks[0].status != "(none)" {
		t.Fatalf("status = %q", d.tasks[0].status)
	}
	// perpetual:"yes" (wrong type) must NOT classify as fixture
	if d.tasks[0].fixture {
		t.Fatal("wrong-typed perpetual must not classify fixture")
	}
}

func TestTimestampWarningsNameIDs(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  `{"id":"BADTS","title":"x","status":"pending","created_at":"soonish"}` + "\n",
		"events.jsonl": `{"id":9,"timestamp":"yesterday-ish","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n",
	})
	d, _ := loadSeed(t, root)
	if d.warns.tsExcl != 2 {
		t.Fatalf("tsExcl = %d", d.warns.tsExcl)
	}
	joined := strings.Join(d.warns.notes, "\n")
	if !strings.Contains(joined, "BADTS") || !strings.Contains(joined, "created_at") {
		t.Fatalf("task warning must name id+field: %s", joined)
	}
	if !strings.Contains(joined, "9") || !strings.Contains(joined, "timestamp") {
		t.Fatalf("event warning must name id+field: %s", joined)
	}
	// the row still renders (loaded)
	if len(d.tasks) != 1 {
		t.Fatalf("tasks = %d", len(d.tasks))
	}
}

func TestWarningNotesCappedAt20(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(`{"torn` + string(rune('a'+i%26)) + `"` + "\n")
	}
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  sb.String(),
		"events.jsonl": emptyEvents,
	})
	d, _ := loadSeed(t, root)
	if d.warns.tasks != 30 {
		t.Fatalf("tasks warnings = %d", d.warns.tasks)
	}
	if len(d.warns.notes) != 20 {
		t.Fatalf("notes = %d, want capped 20", len(d.warns.notes))
	}
}

func TestUnresolvableDirIsError(t *testing.T) {
	empty := t.TempDir()
	if _, err := board.Resolve(empty); err == nil {
		t.Fatal("resolve on empty dir should fail")
	}
}

// ---------- JSON payload round-trip (6.2) ----------

func TestPayloadRoundTripAndShapes(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" +
			`{"id": "NEVER-DONE", "status": "pending", "perpetual": true, "created_at": "2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-02 10:00:00","event_type":"task_completed","task_id":"A","actor":"f","detail":"{\"status\":\"complete\"}","tick_number":3}` + "\n",
	})
	rp, err := Build(root, Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if rp.Schema != SchemaName {
		t.Fatalf("schema = %q", rp.Schema)
	}
	island, err := EscapeJSONIsland(rp)
	if err != nil {
		t.Fatal(err)
	}
	var back ReportPayload
	if err := json.Unmarshal(island, &back); err != nil {
		t.Fatalf("island does not round-trip: %v", err)
	}
	bd := back.Boards[0]
	if len(bd.Tasks) != 2 || len(bd.Events) != 1 {
		t.Fatalf("raw rows = %d/%d", len(bd.Tasks), len(bd.Events))
	}
	// verbatim bytes: the task row round-trips with original key order
	var rawTask map[string]json.RawMessage
	if err := json.Unmarshal(bd.Tasks[0], &rawTask); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawTask["id"]; !ok {
		t.Fatal("raw task lost id")
	}
	if bd.Header == nil || !strings.Contains(string(bd.Header), `"ticks_total":69`) {
		t.Fatalf("header = %s", bd.Header)
	}
	if bd.Slug != "demo" || bd.Topology != "A" {
		t.Fatalf("slug/topology = %s/%s", bd.Slug, bd.Topology)
	}
	// derived sanity
	if bd.Derived.Burnup.Cum[len(bd.Derived.Burnup.Cum)-1] != 1 {
		t.Fatalf("burnup cum = %v", bd.Derived.Burnup.Cum)
	}
	if bd.Derived.Streaks.Delivery.Current != 1 && bd.Derived.Streaks.Delivery.Longest != 1 {
		t.Fatalf("delivery streak = %+v", bd.Derived.Streaks.Delivery)
	}
}

func TestBuildBoardNotFound(t *testing.T) {
	if _, err := Build(t.TempDir(), Options{}); err == nil {
		t.Fatal("Build on boardless dir should fail")
	}
}

// ---------- HTML + XSS ----------

func TestHTMLContainsSingleIslandAndEscapes(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  `{"id":"EVIL","title":"</script><img src=x onerror=alert(1)>","status":"pending","created_at":"2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": emptyEvents,
	})
	rp, err := Build(root, Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(rp)
	if err != nil {
		t.Fatal(err)
	}
	// exactly one JSON island
	if n := strings.Count(html, `<script type="application/json"`); n != 1 {
		t.Fatalf("island count = %d, want 1", n)
	}
	if n := strings.Count(html, "</script>"); n != 2 {
		t.Fatalf("closing script tags = %d, want 2 (island + app)", n)
	}
	// the hostile title is escaped inside the island: \u003c cannot
	// terminate a script element and onerror text is inert data.
	if !strings.Contains(html, `\u003c/script\u003e`) {
		t.Fatal("hostile </script> not escaped in island")
	}
	if strings.Contains(html, "<img src=x") {
		t.Fatal("raw hostile img tag leaked into HTML")
	}
	// escaping is lossless: rebuild payload bytes via Go decoder
	var back ReportPayload
	dec := json.NewDecoder(strings.NewReader(html[strings.Index(html, `{"schema"`):]))
	if err := dec.Decode(&back); err != nil {
		t.Fatalf("island unparseable: %v", err)
	}
	if back.Boards[0].Tasks[0] == nil {
		t.Fatal("task lost")
	}
}

func TestHTMLStaticSkeleton(t *testing.T) {
	rp := &ReportPayload{Schema: SchemaName, Boards: []BoardPayload{}}
	html, err := RenderHTML(rp)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!DOCTYPE html>", "<style>", "function ", "report-data",
		"Board Report", "aria-expanded", `setAttribute("role", "button")`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML missing %q", want)
		}
	}
	// no network in the inline JS
	for _, banned := range []string{"fetch(", "XMLHttpRequest", ".import("} {
		if strings.Contains(html, banned) {
			t.Fatalf("HTML contains banned network call %q", banned)
		}
	}
	// innerHTML is forbidden in the app script (6.3)
	if strings.Contains(strings.ToLower(html), "innerhtml") {
		t.Fatal("inline JS uses innerHTML")
	}
}

func TestDeterministicOutputExceptRenderedAt(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 10:00:00","event_type":"audit","task_id":null,"actor":"f","detail":null,"tick_number":null}` + "\n",
	})
	bog, err := time.LoadLocation("America/Bogota")
	if err != nil {
		t.Skip("no tzdata")
	}
	mk := func(renderedAt time.Time) string {
		rp, err := Build(root, Options{Now: renderedAt, Zone: bog})
		if err != nil {
			t.Fatal(err)
		}
		h, err := RenderHTML(rp)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	// same instant twice -> byte-identical
	a := mk(tstamp("2026-09-12 12:00:00"))
	b := mk(tstamp("2026-09-12 12:00:00"))
	if a != b {
		t.Fatal("two renders at the same instant must be byte-identical")
	}
	// different rendered_at: the only diff region is the rendered_at value
	// (AC 7.1.8: diff after sed'ing rendered_at is empty).
	c := mk(tstamp("2026-09-12 12:00:01"))
	stripped := func(s string) string {
		return regexpRenderedAt.ReplaceAllString(s, `"rendered_at":"X"`)
	}
	if stripped(a) != stripped(c) {
		t.Fatal("output drift beyond rendered_at")
	}
}

var regexpRenderedAt = regexp.MustCompile(`"rendered_at":"[^"]*"`)

func TestRenderedAtFormatIsRFC3339(t *testing.T) {
	rp, err := Build(".", Options{Zone: time.UTC})
	if err != nil {
		t.Skip("no board in cwd")
	}
	if _, err := time.Parse(time.RFC3339, rp.RenderedAt); err != nil {
		t.Fatalf("rendered_at %q not RFC3339: %v", rp.RenderedAt, err)
	}
}

// ---------- BT-020 remediation: 5.7 error state ----------

// TestUnparseableRequiredFileIsErrorState proves the 5.7 gap: a required
// file whose candidate rows ALL fail to parse must surface an error-state
// board (payload Error field) — not a valid empty board with warnings.
func TestUnparseableRequiredFileIsErrorState(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  "this is not json at all\n{{{ broken\n",
		"events.jsonl": emptyEvents,
	})
	b := resolveSeed(t, root)
	d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
	if err != nil {
		t.Fatalf("loadBoard must not fail the load: %v", err)
	}
	if d.fatal == "" {
		t.Fatal("wholly unparseable tasks.jsonl must set the 5.7 error state, got none")
	}
	if !strings.Contains(d.fatal, "tasks.jsonl") {
		t.Fatalf("fatal message must name the file: %q", d.fatal)
	}
	if !strings.Contains(d.fatal, "line 1") {
		t.Fatalf("fatal message must carry the first meaningful error (line 1): %q", d.fatal)
	}
	rp, err := buildFromBoard(b, Options{Now: tstamp("2026-09-12 12:00:00"), Zone: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if rp.Boards[0].Error == "" {
		t.Fatal("payload must carry Error for the unparseable board")
	}
}

// TestUnparseableEventsFileIsErrorState covers the events side of the
// threshold (both required files are covered by 5.7).
func TestUnparseableEventsFileIsErrorState(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl":  topoAHeader,
		"tasks.jsonl":  taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n",
		"events.jsonl": "\x00\x01 binary garbage\nnot json {oops\n",
	})
	b := resolveSeed(t, root)
	d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	if d.fatal == "" || !strings.Contains(d.fatal, "events.jsonl") {
		t.Fatalf("wholly unparseable events.jsonl must set the error state naming events.jsonl, got %q", d.fatal)
	}
}

// TestTornLineAmongValidRowsStaysWarning preserves torn-line tolerance:
// one malformed row among valid rows is a 2.7 parse WARNING, never the 5.7
// error state.
func TestTornLineAmongValidRowsStaysWarning(t *testing.T) {
	root := seedBoard(t, map[string]string{
		"board.jsonl": topoAHeader,
		"tasks.jsonl": taskLine("A", "2026-09-01 00:00:00", "2026-09-02 00:00:00", "complete") + "\n" +
			"torn tail line without closing brace {\n" +
			taskLine("B", "2026-09-02 00:00:00", "", "pending") + "\n",
		"events.jsonl": emptyEvents,
	})
	b := resolveSeed(t, root)
	d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
	if err != nil {
		t.Fatal(err)
	}
	if d.fatal != "" {
		t.Fatalf("one torn row among valid rows is a warning, not an error state; got %q", d.fatal)
	}
	if d.warns.tasks != 1 {
		t.Fatalf("torn row must count one parse warning, got %d", d.warns.tasks)
	}
	if len(d.tasks) != 2 {
		t.Fatalf("valid rows must survive: %d", len(d.tasks))
	}
}

// TestEmptyAndHeaderOnlyBoardsRemainValid pins the threshold's lower bound:
// empty files and header-only boards are VALID empty boards (2.2), never
// error states.
func TestEmptyAndHeaderOnlyBoardsRemainValid(t *testing.T) {
	cases := map[string]map[string]string{
		"empty files": {
			"board.jsonl":  topoAHeader,
			"tasks.jsonl":  "",
			"events.jsonl": "",
		},
		"header only": {
			"board.jsonl":  topoAHeader,
			"tasks.jsonl":  "\n",
			"events.jsonl": "\n\n",
		},
		"topology B header only": {
			"tasks.jsonl":  `{"project": "legacy", "namespace": "lns", "version": 2, "ticks_total": 1, "ticks_idle": 0}` + "\n",
			"events.jsonl": "",
		},
	}
	for name, files := range cases {
		root := seedBoard(t, files)
		b := resolveSeed(t, root)
		d, err := loadBoard(b, tstamp("2026-09-12 12:00:00"))
		if err != nil {
			t.Fatalf("%s: loadBoard: %v", name, err)
		}
		if d.fatal != "" {
			t.Fatalf("%s: empty/header-only board must be a valid empty board, got error state %q", name, d.fatal)
		}
	}
}
