import { withBase } from "@/api/client";
import { registerTheme } from "./themes";

type PackTheme = {
  id: string;
  label: string;
  colorScheme: "dark" | "light";
  cssPath: string;
};

const injected = new Set<string>();

/** Fetch installed theme packs and inject CSS + registry entries. */
export async function loadInstalledThemes(): Promise<void> {
  try {
    const res = await fetch(withBase("/api/v1/themes"), { credentials: "same-origin" });
    if (!res.ok) return;
    const data = (await res.json()) as { themes?: PackTheme[] };
    const packs = data.themes ?? [];
    await Promise.all(
      packs.map(async (p) => {
        if (!p?.id) return;
        registerTheme({
          id: p.id,
          label: p.label || p.id,
          colorScheme: p.colorScheme === "light" ? "light" : "dark",
        });
        if (injected.has(p.id)) return;
        const cssPath = p.cssPath || `/api/v1/themes/${encodeURIComponent(p.id)}/css`;
        const cssRes = await fetch(withBase(cssPath), { credentials: "same-origin" });
        if (!cssRes.ok) return;
        const css = await cssRes.text();
        const style = document.createElement("style");
        style.setAttribute("data-fleet-theme-pack", p.id);
        style.textContent = css;
        document.head.appendChild(style);
        injected.add(p.id);
      }),
    );
  } catch {
    // Pack themes are optional; builtins remain.
  }
}
