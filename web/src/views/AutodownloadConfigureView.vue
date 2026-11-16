<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const { t } = useI18n();
const EXT = "org.fleet.autodownload";

type IrcServer = {
  id: string;
  host: string;
  port: number;
  tls: boolean;
  nick: string;
  password?: string;
  nickServPassword?: string;
  channels: string[];
  enabled: boolean;
};

type Filter = {
  id: string;
  name: string;
  enabled: boolean;
  titleInclude: string;
  titleExclude: string;
  minSizeMB: number;
  maxSizeMB: number;
  resolution: string;
  source: string;
  group: string;
  label: string;
  channels: string[];
  serverIds: string[];
};

type Grab = {
  ts?: string;
  title?: string;
  ok?: boolean;
  dryRun?: boolean;
  source?: string;
  filterId?: string;
  serverId?: string;
  channel?: string;
  error?: string;
  infohash?: string;
};

type IrcStatusRow = {
  id?: string;
  host?: string;
  enabled?: boolean;
  ok?: boolean;
  detail?: string;
  lastAnnounceAt?: string;
  lastAnnounceTitle?: string;
  lastAnnounceChannel?: string;
  lastError?: string;
};

type GrabQuery = {
  serverId: string;
  channel: string;
  filterId: string;
  source: string;
  result: string;
  query: string;
  limit: number;
};

type TestFilterForm = {
  filterId: string;
  title: string;
  channel: string;
  serverId: string;
  sizeMB: number;
};

type TestFilterResult = {
  matched?: boolean;
  reason?: string;
  filterId?: string;
  filterName?: string;
  filters?: Array<{ filterId?: string; filterName?: string; matched?: boolean; reason?: string }>;
};

type Stats24h = {
  grabs?: { total?: number; ok?: number; fail?: number; dryRun?: number; irc?: number; watch?: number };
  announces?: { announces?: number; rejects?: number; grabs?: number };
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const showAdvanced = ref(false);

const global = reactive({
  enabled: false,
  paused: false,
  dryRun: false,
  intervalSec: 30,
  start: true,
  removeAfterAdd: true,
  grabRetention: 500,
});

const dirs = ref<string[]>([]);
const newDir = ref("");
const servers = ref<IrcServer[]>([]);
const filters = ref<Filter[]>([]);
const grabs = ref<Grab[]>([]);
const ircStatus = ref<IrcStatusRow[]>([]);
const stats = ref<Stats24h>({});
const grabQuery = reactive<GrabQuery>({
  serverId: "",
  channel: "",
  filterId: "",
  source: "",
  result: "",
  query: "",
  limit: 100,
});
const testForm = reactive<TestFilterForm>({
  filterId: "",
  title: "",
  channel: "",
  serverId: "",
  sizeMB: 0,
});
const testResult = ref<TestFilterResult | null>(null);
const importJSON = ref("");
const exportPreview = ref("");

const editingServer = ref<IrcServer | null>(null);
const editingFilter = ref<Filter | null>(null);
const serverChannelsText = ref("");
const filterChannelsText = ref("");
const filterServerIdsText = ref("");

let statusTimer: ReturnType<typeof setInterval> | undefined;

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

function emptyServer(): IrcServer {
  return {
    id: "",
    host: "",
    port: 6667,
    tls: false,
    nick: "fleet",
    password: "",
    nickServPassword: "",
    channels: [],
    enabled: true,
  };
}

function emptyFilter(): Filter {
  return {
    id: "",
    name: "",
    enabled: true,
    titleInclude: "",
    titleExclude: "",
    minSizeMB: 0,
    maxSizeMB: 0,
    resolution: "",
    source: "",
    group: "",
    label: "",
    channels: [],
    serverIds: [],
  };
}

function splitList(text: string): string[] {
  return text
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.autodownload.getSettings", {});
  const r = (res.result ?? {}) as Record<string, unknown>;
  global.enabled = Boolean(r.enabled);
  global.paused = Boolean(r.paused);
  global.dryRun = Boolean(r.dryRun);
  global.intervalSec = Number(r.intervalSec) || 30;
  global.start = r.start === undefined ? true : Boolean(r.start);
  global.removeAfterAdd = r.removeAfterAdd === undefined ? true : Boolean(r.removeAfterAdd);
  global.grabRetention = Number(r.grabRetention) || 500;
  dirs.value = Array.isArray(r.dirs) ? (r.dirs as string[]).map(String) : [];
  servers.value = Array.isArray(r.ircServers)
    ? (r.ircServers as IrcServer[]).map((s) => ({
        ...emptyServer(),
        ...s,
        channels: Array.isArray(s.channels) ? s.channels.map(String) : [],
        port: Number(s.port) || 6667,
      }))
    : [];
  filters.value = Array.isArray(r.filters)
    ? (r.filters as Filter[]).map((f) => ({
        ...emptyFilter(),
        ...f,
        channels: Array.isArray(f.channels) ? f.channels.map(String) : [],
        serverIds: Array.isArray(f.serverIds) ? f.serverIds.map(String) : [],
        minSizeMB: Number(f.minSizeMB) || 0,
        maxSizeMB: Number(f.maxSizeMB) || 0,
      }))
    : [];
}

async function loadGrabs() {
  const res = await extensionRPC(EXT, "ext.autodownload.listGrabs", {
    limit: grabQuery.limit || 100,
    serverId: grabQuery.serverId.trim() || undefined,
    channel: grabQuery.channel.trim() || undefined,
    filterId: grabQuery.filterId.trim() || undefined,
    source: grabQuery.source.trim() || undefined,
    result: grabQuery.result.trim() || undefined,
    query: grabQuery.query.trim() || undefined,
  });
  grabs.value = ((res.result as { grabs?: Grab[] })?.grabs) ?? [];
}

async function loadIrcStatus() {
  try {
    const res = await extensionRPC(EXT, "ext.autodownload.ircStatus", {});
    ircStatus.value = ((res.result as { servers?: IrcStatusRow[] })?.servers) ?? [];
  } catch {
    /* status is best-effort */
  }
}

async function loadStats() {
  try {
    const res = await extensionRPC(EXT, "ext.autodownload.stats", {});
    stats.value = (res.result as Stats24h) ?? {};
  } catch {
    stats.value = {};
  }
}

async function refreshAll() {
  busy.value = true;
  notice.value = "";
  try {
    await loadSettings();
    await Promise.all([loadGrabs(), loadIrcStatus(), loadStats()]);
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function saveAll() {
  busy.value = true;
  notice.value = "";
  try {
    const res = await extensionRPC(EXT, "ext.autodownload.saveSettings", {
      // Activation in Extensions list is the only on/off for this worker.
      enabled: true,
      paused: global.paused,
      dryRun: global.dryRun,
      intervalSec: global.intervalSec,
      start: global.start,
      removeAfterAdd: global.removeAfterAdd,
      grabRetention: global.grabRetention,
      dirs: dirs.value,
      ircServers: servers.value,
      filters: filters.value,
    });
    const r = (res.result ?? {}) as Record<string, unknown>;
    const bits = [
      typeof r.dirs === "number" ? `${r.dirs} dirs` : null,
      typeof r.irc === "number" ? `${r.irc} IRC` : null,
      typeof r.filters === "number" ? `${r.filters} filters` : null,
    ].filter(Boolean);
    setNotice(bits.length ? `Saved · ${bits.join(" · ")}` : "Saved");
    await refreshAll();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function runAction(method: string, label: string, params: Record<string, unknown> = {}) {
  busy.value = true;
  notice.value = "";
  try {
    await extensionRPC(EXT, method, params);
    setNotice(label);
    if (method.includes("IRC") || method.includes("irc")) await loadIrcStatus();
    if (method.includes("scan") || method.includes("listGrabs")) await loadGrabs();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

function addDir() {
  const p = newDir.value.trim();
  if (!p) return;
  if (!dirs.value.includes(p)) dirs.value = [...dirs.value, p];
  newDir.value = "";
}

function removeDir(path: string) {
  dirs.value = dirs.value.filter((d) => d !== path);
}

function startEditServer(s?: IrcServer) {
  const copy = s ? { ...s, channels: [...s.channels] } : emptyServer();
  editingServer.value = copy;
  serverChannelsText.value = copy.channels.join(", ");
}

function cancelEditServer() {
  editingServer.value = null;
}

function commitServer() {
  const s = editingServer.value;
  if (!s) return;
  s.id = s.id.trim();
  s.host = s.host.trim();
  if (!s.id || !s.host) {
    setNotice("Server id and host are required", "error");
    return;
  }
  s.channels = splitList(serverChannelsText.value);
  const idx = servers.value.findIndex((x) => x.id === s.id);
  if (idx >= 0) {
    const next = [...servers.value];
    next[idx] = { ...s };
    servers.value = next;
  } else {
    servers.value = [...servers.value, { ...s }];
  }
  editingServer.value = null;
}

function removeServer(id: string) {
  servers.value = servers.value.filter((s) => s.id !== id);
  if (editingServer.value?.id === id) editingServer.value = null;
}

function startEditFilter(f?: Filter) {
  const copy = f
    ? { ...f, channels: [...f.channels], serverIds: [...f.serverIds] }
    : emptyFilter();
  editingFilter.value = copy;
  filterChannelsText.value = copy.channels.join(", ");
  filterServerIdsText.value = copy.serverIds.join(", ");
}

function cancelEditFilter() {
  editingFilter.value = null;
}

function commitFilter() {
  const f = editingFilter.value;
  if (!f) return;
  f.name = f.name.trim();
  f.id = f.id.trim() || f.name.replace(/\s+/g, "_") || `f${filters.value.length + 1}`;
  if (!f.name) {
    setNotice("Filter name is required", "error");
    return;
  }
  f.channels = splitList(filterChannelsText.value);
  f.serverIds = splitList(filterServerIdsText.value);
  const idx = filters.value.findIndex((x) => x.id === f.id);
  if (idx >= 0) {
    const next = [...filters.value];
    next[idx] = { ...f };
    filters.value = next;
  } else {
    filters.value = [...filters.value, { ...f }];
  }
  editingFilter.value = null;
}

function removeFilter(id: string) {
  filters.value = filters.value.filter((f) => f.id !== id);
  if (editingFilter.value?.id === id) editingFilter.value = null;
}

function grabFlag(g: Grab): string {
  if (g.dryRun) return "dry";
  return g.ok ? "ok" : "fail";
}

async function runTestFilter() {
  busy.value = true;
  notice.value = "";
  testResult.value = null;
  try {
    const res = await extensionRPC(EXT, "ext.autodownload.testFilter", {
      filterId: testForm.filterId.trim() || undefined,
      title: testForm.title.trim(),
      channel: testForm.channel.trim() || undefined,
      serverId: testForm.serverId.trim() || undefined,
      sizeMB: testForm.sizeMB || undefined,
    });
    testResult.value = (res.result as TestFilterResult) ?? null;
    const r = testResult.value;
    setNotice(
      r?.matched
        ? `Filter match: ${r.filterName || r.filterId || "yes"}`
        : `No match (${r?.reason || "noMatch"})`,
      r?.matched ? "ok" : "error",
    );
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function doExportConfig() {
  busy.value = true;
  notice.value = "";
  try {
    const res = await extensionRPC(EXT, "ext.autodownload.exportConfig", { includeSecrets: false });
    exportPreview.value = JSON.stringify(res.result ?? {}, null, 2);
    setNotice("Export ready (secrets stripped)");
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function doImportConfig(merge: boolean) {
  const text = importJSON.value.trim();
  if (!text) {
    setNotice("Paste export JSON first", "error");
    return;
  }
  busy.value = true;
  notice.value = "";
  try {
    await extensionRPC(EXT, "ext.autodownload.importConfig", { json: text, merge });
    setNotice(merge ? "Import merged" : "Import replaced servers/filters");
    importJSON.value = "";
    await refreshAll();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function clearGrabHistory() {
  busy.value = true;
  notice.value = "";
  try {
    await extensionRPC(EXT, "ext.autodownload.clearGrabs", {});
    setNotice("Grab history cleared");
    await loadGrabs();
    await loadStats();
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

onMounted(() => {
  void refreshAll();
  statusTimer = setInterval(() => {
    void loadIrcStatus();
    void loadStats();
  }, 15000);
});

onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer);
});
</script>

<template>
  <div class="fleet-autodl">
    <aside class="fleet-autodl-side">
      <header class="fleet-ext-header">
        <p class="fleet-ext-eyebrow">
          AutoDL
        </p>
        <h2>IRC + watch</h2>
        <p class="fleet-ext-note">
          Configure IRC announce filters and watch directories without pipe/JSON DSL.
        </p>
      </header>
      <div class="fleet-ext-actions">
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="refreshAll"
        >
          Reload
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="runAction('ext.autodownload.scan', 'Watch scan started')"
        >
          Scan watch dirs
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="runAction('ext.autodownload.restartIRC', 'IRC restart requested')"
        >
          Restart IRC
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="loadGrabs().then(() => setNotice('Grab history refreshed'))"
        >
          Refresh grabs
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="doExportConfig"
        >
          Export config
        </button>
      </div>
      <div
        v-if="stats.grabs || stats.announces"
        class="fleet-autodl-stats"
      >
        <p class="fleet-ext-note">
          Last 24h
        </p>
        <p>
          Announces {{ stats.announces?.announces ?? 0 }}
          · rejects {{ stats.announces?.rejects ?? 0 }}
          · grabs {{ stats.announces?.grabs ?? 0 }}
        </p>
        <p>
          Journal ok {{ stats.grabs?.ok ?? 0 }}
          · fail {{ stats.grabs?.fail ?? 0 }}
          · dry {{ stats.grabs?.dryRun ?? 0 }}
        </p>
      </div>
    </aside>

    <div class="fleet-autodl-main">
      <div
        v-if="notice"
        class="fleet-banner"
        :data-kind="noticeKind"
      >
        {{ notice }}
      </div>

      <section class="fleet-autodl-section">
        <h3>Global</h3>
        <div class="fleet-autodl-toggles">
          <div class="fleet-autodl-toggles-rest">
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="global.paused"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Paused</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="global.dryRun"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Dry run</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="global.start"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Start after add</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="global.removeAfterAdd"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Remove watched file after add</span>
            </label>
          </div>
        </div>
        <div class="fleet-autodl-grid fleet-autodl-metrics">
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Watch interval (seconds)</span>
            <input
              v-model.number="global.intervalSec"
              class="fleet-decl-control"
              type="number"
              min="5"
              max="3600"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Grab journal retention</span>
            <input
              v-model.number="global.grabRetention"
              class="fleet-decl-control"
              type="number"
              min="1"
              max="5000"
            >
            <span class="fleet-decl-help">Max stored grabs (default 500, max 5000).</span>
          </label>
        </div>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>Watch directories</h3>
        </div>
        <ul class="fleet-autodl-list">
          <li
            v-for="d in dirs"
            :key="d"
          >
            <code>{{ d }}</code>
            <button
              type="button"
              class="fleet-btn"
              @click="removeDir(d)"
            >
              Remove
            </button>
          </li>
          <li
            v-if="!dirs.length"
            class="fleet-ext-note"
          >
            No watch directories.
          </li>
        </ul>
        <div class="fleet-autodl-inline">
          <input
            v-model="newDir"
            class="fleet-decl-control"
            type="text"
            placeholder="/path/under/download_roots"
            @keydown.enter.prevent="addDir"
          >
          <button
            type="button"
            class="fleet-btn"
            @click="addDir"
          >
            Add
          </button>
        </div>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>IRC servers</h3>
          <button
            type="button"
            class="fleet-btn"
            @click="startEditServer()"
          >
            Add server
          </button>
        </div>
        <div
          v-if="ircStatus.length"
          class="fleet-autodl-status"
        >
          <div
            v-for="s in ircStatus"
            :key="s.id || s.host"
            class="fleet-autodl-status-row"
            :data-ok="s.ok ? 'true' : 'false'"
          >
            <strong>{{ s.id || "—" }}</strong>
            <span>{{ s.host }}</span>
            <span>{{ s.enabled === false ? "off" : "on" }} / {{ s.ok ? "up" : "down" }}</span>
            <span class="fleet-ext-note">
              {{ s.detail }}
              <template v-if="s.lastAnnounceAt">
                · last announce {{ s.lastAnnounceAt }}
                <template v-if="s.lastAnnounceChannel"> {{ s.lastAnnounceChannel }}</template>
                <template v-if="s.lastAnnounceTitle"> — {{ s.lastAnnounceTitle }}</template>
              </template>
              <template v-if="s.lastError">
                · last error {{ s.lastError }}
              </template>
            </span>
          </div>
        </div>
        <table class="fleet-autodl-table">
          <thead>
            <tr>
              <th>ID</th>
              <th>Host</th>
              <th>Nick</th>
              <th>Channels</th>
              <th>On</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="s in servers"
              :key="s.id"
            >
              <td>{{ s.id }}</td>
              <td>{{ s.host }}:{{ s.port }}{{ s.tls ? " (tls)" : "" }}</td>
              <td>{{ s.nick }}</td>
              <td>{{ s.channels.join(", ") }}</td>
              <td>{{ s.enabled ? "yes" : "no" }}</td>
              <td class="fleet-autodl-row-actions">
                <button
                  type="button"
                  class="fleet-btn"
                  @click="startEditServer(s)"
                >
                  Edit
                </button>
                <button
                  type="button"
                  class="fleet-btn"
                  @click="removeServer(s.id)"
                >
                  Remove
                </button>
              </td>
            </tr>
            <tr v-if="!servers.length">
              <td
                colspan="6"
                class="fleet-ext-note"
              >
                No IRC servers.
              </td>
            </tr>
          </tbody>
        </table>

        <div
          v-if="editingServer"
          class="fleet-autodl-editor"
        >
          <h4>{{ servers.some((x) => x.id === editingServer!.id) ? "Edit server" : "New server" }}</h4>
          <div class="fleet-autodl-grid">
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">ID</span>
              <input
                v-model="editingServer.id"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Host</span>
              <input
                v-model="editingServer.host"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Port</span>
              <input
                v-model.number="editingServer.port"
                class="fleet-decl-control"
                type="number"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Nick</span>
              <input
                v-model="editingServer.nick"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Password</span>
              <input
                v-model="editingServer.password"
                class="fleet-decl-control"
                type="password"
                autocomplete="off"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">NickServ password</span>
              <input
                v-model="editingServer.nickServPassword"
                class="fleet-decl-control"
                type="password"
                autocomplete="off"
              >
            </label>
            <label
              class="fleet-decl-field"
              data-wide="true"
            >
              <span class="fleet-decl-label">Channels</span>
              <input
                v-model="serverChannelsText"
                class="fleet-decl-control"
                type="text"
                placeholder="#announce, #releases"
              >
              <span class="fleet-decl-help">Comma or newline separated.</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="editingServer.tls"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>TLS</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="editingServer.enabled"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Enabled</span>
            </label>
          </div>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              @click="commitServer"
            >
              Apply server
            </button>
            <button
              type="button"
              class="fleet-btn"
              @click="cancelEditServer"
            >
              Cancel
            </button>
          </div>
        </div>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>Filters</h3>
          <button
            type="button"
            class="fleet-btn"
            @click="startEditFilter()"
          >
            Add filter
          </button>
        </div>
        <table class="fleet-autodl-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Include</th>
              <th>Scope</th>
              <th>On</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="f in filters"
              :key="f.id"
            >
              <td>{{ f.name }}</td>
              <td><code class="fleet-autodl-mono">{{ f.titleInclude || "—" }}</code></td>
              <td>
                {{ f.channels.join(", ") || "any ch" }}
                ·
                {{ f.serverIds.join(", ") || "any srv" }}
              </td>
              <td>{{ f.enabled ? "yes" : "no" }}</td>
              <td class="fleet-autodl-row-actions">
                <button
                  type="button"
                  class="fleet-btn"
                  @click="startEditFilter(f)"
                >
                  Edit
                </button>
                <button
                  type="button"
                  class="fleet-btn"
                  @click="removeFilter(f.id)"
                >
                  Remove
                </button>
              </td>
            </tr>
            <tr v-if="!filters.length">
              <td
                colspan="5"
                class="fleet-ext-note"
              >
                No filters.
              </td>
            </tr>
          </tbody>
        </table>

        <div
          v-if="editingFilter"
          class="fleet-autodl-editor"
        >
          <h4>{{ filters.some((x) => x.id === editingFilter!.id) ? "Edit filter" : "New filter" }}</h4>
          <div class="fleet-autodl-grid">
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">ID</span>
              <input
                v-model="editingFilter.id"
                class="fleet-decl-control"
                type="text"
                placeholder="auto from name"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Name</span>
              <input
                v-model="editingFilter.name"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label
              class="fleet-decl-field"
              data-wide="true"
            >
              <span class="fleet-decl-label">Title include (regex)</span>
              <input
                v-model="editingFilter.titleInclude"
                class="fleet-decl-control fleet-autodl-mono"
                type="text"
              >
            </label>
            <label
              class="fleet-decl-field"
              data-wide="true"
            >
              <span class="fleet-decl-label">Title exclude (regex)</span>
              <input
                v-model="editingFilter.titleExclude"
                class="fleet-decl-control fleet-autodl-mono"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Min size MiB</span>
              <input
                v-model.number="editingFilter.minSizeMB"
                class="fleet-decl-control"
                type="number"
                min="0"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Max size MiB</span>
              <input
                v-model.number="editingFilter.maxSizeMB"
                class="fleet-decl-control"
                type="number"
                min="0"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Resolution</span>
              <input
                v-model="editingFilter.resolution"
                class="fleet-decl-control"
                type="text"
                placeholder="720p"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Source</span>
              <input
                v-model="editingFilter.source"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Group</span>
              <input
                v-model="editingFilter.group"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label class="fleet-decl-field">
              <span class="fleet-decl-label">Label</span>
              <input
                v-model="editingFilter.label"
                class="fleet-decl-control"
                type="text"
              >
            </label>
            <label
              class="fleet-decl-field"
              data-wide="true"
            >
              <span class="fleet-decl-label">Channels</span>
              <input
                v-model="filterChannelsText"
                class="fleet-decl-control"
                type="text"
                placeholder="#announce"
              >
            </label>
            <label
              class="fleet-decl-field"
              data-wide="true"
            >
              <span class="fleet-decl-label">Server IDs</span>
              <input
                v-model="filterServerIdsText"
                class="fleet-decl-control"
                type="text"
                placeholder="main"
              >
              <span class="fleet-decl-help">Empty = any server / channel.</span>
            </label>
            <label
              class="fleet-decl-field"
              data-kind="boolean"
            >
              <input
                v-model="editingFilter.enabled"
                class="fleet-decl-check"
                type="checkbox"
              >
              <span>Enabled</span>
            </label>
          </div>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              @click="commitFilter"
            >
              Apply filter
            </button>
            <button
              type="button"
              class="fleet-btn"
              @click="cancelEditFilter"
            >
              Cancel
            </button>
          </div>
        </div>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>Test filter</h3>
        </div>
        <p class="fleet-ext-note">
          Simulate an announce against one filter (or all) without enqueuing.
        </p>
        <div class="fleet-autodl-grid">
          <label
            class="fleet-decl-field"
            data-wide="true"
          >
            <span class="fleet-decl-label">Title</span>
            <input
              v-model="testForm.title"
              class="fleet-decl-control"
              type="text"
              placeholder="Show.S01E01.720p.WEB-GROUP"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Filter ID (optional)</span>
            <input
              v-model="testForm.filterId"
              class="fleet-decl-control"
              type="text"
              placeholder="all filters if empty"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Server ID</span>
            <input
              v-model="testForm.serverId"
              class="fleet-decl-control"
              type="text"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Channel</span>
            <input
              v-model="testForm.channel"
              class="fleet-decl-control"
              type="text"
              placeholder="#announce"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Size MiB</span>
            <input
              v-model.number="testForm.sizeMB"
              class="fleet-decl-control"
              type="number"
              min="0"
            >
          </label>
        </div>
        <div class="fleet-ext-actions">
          <button
            type="button"
            class="fleet-btn-primary"
            :disabled="busy || !testForm.title.trim()"
            @click="runTestFilter"
          >
            Run test
          </button>
        </div>
        <pre
          v-if="testResult"
          class="fleet-autodl-grabs"
        >{{ JSON.stringify(testResult, null, 2) }}</pre>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>Grab history</h3>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="clearGrabHistory"
          >
            Clear
          </button>
        </div>
        <div class="fleet-autodl-grid fleet-autodl-grab-filters">
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Server</span>
            <input
              v-model="grabQuery.serverId"
              class="fleet-decl-control"
              type="text"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Channel</span>
            <input
              v-model="grabQuery.channel"
              class="fleet-decl-control"
              type="text"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Filter ID</span>
            <input
              v-model="grabQuery.filterId"
              class="fleet-decl-control"
              type="text"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Source</span>
            <select
              v-model="grabQuery.source"
              class="fleet-decl-control"
            >
              <option value="">
                any
              </option>
              <option value="irc">
                irc
              </option>
              <option value="watch">
                watch
              </option>
            </select>
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Result</span>
            <select
              v-model="grabQuery.result"
              class="fleet-decl-control"
            >
              <option value="">
                any
              </option>
              <option value="ok">
                ok
              </option>
              <option value="fail">
                fail
              </option>
              <option value="dry">
                dry
              </option>
            </select>
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">Search</span>
            <input
              v-model="grabQuery.query"
              class="fleet-decl-control"
              type="text"
              placeholder="title or infohash"
            >
          </label>
        </div>
        <div class="fleet-ext-actions">
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="loadGrabs().then(() => setNotice('Grab history refreshed'))"
          >
            Apply filters
          </button>
        </div>
        <pre class="fleet-autodl-grabs">{{ grabs.length
          ? grabs.map((g) => {
            const flag = grabFlag(g);
            const err = g.error ? ` err=${g.error}` : "";
            const scope = [g.serverId, g.channel].filter(Boolean).join("/");
            return `${g.ts || "-"} [${flag}] ${g.source || "?"} ${scope ? `(${scope}) ` : ""}${g.title || g.infohash || "-"} filter=${g.filterId || "-"}${err}`;
          }).join("\n")
          : "No grabs recorded." }}</pre>
      </section>

      <section class="fleet-autodl-section">
        <div class="fleet-autodl-section-head">
          <h3>Export / import</h3>
        </div>
        <p class="fleet-ext-note">
          JSON of IRC servers + filters. Secrets are omitted on export; import keeps existing passwords when blank.
        </p>
        <div class="fleet-ext-actions">
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="doExportConfig"
          >
            Export
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !importJSON.trim()"
            @click="doImportConfig(true)"
          >
            Import (merge)
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy || !importJSON.trim()"
            @click="doImportConfig(false)"
          >
            Import (replace)
          </button>
        </div>
        <label
          class="fleet-decl-field"
          data-wide="true"
        >
          <span class="fleet-decl-label">Import JSON</span>
          <textarea
            v-model="importJSON"
            class="fleet-decl-control fleet-autodl-mono"
            rows="6"
            placeholder="{&quot;schema&quot;:1,&quot;ircServers&quot;:[],&quot;filters&quot;:[]}"
          />
        </label>
        <pre
          v-if="exportPreview"
          class="fleet-autodl-grabs"
        >{{ exportPreview }}</pre>
      </section>

      <section class="fleet-autodl-section">
        <button
          type="button"
          class="fleet-btn"
          @click="showAdvanced = !showAdvanced"
        >
          {{ showAdvanced ? "Hide" : "Show" }} advanced (declarative DSL)
        </button>
        <p
          v-if="showAdvanced"
          class="fleet-ext-note"
        >
          Use Apply below as the primary path. The legacy textarea DSL remains available via the
          declarative forms API for automation; prefer this screen for day-to-day edits.
        </p>
      </section>

      <footer class="fleet-autodl-footer">
        <button
          type="button"
          class="fleet-btn-primary fleet-form-apply"
          :disabled="busy"
          @click="saveAll"
        >
          {{ t("extensions.apply") }}
        </button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.fleet-autodl {
  display: grid;
  grid-template-columns: minmax(13rem, 0.85fr) minmax(0, 1.35fr);
  gap: 1.25rem 1.5rem;
  align-items: start;
}

.fleet-autodl-side {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  padding-right: 0.75rem;
  border-right: 1px solid var(--fleet-border-soft, var(--fleet-border));
  min-width: 0;
}

.fleet-autodl-main {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
  min-width: 0;
  min-height: 0;
}

.fleet-autodl-section h3,
.fleet-autodl-section h4 {
  margin: 0 0 0.55rem;
  font-size: 1rem;
}

.fleet-autodl-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.45rem;
}

.fleet-autodl-section-head h3 {
  margin: 0;
}

.fleet-autodl-toggles {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  margin-bottom: 0.85rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--fleet-border-soft, var(--fleet-border));
}

.fleet-autodl-toggles-rest {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  margin-top: 0.35rem;
  padding-top: 0.35rem;
  border-top: 1px dashed var(--fleet-border-soft, var(--fleet-border));
}

.fleet-autodl-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem 1rem;
  align-items: start;
}

.fleet-autodl-metrics .fleet-decl-field {
  min-width: 0;
}

.fleet-autodl-metrics .fleet-decl-control {
  max-width: 12rem;
}

.fleet-autodl-grid .fleet-decl-field[data-wide="true"],
.fleet-autodl-grid .fleet-decl-field[data-kind="boolean"] {
  grid-column: 1 / -1;
}

.fleet-autodl-footer {
  display: block;
  width: 100%;
  padding-top: 0.85rem;
  margin-top: 0.35rem;
  border-top: 1px solid var(--fleet-border-soft, var(--fleet-border));
}

.fleet-autodl-list {
  list-style: none;
  margin: 0 0 0.65rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.fleet-autodl-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.fleet-autodl-inline {
  display: flex;
  gap: 0.5rem;
}

.fleet-autodl-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.85rem;
}

.fleet-autodl-table th,
.fleet-autodl-table td {
  text-align: left;
  padding: 0.4rem 0.45rem;
  border-bottom: 1px solid var(--fleet-border-soft, var(--fleet-border));
  vertical-align: top;
}

.fleet-autodl-row-actions {
  display: flex;
  gap: 0.35rem;
  white-space: nowrap;
}

.fleet-autodl-editor {
  margin-top: 0.85rem;
  padding: 0.85rem;
  border: 1px solid var(--fleet-border);
  border-radius: var(--fleet-radius);
  background: var(--fleet-surface);
}

.fleet-autodl-status {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 0.65rem;
}

.fleet-autodl-status-row {
  display: grid;
  grid-template-columns: 6rem 1fr auto minmax(0, 1.2fr);
  gap: 0.5rem;
  font-size: 0.82rem;
  padding: 0.35rem 0.45rem;
  border-radius: var(--fleet-radius);
  background: var(--fleet-input);
}

.fleet-autodl-status-row[data-ok="false"] {
  border-left: 3px solid var(--fleet-danger, #b45309);
}

.fleet-autodl-status-row[data-ok="true"] {
  border-left: 3px solid var(--fleet-success, #15803d);
}

.fleet-autodl-mono,
.fleet-autodl-mono.fleet-decl-control {
  font-family: var(--fleet-font-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace);
  font-size: 0.85rem;
}

.fleet-autodl-grabs {
  margin: 0.55rem 0 0;
  padding: 0.75rem 1rem;
  max-height: 16rem;
  overflow: auto;
  white-space: pre-wrap;
  font-family: var(--fleet-font-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace);
  font-size: 0.78rem;
  background: var(--fleet-surface-2, rgba(0, 0, 0, 0.25));
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: 0.35rem;
}

.fleet-autodl-stats {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  font-size: 0.82rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: var(--fleet-radius);
  background: var(--fleet-surface);
}

.fleet-autodl-stats p {
  margin: 0;
}

.fleet-autodl-grab-filters {
  margin-bottom: 0.55rem;
}

@media (max-width: 720px) {
  .fleet-autodl {
    grid-template-columns: 1fr;
  }

  .fleet-autodl-side {
    padding-right: 0;
    padding-bottom: 0.75rem;
    border-right: 0;
    border-bottom: 1px solid var(--fleet-border-soft, var(--fleet-border));
  }

  .fleet-autodl-grid {
    grid-template-columns: 1fr;
  }

  .fleet-autodl-status-row {
    grid-template-columns: 1fr;
  }
}
</style>
