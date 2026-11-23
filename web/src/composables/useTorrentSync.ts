import { onMounted, onUnmounted, ref } from "vue";
import { fetchTorrents, torrentRowId, type CapabilitiesResponse, type Torrent } from "@/api/client";

type WSEvent = {
  type: "snapshot" | "add" | "update" | "remove" | "engine";
  revision?: number;
  torrent?: Torrent;
  torrents?: Torrent[];
  hash?: string;
  engine?: CapabilitiesResponse;
};

/** Shared across AppShell / Torrents / Files so status bar stays global. */
const torrents = ref<Torrent[]>([]);
const engine = ref<CapabilitiesResponse | null>(null);
const error = ref<string | null>(null);
const revision = ref(0);

let ws: WebSocket | null = null;
let pollTimer: ReturnType<typeof setInterval> | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let subscribers = 0;
let started = false;
let stopped = true;

function applyEvent(ev: WSEvent) {
  switch (ev.type) {
    case "snapshot":
      torrents.value = ev.torrents ?? [];
      if (ev.engine) engine.value = ev.engine;
      if (ev.revision != null) revision.value = ev.revision;
      break;
    case "add":
    case "update":
      if (ev.torrent) {
        const key = torrentRowId(ev.torrent);
        const idx = torrents.value.findIndex((t) => torrentRowId(t) === key);
        if (idx >= 0) {
          const next = torrents.value.slice();
          next[idx] = ev.torrent;
          torrents.value = next;
        } else {
          torrents.value = [...torrents.value, ev.torrent];
        }
      }
      if (ev.revision != null) revision.value = ev.revision;
      break;
    case "remove":
      if (ev.hash) {
        const key = ev.hash;
        torrents.value = torrents.value.filter((t) => torrentRowId(t) !== key);
      }
      if (ev.revision != null) revision.value = ev.revision;
      break;
    case "engine":
      if (ev.engine) engine.value = ev.engine;
      break;
    default:
      break;
  }
}

async function httpRefresh() {
  try {
    const data = await fetchTorrents();
    torrents.value = data.torrents;
    engine.value = data.engine;
    revision.value = data.revision;
    const clientsOK = (data.engine?.clientsConnected ?? (data.engine?.connected ? 1 : 0)) > 0;
    error.value = clientsOK ? null : (data.engine?.message ?? "Engine unavailable");
  } catch (err) {
    error.value = err instanceof Error ? err.message : "Failed to load torrents";
  }
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

function startPolling(pollFallbackMs: number) {
  if (pollTimer || stopped) return;
  pollTimer = setInterval(() => {
    void httpRefresh();
  }, pollFallbackMs);
}

function connectWS(pollFallbackMs: number) {
  if (stopped) return;
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const base = (import.meta.env.BASE_URL || "/").replace(/\/$/, "");
  ws = new WebSocket(`${proto}://${location.host}${base}/api/v1/events/ws`);
  ws.addEventListener("open", () => {
    error.value = null;
    stopPolling();
  });
  ws.addEventListener("message", (msg) => {
    try {
      applyEvent(JSON.parse(String(msg.data)) as WSEvent);
    } catch {
      // ignore malformed frames
    }
  });
  ws.addEventListener("close", () => {
    ws = null;
    startPolling(pollFallbackMs);
    if (!stopped) {
      reconnectTimer = setTimeout(() => connectWS(pollFallbackMs), 2000);
    }
  });
  ws.addEventListener("error", () => {
    ws?.close();
  });
}

async function start(pollFallbackMs: number) {
  if (started) return;
  started = true;
  stopped = false;
  await httpRefresh();
  connectWS(pollFallbackMs);
}

function stop() {
  stopped = true;
  started = false;
  stopPolling();
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  ws?.close();
  ws = null;
}

/**
 * Prefer WebSocket deltas; fall back to HTTP polling if the socket drops.
 * Shared singleton — status bar stays correct off the torrents page.
 */
export function useTorrentSync(pollFallbackMs = 5000) {
  onMounted(() => {
    subscribers += 1;
    void start(pollFallbackMs);
  });

  onUnmounted(() => {
    subscribers = Math.max(0, subscribers - 1);
    if (subscribers === 0) stop();
  });

  return {
    torrents,
    engine,
    error,
    revision,
    refresh: httpRefresh,
  };
}
