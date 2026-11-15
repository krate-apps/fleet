<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { extensionRPC } from "@/api/client";
import { useI18n } from "@/i18n/useI18n";

const EXT = "org.fleet.websearch";
const { t } = useI18n();

type Hit = {
  engine?: string;
  title?: string;
  sizeBytes?: number;
  sizeLabel?: string;
  magnet?: string;
  torrentUrl?: string;
  kind?: string;
  parseDiag?: string;
  infoHash?: string;
};

type Diag = {
  engine?: string;
  status?: number;
  error?: string;
  hits?: number;
  parseNote?: string;
};

const busy = ref(false);
const notice = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const query = ref("");
const engine = ref("");
const gatewayId = ref("");
const label = ref("");
const sortBy = ref("engine");
const filterKind = ref("");
const dryRun = ref(true);
const results = ref<Hit[]>([]);
const diagnostics = ref<Diag[]>([]);
const packagedText = ref("");
const enginesText = ref("");

function setNotice(msg: string, kind: "ok" | "error" = "ok") {
  notice.value = msg;
  noticeKind.value = kind;
}

const engineOptions = computed(() => {
  const names = new Set<string>();
  for (const line of packagedText.value.split("\n")) {
    const name = line.split("|")[0]?.trim();
    if (name) names.add(name);
  }
  for (const line of enginesText.value.split("\n")) {
    const name = line.split("|")[0]?.trim();
    if (name) names.add(name);
  }
  return [...names];
});

async function loadSettings() {
  const res = await extensionRPC(EXT, "ext.websearch.getSettings", {});
  const r = (res.result ?? {}) as {
    gatewayId?: string;
    label?: string;
    enginesText?: string;
    packagedEnginesText?: string;
  };
  gatewayId.value = r.gatewayId ?? "";
  label.value = r.label ?? "";
  enginesText.value = r.enginesText ?? "";
  packagedText.value = r.packagedEnginesText ?? "";
}

async function saveDefaults() {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.websearch.saveSettings", {
      enginesText: enginesText.value,
      gatewayId: gatewayId.value,
      label: label.value,
    });
    const r = (res.result ?? {}) as { enginesText?: string };
    enginesText.value = r.enginesText ?? enginesText.value;
    setNotice(t("websearch.saved"));
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function runSearch() {
  if (!query.value.trim()) {
    setNotice(t("websearch.queryRequired"), "error");
    return;
  }
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.websearch.search", {
      query: query.value.trim(),
      engine: engine.value.trim(),
      gatewayId: gatewayId.value.trim(),
      label: label.value.trim(),
      add: false,
      sortBy: sortBy.value,
      filterKind: filterKind.value,
    });
    const r = (res.result ?? {}) as {
      results?: Hit[];
      resultCount?: number;
      diagnostics?: Diag[];
      dryNotice?: string;
    };
    results.value = Array.isArray(r.results) ? r.results : [];
    diagnostics.value = Array.isArray(r.diagnostics) ? r.diagnostics : [];
    setNotice(t("websearch.found", { n: r.resultCount ?? results.value.length }));
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function enqueueHit(hit: Hit) {
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.websearch.enqueue", {
      magnet: hit.magnet ?? "",
      torrentUrl: hit.torrentUrl ?? "",
      engine: hit.engine ?? "",
      title: hit.title ?? "",
      gatewayId: gatewayId.value.trim(),
      label: label.value.trim(),
      dryRun: dryRun.value,
    });
    const r = (res.result ?? {}) as {
      dryRun?: boolean;
      dryNotice?: string;
      notice?: string;
      added?: number;
    };
    if (r.dryRun) {
      setNotice(r.dryNotice || t("websearch.dryNotice"));
    } else {
      setNotice(r.notice || t("websearch.enqueued", { n: r.added ?? 0 }));
    }
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
}

async function enqueueAllDryThenConfirm() {
  if (!results.value.length) return;
  if (dryRun.value) {
    busy.value = true;
    try {
      const res = await extensionRPC(EXT, "ext.websearch.search", {
        query: query.value.trim(),
        engine: engine.value.trim(),
        gatewayId: gatewayId.value.trim(),
        label: label.value.trim(),
        add: true,
        dryRun: true,
        sortBy: sortBy.value,
        filterKind: filterKind.value,
      });
      const r = (res.result ?? {}) as { dryNotice?: string; resultCount?: number };
      setNotice(r.dryNotice || t("websearch.dryNotice"));
    } catch (e) {
      setNotice(e instanceof Error ? e.message : String(e), "error");
    } finally {
      busy.value = false;
    }
    return;
  }
  if (!confirm(t("websearch.confirmAll", { n: results.value.length }))) return;
  busy.value = true;
  try {
    const res = await extensionRPC(EXT, "ext.websearch.search", {
      query: query.value.trim(),
      engine: engine.value.trim(),
      gatewayId: gatewayId.value.trim(),
      label: label.value.trim(),
      add: true,
      dryRun: false,
      sortBy: sortBy.value,
      filterKind: filterKind.value,
    });
    const r = (res.result ?? {}) as { added?: number };
    setNotice(t("websearch.enqueued", { n: r.added ?? 0 }));
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
  } catch (e) {
    setNotice(e instanceof Error ? e.message : String(e), "error");
  } finally {
    busy.value = false;
  }
});
</script>

<template>
  <div class="xs-cfg">
    <header class="xs-cfg__head">
      <h2>{{ t("websearch.title") }}</h2>
      <p class="xs-cfg__hint">
        {{ t("websearch.hint") }}
      </p>
    </header>

    <p
      v-if="notice"
      class="xs-cfg__notice"
      :data-kind="noticeKind"
    >
      {{ notice }}
    </p>

    <section class="xs-cfg__card">
      <h3>{{ t("websearch.search") }}</h3>
      <label class="xs-cfg__row">
        {{ t("websearch.query") }}
        <input
          v-model="query"
          class="fleet-control"
          type="text"
          :disabled="busy"
          @keydown.enter.prevent="runSearch"
        >
      </label>
      <label class="xs-cfg__row">
        {{ t("websearch.engine") }}
        <select
          v-model="engine"
          class="fleet-control"
          :disabled="busy"
        >
          <option value="">
            {{ t("websearch.allEngines") }}
          </option>
          <option
            v-for="name in engineOptions"
            :key="name"
            :value="name"
          >
            {{ name }}
          </option>
        </select>
      </label>
      <label class="xs-cfg__row">
        {{ t("websearch.gateway") }}
        <input
          v-model="gatewayId"
          class="fleet-control"
          type="text"
          :disabled="busy"
          :placeholder="t('websearch.gatewayPh')"
        >
      </label>
      <label class="xs-cfg__row">
        {{ t("websearch.label") }}
        <input
          v-model="label"
          class="fleet-control"
          type="text"
          :disabled="busy"
        >
      </label>
      <label class="xs-cfg__row">
        {{ t("websearch.sort") }}
        <select
          v-model="sortBy"
          class="fleet-control"
          :disabled="busy"
        >
          <option value="engine">
            {{ t("websearch.sortEngine") }}
          </option>
          <option value="size">
            {{ t("websearch.sortSizeDesc") }}
          </option>
          <option value="size-asc">
            {{ t("websearch.sortSizeAsc") }}
          </option>
          <option value="title">
            {{ t("websearch.sortTitle") }}
          </option>
        </select>
      </label>
      <label class="xs-cfg__row">
        {{ t("websearch.filterKind") }}
        <select
          v-model="filterKind"
          class="fleet-control"
          :disabled="busy"
        >
          <option value="">
            {{ t("websearch.filterAll") }}
          </option>
          <option value="magnet">
            magnet
          </option>
          <option value="torrent">
            torrent
          </option>
        </select>
      </label>
      <label class="xs-cfg__row">
        <input
          v-model="dryRun"
          type="checkbox"
          :disabled="busy"
        >
        {{ t("websearch.dryRun") }}
      </label>
      <div class="xs-cfg__actions">
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="runSearch"
        >
          {{ t("websearch.run") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy || !results.length"
          @click="enqueueAllDryThenConfirm"
        >
          {{ dryRun ? t("websearch.previewAll") : t("websearch.enqueueAll") }}
        </button>
        <button
          type="button"
          class="btn"
          :disabled="busy"
          @click="saveDefaults"
        >
          {{ t("websearch.saveDefaults") }}
        </button>
      </div>
    </section>

    <section class="xs-cfg__card">
      <h3>{{ t("websearch.results") }}</h3>
      <div class="xs-cfg__table-wrap">
        <table class="xs-cfg__table">
          <thead>
            <tr>
              <th>{{ t("websearch.colTitle") }}</th>
              <th>{{ t("websearch.colEngine") }}</th>
              <th>{{ t("websearch.colSize") }}</th>
              <th>{{ t("websearch.colKind") }}</th>
              <th>{{ t("websearch.colDiag") }}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-if="!results.length">
              <td colspan="6">
                {{ t("websearch.noResults") }}
              </td>
            </tr>
            <tr
              v-for="(hit, idx) in results"
              :key="(hit.infoHash || hit.magnet || hit.torrentUrl || '') + idx"
            >
              <td>{{ hit.title || "—" }}</td>
              <td>{{ hit.engine }}</td>
              <td>{{ hit.sizeLabel || "—" }}</td>
              <td>{{ hit.kind }}</td>
              <td class="xs-cfg__diag">
                {{ hit.parseDiag || "" }}
              </td>
              <td>
                <button
                  type="button"
                  class="btn"
                  :disabled="busy"
                  @click="enqueueHit(hit)"
                >
                  {{ dryRun ? t("websearch.preview") : t("websearch.enqueue") }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section
      v-if="diagnostics.length"
      class="xs-cfg__card"
    >
      <h3>{{ t("websearch.diagnostics") }}</h3>
      <ul class="xs-cfg__diags">
        <li
          v-for="d in diagnostics"
          :key="d.engine"
        >
          <strong>{{ d.engine }}</strong>
          — hits {{ d.hits ?? 0 }}
          <span v-if="d.status"> · HTTP {{ d.status }}</span>
          <span v-if="d.error"> · {{ d.error }}</span>
          <span v-if="d.parseNote"> · {{ d.parseNote }}</span>
        </li>
      </ul>
    </section>

    <section class="xs-cfg__card">
      <h3>{{ t("websearch.userEngines") }}</h3>
      <textarea
        v-model="enginesText"
        class="fleet-control"
        rows="6"
        :disabled="busy"
      />
      <p class="xs-cfg__hint">
        {{ t("websearch.enginesHelp") }}
      </p>
    </section>
  </div>
</template>

<style scoped>
.xs-cfg {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 0.25rem 0 1.5rem;
  max-width: 64rem;
}
.xs-cfg__head h2 {
  margin: 0 0 0.25rem;
  font-size: 1.15rem;
}
.xs-cfg__hint {
  margin: 0.35rem 0 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.xs-cfg__notice {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--fleet-accent, #3d7) 18%, transparent);
}
.xs-cfg__notice[data-kind="error"] {
  background: color-mix(in srgb, #c44 22%, transparent);
}
.xs-cfg__card {
  border: 1px solid color-mix(in srgb, currentColor 16%, transparent);
  border-radius: 6px;
  padding: 0.75rem 1rem;
}
.xs-cfg__card h3 {
  margin: 0 0 0.75rem;
  font-size: 1rem;
}
.xs-cfg__row {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  margin-bottom: 0.55rem;
}
.xs-cfg__row input[type="text"],
.xs-cfg__row select,
.xs-cfg__card textarea {
  width: 100%;
  max-width: 36rem;
}
.xs-cfg__actions {
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
.xs-cfg__table-wrap {
  overflow: auto;
  max-height: 22rem;
}
.xs-cfg__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
}
.xs-cfg__table th,
.xs-cfg__table td {
  text-align: left;
  padding: 0.3rem 0.45rem;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  vertical-align: top;
}
.xs-cfg__diag {
  opacity: 0.7;
  font-size: 0.8rem;
}
.xs-cfg__diags {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.9rem;
}
</style>
