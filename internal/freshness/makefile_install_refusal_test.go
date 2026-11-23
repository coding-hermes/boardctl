package freshness

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// BT-042 / RELEASE-2026-11-16-02: `make install` is the release deploy path.
// With no staged release assets it must REFUSE loudly — exit non-zero, name
// README '## Install' and the staged files — and must never fall back to a
// from-HEAD build. The fallback (added for QA-BOARDCTL-FOREMAN-3) is what
// re-created deployed-binary drift five times: a checkout build reports `dev`
// / vcs.modified=true and fails install-check. This test drives the real
// Makefile so the invariant cannot regress silently.
//
// The run is hermetic: DIST points at an assets-free directory and
// INSTALL_DIR/INSTALL_BIN are redirected into the test's temp dir, so even a
// regressed (fallback) recipe could never touch ~/.local/bin.

func repoRoot(t *testing.T) string {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return repo
}

func assertInstallRefusal(t *testing.T, repo, dist string) {
	t.Helper()

	installDir := t.TempDir()
	cmd := exec.Command("make", "install",
		"DIST="+dist,
		"INSTALL_DIR="+installDir,
		"INSTALL_BIN="+filepath.Join(installDir, "boardctl"),
	)
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("make install with DIST=%s (no staged assets) exited 0; want loud refusal", dist)
	}
	msg := stderr.String()
	for _, want := range []string{"SHA256SUMS", "README '## Install'", "curl"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal stderr missing %q; stderr:\n%s", want, msg)
		}
	}
}

func TestMakeInstallRefusesWithoutStagedAssets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX make refusal test")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not available")
	}
	repo := repoRoot(t)

	// empty-dist: DIST exists but stages nothing.
	assertInstallRefusal(t, repo, t.TempDir())

	// no-dist: DIST does not exist at all (fresh clone / worktree).
	assertInstallRefusal(t, repo, filepath.Join(t.TempDir(), "dist"))
}
