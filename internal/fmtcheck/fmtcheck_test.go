package fmtcheck

import (
	"os"
	"strings"
	"testing"
)

// TestGofmt is the in-process gofmt gate. It runs under plain
// `go test ./...` and, by design, under `-short` too (CI's Test step uses
// -short): there is deliberately no testing.Short() skip here, because a
// formatting gate that skips in short mode would be green everywhere that
// matters. The command `make fmt-check` and the CI "Gofmt" step run this
// exact test; see fmtcheck.go for how the three surfaces stay in agreement.
func TestGofmt(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := RepoRoot(cwd)
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	unformatted, err := FindUnformatted(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(unformatted) > 0 {
		t.Errorf("found %d file(s) not gofmt-clean — run `gofmt -w` (or `make fmt`) on:\n  %s",
			len(unformatted), strings.Join(unformatted, "\n  "))
	}
}
