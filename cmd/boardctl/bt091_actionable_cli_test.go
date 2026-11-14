package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCmdListActionableExcludesDeferredAndMatchesStats(t *testing.T) {
	dir := seedBT076CLIBoard(t)

	plain, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "list", "--status", "pending", "--json"}); code != 0 {
			t.Fatalf("plain list exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var plainRows []map[string]any
	if err := json.Unmarshal([]byte(plain), &plainRows); err != nil {
		t.Fatalf("plain list JSON: %v\n%s", err, plain)
	}
	if len(plainRows) != 2 {
		t.Fatalf("plain list rows = %d, want 2: %s", len(plainRows), plain)
	}

	actionable, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "list", "--actionable", "--json"}); code != 0 {
			t.Fatalf("actionable list exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var actionableRows []map[string]any
	if err := json.Unmarshal([]byte(actionable), &actionableRows); err != nil {
		t.Fatalf("actionable list JSON: %v\n%s", err, actionable)
	}
	if len(actionableRows) != 1 || actionableRows[0]["id"] != "EXIST-1" {
		t.Fatalf("actionable rows = %v, want only EXIST-1", actionableRows)
	}
	if strings.Contains(actionable, "EXIST-2") {
		t.Fatalf("actionable list included deferred row: %s", actionable)
	}

	statsJSON, err := captureStdout(func() {
		if code := run([]string{"-C", dir, "stats", "--json"}); code != 0 {
			t.Fatalf("stats exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	var stats struct {
		Actionable int `json:"actionable"`
	}
	if err := json.Unmarshal([]byte(statsJSON), &stats); err != nil {
		t.Fatalf("stats JSON: %v\n%s", err, statsJSON)
	}
	if len(actionableRows) != stats.Actionable {
		t.Fatalf("actionable view count = %d, stats.actionable = %d", len(actionableRows), stats.Actionable)
	}
}

func TestCmdListActionableHelpDocumentsFlag(t *testing.T) {
	got, err := captureStdout(func() {
		if code := run([]string{"list", "--help"}); code != 0 {
			t.Fatalf("list --help exit = %d, want 0", code)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "--actionable") {
		t.Fatalf("list help missing --actionable:\n%s", got)
	}
}
