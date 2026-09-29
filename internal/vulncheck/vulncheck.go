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
// Local version pin (DF-BOARDCTL-20): the live gate additionally probes the
// resolved binary with `-version` and refuses it when its self-reported
// Scanner version sits on a different major.minor line than
// PinnedToolVersion — a scan by a different scanner rule set must not stand
// in for the pinned gate (offline fakes prove the arm). Patch drift within
// the pinned minor line is tolerated. A binary whose exit-0 -version yields
// no parsable Scanner line cannot prove which scanner rule set produced the
// verdict: by default the gate WARNS on stderr and proceeds (keeping PATH
// fallbacks usable), and under StrictScannerEnv=1 it refuses before
// scanning, so a stub/reimplementation cannot silently defeat the pin
// (DF-BOARDCTL-23).
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
// package keeps the two in agreement), and — since DF-BOARDCTL-20 — the
// local gate refuses a resolved binary that self-reports a scanner version
// on a different major.minor line (see CheckVersion).
const PinnedToolVersion = "v1.7.0"

// InstallHint is the one-line remediation quoted when the binary is missing.
const InstallHint = "go install golang.org/x/vuln/cmd/govulncheck@" + PinnedToolVersion

// StrictScannerEnv names the environment variable that turns the
// no-Scanner-line arm of CheckVersion from a stderr warning into a gate
// refusal. Accepts 1/true/yes (case-insensitive, surrounding whitespace
// ignored); anything else — including unset and empty — keeps the default
// warn-and-proceed behavior. Named so an operator can find it in one grep.
const StrictScannerEnv = "VULNCHECK_STRICT_SCANNER"

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
	foundInRe = regexp.MustCompile(`(?m)^[ 	]+Found in:[ 	]*(\S+)[ 	]*$`)
	// scannerVersionRe matches the "Scanner: govulncheck@vX.Y.Z" line of
	// `<binary> -version` output, capturing the version.
	scannerVersionRe = regexp.MustCompile(`(?m)^Scanner:[ 	]*govulncheck@(\S+)`)
)

// CheckVersion probes `<bin> -version` and compares the self-reported
// Scanner version against PinnedToolVersion. It returns:
//
//   - nil:              the binary reports PinnedToolVersion — the gate proceeds
//   - *MismatchErr:     the binary reports a different version — the gate must
//     FAIL naming expected and actual, because a scan by a different scanner
//     rule set must never masquerade as this repo's gate
//   - *UnverifiedBinaryErr: the binary exists and its -version exited 0 but
//     produced no parsable Scanner line, so the pin could not be verified.
//     Default: WARN on stderr and nil (proceed — the exit-code contract
//     decides); under StrictScannerEnv=1: returned as the gate-refusing
//     error (DF-BOARDCTL-23)
//   - any other error:  the probe never ran (process start failure)
//
// A non-zero -version exit is data (some builds print the banner before
// failing), not an error: the output is probed regardless. The unparsable
// arm is governed by the -version exit status: an exit-0 with no Scanner
// line is the strict-governed warn/fail arm; a non-zero exit with no
// parsable banner keeps the pre-DF-BOARDCTL-23 "absent" semantics (treated
// as absent, caller's PATH-fallback behavior unchanged) — data cannot be
// strict about output that never arrived.
func CheckVersion(bin string) error {
	out, err := exec.Command(bin, "-version").CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("vulncheck: running %s -version: %w", bin, err)
		}
	}
	ver := scannerVersion(string(out))
	if ver == "" {
		if _, wasExit := err.(*exec.ExitError); !wasExit {
			if strictScanner() {
				return &UnverifiedBinaryErr{Bin: bin}
			}
			warnf("the resolved binary %s exited 0 from -version but printed no parsable 'Scanner: govulncheck@...' line — the version pin could not be verified; set %s=1 to fail the gate on this instead", bin, StrictScannerEnv)
		}
		return nil
	}
	if ver != PinnedToolVersion {
		return &MismatchErr{Expected: PinnedToolVersion, Actual: ver, Bin: bin}
	}
	return nil
}

// UnverifiedBinaryErr is the strict-mode verdict of CheckVersion (DF-
// BOARDCTL-23): the binary ran and its -version exited 0 but the output
// carried no parsable Scanner line, so the version pin could not be
// verified. Gated behind StrictScannerEnv; the default behavior is a
// stderr warning.
type UnverifiedBinaryErr struct {
	// Bin is the binary path whose -version output could not be verified.
	Bin string
}

func (e *UnverifiedBinaryErr) Error() string {
	return fmt.Sprintf("govulncheck version could not be verified: the binary %s exited 0 from -version but printed no parsable 'Scanner: govulncheck@...' line, so this repo's %s pin is unproven — a stub or reimplementation defeats the pin (fix: %s; or unset %s to only warn)",
		e.Bin, PinnedToolVersion, InstallHint, StrictScannerEnv)
}

// MismatchErr is the version-mismatch verdict of CheckVersion: the resolved
// binary self-reports a Scanner version other than PinnedToolVersion.
type MismatchErr struct {
	Expected string
	Actual   string
	Bin      string
}

func (e *MismatchErr) Error() string {
	return fmt.Sprintf("govulncheck version mismatch: this repo's gates pin %s but the resolved binary %s reports %s — a scan by a different scanner rule set must not stand in for the pinned gate (fix: %s)",
		e.Expected, e.Bin, e.Actual, InstallHint)
}

// scannerVersion extracts the "Scanner: govulncheck@vX.Y.Z" version from
// govulncheck -version output (empty when the output carries no such line).
func scannerVersion(output string) string {
	if m := scannerVersionRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}
	return ""
}

// sameMinorLine reports whether two vX.Y.Z version strings share their
// major and minor components — the part of the x/vuln version that defines
// the scanner's rule set. Patch drift alone (a locally installed release can
// predate the patch the go.mod-pinned x/vuln module resolves to, or vice
// versa) does not move it. Unparseable versions are only ever equal to
// themselves.
func sameMinorLine(a, b string) bool {
	pa := strings.SplitN(strings.TrimPrefix(a, "v"), ".", 3)
	pb := strings.SplitN(strings.TrimPrefix(b, "v"), ".", 3)
	if len(pa) < 2 || len(pb) < 2 {
		return a == b
	}
	return pa[0] == pb[0] && pa[1] == pb[1]
}

// strictScanner reports whether StrictScannerEnv opts the no-Scanner-line
// arm of CheckVersion into a hard gate refusal. 1/true/yes (case-
// insensitive, whitespace-trimmed) enable; anything else — unset, empty,
// "0", "off" — keeps the default warn-and-proceed behavior. Malformed
// values must NOT silently disable a safety opt-in, so they warn on stderr
// and stay permissive rather than failing closed.
func strictScanner() bool {
	raw := strings.TrimSpace(strings.ToLower(strings.TrimSpace(os.Getenv(StrictScannerEnv))))
	switch raw {
	case "1", "true", "yes":
		return true
	default:
		if raw != "" && raw != "0" && raw != "off" && raw != "false" && raw != "no" {
			warnf("%s=%q is not a recognized value (1/true/yes enable strict mode; 0/false/no/off or unset keep the warn default) — staying permissive", StrictScannerEnv, raw)
		}
		return false
	}
}

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
	return RunBinary(bin, dir)
}

// RunBinary executes `<bin> ./...` with dir as the working directory (the
// module root) and returns the exit code plus the combined output. It is the
// caller-supplied-binary half of Run, so Check can locate and version-probe
// the binary once and scan with exactly what it probed. The error return is
// reserved for "the scan never ran" — a process start failure; a non-zero
// exit code is NOT an error: it is the scan's verdict, returned in
// Result.Code for Decide.
func RunBinary(bin, dir string) (Result, error) {
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
// nil only on a clean scan (exit 0) by a scanner whose version matches the
// pin. Any finding or tool failure comes back as an error naming the verdict,
// the affected ids with their fixed versions, and the command to re-run.
//
// Before scanning, the resolved binary is probed with `-version` (the
// DF-BOARDCTL-20 rule): a binary that self-reports a Scanner version other
// than PinnedToolVersion FAILS the gate naming expected and actual, because a
// green verdict produced by a different scanner rule set must not stand in
// for the pinned gate. When the probe exits 0 but cannot be parsed (no
// Scanner line), the default is a stderr warning and the exit-code contract
// decides alone; StrictScannerEnv=1 refuses before scanning instead
// (DF-BOARDCTL-23). When the probe never ran (binary absent) the check is
// skipped and the exit-code contract below decides alone — the
// PATH-fallback behavior is unchanged.
func Check(dir string) error {
	bin, err := LocateBinary()
	if err != nil {
		return err
	}
	switch vm := CheckVersion(bin).(type) {
	case nil:
	case *UnverifiedBinaryErr:
		if strictScanner() {
			return fmt.Errorf("vulncheck: %w", vm)
		}
	default:
		return fmt.Errorf("vulncheck: %w", vm)
	}
	res, err := RunBinary(bin, dir)
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

// warnf writes one warning line to stderr. It resolves os.Stderr at call
// time rather than capturing it in a package var, so tests can swap the
// channel per invocation and the production default stays untouched.
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vulncheck: WARN: "+format+"\n", args...)
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
