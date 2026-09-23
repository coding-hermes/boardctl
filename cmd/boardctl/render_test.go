package main

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// seedRenderBoard writes a small topology-A board and returns the -C target.
func seedRenderBoard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"board.jsonl": `{"project": "rendertest", "namespace": "ns", "version": 1, "ticks_total": 5, "ticks_idle": 1, "last_commit": null}` + "\n",
		"tasks.jsonl": `{"id": "R-1", "title": "first", "status": "complete", "priority": "P1", "primary_model": "glm-5.3-flash", "created_at": "2026-09-01 00:00:00", "completed_at": "2026-09-02 00:00:00", "updated_at": "2026-09-02 00:00:00"}` + "\n" +
			`{"id": "R-2", "title": "second", "status": "pending", "priority": "P2", "created_at": "2026-09-03 00:00:00", "updated_at": "2026-09-03 00:00:00"}` + "\n" +
			`{"id": "NEVER-DONE", "title": "fixture", "status": "pending", "active": true, "perpetual": true, "created_at": "2026-09-01 00:00:00"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-02 00:00:00","event_type":"task_completed","task_id":"R-1","actor":"foreman","detail":"{\"status\":\"complete\"}","tick_number":1}` + "\n" +
			`{"id":2,"timestamp":"2026-09-03 00:30:00","event_type":"idle","task_id":null,"actor":"foreman","detail":null,"tick_number":2}` + "\n",
		"fixtures.jsonl": `{"id":"NEVER-DONE","status":"pending","active":true}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// boardHashes hashes every board file so tests can assert render mutated nothing.
func boardHashes(t *testing.T, dir string) map[string]string {
	t.Helper()
	bd := filepath.Join(dir, ".coding-hermes", "board")
	out := map[string]string{}
	entries, err := os.ReadDir(bd)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(bd, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		out[e.Name()] = string(sum[:])
	}
	return out
}

func TestCmdRenderWritesFileAndJSON(t *testing.T) {
	dir := seedRenderBoard(t)
	outHTML := filepath.Join(t.TempDir(), "r.html")
	outJSON := filepath.Join(t.TempDir(), "r.json")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--tz", "America/Bogota", "--json", outJSON}); code != 0 {
		t.Fatalf("render exit code = %d, want 0", code)
	}
	st, err := os.Stat(outHTML)
	if err != nil {
		t.Fatalf("html not written: %v", err)
	}
	if st.Size() < 50*1024 {
		t.Fatalf("html = %d bytes, want >50KB (spec guardrail)", st.Size())
	}
	raw, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if got := strings.Count(s, `<script type="application/json"`); got != 1 {
		t.Fatalf("island count = %d, want 1", got)
	}
	if !strings.Contains(s, "rendertest") {
		t.Fatal("board name missing from report")
	}
	if _, err := os.Stat(outJSON); err != nil {
		t.Fatalf("json not written: %v", err)
	}
	jraw, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jraw), `"schema":"board-report/v1"`) {
		t.Fatalf("json payload missing schema: %s", jraw[:120])
	}
	if !strings.Contains(string(jraw), `"report_timezone":"America/Bogota"`) {
		t.Fatal("json payload missing pinned timezone")
	}
}

func TestCmdRenderStdout(t *testing.T) {
	dir := seedRenderBoard(t)
	var got string
	var code int
	got, err := captureStdout(func() {
		code = run([]string{"-C", dir, "render", "-o", "-", "--tz", "UTC"})
	})
	if err != nil {
		t.Fatalf("captureStdout: %v", err)
	}
	if code != 0 {
		t.Fatalf("render stdout exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(got, "<!DOCTYPE html>") {
		t.Fatalf("stdout does not start with HTML: %q", got[:min(60, len(got))])
	}
	if strings.Count(got, "application/json") != 1 {
		t.Fatal("stdout report island count wrong")
	}
}

func TestCmdRenderBoardNotFoundExit2(t *testing.T) {
	if code := run([]string{"-C", t.TempDir(), "render", "-o", filepath.Join(t.TempDir(), "x.html")}); code != 2 {
		t.Fatalf("render on boardless dir exit code = %d, want 2", code)
	}
}

func TestCmdRenderBadTZExit1(t *testing.T) {
	dir := seedRenderBoard(t)
	if code := run([]string{"-C", dir, "render", "-o", filepath.Join(t.TempDir(), "x.html"), "--tz", "Not/AZone"}); code != 1 {
		t.Fatalf("render with bad tz exit code = %d, want 1", code)
	}
}

func TestCmdRenderUsageErrorExit2(t *testing.T) {
	dir := seedRenderBoard(t)
	// House exit-code contract (README + TestCmdExitCodes): unknown
	// commands and board-not-found exit 2; flag/arg errors are
	// operational errors and exit 1 like every other subcommand.
	if code := run([]string{"-C", dir, "render", "extra"}); code != 1 {
		t.Fatalf("render with positional arg exit code = %d, want 1", code)
	}
	if code := run([]string{"-C", dir, "render", "--nope", "x"}); code != 1 {
		t.Fatalf("render with unknown flag exit code = %d, want 1", code)
	}
	// usage-level failures stay 2: unknown command, boardless dir.
	if code := run([]string{"-C", dir, "renderx"}); code != 2 {
		t.Fatalf("unknown command exit code = %d, want 2", code)
	}
	if code := run([]string{"-C", t.TempDir(), "render"}); code != 2 {
		t.Fatalf("render on boardless dir exit code = %d, want 2", code)
	}
}

func TestCmdRenderNeverMutatesBoard(t *testing.T) {
	dir := seedRenderBoard(t)
	before := boardHashes(t, dir)
	outHTML := filepath.Join(t.TempDir(), "r.html")
	outJSON := filepath.Join(t.TempDir(), "r.json")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--json", outJSON, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render exit = %d", code)
	}
	after := boardHashes(t, dir)
	if len(before) != len(after) {
		t.Fatalf("board file set changed: %d -> %d", len(before), len(after))
	}
	for name, h := range before {
		if after[name] != h {
			t.Fatalf("board file %s mutated by render", name)
		}
	}
	// and the output landed OUTSIDE the board dir
	for name := range after {
		if name == "r.html" || name == "r.json" {
			t.Fatalf("render artifact written into board dir: %s", name)
		}
	}
}

func TestCmdRenderHostileTitleInert(t *testing.T) {
	// AC 7.1.7: exact hostile shape in a task title.
	dir := seedRenderBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	tasksPath := filepath.Join(boardDir, "tasks.jsonl")
	raw, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	hostile := `{"id": "EVIL", "title": "</script><img src=x onerror=alert(1)>", "status": "pending", "created_at": "2026-09-04 00:00:00"}` + "\n"
	if err := os.WriteFile(tasksPath, append(raw, []byte(hostile)...), 0o644); err != nil {
		t.Fatal(err)
	}
	outHTML := filepath.Join(t.TempDir(), "r.html")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render with hostile title exit = %d, want 0", code)
	}
	b, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if got := strings.Count(s, `<script type="application/json"`); got != 1 {
		t.Fatalf("island count = %d, want 1", got)
	}
	// the escaped island must not contain a live `</script>` sequence that
	// terminates early (escaped form is \u003c/script\u003e), and the raw
	// hostile markup must never appear OUTSIDE the island either.
	if !strings.Contains(s, `\u003c/script\u003e`) {
		t.Fatal("hostile </script> not escaped")
	}
	if strings.Contains(s, "<img src=x") {
		t.Fatal("hostile img markup leaked unescaped")
	}
	// verify the island terminates exactly once with the app's own closer
	// (a raw </script> inside the island would break the count of 2)
	if got := strings.Count(s, "</script>"); got != 2 {
		t.Fatalf("closing script tags = %d, want 2 (island + app) — raw </script> in island breaks this", got)
	}
	// the payload still parses (lossless escaping)
	island := s[strings.Index(s, `<script type="application/json" id="report-data">`)+len(`<script type="application/json" id="report-data">`):]
	island = island[:strings.Index(island, "</script>")]
	var probe map[string]any
	if err := json.NewDecoder(strings.NewReader(island)).Decode(&probe); err != nil {
		t.Fatalf("island unparseable after hostile title: %v", err)
	}
}

func TestCmdRenderDeterministicTZ(t *testing.T) {
	// AC 7.1.8 at the CLI level: same board + --tz twice differs only in
	// rendered_at. rendered_at is render-time, so run twice, strip, compare.
	dir := seedRenderBoard(t)
	a := filepath.Join(t.TempDir(), "a.html")
	b := filepath.Join(t.TempDir(), "b.html")
	if code := run([]string{"-C", dir, "render", "-o", a, "--tz", "America/Bogota"}); code != 0 {
		t.Fatalf("render a exit = %d", code)
	}
	if code := run([]string{"-C", dir, "render", "-o", b, "--tz", "America/Bogota"}); code != 0 {
		t.Fatalf("render b exit = %d", code)
	}
	ra, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	strip := func(s string) string {
		i := strings.Index(s, `"rendered_at":"`)
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], `","report_timezone"`)
		if j < 0 {
			return s
		}
		return s[:i] + `"rendered_at":"X"` + s[i+j+len(`","`):]
	}
	if strip(string(ra)) != strip(string(rb)) {
		t.Fatal("renders differ beyond rendered_at")
	}
}

// TestCmdRenderTornLineTail walks AC 7.1.5: a torn trailing row warns
// (payload parse_warnings.tasks == 1) and the X row never appears.
func TestCmdRenderTornLineTail(t *testing.T) {
	dir := seedRenderBoard(t)
	tasksPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	f, err := os.OpenFile(tasksPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"id":"X","title":"torn"`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	outJSON := filepath.Join(t.TempDir(), "r.json")
	outHTML := filepath.Join(t.TempDir(), "r.html")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--json", outJSON, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render with torn tail exit = %d, want 0", code)
	}
	jraw, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jraw), `"tasks":1`) || !strings.Contains(string(jraw), `"notes":[`) {
		t.Fatalf("parse warnings missing torn-task count: %s", jraw[:400])
	}
	hraw, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatal(err)
	}
	// the payload must not contain the torn row id X as a task id; the
	// warning banner machinery is present
	if !strings.Contains(string(hraw), "parse warning") {
		t.Fatal("warning banner copy missing")
	}
}

// TestCmdRenderEmptyBoard walks AC 7.1.4: init a fresh board, render must
// exit 0 and the payload must carry ONLY the seed fixture row (init writes
// the NEVER-DONE fixture per SCHED-GAP-106 — zero real tasks, zero events),
// with empty-state strings in the HTML.
func TestCmdRenderEmptyBoard(t *testing.T) {
	dir := t.TempDir()
	if err := cmdInit(dir, []string{"--project", "emptydemo"}); err != nil {
		t.Fatalf("init: %v", err)
	}
	outHTML := filepath.Join(t.TempDir(), "r.html")
	outJSON := filepath.Join(t.TempDir(), "r.json")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--json", outJSON, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render empty board exit = %d, want 0", code)
	}
	jraw, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Boards []struct {
			Tasks  []json.RawMessage `json:"tasks"`
			Events []json.RawMessage `json:"events"`
		} `json:"boards"`
	}
	if err := json.Unmarshal(jraw, &payload); err != nil {
		t.Fatalf("empty board payload unparseable: %v", err)
	}
	if len(payload.Boards) != 1 {
		t.Fatalf("boards = %d, want 1", len(payload.Boards))
	}
	bd := payload.Boards[0]
	// every seeded row is the NEVER-DONE fixture: zero real tasks, zero events
	if len(bd.Events) != 0 {
		t.Fatalf("events = %d, want 0 on a fresh init board", len(bd.Events))
	}
	for _, tr := range bd.Tasks {
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(tr, &probe); err != nil {
			t.Fatalf("seed row unparseable: %v", err)
		}
		if !strings.HasPrefix(probe.ID, "NEVER-DONE") {
			t.Fatalf("fresh init board carries non-fixture task %q", probe.ID)
		}
	}
	hraw, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatal(err)
	}
	h := string(hraw)
	for _, want := range []string{"no tasks yet", "no completions yet", "no events"} {
		if !strings.Contains(h, want) {
			t.Fatalf("empty-state string %q missing from HTML", want)
		}
	}
}

// TestCmdRenderTopologyB walks AC 7.1.3: copy to a temp board, delete
// board.jsonl, move the header to line 1 of tasks.jsonl -> render exits 0,
// payload keeps ticks_total and topology B.
func TestCmdRenderTopologyB(t *testing.T) {
	dir := seedRenderBoard(t)
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	// read the header from board.jsonl
	hdrRaw, err := os.ReadFile(filepath.Join(boardDir, "board.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	hdr := strings.TrimRight(string(hdrRaw), "\n")
	// delete board.jsonl, prepend header to tasks.jsonl
	if err := os.Remove(filepath.Join(boardDir, "board.jsonl")); err != nil {
		t.Fatal(err)
	}
	tasksRaw, err := os.ReadFile(filepath.Join(boardDir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(hdr+"\n"+string(tasksRaw)), 0o644); err != nil {
		t.Fatal(err)
	}
	outJSON := filepath.Join(t.TempDir(), "r.json")
	if code := run([]string{"-C", dir, "render", "-o", filepath.Join(t.TempDir(), "r.html"), "--json", outJSON, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render topology B exit = %d, want 0", code)
	}
	jraw, err := os.ReadFile(outJSON)
	if err != nil {
		t.Fatal(err)
	}
	s := string(jraw)
	if !strings.Contains(s, `"topology":"B"`) {
		t.Fatal("payload topology not B")
	}
	if !strings.Contains(s, `"ticks_total":5`) {
		t.Fatal("topology B payload lost the header ticks_total")
	}
	if !strings.Contains(s, `"project":"rendertest"`) {
		t.Fatal("topology B payload lost the project name")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestRenderBurndownUsesDays guards DF-BOARDCTL-8 (superseding the
// DF-BOARDCTL-1 pin): the burndown payload now carries per-day labels —
// series is {window bounds, days, open} — so the emitted chart JS must
// consume d.burndown.days. Feeding the 2-entry window bounds as the x-axis
// drew only 2 of the open[] points. The template is a Go string constant
// (template.go htmlTemplate) and the payload rides inline in the same HTML
// (report-data JSON island), so string assertions on the rendered document
// cover both halves. window[0] equals the oldest task birth day, i.e. the
// R-1 fixture's created_at.
func TestRenderBurndownUsesDays(t *testing.T) {
	dir := seedRenderBoard(t)
	outHTML := filepath.Join(t.TempDir(), "r.html")
	if code := run([]string{"-C", dir, "render", "-o", outHTML, "--tz", "UTC"}); code != 0 {
		t.Fatalf("render exit code = %d, want 0", code)
	}
	raw, err := os.ReadFile(outHTML)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)

	// 1. The payload island carries the burndown series under window/days/open.
	var payload struct {
		Boards []struct {
			Derived struct {
				Burndown struct {
					Window []string `json:"window"`
					Days   []string `json:"days"`
					Open   []int    `json:"open"`
				} `json:"burndown"`
			} `json:"derived"`
		} `json:"boards"`
	}
	m := regexp.MustCompile(`<script type="application/json" id="report-data">(.*?)</script>`).FindStringSubmatch(s)
	if m == nil {
		t.Fatal("report-data island not found")
	}
	if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
		t.Fatalf("payload island does not parse: %v", err)
	}
	if len(payload.Boards) != 1 {
		t.Fatalf("boards = %d, want 1", len(payload.Boards))
	}
	bd := payload.Boards[0].Derived.Burndown
	if len(bd.Open) < 1 {
		t.Fatal("burndown open series empty — fixture produced no window")
	}
	firstDay, lastOpen := bd.Window[0], bd.Open[len(bd.Open)-1]
	if firstDay != "2026-09-01" {
		t.Fatalf("burndown window[0] = %q, want 2026-09-01 (oldest fixture birth)", firstDay)
	}
	if lastOpen != 1 {
		t.Fatalf("burndown open[last] = %d, want 1 (only R-2 still open: R-1 completed 09-02, NEVER-DONE is fixture-excluded)", lastOpen)
	}
	if len(bd.Open) < 2 {
		t.Fatalf("burndown series length = %d, want >1 (window spans 2026-09-01..today)", len(bd.Open))
	}

	// 2. The payload's days[] now exists and is chart-shaped: one label per
	// open point, more than the 2 window bounds (DF-BOARDCTL-8 regression).
	if len(bd.Days) != len(bd.Open) {
		t.Fatalf("burndown days = %d, open = %d — x-axis would drop points", len(bd.Days), len(bd.Open))
	}
	if len(bd.Days) <= 2 {
		t.Fatalf("burndown days = %d, want > 2 (window-bounds-only regression)", len(bd.Days))
	}
	if bd.Days[0] != firstDay {
		t.Fatalf("burndown days[0] = %q, want %q", bd.Days[0], firstDay)
	}

	// 3. The chart renders: chartLines bails to emptyChart at runtime only
	// when the fed days[] is empty, so the emitted JS must feed
	// d.burndown.days at both call sites (separate box + overlay) — feeding
	// the 2-entry window bounds here was the DF-BOARDCTL-8 defect — and the
	// day keys the x-axis is drawn from must ride in the document
	// (report-data island — the browser draws the labels from them).
	if strings.Contains(s, "days: d.burndown.window") {
		t.Fatal(`template still feeds d.burndown.window as x-axis — stale 2-bound feed (DF-BOARDCTL-8)`)
	}
	if got := strings.Count(s, "days: d.burndown.days || []"); got != 2 {
		t.Fatalf("burndown days feeds = %d, want 2 (separate + overlay variants)", got)
	}
	for _, label := range []string{`{label: "burndown", empty: "no tasks yet"}`, `{label: "burndown + burn-up overlay", empty: "no tasks yet"}`} {
		if strings.Count(s, label) != 1 {
			t.Fatalf("burndown chart invocation %q missing from emitted report", label)
		}
	}
	for _, day := range []string{firstDay, bd.Window[len(bd.Window)-1]} {
		if !strings.Contains(s, day) {
			t.Fatalf("burndown x-axis label %q missing from emitted report", day)
		}
	}
	// The burndown open-count values must be the series data the chart JS
	// consumes (burndown.open feed, not burnup.cum).
	nBurndownFeeds := strings.Count(s, "values: d.burndown.open")
	if nBurndownFeeds != 2 {
		t.Fatalf("burndown open feeds = %d, want 2 (separate + overlay variants)", nBurndownFeeds)
	}
}
