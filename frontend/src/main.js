import "./style.css";
import {
  ListFiles,
  AddFile,
  RemoveFile,
  History,
  ShowVersion,
  RestoreVersion,
} from "../wailsjs/go/main/App.js";
import { EventsOn } from "../wailsjs/runtime/runtime.js";

const fileListEl = document.getElementById("file-list");
const versionListEl = document.getElementById("version-list");
const emptyStateEl = document.getElementById("empty-state");
const workspaceEl = document.getElementById("workspace");
const selectedPathEl = document.getElementById("selected-path");
const contentTitleEl = document.getElementById("content-title");
const contentViewEl = document.getElementById("content-view");
const addBtn = document.getElementById("add-btn");
const removeBtn = document.getElementById("remove-btn");
const restoreBtn = document.getElementById("restore-btn");
const toastEl = document.getElementById("toast");

let files = [];
let selectedPath = null;
let selectedVersion = null;
let versions = [];

function baseName(path) {
  const parts = path.replace(/\\/g, "/").split("/");
  return parts[parts.length - 1] || path;
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
  emptyStateEl.classList.toggle("hidden", visible);
  workspaceEl.classList.toggle("hidden", !visible);
}

async function refreshFiles(preferPath = selectedPath) {
  files = (await ListFiles()) || [];
  renderFiles();
  if (preferPath && files.some((f) => f.path === preferPath)) {
    await selectFile(preferPath);
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
      <span class="file-name">${baseName(file.path)}</span>
      <span class="file-meta">${file.versionCount} version${file.versionCount === 1 ? "" : "s"}${
        file.missing ? ' · <span class="badge-missing">missing</span>' : ""
      }</span>
    `;
    li.addEventListener("click", () => selectFile(file.path));
    fileListEl.appendChild(li);
  }
}

async function selectFile(path) {
  selectedPath = path;
  selectedVersion = null;
  removeBtn.disabled = false;
  restoreBtn.disabled = true;
  setWorkspaceVisible(true);
  selectedPathEl.textContent = path;
  contentTitleEl.textContent = "Content";
  contentViewEl.textContent = "Select a version to view its contents.";
  renderFiles();
  versions = (await History(path)) || [];
  renderVersions();
  if (versions.length > 0) {
    await selectVersion(versions[versions.length - 1].number);
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
      <span class="version-meta">${v.createdAt} · ${v.size} bytes · ${formatChanges(v)}</span>
    `;
    li.addEventListener("click", () => selectVersion(v.number));
    versionListEl.appendChild(li);
  }
}

async function selectVersion(number) {
  selectedVersion = number;
  restoreBtn.disabled = false;
  renderVersions();
  contentTitleEl.textContent = `Version ${number}`;
  try {
    const text = await ShowVersion(selectedPath, number);
    contentViewEl.textContent = text === "" ? "(empty file)" : text;
  } catch (err) {
    contentViewEl.textContent = String(err);
  }
}

addBtn.addEventListener("click", async () => {
  try {
    const file = await AddFile();
    showToast(`Tracking ${baseName(file.path)}`);
    await refreshFiles(file.path);
  } catch (err) {
    if (!String(err).includes("cancelled")) {
      showToast(String(err));
    }
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
    if (res.createdSafetyNet) {
      msg += ` (saved current as v${res.safetyVersionNum})`;
    }
    showToast(msg);
    await selectFile(selectedPath);
  } catch (err) {
    showToast(String(err));
  }
});

EventsOn("version:created", async (payload) => {
  await refreshFiles(payload?.path || selectedPath);
  if (payload?.path === selectedPath) {
    showToast(`New version ${payload.versionNum}`);
  }
});

refreshFiles().catch((err) => showToast(String(err)));
