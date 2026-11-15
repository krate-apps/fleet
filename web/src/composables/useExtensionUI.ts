import { computed, onMounted, ref, shallowRef } from "vue";
import {
  activateExtension,
  consentExtension,
  deactivateExtension,
  fetchExtensions,
} from "@/api/client";
import type { ExtensionRecord, UIAction, UIColumn, UIPage, UIPanel } from "@/extensions/types";

const extensions = shallowRef<ExtensionRecord[]>([]);
const loading = ref(false);
const error = ref("");
let loaded = false;

async function refresh(force = false) {
  loading.value = true;
  error.value = "";
  try {
    const res = await fetchExtensions(force);
    extensions.value = res.extensions ?? [];
    loaded = true;
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

export function useExtensionUI() {
  onMounted(() => {
    if (!loaded) void refresh();
  });

  const active = computed(() => extensions.value.filter((e) => e.state === "active"));

  const pages = computed(() => {
    const out: Array<UIPage & { extensionId: string; extensionName: string }> = [];
    for (const ext of active.value) {
      for (const page of ext.ui?.pages ?? []) {
        out.push({ ...page, extensionId: ext.id, extensionName: ext.name });
      }
    }
    return out;
  });

  const columns = computed(() => {
    const out: Array<UIColumn & { extensionId: string }> = [];
    for (const ext of active.value) {
      for (const col of ext.ui?.columns ?? []) {
        out.push({ ...col, extensionId: ext.id });
      }
    }
    return out;
  });

  const globalActions = computed(() => {
    const out: Array<UIAction & { extensionId: string }> = [];
    for (const ext of active.value) {
      for (const action of ext.ui?.actions ?? []) {
        if (action.target === "global") {
          out.push({ ...action, extensionId: ext.id });
        }
      }
    }
    return out;
  });

  const selectionActions = computed(() => {
    const out: Array<UIAction & { extensionId: string }> = [];
    for (const ext of active.value) {
      for (const action of ext.ui?.actions ?? []) {
        if (action.target === "selection" || action.target === "torrent") {
          out.push({ ...action, extensionId: ext.id });
        }
      }
    }
    return out;
  });

  const sidebarPanels = computed(() => {
    const out: Array<UIPanel & { extensionId: string; extensionName: string }> = [];
    for (const ext of active.value) {
      for (const panel of ext.ui?.panels ?? []) {
        if (panel.placement === "sidebar") {
          out.push({ ...panel, extensionId: ext.id, extensionName: ext.name });
        }
      }
    }
    return out;
  });

  async function activate(id: string, opts?: { acceptRisk?: boolean }) {
    const current = extensions.value.find((e) => e.id === id);
    if (current?.consentRequired && !current.consented) {
      if (!opts?.acceptRisk) {
        throw new Error(current.consentWarning || "Third-party extension consent required");
      }
      await consentExtension(id);
    }
    const rec = await activateExtension(id);
    await refresh();
    return rec;
  }

  async function deactivate(id: string) {
    const rec = await deactivateExtension(id);
    await refresh();
    return rec;
  }

  function findPage(extensionId: string, pageId: string) {
    const ext = extensions.value.find((e) => e.id === extensionId);
    if (!ext?.ui) return null;
    const page = ext.ui.pages?.find((p) => p.id === pageId);
    if (!page) return null;
    return { extension: ext, page, forms: ext.ui.forms ?? [], actions: ext.ui.actions ?? [] };
  }

  return {
    extensions,
    active,
    pages,
    columns,
    globalActions,
    selectionActions,
    sidebarPanels,
    loading,
    error,
    refresh,
    activate,
    deactivate,
    findPage,
  };
}
