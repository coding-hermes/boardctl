package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBoard writes a full topology-A board into dir.
func writeBoard(t *testing.T, dir string, tasks, events, header string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl":  tasks,
		"events.jsonl": events,
		"board.jsonl":  header,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// asceFixtureTasks is the asce shape: SIX distinct findings under ONE
// recycled id (QA-ASCE-1), plus one unrelated clean row.
func asceFixtureTasks() string {
	rows := []string{
		`{"id":"QA-ASCE-1","title":"[P2] header-frozen on asce main","reasoning":"cell detail: header never advances","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted","reasoning":"cell detail: no free worker slots","status":"in_progress","priority":"P2"}`,
		`{"id":"OTHER-1","title":"unrelated clean row","reasoning":"nothing to see","status":"complete","priority":"P3"}`,
		`{"id":"QA-ASCE-1","title":"[P2] port range exhausted","reasoning":"cell detail: 9200-9202 all bound","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] port range exhausted (recurrence 2)","reasoning":"cell detail: 9203-9205 all bound","status":"pending","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] bunker dropped the agent","reasoning":"cell detail: ssh session died mid-run","status":"blocked","priority":"P2"}`,
		`{"id":"QA-ASCE-1","title":"[P2] spawn pool exhausted on retry","reasoning":"cell detail: pool not released after crash","status":"pending","priority":"P2"}`,
	}
	return strings.Join(rows, "\n") + "\n"
}

const testEvents = `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}` + "\n"
const testHeader = `{"project":"asce","namespace":"asce","version":3,"ticks_total":1,"ticks_idle":0,"last_commit":"abc1234"}` + "\n"

// boardSnapshot hashes every tracked board file.
func boardSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	names := []string{"tasks.jsonl", "events.jsonl", "board.jsonl", "fixtures.jsonl"}
	out := map[string]string{}
	for _, name := range names {
		p := filepath.Join(dir, name)
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		out[name] = hex.EncodeToString(sum[:])
	}
	return out
}

func assertSnapshotsEqual(t *testing.T, before, after map[string]string) {
	t.Helper()
	for name, h := range before {
		if after[name] != h {
			t.Fatalf("%s was written (sha %s -> %s)", name, h, after[name])
		}
	}
	if len(before) != len(after) {
		t.Fatalf("snapshot file count changed: %d -> %d", len(before), len(after))
	}
}

// runCLI runs the real main entry point with args, capturing stdout into a
// buffer (run() writes its human report to os.Stdout directly).
func runCLI(t *testing.T, dir string, extra ...string) (code int, stdout string) {
	t.Helper()
	args := append([]string{"--board-dir", dir}, extra...)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldOut := os.Stdout
	os.Stdout = w
	code = run(args)
	os.Stdout = oldOut
	w.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return code, string(buf)
}

// AC5: the CLI-level path end-to-end on the asce-shaped fixture — dry-run
// plans the collapse and writes nothing; --apply closes lines 5-7 (keeping
// line 1) and appends exactly one audit event per closed line; a second
// --apply is a no-op.
func TestCLI_ByIDAsceShapeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, asceFixtureTasks(), testEvents, testHeader)

	// --- dry run
	code, out := runCLI(t, dir, "--by-id")
	if code != 0 {
		t.Fatalf("dry-run exit = %d, want 0\nstdout:\n%s", code, out)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Fatalf("dry-run output missing DRY-RUN marker:\n%s", out)
	}
	if !strings.Contains(out, "recycled ids: 1") {
		t.Fatalf("dry-run output does not report the recycled id:\n%s", out)
	}
	if !strings.Contains(out, "keep line 1") {
		t.Fatalf("dry-run output does not keep line 1:\n%s", out)
	}
	for _, ln := range []string{"line 5", "line 6", "line 7"} {
		if !strings.Contains(out, "supersede QA-ASCE-1 (line "+strings.TrimPrefix(ln, "line ")+")") && !strings.Contains(out, ln) {
			t.Fatalf("dry-run output missing %s:\n%s", ln, out)
		}
	}

	// --- apply
	code, out = runCLI(t, dir, "--by-id", "--apply")
	if code != 0 {
		t.Fatalf("apply exit = %d, want 0\nstdout:\n%s", code, out)
	}
	if !strings.Contains(out, "applied") {
		t.Fatalf("apply output missing applied marker:\n%s", out)
	}

	// rows: kept line 1 still pending; later QA-ASCE-1 lines complete;
	// OTHER-1 untouched (still complete, its own summary absent).
	raw, err := os.ReadFile(filepath.Join(dir, "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		var row map[string]any
		if err := json.Unmarshal([]byte(l), &row); err != nil {
			t.Fatalf("tasks.jsonl line does not parse: %v (%s)", err, l)
		}
		lines = append(lines, row)
	}
	if lines[0]["status"] != "pending" || lines[0]["title"] != "[P2] header-frozen on asce main" {
		t.Fatalf("kept line mutated: %v", lines[0])
	}
	for _, i := range []int{3, 4, 5, 6} {
		if lines[i]["status"] != "complete" {
			t.Fatalf("line %d status = %v, want complete", i+1, lines[i]["status"])
		}
		if lines[i]["superseded_by"] != "QA-ASCE-1" {
			t.Fatalf("line %d superseded_by = %v", i+1, lines[i]["superseded_by"])
		}
		if lines[i]["title"] == "" {
			t.Fatalf("line %d lost its title", i+1)
		}
	}
	if lines[2]["id"] != "OTHER-1" || lines[2]["status"] != "complete" {
		t.Fatalf("OTHER-1 changed: %v", lines[2])
	}
	// kept line 2 (in_progress -> closed): summary must name kept line 1
	if s, _ := lines[1]["worker_summary"].(string); !strings.Contains(s, "kept line 1") {
		t.Fatalf("line 2 worker_summary = %q, want it to name kept line 1", s)
	}

	// events: seed + exactly 5 audit events, each quoting a distinct title.
	rawEv, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]bool{}
	n := 0
	idDupeEvents := 0
	for _, l := range strings.Split(strings.TrimRight(string(rawEv), "\n"), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("events.jsonl line does not parse: %v", err)
		}
		n++
		if ev["event_type"] != "audit" {
			continue
		}
		d, ok := ev["detail"].(string)
		if !ok || d == "" {
			continue // the seed audit row carries detail null
		}
		var detail map[string]any
		if err := json.Unmarshal([]byte(d), &detail); err != nil {
			t.Fatalf("audit detail does not parse: %v (%s)", err, d)
		}
		if detail["action"] != "id-dupe" {
			t.Fatalf("audit action = %v, want id-dupe", detail["action"])
		}
		titles[detail["title"].(string)] = true
		idDupeEvents++
	}
	if n != 6 {
		t.Fatalf("events.jsonl rows = %d, want 6 (seed + 5 audit)", n)
	}
	if idDupeEvents != 5 {
		t.Fatalf("id-dupe audit events = %d, want 5", idDupeEvents)
	}
	if len(titles) != 5 {
		t.Fatalf("audit events quote %d distinct titles, want 5: %v", len(titles), titles)
	}

	// --- second apply is a no-op
	tasksAfter1, _ := os.ReadFile(filepath.Join(dir, "tasks.jsonl"))
	evAfter1, _ := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	code, out = runCLI(t, dir, "--by-id", "--apply")
	if code != 0 {
		t.Fatalf("second apply exit = %d, want 0\n%s", code, out)
	}
	if !strings.Contains(out, "nothing to do") {
		t.Fatalf("second apply output = %q, want a nothing-to-do report", out)
	}
	tasksAfter2, _ := os.ReadFile(filepath.Join(dir, "tasks.jsonl"))
	evAfter2, _ := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if string(tasksAfter1) != string(tasksAfter2) || string(evAfter1) != string(evAfter2) {
		t.Fatal("second apply rewrote a file")
	}
}

// AC3 at CLI level: --by-id dry-run writes NOTHING (sha256 of every tracked
// file before == after).
func TestCLI_ByIDDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, asceFixtureTasks(), testEvents, testHeader)
	before := boardSnapshot(t, dir)
	code, out := runCLI(t, dir, "--by-id")
	if code != 0 {
		t.Fatalf("dry-run exit = %d\n%s", code, out)
	}
	after := boardSnapshot(t, dir)
	assertSnapshotsEqual(t, before, after)
}

// AC5: the fresh-id rule (fix direction (a)) is in the help text, in two
// places — the flag description and the usage block.
func TestCLI_HelpDocumentsFreshIDRule(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldErr := os.Stderr
	os.Stderr = w
	code := run([]string{"-h"})
	os.Stderr = oldErr
	w.Close()
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	help := string(buf)
	if code != 0 {
		t.Fatalf("-h exit = %d, want 0", code)
	}
	for _, want := range []string{
		"--by-id",
		"a lane must ALWAYS mint a fresh task id for a new finding",
		// wrap-tolerant: the help text is hard-wrapped, so never assert
		// across a line break
		"id-recycled filing",
		"a new finding under an old key",
		"recycled-id repair (DF-BOARDCTL-10)",
		"superseded-by-earliest",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help text missing %q;\nhelp was:\n%s", want, help)
		}
	}
}

// --json emits the by-id report as parseable JSON (dry-run plan included).
func TestCLI_ByIDJSONReport(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, asceFixtureTasks(), testEvents, testHeader)
	before := boardSnapshot(t, dir)
	code, out := runCLI(t, dir, "--by-id", "--json")
	if code != 0 {
		t.Fatalf("--json exit = %d\n%s", code, out)
	}
	var report struct {
		Lane        string `json:"lane"`
		BoardPath   string `json:"board_path"`
		TaskRows    int    `json:"task_rows"`
		RecycledIDs int    `json:"recycled_ids"`
		Groups      []struct {
			ID           string   `json:"id"`
			KeptLine     int      `json:"kept_line"`
			ClosedLines  []int    `json:"closed_lines"`
			ClosedTitles []string `json:"closed_titles"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("--json output does not parse: %v\n%s", err, out)
	}
	if report.Lane != "asce" {
		t.Fatalf("lane = %q, want asce", report.Lane)
	}
	if report.RecycledIDs != 1 || len(report.Groups) != 1 {
		t.Fatalf("json report wrong: %+v", report)
	}
	g := report.Groups[0]
	if g.ID != "QA-ASCE-1" || g.KeptLine != 1 || len(g.ClosedLines) != 5 || len(g.ClosedTitles) != 5 {
		t.Fatalf("json group wrong: %+v", g)
	}
	assertSnapshotsEqual(t, before, boardSnapshot(t, dir))
}

// --json without --by-id is a usage refusal (the default mode keeps its text
// report — default invocations keep working).
func TestCLI_JSONRequiresByID(t *testing.T) {
	dir := t.TempDir()
	writeBoard(t, dir, asceFixtureTasks(), testEvents, testHeader)
	args := []string{"--board-dir", dir, "--json"}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldErr := os.Stderr
	os.Stderr = w
	code := run(args)
	os.Stderr = oldErr
	w.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	if code != 2 {
		t.Fatalf("--json without --by-id exit = %d, want 2", code)
	}
}

// Default (fingerprint) behavior is UNCHANGED: a genuine same-finding pair
// still collapses through the untouched default path with its text report.
func TestCLI_DefaultFingerprintModeUnchanged(t *testing.T) {
	dir := t.TempDir()
	finding := `"title":"[P3] run_battery FAIL — port pool exhausted","reasoning":"cell detail: no free port ranges"`
	tasks := `{"id":"QA-BF-1","status":"pending","priority":"P3",` + finding + `}` + "\n" +
		`{"id":"QA-BF-2","status":"pending","priority":"P3",` + finding + `}` + "\n"
	writeBoard(t, dir, tasks, testEvents, testHeader)

	code, out := runCLI(t, dir, "--apply")
	if code != 0 {
		t.Fatalf("default apply exit = %d\n%s", code, out)
	}
	if !strings.Contains(out, "merge groups: 1") || !strings.Contains(out, "keep QA-BF-1") {
		t.Fatalf("default report wrong:\n%s", out)
	}
	if !strings.Contains(out, "one audit event per merge group") {
		t.Fatalf("default applied marker missing:\n%s", out)
	}
}

// A recycled-id board with an unknown status on a later line refuses with
// exit 1 and writes nothing.
func TestCLI_ByIDRefusesUnknownStatus(t *testing.T) {
	dir := t.TempDir()
	tasks := `{"id":"QA-X-1","title":"first","reasoning":"r1","status":"pending","priority":"P2"}` + "\n" +
		`{"id":"QA-X-1","title":"second","reasoning":"r2","status":"retired","priority":"P2"}` + "\n"
	writeBoard(t, dir, tasks, testEvents, testHeader)
	before := boardSnapshot(t, dir)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldErr := os.Stderr
	os.Stderr = w
	code := run([]string{"--board-dir", dir, "--by-id", "--apply"})
	os.Stderr = oldErr
	w.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	if code != 1 {
		t.Fatalf("refusal exit = %d, want 1 (stderr: %s)", code, buf)
	}
	assertSnapshotsEqual(t, before, boardSnapshot(t, dir))
}
