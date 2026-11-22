<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  addBucket,
  fetchBucketCatalog,
  fetchBuckets,
  installFromArchive,
  installFromBucket,
  installFromURL,
  reloadExtensionWorkers,
  setBucketEnabled,
  uninstallExtension,
  type BucketRecord,
  type CatalogEntry,
} from "@/api/client";
import ExtensionConfigBody from "@/components/ExtensionConfigBody.vue";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import { useExtensionUI } from "@/composables/useExtensionUI";
import { useShellModals } from "@/composables/useShellModals";
import { hasOfficialFirstPartyView } from "@/extensions/officialViews";
import type { ExtensionRecord } from "@/extensions/types";
import { useI18n } from "@/i18n/useI18n";
import type { MessageKey } from "@/i18n/messages";
import { isNewerVersion } from "@/lib/semver";
import AutodownloadConfigureView from "@/views/AutodownloadConfigureView.vue";
import TrafficConfigureView from "@/views/TrafficConfigureView.vue";
import WebsearchConfigureView from "@/views/WebsearchConfigureView.vue";
import LabelbridgeConfigureView from "@/views/LabelbridgeConfigureView.vue";
import JdownloaderConfigureView from "@/views/JdownloaderConfigureView.vue";
import PyloadConfigureView from "@/views/PyloadConfigureView.vue";

defineEmits<{
  close: [];
}>();

const route = useRoute();
const router = useRouter();
const { t } = useI18n();
const { extensionsFilter } = useShellModals();
const { extensions, loading, error, refresh, activate, deactivate } = useExtensionUI();
const busyId = ref("");
const notice = ref("");
const reloadingWorkers = ref(false);
const listFilter = ref(extensionsFilter.value || "");
const buckets = ref<BucketRecord[]>([]);
const catalog = ref<CatalogEntry[]>([]);
const bucketPath = ref("");
const bucketBusy = ref(false);
const archivePath = ref("");
const archiveSha = ref("");
const packageURL = ref("");
const packageSha = ref("");
const configuring = ref<{ extensionId: string; pageId: string } | null>(null);

watch(extensionsFilter, (q) => {
  if (q) listFilter.value = q;
});
watch(listFilter, (q) => {
  extensionsFilter.value = q;
});

/** Prefer settings page, else first declared page. */
function configureTarget(ext: ExtensionRecord): { extensionId: string; pageId: string } | null {
  const pages = ext.ui?.pages ?? [];
  if (!pages.length) return null;
  const settingsPanel = (ext.ui?.panels ?? []).find((p) => p.placement === "settings");
  if (settingsPanel) {
    const match = pages.find((p) => p.title === settingsPanel.title) ?? pages[0];
    return { extensionId: ext.id, pageId: match.id };
  }
  return { extensionId: ext.id, pageId: pages[0].id };
}

function stateLabel(state: string): string {
  const key = `extensions.state.${state}` as MessageKey;
  const translated = t(key);
  return translated === key ? state : translated;
}

/** Only allow http(s) repository links from catalogue metadata (no javascript:). */
function safeHttpUrl(raw: string | undefined): string {
  const v = (raw || "").trim();
  if (!v) return "";
  try {
    const u = new URL(v);
    if (u.protocol !== "http:" && u.protocol !== "https:") return "";
    return u.toString();
  } catch {
    return "";
  }
}

const installedIds = computed(() => new Set(extensions.value.map((e) => e.id)));

const catalogById = computed(() => {
  const map = new Map<string, CatalogEntry>();
  for (const c of catalog.value) map.set(c.id, c);
  return map;
});

const installedRows = computed(() =>
  [...extensions.value]
    .sort((a, b) => {
      const rank = (s: string) => (s === "active" ? 0 : s === "error" ? 2 : 1);
      const d = rank(a.state) - rank(b.state);
      return d !== 0 ? d : a.name.localeCompare(b.name);
    })
    .map((ext) => {
      const cat = catalogById.value.get(ext.id);
      const updateAvailable = Boolean(
        cat?.installable &&
          cat.bucketId &&
          cat.version &&
          isNewerVersion(cat.version, ext.version),
      );
      return {
        ext,
        configure: ext.state === "active" ? configureTarget(ext) : null,
        updateAvailable,
        catalogBucketId: cat?.bucketId || "",
        catalogVersion: cat?.version,
        repository: cat?.repository || "",
      };
    }),
);

function matchesFilter(name: string, description: string | undefined, q: string): boolean {
  if (!q) return true;
  const hay = `${name} ${description ?? ""}`.toLowerCase();
  return hay.includes(q);
}

const filteredInstalledRows = computed(() => {
  const q = listFilter.value.trim().toLowerCase();
  if (!q) return installedRows.value;
  return installedRows.value.filter((row) => matchesFilter(row.ext.name, row.ext.description, q));
});

const availableCatalog = computed(() =>
  catalog.value
    .filter((c) => c.installable && !installedIds.value.has(c.id))
    .sort((a, b) => a.name.localeCompare(b.name)),
);

const filteredAvailableCatalog = computed(() => {
  const q = listFilter.value.trim().toLowerCase();
  if (!q) return availableCatalog.value;
  return availableCatalog.value.filter((c) => matchesFilter(c.name, c.description, q));
});

async function toggle(id: string, state: string) {
  busyId.value = id;
  notice.value = "";
  try {
    if (state === "active") {
      await deactivate(id);
      notice.value = `${t("extensions.deactivate")}: ${id}`;
    } else {
      const ext = extensions.value.find((e) => e.id === id);
      let acceptRisk = false;
      if (ext?.consentRequired && !ext.consented) {
        const ok = window.confirm(
          `${ext.consentWarning || "Third-party extensions are not sandboxed."}\n\n${t("extensions.activate")} ${ext.name}?`,
        );
        if (!ok) {
          return;
        }
        acceptRisk = true;
      }
      await activate(id, { acceptRisk });
      notice.value = `${t("extensions.activate")}: ${id}`;
    }
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    busyId.value = "";
  }
}

async function uninstall(ext: ExtensionRecord) {
  if (ext.state === "active") return;
  const ok = window.confirm(t("extensions.uninstallConfirm", { name: ext.name }));
  if (!ok) return;
  busyId.value = ext.id;
  notice.value = "";
  try {
    await uninstallExtension(ext.id);
    notice.value = `${t("extensions.uninstall")}: ${ext.name}`;
    await refresh(true);
    await loadBuckets();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    busyId.value = "";
  }
}

async function loadBuckets() {
  bucketBusy.value = true;
  try {
    buckets.value = (await fetchBuckets()).buckets ?? [];
    catalog.value = (await fetchBucketCatalog()).extensions ?? [];
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

async function submitBucket() {
  if (!bucketPath.value.trim()) return;
  bucketBusy.value = true;
  notice.value = "";
  try {
    await addBucket(bucketPath.value.trim());
    bucketPath.value = "";
    notice.value = t("extensions.addBucket");
    await loadBuckets();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

async function toggleBucket(b: BucketRecord) {
  bucketBusy.value = true;
  try {
    await setBucketEnabled(b.id, !b.enabled);
    await loadBuckets();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

async function installCatalog(entry: CatalogEntry) {
  bucketBusy.value = true;
  notice.value = "";
  try {
    await installFromBucket(entry.bucketId, entry.id);
    notice.value = `${t("extensions.install")}: ${entry.name}`;
    await refresh(true);
    await loadBuckets();
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

/** Reinstall from the catalog listing (InstallLocal replaces the package dir). */
async function updateExtension(row: {
  ext: ExtensionRecord;
  catalogBucketId: string;
  catalogVersion?: string;
}) {
  if (!row.catalogBucketId) return;
  const wasLive = row.ext.state === "active" || row.ext.state === "error";
  busyId.value = row.ext.id;
  notice.value = "";
  try {
    await installFromBucket(row.catalogBucketId, row.ext.id);
    await refresh(true);
    await loadBuckets();
    if (wasLive) {
      const ext = extensions.value.find((e) => e.id === row.ext.id);
      let acceptRisk = false;
      if (ext?.consentRequired && !ext.consented) {
        acceptRisk = true;
      }
      await activate(row.ext.id, { acceptRisk });
    }
    const ver = row.catalogVersion ? ` → ${row.catalogVersion}` : "";
    notice.value = `${t("extensions.update")}: ${row.ext.name}${ver}`;
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    busyId.value = "";
  }
}

async function submitArchive() {
  if (!archivePath.value.trim() || !archiveSha.value.trim()) return;
  bucketBusy.value = true;
  notice.value = "";
  try {
    await installFromArchive(archivePath.value.trim(), archiveSha.value.trim());
    notice.value = t("extensions.installArchive");
    archivePath.value = "";
    archiveSha.value = "";
    await refresh(true);
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

async function submitURL() {
  if (!packageURL.value.trim() || !packageSha.value.trim()) return;
  bucketBusy.value = true;
  notice.value = "";
  try {
    await installFromURL(packageURL.value.trim(), packageSha.value.trim());
    notice.value = t("extensions.installUrl");
    packageURL.value = "";
    packageSha.value = "";
    await refresh(true);
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    bucketBusy.value = false;
  }
}

function openConfigure(target: { extensionId: string; pageId: string }) {
  configuring.value = target;
  const q = { ...route.query, modal: "extensions", ext: target.extensionId, page: target.pageId };
  void router.replace({ path: route.path === "/files" ? route.path : "/", query: q });
}

function closeConfigure() {
  configuring.value = null;
  const q: Record<string, string> = { modal: "extensions" };
  for (const [k, v] of Object.entries(route.query)) {
    if (k === "ext" || k === "page" || k === "modal") continue;
    if (typeof v === "string") q[k] = v;
  }
  void router.replace({ path: route.path === "/files" ? route.path : "/", query: q });
}

function syncConfigureFromQuery() {
  const ext = route.query.ext;
  const page = route.query.page;
  if (typeof ext === "string" && typeof page === "string" && ext && page) {
    configuring.value = { extensionId: ext, pageId: page };
  }
}

async function refreshAll() {
  await refresh(true);
  await loadBuckets();
}

async function reloadWorkers() {
  reloadingWorkers.value = true;
  notice.value = "";
  try {
    const res = await reloadExtensionWorkers();
    const failed = res.failed ?? {};
    const failIds = Object.keys(failed);
    const count = res.count ?? res.reloaded?.length ?? 0;
    if (failIds.length) {
      const detail = failIds
        .slice(0, 5)
        .map((id) => `${id}: ${failed[id]}`)
        .join("; ");
      notice.value = t("extensions.reloadWorkersPartial", {
        ok: count,
        fail: failIds.length,
        detail,
      });
    } else {
      notice.value = t("extensions.reloadWorkersDone", { count });
    }
    await refresh(true);
  } catch (e) {
    notice.value = e instanceof Error ? e.message : String(e);
  } finally {
    reloadingWorkers.value = false;
  }
}

onMounted(() => {
  syncConfigureFromQuery();
  void refreshAll();
});

watch(
  () => [route.query.ext, route.query.page],
  () => syncConfigureFromQuery(),
);

const refreshing = computed(() => loading.value || bucketBusy.value || reloadingWorkers.value);

defineExpose({
  refreshAll,
  reloadWorkers,
  refreshing,
  reloadingWorkers,
});
</script>

<template>
  <div class="fleet-ext-panel">
    <template v-if="configuring">
      <header class="fleet-ext-config-bar">
        <button
          type="button"
          class="fleet-btn"
          @click="closeConfigure"
        >
          ← {{ t("extensions.back") }}
        </button>
      </header>
      <AutodownloadConfigureView
        v-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.autodownload'"
      />
      <TrafficConfigureView
        v-else-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.traffic'"
      />
      <WebsearchConfigureView
        v-else-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.websearch'"
      />
      <LabelbridgeConfigureView
        v-else-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.labelbridge'"
      />
      <JdownloaderConfigureView
        v-else-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.jdownloader'"
      />
      <PyloadConfigureView
        v-else-if="hasOfficialFirstPartyView(configuring.extensionId) && configuring.extensionId === 'org.fleet.pyload'"
      />
      <ExtensionConfigBody
        v-else
        :extension-id="configuring.extensionId"
        :page-id="configuring.pageId"
      />
    </template>

    <template v-else>
      <div class="fleet-ext-filter">
        <input
          v-model="listFilter"
          class="fleet-control"
          type="search"
          :placeholder="t('extensions.searchPlaceholder')"
          :aria-label="t('extensions.searchPlaceholder')"
        >
      </div>

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

      <section class="fleet-ext-section">
        <div class="fleet-ext-section-head">
          <h3>{{ t("extensions.installed") }}</h3>
          <p>{{ t("extensions.installedHelp") }}</p>
        </div>

        <ul
          v-if="filteredInstalledRows.length"
          class="fleet-ext-list"
        >
          <li
            v-for="row in filteredInstalledRows"
            :key="row.ext.id"
            class="fleet-ext-card"
            :data-state="row.ext.state"
          >
            <div class="fleet-ext-card-main">
              <div class="fleet-ext-card-title">
                <strong>{{ row.ext.name }}</strong>
                <span
                  class="fleet-ext-state"
                  :data-state="row.ext.state"
                >{{ stateLabel(row.ext.state) }}</span>
                <span
                  v-if="row.ext.official"
                  class="fleet-ext-badge"
                >{{ t("extensions.official") }}</span>
                <a
                  v-if="safeHttpUrl(row.repository)"
                  class="fleet-ext-github"
                  :href="safeHttpUrl(row.repository)"
                  target="_blank"
                  rel="noopener noreferrer"
                  :aria-label="t('extensions.repository')"
                  :title="t('extensions.repository')"
                >
                  <ToolbarIcon name="github" />
                </a>
                <span
                  v-if="row.updateAvailable"
                  class="fleet-ext-badge"
                  data-kind="update"
                  :title="row.catalogVersion ? `${row.ext.version} → ${row.catalogVersion}` : undefined"
                >{{ t("extensions.state.update") }}</span>
              </div>
              <p
                v-if="row.ext.description"
                class="fleet-ext-desc"
              >
                {{ row.ext.description }}
              </p>
              <p
                v-if="row.ext.error"
                class="fleet-ext-err"
              >
                {{ row.ext.error }}
              </p>
            </div>
            <div class="fleet-ext-row-actions">
              <button
                v-if="row.updateAvailable"
                type="button"
                class="fleet-btn-primary"
                :disabled="busyId === row.ext.id || bucketBusy"
                :title="row.catalogVersion ? `${row.ext.version} → ${row.catalogVersion}` : undefined"
                @click="updateExtension(row)"
              >
                {{ t("extensions.update") }}
              </button>
              <button
                type="button"
                class="fleet-btn"
                :disabled="busyId === row.ext.id || (row.ext.state === 'error' && !row.ext.ui)"
                @click="toggle(row.ext.id, row.ext.state)"
              >
                {{ row.ext.state === "active" ? t("extensions.deactivate") : t("extensions.activate") }}
              </button>
              <button
                v-if="row.configure"
                type="button"
                :class="row.updateAvailable ? 'fleet-btn' : 'fleet-btn-primary'"
                @click="openConfigure(row.configure)"
              >
                {{ t("extensions.configure") }}
              </button>
              <button
                v-if="row.ext.state !== 'active'"
                type="button"
                class="fleet-btn-danger"
                :disabled="busyId === row.ext.id"
                @click="uninstall(row.ext)"
              >
                {{ t("extensions.uninstall") }}
              </button>
            </div>
          </li>
        </ul>
        <p
          v-else-if="!loading && installedRows.length && listFilter.trim()"
          class="fleet-ext-empty"
        >
          {{ t("extensions.searchEmpty") }}
        </p>
        <p
          v-else-if="!loading"
          class="fleet-ext-empty"
        >
          {{ t("extensions.none") }}
        </p>
      </section>

      <section class="fleet-ext-section">
        <div class="fleet-ext-section-head">
          <h3>{{ t("extensions.available") }}</h3>
          <p>{{ t("extensions.availableHelp") }}</p>
        </div>

        <ul
          v-if="filteredAvailableCatalog.length"
          class="fleet-ext-list"
        >
          <li
            v-for="c in filteredAvailableCatalog"
            :key="c.origin"
            class="fleet-ext-card"
          >
            <div class="fleet-ext-card-main">
              <div class="fleet-ext-card-title">
                <strong>{{ c.name }}</strong>
                <span class="fleet-ext-ver">{{ c.version }}</span>
                <a
                  v-if="safeHttpUrl(c.repository)"
                  class="fleet-ext-github"
                  :href="safeHttpUrl(c.repository)"
                  target="_blank"
                  rel="noopener noreferrer"
                  :aria-label="t('extensions.repository')"
                  :title="t('extensions.repository')"
                >
                  <ToolbarIcon name="github" />
                </a>
              </div>
              <p
                v-if="c.description"
                class="fleet-ext-desc"
              >
                {{ c.description }}
              </p>
            </div>
            <div class="fleet-ext-row-actions">
              <button
                type="button"
                class="fleet-btn-primary"
                :disabled="bucketBusy"
                @click="installCatalog(c)"
              >
                {{ t("extensions.install") }}
              </button>
            </div>
          </li>
        </ul>
        <p
          v-else-if="availableCatalog.length && listFilter.trim()"
          class="fleet-ext-empty"
        >
          {{ t("extensions.searchEmpty") }}
        </p>
        <p
          v-else
          class="fleet-ext-empty"
        >
          {{ t("extensions.catalogEmpty") }}
        </p>
      </section>

      <details class="fleet-ext-advanced">
        <summary>
          <span class="fleet-ext-advanced-title">{{ t("extensions.advanced") }}</span>
          <span class="fleet-ext-advanced-help">{{ t("extensions.advancedHelp") }}</span>
        </summary>

        <div class="fleet-ext-advanced-body">
          <div class="fleet-ext-section-head">
            <h4>{{ t("extensions.buckets") }}</h4>
            <p>{{ t("extensions.bucketsHelp") }}</p>
          </div>

          <label class="fleet-ext-field">
            {{ t("extensions.bucketPath") }}
            <input
              v-model="bucketPath"
              class="fleet-control"
              type="text"
              placeholder="/path/to/fleet-bucket"
            >
          </label>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              :disabled="bucketBusy || !bucketPath.trim()"
              @click="submitBucket"
            >
              {{ t("extensions.addBucket") }}
            </button>
            <button
              type="button"
              class="fleet-btn"
              :disabled="bucketBusy"
              @click="loadBuckets"
            >
              {{ t("extensions.refreshBuckets") }}
            </button>
          </div>

          <ul
            v-if="buckets.length"
            class="fleet-ext-list fleet-ext-list-compact"
          >
            <li
              v-for="b in buckets"
              :key="b.id"
              class="fleet-ext-card"
            >
              <div class="fleet-ext-card-main">
                <div class="fleet-ext-card-title">
                  <strong>{{ b.name }}</strong>
                  <span
                    v-if="b.official"
                    class="fleet-ext-badge"
                  >{{ t("extensions.official") }}</span>
                  <span
                    class="fleet-ext-state"
                    :data-state="b.enabled ? 'active' : 'inactive'"
                  >{{ b.enabled ? t("extensions.enable") : t("extensions.disable") }}</span>
                </div>
                <code class="fleet-ext-path">{{ b.path }}</code>
              </div>
              <div class="fleet-ext-row-actions">
                <button
                  type="button"
                  class="fleet-btn"
                  :disabled="bucketBusy"
                  @click="toggleBucket(b)"
                >
                  {{ b.enabled ? t("extensions.disable") : t("extensions.enable") }}
                </button>
              </div>
            </li>
          </ul>
          <p
            v-else
            class="fleet-ext-empty"
          >
            {{ t("extensions.noBuckets") }}
          </p>

          <div class="fleet-ext-section-head">
            <h4>{{ t("extensions.directInstall") }}</h4>
            <p>{{ t("extensions.directHelp") }}</p>
          </div>

          <label class="fleet-ext-field">
            {{ t("extensions.archivePath") }}
            <input
              v-model="archivePath"
              class="fleet-control"
              type="text"
              placeholder="/path/to/extension.tar.gz"
            >
          </label>
          <label class="fleet-ext-field">
            {{ t("extensions.archiveSha") }}
            <input
              v-model="archiveSha"
              class="fleet-control"
              type="text"
              placeholder="64 hex characters"
            >
          </label>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              :disabled="bucketBusy || !archivePath.trim() || !archiveSha.trim()"
              @click="submitArchive"
            >
              {{ t("extensions.installArchive") }}
            </button>
          </div>

          <label class="fleet-ext-field">
            {{ t("extensions.packageUrl") }}
            <input
              v-model="packageURL"
              class="fleet-control"
              type="url"
              placeholder="https://github.com/…/pkg.tar.gz"
            >
          </label>
          <label class="fleet-ext-field">
            {{ t("extensions.packageSha") }}
            <input
              v-model="packageSha"
              class="fleet-control"
              type="text"
              placeholder="64 hex characters"
            >
          </label>
          <div class="fleet-ext-actions">
            <button
              type="button"
              class="fleet-btn-primary"
              :disabled="bucketBusy || !packageURL.trim() || !packageSha.trim()"
              @click="submitURL"
            >
              {{ t("extensions.installUrl") }}
            </button>
          </div>
        </div>
      </details>
    </template>
  </div>
</template>

<style scoped>
.fleet-ext-panel {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
}

.fleet-ext-config-bar {
  display: flex;
  justify-content: flex-start;
}

.fleet-ext-section {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.fleet-ext-section-head h3,
.fleet-ext-section-head h4 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 650;
}

.fleet-ext-section-head p {
  margin: 0.2rem 0 0;
  font-size: 0.8rem;
  line-height: 1.35;
  color: var(--fleet-fg-muted);
}

.fleet-ext-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.fleet-ext-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--fleet-border, rgba(255, 255, 255, 0.1));
  border-radius: 8px;
  background: var(--fleet-surface-2, rgba(255, 255, 255, 0.03));
}

.fleet-ext-card[data-state="error"] {
  border-color: color-mix(in srgb, #e35d6a 45%, transparent);
}

.fleet-ext-card-main {
  min-width: 0;
  flex: 1;
}

.fleet-ext-card-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem 0.55rem;
}

.fleet-ext-card-title strong {
  font-size: 0.92rem;
}

.fleet-ext-ver {
  font-size: 0.75rem;
  color: var(--fleet-fg-muted);
}

.fleet-ext-state {
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--fleet-fg-muted);
}

.fleet-ext-state[data-state="active"] {
  background: color-mix(in srgb, #3ecf8e 22%, transparent);
  color: #3ecf8e;
}

.fleet-ext-state[data-state="error"] {
  background: color-mix(in srgb, #e35d6a 22%, transparent);
  color: #e35d6a;
}

.fleet-ext-desc {
  margin: 0.35rem 0 0;
  font-size: 0.8rem;
  line-height: 1.35;
  color: var(--fleet-fg-muted);
}

.fleet-ext-err {
  margin: 0.35rem 0 0;
  font-size: 0.78rem;
  color: #e35d6a;
}

.fleet-ext-empty {
  margin: 0;
  font-size: 0.85rem;
  color: var(--fleet-fg-muted);
}

.fleet-ext-row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: flex-end;
  flex-shrink: 0;
}

.fleet-ext-field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  font-size: 0.8rem;
  color: var(--fleet-fg-muted);
}

.fleet-ext-field input {
  width: 100%;
}

.fleet-ext-path {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.72rem;
  color: var(--fleet-fg-muted);
  word-break: break-all;
}

.fleet-ext-advanced {
  border-top: 1px solid var(--fleet-border, rgba(255, 255, 255, 0.1));
  padding-top: 0.75rem;
}

.fleet-ext-advanced > summary {
  cursor: pointer;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.fleet-ext-advanced > summary::-webkit-details-marker {
  display: none;
}

.fleet-ext-advanced-title {
  font-size: 0.9rem;
  font-weight: 650;
}

.fleet-ext-advanced-title::before {
  content: "▸ ";
  color: var(--fleet-fg-muted);
}

.fleet-ext-advanced[open] .fleet-ext-advanced-title::before {
  content: "▾ ";
}

.fleet-ext-advanced-help {
  font-size: 0.78rem;
  color: var(--fleet-fg-muted);
}

.fleet-ext-advanced-body {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  margin-top: 0.75rem;
}

.fleet-ext-list-compact .fleet-ext-card {
  padding: 0.55rem 0.7rem;
}
</style>
