package depcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureGoMod = `module example.com/fixture

go 1.26

require (
	example.com/other v1.0.0
	example.com/two v1.2.3
)

require (
	example.com/residue v0.0.1 // indirect
)
`

// fakeGoScript is the fake `go` executable the offline tests put on PATH.
// It records its argv when GOFAKE_ARGS names a file (the args-pin test),
// then emits a captured `go list -mod=mod -m -u all` output shape selected
// by GOFAKE_MODE — or fails with a proxy-error-shaped stderr.
const fakeGoScript = `#!/bin/sh
# Fake go toolchain for internal/depcheck tests (BT-89).
if [ -n "${GOFAKE_ARGS:-}" ]; then printf '%s\n' "$@" > "$GOFAKE_ARGS"; fi
case "${GOFAKE_MODE:-}" in
  current)
    printf 'example.com/fixture\nexample.com/other v1.0.0\nexample.com/two v1.2.3\n' ;;
  behind)
    printf 'example.com/fixture\nexample.com/other v1.0.0\nexample.com/two v1.2.3 [v1.5.0]\n' ;;
  mixed)
    printf 'example.com/fixture\nexample.com/other v1.0.0\nexample.com/two v1.2.3 [v1.5.0]\nexample.com/residue v0.0.1 [v9.9.9]\n' ;;
  missing)
    printf 'example.com/fixture\nexample.com/other v1.0.0\n' ;;
  proxy-error)
    printf 'go: module example.com/other: proxy: dial tcp: connect: connection refused\n' >&2
    exit 1 ;;
  *)
    printf 'fake go: GOFAKE_MODE=%s is not a known fixture mode\n' "${GOFAKE_MODE:-}" >&2
    exit 99 ;;
esac
`

// fixtureRootWithFakeGo builds a temp module dir holding fixtureGoMod and
// prepends a bin dir holding the fake go executable to PATH.
func fixtureRootWithFakeGo(t *testing.T, mode string) string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fakeGoScript), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	if err := os.WriteFile(argsFile, nil, 0o644); err != nil {
		t.Fatalf("write args file: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, goModName), []byte(fixtureGoMod), 0o644); err != nil {
		t.Fatalf("write fixture go.mod: %v", err)
	}
	t.Setenv("GOFAKE_MODE", mode)
	t.Setenv("GOFAKE_ARGS", argsFile)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// argsFile lives in its own t.TempDir(); no explicit cleanup needed.
	return root
}

// TestParseBuildSet is the offline table over go.mod shapes.
func TestParseBuildSet(t *testing.T) {
	cases := []struct {
		name    string
		goMod   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "block form excludes indirect",
			goMod: fixtureGoMod,
			want:  map[string]string{"example.com/other": "v1.0.0", "example.com/two": "v1.2.3"},
		},
		{
			name:  "this repo's real go.mod shape",
			goMod: "module github.com/coding-hermes/boardctl\n\ngo 1.26.10\n\ntoolchain go1.26.11\n\nrequire (\n\tgithub.com/getkin/kin-openapi v0.152.0\n\tgopkg.in/yaml.v3 v3.0.1\n)\n\nrequire (\n\tgithub.com/go-openapi/jsonpointer v1.0.2 // indirect\n)\n",
			want: map[string]string{
				"github.com/getkin/kin-openapi": "v0.152.0",
				"gopkg.in/yaml.v3":              "v3.0.1",
			},
		},
		{
			name:  "one-line require form counts",
			goMod: "module example.com/fixture\n\nrequire example.com/two v1.2.3\n",
			want:  map[string]string{"example.com/two": "v1.2.3"},
		},
		{
			name:    "unterminated require block is an error",
			goMod:   "module example.com/fixture\n\nrequire (\n\texample.com/two v1.2.3\n",
			wantErr: true,
		},
		{
			name:  "a require line with a comment tail is indirect (marker must survive the strip)",
			goMod: "module example.com/fixture\n\n// a plain comment\n\nrequire (\n	// indirect: a comment-only line inside the block is skipped\n	example.com/two v1.2.3 // deliberate\n)\n",
			want:  map[string]string{},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBuildSet([]byte(tc.goMod))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got set %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBuildSet: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for path, ver := range tc.want {
				if got[path] != ver {
					t.Fatalf("module %s: got %q, want %q", path, got[path], ver)
				}
			}
		})
	}
}

// TestParseListModelLine is the offline table over captured `go list -m -u`
// output shapes.
func TestParseListModelLine(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		wantMod Module
		wantOk  bool
	}{
		{
			name:   "main module line (path only) is not a reading",
			line:   "example.com/fixture",
			wantOk: false,
		},
		{
			name:    "current shape",
			line:    "example.com/two v1.2.3",
			wantMod: Module{Path: "example.com/two", Required: "v1.2.3", Current: "v1.2.3"},
			wantOk:  true,
		},
		{
			name:    "behind shape (bracket form)",
			line:    "example.com/two v1.2.3 [v1.5.0]",
			wantMod: Module{Path: "example.com/two", Required: "v1.2.3", Current: "v1.2.3", Available: "v1.5.0"},
			wantOk:  true,
		},
		{
			name:    "bracket matching current means not behind",
			line:    "example.com/two v1.2.3 [v1.2.3]",
			wantMod: Module{Path: "example.com/two", Required: "v1.2.3", Current: "v1.2.3"},
			wantOk:  true,
		},
		{
			name:    "graph residue shape still parses (filtering is Check's job)",
			line:    "example.com/residue v0.0.1 [v9.9.9]",
			wantMod: Module{Path: "example.com/residue", Required: "v0.0.1", Current: "v0.0.1", Available: "v9.9.9"},
			wantOk:  true,
		},
		{
			name:    "replacement shape reads declared version + bracket",
			line:    "example.com/two v1.2.3 => example.com/two-fork v1.2.4 [v1.6.0]",
			wantMod: Module{Path: "example.com/two", Required: "v1.2.3", Current: "v1.2.3", Available: "v1.6.0"},
			wantOk:  true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseListModelLine(tc.line)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tc.wantOk, got)
			}
			if !tc.wantOk {
				return
			}
			if got != tc.wantMod {
				t.Fatalf("got %+v, want %+v", got, tc.wantMod)
			}
		})
	}
}

// TestCheckFakeGo is the offline table over the four board-row arms plus the
// scope filter, all against the fake go executable on PATH.
func TestCheckFakeGo(t *testing.T) {
	cases := []struct {
		name string
		// mode is the fake go's GOFAKE_MODE.
		mode string
		// assert is called on the report Check returned, and may depend on
		// the recorded argv file created by fixtureRootWithFakeGo.
		assert func(t *testing.T, root, argsFile string, rep Report, err error)
	}{
		{
			name: "current",
			mode: "current",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				if rep.Unknown != nil {
					t.Fatalf("unexpected Unknown: %v", rep.Unknown)
				}
				if rep.BuildSet != 2 {
					t.Fatalf("BuildSet = %d, want 2 (indirect line excluded)", rep.BuildSet)
				}
				if rep.BehindCount() != 0 {
					t.Fatalf("BehindCount = %d, want 0 (%v)", rep.BehindCount(), rep.Modules)
				}
				if len(rep.Missing) != 0 {
					t.Fatalf("Missing = %v, want empty", rep.Missing)
				}
			},
		},
		{
			name: "behind",
			mode: "behind",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				if rep.BehindCount() != 1 {
					t.Fatalf("BehindCount = %d, want 1 (%v)", rep.BehindCount(), rep.Modules)
				}
				m := rep.Modules[1] // sorted: other first, two second
				if m.Path != "example.com/two" || !m.Behind() || m.Available != "v1.5.0" {
					t.Fatalf("unexpected behind reading: %s", m.String())
				}
				out := Render(rep)
				if !strings.Contains(out, "example.com/two v1.2.3 -> v1.5.0") {
					t.Fatalf("render missing current->available line:\n%s", out)
				}
				if !strings.Contains(out, "1 of 2 build-set module(s) behind.") {
					t.Fatalf("render missing behind summary:\n%s", out)
				}
			},
		},
		{
			name: "mixed",
			mode: "mixed",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				if rep.BehindCount() != 1 {
					t.Fatalf("BehindCount = %d, want 1", rep.BehindCount())
				}
				if len(rep.Modules) != 2 {
					t.Fatalf("Modules = %d, want 2 — the build-set filter must drop graph residue", len(rep.Modules))
				}
				// The fake output also names example.com/residue v0.0.1
				// [v9.9.9] (graph-wide residue: an indirect module with an
				// update). It must never reach the report.
				for _, m := range rep.Modules {
					if m.Path == "example.com/residue" {
						t.Fatalf("graph residue (%s) leaked into the report — scope is the BUILD SET only", m.String())
					}
				}
				if !strings.Contains(Render(rep), "current example.com/other v1.0.0") {
					t.Fatalf("render missing current line:\n%s", Render(rep))
				}
			},
		},
		{
			name: "proxy-error",
			mode: "proxy-error",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check must turn a failed go list into Unknown, not a hard error (err=%v, rep=%+v)", err, rep)
				}
				if rep.Unknown == nil {
					t.Fatalf("Unknown = nil — a failed proxy lookup must never read as 'all current' (report: %s)", Render(rep))
				}
				if len(rep.Modules) != 0 {
					t.Fatalf("Modules = %v, want empty on Unknown", rep.Modules)
				}
				text := rep.Unknown.Error()
				if !strings.Contains(text, "proxy: dial tcp: connect: connection refused") {
					t.Fatalf("Unknown must carry the proxy error text, got: %s", text)
				}
				out := Render(rep)
				if !strings.Contains(out, "currency unknown: ") {
					t.Fatalf("render must say 'currency unknown: <error>' on the offline path:\n%s", out)
				}
				// The negative invariant: an unknown render must not claim a
				// behind/current tally — the lookup produced no readings.
				if strings.Contains(out, "of 2 build-set module(s) behind") {
					t.Fatalf("unknown render must not claim a behind/current tally:\n%s", out)
				}
			},
		},
		{
			name: "missing build-set member is reported, never read as current",
			mode: "missing",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				if len(rep.Missing) != 1 || rep.Missing[0] != "example.com/two" {
					t.Fatalf("Missing = %v, want [example.com/two]", rep.Missing)
				}
				if !strings.Contains(Render(rep), "MISSING example.com/two") {
					t.Fatalf("render missing MISSING line:\n%s", Render(rep))
				}
			},
		},
		{
			name: "the checker runs exactly ONE `go list -mod=mod -m -u all`",
			mode: "current",
			assert: func(t *testing.T, _, argsFile string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				data, readErr := os.ReadFile(argsFile)
				if readErr != nil {
					t.Fatalf("read recorded argv: %v", readErr)
				}
				got := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
				want := []string{"list", "-mod=mod", "-m", "-u", "all"}
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Fatalf("fake go recorded argv %q, want %q", got, want)
				}
			},
		},
		{
			name: "strict arm: green on a current fixture, red naming the behind module",
			mode: "mixed",
			assert: func(t *testing.T, _, _ string, rep Report, err error) {
				if err != nil {
					t.Fatalf("Check: %v", err)
				}
				serr := StrictErr(rep)
				if serr == nil {
					t.Fatalf("StrictErr = nil on a behind fixture")
				}
				if !strings.Contains(serr.Error(), "example.com/two v1.2.3 -> v1.5.0") {
					t.Fatalf("strict error must NAME the behind module, got: %s", serr)
				}
				// Graph residue (example.com/residue, indirect) must not be
				// named: the strict gate bumps the build set only.
				if strings.Contains(serr.Error(), "example.com/residue") {
					t.Fatalf("strict error must not name graph residue, got: %s", serr)
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureRootWithFakeGo(t, tc.mode)
			argsFile := os.Getenv("GOFAKE_ARGS")
			rep, err := Check(root, "")
			tc.assert(t, root, argsFile, rep, err)
		})
	}
}

// TestStrictErrGreenAndUnknown pins the two remaining strict-arm verdicts
// that TestCheckFakeGo covers only partially: green on a fully current
// report and non-nil on Unknown. This is the strict arm's RED/GREEN proof
// over the report shape itself.
func TestStrictErrGreenAndUnknown(t *testing.T) {
	green := Report{Root: "/tmp/fixture", BuildSet: 2, Modules: []Module{
		{Path: "example.com/other", Required: "v1.0.0", Current: "v1.0.0"},
		{Path: "example.com/two", Required: "v1.2.3", Current: "v1.2.3"},
	}}
	if err := StrictErr(green); err != nil {
		t.Fatalf("StrictErr on a fully current report = %v, want nil", err)
	}
	unknown := Report{Root: "/tmp/fixture", BuildSet: 2, Unknown: errTest("proxy: dial tcp: connect: connection refused")}
	if err := StrictErr(unknown); err == nil {
		t.Fatalf("StrictErr on Unknown = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "currency unknown") {
		t.Fatalf("strict unknown error must be distinguishable, got: %s", err)
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

// TestDepcheckGateCommand keeps the make surface in agreement with this
// package: the Makefile's dep-check target must run GateCommand verbatim and
// .PHONY must declare the target, so the Makefile and any future CI step
// cannot drift from the checker.
func TestDepcheckGateCommand(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(cwd, "..", "..", "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	mk := string(data)
	if !strings.Contains(mk, GateCommand) {
		t.Fatalf("Makefile's dep-check target does not run GateCommand verbatim (%q) — the Makefile and the checker can now drift", GateCommand)
	}
	for line := range strings.SplitSeq(mk, "\n") {
		if strings.HasPrefix(line, ".PHONY:") {
			if _, rest, cut := strings.Cut(line, ".PHONY:"); cut {
				phony := strings.Fields(rest)
				for _, tgt := range phony {
					if tgt == "dep-check" {
						return // declared: done
					}
				}
				t.Fatalf(".PHONY does not declare dep-check: %s", line)
			}
		}
	}
	t.Fatal("Makefile has no .PHONY line")
}

// TestDocsExist guards the fix hint's pointer: StrictErr names
// docs/dependency-currency.md, so the file must exist.
func TestDocsExist(t *testing.T) {
	for _, docs := range []string{
		filepath.Join("..", "..", "docs", "dependency-currency.md"),
	} {
		if _, err := os.Stat(docs); err != nil {
			t.Fatalf("checker docs missing (%v) — the strict-arm fix hint points at docs/dependency-currency.md", err)
		}
	}
}

// TestBuildSetCurrency is THE gate: `make dep-check` (report arm: always
// exit 0) and `make dep-check DEP_STRICT=1` (strict arm: non-zero naming
// every behind build-set module) both run exactly this test over the real
// module. It shells out to the REAL go toolchain and needs the module
// proxy, so it is SKIPPED under -short: no test may hit the network in
// short mode. DEP_ROOT (set only by the synthetic-fixture red proof) can
// redirect the root; production invocations leave it empty.
//
// The live arm is deliberate, mirroring internal/vulncheck's live
// TestVulncheck: a currency checker tested only against captured fixtures
// cannot notice the day the build set genuinely falls behind the proxy —
// the fixtures keep saying exactly what they said when they were captured.
func TestBuildSetCurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("live currency probe shells out to the real `go list -mod=mod -m -u all` (needs the module proxy); skipped in -short mode")
	}
	strict := strictFromEnv(os.Getenv(DepStrictEnv))
	gobin := os.Getenv(DepGoBinEnv)
	root := os.Getenv(DepRootEnv)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		root, err = RepoRoot(cwd)
		if err != nil {
			t.Fatalf("locate repo root: %v", err)
		}
	}
	rep, err := Check(root, gobin)
	if err != nil {
		t.Fatalf("Check at %s: %v", root, err)
	}
	t.Log("\n" + Render(rep))
	if rep.Unknown != nil && !strict {
		// Report arm: unknown currency is printed above; the report arm
		// exits 0 on every currency reading — never blocks an unrelated
		// commit.
		t.Logf("note: report-only arm keeps exit 0 even with %v", rep.Unknown)
	}
	if strict {
		if err := StrictErr(rep); err != nil {
			t.Errorf("dep-check (DEP_STRICT=1) against %s:\n%v", root, err)
		}
	}
}

// strictFromEnv parses the DEP_STRICT value. Accepted: 1, true, yes
// (case-insensitive); anything else (unset/empty included) is report-only.
func strictFromEnv(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
