package vulncheck

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// readFixture loads one captured govulncheck output from testdata/. The
// fixtures are REAL tool output captured on this repo (the exit-3 report is
// the scan that BT-033 fixed; the tool-error fixtures came from a bogus DB
// URL and an unknown flag), so the parser is exercised against the wire
// truth rather than against a guess at the report format.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// TestDecideOffline is the load-bearing matrix: the exit-code -> verdict
// mapping over captured output. No live call happens here.
func TestDecideOffline(t *testing.T) {
	exit3 := readFixture(t, "report-exit3.txt")
	clean := readFixture(t, "report-clean.txt")
	dbErr := readFixture(t, "tool-error-db.txt")
	usageErr := readFixture(t, "tool-error-usage.txt")

	tests := []struct {
		name     string
		code     int
		output   string
		want     Status
		wantIn   []string // substrings the Reason must carry
		wantFind []string // vuln ids that must be summarized
	}{
		{
			name:   "exit 0 clean report",
			code:   ExitClean,
			output: clean,
			want:   StatusPass,
		},
		{
			name:     "exit 3 reachable stdlib findings",
			code:     ExitVulnerabilitiesFound,
			output:   exit3,
			want:     StatusFail,
			wantIn:   []string{"3", "exit 3"},
			wantFind: []string{"GO-2026-6090", "GO-2026-6089", "GO-2026-5972"},
		},
		{
			name:   "exit 1 is a tool error, never a pass",
			code:   1,
			output: dbErr,
			want:   StatusToolError,
			wantIn: []string{"exited with code 1", "tool/database error"},
		},
		{
			name:   "exit 2 is a tool error, never a pass",
			code:   2,
			output: usageErr,
			want:   StatusToolError,
			wantIn: []string{"exited with code 2", "tool/database error"},
		},
		{
			// The anti-rot case: a scan that DIED must not be rescued by a
			// reassuring stdout. Only exit 0 may pass.
			name:   "clean-looking stdout with a broken exit code still FAILS",
			code:   1,
			output: clean,
			want:   StatusToolError,
			wantIn: []string{"exited with code 1"},
		},
		{
			// And the reverse: exit 3 is a findings verdict even when the
			// report shape defeats the parser — silence is not a pass.
			name:   "exit 3 with an unparseable report still FAILS",
			code:   ExitVulnerabilitiesFound,
			output: "Vulnerability #1: something we do not recognize\n",
			want:   StatusFail,
			wantIn: []string{"exit 3"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.code, tc.output)
			if d.Status != tc.want {
				t.Fatalf("Decide(%d).Status = %s, want %s (reason: %s)", tc.code, d.Status, tc.want, d.Reason)
			}
			if d.Code != tc.code {
				t.Errorf("Decide(%d).Code = %d, want %d", tc.code, d.Code, tc.code)
			}
			if d.Reason == "" {
				t.Error("Decide returned an empty Reason — a verdict must justify itself")
			}
			for _, substr := range tc.wantIn {
				if !strings.Contains(d.Reason, substr) {
					t.Errorf("Reason %q does not mention %q", d.Reason, substr)
				}
			}
			got := make([]string, 0, len(d.Findings))
			for _, f := range d.Findings {
				got = append(got, f.ID)
			}
			if len(got) != len(tc.wantFind) {
				t.Fatalf("findings = %v, want ids %v", got, tc.wantFind)
			}
			for i, id := range tc.wantFind {
				if got[i] != id {
					t.Errorf("findings[%d] = %q, want %q", i, got[i], id)
				}
			}
		})
	}
}

// TestSummarizeExit3Fixture asserts the parser pulls the actionable facts —
// id, package, and the Fixed in versions — out of the real exit-3 report.
func TestSummarizeExit3Fixture(t *testing.T) {
	findings := Summarize(readFixture(t, "report-exit3.txt"))
	if len(findings) != 3 {
		t.Fatalf("Summarize = %d findings, want 3: %+v", len(findings), findings)
	}
	want := []Finding{
		{ID: "GO-2026-6090", Package: "crypto/tls", FixedIn: "crypto/tls@go1.26.6"},
		{ID: "GO-2026-6089", Package: "net/http", FixedIn: "net/http@go1.26.6"},
		{ID: "GO-2026-5972", Package: "encoding/asn1", FixedIn: "encoding/asn1@go1.26.6"},
	}
	for i, w := range want {
		if findings[i] != w {
			t.Errorf("findings[%d] = %+v, want %+v", i, findings[i], w)
		}
	}
}

// TestSummarizeNonFindingsOutput: a clean or broken report yields no findings
// (the verdict comes from the exit code, not from the parser).
func TestSummarizeNonFindingsOutput(t *testing.T) {
	for _, name := range []string{"report-clean.txt", "tool-error-db.txt", "tool-error-usage.txt"} {
		if got := Summarize(readFixture(t, name)); len(got) != 0 {
			t.Errorf("Summarize(%s) = %+v, want no findings", name, got)
		}
	}
}

// TestFormatFindings: the failure summary names the ids AND their fixed
// versions, and never renders as an empty list.
func TestFormatFindings(t *testing.T) {
	out := FormatFindings(Summarize(readFixture(t, "report-exit3.txt")))
	for _, substr := range []string{"GO-2026-6089", "net/http@go1.26.6", "GO-2026-5972", "encoding/asn1@go1.26.6"} {
		if !strings.Contains(out, substr) {
			t.Errorf("FormatFindings output %q does not name %q", out, substr)
		}
	}
	if empty := FormatFindings(nil); !strings.Contains(empty, "no vulnerability ids could be parsed") {
		t.Errorf("FormatFindings(nil) = %q, want an explicit 'could not parse' line", empty)
	}
}

// fakeGovulncheck writes an executable POSIX shell stand-in for the real tool
// and returns its path. It lets Check be driven end-to-end offline: locate ->
// run -> decide -> error, with the report text coming from a fixture.
func fakeGovulncheck(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake govulncheck stand-in is a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "govulncheck")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake govulncheck: %v", err)
	}
	return path
}

// heredocScript emits a report verbatim and then exits with code.
func heredocScript(report string, code int) string {
	return "echo FAKE-GOVULNCHECK-RAN >&2\ncat <<'VULN_FIXTURE_EOF'\n" + report + "VULN_FIXTURE_EOF\nexit " + strconv.Itoa(code) + "\n"
}

// captureStderr runs fn with os.Stderr redirected into a pipe and returns
// whatever fn wrote. The warn path of CheckVersion is user-visible surface,
// so the tests assert on the real channel rather than on a seam variable.
// Assertions belong AFTER fn returns — a t.Fatal inside fn would exit the
// test with the pipe still swapped in.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	return string(data)
}

// TestCheckWithFakeBinary proves the whole chain, not just the mapping: the
// binary is located through $GOVULNCHECK, its output is consumed, and the
// verdict reaches the caller as an actionable error (or as nil).
//
// The last four subtests pin the version dimension on top: a binary whose
// -version arm reports a scanner version other than PinnedToolVersion is
// refused before scanning, one reporting the pin proceeds, and one whose
// -version carries no parsable Scanner line warns and proceeds by default
// but refuses the gate before scanning under StrictScannerEnv=1
// (DF-BOARDCTL-23: the silent pass was the hole).
func TestCheckWithFakeBinary(t *testing.T) {
	exit3 := readFixture(t, "report-exit3.txt")

	t.Run("findings become an actionable error", func(t *testing.T) {
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScript(exit3, ExitVulnerabilitiesFound)))
		err := Check(t.TempDir())
		if err == nil {
			t.Fatal("Check() = nil on a fake exit-3 scan, want an error")
		}
		for _, substr := range []string{"FAIL", "GO-2026-6089", "go1.26.6", "make vuln-check"} {
			if !strings.Contains(err.Error(), substr) {
				t.Errorf("error %q does not mention %q", err, substr)
			}
		}
	})

	t.Run("clean scan passes", func(t *testing.T) {
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScript("No vulnerabilities found.\n", ExitClean)))
		if err := Check(t.TempDir()); err != nil {
			t.Fatalf("Check() = %v on a fake clean scan, want nil", err)
		}
	})

	t.Run("broken scan fails loudly with the quoted output", func(t *testing.T) {
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScript(readFixture(t, "tool-error-usage.txt"), 2)))
		err := Check(t.TempDir())
		if err == nil {
			t.Fatal("Check() = nil on a fake exit-2 scan, want an error")
		}
		for _, substr := range []string{"TOOL-ERROR", "exited with code 2", "flag provided but not defined"} {
			if !strings.Contains(err.Error(), substr) {
				t.Errorf("error %q does not mention %q", err, substr)
			}
		}
	})

	// DF-BOARDCTL-20: the version dimension. The binary reports its own
	// scanner version via -version; when it disagrees with PinnedToolVersion
	// the gate FAILS before scanning — a green verdict produced by a
	// different scanner rule set is not this repo's gate.
	t.Run("wrong -version fails naming the mismatch", func(t *testing.T) {
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScriptWithVersion("No vulnerabilities found.\n", ExitClean, "v1.5.2")))
		err := Check(t.TempDir())
		if err == nil {
			t.Fatal("Check() = nil for a binary reporting a scanner version other than the pin, want an error")
		}
		for _, substr := range []string{"version mismatch", PinnedToolVersion, "v1.5.2", InstallHint} {
			if !strings.Contains(err.Error(), substr) {
				t.Errorf("error %q does not mention %q", err, substr)
			}
		}
	})

	t.Run("correct -version proceeds to the scan", func(t *testing.T) {
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScriptWithVersion("No vulnerabilities found.\n", ExitClean, PinnedToolVersion)))
		if err := Check(t.TempDir()); err != nil {
			t.Fatalf("Check() = %v for a binary reporting the pinned version, want nil", err)
		}
	})

	t.Run("missing -version warns and the scan decides", func(t *testing.T) {
		// The plain heredoc fake prints no Scanner line for -version, so the
		// pin cannot be verified: the default contract is WARN on stderr and
		// proceed — the exit code still decides (DF-BOARDCTL-23).
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScript("No vulnerabilities found.\n", ExitClean)))
		var err error
		stderr := captureStderr(t, func() { err = Check(t.TempDir()) })
		if err != nil {
			t.Fatalf("Check() = %v when -version reports no scanner version, want the warn-and-proceed default", err)
		}
		if !strings.Contains(stderr, "WARN") {
			t.Errorf("stderr %q carries no WARN for the unparsable -version output", stderr)
		}
	})

	t.Run("missing -version refuses the gate in strict mode before scanning", func(t *testing.T) {
		// Strict mode turns the unparsable -version into a pre-scan refusal:
		// a stub must not be able to launder a green verdict through the pin.
		t.Setenv(StrictScannerEnv, "1")
		t.Setenv(BinaryEnv, fakeGovulncheck(t, heredocScript("No vulnerabilities found.\n", ExitClean)))
		err := Check(t.TempDir())
		if err == nil {
			t.Fatal("Check() = nil in strict mode when -version reports no scanner version, want a refusal before the scan")
		}
		for _, substr := range []string{"Scanner", InstallHint, StrictScannerEnv} {
			if !strings.Contains(err.Error(), substr) {
				t.Errorf("error %q does not mention %q", err.Error(), substr)
			}
		}
	})
}

// versionArm emits the shell line that answers a `-version` invocation with
// the given scanner version (the real tool's banner line, then exit 0).
func versionArm(version string) string {
	return "case \" $* \" in *' -version '*) echo \"Scanner: govulncheck@" + version + "\"; exit 0 ;; esac\n"
}

// heredocScriptWithVersion is heredocScript for a binary whose -version arm
// reports the given scanner version. The case must PRECEDE the report/exit
// section: the unconditional exit would otherwise end the script before the
// version arm is ever reached.
func heredocScriptWithVersion(report string, code int, version string) string {
	return versionArm(version) + heredocScript(report, code)
}

// TestLocateBinaryOverrideMustResolve: a bad override is an error, never a
// silent fall-through to a different binary.
func TestLocateBinaryOverrideMustResolve(t *testing.T) {
	t.Setenv(BinaryEnv, filepath.Join(t.TempDir(), "does-not-exist"))
	_, err := LocateBinary()
	if err == nil {
		t.Fatal("LocateBinary() = nil error for an unresolvable override, want an error")
	}
	if !strings.Contains(err.Error(), BinaryEnv) {
		t.Errorf("error %q does not name %s", err, BinaryEnv)
	}
}

// TestGateWiring keeps the three enforcement surfaces in agreement. This is
// the "cannot rot again" half of the task: adding a gate that nothing runs,
// or renaming the test out from under the Makefile/CI, fails here.
func TestGateWiring(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	read := func(rel string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(data)
	}

	t.Run("Makefile has the target, the command, and the .PHONY entry", func(t *testing.T) {
		mk := read("Makefile")
		if !strings.Contains(mk, "vuln-check:") {
			t.Errorf("Makefile has no vuln-check target")
		}
		if !strings.Contains(mk, GateCommand) {
			t.Errorf("Makefile does not run the canonical command %q", GateCommand)
		}
		phony := ""
		for _, line := range strings.Split(mk, "\n") {
			if strings.HasPrefix(line, ".PHONY:") {
				phony = line
				break
			}
		}
		if !strings.Contains(phony, "vuln-check") {
			t.Errorf(".PHONY line %q does not list vuln-check", phony)
		}
	})

	t.Run("CI installs the pinned tool and runs the same command", func(t *testing.T) {
		ci := read(filepath.Join(".github", "workflows", "ci.yml"))
		if !strings.Contains(ci, "- name: Vulnerability-check") {
			t.Errorf("ci.yml has no step named \"Vulnerability-check\"")
		}
		if !strings.Contains(ci, GateCommand) {
			t.Errorf("ci.yml does not run the canonical command %q", GateCommand)
		}
		if !strings.Contains(ci, "govulncheck@"+PinnedToolVersion) {
			t.Errorf("ci.yml does not install govulncheck@%s (the version this package pins)", PinnedToolVersion)
		}
	})

	t.Run("README documents the gate", func(t *testing.T) {
		if !strings.Contains(read("README.md"), "make vuln-check") {
			t.Errorf("README.md does not document `make vuln-check`")
		}
	})
}

// TestVulncheck is the repo's live dependency-vulnerability gate — the third
// call site. It runs the REAL govulncheck over this module and requires exit
// 0 (the AFTER state of BT-033: `go 1.26.6` in go.mod, no reachable stdlib
// vulnerabilities).
//
// It runs under plain `go test ./...` and under `-short` (CI's Test step uses
// -short): there is deliberately no testing.Short() skip, because a gate that
// skips in short mode would be green everywhere that matters. `make vuln-check`
// and the CI "Vulnerability-check" step run this exact test; see the package
// doc for how the three surfaces stay in agreement — and for why this package,
// unlike internal/fmtcheck, owns one live test.
//
// When the binary is absent the test SKIPS with a loud message naming the
// install command. CI installs the pinned binary in its own step, so this
// check does NOT skip there.
func TestVulncheck(t *testing.T) {
	if _, err := LocateBinary(); err != nil {
		t.Skipf("SKIP: %v\nThis gate is enforced in CI, which installs the pinned tool (%s) in its own step, so it does not skip there.", err, InstallHint)
	}
	// QA-BOARDCTL-3: under a hard address-space cap (ulimit -v, as the QA
	// chaos-resource cell applies) the govulncheck child cannot start at
	// all — the Go runtime's virtual-memory reservations exceed a 3 GiB
	// address space before any heap limit (GOMEMLIMIT) is honored, so the
	// child dies with `fatal error: runtime: cannot allocate memory` and
	// the gate reads TOOL-ERROR. That is a harness environment, not a code
	// defect: CI runs uncapped and still enforces this gate, so skip loudly
	// instead of reporting a fake tool failure.
	var vlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &vlim); err == nil && vlim.Cur != ^uint64(0) {
		t.Skipf("SKIP: address-space cap active (RLIMIT_AS=%d bytes) — the govulncheck child cannot start under it; the gate stays enforced by CI (uncapped) and `make vuln-check`.", vlim.Cur)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	// DF-BOARDCTL-20: the local binary must at least carry the pinned
	// scanner's minor line before its verdict counts. A binary that
	// self-reports a different minor line refuses the gate here, exactly as
	// the offline fake-binary arm does (patch drift within the line is
	// tolerated: see sameMinorLine).
	if bin, err := LocateBinary(); err == nil {
		if mm, ok := CheckVersion(bin).(*MismatchErr); ok && !sameMinorLine(mm.Actual, PinnedToolVersion) {
			t.Fatalf("dependency-vulnerability gate refused: %v", mm)
		}
	}
	if err := Check(root); err != nil {
		t.Fatalf("dependency-vulnerability gate failed:\n%v", err)
	}
}

// TestCheckVersionOffline covers the probe in isolation: a matching pin
// passes, any other self-reported version is a *MismatchErr naming expected
// and actual, a binary whose exit-0 -version carries no parsable Scanner line
// WARNs and passes by default but is a *UnverifiedBinaryErr under
// StrictScannerEnv=1 (DF-BOARDCTL-23), and a non-zero -version exit is data,
// not an error, when the banner is still printed.
func TestCheckVersionOffline(t *testing.T) {
	t.Run("pinned version reports nil", func(t *testing.T) {
		path := fakeGovulncheck(t, versionArm(PinnedToolVersion))
		if err := CheckVersion(path); err != nil {
			t.Fatalf("CheckVersion() = %v for the pinned version, want nil", err)
		}
	})

	t.Run("wrong version is a mismatch naming expected and actual", func(t *testing.T) {
		path := fakeGovulncheck(t, versionArm("v1.5.2"))
		err := CheckVersion(path)
		mm, ok := err.(*MismatchErr)
		if !ok {
			t.Fatalf("CheckVersion() = %T(%v), want *MismatchErr", err, err)
		}
		if mm.Expected != PinnedToolVersion || mm.Actual != "v1.5.2" {
			t.Errorf("MismatchErr = %+v, want Expected=%s Actual=v1.5.2", mm, PinnedToolVersion)
		}
		for _, substr := range []string{"version mismatch", PinnedToolVersion, "v1.5.2", InstallHint} {
			if !strings.Contains(mm.Error(), substr) {
				t.Errorf("MismatchErr.Error() %q does not mention %q", mm.Error(), substr)
			}
		}
	})

	t.Run("non-zero -version exit with a banner is still probed", func(t *testing.T) {
		// A build that prints the banner and then exits non-zero: the banner
		// is data, so a wrong version is still caught.
		path := fakeGovulncheck(t, versionArm("v1.5.2")+"exit 7\n")
		err := CheckVersion(path)
		mm, ok := err.(*MismatchErr)
		if !ok || mm.Actual != "v1.5.2" {
			t.Fatalf("CheckVersion() = %v, want a v1.5.2 mismatch from the banner", err)
		}
	})

	t.Run("no scanner line warns and passes by default", func(t *testing.T) {
		// DF-BOARDCTL-23: an exit-0 -version with no Scanner line is no
		// longer a silent pass — the warn default keeps the gate usable
		// while naming the hole loudly.
		path := fakeGovulncheck(t, "exit 0\n")
		var err error
		stderr := captureStderr(t, func() { err = CheckVersion(path) })
		if err != nil {
			t.Fatalf("CheckVersion() = %v for exit-0 output with no Scanner line, want nil (warn default)", err)
		}
		if !strings.Contains(stderr, "WARN") {
			t.Errorf("stderr %q carries no WARN — a Scanner-line-less -version must warn by default", stderr)
		}
		for _, substr := range []string{path, StrictScannerEnv} {
			if !strings.Contains(stderr, substr) {
				t.Errorf("stderr warn %q does not mention %q", stderr, substr)
			}
		}
	})

	t.Run("no scanner line fails under StrictScannerEnv=1", func(t *testing.T) {
		t.Setenv(StrictScannerEnv, "1")
		path := fakeGovulncheck(t, "exit 0\n")
		var err error
		stderr := captureStderr(t, func() { err = CheckVersion(path) })
		if err == nil {
			t.Fatal("CheckVersion() = nil in strict mode for output with no Scanner line, want a *UnverifiedBinaryErr")
		}
		if _, ok := err.(*UnverifiedBinaryErr); !ok {
			t.Errorf("CheckVersion() = %T, want *UnverifiedBinaryErr", err)
		}
		for _, substr := range []string{path, InstallHint, StrictScannerEnv} {
			if !strings.Contains(err.Error(), substr) {
				t.Errorf("error %q does not mention %q", err.Error(), substr)
			}
		}
		if strings.Contains(stderr, "WARN") {
			t.Errorf("strict mode still warned on stderr %q — the refusal replaces the warning", stderr)
		}
	})

	t.Run("strict env parsing accepts 1/true/yes and warns otherwise", func(t *testing.T) {
		path := fakeGovulncheck(t, "exit 0\n")
		for _, tc := range []struct {
			val    string
			strict bool
		}{
			{"1", true},
			{"true", true},
			{"YES", true},
			{" 1 ", true}, // operators quote; surrounding whitespace is trimmed
			{"", false},
			{"0", false},
			{"off", false},
		} {
			t.Setenv(StrictScannerEnv, tc.val)
			err := CheckVersion(path)
			if tc.strict && err == nil {
				t.Errorf("StrictScannerEnv=%q: CheckVersion() = nil, want the strict refusal", tc.val)
			}
			if !tc.strict && err != nil {
				t.Errorf("StrictScannerEnv=%q: CheckVersion() = %v, want the warn default", tc.val, err)
			}
		}
	})

	t.Run("strict mode keeps the wrong-version mismatch", func(t *testing.T) {
		// Strict mode governs only the unparsable arm: a parseable wrong
		// version is a *MismatchErr with or without it.
		t.Setenv(StrictScannerEnv, "1")
		path := fakeGovulncheck(t, versionArm("v1.5.2"))
		err := CheckVersion(path)
		mm, ok := err.(*MismatchErr)
		if !ok || mm.Actual != "v1.5.2" {
			t.Fatalf("CheckVersion() = %v in strict mode, want the v1.5.2 *MismatchErr", err)
		}
	})

	t.Run("start failure surfaces as a plain error", func(t *testing.T) {
		if err := CheckVersion(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
			t.Fatal("CheckVersion() = nil for a binary that cannot start, want an error")
		}
	})
}

// TestSameMinorLine pins the leniency boundary: patch drift within the
// pinned minor line is tolerated, a different minor or major is not.
func TestSameMinorLine(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v1.7.0", "v1.7.0", true},
		{"v1.7.0", "v1.7.3", true},  // patch drift within the line
		{"v1.7.0", "v1.8.0", false}, // minor moves the rule set
		{"v1.7.0", "v2.7.0", false}, // major moves the rule set
		{"v1.7.0", "not-semver", false},
		{"not-semver", "not-semver", true}, // only equal to itself
	}
	for _, tc := range tests {
		if got := sameMinorLine(tc.a, tc.b); got != tc.want {
			t.Errorf("sameMinorLine(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
