<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import {
  applyUpdate,
  authLogout,
  backupNow,
  fetchAuthMe,
  fetchAuthStatus,
  fetchAvailableUpdates,
  fetchCapabilities,
  fetchFeeds,
  fetchFeedsSettings,
  fetchGateways,
  fetchInstalledReleases,
  fetchInstalledThemePacks,
  fetchSystemStatus,
  fetchThemesCatalog,
  fetchUpdates,
  fetchVersion,
  formatBytes,
  installThemeFromBucket,
  prepareUpdate,
  saveFeedsSettings,
  setUpdateChannel,
  switchInstalledRelease,
  withBase,
  type AuthUser,
  type AvailableUpdate,
  type CapabilitiesResponse,
  type ClientProblem,
  type FeedMode,
  type FeedsSettings,
  type GatewayClient,
  type InstalledRelease,
  type SystemDisk,
  type SystemLoad,
  type ThemeCatalogEntry,
  type UpdateStatus,
  type VersionResponse,
} from "@/api/client";
import ClientLogo from "@/components/ClientLogo.vue";
import GatewayClientsPanel from "@/components/GatewayClientsPanel.vue";
import LogsPanel from "@/components/LogsPanel.vue";
import { useShellModals } from "@/composables/useShellModals";
import { useUiDensity, type UiDensity } from "@/composables/useUiDensity";
import { loadInstalledThemes } from "@/theme/loadInstalledThemes";
import { useTheme } from "@/theme/useTheme";
import { useI18n } from "@/i18n/useI18n";
import type { Locale, MessageKey } from "@/i18n/messages";

defineEmits<{
  close: [];
}>();

type SettingsSection =
  | "preferences"
  | "about"
  | "clients"
  | "logs"
  | "feeds"
  | "updates";

const router = useRouter();
const { themeId, themes, setTheme } = useTheme();
const { locale, locales, setLocale, t } = useI18n();
const { density, setDensity } = useUiDensity();
const { closeAll, settingsSection } = useShellModals();

const themeCatalog = ref<ThemeCatalogEntry[]>([]);
const installedThemeKeys = ref<Set<string>>(new Set());
const themeCatalogBusy = ref(false);
const themeCatalogNotice = ref("");
const themeInstallId = ref("");

const availableThemeCatalog = computed(() =>
  themeCatalog.value
    .filter((c) => c.installable)
    .sort((a, b) => a.name.localeCompare(b.name)),
);

function themeAlreadyInstalled(entry: ThemeCatalogEntry): boolean {
  const keys = installedThemeKeys.value;
  if (keys.has(entry.id)) return true;
  const short = entry.id.split(".").pop() || "";
  return Boolean(short && keys.has(short));
}

async function refreshThemeCatalog() {
  try {
    const [cat, packs] = await Promise.all([fetchThemesCatalog(), fetchInstalledThemePacks()]);
    themeCatalog.value = cat.themes ?? [];
    const keys = new Set<string>();
    for (const p of packs.themes ?? []) {
      if (p.id) keys.add(p.id);
      if (p.packageId) keys.add(p.packageId);
    }
    for (const th of themes.value) {
      keys.add(th.id);
    }
    installedThemeKeys.value = keys;
  } catch (e) {
    themeCatalogNotice.value = e instanceof Error ? e.message : String(e);
  }
}

async function installThemeCatalog(entry: ThemeCatalogEntry) {
  themeCatalogBusy.value = true;
  themeInstallId.value = entry.id;
  themeCatalogNotice.value = "";
  try {
    const res = await installThemeFromBucket(entry.bucketId, entry.id);
    await loadInstalledThemes();
    await refreshThemeCatalog();
    if (res.id) {
      setTheme(res.id);
    }
    themeCatalogNotice.value = `${t("settings.themeCatalog.install")}: ${entry.name}`;
  } catch (e) {
    themeCatalogNotice.value =
      e instanceof Error ? e.message : t("settings.themeCatalog.error");
  } finally {
    themeCatalogBusy.value = false;
    themeInstallId.value = "";
  }
}

const initialSection = settingsSection.value as string;
const section = ref<SettingsSection>(
  initialSection === "comingFrom" || !initialSection
    ? "preferences"
    : (initialSection as SettingsSection),
);

const version = ref<VersionResponse | null>(null);
const caps = ref<CapabilitiesResponse | null>(null);
const aboutClients = ref<GatewayClient[]>([]);
const feedModes = ref<FeedMode[]>([]);
const feedsSettings = ref<FeedsSettings>({
  publicHost: "",
  httpPort: 0,
  httpsPort: 0,
  preferHTTPS: false,
  pathPrefix: "",
});
const feedsNotice = ref("");
const user = ref<AuthUser | null>(null);
const authEnabled = ref(false);
const updates = ref<UpdateStatus | null>(null);
const available = ref<AvailableUpdate[]>([]);
const installed = ref<InstalledRelease[]>([]);
const showManual = ref(false);
const updateVersion = ref("");
const updateURL = ref("");
const updateSHA = ref("");
const updateNote = ref("");
const error = ref("");
const busy = ref(false);
const disks = ref<SystemDisk[]>([]);
const load = ref<SystemLoad | null>(null);

const navItems = computed(() => {
  const items: { id: SettingsSection; label: MessageKey }[] = [
    { id: "preferences", label: "settings.nav.preferences" },
    { id: "about", label: "settings.nav.about" },
    { id: "clients", label: "settings.nav.clients" },
    { id: "logs", label: "settings.nav.logs" },
  ];
  if (feedModes.value.length) {
    items.push({ id: "feeds", label: "settings.nav.feeds" });
  }
  items.push({ id: "updates", label: "settings.nav.updates" });
  return items;
});

async function loadUpdates() {
  updates.value = await fetchUpdates();
}

async function loadUpdateLists() {
  try {
    available.value = (await fetchAvailableUpdates()).releases ?? [];
  } catch {
    available.value = [];
  }
  try {
    installed.value = (await fetchInstalledReleases()).releases ?? [];
  } catch {
    installed.value = [];
  }
}

const clientsConnected = computed(
  () => caps.value?.clientsConnected ?? (caps.value?.connected ? 1 : 0),
);
const clientsTotal = computed(
  () => caps.value?.clientsTotal ?? aboutClients.value.length,
);
const clientProblems = computed(() => caps.value?.problems ?? []);

type AboutClientRow = {
  id: string;
  name: string;
  kind: string;
  enabled: boolean;
  isDefault: boolean;
  ok: boolean;
  detail: string;
};

const aboutClientRows = computed((): AboutClientRow[] => {
  const problems = clientProblems.value;
  const problemById = new Map<string, ClientProblem>();
  const problemByName = new Map<string, ClientProblem>();
  for (const p of problems) {
    if (p.id) problemById.set(p.id, p);
    if (p.name) problemByName.set(p.name.toLowerCase(), p);
  }
  const rows: AboutClientRow[] = [];
  const seen = new Set<string>();
  for (const c of aboutClients.value) {
    seen.add(c.id);
    const prob = problemById.get(c.id) || problemByName.get(c.name.toLowerCase());
    const ok = c.enabled ? !prob : false;
    rows.push({
      id: c.id,
      name: c.name,
      kind: c.kind,
      enabled: c.enabled,
      isDefault: Boolean(c.isDefault),
      ok,
      detail: !c.enabled
        ? t("settings.about.clientDisabled")
        : prob?.message
          ? String(prob.message)
          : t("settings.about.clientOk"),
    });
  }
  for (const p of problems) {
    const id = p.id || `problem:${p.name}`;
    if (seen.has(id)) continue;
    rows.push({
      id,
      name: p.name || id,
      kind: p.kind || "unknown",
      enabled: true,
      isDefault: false,
      ok: false,
      detail: p.message || t("settings.about.clientProblem"),
    });
  }
  return rows;
});

function goClientsSection() {
  section.value = "clients";
}

onMounted(async () => {
  try {
    version.value = await fetchVersion();
    caps.value = await fetchCapabilities();
    try {
      aboutClients.value = (await fetchGateways()).clients ?? [];
    } catch {
      aboutClients.value = [];
    }
    try {
      const sys = await fetchSystemStatus();
      disks.value = sys.disks ?? [];
      load.value = sys.load ?? null;
    } catch {
      disks.value = [];
      load.value = null;
    }
    try {
      feedModes.value = (await fetchFeeds()).modes ?? [];
      feedsSettings.value = await fetchFeedsSettings();
    } catch {
      feedModes.value = [];
    }
    const st = await fetchAuthStatus();
    authEnabled.value = st.enabled;
    if (st.enabled && !st.needsSetup) {
      const me = await fetchAuthMe();
      user.value = me.user ?? null;
    }
    await loadUpdates();
    await loadUpdateLists();
    await refreshThemeCatalog();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  }
});

async function logout() {
  busy.value = true;
  try {
    await authLogout();
    closeAll();
    await router.replace({ name: "login" });
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function runBackup() {
  busy.value = true;
  updateNote.value = "";
  error.value = "";
  try {
    const res = await backupNow();
    updateNote.value = t("settings.updates.backupDone", { path: res.path });
    await loadUpdates();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function onChannelChange(ev: Event) {
  const value = (ev.target as HTMLSelectElement).value;
  busy.value = true;
  error.value = "";
  try {
    await setUpdateChannel(value);
    await loadUpdates();
    await loadUpdateLists();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function refreshAvailable() {
  busy.value = true;
  error.value = "";
  try {
    await loadUpdateLists();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function saveFeedsForm() {
  busy.value = true;
  feedsNotice.value = "";
  error.value = "";
  try {
    feedsSettings.value = await saveFeedsSettings({
      publicHost: feedsSettings.value.publicHost.trim(),
      httpPort: Number(feedsSettings.value.httpPort) || 0,
      httpsPort: Number(feedsSettings.value.httpsPort) || 0,
      preferHTTPS: !!feedsSettings.value.preferHTTPS,
      pathPrefix: feedsSettings.value.pathPrefix.trim(),
    });
    feedModes.value = (await fetchFeeds()).modes ?? [];
    feedsNotice.value = t("settings.feeds.saved");
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function runPrepareVersion(rel: AvailableUpdate) {
  if (!rel.version || !rel.url) return;
  busy.value = true;
  updateNote.value = "";
  error.value = "";
  try {
    await prepareUpdate({
      version: rel.version,
      url: rel.url,
      sha256: rel.sha256,
    });
    updateNote.value = t("settings.updates.prepareDone");
    await loadUpdates();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function runPrepare() {
  if (!updateVersion.value.trim()) return;
  busy.value = true;
  updateNote.value = "";
  error.value = "";
  try {
    const body: { version: string; url?: string; sha256?: string } = {
      version: updateVersion.value.trim(),
    };
    if (updateURL.value.trim()) body.url = updateURL.value.trim();
    if (updateSHA.value.trim()) body.sha256 = updateSHA.value.trim();
    await prepareUpdate(body);
    updateNote.value = t("settings.updates.prepareDone");
    await loadUpdates();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function runApply() {
  busy.value = true;
  updateNote.value = "";
  error.value = "";
  try {
    const res = await applyUpdate();
    const helper = res.plan.helperCommand
      ? t("settings.updates.helper", { cmd: res.plan.helperCommand })
      : "";
    updateNote.value = `${res.plan.message}${helper}`;
    await loadUpdates();
    await loadUpdateLists();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}

async function runSwitch(version: string) {
  busy.value = true;
  updateNote.value = "";
  error.value = "";
  try {
    const res = await switchInstalledRelease(version);
    const helper = res.plan.helperCommand
      ? t("settings.updates.helper", { cmd: res.plan.helperCommand })
      : "";
    updateNote.value = `${res.plan.message}${helper}`;
    await loadUpdates();
    await loadUpdateLists();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fleet-settings-panel">
    <nav
      class="fleet-settings-nav"
      :aria-label="t('settings.title')"
    >
      <button
        v-for="item in navItems"
        :key="item.id"
        type="button"
        class="fleet-settings-nav-btn"
        :data-active="section === item.id"
        :aria-current="section === item.id ? 'page' : undefined"
        @click="section = item.id"
      >
        {{ t(item.label) }}
      </button>
    </nav>

    <div class="fleet-settings-content">
      <div
        v-if="error"
        class="fleet-banner"
        data-kind="error"
      >
        {{ error }}
      </div>

      <section
        v-if="section === 'preferences'"
        class="fleet-settings-section"
      >
        <header class="fleet-settings-section-head">
          <h3>{{ t("settings.nav.preferences") }}</h3>
          <p>{{ t("settings.preferences.help") }}</p>
        </header>

        <div
          v-if="authEnabled"
          class="fleet-settings-block"
        >
          <h4>{{ t("settings.account") }}</h4>
          <p class="fleet-ext-note">
            {{ t("settings.signedInAs") }}
            <strong>{{ user?.username || "—" }}</strong>
          </p>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="logout"
          >
            {{ t("settings.signOut") }}
          </button>
        </div>

        <div class="fleet-settings-block">
          <h4>{{ t("settings.appearance") }}</h4>
          <label>
            {{ t("settings.language") }}
            <select
              class="fleet-theme-select"
              :value="locale"
              @change="setLocale(($event.target as HTMLSelectElement).value as Locale)"
            >
              <option
                v-for="opt in locales"
                :key="opt.id"
                :value="opt.id"
              >
                {{ opt.label }}
              </option>
            </select>
          </label>
          <label>
            {{ t("settings.theme") }}
            <select
              class="fleet-theme-select"
              :value="themeId"
              @change="setTheme(($event.target as HTMLSelectElement).value)"
            >
              <option
                v-for="theme in themes"
                :key="theme.id"
                :value="theme.id"
              >
                {{ theme.label }}
              </option>
            </select>
          </label>
          <div class="fleet-theme-catalog">
            <h4>{{ t("settings.themeCatalog") }}</h4>
            <p class="fleet-ext-note">
              {{ t("settings.themeCatalog.help") }}
            </p>
            <p
              v-if="themeCatalogNotice"
              class="fleet-ext-note"
            >
              {{ themeCatalogNotice }}
            </p>
            <p
              v-if="availableThemeCatalog.length === 0"
              class="fleet-ext-note"
            >
              {{ t("settings.themeCatalog.empty") }}
            </p>
            <ul
              v-else
              class="fleet-theme-catalog-list"
            >
              <li
                v-for="entry in availableThemeCatalog"
                :key="entry.origin"
                class="fleet-theme-catalog-row"
              >
                <div>
                  <strong>{{ entry.name }}</strong>
                  <span class="fleet-ext-note">{{ entry.version }}</span>
                  <p
                    v-if="entry.description"
                    class="fleet-ext-note"
                  >
                    {{ entry.description }}
                  </p>
                </div>
                <button
                  v-if="themeAlreadyInstalled(entry)"
                  type="button"
                  class="fleet-btn"
                  disabled
                >
                  {{ t("settings.themeCatalog.installed") }}
                </button>
                <button
                  v-else
                  type="button"
                  class="fleet-btn"
                  :disabled="themeCatalogBusy"
                  @click="installThemeCatalog(entry)"
                >
                  {{
                    themeInstallId === entry.id
                      ? t("settings.themeCatalog.busy")
                      : t("settings.themeCatalog.install")
                  }}
                </button>
              </li>
            </ul>
          </div>
          <label>
            {{ t("settings.density") }}
            <select
              class="fleet-theme-select"
              :value="density"
              @change="setDensity(($event.target as HTMLSelectElement).value as UiDensity)"
            >
              <option value="comfort">
                {{ t("settings.density.comfort") }}
              </option>
              <option value="compact">
                {{ t("settings.density.compact") }}
              </option>
            </select>
          </label>
          <p class="fleet-ext-note">
            {{ t("settings.density.help") }}
          </p>
        </div>
      </section>

      <section
        v-else-if="section === 'about'"
        class="fleet-settings-section"
      >
        <header class="fleet-settings-section-head">
          <h3>{{ t("settings.nav.about") }}</h3>
          <p>{{ t("settings.about.help") }}</p>
        </header>

        <div class="fleet-settings-block">
          <h4>{{ t("settings.fleet") }}</h4>
          <dl class="fleet-settings-dl">
            <div>
              <dt>{{ t("settings.version") }}</dt>
              <dd>{{ version?.version || updates?.currentVersion || "—" }}</dd>
            </div>
            <div>
              <dt>{{ t("settings.commit") }}</dt>
              <dd><code>{{ version?.commit || updates?.currentCommit || "—" }}</code></dd>
            </div>
            <div>
              <dt>{{ t("settings.backup") }}</dt>
              <dd><code>{{ updates?.lastBackup || "—" }}</code></dd>
            </div>
          </dl>
        </div>

        <div class="fleet-settings-block">
          <h4>{{ t("settings.about.clients") }}</h4>
          <dl class="fleet-settings-dl">
            <div>
              <dt>{{ t("settings.status") }}</dt>
              <dd>
                {{
                  clientsConnected > 0
                    ? t("settings.about.clientsConnected", {
                      n: String(clientsConnected),
                      total: String(clientsTotal),
                    })
                    : t("settings.about.clientsNone")
                }}
              </dd>
            </div>
          </dl>
          <ul
            v-if="aboutClientRows.length"
            class="fleet-about-clients"
          >
            <li
              v-for="row in aboutClientRows"
              :key="row.id"
              class="fleet-about-client"
              :data-ok="row.ok ? 'true' : 'false'"
            >
              <ClientLogo
                :client="row.kind"
                :title="row.name"
              />
              <div class="fleet-about-client-meta">
                <div class="fleet-about-client-title">
                  <strong>{{ row.name }}</strong>
                  <span class="fleet-about-client-kind">{{ row.kind }}</span>
                  <span
                    v-if="row.isDefault"
                    class="fleet-about-client-default"
                  >{{ t("settings.clients.defaultBadge") }}</span>
                  <span
                    v-if="!row.enabled"
                    class="fleet-about-client-disabled"
                  >{{ t("settings.about.clientDisabled") }}</span>
                </div>
                <p class="fleet-about-client-detail">
                  {{ row.detail }}
                </p>
              </div>
            </li>
          </ul>
          <p
            v-else
            class="fleet-ext-note"
          >
            {{ t("settings.about.clientsEmpty") }}
          </p>
          <button
            type="button"
            class="fleet-btn"
            @click="goClientsSection"
          >
            {{ t("settings.about.manageClients") }}
          </button>
        </div>

        <div class="fleet-settings-block">
          <h4>{{ t("settings.system") }}</h4>
          <dl class="fleet-settings-dl">
            <div>
              <dt>{{ t("settings.load") }}</dt>
              <dd>
                {{
                  load
                    ? `${load.load1.toFixed(2)} / ${load.load5.toFixed(2)} / ${load.load15.toFixed(2)}`
                    : "—"
                }}
              </dd>
            </div>
            <div
              v-for="d in disks"
              :key="d.path"
            >
              <dt>{{ t("settings.disk") }} {{ d.path }}</dt>
              <dd>
                {{
                  t("settings.diskFree", {
                    free: formatBytes(d.available),
                    total: formatBytes(d.total),
                    pct: d.usedPct.toFixed(0),
                  })
                }}
              </dd>
            </div>
            <div v-if="!disks.length">
              <dt>{{ t("settings.disk") }}</dt>
              <dd>—</dd>
            </div>
          </dl>
        </div>
      </section>

      <section
        v-else-if="section === 'clients'"
        class="fleet-settings-section"
      >
        <GatewayClientsPanel />
      </section>

      <section
        v-else-if="section === 'logs'"
        class="fleet-settings-section"
      >
        <LogsPanel />
      </section>

      <section
        v-else-if="section === 'feeds'"
        class="fleet-settings-section"
      >
        <header class="fleet-settings-section-head">
          <h3>{{ t("settings.nav.feeds") }}</h3>
          <p>{{ t("settings.feeds.help") }}</p>
        </header>

        <form
          class="fleet-settings-feeds-form"
          @submit.prevent="saveFeedsForm"
        >
          <label>
            {{ t("settings.feeds.publicHost") }}
            <input
              v-model="feedsSettings.publicHost"
              class="fleet-control"
              type="text"
              :placeholder="t('settings.feeds.publicHostPlaceholder')"
              autocomplete="off"
            >
          </label>
          <label>
            {{ t("settings.feeds.httpPort") }}
            <input
              v-model.number="feedsSettings.httpPort"
              class="fleet-control"
              type="number"
              min="0"
              max="65535"
            >
          </label>
          <label>
            {{ t("settings.feeds.httpsPort") }}
            <input
              v-model.number="feedsSettings.httpsPort"
              class="fleet-control"
              type="number"
              min="0"
              max="65535"
            >
          </label>
          <label>
            {{ t("settings.feeds.pathPrefix") }}
            <input
              v-model="feedsSettings.pathPrefix"
              class="fleet-control"
              type="text"
              :placeholder="t('settings.feeds.pathPrefixPlaceholder')"
              autocomplete="off"
            >
          </label>
          <label class="fleet-settings-check">
            <input
              v-model="feedsSettings.preferHTTPS"
              type="checkbox"
            >
            <span>{{ t("settings.feeds.preferHTTPS") }}</span>
          </label>
          <button
            type="submit"
            class="fleet-btn"
            :disabled="busy"
          >
            {{ t("settings.feeds.save") }}
          </button>
          <p
            v-if="feedsNotice"
            class="fleet-ext-note"
          >
            {{ feedsNotice }}
          </p>
        </form>

        <ul class="fleet-settings-feed-list">
          <li
            v-for="f in feedModes"
            :key="f.mode"
          >
            <a
              :href="withBase(f.path)"
              target="_blank"
              rel="noopener"
            >{{ f.title }}</a>
            <code>{{ f.url || withBase(f.path) }}</code>
          </li>
        </ul>
      </section>

      <section
        v-else
        class="fleet-settings-section"
      >
        <header class="fleet-settings-section-head">
          <h3>{{ t("settings.nav.updates") }}</h3>
          <p>{{ t("settings.updates.help") }}</p>
        </header>

        <p class="fleet-ext-note">
          {{ t("settings.updates.current", { version: version?.version || updates?.currentVersion || "—" }) }}
          <span v-if="updates?.githubRepo"> · {{ updates.githubRepo }}</span>
        </p>

        <label>
          {{ t("settings.updates.channel") }}
          <select
            class="fleet-control"
            :value="updates?.channel || 'stable'"
            :disabled="busy"
            @change="onChannelChange"
          >
            <option value="stable">
              {{ t("settings.updates.channel.stable") }}
            </option>
            <option value="pre">
              {{ t("settings.updates.channel.pre") }}
            </option>
          </select>
        </label>

        <div class="fleet-updates-block">
          <div class="fleet-updates-block-head">
            <strong>{{ t("settings.updates.available") }}</strong>
            <button
              type="button"
              class="fleet-btn"
              :disabled="busy"
              @click="refreshAvailable"
            >
              {{ t("settings.updates.availableRefresh") }}
            </button>
          </div>
          <ul
            v-if="available.length"
            class="fleet-updates-list"
          >
            <li
              v-for="rel in available"
              :key="rel.version"
            >
              <div class="fleet-updates-meta">
                <code>{{ rel.version }}</code>
                <span
                  v-if="rel.prerelease"
                  class="fleet-updates-badge"
                >{{ t("settings.updates.prerelease") }}</span>
                <span
                  v-if="!rel.sha256"
                  class="fleet-updates-badge muted"
                >{{ t("settings.updates.noSha") }}</span>
              </div>
              <button
                type="button"
                class="fleet-btn-primary"
                :disabled="busy || !rel.url || !rel.sha256"
                @click="runPrepareVersion(rel)"
              >
                {{ t("settings.updates.prepare") }}
              </button>
            </li>
          </ul>
          <p
            v-else
            class="fleet-ext-note"
          >
            {{ t("settings.updates.availableEmpty") }}
          </p>
        </div>

        <div class="fleet-updates-block">
          <strong>{{ t("settings.updates.installed") }}</strong>
          <ul
            v-if="installed.length"
            class="fleet-updates-list"
          >
            <li
              v-for="rel in installed"
              :key="rel.version"
            >
              <div class="fleet-updates-meta">
                <code>{{ rel.version }}</code>
                <span
                  v-if="rel.current"
                  class="fleet-updates-badge"
                >{{ t("settings.updates.currentBadge") }}</span>
              </div>
              <button
                type="button"
                class="fleet-btn"
                :disabled="busy || rel.current || !rel.hasBinary"
                @click="runSwitch(rel.version)"
              >
                {{ t("settings.updates.switch") }}
              </button>
            </li>
          </ul>
          <p
            v-else
            class="fleet-ext-note"
          >
            {{ t("settings.updates.installedEmpty") }}
          </p>
        </div>

        <p
          v-if="updates?.pending"
          class="fleet-ext-note"
        >
          {{ t("settings.updates.pending", { version: updates.pending.version }) }}
        </p>

        <div class="fleet-ext-actions">
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="runBackup"
          >
            {{ t("settings.backup") }}
          </button>
          <button
            type="button"
            class="fleet-btn-primary"
            :disabled="busy || !updates?.pending"
            @click="runApply"
          >
            {{ t("settings.updates.apply") }}
          </button>
          <button
            type="button"
            class="fleet-btn"
            :disabled="busy"
            @click="showManual = !showManual"
          >
            {{ t("settings.updates.manual") }}
          </button>
        </div>

        <template v-if="showManual">
          <label>
            {{ t("settings.version") }}
            <input
              v-model="updateVersion"
              class="fleet-control"
              type="text"
              placeholder="v0.1.12"
            >
          </label>
          <label>
            {{ t("settings.updates.packageUrl") }}
            <input
              v-model="updateURL"
              class="fleet-control"
              type="url"
              placeholder="https://…/fleet-lab-….tgz"
            >
          </label>
          <label>
            {{ t("settings.updates.sha256") }}
            <input
              v-model="updateSHA"
              class="fleet-control"
              type="text"
              placeholder="64 hex characters"
            >
          </label>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              :disabled="busy || !updateVersion.trim()"
              @click="runPrepare"
            >
              {{ t("settings.updates.prepare") }}
            </button>
          </div>
        </template>

        <p
          v-if="updateNote"
          class="fleet-ext-note"
        >
          {{ updateNote }}
        </p>
      </section>
    </div>
  </div>
</template>

<style scoped>
.fleet-settings-panel {
  display: grid;
  grid-template-columns: 10.5rem minmax(0, 1fr);
  gap: 1rem;
  min-height: 22rem;
  align-items: stretch;
}

.fleet-settings-nav {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding-right: 0.75rem;
  border-right: 1px solid var(--fleet-border-soft, var(--fleet-border));
}

.fleet-settings-nav-btn {
  appearance: none;
  border: 0;
  background: transparent;
  color: var(--fleet-fg-muted);
  text-align: left;
  font: inherit;
  font-size: 0.85rem;
  font-weight: 550;
  padding: 0.45rem 0.55rem;
  border-radius: 6px;
  cursor: pointer;
}

.fleet-settings-nav-btn:hover {
  background: color-mix(in srgb, var(--fleet-foreground) 6%, transparent);
  color: var(--fleet-foreground);
}

.fleet-settings-nav-btn[data-active="true"] {
  background: color-mix(in srgb, var(--fleet-accent, #3ecf8e) 18%, transparent);
  color: var(--fleet-foreground);
}

.fleet-settings-content {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.fleet-settings-section {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.fleet-settings-section-head h3 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 650;
}

.fleet-settings-section-head p {
  margin: 0.2rem 0 0;
  font-size: 0.8rem;
  line-height: 1.35;
  color: var(--fleet-fg-muted);
}

.fleet-settings-block {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.fleet-theme-catalog {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin: 0.35rem 0 0.15rem;
}

.fleet-theme-catalog-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-theme-catalog-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.fleet-theme-catalog-row .fleet-ext-note {
  display: inline;
  margin-left: 0.4rem;
}

.fleet-settings-block h4 {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 650;
  color: var(--fleet-fg-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.fleet-settings-dl {
  margin: 0;
  display: grid;
  gap: 0.35rem;
}

.fleet-settings-dl > div {
  display: grid;
  grid-template-columns: 5.5rem 1fr;
  gap: 0.6rem;
  font-size: 0.85rem;
}

.fleet-settings-dl dt {
  color: var(--fleet-fg-muted);
}

.fleet-settings-dl dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.fleet-about-clients {
  list-style: none;
  margin: 0 0 0.75rem;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-about-client {
  display: flex;
  gap: 0.55rem;
  align-items: flex-start;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: 0.4rem;
  background: color-mix(in srgb, var(--fleet-surface-2, rgba(0, 0, 0, 0.2)) 80%, transparent);
}

.fleet-about-client-meta {
  min-width: 0;
  flex: 1;
}

.fleet-about-client-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.5rem;
}

.fleet-about-client-kind,
.fleet-about-client-default,
.fleet-about-client-disabled {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--fleet-fg-muted);
}

.fleet-about-client-default {
  background: color-mix(in srgb, var(--fleet-primary) 28%, transparent);
  color: var(--fleet-primary-text, var(--fleet-fg));
}

.fleet-about-client-detail {
  margin: 0.25rem 0 0;
  font-size: 0.8rem;
  color: var(--fleet-fg-muted);
}

.fleet-about-client[data-ok="false"] .fleet-about-client-detail {
  color: var(--fleet-status-bad, var(--fleet-destructive-text, #b42318));
}

.fleet-settings-feeds-form {
  display: grid;
  gap: 0.55rem;
  margin-bottom: 0.85rem;
  max-width: 28rem;
}

.fleet-settings-feeds-form label {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.85rem;
}

.fleet-settings-feeds-form input[type="text"],
.fleet-settings-feeds-form input[type="number"],
.fleet-settings-feeds-form .fleet-control {
  /* inherits .fleet-control from shell.css */
}

.fleet-settings-feeds-form .fleet-settings-check {
  flex-direction: row;
  align-items: center;
  gap: 0.45rem;
}

.fleet-settings-feed-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-settings-feed-list li {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 0.75rem;
  align-items: baseline;
}

.fleet-settings-feed-list code {
  font-size: 0.8em;
  opacity: 0.85;
}

.fleet-updates-block {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-updates-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.fleet-updates-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  max-height: 12rem;
  overflow: auto;
}

.fleet-updates-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.35rem 0.45rem;
  border-radius: 6px;
  background: color-mix(in srgb, var(--fleet-foreground) 4%, transparent);
}

.fleet-updates-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
  min-width: 0;
}

.fleet-updates-badge {
  font-size: 0.7rem;
  font-weight: 650;
  letter-spacing: 0.03em;
  text-transform: uppercase;
  color: var(--fleet-primary-text, var(--fleet-accent, #3ecf8e));
}

.fleet-updates-badge.muted {
  color: var(--fleet-fg-muted);
}

@media (max-width: 640px) {
  .fleet-settings-panel {
    grid-template-columns: 1fr;
    min-height: 0;
  }

  .fleet-settings-nav {
    flex-direction: row;
    flex-wrap: wrap;
    padding-right: 0;
    padding-bottom: 0.65rem;
    border-right: 0;
    border-bottom: 1px solid var(--fleet-border-soft, var(--fleet-border));
  }
}
</style>
