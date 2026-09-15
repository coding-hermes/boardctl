// Package fmtcheck is the repo's gofmt gate: a check-only formatter
// conformance scanner shared by three enforcement surfaces.
//
// One checker, three gates (kept in agreement by construction):
//
//   - `make fmt-check` runs: go test -count=1 -run TestGofmt ./internal/fmtcheck
//   - .github/workflows/ci.yml has a step with the byte-identical command
//   - plain `go test ./...` (incl. `-short`) picks this package up like any other
//
// Because the Makefile target and the CI step both invoke THIS code rather
// than a parallel `gofmt -l` find expression, the set of files they inspect
// cannot drift from what this test scans — there is only one definition of
// gofmt-clean in the repo. The scan scope below (cmd/ + internal/, the
// module's entire shipped Go source) is therefore the scope of every gate.
//
// This package is a CHECK, never a formatter: nothing here rewrites files.
// The writing helper is `make fmt` (gofmt -w) for the contributor to run.
package fmtcheck

import (
	"bytes"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// scanRoots are the module-relative directories holding shipped Go source.
// Everything else in the repo (skills/, docs/, bin/, dist/) has no .go files.
var scanRoots = []string{"cmd", "internal"}

// FindUnformatted returns the repo-relative paths of all .go files under the
// scan roots that gofmt would reformat (i.e. where format.Source differs from
// the bytes on disk, the same criterion gofmt -l uses). A file that fails to
// parse is NOT reported here — that is a compile error, and `go build`/`go
// vet` already gate it; duplicating it here would only muddy the message.
//
// Directories named testdata are skipped on purpose: fixture trees may
// legitimately hold intentionally-unformatted Go snippets (parser inputs,
// before/after examples), and punishing a fixture for its formatting would
// make the gate unusable for exactly the tests that need such files.
func FindUnformatted(root string) ([]string, error) {
	var unformatted []string
	for _, sub := range scanRoots {
		dir := filepath.Join(root, sub)
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fixed, err := format.Source(src)
			if err != nil {
				// Unparseable: leave it to build/vet (see doc comment).
				return nil
			}
			if !bytes.Equal(fixed, src) {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				unformatted = append(unformatted, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(unformatted)
	return unformatted, nil
}

// RepoRoot returns the module root by walking up from start to the nearest
// directory containing go.mod. The test calls this with the working directory
// first (go test sets cwd to the package dir, so any in-repo invocation
// resolves) and falls back to the test binary's own source location, so the
// result never depends on where the developer happens to cd.
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
