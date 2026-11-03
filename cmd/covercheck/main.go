// Command covercheck evaluates a Go coverage profile against the repo's
// coverage floors (BT-085). It is the CLI half of the gate: `make
// cover-check` generates the real profile (`go test ./... -count=1 -short
// -coverprofile=<tmp>`) and runs this command over it; CI calls the same
// make target, so the floors live in the Makefile only. The policy engine
// is the pure package internal/covercheck.
//
// Flags:
//
//	--profile <path>   coverage profile to evaluate (required)
//	--min-total N      module-wide statement floor, percent (default 80)
//	--min-package N    per-package statement floor, percent (default 60)
//	--exempt LIST      comma-separated import-path substrings exempt from
//	                   the per-package floor (default internal/secretscheck:
//	                   a zero-statement, test-only package — it emits no
//	                   profile blocks to measure; the exemption is printed,
//	                   never silent)
//
// Exit codes: 0 floors hold, 1 floor violation (findings name the package
// and its percentage), 2 usage/IO/parse error.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/covercheck"
)

// defaultExempt mirrors the Makefile/CI flag: the only zero-statement
// package in the module. Keep the two in agreement — the default exists so
// a bare `go run ./cmd/covercheck --profile=...` still applies the
// documented policy.
const defaultExempt = "internal/secretscheck"

func main() {
	os.Exit(run(os.Args[1:]))
}

// outf prints a summary line to stdout. A gate's stdout is best-effort by
// contract — a closed pipe must never turn a passing gate into a crash —
// so the write error is acknowledged and dropped (errcheck-clean).
func outf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format, args...)
}

func run(args []string) int {
	fs := flag.NewFlagSet("covercheck", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: covercheck --profile <path> [--min-total=80] [--min-package=60] [--exempt=internal/secretscheck]")
		fs.PrintDefaults()
	}
	profile := fs.String("profile", "", "path to the `go test -coverprofile` file (required)")
	minTotal := fs.Float64("min-total", 80, "module-wide statement floor (percent)")
	minPackage := fs.Float64("min-package", 60, "per-package statement floor (percent)")
	exempt := fs.String("exempt", defaultExempt, "comma-separated import-path substrings exempt from the per-package floor")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "covercheck: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}
	if *profile == "" {
		fmt.Fprintln(os.Stderr, "covercheck: --profile is required")
		fs.Usage()
		return 2
	}
	f, err := os.Open(*profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "covercheck: cannot open profile: %v\n", err)
		return 2
	}
	cov, err := covercheck.ParseProfile(f)
	closeErr := f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}
	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "covercheck: closing profile: %v\n", closeErr)
		return 2
	}
	var exemptList []string
	for _, ex := range strings.Split(*exempt, ",") {
		if ex = strings.TrimSpace(ex); ex != "" {
			exemptList = append(exemptList, ex)
		}
	}
	report, err := covercheck.Evaluate(cov, covercheck.Policy{
		MinTotal:   *minTotal,
		MinPackage: *minPackage,
		Exempt:     exemptList,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 2
	}

	lowest := lowestEvaluated(report)
	outf("covercheck: total %.1f%% of statements (floor %.1f%%)\n", report.Total, *minTotal)
	if lowest != nil {
		outf("covercheck: lowest package %s %.1f%% (floor %.1f%%)\n", lowest.Package, lowest.Percent, *minPackage)
	}
	for _, ex := range report.Exemptions {
		outf("covercheck: exempt %s: not present in profile (zero statements, exempt by policy)\n", ex)
	}
	for _, f := range report.Findings {
		outf("covercheck: FAIL %s\n", f.Message)
	}
	if len(report.Findings) > 0 {
		outf("covercheck: FAIL: %d finding(s); raise coverage or amend the floors explicitly\n", len(report.Findings))
		return 1
	}
	outf("covercheck: PASS: %d packages evaluated\n", len(report.Packages))
	return 0
}

// lowestEvaluated returns the lowest-coverage non-exempt package (ties
// broken by name via the sorted Report.Packages), or nil when every
// package is exempt.
func lowestEvaluated(rep covercheck.Report) *covercheck.PackageCoverage {
	var lowest *covercheck.PackageCoverage
	for i := range rep.Packages {
		c := &rep.Packages[i]
		if c.Exempt {
			continue
		}
		if lowest == nil || c.Percent < lowest.Percent {
			lowest = c
		}
	}
	return lowest
}
