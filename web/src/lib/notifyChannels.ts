import { parseJsonLines, toJsonLines } from "@/lib/jsonLines";

export type NotifyChannelKind = "ntfy" | "gotify" | "slack" | "pushover" | "email";

export type NotifyChannel = {
  id: string;
  kind: NotifyChannelKind;
  enabled: boolean;
  url?: string;
  token?: string;
  user?: string;
  title?: string;
  smtpHost?: string;
  smtpPort?: number;
  from?: string;
  to?: string;
  password?: string;
};

export const NOTIFY_KINDS: NotifyChannelKind[] = [
  "ntfy",
  "gotify",
  "slack",
  "pushover",
  "email",
];

export function emptyChannel(index = 1): NotifyChannel {
  return {
    id: `ntfy-${index}`,
    kind: "ntfy",
    enabled: true,
    url: "",
  };
}

function normalizeChannel(raw: Record<string, unknown>, index: number): NotifyChannel {
  const kindRaw = String(raw.kind ?? "ntfy").toLowerCase();
  const kind = (NOTIFY_KINDS.includes(kindRaw as NotifyChannelKind)
    ? kindRaw
    : "ntfy") as NotifyChannelKind;
  return {
    id: String(raw.id ?? `${kind}-${index}`),
    kind,
    enabled: raw.enabled === undefined ? true : Boolean(raw.enabled),
    url: raw.url != null ? String(raw.url) : undefined,
    token: raw.token != null ? String(raw.token) : undefined,
    user: raw.user != null ? String(raw.user) : undefined,
    title: raw.title != null ? String(raw.title) : undefined,
    smtpHost: raw.smtpHost != null ? String(raw.smtpHost) : undefined,
    smtpPort: typeof raw.smtpPort === "number" ? raw.smtpPort : undefined,
    from: raw.from != null ? String(raw.from) : undefined,
    to: raw.to != null ? String(raw.to) : undefined,
    password: raw.password != null ? String(raw.password) : undefined,
  };
}

export function parseChannelsText(text: string): { channels: NotifyChannel[]; error?: string } {
  const { items, error } = parseJsonLines<Record<string, unknown>>(text);
  if (error) return { channels: [], error };
  return { channels: items.map((item, i) => normalizeChannel(item, i + 1)) };
}

export function serializeChannels(channels: NotifyChannel[]): string {
  return toJsonLines(
    channels.map((c) => {
      const row: Record<string, unknown> = {
        id: c.id.trim() || c.kind,
        kind: c.kind,
        enabled: Boolean(c.enabled),
      };
      if (c.url?.trim()) row.url = c.url.trim();
      if (c.token?.trim()) row.token = c.token.trim();
      if (c.user?.trim()) row.user = c.user.trim();
      if (c.title?.trim()) row.title = c.title.trim();
      if (c.smtpHost?.trim()) row.smtpHost = c.smtpHost.trim();
      if (typeof c.smtpPort === "number" && Number.isFinite(c.smtpPort) && c.smtpPort > 0) {
        row.smtpPort = c.smtpPort;
      }
      if (c.from?.trim()) row.from = c.from.trim();
      if (c.to?.trim()) row.to = c.to.trim();
      if (c.password?.trim()) row.password = c.password.trim();
      return row;
    }),
  );
}
