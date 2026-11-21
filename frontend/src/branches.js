const BRANCH_KEY = "strata.branches.v2";
const TITLE_KEY = "strata.titles.v1";

function load(key) {
  try {
    return JSON.parse(localStorage.getItem(key) || "{}") || {};
  } catch {
    return {};
  }
}

function save(key, value) {
  localStorage.setItem(key, JSON.stringify(value));
}

function blank() {
  return {
    current: "main",
    branches: [{ name: "main", fromVersion: 0, createdAt: new Date().toISOString() }],
    assignments: {},
  };
}

function stateFor(path) {
  const all = load(BRANCH_KEY);
  const state = all[path] || blank();
  if (!state.branches?.some((b) => b.name === "main")) state.branches = blank().branches.concat(state.branches || []);
  state.assignments ||= {};
  state.current ||= "main";
  return state;
}

function store(path, state) {
  const all = load(BRANCH_KEY);
  all[path] = state;
  save(BRANCH_KEY, all);
}

/** Branches in creation order, main first. Lane index = position. */
export function listBranches(path) {
  return stateFor(path).branches.slice();
}

export function laneOf(path, name) {
  const idx = listBranches(path).findIndex((b) => b.name === name);
  return idx < 0 ? 0 : idx;
}

export function getCurrentBranch(path) {
  return stateFor(path).current;
}

export function setCurrentBranch(path, name) {
  const state = stateFor(path);
  if (!state.branches.some((b) => b.name === name)) throw new Error(`Unknown branch: ${name}`);
  state.current = name;
  store(path, state);
}

export function branchOf(path, versionNumber) {
  return stateFor(path).assignments[String(versionNumber)] || "main";
}

export function branchInfo(path, name) {
  return stateFor(path).branches.find((b) => b.name === name) || null;
}

export function versionsOnBranch(path, name, versions) {
  return versions.filter((v) => branchOf(path, v.number) === name);
}

/** Versions reachable from a branch: its own versions plus main up to the fork point. */
export function versionVisibleOnBranch(path, versionNumber, name) {
  const own = branchOf(path, versionNumber);
  if (own === name) return true;
  if (name === "main") return false;
  const info = branchInfo(path, name);
  return own === "main" && info != null && versionNumber <= info.fromVersion;
}

export function createBranch(path, rawName, fromVersion) {
  const name = String(rawName || "")
    .trim()
    .toLowerCase()
    .replace(/\s+/g, "-")
    .replace(/[^a-z0-9._/-]/g, "")
    .slice(0, 40);
  if (!name) throw new Error("Enter a branch name");
  const state = stateFor(path);
  if (state.branches.some((b) => b.name === name)) throw new Error(`"${name}" already exists`);
  state.branches.push({ name, fromVersion: fromVersion || 0, createdAt: new Date().toISOString() });
  state.current = name;
  store(path, state);
  return name;
}

export function noteVersionOnCurrentBranch(path, versionNumber) {
  const state = stateFor(path);
  if (state.current !== "main") {
    state.assignments[String(versionNumber)] = state.current;
    store(path, state);
  }
}

export function getTitle(path, versionNumber) {
  return load(TITLE_KEY)[path]?.[String(versionNumber)] || "";
}

export function setTitle(path, versionNumber, title) {
  const all = load(TITLE_KEY);
  all[path] ||= {};
  const clean = String(title || "").trim().slice(0, 80);
  if (clean) all[path][String(versionNumber)] = clean;
  else delete all[path][String(versionNumber)];
  save(TITLE_KEY, all);
}
