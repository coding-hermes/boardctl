package vendorcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// listSep is a raw-string newline + indent so gate failures render the
// offending paths as a readable list. Built from a raw literal so the file
// carries no escape sequences to mangle.
const listSep = `
  `

// fixtureCfg is the shape of a vendored upstream golangci config: it pins
// v2, turns every linter on, and (in real upstream files) disables linters
// this build does not ship. Raw literal, real newlines.
const fixtureCfg = `version: "2"
linters:
  default: all
`

// TestVendorcheck is the in-process vendored-config gate — the third call
// site. It runs under plain `go test ./...` and under `-short` (CI's Test
// step uses -short): there is deliberately no testing.Short() skip, because
// a gate that skips in short mode would be green everywhere that matters.
// The command `make vendor-check` and the CI "Vendor-check" step run this
// exact test; see the package doc for how the three surfaces stay in
// agreement.
func TestVendorcheck(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	found, err := FindVendoredConfigs(root)
	if err != nil {
		t.Fatalf("scan vendor/: %v", err)
	}
	if len(found) > 0 {
		t.Errorf("found %d golangci config file(s) under vendor/ — golangci-lint would discover a vendored config instead of the repo-root one and abort go_lint with an unknown-linters error; run make vendor-strip and commit the removal:%s%s",
			len(found), listSep, strings.Join(found, listSep))
	}
}

// TestFindVendoredConfigsFlagsPlantedFixture proves the checker is
// RED-capable on an artificial tree: it must flag every .golangci* shape
// planted anywhere under vendor/ (top level, deep nesting, both config
// suffixes) and must NOT flag the repo-root config this fix itself adds —
// the root config is the remedy, flagging it would make the gate fight its
// own fix.
func TestFindVendoredConfigsFlagsPlantedFixture(t *testing.T) {
	root := t.TempDir()
	plants := []string{
		filepath.Join(root, "vendor", "zz-fixture", ".golangci.yml"),
		filepath.Join(root, "vendor", "github.com", "example", "upstream", ".golangci.yaml"),
	}
	for _, p := range plants {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(fixtureCfg), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	// Non-config dotfiles and the repo-root config must stay invisible.
	noise := []string{
		filepath.Join(root, ".golangci.yml"), // the repo-root pin: NEVER a finding
		filepath.Join(root, "vendor", "zz-fixture", ".editorconfig"),
		filepath.Join(root, "vendor", "zz-fixture", "point.go"),
	}
	for _, p := range noise {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	found, err := FindVendoredConfigs(root)
	if err != nil {
		t.Fatalf("FindVendoredConfigs: %v", err)
	}
	want := []string{
		"vendor/github.com/example/upstream/.golangci.yaml",
		"vendor/zz-fixture/.golangci.yml",
	}
	if len(found) != len(want) {
		t.Fatalf("FindVendoredConfigs() = %v, want exactly %v", found, want)
	}
	for i := range want {
		if found[i] != want[i] {
			t.Errorf("FindVendoredConfigs()[%d] = %q, want %q (list must be sorted)", i, found[i], want[i])
		}
	}
}

// TestFindVendoredConfigsCleanVendor: a vendor/ tree with no golangci
// configs passes with zero findings — the gate must be green on the healthy
// shape, not merely red on the broken one.
func TestFindVendoredConfigsCleanVendor(t *testing.T) {
	root := t.TempDir()
	clean := filepath.Join(root, "vendor", "example.com", "mod", "sub.go.txt")
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(clean, []byte("package mod"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	found, err := FindVendoredConfigs(root)
	if err != nil {
		t.Fatalf("FindVendoredConfigs: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("FindVendoredConfigs() = %v on a clean vendor/, want none", found)
	}
}

// TestFindVendoredConfigsMissingVendorIsError: the module vendors its
// dependencies, so a vanished vendor/ directory is reported as an error
// rather than silently scanning nothing and passing.
func TestFindVendoredConfigsMissingVendorIsError(t *testing.T) {
	if _, err := FindVendoredConfigs(t.TempDir()); err == nil {
		t.Fatal("FindVendoredConfigs() = nil error with no vendor/ directory, want error")
	}
}
