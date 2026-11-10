package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBT090CompletionEvidenceCLIE2E drives a fresh board and scratch Git
// repository through the real binary. Each command is executed directly so
// the assertion observes the process exit code, not a pipeline's last command.
// The sequence proves the verification-only remediation contract end to end:
// unknown commits refuse, debt is WARN-only before a baseline, new debt fails
// after a baseline, --record-debt admits and counts it, and product work passes.
// ch:trace row=BT-090-R1 spec=.coding-hermes/board/tasks.jsonl#BT-090-R1 wave=none:serial-fallback test=cmd/boardctl/bt090_completion_evidence_cli_test.go::TestBT090CompletionEvidenceCLIE2E doc=none:verification-only-remediation evidence=cmd/boardctl/bt090_completion_evidence_cli_test.go::TestBT090CompletionEvidenceCLIE2E witness=none:not-pushed verdict=none:pending-foreman-judge commit=none:worker-commit-not-final memory=none:tick-local STATUS: partial TIER: T2 EVIDENCE: go test ./cmd/boardctl -run '^TestBT090CompletionEvidenceCLIE2E$' -count=1
func TestBT090CompletionEvidenceCLIE2E(t *testing.T) {
	repo := t.TempDir()
	bin := filepath.Join(t.TempDir(), "boardctl")
	root := repoRoot(t)

	build := exec.Command("go", "build", "-o", bin, "./cmd/boardctl")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build boardctl: %v\n%s", err, output)
	}

	var git func(...string) string
	git = func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
		if len(args) > 0 && args[0] == "commit" {
			return git("rev-parse", "HEAD")
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "-q", "-b", "main")
	git("config", "user.name", "BT-090 CLI test")
	git("config", "user.email", "bt-090@example.invalid")

	run := func(args ...string) (int, string, string) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("boardctl %s: %v", strings.Join(args, " "), err)
			}
			code = exitErr.ExitCode()
		}
		return code, stdout.String(), stderr.String()
	}

	if code, out, errOut := run("init", "-C", repo, "--project", "bt090-cli"); code != 0 {
		t.Fatalf("boardctl init exit = %d, want 0 (stdout %q, stderr %q)", code, out, errOut)
	}
	git("add", ".")
	git("commit", "-qm", "initial board")
	if err := os.MkdirAll(filepath.Join(repo, ".gitreins", "history", "synthetic-run"), 0o755); err != nil {
		t.Fatal(err)
	}

	const firstID = "BT-090-CLI-1"
	const secondID = "BT-090-CLI-2"
	const productID = "BT-090-CLI-3"
	create := func(id string) {
		t.Helper()
		if code, out, errOut := run("-C", repo, "create", "--id", id, "--title", "BT-090 CLI target"); code != 0 {
			t.Fatalf("create %s exit = %d (stdout %q, stderr %q)", id, code, out, errOut)
		}
	}
	complete := func(id, hash string, recordDebt bool) (int, string, string) {
		t.Helper()
		args := []string{"-C", repo, "update", id, "--status", "complete", "--commit-hash", hash, "--evidence-run-id", "synthetic-run"}
		if recordDebt {
			args = append(args, "--record-debt")
		}
		return run(args...)
	}
	writeAndCommit := func(name, content, message string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", name)
		return git("commit", "-qm", message)
	}

	create(firstID)
	unknown := strings.Repeat("0", 40)
	if code, _, errOut := complete(firstID, unknown, false); code == 0 || !strings.Contains(errOut, "not a resolvable commit") {
		t.Fatalf("unknown commit: exit = %d, stderr = %q; want nonzero refusal", code, errOut)
	}

	bookkeepingHash := writeAndCommit(".coding-hermes/bookkeeping-1.json", "{\"bookkeeping\":true}\n", "bookkeeping one")
	if code, out, errOut := complete(firstID, bookkeepingHash, false); code != 0 || !strings.Contains(errOut, "WARN completion debt") {
		t.Fatalf("baseline-free bookkeeping completion: exit = %d, stdout = %q, stderr = %q; want exit 0 and WARN", code, out, errOut)
	}

	if code, out, errOut := run("-C", repo, "evidence", "baseline", "--record"); code != 0 || !strings.Contains(out, "1 debt entry") {
		t.Fatalf("record baseline: exit = %d, stdout = %q, stderr = %q; want one counted debt", code, out, errOut)
	}

	create(secondID)
	newDebtHash := writeAndCommit(".coding-hermes/bookkeeping-2.json", "{\"bookkeeping\":true}\n", "bookkeeping two")
	if code, _, errOut := complete(secondID, newDebtHash, false); code == 0 || !strings.Contains(errOut, "completion REFUSED") {
		t.Fatalf("new bookkeeping debt after baseline: exit = %d, stderr = %q; want nonzero refusal", code, errOut)
	}
	if code, out, errOut := complete(secondID, newDebtHash, true); code != 0 || !strings.Contains(errOut, "WARN completion debt") {
		t.Fatalf("--record-debt: exit = %d, stdout = %q, stderr = %q; want exit 0 and WARN", code, out, errOut)
	}
	if code, out, errOut := run("-C", repo, "evidence", "baseline", "--record"); code != 0 || !strings.Contains(out, "2 debt entries") {
		t.Fatalf("count recorded debt: exit = %d, stdout = %q, stderr = %q; want two counted debts", code, out, errOut)
	}

	create(productID)
	productHash := writeAndCommit("product.go", "package product\n", "product change")

	if code, out, errOut := complete(productID, productHash, false); code != 0 || strings.Contains(errOut, "WARN completion debt") {
		t.Fatalf("product completion: exit = %d, stdout = %q, stderr = %q; want accepted without debt warning", code, out, errOut)
	}

}
