package main

// DF-BOARDCTL-30: the serve pages (GET /ui and the GET / uploader) answered
// /favicon.ico with a 404 on every page view — the pages' heads carry no
// icon, so browsers probe for one. Fix posture: each head carries exactly
// one inline data: SVG icon link, so the browser never requests
// /favicon.ico. R1's closed route set is untouched (no /favicon.ico route)
// and R17's no-external-anything posture is kept: the icon is inline, not
// a fetched asset.

import (
	"encoding/base64"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// df30IconLinkRe extracts <link rel="icon" ...> tags from an HTML head.
var df30IconLinkRe = regexp.MustCompile(`<link rel="icon"[^>]*>`)

// df30DataHRefRe captures the href="data:image/svg+xml;base64,..." value of
// an inline icon link.
var df30DataHRefRe = regexp.MustCompile(`href="data:image/svg\+xml;base64,([A-Za-z0-9+/=]+)"`)

// df30Head returns the head section of an HTML document body.
func df30Head(t *testing.T, body, page string) string {
	t.Helper()
	i := strings.Index(body, "</head>")
	if i < 0 {
		t.Fatalf("%s document has no </head>", page)
	}
	return body[:i]
}

// TestDF30UIIconLink asserts the /ui head carries exactly one icon link,
// that it is an inline data: SVG (no external fetch — R17), and that its
// base64 payload decodes to a well-formed SVG document.
func TestDF30UIIconLink(t *testing.T) {
	h := bt072Server(t).routes()
	body := bt072UIBody(t, h)
	head := df30Head(t, body, "/ui")

	links := df30IconLinkRe.FindAllString(head, -1)
	if len(links) != 1 {
		t.Fatalf("/ui <link rel=\"icon\"> count = %d, want exactly 1", len(links))
	}
	m := df30DataHRefRe.FindStringSubmatch(links[0])
	if m == nil {
		t.Fatalf("/ui icon link is not an inline data:image/svg+xml link: %q", links[0])
	}
	raw, err := base64.StdEncoding.DecodeString(m[1])
	if err != nil {
		t.Fatalf("/ui icon data URL does not decode as base64: %v", err)
	}
	s := string(raw)
	if !strings.HasPrefix(s, "<svg") || !strings.Contains(s, "</svg>") {
		t.Fatalf("/ui icon payload is not an SVG document: %q", s)
	}
}

// TestDF30UploaderIconLink asserts the same for the GET / uploader page
// (the serve landing page 404s /favicon.ico exactly like /ui did).
func TestDF30UploaderIconLink(t *testing.T) {
	h := bt072Server(t).routes()
	rec := bt072Get(h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200", rec.Code)
	}
	head := df30Head(t, rec.Body.String(), "/")
	links := df30IconLinkRe.FindAllString(head, -1)
	if len(links) != 1 {
		t.Fatalf("/ <link rel=\"icon\"> count = %d, want exactly 1", len(links))
	}
	if m := df30DataHRefRe.FindStringSubmatch(links[0]); m == nil {
		t.Fatalf("/ icon link is not an inline data:image/svg+xml link: %q", links[0])
	}
}
