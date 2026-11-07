package board

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain canonicalises TMPDIR once, before any test runs.
//
// boardctl canonicalises board directories (filepath.EvalSymlinks) before it
// reports them, while testing.T.TempDir() echoes TMPDIR exactly as given. On a
// host whose temp root is a symlink — this box runs with
// GOTMPDIR=/var/tmp/hermes-go-tmp -> /var/tmp/hermes-go-tmp-real — every path
// assertion compared two spellings of the same directory, and the suite failed
// for the harness rather than for the contract it exists to pin. Resolving the
// root once here makes the harness agree with the product everywhere instead of
// patching each assertion, and costs nothing on a host whose temp dir is real.
func TestMain(m *testing.M) {
	if dir := os.Getenv("TMPDIR"); dir != "" {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil && resolved != dir {
			_ = os.Setenv("TMPDIR", resolved)
		}
	}
	os.Exit(m.Run())
}
