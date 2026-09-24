package main

// DF-BOARDCTL-12: the README quickstart checksum recipe, proven live and
// pinned in-process.
//
// Dogfood run 15 (2026-09-24) found the install recipe failing verbatim when
// the asset is saved under a different filename: `sha256sum -c` matches by
// the name recorded in SHA256SUMS, so a release asset must keep its published
// filename. Commit ac79279 rewrote the README block (underscore asset names,
// --ignore-missing, the filename caveat) but the result was never proven
// against the real published files — this task proved it before pinning it:
// the four documented commands were run verbatim against the live v0.1.8
// assets and passed (downloads OK, `sha256sum -c --ignore-missing SHA256SUMS`
// exiting 0 with "boardctl_linux_amd64: OK", `./boardctl_linux_amd64 version`
// printing `boardctl version v0.1.8`). The README needed no fix; what it
// needed was a pin, so the proven recipe cannot silently drift again.
//
// Three tests, mirroring the in-process pin logic of TestVersioncheck
// (BT-036) in internal/versioncheck:
//
//   - TestDF12QuickstartBlockPinned — static: the first bash block of the
//     install section contains exactly the documented command shapes; every
//     /releases/download/ URL in it names the `Current release:` pin tag
//     (extracted with internal/versioncheck's own regex — shared, never a
//     drift-prone copy); the asset names follow the published
//     `<binary>_<goos>_<goarch>` rule (+SHA256SUMS), the example platform is
//     linux_amd64, and the retired hyphenated spellings stay banned.
//
//   - TestDF12QuickstartSHA256SUMSPublishesBinaryAsset — THE LIVE CHECK:
//     fetches the pinned tag's SHA256SUMS over the network (the one small
//     text file the recipe itself fetches) and verifies every binary asset
//     the README downloads is listed in it. Gated behind BOARDCTL_DF12_LIVE=1
//     so plain `go test ./...` stays hermetic; `make test` and CI run offline.
//
//   - TestDF12QuickstartDetectorsDiscriminate — the shape checkers separate
//     the healthy recipe from the drift shapes they must catch, so this pin
//     cannot rot into a test that passes on broken READMEs.
import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/coding-hermes/boardctl/internal/versioncheck"
)

// df12AssetNameRe is the published asset-naming rule from README's install
// section: `<binary>_<goos>_<goarch>` with underscores (`.exe` on windows),
// matching what both cut paths (tag-push CI and `make release`) emit.
var df12AssetNameRe = regexp.MustCompile(`^boardctl_[a-z0-9]+_[a-z0-9]+(\.exe)?$`)

// df12SumsLineRe is one line of a published SHA256SUMS: a lowercase sha256
// hex digest, two spaces of separation, the asset name.
var df12SumsLineRe = regexp.MustCompile(`^([0-9a-f]{64})\s+(\S+)$`)

// Command shapes of the quickstart recipe. The comment tail a README line may
// carry for prose (`... # verify the binary you downloaded`) is stripped
// before matching, so the checker reads the COMMAND, not the annotation.
var (
	df12CurlRe  = regexp.MustCompile(`^curl -sL -o (\S+) (\S+)$`)
	df12ShaRe   = regexp.MustCompile(`^sha256sum -c --ignore-missing (\S+)$`)
	df12ChmodRe = regexp.MustCompile(`^chmod \+x (\S+) && \./(\S+) version$`)
)

// df12Quickstart is the parsed shape of the README quickstart block.
type df12Quickstart struct {
	// curlAssets are the `-o` names on the curl lines, in block order.
	curlAssets []string
	// urls are the URLs on those curl lines, in the same order.
	urls []string
	// checksum is the file passed to `sha256sum -c --ignore-missing`.
	checksum string
	// chmodAsset is the asset the last line chmods and executes.
	chmodAsset string
}

// df12QuickstartBlock extracts the FIRST bash code block of README.md's
// install section. The anchor is the `Current release:` pin line (the same
// anchor internal/versioncheck gates on); the `[releases](../../releases)`
// link must appear before it, proving the pin sits in the Install section and
// not in some other prose block a future edit may add.
func df12QuickstartBlock(t *testing.T, root string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, versioncheck.READMEName))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	pin := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "Current release: **") {
			pin = i
			break
		}
	}
	if pin < 0 {
		t.Fatalf("README has no `Current release:` pin line to anchor the install block on")
	}
	install := false
	for _, l := range lines[:pin] {
		if strings.Contains(l, "[releases](../../releases)") {
			install = true
		}
	}
	if !install {
		t.Fatalf("the `Current release:` pin line (line %d) has no [releases](../../releases) link above it — it is not in the Install section", pin+1)
	}

	// The first fence after the pin must be the bash quickstart block.
	start := -1
	for i := pin; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			if !strings.HasPrefix(lines[i], "```bash") {
				t.Fatalf("first code fence after the Current release line (line %d) is %q, not a bash block", i+1, lines[i])
			}
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("no code fence opens after the Current release line — the install block is gone")
	}
	var block []string
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "```") {
			return block
		}
		block = append(block, lines[i])
	}
	t.Fatalf("the quickstart bash block is never closed (no ``` after line %d)", start)
	return nil
}

// df12ParseQuickstart validates every block line against the documented
// command shapes and returns the parsed recipe. Every line must be one of:
// `curl -sL -o <asset> <url>`, `sha256sum -c --ignore-missing <file>`,
// `chmod +x <asset> && ./<asset> version` (a trailing `# comment` is prose
// and ignored). An unknown line means the recipe changed shape — the
// copy-paste-ability the dogfood run depended on is no longer proven.
func df12ParseQuickstart(lines []string) (df12Quickstart, error) {
	q := df12Quickstart{}
	for i, raw := range lines {
		cmd := raw
		if c := strings.Index(cmd, " #"); c >= 0 {
			cmd = cmd[:c]
		}
		cmd = strings.TrimRight(cmd, " \t")

		switch {
		case df12CurlRe.MatchString(cmd):
			m := df12CurlRe.FindStringSubmatch(cmd)
			q.curlAssets = append(q.curlAssets, m[1])
			q.urls = append(q.urls, m[2])
		case df12ShaRe.MatchString(cmd):
			q.checksum = df12ShaRe.FindStringSubmatch(cmd)[1]
		case df12ChmodRe.MatchString(cmd):
			m := df12ChmodRe.FindStringSubmatch(cmd)
			if m[1] != m[2] {
				return q, fmt.Errorf("line %d chmods %q but executes ./%q — the executed binary would not be the verified one: %q", i+1, m[1], m[2], raw)
			}
			q.chmodAsset = m[1]
		default:
			return q, fmt.Errorf("line %d does not match a documented recipe shape (curl -sL -o <asset> <url>, sha256sum -c --ignore-missing <file>, chmod +x <asset> && ./<asset> version): %q", i+1, raw)
		}
	}
	return q, nil
}

// TestDF12QuickstartBlockPinned is the static half: shape, tag agreement, and
// the published naming rule for the recipe README documents today.
func TestDF12QuickstartBlockPinned(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := versioncheck.RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	surfaces, err := versioncheck.ReadSurfaces(filepath.Join(root, versioncheck.READMEName))
	if err != nil {
		t.Fatalf("ReadSurfaces: %v", err)
	}
	if len(surfaces.PinTags) == 0 {
		t.Fatal("no Current release pin line to read the pinned tag from")
	}
	tag := surfaces.PinTags[0]

	block := df12QuickstartBlock(t, root)
	q, err := df12ParseQuickstart(block)
	if err != nil {
		t.Fatalf("README quickstart block no longer parses as the documented recipe:\n%v", err)
	}

	// Shape: the recipe is two downloads, one verify, one run.
	if len(q.curlAssets) < 2 || len(q.urls) != len(q.curlAssets) {
		t.Fatalf("quickstart block has %d curl lines (want >=2) and %d URLs — the download pair is not documented", len(q.curlAssets), len(q.urls))
	}
	if q.checksum == "" {
		t.Fatal("quickstart block has no `sha256sum -c --ignore-missing <file>` line — the checksum step is gone")
	}
	if q.chmodAsset == "" {
		t.Fatal("quickstart block has no `chmod +x <asset> && ./<asset> version` line — the run step is gone")
	}

	// (b) Every URL in the block pins the SAME tag as the Current release
	//     line, extracted with internal/versioncheck's own regex so this
	//     test cannot drift from what `make version-check` enforces.
	for i, u := range q.urls {
		m := versioncheck.DownloadURLRe.FindStringSubmatch(u)
		if m == nil {
			t.Errorf("curl URL #%d %q is not a /releases/download/<tag>/ URL — it would not fetch the pinned release", i+1, u)
			continue
		}
		if m[1] != tag {
			t.Errorf("curl URL #%d pins %q but the Current release line says %q", i+1, m[1], tag)
		}
	}

	// (c) Asset names follow the published rule; the example platform is
	//     linux_amd64 and SHA256SUMS is downloaded; the retired hyphenated
	//     spellings (the shape the dogfood run saw fail) stay banned.
	for _, a := range q.curlAssets {
		if a == "SHA256SUMS" {
			continue
		}
		if !df12AssetNameRe.MatchString(a) {
			t.Errorf("curl asset %q does not follow the published naming rule <binary>_<goos>_<goarch> (underscore names; .exe on windows)", a)
		}
	}
	for _, want := range []string{"SHA256SUMS", "boardctl_linux_amd64"} {
		if !slices.Contains(q.curlAssets, want) {
			t.Errorf("quickstart block does not download %q — the documented assets and the published assets must be the same set", want)
		}
	}
	for _, stale := range []string{"sha256sums.txt", "boardctl-linux-", "boardctl-darwin-", "boardctl-windows-", "boardctl-freebsd-"} {
		for _, a := range q.curlAssets {
			if strings.Contains(a, stale) {
				t.Errorf("quickstart block still names the retired asset spelling %q", a)
			}
		}
	}

	// The verify and run steps must operate on files the block downloads:
	// that is the exact shape the bare-download dogfood failure broke.
	if !slices.Contains(q.curlAssets, q.checksum) {
		t.Errorf("sha256sum -c reads %q, which the block never downloads", q.checksum)
	}
	if !slices.Contains(q.curlAssets, q.chmodAsset) {
		t.Errorf("chmod/run step uses %q, which the block never downloads", q.chmodAsset)
	}
	if q.chmodAsset != "boardctl_linux_amd64" {
		t.Errorf("the example executes %q; the documented platform example is linux_amd64", q.chmodAsset)
	}
}

// TestDF12QuickstartSHA256SUMSPublishesBinaryAsset is the LIVE check
// (DF-BOARDCTL-12 acceptance d): it fetches the pinned tag's SHA256SUMS — the
// small text file the recipe itself downloads — from GitHub and verifies the
// binary asset the README documents is actually listed in it. This is the
// proof that the pinned tag publishes the names the install block fetches,
// which is what run 15's bare-download failure invalidated.
//
// Skipped unless BOARDCTL_DF12_LIVE=1, so plain `go test ./...` (and
// `make test`, and CI) stays hermetic. Run it deliberately:
//
//	BOARDCTL_DF12_LIVE=1 go test -run TestDF12 ./cmd/boardctl/
func TestDF12QuickstartSHA256SUMSPublishesBinaryAsset(t *testing.T) {
	if os.Getenv("BOARDCTL_DF12_LIVE") != "1" {
		t.Skipf("BOARDCTL_DF12_LIVE not set — the live GitHub check is skipped so `go test ./...` stays hermetic; run with BOARDCTL_DF12_LIVE=1 to fetch https://github.com/coding-hermes/boardctl/releases over the network")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := versioncheck.RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	surfaces, err := versioncheck.ReadSurfaces(filepath.Join(root, versioncheck.READMEName))
	if err != nil {
		t.Fatalf("ReadSurfaces: %v", err)
	}
	if len(surfaces.PinTags) == 0 {
		t.Fatal("no Current release pin line to read the pinned tag from")
	}
	tag := surfaces.PinTags[0]

	q, err := df12ParseQuickstart(df12QuickstartBlock(t, root))
	if err != nil {
		t.Fatalf("README quickstart block no longer parses as the documented recipe:\n%v", err)
	}
	var binaryAssets []string
	for _, a := range q.curlAssets {
		if a != "SHA256SUMS" {
			binaryAssets = append(binaryAssets, a)
		}
	}
	if len(binaryAssets) == 0 {
		t.Fatal("quickstart block downloads no binary asset — nothing to verify against SHA256SUMS")
	}

	url := fmt.Sprintf("https://github.com/coding-hermes/boardctl/releases/download/%s/SHA256SUMS", tag)
	sumsPath := filepath.Join(t.TempDir(), "SHA256SUMS")
	cmd := exec.Command("curl", "-sSLf", "--retry", "2", "--retry-delay", "2", "-m", "60", "-o", sumsPath, url)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("curl of %s failed: %v\nstderr:\n%s", url, err, strings.TrimSpace(stderr.String()))
	}
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		t.Fatalf("read downloaded SHA256SUMS: %v", err)
	}

	sums := map[string]string{}
	for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		m := df12SumsLineRe.FindStringSubmatch(line)
		if m == nil {
			t.Errorf("SHA256SUMS line %d is not `<sha256>  <name>`: %q", i+1, line)
			continue
		}
		sums[m[2]] = m[1]
	}
	if len(sums) == 0 {
		t.Fatal("downloaded SHA256SUMS parsed to zero entries")
	}
	for _, a := range binaryAssets {
		if _, ok := sums[a]; !ok {
			t.Errorf("the quickstart downloads %q from release %s, but that release's SHA256SUMS does not list it — the documented recipe cannot verify this binary (listed: %v)", a, tag, df12SortedNames(sums))
		}
	}
	t.Logf("live proof: release %s publishes %d checksummed assets; quickstart assets %v are all listed", tag, len(sums), binaryAssets)
}

// df12SortedNames lists a SHA256SUMS name->digest map's names sorted, for a
// readable failure message (sorted output is deterministic across runs).
func df12SortedNames(sums map[string]string) []string {
	names := make([]string, 0, len(sums))
	for name := range sums {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// TestDF12QuickstartDetectorsDiscriminate proves the shape checkers separate
// the healthy recipe from the drift shapes they must catch — the same
// anti-vacuity discipline TestUsageDetectorsDiscriminate applies to the
// usage-text checkers. Without this, a loosened regex would let the pin pass
// on a broken README and every assertion above would be decorative.
func TestDF12QuickstartDetectorsDiscriminate(t *testing.T) {
	good := []string{
		"curl -sL -o boardctl_linux_amd64 https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/boardctl_linux_amd64",
		"curl -sL -o SHA256SUMS https://github.com/coding-hermes/boardctl/releases/download/v1.2.3/SHA256SUMS",
		"sha256sum -c --ignore-missing SHA256SUMS   # verify the binary you downloaded",
		"chmod +x boardctl_linux_amd64 && ./boardctl_linux_amd64 version",
	}
	if _, err := df12ParseQuickstart(good); err != nil {
		t.Fatalf("healthy recipe rejected: %v", err)
	}

	drift := []struct {
		name  string
		lines []string
	}{
		{
			name:  "unknown command line",
			lines: append(slices.Clone(good), "pip install boardctl"),
		},
		{
			name:  "curl with different flags",
			lines: []string{"curl -O https://example.com/boardctl_linux_amd64"},
		},
		{
			name:  "sha256sum without --ignore-missing",
			lines: []string{"sha256sum -c SHA256SUMS"},
		},
		{
			name:  "chmod asset differs from executed asset",
			lines: []string{"chmod +x boardctl_linux_amd64 && ./boardctl version"},
		},
		{
			name:  "comment-only line",
			lines: []string{"# download then verify"},
		},
	}
	for _, tc := range drift {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := df12ParseQuickstart(tc.lines); err == nil {
				t.Fatalf("drift shape accepted: %v", tc.lines)
			}
		})
	}

	// Asset-name rule: underscores good (incl. the windows .exe form),
	// the retired hyphenated spelling and the old checksums name rejected.
	if !df12AssetNameRe.MatchString("boardctl_linux_amd64") ||
		!df12AssetNameRe.MatchString("boardctl_windows_amd64.exe") {
		t.Fatal("asset rule rejects a published naming shape")
	}
	if df12AssetNameRe.MatchString("boardctl-linux-amd64") ||
		df12AssetNameRe.MatchString("sha256sums.txt") {
		t.Fatal("asset rule accepts a retired spelling")
	}

	// SHA256SUMS line rule: the published two-space digest shape parses,
	// prose does not.
	if !df12SumsLineRe.MatchString("5d422d6ab37d13032f49fa7e4dccb01cdb6b4480c6b56403f6fe529f87e49b6d  boardctl_linux_amd64") {
		t.Fatal("sums rule rejects the published line shape")
	}
	if df12SumsLineRe.MatchString("boardctl_linux_amd64: OK") {
		t.Fatal("sums rule accepts non-checksum output")
	}
}
