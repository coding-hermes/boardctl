package board

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWriteMetadataCLIEndToEnd(t *testing.T) {
	b := bt098Seed(t)
	binary := filepath.Join(t.TempDir(), "boardctl")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/boardctl")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	run := func(args ...string) []byte {
		t.Helper()
		args = append(args, "-C", b.Dir)
		output, err := exec.Command(binary, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("boardctl %v: %v: %s", args, err, output)
		}
		return output
	}
	run("create", "--id", "REL-7", "--title", "release", "--type", "release", "--meta", "channel=stable", "--meta", "source_commit=abcdefg", "--meta", "signoffs=a@x,b@x")
	run("update", "REL-7", "--meta", "channel=beta", "--status", "pending")
	// show emits JSON by default; it does not accept a --json flag.
	output := run("show", "REL-7")
	var row struct {
		Detail struct {
			Metadata map[string]json.RawMessage `json:"metadata"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(output, &row); err != nil {
		t.Fatal(err)
	}
	var signoffs []string
	if err := json.Unmarshal(row.Detail.Metadata["signoffs"], &signoffs); err != nil {
		t.Fatalf("decode signoffs: %v; show: %s", err, output)
	}
	if !reflect.DeepEqual(signoffs, []string{"a@x", "b@x"}) {
		t.Fatalf("signoffs = %v; show: %s", signoffs, output)
	}
}

func TestWriteMetadataPreservesStoredValues(t *testing.T) {
	b := bt098Seed(t)
	_, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "release", Type: "release", HasType: true, Status: "pending", Metadata: map[string]string{"channel": "stable", "source_commit": "abcdefg", "signoffs": "a@x,b@x", "approval_ticket": "REL-TEST-1"}})
	if err != nil {
		t.Fatal(err)
	}
	// Bool and number are undeclared: the type registry only supports string,
	// enum, list and ref, but arbitrary metadata must still be retained.
	path := b.TasksPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"signoffs":["a@x","b@x"]`) {
		t.Fatal("created row lacks expected signoffs bytes")
	}
	data = []byte(strings.Replace(string(data), `"signoffs":["a@x","b@x"]`, `"enabled":true,"count":12345678901234567890,"signoffs":["a@x","b@x"]`, 1))
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	update := map[string]string{"channel": "beta"}
	if _, err := b.UpdateTask("REL-TEST-2", UpdateSpec{Metadata: &update}); err != nil {
		t.Fatal(err)
	}
	row := bt098RowByID(t, b, "REL-TEST-2")
	for _, want := range []string{`"signoffs":["a@x","b@x"]`, `"approval_ticket":"REL-TEST-1"`, `"enabled":true`, `"count":12345678901234567890`, `"channel":"beta"`} {
		if !strings.Contains(row, want) {
			t.Errorf("missing %s in %s", want, row)
		}
	}
	if strings.Contains(row, `"signoffs":null`) {
		t.Fatal(row)
	}
	var parsed struct {
		Detail struct {
			Metadata map[string]json.RawMessage `json:"metadata"`
		} `json:"detail"`
	}
	if err := json.Unmarshal([]byte(row), &parsed); err != nil {
		t.Fatal(err)
	}
	if string(parsed.Detail.Metadata["signoffs"]) != `["a@x","b@x"]` {
		t.Fatal(string(parsed.Detail.Metadata["signoffs"]))
	}
}

func TestWriteMetadataRetagPreservesList(t *testing.T) {
	b := bt098Seed(t)
	_, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "release", Type: "release", HasType: true, Status: "pending", Metadata: map[string]string{"channel": "stable", "source_commit": "abcdefg", "signoffs": "a@x,b@x"}})
	if err != nil {
		t.Fatal(err)
	}
	typ := "release"
	if _, err := b.UpdateTask("REL-TEST-2", UpdateSpec{Type: &typ}); err != nil {
		t.Fatal(err)
	}
	if row := bt098RowByID(t, b, "REL-TEST-2"); !strings.Contains(row, `"signoffs":["a@x","b@x"]`) {
		t.Fatal(row)
	}
}

func TestWriteMetadataInvalidStoredRefuses(t *testing.T) {
	for _, retag := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "retag"}[retag], func(t *testing.T) {
			b := bt098Seed(t)
			path := filepath.Join(b.Dir, "tasks.jsonl")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.Replace(string(data), `"channel":"beta"`, `"channel":["beta"]`, 1))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			tasks, events := bt098Snapshot(t, b)
			update := map[string]string{"source_commit": "new"}
			spec := UpdateSpec{Metadata: &update}
			if retag {
				typ := "release"
				spec = UpdateSpec{Type: &typ}
			}
			if _, err := b.UpdateTask("REL-TEST-1", spec); err == nil || !strings.Contains(err.Error(), "channel") {
				t.Fatalf("error = %v", err)
			}
			bt098AssertUnchanged(t, b, tasks, events)
		})
	}
}

func TestWriteMetadataOptionalListAbsent(t *testing.T) {
	b := bt098Seed(t)
	_, err := b.Create(TaskRowSpec{ID: "REL-TEST-2", Title: "release", Type: "release", HasType: true, Status: "pending", Metadata: map[string]string{"channel": "stable", "source_commit": "abcdefg"}})
	if err != nil {
		t.Fatal(err)
	}
	update := map[string]string{"channel": "beta"}
	if _, err := b.UpdateTask("REL-TEST-2", UpdateSpec{Metadata: &update}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"signoffs"`, `"approval_ticket"`} {
		if row := bt098RowByID(t, b, "REL-TEST-2"); strings.Contains(row, key) {
			t.Fatalf("optional %s present: %s", key, row)
		}
	}
}
