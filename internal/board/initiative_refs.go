package board

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const InitiativeRefsKey = "initiative_refs"

var (
	initiativeRefRepoRE   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	initiativeRefAnchorRE = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)
)

// InitiativeRef is the canonical cross-repository location named by an
// initiative ref flag. Its string form is REPO:PATH#ANCHOR.
type InitiativeRef struct {
	Repo   string
	Path   string
	Anchor string
}

func (r InitiativeRef) String() string { return r.Repo + ":" + r.Path + "#" + r.Anchor }

// ParseInitiativeRef validates and parses one CLI ref. The separators are
// deliberately unambiguous: repo is before the first colon and anchor after
// the last hash, leaving colons legal in neither repo nor anchor boundaries.
func ParseInitiativeRef(raw string) (InitiativeRef, error) {
	var zero InitiativeRef
	i := strings.IndexByte(raw, ':')
	if i <= 0 {
		return zero, fmt.Errorf("initiative ref %q has invalid REPO %q (expected REPO:PATH#ANCHOR)", raw, raw)
	}
	h := strings.LastIndexByte(raw, '#')
	if h <= i+1 {
		return zero, fmt.Errorf("initiative ref %q has invalid PATH %q (expected REPO:PATH#ANCHOR)", raw, raw[i+1:])
	}
	repo, p, anchor := raw[:i], raw[i+1:h], raw[h+1:]
	if !initiativeRefRepoRE.MatchString(repo) {
		return zero, fmt.Errorf("initiative ref %q has invalid REPO %q (must match ^[a-z0-9][a-z0-9._-]*$)", raw, repo)
	}
	if strings.HasPrefix(p, "/") || path.IsAbs(p) {
		return zero, fmt.Errorf("initiative ref %q has invalid PATH %q (must be repo-relative with no leading slash)", raw, p)
	}
	for _, part := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if part == ".." {
			return zero, fmt.Errorf("initiative ref %q has invalid PATH %q (must not contain a .. segment)", raw, p)
		}
	}
	if p == "" || path.Clean(p) != p {
		return zero, fmt.Errorf("initiative ref %q has invalid PATH %q (must be a normalized repo-relative path)", raw, p)
	}
	if !initiativeRefAnchorRE.MatchString(anchor) {
		return zero, fmt.Errorf("initiative ref %q has invalid ANCHOR %q (must match ^[A-Za-z0-9._:-]+$)", raw, anchor)
	}
	return InitiativeRef{Repo: repo, Path: p, Anchor: anchor}, nil
}

func ValidateInitiativeRef(raw string) error {
	_, err := ParseInitiativeRef(raw)
	return err
}

func InitiativeRefRepoValid(repo string) bool { return initiativeRefRepoRE.MatchString(repo) }

func InitiativeRefStrings(refs []InitiativeRef) []string {
	out := make([]string, len(refs))
	for i, ref := range refs {
		out[i] = ref.String()
	}
	return out
}

func ParseInitiativeRefStrings(raw []string) ([]string, error) {
	out := make([]string, len(raw))
	for i, value := range raw {
		ref, err := ParseInitiativeRef(value)
		if err != nil {
			return nil, err
		}
		out[i] = ref.String()
	}
	return out, nil
}

func InitiativeRefRepos(row *Row) []string {
	raw := row.Get(InitiativeRefsKey)
	if raw == nil {
		return nil
	}
	var refs []string
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil
	}
	return refs
}

func InitiativeRefsMatchRepo(row *Row, repo string) bool {
	for _, raw := range InitiativeRefRepos(row) {
		ref, err := ParseInitiativeRef(raw)
		if err == nil && ref.Repo == repo {
			return true
		}
	}
	return false
}

func InitiativeRefsDisplay(row *Row) []string { return InitiativeRefRepos(row) }
