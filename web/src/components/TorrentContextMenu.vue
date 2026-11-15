<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "@/i18n/useI18n";

const props = defineProps<{
  x: number;
  y: number;
  count: number;
  labels: string[];
  /** Label shared by all selected torrents; empty string if unlabeled; null if mixed. */
  activeLabel?: string | null;
  canPause?: boolean;
  canStop?: boolean;
  showCrossSeed?: boolean;
  showViewNfo?: boolean;
  showTitleSearch?: boolean;
  showBandwidth?: boolean;
}>();

const emit = defineEmits<{
  close: [];
  action: [action: string];
  setLabel: [label: string];
  setPriority: [priority: number];
  copy: [kind: "hash" | "name" | "path"];
  properties: [];
  download: [];
  crossSeed: [];
  viewNfo: [];
  titleSearch: [];
  bandwidth: [];
}>();

const { t } = useI18n();
const root = ref<HTMLElement | null>(null);
const openSub = ref<"labels" | "priority" | "erase" | "copy" | null>(null);
const newLabel = ref("");
const pos = ref({ left: props.x, top: props.y });

const priorities = computed(() => [
  { value: 0, label: t("details.priority.off") },
  { value: 1, label: t("details.priority.low") },
  { value: 2, label: t("details.priority.normal") },
  { value: 3, label: t("details.priority.high") },
]);

function clampPosition() {
  const el = root.value;
  if (!el || typeof window === "undefined") return;
  const rect = el.getBoundingClientRect();
  const pad = 8;
  let left = props.x;
  let top = props.y;
  if (left + rect.width > window.innerWidth - pad) {
    left = Math.max(pad, window.innerWidth - rect.width - pad);
  }
  if (top + rect.height > window.innerHeight - pad) {
    top = Math.max(pad, window.innerHeight - rect.height - pad);
  }
  pos.value = { left, top };
}

function onDocPointer(ev: MouseEvent) {
  if (!root.value) return;
  if (root.value.contains(ev.target as Node)) return;
  emit("close");
}

function onKey(ev: KeyboardEvent) {
  if (ev.key === "Escape") emit("close");
}

onMounted(async () => {
  await nextTick();
  clampPosition();
  window.addEventListener("mousedown", onDocPointer, true);
  window.addEventListener("keydown", onKey);
  window.addEventListener("resize", clampPosition);
});

onBeforeUnmount(() => {
  window.removeEventListener("mousedown", onDocPointer, true);
  window.removeEventListener("keydown", onKey);
  window.removeEventListener("resize", clampPosition);
});

watch(
  () => [props.x, props.y],
  async () => {
    await nextTick();
    clampPosition();
  },
);

function run(action: string) {
  emit("action", action);
  emit("close");
}

function label(value: string) {
  emit("setLabel", value);
  emit("close");
}

/** Clicking the already-applied label removes it (toggle off). */
function toggleLabel(value: string) {
  if (props.activeLabel != null && props.activeLabel === value) {
    label("");
    return;
  }
  label(value);
}

function applyNewLabel() {
  const value = newLabel.value.trim();
  if (!value) return;
  label(value);
}

function goProperties() {
  // Parent closes the menu after focusing the details panel so a
  // mousedown→unmount cannot swallow the action or click-through the row.
  emit("properties");
}

function goDownload() {
  emit("download");
  emit("close");
}

function goCrossSeed() {
  emit("crossSeed");
  emit("close");
}

function goViewNfo() {
  emit("viewNfo");
  emit("close");
}

function goTitleSearch() {
  emit("titleSearch");
  emit("close");
}

function goBandwidth() {
  emit("bandwidth");
  emit("close");
}

function priority(value: number) {
  emit("setPriority", value);
  emit("close");
}

function copy(kind: "hash" | "name" | "path") {
  emit("copy", kind);
  emit("close");
}
</script>

<template>
  <Teleport to="body">
    <div
      ref="root"
      class="fleet-ctx"
      role="menu"
      :style="{ left: `${pos.left}px`, top: `${pos.top}px` }"
      @contextmenu.prevent
      @mousedown.stop
    >
      <div
        class="fleet-ctx-head"
        role="presentation"
      >
        ✓ {{ t("ctx.count", { n: count }) }}
      </div>

      <div
        class="fleet-ctx-sep"
        role="separator"
      />

      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="run('start')"
      >
        {{ t("action.start") }}
      </button>
      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        :disabled="canPause === false"
        @click="run('pause')"
      >
        {{ t("action.pause") }}
      </button>
      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        :disabled="canStop === false"
        @click="run('stop')"
      >
        {{ t("action.stop") }}
      </button>
      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="run('hash')"
      >
        {{ t("ctx.recheck") }}
      </button>

      <div
        class="fleet-ctx-sep"
        role="separator"
      />

      <div
        class="fleet-ctx-fly"
        @mouseenter="openSub = 'labels'"
        @mouseleave="openSub = openSub === 'labels' ? null : openSub"
      >
        <button
          type="button"
          class="fleet-ctx-item has-sub"
          role="menuitem"
          aria-haspopup="true"
          :aria-expanded="openSub === 'labels'"
          @click="openSub = openSub === 'labels' ? null : 'labels'"
        >
          <span>{{ t("details.label") }}</span>
          <span class="fleet-ctx-caret">›</span>
        </button>
        <div
          v-if="openSub === 'labels'"
          class="fleet-ctx-sub"
          role="menu"
          @mousedown.stop
        >
          <button
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            :data-active="activeLabel === ''"
            @click="label('')"
          >
            <span>{{ t("ctx.label.clear") }}</span>
            <span
              v-if="activeLabel === ''"
              class="fleet-ctx-check"
              aria-hidden="true"
            >✓</span>
          </button>
          <button
            v-for="l in labels"
            :key="l"
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            :data-active="activeLabel === l"
            :title="activeLabel === l ? t('ctx.label.remove') : undefined"
            @click="toggleLabel(l)"
          >
            <span>{{ l }}</span>
            <span
              v-if="activeLabel === l"
              class="fleet-ctx-check"
              aria-hidden="true"
            >✓</span>
          </button>
          <div class="fleet-ctx-new-label">
            <span class="fleet-ctx-new-label-title">{{ t("ctx.label.new") }}</span>
            <input
              v-model="newLabel"
              class="fleet-control"
              type="text"
              :placeholder="t('ctx.label.placeholder')"
              @keydown.enter.prevent="applyNewLabel"
              @keydown.escape.stop="openSub = null"
            >
            <button
              type="button"
              class="fleet-btn-primary"
              :disabled="!newLabel.trim()"
              @click="applyNewLabel"
            >
              {{ t("ctx.label.apply") }}
            </button>
          </div>
        </div>
      </div>

      <div
        class="fleet-ctx-fly"
        @mouseenter="openSub = 'priority'"
        @mouseleave="openSub = openSub === 'priority' ? null : openSub"
      >
        <button
          type="button"
          class="fleet-ctx-item has-sub"
          role="menuitem"
          aria-haspopup="true"
          :aria-expanded="openSub === 'priority'"
          @click="openSub = openSub === 'priority' ? null : 'priority'"
        >
          <span>{{ t("details.priority") }}</span>
          <span class="fleet-ctx-caret">›</span>
        </button>
        <div
          v-if="openSub === 'priority'"
          class="fleet-ctx-sub"
          role="menu"
        >
          <button
            v-for="p in priorities"
            :key="p.value"
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            @click="priority(p.value)"
          >
            {{ p.label }}
          </button>
        </div>
      </div>

      <div
        class="fleet-ctx-sep"
        role="separator"
      />

      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="run('erase')"
      >
        {{ t("ctx.remove") }}
      </button>
      <div
        class="fleet-ctx-fly"
        @mouseenter="openSub = 'erase'"
        @mouseleave="openSub = openSub === 'erase' ? null : openSub"
      >
        <button
          type="button"
          class="fleet-ctx-item has-sub"
          role="menuitem"
          aria-haspopup="true"
          :aria-expanded="openSub === 'erase'"
          @click="openSub = openSub === 'erase' ? null : 'erase'"
        >
          <span>{{ t("ctx.removeAnd") }}</span>
          <span class="fleet-ctx-caret">›</span>
        </button>
        <div
          v-if="openSub === 'erase'"
          class="fleet-ctx-sub"
          role="menu"
        >
          <button
            type="button"
            class="fleet-ctx-item danger"
            role="menuitem"
            @click="run('erase_data')"
          >
            {{ t("ctx.removeData") }}
          </button>
        </div>
      </div>

      <div
        class="fleet-ctx-sep"
        role="separator"
      />

      <button
        v-if="showCrossSeed"
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goCrossSeed"
      >
        {{ t("ctx.crossSeed") }}
      </button>

      <button
        v-if="showViewNfo"
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goViewNfo"
      >
        {{ t("ctx.viewNfo") }}
      </button>

      <button
        v-if="showTitleSearch"
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goTitleSearch"
      >
        {{ t("ctx.titlesearch") }}
      </button>

      <button
        v-if="showBandwidth"
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goBandwidth"
      >
        {{ t("ctx.bandwidth") }}
      </button>

      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goProperties"
      >
        {{ t("ctx.properties") }}
      </button>
      <button
        type="button"
        class="fleet-ctx-item"
        role="menuitem"
        @click="goDownload"
      >
        {{ t("ctx.downloadTorrent") }}
      </button>
      <div
        class="fleet-ctx-fly"
        @mouseenter="openSub = 'copy'"
        @mouseleave="openSub = openSub === 'copy' ? null : openSub"
      >
        <button
          type="button"
          class="fleet-ctx-item has-sub"
          role="menuitem"
          aria-haspopup="true"
          :aria-expanded="openSub === 'copy'"
          @click="openSub = openSub === 'copy' ? null : 'copy'"
        >
          <span>{{ t("details.copy") }}</span>
          <span class="fleet-ctx-caret">›</span>
        </button>
        <div
          v-if="openSub === 'copy'"
          class="fleet-ctx-sub"
          role="menu"
        >
          <button
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            @click="copy('name')"
          >
            {{ t("ctx.copy.name") }}
          </button>
          <button
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            @click="copy('hash')"
          >
            {{ t("ctx.copy.hash") }}
          </button>
          <button
            type="button"
            class="fleet-ctx-item"
            role="menuitem"
            @click="copy('path')"
          >
            {{ t("ctx.copy.path") }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.fleet-ctx {
  position: fixed;
  z-index: 1200;
  min-width: 14.5rem;
  max-width: 18rem;
  padding: 0.3rem 0;
  border: 1px solid var(--fleet-border);
  border-radius: 6px;
  background: var(--fleet-surface);
  box-shadow: var(--fleet-shadow);
  color: var(--fleet-foreground);
  font-size: 0.85rem;
}

.fleet-ctx-head {
  padding: 0.4rem 0.85rem 0.45rem;
  color: var(--fleet-primary-text);
  font-weight: 650;
  font-size: 0.82rem;
}

.fleet-ctx-sep {
  height: 1px;
  margin: 0.3rem 0;
  background: var(--fleet-border-soft);
}

.fleet-ctx-item {
  appearance: none;
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0;
  padding: 0.38rem 0.85rem;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.fleet-ctx-item:hover:not(:disabled) {
  background: color-mix(in srgb, var(--fleet-primary) 16%, transparent);
}

.fleet-ctx-item:disabled {
  color: var(--fleet-fg-muted);
  opacity: 0.45;
  cursor: default;
}

.fleet-ctx-item[data-active="true"] {
  color: var(--fleet-primary-text);
  font-weight: 600;
}

.fleet-ctx-check {
  color: var(--fleet-primary-text);
  font-size: 0.85rem;
  line-height: 1;
}

.fleet-ctx-item.danger {
  color: var(--fleet-destructive-text);
}

.fleet-ctx-caret {
  color: var(--fleet-fg-muted);
  font-size: 1rem;
  line-height: 1;
}

.fleet-ctx-fly {
  position: relative;
}

.fleet-ctx-sub {
  position: absolute;
  top: -0.3rem;
  left: calc(100% - 4px);
  min-width: 11rem;
  max-width: 16rem;
  max-height: min(50vh, 20rem);
  overflow: auto;
  padding: 0.3rem 0;
  border: 1px solid var(--fleet-border);
  border-radius: 6px;
  background: var(--fleet-surface);
  box-shadow: var(--fleet-shadow);
}

.fleet-ctx-empty {
  margin: 0;
  padding: 0.4rem 0.85rem;
  color: var(--fleet-fg-muted);
  font-size: 0.8rem;
}

.fleet-ctx-new-label {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.45rem 0.75rem 0.55rem;
  border-top: 1px solid var(--fleet-border-soft);
  margin-top: 0.2rem;
}

.fleet-ctx-new-label-title {
  font-size: 0.72rem;
  font-weight: 650;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--fleet-fg-muted);
}

.fleet-ctx-new-label input {
  width: 100%;
  padding: 0.35rem 0.45rem;
  border: 1px solid var(--fleet-border);
  border-radius: 4px;
  background: var(--fleet-input);
  color: var(--fleet-foreground);
  font: inherit;
}

.fleet-ctx-new-label .fleet-btn-primary {
  align-self: stretch;
}
</style>
