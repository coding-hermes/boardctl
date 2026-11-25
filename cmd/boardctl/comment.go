package main

// BT-099 — `comment`: append ONE typed comment event to events.jsonl.
//
// Doctrine (spec §1/§3): comments are TYPED APPEND-ONLY EVENTS, never row
// rewrites. tasks.jsonl is never touched, updated_at never moves, and the
// header ticks_total never bumps (the event carries no Tick, and the write
// path bumps only tick-bearing events — panel amendment 5). The audit trail
// IS the feature.

import (
	"fmt"
	"os"
	"strings"

	"github.com/coding-hermes/boardctl/internal/board"
)

// cmdComment implements `boardctl comment <ID> (--body TEXT | --body-file
// PATH) [--author NAME] [--reply-to EVENT-ID] [-C dir]`.
func cmdComment(dir string, args []string) error {
	fs := newFlagSet("comment")
	args = reorderArgs(args, valueFlags("C", "body", "body-file", "author", "reply-to"))
	body := fs.String("body", "", "comment text (mutually exclusive with --body-file)")
	bodyFile := fs.String("body-file", "", "read the comment text from a file (mutually exclusive with --body)")
	author := fs.String("author", "", "comment author (default foreman)")
	replyTo := fs.String("reply-to", "", "event id of the comment this one replies to (must be a comment event on the same task)")
	var cdir string
	addCFlag(fs, &cdir)
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "boardctl comment <ID> (--body \"text\" | --body-file <path>) [--author NAME] [--reply-to <event-id>] [-C dir]\n")
		_, _ = fmt.Fprintf(fs.Output(), "\n          appends ONE typed comment event to events.jsonl (the audit trail IS the feature):\n")
		_, _ = fmt.Fprintf(fs.Output(), "          tasks.jsonl is never touched, updated_at never moves, the header tick counter\n")
		_, _ = fmt.Fprintf(fs.Output(), "          never bumps. Exactly one of --body/--body-file is required; an empty body\n")
		_, _ = fmt.Fprintf(fs.Output(), "          after trim is a refusal. --reply-to must name an existing comment event on\n")
		_, _ = fmt.Fprintf(fs.Output(), "          the SAME task. Every refusal exits 1 and appends NOTHING.\n")
	}
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	if dir == "" {
		dir = cdir
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("comment requires exactly one task id")
	}
	// Exactly one of --body / --body-file: both (or neither) is a refusal
	// BEFORE the board is even opened, so nothing can be appended.
	haveBody := *body != ""
	haveFile := *bodyFile != ""
	if haveBody && haveFile {
		return fmt.Errorf("comment: --body and --body-file are mutually exclusive — exactly one is required")
	}
	if !haveBody && !haveFile {
		return fmt.Errorf("comment: exactly one of --body or --body-file is required (nothing appended)")
	}
	text := ""
	if haveFile {
		raw, err := os.ReadFile(*bodyFile)
		if err != nil {
			return fmt.Errorf("comment: --body-file: %w", err)
		}
		text = string(raw)
	} else {
		text = *body
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("comment: body is empty after trim — refusing to append an empty comment")
	}
	var inReplyTo *int64
	if *replyTo != "" {
		n, err := parseIntFlag("--reply-to", *replyTo)
		if err != nil {
			return fmt.Errorf("comment: --reply-to: %q is not an event id (a non-negative integer)", *replyTo)
		}
		inReplyTo = &n
	}
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	id, err := b.AddComment(fs.Arg(0), text, *author, inReplyTo)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "comment event id %d appended to %s\n", id, b.EventsPath())
	return nil
}

// commentEntry is one parsed comment event — the thread unit show renders
// and show --json emits (stable: ordered by event id). The shape lives in
// internal/board (one definition: the engine parses it, the CLI renders it).
type commentEntry = board.CommentEntry

// buildCommentThread parses a task's comment events (event_type "comment"),
// ordered by event id, and computes each entry's thread depth: a reply
// indents one level under its ancestor, and deeper chains keep indenting
// under their ancestor (depth = chain length - 1). A reply whose parent
// comment is missing from the thread (not a comment event, or on another
// task — unrepresentable here) renders at the top level rather than being
// dropped. Cycles in in_reply_to (hand-editable) are cut by the depth cap.
func buildCommentThread(events []*board.Row) []commentEntry {
	thread := board.CommentThread(events)
	out := make([]commentEntry, len(thread))
	copy(out, thread)
	return out
}

// renderCommentThread renders the show text block:
//
//	COMMENTS (2)
//	  #4179 2026-11-08T04:55 bane: split BT-098's validator first
//	    #4183 2026-11-08T05:20 foreman: agreed, filed as BT-098a
//
// Ordered by event id; every comment line indents one level under the row
// body, and a reply indents one level further under its ancestor. Empty
// thread renders nothing.
func renderCommentThread(thread []commentEntry) string {
	if len(thread) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "COMMENTS (%d)\n", len(thread))
	for _, c := range thread {
		fmt.Fprintf(&sb, "%s#%d %s %s: %s\n", strings.Repeat("  ", c.Depth+1), c.EventID, c.TS, c.Author, c.Body)
	}
	return sb.String()
}
