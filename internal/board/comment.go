package board

// BT-099 — the comment append path. Lives beside AppendEvent's contract: ONE
// typed event row, id = MAX(existing)+1, tasks.jsonl untouched, no tick bump
// (Tick stays nil — a comment is human chatter, not tick work; panel
// amendment 5). Refusals append NOTHING and consume no event id (MAX+1
// recomputes on the next append).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// AddComment appends one comment event for taskID through the existing
// AppendEvent write path. Refusals (unknown task id, empty body after trim,
// --reply-to not an existing comment event on the same task) return an error
// with nothing appended. author defaults to "foreman" (AppendEvent's default).
func (b *Board) AddComment(taskID, body, author string, inReplyTo *int64) (int64, error) {
	if taskID == "" {
		return 0, fmt.Errorf("comment: task id is required")
	}
	// The comment must land on a real row: the same lookup `show` uses, so a
	// fixture row is commentable like a tasks.jsonl row.
	row, _, err := b.ShowTask(taskID)
	if err != nil {
		return 0, err
	}
	if row == nil {
		return 0, fmt.Errorf("comment refused: task %q not found (searched tasks.jsonl and fixtures.jsonl); nothing appended", taskID)
	}
	if len(bytes.TrimSpace([]byte(body))) == 0 {
		return 0, fmt.Errorf("comment refused: body is empty after trim — nothing appended")
	}
	if inReplyTo != nil {
		events, err := b.EventsForTask(taskID)
		if err != nil {
			return 0, err
		}
		found := false
		for _, e := range events {
			if id, ok := e.Int("id"); ok && id == *inReplyTo {
				if e.String("event_type") == "comment" {
					found = true
					break
				}
				return 0, fmt.Errorf("comment refused: --reply-to %d is event_type %q, not a comment — nothing appended", *inReplyTo, e.String("event_type"))
			}
		}
		if !found {
			return 0, fmt.Errorf("comment refused: --reply-to %d is not an existing comment event on task %q — nothing appended", *inReplyTo, taskID)
		}
	}
	detail := commentEventDetail{Body: body}
	if inReplyTo != nil {
		v := *inReplyTo
		detail.InReplyTo = &v
	}
	enc, err := json.Marshal(detail)
	if err != nil {
		return 0, err
	}
	return b.AppendEvent(EventSpec{
		Type:   "comment",
		TaskID: taskID,
		Actor:  author,
		Detail: enc,
	})
}

// commentEventDetail is the event detail's exact shape: {body, in_reply_to?}
// — in_reply_to present only when replying (schema §3).
type commentEventDetail struct {
	Body      string `json:"body"`
	InReplyTo *int64 `json:"in_reply_to,omitempty"`
}

// CommentEntry is one parsed comment event — the thread unit `show` renders
// and `show --json` emits (stable: ordered by event id, replies under their
// ancestors). Depth is the render indent level (0 = top-level); it is not
// part of the JSON shape.
type CommentEntry struct {
	EventID   int64  `json:"event_id"`
	TS        string `json:"ts"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	InReplyTo *int64 `json:"in_reply_to"`
	Depth     int    `json:"-"`
}

// CommentThread parses one task's comment events (event_type "comment"),
// ordered by event id, and computes each entry's thread depth: a reply
// indents one level under its ancestor, and deeper chains keep indenting
// under their ancestor (depth = chain length - 1). A reply whose parent
// comment is missing from the thread (not a comment event, or on another
// task) renders at the top level rather than being dropped. Cycles in
// in_reply_to (hand-editable) are cut by the depth cap.
func CommentThread(events []*Row) []CommentEntry {
	type parsed struct {
		entry CommentEntry
		reply *int64
	}
	var comments []parsed
	for _, e := range events {
		if e.String("event_type") != "comment" {
			continue
		}
		id, ok := e.Int("id")
		if !ok {
			continue
		}
		entry := CommentEntry{
			EventID: id,
			TS:      normalizeCommentTS(e.String("timestamp")),
			Author:  e.String("actor"),
		}
		var detail commentEventDetail
		if raw := e.Get("detail"); len(raw) > 0 {
			// detail is a JSON object STRING; decode the string, then the
			// object. A legacy free-form detail degrades to empty body.
			var inner string
			if err := json.Unmarshal(raw, &inner); err == nil && strings.TrimSpace(inner) != "" {
				_ = json.Unmarshal([]byte(inner), &detail)
			} else if raw[0] == '{' {
				_ = json.Unmarshal(raw, &detail)
			}
		}
		if entry.Author == "" {
			entry.Author = "foreman"
		}
		entry.Body = detail.Body
		entry.InReplyTo = detail.InReplyTo
		comments = append(comments, parsed{entry: entry, reply: detail.InReplyTo})
	}
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].entry.EventID < comments[j].entry.EventID })
	byID := make(map[int64]int, len(comments))
	for i, c := range comments {
		byID[c.entry.EventID] = i
	}
	const maxDepth = 64 // in_reply_to cycle cut (hand-edited events)
	depths := make([]int, len(comments))
	resolved := make([]bool, len(comments))
	var depth func(i int) int
	depth = func(i int) int {
		if resolved[i] {
			return depths[i]
		}
		resolved[i] = true // guard against in_reply_to cycles while walking
		d := 0
		if c := comments[i]; c.reply != nil {
			if parentIdx, ok := byID[*c.reply]; ok && parentIdx != i {
				d = depth(parentIdx) + 1
				if d > maxDepth {
					d = maxDepth
				}
			}
		}
		depths[i] = d
		return d
	}
	out := make([]CommentEntry, 0, len(comments))
	for i, c := range comments {
		c.entry.Depth = depth(i)
		out = append(out, c.entry)
	}
	return out
}

// normalizeCommentTS renders an event timestamp as the thread's display form
// "YYYY-MM-DDTHH:MM" (the spec's "2026-11-08T04:55"). Anything that does not
// start with a date stamp renders raw — never fabricated.
func normalizeCommentTS(ts string) string {
	ts = strings.TrimSpace(ts)
	if len(ts) < 16 || ts[4] != '-' || ts[7] != '-' || (ts[10] != ' ' && ts[10] != 'T') || ts[13] != ':' {
		return ts
	}
	return strings.Replace(ts[:16], " ", "T", 1)
}
