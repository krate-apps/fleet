<script setup lang="ts">
import { computed, ref, toRef, watch } from "vue";
import { useRouter } from "vue-router";
import { createShare, deleteShare, extensionRPC, fetchShares } from "@/api/client";
import DeclarativeForm from "@/components/DeclarativeForm.vue";
import DslEditorChrome from "@/components/DslEditorChrome.vue";
import SchedulerSlotsBuilder from "@/components/SchedulerSlotsBuilder.vue";
import { useExtensionUI } from "@/composables/useExtensionUI";
import { useShellModals } from "@/composables/useShellModals";
import type { UIAction, UIForm } from "@/extensions/types";
import { useI18n } from "@/i18n/useI18n";
import { parseExtratioRulesText } from "@/lib/extratioDsl";
import { parseHooksText } from "@/lib/hooksDsl";
import { parseChannelsText } from "@/lib/notifyChannels";
import { parseRcloneRulesText } from "@/lib/rcloneRulesDsl";
import { parseRulesText } from "@/lib/rulesDsl";
import {
  parseSchedule,
  toSavePayload,
  validateSchedule,
  type SchedulerSlot,
} from "@/lib/schedulerSlots";

const props = defineProps<{
  extensionId: string;
  pageId: string;
}>();

const router = useRouter();
const { closeAll } = useShellModals();
const { findPage, refresh, loading, error: loadError } = useExtensionUI();
const { t } = useI18n();
const notice = ref("");
const noticeDetail = ref("");
const noticeKind = ref<"ok" | "error">("ok");
const frameURLs = ref<string[]>([]);
const nfoText = ref("");
const busy = ref(false);
const formInitials = ref<Record<string, Record<string, unknown>>>({});
const dslMode = ref<"builder" | "expert">("builder");
const rulesTextDraft = ref("");
const channelsTextDraft = ref("");
const hooksTextDraft = ref("");
const rcloneRulesDraft = ref("");
const extratioRulesDraft = ref("");
const dslParseError = ref("");
const scheduleSlots = ref<SchedulerSlot[]>([]);

type MetricsStatus = {
  scrapeURL: string;
  listening: boolean;
  lastSuccessAt: string;
  lastError: string;
};

type JanitorReportEntry = {
  time?: string;
  kind?: string;
  hash?: string;
  name?: string;
  directory?: string;
  label?: string;
  clientId?: string;
};

type HistoryEvent = {
  id?: string;
  action?: string;
  actionTime?: number;
  hash?: string;
  name?: string;
  size?: number;
  label?: string;
};

const metricsStatus = ref<MetricsStatus | null>(null);
const janitorReport = ref<JanitorReportEntry[]>([]);
const historyEvents = ref<HistoryEvent[]>([]);
const historyActionFilter = ref<"" | "addition" | "finish" | "deletion">("");

type PortwatchDay = {
  date?: string;
  upCount?: number;
  downCount?: number;
  upRatio?: number;
  samples?: number;
};

const portwatchDays = ref<PortwatchDay[]>([]);
const portwatchSampleCount = ref(0);

const extensionId = toRef(props, "extensionId");
const pageId = toRef(props, "pageId");

const resolved = computed(() => findPage(extensionId.value, pageId.value));

const showSchedulerChrome = computed(() => extensionId.value === "org.fleet.scheduler");

type DslKind = "rules" | "notify" | "hooks" | "rclone" | "extratio";

const dslKind = computed<DslKind | null>(() => {
  switch (extensionId.value) {
    case "org.fleet.automation":
      return "rules";
    case "org.fleet.notifications":
      return "notify";
    case "org.fleet.eventhooks":
      return "hooks";
    case "org.fleet.rclone":
      return "rclone";
    case "org.fleet.ratiorules":
      return "extratio";
    default:
      return null;
  }
});

const showDslChrome = computed(() => dslKind.value != null);

const DSL_HIDE_FIELDS: Record<string, string> = {
  "org.fleet.automation": "rulesText",
  "org.fleet.notifications": "channelsText",
  "org.fleet.eventhooks": "hooksText",
  "org.fleet.rclone": "rulesText",
  "org.fleet.ratiorules": "rulesText",
};

const DSL_FORM_IDS: Record<DslKind, string> = {
  rules: "rules-settings",
  notify: "notify-settings",
  hooks: "hooks-settings",
  rclone: "rclone-settings",
  extratio: "extratio-rules",
};

/** Form that hosts Editor / slots / history chrome (settings first, chrome, then Apply). */
const chromeHostFormId = computed<string | null>(() => {
  if (dslKind.value) return DSL_FORM_IDS[dslKind.value];
  switch (extensionId.value) {
    case "org.fleet.scheduler":
      return "schedule-enable";
    case "org.fleet.history":
      return "history-settings";
    case "org.fleet.portwatch":
      return "portwatch-settings";
    default:
      return null;
  }
});

function isChromeHostForm(form: UIForm, formIdx: number, formCount: number): boolean {
  if (chromeHostFormId.value) return form.id === chromeHostFormId.value;
  return formIdx === formCount - 1;
}

const DSL_FIELD_NAMES: Record<DslKind, string> = {
  rules: "rulesText",
  notify: "channelsText",
  hooks: "hooksText",
  rclone: "rulesText",
  extratio: "rulesText",
};

const dslDraft = computed({
  get() {
    switch (dslKind.value) {
      case "rules":
        return rulesTextDraft.value;
      case "notify":
        return channelsTextDraft.value;
      case "hooks":
        return hooksTextDraft.value;
      case "rclone":
        return rcloneRulesDraft.value;
      case "extratio":
        return extratioRulesDraft.value;
      default:
        return "";
    }
  },
  set(v: string) {
    switch (dslKind.value) {
      case "rules":
        rulesTextDraft.value = v;
        break;
      case "notify":
        channelsTextDraft.value = v;
        break;
      case "hooks":
        hooksTextDraft.value = v;
        break;
      case "rclone":
        rcloneRulesDraft.value = v;
        break;
      case "extratio":
        extratioRulesDraft.value = v;
        break;
      default:
        break;
    }
  },
});

const dslExpertLabelKey = computed(() => {
  switch (dslKind.value) {
    case "rules":
      return "dsl.rules.expertLabel" as const;
    case "notify":
      return "dsl.notify.expertLabel" as const;
    case "hooks":
      return "dsl.hooks.expertLabel" as const;
    case "rclone":
      return "dsl.rclone.expertLabel" as const;
    case "extratio":
      return "dsl.extratio.expertLabel" as const;
    default:
      return "dsl.modeExpert" as const;
  }
});

/** Hide DSL textarea + master Enabled (Extensions Activate is the only on/off). */
const displayForms = computed(() => {
  const forms = resolved.value?.forms ?? [];
  const hideDsl = DSL_HIDE_FIELDS[extensionId.value];
  return forms.map((form) => ({
    ...form,
    fields: form.fields.filter((f) => {
      if (hideDsl && f.name === hideDsl) return false;
      // Soft-disable checkbox removed — activate/deactivate owns power state.
      if (f.name === "enabled" && f.kind === "boolean") return false;
      return true;
    }),
  }));
});

function formHasMasterEnabled(form: UIForm | undefined): boolean {
  return Boolean(form?.fields.some((f) => f.name === "enabled" && f.kind === "boolean"));
}

function parseCurrentDsl(text: string): { error?: string } {
  switch (dslKind.value) {
    case "rules":
      return parseRulesText(text);
    case "notify":
      return parseChannelsText(text);
    case "hooks":
      return parseHooksText(text);
    case "rclone":
      return parseRcloneRulesText(text);
    case "extratio":
      return parseExtratioRulesText(text);
    default:
      return {};
  }
}

function setDslMode(mode: "builder" | "expert") {
  if (mode === "builder") {
    const parsed = parseCurrentDsl(dslDraft.value);
    if (parsed.error) {
      dslParseError.value = parsed.error;
      return;
    }
    dslParseError.value = "";
  }
  dslMode.value = mode;
}

function onDslParseError(msg: string) {
  dslParseError.value = msg;
  if (msg) dslMode.value = "expert";
}

function syncDslDraftsFromInitials() {
  const kind = dslKind.value;
  if (!kind) return;
  const formId = DSL_FORM_IDS[kind];
  const field = DSL_FIELD_NAMES[kind];
  const raw = formInitials.value[formId]?.[field];
  const text = typeof raw === "string" ? raw : "";
  dslDraft.value = text;
  const parsed = parseCurrentDsl(text);
  if (parsed.error) {
    dslParseError.value = parsed.error;
    dslMode.value = "expert";
  } else {
    dslParseError.value = "";
  }
}

const globalActions = computed(() => {
  const actions = (resolved.value?.actions ?? []).filter((a) => a.target === "global");
  return [...actions].sort((a, b) => {
    const rank = (x: UIAction) => (x.primary ? 0 : x.danger ? 2 : 1);
    return rank(a) - rank(b);
  });
});

function actionBtnClass(a: UIAction): string {
  if (a.primary) return "fleet-btn-primary";
  if (a.danger) return "fleet-btn fleet-btn-danger";
  return "fleet-btn";
}

const COUNT_LABELS: Record<string, string> = {
  engines: "engines",
  userEngines: "engines",
  packagedEngines: "packaged",
  rules: "rules",
  hosts: "hosts",
  accounts: "accounts",
  channels: "channels",
  dirs: "dirs",
  filters: "filters",
  irc: "IRC servers",
  ircServers: "IRC servers",
  trackers: "trackers",
  pairs: "pairs",
  remotes: "remotes",
  groups: "groups",
  feeds: "feeds",
  scanned: "scanned",
  matched: "matched",
  added: "added",
  removed: "removed",
  updated: "updated",
  cleared: "cleared",
  jobs: "jobs",
  entries: "entries",
  count: "items",
  checked: "checked",
  notified: "notified",
  rewritten: "rewritten",
  hits: "hits",
  shares: "shares",
};

function clearNotice() {
  notice.value = "";
  noticeDetail.value = "";
  noticeKind.value = "ok";
}

function setNotice(summary: string, kind: "ok" | "error" = "ok", detail?: string) {
  notice.value = summary;
  noticeKind.value = kind;
  noticeDetail.value = detail ?? "";
}

function setError(e: unknown) {
  setNotice(e instanceof Error ? e.message : String(e), "error");
}

function humanSaveNotice(result: unknown, fallback = "Saved"): { summary: string; detail: string } {
  const detail = result === undefined ? "" : JSON.stringify(result, null, 2);
  if (!result || typeof result !== "object") {
    return { summary: fallback, detail };
  }
  const r = result as Record<string, unknown>;
  const bits: string[] = [];
  for (const [key, label] of Object.entries(COUNT_LABELS)) {
    const v = r[key];
    if (typeof v === "number" && Number.isFinite(v)) {
      bits.push(`${v} ${label}`);
    } else if (Array.isArray(v)) {
      bits.push(`${v.length} ${label}`);
    }
  }
  for (const [key, label] of [
    ["enginesText", "engines"],
    ["rulesText", "rules"],
    ["hostsText", "hosts"],
    ["accountsText", "accounts"],
    ["channelsText", "channels"],
    ["dirsText", "dirs"],
    ["filtersText", "filters"],
    ["ircServersText", "IRC servers"],
    ["trackersText", "trackers"],
    ["pairsText", "pairs"],
    ["urlsText", "URLs"],
    ["remotesAllowText", "remotes"],
    ["categoryMapsText", "maps"],
    ["targetGatewayIdsText", "gateways"],
  ] as const) {
    if (bits.some((b) => b.endsWith(label))) continue;
    const text = r[key];
    if (typeof text !== "string" || !text.trim()) continue;
    const n = text.split("\n").map((l) => l.trim()).filter((l) => l && !l.startsWith("#")).length;
    bits.push(`${n} ${label}`);
  }
  if (typeof r.ok === "boolean" && bits.length === 0) {
    return { summary: r.ok ? fallback : "Save failed", detail };
  }
  return {
    summary: bits.length ? `${fallback} · ${bits.join(" · ")}` : fallback,
    detail,
  };
}

function applySaveNotice(result: unknown, fallback = "Saved") {
  const { summary, detail } = humanSaveNotice(result, fallback);
  setNotice(summary, "ok", detail);
}

function pickFormValues(form: UIForm, raw: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const field of form.fields) {
    if (field.name in raw) out[field.name] = raw[field.name];
  }
  return out;
}

async function loadFormPrefill(form: UIForm) {
  if (!form.loadMethod) return;
  try {
    const res = await extensionRPC(extensionId.value, form.loadMethod, {});
    const raw = (res.result && typeof res.result === "object")
      ? (res.result as Record<string, unknown>)
      : {};
    formInitials.value = {
      ...formInitials.value,
      [form.id]: pickFormValues(form, raw),
    };
  } catch {
    // Prefill is best-effort; leave defaults.
  }
}

async function loadAllPrefills() {
  const forms = resolved.value?.forms ?? [];
  formInitials.value = {};
  await Promise.all(forms.filter((f) => f.loadMethod).map((f) => loadFormPrefill(f)));
}

async function loadSchedulerSchedule() {
  if (!showSchedulerChrome.value) {
    scheduleSlots.value = [];
    return;
  }
  try {
    const res = await extensionRPC(extensionId.value, "ext.scheduler.get", {});
    const st = parseSchedule(res.result);
    scheduleSlots.value = st.slots;
    formInitials.value = {
      ...formInitials.value,
      "schedule-enable": {
        ...(formInitials.value["schedule-enable"] ?? {}),
        enabled: st.enabled,
      },
    };
  } catch {
    scheduleSlots.value = [];
  }
}

async function refreshMetricsStatus() {
  if (extensionId.value !== "org.fleet.metrics") {
    metricsStatus.value = null;
    return;
  }
  try {
    const res = await extensionRPC(extensionId.value, "ext.metrics.getSettings", {});
    const raw = (res.result && typeof res.result === "object")
      ? (res.result as Record<string, unknown>)
      : {};
    metricsStatus.value = {
      scrapeURL: typeof raw.scrapeURL === "string" ? raw.scrapeURL : "",
      listening: Boolean(raw.listening),
      lastSuccessAt: typeof raw.lastSuccessAt === "string" ? raw.lastSuccessAt : "",
      lastError: typeof raw.lastError === "string" ? raw.lastError : "",
    };
  } catch {
    metricsStatus.value = null;
  }
}

function openJanitorPath(directory: string) {
  const path = directory.trim();
  if (!path) return;
  closeAll();
  void router.push({ path: "/files", query: { path } });
}

function filterJanitorLabel(label: string) {
  const value = label.trim();
  if (!value) return;
  closeAll();
  void router.push({ path: "/", query: { filter: `label:${value}` } });
}

function downloadBlob(filename: string, content: string, mime: string) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

function formatHistoryTime(ts?: number) {
  if (!ts) return "—";
  try {
    return new Date(ts * 1000).toLocaleString();
  } catch {
    return String(ts);
  }
}

async function refreshHistoryEvents() {
  if (extensionId.value !== "org.fleet.history") {
    historyEvents.value = [];
    return;
  }
  try {
    const res = await extensionRPC(extensionId.value, "ext.history.list", {
      since: 0,
      action: historyActionFilter.value,
    });
    const items = (res.result as { items?: HistoryEvent[] } | undefined)?.items;
    historyEvents.value = Array.isArray(items) ? items : [];
  } catch {
    historyEvents.value = [];
  }
}

async function refreshPortwatchHistory() {
  if (extensionId.value !== "org.fleet.portwatch") {
    portwatchDays.value = [];
    portwatchSampleCount.value = 0;
    return;
  }
  try {
    const res = await extensionRPC(extensionId.value, "ext.portwatch.getHistory", {});
    const raw = (res.result && typeof res.result === "object")
      ? (res.result as Record<string, unknown>)
      : {};
    const days = raw.days;
    portwatchDays.value = Array.isArray(days) ? (days as PortwatchDay[]) : [];
    portwatchSampleCount.value = typeof raw.sampleCount === "number" ? raw.sampleCount : 0;
  } catch {
    portwatchDays.value = [];
    portwatchSampleCount.value = 0;
  }
}

function formatUpRatio(ratio?: number) {
  if (typeof ratio !== "number" || Number.isNaN(ratio)) return "—";
  return `${Math.round(ratio * 100)}%`;
}

watch(
  () => [extensionId.value, pageId.value, resolved.value?.forms?.map((f) => `${f.id}:${f.loadMethod ?? ""}`).join("|")] as const,
  () => {
    dslMode.value = "builder";
    dslParseError.value = "";
    janitorReport.value = [];
    historyEvents.value = [];
    historyActionFilter.value = "";
    portwatchDays.value = [];
    portwatchSampleCount.value = 0;
    void loadAllPrefills()
      .then(() => syncDslDraftsFromInitials())
      .then(() => loadSchedulerSchedule())
      .then(() => refreshMetricsStatus())
      .then(() => refreshHistoryEvents())
      .then(() => refreshPortwatchHistory());
  },
  { immediate: true },
);

async function onFormSubmit(values: Record<string, unknown>) {
  busy.value = true;
  clearNotice();
  frameURLs.value = [];
  nfoText.value = "";
  try {
    const formId = String(values.__formId ?? "");
    const form = resolved.value?.forms.find((f) => f.id === formId);
    if (form?.submitMethod) {
      const payload: Record<string, unknown> = { ...values };
      delete payload.__formId;
      if (dslKind.value === "rules") payload.rulesText = rulesTextDraft.value;
      if (dslKind.value === "notify") payload.channelsText = channelsTextDraft.value;
      if (dslKind.value === "hooks") payload.hooksText = hooksTextDraft.value;
      if (dslKind.value === "rclone") payload.rulesText = rcloneRulesDraft.value;
      if (dslKind.value === "extratio") payload.rulesText = extratioRulesDraft.value;
      // Master enabled is no longer in the form — keep workers soft-on while activated.
      if (formHasMasterEnabled(form)) payload.enabled = true;
      const res = await extensionRPC(extensionId.value, form.submitMethod, payload);
      applySaveNotice(res.result);
      await loadFormPrefill(form);
      syncDslDraftsFromInitials();
      if (extensionId.value === "org.fleet.metrics") await refreshMetricsStatus();
      return;
    }
    if (extensionId.value === "org.fleet.rssfeeds") {
      const payload = {
        url: String(values.url ?? ""),
        label: String(values.label ?? ""),
        loginAccount: String(values.loginAccount ?? ""),
        includeRegex: String(values.includeRegex ?? ""),
        excludeRegex: String(values.excludeRegex ?? ""),
        intervalSec: Number(values.intervalSec) || 300,
        gatewayId: String(values.gatewayId ?? ""),
        enabled: values.enabled === undefined ? true : Boolean(values.enabled),
      };
      const res = await extensionRPC(extensionId.value, "ext.rssfeeds.add", payload);
      applySaveNotice(res.result, "Feed saved");
      return;
    }
    if (extensionId.value === "org.fleet.automove") {
      const res = await extensionRPC(extensionId.value, "ext.automove.add", values);
      applySaveNotice(res.result, "Rule saved");
      return;
    }
    if (extensionId.value === "org.fleet.scheduler") {
      const payload = toSavePayload({
        enabled: true,
        slots: scheduleSlots.value,
      });
      const err = validateSchedule(payload);
      if (err) {
        setNotice(err, "error");
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.scheduler.save", payload);
      applySaveNotice(res.result, "Schedule saved");
      await loadSchedulerSchedule();
      return;
    }
    if (extensionId.value === "org.fleet.ratio") {
      const name = String(values.name ?? "").trim();
      const group = {
        name,
        enabled: values.enabled === undefined ? true : Boolean(values.enabled),
        minRatio: Number(values.minRatio) || 0,
        maxRatio: Number(values.maxRatio) || 0,
        minUploadBytes: Number(values.minUploadBytes) || 0,
        maxSeedSeconds: Number(values.maxSeedSeconds) || 0,
        action: String(values.action ?? "stop").trim() || "stop",
      };
      const cur = await extensionRPC(extensionId.value, "ext.ratio.get", {});
      const prev = (cur.result ?? {}) as {
        enabled?: boolean;
        intervalSec?: number;
        groups?: Record<string, unknown>[];
      };
      const groups = [...(prev.groups ?? [])];
      const idx = groups.findIndex((g) => String(g.name ?? "").toLowerCase() === name.toLowerCase());
      if (idx >= 0) groups[idx] = group;
      else groups.push(group);
      const payload = {
        enabled: values.enabled === undefined ? (prev.enabled ?? true) : Boolean(values.enabled),
        intervalSec: Number(values.intervalSec) || prev.intervalSec || 60,
        groups,
      };
      const res = await extensionRPC(extensionId.value, "ext.ratio.save", payload);
      applySaveNotice(res.result, "Ratio group saved");
      return;
    }
    if (extensionId.value === "org.fleet.mediainfo") {
      const res = await extensionRPC(extensionId.value, "ext.mediainfo.run", {
        hash: String(values.hash ?? ""),
        index: Number(values.index) || 0,
        path: String(values.path ?? ""),
      });
      if (typeof (res.result as { text?: string })?.text === "string") {
        setNotice("MediaInfo", "ok", (res.result as { text: string }).text);
      } else {
        applySaveNotice(res.result, "MediaInfo");
      }
      return;
    }
    if (extensionId.value === "org.fleet.titlesearch") {
      const res = await extensionRPC(extensionId.value, "ext.titlesearch.resolve", {
        title: String(values.title ?? ""),
        engine: String(values.engine ?? ""),
      });
      const links = (res.result as { links?: { name: string; url: string }[] })?.links ?? [];
      let opened = 0;
      for (const l of links) {
        if (!l.url) continue;
        const win = window.open(l.url, "_blank", "noopener,noreferrer");
        if (win) opened += 1;
      }
      if (links.length) {
        setNotice(
          opened
            ? t("ctx.titlesearch.done", { opened: String(opened), total: String(links.length), query: String(values.title ?? "") })
            : t("ctx.titlesearch.blocked", { total: String(links.length), query: String(values.title ?? "") }),
          "ok",
          links.map((l) => `${l.name}: ${l.url}`).join("\n"),
        );
      } else {
        applySaveNotice(res.result, "Look At");
      }
      return;
    }
    if (extensionId.value === "org.fleet.rssfeedsrewrite") {
      const res = await extensionRPC(extensionId.value, "ext.rssrewrite.apply", {
        url: String(values.url ?? ""),
      });
      const out = res.result as { url?: string; changed?: boolean; matchedRule?: string };
      if (out?.url) {
        const match = out.matchedRule ? ` (rule: ${out.matchedRule})` : "";
        notice.value = `${out.changed ? "Rewrote" : "Unchanged"}${match}: ${out.url}`;
      } else {
        applySaveNotice(res.result, "Apply done");
      }
      return;
    }
    if (extensionId.value === "org.fleet.trackerlogin") {
      const res = await extensionRPC(extensionId.value, "ext.trackerlogin.login", {
        name: String(values.name ?? ""),
      });
      applySaveNotice(res.result, "Login");
      return;
    }
    if (extensionId.value === "org.fleet.websearch") {
      const res = await extensionRPC(extensionId.value, "ext.websearch.search", {
        query: String(values.query ?? ""),
        engine: String(values.engine ?? ""),
        gatewayId: String(values.gatewayId ?? ""),
        label: String(values.label ?? ""),
        add: Boolean(values.add),
        dryRun: Boolean(values.dryRun),
        sortBy: String(values.sortBy ?? ""),
        filterKind: String(values.filterKind ?? ""),
      });
      const out = res.result as {
        resultCount?: number;
        added?: number;
        dryRun?: boolean;
        dryNotice?: string;
        results?: {
          engine: string;
          title?: string;
          sizeLabel?: string;
          kind?: string;
          parseDiag?: string;
          magnet?: string;
          torrentUrl?: string;
        }[];
        diagnostics?: { engine: string; hits?: number; error?: string; parseNote?: string; status?: number }[];
        links?: { name: string; url: string }[];
      };
      const lines: string[] = [];
      if (out?.dryNotice) lines.push(out.dryNotice);
      if (out?.links?.length) {
        lines.push(...out.links.map((l) => `${l.name}: ${l.url}`));
      }
      lines.push(`Found ${out?.resultCount ?? 0} hit(s); added ${out?.added ?? 0}${out?.dryRun ? " (dry-run)" : ""}`);
      if (out?.diagnostics?.length) {
        lines.push(
          ...out.diagnostics.map(
            (d) =>
              `diag ${d.engine}: hits=${d.hits ?? 0}${d.status ? ` http=${d.status}` : ""}${d.error ? ` err=${d.error}` : ""}${d.parseNote ? ` (${d.parseNote})` : ""}`,
          ),
        );
      }
      if (out?.results?.length) {
        lines.push(
          ...out.results.slice(0, 30).map((r) => {
            const payload = r.magnet || r.torrentUrl || "";
            return `${r.engine} [${r.kind ?? "?"}] ${r.title ?? ""} ${r.sizeLabel ?? ""} · ${payload}${r.parseDiag ? ` · ${r.parseDiag}` : ""}`;
          }),
        );
      }
      setNotice(lines[0] ?? "Search done", "ok", lines.join("\n"));
      return;
    }
    if (extensionId.value === "org.fleet.screenshots") {
      const res = await extensionRPC(extensionId.value, "ext.screenshots.run", {
        hash: String(values.hash ?? ""),
        index: Number(values.index) || 0,
        path: String(values.path ?? ""),
        count: Number(values.count) || 6,
        offset: Number(values.offset) || 5,
        interval: Number(values.interval) || 5,
        width: Number(values.width) || 480,
      });
      const out = res.result as { count?: number; frames?: { dataUrl?: string; offsetSec?: number }[] };
      frameURLs.value = (out?.frames ?? []).map((f) => f.dataUrl || "").filter(Boolean);
      notice.value = `Captured ${out?.count ?? frameURLs.value.length} frame(s)`;
      return;
    }
    if (extensionId.value === "org.fleet.spectrogram") {
      const res = await extensionRPC(extensionId.value, "ext.spectrogram.run", {
        hash: String(values.hash ?? ""),
        index: Number(values.index) || 0,
        path: String(values.path ?? ""),
        title: String(values.title ?? ""),
        width: Number(values.width) || 0,
      });
      const out = res.result as { image?: { dataUrl?: string; bytes?: number } };
      const url = out?.image?.dataUrl || "";
      frameURLs.value = url ? [url] : [];
      if (url) {
        setNotice(`Spectrogram ready (${out?.image?.bytes ?? 0} bytes)`, "ok", url);
      } else {
        applySaveNotice(res.result, "Spectrogram");
      }
      return;
    }
    if (extensionId.value === "org.fleet.rutracker_check") {
      const res = await extensionRPC(extensionId.value, "ext.rutracker_check.check", {
        hashesText: String(values.hashesText ?? ""),
        replace: Boolean(values.replace),
      });
      const out = res.result as { count?: number; results?: { hash: string; status: string; message?: string }[] };
      const lines = [`Checked ${out?.count ?? 0}`];
      for (const r of out?.results ?? []) {
        lines.push(`${r.hash}: ${r.status}${r.message ? ` (${r.message})` : ""}`);
      }
      notice.value = lines.join("\n");
      return;
    }
    if (extensionId.value === "org.fleet.telegram") {
      const res = await extensionRPC(extensionId.value, "ext.telegram.saveSettings", {
        enabled: Boolean(values.enabled),
        botToken: String(values.botToken ?? ""),
        clearToken: Boolean(values.clearToken),
        chatId: String(values.chatId ?? ""),
        notifyAdded: Boolean(values.notifyAdded),
        notifyFinished: Boolean(values.notifyFinished),
        notifyRemoved: Boolean(values.notifyRemoved),
        intervalSec: Number(values.intervalSec) || 15,
        templateAdded: String(values.templateAdded ?? ""),
        templateFinished: String(values.templateFinished ?? ""),
        templateRemoved: String(values.templateRemoved ?? ""),
      });
      applySaveNotice(res.result, "Telegram saved");
      return;
    }
    if (extensionId.value === "org.fleet.xmpp") {
      const res = await extensionRPC(extensionId.value, "ext.xmpp.saveSettings", {
        enabled: Boolean(values.enabled),
        jid: String(values.jid ?? ""),
        password: String(values.password ?? ""),
        clearPassword: Boolean(values.clearPassword),
        to: String(values.to ?? ""),
        host: String(values.host ?? ""),
        port: Number(values.port) || 0,
        useTLS: Boolean(values.useTLS),
        intervalSec: Number(values.intervalSec) || 15,
        message: String(values.message ?? ""),
      });
      applySaveNotice(res.result, "XMPP saved");
      return;
    }
    if (extensionId.value === "org.fleet.discord") {
      const res = await extensionRPC(extensionId.value, "ext.discord.saveSettings", {
        enabled: Boolean(values.enabled),
        webhookURL: String(values.webhookURL ?? ""),
        clearWebhook: Boolean(values.clearWebhook),
        mention: String(values.mention ?? ""),
        notifyAdded: Boolean(values.notifyAdded),
        notifyFinished: Boolean(values.notifyFinished),
        notifyRemoved: Boolean(values.notifyRemoved),
        intervalSec: Number(values.intervalSec) || 15,
        templateAdded: String(values.templateAdded ?? ""),
        templateFinished: String(values.templateFinished ?? ""),
        templateRemoved: String(values.templateRemoved ?? ""),
      });
      applySaveNotice(res.result, "Discord saved");
      return;
    }
    if (extensionId.value === "org.fleet.ratiocolors") {
      const res = await extensionRPC(extensionId.value, "ext.ratiocolors.saveSettings", {
        enabled: Boolean(values.enabled),
        lowMax: Number(values.lowMax) || 1,
        midMax: Number(values.midMax) || 2,
      });
      applySaveNotice(res.result, "Ratio Color saved");
      return;
    }
    if (extensionId.value === "org.fleet.filemanager") {
      const formId = String(values.__formId ?? "");
      if (formId === "fm-mkdir") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.mkdir", {
          path: String(values.path ?? ""),
        });
        applySaveNotice(res.result, "Directory created");
        return;
      }
      if (formId === "fm-rename") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.rename", {
          from: String(values.from ?? ""),
          to: String(values.to ?? ""),
        });
        applySaveNotice(res.result, "Renamed");
        return;
      }
      if (formId === "fm-copy") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.copy", {
          from: String(values.from ?? ""),
          to: String(values.to ?? ""),
        });
        applySaveNotice(res.result, "Copied");
        return;
      }
      if (formId === "fm-remove") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.remove", {
          path: String(values.path ?? ""),
          recursive: Boolean(values.recursive),
        });
        applySaveNotice(res.result, "Removed");
        return;
      }
      if (formId === "fm-bulk") {
        const paths = String(values.pathsText ?? "")
          .split("\n")
          .map((l) => l.trim())
          .filter(Boolean);
        const res = await extensionRPC(extensionId.value, "ext.filemanager.bulk", {
          op: String(values.op ?? ""),
          paths,
          to: String(values.to ?? ""),
          confirm: Boolean(values.confirm),
        });
        applySaveNotice(res.result, "Bulk done");
        return;
      }
      if (formId === "fm-checksum") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.checksum", {
          path: String(values.path ?? ""),
        });
        applySaveNotice(res.result, "Checksum");
        return;
      }
      if (formId === "fm-text") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.readText", {
          path: String(values.path ?? ""),
        });
        if (typeof (res.result as { text?: string })?.text === "string") {
          setNotice("Text", "ok", (res.result as { text: string }).text);
        } else {
          applySaveNotice(res.result, "Text");
        }
        return;
      }
      if (formId === "fm-preview") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.preview", {
          path: String(values.path ?? ""),
        });
        const url = (res.result as { dataUrl?: string })?.dataUrl;
        if (url) frameURLs.value = [url];
        if (url) setNotice("Preview loaded");
        else applySaveNotice(res.result, "Preview");
        return;
      }
      if (formId === "fm-mediainfo") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager.mediainfo", {
          path: String(values.path ?? ""),
        });
        if (typeof (res.result as { text?: string })?.text === "string") {
          setNotice("MediaInfo", "ok", (res.result as { text: string }).text);
        } else {
          applySaveNotice(res.result, "MediaInfo");
        }
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.filemanager.list", {
        path: String(values.path ?? ""),
      });
      applySaveNotice(res.result, "List");
      return;
    }
    if (extensionId.value === "org.fleet.filemanager-media") {
      // Deprecated: use org.fleet.filemanager preview/mediainfo (kept for existing installs).
      const formId = String(values.__formId ?? "");
      if (formId === "fm-preview") {
        const res = await extensionRPC(extensionId.value, "ext.filemanager-media.preview", {
          path: String(values.path ?? ""),
        });
        const url = (res.result as { dataUrl?: string })?.dataUrl;
        if (url) frameURLs.value = [url];
        if (url) setNotice("Preview loaded");
        else applySaveNotice(res.result, "Preview");
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.filemanager-media.mediainfo", {
        path: String(values.path ?? ""),
      });
      if (typeof (res.result as { text?: string })?.text === "string") {
        setNotice("MediaInfo", "ok", (res.result as { text: string }).text);
      } else {
        applySaveNotice(res.result, "MediaInfo");
      }
      return;
    }
    if (extensionId.value === "org.fleet.filemanager-share") {
      const formId = String(values.__formId ?? "");
      if (formId === "share-revoke") {
        await deleteShare(String(values.token ?? ""));
        notice.value = "Share revoked";
        return;
      }
      const res = await createShare({
        path: String(values.path ?? ""),
        ttlSec: Number(values.ttlSec) || 0,
        maxDownloads: Number(values.maxDownloads) || 0,
      });
      notice.value = `Share created: ${res.url}`;
      return;
    }
    if (extensionId.value === "org.fleet.rclone") {
      const cancelId = String(values.cancelJobId ?? "").trim();
      if (cancelId) {
        const res = await extensionRPC(extensionId.value, "ext.rclone.cancelJob", { id: cancelId });
        applySaveNotice(res.result, "Cancelled");
        return;
      }
    }
    if (extensionId.value === "org.fleet.metrics") {
      const res = await extensionRPC(extensionId.value, "ext.metrics.saveSettings", {
        enabled: Boolean(values.enabled),
        listenAddr: String(values.listenAddr ?? "127.0.0.1:19105"),
        bearerToken: String(values.bearerToken ?? ""),
        intervalSec: Number(values.intervalSec) || 15,
      });
      applySaveNotice(res.result, "Metrics saved");
      return;
    }
    if (extensionId.value === "org.fleet.diskcleanup") {
      const formId = String(values.__formId ?? "");
      if (formId === "janitor-apply") {
        const res = await extensionRPC(extensionId.value, "ext.diskcleanup.applySuggested", {
          hash: String(values.hash ?? ""),
          action: String(values.action ?? ""),
          confirm: Boolean(values.confirm),
        });
        applySaveNotice(res.result, "Apply done");
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.diskcleanup.saveSettings", {
        enabled: Boolean(values.enabled),
        intervalSec: Number(values.intervalSec) || 60,
        stalledSec: Number(values.stalledSec) || 1800,
        scanOrphans: Boolean(values.scanOrphans),
        orphanMaxEntries: Number(values.orphanMaxEntries) || 200,
        orphanMaxDepth: Number(values.orphanMaxDepth) || 3,
        notifyURL: String(values.notifyURL ?? ""),
        notifyOnChange: Boolean(values.notifyOnChange),
      });
      applySaveNotice(res.result, "Janitor saved");
      return;
    }
    if (extensionId.value === "org.fleet.duplicates") {
      const formId = String(values.__formId ?? "");
      if (formId === "dupes-hardlink") {
        const res = await extensionRPC(extensionId.value, "ext.duplicates.applyHardlink", {
          from: String(values.from ?? ""),
          to: String(values.to ?? ""),
          confirm: Boolean(values.confirm),
        });
        applySaveNotice(res.result, "Hardlink done");
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.duplicates.saveSettings", {
        enabled: Boolean(values.enabled),
        intervalSec: Number(values.intervalSec) || 120,
        fingerprintEnabled: Boolean(values.fingerprintEnabled),
        fingerprintBytes: Number(values.fingerprintBytes) || 65536,
      });
      applySaveNotice(res.result, "Dupes saved");
      return;
    }
    if (extensionId.value === "org.fleet.nfo") {
      const formId = String(values.__formId ?? "");
      if (formId === "nfo-view") {
        const res = await extensionRPC(extensionId.value, "ext.nfo.view", {
          hash: String(values.hash ?? ""),
          path: String(values.path ?? ""),
        });
        const result = (res.result ?? {}) as { text?: string; path?: string; encoding?: string };
        nfoText.value = result.text ?? "";
        notice.value = `NFO ${result.path ?? ""} (${result.encoding ?? ""})`;
        return;
      }
      const res = await extensionRPC(extensionId.value, "ext.nfo.saveSettings", {
        maxDepth: Number(values.maxDepth) || 4,
        maxFiles: Number(values.maxFiles) || 50,
      });
      applySaveNotice(res.result, "NFO saved");
      return;
    }
    if (extensionId.value === "org.fleet.portwatch") {
      const res = await extensionRPC(extensionId.value, "ext.portwatch.saveSettings", {
        enabled: Boolean(values.enabled),
        intervalSec: Number(values.intervalSec) || 60,
        downMinutes: Number(values.downMinutes) || 5,
        checkHost: String(values.checkHost ?? "127.0.0.1"),
        notifyURL: String(values.notifyURL ?? ""),
        alertCooldownMin: Number(values.alertCooldownMin) || 30,
        notifyRecovery: Boolean(values.notifyRecovery),
      });
      applySaveNotice(res.result, "Port watch saved");
      return;
    }
    setNotice("Form submitted", "ok", JSON.stringify(values, null, 2));
  } catch (e) {
    setError(e);
  } finally {
    busy.value = false;
  }
}

async function onAction(actionId: string, label: string, needsConfirm = false) {
  if (needsConfirm) {
    const ok = window.confirm(`Confirm: ${label}?`);
    if (!ok) return;
  }
  busy.value = true;
  clearNotice();
  frameURLs.value = [];
  try {
    if (extensionId.value === "org.fleet.rssfeeds" && actionId === "refresh-feeds") {
      const res = await extensionRPC(extensionId.value, "ext.rssfeeds.poll", {});
      const out = res.result as {
        added?: number;
        feeds?: {
          url?: string;
          lastOk?: string;
          lastError?: string;
          itemsSeen?: number;
          lastMatchedRule?: string;
        }[];
      };
      const lines = (out.feeds ?? []).map((f) => {
        const err = f.lastError ? ` error=${f.lastError}` : "";
        const rule = f.lastMatchedRule ? ` rule=${f.lastMatchedRule}` : "";
        return `${f.url}: ok=${f.lastOk || "-"} seen=${f.itemsSeen ?? 0}${rule}${err}`;
      });
      setNotice(`Poll · added ${out.added ?? 0}`, "ok", lines.join("\n") || undefined);
      return;
    }
    if (extensionId.value === "org.fleet.rssfeeds" && actionId === "show-status") {
      const res = await extensionRPC(extensionId.value, "ext.rssfeeds.list", {});
      const feeds = (res.result as { feeds?: {
        url?: string;
        enabled?: boolean;
        loginAccount?: string;
        lastOk?: string;
        lastError?: string;
        itemsSeen?: number;
        lastMatchedRule?: string;
        intervalSec?: number;
      }[] })?.feeds ?? [];
      if (!feeds.length) {
        setNotice("No RSS feeds configured.");
        return;
      }
      setNotice(
        `${feeds.length} feed(s)`,
        "ok",
        feeds.map((f) => {
          const acc = f.loginAccount ? ` login=${f.loginAccount}` : "";
          const err = f.lastError ? ` error=${f.lastError}` : "";
          const rule = f.lastMatchedRule ? ` rule=${f.lastMatchedRule}` : "";
          return `${f.enabled === false ? "[off] " : ""}${f.url} every=${f.intervalSec ?? 300}s seen=${f.itemsSeen ?? 0} ok=${f.lastOk || "-"}${acc}${rule}${err}`;
        }).join("\n"),
      );
      return;
    }
    if (extensionId.value === "org.fleet.scheduler" && actionId === "apply-now") {
      const res = await extensionRPC(extensionId.value, "ext.scheduler.apply", {});
      applySaveNotice(res.result, "Apply done");
      return;
    }
    if (extensionId.value === "org.fleet.scheduler" && actionId === "show-status") {
      const res = await extensionRPC(extensionId.value, "ext.scheduler.status", {});
      applySaveNotice(res.result, "Status");
      return;
    }
    if (extensionId.value === "org.fleet.ratio" && actionId === "evaluate-now") {
      const res = await extensionRPC(extensionId.value, "ext.ratio.evaluate", {});
      applySaveNotice(res.result, "Evaluate done");
      return;
    }
    if (extensionId.value === "org.fleet.ratio" && actionId === "preview") {
      const res = await extensionRPC(extensionId.value, "ext.ratio.preview", {});
      applySaveNotice(res.result, "Preview");
      return;
    }
    if (extensionId.value === "org.fleet.trackerreplace" && actionId === "apply-all") {
      const res = await extensionRPC(extensionId.value, "ext.trackerreplace.apply", {});
      applySaveNotice(res.result, "Apply done");
      return;
    }
    if (extensionId.value === "org.fleet.trackerreplace" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.trackerreplace.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.bandwidth" && (actionId === "set-limits" || actionId === "clear-limits")) {
      setNotice(t("ctx.bandwidth.help"), "ok");
      return;
    }
    if (extensionId.value === "org.fleet.titlesearch" && actionId === "reset-defaults") {
      const res = await extensionRPC(extensionId.value, "ext.titlesearch.resetDefaults", {});
      applySaveNotice(res.result, "Defaults restored");
      return;
    }
    if (extensionId.value === "org.fleet.rssfeedsrewrite" && actionId === "clear-rules") {
      const res = await extensionRPC(extensionId.value, "ext.rssrewrite.clearRules", {});
      applySaveNotice(res.result, "Rules cleared");
      return;
    }
    if (extensionId.value === "org.fleet.cookies" && actionId === "clear-cookies") {
      const res = await extensionRPC(extensionId.value, "ext.cookies.clearHosts", {});
      applySaveNotice(res.result, "Cookies cleared");
      return;
    }
    if (extensionId.value === "org.fleet.trackerlogin" && actionId === "clear-accounts") {
      const res = await extensionRPC(extensionId.value, "ext.trackerlogin.clearAccounts", {});
      applySaveNotice(res.result, "Accounts cleared");
      return;
    }
    if (extensionId.value === "org.fleet.websearch" && actionId === "reset-defaults") {
      const res = await extensionRPC(extensionId.value, "ext.websearch.resetDefaults", {});
      applySaveNotice(res.result, "Defaults restored");
      return;
    }
    if (extensionId.value === "org.fleet.ratiorules" && actionId === "apply-now") {
      const res = await extensionRPC(extensionId.value, "ext.ratiorules.apply", {});
      applySaveNotice(res.result, "Apply done");
      return;
    }
    if (extensionId.value === "org.fleet.ratiorules" && actionId === "preview") {
      const res = await extensionRPC(extensionId.value, "ext.ratiorules.preview", {});
      applySaveNotice(res.result, "Preview");
      return;
    }
    if (extensionId.value === "org.fleet.rutracker_check" && actionId === "check-now") {
      const res = await extensionRPC(extensionId.value, "ext.rutracker_check.check", {});
      applySaveNotice(res.result, "Check done");
      return;
    }
    if (extensionId.value === "org.fleet.rutracker_check" && actionId === "reset-defaults") {
      const res = await extensionRPC(extensionId.value, "ext.rutracker_check.resetDefaults", {});
      applySaveNotice(res.result, "Defaults restored");
      return;
    }
    if (extensionId.value === "org.fleet.telegram" && actionId === "test-message") {
      const res = await extensionRPC(extensionId.value, "ext.telegram.test", {});
      applySaveNotice(res.result, "Test sent");
      return;
    }
    if (extensionId.value === "org.fleet.telegram" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.telegram.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.xmpp" && actionId === "test-message") {
      const res = await extensionRPC(extensionId.value, "ext.xmpp.test", {});
      applySaveNotice(res.result, "Test sent");
      return;
    }
    if (extensionId.value === "org.fleet.xmpp" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.xmpp.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.discord" && actionId === "test-message") {
      const res = await extensionRPC(extensionId.value, "ext.discord.test", {});
      applySaveNotice(res.result, "Test sent");
      return;
    }
    if (extensionId.value === "org.fleet.discord" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.discord.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.ratiocolors" && actionId === "reset-defaults") {
      const res = await extensionRPC(extensionId.value, "ext.ratiocolors.resetDefaults", {});
      applySaveNotice(res.result, "Defaults restored");
      return;
    }
    if (extensionId.value === "org.fleet.publicthrottle" && actionId === "apply-now") {
      const res = await extensionRPC(extensionId.value, "ext.publicthrottle.apply", {});
      applySaveNotice(res.result, "Apply done");
      return;
    }
    if (extensionId.value === "org.fleet.publicthrottle" && actionId === "preview") {
      const res = await extensionRPC(extensionId.value, "ext.publicthrottle.preview", {});
      applySaveNotice(res.result, "Preview");
      return;
    }
    if (extensionId.value === "org.fleet.publicthrottle" && actionId === "show-audit") {
      const res = await extensionRPC(extensionId.value, "ext.publicthrottle.getAudit", {});
      applySaveNotice(res.result, "Audit");
      return;
    }
    if (extensionId.value === "org.fleet.filemanager-share" && actionId === "list-shares") {
      const data = await fetchShares();
      applySaveNotice({ shares: data.shares }, "Shares");
      return;
    }
    if (extensionId.value === "org.fleet.notifications" && actionId === "test-channel") {
      const res = await extensionRPC(extensionId.value, "ext.notifications.test", {});
      applySaveNotice(res.result, "Test sent");
      return;
    }
    if (extensionId.value === "org.fleet.notifications" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.notifications.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.servarrbridge" && actionId === "rotate-token") {
      const res = await extensionRPC(extensionId.value, "ext.servarrbridge.rotateToken", {});
      applySaveNotice(res.result, "Token rotated");
      return;
    }
    if (extensionId.value === "org.fleet.servarrbridge" && actionId === "list-journal") {
      const res = await extensionRPC(extensionId.value, "ext.servarrbridge.listJournal", {});
      applySaveNotice(res.result, "Journal");
      return;
    }
    if (extensionId.value === "org.fleet.servarrbridge" && actionId === "clear-journal") {
      const res = await extensionRPC(extensionId.value, "ext.servarrbridge.clearJournal", {});
      applySaveNotice(res.result, "Journal cleared");
      return;
    }
    if (extensionId.value === "org.fleet.nzbbridge" && actionId === "rotate-token") {
      const res = await extensionRPC(extensionId.value, "ext.nzbbridge.rotateToken", {});
      applySaveNotice(res.result, "Token rotated");
      return;
    }
    if (extensionId.value === "org.fleet.nzbbridge" && actionId === "list-journal") {
      const res = await extensionRPC(extensionId.value, "ext.nzbbridge.listJournal", {});
      applySaveNotice(res.result, "Journal");
      return;
    }
    if (extensionId.value === "org.fleet.nzbbridge" && actionId === "clear-journal") {
      const res = await extensionRPC(extensionId.value, "ext.nzbbridge.clearJournal", {});
      applySaveNotice(res.result, "Journal cleared");
      return;
    }
    if (extensionId.value === "org.fleet.rclone" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.rclone.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.rclone" && actionId === "list-jobs") {
      const res = await extensionRPC(extensionId.value, "ext.rclone.listJobs", {});
      applySaveNotice(res.result, "Running jobs");
      return;
    }
    if (extensionId.value === "org.fleet.rclone" && actionId === "list-journal") {
      const res = await extensionRPC(extensionId.value, "ext.rclone.listJournal", {});
      applySaveNotice(res.result, "Journal");
      return;
    }
    if (extensionId.value === "org.fleet.rclone" && actionId === "clear-journal") {
      const res = await extensionRPC(extensionId.value, "ext.rclone.clearJournal", {});
      applySaveNotice(res.result, "Journal cleared");
      return;
    }
    if (extensionId.value === "org.fleet.traffic" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.traffic.scan", {});
      applySaveNotice(res.result, "Traffic collect");
      return;
    }
    if (extensionId.value === "org.fleet.traffic" && (actionId === "show-day" || actionId === "show-month" || actionId === "show-year")) {
      const mode = actionId === "show-day" ? "day" : actionId === "show-month" ? "month" : "year";
      const res = await extensionRPC(extensionId.value, "ext.traffic.get", { mode });
      applySaveNotice(res.result, `Traffic ${mode}`);
      return;
    }
    if (extensionId.value === "org.fleet.traffic" && actionId === "show-trackers") {
      const res = await extensionRPC(extensionId.value, "ext.traffic.getTrackers", {});
      const count = (res.result as { count?: number })?.count;
      applySaveNotice(res.result, typeof count === "number" ? `Trackers · ${count}` : "Trackers");
      return;
    }
    if (extensionId.value === "org.fleet.traffic" && (actionId === "export-json" || actionId === "export-csv")) {
      const format = actionId === "export-json" ? "json" : "csv";
      const res = await extensionRPC(extensionId.value, "ext.traffic.export", { format, scope: "all" });
      const r = (res.result ?? {}) as { content?: string; filename?: string };
      setNotice(`Export ${r.filename ?? format}`, "ok", typeof r.content === "string" ? r.content : JSON.stringify(res.result, null, 2));
      return;
    }
    if (extensionId.value === "org.fleet.traffic" && actionId === "clear-stats") {
      const res = await extensionRPC(extensionId.value, "ext.traffic.clear", {});
      applySaveNotice(res.result, "Traffic cleared");
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.automation.scan", {});
      applySaveNotice(res.result, "Rules scan");
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "list-log") {
      const res = await extensionRPC(extensionId.value, "ext.automation.listLog", {});
      const entries = ((res.result as { entries?: Array<Record<string, unknown>> })?.entries ?? []) as Array<Record<string, unknown>>;
      const lines = [
        "time\truleId\thash\tactions\tdryRun\terror",
        ...entries.map((e) =>
          [
            e.time ?? "",
            e.ruleId ?? "",
            e.hash ?? "",
            Array.isArray(e.actions) ? (e.actions as string[]).join(",") : "",
            e.dryRun ? "yes" : "no",
            e.error ?? "",
          ].join("\t"),
        ),
      ];
      setNotice(`Log · ${entries.length} entries (retention 200)`, "ok", lines.join("\n"));
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "clear-log") {
      const res = await extensionRPC(extensionId.value, "ext.automation.clearLog", {});
      applySaveNotice(res.result, "Log cleared");
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "list-hints") {
      const res = await extensionRPC(extensionId.value, "ext.automation.listHints", {});
      applySaveNotice(res.result, "Hints");
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "list-examples") {
      const res = await extensionRPC(extensionId.value, "ext.automation.listExamples", {});
      applySaveNotice(res.result, "Examples");
      return;
    }
    if (extensionId.value === "org.fleet.automation" && actionId === "append-examples") {
      const res = await extensionRPC(extensionId.value, "ext.automation.appendExamples", {});
      applySaveNotice(res.result, "Examples appended");
      const forms = resolved.value?.forms ?? [];
      await Promise.all(forms.filter((f) => f.loadMethod).map((f) => loadFormPrefill(f)));
      syncDslDraftsFromInitials();
      return;
    }
    if (extensionId.value === "org.fleet.metrics" && actionId === "preview") {
      const res = await extensionRPC(extensionId.value, "ext.metrics.preview", {});
      applySaveNotice(res.result, "Preview");
      await refreshMetricsStatus();
      return;
    }
    if (extensionId.value === "org.fleet.metrics" && actionId === "scrape-url") {
      const res = await extensionRPC(extensionId.value, "ext.metrics.scrapeURL", {});
      applySaveNotice(res.result, "Scrape URL");
      await refreshMetricsStatus();
      return;
    }
    if (extensionId.value === "org.fleet.history" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.history.scan", {});
      applySaveNotice(res.result, "History scan");
      await refreshHistoryEvents();
      return;
    }
    if (extensionId.value === "org.fleet.history" && actionId === "list-log") {
      await refreshHistoryEvents();
      setNotice(
        historyEvents.value.length
          ? t("obs.history.count", { count: String(historyEvents.value.length) })
          : t("obs.history.empty"),
        "ok",
      );
      return;
    }
    if (extensionId.value === "org.fleet.history" && actionId === "export-csv") {
      const res = await extensionRPC(extensionId.value, "ext.history.export", {
        format: "csv",
        action: historyActionFilter.value,
      });
      const r = (res.result ?? {}) as { content?: string; filename?: string; count?: number };
      const content = typeof r.content === "string" ? r.content : "";
      const filename = typeof r.filename === "string" ? r.filename : "fleet-history.csv";
      downloadBlob(filename, content, "text/csv;charset=utf-8");
      setNotice(
        t("obs.history.exported", {
          filename,
          count: String(typeof r.count === "number" ? r.count : historyEvents.value.length),
        }),
        "ok",
      );
      return;
    }
    if (extensionId.value === "org.fleet.history" && actionId === "clear-log") {
      const res = await extensionRPC(extensionId.value, "ext.history.clear", {});
      applySaveNotice(res.result, "History cleared");
      historyEvents.value = [];
      return;
    }
    if (extensionId.value === "org.fleet.diskcleanup" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.diskcleanup.scan", {});
      applySaveNotice(res.result, "Scan done");
      const entries = (res.result as { entries?: JanitorReportEntry[] } | undefined)?.entries;
      if (Array.isArray(entries)) janitorReport.value = entries;
      return;
    }
    if (extensionId.value === "org.fleet.diskcleanup" && actionId === "list-report") {
      const res = await extensionRPC(extensionId.value, "ext.diskcleanup.listReport", {});
      const entries = (res.result as { entries?: JanitorReportEntry[] } | undefined)?.entries ?? [];
      janitorReport.value = Array.isArray(entries) ? entries : [];
      setNotice(
        janitorReport.value.length
          ? t("obs.janitor.reportCount", { count: String(janitorReport.value.length) })
          : t("obs.janitor.reportEmpty"),
      );
      return;
    }
    if (extensionId.value === "org.fleet.diskcleanup" && actionId === "clear-report") {
      const res = await extensionRPC(extensionId.value, "ext.diskcleanup.clearReport", {});
      janitorReport.value = [];
      applySaveNotice(res.result, "Report cleared");
      return;
    }
    if (extensionId.value === "org.fleet.duplicates" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.duplicates.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.duplicates" && actionId === "list-report") {
      const res = await extensionRPC(extensionId.value, "ext.duplicates.listReport", {});
      applySaveNotice(res.result, "Report");
      return;
    }
    if (extensionId.value === "org.fleet.duplicates" && actionId === "clear-report") {
      const res = await extensionRPC(extensionId.value, "ext.duplicates.clearReport", {});
      applySaveNotice(res.result, "Report cleared");
      return;
    }
    if (extensionId.value === "org.fleet.labelbridge" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.labelbridge.scan", {});
      applySaveNotice(res.result, "Label Sync scan");
      return;
    }
    if (extensionId.value === "org.fleet.labelbridge" && actionId === "show-status") {
      const res = await extensionRPC(extensionId.value, "ext.labelbridge.getStatus", {});
      applySaveNotice(res.result, "Pair status");
      return;
    }
    if (extensionId.value === "org.fleet.labelbridge" && actionId === "list-log") {
      const res = await extensionRPC(extensionId.value, "ext.labelbridge.listLog", {});
      const entries = ((res.result as { entries?: Array<Record<string, unknown>> })?.entries ?? []) as Array<
        Record<string, unknown>
      >;
      const lines = [
        "time\thash\tclientId\tlabel\treason\tskipped\terror",
        ...entries.map((e) =>
          [e.time, e.hash, e.clientId, e.label, e.reason, e.skipped ? "yes" : "", e.error ?? ""].join("\t"),
        ),
      ];
      setNotice(`Log · ${entries.length}`, "ok", lines.join("\n"));
      return;
    }
    if (extensionId.value === "org.fleet.labelbridge" && actionId === "clear-log") {
      const res = await extensionRPC(extensionId.value, "ext.labelbridge.clearLog", {});
      applySaveNotice(res.result, "Log cleared");
      return;
    }
    if (extensionId.value === "org.fleet.portwatch" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.portwatch.scan", {});
      applySaveNotice(res.result, "Scan done");
      await refreshPortwatchHistory();
      return;
    }
    if (extensionId.value === "org.fleet.portwatch" && actionId === "show-history") {
      await refreshPortwatchHistory();
      setNotice(
        t("obs.portwatch.historyCount", {
          days: String(portwatchDays.value.length),
          samples: String(portwatchSampleCount.value),
        }),
        "ok",
      );
      return;
    }
    if (extensionId.value === "org.fleet.portwatch" && actionId === "list-log") {
      const res = await extensionRPC(extensionId.value, "ext.portwatch.listLog", {});
      applySaveNotice(res.result, "Log");
      return;
    }
    if (extensionId.value === "org.fleet.portwatch" && actionId === "clear-log") {
      const res = await extensionRPC(extensionId.value, "ext.portwatch.clearLog", {});
      applySaveNotice(res.result, "Log cleared");
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.autodownload.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "restart-irc") {
      const res = await extensionRPC(extensionId.value, "ext.autodownload.restartIRC", {});
      applySaveNotice(res.result, "IRC restarted");
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "list-grabs") {
      const res = await extensionRPC(extensionId.value, "ext.autodownload.listGrabs", { limit: 50 });
      const grabs = (res.result as {
        grabs?: {
          ts?: string;
          title?: string;
          ok?: boolean;
          dryRun?: boolean;
          source?: string;
          filterId?: string;
          error?: string;
          infohash?: string;
        }[];
      })?.grabs ?? [];
      if (!grabs.length) {
        setNotice("No grabs recorded.");
        return;
      }
      setNotice(
        `Grabs · ${grabs.length}`,
        "ok",
        grabs.map((g) => {
          const flag = g.dryRun ? "dry" : g.ok ? "ok" : "fail";
          const err = g.error ? ` err=${g.error}` : "";
          return `${g.ts || "-"} [${flag}] ${g.source || "?"} ${g.title || g.infohash || "-"} filter=${g.filterId || "-"}${err}`;
        }).join("\n"),
      );
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "irc-status") {
      const res = await extensionRPC(extensionId.value, "ext.autodownload.ircStatus", {});
      const servers = (res.result as {
        servers?: {
          id?: string;
          host?: string;
          enabled?: boolean;
          ok?: boolean;
          detail?: string;
          lastAnnounceAt?: string;
          lastError?: string;
        }[];
      })?.servers ?? [];
      if (!servers.length) {
        setNotice("No IRC status yet (save servers and Restart IRC).");
        return;
      }
      setNotice(
        `IRC · ${servers.length} server(s)`,
        "ok",
        servers.map((s) => {
          const en = s.enabled === false ? "off" : "on";
          const ok = s.ok ? "up" : "down";
          const last = s.lastAnnounceAt ? ` last=${s.lastAnnounceAt}` : "";
          const err = s.lastError ? ` err=${s.lastError}` : "";
          return `${s.id || "-"} ${s.host || "-"} [${en}/${ok}] ${s.detail || ""}${last}${err}`;
        }).join("\n"),
      );
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "test-filter") {
      notice.value = "Open AutoDL Configure to run Test filter (title + optional filter/server/channel).";
      return;
    }
    if (extensionId.value === "org.fleet.autodownload" && actionId === "export-config") {
      const res = await extensionRPC(extensionId.value, "ext.autodownload.exportConfig", { includeSecrets: false });
      setNotice("Config export", "ok", JSON.stringify(res.result ?? {}, null, 2));
      return;
    }
    if (extensionId.value === "org.fleet.crossseed" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.crossseed.scan", {});
      applySaveNotice(res.result, "Scan done");
      return;
    }
    if (extensionId.value === "org.fleet.postaddcheck" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.postaddcheck.scan", {});
      applySaveNotice(res.result, "Add check scan");
      return;
    }
    if (extensionId.value === "org.fleet.postaddcheck" && actionId === "show-status") {
      const res = await extensionRPC(extensionId.value, "ext.postaddcheck.status", {});
      applySaveNotice(res.result, "Tracked");
      return;
    }
    if (extensionId.value === "org.fleet.blocklist" && actionId === "refresh-now") {
      const res = await extensionRPC(extensionId.value, "ext.blocklist.refresh", {});
      applySaveNotice(res.result, "Blocklist refresh");
      return;
    }
    if (extensionId.value === "org.fleet.blocklist" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.blocklist.scan", {});
      applySaveNotice(res.result, "Blocklist scan");
      return;
    }
    if (extensionId.value === "org.fleet.blocklist" && actionId === "preview") {
      const res = await extensionRPC(extensionId.value, "ext.blocklist.preview", {});
      applySaveNotice(res.result, "Preview");
      return;
    }
    if (extensionId.value === "org.fleet.eventhooks" && actionId === "scan-now") {
      const res = await extensionRPC(extensionId.value, "ext.eventhooks.scan", {});
      applySaveNotice(res.result, "Hooks scan");
      return;
    }
    if (extensionId.value === "org.fleet.eventhooks" && actionId === "test-hook") {
      const res = await extensionRPC(extensionId.value, "ext.eventhooks.test", {});
      applySaveNotice(res.result, "Hooks test");
      return;
    }
    if (extensionId.value === "org.fleet.eventhooks" && actionId === "list-journal") {
      const res = await extensionRPC(extensionId.value, "ext.eventhooks.journal", {});
      applySaveNotice(res.result, "Journal");
      return;
    }
    setNotice(`Action “${label}” (${actionId})`);
  } catch (e) {
    setError(e);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="fleet-ext-config">
    <div
      v-if="loading"
      class="fleet-banner"
    >
      Loading extensions…
    </div>
    <div
      v-else-if="loadError"
      class="fleet-banner"
      data-kind="error"
    >
      {{ loadError }}
      <button
        type="button"
        class="fleet-btn-primary"
        @click="refresh(true)"
      >
        Retry
      </button>
    </div>
    <div
      v-else-if="!resolved"
      class="fleet-banner"
      data-kind="error"
    >
      Unknown extension page.
    </div>
    <template v-else>
      <aside class="fleet-ext-config-side">
        <header class="fleet-ext-header">
          <p class="fleet-ext-eyebrow">
            {{ resolved.extension.name }}
          </p>
          <h2>{{ resolved.page.title }}</h2>
          <p
            v-if="resolved.extension.description"
            class="fleet-ext-note"
          >
            {{ resolved.extension.description }}
          </p>
          <p class="fleet-ext-tip">
            {{ t("extensions.configTip") }}
          </p>
        </header>
        <div
          v-if="globalActions.length"
          class="fleet-ext-actions"
        >
          <button
            v-for="a in globalActions"
            :key="a.id"
            type="button"
            :class="actionBtnClass(a)"
            :disabled="busy"
            @click="onAction(a.id, a.label, Boolean(a.confirm))"
          >
            {{ a.label }}
          </button>
        </div>
      </aside>

      <div class="fleet-ext-config-main">
        <div
          v-if="notice"
          class="fleet-banner"
          :data-kind="noticeKind"
        >
          <div class="fleet-ext-notice-summary">
            {{ notice }}
          </div>
          <details
            v-if="noticeDetail"
            class="fleet-ext-notice-detail"
          >
            <summary>{{ t("extensions.details") }}</summary>
            <pre>{{ noticeDetail }}</pre>
          </details>
        </div>

        <div
          v-if="extensionId === 'org.fleet.metrics' && metricsStatus"
          class="fleet-obs-status"
        >
          <h4>{{ t("obs.metrics.status") }}</h4>
          <dl class="fleet-obs-status-grid">
            <div>
              <dt>{{ t("obs.metrics.scrapeURL") }}</dt>
              <dd>
                <code>{{ metricsStatus.scrapeURL || "—" }}</code>
              </dd>
            </div>
            <div>
              <dt>{{ t("obs.metrics.listening") }}</dt>
              <dd>
                {{
                  metricsStatus.listening
                    ? t("obs.metrics.listeningOn")
                    : t("obs.metrics.listeningOff")
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t("obs.metrics.lastSuccess") }}</dt>
              <dd>{{ metricsStatus.lastSuccessAt || t("obs.metrics.never") }}</dd>
            </div>
            <div v-if="metricsStatus.lastError">
              <dt>{{ t("obs.metrics.lastError") }}</dt>
              <dd class="fleet-obs-error">
                {{ metricsStatus.lastError }}
              </dd>
            </div>
          </dl>
        </div>

        <div
          v-if="extensionId === 'org.fleet.diskcleanup' && janitorReport.length"
          class="fleet-obs-report"
        >
          <h4>{{ t("obs.janitor.report") }}</h4>
          <ul class="fleet-obs-report-list">
            <li
              v-for="(entry, idx) in janitorReport"
              :key="`${entry.kind}-${entry.hash || entry.directory || idx}`"
              class="fleet-obs-report-row"
            >
              <div class="fleet-obs-report-meta">
                <strong>{{ entry.kind || "?" }}</strong>
                <span>{{ entry.name || entry.hash || entry.directory || "—" }}</span>
                <span
                  v-if="entry.label"
                  class="fleet-ext-note"
                >label:{{ entry.label }}</span>
                <span
                  v-if="entry.directory"
                  class="fleet-ext-note"
                >{{ entry.directory }}</span>
              </div>
              <div class="fleet-obs-report-actions">
                <button
                  type="button"
                  class="fleet-btn"
                  :disabled="!entry.directory"
                  @click="openJanitorPath(entry.directory || '')"
                >
                  {{ t("obs.janitor.openPath") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn"
                  :disabled="!entry.label"
                  @click="filterJanitorLabel(entry.label || '')"
                >
                  {{ t("obs.janitor.filterLabel") }}
                </button>
              </div>
            </li>
          </ul>
        </div>

        <div
          v-if="frameURLs.length"
          class="fleet-ext-frames"
        >
          <img
            v-for="(src, i) in frameURLs"
            :key="i"
            :src="src"
            :alt="`Frame ${i + 1}`"
          >
        </div>
        <pre
          v-if="nfoText"
          class="fleet-nfo"
        >{{ nfoText }}</pre>

        <DeclarativeForm
          v-for="(form, formIdx) in displayForms"
          :key="form.id"
          :form="form"
          :initial-values="formInitials[form.id] ?? null"
          @submit="onFormSubmit"
        >
          <!-- Settings fields first; slots / history / Editor, then Apply. -->
          <template
            v-if="isChromeHostForm(form, formIdx, displayForms.length)"
            #before-submit
          >
            <SchedulerSlotsBuilder
              v-if="showSchedulerChrome"
              v-model="scheduleSlots"
            />
            <div
              v-if="extensionId === 'org.fleet.history'"
              class="fleet-obs-report"
            >
              <h4>{{ t("obs.history.title") }}</h4>
              <div class="fleet-obs-report-actions fleet-obs-history-toolbar">
                <label class="fleet-obs-history-filter">
                  <span>{{ t("obs.history.filter") }}</span>
                  <select
                    v-model="historyActionFilter"
                    class="fleet-decl-control"
                    @change="refreshHistoryEvents"
                  >
                    <option value="">
                      {{ t("obs.history.filterAll") }}
                    </option>
                    <option value="addition">
                      {{ t("obs.history.filterAddition") }}
                    </option>
                    <option value="finish">
                      {{ t("obs.history.filterFinish") }}
                    </option>
                    <option value="deletion">
                      {{ t("obs.history.filterDeletion") }}
                    </option>
                  </select>
                </label>
                <button
                  type="button"
                  class="fleet-btn"
                  :disabled="busy"
                  @click="onAction('list-log', 'Show history')"
                >
                  {{ t("obs.history.refresh") }}
                </button>
                <button
                  type="button"
                  class="fleet-btn-primary"
                  :disabled="busy"
                  @click="onAction('export-csv', 'Export CSV')"
                >
                  {{ t("obs.history.exportCsv") }}
                </button>
              </div>
              <p
                v-if="!historyEvents.length"
                class="fleet-ext-note"
              >
                {{ t("obs.history.empty") }}
              </p>
              <ul
                v-else
                class="fleet-obs-report-list"
              >
                <li
                  v-for="(entry, idx) in historyEvents"
                  :key="entry.id || `${entry.hash}-${entry.actionTime}-${idx}`"
                  class="fleet-obs-report-row"
                >
                  <div class="fleet-obs-report-meta">
                    <strong>{{ entry.action || "?" }}</strong>
                    <span>{{ entry.name || entry.hash || "—" }}</span>
                    <span class="fleet-ext-note">{{ formatHistoryTime(entry.actionTime) }}</span>
                    <span
                      v-if="entry.label"
                      class="fleet-ext-note"
                    >label:{{ entry.label }}</span>
                  </div>
                </li>
              </ul>
            </div>
            <div
              v-if="extensionId === 'org.fleet.portwatch'"
              class="fleet-obs-report"
            >
              <h4>{{ t("obs.portwatch.history") }}</h4>
              <p class="fleet-ext-note">
                {{ t("obs.portwatch.historyHint", { samples: String(portwatchSampleCount) }) }}
              </p>
              <ul class="fleet-obs-report-list">
                <li
                  v-for="day in portwatchDays"
                  :key="day.date || 'day'"
                  class="fleet-obs-report-row"
                >
                  <div class="fleet-obs-report-meta">
                    <strong>{{ day.date || "—" }}</strong>
                    <span>{{ t("obs.portwatch.upRatio", { ratio: formatUpRatio(day.upRatio) }) }}</span>
                    <span class="fleet-ext-note">
                      {{ t("obs.portwatch.dayCounts", {
                        up: String(day.upCount ?? 0),
                        down: String(day.downCount ?? 0),
                        samples: String(day.samples ?? 0),
                      }) }}
                    </span>
                  </div>
                  <div
                    class="fleet-obs-portwatch-bar"
                    :title="formatUpRatio(day.upRatio)"
                  >
                    <span
                      class="fleet-obs-portwatch-bar-fill"
                      :style="{ width: `${Math.round((day.upRatio || 0) * 100)}%` }"
                    />
                  </div>
                </li>
              </ul>
            </div>
            <DslEditorChrome
              v-if="showDslChrome && dslKind"
              v-model="dslDraft"
              :kind="dslKind"
              :mode="dslMode"
              :parse-error="dslParseError"
              :expert-label="t(dslExpertLabelKey)"
              @set-mode="setDslMode"
              @parse-error="onDslParseError"
            />
          </template>
        </DeclarativeForm>

        <!-- Chrome-only fallbacks when the host settings form is missing. -->
        <SchedulerSlotsBuilder
          v-if="showSchedulerChrome && !displayForms.some((f) => f.id === 'schedule-enable')"
          v-model="scheduleSlots"
        />
        <DslEditorChrome
          v-if="showDslChrome && dslKind && !displayForms.some((f) => f.id === chromeHostFormId)"
          v-model="dslDraft"
          :kind="dslKind"
          :mode="dslMode"
          :parse-error="dslParseError"
          :expert-label="t(dslExpertLabelKey)"
          @set-mode="setDslMode"
          @parse-error="onDslParseError"
        />

        <p
          v-if="!resolved.forms.length && !showDslChrome && !showSchedulerChrome"
          class="fleet-ext-note"
        >
          This page has no forms yet.
        </p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.fleet-ext-config {
  display: grid;
  grid-template-columns: minmax(13rem, 0.85fr) minmax(0, 1.35fr);
  gap: 1.25rem 1.5rem;
  align-items: start;
}

.fleet-ext-config-side {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
  padding-right: 0.75rem;
  border-right: 1px solid var(--fleet-border-soft, var(--fleet-border));
  min-width: 0;
}

.fleet-ext-config-main {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  min-width: 0;
}

.fleet-ext-header h2 {
  margin: 0.15rem 0;
  font-size: 1.15rem;
}

.fleet-ext-header .fleet-ext-note {
  margin-bottom: 0;
}

.fleet-ext-frames {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin: 0.25rem 0;
}

.fleet-ext-frames img {
  max-width: 240px;
  height: auto;
  border: 1px solid var(--fleet-border, #333);
}

.fleet-nfo {
  margin: 0.25rem 0;
  padding: 0.75rem 1rem;
  max-height: 28rem;
  overflow: auto;
  white-space: pre;
  font-family: var(--fleet-font-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace);
  font-size: 0.8rem;
  line-height: 1.35;
  background: var(--fleet-surface-2, rgba(0, 0, 0, 0.25));
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: 0.35rem;
}

.fleet-obs-status,
.fleet-obs-report {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin: 0 0 0.75rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--fleet-border-soft, var(--fleet-border));
  border-radius: 0.4rem;
  background: color-mix(in srgb, var(--fleet-surface-2, rgba(0, 0, 0, 0.2)) 80%, transparent);
}

.fleet-obs-status h4,
.fleet-obs-report h4 {
  margin: 0;
  font-size: 0.9rem;
}

.fleet-obs-status-grid {
  display: grid;
  gap: 0.4rem 1rem;
  margin: 0;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
}

.fleet-obs-status-grid dt {
  margin: 0;
  font-size: 0.72rem;
  color: var(--fleet-fg-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.fleet-obs-status-grid dd {
  margin: 0.1rem 0 0;
  font-size: 0.85rem;
  word-break: break-all;
}

.fleet-obs-error {
  color: var(--fleet-danger, #e35d6a);
}

.fleet-obs-report-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  max-height: 18rem;
  overflow: auto;
}

.fleet-obs-report-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.fleet-obs-report-meta {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

.fleet-obs-report-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  flex-shrink: 0;
}

.fleet-obs-history-toolbar {
  align-items: end;
  margin-bottom: 0.35rem;
}

.fleet-obs-history-filter {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  font-size: 0.8rem;
  min-width: 9rem;
}

.fleet-obs-history-filter .fleet-decl-control {
  min-width: 9rem;
}

.fleet-obs-portwatch-bar {
  flex: 1 1 6rem;
  min-width: 6rem;
  max-width: 12rem;
  height: 0.55rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--fleet-status-bad, #b42318) 35%, transparent);
  overflow: hidden;
  align-self: center;
}

.fleet-obs-portwatch-bar-fill {
  display: block;
  height: 100%;
  background: var(--fleet-status-ok, #067647);
}

.fleet-ext-notice-summary {
  white-space: pre-wrap;
}

.fleet-ext-notice-detail {
  margin-top: 0.35rem;
  font-size: 0.8rem;
}

.fleet-ext-notice-detail summary {
  cursor: pointer;
  color: var(--fleet-fg-muted);
}

.fleet-ext-notice-detail pre {
  margin: 0.35rem 0 0;
  max-height: 14rem;
  overflow: auto;
  white-space: pre-wrap;
  font-family: var(--fleet-font-mono, ui-monospace, SFMono-Regular, Menlo, Consolas, monospace);
  font-size: 0.75rem;
}

@media (max-width: 720px) {
  .fleet-ext-config {
    grid-template-columns: 1fr;
  }

  .fleet-ext-config-side {
    padding-right: 0;
    padding-bottom: 0.75rem;
    border-right: 0;
    border-bottom: 1px solid var(--fleet-border-soft, var(--fleet-border));
  }
}
</style>
