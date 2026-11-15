<script setup lang="ts">
import { onMounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.traffic";
const { t } = useI18n();

type Settings = {
  enabled: boolean;
  intervalMin: number;
};

type Series = {
  mode?: string;
  labels?: string[];
  up?: number[];
  down?: number[];
};

type TrackerRow = {
  host: string;
  up: number;
  down: number;
  lastHit?: number;
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const settings = ref<Settings>({ enabled: true, intervalMin: 15 });
const mode = ref<"day" | "week" | "month" | "year">("day");
const series = ref<Series>({});
const trackers = ref<TrackerRow[]>([]);
const exportPreview = ref("");

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

function fmtBytes(n: number | undefined): string {
  const v = typeof n === "number" && Number.isFinite(n) ? n : 0;
  if (v < 1024) return `${v} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let x = v;
  let i = -1;
  do {
    x /= 1024;
    i++;
  } while (x >= 1024 && i < units.length - 1);
  return `${x.toFixed(x >= 10 || i === 0 ? 1 : 2)} ${units[i]}`;
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.traffic.getSettings", {});
  const r = (res.result ?? {}) as Settings;
  settings.value = {
    enabled: !!r.enabled,
    intervalMin: typeof r.intervalMin === "number" ? r.intervalMin : 15,
  };
}

async function saveSettings() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.traffic.saveSettings", {
      enabled: settings.value.enabled,
      intervalMin: settings.value.intervalMin,
    });
    const r = (res.result ?? {}) as Settings;
    settings.value = {
      enabled: !!r.enabled,
      intervalMin: typeof r.intervalMin === "number" ? r.intervalMin : settings.value.intervalMin,
    };
    setNotice("Settings saved");
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function loadSeries(next?: "day" | "week" | "month" | "year") {
  if (next) mode.value = next;
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.traffic.get", { mode: mode.value });
    series.value = (res.result ?? {}) as Series;
    setNotice(`Series · ${mode.value}`);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function loadTrackers() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.traffic.getTrackers", {});
    const r = (res.result ?? {}) as { trackers?: TrackerRow[]; count?: number; note?: string };
    trackers.value = Array.isArray(r.trackers) ? r.trackers : [];
    setNotice(`Trackers · ${trackers.value.length}${r.note ? ` — ${r.note}` : ""}`);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function collectNow() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.traffic.scan", {});
    setNotice(`Collect · ${JSON.stringify(res.result)}`);
    await Promise.all([loadSeries(), loadTrackers()]);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function clearStats() {
  if (!confirm("Clear all traffic stats (global + per-tracker)?")) return;
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.traffic.clear", {});
    series.value = {};
    trackers.value = [];
    exportPreview.value = "";
    setNotice("Stats cleared");
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

function downloadBlob(filename: string, content: string, mime: string) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

async function exportStats(format: "json" | "csv") {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.traffic.export", { format, scope: "all" });
    const r = (res.result ?? {}) as { content?: string; filename?: string };
    const content = typeof r.content === "string" ? r.content : "";
    const filename = typeof r.filename === "string" ? r.filename : `fleet-traffic.${format}`;
    exportPreview.value = content.slice(0, 4000);
    downloadBlob(filename, content, format === "json" ? "application/json" : "text/csv");
    setNotice(`Exported ${filename}`);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

const seriesRows = () => {
  const labels = series.value.labels ?? [];
  const up = series.value.up ?? [];
  const down = series.value.down ?? [];
  const n = Math.max(labels.length, up.length, down.length);
  const rows: { label: string; up: number; down: number }[] = [];
  for (let i = 0; i < n; i++) {
    rows.push({
      label: labels[i] ?? String(i),
      up: Number(up[i] ?? 0),
      down: Number(down[i] ?? 0),
    });
  }
  return rows;
};

onMounted(async () => {
  busy.value = true;
  try {
    await loadSettings();
    await Promise.all([loadSeries("day"), loadTrackers()]);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
});
</script>

<template>
  <div class="trafic-cfg">
    <header class="trafic-cfg__head">
      <h2>{{ t("traffic.title") }}</h2>
      <p class="trafic-cfg__hint">
        {{ t("traffic.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="trafic-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="trafic-cfg__card">
      <h3>{{ t("traffic.settings") }}</h3>
      <label class="trafic-cfg__row">
        <input
          v-model="settings.enabled"
          type="checkbox"
          :disabled="busy"
        >
        {{ t("traffic.enabled") }}
      </label>
      <label class="trafic-cfg__row">
        {{ t("traffic.interval") }}
        <input
          v-model.number="settings.intervalMin"
          class="fleet-control"
          type="number"
          min="1"
          max="60"
          :disabled="busy"
        >
      </label>
      <div class="trafic-cfg__actions">
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("traffic.save") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="collectNow"
        >
          {{ t("traffic.collect") }}
        </button>
        <button
          type="button"
          class="btn btn--danger"
          :disabled="busy"
          @click="clearStats"
        >
          {{ t("traffic.clear") }}
        </button>
      </div>
    </section>

    <section class="trafic-cfg__card">
      <h3>{{ t("traffic.series") }}</h3>
      <div class="trafic-cfg__actions">
        <button
          type="button"
          class="btn"
          :class="{ 'btn--active': mode === 'day' }"
          :disabled="busy"
          @click="loadSeries('day')"
        >
          {{ t("traffic.day") }}
        </button>
        <button
          type="button"
          class="btn"
          :class="{ 'btn--active': mode === 'week' }"
          :disabled="busy"
          @click="loadSeries('week')"
        >
          {{ t("traffic.week") }}
        </button>
        <button
          type="button"
          class="btn"
          :class="{ 'btn--active': mode === 'month' }"
          :disabled="busy"
          @click="loadSeries('month')"
        >
          {{ t("traffic.month") }}
        </button>
        <button
          type="button"
          class="btn"
          :class="{ 'btn--active': mode === 'year' }"
          :disabled="busy"
          @click="loadSeries('year')"
        >
          {{ t("traffic.year") }}
        </button>
      </div>
      <div class="trafic-cfg__table-wrap">
        <table class="trafic-cfg__table">
          <thead>
            <tr>
              <th>{{ t("traffic.bucket") }}</th>
              <th>{{ t("traffic.up") }}</th>
              <th>{{ t("traffic.down") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in seriesRows()"
              :key="row.label"
            >
              <td>{{ row.label }}</td>
              <td>{{ fmtBytes(row.up) }}</td>
              <td>{{ fmtBytes(row.down) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="trafic-cfg__card">
      <h3>{{ t("traffic.trackers") }}</h3>
      <div class="trafic-cfg__actions">
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="loadTrackers"
        >
          {{ t("traffic.refreshTrackers") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="exportStats('json')"
        >
          {{ t("traffic.exportJson") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="exportStats('csv')"
        >
          {{ t("traffic.exportCsv") }}
        </button>
      </div>
      <div class="trafic-cfg__table-wrap">
        <table class="trafic-cfg__table">
          <thead>
            <tr>
              <th>{{ t("traffic.host") }}</th>
              <th>{{ t("traffic.up") }}</th>
              <th>{{ t("traffic.down") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!trackers.length">
              <td colspan="3">
                {{ t("traffic.noTrackers") }}
              </td>
            </tr>
            <tr
              v-for="row in trackers"
              :key="row.host"
            >
              <td>{{ row.host }}</td>
              <td>{{ fmtBytes(row.up) }}</td>
              <td>{{ fmtBytes(row.down) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <pre
        v-if="exportPreview"
        class="trafic-cfg__export"
      >{{ exportPreview }}</pre>
    </section>
  </div>
</template>

<style scoped>
.trafic-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0.25rem 0 1.5rem;
  max-width: 56rem;
}
.trafic-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.trafic-cfg__hint {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.trafic-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--fleet-accent, #3d7) 18%, transparent);
}
.trafic-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #c44 22%, transparent);
}
.trafic-cfg__card {
  border: 1px solid color-mix(in srgb, currentColor 16%, transparent);
  border-radius: 6px;
  padding: 0.75rem 1rem;
}
.trafic-cfg__card h3 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.trafic-cfg__row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
}
.trafic-cfg__row input[type="number"] {
  width: 5rem;
}
.trafic-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-bottom: 0.75rem;
}
.btn {
  border: 1px solid color-mix(in srgb, currentColor 25%, transparent);
  background: transparent;
  color: inherit;
  border-radius: 4px;
  padding: 0.35rem 0.7rem;
  cursor: pointer;
}
.btn:disabled {
  opacity: 0.5;
  cursor: default;
}
.btn--active {
  border-color: var(--fleet-accent, #3d7);
}
.btn--danger {
  border-color: #c44;
}
.trafic-cfg__table-wrap {
  overflow: auto;
  max-height: 18rem;
}
.trafic-cfg__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}
.trafic-cfg__table th,
.trafic-cfg__table td {
  text-align: left;
  padding: 0.3rem 0.45rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
}
.trafic-cfg__export {
  margin: 0.75rem 0 0;
  padding: 0.5rem;
  max-height: 12rem;
  overflow: auto;
  font-size: 0.75rem;
  background: color-mix(in srgb, currentColor 6%, transparent);
  border-radius: 4px;
}
</style>
