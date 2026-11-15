/** Best-effort semver-ish compare for catalog update badges (PD-UX-10). */
export function parseVersionParts(raw: string): number[] {
  const core = String(raw || "")
    .trim()
    .replace(/^v/i, "")
    .split(/[-+]/)[0] ?? "";
  return core.split(".").map((p) => {
    const n = parseInt(p.replace(/^\D+/, ""), 10);
    return Number.isFinite(n) ? n : 0;
  });
}

/** True when `candidate` is strictly newer than `current`. */
export function isNewerVersion(candidate: string, current: string): boolean {
  const a = parseVersionParts(candidate);
  const b = parseVersionParts(current);
  const len = Math.max(a.length, b.length);
  for (let i = 0; i < len; i++) {
    const x = a[i] ?? 0;
    const y = b[i] ?? 0;
    if (x > y) return true;
    if (x < y) return false;
  }
  return false;
}
