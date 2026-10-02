package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coding-hermes/boardctl/internal/board"
	"github.com/coding-hermes/boardctl/internal/render"
)

// ---------- BT-075: the boardctl web UI ----------
//
// docs/web-ui-spec.md (BT-072, DESIGN AUTHORITY) + docs/web-ui-design.md
// (BT-073) + docs/web-ui-ux.md (BT-074). D1: the UI is part of `boardctl
// serve` — the same process, the same listener; the route set STAYS CLOSED
// (R1): GET /, POST /, GET /api/boards (the pre-existing surface, R2) plus
// exactly three new routes: GET /ui, GET /api/board/{slug},
// GET /api/events/{slug}. Anything else 404s via mux routing.
//
// Posture inherited wholesale (BT-021): loopback-only bind (R3), no auth,
// no TLS, no cookies (R4), provide-dont-force — nothing outside serve
// references the UI (R5). Read-only: zero write endpoints (D3, R20); board
// files are never written; validate and sweep-status are surfaced as
// findings/census only, their write arms stay CLI-only.
//
// Read model (R6/R7/R9): every number the UI shows is derived by Go with
// the same render.BuildBoards machinery the report uses. A board's
// snapshot (payload + validate findings + sweep census + loaded_at) is
// computed once per load and cached by board identity: files changing on
// disk afterwards never change what /api/board/{slug}, /api/events/{slug}
// or /ui show until a new upload or a serve restart.

// uiSizeGuardrail is the R19 bound (analytics 6.3): the /ui document for a
// 300-task/2000-event board should stay under ~5 MB (page + embedded
// payloads combined). Exceeding it is not an error; uiNoteIfOver logs a
// note to serve stderr, exactly like parse warnings today.
const uiSizeGuardrail = 5 << 20 // 5 MB

// uiNoteIfOver logs the R19 note when size exceeds the guardrail. Factored
// as a pure predicate + side effect so the size test can drive both arms.
func uiNoteIfOver(size int) bool {
	if size <= uiSizeGuardrail {
		return false
	}
	fmt.Fprintf(os.Stderr, "boardctl serve: /ui payload is %.1f MB, over the ~5 MB size guardrail (analytics 6.3) — not an error\n", float64(size)/(1<<20))
	return true
}

// boardSnapshot is one board's read model: the render payload for that
// board, its validate findings, the sweep-status READ census, and the
// loaded_at stamp (R9/R13). Computed once per load; immutable afterwards.
type boardSnapshot struct {
	payload  render.BoardPayload
	validate *board.Report
	sweep    *board.StatusSweepReport
	loadedAt time.Time
}

// computeSnapshot derives the full read model for one board from the
// in-memory board (its loader reads the tracked files once; everything
// below is pure computation on those rows). Validate and the sweep run in
// their READ modes — neither touches a file (D3/R20).
func computeSnapshot(b *board.Board, now time.Time) *boardSnapshot {
	snap := &boardSnapshot{loadedAt: now}
	payload, err := render.BuildBoards([]*board.Board{b}, render.Options{Now: now, Zone: time.Local})
	if err == nil && len(payload.Boards) > 0 {
		snap.payload = payload.Boards[0]
	}
	if rep, err := b.Validate(); err == nil {
		snap.validate = rep
	}
	// apply=false: the sweep computes and reports only (dry-run by default;
	// --apply/--decide remain CLI-only, NG5).
	if sw, err := b.StatusSweep(false); err == nil {
		snap.sweep = sw
	}
	return snap
}

// snapshotKey is the cache key: the DF-BOARDCTL-2 board identity (report
// slug + topology). A re-upload of the same identity REPLACES the board and
// gets a fresh snapshot; boards sharing a slug but differing in topology
// stay distinct cache entries (R10).
func snapshotKey(b *board.Board) string { return serveBoardIdentity(b) }

// snapshotFor returns the cached snapshot for b, computing and caching it
// on first use (lazy fallback; the eager seeds run at -C resolve and after
// each upload). Cached-by-identity keeps R9: one computation per load.
func (s *serveServer) snapshotFor(b *board.Board) *boardSnapshot {
	key := snapshotKey(b)
	s.lock()
	snap := s.snapshots[key]
	s.unlock()
	if snap != nil {
		return snap
	}
	snap = computeSnapshot(b, time.Now())
	s.lock()
	if s.snapshots == nil {
		s.snapshots = map[string]*boardSnapshot{}
	}
	s.snapshots[key] = snap
	s.unlock()
	return snap
}

// seedSnapshots eagerly computes the snapshots of the given boards (called
// for the -C resolve at startup and for each upload's boards at
// registration — R13: findings are computed once per snapshot load).
func (s *serveServer) seedSnapshots(boards []*board.Board) {
	for _, b := range boards {
		if b != nil {
			s.snapshotFor(b)
		}
	}
}

// snapshotForSlug resolves the live board whose report slug is slug (first
// match in current-board order — the report's own order; cross-topology
// slug twins are distinct identities, R10) and returns its snapshot.
func (s *serveServer) snapshotForSlug(slug string) *boardSnapshot {
	for _, b := range s.currentBoards() {
		if serveBoardSlug(b) == slug {
			return s.snapshotFor(b)
		}
	}
	return nil
}

// serveBoardSlug derives the report slug of a board the way render does
// (slugify of the header project, else the dir base name) — the API identity
// key of R7. Mirrors serveBoardIdentity's name ladder + slugifyServe.
func serveBoardSlug(b *board.Board) string {
	name := ""
	if h := boardHeaderRow(b); h != nil {
		name = strings.TrimSpace(h.String("project"))
	}
	if name == "" {
		name = filepath.Base(b.Dir)
	}
	return slugifyServe(name)
}

// ---------- GET /ui ----------

// handleUI serves the self-contained UI document (R1): shell + inline
// CSS/JS + the initial snapshot island (R18 — the floor that renders the
// overview even when /api fetches fail). R16: the island is serialized by
// render.EscapeJSONIsland (encoding/json + HTML escaping), so `</script>`
// cannot occur inside it.
func (s *serveServer) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ui" {
		http.NotFound(w, r)
		return
	}
	island := s.uiIsland()
	page := strings.NewReplacer(
		"@@ISLAND@@", island,
		"@@VERSION@@", version, // R11: the version string, footer only
	).Replace(uiPage)
	uiNoteIfOver(len(page)) // R19: over ~5 MB is not an error; log a note
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, page)
}

// uiIsland builds the R18 island payload: every currently loaded board's
// full report payload plus its snapshot-time validation and sweep census
// (R13: findings ride the read model, computed once per load).
func (s *serveServer) uiIsland() string {
	payload := s.uiPayload()
	island, err := render.EscapeJSONIsland(payload)
	if err != nil {
		// Unserializable payloads are an infrastructure failure; the island
		// degrades to an honest empty fleet rather than a broken document.
		return `{"schema":"` + render.SchemaName + `","rendered_at":"","generated_by":"boardctl serve /ui","boards":[]}`
	}
	return string(island)
}

// uiPayload assembles the island payload. Index alignment: BuildBoards
// preserves board order and currentBoards never yields nils, so
// payload.Boards[i] corresponds to boards[i].
func (s *serveServer) uiPayload() uiPayloadShape {
	out := uiPayloadShape{
		Schema:      render.SchemaName,
		RenderedAt:  time.Now().Format(time.RFC3339),
		GeneratedBy: "boardctl serve /ui (board-report/v1)",
		Boards:      []uiBoard{},
	}
	boards := s.currentBoards()
	payload, err := render.BuildBoards(boards, render.Options{Now: time.Now(), Zone: time.Local})
	if err != nil {
		return out // honest empty fleet; the page shows its empty state
	}
	for i := range payload.Boards {
		ub := uiBoard{
			BoardPayload: payload.Boards[i],
			Validate:     uiValidate{Pill: "PASS"},
		}
		if i < len(boards) {
			if snap := s.snapshotFor(boards[i]); snap != nil {
				ub.Validate = uiValidateFromSnapshot(snap)
				ub.LoadedAt = snap.loadedAt.Format(time.RFC3339)
			}
		}
		out.Boards = append(out.Boards, ub)
	}
	return out
}

// uiPayloadShape is the island's top-level shape: board-report/v1's fields
// plus the UI refresh stamp.
type uiPayloadShape struct {
	Schema      string    `json:"schema"`
	RenderedAt  string    `json:"rendered_at"`
	GeneratedBy string    `json:"generated_by"`
	Boards      []uiBoard `json:"boards"`
}

// uiBoard is one board in the /ui island: the full report BoardPayload
// (R18 — the same payload /api/board/{slug} serves at load time) plus the
// BT-075 additive validation block and loaded_at.
type uiBoard struct {
	render.BoardPayload
	Validate uiValidate `json:"validate"`
	LoadedAt string     `json:"loaded_at"`
}

// uiValidate is the R13 validation block: the pill, findings, the verbatim
// key-uniformity census line, and the sweep-status READ census.
type uiValidate struct {
	Pill     string        `json:"pill"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
	Findings []uiFinding   `json:"findings"`
	Census   string        `json:"census"`
	Sweep    uiSweepCensus `json:"sweep_census"`
}

// uiFinding is one validate finding, verbatim (level + message with line
// numbers as the CLI prints them, F9).
type uiFinding struct {
	Level string `json:"level"`
	Msg   string `json:"message"`
}

// uiSweepCensus is the REVIEW-BOARDCTL-001 read census: off-vocabulary
// counts, unknown statuses NAMED with their row ids, and the explicit
// no-decisions line (never whitespace where the statement belongs).
type uiSweepCensus struct {
	Rows          int      `json:"rows"`
	OffVocabulary int      `json:"off_vocabulary"`
	Fixable       int      `json:"fixable_by_normalize"`
	Explicit      int      `json:"need_explicit_decision"`
	UnknownRows   []string `json:"unknown_rows"`
	DecisionsNote string   `json:"decisions_note"`
}

// uiValidateFromSnapshot maps the snapshot-time validate report + sweep
// census into the wire block. Pill semantics (R13): PASS = 0 errors
// (warnings allowed — validate's exit-0-with-warnings shape), WARN =
// warnings only / 0 errors, FAIL = >=1 error.
func uiValidateFromSnapshot(snap *boardSnapshot) uiValidate {
	uv := uiValidate{Pill: "PASS"}
	if snap.validate != nil {
		uv.Errors = snap.validate.Errors()
		uv.Warnings = len(snap.validate.Findings) - uv.Errors
		uv.Census = snap.validate.Keys.Summary()
		for _, f := range snap.validate.Findings {
			uv.Findings = append(uv.Findings, uiFinding{Level: f.Level, Msg: f.Msg})
		}
	}
	if uv.Errors > 0 {
		uv.Pill = "FAIL"
	} else if uv.Warnings > 0 {
		uv.Pill = "WARN"
	}
	if snap.sweep != nil {
		uv.Sweep = uiSweepCensus{
			Rows:          snap.sweep.Rows,
			OffVocabulary: snap.sweep.OffBefore,
			Fixable:       len(snap.sweep.Normalized),
			Explicit:      len(snap.sweep.Explicit),
			DecisionsNote: "no decisions applied — CLI-only",
		}
		seen := map[string]bool{}
		for _, r := range snap.sweep.Explicit {
			if !seen[r.ID] {
				seen[r.ID] = true
				uv.Sweep.UnknownRows = append(uv.Sweep.UnknownRows, r.ID)
			}
		}
		sort.Strings(uv.Sweep.UnknownRows)
	}
	return uv
}

// ---------- GET /api/board/{slug} ----------

// apiBoardDetail is one board's full read model (R7): header + task rows +
// events + validation + derived fields + loaded_at, values equal to what
// BuildBoards produces for that snapshot (R6). Served from the cached
// snapshot (R9); encoding/json's HTML escaping (jsonWrite) keeps the
// response inert (R16).
type apiBoardDetail struct {
	Name          string               `json:"name"`
	Slug          string               `json:"slug"`
	Topology      string               `json:"topology"`
	Header        render.BoardHeader   `json:"header"`
	Tasks         []json.RawMessage    `json:"tasks"`
	Events        []json.RawMessage    `json:"events"`
	FixtureIDs    []string             `json:"fixture_ids"`
	ParseWarnings render.ParseWarnings `json:"parse_warnings"`
	Derived       render.Derived       `json:"derived"`
	Validate      uiValidate           `json:"validate"`
	LoadedAt      string               `json:"loaded_at"`
}

func (s *serveServer) handleAPIBoard(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	snap := s.snapshotForSlug(slug)
	if snap == nil {
		jsonNotFound(w, slug)
		return
	}
	p := snap.payload
	w.Header().Set("Content-Type", "application/json")
	jsonWrite(w, apiBoardDetail{
		Name:          p.Name,
		Slug:          p.Slug,
		Topology:      p.Topology,
		Header:        p.Header,
		Tasks:         p.Tasks,
		Events:        p.Events,
		FixtureIDs:    p.FixtureIDs,
		ParseWarnings: p.ParseWarnings,
		Derived:       p.Derived,
		Validate:      uiValidateFromSnapshot(snap),
		LoadedAt:      snap.loadedAt.Format(time.RFC3339),
	})
}

// ---------- GET /api/events/{slug} ----------

// apiEvent is one event row of the timeline with the detail ladder already
// applied (R7, analytics 2.4): detail is the DECODED value — object/array
// as stored, JSON string parsed, strict base64-wrapped JSON decoded, raw
// string only on fall-through. Decoding happens in Go (R6: no client-side
// derivations); hostile strings stay inert through response escaping and
// the UI's textContent-only rendering (R16).
type apiEvent struct {
	ID         any    `json:"id"`
	Timestamp  string `json:"timestamp"`
	EventType  string `json:"event_type"`
	TaskID     any    `json:"task_id"`
	Actor      string `json:"actor"`
	Detail     any    `json:"detail"`
	TickNumber any    `json:"tick_number"`
}

func (s *serveServer) handleAPIEvents(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	snap := s.snapshotForSlug(slug)
	if snap == nil {
		jsonNotFound(w, slug)
		return
	}
	out := make([]apiEvent, 0, len(snap.payload.Events))
	for _, raw := range snap.payload.Events {
		var row struct {
			ID         any             `json:"id"`
			Timestamp  string          `json:"timestamp"`
			EventType  string          `json:"event_type"`
			TaskID     any             `json:"task_id"`
			Actor      string          `json:"actor"`
			Detail     json.RawMessage `json:"detail"`
			TickNumber any             `json:"tick_number"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			continue // a row JSON cannot parse never fails the timeline
		}
		out = append(out, apiEvent{
			ID:         row.ID,
			Timestamp:  row.Timestamp,
			EventType:  row.EventType,
			TaskID:     row.TaskID,
			Actor:      row.Actor,
			Detail:     decodeDetailLadder(row.Detail),
			TickNumber: row.TickNumber,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	jsonWrite(w, out)
}

// decodeDetailLadder applies the analytics 2.4 detail ladder in Go:
// (1) object/array as-is; (2) a JSON string parsing to an object/array is
// decoded; (3) a strict base64 string whose UTF-8 bytes parse as a JSON
// object/array is decoded; (4) otherwise the raw value verbatim (which may
// be a hostile string — kept inert downstream, R16). Never errors.
func decodeDetailLadder(raw json.RawMessage) any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw) // not valid JSON at all: the raw bytes, quoted as a string
	}
	s, ok := v.(string)
	if !ok {
		return v // already an object/array/number/bool: step 1
	}
	trimmed := strings.TrimSpace(s)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && isJSONContainer(parsed) {
			return parsed // step 2
		}
	}
	if decoded, ok := strictBase64JSON(trimmed); ok {
		return decoded // step 3
	}
	return v // step 4: raw string
}

func isJSONContainer(v any) bool {
	_, isMap := v.(map[string]any)
	_, isArr := v.([]any)
	return isMap || isArr
}

// strictBase64JSON decodes standard base64 whose decoded bytes parse as a
// JSON object/array. Strict: anything else (base64 of plain text, junk,
// whitespace-padded containers) falls through to the raw arm, mirroring
// the report's strictBase64ToJSON.
func strictBase64JSON(s string) (any, bool) {
	if s == "" {
		return nil, false
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	t := strings.TrimSpace(string(b))
	if !strings.HasPrefix(t, "{") && !strings.HasPrefix(t, "[") {
		return nil, false
	}
	var v any
	if err := json.Unmarshal([]byte(t), &v); err != nil || !isJSONContainer(v) {
		return nil, false
	}
	return v, true
}

// ---------- R8: unknown slug is a JSON 404, not HTML ----------

func jsonNotFound(w http.ResponseWriter, slug string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	jsonWrite(w, map[string]string{"error": "board not found: " + slug})
}
