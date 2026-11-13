package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

// BT-098 §3.5: the `types` verb — a read-only listing of the board's
// configured ticket-type registry.
//
//	boardctl types [-C dir] [--json] [--doctor]
//
// For each active type: slug, display name, template summary (prefix /
// priority / skeleton present) and its metadata fields as
// name(type, required?, values…). Retired types are listed in a trailing
// `retired:` block. Exit 0. --doctor names the offending lines of a
// degraded registry (duplicate-slug conflicts degraded to last-line-wins —
// panel amendment 2); a CORRUPT row (unparseable) refuses the command.
func cmdTypes(dir string, args []string) error {
	fs := newFlagSet("types")
	args = reorderArgs(args, valueFlags("C"))
	asJSON := fs.Bool("json", false, "emit the registry as JSON")
	doctor := fs.Bool("doctor", false, "name the offending lines of a degraded registry (duplicate slugs degraded to last-line-wins) instead of the normal listing")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl types [-C dir] [--json] [--doctor]\n")
		_, _ = fmt.Fprintf(fs.Output(), "\n  list the configured ticket types and their required metadata fields\n")
		_, _ = fmt.Fprintf(fs.Output(), "  (types.jsonl; absent file = the built-ins task|bug|feat|chore).\n")
		_, _ = fmt.Fprintf(fs.Output(), "  --doctor names offending lines (duplicate-slug conflicts); a corrupt\n")
		_, _ = fmt.Fprintf(fs.Output(), "  row refuses with its line number.\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("types takes no positional args (got %q)", fs.Arg(0))
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	reg, err := b.LoadTypes()
	if err != nil {
		return fmt.Errorf("types refused: %v", err)
	}
	// Canon-drift warnings (unknown keys inside type rows) go to stderr —
	// the write is never refused for them, but the drift is named.
	for _, w := range reg.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	if *doctor {
		printTypesDoctor(reg)
		return nil
	}
	if *asJSON {
		out, err := json.MarshalIndent(typesRegistryJSON(reg), "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, string(out))
		return nil
	}
	_, _ = fmt.Fprint(os.Stdout, renderTypesText(reg))
	return nil
}

// typesRegistryJSON is the --json shape: active then retired, schema fields
// spelled out.
func typesRegistryJSON(reg *board.TypeRegistry) any {
	type fieldJSON struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		Required    bool     `json:"required"`
		Description string   `json:"description,omitempty"`
		Values      []string `json:"values,omitempty"`
		Ref         string   `json:"ref,omitempty"`
	}
	type typeJSON struct {
		Slug     string `json:"slug"`
		Display  string `json:"display"`
		Rev      int64  `json:"rev"`
		Retired  bool   `json:"retired"`
		Line     int    `json:"line"`
		Template *struct {
			TitlePrefix     string `json:"title_prefix,omitempty"`
			ACSkeleton      string `json:"ac_skeleton,omitempty"`
			DefaultPriority string `json:"default_priority,omitempty"`
		} `json:"template,omitempty"`
		Metadata []fieldJSON `json:"metadata"`
	}
	type registryJSON struct {
		Source    string     `json:"source"`
		Types     []typeJSON `json:"types"`
		Retired   []typeJSON `json:"retired"`
		Conflicts []string   `json:"conflicts,omitempty"`
		Warnings  []string   `json:"warnings,omitempty"`
		BuiltIn   bool       `json:"built_in_defaults"`
	}
	out := registryJSON{Source: reg.Source, BuiltIn: reg.Source == ""}
	render := func(t *board.TicketType) typeJSON {
		tj := typeJSON{
			Slug:    t.Slug,
			Display: t.Display,
			Rev:     t.Rev,
			Retired: t.Retired,
			Line:    t.Line,
		}
		if t.Template != nil {
			tj.Template = &struct {
				TitlePrefix     string `json:"title_prefix,omitempty"`
				ACSkeleton      string `json:"ac_skeleton,omitempty"`
				DefaultPriority string `json:"default_priority,omitempty"`
			}{
				TitlePrefix:     t.Template.TitlePrefix,
				ACSkeleton:      t.Template.ACSkeleton,
				DefaultPriority: t.Template.DefaultPriority,
			}
		}
		for _, f := range t.Metadata {
			tj.Metadata = append(tj.Metadata, fieldJSON{
				Name: f.Name, Type: f.Type, Required: f.Required,
				Description: f.Description, Values: f.Values, Ref: f.Ref,
			})
		}
		return tj
	}
	for _, t := range reg.ActiveSorted() {
		out.Types = append(out.Types, render(t))
	}
	for _, t := range reg.RetiredSorted() {
		out.Retired = append(out.Retired, render(t))
	}
	for _, c := range reg.Conflicts {
		out.Conflicts = append(out.Conflicts, c.Detail)
	}
	out.Warnings = reg.Warnings
	return out
}

// renderTypesText is the human listing (spec §3.5). Slugs sort alphabetically
// (deterministic for diffing; on built-ins the order is exactly the AC-2 set
// bug, chore, feat, task).
func renderTypesText(reg *board.TypeRegistry) string {
	var sb strings.Builder
	src := "built-in defaults (no types.jsonl)"
	if reg.Source != "" {
		src = reg.Source
	}
	fmt.Fprintf(&sb, "ticket types (%s):\n", src)
	active := reg.ActiveSorted()
	if len(active) == 0 {
		fmt.Fprintf(&sb, "  (none)\n")
	}
	for _, t := range active {
		fmt.Fprintf(&sb, "  %s (%s) [rev %d]\n", t.Slug, t.Display, t.Rev)
		if t.Template == nil {
			fmt.Fprintf(&sb, "    template: none\n")
		} else {
			parts := []string{}
			if t.Template.TitlePrefix != "" {
				parts = append(parts, fmt.Sprintf("title_prefix %q", t.Template.TitlePrefix))
			}
			if t.Template.DefaultPriority != "" {
				parts = append(parts, "default_priority "+t.Template.DefaultPriority)
			}
			if t.Template.ACSkeleton != "" {
				parts = append(parts, "ac_skeleton present")
			}
			if len(parts) == 0 {
				fmt.Fprintf(&sb, "    template: none\n")
			} else {
				fmt.Fprintf(&sb, "    template: %s\n", strings.Join(parts, " / "))
			}
		}
		if len(t.Metadata) == 0 {
			fmt.Fprintf(&sb, "    metadata: none\n")
		} else {
			fmt.Fprintf(&sb, "    metadata:\n")
			for _, f := range t.Metadata {
				req := ""
				if f.Required {
					req = ", required"
				}
				line := fmt.Sprintf("      - %s (%s%s)", f.Name, f.Constraint(), req)
				if f.Description != "" {
					line += " — " + f.Description
				}
				fmt.Fprintf(&sb, "%s\n", line)
			}
		}
	}
	if retired := reg.RetiredSorted(); len(retired) > 0 {
		fmt.Fprintf(&sb, "retired:\n")
		for _, t := range retired {
			fmt.Fprintf(&sb, "  %s (%s) [rev %d] — resolvable for existing rows; 'create --type %s' refuses it\n", t.Slug, t.Display, t.Rev, t.Slug)
		}
	}
	return sb.String()
}

// printTypesDoctor names the offending lines of a degraded registry (panel
// amendment 2's recovery verb). Corrupt rows never reach here — the loader
// refuses them with the line number.
func printTypesDoctor(reg *board.TypeRegistry) {
	if reg.Source == "" {
		_, _ = fmt.Fprintf(os.Stdout, "types doctor: built-in defaults in effect (no types.jsonl) — nothing to check\n")
		return
	}
	_, _ = fmt.Fprintf(os.Stdout, "types doctor: %s\n", reg.Source)
	if len(reg.Conflicts) == 0 {
		_, _ = fmt.Fprintf(os.Stdout, "  no conflicts: every slug is declared on exactly one winning line (last line wins per slug)\n")
		return
	}
	for _, c := range reg.Conflicts {
		_, _ = fmt.Fprintf(os.Stdout, "  line %d: %s\n", c.Line, c.Detail)
	}
	_, _ = fmt.Fprintf(os.Stdout, "  remedy: append a corrected declaration with a higher rev — never edit history\n")
}
