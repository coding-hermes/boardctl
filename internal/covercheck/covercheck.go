// Package covercheck is the repo's test-coverage floor gate: a pure,
// hermetic parser and evaluator for Go coverage profiles (BT-085).
//
// It parses the RAW coverage-profile format emitted by
// `go test -coverprofile` directly:
//
//	mode: set
//	github.com/coding-hermes/boardctl/internal/board/board.go:79.79,80.16 1 1
//
// (the `go tool cover -func` rendering is deliberately NOT used; parsing
// the raw block form keeps this package offline, dependency-free, and
// testable against synthetic profiles in t.TempDir()). Blocks are
// aggregated per package — the directory part of the profile path, which
// for this module is the import path — into statement totals, then
// evaluated against a Policy of floors.
//
// One evaluator, three gates (kept in agreement by construction):
//
//   - `make cover-check` runs the real suite into a temp profile and then
//     ./cmd/covercheck over it (the repo shell, never inside the suite —
//     a gate that ran `go test ./...` from within a test would recurse)
//   - .github/workflows/ci.yml has a `Coverage` step that calls the same
//     make target, so the floors cannot drift between local and CI
//   - plain `go test ./...` (incl. -short) exercises this package's own
//     hermetic tests
//
// Zero-statement packages emit no profile blocks and cannot be measured.
// internal/secretscheck is that package today (it is a test-only package
// pinning the gitleaks regex boundary; it ships no code). It is exempt BY
// NAME with a documented reason — never silently ignored: Evaluate
// surfaces every exempt name that matched no package in the profile
// (Report.Exemptions), and cmd/covercheck prints it.
package covercheck

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// PackageCoverage is one package's statement coverage, aggregated from the
// profile blocks whose file paths live under that package's directory.
type PackageCoverage struct {
	// Package is the import path: the directory part of the profile path.
	Package string
	// Statements is the total instrumented statement count.
	Statements int
	// Covered is the number of statements executed by the test run.
	Covered int
	// Percent is 100*Covered/Statements (100 when Statements is 0 — a
	// package with nothing to cover cannot violate a coverage floor).
	Percent float64
	// Exempt is set by Evaluate: the package name matched an entry of
	// Policy.Exempt, so the per-package floor was skipped for it.
	Exempt bool
}

// Policy is the coverage-floor configuration.
type Policy struct {
	// MinTotal is the module-wide statement-coverage floor (percent).
	MinTotal float64
	// MinPackage is the per-package statement-coverage floor (percent),
	// applied to every package whose name does not match Exempt.
	MinPackage float64
	// Exempt lists import-path substrings/suffixes excluded from the
	// per-package floor. Zero-statement packages (no profile blocks to
	// measure) must be exempt by name with a documented reason.
	Exempt []string
}

// Finding is one floor violation. Package is empty for Kind "total".
type Finding struct {
	Kind    string  // "total" or "package"
	Package string  // import path (empty for the total finding)
	Percent float64 // observed, unrounded
	Floor   float64
	Message string
}

// Report is the outcome of evaluating a profile against a Policy.
type Report struct {
	// Total is the module-wide statement percentage, summed over ALL
	// profile packages (exempt or not — exemption waives the per-package
	// floor, not a package's real uncovered statements).
	Total float64
	// Packages holds one entry per profile package, sorted by name.
	Packages []PackageCoverage
	// Findings holds the violations: the total finding first (if any),
	// then package findings in name order. Empty means the floors hold.
	Findings []Finding
	// Exemptions lists Policy.Exempt names that matched NO package in the
	// profile — by construction these are zero-statement packages (a
	// package with statements always emits profile blocks). The gate
	// prints them so an exemption is a documented decision, never a
	// silent skip.
	Exemptions []string
}

// ParseProfile parses a raw Go coverage profile (`go test -coverprofile`
// format: a leading `mode:` line, then one
// `path:startLine.startCol,endLine.endCol numStatements count` block per
// line) and aggregates statement totals per package (the directory part of
// each block's path). The file may live anywhere; nothing outside r is
// read. It errors on an empty or header-only profile, a missing `mode:`
// header, a misplaced `mode:` header, or a malformed block, naming the
// offending line number in every case.
func ParseProfile(r io.Reader) (map[string]PackageCoverage, error) {
	cov := make(map[string]PackageCoverage)
	sc := bufio.NewScanner(r)
	lineNo := 0
	headerSeen := false
	blocks := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			if headerSeen {
				return nil, fmt.Errorf("covercheck: line %d: unexpected second mode: header after profile data: %q", lineNo, line)
			}
			headerSeen = true
			continue
		}
		if !headerSeen {
			return nil, fmt.Errorf("covercheck: line %d: missing 'mode:' header; profiles start with a mode: line, got %q", lineNo, line)
		}
		pkg, stmts, covered, err := parseBlockLine(line)
		if err != nil {
			return nil, fmt.Errorf("covercheck: line %d: %v", lineNo, err)
		}
		c := cov[pkg]
		c.Package = pkg
		c.Statements += stmts
		c.Covered += covered
		cov[pkg] = c
		blocks++
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("covercheck: reading profile: %v", err)
	}
	if !headerSeen {
		return nil, errors.New("covercheck: empty coverage profile: no 'mode:' header and no coverage blocks")
	}
	if blocks == 0 {
		return nil, errors.New("covercheck: header-only coverage profile: no coverage blocks to evaluate")
	}
	for pkg, c := range cov {
		if c.Statements > 0 {
			c.Percent = 100 * float64(c.Covered) / float64(c.Statements)
		} else {
			c.Percent = 100
		}
		cov[pkg] = c
	}
	return cov, nil
}

// parseBlockLine parses one
// `path:startLine.startCol,endLine.endCol numStatements count` block. The
// path may itself contain colons (Windows drive letters), so the field is
// split at the LAST colon. A block with a positive execution count
// contributes its numStatements to Covered; count 0 contributes none.
func parseBlockLine(line string) (pkg string, statements, covered int, err error) {
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return "", 0, 0, fmt.Errorf("malformed coverage block (want 'path:start,end numstatements count'): %q", line)
	}
	loc := fields[0]
	ci := strings.LastIndex(loc, ":")
	if ci < 0 {
		return "", 0, 0, fmt.Errorf("malformed coverage block: no ':' separating path from position: %q", line)
	}
	path, pos := loc[:ci], loc[ci+1:]
	comma := strings.Index(pos, ",")
	if comma < 0 {
		return "", 0, 0, fmt.Errorf("malformed coverage block: position %q lacks a ',' between start and end", pos)
	}
	for _, half := range []string{pos[:comma], pos[comma+1:]} {
		if _, err := strconv.ParseFloat(half, 64); err != nil {
			return "", 0, 0, fmt.Errorf("malformed coverage block: position half %q is not line.col: %v", half, err)
		}
	}
	stmts, err := strconv.Atoi(fields[1])
	if err != nil || stmts < 0 {
		return "", 0, 0, fmt.Errorf("malformed coverage block: statement count %q is not a non-negative integer", fields[1])
	}
	cnt, err := strconv.Atoi(fields[2])
	if err != nil || cnt < 0 {
		return "", 0, 0, fmt.Errorf("malformed coverage block: execution count %q is not a non-negative integer", fields[2])
	}
	pkg = packageOf(path)
	if pkg == "" {
		return "", 0, 0, fmt.Errorf("malformed coverage block: empty package directory in path %q", path)
	}
	if cnt > 0 {
		covered = stmts
	}
	return pkg, stmts, covered, nil
}

// packageOf maps a profile file path to its package import path (the
// directory part). A path with no directory component maps to "." (the
// module root package); a directory-less block is still accounted, never
// dropped.
func packageOf(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i]
	}
	return "."
}

// Evaluate checks a parsed profile against the policy floors and returns a
// Report. It errors on an empty profile (nothing to evaluate) or negative
// floors. A package passes its floor when Percent >= MinPackage (floors
// compare the unrounded percentage, so a package exactly at the floor
// passes); the module total passes when Total >= MinTotal. Exempt packages
// skip only the per-package floor — their statements still count toward
// the total. Every Exempt name that matched no package lands in
// Report.Exemptions so zero-statement exemptions stay visible.
func Evaluate(cov map[string]PackageCoverage, pol Policy) (Report, error) {
	if len(cov) == 0 {
		return Report{}, errors.New("covercheck: no packages in profile: nothing to evaluate")
	}
	if pol.MinTotal < 0 || pol.MinPackage < 0 {
		return Report{}, fmt.Errorf("covercheck: policy floors must be non-negative (min-total %v, min-package %v)", pol.MinTotal, pol.MinPackage)
	}
	names := make([]string, 0, len(cov))
	for name := range cov {
		names = append(names, name)
	}
	sort.Strings(names)

	rep := Report{}
	totalStmts, totalCovered := 0, 0
	for _, name := range names {
		c := cov[name]
		c.Exempt = isExempt(name, pol.Exempt)
		totalStmts += c.Statements
		totalCovered += c.Covered
		if !c.Exempt && c.Percent < pol.MinPackage {
			rep.Findings = append(rep.Findings, Finding{
				Kind:    "package",
				Package: name,
				Percent: c.Percent,
				Floor:   pol.MinPackage,
				Message: fmt.Sprintf("package %s coverage %.1f%% is below floor %.1f%%", name, c.Percent, pol.MinPackage),
			})
		}
		rep.Packages = append(rep.Packages, c)
	}
	rep.Total = percent(totalStmts, totalCovered)
	if rep.Total < pol.MinTotal {
		rep.Findings = append([]Finding{{
			Kind:    "total",
			Percent: rep.Total,
			Floor:   pol.MinTotal,
			Message: fmt.Sprintf("total coverage %.1f%% is below floor %.1f%%", rep.Total, pol.MinTotal),
		}}, rep.Findings...)
	}
	for _, ex := range pol.Exempt {
		matched := false
		for name := range cov {
			if strings.Contains(name, ex) {
				matched = true
				break
			}
		}
		if !matched {
			rep.Exemptions = append(rep.Exemptions, ex)
		}
	}
	sort.Strings(rep.Exemptions)
	return rep, nil
}

// isExempt reports whether name matches any exempt entry. Entries are
// import-path substrings (a suffix match is a substring match), so
// "internal/secretscheck" exempts that package and "secretscheck" would
// exempt it too.
func isExempt(name string, exempt []string) bool {
	for _, ex := range exempt {
		if ex != "" && strings.Contains(name, ex) {
			return true
		}
	}
	return false
}

// percent is 100*covered/total, or 100 when nothing is measurable.
func percent(total, covered int) float64 {
	if total <= 0 {
		return 100
	}
	return 100 * float64(covered) / float64(total)
}
