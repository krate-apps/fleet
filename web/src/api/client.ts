import type { ExtensionRecord } from "@/extensions/types";

export type { ExtensionRecord, UIContribution } from "@/extensions/types";

/** Public path prefix when Fleet sits behind a reverse-proxy base (Vite `base`). */
export function uiBase(): string {
  const raw = import.meta.env.BASE_URL || "/";
  return raw.endsWith("/") ? raw.slice(0, -1) : raw;
}

/** Prefix an absolute app path with the UI base (e.g. `/api/...` → `/trevor/fleet/api/...`). */
export function withBase(path: string): string {
  if (!path.startsWith("/")) return path;
  const base = uiBase();
  return base ? `${base}${path}` : path;
}

export type HealthResponse = {
  status: string;
  service: string;
  time: string;
};

export type VersionResponse = {
  version: string;
  commit: string;
  builtAt: string;
};

export type ClientProblem = {
  id?: string;
  name: string;
  kind?: string;
  message?: string;
};

export type CapabilitiesResponse = {
  engine: string;
  connected: boolean;
  address?: string;
  clientVersion?: string;
  apiVersion?: string;
  listenPort?: number;
  capabilities?: string[];
  message?: string;
  lastSync?: string;
  geoip?: boolean;
  clientsConnected?: number;
  clientsTotal?: number;
  problems?: ClientProblem[];
};

export type Torrent = {
  /** Unique list row id (hash@clientId). Falls back to hash when absent. */
  id?: string;
  hash: string;
  name: string;
  sizeBytes: number;
  bytesDone: number;
  leftBytes?: number;
  complete: boolean;
  active: boolean;
  open: boolean;
  hashing: boolean;
  downRate: number;
  upRate: number;
  downTotal?: number;
  upTotal?: number;
  ratio: number;
  priority?: number;
  directory: string;
  label?: string;
  state: string;
  progress: number;
  etaSeconds?: number;
  uploadEtaSeconds?: number;
  startedAt?: number;
  finishedAt?: number;
  seedingSeconds?: number;
  seedsComplete?: number;
  peersConnected?: number;
  clientId?: string;
  clientKind?: string;
  clientName?: string;
};

/** Stable row key for selection / virtual table (never collapses multi-client rows). */
export function torrentRowId(
  t: Pick<Torrent, "id" | "hash" | "clientId" | "clientKind">,
): string {
  if (t.id) return t.id;
  if (t.clientId) return `${t.hash}@${t.clientId}`;
  if (t.clientKind) return `${t.hash}@${t.clientKind}`;
  return t.hash;
}

export type TorrentsResponse = {
  revision: number;
  torrents: Torrent[];
  engine: CapabilitiesResponse;
};

export type ActionResult = {
  hash: string;
  ok: boolean;
  message?: string;
};

function csrfToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)fleet_csrf=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : "";
}

function apiHeaders(jsonBody: boolean): HeadersInit {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (jsonBody) headers["Content-Type"] = "application/json";
  const csrf = csrfToken();
  if (csrf) headers["X-CSRF-Token"] = csrf;
  return headers;
}

async function getJSON<T>(path: string): Promise<T> {
  const url = withBase(path);
  const res = await fetch(url, {
    credentials: "same-origin",
    headers: apiHeaders(false),
  });
  if (!res.ok) {
    const data = (await res.json().catch(() => ({}))) as { code?: string; error?: string };
    const err = new Error(data.error ?? `${url} → HTTP ${res.status}`) as Error & {
      status?: number;
      code?: string;
    };
    err.status = res.status;
    err.code = data.code;
    throw err;
  }
  return (await res.json()) as T;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const url = withBase(path);
  const res = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: apiHeaders(true),
    body: JSON.stringify(body),
  });
  const data = (await res.json().catch(() => ({}))) as T & { error?: string; code?: string };
  if (!res.ok) {
    const err = new Error(data.error ?? `${url} → HTTP ${res.status}`) as Error & {
      status?: number;
      code?: string;
    };
    err.status = res.status;
    err.code = data.code;
    throw err;
  }
  return data;
}

async function putJSON<T>(path: string, body: unknown): Promise<T> {
  const url = withBase(path);
  const res = await fetch(url, {
    method: "PUT",
    credentials: "same-origin",
    headers: apiHeaders(true),
    body: JSON.stringify(body),
  });
  const data = (await res.json().catch(() => ({}))) as T & { error?: string; code?: string };
  if (!res.ok) {
    const err = new Error(data.error ?? `${url} → HTTP ${res.status}`) as Error & {
      status?: number;
      code?: string;
    };
    err.status = res.status;
    err.code = data.code;
    throw err;
  }
  return data;
}

async function deleteJSON<T>(path: string): Promise<T> {
  const url = withBase(path);
  const res = await fetch(url, {
    method: "DELETE",
    credentials: "same-origin",
    headers: apiHeaders(false),
  });
  const data = (await res.json().catch(() => ({}))) as T & { error?: string; code?: string };
  if (!res.ok) {
    const err = new Error(data.error ?? `${url} → HTTP ${res.status}`) as Error & {
      status?: number;
      code?: string;
    };
    err.status = res.status;
    err.code = data.code;
    throw err;
  }
  return data;
}

export type AuthStatus = { enabled: boolean; needsSetup: boolean };
export type AuthUser = { id: number; username: string; isAdmin: boolean };

export function fetchAuthStatus() {
  return getJSON<AuthStatus>("/api/v1/auth/status");
}

export function fetchAuthMe() {
  return getJSON<{ authenticated: boolean; user?: AuthUser; csrfToken?: string }>("/api/v1/auth/me");
}

export function authSetup(username: string, password: string) {
  return postJSON<{ ok: boolean; user: AuthUser }>("/api/v1/auth/setup", { username, password });
}

export function authLogin(username: string, password: string) {
  return postJSON<{ ok: boolean; user: AuthUser }>("/api/v1/auth/login", { username, password });
}

export function authLogout() {
  return postJSON<{ ok: boolean }>("/api/v1/auth/logout", {});
}

export type UpdateStatus = {
  currentVersion: string;
  currentCommit: string;
  channel: string;
  autoUpdate: boolean;
  lastBackup?: string;
  installPrefix?: string;
  githubRepo?: string;
  pending?: {
    version: string;
    url: string;
    sha256: string;
    changelog?: string;
    downloadedAt: string;
    path: string;
  };
};

export type AvailableUpdate = {
  version: string;
  name?: string;
  prerelease: boolean;
  publishedAt?: string;
  url?: string;
  sha256?: string;
  assetName?: string;
};

export type InstalledRelease = {
  version: string;
  path: string;
  current: boolean;
  hasBinary: boolean;
};

export type UpdatePlan = {
  backupPath: string;
  pendingPath?: string;
  pendingVersion: string;
  installedPath?: string;
  helperCommand?: string;
  applyRequestPath?: string;
  copied?: boolean;
  message: string;
};

export function fetchUpdates() {
  return getJSON<UpdateStatus>("/api/v1/updates");
}

export function setUpdateChannel(channel: string) {
  return putJSON<{ ok: boolean; channel: string }>("/api/v1/updates/channel", { channel });
}

export function fetchAvailableUpdates() {
  return getJSON<{ releases: AvailableUpdate[] }>("/api/v1/updates/available");
}

export function fetchInstalledReleases() {
  return getJSON<{ releases: InstalledRelease[] }>("/api/v1/updates/installed");
}

export function prepareUpdate(manifest: {
  version: string;
  url?: string;
  sha256?: string;
  changelog?: string;
}) {
  return postJSON<{ ok: boolean; pending: UpdateStatus["pending"] }>("/api/v1/updates/prepare", manifest);
}

export function applyUpdate(opts?: { prefix?: string; healthUrl?: string }) {
  return postJSON<{ ok: boolean; plan: UpdatePlan }>("/api/v1/updates/apply", opts ?? {});
}

export function switchInstalledRelease(version: string, opts?: { prefix?: string; healthUrl?: string }) {
  return postJSON<{ ok: boolean; plan: UpdatePlan }>("/api/v1/updates/switch", {
    version,
    ...(opts ?? {}),
  });
}

export function backupNow() {
  return postJSON<{ ok: boolean; path: string }>("/api/v1/updates/backup", {});
}

export function fetchHealth(): Promise<HealthResponse> {
  return getJSON<HealthResponse>("/api/v1/health");
}

export function fetchVersion(): Promise<VersionResponse> {
  return getJSON<VersionResponse>("/api/v1/version");
}

export function fetchCapabilities(): Promise<CapabilitiesResponse> {
  return getJSON<CapabilitiesResponse>("/api/v1/engine/capabilities");
}

export function fetchTorrents(): Promise<TorrentsResponse> {
  return getJSON<TorrentsResponse>("/api/v1/torrents");
}

export type FeedMode = {
  mode: string;
  title: string;
  path: string;
  url: string;
};

export type FeedsSettings = {
  publicHost: string;
  httpPort: number;
  httpsPort: number;
  preferHTTPS: boolean;
  pathPrefix: string;
};

export function fetchFeeds(): Promise<{ modes: FeedMode[] }> {
  return getJSON<{ modes: FeedMode[] }>("/api/v1/feeds");
}

export function fetchFeedsSettings(): Promise<FeedsSettings> {
  return getJSON<FeedsSettings>("/api/v1/settings/feeds");
}

export function saveFeedsSettings(settings: FeedsSettings): Promise<FeedsSettings> {
  return putJSON<FeedsSettings>("/api/v1/settings/feeds", settings);
}

export function postTorrentActions(action: string, hashes: string[]) {
  return postJSON<{ action: string; results: ActionResult[] }>("/api/v1/torrents/actions", {
    action,
    hashes,
  });
}

/** Prefer hash@clientId row ids so multi-client actions hit the right gateway. */
export function postTorrentActionsForRows(action: string, rowIds: string[]) {
  return postTorrentActions(action, rowIds);
}

export function addMagnets(magnets: string[], start = true) {
  return postJSON<{ ok: boolean }>("/api/v1/torrents", { magnets, start });
}

export function addTorrentBase64(torrentBase64: string, start = true) {
  return postJSON<{ ok: boolean }>("/api/v1/torrents", { torrentBase64, start });
}

export function setTorrentLabel(hash: string, label: string, clientId?: string) {
  return postJSON<{ hash: string; label: string; clientId?: string }>(
    `/api/v1/torrents/${encodeURIComponent(hash)}/label`,
    { label, clientId: clientId || undefined },
  );
}

export function setTorrentName(hash: string, name: string) {
  return postJSON<{ hash: string; name: string }>(`/api/v1/torrents/${hash}/name`, { name });
}

export function setTorrentPriority(hash: string, priority: number) {
  return postJSON<{ hash: string; priority: number }>(`/api/v1/torrents/${hash}/priority`, { priority });
}

export function setFilePriorities(hash: string, indexes: number[], priority: number) {
  return postJSON<{ hash: string }>(`/api/v1/torrents/${hash}/files/priority`, { indexes, priority });
}

export type FileEntry = {
  index: number;
  path: string;
  sizeBytes: number;
  priority: number;
  completedChunks: number;
};

export type TrackerEntry = {
  index: number;
  id: string;
  url: string;
  enabled: boolean;
  open: boolean;
  seeders: number;
  leechers: number;
};

export type PeerEntry = {
  address: string;
  clientVersion: string;
  downRate: number;
  upRate: number;
  completedPercent: number;
  encrypted: boolean;
  incoming: boolean;
  country?: string;
  comment?: string;
};

function torrentDetailPath(hash: string, suffix: string, clientId?: string) {
  const base = `/api/v1/torrents/${encodeURIComponent(hash)}/${suffix}`;
  if (!clientId) return base;
  return `${base}?clientId=${encodeURIComponent(clientId)}`;
}

export function fetchTorrentFiles(hash: string, clientId?: string) {
  return getJSON<{ hash: string; files: FileEntry[] }>(torrentDetailPath(hash, "files", clientId));
}

export function fetchTorrentTrackers(hash: string, clientId?: string) {
  return getJSON<{ hash: string; trackers: TrackerEntry[] }>(
    torrentDetailPath(hash, "trackers", clientId),
  );
}

export function fetchTorrentPeers(hash: string, clientId?: string) {
  return getJSON<{ hash: string; peers: PeerEntry[] }>(torrentDetailPath(hash, "peers", clientId));
}

export function setPeerComment(address: string, comment: string) {
  return putJSON<{ ok: boolean; address: string; comment: string }>("/api/v1/peers/comment", {
    address,
    comment,
  });
}

export type ChunkMap = {
  hash: string;
  bitfield: string;
  chunkSize: number;
  sizeChunks: number;
  completed: number;
};

export function fetchTorrentChunks(hash: string) {
  return getJSON<ChunkMap>(`/api/v1/torrents/${hash}/chunks`);
}

/** Decode hex bitfield MSB-first into a boolean per chunk (length = sizeChunks). */
export function decodeBitfield(hexStr: string, sizeChunks: number): boolean[] {
  const out: boolean[] = [];
  const clean = (hexStr || "").trim().toLowerCase();
  if (!clean || sizeChunks <= 0) return out;
  for (let i = 0; i < clean.length; i += 2) {
    const byte = parseInt(clean.slice(i, i + 2), 16);
    if (Number.isNaN(byte)) break;
    for (let bit = 7; bit >= 0; bit--) {
      if (out.length >= sizeChunks) return out;
      out.push((byte & (1 << bit)) !== 0);
    }
  }
  while (out.length < sizeChunks) out.push(false);
  return out;
}

export function addTracker(hash: string, url: string) {
  return postJSON<{ ok: boolean }>(`/api/v1/torrents/${hash}/trackers`, { url });
}

export function setTrackerEnabled(hash: string, id: string, enabled: boolean) {
  return postJSON<{ ok: boolean }>(`/api/v1/torrents/${hash}/trackers/enable`, { id, enabled });
}

export function removeTracker(hash: string, id: string) {
  return postJSON<{ ok: boolean }>(`/api/v1/torrents/${hash}/trackers/remove`, { id });
}

export function dumpTorrent(torrentBase64: string) {
  return postJSON<{ ok: boolean; result: { parsed?: Record<string, unknown>; raw: string } }>(
    "/api/v1/tools/dumptorrent",
    { torrentBase64 },
  );
}

export type CreateTorrentRequest = {
  paths: string[];
  announce: string[];
  pieceLength?: number;
  private?: boolean;
  name?: string;
  comment?: string;
  outputName?: string;
};

export type Job = {
  id: string;
  kind: string;
  status: string;
  progress: number;
  message?: string;
  error?: string;
  result?: {
    downloadPath?: string;
    torrentBase64?: string;
    outputPath?: string;
  };
};

export function createTorrent(body: CreateTorrentRequest) {
  return postJSON<{ ok: boolean; jobId: string; job: Job }>("/api/v1/tools/mktorrent", body);
}

export function fetchJob(id: string) {
  return getJSON<Job>(`/api/v1/jobs/${encodeURIComponent(id)}`);
}

export function jobDownloadURL(id: string) {
  return `/api/v1/jobs/${encodeURIComponent(id)}/download`;
}

/** Authenticated same-origin download of the session/tied .torrent (source plugin parity). */
export function torrentSourceURL(hash: string) {
  return `/api/v1/torrents/${encodeURIComponent(hash)}/source`;
}

/** Authenticated same-origin download of a torrent data file (ruTorrent data plugin). */
export function torrentFileContentURL(hash: string, index: number) {
  return `/api/v1/torrents/${encodeURIComponent(hash)}/files/${index}/content`;
}

export function fetchExtensions(refresh = false) {
  const q = refresh ? "?refresh=1" : "";
  return getJSON<{ extensions: ExtensionRecord[] }>(`/api/v1/extensions${q}`);
}

export function activateExtension(id: string) {
  return postJSON<ExtensionRecord>(`/api/v1/extensions/${encodeURIComponent(id)}/activate`, {});
}

export function deactivateExtension(id: string) {
  return postJSON<ExtensionRecord>(`/api/v1/extensions/${encodeURIComponent(id)}/deactivate`, {});
}

export function reloadExtensionWorkers() {
  return postJSON<{
    ok: boolean;
    reloaded?: string[];
    failed?: Record<string, string>;
    count?: number;
  }>("/api/v1/extensions/reload-workers", {});
}

export function consentExtension(id: string) {
  return postJSON<{ ok: boolean; extension?: ExtensionRecord }>(
    `/api/v1/extensions/${encodeURIComponent(id)}/consent`,
    { accepted: true },
  );
}

export function extensionRPC(id: string, method: string, params?: unknown) {
  return postJSON<{ ok: boolean; result: unknown }>(
    `/api/v1/extensions/${encodeURIComponent(id)}/rpc`,
    { method, params: params ?? {} },
  );
}

export type PortWatchStatus = {
  reachable: boolean;
  known: boolean;
  lastOKAt?: string;
  lastError?: string;
  lastPort?: number;
  checkHost?: string;
  downSince?: string;
};

export type PortWatchScanResult = {
  reachable?: boolean;
  port?: number;
  host?: string;
  connected?: boolean;
  skipped?: boolean;
  downSince?: string;
  entries?: { kind?: string; message?: string; error?: string }[];
};

export function fetchPortWatchStatus() {
  return extensionRPC("org.fleet.portwatch", "ext.portwatch.getStatus", {}).then(
    (r) => r.result as PortWatchStatus,
  );
}

export function scanPortWatch() {
  return extensionRPC("org.fleet.portwatch", "ext.portwatch.scan", {}).then(
    (r) => r.result as PortWatchScanResult,
  );
}

export type FSEntry = {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  modTime: number;
  mode?: string;
};

export function fetchFSRoots() {
  return getJSON<{ roots: string[] }>("/api/v1/fs/roots");
}

export function fetchFSList(path = "") {
  const q = path ? `?path=${encodeURIComponent(path)}` : "";
  return getJSON<{ cwd: string; entries: FSEntry[] }>(`/api/v1/fs/list${q}`);
}

export function fetchFSText(path: string, encoding?: string) {
  const enc = encoding ? `&encoding=${encodeURIComponent(encoding)}` : "";
  return getJSON<{ path: string; text: string; encoding?: string }>(
    `/api/v1/fs/text?path=${encodeURIComponent(path)}${enc}`,
  );
}

export function fsMkdir(path: string) {
  return postJSON<{ ok: boolean; path: string }>("/api/v1/fs/mkdir", { path });
}

export function fsMove(from: string, to: string) {
  return postJSON<{ ok: boolean; from: string; to: string }>("/api/v1/fs/move", { from, to });
}

export function fsCopy(from: string, to: string) {
  return postJSON<{ ok: boolean; from: string; to: string }>("/api/v1/fs/copy", { from, to });
}

export function fsRemove(path: string, recursive = true) {
  return postJSON<{ ok: boolean; path: string }>("/api/v1/fs/remove", { path, recursive });
}

export function fsContentURL(path: string) {
  return withBase(`/api/v1/fs/content?path=${encodeURIComponent(path)}`);
}

export type ShareLink = {
  token: string;
  path: string;
  name: string;
  createdAt: number;
  expiresAt?: number;
  maxDownloads?: number;
  downloads: number;
};

export function fetchShares() {
  return getJSON<{ shares: ShareLink[] }>("/api/v1/shares");
}

export function createShare(body: { path: string; ttlSec?: number; maxDownloads?: number }) {
  return postJSON<{ share: ShareLink; url: string }>("/api/v1/shares", body);
}

export function deleteShare(token: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/shares/${encodeURIComponent(token)}`);
}

export type BucketRecord = {
  id: string;
  name: string;
  path: string;
  enabled: boolean;
  official?: boolean;
  updatedAt: string;
};

export type CatalogEntry = {
  bucketId: string;
  id: string;
  name: string;
  description?: string;
  version: string;
  api: string;
  origin: string;
  path?: string;
  repository?: string;
  packageUrl?: string;
  installable?: boolean;
};

export function fetchBuckets() {
  return getJSON<{ buckets: BucketRecord[] }>("/api/v1/buckets");
}

export function addBucket(path: string) {
  return postJSON<BucketRecord>("/api/v1/buckets", { path });
}

export function setBucketEnabled(id: string, enabled: boolean) {
  return postJSON<BucketRecord>(`/api/v1/buckets/${encodeURIComponent(id)}/enable`, { enabled });
}

export function fetchBucketCatalog() {
  return getJSON<{ extensions: CatalogEntry[] }>("/api/v1/buckets/catalog");
}

export type ThemeCatalogEntry = {
  bucketId: string;
  id: string;
  name: string;
  description?: string;
  version: string;
  origin: string;
  path?: string;
  packageUrl?: string;
  installable?: boolean;
};

export type InstalledThemePack = {
  id: string;
  packageId?: string;
  label: string;
  colorScheme?: string;
  version?: string;
  cssPath?: string;
};

export function fetchThemesCatalog() {
  return getJSON<{ themes: ThemeCatalogEntry[] }>("/api/v1/buckets/themes-catalog");
}

export function fetchInstalledThemePacks() {
  return getJSON<{ themes: InstalledThemePack[] }>("/api/v1/themes");
}

export function installThemeFromBucket(bucketId: string, themeId: string) {
  return postJSON<{ ok: boolean; id: string; packageId?: string; label?: string }>(
    "/api/v1/themes/install",
    {
      source: "bucket",
      bucketId,
      themeId,
    },
  );
}

export function uninstallThemePack(id: string) {
  return postJSON<{ ok: boolean }>("/api/v1/themes/uninstall", { id });
}

export function installFromBucket(bucketId: string, extensionId: string) {
  return postJSON<{ ok: boolean }>("/api/v1/extensions/install", {
    source: "bucket",
    bucketId,
    extensionId,
  });
}

export function installFromArchive(path: string, sha256: string) {
  return postJSON<{ ok: boolean }>("/api/v1/extensions/install", {
    source: "archive",
    path,
    sha256,
  });
}

export function installFromURL(url: string, sha256: string) {
  return postJSON<{ ok: boolean }>("/api/v1/extensions/install", {
    source: "url",
    url,
    sha256,
  });
}

export function uninstallExtension(id: string) {
  return deleteJSON<{ ok: boolean; id: string }>(
    `/api/v1/extensions/${encodeURIComponent(id)}/install`,
  );
}

export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

export function formatRate(n: number): string {
  return `${formatBytes(n)}/s`;
}

export function formatETA(seconds?: number): string {
  if (seconds == null || seconds <= 0 || !Number.isFinite(seconds)) return "—";
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const r = s % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${r}s`;
  return `${r}s`;
}

/** Seeding duration from finishedAt epoch (seedingtime parity). */
export function seedingSecondsFrom(finishedAt?: number, nowSec = Date.now() / 1000): number {
  if (!finishedAt || finishedAt <= 0) return 0;
  const n = Math.floor(nowSec - finishedAt);
  return n > 0 ? n : 0;
}

export type SystemDisk = {
  path: string;
  total: number;
  free: number;
  available: number;
  used: number;
  usedPct: number;
};

export type SystemLoad = {
  load1: number;
  load5: number;
  load15: number;
  procs?: string;
};

export function fetchSystemStatus(path?: string) {
  const q = path ? `?path=${encodeURIComponent(path)}` : "";
  return getJSON<{ disks: SystemDisk[]; load?: SystemLoad; loadError?: string }>(
    `/api/v1/system/status${q}`,
  );
}

export type GatewayKind = "rtorrent" | "deluge" | "transmission" | "qbittorrent";

export type GatewayCapabilities = {
  list?: boolean;
  add?: boolean;
  label?: boolean;
  startStop?: boolean;
  erase?: boolean;
  eraseData?: boolean;
  reannounce?: boolean;
  files?: boolean;
  trackers?: boolean;
  peers?: boolean;
  chunks?: boolean;
  torrentRates?: boolean;
};

export type GatewayClient = {
  id: string;
  kind: GatewayKind;
  name: string;
  url: string;
  username: string;
  hasPassword: boolean;
  enabled: boolean;
  isDefault?: boolean;
  journalUnit?: string;
  logPath?: string;
  createdAt: string;
  updatedAt: string;
  capabilities?: GatewayCapabilities;
};

export type GatewayProbeResult = {
  ok: boolean;
  kind: GatewayKind | string;
  version?: string;
  message?: string;
};

export type GatewayWritePayload = {
  id?: string;
  kind: GatewayKind;
  name: string;
  url: string;
  username?: string;
  password?: string;
  enabled?: boolean;
  journalUnit?: string;
  logPath?: string;
};

export type LogSource = {
  id: string;
  kind: string;
  name: string;
  ok: boolean;
  detail?: string;
};

export type LogEntry = {
  ts: string;
  level: string;
  message: string;
  attrs?: Record<string, string>;
  sourceId: string;
  sourceKind: string;
  sourceName: string;
  stream?: string;
};

export type LogsResponse = {
  sources: LogSource[];
  entries: LogEntry[];
};

export function fetchLogs(opts?: { source?: string; level?: string; limit?: number }) {
  const q = new URLSearchParams();
  if (opts?.source) q.set("source", opts.source);
  if (opts?.level) q.set("level", opts.level);
  if (opts?.limit) q.set("limit", String(opts.limit));
  const qs = q.toString();
  return getJSON<LogsResponse>(`/api/v1/logs${qs ? `?${qs}` : ""}`);
}

export function fetchGateways() {
  return getJSON<{ clients: GatewayClient[] }>("/api/v1/gateways");
}

export function saveGateway(payload: GatewayWritePayload) {
  if (payload.id) {
    return putJSON<{ client: GatewayClient }>(`/api/v1/gateways/${encodeURIComponent(payload.id)}`, payload);
  }
  return postJSON<{ client: GatewayClient }>("/api/v1/gateways", payload);
}

export function deleteGateway(id: string) {
  return deleteJSON<{ ok: boolean }>(`/api/v1/gateways/${encodeURIComponent(id)}`);
}

export function setDefaultGateway(id: string) {
  return postJSON<{ ok: boolean; client?: GatewayClient }>(
    `/api/v1/gateways/${encodeURIComponent(id)}/default`,
    {},
  );
}

export function clearDefaultGateway() {
  return deleteJSON<{ ok: boolean }>("/api/v1/gateways/default");
}

export function probeGateway(id: string) {
  return postJSON<GatewayProbeResult>(`/api/v1/gateways/${encodeURIComponent(id)}/probe`, {});
}

export function probeGatewayDraft(payload: {
  kind: GatewayKind;
  url: string;
  username?: string;
  password?: string;
}) {
  return postJSON<GatewayProbeResult>("/api/v1/gateways/probe", payload);
}
