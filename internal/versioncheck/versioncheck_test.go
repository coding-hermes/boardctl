package versioncheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The three files that carry the published asset naming + platform set, and the
// released CLI name (the workflow's `binary:` input).
const (
	makefileName = "Makefile"
	workflowName = ".github/workflows/multiarch.yml"
	binaryName   = "boardctl"
)

// goodREADME is the shape the repo's real README has: one pin line, two
// download URLs, all naming the same tag.
const goodREADME = `# sample

Current release: **v1.2.3**.

` + "```bash\n" +
	`curl -sL -o boardctl-linux-amd64 https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/boardctl-linux-amd64
curl -sL -o sha256sums.txt https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/sha256sums.txt` +
	"\n```\n"

func writeREADME(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), READMEName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	return path
}

// TestCheckTable is the table-driven core: every drift shape the checker
// must catch, plus the consistent shape it must pass.
func TestCheckTable(t *testing.T) {
	urlSwap := func(content, from, to string) string {
		return strings.Replace(content, from, to, 1)
	}

	tests := []struct {
		name    string
		content string
		wantErr []string // substrings the error must contain
		wantOK  bool
	}{
		{
			name:    "consistent README passes",
			content: goodREADME,
			wantOK:  true,
		},
		{
			name: "URL with a different tag fails naming BOTH values",
			content: urlSwap(goodREADME,
				"releases/download/v1.2.3/boardctl-linux-amd64",
				"releases/download/v9.8.7/boardctl-linux-amd64"),
			wantErr: []string{"v9.8.7", "v1.2.3"},
		},
		{
			name: "missing Current release pin line fails",
			content: strings.Replace(goodREADME,
				"Current release: **v1.2.3**.\n\n", "", 1),
			wantErr: []string{`no "Current release`},
		},
		{
			name: "zero download URLs fails",
			content: strings.ReplaceAll(
				strings.ReplaceAll(goodREADME,
					"curl -sL -o boardctl-linux-amd64 https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/boardctl-linux-amd64\n", ""),
				"curl -sL -o sha256sums.txt https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/sha256sums.txt", ""),
			wantErr: []string{"no /releases/download"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Check(writeREADME(t, tc.content))
			if tc.wantOK {
				if err != nil {
					t.Fatalf("Check() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Check() = nil, want error")
			}
			for _, substr := range tc.wantErr {
				if !strings.Contains(err.Error(), substr) {
					t.Errorf("error %q does not mention %q", err, substr)
				}
			}
		})
	}
}

// TestCheckNamesFixCommand: the failure must tell the operator what to run.
func TestCheckNamesFixCommand(t *testing.T) {
	drifted := strings.Replace(goodREADME,
		"releases/download/v1.2.3/boardctl-linux-amd64",
		"releases/download/v0.0.9/boardctl-linux-amd64", 1)
	err := Check(writeREADME(t, drifted))
	if err == nil {
		t.Fatal("Check() = nil on drifted README, want error")
	}
	if !strings.Contains(err.Error(), "make version-check") {
		t.Errorf("error %q does not name the fix command", err)
	}
}

// TestReadSurfacesOrder: surfaces are reported in file order so the error
// can point at the offending URL by number.
func TestReadSurfacesOrder(t *testing.T) {
	s, err := ReadSurfaces(writeREADME(t, goodREADME))
	if err != nil {
		t.Fatalf("ReadSurfaces: %v", err)
	}
	if len(s.PinTags) != 1 || s.PinTags[0] != "v1.2.3" {
		t.Fatalf("PinTags = %v, want [v1.2.3]", s.PinTags)
	}
	if len(s.URLTags) != 2 || s.URLTags[0] != "v1.2.3" || s.URLTags[1] != "v1.2.3" {
		t.Fatalf("URLTags = %v, want [v1.2.3 v1.2.3]", s.URLTags)
	}
}

// TestVersioncheck is the in-process README pin gate — the third call site.
// It runs under plain `go test ./...` and under `-short` (CI's Test step uses
// -short): there is deliberately no testing.Short() skip, because a gate that
// skips in short mode would be green everywhere that matters. The command
// `make version-check` and the CI "Version-check" step run this exact test;
// see the package doc for how the three surfaces stay in agreement.
func TestVersioncheck(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	readme := filepath.Join(root, READMEName)
	if _, err := os.Stat(readme); err != nil {
		t.Fatalf("repo README missing at %s: %v", readme, err)
	}
	if err := Check(readme); err != nil {
		t.Fatalf("repo README failed the release-pin check:\n%v", err)
	}
	checkReleaseSurfaces(t, root)
}

// checkReleaseSurfaces pins the ONE published naming + platform set across the
// three surfaces that must agree (BT-036). A tag-push cut and a `make release`
// cut are supposed to be interchangeable, so the install block README documents,
// the names the Makefile writes, and the platform list the CI workflow builds
// cannot drift apart — that drift is what published a release whose asset names
// README's `curl` block could not fetch.
func checkReleaseSurfaces(t *testing.T, root string) {
	t.Helper()

	surfaces, err := ReadSurfaces(filepath.Join(root, READMEName))
	if err != nil {
		t.Fatalf("ReadSurfaces: %v", err)
	}
	if len(surfaces.PinTags) == 0 {
		t.Fatal("no Current release pin line to read the published tag from")
	}
	tag := surfaces.PinTags[0]

	readme := readRepoFile(t, root, READMEName)
	mk := readRepoFile(t, root, makefileName)
	wf := readRepoFile(t, root, workflowName)

	// 1. README downloads exactly the names the release publishes: the
	//    underscore asset form and the SHA256SUMS checksum file.
	for _, want := range []string{
		fmt.Sprintf("/releases/download/%s/%s_linux_amd64", tag, binaryName),
		fmt.Sprintf("/releases/download/%s/SHA256SUMS", tag),
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README install block does not download %q — the documented assets and the published assets must be the same set", want)
		}
	}
	for _, stale := range []string{"sha256sums.txt", binaryName + "-linux-", binaryName + "-darwin-", binaryName + "-windows-", binaryName + "-freebsd-"} {
		if strings.Contains(readme, stale) {
			t.Errorf("README still names the retired asset spelling %q", stale)
		}
	}

	// 2. `make release` writes those same names and the same checksum file.
	if !strings.Contains(mk, "> SHA256SUMS") {
		t.Errorf("Makefile release target does not write SHA256SUMS")
	}
	if !strings.Contains(mk, "$${os}_$${arch}") {
		t.Errorf("Makefile release target does not build <binary>_${os}_${arch}: underscore names need brace-quoted shell variables, because `$os_$arch` parses as the shell variable `os_` and silently drops the arch")
	}

	// 3. The CI cut and the local cut build the SAME platforms in the SAME order.
	want := makefilePlatforms(t, mk)
	got := workflowPlatforms(t, wf)
	if !slices.Equal(got, want) {
		t.Errorf("CI platforms %v do not match the Makefile PLATFORMS %v — a tag-push cut would publish a different platform set than `make release`", got, want)
	}

	// 4. The tag push must actually trigger the workflow: a branches-only push
	//    filter skips tag refs, so the Release job (tag refs only) never runs.
	if !strings.Contains(wf, "tags:") {
		t.Errorf("workflow %s has no `tags:` trigger — pushing a release tag would not run the release job at all", workflowName)
	}
}

// readRepoFile reads one repo-relative file or fails the test.
func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// makefilePlatforms extracts the GOOS/GOARCH list from the Makefile's
// `PLATFORMS := \` continuation lines (tab-indented, backslash-continued).
func makefilePlatforms(t *testing.T, mk string) []string {
	t.Helper()
	lines := strings.Split(mk, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "PLATFORMS :=") {
			start = i + 1
			break
		}
	}
	if start == -1 {
		t.Fatalf("Makefile has no PLATFORMS list")
	}
	var out []string
	for _, l := range lines[start:] {
		if !strings.HasPrefix(l, "	") {
			break
		}
		for _, f := range strings.Fields(l) {
			if f == `\` {
				continue
			}
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatalf("Makefile PLATFORMS list is empty")
	}
	return out
}

// workflowPlatforms parses the workflow's explicit `platforms:` JSON array.
func workflowPlatforms(t *testing.T, wf string) []string {
	t.Helper()
	m := workflowPlatformsRe.FindStringSubmatch(wf)
	if m == nil {
		t.Fatalf("workflow %s has no explicit `platforms: '[…]'` input — it would build the reusable workflow's default platform set instead of the Makefile's", workflowName)
	}
	var out []string
	if err := json.Unmarshal([]byte(m[1]), &out); err != nil {
		t.Fatalf("workflow platforms %s is not a JSON array: %v", m[1], err)
	}
	return out
}

// workflowPlatformsRe matches the single-quoted JSON platforms input.
var workflowPlatformsRe = regexp.MustCompile(`(?m)^\s*platforms: '(\[[^']*\])'`)
