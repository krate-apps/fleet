<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { extensionRPC } from "@/api/client";
import type { QueueControlAction } from "@/extensions/downloadQueues";
import { useI18n } from "@/i18n/useI18n";

export type QueueItem = {
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

const props = withDefaults(
  defineProps<{
    extensionId: string;
    listMethod: string;
    controlMethod: string;
    actions?: QueueControlAction[];
    /** When false, skip auto-poll until credentials exist (parent can still call refresh). */
    autoPoll?: boolean;
  }>(),
  {
    actions: () => ["pause", "resume", "remove"],
    autoPoll: true,
  },
);

const emit = defineEmits<{
  notice: [msg: string, kind: "ok" | "error"];
  loaded: [count: number];
}>();

const { t } = useI18n();
const busy = ref(false);
const items = ref<QueueItem[]>([]);
const error = ref("");
let pollTimer: ReturnType<typeof setInterval> | undefined;

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

function actionLabel(action: QueueControlAction): string {
  switch (action) {
    case "pause":
      return t("files.queue.pause");
    case "resume":
      return t("files.queue.resume");
    case "remove":
      return t("files.queue.remove");
    case "abort":
      return t("files.queue.abort");
    case "restart":
      return t("files.queue.restart");
    case "delete":
      return t("files.queue.delete");
    case "stop":
      return t("files.queue.stop");
    case "force":
      return t("files.queue.force");
    default:
      return action;
  }
}

async function refreshQueue(silent = false) {
  if (!silent) busy.value = true;
  error.value = "";
  try {
    const res = await extensionRPC(props.extensionId, props.listMethod, {});
    const r = (res.result ?? {}) as { items?: QueueItem[] };
    items.value = r.items ?? [];
    emit("loaded", items.value.length);
    if (!silent) emit("notice", t("files.queue.loaded", { n: items.value.length }), "ok");
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    error.value = msg;
    items.value = [];
    if (!silent) emit("notice", msg, "error");
  } finally {
    if (!silent) busy.value = false;
  }
}

async function control(action: QueueControlAction, id: string) {
  busy.value = true;
  try {
    await extensionRPC(props.extensionId, props.controlMethod, { action, id });
    emit("notice", t("files.queue.controlOk", { action }), "ok");
    await refreshQueue(true);
  } catch (e) {
    emit("notice", e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

function startPoll() {
  stopPoll();
  if (!props.autoPoll) return;
  pollTimer = setInterval(() => {
    if (!busy.value) void refreshQueue(true);
  }, 4000);
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = undefined;
  }
}

defineExpose({ refreshQueue });

onMounted(async () => {
  await refreshQueue(true);
  startPoll();
});

onUnmounted(() => stopPoll());

watch(
  () => [props.extensionId, props.listMethod, props.autoPoll] as const,
  async () => {
    await refreshQueue(true);
    startPoll();
  },
);
</script>

<template>
  <div class="dq-panel">
    <div class="dq-panel__toolbar">
      <button
        type="button"
        class="fleet-btn"
        :disabled="busy"
        @click="refreshQueue(false)"
      >
        {{ t("files.queue.refresh") }}
      </button>
    </div>
    <p
      v-if="error"
      class="dq-panel__error"
    >
      {{ error }}
    </p>
    <p
      v-else-if="!items.length"
      class="dq-panel__empty"
    >
      {{ t("files.queue.empty") }}
    </p>
    <div
      v-else
      class="dq-panel__wrap"
    >
      <table class="dq-panel__table">
        <thead>
          <tr>
            <th>{{ t("files.queue.colName") }}</th>
            <th>{{ t("files.queue.colStatus") }}</th>
            <th>{{ t("files.queue.colProgress") }}</th>
            <th>{{ t("files.queue.colSpeed") }}</th>
            <th>{{ t("files.queue.colEta") }}</th>
            <th>{{ t("files.queue.colActions") }}</th>
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
              <div class="dq-panel__prog">
                <div
                  class="dq-panel__prog-bar"
                  :style="{ width: progress(it) + '%' }"
                />
              </div>
              <span class="dq-panel__prog-label">
                {{ formatBytes(it.bytesDone) }} / {{ formatBytes(it.bytesTotal) }}
              </span>
            </td>
            <td>{{ formatBytes(it.speed) }}/s</td>
            <td>{{ formatEta(it.etaSec) }}</td>
            <td class="dq-panel__actions">
              <button
                v-for="action in actions"
                :key="action"
                type="button"
                class="fleet-btn fleet-btn-sm"
                :disabled="busy"
                @click="control(action, it.id)"
              >
                {{ actionLabel(action) }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.dq-panel {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  min-height: 0;
  flex: 1;
}
.dq-panel__toolbar {
  display: flex;
  gap: 0.5rem;
}
.dq-panel__error {
  margin: 0;
  color: var(--fleet-destructive-text, #f87171);
  font-size: 0.9rem;
}
.dq-panel__empty {
  margin: 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.dq-panel__wrap {
  overflow: auto;
  min-height: 0;
  flex: 1;
  align-self: stretch;
}
.dq-panel__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
  height: auto;
}
.dq-panel__table th,
.dq-panel__table td {
  text-align: left;
  padding: 0.35rem 0.5rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  vertical-align: top;
}
.dq-panel__table tbody tr {
  height: auto;
}
.dq-panel__prog {
  height: 0.35rem;
  background: color-mix(in srgb, currentColor 12%, transparent);
  border-radius: 999px;
  overflow: hidden;
  margin-bottom: 0.2rem;
  min-width: 6rem;
}
.dq-panel__prog-bar {
  height: 100%;
  background: var(--fleet-accent, #3b82f6);
}
.dq-panel__prog-label {
  font-size: 0.75rem;
  opacity: 0.75;
}
.dq-panel__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
