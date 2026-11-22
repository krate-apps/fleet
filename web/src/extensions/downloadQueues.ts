import type { MessageKey } from "@/i18n/messages";

/** First-party download-queue providers shown as Files tabs when the extension is active. */
export type QueueControlAction = "pause" | "resume" | "remove" | "abort" | "restart" | "delete" | "stop" | "force";

export type DownloadQueueProvider = {
  extensionId: string;
  /** i18n key for the tab label */
  labelKey: MessageKey;
  listMethod: string;
  controlMethod: string;
  actions: QueueControlAction[];
};

export const DOWNLOAD_QUEUE_PROVIDERS: DownloadQueueProvider[] = [
  {
    extensionId: "org.fleet.jdownloader",
    labelKey: "files.tab.jdownloader",
    listMethod: "ext.jdownloader.listQueue",
    controlMethod: "ext.jdownloader.control",
    actions: ["pause", "resume", "remove"],
  },
  {
    extensionId: "org.fleet.pyload",
    labelKey: "files.tab.pyload",
    listMethod: "ext.pyload.listQueue",
    controlMethod: "ext.pyload.control",
    actions: ["pause", "resume", "abort", "restart", "delete"],
  },
];

export const FILES_TAB_FOLDERS = "folders";
