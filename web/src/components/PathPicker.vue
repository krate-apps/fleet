<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { fetchFSList, fetchFSRoots, type FSEntry } from "@/api/client";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import { useI18n } from "@/i18n/useI18n";
import { groupRootsByApp, pathParts, rootLabel } from "@/lib/downloadRoots";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    /** When true, only directories are selectable / listed. */
    directoriesOnly?: boolean;
    /** Always show the browser panel (for modals); path field is read-only. */
    panelOnly?: boolean;
    /** Hide free-text path editing (path stays under download_roots). */
    guarded?: boolean;
  }>(),
  {
    directoriesOnly: false,
    panelOnly: false,
    guarded: true,
  },
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const { t } = useI18n();
const open = ref(false);
const cwd = ref("");
const roots = ref<string[]>([]);
const entries = ref<FSEntry[]>([]);
const error = ref("");
const busy = ref(false);
/** True while listing download_roots (not a real directory). */
const atRoots = ref(false);

function normalize(path: string): string {
  return path.replace(/\\/g, "/").replace(/\/+$/, "");
}

function isUnderRoot(path: string, root: string): boolean {
  const p = normalize(path);
  const r = normalize(root);
  return p === r || p.startsWith(r + "/");
}

function containingRoot(path: string): string | null {
  const p = normalize(path);
  let best: string | null = null;
  for (const root of roots.value) {
    if (!isUnderRoot(p, root)) continue;
    if (!best || normalize(root).length > normalize(best).length) best = root;
  }
  return best;
}

function filterEntries(list: FSEntry[]): FSEntry[] {
  return list.filter((e) => {
    if (e.name.startsWith(".")) return false;
    if (props.directoriesOnly && !e.isDir) return false;
    return true;
  });
}

function folderLeaf(path: string): string {
  const parts = pathParts(path);
  return parts[parts.length - 1] || path;
}

async function ensureRoots() {
  if (roots.value.length) return;
  const data = await fetchFSRoots();
  roots.value = data.roots ?? [];
}

const rootGroups = computed(() => groupRootsByApp(roots.value));

async function load(path = "") {
  busy.value = true;
  error.value = "";
  try {
    await ensureRoots();
    const trimmed = String(path || "").trim();
    if (!trimmed) {
      atRoots.value = true;
      entries.value = roots.value.map((r) => ({
        name: rootLabel(r),
        path: r,
        isDir: true,
        size: 0,
        modTime: 0,
      }));
      cwd.value = "";
      return;
    }
    // Refuse browsing above / outside download_roots (hide-dotpaths / path UX).
    if (!containingRoot(trimmed)) {
      error.value = t("files.pathOutsideRoots");
      entries.value = [];
      cwd.value = "";
      await load("");
      return;
    }
    atRoots.value = false;
    const data = await fetchFSList(trimmed);
    cwd.value = data.cwd || trimmed;
    entries.value = filterEntries(data.entries ?? []);
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

function select(entry: FSEntry) {
  if (entry.isDir) {
    void load(entry.path);
    emit("update:modelValue", entry.path);
    return;
  }
  emit("update:modelValue", entry.path);
  if (!props.panelOnly) open.value = false;
}

function useCurrent() {
  if (cwd.value) {
    emit("update:modelValue", cwd.value);
  }
  if (!props.panelOnly) open.value = false;
}

function goUp() {
  if (!cwd.value) {
    void load("");
    return;
  }
  const root = containingRoot(cwd.value);
  if (root && normalize(cwd.value) === normalize(root)) {
    void load("");
    return;
  }
  const parts = normalize(cwd.value).split("/");
  parts.pop();
  const parent = parts.join("/") || "";
  if (root && parent && !isUnderRoot(parent, root) && !containingRoot(parent)) {
    void load("");
    return;
  }
  if (parent && containingRoot(parent)) void load(parent);
  else void load("");
}

function onPathInput(raw: string) {
  if (props.guarded) return;
  emit("update:modelValue", raw);
}

function entryLabel(e: FSEntry): string {
  if (atRoots.value) return rootLabel(e.path);
  return e.name;
}

async function openAtModel() {
  await ensureRoots();
  const start = String(props.modelValue || "").trim();
  void load(start && containingRoot(start) ? start : "");
}

watch(open, (v) => {
  if (v) void openAtModel();
});

onMounted(() => {
  if (props.panelOnly) void openAtModel();
});
</script>

<template>
  <div class="fleet-path-picker">
    <div class="fleet-path-picker-row">
      <input
        class="fleet-control"
        :value="modelValue"
        type="text"
        :readonly="guarded || panelOnly"
        :placeholder="t('files.pathPlaceholder')"
        @input="onPathInput(($event.target as HTMLInputElement).value)"
      >
      <button
        v-if="!panelOnly"
        type="button"
        class="fleet-btn"
        @click="open = !open"
      >
        {{ t("files.browse") }}
      </button>
    </div>
    <div
      v-if="panelOnly || open"
      class="fleet-path-picker-panel"
      :data-panel-only="panelOnly ? 'true' : undefined"
    >
      <div class="fleet-path-picker-toolbar">
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="goUp"
        >
          {{ t("files.up") }}
        </button>
        <button
          type="button"
          class="fleet-btn fleet-btn-primary"
          :disabled="busy || !cwd"
          @click="useCurrent"
        >
          {{ t("files.useFolder") }}
        </button>
        <code
          class="fleet-path-picker-cwd"
          :title="cwd || undefined"
        >{{ cwd || t("files.rootsHint") }}</code>
      </div>
      <p
        v-if="error"
        class="fleet-error"
      >
        {{ error }}
      </p>

      <!-- Grouped download_roots: app header + folder leaf (readable vs duplicate basenames). -->
      <template v-if="atRoots">
        <div
          v-for="group in rootGroups"
          :key="group.id"
          class="fleet-path-picker-group"
        >
          <p class="fleet-path-picker-group-label">
            {{ group.label }}
          </p>
          <ul>
            <li
              v-for="r in group.roots"
              :key="r"
            >
              <button
                type="button"
                class="fleet-path-picker-entry"
                :title="r"
                @click="select({ name: folderLeaf(r), path: r, isDir: true, size: 0, modTime: 0 })"
              >
                <ToolbarIcon
                  class="fleet-path-picker-icon"
                  name="folder"
                />
                <span class="fleet-path-picker-name">{{ folderLeaf(r) }}</span>
                <span class="fleet-path-picker-path">{{ r }}</span>
              </button>
            </li>
          </ul>
        </div>
      </template>

      <ul v-else>
        <li
          v-for="e in entries"
          :key="e.path"
        >
          <button
            type="button"
            class="fleet-path-picker-entry fleet-path-picker-entry-simple"
            :title="e.path"
            @click="select(e)"
          >
            <ToolbarIcon
              class="fleet-path-picker-icon"
              :name="e.isDir ? 'folder' : 'file'"
            />
            <span class="fleet-path-picker-name">{{ entryLabel(e) }}</span>
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.fleet-path-picker {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  width: 100%;
}
.fleet-path-picker-entry {
  display: grid;
  grid-template-columns: 1rem minmax(4rem, max-content) minmax(0, 1fr);
  align-items: center;
  gap: 0.4rem;
  max-width: 100%;
}
.fleet-path-picker-entry-simple {
  grid-template-columns: 1rem minmax(0, 1fr);
}
.fleet-path-picker-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.fleet-path-picker-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: var(--fleet-fg-muted);
  opacity: 0.9;
}
.fleet-path-picker-icon {
  flex-shrink: 0;
  width: 1rem;
  height: 1rem;
  opacity: 0.85;
}
.fleet-path-picker-row {
  display: flex;
  gap: 0.35rem;
}
.fleet-path-picker-row input {
  flex: 1;
}
.fleet-path-picker-panel {
  border: 1px solid var(--fleet-border);
  background: var(--fleet-surface);
  padding: 0.5rem;
  max-height: 14rem;
  overflow: auto;
}
.fleet-path-picker-panel[data-panel-only="true"] {
  max-height: min(22rem, 50vh);
}
.fleet-path-picker-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
  margin-bottom: 0.35rem;
  font-size: 0.8rem;
}
.fleet-path-picker-cwd {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.fleet-path-picker-group {
  margin-top: 0.35rem;
}
.fleet-path-picker-group:first-of-type {
  margin-top: 0;
}
.fleet-path-picker-group-label {
  margin: 0 0 0.15rem;
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--fleet-fg-muted);
}
.fleet-path-picker-panel ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
.fleet-path-picker-panel li button {
  width: 100%;
  text-align: left;
  background: transparent;
  border: 0;
  color: inherit;
  padding: 0.3rem 0.15rem;
  cursor: pointer;
  font-size: 0.85rem;
}
.fleet-path-picker-panel li button:hover {
  color: var(--fleet-primary-text);
}
.fleet-path-picker-panel li button:hover .fleet-path-picker-path {
  color: inherit;
  opacity: 0.75;
}
.fleet-error {
  color: var(--fleet-destructive-text);
  font-size: 0.85rem;
}
@media (max-width: 640px) {
  .fleet-path-picker-entry {
    grid-template-columns: 1rem minmax(0, 1fr);
  }
  .fleet-path-picker-path {
    grid-column: 2;
  }
}
</style>
