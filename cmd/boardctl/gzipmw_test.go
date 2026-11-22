package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// staticBodyHandler returns a handler emitting the given body on all
// requests.
func staticBodyHandler(body []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
}

// gunzipBody decompresses a gzip Content-Encoding response body.
func gunzipBody(t *testing.T, body []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	if err := zr.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return out
}

// TestGzipWrapperAccepted: a gzip-capable request gets
// Content-Encoding: gzip + Vary: Accept-Encoding, no Content-Length,
// and a body that decompresses to the handler's bytes.
func TestGzipWrapperAccepted(t *testing.T) {
	payload := []byte(`{"hello":"world","repeat":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	h := withGzip(staticBodyHandler(payload))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/x", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", ce)
	}
	if vary := rec.Header().Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
		t.Errorf("Vary = %q, want Accept-Encoding", vary)
	}
	if _, present := rec.Header()["Content-Length"]; present {
		t.Errorf("Content-Length = %q, want dropped when compressed", rec.Header().Get("Content-Length"))
	}
	if got := gunzipBody(t, rec.Body.Bytes()); !bytes.Equal(got, payload) {
		t.Errorf("round-trip body = %q, want %q", got, payload)
	}
	// Compression must actually halve a repetitive payload.
	if rec.Body.Len() >= len(payload) {
		t.Errorf("gzip arm %d bytes >= payload %d bytes; no compression observed", rec.Body.Len(), len(payload))
	}
}

// TestGzipWrapperIdentityByteIdentical: no Accept-Encoding means the
// response passes through untouched — exact bytes, no encoding headers.
func TestGzipWrapperIdentityByteIdentical(t *testing.T) {
	payload := []byte(`{"identity":true}`)
	payload = append(payload, bytes.Repeat([]byte{0x00, 0xfe, 0x7f}, 33)...)
	h := withGzip(staticBodyHandler(payload))
	for _, hdr := range []string{"", "identity", "deflate"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		if hdr != "" {
			req.Header.Set("Accept-Encoding", hdr)
		}
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("header %q: status = %d", hdr, rec.Code)
		}
		if ce := rec.Header().Get("Content-Encoding"); ce != "" {
			t.Errorf("header %q: Content-Encoding = %q, want empty", hdr, ce)
		}
		if vary := rec.Header().Get("Vary"); vary != "" {
			t.Errorf("header %q: Vary = %q, want empty", hdr, vary)
		}
		if !bytes.Equal(rec.Body.Bytes(), payload) {
			t.Errorf("header %q: body not byte-identical to payload", hdr)
		}
	}
}

// TestGzipWrapperSkipsAlreadyEncoded: a response that carries its own
// Content-Encoding must never be double-compressed.
func TestGzipWrapperSkipsAlreadyEncoded(t *testing.T) {
	payload := []byte("pre-encoded-bytes-not-gzip")
	h := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		_, _ = w.Write(payload)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	h.ServeHTTP(rec, req)
	if ce := rec.Header().Get("Content-Encoding"); ce != "br" {
		t.Errorf("Content-Encoding = %q, want br untouched", ce)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Errorf("body was re-compressed: %q", rec.Body.Bytes())
	}
}

// TestGzipWrapperRedirectPassthrough: a 302 has no compressible body;
// it must keep its exact header semantics.
func TestGzipWrapperRedirectPassthrough(t *testing.T) {
	h := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty on redirect", ce)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/elsewhere") {
		t.Errorf("Location = %q", rec.Header().Get("Location"))
	}
}

// TestGzipWrapperContentLengthDroppedExplicit: a handler that pre-stamps
// Content-Length must not ship a stale length alongside gzip.
func TestGzipWrapperContentLengthDroppedExplicit(t *testing.T) {
	payload := []byte("0123456789")
	h := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		_, _ = w.Write(payload)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)
	if _, present := rec.Header()["Content-Length"]; present {
		t.Errorf("Content-Length = %q, want dropped on the gzip arm", rec.Header().Get("Content-Length"))
	}
	if got := gunzipBody(t, rec.Body.Bytes()); !bytes.Equal(got, payload) {
		t.Errorf("round trip = %q", got)
	}
}

// TestGzipWrapper204: 204 has no body; no gzip markers on it.
func TestGzipWrapper204(t *testing.T) {
	h := withGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Errorf("Content-Encoding = %q, want empty on 204", ce)
	}
}

// gzipMinCompressiblePayload is the minimum identity-body size below
// which the strict "gzip wire smaller than identity" assertion must be
// skipped. gzip output carries a FIXED framing cost on every response a
// tiny body cannot amortize: the 2-byte magic number and header, the
// 8-byte trailer (CRC32 + length), and ~10-30 bytes of flate overhead
// even for already-hard-to-compress JSON. For payloads below that
// floor the gzip wire can legitimately be LARGER than the identity
// body — observed on hosted arm64-native CI, where the /api/boards
// identity was ~144-149 bytes and gzip came out within one byte of it
// (and sometimes larger). The strict size invariant is therefore only
// meaningful once a body comfortably exceeds the framing cost; 512 is
// a conservative floor (framing is bounded well under 100 bytes for
// these payloads, so anything at or above 512 must compress or the
// middleware is not doing its job on large payloads).
const gzipMinCompressiblePayload = 512

// TestServeJSONEndpointsThroughGzip: the real serve surface (routes())
// serves /api/boards, /api/board/{slug} and /api/events/{slug} with
// gzip on the wire and valid JSON after decompression; the identity arm
// still returns byte-identical parseable JSON.
func TestServeJSONEndpointsThroughGzip(t *testing.T) {
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "gzboard"), "A")
	zipBytes := zipFromPaths(t, root, filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))

	srv := newServeServer(nil)
	h := srv.routes()
	if rec := postUpload(h, map[string][]byte{"zip": zipBytes}); rec.Code != 200 {
		t.Fatalf("setup upload status = %d: %s", rec.Code, rec.Body.String())
	}

	roundTrip := func(url string, acceptGzip bool) (*http.Response, []byte) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if acceptGzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		// Default Transport auto-decompresses; a fresh unshared
		// transport without CheckRedirect surprises, no auto gzip.
		client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body %s: %v", url, err)
		}
		return resp, body
	}

	httpSrv := httptest.NewServer(h)
	t.Cleanup(httpSrv.Close)

	type apiBody struct {
		Name string `json:"name"`
	}
	for _, url := range []string{"/api/boards", "/api/board/gzboard", "/api/events/gzboard"} {
		// Identity arm: untouched JSON.
		resp, body := roundTrip(httpSrv.URL+url, false)
		if ce := resp.Header.Get("Content-Encoding"); ce != "" {
			t.Errorf("%s identity: Content-Encoding = %q, want empty", url, ce)
		}
		identity := body
		if strings.HasSuffix(url, "boards") {
			var boards []apiBody
			if err := json.Unmarshal(identity, &boards); err != nil {
				t.Fatalf("%s identity json: %v", url, err)
			}
			if len(boards) != 1 || boards[0].Name != "gzboard" {
				t.Fatalf("%s identity boards = %+v", url, boards)
			}
		} else {
			var b apiBody
			if strings.Contains(url, "/api/events/") {
				// events endpoint shape: a JSON array.
				var events []map[string]any
				if err := json.Unmarshal(identity, &events); err != nil {
					t.Fatalf("%s identity json: %v", url, err)
				}
				if len(events) == 0 {
					t.Fatalf("%s identity: empty events", url)
				}
			} else {
				if err := json.Unmarshal(identity, &b); err != nil {
					t.Fatalf("%s identity json: %v", url, err)
				}
			}
		}
		// Gzip arm: headers set, body decompresses to the SAME bytes
		// and still parses.
		resp, wire := roundTrip(httpSrv.URL+url, true)
		if ce := resp.Header.Get("Content-Encoding"); ce != "gzip" {
			t.Errorf("%s gzip: Content-Encoding = %q, want gzip", url, ce)
		}
		if vary := resp.Header.Get("Vary"); !strings.Contains(vary, "Accept-Encoding") {
			t.Errorf("%s gzip: Vary = %q, want Accept-Encoding", url, vary)
		}
		decompressed := gunzipBody(t, wire)
		if !bytes.Equal(decompressed, identity) {
			t.Errorf("%s: decompressed gzip arm differs from identity arm", url)
		}
		// Compression size check, threshold-aware.
		//
		// Route chosen: keep seedServeBoard untouched and make the
		// assertion threshold-aware. seedServeBoard's shape is
		// load-bearing across a dozen other tests (island task
		// counts, board counts, dedupe ids), and /api/boards emits
		// a fixed-cost ~150-byte summary per board that no amount
		// of extra seeded task rows can push past a 512-byte
		// floor, so growing the seed could never make THIS
		// endpoint's strict assertion safe. Below the floor we
		// still prove real compression happens on the live serve
		// surface: the body must be valid gzip that decompresses
		// byte-for-byte to the identity arm (above), and the
		// /api/board/gzboard endpoint (identity ~5.6 KB, well
		// above the floor) keeps the strict wire-smaller-than-
		// identity invariant unconditionally.
		if len(identity) >= gzipMinCompressiblePayload && len(wire) >= len(identity) {
			t.Errorf("%s: gzip wire %d >= identity %d; no compression", url, len(wire), len(identity))
		}
	}
}

// TestServeUIThroughGzipLive: /ui through a live serve server serves the
// same island payload on both arms; gzip arm compresses it.
func TestServeUIThroughGzipLive(t *testing.T) {
	root := t.TempDir()
	dirA := seedServeBoard(t, filepath.Join(root, "gzboard"), "A")
	zipBytes := zipFromPaths(t, root, filepath.Join(dirA, "tasks.jsonl"), filepath.Join(dirA, "events.jsonl"))

	srv := newServeServer(nil)
	h := srv.routes()
	if rec := postUpload(h, map[string][]byte{"zip": zipBytes}); rec.Code != 200 {
		t.Fatalf("setup upload status = %d: %s", rec.Code, rec.Body.String())
	}
	httpSrv := httptest.NewServer(h)
	t.Cleanup(httpSrv.Close)

	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	get := func(acceptGzip bool) (*http.Response, []byte) {
		req, err := http.NewRequest("GET", httpSrv.URL+"/ui", nil)
		if err != nil {
			t.Fatal(err)
		}
		if acceptGzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}

	_, identityBody := get(false)
	resp, wire := get(true)
	if ce := resp.Header.Get("Content-Encoding"); ce != "gzip" {
		t.Errorf("/ui Content-Encoding = %q, want gzip", ce)
	}
	decompressed := gunzipBody(t, wire)
	if !bytes.Equal(decompressed, identityBody) {
		t.Fatalf("/ui: decompressed gzip arm differs from identity arm")
	}
	p := bt072Island(t, string(decompressed))
	if len(p.Boards) != 1 {
		t.Fatalf("/ui island boards = %d, want 1", len(p.Boards))
	}
	// Seed board A has 3 task rows; the island must carry them.
	if tasks := islandTaskCount(p.Boards); tasks != 3 {
		t.Errorf("/ui island tasks = %d, want 3", tasks)
	}
	// The /ui island payload is large (whole HTML document), so the
	// strict invariant always applies here; the floor is only there
	// for genuinely tiny bodies (see gzipMinCompressiblePayload).
	if len(identityBody) >= gzipMinCompressiblePayload && len(wire) >= len(identityBody) {
		t.Errorf("/ui gzip wire %d >= identity %d; no compression", len(wire), len(identityBody))
	}
}

// islandTaskCount counts task rows in an /ui island's boards.
func islandTaskCount(boards []uiBoard) int {
	tasks := 0
	for _, b := range boards {
		tasks += len(b.Tasks)
	}
	return tasks
}
