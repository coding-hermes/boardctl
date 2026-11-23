package freshness

import (
	"bytes"
	"os"
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

// Make-independent guard for the same invariant: on a host without GNU make
// TestMakeInstallRefusesWithoutStagedAssets skips, so the no-fallback rule is
// ALSO asserted structurally from the Makefile text (runs everywhere, no
// toolchain needed). Closes the uncovered failure path gitreins run c659f0e5
// flagged (skip on make-less hosts leaves the invariant unguarded there).
func TestMakefileInstallPathHasNoSourceFallback(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	text := string(data)

	install := recipeBlock(t, text, "install")
	if !strings.Contains(install, "$(MAKE) install-release") {
		t.Errorf("`install` recipe must delegate to install-release; got:\n%s", install)
	}
	if strings.Contains(install, "build") {
		t.Errorf("`install` recipe must not build from source; got:\n%s", install)
	}

	guard := recipeBlock(t, text, "install-release")
	if !strings.Contains(guard, "exit 1") {
		t.Errorf("install-release must exit non-zero when assets are missing; got:\n%s", guard)
	}
	if !strings.Contains(guard, "SHA256SUMS") {
		t.Errorf("install-release guard must name the staged SHA256SUMS; got:\n%s", guard)
	}
}

// recipeBlock returns the recipe lines of one target as trimmed text. A block
// starts at a non-indented `target:` line (first listed name wins the match)
// and ends at the next non-indented, non-comment target/variable line.
func recipeBlock(t *testing.T, makefile, target string) string {
	t.Helper()
	var out []string
	collecting := false
	for _, line := range strings.Split(makefile, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimLeft(line, " "), "#") {
			continue
		}
		if !strings.HasPrefix(line, "	") {
			head := strings.SplitN(line, ":", 2)[0]
			names := strings.Fields(head)
			isTargetStart := len(names) > 0 && !strings.HasSuffix(head, "=")
			if collecting {
				break
			}
			if isTargetStart {
				for _, n := range names {
					if n == target {
						collecting = true
						break
					}
				}
			}
			continue
		}
		if collecting {
			out = append(out, strings.TrimPrefix(line, "	"))
		}
	}
	if len(out) == 0 {
		t.Fatalf("target %s not found or has no recipe in Makefile", target)
	}
	return strings.Join(out, "\n")
}
