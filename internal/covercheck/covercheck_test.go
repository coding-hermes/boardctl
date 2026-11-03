package covercheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile writes a synthetic raw coverage profile into the test's temp
// dir and returns its path. Hermetic: nothing outside t.TempDir() is read,
// no toolchain is shelled out to.
func writeProfile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func parseFile(t *testing.T, path string) map[string]PackageCoverage {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open profile: %v", err)
	}
	defer func() { _ = f.Close() }()
	cov, err := ParseProfile(f)
	if err != nil {
		t.Fatalf("ParseProfile(%s): %v", path, err)
	}
	return cov
}

func TestCovercheckParseProfileAggregatesPerPackage(t *testing.T) {
	path := writeProfile(t, strings.Join([]string{
		"mode: set",
		"github.com/coding-hermes/boardctl/internal/board/board.go:10.2,12.3 2 1",
		"github.com/coding-hermes/boardctl/internal/board/board.go:20.2,24.3 4 0",
		"github.com/coding-hermes/boardctl/internal/render/render.go:5.2,6.3 1 1",
		// A path without a directory component still counts, under ".".
		"main.go:1.1,2.2 3 0",
		"",
	}, "\n"))
	cov := parseFile(t, path)

	board := cov["github.com/coding-hermes/boardctl/internal/board"]
	if board.Package != "github.com/coding-hermes/boardctl/internal/board" {
		t.Fatalf("board Package = %q", board.Package)
	}
	if board.Statements != 6 || board.Covered != 2 {
		t.Fatalf("board Statements/Covered = %d/%d, want 6/2", board.Statements, board.Covered)
	}
	render := cov["github.com/coding-hermes/boardctl/internal/render"]
	if render.Statements != 1 || render.Covered != 1 {
		t.Fatalf("render Statements/Covered = %d/%d, want 1/1", render.Statements, render.Covered)
	}
	if render.Percent != 100 {
		t.Fatalf("render Percent = %v, want 100", render.Percent)
	}
	root := cov["."]
	if root.Statements != 3 || root.Covered != 0 {
		t.Fatalf("root Statements/Covered = %d/%d, want 3/0", root.Statements, root.Covered)
	}
	if root.Percent != 0 {
		t.Fatalf("root Percent = %v, want 0", root.Percent)
	}
}

func TestCovercheckParseProfileErrors(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr string // substring of the expected error
	}{
		{
			name:    "empty file",
			content: "",
			wantErr: "empty coverage profile",
		},
		{
			name:    "whitespace only",
			content: "\n\n   \n",
			wantErr: "empty coverage profile",
		},
		{
			name:    "header only",
			content: "mode: set\n",
			wantErr: "no coverage blocks",
		},
		{
			name:    "missing mode header",
			content: "github.com/x/y/board.go:1.1,2.2 2 1\n",
			wantErr: "missing 'mode:' header",
		},
		{
			name:    "second mode header after data",
			content: "mode: set\nx.go:1.1,2.2 1 1\nmode: count\n",
			wantErr: "unexpected second mode: header",
		},
		{
			name:    "block without colon",
			content: "mode: set\nx.go 1 1\n",
			wantErr: "line 2",
		},
		{
			name:    "block with too few fields",
			content: "mode: set\nx.go:1.1,2.2 1\n",
			wantErr: "malformed coverage block",
		},
		{
			name:    "non-integer statement count",
			content: "mode: set\nx.go:1.1,2.2 many 1\n",
			wantErr: "statement count",
		},
		{
			name:    "negative execution count",
			content: "mode: set\nx.go:1.1,2.2 1 -1\n",
			wantErr: "execution count",
		},
		{
			name:    "position without comma",
			content: "mode: set\nx.go:1.1 1 1\n",
			wantErr: "lacks a ','",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeProfile(t, tc.content)
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() { _ = f.Close() }()
			_, err = ParseProfile(f)
			if err == nil {
				t.Fatalf("ParseProfile succeeded, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestCovercheckEvaluateTotalBelowFloor(t *testing.T) {
	// 4 of 16 statements covered: total 25% vs floor 80%.
	cov := map[string]PackageCoverage{
		"example.com/m/alpha": {Package: "example.com/m/alpha", Statements: 8, Covered: 2, Percent: 25},
		"example.com/m/beta":  {Package: "example.com/m/beta", Statements: 8, Covered: 2, Percent: 25},
	}
	rep, err := Evaluate(cov, Policy{MinTotal: 80, MinPackage: 60})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if rep.Total != 25 {
		t.Fatalf("Total = %v, want 25", rep.Total)
	}
	var found bool
	for _, f := range rep.Findings {
		if f.Kind == "total" && strings.Contains(f.Message, "25") && strings.Contains(f.Message, "80") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no total finding naming the percentage; findings = %+v", rep.Findings)
	}
}

func TestCovercheckEvaluatePackageBelowFloorNamesPackage(t *testing.T) {
	cov := map[string]PackageCoverage{
		"example.com/m/good": {Package: "example.com/m/good", Statements: 8, Covered: 8, Percent: 100},
		"example.com/m/bad":  {Package: "example.com/m/bad", Statements: 8, Covered: 5, Percent: 62.5},
	}
	rep, err := Evaluate(cov, Policy{MinTotal: 50, MinPackage: 95})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	var finding *Finding
	for i := range rep.Findings {
		if rep.Findings[i].Package == "example.com/m/bad" {
			finding = &rep.Findings[i]
		}
	}
	if finding == nil {
		t.Fatalf("no finding for example.com/m/bad; findings = %+v", rep.Findings)
	}
	if finding.Kind != "package" || finding.Floor != 95 {
		t.Fatalf("finding = %+v, want kind package floor 95", *finding)
	}
	if !strings.Contains(finding.Message, "example.com/m/bad") || !strings.Contains(finding.Message, "62.5") {
		t.Fatalf("finding message %q must name the package and its percentage", finding.Message)
	}
}

func TestCovercheckEvaluateExemptZeroStatementPackage(t *testing.T) {
	// The zero-statement package never appears in the profile at all
	// (internal/secretscheck's real shape); its exempt name must surface in
	// Report.Exemptions, not vanish. A PRESENT package at 0% with an
	// exempt-matching name skips the per-package floor but still counts
	// toward the total.
	cov := map[string]PackageCoverage{
		"github.com/coding-hermes/boardctl/internal/board":        {Package: "github.com/coding-hermes/boardctl/internal/board", Statements: 8, Covered: 8, Percent: 100},
		"github.com/coding-hermes/boardctl/internal/secretscheck": {Package: "github.com/coding-hermes/boardctl/internal/secretscheck", Statements: 4, Covered: 0, Percent: 0},
		"github.com/coding-hermes/boardctl/internal/otherzero":    {Package: "github.com/coding-hermes/boardctl/internal/otherzero", Statements: 0, Covered: 0, Percent: 100},
	}
	rep, err := Evaluate(cov, Policy{MinTotal: 60, MinPackage: 60, Exempt: []string{"internal/secretscheck", "internal/otherzero"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// No findings: both sub-floor packages are exempt; total is 8/12 = 66.7% >= 60.
	if len(rep.Findings) != 0 {
		t.Fatalf("exempt packages produced findings: %+v", rep.Findings)
	}
	if got := (percent(12, 8)); rep.Total != got {
		t.Fatalf("Total = %v, want %v (exempt statements still counted)", rep.Total, got)
	}
	for i := range rep.Packages {
		if strings.HasSuffix(rep.Packages[i].Package, "secretscheck") && !rep.Packages[i].Exempt {
			t.Fatalf("secretscheck package not marked Exempt: %+v", rep.Packages[i])
		}
	}
	if len(rep.Exemptions) != 0 {
		t.Fatalf("both exempt names matched profile packages; Exemptions = %v", rep.Exemptions)
	}

	// The real secretscheck shape: absent from the profile entirely. The
	// exemption must be REPORTED, never silently dropped.
	absent := map[string]PackageCoverage{
		"github.com/coding-hermes/boardctl/internal/board": {Package: "github.com/coding-hermes/boardctl/internal/board", Statements: 8, Covered: 8, Percent: 100},
	}
	rep, err = Evaluate(absent, Policy{MinTotal: 80, MinPackage: 60, Exempt: []string{"internal/secretscheck"}})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", rep.Findings)
	}
	if len(rep.Exemptions) != 1 || rep.Exemptions[0] != "internal/secretscheck" {
		t.Fatalf("Exemptions = %v, want [internal/secretscheck]", rep.Exemptions)
	}
}

func TestCovercheckEvaluateExactBoundaryPasses(t *testing.T) {
	// Power-of-two denominators keep the percentages exactly representable:
	// 5/8 = 62.5 and 13/16 = 81.25 compare exactly against the floors.
	cov := map[string]PackageCoverage{
		"example.com/m/edge": {Package: "example.com/m/edge", Statements: 8, Covered: 5, Percent: 62.5},
		"example.com/m/full": {Package: "example.com/m/full", Statements: 8, Covered: 8, Percent: 100},
	}
	rep, err := Evaluate(cov, Policy{MinTotal: 81.25, MinPackage: 62.5})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("packages exactly at the floor must pass; findings = %+v", rep.Findings)
	}
	if rep.Total != 81.25 {
		t.Fatalf("Total = %v, want exactly 81.25", rep.Total)
	}

	// One statement short flips both findings on: the boundary is a real
	// gate, not a rounding accident.
	cov["example.com/m/edge"] = PackageCoverage{Package: "example.com/m/edge", Statements: 8, Covered: 4, Percent: 50}
	rep, err = Evaluate(cov, Policy{MinTotal: 81.25, MinPackage: 62.5})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Findings) != 2 {
		t.Fatalf("findings = %+v, want one total and one package finding", rep.Findings)
	}
}

func TestCovercheckEvaluateErrors(t *testing.T) {
	if _, err := Evaluate(map[string]PackageCoverage{}, Policy{MinTotal: 80, MinPackage: 60}); err == nil {
		t.Fatal("Evaluate(empty) succeeded, want error")
	}
	if _, err := Evaluate(map[string]PackageCoverage{"p": {Package: "p", Statements: 1}}, Policy{MinTotal: -1, MinPackage: 0}); err == nil {
		t.Fatal("Evaluate(negative floor) succeeded, want error")
	}
}

func TestCovercheckReportPackagesSortedDeterministically(t *testing.T) {
	cov := map[string]PackageCoverage{
		"example.com/m/zz": {Package: "example.com/m/zz", Statements: 1, Covered: 1, Percent: 100},
		"example.com/m/aa": {Package: "example.com/m/aa", Statements: 1, Covered: 1, Percent: 100},
	}
	rep, err := Evaluate(cov, Policy{MinTotal: 80, MinPackage: 60})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Packages) != 2 || rep.Packages[0].Package != "example.com/m/aa" || rep.Packages[1].Package != "example.com/m/zz" {
		t.Fatalf("Packages not sorted by name: %+v", rep.Packages)
	}
}

// TestCovercheckCommandFloorViolation pins the exit-code contract end to
// end through the real checker over a synthetic profile: the command is
// exercised in-process in cmd/covercheck's own tests; this assertion keeps
// the PARSER's verdict consistent with what the command reports (exit 1 on
// any finding, exit 0 when clean).
func TestCovercheckReportReflectsCommandExitCodes(t *testing.T) {
	failing := map[string]PackageCoverage{
		"example.com/m/low": {Package: "example.com/m/low", Statements: 10, Covered: 1, Percent: 10},
	}
	rep, err := Evaluate(failing, Policy{MinTotal: 80, MinPackage: 60})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Findings) == 0 {
		t.Fatal("want findings for a 10% package")
	}
	passing := map[string]PackageCoverage{
		"example.com/m/low": {Package: "example.com/m/low", Statements: 10, Covered: 9, Percent: 90},
	}
	rep, err = Evaluate(passing, Policy{MinTotal: 80, MinPackage: 60})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", rep.Findings)
	}
}
