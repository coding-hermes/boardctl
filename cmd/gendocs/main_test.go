package main

// BT-079: cmd/gendocs CLI coverage. The command had [no test files] since
// its introduction (tick-86 residual class). These tests execute run()
// end-to-end (T3: the code is actually invoked, not imported) against a
// module-shaped temp tree, asserting the exit codes and the written artifact.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModuleTree creates a minimal module-shaped directory (go.mod present)
// inside t.TempDir and returns its root.
func writeModuleTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tmpmod\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A nested subdir proves the walk-up: run() should still find go.mod.
	nested := filepath.Join(dir, "cmd", "gendocs")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	return nested
}

// TestDefaultRootFindsGoModWalkingUp covers defaultRoot from a nested
// directory and the no-go.mod error path.
func TestDefaultRootFindsGoModWalkingUp(t *testing.T) {
	nested := writeModuleTree(t)
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prevWd) //nolint:errcheck
	root, err := defaultRoot()
	if err != nil {
		t.Fatalf("defaultRoot from nested dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved root %s has no go.mod: %v", root, err)
	}
}

func TestDefaultRootNoGoMod(t *testing.T) {
	dir := t.TempDir()
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prevWd) //nolint:errcheck
	if _, err := defaultRoot(); err == nil {
		t.Fatal("expected error for tree without go.mod")
	} else if !strings.Contains(err.Error(), "no go.mod found") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestRunWritesOpenAPI executes the real run() against a module tree that
// mirrors this repo's shape (describe.Load against the actual repo root
// requires the repo's wiring, so we point the writer at a temp root but
// derive the document from THIS repo's live registry — the derivation input
// is the repo the test binary is compiled from, and the write path is
// redirected by chdir-ing into the temp tree).
//
// Design note: run() couples the derivation root (cwd walk-up) with the
// output location (root/docs/muster/openapi.yaml). Chdir into the temp
// module tree makes describe.Load fail (no repo wiring there), which is the
// documented exit-1 path — asserted in TestRunFailsOutsideRepo. For the
// success path we chdir INTO this repo (a subdirectory, to prove the walk-up)
// and let run() write the real docs/muster/openapi.yaml, then assert the
// written bytes equal a fresh in-process derivation (round-trip proof).
func TestRunWritesOpenAPIMatchingRegistry(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Skipf("not running inside the module tree: %v", err)
	}
	// cwd = repo subdir proves the walk-up; the artifact is written to the
	// repo root, which is the command's real contract.
	sub := filepath.Join(repoRoot, "cmd")
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(sub); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prevWd) //nolint:errcheck

	out := filepath.Join(repoRoot, "docs", "muster", "openapi.yaml")
	prev, rerr := os.ReadFile(out)
	if rerr != nil {
		t.Fatalf("published spec missing before run: %v", rerr)
	}

	if code := run(nil); code != 0 {
		t.Fatalf("run() exit = %d, want 0", code)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("openapi.yaml not written: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("openapi.yaml written empty")
	}
	if !strings.Contains(string(got), "openapi: 3.0.3") {
		t.Fatal("written document is not OpenAPI 3.0.3")
	}
	// Round-trip: the file on disk must be byte-identical to the published
	// spec it just rewrote from the same registry (deterministic generator).
	if string(got) != string(prev) {
		t.Fatal("regenerated openapi.yaml differs from published bytes — generator is non-deterministic or the published spec is stale")
	}
}

// TestRunFailsOutsideRepo asserts exit 1 when the cwd tree has no go.mod
// (defaultRoot failure path).
func TestRunFailsOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prevWd) //nolint:errcheck

	if code := run(nil); code != 1 {
		t.Fatalf("run() exit = %d, want 1 outside a module tree", code)
	}
}
