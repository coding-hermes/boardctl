// Command gendocs renders internal/describe's registry as the OpenAPI 3.0.3
// document muster consumes (BT-062), and writes it to the repo's published
// spec location.
//
// Generation rule (this repo's contract doctrine): the registry is the
// single source. The contract is ALWAYS generated from the live wiring in
// cmd/boardctl — never hand-edited; a hand-edit is overwritten by the next
// regeneration and the parity gate (internal/speccheck) fails a stale
// document anyway. Regenerate after every surface change with:
//
//	go run ./cmd/gendocs            # writes docs/muster/openapi.yaml
//	go test ./internal/speccheck    # proves the document matches the wiring
//
// Exit codes: 0 success, 1 derivation/write failure.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/coding-hermes/boardctl/internal/describe"
)

// defaultRoot resolves the module root from the working directory (walks up
// to go.mod), so the command works from anywhere inside the checkout.
func defaultRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found walking up from %s", dir)
		}
		dir = parent
	}
}

func run(args []string) int {
	root, err := defaultRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		return 1
	}
	reg, err := describe.Load(root, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		return 1
	}
	doc, err := describe.RenderOpenAPI(reg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		return 1
	}
	out := filepath.Join(root, "docs", "muster", "openapi.yaml")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		return 1
	}
	if err := os.WriteFile(out, []byte(doc), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gendocs: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "wrote %s (%d http routes from %s)\n", out, len(reg.HTTP), reg.Serve)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:]))
}
