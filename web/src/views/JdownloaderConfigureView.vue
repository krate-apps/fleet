<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.jdownloader";
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
  enabled?: boolean;
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const email = ref("");
const password = ref("");
const passwordConfigured = ref(false);
const deviceName = ref("");
const items = ref<QueueItem[]>([]);
const devices = ref<string[]>([]);
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
  const res = await extensionRPC(EXT, "ext.jdownloader.getSettings", {});
  const r = (res.result ?? {}) as {
    email?: string;
    deviceName?: string;
    passwordConfigured?: boolean;
  };
  email.value = r.email || "";
  deviceName.value = r.deviceName || "";
  passwordConfigured.value = !!r.passwordConfigured;
  password.value = "";
}

async function saveSettings() {
  busy.value = true;
  try {
    const payload: Record<string, unknown> = {
      enabled: true,
      email: email.value.trim(),
      deviceName: deviceName.value.trim(),
    };
    if (password.value) payload.password = password.value;
    await extensionRPC(EXT, "ext.jdownloader.saveSettings", payload);
    setNotice(t("jdownloader.saved"));
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
    const res = await extensionRPC(EXT, "ext.jdownloader.testConnection", {});
    const r = (res.result ?? {}) as { devices?: string[]; deviceId?: string };
    devices.value = r.devices ?? [];
    setNotice(t("jdownloader.testOk", { n: devices.value.length }));
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function refreshQueue(silent = false) {
  if (!silent) busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.jdownloader.listQueue", {});
    const r = (res.result ?? {}) as { items?: QueueItem[] };
    items.value = r.items ?? [];
    if (!silent) setNotice(t("jdownloader.queueLoaded", { n: items.value.length }));
  } catch (e) {
    if (!silent) setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    if (!silent) busy.value = false;
  }
}

async function control(action: string, id: string) {
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.jdownloader.control", { action, id });
    setNotice(t("jdownloader.controlOk", { action }));
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
    if (passwordConfigured.value) {
      await refreshQueue(true);
    }
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
  <div class="jd-cfg">
    <header class="jd-cfg__head">
      <h2>{{ t("jdownloader.title") }}</h2>
      <p class="jd-cfg__hint">
        {{ t("jdownloader.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="jd-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="jd-cfg__card">
      <h3>{{ t("jdownloader.settings") }}</h3>
      <label class="jd-cfg__row">
        {{ t("jdownloader.email") }}
        <input
          v-model="email"
          class="fleet-control"
          type="email"
          autocomplete="username"
          :disabled="busy"
        >
      </label>
      <label class="jd-cfg__row">
        {{ t("jdownloader.password") }}
        <input
          v-model="password"
          class="fleet-control"
          type="password"
          autocomplete="current-password"
          :placeholder="passwordConfigured ? t('jdownloader.passwordKeep') : ''"
          :disabled="busy"
        >
      </label>
      <label class="jd-cfg__row">
        {{ t("jdownloader.device") }}
        <input
          v-model="deviceName"
          class="fleet-control"
          type="text"
          :placeholder="t('jdownloader.devicePlaceholder')"
          :disabled="busy"
        >
      </label>
      <p
        v-if="devices.length"
        class="jd-cfg__devices"
      >
        {{ t("jdownloader.devices") }}: {{ devices.join(", ") }}
      </p>
      <div class="jd-cfg__actions">
        <button
          type="button"
          class="fleet-btn fleet-btn-primary"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("jdownloader.save") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="testConnection"
        >
          {{ t("jdownloader.test") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="refreshQueue(false)"
        >
          {{ t("jdownloader.refresh") }}
        </button>
      </div>
    </section>

    <section class="jd-cfg__card">
      <h3>{{ t("jdownloader.queue") }}</h3>
      <p
        v-if="!items.length"
        class="jd-cfg__empty"
      >
        {{ t("jdownloader.queueEmpty") }}
      </p>
      <div
        v-else
        class="jd-cfg__table-wrap"
      >
        <table class="jd-cfg__table">
          <thead>
            <tr>
              <th>{{ t("jdownloader.colName") }}</th>
              <th>{{ t("jdownloader.colStatus") }}</th>
              <th>{{ t("jdownloader.colProgress") }}</th>
              <th>{{ t("jdownloader.colSpeed") }}</th>
              <th>{{ t("jdownloader.colEta") }}</th>
              <th>{{ t("jdownloader.colActions") }}</th>
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
                <div class="jd-cfg__prog">
                  <div
                    class="jd-cfg__prog-bar"
                    :style="{ width: progress(it) + '%' }"
                  />
                </div>
                <span class="jd-cfg__prog-label">
                  {{ formatBytes(it.bytesDone) }} / {{ formatBytes(it.bytesTotal) }}
                </span>
              </td>
              <td>{{ formatBytes(it.speed) }}/s</td>
              <td>{{ formatEta(it.etaSec) }}</td>
              <td class="jd-cfg__row-actions">
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('pause', it.id)"
                >
                  {{ t("jdownloader.pause") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('resume', it.id)"
                >
                  {{ t("jdownloader.resume") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn fleet-btn-sm"
                  :disabled="busy"
                  @click="control('remove', it.id)"
                >
                  {{ t("jdownloader.remove") }}
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
.jd-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 56rem;
}
.jd-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.jd-cfg__hint {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.jd-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 0.35rem;
  background: color-mix(in srgb, var(--fleet-accent, #3b82f6) 18%, transparent);
}
.jd-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #ef4444 22%, transparent);
}
.jd-cfg__card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.jd-cfg__card h3 {
  margin: 0;
  font-size: 1rem;
}
.jd-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.9rem;
}
.jd-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.jd-cfg__devices,
.jd-cfg__empty {
  margin: 0;
  font-size: 0.85rem;
  opacity: 0.8;
}
.jd-cfg__table-wrap {
  overflow-x: auto;
}
.jd-cfg__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}
.jd-cfg__table th,
.jd-cfg__table td {
  text-align: left;
  padding: 0.4rem 0.5rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  vertical-align: top;
}
.jd-cfg__prog {
  height: 0.35rem;
  background: color-mix(in srgb, currentColor 12%, transparent);
  border-radius: 999px;
  overflow: hidden;
  margin-bottom: 0.2rem;
  min-width: 6rem;
}
.jd-cfg__prog-bar {
  height: 100%;
  background: var(--fleet-accent, #3b82f6);
}
.jd-cfg__prog-label {
  font-size: 0.75rem;
  opacity: 0.75;
}
.jd-cfg__row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
