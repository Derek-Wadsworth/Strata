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

const fileListEl = document.getElementById("file-list");
const versionListEl = document.getElementById("version-list");
const emptyStateEl = document.getElementById("empty-state");
const workspaceEl = document.getElementById("workspace");
const statsPanelEl = document.getElementById("stats-panel");
const selectedPathEl = document.getElementById("selected-path");
const contentViewEl = document.getElementById("content-view");
const diffViewEl = document.getElementById("diff-view");
const diffControlsEl = document.getElementById("diff-controls");
const diffStatEl = document.getElementById("diff-stat");
const fromVerEl = document.getElementById("from-ver");
const toVerEl = document.getElementById("to-ver");
const addBtn = document.getElementById("add-btn");
const removeBtn = document.getElementById("remove-btn");
const restoreBtn = document.getElementById("restore-btn");
const prevChangeBtn = document.getElementById("prev-change");
const nextChangeBtn = document.getElementById("next-change");
const toastEl = document.getElementById("toast");
const navFilesBtn = document.getElementById("nav-files");
const navStatsBtn = document.getElementById("nav-stats");
const fileActionsEl = document.getElementById("file-actions");

let files = [];
let selectedPath = null;
let selectedVersion = null;
let versions = [];
let activeView = "diff"; // diff | content
let appMode = "files"; // files | stats
let diffMode = "inline"; // inline | side
let currentDiff = null;
let changeIndexes = [];
let changeCursor = -1;
let syncingSelectors = false;

function baseName(path) {
  const parts = path.replace(/\\/g, "/").split("/");
  return parts[parts.length - 1] || path;
}

function escapeHtml(text) {
  return String(text)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function highlightLine(text, path) {
  const escaped = escapeHtml(text);
  const ext = (baseName(path).split(".").pop() || "").toLowerCase();
  if (!["go", "js", "ts", "jsx", "tsx", "py", "css", "html", "json", "md", "sh"].includes(ext)) {
    return escaped;
  }
  return escaped
    .replace(
      /\b(func|package|import|return|const|let|var|if|else|for|while|class|def|from|async|await|type|struct|interface|switch|case)\b/g,
      '<span class="tok-kw">$1</span>'
    )
    .replace(/(&quot;|&#39;|")(?:\\.|(?!\1).)*\1/g, '<span class="tok-str">$&</span>')
    .replace(/(\/\/.*?$|#.*?$)/gm, '<span class="tok-comment">$1</span>');
}

function formatChanges(v) {
  if (v.linesAdded == null || v.linesRemoved == null) return "—";
  return `+${v.linesAdded} -${v.linesRemoved}`;
}

function showToast(message) {
  toastEl.textContent = message;
  toastEl.classList.remove("hidden");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(() => toastEl.classList.add("hidden"), 2800);
}

function setWorkspaceVisible(visible) {
  emptyStateEl.classList.toggle("hidden", visible || appMode === "stats");
  workspaceEl.classList.toggle("hidden", !visible || appMode === "stats");
}

function setAppMode(mode) {
  appMode = mode;
  navFilesBtn.classList.toggle("active", mode === "files");
  navStatsBtn.classList.toggle("active", mode === "stats");
  fileActionsEl.classList.toggle("hidden", mode !== "files");
  fileListEl.classList.toggle("hidden", mode !== "files");
  statsPanelEl.classList.toggle("hidden", mode !== "stats");
  if (mode === "stats") {
    emptyStateEl.classList.add("hidden");
    workspaceEl.classList.add("hidden");
    loadDashboard();
  } else {
    setWorkspaceVisible(!!selectedPath);
  }
}

function formatMinutes(mins) {
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return m ? `${h}h ${m}m` : `${h}h`;
}

async function loadDashboard() {
  try {
    const dash = await GetDashboard();
    renderDashboard(dash || {});
  } catch (err) {
    showToast(String(err));
  }
}

function renderDashboard(dash) {
  const cards = document.getElementById("stat-cards");
  cards.innerHTML = `
    <div class="stat-card"><span>Files</span><strong>${dash.totalFiles || 0}</strong></div>
    <div class="stat-card"><span>Versions</span><strong>${dash.totalVersions || 0}</strong></div>
    <div class="stat-card"><span>Lines</span><strong>+${dash.linesAdded || 0} / -${dash.linesRemoved || 0}</strong></div>
    <div class="stat-card"><span>Active time</span><strong>${formatMinutes(dash.estimatedActiveMinutes || 0)}</strong></div>
    <div class="stat-card"><span>Sessions</span><strong>${dash.sessionCount || 0}</strong></div>
  `;

  renderBarChart(document.getElementById("edits-chart"), dash.editsPerDay || [], (d) => d.day.slice(5), (d) => d.count);
  renderBarChart(
    document.getElementById("active-chart"),
    dash.mostActive || [],
    (d) => d.name,
    (d) => d.versions
  );
  renderHeatmap(document.getElementById("heatmap"), dash.heatmap || []);
  renderSizeChart(document.getElementById("size-chart"), dash.files || []);
  renderSessions(document.getElementById("session-list"), dash.sessions || []);
}

function renderBarChart(el, rows, labelFn, valueFn) {
  if (!rows.length) {
    el.innerHTML = `<div class="chart-empty">No data yet</div>`;
    return;
  }
  const max = Math.max(...rows.map(valueFn), 1);
  const width = Math.max(320, rows.length * 36);
  const height = 160;
  const bars = rows
    .map((row, i) => {
      const v = valueFn(row);
      const h = Math.max(2, Math.round((v / max) * 110));
      const x = 24 + i * 36;
      const y = 130 - h;
      return `<rect class="bar" x="${x}" y="${y}" width="22" height="${h}" rx="4"></rect>
        <text x="${x + 11}" y="148" text-anchor="middle">${escapeHtml(labelFn(row))}</text>
        <text x="${x + 11}" y="${y - 6}" text-anchor="middle">${v}</text>`;
    })
    .join("");
  el.innerHTML = `<svg viewBox="0 0 ${width} ${height}" width="100%" height="${height}">${bars}</svg>`;
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
  start.setUTCDate(end.getUTCDate() - 119); // ~17 weeks
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
  // Show the most active file's size trend for clarity.
  const top = [...series].sort((a, b) => b.totalVersions - a.totalVersions)[0];
  const points = top.sizeOverTime || [];
  const maxSize = Math.max(...points.map((p) => p.size), 1);
  const width = 640;
  const height = 170;
  const coords = points.map((p, i) => {
    const x = points.length === 1 ? 40 : 20 + (i / (points.length - 1)) * (width - 40);
    const y = 140 - (p.size / maxSize) * 110;
    return `${x},${y}`;
  });
  el.innerHTML = `<svg viewBox="0 0 ${width} ${height}" width="100%" height="${height}">
    <polyline fill="none" stroke="#0f766e" stroke-width="3" points="${coords.join(" ")}"></polyline>
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

function setActiveView(view) {
  activeView = view;
  document.getElementById("tab-diff").classList.toggle("active", view === "diff");
  document.getElementById("tab-content").classList.toggle("active", view === "content");
  diffControlsEl.classList.toggle("hidden", view !== "diff");
  diffViewEl.classList.toggle("hidden", view !== "diff");
  contentViewEl.classList.toggle("hidden", view !== "content");
}

async function refreshFiles(preferPath = selectedPath) {
  files = (await ListFiles()) || [];
  renderFiles();
  if (preferPath && files.some((f) => f.path === preferPath)) {
    await selectFile(preferPath, selectedVersion);
  } else if (files.length > 0) {
    await selectFile(files[0].path);
  } else {
    selectedPath = null;
    selectedVersion = null;
    setWorkspaceVisible(false);
    removeBtn.disabled = true;
    restoreBtn.disabled = true;
  }
}

function renderFiles() {
  fileListEl.innerHTML = "";
  for (const file of files) {
    const li = document.createElement("li");
    li.className = file.path === selectedPath ? "active" : "";
    li.innerHTML = `
      <span class="file-name">${escapeHtml(baseName(file.path))}</span>
      <span class="file-meta">${file.versionCount} version${file.versionCount === 1 ? "" : "s"}${
        file.missing ? ' · <span class="badge-missing">missing</span>' : ""
      }</span>
    `;
    li.addEventListener("click", () => selectFile(file.path));
    fileListEl.appendChild(li);
  }
}

function fillVersionSelectors(toVer) {
  syncingSelectors = true;
  fromVerEl.innerHTML = "";
  toVerEl.innerHTML = "";
  for (const v of versions) {
    const toOpt = document.createElement("option");
    toOpt.value = String(v.number);
    toOpt.textContent = `v${v.number}`;
    toVerEl.appendChild(toOpt);

    if (v.number < (toVer || versions[versions.length - 1]?.number || 1)) {
      const fromOpt = document.createElement("option");
      fromOpt.value = String(v.number);
      fromOpt.textContent = `v${v.number}`;
      fromVerEl.appendChild(fromOpt);
    }
  }
  if (toVer) toVerEl.value = String(toVer);
  const preferredFrom = Math.max(1, (toVer || 1) - 1);
  if ([...fromVerEl.options].some((o) => o.value === String(preferredFrom))) {
    fromVerEl.value = String(preferredFrom);
  } else if (fromVerEl.options.length > 0) {
    fromVerEl.selectedIndex = fromVerEl.options.length - 1;
  }
  syncingSelectors = false;
}

async function selectFile(path, keepVersion = null) {
  selectedPath = path;
  selectedVersion = null;
  removeBtn.disabled = false;
  restoreBtn.disabled = true;
  setWorkspaceVisible(true);
  selectedPathEl.textContent = path;
  renderFiles();
  versions = (await History(path)) || [];
  renderVersions();
  const target =
    keepVersion && versions.some((v) => v.number === keepVersion)
      ? keepVersion
      : versions.length > 0
        ? versions[versions.length - 1].number
        : null;
  if (target != null) {
    await selectVersion(target);
  } else {
    diffViewEl.innerHTML = `<div class="diff-empty">No versions yet.</div>`;
    contentViewEl.textContent = "No versions yet.";
  }
}

function renderVersions() {
  versionListEl.innerHTML = "";
  const ordered = [...versions].reverse();
  for (const v of ordered) {
    const li = document.createElement("li");
    li.className = v.number === selectedVersion ? "active" : "";
    li.innerHTML = `
      <span class="version-num">v${v.number}</span>
      <span class="version-meta">${escapeHtml(v.createdAt)} · ${v.size} bytes · ${formatChanges(v)}</span>
    `;
    li.addEventListener("click", () => selectVersion(v.number));
    versionListEl.appendChild(li);
  }
}

async function selectVersion(number) {
  selectedVersion = number;
  restoreBtn.disabled = false;
  renderVersions();
  fillVersionSelectors(number);
  if (activeView === "content") {
    await loadContent(number);
  } else {
    await loadDiff();
  }
}

async function loadContent(number) {
  try {
    const text = await ShowVersion(selectedPath, number);
    contentViewEl.textContent = text === "" ? "(empty file)" : text;
  } catch (err) {
    contentViewEl.textContent = String(err);
  }
}

async function loadDiff() {
  if (!selectedPath || !selectedVersion) return;
  const toVer = Number(toVerEl.value || selectedVersion);
  const fromVer = Number(fromVerEl.value || Math.max(1, toVer - 1));

  if (toVer === 1 && versions.length === 1) {
    currentDiff = null;
    changeIndexes = [];
    changeCursor = -1;
    updateChangeNav();
    diffStatEl.textContent = "v1 has no previous version to compare";
    diffViewEl.innerHTML = `<div class="diff-empty">This is the first version. Add another save to see a diff.</div>`;
    return;
  }

  try {
    currentDiff = await DiffVersions(selectedPath, fromVer, toVer);
    renderDiff();
  } catch (err) {
    currentDiff = null;
    diffViewEl.innerHTML = `<div class="diff-empty">${escapeHtml(String(err))}</div>`;
    diffStatEl.textContent = "";
    changeIndexes = [];
    updateChangeNav();
  }
}

function renderDiff() {
  if (!currentDiff) return;

  if (currentDiff.binary) {
    diffStatEl.textContent = `v${currentDiff.fromVer} → v${currentDiff.toVer} · binary files differ`;
    diffViewEl.innerHTML = `
      <div class="diff-empty binary">
        <strong>Binary files differ</strong>
        <p>Strata versions binary files, but does not show a line diff. Use Restore or Export to inspect the file outside Strata.</p>
      </div>`;
    changeIndexes = [];
    updateChangeNav();
    return;
  }

  diffStatEl.textContent = `v${currentDiff.fromVer} → v${currentDiff.toVer} · +${currentDiff.linesAdded} -${currentDiff.linesRemoved}`;
  changeIndexes = [];
  changeCursor = -1;

  if (diffMode === "side") {
    renderSideBySide(currentDiff.lines || []);
  } else {
    renderInline(currentDiff.lines || []);
  }
  updateChangeNav();
}

function renderInline(lines) {
  const rows = lines
    .map((line, idx) => {
      const isChange = line.op === "add" || line.op === "remove";
      if (isChange) changeIndexes.push(idx);
      const sign = line.op === "add" ? "+" : line.op === "remove" ? "-" : " ";
      const oldNo = line.oldNo || "";
      const newNo = line.newNo || "";
      return `<div class="diff-row ${line.op}" data-idx="${idx}">
        <span class="ln old">${oldNo}</span>
        <span class="ln new">${newNo}</span>
        <span class="sign">${sign}</span>
        <code>${highlightLine(line.text, selectedPath)}</code>
      </div>`;
    })
    .join("");
  diffViewEl.innerHTML = `<div class="diff-inline">${rows || '<div class="diff-empty">No changes</div>'}</div>`;
}

function renderSideBySide(lines) {
  const left = [];
  const right = [];
  lines.forEach((line, idx) => {
    if (line.op === "equal") {
      left.push({ idx, op: "equal", no: line.oldNo, text: line.text, change: false });
      right.push({ idx, op: "equal", no: line.newNo, text: line.text, change: false });
    } else if (line.op === "remove") {
      changeIndexes.push(idx);
      left.push({ idx, op: "remove", no: line.oldNo, text: line.text, change: true });
      right.push({ idx, op: "gap", no: "", text: "", change: true });
    } else if (line.op === "add") {
      changeIndexes.push(idx);
      left.push({ idx, op: "gap", no: "", text: "", change: true });
      right.push({ idx, op: "add", no: line.newNo, text: line.text, change: true });
    }
  });

  const max = Math.max(left.length, right.length);
  let html = `<div class="diff-side">`;
  for (let i = 0; i < max; i++) {
    const l = left[i] || { op: "gap", no: "", text: "", idx: -1 };
    const r = right[i] || { op: "gap", no: "", text: "", idx: -1 };
    html += `<div class="side-row" data-idx="${l.idx >= 0 ? l.idx : r.idx}">
      <div class="side-cell ${l.op}"><span class="ln">${l.no}</span><code>${highlightLine(l.text, selectedPath)}</code></div>
      <div class="side-cell ${r.op}"><span class="ln">${r.no}</span><code>${highlightLine(r.text, selectedPath)}</code></div>
    </div>`;
  }
  html += `</div>`;
  diffViewEl.innerHTML = html || `<div class="diff-empty">No changes</div>`;
}

function updateChangeNav() {
  const has = changeIndexes.length > 0;
  prevChangeBtn.disabled = !has;
  nextChangeBtn.disabled = !has;
}

function jumpChange(delta) {
  if (changeIndexes.length === 0) return;
  if (changeCursor < 0) {
    changeCursor = delta > 0 ? 0 : changeIndexes.length - 1;
  } else {
    changeCursor = (changeCursor + delta + changeIndexes.length) % changeIndexes.length;
  }
  const idx = changeIndexes[changeCursor];
  const el =
    diffViewEl.querySelector(`.diff-row[data-idx="${idx}"]`) ||
    diffViewEl.querySelector(`.side-row[data-idx="${idx}"]`);
  if (!el) return;
  diffViewEl.querySelectorAll(".current-change").forEach((n) => n.classList.remove("current-change"));
  el.classList.add("current-change");
  el.scrollIntoView({ block: "center", behavior: "smooth" });
}

document.getElementById("tab-diff").addEventListener("click", async () => {
  setActiveView("diff");
  await loadDiff();
});
document.getElementById("tab-content").addEventListener("click", async () => {
  setActiveView("content");
  if (selectedVersion != null) await loadContent(selectedVersion);
});
document.getElementById("mode-inline").addEventListener("click", () => {
  diffMode = "inline";
  document.getElementById("mode-inline").classList.add("active");
  document.getElementById("mode-side").classList.remove("active");
  renderDiff();
});
document.getElementById("mode-side").addEventListener("click", () => {
  diffMode = "side";
  document.getElementById("mode-side").classList.add("active");
  document.getElementById("mode-inline").classList.remove("active");
  renderDiff();
});

fromVerEl.addEventListener("change", () => {
  if (!syncingSelectors) loadDiff();
});
toVerEl.addEventListener("change", () => {
  if (syncingSelectors) return;
  const toVer = Number(toVerEl.value);
  selectedVersion = toVer;
  renderVersions();
  fillVersionSelectors(toVer);
  loadDiff();
});

prevChangeBtn.addEventListener("click", () => jumpChange(-1));
nextChangeBtn.addEventListener("click", () => jumpChange(1));

addBtn.addEventListener("click", async () => {
  try {
    const file = await AddFile();
    showToast(`Tracking ${baseName(file.path)}`);
    await refreshFiles(file.path);
  } catch (err) {
    if (!String(err).includes("cancelled")) showToast(String(err));
  }
});

removeBtn.addEventListener("click", async () => {
  if (!selectedPath) return;
  const ok = window.confirm(`Stop tracking this file?\n\n${selectedPath}\n\nHistory will be kept.`);
  if (!ok) return;
  try {
    await RemoveFile(selectedPath);
    showToast(`Stopped tracking ${baseName(selectedPath)}`);
    await refreshFiles(null);
  } catch (err) {
    showToast(String(err));
  }
});

restoreBtn.addEventListener("click", async () => {
  if (!selectedPath || selectedVersion == null) return;
  const ok = window.confirm(
    `Restore version ${selectedVersion}?\n\nCurrent content will be snapshotted first if it differs.`
  );
  if (!ok) return;
  try {
    const res = await RestoreVersion(selectedPath, selectedVersion);
    let msg = `Restored v${res.restoredVersion}`;
    if (res.createdSafetyNet) msg += ` (saved current as v${res.safetyVersionNum})`;
    showToast(msg);
    await selectFile(selectedPath);
  } catch (err) {
    showToast(String(err));
  }
});

navFilesBtn.addEventListener("click", () => setAppMode("files"));
navStatsBtn.addEventListener("click", () => setAppMode("stats"));
document.getElementById("refresh-stats").addEventListener("click", () => loadDashboard());

EventsOn("version:created", async (payload) => {
  if (appMode === "stats") {
    await loadDashboard();
    return;
  }
  await refreshFiles(payload?.path || selectedPath);
  if (payload?.path === selectedPath) {
    showToast(`New version ${payload.versionNum}`);
  }
});

setActiveView("diff");
setAppMode("files");
refreshFiles().catch((err) => showToast(String(err)));
