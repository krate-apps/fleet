<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import AppShell from "@/components/AppShell.vue";
import ClientLogo from "@/components/ClientLogo.vue";
import Modal from "@/components/Modal.vue";
import PathPicker from "@/components/PathPicker.vue";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import TorrentContextMenu from "@/components/TorrentContextMenu.vue";
import VirtualTable from "@/components/VirtualTable.vue";
import { useExtensionUI } from "@/composables/useExtensionUI";
import { useResizableColumns } from "@/composables/useResizableColumns";
import { useShellModals } from "@/composables/useShellModals";
import { useTorrentSync } from "@/composables/useTorrentSync";
import { useUiDensity } from "@/composables/useUiDensity";
import { DOWNLOAD_QUEUE_PROVIDERS } from "@/extensions/downloadQueues";
import { useI18n } from "@/i18n/useI18n";
import { dominantClient } from "@/lib/clientFamily";
import {
  addMagnets,
  addTorrentBase64,
  addTracker,
  createTorrent,
  decodeBitfield,
  dumpTorrent,
  fetchJob,
  fetchTorrentChunks,
  fetchTorrentFiles,
  fetchTorrentPeers,
  fetchTorrentTrackers,
  formatBytes,
  formatETA,
  formatRate,
  seedingSecondsFrom,
  jobDownloadURL,
  postTorrentActions,
  removeTracker,
  setFilePriorities,
  setPeerComment,
  setTorrentLabel,
  setTorrentName,
  setTorrentPriority,
  extensionRPC,
  setTrackerEnabled,
  torrentFileContentURL,
  torrentSourceURL,
  torrentRowId,
  type ChunkMap,
  type FileEntry,
  type PeerEntry,
  type Torrent,
  type TrackerEntry,
} from "@/api/client";

type FilterId =
  | "all"
  | "downloading"
  | "seeding"
  | "active"
  | "paused"
  | "stopped"
  | "complete"
  | "incomplete"
  | `label:${string}`;
type DetailTab = "general" | "files" | "trackers" | "peers" | "chunks";
type SortKey =
  | "client"
  | "name"
  | "state"
  | "progress"
  | "downRate"
  | "upRate"
  | "sizeBytes"
  | "ratio"
  | "etaSeconds"
  | "seedsComplete";

type RatioColorSettings = {
  enabled: boolean;
  lowMax: number;
  midMax: number;
};

type RatioBand = "low" | "mid" | "high";

const PREFS_KEY = "fleet.torrentPrefs.v1";
const DETAILS_HEIGHT_KEY = "fleet.detailsHeight.v1";
const DETAILS_HEIGHT_MIN = 160;
const DETAILS_HEIGHT_DEFAULT = 280;

type TorrentPrefs = {
  filter: FilterId;
  sortKey: SortKey;
  sortAsc: boolean;
  query: string;
};

function loadPrefs(): Partial<TorrentPrefs> {
  if (typeof localStorage === "undefined") return {};
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    if (!raw) return {};
    return JSON.parse(raw) as Partial<TorrentPrefs>;
  } catch {
    return {};
  }
}

function clampDetailsHeight(h: number): number {
  const max =
    typeof window !== "undefined"
      ? Math.max(DETAILS_HEIGHT_MIN, Math.floor(window.innerHeight * 0.7))
      : 560;
  return Math.round(Math.min(max, Math.max(DETAILS_HEIGHT_MIN, h)));
}

function loadDetailsHeight(): number {
  if (typeof localStorage === "undefined") return DETAILS_HEIGHT_DEFAULT;
  try {
    const raw = localStorage.getItem(DETAILS_HEIGHT_KEY);
    if (!raw) return DETAILS_HEIGHT_DEFAULT;
    const n = Number(raw);
    return Number.isFinite(n) ? clampDetailsHeight(n) : DETAILS_HEIGHT_DEFAULT;
  } catch {
    return DETAILS_HEIGHT_DEFAULT;
  }
}

const savedPrefs = loadPrefs();
const detailsHeight = ref(loadDetailsHeight());
let detailsResize: { startY: number; startH: number } | null = null;

function onDetailsResizeMove(ev: MouseEvent) {
  if (!detailsResize) return;
  detailsHeight.value = clampDetailsHeight(
    detailsResize.startH + (detailsResize.startY - ev.clientY),
  );
}

function onDetailsResizeUp() {
  detailsResize = null;
  window.removeEventListener("mousemove", onDetailsResizeMove);
  window.removeEventListener("mouseup", onDetailsResizeUp);
  try {
    localStorage.setItem(DETAILS_HEIGHT_KEY, String(detailsHeight.value));
  } catch {
    // ignore
  }
}

function startDetailsResize(ev: MouseEvent) {
  ev.preventDefault();
  ev.stopPropagation();
  detailsResize = { startY: ev.clientY, startH: detailsHeight.value };
  window.addEventListener("mousemove", onDetailsResizeMove);
  window.addEventListener("mouseup", onDetailsResizeUp);
}

onBeforeUnmount(() => {
  window.removeEventListener("mousemove", onDetailsResizeMove);
  window.removeEventListener("mouseup", onDetailsResizeUp);
});

const route = useRoute();
const { t } = useI18n();
const { anyModalOpen } = useShellModals();
const { density, rowHeightPx } = useUiDensity();
const { torrents, engine, error: syncError, refresh } = useTorrentSync();
const { active: activeExtensions, selectionActions } = useExtensionUI();
const downloadersConnected = computed(
  () =>
    DOWNLOAD_QUEUE_PROVIDERS.filter((p) =>
      activeExtensions.value.some((e) => e.id === p.extensionId),
    ).length,
);
const clientLogosEnabled = computed(() =>
  activeExtensions.value.some((e) => e.id === "org.fleet.clientlogos"),
);
/** Client column (host + peer logos) only while org.fleet.clientlogos is active. */
const showClientCol = computed(() => clientLogosEnabled.value);
const crossSeedEnabled = computed(() =>
  activeExtensions.value.some((e) => e.id === "org.fleet.crossseed") &&
  selectionActions.value.some((a) => a.extensionId === "org.fleet.crossseed" && a.id === "cross-seed-now"),
);
const nfoEnabled = computed(() =>
  activeExtensions.value.some((e) => e.id === "org.fleet.nfo") &&
  selectionActions.value.some((a) => a.extensionId === "org.fleet.nfo" && a.id === "view-nfo"),
);
const titleSearchEnabled = computed(() =>
  activeExtensions.value.some((e) => e.id === "org.fleet.titlesearch") &&
  selectionActions.value.some((a) => a.extensionId === "org.fleet.titlesearch" && a.id === "open-engines"),
);
const bandwidthEnabled = computed(() =>
  activeExtensions.value.some((e) => e.id === "org.fleet.bandwidth") &&
  selectionActions.value.some((a) => a.extensionId === "org.fleet.bandwidth" && a.id === "set-limits"),
);
const showBandwidthModal = ref(false);
const bandwidthUpKiB = ref(0);
const bandwidthDownKiB = ref(0);
const { columns: tableCols, startResize } = useResizableColumns([
  { id: "check", label: "", width: 36, minWidth: 36 },
  { id: "name", label: "Name", width: 320, minWidth: 140, sortable: true },
  { id: "state", label: "Status", width: 110, minWidth: 72, sortable: true },
  { id: "progress", label: "Done", width: 120, minWidth: 80, sortable: true },
  { id: "downRate", label: "Down", width: 96, minWidth: 72, sortable: true },
  { id: "upRate", label: "Up", width: 96, minWidth: 72, sortable: true },
  { id: "etaSeconds", label: "ETA", width: 80, minWidth: 56, sortable: true },
  { id: "sizeBytes", label: "Size", width: 96, minWidth: 72, sortable: true },
  { id: "ratio", label: "Ratio", width: 72, minWidth: 56, sortable: true },
  { id: "seedsComplete", label: "Seeds", width: 72, minWidth: 56, sortable: true },
]);
/** PD-UX-02: phone hides secondary columns by default. */
const PHONE_HIDDEN_COLS = new Set(["upRate", "etaSeconds", "sizeBytes", "ratio", "seedsComplete"]);
const phoneNarrow = ref(false);
const detailsSheet = computed(() => phoneNarrow.value);
const showClientColEffective = computed(() => showClientCol.value && !phoneNarrow.value);
const visibleTableCols = computed(() => tableCols.filter((c) => !c.hidden));
const tableWidth = computed(() =>
  visibleTableCols.value.reduce((sum, col) => sum + col.width, 0),
);
const tableWidthStyle = computed(() => ({
  "--fleet-table-w": `${tableWidth.value}px`,
}));
const dataColOffset = computed(() => (showClientColEffective.value ? 2 : 1));
const query = ref(savedPrefs.query ?? "");
const filter = ref<FilterId>(savedPrefs.filter ?? "all");
const sortKey = ref<SortKey>(savedPrefs.sortKey ?? "name");
const sortAsc = ref(savedPrefs.sortAsc ?? true);
const ratioColors = ref<RatioColorSettings | null>(null);
const dragActive = ref(false);
/** Row ids currently selected (object map for reliable Vue reactivity). */
const selectedIds = ref<Record<string, true>>({});
const selectedCount = computed(() => Object.keys(selectedIds.value).length);
const focusHash = ref<string | null>(null);

function isRowSelected(id: string): boolean {
  return selectedIds.value[id] === true;
}

function selectOnly(id: string) {
  selectedIds.value = { [id]: true };
  focusHash.value = id;
}

function toggleRow(id: string) {
  const next = { ...selectedIds.value };
  if (next[id]) delete next[id];
  else next[id] = true;
  selectedIds.value = next;
  focusHash.value = id;
}

function clearSelection() {
  selectedIds.value = {};
}

function onRowClick(t: Torrent, ev: MouseEvent) {
  const id = torrentRowId(t);
  if (ev.ctrlKey || ev.metaKey) toggleRow(id);
  else selectOnly(id);
}

function onRowCheckboxChange(t: Torrent) {
  toggleRow(torrentRowId(t));
}
const magnet = ref("");
const torrentFileName = ref("");
const torrentBase64 = ref("");
const showAdd = ref(false);
const showCreate = ref(false);
const showMenu = ref(false);
const nfoModal = ref<{ path: string; text: string; encoding: string } | null>(null);
const notice = ref<string | null>(null);
const localError = ref<string | null>(null);
const busy = ref(false);
const createPaths = ref("");
const createExtraPaths = ref("");
const createAnnounce = ref("");
const createName = ref("");
const createComment = ref("");
const createPrivate = ref(true);
const createPieceLength = ref(0);
const createJobId = ref("");
const createDownload = ref("");
const detailTab = ref<DetailTab>("general");
const ctxMenu = ref<{ x: number; y: number } | null>(null);
const files = ref<FileEntry[]>([]);
const trackers = ref<TrackerEntry[]>([]);
const peers = ref<PeerEntry[]>([]);
const chunks = ref<ChunkMap | null>(null);
const chunkCells = computed(() => {
  if (!chunks.value) return [] as boolean[];
  const max = 4096;
  const n = Math.min(chunks.value.sizeChunks || 0, max);
  return decodeBitfield(chunks.value.bitfield, n);
});
/** Dominant peer client version string per torrent hash (for Client column logos). */
const clientByHash = ref<Record<string, string>>({});
watch(
  showClientCol,
  (on) => {
    const idx = tableCols.findIndex((c) => c.id === "client");
    if (on && idx < 0) {
      tableCols.splice(1, 0, {
        id: "client",
        label: "Client",
        width: 56,
        minWidth: 48,
        sortable: true,
        hidden: false,
      });
    } else if (!on && idx >= 0) {
      tableCols.splice(idx, 1);
      clientByHash.value = {};
    }
  },
  { immediate: true },
);

watch(clientLogosEnabled, (on) => {
  if (!on) clientByHash.value = {};
});

function clientSortValue(t: Torrent): string {
  return (t.clientName || t.clientKind || "").toLowerCase();
}

/** Action targets as hash@clientId (or bare hash) so gateway writes hit the right client. */
function selectedActionIds(): string[] {
  const ids = selectedIds.value;
  return torrents.value.filter((t) => ids[torrentRowId(t)]).map((t) => torrentRowId(t));
}

/** Bare infohashes when an API still expects hash-only (legacy download/NFO helpers). */
function selectedInfoHashes(): string[] {
  return [...new Set(selectedActionIds().map((id) => id.split("@")[0] || id))];
}
const detailError = ref<string | null>(null);
const labelDraft = ref("");
const nameDraft = ref("");
const trackerDraft = ref("");
const dumpPreview = ref<string>("");
const shell = ref<{
  expandSidebar: () => void;
  closeMobileNav?: () => void;
} | null>(null);

const error = computed(() => localError.value || syncError.value);
const engineConnected = computed(() => Boolean(engine.value?.connected));

const labels = computed(() => {
  const set = new Set<string>();
  for (const t of torrents.value) {
    const l = (t.label ?? "").trim();
    if (l) set.add(l);
  }
  return [...set].sort((a, b) => a.localeCompare(b));
});

const counts = computed(() => {
  const list = torrents.value;
  const base = {
    all: list.length,
    downloading: list.filter((t) => t.state === "downloading").length,
    seeding: list.filter((t) => t.state === "seeding").length,
    active: list.filter((t) => t.active).length,
    paused: list.filter((t) => t.state === "paused").length,
    stopped: list.filter((t) => t.state === "stopped" || t.state === "closed").length,
    complete: list.filter((t) => t.complete).length,
    incomplete: list.filter((t) => !t.complete).length,
  } as Record<string, number>;
  for (const l of labels.value) {
    base[`label:${l}`] = list.filter((t) => (t.label ?? "") === l).length;
  }
  return base;
});

const filtered = computed(() => {
  let list = torrents.value.slice();
  if (filter.value.startsWith("label:")) {
    const label = filter.value.slice("label:".length);
    list = list.filter((t) => (t.label ?? "") === label);
  } else {
    switch (filter.value) {
      case "downloading":
        list = list.filter((t) => t.state === "downloading");
        break;
      case "seeding":
        list = list.filter((t) => t.state === "seeding");
        break;
      case "active":
        list = list.filter((t) => t.active);
        break;
      case "paused":
        list = list.filter((t) => t.state === "paused");
        break;
      case "stopped":
        list = list.filter((t) => t.state === "stopped" || t.state === "closed");
        break;
      case "complete":
        list = list.filter((t) => t.complete);
        break;
      case "incomplete":
        list = list.filter((t) => !t.complete);
        break;
      default:
        break;
    }
  }
  const q = query.value.trim().toLowerCase();
  if (q) {
    list = list.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.hash.toLowerCase().includes(q) ||
        (t.label ?? "").toLowerCase().includes(q),
    );
  }
  const key = sortKey.value;
  const dir = sortAsc.value ? 1 : -1;
  list.sort((a, b) => {
    if (key === "client") {
      return clientSortValue(a).localeCompare(clientSortValue(b)) * dir;
    }
    const av = a[key] ?? 0;
    const bv = b[key] ?? 0;
    if (typeof av === "string" && typeof bv === "string") {
      return av.localeCompare(bv) * dir;
    }
    return ((av as number) - (bv as number)) * dir;
  });
  return list;
});

const totalDown = computed(() => torrents.value.reduce((s, t) => s + t.downRate, 0));
const totalUp = computed(() => torrents.value.reduce((s, t) => s + t.upRate, 0));
const focused = computed(() => torrents.value.find((t) => torrentRowId(t) === focusHash.value) ?? null);
const focusedHash = computed(() => focused.value?.hash ?? null);

const statusFilters: { id: FilterId; label: string }[] = [
  { id: "all", label: "All" },
  { id: "downloading", label: "Downloading" },
  { id: "seeding", label: "Seeding" },
  { id: "active", label: "Active" },
  { id: "paused", label: "Paused" },
  { id: "stopped", label: "Stopped" },
  { id: "complete", label: "Complete" },
  { id: "incomplete", label: "Incomplete" },
];

watch(focused, (t) => {
  labelDraft.value = t?.label ?? "";
  nameDraft.value = t?.name ?? "";
});

watch(
  activeExtensions,
  async (list) => {
    const has = list.some((e) => e.id === "org.fleet.ratiocolors");
    if (!has) {
      ratioColors.value = null;
      return;
    }
    try {
      const res = await extensionRPC("org.fleet.ratiocolors", "ext.ratiocolors.getSettings", {});
      ratioColors.value = (res.result as RatioColorSettings) ?? null;
    } catch {
      ratioColors.value = null;
    }
  },
  { immediate: true },
);

/** Band from extension thresholds; colors come from theme `--fleet-ratio-*`. */
function ratioBand(ratio: number): RatioBand | undefined {
  const cfg = ratioColors.value;
  if (!cfg?.enabled) return undefined;
  if (ratio < cfg.lowMax) return "low";
  if (ratio < cfg.midMax) return "mid";
  return "high";
}

watch(
  [filter, sortKey, sortAsc, query],
  () => {
    if (typeof localStorage === "undefined") return;
    const prefs: TorrentPrefs = {
      filter: filter.value,
      sortKey: sortKey.value,
      sortAsc: sortAsc.value,
      query: query.value,
    };
    localStorage.setItem(PREFS_KEY, JSON.stringify(prefs));
  },
  { deep: false },
);

function restoreSidebar() {
  shell.value?.expandSidebar();
  showMenu.value = false;
}

function onFiltersClick() {
  if (typeof window !== "undefined" && window.matchMedia("(max-width: 900px)").matches) {
    shell.value?.expandSidebar();
    return;
  }
  showMenu.value = true;
}

function applyCompactTableWidths(viewportCompact: boolean, phone: boolean) {
  const compact = viewportCompact || phone || density.value === "compact";
  const targets: Record<string, { normal: number; compact: number }> = {
    name: { normal: 320, compact: 160 },
    state: { normal: 110, compact: 84 },
    progress: { normal: 120, compact: 88 },
    downRate: { normal: 96, compact: 72 },
    upRate: { normal: 96, compact: 72 },
    etaSeconds: { normal: 80, compact: 56 },
    sizeBytes: { normal: 96, compact: 72 },
    ratio: { normal: 72, compact: 56 },
    seedsComplete: { normal: 72, compact: 52 },
    client: { normal: 56, compact: 48 },
  };
  for (const col of tableCols) {
    const t = targets[col.id];
    if (t) col.width = compact ? t.compact : t.normal;
    if (PHONE_HIDDEN_COLS.has(col.id)) {
      col.hidden = phone;
    } else {
      col.hidden = false;
    }
  }
  phoneNarrow.value = phone;
}

function closeDetailsSheet() {
  focusHash.value = null;
}

function isTypingTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  return el.isContentEditable;
}

function focusAt(index: number) {
  const list = filtered.value;
  if (list.length === 0) return;
  const i = Math.max(0, Math.min(list.length - 1, index));
  selectOnly(torrentRowId(list[i]));
}

function moveFocus(delta: number) {
  const list = filtered.value;
  if (list.length === 0) return;
  const cur = focusHash.value ? list.findIndex((t) => torrentRowId(t) === focusHash.value) : -1;
  focusAt(cur < 0 ? 0 : cur + delta);
}

async function ingestTorrentFile(file: File) {
  torrentFileName.value = file.name;
  const buf = await file.arrayBuffer();
  const bytes = new Uint8Array(buf);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  torrentBase64.value = btoa(binary);
  showAdd.value = true;
  notice.value = `Ready to add ${file.name}`;
}

async function onDrop(ev: DragEvent) {
  ev.preventDefault();
  dragActive.value = false;
  const dt = ev.dataTransfer;
  if (!dt) return;
  const file = [...(dt.files ?? [])].find((f) => f.name.toLowerCase().endsWith(".torrent"));
  if (file) {
    await ingestTorrentFile(file);
    return;
  }
  const text = dt.getData("text/plain")?.trim() ?? "";
  if (text.startsWith("magnet:")) {
    magnet.value = text;
    showAdd.value = true;
    notice.value = "Magnet ready to add";
  }
}

function onDragOver(ev: DragEvent) {
  ev.preventDefault();
  dragActive.value = true;
}

function onDragLeave(ev: DragEvent) {
  const cur = ev.currentTarget as HTMLElement | null;
  const related = ev.relatedTarget as Node | null;
  if (cur && related && cur.contains(related)) return;
  dragActive.value = false;
}

function focusTorrentSearch() {
  const el = document.querySelector<HTMLInputElement>(".fleet-search input");
  if (!el) return;
  el.focus();
  el.select();
}

function isSearchHotkey(ev: KeyboardEvent): boolean {
  return (
    (ev.ctrlKey || ev.metaKey) &&
    !ev.altKey &&
    !ev.shiftKey &&
    (ev.code === "KeyK" || ev.key.toLowerCase() === "k")
  );
}

function onGlobalKey(ev: KeyboardEvent) {
  // Ctrl/Cmd+K focuses search (AppShell handles the same in capture; keep as fallback).
  if (isSearchHotkey(ev) && !anyModalOpen()) {
    ev.preventDefault();
    ev.stopImmediatePropagation();
    focusTorrentSearch();
    return;
  }
  if (isTypingTarget(ev.target) || showAdd.value || showCreate.value || showMenu.value || anyModalOpen()) return;
  if (ev.key === "Escape" && detailsSheet.value && focusHash.value) {
    ev.preventDefault();
    closeDetailsSheet();
    return;
  }
  switch (ev.key) {
    case "ArrowDown":
    case "j":
      ev.preventDefault();
      moveFocus(1);
      break;
    case "ArrowUp":
    case "k":
      // Never treat Ctrl/Cmd+K as vim-style move-up (that opens the details panel).
      if (ev.ctrlKey || ev.metaKey || ev.altKey) break;
      ev.preventDefault();
      moveFocus(-1);
      break;
    case " ":
      if (focusHash.value) {
        ev.preventDefault();
        toggleRow(focusHash.value);
      }
      break;
    case "a":
      ev.preventDefault();
      showAdd.value = true;
      break;
    case "s":
      ev.preventDefault();
      void runAction("start");
      break;
    case "p":
      ev.preventDefault();
      void runAction("pause");
      break;
    case "Delete":
    case "Backspace":
      ev.preventDefault();
      void runAction("erase");
      break;
    case "/":
      ev.preventDefault();
      focusTorrentSearch();
      break;
    default:
      break;
  }
}

let tableMq: MediaQueryList | undefined;
let phoneMq: MediaQueryList | undefined;
function syncTableLayout() {
  const compact = tableMq?.matches ?? false;
  const phone = phoneMq?.matches ?? false;
  applyCompactTableWidths(compact, phone);
}
function onTableMqChange() {
  syncTableLayout();
}

watch(density, () => syncTableLayout());

function applyFilterFromRoute() {
  const raw = route.query.filter;
  if (typeof raw !== "string" || !raw.trim()) return;
  const value = raw.trim() as FilterId;
  const allowed =
    value === "all" ||
    value === "downloading" ||
    value === "seeding" ||
    value === "active" ||
    value === "paused" ||
    value === "stopped" ||
    value === "complete" ||
    value === "incomplete" ||
    value.startsWith("label:");
  if (allowed) filter.value = value;
}

onMounted(() => {
  applyFilterFromRoute();
  window.addEventListener("keydown", onGlobalKey);
  tableMq = window.matchMedia("(max-width: 900px)");
  phoneMq = window.matchMedia("(max-width: 560px)");
  syncTableLayout();
  tableMq.addEventListener("change", onTableMqChange);
  phoneMq.addEventListener("change", onTableMqChange);
});

watch(
  () => route.query.filter,
  () => applyFilterFromRoute(),
);
onBeforeUnmount(() => {
  window.removeEventListener("keydown", onGlobalKey);
  tableMq?.removeEventListener("change", onTableMqChange);
  phoneMq?.removeEventListener("change", onTableMqChange);
});

const ctxTargets = computed(() =>
  torrents.value.filter((t) => isRowSelected(torrentRowId(t))),
);

const ctxCanPause = computed(() =>
  ctxTargets.value.some((t) => t.active || t.state === "downloading" || t.state === "seeding"),
);

const ctxCanStop = computed(() =>
  ctxTargets.value.some((t) => t.open || t.active || t.state === "paused" || t.state === "downloading" || t.state === "seeding"),
);

/** Shared label of the selection, or null when torrents disagree. */
const ctxActiveLabel = computed(() => {
  const targets = ctxTargets.value;
  if (!targets.length) return "";
  const first = targets[0]?.label ?? "";
  return targets.every((t) => (t.label ?? "") === first) ? first : null;
});

function openTorrentContextMenu(ev: MouseEvent, rowId: string) {
  ev.preventDefault();
  ev.stopPropagation();
  if (!isRowSelected(rowId)) {
    selectOnly(rowId);
  } else {
    focusHash.value = rowId;
  }
  ctxMenu.value = { x: ev.clientX, y: ev.clientY };
}

function closeTorrentContextMenu() {
  ctxMenu.value = null;
}

async function ctxSetLabel(label: string) {
  const ids = selectedIds.value;
  const rows = torrents.value.filter((t) => ids[torrentRowId(t)]);
  if (!rows.length) return;
  busy.value = true;
  try {
    await Promise.all(rows.map((t) => setTorrentLabel(t.hash, label, t.clientId)));
    notice.value = t("details.label");
    if (focusedHash.value && rows.some((t) => t.hash === focusedHash.value)) {
      labelDraft.value = label;
    }
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Label failed";
  } finally {
    busy.value = false;
  }
}

async function ctxCrossSeed() {
  const hashes = selectedInfoHashes();
  if (!hashes.length) return;
  busy.value = true;
  try {
    const res = await extensionRPC("org.fleet.crossseed", "ext.crossseed.crossSeed", { hashes });
    const result = (res.result ?? {}) as { sent?: number; skipped?: number; failed?: number };
    notice.value = t("ctx.crossSeed.done", {
      sent: result.sent ?? 0,
      skipped: result.skipped ?? 0,
      failed: result.failed ?? 0,
    });
  } catch (err) {
    localError.value = err instanceof Error ? err.message : t("ctx.crossSeed.fail");
  } finally {
    busy.value = false;
  }
}

function isNfoPath(path: string): boolean {
  return path.toLowerCase().endsWith(".nfo");
}

function joinTorrentPath(directory: string, rel: string): string {
  const dir = directory.replace(/[/\\]+$/, "");
  const leaf = rel.replace(/^[/\\]+/, "");
  if (!dir) return leaf;
  if (!leaf) return dir;
  if (dir.endsWith(leaf) || dir.endsWith("/" + leaf) || dir.endsWith("\\" + leaf)) {
    return dir;
  }
  const sep = dir.includes("\\") && !dir.includes("/") ? "\\" : "/";
  return `${dir}${sep}${leaf}`;
}

async function openNfoView(params: { hash?: string; path?: string }) {
  busy.value = true;
  localError.value = null;
  try {
    const res = await extensionRPC("org.fleet.nfo", "ext.nfo.view", {
      hash: params.hash ?? "",
      path: params.path ?? "",
    });
    const result = (res.result ?? {}) as { path?: string; text?: string; encoding?: string };
    nfoModal.value = {
      path: result.path ?? params.path ?? "",
      text: result.text ?? "",
      encoding: result.encoding ?? "",
    };
  } catch (err) {
    localError.value = err instanceof Error ? err.message : t("ctx.viewNfo.fail");
  } finally {
    busy.value = false;
  }
}

async function ctxViewNfo() {
  const hashes = selectedInfoHashes();
  if (!hashes.length) return;
  await openNfoView({ hash: hashes[0] });
}

async function ctxTitleSearch() {
  const ids = selectedIds.value;
  const row =
    torrents.value.find((t) => focusHash.value && torrentRowId(t) === focusHash.value && ids[torrentRowId(t)]) ||
    torrents.value.find((t) => ids[torrentRowId(t)]);
  const title = row?.name?.trim();
  if (!title) return;
  busy.value = true;
  try {
    const res = await extensionRPC("org.fleet.titlesearch", "ext.titlesearch.resolve", { title, engine: "" });
    const links = (res.result as { links?: { name?: string; url?: string }[]; query?: string } | undefined)?.links ?? [];
    const query = (res.result as { query?: string } | undefined)?.query ?? title;
    let opened = 0;
    for (const link of links) {
      const url = link.url?.trim();
      if (!url) continue;
      const win = window.open(url, "_blank", "noopener,noreferrer");
      if (win) opened += 1;
    }
    if (opened > 0) {
      notice.value = t("ctx.titlesearch.done", { opened: String(opened), total: String(links.length), query });
    } else if (links.length) {
      notice.value = t("ctx.titlesearch.blocked", { total: String(links.length), query });
      localError.value = links.map((l) => `${l.name || "?"}: ${l.url || ""}`).join("\n");
    } else {
      notice.value = t("ctx.titlesearch.empty");
    }
  } catch (err) {
    localError.value = err instanceof Error ? err.message : t("ctx.titlesearch.fail");
  } finally {
    busy.value = false;
  }
}

function ctxBandwidth() {
  if (!bandwidthEnabled.value) return;
  showBandwidthModal.value = true;
}

async function applyBandwidthLimits() {
  const hashes = Object.keys(selectedIds.value).filter((id) => selectedIds.value[id]);
  if (!hashes.length) return;
  busy.value = true;
  try {
    const res = await extensionRPC("org.fleet.bandwidth", "ext.bandwidth.apply", {
      hashes,
      upKiBps: Number(bandwidthUpKiB.value) || 0,
      downKiBps: Number(bandwidthDownKiB.value) || 0,
    });
    const applied = (res.result as { applied?: number } | undefined)?.applied ?? hashes.length;
    notice.value = t("ctx.bandwidth.done", {
      count: String(applied),
      up: String(Number(bandwidthUpKiB.value) || 0),
      down: String(Number(bandwidthDownKiB.value) || 0),
    });
    showBandwidthModal.value = false;
  } catch (err) {
    localError.value = err instanceof Error ? err.message : t("ctx.bandwidth.fail");
  } finally {
    busy.value = false;
  }
}

async function clearBandwidthLimits() {
  const hashes = Object.keys(selectedIds.value).filter((id) => selectedIds.value[id]);
  if (!hashes.length) return;
  busy.value = true;
  try {
    const res = await extensionRPC("org.fleet.bandwidth", "ext.bandwidth.clear", { hashes });
    const applied = (res.result as { applied?: number } | undefined)?.applied ?? hashes.length;
    notice.value = t("ctx.bandwidth.cleared", { count: String(applied) });
    showBandwidthModal.value = false;
  } catch (err) {
    localError.value = err instanceof Error ? err.message : t("ctx.bandwidth.fail");
  } finally {
    busy.value = false;
  }
}

async function viewFileNfo(relPath: string) {
  const f = focused.value;
  if (!f) return;
  const abs = joinTorrentPath(f.directory || "", relPath);
  await openNfoView({ hash: f.hash, path: abs });
}

async function ctxSetPriority(priority: number) {
  const hashes = selectedInfoHashes();
  if (!hashes.length) return;
  busy.value = true;
  try {
    await Promise.all(hashes.map((h) => setTorrentPriority(h, priority)));
    notice.value = t("details.priority");
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Priority failed";
  } finally {
    busy.value = false;
  }
}

function ctxCopy(kind: "hash" | "name" | "path") {
  const targets = ctxTargets.value;
  if (!targets.length) return;
  const text = targets
    .map((t) => {
      if (kind === "hash") return t.hash;
      if (kind === "name") return t.name;
      return t.directory || "";
    })
    .filter(Boolean)
    .join("\n");
  void copyText(text);
}

function ctxProperties() {
  const id = focusHash.value || Object.keys(selectedIds.value)[0];
  if (!id) return;
  if (!isRowSelected(id)) {
    selectOnly(id);
  } else {
    focusHash.value = id;
  }
  detailTab.value = "general";
  // Always open a usable panel height — Properties must be visibly distinct
  // from a collapsed or already-open strip.
  detailsHeight.value = clampDetailsHeight(Math.max(detailsHeight.value, 280));
  try {
    localStorage.setItem(DETAILS_HEIGHT_KEY, String(detailsHeight.value));
  } catch {
    // ignore
  }
  closeTorrentContextMenu();
  void nextTick(() => {
    const panel = document.querySelector(".fleet-details") as HTMLElement | null;
    if (!panel) return;
    panel.scrollIntoView({ behavior: "smooth", block: "end" });
    panel.classList.remove("fleet-details-flash");
    void panel.offsetWidth;
    panel.classList.add("fleet-details-flash");
    panel.focus?.();
  });
}

function ctxDownloadTorrents() {
  const hashes = selectedInfoHashes();
  for (const hash of hashes) {
    const a = document.createElement("a");
    a.href = torrentSourceURL(hash);
    a.download = "";
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
  }
}

function toggleAll() {
  if (selectedCount.value === filtered.value.length && filtered.value.length > 0) {
    clearSelection();
    return;
  }
  const next: Record<string, true> = {};
  for (const t of filtered.value) next[torrentRowId(t)] = true;
  selectedIds.value = next;
}

function selectFilter(id: FilterId) {
  filter.value = id;
  showMenu.value = false;
  shell.value?.closeMobileNav?.();
}

function setSort(key: SortKey) {
  if (sortKey.value === key) sortAsc.value = !sortAsc.value;
  else {
    sortKey.value = key;
    sortAsc.value = key === "name" || key === "state" || key === "client";
  }
}

function onHeaderClick(col: { id: string; sortable?: boolean }) {
  if (!col.sortable) return;
  setSort(col.id as SortKey);
}

async function runAction(action: string) {
  const targets = selectedActionIds();
  if (targets.length === 0) {
    notice.value = "Select at least one torrent";
    return;
  }
  if (action === "erase" || action === "erase_data") {
    const label =
      action === "erase_data"
        ? "Remove torrents AND delete data?"
        : "Remove selected torrents from their clients?";
    if (!window.confirm(label)) return;
  }
  busy.value = true;
  notice.value = null;
  try {
    const res = await postTorrentActions(action, targets);
    const failed = res.results.filter((r) => !r.ok);
    notice.value = failed.length
      ? `${failed.length} failed: ${failed.map((f) => f.message).join("; ")}`
      : `${action} OK (${targets.length})`;
    clearSelection();
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Action failed";
  } finally {
    busy.value = false;
  }
}

async function onTorrentFile(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  await ingestTorrentFile(file);
}

async function submitAdd() {
  const lines = magnet.value
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (lines.length === 0 && !torrentBase64.value) {
    notice.value = "Provide a magnet or .torrent file";
    return;
  }
  busy.value = true;
  try {
    if (lines.length > 0) await addMagnets(lines, true);
    if (torrentBase64.value) await addTorrentBase64(torrentBase64.value, true);
    magnet.value = "";
    torrentBase64.value = "";
    torrentFileName.value = "";
    showAdd.value = false;
    notice.value = "Torrent(s) queued";
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Add failed";
  } finally {
    busy.value = false;
  }
}

async function saveLabel() {
  if (!focusedHash.value) return;
  busy.value = true;
  try {
    await setTorrentLabel(
      focusedHash.value,
      labelDraft.value.trim(),
      focused.value?.clientId,
    );
    notice.value = "Label saved";
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Label failed";
  } finally {
    busy.value = false;
  }
}

async function saveName() {
  if (!focusedHash.value) return;
  const name = nameDraft.value.trim();
  if (!name) return;
  busy.value = true;
  try {
    await setTorrentName(focusedHash.value, name);
    notice.value = "Name saved";
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Rename failed";
  } finally {
    busy.value = false;
  }
}

async function changeTorrentPriority(ev: Event) {
  if (!focusedHash.value) return;
  const priority = Number((ev.target as HTMLSelectElement).value);
  busy.value = true;
  try {
    await setTorrentPriority(focusedHash.value, priority);
    notice.value = "Priority saved";
    await refresh();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Priority failed";
  } finally {
    busy.value = false;
  }
}

async function changeFilePriority(index: number, ev: Event) {
  if (!focusedHash.value) return;
  const priority = Number((ev.target as HTMLSelectElement).value);
  busy.value = true;
  try {
    await setFilePriorities(focusedHash.value, [index], priority);
    const row = files.value.find((f) => f.index === index);
    if (row) row.priority = priority;
    notice.value = "File priority saved";
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "File priority failed";
  } finally {
    busy.value = false;
  }
}

async function submitTracker() {
  if (!focusedHash.value || !trackerDraft.value.trim()) return;
  busy.value = true;
  try {
    await addTracker(focusedHash.value, trackerDraft.value.trim());
    trackerDraft.value = "";
    notice.value = "Tracker added";
    await loadDetails();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Add tracker failed";
  } finally {
    busy.value = false;
  }
}

async function toggleTracker(t: TrackerEntry) {
  if (!focusedHash.value) return;
  busy.value = true;
  try {
    await setTrackerEnabled(focusedHash.value, t.id, !t.enabled);
    await loadDetails();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Tracker toggle failed";
  } finally {
    busy.value = false;
  }
}

async function onPeerComment(p: PeerEntry, value: string) {
  try {
    const res = await setPeerComment(p.address, value);
    p.comment = res.comment || undefined;
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Comment failed";
  }
}

async function deleteTracker(t: TrackerEntry) {
  if (!focusedHash.value) return;
  if (!window.confirm(`Remove tracker ${t.url}?`)) return;
  busy.value = true;
  try {
    await removeTracker(focusedHash.value, t.id);
    await loadDetails();
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Remove tracker failed";
  } finally {
    busy.value = false;
  }
}

async function inspectSelectedTorrent() {
  if (!torrentBase64.value) {
    notice.value = "Choose a .torrent file to inspect";
    return;
  }
  busy.value = true;
  dumpPreview.value = "";
  try {
    const res = await dumpTorrent(torrentBase64.value);
    dumpPreview.value = JSON.stringify(res.result.parsed ?? res.result, null, 2);
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Inspect failed";
  } finally {
    busy.value = false;
  }
}

async function submitCreateTorrent() {
  const paths = [createPaths.value, ...createExtraPaths.value.split(/\r?\n/)]
    .map((s) => s.trim())
    .filter(Boolean);
  const announce = createAnnounce.value
    .split(/\r?\n/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (!paths.length || !announce.length) {
    notice.value = "Paths and announce URLs are required";
    return;
  }
  busy.value = true;
  createJobId.value = "";
  createDownload.value = "";
  localError.value = null;
  try {
    const res = await createTorrent({
      paths,
      announce,
      name: createName.value.trim() || undefined,
      comment: createComment.value.trim() || undefined,
      private: createPrivate.value,
      pieceLength: createPieceLength.value > 0 ? createPieceLength.value : undefined,
    });
    createJobId.value = res.jobId;
    notice.value = `Create job ${res.jobId} queued`;
    for (let i = 0; i < 120; i += 1) {
      await new Promise((r) => setTimeout(r, 500));
      const job = await fetchJob(res.jobId);
      if (job.status === "succeeded") {
        createDownload.value = jobDownloadURL(res.jobId);
        notice.value = "Torrent created — download ready";
        return;
      }
      if (job.status === "failed" || job.status === "cancelled") {
        throw new Error(job.error || `job ${job.status}`);
      }
    }
    throw new Error("timed out waiting for mktorrent job");
  } catch (err) {
    localError.value = err instanceof Error ? err.message : "Create failed";
  } finally {
    busy.value = false;
  }
}

async function refreshClientHint(hash: string) {
  if (!clientLogosEnabled.value) return;
  try {
    const list = (await fetchTorrentPeers(hash)).peers;
    const dom = dominantClient(list.map((p) => p.clientVersion));
    if (dom) {
      clientByHash.value = { ...clientByHash.value, [hash]: dom };
    } else if (!(hash in clientByHash.value)) {
      clientByHash.value = { ...clientByHash.value, [hash]: "" };
    }
  } catch {
    // Client logos are best-effort; ignore probe failures.
  }
}

const clientProbeBusy = new Set<string>();
let clientProbeTimer: ReturnType<typeof setTimeout> | undefined;

function scheduleClientProbes() {
  if (!clientLogosEnabled.value) return;
  if (clientProbeTimer) clearTimeout(clientProbeTimer);
  clientProbeTimer = setTimeout(() => {
    void probeVisibleClients();
  }, 250);
}

async function probeVisibleClients() {
  if (!clientLogosEnabled.value || !engineConnected.value) return;
  const missing = filtered.value
    .map((t) => t.hash)
    .filter((h) => !(h in clientByHash.value) && !clientProbeBusy.has(h))
    .slice(0, 12);
  for (const hash of missing) {
    clientProbeBusy.add(hash);
    void refreshClientHint(hash).finally(() => clientProbeBusy.delete(hash));
  }
}

watch(
  () =>
    [
      clientLogosEnabled.value,
      filtered.value.map((t) => t.hash).join(","),
      engineConnected.value,
    ] as const,
  () => scheduleClientProbes(),
  { immediate: true },
);

onBeforeUnmount(() => {
  if (clientProbeTimer) clearTimeout(clientProbeTimer);
});

const focusedProgressPct = computed(() => {
  const p = focused.value?.progress ?? 0;
  return Math.max(0, Math.min(100, p * 100));
});

const focusedRingDash = computed(() => {
  const pct = focusedProgressPct.value;
  return `${pct.toFixed(1)} ${Math.max(0, 100 - pct).toFixed(1)}`;
});

const primaryTracker = computed(() => trackers.value.find((tr) => tr.enabled) ?? trackers.value[0] ?? null);

const focusedElapsed = computed(() => {
  const started = focused.value?.startedAt;
  if (!started || started <= 0) return "—";
  return formatETA(Math.max(0, Math.floor(Date.now() / 1000 - started)));
});

const focusedRemaining = computed(() => {
  const f = focused.value;
  if (!f) return "—";
  if (f.complete) return formatBytes(0);
  const left = f.leftBytes ?? Math.max(0, f.sizeBytes - f.bytesDone);
  return formatBytes(left);
});

const focusedRatioTone = computed(() => {
  const ratio = focused.value?.ratio ?? 0;
  const cfg = ratioColors.value;
  if (cfg?.enabled) {
    if (ratio < cfg.lowMax) return "ratio-low";
    if (ratio < cfg.midMax) return "ratio-mid";
    return "ratio-high";
  }
  if (ratio < 1) return "ratio-low";
  if (ratio < 2) return "ratio-mid";
  return "ratio-high";
});

function speedBarPct(rate: number): number {
  // 1 MiB/s ≈ full bar; soft-cap for readability.
  return Math.max(0, Math.min(100, (rate / (1024 * 1024)) * 100));
}

async function copyText(value: string) {
  if (!value) return;
  try {
    await navigator.clipboard.writeText(value);
    notice.value = t("details.copy");
  } catch {
    notice.value = value;
  }
}

async function loadDetails() {
  const hash = focusedHash.value;
  const clientId = focused.value?.clientId;
  const canLoad = Boolean(hash) && (engineConnected.value || Boolean(clientId));
  if (!canLoad || !hash) {
    files.value = [];
    trackers.value = [];
    peers.value = [];
    chunks.value = null;
    detailError.value = null;
    return;
  }
  detailError.value = null;
  if (clientLogosEnabled.value) void refreshClientHint(hash);
  try {
    if (detailTab.value === "general") {
      trackers.value = (await fetchTorrentTrackers(hash, clientId)).trackers;
    } else if (detailTab.value === "files") {
      files.value = (await fetchTorrentFiles(hash, clientId)).files;
    } else if (detailTab.value === "trackers") {
      trackers.value = (await fetchTorrentTrackers(hash, clientId)).trackers;
    } else if (detailTab.value === "peers") {
      peers.value = (await fetchTorrentPeers(hash, clientId)).peers;
      if (clientLogosEnabled.value) {
        const dom = dominantClient(peers.value.map((p) => p.clientVersion));
        if (dom) {
          clientByHash.value = { ...clientByHash.value, [hash]: dom };
        }
      }
    } else if (detailTab.value === "chunks") {
      if (clientId && clientId !== "local-rtorrent") {
        chunks.value = null;
        detailError.value = t("details.unsupported", { feature: "chunks" });
        return;
      }
      chunks.value = await fetchTorrentChunks(hash);
    }
  } catch (err) {
    const msg = err instanceof Error ? err.message : "Failed to load details";
    detailError.value = msg.includes("unsupported")
      ? t("details.unsupported", { feature: detailTab.value })
      : msg;
  }
}

watch([focusHash, detailTab], () => {
  void loadDetails();
});

function torrentKey(t: Torrent) {
  return torrentRowId(t);
}
</script>

<template>
  <AppShell
    ref="shell"
    :down-rate="totalDown"
    :up-rate="totalUp"
    :torrent-count="torrents.length"
    :downloading-count="counts.downloading"
    :seeding-count="counts.seeding"
    :clients-connected="engine?.clientsConnected ?? (engineConnected ? 1 : 0)"
    :clients-total="engine?.clientsTotal ?? (engineConnected ? 1 : 0)"
    :downloaders-connected="downloadersConnected"
    :client-problems="engine?.problems ?? []"
    :initial-search="query"
    @add="showAdd = true"
    @create="showCreate = true"
    @filters="onFiltersClick"
    @action="runAction"
    @search="query = $event"
  >
    <template #sidebar>
      <p class="fleet-side-heading">
        Status
      </p>
      <button
        v-for="f in statusFilters"
        :key="f.id"
        type="button"
        class="fleet-side-item"
        :data-active="filter === f.id"
        @click="selectFilter(f.id)"
      >
        <span>{{ f.label }}</span>
        <span class="count">{{ counts[f.id] ?? 0 }}</span>
      </button>
      <template v-if="labels.length">
        <p class="fleet-side-heading">
          Labels
        </p>
        <button
          v-for="l in labels"
          :key="l"
          type="button"
          class="fleet-side-item"
          :data-active="filter === `label:${l}`"
          @click="selectFilter(`label:${l}`)"
        >
          <span>{{ l }}</span>
          <span class="count">{{ counts[`label:${l}`] ?? 0 }}</span>
        </button>
      </template>
    </template>

    <div
      class="fleet-main-split"
      :data-drop-active="dragActive"
      @dragenter="onDragOver"
      @dragover="onDragOver"
      @dragleave="onDragLeave"
      @drop="onDrop"
    >
      <div
        v-if="error"
        class="fleet-banner"
        data-kind="error"
      >
        {{ error }}
      </div>
      <div
        v-else-if="notice"
        class="fleet-banner"
        data-kind="ok"
      >
        {{ notice }}
      </div>
      <div
        v-else
        class="fleet-banner"
      >
        {{ filtered.length }} shown · {{ selectedCount }} selected
        <span class="fleet-kbd-hint"> · ↑↓/jk select · space toggle · a add · / search · Del remove</span>
      </div>
      <div
        v-if="dragActive"
        class="fleet-drop-overlay"
        aria-hidden="true"
      >
        Drop .torrent or magnet
      </div>

      <div
        class="fleet-table-wrap"
        :style="tableWidthStyle"
      >
        <div class="fleet-table-head">
          <table class="fleet-table">
            <colgroup>
              <col
                v-for="col in visibleTableCols"
                :key="col.id"
                :style="{ width: `${col.width}px` }"
              >
            </colgroup>
            <thead>
              <tr>
                <th>
                  <input
                    type="checkbox"
                    :checked="filtered.length > 0 && selectedCount === filtered.length"
                    @change="toggleAll"
                  >
                  <span
                    class="fleet-col-resize"
                    @mousedown="startResize(0, $event)"
                  />
                </th>
                <th
                  v-if="showClientColEffective"
                  class="fleet-th-sortable"
                  :class="{ 'fleet-th-sorted': sortKey === 'client' }"
                  :aria-sort="sortKey === 'client' ? (sortAsc ? 'ascending' : 'descending') : undefined"
                  @click="setSort('client')"
                >
                  <button
                    type="button"
                    class="fleet-sort"
                    @click.stop="setSort('client')"
                  >
                    <span class="fleet-sort-label">Client</span>
                    <span
                      class="fleet-sort-ind"
                      aria-hidden="true"
                    >{{ sortKey === 'client' ? (sortAsc ? "▲" : "▼") : "↕" }}</span>
                  </button>
                  <span
                    class="fleet-col-resize"
                    @mousedown.stop="startResize(1, $event)"
                  />
                </th>
                <th
                  v-for="col in visibleTableCols.slice(dataColOffset)"
                  :key="col.id"
                  :class="{
                    'fleet-th-sortable': col.sortable,
                    'fleet-th-sorted': col.sortable && sortKey === col.id,
                  }"
                  :aria-sort="col.sortable && sortKey === col.id ? (sortAsc ? 'ascending' : 'descending') : undefined"
                  @click="onHeaderClick(col)"
                >
                  <button
                    v-if="col.sortable"
                    type="button"
                    class="fleet-sort"
                    @click.stop="setSort(col.id as SortKey)"
                  >
                    <span class="fleet-sort-label">{{ col.label }}</span>
                    <span
                      class="fleet-sort-ind"
                      aria-hidden="true"
                    >{{ sortKey === col.id ? (sortAsc ? "▲" : "▼") : "↕" }}</span>
                  </button>
                  <template v-else>
                    {{ col.label }}
                  </template>
                  <span
                    class="fleet-col-resize"
                    @mousedown.stop="startResize(tableCols.findIndex((c) => c.id === col.id), $event)"
                  />
                </th>
              </tr>
            </thead>
          </table>
        </div>
        <VirtualTable
          v-if="filtered.length > 0"
          :items="filtered"
          :row-height="rowHeightPx"
          :get-key="torrentKey"
        >
          <template #row="{ item: row }">
            <div
              class="fleet-vrow"
              :data-selected="isRowSelected(torrentRowId(row)) || focusHash === torrentRowId(row)"
              :data-ratio-band="ratioBand(row.ratio)"
              @click="onRowClick(row, $event)"
              @contextmenu="openTorrentContextMenu($event, torrentRowId(row))"
            >
              <table class="fleet-table">
                <colgroup>
                  <col
                    v-for="col in visibleTableCols"
                    :key="col.id"
                    :style="{ width: `${col.width}px` }"
                  >
                </colgroup>
                <tbody>
                  <tr>
                    <td @click.stop>
                      <input
                        type="checkbox"
                        :checked="isRowSelected(torrentRowId(row))"
                        @change="onRowCheckboxChange(row)"
                      >
                    </td>
                    <td
                      v-if="showClientColEffective"
                      class="fleet-client-cell"
                    >
                      <ClientLogo
                        v-if="row.clientKind"
                        :client="row.clientKind"
                        :title="row.clientName || row.clientKind"
                      />
                      <ClientLogo
                        v-else
                        :client="clientByHash[row.hash] || ''"
                      />
                    </td>
                    <td class="fleet-name">
                      <strong>{{ row.name }}</strong>
                      <small>{{ row.hash.slice(0, 8) }}{{ row.label ? ` · ${row.label}` : "" }}</small>
                    </td>
                    <td>
                      <span
                        class="fleet-state"
                        :data-state="row.state"
                      >{{ row.state }}</span>
                    </td>
                    <td>
                      <div class="fleet-progress">
                        <div class="fleet-progress-bar">
                          <span :style="{ width: `${Math.min(100, row.progress * 100)}%` }" />
                        </div>
                        <small>{{ (row.progress * 100).toFixed(1) }}%</small>
                      </div>
                    </td>
                    <td>{{ formatRate(row.downRate) }}</td>
                    <td v-if="!phoneNarrow">
                      {{ formatRate(row.upRate) }}
                    </td>
                    <td v-if="!phoneNarrow">
                      {{ formatETA(row.etaSeconds) }}
                    </td>
                    <td v-if="!phoneNarrow">
                      {{ formatBytes(row.sizeBytes) }}
                    </td>
                    <td
                      v-if="!phoneNarrow"
                      class="fleet-ratio-cell"
                      :data-ratio-band="ratioBand(row.ratio)"
                    >
                      {{ row.ratio.toFixed(3) }}
                    </td>
                    <td v-if="!phoneNarrow">
                      {{ row.seedsComplete ?? 0 }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </template>
        </VirtualTable>
        <p
          v-else
          class="fleet-banner"
        >
          No torrents match this view
        </p>
      </div>

      <section
        v-if="focused"
        class="fleet-details"
        tabindex="-1"
        :data-sheet="detailsSheet ? 'true' : undefined"
        :style="detailsSheet ? undefined : { height: `${detailsHeight}px` }"
      >
        <div
          class="fleet-details-chrome"
        >
          <p class="fleet-details-chrome-title">
            {{ focused.name }}
          </p>
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('details.close')"
            :aria-label="t('details.close')"
            @click="closeDetailsSheet"
          >
            <ToolbarIcon name="close" />
          </button>
        </div>
        <div
          v-if="!detailsSheet"
          class="fleet-details-resize"
          role="separator"
          aria-orientation="horizontal"
          :aria-label="t('details.resize')"
          @mousedown="startDetailsResize"
        />
        <div class="fleet-details-tabs">
          <button
            type="button"
            :data-active="detailTab === 'general'"
            @click="detailTab = 'general'"
          >
            {{ t("details.tab.general") }}
          </button>
          <button
            type="button"
            :data-active="detailTab === 'files'"
            @click="detailTab = 'files'"
          >
            {{ t("details.tab.files") }}
          </button>
          <button
            type="button"
            :data-active="detailTab === 'trackers'"
            @click="detailTab = 'trackers'"
          >
            {{ t("details.tab.trackers") }}
          </button>
          <button
            type="button"
            :data-active="detailTab === 'peers'"
            @click="detailTab = 'peers'"
          >
            {{ t("details.tab.peers") }}
          </button>
          <button
            type="button"
            :data-active="detailTab === 'chunks'"
            @click="detailTab = 'chunks'"
          >
            {{ t("details.tab.chunks") }}
          </button>
        </div>
        <div class="fleet-details-body">
          <p
            v-if="detailError"
            class="fleet-banner"
            data-kind="error"
          >
            {{ detailError }}
          </p>
          <div
            v-else-if="detailTab === 'general'"
            class="fleet-detail-general"
          >
            <div class="fleet-detail-top">
              <article class="fleet-detail-card fleet-detail-identity">
                <svg
                  class="fleet-detail-ring"
                  viewBox="0 0 36 36"
                  aria-hidden="true"
                >
                  <circle
                    class="bg"
                    cx="18"
                    cy="18"
                    r="15.5"
                  />
                  <circle
                    class="fg"
                    cx="18"
                    cy="18"
                    r="15.5"
                    pathLength="100"
                    :stroke-dasharray="focusedRingDash"
                  />
                  <text
                    x="18"
                    y="20.5"
                  >
                    {{ focusedProgressPct.toFixed(0) }}%
                  </text>
                </svg>
                <div class="fleet-detail-identity-meta">
                  <p
                    class="fleet-detail-name"
                    :title="focused.name"
                  >
                    {{ focused.name }}
                  </p>
                  <p class="fleet-detail-sub">
                    {{ formatBytes(focused.sizeBytes) }}
                    <span v-if="focused.label"> · {{ focused.label }}</span>
                  </p>
                  <span
                    class="fleet-detail-state"
                    :data-state="focused.state"
                  >{{ focused.state }}</span>
                </div>
              </article>

              <article class="fleet-detail-card fleet-detail-metric">
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.download") }}</span>
                  <span class="fleet-detail-metric-value">{{ formatRate(focused.downRate) }}</span>
                  <div
                    class="fleet-detail-speedbar"
                    data-kind="down"
                  >
                    <span :style="{ width: `${speedBarPct(focused.downRate)}%` }" />
                  </div>
                </div>
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.upload") }}</span>
                  <span class="fleet-detail-metric-value">{{ formatRate(focused.upRate) }}</span>
                  <div
                    class="fleet-detail-speedbar"
                    data-kind="up"
                  >
                    <span :style="{ width: `${speedBarPct(focused.upRate)}%` }" />
                  </div>
                </div>
              </article>

              <article class="fleet-detail-card fleet-detail-metric">
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.ratio") }}</span>
                  <span
                    class="fleet-detail-metric-value"
                    :data-tone="focusedRatioTone"
                  >{{ focused.ratio.toFixed(3) }}</span>
                  <span class="fleet-detail-sub">
                    {{
                      t("details.ratioTotals", {
                        up: formatBytes(focused.upTotal ?? 0),
                        down: formatBytes(focused.downTotal ?? 0),
                      })
                    }}
                  </span>
                </div>
              </article>

              <article class="fleet-detail-card fleet-detail-metric">
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.eta") }}</span>
                  <span class="fleet-detail-metric-value">{{ formatETA(focused.etaSeconds) }}</span>
                </div>
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.elapsed") }}</span>
                  <span class="fleet-detail-metric-value">{{ focusedElapsed }}</span>
                </div>
                <div class="fleet-detail-metric-row">
                  <span class="fleet-detail-metric-label">{{ t("details.remaining") }}</span>
                  <span class="fleet-detail-metric-value">{{ focusedRemaining }}</span>
                </div>
              </article>
            </div>

            <div class="fleet-detail-bottom">
              <article class="fleet-detail-card">
                <dl class="fleet-detail-kv">
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.seeds") }}</dt>
                    <dd>{{ focused.seedsComplete ?? 0 }}</dd>
                  </div>
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.peers") }}</dt>
                    <dd>{{ focused.peersConnected ?? 0 }}</dd>
                  </div>
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.seeded") }}</dt>
                    <dd>{{ formatETA(seedingSecondsFrom(focused.finishedAt)) }}</dd>
                  </div>
                </dl>
              </article>

              <article class="fleet-detail-card">
                <dl class="fleet-detail-kv">
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.tracker") }}</dt>
                    <dd>
                      <span :title="primaryTracker?.url || ''">{{ primaryTracker?.url || "—" }}</span>
                      <button
                        v-if="primaryTracker?.url"
                        type="button"
                        class="fleet-detail-copy"
                        :title="t('details.copy')"
                        @click="copyText(primaryTracker.url)"
                      >
                        ⎘
                      </button>
                    </dd>
                  </div>
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.trackerStatus") }}</dt>
                    <dd>
                      <span
                        v-if="primaryTracker?.enabled"
                        class="fleet-detail-ok"
                      >OK</span>
                      <span
                        v-else
                        class="fleet-detail-bad"
                      >—</span>
                    </dd>
                  </div>
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.trackerNext") }}</dt>
                    <dd>—</dd>
                  </div>
                </dl>
              </article>

              <article class="fleet-detail-card">
                <dl class="fleet-detail-kv">
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.savePath") }}</dt>
                    <dd>
                      <span :title="focused.directory || ''">{{ focused.directory || "—" }}</span>
                      <button
                        v-if="focused.directory"
                        type="button"
                        class="fleet-detail-copy"
                        :title="t('details.copy')"
                        @click="copyText(focused.directory)"
                      >
                        ⎘
                      </button>
                    </dd>
                  </div>
                  <div class="fleet-detail-kv-row">
                    <dt>{{ t("details.hash") }}</dt>
                    <dd>
                      <span :title="focused.hash">{{ focused.hash }}</span>
                      <button
                        type="button"
                        class="fleet-detail-copy"
                        :title="t('details.copy')"
                        @click="copyText(focused.hash)"
                      >
                        ⎘
                      </button>
                    </dd>
                  </div>
                </dl>
              </article>
            </div>

            <div class="fleet-detail-manage">
              <label>
                {{ t("details.rename") }}
                <span class="fleet-inline">
                  <input
                    v-model="nameDraft"
                    class="fleet-control"
                    type="text"
                    placeholder="torrent display name"
                  >
                  <button
                    type="button"
                    class="fleet-btn"
                    :disabled="busy"
                    @click="saveName"
                  >
                    {{ t("details.rename") }}
                  </button>
                </span>
              </label>
              <label>
                {{ t("details.label") }}
                <span class="fleet-inline">
                  <input
                    v-model="labelDraft"
                    class="fleet-control"
                    type="text"
                    placeholder="optional label"
                  >
                  <button
                    type="button"
                    class="fleet-btn"
                    :disabled="busy"
                    @click="saveLabel"
                  >
                    {{ t("details.save") }}
                  </button>
                </span>
              </label>
              <label>
                {{ t("details.priority") }}
                <select
                  class="fleet-control"
                  :value="focused.priority ?? 2"
                  @change="changeTorrentPriority"
                >
                  <option :value="0">
                    {{ t("details.priority.off") }}
                  </option>
                  <option :value="1">
                    {{ t("details.priority.low") }}
                  </option>
                  <option :value="2">
                    {{ t("details.priority.normal") }}
                  </option>
                  <option :value="3">
                    {{ t("details.priority.high") }}
                  </option>
                </select>
              </label>
              <a
                class="fleet-btn"
                :href="torrentSourceURL(focused.hash)"
                download
              >{{ t("details.downloadTorrent") }}</a>
            </div>
          </div>
          <table v-else-if="detailTab === 'files'">
            <thead>
              <tr>
                <th>Path</th>
                <th>Size</th>
                <th>Priority</th>
                <th />
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="f in files"
                :key="f.index"
              >
                <td>{{ f.path }}</td>
                <td>{{ formatBytes(f.sizeBytes) }}</td>
                <td>
                  <select
                    class="fleet-control"
                    :value="f.priority"
                    @change="changeFilePriority(f.index, $event)"
                  >
                    <option :value="0">
                      Off
                    </option>
                    <option :value="1">
                      Normal
                    </option>
                    <option :value="2">
                      High
                    </option>
                  </select>
                </td>
                <td class="fleet-files-actions">
                  <button
                    v-if="nfoEnabled && isNfoPath(f.path)"
                    type="button"
                    class="fleet-btn"
                    :disabled="busy"
                    @click="viewFileNfo(f.path)"
                  >
                    {{ t("details.viewNfo") }}
                  </button>
                  <a
                    class="fleet-btn"
                    :href="torrentFileContentURL(focused.hash, f.index)"
                    download
                  >Download</a>
                </td>
              </tr>
              <tr v-if="files.length === 0">
                <td colspan="4">
                  No files
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else-if="detailTab === 'trackers'">
            <label>
              Add tracker
              <span class="fleet-inline">
                <input
                  v-model="trackerDraft"
                  class="fleet-control"
                  type="url"
                  placeholder="https://tracker.example/announce"
                >
                <button
                  type="button"
                  class="fleet-btn"
                  :disabled="busy"
                  @click="submitTracker"
                >
                  Add
                </button>
              </span>
            </label>
            <table>
              <thead>
                <tr>
                  <th>URL</th>
                  <th>Seeds</th>
                  <th>Peers</th>
                  <th>Enabled</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="tr in trackers"
                  :key="tr.id"
                >
                  <td>{{ tr.url }}</td>
                  <td>{{ tr.seeders }}</td>
                  <td>{{ tr.leechers }}</td>
                  <td>
                    <button
                      type="button"
                      class="fleet-btn"
                      @click="toggleTracker(tr)"
                    >
                      {{ tr.enabled ? "on" : "off" }}
                    </button>
                  </td>
                  <td>
                    <button
                      type="button"
                      class="fleet-btn"
                      @click="deleteTracker(tr)"
                    >
                      Remove
                    </button>
                  </td>
                </tr>
                <tr v-if="trackers.length === 0">
                  <td colspan="5">
                    No trackers
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <table v-else-if="detailTab === 'peers'">
            <thead>
              <tr>
                <th>CC</th>
                <th>Address</th>
                <th>Client</th>
                <th>Comment</th>
                <th>Down</th>
                <th>Up</th>
                <th>%</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(p, i) in peers"
                :key="i"
              >
                <td>{{ p.country || "—" }}</td>
                <td>{{ p.address }}</td>
                <td>{{ p.clientVersion }}</td>
                <td>
                  <input
                    class="fleet-control"
                    type="text"
                    maxlength="512"
                    :value="p.comment || ''"
                    placeholder="Note"
                    @change="onPeerComment(p, ($event.target as HTMLInputElement).value)"
                  >
                </td>
                <td>{{ formatRate(p.downRate) }}</td>
                <td>{{ formatRate(p.upRate) }}</td>
                <td>{{ p.completedPercent }}</td>
              </tr>
              <tr v-if="peers.length === 0">
                <td colspan="7">
                  No peers
                </td>
              </tr>
            </tbody>
          </table>
          <div
            v-else-if="detailTab === 'chunks' && chunks"
            class="fleet-chunks"
          >
            <p>
              {{ chunks.completed }} / {{ chunks.sizeChunks }} pieces
              · {{ formatBytes(chunks.chunkSize) }} each
              <span v-if="(chunks.sizeChunks || 0) > chunkCells.length">
                · showing first {{ chunkCells.length }}
              </span>
            </p>
            <div
              class="fleet-chunk-grid"
              role="img"
              :aria-label="t('details.chunksComplete', { done: chunks.completed, total: chunks.sizeChunks })"
            >
              <span
                v-for="(done, i) in chunkCells"
                :key="i"
                class="fleet-chunk-cell"
                :data-done="done"
              />
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppShell>

  <TorrentContextMenu
    v-if="ctxMenu"
    :x="ctxMenu.x"
    :y="ctxMenu.y"
    :count="selectedCount || 1"
    :labels="labels"
    :active-label="ctxActiveLabel"
    :can-pause="ctxCanPause"
    :can-stop="ctxCanStop"
    :show-cross-seed="crossSeedEnabled"
    :show-view-nfo="nfoEnabled"
    :show-title-search="titleSearchEnabled"
    :show-bandwidth="bandwidthEnabled"
    @close="closeTorrentContextMenu"
    @action="runAction"
    @set-label="ctxSetLabel"
    @set-priority="ctxSetPriority"
    @copy="ctxCopy"
    @properties="ctxProperties"
    @download="ctxDownloadTorrents"
    @cross-seed="ctxCrossSeed"
    @view-nfo="ctxViewNfo"
    @title-search="ctxTitleSearch"
    @bandwidth="ctxBandwidth"
  />

  <Modal
    v-if="showBandwidthModal"
    :title="t('ctx.bandwidth')"
    @close="showBandwidthModal = false"
  >
    <p class="fleet-ext-note">
      {{ t("ctx.bandwidth.help") }}
    </p>
    <label>
      {{ t("ctx.bandwidth.up") }}
      <input
        v-model.number="bandwidthUpKiB"
        class="fleet-control"
        type="number"
        min="0"
        step="1"
      >
    </label>
    <label>
      {{ t("ctx.bandwidth.down") }}
      <input
        v-model.number="bandwidthDownKiB"
        class="fleet-control"
        type="number"
        min="0"
        step="1"
      >
    </label>
    <div class="fleet-ext-actions">
      <button
        type="button"
        class="fleet-btn-primary"
        :disabled="busy"
        @click="applyBandwidthLimits"
      >
        {{ t("ctx.bandwidth.apply") }}
      </button>
      <button
        type="button"
        class="fleet-btn"
        :disabled="busy"
        @click="clearBandwidthLimits"
      >
        {{ t("ctx.bandwidth.clear") }}
      </button>
    </div>
  </Modal>

  <Modal
    v-if="nfoModal"
    :title="nfoModal.path || t('ctx.viewNfo')"
    @close="nfoModal = null"
  >
    <p
      v-if="nfoModal.encoding"
      class="fleet-ext-note"
    >
      {{ nfoModal.encoding }}
    </p>
    <pre class="fleet-nfo">{{ nfoModal.text }}</pre>
  </Modal>

  <Modal
    v-if="showMenu"
    :title="t('nav.filters')"
    @close="showMenu = false"
  >
    <div class="fleet-menu-list">
      <button
        v-for="f in statusFilters"
        :key="f.id"
        type="button"
        class="fleet-side-item"
        :data-active="filter === f.id"
        @click="selectFilter(f.id)"
      >
        <span>{{ f.label }}</span>
        <span class="count">{{ counts[f.id] ?? 0 }}</span>
      </button>
      <template v-if="labels.length">
        <p class="fleet-side-heading">
          Labels
        </p>
        <button
          v-for="l in labels"
          :key="l"
          type="button"
          class="fleet-side-item"
          :data-active="filter === `label:${l}`"
          @click="selectFilter(`label:${l}`)"
        >
          <span>{{ l }}</span>
          <span class="count">{{ counts[`label:${l}`] ?? 0 }}</span>
        </button>
      </template>
    </div>
    <template #footer>
      <button
        type="button"
        class="fleet-btn"
        @click="restoreSidebar"
      >
        Show sidebar
      </button>
      <button
        type="button"
        class="fleet-btn"
        @click="showMenu = false"
      >
        Close
      </button>
    </template>
  </Modal>

  <Modal
    v-if="showAdd"
    :title="t('add.title')"
    @close="showAdd = false"
  >
    <label>
      Magnet link(s)
      <textarea
        v-model="magnet"
        class="fleet-control"
        placeholder="magnet:?xt=urn:btih:…"
      />
    </label>
    <label>
      Or .torrent file
      <input
        type="file"
        accept=".torrent,application/x-bittorrent"
        @change="onTorrentFile"
      >
      <span v-if="torrentFileName">{{ torrentFileName }}</span>
    </label>
    <pre
      v-if="dumpPreview"
      class="fleet-dump"
    >{{ dumpPreview }}</pre>
    <template #footer>
      <button
        type="button"
        class="fleet-btn"
        :disabled="busy || !torrentBase64"
        @click="inspectSelectedTorrent"
      >
        Inspect
      </button>
      <button
        type="button"
        class="fleet-btn"
        @click="showAdd = false"
      >
        Cancel
      </button>
      <button
        type="button"
        class="fleet-btn-primary"
        :disabled="busy"
        @click="submitAdd"
      >
        Add
      </button>
    </template>
  </Modal>

  <Modal
    v-if="showCreate"
    :title="t('create.title')"
    @close="showCreate = false"
  >
    <label>
      Source path under download_roots
      <PathPicker
        :model-value="createPaths.split('\n')[0] || ''"
        @update:model-value="(v) => { createPaths = v; }"
      />
    </label>
    <label>
      Extra source paths (optional, one per line)
      <textarea
        v-model="createExtraPaths"
        class="fleet-control"
        placeholder="/home/alice/downloads/other"
      />
    </label>
    <label>
      Announce URL(s)
      <textarea
        v-model="createAnnounce"
        class="fleet-control"
        placeholder="https://tracker.example/announce"
      />
    </label>
    <label>
      Name (optional)
      <input
        v-model="createName"
        class="fleet-control"
        type="text"
      >
    </label>
    <label>
      Comment (optional)
      <input
        v-model="createComment"
        class="fleet-control"
        type="text"
      >
    </label>
    <label class="fleet-inline">
      <input
        v-model="createPrivate"
        type="checkbox"
      >
      Private torrent
    </label>
    <label>
      Piece length bytes (0 = default)
      <input
        v-model.number="createPieceLength"
        class="fleet-control"
        type="number"
        min="0"
        step="1"
      >
    </label>
    <p
      v-if="createJobId"
      class="fleet-ext-note"
    >
      Job: {{ createJobId }}
    </p>
    <p v-if="createDownload">
      <a
        :href="createDownload"
        download
      >Download .torrent</a>
    </p>
    <template #footer>
      <button
        type="button"
        class="fleet-btn"
        @click="showCreate = false"
      >
        Close
      </button>
      <button
        type="button"
        class="fleet-btn-primary"
        :disabled="busy"
        @click="submitCreateTorrent"
      >
        Create
      </button>
    </template>
  </Modal>
</template>

<style scoped>
.fleet-th-sortable {
  cursor: pointer;
  user-select: none;
}

.fleet-th-sortable:hover,
.fleet-th-sorted {
  color: var(--fleet-foreground);
}

.fleet-sort {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  width: 100%;
  min-width: 0;
  min-height: 1.25rem;
  border: none;
  background: transparent;
  color: inherit;
  font: inherit;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  cursor: pointer;
  padding: 0;
  text-align: left;
}

.fleet-sort-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fleet-sort-ind {
  flex: none;
  font-size: 0.7em;
  opacity: 0.45;
  line-height: 1;
}

.fleet-th-sorted .fleet-sort-ind {
  opacity: 1;
  color: var(--fleet-primary-text);
}

.fleet-inline {
  display: flex;
  gap: 8px;
  align-items: center;
}

.fleet-inline input,
.fleet-details-body select {
  border: 1px solid var(--fleet-border);
  border-radius: var(--fleet-radius);
  background: var(--fleet-input);
  color: var(--fleet-foreground);
  padding: 6px 8px;
}

.fleet-inline input {
  flex: 1;
}

.fleet-details-body > label,
.fleet-details-body > div > label {
  display: grid;
  gap: 6px;
  margin-top: 10px;
  color: var(--fleet-fg-muted);
  font-weight: 600;
}

.fleet-files-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
}

.fleet-nfo {
  margin: 0;
  padding: 0.75rem 1rem;
  max-height: min(60vh, 28rem);
  overflow: auto;
  white-space: pre;
  font-family: var(--fleet-font-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace);
  font-size: 0.8rem;
  line-height: 1.35;
  background: var(--fleet-surface-2, rgba(0, 0, 0, 0.25));
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: 0.35rem;
}

.fleet-detail-manage label {
  margin-top: 0;
}

.fleet-dump {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  padding: 8px;
  border-radius: var(--fleet-radius);
  border: 1px solid var(--fleet-border);
  background: var(--fleet-input);
  font-family: var(--fleet-font-mono);
  font-size: 11px;
  white-space: pre-wrap;
}
</style>
