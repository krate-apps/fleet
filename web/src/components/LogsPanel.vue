<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { fetchLogs, type LogEntry, type LogSource } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const { t } = useI18n();

const sources = ref<LogSource[]>([]);
const entries = ref<LogEntry[]>([]);
const filter = ref("");
const level = ref("");
const busy = ref(false);
const error = ref("");

const chips = computed(() => {
  const out: { id: string; label: string }[] = [{ id: "", label: t("settings.logs.filterAll") }];
  const seen = new Set<string>();
  for (const s of sources.value) {
    const key = s.kind === "fleet" ? "fleet" : s.id;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push({ id: key, label: s.name });
  }
  return out;
});

function sourceClass(kind: string): string {
  switch (kind) {
    case "fleet":
      return "fleet";
    case "rtorrent":
      return "rtorrent";
    case "deluge":
      return "deluge";
    case "qbittorrent":
      return "qbittorrent";
    case "transmission":
      return "transmission";
    default:
      return "other";
  }
}

function shortSource(name: string, kind: string): string {
  if (kind === "fleet") return "Fleet";
  return name.replace(/\s*\([^)]*\)\s*$/, "") || name;
}

function formatTs(ts: string): string {
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return ts;
  return d.toLocaleString(undefined, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function formatMessage(e: LogEntry): string {
  const skip = new Set(["unit", "path"]);
  const parts: string[] = [];
  if (e.attrs) {
    for (const [k, v] of Object.entries(e.attrs)) {
      if (skip.has(k) || v === "") continue;
      parts.push(`${k}=${v}`);
    }
  }
  if (!parts.length) return e.message;
  return `${e.message}  ${parts.join(" ")}`;
}

async function load() {
  busy.value = true;
  error.value = "";
  try {
    const res = await fetchLogs({
      source: filter.value || undefined,
      level: level.value || undefined,
      limit: 400,
    });
    sources.value = res.sources ?? [];
    entries.value = res.entries ?? [];
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="fleet-logs">
    <header class="fleet-settings-section-head">
      <h3>{{ t("settings.nav.logs") }}</h3>
      <p>{{ t("settings.logs.help") }}</p>
    </header>

    <div
      v-if="error"
      class="fleet-banner"
      data-kind="error"
    >
      {{ error }}
    </div>

    <div class="fleet-logs-toolbar">
      <div
        class="fleet-logs-chips"
        role="group"
        :aria-label="t('settings.logs.sources')"
      >
        <button
          v-for="chip in chips"
          :key="chip.id || 'all'"
          type="button"
          class="fleet-logs-chip"
          :data-active="filter === chip.id"
          :disabled="busy"
          @click="filter = chip.id; load()"
        >
          {{ chip.label }}
        </button>
      </div>
      <div class="fleet-logs-actions">
        <select
          v-model="level"
          class="fleet-theme-select"
          :disabled="busy"
          @change="load()"
        >
          <option value="">
            {{ t("settings.logs.levelAll") }}
          </option>
          <option value="debug">
            debug
          </option>
          <option value="info">
            info
          </option>
          <option value="warn">
            warn
          </option>
          <option value="error">
            error
          </option>
        </select>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="load"
        >
          {{ t("settings.logs.refresh") }}
        </button>
      </div>
    </div>

    <ul
      v-if="entries.length"
      class="fleet-logs-list"
    >
      <li
        v-for="(e, i) in entries"
        :key="`${e.ts}-${e.sourceId}-${i}`"
        class="fleet-logs-row"
        :data-level="e.level"
        :title="formatMessage(e)"
      >
        <time class="fleet-logs-ts">{{ formatTs(e.ts) }}</time>
        <span
          class="fleet-logs-level"
          :data-level="e.level"
        >{{ e.level }}</span>
        <span
          class="fleet-logs-source"
          :data-kind="sourceClass(e.sourceKind)"
        >{{ shortSource(e.sourceName, e.sourceKind) }}</span>
        <span class="fleet-logs-msg">{{ formatMessage(e) }}</span>
      </li>
    </ul>
    <p
      v-else
      class="fleet-ext-note"
    >
      {{ busy ? t("settings.logs.loading") : t("settings.logs.empty") }}
    </p>
  </div>
</template>

<style scoped>
.fleet-logs {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-height: 0;
}

.fleet-logs-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  align-items: center;
  justify-content: space-between;
}

.fleet-logs-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.fleet-logs-chip {
  border: 1px solid color-mix(in srgb, var(--fleet-border, #444) 80%, transparent);
  background: transparent;
  color: inherit;
  padding: 0.25rem 0.55rem;
  border-radius: 0.35rem;
  cursor: pointer;
  font-size: 0.85rem;
}

.fleet-logs-chip[data-active="true"] {
  background: color-mix(in srgb, var(--fleet-accent, #3a7) 28%, transparent);
  border-color: var(--fleet-accent, #3a7);
}

.fleet-logs-actions {
  display: flex;
  gap: 0.45rem;
  align-items: center;
}

.fleet-logs-list {
  list-style: none;
  margin: 0;
  padding: 0.15rem 0;
  max-height: min(60vh, 28rem);
  overflow-x: auto;
  overflow-y: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.76rem;
  line-height: 1.35;
  border: 1px solid color-mix(in srgb, var(--fleet-border, #444) 70%, transparent);
  border-radius: 0.4rem;
  background: color-mix(in srgb, #000 22%, transparent);
}

.fleet-logs-row {
  display: grid;
  grid-template-columns: 7.6rem 3.2rem 5.5rem max-content;
  gap: 0.55rem;
  padding: 0.18rem 0.65rem;
  align-items: baseline;
  width: max-content;
  min-width: 100%;
  box-sizing: border-box;
}

.fleet-logs-row:hover {
  background: color-mix(in srgb, var(--fleet-accent, #3a7) 10%, transparent);
}

.fleet-logs-row[data-level="error"] {
  background: color-mix(in srgb, #c44 12%, transparent);
}

.fleet-logs-row[data-level="warn"] {
  background: color-mix(in srgb, #c90 10%, transparent);
}

.fleet-logs-source {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fleet-logs-source[data-kind="fleet"] {
  color: #5b9bd5;
}

.fleet-logs-source[data-kind="rtorrent"] {
  color: #7ec87e;
}

.fleet-logs-source[data-kind="deluge"] {
  color: #c9a0dc;
}

.fleet-logs-source[data-kind="qbittorrent"] {
  color: #e0a060;
}

.fleet-logs-source[data-kind="transmission"] {
  color: #d0c050;
}

.fleet-logs-level {
  text-transform: uppercase;
  letter-spacing: 0.02em;
  opacity: 0.8;
  font-weight: 600;
}

.fleet-logs-level[data-level="error"] {
  color: #f07178;
  opacity: 1;
}

.fleet-logs-level[data-level="warn"] {
  color: #e5c07b;
  opacity: 1;
}

.fleet-logs-level[data-level="debug"] {
  opacity: 0.55;
}

.fleet-logs-ts {
  opacity: 0.55;
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
}

.fleet-logs-msg {
  white-space: pre;
  min-width: max-content;
}

@media (max-width: 720px) {
  .fleet-logs-row {
    grid-template-columns: 5.5rem 2.8rem max-content;
    grid-template-rows: auto auto;
  }

  .fleet-logs-source {
    display: none;
  }

  .fleet-logs-msg {
    grid-column: 1 / -1;
  }
}
</style>
