/** Krate medias/<client>/… → display family (avoids duplicate "Downloads" basenames). */
export const ROOT_FAMILY: Record<string, string> = {
  rtorrent: "rTorrent",
  deluge: "Deluge",
  qbittorrent: "qBittorrent",
  transmission: "Transmission",
  jdownloader: "JDownloader",
  pyload: "pyLoad",
  nzbget: "NZBGet",
  sabnzbd: "SABnzbd",
};

export function pathParts(path: string): string[] {
  return path.replace(/\\/g, "/").replace(/\/+$/, "").split("/").filter(Boolean);
}

export function rootAppId(path: string): string {
  const parts = pathParts(path);
  const parent = (parts[parts.length - 2] || "").toLowerCase();
  if (ROOT_FAMILY[parent]) return parent;
  return parts[parts.length - 2] || parts[parts.length - 1] || path;
}

/** Distinct label for a download_roots path (e.g. "rTorrent · downloads"). */
export function rootLabel(path: string): string {
  const parts = pathParts(path);
  const base = parts[parts.length - 1] || path;
  const parent = parts[parts.length - 2] || "";
  const family = ROOT_FAMILY[parent.toLowerCase()];
  if (family) return `${family} · ${base}`;
  if (parent) return `${parent}/${base}`;
  return base;
}

export type RootGroup = {
  id: string;
  label: string;
  roots: string[];
};

/** Group download_roots by client/app for pickers and sidebars. */
export function groupRootsByApp(roots: string[]): RootGroup[] {
  const byId = new Map<string, RootGroup>();
  for (const r of roots) {
    const id = rootAppId(r);
    const label = ROOT_FAMILY[id] || id;
    const cur = byId.get(id);
    if (cur) cur.roots.push(r);
    else byId.set(id, { id, label, roots: [r] });
  }
  return [...byId.values()].sort((a, b) => a.label.localeCompare(b.label));
}

export function preferDownloadsRoot(appRoots: string[]): string {
  const downloads = appRoots.find((r) => /\/downloads$/i.test(r) || /\/Downloads$/.test(r));
  return downloads || appRoots[0] || "";
}

/** True when path looks like …/mounts/<name> or …/mount/<name> (rclone/FUSE layout). */
export function isMountRootPath(path: string): boolean {
  const parts = pathParts(path);
  const i = parts.findIndex((p) => p === "mounts" || p === "mount");
  return i >= 0 && i < parts.length - 1;
}

export function mountLabel(path: string): string {
  const parts = pathParts(path);
  const i = parts.findIndex((p) => p === "mounts" || p === "mount");
  if (i >= 0 && parts[i + 1]) return parts[i + 1]!;
  return parts[parts.length - 1] || path;
}

export const FILEMANAGER_EXT_ID = "org.fleet.filemanager";
export const RCLONE_EXT_ID = "org.fleet.rclone";
