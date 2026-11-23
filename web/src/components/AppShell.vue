<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute, useRouter } from "vue-router";
import {
  fetchSystemStatus,
  formatBytes,
  formatRate,
  type ClientProblem,
  type SystemDisk,
  type SystemLoad,
} from "@/api/client";
import ExtensionsPanel from "@/components/ExtensionsPanel.vue";
import Modal from "@/components/Modal.vue";
import SettingsPanel from "@/components/SettingsPanel.vue";
import ShortcutsHelp from "@/components/ShortcutsHelp.vue";
import ToolbarIcon from "@/components/ToolbarIcon.vue";
import { useExtensionUI } from "@/composables/useExtensionUI";
import { useFocusTrap } from "@/composables/useFocusTrap";
import { useShellModals } from "@/composables/useShellModals";
import { useTorrentSync } from "@/composables/useTorrentSync";
import { countActiveQueueProviders } from "@/extensions/downloadQueues";
import { useI18n } from "@/i18n/useI18n";
import { FILEMANAGER_EXT_ID } from "@/lib/downloadRoots";
import fleetLogo from "@/assets/fleet-logo.png";

const { t } = useI18n();
const { active: activeExtensions } = useExtensionUI();
const { torrents: syncTorrents, engine: syncEngine } = useTorrentSync();
const filesExtActive = computed(() =>
  activeExtensions.value.some((e) => e.id === FILEMANAGER_EXT_ID),
);

const props = defineProps<{
  downRate?: number;
  upRate?: number;
  torrentCount?: number;
  downloadingCount?: number;
  seedingCount?: number;
  clientsConnected?: number;
  clientsTotal?: number;
  /** Active hoster/NZB queue downloaders (JD, pyLoad, …). */
  downloadersConnected?: number;
  clientProblems?: ClientProblem[];
  initialSearch?: string;
}>();

const emit = defineEmits<{
  add: [];
  create: [];
  filters: [];
  action: [action: string];
  search: [value: string];
}>();

const route = useRoute();
const router = useRouter();
const {
  showSettings,
  showExtensions,
  showShortcuts,
  openSettings,
  openExtensions,
  openShortcuts,
  closeSettings,
  closeExtensions,
  closeShortcuts,
  anyModalOpen,
} = useShellModals();

const sidebarCollapsed = ref(false);
const mobileNavOpen = ref(false);
const narrowViewport = ref(false);
const search = ref(props.initialSearch ?? "");
const searchInputRef = ref<HTMLInputElement | null>(null);
const sidebarRef = ref<HTMLElement | null>(null);
const extensionsPanel = ref<{
  refreshAll: () => Promise<void>;
  reloadWorkers: () => Promise<void>;
  refreshing: boolean;
} | null>(null);
const MOBILE_NAV_MQ = "(max-width: 900px)";
const drawerTrapActive = computed(() => narrowViewport.value && mobileNavOpen.value);
useFocusTrap(sidebarRef, drawerTrapActive);

function isMobileNav() {
  return narrowViewport.value;
}

function consumeModalQuery() {
  const modal = route.query.modal;
  if (modal === "settings") {
    openSettings();
    const q = { ...route.query };
    delete q.modal;
    void router.replace({ path: route.path, query: q });
  } else if (modal === "extensions") {
    openExtensions();
    const q = { ...route.query };
    delete q.modal;
    void router.replace({ path: route.path, query: q });
  }
}

onMounted(consumeModalQuery);
watch(() => route.query.modal, consumeModalQuery);

const disks = ref<SystemDisk[]>([]);
const load = ref<SystemLoad | null>(null);
let sysTimer: ReturnType<typeof setInterval> | undefined;

async function refreshSystem() {
  try {
    const data = await fetchSystemStatus();
    disks.value = data.disks ?? [];
    load.value = data.load ?? null;
  } catch {
    // optional metrics; keep last known
  }
}

onMounted(() => {
  void refreshSystem();
  sysTimer = setInterval(() => void refreshSystem(), 15000);
});
onBeforeUnmount(() => {
  if (sysTimer) clearInterval(sysTimer);
});

const activeExtIds = computed(() => activeExtensions.value.map((e) => e.id));
const torrentClients = computed(
  () => props.clientsConnected ?? syncEngine.value?.clientsConnected ?? 0,
);
const clientsTotal = computed(
  () => props.clientsTotal ?? syncEngine.value?.clientsTotal ?? 0,
);
const hosterClients = computed(() => countActiveQueueProviders(activeExtIds.value, "hoster"));
const nzbClients = computed(() => countActiveQueueProviders(activeExtIds.value, "nzb"));
const clientsTotalLive = computed(
  () => torrentClients.value + hosterClients.value + nzbClients.value,
);
const problems = computed(() => props.clientProblems ?? syncEngine.value?.problems ?? []);
const clientsClass = computed(() => {
  if (clientsTotal.value === 0 && hosterClients.value === 0 && nzbClients.value === 0) return "bad";
  if (problems.value.length > 0) return "warn";
  if (clientsTotalLive.value > 0) return "ok";
  return "bad";
});
const clientsText = computed(() => t("status.clientsConnected", { n: clientsTotalLive.value }));
const clientsTitle = computed(() =>
  [
    t("status.clientsBreakdown.torrents", { n: torrentClients.value }),
    t("status.clientsBreakdown.nzb", { n: nzbClients.value }),
    t("status.clientsBreakdown.hosters", { n: hosterClients.value }),
  ].join("\n"),
);
const problemsText = computed(() => {
  const list = problems.value;
  if (!list.length) return "";
  if (list.length === 1) {
    return t("status.clientProblem", { name: list[0]?.name || "—" });
  }
  return t("status.clientProblems", { n: list.length });
});
const problemsTitle = computed(() =>
  problems.value
    .map((p) => `${p.name}${p.message ? `: ${p.message}` : ""}`)
    .join("\n"),
);
const downRateLabel = computed(() => {
  if (props.downRate != null) return formatRate(props.downRate);
  const sum = syncTorrents.value.reduce((acc, t) => acc + (t.downRate || 0), 0);
  return formatRate(sum);
});
const upRateLabel = computed(() => {
  if (props.upRate != null) return formatRate(props.upRate);
  const sum = syncTorrents.value.reduce((acc, t) => acc + (t.upRate || 0), 0);
  return formatRate(sum);
});
const primaryDisk = computed(() => disks.value[0] ?? null);
const diskLabel = computed(() => {
  const d = primaryDisk.value;
  if (!d) return "";
  const pct = Math.round(d.usedPct);
  return t("disk.status", { size: formatBytes(d.available), pct });
});
const diskClass = computed(() => {
  const d = primaryDisk.value;
  if (!d) return "";
  if (d.usedPct >= 95) return "bad";
  if (d.usedPct >= 85) return "warn";
  return "";
});
const diskTitle = computed(() => {
  const d = primaryDisk.value;
  if (!d) return "";
  return t("disk.detail", {
    path: d.path,
    free: formatBytes(d.available),
    used: formatBytes(d.used),
    total: formatBytes(d.total),
    pct: Math.round(d.usedPct),
  });
});
const loadLabel = computed(() => {
  if (!load.value) return "";
  return t("load.label", { value: load.value.load1.toFixed(2) });
});
const loadTitle = computed(() => {
  const l = load.value;
  if (!l) return "";
  const parts = [
    t("load.detail", {
      load1: l.load1.toFixed(2),
      load5: l.load5.toFixed(2),
      load15: l.load15.toFixed(2),
    }),
  ];
  if (l.procs) parts.push(t("load.procs", { procs: l.procs }));
  return parts.join(" · ");
});
const torrentsText = computed(() => {
  // Prefer live shared sync so Files (and other routes) show the global list size.
  const list = syncTorrents.value;
  const n = list.length || props.torrentCount || 0;
  const down =
    props.downloadingCount ??
    list.filter((t) => !t.complete && (t.downRate > 0 || t.state === "downloading")).length;
  const seed =
    props.seedingCount ??
    list.filter((t) => t.complete && t.active).length;
  return t("status.torrentsActivity", { n, down, up: seed });
});

function toggleSidebar() {
  if (isMobileNav()) {
    mobileNavOpen.value = !mobileNavOpen.value;
    return;
  }
  sidebarCollapsed.value = !sidebarCollapsed.value;
}

function expandSidebar() {
  sidebarCollapsed.value = false;
  if (isMobileNav()) {
    mobileNavOpen.value = true;
  }
}

function closeMobileNav() {
  mobileNavOpen.value = false;
}

function syncNarrowViewport(matches: boolean) {
  narrowViewport.value = matches;
  if (!matches) {
    mobileNavOpen.value = false;
  }
}

let mobileMq: MediaQueryList | undefined;
function onMobileMqChange(ev: MediaQueryListEvent) {
  syncNarrowViewport(ev.matches);
}

/** PD-UX-05: swipe drawer closed (left). No destructive gestures. */
let touchStartX = 0;
let touchStartY = 0;
function onDrawerTouchStart(ev: TouchEvent) {
  const t = ev.changedTouches[0];
  if (!t) return;
  touchStartX = t.clientX;
  touchStartY = t.clientY;
}
function onDrawerTouchEnd(ev: TouchEvent) {
  if (!mobileNavOpen.value) return;
  const t = ev.changedTouches[0];
  if (!t) return;
  const dx = t.clientX - touchStartX;
  const dy = t.clientY - touchStartY;
  if (dx < -56 && Math.abs(dx) > Math.abs(dy) * 1.2) {
    closeMobileNav();
  }
}

function isTypingTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  const tag = el.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || el.isContentEditable;
}

function focusShellSearch() {
  const el = searchInputRef.value ?? document.querySelector<HTMLInputElement>(".fleet-search input");
  if (!el) return;
  el.focus();
  el.select();
}

function isSearchHotkey(ev: KeyboardEvent): boolean {
  // Prefer ev.code (physical KeyK) so layout / browser quirks don't miss Ctrl+K.
  return (
    (ev.ctrlKey || ev.metaKey) &&
    !ev.altKey &&
    !ev.shiftKey &&
    (ev.code === "KeyK" || ev.key.toLowerCase() === "k")
  );
}

function onShellKeydown(ev: KeyboardEvent) {
  if (ev.key === "Escape" && mobileNavOpen.value) {
    ev.preventDefault();
    closeMobileNav();
    return;
  }
  // Ctrl/Cmd+K → focus torrent search (capture phase also used; beats browser omnibox + bare "k").
  if (isSearchHotkey(ev) && !anyModalOpen()) {
    ev.preventDefault();
    ev.stopImmediatePropagation();
    focusShellSearch();
    return;
  }
  // PD-UX-12: "?" opens shortcuts help on desktop; ignored while typing or when another modal is open.
  if (ev.key === "?" && !narrowViewport.value && !anyModalOpen() && !isTypingTarget(ev.target)) {
    ev.preventDefault();
    openShortcuts();
  }
}

onMounted(() => {
  if (typeof window === "undefined") return;
  mobileMq = window.matchMedia(MOBILE_NAV_MQ);
  syncNarrowViewport(mobileMq.matches);
  mobileMq.addEventListener("change", onMobileMqChange);
  // Capture: run before TorrentsView's "k" = moveFocus, and before Chrome's Ctrl+K omnibox.
  window.addEventListener("keydown", onShellKeydown, true);
});

onBeforeUnmount(() => {
  mobileMq?.removeEventListener("change", onMobileMqChange);
  window.removeEventListener("keydown", onShellKeydown, true);
});

const sidebarAriaHidden = computed(
  () => (narrowViewport.value ? !mobileNavOpen.value : sidebarCollapsed.value),
);

defineExpose({
  expandSidebar,
  closeMobileNav,
  sidebarCollapsed,
  mobileNavOpen,
  openSettings,
  openExtensions,
});

function onSearchInput(ev: Event) {
  const value = (ev.target as HTMLInputElement).value;
  search.value = value;
  emit("search", value);
}
</script>

<template>
  <div
    class="fleet-shell"
    :data-sidebar-collapsed="sidebarCollapsed"
    :data-mobile-nav="mobileNavOpen"
  >
    <header class="fleet-topbar">
      <div class="fleet-topbar-start">
        <button
          type="button"
          class="fleet-icon-btn"
          :title="sidebarCollapsed && !mobileNavOpen ? t('sidebar.expand') : t('sidebar.collapse')"
          :aria-label="sidebarCollapsed && !mobileNavOpen ? t('sidebar.expand') : t('sidebar.collapse')"
          :aria-expanded="mobileNavOpen || !sidebarCollapsed"
          @click="toggleSidebar"
        >
          <ToolbarIcon name="menu" />
        </button>

        <RouterLink
          class="fleet-brand"
          to="/"
          :title="t('brand.torrents')"
          :aria-label="t('brand.ariaHome')"
          @click="closeMobileNav"
        >
          <img
            class="fleet-brand-mark"
            :src="fleetLogo"
            alt=""
            width="26"
            height="26"
            draggable="false"
          >
          <span class="fleet-brand-word">Fleet</span>
        </RouterLink>

        <RouterLink
          v-if="filesExtActive"
          class="fleet-btn fleet-topbar-files"
          to="/files"
          :title="t('files.title')"
          @click="closeMobileNav"
        >
          {{ t("nav.files") }}
        </RouterLink>

        <button
          type="button"
          class="fleet-btn-primary"
          :aria-label="t('nav.add')"
          @click="emit('add')"
        >
          {{ t("nav.add") }}
        </button>
        <button
          type="button"
          class="fleet-btn fleet-topbar-create"
          :title="t('nav.createHelp')"
          :aria-label="t('nav.create')"
          @click="emit('create')"
        >
          {{ t("nav.create") }}
        </button>
      </div>

      <div class="fleet-topbar-mid">
        <div class="fleet-toolbar-group">
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('action.start')"
            :aria-label="t('action.start')"
            @click="emit('action', 'start')"
          >
            <ToolbarIcon name="play" />
          </button>
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('action.pause')"
            :aria-label="t('action.pause')"
            @click="emit('action', 'pause')"
          >
            <ToolbarIcon name="pause" />
          </button>
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('action.stop')"
            :aria-label="t('action.stop')"
            @click="emit('action', 'stop')"
          >
            <ToolbarIcon name="stop" />
          </button>
          <button
            type="button"
            class="fleet-icon-btn fleet-topbar-hash"
            :title="t('action.hash')"
            :aria-label="t('action.hash')"
            @click="emit('action', 'hash')"
          >
            <ToolbarIcon name="hash" />
          </button>
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('action.remove')"
            :aria-label="t('action.remove')"
            @click="emit('action', 'erase')"
          >
            <ToolbarIcon name="remove" />
          </button>
          <button
            type="button"
            class="fleet-icon-btn"
            :title="t('nav.filters')"
            :aria-label="t('nav.filters')"
            @click="emit('filters')"
          >
            <ToolbarIcon name="filters" />
          </button>
        </div>

        <label class="fleet-search">
          <input
            ref="searchInputRef"
            :value="search"
            type="search"
            :placeholder="t('search.placeholder')"
            autocomplete="off"
            @input="onSearchInput"
          >
        </label>
      </div>

      <div class="fleet-topbar-end">
        <div
          class="fleet-speed"
          :aria-label="t('status.transferRates')"
        >
          <span class="down">↓ {{ downRateLabel }}</span>
          <span class="up">↑ {{ upRateLabel }}</span>
        </div>

        <button
          type="button"
          class="fleet-icon-btn"
          :title="t('nav.extensions')"
          :aria-label="t('nav.extensions')"
          :aria-pressed="showExtensions"
          @click="openExtensions(); closeMobileNav()"
        >
          <ToolbarIcon name="extensions" />
        </button>

        <button
          type="button"
          class="fleet-icon-btn"
          :title="t('nav.settings')"
          :aria-label="t('nav.settings')"
          :aria-pressed="showSettings"
          @click="openSettings(); closeMobileNav()"
        >
          <ToolbarIcon name="settings" />
        </button>
      </div>
    </header>

    <button
      v-if="mobileNavOpen"
      type="button"
      class="fleet-mobile-backdrop"
      :aria-label="t('nav.closeDrawer')"
      @click="closeMobileNav"
    />

    <div class="fleet-body">
      <aside
        ref="sidebarRef"
        class="fleet-sidebar"
        :aria-hidden="sidebarAriaHidden"
        :aria-modal="drawerTrapActive ? 'true' : undefined"
        :role="drawerTrapActive ? 'dialog' : undefined"
        :aria-label="drawerTrapActive ? t('nav.primary') : undefined"
        @touchstart.passive="onDrawerTouchStart"
        @touchend.passive="onDrawerTouchEnd"
      >
        <div class="fleet-sidebar-inner">
          <nav
            class="fleet-mobile-chrome-nav"
            :aria-label="t('nav.primary')"
          >
            <RouterLink
              class="fleet-side-item"
              to="/"
              :data-active="route.path === '/' ? 'true' : undefined"
              @click="closeMobileNav"
            >
              <span>{{ t("brand.torrents") }}</span>
            </RouterLink>
            <RouterLink
              v-if="filesExtActive"
              class="fleet-side-item"
              to="/files"
              :data-active="route.path.startsWith('/files') ? 'true' : undefined"
              @click="closeMobileNav"
            >
              <span>{{ t("nav.files") }}</span>
            </RouterLink>
          </nav>
          <slot name="sidebar" />
        </div>
      </aside>
      <main class="fleet-main">
        <slot />
      </main>
    </div>

    <footer class="fleet-statusbar">
      <span
        :class="clientsClass"
        :title="clientsTitle"
      >{{ clientsText }}</span>
      <span
        v-if="problemsText"
        class="bad fleet-statusbar-problems"
        :title="problemsTitle"
      >{{ problemsText }}</span>
      <span>{{ torrentsText }}</span>
      <span
        v-if="diskLabel"
        class="fleet-statusbar-disk"
        :class="diskClass"
        :title="diskTitle"
      >{{ diskLabel }}</span>
      <span
        v-if="loadLabel"
        class="fleet-statusbar-load"
        :title="loadTitle"
      >{{ loadLabel }}</span>
    </footer>

    <Modal
      v-if="showExtensions"
      :title="t('extensions.title')"
      wide
      @close="closeExtensions"
    >
      <template #actions>
        <button
          type="button"
          class="fleet-btn"
          :disabled="extensionsPanel?.refreshing"
          @click="extensionsPanel?.reloadWorkers()"
        >
          {{ t("extensions.reloadWorkers") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="extensionsPanel?.refreshing"
          @click="extensionsPanel?.refreshAll()"
        >
          {{ t("extensions.refresh") }}
        </button>
      </template>
      <ExtensionsPanel
        ref="extensionsPanel"
        @close="closeExtensions"
      />
    </Modal>

    <Modal
      v-if="showSettings"
      :title="t('settings.title')"
      wide
      @close="closeSettings"
    >
      <SettingsPanel @close="closeSettings" />
    </Modal>

    <Modal
      v-if="showShortcuts"
      :title="t('shortcuts.title')"
      @close="closeShortcuts"
    >
      <ShortcutsHelp />
    </Modal>
  </div>
</template>
