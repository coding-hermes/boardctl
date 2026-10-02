package main

// uiPage is the BT-075 /ui document: one self-contained HTML file (R1),
// inline CSS + JS, zero external assets (R17/NG9), the R18 snapshot island
// embedded at @@ISLAND@@ (serialized by render.EscapeJSONIsland so
// </script> cannot occur inside it, R16) and the version string at
// @@VERSION@@ (R11 footer).
//
// Safety posture baked into the JS (R16): every payload value reaches the
// DOM through textContent / createTextNode / setAttribute; there is no
// innerHTML (nor insertAdjacentHTML) anywhere; the page never throws on
// bad input (render + hash handling are wrapped). Network confinement
// (R17): the only fetch calls are same-origin literals under /api/; there
// is no http:// or https:// literal in the inline JS — the SVG namespace
// for the hand-rolled charts is taken from the static template's
// namespaceURI property, never spelled out in script.
//
// The visual system is docs/web-ui-design.md §8 (tokens, chips, pills) and
// the interaction model is docs/web-ui-ux.md (hash grammar 1.1, flows,
// keyboard model 5.7, batching at 500 lifted under filters, pinned-row
// rule, offline banner copy).

const uiPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>boardctl — fleet boards</title>
<style>
:root{
--bg:#0d1117;--bg2:#161b22;--bg3:#1c2330;--line:#30363d;
--fg:#e6edf3;--muted:#8b949e;--accent:#58a6ff;
--green:#3fb950;--green-bright:#56d364;--red:#f85149;--red-bright:#ff7b72;
--orange:#d29922;--orange-bright:#e3b341;--blue:#58a6ff;--purple:#bc8cff;
--cyan:#56d4dd;--gray:#6e7681;--gray-bright:#9ea7b3;
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 -apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
a{color:var(--accent);text-decoration:none}
a:hover{text-decoration:underline}
button{background:var(--bg3);color:var(--fg);border:1px solid var(--line);border-radius:6px;padding:5px 12px;font-size:12px;cursor:pointer}
button:hover{border-color:var(--muted)}
input,select{background:var(--bg3);color:var(--fg);border:1px solid var(--line);border-radius:6px;padding:5px 8px;font-size:13px}
:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.skip{position:absolute;left:-9999px;top:0;background:var(--bg3);color:var(--fg);padding:8px 14px;z-index:20}
.skip:focus{left:8px;top:8px}
header#topbar{position:sticky;top:0;z-index:10;display:flex;align-items:center;gap:16px;padding:10px 18px;background:var(--bg2);border-bottom:1px solid var(--line);flex-wrap:wrap}
.wordmark{font-weight:700;font-size:16px}
.wordmark .uitag{color:var(--muted);font-weight:400;font-size:12px;margin-left:4px}
#tabs{display:flex;gap:6px}
.tab{background:transparent;border:1px solid transparent;color:var(--muted)}
.tab.active{color:var(--fg);border-color:var(--line);background:var(--bg3)}
.meta{color:var(--muted);font-size:12px;margin-left:auto;display:flex;gap:14px;flex-wrap:wrap;align-items:center}
#notices,#offline-banner{max-width:1180px;margin:12px auto 0;padding:0 16px}
.banner{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:10px 14px;font-size:13px;display:flex;gap:10px;align-items:baseline;flex-wrap:wrap}
.banner .muted{color:var(--muted)}
#offline-banner .banner{border-color:var(--orange)}
main#content{max-width:1180px;margin:0 auto;padding:16px}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}
.card{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:14px;position:relative}
.card h2{margin:0 0 8px;font-size:16px;display:flex;gap:8px;align-items:center;flex-wrap:wrap}
.card .slug{color:var(--muted);font-size:12px;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}
.kv{display:flex;gap:18px;margin:10px 0;flex-wrap:wrap}
.kv .k{color:var(--muted);font-size:12px}
.kv .v{font-size:18px;font-weight:600}
.chip{display:inline-block;font-size:12px;font-weight:600;border-radius:6px;padding:1px 8px;border:1px solid var(--line);color:var(--muted);background:color-mix(in srgb,var(--muted) 15%,var(--bg2))}
.chip[data-status="complete"]{color:var(--green);border-color:var(--green);background:color-mix(in srgb,var(--green) 15%,var(--bg2))}
.chip[data-status="pending"]{color:var(--muted);border-color:var(--muted)}
.chip[data-status="in_progress"]{color:var(--blue);border-color:var(--blue);background:color-mix(in srgb,var(--blue) 15%,var(--bg2))}
.chip[data-status="review"]{color:var(--purple);border-color:var(--purple);background:color-mix(in srgb,var(--purple) 15%,var(--bg2))}
.chip[data-status="blocked"]{color:var(--orange);border-color:var(--orange);background:color-mix(in srgb,var(--orange) 15%,var(--bg2))}
.chip[data-status="failed"]{color:var(--red-bright);border-color:var(--red);background:color-mix(in srgb,var(--red) 15%,var(--bg2))}
.chip.chip-unknown{color:var(--gray-bright);border-color:var(--gray);background:color-mix(in srgb,var(--gray) 15%,var(--bg2))}
.chip.chip-fixture{color:var(--gray-bright);border-color:var(--gray)}
.pchip{display:inline-block;font-size:12px;font-weight:600;border-radius:6px;padding:1px 8px;border:1px solid var(--line);color:var(--muted)}
.pchip[data-p="P0"]{color:var(--red-bright);border-color:var(--red)}
.pchip[data-p="P1"]{color:var(--orange);border-color:var(--orange)}
.pchip[data-p="P2"]{color:var(--blue);border-color:var(--blue)}
.pchip[data-p="P3"]{color:var(--purple);border-color:var(--purple)}
.pchip[data-p="P4"]{color:var(--cyan);border-color:var(--cyan)}
.pchip[data-p="P5"]{color:var(--muted);border-color:var(--muted)}
.pchip.p-unknown{color:var(--gray-bright);border-color:var(--gray)}
.pill{display:inline-block;font-size:12px;font-weight:700;border-radius:6px;padding:2px 10px;border:1px solid var(--line)}
.pill.pill-pass{color:var(--green-bright);border-color:var(--green);background:color-mix(in srgb,var(--green) 25%,var(--bg))}
.pill.pill-warn{color:var(--orange-bright);border-color:var(--orange);background:color-mix(in srgb,var(--orange) 25%,var(--bg))}
.pill.pill-fail{color:var(--red-bright);border-color:var(--red);background:color-mix(in srgb,var(--red) 25%,var(--bg))}
.badge{display:inline-block;font-size:12px;border:1px solid var(--line);border-radius:6px;padding:1px 8px;color:var(--muted)}
.badge.err{color:var(--red-bright);border-color:var(--red)}
.panel{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:14px 16px;margin:16px 0}
.panel h3{margin:0 0 10px;font-size:16px}
.panel h4{margin:14px 0 6px;font-size:13px;color:var(--fg)}
.muted{color:var(--muted)}
.mono{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,"Liberation Mono",monospace}
.census{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px;background:var(--bg3);border:1px solid var(--line);border-radius:6px;padding:6px 10px;margin:6px 0;white-space:pre-wrap;word-break:break-word}
.findings{margin:6px 0;padding:0;list-style:none}
.findings li{display:flex;gap:8px;align-items:baseline;padding:3px 0;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px;flex-wrap:wrap}
.lv{flex:0 0 auto;font-size:11px;font-weight:700;border-radius:4px;padding:0 6px;border:1px solid var(--line)}
.lv.error{color:var(--red-bright);border-color:var(--red)}
.lv.warn{color:var(--orange-bright);border-color:var(--orange)}
.toolbar{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin:10px 0}
.toolbar .count{color:var(--muted);font-size:12px}
.tablewrap{overflow-x:auto;border:1px solid var(--line);border-radius:10px}
table.rows{border-collapse:collapse;width:100%;font-size:13px}
table.rows th{color:var(--muted);font-weight:600;text-align:left;background:var(--bg2);border-bottom:1px solid var(--line);padding:6px 10px;font-size:12px}
table.rows td{border-bottom:1px solid var(--line);padding:6px 10px;vertical-align:top}
tr.trow{cursor:pointer}
tr.trow:hover td{background:var(--bg2)}
tr.trow .title{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:520px}
td.tid{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;white-space:nowrap}
.detail{background:var(--bg);padding:12px 16px}
.detail h5{margin:10px 0 4px;font-size:12px;color:var(--muted);text-transform:uppercase;letter-spacing:.05em}
.detail pre{background:var(--bg2);border:1px solid var(--line);border-radius:8px;padding:10px;overflow:auto;max-height:320px;font-size:12px;color:#c9d1d9}
.facts{display:flex;gap:8px;flex-wrap:wrap;margin:4px 0}
.empty{color:var(--muted);text-align:center;padding:24px 10px}
.charts{display:grid;grid-template-columns:repeat(auto-fit,minmax(320px,1fr));gap:16px;margin:12px 0}
.chartbox{background:var(--bg2);border:1px solid var(--line);border-radius:10px;padding:12px}
.chartbox h4{margin:0 0 8px;font-size:13px}
.chartbox svg{max-width:100%;height:auto}
.cq{color:var(--muted);font-size:12px;margin-top:6px}
ol.timeline{list-style:none;margin:8px 0;padding:0 0 0 14px;border-left:2px solid var(--line)}
ol.timeline li{padding:6px 0 6px 10px;position:relative}
ol.timeline li:before{content:"";position:absolute;left:-19px;top:12px;width:6px;height:6px;border-radius:50%;background:var(--gray);border:1px solid var(--line)}
.ev-meta{display:flex;gap:10px;align-items:baseline;flex-wrap:wrap;font-size:13px}
.ev-ts{color:var(--muted);font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:11px}
.ev-detail{margin:4px 0 0}
.ev-detail pre{background:var(--bg3);border:1px solid var(--line);border-radius:6px;padding:8px;overflow:auto;max-height:220px;font-size:12px;color:#c9d1d9;margin:0}
table.plain{border-collapse:collapse;font-size:13px;min-width:420px}
table.plain th,table.plain td{border:1px solid var(--line);padding:5px 10px;text-align:left}
table.plain th{color:var(--muted);font-weight:600;background:var(--bg2)}
.streakline{display:flex;gap:14px;flex-wrap:wrap;margin:8px 0;font-size:13px}
.errbox{background:var(--bg2);border:1px solid var(--red);border-radius:10px;padding:18px;margin:20px auto;max-width:560px;text-align:center}
.errbox .detail-line{color:var(--muted);font-size:13px;margin:8px 0 14px;word-break:break-word}
.crumb{font-size:13px;margin-bottom:8px}
footer#foot{max-width:1180px;margin:24px auto 0;padding:14px 16px 40px;border-top:1px solid var(--line);color:var(--muted);font-size:12px}
.btnrow{display:flex;gap:8px;margin-top:10px;flex-wrap:wrap}
.notice-line{background:var(--bg3);border:1px solid var(--line);border-radius:6px;padding:6px 10px;margin:6px 0;font-size:13px}
.spin{opacity:.55}
@media (max-width:720px){
header#topbar{gap:8px}
.meta{display:none}
}
</style>
</head>
<body>
<a class="skip" href="#content">skip to board list</a>
<header id="topbar">
  <span class="wordmark">boardctl<span class="uitag">/ui</span></span>
  <nav id="tabs" role="tablist" aria-label="views">
    <button id="tab-boards" class="tab active" role="tab" aria-selected="true">Boards</button>
    <button id="tab-compare" class="tab" role="tab" aria-selected="false" hidden>Compare</button>
  </nav>
  <span class="meta"><span id="server-meta"></span><button id="refresh" class="btn">refresh</button></span>
</header>
<div id="notices"></div>
<div id="offline-banner" hidden role="status" aria-live="polite"></div>
<main id="content">
  <section id="view-overview"></section>
  <section id="view-board" hidden></section>
  <section id="view-compare" hidden></section>
</main>
<footer id="foot"></footer>
<template id="svg-tpl"><svg xmlns="http://www.w3.org/2000/svg"></svg></template>
<script type="application/json" id="ui-data">@@ISLAND@@</script>
<script>
"use strict";
(function(){
// ---------- snapshot island (R18 floor) ----------
var payload;
try { payload = JSON.parse(document.getElementById("ui-data").textContent); }
catch(e){ fatal("the page could not render its snapshot: " + e); return; }
if (!payload || payload.schema !== "board-report/v1") { fatal("the page could not render its snapshot: unsupported payload"); return; }
var VERSION = "@@VERSION@@";
var boards = (payload.boards || []).filter(function(b){ return b && typeof b === "object"; });

var fresh = {};       // slug -> upgraded board detail from /api/board/{slug}
var freshEvents = {}; // slug -> decoded events from /api/events/{slug}
var state = { view:"overview", board:null, expanded:{}, showFixtures:false, q:"", status:"all",
              sort:"file", evType:"all", evTask:"", batchRows:{}, batchEvents:{}, selecting:{} };

// ---------- safe DOM helpers (R16: textContent / setAttribute only) ----------
function el(tag, cls, text){
  var n = document.createElement(tag);
  if (cls) n.className = cls;
  if (text !== undefined && text !== null) n.textContent = String(text);
  return n;
}
function svgNS(){
  var tpl = document.getElementById("svg-tpl");
  var proto = tpl && tpl.content && tpl.content.firstElementChild;
  return proto ? proto.namespaceURI : null;
}
var NS = svgNS();
function svgel(tag){
  if (NS) return document.createElementNS(NS, tag);
  return document.createElement(tag); // namespace-less fallback: charts degrade, page never throws
}
function clear(n){ while (n.firstChild) n.removeChild(n.firstChild); }
function fmtPct(a, b){ return b > 0 ? Math.round(100 * a / b) + "%" : "—"; }
function streakFmt(n, suffix){
  var x = (n === undefined || n === null) ? 0 : n;
  if (x <= 0) return "0 (no activity)";
  return x + "d" + (suffix || "");
}
function taskStatus(row){
  var s = row ? row.status : "";
  if (s === "completed") s = "complete"; // BT-025 read alias, same as the report
  return s ? s : "(none)";
}
function isFixtureRow(b, row){
  var id = (row && row.id) || "";
  if ((b.fixture_ids || []).indexOf(id) >= 0) return true;
  if (id.indexOf("NEVER-DONE") === 0) return true;
  return row.perpetual === true;
}
function boardTasks(b){ return Array.isArray(b.tasks) ? b.tasks : []; }
function boardEvents(b){ return Array.isArray(b.events) ? b.events : []; }
function boardBySlug(slug){
  for (var i = 0; i < boards.length; i++) if (boards[i].slug === slug) return boards[i];
  return null;
}
function healthyBoards(){ return boards.filter(function(b){ return !b.error; }); }
function boardData(slug){ return fresh[slug] || boardBySlug(slug) || null; }
function eventsOf(slug){
  if (freshEvents[slug]) return freshEvents[slug];
  var b = boardBySlug(slug);
  return b ? boardEvents(b).map(function(e){ return { id:e.id, timestamp:e.timestamp, event_type:e.event_type, task_id:e.task_id, actor:e.actor, detail:e.detail, tick_number:e.tick_number }; }) : [];
}
function loadedAtOf(b){ return (fresh[b.slug] && fresh[b.slug].loaded_at) || b.loaded_at || ""; }

// ---------- boundary (never a blank page, R18 / 5.7) ----------
function fatal(msg){
  var v = document.getElementById("view-overview");
  if (!v) return;
  clear(v);
  var box = el("div", "errbox");
  box.setAttribute("role", "alert");
  box.appendChild(el("strong", null, msg));
  var retry = el("button", "btn", "retry");
  retry.addEventListener("click", function(){ location.reload(); });
  var up = el("a", null, "upload a board →");
  up.setAttribute("href", "/");
  var row = el("div", "btnrow");
  row.appendChild(retry);
  var link = el("span", null, "  ·  ");
  link.appendChild(up);
  row.appendChild(link);
  box.appendChild(row);
  v.appendChild(box);
}
function boundary(fn){
  return function(){
    try { return fn.apply(null, arguments); }
    catch(e){ fatal("the page could not render its snapshot: " + e); }
  };
}

// ---------- chips + pills ----------
var CANON = { complete:1, pending:1, in_progress:1, review:1, blocked:1, failed:1 };
function chip(status){
  var c = el("span", "chip", status);
  c.setAttribute("data-status", status);
  if (!CANON[status]) c.className = "chip chip-unknown";
  return c;
}
function pchip(p){
  var c = el("span", "pchip", p === "" || p === null || p === undefined ? "—" : String(p));
  var v = (p === null || p === undefined) ? "" : String(p);
  c.setAttribute("data-p", v);
  if (!/^P[0-5]$/.test(v)) c.className = "pchip p-unknown";
  return c;
}
function pill(v){
  var uv = { PASS:"pass", WARN:"warn", FAIL:"fail" }[v] || "pass";
  return el("span", "pill pill-" + uv, v || "PASS");
}

// ---------- SVG chart helpers (hand-rolled, payload-driven, NG8) ----------
function svg(w, h, label){
  var s = svgel("svg");
  s.setAttribute("width", w); s.setAttribute("height", h);
  s.setAttribute("viewBox", "0 0 " + w + " " + h);
  s.setAttribute("preserveAspectRatio", "xMidYMid meet");
  s.setAttribute("role", "img");
  if (label) s.setAttribute("aria-label", label);
  return s;
}
function stag(s, x, y, txt, anchor){
  var t = svgel("text");
  t.setAttribute("x", x); t.setAttribute("y", y);
  t.setAttribute("style", "fill:var(--muted);font-size:9px");
  t.setAttribute("text-anchor", anchor || "start");
  t.textContent = txt;
  s.appendChild(t);
}
function sline(s, x1, x2, y){
  var l = svgel("line");
  l.setAttribute("x1", x1); l.setAttribute("x2", x2); l.setAttribute("y1", y); l.setAttribute("y2", y);
  l.setAttribute("stroke", "var(--line)"); l.setAttribute("stroke-width", ".5");
  s.appendChild(l);
}
function linePath(xs, ys, x, y, w, h){
  var d = "";
  for (var i = 0; i < xs.length; i++) {
    d += (i === 0 ? "M" : "L") + (x + w * xs[i]).toFixed(2) + " " + (y + h - h * ys[i]).toFixed(2);
  }
  return d;
}
function emptyChart(msg, w, h){
  var s = svg(w || 400, h || 120, "empty chart");
  stag(s, (w || 400) / 2, (h || 120) / 2, msg, "middle");
  return s;
}
function chartLines(series, label, emptyMsg){
  var W = 460, H = 170, pad = { l: 30, r: 8, t: 10, b: 22 };
  var days = series[0] ? series[0].days : [];
  if (!days || !days.length) return emptyChart(emptyMsg, W, H);
  var max = 1;
  series.forEach(function(sr){ (sr.values || []).forEach(function(v){ if (v > max) max = v; }); });
  var s = svg(W, H, label);
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  [0, .25, .5, .75, 1].forEach(function(g){
    sline(s, pad.l, W - pad.r, pad.t + ih * (1 - g));
    stag(s, pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "end");
  });
  series.forEach(function(sr){
    if (!sr.days || !sr.days.length) return;
    var xs = sr.days.map(function(_, i){ return sr.days.length === 1 ? 0 : i / (sr.days.length - 1); });
    var ys = (sr.values || []).map(function(v){ return v / max; });
    var p = svgel("path");
    p.setAttribute("d", linePath(xs, ys, pad.l, pad.t, iw, ih));
    p.setAttribute("fill", "none");
    p.setAttribute("stroke", sr.color);
    p.setAttribute("stroke-width", "1.6");
    s.appendChild(p);
  });
  stag(s, pad.l, H - 6, days[0]);
  stag(s, W - pad.r, H - 6, days[days.length - 1], "end");
  return s;
}
function chartBars(cats, values, label, emptyMsg, color){
  var W = 460, H = 170, pad = { l: 30, r: 8, t: 10, b: 26 };
  if (!cats.length) return emptyChart(emptyMsg, W, H);
  var max = 1; values.forEach(function(v){ if (v > max) max = v; });
  var s = svg(W, H, label);
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  var n = cats.length;
  var bw = Math.max(2, iw / n * 0.72);
  for (var i = 0; i < n; i++) {
    var x = pad.l + iw * (i + 0.5) / n - bw / 2;
    var hgt = ih * values[i] / max;
    var r = svgel("rect");
    r.setAttribute("x", x.toFixed(1));
    r.setAttribute("y", (pad.t + ih - hgt).toFixed(1));
    r.setAttribute("width", bw.toFixed(1));
    r.setAttribute("height", Math.max(hgt, values[i] > 0 ? 1 : 0).toFixed(1));
    r.setAttribute("fill", color || "var(--accent)");
    r.setAttribute("rx", "1");
    if (values[i] > 0) {
      var ti = svgel("title");
      ti.textContent = cats[i] + ": " + values[i];
      r.appendChild(ti);
    }
    s.appendChild(r);
  }
  [0, .5, 1].forEach(function(g){
    sline(s, pad.l, W - pad.r, pad.t + ih * (1 - g));
    stag(s, pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "end");
  });
  var step = Math.ceil(n / 6);
  for (var j = 0; j < n; j += step) stag(s, pad.l + iw * (j + 0.5) / n, H - 6, String(cats[j]), "middle");
  return s;
}
function chartTickHealth(th){
  var W = 460, H = 170, pad = { l: 30, r: 8, t: 10, b: 26 };
  th = th || {};
  var days = th.days || [];
  if (!days.length) return emptyChart("no events", W, H);
  var max = 1;
  for (var i = 0; i < days.length; i++) max = Math.max(max, (th.completed[i] || 0) + (th.failed[i] || 0) + (th.audit[i] || 0));
  var s = svg(W, H, "tick health per day");
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  var n = days.length;
  var bw = Math.max(2, iw / n * 0.7);
  for (var k = 0; k < n; k++) {
    var segs = [["var(--green)", th.completed[k] || 0], ["var(--red)", th.failed[k] || 0], ["var(--purple)", th.audit[k] || 0]];
    var acc = 0;
    segs.forEach(function(sg){
      if (!sg[1]) return;
      var h1 = ih * sg[1] / max;
      var r = svgel("rect");
      r.setAttribute("x", (pad.l + iw * (k + 0.5) / n - bw / 2).toFixed(1));
      r.setAttribute("y", (pad.t + ih - (acc + h1)).toFixed(1));
      r.setAttribute("width", bw.toFixed(1));
      r.setAttribute("height", Math.max(h1, 0.8).toFixed(1));
      r.setAttribute("fill", sg[0]);
      r.setAttribute("rx", "1");
      var ti = svgel("title");
      ti.textContent = days[k] + ": " + sg[1];
      r.appendChild(ti);
      s.appendChild(r);
      acc += h1;
    });
  }
  [0, .5, 1].forEach(function(g){
    sline(s, pad.l, W - pad.r, pad.t + ih * (1 - g));
    stag(s, pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(max * g), "end");
  });
  var step = Math.ceil(n / 6);
  for (var j = 0; j < n; j += step) stag(s, pad.l + iw * (j + 0.5) / n, H - 6, String(days[j]), "middle");
  return s;
}
var WDLABELS = ["Mon","Tue","Wed","Thu","Fri","Sat","Sun"];
function chartWorkClock(wc){
  wc = wc || { grid: [], max: 0 };
  var cw = 22, ch = 16, lx = 30, ty = 4;
  var W = lx + 24 * cw + 10, H = ty + 7 * ch + 18;
  var s = svg(W, H, "work clock weekday by hour heatmap");
  var grid = wc.grid || [];
  function color(v){
    if (v <= 0) return "var(--bg3)";
    if (v >= 8) return "#58a6ff";
    if (v >= 4) return "#58a6ffcc";
    if (v >= 2) return "#1f6feb99";
    return "#1f6feb55";
  }
  for (var d = 0; d < 7; d++) {
    stag(s, lx - 4, ty + d * ch + ch / 2 + 3, WDLABELS[d], "end");
    var row = grid[d] || [];
    for (var h = 0; h < 24; h++) {
      var v = row[h] || 0;
      var r = svgel("rect");
      r.setAttribute("x", lx + h * cw);
      r.setAttribute("y", ty + d * ch);
      r.setAttribute("width", cw - 2);
      r.setAttribute("height", ch - 2);
      r.setAttribute("rx", "2");
      r.setAttribute("fill", color(v));
      var ti = svgel("title");
      ti.textContent = WDLABELS[d] + " " + h + ":00 — " + v + " events";
      r.appendChild(ti);
      s.appendChild(r);
    }
  }
  for (var hh = 0; hh < 24; hh += 4) stag(s, lx + hh * cw + cw / 2, H - 4, String(hh), "middle");
  return s;
}
function chartModelShare(ms){
  ms = ms || { models: [], counts: [], completed_n: 0 };
  var W = 460, rh = 22, lx = 8;
  if (!ms.models || !ms.models.length) return emptyChart("no completed tasks", W, 40);
  var H = 30 + ms.models.length * rh;
  var s = svg(W, H, "model share across completed tasks");
  var max = 1; ms.counts.forEach(function(c){ if (c > max) max = c; });
  var barW = W - lx - 130;
  ms.models.forEach(function(m, i){
    var y = 24 + i * rh;
    var w = barW * ms.counts[i] / max;
    var r = svgel("rect");
    r.setAttribute("x", lx); r.setAttribute("y", y);
    r.setAttribute("width", Math.max(w, 2)); r.setAttribute("height", rh - 8);
    r.setAttribute("rx", "3"); r.setAttribute("fill", "var(--accent)");
    var ti = svgel("title");
    ti.textContent = m + ": " + ms.counts[i] + " of " + ms.completed_n;
    r.appendChild(ti);
    s.appendChild(r);
    var t = svgel("text");
    t.setAttribute("x", lx + 6); t.setAttribute("y", y + rh - 14);
    t.setAttribute("style", "fill:#0d1117;font-size:10px;font-weight:600");
    t.textContent = m + "  " + ms.counts[i] + " (" + fmtPct(ms.counts[i], ms.completed_n) + ")";
    if (w <= 150) { t.setAttribute("x", lx + w + 8); t.setAttribute("style", "fill:var(--muted);font-size:10px"); }
    s.appendChild(t);
  });
  return s;
}
function sparkline(cum, w, h){
  var s = svg(w, h, "burn-up sparkline");
  if (!cum || !cum.length) { stag(s, w / 2, h / 2 + 3, "no data", "middle"); return s; }
  var max = 1; cum.forEach(function(v){ if (v > max) max = v; });
  var xs = cum.map(function(_, i){ return cum.length === 1 ? 0 : i / (cum.length - 1); });
  var ys = cum.map(function(v){ return v / max; });
  var p = svgel("path");
  p.setAttribute("d", linePath(xs, ys, 1, 1, w - 2, h - 2));
  p.setAttribute("fill", "none");
  p.setAttribute("stroke", "var(--green)");
  p.setAttribute("stroke-width", "1.5");
  s.appendChild(p);
  return s;
}

// ---------- hash grammar (UX 1.1) ----------
function parseHash(){
  var h = location.hash.replace(/^#/, "");
  var o = {};
  h.split("&").forEach(function(p){
    if (!p) return;
    var kv = p.split("=");
    try {
      o[decodeURIComponent(kv[0])] = decodeURIComponent(kv.slice(1).join("="));
    } catch(e) { /* unrecognized param ignored (defensive hashchange) */ }
  });
  return o;
}
function buildHash(){
  var parts = [];
  if (state.view === "board" && state.board) {
    parts.push("b=" + encodeURIComponent(state.board));
    var ex = Object.keys(state.expanded).filter(function(k){ return state.expanded[k]; });
    if (ex.length === 1) parts.push("t=" + encodeURIComponent(ex[0]));
    if (state.q) parts.push("q=" + encodeURIComponent(state.q));
    if (state.status !== "all") parts.push("s=" + encodeURIComponent(state.status));
    if (state.evTask) parts.push("ev=" + encodeURIComponent(state.evTask));
  }
  if (state.view === "compare") parts.push("compare");
  return parts.length ? "#" + parts.join("&") : "#";
}
window.addEventListener("hashchange", boundary(function(){ applyHash(); }));

function showView(v){
  state.view = v;
  document.getElementById("view-overview").hidden = v !== "overview";
  document.getElementById("view-board").hidden = v !== "board";
  document.getElementById("view-compare").hidden = v !== "compare";
  var tb = document.getElementById("tab-boards");
  var tc = document.getElementById("tab-compare");
  tb.classList.toggle("active", v !== "compare");
  tb.setAttribute("aria-selected", v !== "compare" ? "true" : "false");
  tc.classList.toggle("active", v === "compare");
  tc.setAttribute("aria-selected", v === "compare" ? "true" : "false");
  if (v === "overview") renderOverview();
  if (v === "board") renderBoardPage();
  if (v === "compare") renderCompare();
}
document.getElementById("tab-boards").addEventListener("click", function(){ state.view = "overview"; state.board = null; location.hash = "#"; showView("overview"); });
document.getElementById("tab-compare").addEventListener("click", function(){ location.hash = "#compare"; showView("compare"); });

function applyHash(){
  var h = parseHash();
  if (h.compare !== undefined) {
    if (healthyBoards().length >= 2) { showView("compare"); return; }
    showView("overview");
    inlineNote(document.getElementById("view-overview"), "compare needs at least two boards — showing the overview");
    return;
  }
  if (h.b) {
    var b = boardBySlug(h.b);
    if (!b) { boardError(h.b); return; }
    state.board = h.b;
    state.expanded = {};
    state.evTask = h.ev || "";
    state.q = h.q || "";
    state.status = h.s || "all";
    if (h.t) state.expanded[h.t] = true;
    showView("board");
    upgradeBoard(h.b);
    if (h.t) {
      var anchor = document.getElementById("task-" + cssEscape(h.t));
      if (anchor) { anchor.scrollIntoView({ block: "start" }); anchor.focus(); }
    }
    return;
  }
  showView("overview");
}
function cssEscape(s){
  return (window.CSS && CSS.escape) ? CSS.escape(s) : String(s).replace(/["\\]/g, "\\$&");
}
function inlineNote(host, msg){
  if (!host) return;
  host.appendChild(el("div", "notice-line", msg));
}

// ---------- header meta + notices + footer ----------
function renderMeta(){
  var host = document.getElementById("server-meta");
  clear(host);
  host.appendChild(el("span", null, location.host || "loopback"));
  host.appendChild(el("span", null, boards.length + " board" + (boards.length === 1 ? "" : "s")));
  var freshest = "";
  boards.forEach(function(b){
    var la = loadedAtOf(b);
    if (la > freshest) freshest = la;
  });
  if (freshest) host.appendChild(el("span", null, "snapshot " + freshest));
}
function renderNotices(){
  var host = document.getElementById("notices");
  clear(host);
  var parts = [];
  boards.forEach(function(b){
    var w = b.parse_warnings || {};
    var n = (w.tasks || 0) + (w.events || 0) + (w.fixtures || 0) + (w.timestamp_excluded || 0) + (w.duplicate_ids || 0);
    if (n > 0) parts.push(b.name + ": " + n + " parse warnings");
  });
  if (!parts.length) return;
  var banner = el("div", "banner");
  banner.appendChild(el("strong", null, "Loaded with parse warnings"));
  banner.appendChild(el("span", "muted", " — " + parts.join("; ")));
  var dismiss = el("button", "btn", "dismiss");
  dismiss.addEventListener("click", function(){ clear(host); });
  banner.appendChild(dismiss);
  host.appendChild(banner);
}
function showOffline(){
  var host = document.getElementById("offline-banner");
  clear(host);
  var freshest = "";
  boards.forEach(function(b){ var la = loadedAtOf(b); if (la > freshest) freshest = la; });
  var banner = el("div", "banner");
  banner.appendChild(el("strong", null, "API unreachable — showing the snapshot embedded at load (" + (freshest || "unknown time") + "). Refresh = re-upload or restart serve."));
  var dismiss = el("button", "btn", "dismiss");
  dismiss.addEventListener("click", function(){ host.hidden = true; });
  banner.appendChild(dismiss);
  host.appendChild(banner);
  host.hidden = false;
}
function renderFooter(){
  var f = document.getElementById("foot");
  clear(f);
  f.appendChild(el("div", null, "boardctl /ui · version " + VERSION + " · loopback-only · read-only snapshot"));
  f.appendChild(el("div", null, "served by boardctl serve — the uploader stays at "));
  var up = el("a", null, "/");
  up.setAttribute("href", "/");
  f.lastChild.appendChild(up);
  f.appendChild(el("div", null, "upload a board →"));
  var up2 = f.lastChild;
  up2.textContent = "";
  var upLink = el("a", null, "upload a board →");
  upLink.setAttribute("href", "/");
  up2.appendChild(upLink);
}

// ---------- S0 overview ----------
function renderOverview(){
  var host = document.getElementById("view-overview");
  clear(host);
  renderMeta(); renderNotices(); renderFooter();
  if (!boards.length) {
    host.appendChild(el("div", "empty", "no boards loaded"));
    return;
  }
  var grid = el("div", "grid");
  boards.forEach(function(b){
    var d = b.derived || {};
    if (b.error) {
      var ecard = el("div", "card");
      ecard.setAttribute("aria-label", "board " + b.name + " failed to load");
      var eh = el("h2");
      eh.appendChild(el("span", null, b.name));
      eh.appendChild(el("span", "badge err", "error"));
      ecard.appendChild(eh);
      ecard.appendChild(el("pre", "muted", String(b.error)));
      grid.appendChild(ecard);
      return;
    }
    var card = el("div", "card");
    card.setAttribute("role", "button");
    card.setAttribute("tabindex", "0");
    card.setAttribute("aria-label", "open board " + b.name);
    var h = el("h2");
    h.appendChild(el("span", null, b.name));
    h.appendChild(el("span", "badge", "topology " + b.topology));
    card.appendChild(h);
    var slugLine = el("div", "slug", b.slug);
    card.appendChild(slugLine);
    card.appendChild(sparkline((d.burnup || {}).cum || [], 120, 28));
    var v = b.validate || { pill: "PASS" };
    var pillRow = el("div", "facts");
    pillRow.appendChild(pill(v.pill));
    pillRow.appendChild(el("span", "muted", v.errors + " error" + (v.errors === 1 ? "" : "s") + " · " + v.warnings + " warning" + (v.warnings === 1 ? "" : "s")));
    card.appendChild(pillRow);
    var kv = el("div", "kv");
    function kblock(k, val){
      var blk = el("div");
      blk.appendChild(el("div", "k", k));
      blk.appendChild(el("div", "v", String(val)));
      return blk;
    }
    kv.appendChild(kblock("tasks", b.derived ? d.total_nonfixture : 0));
    kv.appendChild(kblock("done", d.complete_count || 0));
    kv.appendChild(kblock("open", d.open_count || 0));
    card.appendChild(kv);
    if (loadedAtOf(b)) card.appendChild(el("div", "muted", "snapshot loaded " + loadedAtOf(b)));
    function open(){
      state.board = b.slug; state.expanded = {}; state.q = ""; state.status = ""; state.evTask = "";
      location.hash = buildHash();
      showView("board");
      upgradeBoard(b.slug);
    }
    card.addEventListener("click", open);
    card.addEventListener("keydown", function(e){ if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); } });
    grid.appendChild(card);
  });
  host.appendChild(grid);
}

// ---------- S1 board page ----------
function boardError(slug){
  state.board = null;
  showView("board");
  var host = document.getElementById("view-board");
  clear(host);
  var box = el("div", "errbox");
  box.setAttribute("role", "alert");
  box.appendChild(el("strong", null, "this board could not be loaded"));
  box.appendChild(el("div", "detail-line", "board not found: " + slug));
  var back = el("button", "btn", "back to overview");
  back.addEventListener("click", function(){ location.hash = "#"; showView("overview"); });
  box.appendChild(back);
  host.appendChild(box);
}

function rowMatches(bd, row){
  if (!state.showFixtures && isFixtureRow(bd, row)) return false;
  if (state.status && state.status !== "all" && taskStatus(row) !== state.status) return false;
  if (state.q) {
    var q = state.q.toLowerCase();
    var hay = [row.id, row.title, row.worker_summary, row.commit_hash].map(function(x){ return (x === undefined || x === null) ? "" : String(x).toLowerCase(); });
    if (!hay.some(function(hs){ return hs.indexOf(q) >= 0; })) return false;
  }
  return true;
}
function sortRows(rows){
  var out = rows.slice();
  if (state.sort === "id") out.sort(function(a, b){ return String(a.id) < String(b.id) ? -1 : String(a.id) > String(b.id) ? 1 : 0; });
  if (state.sort === "p05" || state.sort === "p50") {
    out.sort(function(a, b){
      var pa = a.priority === undefined || a.priority === null ? "" : String(a.priority);
      var pb = b.priority === undefined || b.priority === null ? "" : String(b.priority);
      var c = pa < pb ? -1 : pa > pb ? 1 : 0;
      return state.sort === "p05" ? c : -c;
    });
  }
  return out;
}

function renderBoardPage(){
  var host = document.getElementById("view-board");
  clear(host);
  renderMeta(); renderFooter();
  var b = boardData(state.board);
  if (!b) { boardError(state.board || ""); return; }
  var crumb = el("div", "crumb");
  var back = el("a", null, "← all boards");
  back.setAttribute("href", "#");
  back.addEventListener("click", function(e){ e.preventDefault(); location.hash = "#"; showView("overview"); });
  crumb.appendChild(back);
  host.appendChild(crumb);

  if (b.error) {
    var ebox = el("div", "errbox");
    ebox.appendChild(el("strong", null, "this board could not be loaded"));
    ebox.appendChild(el("pre", "detail-line", String(b.error)));
    host.appendChild(ebox);
    return;
  }
  var d = b.derived || {};
  var title = el("h2", null, b.name + " ");
  title.style.fontSize = "20px";
  title.appendChild(el("span", "badge", "topology " + b.topology));
  if (b.header && b.header.namespace) title.appendChild(el("span", "badge", String(b.header.namespace)));
  host.appendChild(title);

  // header strip (R12: project, namespace, ticks, last_commit — omitted honestly when absent)
  var strip = el("div", "facts");
  function hfact(k, v){ if (v !== undefined && v !== null && v !== "") strip.appendChild(el("span", "badge", k + ": " + v)); }
  hfact("project", b.header && b.header.project);
  hfact("namespace", b.header && b.header.namespace);
  hfact("ticks_total", b.header && b.header.ticks_total);
  hfact("ticks_idle", b.header && b.header.ticks_idle);
  hfact("last_commit", b.header && b.header.last_commit);
  if (strip.childNodes.length) host.appendChild(strip);

  // key numbers
  var kv = el("div", "kv");
  function kblock(k, val){
    var blk = el("div");
    blk.appendChild(el("div", "k", k));
    blk.appendChild(el("div", "v", String(val)));
    return blk;
  }
  kv.appendChild(kblock("tasks", d.total_nonfixture || 0));
  kv.appendChild(kblock("open", d.open_count || 0));
  kv.appendChild(kblock("complete", d.complete_count || 0));
  kv.appendChild(kblock("completion", fmtPct(d.complete_count || 0, (d.complete_count || 0) + (d.open_count || 0))));
  kv.appendChild(kblock("median cycle", d.cycle && d.cycle.median_days !== null && d.cycle.median_days !== undefined ? d.cycle.median_days + "d" : "—"));
  kv.appendChild(kblock("p90 cycle", d.cycle && d.cycle.p90_days !== null && d.cycle.p90_days !== undefined ? d.cycle.p90_days + "d" : "—"));
  kv.appendChild(kblock("last activity", d.last_activity_day || "—"));
  host.appendChild(kv);

  // streaks (0 -> "0 (no activity)")
  var st = d.streaks || {};
  var dl = st.delivery || {}, ac = st.activity || {}, at = st.any_tick || {};
  var sl = el("div", "streakline");
  sl.appendChild(el("strong", null, "Delivery streak: " + streakFmt(dl.current, dl.current_state === "live" ? " (live)" : " (ended)")));
  sl.appendChild(el("span", "muted", "Activity streak: " + streakFmt(ac.current, " (longest " + (ac.longest || 0) + "d)") + " · raw any-tick: " + streakFmt(at.current, " current") + " / " + streakFmt(at.longest, " longest")));
  host.appendChild(sl);

  buildToolbar(b, host);

  // row table (R12: five collapsed columns; expansion from the payload, no refetch)
  var wrap = el("div", "tablewrap");
  var tbl = el("table", "rows");
  tbl.setAttribute("id", "task-table");
  wrap.appendChild(tbl);
  host.appendChild(wrap);
  renderTable(b);

  renderValidation(b, host);

  // analytics (R15: payload-driven SVG, no client-side derivations)
  var charts = el("div", "charts");
  function box(titleTxt, node, question){
    var c = el("div", "chartbox");
    c.appendChild(el("h4", null, titleTxt));
    if (node) c.appendChild(node);
    if (question) c.appendChild(el("div", "cq", question));
    return c;
  }
  charts.appendChild(box("Burndown — open tasks per day", chartLines([{ days: (d.burndown || {}).days || [], values: (d.burndown || {}).open || [], color: "var(--orange)" }], "burndown", "no tasks yet"), "Is the backlog actually shrinking?"));
  charts.appendChild(box("Burn-up — cumulative completions", chartLines([{ days: (d.burnup || {}).days || [], values: (d.burnup || {}).cum || [], color: "var(--green)" }], "burn-up", "no completions yet"), "How much real work has this board delivered?"));
  charts.appendChild(box("Weekly velocity", chartBars((d.velocity || {}).weeks || [], (d.velocity || {}).counts || [], "weekly velocity", "no completions yet"), "More or fewer completions per week over time?"));
  charts.appendChild(box("Tick health per day", chartTickHealth(d.tick_health), "Were the ticks healthy, or throwing audits and failures?"));
  charts.appendChild(box("Work clock — weekday × hour", chartWorkClock(d.work_clock), "Which hours and days carry the load?"));
  charts.appendChild(box("Model share — completed tasks", chartModelShare(d.model_share), "Which worker models do the delivery?"));
  host.appendChild(charts);

  renderCounts(b, host);
  renderEvents(b, host);

  // per-view footer strip (R9: snapshot age honest, refresh semantics visible)
  var stripFoot = el("div", "muted");
  stripFoot.style.margin = "14px 0";
  stripFoot.textContent = "snapshot loaded " + (loadedAtOf(b) || "unknown time") + " · Refresh = re-upload or restart serve";
  host.appendChild(stripFoot);
}

function buildToolbar(b, host){
  var tb = el("div", "toolbar");
  var q = el("input");
  q.type = "text";
  q.placeholder = "filter id / title / summary / commit";
  q.value = state.q;
  q.setAttribute("aria-label", "filter tasks");
  q.setAttribute("id", "task-search");
  var deb = null;
  q.addEventListener("input", function(){
    window.clearTimeout(deb);
    deb = window.setTimeout(function(){
      state.q = q.value;
      renderTable(b);
      history.replaceState(null, "", buildHash());
    }, 150);
  });
  tb.appendChild(q);
  var sel = el("select");
  sel.setAttribute("aria-label", "filter by status");
  var statuses = {};
  boardTasks(b).forEach(function(r){
    if (!state.showFixtures && isFixtureRow(b, r)) return;
    statuses[taskStatus(r)] = true;
  });
  var opts = ["all"].concat(Object.keys(statuses).sort());
  opts.forEach(function(o){
    var op = el("option", null, o);
    op.setAttribute("value", o);
    sel.appendChild(op);
  });
  sel.value = opts.indexOf(state.status) >= 0 ? state.status : "all";
  sel.addEventListener("change", function(){ state.status = sel.value; renderTable(b); history.replaceState(null, "", buildHash()); });
  tb.appendChild(sel);
  var fx = el("input");
  fx.type = "checkbox";
  fx.checked = state.showFixtures;
  var fxl = el("label", null, " show fixtures");
  fxl.insertBefore(fx, fxl.firstChild);
  fx.addEventListener("change", function(){ state.showFixtures = fx.checked; renderTable(b); });
  tb.appendChild(fxl);
  var sort = el("select");
  sort.setAttribute("aria-label", "sort rows");
  [["file", "file order"], ["id", "id A→Z"], ["p05", "priority P0→P5"], ["p50", "priority P5→P0"]].forEach(function(o){
    var op = el("option", null, o[1]);
    op.setAttribute("value", o[0]);
    sort.appendChild(op);
  });
  sort.value = state.sort;
  sort.addEventListener("change", function(){ state.sort = sort.value; renderTable(b); });
  tb.appendChild(sort);
  var ea = el("button", "btn", "Expand all");
  ea.addEventListener("click", function(){ boardTasks(b).forEach(function(r){ state.expanded[r.id] = true; }); renderTable(b); });
  var ca = el("button", "btn", "Collapse all");
  ca.addEventListener("click", function(){ state.expanded = {}; renderTable(b); });
  tb.appendChild(ea); tb.appendChild(ca);
  var count = el("span", "count");
  count.setAttribute("id", "rowcount");
  count.setAttribute("aria-live", "polite");
  tb.appendChild(count);
  host.appendChild(tb);
}

var BATCH = 500;
function renderTable(b){
  var tbl = document.getElementById("task-table");
  if (!tbl) return;
  clear(tbl);
  var rows = boardTasks(b);
  var visible = sortRows(rows.filter(function(r){ return rowMatches(b, r); }));
  var pinnedId = Object.keys(state.expanded).filter(function(k){ return state.expanded[k]; })[0] || "";
  var pinned = "";
  if (pinnedId) {
    var prow = null;
    rows.forEach(function(r){ if (r.id === pinnedId) prow = r; });
    if (prow && visible.indexOf(prow) < 0) { visible.push(prow); pinned = " · 1 pinned by link"; }
  }
  var rc = document.getElementById("rowcount");
  if (rc) rc.textContent = rows.length + " rows · " + visible.length + " shown" + pinned + (state.showFixtures ? " · fixtures shown" : " · fixtures hidden");
  var filtered = state.q || (state.status && state.status !== "all") || state.sort !== "file" || state.showFixtures;
  var shown = visible;
  if (!filtered) {
    var n = state.batchRows[b.slug] || BATCH;
    if (visible.length > n) shown = visible.slice(0, n);
  }
  var hr = el("tr");
  ["id", "title", "status", "priority", "depends_on"].forEach(function(h){
    var c = el("th", null, h);
    c.setAttribute("scope", "col");
    hr.appendChild(c);
  });
  tbl.appendChild(hr);
  if (!visible.length) {
    var er = el("tr");
    var ed = el("td", "empty", rows.length ? "no match for the current filters" : "no tasks yet");
    ed.setAttribute("colspan", "5");
    if (rows.length) {
      var clearBtn = el("button", "btn", "clear filters");
      clearBtn.addEventListener("click", function(){ state.q = ""; state.status = "all"; state.showFixtures = false; renderBoardPage(); });
      ed.appendChild(document.createTextNode(" "));
      ed.appendChild(clearBtn);
    }
    er.appendChild(ed);
    tbl.appendChild(er);
    return;
  }
  shown.forEach(function(row){ appendRow(tbl, b, row); });
  if (!filtered && visible.length > shown.length) {
    var more = el("tr");
    var mored = el("td", "empty");
    mored.setAttribute("colspan", "5");
    var btn = el("button", "btn", "show " + Math.min(BATCH, visible.length - shown.length) + " more");
    btn.addEventListener("click", function(){
      state.batchRows[b.slug] = (state.batchRows[b.slug] || BATCH) + BATCH;
      renderTable(b);
    });
    mored.appendChild(btn);
    more.appendChild(mored);
    tbl.appendChild(more);
  }
}
function appendRow(tbl, b, row){
  var expanded = !!state.expanded[row.id];
  var tr = el("tr", "trow");
  tr.setAttribute("id", "task-" + row.id);
  tr.setAttribute("role", "button");
  tr.setAttribute("tabindex", "0");
  tr.setAttribute("aria-expanded", expanded ? "true" : "false");
  tr.appendChild(el("td", "tid", row.id || ""));
  var tdTitle = el("td");
  var t = row.title === undefined || row.title === null ? "" : String(row.title);
  var span = el("span", "title", t.length > 100 ? t.slice(0, 99) + "…" : t);
  tdTitle.appendChild(span);
  if (isFixtureRow(b, row)) tdTitle.appendChild(el("span", "chip chip-fixture", "fixture"));
  tr.appendChild(tdTitle);
  var tdStatus = el("td");
  tdStatus.appendChild(chip(taskStatus(row)));
  tr.appendChild(tdStatus);
  var tdPrio = el("td");
  tdPrio.appendChild(pchip(row.priority));
  tr.appendChild(tdPrio);
  var deps = Array.isArray(row.depends_on) ? row.depends_on.length : 0;
  tr.appendChild(el("td", null, String(deps)));
  function toggle(){
    state.expanded[row.id] = !expanded;
    history.replaceState(null, "", buildHash());
    renderTable(b);
    var again = document.getElementById("task-" + cssEscape(row.id));
    if (again) again.focus();
  }
  tr.addEventListener("click", toggle);
  tr.addEventListener("keydown", function(e){
    if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggle(); }
    if (e.key === "Escape" && expanded) { state.expanded[row.id] = false; renderTable(b); }
  });
  tbl.appendChild(tr);
  if (expanded) {
    var dr = el("tr", null);
    var dd = el("td");
    dd.setAttribute("colspan", "5");
    dd.appendChild(rowDetail(b, row));
    dr.appendChild(dd);
    tbl.appendChild(dr);
  }
}

// row detail (design §3): title + chips, prose, verdicts, timestamps,
// links, related events, raw JSON — each block omitted when absent.
function rowDetail(b, row){
  var d = el("div", "detail");
  d.setAttribute("aria-label", "detail of " + row.id);
  function sec(name, node){ if (node) { d.appendChild(el("h5", null, name)); d.appendChild(node); } }
  var head = el("div", "facts");
  head.appendChild(el("strong", null, row.title === undefined || row.title === null ? "" : String(row.title)));
  head.appendChild(chip(taskStatus(row)));
  head.appendChild(pchip(row.priority));
  if (isFixtureRow(b, row)) head.appendChild(el("span", "chip chip-fixture", "fixture"));
  d.appendChild(head);
  ["reasoning", "worker_summary", "foreman_note", "review_notes"].forEach(function(k){
    if (!row[k]) return;
    var p = el("p", null, String(row[k]));
    p.style.whiteSpace = "pre-wrap";
    sec(k.replace(/_/g, " "), p);
  });
  var facts = el("div", "facts");
  function fact(label, val){
    if (val === undefined || val === null || val === "") return;
    facts.appendChild(el("span", "badge", label + ": " + val));
  }
  fact("attempts", row.attempts);
  fact("exit_code", row.exit_code);
  fact("complexity", row.complexity);
  fact("lines +", row.lines_added);
  fact("lines −", row.lines_removed);
  var g = verdictChip(row.guard_result, ["PASS"], ["FAIL"]);
  if (g) facts.appendChild(g);
  var c = verdictChip(row.ci_result, ["GREEN"], ["RED"]);
  if (c) facts.appendChild(c);
  if (facts.childNodes.length) sec("facts", facts);
  var ts = el("div", "facts");
  fact2(ts, "created_at", row.created_at);
  fact2(ts, "dispatched_at", row.dispatched_at);
  fact2(ts, "completed_at", row.completed_at);
  fact2(ts, "updated_at", row.updated_at);
  fact2(ts, "blocked_since", row.blocked_since);
  fact2(ts, "blocked_reason", row.blocked_reason);
  if (ts.childNodes.length) sec("timestamps", ts);
  var links = el("div", "facts");
  ["depends_on", "blocks"].forEach(function(key){
    var list = row[key];
    if (!Array.isArray(list)) return;
    list.forEach(function(dep){
      var known = boardTasks(b).some(function(r){ return r.id === dep; });
      if (known) {
        var a = el("a", null, key + " " + dep);
        a.setAttribute("href", "#b=" + encodeURIComponent(b.slug) + "&t=" + encodeURIComponent(dep));
        a.addEventListener("click", function(ev){
          ev.preventDefault();
          state.expanded = {};
          state.expanded[dep] = true;
          location.hash = buildHash();
          applyHash();
        });
        links.appendChild(a);
      } else {
        links.appendChild(el("span", "badge", key + " " + dep + " (unknown)"));
      }
    });
  });
  if (links.childNodes.length) sec("dependencies", links);
  sec("related events", relatedEvents(b, row));
  var jp = el("pre");
  var raw = null;
  try { raw = JSON.stringify(row, null, 2); } catch(e2) { raw = null; }
  jp.textContent = raw !== null ? raw : "(unserializable row)";
  sec("task JSON", jp);
  return d;
}
function fact2(parent, label, val){
  if (val === undefined || val === null || val === "") return;
  parent.appendChild(el("span", "badge", label + ": " + val));
}
function verdictChip(v, okWords, badWords){
  if (v === undefined || v === null || v === "") return null;
  var up = String(v).toUpperCase();
  var cls = "chip";
  if (okWords.indexOf(up) >= 0) cls += " chip-complete";
  else if (badWords.indexOf(up) >= 0) cls += " chip-failed";
  var c = el("span", cls, up);
  c.removeAttribute("data-status");
  return c;
}
function relatedEvents(b, row){
  var all = eventsOf(b.slug).filter(function(e){ return String(e.task_id) === String(row.id); });
  var wrap = el("div");
  if (!all.length) {
    wrap.appendChild(el("div", "muted", "no events for this task"));
    return wrap;
  }
  var showN = 8;
  var shown = all.slice(Math.max(0, all.length - showN));
  shown.forEach(function(e){ wrap.appendChild(eventItem(e)); });
  if (all.length > showN) {
    var btn = el("button", "btn", "show all " + all.length);
    btn.addEventListener("click", function(){
      state.evTask = String(row.id);
      location.hash = buildHash();
      renderBoardPage();
      var tl = document.getElementById("events-section");
      if (tl) { tl.scrollIntoView({ block: "start" }); tl.focus(); }
    });
    wrap.appendChild(btn);
  }
  return wrap;
}

// ---------- S3 validation panel (R13: findings, not enforcement) ----------
function renderValidation(b, host){
  var v = b.validate || { pill: "PASS", errors: 0, warnings: 0, findings: [], census: "", sweep_census: {} };
  var panel = el("div", "panel");
  panel.setAttribute("id", "validation-section");
  panel.setAttribute("tabindex", "-1");
  var h = el("h3", null, "Validation ");
  h.appendChild(pill(v.pill));
  h.appendChild(el("span", "muted", "  " + v.errors + " error" + (v.errors === 1 ? "" : "s") + " · " + v.warnings + " warning" + (v.warnings === 1 ? "" : "s")));
  panel.appendChild(h);
  if (v.pill === "WARN") panel.appendChild(el("div", "muted", "WARN is validate's exit-0-with-warnings shape: nothing blocks, the findings below name what a CLI pass would flag."));
  if (v.pill === "FAIL") panel.appendChild(el("div", "muted", "The board stays fully browsable — validation is a panel, not a gate. Repairs are CLI-only."));
  if (Array.isArray(v.findings) && v.findings.length) {
    var list = el("ul", "findings");
    v.findings.forEach(function(f){
      var li = el("li");
      var lv = el("span", "lv " + (f.level === "error" ? "error" : "warn"), f.level);
      li.appendChild(lv);
      li.appendChild(el("span", "mono", String(f.message)));
      list.appendChild(li);
    });
    panel.appendChild(list);
  }
  if (v.census) {
    panel.appendChild(el("h4", null, "Key uniformity"));
    panel.appendChild(el("div", "census", String(v.census)));
  }
  var sw = v.sweep_census || {};
  if (sw.rows !== undefined) {
    panel.appendChild(el("h4", null, "Sweep census (read-only)"));
    panel.appendChild(el("div", null, sw.off_vocabulary + "/" + sw.rows + " rows off-vocabulary: " + sw.fixable_by_normalize + " fixable by --normalize, " + sw.need_explicit_decision + " need explicit decision"));
    if (Array.isArray(sw.unknown_rows) && sw.unknown_rows.length) {
      panel.appendChild(el("div", "mono", "unknown statuses: " + sw.unknown_rows.join(", ")));
    }
    panel.appendChild(el("div", "muted", sw.decisions_note || "no decisions applied — CLI-only"));
  }
  host.appendChild(panel);
}

// ---------- S4 events timeline (R14) ----------
function renderEvents(b, host){
  var sec = el("div", "panel");
  sec.setAttribute("id", "events-section");
  sec.setAttribute("tabindex", "-1");
  sec.appendChild(el("h3", null, "Events"));
  var all = eventsOf(b.slug);
  var toolbar = el("div", "toolbar");
  var typeSel = el("select");
  typeSel.setAttribute("aria-label", "filter by event type");
  var types = {};
  all.forEach(function(e){ types[e.event_type || "(none)"] = true; });
  ["all"].concat(Object.keys(types).sort()).forEach(function(t){
    var op = el("option", null, t);
    op.setAttribute("value", t);
    typeSel.appendChild(op);
  });
  typeSel.value = state.evType !== "all" && types[state.evType] ? state.evType : "all";
  state.evType = typeSel.value;
  typeSel.addEventListener("change", function(){ state.evType = typeSel.value; renderEventList(b); });
  toolbar.appendChild(typeSel);
  var tf = el("input");
  tf.type = "text";
  tf.placeholder = "filter by task id";
  tf.value = state.evTask;
  tf.setAttribute("aria-label", "filter events by task id");
  tf.addEventListener("input", function(){ state.evTask = tf.value.trim(); renderEventList(b); history.replaceState(null, "", buildHash()); });
  toolbar.appendChild(tf);
  var evCount = el("span", "count");
  evCount.setAttribute("id", "evcount");
  evCount.setAttribute("aria-live", "polite");
  toolbar.appendChild(evCount);
  sec.appendChild(toolbar);
  var listHost = el("div");
  listHost.setAttribute("id", "event-list");
  sec.appendChild(listHost);
  host.appendChild(sec);
  renderEventList(b);
}
function renderEventList(b){
  var host = document.getElementById("event-list");
  if (!host) return;
  clear(host);
  var all = eventsOf(b.slug);
  var visible = all.filter(function(e){
    if (state.evType !== "all" && (e.event_type || "(none)") !== state.evType) return false;
    if (state.evTask && String(e.task_id || "") !== state.evTask) return false;
    return true;
  });
  var ec = document.getElementById("evcount");
  if (ec) ec.textContent = all.length + " events · " + visible.length + " shown";
  if (!all.length) {
    host.appendChild(el("div", "empty", "no events"));
    return;
  }
  if (!visible.length) {
    var em = el("div", "empty", "no match for the current filters");
    var clearBtn = el("button", "btn", "clear filters");
    clearBtn.addEventListener("click", function(){ state.evType = "all"; state.evTask = ""; renderEvents(b, document.getElementById("view-board")); renderBoardPage(); });
    em.appendChild(document.createTextNode(" "));
    em.appendChild(clearBtn);
    host.appendChild(em);
    return;
  }
  var filtered = state.evType !== "all" || state.evTask;
  var shown = visible;
  if (!filtered) {
    var n = state.batchEvents[b.slug] || BATCH;
    if (visible.length > n) shown = visible.slice(0, n);
  }
  var ol = el("ol", "timeline");
  shown.forEach(function(e){
    var li = el("li");
    li.appendChild(eventItem(e));
    ol.appendChild(li);
  });
  host.appendChild(ol);
  if (!filtered && visible.length > shown.length) {
    var btn = el("button", "btn", "show " + Math.min(BATCH, visible.length - shown.length) + " more");
    btn.addEventListener("click", function(){
      state.batchEvents[b.slug] = (state.batchEvents[b.slug] || BATCH) + BATCH;
      renderEventList(b);
    });
    host.appendChild(btn);
  }
}
function eventItem(e){
  var wrap = el("div");
  var meta = el("div", "ev-meta");
  meta.appendChild(el("span", "ev-ts", e.timestamp || "?"));
  meta.appendChild(el("span", "badge", e.event_type || "(none)"));
  if (e.actor) meta.appendChild(el("span", "muted", e.actor));
  if (e.task_id !== null && e.task_id !== undefined && e.task_id !== "") {
    var a = el("a", null, String(e.task_id));
    a.setAttribute("href", "#b=" + encodeURIComponent(state.board || "") + "&t=" + encodeURIComponent(String(e.task_id)));
    a.addEventListener("click", function(ev){
      ev.preventDefault();
      state.expanded = {};
      state.expanded[String(e.task_id)] = true;
      location.hash = buildHash();
      applyHash();
    });
    meta.appendChild(a);
  } else {
    meta.appendChild(el("span", "muted", "—"));
  }
  if (e.tick_number !== null && e.tick_number !== undefined && e.tick_number !== "") meta.appendChild(el("span", "ev-ts", "tick " + e.tick_number));
  wrap.appendChild(meta);
  var de = el("div", "ev-detail");
  var v = e.detail;
  if (v === null || v === undefined) {
    de.appendChild(el("span", "muted", "(no detail)"));
  } else if (typeof v === "object") {
    var pre = el("pre");
    var txt = null;
    try { txt = JSON.stringify(v, null, 2); } catch(err) { txt = null; }
    pre.textContent = txt !== null ? txt : "(unrenderable detail)";
    de.appendChild(pre);
  } else {
    de.appendChild(el("span", null, String(v)));
  }
  wrap.appendChild(de);
  return wrap;
}

// ---------- S5 compare (>=2 boards) ----------
function renderCompare(){
  var host = document.getElementById("view-compare");
  clear(host);
  renderMeta(); renderFooter();
  var healthy = healthyBoards();
  if (healthy.length < 2) {
    host.appendChild(el("div", "empty", "compare needs at least two boards"));
    return;
  }
  var crumb = el("div", "crumb");
  var back = el("a", null, "← all boards");
  back.setAttribute("href", "#");
  back.addEventListener("click", function(e){ e.preventDefault(); location.hash = "#"; showView("overview"); });
  crumb.appendChild(back);
  host.appendChild(crumb);
  host.appendChild(el("h2", null, "Compare boards"));
  var chips = el("div", "facts");
  if (!Object.keys(state.selecting).length) healthy.forEach(function(b){ state.selecting[b.slug] = true; });
  healthy.forEach(function(b){
    var cb = el("input");
    cb.type = "checkbox";
    cb.checked = !!state.selecting[b.slug];
    cb.addEventListener("change", function(){ state.selecting[b.slug] = cb.checked; renderCompare(); });
    var lab = el("label", null, " " + b.name);
    lab.insertBefore(cb, lab.firstChild);
    chips.appendChild(lab);
  });
  host.appendChild(chips);
  var selected = healthy.filter(function(b){ return state.selecting[b.slug]; });
  if (!selected.length) {
    host.appendChild(el("div", "empty", "select at least one board"));
    return;
  }
  var t = el("table", "plain");
  var hr = el("tr");
  ["board", "ticks_total", "ticks_idle", "tasks", "open", "complete", "completion %", "delivery streak", "median cycle", "last activity"].forEach(function(c){ hr.appendChild(el("th", null, c)); });
  t.appendChild(hr);
  selected.forEach(function(b){
    var d = b.derived || {};
    var h = b.header || {};
    var r = el("tr");
    function cc(v){ r.appendChild(el("td", null, String(v))); }
    cc(b.name);
    cc(h.ticks_total !== undefined && h.ticks_total !== null ? h.ticks_total : "—");
    cc(h.ticks_idle !== undefined && h.ticks_idle !== null ? h.ticks_idle : "—");
    cc(d.total_nonfixture || 0);
    cc(d.open_count || 0);
    cc(d.complete_count || 0);
    cc(fmtPct(d.complete_count || 0, (d.complete_count || 0) + (d.open_count || 0)));
    var dl = (d.streaks || {}).delivery || {};
    cc(streakFmt(dl.current, dl.current_state === "live" ? " (live)" : " (ended)"));
    cc(d.cycle && d.cycle.median_days !== null && d.cycle.median_days !== undefined ? d.cycle.median_days + "d" : "—");
    cc(d.last_activity_day || "—");
    t.appendChild(r);
  });
  host.appendChild(t);
  host.appendChild(el("h3", null, "Overlaid burn-up — cumulative completions"));
  var colors = ["#58a6ff", "#3fb950", "#bc8cff", "#d29922", "#f85149", "#39c5cf"];
  var W = 900, H = 260, pad = { l: 36, r: 10, t: 12, b: 26 };
  var s = svg(W, H, "overlaid burn-up");
  var globalMax = 1;
  selected.forEach(function(b){ ((b.derived || {}).burnup || {}).cum && (b.derived.burnup.cum || []).forEach(function(v){ if (v > globalMax) globalMax = v; }); });
  var iw = W - pad.l - pad.r, ih = H - pad.t - pad.b;
  [0, .25, .5, .75, 1].forEach(function(g){
    sline(s, pad.l, W - pad.r, pad.t + ih * (1 - g));
    stag(s, pad.l - 4, pad.t + ih * (1 - g) + 3, Math.round(globalMax * g), "end");
  });
  var legend = el("div", "facts");
  selected.forEach(function(b, bi){
    var cum = (((b.derived || {}).burnup) || {}).cum || [];
    if (!cum.length) return;
    var color = colors[bi % colors.length];
    var xs = cum.map(function(_, i){ return cum.length === 1 ? 0 : i / (cum.length - 1); });
    var ys = cum.map(function(v){ return v / globalMax; });
    var p = svgel("path");
    p.setAttribute("d", linePath(xs, ys, pad.l, pad.t, iw, ih));
    p.setAttribute("fill", "none");
    p.setAttribute("stroke", color);
    p.setAttribute("stroke-width", "1.6");
    s.appendChild(p);
    var sw2 = el("span", "badge", b.name);
    sw2.style.color = color;
    legend.appendChild(sw2);
  });
  stag(s, pad.l, H - 6, "day 0");
  stag(s, W - pad.r, H - 6, "day N", "end");
  var cb = el("div", "chartbox");
  cb.appendChild(s);
  cb.appendChild(legend);
  host.appendChild(cb);
}

// ---------- refresh (R18 upgrade path; fetch only same-origin /api/) ----------
function upgradeBoard(slug){
  fetch("/api/board/" + encodeURIComponent(slug)).then(function(r){
    if (!r.ok) throw new Error("board fetch " + r.status);
    return r.json();
  }).then(function(detail){
    fresh[slug] = detail;
    return fetch("/api/events/" + encodeURIComponent(slug));
  }).then(function(r){
    if (!r.ok) throw new Error("events fetch " + r.status);
    return r.json();
  }).then(function(evs){
    freshEvents[slug] = Array.isArray(evs) ? evs : [];
    if (state.view === "board" && state.board === slug) renderBoardPage();
    if (state.view === "overview") renderOverview();
  }).catch(function(){
    showOffline(); // island floor keeps the page fully usable (R18)
  });
}
document.getElementById("refresh").addEventListener("click", boundary(function(){
  var btn = document.getElementById("refresh");
  btn.classList.add("spin");
  fetch("/api/boards").then(function(r){
    if (!r.ok) throw new Error("boards fetch " + r.status);
    return r.json();
  }).then(function(list){
    var b0 = boardBySlug(state.board);
    if (b0) upgradeBoard(state.board);
    if (state.view === "overview") renderOverview();
    btn.classList.remove("spin");
  }).catch(function(){
    btn.classList.remove("spin");
    showOffline();
  });
}));

// ---------- keyboard model (UX 5.7) ----------
document.addEventListener("keydown", boundary(function(e){
  var t = e.target || {};
  var typing = t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA";
  if (e.key === "/" && !typing && state.view === "board") {
    var q = document.getElementById("task-search");
    if (q) { e.preventDefault(); q.focus(); }
  }
  if ((e.key === "e" || e.key === "c") && !typing && state.view === "board") {
    var b = boardData(state.board);
    if (!b) return;
    if (e.key === "e") boardTasks(b).forEach(function(r){ state.expanded[r.id] = true; });
    else state.expanded = {};
    renderTable(b);
  }
}));

// ---------- init ----------
renderFooter();
try { applyHash(); }
catch(e){ fatal("the page could not render its snapshot: " + e); }
})();
</script>
</body>
</html>
`
