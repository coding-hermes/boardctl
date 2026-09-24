package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"syscall"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// errUsage marks a flag-level failure: run() maps it to exit 2 (the same
// usage class as board.ErrBoardNotFound, README exit-code contract).
var errUsage = errors.New("usage")

// usageErrorf marks a flag-level failure so run() maps it to exit 2.
func usageErrorf(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, a...))
}

// ---------- serve ----------

// BT-021: serve is a loopback-only convenience uploader — upload a board
// folder (webkitdirectory multi-file POST) or a .zip and get the
// self-contained multi-board HTML report back. Design authority:
// docs/specs/board-analytics-report.md §6.1 (serve line), §5.6/5.7 (compare
// + error states), §6.2 (payload), §6.3 (single inline page). Hard
// exclusions (§1 non-goals): no auth, no TLS, no exec, no git reads, no
// writes to board files, not a hosted multi-user app — hence bind refusal
// for any non-loopback host and extraction only under a per-upload temp dir
// that is removed on shutdown.

const (
	// defaultServeAddr is the loopback default (§6.1).
	defaultServeAddr = "127.0.0.1:8787"
	// maxUploadBytes is the upload size cap (7.2.6). A payload above the
	// cap is rejected with HTTP 413 and a readable message; the server
	// stays up.
	maxUploadBytes = 512 << 20 // 512 MB
	// maxBoardsPerUpload caps how many boards one upload may register
	// (~50 per BT-021) so a pathological tree cannot explode the payload.
	maxBoardsPerUpload = 50
)

// zeroBoardsNotice is the visible report banner (payload upload_notice +
// #banner lead, BT-027) when a request carried file parts but registered
// no board while the -C board still renders.
const zeroBoardsNotice = "0 boards found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl"

// cmdServe runs the loopback uploader. Exit codes via the shared contract:
// 2 usage (non-loopback --addr, bad flag), 1 operational failure, 0 after a
// clean SIGINT/SIGTERM shutdown.
func cmdServe(dir string, args []string) error {
	fs := newFlagSet("serve")
	args = reorderArgs(args, valueFlags("C", "addr"))
	addr := fs.String("addr", defaultServeAddr, "listen address (loopback only: 127.0.0.1, localhost or ::1)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "boardctl serve [-C dir] [--addr 127.0.0.1:8787]\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("serve takes no positional args (got %q)", fs.Arg(0))
	}
	if dir == "" {
		dir = cdir
	}

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		return fmt.Errorf("--addr %q: %w", *addr, err)
	}
	if !loopbackHost(host) {
		return usageErrorf("--addr %q refuses non-loopback host %q: serve binds 127.0.0.1, localhost or ::1 only (spec 6.1: loopback convenience uploader, no auth, no TLS)", *addr, host)
	}
	listenAddr := net.JoinHostPort(host, port)

	// The optional -C board, when present, is pre-loaded as the first
	// board of every report (spec 6.1: serve accepts -C). A -C that
	// resolves to nothing is fine: serve still runs, uploads render alone.
	var cBoards []*board.Board
	if b, err := board.Resolve(dir); err == nil {
		cBoards = append(cBoards, b)
		fmt.Fprintf(os.Stdout, "including -C board: %s (%s)\n", b.Dir, b.Topology)
	}

	srv := newServeServer(cBoards)
	httpSrv := &http.Server{
		Addr:              listenAddr,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	// Prove the bound address is the requested one (the kernel answer,
	// not just our intent) before advertising it.
	bound := ln.Addr().String()
	fmt.Fprintf(os.Stdout, "boardctl serve listening on http://%s (loopback only)\n", bound)
	fmt.Fprintf(os.Stdout, "open the uploader in a browser; POST a board folder or .zip to render\n")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			stop()
			srv.removeAllTemps()
			return fmt.Errorf("serve: %w", err)
		}
	case <-ctx.Done():
		// Clean shutdown on SIGINT/SIGTERM exits 0 (BT-021 requirement 6).
		fmt.Fprintln(os.Stdout, "\nshutting down (signal received)")
	}
	stop()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		fmt.Fprintf(os.Stdout, "shutdown: %v\n", err)
	}
	srv.removeAllTemps()
	fmt.Fprintln(os.Stdout, "serve stopped cleanly; upload temp dirs removed")
	return nil
}

// loopbackHost reports whether host is one of the loopback names the spec
// allows (6.1: 127.0.0.1, localhost, ::1).
func loopbackHost(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// uploadSession holds the per-upload extraction state. Every upload gets a
// FRESH temp dir (7.2.8: zero writes to board sources; removed on shutdown).
type uploadSession struct {
	root   string // shared parent under os.TempDir()
	boards []*board.Board
	// identities carries serveBoardIdentity per live board, index-aligned
	// with boards (DF-BOARDCTL-2 replace-on-re-upload bookkeeping).
	identities []string
	warns      []string
}

// serveServer carries the live server state.
type serveServer struct {
	cBoards  []*board.Board
	mu       chan struct{} // 1-slot semaphore: lock() sends, unlock() receives
	sessions []*uploadSession
}

func newServeServer(cBoards []*board.Board) *serveServer {
	return &serveServer{cBoards: cBoards, mu: make(chan struct{}, 1)}
}

func (s *serveServer) lock()   { s.mu <- struct{}{} }
func (s *serveServer) unlock() { <-s.mu }

// serveBoardIdentity is the stable DF-BOARDCTL-2 identity of one board:
// the report-display name's slug (render.boardData.slug = slugify(name)
// with the same empty-name fallback), then the topology (two co-located
// boards of different topologies stay distinct entries). render.slugify is
// unexported, so the slug part is mirrored here; render tests pin the
// shared derivation, and slug collisions only over-merge boards the report
// already presents under one name.
func serveBoardIdentity(b *board.Board) string {
	name := ""
	if h := boardHeaderRow(b); h != nil {
		name = strings.TrimSpace(h.String("project"))
	}
	if name == "" {
		name = filepath.Base(b.Dir)
	}
	return slugifyServe(name) + "|" + b.Topology
}

// boardHeaderRow returns the board's header row under the loader's rules
// (nil when none: topology B, blank/unreadable board.jsonl, or line 1 of
// tasks.jsonl without header shape). Mirrors loadBoard's header ladder.
func boardHeaderRow(b *board.Board) *board.Row {
	if b.IsTopologyA() {
		return firstHeaderShapedRow(b.HeaderPath())
	}
	lines, err := tolerantBoardLines(b.TasksPath())
	if err != nil {
		return nil
	}
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		row, err := board.ParseRow(line)
		if err != nil || !isHeaderShapedServe(row) {
			return nil
		}
		return row
	}
	return nil
}

// firstHeaderShapedRow parses the first non-blank line of board.jsonl and
// keeps it only when it has header shape (nil otherwise).
func firstHeaderShapedRow(path string) *board.Row {
	lines, err := tolerantBoardLines(path)
	if err != nil {
		return nil
	}
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		row, err := board.ParseRow(line)
		if err != nil || !isHeaderShapedServe(row) {
			return nil
		}
		return row
	}
	return nil
}

// headerShapeKeysServe mirrors render.headerShapeKeys: the keys that make a
// row a header (no id + at least one of these).
var headerShapeKeysServe = []string{"project", "namespace", "version", "ticks_total"}

// isHeaderShapedServe mirrors render.isHeaderShape.
func isHeaderShapedServe(row *board.Row) bool {
	if row.String("id") != "" {
		return false
	}
	for _, k := range headerShapeKeysServe {
		if row.Has(k) {
			return true
		}
	}
	return false
}

// tolerantBoardLines mirrors render.tolerantLines for identity reads.
func tolerantBoardLines(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.Split(data, []byte("\n")), nil
}

// slugifyServe is byte-for-byte render.slugify (internal/render/load.go):
// lowercased, runs of non-alphanumerics collapsed to a single "-", trimmed
// at the edges, empty input -> "board".
func slugifyServe(name string) string {
	var sb strings.Builder
	prev := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prev = false
			continue
		}
		if !prev && sb.Len() > 0 {
			sb.WriteByte('-')
			prev = true
		}
	}
	out := strings.Trim(sb.String(), "-")
	if out == "" {
		out = "board"
	}
	return out
}

// mergeByIdentity returns boards with later same-identity entries replacing
// earlier ones (first-seen order kept): re-uploading a board must replace
// its previous copy everywhere the report is derived from (DF-BOARDCTL-2).
func mergeByIdentity(boards []*board.Board) []*board.Board {
	idx := map[string]int{}
	out := make([]*board.Board, 0, len(boards))
	for _, b := range boards {
		if b == nil {
			continue
		}
		id := serveBoardIdentity(b)
		if i, ok := idx[id]; ok {
			out[i] = b
			continue
		}
		idx[id] = len(out)
		out = append(out, b)
	}
	return out
}

// currentBoards returns the boards carried by live sessions with
// same-identity entries collapsed (later uploads replace earlier ones, for
// /api/boards, the compare view and every payload built from this list).
func (s *serveServer) currentBoards() []*board.Board {
	s.lock()
	defer s.unlock()
	var out []*board.Board
	out = append(out, s.cBoards...)
	for _, sess := range s.sessions {
		out = append(out, sess.boards...)
	}
	return mergeByIdentity(out)
}

// registerSession records a live upload session under the lock, dropping
// any boards it replaces: a superseded board's session loses that board
// (and is deleted outright once it carries none), together with its now
// orphaned temp extraction tree (its files were copied per upload).
func (s *serveServer) registerSession(sess *uploadSession) {
	s.lock()
	defer s.unlock()
	superseded := map[string]bool{}
	for _, id := range sess.identities {
		superseded[id] = true
	}
	kept := s.sessions[:0]
	for _, old := range s.sessions {
		live := old.boards[:0]
		for i, b := range old.boards {
			if !superseded[old.identities[i]] {
				live = append(live, b)
			}
		}
		if len(live) == len(old.boards) {
			kept = append(kept, old)
			continue
		}
		// Index-aligned compaction: identities[i] belongs to boards[i],
		// so the survivors keep their identity entries under the same
		// relative order.
		liveIDs := old.identities[:0]
		for _, id := range old.identities {
			if !superseded[id] {
				liveIDs = append(liveIDs, id)
			}
		}
		old.boards = live
		old.identities = liveIDs
		if len(live) == 0 {
			os.RemoveAll(old.root)
			continue
		}
		kept = append(kept, old)
	}
	s.sessions = kept
	s.sessions = append(s.sessions, sess)
}

// routes wires the serve HTTP surface (shared by cmdServe and tests).
func (s *serveServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("POST /", s.handleUpload)
	mux.HandleFunc("GET /api/boards", s.handleAPIBoards)
	return mux
}

// removeAllTemps deletes every upload temp tree (shutdown path, 7.2.8).
func (s *serveServer) removeAllTemps() {
	s.lock()
	defer s.unlock()
	for _, sess := range s.sessions {
		os.RemoveAll(sess.root)
	}
	s.sessions = nil
}

// handleIndex serves the inline uploader page (7.2.1: GET / -> 200 + form;
// 6.3: one self-contained document, inline CSS+JS, no CDN, no external
// fonts/images, no fetch to external origins).
func (s *serveServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, uploaderPage)
}

const uploaderPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>boardctl serve — board report uploader</title>
<style>
:root{--bg:#0d1117;--bg2:#161b22;--fg:#e6edf3;--muted:#8b949e;--line:#30363d;--accent:#58a6ff}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.5 system-ui,sans-serif;display:flex;min-height:100vh;align-items:center;justify-content:center}
main{width:min(680px,92vw)}
h1{font-size:20px;margin:0 0 4px}
p.sub{color:var(--muted);margin:0 0 20px}
.card{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:20px}
label{display:block;font-weight:600;margin:14px 0 6px}
input[type=file]{color:var(--fg)}
.hint{color:var(--muted);font-size:12px;margin:4px 0 0}
button{margin-top:20px;background:var(--accent);color:#0d1117;border:0;border-radius:6px;padding:9px 18px;font-weight:700;font-size:14px;cursor:pointer}
button:disabled{opacity:.5;cursor:wait}
code{background:var(--bg);border:1px solid var(--line);border-radius:4px;padding:1px 5px;font-size:12px}
</style>
</head>
<body>
<main>
<h1>Board report uploader</h1>
<p class="sub">Upload a board folder (or a .zip of board folders) and get the self-contained multi-board analytics report back. Loopback only; nothing is written to your board files.</p>
<div class="card">
<form id="up" method="post" action="/" enctype="multipart/form-data">
<label for="folder">Board folder (every directory containing tasks.jsonl + events.jsonl is registered as a board)</label>
<input type="file" id="folder" name="files" webkitdirectory multiple>
<p class="hint">Folder upload sends every file under the chosen directory with relative paths preserved.</p>
<label for="zipfile">…or a .zip archive (max 512 MB, up to 50 boards)</label>
<input type="file" id="zipfile" name="zip" accept=".zip,application/zip">
<p class="hint">The zip may contain one or more board directories (topology A or B).</p>
<button type="submit" id="go">Render report</button>
</form>
</div>
<p class="sub" style="margin-top:16px">Loaded boards: <a href="/api/boards" style="color:var(--accent)">/api/boards</a> — CLI equivalent: <code>boardctl render -C dir</code>.</p>
</main>
<script>
document.getElementById("up").addEventListener("submit", function(){
  var b = document.getElementById("go");
  b.disabled = true; b.textContent = "Rendering…";
});
</script>
</body>
</html>
`

// handleUpload accepts a multipart folder upload (multiple files with
// relative paths in their filenames) or a single .zip part, extracts into a
// fresh per-upload temp dir, resolves every board, and answers with the
// rendered multi-board HTML report.
func (s *serveServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Enforce the size cap before buffering the whole body (7.2.6): an
	// oversized Content-Length is refused up front; a lying/absent one is
	// caught while streaming through MaxBytesReader.
	if r.ContentLength > maxUploadBytes {
		http.Error(w, fmt.Sprintf("upload too large: payload exceeds the %d MB cap", maxUploadBytes>>20), http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, fmt.Sprintf("upload too large: payload exceeds the %d MB cap", maxUploadBytes>>20), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "bad multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	root, err := os.MkdirTemp("", "boardctl-serve-")
	if err != nil {
		http.Error(w, "internal: create temp dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sess := &uploadSession{root: root}
	defer func() {
		if sess.boards == nil {
			// Nothing worth keeping (failed upload): drop the temp tree now.
			os.RemoveAll(root)
		}
	}()

	// A single .zip part wins when present; otherwise treat every other
	// file part as one entry of a folder (webkitdirectory) upload.
	if zparts := r.MultipartForm.File["zip"]; len(zparts) > 0 {
		f, err := zparts[0].Open()
		if err != nil {
			http.Error(w, "open zip part: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		if err := importZip(f, zparts[0].Size, root, sess); err != nil {
			http.Error(w, err.msg, err.status)
			return
		}
	} else {
		var files []*multipart.FileHeader
		for _, fps := range r.MultipartForm.File {
			files = append(files, fps...)
		}
		if len(files) == 0 {
			http.Error(w, "no files uploaded: pick a board folder or a .zip", http.StatusBadRequest)
			return
		}
		if err := extractFolder(files, root, sess); err != nil {
			http.Error(w, err.msg, err.status)
			return
		}
	}

	boards := resolveBoards(root, sess)
	total := len(boards) + len(s.cBoards)
	if total == 0 {
		http.Error(w, "no board found in upload: a board is a directory containing BOTH tasks.jsonl and events.jsonl", http.StatusBadRequest)
		return
	}
	// BT-027: reaching this point means the request carried at least one
	// file part (a zero-part request was rejected 400 above), so an upload
	// that registered 0 boards while -C carries the response must warn
	// VISIBLY in the report. The old behavior fell through and rendered a
	// clean-looking report of the -C board only (HTTP 200, no hint), so a
	// caller posting a mis-named part or a tree without the board pair saw
	// success. The still-200 verdict is deliberate: the -C board renders,
	// only the upload contributed nothing.
	var uploadNotice string
	if len(boards) == 0 && len(s.cBoards) > 0 {
		uploadNotice = zeroBoardsNotice
	}
	if total > maxBoardsPerUpload {
		http.Error(w, fmt.Sprintf("upload registers %d boards; cap is %d", total, maxBoardsPerUpload), http.StatusRequestEntityTooLarge)
		return
	}
	// Mark the session as live BEFORE registering it: a session with
	// boards set survives until shutdown (7.2.8); a failed upload leaves
	// boards nil and the deferred cleanup removes its temp dir at once.
	// DF-BOARDCTL-2: identity strings ride the session (index-aligned with
	// boards) so registerSession can drop superseded boards.
	sess.boards = boards
	for _, b := range boards {
		sess.identities = append(sess.identities, serveBoardIdentity(b))
	}
	s.registerSession(sess)

	all := mergeByIdentity(append(append([]*board.Board{}, s.cBoards...), boards...))
	payload, err := render.BuildBoards(all, render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		http.Error(w, "build report: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Empty on every non-serve path (render/Build never sets it; omitempty
	// keeps those payloads byte-identical to board-report/v1).
	payload.UploadNotice = uploadNotice
	html, err := render.RenderHTML(payload)
	if err != nil {
		http.Error(w, "render report: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, html)
}

// uploadError is an HTTP-mapped failure.
type uploadError struct {
	status int
	msg    string
}

// importZip unpacks a zip stream under root with zip-slip guarding (7.2.5:
// an entry like ../../evil.txt never escapes root; it is rejected with a
// warning and the report still renders). r is typically a multipart.File
// (which is an io.ReaderAt) or a *bytes.Reader in tests.
func importZip(r io.ReaderAt, size int64, root string, sess *uploadSession) *uploadError {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return &uploadError{http.StatusBadRequest, "not a readable zip archive: " + err.Error()}
	}
	for _, zf := range zr.File {
		name := zf.Name
		if zf.FileInfo().IsDir() {
			continue
		}
		clean, ok := safeJoin(root, name)
		if !ok {
			sess.warns = append(sess.warns, "zip entry rejected (escapes the archive root): "+name)
			continue
		}
		if err := writeZipEntry(zf, clean); err != nil {
			return &uploadError{http.StatusBadRequest, "extract zip: " + err.Error()}
		}
	}
	return nil
}

// safeJoin cleans name and refuses any path that would land outside root
// (zip-slip). ok=false means the entry was rejected.
func safeJoin(root, name string) (string, bool) {
	p := path.Clean("/" + filepath.ToSlash(name)) // forces absolute, collapses ..
	if p == "/" {
		return "", false
	}
	return filepath.Join(root, filepath.FromSlash(p)), true
}

func writeZipEntry(zf *zip.File, dest string) error {
	if dir := filepath.Dir(dest); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// wireFilename recovers the raw filename parameter of a multipart file
// part. net/http's FileHeader.Filename is base-stripped per RFC 7578
// §4.2 ("directory path information must not be used"), which destroys
// the directory-relative path browsers send for webkitdirectory uploads
// ("repo/.coding-hermes/board/tasks.jsonl" arrives as "tasks.jsonl") and
// flattens a folder upload into the extraction root. The raw
// Content-Disposition parameters survive on the part header, so recover
// the wire filename from there (DF-BOARDCTL-6: proven by an E2E multipart
// test posting a nested two-board folder on field "files" — pre-fix it
// registered 1 board instead of 2). Browsers use forward slashes on all
// platforms; safeJoin's FromSlash/slash handling below still applies.
func wireFilename(fh *multipart.FileHeader) string {
	for _, v := range fh.Header.Values("Content-Disposition") {
		_, params, err := mime.ParseMediaType(v)
		if err != nil {
			continue
		}
		if name := params["filename"]; name != "" {
			return name
		}
	}
	return fh.Filename
}

// extractFolder writes the parts of a webkitdirectory upload under root,
// preserving each file's relative path (browsers put webkitRelativePath in
// the multipart filename, e.g. "myrepo/.coding-hermes/board/tasks.jsonl").
func extractFolder(files []*multipart.FileHeader, root string, sess *uploadSession) *uploadError {
	for _, fh := range files {
		name := wireFilename(fh)
		// Some browsers include a leading chosen-directory component or
		// backslashes; normalize. Zip-slip guard applies equally here.
		clean, ok := safeJoin(root, name)
		if !ok {
			sess.warns = append(sess.warns, "file part rejected (escapes the upload root): "+name)
			continue
		}
		// Directories in the upload have no parts of their own, so create
		// every parent as the files demand it (mirrors writeZipEntry).
		if dir := filepath.Dir(clean); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return &uploadError{http.StatusBadRequest, "create dir for part " + name + ": " + err.Error()}
			}
		}
		src, err := fh.Open()
		if err != nil {
			return &uploadError{http.StatusBadRequest, "read part " + name + ": " + err.Error()}
		}
		dst, err := os.Create(clean)
		if err != nil {
			src.Close()
			return &uploadError{http.StatusBadRequest, "write part " + name + ": " + err.Error()}
		}
		_, cpErr := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if cpErr != nil {
			return &uploadError{http.StatusBadRequest, "copy part " + name + ": " + cpErr.Error()}
		}
	}
	return nil
}

// resolveBoards registers EVERY directory at ANY depth under root that
// contains BOTH tasks.jsonl and events.jsonl as a board (BT-021 requirement
// 3). Boards are ordered by path for deterministic payload order.
func resolveBoards(root string, sess *uploadSession) []*board.Board {
	var dirs []string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if isBoardDir(p) {
			dirs = append(dirs, p)
			return filepath.SkipDir // nested board-in-board is one board
		}
		return nil
	})
	sort.Strings(dirs)
	var boards []*board.Board
	for _, d := range dirs {
		b, err := board.Resolve(d)
		if err != nil {
			continue // unreachable: isBoardDir pre-checked the pair
		}
		boards = append(boards, b)
	}
	for _, w := range sess.warns {
		fmt.Fprintf(os.Stderr, "boardctl serve: %s\n", w)
	}
	return boards
}

// isBoardDir reports whether dir contains BOTH required board files — the
// same pair rule as board.Resolve's candidates.
func isBoardDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "tasks.jsonl"))
	if err != nil || st.IsDir() {
		return false
	}
	st, err = os.Stat(filepath.Join(dir, "events.jsonl"))
	return err == nil && !st.IsDir()
}

// apiBoard is one entry of GET /api/boards.
type apiBoard struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Topology string `json:"topology"`
	Total    int    `json:"task_total"`
	Done     int    `json:"done"`
	Open     int    `json:"open"`
}

// handleAPIBoards answers with the currently loaded boards: name, slug,
// topology, task total, done/open counts (BT-021 requirement 4). Board
// totals come from the payload derivation so fixtures behave exactly as in
// the report (fixture exclusion, last-row-wins dedup).
func (s *serveServer) handleAPIBoards(w http.ResponseWriter, r *http.Request) {
	boards := s.currentBoards()
	payload, err := render.BuildBoards(boards, render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		http.Error(w, "build boards: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]apiBoard, 0, len(payload.Boards))
	for _, b := range payload.Boards {
		out = append(out, apiBoard{
			Name:     b.Name,
			Slug:     b.Slug,
			Topology: b.Topology,
			Total:    b.Derived.TotalNonFix,
			Done:     b.Derived.CompleteCount,
			Open:     b.Derived.OpenCount,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	jsonWrite(w, out)
}

func jsonWrite(w http.ResponseWriter, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "encode json: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}
