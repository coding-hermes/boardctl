package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/versioncheck"
)

// TestREADMEAcceptanceCriteriaPublishedReleaseContract keeps the published
// release contract aligned with the current release. The source and help tests
// document the flag, and the current v0.4.0 asset must expose it.
func TestREADMEAcceptanceCriteriaPublishedReleaseContract(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := versioncheck.RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, versioncheck.READMEName))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	doc := string(data)

	if !strings.Contains(doc, "Current release: **v0.4.0**") {
		t.Fatal("README release pin changed: expected current release v0.4.0")
	}

	sectionStart := strings.Index(doc, "## Acceptance criteria (DF-BOARDCTL-50)")
	if sectionStart < 0 {
		t.Fatal("README has no DF-BOARDCTL-50 acceptance-criteria section")
	}
	sectionEnd := strings.Index(doc[sectionStart+len("## Acceptance criteria (DF-BOARDCTL-50)"):], "\n## ")
	if sectionEnd < 0 {
		t.Fatal("README acceptance-criteria section has no closing heading")
	}
	section := doc[sectionStart : sectionStart+len("## Acceptance criteria (DF-BOARDCTL-50)")+sectionEnd]

	for _, want := range []string{
		"The published v0.4.0 asset accepts `--acceptance-criteria TEXT`",
		"Published v0.4.0 contract",
		"download the v0.4.0 asset and verify its checksum",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("README acceptance-criteria section missing release-contract wording %q", want)
		}
	}

	// The command examples in the Usage section must carry the published-release
	// qualification as the current source feature.
	usageStart := strings.Index(doc, "## Usage")
	if usageStart < 0 {
		t.Fatal("README has no Usage section")
	}
	usageEnd := strings.Index(doc[usageStart+len("## Usage"):], "\n## ")
	if usageEnd < 0 {
		t.Fatal("README Usage section has no closing heading")
	}
	usage := doc[usageStart : usageStart+len("## Usage")+usageEnd]
	if strings.Count(usage, "published v0.4.0 asset accepts --acceptance-criteria") != 2 {
		t.Fatalf("README Usage acceptance-criteria examples have %d v0.4.0 release qualifiers, want 2", strings.Count(usage, "published v0.4.0 asset accepts --acceptance-criteria"))
	}
	if strings.Count(usage, "--acceptance-criteria") != 6 {
		t.Fatalf("README Usage acceptance-criteria mentions = %d, want 6 (three per qualified example: note, feature description, command)", strings.Count(usage, "--acceptance-criteria"))
	}
}
