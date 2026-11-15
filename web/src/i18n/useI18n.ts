import { computed, ref } from "vue";
import { messages, type Locale, type MessageKey } from "./messages";

const STORAGE_KEY = "fleet.locale";

function detectLocale(): Locale {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "en" || saved === "fr") return saved;
  } catch {
    // ignore
  }
  const nav = typeof navigator !== "undefined" ? navigator.language.toLowerCase() : "en";
  return nav.startsWith("fr") ? "fr" : "en";
}

const locale = ref<Locale>(detectLocale());

export function useI18n() {
  const localeRef = locale;

  function setLocale(next: Locale) {
    locale.value = next;
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // ignore
    }
    if (typeof document !== "undefined") {
      document.documentElement.lang = next;
    }
  }

  function t(key: MessageKey, vars?: Record<string, string | number>): string {
    const table = messages[locale.value] ?? messages.en;
    let out: string = table[key] ?? messages.en[key] ?? String(key);
    if (vars) {
      for (const [k, v] of Object.entries(vars)) {
        out = out.replaceAll(`{${k}}`, String(v));
      }
    }
    return out;
  }

  if (typeof document !== "undefined") {
    document.documentElement.lang = locale.value;
  }

  return {
    locale: localeRef,
    locales: [
      { id: "en" as const, label: "English" },
      { id: "fr" as const, label: "Français" },
    ],
    setLocale,
    t,
    isFr: computed(() => locale.value === "fr"),
  };
}
