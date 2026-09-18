// Package vulncheck is the repo's dependency-vulnerability gate: a
// check-only scanner around govulncheck (golang.org/x/vuln) shared by three
// enforcement surfaces (mirrors internal/fmtcheck and internal/versioncheck).
//
// One checker, three gates (kept in agreement by construction):
//
//   - `make vuln-check` runs: go test -count=1 -run TestVulncheck ./internal/vulncheck
//   - .github/workflows/ci.yml installs govulncheck at a pinned version and
//     has a step running the byte-identical command
//   - plain `go test ./...` (incl. `-short`) picks this package up like any other
//
// Unlike internal/fmtcheck (entirely offline, pure filesystem work) this
// package owns ONE live test: TestVulncheck shells out to the real
// govulncheck binary against this module and requires exit 0. That is
// deliberate. A vulnerability gate that only ever reasons about captured
// fixtures cannot notice the day the pinned toolchain regains a reachable
// stdlib CVE — the fixtures keep saying exactly what they said when they were
// captured. The live test SKIPS — loudly, naming the install command — when
// the binary is absent, so a developer without govulncheck can still run the
// suite; CI installs the pinned binary in its own step, so the check does NOT
// skip there. Everything else in this package (Run/Decide/Summarize over a
// caller-supplied binary) stays offline: those tests never make a live call.
//
// This package is a CHECK, never a fixer: nothing here edits go.mod, and the
// remediation for a finding is a deliberate toolchain or dependency bump.
package vulncheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// BinaryEnv names the environment variable that overrides the govulncheck
// binary. It may be an absolute path or a name resolvable on PATH, so CI and
// tests can point the gate at one specific build instead of whatever PATH
// happens to hold.
const BinaryEnv = "GOVULNCHECK"

// BinaryName is the default binary resolved from PATH.
const BinaryName = "govulncheck"

// PinnedToolVersion is the govulncheck version this repo's gates expect. CI
// pins the identical version in its install step (a wiring test in this
// package keeps the two in agreement).
const PinnedToolVersion = "v1.7.0"

// InstallHint is the one-line remediation quoted when the binary is missing.
const InstallHint = "go install golang.org/x/vuln/cmd/govulncheck@" + PinnedToolVersion

// TestName is the single gate test every enforcement surface runs.
const TestName = "TestVulncheck"

// GateCommand is the byte-identical command run by `make vuln-check`, by the
// CI "Vulnerability-check" step, and named in this package's doc comment. It
// is exported so the wiring test can prove all three surfaces still carry it
// — that is what keeps the gate from rotting.
const GateCommand = "go test -count=1 -run " + TestName + " ./internal/vulncheck"

// govulncheck exit codes (documented semantics of the tool):
//
//	0 = no vulnerabilities found
//	3 = one or more vulnerabilities found (reachable or otherwise)
//
// Every other non-zero code is a tool/database failure (usage error, module
// load failure, unreachable vuln DB, ...) and must FAIL LOUDLY: reporting a
// broken scan as a pass is how a gate silently rots.
const (
	// ExitClean means govulncheck found nothing.
	ExitClean = 0
	// ExitVulnerabilitiesFound means govulncheck found vulnerabilities.
	ExitVulnerabilitiesFound = 3
)

// Status is the verdict of one scan.
type Status string

const (
	// StatusPass means exit 0: no vulnerabilities found.
	StatusPass Status = "PASS"
	// StatusFail means exit 3: vulnerabilities were found.
	StatusFail Status = "FAIL"
	// StatusToolError means any other non-zero exit: the scan itself failed
	// (missing binary, load error, unreachable vulnerability DB). Never a pass.
	StatusToolError Status = "TOOL-ERROR"
)

// Result is one govulncheck invocation's outcome.
type Result struct {
	// Code is the process exit code. Only meaningful when the run started at
	// all (Run returned a nil error).
	Code int
	// Output is the combined stdout+stderr of the scan.
	Output string
}

// Finding is one vulnerability extracted from a govulncheck report.
type Finding struct {
	// ID is the GO-YYYY-NNNN vulnerability id (e.g. GO-2026-6089).
	ID string
	// Package is the affected package/import path, best effort. Empty when
	// the report shape did not expose one.
	Package string
	// FixedIn is the version that fixes the report, verbatim (e.g.
	// "net/http@go1.26.6", "N/A" when no fix exists yet). Empty when the
	// report had no "Fixed in:" line.
	FixedIn string
}

var (
	// vulnHeaderRe matches the line that opens one vulnerability block.
	vulnHeaderRe = regexp.MustCompile(`(?m)^Vulnerability #\d+:\s*(GO-[0-9]{4}-[0-9]+)\s*$`)
	// fixedInRe matches the block's fix line.
	fixedInRe = regexp.MustCompile(`(?m)^[ \t]+Fixed in:[ \t]*(\S+)[ \t]*$`)
	// foundInRe matches the block's affected-package line.
	foundInRe = regexp.MustCompile(`(?m)^[ \t]+Found in:[ \t]*(\S+)[ \t]*$`)
)

// LocateBinary resolves the govulncheck binary: $GOVULNCHECK when set and
// non-empty, otherwise "govulncheck" from PATH. The override must itself
// resolve to an executable — a typo'd override is an error, never a silent
// fall-through to a different binary.
func LocateBinary() (string, error) {
	if override := strings.TrimSpace(os.Getenv(BinaryEnv)); override != "" {
		path, err := exec.LookPath(override)
		if err != nil {
			return "", fmt.Errorf("vulncheck: %s=%q does not resolve to an executable: %w",
				BinaryEnv, override, err)
		}
		return path, nil
	}
	path, err := exec.LookPath(BinaryName)
	if err != nil {
		return "", fmt.Errorf("vulncheck: %s not found on PATH: %w\ninstall it with: %s",
			BinaryName, err, InstallHint)
	}
	return path, nil
}

// Run executes `govulncheck ./...` with dir as the working directory (the
// module root) and returns the exit code plus the combined output. A non-zero
// exit code is NOT an error: it is the scan's verdict and is returned in
// Result.Code for Decide. The error return is reserved for "the scan never
// ran" — binary not found, or the process could not be started.
func Run(dir string) (Result, error) {
	bin, err := LocateBinary()
	if err != nil {
		return Result{}, err
	}
	cmd := exec.Command(bin, "./...")
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	res := Result{Output: string(out)}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			// The process did not run to completion at all (start failure).
			return Result{}, fmt.Errorf("vulncheck: running %s ./... in %s: %w", bin, dir, runErr)
		}
		res.Code = exitErr.ExitCode()
	}
	return res, nil
}

// Decide maps a govulncheck exit code (plus its output, for the summary) to a
// verdict. It is a pure function so the mapping can be proven offline over
// captured reports: an exit-3 report FAILS naming its findings, exit 0 PASSES,
// and every other non-zero code is a TOOL-ERROR that names the code so a
// broken scan can never masquerade as a pass.
func Decide(code int, output string) Decision {
	findings := Summarize(output)
	switch code {
	case ExitClean:
		return Decision{
			Status: StatusPass, Code: code,
			Reason: "no vulnerabilities found",
		}
	case ExitVulnerabilitiesFound:
		noun := "vulnerability"
		if len(findings) != 1 {
			noun = "vulnerabilities"
		}
		return Decision{
			Status: StatusFail, Code: code, Findings: findings,
			Reason: fmt.Sprintf("%d reachable %s found (govulncheck exit %d)",
				len(findings), noun, code),
		}
	default:
		return Decision{
			Status: StatusToolError, Code: code,
			Reason: fmt.Sprintf("govulncheck exited with code %d, which is neither %d (clean) nor %d (findings) — treating it as a tool/database error, never a pass",
				code, ExitClean, ExitVulnerabilitiesFound),
		}
	}
}

// Decision is the verdict Decide produced.
type Decision struct {
	// Status is PASS, FAIL (findings) or TOOL-ERROR.
	Status Status
	// Code is the exit code the verdict came from.
	Code int
	// Reason is a one-line, human-readable justification.
	Reason string
	// Findings is the parsed vulnerability list (empty for PASS/TOOL-ERROR).
	Findings []Finding
}

// Summarize extracts the affected vulnerability ids and their "Fixed in"
// versions from a govulncheck report, so a failure is actionable instead of a
// wall of text. It parses the report only — it never runs the tool.
func Summarize(output string) []Finding {
	idx := vulnHeaderRe.FindAllStringSubmatchIndex(output, -1)
	if len(idx) == 0 {
		return nil
	}
	findings := make([]Finding, 0, len(idx))
	for i, m := range idx {
		end := len(output)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		block := output[m[0]:end]
		f := Finding{ID: output[m[2]:m[3]]}
		if fm := fixedInRe.FindStringSubmatch(block); fm != nil {
			f.FixedIn = fm[1]
		}
		if fm := foundInRe.FindStringSubmatch(block); fm != nil {
			f.Package = packageOf(fm[1])
		}
		findings = append(findings, f)
	}
	return findings
}

// packageOf strips the version from a "Found in: pkg@version" value.
func packageOf(foundIn string) string {
	if at := strings.LastIndex(foundIn, "@"); at > 0 {
		return foundIn[:at]
	}
	return foundIn
}

// FormatFindings renders findings as the indented block that a failure
// message carries. It always returns at least one line — an exit-3 report
// whose vulnerabilities could not be parsed says so explicitly rather than
// printing an empty list that reads like "nothing found".
func FormatFindings(findings []Finding) string {
	if len(findings) == 0 {
		return "  (no vulnerability ids could be parsed from the govulncheck output — see the raw output above)"
	}
	lines := make([]string, 0, len(findings))
	for _, f := range findings {
		pkg := f.Package
		if pkg == "" {
			pkg = "unknown package"
		}
		fix := f.FixedIn
		if fix == "" {
			fix = "unknown (the report carried no fixed version)"
		}
		lines = append(lines, fmt.Sprintf("  %s (%s — fixed in %s)", f.ID, pkg, fix))
	}
	return strings.Join(lines, "\n")
}

// Check is the gate: run govulncheck over the module rooted at dir and return
// nil only on a clean scan (exit 0). Any finding or tool failure comes back as
// an error naming the verdict, the affected ids with their fixed versions, and
// the command to re-run.
func Check(dir string) error {
	res, err := Run(dir)
	if err != nil {
		return err
	}
	d := Decide(res.Code, res.Output)
	if d.Status == StatusPass {
		return nil
	}
	if d.Status == StatusToolError {
		return fmt.Errorf("vulncheck: %s — %s\n%s\nre-run: make vuln-check",
			d.Status, d.Reason, indent(res.Output))
	}
	return fmt.Errorf("vulncheck: %s — %s\n%s\nfix: bump the affected package (or the `go` directive in go.mod, for stdlib findings) to the version named above, then re-run make vuln-check",
		d.Status, d.Reason, FormatFindings(d.Findings))
}

// indent prefixes every line of raw tool output so it reads as quoted
// evidence rather than as this package's own message.
func indent(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return "  (no output)"
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

// RepoRoot returns the module root by walking up from start to the nearest
// directory containing go.mod (same pattern as internal/fmtcheck.RepoRoot and
// internal/versioncheck.RepoRoot).
func RepoRoot(start string) (string, error) {
	if root, err := walkUpToGoMod(start); err == nil {
		return root, nil
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		if root, err := walkUpToGoMod(filepath.Dir(file)); err == nil {
			return root, nil
		}
	}
	return "", fmt.Errorf("no go.mod found walking up from %s", start)
}

func walkUpToGoMod(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", start)
		}
		dir = parent
	}
}
