<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.pyload";
const { t } = useI18n();

type QueueItem = {
  id: string;
  name: string;
  status: string;
  bytesDone: number;
  bytesTotal: number;
  speed: number;
  etaSec: number;
  kind: string;
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const baseUrl = ref("http://127.0.0.1:8000");
const username = ref("");
const password = ref("");
const passwordConfigured = ref(false);
const items = ref<QueueItem[]>([]);
let pollTimer: ReturnType<typeof setInterval> | undefined;

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n < 0) return "—";
  const u = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${u[i]}`;
}

function formatEta(sec: number): string {
  if (!sec || sec <= 0) return "—";
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function progress(it: QueueItem): number {
  if (!it.bytesTotal) return 0;
  return Math.min(100, Math.round((100 * it.bytesDone) / it.bytesTotal));
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.pyload.getSettings", {});
  const r = (res.result ?? {}) as {
    baseUrl?: string;
    username?: string;
    passwordConfigured?: boolean;
  };
  baseUrl.value = r.baseUrl || "http://127.0.0.1:8000";
  username.value = r.username || "";
  passwordConfigured.value = !!r.passwordConfigured;
  password.value = "";
}

async function saveSettings() {
  busy.value = true;
  try {
    const payload: Record<string, unknown> = {
      enabled: true,
      baseUrl: baseUrl.value.trim(),
      username: username.value.trim(),
    };
    if (password.value) payload.password = password.value;
    await extensionRPC(EXT, "ext.pyload.saveSettings", payload);
    setNotice(t("pyload.saved"));
    password.value = "";
    await loadSettings();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function testConnection() {
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.pyload.testConnection", {});
    setNotice(t("pyload.testOk"));
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function refreshQueue(silent = false) {
  if (!silent) busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.pyload.listQueue", {});
    const r = (res.result ?? {}) as { items?: QueueItem[] };
    items.value = r.items ?? [];
    if (!silent) setNotice(t("pyload.queueLoaded", { n: items.value.length }));
  } catch (e) {
    if (!silent) setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    if (!silent) busy.value = false;
  }
}

async function control(action: string, id: string) {
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.pyload.control", { action, id });
    setNotice(t("pyload.controlOk", { action }));
    await refreshQueue(true);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

onMounted(async () => {
  busy.value = true;
  try {
    await loadSettings();
    if (passwordConfigured.value) await refreshQueue(true);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
  pollTimer = setInterval(() => {
    if (passwordConfigured.value && !busy.value) void refreshQueue(true);
  }, 4000);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<template>
  <div class="py-cfg">
    <header class="py-cfg__head">
      <h2>{{ t("pyload.title") }}</h2>
      <p class="py-cfg__hint">
        {{ t("pyload.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="py-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="py-cfg__card">
      <h3>{{ t("pyload.settings") }}</h3>
      <label class="py-cfg__row">
        {{ t("pyload.baseUrl") }}
        <input
          v-model="baseUrl"
          class="fleet-control"
          type="url"
          :disabled="busy"
        >
        <span class="py-cfg__help">{{ t("pyload.baseUrlHelp") }}</span>
      </label>
      <label class="py-cfg__row">
        {{ t("pyload.username") }}
        <input
          v-model="username"
          class="fleet-control"
          type="text"
          autocomplete="username"
          :disabled="busy"
        >
      </label>
      <label class="py-cfg__row">
        {{ t("pyload.password") }}
        <input
          v-model="password"
          class="fleet-control"
          type="password"
          autocomplete="current-password"
          :placeholder="passwordConfigured ? t('pyload.passwordKeep') : ''"
          :disabled="busy"
        >
      </label>
      <div class="py-cfg__actions">
        <button
          type="button"
          class="fleet-btn fleet-btn-primary"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("pyload.save") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="testConnection"
        >
          {{ t("pyload.test") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="refreshQueue(false)"
        >
          {{ t("pyload.refresh") }}
        </button>
      </div>
    </section>

    <section class="py-cfg__card">
      <h3>{{ t("pyload.queue") }}</h3>
      <p
        v-if="!items.length"
        class="py-cfg__empty"
      >
        {{ t("pyload.queueEmpty") }}
      </p>
      <div
        v-else
        class="py-cfg__table-wrap"
      >
        <table class="py-cfg__table">
          <thead>
            <tr>
              <th>{{ t("pyload.colName") }}</th>
              <th>{{ t("pyload.colStatus") }}</th>
              <th>{{ t("pyload.colProgress") }}</th>
              <th>{{ t("pyload.colSpeed") }}</th>
              <th>{{ t("pyload.colEta") }}</th>
              <th>{{ t("pyload.colActions") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="it in items"
              :key="it.id"
            >
              <td>{{ it.name || it.id }}</td>
              <td>{{ it.status }}</td>
              <td>
                <div class="py-cfg__prog">
                  <div
                    class="py-cfg__prog-bar"
                    :style="{ width: progress(it) + '%' }"
                  />
                </div>
                <span class="py-cfg__prog-label">
                  {{ formatBytes(it.bytesDone) }} / {{ formatBytes(it.bytesTotal) }}
                </span>
              </td>
              <td>{{ formatBytes(it.speed) }}/s</td>
              <td>{{ formatEta(it.etaSec) }}</td>
              <td class="py-cfg__row-actions">
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('pause', it.id)"
                >
                  {{ t("pyload.pause") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('resume', it.id)"
                >
                  {{ t("pyload.resume") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('abort', it.id)"
                >
                  {{ t("pyload.abort") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('restart', it.id)"
                >
                  {{ t("pyload.restart") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('delete', it.id)"
                >
                  {{ t("pyload.delete") }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<style scoped>
.py-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 56rem;
}
.py-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.py-cfg__hint {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.py-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--fleet-accent, #3b82f6) 18%, transparent);
}
.py-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #ef4444 22%, transparent);
}
.py-cfg__card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.py-cfg__card h3 {
  margin: 0;
  font-size: 1rem;
}
.py-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
}
.py-cfg__help {
  font-size: 0.8rem;
  opacity: 0.7;
}
.py-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.py-cfg__empty {
  margin: 0;
  font-size: 0.85rem;
  opacity: 0.8;
}
.py-cfg__table-wrap {
  overflow-x: auto;
}
.py-cfg__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
.py-cfg__table th,
.py-cfg__table td {
  text-align: left;
  padding: 0.4rem 0.5rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  vertical-align: top;
}
.py-cfg__prog {
  height: 0.35rem;
  background: color-mix(in srgb, currentColor 12%, transparent);
  border-radius: 999px;
  overflow: hidden;
  margin-bottom: 0.2rem;
  min-width: 6rem;
}
.py-cfg__prog-bar {
  height: 100%;
  background: var(--fleet-accent, #3b82f6);
}
.py-cfg__prog-label {
  font-size: 0.75rem;
  opacity: 0.75;
}
.py-cfg__row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
