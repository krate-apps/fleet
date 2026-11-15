/** Map peer/client version strings to a stable family id for logos. */

const FAMILIES: Array<{ id: string; label: string; match: RegExp }> = [
  { id: "qbittorrent", label: "qBittorrent", match: /qbittorrent|\bqbit\b/i },
  { id: "utorrent", label: "µTorrent", match: /utorrent|μtorrent|\bu(?:torrent)?\b/i },
  { id: "transmission", label: "Transmission", match: /transmission/i },
  { id: "deluge", label: "Deluge", match: /deluge/i },
  { id: "rtorrent", label: "rTorrent", match: /rtorrent|libtorrent\s*rasterbar|rakshasa/i },
  { id: "aria2", label: "aria2", match: /aria2/i },
  { id: "bittorrent", label: "BitTorrent", match: /\bbittorrent\b|\bbt\b/i },
  { id: "vuze", label: "Vuze", match: /vuze|azureus/i },
  { id: "tixati", label: "Tixati", match: /tixati/i },
  { id: "bitcomet", label: "BitComet", match: /bitcomet/i },
  { id: "libtorrent", label: "libtorrent", match: /libtorrent/i },
];

export function clientFamily(raw: string): string {
  const s = raw.trim();
  if (!s) return "unknown";
  for (const f of FAMILIES) {
    if (f.match.test(s)) return f.id;
  }
  return "unknown";
}

export function clientLabel(family: string, raw = ""): string {
  const known = FAMILIES.find((f) => f.id === family);
  if (known) return known.label;
  return raw.trim() || "Unknown client";
}

/** Pick the most frequent peer client version string. */
export function dominantClient(versions: string[]): string {
  const counts = new Map<string, number>();
  for (const v of versions) {
    const t = v.trim();
    if (!t) continue;
    counts.set(t, (counts.get(t) ?? 0) + 1);
  }
  let best = "";
  let n = 0;
  for (const [k, c] of counts) {
    if (c > n) {
      best = k;
      n = c;
    }
  }
  return best;
}
