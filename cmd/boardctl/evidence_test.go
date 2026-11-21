package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceBaselineCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing", nil, "evidence requires"},
		{"no record", []string{"baseline"}, "evidence requires"},
		{"wrong command", []string{"other", "--record"}, "evidence requires"},
		{"bad flag", []string{"--bad"}, "flag provided but not defined"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := cmdEvidence(t.TempDir(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	t.Run("missing board", func(t *testing.T) {
		if err := cmdEvidence(t.TempDir(), []string{"baseline", "--record"}); err == nil {
			t.Fatal("missing board accepted")
		}
	})
	for _, tc := range []struct{ name, rows, want string }{
		{"empty", "", "0 debt entries"},
		{"one", "{\"id\":\"A\",\"status\":\"complete\",\"commit_hash\":\"0000000000000000000000000000000000000000\"}\n", "1 debt entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := seedCLIBoard(t)
			if err := os.WriteFile(filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl"), []byte(tc.rows), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			out, _ := captureStdout(func() { err = cmdEvidence("", []string{"baseline", "--record", "-C", dir}) })
			if err != nil || !strings.Contains(out, tc.want) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}
