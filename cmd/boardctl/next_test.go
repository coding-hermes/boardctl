package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeNextBoard(t *testing.T, rows string) string {
	t.Helper()
	dir := t.TempDir()
	boardDir := filepath.Join(dir, ".coding-hermes", "board")
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "tasks.jsonl"), []byte(rows), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(boardDir, "events.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func nextTestRow(id, status, priority string, extra string) string {
	return fmt.Sprintf(`{"id":"%s","title":"title %s","status":"%s","priority":"%s","acceptance_criteria":"criteria %s"%s}`+"\n", id, id, status, priority, id, extra)
}

func TestCmdNextDispatchableRows(t *testing.T) {
	tests := []struct {
		name string
		rows string
		want string
		code int
		err  string
	}{
		{
			name: "500 complete plus pending",
			rows: func() string {
				var b strings.Builder
				for i := 0; i < 500; i++ {
					b.WriteString(nextTestRow(fmt.Sprintf("BT-C-%03d", i), "complete", "P5", ""))
				}
				b.WriteString(nextTestRow("BT-NEXT", "pending", "P1", ""))
				return b.String()
			}(),
			want: "id: BT-NEXT\npriority: P1\ntitle: title BT-NEXT\nacceptance_criteria: criteria BT-NEXT\n",
		},
		{
			name: "all complete",
			rows: nextTestRow("BT-DONE", "complete", "P2", ""),
			code: 3,
			err:  "no dispatchable work: all complete",
		},
		{
			name: "deferred excluded",
			rows: nextTestRow("BT-DEFER", "pending", "P1", `,"deferred":true`) + nextTestRow("BT-READY", "pending", "P2", ""),
			want: "id: BT-READY\npriority: P2\ntitle: title BT-READY\nacceptance_criteria: criteria BT-READY\n",
		},
		{
			name: "unsatisfied dependency skipped",
			rows: nextTestRow("BT-WAIT", "pending", "P1", `,"depends_on":["BT-MISSING"]`) + nextTestRow("BT-READY", "pending", "P2", ""),
			want: "id: BT-READY\npriority: P2\ntitle: title BT-READY\nacceptance_criteria: criteria BT-READY\n",
		},
		{
			name: "string-deferred excluded",
			rows: nextTestRow("BT-STRDEF", "pending", "P0", `,"deferred":"true"`) + nextTestRow("BT-READY", "pending", "P2", ""),
			want: "id: BT-READY\npriority: P2\ntitle: title BT-READY\nacceptance_criteria: criteria BT-READY\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeNextBoard(t, tc.rows)
			var got string
			got, _ = captureStdout(func() {
				code := run([]string{"next", "-C", dir})
				if code != tc.code {
					t.Errorf("exit code = %d, want %d", code, tc.code)
				}
			})
			if tc.want != "" && got != tc.want {
				t.Fatalf("output = %q, want %q", got, tc.want)
			}

		})
	}
}

func TestCmdNextPerformance(t *testing.T) {
	var rows strings.Builder
	for i := 0; i < 10000; i++ {
		rows.WriteString(nextTestRow(fmt.Sprintf("BT-L-%05d", i), "complete", "P5", ""))
	}
	rows.WriteString(nextTestRow("BT-LAST", "pending", "P3", ""))
	dir := writeBoard(t, rows.String())
	start := time.Now()
	out, _ := captureStdout(func() {
		if err := cmdNext(dir, nil); err != nil {
			t.Fatalf("cmdNext: %v", err)
		}
	})
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("cmdNext took %s, want under 1s", elapsed)
	}
	if !strings.Contains(out, "id: BT-LAST\n") {
		t.Fatalf("output does not select pending row: %q", out)
	}
}
