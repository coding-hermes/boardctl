package board

import (
	"strings"
	"testing"
)

func TestParseInitiativeRef(t *testing.T) {
	ref, err := ParseInitiativeRef("code-factory:internal/board#TaskFilter")
	if err != nil {
		t.Fatal(err)
	}
	if ref.String() != "code-factory:internal/board#TaskFilter" {
		t.Fatalf("round trip = %q", ref.String())
	}
	for _, tc := range []struct {
		name string
		ref  string
		part string
	}{
		{"repo", "BadRepo:internal/board#TaskFilter", "REPO"},
		{"path", "repo:/internal/board#TaskFilter", "PATH"},
		{"path traversal", "repo:internal/../board#TaskFilter", "PATH"},
		{"anchor", "repo:internal/board#bad anchor", "ANCHOR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateInitiativeRef(tc.ref)
			if err == nil || !strings.Contains(err.Error(), tc.part) {
				t.Fatalf("error = %v, want offending %s", err, tc.part)
			}
		})
	}
}

func TestInitiativeRefsFilterMatchesRepo(t *testing.T) {
	row, err := ParseRow([]byte(`{"id":"BT-092","status":"pending","initiative_refs":["repo-a:docs/spec.md#One","repo-b:internal/board#Two"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !InitiativeRefsMatchRepo(row, "repo-b") || InitiativeRefsMatchRepo(row, "repo-c") {
		t.Fatal("repo-only filter mismatch")
	}
}

// TestCreateWithoutRefFlagWritesNoKeyOnMirroredBoard pins the mirroring half of
// acceptance (1): once a board carries a ref-bearing row, every later create
// inherits that row's key ORDER as its schema template. The key must still be
// absent when the caller omitted --initiative-ref, otherwise "omitted writes no
// key" only holds for the first row of a fresh board.
func TestCreateWithoutRefFlagWritesNoKeyOnMirroredBoard(t *testing.T) {
	b := newTestBoard(t)
	if _, err := b.Create(TaskRowSpec{ID: "REF-1", Title: "with refs", InitiativeRefs: refsPtr("repo-a:docs/spec.md#One")}); err != nil {
		t.Fatalf("create with refs: %v", err)
	}
	if _, err := b.Create(TaskRowSpec{ID: "PLAIN-1", Title: "no refs"}); err != nil {
		t.Fatalf("create without refs: %v", err)
	}
	row, _, err := b.ShowTask("PLAIN-1")
	if err != nil {
		t.Fatalf("get row: %v", err)
	}
	if row == nil {
		t.Fatal("row PLAIN-1 not found")
	}
	if row.Has(InitiativeRefsKey) {
		t.Fatalf("omitted --initiative-ref still wrote the key: %s", row.Get(InitiativeRefsKey))
	}
	if refs := InitiativeRefRepos(row); len(refs) != 0 {
		t.Fatalf("refs = %v, want none", refs)
	}
}

func refsPtr(vals ...string) *[]string { return &vals }
