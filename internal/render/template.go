package render

// template.html.tmpl is compiled into the binary; BuildHTML injects the
// escaped JSON island. The page is a fully self-contained file:// SPA:
// inline CSS, inline JS, hand-rolled SVG charts, no network of any kind.

import "strings"

// BuildHTML renders the report page around the escaped JSON island.
func BuildHTML(island string) string {
	return strings.Replace(htmlTemplate, "@@DATA@@", island, 1)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Board Report</title>
<style>
:root{
  --bg:#0d1117; --bg2:#161b22; --bg3:#1c2330; --fg:#e6edf3; --muted:#8b949e;
  --line:#30363d; --accent:#58a6ff; --green:#3fb950; --red:#f85149;
  --orange:#d29922; --blue:#58a6ff; --purple:#bc8cff; --gray:#6e7681;
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 -apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
a{color:var(--accent);text-decoration:none;cursor:pointer}
a:hover{text-decoration:underline}
header#topbar{display:flex;align-items:center;gap:16px;padding:10px 18px;background:var(--bg2);border-bottom:1px solid var(--line);position:sticky;top:0;z-index:5}
header h1{font-size:16px;margin:0;font-weight:600}
.tab{background:none;border:1px solid transparent;color:var(--muted);padding:4px 12px;border-radius:6px;cursor:pointer;font-size:13px}
.tab.active{color:var(--fg);border-color:var(--line);background:var(--bg3)}
.tab:hover{color:var(--fg)}
#report-meta{margin-left:auto;color:var(--muted);font-size:12px}
main{padding:18px;max-width:1280px;margin:0 auto}
#banner{margin:12px 18px 0;padding:10px 14px;border:1px solid var(--orange);border-radius:8px;background:rgba(210,153,34,.08);display:flex;gap:10px;align-items:center}
#banner .bn-notes{margin-top:8px;display:none;white-space:pre-wrap;color:var(--muted);font-size:12px;max-height:200px;overflow:auto}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:14px}
.card{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:14px;cursor:pointer;position:relative}
.card:hover{border-color:var(--accent)}
.card h2{margin:0 0 6px;font-size:15px;display:flex;gap:8px;align-items:center;flex-wrap:wrap}
.kv{display:flex;gap:14px;flex-wrap:wrap;margin-top:10px;font-size:13px}
.kv .k{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.04em}
.kv .v{font-size:18px;font-weight:600}
.warnbadge{position:absolute;top:10px;right:10px;background:var(--orange);color:#000;border-radius:10px;font-size:11px;padding:1px 8px;font-weight:600}
.badge{display:inline-block;border:1px solid var(--line);border-radius:6px;padding:0 7px;font-size:11px;color:var(--muted);text-transform:uppercase;letter-spacing:.04em}
.chip{display:inline-block;border-radius:9px;padding:0 8px;font-size:11px;font-weight:600;border:1px solid var(--line);color:var(--fg);white-space:nowrap}
.chip.complete{background:rgba(63,185,80,.15);border-color:var(--green);color:var(--green)}
.chip.pending{color:var(--muted)}
.chip.failed{background:rgba(248,81,73,.15);border-color:var(--red);color:var(--red)}
.chip.blocked{background:rgba(210,153,34,.15);border-color:var(--orange);color:var(--orange)}
.chip.in_progress{background:rgba(88,166,255,.15);border-color:var(--blue);color:var(--blue)}
.chip.review{background:rgba(188,140,255,.15);border-color:var(--purple);color:var(--purple)}
.chip.fixture{background:rgba(110,118,129,.2);border-color:var(--gray);color:var(--muted)}
.chip.pass{color:var(--green);border-color:var(--green)}
.chip.fail{color:var(--red);border-color:var(--red)}
.chip.skip{color:var(--muted)}
h3.sec{font-size:13px;text-transform:uppercase;letter-spacing:.05em;color:var(--muted);margin:26px 0 10px}
.knstrip{display:flex;gap:22px;flex-wrap:wrap;background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:12px 16px}
.knstrip .kv{margin:0}
.streakline{margin-top:8px;font-size:13px}
.muted{color:var(--muted);font-size:12px}
.stale{color:var(--muted);font-size:11px}
#board-toolbar{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin:14px 0}
#board-toolbar input[type=text]{background:var(--bg3);border:1px solid var(--line);color:var(--fg);border-radius:6px;padding:5px 10px;min-width:220px}
#board-toolbar select{background:var(--bg3);border:1px solid var(--line);color:var(--fg);border-radius:6px;padding:5px 8px}
.btn{background:var(--bg3);border:1px solid var(--line);color:var(--fg);border-radius:6px;padding:5px 12px;cursor:pointer;font-size:13px}
.btn:hover{border-color:var(--accent)}
label.toggle{display:inline-flex;gap:6px;align-items:center;color:var(--muted);cursor:pointer;font-size:13px}
.charts{display:grid;grid-template-columns:repeat(auto-fit,minmax(420px,1fr));gap:16px}
.chartbox{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:12px}
.chartbox h4{margin:0 0 8px;font-size:13px;font-weight:600;color:var(--fg)}
.chartbox svg{width:100%;height:auto;display:block}
.chartbox .cq{color:var(--muted);font-size:11px;margin-top:6px}
.legend{display:flex;gap:12px;font-size:11px;color:var(--muted);margin-top:6px;flex-wrap:wrap}
.legend .sw{display:inline-block;width:10px;height:10px;border-radius:2px;margin-right:4px;vertical-align:-1px}
.empty{color:var(--muted);text-align:center;padding:34px 0;font-size:13px}
table.tasks{width:100%;border-collapse:collapse;background:var(--bg2);border:1px solid var(--line);border-radius:10px;overflow:hidden}
table.tasks tr.trow{border-top:1px solid var(--line);cursor:pointer;outline:none}
table.tasks tr.trow:first-child{border-top:none}
table.tasks tr.trow:hover,table.tasks tr.trow:focus{background:var(--bg3)}
table.tasks tr.trow[aria-expanded=true]{background:var(--bg3)}
table.tasks td{padding:7px 10px;vertical-align:middle;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;max-width:0}
table.tasks td.tid{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px;width:150px}
table.tasks td.ttitle{width:auto;min-width:220px}
table.tasks td.tnum{width:70px;color:var(--muted);font-size:12px}
table.tasks td.tmono{width:90px;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:12px;color:var(--muted)}
tr.detailrow>td{padding:0;white-space:normal;max-width:none}
.detail{padding:14px 18px 18px;background:var(--bg);border-top:1px dashed var(--line)}
.detail h5{margin:14px 0 4px;font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em}
.detail pre{background:var(--bg2);border:1px solid var(--line);border-radius:8px;padding:10px;overflow:auto;max-height:320px;font-size:12px;color:#c9d1d9}
.detail .facts{display:flex;gap:10px;flex-wrap:wrap;margin:6px 0}
.ev{border-left:2px solid var(--line);padding:2px 10px;margin:6px 0;font-size:12px}
.ev .ets{color:var(--muted);font-family:ui-monospace,Menlo,monospace;font-size:11px}
table.plain{border-collapse:collapse;font-size:13px;min-width:420px}
table.plain th,table.plain td{border:1px solid var(--line);padding:5px 10px;text-align:left}
table.plain th{color:var(--muted);font-weight:600;background:var(--bg2)}
footer{border-top:1px solid var(--line);margin-top:34px;padding:16px 18px 40px;color:var(--muted);font-size:12px}
footer h4{color:var(--fg);font-size:12px;margin:14px 0 4px}
footer ul{margin:4px 0;padding-left:18px}
.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap}
#errbox{margin:30px auto;max-width:560px;text-align:center;color:var(--muted);padding:30px;border:1px solid var(--line);border-radius:10px}
</style>
</head>
<body>
<header id="topbar">
  <h1>Board Report</h1>
  <nav id="tabs">
    <button id="tab-overview" class="tab active" role="tab" aria-selected="true">Overview</button>
    <button id="tab-compare" class="tab" role="tab" aria-selected="false" hidden>Compare</button>
  </nav>
  <span id="report-meta"></span>
</header>
<div id="banner" hidden role="alert"></div>
<main id="view-overview"><div id="cards" class="cards"></div></main>
<main id="view-board" hidden></main>
<main id="view-compare" hidden></main>
<div id="errbox" hidden></div>
<footer id="foot"></footer>
<script type="application/json" id="report-data">@@DATA@@</script>
<script>
"use strict";
(function(){
var payload;
try { payload = JSON.parse(document.getElementById("report-data").textContent); }
catch(e){ fatal("report payload failed to parse: " + e); return; }
if (!payload || payload.schema !== "board-report/v1") { fatal("unsupported report payload"); return; }
var boards = (payload.boards || []).filter(function(b){ return b && typeof b === "object"; });

function fatal(msg){
  var eb = document.getElementById("errbox");
  eb.hidden = false;
  eb.textContent = "This report cannot be displayed: " + msg;
}

function el(tag, cls, text){
  var n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined && text !== null) n.textContent = String(text);
  return n;
}
function chip(status){
  var c = el("span", "chip", status);
  var known = {complete:1,pending:1,failed:1,blocked:1,in_progress:1,review:1,"(none)":1};
  c.setAttribute("data-status", status);
  if (!known[status]) { c.className = "chip"; c.style.color = "var(--muted)"; }
  else if (status === "(none)") { c.className = "chip"; c.style.color = "var(--muted)"; }
  return c;
}
function verdictChip(v, okWords, badWords){
  if (!v) return null;
  var up = String(v).toUpperCase();
  var cls = "chip";
  if (okWords.indexOf(up) >= 0) cls += " pass";
  else if (badWords.indexOf(up) >= 0) cls += " fail";
  else cls += " skip";
  return el("span", cls, up);
}
function svgtag(name, attrs){
  var n = document.createElementNS("http://www.w3.org/2000/svg", name);
  for (var k in attrs) n.setAttribute(k, attrs[k]);
  return n;
}
function esc(s){ return String(s === undefined || s === null ? "" : s); }
function fmtPct(a, b){ return b > 0 ? Math.round(100 * a / b) + "%" : "—"; }
function boardBySlug(slug){
  for (var i = 0; i < boards.length; i++) if (boards[i].slug === slug) return boards[i];
  return null;
}
function boardTasks(b){ return Array.isArray(b.tasks) ? b.tasks : []; }
function boardEvents(b){ return Array.isArray(b.events) ? b.events : []; }
function isFixtureRow(b, row){
  var id = row.id || "";
  if ((b.fixture_ids || []).indexOf(id) >= 0) return true;
  if (id.indexOf("NEVER-DONE") === 0) return true;
  return row.perpetual === true;
}
function taskStatus(row){
  var s = row.status;
  if (s === "completed") s = "complete";
  return s ? s : "(none)";
}
function taskIsComplete(row){ return taskStatus(row) === "complete"; }

// ---------- state + hash routing ----------
var state = { view: "overview", board: null, expanded: {}, showFixtures: false, q: "", status: "all", showAllEvents: {} };
function parseHash(){
  var h = location.hash.replace(/^#/, "");
  var o = {};
  h.split("&").forEach(function(p){
    if (!p) return;
    var kv = p.split("=");
    o[decodeURIComponent(kv[0])] = decodeURIComponent(kv.slice(1).join("="));
  });
  return o;
}
function buildHash(){
  var parts = [];
  if (state.view === "board" && state.board) parts.push("b=" + encodeURIComponent(state.board));
  if (state.view === "compare") parts.push("v=compare");
  if (state.board) {
    var ex = Object.keys(state.expanded).filter(function(k){ return state.expanded[k]; });
    if (ex.length === 1) parts.push("t=" + encodeURIComponent(ex[0]));
    if (state.q) parts.push("q=" + encodeURIComponent(state.q));
    if (state.status !== "all") parts.push("s=" + encodeURIComponent(state.status));
  }
  return parts.length ? "#" + parts.join("&") : "#";
}
function applyHash(){
  var h = parseHash();
  if (h.v === "compare" && boards.length >= 2) { showView("compare"); return; }
  if (h.b) {
    var b = boardBySlug(h.b);
    if (!b) { showView("overview"); flashNoMatch("unknown board in link"); return; }
    state.board = b.slug;
    state.expanded = {};
    state.showAllEvents = {};
    if (h.t) state.expanded[h.t] = true;
    state.q = h.q || "";
    state.status = h.s || "all";
    showView("board");
    var anchor = h.t ? document.getElementById("task-" + cssEscape(h.t)) : null;
    if (anchor) anchor.scrollIntoView({block: "start"});
    return;
  }
  showView("overview");
}
function cssEscape(s){
  return (window.CSS && CSS.escape) ? CSS.escape(s) : String(s).replace(/["\\]/g, "\\$&");
}
function flashNoMatch(msg){
  var b = document.getElementById("banner");
  b.hidden = false;
  b.textContent = "no match: " + msg;
  window.setTimeout(function(){ if (b.textContent.indexOf("no match:") === 0) { b.hidden = true; b.textContent = ""; } }, 4000);
}
window.addEventListener("hashchange", applyHash);

// ---------- views ----------
var elOverview = document.getElementById("view-overview");
var elBoard = document.getElementById("view-board");
var elCompare = document.getElementById("view-compare");
var tabOv = document.getElementById("tab-overview");
var tabCmp = document.getElementById("tab-compare");
function showView(v){
  state.view = v;
  elOverview.hidden = v !== "overview";
  elBoard.hidden = v !== "board";
  elCompare.hidden = v !== "compare";
  tabOv.classList.toggle("active", v === "overview");
  tabOv.setAttribute("aria-selected", v === "overview" ? "true" : "false");
  tabCmp.classList.toggle("active", v === "compare");
  tabCmp.setAttribute("aria-selected", v === "compare" ? "true" : "false");
  if (v === "overview") { renderOverview(); location.hash = buildHash() === "#" ? "#" : buildHash(); }
  if (v === "board") renderBoard();
  if (v === "compare") renderCompare();
}
tabOv.addEventListener("click", function(){ showView("overview"); });
tabCmp.addEventListener("click", function(){ showView("compare"); });

// ---------- banner ----------
function renderBanner(){
  var b = document.getElementById("banner");
  var total = 0, parts = [];
  boards.forEach(function(bd){
    var w = bd.parse_warnings || {};
    var n = (w.tasks||0) + (w.events||0) + (w.fixtures||0) + (w.timestamp_excluded||0) + (w.duplicate_ids||0);
    if (n > 0) parts.push(bd.name + ": " + n + " (" + warnSummary(w) + ")");
    total += n;
  });
  if (total === 0) { b.hidden = true; b.textContent = ""; return; }
  b.hidden = false; b.textContent = "";
  b.appendChild(el("strong", null, "Loaded " + boards.map(function(x){return x.name;}).join(", ") + " with " + total + " parse warning" + (total===1?"":"s")));
  var details = el("span", "muted", " — " + parts.join("; "));
  b.appendChild(details);
  var btn = el("button", "btn", "show notes");
  btn.setAttribute("aria-expanded", "false");
  var notes = el("div", "bn-notes");
  boards.forEach(function(bd){
    var w = bd.parse_warnings || {};
    (w.notes || []).forEach(function(n){ notes.appendChild(el("div", null, bd.name + ": " + n)); });
  });
  btn.addEventListener("click", function(){
    var open = notes.style.display === "block";
    notes.style.display = open ? "none" : "block";
    btn.textContent = open ? "show notes" : "hide notes";
    btn.setAttribute("aria-expanded", open ? "false" : "true");
  });
  var dismiss = el("button", "btn", "dismiss");
  dismiss.addEventListener("click", function(){ b.hidden = true; });
  b.appendChild(btn); b.appendChild(dismiss); b.appendChild(notes);
}
function warnSummary(w){
  var a = [];
  if (w.tasks) a.push("tasks " + w.tasks);
  if (w.events) a.push("events " + w.events);
  if (w.fixtures) a.push("fixtures " + w.fixtures);
  if (w.timestamp_excluded) a.push("ts " + w.timestamp_excluded);
  if (w.duplicate_ids) a.push("dup ids " + w.duplicate_ids);
  return a.join(", ");
}

// ---------- SVG helpers ----------
var NS = "http://www.w3.org/2000/svg";
function svg(w, h, box){
  var s = document.createElementNS(NS, "svg");
  s.setAttribute("width", w); s.setAttribute("height", h);
  s.setAttribute("viewBox", "0 0 " + w + " " + h);
  s.setAttribute("preserveAspectRatio", "xMidYMid meet");
  s.setAttribute("role", "img");
  if (box) s.setAttribute("aria-label", box);
  return s;
}
function linePath(xs, ys, x, y, w, h){
  var d = "";
  for (var i = 0; i < xs.length; i++) {
    d += (i === 0 ? "M" : "L") + (x + w * xs[i]).toFixed(2) + " " + (y + h - h * ys[i]).toFixed(2);
  }
  return d;
}
function sparkline(series, w, h){
  var s = svg(w, h, "burn-up sparkline");
  if (!series.length) { s.appendChild(svgText(w/2, h/2+3, "no data", "fill:var(--muted);font-size:9px", "middle")); return s; }
  var max = 1; series.forEach(function(v){ if (v > max) max = v; });
  var xs = series.map(function(_, i){ return series.length === 1 ? 0 : i / (series.length - 1); });
  var ys = series.map(function(v){ return v / max; });
  var p = svgtag("path", {d: linePath(xs, ys, 1, 1, w - 2, h - 2), fill: "none", stroke: "var(--green)", "stroke-width": "1.5"});
  s.appendChild(p);
  return s;
}
function svgText(x, y, txt, style, anchor){
  var t = svgtag("text", {x: x, y: y});
  t.setAttribute("style", style || "fill:var(--muted);font-size:10px");
  t.setAttribute("text-anchor", anchor || "start");
  t.textContent = txt;
  return t;
}
function emptyChart(msg, w, h){
  var s = svg(w || 400, h || 120, "empty chart");
  s.appendChild(svgText((w||400)/2, (h||120)/2, msg, "fill:var(--muted);font-size:12px", "middle"));
  return s;
}

// ---------- charts (hand-rolled SVG) ----------
function axisLabels(s, x, y, w, labels){
  // x: array of {pos, text} in 0..1
  labels.forEach(function(l){
    var t = svgText(x + w * l.pos, y, l.text, "fill:var(--muted);font-size:9px", "middle");
    s.appendChild(t);
  });
}
function chartLines(bd, series, opts){
  // series: [{name, days[], values[], color}]; x aligned on days of first series
  var W = 460, H = 170, pad = {l: 30, r: 8, t: 10, b: 22};
  var days = series[0] ? series[0].days : [];
  if (!days.length) return emptyChart(opts.empty, W, H);
  var max = 1;
  series.forEach(function(sr){ sr.values.forEach(function(v){ if (v > max) max = v; }); });
  var s = svg(W, H, opts.label);
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  // gridlines
  [0, .25, .5, .75, 1].forEach(function(g){
    s.appendChild(svgtag("line", {x1: pad.l, x2: W - pad.r, y1: pad.t + ih * (1 - g), y2: pad.t + ih * (1 - g), stroke: "var(--line)", "stroke-width": .5}));
    s.appendChild(svgText(pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "fill:var(--muted);font-size:9px", "end"));
  });
  series.forEach(function(sr){
    if (!sr.days.length) return;
    var xs = sr.days.map(function(_, i){ return sr.days.length === 1 ? 0 : i / (sr.days.length - 1); });
    var ys = sr.values.map(function(v){ return v / max; });
    var p = svgtag("path", {d: linePath(xs, ys, pad.l, pad.t, iw, ih), fill: "none", stroke: sr.color, "stroke-width": 1.6});
    s.appendChild(p);
  });
  var l0 = days[0], l1 = days[days.length - 1];
  s.appendChild(svgText(pad.l, H - 6, l0, "fill:var(--muted);font-size:9px"));
  var t1 = svgText(W - pad.r, H - 6, l1, "fill:var(--muted);font-size:9px", "end");
  s.appendChild(t1);
  return s;
}
function chartBars(bd, cats, values, opts){
  var W = 460, H = 170, pad = {l: 30, r: 8, t: 10, b: 26};
  if (!cats.length) return emptyChart(opts.empty, W, H);
  var max = 1; values.forEach(function(v){ if (v > max) max = v; });
  var s = svg(W, H, opts.label);
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  var n = cats.length;
  var bw = Math.max(2, iw / n * 0.72);
  for (var i = 0; i < n; i++) {
    var x = pad.l + iw * (i + 0.5) / n - bw / 2;
    var hgt = ih * values[i] / max;
    var r = svgtag("rect", {x: x.toFixed(1), y: (pad.t + ih - hgt).toFixed(1), width: bw.toFixed(1), height: Math.max(hgt, values[i] > 0 ? 1 : 0).toFixed(1), fill: opts.color || "var(--accent)", rx: 1});
    if (values[i] > 0) {
      var ti = svgtag("title");
      ti.textContent = cats[i] + ": " + values[i];
      r.appendChild(ti);
    }
    s.appendChild(r);
  }
  [0, .5, 1].forEach(function(g){
    s.appendChild(svgtag("line", {x1: pad.l, x2: W - pad.r, y1: pad.t + ih * (1 - g), y2: pad.t + ih * (1 - g), stroke: "var(--line)", "stroke-width": .5}));
    s.appendChild(svgText(pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "fill:var(--muted);font-size:9px", "end"));
  });
  var step = Math.ceil(n / 6);
  for (var j = 0; j < n; j += step) {
    s.appendChild(svgText(pad.l + iw * (j + 0.5) / n, H - 6, cats[j], "fill:var(--muted);font-size:8px", "middle"));
  }
  return s;
}
function chartTickHealth(bd){
  var th = bd.derived.tick_health || {days: [], completed: [], failed: [], audit: []};
  var W = 460, H = 170, pad = {l: 30, r: 8, t: 10, b: 26};
  if (!th.days.length) return emptyChart("no events", W, H);
  var max = 1;
  for (var i = 0; i < th.days.length; i++) {
    max = Math.max(max, (th.completed[i]||0) + (th.failed[i]||0) + (th.audit[i]||0));
  }
  var s = svg(W, H, "tick health per day");
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  var n = th.days.length;
  var bw = Math.max(2, iw / n * 0.7);
  for (var k = 0; k < n; k++) {
    var segs = [["var(--green)", th.completed[k]||0], ["var(--red)", th.failed[k]||0], ["var(--purple)", th.audit[k]||0]];
    var acc = 0;
    segs.forEach(function(sg){
      if (!sg[1]) return;
      var h1 = ih * sg[1] / max;
      var x = pad.l + iw * (k + 0.5) / n - bw / 2;
      var r = svgtag("rect", {x: x.toFixed(1), y: (pad.t + ih - (acc + h1) * ih / ih * (acc + h1 === 0 ? 0 : 1) - 0).toFixed(1), width: bw.toFixed(1), height: "0", fill: sg[0], rx: 1});
      // stack properly:
      r.setAttribute("y", (pad.t + ih - (acc + h1)).toFixed(1));
      r.setAttribute("height", Math.max(h1, .8).toFixed(1));
      var ti = svgtag("title"); ti.textContent = th.days[k] + ": " + sg[1];
      r.appendChild(ti);
      s.appendChild(r);
      acc += h1;
    });
  }
  [0, .5, 1].forEach(function(g){
    s.appendChild(svgtag("line", {x1: pad.l, x2: W - pad.r, y1: pad.t + ih * (1 - g), y2: pad.t + ih * (1 - g), stroke: "var(--line)", "stroke-width": .5}));
    s.appendChild(svgText(pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "fill:var(--muted);font-size:9px", "end"));
  });
  var step = Math.ceil(n / 6);
  for (var j = 0; j < n; j += step) s.appendChild(svgText(pad.l + iw * (j + 0.5) / n, H - 6, th.days[j], "fill:var(--muted);font-size:8px", "middle"));
  return s;
}
var WDLABELS = ["Mon","Tue","Wed","Thu","Fri","Sat","Sun"];
function chartWorkClock(bd){
  var wc = bd.derived.work_clock || {grid: [], max: 0};
  var cw = 22, ch = 16, lx = 30, ty = 4;
  var W = lx + 24 * cw + 10, H = ty + 7 * ch + 18;
  var s = svg(W, H, "work clock weekday x hour heatmap");
  var buckets = [1, 2, 4, 8];
  function color(v){
    if (v <= 0) return "var(--bg3)";
    var idx = 0;
    if (v >= buckets[3]) idx = 4; else if (v >= buckets[2]) idx = 3; else if (v >= buckets[1]) idx = 2; else idx = 1;
    return ["var(--bg3)", "#1f6feb55", "#1f6feb99", "#58a6ffcc", "#58a6ff"][idx];
  }
  for (var d = 0; d < 7; d++) {
    s.appendChild(svgText(lx - 4, ty + d * ch + ch / 2 + 3, WDLABELS[d], "fill:var(--muted);font-size:9px", "end"));
    for (var h = 0; h < 24; h++) {
      var v = (wc.grid[d] || [])[h] || 0;
      var r = svgtag("rect", {x: lx + h * cw, y: ty + d * ch, width: cw - 2, height: ch - 2, rx: 2, fill: color(v)});
      var ti = svgtag("title"); ti.textContent = WDLABELS[d] + " " + h + ":00 — " + v + " events";
      r.appendChild(ti);
      s.appendChild(r);
    }
  }
  for (var hh = 0; hh < 24; hh += 4) s.appendChild(svgText(lx + hh * cw + cw / 2, H - 4, hh, "fill:var(--muted);font-size:8px", "middle"));
  return s;
}
function chartModelShare(bd){
  var ms = bd.derived.model_share || {models: [], counts: [], completed_n: 0};
  var W = 460, rh = 22, lx = 8;
  if (!ms.models.length) return emptyChart("no completed tasks", W, 30 + 10);
  var H = 30 + ms.models.length * rh;
  var s = svg(W, H, "model share across completed tasks");
  var max = 1; ms.counts.forEach(function(c){ if (c > max) max = c; });
  var barW = W - lx - 130;
  ms.models.forEach(function(m, i){
    var y = 24 + i * rh;
    var w = barW * ms.counts[i] / max;
    var r = svgtag("rect", {x: lx, y: y, width: Math.max(w, 2), height: rh - 8, rx: 3, fill: "var(--accent)"});
    var ti = svgtag("title"); ti.textContent = m + ": " + ms.counts[i] + " of " + ms.completed_n;
    r.appendChild(ti);
    s.appendChild(r);
    var t = svgText(lx + 6, y + rh - 14, m + "  " + ms.counts[i] + " (" + fmtPct(ms.counts[i], ms.completed_n) + ")", "fill:#0d1117;font-size:10px;font-weight:600");
    if (w > 150) { t.setAttribute("style", "fill:#0d1117;font-size:10px;font-weight:600"); }
    else { t.setAttribute("x", lx + w + 8); t.setAttribute("style", "fill:var(--muted);font-size:10px"); }
    s.appendChild(t);
  });
  return s;
}
function chartCalendarDays(bd, activeDays, small){
  var cw = small ? 4 : 11, ch = small ? 4 : 11, lgap = small ? 12 : 18;
  var W = lgap + 26 * (cw + 1), H = 7 * (ch + 1) + 12;
  var s = svg(W, H, "26-week activity calendar");
  var today = (payload.rendered_at || "").slice(0, 10);
  var t = today ? new Date(today + "T00:00:00Z") : new Date();
  if (isNaN(t.getTime())) t = new Date();
  var dow = (t.getUTCDay() + 6) % 7; // 0=Mon
  var weekStart = new Date(t.getTime() - (dow + 25 * 7) * 86400000);
  for (var w = 0; w < 26; w++) {
    for (var d = 0; d < 7; d++) {
      var day = new Date(weekStart.getTime() + (w * 7 + d) * 86400000);
      if (day > t) continue;
      var key = day.toISOString().slice(0, 10);
      var on = activeDays[key];
      var r = svgtag("rect", {x: lgap + w * (cw + 1), y: d * (ch + 1), width: cw, height: ch, rx: 1.5, fill: on ? "var(--green)" : "var(--bg3)"});
      var ti = svgtag("title"); ti.textContent = key + (on ? ": active" : "");
      r.appendChild(ti);
      s.appendChild(r);
    }
  }
  return s;
}

// ---------- overview ----------
function keyNumbers(bd){
  var d = bd.derived;
  return {
    total: boardTasks(bd).filter(function(r){ return !isFixtureRow(bd, r); }).length,
    open: d.open_count, complete: d.complete_count,
    pct: fmtPct(d.complete_count, d.complete_count + d.open_count),
    dcurr: d.streaks.delivery.current, dstate: d.streaks.delivery.current_state
  };
}
function ticksInfo(bd){
  var h = bd.header;
  if (!h) return null;
  function num(k){
    var v = h[k];
    return (typeof v === "number") ? v : null;
  }
  var tt = num("ticks_total"), ti = num("ticks_idle");
  if (tt === null && ti === null) return null;
  return {total: tt, idle: ti};
}
function renderOverview(){
  var cards = document.getElementById("cards");
  cards.textContent = "";
  if (!boards.length) {
    cards.appendChild(el("div", "empty", "no boards in this report"));
    return;
  }
  boards.forEach(function(bd){
    var d = bd.derived;
    var card = el("div", "card");
    card.setAttribute("role", "button");
    card.setAttribute("tabindex", "0");
    card.setAttribute("aria-label", "open board " + bd.name);
    var h = el("h2");
    h.appendChild(el("span", null, bd.name));
    h.appendChild(el("span", "badge", "topology " + bd.topology));
    var ns = bd.header && bd.header.namespace;
    if (ns) h.appendChild(el("span", "badge", String(ns)));
    var w = bd.parse_warnings || {};
    var wn = (w.tasks||0)+(w.events||0)+(w.fixtures||0)+(w.timestamp_excluded||0)+(w.duplicate_ids||0);
    card.appendChild(h);
    var spark = sparkline(d.burnup.cum || [], 120, 28);
    spark.setAttribute("aria-label", "burn-up sparkline");
    card.appendChild(spark);
    var kn = keyNumbers(bd);
    var kv = el("div", "kv");
    function kblock(k, v){
      var b = el("div");
      b.appendChild(el("div", "k", k));
      b.appendChild(el("div", "v", v));
      return b;
    }
    kv.appendChild(kblock("tasks", kn.total));
    kv.appendChild(kblock("open", kn.open));
    kv.appendChild(kblock("complete", kn.pct));
    kv.appendChild(kblock("delivery streak", kn.dcurr + (kn.dstate === "live" ? "" : " (ended)")));
    var ti = ticksInfo(bd);
    if (ti) kv.appendChild(kblock("ticks", (ti.total === null ? "—" : ti.total) + " / " + (ti.idle === null ? "—" : ti.idle) + " idle"));
    card.appendChild(kv);
    if (wn > 0) {
      var wb = el("span", "warnbadge", wn + " ⚠");
      wb.setAttribute("title", "parse warnings — see banner");
      card.appendChild(wb);
    }
    function open(){ state.board = bd.slug; state.expanded = {}; state.q = ""; state.status = "all"; location.hash = buildHash(); showView("board"); }
    card.addEventListener("click", open);
    card.addEventListener("keydown", function(e){ if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); } });
    cards.appendChild(card);
  });
}

// ---------- board page ----------
function renderBoard(){
  var bd = boardBySlug(state.board) || boards[0];
  if (!bd) { showView("overview"); return; }
  state.board = bd.slug;
  elBoard.textContent = "";
  var d = bd.derived;

  var title = el("h2", null, bd.name + " ");
  title.appendChild(el("span", "badge", "topology " + bd.topology));
  var ns = bd.header && bd.header.namespace;
  if (ns) title.appendChild(el("span", "badge", String(ns)));
  elBoard.appendChild(title);

  // key numbers strip
  var kn = el("div", "knstrip");
  function kblock(k, v){
    var b = el("div");
    b.appendChild(el("div", "k", k));
    b.appendChild(el("div", "v", v));
    return b;
  }
  var total = boardTasks(bd).filter(function(r){ return !isFixtureRow(bd, r); }).length;
  kn.appendChild(kblock("tasks", total));
  kn.appendChild(kblock("open", d.open_count));
  kn.appendChild(kblock("complete", fmtPct(d.complete_count, d.complete_count + d.open_count)));
  kn.appendChild(kblock("median cycle", d.cycle.median_days === null || d.cycle.median_days === undefined ? "—" : d.cycle.median_days + "d"));
  kn.appendChild(kblock("p90 cycle", d.cycle.p90_days === null || d.cycle.p90_days === undefined ? "—" : d.cycle.p90_days + "d"));
  kn.appendChild(kblock("last activity", d.last_activity_day || "—"));
  var ti = ticksInfo(bd);
  if (ti) kn.appendChild(kblock("ticks", (ti.total === null ? "—" : ti.total) + " / " + (ti.idle === null ? "—" : ti.idle)));
  elBoard.appendChild(kn);

  // streaks block (delivery headline, activity secondary, any-tick muted)
  var sl = el("div", "streakline");
  var dv = d.streaks.delivery;
  sl.appendChild(el("strong", null, "Delivery streak: " + dv.current + "d"));
  if (dv.current_state === "ended" && dv.current_since) {
    sl.appendChild(el("span", "stale", "  ended " + dv.current_since));
  }
  var ac = d.streaks.activity;
  sl.appendChild(el("span", null, "   ·   Activity streak: " + ac.current + "d (longest " + ac.longest + "d)"));
  if (ac.current_state === "ended" && ac.current_since) sl.appendChild(el("span", "stale", " ended " + ac.current_since));
  sl.appendChild(el("span", "stale", "   ·   raw any-tick: " + d.streaks.any_tick.current + "d current / " + d.streaks.any_tick.longest + "d longest"));
  elBoard.appendChild(sl);
  elBoard.appendChild(el("div", "muted", "cycle time over " + d.cycle.n + " of " + (d.cycle.n + d.cycle.excluded) + " completed tasks"));

  // charts
  var charts = el("div", "charts");
  function box(titleTxt, node, question){
    var c = el("div", "chartbox");
    c.appendChild(el("h4", null, titleTxt));
    c.appendChild(node);
    if (question) c.appendChild(el("div", "cq", question));
    return c;
  }
  var burndownBox = box("Burndown — open tasks per day", chartLines(bd, [{days: d.burndown.days || [], values: d.burndown.open || [], color: "var(--orange)"}], {label: "burndown", empty: "no tasks yet"}), "Is the backlog actually shrinking, or are we opening work as fast as we close it?");
  var burnupBox = box("Burn-up — cumulative completions", chartLines(bd, [{days: d.burnup.days || [], values: d.burnup.cum || [], color: "var(--green)"}], {label: "burn-up", empty: "no completions yet"}), "How much real work has this board delivered over time, and is the slope still alive?");
  charts.appendChild(burndownBox); charts.appendChild(burnupBox);
  // overlay toggle (default separate): button swaps the two panels
  var separateHolder = el("div");
  separateHolder.appendChild(burndownBox);
  separateHolder.appendChild(burnupBox);
  var overlayHolder = el("div");
  overlayHolder.setAttribute("hidden", "");
  function renderOverlay(){
    var overlaid = chartLines(bd, [
      {days: d.burndown.days || [], values: d.burndown.open || [], color: "var(--orange)"},
      {days: d.burnup.days || [], values: d.burnup.cum || [], color: "var(--green)"}
    ], {label: "burndown + burn-up overlay", empty: "no tasks yet"});
    var lg = el("div", "legend");
    function legendItem(color, label){
      var sp = el("span");
      var swb = el("span", "sw");
      swb.style.background = color;
      sp.appendChild(swb);
      sp.appendChild(document.createTextNode(label));
      return sp;
    }
    lg.appendChild(legendItem("var(--orange)", "open"));
    lg.appendChild(legendItem("var(--green)", "cumulative complete"));
    overlayHolder.textContent = "";
    overlayHolder.appendChild(overlaid);
    overlayHolder.appendChild(lg);
  }
  var overlayBtn = el("button", "btn", "Overlay burndown + burn-up");
  var overlaid = false;
  overlayBtn.addEventListener("click", function(){
    overlaid = !overlaid;
    if (overlaid) { renderOverlay(); overlayHolder.removeAttribute("hidden"); separateHolder.setAttribute("hidden", ""); overlayBtn.textContent = "Show separate"; }
    else { overlayHolder.setAttribute("hidden", ""); separateHolder.removeAttribute("hidden"); overlayBtn.textContent = "Overlay burndown + burn-up"; }
  });
  charts.appendChild(overlayBtn);
  charts.appendChild(separateHolder);
  charts.appendChild(overlayHolder);
  charts.appendChild(box("Weekly velocity — completions per ISO week", velocityChart(bd), "Are we completing more or fewer tasks per week than last month?"));
  charts.appendChild(box("Tick health per day", chartTickHealth(bd), "When did things happen, and were the ticks healthy or throwing audits and failures?"));
  charts.appendChild(box("Work clock — weekday × hour", chartWorkClock(bd), "When does this fleet actually work — which hours/days carry the load?"));
  charts.appendChild(box("Model share — completed tasks", chartModelShare(bd), "Which worker models are doing the delivery, and is one model carrying everything?"));
  elBoard.appendChild(charts);

  // calendar heatmap
  var cal = el("div", "chartbox");
  cal.appendChild(el("h4", null, "Last 26 weeks of activity"));
  cal.appendChild(chartCalendarDays(bd, activityDays(bd), false));
  cal.appendChild(el("div", "cq", "What does the last half-year of activity look like at a glance — are there dead weeks?"));
  elBoard.appendChild(cal);

  // worst5
  if (d.cycle.worst5 && d.cycle.worst5.length) {
    elBoard.appendChild(el("h3", "sec", "Slowest tasks (cycle time)"));
    var wt = el("table", "plain");
    var wr = el("tr");
    ["task", "title", "days", "status"].forEach(function(hh){ var c = el("th", null, hh); wr.appendChild(c); });
    wt.appendChild(wr);
    d.cycle.worst5.forEach(function(w5){
      var r = el("tr");
      r.appendChild(el("td", null, w5.id));
      r.appendChild(el("td", null, w5.title));
      r.appendChild(el("td", null, w5.days));
      var sc = el("td"); sc.appendChild(chip(w5.status)); r.appendChild(sc);
      wt.appendChild(r);
    });
    elBoard.appendChild(wt);
  }

  // toolbar + table
  elBoard.appendChild(el("h3", "sec", "Tasks"));
  buildToolbar(bd);
  var tbl = el("table", "tasks");
  tbl.setAttribute("id", "task-table");
  elBoard.appendChild(tbl);
  renderTable(bd);
}

function velocityChart(bd){
  var v = bd.derived.velocity || {weeks: [], counts: []};
  var weeks = v.weeks, counts = v.counts;
  if (!weeks.length) return emptyChart("no tasks yet");
  // display window: most recent 12 weeks that have a completion or a birth
  var birthWeeks = {};
  boardTasks(bd).forEach(function(r){
    if (isFixtureRow(bd, r)) return;
    var c = r.created_at; if (!c) return;
    var wk = isoWeekOf(c.slice(0, 10)); if (wk) birthWeeks[wk] = true;
  });
  var lastNonZero = -1;
  for (var i = weeks.length - 1; i >= 0; i--) { if (counts[i] > 0 || birthWeeks[weeks[i]]) { lastNonZero = i; break; } }
  var start = Math.max(0, (lastNonZero >= 0 ? lastNonZero : weeks.length - 1) - 11);
  var w2 = weeks.slice(start), c2 = counts.slice(start);
  return chartBars(bd, w2, c2, {label: "weekly velocity", empty: "no completions yet", color: "var(--accent)"});
}
function isoWeekOf(dayKey){
  if (!/^\d{4}-\d{2}-\d{2}$/.test(dayKey || "")) return null;
  var t = new Date(dayKey + "T00:00:00Z");
  var d = new Date(Date.UTC(t.getUTCFullYear(), t.getUTCMonth(), t.getUTCDate()));
  var dayNum = (d.getUTCDay() + 6) % 7;
  d.setUTCDate(d.getUTCDate() - dayNum + 3);
  var firstThursday = new Date(Date.UTC(d.getUTCFullYear(), 0, 4));
  var fDayNum = (firstThursday.getUTCDay() + 6) % 7;
  firstThursday.setUTCDate(firstThursday.getUTCDate() - fDayNum + 3);
  var wk = 1 + Math.round((d - firstThursday) / (7 * 86400000));
  return d.getUTCFullYear() + "-W" + (wk < 10 ? "0" : "") + wk;
}
function activityDays(bd){
  // non-idle event days from raw events (fallback when tick_health empty)
  var days = {};
  var th = (bd.derived.tick_health && bd.derived.tick_health.days) || [];
  th.forEach(function(d){ days[d] = true; });
  boardEvents(bd).forEach(function(e){
    var ts = e.timestamp || "";
    if (!ts) return;
    var key = ts.slice(0, 10);
    if ((e.event_type || "") === "idle") return;
    days[key] = true;
  });
  // completion days
  boardTasks(bd).forEach(function(r){
    if (isFixtureRow(bd, r)) return;
    if (!taskIsComplete(r)) return;
    var c = r.completed_at || (r.updated_at || "");
    if (c) days[c.slice(0, 10)] = true;
  });
  return days;
}

// ---------- toolbar + task table ----------
function buildToolbar(bd){
  var tb = el("div");
  tb.setAttribute("id", "board-toolbar");
  var q = el("input");
  q.type = "text";
  q.placeholder = "filter id / title / summary / commit";
  q.value = state.q;
  q.setAttribute("aria-label", "filter tasks");
  var deb = null;
  q.addEventListener("input", function(){
    window.clearTimeout(deb);
    deb = window.setTimeout(function(){
      state.q = q.value;
      renderTable(bd);
      history.replaceState(null, "", buildHash());
    }, 150);
  });
  tb.appendChild(q);
  var sel = el("select");
  sel.setAttribute("aria-label", "filter by status");
  var statuses = {};
  boardTasks(bd).forEach(function(r){
    if (!state.showFixtures && isFixtureRow(bd, r)) return;
    statuses[taskStatus(r)] = true;
  });
  var opts = ["all"].concat(Object.keys(statuses).sort());
  opts.forEach(function(o){
    var op = el("option", null, o);
    op.setAttribute("value", o);
    sel.appendChild(op);
  });
  sel.value = state.status;
  if (opts.indexOf(state.status) < 0) { state.status = "all"; sel.value = "all"; }
  sel.addEventListener("change", function(){ state.status = sel.value; renderTable(bd); history.replaceState(null, "", buildHash()); });
  tb.appendChild(sel);
  var fx = el("input"); fx.type = "checkbox"; fx.checked = state.showFixtures; fx.setAttribute("id", "fx-" + bd.slug);
  fx.addEventListener("change", function(){ state.showFixtures = fx.checked; renderTable(bd); });
  var fxl = el("label", "toggle");
  fxl.setAttribute("for", "fx-" + bd.slug);
  fxl.appendChild(fx); fxl.appendChild(document.createTextNode(" show fixtures"));
  tb.appendChild(fxl);
  var ea = el("button", "btn", "Expand all");
  ea.addEventListener("click", function(){
    boardTasks(bd).forEach(function(r){ state.expanded[r.id] = true; });
    renderTable(bd);
  });
  var ca = el("button", "btn", "Collapse all");
  ca.addEventListener("click", function(){ state.expanded = {}; renderTable(bd); });
  tb.appendChild(ea); tb.appendChild(ca);
  var count = el("span", "muted", "");
  count.setAttribute("id", "rowcount");
  tb.appendChild(count);
  elBoard.appendChild(tb);
}
function rowMatches(bd, row){
  if (!state.showFixtures && isFixtureRow(bd, row)) return false;
  if (state.status !== "all" && taskStatus(row) !== state.status) return false;
  if (state.q) {
    var q = state.q.toLowerCase();
    var hay = [row.id, row.title, row.worker_summary, row.commit_hash].map(function(x){ return (x === undefined || x === null) ? "" : String(x).toLowerCase(); });
    if (!hay.some(function(h){ return h.indexOf(q) >= 0; })) return false;
  }
  return true;
}
function detailFor(bd, row){
  // related events: top-level task_id match, file order; last 8 by default
  var id = row.id;
  var all = boardEvents(bd).filter(function(e){ return e.task_id === id; });
  var wrap = el("div");
  var showN = state.showAllEvents[id] ? all.length : Math.min(8, all.length);
  if (!all.length) {
    wrap.appendChild(el("div", "muted", "no events for this task"));
    return wrap;
  }
  var head = el("h5", null, "Events (" + all.length + ")");
  wrap.appendChild(head);
  var shown = all.slice(Math.max(0, all.length - showN));
  shown.forEach(function(e){
    var ev = el("div", "ev");
    var meta = el("div");
    meta.appendChild(el("span", "ets", (e.timestamp || "?") + "  "));
    meta.appendChild(el("strong", null, e.event_type || "?"));
    meta.appendChild(el("span", "ets", "  " + (e.actor || "") + (e.tick_number ? "  tick " + e.tick_number : "")));
    ev.appendChild(meta);
    var de = detailBody(e);
    ev.appendChild(de);
    wrap.appendChild(ev);
  });
  if (all.length > 8) {
    var btn = el("button", "btn", state.showAllEvents[id] ? "show latest 8 only" : "show all " + all.length);
    btn.addEventListener("click", function(){
      state.showAllEvents[id] = !state.showAllEvents[id];
      renderTable(bd);
      var t = document.getElementById("task-" + cssEscape(id));
      if (t) t.scrollIntoView({block: "nearest"});
    });
    wrap.appendChild(btn);
  }
  return wrap;
}
function detailBody(e){
  var pre = el("pre");
  var v = e.detail;
  if (v === undefined || v === null) { pre.textContent = "(no detail)"; return pre; }
  if (typeof v === "object") {
    var txt = null;
    try { txt = JSON.stringify(v, null, 2); } catch (err) { txt = null; }
    if (txt !== null) { pre.textContent = txt; return pre; }
  }
  pre.textContent = String(v);
  return pre;
}
function taskDetail(bd, row){
  var d = el("div", "detail");
  function sec(name, node){ if (node) { var h = el("h5", null, name); d.appendChild(h); d.appendChild(node); } }
  if (row.reasoning) sec("reasoning", textBlock(row.reasoning));
  if (row.worker_summary) sec("worker summary", textBlock(row.worker_summary));
  if (row.foreman_note) sec("foreman note", textBlock(row.foreman_note));
  if (row.review_notes) sec("review notes", textBlock(row.review_notes));
  var facts = el("div", "facts");
  function fact(label, val){
    if (val === undefined || val === null || val === "") return;
    var f = el("span", "badge", label + ": " + val);
    facts.appendChild(f);
  }
  fact("attempts", row.attempts);
  fact("exit_code", row.exit_code);
  fact("complexity", row.complexity);
  fact("lines +", row.lines_added);
  fact("lines −", row.lines_removed);
  var g = verdictChip(row.guard_result, ["PASS"], ["FAIL"]);
  if (g) { g.setAttribute("title", "guard_result"); facts.appendChild(g); }
  var c = verdictChip(row.ci_result, ["GREEN"], ["RED"]);
  if (c) { c.setAttribute("title", "ci_result"); facts.appendChild(c); }
  if (facts.childNodes.length) { d.appendChild(el("h5", null, "facts")); d.appendChild(facts); }
  var ts = el("div", "facts");
  fact2(ts, "created_at", row.created_at);
  fact2(ts, "dispatched_at", row.dispatched_at);
  fact2(ts, "completed_at", row.completed_at);
  fact2(ts, "updated_at", row.updated_at);
  fact2(ts, "blocked_since", row.blocked_since);
  if (row.blocked_reason) fact2(ts, "blocked_reason", row.blocked_reason);
  if (ts.childNodes.length) { d.appendChild(el("h5", null, "timestamps")); d.appendChild(ts); }
  function fact2(parent, label, val){
    if (val === undefined || val === null || val === "") return;
    parent.appendChild(el("span", "badge", label + ": " + val));
  }
  var links = el("div", "facts");
  function rel(label, list){
    if (!Array.isArray(list) || !list.length) return;
    list.forEach(function(dep){
      var known = boardTasks(bd).some(function(r){ return r.id === dep; });
      if (known) {
        var a = el("a", null, label + " " + dep);
        a.setAttribute("href", "#b=" + encodeURIComponent(bd.slug) + "&t=" + encodeURIComponent(dep));
        a.addEventListener("click", function(ev){
          ev.preventDefault();
          state.expanded = {};
          state.expanded[dep] = true;
          location.hash = "#b=" + encodeURIComponent(bd.slug) + "&t=" + encodeURIComponent(dep);
          applyHash();
        });
        links.appendChild(a);
      } else {
        links.appendChild(el("span", "badge", label + " " + dep + " (unknown)"));
      }
    });
  }
  rel("depends_on", row.depends_on);
  rel("blocks", row.blocks);
  if (links.childNodes.length) { d.appendChild(el("h5", null, "dependencies")); d.appendChild(links); }
  if (Array.isArray(row.files_changed) && row.files_changed.length) {
    var fl = el("ul");
    row.files_changed.forEach(function(f){ fl.appendChild(el("li", null, String(f))); });
    sec("files changed", fl);
  }
  if (Array.isArray(row.capability_tags) && row.capability_tags.length) {
    var tl = el("div", "facts");
    row.capability_tags.forEach(function(t){ tl.appendChild(el("span", "badge", String(t))); });
    d.appendChild(el("h5", null, "capability tags")); d.appendChild(tl);
  }
  sec("related events", detailFor(bd, row));
  var jp = el("pre");
  var raw = null;
  try { raw = JSON.stringify(row, null, 2); } catch (e2) { raw = null; }
  jp.textContent = raw !== null ? raw : "(unserializable row)";
  sec("task JSON", jp);
  return d;
}
function textBlock(v){
  var p = el("p", null, String(v));
  p.style.whiteSpace = "pre-wrap";
  p.style.margin = "4px 0";
  return p;
}
function renderTable(bd){
  var tbl = document.getElementById("task-table");
  if (!tbl) return;
  while (tbl.firstChild) tbl.removeChild(tbl.firstChild);
  var rows = boardTasks(bd);
  var visible = rows.filter(function(r){ return rowMatches(bd, r); });
  var rc = document.getElementById("rowcount");
  if (rc) rc.textContent = visible.length + " of " + rows.length + " rows";
  if (!visible.length) {
    var er = el("tr");
    var ed = el("td", "empty", rows.length ? "no match for the current filters" : "no tasks yet");
    ed.setAttribute("colspan", "6");
    er.appendChild(ed);
    tbl.appendChild(er);
    return;
  }
  visible.forEach(function(row){
    var isFix = isFixtureRow(bd, row);
    var expanded = !!state.expanded[row.id];
    var tr = el("tr", "trow");
    tr.setAttribute("id", "task-" + row.id);
    tr.setAttribute("role", "button");
    tr.setAttribute("tabindex", "0");
    tr.setAttribute("aria-expanded", expanded ? "true" : "false");
    var td1 = el("td", "tid", row.id || "");
    tr.appendChild(td1);
    var td2 = el("td", "ttitle");
    var titleFull = row.title === undefined || row.title === null ? "" : String(row.title);
    var span = el("span", null, titleFull.length > 100 ? titleFull.slice(0, 99) + "…" : titleFull);
    span.style.display = "block";
    span.style.overflow = "hidden";
    span.style.textOverflow = "ellipsis";
    span.style.whiteSpace = "nowrap";
    td2.appendChild(span);
    if (isFix) td2.appendChild(el("span", "chip fixture", "fixture"));
    tr.appendChild(td2);
    var td3 = el("td");
    td3.appendChild(chip(taskStatus(row)));
    tr.appendChild(td3);
    var prio = row.priority === undefined || row.priority === null ? "" : String(row.priority);
    tr.appendChild(el("td", "tnum", prio || "—"));
    var model = row.primary_model ? String(row.primary_model) : "—";
    tr.appendChild(el("td", "tnum", model));
    var ch = row.commit_hash ? String(row.commit_hash).slice(0, 7) : "—";
    tr.appendChild(el("td", "tmono", ch));
    tr.appendChild(el("td", "tnum", (row.updated_at || row.created_at || "—").slice(0, 10)));
    function toggle(){
      state.expanded[row.id] = !expanded;
      history.replaceState(null, "", buildHash());
      renderTable(bd);
      // re-render replaced this row; restore focus so keyboard users keep
      // working the same row (Escape collapses the focused expanded row)
      var again = document.getElementById("task-" + row.id);
      if (again) again.focus();
    }
    tr.addEventListener("click", toggle);
    tr.addEventListener("keydown", function(e){
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggle(); }
      if (e.key === "Escape" && expanded) { state.expanded[row.id] = false; renderTable(bd); }
    });
    tbl.appendChild(tr);
    if (expanded) {
      var dr = el("tr", "detailrow");
      var dd = el("td");
      dd.setAttribute("colspan", "7");
      dd.appendChild(taskDetail(bd, row));
      dr.appendChild(dd);
      tbl.appendChild(dr);
    }
  });
}

// ---------- compare ----------
function renderCompare(){
  if (boards.length < 2) { showView("overview"); return; }
  elCompare.textContent = "";
  elCompare.appendChild(el("h2", null, "Compare boards"));
  // key numbers table
  var t = el("table", "plain");
  var cols = ["board", "ticks_total", "ticks_idle", "idle %", "tasks", "open", "complete", "completion %", "delivery streak", "median cycle", "last activity"];
  var hr = el("tr");
  cols.forEach(function(c){ hr.appendChild(el("th", null, c)); });
  t.appendChild(hr);
  boards.forEach(function(bd){
    var d = bd.derived;
    var ti = ticksInfo(bd);
    var r = el("tr");
    function cc(v){ r.appendChild(el("td", null, v)); }
    cc(bd.name);
    cc(ti && ti.total !== null ? ti.total : "—");
    cc(ti && ti.idle !== null ? ti.idle : "—");
    cc(ti && ti.total ? Math.round(100 * (ti.idle || 0) / ti.total) + "%" : "—");
    cc(keyNumbers(bd).total);
    cc(d.open_count);
    cc(d.complete_count);
    cc(fmtPct(d.complete_count, d.complete_count + d.open_count));
    cc(d.streaks.delivery.current + (d.streaks.delivery.current_state === "live" ? "" : " (ended)"));
    cc(d.cycle.median_days === null || d.cycle.median_days === undefined ? "—" : d.cycle.median_days + "d");
    cc(d.last_activity_day || "—");
    t.appendChild(r);
  });
  elCompare.appendChild(t);
  // overlaid burn-up (x = days since first birth)
  elCompare.appendChild(el("h3", "sec", "Overlaid burn-up — days since first task"));
  var colors = ["#58a6ff", "#3fb950", "#bc8cff", "#d29922", "#f85149", "#39c5cf"];
  var W = 900, H = 260, pad = {l: 36, r: 10, t: 12, b: 26};
  var s = svg(W, H, "overlaid burn-up");
  var globalMax = 1;
  boards.forEach(function(bd){
    (bd.derived.burnup.cum || []).forEach(function(v){ if (v > globalMax) globalMax = v; });
  });
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  [0, .25, .5, .75, 1].forEach(function(g){
    s.appendChild(svgtag("line", {x1: pad.l, x2: W - pad.r, y1: pad.t + ih * (1 - g), y2: pad.t + ih * (1 - g), stroke: "var(--line)", "stroke-width": .5}));
    s.appendChild(svgText(pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(globalMax * g), "fill:var(--muted);font-size:9px", "end"));
  });
  var legend = el("div", "legend");
  boards.forEach(function(bd, bi){
    var cum = bd.derived.burnup.cum || [];
    if (!cum.length) return;
    var color = colors[bi % colors.length];
    var xs = cum.map(function(_, i){ return cum.length === 1 ? 0 : i / (cum.length - 1); });
    var ys = cum.map(function(v){ return v / globalMax; });
    s.appendChild(svgtag("path", {d: linePath(xs, ys, pad.l, pad.t, iw, ih), fill: "none", stroke: color, "stroke-width": 1.6}));
    var sw = el("span");
    var swb = el("span", "sw");
    swb.style.background = color;
    sw.appendChild(swb);
    sw.appendChild(document.createTextNode(bd.name));
    legend.appendChild(sw);
  });
  s.appendChild(svgText(pad.l, H - 6, "day 0", "fill:var(--muted);font-size:9px"));
  s.appendChild(svgText(W - pad.r, H - 6, "day N", "fill:var(--muted);font-size:9px", "end"));
  var cb = el("div", "chartbox");
  cb.appendChild(s);
  cb.appendChild(legend);
  elCompare.appendChild(cb);
  // velocity race: first 12 weeks per board
  elCompare.appendChild(el("h3", "sec", "Velocity race — first 12 weeks of each board"));
  var race = el("div", "charts");
  boards.forEach(function(bd){
    var v = bd.derived.velocity || {weeks: [], counts: []};
    var w12 = v.weeks.slice(0, 12).map(function(_, i){ return "W" + (i + 1); });
    var c12 = v.counts.slice(0, 12);
    var box = el("div", "chartbox");
    box.appendChild(el("h4", null, bd.name));
    box.appendChild(chartBars(bd, w12, c12, {label: "first-12-week velocity", empty: "no completions yet"}));
    race.appendChild(box);
  });
  elCompare.appendChild(race);
}

// ---------- footer ----------
function renderFooter(){
  var f = document.getElementById("foot");
  f.textContent = "";
  f.appendChild(el("div", null, payload.generated_by || "boardctl render"));
  f.appendChild(el("div", null, "rendered " + payload.rendered_at + "  ·  report timezone: " + payload.report_timezone));
  var h1 = el("h4", null, "Streak honesty (why each number can or cannot be gamed)");
  f.appendChild(h1);
  var ul = el("ul");
  var li1 = el("li", null, "Delivery streak (headline) requires a real task row to move to complete with a real completed_at; duplicate ids collapse to one completion, fixtures are excluded, and the worst-5 cycle table makes suspicious re-completions visible. Residual risk: a re-opened task re-dated today.");
  var li2 = el("li", null, "Activity streak counts any non-idle event — one cheap audit per day keeps it alive. That is exactly why it is not the headline.");
  var li3 = el("li", null, "Raw any-tick streak counts idle ticks by definition — context only, never a headline.");
  ul.appendChild(li1); ul.appendChild(li2); ul.appendChild(li3);
  f.appendChild(ul);
  if (Array.isArray(payload.completed_no_done_ids) && payload.completed_no_done_ids.length) {
    var h2 = el("h4", null, "Data quality");
    f.appendChild(h2);
    f.appendChild(el("div", null, "completed tasks with no computable completion day (never counted in time series): " + payload.completed_no_done_ids.join(", ")));
  }
  var h3 = el("h4", null, "View catalog");
  f.appendChild(h3);
  var ul2 = el("ul");
  [
    ["burndown", "Is the backlog actually shrinking, or are we opening work as fast as we close it?"],
    ["burn-up", "How much real work has this board delivered over time, and is the slope still alive?"],
    ["weekly velocity", "Are we completing more or fewer tasks per week than last month?"],
    ["cycle time", "How long does a task take from filing to done, and which tasks are the outliers?"],
    ["delivery streak", "Have we shipped something real every day — is the board genuinely moving?"],
    ["activity streak", "Is the project being worked on at all, even on days without a completion?"],
    ["raw any-tick streak", "How long has the scheduler been firing? (context only)"],
    ["tick-health timeline", "When did things happen, and were the ticks healthy?"],
    ["work clock", "When does this fleet actually work — which hours/days carry the load?"],
    ["model share", "Which worker models are doing the delivery?"],
    ["status / priority counts", "What is the board's composition right now?"],
    ["26-week calendar", "What does the last half-year of activity look like at a glance?"],
    ["compare", "Which of these boards is healthiest — who is delivering fastest, who is stuck?"],
    ["task table", "What is the actual state and history of this one task?"]
  ].forEach(function(p){
    var li = el("li");
    li.appendChild(el("strong", null, p[0]));
    li.appendChild(document.createTextNode(" — " + p[1]));
    ul2.appendChild(li);
  });
  f.appendChild(ul2);
  var h4 = el("h4", null, "Filter / gameability of counts");
  f.appendChild(h4);
  f.appendChild(el("div", null, "Status and priority counts follow boardctl stats defaults: fixture rows (fixtures.jsonl membership, NEVER-DONE ids, perpetual:true) are hidden until the fixtures toggle is on, matching stats --all. String and numeric priorities are distinct buckets, exactly as stored."));
}

// ---------- init ----------
function initFromHash(){
  renderBanner();
  renderFooter();
  var h = parseHash();
  if (boards.length >= 2) tabCmp.hidden = false;
  if (h.v === "compare" && boards.length >= 2) { showView("compare"); return; }
  if (h.b) { applyHash(); return; }
  showView("overview");
}
initFromHash();
})();
</script>
</body>
</html>
`
