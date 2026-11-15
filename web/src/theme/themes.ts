/**
 * Fleet theme registry.
 * Built-in: dark + light only. Pack themes register via loadInstalledThemes
 * (CSS [data-fleet-theme="id"] + registerTheme) without changing call sites.
 */

import { ref } from "vue";

export type ThemeDefinition = {
  id: string;
  label: string;
  /** Maps to CSS color-scheme and prefers-color-scheme hints. */
  colorScheme: "dark" | "light";
};

const themes = new Map<string, ThemeDefinition>();

/** Bumped on register so Vue computeds can react to pack installs. */
export const themeRegistryRevision = ref(0);

export function registerTheme(theme: ThemeDefinition): void {
  if (!theme.id.trim()) {
    throw new Error("theme id required");
  }
  themes.set(theme.id, theme);
  themeRegistryRevision.value += 1;
}

export function listThemes(): ThemeDefinition[] {
  return [...themes.values()];
}

export function getTheme(id: string): ThemeDefinition | undefined {
  return themes.get(id);
}

export const DEFAULT_THEME_ID = "dark";
export const THEME_STORAGE_KEY = "fleet.theme";

registerTheme({ id: "dark", label: "Dark", colorScheme: "dark" });
registerTheme({ id: "light", label: "Light", colorScheme: "light" });

export function resolveThemeId(raw: string | null | undefined): string {
  if (raw && themes.has(raw)) return raw;
  return DEFAULT_THEME_ID;
}
