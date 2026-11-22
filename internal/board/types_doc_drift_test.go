package board

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadmeTicketTypeExample(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	const heading = "### Example types.jsonl\n"
	parts := strings.SplitN(string(readme), heading, 2)
	if len(parts) != 2 {
		t.Fatal("README types example heading missing")
	}
	section := strings.SplitN(parts[1], "\n## ", 2)[0]
	start := strings.Index(section, "```jsonl\n")
	if start < 0 {
		t.Fatal("README types example fence missing")
	}
	body := section[start+len("```jsonl\n"):]
	end := strings.Index(body, "\n```")
	if end < 0 {
		t.Fatal("README types example closing fence missing")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.jsonl"), []byte(body[:end]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	registry, err := LoadTicketTypes(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := TicketType{
		Slug: "release", Display: "Release", Rev: 1,
		Template: &TicketTemplate{TitlePrefix: "[RELEASE] ", DefaultPriority: "P1", ACSkeleton: "- [ ] Verify artifact"},
		Metadata: []MetadataField{
			{Name: "channel", Type: "enum", Required: true, Values: []string{"stable", "beta"}, Description: "Release channel"},
			{Name: "owner", Type: "string", Required: true},
		},
	}
	if !registry.Present || len(registry.Warnings) != 0 || len(registry.Types) != 1 || !reflect.DeepEqual(registry.Types["release"], want) {
		t.Fatalf("README example loaded as %#v, want %#v", registry, want)
	}
}
