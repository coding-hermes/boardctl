// Package vendorcheck is the repo's vendored-config gate: a check-only
// scanner that keeps golangci-lint config discovery pinned to the repo root
// (mirrors internal/fmtcheck).
//
// One checker, three gates (kept in agreement by construction):
//
//   - `make vendor-check` runs: go test -count=1 -run TestVendorcheck ./internal/vendorcheck
//   - .github/workflows/ci.yml has a step with the byte-identical command
//   - plain `go test ./...` (incl. `-short`) picks this package up like any other
//
// What it checks: no file named .golangci* may exist anywhere under vendor/.
//
// The reason this needs a gate (BT-087): golangci-lint 2.x discovers the
// nearest config enclosing each analyzed directory, and upstream modules
// vendor their own .golangci.yml (they are upstream build tooling, not part
// of the vendored API). When the GitReins go_lint arm lints a vendor package
// directory (`golangci-lint run --new-from-rev=HEAD~1 vendor/github.com/...`),
// the vendored config won and declared linters this build does not ship, so
// the run aborted with "unknown linters: 'exhaustruct_v5'" — a hard error,
// not a finding, which failed the whole arm for any commit touching
// vendor/*.go. The repo-root .golangci.yml (version: "2") is the fix: the
// root config wins over any nested one. This package is the regression gate
// that keeps a future `go mod vendor` from silently reinstating the configs.
//
// This package is a CHECK, never a remover: nothing here deletes files.
// The remedy is `make vendor-strip` for the contributor to run.
package vendorcheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// vendorDirName is the module-relative directory holding vendored code.
const vendorDirName = "vendor"

// configPrefix is the file-name prefix marking a golangci-lint config
// (.golangci.yml, .golangci.yaml, .golangci.toml, ...). A prefix match on
// the base name is the criterion: every config shape golangci-lint discovers
// starts with .golangci, and no other vendored file does.
const configPrefix = ".golangci"

// FindVendoredConfigs returns the repo-relative paths of every .golangci*
// file under root's vendor/ directory, sorted. Each finding is a config a
// future go_lint run could pick up while linting a vendor package directory,
// so the gate fails naming every one of them.
//
// A missing vendor/ directory is an error (the module vendors its
// dependencies; a vanished vendor/ is a different breakage that `go build`
// already reports).
func FindVendoredConfigs(root string) ([]string, error) {
	var found []string
	dir := filepath.Join(root, vendorDirName)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(d.Name(), configPrefix) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			found = append(found, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(found)
	return found, nil
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
