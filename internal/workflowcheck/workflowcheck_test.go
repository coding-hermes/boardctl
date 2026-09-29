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

// goodCI mirrors the real .github/workflows/ci.yml step block, including the
// BT-033 comment that MENTIONS the install step by name — the tamper test
// below deletes the step while leaving that comment, proving the census is
// not satisfied by a prose mention of the step (a strings.Contains check
// would be).
const goodCI = `name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    name: Build & Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Build
        run: go build ./...

      - name: Gofmt
        run: go test -count=1 -run TestGofmt ./internal/fmtcheck

      - name: Version-check
        run: go test -count=1 -run TestVersioncheck ./internal/versioncheck

      - name: Spec-check
        run: go test -count=1 ./internal/speccheck

      # BT-033: the dependency-vulnerability gate runs the pinned scanner, so
      # the "Install govulncheck v1.7.0" step must precede it (and the Test
      # step below, which also runs TestVulncheck).
      - name: Install govulncheck v1.7.0
        run: go install golang.org/x/vuln/cmd/govulncheck@v1.7.0

      - name: Vulnerability-check
        run: go test -count=1 -run TestVulncheck ./internal/vulncheck

      - name: Test
        run: go test -short -count=1 ./...
`

// writeCIFixture materializes a ci.yml at the canonical path inside a temp
// dir and returns the file's path.
func writeCIFixture(t *testing.T, ci string) string {
	t.Helper()
	dir := t.TempDir()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(wfDir, "ci.yml")
	if err := os.WriteFile(path, []byte(ci), 0o644); err != nil {
		t.Fatalf("write ci.yml: %v", err)
	}
	return path
}

// removeStep deletes one `- name: <name>` step (the name line plus its
// indented body) from a workflow's text, the same edit a tampering
// contributor would make by hand.
func removeStep(ci, name string) string {
	lines := strings.Split(ci, "\n")
	out := make([]string, 0, len(lines))
	skipping := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		switch {
		case trimmed == "- name: "+name:
			skipping = true
		case skipping:
			// The step's body continues while lines stay indented; the next
			// `- name:` or any dedented key ends it (and is kept).
			if strings.HasPrefix(trimmed, "- ") || (trimmed != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "	")) {
				skipping = false
				out = append(out, l)
			}
		default:
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// TestGateStepCensusDeletedStepFails is the row's tamper proof, held as a
// permanent fixture test: deleting any census step from a copy of ci.yml
// must FAIL CheckGateSteps naming the deleted step. This is the exact tamper
// dogfood run 17 proved silent at HEAD 9307bfa (Gofmt, Version-check and
// Spec-check deletions were noticed by nothing). Non-vacuous by
// construction: every arm asserts its tamper actually removed the step, and
// the Install-govulncheck arm leaves behind the BT-033 comment that mentions
// the step by name — a prose mention must not satisfy the census.
func TestGateStepCensusDeletedStepFails(t *testing.T) {
	for _, step := range ciGateSteps {
		t.Run(step.Name, func(t *testing.T) {
			tampered := removeStep(goodCI, step.Name)
			if strings.Contains(tampered, "- name: "+step.Name) {
				t.Fatalf("tamper did not apply: the %q step is still in the fixture copy", step.Name)
			}
			err := CheckGateSteps(writeCIFixture(t, tampered))
			if err == nil {
				t.Fatalf("CheckGateSteps() = nil after deleting the %q step — CI tamper is not noticed", step.Name)
			}
			if !strings.Contains(err.Error(), step.Name) {
				t.Errorf("error %q does not name the deleted step %q", err, step.Name)
			}
			if !strings.Contains(err.Error(), step.Run) {
				t.Errorf("error %q does not name the expected run command %q", err, step.Run)
			}
		})
	}
}

// TestGateStepCensusEditedCommandFails: keeping the step name while editing
// its command must also fail — the step exists but no longer runs what the
// Makefile and the in-process tests run, which is drift just the same.
func TestGateStepCensusEditedCommandFails(t *testing.T) {
	edited := strings.Replace(goodCI,
		"run: go test -count=1 ./internal/speccheck",
		"run: go test -count=1 -run TestSpeccheck ./internal/speccheck", 1)
	if edited == goodCI {
		t.Fatal("tamper did not apply: the Spec-check command is unchanged")
	}
	err := CheckGateSteps(writeCIFixture(t, edited))
	if err == nil {
		t.Fatal("CheckGateSteps() = nil after editing the Spec-check command — command drift is not noticed")
	}
	for _, want := range []string{"Spec-check", "exists but does not run"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// TestGateStepCensusCompleteFixturePasses: the undrifted fixture passes the
// census — and doubles as a coverage guard, since ciGateSteps is ranged over
// by the deletion test: a census entry added without a matching fixture step
// turns this test red instead of silently narrowing the tamper coverage.
func TestGateStepCensusCompleteFixturePasses(t *testing.T) {
	if err := CheckGateSteps(writeCIFixture(t, goodCI)); err != nil {
		t.Fatalf("CheckGateSteps() = %v on the complete fixture, want nil (add the new census step to goodCI):\n%v", err, err)
	}
}

// TestGateStepsWiredInCI is the in-process census over the real repo tree:
// every gate step the checker family relies on must still exist in ci.yml
// with its byte-exact run command. Runs under plain `go test ./...` and
// `-short` (CI's own Test step runs it — the same step the census covers);
// there is deliberately no testing.Short() skip, matching TestWorkflowcheck.
// This is the enforcement the vulncheck package gave itself with
// TestGateWiring, extended to the gates that had none.
func TestGateStepsWiredInCI(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	if err := CheckGateSteps(filepath.Join(root, CIWorkflowName)); err != nil {
		t.Fatalf("repo tree failed the CI gate-step census:\n%v", err)
	}
}
