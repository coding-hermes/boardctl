package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readyTestBoard(t *testing.T, tasks string) *Board {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"tasks.jsonl":  tasks + "\n",
		"events.jsonl": "{\"id\":1,\"event_type\":\"audit\",\"task_id\":null}\n",
		"board.jsonl":  "{\"project\":\"test\",\"namespace\":\"test\",\"version\":3,\"ticks_total\":1,\"ticks_idle\":0}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnalyzeDispatchabilityTransitiveAndBlocked(t *testing.T) {
	b := readyTestBoard(t,
		`{"id":"A-1","title":"A","status":"pending","priority":"P1","depends_on":["B-1"],"initiative":"alpha"}`+"\n"+
			`{"id":"B-1","title":"B","status":"pending","priority":"P1","depends_on":["C-1"],"initiative":"alpha"}`+"\n"+
			`{"id":"C-1","title":"C","status":"complete","priority":"P1","initiative":"alpha"}`)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || len(report.Blocked) != 1 {
		t.Fatalf("ready=%d blocked=%d, want 1/1", len(report.Ready), len(report.Blocked))
	}
	if got := report.Blocked[0].ChainText; got == "" || !strings.Contains(got, "B-1") {
		t.Fatalf("blocker chain not named: %q", got)
	}
}

func TestAnalyzeDispatchabilityCycleIsNamed(t *testing.T) {
	b := readyTestBoard(t,
		`{"id":"A-1","title":"A","status":"pending","priority":"P1","depends_on":["B-1"]}`+"\n"+
			`{"id":"B-1","title":"B","status":"pending","priority":"P1","depends_on":["A-1"]}`)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cycles) != 1 || len(report.Cycles[0]) != 3 {
		t.Fatalf("cycles=%v, want one closed cycle", report.Cycles)
	}
	if len(report.Blocked) != 2 || report.Blocked[0].Class != "cycle" {
		t.Fatalf("blocked=%v, want cycle classifications", report.Blocked)
	}
}

func TestAnalyzeDispatchabilityInitiativeAndDeferred(t *testing.T) {
	b := readyTestBoard(t,
		`{"id":"A-1","title":"A","status":"pending","priority":"P1","initiative":"alpha"}`+"\n"+
			`{"id":"B-1","title":"B","status":"pending","priority":"P1","deferred":true,"initiative":"beta"}`)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || report.Ready[0].String("id") != "A-1" {
		t.Fatalf("ready=%v, want alpha A-1", report.Ready)
	}
	if len(report.Blocked) != 1 || report.Blocked[0].Class != "deferred" {
		t.Fatalf("blocked=%v, want deferred beta row", report.Blocked)
	}
}
