package board

import (
	"encoding/json"
	"strings"
	"testing"
)

// BT-105 (CF-403 mechanism): a completion bound to an evidence run must
// carry that run id IN the task_completed EVENT detail — the append-only
// trail is the load-bearing audit surface, and before this fix the id the
// gate validated survived only in the row's detail envelope plus
// git-untracked .gitreins/history. The fix adds exactly one key when a run
// is bound: {"status":"complete","evidence_run_id":"<run>"}. The shape
// rules pinned here:
//
//   - task_completed WITH a bound run carries the run id in the event detail
//   - task_completed with NO run bound keeps exactly {"status":"complete"}
//     (the legacy complete-row re-stamp path through gateCompletionEvidence's
//     idempotent exemption)
//   - task_updated events keep the status-only detail — the run id must not
//     leak into a non-completion status event
func TestBT105CompletionEventCarriesEvidenceRunID(t *testing.T) {
	b := newTestBoard(t)
	status := "complete"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status, EvidenceRunID: "run-bt105"}); err != nil {
		t.Fatal(err)
	}
	raw := lastEventLine(t, b)
	// AC1 (grep-level): the raw event line itself carries the run id, so a
	// closed row resolves from tracked events alone — no dependence on the
	// git-untracked .gitreins/history directory the gate validated against.
	if !strings.Contains(string(raw), "run-bt105") {
		t.Fatalf("task_completed event line does not carry the evidence run id: %s", raw)
	}
	var e map[string]any
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e["event_type"] != "task_completed" || e["task_id"] != "EXIST-1" {
		t.Fatalf("event wrong: %s", raw)
	}
	detail, ok := e["detail"].(string)
	if !ok {
		t.Fatalf("detail not a JSON string: %s", raw)
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		t.Fatalf("detail %q is not decodable JSON: %v", detail, err)
	}
	if d["status"] != "complete" {
		t.Fatalf("detail status = %v, want complete", d["status"])
	}
	if d["evidence_run_id"] != "run-bt105" {
		t.Fatalf("detail = %s, want evidence_run_id \"run-bt105\" (BT-105)", detail)
	}
}

// BT-105: a completion with NO evidence run bound keeps exactly
// {"status":"complete"}. newTestBoard seeds EXIST-1 already complete with
// no completion event — the legacy-row path where gateCompletionEvidence's
// idempotent exemption lets a run-less completion through and the event is
// still appended.
func TestBT105CompletionWithoutRunKeepsStatusOnlyDetail(t *testing.T) {
	b := newTestBoard(t)
	status := "complete"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status}); err != nil {
		t.Fatal(err)
	}
	raw := lastEventLine(t, b)
	var e map[string]any
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e["event_type"] != "task_completed" {
		t.Fatalf("event type = %v, want task_completed", e["event_type"])
	}
	detail, ok := e["detail"].(string)
	if !ok {
		t.Fatalf("detail not a JSON string: %s", raw)
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		t.Fatalf("detail %q is not decodable JSON: %v", detail, err)
	}
	if len(d) != 1 || d["status"] != "complete" {
		t.Fatalf("detail = %s, want exactly {\"status\":\"complete\"}", detail)
	}
}

// BT-105: task_updated events keep the status-only shape even when the same
// update binds an evidence run (the run goes to the row's detail envelope;
// a non-completion status event must not carry it).
func TestBT105TaskUpdatedEventNeverCarriesEvidenceRunID(t *testing.T) {
	b := newTestBoard(t)
	status := "in_progress"
	if _, err := b.UpdateTask("EXIST-1", UpdateSpec{Status: &status, EvidenceRunID: "run-bt105"}); err != nil {
		t.Fatal(err)
	}
	raw := lastEventLine(t, b)
	var e map[string]any
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e["event_type"] != "task_updated" {
		t.Fatalf("event type = %v, want task_updated", e["event_type"])
	}
	detail, ok := e["detail"].(string)
	if !ok {
		t.Fatalf("detail not a JSON string: %s", raw)
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(detail), &d); err != nil {
		t.Fatalf("detail %q is not decodable JSON: %v", detail, err)
	}
	if len(d) != 1 || d["status"] != "in_progress" {
		t.Fatalf("detail = %s, want exactly {\"status\":\"in_progress\"}", detail)
	}
	if strings.Contains(string(raw), "run-bt105") {
		t.Fatalf("task_updated event must not carry the evidence run id: %s", raw)
	}
}
