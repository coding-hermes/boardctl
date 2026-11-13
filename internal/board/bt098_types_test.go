package board

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const bt098Release = `{"slug":"release","display":"Release","rev":1,"template":{"title_prefix":"[RELEASE] ","default_priority":"P1","ac_skeleton":"ship safely"},"metadata":[{"name":"channel","type":"enum","required":true,"values":["stable","beta"]},{"name":"source_commit","type":"string","required":true}]}` + "\n"

func bt098Config(t *testing.T, b *Board, s string) {
	t.Helper()
	if err := os.WriteFile(b.TypesPath(), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
func bt098Bytes(t *testing.T, b *Board) (string, string) {
	t.Helper()
	a, e := os.ReadFile(b.TasksPath())
	if e != nil {
		t.Fatal(e)
	}
	c, e := os.ReadFile(b.EventsPath())
	if e != nil {
		t.Fatal(e)
	}
	return string(a), string(c)
}

// ch:trace row=BT-098 spec=specs/BT-098-ticket-types.md wave=none:serial-worker test=internal/board/bt098_types_test.go::TestBT098CreateAndUpdate doc=skills/boardctl-usage/SKILL.md prompt=/home/kara/.hermes/cache/scratch/boardctl-foreman-20261113-173504/briefs/BT-098.txt evidence=none:worker-tests witness=none:local-test-only verdict=foreman-pending commit=worker-pending memory=foreman-pending
func TestBT098CreateAndUpdate(t *testing.T) {
	b := seedFindingBoard(t, `{"id":"OLD-1","title":"old","status":"pending","priority":"P2"}`)
	bt098Config(t, b, bt098Release)
	before, ev := bt098Bytes(t, b)
	_, err := b.Create(TaskRowSpec{ID: "REL-1", Title: "R1", Type: "release"})
	if err == nil || !strings.Contains(err.Error(), "channel") || !strings.Contains(err.Error(), "source_commit") {
		t.Fatalf("missing-field refusal: %v", err)
	}
	a, c := bt098Bytes(t, b)
	if a != before || c != ev {
		t.Fatal("refusal wrote board")
	}
	spec := TaskRowSpec{ID: "REL-1", Title: "R1", Type: "release", HasMeta: true, Meta: []MetaKV{{"channel", "beta"}, {"source_commit", "abc"}}}
	if _, err = b.Create(spec); err != nil {
		t.Fatal(err)
	}
	row := taskRowByID(t, b, "REL-1")
	if row["title"] != "[RELEASE] R1" || row["priority"] != "P1" || row["acceptance_criteria"] != "ship safely" || row["type"] != "release" {
		t.Fatalf("template: %v", row)
	}
	md := row["detail"].(map[string]any)["metadata"].(map[string]any)
	if md["channel"] != "beta" || md["source_commit"] != "abc" {
		t.Fatalf("metadata: %v", md)
	}
	before, ev = bt098Bytes(t, b)
	_, err = b.UpdateTask("REL-1", UpdateSpec{HasMeta: true, Meta: []MetaKV{{"channel", ""}}})
	if err == nil || !strings.Contains(err.Error(), "REQUIRED") {
		t.Fatalf("update refusal: %v", err)
	}
	a, c = bt098Bytes(t, b)
	if a != before || c != ev {
		t.Fatal("refused update wrote board")
	}
	spec.ID = "REL-2"
	spec.Title = "[RELEASE] R2"
	spec.Priority = "P3"
	ac := "custom"
	spec.AcceptanceCriteria = &ac
	if _, err = b.Create(spec); err != nil {
		t.Fatal(err)
	}
	row = taskRowByID(t, b, "REL-2")
	if row["title"] != "[RELEASE] R2" || row["priority"] != "P3" || row["acceptance_criteria"] != "custom" {
		t.Fatalf("explicit precedence: %v", row)
	}
}

func TestBT098ValidateMissingMetadata(t *testing.T) {
	b := seedFindingBoard(t, `{"id":"REL-1","title":"x","status":"pending","priority":"P2","type":"release"}`)
	bt098Config(t, b, bt098Release)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if !rep.HasErrors() || !strings.Contains(rep.RenderText(), `missing required metadata field "channel"`) || !strings.Contains(rep.RenderText(), `missing required metadata field "source_commit"`) {
		t.Fatalf("missing required metadata not detected:\n%s", rep.RenderText())
	}
}

func TestBT098TypeChangesAndMalformed(t *testing.T) {
	b := seedFindingBoard(t, `{"id":"OLD-1","title":"old","status":"pending","priority":"P2"}`)
	bt098Config(t, b, bt098Release)
	before, ev := bt098Bytes(t, b)
	_, err := b.UpdateTask("OLD-1", UpdateSpec{Type: "release"})
	if err == nil {
		t.Fatal("missing required metadata accepted")
	}
	a, c := bt098Bytes(t, b)
	if a != before || c != ev {
		t.Fatal("failed type change wrote board")
	}
	_, err = b.UpdateTask("OLD-1", UpdateSpec{Type: "release", HasMeta: true, Meta: []MetaKV{{"channel", "beta"}, {"source_commit", "abc"}}})
	if err != nil {
		t.Fatal(err)
	}
	event := lastEventRaw(t, b)
	if event["event_type"] != "task_type_changed" {
		t.Fatalf("event: %v", event)
	}
	d := eventDetailMap(t, event)
	if d["old_type"] != "" || d["new_type"] != "release" {
		t.Fatalf("event detail: %v", d)
	}
	bt098Config(t, b, `{"slug":"release","display":"Release","metadata":[{"name":"x","type":"enum"}]}`+"\n")
	before, ev = bt098Bytes(t, b)
	_, err = b.Create(TaskRowSpec{ID: "NEW-1", Title: "new", Type: "release"})
	if err == nil {
		t.Fatal("malformed enum accepted")
	}
	a, c = bt098Bytes(t, b)
	if a != before || c != ev {
		t.Fatal("malformed registry wrote board")
	}
}

func TestBT098DefaultUnchangedReadOnly(t *testing.T) {
	b := seedFindingBoard(t, `{"id":"OLD-1","title":"old","status":"pending","priority":"P2"}`)
	before, ev := bt098Bytes(t, b)
	reg, err := b.LoadTypes()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(reg.ActiveSlugs(), ",") != "bug,chore,feat,task" {
		t.Fatalf("defaults %v", reg.ActiveSlugs())
	}
	if _, err = b.Validate(); err != nil {
		t.Fatal(err)
	}
	a, c := bt098Bytes(t, b)
	if !bytes.Equal([]byte(a), []byte(before)) || c != ev {
		t.Fatal("read mutated board")
	}
}
