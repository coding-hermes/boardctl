// Package depcheck is the repo's build-set dependency-currency checker: a
// report-only scanner around `go list -m -u` (mirrors internal/fmtcheck,
// internal/versioncheck and internal/vendorcheck).
//
// One checker, two gates over one gate test (kept in agreement by
// construction; BT-89):
//
//   - `make dep-check` runs the REPORT-ONLY arm: it prints a
//     "current -> available" line for every build-set module that is behind
//     and ALWAYS exits 0 — a currency report must never block an unrelated
//     commit.
//   - `make dep-check DEP_STRICT=1` is the STRICT arm: it exits non-zero
//     naming every behind build-set module. It is deliberately not wired
//     into CI (BT-89 ships no CI config changes); it exists so a release
//     cut or a quarterly board-row refresh can gate on currency on demand.
//
// The cadence lives in docs/dependency-currency.md: run at every release
// cut, refreshed quarterly via a board row.
//
// SCOPE (load-bearing): the BUILD SET ONLY — the modules in go.mod's
// require block (the non-indirect require lines): what this binary compiles
// and vendors. The checker parses that require block, runs ONE
// `go list -mod=mod -m -u all`, and filters the graph-wide output down to
// the require set. Graph residue (test-only transitive deps of transitive
// deps) is NEVER reported. Policy: bump the build set, never promote graph
// residue.
//
// OFFLINE BEHAVIOUR (load-bearing): the `-u` lookup needs the module proxy.
// When the go list invocation fails (network/proxy error, missing go
// toolchain), the report answers on a distinguishable path —
// "currency unknown: <the proxy error text>" — and never a silent "all
// current": the Unknown field is set, the render says "currency unknown",
// and the strict arm fails on it. A gate that cannot see the proxy cannot
// assert currency, and a quiet pass would be exactly the rot this gate
// exists to prevent. The offline leg is proven in tests with a fake `go`
// executable on PATH (fixture modes behind / current / mixed /
// proxy-error); no test hits the network under -short.
//
// This package is a CHECK, never a fixer: nothing here edits go.mod and
// nothing here runs `go get`/`go mod tidy`. The remedy for a behind module
// is a deliberate bump of the build set by the contributor — never
// promoting graph residue into the build set.
package depcheck

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// TestName is the single gate test every enforcement surface runs.
const TestName = "TestBuildSetCurrency"

// GateCommand is the byte-identical command `make dep-check` runs and this
// package's doc comment names.
const GateCommand = "go test -count=1 -v -run " + TestName + " ./internal/depcheck"

// DepStrictEnv is the environment variable `make dep-check DEP_STRICT=1`
// exports (target-scoped in the Makefile) that turns the gate from the
// report-only arm into the strict arm. Accepted values: 1, true, yes
// (case-insensitive). Anything else — including unset/empty — stays
// report-only.
const DepStrictEnv = "DEP_STRICT"

// DepRootEnv is the environment variable that redirects the gate's module
// root. It exists ONLY so the strict arm's red proof can run against a
// synthetic behind fixture through the real gate command (BT-89
// requirement); production invocations leave it empty and the root is
// RepoRoot.
const DepRootEnv = "DEP_ROOT"

// DepGoBinEnv is the environment variable that overrides the go binary the
// gate shells out to (same pattern as vulncheck's GOVULNCHECK binary
// override). The `go test` driver rewrites its subprocess PATH to put
// GOROOT/bin first, so a PATH-shimmed fake toolchain cannot survive the
// driver; an explicit override can, which is what the offline fixture
// proofs and the synthetic-behind-fixture red proof use (BT-89).
// Production invocations leave it empty: the first `go` on PATH runs.
const DepGoBinEnv = "DEP_GOBIN"

// goModName is the file the build set is parsed from.
const goModName = "go.mod"

// Module is one build-set module's currency reading.
type Module struct {
	// Path is the module path as it appears in go.mod's require block.
	Path string
	// Required is the version go.mod's require block pins.
	Required string
	// Current is the version `go list -m` resolved for this build.
	Current string
	// Available is non-empty when the proxy offers a newer version.
	Available string
}

// Behind reports whether the proxy offers a version newer than the one this
// build resolves.
func (m Module) Behind() bool { return m.Available != "" && m.Available != m.Current }

// String renders the reporting shape: "current -> available" when behind,
// plus a drift note when the require block and the resolved graph disagree
// without an update being offered.
func (m Module) String() string {
	s := m.Path + " " + m.Current
	if m.Behind() {
		s += " -> " + m.Available
	} else if m.Required != m.Current {
		s += fmt.Sprintf(" (go.mod pins %s, go list resolved %s — require/graph drift; re-run `go mod tidy`)", m.Required, m.Current)
	}
	return s
}

// Report is one currency reading of the module root.
type Report struct {
	// Root is the module directory the reading ran against.
	Root string
	// BuildSet is the count of non-indirect require lines in go.mod.
	BuildSet int
	// Modules are the build-set readings, sorted by module path.
	Modules []Module
	// Missing lists build-set modules that produced NO reading in the
	// `go list -m -u all` output. A healthy module graph never leaves one
	// out; missing is reported, never silently read as "all current".
	Missing []string
	// Unknown is non-nil exactly when the `go list -mod=mod -m -u all`
	// invocation failed. Currency is UNKNOWN then — never "all current".
	Unknown error
}

// BehindCount is the number of behind build-set modules in the report.
func (r Report) BehindCount() int {
	n := 0
	for _, m := range r.Modules {
		if m.Behind() {
			n++
		}
	}
	return n
}

// ParseBuildSet parses go.mod bytes and returns the BUILD SET: the
// module -> version map of every non-indirect require line. Only require
// entries count — both the block form (`require ( ... )`) and the one-line
// form (`require path version`); every `// indirect` line and every
// non-require directive (module/go/toolchain) are excluded. Indirectness
// is detected BEFORE the comment strip: in a require block, a line that
// carries a trailing comment is by go.mod's semantics exactly an
// `// indirect` line, so a line with a comment tail is skipped whole
// (stripping first would erase the marker and wrongly count the dep as
// part of the build set). An unterminated require block is an error rather
// than a truncated build set.
func ParseBuildSet(goModData []byte) (map[string]string, error) {
	set := make(map[string]string)
	inBlock := false
	for _, raw := range strings.Split(string(goModData), "\n") {
		if hasCommentTail(raw) {
			// `// indirect` in a require block, or a comment-only line:
			// never a build-set member.
			continue
		}
		line := strings.TrimSpace(stripInlineComment(raw))
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !inBlock {
			if fields[0] != "require" {
				continue
			}
			if len(fields) == 2 && fields[1] == "(" {
				inBlock = true
			} else if len(fields) == 3 {
				addBuildSetLine(set, fields[1], fields[2])
			}
			continue
		}
		if len(fields) == 1 && fields[0] == ")" {
			inBlock = false
			continue
		}
		if len(fields) >= 2 {
			addBuildSetLine(set, fields[0], fields[1])
		}
	}
	if inBlock {
		return nil, fmt.Errorf("depcheck: unterminated require ( block in go.mod")
	}
	return set, nil
}

// hasCommentTail reports whether the line carries a trailing Go comment.
// The board-row scope lives on this predicate: in a require block the only
// lines with comment tails are the `// indirect` ones, so a comment tail
// marks a module OUT of the build set.
func hasCommentTail(line string) bool {
	return strings.Contains(line, "//") && strings.TrimSpace(stripInlineComment(line)) != ""
}

// stripInlineComment cuts a Go comment tail ("// indirect", ...). The cut is
// exactly how `// indirect` deps are excluded from the build set: the
// comment is the marker.
func stripInlineComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

func addBuildSetLine(set map[string]string, path, version string) {
	if path == "" || version == "" {
		return
	}
	set[path] = version
}

// ParseListModelLine parses one line of `go list -m [-u] all` output. The
// recognized shapes are "path version" (current) and
// "path version [latest]" (behind: the bracket field is the proxy-offered
// update). The main module line ("path" only) and any unrecognized line
// return ok=false. Replacement lines ("path version => new version
// [latest]") read Current from the declared version and Available from
// their bracket field, so a replaced build-set member still reports its
// update; this module declares no replacements.
func ParseListModelLine(line string) (Module, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Module{}, false
	}
	m := Module{Path: fields[0], Required: fields[1], Current: fields[1]}
	for _, f := range fields[2:] {
		if strings.HasPrefix(f, "[") {
			// Trim the surrounding brackets (+ no parens participate);
			// e.g. "[v0.153.0]" -> "v0.153.0".
			m.Available = strings.TrimPrefix(strings.TrimSuffix(f, "]"), "[")
			break
		}
	}
	if m.Available == m.Current {
		m.Available = ""
	}
	return m, true
}

// Check runs ONE `go list -mod=mod -m -u all` at root (goBin empty means the
// first `go` on PATH), parses go.mod's require block, and filters the graph
// output down to the build set. The returned error is non-nil only when
// go.mod is missing or malformed; a failed go list invocation is reported as
// Report.Unknown on the distinguishable currency-unknown path instead. This
// function is report-only: it never edits go.mod and never runs go get.
func Check(root, goBin string) (Report, error) {
	if goBin == "" {
		if resolved, err := exec.LookPath("go"); err == nil {
			goBin = resolved
		} else {
			// Deliberately fall through with the plain name: runGoList
			// fails, which is the currency-unknown path — a missing go
			// toolchain must never be read as "all current".
			goBin = "go"
		}
	}
	rep := Report{Root: root}
	data, err := os.ReadFile(filepath.Join(root, goModName))
	if err != nil {
		return Report{}, err
	}
	set, err := ParseBuildSet(data)
	if err != nil {
		return Report{}, err
	}
	rep.BuildSet = len(set)
	out, listErr := runGoList(root, goBin)
	if listErr != nil {
		rep.Unknown = listErr
		return rep, nil
	}
	seen := make(map[string]bool, len(set))
	for _, line := range out {
		m, ok := ParseListModelLine(line)
		if !ok {
			continue
		}
		required, inBuildSet := set[m.Path]
		if !inBuildSet {
			continue // graph residue: never reported (BT-89 scope)
		}
		seen[m.Path] = true
		m.Required = required
		rep.Modules = append(rep.Modules, m)
	}
	for path := range set {
		if !seen[path] {
			rep.Missing = append(rep.Missing, path)
		}
	}
	sort.Slice(rep.Modules, func(i, j int) bool { return rep.Modules[i].Path < rep.Modules[j].Path })
	sort.Strings(rep.Missing)
	return rep, nil
}

// runGoList shells out ONE `go list -mod=mod -m -u all` invocation in root.
// -mod=mod keeps the lookup self-healing for go.sum drift, matching the
// board-row contract. On failure the error carries the tool's stderr — the
// caller renders it as the proxy error text after "currency unknown:".
func runGoList(root, goBin string) ([]string, error) {
	cmd := exec.Command(goBin, "list", "-mod=mod", "-m", "-u", "all")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("`go list -mod=mod -m -u all` (in %s) failed: %v: %s",
			root, err, strings.TrimSpace(stderr.String()))
	}
	var out []string
	for _, line := range strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out, nil
}

// Render renders the report-only arm's report. The key contract: when
// Unknown is set the text says "currency unknown: <the proxy error text>"
// and claims NO module current — never the silent "all current" lie. Exit
// code is the caller's business (report-only arm always 0).
func Render(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dependency-currency (BT-89): build set = go.mod require block (non-indirect), %d module(s), root %s\n", r.BuildSet, r.Root)
	if r.Unknown != nil {
		fmt.Fprintf(&b, "currency unknown: %v\n", r.Unknown)
		return b.String()
	}
	for _, m := range r.Modules {
		if m.Behind() {
			fmt.Fprintf(&b, "  BEHIND  %s\n", m.String())
		} else {
			fmt.Fprintf(&b, "  current %s\n", m.String())
		}
	}
	for _, path := range r.Missing {
		fmt.Fprintf(&b, "  MISSING %s — in go.mod's require block but absent from `go list -mod=mod -m -u all` output\n", path)
	}
	fmt.Fprintf(&b, "%d of %d build-set module(s) behind.\n", r.BehindCount(), r.BuildSet)
	fmt.Fprintf(&b, "policy: bump the build set, never promote graph residue (docs/dependency-currency.md)\n")
	return b.String()
}

// StrictErr is the STRICT arm's verdict: nil only when the build set is
// fully current AND nothing is missing AND the currency lookup succeeded.
// Any behind line, missing module, or failed proxy lookup is a non-nil
// error naming every offender. A strict gate that cannot see the proxy
// must fail, not pass.
func StrictErr(r Report) error {
	if r.Unknown != nil {
		return fmt.Errorf("depcheck: currency unknown: %v — the strict gate cannot pass without a successful `go list -mod=mod -m -u all`; never a silent \"all current\" (re-run when the module proxy is reachable)", r.Unknown)
	}
	var problems []string
	for _, m := range r.Modules {
		if m.Behind() {
			problems = append(problems, "behind: "+m.String())
		}
	}
	for _, path := range r.Missing {
		problems = append(problems, "missing: "+path+" (in go.mod's require block, absent from go list output)")
	}
	if len(problems) == 0 {
		return nil
	}
	fix := "fix: bump the build-set modules (`go get <module> && go mod tidy`) — never promote graph residue into the build set; cadence in docs/dependency-currency.md"
	return fmt.Errorf("depcheck: strict currency gate: %d behind / %d missing build-set module(s):\n  %s\n%s",
		r.BehindCount(), len(r.Missing), strings.Join(problems, "\n  "), fix)
}

// RepoRoot returns the module root by walking up from start to the nearest
// directory containing go.mod. The test calls this with the working directory
// first (go test sets cwd to the package dir, so any in-repo invocation
// resolves) and falls back to the test binary's own source location, so the
// result never depends on where the developer happens to cd. Same pattern
// as internal/fmtcheck.RepoRoot.
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
		if _, err := os.Stat(filepath.Join(dir, goModName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", start)
		}
		dir = parent
	}
}
