package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/versioncheck"
)

// TestREADMEAcceptanceCriteriaPublishedReleaseContract keeps the published
// release contract distinct from the current source contract. The source and
// help tests may continue to document the flag, but README examples must not
// imply that the v0.1.11 asset accepts it.
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

	if !strings.Contains(doc, "Current release: **v0.2.0**") {
		t.Fatal("README release pin changed: expected current release v0.2.0")
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
		"Current HEAD and the next release add `--acceptance-criteria TEXT`",
		"The published v0.1.11 asset does not accept this flag",
		"do not use these examples with v0.1.11",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("README acceptance-criteria section missing release-contract wording %q", want)
		}
	}

	// This was the contradictory claim that caused README-012: it presents
	// the source flag as available without distinguishing the published asset.
	for _, forbidden := range []string{
		"Both verbs now take `--acceptance-criteria TEXT`:",
		"sanctioned write path for the field, so never hand-edit tasks.jsonl for it.",
	} {
		if strings.Contains(doc, forbidden) {
			t.Errorf("README still contains the pre-release contradiction %q", forbidden)
		}
	}

	// The command examples in the Usage section must carry the same explicit
	// source-build/next-release qualification as the current source feature.
	usageStart := strings.Index(doc, "## Usage")
	if usageStart < 0 {
		t.Fatal("README has no Usage section")
	}
	usageEnd := strings.Index(doc[usageStart+len("## Usage"):], "\n## ")
	if usageEnd < 0 {
		t.Fatal("README Usage section has no closing heading")
	}
	usage := doc[usageStart : usageStart+len("## Usage")+usageEnd]
	if strings.Count(usage, "SOURCE BUILD / NEXT RELEASE ONLY") != 2 {
		t.Fatalf("README Usage acceptance-criteria examples have %d release qualifiers, want 2", strings.Count(usage, "SOURCE BUILD / NEXT RELEASE ONLY"))
	}
	if strings.Count(usage, "--acceptance-criteria") != 6 {
		t.Fatalf("README Usage acceptance-criteria mentions = %d, want 6 (three per qualified example: note, feature description, command)", strings.Count(usage, "--acceptance-criteria"))
	}
}
