import { createRouter, createWebHistory } from "vue-router";
import { fetchAuthMe, fetchAuthStatus } from "./api/client";
import FilesView from "./views/FilesView.vue";
import TorrentsView from "./views/TorrentsView.vue";
import LoginView from "./views/LoginView.vue";
import SetupView from "./views/SetupView.vue";

export const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: "/login", name: "login", component: LoginView, meta: { public: true } },
    { path: "/setup", name: "setup", component: SetupView, meta: { public: true } },
    { path: "/", name: "torrents", component: TorrentsView },
    { path: "/torrents", redirect: "/" },
    { path: "/files", name: "files", component: FilesView },
    { path: "/extensions", redirect: { path: "/", query: { modal: "extensions" } } },
    { path: "/settings", redirect: { path: "/", query: { modal: "settings" } } },
    {
      path: "/ext/:extensionId/:pageId",
      redirect: (to) => {
        // File Manager product UI is the native /files view (gated on activation).
        if (
          String(to.params.extensionId) === "org.fleet.filemanager" &&
          String(to.params.pageId) === "browse"
        ) {
          return { path: "/files" };
        }
        return {
          path: "/",
          query: {
            modal: "extensions",
            ext: String(to.params.extensionId),
            page: String(to.params.pageId),
          },
        };
      },
    },
  ],
});

router.beforeEach(async (to) => {
  if (to.meta.public) return true;
  try {
    const status = await fetchAuthStatus();
    if (!status.enabled) return true;
    if (status.needsSetup) return { name: "setup" };
    await fetchAuthMe();
    return true;
  } catch (e) {
    const err = e as Error & { status?: number; code?: string };
    if (err.code === "needs_setup") return { name: "setup" };
    return { name: "login", query: { redirect: to.fullPath } };
  }
});
