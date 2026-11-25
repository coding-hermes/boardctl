package board

// BT-099 — the falsifiable test plan (spec §8) plus panel amendment 4's
// shared-node cycle fixture. Each test names the falsifier it kills.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bt099Board seeds a topology-A board with the given task rows and a
// full-shape event trail seed.
func bt099Board(t *testing.T, tasks string, events string) *Board {
	t.Helper()
	if events == "" {
		events = `{"id":1,"timestamp":"2026-11-08 04:00:00","event_type":"audit","task_id":null,"actor":"foreman","detail":null,"tick_number":1}`
	}
	dir := t.TempDir()
	files := map[string]string{
		"tasks.jsonl":  tasks + "\n",
		"events.jsonl": events + "\n",
		"board.jsonl":  `{"project":"bt099","namespace":"bt099","version":3,"ticks_total":1,"ticks_idle":0}` + "\n",
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

func bt099Read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestBT099_ParentRefusals is test plan #5: self-parent, nonexistent parent,
// complete parent, 2-node and 3-node cycles — each REFUSED with a message
// naming the ids/cycle, and the row bytes unchanged afterwards.
func TestBT099_ParentRefusals(t *testing.T) {
	tasks := `{"id":"PR-1","title":"parent","status":"pending","priority":"P1"}` + "\n" +
		`{"id":"CH-1","title":"child","status":"pending","priority":"P1","parent":"PR-1"}` + "\n" +
		`{"id":"CY-1","title":"cycle a","status":"pending","priority":"P1","parent":"CY-2"}` + "\n" +
		`{"id":"CY-2","title":"cycle b","status":"pending","priority":"P1","parent":"CY-1"}` + "\n" +
		`{"id":"CY-3","title":"cycle c","status":"pending","priority":"P1","parent":"CY-1"}` + "\n" +
		`{"id":"DONE-1","title":"done parent","status":"complete","priority":"P1"}`
	b := bt099Board(t, tasks, "")
	tasksPath := b.TasksPath()
	before := bt099Read(t, tasksPath)

	refuse := func(name, want string, fn func() error) {
		t.Run(name, func(t *testing.T) {
			err := fn()
			if err == nil {
				t.Fatalf("%s: expected a refusal", name)
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: error %q does not name %q", name, err.Error(), want)
			}
		})
	}

	pending := "pending"
	refuse("create self-parent", "self-parent", func() error {
		_, err := b.Create(TaskRowSpec{ID: "NW-1", Title: "x", Parent: strPtr("NW-1")})
		return err
	})
	refuse("create missing parent", `parent task "NOPE-1" does not exist`, func() error {
		_, err := b.Create(TaskRowSpec{ID: "NW-1", Title: "x", Parent: strPtr("NOPE-1")})
		return err
	})
	refuse("create complete parent", `parent "DONE-1" is complete — open rows only`, func() error {
		_, err := b.Create(TaskRowSpec{ID: "NW-1", Title: "x", Parent: strPtr("DONE-1")})
		return err
	})
	refuse("update self-parent", "self-parent", func() error {
		_, err := b.UpdateTask("CH-1", UpdateSpec{Parent: strPtr("CH-1"), Status: &pending})
		return err
	})
	refuse("update missing parent", `parent task "NOPE-1" does not exist`, func() error {
		_, err := b.UpdateTask("CH-1", UpdateSpec{Parent: strPtr("NOPE-1")})
		return err
	})
	refuse("2-node cycle", "closes a parent cycle PR-1 -> CH-1 -> PR-1", func() error {
		// Setting P-1's parent to C-1 while C-1's parent is P-1 closes the
		// 2-node cycle — refused with the cycle printed.
		_, err := b.UpdateTask("PR-1", UpdateSpec{Parent: strPtr("CH-1")})
		return err
	})
	refuse("update complete parent", `parent "DONE-1" is complete`, func() error {
		_, err := b.UpdateTask("CH-1", UpdateSpec{Parent: strPtr("DONE-1")})
		return err
	})

	// The refused writes must have left the board byte-identical.
	if after := bt099Read(t, tasksPath); after != before {
		t.Fatalf("refusals rewrote tasks.jsonl:\n--- before\n%s\n--- after\n%s", before, after)
	}

	// Real 3-node cycle, built stepwise; every write-time gate must refuse
	// the closing link.
	b3 := bt099Board(t,
		`{"id":"AA-1","title":"a","status":"pending","priority":"P1"}`+"\n"+
			`{"id":"BB-1","title":"b","status":"pending","priority":"P1"}`+"\n"+
			`{"id":"DD-1","title":"d","status":"pending","priority":"P1"}`, "")
	if _, err := b3.UpdateTask("AA-1", UpdateSpec{Parent: strPtr("BB-1")}); err != nil {
		t.Fatal(err)
	}
	if _, err := b3.UpdateTask("BB-1", UpdateSpec{Parent: strPtr("DD-1")}); err != nil {
		t.Fatal(err)
	}
	if _, err := b3.UpdateTask("DD-1", UpdateSpec{Parent: strPtr("AA-1")}); err == nil {
		t.Fatal("3-node cycle: closing link was accepted")
	} else if !strings.Contains(err.Error(), "DD-1 -> AA-1 -> BB-1 -> DD-1") {
		t.Fatalf("3-node cycle: refusal does not print the cycle: %q", err.Error())
	}
}

// TestBT099_ParentCyclesSharedNode is panel amendment 4's fixture: a
// hand-edited board whose walks SHARE nodes (a row leading into a found
// cycle's members, plus a second independent cycle) must report every cycle
// exactly once and must never hang — the single-DFS-under-reporting
// falsifier. The parent graph is functional (one parent per row), so the
// "shared node" shape is a walk passing through a cycle's members; a
// detector with a global walk memo would drop or duplicate cycles here.
func TestBT099_ParentCyclesSharedNode(t *testing.T) {
	tasks := `{"id":"AA-1","title":"a","status":"pending","priority":"P1","parent":"BB-1"}` + "\n" +
		`{"id":"BB-1","title":"b","status":"pending","priority":"P1","parent":"AA-1"}` + "\n" +
		`{"id":"EE-1","title":"leads into the A/B cycle","status":"pending","priority":"P1","parent":"AA-1"}` + "\n" +
		`{"id":"CH-1","title":"c","status":"pending","priority":"P1","parent":"DD-1"}` + "\n" +
		`{"id":"DD-1","title":"d","status":"pending","priority":"P1","parent":"CH-1"}` + "\n" +
		`{"id":"FF-1","title":"f","status":"pending","priority":"P1","parent":"CH-1"}`
	b := bt099Board(t, tasks, "")
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	cycles := ParentCycles(rows)
	if len(cycles) != 2 {
		t.Fatalf("ParentCycles = %v, want exactly 2 cycles (the A/B cycle walked from A-1, B-1 AND E-1/F-1; the C/D cycle)", cycles)
	}
	members := map[string]bool{}
	for _, cycle := range cycles {
		members[strings.Join(cycle, "->")] = true
	}
	if !members["AA-1->BB-1->AA-1"] && !members["BB-1->AA-1->BB-1"] {
		t.Fatalf("the AA-1/BB-1 cycle is missing from %v", cycles)
	}
	if !members["CH-1->DD-1->CH-1"] && !members["DD-1->CH-1->DD-1"] {
		t.Fatalf("the CH-1/DD-1 cycle is missing from %v", cycles)
	}
	// The write path must also terminate (depth cap) when walked into the
	// cycle: F-1 re-pointed at A-1 — the walk enters the A/B cycle and is
	// refused with the cycle printed, in bounded time (the test itself is
	// the hang proof).
	err = b.ValidateParentLink("FF-1", "AA-1")
	if err == nil || !strings.Contains(err.Error(), "closes a parent cycle") {
		t.Fatalf("walk into a pre-existing cycle: got %v, want a cycle refusal", err)
	}

	// validate reports both cycles as parent-cycle errors naming the ids.
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if got := rep.CountFindingsByClass("parent-cycle", "error"); got != 2 {
		t.Fatalf("validate parent-cycle findings = %d, want 2:\n%s", got, rep.RenderText())
	}
}

// TestBT099_ParentRollup is test plan #6: parent with 5 children (3
// complete) — show's rollup numbers, the union of child deps omitting
// satisfied and including unsatisfied deps, and the STALENESS falsifier:
// mutating a child flips the rollup on the next read with the parent row
// bytes identical (nothing about children is ever stored).
func TestBT099_ParentRollup(t *testing.T) {
	tasks := `{"id":"PAR-1","title":"parent","status":"pending","priority":"P1"}` + "\n" +
		`{"id":"EXT-1","title":"outside dep","status":"pending","priority":"P1"}` + "\n" +
		`{"id":"SAT-1","title":"satisfied dep","status":"complete","priority":"P1"}` + "\n" +
		`{"id":"CH-1","title":"c1","status":"complete","priority":"P1","parent":"PAR-1"}` + "\n" +
		`{"id":"CH-2","title":"c2","status":"complete","priority":"P1","parent":"PAR-1","depends_on":["SAT-1"]}` + "\n" +
		`{"id":"CH-3","title":"c3","status":"complete","priority":"P1","parent":"PAR-1","depends_on":["EXT-1"]}` + "\n" +
		`{"id":"CH-4","title":"c4","status":"pending","priority":"P1","parent":"PAR-1","depends_on":["EXT-1","SAT-1"]}` + "\n" +
		`{"id":"CH-5","title":"c5","status":"in_progress","priority":"P1","parent":"PAR-1"}`
	b := bt099Board(t, tasks, "")
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	rollup := ComputeParentRollup("PAR-1", rows)
	if rollup == nil || rollup.Children != 5 || rollup.Open != 2 {
		t.Fatalf("rollup = %+v, want children 5 open 2", rollup)
	}
	if got := rollup.ChildrenLine(); got != "CHILDREN: 2/5 open (complete 3, pending 1, in_progress 1)" {
		t.Fatalf("children line = %q", got)
	}
	// The union omits the satisfied SAT-1 (complete) and includes EXT-1
	// (open), deduplicated across children.
	if got, want := rollup.DepsLine(), "CHILD DEPS (union): EXT-1"; got != want {
		t.Fatalf("deps line = %q, want %q", got, want)
	}
	parentBefore := bt099Read(t, b.TasksPath())
	parentLineBefore := bt099RowLine(parentBefore, "PAR-1")
	// Mutate a child; the rollup flips on the NEXT read with the parent row
	// untouched. (Status complete carries TR-251's evidence binding — the
	// synthetic run + force is the same escape the guard-003 suite uses.)
	if _, err := b.UpdateTask("CH-4", UpdateSpec{Status: strPtr("complete"), EvidenceRunID: "synthetic-run", CommitHash: strPtr(strings.Repeat("a", 40)), Force: true}); err != nil {
		t.Fatal(err)
	}
	rows, err = b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	rollup = ComputeParentRollup("PAR-1", rows)
	if rollup.Open != 1 || rollup.ByStatus["complete"] != 4 {
		t.Fatalf("rollup did not flip on the next read: %+v", rollup)
	}
	if got := rollup.ChildrenLine(); got != "CHILDREN: 1/5 open (complete 4, in_progress 1)" {
		t.Fatalf("children line after mutation = %q", got)
	}
	parentAfter := bt099Read(t, b.TasksPath())
	if got := bt099RowLine(parentAfter, "PAR-1"); got != parentLineBefore {
		t.Fatalf("parent row bytes changed on a child-only mutation:\n--- before\n%s\n--- after\n%s", parentLineBefore, got)
	}
}

// bt099RowLine returns the tasks.jsonl line carrying the row with the id.
func bt099RowLine(content, id string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, `"id":"`+id+`"`) {
			return line
		}
	}
	return ""
}

// TestBT099_NoDoubleDispatch is test plan #9: parent P (deps satisfied) +
// open child C (deps satisfied) — ready lists C and WITHHOLDS P; after C
// completes, ready lists P. With the child blocked, P (deps satisfied) may
// appear.
func TestBT099_NoDoubleDispatch(t *testing.T) {
	tasks := `{"id":"PP-1","title":"parent","status":"pending","priority":"P1"}` + "\n" +
		`{"id":"CC-1","title":"child","status":"pending","priority":"P1","parent":"PP-1"}`
	b := bt099Board(t, tasks, "")
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || report.Ready[0].String("id") != "CC-1" {
		t.Fatalf("ready = %v, want only the child", report.Ready)
	}
	if len(report.Withheld) != 1 || report.Withheld[0].ID != "PP-1" {
		t.Fatalf("withheld = %+v, want the parent", report.Withheld)
	}
	if report.Withheld[0].Reason != "open children ready" || !strings.Contains(report.Withheld[0].Detail, "dispatch CC-1 first") {
		t.Fatalf("withheld row = %+v", report.Withheld[0])
	}
	// The child completes; the parent becomes listable. (Same TR-251
	// evidence binding as the guard-003 suite's synthetic completions.)
	if _, err := b.UpdateTask("CC-1", UpdateSpec{Status: strPtr("complete"), EvidenceRunID: "synthetic-run", CommitHash: strPtr(strings.Repeat("a", 40)), Force: true}); err != nil {
		t.Fatal(err)
	}
	report, err = b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || report.Ready[0].String("id") != "PP-1" {
		t.Fatalf("ready after child completes = %v, want the parent", report.Ready)
	}
	if len(report.Withheld) != 0 {
		t.Fatalf("withheld after child completes = %+v, want empty", report.Withheld)
	}
	// With the child BLOCKED (deps unsatisfied), the parent (deps satisfied)
	// may appear in the ready set.
	b2 := bt099Board(t,
		`{"id":"PP-2","title":"parent","status":"pending","priority":"P1"}`+"\n"+
			`{"id":"CC-2","title":"child","status":"pending","priority":"P1","parent":"PP-2","depends_on":["GHOST-1"]}`, "")
	report, err = b2.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Ready) != 1 || report.Ready[0].String("id") != "PP-2" {
		t.Fatalf("ready with a blocked child = %v, want the parent", report.Ready)
	}
	// next (the single-pick surface) must never pick a withheld parent:
	// engine-level, that is "not in Ready", asserted via IsReady.
	if report.IsReady(report.Rows[1]) {
		t.Fatal("withheld parent still answers IsReady")
	}
}

// TestBT099_CommentAppendOnly is test plan #1: events.jsonl pre-bytes are an
// EXACT PREFIX of post-bytes, tasks.jsonl is byte-identical, and exactly one
// line was added. Falsifies comment-as-row-rewrite.
func TestBT099_CommentAppendOnly(t *testing.T) {
	b := bt099Board(t, `{"id":"TA-1","title":"t","status":"pending","priority":"P1"}`, "")
	eventsPath := b.EventsPath()
	eventsBefore := bt099Read(t, eventsPath)
	tasksBefore := bt099Read(t, b.TasksPath())
	headerBefore := bt099Read(t, b.HeaderPath())

	if _, err := b.AddComment("TA-1", "split BT-098's validator first", "", nil); err != nil {
		t.Fatal(err)
	}
	eventsAfter := bt099Read(t, eventsPath)
	if !bytes.HasPrefix([]byte(eventsAfter), []byte(eventsBefore)) {
		t.Fatal("events.jsonl pre-bytes are not an exact prefix of post-bytes — the append REWROTE history")
	}
	added := strings.TrimPrefix(eventsAfter, eventsBefore)
	if got := len(strings.Split(strings.TrimSuffix(added, "\n"), "\n")); got != 1 {
		t.Fatalf("comment added %d lines, want exactly 1 (%q)", got, added)
	}
	if after := bt099Read(t, b.TasksPath()); after != tasksBefore {
		t.Fatal("comment rewrote tasks.jsonl — comments are events, never row rewrites")
	}
	if after := bt099Read(t, b.HeaderPath()); after != headerBefore {
		t.Fatal("comment bumped the header (ticks_total/updated_at) — a comment is not tick work (amendment 5)")
	}
}

// TestBT099_CommentSchema is test plan #2: the appended event parses,
// event_type is "comment", the actor default is foreman, --author is
// honored, and detail round-trips to {body, in_reply_to} with in_reply_to
// present ONLY when passed. Falsifies schema drift / loose keys.
func TestBT099_CommentSchema(t *testing.T) {
	seed := `{"id":1,"timestamp":"2026-11-08 04:50:00","event_type":"comment","task_id":"TA-1","actor":"lane","detail":"{\"body\":\"root comment\"}","tick_number":null}`
	b := bt099Board(t, `{"id":"TA-1","title":"t","status":"pending","priority":"P1"}`, seed)
	replyTo := int64(1)
	if _, err := b.AddComment("TA-1", "plain note", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AddComment("TA-1", "the reply", "bane", &replyTo); err != nil {
		t.Fatal(err)
	}
	events, err := b.EventsForTask("TA-1")
	if err != nil {
		t.Fatal(err)
	}
	var comments []*Row
	for _, e := range events {
		if e.String("event_type") == "comment" {
			comments = append(comments, e)
		}
	}
	if len(comments) != 3 {
		t.Fatalf("comment events = %d, want 3", len(comments))
	}
	parse := func(e *Row) (string, string, *int64) {
		var inner string
		raw := e.Get("detail")
		if err := json.Unmarshal(raw, &inner); err != nil {
			t.Fatalf("detail is not a JSON string: %v (%s)", err, raw)
		}
		var d struct {
			Body      string `json:"body"`
			InReplyTo *int64 `json:"in_reply_to"`
		}
		if err := json.Unmarshal([]byte(inner), &d); err != nil {
			t.Fatalf("detail does not round-trip to {body, in_reply_to}: %v (%s)", err, inner)
		}
		return d.Body, e.String("actor"), d.InReplyTo
	}
	body, actor, reply := parse(comments[1])
	if body != "plain note" || actor != "foreman" || reply != nil {
		t.Fatalf("plain comment = %q %q %v, want body/foreman/nil", body, actor, reply)
	}
	body, actor, reply = parse(comments[2])
	if body != "the reply" || actor != "bane" || reply == nil || *reply != 1 {
		t.Fatalf("reply comment = %q %q %v, want body/bane/1", body, actor, reply)
	}
	// Loose keys refused: exactly body + in_reply_to when replying.
	var keys map[string]json.RawMessage
	var inner string
	_ = json.Unmarshal(comments[2].Get("detail"), &inner)
	if err := json.Unmarshal([]byte(inner), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("reply detail keys = %v, want exactly body+in_reply_to", keys)
	}
}

// TestBT099_CommentRefusals is test plan #3: unknown id, empty body,
// --reply-to at a non-comment event and at a foreign-task comment — each
// REFUSES with NOTHING appended (event count and MAX(id) unchanged).
// Falsifies half-written audit.
func TestBT099_CommentRefusals(t *testing.T) {
	events := `{"id":1,"timestamp":"2026-11-08 04:00:00","event_type":"audit","task_id":"TA-1","actor":"foreman","detail":null,"tick_number":1}` + "\n" +
		`{"id":2,"timestamp":"2026-11-08 04:55:00","event_type":"comment","task_id":"OTHER-1","actor":"foreman","detail":"{\"body\":\"other thread\"}","tick_number":null}`
	b := bt099Board(t, `{"id":"TA-1","title":"t","status":"pending","priority":"P1"}`+"\n"+`{"id":"OTHER-1","title":"o","status":"pending","priority":"P1"}`, events)
	eventsPath := b.EventsPath()
	before := bt099Read(t, eventsPath)
	countBefore := len(strings.Split(strings.TrimSpace(before), "\n"))

	refuse := func(name string, fn func() error) {
		t.Run(name, func(t *testing.T) {
			if err := fn(); err == nil {
				t.Fatalf("%s: expected a refusal", name)
			}
		})
	}
	badReply := int64(1)
	foreign := int64(2)
	refuse("unknown task id", func() error { _, err := b.AddComment("NOPE-1", "x", "", nil); return err })
	refuse("empty body", func() error { _, err := b.AddComment("TA-1", "   ", "", nil); return err })
	refuse("reply to a non-comment event", func() error { _, err := b.AddComment("TA-1", "x", "", &badReply); return err })
	refuse("reply to a foreign-task comment", func() error { _, err := b.AddComment("TA-1", "x", "", &foreign); return err })

	if after := bt099Read(t, eventsPath); after != before {
		t.Fatalf("refusals appended to events.jsonl:\n--- before\n%s\n--- after\n%s", before, after)
	}
	if got := len(strings.Split(strings.TrimSpace(afterCount(t, eventsPath)), "\n")); got != countBefore {
		t.Fatalf("event count moved: %d -> %d", countBefore, got)
	}
}

func afterCount(t *testing.T, path string) string {
	t.Helper()
	return bt099Read(t, path)
}

// TestBT099_CommentThreadRendering is test plan #4: three comments with one
// reply render in id order with the reply indented one level under its
// ancestor; a deeper chain keeps indenting under its ancestor; --json (the
// CLI test covers the wire) matches the stable {event_id, ts, author, body,
// in_reply_to} shape.
func TestBT099_CommentThreadRendering(t *testing.T) {
	events := `{"id":10,"timestamp":"2026-11-08 04:50:00","event_type":"task_created","task_id":"TA-1","actor":"foreman","detail":null,"tick_number":null}` + "\n" +
		`{"id":11,"timestamp":"2026-11-08 04:55:11","event_type":"comment","task_id":"TA-1","actor":"bane","detail":"{\"body\":\"split BT-098's validator first\"}","tick_number":null}` + "\n" +
		`{"id":12,"timestamp":"2026-11-08 05:20:09","event_type":"comment","task_id":"TA-1","actor":"foreman","detail":"{\"body\":\"agreed, filed as BT-098a\",\"in_reply_to\":11}","tick_number":null}` + "\n" +
		`{"id":13,"timestamp":"2026-11-08 05:30:00","event_type":"comment","task_id":"TA-1","actor":"lane","detail":"{\"body\":\"deeper chain\",\"in_reply_to\":12}","tick_number":null}` + "\n" +
		`{"id":14,"timestamp":"2026-11-08 05:40:00","event_type":"comment","task_id":"TA-1","actor":"foreman","detail":"{\"body\":\"after the reply\"}","tick_number":null}`
	b := bt099Board(t, `{"id":"TA-1","title":"t","status":"pending","priority":"P1"}`, events)
	evs, err := b.EventsForTask("TA-1")
	if err != nil {
		t.Fatal(err)
	}
	thread := CommentThread(evs)
	if len(thread) != 4 {
		t.Fatalf("thread = %d entries, want 4", len(thread))
	}
	if thread[0].Depth != 0 || thread[1].Depth != 1 || thread[2].Depth != 2 || thread[3].Depth != 0 {
		t.Fatalf("thread depths = %d %d %d %d, want 0 1 2 0 (replies indent under their ancestor)",
			thread[0].Depth, thread[1].Depth, thread[2].Depth, thread[3].Depth)
	}
	want := `COMMENTS (4)
  #11 2026-11-08T04:55 bane: split BT-098's validator first
    #12 2026-11-08T05:20 foreman: agreed, filed as BT-098a
      #13 2026-11-08T05:30 lane: deeper chain
  #14 2026-11-08T05:40 foreman: after the reply
`
	if got := renderThreadForTest(thread); got != want {
		t.Fatalf("thread render:\n--- got\n%s--- want\n%s", got, want)
	}
}

// renderThreadForTest mirrors the CLI's renderCommentThread block (the CLI
// test proves the verb wiring; this proves the shape).
func renderThreadForTest(thread []CommentEntry) string {
	if len(thread) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "COMMENTS (%d)\n", len(thread))
	for _, c := range thread {
		fmt.Fprintf(&sb, "%s#%d %s %s: %s\n", strings.Repeat("  ", c.Depth+1), c.EventID, c.TS, c.Author, c.Body)
	}
	return sb.String()
}

// TestBT099_DecomposeScaffolding is test plan #7: --count 3 sequential —
// children i>1 depend on i-1, all carry parent, template fields inherited,
// the parent's audit event lists all three ids; --parallel produces no
// inter-child deps. Falsifies wrong chain shape.
func TestBT099_DecomposeScaffolding(t *testing.T) {
	parent := `{"id":"BIG-100","title":"Do the big thing","status":"pending","priority":"P1","capability_tags":["go","cli"],"type":"feat","detail":{"metadata":{"expected_files":["a.go","b_test.go"],"verify_steps":["run go test"]}}}`
	b := bt099Board(t, parent, "")
	ids, err := b.Decompose("BIG-100", DecomposeOptions{Count: 3, Sequential: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || ids[0] != "BIG-101" || ids[1] != "BIG-102" || ids[2] != "BIG-103" {
		t.Fatalf("ids = %v, want BIG-101..BIG-103 (next-id mint from the parent's family)", ids)
	}
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*Row{}
	for _, r := range rows {
		byID[r.String("id")] = r
	}
	for i, id := range ids {
		row := byID[id]
		if row == nil {
			t.Fatalf("child %s missing", id)
		}
		if got := RowParentID(row); got != "BIG-100" {
			t.Fatalf("%s parent = %q, want BIG-100", id, got)
		}
		if got := row.String("priority"); got != "P1" {
			t.Fatalf("%s priority = %q, want inherited P1", id, got)
		}
		if got := row.String("type"); got != "feat" {
			t.Fatalf("%s type = %q, want inherited feat", id, got)
		}
		if got := rowStringSlice(row, "capability_tags"); len(got) != 2 || got[0] != "go" {
			t.Fatalf("%s capability_tags = %v, want inherited [go cli]", id, got)
		}
		md := inheritedMetadata(row)
		if got := md["expected_files"]; got != "a.go,b_test.go" {
			t.Fatalf("%s expected_files = %q, want inherited a.go,b_test.go", id, got)
		}
		if got := md["verify_steps"]; got != "run go test" {
			t.Fatalf("%s verify_steps = %q, want inherited", id, got)
		}
		wantDeps := []string(nil)
		if i > 0 {
			wantDeps = []string{ids[i-1]}
		}
		if got := rowStringSlice(row, "depends_on"); strings.Join(got, ",") != strings.Join(wantDeps, ",") {
			t.Fatalf("%s depends_on = %v, want %v (sequential chain)", id, got, wantDeps)
		}
	}
	// The parent's ONE audit event lists all three ids.
	evs, err := b.EventsForTask("BIG-100")
	if err != nil {
		t.Fatal(err)
	}
	var detail string
	for _, e := range evs {
		if e.String("event_type") != "task_updated" {
			continue
		}
		if err := json.Unmarshal(e.Get("detail"), &detail); err != nil {
			t.Fatalf("decomposed_into detail is not a JSON string: %v", err)
		}
		var d struct {
			DecomposedInto []string `json:"decomposed_into"`
		}
		if err := json.Unmarshal([]byte(detail), &d); err != nil {
			t.Fatal(err)
		}
		if strings.Join(d.DecomposedInto, ",") != "BIG-101,BIG-102,BIG-103" {
			t.Fatalf("decomposed_into = %v", d.DecomposedInto)
		}
		return
	}
	t.Fatal("no decomposed_into event on the parent")

	// --parallel: NO inter-child deps.
	b2 := bt099Board(t, parent, "")
	ids2, err := b2.Decompose("BIG-100", DecomposeOptions{Count: 3, Parallel: true})
	if err != nil {
		t.Fatal(err)
	}
	rows2, _ := b2.TaskRows()
	for _, r := range rows2 {
		if RowParentID(r) != "BIG-100" {
			continue
		}
		if got := rowStringSlice(r, "depends_on"); len(got) != 0 {
			t.Fatalf("parallel child %s carries deps %v, want none", r.String("id"), got)
		}
	}
	if len(ids2) != 3 {
		t.Fatalf("parallel ids = %v", ids2)
	}
}

// TestBT099_DecomposeRefusals is test plan #8: complete parent, --count 0,
// and a mid-loop id collision — the standard named refusal with the
// documented partial-scaffold behavior (earlier children REMAIN, append-only
// doctrine; the parent gets no decomposed_into event).
func TestBT099_DecomposeRefusals(t *testing.T) {
	tasks := `{"id":"FIN-100","title":"finished","status":"complete","priority":"P1"}` + "\n" +
		`{"id":"OPEN-1","title":"open row","status":"pending","priority":"P1"}` + "\n" +
		`{"id":"DUP-101","title":"already here","status":"pending","priority":"P1"}`
	b := bt099Board(t, tasks, "")
	if _, err := b.Decompose("FIN-100", DecomposeOptions{Count: 2, Parallel: true}); err == nil || !strings.Contains(err.Error(), "complete — open rows only") {
		t.Fatalf("complete parent: got %v, want the open-rows-only refusal", err)
	}
	if _, err := b.Decompose("OPEN-1", DecomposeOptions{Count: 0, Parallel: true}); err == nil || !strings.Contains(err.Error(), "at least 1") {
		t.Fatalf("--count 0: got %v, want the count refusal", err)
	}
	before := bt099Read(t, b.TasksPath())
	if _, err := b.Decompose("OPEN-1", DecomposeOptions{Count: 2, Parallel: true, FirstID: "DUP-101"}); err == nil {
		t.Fatal("--first-id collision was accepted")
	}
	if after := bt099Read(t, b.TasksPath()); after != before {
		t.Fatalf("a fully-refused decompose wrote to the board:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

// TestBT099_DecomposePartialScaffold documents the mid-loop refusal shape:
// children created BEFORE the refusal remain (append-only doctrine), the
// parent gets NO decomposed_into event, and the error names what landed.
// The deterministic mid-loop refusal is the standard SG-126 duplicate-finding
// gate: child 2's title fingerprint matches an existing OPEN row.
func TestBT099_DecomposePartialScaffold(t *testing.T) {
	b := bt099Board(t, `{"id":"PAR-50","title":"parent","status":"pending","priority":"P1"}`, "")
	// The pre-existing OPEN row went through Create, so it carries a STORED
	// finding fingerprint (the dedupe index reads stored fingerprints, not
	// computed ones — a hand-seeded row would never collide).
	if _, err := b.Create(TaskRowSpec{ID: "OTHER-1", Title: "STEP 2/2", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	eventsBefore := len(strings.Split(strings.TrimSpace(bt099Read(t, b.EventsPath())), "\n"))
	ids, err := b.Decompose("PAR-50", DecomposeOptions{Count: 2, Parallel: true, TitlePrefix: "STEP"})
	if err == nil {
		t.Fatalf("mid-loop refusal expected; ids=%v", ids)
	}
	if len(ids) != 1 || ids[0] != "PAR-051" {
		t.Fatalf("ids = %v, want exactly the first child PAR-051", ids)
	}
	rows, _ := b.TaskRows()
	found := map[string]bool{}
	for _, r := range rows {
		found[r.String("id")] = true
	}
	if !found["PAR-051"] || found["PAR-052"] {
		t.Fatalf("partial scaffold wrong: PAR-051=%v PAR-052=%v", found["PAR-051"], found["PAR-052"])
	}
	for _, e := range mustEvents(t, b, "PAR-50") {
		if e.String("event_type") == "task_updated" && strings.Contains(e.String("detail"), "decomposed_into") {
			t.Fatal("a partial scaffold wrote the parent's decomposed_into event")
		}
	}
	if got := len(strings.Split(strings.TrimSpace(bt099Read(t, b.EventsPath())), "\n")); got < eventsBefore {
		t.Fatal("events.jsonl shrank")
	}
}

func mustEvents(t *testing.T, b *Board, id string) []*Row {
	t.Helper()
	evs, err := b.EventsForTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

// TestBT099_ZeroMigration is test plan #10: a board with 219 legacy rows and
// NO parent anywhere — validate clean, and the ready/stats surfaces carry
// their pre-change shape (no withheld key, no CHILDREN block, no COMMENTS
// block). Falsifies accidental shape change to existing boards.
func TestBT099_ZeroMigration(t *testing.T) {
	var sb strings.Builder
	statuses := []string{"pending", "in_progress", "complete", "review", "blocked", "failed"}
	for i := 1; i <= 219; i++ {
		status := statuses[i%len(statuses)]
		sb.WriteString(`{"id":"LEG-` + fmt.Sprintf("%03d", i) + `","title":"legacy row ` + fmt.Sprintf("%03d", i) + `","status":"` + status + `","priority":"P2","complexity":3,"depends_on":[],"created_at":"2026-01-01 00:00:00","updated_at":"2026-01-01 00:00:00"}` + "\n")
	}
	b := bt099Board(t, strings.TrimSuffix(sb.String(), "\n"), "")
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() || len(rep.Findings) != 0 {
		t.Fatalf("legacy board must validate clean:\n%s", rep.RenderText())
	}
	if got := rep.Keys.Summary(); !strings.Contains(got, "219/219 rows carry only canon keys") {
		t.Fatalf("key uniformity = %q", got)
	}
	// stats shape unchanged.
	st, err := b.ComputeStats(TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 219 || st.Actionable == 0 {
		t.Fatalf("stats = total %d actionable %d", st.Total, st.Actionable)
	}
	// ready shape: no withheld key anywhere in the JSON payload (omitempty),
	// no CHILDREN/COMMENTS block in show text.
	report, err := b.AnalyzeDispatchability()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Withheld) != 0 {
		t.Fatalf("parentless board withheld rows = %v", report.Withheld)
	}
	if rollup := ComputeParentRollup("LEG-001", report.Rows); rollup != nil {
		t.Fatalf("parentless row produced a rollup: %+v", rollup)
	}
}
