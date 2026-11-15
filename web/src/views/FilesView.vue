<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import {
  createShare,
  deleteShare,
  fetchFSList,
  fetchFSRoots,
  fetchFSText,
  fetchShares,
  fsContentURL,
  fsCopy,
  fsMove,
  fsRemove,
  withBase,
  type FSEntry,
  type ShareLink,
} from "@/api/client";
import AppShell from "@/components/AppShell.vue";

const route = useRoute();

const roots = ref<string[]>([]);
const cwd = ref("");
const entries = ref<FSEntry[]>([]);
const error = ref("");
const busy = ref(false);
const notice = ref("");
const previewText = ref("");
const previewEncoding = ref("");
const previewKind = ref<"none" | "text" | "image" | "video">("none");
const shares = ref<ShareLink[]>([]);
const selected = ref<FSEntry | null>(null);
const checked = ref<Record<string, boolean>>({});
const shareOpen = ref(false);
const shareTtlSec = ref(86400);
const shareMaxDownloads = ref(0);

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

async function loadRoots() {
  const data = await fetchFSRoots();
  roots.value = data.roots ?? [];
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

async function bulkOp(op: "move" | "copy" | "remove") {
  const items = checkedEntries.value;
  if (!items.length) {
    notice.value = "Select one or more items first.";
    return;
  }
  if (op === "remove") {
    const ok = window.confirm(`Delete ${items.length} item(s)? This cannot be undone.`);
    if (!ok) return;
  }
  let destDir = "";
  if (op === "move" || op === "copy") {
    destDir = String(window.prompt(`Destination directory for ${op} (${items.length} item(s))`, cwd.value) || "").trim();
    if (!destDir) return;
  }
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
    } catch (e) {
      failed++;
      errors.push(`${item.name}: ${e instanceof Error ? e.message : String(e)}`);
      notice.value = `${op}: ${done}/${items.length} (failed ${failed})`;
    }
  }
  if (errors.length) {
    notice.value = `${op} done=${done} failed=${failed}\n${errors.slice(0, 5).join("\n")}`;
  } else {
    notice.value = `${op} complete: ${done}/${items.length}`;
  }
  busy.value = false;
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
    shareOpen.value = false;
    await loadShares();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
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

onMounted(async () => {
  try {
    await loadRoots();
    const initial = typeof route.query.path === "string" ? route.query.path : "";
    await loadDir(initial);
    await loadShares();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
});

watch(
  () => route.query.path,
  (path) => {
    if (typeof path === "string" && path !== cwd.value) {
      void loadDir(path);
    }
  },
);
</script>

<template>
  <AppShell :torrent-count="0">
    <template #sidebar>
      <div class="fleet-files-side">
        <p class="fleet-files-label">
          Roots
        </p>
        <button
          v-for="r in roots"
          :key="r"
          type="button"
          class="fleet-files-root"
          @click="loadDir(r)"
        >
          {{ r }}
        </button>
        <p
          v-if="!roots.length"
          class="fleet-muted"
        >
          Configure download_roots to enable the file manager.
        </p>
      </div>
    </template>

    <div class="fleet-files">
      <header class="fleet-files-toolbar">
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || (!cwd && !parentPath)"
          @click="loadDir(parentPath)"
        >
          Up
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="loadDir(cwd || '')"
        >
          Refresh
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !checkedCount"
          @click="bulkOp('move')"
        >
          Move
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !checkedCount"
          @click="bulkOp('copy')"
        >
          Copy
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !checkedCount"
          @click="bulkOp('remove')"
        >
          Delete
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !selected || selected.isDir"
          @click="openShareForm"
        >
          Share
        </button>
        <a
          v-if="selected && !selected.isDir"
          class="fleet-btn"
          :href="fsContentURL(selected.path)"
          download
        >Download</a>
        <code class="fleet-files-cwd">{{ cwd || "(roots)" }}{{ checkedCount ? ` · ${checkedCount} selected` : "" }}</code>
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

      <p
        v-if="error"
        class="fleet-error"
      >
        {{ error }}
      </p>
      <p
        v-if="notice"
        class="fleet-notice"
      >
        {{ notice }}
      </p>

      <div class="fleet-files-grid">
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
              <th>Name</th>
              <th>Size</th>
              <th>Modified</th>
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
              <td>{{ e.isDir ? "[dir] " : "" }}{{ e.name }}</td>
              <td>{{ e.isDir ? "—" : e.size }}</td>
              <td>{{ e.modTime ? new Date(e.modTime * 1000).toLocaleString() : "—" }}</td>
            </tr>
          </tbody>
        </table>

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

      <section
        v-if="shares.length"
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
}
.fleet-files-grid:not(:has(.fleet-files-preview)) {
  grid-template-columns: 1fr;
}
.fleet-files-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}
.fleet-files-table th,
.fleet-files-table td {
  text-align: left;
  padding: 0.4rem 0.5rem;
  border-bottom: 1px solid var(--fleet-border-soft);
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
.fleet-files-root {
  text-align: left;
  background: transparent;
  border: 0;
  color: inherit;
  padding: 0.35rem 0.25rem;
  cursor: pointer;
  font-size: 0.8rem;
  word-break: break-all;
}
.fleet-files-root:hover {
  color: var(--fleet-primary-text);
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
@media (max-width: 900px) {
  .fleet-files-grid {
    grid-template-columns: 1fr;
  }
}
</style>
