package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// graphCLIBoard creates a topology-A board with ready, blocked, deferred, and
// unscoped rows so the CLI tests exercise the graph report rather than mocks.
func graphCLIBoard(t *testing.T, tasks string) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"tasks.jsonl":  strings.TrimSpace(tasks) + "\n",
		"events.jsonl": `{"id":1,"event_type":"audit","task_id":null}` + "\n",
		"board.jsonl":  `{"project":"graph-test","namespace":"graph-test","version":3,"ticks_total":1,"ticks_idle":0}` + "\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(boardDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func graphRows() string {
	return strings.Join([]string{
		`{"id":"DONE-1","title":"done","status":"complete","priority":"P1","initiative":"alpha"}`,
		`{"id":"READY-1","title":"ready alpha","status":"pending","priority":"P1","depends_on":["DONE-1"],"initiative":"alpha"}`,
		`{"id":"READY-2","title":"ready unscoped","status":"pending","priority":"P2","depends_on":[]}`,
		`{"id":"BLOCKED-1","title":"blocked by ready","status":"pending","priority":"P3","depends_on":["READY-1"],"initiative":"alpha"}`,
		`{"id":"DEFER-1","title":"deferred beta","status":"pending","priority":"P1","deferred":true,"initiative":"beta"}`,
		`{"id":"MISSING-1","title":"missing dependency","status":"pending","priority":"P2","depends_on":["GHOST-1"],"initiative":"gamma"}`,
		`{"id":"REVIEW-1","title":"review status","status":"review","priority":"P2","initiative":"gamma"}`,
	}, "\n")
}

// TestGraphCLIReadyAndDirectionPaths pins the normal and JSON graph surfaces,
// including initiative filtering, blocker chains, deferred rows, wave data,
// and unscoped rows. ch:trace row=RELEASE-2026-11-09-07 spec=.coding-hermes/board/tasks.jsonl#RELEASE-2026-11-09-07 test=cmd/boardctl/*graph* doc=none:release-row worker evidence=make-cover-check witness=worker-commit verdict=none:foreman-judge commit=none:worker-sha memory=none:foreman-closeout
func TestGraphCLIReadyAndDirectionPaths(t *testing.T) {
	dir := graphCLIBoard(t, graphRows())

	readyText, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "ready", "--blocked"}); got != 0 {
			t.Errorf("ready --blocked exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ready now: 2 row(s)", "READY-1", "READY-2", "blocked: 4 row(s)", "BLOCKED-1", "DEFER-1", "MISSING-1", "REVIEW-1"} {
		if !strings.Contains(readyText, want) {
			t.Errorf("ready text missing %q:\n%s", want, readyText)
		}
	}

	filtered, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "ready", "--initiative", "ALPHA"}); got != 0 {
			t.Errorf("ready --initiative ALPHA exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filtered, "ready now: 1 row(s)") || !strings.Contains(filtered, "READY-1") || strings.Contains(filtered, "READY-2") {
		t.Fatalf("initiative filter output = %q, want only alpha READY-1", filtered)
	}

	unknown, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "ready", "--initiative", "absent"}); got != 0 {
			t.Errorf("ready unknown initiative exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unknown, `no rows carry initiative "absent"`) {
		t.Fatalf("unknown initiative output = %q", unknown)
	}

	readyJSONOutput, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "ready", "--json"}); got != 0 {
			t.Errorf("ready --json exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var readyPayload readyJSON
	if err := json.Unmarshal([]byte(readyJSONOutput), &readyPayload); err != nil {
		t.Fatalf("ready --json is invalid JSON: %v\n%s", err, readyJSONOutput)
	}
	if readyPayload.TotalReady != 2 || readyPayload.Stats.OpenRows != 6 || readyPayload.Stats.DeferredOpen != 1 {
		t.Fatalf("ready stats = %+v, want ready=2 open=6 deferred=1", readyPayload.Stats)
	}
	if len(readyPayload.Groups) != 2 || len(readyPayload.Blocked) != 4 || len(readyPayload.Dangling) != 1 {
		t.Fatalf("ready groups=%d blocked=%d dangling=%d, want 2/4/1", len(readyPayload.Groups), len(readyPayload.Blocked), len(readyPayload.Dangling))
	}
	if readyPayload.Groups[0].Initiative != "" || readyPayload.Groups[1].Initiative != "alpha" {
		t.Fatalf("ready groups = %+v, want unscoped then alpha", readyPayload.Groups)
	}

	directionText, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "direction"}); got != 0 {
			t.Errorf("direction exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"open initiative(s)", "alpha", "ready now: 1 (READY-1)", "(no initiative)  total 1 | remaining 1 | ready now 1"} {
		if !strings.Contains(directionText, want) {
			t.Errorf("direction text missing %q:\n%s", want, directionText)
		}
	}

	directionJSONOutput, err := captureStdout(func() {
		if got := run([]string{"-C", dir, "direction", "--json"}); got != 0 {
			t.Errorf("direction --json exit = %d, want 0", got)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var directionPayload directionJSON
	if err := json.Unmarshal([]byte(directionJSONOutput), &directionPayload); err != nil {
		t.Fatalf("direction --json is invalid JSON: %v\n%s", err, directionJSONOutput)
	}
	if directionPayload.Stats.TotalReady != 2 || directionPayload.Stats.OpenInitiatives != 3 || len(directionPayload.Initiatives) != 3 {
		t.Fatalf("direction stats = %+v initiatives=%d, want ready=2 open=3", directionPayload.Stats, len(directionPayload.Initiatives))
	}
	if directionPayload.Unscoped.Total != 1 || directionPayload.Unscoped.ReadyNow != 1 {
		t.Fatalf("direction unscoped = %+v, want total=1 ready=1", directionPayload.Unscoped)
	}
}

func TestGraphCLIReportsCycleAndMalformedDependency(t *testing.T) {
	cycleDir := graphCLIBoard(t, strings.Join([]string{
		`{"id":"CYCLE-A","title":"cycle a","status":"pending","priority":"P1","depends_on":["CYCLE-B"]}`,
		`{"id":"CYCLE-B","title":"cycle b","status":"pending","priority":"P1","depends_on":["CYCLE-A"]}`,
	}, "\n"))
	cycleOutput, err := captureStdout(func() {
		if got := run([]string{"-C", cycleDir, "ready", "--blocked"}); got == 0 {
			t.Error("ready cycle exit = 0, want a cycle error")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cycleOutput, "blocked: 2 row(s)") || !strings.Contains(cycleOutput, "CYCLE-A") || !strings.Contains(cycleOutput, "CYCLE-B") {
		t.Fatalf("cycle output = %q, want named cycle blockers", cycleOutput)
	}

	for _, verb := range []string{"ready", "direction"} {
		var code int
		out, err := captureStdout(func() {
			code = run([]string{"-C", cycleDir, verb, "--json"})
		})
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Cycles [][]string `json:"cycles"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil || code == 0 || len(result.Cycles) != 1 || len(result.Cycles[0]) != 3 {
			t.Errorf("%s cycle JSON: exit=%d result=%+v error=%v output=%s", verb, code, result, err, out)
		}
	}

	malformedDir := graphCLIBoard(t, `{"id":"BAD-1","title":"bad","status":"pending","priority":"P1","depends_on":{"not":"an array"}}`)
	if err := cmdDirection(malformedDir, nil); err == nil || !strings.Contains(err.Error(), "depends_on is not an array") {
		t.Fatalf("cmdDirection malformed dependency error = %v, want parser error", err)
	}
}

func TestGraphCLIRejectsPositionalAndMissingBoards(t *testing.T) {
	if err := cmdReady("", []string{"unexpected"}); err == nil || !strings.Contains(err.Error(), "ready takes no positional args") {
		t.Fatalf("cmdReady positional error = %v", err)
	}
	if err := cmdDirection("", []string{"unexpected"}); err == nil || !strings.Contains(err.Error(), "direction takes no positional args") {
		t.Fatalf("cmdDirection positional error = %v", err)
	}
	if err := cmdReady(t.TempDir(), nil); err == nil {
		t.Fatal("cmdReady on boardless dir returned nil")
	}
	if err := cmdDirection(t.TempDir(), nil); err == nil {
		t.Fatal("cmdDirection on boardless dir returned nil")
	}
}

func TestGraphHelperFallbacks(t *testing.T) {
	if got := sortedStrings(nil); got == nil || len(got) != 0 {
		t.Fatalf("sortedStrings(nil) = %#v, want non-nil empty slice", got)
	}
	if got := nonNilCycles(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilCycles(nil) = %#v, want non-nil empty slice", got)
	}
	if got := nonNilDangling(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilDangling(nil) = %#v, want non-nil empty slice", got)
	}
	if got := waveProjection(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("waveProjection(nil, nil) = %#v, want non-nil empty slice", got)
	}
	if got := directionUnscoped(nil, nil, nil); got.Total != 0 || got.WaveProjection == nil {
		t.Fatalf("directionUnscoped empty = %+v, want empty projection", got)
	}
}
