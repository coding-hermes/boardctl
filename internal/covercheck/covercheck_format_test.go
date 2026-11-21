package covercheck

import (
	"fmt"
	"strings"
	"testing"
)

func TestTotalFindingDisplaysBoundaryDifference(t *testing.T) {
	profile := "mode: set\nexample/pkg/file.go:1.1,2.1 799866 1\nexample/pkg/file.go:3.1,4.1 200134 0\n"
	cov, err := ParseProfile(strings.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	report, err := Evaluate(cov, Policy{MinTotal: 80})
	if err != nil || len(report.Findings) != 1 {
		t.Fatalf("Evaluate: findings=%v error=%v", report.Findings, err)
	}
	var measured, floor float64
	if _, err := fmt.Sscanf(report.Findings[0].Message, "total coverage %f%% is below floor %f%%", &measured, &floor); err != nil {
		t.Fatalf("cannot parse finding %q: %v", report.Findings[0].Message, err)
	}
	if measured >= floor || measured != 79.987 || floor != 80 {
		t.Fatalf("finding does not show the gap: %q", report.Findings[0].Message)
	}
}
