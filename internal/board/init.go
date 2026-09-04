package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// InitOptions carries the header identity for a freshly bootstrapped board.
type InitOptions struct {
	Project   string // written into the header; default: target dir basename
	Namespace string // default: project
}

// ErrAlreadyInitialized is returned by Init when the target board dir already
// carries all three topology-A files (tasks.jsonl, events.jsonl, and a
// board.jsonl with a header). Nothing was written.
var ErrAlreadyInitialized = errors.New("board already initialized")

// Init bootstraps a fresh topology-A board so `create` works on a brand-new
// project. The board dir is resolved with the same candidate order Resolve
// probes (the target itself, its .coding-hermes, then .coding-hermes/board —
// see boardDirCandidates); when no existing board is found the standard fresh
// location <target>/.coding-hermes/board is used, so a follow-up Resolve
// finds the board.
//
// Files written (idempotent, NO-CLOBBER):
//
//	tasks.jsonl   empty (0 bytes; the first task row is written by `create`)
//	events.jsonl  empty (0 bytes; append-only audit log)
//	board.jsonl   line-1 header: project, namespace, version, tick counters
//
// tasks.jsonl / events.jsonl are never touched when they already exist (they
// are SUPPOSED to be empty); an existing board.jsonl is seeded only when it
// is missing or 0 bytes. When nothing needs writing,
// ErrAlreadyInitialized is returned.
//
// Init writes board files ONLY: it never runs git, never commits, and never
// writes a header row into tasks.jsonl — a header on line 1 of tasks.jsonl is
// topology B, which is read-only. If tasks.jsonl's first row looks like a
// topology-B header, Init refuses rather than layering a fresh board.jsonl
// over it (which would silently flip the read-side topology).
//
// It returns the resolved board dir and the paths of the files it wrote.
func Init(dir string, opts InitOptions) (boardDir string, wrote []string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, err
	}
	if fi, statErr := os.Stat(abs); statErr == nil && !fi.IsDir() {
		return "", nil, fmt.Errorf("init target %s is not a directory", abs)
	}
	boardDir = defaultBoardDir(abs)
	if err := os.MkdirAll(boardDir, 0o755); err != nil {
		return "", nil, err
	}

	project := opts.Project
	if project == "" {
		project = defaultProjectName(abs)
	}
	namespace := opts.Namespace
	if namespace == "" {
		namespace = project
	}
	headerLine, err := marshalInitHeader(project, namespace)
	if err != nil {
		return "", nil, err
	}

	// Never layer a fresh board.jsonl over a topology-B board.
	if !headerHasContent(boardDir) {
		isHeader, err := tasksLine1IsHeader(filepath.Join(boardDir, "tasks.jsonl"))
		if err != nil {
			return boardDir, nil, err
		}
		if isHeader {
			return boardDir, nil, errors.New("tasks.jsonl line 1 looks like a topology-B header (metadata row without a task id) — topology B is read-only; migrate by splitting line 1 of tasks.jsonl into board.jsonl instead of running init")
		}
	}

	type seed struct{ name, content string }
	seeds := []seed{
		{"tasks.jsonl", ""},
		{"events.jsonl", ""},
		{"board.jsonl", headerLine},
	}
	for _, s := range seeds {
		p := filepath.Join(boardDir, s.name)
		if fileExists(p) {
			if s.name != "board.jsonl" {
				continue // tasks/events: existence = initialized (never clobber)
			}
			content, readErr := os.ReadFile(p)
			if readErr != nil {
				return boardDir, nil, readErr
			}
			if len(bytes.TrimSpace(content)) > 0 {
				continue // header with content: no-clobber
			}
		}
		if err := os.WriteFile(p, []byte(s.content), 0o644); err != nil {
			return boardDir, nil, err
		}
		wrote = append(wrote, p)
	}
	if len(wrote) == 0 {
		return boardDir, nil, ErrAlreadyInitialized
	}
	return boardDir, wrote, nil
}

// defaultBoardDir mirrors Resolve's candidate probing: an existing
// tasks.jsonl+events.jsonl pair wins (the init target itself, its
// .coding-hermes, then .coding-hermes/board); otherwise the fresh board goes
// to the STANDARD fleet location <target>/.coding-hermes/board so a follow-up
// Resolve finds it. Targets that already carry the .coding-hermes or board
// component are normalized (not doubled): init -C <repo>/.coding-hermes seeds
// <repo>/.coding-hermes/board, init -C <repo>/.coding-hermes/board seeds the
// board dir itself.
func defaultBoardDir(abs string) string {
	cands := boardDirCandidates(abs)
	for _, c := range cands {
		if fileExists(filepath.Join(c, "tasks.jsonl")) && fileExists(filepath.Join(c, "events.jsonl")) {
			return c
		}
	}
	switch filepath.Base(abs) {
	case ".coding-hermes":
		return filepath.Join(abs, "board")
	case "board":
		return abs
	}
	return cands[len(cands)-1]
}

// headerHasContent reports whether board.jsonl already exists with content in
// the board dir.
func headerHasContent(boardDir string) bool {
	p := filepath.Join(boardDir, "board.jsonl")
	if !fileExists(p) {
		return false
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(content)) > 0
}

// tasksLine1IsHeader reports whether tasks.jsonl's first non-empty line parses
// as a topology-B header row (board metadata — project/namespace/version/
// ticks_total — without a task id). A missing/empty tasks.jsonl reports false
// with no error; a parse failure on line 1 IS an error (the caller would
// otherwise seed a board on top of a corrupt file).
func tasksLine1IsHeader(path string) (bool, error) {
	if !fileExists(path) {
		return false, nil
	}
	lines, err := ReadJSONLLines(path)
	if err != nil {
		return false, err
	}
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		row, err := ParseRow(l)
		if err != nil {
			return false, fmt.Errorf("tasks.jsonl line 1: %w", err)
		}
		return row.String("id") == "" &&
			(row.Has("project") || row.Has("namespace") || row.Has("version") || row.Has("ticks_total")), nil
	}
	return false, nil
}

// defaultProjectName derives the header project default from the init target:
// the target directory's basename, skipping board/.coding-hermes path
// components (a board nested under .coding-hermes/board is still named after
// the project dir, matching the fleet's own header).
func defaultProjectName(abs string) string {
	d := abs
	for {
		base := filepath.Base(d)
		if base != "board" && base != ".coding-hermes" && base != "." && base != string(filepath.Separator) {
			return base
		}
		parent := filepath.Dir(d)
		if parent == d {
			return base
		}
		d = parent
	}
}

// marshalInitHeader renders the line-1 board.jsonl header for a fresh board.
// Compact separators match the fleet's real board.jsonl shape; last_commit
// stays null until the first `header --set-last-commit`; cooldown_s 21600 is
// the fleet default (6h).
func marshalInitHeader(project, namespace string) (string, error) {
	row := &Row{Vals: map[string]json.RawMessage{}}
	style := Style{} // compact, no ASCII escapes — matches the fleet's board.jsonl
	keys := []string{"project", "namespace", "version", "ticks_total", "ticks_idle", "cooldown_s", "last_commit"}
	values := []any{project, namespace, int64(1), int64(0), int64(0), int64(21600), nil}
	for i, k := range keys {
		if err := row.SetGoValue(k, values[i], style); err != nil {
			return "", err
		}
	}
	return string(append(row.Marshal(style), '\n')), nil
}
