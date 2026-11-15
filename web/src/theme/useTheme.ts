import { computed, ref, watch } from "vue";
import {
  DEFAULT_THEME_ID,
  THEME_STORAGE_KEY,
  getTheme,
  listThemes,
  resolveThemeId,
  themeRegistryRevision,
} from "./themes";

const currentId = ref(DEFAULT_THEME_ID);

function applyToDocument(id: string) {
  const theme = getTheme(id) ?? getTheme(DEFAULT_THEME_ID)!;
  const root = document.documentElement;
  root.setAttribute("data-fleet-theme", theme.id);
  root.style.colorScheme = theme.colorScheme;
}

/** Call once at app boot before mount so first paint uses the stored theme. */
export function initThemeFromStorage(): void {
  const id = resolveThemeId(
    typeof localStorage !== "undefined" ? localStorage.getItem(THEME_STORAGE_KEY) : null,
  );
  currentId.value = id;
  if (typeof document !== "undefined") {
    applyToDocument(id);
  }
}

export function useTheme() {
  watch(
    currentId,
    (id) => {
      localStorage.setItem(THEME_STORAGE_KEY, id);
      applyToDocument(id);
    },
    { flush: "sync" },
  );

  const theme = computed(() => getTheme(currentId.value) ?? getTheme(DEFAULT_THEME_ID)!);
  const themes = computed(() => {
    void themeRegistryRevision.value;
    return listThemes();
  });

  function setTheme(id: string) {
    currentId.value = resolveThemeId(id);
  }

  function toggleLightDark() {
    setTheme(currentId.value === "light" ? "dark" : "light");
  }

  return {
    themeId: currentId,
    theme,
    themes,
    setTheme,
    toggleLightDark,
  };
}
