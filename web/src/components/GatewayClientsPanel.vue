<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import {
  deleteGateway,
  fetchGateways,
  probeGateway,
  probeGatewayDraft,
  saveGateway,
  setDefaultGateway,
  type GatewayCapabilities,
  type GatewayClient,
  type GatewayKind,
  type GatewayProbeResult,
} from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const { t } = useI18n();

function capabilityChips(caps: GatewayCapabilities) {
  const rows: Array<{ key: string; short: string; label: string; on: boolean }> = [
    { key: "list", short: "L", label: t("settings.clients.cap.list"), on: Boolean(caps.list) },
    { key: "add", short: "A", label: t("settings.clients.cap.add"), on: Boolean(caps.add) },
    { key: "label", short: "Lb", label: t("settings.clients.cap.label"), on: Boolean(caps.label) },
    { key: "startStop", short: "S", label: t("settings.clients.cap.startStop"), on: Boolean(caps.startStop) },
    { key: "erase", short: "E", label: t("settings.clients.cap.erase"), on: Boolean(caps.erase) },
    { key: "reannounce", short: "R", label: t("settings.clients.cap.reannounce"), on: Boolean(caps.reannounce) },
    { key: "files", short: "F", label: t("settings.clients.cap.files"), on: Boolean(caps.files) },
    { key: "trackers", short: "T", label: t("settings.clients.cap.trackers"), on: Boolean(caps.trackers) },
    { key: "peers", short: "P", label: t("settings.clients.cap.peers"), on: Boolean(caps.peers) },
    { key: "torrentRates", short: "Bw", label: t("settings.clients.cap.torrentRates"), on: Boolean(caps.torrentRates) },
  ];
  return rows;
}

const clients = ref<GatewayClient[]>([]);
const busy = ref(false);
const error = ref("");
const notice = ref("");
const probeById = ref<Record<string, GatewayProbeResult>>({});
const editingId = ref<string | null>(null);
const formOpen = ref(false);

const draft = reactive({
  kind: "rtorrent" as GatewayKind,
  name: "",
  url: "",
  username: "",
  password: "",
  enabled: true,
  journalUnit: "",
  logPath: "",
});

const kinds: GatewayKind[] = ["rtorrent", "qbittorrent", "transmission", "deluge"];

function kindLabel(kind: GatewayKind | string): string {
  switch (kind) {
    case "rtorrent":
      return t("settings.clients.kind.rtorrent");
    case "qbittorrent":
      return t("settings.clients.kind.qbittorrent");
    case "transmission":
      return t("settings.clients.kind.transmission");
    case "deluge":
      return t("settings.clients.kind.deluge");
    default:
      return String(kind);
  }
}

function urlPlaceholder(kind: GatewayKind): string {
  if (kind === "rtorrent") return "unix:///run/user/…/rtorrent.sock";
  return "http://127.0.0.1:8080";
}

function resetDraft() {
  editingId.value = null;
  formOpen.value = false;
  draft.kind = "rtorrent";
  draft.name = "";
  draft.url = "";
  draft.username = "";
  draft.password = "";
  draft.enabled = true;
  draft.journalUnit = "";
  draft.logPath = "";
}

function openAddForm() {
  editingId.value = null;
  draft.kind = "rtorrent";
  draft.name = "";
  draft.url = "";
  draft.username = "";
  draft.password = "";
  draft.enabled = true;
  draft.journalUnit = "";
  draft.logPath = "";
  formOpen.value = true;
}

function editClient(c: GatewayClient) {
  editingId.value = c.id;
  draft.kind = c.kind;
  draft.name = c.name;
  draft.url = c.url;
  draft.username = c.username;
  draft.password = "";
  draft.enabled = c.enabled;
  draft.journalUnit = c.journalUnit || "";
  draft.logPath = c.logPath || "";
  formOpen.value = true;
}

async function loadClients() {
  clients.value = (await fetchGateways()).clients ?? [];
}

onMounted(async () => {
  try {
    await loadClients();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
});

async function onSave() {
  if (!draft.name.trim() || !draft.url.trim()) return;
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    await saveGateway({
      id: editingId.value ?? undefined,
      kind: draft.kind,
      name: draft.name.trim(),
      url: draft.url.trim(),
      username: draft.username.trim(),
      password: draft.password,
      enabled: draft.enabled,
      journalUnit: draft.journalUnit.trim(),
      logPath: draft.logPath.trim(),
    });
    notice.value = t("settings.clients.saved");
    resetDraft();
    await loadClients();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function onDelete(id: string) {
  if (!window.confirm(t("settings.clients.confirmDelete"))) return;
  busy.value = true;
  error.value = "";
  try {
    await deleteGateway(id);
    if (editingId.value === id) resetDraft();
    delete probeById.value[id];
    await loadClients();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function onSetDefault(id: string) {
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    await setDefaultGateway(id);
    notice.value = t("settings.clients.defaultSet");
    await loadClients();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function onProbeSaved(id: string) {
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    const res = await probeGateway(id);
    probeById.value = { ...probeById.value, [id]: res };
    notice.value = res.ok
      ? t("settings.clients.probeOk", { version: res.version || "—" })
      : t("settings.clients.probeFail", { message: res.message || "error" });
    if (!res.ok) {
      error.value = t("settings.clients.probeFail", { message: res.message || "error" });
      notice.value = "";
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function onProbeDraft() {
  if (!draft.url.trim()) return;
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    const res = await probeGatewayDraft({
      kind: draft.kind,
      url: draft.url.trim(),
      username: draft.username.trim(),
      password: draft.password,
    });
    notice.value = res.ok
      ? t("settings.clients.probeOk", { version: res.version || "—" })
      : t("settings.clients.probeFail", { message: res.message || "error" });
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fleet-gateways">
    <header class="fleet-settings-section-head">
      <h3>{{ t("settings.nav.clients") }}</h3>
      <p>{{ t("settings.clients.help") }}</p>
      <p class="fleet-ext-note">
        {{ t("settings.clients.defaultHelp") }}
      </p>
    </header>

    <div
      v-if="error"
      class="fleet-banner"
      data-kind="error"
    >
      {{ error }}
    </div>
    <div
      v-else-if="notice"
      class="fleet-banner"
      data-kind="ok"
    >
      {{ notice }}
    </div>

    <div class="fleet-settings-block">
      <h4>{{ t("settings.clients.registered") }}</h4>
      <ul
        v-if="clients.length"
        class="fleet-gateway-list"
      >
        <li
          v-for="c in clients"
          :key="c.id"
          class="fleet-gateway-card"
          :data-enabled="c.enabled"
        >
          <div class="fleet-gateway-main">
            <div class="fleet-gateway-title">
              <strong>{{ c.name }}</strong>
              <span class="fleet-gateway-kind">{{ kindLabel(c.kind) }}</span>
              <span
                v-if="c.isDefault"
                class="fleet-gateway-default"
              >{{ t("settings.clients.defaultBadge") }}</span>
              <span
                class="fleet-gateway-state"
                :data-state="c.enabled ? 'on' : 'off'"
              >{{ c.enabled ? t("settings.on") : t("settings.off") }}</span>
            </div>
            <code class="fleet-gateway-url">{{ c.url }}</code>
            <p
              v-if="c.capabilities"
              class="fleet-gateway-caps"
            >
              <span class="fleet-ext-note">{{ t("settings.clients.caps") }}:</span>
              <span
                v-for="chip in capabilityChips(c.capabilities)"
                :key="chip.key"
                class="fleet-gateway-cap"
                :data-on="chip.on ? 'true' : 'false'"
                :title="chip.label"
              >{{ chip.short }}</span>
            </p>
            <p
              v-if="probeById[c.id]"
              class="fleet-gateway-probe"
              :data-ok="probeById[c.id].ok"
            >
              <template v-if="probeById[c.id].ok">
                {{ t("settings.clients.probeOk", { version: probeById[c.id].version || "—" }) }}
              </template>
              <template v-else>
                {{ t("settings.clients.probeFail", { message: probeById[c.id].message || "error" }) }}
              </template>
            </p>
          </div>
          <div class="fleet-gateway-actions">
            <button
              type="button"
              class="fleet-btn"
              :disabled="busy || c.isDefault || !c.enabled"
              @click="onSetDefault(c.id)"
            >
              {{ c.isDefault ? t("settings.clients.defaultBadge") : t("settings.clients.setDefault") }}
            </button>
            <button
              type="button"
              class="fleet-btn"
              :disabled="busy"
              @click="onProbeSaved(c.id)"
            >
              {{ t("settings.clients.probe") }}
            </button>
            <button
              type="button"
              class="fleet-btn"
              :disabled="busy"
              @click="editClient(c)"
            >
              {{ t("settings.clients.edit") }}
            </button>
            <button
              type="button"
              class="fleet-btn"
              :disabled="busy"
              @click="onDelete(c.id)"
            >
              {{ t("settings.clients.delete") }}
            </button>
          </div>
        </li>
      </ul>
      <p
        v-else
        class="fleet-ext-note"
      >
        {{ t("settings.clients.empty") }}
      </p>
    </div>

    <div
      v-if="!formOpen"
      class="fleet-settings-block"
    >
      <button
        type="button"
        class="fleet-btn-primary"
        :disabled="busy"
        @click="openAddForm"
      >
        {{ t("settings.clients.addTitle") }}
      </button>
    </div>

    <div
      v-else
      class="fleet-settings-block"
    >
      <h4>{{ editingId ? t("settings.clients.editTitle") : t("settings.clients.addTitle") }}</h4>
      <label>
        {{ t("settings.clients.kind") }}
        <select
          v-model="draft.kind"
          class="fleet-theme-select"
        >
          <option
            v-for="k in kinds"
            :key="k"
            :value="k"
          >
            {{ kindLabel(k) }}
          </option>
        </select>
      </label>
      <label>
        {{ t("settings.clients.name") }}
        <input
          v-model="draft.name"
          class="fleet-control"
          type="text"
          :placeholder="t('settings.clients.namePlaceholder')"
        >
      </label>
      <label>
        {{ draft.kind === "rtorrent" ? t("settings.clients.address") : t("settings.clients.url") }}
        <input
          v-model="draft.url"
          class="fleet-control"
          :type="draft.kind === 'rtorrent' ? 'text' : 'url'"
          :placeholder="urlPlaceholder(draft.kind)"
        >
      </label>
      <label>
        {{ t("settings.clients.username") }}
        <input
          v-model="draft.username"
          class="fleet-control"
          type="text"
          autocomplete="off"
        >
      </label>
      <label>
        {{ t("settings.clients.password") }}
        <input
          v-model="draft.password"
          class="fleet-control"
          type="password"
          autocomplete="new-password"
          :placeholder="editingId ? t('settings.clients.passwordKeep') : ''"
        >
      </label>
      <label class="fleet-gateway-check">
        <input
          v-model="draft.enabled"
          type="checkbox"
        >
        <span>{{ t("settings.clients.enabled") }}</span>
      </label>
      <p class="fleet-ext-note">
        {{ t("settings.clients.logsHelp") }}
      </p>
      <label>
        {{ t("settings.clients.journalUnit") }}
        <input
          v-model="draft.journalUnit"
          class="fleet-control"
          type="text"
          :placeholder="t('settings.clients.journalUnitPlaceholder')"
          autocomplete="off"
        >
      </label>
      <label>
        {{ t("settings.clients.logPath") }}
        <input
          v-model="draft.logPath"
          class="fleet-control"
          type="text"
          :placeholder="t('settings.clients.logPathPlaceholder')"
          autocomplete="off"
        >
      </label>
      <div class="fleet-ext-actions">
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !draft.url.trim()"
          @click="onProbeDraft"
        >
          {{ t("settings.clients.probe") }}
        </button>
        <button
          type="button"
          class="fleet-btn-primary"
          :disabled="busy || !draft.name.trim() || !draft.url.trim()"
          @click="onSave"
        >
          {{ editingId ? t("settings.clients.update") : t("settings.clients.add") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="resetDraft"
        >
          {{ t("settings.clients.cancel") }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.fleet-gateways {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.fleet-gateway-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-gateway-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--fleet-border);
  border-radius: 8px;
  background: var(--fleet-surface-2);
}

.fleet-gateway-card[data-enabled="false"] {
  opacity: 0.7;
}

.fleet-gateway-main {
  min-width: 0;
  flex: 1;
}

.fleet-gateway-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.55rem;
}

.fleet-gateway-kind,
.fleet-gateway-state,
.fleet-gateway-default {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--fleet-fg-muted);
}

.fleet-gateway-default {
  background: color-mix(in srgb, var(--fleet-primary) 28%, transparent);
  color: var(--fleet-primary-text, var(--fleet-fg));
}

.fleet-gateway-state[data-state="on"] {
  background: color-mix(in srgb, #3ecf8e 22%, transparent);
  color: #3ecf8e;
}

.fleet-gateway-url {
  display: block;
  margin-top: 0.3rem;
  font-size: 0.75rem;
  color: var(--fleet-fg-muted);
  word-break: break-all;
}

.fleet-gateway-caps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  margin: 0.4rem 0 0;
}

.fleet-gateway-cap {
  min-width: 1.35rem;
  text-align: center;
  font-size: 0.68rem;
  font-weight: 650;
  padding: 0.12rem 0.28rem;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--fleet-fg-muted);
  opacity: 0.45;
}

.fleet-gateway-cap[data-on="true"] {
  opacity: 1;
  background: color-mix(in srgb, var(--fleet-primary) 22%, transparent);
  color: var(--fleet-primary-text, var(--fleet-fg));
}

.fleet-gateway-probe {
  margin: 0.35rem 0 0;
  font-size: 0.78rem;
  color: var(--fleet-fg-muted);
}

.fleet-gateway-probe[data-ok="true"] {
  color: var(--fleet-primary-text);
}

.fleet-gateway-probe[data-ok="false"] {
  color: var(--fleet-destructive-text);
}

.fleet-gateway-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: flex-end;
  flex-shrink: 0;
}

.fleet-gateway-check {
  display: flex !important;
  flex-direction: row !important;
  align-items: center;
  gap: 0.5rem;
}

.fleet-settings-block > label {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  margin-bottom: 0.55rem;
  font-size: 0.85rem;
}
</style>
