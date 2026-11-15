import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router";
import { loadInstalledThemes } from "./theme/loadInstalledThemes";
import { initThemeFromStorage } from "./theme/useTheme";
import "./styles/base.css";

async function boot() {
  initThemeFromStorage();
  await loadInstalledThemes();
  // Re-apply in case a pack theme was stored and just registered.
  initThemeFromStorage();
  createApp(App).use(router).mount("#app");
}

void boot();
