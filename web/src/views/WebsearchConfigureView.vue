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
  <div class="fleet-ext-config">
    <aside class="fleet-ext-config-side">
      <header class="fleet-ext-header">
        <p class="fleet-ext-eyebrow">
          {{ t("websearch.title") }}
        </p>
        <h2>{{ t("websearch.title") }}</h2>
        <p class="fleet-ext-note">
          {{ t("websearch.hint") }}
        </p>
        <p class="fleet-ext-tip">
          {{ t("extensions.configTip") }}
        </p>
      </header>
      <div class="fleet-ext-actions">
        <button
          type="button"
          class="fleet-btn-primary"
          :disabled="busy"
          @click="runSearch"
        >
          {{ t("websearch.run") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy || !results.length"
          @click="enqueueAllDryThenConfirm"
        >
          {{ dryRun ? t("websearch.previewAll") : t("websearch.enqueueAll") }}
        </button>
        <button
          type="button"
          class="fleet-btn"
          :disabled="busy"
          @click="saveDefaults"
        >
          {{ t("websearch.saveDefaults") }}
        </button>
      </div>
    </aside>

    <div class="fleet-ext-config-main">
      <div
        v-if="notice"
        class="fleet-banner"
        :data-kind="noticeKind"
      >
        {{ notice }}
      </div>

      <form
        class="fleet-decl-form"
        @submit.prevent="runSearch"
      >
        <header class="fleet-decl-form-head">
          <h3 class="fleet-decl-form-title">
            {{ t("websearch.search") }}
          </h3>
        </header>
        <div class="fleet-decl-fields">
          <label
            class="fleet-decl-field"
            data-wide="true"
          >
            <span class="fleet-decl-label">{{ t("websearch.query") }}</span>
            <input
              v-model="query"
              class="fleet-decl-control"
              type="text"
              :disabled="busy"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("websearch.engine") }}</span>
            <select
              v-model="engine"
              class="fleet-decl-control"
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
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("websearch.gateway") }}</span>
            <input
              v-model="gatewayId"
              class="fleet-decl-control"
              type="text"
              :disabled="busy"
              :placeholder="t('websearch.gatewayPh')"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("websearch.label") }}</span>
            <input
              v-model="label"
              class="fleet-decl-control"
              type="text"
              :disabled="busy"
            >
          </label>
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("websearch.sort") }}</span>
            <select
              v-model="sortBy"
              class="fleet-decl-control"
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
          <label class="fleet-decl-field">
            <span class="fleet-decl-label">{{ t("websearch.filterKind") }}</span>
            <select
              v-model="filterKind"
              class="fleet-decl-control"
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
          <label
            class="fleet-decl-field"
            data-kind="boolean"
            data-wide="true"
          >
            <input
              v-model="dryRun"
              class="fleet-decl-check"
              type="checkbox"
              :disabled="busy"
            >
            <span>{{ t("websearch.dryRun") }}</span>
          </label>
        </div>
        <button
          type="submit"
          class="fleet-btn-primary fleet-form-apply"
          :disabled="busy"
        >
          {{ t("websearch.run") }}
        </button>
      </form>

      <section class="fleet-obs-report">
        <h4>{{ t("websearch.results") }}</h4>
        <div class="ws-table-wrap">
          <table class="fleet-ext-table">
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
                <td class="ws-diag">
                  {{ hit.parseDiag || "" }}
                </td>
                <td>
                  <button
                    type="button"
                    class="fleet-btn"
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
        class="fleet-obs-report"
      >
        <h4>{{ t("websearch.diagnostics") }}</h4>
        <ul class="ws-diags">
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

      <form
        class="fleet-decl-form"
        @submit.prevent="saveDefaults"
      >
        <header class="fleet-decl-form-head">
          <h3 class="fleet-decl-form-title">
            {{ t("websearch.userEngines") }}
          </h3>
          <p class="fleet-decl-form-desc">
            {{ t("websearch.enginesHelp") }}
          </p>
        </header>
        <div class="fleet-decl-fields">
          <label
            class="fleet-decl-field"
            data-kind="textarea"
            data-wide="true"
          >
            <span class="fleet-decl-label">{{ t("websearch.userEngines") }}</span>
            <textarea
              v-model="enginesText"
              class="fleet-decl-control fleet-decl-textarea"
              rows="6"
              :disabled="busy"
            />
          </label>
        </div>
        <button
          type="submit"
          class="fleet-btn-primary fleet-form-apply"
          :disabled="busy"
        >
          {{ t("websearch.saveDefaults") }}
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.ws-table-wrap {
  overflow: auto;
  max-height: 22rem;
}

.ws-diag {
  opacity: 0.7;
  font-size: 0.8rem;
}

.ws-diags {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.9rem;
}
</style>
