package main

// BT-083 (BOARDCTL-HOOK-001) — the boardctl git-hook intake, proven the way
// the row demands:
//
//   - install writes a delimited managed block beside a pre-existing sibling
//     hook body WITHOUT changing a sibling byte, is reachable (the block lands
//     before a top-level `exit`), and uninstall restores the pre-install body
//     BYTE-IDENTICALLY (a second uninstall exits 1 naming the hook);
//   - a REAL `git commit` that adds a pending board row, with the fixture crier
//     answering 503 to every POST, still exits 0 (the hook never blocks git),
//     the row spools, and after the bounded 3 attempts it becomes an
//     operator-visible dead-letter record that `hook doctor` reports;
//   - the posted body is HMAC-signed over timestamp.body and carries
//     Idempotency-Key boardctl:<row-id>:<content-hash> with the row id
//     EXCLUDED from the hash (spec §10.3);
//   - detection is narrow (only newly added pending non-fixture rows), a
//     post-merge with no ORIG_HEAD is a named no-op, and `dispatch` exits 0 on
//     every path — including a broken invocation.
//
// The real-commit tests execute the REAL boardctl binary because the managed
// block is run by GIT as a child process: the in-process run() harness cannot
// stand in for that hop.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// bt083Git runs git in dir with identity/env pinned so a box-global config or
// hook cannot reach the fixture.
func bt083Git(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=bt083-hook", "GIT_AUTHOR_EMAIL=bt083@example.invalid",
		"GIT_COMMITTER_NAME=bt083-hook", "GIT_COMMITTER_EMAIL=bt083@example.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// bt083Repo seeds a real git work tree holding a real boardctl-init'd board
// with one committed baseline row, so a later commit's HEAD^ carries a board
// file to diff against.
func bt083Repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := bt083Git(t, dir, "init", "-q", "-b", "main", "."); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, errOut := instCapture(func() {
		if code := run([]string{"init", "-C", dir, "--project", "t", "--namespace", "t"}); code != 0 {
			t.Errorf("boardctl init inside seed failed (exit %d)", code)
		}
	}); errOut != "" {
		t.Fatalf("boardctl init emitted stderr: %s", errOut)
	}
	board := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	if err := os.WriteFile(board, []byte(`{"id":"BASE-1","status":"complete","title":"baseline","created_at":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write baseline board: %v", err)
	}
	if _, err := bt083Git(t, dir, "add", "."); err != nil {
		t.Fatalf("git add baseline: %v", err)
	}
	if _, err := bt083Git(t, dir, "commit", "-q", "-m", "baseline"); err != nil {
		t.Fatalf("git commit baseline: %v", err)
	}
	return dir
}

// bt083Secret writes a 0600 secret file and returns its path.
func bt083Secret(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "crier.secret")
	if err := os.WriteFile(p, []byte("s3cr3t-not-logged"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatalf("chmod secret: %v", err)
	}
	return p
}

// bt083Bin builds the REAL boardctl binary once per test process. The path is
// pid-keyed under the temp root (NOT t.TempDir(): the first caller's dir is
// deleted when its test ends and later hooks would point at a vanished
// binary).
var (
	bt083BinOnce sync.Once
	bt083BinPath string
	bt083BinErr  error
)

func bt083RealBin(t *testing.T) string {
	t.Helper()
	bt083BinOnce.Do(func() {
		out := filepath.Join(os.TempDir(), fmt.Sprintf("bt083-boardctl-%d", os.Getpid()))
		cmd := exec.Command("go", "build", "-o", out, "github.com/coding-hermes/boardctl/cmd/boardctl")
		cmd.Dir = "" // the test's working directory (the package dir) resolves the module
		if combined, err := cmd.CombinedOutput(); err != nil {
			bt083BinErr = fmt.Errorf("go build boardctl for hook execution: %v\n%s", err, combined)
			return
		}
		bt083BinPath = out
	})
	if bt083BinErr != nil {
		t.Fatal(bt083BinErr)
	}
	return bt083BinPath
}

// bt083RunBin executes the real binary and returns exit code + both streams.
func bt083RunBin(t *testing.T, bin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run %s %v: %v", bin, args, err)
		}
	}
	return code, out.String(), errb.String()
}

// bt083HookRun runs a hook verb in-process, capturing both streams.
func bt083HookRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb string
	code := 0
	out, errb = instCapture(func() { code = run(args) })
	return code, out, errb
}

// TestBT083HookInstallWritesManagedBlock is the table test over both hooks:
// --dry-run writes nothing and prints the exact block; install writes it with
// the BEGIN/END markers and the literal doctor marker.
func TestBT083HookInstallWritesManagedBlock(t *testing.T) {
	for _, hook := range []string{hookKindPostCommit, hookKindPostMerge} {
		t.Run(hook, func(t *testing.T) {
			dir := bt083Repo(t)
			secret := bt083Secret(t, dir)
			target := filepath.Join(dir, ".git", "hooks", hook)

			code, out, errb := bt083HookRun(t, "hook", "install", "--hook", hook, "-C", dir,
				"--crier-url", "http://127.0.0.1:9", "--secret-file", secret, "--self", "/bin/true", "--dry-run")
			if code != 0 {
				t.Fatalf("dry-run exit = %d (stdout %q stderr %q)", code, out, errb)
			}
			if !strings.Contains(out, hookBeginLine(hook)) {
				t.Errorf("dry-run did not print the exact block:\n%s", out)
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("dry-run wrote the hook file (stat err = %v)", err)
			}

			code, out, errb = bt083HookRun(t, "hook", "install", "--hook", hook, "-C", dir,
				"--crier-url", "http://127.0.0.1:9", "--secret-file", secret, "--self", "/bin/true")
			if code != 0 {
				t.Fatalf("install exit = %d (stdout %q stderr %q)", code, out, errb)
			}
			raw, err := os.ReadFile(target)
			if err != nil {
				t.Fatalf("hook not written: %v", err)
			}
			if st, err := os.Stat(target); err != nil || st.Mode()&0o111 == 0 {
				t.Fatalf("hook not executable: mode=%v err=%v", st.Mode(), err)
			}
			for _, want := range []string{
				hookBeginLine(hook),
				hookEndLine,
				fmt.Sprintf("# boardctl-dispatch marker=boardctl hook dispatch --hook %s \"$@\"", hook),
				"boardctl-dispatch block " + hookBlockVersion,
			} {
				if !strings.Contains(string(raw), want) {
					t.Errorf("installed hook missing %q:\n%s", want, raw)
				}
			}
			// The spool never shows in board diffs (spec Q-3).
			gi, err := os.ReadFile(filepath.Join(dir, ".coding-hermes", "board", ".gitignore"))
			if err != nil || !strings.Contains(string(gi), hookSpoolName+"/") {
				t.Errorf("spool not gitignored (err %v): %q", err, gi)
			}
		})
	}
}

// TestBT083HookUninstallRestoresSiblingByteIdentical is the byte-identical
// sibling proof: install never rewrites a sibling byte, places the block so it
// is reachable, and uninstall removes exactly the bytes install added.
func TestBT083HookUninstallRestoresSiblingByteIdentical(t *testing.T) {
	dir := bt083Repo(t)
	secret := bt083Secret(t, dir)

	sibling := "#!/bin/sh\n# a sibling body the installer must never rewrite\nprintf '%s\\n' 'sibling ran' >&2\nexit 0\n"
	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}
	if err := os.WriteFile(hookPath, []byte(sibling), 0o755); err != nil {
		t.Fatalf("write sibling hook: %v", err)
	}

	code, out, errb := bt083HookRun(t, "hook", "install", "--hook", "post-commit", "-C", dir,
		"--crier-url", "http://127.0.0.1:9", "--source-id", "boardctl", "--secret-file", secret, "--self", "/bin/true")
	if code != 0 {
		t.Fatalf("install exit = %d (stdout %q stderr %q)", code, out, errb)
	}
	installed, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read installed hook: %v", err)
	}
	start, end, ok, ferr := hookFindBlock(string(installed), "post-commit")
	if ferr != nil || !ok {
		t.Fatalf("installed hook has no managed block (err %v):\n%s", ferr, installed)
	}
	if got := string(installed[:start]) + string(installed[end:]); got != sibling {
		t.Fatalf("install changed sibling bytes:\n--- before ---\n%q\n--- around block ---\n%q", sibling, got)
	}
	// DF-CRIER-433: the sibling exits 0 at top level, so the block must sit
	// BEFORE that line, or it can never run.
	if why := hookMarkerUnreachable(string(installed), "post-commit"); why != "" {
		t.Fatalf("installed block is unreachable: %s\n%s", why, installed)
	}
	if start > strings.Index(string(installed), "\nexit 0") {
		t.Fatalf("managed block was appended after the top-level `exit 0` (dead code):\n%s", installed)
	}

	code, out, errb = bt083HookRun(t, "hook", "uninstall", "--hook", "post-commit", "-C", dir)
	if code != 0 {
		t.Fatalf("uninstall exit = %d (stdout %q stderr %q)", code, out, errb)
	}
	restored, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read restored hook: %v", err)
	}
	if !bytes.Equal([]byte(sibling), restored) {
		t.Fatalf("sibling body is NOT byte-identical after uninstall:\n--- want ---\n%q\n--- got ---\n%q", sibling, string(restored))
	}

	code, _, errb = bt083HookRun(t, "hook", "uninstall", "--hook", "post-commit", "-C", dir)
	if code != 1 {
		t.Fatalf("second uninstall exit = %d, want 1 (no silent success); stderr %q", code, errb)
	}
	if !strings.Contains(errb, "not installed") {
		t.Errorf("second uninstall stderr %q does not name the failure", errb)
	}
}

// TestBT083HookCommit503ExitsZeroAndDeadLetters is the acceptance story with
// crier down: the commit exists before the hook runs, so the hook cannot block
// it; the row spools; after three attempts it becomes an operator-visible
// dead-letter record.
func TestBT083HookCommit503ExitsZeroAndDeadLetters(t *testing.T) {
	dir := bt083Repo(t)
	secret := bt083Secret(t, dir)
	bin := bt083RealBin(t)

	var mu sync.Mutex
	var received []struct {
		key, ts, sig string
		body         []byte
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		_, _ = body.ReadFrom(r.Body)
		mu.Lock()
		received = append(received, struct {
			key, ts, sig string
			body         []byte
		}{r.Header.Get("Idempotency-Key"), r.Header.Get("X-Crier-Timestamp"), r.Header.Get("X-Crier-Signature"), body.Bytes()})
		mu.Unlock()
		http.Error(w, "crier is down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	code, out, errb := bt083HookRun(t, "hook", "install", "--hook", "post-commit", "-C", dir,
		"--crier-url", srv.URL, "--source-id", "boardctl", "--secret-file", secret, "--self", bin)
	if code != 0 {
		t.Fatalf("install exit = %d (stdout %q stderr %q)", code, out, errb)
	}

	row := `{"id":"BT083-ROW-1","status":"pending","title":"a row that must reach crier","priority":"P2","acceptance_criteria":"delivered","created_at":"2026-01-02T00:00:00Z"}`
	boardPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	f, err := os.OpenFile(boardPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board: %v", err)
	}
	if _, err := f.WriteString(row + "\n"); err != nil {
		t.Fatalf("append row: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close board: %v", err)
	}
	if _, err := bt083Git(t, dir, "add", boardPath); err != nil {
		t.Fatalf("git add row: %v", err)
	}
	// THE proof: crier answers 503 to every POST the hook makes, and the
	// commit still succeeds.
	if _, err := bt083Git(t, dir, "commit", "-q", "-m", "add a board row"); err != nil {
		t.Fatalf("git commit must exit 0 even when crier is down: %v", err)
	}

	mu.Lock()
	n := len(received)
	first := received
	mu.Unlock()
	if n != 1 {
		t.Fatalf("crier fixture saw %d POST(s), want exactly 1 for the one added row", n)
	}
	// The signed body verifies against the registered secret, and the key is
	// boardctl:<row-id>:<content-hash> with the row id excluded from the hash.
	mac := hmac.New(sha256.New, []byte("s3cr3t-not-logged"))
	mac.Write([]byte(first[0].ts + "."))
	mac.Write(first[0].body)
	wantSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if first[0].sig != wantSig {
		t.Errorf("signature = %q, want %q (HMAC over timestamp.body)", first[0].sig, wantSig)
	}
	parts := strings.Split(first[0].key, ":")
	if len(parts) != 3 || parts[0] != "boardctl" || parts[1] != "BT083-ROW-1" || len(parts[2]) != 64 {
		t.Errorf("Idempotency-Key = %q, want boardctl:BT083-ROW-1:<64-hex content hash>", first[0].key)
	}
	var body map[string]any
	if err := json.Unmarshal(first[0].body, &body); err != nil {
		t.Fatalf("posted body is not JSON: %v: %s", err, first[0].body)
	}
	if body["id"] != "BT083-ROW-1" || body["event"] != "row_added" {
		t.Errorf("posted body = %v, want id + event row_added", body)
	}

	spoolDir := filepath.Join(dir, ".coding-hermes", "board", hookSpoolName)
	if depth, err := hookSpoolDepth(spoolDir); err != nil || depth != 1 {
		t.Fatalf("spool depth = %d (err %v), want 1 after the failed delivery", depth, err)
	}

	// Attempt 2 (flush #1) fails; attempt 3 (flush #2) exhausts the budget and
	// dead-letters. Both flushes are the real binary, as an operator runs them.
	for i := 0; i < 2; i++ {
		code, cout, cerr := bt083RunBin(t, bin, "hook", "flush", "-C", dir)
		if code != 0 {
			t.Fatalf("flush attempt %d exit = %d: %s%s", i+2, code, cout, cerr)
		}
	}
	if depth, err := hookSpoolDepth(spoolDir); err != nil || depth != 0 {
		t.Fatalf("spool depth after 3 attempts = %d (err %v), want 0 (dead-lettered)", depth, err)
	}
	dlRaw, err := os.ReadFile(filepath.Join(spoolDir, hookDeadLetterName))
	if err != nil {
		t.Fatalf("dead-letter record missing: %v", err)
	}
	if !strings.Contains(string(dlRaw), "BT083-ROW-1") || !strings.Contains(string(dlRaw), "503") {
		t.Fatalf("dead-letter record does not name the row and the failure:\n%s", dlRaw)
	}
	if strings.Contains(string(dlRaw), "s3cr3t") {
		t.Fatalf("dead-letter record leaked the secret:\n%s", dlRaw)
	}

	// doctor is the operator surface for it: healthy hook, visible census.
	code, dout, derr := bt083RunBin(t, bin, "hook", "doctor", "-C", dir)
	if code != 0 {
		t.Fatalf("doctor exit = %d on a healthy hook: %s%s", code, dout, derr)
	}
	for _, want := range []string{"dead-letter: count=1", "last-delivery: ts=", "BT083-ROW-1"} {
		if !strings.Contains(dout, want) {
			t.Errorf("doctor output missing %q:\n%s", want, dout)
		}
	}
	if strings.Contains(dout, "s3cr3t") {
		t.Errorf("doctor leaked the secret:\n%s", dout)
	}
}

// TestBT083HookDoctorFailsWhenMarkerDeleted pins R7: a managed block whose
// marker line was hand-deleted is refused by name.
func TestBT083HookDoctorFailsWhenMarkerDeleted(t *testing.T) {
	dir := bt083Repo(t)
	secret := bt083Secret(t, dir)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	if code, _, errb := bt083HookRun(t, "hook", "install", "--hook", "post-commit", "-C", dir,
		"--crier-url", srv.URL, "--secret-file", secret, "--self", "/bin/true"); code != 0 {
		t.Fatalf("install exit = %d (stderr %q)", code, errb)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "# boardctl-dispatch marker=") {
			continue // the hand-deleted marker line
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(hookPath, []byte(strings.Join(kept, "\n")), 0o755); err != nil {
		t.Fatalf("rewrite hook: %v", err)
	}

	code, out, _ := bt083HookRun(t, "hook", "doctor", "-C", dir)
	if code != 1 {
		t.Fatalf("doctor exit = %d, want 1 when the marker line is gone (output %q)", code, out)
	}
	if !strings.Contains(out, "post-commit") || !strings.Contains(out, "marker line missing") {
		t.Fatalf("doctor does not name the failing post-commit marker:\n%s", out)
	}
}

// TestBT083HookDetectionSkipsIneligibleRows keeps the intake narrow: a new
// complete row and a fixture row are named SKIPs, never posted, and a
// post-merge with no ORIG_HEAD is a named no-op.
func TestBT083HookDetectionSkipsIneligibleRows(t *testing.T) {
	dir := bt083Repo(t)
	secret := bt083Secret(t, dir)
	var posts int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		posts++
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	boardPath := filepath.Join(dir, ".coding-hermes", "board", "tasks.jsonl")
	f, err := os.OpenFile(boardPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open board: %v", err)
	}
	if _, err := f.WriteString(`{"id":"MERGED-ONLY","status":"complete","title":"already done"}` + "\n" +
		`{"id":"FIXTURE-1","status":"pending","title":"fixture","fixtures":true}` + "\n"); err != nil {
		t.Fatalf("append rows: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close board: %v", err)
	}
	if _, err := bt083Git(t, dir, "add", boardPath); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := bt083Git(t, dir, "commit", "-q", "-m", "ineligible rows"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	code, out, errb := bt083HookRun(t, "hook", "dispatch", "--hook", "post-commit", "-C", dir,
		"--crier-url", srv.URL, "--secret-file", secret)
	if code != 0 {
		t.Fatalf("dispatch exit = %d (stderr %q)", code, errb)
	}
	if posts != 0 {
		t.Fatalf("dispatch posted %d ineligible row(s), want 0", posts)
	}
	if !strings.Contains(out, "SKIP rows") || !strings.Contains(out, "MERGED-ONLY") || !strings.Contains(out, "FIXTURE-1") {
		t.Errorf("dispatch did not name its skips:\n%s", out)
	}

	code, out, _ = bt083HookRun(t, "hook", "dispatch", "--hook", "post-merge", "-C", dir,
		"--crier-url", srv.URL, "--secret-file", secret)
	if code != 0 {
		t.Fatalf("post-merge dispatch with no ORIG_HEAD exit = %d", code)
	}
	if !strings.Contains(out, "NOTE") || posts != 0 {
		t.Errorf("post-merge with no ORIG_HEAD should be a named no-op (output %q, posts %d)", out, posts)
	}
}

// TestBT083HookDispatchNeverBlocks is the table of every way a dispatch can
// be handed nonsense: each one exits 0 toward git and names its refusal.
func TestBT083HookDispatchNeverBlocks(t *testing.T) {
	dir := bt083Repo(t)
	empty := t.TempDir() // no board anywhere under it
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown hook", []string{"hook", "dispatch", "--hook", "not-a-hook", "-C", dir}, "REFUSE"},
		{"no board", []string{"hook", "dispatch", "--hook", "post-commit", "-C", empty}, "REFUSE"},
		{"bad flag value", []string{"hook", "dispatch", "--hook", "post-commit", "-C", dir, "--batch-cap", "NaN"}, "REFUSE"},
		{"no hook at all", []string{"hook", "dispatch", "-C", dir}, "REFUSE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errb := bt083HookRun(t, tc.args...)
			if code != 0 {
				t.Fatalf("dispatch exit = %d, want 0 (never block the commit); stderr %q", code, errb)
			}
			if !strings.Contains(errb, tc.want) {
				t.Errorf("refusal was silent: %q (want %s)", errb, tc.want)
			}
		})
	}
}

// TestBT083HookUsageSurface pins the subcommand wiring: a bare `hook` and an
// unknown verb are usage errors (exit 2) and the top-level help advertises
// the family.
func TestBT083HookUsageSurface(t *testing.T) {
	code, out, errb := bt083HookRun(t, "hook", "-h")
	if code != 0 || !strings.Contains(out, "boardctl hook install") {
		t.Fatalf("hook -h = exit %d, stdout %q, stderr %q; want exit 0 with the family usage", code, out, errb)
	}
	code, _, errb = bt083HookRun(t, "hook")
	if code != 2 || !strings.Contains(errb, "a verb is required") {
		t.Fatalf("bare hook = exit %d (%q), want 2 naming the verbs", code, errb)
	}
	code, _, errb = bt083HookRun(t, "hook", "bogus")
	if code != 2 || !strings.Contains(errb, "unknown verb") {
		t.Fatalf("hook bogus = exit %d (%q), want 2 naming the unknown verb", code, errb)
	}
	out, _ = instCapture(func() { _ = run([]string{"help"}) })
	if !strings.Contains(out, "hook    install") {
		t.Fatalf("usage text does not advertise the hook family:\n%s", out)
	}
}
