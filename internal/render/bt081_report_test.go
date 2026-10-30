package render

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
)

func bt81Row(t *testing.T, raw string) *board.Row {
	t.Helper()
	row, err := board.ParseRow([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func bt81Board(tasks []*board.Row) *boardData {
	d := &boardData{topology: "A", loc: time.UTC, now: time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC), fixtureI: map[string]bool{}}
	for _, row := range tasks {
		d.tasks = append(d.tasks, buildTaskRec(row, row.String("id"), d, d.now))
	}
	return d
}

func TestDerivedReportedVerifiedCompleteCounts(t *testing.T) {
	tasks := []*board.Row{
		bt81Row(t, `{"id":"R","status":"complete","created_at":"2026-09-01 00:00:00","completed_at":"2026-09-02 00:00:00"}`),
		bt81Row(t, `{"id":"V1","status":"complete","created_at":"2026-09-01 00:00:00","detail":{"evidence":[{"run_id":"run-v1"}]}}`),
		bt81Row(t, `{"id":"V2","status":"complete","created_at":"2026-09-01 00:00:00","evidence_run_id":"run-v2"}`),
		bt81Row(t, `{"id":"P","status":"pending","created_at":"2026-09-01 00:00:00"}`),
	}
	der := derive(bt81Board(tasks))
	if der.CompleteCount != 3 || der.ReportedCompleteCount != 3 || der.VerifiedCompleteCount != 2 || der.UnverifiedCompleteCount != 1 {
		t.Fatalf("reported/verified counts = %+v", der)
	}
	payload, err := json.Marshal(der)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]float64{
		"complete_count":            3,
		"reported_complete_count":   3,
		"verified_complete_count":   2,
		"unverified_complete_count": 1,
	} {
		if fields[key] != want {
			t.Fatalf("payload[%q] = %v, want %v", key, fields[key], want)
		}
	}
}
