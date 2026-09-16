package versioncheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
}
