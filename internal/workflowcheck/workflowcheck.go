// Package workflowcheck is the repo's CI-vs-Makefile platform-parity gate plus
// the reusable-workflow pin gate (mirrors internal/versioncheck): a check-only
// scanner over two release surfaces that must never drift apart.
//
// One checker, three gates (kept in agreement by construction):
//
//   - plain `go test ./...` (incl. `-short`) picks this package up like any other
//   - TestWorkflowcheck / TestRefPinned run in-process against the repo tree
//   - `go test -count=1 -run 'TestWorkflowcheck|TestRefPinned' ./internal/workflowcheck`
//     is the standalone command the failure messages name
//
// The surfaces checked:
//
//   - .github/workflows/multiarch.yml: the `platforms:` input (a JSON array in
//     a single-quoted string) and the `uses:` ref of the reusable workflow
//     (coding-hermes/.github), which must be pinned — a full 40-hex commit SHA
//     or a vX.Y.Z tag — never a floating branch ref like @main, which lets an
//     upstream change silently alter what this repo publishes.
//   - Makefile: the `PLATFORMS := \` backslash-continued list.
//
// Both platform lists must agree exactly (same set, same order, no
// duplicates): a tag-push cut and a `make release` cut publish the same
// assets, so the lists cannot be allowed to differ even in order.
//
// Zero new dependencies: the platforms line is a JSON array inside a
// single-quoted string, so encoding/json on the extracted string is enough.
//
// This package is a CHECK, never a rewriter: nothing here modifies files.
// It is pure filesystem + string work: no network calls, no git invocation.
package workflowcheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

const (
	// WorkflowName is the CI workflow whose platforms input and uses ref are
	// checked.
	WorkflowName = ".github/workflows/multiarch.yml"
	// MakefileName is the Makefile whose PLATFORMS list is checked.
	MakefileName = "Makefile"
)

var (
	// usesRe matches the reusable workflow's `uses:` value.
	usesRe = regexp.MustCompile(`(?m)^\s*uses:\s*(\S+)`)
	// platformsRe matches the single-quoted JSON platforms input.
	platformsRe = regexp.MustCompile(`(?m)^\s*platforms:\s*'([^']*)'`)
	// pinCommentRe matches the pin-date documentation comment the ref test
	// requires beside the uses line ("Pinned 2026-09-24 ...").
	pinCommentRe = regexp.MustCompile(`(?i)pinned \d{4}-\d{2}-\d{2}`)
	// shaRefRe is a full 40-hex lowercase git commit SHA.
	shaRefRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
	// tagRefRe is a vX.Y.Z(-suffix) release tag.
	tagRefRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$`)
)

// ReadWorkflowPlatforms extracts the platforms list from the workflow's
// `platforms: '[...]'` input, in file order. It fails when the input is
// missing (the reusable workflow's default list would silently apply instead)
// or is not a JSON array of strings.
func ReadWorkflowPlatforms(workflowPath string) ([]string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, err
	}
	m := platformsRe.FindStringSubmatch(string(data))
	if m == nil {
		return nil, fmt.Errorf("workflowcheck: %s has no explicit `platforms: '[…]'` input — the reusable workflow's default platform list would apply instead of the Makefile's", workflowPath)
	}
	var out []string
	if err := json.Unmarshal([]byte(m[1]), &out); err != nil {
		return nil, fmt.Errorf("workflowcheck: %s platforms %q is not a JSON array of strings: %v", workflowPath, m[1], err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("workflowcheck: %s platforms list is empty", workflowPath)
	}
	return out, nil
}

// ReadWorkflowRef extracts the `uses:` value of the reusable workflow call.
func ReadWorkflowRef(workflowPath string) (string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", err
	}
	m := usesRe.FindStringSubmatch(string(data))
	if m == nil {
		return "", fmt.Errorf("workflowcheck: %s has no `uses:` line", workflowPath)
	}
	return m[1], nil
}

// RefName returns the git ref part of a `uses:` value: everything after the
// last '@' (owner/repo paths may contain '@'-free subgroups, but not '@').
func RefName(uses string) string {
	if i := strings.LastIndex(uses, "@"); i >= 0 {
		return uses[i+1:]
	}
	return uses
}

// CheckRef enforces the pin policy on a reusable-workflow ref: a full 40-hex
// commit SHA or a vX.Y.Z tag is immutable enough; a branch name (@main) or a
// short SHA is a floating ref and fails.
func CheckRef(ref string) error {
	switch {
	case shaRefRe.MatchString(ref):
		return nil
	case tagRefRe.MatchString(ref):
		return nil
	}
	return fmt.Errorf("workflowcheck: workflow ref %q is a floating ref (mutable) — pin `uses:` to a full 40-hex commit SHA or a vX.Y.Z release tag of the reusable workflow repo, then re-run go test ./internal/workflowcheck", ref)
}

// ReadMakefilePlatforms extracts the PLATFORMS list from the Makefile, in file
// order. It follows make's own continuation rule: the assignment continues
// while lines end with a backslash, so a recipe line can never bleed in.
func ReadMakefilePlatforms(makefilePath string) ([]string, error) {
	data, err := os.ReadFile(makefilePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	def := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "PLATFORMS :=") {
			def = i
			break
		}
	}
	if def == -1 {
		return nil, fmt.Errorf("workflowcheck: %s has no `PLATFORMS :=` list — add one (the release target iterates it)", makefilePath)
	}
	var out []string
	for i := def; i < len(lines); i++ {
		l := lines[i]
		fields := l
		if i == def {
			fields = strings.TrimPrefix(l, "PLATFORMS :=")
		}
		for _, f := range strings.Fields(fields) {
			if f == `\` {
				continue
			}
			out = append(out, f)
		}
		if !strings.HasSuffix(l, `\`) {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("workflowcheck: %s PLATFORMS list is empty", makefilePath)
	}
	return out, nil
}

// Check verifies that the workflow platforms input and the Makefile PLATFORMS
// list name exactly the same platforms, in the same order, with no duplicates
// on either side. The error names both lists and the fix.
func Check(workflowPath, makefilePath string) error {
	wf, err := ReadWorkflowPlatforms(workflowPath)
	if err != nil {
		return err
	}
	mk, err := ReadMakefilePlatforms(makefilePath)
	if err != nil {
		return err
	}
	var problems []string
	for _, d := range []struct {
		name string
		list []string
	}{{"workflow platforms", wf}, {"Makefile PLATFORMS", mk}} {
		if dup := duplicates(d.list); len(dup) > 0 {
			problems = append(problems, fmt.Sprintf("%s lists %v more than once", d.name, dup))
		}
	}
	if !slices.Equal(wf, mk) {
		problems = append(problems, fmt.Sprintf(
			"platform drift: workflow platforms %v != Makefile PLATFORMS %v (order counts — a tag-push cut and `make release` must publish the same assets)",
			wf, mk))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("workflowcheck: %d problem(s):\n  %s\nfix: edit .github/workflows/multiarch.yml's `platforms:` and the Makefile's PLATFORMS list to the same platforms in the same order, then re-run go test ./internal/workflowcheck",
		len(problems), strings.Join(problems, "\n  "))
}

// duplicates returns the values appearing more than once, in first-seen order.
func duplicates(list []string) []string {
	seen := make(map[string]bool, len(list))
	var dup []string
	for _, p := range list {
		if seen[p] {
			dup = append(dup, p)
		}
		seen[p] = true
	}
	return dup
}

// RepoRoot returns the module root by walking up from start to the nearest
// directory containing go.mod. The tests call this with the working directory
// first (go test sets cwd to the package dir, so any in-repo invocation
// resolves) and fall back to the test binary's own source location, so the
// result never depends on where the developer happens to cd. Same pattern as
// internal/versioncheck.RepoRoot.
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
