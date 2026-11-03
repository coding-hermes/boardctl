package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProfile stages a synthetic raw coverage profile in the test's temp
// dir. Hermetic: the command's IO/parse/exit-code mapping is exercised
// in-process via run(), never by shelling out to `go test`.
func writeProfile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "coverage.out")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

const passingProfile = "mode: set\n" +
	"github.com/coding-hermes/boardctl/internal/board/board.go:1.1,2.2 4 1\n" +
	"github.com/coding-hermes/boardctl/internal/board/board.go:3.1,4.2 4 1\n"

const lowPackageProfile = "mode: set\n" +
	"github.com/coding-hermes/boardctl/internal/board/board.go:1.1,2.2 4 1\n" +
	"github.com/coding-hermes/boardctl/internal/board/board.go:3.1,4.2 4 0\n" +
	"github.com/coding-hermes/boardctl/internal/describe/describe.go:1.1,2.2 32 1\n"

func TestCovercheckCommandPassesAtOrAboveFloors(t *testing.T) {
	if code := run([]string{"--profile", writeProfile(t, passingProfile)}); code != 0 {
		t.Fatalf("run exit = %d, want 0 (clean profile)", code)
	}
}

func TestCovercheckCommandFloorViolationExitsOne(t *testing.T) {
	// Total is 90% (>= 80) but internal/board lands at 50%: below the 60%
	// package floor -> exit 1 names the package, not the total.
	if code := run([]string{"--profile", writeProfile(t, lowPackageProfile)}); code != 1 {
		t.Fatalf("run exit = %d, want 1 (package below floor)", code)
	}
}

func TestCovercheckCommandForcedStrictFloorExitsOne(t *testing.T) {
	// A profile that passes the DEFAULT floors (board 100%, describe 87.5%,
	// total 93.75%) still fails when the caller raises the floor above what
	// the packages achieve.
	strict := "mode: set\n" +
		"github.com/coding-hermes/boardctl/internal/board/board.go:1.1,2.2 8 1\n" +
		"github.com/coding-hermes/boardctl/internal/describe/describe.go:1.1,2.2 7 1\n" +
		"github.com/coding-hermes/boardctl/internal/describe/describe.go:2.2,3.3 1 0\n"
	path := writeProfile(t, strict)
	if code := run([]string{"--profile", path}); code != 0 {
		t.Fatalf("run exit = %d, want 0 under default floors", code)
	}
	if code := run([]string{"--profile", path, "--min-package", "95"}); code != 1 {
		t.Fatalf("run exit = %d, want 1 (min-package=95 not met)", code)
	}
}

func TestCovercheckCommandUsageAndIOErrorsExitTwo(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing --profile", []string{}},
		{"empty --profile", []string{"--profile", ""}},
		{"nonexistent file", []string{"--profile", filepath.Join(t.TempDir(), "absent.out")}},
		{"empty profile file", []string{"--profile", writeProfile(t, "")}},
		{"header-only profile", []string{"--profile", writeProfile(t, "mode: set\n")}},
		{"malformed profile", []string{"--profile", writeProfile(t, "mode: set\nnot-a-block\n")}},
		{"negative floor", []string{"--profile", writeProfile(t, passingProfile), "--min-total", "-1"}},
		{"positional arg", []string{"--profile", writeProfile(t, passingProfile), "stray"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := run(tc.args); code != 2 {
				t.Fatalf("run(%v) exit = %d, want 2", tc.args, code)
			}
		})
	}
}

func TestCovercheckCommandExemptListParsing(t *testing.T) {
	// internal/board is sub-floor, so without the exemption the run fails;
	// an exempt entry naming it (comma form, with surrounding spaces)
	// turns the same profile into a pass and must still exit 0.
	path := writeProfile(t, lowPackageProfile)
	if code := run([]string{"--profile", path, "--exempt", " internal/secretscheck ,internal/board"}); code != 0 {
		t.Fatalf("run exit = %d, want 0 with internal/board exempt", code)
	}
	if code := run([]string{"--profile", path, "--exempt", "internal/secretscheck"}); code != 1 {
		t.Fatalf("run exit = %d, want 1 when internal/board is not exempt", code)
	}
}

func TestCovercheckCommandSummaryMentionsLowestPackageAndExempt(t *testing.T) {
	// Capture os.Stdout around run() to assert the human summary surfaces
	// the total, the lowest package and the named exemption.
	path := writeProfile(t, "mode: set\n"+
		"github.com/coding-hermes/boardctl/internal/board/board.go:1.1,2.2 4 1\n"+
		"github.com/coding-hermes/boardctl/internal/render/render.go:1.1,2.2 8 1\n"+
		"github.com/coding-hermes/boardctl/internal/render/render.go:2.2,3.3 1 0\n")
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := run([]string{"--profile", path})
	os.Stdout = orig
	_ = w.Close()
	outBytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(outBytes)
	if code != 0 {
		t.Fatalf("run exit = %d, want 0", code)
	}
	for _, want := range []string{"lowest package", "internal/render", "exempt internal/secretscheck", "PASS"} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary missing %q; output:\n%s", want, out)
		}
	}
}
