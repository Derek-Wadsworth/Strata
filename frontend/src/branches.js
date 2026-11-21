/** Pure helpers for branch lanes / filtering. Branch data comes from the Go API. */

export function laneOf(branches, name) {
  const idx = (branches || []).findIndex((b) => b.name === name);
  return idx < 0 ? 0 : idx;
}

export function branchMeta(branches, name) {
  return (branches || []).find((b) => b.name === name) || null;
}

export function versionsOnBranch(versions, name) {
  return (versions || []).filter((v) => (v.branch || "main") === name);
}

/** Versions reachable from a branch: its own versions plus main up to the fork point. */
export function versionVisibleOnBranch(versions, branches, versionNumber, name) {
  const own = (versions || []).find((v) => v.number === versionNumber);
  const branch = own?.branch || "main";
  if (branch === name) return true;
  if (name === "main") return false;
  const info = branchMeta(branches, name);
  return branch === "main" && info != null && versionNumber <= (info.fromVersion || 0);
}
