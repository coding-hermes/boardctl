package board

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// BT-077: plural work-artifact associations (pull_requests, branches,
// worktrees) as first-class row fields. These tests pin the whole contract:
//
//   - create --pull-request/--assoc-branch/--assoc-worktree land canonical
//     elements on the fresh row; multiple flags = multiple elements, in
//     CLI order
//   - update MERGES one element per dimension without overwriting
//     siblings: other dimensions, other elements of the same dimension,
//     the singular worktree/branch fields and sessions all survive
//     byte-verbatim (including untouched-line round-trip)
//   - PR normalization: bare number -> {"number": N}; GitHub PR URL ->
//     {"number": N, "url": U} (number parsed from the URL, trailing
//     fragment/query/slash stripped); malformed input ("bad id!", a
//     non-GitHub URL, 0, empty) is REJECTED by the write path with a
//     clear error naming the dimension and value
//   - PR identity is the NUMBER: merging URL-form then bare-form of the
//     same PR is one element (idempotent), and the richer stored form
//     (with url) is kept
//   - legacy singular rows (worktree/branch/sessions only) load, validate
//     (zero findings), show, render and merge-extend fine
//   - validate itemizes malformed stored elements as WARNINGS (exit-0);
//     canonical rows and legacy rows stay silent
//   - the new keys are canon-sanctioned (no BT-056 drift warning)
//
// Seed rows use spaced style on purpose: arrays must be encoded in the
// file's own style (the encodeValue []any path through Style), never a
// hand-rolled compact json.Marshal.

// seedBT077Board writes a topology-A board with one plain row to update
// and one LEGACY singular row (worktree/branch/sessions, no plural keys).
func seedBT077Board(t *testing.T) *Board {
	t.Helper()
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "AS-1", "title": "plain pending", "status": "pending", "priority": "P2"}` + "\n" +
			`{"id": "AS-LEG", "title": "legacy singular", "status": "pending", "priority": "P3", "worktree": "/home/me/wt-1", "branch": "wt/as-1", "sessions": ["s1", "s2"]}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt077", "namespace": "bt077", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// bt077RowOrNil returns the parsed row with the given id (nil when absent).
func bt077RowOrNil(t *testing.T, b *Board, id string) *Row {
	t.Helper()
	rows, err := b.TaskRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.String("id") == id {
			return r
		}
	}
	return nil
}

// bt077Line returns the raw tasks.jsonl line carrying the given id (the
// exact-bytes oracle for style and sibling-preservation assertions).
func bt077Line(t *testing.T, b *Board, id string) string {
	t.Helper()
	raw, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(raw), "\n") {
		row, err := ParseRow([]byte(l))
		if err == nil && row.String("id") == id {
			return l
		}
	}
	t.Fatalf("line for %q not found", id)
	return ""
}

// bt077Objs decodes a dimension into decoded objects for assertion.
func bt077Objs(t *testing.T, line, key string) []map[string]any {
	t.Helper()
	row, err := ParseRow([]byte(line))
	if err != nil {
		t.Fatalf("line does not parse: %v", err)
	}
	raw := row.Get(key)
	if raw == nil {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s does not decode as an array of objects: %v (%s)", key, err, raw)
	}
	return out
}

// bt077Strings decodes a string-array dimension.
func bt077Strings(t *testing.T, line, key string) []string {
	t.Helper()
	row, err := ParseRow([]byte(line))
	if err != nil {
		t.Fatalf("line does not parse: %v", err)
	}
	raw := row.Get(key)
	if raw == nil {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s does not decode as a string array: %v (%s)", key, err, raw)
	}
	return out
}

// ---------- create ----------

// TestCreateAssociationsMultiValue: create with repeated flags lands
// multiple canonical elements in the dimension's array in CLI order, in
// the file's own spaced style; the omitted dimensions write no keys; the
// create stays append-only (pre-existing bytes intact).
func TestCreateAssociationsMultiValue(t *testing.T) {
	b := seedBT077Board(t)
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Create(TaskRowSpec{
		ID: "AS-NEW", Title: "assoc row",
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"21", "https://github.com/o/r/pull/22"},
	}); err != nil {
		t.Fatalf("create with pull_requests: %v", err)
	}
	// append-only: every pre-existing byte is still there unchanged.
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, before) {
		t.Fatalf("create rewrote existing bytes:\nbefore %q\nafter  %q", before, after)
	}
	line := bt077Line(t, b, "AS-NEW")
	// Spaced style like the seed file; number before url; the URL's own
	// number is parsed into the object.
	want := `"pull_requests": [{"number":21}, {"number":22,"url":"https://github.com/o/r/pull/22"}]`
	if !strings.Contains(line, want) {
		t.Fatalf("pull_requests not serialized canonically in the file's style:\nwant: %s\ngot:  %s", want, line)
	}
	// The dimensions not named write no key (nil-untouched discipline).
	if strings.Contains(line, `"branches"`) || strings.Contains(line, `"worktrees"`) {
		t.Fatalf("omitted association dimensions wrote keys anyway:\n%s", line)
	}
}

// TestCreateAssociationValidationRejectsMalformed: the write path refuses
// malformed elements with an error naming the dimension and value; NOTHING
// is appended (the file is byte-identical after each failed create).
func TestCreateAssociationValidationRejectsMalformed(t *testing.T) {
	b := seedBT077Board(t)
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		dim, val, wantSub string
	}{
		{PullRequestsKey, "bad id!", "not a GitHub PR URL"},
		{PullRequestsKey, "https://gitlab.com/o/r/-/merge_requests/3", "not a GitHub PR URL"},
		{PullRequestsKey, "0", "positive integer"},
		{PullRequestsKey, "-3", "positive integer"},
		{PullRequestsKey, "", "empty pull_requests"},
		{BranchesKey, "   ", "non-empty string"},
		{WorktreesKey, "", "non-empty string"},
	}
	for _, tc := range cases {
		_, err := b.Create(TaskRowSpec{
			ID: "AS-BAD", Title: "never lands",
			AssocDim:    tc.dim,
			AssocValues: []string{tc.val},
		})
		if err == nil {
			t.Fatalf("%s %q: create accepted a malformed element", tc.dim, tc.val)
		}
		if !strings.Contains(err.Error(), tc.wantSub) {
			t.Errorf("%s %q: error %q does not name the problem (want substring %q)", tc.dim, tc.val, err, tc.wantSub)
		}
	}
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("failed create left bytes behind:\nbefore %q\nafter  %q", before, after)
	}
	if bt077RowOrNil(t, b, "AS-BAD") != nil {
		t.Fatal("failed create appended its row anyway")
	}
}

// ---------- PR normalization ----------

// TestNormalizePullRequestRules pins the accepted shapes and canonical
// outputs element-by-element (the unit oracle behind the CLI behavior).
func TestNormalizePullRequestRules(t *testing.T) {
	cases := []struct {
		in   string
		want string // canonical JSON
	}{
		{"21", `{"number":21}`},
		{"  21  ", `{"number":21}`},
		{"007", `{"number":7}`},
		{"https://github.com/o/r/pull/22", `{"number":22,"url":"https://github.com/o/r/pull/22"}`},
		{"https://www.github.com/o/r/pull/22", `{"number":22,"url":"https://www.github.com/o/r/pull/22"}`},
		{"http://github.com/o/r/pull/22", `{"number":22,"url":"http://github.com/o/r/pull/22"}`},
		{"https://github.com/o/r/pull/22/files", `{"number":22,"url":"https://github.com/o/r/pull/22/files"}`},
		{"https://github.com/o/r/pull/22#issuecomment-1", `{"number":22,"url":"https://github.com/o/r/pull/22"}`},
		{"https://github.com/o/r/pull/22?diff=split", `{"number":22,"url":"https://github.com/o/r/pull/22"}`},
		{"https://github.com/o/r/pull/22/", `{"number":22,"url":"https://github.com/o/r/pull/22"}`},
		{`{"number": 5}`, `{"number":5}`},
		{`{"url": "https://github.com/o/r/pull/6"}`, `{"number":6,"url":"https://github.com/o/r/pull/6"}`},
		{`{"number": 8, "url": "https://github.com/o/r/pull/8"}`, `{"number":8,"url":"https://github.com/o/r/pull/8"}`},
	}
	for _, tc := range cases {
		got, err := NormalizePullRequest(tc.in)
		if err != nil {
			t.Errorf("%q: unexpected error %v", tc.in, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("%q: got %s, want %s", tc.in, got, tc.want)
		}
	}
	bad := []string{
		"", " ", "0", "-1", "bad id!", "22a",
		"https://gitlab.com/o/r/-/merge_requests/3",
		"https://github.com/o/r",                                // not a PR URL
		"https://github.com/o/pull/3",                           // one path segment: not owner/repo/pull
		"ftp://github.com/o/r/pull/4",                           // scheme
		`{"number": 0}`,                                         // zero
		`{"number": -2}`,                                        // negative
		`{"url": "https://gitlab.com/x/y/-/mr/1"}`,              // non-GitHub url
		`{"number": 5, "url": "https://github.com/o/r/pull/6"}`, // disagreement
		`{}`,              // neither field
		`{"number": "x"}`, // wrong type
	}
	for _, tc := range bad {
		if got, err := NormalizePullRequest(tc); err == nil {
			t.Errorf("%q: accepted, produced %s", tc, got)
		}
	}
}

// TestValidateAssociationValueStrings: the string dimensions trim
// whitespace, reject empties, and reject unknown dimensions.
func TestValidateAssociationValueStrings(t *testing.T) {
	for _, dim := range []string{BranchesKey, WorktreesKey} {
		got, err := ValidateAssociationValue(dim, "  wt/x  ")
		if err != nil || string(got) != `"wt/x"` {
			t.Errorf("%s: got %s, %v; want \"wt/x\"", dim, got, err)
		}
		_, err = ValidateAssociationValue(dim, "   ")
		if err == nil {
			t.Errorf("%s: accepted an empty element", dim)
		}
		if !strings.Contains(err.Error(), "non-empty string") {
			t.Errorf("%s: error does not say non-empty string: %v", dim, err)
		}
	}
	_, err := ValidateAssociationValue("commits", "abc")
	if err == nil || !strings.Contains(err.Error(), "unknown association dimension") {
		t.Errorf("unknown dimension not rejected: %v", err)
	}
}

// ---------- update: merge semantics ----------

// TestUpdateMergesWithoutOverwritingSiblings: adding a third PR keeps the
// first two, both other dimensions, and the singular fields; untouched
// bytes of the row and every other line round-trip.
func TestUpdateMergesWithoutOverwritingSiblings(t *testing.T) {
	b := seedBT077Board(t)
	// Build the multi-association row: two PRs, two branches, two
	// worktrees, plus singular fields and sessions.
	wt := "/tmp/w1"
	br := "wt/x"
	list := []string{"s1"}
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		Worktree: &wt, Branch: &br, Sessions: &list,
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"21", "https://github.com/o/r/pull/22"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    BranchesKey,
		AssocValues: []string{"feat/a", "feat/b"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    WorktreesKey,
		AssocValues: []string{"/tmp/w1", "/tmp/w2"},
	}); err != nil {
		t.Fatal(err)
	}
	// Adding a THIRD PR must keep the first two PRs, all three other
	// dimensions, and the singular fields.
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"23"},
	}); err != nil {
		t.Fatal(err)
	}
	line := bt077Line(t, b, "AS-1")
	prs := bt077Objs(t, line, PullRequestsKey)
	if len(prs) != 3 {
		t.Fatalf("pull_requests = %d elements, want 3 (adding one must not drop siblings)", len(prs))
	}
	if n := prs[0]["number"]; n != float64(21) {
		t.Errorf("first PR = %v, want 21", n)
	}
	if n := prs[1]["number"]; n != float64(22) {
		t.Errorf("second PR = %v, want 22", n)
	}
	if u, _ := prs[1]["url"].(string); u != "https://github.com/o/r/pull/22" {
		t.Errorf("second PR url = %v, want the github URL (the richer stored form survives)", prs[1]["url"])
	}
	branches := bt077Strings(t, line, BranchesKey)
	if len(branches) != 2 || branches[0] != "feat/a" || branches[1] != "feat/b" {
		t.Fatalf("branches changed by a pull_requests update: %v", branches)
	}
	wts := bt077Strings(t, line, WorktreesKey)
	if len(wts) != 2 || wts[0] != "/tmp/w1" || wts[1] != "/tmp/w2" {
		t.Fatalf("worktrees changed by a pull_requests update: %v", wts)
	}
	if !strings.Contains(line, `"worktree": "/tmp/w1"`) || !strings.Contains(line, `"branch": "wt/x"`) {
		t.Fatalf("singular worktree/branch did not survive:\n%s", line)
	}
	if !strings.Contains(line, `"sessions": ["s1"]`) {
		t.Fatalf("sessions did not survive:\n%s", line)
	}
	// other rows byte-identical
	legacy := bt077Line(t, b, "AS-LEG")
	if !strings.Contains(legacy, `"worktree": "/home/me/wt-1"`) || strings.Contains(legacy, "pull_requests") {
		t.Fatalf("legacy sibling row mutated:\n%s", legacy)
	}
}

// TestUpdateAssociationIdempotentByNumber: merging the same PR in its other
// spelling (bare <-> URL) is a no-op reporting no change; the element is
// not duplicated and nothing else moves. Exact duplicates on the string
// dimensions are idempotent the same way.
func TestUpdateAssociationIdempotentByNumber(t *testing.T) {
	b := seedBT077Board(t)
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"https://github.com/o/r/pull/22"},
	}); err != nil {
		t.Fatal(err)
	}
	changed, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"22"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changed {
		if c == PullRequestsKey {
			t.Fatalf("duplicate PR merge reported a change: %v", changed)
		}
	}
	line := bt077Line(t, b, "AS-1")
	if !strings.Contains(line, `{"number":22,"url":"https://github.com/o/r/pull/22"}`) {
		t.Fatalf("url-bearing element not preserved across the no-op merge:\n%s", line)
	}
	if strings.Count(line, `"number":22`) != 1 {
		t.Fatalf("PR 22 duplicated:\n%s", line)
	}
	// exact-duplicate idempotence on the string dimensions too
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    BranchesKey,
		AssocValues: []string{"feat/x"},
	}); err != nil {
		t.Fatal(err)
	}
	changed, err = b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    BranchesKey,
		AssocValues: []string{"feat/x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range changed {
		if c == BranchesKey {
			t.Fatalf("exact-duplicate branch merge reported a change: %v", changed)
		}
	}
	if n := strings.Count(bt077Line(t, b, "AS-1"), `"feat/x"`); n != 1 {
		t.Fatalf("branch duplicated %d times", n)
	}
}

// TestUpdateAssociationValidationAbortsWrite: a malformed element fails the
// whole update with NOTHING written — every line byte-identical after.
func TestUpdateAssociationValidationAbortsWrite(t *testing.T) {
	b := seedBT077Board(t)
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"21"},
	}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	_, err = b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"bad id!"},
	})
	if err == nil {
		t.Fatal("update accepted a malformed element")
	}
	after, err := os.ReadFile(b.TasksPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("failed update left bytes behind:\nbefore %q\nafter  %q", before, after)
	}
}

// ---------- legacy compat ----------

// TestLegacySingularRowSurvivesEverywhere: a task that only carries the
// singular worktree/branch/sessions fields loads, validates with zero
// findings, shows, renders into the report payload — and an association
// merge onto it preserves the singular fields byte-verbatim.
func TestLegacySingularRowSurvivesEverywhere(t *testing.T) {
	b := seedBT077Board(t)
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() || len(rep.Findings) != 0 {
		t.Fatalf("legacy board has findings:\n%s", rep.RenderText())
	}
	row, _, err := b.ShowTask("AS-LEG")
	if err != nil || row == nil {
		t.Fatalf("show lost the legacy row: %v %v", row, err)
	}
	// merge onto the legacy row keeps every singular byte
	if _, err := b.UpdateTask("AS-LEG", UpdateSpec{
		AssocDim:    WorktreesKey,
		AssocValues: []string{"/home/me/wt-2"},
	}); err != nil {
		t.Fatal(err)
	}
	line := bt077Line(t, b, "AS-LEG")
	for _, want := range []string{
		`"worktree": "/home/me/wt-1"`, `"branch": "wt/as-1"`,
		`"sessions": ["s1", "s2"]`, `"worktrees": ["/home/me/wt-2"]`,
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("legacy merge lost %s:\n%s", want, line)
		}
	}
	// the CLI render surface is pinned in cmd/boardctl's bt077 test (the
	// render package imports this one; a payload check here would be an
	// import cycle)
}

// ---------- validate ----------

// TestValidateWarnsOnMalformedStoredElements: hand-edited rows carrying
// malformed elements are itemized WARNINGS (exit-0); a canonical row stays
// silent; a non-array dimension value warns too.
func TestValidateWarnsOnMalformedStoredElements(t *testing.T) {
	dir := t.TempDir()
	writeBoardFiles(t, dir, map[string]string{
		"tasks.jsonl": `{"id": "AS-OK", "title": "canonical", "status": "pending", "priority": "P2", "pull_requests": [{"number": 21}], "branches": ["feat/a"]}` + "\n" +
			`{"id": "AS-BADPR", "title": "hand-edited", "status": "pending", "priority": "P2", "pull_requests": [{"number": 21}, "not-an-object"]}` + "\n" +
			`{"id": "AS-BADSTR", "title": "empty branch", "status": "pending", "priority": "P2", "branches": [""]}` + "\n" +
			`{"id": "AS-BADSHAPE", "title": "not an array", "status": "pending", "priority": "P2", "worktrees": "/tmp/w1"}` + "\n",
		"events.jsonl": `{"id": 1, "timestamp": "2026-10-07 00:00:00", "event_type": "audit", "task_id": null, "actor": "foreman", "detail": null, "tick_number": 1}` + "\n",
		"board.jsonl":  `{"project": "bt077", "namespace": "bt077", "version": 1, "ticks_total": 1, "ticks_idle": 0, "last_commit": "abc1234"}` + "\n",
	})
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if rep.HasErrors() {
		t.Fatalf("malformed ASSOCIATION elements must warn, not fail:\n%s", rep.RenderText())
	}
	got := map[string]int{}
	for _, f := range rep.Findings {
		if f.Level != "warn" {
			t.Fatalf("non-warn finding on an associations board:\n[%s] %s", f.Level, f.Msg)
		}
		switch {
		case strings.Contains(f.Msg, "AS-BADPR") && strings.Contains(f.Msg, PullRequestsKey):
			got["badpr"]++
		case strings.Contains(f.Msg, "AS-BADSTR") && strings.Contains(f.Msg, BranchesKey):
			got["badstr"]++
		case strings.Contains(f.Msg, "AS-BADSHAPE") && strings.Contains(f.Msg, WorktreesKey) && strings.Contains(f.Msg, "not an array"):
			got["badshape"]++
		case strings.Contains(f.Msg, "AS-OK"):
			t.Fatalf("canonical row AS-OK flagged:\n%s", f.Msg)
		}
	}
	if got["badpr"] != 1 || got["badstr"] != 1 || got["badshape"] != 1 {
		t.Fatalf("expected exactly one warning per malformed row, got %v:\n%s", got, rep.RenderText())
	}
}

// TestAssociationKeysSanctioned: the three plural keys must not flag as
// canon drift (BT-056) on any row that carries them.
func TestAssociationKeysSanctioned(t *testing.T) {
	for _, k := range AssociationKeys {
		if !IsSanctionedTaskKey(k) {
			t.Errorf("key %q should be sanctioned (BT-077)", k)
		}
	}
	b := seedBT077Board(t)
	if _, err := b.UpdateTask("AS-1", UpdateSpec{
		AssocDim:    PullRequestsKey,
		AssocValues: []string{"1"},
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := b.Validate()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if strings.Contains(f.Msg, "outside the declared canon") {
			t.Fatalf("association key flagged as drift:\n%s", f.Msg)
		}
	}
}
