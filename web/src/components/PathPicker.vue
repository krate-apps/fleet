<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import { fetchFSList, fetchFSRoots, type FSEntry } from "@/api/client";

const props = defineProps<{
  modelValue: string;
  /** When true, only directories are selectable. */
  directoriesOnly?: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const open = ref(false);
const cwd = ref("");
const entries = ref<FSEntry[]>([]);
const error = ref("");
const busy = ref(false);

async function load(path = "") {
  busy.value = true;
  error.value = "";
  try {
    if (!path) {
      const roots = await fetchFSRoots();
      entries.value = (roots.roots ?? []).map((r) => ({
        name: r,
        path: r,
        isDir: true,
        size: 0,
        modTime: 0,
      }));
      cwd.value = "";
      return;
    }
    const data = await fetchFSList(path);
    cwd.value = data.cwd || path;
    entries.value = (data.entries ?? []).filter((e) => (props.directoriesOnly ? e.isDir : true));
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
  open.value = false;
}

function useCurrent() {
  if (cwd.value) {
    emit("update:modelValue", cwd.value);
  }
  open.value = false;
}

function goUp() {
  if (!cwd.value) {
    void load("");
    return;
  }
  const parts = cwd.value.replace(/\\/g, "/").split("/");
  parts.pop();
  const parent = parts.join("/") || "";
  void load(parent);
}

watch(open, (v) => {
  if (v) void load(props.modelValue || "");
});

onMounted(() => {
  // warm roots check only when opened
});
</script>

<template>
  <div class="fleet-path-picker">
    <div class="fleet-path-picker-row">
      <input
        class="fleet-control"
        :value="modelValue"
        type="text"
        placeholder="/path/under/download_roots"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      >
      <button
        type="button"
        class="fleet-btn"
        @click="open = !open"
      >
        Browse
      </button>
    </div>
    <div
      v-if="open"
      class="fleet-path-picker-panel"
    >
      <div class="fleet-path-picker-toolbar">
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="goUp"
        >
          Up
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !cwd"
          @click="useCurrent"
        >
          Use folder
        </button>
        <code>{{ cwd || "(roots)" }}</code>
      </div>
      <p
        v-if="error"
        class="fleet-error"
      >
        {{ error }}
      </p>
      <ul>
        <li
          v-for="e in entries"
          :key="e.path"
        >
          <button
            type="button"
            @click="select(e)"
          >
            {{ e.isDir ? "[dir] " : "" }}{{ e.name }}
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
.fleet-path-picker-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  align-items: center;
  margin-bottom: 0.35rem;
  font-size: 0.8rem;
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
  padding: 0.25rem 0.15rem;
  cursor: pointer;
  font-size: 0.85rem;
}
.fleet-path-picker-panel li button:hover {
  color: var(--fleet-primary-text);
}
.fleet-error {
  color: var(--fleet-destructive-text);
  font-size: 0.85rem;
}
</style>
