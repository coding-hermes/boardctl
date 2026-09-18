package vulncheck

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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
// id, package, and the Fixed in version — out of the real exit-3 report.
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

// TestCheckWithFakeBinary proves the whole chain, not just the mapping: the
// binary is located through $GOVULNCHECK, its output is consumed, and the
// verdict reaches the caller as an actionable error (or as nil).
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
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	if err := Check(root); err != nil {
		t.Fatalf("dependency-vulnerability gate failed:\n%v", err)
	}
}
