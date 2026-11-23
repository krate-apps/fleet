<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  createShare,
  deleteShare,
  extensionRPC,
  fetchCapabilities,
  fetchFSList,
  fetchFSRoots,
  fetchFSText,
  fetchShares,
  fsContentURL,
  fsCopy,
  fsMove,
  fsRemove,
  withBase,
  type CapabilitiesResponse,
  type FSEntry,
  type ShareLink,
} from "@/api/client";
import AppShell from "@/components/AppShell.vue";
import DownloadQueuePanel from "@/components/DownloadQueuePanel.vue";
import Modal from "@/components/Modal.vue";
import PathPicker from "@/components/PathPicker.vue";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import { useExtensionUI } from "@/composables/useExtensionUI";
import {
  APP_ID_TO_QUEUE_EXT,
  DOWNLOAD_QUEUE_PROVIDERS,
  FILES_TAB_FOLDERS,
  type DownloadQueueProvider,
} from "@/extensions/downloadQueues";
import { useI18n } from "@/i18n/useI18n";
import {
  FILEMANAGER_EXT_ID,
  RCLONE_EXT_ID,
  groupRootsByApp,
  isMountRootPath,
  mountLabel,
  pathParts,
  preferDownloadsRoot,
  rootAppId,
  rootLabel,
  type RootGroup,
} from "@/lib/downloadRoots";

type RcloneDrive = {
  id: string;
  name: string;
  remote: string;
  path?: string;
  browsable?: boolean;
};

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const { active, refresh: refreshExtensions } = useExtensionUI();

const roots = ref<string[]>([]);
const cwd = ref("");
const entries = ref<FSEntry[]>([]);
const error = ref("");
const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const previewText = ref("");
const previewEncoding = ref("");
const previewKind = ref<"none" | "text" | "image" | "video">("none");
const shares = ref<ShareLink[]>([]);
const selected = ref<FSEntry | null>(null);
const checked = ref<Record<string, boolean>>({});
const shareOpen = ref(false);
const shareTtlSec = ref(86400);
const shareMaxDownloads = ref(0);
const engine = ref<CapabilitiesResponse | null>(null);
const selectedAppId = ref("");
const destPickerOpen = ref(false);
const destPickerOp = ref<"move" | "copy">("move");
const destPath = ref("");
const rcloneDrives = ref<RcloneDrive[]>([]);
let engineTimer: ReturnType<typeof setInterval> | undefined;

const filemanagerActive = computed(() =>
  active.value.some((e) => e.id === FILEMANAGER_EXT_ID),
);
const rcloneActive = computed(() => active.value.some((e) => e.id === RCLONE_EXT_ID));

async function refreshEngine() {
  try {
    engine.value = await fetchCapabilities();
  } catch {
    // keep last known footer state
  }
}

const queueProviders = computed(() => {
  const activeIds = new Set(active.value.map((e) => e.id));
  return DOWNLOAD_QUEUE_PROVIDERS.filter((p) => activeIds.has(p.extensionId));
});

const downloadersConnected = computed(() => queueProviders.value.length);

const activeTab = computed(() => {
  const raw = typeof route.query.tab === "string" ? route.query.tab : FILES_TAB_FOLDERS;
  if (raw === FILES_TAB_FOLDERS) return FILES_TAB_FOLDERS;
  if (queueProviders.value.some((p) => p.extensionId === raw)) return raw;
  return FILES_TAB_FOLDERS;
});

const isFoldersTab = computed(() => activeTab.value === FILES_TAB_FOLDERS);

const activeProvider = computed((): DownloadQueueProvider | null => {
  if (isFoldersTab.value) return null;
  return queueProviders.value.find((p) => p.extensionId === activeTab.value) ?? null;
});

const parentPath = computed(() => {
  if (!cwd.value) return "";
  const parts = cwd.value.replace(/\\/g, "/").split("/");
  parts.pop();
  const parent = parts.join("/") || "/";
  if (roots.value.some((r) => cwd.value === r)) return "";
  return parent;
});

const checkedEntries = computed(() => entries.value.filter((e) => checked.value[e.path]));
const checkedCount = computed(() => checkedEntries.value.length);

const maxVideoBytes = 100 * 1024 * 1024;

function setTab(tab: string) {
  const q: Record<string, string> = {};
  if (typeof route.query.path === "string" && route.query.path) q.path = route.query.path;
  if (tab !== FILES_TAB_FOLDERS) q.tab = tab;
  void router.replace({ path: "/files", query: q });
}

function clearPreview() {
  previewText.value = "";
  previewEncoding.value = "";
  previewKind.value = "none";
}

function isImageName(name: string) {
  return /\.(png|jpe?g|gif|webp|svg)$/i.test(name);
}

function isVideoName(name: string) {
  return /\.(mp4|webm|mkv|avi)$/i.test(name);
}

function isMediaInfoName(name: string) {
  return /\.(mp4|webm|mkv|avi|m4v|mov|wmv|flv|ts|m2ts|mp3|flac|aac|ogg|wav|m4a)$/i.test(name);
}

type FilesApp = RootGroup;

function rootFolderName(path: string): string {
  const parts = pathParts(path);
  return parts[parts.length - 1] || path;
}

/** download_roots that belong to client apps (not …/mounts/…). */
const appRootsAll = computed(() => roots.value.filter((r) => !isMountRootPath(r)));

const apps = computed((): FilesApp[] => groupRootsByApp(appRootsAll.value));

const selectedApp = computed(() => apps.value.find((a) => a.id === selectedAppId.value) ?? null);

const selectedAppRoots = computed(() => selectedApp.value?.roots ?? []);

/** Local mount paths already listed in download_roots. */
const mountRootsFromFS = computed(() => roots.value.filter(isMountRootPath));

/** Sidebar Mounts: rclone drives + FS mount roots (deduped by path). */
const mountEntries = computed(() => {
  const out: Array<{ id: string; label: string; path: string; browsable: boolean; title: string }> = [];
  const seen = new Set<string>();
  for (const r of mountRootsFromFS.value) {
    const p = r.replace(/\/+$/, "");
    seen.add(p);
    out.push({
      id: `fs:${p}`,
      label: mountLabel(r),
      path: r,
      browsable: true,
      title: r,
    });
  }
  for (const d of rcloneDrives.value) {
    const path = (d.path || "").replace(/\/+$/, "");
    if (path && seen.has(path)) continue;
    if (path) seen.add(path);
    out.push({
      id: `rclone:${d.id}`,
      label: d.name,
      path: d.path || "",
      browsable: !!d.browsable && !!d.path,
      title: d.path || d.remote,
    });
  }
  return out.sort((a, b) => a.label.localeCompare(b.label));
});

function queueProviderForApp(appId: string): DownloadQueueProvider | null {
  const extId = APP_ID_TO_QUEUE_EXT[appId];
  if (!extId) return null;
  return queueProviders.value.find((p) => p.extensionId === extId) ?? null;
}

function selectApp(app: FilesApp) {
  selectedAppId.value = app.id;
  const qp = queueProviderForApp(app.id);
  if (qp) {
    // Hosters: open live queue from the sidebar (no Folders/JD/pyLoad tab strip).
    setTab(qp.extensionId);
    return;
  }
  setTab(FILES_TAB_FOLDERS);
  const target = preferDownloadsRoot(app.roots);
  if (target) void loadDir(target);
}

function isRootPath(path: string): boolean {
  return roots.value.some((r) => r === path);
}

function entryDisplayName(e: FSEntry): string {
  if (isRootPath(e.path)) return rootLabel(e.path);
  return e.name;
}

async function loadRcloneDrives() {
  if (!rcloneActive.value) {
    rcloneDrives.value = [];
    return;
  }
  try {
    const res = await extensionRPC(RCLONE_EXT_ID, "ext.rclone.listDrives", {});
    const drives = (res.result as { drives?: RcloneDrive[] } | undefined)?.drives;
    rcloneDrives.value = Array.isArray(drives) ? drives : [];
  } catch {
    rcloneDrives.value = [];
  }
}

function openMount(entry: { path: string; browsable: boolean }) {
  if (!entry.browsable || !entry.path) {
    notice.value = t("files.mountUnavailable");
    noticeKind.value = "error";
    return;
  }
  setTab(FILES_TAB_FOLDERS);
  void loadDir(entry.path);
}

async function loadRoots() {
  const data = await fetchFSRoots();
  roots.value = data.roots ?? [];
  if (!selectedAppId.value && apps.value.length) {
    selectedAppId.value = apps.value[0]!.id;
  } else if (selectedAppId.value && !apps.value.some((a) => a.id === selectedAppId.value)) {
    selectedAppId.value = apps.value[0]?.id ?? "";
  }
  await loadRcloneDrives();
}

async function loadDir(path = "") {
  busy.value = true;
  error.value = "";
  clearPreview();
  notice.value = "";
  checked.value = {};
  try {
    const data = await fetchFSList(path);
    cwd.value = data.cwd || path;
    entries.value = data.entries ?? [];
    selected.value = null;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function loadShares() {
  try {
    const data = await fetchShares();
    shares.value = data.shares ?? [];
  } catch {
    shares.value = [];
  }
}

function toggleCheck(entry: FSEntry, ev: Event) {
  ev.stopPropagation();
  checked.value = { ...checked.value, [entry.path]: !checked.value[entry.path] };
}

function toggleCheckAll() {
  const allOn = entries.value.length > 0 && entries.value.every((e) => checked.value[e.path]);
  const next: Record<string, boolean> = {};
  if (!allOn) {
    for (const e of entries.value) next[e.path] = true;
  }
  checked.value = next;
}

function joinDest(destDir: string, name: string) {
  const base = destDir.replace(/\/+$/, "");
  return `${base}/${name}`;
}

function openDestPicker(op: "move" | "copy") {
  const items = checkedEntries.value;
  if (!items.length) {
    notice.value = t("files.selectFirst");
    noticeKind.value = "error";
    return;
  }
  destPickerOp.value = op;
  destPath.value = cwd.value || preferDownloadsRoot(selectedAppRoots.value) || roots.value[0] || "";
  destPickerOpen.value = true;
}

async function confirmDestPicker() {
  const destDir = destPath.value.trim();
  if (!destDir) {
    notice.value = t("files.pickDestination");
    noticeKind.value = "error";
    return;
  }
  destPickerOpen.value = false;
  await runBulkOp(destPickerOp.value, destDir);
}

async function bulkOp(op: "move" | "copy" | "remove") {
  if (op === "move" || op === "copy") {
    openDestPicker(op);
    return;
  }
  const items = checkedEntries.value;
  if (!items.length) {
    notice.value = t("files.selectFirst");
    noticeKind.value = "error";
    return;
  }
  const ok = window.confirm(t("files.deleteConfirm", { n: items.length }));
  if (!ok) return;
  await runBulkOp("remove", "");
}

async function runBulkOp(op: "move" | "copy" | "remove", destDir: string) {
  const items = checkedEntries.value;
  if (!items.length) return;
  busy.value = true;
  notice.value = "";
  let done = 0;
  let failed = 0;
  const errors: string[] = [];
  for (const item of items) {
    try {
      if (op === "remove") {
        await fsRemove(item.path, true);
      } else {
        const to = joinDest(destDir, item.name);
        if (op === "move") await fsMove(item.path, to);
        else await fsCopy(item.path, to);
      }
      done++;
      notice.value = `${op}: ${done}/${items.length}`;
      noticeKind.value = "ok";
    } catch (e) {
      failed++;
      errors.push(`${item.name}: ${e instanceof Error ? e.message : String(e)}`);
      notice.value = `${op}: ${done}/${items.length} (failed ${failed})`;
      noticeKind.value = "error";
    }
  }
  if (errors.length) {
    notice.value = `${op} done=${done} failed=${failed}\n${errors.slice(0, 5).join("\n")}`;
    noticeKind.value = "error";
  } else {
    notice.value = `${op} complete: ${done}/${items.length}`;
    noticeKind.value = "ok";
  }
  busy.value = false;
  checked.value = {};
  await loadDir(cwd.value || "");
}

async function openEntry(entry: FSEntry) {
  selected.value = entry;
  clearPreview();
  if (entry.isDir) {
    await loadDir(entry.path);
    return;
  }
  const lower = entry.name.toLowerCase();
  if (isImageName(lower)) {
    previewKind.value = "image";
    return;
  }
  if (isVideoName(lower) && (entry.size <= 0 || entry.size <= maxVideoBytes)) {
    previewKind.value = "video";
    return;
  }
  if (lower.endsWith(".nfo") || lower.endsWith(".txt") || lower.endsWith(".md") || lower.endsWith(".log")) {
    try {
      const data = await fetchFSText(entry.path);
      previewText.value = data.text;
      previewEncoding.value = data.encoding || "";
      previewKind.value = "text";
    } catch (e) {
      previewText.value = e instanceof Error ? e.message : String(e);
      previewKind.value = "text";
    }
  }
}

function openShareForm() {
  if (!selected.value || selected.value.isDir) return;
  shareOpen.value = true;
  shareTtlSec.value = 86400;
  shareMaxDownloads.value = 0;
}

async function runMediaInfo() {
  if (!selected.value || selected.value.isDir) return;
  busy.value = true;
  notice.value = "";
  try {
    const res = await extensionRPC(FILEMANAGER_EXT_ID, "ext.filemanager.mediainfo", {
      path: selected.value.path,
    });
    const text = (res.result as { text?: string } | undefined)?.text;
    if (!text) {
      throw new Error(t("files.mediainfoEmpty"));
    }
    previewEncoding.value = "";
    previewText.value = text;
    previewKind.value = "text";
    notice.value = t("files.mediainfoOk");
    noticeKind.value = "ok";
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
    noticeKind.value = "error";
  } finally {
    busy.value = false;
  }
}

async function shareSelected() {
  if (!selected.value || selected.value.isDir) return;
  busy.value = true;
  notice.value = "";
  try {
    const res = await createShare({
      path: selected.value.path,
      ttlSec: Number(shareTtlSec.value) || 86400,
      maxDownloads: Number(shareMaxDownloads.value) || 0,
    });
    notice.value = `Share URL: ${res.url}`;
    noticeKind.value = "ok";
    shareOpen.value = false;
    await loadShares();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
    noticeKind.value = "error";
  } finally {
    busy.value = false;
  }
}

async function revokeShare(token: string) {
  await deleteShare(token);
  await loadShares();
}

function formatExpiry(s: ShareLink) {
  if (!s.expiresAt) return "no expiry";
  return new Date(s.expiresAt * 1000).toLocaleString();
}

function onQueueNotice(msg: string, kind: "ok" | "error") {
  notice.value = msg;
  noticeKind.value = kind;
}

onMounted(async () => {
  try {
    await refreshExtensions(true);
    if (!filemanagerActive.value) {
      notice.value = t("files.extRequired");
      noticeKind.value = "error";
      void router.replace({ path: "/", query: { modal: "extensions", ext: FILEMANAGER_EXT_ID } });
      return;
    }
    await refreshEngine();
    engineTimer = setInterval(() => void refreshEngine(), 10000);
    await loadRoots();
    const initial = typeof route.query.path === "string" ? route.query.path : "";
    await loadDir(initial);
    await loadShares();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
});

onUnmounted(() => {
  if (engineTimer) clearInterval(engineTimer);
});

watch(
  () => route.query.path,
  (path) => {
    if (!isFoldersTab.value) return;
    if (typeof path === "string" && path !== cwd.value) {
      void loadDir(path);
    }
  },
);

watch(cwd, (path) => {
  if (!path || isMountRootPath(path)) return;
  const match = appRootsAll.value.find((r) => path === r || path.startsWith(r.replace(/\/+$/, "") + "/"));
  if (match) selectedAppId.value = rootAppId(match);
});

watch(rcloneActive, () => {
  void loadRcloneDrives();
});
</script>

<template>
  <AppShell
    :torrent-count="0"
    :clients-connected="engine?.clientsConnected ?? 0"
    :clients-total="engine?.clientsTotal ?? 0"
    :downloaders-connected="downloadersConnected"
    :client-problems="engine?.problems ?? []"
  >
    <template #sidebar>
      <div class="fleet-files-side">
        <p class="fleet-files-label">
          {{ t("files.apps") }}
        </p>
        <button
          v-for="app in apps"
          :key="app.id"
          type="button"
          class="fleet-files-root"
          :data-active="selectedAppId === app.id ? 'true' : undefined"
          @click="selectApp(app)"
        >
          {{ app.label }}
        </button>
        <p
          v-if="!apps.length"
          class="fleet-muted"
        >
          {{ t("files.noApps") }}
        </p>

        <p class="fleet-files-label fleet-files-label-sub">
          {{ t("files.folders") }}
        </p>
        <template v-if="selectedAppRoots.length">
          <button
            v-for="r in selectedAppRoots"
            :key="r"
            type="button"
            class="fleet-files-root fleet-files-location"
            :data-active="cwd === r ? 'true' : undefined"
            :title="r"
            @click="setTab(FILES_TAB_FOLDERS); loadDir(r)"
          >
            {{ rootFolderName(r) }}
          </button>
        </template>
        <p
          v-else
          class="fleet-muted"
        >
          {{ t("files.noApps") }}
        </p>

        <template v-if="mountEntries.length">
          <p class="fleet-files-label fleet-files-label-sub">
            {{ t("files.mounts") }}
          </p>
          <button
            v-for="m in mountEntries"
            :key="m.id"
            type="button"
            class="fleet-files-root fleet-files-location"
            :data-active="m.path && cwd === m.path ? 'true' : undefined"
            :data-disabled="m.browsable ? undefined : 'true'"
            :title="m.title"
            @click="openMount(m)"
          >
            {{ m.label }}
          </button>
        </template>
      </div>
    </template>

    <div class="fleet-files">
      <template v-if="isFoldersTab">
        <header class="fleet-files-toolbar">
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || (!cwd && !parentPath)"
            @click="loadDir(parentPath)"
          >
            {{ t("files.up") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="loadDir(cwd || '')"
          >
            {{ t("files.refresh") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !checkedCount"
            @click="bulkOp('move')"
          >
            {{ t("files.move") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !checkedCount"
            @click="bulkOp('copy')"
          >
            {{ t("files.copy") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !checkedCount"
            @click="bulkOp('remove')"
          >
            {{ t("files.delete") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !selected || selected.isDir || !isMediaInfoName(selected.name)"
            @click="runMediaInfo"
          >
            {{ t("files.mediainfo") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !selected || selected.isDir"
            @click="openShareForm"
          >
            {{ t("files.share") }}
          </button>
          <a
            v-if="selected && !selected.isDir"
            class="fleet-btn"
            :href="fsContentURL(selected.path)"
            download
          >{{ t("files.download") }}</a>
          <code class="fleet-files-cwd">{{ cwd || t("files.rootsHint") }}{{ checkedCount ? ` · ${checkedCount}` : "" }}</code>
        </header>

        <div
          v-if="shareOpen && selected && !selected.isDir"
          class="fleet-files-share-form"
        >
          <p class="fleet-muted">
            Share {{ selected.name }}
          </p>
          <label>
            TTL seconds (default 24h)
            <input
              v-model.number="shareTtlSec"
              class="fleet-control"
              type="number"
              min="60"
            >
          </label>
          <label>
            Max downloads (0=unlimited)
            <input
              v-model.number="shareMaxDownloads"
              class="fleet-control"
              type="number"
              min="0"
            >
          </label>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="shareSelected"
          >
            Create share
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="shareOpen = false"
          >
            Cancel
          </button>
        </div>
      </template>

      <p
        v-if="error"
        class="fleet-error"
      >
        {{ error }}
      </p>
      <p
        v-if="notice"
        class="fleet-notice"
        :data-kind="noticeKind"
      >
        {{ notice }}
      </p>

      <div
        v-if="isFoldersTab"
        class="fleet-files-grid"
      >
        <div class="fleet-files-table-wrap">
          <table class="fleet-files-table">
            <thead>
              <tr>
                <th class="fleet-files-check">
                  <input
                    type="checkbox"
                    :checked="entries.length > 0 && entries.every((e) => checked[e.path])"
                    @change="toggleCheckAll"
                  >
                </th>
                <th>{{ t("files.colName") }}</th>
                <th>{{ t("files.colSize") }}</th>
                <th>{{ t("files.colModified") }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="e in entries"
                :key="e.path"
                :class="{ selected: selected?.path === e.path }"
                @click="openEntry(e)"
              >
                <td
                  class="fleet-files-check"
                  @click.stop
                >
                  <input
                    type="checkbox"
                    :checked="!!checked[e.path]"
                    @change="toggleCheck(e, $event)"
                  >
                </td>
                <td
                  class="fleet-files-name"
                  :title="e.path"
                >
                  <ToolbarIcon
                    class="fleet-files-type-icon"
                    :name="e.isDir ? 'folder' : 'file'"
                  />
                  <span>{{ entryDisplayName(e) }}</span>
                </td>
                <td>{{ e.isDir ? "—" : e.size }}</td>
                <td>{{ e.modTime ? new Date(e.modTime * 1000).toLocaleString() : "—" }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <aside
          v-if="previewKind !== 'none'"
          class="fleet-files-preview"
        >
          <p
            v-if="previewEncoding"
            class="fleet-muted"
          >
            encoding={{ previewEncoding }}
          </p>
          <pre v-if="previewKind === 'text'">{{ previewText }}</pre>
          <img
            v-else-if="previewKind === 'image' && selected"
            :src="fsContentURL(selected.path)"
            :alt="selected.name"
            class="fleet-files-media"
          >
          <video
            v-else-if="previewKind === 'video' && selected"
            class="fleet-files-media"
            controls
            :src="fsContentURL(selected.path)"
          />
        </aside>
      </div>

      <DownloadQueuePanel
        v-else-if="activeProvider"
        :key="activeProvider.extensionId"
        :extension-id="activeProvider.extensionId"
        :list-method="activeProvider.listMethod"
        :control-method="activeProvider.controlMethod"
        :actions="activeProvider.actions"
        @notice="onQueueNotice"
      />

      <section
        v-if="isFoldersTab && shares.length"
        class="fleet-files-shares"
      >
        <h3>Active shares</h3>
        <ul>
          <li
            v-for="s in shares"
            :key="s.token"
          >
            <code>{{ s.name }}</code>
            →
            <a :href="withBase(`/api/v1/share/${s.token}`)">{{ withBase(`/api/v1/share/${s.token.slice(0, 8)}`) }}…</a>
            <span class="fleet-muted">expires {{ formatExpiry(s) }} · downloads {{ s.downloads }}{{ s.maxDownloads ? `/${s.maxDownloads}` : "" }}</span>
            <button
              type="button"
              class="fleet-btn"
              @click="revokeShare(s.token)"
            >
              Revoke
            </button>
          </li>
        </ul>
      </section>
    </div>

    <Modal
      v-if="destPickerOpen"
      :title="destPickerOp === 'move' ? t('files.moveTo') : t('files.copyTo')"
      wide
      @close="destPickerOpen = false"
    >
      <p class="fleet-muted">
        {{ t("files.pickDestination") }}
      </p>
      <PathPicker
        v-model="destPath"
        directories-only
        panel-only
        guarded
      />
      <div class="fleet-files-dest-actions">
        <button
          type="button"
          class="fleet-btn-primary"
          :disabled="busy || !destPath.trim()"
          @click="confirmDestPicker"
        >
          {{ destPickerOp === "move" ? t("files.move") : t("files.copy") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="destPickerOpen = false"
        >
          {{ t("files.cancel") }}
        </button>
      </div>
    </Modal>
  </AppShell>
</template>

<style scoped>
.fleet-files {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  height: 100%;
  min-height: 0;
  padding: 0.75rem 1rem;
}
.fleet-files-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}
.fleet-files-cwd {
  margin-left: auto;
  font-size: 0.8rem;
  opacity: 0.85;
}
.fleet-files-share-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: end;
  padding: 0.5rem 0;
}
.fleet-files-share-form label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.8rem;
}
.fleet-files-share-form input {
  min-width: 8rem;
}
.fleet-files-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 0.8fr);
  gap: 0.75rem;
  min-height: 0;
  flex: 1;
  align-items: start;
}
.fleet-files-grid:not(:has(.fleet-files-preview)) {
  grid-template-columns: 1fr;
}
.fleet-files-table-wrap {
  overflow: auto;
  min-height: 0;
  max-height: 100%;
  align-self: start;
  width: 100%;
}
.fleet-files-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
  height: max-content;
}
.fleet-files-table th,
.fleet-files-table td {
  text-align: left;
  padding: 0.35rem 0.5rem;
  border-bottom: 1px solid var(--fleet-border-soft);
  vertical-align: middle;
}
.fleet-files-name {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}
.fleet-files-name span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fleet-files-type-icon {
  flex-shrink: 0;
  opacity: 0.85;
  width: 1rem;
  height: 1rem;
}
.fleet-files-table tbody tr {
  height: auto;
}
.fleet-files-check {
  width: 2rem;
}
.fleet-files-table tr {
  cursor: pointer;
}
.fleet-files-table tr.selected,
.fleet-files-table tr:hover {
  background: var(--fleet-surface-2);
}
.fleet-files-preview {
  overflow: auto;
  background: var(--fleet-surface);
  border: 1px solid var(--fleet-border);
  padding: 0.75rem;
  align-self: stretch;
  min-height: 0;
}
.fleet-files-preview pre {
  margin: 0;
  white-space: pre-wrap;
  font-family: var(--fleet-font-mono);
  font-size: 0.8rem;
}
.fleet-files-media {
  max-width: 100%;
  max-height: 70vh;
  display: block;
}
.fleet-files-side {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.75rem;
}
.fleet-files-label {
  margin: 0;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--fleet-fg-muted);
}
.fleet-files-label-sub {
  margin-top: 0.75rem;
}
.fleet-files-root {
  text-align: left;
  background: transparent;
  border: 0;
  color: inherit;
  padding: 0.35rem 0.25rem;
  cursor: pointer;
  font-size: 0.8rem;
  word-break: break-all;
  border-radius: 0.25rem;
}
.fleet-files-root:hover {
  color: var(--fleet-primary-text);
}
.fleet-files-root[data-active="true"] {
  background: var(--fleet-surface-2, color-mix(in srgb, currentColor 8%, transparent));
  color: var(--fleet-primary-text, inherit);
}
.fleet-files-location {
  padding-left: 0.5rem;
  opacity: 0.9;
}
.fleet-files-root[data-disabled="true"] {
  opacity: 0.45;
}
.fleet-files-dest-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.75rem;
}
.fleet-files-shares ul {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.fleet-files-shares li {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  font-size: 0.85rem;
}
.fleet-muted {
  color: var(--fleet-fg-muted);
  font-size: 0.85rem;
}
.fleet-error {
  color: var(--fleet-destructive-text);
}
.fleet-notice {
  color: var(--fleet-accent-text);
  white-space: pre-wrap;
}
.fleet-notice[data-kind="error"] {
  color: var(--fleet-destructive-text);
}
@media (max-width: 900px) {
  .fleet-files-grid {
    grid-template-columns: 1fr;
  }
}
</style>
