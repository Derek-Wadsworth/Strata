import "./style.css";
import {
  ListFiles,
  AddFile,
  RemoveFile,
  History,
  ShowVersion,
  RestoreVersion,
  DiffVersions,
  GetDashboard,
} from "../wailsjs/go/main/App.js";
import { EventsOn } from "../wailsjs/runtime/runtime.js";
import {
  listBranches,
  laneOf,
  getCurrentBranch,
  setCurrentBranch,
  branchOf,
  versionsOnBranch,
  versionVisibleOnBranch,
  createBranch,
  noteVersionOnCurrentBranch,
  getTitle,
  setTitle,
} from "./branches.js";

const $ = (id) => document.getElementById(id);

const shellEl = $("app");
const fileListEl = $("file-list");
const branchListEl = $("branch-list");
const projectLabelEl = $("project-label");
const watchStatusEl = $("watch-status");
const versionListEl = $("version-list");
const branchFilterBtn = $("branch-filter-btn");
const searchInput = $("search-input");
const branchSelectEl = $("branch-select");
const emptyStateEl = $("empty-state");
const workspaceEl = $("workspace");
const statsPanelEl = $("stats-panel");
const graphPanelEl = $("graph-panel");
const selectedNameEl = $("selected-name");
const selectedPathEl = $("selected-path");
const selectedBranchPillEl = $("selected-branch-pill");
const modeSegEl = $("mode-seg");
const prevChangeBtn = $("prev-change");
const nextChangeBtn = $("next-change");
const compareEl = $("compare");
const fromVerEl = $("from-ver");
const toLabelEl = $("to-label");
const diffStatEl = $("diff-stat");
const restoreBtn = $("restore-btn");
const summaryEl = $("summary");
const summaryTextEl = $("summary-text");
const diffCardEl = $("diff-card");
const diffHeadEl = $("diff-head");
const diffViewEl = $("diff-view");
const contentViewEl = $("content-view");
const threadEl = $("thread");
const agentInput = $("agent-input");
const toastEl = $("toast");
const branchModalEl = $("branch-modal");
const branchNameInput = $("branch-name-input");
const branchModalHint = $("branch-modal-hint");
const branchModalDesc = $("branch-modal-desc");

const LANE_COLORS = ["var(--blue)", "var(--purple)", "#e3b341", "#f778ba"];
const MAX_LANES = LANE_COLORS.length;
const CONTEXT = 3;
const DOT_Y = 18;

let files = [];
let selectedPath = null;
let versions = [];
let selectedVersion = null;
let fromVersion = null;
let activeView = "diff"; // diff | content
let diffMode = "side"; // inline | side
let appMode = "files"; // files | stats | graph
let showAllBranches = true;
let searchQuery = "";
let currentDiff = null;
let expandedFolds = new Set();
let changeAnchors = [];
let changeCursor = -1;

/* ———————————————————— Helpers ———————————————————— */

function baseName(path) {
  const parts = String(path).replace(/\\/g, "/").split("/");
  return parts[parts.length - 1] || path;
}

function dirName(path) {
  const parts = String(path).replace(/\\/g, "/").split("/");
  parts.pop();
  return parts;
}

function escapeHtml(text) {
  return String(text ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function fileBadge(path) {
  const ext = (baseName(path).split(".").pop() || "").toLowerCase();
  if (!ext || ext === baseName(path).toLowerCase()) return "FILE";
  return ext.slice(0, 3).toUpperCase();
}

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function relTime(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const sec = Math.round((Date.now() - d.getTime()) / 1000);
  if (sec < 60) return "just now";
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
  const today = new Date();
  const startOfToday = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime();
  if (d.getTime() >= startOfToday) return `${Math.floor(sec / 3600)}h ago`;
  if (d.getTime() >= startOfToday - 86400000) return "Yesterday";
  const opts = { month: "short", day: "numeric" };
  if (d.getFullYear() !== today.getFullYear()) opts.year = "numeric";
  return d.toLocaleDateString(undefined, opts);
}

function versionByNum(n) {
  return versions.find((v) => v.number === n) || null;
}

function latestVersion() {
  return versions.length ? versions[versions.length - 1].number : null;
}

function defaultTitle(v) {
  if (v.number === 1) return "Initial version";
  const a = v.linesAdded;
  const r = v.linesRemoved;
  if (a == null || r == null) return "Saved version";
  if (a === 0 && r === 0) return "No text changes";
  if (r === 0) return `Added ${a} line${a === 1 ? "" : "s"}`;
  if (a === 0) return `Removed ${r} line${r === 1 ? "" : "s"}`;
  return `Edited ${a + r} lines`;
}

function titleFor(v) {
  return getTitle(selectedPath, v.number) || defaultTitle(v);
}

function pillClass(path, branch) {
  const lane = Math.min(laneOf(path, branch), MAX_LANES - 1);
  return lane === 0 ? "pill" : `pill branch-${lane}`;
}

function changesHtml(a, r) {
  if (a == null || r == null) return "";
  return `+${a} −${r}`;
}

function highlightLine(text, path) {
  const escaped = escapeHtml(String(text ?? "").replace(/\r$/, ""));
  const ext = (baseName(path).split(".").pop() || "").toLowerCase();
  if (!["go", "js", "ts", "jsx", "tsx", "py", "css", "html", "json", "sh", "rs", "java", "c", "cpp", "cs"].includes(ext)) {
    return escaped;
  }
  return escaped
    .replace(/(&quot;)(?:(?!&quot;).)*?&quot;/g, '<span class="tok-str">$&</span>')
    .replace(
      /\b(func|package|import|return|const|let|var|if|else|for|while|class|def|from|async|await|type|struct|interface|switch|case|nil|null|true|false|err)\b(?![^<]*<\/span>)/g,
      '<span class="tok-kw">$1</span>'
    )
    .replace(/(\/\/.*$)/, '<span class="tok-comment">$1</span>');
}

function showToast(message) {
  toastEl.textContent = message;
  toastEl.classList.remove("hidden");
  clearTimeout(showToast._t);
  showToast._t = setTimeout(() => toastEl.classList.add("hidden"), 2600);
}

/* ———————————————————— Theme ———————————————————— */

function applyTheme(theme) {
  document.documentElement.setAttribute("data-theme", theme === "light" ? "light" : "dark");
  localStorage.setItem("strata.theme", theme);
}

applyTheme(localStorage.getItem("strata.theme") === "light" ? "light" : "dark");

/* ———————————————————— Modes ———————————————————— */

function setAppMode(mode) {
  appMode = mode;
  $("nav-files").classList.toggle("active", mode === "files");
  $("nav-stats").classList.toggle("active", mode === "stats");
  $("nav-graph").classList.toggle("active", mode === "graph");
  shellEl.classList.toggle("wide-main", mode !== "files");
  statsPanelEl.classList.toggle("hidden", mode !== "stats");
  graphPanelEl.classList.toggle("hidden", mode !== "graph");
  updateMainVisibility();
  if (mode === "stats") loadDashboard();
  if (mode === "graph") renderGraph();
}

function updateMainVisibility() {
  const filesMode = appMode === "files";
  emptyStateEl.classList.toggle("hidden", !filesMode || !!selectedPath);
  workspaceEl.classList.toggle("hidden", !filesMode || !selectedPath);
}

/* ———————————————————— Sidebar ———————————————————— */

function renderProjectLabel() {
  if (!files.length) {
    projectLabelEl.textContent = "Tracked files";
    return;
  }
  let common = dirName(files[0].path);
  for (const f of files.slice(1)) {
    const parts = dirName(f.path);
    let i = 0;
    while (i < common.length && i < parts.length && common[i].toLowerCase() === parts[i].toLowerCase()) i++;
    common = common.slice(0, i);
  }
  const name = common[common.length - 1];
  projectLabelEl.textContent = name && !/^[a-z]:$/i.test(name) ? `Project: ${name}` : "Tracked files";
}

function renderFiles() {
  const q = searchQuery.toLowerCase();
  const shown = q ? files.filter((f) => baseName(f.path).toLowerCase().includes(q)) : files;
  fileListEl.innerHTML = "";
  for (const file of shown) {
    const li = document.createElement("li");
    li.className = file.path === selectedPath ? "active" : "";
    li.title = file.path;
    li.innerHTML = `
      <span class="file-icon">${fileBadge(file.path)}</span>
      <span class="file-text">
        <span class="file-name">${escapeHtml(baseName(file.path))}</span>
        <span class="file-meta">${file.versionCount} version${file.versionCount === 1 ? "" : "s"}${
          file.missing ? ' · <span class="missing">missing</span>' : ""
        }</span>
      </span>`;
    li.addEventListener("click", () => {
      if (appMode !== "files") setAppMode("files");
      selectFile(file.path);
    });
    li.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      removeFileFlow(file.path);
    });
    fileListEl.appendChild(li);
  }
  if (!shown.length) {
    fileListEl.innerHTML = `<li class="history-empty" style="cursor:default;display:block">${
      files.length ? "No matching files" : "No files tracked yet"
    }</li>`;
  }
  const missing = files.filter((f) => f.missing).length;
  watchStatusEl.textContent = `Watching ${files.length} file${files.length === 1 ? "" : "s"} · ${
    missing ? `${missing} missing` : "up to date"
  }`;
  renderProjectLabel();
}

function renderBranches() {
  branchListEl.innerHTML = "";
  branchSelectEl.innerHTML = "";
  if (!selectedPath) {
    branchListEl.innerHTML = `<li class="current"><span class="pill">main</span><span class="branch-note">current</span></li>`;
    branchSelectEl.innerHTML = `<option>main</option>`;
    branchSelectEl.disabled = true;
    $("create-branch-btn").disabled = true;
    return;
  }
  branchSelectEl.disabled = false;
  $("create-branch-btn").disabled = false;
  const current = getCurrentBranch(selectedPath);
  for (const b of listBranches(selectedPath)) {
    const own = versionsOnBranch(selectedPath, b.name, versions).length;
    let note;
    if (b.name === current) note = "current";
    else if (b.name === "main") note = `${own} version${own === 1 ? "" : "s"}`;
    else note = own ? `${own} ahead` : `from v${b.fromVersion}`;

    const li = document.createElement("li");
    li.className = b.name === current ? "current" : "";
    li.innerHTML = `<span class="${pillClass(selectedPath, b.name)}">${escapeHtml(b.name)}</span><span class="branch-note">${note}</span>`;
    li.addEventListener("click", () => switchBranch(b.name));
    branchListEl.appendChild(li);

    const opt = document.createElement("option");
    opt.value = b.name;
    opt.textContent = b.name;
    branchSelectEl.appendChild(opt);
  }
  branchSelectEl.value = current;
  selectedBranchPillEl.textContent = current;
  selectedBranchPillEl.className = pillClass(selectedPath, current);
}

function switchBranch(name) {
  if (!selectedPath) return;
  setCurrentBranch(selectedPath, name);
  renderBranches();
  renderVersions();
  showToast(`Switched to ${name}`);
}

/* ———————————————————— Files & versions ———————————————————— */

async function refreshFiles(preferPath = selectedPath) {
  files = (await ListFiles()) || [];
  renderFiles();
  if (preferPath && files.some((f) => f.path === preferPath)) {
    await selectFile(preferPath, selectedVersion);
  } else if (files.length) {
    await selectFile(files[0].path);
  } else {
    selectedPath = null;
    versions = [];
    selectedVersion = null;
    renderBranches();
    renderVersions();
    updateMainVisibility();
  }
}

async function selectFile(path, keepVersion = null) {
  const changed = path !== selectedPath;
  selectedPath = path;
  if (changed) expandedFolds = new Set();
  selectedNameEl.textContent = baseName(path);
  selectedPathEl.textContent = path;
  versions = (await History(path)) || [];
  renderFiles();
  renderBranches();
  updateMainVisibility();
  const target = keepVersion && versionByNum(keepVersion) ? keepVersion : latestVersion();
  if (target != null) {
    await selectVersion(target);
  } else {
    selectedVersion = null;
    renderVersions();
    currentDiff = null;
    diffHeadEl.innerHTML = "";
    diffViewEl.innerHTML = `<div class="diff-empty">No versions yet.</div>`;
    summaryEl.classList.add("hidden");
  }
}

function visibleVersions() {
  let list = versions;
  if (selectedPath && !showAllBranches) {
    const cur = getCurrentBranch(selectedPath);
    list = list.filter((v) => versionVisibleOnBranch(selectedPath, v.number, cur));
  }
  const q = searchQuery.toLowerCase().trim();
  if (q) {
    list = list.filter((v) => `v${v.number}`.includes(q) || titleFor(v).toLowerCase().includes(q));
  }
  return list;
}

function laneSpans() {
  const spans = [];
  const branches = listBranches(selectedPath).slice(0, MAX_LANES);
  branches.forEach((b, lane) => {
    const own = versionsOnBranch(selectedPath, b.name, versions).map((v) => v.number);
    if (!own.length) return;
    const hi = Math.max(...own);
    const lo = lane === 0 ? Math.min(...own) : b.fromVersion || Math.min(...own);
    spans.push({ lane, name: b.name, hi, lo, fork: lane === 0 ? null : b.fromVersion });
  });
  return { spans, laneCount: Math.max(1, Math.min(branches.length, MAX_LANES)) };
}

function laneX(lane) {
  return 12 + lane * 14;
}

function graphCellHtml(v, spans) {
  let html = "";
  for (const s of spans) {
    if (v.number > s.hi || v.number < s.lo) continue;
    const x = laneX(s.lane);
    const color = s.lane === 0 ? `color-mix(in srgb, var(--blue) 55%, transparent)` : LANE_COLORS[s.lane];
    if (s.fork != null && v.number === s.fork && s.fork > 0) {
      const x0 = laneX(0);
      html += `<svg class="lane-fork" width="${x + 4}" height="${DOT_Y + 2}">
        <path d="M${x} 0 C ${x} ${DOT_Y * 0.65}, ${x0} ${DOT_Y * 0.35}, ${x0} ${DOT_Y}" fill="none" stroke="${color}" stroke-width="2"/>
      </svg>`;
      continue;
    }
    const top = v.number === s.hi ? DOT_Y : 0;
    const bottomEnds = s.lane === 0 ? v.number === s.lo : false;
    const style = `left:${x - 1}px;top:${top}px;${bottomEnds ? `height:${Math.max(0, DOT_Y - top)}px` : "bottom:0"};background:${color}`;
    if (s.hi === s.lo && s.lane === 0) continue;
    html += `<span class="lane-line" style="${style}"></span>`;
  }
  const lane = Math.min(laneOf(selectedPath, branchOf(selectedPath, v.number)), MAX_LANES - 1);
  html += `<span class="lane-dot" style="left:${laneX(lane)}px;background:${LANE_COLORS[lane]}"></span>`;
  return html;
}

function renderVersions() {
  versionListEl.innerHTML = "";
  if (selectedPath) {
    const cur = getCurrentBranch(selectedPath);
    branchFilterBtn.textContent = showAllBranches ? "All branches" : `${cur} only`;
  }
  if (!selectedPath) {
    versionListEl.innerHTML = `<li class="history-empty">Select a file to see its history.</li>`;
    return;
  }
  const shown = [...visibleVersions()].reverse();
  if (!shown.length) {
    versionListEl.innerHTML = `<li class="history-empty">No versions${searchQuery ? " match your search" : " on this branch yet"}.</li>`;
    return;
  }
  const { spans, laneCount } = laneSpans();
  const graphW = laneX(laneCount - 1) + 14;

  for (const v of shown) {
    const branch = branchOf(selectedPath, v.number);
    const li = document.createElement("li");
    li.className = `vrow${v.number === selectedVersion ? " active" : ""}`;
    li.style.setProperty("--graph-w", `${graphW}px`);
    const meta = [changesHtml(v.linesAdded, v.linesRemoved), formatSize(v.size)].filter(Boolean).join(" · ");
    li.innerHTML = `
      <div class="graph-cell">${graphCellHtml(v, spans)}</div>
      <div class="vrow-body">
        <div class="vrow-top">
          <span class="vrow-num">v${v.number}</span>
          <span class="${pillClass(selectedPath, branch)}">${escapeHtml(branch)}</span>
          <span class="vrow-time" title="${escapeHtml(new Date(v.createdAt).toLocaleString())}">${escapeHtml(relTime(v.createdAt))}</span>
        </div>
        <div class="vrow-title" title="Double-click to rename">${escapeHtml(titleFor(v))}</div>
        <div class="vrow-meta">${escapeHtml(meta)}</div>
      </div>`;
    li.addEventListener("click", () => selectVersion(v.number));
    li.querySelector(".vrow-title").addEventListener("dblclick", (e) => {
      e.stopPropagation();
      editTitle(li.querySelector(".vrow-title"), v);
    });
    versionListEl.appendChild(li);
  }
}

function editTitle(el, v) {
  const input = document.createElement("input");
  input.className = "vrow-title-input";
  input.value = titleFor(v);
  el.replaceWith(input);
  input.focus();
  input.select();
  let done = false;
  const finish = (commit) => {
    if (done) return;
    done = true;
    if (commit) setTitle(selectedPath, v.number, input.value === defaultTitle(v) ? "" : input.value);
    renderVersions();
  };
  input.addEventListener("click", (e) => e.stopPropagation());
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") finish(true);
    if (e.key === "Escape") finish(false);
  });
  input.addEventListener("blur", () => finish(true));
}

async function selectVersion(number) {
  selectedVersion = number;
  const prev = versions.filter((v) => v.number < number).pop();
  fromVersion = prev ? prev.number : null;
  expandedFolds = new Set();
  renderVersions();
  fillCompare();
  await refreshView();
}

function fillCompare() {
  fromVerEl.innerHTML = "";
  for (const v of versions.filter((x) => x.number < selectedVersion)) {
    const opt = document.createElement("option");
    opt.value = String(v.number);
    opt.textContent = `v${v.number}`;
    fromVerEl.appendChild(opt);
  }
  if (fromVersion != null) fromVerEl.value = String(fromVersion);
  toLabelEl.textContent = `v${selectedVersion}`;
}

async function refreshView() {
  updateRestoreButton();
  if (activeView === "content") {
    await loadContent();
  } else {
    await loadDiff();
  }
}

function updateRestoreButton() {
  const target = activeView === "diff" ? fromVersion : selectedVersion;
  restoreBtn.disabled = target == null;
  restoreBtn.textContent = target == null ? "Restore" : `Restore v${target}`;
  restoreBtn.dataset.version = target ?? "";
}

/* ———————————————————— Views ———————————————————— */

function setActiveView(view) {
  activeView = view;
  $("tab-diff").classList.toggle("active", view === "diff");
  $("tab-content").classList.toggle("active", view === "content");
  const diff = view === "diff";
  modeSegEl.classList.toggle("hidden", !diff);
  prevChangeBtn.classList.toggle("hidden", !diff);
  nextChangeBtn.classList.toggle("hidden", !diff);
  compareEl.classList.toggle("hidden", !diff);
  summaryEl.classList.toggle("hidden", !diff);
  diffCardEl.classList.toggle("hidden", !diff);
  contentViewEl.classList.toggle("hidden", diff);
}

async function loadContent() {
  try {
    const text = await ShowVersion(selectedPath, selectedVersion);
    contentViewEl.textContent = text === "" ? "(empty file)" : text;
  } catch (err) {
    contentViewEl.textContent = String(err);
  }
}

async function loadDiff() {
  if (!selectedPath || selectedVersion == null) return;
  if (fromVersion == null) {
    currentDiff = null;
    changeAnchors = [];
    updateChangeNav();
    diffStatEl.textContent = "first version";
    summaryEl.classList.remove("hidden");
    summaryTextEl.innerHTML = `<strong>Summary:</strong> this is the first saved version of <code>${escapeHtml(
      baseName(selectedPath)
    )}</code>. Save the file again to see changes here.`;
    diffHeadEl.className = "diff-head single";
    diffHeadEl.innerHTML = `<span>v${selectedVersion} · ${escapeHtml(relTime(versionByNum(selectedVersion).createdAt))}</span>`;
    diffViewEl.innerHTML = `<div class="diff-empty">Nothing to compare yet.</div>`;
    return;
  }
  try {
    currentDiff = await DiffVersions(selectedPath, fromVersion, selectedVersion);
    renderDiff();
  } catch (err) {
    currentDiff = null;
    diffViewEl.innerHTML = `<div class="diff-empty">${escapeHtml(String(err))}</div>`;
    changeAnchors = [];
    updateChangeNav();
  }
}

function blocksOf(lines) {
  const blocks = [];
  let i = 0;
  while (i < lines.length) {
    const equal = lines[i].op === "equal";
    const start = i;
    while (i < lines.length && (lines[i].op === "equal") === equal) i++;
    blocks.push({ type: equal ? "equal" : "change", start, lines: lines.slice(start, i) });
  }
  return blocks;
}

function summarize(diff) {
  const name = escapeHtml(baseName(selectedPath));
  if (diff.binary) {
    return `<strong>Summary:</strong> <code>${name}</code> is a binary file. Strata keeps every version and can restore it; image and video previews are coming later.`;
  }
  const a = diff.linesAdded;
  const r = diff.linesRemoved;
  if (!a && !r) return `<strong>Summary:</strong> no text changes between v${diff.fromVer} and v${diff.toVer}.`;
  const changes = blocksOf(diff.lines || []).filter((b) => b.type === "change");
  const n = changes.length;
  let shape;
  if (r > a * 2) shape = `mostly removals: ${r} lines removed, ${a} added`;
  else if (a > r * 2) shape = `mostly additions: ${a} lines added, ${r} removed`;
  else shape = `${a} lines added and ${r} removed`;
  let text = `<strong>Summary:</strong> ${shape} across ${n} change${n === 1 ? "" : "s"}.`;
  const biggest = changes.slice().sort((x, y) => y.lines.length - x.lines.length)[0];
  if (biggest && n > 1) {
    const first = biggest.lines[0];
    const at = first.newNo || first.oldNo;
    text += ` The largest change (${biggest.lines.length} lines) is around line ${at}.`;
  }
  const sample = biggest?.lines.find((l) => l.text.trim().length > 3);
  if (sample) {
    const snippet = sample.text.trim().slice(0, 48) + (sample.text.trim().length > 48 ? "…" : "");
    text += ` It ${sample.op === "add" ? "adds" : "removes"} <code>${escapeHtml(snippet)}</code>.`;
  }
  return text;
}

function renderDiff() {
  if (!currentDiff) return;
  const d = currentDiff;
  const fromV = versionByNum(d.fromVer);
  const toV = versionByNum(d.toVer);
  const fromLabel = `v${d.fromVer} · ${fromV ? relTime(fromV.createdAt) : ""}`;
  const toLabel = `v${d.toVer} · ${toV ? relTime(toV.createdAt) : ""}`;

  summaryEl.classList.remove("hidden");
  summaryTextEl.innerHTML = summarize(d);

  if (d.binary) {
    diffStatEl.textContent = "binary";
    diffHeadEl.className = "diff-head";
    diffHeadEl.innerHTML = `<span>${escapeHtml(fromLabel)}</span><span>${escapeHtml(toLabel)}</span>`;
    diffViewEl.innerHTML = `<div class="diff-empty"><strong>Binary files differ</strong>Visual diffs for images and video are planned. Use Restore to bring back an earlier version.</div>`;
    changeAnchors = [];
    updateChangeNav();
    return;
  }

  diffStatEl.innerHTML = `<span class="add">+${d.linesAdded}</span> <span class="del">−${d.linesRemoved}</span>`;
  if (diffMode === "side") {
    diffHeadEl.className = "diff-head";
    diffHeadEl.innerHTML = `<span>${escapeHtml(fromLabel)}</span><span>${escapeHtml(toLabel)}</span>`;
  } else {
    diffHeadEl.className = "diff-head single";
    diffHeadEl.innerHTML = `<span>${escapeHtml(fromLabel)} → ${escapeHtml(toLabel)}</span>`;
  }

  changeAnchors = [];
  changeCursor = -1;
  const blocks = blocksOf(d.lines || []);
  const rows = [];

  blocks.forEach((block, bi) => {
    if (block.type === "equal") {
      const isFirst = bi === 0;
      const isLast = bi === blocks.length - 1;
      const head = isFirst ? CONTEXT : CONTEXT;
      const tail = isLast ? 0 : CONTEXT;
      const hidden = block.lines.length - head - tail;
      if (hidden >= 2 && !expandedFolds.has(block.start)) {
        block.lines.slice(0, head).forEach((l) => rows.push(equalRow(l)));
        rows.push(
          `<div class="d-fold" data-fold="${block.start}">··· ${hidden} unchanged line${hidden === 1 ? "" : "s"}</div>`
        );
        if (tail) block.lines.slice(-tail).forEach((l) => rows.push(equalRow(l)));
      } else {
        block.lines.forEach((l) => rows.push(equalRow(l)));
      }
      return;
    }
    changeAnchors.push(block.start);
    rows.push(...changeRows(block));
  });

  const cls = diffMode === "side" ? "d-side" : "d-inline";
  diffViewEl.innerHTML = `<div class="d-table ${cls}">${rows.join("") || '<div class="diff-empty">No changes</div>'}</div>`;
  diffViewEl.querySelectorAll(".d-fold").forEach((el) => {
    el.addEventListener("click", () => {
      expandedFolds.add(Number(el.dataset.fold));
      renderDiff();
    });
  });
  updateChangeNav();
}

function equalRow(l) {
  const code = highlightLine(l.text, selectedPath);
  if (diffMode === "side") {
    return `<div class="d-row"><div class="d-half"><span class="d-ln">${l.oldNo}</span><span class="d-code">${code}</span></div><div class="d-half"><span class="d-ln">${l.newNo}</span><span class="d-code">${code}</span></div></div>`;
  }
  return `<div class="d-row"><span class="d-ln">${l.oldNo}</span><span class="d-ln">${l.newNo}</span><span class="d-sign"></span><span class="d-code">${code}</span></div>`;
}

function changeRows(block) {
  const anchor = `data-anchor="${block.start}"`;
  if (diffMode !== "side") {
    return block.lines.map((l, i) => {
      const sign = l.op === "add" ? "+" : "−";
      return `<div class="d-row ${l.op}" ${i === 0 ? anchor : ""}><span class="d-ln">${l.oldNo || ""}</span><span class="d-ln">${
        l.newNo || ""
      }</span><span class="d-sign">${sign}</span><span class="d-code">${highlightLine(l.text, selectedPath)}</span></div>`;
    });
  }
  const removed = block.lines.filter((l) => l.op === "remove");
  const added = block.lines.filter((l) => l.op === "add");
  const n = Math.max(removed.length, added.length);
  const out = [];
  for (let i = 0; i < n; i++) {
    const r = removed[i];
    const a = added[i];
    const left = r
      ? `<div class="d-half remove"><span class="d-ln">${r.oldNo}</span><span class="d-code">${highlightLine(r.text, selectedPath)}</span></div>`
      : `<div class="d-half d-blank"><span class="d-ln"></span><span class="d-code"></span></div>`;
    const right = a
      ? `<div class="d-half add"><span class="d-ln">${a.newNo}</span><span class="d-code">${highlightLine(a.text, selectedPath)}</span></div>`
      : `<div class="d-half d-blank"><span class="d-ln"></span><span class="d-code"></span></div>`;
    out.push(`<div class="d-row" ${i === 0 ? anchor : ""}>${left}${right}</div>`);
  }
  return out;
}

function updateChangeNav() {
  const has = changeAnchors.length > 0;
  prevChangeBtn.disabled = !has;
  nextChangeBtn.disabled = !has;
}

function jumpChange(delta) {
  if (!changeAnchors.length) return;
  changeCursor =
    changeCursor < 0
      ? delta > 0
        ? 0
        : changeAnchors.length - 1
      : (changeCursor + delta + changeAnchors.length) % changeAnchors.length;
  const el = diffViewEl.querySelector(`[data-anchor="${changeAnchors[changeCursor]}"]`);
  if (!el) return;
  diffViewEl.querySelectorAll(".current-change").forEach((n) => n.classList.remove("current-change"));
  el.classList.add("current-change");
  el.scrollIntoView({ block: "center", behavior: "smooth" });
}

/* ———————————————————— Actions ———————————————————— */

async function addFileFlow() {
  try {
    const file = await AddFile();
    showToast(`Tracking ${baseName(file.path)}`);
    if (appMode !== "files") setAppMode("files");
    await refreshFiles(file.path);
  } catch (err) {
    if (!String(err).toLowerCase().includes("cancel")) showToast(String(err));
  }
}

async function removeFileFlow(path) {
  if (!window.confirm(`Stop tracking ${baseName(path)}?\n\n${path}\n\nIts history is kept.`)) return;
  try {
    await RemoveFile(path);
    showToast(`Stopped tracking ${baseName(path)}`);
    await refreshFiles(path === selectedPath ? null : selectedPath);
  } catch (err) {
    showToast(String(err));
  }
}

async function restoreFlow(number) {
  if (!selectedPath || number == null) return false;
  const ok = window.confirm(
    `Restore ${baseName(selectedPath)} to v${number}?\n\nIf the current file differs, it's saved as a new version first.`
  );
  if (!ok) return false;
  try {
    const res = await RestoreVersion(selectedPath, number);
    let msg = `Restored v${res.restoredVersion}`;
    if (res.createdSafetyNet) msg += ` · current saved as v${res.safetyVersionNum}`;
    showToast(msg);
    await selectFile(selectedPath);
    return true;
  } catch (err) {
    showToast(String(err));
    return false;
  }
}

function openBranchModal() {
  if (!selectedPath) return;
  const from = selectedVersion ?? latestVersion();
  branchModalDesc.textContent = from
    ? `Branch ${baseName(selectedPath)} from v${from}. New saves will be recorded on this branch.`
    : "Create a branch. New saves will be recorded on it.";
  branchModalHint.textContent = "";
  branchNameInput.value = "";
  branchModalEl.classList.remove("hidden");
  branchNameInput.focus();
}

function closeBranchModal() {
  branchModalEl.classList.add("hidden");
}

function confirmBranch() {
  try {
    const from = selectedVersion ?? latestVersion() ?? 0;
    const name = createBranch(selectedPath, branchNameInput.value, from);
    closeBranchModal();
    renderBranches();
    renderVersions();
    showToast(`Created ${name} from v${from}`);
  } catch (err) {
    branchModalHint.textContent = err.message || String(err);
  }
}

/* ———————————————————— Assistant ———————————————————— */

function scrollThread() {
  threadEl.scrollTop = threadEl.scrollHeight;
}

function userSay(text) {
  const el = document.createElement("div");
  el.className = "msg user";
  el.textContent = text;
  threadEl.appendChild(el);
  scrollThread();
}

function botSay(html, actions = []) {
  const el = document.createElement("div");
  el.className = "msg bot";
  el.innerHTML = html;
  if (actions.length) {
    const row = document.createElement("div");
    row.className = "msg-actions";
    for (const act of actions) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = `btn ${act.primary ? "btn-primary" : "btn-ghost"}`;
      b.textContent = act.label;
      b.addEventListener("click", act.onClick);
      row.appendChild(b);
    }
    el.appendChild(row);
  }
  threadEl.appendChild(el);
  scrollThread();
}

function showDiffBetween(from, to) {
  if (appMode !== "files") setAppMode("files");
  setActiveView("diff");
  selectVersion(to).then(() => {
    if (from != null && from !== fromVersion) {
      fromVersion = from;
      fillCompare();
      refreshView();
    }
  });
}

async function handlePrompt(raw) {
  const text = raw.trim();
  if (!text) return;
  userSay(text);
  const q = text.toLowerCase();
  const name = selectedPath ? escapeHtml(baseName(selectedPath)) : "";
  const latest = latestVersion();
  const prev = versions.length > 1 ? versions[versions.length - 2].number : null;
  const explicit = q.match(/\bv(\d+)\b/);

  if (/\b(restore|revert|undo|roll ?back|go back)\b/.test(q)) {
    if (!selectedPath || !prev) {
      botSay(`<p>Select a file with at least two versions and I can restore an earlier one.</p>`);
      return;
    }
    const target = explicit && versionByNum(Number(explicit[1])) ? Number(explicit[1]) : prev;
    const tv = versionByNum(target);
    botSay(
      `<p>I can restore <strong>${name}</strong> to v${target} (${escapeHtml(relTime(tv.createdAt))}). If the current file differs, it's saved as a new version first.</p>`,
      [
        { label: "Show diff", onClick: () => showDiffBetween(target, latest) },
        { label: `Restore v${target}…`, primary: true, onClick: () => restoreFlow(target) },
      ]
    );
    return;
  }

  if (/\b(change|changed|changes|diff|differen\w*|compare|happened|since)\b/.test(q)) {
    if (!selectedPath || !prev) {
      botSay(`<p>Select a file with at least two versions and I'll summarize what changed.</p>`);
      return;
    }
    try {
      const d = await DiffVersions(selectedPath, prev, latest);
      botSay(`<p>${summarize(d).replace("<strong>Summary:</strong> ", `Between v${prev} and v${latest}: `)}</p>`, [
        { label: "Show diff", onClick: () => showDiffBetween(prev, latest) },
        { label: `Restore v${prev}…`, onClick: () => restoreFlow(prev) },
      ]);
    } catch (err) {
      botSay(`<p>${escapeHtml(String(err))}</p>`);
    }
    return;
  }

  if (/\b(find|track|search|look for|locate|add)\b/.test(q)) {
    const words = q.replace(/\b(find|track|search|look|for|locate|add|the|my|files?|in|to|a|an|me|please|docs?)\b/g, " ").split(/\s+/).filter((w) => w.length > 2);
    const matches = words.length ? files.filter((f) => words.some((w) => f.path.toLowerCase().includes(w))) : [];
    if (matches.length) {
      botSay(
        `<p>These tracked files match:</p><ul>${matches
          .slice(0, 6)
          .map((f) => `<li>${escapeHtml(baseName(f.path))}</li>`)
          .join("")}</ul>`,
        [
          { label: "Open first", primary: true, onClick: () => selectFile(matches[0].path) },
          { label: "Add another…", onClick: addFileFlow },
        ]
      );
      return;
    }
    botSay(
      `<p>Searching your disk for new files to track arrives with the AI agent. For now you can pick them yourself.</p>`,
      [{ label: "Add file…", primary: true, onClick: addFileFlow }]
    );
    return;
  }

  botSay(
    `<p>I'm not connected to a model yet. Today I can summarize recent changes, restore versions, and help you add files. Try <em>“What changed in this file?”</em></p>`
  );
}

function greet() {
  botSay(
    `<p>Hi! I can summarize what changed, restore earlier versions, and help you track files. Full AI search is coming soon.</p>`,
    [
      { label: "What changed?", primary: true, onClick: () => handlePrompt("What changed in this file?") },
      { label: "Restore…", onClick: () => handlePrompt("Restore the previous version") },
    ]
  );
}

/* ———————————————————— Stats ———————————————————— */

function formatMinutes(mins) {
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return m ? `${h}h ${m}m` : `${h}h`;
}

async function loadDashboard() {
  try {
    renderDashboard((await GetDashboard()) || {});
  } catch (err) {
    showToast(String(err));
  }
}

function renderDashboard(dash) {
  $("stat-cards").innerHTML = `
    <div class="stat-card"><span>Files</span><strong>${dash.totalFiles || 0}</strong></div>
    <div class="stat-card"><span>Versions</span><strong>${dash.totalVersions || 0}</strong></div>
    <div class="stat-card"><span>Lines</span><strong>+${dash.linesAdded || 0} / −${dash.linesRemoved || 0}</strong></div>
    <div class="stat-card"><span>Active time</span><strong>${formatMinutes(dash.estimatedActiveMinutes || 0)}</strong></div>
    <div class="stat-card"><span>Sessions</span><strong>${dash.sessionCount || 0}</strong></div>`;
  renderBarChart($("edits-chart"), dash.editsPerDay || [], (d) => d.day.slice(5), (d) => d.count);
  renderBarChart($("active-chart"), dash.mostActive || [], (d) => d.name, (d) => d.versions);
  renderHeatmap($("heatmap"), dash.heatmap || []);
  renderSizeChart($("size-chart"), dash.files || []);
  renderSessions($("session-list"), dash.sessions || []);
}

function renderBarChart(el, rows, labelFn, valueFn) {
  if (!rows.length) {
    el.innerHTML = `<div class="chart-empty">No data yet</div>`;
    return;
  }
  const max = Math.max(...rows.map(valueFn), 1);
  const width = Math.max(320, rows.length * 40);
  const bars = rows
    .map((row, i) => {
      const v = valueFn(row);
      const h = Math.max(2, Math.round((v / max) * 110));
      const x = 24 + i * 40;
      const y = 130 - h;
      return `<rect class="bar" x="${x}" y="${y}" width="24" height="${h}" rx="3"></rect>
        <text x="${x + 12}" y="148" text-anchor="middle">${escapeHtml(labelFn(row))}</text>
        <text x="${x + 12}" y="${y - 6}" text-anchor="middle">${v}</text>`;
    })
    .join("");
  el.innerHTML = `<svg viewBox="0 0 ${width} 160" width="100%" height="160">${bars}</svg>`;
}

function renderHeatmap(el, days) {
  if (!days.length) {
    el.innerHTML = `<div class="chart-empty">No activity yet</div>`;
    return;
  }
  const map = Object.fromEntries(days.map((d) => [d.day, d.count]));
  const max = Math.max(...days.map((d) => d.count), 1);
  const end = new Date(days[days.length - 1].day + "T00:00:00Z");
  const start = new Date(end);
  start.setUTCDate(end.getUTCDate() - 119);
  const cells = [];
  for (let d = new Date(start); d <= end; d.setUTCDate(d.getUTCDate() + 1)) {
    const key = d.toISOString().slice(0, 10);
    const count = map[key] || 0;
    const level = count === 0 ? 0 : Math.min(4, Math.ceil((count / max) * 4));
    cells.push(`<div class="heat-cell level-${level}" title="${key}: ${count} edit${count === 1 ? "" : "s"}"></div>`);
  }
  el.innerHTML = `<div class="heat-grid">${cells.join("")}</div>`;
}

function renderSizeChart(el, fileStats) {
  const series = (fileStats || []).filter((f) => (f.sizeOverTime || []).length > 0);
  if (!series.length) {
    el.innerHTML = `<div class="chart-empty">No size history yet</div>`;
    return;
  }
  const top = [...series].sort((a, b) => b.totalVersions - a.totalVersions)[0];
  const points = top.sizeOverTime || [];
  const maxSize = Math.max(...points.map((p) => p.size), 1);
  const width = 640;
  const coords = points.map((p, i) => {
    const x = points.length === 1 ? 40 : 20 + (i / (points.length - 1)) * (width - 40);
    const y = 140 - (p.size / maxSize) * 110;
    return `${x},${y}`;
  });
  el.innerHTML = `<svg viewBox="0 0 ${width} 170" width="100%" height="170">
    <polyline fill="none" stroke="var(--blue)" stroke-width="2.5" points="${coords.join(" ")}"></polyline>
    <text x="20" y="20">${escapeHtml(top.name)} size</text>
  </svg>`;
}

function renderSessions(el, sessions) {
  if (!sessions.length) {
    el.innerHTML = `<li class="chart-empty">No sessions yet</li>`;
    return;
  }
  el.innerHTML = sessions
    .slice()
    .reverse()
    .slice(0, 12)
    .map((s) => {
      const start = new Date(s.start).toLocaleString();
      const end = new Date(s.end).toLocaleString();
      return `<li><strong>${formatMinutes(s.minutes)}</strong> · ${s.saves} saves<br><span>${escapeHtml(start)} → ${escapeHtml(end)}</span></li>`;
    })
    .join("");
}

/* ———————————————————— Graph preview ———————————————————— */

function renderGraph() {
  const el = $("graph-view");
  if (!files.length) {
    el.innerHTML = `<div class="chart-empty">Track some files to see them here.</div>`;
    return;
  }
  const W = 800;
  const H = 520;
  const cx = W / 2;
  const cy = H / 2;
  const radius = Math.min(W, H) / 2 - 80;
  const center = projectLabelEl.textContent.replace(/^Project:\s*/, "") || "Strata";
  const nodes = files.map((f, i) => {
    const angle = (i / files.length) * Math.PI * 2 - Math.PI / 2;
    return { f, x: cx + Math.cos(angle) * radius, y: cy + Math.sin(angle) * radius };
  });
  el.innerHTML = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="xMidYMid meet">
    ${nodes.map((n) => `<line class="g-edge" x1="${cx}" y1="${cy}" x2="${n.x}" y2="${n.y}"/>`).join("")}
    <circle class="g-node center" cx="${cx}" cy="${cy}" r="44"/>
    <text x="${cx}" y="${cy + 4}" text-anchor="middle" font-weight="600">${escapeHtml(center)}</text>
    ${nodes
      .map(
        (n) => `<g style="cursor:pointer" data-path="${escapeHtml(n.f.path)}">
          <circle class="g-node" cx="${n.x}" cy="${n.y}" r="26"/>
          <text x="${n.x}" y="${n.y + 3}" text-anchor="middle" font-size="10" font-weight="700">${fileBadge(n.f.path)}</text>
          <text x="${n.x}" y="${n.y + 44}" text-anchor="middle">${escapeHtml(baseName(n.f.path))}</text>
        </g>`
      )
      .join("")}
  </svg>`;
  el.querySelectorAll("g[data-path]").forEach((g) => {
    g.addEventListener("click", () => {
      setAppMode("files");
      selectFile(g.dataset.path);
    });
  });
}

/* ———————————————————— Wiring ———————————————————— */

$("nav-files").addEventListener("click", () => setAppMode("files"));
$("nav-stats").addEventListener("click", () => setAppMode("stats"));
$("nav-graph").addEventListener("click", () => setAppMode("graph"));
$("refresh-stats").addEventListener("click", loadDashboard);

$("add-btn").addEventListener("click", addFileFlow);
$("empty-add-btn").addEventListener("click", addFileFlow);
$("find-btn").addEventListener("click", () => handlePrompt("Find files to track"));
$("create-branch-btn").addEventListener("click", openBranchModal);
$("theme-toggle").addEventListener("click", () => {
  applyTheme(document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark");
});

branchFilterBtn.addEventListener("click", () => {
  showAllBranches = !showAllBranches;
  renderVersions();
});
branchSelectEl.addEventListener("change", () => switchBranch(branchSelectEl.value));

$("tab-diff").addEventListener("click", async () => {
  setActiveView("diff");
  await refreshView();
});
$("tab-content").addEventListener("click", async () => {
  setActiveView("content");
  await refreshView();
});
$("mode-inline").addEventListener("click", () => {
  diffMode = "inline";
  $("mode-inline").classList.add("active");
  $("mode-side").classList.remove("active");
  renderDiff();
});
$("mode-side").addEventListener("click", () => {
  diffMode = "side";
  $("mode-side").classList.add("active");
  $("mode-inline").classList.remove("active");
  renderDiff();
});
prevChangeBtn.addEventListener("click", () => jumpChange(-1));
nextChangeBtn.addEventListener("click", () => jumpChange(1));
fromVerEl.addEventListener("change", () => {
  fromVersion = Number(fromVerEl.value);
  expandedFolds = new Set();
  refreshView();
});
restoreBtn.addEventListener("click", () => {
  const n = Number(restoreBtn.dataset.version);
  if (n) restoreFlow(n);
});

searchInput.addEventListener("input", () => {
  searchQuery = searchInput.value;
  renderFiles();
  renderVersions();
});
searchInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && searchInput.value.trim()) {
    handlePrompt(searchInput.value);
    searchInput.value = "";
    searchQuery = "";
    renderFiles();
    renderVersions();
  }
  if (e.key === "Escape") {
    searchInput.value = "";
    searchQuery = "";
    renderFiles();
    renderVersions();
    searchInput.blur();
  }
});
$("ask-ai-btn").addEventListener("click", () => {
  if (searchInput.value.trim()) {
    handlePrompt(searchInput.value);
    searchInput.value = "";
    searchQuery = "";
    renderFiles();
    renderVersions();
  } else {
    agentInput.focus();
  }
});

$("agent-form").addEventListener("submit", (e) => {
  e.preventDefault();
  const text = agentInput.value;
  agentInput.value = "";
  handlePrompt(text);
});

document.addEventListener("keydown", (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
    e.preventDefault();
    searchInput.focus();
    searchInput.select();
  }
});

$("branch-cancel").addEventListener("click", closeBranchModal);
$("branch-confirm").addEventListener("click", confirmBranch);
branchNameInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") confirmBranch();
  if (e.key === "Escape") closeBranchModal();
});
branchModalEl.addEventListener("click", (e) => {
  if (e.target === branchModalEl) closeBranchModal();
});

EventsOn("version:created", async (payload) => {
  if (payload?.path && payload?.versionNum != null) {
    noteVersionOnCurrentBranch(payload.path, payload.versionNum);
  }
  if (appMode === "stats") {
    await loadDashboard();
    return;
  }
  const followLatest = payload?.path === selectedPath && selectedVersion === latestVersion();
  await refreshFiles(selectedPath || payload?.path);
  if (followLatest) await selectVersion(latestVersion());
  if (payload?.path) showToast(`${baseName(payload.path)} · new version v${payload.versionNum}`);
});

setActiveView("diff");
setAppMode("files");
greet();
refreshFiles().catch((err) => showToast(String(err)));
