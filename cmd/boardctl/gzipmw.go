package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPool recycles gzip writers across requests.
var gzipWriterPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// acceptsGzip reports whether the client asked for gzip in
// Accept-Encoding.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}

// directWriter is the gzip.Writer's UNDERLYING writer: it bypasses the
// gzipResponseWriter so the gzip stream's own header and compressor
// output go straight to the response (never re-entering gz.Write, which
// would recurse before flate is initialized).
type directWriter struct {
	w io.Writer
}

func (d *directWriter) Write(p []byte) (int, error) { return d.w.Write(p) }

// gzipResponseWriter wraps an http.ResponseWriter so handler writes are
// transparently gzip-compressed. The decision gzip-vs-identity is made
// at WriteHeader time, after the inner handler has set every header, so
// handlers that stamp their own Content-Encoding fall through
// untouched.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz         *gzip.Writer
	gzipOn     bool
	headerDone bool
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.headerDone {
		return
	}
	g.headerDone = true
	// Identity passthrough when no compression can apply: 204/304 have
	// no body, >=300 redirects carry Location only, and an
	// already-encoded response must never be double-compressed.
	if code == http.StatusNoContent || code == http.StatusNotModified ||
		code >= 300 || g.ResponseWriter.Header().Get("Content-Encoding") != "" {
		g.ResponseWriter.WriteHeader(code)
		return
	}
	h := g.Header()
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Del("Content-Length")
	g.gzipOn = true
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.headerDone {
		g.WriteHeader(http.StatusOK)
	}
	if !g.gzipOn {
		return g.ResponseWriter.Write(b)
	}
	return g.gz.Write(b)
}

// Flush pushes compressed bytes downstream when the underlying writer
// supports it (streaming responses).
func (g *gzipResponseWriter) Flush() {
	if g.gzipOn {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withGzip wraps a handler with gzip Content-Encoding when the request
// accepts it; identity (no Accept-Encoding) responses pass through
// byte-identical.
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer func() {
			gz.Reset(io.Discard)
			gzipWriterPool.Put(gz)
		}()
		// The compressed stream writes its gzip framing + flate
		// output directly to w; handler payload flows through gz.
		gz.Reset(&directWriter{w: w})
		g := &gzipResponseWriter{ResponseWriter: w, gz: gz}
		next.ServeHTTP(g, r)
		// Flush the final flate block ONLY when the gzip stream was
		// actually used — closing an untouched pooled writer would
		// append gzip framing to an identity response.
		if g.gzipOn {
			_ = gz.Close()
		}
	})
}
