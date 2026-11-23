import type { MessageKey } from "@/i18n/messages";

/** First-party download-queue providers shown as Files tabs when the extension is active. */
export type QueueControlAction = "pause" | "resume" | "remove" | "abort" | "restart" | "delete" | "stop" | "force";

export type QueueFamily = "hoster" | "nzb";

export type DownloadQueueProvider = {
  extensionId: string;
  /** i18n key for the tab label */
  labelKey: MessageKey;
  listMethod: string;
  controlMethod: string;
  actions: QueueControlAction[];
  family: QueueFamily;
};

export const DOWNLOAD_QUEUE_PROVIDERS: DownloadQueueProvider[] = [
  {
    extensionId: "org.fleet.jdownloader",
    labelKey: "files.tab.jdownloader",
    listMethod: "ext.jdownloader.listQueue",
    controlMethod: "ext.jdownloader.control",
    actions: ["pause", "resume", "remove"],
    family: "hoster",
  },
  {
    extensionId: "org.fleet.pyload",
    labelKey: "files.tab.pyload",
    listMethod: "ext.pyload.listQueue",
    controlMethod: "ext.pyload.control",
    actions: ["pause", "resume", "abort", "restart", "delete"],
    family: "hoster",
  },
];

/** medias/<client> app id → queue extension (sidebar opens queue, not a top tab). */
export const APP_ID_TO_QUEUE_EXT: Record<string, string> = {
  jdownloader: "org.fleet.jdownloader",
  pyload: "org.fleet.pyload",
};

export function countActiveQueueProviders(
  activeIds: Iterable<string>,
  family?: QueueFamily,
): number {
  const set = new Set(activeIds);
  return DOWNLOAD_QUEUE_PROVIDERS.filter(
    (p) => set.has(p.extensionId) && (!family || p.family === family),
  ).length;
}

export const FILES_TAB_FOLDERS = "folders";
