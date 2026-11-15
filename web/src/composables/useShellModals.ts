import { ref } from "vue";

const showSettings = ref(false);
const showExtensions = ref(false);
const showShortcuts = ref(false);
const extensionsFilter = ref("");
const settingsSection = ref<string>("");

export function useShellModals() {
  function openSettings(section?: string) {
    showExtensions.value = false;
    showShortcuts.value = false;
    if (section) settingsSection.value = section;
    showSettings.value = true;
  }

  function openExtensions(opts?: { filter?: string }) {
    showSettings.value = false;
    showShortcuts.value = false;
    if (opts?.filter != null) extensionsFilter.value = opts.filter;
    showExtensions.value = true;
  }

  function openShortcuts() {
    showSettings.value = false;
    showExtensions.value = false;
    showShortcuts.value = true;
  }

  function closeSettings() {
    showSettings.value = false;
    settingsSection.value = "";
  }

  function closeExtensions() {
    showExtensions.value = false;
  }

  function closeShortcuts() {
    showShortcuts.value = false;
  }

  function closeAll() {
    showSettings.value = false;
    showExtensions.value = false;
    showShortcuts.value = false;
    settingsSection.value = "";
  }

  const anyModalOpen = () => showSettings.value || showExtensions.value || showShortcuts.value;

  return {
    showSettings,
    showExtensions,
    showShortcuts,
    extensionsFilter,
    settingsSection,
    openSettings,
    openExtensions,
    openShortcuts,
    closeSettings,
    closeExtensions,
    closeShortcuts,
    closeAll,
    anyModalOpen,
  };
}
