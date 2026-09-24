package workflowcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodWorkflow / goodMakefile are the shape the repo's real files have at the
// fixed tree: identical 7-platform lists, one per line order.
const goodWorkflow = `name: Multi-Arch
on:
  push:
    branches: [main]
    tags: ["v*"]
jobs:
  multiarch:
    # Org standard: coding-hermes/.github/.github/workflows/go-multiarch.yml
    # Pinned 2026-09-24 to 917f3216b505a2decefd8d7878b22d24fbf2a89e (no release tags exist upstream).
    uses: coding-hermes/.github/.github/workflows/go-multiarch.yml@917f3216b505a2decefd8d7878b22d24fbf2a89e
    with:
      binary: boardctl
      platforms: '["linux/amd64","linux/arm64","linux/arm","darwin/amd64","darwin/arm64","windows/amd64","freebsd/amd64"]'
`

const goodMakefile = `BINARY := boardctl
PLATFORMS := \
	linux/amd64 linux/arm64 linux/arm \
	darwin/amd64 darwin/arm64 \
	windows/amd64 \
	freebsd/amd64

release:
	@echo releasing $(PLATFORMS)
	@echo done
`

// writeFixtures materializes a workflow + Makefile pair in a temp dir and
// returns their paths.
func writeFixtures(t *testing.T, workflow, makefile string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wfPath := filepath.Join(wfDir, "multiarch.yml")
	mkPath := filepath.Join(dir, "Makefile")
	if err := os.WriteFile(wfPath, []byte(workflow), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	if err := os.WriteFile(mkPath, []byte(makefile), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	return wfPath, mkPath
}

// TestPlatformDriftDetected is the row's core RED proof, held as a permanent
// fixture test: dropping one platform from the workflow must fail the check
// naming the missing platform — a comment asking for parity is not a check.
// This test never passes trivially: the fixtures are drifted by construction.
func TestPlatformDriftDetected(t *testing.T) {
	driftedWorkflow := strings.Replace(goodWorkflow,
		`"linux/arm",`, `"",`, 1)
	if driftedWorkflow == goodWorkflow {
		t.Fatal("fixture drift did not apply")
	}
	wfPath, mkPath := writeFixtures(t, driftedWorkflow, goodMakefile)
	err := Check(wfPath, mkPath)
	if err == nil {
		t.Fatal("Check() = nil with the workflow missing linux/arm — platform drift is not detected")
	}
	for _, want := range []string{"linux/arm", "freebsd/amd64"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name the drifted platform %q", err, want)
		}
	}
}

// TestPlatformOrderDriftDetected: the same set in a different order must also
// fail — asset publishing order is part of the parity contract.
func TestPlatformOrderDriftDetected(t *testing.T) {
	driftedMakefile := strings.Replace(goodMakefile,
		"windows/amd64 \\\n\tfreebsd/amd64",
		"freebsd/amd64 \\\n\twindows/amd64", 1)
	if driftedMakefile == goodMakefile {
		t.Fatal("fixture drift did not apply")
	}
	wfPath, mkPath := writeFixtures(t, goodWorkflow, driftedMakefile)
	if err := Check(wfPath, mkPath); err == nil {
		t.Fatal("Check() = nil with reordered PLATFORMS — order drift is not detected")
	}
}

// TestDuplicatePlatformFails: a duplicated entry on either side must fail
// even though the sets would compare equal.
func TestDuplicatePlatformFails(t *testing.T) {
	dupWorkflow := strings.Replace(goodWorkflow,
		`"windows/amd64",`, `"windows/amd64","windows/amd64",`, 1)
	wfPath, mkPath := writeFixtures(t, dupWorkflow, goodMakefile)
	err := Check(wfPath, mkPath)
	if err == nil {
		t.Fatal("Check() = nil with a duplicated workflow platform")
	}
	if !strings.Contains(err.Error(), "more than once") {
		t.Errorf("error %q does not name the duplicate", err)
	}
}

// TestConsistentFixturesPass: the undrifted fixture pair passes every gate.
func TestConsistentFixturesPass(t *testing.T) {
	wfPath, mkPath := writeFixtures(t, goodWorkflow, goodMakefile)
	if err := Check(wfPath, mkPath); err != nil {
		t.Fatalf("Check() = %v on consistent fixtures, want nil", err)
	}
}

// TestCheckRefTable pins the ref policy: full 40-hex SHA or vX.Y.Z tag pass;
// branch names, short SHAs and non-semver refs fail.
func TestCheckRefTable(t *testing.T) {
	tests := []struct {
		ref    string
		wantOK bool
	}{
		{"917f3216b505a2decefd8d7878b22d24fbf2a89e", true},
		{"v0.1.8", true},
		{"v1.2.3-rc.1", true},
		{"main", false},
		{"917f321", false},
		{"HEAD", false},
		{"v1.2", false},
		{"", false},
	}
	for _, tc := range tests {
		err := CheckRef(tc.ref)
		if tc.wantOK && err != nil {
			t.Errorf("CheckRef(%q) = %v, want nil", tc.ref, err)
		}
		if !tc.wantOK && err == nil {
			t.Errorf("CheckRef(%q) = nil, want error", tc.ref)
		}
	}
}

// TestReadMakefilePlatformsContinuation: the parser follows make's own
// backslash-continuation rule — a recipe line after the list ends must never
// bleed into the platform list, and an indented `$(PLATFORMS)` reference must
// not be mistaken for the definition.
func TestReadMakefilePlatformsContinuation(t *testing.T) {
	dir := t.TempDir()
	mkPath := filepath.Join(dir, "Makefile")
	content := "release:\n" +
		"\t@echo $(PLATFORMS)\n" +
		"PLATFORMS := \\\n" +
		"\tlinux/amd64 \\\n" +
		"\tlinux/arm64\n" +
		"\n" +
		"clean:\n" +
		"\t@echo windows/amd64 is a recipe line, not a platform\n"
	if err := os.WriteFile(mkPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadMakefilePlatforms(mkPath)
	if err != nil {
		t.Fatalf("ReadMakefilePlatforms: %v", err)
	}
	want := []string{"linux/amd64", "linux/arm64"}
	if len(got) != len(want) {
		t.Fatalf("ReadMakefilePlatforms = %v, want %v (recipe lines must not bleed in)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ReadMakefilePlatforms = %v, want %v", got, want)
		}
	}
}

// TestReadWorkflowPlatformsJSONErrors: a non-JSON or empty platforms input
// fails with a named error instead of silently passing.
func TestReadWorkflowPlatformsJSONErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wfPath := filepath.Join(dir, ".github", "workflows", "multiarch.yml")

	for _, tc := range []struct {
		name    string
		line    string
		wantErr string
	}{
		{"not JSON", `platforms: 'linux/amd64'`, "not a JSON array"},
		{"empty array", `platforms: '[]'`, "empty"},
		{"missing input", "binary: boardctl", "no explicit"},
	} {
		content := "jobs:\n  x:\n    with:\n      " + tc.line + "\n"
		if err := os.WriteFile(wfPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, err := ReadWorkflowPlatforms(wfPath)
		if err == nil {
			t.Errorf("%s: ReadWorkflowPlatforms = nil error, want %q", tc.name, tc.wantErr)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error %q does not contain %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestWorkflowcheck is the in-process gate over the real repo tree: the
// workflow's platforms input and the Makefile PLATFORMS list must be
// identical. Runs under plain `go test ./...` and `-short`; there is
// deliberately no testing.Short() skip — a gate that skips in short mode
// would be green everywhere that matters.
func TestWorkflowcheck(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	wfPath := filepath.Join(root, WorkflowName)
	mkPath := filepath.Join(root, MakefileName)
	if err := Check(wfPath, mkPath); err != nil {
		t.Fatalf("repo tree failed the platform-parity check:\n%v", err)
	}
}

// TestRefPinned is the in-process pin gate over the real workflow: the
// `uses:` ref must be a full 40-hex SHA or a vX.Y.Z tag — never a floating
// branch like @main, which lets an upstream change silently alter what this
// repo publishes — and the pin must be documented with its date in a comment.
func TestRefPinned(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	wfPath := filepath.Join(root, WorkflowName)
	uses, err := ReadWorkflowRef(wfPath)
	if err != nil {
		t.Fatalf("ReadWorkflowRef: %v", err)
	}
	if err := CheckRef(RefName(uses)); err != nil {
		t.Fatalf("workflow %s failed the ref-pin check:\n%v", WorkflowName, err)
	}
	data, err := os.ReadFile(wfPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if !pinCommentRe.Match(data) {
		t.Fatalf("workflow %s has no pin-date comment: the pin must be documented as \"Pinned YYYY-MM-DD ...\" beside the uses line so a reader knows when the SHA was captured and can re-verify it", WorkflowName)
	}
}
