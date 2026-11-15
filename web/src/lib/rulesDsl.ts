import { parseJsonLines, toJsonLines } from "@/lib/jsonLines";

export type RulesWhen = {
  label?: string;
  labelRegex?: string;
  nameRegex?: string;
  private?: boolean | null;
  complete?: boolean | null;
  trackerHostRegex?: string;
  ratioMin?: number | null;
  ratioMax?: number | null;
  ageSecMin?: number | null;
  statusIn?: string[];
  clientKind?: string;
  clientId?: string;
};

export type RulesThenAction = {
  action: string;
  label?: string;
  name?: string;
  dest?: string;
  upRate?: number;
  downRate?: number;
  url?: string;
  body?: string;
  title?: string;
};

export type RulesRule = {
  id: string;
  priority: number;
  enabled: boolean;
  stopOnMatch: boolean;
  when: RulesWhen;
  then: RulesThenAction[];
};

export const RULE_ACTIONS = [
  "setLabel",
  "moveTo",
  "throttle",
  "pause",
  "stop",
  "notify",
  "crossseedHint",
] as const;

export const CLIENT_KINDS = ["", "rtorrent", "qbittorrent", "deluge", "transmission"] as const;

export function emptyAction(): RulesThenAction {
  return { action: "setLabel", label: "" };
}

export function emptyRule(index = 1): RulesRule {
  return {
    id: `rule-${index}`,
    priority: index * 10,
    enabled: true,
    stopOnMatch: false,
    when: {},
    then: [emptyAction()],
  };
}

function normalizeRule(raw: Record<string, unknown>, index: number): RulesRule {
  const whenRaw = (raw.when && typeof raw.when === "object" ? raw.when : {}) as Record<string, unknown>;
  const thenRaw = Array.isArray(raw.then) ? raw.then : [];
  const enabled = raw.enabled === undefined || raw.enabled === null ? true : Boolean(raw.enabled);
  return {
    id: String(raw.id ?? `rule-${index}`),
    priority: typeof raw.priority === "number" ? raw.priority : index * 10,
    enabled,
    stopOnMatch: Boolean(raw.stopOnMatch),
    when: {
      label: whenRaw.label != null ? String(whenRaw.label) : undefined,
      labelRegex: whenRaw.labelRegex != null ? String(whenRaw.labelRegex) : undefined,
      nameRegex: whenRaw.nameRegex != null ? String(whenRaw.nameRegex) : undefined,
      private: typeof whenRaw.private === "boolean" ? whenRaw.private : null,
      complete: typeof whenRaw.complete === "boolean" ? whenRaw.complete : null,
      trackerHostRegex: whenRaw.trackerHostRegex != null ? String(whenRaw.trackerHostRegex) : undefined,
      ratioMin: typeof whenRaw.ratioMin === "number" ? whenRaw.ratioMin : null,
      ratioMax: typeof whenRaw.ratioMax === "number" ? whenRaw.ratioMax : null,
      ageSecMin: typeof whenRaw.ageSecMin === "number" ? whenRaw.ageSecMin : null,
      statusIn: Array.isArray(whenRaw.statusIn)
        ? whenRaw.statusIn.map(String).filter(Boolean)
        : undefined,
      clientKind: whenRaw.clientKind != null ? String(whenRaw.clientKind) : undefined,
      clientId: whenRaw.clientId != null ? String(whenRaw.clientId) : undefined,
    },
    then: thenRaw.map((a) => {
      const act = (a && typeof a === "object" ? a : {}) as Record<string, unknown>;
      return {
        action: String(act.action ?? "setLabel"),
        label: act.label != null ? String(act.label) : undefined,
        name: act.name != null ? String(act.name) : undefined,
        dest: act.dest != null ? String(act.dest) : undefined,
        upRate: typeof act.upRate === "number" ? act.upRate : undefined,
        downRate: typeof act.downRate === "number" ? act.downRate : undefined,
        url: act.url != null ? String(act.url) : undefined,
        body: act.body != null ? String(act.body) : undefined,
        title: act.title != null ? String(act.title) : undefined,
      };
    }),
  };
}

export function parseRulesText(text: string): { rules: RulesRule[]; error?: string } {
  const { items, error } = parseJsonLines<Record<string, unknown>>(text);
  if (error) return { rules: [], error };
  return { rules: items.map((item, i) => normalizeRule(item, i + 1)) };
}

function cleanWhen(w: RulesWhen): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (w.label?.trim()) out.label = w.label.trim();
  if (w.labelRegex?.trim()) out.labelRegex = w.labelRegex.trim();
  if (w.nameRegex?.trim()) out.nameRegex = w.nameRegex.trim();
  if (w.trackerHostRegex?.trim()) out.trackerHostRegex = w.trackerHostRegex.trim();
  if (w.clientKind?.trim()) out.clientKind = w.clientKind.trim();
  if (w.clientId?.trim()) out.clientId = w.clientId.trim();
  if (w.private === true || w.private === false) out.private = w.private;
  if (w.complete === true || w.complete === false) out.complete = w.complete;
  if (typeof w.ratioMin === "number" && Number.isFinite(w.ratioMin)) out.ratioMin = w.ratioMin;
  if (typeof w.ratioMax === "number" && Number.isFinite(w.ratioMax)) out.ratioMax = w.ratioMax;
  if (typeof w.ageSecMin === "number" && Number.isFinite(w.ageSecMin)) out.ageSecMin = w.ageSecMin;
  if (w.statusIn?.length) out.statusIn = w.statusIn;
  return out;
}

function cleanAction(a: RulesThenAction): Record<string, unknown> {
  const action = String(a.action || "setLabel").trim() || "setLabel";
  const out: Record<string, unknown> = { action };
  if (a.label?.trim()) out.label = a.label.trim();
  if (a.name?.trim()) out.name = a.name.trim();
  if (a.dest?.trim()) out.dest = a.dest.trim();
  if (a.url?.trim()) out.url = a.url.trim();
  if (a.body?.trim()) out.body = a.body.trim();
  if (a.title?.trim()) out.title = a.title.trim();
  if (typeof a.upRate === "number" && Number.isFinite(a.upRate)) out.upRate = a.upRate;
  if (typeof a.downRate === "number" && Number.isFinite(a.downRate)) out.downRate = a.downRate;
  return out;
}

export function serializeRules(rules: RulesRule[]): string {
  return toJsonLines(
    rules.map((r) => {
      const row: Record<string, unknown> = {
        id: r.id.trim() || "rule",
        priority: Number(r.priority) || 0,
        stopOnMatch: Boolean(r.stopOnMatch),
        when: cleanWhen(r.when),
        then: (r.then.length ? r.then : [emptyAction()]).map(cleanAction),
      };
      if (!r.enabled) row.enabled = false;
      return row;
    }),
  );
}
