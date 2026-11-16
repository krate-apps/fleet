<script setup lang="ts">
import { onMounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.labelbridge";
const { t } = useI18n();

type PairStatus = {
  from: string;
  to: string;
  enabled: boolean;
  fromKind?: string;
  toKind?: string;
  lastSyncAt?: string;
  lastApplied?: number;
  lastSkipped?: number;
  lastErrors?: number;
  lastConflicts?: number;
  lastError?: string;
  transmissionSkip?: boolean;
};

type SyncStatus = {
  lastScanAt?: string;
  direction?: string;
  primaryClientId?: string;
  applied?: number;
  skipped?: number;
  errors?: number;
  hashes?: number;
  pairs?: PairStatus[];
  recentConflicts?: { hash?: string; clientId?: string; label?: string; reason?: string }[];
  recentErrors?: { hash?: string; clientId?: string; error?: string }[];
  note?: string;
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const intervalSec = ref(10);
const direction = ref("primaryToOthers");
const primaryClientId = ref("local-rtorrent");
const pairsText = ref("");
const status = ref<SyncStatus>({});
const logLines = ref("");

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.labelbridge.getSettings", {});
  const r = (res.result ?? {}) as {
    intervalSec?: number;
    direction?: string;
    primaryClientId?: string;
    pairsText?: string;
  };
  intervalSec.value = typeof r.intervalSec === "number" ? r.intervalSec : 10;
  direction.value = r.direction || "primaryToOthers";
  primaryClientId.value = r.primaryClientId || "local-rtorrent";
  pairsText.value = r.pairsText || "";
}

async function loadStatus() {
  const res = await extensionRPC(EXT, "ext.labelbridge.getStatus", {});
  status.value = (res.result ?? {}) as SyncStatus;
}

async function saveSettings() {
  busy.value = true;
  try {
    await extensionRPC(EXT, "ext.labelbridge.saveSettings", {
      enabled: true,
      intervalSec: intervalSec.value,
      direction: direction.value,
      primaryClientId: primaryClientId.value,
      pairsText: pairsText.value,
    });
    setNotice(t("labelsync.saved"));
    await loadStatus();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function scanNow() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.labelbridge.scan", {});
    const r = (res.result ?? {}) as {
      applied?: number;
      skipped?: number;
      errors?: number;
      status?: SyncStatus;
    };
    if (r.status) status.value = r.status;
    else await loadStatus();
    setNotice(
      t("labelsync.scanDone", {
        a: r.applied ?? 0,
        s: r.skipped ?? 0,
        e: r.errors ?? 0,
      }),
    );
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function loadLog() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.labelbridge.listLog", {});
    const entries = ((res.result as { entries?: Record<string, unknown>[] })?.entries ?? []) as Record<
      string,
      unknown
    >[];
    logLines.value = [
      "time\thash\tclientId\tlabel\treason\tskipped\terror",
      ...entries.slice(0, 80).map((e) =>
        [e.time, e.hash, e.clientId, e.label, e.reason, e.skipped ? "yes" : "", e.error ?? ""].join("\t"),
      ),
    ].join("\n");
    setNotice(t("labelsync.logLoaded", { n: entries.length }));
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
    await loadStatus();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
});
</script>

<template>
  <div class="ls-cfg">
    <header class="ls-cfg__head">
      <h2>{{ t("labelsync.title") }}</h2>
      <p class="ls-cfg__hint">
        {{ t("labelsync.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="ls-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="ls-cfg__card">
      <h3>{{ t("labelsync.settings") }}</h3>
      <label class="ls-cfg__row">
        {{ t("labelsync.interval") }}
        <input
          v-model.number="intervalSec"
          class="fleet-control"
          type="number"
          min="5"
          max="3600"
          :disabled="busy"
        >
      </label>
      <label class="ls-cfg__row">
        {{ t("labelsync.direction") }}
        <select
          v-model="direction"
          class="fleet-control"
          :disabled="busy"
        >
          <option value="primaryToOthers">
            {{ t("labelsync.dirPrimary") }}
          </option>
          <option value="bidirectional">
            {{ t("labelsync.dirBi") }}
          </option>
        </select>
      </label>
      <label class="ls-cfg__row">
        {{ t("labelsync.primary") }}
        <input
          v-model="primaryClientId"
          class="fleet-control"
          type="text"
          :disabled="busy"
        >
      </label>
      <label class="ls-cfg__row">
        {{ t("labelsync.pairs") }}
        <textarea
          v-model="pairsText"
          class="fleet-control"
          rows="6"
          :disabled="busy"
        />
      </label>
      <p class="ls-cfg__hint">
        {{ t("labelsync.pairsHelp") }}
      </p>
      <div class="ls-cfg__actions">
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="saveSettings"
        >
          {{ t("labelsync.save") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="scanNow"
        >
          {{ t("labelsync.scan") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="loadStatus"
        >
          {{ t("labelsync.refreshStatus") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="loadLog"
        >
          {{ t("labelsync.showLog") }}
        </button>
      </div>
    </section>

    <section class="ls-cfg__card">
      <h3>{{ t("labelsync.status") }}</h3>
      <p class="ls-cfg__meta">
        <span v-if="status.lastScanAt">{{ t("labelsync.lastScan") }}: {{ status.lastScanAt }}</span>
        <span v-if="status.direction"> · {{ status.direction }}</span>
        <span v-if="status.primaryClientId"> · primary {{ status.primaryClientId }}</span>
        <span>
          · {{ t("labelsync.counters", { a: status.applied ?? 0, s: status.skipped ?? 0, e: status.errors ?? 0 }) }}
        </span>
      </p>
      <p
        v-if="status.note"
        class="ls-cfg__hint"
      >
        {{ status.note }}
      </p>
      <div class="ls-cfg__table-wrap">
        <table class="ls-cfg__table">
          <thead>
            <tr>
              <th>{{ t("labelsync.colPair") }}</th>
              <th>{{ t("labelsync.colKinds") }}</th>
              <th>{{ t("labelsync.colSync") }}</th>
              <th>{{ t("labelsync.colApplied") }}</th>
              <th>{{ t("labelsync.colSkipped") }}</th>
              <th>{{ t("labelsync.colErrors") }}</th>
              <th>{{ t("labelsync.colConflicts") }}</th>
              <th>{{ t("labelsync.colError") }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!(status.pairs && status.pairs.length)">
              <td colspan="8">
                {{ t("labelsync.noPairs") }}
              </td>
            </tr>
            <tr
              v-for="p in status.pairs || []"
              :key="p.from + '->' + p.to"
            >
              <td>
                <span :data-off="!p.enabled">{{ p.from }} → {{ p.to }}</span>
                <span
                  v-if="p.transmissionSkip"
                  class="ls-cfg__badge"
                >TX skip</span>
              </td>
              <td>{{ p.fromKind || "?" }} / {{ p.toKind || "?" }}</td>
              <td>{{ p.lastSyncAt || "—" }}</td>
              <td>{{ p.lastApplied ?? 0 }}</td>
              <td>{{ p.lastSkipped ?? 0 }}</td>
              <td>{{ p.lastErrors ?? 0 }}</td>
              <td>{{ p.lastConflicts ?? 0 }}</td>
              <td class="ls-cfg__err">
                {{ p.lastError || "" }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="status.recentConflicts?.length"
        class="ls-cfg__sub"
      >
        <h4>{{ t("labelsync.conflicts") }}</h4>
        <ul>
          <li
            v-for="(c, i) in status.recentConflicts"
            :key="i"
          >
            {{ c.hash }} @ {{ c.clientId }} → {{ c.label }} ({{ c.reason }})
          </li>
        </ul>
      </div>
      <div
        v-if="status.recentErrors?.length"
        class="ls-cfg__sub"
      >
        <h4>{{ t("labelsync.errors") }}</h4>
        <ul>
          <li
            v-for="(c, i) in status.recentErrors"
            :key="i"
          >
            {{ c.hash }} @ {{ c.clientId }}: {{ c.error }}
          </li>
        </ul>
      </div>
    </section>

    <section
      v-if="logLines"
      class="ls-cfg__card"
    >
      <h3>{{ t("labelsync.log") }}</h3>
      <pre class="ls-cfg__log">{{ logLines }}</pre>
    </section>
  </div>
</template>

<style scoped>
.ls-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0.25rem 0 1.5rem;
  max-width: 64rem;
}
.ls-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.ls-cfg__hint {
  margin: 0.35rem 0 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.ls-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--fleet-accent, #3d7) 18%, transparent);
}
.ls-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #c44 22%, transparent);
}
.ls-cfg__card {
  border: 1px solid color-mix(in srgb, currentColor 16%, transparent);
  border-radius: 6px;
  padding: 0.75rem 1rem;
}
.ls-cfg__card h3 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.ls-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-bottom: 0.55rem;
}
.ls-cfg__row input[type="text"],
.ls-cfg__row input[type="number"],
.ls-cfg__row select,
.ls-cfg__row textarea {
  width: 100%;
  max-width: 36rem;
}
.ls-cfg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin-top: 0.5rem;
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
.ls-cfg__meta {
  margin: 0 0 0.5rem;
  font-size: 0.9rem;
}
.ls-cfg__table-wrap {
  overflow: auto;
  max-height: 18rem;
}
.ls-cfg__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}
.ls-cfg__table th,
.ls-cfg__table td {
  text-align: left;
  padding: 0.3rem 0.45rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  vertical-align: top;
}
.ls-cfg__badge {
  margin-left: 0.35rem;
  font-size: 0.75rem;
  opacity: 0.75;
}
.ls-cfg__err {
  color: #c44;
  font-size: 0.8rem;
}
.ls-cfg__sub {
  margin-top: 0.75rem;
}
.ls-cfg__sub h4 {
  margin: 0 0 0.35rem;
  font-size: 0.95rem;
}
.ls-cfg__sub ul {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.88rem;
}
.ls-cfg__log {
  margin: 0;
  padding: 0.5rem;
  max-height: 14rem;
  overflow: auto;
  font-size: 0.75rem;
  background: color-mix(in srgb, currentColor 6%, transparent);
  border-radius: 4px;
}
[data-off="true"] {
  opacity: 0.45;
  text-decoration: line-through;
}
</style>
