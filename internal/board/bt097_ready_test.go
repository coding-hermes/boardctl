package board

import (
	"fmt"
	"testing"
	"time"
)

func TestBT097DispatchabilityCoverageAndControls(t *testing.T) {
	b := readyTestBoard(t,
		`{"id":"READY-1","title":"ready","status":"pending","priority":"P1"}`+"\n"+
			`{"id":"DANGLING-1","title":"dangling","status":"pending","priority":"P1","depends_on":["MISSING-1"]}`+"\n"+
			`{"id":"DONE-1","title":"done","status":"done","priority":"P1"}`+"\n"+
			`{"id":"BLOCKED-1","title":"blocked","status":"blocked","priority":"P1"}`+"\n"+
			`{"id":"REVIEW-1","title":"review","status":"review","priority":"P1"}`)
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || report.Ready[0].String("id") != "READY-1" {
		t.Fatalf("ready=%v, want only READY-1", report.Ready)
	}
	if len(report.Dangling) != 1 || report.Dangling[0].Dep != "MISSING-1" {
		t.Fatalf("dangling=%v, want named missing dependency", report.Dangling)
	}
	classes := map[string]string{}
	for _, row := range report.Blocked {
		classes[row.Row.String("id")] = row.Class
	}
	for id, want := range map[string]string{"DANGLING-1": "dangling_dep", "BLOCKED-1": "status", "REVIEW-1": "status"} {
		if classes[id] != want {
			t.Fatalf("%s class=%q, want %q", id, classes[id], want)
		}
	}
	if !Dispatchable(report.Ready[0], b) || Dispatchable(report.Blocked[0].Row, b) {
		t.Fatal("exported predicate disagrees with report")
	}
}

func TestBT097DispatchabilityScalingRatio(t *testing.T) {
	if testing.Short() {
		t.Skip("scaling probe is disabled in short mode")
	}
	measure := func(n int) time.Duration {
		var tasks string
		for i := 0; i < n; i++ {
			status := "pending"
			deps := ""
			if i == 0 {
				status = "complete"
			} else {
				deps = fmt.Sprintf(`,"depends_on":["BT-%05d"]`, i-1)
			}
			tasks += fmt.Sprintf(`{"id":"BT-%05d","title":"row","status":"%s","priority":"P1"%s}`+"\n", i, status, deps)
		}
		b := readyTestBoard(t, tasks)
		rows, err := b.TaskRows()
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		report, err := analyzeDispatchabilityRowsCore(rows)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Rows) != n {
			t.Fatalf("rows=%d, want %d", len(report.Rows), n)
		}
		return elapsed
	}
	small := measure(1000)
	large := measure(10000)
	ratio := float64(large) / float64(small)
	t.Logf("dispatchability scaling: n=1000 %s, n=10000 %s, ratio=%.2fx", small, large, ratio)
	if ratio >= 20 {
		t.Fatalf("scaling ratio %.2fx, want <20x", ratio)
	}
}
