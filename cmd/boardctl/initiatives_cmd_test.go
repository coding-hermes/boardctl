package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitiativesCommandCases(t *testing.T) {
	dir := seedCLIBoard(t)
	for _, args := range [][]string{
		{"-C", dir, "create", "--id", "COV-1", "--title", "one", "--initiative", "alpha"},
		{"-C", dir, "create", "--id", "COV-2", "--title", "two", "--initiative", "beta"},
		{"-C", dir, "update", "COV-2", "--status", "blocked"},
	} {
		if code := run(args); code != 0 {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
	cases := []struct {
		name              string
		args              []string
		want, absent, err string
	}{
		{"table", nil, "alpha", "", ""},
		{"filtered", []string{"--status", "blocked"}, "beta", "alpha", ""},
		{"empty", []string{"--status", "review"}, "no initiative-scoped rows", "alpha", ""},
		{"json", []string{"--status", "pending", "--json"}, `"initiative": "alpha"`, `"initiative": "beta"`, ""},
		{"invalid status", []string{"--status", "nonsense"}, "", "", "not in vocabulary"},
		{"positional", []string{"unexpected"}, "", "", "takes no positional"},
		{"invalid flag", []string{"--not-a-flag"}, "", "", "flag provided but not defined"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			out, _ := captureStdout(func() { err = cmdInitiatives(dir, tc.args) })
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error=%v want %s", err, tc.err)
				}
				return
			}
			if err != nil || !strings.Contains(out, tc.want) || (tc.absent != "" && strings.Contains(out, tc.absent)) {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
	t.Run("malformed board", func(t *testing.T) {
		d := seedCLIBoard(t)
		p := filepath.Join(d, ".coding-hermes", "board", "tasks.jsonl")
		if err := os.WriteFile(p, []byte("not json\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cmdInitiatives(d, nil); err == nil {
			t.Fatal("malformed board accepted")
		}
	})
	t.Run("missing board", func(t *testing.T) {
		if err := cmdInitiatives(t.TempDir(), nil); err == nil {
			t.Fatal("missing board accepted")
		}
	})
}
