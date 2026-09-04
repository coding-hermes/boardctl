package board

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initFileMap reads the board dir into a name -> content map.
func initFileMap(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(data)
	}
	return out
}

// TestInitEmptyDirCreatesTopologyA: init on a fresh directory writes exactly
// the three topology-A files under .coding-hermes/board, the header parses
// with project/namespace identity, and the board resolves as topology A.
func TestInitEmptyDirCreatesTopologyA(t *testing.T) {
	dir := t.TempDir()
	boardDir, wrote, err := Init(dir, InitOptions{Project: "demo", Namespace: "ns-demo"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".coding-hermes", "board")
	if boardDir != want {
		t.Fatalf("board dir = %q, want %q", boardDir, want)
	}
	if len(wrote) != 3 {
		t.Fatalf("wrote %d files, want 3: %v", len(wrote), wrote)
	}
	files := initFileMap(t, boardDir)
	if len(files) != 3 {
		t.Fatalf("board dir holds %d files, want 3 (tasks, events, board): %v", len(files), files)
	}
	for _, name := range []string{"tasks.jsonl", "events.jsonl", "board.jsonl"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("%s missing after init", name)
		}
	}
	if files["tasks.jsonl"] != "" || files["events.jsonl"] != "" {
		t.Fatalf("tasks/events must start empty: %q / %q", files["tasks.jsonl"], files["events.jsonl"])
	}
	hdr, err := ParseRow([]byte(strings.TrimRight(files["board.jsonl"], "\n")))
	if err != nil {
		t.Fatalf("header does not parse: %v (%s)", err, files["board.jsonl"])
	}
	if hdr.String("project") != "demo" || hdr.String("namespace") != "ns-demo" {
		t.Fatalf("header identity wrong: %s", files["board.jsonl"])
	}
	if _, ok := hdr.Int("version"); !ok {
		t.Fatalf("header version missing/not integer: %s", files["board.jsonl"])
	}

	b, err := Resolve(dir)
	if err != nil {
		t.Fatalf("board does not resolve after init: %v", err)
	}
	if b.Topology != "A" {
		t.Fatalf("topology = %q, want A", b.Topology)
	}
	if b.Dir != boardDir {
		t.Fatalf("resolved dir %q != init dir %q", b.Dir, boardDir)
	}
}

// TestInitDefaultsProjectFromDirBasename: with no --project/--namespace the
// header takes the init target's basename for both (skipping the
// board/.coding-hermes path components).
func TestInitDefaultsProjectFromDirBasename(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Init(dir, InitOptions{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", "board.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	hdr, err := ParseRow(raw)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(dir)
	if hdr.String("project") != base || hdr.String("namespace") != base {
		t.Fatalf("default identity = %q/%q, want %q/%q",
			hdr.String("project"), hdr.String("namespace"), base, base)
	}
}

// TestInitDefaultsWhenBoardIsNestedExplicitly: init -C <dir>/.coding-hermes
// still lands the board at .coding-hermes/board (Resolve's candidate order),
// and the project default skips the .coding-hermes component.
func TestInitDefaultsWhenBoardIsNestedExplicitly(t *testing.T) {
	dir := t.TempDir()
	boardDir, _, err := Init(filepath.Join(dir, ".coding-hermes"), InitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".coding-hermes", "board")
	if boardDir != want {
		t.Fatalf("board dir = %q, want %q", boardDir, want)
	}
	if _, err := Resolve(dir); err != nil {
		t.Fatalf("board does not resolve from repo root: %v", err)
	}
}

// TestInitIdempotentAndNoClobber: re-running init on an initialized board is
// a no-op returning ErrAlreadyInitialized, and a board with existing content
// is left byte-identical.
func TestInitIdempotentAndNoClobber(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Init(dir, InitOptions{Project: "demo"}); err != nil {
		t.Fatal(err)
	}
	boardDir := filepath.Join(dir, ".coding-hermes", "board")

	// Give tasks.jsonl and events.jsonl real content, then re-init.
	taskLine := `{"id":"ONE","title":"First","status":"pending","priority":"P2"}` + "\n"
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(taskLine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := initFileMap(t, boardDir)

	_, wrote, err := Init(dir, InitOptions{Project: "demo"})
	if !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("re-init err = %v, want ErrAlreadyInitialized (wrote=%v)", err, wrote)
	}
	if len(wrote) != 0 {
		t.Fatalf("re-init wrote %v, want nothing", wrote)
	}
	after := initFileMap(t, boardDir)
	if len(before) != len(after) {
		t.Fatalf("file set changed: %v -> %v", before, after)
	}
	for name, content := range before {
		if after[name] != content {
			t.Fatalf("%s clobbered by re-init:\n got %q\nwant %q", name, after[name], content)
		}
	}

	// An in-progress bootstrap (tasks.jsonl exists empty, board.jsonl missing)
	// must be COMPLETABLE: board.jsonl gets seeded, tasks/events untouched.
	if err := os.Remove(filepath.Join(boardDir, "board.jsonl")); err != nil {
		t.Fatal(err)
	}
	_, wrote, err = Init(dir, InitOptions{Project: "demo"})
	if err != nil {
		t.Fatalf("completing half-initialized board: %v", err)
	}
	if len(wrote) != 1 || filepath.Base(wrote[0]) != "board.jsonl" {
		t.Fatalf("wrote %v, want only board.jsonl", wrote)
	}
	if got := initFileMap(t, boardDir)["tasks.jsonl"]; got != taskLine {
		t.Fatalf("tasks.jsonl clobbered while completing bootstrap: %q", got)
	}
}

// TestInitRejectsTopologyBBoard: a hand-made topology-B board (header row on
// line 1 of tasks.jsonl, no board.jsonl) must NOT be re-initialized — init
// bootstraps fresh boards only, and refuses to layer a new board.jsonl over
// the real header (the board itself is writable; migration is a manual,
// optional step).
func TestInitRejectsTopologyBBoard(t *testing.T) {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"project":"legacy","namespace":"legacy","version":1,"ticks_total":0}` + "\n" + `{"id":"W-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	_, wrote, err := Init(dir, InitOptions{})
	if err == nil {
		t.Fatalf("init on topology-B board accepted (wrote=%v)", wrote)
	}
	if !strings.Contains(err.Error(), "topology B") || !strings.Contains(err.Error(), "fresh boards") {
		t.Fatalf("error %q does not explain the topology-B situation", err)
	}
	if _, err := os.Stat(filepath.Join(boardDir, "board.jsonl")); !os.IsNotExist(err) {
		t.Fatal("board.jsonl was written despite the topology-B refusal")
	}
}

// TestCreateOnEmptyInitializedBoard: `create` on the empty-but-initialized
// tasks.jsonl (exactly what init produces) must succeed using the built-in
// default schema — the standard fleet task fields — and the second create
// mirrors THAT row's schema (fresh boards stay self-similar).
func TestCreateOnEmptyInitializedBoard(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Init(dir, InitOptions{Project: "demo"}); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Create(TaskRowSpec{ID: "T-1", Title: "First task", Status: "pending", Priority: "P1"}); err != nil {
		t.Fatalf("create on empty initialized board: %v", err)
	}
	rows, _, err := ReadAllRows(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("tasks.jsonl holds %d rows, want 1", len(rows))
	}
	first := rows[0]
	if got := first.String("status"); got != "pending" {
		t.Fatalf("status = %q, want pending", got)
	}
	// The built-in default schema: every standard fleet field present.
	for _, k := range DefaultTaskRowKeys {
		if !first.Has(k) {
			t.Fatalf("first row missing default-schema key %q: %s", k, RowJSONCompact(first))
		}
	}
	if first.String("worker_status") != "pending" {
		t.Fatalf("worker_status = %q, want pending", first.String("worker_status"))
	}
	if n, ok := first.Int("attempts"); !ok || n != 0 {
		t.Fatalf("attempts = %v, want 0", first.Get("attempts"))
	}
	if raw := first.Get("depends_on"); raw == nil || strings.Contains(string(raw), "null") {
		t.Fatalf("depends_on should be an array, got %s", raw)
	}
	if first.String("created_at") == "" {
		t.Fatal("created_at not stamped")
	}

	// The empty-board path is repeatable: second create mirrors row 1.
	if _, err := b.Create(TaskRowSpec{ID: "T-2", Title: "Second", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	rows, _, err = ReadAllRows(b.tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("tasks.jsonl holds %d rows, want 2", len(rows))
	}
	if len(rows[1].Keys) != len(rows[0].Keys) {
		t.Fatalf("second row schema diverged: %d keys vs %d", len(rows[1].Keys), len(rows[0].Keys))
	}
}

// initTopologyBRefusalMessage is the exact error Init returns when the
// target's tasks.jsonl line 1 is a topology-B header (built here on a temp
// probe dir). Feeds the stale-copy guard below.
func initTopologyBRefusalMessage(t *testing.T) string {
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	writeBoardFiles(t, boardDir, map[string]string{
		"tasks.jsonl":  `{"project":"probe","namespace":"probe","version":1,"ticks_total":0}` + "\n" + `{"id":"W-1","title":"Work","status":"pending","priority":"P1"}` + "\n",
		"events.jsonl": `{"id":1,"timestamp":"2026-09-03 00:00:00.000000","event_type":"audit","task_id":null,"actor":"foreman","detail":"{}","tick_number":1}` + "\n",
	})
	_, _, err := Init(dir, InitOptions{})
	if err == nil {
		return ""
	}
	return err.Error()
}

// staleNonTestSourceMarkers are phrases that must never reappear in live
// non-test source: they date from the retired DuckDB-cache era or misstate
// topology-B writability. ("board.db" is scanned too; it still legitimately
// appears in doctor.go as the tracked-file check, so only the live
// error/warn COPY is asserted clean via the refusal message above.)
func staleNonTestSourceMarkers() []string {
	return []string{"board.db", "scripts/update", "DuckDB cache", "read-only legacy layout"}
}

// TestStaleTopologyBMessagesGone: the DuckDB-era copy must be fully retired —
// no live error/warning may mention board.db-as-header-home or scripts/update.
// BT-010 world: topology-B boards are WRITABLE, so the copy must say so
// (legacy layout, line-1 tasks.jsonl header) while still pointing at the
// optional migration (split line 1 of tasks.jsonl into board.jsonl).
func TestStaleTopologyBMessagesGone(t *testing.T) {
	stale := []string{"board.db", "scripts/update", "DuckDB cache"}
	for _, s := range []string{initTopologyBRefusalMessage(t)} {
		for _, bad := range stale {
			if strings.Contains(s, bad) {
				t.Fatalf("stale topology-B text %q still present in: %s", bad, s)
			}
		}
		if !strings.Contains(s, "topology B") || !strings.Contains(s, "writable") {
			t.Fatalf("topology-B message does not describe the writable reality: %s", s)
		}
		if !strings.Contains(s, "tasks.jsonl") || !strings.Contains(s, "board.jsonl") {
			t.Fatalf("topology-B message lacks migration guidance: %s", s)
		}
	}
	// Also scan the package source for any lingering stale copy.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range append([]string{"scripts/update"}, staleNonTestSourceMarkers()...) {
			if strings.Contains(string(data), bad) {
				t.Fatalf("%s still references %q", e.Name(), bad)
			}
		}
	}
}
