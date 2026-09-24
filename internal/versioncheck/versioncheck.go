// Package versioncheck is the repo's README release-pin gate: a check-only
// scanner that keeps every release-tag surface in README.md in agreement,
// shared by three enforcement surfaces (mirrors internal/fmtcheck).
//
// One checker, three gates (kept in agreement by construction):
//
//   - `make version-check` runs: go test -count=1 -run TestVersioncheck ./internal/versioncheck
//   - .github/workflows/ci.yml has a step with the byte-identical command
//   - plain `go test ./...` (incl. `-short`) picks this package up like any other
//
// The surfaces checked (all must name the SAME vX.Y.Z tag):
//
//   - the `Current release: **vX.Y.Z**` pin line
//   - every `/releases/download/<tag>/` asset URL in the install block
//
// In-process, TestVersioncheck additionally pins the ONE published asset-naming
// and platform set across the three surfaces that must agree (BT-036): README's
// install block downloads (`<binary>_<goos>_<goarch>`, `SHA256SUMS`), the
// Makefile `release` target that writes them, and the CI workflow's `platforms:`
// input plus its tag trigger. Drift there is what shipped a release whose asset
// names README's `curl` block could not fetch.
//
// This package is a CHECK, never a rewriter: nothing here modifies files.
// It is pure filesystem + string work: no network calls, no git invocation
// (CI checkouts are shallow and may lack tags; the README is always there).
package versioncheck

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// READMEName is the single file every release-tag surface lives in.
const READMEName = "README.md"

var (
	// pinLineRe matches `Current release: **vX.Y.Z**` at the start of a line.
	pinLineRe = regexp.MustCompile(`(?m)^Current release: \*\*(v[0-9]+\.[0-9]+\.[0-9]+)\*\*`)
	// DownloadURLRe matches every release-asset download URL, capturing its
	// tag. Exported so other README-surface tests (e.g. cmd/boardctl's
	// DF-BOARDCTL-12 quickstart pin) extract tags with the exact pattern this
	// package enforces instead of a drift-prone copy.
	DownloadURLRe = regexp.MustCompile(`/releases/download/(v[0-9]+\.[0-9]+\.[0-9]+)/`)
)

// Surfaces is everything the checker extracted from one README.
type Surfaces struct {
	// PinTags holds the tag of every "Current release:" pin line, in file
	// order. Empty when the pin line is missing.
	PinTags []string
	// URLTags holds the tag of every /releases/download/<tag>/ URL, in file
	// order. Empty when there are none.
	URLTags []string
}

// ReadSurfaces extracts the release-tag surfaces from one README file.
func ReadSurfaces(readmePath string) (Surfaces, error) {
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return Surfaces{}, err
	}
	content := string(data)
	s := Surfaces{}
	for _, m := range pinLineRe.FindAllStringSubmatch(content, -1) {
		s.PinTags = append(s.PinTags, m[1])
	}
	for _, m := range DownloadURLRe.FindAllStringSubmatch(content, -1) {
		s.URLTags = append(s.URLTags, m[1])
	}
	return s, nil
}

// Check verifies that every release-tag surface in readmePath names the same
// vX.Y.Z tag, and that the required surfaces exist at all (a pin line, at
// least one download URL). The error names every surface found with its
// value and the fix command.
func Check(readmePath string) error {
	s, err := ReadSurfaces(readmePath)
	if err != nil {
		return err
	}
	var problems []string

	base := ""
	if len(s.PinTags) > 0 {
		base = s.PinTags[0]
	} else {
		problems = append(problems,
			`no "Current release: **vX.Y.Z**" pin line found (required)`)
	}
	if len(s.URLTags) == 0 {
		problems = append(problems,
			"no /releases/download/<tag>/ URL pins found (need at least one so the install block names the release)")
	}

	for i, tag := range s.PinTags[min(1, len(s.PinTags)):] {
		if tag != base {
			problems = append(problems, fmt.Sprintf(
				"pin line #%d says %q but pin line #1 says %q", i+2, tag, base))
		}
	}
	for i, tag := range s.URLTags {
		if base != "" && tag != base {
			problems = append(problems, fmt.Sprintf(
				"download URL #%d pins %q but the release pin line says %q", i+1, tag, base))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("versioncheck: %d problem(s) in %s:\n  %s\nfix: edit every release surface (the Current release line and all /releases/download/ URLs) to the same vX.Y.Z tag, then re-run make version-check",
		len(problems), readmePath, strings.Join(problems, "\n  "))
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
